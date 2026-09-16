package v1

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectWithSourcesAppliesDefaults(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "project.yaml")
	content := `schema_version: otter.project/v1
project:
  id: cohort-a
workflow:
  scenario: rrbs
execution: {}
samples:
  manifest: samples.tsv
references:
  primary: hg38@GRCh38.p14
`
	if err := os.WriteFile(projectPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	project, sources, err := LoadProjectWithSources(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if project.Execution.Executor != ExecutorCraftmake || sources.Executor != SourceDefault {
		t.Fatalf("executor default/source = %q/%q", project.Execution.Executor, sources.Executor)
	}
	if project.Execution.Backend != BackendAuto || sources.Backend != SourceDefault {
		t.Fatalf("backend default/source = %q/%q", project.Execution.Backend, sources.Backend)
	}
	if project.Workflow.Toolchain != ToolchainModern || sources.Toolchain != SourceDefault {
		t.Fatalf("toolchain default/source = %q/%q", project.Workflow.Toolchain, sources.Toolchain)
	}
}

func TestLoadProjectRejectsUnknownFields(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "project.yaml")
	content := `schema_version: otter.project/v1
project:
  id: cohort-a
  unexpected: true
workflow:
  scenario: rrbs
execution: {}
samples:
  manifest: samples.tsv
references:
  primary: hg38@GRCh38.p14
`
	if err := os.WriteFile(projectPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProject(projectPath); err == nil {
		t.Fatal("expected strict YAML decoding to reject an unknown field")
	}
}

func TestValidateProjectRequiresExplicitPDXRoles(t *testing.T) {
	project := ProjectConfig{
		SchemaVersion: ProjectSchemaVersion,
		Project:       ProjectMetadata{ID: "pdx-project"},
		Workflow:      ProjectWorkflow{Scenario: ScenarioBSPDX, Toolchain: ToolchainModern},
		Execution:     ProjectExecution{Executor: ExecutorCraftmake, Backend: BackendAuto, Site: "auto"},
		Samples:       SamplesDeclaration{Manifest: "samples.tsv"},
		References:    ProjectReferences{Primary: "hg38@GRCh38.p14"},
	}
	if err := ValidateProject(project); err == nil {
		t.Fatal("expected PDX project with primary-only reference to fail")
	}
}

func TestLoadProjectWithSourcesTracksExplicitSourceProject(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "project.yaml")
	content := `schema_version: otter.project/v1
project:
  id: cohort-b
workflow:
  scenario: wgbs
  toolchain: legacy-equivalent
execution:
  executor: snakemake
  backend: slurm
  site: hpc-cluster-1
samples:
  manifest: samples.tsv
references:
  primary: hg38@GRCh38.p14
`
	if err := os.WriteFile(projectPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	project, sources, err := LoadProjectWithSources(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if project.Execution.Executor != ExecutorSnakemake || sources.Executor != SourceProject {
		t.Fatalf("executor value/source = %q/%q, expected snakemake/SourceProject", project.Execution.Executor, sources.Executor)
	}
	if project.Execution.Backend != BackendSlurm || sources.Backend != SourceProject {
		t.Fatalf("backend value/source = %q/%q, expected slurm/SourceProject", project.Execution.Backend, sources.Backend)
	}
	if project.Execution.Site != "hpc-cluster-1" || sources.Site != SourceProject {
		t.Fatalf("site value/source = %q/%q, expected hpc-cluster-1/SourceProject", project.Execution.Site, sources.Site)
	}
	if project.Workflow.Toolchain != ToolchainLegacyEquivalent || sources.Toolchain != SourceProject {
		t.Fatalf("toolchain value/source = %q/%q, expected legacy-equivalent/SourceProject", project.Workflow.Toolchain, sources.Toolchain)
	}
}
