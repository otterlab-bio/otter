package reference

// A published release is more than its indexes.
//
// The indexes are the expensive part and the reason this fetch exists, but a release also
// carries the reference sequence, its annotation, and three small contract files. Registry
// verification reads the contract files, so a fetch that produced only indexes would yield a
// release that cannot be resolved. The asset set is therefore declared here rather than
// implied by the index types alone.

import (
	"fmt"
	"path"
	"strings"
)

// Asset is one published archive.
type Asset struct {
	// Name is the asset name: an index type, or "fasta" or "annotations".
	Name string
	// Segment is the release subdirectory the asset unpacks into, empty for the release root.
	Segment string
}

// DefaultAssets returns the archives every release publishes: the index builds first, then
// the reference sequence and its annotation.
func DefaultAssets() []Asset {
	assets := make([]Asset, 0, len(DefaultIndexTypes)+2)
	for _, indexType := range DefaultIndexTypes {
		assets = append(assets, Asset{Name: indexType, Segment: "indexes"})
	}
	return append(assets, Asset{Name: "fasta"}, Asset{Name: "annotations"})
}

// ContractFiles make a release verifiable. They are small and are published directly rather
// than archived.
var ContractFiles = []string{"reference.yaml", "manifest.json", "checksums.sha256"}

// ArchiveFileName is an asset's archive name. The two shapes differ because only the indexes
// need the release spelled out: they live in a shared subdirectory, where "fasta.tar.gz"
// would be ambiguous, while a release-root archive is already scoped by its directory.
func ArchiveFileName(selection Selection, asset Asset) string {
	if asset.Segment == "" {
		return asset.Name + ".tar.gz"
	}
	return fmt.Sprintf("%s_%s_%s.tar.gz", selection.ID, selection.Release, asset.Name)
}

// AssetArchivePath is the dataset-relative path of an asset's archive.
func AssetArchivePath(selection Selection, asset Asset) string {
	return path.Join("genomes", selection.ID, selection.Release, asset.Segment, ArchiveFileName(selection, asset))
}

// AssetDirectory is where an asset unpacks, relative to the release directory. It is also the
// leading path every entry of the archive must carry, since that is what the contract
// declares: "indexes/bismark" for an index, "fasta" for the sequence.
func AssetDirectory(asset Asset) string {
	if asset.Segment == "" {
		return asset.Name
	}
	return path.Join(asset.Segment, asset.Name)
}

// ContractFilePath is the dataset-relative path of a contract file.
func ContractFilePath(selection Selection, name string) string {
	return path.Join("genomes", selection.ID, selection.Release, name)
}

// parseAssets reads a comma-separated subset of asset names, defaulting to every asset when
// the value is empty. Unknown names are rejected rather than silently ignored, because a
// typo would otherwise produce a quietly incomplete release.
func parseAssets(value string) ([]Asset, error) {
	if strings.TrimSpace(value) == "" {
		return DefaultAssets(), nil
	}
	available := map[string]Asset{}
	for _, asset := range DefaultAssets() {
		available[asset.Name] = asset
	}
	var assets []Asset
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		asset, found := available[name]
		if !found {
			return nil, fmt.Errorf("unknown reference asset %q", name)
		}
		assets = append(assets, asset)
	}
	if len(assets) == 0 {
		return nil, fmt.Errorf("no reference assets selected")
	}
	return assets, nil
}
