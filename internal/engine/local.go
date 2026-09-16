package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/otterlab-bio/otter/internal/enva"
	"github.com/otterlab-bio/otter/internal/logger"
)

// LocalEngine represents a local execution engine
type LocalEngine struct {
	maxCores    int
	maxMemory   string
	maxParallel int
	cmd         *exec.Cmd
	ctx         context.Context
	cancel      context.CancelFunc
	status      *Status
}

// LocalConfig represents local execution configuration
type LocalConfig struct {
	MaxCores  int    `json:"max_cores"`
	MaxMemory string `json:"max_memory"`
}

// NewLocalEngine creates a new local engine
func NewLocalEngine(config *LocalConfig) *LocalEngine {
	maxParallel := config.MaxCores
	if maxParallel <= 0 {
		maxParallel = 4 // Default to 4 parallel jobs
	}

	return &LocalEngine{
		maxCores:    config.MaxCores,
		maxMemory:   config.MaxMemory,
		maxParallel: maxParallel,
		status: &Status{
			State:     StatusPending,
			StartTime: time.Now(),
		},
	}
}

// SetMaxParallelJobs sets the maximum number of parallel jobs
func (e *LocalEngine) SetMaxParallelJobs(maxJobs int) {
	if maxJobs > 0 {
		e.maxParallel = maxJobs
		logger.Debugf("Set max parallel jobs to: %d", maxJobs)
	}
}

// Execute executes a command locally
func (e *LocalEngine) Execute(cmd []string) error {
	e.status.State = StatusRunning
	e.status.Message = "Executing locally"

	// Create context with timeout
	e.ctx, e.cancel = context.WithTimeout(context.Background(), 24*time.Hour)
	defer e.cancel()

	// Create command
	e.cmd = exec.CommandContext(e.ctx, cmd[0], cmd[1:]...)

	// Set working directory to current directory (where Snakefiles are located)
	if dir, err := os.Getwd(); err == nil {
		e.cmd.Dir = dir
		logger.Debugf("Setting working directory: %s", dir)
	}

	// Stream progress to the task log while retaining diagnostics for errors.
	var stdout, stderr bytes.Buffer
	e.cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	e.cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	// Start execution
	logger.Debugf("Executing command: %s", strings.Join(cmd, " "))

	if err := e.cmd.Start(); err != nil {
		e.status.State = StatusFailed
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Wait for completion
	if err := e.cmd.Wait(); err != nil {
		e.status.State = StatusFailed
		output := stderr.String()
		if stdout.Len() > 0 {
			output = stdout.String() + "\n" + output
		}
		return fmt.Errorf("command execution failed: %w\nOutput:\n%s", err, output)
	}

	e.status.State = StatusCompleted
	e.status.EndTime = time.Now()
	logger.Info("Local execution completed successfully")

	return nil
}

// ExecuteWithOutput executes a command and returns output
func (e *LocalEngine) ExecuteWithOutput(cmd []string) (string, error) {
	e.status.State = StatusRunning
	e.status.Message = "Executing locally"

	// Create command
	cmdExec := exec.Command(cmd[0], cmd[1:]...)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmdExec.Stdout = &stdout
	cmdExec.Stderr = &stderr

	logger.Debugf("Executing command: %s", strings.Join(cmd, " "))

	// Run command
	err := cmdExec.Run()

	e.status.State = StatusCompleted
	e.status.EndTime = time.Now()

	if err != nil {
		return "", fmt.Errorf("command execution failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// GetName returns the engine name
func (e *LocalEngine) GetName() EngineType {
	return EngineLocal
}

// GetStatus returns the current status
func (e *LocalEngine) GetStatus() *Status {
	if e.cmd != nil && e.cmd.ProcessState != nil {
		if e.cmd.ProcessState.Exited() {
			if e.cmd.ProcessState.ExitCode() == 0 {
				e.status.State = StatusCompleted
			} else {
				e.status.State = StatusFailed
			}
		} else {
			e.status.State = StatusRunning
		}
	}
	return e.status
}

// Wait waits for the command to complete
func (e *LocalEngine) Wait() error {
	if e.cmd == nil {
		return fmt.Errorf("no command running")
	}
	return e.cmd.Wait()
}

// Kill terminates the running command
func (e *LocalEngine) Kill() error {
	if e.cancel != nil {
		e.cancel()
	}
	if e.cmd != nil && e.cmd.Process != nil {
		if err := e.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}
	e.status.State = StatusKilled
	e.status.EndTime = time.Now()
	logger.Info("Local process terminated")
	return nil
}

// SetLogDir is a no-op for local engine (logs handled by logger package)
func (e *LocalEngine) SetLogDir(dir string) error {
	// Local engine doesn't need a separate log directory
	// Logs are handled by the logger package (otter.log)
	return nil
}

// ExecuteWithParallel executes multiple commands in parallel with a worker pool
func (e *LocalEngine) ExecuteWithParallel(commands [][]string, maxParallel int) error {
	if len(commands) == 0 {
		return nil
	}

	// Set max parallel jobs
	if maxParallel > 0 {
		e.maxParallel = maxParallel
	} else if e.maxParallel <= 0 {
		e.maxParallel = 4
	}

	e.status.State = StatusRunning
	e.status.Message = fmt.Sprintf("Executing %d commands with %d parallel workers", len(commands), e.maxParallel)
	logger.Infof("Starting parallel execution: %d commands, max parallel: %d", len(commands), e.maxParallel)

	// Create worker pool
	workerPool := make(chan struct{}, e.maxParallel)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	// Execute commands
	for i, cmd := range commands {
		wg.Add(1)
		go func(index int, command []string) {
			defer wg.Done()

			// Acquire worker slot
			workerPool <- struct{}{}
			defer func() { <-workerPool }()

			logger.Debugf("Executing command %d/%d: %s", index+1, len(commands), strings.Join(command, " "))

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
			defer cancel()

			// Create command
			cmdExec := exec.CommandContext(ctx, command[0], command[1:]...)

			// Set working directory
			if dir, err := os.Getwd(); err == nil {
				cmdExec.Dir = dir
			}

			// Stream each sample's progress to the detached task log.
			var stdout, stderr bytes.Buffer
			cmdExec.Stdout = io.MultiWriter(os.Stdout, &stdout)
			cmdExec.Stderr = io.MultiWriter(os.Stderr, &stderr)

			// Execute
			err := cmdExec.Run()

			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("command %d failed: %w\nOutput: %s", index+1, err, stderr.String()))
				mu.Unlock()
				logger.Errorf("Command %d failed: %v", index+1, err)
			} else {
				logger.Debugf("Command %d completed successfully", index+1)
			}
		}(i, cmd)
	}

	// Wait for all commands to complete
	wg.Wait()

	e.status.EndTime = time.Now()

	if len(errors) > 0 {
		e.status.State = StatusFailed
		return fmt.Errorf("%d commands failed: %v", len(errors), errors)
	}

	e.status.State = StatusCompleted
	e.status.Message = "All parallel commands completed successfully"
	logger.Info("All parallel commands completed successfully")

	return nil
}

// ExecuteSamples executes a workflow for multiple samples in parallel
func (e *LocalEngine) ExecuteSamples(step int, samples []string, condaEnv string, workflowFile string, configFile string, parallelJobs int) error {
	if len(samples) == 0 {
		return fmt.Errorf("no samples provided")
	}

	e.status.State = StatusRunning
	e.status.Message = fmt.Sprintf("Executing Step %d for %d samples with %d parallel jobs", step, len(samples), parallelJobs)

	// Determine max parallel jobs
	maxParallel := e.maxParallel
	if parallelJobs > 0 {
		maxParallel = parallelJobs
	}

	// Build commands for each sample
	commands := make([][]string, len(samples))
	for i, sample := range samples {
		cmd := buildSampleSnakemakeCommand(sample, workflowFile, configFile)

		if condaEnv != "" {
			if enva.IsAvailable() {
				// 使用 enva: enva run <env> -- <cmd...>
				cmd = append([]string{"enva", "run", condaEnv, "--"}, cmd...)
			} else {
				// 回退到 conda: conda run -n <env> <cmd...>
				cmd = append([]string{"conda", "run", "-n", condaEnv}, cmd...)
			}
		}

		commands[i] = cmd
	}

	// Execute in parallel with controlled max parallel jobs
	return e.ExecuteWithParallel(commands, maxParallel)
}

func buildSampleSnakemakeCommand(sample string, workflowFile string, configFile string) []string {
	// Use shared command shape with the main executor.
	cmd := []string{"snakemake", "--cores", "all"}

	if workflowFile != "" {
		cmd = append(cmd, "--snakefile", workflowFile)
	}

	if configFile != "" {
		cmd = append(cmd, "--configfile", configFile)
	}

	// Use config override to process single sample.
	cmd = append(cmd, "--config", fmt.Sprintf("SIDs=[%s]", sample))
	return cmd
}
