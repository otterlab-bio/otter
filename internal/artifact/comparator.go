package artifact

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/360EntSecGroup-Skylar/excelize"
)

const (
	ComparatorExactFile             = "exact-file/v1"
	ComparatorTSVStructure          = "tsv-structure/v1"
	ComparatorExpressionMatrix      = "expression-count-matrix/v1"
	ComparatorPDXGraftReadCounts    = "pdx-graft-read-counts/v1"
	ComparatorBAMPairStructure      = "bam-pair-structure/v1"
	ComparatorXLSXStructure         = "xlsx-structure/v1"
	ComparatorHTMLDocument          = "html-document/v1"
	ComparatorMethrixSemantics      = "methrix-semantics/v1"
	ComparatorRNASplicingOutcome    = "rna-splicing-outcome/v1"
	ComparatorUnsupportedSchemaOnly = "schema-only/v1"
)

var ErrComparatorUnavailable = errors.New("artifact comparator is unavailable")

type Comparator interface {
	Compare(ComparisonInput) (ComparisonResult, error)
}

type ComparisonInput struct {
	ArtifactID       string
	LeftEntry        Entry
	RightEntry       Entry
	LeftResultsRoot  string
	RightResultsRoot string
	LeftManifest     Manifest
	RightManifest    Manifest
}

type ComparisonResult struct {
	Passed  bool
	Details string
}

type ComparisonIssue struct {
	ArtifactID string `json:"artifact_id"`
	Comparator string `json:"comparator"`
	Tier       string `json:"tier"`
	Details    string `json:"details"`
}

type ComparisonReport struct {
	Passed      bool              `json:"passed"`
	Comparable  bool              `json:"comparable"`
	Issues      []ComparisonIssue `json:"issues,omitempty"`
	ComparedIDs []string          `json:"compared_ids"`
}

type ComparatorRegistry struct {
	comparators map[string]Comparator
}

func NewComparatorRegistry() *ComparatorRegistry {
	return &ComparatorRegistry{comparators: map[string]Comparator{
		ComparatorExactFile:          exactFileComparator{},
		ComparatorTSVStructure:       tsvComparator{numericValues: false},
		ComparatorExpressionMatrix:   tsvComparator{numericValues: true},
		ComparatorPDXGraftReadCounts: pdxGraftReadCountsComparator{},
		ComparatorBAMPairStructure:   bamPairComparator{},
		ComparatorXLSXStructure:      xlsxStructureComparator{},
		ComparatorHTMLDocument:       htmlDocumentComparator{},
		"bismark-summary/v1":         htmlDocumentComparator{},
		ComparatorMethrixSemantics:   methrixComparator{},
		ComparatorRNASplicingOutcome: rnaSplicingOutcomeComparator{},
	}}
}

func (registry *ComparatorRegistry) Compare(leftResultsRoot string, leftManifest Manifest, rightResultsRoot string, rightManifest Manifest) (ComparisonReport, error) {
	if registry == nil {
		return ComparisonReport{}, fmt.Errorf("artifact comparator registry is required")
	}
	if err := Validate(leftManifest); err != nil {
		return ComparisonReport{}, fmt.Errorf("validate left artifact manifest: %w", err)
	}
	if err := Validate(rightManifest); err != nil {
		return ComparisonReport{}, fmt.Errorf("validate right artifact manifest: %w", err)
	}
	if !filepath.IsAbs(leftResultsRoot) || !filepath.IsAbs(rightResultsRoot) {
		return ComparisonReport{}, fmt.Errorf("comparison results roots must be absolute")
	}

	report := ComparisonReport{Passed: true, Comparable: true}
	leftEntries := entriesByID(leftManifest.Artifacts)
	rightEntries := entriesByID(rightManifest.Artifacts)
	artifactIDs := unionArtifactIDs(leftEntries, rightEntries)
	for _, artifactID := range artifactIDs {
		leftEntry, leftPresent := leftEntries[artifactID]
		rightEntry, rightPresent := rightEntries[artifactID]
		if !leftPresent || !rightPresent {
			report.Passed = false
			report.Comparable = false
			report.Issues = append(report.Issues, ComparisonIssue{
				ArtifactID: artifactID,
				Details:    "artifact is absent from one manifest",
			})
			continue
		}
		report.ComparedIDs = append(report.ComparedIDs, artifactID)
		if leftEntry.Schema != rightEntry.Schema || leftEntry.Comparison != rightEntry.Comparison {
			report.Passed = false
			report.Comparable = false
			report.Issues = append(report.Issues, ComparisonIssue{
				ArtifactID: artifactID,
				Comparator: leftEntry.Comparison.Comparator,
				Tier:       string(leftEntry.Comparison.Tier),
				Details:    "artifact schema or comparison contract differs between manifests",
			})
			continue
		}
		comparator, present := registry.comparators[leftEntry.Comparison.Comparator]
		if !present {
			report.Passed = false
			report.Comparable = false
			report.Issues = append(report.Issues, ComparisonIssue{
				ArtifactID: artifactID,
				Comparator: leftEntry.Comparison.Comparator,
				Tier:       string(leftEntry.Comparison.Tier),
				Details:    "no registered implementation for declared comparator",
			})
			continue
		}
		result, err := comparator.Compare(ComparisonInput{
			ArtifactID:       artifactID,
			LeftEntry:        leftEntry,
			RightEntry:       rightEntry,
			LeftResultsRoot:  leftResultsRoot,
			RightResultsRoot: rightResultsRoot,
			LeftManifest:     leftManifest,
			RightManifest:    rightManifest,
		})
		if err != nil {
			report.Passed = false
			if errors.Is(err, ErrComparatorUnavailable) {
				report.Comparable = false
			}
			report.Issues = append(report.Issues, ComparisonIssue{
				ArtifactID: artifactID,
				Comparator: leftEntry.Comparison.Comparator,
				Tier:       string(leftEntry.Comparison.Tier),
				Details:    err.Error(),
			})
			continue
		}
		if !result.Passed {
			report.Passed = false
			report.Issues = append(report.Issues, ComparisonIssue{
				ArtifactID: artifactID,
				Comparator: leftEntry.Comparison.Comparator,
				Tier:       string(leftEntry.Comparison.Tier),
				Details:    result.Details,
			})
		}
	}
	return report, nil
}

func entriesByID(entries []Entry) map[string]Entry {
	indexedEntries := make(map[string]Entry, len(entries))
	for _, entry := range entries {
		indexedEntries[entry.ID] = entry
	}
	return indexedEntries
}

func unionArtifactIDs(leftEntries map[string]Entry, rightEntries map[string]Entry) []string {
	identifierSet := make(map[string]bool, len(leftEntries)+len(rightEntries))
	for artifactID := range leftEntries {
		identifierSet[artifactID] = true
	}
	for artifactID := range rightEntries {
		identifierSet[artifactID] = true
	}
	artifactIDs := make([]string, 0, len(identifierSet))
	for artifactID := range identifierSet {
		artifactIDs = append(artifactIDs, artifactID)
	}
	sort.Strings(artifactIDs)
	return artifactIDs
}

func resolveComparisonPath(resultsRoot string, entry Entry) string {
	return filepath.Join(resultsRoot, filepath.FromSlash(entry.Path))
}

type exactFileComparator struct{}

func (exactFileComparator) Compare(input ComparisonInput) (ComparisonResult, error) {
	if input.LeftEntry.Checksum != input.RightEntry.Checksum {
		return ComparisonResult{Details: "artifact checksums differ"}, nil
	}
	return ComparisonResult{Passed: true, Details: "artifact checksums match"}, nil
}

type tsvComparator struct {
	numericValues bool
}

func (comparator tsvComparator) Compare(input ComparisonInput) (ComparisonResult, error) {
	leftTable, err := readTSV(resolveComparisonPath(input.LeftResultsRoot, input.LeftEntry))
	if err != nil {
		return ComparisonResult{}, err
	}
	rightTable, err := readTSV(resolveComparisonPath(input.RightResultsRoot, input.RightEntry))
	if err != nil {
		return ComparisonResult{}, err
	}
	if !equalStringSlices(leftTable.header, rightTable.header) {
		return ComparisonResult{Details: "TSV headers differ"}, nil
	}
	if len(leftTable.rows) != len(rightTable.rows) {
		return ComparisonResult{Details: "TSV row counts differ"}, nil
	}
	for rowIndex := range leftTable.rows {
		leftRow := leftTable.rows[rowIndex]
		rightRow := rightTable.rows[rowIndex]
		if len(leftRow) != len(rightRow) {
			return ComparisonResult{Details: fmt.Sprintf("TSV row %d column counts differ", rowIndex+1)}, nil
		}
		if !comparator.numericValues {
			continue
		}
		if len(leftRow) == 0 || leftRow[0] != rightRow[0] {
			return ComparisonResult{Details: fmt.Sprintf("TSV row %d feature identifiers differ", rowIndex+1)}, nil
		}
		for columnIndex := 1; columnIndex < len(leftRow); columnIndex++ {
			leftValue, leftErr := strconv.ParseFloat(leftRow[columnIndex], 64)
			rightValue, rightErr := strconv.ParseFloat(rightRow[columnIndex], 64)
			if leftErr != nil || rightErr != nil {
				return ComparisonResult{Details: fmt.Sprintf("TSV row %d column %d requires numeric values", rowIndex+1, columnIndex+1)}, nil
			}
			if math.Abs(leftValue-rightValue) > 1e-6 {
				return ComparisonResult{Details: fmt.Sprintf("TSV row %d column %d exceeds numeric tolerance", rowIndex+1, columnIndex+1)}, nil
			}
		}
	}
	return ComparisonResult{Passed: true, Details: "TSV structure is equivalent"}, nil
}

type pdxGraftReadCountsComparator struct{}

func (pdxGraftReadCountsComparator) Compare(input ComparisonInput) (ComparisonResult, error) {
	leftTable, err := readTSV(resolveComparisonPath(input.LeftResultsRoot, input.LeftEntry))
	if err != nil {
		return ComparisonResult{}, err
	}
	rightTable, err := readTSV(resolveComparisonPath(input.RightResultsRoot, input.RightEntry))
	if err != nil {
		return ComparisonResult{}, err
	}
	expectedHeader := []string{"artifact_index", "source_bam", "source_bai", "mapped_reads"}
	if !equalStringSlices(leftTable.header, expectedHeader) || !equalStringSlices(rightTable.header, expectedHeader) {
		return ComparisonResult{Details: "PDX classification header is invalid"}, nil
	}
	if err := validatePDXClassificationManifest(leftTable, input.LeftManifest); err != nil {
		return ComparisonResult{}, err
	}
	if err := validatePDXClassificationManifest(rightTable, input.RightManifest); err != nil {
		return ComparisonResult{}, err
	}
	if len(leftTable.rows) != len(rightTable.rows) {
		return ComparisonResult{Details: "PDX graft classification row counts differ"}, nil
	}
	for rowIndex := range leftTable.rows {
		leftRow := leftTable.rows[rowIndex]
		rightRow := rightTable.rows[rowIndex]
		if len(leftRow) != len(expectedHeader) || len(rightRow) != len(expectedHeader) {
			return ComparisonResult{Details: fmt.Sprintf("PDX classification row %d has invalid width", rowIndex+1)}, nil
		}
		leftMappedReads, leftErr := strconv.ParseInt(leftRow[3], 10, 64)
		rightMappedReads, rightErr := strconv.ParseInt(rightRow[3], 10, 64)
		if leftErr != nil || rightErr != nil || leftMappedReads <= 0 || rightMappedReads <= 0 {
			return ComparisonResult{Details: fmt.Sprintf("PDX classification row %d has invalid mapped_reads", rowIndex+1)}, nil
		}
		if leftRow[1] != rightRow[1] || leftRow[2] != rightRow[2] || leftMappedReads != rightMappedReads {
			return ComparisonResult{Details: fmt.Sprintf("PDX classification row %d differs", rowIndex+1)}, nil
		}
	}
	return ComparisonResult{Passed: true, Details: "PDX graft classification is equivalent"}, nil
}

func validatePDXClassificationManifest(table tsvTable, manifest Manifest) error {
	declaredBAMPaths := make([]string, 0)
	for _, entry := range manifest.Artifacts {
		if strings.HasSuffix(entry.Path, ".bam") && strings.HasPrefix(entry.Path, "pdx/graft/") {
			declaredBAMPaths = append(declaredBAMPaths, entry.Path)
		}
	}
	sort.Strings(declaredBAMPaths)
	if len(declaredBAMPaths) == 0 {
		return fmt.Errorf("PDX classification requires declared graft BAM artifacts")
	}
	if len(table.rows) != len(declaredBAMPaths) {
		return fmt.Errorf("PDX classification rows do not match declared graft BAM artifacts")
	}
	for rowIndex, bamPath := range declaredBAMPaths {
		row := table.rows[rowIndex]
		if len(row) != 4 {
			return fmt.Errorf("PDX classification row %d has invalid width", rowIndex+1)
		}
		expectedIndex := fmt.Sprintf("%04d", rowIndex+1)
		expectedBAMName := filepath.Base(bamPath)
		expectedBAIName := expectedBAMName + ".bai"
		if row[0] != expectedIndex || row[1] != expectedBAMName || row[2] != expectedBAIName {
			return fmt.Errorf("PDX classification row %d does not match declared BAM/BAI pair", rowIndex+1)
		}
		if !manifestContainsArtifactPath(manifest, bamPath+".bai") {
			return fmt.Errorf("PDX classification BAM index %q is not declared", bamPath+".bai")
		}
	}
	return nil
}

type rnaSplicingOutcomeComparator struct{}

type rnaSplicingOutcome struct {
	SchemaVersion string   `json:"schema_version"`
	Status        string   `json:"status"`
	SourceRoot    string   `json:"source_root"`
	ArtifactPaths []string `json:"artifact_paths"`
}

func (rnaSplicingOutcomeComparator) Compare(input ComparisonInput) (ComparisonResult, error) {
	leftOutcome, err := loadRNASplicingOutcome(resolveComparisonPath(input.LeftResultsRoot, input.LeftEntry))
	if err != nil {
		return ComparisonResult{}, err
	}
	rightOutcome, err := loadRNASplicingOutcome(resolveComparisonPath(input.RightResultsRoot, input.RightEntry))
	if err != nil {
		return ComparisonResult{}, err
	}
	if err := validateRNASplicingOutcomeManifest(leftOutcome, input.LeftEntry, input.LeftManifest); err != nil {
		return ComparisonResult{}, err
	}
	if err := validateRNASplicingOutcomeManifest(rightOutcome, input.RightEntry, input.RightManifest); err != nil {
		return ComparisonResult{}, err
	}
	if leftOutcome.Status != rightOutcome.Status {
		return ComparisonResult{Details: "RNA splicing outcome statuses differ"}, nil
	}
	if !equalStringSlices(leftOutcome.ArtifactPaths, rightOutcome.ArtifactPaths) {
		return ComparisonResult{Details: "RNA splicing outcome artifact paths differ"}, nil
	}
	return ComparisonResult{Passed: true, Details: "RNA splicing outcome structure is equivalent"}, nil
}

func loadRNASplicingOutcome(path string) (rnaSplicingOutcome, error) {
	outcomeContent, err := os.ReadFile(path)
	if err != nil {
		return rnaSplicingOutcome{}, fmt.Errorf("read RNA splicing outcome %q: %w", path, err)
	}
	var outcome rnaSplicingOutcome
	decoder := json.NewDecoder(bytes.NewReader(outcomeContent))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&outcome); err != nil {
		return rnaSplicingOutcome{}, fmt.Errorf("parse RNA splicing outcome %q: %w", path, err)
	}
	var trailingValue json.RawMessage
	if err := decoder.Decode(&trailingValue); err != io.EOF {
		if err == nil {
			return rnaSplicingOutcome{}, fmt.Errorf("parse RNA splicing outcome %q: multiple JSON values are not supported", path)
		}
		return rnaSplicingOutcome{}, fmt.Errorf("parse RNA splicing outcome %q: %w", path, err)
	}
	if outcome.SchemaVersion != "otter.rna-splicing-outcome/v1" {
		return rnaSplicingOutcome{}, fmt.Errorf("RNA splicing outcome %q has unsupported schema %q", path, outcome.SchemaVersion)
	}
	if strings.TrimSpace(outcome.SourceRoot) == "" || !filepath.IsAbs(outcome.SourceRoot) {
		return rnaSplicingOutcome{}, fmt.Errorf("RNA splicing outcome %q must have an absolute source_root", path)
	}
	if outcome.Status != "produced" && outcome.Status != "not_applicable" {
		return rnaSplicingOutcome{}, fmt.Errorf("RNA splicing outcome %q has invalid status %q", path, outcome.Status)
	}
	if outcome.Status == "produced" && len(outcome.ArtifactPaths) == 0 {
		return rnaSplicingOutcome{}, fmt.Errorf("produced RNA splicing outcome %q has no artifact paths", path)
	}
	if outcome.Status == "not_applicable" && len(outcome.ArtifactPaths) != 0 {
		return rnaSplicingOutcome{}, fmt.Errorf("not_applicable RNA splicing outcome %q declares artifact paths", path)
	}
	artifactPathSet := make(map[string]bool, len(outcome.ArtifactPaths))
	for artifactIndex, artifactPath := range outcome.ArtifactPaths {
		cleanedArtifactPath, err := safeRNASplicingArtifactPath(artifactPath)
		if err != nil {
			return rnaSplicingOutcome{}, fmt.Errorf("RNA splicing outcome %q artifact_paths[%d]: %w", path, artifactIndex, err)
		}
		if artifactPathSet[cleanedArtifactPath] {
			return rnaSplicingOutcome{}, fmt.Errorf("RNA splicing outcome %q has duplicated artifact path %q", path, cleanedArtifactPath)
		}
		artifactPathSet[cleanedArtifactPath] = true
		outcome.ArtifactPaths[artifactIndex] = cleanedArtifactPath
	}
	sort.Strings(outcome.ArtifactPaths)
	return outcome, nil
}

func safeRNASplicingArtifactPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") {
		return "", fmt.Errorf("unsafe artifact path %q", path)
	}
	cleanedPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if cleanedPath == "." || cleanedPath == ".." || strings.HasPrefix(cleanedPath, "../") {
		return "", fmt.Errorf("unsafe artifact path %q", path)
	}
	return cleanedPath, nil
}

func validateRNASplicingOutcomeManifest(outcome rnaSplicingOutcome, outcomeEntry Entry, manifest Manifest) error {
	outcomeDirectory := filepath.ToSlash(filepath.Dir(outcomeEntry.Path))
	expectedOutputPaths := make([]string, len(outcome.ArtifactPaths))
	for artifactIndex, artifactPath := range outcome.ArtifactPaths {
		expectedOutputPaths[artifactIndex] = outcomeDirectory + "/files/" + artifactPath
	}
	sort.Strings(expectedOutputPaths)
	declaredOutputPaths := make([]string, 0)
	outputPrefix := outcomeDirectory + "/files/"
	for _, entry := range manifest.Artifacts {
		if strings.HasPrefix(entry.Path, outputPrefix) {
			declaredOutputPaths = append(declaredOutputPaths, entry.Path)
		}
	}
	sort.Strings(declaredOutputPaths)
	if !equalStringSlices(expectedOutputPaths, declaredOutputPaths) {
		return fmt.Errorf("RNA splicing outcome %q does not match declared published output paths", outcomeEntry.Path)
	}
	return nil
}

const (
	bamMagic                    = "BAM\x01"
	baiMagic                    = "BAI\x01"
	bamMaximumHeaderTextLength  = 16 * 1024 * 1024
	bamMaximumReferenceCount    = 1_000_000
	bamMaximumReferenceNameSize = 1 * 1024 * 1024
	baiMaximumBinsPerReference  = 100_000
	baiMaximumChunksPerBin      = 1_000_000
	baiMaximumLinearOffsets     = 1_000_000
)

type bamPairComparator struct{}

func (bamPairComparator) Compare(input ComparisonInput) (ComparisonResult, error) {
	if err := validateBAMPair(input.LeftEntry, input.LeftManifest, input.LeftResultsRoot); err != nil {
		return ComparisonResult{}, err
	}
	if err := validateBAMPair(input.RightEntry, input.RightManifest, input.RightResultsRoot); err != nil {
		return ComparisonResult{}, err
	}
	return ComparisonResult{Passed: true, Details: "BAM/BAI pairs have valid declared binary structures on both sides"}, nil
}

func validateBAMPair(entry Entry, manifest Manifest, resultsRoot string) error {
	bamPath, baiPath, err := bamAndBAIPaths(entry.Path)
	if err != nil {
		return err
	}
	if !manifestContainsArtifactPath(manifest, baiPath) {
		return fmt.Errorf("BAM pair artifact %q is not declared", baiPath)
	}
	if !manifestContainsArtifactPath(manifest, bamPath) {
		return fmt.Errorf("BAM pair artifact %q is not declared", bamPath)
	}
	bamReferenceCount, err := validateBAMFile(filepath.Join(resultsRoot, filepath.FromSlash(bamPath)))
	if err != nil {
		return err
	}
	baiReferenceCount, err := validateBAIFile(filepath.Join(resultsRoot, filepath.FromSlash(baiPath)))
	if err != nil {
		return err
	}
	if bamReferenceCount != baiReferenceCount {
		return fmt.Errorf("BAM/BAI reference counts differ for %q and %q", bamPath, baiPath)
	}
	return nil
}

func bamAndBAIPaths(path string) (string, string, error) {
	if strings.HasSuffix(path, ".bam") {
		return path, path + ".bai", nil
	}
	if strings.HasSuffix(path, ".bam.bai") {
		return strings.TrimSuffix(path, ".bai"), path, nil
	}
	return "", "", fmt.Errorf("BAM pair comparator received unsupported path %q", path)
}

func manifestContainsArtifactPath(manifest Manifest, path string) bool {
	for _, candidate := range manifest.Artifacts {
		if candidate.Path == path {
			return true
		}
	}
	return false
}

func validateBAMFile(path string) (int32, error) {
	if err := requireNonEmptyRegularArtifact(path, "BAM"); err != nil {
		return 0, err
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open BAM artifact %q: %w", path, err)
	}
	defer file.Close()
	if err := validateBGZFHeader(file); err != nil {
		return 0, fmt.Errorf("validate BAM BGZF header %q: %w", path, err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("rewind BAM artifact %q: %w", path, err)
	}
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, fmt.Errorf("open BAM BGZF stream %q: %w", path, err)
	}
	defer gzipReader.Close()

	magic := make([]byte, len(bamMagic))
	if _, err := io.ReadFull(gzipReader, magic); err != nil {
		return 0, fmt.Errorf("read BAM magic %q: %w", path, err)
	}
	if string(magic) != bamMagic {
		return 0, fmt.Errorf("BAM artifact %q has invalid BAM magic", path)
	}
	headerTextLength, err := readBAMInt32(gzipReader, path, "header text length")
	if err != nil {
		return 0, err
	}
	if headerTextLength < 0 || headerTextLength > bamMaximumHeaderTextLength {
		return 0, fmt.Errorf("BAM artifact %q has invalid header text length %d", path, headerTextLength)
	}
	if _, err := io.CopyN(io.Discard, gzipReader, int64(headerTextLength)); err != nil {
		return 0, fmt.Errorf("read BAM header text %q: %w", path, err)
	}
	referenceCount, err := readBAMInt32(gzipReader, path, "reference count")
	if err != nil {
		return 0, err
	}
	if referenceCount < 0 || referenceCount > bamMaximumReferenceCount {
		return 0, fmt.Errorf("BAM artifact %q has invalid reference count %d", path, referenceCount)
	}
	for referenceIndex := int32(0); referenceIndex < referenceCount; referenceIndex++ {
		if err := validateBAMReference(gzipReader, path, referenceIndex); err != nil {
			return 0, err
		}
	}
	if _, err := io.Copy(io.Discard, gzipReader); err != nil {
		return 0, fmt.Errorf("read BAM BGZF stream %q: %w", path, err)
	}
	return referenceCount, nil
}

func validateBGZFHeader(reader io.Reader) error {
	header := make([]byte, 18)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}
	if !bytes.Equal(header[:4], []byte{0x1f, 0x8b, 0x08, 0x04}) || binary.LittleEndian.Uint16(header[10:12]) != 6 || !bytes.Equal(header[12:16], []byte{'B', 'C', 0x02, 0x00}) {
		return fmt.Errorf("missing required BGZF extra field")
	}
	if binary.LittleEndian.Uint16(header[16:18]) < 25 {
		return fmt.Errorf("invalid BGZF block size")
	}
	return nil
}

func readBAMInt32(reader io.Reader, path string, field string) (int32, error) {
	var encodedValue [4]byte
	if _, err := io.ReadFull(reader, encodedValue[:]); err != nil {
		return 0, fmt.Errorf("read BAM %s %q: %w", field, path, err)
	}
	return int32(binary.LittleEndian.Uint32(encodedValue[:])), nil
}

func validateBAMReference(reader io.Reader, path string, referenceIndex int32) error {
	referenceNameLength, err := readBAMInt32(reader, path, "reference name length")
	if err != nil {
		return err
	}
	if referenceNameLength < 2 || referenceNameLength > bamMaximumReferenceNameSize {
		return fmt.Errorf("BAM artifact %q has invalid reference name length at index %d", path, referenceIndex)
	}
	referenceName := make([]byte, referenceNameLength)
	if _, err := io.ReadFull(reader, referenceName); err != nil {
		return fmt.Errorf("read BAM reference name %q: %w", path, err)
	}
	if referenceName[len(referenceName)-1] != '\x00' {
		return fmt.Errorf("BAM artifact %q has unterminated reference name at index %d", path, referenceIndex)
	}
	referenceLength, err := readBAMInt32(reader, path, "reference length")
	if err != nil {
		return err
	}
	if referenceLength < 1 {
		return fmt.Errorf("BAM artifact %q has invalid reference length at index %d", path, referenceIndex)
	}
	return nil
}

func requireNonEmptyRegularArtifact(path string, artifactType string) error {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s artifact %q: %w", artifactType, path, err)
	}
	if !fileInfo.Mode().IsRegular() || fileInfo.Size() == 0 {
		return fmt.Errorf("%s artifact %q must be a non-empty regular file", artifactType, path)
	}
	return nil
}

func validateBAIFile(path string) (int32, error) {
	if err := requireNonEmptyRegularArtifact(path, "BAI"); err != nil {
		return 0, err
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open BAI artifact %q: %w", path, err)
	}
	defer file.Close()
	header := make([]byte, 8)
	if _, err := io.ReadFull(file, header); err != nil {
		return 0, fmt.Errorf("read BAI header %q: %w", path, err)
	}
	if string(header[:4]) != baiMagic {
		return 0, fmt.Errorf("BAI artifact %q has invalid BAI magic", path)
	}
	referenceCount := int32(binary.LittleEndian.Uint32(header[4:8]))
	if referenceCount < 0 || referenceCount > bamMaximumReferenceCount {
		return 0, fmt.Errorf("BAI artifact %q has invalid reference count %d", path, referenceCount)
	}
	for referenceIndex := int32(0); referenceIndex < referenceCount; referenceIndex++ {
		if err := validateBAIReference(file, path, referenceIndex); err != nil {
			return 0, err
		}
	}
	return referenceCount, nil
}

func validateBAIReference(reader io.Reader, path string, referenceIndex int32) error {
	binCount, err := readBAIInt32(reader, path, "bin count")
	if err != nil {
		return err
	}
	if binCount < 0 || binCount > baiMaximumBinsPerReference {
		return fmt.Errorf("BAI artifact %q has invalid bin count %d at reference index %d", path, binCount, referenceIndex)
	}
	for binIndex := int32(0); binIndex < binCount; binIndex++ {
		if _, err := readBAIInt32(reader, path, "bin identifier"); err != nil {
			return err
		}
		chunkCount, err := readBAIInt32(reader, path, "chunk count")
		if err != nil {
			return err
		}
		if chunkCount < 0 || chunkCount > baiMaximumChunksPerBin {
			return fmt.Errorf("BAI artifact %q has invalid chunk count %d at reference index %d bin index %d", path, chunkCount, referenceIndex, binIndex)
		}
		if _, err := io.CopyN(io.Discard, reader, int64(chunkCount)*16); err != nil {
			return fmt.Errorf("read BAI chunks %q at reference index %d bin index %d: %w", path, referenceIndex, binIndex, err)
		}
	}
	linearOffsetCount, err := readBAIInt32(reader, path, "linear offset count")
	if err != nil {
		return err
	}
	if linearOffsetCount < 0 || linearOffsetCount > baiMaximumLinearOffsets {
		return fmt.Errorf("BAI artifact %q has invalid linear offset count %d at reference index %d", path, linearOffsetCount, referenceIndex)
	}
	if _, err := io.CopyN(io.Discard, reader, int64(linearOffsetCount)*8); err != nil {
		return fmt.Errorf("read BAI linear offsets %q at reference index %d: %w", path, referenceIndex, err)
	}
	return nil
}

func readBAIInt32(reader io.Reader, path string, field string) (int32, error) {
	var encodedValue [4]byte
	if _, err := io.ReadFull(reader, encodedValue[:]); err != nil {
		return 0, fmt.Errorf("read BAI %s %q: %w", field, path, err)
	}
	return int32(binary.LittleEndian.Uint32(encodedValue[:])), nil
}

type xlsxStructureComparator struct{}

func (xlsxStructureComparator) Compare(input ComparisonInput) (ComparisonResult, error) {
	leftPath := resolveComparisonPath(input.LeftResultsRoot, input.LeftEntry)
	rightPath := resolveComparisonPath(input.RightResultsRoot, input.RightEntry)
	leftMembers, err := xlsxMemberNames(leftPath)
	if err != nil {
		return ComparisonResult{}, err
	}
	rightMembers, err := xlsxMemberNames(rightPath)
	if err != nil {
		return ComparisonResult{}, err
	}
	if !equalStringSlices(leftMembers, rightMembers) {
		return ComparisonResult{Details: "XLSX package members differ"}, nil
	}
	leftSheetTitles, err := xlsxSheetTitles(leftPath)
	if err != nil {
		return ComparisonResult{}, err
	}
	rightSheetTitles, err := xlsxSheetTitles(rightPath)
	if err != nil {
		return ComparisonResult{}, err
	}
	if !equalStringSlices(leftSheetTitles, rightSheetTitles) {
		return ComparisonResult{Details: "XLSX workbook sheet titles differ"}, nil
	}
	return ComparisonResult{Passed: true, Details: "XLSX package and workbook sheet structure are equivalent"}, nil
}

func xlsxMemberNames(path string) ([]string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open XLSX package %q: %w", path, err)
	}
	defer archive.Close()
	memberNames := make([]string, 0, len(archive.File))
	for _, member := range archive.File {
		memberNames = append(memberNames, member.Name)
	}
	sort.Strings(memberNames)
	if !containsString(memberNames, "[Content_Types].xml") || !containsString(memberNames, "xl/workbook.xml") {
		return nil, fmt.Errorf("XLSX package %q is missing required workbook members", path)
	}
	return memberNames, nil
}

func xlsxSheetTitles(path string) ([]string, error) {
	workbook, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open XLSX workbook %q: %w", path, err)
	}
	sheetMap := workbook.GetSheetMap()
	if len(sheetMap) == 0 {
		return nil, fmt.Errorf("XLSX workbook %q has no sheets", path)
	}
	sheetIndexes := make([]int, 0, len(sheetMap))
	for sheetIndex := range sheetMap {
		sheetIndexes = append(sheetIndexes, sheetIndex)
	}
	sort.Ints(sheetIndexes)
	sheetTitles := make([]string, 0, len(sheetIndexes))
	for _, sheetIndex := range sheetIndexes {
		sheetTitle := sheetMap[sheetIndex]
		if sheetTitle == "" {
			return nil, fmt.Errorf("XLSX workbook %q has an empty sheet title", path)
		}
		sheetTitles = append(sheetTitles, sheetTitle)
	}
	return sheetTitles, nil
}

type htmlDocumentComparator struct{}

func (htmlDocumentComparator) Compare(input ComparisonInput) (ComparisonResult, error) {
	for _, path := range []string{
		resolveComparisonPath(input.LeftResultsRoot, input.LeftEntry),
		resolveComparisonPath(input.RightResultsRoot, input.RightEntry),
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			return ComparisonResult{}, fmt.Errorf("read HTML artifact %q: %w", path, err)
		}
		if !bytes.Contains(bytes.ToLower(content), []byte("<html")) {
			return ComparisonResult{Details: "HTML artifact does not contain an html document root"}, nil
		}
	}
	return ComparisonResult{Passed: true, Details: "HTML documents are present"}, nil
}

type methrixComparator struct{}

func (methrixComparator) Compare(input ComparisonInput) (ComparisonResult, error) {
	for _, path := range []string{
		resolveComparisonPath(input.LeftResultsRoot, input.LeftEntry),
		resolveComparisonPath(input.RightResultsRoot, input.RightEntry),
	} {
		if err := validateHDF5Signature(path); err != nil {
			return ComparisonResult{}, err
		}
	}
	if input.LeftEntry.Checksum != input.RightEntry.Checksum {
		return ComparisonResult{Details: "Methrix HDF5 files differ; v1 requires byte-identical validated outputs"}, nil
	}
	return ComparisonResult{Passed: true, Details: "Methrix HDF5 signatures and checksums match"}, nil
}

func validateHDF5Signature(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open HDF5 artifact %q: %w", path, err)
	}
	defer file.Close()
	magic := make([]byte, 8)
	if _, err := io.ReadFull(file, magic); err != nil {
		return fmt.Errorf("read HDF5 signature %q: %w", path, err)
	}
	if !bytes.Equal(magic, []byte{'\x89', 'H', 'D', 'F', '\r', '\n', '\x1a', '\n'}) {
		return fmt.Errorf("artifact %q does not have an HDF5 signature", path)
	}
	return nil
}

type tsvTable struct {
	header []string
	rows   [][]string
}

func readTSV(path string) (tsvTable, error) {
	file, err := os.Open(path)
	if err != nil {
		return tsvTable{}, fmt.Errorf("open TSV artifact %q: %w", path, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return tsvTable{}, fmt.Errorf("read TSV header %q: %w", path, err)
		}
		return tsvTable{}, fmt.Errorf("TSV artifact %q is empty", path)
	}
	table := tsvTable{header: strings.Split(scanner.Text(), "\t")}
	if len(table.header) == 0 || table.header[0] == "" {
		return tsvTable{}, fmt.Errorf("TSV artifact %q has an invalid header", path)
	}
	for scanner.Scan() {
		table.rows = append(table.rows, strings.Split(scanner.Text(), "\t"))
	}
	if err := scanner.Err(); err != nil {
		return tsvTable{}, fmt.Errorf("read TSV artifact %q: %w", path, err)
	}
	return table, nil
}

func equalStringSlices(leftValues []string, rightValues []string) bool {
	if len(leftValues) != len(rightValues) {
		return false
	}
	for index := range leftValues {
		if leftValues[index] != rightValues[index] {
			return false
		}
	}
	return true
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
