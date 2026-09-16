package engine

import (
	"strings"
	"testing"

	"github.com/otterlab-bio/otter/internal/logger"
)

func init() {
	logger.Init(false)
}

func TestNewLocalEngine(t *testing.T) {
	config := &LocalConfig{
		MaxCores:  8,
		MaxMemory: "16G",
	}

	engine := NewLocalEngine(config)

	if engine == nil {
		t.Fatal("NewLocalEngine returned nil")
	}

	if engine.maxCores != 8 {
		t.Errorf("Expected maxCores to be 8, got %d", engine.maxCores)
	}

	if engine.maxMemory != "16G" {
		t.Errorf("Expected maxMemory to be '16G', got '%s'", engine.maxMemory)
	}

	if engine.status.State != StatusPending {
		t.Errorf("Expected status to be PENDING, got %s", engine.status.State)
	}
}

func TestLocalEngine_GetName(t *testing.T) {
	config := &LocalConfig{
		MaxCores: 4,
	}

	engine := NewLocalEngine(config)

	if engine.GetName() != EngineLocal {
		t.Errorf("Expected engine name to be LOCAL, got %s", engine.GetName())
	}
}

func TestLocalEngine_GetStatus(t *testing.T) {
	config := &LocalConfig{
		MaxCores: 4,
	}

	engine := NewLocalEngine(config)

	status := engine.GetStatus()

	if status == nil {
		t.Fatal("GetStatus() returned nil")
	}

	if status.State != StatusPending {
		t.Errorf("Expected status to be PENDING, got %s", status.State)
	}
}

func TestLocalEngine_Kill(t *testing.T) {
	config := &LocalConfig{
		MaxCores: 4,
	}

	engine := NewLocalEngine(config)

	// Test kill before starting any command
	err := engine.Kill()

	// Should not panic
	if err != nil {
		t.Errorf("Kill() returned unexpected error: %v", err)
	}

	if engine.status.State != StatusKilled {
		t.Errorf("Expected status to be KILLED after Kill(), got %s", engine.status.State)
	}
}

func TestLocalEngine_StatusTransitions(t *testing.T) {
	config := &LocalConfig{
		MaxCores: 4,
	}

	engine := NewLocalEngine(config)

	// Initial status should be PENDING
	if engine.status.State != StatusPending {
		t.Errorf("Expected initial status to be PENDING, got %s", engine.status.State)
	}

	// Kill to test status transition
	err := engine.Kill()

	if err != nil {
		t.Errorf("Kill() returned unexpected error: %v", err)
	}

	// After kill, status should be KILLED
	if engine.status.State != StatusKilled {
		t.Errorf("Expected status to be KILLED after Kill(), got %s", engine.status.State)
	}
}

func TestLocalEngine_WaitWithoutCommand(t *testing.T) {
	config := &LocalConfig{
		MaxCores: 4,
	}

	engine := NewLocalEngine(config)

	// Try to wait without starting a command
	err := engine.Wait()

	if err == nil {
		t.Error("Wait() should have returned an error when no command is running")
	}
}

func TestBuildSampleSnakemakeCommandIncludesConfigfile(t *testing.T) {
	cmd := buildSampleSnakemakeCommand("sampleA", "workflow.smk", "/tmp/config.yaml")
	commandLine := strings.Join(cmd, " ")

	if !strings.Contains(commandLine, "--snakefile workflow.smk") {
		t.Fatalf("expected snakefile argument in command, got: %s", commandLine)
	}
	if !strings.Contains(commandLine, "--configfile /tmp/config.yaml") {
		t.Fatalf("expected configfile argument in command, got: %s", commandLine)
	}
	if !strings.Contains(commandLine, "SIDs=[sampleA]") {
		t.Fatalf("expected single-sample override in command, got: %s", commandLine)
	}
}
