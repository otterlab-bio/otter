package e2esupport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	refpkg "github.com/otterlab-bio/otter/internal/reference"
)

// TestStubReleaseSatisfiesProductionResolver is the contract that makes the
// whole simulated-registry approach legitimate: a stub release must survive the
// same resolver path a real registry release does.
func TestStubReleaseSatisfiesProductionResolver(t *testing.T) {
	registryRoot := t.TempDir()
	result, err := WriteStubRelease(StubReleaseRequest{
		RegistryRoot: registryRoot,
		ReferenceID:  "hg19",
		Release:      "hg19-legacy",
		Organism:     "Homo sapiens",
		Assembly:     "hg19",
	})
	if err != nil {
		t.Fatalf("WriteStubRelease failed: %v", err)
	}

	// The production resolver must accept it for every scenario the release
	// claims. The default stub carries bismark, bowtie2, and star, so all five
	// scenarios are legitimate.
	resolver := refpkg.Resolver{RegistryRoot: registryRoot}
	for _, scenario := range []configv1.Scenario{
		configv1.ScenarioRRBS,
		configv1.ScenarioWGBS,
		configv1.ScenarioRNASeq,
		configv1.ScenarioBSPDX,
		configv1.ScenarioRNAPDX,
	} {
		resolved, err := resolver.ResolveOverride(configv1.ReferenceRolePrimary, "hg19@hg19-legacy", scenario)
		if err != nil {
			t.Fatalf("stub release did not resolve for scenario %s: %v", scenario, err)
		}
		if resolved.Organism != "Homo sapiens" {
			t.Fatalf("organism was not carried through: %q", resolved.Organism)
		}
		if len(resolved.Indexes) == 0 {
			t.Fatalf("resolved reference has no indexes for scenario %s", scenario)
		}
	}

	// The published manifest digest must be stable for the same content.
	repeatRoot := t.TempDir()
	repeat, err := WriteStubRelease(StubReleaseRequest{
		RegistryRoot: repeatRoot,
		ReferenceID:  "hg19",
		Release:      "hg19-legacy",
		Organism:     "Homo sapiens",
		Assembly:     "hg19",
	})
	if err != nil {
		t.Fatalf("second WriteStubRelease failed: %v", err)
	}
	if repeat.ManifestDigest != result.ManifestDigest {
		t.Fatalf("stub manifest digest is not reproducible:\n%s\n%s", result.ManifestDigest, repeat.ManifestDigest)
	}
}

func TestStubReleaseStarOnlySupportsRNA(t *testing.T) {
	registryRoot := t.TempDir()
	if _, err := WriteStubRelease(StubReleaseRequest{
		RegistryRoot: registryRoot,
		ReferenceID:  "hg38",
		Release:      "GRCh38.p14",
		Organism:     "Homo sapiens",
		Assembly:     "GRCh38",
		Indexes:      []string{"star"},
	}); err != nil {
		t.Fatalf("WriteStubRelease failed: %v", err)
	}
	resolver := refpkg.Resolver{RegistryRoot: registryRoot}
	if _, err := resolver.ResolveOverride(configv1.ReferenceRolePrimary, "hg38@GRCh38.p14", configv1.ScenarioRNASeq); err != nil {
		t.Fatalf("star-only stub should resolve for rnaseq: %v", err)
	}
	if _, err := resolver.ResolveOverride(configv1.ReferenceRolePrimary, "hg38@GRCh38.p14", configv1.ScenarioRRBS); err == nil {
		t.Fatal("star-only stub must not resolve for rrbs")
	}
}

func TestStubReleaseRefusesToOverwrite(t *testing.T) {
	registryRoot := t.TempDir()
	request := StubReleaseRequest{
		RegistryRoot: registryRoot,
		ReferenceID:  "hg19",
		Release:      "hg19-legacy",
		Organism:     "Homo sapiens",
		Assembly:     "hg19",
	}
	if _, err := WriteStubRelease(request); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteStubRelease(request); err == nil {
		t.Fatal("expected a second identical release to be refused")
	}
}

func TestStubReleaseRejectsUnknownIndex(t *testing.T) {
	_, err := WriteStubRelease(StubReleaseRequest{
		RegistryRoot: t.TempDir(),
		ReferenceID:  "hg19",
		Release:      "x",
		Organism:     "Homo sapiens",
		Assembly:     "hg19",
		Indexes:      []string{"bwa"},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported stub index type") {
		t.Fatalf("expected an unsupported-index error, got %v", err)
	}
}

// TestStubReleaseUsesRealReferenceBuildLayout pins the on-disk layout to the
// one "otter reference build" produces, because "otter create" reads it.
func TestStubReleaseUsesRealReferenceBuildLayout(t *testing.T) {
	registryRoot := t.TempDir()
	result, err := WriteStubRelease(StubReleaseRequest{
		RegistryRoot: registryRoot,
		ReferenceID:  "mm10",
		Release:      "GRCm38",
		Organism:     "Mus musculus",
		Assembly:     "GRCm38",
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedRoot := filepath.Join(registryRoot, "genomes", "mm10", "GRCm38")
	if result.ReleaseRoot != expectedRoot {
		t.Fatalf("release root = %q, want %q", result.ReleaseRoot, expectedRoot)
	}
	for _, relativePath := range []string{
		"reference.yaml",
		"manifest.json",
		"fasta/mm10.fa",
		"fasta/mm10.fa.fai",
		"annotations/mm10.gtf",
		"indexes/bismark/genome/Bisulfite_Genome/CT_conversion/BS_CT.1.bt2",
		"indexes/bowtie2/genome.1.bt2",
		"indexes/star/Genome",
	} {
		if _, err := os.Stat(filepath.Join(expectedRoot, filepath.FromSlash(relativePath))); err != nil {
			t.Errorf("expected layout entry %s: %v", relativePath, err)
		}
	}
}
