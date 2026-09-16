package assets

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/otterlab-bio/otter/internal/logger"
)

// V1AssetSet maps one packaged asset directory onto a canonical v1 project
// directory. The project directory names are a contract: the canonical resolver
// digests exactly these paths (see internal/config/resolver), so renaming a role
// here without updating the resolver breaks run resolution.
type V1AssetSet struct {
	// Role is the project-relative directory, for example "workflows".
	Role string
	// Source is the packaged origin inside the embedded filesystem, for
	// example "inst/snakefiles".
	Source string
}

// V1AssetSets is the ordered set of packaged assets pinned into a canonical
// project. "workflows" and "rules" come from the Snakemake compatibility assets
// because the Craftmake catalog is resolved from the installed catalog or an
// explicit --catalog path rather than being copied into the project.
var V1AssetSets = []V1AssetSet{
	{Role: "workflows", Source: "inst/snakefiles"},
	{Role: "rules", Source: "inst/rules"},
	{Role: "environments", Source: "inst/envs"},
	{Role: "schemas", Source: "docs/schema"},
}

// V1ProjectDirs are the project-level directories a canonical project owns.
var V1ProjectDirs = []string{"workflows", "rules", "environments", "schemas", "runs"}

// CopiedAssetSet reports what was pinned for one role.
type CopiedAssetSet struct {
	Role    string
	Source  string
	DestDir string
	Files   int
}

// CreateV1DirectoryStructure creates the canonical project directories.
func (c *AssetCopier) CreateV1DirectoryStructure() error {
	for _, dir := range V1ProjectDirs {
		fullPath := filepath.Join(c.ProjectDir, dir)
		if err := os.MkdirAll(fullPath, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
		}
		logger.Debugf("Created directory: %s", fullPath)
	}
	return nil
}

// CopyV1ProjectAssets pins every canonical asset set into the project and
// reports the destination and file count of each set so the caller can record
// digests in project.lock.yaml.
func (c *AssetCopier) CopyV1ProjectAssets() ([]CopiedAssetSet, error) {
	copied := make([]CopiedAssetSet, 0, len(V1AssetSets))
	for _, assetSet := range V1AssetSets {
		destDir := filepath.Join(c.ProjectDir, assetSet.Role)
		files, err := c.copyEmbeddedDir(assetSet.Source, destDir)
		if err != nil {
			return nil, fmt.Errorf("failed to copy %s into %s: %w", assetSet.Source, assetSet.Role, err)
		}
		copied = append(copied, CopiedAssetSet{
			Role:    assetSet.Role,
			Source:  assetSet.Source,
			DestDir: destDir,
			Files:   files,
		})
		logger.Infof("Pinned %s -> %s/ (%d files)", assetSet.Source, assetSet.Role, files)
	}
	return copied, nil
}

// copyEmbeddedDir copies one embedded directory tree and returns the number of
// regular files written.
func (c *AssetCopier) copyEmbeddedDir(sourceDir, destDir string) (int, error) {
	if EmbeddedAssets == nil {
		return 0, fmt.Errorf("embedded assets are not initialised")
	}
	if _, err := fs.Stat(EmbeddedAssets, sourceDir); err != nil {
		return 0, fmt.Errorf("embedded asset source %q is unavailable: %w", sourceDir, err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return 0, err
	}
	writtenFiles := 0
	err := fs.WalkDir(EmbeddedAssets, sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == sourceDir {
			return nil
		}
		relativePath := strings.TrimPrefix(path, sourceDir+"/")
		destinationPath := filepath.Join(destDir, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(destinationPath, 0o755)
		}
		content, readErr := fs.ReadFile(EmbeddedAssets, path)
		if readErr != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, readErr)
		}
		if writeErr := os.WriteFile(destinationPath, content, 0o644); writeErr != nil {
			return fmt.Errorf("failed to write file %s: %w", destinationPath, writeErr)
		}
		writtenFiles++
		logger.Debugf("Copied: %s -> %s", path, destinationPath)
		return nil
	})
	return writtenFiles, err
}

// CountRegularFiles reports how many regular files a directory holds. It is used
// to reconcile project.lock.yaml against the pinned tree.
func CountRegularFiles(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// EmbeddedAssetSetSources lists the packaged sources present in the embedded
// filesystem, sorted for deterministic diagnostics.
func EmbeddedAssetSetSources() ([]string, error) {
	sources := make([]string, 0, len(V1AssetSets))
	for _, assetSet := range V1AssetSets {
		sources = append(sources, assetSet.Source)
	}
	sort.Strings(sources)
	return sources, nil
}
