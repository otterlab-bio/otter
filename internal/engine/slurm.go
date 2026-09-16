package engine

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/otterlab-bio/otter/internal/logger"
	taskruntime "github.com/otterlab-bio/otter/internal/task"
)

// NodeInfo 存储节点信息
type NodeInfo struct {
	Name            string
	State           string // idle/alloc/mixed/down
	TotalCores      int    // 总CPU核心数
	AvailableCores  int    // 可用CPU核心数
	TotalMemory     int    // 总内存 (MB)
	AvailableMemory int    // 可用内存 (MB)
}

// ValidateSlurmPartition checks if a SLURM partition exists and is accessible
func ValidateSlurmPartition(partition string) error {
	if partition == "" {
		return fmt.Errorf("partition name cannot be empty")
	}
	cmd := exec.Command("sinfo", "-h", "-p", partition)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("partition '%s' does not exist or is not accessible: %w", partition, err)
	}
	// sinfo returns empty output for non-existent partitions (but exit code 0)
	if len(strings.TrimSpace(string(output))) == 0 {
		return fmt.Errorf("partition '%s' does not exist", partition)
	}
	return nil
}

// ValidateSlurmNodeResources 检查分区中是否有节点满足资源需求
// 注意：sbatch是按空闲节点投递的，只需要检查是否有任意一个节点满足需求
func ValidateSlurmNodeResources(partition string, requestedCores int, requestedMemory string) error {
	// 1. 验证输入参数
	if requestedCores <= 0 {
		return fmt.Errorf("requested cores must be greater than 0, got %d", requestedCores)
	}

	// 2. 获取分区节点信息
	nodes, err := getPartitionNodes(partition)
	if err != nil {
		return fmt.Errorf("failed to get partition nodes: %w", err)
	}

	// 3. 解析请求的内存
	requestedMemMB, err := ParseMemory(requestedMemory)
	if err != nil {
		return fmt.Errorf("invalid memory format: %w", err)
	}

	// 4. 查找满足条件的节点
	suitableNodes := 0
	for _, node := range nodes {
		// 只考虑空闲或部分空闲的节点
		if node.State == "idle" || node.State == "mixed" {
			// 检查节点是否有足够资源
			if node.AvailableCores >= requestedCores && int64(node.AvailableMemory) >= requestedMemMB {
				suitableNodes++
				logger.Debugf("Found suitable node: %s - CPU: %d/%d, Memory: %dMB",
					node.Name, node.AvailableCores, node.TotalCores, node.AvailableMemory)
			}
		}
	}

	// 5. 验证结果
	if suitableNodes == 0 {
		return fmt.Errorf("no suitable nodes found in partition '%s' with %d cores and %dMB memory",
			partition, requestedCores, requestedMemMB)
	}

	logger.Infof("Found %d suitable nodes in partition '%s' for %d cores and %dMB",
		suitableNodes, partition, requestedCores, requestedMemMB)

	return nil
}

// getPartitionNodes 获取分区节点信息
func getPartitionNodes(partition string) ([]NodeInfo, error) {
	// 使用 -N 标志按节点列出
	// 格式: NODELIST,STATE,CPUS(A/I/O/T),MEMORY
	cmd := exec.Command("sinfo", "-N", "-h", "-p", partition,
		"-o", "%n,%T,%C,%m")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute sinfo: %w", err)
	}

	var nodes []NodeInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// sinfo output is comma-separated: NODELIST,STATE,CPUS(A/I/O/T),MEMORY
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}

		// 解析 CPU 信息 (A/I/O/T = Allocated/Idle/Other/Total)
		cpuParts := strings.Split(parts[2], "/")
		totalCores := 0
		availableCores := 0
		if len(cpuParts) >= 4 {
			totalCores, _ = strconv.Atoi(cpuParts[3]) // Total (第4个值)
			idle, _ := strconv.Atoi(cpuParts[1])      // Idle (第2个值，available)
			availableCores = idle
		}

		// 解析内存 (MB) - 跳过CPU parts，使用索引 3
		// 因为 parts[2] 是CPU，parts[3] 是内存
		totalMem := 0
		if len(parts) >= 4 {
			totalMem, _ = strconv.Atoi(parts[3])
		}

		// 检查节点状态
		state := parts[1]
		if state == "down" {
			continue // 跳过故障节点
		}

		node := NodeInfo{
			Name:            parts[0],
			State:           state,
			TotalCores:      totalCores,
			AvailableCores:  availableCores,
			TotalMemory:     totalMem,
			AvailableMemory: totalMem, // 简化：假设全部内存可用
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// SlurmEngine represents a Slurm cluster execution engine
type SlurmEngine struct {
	partition   string
	cores       int
	memory      string
	timeLimit   string
	jobName     string
	maxRetries  int
	waitTimeout time.Duration
	scriptPath  string
	successFile string
	status      *Status
	logDir      string
}

// SlurmConfig represents Slurm-specific configuration
type SlurmConfig struct {
	Partition   string        `json:"partition"`
	Cores       int           `json:"cores"`
	Memory      string        `json:"memory"`
	Time        string        `json:"time"`
	JobName     string        `json:"job_name"`
	MaxRetries  int           `json:"max_retries"`
	WaitTimeout time.Duration `json:"wait_timeout"` // 0 = no timeout (default)
}

// NewSlurmEngine creates a new Slurm engine
func NewSlurmEngine(config *SlurmConfig) *SlurmEngine {
	// Set default values if not provided
	partition := config.Partition
	if partition == "" {
		partition = "all" // Use "all" as default instead of hardcoded "cpu112c"
	}
	cores := config.Cores
	if cores <= 0 {
		cores = 4
	}
	memory := config.Memory
	if memory == "" {
		memory = "8G"
	}
	timeLimit := config.Time
	if timeLimit == "" {
		timeLimit = "24:00:00"
	}
	jobName := config.JobName
	if jobName == "" {
		jobName = "otter_job"
	}
	maxRetries := config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	return &SlurmEngine{
		partition:   partition,
		cores:       cores,
		memory:      memory,
		timeLimit:   timeLimit,
		jobName:     jobName,
		maxRetries:  maxRetries,
		waitTimeout: config.WaitTimeout,
		status: &Status{
			State:     StatusPending,
			StartTime: time.Now(),
		},
	}
}

// Execute executes a command via Slurm
func (e *SlurmEngine) Execute(cmd []string) error {
	e.status.State = StatusRunning
	e.status.Message = "Executing via Slurm"

	// Generate Slurm script
	scriptPath, err := e.generateSlurmScript(cmd)
	if err != nil {
		return fmt.Errorf("failed to generate Slurm script: %w", err)
	}
	e.scriptPath = scriptPath

	// Submit job
	jobID, err := e.submitJob(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to submit Slurm job: %w", err)
	}

	e.status.JobID = jobID
	logger.Infof("Slurm job submitted: %s", jobID)

	// Wait for completion
	if err := e.waitForCompletion(); err != nil {
		return fmt.Errorf("job execution failed: %w", err)
	}

	return nil
}

// ExecuteWithOutput executes a command and returns output
func (e *SlurmEngine) ExecuteWithOutput(cmd []string) (string, error) {
	// For Slurm engine, ExecuteWithOutput first submits the job and then
	// waits for completion while collecting output

	// Generate and submit the job
	scriptPath, err := e.generateSlurmScript(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to generate Slurm script: %w", err)
	}
	e.scriptPath = scriptPath

	// Submit job
	jobID, err := e.submitJob(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to submit Slurm job: %w", err)
	}

	e.status.JobID = jobID
	logger.Infof("Slurm job submitted: %s", jobID)

	// Wait for completion and collect output
	if err := e.waitForCompletion(); err != nil {
		return "", fmt.Errorf("job execution failed: %w", err)
	}

	// Collect job output using sacct
	return e.collectJobOutput(jobID)
}

// GetName returns the engine name
func (e *SlurmEngine) GetName() EngineType {
	return EngineSlurm
}

// GetStatus returns the current status
func (e *SlurmEngine) GetStatus() *Status {
	return e.status
}

// Wait waits for the job to complete
func (e *SlurmEngine) Wait() error {
	return e.waitForCompletion()
}

// getWorkDir returns the working directory for the job
func (e *SlurmEngine) getWorkDir() string {
	// Get current working directory
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	// Fallback to home directory
	return os.Getenv("HOME")
}

// Kill terminates the Slurm job
func (e *SlurmEngine) Kill() error {
	if e.status.JobID == "" {
		return fmt.Errorf("no job ID available")
	}

	cmd := exec.Command("scancel", e.status.JobID)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to cancel job: %w", err)
	}
	if err := taskruntime.CompleteCurrentSlurmJob(e.status.JobID); err != nil {
		logger.Warnf("Failed to update cancelled SLURM job %s: %v", e.status.JobID, err)
	}

	e.status.State = StatusKilled
	e.status.EndTime = time.Now()
	logger.Infof("Slurm job cancelled: %s", e.status.JobID)

	return nil
}

// SetLogDir sets the directory for SLURM log files
func (e *SlurmEngine) SetLogDir(dir string) error {
	e.logDir = dir
	return nil
}

// generateSlurmScript generates a Slurm batch script
func (e *SlurmEngine) generateSlurmScript(cmd []string) (string, error) {
	// Use log directory if set, otherwise fall back to temp directory
	tmpDir := e.logDir
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	scriptPath := filepath.Join(tmpDir, fmt.Sprintf("otter_%s.sh", e.jobName))

	// Slurm script template
	const scriptTemplate = `#!/bin/bash
#SBATCH --job-name={{.JobName}}
#SBATCH --partition={{.Partition}}
#SBATCH --cpus-per-task={{.Cores}}
#SBATCH --mem={{.Memory}}
#SBATCH --time={{.Time}}
#SBATCH --output={{.OutputFile}}
#SBATCH --error={{.ErrorFile}}

set -e

# Change to working directory
cd {{.WorkDir}}

# Execute command
{{range .Commands}}{{.}} {{end}}

# Touch success file
touch {{.SuccessFile}}
`

	slurmMemory, err := formatSlurmMemory(e.memory)
	if err != nil {
		return "", err
	}

	data := struct {
		JobName     string
		Partition   string
		Cores       int
		Memory      string
		Time        string
		WorkDir     string
		OutputFile  string
		ErrorFile   string
		SuccessFile string
		Commands    []string
	}{
		JobName:     e.jobName,
		Partition:   e.partition,
		Cores:       e.cores,
		Memory:      slurmMemory,
		Time:        e.timeLimit,
		WorkDir:     e.getWorkDir(),
		OutputFile:  filepath.Join(tmpDir, fmt.Sprintf("%s.out", e.jobName)),
		ErrorFile:   filepath.Join(tmpDir, fmt.Sprintf("%s.err", e.jobName)),
		SuccessFile: filepath.Join(tmpDir, fmt.Sprintf("%s.success", e.jobName)),
		Commands:    cmd,
	}

	// Record success file path for later job status checking
	e.successFile = data.SuccessFile

	// Remove any stale success file from a previous run before submitting a new job.
	// Without this, a failed job would appear successful because the old file still exists.
	_ = os.Remove(e.successFile)

	tmpl, err := template.New("slurm").Parse(scriptTemplate)
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

// submitJob submits a Slurm batch job
func (e *SlurmEngine) submitJob(scriptPath string) (string, error) {
	return e.submitSlurmScript(scriptPath, "job")
}

func (e *SlurmEngine) submitSlurmScript(scriptPath, submissionKind string) (string, error) {
	maximumAttempts := e.maxRetries + 1
	var lastSubmissionError error

	for attemptNumber := 1; attemptNumber <= maximumAttempts; attemptNumber++ {
		command := exec.Command("sbatch", scriptPath)
		var standardOutput, standardError bytes.Buffer
		command.Stdout = &standardOutput
		command.Stderr = &standardError

		if err := command.Run(); err == nil {
			jobID, parseErr := parseSlurmSubmissionID(standardOutput.String())
			if parseErr != nil {
				return "", parseErr
			}
			if err := taskruntime.RegisterCurrentSlurmJob(jobID); err != nil {
				logger.Warnf("Failed to persist SLURM job %s: %v", jobID, err)
			}
			return jobID, nil
		} else if standardError.Len() > 0 {
			lastSubmissionError = fmt.Errorf("sbatch command failed: %w\nstderr: %s", err, standardError.String())
		} else {
			lastSubmissionError = fmt.Errorf("sbatch command failed: %w\nstdout: %s", err, standardOutput.String())
		}

		if attemptNumber == maximumAttempts || !isRetriableSlurmSubmissionError(lastSubmissionError) {
			break
		}
		logger.Warnf("SLURM %s submission attempt %d/%d failed; retrying: %v", submissionKind, attemptNumber, maximumAttempts, lastSubmissionError)
		time.Sleep(time.Duration(attemptNumber) * time.Second)
	}

	return "", fmt.Errorf("submit SLURM %s after %d attempt(s): %w", submissionKind, maximumAttempts, lastSubmissionError)
}

func isRetriableSlurmSubmissionError(submissionError error) bool {
	message := strings.ToLower(submissionError.Error())
	for _, transientFragment := range []string{
		"unable to contact slurm controller",
		"connection refused",
		"connection timed out",
		"socket timed out",
		"temporarily unavailable",
		"resource temporarily unavailable",
	} {
		if strings.Contains(message, transientFragment) {
			return true
		}
	}
	return false
}

func parseSlurmSubmissionID(submissionOutput string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(submissionOutput))
	if len(fields) >= 4 && fields[0] == "Submitted" && fields[1] == "batch" && fields[2] == "job" {
		return fields[3], nil
	}
	return "", fmt.Errorf("failed to parse job ID from output: %s", strings.TrimSpace(submissionOutput))
}

// waitForCompletion waits for the Slurm job to complete.
// If e.waitTimeout > 0, the wait is bounded; if 0, waits indefinitely.
// The actual job walltime is controlled by SLURM's --time, not this function.
func (e *SlurmEngine) waitForCompletion() error {
	if e.status.JobID == "" {
		return fmt.Errorf("no job ID available")
	}
	defer func() {
		if err := taskruntime.CompleteCurrentSlurmJob(e.status.JobID); err != nil {
			logger.Warnf("Failed to update completed SLURM job %s: %v", e.status.JobID, err)
		}
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// If a wait timeout is configured, use it; otherwise wait indefinitely.
	var timeout <-chan time.Time
	if e.waitTimeout > 0 {
		timer := time.NewTimer(e.waitTimeout)
		defer timer.Stop()
		timeout = timer.C
	}

	for {
		select {
		case <-timeout:
			e.status.State = StatusFailed
			e.status.EndTime = time.Now()
			e.status.Message = "SLURM job timed out after " + e.waitTimeout.String()
			return fmt.Errorf("SLURM job %s timed out after %s", e.status.JobID, e.waitTimeout.String())
		case <-ticker.C:
			status, err := e.checkJobStatus(e.status.JobID)
			if err != nil {
				logger.Warnf("Failed to check job status: %v", err)
				continue
			}

			e.status.Progress = status.Progress
			e.status.Message = status.Message

			switch status.State {
			case StatusCompleted:
				e.status.State = StatusCompleted
				e.status.EndTime = time.Now()
				logger.Info("Slurm job completed successfully")
				return nil
			case StatusFailed:
				e.status.State = StatusFailed
				e.status.EndTime = time.Now()
				return fmt.Errorf("Slurm job failed")
			case StatusRunning:
				logger.Debugf("Slurm job running: %s", status.Message)
			}
		}
	}
}

// checkJobStatus checks the status of a Slurm job
func (e *SlurmEngine) checkJobStatus(jobID string) (*Status, error) {
	queueOutput, queueErr := exec.Command("squeue", "-j", jobID, "-o", "%T,%L", "--noheader").Output()
	queueText := strings.TrimSpace(string(queueOutput))
	if queueErr == nil && queueText != "" {
		parts := strings.SplitN(queueText, ",", 2)
		if len(parts) == 2 {
			switch normalizeSlurmState(parts[0]) {
			case "RUNNING", "PENDING", "CONFIGURING", "COMPLETING":
				return &Status{State: StatusRunning, Message: strings.TrimSpace(parts[1])}, nil
			case "FAILED", "CANCELLED", "TIMEOUT", "OUT_OF_MEMORY", "NODE_FAIL", "PREEMPTED":
				return &Status{State: StatusFailed, Message: queueText}, nil
			case "COMPLETED":
				return &Status{State: StatusCompleted, Message: queueText}, nil
			}
		}
	}

	// A job can disappear from squeue, or squeue can temporarily fail while
	// the job is still running. Accounting is authoritative for terminal state.
	accountingState, accountingErr := querySlurmAccountingState(jobID)
	if accountingErr == nil {
		switch accountingState {
		case "COMPLETED":
			return &Status{State: StatusCompleted, Message: "Job completed according to Slurm accounting"}, nil
		case "FAILED", "CANCELLED", "TIMEOUT", "OUT_OF_MEMORY", "NODE_FAIL", "PREEMPTED", "BOOT_FAIL", "DEADLINE", "REVOKED":
			return &Status{State: StatusFailed, Message: "Job terminated with Slurm state " + accountingState}, nil
		case "RUNNING", "PENDING", "CONFIGURING", "COMPLETING":
			return &Status{State: StatusRunning, Message: "Job state from Slurm accounting: " + accountingState}, nil
		}
	}

	// The marker is only a positive completion signal. Its absence is not
	// evidence of failure when both Slurm control-plane queries are transiently unavailable.
	if e.successFile != "" {
		if _, statErr := os.Stat(e.successFile); statErr == nil {
			return &Status{State: StatusCompleted, Message: "Job completed (success marker)"}, nil
		}
	}
	if queueErr != nil && accountingErr != nil {
		return &Status{State: StatusRunning, Message: "Slurm status temporarily unavailable; retrying"}, nil
	}
	return &Status{State: StatusRunning, Message: "Waiting for Slurm accounting state"}, nil
}

func querySlurmAccountingState(jobID string) (string, error) {
	output, err := exec.Command("sacct", "-j", jobID, "-X", "-o", "State", "--noheader", "--parsable2").Output()
	if err != nil {
		return "", err
	}
	return normalizeSlurmState(string(output)), nil
}

func normalizeSlurmState(rawState string) string {
	stateFields := strings.Fields(strings.TrimSpace(rawState))
	if len(stateFields) == 0 {
		return ""
	}
	return strings.TrimSuffix(stateFields[0], "+")
}

// collectJobOutput collects the output of a completed Slurm job
func (e *SlurmEngine) collectJobOutput(jobID string) (string, error) {
	// Use sacct to get job output
	cmd := exec.Command("sacct", "-j", jobID, "-X", "-o", "JobID,JobName,State,ExitCode", "--parsable2")
	output, err := cmd.Output()
	if err != nil {
		logger.Warnf("Failed to collect job output: %v", err)
		return "", fmt.Errorf("failed to collect job output: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	logger.Infof("Job output collected: %s", outputStr)

	return outputStr, nil
}

// SlurmUserLimits 存储 SLURM 用户限制信息
type SlurmUserLimits struct {
	MaxSubmitJobs   int // 最大提交任务数
	CurrentJobCount int // 当前任务数
	AvailableJobs   int // 可用任务数
	QOSMaxJobs      int // QoS 最大任务数
	AccountMaxJobs  int // 账户最大任务数
}

// GetSlurmUserLimits 获取用户的 SLURM 任务提交限制
// 通过多个命令尝试获取限制信息，如果无法获取则返回安全的默认值
func GetSlurmUserLimits() *SlurmUserLimits {
	limits := &SlurmUserLimits{
		MaxSubmitJobs:   100, // 默认值
		CurrentJobCount: 0,
		AvailableJobs:   100,
	}

	username := os.Getenv("USER")
	if username == "" {
		logger.Debugf("Cannot get username for SLURM limit detection")
		return limits
	}

	// 1. 获取当前任务数
	limits.CurrentJobCount = getCurrentJobCount(username)

	// 2. 尝试从 sacctmgr 获取 MaxSubmitJobs 限制
	if maxJobs := getAssocMaxJobs(username); maxJobs > 0 {
		limits.MaxSubmitJobs = maxJobs
		logger.Debugf("Detected SLURM MaxSubmitJobs limit: %d", maxJobs)
	}

	// 3. 尝试从 scontrol show assoc 获取限制
	if maxJobs := getControlMaxJobs(username); maxJobs > 0 {
		if limits.MaxSubmitJobs == 100 || maxJobs < limits.MaxSubmitJobs {
			limits.MaxSubmitJobs = maxJobs
		}
		logger.Debugf("Detected SLURM association limit: %d", maxJobs)
	}

	// 4. 计算可用任务数
	limits.AvailableJobs = limits.MaxSubmitJobs - limits.CurrentJobCount
	if limits.AvailableJobs < 0 {
		limits.AvailableJobs = 0
	}

	logger.Infof("SLURM limits: MaxSubmit=%d, Current=%d, Available=%d",
		limits.MaxSubmitJobs, limits.CurrentJobCount, limits.AvailableJobs)

	return limits
}

// getCurrentJobCount 获取用户当前运行中的任务数
func getCurrentJobCount(username string) int {
	cmd := exec.Command("squeue", "-u", username, "-h", "-o", "%i")
	output, err := cmd.Output()
	if err != nil {
		logger.Debugf("Failed to get current job count: %v", err)
		return 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// getAssocMaxJobs 从 sacctmgr 获取最大任务数限制
func getAssocMaxJobs(username string) int {
	cmd := exec.Command("sacctmgr", "show", "assoc", "where", fmt.Sprintf("user=%s", username),
		"-s", "-n", "-o", "MaxSubmitJobs")
	output, err := cmd.Output()
	if err != nil {
		logger.Debugf("sacctmgr command failed: %v", err)
		return 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "MaxSubmitJobs" {
			continue
		}
		// 尝试解析数字
		if val, err := strconv.Atoi(line); err == nil && val > 0 {
			return val
		}
	}
	return 0
}

// getControlMaxJobs 从 scontrol 获取最大任务数限制
func getControlMaxJobs(username string) int {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("scontrol show assoc | grep -A 30 'UserName=%s(' | grep 'MaxSubmitJobs=' | head -1 || true", username))
	output, err := cmd.Output()
	if err != nil {
		logger.Debugf("scontrol show assoc failed: %v", err)
		return 0
	}

	outputStr := strings.TrimSpace(string(output))
	logger.Debugf("scontrol MaxSubmitJobs output: [%s]", outputStr)

	// 查找 MaxSubmitJobs=20(0) 格式，可能前面有其他字段
	if idx := strings.Index(outputStr, "MaxSubmitJobs="); idx >= 0 {
		maxStr := outputStr[idx+len("MaxSubmitJobs="):] // 跳过 "MaxSubmitJobs="（14字符）
		// 去掉括号部分，例如 "20(0)" -> "20"
		if parenIdx := strings.Index(maxStr, "("); parenIdx > 0 {
			maxStr = maxStr[:parenIdx]
		}
		if endIdx := strings.IndexAny(maxStr, " \t"); endIdx > 0 {
			maxStr = maxStr[:endIdx]
		}
		// 处理 "N(0)" 格式，N 表示无限制
		if maxStr == "N" || maxStr == "" {
			return -1 // -1 表示无限制
		}
		if val, err := strconv.Atoi(maxStr); err == nil && val > 0 {
			logger.Debugf("Found MaxSubmitJobs=%s from scontrol", maxStr)
			return val
		}
	}
	return 0
}

// GetJobState returns the SLURM state string for a single job (non-blocking).
// Returns one of: "RUNNING", "PENDING", "COMPLETED", "FAILED", "CANCELLED", "TIMEOUT", "UNKNOWN".
func GetJobState(jobID string) string {
	// 1. 先查 squeue（job 仍在队列中）
	out, err := exec.Command("squeue", "-j", jobID, "-o", "%T", "--noheader").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if l := strings.TrimSpace(line); l != "" {
				return l
			}
		}
	}
	// 2. squeue 无结果 → job 已离队，查 sacct
	out, err = exec.Command("sacct", "-j", jobID, "-X", "-o", "State",
		"--noheader", "--parsable2").Output()
	if err != nil {
		return "UNKNOWN"
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l := strings.TrimSpace(line); l != "" {
			// sacct state 可能有 "CANCELLED by 1234" 后缀，取第一个词
			return strings.Fields(l)[0]
		}
	}
	return "UNKNOWN"
}
