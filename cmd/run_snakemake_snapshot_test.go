package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otterlab-bio/otter/internal/config"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"gopkg.in/yaml.v3"
)

func TestEquivalentYAMLDocumentsIgnoresMappingOrder(t *testing.T) {
	leftDocument := []byte("workflow:\n  mode: RRBS\nstep_resources:\n  1:\n    cores: 6\n  2:\n    cores: 40\n")
	rightDocument := []byte("step_resources:\n  2:\n    cores: 40\n  1:\n    cores: 6\nworkflow:\n  mode: RRBS\n")

	equivalent, err := equivalentYAMLDocuments(leftDocument, rightDocument)
	if err != nil {
		t.Fatalf("compare semantically equivalent YAML: %v", err)
	}
	if !equivalent {
		t.Fatal("mapping-order-only changes must be equivalent")
	}
}

func TestEquivalentYAMLDocumentsRejectsValueChanges(t *testing.T) {
	leftDocument := []byte("workflow:\n  mode: RRBS\n")
	rightDocument := []byte("workflow:\n  mode: WGBS\n")

	equivalent, err := equivalentYAMLDocuments(leftDocument, rightDocument)
	if err != nil {
		t.Fatalf("compare distinct YAML: %v", err)
	}
	if equivalent {
		t.Fatal("different compatibility configuration values must not be equivalent")
	}
}

func TestWriteLegacyRuntimeConfigIsIdempotentWithPhaseResources(t *testing.T) {
	temporaryDirectory := t.TempDir()
	configuration := &config.OtterConfig{
		Workflow: config.WorkflowConfig{
			Mode:    "PDX",
			Species: config.SpeciesConfig{Primary: "hg19"},
		},
		Metadata: config.MetadataConfig{SampleIDs: []string{"RRBS"}},
		Directories: config.DirectoryConfig{
			Work: filepath.Join(temporaryDirectory, "work"),
			QC: config.QCConfig{
				Main:   filepath.Join(temporaryDirectory, "work", "QC"),
				Before: filepath.Join(temporaryDirectory, "work", "fastqc_raw"),
				After:  filepath.Join(temporaryDirectory, "work", "fastqc_clean"),
			},
			BSMAP:           config.BSMAPConfig{Main: filepath.Join(temporaryDirectory, "work", "bsmap")},
			Qualimap:        filepath.Join(temporaryDirectory, "work", "QC", "qualimap"),
			QCSummary:       filepath.Join(temporaryDirectory, "results", "qc"),
			MethylationCall: filepath.Join(temporaryDirectory, "work", "mCall"),
		},
		Reference: config.ReferenceConfig{
			Indices: config.ReferenceIndices{Genome: []string{"/refs/hg19/bismark/genome"}},
		},
		StepResources: map[int]*config.StepResource{
			1:   {Cores: 6, Memory: "8GiB", Time: "08:00:00", Partition: "amd_512"},
			2:   {Cores: 40, Memory: "160GiB", Time: "08:00:00", Partition: "amd_512"},
			3:   {Cores: 20, Memory: "48GiB", Time: "08:00:00", Partition: "amd_512"},
			102: {Cores: 8, Memory: "64GiB", Time: "08:00:00", Partition: "amd_512"},
			103: {Cores: 10, Memory: "34GiB", Time: "08:00:00", Partition: "amd_512"},
		},
	}
	snapshot := configv1.RunSnapshot{
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{
			{Role: configv1.ReferenceRolePrimary, ID: "hg19"},
		}},
		Paths: configv1.RunPaths{
			RunRoot: filepath.Join(temporaryDirectory, "runs", "run-20260805T010203Z-abcdef"),
			State:   filepath.Join(temporaryDirectory, "state"),
		},
	}

	if _, err := writeLegacyRuntimeConfig(snapshot, configuration); err != nil {
		t.Fatalf("write initial legacy runtime config: %v", err)
	}
	if _, err := writeLegacyRuntimeConfig(snapshot, configuration); err != nil {
		t.Fatalf("rewrite legacy runtime config with phase resources: %v", err)
	}
}

func TestLegacyRuntimeConfigMatchesSnapshotAcceptsSemanticallyStableProjection(t *testing.T) {
	snapshot := configv1.RunSnapshot{
		Run: configv1.RunMetadata{ID: "run-20260805T010203Z-abcdef"},
		Paths: configv1.RunPaths{
			RunRoot: "/project/runs/run-20260805T010203Z-abcdef",
			State:   "/project/runs/run-20260805T010203Z-abcdef/state",
		},
	}
	document := []byte("workflow:\n  jobid: run-20260805T010203Z-abcdef\noutput:\n  base_dir: /project/runs/run-20260805T010203Z-abcdef\ndirectories:\n  selfconfig: /project/runs/run-20260805T010203Z-abcdef/state\n  qctb_config: /project/runs/run-20260805T010203Z-abcdef/run.yaml\n")

	matchesSnapshot, err := legacyRuntimeConfigMatchesSnapshot(document, snapshot)
	if err != nil {
		t.Fatalf("validate matching compatibility config: %v", err)
	}
	if !matchesSnapshot {
		t.Fatal("compatibility config anchored to immutable run must be accepted")
	}
}

func TestLegacyRuntimeConfigMatchesSnapshotRejectsDifferentRun(t *testing.T) {
	snapshot := configv1.RunSnapshot{
		Run: configv1.RunMetadata{ID: "run-20260805T010203Z-abcdef"},
		Paths: configv1.RunPaths{
			RunRoot: "/project/runs/run-20260805T010203Z-abcdef",
			State:   "/project/runs/run-20260805T010203Z-abcdef/state",
		},
	}
	document := []byte("workflow:\n  jobid: run-20260805T010204Z-fedcba\noutput:\n  base_dir: /project/runs/run-20260805T010204Z-fedcba\ndirectories:\n  selfconfig: /project/runs/run-20260805T010204Z-fedcba/state\n  qctb_config: /project/runs/run-20260805T010204Z-fedcba/run.yaml\n")

	matchesSnapshot, err := legacyRuntimeConfigMatchesSnapshot(document, snapshot)
	if err != nil {
		t.Fatalf("validate different compatibility config: %v", err)
	}
	if matchesSnapshot {
		t.Fatal("compatibility config for another immutable run must be rejected")
	}
}

func TestSnakemakeStepForPhase(t *testing.T) {
	testCases := []struct {
		name      string
		phase     string
		wantStep  int
		wantError bool
	}{
		{name: "step one", phase: "step1", wantStep: 1},
		{name: "step two", phase: "step2", wantStep: 2},
		{name: "step two checker", phase: "step2-check", wantStep: 102},
		{name: "step three", phase: "step3", wantStep: 3},
		{name: "step three checker", phase: "step3-check", wantStep: 103},
		{name: "case and whitespace normalized", phase: " STEP1 ", wantStep: 1},
		{name: "unknown phase rejected", phase: "publish", wantError: true},
		{name: "empty phase rejected", phase: "", wantError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotStep, err := snakemakeStepForPhase(testCase.phase)
			if testCase.wantError {
				if err == nil {
					t.Fatalf("snakemakeStepForPhase(%q) error = nil, want error", testCase.phase)
				}
				return
			}
			if err != nil {
				t.Fatalf("snakemakeStepForPhase(%q) error = %v", testCase.phase, err)
			}
			if gotStep != testCase.wantStep {
				t.Fatalf("snakemakeStepForPhase(%q) = %d, want %d", testCase.phase, gotStep, testCase.wantStep)
			}
		})
	}
}

func TestClearSnakemakeInheritedJavaHomeRestoresOriginalValue(t *testing.T) {
	t.Setenv("JAVA_HOME", "/incompatible/java")

	restoreJavaHome := clearSnakemakeInheritedJavaHome()
	if _, found := os.LookupEnv("JAVA_HOME"); found {
		t.Fatal("JAVA_HOME must be absent while Snakemake work is submitted")
	}

	restoreJavaHome()
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "/incompatible/java" {
		t.Fatalf("JAVA_HOME after restore = %q, want original value", javaHome)
	}
}

func TestLegacyRuntimeSpeciesIdentifierUsesGraftWithoutPrimaryReference(t *testing.T) {
	snapshot := configv1.RunSnapshot{
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{
			{Role: configv1.ReferenceRoleGraft, ID: "hg38"},
			{Role: configv1.ReferenceRoleHost, ID: "mm10"},
		}},
	}

	identifier := legacyRuntimeSpeciesIdentifier(snapshot, "Homo sapiens")
	if identifier != "hg38" {
		t.Fatalf("legacy runtime species identifier = %q, want hg38", identifier)
	}
}

func TestLegacyRuntimeSpeciesIdentifiersPreservesResolvedReferenceOrder(t *testing.T) {
	snapshot := configv1.RunSnapshot{
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{
			{Role: configv1.ReferenceRolePrimary, ID: "hg19"},
			{Role: configv1.ReferenceRoleGraft, ID: "hg19"},
			{Role: configv1.ReferenceRoleHost, ID: "mm10"},
		}},
	}
	identifiers := legacyRuntimeSpeciesIdentifiers(snapshot, "fallback")
	if len(identifiers) != 2 || identifiers[0] != "hg19" || identifiers[1] != "mm10" {
		t.Fatalf("resolved legacy species identifiers = %#v, want [hg19 mm10]", identifiers)
	}
}

func TestSetPDXReferenceFASTAMappingsUsesResolvedReferenceRoles(t *testing.T) {
	document := yaml.Node{}
	if err := yaml.Unmarshal([]byte("reference: {}\n"), &document); err != nil {
		t.Fatal(err)
	}
	referenceNode, found := yamlMappingValue(document.Content[0], "reference")
	if !found {
		t.Fatal("reference mapping is missing")
	}
	snapshot := configv1.RunSnapshot{
		Workflow: configv1.ResolvedWorkflow{Scenario: configv1.ScenarioBSPDX},
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{
			{Role: configv1.ReferenceRoleGraft, ID: "hg38", Fasta: configv1.ResolvedAsset{Path: "/refs/hg38.fa"}},
			{Role: configv1.ReferenceRoleHost, ID: "mm10", Fasta: configv1.ResolvedAsset{Path: "/refs/mm10.fa"}},
		}},
	}

	if err := setPDXReferenceFASTAMappings(referenceNode, snapshot); err != nil {
		t.Fatalf("set PDX FASTA mappings: %v", err)
	}
	graftFASTANode, found := yamlMappingValue(referenceNode, "graft_fasta")
	if !found || graftFASTANode.Value != "/refs/hg38.fa" {
		t.Fatalf("reference.graft_fasta = %#v, want /refs/hg38.fa", graftFASTANode)
	}
	hostFASTANode, found := yamlMappingValue(referenceNode, "host_fasta")
	if !found || hostFASTANode.Value != "/refs/mm10.fa" {
		t.Fatalf("reference.host_fasta = %#v, want /refs/mm10.fa", hostFASTANode)
	}
}

func TestSetPDXReferenceFASTAMappingsRejectsMissingHostReference(t *testing.T) {
	document := yaml.Node{}
	if err := yaml.Unmarshal([]byte("reference: {}\n"), &document); err != nil {
		t.Fatal(err)
	}
	referenceNode, found := yamlMappingValue(document.Content[0], "reference")
	if !found {
		t.Fatal("reference mapping is missing")
	}
	snapshot := configv1.RunSnapshot{
		Workflow: configv1.ResolvedWorkflow{Scenario: configv1.ScenarioRNAPDX},
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{
			{Role: configv1.ReferenceRoleGraft, ID: "hg38", Fasta: configv1.ResolvedAsset{Path: "/refs/hg38.fa"}},
		}},
	}

	if err := setPDXReferenceFASTAMappings(referenceNode, snapshot); err == nil {
		t.Fatal("missing PDX host reference must be rejected")
	}
}

func TestWriteLegacyRuntimeConfigProjectsLegacyKeys(t *testing.T) {
	temporaryDirectory := t.TempDir()
	stateDirectory := filepath.Join(temporaryDirectory, "state")
	configuration := &config.OtterConfig{
		Workflow: config.WorkflowConfig{
			Mode:    "PDX",
			Species: config.SpeciesConfig{Primary: "Homo sapiens", Graft: "hg19"},
		},
		Metadata: config.MetadataConfig{SampleIDs: []string{"RRBS_GATE6"}},
		Directories: config.DirectoryConfig{
			Work: "/project/work",
			QC: config.QCConfig{
				Main:   "/project/work/QC",
				Before: "/project/work/fastqc_raw",
				After:  "/project/work/fastqc_clean",
			},
			Qualimap:        "/project/work/qualimap",
			QCSummary:       "/project/work/QC/summary",
			MethylationCall: "/project/work/methylation",
		},
		Reference: config.ReferenceConfig{
			Indices: config.ReferenceIndices{Genome: []string{"/refs/hg19/bismark/genome"}},
		},
	}
	snapshot := configv1.RunSnapshot{
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{
			{Role: configv1.ReferenceRolePrimary, ID: "hg19"},
		}},
		Paths: configv1.RunPaths{
			RunRoot: filepath.Join(temporaryDirectory, "runs", "run-20260805T010203Z-abcdef"),
			State:   stateDirectory,
		},
	}

	compatibilityPath, err := writeLegacyRuntimeConfig(snapshot, configuration)
	if err != nil {
		t.Fatal(err)
	}
	if compatibilityPath != filepath.Join(stateDirectory, "config.yaml") {
		t.Fatalf("compatibility config path = %q, want %q", compatibilityPath, filepath.Join(stateDirectory, "config.yaml"))
	}
	configurationFileInfo, err := os.Stat(compatibilityPath)
	if err != nil {
		t.Fatal(err)
	}
	if configurationFileInfo.Mode().Perm() != 0o444 {
		t.Fatalf("legacy runtime config permissions = %o, want 444", configurationFileInfo.Mode().Perm())
	}
	idempotentPath, err := writeLegacyRuntimeConfig(snapshot, configuration)
	if err != nil {
		t.Fatal(err)
	}
	if idempotentPath != compatibilityPath {
		t.Fatalf("idempotent legacy runtime config path = %q, want %q", idempotentPath, compatibilityPath)
	}
	encoded, err := os.ReadFile(compatibilityPath)
	if err != nil {
		t.Fatal(err)
	}

	compatibilityDocument := yaml.Node{}
	if err := yaml.Unmarshal(encoded, &compatibilityDocument); err != nil {
		t.Fatal(err)
	}
	rootMapping := compatibilityDocument.Content[0]
	rootModeNode, found := yamlMappingValue(rootMapping, "mode")
	if !found || rootModeNode.Value != "PDX" {
		t.Fatalf("root mode = %#v, want PDX", rootModeNode)
	}
	speciesNode, found := yamlMappingValue(rootMapping, "species")
	if !found || speciesNode.Value != "hg19" {
		t.Fatalf("root species = %#v, want hg19", speciesNode)
	}
	workflowNode, found := yamlMappingValue(rootMapping, "workflow")
	if !found {
		t.Fatal("workflow mapping is missing")
	}
	workflowSpeciesNode, found := yamlMappingValue(workflowNode, "species")
	if !found {
		t.Fatal("workflow.species mapping is missing")
	}
	workflowSpeciesNameNode, found := yamlMappingValue(workflowSpeciesNode, "name")
	if !found || workflowSpeciesNameNode.Kind != yaml.SequenceNode || len(workflowSpeciesNameNode.Content) != 1 || workflowSpeciesNameNode.Content[0].Value != "hg19" {
		t.Fatalf("workflow.species.name = %#v, want [hg19]", workflowSpeciesNameNode)
	}
	sampleIDsNode, found := yamlMappingValue(rootMapping, "SIDs")
	if !found || len(sampleIDsNode.Content) != 1 || sampleIDsNode.Content[0].Value != "RRBS_GATE6" {
		t.Fatalf("root SIDs = %#v, want [RRBS_GATE6]", sampleIDsNode)
	}
	qualimapNode, found := yamlMappingValue(rootMapping, "outdir_qualimap")
	if !found || qualimapNode.Value != "/project/work/qualimap" {
		t.Fatalf("root outdir_qualimap = %#v, want /project/work/qualimap", qualimapNode)
	}
	qualityControlSummaryNode, found := yamlMappingValue(rootMapping, "qc_summary")
	if !found || qualityControlSummaryNode.Value != "/project/work/QC/summary" {
		t.Fatalf("root qc_summary = %#v, want /project/work/QC/summary", qualityControlSummaryNode)
	}
	methylationCallNode, found := yamlMappingValue(rootMapping, "outDir_mCall")
	if !found || methylationCallNode.Value != "/project/work/methylation" {
		t.Fatalf("root outDir_mCall = %#v, want /project/work/methylation", methylationCallNode)
	}
	graftNode, found := yamlMappingValue(rootMapping, "graft")
	if !found || graftNode.Value != "hg19" {
		t.Fatalf("root graft = %#v, want hg19", graftNode)
	}
	metadataNode, found := yamlMappingValue(rootMapping, "metadata")
	if !found {
		t.Fatal("metadata mapping is missing")
	}
	legacySampleIDsNode, found := yamlMappingValue(metadataNode, "sample_ids")
	if !found || len(legacySampleIDsNode.Content) != 1 || legacySampleIDsNode.Content[0].Value != "RRBS_GATE6" {
		t.Fatalf("metadata.sample_ids = %#v, want [RRBS_GATE6]", legacySampleIDsNode)
	}
	directoriesNode, found := yamlMappingValue(rootMapping, "directories")
	if !found {
		t.Fatal("directories mapping is missing")
	}
	qualityControlNode, found := yamlMappingValue(directoriesNode, "qc")
	if !found {
		t.Fatal("directories.qc mapping is missing")
	}
	beforeNode, found := yamlMappingValue(qualityControlNode, "before")
	if !found || beforeNode.Value != "/project/work/fastqc_raw" {
		t.Fatalf("directories.qc.before = %#v, want /project/work/fastqc_raw", beforeNode)
	}
	afterNode, found := yamlMappingValue(qualityControlNode, "after")
	if !found || afterNode.Value != "/project/work/fastqc_clean" {
		t.Fatalf("directories.qc.after = %#v, want /project/work/fastqc_clean", afterNode)
	}
	bsmapNode, found := yamlMappingValue(directoriesNode, "bsmap")
	if !found {
		t.Fatal("directories.bsmap mapping is missing")
	}
	bsmapMainNode, found := yamlMappingValue(bsmapNode, "main")
	if !found || bsmapMainNode.Value != "" {
		t.Fatalf("directories.bsmap.main = %#v, want empty fixture value", bsmapMainNode)
	}
	qualimapDirectoryNode, found := yamlMappingValue(directoriesNode, "qualimap")
	if !found || qualimapDirectoryNode.Value != "/project/work/qualimap" {
		t.Fatalf("directories.qualimap = %#v, want /project/work/qualimap", qualimapDirectoryNode)
	}
	workDirectoryNode, found := yamlMappingValue(directoriesNode, "work")
	if !found || workDirectoryNode.Value != "/project/work" {
		t.Fatalf("directories.work = %#v, want /project/work", workDirectoryNode)
	}
	selfConfigDirectoryNode, found := yamlMappingValue(directoriesNode, "selfconfig")
	if !found || selfConfigDirectoryNode.Value != stateDirectory {
		t.Fatalf("directories.selfconfig = %#v, want %s", selfConfigDirectoryNode, stateDirectory)
	}
	qualityControlToolConfigNode, found := yamlMappingValue(directoriesNode, "qctb_config")
	expectedQCTBConfigurationPath := filepath.Join(snapshot.Paths.RunRoot, "run.yaml")
	if !found || qualityControlToolConfigNode.Value != expectedQCTBConfigurationPath {
		t.Fatalf("directories.qctb_config = %#v, want %s", qualityControlToolConfigNode, expectedQCTBConfigurationPath)
	}
	methylationCallDirectoryNode, found := yamlMappingValue(directoriesNode, "methylation_call")
	if !found || methylationCallDirectoryNode.Value != "/project/work/methylation" {
		t.Fatalf("directories.methylation_call = %#v, want /project/work/methylation", methylationCallDirectoryNode)
	}
	referenceNode, found := yamlMappingValue(rootMapping, "reference")
	if !found {
		t.Fatal("reference mapping is missing")
	}
	referenceIndicesNode, found := yamlMappingValue(referenceNode, "indices")
	if !found {
		t.Fatal("reference.indices mapping is missing")
	}
	genomeNode, found := yamlMappingValue(referenceIndicesNode, "genome")
	if !found || len(genomeNode.Content) != 1 || genomeNode.Content[0].Value != "/refs/hg19/bismark/genome" {
		t.Fatalf("reference.indices.genome = %#v, want [/refs/hg19/bismark/genome]", genomeNode)
	}
}
