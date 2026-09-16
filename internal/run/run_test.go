package run

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"gopkg.in/yaml.v3"
)

func TestGenerateIDUsesUTCAndLowercaseLetters(t *testing.T) {
	createdAt := time.Date(2026, 7, 26, 1, 32, 45, 0, time.FixedZone("offset", 8*60*60))
	runID, err := generateID(createdAt, bytes.NewReader([]byte{0, 1, 2, 3, 4, 5}))
	if err != nil {
		t.Fatal(err)
	}
	if runID != "run-20260725T173245Z-abcdef" {
		t.Fatalf("unexpected run ID: %s", runID)
	}
	parsedAt, err := CreatedAtFromID(runID)
	if err != nil {
		t.Fatal(err)
	}
	if parsedAt.Format(time.RFC3339) != "2026-07-25T17:32:45Z" {
		t.Fatalf("unexpected parsed time: %s", parsedAt)
	}
}

func assertCreatedAtIsYAMLString(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	root := document.Content[0]
	for index := 0; index < len(root.Content); index += 2 {
		if root.Content[index].Value != "run" {
			continue
		}
		runNode := root.Content[index+1]
		for runIndex := 0; runIndex < len(runNode.Content); runIndex += 2 {
			if runNode.Content[runIndex].Value == "created_at" {
				if runNode.Content[runIndex+1].Tag != "!!str" {
					t.Fatalf("created_at YAML tag = %s, want !!str", runNode.Content[runIndex+1].Tag)
				}
				return
			}
		}
	}
	t.Fatal("created_at node not found")
}

func TestDigestPathsRejectsMissingRequiredAssets(t *testing.T) {
	requiredAssetPath := filepath.Join(t.TempDir(), "missing-workflow-asset")
	if _, err := DigestPaths([]string{requiredAssetPath}); err == nil {
		t.Fatal("expected a missing required workflow asset to fail closed")
	}
}

func TestWriteSnapshotIsImmutableAndDetectsDigestDrift(t *testing.T) {
	projectRoot := t.TempDir()
	runID := "run-20260726T013245Z-abcdef"
	createdAt, err := CreatedAtFromID(runID)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := CreateDirectoryWithID(projectRoot, runID, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + string(bytes.Repeat([]byte{'1'}, 64))
	snapshot := configv1.RunSnapshot{
		SchemaVersion: configv1.RunSchemaVersion,
		Run:           configv1.RunMetadata{ID: runID, CreatedAt: configv1.Timestamp(createdAt.Format(time.RFC3339)), Immutable: true},
		Project:       configv1.ResolvedProject{ID: "cohort", Root: projectRoot},
		Workflow:      configv1.ResolvedWorkflow{Scenario: configv1.ScenarioRRBS, Toolchain: configv1.ToolchainModern, AssetRoot: filepath.Join(projectRoot, "workflows")},
		Execution: configv1.ResolvedExecution{
			Executor: configv1.ResolvedExecutor{Value: configv1.ExecutorCraftmake, Source: configv1.SourceDefault},
			Backend:  configv1.ResolvedBackend{Value: configv1.BackendLocal, Source: configv1.SourceCLI, Evidence: configv1.BackendEvidence{Reason: "test"}},
			Site:     configv1.ResolvedString{Value: "test", Source: configv1.SourceCLI},
		},
		Samples: []configv1.SampleRecord{{ID: "S01", R1: "/data/S01_R1.fastq.gz", R2: "/data/S01_R2.fastq.gz"}},
		References: configv1.ResolvedReferences{Resolved: []configv1.ResolvedReference{{
			Role: configv1.ReferenceRolePrimary, ID: "hg38", Release: "GRCh38", RegistryRoot: "/refs/hg38", ManifestDigest: digest,
			Fasta: configv1.ResolvedAsset{Type: "fasta", Path: "/refs/hg38/genome.fa", SHA256: digest},
		}}},
		Paths:   configv1.RunPaths{RunRoot: directory.Root, Work: filepath.Join(directory.Root, "work"), Results: filepath.Join(directory.Root, "results"), Logs: filepath.Join(directory.Root, "logs"), State: filepath.Join(directory.Root, "state"), Metrics: filepath.Join(directory.Root, "metrics")},
		Digests: configv1.RunDigests{Project: digest, Samples: digest, WorkflowAssets: digest},
	}
	path, err := WriteSnapshot(directory, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	assertCreatedAtIsYAMLString(t, path)
	if _, err := os.Stat(filepath.Join(directory.Root, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSnapshot(directory, snapshot); err == nil {
		t.Fatal("expected immutable snapshot publication to reject overwrite")
	}
	if err := VerifySnapshotDigests(path, snapshot.Digests); err != nil {
		t.Fatal(err)
	}
	drifted := snapshot.Digests
	drifted.Project = "sha256:" + string(bytes.Repeat([]byte{'2'}, 64))
	if err := VerifySnapshotDigests(path, drifted); err == nil {
		t.Fatal("expected digest drift rejection")
	}
}
