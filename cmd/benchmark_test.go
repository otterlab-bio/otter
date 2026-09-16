package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

func TestParseBenchmarkSacctOutputRequiresCompletedJobs(t *testing.T) {
	jobs, err := parseBenchmarkSacctOutput("41070001|COMPLETED|0:0|6|8G|29|00:00:20|1234K|10M|5M|\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].JobID != "41070001" || jobs[0].ElapsedRawSec != 29 || jobs[0].AllocationCPU != 6 {
		t.Fatalf("unexpected parsed benchmark jobs: %#v", jobs)
	}
	jobs, err = parseBenchmarkSacctOutput("41070003|COMPLETED|0:0|6|8G|29||||\n")
	if err != nil || len(jobs) != 1 || jobs[0].TotalCPU != "" || jobs[0].MaxRSS != "" {
		t.Fatalf("expected unavailable optional metrics to be accepted: %#v, %v", jobs, err)
	}

	if _, err := parseBenchmarkSacctOutput("41070002|FAILED|1:0|6|8G|29|00:00:20|1234K|10M|5M|\n"); err == nil {
		t.Fatal("expected failed Slurm job to be rejected")
	}
}

func TestValidateBenchmarkPairRequiresExecutorOnlyDifference(t *testing.T) {
	resources := configv1.ResourceSpec{Cores: 6, Memory: "8GiB", Time: "02:00:00", Partition: "amd_512"}
	references := []benchmarkReferenceIdentity{{
		Role:           configv1.ReferenceRolePrimary,
		ID:             "hg19",
		Release:        "GRCh37.p13-gencode-v19",
		Organism:       "Homo sapiens",
		ManifestDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	leftCell := benchmarkCell{
		Executor:         configv1.ExecutorCraftmake,
		Scenario:         configv1.ScenarioRRBS,
		Toolchain:        configv1.ToolchainModern,
		Backend:          configv1.BackendSlurm,
		Site:             "paracloud-gate6",
		Phase:            "step1",
		Resources:        resources,
		SamplesDigest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ReferencesDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		References:       references,
	}
	rightCell := leftCell
	rightCell.Executor = configv1.ExecutorSnakemake
	if err := validateBenchmarkPair(leftCell, rightCell); err != nil {
		t.Fatal(err)
	}

	rightCell.WorkflowDigest = "sha256:workflow-mismatch"
	if err := validateBenchmarkPair(leftCell, rightCell); err == nil || !strings.Contains(err.Error(), "workflow assets") {
		t.Fatalf("expected workflow asset mismatch to be rejected, got %v", err)
	}

	rightCell = leftCell
	rightCell.Executor = configv1.ExecutorSnakemake
	if err := validateRequestedBenchmarkJobs(
		[]string{"41070001", "41070002"},
		[]benchmarkSlurmJob{{JobID: "41070001"}, {JobID: "41070002"}},
	); err != nil {
		t.Fatal(err)
	}
	if err := validateRequestedBenchmarkJobs(
		[]string{"41070001", "41070002"},
		[]benchmarkSlurmJob{{JobID: "41070001"}, {JobID: "41070003"}},
	); err == nil {
		t.Fatal("expected unexpected Slurm job ID to be rejected")
	}

	rightCell.Resources.Cores = 8
	if err := validateBenchmarkPair(leftCell, rightCell); err == nil {
		t.Fatal("expected mismatched phase resources to be rejected")
	}
}

func TestWriteCreateOnlyBenchmarkEvidence(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "benchmark.json")
	evidence := benchmarkEvidence{SchemaVersion: benchmarkEvidenceSchemaVersion, Comparison: "executor-parity", Phase: "step1"}
	if err := writeCreateOnlyBenchmarkEvidence(outputPath, evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatal(err)
	}
	if err := writeCreateOnlyBenchmarkEvidence(outputPath, evidence); err == nil {
		t.Fatal("expected create-only benchmark evidence to reject overwrite")
	}
}
