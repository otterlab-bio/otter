package workflow

import (
	"time"

	"github.com/otterlab-bio/otter/internal/config"
	"github.com/otterlab-bio/otter/internal/engine"
)

// Workflow represents a workflow instance
type Workflow struct {
	ID        string              `json:"id"`
	Config    *config.OtterConfig `json:"config"`
	Status    *engine.Status      `json:"status"`
	Samples   []string            `json:"samples"`
	Steps     int                 `json:"steps"`
	OutputDir string              `json:"output_dir"`
	LogFile   string              `json:"log_file"`
	Engine    engine.Engine       `json:"-"`
	Options   *WorkflowOptions    `json:"options,omitempty"`
}

// Step represents a workflow step
type Step struct {
	Number      int            `json:"number"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Status      *engine.Status `json:"status"`
	Config      *StepConfig    `json:"config"`
}

// StepConfig represents configuration for a workflow step
type StepConfig struct {
	Cores     int               `json:"cores"`
	Memory    string            `json:"memory"`
	EnvVars   map[string]string `json:"env_vars"`
	ExtraArgs []string          `json:"extra_args"`
}

// WorkflowOptions represents options for workflow creation
type WorkflowOptions struct {
	DryRun     bool
	Resume     bool
	ForceAll   bool
	Jobs       int
	Cores      int
	MaxMemory  string
	ExtraArgs  []string
	Snakefile  string
	ConfigFile string
}

// NewWorkflow creates a new workflow
func NewWorkflow(config *config.OtterConfig, samples []string) *Workflow {
	return &Workflow{
		ID:     config.Workflow.JobID,
		Config: config,
		Status: &engine.Status{
			State:     engine.StatusPending,
			StartTime: time.Now(),
		},
		Samples:   samples,
		OutputDir: config.Output.BaseDir,
	}
}

// SetEngine sets the execution engine
func (w *Workflow) SetEngine(eng engine.Engine) {
	w.Engine = eng
}
