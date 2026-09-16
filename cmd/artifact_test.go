package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otterlab-bio/otter/internal/artifact"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	runstate "github.com/otterlab-bio/otter/internal/run"
	"github.com/spf13/cobra"
)

func TestRunArtifactPublishBuildsImmutableManifestFromDeclarations(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")
	writeConfigResolveProjectFixture(t, projectRoot, referenceRoot)
	output := executeCommand(t, rootCmd, "config", "resolve",
		"--project", filepath.Join(projectRoot, "project.yaml"),
		"--reference-root", referenceRoot,
		"--backend", "local",
	)
	if output.exitCode != 0 {
		t.Fatalf("config resolve failed: %s\n%s", output.stdout, output.stderr)
	}
	runEntries, err := os.ReadDir(filepath.Join(projectRoot, "runs"))
	if err != nil || len(runEntries) != 1 {
		t.Fatalf("expected exactly one resolved run: %v", err)
	}
	runSnapshotPath := filepath.Join(projectRoot, "runs", runEntries[0].Name(), "run.yaml")
	snapshot, err := configv1.LoadRunSnapshot(runSnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(snapshot.Paths.Results, "qc", "summary.json")
	artifactContent := []byte("{\"passed\":true}\n")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, artifactContent, 0o644); err != nil {
		t.Fatal(err)
	}
	declarations := artifact.DeclarationDocument{
		SchemaVersion: artifact.DeclarationSchemaVersion,
		Artifacts: []artifact.Declaration{{
			ID:        "qc-summary",
			Path:      "qc/summary.json",
			MediaType: "application/json",
			Schema:    "otter.qc-summary/v1",
			Comparison: artifact.Comparison{
				Tier:       artifact.ComparisonTierStructural,
				Comparator: "json-structure/v1",
			},
		}},
	}
	declarationBytes, err := json.Marshal(declarations)
	if err != nil {
		t.Fatal(err)
	}
	declarationsPath := filepath.Join(snapshot.Paths.Work, "artifact-declarations.json")
	if err := os.WriteFile(declarationsPath, declarationBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	command := &cobra.Command{}
	commandOutput := &testWriter{}
	command.SetOut(commandOutput)
	if err := runArtifactPublish(command, runSnapshotPath, declarationsPath); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName)
	manifest, err := artifact.Load(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Artifacts[0].Checksum != testArtifactChecksum(artifactContent) {
		t.Fatalf("unexpected published artifact checksum: %#v", manifest.Artifacts)
	}
	if err := runArtifactPublish(command, runSnapshotPath, declarationsPath); err == nil || !strings.Contains(err.Error(), "immutable artifact manifest") {
		t.Fatalf("expected repeat publication rejection, got %v", err)
	}
}

func TestRunArtifactVerifyRevalidatesSnapshotManifestAndArtifacts(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")
	writeConfigResolveProjectFixture(t, projectRoot, referenceRoot)
	output := executeCommand(t, rootCmd, "config", "resolve",
		"--project", filepath.Join(projectRoot, "project.yaml"),
		"--reference-root", referenceRoot,
		"--backend", "local",
	)
	if output.exitCode != 0 {
		t.Fatalf("config resolve failed: %s\n%s", output.stdout, output.stderr)
	}
	runEntries, err := os.ReadDir(filepath.Join(projectRoot, "runs"))
	if err != nil || len(runEntries) != 1 {
		t.Fatalf("expected exactly one resolved run: %v", err)
	}
	runSnapshotPath := filepath.Join(projectRoot, "runs", runEntries[0].Name(), "run.yaml")
	snapshot, err := configv1.LoadRunSnapshot(runSnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(snapshot.Paths.Results, "qc", "summary.json")
	artifactContent := []byte("{\"passed\":true}\n")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, artifactContent, 0o644); err != nil {
		t.Fatal(err)
	}
	runSnapshotDigest, err := runstate.DigestFile(runSnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := artifact.Manifest{
		SchemaVersion:     artifact.SchemaVersion,
		RunID:             snapshot.Run.ID,
		RunSnapshotDigest: runSnapshotDigest,
		Scenario:          snapshot.Workflow.Scenario,
		Toolchain:         snapshot.Workflow.Toolchain,
		Executor:          snapshot.Execution.Executor.Value,
		Backend:           snapshot.Execution.Backend.Value,
		Artifacts: []artifact.Entry{{
			ID:        "qc-summary",
			Path:      "qc/summary.json",
			MediaType: "application/json",
			Schema:    "otter.qc-summary/v1",
			Checksum:  testArtifactChecksum(artifactContent),
			Comparison: artifact.Comparison{
				Tier:       artifact.ComparisonTierStructural,
				Comparator: "json-structure/v1",
			},
		}},
	}
	if err := artifact.Write(filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName), manifest); err != nil {
		t.Fatal(err)
	}
	command := &cobra.Command{}
	commandOutput := &testWriter{}
	command.SetOut(commandOutput)
	if err := runArtifactVerify(command, runSnapshotPath, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(commandOutput.String(), `"passed": true`) {
		t.Fatalf("expected successful verification report, got %q", commandOutput.String())
	}
}

func TestVerifyArtifactManifestIdentityBindsManifestToRunSnapshot(t *testing.T) {
	runSnapshotPath := filepath.Join(t.TempDir(), "run.yaml")
	runSnapshotContent := []byte("immutable run snapshot\n")
	if err := os.WriteFile(runSnapshotPath, runSnapshotContent, 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := configv1.RunSnapshot{
		Run: configv1.RunMetadata{ID: "run-20260727T010203Z-abcdef"},
		Workflow: configv1.ResolvedWorkflow{
			Scenario:  configv1.ScenarioRRBS,
			Toolchain: configv1.ToolchainModern,
		},
		Execution: configv1.ResolvedExecution{
			Executor: configv1.ResolvedExecutor{Value: configv1.ExecutorCraftmake},
			Backend:  configv1.ResolvedBackend{Value: configv1.BackendSlurm},
		},
	}
	manifest := artifact.Manifest{
		RunID:             snapshot.Run.ID,
		RunSnapshotDigest: testArtifactChecksum(runSnapshotContent),
		Scenario:          snapshot.Workflow.Scenario,
		Toolchain:         snapshot.Workflow.Toolchain,
		Executor:          snapshot.Execution.Executor.Value,
		Backend:           snapshot.Execution.Backend.Value,
	}
	if err := verifyArtifactManifestIdentity(snapshot, runSnapshotPath, manifest); err != nil {
		t.Fatalf("expected matching artifact identity, got %v", err)
	}

	manifest.RunSnapshotDigest = testArtifactChecksum([]byte("different snapshot"))
	err := verifyArtifactManifestIdentity(snapshot, runSnapshotPath, manifest)
	if err == nil || !strings.Contains(err.Error(), "run_snapshot_digest mismatch") {
		t.Fatalf("expected snapshot digest mismatch, got %v", err)
	}
}

func TestVerifyArtifactManifestIdentityRejectsExecutionIdentityMismatch(t *testing.T) {
	runSnapshotPath := filepath.Join(t.TempDir(), "run.yaml")
	runSnapshotContent := []byte("immutable run snapshot\n")
	if err := os.WriteFile(runSnapshotPath, runSnapshotContent, 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := configv1.RunSnapshot{
		Run: configv1.RunMetadata{ID: "run-20260727T010203Z-abcdef"},
		Workflow: configv1.ResolvedWorkflow{
			Scenario:  configv1.ScenarioRRBS,
			Toolchain: configv1.ToolchainModern,
		},
		Execution: configv1.ResolvedExecution{
			Executor: configv1.ResolvedExecutor{Value: configv1.ExecutorCraftmake},
			Backend:  configv1.ResolvedBackend{Value: configv1.BackendSlurm},
		},
	}
	manifest := artifact.Manifest{
		RunID:             snapshot.Run.ID,
		RunSnapshotDigest: testArtifactChecksum(runSnapshotContent),
		Scenario:          configv1.ScenarioWGBS,
		Toolchain:         snapshot.Workflow.Toolchain,
		Executor:          snapshot.Execution.Executor.Value,
		Backend:           snapshot.Execution.Backend.Value,
	}
	err := verifyArtifactManifestIdentity(snapshot, runSnapshotPath, manifest)
	if err == nil || !strings.Contains(err.Error(), "execution identity") {
		t.Fatalf("expected execution identity mismatch, got %v", err)
	}
}

func testArtifactChecksum(content []byte) string {
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}
