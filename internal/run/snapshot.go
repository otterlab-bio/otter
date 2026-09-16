package run

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

type Manifest struct {
	SchemaVersion string              `json:"schema_version"`
	RunID         string              `json:"run_id"`
	CreatedAt     string              `json:"created_at"`
	Immutable     bool                `json:"immutable"`
	Digests       configv1.RunDigests `json:"digests"`
}

func WriteSnapshot(directory Directory, snapshot configv1.RunSnapshot) (string, error) {
	if snapshot.Run.ID != directory.ID || snapshot.Paths.RunRoot != directory.Root {
		return "", fmt.Errorf("snapshot identity does not match allocated run directory")
	}
	if err := configv1.ValidateRunSnapshot(snapshot); err != nil {
		return "", err
	}
	encoded, err := configv1.MarshalRunSnapshot(snapshot)
	if err != nil {
		return "", err
	}
	path := filepath.Join(directory.Root, "run.yaml")
	if err := writeImmutableFile(path, encoded, 0o444); err != nil {
		return "", err
	}
	manifest := Manifest{
		SchemaVersion: "otter.manifest/v1",
		RunID:         snapshot.Run.ID,
		CreatedAt:     string(snapshot.Run.CreatedAt),
		Immutable:     true,
		Digests:       snapshot.Digests,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal run manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := writeImmutableFile(filepath.Join(directory.Root, "manifest.json"), manifestBytes, 0o444); err != nil {
		return "", err
	}
	return path, nil
}

func VerifySnapshotDigests(path string, expected configv1.RunDigests) error {
	snapshot, err := configv1.LoadRunSnapshot(path)
	if err != nil {
		return err
	}
	if snapshot.Digests != expected {
		return fmt.Errorf("run snapshot digest drift detected")
	}
	return nil
}

func DigestFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open digest input %s: %w", path, err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func DigestPaths(paths []string) (string, error) {
	hasher := sha256.New()
	sortedPaths := append([]string(nil), paths...)
	sort.Strings(sortedPaths)
	for _, path := range sortedPaths {
		info, err := os.Stat(path)
		if err != nil {
			return "", fmt.Errorf("inspect required digest path %s: %w", path, err)
		}
		if info.IsDir() {
			if _, err := io.WriteString(hasher, filepath.Base(path)+"/\x00"); err != nil {
				return "", fmt.Errorf("hash directory role %s: %w", path, err)
			}
			if err := hashDirectory(hasher, path); err != nil {
				return "", err
			}
			continue
		}
		if err := hashFileInto(hasher, filepath.Base(path), path); err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func hashDirectory(hasher io.Writer, root string) error {
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("walk digest directory %s: %w", root, err)
	}
	sort.Strings(files)
	for _, path := range files {
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("resolve digest path: %w", err)
		}
		if err := hashFileInto(hasher, filepath.ToSlash(relativePath), path); err != nil {
			return err
		}
	}
	return nil
}

func hashFileInto(hasher io.Writer, logicalPath string, path string) error {
	if _, err := io.WriteString(hasher, logicalPath+"\x00"); err != nil {
		return fmt.Errorf("hash logical path: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open digest input %s: %w", path, err)
	}
	defer file.Close()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("hash %s: %w", path, err)
	}
	return nil
}

func writeImmutableFile(path string, content []byte, permissions os.FileMode) error {
	directory := filepath.Dir(path)
	temporaryFile, err := os.CreateTemp(directory, ".otter-write-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporaryFile.Write(content); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("write temporary file for %s: %w", path, err)
	}
	if err := temporaryFile.Sync(); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("sync temporary file for %s: %w", path, err)
	}
	if err := temporaryFile.Chmod(permissions); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("set permissions for %s: %w", path, err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("close temporary file for %s: %w", path, err)
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return fmt.Errorf("publish immutable file %s: %w", path, err)
	}
	directoryHandle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open directory for sync %s: %w", directory, err)
	}
	defer directoryHandle.Close()
	if err := directoryHandle.Sync(); err != nil {
		return fmt.Errorf("sync directory %s: %w", directory, err)
	}
	return nil
}
