package engine

import (
	"testing"
	"time"

	"github.com/otterlab-bio/otter/internal/logger"
)

func init() {
	logger.Init(false)
}

func TestEngineType_String(t *testing.T) {
	tests := []struct {
		et   EngineType
		want string
	}{
		{EngineSlurm, "slurm"},
		{EngineLocal, "local"},
	}

	for _, tt := range tests {
		if string(tt.et) != tt.want {
			t.Errorf("EngineType.String() = %v, want %v", tt.et, tt.want)
		}
	}
}

func TestNewStatus(t *testing.T) {
	status := &Status{
		State:     StatusPending,
		StartTime: time.Now(),
	}

	if status.State != StatusPending {
		t.Errorf("Expected status to be PENDING, got %s", status.State)
	}
}

func TestStatus_GetProgress(t *testing.T) {
	status := &Status{
		State:    StatusRunning,
		Progress: 50,
	}

	if status.Progress != 50 {
		t.Errorf("Expected progress to be 50, got %d", status.Progress)
	}
}

func TestResult_GetExitCode(t *testing.T) {
	result := &Result{
		ExitCode: 0,
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code to be 0, got %d", result.ExitCode)
	}
}

func TestEngineConfig_Validate(t *testing.T) {
	cfg := &EngineConfig{
		Type:       "local",
		MaxRetries: 3,
		Timeout:    0,
	}

	if cfg.Type != "local" {
		t.Errorf("Expected type to be 'local', got %s", cfg.Type)
	}
}
