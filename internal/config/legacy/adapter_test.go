package legacy

import (
	"testing"

	legacyconfig "github.com/otterlab-bio/otter/internal/config"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

func TestAdaptLegacyConfiguration(t *testing.T) {
	configuration := legacyconfig.LoadDefaults()
	configuration.Workflow.Mode = "RRBS"
	configuration.Workflow.JobID = "legacy-job"
	configuration.Workflow.Species.Primary = "human"
	configuration.Engine.Type = "slurm"
	configuration.Input.FastqDir = "data"
	configuration.Input.Suffix1 = "_R1.fastq.gz"
	configuration.Input.Suffix2 = "_R2.fastq.gz"
	configuration.Metadata.SampleIDs = []string{"S01"}

	result := Adapt(*configuration, MigrationOptions{PrimaryReference: "hg38@GRCh38.p14"})
	if result.Report.HasConflicts() {
		t.Fatalf("unexpected conflicts: %+v", result.Report.Conflicts)
	}
	if result.Project.Execution.Executor != configv1.ExecutorSnakemake {
		t.Fatalf("legacy executor should remain snakemake, got %s", result.Project.Execution.Executor)
	}
	if result.Project.Workflow.Toolchain != configv1.ToolchainLegacyEquivalent {
		t.Fatalf("unexpected toolchain: %s", result.Project.Workflow.Toolchain)
	}
	if len(result.Samples) != 1 || result.Samples[0].ID != "S01" {
		t.Fatalf("unexpected samples: %+v", result.Samples)
	}
}

func TestAdaptRejectsAmbiguousBSSEQ(t *testing.T) {
	configuration := legacyconfig.LoadDefaults()
	configuration.Workflow.Mode = "BSSEQ"
	configuration.Metadata.SampleIDs = []string{"S01"}
	configuration.Input.FastqDir = "data"
	configuration.Input.Suffix1 = "_R1.fastq.gz"
	configuration.Input.Suffix2 = "_R2.fastq.gz"

	result := Adapt(*configuration, MigrationOptions{PrimaryReference: "hg38@GRCh38.p14"})
	if !result.Report.HasConflicts() {
		t.Fatal("expected ambiguous BSSEQ migration conflict")
	}
}
