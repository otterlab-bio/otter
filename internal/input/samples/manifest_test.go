package samples

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadResolvesProjectRelativePaths(t *testing.T) {
	projectRoot := t.TempDir()
	manifestPath := filepath.Join(projectRoot, "samples.tsv")
	content := "sample_id\tr1\tr2\tgroup\nS01\tdata/S01_R1.fastq.gz\tdata/S01_R2.fastq.gz\tcase\n"
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := Load(manifestPath, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records", len(records))
	}
	if records[0].R1 != filepath.Join(projectRoot, "data", "S01_R1.fastq.gz") {
		t.Fatalf("unexpected R1 path: %s", records[0].R1)
	}
}

func TestLoadRejectsDuplicateSampleIDs(t *testing.T) {
	projectRoot := t.TempDir()
	manifestPath := filepath.Join(projectRoot, "samples.tsv")
	content := "sample_id\tr1\tr2\nS01\ta_R1.fastq.gz\ta_R2.fastq.gz\nS01\tb_R1.fastq.gz\tb_R2.fastq.gz\n"
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(manifestPath, projectRoot); err == nil {
		t.Fatal("expected duplicate sample ID error")
	}
}

func TestLoadRejectsUnsupportedColumns(t *testing.T) {
	projectRoot := t.TempDir()
	manifestPath := filepath.Join(projectRoot, "samples.tsv")
	content := "sample_id\tr1\tr2\tlegacy_species\nS01\ta_R1.fastq.gz\ta_R2.fastq.gz\thuman\n"
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(manifestPath, projectRoot); err == nil {
		t.Fatal("expected unsupported column error")
	}
}
