package sra

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestWriteLoadAndVerifyManifest(t *testing.T) {
	acquisitionRoot := t.TempDir()
	archivePath := writeFixtureFile(t, acquisitionRoot, "archive/SRR31480456.sra", "archive")
	r1Path := writeFixtureFile(t, acquisitionRoot, "fastq/SRR31480456_1.fastq.gz", "r1")
	r2Path := writeFixtureFile(t, acquisitionRoot, "fastq/SRR31480456_2.fastq.gz", "r2")
	manifest := validManifest(t, acquisitionRoot, archivePath, r1Path, r2Path)
	manifestPath := filepath.Join(acquisitionRoot, "sra-acquisition.json")

	if err := WriteManifest(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0o444 {
		t.Fatalf("manifest permissions = %o, want 444", fileInfo.Mode().Perm())
	}
	loadedManifest, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if loadedManifest.Entries[0].Scenario != configv1.ScenarioRRBS {
		t.Fatalf("scenario = %q", loadedManifest.Entries[0].Scenario)
	}
	if err := VerifyManifestFiles(loadedManifest); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(manifestPath, manifest); err == nil {
		t.Fatal("expected immutable create-only publication to reject overwrite")
	}
}

func TestValidateManifestRejectsMismatchedPairCounts(t *testing.T) {
	acquisitionRoot := t.TempDir()
	archivePath := writeFixtureFile(t, acquisitionRoot, "archive/SRR31480456.sra", "archive")
	r1Path := writeFixtureFile(t, acquisitionRoot, "fastq/SRR31480456_1.fastq.gz", "r1")
	r2Path := writeFixtureFile(t, acquisitionRoot, "fastq/SRR31480456_2.fastq.gz", "r2")
	manifest := validManifest(t, acquisitionRoot, archivePath, r1Path, r2Path)
	manifest.Entries[0].Output.R2.PairedRecordCount = 101

	if err := ValidateManifest(manifest); err == nil {
		t.Fatal("expected mismatched mate record counts to fail")
	}
}

func TestValidateManifestRequiresPDXRoles(t *testing.T) {
	acquisitionRoot := t.TempDir()
	archivePath := writeFixtureFile(t, acquisitionRoot, "archive/SRR23802966.sra", "archive")
	r1Path := writeFixtureFile(t, acquisitionRoot, "fastq/SRR23802966_1.fastq.gz", "r1")
	r2Path := writeFixtureFile(t, acquisitionRoot, "fastq/SRR23802966_2.fastq.gz", "r2")
	manifest := validManifest(t, acquisitionRoot, archivePath, r1Path, r2Path)
	manifest.Entries[0].Scenario = configv1.ScenarioBSPDX

	if err := ValidateManifest(manifest); err == nil {
		t.Fatal("expected PDX entry with a primary-only reference to fail")
	}
}

func TestValidateManifestAcceptsPDXRoles(t *testing.T) {
	acquisitionRoot := t.TempDir()
	archivePath := writeFixtureFile(t, acquisitionRoot, "archive/SRR23802966.sra", "archive")
	r1Path := writeFixtureFile(t, acquisitionRoot, "fastq/SRR23802966_1.fastq.gz", "r1")
	r2Path := writeFixtureFile(t, acquisitionRoot, "fastq/SRR23802966_2.fastq.gz", "r2")
	manifest := validManifest(t, acquisitionRoot, archivePath, r1Path, r2Path)
	manifest.Entries[0].Scenario = configv1.ScenarioBSPDX
	manifest.Entries[0].Reference = configv1.SRAAcquisitionReference{Species: []configv1.SRAReferenceSelection{
		{
			Role:           configv1.ReferenceRoleGraft,
			ID:             "hg38",
			Release:        "GRCh38-gencode-v44",
			ManifestSHA256: "sha256:bda77ed9591ec4b9b6c4fadf032e9bfeafec0a6e03e7854e4d706e4f8909a132",
		},
		{
			Role:           configv1.ReferenceRoleHost,
			ID:             "mm10",
			Release:        "GRCm38-gencode-M25",
			ManifestSHA256: "sha256:777158ba49da3f43c76b449c635878ba65d26dfc2ebd44cd2af212a3aabf29e2",
		},
	}}

	if err := ValidateManifest(manifest); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaAndStrictLoaderAgreeOnFixture(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	fixturePath := filepath.Join(repositoryRoot, "testdata", "gate6", "sra-acquisition", "production-inputs.json")
	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixtureDocument any
	if err := json.NewDecoder(bytes.NewReader(fixtureBytes)).Decode(&fixtureDocument); err != nil {
		t.Fatal(err)
	}
	schemaPath := filepath.Join(repositoryRoot, "docs", "schema", "otter-sra-acquisition-v1.schema.json")
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var schemaDocument any
	if err := json.Unmarshal(schemaBytes, &schemaDocument); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	schemaURL := "file://" + schemaPath
	if err := compiler.AddResource(schemaURL, schemaDocument); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(fixtureDocument); err != nil {
		t.Fatal(err)
	}

	fixturePathForLoader := filepath.Join(t.TempDir(), "production-inputs.json")
	if err := os.WriteFile(fixturePathForLoader, fixtureBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(fixturePathForLoader); err != nil {
		t.Fatal(err)
	}
}

func validManifest(t *testing.T, acquisitionRoot, archivePath, r1Path, r2Path string) configv1.SRAAcquisitionManifest {
	return configv1.SRAAcquisitionManifest{
		SchemaVersion: configv1.SRAAcquisitionSchemaVersion,
		Acquisition: configv1.SRAAcquisitionIdentity{
			ID:        "sra-20260812T130000Z-production-inputs",
			CreatedAt: "2026-08-12T13:00:00Z",
			Immutable: true,
			Root:      acquisitionRoot,
		},
		Entries: []configv1.SRAAcquisitionEntry{{
			Scenario:  configv1.ScenarioRRBS,
			Accession: "SRR31480456",
			Download: configv1.SRAArchive{
				Path:   archivePath,
				SHA256: fileDigest(t, archivePath),
				Bytes:  fileSize(t, archivePath),
				MD5:    "0123456789abcdef0123456789abcdef",
			},
			Output: configv1.SRAAcquiredFASTQPair{
				R1: configv1.SRAAcquiredFASTQ{Path: r1Path, SHA256: fileDigest(t, r1Path), Bytes: fileSize(t, r1Path), PairedRecordCount: 100},
				R2: configv1.SRAAcquiredFASTQ{Path: r2Path, SHA256: fileDigest(t, r2Path), Bytes: fileSize(t, r2Path), PairedRecordCount: 100},
			},
			Reference: configv1.SRAAcquisitionReference{Primary: &configv1.SRAReferenceSelection{
				Role:           configv1.ReferenceRolePrimary,
				ID:             "hg19",
				Release:        "GRCh37.p13-gencode-v19",
				ManifestSHA256: "sha256:33dfd7d4ec0a90c6e11fdc45d02b2d4e9b6d82e4a607148d1ab467a0e555accc",
			}},
			Tool: configv1.SRAAcquisitionTool{Name: "sra-tools", Version: "3.1.1", Command: "prefetch SRR31480456 && fasterq-dump --split-files SRR31480456"},
		}},
	}
}

func writeFixtureFile(t *testing.T, root, relativePath, content string) string {
	t.Helper()
	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fileInfo.Size()
}
