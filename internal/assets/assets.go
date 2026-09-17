package assets

import (
	"io/fs"
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
}

// NewAssetCopier creates a new AssetCopier
func NewAssetCopier(projectDir string) *AssetCopier {
	return &AssetCopier{
		ProjectDir: projectDir,
	}
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
