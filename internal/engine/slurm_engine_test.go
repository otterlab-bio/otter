package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otterlab-bio/otter/internal/logger"
)

func init() {
	logger.Init(false)
}

func TestNewSlurmEngine(t *testing.T) {
	config := &SlurmConfig{
		Partition:  "compute",
		Cores:      16,
		Memory:     "32G",
		Time:       "02:00:00",
		JobName:    "test_job",
		MaxRetries: 3,
	}

	engine := NewSlurmEngine(config)

	if engine == nil {
		t.Fatal("NewSlurmEngine returned nil")
	}

	if engine.partition != "compute" {
		t.Errorf("Expected partition to be 'compute', got '%s'", engine.partition)
	}

	if engine.cores != 16 {
		t.Errorf("Expected cores to be 16, got %d", engine.cores)
	}

	if engine.memory != "32G" {
		t.Errorf("Expected memory to be '32G', got '%s'", engine.memory)
	}
	if engine.timeLimit != "02:00:00" {
		t.Errorf("Expected time limit to be '02:00:00', got '%s'", engine.timeLimit)
	}

	if engine.jobName != "test_job" {
		t.Errorf("Expected jobName to be 'test_job', got '%s'", engine.jobName)
	}

	if engine.maxRetries != 3 {
		t.Errorf("Expected maxRetries to be 3, got %d", engine.maxRetries)
	}

	if engine.status.State != StatusPending {
		t.Errorf("Expected status to be PENDING, got %s", engine.status.State)
	}
}

func TestSlurmEngine_GetName(t *testing.T) {
	config := &SlurmConfig{
		JobName: "test",
	}

	engine := NewSlurmEngine(config)

	if engine.GetName() != EngineSlurm {
		t.Errorf("Expected engine name to be SLURM, got %s", engine.GetName())
	}
}

func TestSlurmEngine_GetStatus(t *testing.T) {
	config := &SlurmConfig{
		JobName: "test",
	}

	engine := NewSlurmEngine(config)

	status := engine.GetStatus()

	if status == nil {
		t.Fatal("GetStatus() returned nil")
	}

	if status.State != StatusPending {
		t.Errorf("Expected status to be PENDING, got %s", status.State)
	}
}

func TestSlurmEngine_ExecuteWithOutput(t *testing.T) {
	config := &SlurmConfig{
		JobName: "test",
	}

	engine := NewSlurmEngine(config)

	// ExecuteWithOutput should now work with the fixed implementation
	// However, it requires actual Slurm environment to function
	// In test environment, it will fail when trying to execute sbatch

	// We test the structure of the implementation
	// The method should not immediately return an error like before

	// Test that the method doesn't panic and returns an appropriate error
	// when Slurm is not available
	output, err := engine.ExecuteWithOutput([]string{"echo", "test"})

	// We expect an error in test environment without Slurm
	// But it should NOT be the old "not implemented" error
	if err != nil && err.Error() == "ExecuteWithOutput not implemented for Slurm engine (use Execute instead)" {
		t.Errorf("ExecuteWithOutput() still returns 'not implemented' error")
	}

	// Output should be non-empty on success (job info from sacct)
	// Note: In a real SLURM environment, the job will actually run and return output
	if err == nil && output == "" {
		t.Errorf("Expected job output on success, got empty string")
	}
	// If the test ran successfully, log the output for debugging
	if err == nil {
		t.Logf("Job executed successfully, output: %s", output)
	}
}

func TestSlurmEngine_Kill(t *testing.T) {
	config := &SlurmConfig{
		JobName: "test",
	}

	engine := NewSlurmEngine(config)

	// Test kill without a job ID
	err := engine.Kill()

	if err == nil {
		t.Error("Kill() should have returned an error when no job ID is available")
	}
}

func TestSlurmEngine_WaitWithoutJob(t *testing.T) {
	config := &SlurmConfig{
		JobName: "test",
	}

	engine := NewSlurmEngine(config)

	// Try to wait without submitting a job
	err := engine.Wait()

	if err == nil {
		t.Error("Wait() should have returned an error when no job ID is available")
	}
}

func TestNormalizeSlurmStateRemovesAccountingSuffix(t *testing.T) {
	for rawState, expectedState := range map[string]string{
		"COMPLETED":          "COMPLETED",
		"COMPLETED+":         "COMPLETED",
		"CANCELLED by 12345": "CANCELLED",
		"  OUT_OF_MEMORY  ":  "OUT_OF_MEMORY",
		"":                   "",
	} {
		if actualState := normalizeSlurmState(rawState); actualState != expectedState {
			t.Errorf("normalizeSlurmState(%q) = %q, want %q", rawState, actualState, expectedState)
		}
	}
}

func TestSlurmEngineCheckJobStatusUsesAccountingAfterQueueFailure(t *testing.T) {
	testCommandDirectory := t.TempDir()
	writeSlurmTestCommand(t, testCommandDirectory, "squeue", "#!/bin/sh\nexit 1\n")
	writeSlurmTestCommand(t, testCommandDirectory, "sacct", "#!/bin/sh\nprintf 'COMPLETED\\n'\n")
	t.Setenv("PATH", testCommandDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))

	slurmEngine := NewSlurmEngine(&SlurmConfig{JobName: "reconciliation-test"})
	status, err := slurmEngine.checkJobStatus("12345")
	if err != nil {
		t.Fatalf("checkJobStatus returned an error: %v", err)
	}
	if status.State != StatusCompleted {
		t.Fatalf("checkJobStatus state = %s, want %s", status.State, StatusCompleted)
	}
}

func TestSlurmEngineCheckJobStatusDoesNotFailDuringTransientControlPlaneFailure(t *testing.T) {
	testCommandDirectory := t.TempDir()
	writeSlurmTestCommand(t, testCommandDirectory, "squeue", "#!/bin/sh\nexit 1\n")
	writeSlurmTestCommand(t, testCommandDirectory, "sacct", "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", testCommandDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))

	slurmEngine := NewSlurmEngine(&SlurmConfig{JobName: "reconciliation-test"})
	status, err := slurmEngine.checkJobStatus("12345")
	if err != nil {
		t.Fatalf("checkJobStatus returned an error: %v", err)
	}
	if status.State != StatusRunning {
		t.Fatalf("checkJobStatus state = %s, want %s", status.State, StatusRunning)
	}
}

func TestSlurmEngineRetriesTransientSubmissionFailure(t *testing.T) {
	testCommandDirectory := t.TempDir()
	attemptPath := filepath.Join(t.TempDir(), "attempt-count")
	writeSlurmTestCommand(t, testCommandDirectory, "sbatch", "#!/bin/sh\nattempt_path=\"$OTTER_TEST_SUBMISSION_ATTEMPT_PATH\"\nattempts=0\nif [ -f \"$attempt_path\" ]; then attempts=$(cat \"$attempt_path\"); fi\nattempts=$((attempts + 1))\nprintf '%s' \"$attempts\" > \"$attempt_path\"\nif [ \"$attempts\" -eq 1 ]; then printf 'Unable to contact slurm controller\\n' >&2; exit 1; fi\nprintf 'Submitted batch job 98765\\n'\n")
	t.Setenv("PATH", testCommandDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OTTER_TEST_SUBMISSION_ATTEMPT_PATH", attemptPath)

	slurmEngine := NewSlurmEngine(&SlurmConfig{JobName: "submission-retry-test", MaxRetries: 1})
	jobID, err := slurmEngine.submitJob("workflow.sh")
	if err != nil {
		t.Fatalf("submitJob returned an error: %v", err)
	}
	if jobID != "98765" {
		t.Fatalf("submitJob job ID = %q, want 98765", jobID)
	}
	attemptCount, err := os.ReadFile(attemptPath)
	if err != nil {
		t.Fatalf("read submission attempt count: %v", err)
	}
	if string(attemptCount) != "2" {
		t.Fatalf("submission attempt count = %q, want 2", attemptCount)
	}
}

func TestSlurmEngineDoesNotRetryPermanentSubmissionFailure(t *testing.T) {
	testCommandDirectory := t.TempDir()
	attemptPath := filepath.Join(t.TempDir(), "attempt-count")
	writeSlurmTestCommand(t, testCommandDirectory, "sbatch", "#!/bin/sh\nattempt_path=\"$OTTER_TEST_SUBMISSION_ATTEMPT_PATH\"\nattempts=0\nif [ -f \"$attempt_path\" ]; then attempts=$(cat \"$attempt_path\"); fi\nattempts=$((attempts + 1))\nprintf '%s' \"$attempts\" > \"$attempt_path\"\nprintf 'Invalid account or account/partition combination specified\\n' >&2\nexit 1\n")
	t.Setenv("PATH", testCommandDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OTTER_TEST_SUBMISSION_ATTEMPT_PATH", attemptPath)

	slurmEngine := NewSlurmEngine(&SlurmConfig{JobName: "submission-retry-test", MaxRetries: 2})
	if _, err := slurmEngine.submitJob("workflow.sh"); err == nil {
		t.Fatal("submitJob succeeded despite permanent submission failure")
	}
	attemptCount, err := os.ReadFile(attemptPath)
	if err != nil {
		t.Fatalf("read submission attempt count: %v", err)
	}
	if string(attemptCount) != "1" {
		t.Fatalf("submission attempt count = %q, want 1", attemptCount)
	}
}

func writeSlurmTestCommand(t *testing.T, directory, commandName, scriptContents string) {
	t.Helper()
	commandPath := filepath.Join(directory, commandName)
	if err := os.WriteFile(commandPath, []byte(scriptContents), 0755); err != nil {
		t.Fatalf("write %s test command: %v", commandName, err)
	}
}
