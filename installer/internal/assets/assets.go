// Package assets embeds and deploys the default Craftmake workflow catalog
// and configuration templates.
package assets

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed workflows configs
var EmbeddedFS embed.FS

// ExtractAll extracts all embedded workflows and configs into the standard target locations:
// - workflows -> <shareDir>/workflows and <installDir>/workflows
// - configs   -> <shareDir>/configs
func ExtractAll(installDir string, dryRun bool) error {
	shareDir := filepath.Join(filepath.Dir(installDir), "share", "craftmake")

	targets := []struct {
		subDir string
		dest   string
	}{
		{subDir: "workflows", dest: filepath.Join(shareDir, "workflows")},
		{subDir: "workflows", dest: filepath.Join(installDir, "workflows")},
		{subDir: "configs", dest: filepath.Join(shareDir, "configs")},
	}

	for _, target := range targets {
		if dryRun {
			fmt.Printf("  [DRY-RUN] would deploy %s -> %s\n", target.subDir, target.dest)
			continue
		}
		if err := extractSubDir(target.subDir, target.dest); err != nil {
			return fmt.Errorf("deploy %s to %s: %w", target.subDir, target.dest, err)
		}
	}
	return nil
}

func extractSubDir(subDir, destRoot string) error {
	return fs.WalkDir(EmbeddedFS, subDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(subDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(destRoot, 0o755)
		}
		targetPath := filepath.Join(destRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		data, err := EmbeddedFS.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o644)
	})
}
