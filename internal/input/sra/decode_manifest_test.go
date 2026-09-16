package sra

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

func TestBuildProvenanceManifestConvertsVerifiedDecodeManifest(t *testing.T) {
	acquisitionRoot := t.TempDir()
	archivePath := writeFixtureFile(t, acquisitionRoot, "archive/SRR31480456.sra", "archive")
	r1Path := writeFixtureFile(t, acquisitionRoot, "decoded/R1.fastq.gz", "r1")
	r2Path := writeFixtureFile(t, acquisitionRoot, "decoded/R2.fastq.gz", "r2")
	decodeManifestPath := writeDecodeManifestFixture(t, acquisitionRoot, archivePath, r1Path, r2Path)

	provenanceManifest, err := BuildProvenanceManifest(decodeManifestPath, ProvenanceOptions{
		AcquisitionID: "sra-20260815T070000Z-rrbs-srr31480456",
		CreatedAt:     time.Date(2026, time.August, 15, 7, 0, 0, 0, time.UTC),
		Scenario:      configv1.ScenarioRRBS,
		Reference: configv1.SRAAcquisitionReference{Primary: &configv1.SRAReferenceSelection{
			Role:           configv1.ReferenceRolePrimary,
			ID:             "hg19",
			Release:        "GRCh37.p13-gencode-v19",
			ManifestSHA256: "sha256:33dfd7d4ec0a90c6e11fdc45d02b2d4e9b6d82e4a607148d1ab467a0e555accc",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if provenanceManifest.Acquisition.Root != acquisitionRoot {
		t.Fatalf("acquisition root = %q, want %q", provenanceManifest.Acquisition.Root, acquisitionRoot)
	}
	entry := provenanceManifest.Entries[0]
	if entry.Accession != "SRR31480456" || entry.Scenario != configv1.ScenarioRRBS {
		t.Fatalf("unexpected acquisition entry: %#v", entry)
	}
	if entry.Output.R1.Path != r1Path || entry.Output.R2.Path != r2Path {
		t.Fatalf("provenance did not preserve decoded FASTQ paths: %#v", entry.Output)
	}
	if entry.Tool.Version != "3.1.1" {
		t.Fatalf("sra-tools version = %q, want 3.1.1", entry.Tool.Version)
	}
	if err := VerifyManifestFiles(provenanceManifest); err != nil {
		t.Fatal(err)
	}
}

func TestBuildProvenanceManifestRejectsInvalidDecodeSchema(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "sra-acquisition-decode.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schema_version":"invalid"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := BuildProvenanceManifest(manifestPath, ProvenanceOptions{})
	if err == nil {
		t.Fatal("expected invalid decode manifest to fail")
	}
}

func writeDecodeManifestFixture(t *testing.T, acquisitionRoot string, archivePath string, r1Path string, r2Path string) string {
	t.Helper()
	manifest := DecodeManifest{
		SchemaVersion: DecodeManifestSchemaVersion,
		Accession:     "SRR31480456",
		Archive: configv1.SRAArchive{
			Path:   archivePath,
			SHA256: fileDigest(t, archivePath),
			Bytes:  fileSize(t, archivePath),
			MD5:    "0123456789abcdef0123456789abcdef",
		},
		Outputs: DecodeManifestOutputs{
			R1: configv1.SRAAcquiredFASTQ{Path: r1Path, SHA256: fileDigest(t, r1Path), Bytes: fileSize(t, r1Path), PairedRecordCount: 100},
			R2: configv1.SRAAcquiredFASTQ{Path: r2Path, SHA256: fileDigest(t, r2Path), Bytes: fileSize(t, r2Path), PairedRecordCount: 100},
		},
		Tool: DecodeManifestTool{
			Name:               "sra-tools",
			FasterqDump:        "/tools/fasterq-dump",
			FasterqDumpVersion: "fasterq-dump : 3.1.1",
			Compression:        "/tools/pigz -n -p 8",
			PigzVersion:        "pigz 2.8",
		},
	}
	contents, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(acquisitionRoot, "decoded", "sra-acquisition-decode.json")
	if err := os.WriteFile(manifestPath, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	return manifestPath
}
