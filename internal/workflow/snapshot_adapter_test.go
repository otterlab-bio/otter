package workflow

import (
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

func TestSnapshotToLegacyConfigAcceptsCraftmakeSnapshot(t *testing.T) {
	snapshot := configv1.RunSnapshot{
		Workflow: configv1.ResolvedWorkflow{Scenario: configv1.ScenarioRRBS},
		Execution: configv1.ResolvedExecution{
			Executor: configv1.ResolvedExecutor{Value: configv1.ExecutorCraftmake},
			Backend:  configv1.ResolvedBackend{Value: configv1.BackendSlurm},
			Resources: configv1.ProjectResources{
				Defaults: configv1.ResourceSpec{Cores: 10, Memory: "34G", Time: "12:00:00", Partition: "amd_512"},
			},
		},
		Run:     configv1.RunMetadata{ID: "run-20260804T132112Z-rbcmaq"},
		Project: configv1.ResolvedProject{ID: "gate6-rrbs", Root: "/project"},
		Samples: []configv1.SampleRecord{
			{ID: "RRBS_GATE6", R1: "/inputs/RRBS_GATE6_R1.fastq.gz", R2: "/inputs/RRBS_GATE6_R2.fastq.gz"},
		},
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{
			{
				Role:        configv1.ReferenceRolePrimary,
				ID:          "hg19",
				Organism:    "Homo sapiens",
				Fasta:       configv1.ResolvedAsset{Path: "/refs/hg19/hg19.fa"},
				Indexes:     []configv1.ResolvedAsset{{Type: "bismark", Path: "/refs/hg19/bismark"}},
				Annotations: []configv1.ResolvedAsset{{Type: "gtf", Path: "/refs/hg19/annotations/hg19.gtf"}},
			},
		}},
		Paths: configv1.RunPaths{
			RunRoot: "/project/runs/run-20260804T132112Z-rbcmaq",
			Work:    "/project/runs/run-20260804T132112Z-rbcmaq/work",
			Results: "/project/runs/run-20260804T132112Z-rbcmaq/results",
			Logs:    "/project/runs/run-20260804T132112Z-rbcmaq/logs",
		},
	}

	configuration, err := SnapshotToLegacyConfig(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	resolvedGTF, isStringSlice := configuration.Reference.RNAseq.GTF.([]string)
	if !isStringSlice || len(resolvedGTF) != 1 || resolvedGTF[0] != "/refs/hg19/annotations/hg19.gtf" {
		t.Fatalf("resolved GTF was not preserved: %#v", configuration.Reference.RNAseq.GTF)
	}
	if configuration.StepResources[1].Cores != 10 || configuration.StepResources[1].Memory != "34G" || configuration.StepResources[1].Partition != "amd_512" {
		t.Fatalf("Craftmake resource envelope was not preserved: %+v", configuration.StepResources[1])
	}
}

func TestSnapshotToLegacyConfigUsesPDXModeForBSPDX(t *testing.T) {
	snapshot := configv1.RunSnapshot{
		Workflow: configv1.ResolvedWorkflow{Scenario: configv1.ScenarioBSPDX},
		Execution: configv1.ResolvedExecution{
			Executor: configv1.ResolvedExecutor{Value: configv1.ExecutorSnakemake},
			Backend:  configv1.ResolvedBackend{Value: configv1.BackendSlurm},
		},
		Run:     configv1.RunMetadata{ID: "run-20260812T051500Z-sndpxa"},
		Project: configv1.ResolvedProject{ID: "pdx-parity", Root: "/project"},
		Samples: []configv1.SampleRecord{{
			ID: "PDX_SAMPLE", R1: "/inputs/PDX_SAMPLE_R1.fastq.gz", R2: "/inputs/PDX_SAMPLE_R2.fastq.gz",
		}},
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{
			{
				Role: configv1.ReferenceRoleGraft, ID: "hg38", Organism: "Homo sapiens",
				Fasta:   configv1.ResolvedAsset{Path: "/refs/hg38/genome.fa"},
				Indexes: []configv1.ResolvedAsset{{Type: "bismark", Path: "/refs/hg38/bismark"}},
			},
			{
				Role: configv1.ReferenceRoleHost, ID: "mm10", Organism: "Mus musculus",
				Fasta:   configv1.ResolvedAsset{Path: "/refs/mm10/genome.fa"},
				Indexes: []configv1.ResolvedAsset{{Type: "bismark", Path: "/refs/mm10/bismark"}},
			},
		}},
		Paths: configv1.RunPaths{
			RunRoot: "/project/runs/run-20260812T051500Z-sndpxa",
			Work:    "/project/runs/run-20260812T051500Z-sndpxa/work",
			Results: "/project/runs/run-20260812T051500Z-sndpxa/results",
			Logs:    "/project/runs/run-20260812T051500Z-sndpxa/logs",
		},
	}

	configuration, err := SnapshotToLegacyConfig(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Workflow.Mode != "PDX" {
		t.Fatalf("BS-PDX must use legacy PDX mode, got %q", configuration.Workflow.Mode)
	}
	if configuration.Workflow.Species.Graft != "hg38" || configuration.Workflow.Species.Host != "mm10" {
		t.Fatalf("PDX species were not projected: %+v", configuration.Workflow.Species)
	}
}

func TestSnapshotToLegacyConfigUsesRNASEQModeForRNAPDX(t *testing.T) {
	snapshot := configv1.RunSnapshot{
		Workflow: configv1.ResolvedWorkflow{Scenario: configv1.ScenarioRNAPDX},
		Execution: configv1.ResolvedExecution{
			Executor: configv1.ResolvedExecutor{Value: configv1.ExecutorSnakemake},
			Backend:  configv1.ResolvedBackend{Value: configv1.BackendSlurm},
		},
		Run:     configv1.RunMetadata{ID: "run-20260812T060000Z-rnapdx"},
		Project: configv1.ResolvedProject{ID: "rna-pdx-parity", Root: "/project"},
		Samples: []configv1.SampleRecord{{
			ID: "RNA_PDX_SAMPLE", R1: "/inputs/RNA_PDX_SAMPLE_R1.fastq.gz", R2: "/inputs/RNA_PDX_SAMPLE_R2.fastq.gz",
		}},
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{
			{
				Role: configv1.ReferenceRoleGraft, ID: "hg38", Organism: "Homo sapiens",
				Fasta:   configv1.ResolvedAsset{Path: "/refs/hg38/genome.fa"},
				Indexes: []configv1.ResolvedAsset{{Type: "star", Path: "/refs/hg38/star"}},
			},
			{
				Role: configv1.ReferenceRoleHost, ID: "mm10", Organism: "Mus musculus",
				Fasta:   configv1.ResolvedAsset{Path: "/refs/mm10/genome.fa"},
				Indexes: []configv1.ResolvedAsset{{Type: "star", Path: "/refs/mm10/star"}},
			},
		}},
		Paths: configv1.RunPaths{
			RunRoot: "/project/runs/run-20260812T060000Z-rnapdx",
			Work:    "/project/runs/run-20260812T060000Z-rnapdx/work",
			Results: "/project/runs/run-20260812T060000Z-rnapdx/results",
			Logs:    "/project/runs/run-20260812T060000Z-rnapdx/logs",
		},
	}

	configuration, err := SnapshotToLegacyConfig(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Workflow.Mode != "RNASEQ" {
		t.Fatalf("RNA-PDX must retain legacy RNASEQ mode, got %q", configuration.Workflow.Mode)
	}
	if configuration.Workflow.Species.Graft != "hg38" || configuration.Workflow.Species.Host != "mm10" {
		t.Fatalf("RNA-PDX species were not projected: %+v", configuration.Workflow.Species)
	}
}

func TestSnapshotToLegacyConfigPreservesCanonicalInputs(t *testing.T) {
	snapshot := configv1.RunSnapshot{
		Workflow: configv1.ResolvedWorkflow{Scenario: configv1.ScenarioWGBS},
		Execution: configv1.ResolvedExecution{
			Executor: configv1.ResolvedExecutor{Value: configv1.ExecutorSnakemake},
			Backend:  configv1.ResolvedBackend{Value: configv1.BackendLocal},
			Resources: configv1.ProjectResources{
				Defaults: configv1.ResourceSpec{Cores: 4, Memory: "8GiB", Time: "01:00:00", Partition: "cpu"},
				Phases: map[string]configv1.ResourceSpec{
					"align": {Cores: 8, Memory: "16GiB", Time: "02:00:00", Partition: "cpu"},
				},
			},
		},
		Run:     configv1.RunMetadata{ID: "run-20260726T013245Z-abcdef"},
		Project: configv1.ResolvedProject{ID: "cohort-a", Root: "/project"},
		Samples: []configv1.SampleRecord{
			{ID: "S01", R1: "/inputs/S01_R1.fastq.gz", R2: "/inputs/S01_R2.fastq.gz"},
		},
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{
			{
				Role:     configv1.ReferenceRolePrimary,
				ID:       "hg38",
				Organism: "Homo sapiens",
				Fasta:    configv1.ResolvedAsset{Path: "/refs/hg38/genome.fa.gz"},
				Indexes: []configv1.ResolvedAsset{
					{Type: "bismark", Path: "/refs/hg38/bismark"},
				},
			},
		}},
		Paths: configv1.RunPaths{
			RunRoot: "/project/runs/run-20260726T013245Z-abcdef",
			Work:    "/project/runs/run-20260726T013245Z-abcdef/work",
			Results: "/project/runs/run-20260726T013245Z-abcdef/results",
			Logs:    "/project/runs/run-20260726T013245Z-abcdef/logs",
		},
	}
	configuration, err := SnapshotToLegacyConfig(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Workflow.Mode != "WGBS" || configuration.Workflow.JobID != snapshot.Run.ID {
		t.Fatalf("workflow identity was not preserved: %+v", configuration.Workflow)
	}
	if len(configuration.Workflow.Samples) != 1 || configuration.Workflow.Samples[0].R1 != snapshot.Samples[0].R1 {
		t.Fatalf("sample paths were not preserved: %+v", configuration.Workflow.Samples)
	}
	if configuration.Reference.Files.Fasta[0] != "/refs/hg38/genome.fa.gz" {
		t.Fatalf("reference FASTA was not preserved: %+v", configuration.Reference.Files.Fasta)
	}
	if configuration.Reference.Indices.Genome[0] != "/refs/hg38/bismark/genome" {
		t.Fatalf("Bismark execution root was not normalized: %#v", configuration.Reference.Indices.Genome)
	}
	if configuration.Workflow.Species.Primary != "Homo sapiens" || len(configuration.Workflow.Species.Name) != 1 || configuration.Workflow.Species.Name[0] != "Homo sapiens" {
		t.Fatalf("organism metadata was not mapped to legacy species: %+v", configuration.Workflow.Species)
	}
	if configuration.StepResources[2].Cores != 8 || configuration.StepResources[2].Memory != "16GiB" || configuration.StepResources[2].Time != "02:00:00" || configuration.StepResources[2].Partition != "cpu" {
		t.Fatalf("canonical align resources were not mapped: %+v", configuration.StepResources)
	}
}
