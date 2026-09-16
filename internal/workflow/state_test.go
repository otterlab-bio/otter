package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStateInitializeUsesDynamicStepCount(t *testing.T) {
	state := NewState(t.TempDir(), "job-rna")
	err := state.Initialize("job-rna", "RNASEQ", "human", "", 2, []string{"s1", "s2"}, "local", "")
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if got := len(state.GetData().Steps); got != 2 {
		t.Fatalf("expected 2 steps, got %d", got)
	}
}

func TestStateLoadNormalizesLegacyRnaSeqSteps(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, StateFileName)

	legacyJSON := `{
  "version": "1.0",
  "job_id": "legacy-job",
  "start_time": "2026-03-05T00:00:00Z",
  "last_update": "2026-03-05T01:00:00Z",
  "status": "running",
  "config": {
    "workflow_mode": "RNASEQ",
    "species1": "human",
    "sample_count": 2,
    "engine_type": "local"
  },
  "steps": [
    {"step": 1, "name": "quality_control", "status": "completed"},
    {"step": 2, "name": "alignment", "status": "running"},
    {"step": 3, "name": "methylation_calling", "status": "pending"}
  ],
  "samples": {
    "completed": [],
    "running": [],
    "pending": []
  }
}`
	if err := os.WriteFile(statePath, []byte(strings.TrimSpace(legacyJSON)), 0644); err != nil {
		t.Fatalf("failed to write legacy state file: %v", err)
	}

	state := NewState(dir, "legacy-job")
	if err := state.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got := len(state.GetData().Steps); got != 2 {
		t.Fatalf("expected normalized RNASEQ legacy state to have 2 steps, got %d", got)
	}

	if state.GetData().Steps[1].Status != "running" {
		t.Fatalf("expected step 2 status to be preserved, got %s", state.GetData().Steps[1].Status)
	}
}

func TestStateInitializeDefaultsToThreeSteps(t *testing.T) {
	state := NewState(t.TempDir(), "job-default")
	if err := state.Initialize("job-default", "RRBS", "human", "", 0, []string{"s1"}, "local", ""); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if got := len(state.GetData().Steps); got != 3 {
		t.Fatalf("expected default step count 3, got %d", got)
	}

	if state.GetData().StartTime.After(time.Now().Add(2 * time.Second)) {
		t.Fatalf("unexpected start time in future: %s", state.GetData().StartTime)
	}
}
