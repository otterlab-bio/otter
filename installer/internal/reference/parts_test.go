package reference

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPartsManifestValidate(t *testing.T) {
	valid := func() PartsManifest {
		return PartsManifest{
			Schema: PartsManifestSchema,
			File:   "hg19_x_bismark.tar.gz",
			Size:   6,
			SHA256: "deadbeef",
			Parts: []PartEntry{
				{Name: "hg19_x_bismark.tar.gz.part-00", Size: 4, SHA256: "aa"},
				{Name: "hg19_x_bismark.tar.gz.part-01", Size: 2, SHA256: "bb"},
			},
		}
	}

	tests := []struct {
		name    string
		mutate  func(*PartsManifest)
		wantErr bool
	}{
		{name: "well formed", mutate: func(*PartsManifest) {}},
		{name: "wrong schema", mutate: func(m *PartsManifest) { m.Schema = "something/else" }, wantErr: true},
		{name: "no parts", mutate: func(m *PartsManifest) { m.Parts = nil }, wantErr: true},
		{name: "unnamed part", mutate: func(m *PartsManifest) { m.Parts[0].Name = "  " }, wantErr: true},
		{name: "non-positive size", mutate: func(m *PartsManifest) { m.Parts[1].Size = 0 }, wantErr: true},
		{name: "part name escapes the directory", mutate: func(m *PartsManifest) { m.Parts[0].Name = "../evil" }, wantErr: true},
		{name: "total does not match the declared size", mutate: func(m *PartsManifest) { m.Size = 99 }, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := valid()
			test.mutate(&manifest)
			err := manifest.Validate()
			if test.wantErr && err == nil {
				t.Fatal("expected the manifest to be rejected")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected rejection: %v", err)
			}
		})
	}
}

func TestJoinFilesReassemblesInOrder(t *testing.T) {
	directory := t.TempDir()
	parts := []string{}
	for index, content := range []string{"first-", "second-", "third"} {
		partPath := filepath.Join(directory, fmt.Sprintf("part-%02d", index))
		if err := os.WriteFile(partPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		parts = append(parts, partPath)
	}
	destination := filepath.Join(directory, "joined")
	if err := joinFiles(parts, destination); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if want := "first-second-third"; string(got) != want {
		t.Errorf("joined content = %q, want %q", got, want)
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	directory := t.TempDir()
	filePath := filepath.Join(directory, "content")
	content := []byte("reassembly must be verifiable")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	hexDigest := hex.EncodeToString(digest[:])
	pointer := func(value string) *string { return &value }

	tests := []struct {
		name     string
		expected *string
		wantErr  bool
	}{
		{name: "bare hex matches", expected: pointer(hexDigest)},
		{name: "sha256 prefix is accepted", expected: pointer("sha256:" + hexDigest)},
		{name: "empty expectation skips the check", expected: pointer("")},
		{name: "absent expectation skips the check", expected: nil},
		{name: "wrong digest is rejected", expected: pointer("00"), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := ""
			if test.expected != nil {
				expected = *test.expected
			}
			err := verifyFileSHA256(filePath, expected)
			if test.wantErr && err == nil {
				t.Fatal("expected a mismatch")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected failure: %v", err)
			}
		})
	}
}
