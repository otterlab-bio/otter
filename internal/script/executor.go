package script

import (
	"fmt"
	"strings"
	"time"

	"github.com/otterlab-bio/otter/internal/engine"
	"github.com/otterlab-bio/otter/internal/enva"
	"github.com/otterlab-bio/otter/internal/logger"
)

// ScriptConfig represents configuration for script execution
type ScriptConfig struct {
	ScriptPath string            `json:"script_path"`
	Args       map[string]string `json:"args"`
	EnvVars    map[string]string `json:"env_vars"`
	WorkingDir string            `json:"working_dir"`
	Timeout    time.Duration     `json:"timeout"`
}

// Executor executes R/Python scripts
type Executor struct {
	engine   engine.Engine
	condaEnv string
}

// NewExecutor creates a new script executor
func NewExecutor(eng engine.Engine, condaEnv string) *Executor {
	return &Executor{
		engine:   eng,
		condaEnv: condaEnv,
	}
}

// ExecuteRScript executes an R script
func (e *Executor) ExecuteRScript(config ScriptConfig) error {
	// 构建基础命令（使用 enva 或 conda）
	cmd := e.buildCommand("Rscript", config.ScriptPath)

	// Add arguments
	for key, value := range config.Args {
		cmd = append(cmd, fmt.Sprintf("--%s=%s", key, value))
	}

	logger.Debugf("Executing R script: %s", strings.Join(cmd, " "))

	return e.execute(cmd)
}

// ExecutePythonScript executes a Python script
func (e *Executor) ExecutePythonScript(config ScriptConfig) error {
	// 构建基础命令（使用 enva 或 conda）
	cmd := e.buildCommand("python", config.ScriptPath)

	// Add arguments (Python uses -O for single char args)
	for key, value := range config.Args {
		cmd = append(cmd, fmt.Sprintf("-%s", key[0:1]), value)
	}

	logger.Debugf("Executing Python script: %s", strings.Join(cmd, " "))

	return e.execute(cmd)
}

// ExecuteTool executes a tool command
func (e *Executor) ExecuteTool(toolName string, args []string) error {
	cmd := e.buildCommand(toolName, args...)
	logger.Debugf("Executing tool: %s", strings.Join(cmd, " "))
	return e.execute(cmd)
}

// buildCommand builds the command array using enva (if available) or conda run
func (e *Executor) buildCommand(tool string, args ...string) []string {
	if enva.IsAvailable() {
		// 使用 enva: enva run <env> -- <tool> <args...>
		result := []string{"enva", "run", e.condaEnv, "--", tool}
		result = append(result, args...)
		logger.Debugf("Using enva for optimal performance")
		return result
	}

	// 回退到 conda: conda run -n <env> <tool> <args...>
	result := []string{"conda", "run", "-n", e.condaEnv, tool}
	result = append(result, args...)
	logger.Debugf("enva not found, using conda run")
	return result
}

// execute executes a command via the engine
func (e *Executor) execute(cmd []string) error {
	if err := e.engine.Execute(cmd); err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}
	return nil
}
