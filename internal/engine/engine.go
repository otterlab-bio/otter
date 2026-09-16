package engine

import (
	"time"
)

// Engine defines the interface for execution engines
type Engine interface {
	Execute(cmd []string) error
	ExecuteWithOutput(cmd []string) (string, error)
	GetName() EngineType
	GetStatus() *Status
	Wait() error
	Kill() error
	SetLogDir(dir string) error
}

// Status represents the execution status
type Status struct {
	State     string    `json:"state"`
	JobID     string    `json:"job_id"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
}

// Status states
const (
	StatusPending   = "PENDING"
	StatusRunning   = "RUNNING"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
	StatusKilled    = "KILLED"
)

// Result represents the result of an execution
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// EngineConfig represents configuration for an engine
type EngineConfig struct {
	Type       string                 `json:"type"`
	Slurm      map[string]interface{} `json:"slurm,omitempty"`
	Local      map[string]interface{} `json:"local,omitempty"`
	MaxRetries int                    `json:"max_retries"`
	Timeout    time.Duration          `json:"timeout"`
}
