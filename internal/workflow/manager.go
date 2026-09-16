package workflow

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/otterlab-bio/otter/internal/config"
	"github.com/otterlab-bio/otter/internal/engine"
	"github.com/otterlab-bio/otter/internal/logger"
	taskruntime "github.com/otterlab-bio/otter/internal/task"
)

// Manager manages workflow execution
type Manager struct {
	workflow      *Workflow
	state         *State
	dryRun        bool
	condaEnv      string
	fallbackEnv   string // Fallback environment (default: otter-snakemake)
	noFallback    bool   // Disable automatic fallback
	samples       []string
	stepResources map[int]*config.StepResource
	parallelJobs  int
	loadRatio     float64 // > 0: dynamic pool mode; 0: legacy batch mode

	cacheReady    bool
	pdxMode       bool
	workflowName  string
	configFileAbs string
}

// NewManager creates a new workflow manager
func NewManager(w *Workflow) *Manager {
	return &Manager{
		workflow:    w,
		dryRun:      false,
		condaEnv:    "",
		fallbackEnv: DefaultFallbackEnv,
		noFallback:  false,
	}
}

// SetDryRun sets the dry-run mode for the workflow
func (m *Manager) SetDryRun(dryRun bool) {
	m.dryRun = dryRun
}

// SetCondaEnv sets the conda environment for Snakemake execution
func (m *Manager) SetCondaEnv(condaEnv string) {
	m.condaEnv = condaEnv
}

// SetFallbackConfig sets the fallback environment configuration
func (m *Manager) SetFallbackConfig(fallbackEnv string, noFallback bool) {
	m.fallbackEnv = fallbackEnv
	m.noFallback = noFallback
}

// SetSamples sets the sample names for the workflow
func (m *Manager) SetSamples(samples []string) {
	m.samples = samples
}

// SetStepResources sets the step resources for the workflow
func (m *Manager) SetStepResources(resources map[int]*config.StepResource) {
	m.stepResources = resources
}

// SetParallelJobs sets the number of parallel jobs for both local and SLURM execution
func (m *Manager) SetParallelJobs(jobs int) {
	m.parallelJobs = jobs
}

// SetLoadRatio sets the dynamic pool load ratio for SLURM execution
func (m *Manager) SetLoadRatio(r float64) {
	m.loadRatio = r
}

func (m *Manager) SetConfigFile(path string) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve workflow config file %q: %w", path, err)
	}
	m.configFileAbs = filepath.Clean(absolutePath)
	return nil
}

func (m *Manager) ensureRuntimeCache() {
	if m.cacheReady {
		return
	}

	m.pdxMode = config.DetectPDXMode(m.workflow.Config)
	m.workflowName = config.GetWorkflowName(m.workflow.Config.Workflow.Mode, m.pdxMode)
	m.cacheReady = true
}

// SetJobID updates the job ID for a specific step in the state
func (m *Manager) SetJobID(step int, jobID string) error {
	if m.state == nil {
		return nil // State tracking not enabled
	}
	if err := m.state.UpdateStepStatus(step, "running", time.Now(), jobID); err != nil {
		return fmt.Errorf("failed to update job ID: %w", err)
	}
	return m.state.Save()
}

// getStepResource returns the resource configuration for a specific step
func (m *Manager) getStepResource(step int) *config.StepResource {
	m.ensureRuntimeCache()

	if resource, exists := m.stepResources[step]; exists {
		return resource
	}

	// Checker steps inherit from main steps if not explicitly configured
	if step == 102 { // Step 2 Checker
		if resource, exists := m.stepResources[2]; exists {
			// Create a copy to avoid modifying the original
			inherited := *resource
			return &inherited
		}
	} else if step == 103 { // Step 3 Checker
		if resource, exists := m.stepResources[3]; exists {
			// Create a copy to avoid modifying the original
			inherited := *resource
			return &inherited
		}
	}

	// Return default resource
	return config.GetDefaultStepResource(step, m.workflow.Config.Workflow.Mode, m.pdxMode)
}

// shouldUseSingleSampleMode returns true if the step should use single-sample mode
// Step 2 and 3 use single-sample mode, Step 1 and checkers use all-samples mode
func (m *Manager) shouldUseSingleSampleMode(step int) bool {
	// Step 2 and 3 use single-sample mode
	return step == 2 || step == 3
}

// Initialize initializes the workflow
func (m *Manager) Initialize() error {
	logger.Info("Initializing workflow...")

	// Create output directory
	if err := os.MkdirAll(m.workflow.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create logs directory
	logsDir := filepath.Join(m.workflow.OutputDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Initialize file logging
	logFilePath := filepath.Join(logsDir, "otter.log")
	logger.InitWithFile(false, logFilePath)
	logger.Infof("Logging to file: %s", logFilePath)

	// Set log directory on engine if it supports it
	if m.workflow.Engine != nil {
		if err := m.workflow.Engine.SetLogDir(logsDir); err != nil {
			logger.Warnf("Failed to set engine log directory: %v", err)
		}
	}

	// Detect workflow mode and steps
	m.workflow.Status.State = engine.StatusRunning
	m.workflow.Status.Message = "Initializing workflow"

	// Determine steps based on mode
	m.ensureRuntimeCache()
	m.workflow.Steps = config.GetStepCount(m.workflow.Config.Workflow.Mode, m.pdxMode)
	logger.Infof("Workflow initialized with %d steps", m.workflow.Steps)

	if err := taskruntime.UpdateCurrent(func(record *taskruntime.Record) error {
		record.CurrentStep = 0
		record.Message = "initializing workflow"
		return nil
	}); err != nil {
		logger.Warnf("Failed to persist task initialization state: %v", err)
	}

	return nil
}

// ExecuteStep executes a specific step with intelligent parallelization strategy
func (m *Manager) ExecuteStep(step int) error {
	isChecker := step == 102 || step == 103
	if !isChecker && (step < 1 || step > m.workflow.Steps) {
		return fmt.Errorf("invalid step: %d (valid range: 1-%d)", step, m.workflow.Steps)
	}

	if isChecker {
		logger.Infof("Executing checker for step %d", step)
	} else {
		logger.Infof("Executing step %d/%d", step, m.workflow.Steps)
	}
	_ = taskruntime.UpdateCurrent(func(record *taskruntime.Record) error {
		record.CurrentStep = step
		if isChecker {
			record.Message = fmt.Sprintf("running checker step %d", step)
		} else {
			record.Message = fmt.Sprintf("running workflow step %d of %d", step, m.workflow.Steps)
		}
		return nil
	})

	stepSucceeded := false
	defer func() {
		if stepSucceeded || isChecker || m.state == nil {
			return
		}
		_ = m.state.UpdateStepStatus(step, "failed", time.Now(), "")
		_ = m.state.Save()
	}()

	// Update state: step started
	if m.state != nil {
		if err := m.state.UpdateStepStatus(step, "running", time.Now(), ""); err != nil {
			if isChecker {
				logger.Debugf("Checker step %d state update skipped: %v", step, err)
			} else {
				logger.Warnf("Failed to update step status: %v", err)
			}
		}
		_ = m.state.Save()
	}

	// Get step resource configuration
	stepResource := m.getStepResource(step)

	// Unified parallelization control via --parallel-jobs parameter
	// For step2 and step3: ensure JobArray is enabled when parallel-jobs > 1 or load-ratio > 0
	useParallel := m.parallelJobs > 1 || m.loadRatio > 0
	if m.shouldUseSingleSampleMode(step) && useParallel {
		// Force JobArray mode for single-sample steps when using parallel execution
		if !stepResource.JobArray {
			stepResource.JobArray = true
		}
	}

	// Set MaxJobs from parallelJobs for unified control
	// This ensures job-array and local parallel execution use the same value
	if m.parallelJobs > 0 {
		stepResource.MaxJobs = m.parallelJobs
	}

	// Determine workflow index
	m.ensureRuntimeCache()
	workflowIdx := m.workflowName

	// Create executor
	executor := NewSnakemakeExecutor(workflowIdx, step, m.getConfigFile(), &WorkflowOptions{
		DryRun: m.dryRun,
	}, m.condaEnv)

	// Set fallback configuration from manager or config
	if m.fallbackEnv != "" {
		executor.SetFallbackConfig(m.fallbackEnv, m.noFallback)
	} else if m.workflow.Config.Engine.FallbackEnv != "" {
		executor.SetFallbackConfig(m.workflow.Config.Engine.FallbackEnv, m.workflow.Config.Engine.NoFallback)
	}

	// Validate environment and apply fallback if needed
	if err := executor.ValidateAndFallback(); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}

	// Build command (may use fallback environment)
	cmd := executor.BuildCommand()

	// 提取 executor 中已解析的 snakefile 路径和 configfile 路径
	resolvedSnakefile := executor.GetResolvedSnakefilePath()
	configFile := executor.ConfigFile

	// Dry runs validate the environment and command construction without submitting compute work.
	if m.dryRun {
		logger.Infof("Dry run: validated step %d resources (cores=%d, memory=%s, time=%s, partition=%s)", step, stepResource.Cores, stepResource.Memory, stepResource.Time, stepResource.Partition)
		stepSucceeded = true
		return nil
	}

	// Unified parallelization strategy based on --parallel-jobs / --load-ratio parameters
	// Step 2 and 3 use single-sample mode, Step 1 and checkers use all-samples mode
	if m.shouldUseSingleSampleMode(step) {
		// Step 2 and 3: Always use single-sample mode
		// Parallelization is controlled by --parallel-jobs or --load-ratio parameter
		if useParallel {
			// Use parallel execution
			logger.Infof("Using parallel single-sample strategy for step %d (parallel-jobs=%d, load-ratio=%.1f)", step, m.parallelJobs, m.loadRatio)

			switch m.workflow.Engine.(type) {
			case *engine.SlurmArrayEngine:
				// Use Job Array for SLURM environments
				logger.Infof("Using Job Array strategy for step %d with max %d concurrent jobs", step, m.parallelJobs)
				if err := m.executeStepWithJobArray(step, stepResource, cmd, resolvedSnakefile, configFile); err != nil {
					return fmt.Errorf("step %d Job Array execution failed: %w", step, err)
				}

			case *engine.LocalEngine:
				// Use local parallel execution for local environments
				logger.Infof("Using local parallel strategy for step %d with %d workers", step, m.parallelJobs)
				if err := m.executeStepWithLocalParallel(step, stepResource, cmd, resolvedSnakefile, configFile); err != nil {
					return fmt.Errorf("step %d parallel execution failed: %w", step, err)
				}

			default:
				// Other engines: check if we should auto-switch to SlurmArrayEngine
				if _, ok := m.workflow.Engine.(*engine.SlurmEngine); ok {
					logger.Infof("Auto-switching from SlurmEngine to SlurmArrayEngine for step %d", step)
					// Ensure stepResource has default values if cores/memory are not set
					if stepResource.Cores == 0 || stepResource.Memory == "" {
						defaultResource := config.GetDefaultStepResource(step, m.workflow.Config.Workflow.Mode, m.pdxMode)
						if stepResource.Cores == 0 {
							stepResource.Cores = defaultResource.Cores
						}
						if stepResource.Memory == "" {
							stepResource.Memory = defaultResource.Memory
						}
						if stepResource.Partition == "" && defaultResource.Partition != "" {
							stepResource.Partition = defaultResource.Partition
						}
					}
					logger.Infof("stepResource before createSlurmArrayEngine: Cores=%d, Memory=%s", stepResource.Cores, stepResource.Memory)
					arrayEngine, err := m.createSlurmArrayEngine(stepResource)
					if err != nil {
						return fmt.Errorf("failed to create SlurmArrayEngine: %w", err)
					}
					m.workflow.SetEngine(arrayEngine)
					// Now execute with Job Array strategy
					logger.Infof("Using Job Array strategy for step %d with max %d concurrent jobs", step, m.parallelJobs)
					if err := m.executeStepWithJobArray(step, stepResource, cmd, resolvedSnakefile, configFile); err != nil {
						return fmt.Errorf("step %d Job Array execution failed: %w", step, err)
					}
				} else {
					// Use standard execution for other engine types
					logger.Infof("Using standard execution for step %d", step)
					if err := m.workflow.Engine.Execute(cmd); err != nil {
						return fmt.Errorf("step %d execution failed: %w", step, err)
					}
				}
			}
		} else {
			// Sequential execution (parallel-jobs = 1)
			logger.Infof("Using single-sample sequential execution for step %d", step)
			if err := m.workflow.Engine.Execute(cmd); err != nil {
				return fmt.Errorf("step %d execution failed: %w", step, err)
			}
		}
	} else {
		// Step 1 and checkers: use all-samples execution (standard mode)
		logger.Infof("Using all-samples standard execution for step %d", step)
		if err := m.workflow.Engine.Execute(cmd); err != nil {
			return fmt.Errorf("step %d execution failed: %w", step, err)
		}
	}

	logger.Infof("Step %d completed successfully", step)

	// Get Job ID from engine (for SLURM jobs)
	jobID := ""
	if engineStatus := m.workflow.Engine.GetStatus(); engineStatus != nil && engineStatus.JobID != "" {
		jobID = engineStatus.JobID
	}

	// Update state: step completed
	if m.state != nil {
		if err := m.state.UpdateStepStatus(step, "completed", time.Now(), jobID); err != nil {
			if isChecker {
				logger.Debugf("Checker step %d state update skipped: %v", step, err)
			} else {
				logger.Warnf("Failed to update step status: %v", err)
			}
		}
		_ = m.state.Save()
	}

	stepSucceeded = true
	_ = taskruntime.UpdateCurrent(func(record *taskruntime.Record) error {
		record.Message = fmt.Sprintf("completed step %d", step)
		return nil
	})
	return nil
}

// executeStepWithJobArray executes a step using SLURM Job Array
func (m *Manager) executeStepWithJobArray(step int, resource *config.StepResource, cmd []string, workflowFile string, configFile string) error {
	// Check if engine is SlurmArrayEngine
	if arrayEngine, ok := m.workflow.Engine.(*engine.SlurmArrayEngine); ok {
		if err := arrayEngine.ExecuteStepWithArray(step, m.condaEnv, workflowFile, configFile, resource); err != nil {
			return err
		}

		// Note: Job ID will be captured in ExecuteStep after this returns
		// The engine's GetStatus() will contain the submitted Job ID
		return nil
	}

	// Fallback to regular engine if SlurmArrayEngine not available
	logger.Warn("SlurmArrayEngine not available, falling back to regular execution")
	return m.workflow.Engine.Execute(cmd)
}

// executeStepWithLocalParallel executes a step using local parallel execution
func (m *Manager) executeStepWithLocalParallel(step int, resource *config.StepResource, cmd []string, workflowFile string, configFile string) error {
	// Check if engine supports parallel execution
	if localEngine, ok := m.workflow.Engine.(*engine.LocalEngine); ok {
		// Compute slot limit: floor(parallelJobs × loadRatio), minimum 1
		slotLimit := m.parallelJobs
		if m.loadRatio > 0 && m.parallelJobs > 0 {
			slotLimit = int(math.Floor(float64(m.parallelJobs) * m.loadRatio))
			if slotLimit < 1 {
				slotLimit = 1
			}
			logger.Infof("Local dynamic pool: slot_limit=%d (parallel-jobs=%d × %.2f)", slotLimit, m.parallelJobs, m.loadRatio)
		}
		return localEngine.ExecuteSamples(step, m.samples, m.condaEnv, workflowFile, configFile, slotLimit)
	}

	// Fallback to regular engine
	logger.Warn("LocalEngine parallel execution not available, falling back to regular execution")
	return m.workflow.Engine.Execute(cmd)
}

// ExecuteSelectedSteps executes an explicit ordered subset of workflow steps.
// Selected execution is used by bounded compatibility canaries; callers must
// provide only the steps they intend to run and any required predecessors.
func (m *Manager) ExecuteSelectedSteps(steps []int) error {
	if len(steps) == 0 {
		return fmt.Errorf("at least one workflow step must be selected")
	}
	if err := m.Initialize(); err != nil {
		return err
	}

	m.state = NewState(m.workflow.Config.Output.BaseDir, m.workflow.Config.Workflow.JobID)
	if err := m.initializeState(); err != nil {
		logger.Warnf("Failed to initialize state: %v (continuing without state tracking)", err)
	}

	for _, step := range steps {
		if err := m.ExecuteStep(step); err != nil {
			if m.state != nil {
				_ = m.state.MarkFailed()
			}
			return fmt.Errorf("failed to execute selected step %d: %w", step, err)
		}
	}

	m.workflow.Status.State = engine.StatusCompleted
	m.workflow.Status.Message = "Selected workflow steps completed"
	m.workflow.Status.EndTime = time.Now()
	if m.state != nil {
		_ = m.state.MarkCompleted()
	}
	logger.Info("Selected workflow steps completed successfully")
	return nil
}

// ExecuteAll executes all workflow steps
func (m *Manager) ExecuteAll() error {
	if err := m.Initialize(); err != nil {
		return err
	}

	// Initialize state
	m.state = NewState(m.workflow.Config.Output.BaseDir, m.workflow.Config.Workflow.JobID)

	// Check for resume mode
	if m.workflow.Options != nil && m.workflow.Options.Resume && m.state.Exists() {
		if err := m.state.Load(); err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		// Validate config consistency
		if err := m.validateConfigForResume(); err != nil {
			return fmt.Errorf("config validation failed: %w", err)
		}

		// Find last completed step
		lastCompleted := m.state.GetLastCompletedStep()
		logger.Infof("Resuming from step %d (last completed: %d)",
			lastCompleted+1, lastCompleted)

		checkerOf := map[int]int{2: 102, 3: 103}
		m.ensureRuntimeCache()
		workflowIdx := m.workflowName

		// Start from next step, run checker after each main step
		for step := lastCompleted + 1; step <= m.workflow.Steps; step++ {
			if err := m.ExecuteStep(step); err != nil {
				// Mark workflow as failed
				_ = m.state.MarkFailed()
				return fmt.Errorf("failed to execute step %d: %w", step, err)
			}
			if err := m.runCheckerIfExists(checkerOf, workflowIdx, step); err != nil {
				_ = m.state.MarkFailed()
				return fmt.Errorf("failed to execute checker for step %d: %w", step, err)
			}
		}

		// Catch-up: run checkers for already-completed main steps (handles lastCompleted == m.workflow.Steps)
		// Snakemake idempotency ensures no double execution if outputs already exist
		for step := 1; step <= lastCompleted; step++ {
			if err := m.runCheckerIfExists(checkerOf, workflowIdx, step); err != nil {
				_ = m.state.MarkFailed()
				return fmt.Errorf("failed to execute checker for step %d: %w", step, err)
			}
		}
	} else {
		// Initialize new state
		if err := m.initializeState(); err != nil {
			logger.Warnf("Failed to initialize state: %v (continuing without state tracking)", err)
		}

		// Normal execution: run all steps
		checkerOf := map[int]int{2: 102, 3: 103}
		m.ensureRuntimeCache()
		workflowIdx := m.workflowName

		for step := 1; step <= m.workflow.Steps; step++ {
			if err := m.ExecuteStep(step); err != nil {
				// Mark workflow as failed
				if m.state != nil {
					_ = m.state.MarkFailed()
				}
				return fmt.Errorf("failed to execute step %d: %w", step, err)
			}
			if err := m.runCheckerIfExists(checkerOf, workflowIdx, step); err != nil {
				if m.state != nil {
					_ = m.state.MarkFailed()
				}
				return fmt.Errorf("failed to execute checker for step %d: %w", step, err)
			}
		}
	}

	m.workflow.Status.State = engine.StatusCompleted
	m.workflow.Status.Message = "Workflow completed"
	m.workflow.Status.EndTime = time.Now()

	// Mark state as completed
	if m.state != nil {
		_ = m.state.MarkCompleted()
	}

	logger.Info("All workflow steps completed successfully")

	return nil
}

// initializeState initializes the state with current workflow configuration
func (m *Manager) initializeState() error {
	if m.state == nil {
		return nil
	}

	// Get partition for state tracking
	partition := ""
	if m.workflow.Config.Engine.Slurm.Partition != "" {
		partition = m.workflow.Config.Engine.Slurm.Partition
	}

	return m.state.Initialize(
		m.workflow.Config.Workflow.JobID,
		m.workflow.Config.Workflow.Mode,
		m.workflow.Config.Workflow.Species.Primary,
		m.workflow.Config.Workflow.Species.Secondary,
		m.workflow.Steps,
		m.samples,
		m.workflow.Config.Engine.Type,
		partition,
	)
}

// validateConfigForResume checks if current config matches saved state
func (m *Manager) validateConfigForResume() error {
	stateData := m.state.GetData()

	// Check critical config parameters
	if stateData.Config.WorkflowMode != m.workflow.Config.Workflow.Mode {
		return fmt.Errorf("workflow mode changed: was %s, now %s",
			stateData.Config.WorkflowMode, m.workflow.Config.Workflow.Mode)
	}

	if stateData.Config.SampleCount != len(m.samples) {
		return fmt.Errorf("sample count changed: was %d, now %d",
			stateData.Config.SampleCount, len(m.samples))
	}

	// Check species consistency
	if stateData.Config.Species1 != m.workflow.Config.Workflow.Species.Primary {
		return fmt.Errorf("species changed: was %s, now %s",
			stateData.Config.Species1, m.workflow.Config.Workflow.Species.Primary)
	}

	// Warn on non-critical changes
	if stateData.Config.Partition != "" && m.workflow.Config.Engine.Slurm.Partition != "" {
		if stateData.Config.Partition != m.workflow.Config.Engine.Slurm.Partition {
			logger.Warnf("Partition changed: %s -> %s",
				stateData.Config.Partition, m.workflow.Config.Engine.Slurm.Partition)
		}
	}

	return nil
}

// CreateFilework creates the directory structure for the workflow
func (m *Manager) CreateFilework() error {
	logger.Info("Creating workflow directory structure...")

	dirs := []string{
		m.workflow.Config.Output.WorkflowDir,
		m.workflow.Config.Output.AnalysisDir,
		m.workflow.Config.Directories.QC.Main,
		m.workflow.Config.Output.TrimDir,
		m.workflow.Config.Directories.BSMAP.Main,
		m.workflow.Config.Directories.MethylationCall,
		m.workflow.Config.Directories.UMX,
		m.workflow.Config.Directories.SIDLog,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	logger.Info("Directory structure created successfully")

	return nil
}

// MoveFastq moves FASTQ files to workflow directory
func (m *Manager) MoveFastq(samples []string, method string) error {
	logger.Infof("Moving FASTQ files using method: %s", method)

	// This would copy/move FASTQ files from input directory to workflow directory
	// Implementation depends on specific requirements

	logger.Info("FASTQ files moved successfully")

	return nil
}

// AggregateResults aggregates workflow results
func (m *Manager) AggregateResults() error {
	logger.Info("Aggregating workflow results...")

	// This would aggregate results from different steps
	// Implementation depends on specific requirements

	logger.Info("Results aggregated successfully")

	return nil
}

// GetStatus returns the current workflow status
func (m *Manager) GetStatus() *engine.Status {
	return m.workflow.Status
}

// runCheckerIfExists runs the checker step for the given main step (if the checker file exists).
// Uses Snakemake idempotency: if outputs already exist, snakemake exits immediately.
func (m *Manager) runCheckerIfExists(checkerOf map[int]int, workflowIdx string, step int) error {
	checkerStep, ok := checkerOf[step]
	if !ok {
		return nil
	}
	checkerFile := snakefileForStep(workflowIdx, checkerStep)
	exists := false
	for _, c := range []string{checkerFile, filepath.Join("otter-project", checkerFile)} {
		if _, err := os.Stat(c); err == nil {
			exists = true
			break
		}
	}
	if !exists {
		logger.Debugf("No checker found for step %d, skipping", step)
		return nil
	}
	logger.Infof("Running checker for step %d (step %d)", step, checkerStep)
	return m.ExecuteStep(checkerStep)
}

// getConfigFile returns the path to the Snakemake config file
func (m *Manager) getConfigFile() string {
	if m.configFileAbs != "" {
		return m.configFileAbs
	}

	configPath := filepath.Join(m.workflow.Config.Directories.Config, "config.yaml")

	// Convert to absolute path to ensure Snakemake can find it
	if !filepath.IsAbs(configPath) {
		// Get current working directory
		if wd, err := os.Getwd(); err == nil {
			configPath = filepath.Join(wd, configPath)
		}
	}

	m.configFileAbs = configPath
	return m.configFileAbs
}

// createSlurmArrayEngine creates a SlurmArrayEngine with current configuration
// This is used when auto-switching from SlurmEngine to SlurmArrayEngine
func (m *Manager) createSlurmArrayEngine(stepResource *config.StepResource) (*engine.SlurmArrayEngine, error) {
	factory := &engine.EngineFactory{}

	// Get partition from step resources or config
	partition := m.getPartition()

	// Update engine config with partition from step resources
	m.workflow.Config.Engine.Slurm.Partition = partition

	// 检测 SLURM 提交限制，计算每批最大 Task 数
	maxBatchSize := 0
	limits := engine.GetSlurmUserLimits()
	if limits.MaxSubmitJobs > 1 {
		maxBatchSize = limits.MaxSubmitJobs - 1 // 留 1 个给 parent Job
		logger.Infof("Batch size set to %d (MaxSubmitJobs=%d)", maxBatchSize, limits.MaxSubmitJobs)
	}

	arrayEngine, err := factory.NewSlurmArrayEngineWithResources(
		&m.workflow.Config.Engine,
		m.samples,
		stepResource,
		maxBatchSize,
		m.loadRatio,
	)
	if err != nil {
		return nil, err
	}
	if err := arrayEngine.SetLogDir(m.workflow.Config.Output.LogDir); err != nil {
		return nil, fmt.Errorf("set Slurm array log directory: %w", err)
	}
	return arrayEngine, nil
}

// getPartition returns the partition to use for SLURM jobs
// It checks step resources first, then falls back to config
func (m *Manager) getPartition() string {
	// Try to get partition from step resources first
	if m.stepResources != nil {
		for _, res := range m.stepResources {
			if res != nil && res.Partition != "" {
				return res.Partition
			}
		}
	}
	// Fallback to config
	return m.workflow.Config.Engine.Slurm.Partition
}
