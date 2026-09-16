package samples

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

var sampleIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var canonicalColumns = []string{
	"sample_id",
	"r1",
	"r2",
	"group",
	"batch",
	"adapter_r1",
	"adapter_r2",
}

func Load(path string, projectRoot string) ([]configv1.SampleRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open samples manifest %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = false

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read samples manifest header: %w", err)
	}
	columnIndexes, err := validateHeader(header)
	if err != nil {
		return nil, err
	}

	var records []configv1.SampleRecord
	seenSampleIDs := make(map[string]bool)
	for lineNumber := 2; ; lineNumber++ {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read samples manifest line %d: %w", lineNumber, readErr)
		}
		if isEmptyRow(row) {
			continue
		}
		record, parseErr := parseRecord(row, columnIndexes, projectRoot)
		if parseErr != nil {
			return nil, fmt.Errorf("samples manifest line %d: %w", lineNumber, parseErr)
		}
		if seenSampleIDs[record.ID] {
			return nil, fmt.Errorf("samples manifest line %d: duplicate sample_id %q", lineNumber, record.ID)
		}
		seenSampleIDs[record.ID] = true
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("samples manifest contains no samples")
	}
	return records, nil
}

func Write(path string, records []configv1.SampleRecord, projectRoot string) error {
	if len(records) == 0 {
		return fmt.Errorf("cannot write an empty samples manifest")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create samples manifest directory: %w", err)
	}
	seenSampleIDs := make(map[string]bool, len(records))
	for _, record := range records {
		if !sampleIDPattern.MatchString(record.ID) {
			return fmt.Errorf("sample_id %q is invalid", record.ID)
		}
		if seenSampleIDs[record.ID] {
			return fmt.Errorf("duplicate sample_id %q", record.ID)
		}
		if strings.TrimSpace(record.R1) == "" || strings.TrimSpace(record.R2) == "" {
			return fmt.Errorf("sample %q requires paired R1 and R2 paths", record.ID)
		}
		seenSampleIDs[record.ID] = true
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create samples manifest %s: %w", path, err)
	}
	writer := csv.NewWriter(file)
	writer.Comma = '\t'
	if err := writer.Write(canonicalColumns); err != nil {
		file.Close()
		return fmt.Errorf("write samples manifest header: %w", err)
	}
	for _, record := range records {
		r1, err := projectRelativePath(projectRoot, record.R1)
		if err != nil {
			file.Close()
			return fmt.Errorf("sample %q R1: %w", record.ID, err)
		}
		r2, err := projectRelativePath(projectRoot, record.R2)
		if err != nil {
			file.Close()
			return fmt.Errorf("sample %q R2: %w", record.ID, err)
		}
		row := []string{record.ID, r1, r2, record.Group, record.Batch, record.AdapterR1, record.AdapterR2}
		if err := writer.Write(row); err != nil {
			file.Close()
			return fmt.Errorf("write sample %q: %w", record.ID, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		file.Close()
		return fmt.Errorf("flush samples manifest: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync samples manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close samples manifest: %w", err)
	}
	return nil
}

func validateHeader(header []string) (map[string]int, error) {
	indexes := make(map[string]int, len(header))
	allowed := make(map[string]bool, len(canonicalColumns))
	for _, column := range canonicalColumns {
		allowed[column] = true
	}
	for index, rawColumn := range header {
		column := strings.TrimSpace(rawColumn)
		if !allowed[column] {
			return nil, fmt.Errorf("samples manifest has unsupported column %q", column)
		}
		if _, exists := indexes[column]; exists {
			return nil, fmt.Errorf("samples manifest has duplicate column %q", column)
		}
		indexes[column] = index
	}
	for _, requiredColumn := range []string{"sample_id", "r1", "r2"} {
		if _, exists := indexes[requiredColumn]; !exists {
			return nil, fmt.Errorf("samples manifest is missing required column %q", requiredColumn)
		}
	}
	return indexes, nil
}

func parseRecord(row []string, indexes map[string]int, projectRoot string) (configv1.SampleRecord, error) {
	record := configv1.SampleRecord{
		ID:        field(row, indexes, "sample_id"),
		Group:     field(row, indexes, "group"),
		Batch:     field(row, indexes, "batch"),
		AdapterR1: field(row, indexes, "adapter_r1"),
		AdapterR2: field(row, indexes, "adapter_r2"),
	}
	if !sampleIDPattern.MatchString(record.ID) {
		return configv1.SampleRecord{}, fmt.Errorf("sample_id %q is invalid", record.ID)
	}
	var err error
	record.R1, err = resolveInputPath(projectRoot, field(row, indexes, "r1"))
	if err != nil {
		return configv1.SampleRecord{}, fmt.Errorf("R1: %w", err)
	}
	record.R2, err = resolveInputPath(projectRoot, field(row, indexes, "r2"))
	if err != nil {
		return configv1.SampleRecord{}, fmt.Errorf("R2: %w", err)
	}
	return record, nil
}

func resolveInputPath(projectRoot string, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	return filepath.Clean(absolutePath), nil
}

func projectRelativePath(projectRoot string, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	relativePath, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return "", fmt.Errorf("resolve project-relative path: %w", err)
	}
	return filepath.ToSlash(relativePath), nil
}

func field(row []string, indexes map[string]int, column string) string {
	index, exists := indexes[column]
	if !exists || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func isEmptyRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
