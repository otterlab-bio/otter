package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	inputSRA "github.com/otterlab-bio/otter/internal/input/sra"
)

func TestAcquisitionPublishCreatesVerifiedReadOnlyProvenance(t *testing.T) {
	acquisitionRoot := t.TempDir()
	decodeManifestPath := writeCommandDecodeManifest(t, acquisitionRoot)
	outputPath := filepath.Join(acquisitionRoot, "provenance", "sra-acquisition.json")
	command := newAcquisitionPublishCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{
		"--decode-manifest", decodeManifestPath,
		"--output", outputPath,
		"--scenario", "rrbs",
		"--acquisition-id", "sra-20260815T080000Z-rrbs-srr31480456",
		"--primary-id", "hg19",
		"--primary-release", "GRCh37.p13-gencode-v19",
		"--primary-manifest-sha256", "sha256:33dfd7d4ec0a90c6e11fdc45d02b2d4e9b6d82e4a607148d1ab467a0e555accc",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if output.String() != outputPath+"\n" {
		t.Fatalf("unexpected command output %q", output.String())
	}
	manifest, err := inputSRA.LoadManifest(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Entries[0].Reference.Primary == nil || manifest.Entries[0].Reference.Primary.ID != "hg19" {
		t.Fatalf("unexpected primary reference binding: %#v", manifest.Entries[0].Reference)
	}
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0o444 {
		t.Fatalf("manifest permissions = %o, want 444", fileInfo.Mode().Perm())
	}
	if err := command.Execute(); err == nil {
		t.Fatal("expected create-only publication to reject overwrite")
	}
}

func TestAcquisitionPublishRequiresPDXGraftAndHostReferences(t *testing.T) {
	acquisitionRoot := t.TempDir()
	decodeManifestPath := writeCommandDecodeManifest(t, acquisitionRoot)
	command := newAcquisitionPublishCommand()
	command.SetArgs([]string{
		"--decode-manifest", decodeManifestPath,
		"--output", filepath.Join(acquisitionRoot, "sra-acquisition.json"),
		"--scenario", "bs-pdx",
		"--graft-id", "hg38",
		"--graft-release", "GRCh38-gencode-v44",
		"--graft-manifest-sha256", "sha256:bda77ed9591ec4b9b6c4fadf032e9bfeafec0a6e03e7854e4d706e4f8909a132",
	})
	if err := command.Execute(); err == nil {
		t.Fatal("expected missing host reference to fail")
	}
}

func writeCommandDecodeManifest(t *testing.T, acquisitionRoot string) string {
	t.Helper()
	archivePath := writeCommandFile(t, acquisitionRoot, "archive/SRR31480456.sra", "archive")
	r1Path := writeCommandFile(t, acquisitionRoot, "decoded/R1.fastq.gz", "r1")
	r2Path := writeCommandFile(t, acquisitionRoot, "decoded/R2.fastq.gz", "r2")
	manifest := inputSRA.DecodeManifest{
		SchemaVersion: inputSRA.DecodeManifestSchemaVersion,
		Accession:     "SRR31480456",
		Archive: configv1.SRAArchive{
			Path: archivePath, SHA256: commandDigest(t, archivePath), Bytes: commandSize(t, archivePath), MD5: "0123456789abcdef0123456789abcdef",
		},
		Outputs: inputSRA.DecodeManifestOutputs{
			R1: configv1.SRAAcquiredFASTQ{Path: r1Path, SHA256: commandDigest(t, r1Path), Bytes: commandSize(t, r1Path), PairedRecordCount: 100},
			R2: configv1.SRAAcquiredFASTQ{Path: r2Path, SHA256: commandDigest(t, r2Path), Bytes: commandSize(t, r2Path), PairedRecordCount: 100},
		},
		Tool: inputSRA.DecodeManifestTool{
			Name: "sra-tools", FasterqDump: "/tools/fasterq-dump", FasterqDumpVersion: "fasterq-dump : 3.1.1", Compression: "/tools/pigz -n -p 8", PigzVersion: "pigz 2.8",
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

func writeCommandFile(t *testing.T, root string, relativePath string, contents string) string {
	t.Helper()
	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func commandDigest(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func commandSize(t *testing.T, path string) int64 {
	t.Helper()
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fileInfo.Size()
}
