package sra

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

var acquisitionIDPattern = regexp.MustCompile(`^sra-[0-9]{8}T[0-9]{6}Z-[a-z0-9-]+$`)
var accessionPattern = regexp.MustCompile(`^SRR[0-9]+$`)
var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var md5Pattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func LoadManifest(path string) (configv1.SRAAcquisitionManifest, error) {
	manifestFile, err := os.Open(path)
	if err != nil {
		return configv1.SRAAcquisitionManifest{}, fmt.Errorf("open SRA acquisition manifest %q: %w", path, err)
	}
	defer manifestFile.Close()

	decoder := json.NewDecoder(manifestFile)
	decoder.DisallowUnknownFields()
	var manifest configv1.SRAAcquisitionManifest
	if err := decoder.Decode(&manifest); err != nil {
		return configv1.SRAAcquisitionManifest{}, fmt.Errorf("decode SRA acquisition manifest %q: %w", path, err)
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		return configv1.SRAAcquisitionManifest{}, fmt.Errorf("decode SRA acquisition manifest %q: %w", path, err)
	}
	if err := ValidateManifest(manifest); err != nil {
		return configv1.SRAAcquisitionManifest{}, fmt.Errorf("validate SRA acquisition manifest %q: %w", path, err)
	}
	return manifest, nil
}

func WriteManifest(path string, manifest configv1.SRAAcquisitionManifest) error {
	if err := ValidateManifest(manifest); err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("SRA acquisition manifest path must be absolute")
	}
	manifestBytes, err := json.MarshalIndent(sortedManifest(manifest), "", "  ")
	if err != nil {
		return fmt.Errorf("encode SRA acquisition manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create SRA acquisition manifest directory: %w", err)
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(path), ".sra-acquisition-*.json")
	if err != nil {
		return fmt.Errorf("create temporary SRA acquisition manifest: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporaryFile.Write(manifestBytes); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("write temporary SRA acquisition manifest: %w", err)
	}
	if err := temporaryFile.Chmod(0o444); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("protect temporary SRA acquisition manifest: %w", err)
	}
	if err := temporaryFile.Sync(); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("sync temporary SRA acquisition manifest: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("close temporary SRA acquisition manifest: %w", err)
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return fmt.Errorf("publish immutable SRA acquisition manifest %q: %w", path, err)
	}
	return nil
}

func VerifyManifestFiles(manifest configv1.SRAAcquisitionManifest) error {
	if err := ValidateManifest(manifest); err != nil {
		return err
	}
	for entryIndex, entry := range manifest.Entries {
		for fileRole, file := range map[string]configv1.SRAAcquiredFASTQ{
			"R1": entry.Output.R1,
			"R2": entry.Output.R2,
		} {
			if err := verifyRegularFile(file.Path, file.SHA256, file.Bytes); err != nil {
				return fmt.Errorf("entries[%d] %s: %w", entryIndex, fileRole, err)
			}
		}
		if err := verifyRegularFile(entry.Download.Path, entry.Download.SHA256, entry.Download.Bytes); err != nil {
			return fmt.Errorf("entries[%d] archive: %w", entryIndex, err)
		}
	}
	return nil
}

func ValidateManifest(manifest configv1.SRAAcquisitionManifest) error {
	if manifest.SchemaVersion != configv1.SRAAcquisitionSchemaVersion {
		return fmt.Errorf("schema_version must be %q", configv1.SRAAcquisitionSchemaVersion)
	}
	if !acquisitionIDPattern.MatchString(manifest.Acquisition.ID) {
		return fmt.Errorf("acquisition.id %q is invalid", manifest.Acquisition.ID)
	}
	if _, err := time.Parse(time.RFC3339, string(manifest.Acquisition.CreatedAt)); err != nil {
		return fmt.Errorf("acquisition.created_at must be RFC3339: %w", err)
	}
	if !manifest.Acquisition.Immutable {
		return fmt.Errorf("acquisition.immutable must be true")
	}
	if !filepath.IsAbs(manifest.Acquisition.Root) {
		return fmt.Errorf("acquisition.root must be absolute")
	}
	if len(manifest.Entries) == 0 {
		return fmt.Errorf("entries must not be empty")
	}
	seenScenarios := make(map[configv1.Scenario]bool, len(manifest.Entries))
	seenAccessions := make(map[string]bool, len(manifest.Entries))
	for entryIndex, entry := range manifest.Entries {
		if err := validateEntry(entry); err != nil {
			return fmt.Errorf("entries[%d]: %w", entryIndex, err)
		}
		if seenScenarios[entry.Scenario] {
			return fmt.Errorf("entries[%d].scenario %q is duplicated", entryIndex, entry.Scenario)
		}
		if seenAccessions[entry.Accession] {
			return fmt.Errorf("entries[%d].accession %q is duplicated", entryIndex, entry.Accession)
		}
		seenScenarios[entry.Scenario] = true
		seenAccessions[entry.Accession] = true
	}
	return nil
}

func validateEntry(entry configv1.SRAAcquisitionEntry) error {
	if !isScenario(entry.Scenario) {
		return fmt.Errorf("scenario %q is invalid", entry.Scenario)
	}
	if !accessionPattern.MatchString(entry.Accession) {
		return fmt.Errorf("accession %q is invalid", entry.Accession)
	}
	if err := validateArchive(entry.Download); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if err := validateFASTQPair(entry.Output); err != nil {
		return fmt.Errorf("output: %w", err)
	}
	if entry.Output.R1.PairedRecordCount != entry.Output.R2.PairedRecordCount {
		return fmt.Errorf("output R1/R2 paired_record_count values differ")
	}
	if err := validateReference(entry.Scenario, entry.Reference); err != nil {
		return fmt.Errorf("reference: %w", err)
	}
	if strings.TrimSpace(entry.Tool.Name) != "sra-tools" || strings.TrimSpace(entry.Tool.Version) == "" || strings.TrimSpace(entry.Tool.Command) == "" {
		return fmt.Errorf("tool must provide sra-tools name, version, and command")
	}
	return nil
}

func validateArchive(archive configv1.SRAArchive) error {
	if !filepath.IsAbs(archive.Path) || !digestPattern.MatchString(archive.SHA256) || archive.Bytes < 1 || !md5Pattern.MatchString(archive.MD5) {
		return fmt.Errorf("path, sha256, bytes, and md5 are required")
	}
	return nil
}

func validateFASTQPair(pair configv1.SRAAcquiredFASTQPair) error {
	if err := validateFASTQ(pair.R1); err != nil {
		return fmt.Errorf("R1: %w", err)
	}
	if err := validateFASTQ(pair.R2); err != nil {
		return fmt.Errorf("R2: %w", err)
	}
	return nil
}

func validateFASTQ(file configv1.SRAAcquiredFASTQ) error {
	if !filepath.IsAbs(file.Path) || !digestPattern.MatchString(file.SHA256) || file.Bytes < 1 || file.PairedRecordCount < 1 {
		return fmt.Errorf("path, sha256, bytes, and paired_record_count are required")
	}
	return nil
}

func validateReference(scenario configv1.Scenario, reference configv1.SRAAcquisitionReference) error {
	if reference.Primary != nil && len(reference.Species) != 0 {
		return fmt.Errorf("must define exactly one of primary or species")
	}
	if reference.Primary != nil {
		if scenario == configv1.ScenarioBSPDX || scenario == configv1.ScenarioRNAPDX {
			return fmt.Errorf("PDX scenario requires graft and host references")
		}
		if err := validateReferenceSelection(*reference.Primary); err != nil {
			return err
		}
		if reference.Primary.Role != configv1.ReferenceRolePrimary {
			return fmt.Errorf("primary reference must use the primary role")
		}
		return nil
	}
	if scenario != configv1.ScenarioBSPDX && scenario != configv1.ScenarioRNAPDX {
		return fmt.Errorf("non-PDX scenario requires a primary reference")
	}
	if len(reference.Species) != 2 {
		return fmt.Errorf("PDX scenario requires exactly graft and host references")
	}
	seenRoles := make(map[configv1.ReferenceRole]bool, len(reference.Species))
	for _, selection := range reference.Species {
		if err := validateReferenceSelection(selection); err != nil {
			return err
		}
		if seenRoles[selection.Role] {
			return fmt.Errorf("reference role %q is duplicated", selection.Role)
		}
		seenRoles[selection.Role] = true
	}
	if !seenRoles[configv1.ReferenceRoleGraft] || !seenRoles[configv1.ReferenceRoleHost] {
		return fmt.Errorf("PDX scenario requires graft and host references")
	}
	return nil
}

func validateReferenceSelection(selection configv1.SRAReferenceSelection) error {
	if selection.Role != configv1.ReferenceRolePrimary && selection.Role != configv1.ReferenceRoleGraft && selection.Role != configv1.ReferenceRoleHost {
		return fmt.Errorf("role %q is invalid", selection.Role)
	}
	if strings.TrimSpace(selection.ID) == "" || strings.TrimSpace(selection.Release) == "" || !digestPattern.MatchString(selection.ManifestSHA256) {
		return fmt.Errorf("role, id, release, and manifest_sha256 are required")
	}
	return nil
}

func verifyRegularFile(path string, expectedDigest string, expectedBytes int64) error {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect file: %w", err)
	}
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("file is not regular")
	}
	if fileInfo.Size() != expectedBytes {
		return fmt.Errorf("size mismatch: expected %d, got %d", expectedBytes, fileInfo.Size())
	}
	actualDigest, err := digestFile(path)
	if err != nil {
		return err
	}
	if actualDigest != expectedDigest {
		return fmt.Errorf("digest mismatch: expected %s, got %s", expectedDigest, actualDigest)
	}
	return nil
}

func digestFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("digest file: %w", err)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func sortedManifest(manifest configv1.SRAAcquisitionManifest) configv1.SRAAcquisitionManifest {
	sorted := manifest
	sorted.Entries = append([]configv1.SRAAcquisitionEntry(nil), manifest.Entries...)
	sort.Slice(sorted.Entries, func(left, right int) bool {
		return sorted.Entries[left].Scenario < sorted.Entries[right].Scenario
	})
	return sorted
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var trailingValue json.RawMessage
	if err := decoder.Decode(&trailingValue); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not supported")
		}
		return err
	}
	return nil
}

func isScenario(scenario configv1.Scenario) bool {
	switch scenario {
	case configv1.ScenarioRRBS, configv1.ScenarioWGBS, configv1.ScenarioRNASeq, configv1.ScenarioBSPDX, configv1.ScenarioRNAPDX:
		return true
	default:
		return false
	}
}
