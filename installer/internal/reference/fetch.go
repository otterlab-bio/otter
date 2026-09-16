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

	"github.com/otterlab-bio/otter/installer/internal/download"
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

// Fetcher downloads a published release and extracts it into the local registry.
//
// The dataset mirrors the registry layout, one archive per asset:
//
//	genomes/<id>/<release>/indexes/<id>_<release>_<type>.tar.gz   root: indexes/<type>/
//	genomes/<id>/<release>/fasta.tar.gz                          root: fasta/
//	genomes/<id>/<release>/annotations.tar.gz                    root: annotations/
//	genomes/<id>/<release>/{reference.yaml,manifest.json,checksums.sha256}
//
// Each archive's root matches the path the contract declares, so extracting into the release
// directory restores exactly what reference.yaml describes.
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
	// AssetNames limits the fetch to a comma-separated subset of assets; empty means all.
	AssetNames string
	// Selections are the releases to fetch.
	Selections []Selection
	// Client performs the download, so proxy handling and dry-run are shared with the rest
	// of the installer.
	Client *download.Client
	// DryRun reports the planned work without downloading or extracting.
	DryRun bool
}

// assetURL is the URL an asset's archive is served from.
func (fetcher *Fetcher) assetURL(selection Selection, asset Asset) string {
	return fetcher.resourceURL(AssetArchivePath(selection, asset))
}

// releaseAssetURL is the URL an index archive is served from. Retained for the index-only
// callers and tests that predate the general asset model.
func (fetcher *Fetcher) releaseAssetURL(selection Selection, indexType string) string {
	return fetcher.assetURL(selection, Asset{Name: indexType, Segment: "indexes"})
}

// ReleaseDirectory is the release directory an archive extracts into.
func (fetcher *Fetcher) ReleaseDirectory(selection Selection) string {
	return filepath.Join(fetcher.RegistryRoot, "genomes", selection.ID, selection.Release)
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

// Fetch downloads and extracts every selection, skipping assets already present.
//
// A release is only complete once its contract files are in place, so those are written last:
// a half-written release that verification would reject is worse than an obviously absent one.
func (fetcher *Fetcher) Fetch(ctx context.Context) error {
	if fetcher.Client == nil {
		return fmt.Errorf("a download client is required")
	}
	selections := fetcher.Selections
	if len(selections) == 0 {
		return fmt.Errorf("no reference selections given")
	}
	assets, err := parseAssets(fetcher.AssetNames)
	if err != nil {
		return err
	}
	staging, err := os.MkdirTemp("", "otter-reference-fetch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	for _, selection := range selections {
		releaseDirectory := fetcher.ReleaseDirectory(selection)
		for _, asset := range assets {
			destination := filepath.Join(releaseDirectory, filepath.FromSlash(AssetDirectory(asset)))
			if _, statErr := os.Stat(destination); statErr == nil {
				fmt.Printf("  %s %s already present\n", selection.String(), AssetDirectory(asset))
				continue
			}
			if fetcher.DryRun {
				fmt.Printf("  [DRY-RUN] would fetch %s into %s\n", AssetArchivePath(selection, asset), destination)
				continue
			}
			fmt.Printf("  fetching %s -> %s\n", AssetArchivePath(selection, asset), destination)
			archive, fetchErr := fetcher.fetchPublishedArchive(ctx, selection, asset, staging)
			if fetchErr != nil {
				return fetchErr
			}
			// The archive root must match the directory the contract declares, so a
			// mismatched archive is rejected rather than unpacked somewhere unexpected.
			if err := verifyArchiveRoot(ctx, archive, AssetDirectory(asset)); err != nil {
				return fmt.Errorf("%s: %w", ArchiveFileName(selection, asset), err)
			}
			if err := os.MkdirAll(releaseDirectory, 0o755); err != nil {
				return err
			}
			if err := extractArchive(ctx, archive, releaseDirectory); err != nil {
				return fmt.Errorf("extract %s: %w", archive, err)
			}
		}

		for _, name := range ContractFiles {
			destination := filepath.Join(releaseDirectory, name)
			if _, statErr := os.Stat(destination); statErr == nil {
				continue
			}
			if fetcher.DryRun {
				fmt.Printf("  [DRY-RUN] would fetch %s\n", ContractFilePath(selection, name))
				continue
			}
			if err := fetcher.Client.Download(ctx, fetcher.resourceURL(ContractFilePath(selection, name)), destination); err != nil {
				return fmt.Errorf("fetch %s: %w", ContractFilePath(selection, name), err)
			}
			fmt.Printf("  fetched %s\n", ContractFilePath(selection, name))
		}
	}
	return nil
}

// verifyArchiveRoot enforces the archive contract before anything is written.
//
// Every entry must live inside the directory the contract declares — indexes/<type>/ for an
// index, fasta/ or annotations/ for those — because that path is what reference.yaml records.
// An archive rooted anywhere else would extract into the wrong place and leave a registry that
// cannot be resolved, so it is rejected rather than unpacked.
//
// Directory entries for the ancestors of the expected root are tolerated, because tar records
// them when a parent path is packed and they extract nothing outside the release directory.
func verifyArchiveRoot(ctx context.Context, archive, expectedRoot string) error {
	command := exec.CommandContext(ctx, "tar", "-tzf", archive)
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("cannot list archive: %w", err)
	}
	root := strings.TrimSuffix(expectedRoot, "/")
	prefix := root + "/"
	entries := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(entries) == 0 || strings.TrimSpace(entries[0]) == "" {
		return fmt.Errorf("archive is empty")
	}
	for _, entry := range entries {
		entry = strings.TrimSuffix(strings.TrimSpace(entry), "/")
		if entry == "" {
			continue
		}
		if entry == root || strings.HasPrefix(entry, prefix) {
			continue
		}
		// An ancestor directory such as "indexes" is acceptable; anything else is not.
		if strings.HasPrefix(root, entry+"/") {
			continue
		}
		return fmt.Errorf("archive root must be %s/, found %q", root, entry)
	}
	return nil
}

func extractArchive(ctx context.Context, archive, destination string) error {
	command := exec.CommandContext(ctx, "tar", "-xzf", archive, "-C", destination)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
