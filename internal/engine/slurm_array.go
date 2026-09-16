package engine

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/otterlab-bio/otter/internal/config"
	"github.com/otterlab-bio/otter/internal/enva"
	"github.com/otterlab-bio/otter/internal/logger"
	taskruntime "github.com/otterlab-bio/otter/internal/task"
)

// SlurmArrayEngine represents a SLURM Job Array engine for multi-sample parallelization
type SlurmArrayEngine struct {
	*SlurmEngine
	samples      []string
	stepResource *config.StepResource
	arraySize    int
	maxArrayJobs int
	maxBatchSize int     // 每批最大 Task 数（0 = 不限制，一次提交全部）
	loadRatio    float64 // > 0: 动态池模式；0: 旧批处理模式
}

// NewSlurmArrayEngine creates a new SlurmArrayEngine
func NewSlurmArrayEngine(config *SlurmConfig, samples []string, stepResource *config.StepResource) *SlurmArrayEngine {
	slurmEngine := NewSlurmEngine(config)
	maxArrayJobs := 0
	if stepResource != nil {
		maxArrayJobs = stepResource.MaxJobs
	}

	return &SlurmArrayEngine{
		SlurmEngine:  slurmEngine,
		samples:      samples,
		stepResource: stepResource,
		arraySize:    len(samples),
		maxArrayJobs: maxArrayJobs,
	}
}

// ExecuteStepWithArray executes a workflow step using SLURM Job Array for multi-sample parallelization
func (e *SlurmArrayEngine) ExecuteStepWithArray(step int, condaEnv string, workflowFile string, configFile string, stepResource *config.StepResource) error {
	e.status.State = StatusRunning
	e.status.Message = fmt.Sprintf("Executing Step %d with Job Array (%d samples)", step, e.arraySize)

	// Use updated MaxJobs from stepResource for unified parallelization control
	maxJobs := stepResource.MaxJobs
	if maxJobs > 0 {
		e.maxArrayJobs = maxJobs
	}

	// 动态池优先：loadRatio > 0 时使用动态池调度
	if e.loadRatio > 0 {
		return e.executeWithDynamicPool(step, condaEnv, workflowFile, configFile)
	}

	// 检查是否需要分批提交
	if e.maxBatchSize > 0 && len(e.samples) > e.maxBatchSize {
		return e.executeStepWithBatches(step, condaEnv, workflowFile, configFile)
	}

	// 原有逻辑：一次提交所有样本
	scriptPath, err := e.generateArrayScript(step, condaEnv, workflowFile, configFile)
	if err != nil {
		return fmt.Errorf("failed to generate Job Array script: %w", err)
	}

	e.scriptPath = scriptPath

	// Submit Job Array
	jobID, err := e.submitArrayJob(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to submit Job Array: %w", err)
	}

	e.status.JobID = jobID
	logger.Infof("SLURM Job Array submitted: %s (Array size: %d)", jobID, e.arraySize)

	// Wait for completion
	if err := e.waitForArrayJob(jobID, e.arraySize); err != nil {
		return fmt.Errorf("Job Array execution failed: %w", err)
	}

	return nil
}

// buildCondaCommand constructs the conda/enva prefix for snakemake invocation.
func buildCondaCommand(condaEnv string) string {
	if condaEnv != "" {
		if enva.IsAvailable() {
			return fmt.Sprintf("enva run %s --", condaEnv)
		}
		return fmt.Sprintf("conda run -n %s", condaEnv)
	}
	return ""
}

// executeWithDynamicPool 动态池调度：始终保持 slot_limit 个 job 并发运行。
// slot_limit = floor(min(maxArrayJobs, MaxSubmitJobs-1) × loadRatio)，最小为 1。
func (e *SlurmArrayEngine) executeWithDynamicPool(step int, condaEnv, workflowFile, configFile string) error {
	// 1. 计算槽位限制
	limits := GetSlurmUserLimits()
	slurmMax := limits.MaxSubmitJobs - 1
	if slurmMax < 1 {
		slurmMax = 1
	}
	effectiveMax := slurmMax
	if e.maxArrayJobs > 0 && e.maxArrayJobs < effectiveMax {
		effectiveMax = e.maxArrayJobs
	}
	slotLimit := int(math.Floor(float64(effectiveMax) * e.loadRatio))
	if slotLimit < 1 {
		slotLimit = 1
	}
	logger.Infof("Dynamic pool: slot_limit=%d (min(parallel-jobs=%d, MaxSubmitJobs-1=%d) × %.2f)",
		slotLimit, e.maxArrayJobs, slurmMax, e.loadRatio)

	// 2. 初始化状态
	pending := make([]string, len(e.samples))
	copy(pending, e.samples)
	running := make(map[string]string) // jobID → sampleName
	var completed, failed []string

	// 3. 初始填充
	pending = e.refillPool(step, condaEnv, workflowFile, configFile, pending, running, slotLimit)

	// 4. 轮询循环
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for len(pending) > 0 || len(running) > 0 {
		<-ticker.C

		// 检查运行中 jobs 的状态
		for jobID, sample := range running {
			state := GetJobState(jobID)
			switch state {
			case "COMPLETED":
				completed = append(completed, sample)
				delete(running, jobID)
				if err := taskruntime.CompleteCurrentSlurmJob(jobID); err != nil {
					logger.Warnf("Failed to update completed SLURM job %s: %v", jobID, err)
				}
			case "FAILED", "CANCELLED", "TIMEOUT":
				failed = append(failed, sample)
				delete(running, jobID)
				if err := taskruntime.CompleteCurrentSlurmJob(jobID); err != nil {
					logger.Warnf("Failed to update finished SLURM job %s: %v", jobID, err)
				}
				logger.Errorf("Sample %s failed (job %s, state %s)", sample, jobID, state)
			}
		}

		// 动态填充空槽
		pending = e.refillPool(step, condaEnv, workflowFile, configFile, pending, running, slotLimit)

		logger.Infof("Pool status: %d running, %d pending, %d completed, %d failed",
			len(running), len(pending), len(completed), len(failed))

		// 有失败立即终止（fail-fast）
		if len(failed) > 0 {
			e.status.State = StatusFailed
			return fmt.Errorf("dynamic pool: %d sample(s) failed: %v", len(failed), failed)
		}
	}

	e.status.State = StatusCompleted
	e.status.EndTime = time.Now()
	logger.Infof("Dynamic pool completed: %d/%d samples", len(completed), len(e.samples))
	return nil
}

// refillPool 向运行池中补充样本，直到达到 slotLimit 或 pending 耗尽。
func (e *SlurmArrayEngine) refillPool(
	step int, condaEnv, workflowFile, configFile string,
	pending []string, running map[string]string, slotLimit int,
) []string {
	for len(running) < slotLimit && len(pending) > 0 {
		sample := pending[0]
		pending = pending[1:]

		scriptPath, err := e.generateSingleSampleScript(step, condaEnv, workflowFile, configFile, sample)
		if err != nil {
			logger.Errorf("Failed to generate script for %s: %v", sample, err)
			continue
		}
		jobID, err := e.submitSingleJob(scriptPath)
		if err != nil {
			logger.Errorf("Failed to submit job for %s: %v", sample, err)
			continue
		}
		running[jobID] = sample
		logger.Infof("Submitted job %s for sample %s (%d/%d slots used)",
			jobID, sample, len(running), slotLimit)
	}
	return pending
}

// generateSingleSampleScript 为单个样本生成独立的 SLURM 脚本（SAMPLE_NAME 硬编码）。
func (e *SlurmArrayEngine) generateSingleSampleScript(
	step int, condaEnv, workflowFile, configFile, sample string,
) (string, error) {
	tmpDir := e.logDir
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	scriptPath := filepath.Join(tmpDir,
		fmt.Sprintf("otter_pool_step%d_%s_%s.sh", step, e.jobName, sample))

	const scriptTemplate = `#!/bin/bash
#SBATCH --job-name={{.JobName}}
#SBATCH --partition={{.Partition}}
#SBATCH --cpus-per-task={{.Cores}}
#SBATCH --mem={{.Memory}}
#SBATCH --output={{.OutputFile}}
#SBATCH --error={{.ErrorFile}}
#SBATCH --time={{.Time}}

set -e

cd {{.WorkDir}}

SAMPLE_NAME="{{.SampleName}}"

{{.CondaCommand}} snakemake --cores all \
  --snakefile {{.WorkflowFile}} \
  --configfile {{.ConfigFile}} \
  --rerun-incomplete --nolock \
  --config "SIDs=['$SAMPLE_NAME']"
`

	cores := e.cores
	memory := e.memory
	if e.stepResource != nil {
		if e.stepResource.Cores > 0 {
			cores = e.stepResource.Cores
		}
		if e.stepResource.Memory != "" {
			memory = e.stepResource.Memory
		}
	}
	slurmMemory, err := formatSlurmMemory(memory)
	if err != nil {
		return "", err
	}

	data := struct {
		JobName, Partition, Memory, Time, WorkDir string
		Cores                                     int
		OutputFile, ErrorFile                     string
		SampleName, CondaCommand                  string
		WorkflowFile, ConfigFile                  string
	}{
		JobName:      fmt.Sprintf("%s_step%d_%s", e.jobName, step, sample),
		Partition:    e.partition,
		Cores:        cores,
		Memory:       slurmMemory,
		Time:         e.timeLimit,
		WorkDir:      e.getWorkDir(),
		OutputFile:   filepath.Join(tmpDir, fmt.Sprintf("%s_step%d_%s_%%j.out", e.jobName, step, sample)),
		ErrorFile:    filepath.Join(tmpDir, fmt.Sprintf("%s_step%d_%s_%%j.err", e.jobName, step, sample)),
		SampleName:   sample,
		CondaCommand: buildCondaCommand(condaEnv),
		WorkflowFile: workflowFile,
		ConfigFile:   configFile,
	}

	tmpl, err := template.New("slurm_pool").Parse(scriptTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	if err := os.WriteFile(scriptPath, buf.Bytes(), 0755); err != nil {
		return "", err
	}
	return scriptPath, nil
}

// submitSingleJob submits a single SLURM script and returns its job ID.
func (e *SlurmArrayEngine) submitSingleJob(scriptPath string) (string, error) {
	return e.submitSlurmScript(scriptPath, "single-sample job")
}

// executeStepWithBatches 将样本分批提交，每批独立提交并等待完成
func (e *SlurmArrayEngine) executeStepWithBatches(step int, condaEnv string, workflowFile string, configFile string) error {
	batches := splitIntoBatches(e.samples, e.maxBatchSize)
	totalBatches := len(batches)
	logger.Infof("Splitting %d samples into %d batches (max %d per batch)", len(e.samples), totalBatches, e.maxBatchSize)

	for i, batch := range batches {
		logger.Infof("Submitting batch %d/%d: %d samples as Job Array", i+1, totalBatches, len(batch))

		scriptPath, err := e.generateArrayScriptForBatch(step, condaEnv, workflowFile, configFile, batch, i)
		if err != nil {
			return fmt.Errorf("batch %d/%d: failed to generate script: %w", i+1, totalBatches, err)
		}

		jobID, err := e.submitArrayJob(scriptPath)
		if err != nil {
			return fmt.Errorf("batch %d/%d: failed to submit Job Array: %w", i+1, totalBatches, err)
		}

		e.status.JobID = jobID
		logger.Infof("SLURM Job Array submitted: %s", jobID)

		if err := e.waitForArrayJob(jobID, len(batch)); err != nil {
			return fmt.Errorf("batch %d/%d: Job Array execution failed: %w", i+1, totalBatches, err)
		}

		logger.Infof("Batch %d/%d completed successfully", i+1, totalBatches)
	}

	e.status.State = StatusCompleted
	e.status.EndTime = time.Now()
	e.status.Message = "All batches completed successfully"
	logger.Info("All batches completed successfully")

	return nil
}

// splitIntoBatches 将切片分成指定大小的批次
func splitIntoBatches(items []string, batchSize int) [][]string {
	if batchSize <= 0 || len(items) <= batchSize {
		return [][]string{items}
	}
	var batches [][]string
	for len(items) > batchSize {
		batches = append(batches, items[:batchSize])
		items = items[batchSize:]
	}
	if len(items) > 0 {
		batches = append(batches, items)
	}
	return batches
}

// generateArrayScript generates a SLURM Job Array batch script
func (e *SlurmArrayEngine) generateArrayScript(step int, condaEnv string, workflowFile string, configFile string) (string, error) {
	// Use log directory if set, otherwise fall back to temp directory
	tmpDir := e.logDir
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	scriptPath := filepath.Join(tmpDir, fmt.Sprintf("otter_array_step%d_%s.sh", step, e.jobName))

	// Prepare sample array for script
	sampleLines := make([]string, len(e.samples))
	for i, sample := range e.samples {
		sampleLines[i] = fmt.Sprintf(`SAMPLES[%d]=%q`, i, sample)
	}

	// Job Array script template
	const scriptTemplate = `#!/bin/bash
#SBATCH --job-name={{.JobName}}
#SBATCH --partition={{.Partition}}
#SBATCH --cpus-per-task={{.Cores}}
#SBATCH --mem={{.Memory}}
#SBATCH --output={{.OutputFile}}
#SBATCH --error={{.ErrorFile}}
#SBATCH --array=0-{{.ArraySize}}{{.MaxArrayJobs}}
#SBATCH --time={{.Time}}

set -e

# Sample array
{{.SampleLines}}

# Change to working directory
cd {{.WorkDir}}

# Get sample name for this array task
SAMPLE_NAME=${SAMPLES[$SLURM_ARRAY_TASK_ID]}

# Execute step using conda environment
{{.CondaCommand}} snakemake --cores all --snakefile {{.WorkflowFile}} --configfile {{.ConfigFile}} --rerun-incomplete --nolock --config "SIDs=['$SAMPLE_NAME']"

# Touch success file for this task
touch {{.SuccessFile}}.$SLURM_ARRAY_TASK_ID
`

	slurmMemory, err := formatSlurmMemory(e.stepResource.Memory)
	if err != nil {
		return "", err
	}

	data := struct {
		JobName      string
		Partition    string
		Cores        int
		Memory       string
		Time         string
		WorkDir      string
		OutputFile   string
		ErrorFile    string
		SuccessFile  string
		ArraySize    int
		MaxArrayJobs string
		SampleLines  string
		CondaCommand string
		Step         int
		WorkflowFile string
		ConfigFile   string
	}{
		JobName:     fmt.Sprintf("%s_step%d_array", e.jobName, step),
		Partition:   e.partition,
		Cores:       e.stepResource.Cores,
		Memory:      slurmMemory,
		Time:        e.timeLimit,
		WorkDir:     e.getWorkDir(),
		OutputFile:  filepath.Join(tmpDir, fmt.Sprintf("%s_step%d_array_%%A_%%a.out", e.jobName, step)),
		ErrorFile:   filepath.Join(tmpDir, fmt.Sprintf("%s_step%d_array_%%A_%%a.err", e.jobName, step)),
		SuccessFile: filepath.Join(tmpDir, fmt.Sprintf("%s_step%d_array_success", e.jobName, step)),
		ArraySize:   e.arraySize,
		MaxArrayJobs: func() string {
			if e.maxArrayJobs > 0 {
				return fmt.Sprintf("%%%d", e.maxArrayJobs)
			}
			return ""
		}(),
		SampleLines: strings.Join(sampleLines, "\n"),
		CondaCommand: func() string {
			if condaEnv != "" {
				if enva.IsAvailable() {
					// 使用 enva: enva run <env> --
					return fmt.Sprintf("enva run %s --", condaEnv)
				}
				// 回退到 conda: conda run -n <env>
				return fmt.Sprintf("conda run -n %s", condaEnv)
			}
			return ""
		}(),
		Step:         step,
		WorkflowFile: workflowFile,
		ConfigFile:   configFile,
	}

	tmpl, err := template.New("slurm_array").Parse(scriptTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	// Write script to file
	if err := os.WriteFile(scriptPath, buf.Bytes(), 0755); err != nil {
		return "", err
	}

	return scriptPath, nil
}

// generateArrayScriptForBatch generates a SLURM Job Array script for a specific batch of samples
func (e *SlurmArrayEngine) generateArrayScriptForBatch(step int, condaEnv string, workflowFile string, configFile string, samples []string, batchIdx int) (string, error) {
	tmpDir := e.logDir
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	scriptPath := filepath.Join(tmpDir, fmt.Sprintf("otter_array_step%d_%s_batch%d.sh", step, e.jobName, batchIdx))

	// Prepare sample array for script
	sampleLines := make([]string, len(samples))
	for i, sample := range samples {
		sampleLines[i] = fmt.Sprintf(`SAMPLES[%d]=%q`, i, sample)
	}

	// ArraySize = len(samples) - 1 for 0-indexed SLURM array
	arraySize := len(samples) - 1
	if arraySize < 0 {
		arraySize = 0
	}

	const scriptTemplate = `#!/bin/bash
#SBATCH --job-name={{.JobName}}
#SBATCH --partition={{.Partition}}
#SBATCH --cpus-per-task={{.Cores}}
#SBATCH --mem={{.Memory}}
#SBATCH --output={{.OutputFile}}
#SBATCH --error={{.ErrorFile}}
#SBATCH --array=0-{{.ArraySize}}{{.MaxArrayJobs}}
#SBATCH --time={{.Time}}

set -e

# Sample array
{{.SampleLines}}

# Change to working directory
cd {{.WorkDir}}

# Get sample name for this array task
SAMPLE_NAME=${SAMPLES[$SLURM_ARRAY_TASK_ID]}

# Execute step using conda environment
{{.CondaCommand}} snakemake --cores all --snakefile {{.WorkflowFile}} --configfile {{.ConfigFile}} --rerun-incomplete --nolock --config "SIDs=['$SAMPLE_NAME']"

# Touch success file for this task
touch {{.SuccessFile}}.$SLURM_ARRAY_TASK_ID
`

	slurmMemory, err := formatSlurmMemory(e.stepResource.Memory)
	if err != nil {
		return "", err
	}

	data := struct {
		JobName      string
		Partition    string
		Cores        int
		Memory       string
		Time         string
		WorkDir      string
		OutputFile   string
		ErrorFile    string
		SuccessFile  string
		ArraySize    int
		MaxArrayJobs string
		SampleLines  string
		CondaCommand string
		Step         int
		WorkflowFile string
		ConfigFile   string
	}{
		JobName:     fmt.Sprintf("%s_step%d_array_b%d", e.jobName, step, batchIdx),
		Partition:   e.partition,
		Cores:       e.stepResource.Cores,
		Memory:      slurmMemory,
		Time:        e.timeLimit,
		WorkDir:     e.getWorkDir(),
		OutputFile:  filepath.Join(tmpDir, fmt.Sprintf("%s_step%d_batch%d_array_%%A_%%a.out", e.jobName, step, batchIdx)),
		ErrorFile:   filepath.Join(tmpDir, fmt.Sprintf("%s_step%d_batch%d_array_%%A_%%a.err", e.jobName, step, batchIdx)),
		SuccessFile: filepath.Join(tmpDir, fmt.Sprintf("%s_step%d_batch%d_array_success", e.jobName, step, batchIdx)),
		ArraySize:   arraySize,
		MaxArrayJobs: func() string {
			if e.maxArrayJobs > 0 {
				return fmt.Sprintf("%%%d", e.maxArrayJobs)
			}
			return ""
		}(),
		SampleLines: strings.Join(sampleLines, "\n"),
		CondaCommand: func() string {
			if condaEnv != "" {
				if enva.IsAvailable() {
					return fmt.Sprintf("enva run %s --", condaEnv)
				}
				return fmt.Sprintf("conda run -n %s", condaEnv)
			}
			return ""
		}(),
		Step:         step,
		WorkflowFile: workflowFile,
		ConfigFile:   configFile,
	}

	tmpl, err := template.New("slurm_array_batch").Parse(scriptTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	if err := os.WriteFile(scriptPath, buf.Bytes(), 0755); err != nil {
		return "", err
	}

	return scriptPath, nil
}

// submitArrayJob submits a SLURM Job Array batch job
func (e *SlurmArrayEngine) submitArrayJob(scriptPath string) (string, error) {
	return e.submitSlurmScript(scriptPath, "job array")
}

// waitForArrayJob waits for the SLURM Job Array to complete
func (e *SlurmArrayEngine) waitForArrayJob(jobID string, totalTasks int) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	completedTasks := 0

	for {
		select {
		case <-ticker.C:
			status, err := e.checkArrayJobStatus(jobID)
			if err != nil {
				logger.Warnf("Failed to check Job Array status: %v", err)
				continue
			}

			completedTasks = status.Completed
			e.status.Progress = (completedTasks * 100) / totalTasks
			e.status.Message = fmt.Sprintf("Job Array running: %d/%d tasks completed", completedTasks, totalTasks)

			logger.Infof("Job Array progress: %d/%d tasks completed", completedTasks, totalTasks)

			if completedTasks >= totalTasks {
				e.status.State = StatusCompleted
				e.status.EndTime = time.Now()
				e.status.Message = "Job Array completed successfully"
				if err := taskruntime.CompleteCurrentSlurmJob(jobID); err != nil {
					logger.Warnf("Failed to update completed SLURM Job Array %s: %v", jobID, err)
				}
				logger.Info("Job Array completed successfully")
				return nil
			}

			// Check if any tasks failed
			if status.Failed > 0 {
				e.status.State = StatusFailed
				e.status.EndTime = time.Now()
				e.status.Message = fmt.Sprintf("Job Array failed: %d tasks failed", status.Failed)
				if err := taskruntime.CompleteCurrentSlurmJob(jobID); err != nil {
					logger.Warnf("Failed to update failed SLURM Job Array %s: %v", jobID, err)
				}
				return fmt.Errorf("Job Array execution failed: %d tasks failed", status.Failed)
			}
		}
	}
}

// ArrayJobStatus represents the status of a Job Array
type ArrayJobStatus struct {
	Completed int
	Running   int
	Pending   int
	Failed    int
}

// checkArrayJobStatus checks the status of a SLURM Job Array
func (e *SlurmArrayEngine) checkArrayJobStatus(jobID string) (*ArrayJobStatus, error) {
	// Extract base job ID (without array index)
	baseJobID := strings.Split(jobID, "[")[0]

	// Use squeue to check job array status
	cmd := exec.Command("squeue", "-j", baseJobID, "-o", "%T", "--noheader")
	output, err := cmd.Output()
	if err != nil {
		// Job might have completed, use sacct to get final status
		return e.getArrayJobFinalStatus(baseJobID)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		// An empty queue result means the array may have left the queue; use
		// accounting rather than interpreting it as zero completed tasks.
		return e.getArrayJobFinalStatus(baseJobID)
	}
	lines := strings.Split(outputStr, "\n")

	status := &ArrayJobStatus{
		Completed: 0,
		Running:   0,
		Pending:   0,
		Failed:    0,
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch normalizeSlurmState(line) {
		case "RUNNING", "CONFIGURING", "COMPLETING":
			status.Running++
		case "PENDING":
			status.Pending++
		case "COMPLETED":
			status.Completed++
		case "FAILED", "CANCELLED", "TIMEOUT", "OUT_OF_MEMORY", "NODE_FAIL", "PREEMPTED":
			status.Failed++
		}
	}

	return status, nil
}

// getArrayJobFinalStatus gets the final status using sacct
func (e *SlurmArrayEngine) getArrayJobFinalStatus(jobID string) (*ArrayJobStatus, error) {
	cmd := exec.Command("sacct", "-j", jobID, "-X", "-o", "State", "--noheader", "--parsable2")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get job status: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	lines := strings.Split(outputStr, "\n")

	status := &ArrayJobStatus{
		Completed: 0,
		Running:   0,
		Pending:   0,
		Failed:    0,
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "JobID" {
			continue
		}

		switch normalizeSlurmState(line) {
		case "COMPLETED":
			status.Completed++
		case "FAILED", "CANCELLED", "TIMEOUT", "OUT_OF_MEMORY", "NODE_FAIL", "PREEMPTED":
			status.Failed++
		}
	}

	return status, nil
}

// GetArraySize returns the size of the job array
func (e *SlurmArrayEngine) GetArraySize() int {
	return e.arraySize
}

// GetSamples returns the samples assigned to this array job
func (e *SlurmArrayEngine) GetSamples() []string {
	return e.samples
}
