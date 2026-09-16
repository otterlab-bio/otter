// Package reference: fetching published index builds.
//
// ReferenceBuild compiles a release from source. This file covers the other direction: taking
// the index builds that were already produced once and published as a dataset, so a new
// machine does not have to spend hours rebuilding STAR, bowtie2 and Bismark indexes.
package reference

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rainoffallingstar/otter/installer/internal/download"
)

// Selection identifies one reference release to fetch.
type Selection struct {
	ID      string
	Release string
}

// String renders the selection the way the CLI accepts it.
func (selection Selection) String() string {
	return selection.ID + "@" + selection.Release
}

// DefaultIndexTypes are the index builds the registry contract defines.
var DefaultIndexTypes = []string{"bismark", "bowtie2", "star"}

// Fetcher downloads published index archives and extracts them into the local registry.
//
// The dataset mirrors the registry layout, one archive per index type:
//
//	genomes/<id>/<release>/indexes/<id>_<release>_<type>.tar.gz
//
// and each archive's root is indexes/<type>/..., which is what reference.yaml declares as
// "path: indexes/<type>" relative to the release directory. Extracting an archive into the
// release directory therefore restores exactly what the contract describes.
type Fetcher struct {
	// BaseURL is the dataset host. The public dataset needs no credentials, so plain HTTPS
	// works; a mirror can be substituted here.
	BaseURL string
	// Repo is the dataset repository, for example "fallingstar10/xdxtools-genomes".
	Repo string
	// Revision is the branch, tag, or commit to resolve.
	Revision string
	// RegistryRoot is the directory that contains (or will contain) "genomes/".
	RegistryRoot string
	// IndexTypes defaults to DefaultIndexTypes when empty.
	IndexTypes []string
	// Selections are the releases to fetch.
	Selections []Selection
	// Client performs the download, so proxy handling and dry-run are shared with the rest
	// of the installer.
	Client *download.Client
	// DryRun reports the planned work without downloading or extracting.
	DryRun bool
}

// ArchiveName is the archive basename for one index type, matching the publishing side.
func ArchiveName(selection Selection, indexType string) string {
	return fmt.Sprintf("%s_%s_%s.tar.gz", selection.ID, selection.Release, indexType)
}

// ArchivePath is the dataset-relative path of one archive.
func ArchivePath(selection Selection, indexType string) string {
	return fmt.Sprintf("genomes/%s/%s/indexes/%s", selection.ID, selection.Release, ArchiveName(selection, indexType))
}

// ArchiveURL is the URL the archive is served from.
func (fetcher *Fetcher) ArchiveURL(selection Selection, indexType string) string {
	return fmt.Sprintf("%s/datasets/%s/resolve/%s/%s",
		strings.TrimSuffix(fetcher.BaseURL, "/"),
		fetcher.Repo,
		fetcher.Revision,
		ArchivePath(selection, indexType),
	)
}

// ReleaseDirectory is the release directory an archive extracts into.
func (fetcher *Fetcher) ReleaseDirectory(selection Selection) string {
	return filepath.Join(fetcher.RegistryRoot, "genomes", selection.ID, selection.Release)
}

func (fetcher *Fetcher) indexTypes() []string {
	if len(fetcher.IndexTypes) == 0 {
		return DefaultIndexTypes
	}
	return fetcher.IndexTypes
}

// ParseSelections reads a comma-separated list of "<id>@<release>" entries.
//
// The release label is mandatory rather than optional: <id> alone does not identify an
// immutable artifact, and the registry contract is built on the pair.
func ParseSelections(value string) ([]Selection, error) {
	var selections []Selection
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		id, release, found := strings.Cut(entry, "@")
		id = strings.TrimSpace(id)
		release = strings.TrimSpace(release)
		if !found || id == "" || release == "" {
			return nil, fmt.Errorf("reference selection %q must be written as <id>@<release>", entry)
		}
		selections = append(selections, Selection{ID: id, Release: release})
	}
	if len(selections) == 0 {
		return nil, fmt.Errorf("no reference selections given")
	}
	return selections, nil
}

// Fetch downloads and extracts every selection, skipping archives already present.
func (fetcher *Fetcher) Fetch(ctx context.Context) error {
	if fetcher.Client == nil {
		return fmt.Errorf("a download client is required")
	}
	selections := fetcher.Selections
	if len(selections) == 0 {
		return fmt.Errorf("no reference selections given")
	}
	staging, err := os.MkdirTemp("", "otter-reference-fetch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	for _, selection := range selections {
		releaseDirectory := fetcher.ReleaseDirectory(selection)
		for _, indexType := range fetcher.indexTypes() {
			archivePath := ArchivePath(selection, indexType)
			destination := filepath.Join(releaseDirectory, "indexes", indexType)
			if _, statErr := os.Stat(destination); statErr == nil {
				fmt.Printf("  %s already present at %s\n", selection.String(), destination)
				continue
			}
			url := fetcher.ArchiveURL(selection, indexType)
			if fetcher.DryRun {
				fmt.Printf("  [DRY-RUN] would download %s\n", url)
				fmt.Printf("  [DRY-RUN] (reassembling parts first if a %s is published)\n", PartsManifestSuffix)
				fmt.Printf("  [DRY-RUN] would extract %s into %s\n", archivePath, releaseDirectory)
				continue
			}
			fmt.Printf("  fetching %s -> %s\n", archivePath, destination)
			archive, err := fetcher.fetchPublishedArchive(ctx, selection, indexType, staging)
			if err != nil {
				return err
			}
			if err := verifyArchiveRoot(ctx, archive); err != nil {
				return fmt.Errorf("%s: %w", ArchiveName(selection, indexType), err)
			}
			if err := os.MkdirAll(releaseDirectory, 0o755); err != nil {
				return err
			}
			if err := extractArchive(ctx, archive, releaseDirectory); err != nil {
				return fmt.Errorf("extract %s: %w", archive, err)
			}
		}
	}
	return nil
}

// verifyArchiveRoot enforces the archive contract before anything is written.
//
// Every entry must live under indexes/<type>/, because that is the path reference.yaml
// declares. An archive whose root is a bare directory name would extract into the wrong
// place and leave a registry that cannot be resolved, so it is rejected rather than unpacked.
func verifyArchiveRoot(ctx context.Context, archive string) error {
	command := exec.CommandContext(ctx, "tar", "-tzf", archive)
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("cannot list archive: %w", err)
	}
	entries := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(entries) == 0 || strings.TrimSpace(entries[0]) == "" {
		return fmt.Errorf("archive is empty")
	}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.HasPrefix(entry, "indexes/") {
			return fmt.Errorf("archive root must be indexes/<type>/, found %q", entry)
		}
	}
	return nil
}

func extractArchive(ctx context.Context, archive, destination string) error {
	command := exec.CommandContext(ctx, "tar", "-xzf", archive, "-C", destination)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
