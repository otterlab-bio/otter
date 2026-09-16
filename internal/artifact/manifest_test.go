package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

func TestWriteLoadAndVerifyManifest(t *testing.T) {
	resultsRoot := t.TempDir()
	artifactPath := filepath.Join(resultsRoot, "methylation", "matrix.h5")
	artifactContent := []byte("methylation-matrix\n")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, artifactContent, 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := validManifest(checksum(artifactContent))
	manifestPath := filepath.Join(resultsRoot, DefaultManifestFileName)
	if err := Write(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}
	loadedManifest, err := Load(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if loadedManifest.Artifacts[0].Path != "methylation/matrix.h5" {
		t.Fatalf("unexpected artifact path: %#v", loadedManifest.Artifacts)
	}
	fileInfo, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm()&0o222 != 0 {
		t.Fatalf("artifact manifest must be read-only, got 0o%o", fileInfo.Mode().Perm())
	}
	report, err := Verify(resultsRoot, loadedManifest)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || len(report.Issues) != 0 {
		t.Fatalf("expected verified manifest, got %#v", report)
	}
}

func TestLoadDeclarationsRejectsUnversionedOrInvalidDocuments(t *testing.T) {
	declarationsPath := filepath.Join(t.TempDir(), "declarations.json")
	validDocument := DeclarationDocument{
		SchemaVersion: DeclarationSchemaVersion,
		Artifacts: []Declaration{{
			ID:        "qc-summary",
			Path:      "qc/summary.json",
			MediaType: "application/json",
			Schema:    "otter.qc-summary/v1",
			Comparison: Comparison{
				Tier:       ComparisonTierStructural,
				Comparator: "json-structure/v1",
			},
		}},
	}
	declarationBytes, err := json.Marshal(validDocument)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(declarationsPath, declarationBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDeclarations(declarationsPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(declarationsPath, []byte(`[{"id":"qc-summary"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDeclarations(declarationsPath); err == nil || !strings.Contains(err.Error(), "parse artifact declarations") {
		t.Fatalf("expected unversioned declaration rejection, got %v", err)
	}
}

func TestBuildCalculatesChecksumsFromArtifactDeclarations(t *testing.T) {
	resultsRoot := t.TempDir()
	artifactPath := filepath.Join(resultsRoot, "qc", "summary.json")
	artifactContent := []byte("{\"passed\":true}\n")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, artifactContent, 0o644); err != nil {
		t.Fatal(err)
	}
	identity := PublicationIdentity{
		RunID:             "run-20260727T010203Z-abcdef",
		RunSnapshotDigest: checksum([]byte("run snapshot")),
		Scenario:          configv1.ScenarioRNASeq,
		Toolchain:         configv1.ToolchainLegacyEquivalent,
		Executor:          configv1.ExecutorSnakemake,
		Backend:           configv1.BackendSlurm,
	}
	manifest, err := Build(identity, resultsRoot, []Declaration{{
		ID:        "qc-summary",
		Path:      "qc/summary.json",
		MediaType: "application/json",
		Schema:    "otter.qc-summary/v1",
		Comparison: Comparison{
			Tier:       ComparisonTierStructural,
			Comparator: "json-structure/v1",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Artifacts[0].Checksum != checksum(artifactContent) {
		t.Fatalf("unexpected manifest checksum: %#v", manifest.Artifacts)
	}
	if _, err := Build(identity, resultsRoot, []Declaration{{
		ID:        "missing",
		Path:      "qc/missing.json",
		MediaType: "application/json",
		Schema:    "otter.qc-summary/v1",
		Comparison: Comparison{
			Tier:       ComparisonTierStructural,
			Comparator: "json-structure/v1",
		},
	}}); err == nil || !strings.Contains(err.Error(), "digest artifact declaration") {
		t.Fatalf("expected missing artifact declaration failure, got %v", err)
	}
}

func TestBuildRejectsSymbolicLinkArtifacts(t *testing.T) {
	resultsRoot := t.TempDir()
	outsideArtifactPath := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outsideArtifactPath, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkedArtifactPath := filepath.Join(resultsRoot, "qc", "summary.json")
	if err := os.MkdirAll(filepath.Dir(linkedArtifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideArtifactPath, linkedArtifactPath); err != nil {
		t.Fatal(err)
	}
	identity := PublicationIdentity{
		RunID:             "run-20260727T010203Z-abcdef",
		RunSnapshotDigest: checksum([]byte("run snapshot")),
		Scenario:          configv1.ScenarioRRBS,
		Toolchain:         configv1.ToolchainModern,
		Executor:          configv1.ExecutorCraftmake,
		Backend:           configv1.BackendLocal,
	}
	_, err := Build(identity, resultsRoot, []Declaration{{
		ID:        "qc-summary",
		Path:      "qc/summary.json",
		MediaType: "application/json",
		Schema:    "otter.qc-summary/v1",
		Comparison: Comparison{
			Tier:       ComparisonTierStructural,
			Comparator: "json-structure/v1",
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected symbolic link artifact rejection, got %v", err)
	}
}

func TestWriteRejectsExistingImmutableManifest(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), DefaultManifestFileName)
	manifest := validManifest(checksum([]byte("matrix")))
	if err := Write(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}
	if err := Write(manifestPath, manifest); err == nil || !strings.Contains(err.Error(), "immutable artifact manifest") {
		t.Fatalf("expected immutable overwrite rejection, got %v", err)
	}
}

func TestVerifyReportsModifiedAndMissingArtifacts(t *testing.T) {
	resultsRoot := t.TempDir()
	artifactPath := filepath.Join(resultsRoot, "methylation", "matrix.h5")
	artifactContent := []byte("original\n")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, artifactContent, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := validManifest(checksum(artifactContent))
	manifest.Artifacts = append(manifest.Artifacts, Entry{
		ID:        "qc-summary",
		Path:      "qc/summary.json",
		MediaType: "application/json",
		Schema:    "otter.qc-summary/v1",
		Checksum:  checksum([]byte("missing")),
		Comparison: Comparison{
			Tier:       ComparisonTierStructural,
			Comparator: "json-structure/v1",
		},
	})
	if err := os.WriteFile(artifactPath, []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Verify(resultsRoot, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || len(report.Issues) != 2 {
		t.Fatalf("expected modified and missing artifact issues, got %#v", report)
	}
	if report.Issues[0].ArtifactID != "methylation-matrix" || report.Issues[1].ArtifactID != "qc-summary" {
		t.Fatalf("unexpected verification issues: %#v", report.Issues)
	}
}

func TestValidateRejectsEscapingDuplicateAndIncompleteEntries(t *testing.T) {
	testCases := []struct {
		name     string
		mutate   func(*Manifest)
		expected string
	}{
		{
			name: "escaping path",
			mutate: func(manifest *Manifest) {
				manifest.Artifacts[0].Path = "../outside.txt"
			},
			expected: "must not escape",
		},
		{
			name: "noncanonical separators",
			mutate: func(manifest *Manifest) {
				manifest.Artifacts[0].Path = `methylation\\matrix.h5`
			},
			expected: "slash-separated",
		},
		{
			name: "duplicate artifact id",
			mutate: func(manifest *Manifest) {
				manifest.Artifacts = append(manifest.Artifacts, manifest.Artifacts[0])
				manifest.Artifacts[1].Path = "methylation/other.h5"
			},
			expected: "duplicated",
		},
		{
			name: "scientific comparator required",
			mutate: func(manifest *Manifest) {
				manifest.Artifacts[0].Comparison.Comparator = ""
			},
			expected: "comparator is required",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			manifest := validManifest(checksum([]byte("matrix")))
			testCase.mutate(&manifest)
			err := Validate(manifest)
			if err == nil || !strings.Contains(err.Error(), testCase.expected) {
				t.Fatalf("expected error containing %q, got %v", testCase.expected, err)
			}
		})
	}
}

func TestLoadRejectsUnknownOrConcatenatedJSONValues(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), DefaultManifestFileName)
	manifest := validManifest(checksum([]byte("matrix")))
	if err := Write(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(manifestPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(manifestData, []byte(`{"unexpected":true}`)...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(manifestPath); err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("expected concatenated JSON rejection, got %v", err)
	}
}

func validManifest(artifactChecksum string) Manifest {
	return Manifest{
		SchemaVersion:     SchemaVersion,
		RunID:             "run-20260727T010203Z-abcdef",
		RunSnapshotDigest: checksum([]byte("run snapshot")),
		Scenario:          configv1.ScenarioRRBS,
		Toolchain:         configv1.ToolchainModern,
		Executor:          configv1.ExecutorCraftmake,
		Backend:           configv1.BackendSlurm,
		Artifacts: []Entry{{
			ID:        "methylation-matrix",
			Path:      "methylation/matrix.h5",
			MediaType: "application/x-hdf5",
			Schema:    "methrix-se/v1",
			Checksum:  artifactChecksum,
			Comparison: Comparison{
				Tier:       ComparisonTierScientific,
				Comparator: "methrix-semantics/v1",
			},
		}},
	}
}

func checksum(content []byte) string {
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}
