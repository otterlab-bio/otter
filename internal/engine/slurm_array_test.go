package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otterlab-bio/otter/internal/config"
)

// testPartition returns a SLURM partition name suitable for testing.
// Defaults to "test" unless OTTER_TEST_PARTITION is set.
func testPartition() string {
	if p := os.Getenv("OTTER_TEST_PARTITION"); p != "" {
		return p
	}
	return "test"
}

func TestSlurmArrayEngine(t *testing.T) {
	// Create a SLURM config
	slurmConfig := &SlurmConfig{
		Partition:  testPartition(),
		Cores:      16,
		Memory:     "32G",
		JobName:    "test_job",
		MaxRetries: 3,
	}

	// Create sample list
	samples := []string{"sample1", "sample2", "sample3", "sample4", "sample5"}

	// Create step resource
	stepResource := &config.StepResource{
		Cores:     16,
		Memory:    "32G",
		Partition: testPartition(),
		Threads:   8,
		JobArray:  true,
		MaxJobs:   5,
	}

	// Create SlurmArrayEngine
	arrayEngine := NewSlurmArrayEngine(slurmConfig, samples, stepResource)

	// Test array size
	if arrayEngine.GetArraySize() != 5 {
		t.Errorf("Expected array size 5, got %d", arrayEngine.GetArraySize())
	}

	// Test samples
	if len(arrayEngine.GetSamples()) != len(samples) {
		t.Errorf("Expected %d samples, got %d", len(samples), len(arrayEngine.GetSamples()))
	}

	// Test step resource
	if arrayEngine.stepResource.Cores != 16 {
		t.Errorf("Expected cores 16, got %d", arrayEngine.stepResource.Cores)
	}
	if arrayEngine.stepResource.Memory != "32G" {
		t.Errorf("Expected memory 32G, got %s", arrayEngine.stepResource.Memory)
	}
	if arrayEngine.stepResource.Partition != testPartition() {
		t.Errorf("Expected partition %s, got %s", testPartition(), arrayEngine.stepResource.Partition)
	}
	if arrayEngine.maxArrayJobs != 5 {
		t.Errorf("Expected max jobs 5, got %d", arrayEngine.maxArrayJobs)
	}
}

func TestSlurmArrayEngineEmptySamples(t *testing.T) {
	slurmConfig := &SlurmConfig{
		Partition: testPartition(),
		Cores:     4,
		Memory:    "8G",
	}

	arrayEngine := NewSlurmArrayEngine(slurmConfig, []string{}, &config.StepResource{})
	if arrayEngine.GetArraySize() != 0 {
		t.Errorf("Expected array size 0, got %d", arrayEngine.GetArraySize())
	}
	if len(arrayEngine.GetSamples()) != 0 {
		t.Errorf("Expected empty samples, got %d samples", len(arrayEngine.GetSamples()))
	}
}

func TestSlurmArrayCheckJobStatusUsesAccountingForEmptyQueue(t *testing.T) {
	testCommandDirectory := t.TempDir()
	writeSlurmTestCommand(t, testCommandDirectory, "squeue", "#!/bin/sh\nprintf '\\n'\n")
	writeSlurmTestCommand(t, testCommandDirectory, "sacct", "#!/bin/sh\nprintf 'COMPLETED\\nCOMPLETED\\n'\n")
	t.Setenv("PATH", testCommandDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))

	arrayEngine := NewSlurmArrayEngine(&SlurmConfig{JobName: "array-reconciliation-test"}, []string{"sample1", "sample2"}, nil)
	status, err := arrayEngine.checkArrayJobStatus("45678")
	if err != nil {
		t.Fatalf("checkArrayJobStatus returned an error: %v", err)
	}
	if status.Completed != 2 || status.Failed != 0 {
		t.Fatalf("checkArrayJobStatus = %+v, want two completed tasks and no failures", status)
	}
}

func TestEngineFactorySlurmArray(t *testing.T) {
	factory := &EngineFactory{}

	// Test with resources
	samples := []string{"sample1", "sample2"}
	stepResource := &config.StepResource{
		Cores:     8,
		Memory:    "16G",
		Time:      "02:00:00",
		Partition: "cpu",
	}

	slurmCfg := &SlurmConfig{
		Partition: "cpu",
		Cores:     8,
		Memory:    "16G",
		JobName:   "test",
	}

	arrayEngine, err := factory.NewSlurmArrayEngineWithResources(
		&config.EngineConfig{}, // Minimal config, not used in this test
		samples,
		stepResource,
		0,   // maxBatchSize=0: no batching limit
		0.0, // loadRatio=0: disable dynamic pool for test
	)

	if arrayEngine == nil {
		t.Fatal("Array engine should not be nil")
	}
	if arrayEngine.partition != "cpu" || arrayEngine.cores != 8 || arrayEngine.memory != "16G" || arrayEngine.timeLimit != "02:00:00" {
		t.Fatalf("step resources did not override Slurm allocation: partition=%q cores=%d memory=%q time=%q", arrayEngine.partition, arrayEngine.cores, arrayEngine.memory, arrayEngine.timeLimit)
	}

	// Override with our specific config for testing
	arrayEngine.partition = slurmCfg.Partition
	arrayEngine.cores = slurmCfg.Cores
	arrayEngine.memory = slurmCfg.Memory

	if err != nil {
		t.Errorf("Should not error creating array engine, got: %v", err)
	}
	if arrayEngine == nil {
		t.Error("Array engine should not be nil")
	}
	if arrayEngine.GetArraySize() != 2 {
		t.Errorf("Expected array size 2, got %d", arrayEngine.GetArraySize())
	}
	if len(arrayEngine.GetSamples()) != 2 {
		t.Errorf("Expected 2 samples, got %d", len(arrayEngine.GetSamples()))
	}
}

func TestSlurmArrayScriptUsesResolvedPhaseTimeLimit(t *testing.T) {
	stepResource := &config.StepResource{Cores: 8, Memory: "16GiB", Time: "02:00:00", Partition: "amd_512"}
	arrayEngine := NewSlurmArrayEngine(
		&SlurmConfig{Partition: "amd_512", Cores: 8, Memory: "16GiB", Time: "02:00:00", JobName: "parity"},
		[]string{"sample-a"},
		stepResource,
	)
	if err := arrayEngine.SetLogDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	scriptPath, err := arrayEngine.generateArrayScript(2, "", "workflow.smk", "run.yaml")
	if err != nil {
		t.Fatal(err)
	}
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, directive := range []string{"#SBATCH --partition=amd_512", "#SBATCH --cpus-per-task=8", "#SBATCH --mem=16384M", "#SBATCH --time=02:00:00"} {
		if !strings.Contains(string(scriptContent), directive) {
			t.Fatalf("generated array script is missing %q:\n%s", directive, scriptContent)
		}
	}
}

func TestSlurmArraySingleSampleScriptRetainsLogsInConfiguredDirectory(t *testing.T) {
	logDirectory := t.TempDir()
	arrayEngine := NewSlurmArrayEngine(
		&SlurmConfig{Partition: "amd_512", Cores: 8, Memory: "16GiB", Time: "02:00:00", JobName: "parity"},
		[]string{"sample-a"},
		&config.StepResource{Cores: 8, Memory: "16GiB", Time: "02:00:00", Partition: "amd_512"},
	)
	if err := arrayEngine.SetLogDir(logDirectory); err != nil {
		t.Fatal(err)
	}

	scriptPath, err := arrayEngine.generateSingleSampleScript(2, "", "workflow.smk", "run.yaml", "sample-a")
	if err != nil {
		t.Fatal(err)
	}
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, directive := range []string{
		"#SBATCH --output=" + filepath.Join(logDirectory, "parity_step2_sample-a_%j.out"),
		"#SBATCH --error=" + filepath.Join(logDirectory, "parity_step2_sample-a_%j.err"),
	} {
		if !strings.Contains(string(scriptContent), directive) {
			t.Fatalf("generated single-sample script is missing %q:\n%s", directive, scriptContent)
		}
	}
}

func TestLocalEngineSetMaxParallel(t *testing.T) {
	config := &LocalConfig{
		MaxCores:  8,
		MaxMemory: "16G",
	}

	engine := NewLocalEngine(config)
	if engine.maxParallel != 8 {
		t.Errorf("Expected max parallel 8, got %d", engine.maxParallel)
	}

	engine.SetMaxParallelJobs(16)
	if engine.maxParallel != 16 {
		t.Errorf("Expected max parallel 16, got %d", engine.maxParallel)
	}

	engine.SetMaxParallelJobs(0)
	if engine.maxParallel != 16 {
		t.Errorf("Expected max parallel to remain 16, got %d", engine.maxParallel)
	}
}

func TestLocalEngineDefaultMaxParallel(t *testing.T) {
	config := &LocalConfig{
		MaxCores:  0,
		MaxMemory: "8G",
	}

	engine := NewLocalEngine(config)
	if engine.maxParallel != 4 {
		t.Errorf("Expected max parallel to default to 4, got %d", engine.maxParallel)
	}
}
