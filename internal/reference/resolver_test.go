package reference

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

const fixtureDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

func TestResolveMatchingLockSucceeds(t *testing.T) {
	referenceRoot := t.TempDir()
	manifestDigest := writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", true)
	lock := configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References: map[string]configv1.LockedReference{
			"primary": {ID: "hg38", Release: "GRCh38.p14", ManifestDigest: manifestDigest},
		},
	}
	resolver := Resolver{RegistryRoot: referenceRoot, Lock: lock}
	resolved, err := resolver.Resolve(configv1.ReferenceRolePrimary, "hg38@GRCh38.p14", configv1.ScenarioRRBS)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != "hg38" || resolved.Release != "GRCh38.p14" || resolved.Organism != "Homo sapiens" || resolved.Role != configv1.ReferenceRolePrimary {
		t.Fatalf("unexpected resolved reference: %+v", resolved)
	}
	if resolved.Fasta.SHA256 != ComputeDigest(">chr1\nACGT\n") {
		t.Fatalf("unexpected fasta digest: %q", resolved.Fasta.SHA256)
	}
	if resolved.ManifestDigest != manifestDigest {
		t.Fatalf("unexpected manifest digest: %q", resolved.ManifestDigest)
	}
	if resolved.RegistryRoot == "" {
		t.Fatal("resolved reference must include registry root")
	}
}

func TestVerifyReleaseIdentityRejectsMissingDeclaredAsset(t *testing.T) {
	referenceRoot := t.TempDir()
	manifestDigest := writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", true)
	releaseRoot := filepath.Join(referenceRoot, "genomes", "hg38", "GRCh38.p14")
	if err := os.Remove(filepath.Join(releaseRoot, "fasta", "genome.fa.gz")); err != nil {
		t.Fatal(err)
	}

	err := VerifyReleaseIdentity(releaseRoot, manifestDigest)
	if err == nil || !strings.Contains(err.Error(), "declared asset") {
		t.Fatalf("expected missing declared asset identity failure, got %v", err)
	}
}

func TestResolveAliasUsesCanonicalReleaseWithoutDuplicatingAssets(t *testing.T) {
	referenceRoot := t.TempDir()
	writeReferenceFixture(t, referenceRoot, "mm10", "GRCm38", false)
	releaseRoot := filepath.Join(referenceRoot, "genomes", "mm10", "GRCm38")
	definitionPath := filepath.Join(releaseRoot, "reference.yaml")
	definitionData, err := os.ReadFile(definitionPath)
	if err != nil {
		t.Fatal(err)
	}
	aliasedDefinition := strings.Replace(string(definitionData), "  release: GRCm38\n", "  release: GRCm38\n  aliases: [mm38]\n", 1)
	if err := os.WriteFile(definitionPath, []byte(aliasedDefinition), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := BuildManifest(releaseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Write(""); err != nil {
		t.Fatal(err)
	}
	manifestData, err := os.ReadFile(filepath.Join(releaseRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := ComputeDigest(string(manifestData))
	resolver := Resolver{
		RegistryRoot: referenceRoot,
		Lock: configv1.ReferencesLock{
			SchemaVersion: configv1.ReferencesLockSchemaVersion,
			References: map[string]configv1.LockedReference{
				"primary": {ID: "mm38", Release: "GRCm38", ManifestDigest: manifestDigest},
			},
		},
	}
	resolved, err := resolver.Resolve(configv1.ReferenceRolePrimary, "mm38@GRCm38", configv1.ScenarioRRBS)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != "mm38" {
		t.Fatalf("expected requested alias to remain visible in the resolved selection, got %q", resolved.ID)
	}
	if resolved.RegistryRoot != releaseRoot {
		t.Fatalf("expected mm38 alias to reuse the canonical mm10 release root %q, got %q", releaseRoot, resolved.RegistryRoot)
	}
}

func TestVerifyReleaseRejectsReferenceDefinitionDriftNotReflectedInManifestDigest(t *testing.T) {
	referenceRoot := t.TempDir()
	manifestDigest := writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", true)
	releaseRoot := filepath.Join(referenceRoot, "genomes", "hg38", "GRCh38.p14")
	definitionPath := filepath.Join(releaseRoot, "reference.yaml")
	definitionData, err := os.ReadFile(definitionPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(definitionPath, append(definitionData, []byte("# altered after publication\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := VerifyRelease(releaseRoot, manifestDigest)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed {
		t.Fatalf("expected reference.yaml drift to fail manifest verification: %#v", report)
	}
	if len(report.Issues) == 0 || report.Issues[0].Path != "reference.yaml" {
		t.Fatalf("expected reference.yaml manifest issue, got %#v", report.Issues)
	}
}

func TestVerifyReleaseRejectsUntrackedIndexFile(t *testing.T) {
	referenceRoot := t.TempDir()
	manifestDigest := writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", true)
	releaseRoot := filepath.Join(referenceRoot, "genomes", "hg38", "GRCh38.p14")
	writeFile(t, filepath.Join(releaseRoot, "indexes", "bismark", "untracked.bin"), "untracked\n")

	report, err := VerifyRelease(releaseRoot, manifestDigest)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed {
		t.Fatalf("expected untracked index file to fail manifest verification: %#v", report)
	}
	foundUntrackedFile := false
	for _, issue := range report.Issues {
		if issue.Path == "indexes/bismark/untracked.bin" {
			foundUntrackedFile = true
			break
		}
	}
	if !foundUntrackedFile {
		t.Fatalf("expected untracked index file issue, got %#v", report.Issues)
	}
}

func TestResolveRejectsLockIDMismatch(t *testing.T) {
	referenceRoot := t.TempDir()
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	lock := configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References: map[string]configv1.LockedReference{
			"primary": {ID: "mm10", Release: "GRCm39", ManifestDigest: fixtureDigest},
		},
	}
	resolver := Resolver{RegistryRoot: referenceRoot, Lock: lock}
	_, err := resolver.Resolve(configv1.ReferenceRolePrimary, "mm10@GRCm39", configv1.ScenarioRRBS)
	if err == nil {
		t.Fatal("expected lock mismatch to fail when lock has mm10 but selection is hg38")
	}
}

func TestResolveRejectsLockReleaseMismatch(t *testing.T) {
	referenceRoot := t.TempDir()
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	lock := configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References: map[string]configv1.LockedReference{
			"primary": {ID: "hg38", Release: "GRCh38.p13", ManifestDigest: fixtureDigest},
		},
	}
	resolver := Resolver{RegistryRoot: referenceRoot, Lock: lock}
	_, err := resolver.Resolve(configv1.ReferenceRolePrimary, "hg38@GRCh38.p13", configv1.ScenarioRRBS)
	if err == nil {
		t.Fatal("expected lock release mismatch to fail")
	}
}

func TestResolveRejectsLockMissingRole(t *testing.T) {
	referenceRoot := t.TempDir()
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	lock := configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References:    map[string]configv1.LockedReference{},
	}
	resolver := Resolver{RegistryRoot: referenceRoot, Lock: lock}
	_, err := resolver.Resolve(configv1.ReferenceRolePrimary, "hg38@GRCh38.p14", configv1.ScenarioRRBS)
	if err == nil {
		t.Fatal("expected missing lock entry to fail")
	}
}

func TestResolveRejectsScenarioIncompatibility(t *testing.T) {
	referenceRoot := t.TempDir()
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	lock := configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References: map[string]configv1.LockedReference{
			"primary": {ID: "hg38", Release: "GRCh38.p14", ManifestDigest: fixtureDigest},
		},
	}
	resolver := Resolver{RegistryRoot: referenceRoot, Lock: lock}
	_, err := resolver.Resolve(configv1.ReferenceRolePrimary, "hg38@GRCh38.p14", configv1.ScenarioRNASeq)
	if err == nil {
		t.Fatal("expected scenario incompatibility to fail (fixture only supports rrbs)")
	}
}

func TestResolveRejectsReferenceDefinitionIdentityMismatch(t *testing.T) {
	referenceRoot := t.TempDir()
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	lock := configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References: map[string]configv1.LockedReference{
			"primary": {ID: "hg38", Release: "GRCh38.p14", ManifestDigest: fixtureDigest},
		},
	}
	resolver := Resolver{RegistryRoot: referenceRoot, Lock: lock}
	_, err := resolver.Resolve(configv1.ReferenceRolePrimary, "hg38@GRCh38.p14", configv1.ScenarioBSPDX)
	if err == nil {
		t.Fatal("expected reference definition scenario mismatch for bs-pdx to fail")
	}
}

func TestResolveOverrideComputesFreshManifestDigest(t *testing.T) {
	referenceRoot := t.TempDir()
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", true)
	lock := configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References: map[string]configv1.LockedReference{
			"primary": {ID: "hg38", Release: "GRCh38.p14", ManifestDigest: fixtureDigest},
		},
	}
	resolver := Resolver{RegistryRoot: referenceRoot, Lock: lock}
	resolved, err := resolver.ResolveOverride(configv1.ReferenceRolePrimary, "hg38@GRCh38.p14", configv1.ScenarioRRBS)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ManifestDigest == fixtureDigest {
		t.Fatalf("ResolveOverride should compute a fresh manifest digest from the actual manifest.json file, but got the lock fixture digest %q", resolved.ManifestDigest)
	}
	if len(resolved.ManifestDigest) != len(fixtureDigest) || resolved.ManifestDigest[:7] != "sha256:" {
		t.Fatalf("unexpected manifest digest format: %q", resolved.ManifestDigest)
	}
}

func TestResolveOverrideRejectsLockMismatch(t *testing.T) {
	referenceRoot := t.TempDir()
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", true)
	lock := configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References: map[string]configv1.LockedReference{
			"primary": {ID: "mm10", Release: "GRCm39", ManifestDigest: fixtureDigest},
		},
	}
	resolver := Resolver{RegistryRoot: referenceRoot, Lock: lock}
	_, err := resolver.ResolveOverride(configv1.ReferenceRolePrimary, "mm10@GRCm39", configv1.ScenarioRRBS)
	if err == nil {
		t.Fatal("expected ResolveOverride with lock mismatch to fail")
	}
}

func TestResolveOverrideRejectsMissingManifest(t *testing.T) {
	referenceRoot := t.TempDir()
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	lock := configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References: map[string]configv1.LockedReference{
			"primary": {ID: "hg38", Release: "GRCh38.p14", ManifestDigest: fixtureDigest},
		},
	}
	resolver := Resolver{RegistryRoot: referenceRoot, Lock: lock}
	_, err := resolver.ResolveOverride(configv1.ReferenceRolePrimary, "hg38@GRCh38.p14", configv1.ScenarioRRBS)
	if err == nil {
		t.Fatal("expected ResolveOverride to fail when manifest.json is missing")
	}
}

func writeReferenceFixture(t *testing.T, referenceRoot string, id string, release string, includeManifest bool) string {
	t.Helper()
	releaseRoot := filepath.Join(referenceRoot, "genomes", id, release)
	fastaContent := ">chr1\nACGT\n"
	fastaDigest := ComputeDigest(fastaContent)
	indexContent := "index\n"
	indexDigest := ComputeDigest(indexContent)
	indexManifestDigest := ComputeDigest("index.bin:" + indexDigest)
	writeFile(t, filepath.Join(releaseRoot, "fasta", "genome.fa.gz"), fastaContent)
	writeFile(t, filepath.Join(releaseRoot, "fasta", "genome.fa.gz.fai"), "chr1\t4\t0\t4\t5\n")
	writeFile(t, filepath.Join(releaseRoot, "indexes", "bismark", "index.bin"), indexContent)
	definition := fmt.Sprintf(`schema_version: otter.reference/v1
reference:
  id: %s
  release: %s
  organism: Homo sapiens
  assembly: test
assets:
  fasta:
    path: fasta/genome.fa.gz
    sha256: %s
    size_bytes: %d
    fai: fasta/genome.fa.gz.fai
  indexes:
    - type: bismark
      path: indexes/bismark
      reference_fasta_sha256: %s
      tool: bismark
      tool_version: test
      manifest_sha256: %s
compatibility:
  scenarios: [rrbs]
`, id, release, fastaDigest, len(fastaContent), fastaDigest, indexManifestDigest)
	writeFile(t, filepath.Join(releaseRoot, "reference.yaml"), definition)
	if !includeManifest {
		return ""
	}
	report, err := BuildManifest(releaseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Write(""); err != nil {
		t.Fatal(err)
	}
	manifestData, err := os.ReadFile(filepath.Join(releaseRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	return ComputeDigest(string(manifestData))
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
