package task

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreCreateLoadAndUpdate(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "tasks"))
	record := NewRecord("task-1", "/project", "/project/config.yaml", "local", []string{"otter", "run"})

	if err := store.Create(record); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	loaded, err := store.Load(record.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Status != StatusQueued {
		t.Fatalf("Load() status = %q, want %q", loaded.Status, StatusQueued)
	}

	if err := store.Update(record.ID, func(current *Record) error {
		current.Status = StatusRunning
		current.PID = os.Getpid()
		current.StartedAt = time.Now()
		return nil
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	updated, err := store.Load(record.ID)
	if err != nil {
		t.Fatalf("Load() after Update error = %v", err)
	}
	if updated.Status != StatusRunning || updated.PID != os.Getpid() {
		t.Fatalf("updated record = %+v", updated)
	}
}

func TestStoreRefreshProcessStatusMarksMissingWorkerInterrupted(t *testing.T) {
	store := NewStore(t.TempDir())
	record := NewRecord("task-2", "/project", "/project/config.yaml", "local", nil)
	record.Status = StatusRunning
	record.PID = 2147483647
	if err := store.Create(record); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := store.RefreshProcessStatus(record); err != nil {
		t.Fatalf("RefreshProcessStatus() error = %v", err)
	}
	refreshed, err := store.Load(record.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if refreshed.Status != StatusInterrupted {
		t.Fatalf("status = %q, want %q", refreshed.Status, StatusInterrupted)
	}
	if refreshed.FinishedAt.IsZero() {
		t.Fatal("FinishedAt should be recorded")
	}
}

func TestStoreSlurmJobLifecycle(t *testing.T) {
	t.Setenv(EnvironmentTaskID, "task-3")
	t.Setenv(EnvironmentTaskStateDir, filepath.Join(t.TempDir(), "tasks"))
	store := NewStore(os.Getenv(EnvironmentTaskStateDir))
	if err := store.Create(NewRecord("task-3", "/project", "/project/config.yaml", "slurm", nil)); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := RegisterCurrentSlurmJob("12345"); err != nil {
		t.Fatalf("RegisterCurrentSlurmJob() error = %v", err)
	}
	if err := RegisterCurrentSlurmJob("12345"); err != nil {
		t.Fatalf("second RegisterCurrentSlurmJob() error = %v", err)
	}
	loaded, err := store.Load("task-3")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.ActiveSlurmJobIDs) != 1 || len(loaded.SlurmJobIDs) != 1 {
		t.Fatalf("job IDs after register = active %v history %v", loaded.ActiveSlurmJobIDs, loaded.SlurmJobIDs)
	}

	if err := CompleteCurrentSlurmJob("12345"); err != nil {
		t.Fatalf("CompleteCurrentSlurmJob() error = %v", err)
	}
	loaded, err = store.Load("task-3")
	if err != nil {
		t.Fatalf("Load() after complete error = %v", err)
	}
	if len(loaded.ActiveSlurmJobIDs) != 0 || len(loaded.SlurmJobIDs) != 1 {
		t.Fatalf("job IDs after complete = active %v history %v", loaded.ActiveSlurmJobIDs, loaded.SlurmJobIDs)
	}
}
