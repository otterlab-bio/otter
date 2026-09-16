package reference

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseSelections(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []Selection
		wantErr bool
	}{
		{
			name:  "single selection",
			value: "hg19@GRCh37.p13-gencode-v19",
			want:  []Selection{{ID: "hg19", Release: "GRCh37.p13-gencode-v19"}},
		},
		{
			name:  "several selections with whitespace",
			value: " hg19@GRCh37.p13-gencode-v19 , mm9@NCBIM37-gencode-M1 ",
			want: []Selection{
				{ID: "hg19", Release: "GRCh37.p13-gencode-v19"},
				{ID: "mm9", Release: "NCBIM37-gencode-M1"},
			},
		},
		{
			name:    "release label is mandatory",
			value:   "hg19",
			wantErr: true,
		},
		{
			name:    "empty release is rejected",
			value:   "hg19@",
			wantErr: true,
		},
		{
			name:    "empty id is rejected",
			value:   "@GRCh37",
			wantErr: true,
		},
		{
			name:    "nothing to fetch",
			value:   " , ",
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseSelections(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("got %d selections, want %d", len(got), len(test.want))
			}
			for index := range got {
				if got[index] != test.want[index] {
					t.Errorf("selection %d = %+v, want %+v", index, got[index], test.want[index])
				}
			}
		})
	}
}

func TestArchiveLocationFollowsTheRegistryLayout(t *testing.T) {
	fetcher := &Fetcher{BaseURL: "https://example.test/", Repo: "owner/dataset", Revision: "main", RegistryRoot: "/registry"}
	selection := Selection{ID: "hg19", Release: "GRCh37.p13-gencode-v19"}

	if got, want := ArchiveName(selection, "bowtie2"), "hg19_GRCh37.p13-gencode-v19_bowtie2.tar.gz"; got != want {
		t.Errorf("ArchiveName = %q, want %q", got, want)
	}
	if got, want := ArchivePath(selection, "bowtie2"), "genomes/hg19/GRCh37.p13-gencode-v19/indexes/hg19_GRCh37.p13-gencode-v19_bowtie2.tar.gz"; got != want {
		t.Errorf("ArchivePath = %q, want %q", got, want)
	}
	if got, want := fetcher.ArchiveURL(selection, "bowtie2"),
		"https://example.test/datasets/owner/dataset/resolve/main/genomes/hg19/GRCh37.p13-gencode-v19/indexes/hg19_GRCh37.p13-gencode-v19_bowtie2.tar.gz"; got != want {
		t.Errorf("ArchiveURL = %q, want %q", got, want)
	}
	if got, want := fetcher.ReleaseDirectory(selection), filepath.Join("/registry", "genomes", "hg19", "GRCh37.p13-gencode-v19"); got != want {
		t.Errorf("ReleaseDirectory = %q, want %q", got, want)
	}
}

// buildArchive is intentionally absent: the archive fixtures are built inline by
// TestVerifyArchiveRoot so each case controls exactly what the tar contains.

func TestVerifyArchiveRoot(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		wantErr bool
	}{
		{
			name:    "contract layout is accepted",
			entries: []string{"indexes/bowtie2/genome.1.bt2", "indexes/bowtie2/genome.rev.1.bt2"},
		},
		{
			name:    "bare directory name is rejected",
			entries: []string{"bowtie2/genome.1.bt2"},
			wantErr: true,
		},
		{
			name:    "single file at the root is rejected",
			entries: []string{"genome.1.bt2"},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			for _, entry := range test.entries {
				full := filepath.Join(directory, entry)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			archive := filepath.Join(directory, "out.tar.gz")
			// Pack the top-level element of the first entry, which is what the publisher does.
			top := test.entries[0]
			if index := indexByte(top, '/'); index >= 0 {
				top = top[:index]
			}
			command := exec.Command("tar", "-czf", archive, "-C", directory, top)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("building the fixture archive failed: %v %s", err, output)
			}
			err := verifyArchiveRoot(context.Background(), archive)
			if test.wantErr && err == nil {
				t.Fatal("expected the archive to be rejected")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected rejection: %v", err)
			}
		})
	}
}

func indexByte(value string, target byte) int {
	for index := 0; index < len(value); index++ {
		if value[index] == target {
			return index
		}
	}
	return -1
}
