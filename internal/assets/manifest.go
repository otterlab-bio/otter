package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/otterlab-bio/otter/internal/logger"
)

const (
	manifestSchemaVersion = 1
	manifestRelPath       = ".otter/assets.manifest.json"
)

type Manifest struct {
	SchemaVersion int             `json:"schema_version"`
	GeneratedAt   string          `json:"generated_at_utc"`
	HashAlg       string          `json:"hash_alg"`
	Tool          ManifestTool    `json:"tool"`
	Entries       []ManifestEntry `json:"entries"`
}

type ManifestTool struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

type ManifestEntry struct {
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	Deprecated bool   `json:"deprecated,omitempty"`
}

type ManifestDiff struct {
	Missing  []string
	Extra    []string
	Modified []string
}

func (d ManifestDiff) IsClean() bool {
	return len(d.Missing) == 0 && len(d.Extra) == 0 && len(d.Modified) == 0
}

func ManifestPath(projectDir string) string {
	return filepath.Join(projectDir, manifestRelPath)
}

// StampWorkflowAssets writes a SHA-256 manifest for the workflow assets under projectDir.
// It includes:
//   - R/** (deprecated)
//   - rules/**
//   - envs/**
//   - data/**
//   - project-root: snakefile + *.snakemake
func StampWorkflowAssets(projectDir, toolVersion, toolCommit, toolDate string) (*Manifest, error) {
	paths, err := discoverWorkflowAssetFiles(projectDir)
	if err != nil {
		return nil, err
	}

	entries := make([]ManifestEntry, 0, len(paths))
	for _, rel := range paths {
		abs := filepath.Join(projectDir, filepath.FromSlash(rel))
		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", rel, err)
		}
		sum, err := sha256File(abs)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", rel, err)
		}
		entries = append(entries, ManifestEntry{
			Path:       rel,
			Size:       info.Size(),
			SHA256:     sum,
			Deprecated: strings.HasPrefix(rel, "R/"),
		})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	m := &Manifest{
		SchemaVersion: manifestSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		HashAlg:       "sha256",
		Tool: ManifestTool{
			Version: toolVersion,
			Commit:  toolCommit,
			Date:    toolDate,
		},
		Entries: entries,
	}

	if err := writeManifest(projectDir, m); err != nil {
		return nil, err
	}
	return m, nil
}

func LoadManifest(projectDir string) (*Manifest, error) {
	p := ManifestPath(projectDir)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", p, err)
	}
	return &m, nil
}

// VerifyWorkflowAssets checks current workflow asset files against the manifest.
// If the manifest is missing, it returns os.ErrNotExist.
func VerifyWorkflowAssets(projectDir string, m *Manifest) (ManifestDiff, error) {
	if m == nil {
		var err error
		m, err = LoadManifest(projectDir)
		if err != nil {
			return ManifestDiff{}, err
		}
	}

	if strings.ToLower(strings.TrimSpace(m.HashAlg)) != "sha256" {
		return ManifestDiff{}, fmt.Errorf("unsupported manifest hash_alg %q", m.HashAlg)
	}

	want := make(map[string]ManifestEntry, len(m.Entries))
	for _, e := range m.Entries {
		want[e.Path] = e
	}

	current, err := discoverWorkflowAssetFiles(projectDir)
	if err != nil {
		return ManifestDiff{}, err
	}

	gotSet := make(map[string]struct{}, len(current))
	for _, p := range current {
		gotSet[p] = struct{}{}
	}

	var diff ManifestDiff
	for p := range want {
		if _, ok := gotSet[p]; !ok {
			diff.Missing = append(diff.Missing, p)
		}
	}
	for p := range gotSet {
		if _, ok := want[p]; !ok {
			diff.Extra = append(diff.Extra, p)
		}
	}

	for p, entry := range want {
		if _, ok := gotSet[p]; !ok {
			continue
		}
		abs := filepath.Join(projectDir, filepath.FromSlash(p))
		sum, err := sha256File(abs)
		if err != nil {
			return ManifestDiff{}, fmt.Errorf("hash %s: %w", p, err)
		}
		if sum != entry.SHA256 {
			diff.Modified = append(diff.Modified, p)
		}
	}

	sort.Strings(diff.Missing)
	sort.Strings(diff.Extra)
	sort.Strings(diff.Modified)
	return diff, nil
}

func LogManifestSummary(m *Manifest) {
	if m == nil {
		return
	}
	deprecated := 0
	for _, e := range m.Entries {
		if e.Deprecated {
			deprecated++
		}
	}
	logger.Infof("Assets manifest: %d files (deprecated: %d) [%s]", len(m.Entries), deprecated, ManifestPath("."))
}

func writeManifest(projectDir string, m *Manifest) error {
	p := ManifestPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(p), err)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(p, b, 0644); err != nil {
		return fmt.Errorf("write manifest %s: %w", p, err)
	}
	return nil
}

func discoverWorkflowAssetFiles(projectDir string) ([]string, error) {
	var rels []string

	addTree := func(relDir string) error {
		absDir := filepath.Join(projectDir, filepath.FromSlash(relDir))
		st, err := os.Stat(absDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !st.IsDir() {
			return nil
		}
		return filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			r, err := filepath.Rel(projectDir, path)
			if err != nil {
				return err
			}
			rels = append(rels, filepath.ToSlash(r))
			return nil
		})
	}

	for _, d := range []string{"R", "rules", "envs", "data"} {
		if err := addTree(d); err != nil {
			return nil, err
		}
	}

	// Root-level workflow entry points.
	rootCandidates := []string{"snakefile"}
	for _, f := range rootCandidates {
		abs := filepath.Join(projectDir, f)
		if st, err := os.Stat(abs); err == nil && !st.IsDir() {
			rels = append(rels, f)
		}
	}
	matches, err := filepath.Glob(filepath.Join(projectDir, "*.snakemake"))
	if err != nil {
		return nil, err
	}
	for _, abs := range matches {
		if st, err := os.Stat(abs); err == nil && !st.IsDir() {
			r, err := filepath.Rel(projectDir, abs)
			if err != nil {
				return nil, err
			}
			rels = append(rels, filepath.ToSlash(r))
		}
	}

	sort.Strings(rels)
	// Dedup.
	out := rels[:0]
	seen := make(map[string]struct{}, len(rels))
	for _, r := range rels {
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	return out, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
