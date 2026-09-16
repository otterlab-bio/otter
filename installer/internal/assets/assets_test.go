package assets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAll(t *testing.T) {
	tempDir := t.TempDir()
	installDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ExtractAll(installDir, false); err != nil {
		t.Fatalf("ExtractAll failed: %v", err)
	}

	expectedFiles := []string{
		filepath.Join(tempDir, "share", "craftmake", "workflows", "BeaverBS", "step1.yaml"),
		filepath.Join(tempDir, "share", "craftmake", "workflows", "BeaverRNA", "step1.yaml"),
		filepath.Join(tempDir, "share", "craftmake", "workflows", "ReferenceBuild", "build.yaml"),
		filepath.Join(installDir, "workflows", "BeaverBS", "step1.yaml"),
		filepath.Join(installDir, "workflows", "ReferenceBuild", "build.yaml"),
		filepath.Join(tempDir, "share", "craftmake", "configs", "reference-build.yaml"),
	}

	for _, expectedFile := range expectedFiles {
		if info, err := os.Stat(expectedFile); err != nil {
			t.Errorf("expected file missing: %s (%v)", expectedFile, err)
		} else if info.Size() == 0 {
			t.Errorf("expected file is empty: %s", expectedFile)
		}
	}
}

func TestExtractAllDryRun(t *testing.T) {
	tempDir := t.TempDir()
	installDir := filepath.Join(tempDir, "bin")

	if err := ExtractAll(installDir, true); err != nil {
		t.Fatalf("ExtractAll dry-run failed: %v", err)
	}

	expectedMissing := filepath.Join(tempDir, "share", "craftmake", "workflows")
	if _, err := os.Stat(expectedMissing); !os.IsNotExist(err) {
		t.Errorf("expected %s not to exist in dry-run mode", expectedMissing)
	}
}
