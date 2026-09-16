package reference

// Publishing an archive in parts.
//
// A single 8.98 GB upload was rejected by the dataset host: the push was refused because an
// LFS pointer referenced a blob that never arrived, which is how a very large upload fails
// when it does not complete. Publishing parts keeps each upload small enough to finish, and
// the manifest makes reassembly verifiable instead of a blind concatenation.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/installer/internal/download"
)

const (
	// PartsManifestSchema identifies the manifest format.
	PartsManifestSchema = "otter.reference-parts/v1"
	// PartsManifestSuffix names an archive's manifest beside it.
	PartsManifestSuffix = ".parts.json"
)

// PartEntry describes one published part.
type PartEntry struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// PartsManifest describes an archive that was published as an ordered list of parts.
type PartsManifest struct {
	// Schema is PartsManifestSchema.
	Schema string `json:"schema"`
	// File is the archive name the parts reassemble into.
	File string `json:"file"`
	// Size is the reassembled size in bytes.
	Size int64 `json:"size"`
	// SHA256 is the digest of the reassembled archive.
	SHA256 string `json:"sha256"`
	// PartSize is the nominal size each part was cut to.
	PartSize int64 `json:"part_size"`
	// Parts are ordered; concatenating them in this order yields the archive.
	Parts []PartEntry `json:"parts"`
}

// Validate checks the manifest is internally consistent before any part is downloaded, so an
// incomplete publication fails immediately rather than after gigabytes of transfer.
func (manifest *PartsManifest) Validate() error {
	if manifest.Schema != PartsManifestSchema {
		return fmt.Errorf("unexpected parts manifest schema %q", manifest.Schema)
	}
	if len(manifest.Parts) == 0 {
		return fmt.Errorf("parts manifest lists no parts")
	}
	var total int64
	for index, part := range manifest.Parts {
		if strings.TrimSpace(part.Name) == "" {
			return fmt.Errorf("part %d has no name", index)
		}
		if part.Size <= 0 {
			return fmt.Errorf("part %s has a non-positive size", part.Name)
		}
		if strings.Contains(part.Name, "/") {
			return fmt.Errorf("part %s must be a bare file name", part.Name)
		}
		total += part.Size
	}
	if manifest.Size > 0 && total != manifest.Size {
		return fmt.Errorf("parts total %d bytes but the manifest declares %d", total, manifest.Size)
	}
	return nil
}

func (fetcher *Fetcher) resourceURL(resourcePath string) string {
	return fmt.Sprintf("%s/datasets/%s/resolve/%s/%s",
		strings.TrimSuffix(fetcher.BaseURL, "/"), fetcher.Repo, fetcher.Revision, resourcePath)
}

// ManifestURL is where an archive's parts manifest is published.
func (fetcher *Fetcher) ManifestURL(selection Selection, indexType string) string {
	return fetcher.resourceURL(ArchivePath(selection, indexType) + PartsManifestSuffix)
}

// partURL is where one part lives, beside the archive it belongs to.
func (fetcher *Fetcher) partURL(selection Selection, indexType, partName string) string {
	return fetcher.resourceURL(path.Dir(ArchivePath(selection, indexType)) + "/" + partName)
}

// fetchPublishedArchive obtains the archive in staging, reassembling it from parts when the
// publisher split it, and returns the path to the complete file.
//
// A missing manifest means the archive was published whole, so the plain path still works.
//
// A manifest that exists but cannot be satisfied also falls back to the whole archive. That
// matters because a dataset can be in a mixed state while a publication is in progress, or
// because a split was abandoned after its manifest was committed. Trusting the manifest in
// that state would fail outright, where the whole archive is very likely present.
func (fetcher *Fetcher) fetchPublishedArchive(ctx context.Context, selection Selection, indexType, staging string) (string, error) {
	archive := filepath.Join(staging, ArchiveName(selection, indexType))
	wholeArchive := func() (string, error) {
		url := fetcher.ArchiveURL(selection, indexType)
		if err := fetcher.Client.Download(ctx, url, archive); err != nil {
			return "", fmt.Errorf("download %s: %w", url, err)
		}
		return archive, nil
	}

	manifestBytes, err := fetcher.Client.FetchResource(ctx, fetcher.ManifestURL(selection, indexType))
	if errors.Is(err, download.ErrResourceNotFound) {
		return wholeArchive()
	}
	if err != nil {
		return "", fmt.Errorf("read parts manifest: %w", err)
	}

	var manifest PartsManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return "", fmt.Errorf("parse parts manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return "", err
	}

	assembled, partsErr := fetcher.assembleParts(ctx, selection, indexType, staging, archive, manifest)
	if partsErr != nil {
		fmt.Fprintf(os.Stderr, "  warning: %v; falling back to the whole archive\n", partsErr)
		return wholeArchive()
	}
	return assembled, nil
}

// assembleParts downloads every part, verifies it, joins the parts and verifies the result.
func (fetcher *Fetcher) assembleParts(ctx context.Context, selection Selection, indexType, staging, archive string, manifest PartsManifest) (string, error) {
	partPaths := make([]string, 0, len(manifest.Parts))
	for _, part := range manifest.Parts {
		partPath := filepath.Join(staging, part.Name)
		if err := fetcher.Client.Download(ctx, fetcher.partURL(selection, indexType, part.Name), partPath); err != nil {
			return "", fmt.Errorf("download part %s: %w", part.Name, err)
		}
		if err := verifyFileSHA256(partPath, part.SHA256); err != nil {
			return "", fmt.Errorf("part %s: %w", part.Name, err)
		}
		partPaths = append(partPaths, partPath)
	}

	if err := joinFiles(partPaths, archive); err != nil {
		return "", err
	}
	if err := verifyFileSHA256(archive, manifest.SHA256); err != nil {
		return "", fmt.Errorf("reassembled %s: %w", manifest.File, err)
	}
	// The parts are an upload mechanism; only the reassembled archive is kept.
	for _, partPath := range partPaths {
		os.Remove(partPath)
	}
	return archive, nil
}

// joinFiles concatenates parts in order into destination.
func joinFiles(parts []string, destination string) error {
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer output.Close()
	for _, part := range parts {
		input, err := os.Open(part)
		if err != nil {
			return err
		}
		if _, err := io.Copy(output, input); err != nil {
			input.Close()
			return err
		}
		input.Close()
	}
	return output.Sync()
}

// verifyFileSHA256 compares a file against an expected digest, which may be bare hex or
// prefixed with "sha256:".
func verifyFileSHA256(filePath, expected string) error {
	if strings.TrimSpace(expected) == "" {
		return nil
	}
	want := strings.TrimPrefix(strings.TrimSpace(expected), "sha256:")
	handle, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer handle.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, handle); err != nil {
		return err
	}
	got := hex.EncodeToString(digest.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", got, want)
	}
	return nil
}
