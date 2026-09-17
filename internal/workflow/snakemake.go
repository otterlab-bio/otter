package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/internal/enva"
	"github.com/otterlab-bio/otter/internal/logger"
)

const DefaultFallbackEnv = "otter-snakemake"

// SnakemakeExecutor executes Snakemake workflows
type SnakemakeExecutor struct {
	WorkflowIdx string
	Step        int
	ConfigFile  string
	Options     *WorkflowOptions
	CondaEnv    string // Conda environment for Snakemake
	FallbackEnv string // Fallback environment (default: otter-snakemake)
	NoFallback  bool   // Disable automatic fallback
}

// NewSnakemakeExecutor creates a new Snakemake executor
func NewSnakemakeExecutor(workflowIdx string, step int, configFile string, options *WorkflowOptions, condaEnv string) *SnakemakeExecutor {
	return &SnakemakeExecutor{
		WorkflowIdx: workflowIdx,
		Step:        step,
		ConfigFile:  configFile,
		Options:     options,
		CondaEnv:    condaEnv,
		FallbackEnv: DefaultFallbackEnv,
		NoFallback:  false,
	}
}

// SetFallbackConfig sets the fallback configuration
func (e *SnakemakeExecutor) SetFallbackConfig(fallbackEnv string, noFallback bool) {
	e.FallbackEnv = fallbackEnv
	e.NoFallback = noFallback
}

// ValidateAndFallback validates the conda environment and falls back if needed
func (e *SnakemakeExecutor) ValidateAndFallback() error {
	if e.NoFallback {
		logger.Debug("Fallback disabled, skipping validation")
		return nil
	}

	// Get the environment to validate
	envToValidate := e.CondaEnv
	logger.Infof("Validating conda environment: %s", getEnvDisplayName(envToValidate))

	// Validate the environment
	if err := enva.ValidateEnvironment(envToValidate); err != nil {
		// Validation failed
		if envToValidate == "" || envToValidate == e.FallbackEnv {
			// Already using fallback or system snakemake, no more options
			return fmt.Errorf("snakemake environment validation failed: %w", err)
		}

		// Fall back to default environment
		logger.Warnf("Environment '%s' validation failed: %v", getEnvDisplayName(envToValidate), err)
		logger.Infof("Falling back to '%s' environment...", e.FallbackEnv)

		e.CondaEnv = e.FallbackEnv

		// Validate the fallback environment
		if err := enva.ValidateEnvironment(e.CondaEnv); err != nil {
			return fmt.Errorf("fallback environment '%s' also failed: %w", e.FallbackEnv, err)
		}

		logger.Infof("Successfully using fallback environment: %s", e.FallbackEnv)
	} else {
		logger.Infof("Environment validation successful: %s", getEnvDisplayName(envToValidate))
	}

	return nil
}

// getEnvDisplayName returns a display name for the environment
func getEnvDisplayName(env string) string {
	if env == "" {
		return "system"
	}
	return env
}

// snakefileForStep returns the snakemake filename for a given step number.
func snakefileForStep(workflowIdx string, step int) string {
	switch step {
	case 102:
		return fmt.Sprintf("%s_step2_checker.snakemake", workflowIdx)
	case 103:
		return fmt.Sprintf("%s_step3_checker.snakemake", workflowIdx)
	default:
		return fmt.Sprintf("%s_step%d.snakemake", workflowIdx, step)
	}
}

// BuildCommand builds the Snakemake command
func (e *SnakemakeExecutor) BuildCommand() []string {
	cmd := []string{}

	// If conda environment is specified, use enva (if available) or conda run
	if e.CondaEnv != "" {
		if enva.IsAvailable() {
			// 使用 enva: enva run <env> -- snakemake <args...>
			cmd = append(cmd, "enva", "run", e.CondaEnv, "--")
			logger.Debugf("Using enva for optimal performance")
		} else {
			// 回退到 conda: conda run -n <env> --no-capture-output snakemake <args...>
			cmd = append(cmd, "conda", "run", "-n", e.CondaEnv, "--no-capture-output")
			logger.Debugf("enva not found, using conda run")
		}
	}

	// Add Snakemake command
	cmd = append(cmd, "snakemake")

	// Dry run
	if e.Options.DryRun {
		cmd = append(cmd, "-n")
	}

	// Cores
	cmd = append(cmd, "--cores", "all")

	// Snakefile
	snakefile := snakefileForStep(e.WorkflowIdx, e.Step)
	if e.Options.Snakefile != "" {
		snakefile = e.Options.Snakefile
	}
	// Resolve snakemake file path (check current dir and otter-project/)
	snakefile = e.resolveSnakefilePath(snakefile)
	cmd = append(cmd, "--snakefile", snakefile)

	// Config file
	logger.Debugf("ConfigFile path: %s", e.ConfigFile)
	cmd = append(cmd, "--configfile", e.ConfigFile)

	// Rerun incomplete
	cmd = append(cmd, "--rerun-incomplete")

	// Jobs
	if e.Options.Jobs > 0 {
		cmd = append(cmd, "--jobs", fmt.Sprintf("%d", e.Options.Jobs))
	}

	// Force all
	if e.Options.ForceAll {
		cmd = append(cmd, "--forceall")
	}

	// Extra arguments
	if len(e.Options.ExtraArgs) > 0 {
		cmd = append(cmd, e.Options.ExtraArgs...)
	}

	logger.Debugf("Snakemake command: %s", strings.Join(cmd, " "))

	return cmd
}

// GetResolvedSnakefilePath 返回已解析的 snakemake 文件路径（与 BuildCommand 保持一致）
func (e *SnakemakeExecutor) GetResolvedSnakefilePath() string {
	snakefile := snakefileForStep(e.WorkflowIdx, e.Step)
	return e.resolveSnakefilePath(snakefile)
}

// GetSnakefilePath returns the path to the snakefile
func (e *SnakemakeExecutor) GetSnakefilePath() string {
	return snakefileForStep(e.WorkflowIdx, e.Step)
}

// resolveSnakefilePath finds the snakemake file by checking multiple locations
// It checks: 1) current directory, 2) workflows/, 3) otter-project/ subdirectory
//
// The workflows/ entries matter for canonical projects. Those pin the Snakefiles
// under workflows/ rather than at the project root, and a Snakefile locates its
// own include: directives relative to its own directory, so the file is found
// without anything being written to the project root. The otter-project/ and
// root entries remain for the legacy compatibility layout.
func (e *SnakemakeExecutor) resolveSnakefilePath(snakefile string) string {
	// If it's an absolute path, return as-is
	if filepath.IsAbs(snakefile) {
		return snakefile
	}

	// Check current directory first
	if _, err := os.Stat(snakefile); err == nil {
		logger.Debugf("Found snakemake file in current directory: %s", snakefile)
		return snakefile
	}

	// Check the canonical pinned asset directory
	canonicalPath := filepath.Join("workflows", snakefile)
	if _, err := os.Stat(canonicalPath); err == nil {
		logger.Debugf("Found snakemake file in workflows/: %s", canonicalPath)
		return canonicalPath
	}

	// Check otter-project subdirectory
	projectPath := filepath.Join("otter-project", snakefile)
	if _, err := os.Stat(projectPath); err == nil {
		logger.Debugf("Found snakemake file in otter-project/: %s", projectPath)
		return projectPath
	}

	// Fallback to original path (will cause error if not found)
	logger.Warnf("Snakemake file not found in the current directory, workflows/, or otter-project/, using: %s", snakefile)
	return snakefile
}
