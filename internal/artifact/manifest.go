package artifact

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

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

const SchemaVersion = "otter.artifacts/v1"

const DeclarationSchemaVersion = "otter.artifact-declarations/v1"

const DefaultManifestFileName = "artifacts.json"

var (
	digestPattern     = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	runIDPattern      = regexp.MustCompile(`^run-[0-9]{8}T[0-9]{6}Z-[a-z]{6}$`)
	artifactIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

type ComparisonTier string

const (
	ComparisonTierExact         ComparisonTier = "exact"
	ComparisonTierStructural    ComparisonTier = "structural"
	ComparisonTierScientific    ComparisonTier = "scientific"
	ComparisonTierInformational ComparisonTier = "informational"
)

type Manifest struct {
	SchemaVersion     string             `json:"schema_version"`
	RunID             string             `json:"run_id"`
	RunSnapshotDigest string             `json:"run_snapshot_digest"`
	Scenario          configv1.Scenario  `json:"scenario"`
	Toolchain         configv1.Toolchain `json:"toolchain"`
	Executor          configv1.Executor  `json:"executor"`
	Backend           configv1.Backend   `json:"backend"`
	Artifacts         []Entry            `json:"artifacts"`
}

type Entry struct {
	ID         string     `json:"id"`
	Path       string     `json:"path"`
	MediaType  string     `json:"media_type"`
	Schema     string     `json:"schema"`
	Checksum   string     `json:"checksum"`
	Comparison Comparison `json:"comparison"`
}

type Declaration struct {
	ID         string     `json:"id"`
	Path       string     `json:"path"`
	MediaType  string     `json:"media_type"`
	Schema     string     `json:"schema"`
	Comparison Comparison `json:"comparison"`
}

type DeclarationDocument struct {
	SchemaVersion string        `json:"schema_version"`
	Artifacts     []Declaration `json:"artifacts"`
}

type PublicationIdentity struct {
	RunID             string             `json:"run_id"`
	RunSnapshotDigest string             `json:"run_snapshot_digest"`
	Scenario          configv1.Scenario  `json:"scenario"`
	Toolchain         configv1.Toolchain `json:"toolchain"`
	Executor          configv1.Executor  `json:"executor"`
	Backend           configv1.Backend   `json:"backend"`
}

type Comparison struct {
	Tier       ComparisonTier `json:"tier"`
	Comparator string         `json:"comparator"`
}

type VerificationIssue struct {
	ArtifactID string `json:"artifact_id"`
	Path       string `json:"path"`
	Expected   string `json:"expected"`
	Actual     string `json:"actual"`
}

type VerificationReport struct {
	Passed bool                `json:"passed"`
	Issues []VerificationIssue `json:"issues,omitempty"`
}

func Load(path string) (Manifest, error) {
	manifestBytes, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read artifact manifest %q: %w", path, err)
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(manifestBytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse artifact manifest %q: %w", path, err)
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		return Manifest{}, fmt.Errorf("parse artifact manifest %q: %w", path, err)
	}
	if err := Validate(manifest); err != nil {
		return Manifest{}, fmt.Errorf("validate artifact manifest %q: %w", path, err)
	}
	return manifest, nil
}

func LoadDeclarations(path string) ([]Declaration, error) {
	declarationBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read artifact declarations %q: %w", path, err)
	}
	var document DeclarationDocument
	decoder := json.NewDecoder(strings.NewReader(string(declarationBytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse artifact declarations %q: %w", path, err)
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		return nil, fmt.Errorf("parse artifact declarations %q: %w", path, err)
	}
	if err := ValidateDeclarations(document); err != nil {
		return nil, fmt.Errorf("validate artifact declarations %q: %w", path, err)
	}
	return document.Artifacts, nil
}

func ValidateDeclarations(document DeclarationDocument) error {
	if document.SchemaVersion != DeclarationSchemaVersion {
		return fmt.Errorf("schema_version must be %q", DeclarationSchemaVersion)
	}
	if len(document.Artifacts) == 0 {
		return fmt.Errorf("artifacts must not be empty")
	}
	artifactIDs := make(map[string]bool, len(document.Artifacts))
	artifactPaths := make(map[string]bool, len(document.Artifacts))
	for artifactIndex, declaration := range document.Artifacts {
		if err := validateDeclaration(declaration); err != nil {
			return fmt.Errorf("artifacts[%d]: %w", artifactIndex, err)
		}
		if artifactIDs[declaration.ID] {
			return fmt.Errorf("artifacts[%d].id %q is duplicated", artifactIndex, declaration.ID)
		}
		if artifactPaths[declaration.Path] {
			return fmt.Errorf("artifacts[%d].path %q is duplicated", artifactIndex, declaration.Path)
		}
		artifactIDs[declaration.ID] = true
		artifactPaths[declaration.Path] = true
	}
	return nil
}

func Build(
	identity PublicationIdentity,
	resultsRoot string,
	declarations []Declaration,
) (Manifest, error) {
	if err := validatePublicationIdentity(identity); err != nil {
		return Manifest{}, err
	}
	if !filepath.IsAbs(resultsRoot) {
		return Manifest{}, fmt.Errorf("results root must be absolute")
	}
	if len(declarations) == 0 {
		return Manifest{}, fmt.Errorf("artifact declarations must not be empty")
	}
	artifactIDs := make(map[string]bool, len(declarations))
	artifactPaths := make(map[string]bool, len(declarations))
	entries := make([]Entry, 0, len(declarations))
	for declarationIndex, declaration := range declarations {
		if err := validateDeclaration(declaration); err != nil {
			return Manifest{}, fmt.Errorf("artifact declarations[%d]: %w", declarationIndex, err)
		}
		if artifactIDs[declaration.ID] {
			return Manifest{}, fmt.Errorf("artifact declarations[%d].id %q is duplicated", declarationIndex, declaration.ID)
		}
		if artifactPaths[declaration.Path] {
			return Manifest{}, fmt.Errorf("artifact declarations[%d].path %q is duplicated", declarationIndex, declaration.Path)
		}
		artifactPath := filepath.Join(resultsRoot, filepath.FromSlash(declaration.Path))
		checksum, err := digestRegularFile(artifactPath)
		if err != nil {
			return Manifest{}, fmt.Errorf("digest artifact declaration %q: %w", declaration.ID, err)
		}
		artifactIDs[declaration.ID] = true
		artifactPaths[declaration.Path] = true
		entries = append(entries, Entry{
			ID:         declaration.ID,
			Path:       declaration.Path,
			MediaType:  declaration.MediaType,
			Schema:     declaration.Schema,
			Checksum:   checksum,
			Comparison: declaration.Comparison,
		})
	}
	manifest := Manifest{
		SchemaVersion:     SchemaVersion,
		RunID:             identity.RunID,
		RunSnapshotDigest: identity.RunSnapshotDigest,
		Scenario:          identity.Scenario,
		Toolchain:         identity.Toolchain,
		Executor:          identity.Executor,
		Backend:           identity.Backend,
		Artifacts:         entries,
	}
	if err := Validate(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Validate(manifest Manifest) error {
	if err := validatePublicationIdentity(PublicationIdentity{
		RunID:             manifest.RunID,
		RunSnapshotDigest: manifest.RunSnapshotDigest,
		Scenario:          manifest.Scenario,
		Toolchain:         manifest.Toolchain,
		Executor:          manifest.Executor,
		Backend:           manifest.Backend,
	}); err != nil {
		return err
	}
	if len(manifest.Artifacts) == 0 {
		return fmt.Errorf("artifacts must not be empty")
	}

	artifactIDs := make(map[string]bool, len(manifest.Artifacts))
	artifactPaths := make(map[string]bool, len(manifest.Artifacts))
	for artifactIndex, entry := range manifest.Artifacts {
		if err := validateEntry(entry); err != nil {
			return fmt.Errorf("artifacts[%d]: %w", artifactIndex, err)
		}
		if artifactIDs[entry.ID] {
			return fmt.Errorf("artifacts[%d].id %q is duplicated", artifactIndex, entry.ID)
		}
		if artifactPaths[entry.Path] {
			return fmt.Errorf("artifacts[%d].path %q is duplicated", artifactIndex, entry.Path)
		}
		artifactIDs[entry.ID] = true
		artifactPaths[entry.Path] = true
	}
	return nil
}

func Publish(
	identity PublicationIdentity,
	resultsRoot string,
	declarations []Declaration,
) (Manifest, error) {
	manifest, err := Build(identity, resultsRoot, declarations)
	if err != nil {
		return Manifest{}, err
	}
	manifestPath := filepath.Join(resultsRoot, DefaultManifestFileName)
	if err := Write(manifestPath, manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Write(path string, manifest Manifest) error {
	if err := Validate(manifest); err != nil {
		return err
	}
	manifestBytes, err := json.MarshalIndent(sortedManifest(manifest), "", "  ")
	if err != nil {
		return fmt.Errorf("encode artifact manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create artifact manifest directory: %w", err)
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(path), ".artifacts-*.json")
	if err != nil {
		return fmt.Errorf("create temporary artifact manifest: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporaryFile.Write(manifestBytes); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("write temporary artifact manifest: %w", err)
	}
	if err := temporaryFile.Chmod(0o444); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("protect temporary artifact manifest: %w", err)
	}
	if err := temporaryFile.Sync(); err != nil {
		temporaryFile.Close()
		return fmt.Errorf("sync temporary artifact manifest: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("close temporary artifact manifest: %w", err)
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return fmt.Errorf("publish immutable artifact manifest %q: %w", path, err)
	}
	return nil
}

func Verify(resultsRoot string, manifest Manifest) (VerificationReport, error) {
	if err := Validate(manifest); err != nil {
		return VerificationReport{}, err
	}
	if !filepath.IsAbs(resultsRoot) {
		return VerificationReport{}, fmt.Errorf("results root must be absolute")
	}
	report := VerificationReport{Passed: true}
	for _, entry := range manifest.Artifacts {
		artifactPath := filepath.Join(resultsRoot, filepath.FromSlash(entry.Path))
		actualChecksum, err := digestRegularFile(artifactPath)
		if err != nil {
			report.Passed = false
			report.Issues = append(report.Issues, VerificationIssue{
				ArtifactID: entry.ID,
				Path:       entry.Path,
				Expected:   entry.Checksum,
				Actual:     err.Error(),
			})
			continue
		}
		if actualChecksum != entry.Checksum {
			report.Passed = false
			report.Issues = append(report.Issues, VerificationIssue{
				ArtifactID: entry.ID,
				Path:       entry.Path,
				Expected:   entry.Checksum,
				Actual:     actualChecksum,
			})
		}
	}
	return report, nil
}

func validatePublicationIdentity(identity PublicationIdentity) error {
	if !runIDPattern.MatchString(identity.RunID) {
		return fmt.Errorf("run_id %q is invalid", identity.RunID)
	}
	if !digestPattern.MatchString(identity.RunSnapshotDigest) {
		return fmt.Errorf("run_snapshot_digest is invalid")
	}
	if !isScenario(identity.Scenario) {
		return fmt.Errorf("scenario %q is invalid", identity.Scenario)
	}
	if identity.Toolchain != configv1.ToolchainModern && identity.Toolchain != configv1.ToolchainLegacyEquivalent {
		return fmt.Errorf("toolchain %q is invalid", identity.Toolchain)
	}
	if identity.Executor != configv1.ExecutorCraftmake && identity.Executor != configv1.ExecutorSnakemake {
		return fmt.Errorf("executor %q is invalid", identity.Executor)
	}
	if identity.Backend != configv1.BackendLocal && identity.Backend != configv1.BackendSlurm {
		return fmt.Errorf("backend %q is invalid", identity.Backend)
	}
	return nil
}

func validateDeclaration(declaration Declaration) error {
	if !artifactIDPattern.MatchString(declaration.ID) {
		return fmt.Errorf("id %q is invalid", declaration.ID)
	}
	if err := validateRelativePath(declaration.Path); err != nil {
		return fmt.Errorf("path: %w", err)
	}
	if strings.TrimSpace(declaration.MediaType) == "" {
		return fmt.Errorf("media_type is required")
	}
	if strings.TrimSpace(declaration.Schema) == "" {
		return fmt.Errorf("schema is required")
	}
	switch declaration.Comparison.Tier {
	case ComparisonTierExact, ComparisonTierStructural, ComparisonTierScientific, ComparisonTierInformational:
	default:
		return fmt.Errorf("comparison.tier %q is invalid", declaration.Comparison.Tier)
	}
	if strings.TrimSpace(declaration.Comparison.Comparator) == "" {
		return fmt.Errorf("comparison.comparator is required")
	}
	return nil
}

func validateEntry(entry Entry) error {
	if err := validateDeclaration(Declaration{
		ID:         entry.ID,
		Path:       entry.Path,
		MediaType:  entry.MediaType,
		Schema:     entry.Schema,
		Comparison: entry.Comparison,
	}); err != nil {
		return err
	}
	if !digestPattern.MatchString(entry.Checksum) {
		return fmt.Errorf("checksum is invalid")
	}
	return nil
}

func validateRelativePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("must not be empty")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("must be relative")
	}
	if strings.Contains(path, "\\") {
		return fmt.Errorf("must use slash-separated paths")
	}
	cleanedPath := filepath.Clean(filepath.FromSlash(path))
	if cleanedPath == "." || cleanedPath == ".." || strings.HasPrefix(cleanedPath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("must not escape results root")
	}
	return nil
}

func digestRegularFile(path string) (string, error) {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect artifact: %w", err)
	}
	if !fileInfo.Mode().IsRegular() {
		return "", fmt.Errorf("artifact is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open artifact: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("digest artifact: %w", err)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func sortedManifest(manifest Manifest) Manifest {
	sorted := manifest
	sorted.Artifacts = append([]Entry(nil), manifest.Artifacts...)
	sort.Slice(sorted.Artifacts, func(left, right int) bool {
		return sorted.Artifacts[left].ID < sorted.Artifacts[right].ID
	})
	return sorted
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var trailingValue any
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
