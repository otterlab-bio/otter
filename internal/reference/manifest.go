package reference

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

type ManifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type ManifestReport struct {
	ReleaseRoot  string
	Entries      []ManifestEntry
	ManifestPath string
}

type VerificationIssue struct {
	Asset    string
	Path     string
	Expected string
	Actual   string
}

type VerificationReport struct {
	Passed bool
	Issues []VerificationIssue
}

func BuildManifest(releaseRoot string) (*ManifestReport, error) {
	if !filepath.IsAbs(releaseRoot) {
		return nil, fmt.Errorf("release root must be absolute")
	}
	entries := make([]ManifestEntry, 0)
	if err := appendManifestFile(releaseRoot, filepath.Join(releaseRoot, "reference.yaml"), &entries); err != nil {
		return nil, fmt.Errorf("include reference definition: %w", err)
	}
	walkPaths := []string{
		filepath.Join(releaseRoot, "fasta"),
		filepath.Join(releaseRoot, "annotations"),
		filepath.Join(releaseRoot, "indexes"),
	}
	for _, dir := range walkPaths {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("walk release directory %q: %w", dir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("expected directory: %q", dir)
		}
		if err := filepath.WalkDir(dir, func(path string, dirEntry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if dirEntry.IsDir() {
				return nil
			}
			relPath, err := filepath.Rel(releaseRoot, path)
			if err != nil {
				return fmt.Errorf("resolve relative path for %q: %w", path, err)
			}
			fileDigest, err := digestFile(path)
			if err != nil {
				return fmt.Errorf("digest %q: %w", path, err)
			}
			entries = append(entries, ManifestEntry{
				Path:   filepath.ToSlash(relPath),
				SHA256: fileDigest,
			})
			return nil
		}); err != nil {
			return nil, fmt.Errorf("walk %q: %w", dir, err)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return &ManifestReport{
		ReleaseRoot:  releaseRoot,
		Entries:      entries,
		ManifestPath: filepath.Join(releaseRoot, "manifest.json"),
	}, nil
}

func appendManifestFile(releaseRoot, path string, entries *[]ManifestEntry) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect %q: %w", path, err)
	}
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", path)
	}
	relativePath, err := filepath.Rel(releaseRoot, path)
	if err != nil {
		return fmt.Errorf("resolve relative path for %q: %w", path, err)
	}
	fileDigest, err := digestFile(path)
	if err != nil {
		return fmt.Errorf("digest %q: %w", path, err)
	}
	*entries = append(*entries, ManifestEntry{
		Path:   filepath.ToSlash(relativePath),
		SHA256: fileDigest,
	})
	return nil
}

func (report *ManifestReport) Write(path string) error {
	if path == "" {
		path = report.ManifestPath
	}
	data, err := json.MarshalIndent(report.Entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest %q: %w", path, err)
	}
	return nil
}

func PublishRelease(releaseRoot string) (*ManifestReport, error) {
	stagingDir, err := os.MkdirTemp(filepath.Dir(releaseRoot), ".staging-")
	if err != nil {
		return nil, fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	report, buildErr := BuildManifest(releaseRoot)
	if buildErr != nil {
		return nil, fmt.Errorf("build manifest: %w", buildErr)
	}
	manifestPath := filepath.Join(stagingDir, "manifest.json")
	if err := report.Write(manifestPath); err != nil {
		return nil, fmt.Errorf("stage manifest: %w", err)
	}

	verifyReport, verifyErr := VerifyChecksums(releaseRoot)
	if verifyErr != nil {
		return nil, fmt.Errorf("verify checksums: %w", verifyErr)
	}
	if !verifyReport.Passed {
		for _, issue := range verifyReport.Issues {
			fmt.Fprintf(os.Stderr, "checksum mismatch: %s %s: expected %s, got %s\n", issue.Asset, issue.Path, issue.Expected, issue.Actual)
		}
		return nil, fmt.Errorf("checksum verification failed with %d issue(s)", len(verifyReport.Issues))
	}

	finalManifestPath := filepath.Join(releaseRoot, "manifest.json")
	if err := os.Rename(manifestPath, finalManifestPath); err != nil {
		return nil, fmt.Errorf("publish manifest to %q: %w", finalManifestPath, err)
	}
	if err := os.Chmod(finalManifestPath, 0o444); err != nil {
		return nil, fmt.Errorf("make manifest %q immutable: %w", finalManifestPath, err)
	}
	return report, nil
}

func VerifyReleaseIdentity(releaseRoot string, expectedManifestDigest string) error {
	if !filepath.IsAbs(releaseRoot) {
		return fmt.Errorf("release root must be absolute")
	}
	manifestPath := filepath.Join(releaseRoot, "manifest.json")
	actualManifestDigest, err := digestFile(manifestPath)
	if err != nil {
		return fmt.Errorf("digest manifest %q: %w", manifestPath, err)
	}
	if expectedManifestDigest != "" && actualManifestDigest != expectedManifestDigest {
		return fmt.Errorf("manifest digest mismatch: expected %s, got %s", expectedManifestDigest, actualManifestDigest)
	}

	definition, err := configv1.LoadReferenceDefinition(filepath.Join(releaseRoot, "reference.yaml"))
	if err != nil {
		return fmt.Errorf("load reference definition: %w", err)
	}
	if err := validateReleaseIdentityAssets(releaseRoot, definition); err != nil {
		return err
	}
	return nil
}

func validateReleaseIdentityAssets(releaseRoot string, definition configv1.ReferenceDefinition) error {
	assetPaths := []string{definition.Assets.Fasta.Path}
	if definition.Assets.Fasta.FAI != "" {
		assetPaths = append(assetPaths, definition.Assets.Fasta.FAI)
	}
	for _, annotation := range definition.Assets.Annotations {
		assetPaths = append(assetPaths, annotation.Path)
	}
	for _, assetPath := range assetPaths {
		fileInfo, err := os.Stat(filepath.Join(releaseRoot, assetPath))
		if err != nil {
			return fmt.Errorf("inspect declared asset %q: %w", assetPath, err)
		}
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("declared asset %q is not a regular file", assetPath)
		}
	}
	for _, index := range definition.Assets.Indexes {
		indexInfo, err := os.Stat(filepath.Join(releaseRoot, index.Path))
		if err != nil {
			return fmt.Errorf("inspect declared index %q: %w", index.Path, err)
		}
		if !indexInfo.IsDir() {
			return fmt.Errorf("declared index %q is not a directory", index.Path)
		}
	}
	return nil
}

func VerifyRelease(releaseRoot string, expectedManifestDigest string) (*VerificationReport, error) {
	manifestPath := filepath.Join(releaseRoot, "manifest.json")
	actualManifestDigest, err := digestFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("digest manifest %q: %w", manifestPath, err)
	}
	if expectedManifestDigest != "" && actualManifestDigest != expectedManifestDigest {
		return nil, fmt.Errorf("manifest digest mismatch: expected %s, got %s", expectedManifestDigest, actualManifestDigest)
	}
	manifestReport, err := VerifyManifestEntries(releaseRoot, manifestPath)
	if err != nil {
		return nil, err
	}
	checksumReport, err := VerifyChecksums(releaseRoot)
	if err != nil {
		return nil, err
	}
	checksumReport.Issues = append(manifestReport.Issues, checksumReport.Issues...)
	checksumReport.Passed = manifestReport.Passed && checksumReport.Passed
	return checksumReport, nil
}

func VerifyManifestEntries(releaseRoot, manifestPath string) (*VerificationReport, error) {
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", manifestPath, err)
	}
	var declaredEntries []ManifestEntry
	if err := json.Unmarshal(manifestData, &declaredEntries); err != nil {
		return nil, fmt.Errorf("parse manifest %q: %w", manifestPath, err)
	}
	if len(declaredEntries) == 0 {
		return nil, fmt.Errorf("manifest %q has no entries", manifestPath)
	}
	actualManifest, err := BuildManifest(releaseRoot)
	if err != nil {
		return nil, fmt.Errorf("build current manifest for %q: %w", releaseRoot, err)
	}
	declaredByPath := make(map[string]string, len(declaredEntries))
	report := &VerificationReport{Passed: true}
	for _, entry := range declaredEntries {
		cleanPath := filepath.ToSlash(filepath.Clean(entry.Path))
		if entry.Path == "" || filepath.IsAbs(entry.Path) || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") || declaredByPath[cleanPath] != "" {
			return nil, fmt.Errorf("manifest %q contains invalid or duplicate path %q", manifestPath, entry.Path)
		}
		declaredByPath[cleanPath] = entry.SHA256
	}
	actualByPath := make(map[string]string, len(actualManifest.Entries))
	for _, entry := range actualManifest.Entries {
		actualByPath[entry.Path] = entry.SHA256
		declaredDigest, exists := declaredByPath[entry.Path]
		if !exists {
			report.Passed = false
			report.Issues = append(report.Issues, VerificationIssue{Asset: "manifest", Path: entry.Path, Expected: "not present", Actual: entry.SHA256})
			continue
		}
		if declaredDigest != entry.SHA256 {
			report.Passed = false
			report.Issues = append(report.Issues, VerificationIssue{Asset: "manifest", Path: entry.Path, Expected: declaredDigest, Actual: entry.SHA256})
		}
	}
	for _, entry := range declaredEntries {
		cleanPath := filepath.ToSlash(filepath.Clean(entry.Path))
		if _, exists := actualByPath[cleanPath]; !exists {
			report.Passed = false
			report.Issues = append(report.Issues, VerificationIssue{Asset: "manifest", Path: cleanPath, Expected: entry.SHA256, Actual: "missing"})
		}
	}
	return report, nil
}

func VerifyChecksums(releaseRoot string) (*VerificationReport, error) {
	definitionPath := filepath.Join(releaseRoot, "reference.yaml")
	definition, err := configv1.LoadReferenceDefinition(definitionPath)
	if err != nil {
		return nil, fmt.Errorf("load reference definition %q: %w", definitionPath, err)
	}
	report := &VerificationReport{Passed: true}

	actualFasta, err := digestFile(filepath.Join(releaseRoot, definition.Assets.Fasta.Path))
	if err != nil {
		report.Issues = append(report.Issues, VerificationIssue{
			Asset: "fasta", Path: definition.Assets.Fasta.Path, Expected: definition.Assets.Fasta.SHA256, Actual: fmt.Sprintf("error: %v", err),
		})
		report.Passed = false
	} else if actualFasta != definition.Assets.Fasta.SHA256 {
		report.Issues = append(report.Issues, VerificationIssue{
			Asset: "fasta", Path: definition.Assets.Fasta.Path, Expected: definition.Assets.Fasta.SHA256, Actual: actualFasta,
		})
		report.Passed = false
	}
	if definition.Assets.Fasta.FAI != "" {
		faiPath := filepath.Join(releaseRoot, definition.Assets.Fasta.FAI)
		faiFile, openErr := os.Open(faiPath)
		if openErr != nil {
			report.Issues = append(report.Issues, VerificationIssue{
				Asset: "fai", Path: definition.Assets.Fasta.FAI, Expected: "readable FAI file", Actual: fmt.Sprintf("error: %v", openErr),
			})
			report.Passed = false
		} else if closeErr := faiFile.Close(); closeErr != nil {
			report.Issues = append(report.Issues, VerificationIssue{
				Asset: "fai", Path: definition.Assets.Fasta.FAI, Expected: "readable FAI file", Actual: fmt.Sprintf("error closing file: %v", closeErr),
			})
			report.Passed = false
		}
	}

	for _, annotation := range definition.Assets.Annotations {
		actualDigest, err := digestFile(filepath.Join(releaseRoot, annotation.Path))
		if err != nil {
			report.Issues = append(report.Issues, VerificationIssue{
				Asset: fmt.Sprintf("annotation:%s", annotation.Type), Path: annotation.Path, Expected: annotation.SHA256, Actual: fmt.Sprintf("error: %v", err),
			})
			report.Passed = false
		} else if actualDigest != annotation.SHA256 {
			report.Issues = append(report.Issues, VerificationIssue{
				Asset: fmt.Sprintf("annotation:%s", annotation.Type), Path: annotation.Path, Expected: annotation.SHA256, Actual: actualDigest,
			})
			report.Passed = false
		}
	}

	for _, index := range definition.Assets.Indexes {
		if index.ReferenceFastaSHA256 != "" && index.ReferenceFastaSHA256 != definition.Assets.Fasta.SHA256 {
			report.Issues = append(report.Issues, VerificationIssue{
				Asset: fmt.Sprintf("index:%s", index.Type), Path: index.Path,
				Expected: fmt.Sprintf("index built from FASTA %s", definition.Assets.Fasta.SHA256),
				Actual:   fmt.Sprintf("index declares FASTA %s", index.ReferenceFastaSHA256),
			})
			report.Passed = false
		}
		actualIndexDigest, err := digestDirectory(filepath.Join(releaseRoot, index.Path))
		if err != nil {
			report.Issues = append(report.Issues, VerificationIssue{
				Asset: fmt.Sprintf("index:%s", index.Type), Path: index.Path, Expected: index.ManifestSHA256, Actual: fmt.Sprintf("error: %v", err),
			})
			report.Passed = false
		} else if actualIndexDigest != index.ManifestSHA256 {
			report.Issues = append(report.Issues, VerificationIssue{
				Asset: fmt.Sprintf("index:%s", index.Type), Path: index.Path, Expected: index.ManifestSHA256, Actual: actualIndexDigest,
			})
			report.Passed = false
		}
	}

	return report, nil
}

func digestDirectory(dirPath string) (string, error) {
	var digests []string
	if err := filepath.WalkDir(dirPath, func(path string, dirEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if dirEntry.IsDir() {
			return nil
		}
		fileDigest, err := digestFile(path)
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(dirPath, path)
		digests = append(digests, relPath+":"+fileDigest)
		return nil
	}); err != nil {
		return "", fmt.Errorf("walk index directory %q: %w", dirPath, err)
	}
	sort.Strings(digests)
	hasher := sha256.New()
	for _, entry := range digests {
		hasher.Write([]byte(entry))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func ComputeDigest(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func parseDigest(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "sha256:") {
		return raw
	}
	return "sha256:" + raw
}
