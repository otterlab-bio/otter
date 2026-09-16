package assets

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/internal/logger"
)

// EmbeddedAssets will be set from the root package where files are embedded.
// This is because Go embed requires files to be in the same package or subdirectory.
// Uses fs.FS (not embed.FS) to allow testing with fstest.MapFS.
var EmbeddedAssets fs.FS

// SetEmbeddedAssets sets the embedded filesystem from the main package.
func SetEmbeddedAssets(fsys fs.FS) {
	EmbeddedAssets = fsys
}

// AssetCopier handles copying embedded assets to a project directory
type AssetCopier struct {
	ProjectDir string
	RulesType  string // "rootless" or "legacy"
	EngineType string // "root" or "rootless" (deprecated)
}

// NewAssetCopier creates a new AssetCopier
func NewAssetCopier(projectDir, rulesType string) *AssetCopier {
	if rulesType == "" {
		rulesType = "rootless"
	}
	return &AssetCopier{
		ProjectDir: projectDir,
		RulesType:  rulesType,
		EngineType: "rootless", // Deprecated, always rootless
	}
}

// CopyAll copies all required assets to the project directory
func (c *AssetCopier) CopyAll() error {
	// 1. Copy Rscripts to R/
	if err := c.copyDir("inst/Rscripts", "R"); err != nil {
		return fmt.Errorf("failed to copy Rscripts: %w", err)
	}
	logger.Info("Copied R scripts to R/")

	// 2. Copy snakefiles to root
	if err := c.copyDir("inst/snakefiles", ""); err != nil {
		return fmt.Errorf("failed to copy snakefiles: %w", err)
	}
	logger.Info("Copied Snakemake files")

	// 3. Copy rules based on type
	var rulesDir string
	if c.RulesType == "legacy" {
		rulesDir = "inst/rules_legacy"
	} else {
		rulesDir = "inst/rules"
	}

	if err := c.copyDir(rulesDir, "rules"); err != nil {
		return fmt.Errorf("failed to copy rules: %w", err)
	}

	if c.RulesType == "legacy" {
		logger.Info("Copied legacy rules to rules/")
	} else {
		logger.Info("Copied new rules (4-environment aligned) to rules/")
	}

	// 4. Copy conda environments
	if err := c.copyDir("inst/envs", "envs"); err != nil {
		return fmt.Errorf("failed to copy envs: %w", err)
	}
	logger.Info("Copied conda environments to envs/")

	// 5. Copy gene annotation databases
	if err := c.copyDir("inst/data", "data"); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}
	logger.Info("Copied gene annotation databases to data/")

	return nil
}

// copyDir copies files from embedded source to destination directory
func (c *AssetCopier) copyDir(srcDir, destSubDir string) error {
	if EmbeddedAssets == nil {
		return fmt.Errorf("embedded assets are not initialised")
	}
	destDir := c.ProjectDir
	if destSubDir != "" {
		destDir = filepath.Join(c.ProjectDir, destSubDir)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Walk through embedded files
	return fs.WalkDir(EmbeddedAssets, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == srcDir {
			return nil
		}

		// Get relative path from source directory
		relPath := strings.TrimPrefix(path, srcDir+"/")
		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Read file content
		content, err := fs.ReadFile(EmbeddedAssets, path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}

		// Write to destination
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", destPath, err)
		}

		logger.Debugf("Copied: %s -> %s", path, destPath)
		return nil
	})
}

// CreateDirectoryStructure creates the standard project directory structure
func (c *AssetCopier) CreateDirectoryStructure() error {
	dirs := []string{
		"config",
		"data",
		"envs",
		"inst",
		"R",
		"rules",
		"saveRDS",
		"temp",
		"userspace",
		"workflows",
		"www",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(c.ProjectDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
		}
		logger.Debugf("Created directory: %s", fullPath)
	}

	return nil
}

// ListEmbeddedFiles lists all embedded files (for debugging)
func ListEmbeddedFiles() ([]string, error) {
	var files []string
	err := fs.WalkDir(EmbeddedAssets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
