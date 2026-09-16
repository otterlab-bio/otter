package input

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otterlab-bio/otter/internal/logger"
)

func init() {
	logger.Init(false)
}

func TestNewValidator(t *testing.T) {
	validator := NewValidator()

	if validator == nil {
		t.Fatal("NewValidator returned nil")
	}
}

func TestValidator_ValidateInput_EmptySamples(t *testing.T) {
	validator := NewValidator()

	result := validator.ValidateInput("", "", nil, nil)

	if result == nil {
		t.Fatal("ValidateInput returned nil")
	}

	if result.Valid {
		t.Error("Expected validation to fail with no samples")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors for no samples")
	}
}

func TestValidator_ValidateInput_ValidSamples(t *testing.T) {
	// Create temporary directory and files
	tmpDir := t.TempDir()

	// Create test FASTQ files
	r1File := filepath.Join(tmpDir, "sample1_R1.fastq.gz")
	r2File := filepath.Join(tmpDir, "sample1_R2.fastq.gz")

	if err := os.WriteFile(r1File, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create R1 file: %v", err)
	}
	if err := os.WriteFile(r2File, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create R2 file: %v", err)
	}

	validator := NewValidator()

	pairedSamples := []PairedSample{
		{Name: "sample1", R1Path: r1File, R2Path: r2File, Valid: true},
	}

	result := validator.ValidateInput(tmpDir, "", pairedSamples, nil)

	if result == nil {
		t.Fatal("ValidateInput returned nil")
	}

	if !result.Valid {
		t.Errorf("Expected validation to pass: %v", result.Errors)
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
}

func TestValidator_ValidateInput_InvalidR1File(t *testing.T) {
	validator := NewValidator()

	// R1 file doesn't exist
	pairedSamples := []PairedSample{
		{Name: "sample1", R1Path: "/nonexistent/sample1_R1.fastq.gz", R2Path: "", Valid: false},
	}

	result := validator.ValidateInput("", "", pairedSamples, nil)

	if result == nil {
		t.Fatal("ValidateInput returned nil")
	}

	if result.Valid {
		t.Error("Expected validation to fail with missing R1 file")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors for missing R1 file")
	}
}

func TestValidator_ValidateInput_InvalidR2File(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only R1 file
	r1File := filepath.Join(tmpDir, "sample1_R1.fastq.gz")
	if err := os.WriteFile(r1File, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create R1 file: %v", err)
	}

	validator := NewValidator()

	pairedSamples := []PairedSample{
		{Name: "sample1", R1Path: r1File, R2Path: "/nonexistent/sample1_R2.fastq.gz", Valid: true},
	}

	result := validator.ValidateInput(tmpDir, "", pairedSamples, nil)

	if result == nil {
		t.Fatal("ValidateInput returned nil")
	}

	if result.Valid {
		t.Error("Expected validation to fail with missing R2 file")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors for missing R2 file")
	}
}

func TestValidator_ValidateInput_PDataMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test FASTQ files
	r1File := filepath.Join(tmpDir, "sample1_R1.fastq.gz")
	r2File := filepath.Join(tmpDir, "sample1_R2.fastq.gz")

	if err := os.WriteFile(r1File, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create R1 file: %v", err)
	}
	if err := os.WriteFile(r2File, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create R2 file: %v", err)
	}

	validator := NewValidator()

	pairedSamples := []PairedSample{
		{Name: "sample1", R1Path: r1File, R2Path: r2File, Valid: true},
	}

	// pdata contains different samples
	pdata := &PData{
		Samples: []string{"sample2", "sample3"},
		Data: map[string]map[string]string{
			"sample2": {"sampleid": "sample2"},
			"sample3": {"sampleid": "sample3"},
		},
	}

	result := validator.ValidateInput(tmpDir, "", pairedSamples, pdata)

	if result == nil {
		t.Fatal("ValidateInput returned nil")
	}

	if result.Valid {
		t.Error("Expected validation to fail with pdata mismatch")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors for pdata mismatch")
	}
}

func TestValidator_ValidateInput_PDataMatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test FASTQ files
	r1File1 := filepath.Join(tmpDir, "sample1_R1.fastq.gz")
	r2File1 := filepath.Join(tmpDir, "sample1_R2.fastq.gz")
	r1File2 := filepath.Join(tmpDir, "sample2_R1.fastq.gz")
	r2File2 := filepath.Join(tmpDir, "sample2_R2.fastq.gz")

	if err := os.WriteFile(r1File1, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create R1 file: %v", err)
	}
	if err := os.WriteFile(r2File1, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create R2 file: %v", err)
	}
	if err := os.WriteFile(r1File2, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create R1 file: %v", err)
	}
	if err := os.WriteFile(r2File2, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create R2 file: %v", err)
	}

	validator := NewValidator()

	pairedSamples := []PairedSample{
		{Name: "sample1", R1Path: r1File1, R2Path: r2File1, Valid: true},
		{Name: "sample2", R1Path: r1File2, R2Path: r2File2, Valid: true},
	}

	// pdata matches FASTQ samples
	pdata := &PData{
		Samples: []string{"sample1", "sample2"},
		Data: map[string]map[string]string{
			"sample1": {"sampleid": "sample1"},
			"sample2": {"sampleid": "sample2"},
		},
	}

	result := validator.ValidateInput(tmpDir, "", pairedSamples, pdata)

	if result == nil {
		t.Fatal("ValidateInput returned nil")
	}

	if !result.Valid {
		t.Errorf("Expected validation to pass: %v", result.Errors)
	}
}

func TestValidator_validateFastqDir(t *testing.T) {
	validator := NewValidator()

	// Test nonexistent directory
	err := validator.validateFastqDir("/nonexistent/directory")
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}

	// Test valid directory
	tmpDir := t.TempDir()
	err = validator.validateFastqDir(tmpDir)
	if err != nil {
		t.Errorf("Expected no error for valid directory: %v", err)
	}
}

func TestValidator_validatePdataFile(t *testing.T) {
	validator := NewValidator()

	// Test nonexistent file
	err := validator.validatePdataFile("/nonexistent/file.csv")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}

	// Test CSV file
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "data.csv")
	if err := os.WriteFile(csvFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create CSV file: %v", err)
	}

	err = validator.validatePdataFile(csvFile)
	if err != nil {
		t.Errorf("Expected no error for CSV file: %v", err)
	}

	// Test Excel file
	xlsxFile := filepath.Join(tmpDir, "data.xlsx")
	if err := os.WriteFile(xlsxFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create XLSX file: %v", err)
	}

	err = validator.validatePdataFile(xlsxFile)
	if err != nil {
		t.Errorf("Expected no error for XLSX file: %v", err)
	}

	// Test invalid extension
	invalidFile := filepath.Join(tmpDir, "data.txt")
	if err := os.WriteFile(invalidFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create TXT file: %v", err)
	}

	err = validator.validatePdataFile(invalidFile)
	if err == nil {
		t.Error("Expected error for invalid file extension")
	}
}

func TestValidator_validatePdataSamples(t *testing.T) {
	validator := NewValidator()

	// Test matching samples
	pairedSamples := []PairedSample{
		{Name: "sample1", R1Path: "", R2Path: "", Valid: true},
		{Name: "sample2", R1Path: "", R2Path: "", Valid: true},
	}

	pdata := &PData{
		Samples: []string{"sample1", "sample2"},
		Data: map[string]map[string]string{
			"sample1": {"sampleid": "sample1"},
			"sample2": {"sampleid": "sample2"},
		},
	}

	errors := validator.validatePdataSamples(pairedSamples, pdata)
	if len(errors) != 0 {
		t.Errorf("Expected no errors for matching samples: %v", errors)
	}

	// Test mismatched samples
	pairedSamples = []PairedSample{
		{Name: "sample1", R1Path: "", R2Path: "", Valid: true},
	}

	pdata = &PData{
		Samples: []string{"sample2"},
		Data: map[string]map[string]string{
			"sample2": {"sampleid": "sample2"},
		},
	}

	errors = validator.validatePdataSamples(pairedSamples, pdata)
	if len(errors) == 0 {
		t.Error("Expected errors for mismatched samples")
	}
}

func TestGetSampleNames(t *testing.T) {
	pairedSamples := []PairedSample{
		{Name: "sample1", R1Path: "", R2Path: "", Valid: true},
		{Name: "sample2", R1Path: "", R2Path: "", Valid: true},
		{Name: "sample3", R1Path: "", R2Path: "", Valid: true},
	}

	names := GetSampleNames(pairedSamples)

	if len(names) != 3 {
		t.Errorf("Expected 3 sample names, got %d", len(names))
	}

	expectedNames := []string{"sample1", "sample2", "sample3"}
	for i, expectedName := range expectedNames {
		if i >= len(names) {
			t.Errorf("Missing sample name at index %d", i)
		} else if names[i] != expectedName {
			t.Errorf("Expected sample name '%s' at index %d, got '%s'", expectedName, i, names[i])
		}
	}
}
