package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otterlab-bio/otter/internal/logger"
)

func init() {
	logger.Init(false)
}

func TestNewPDataParser(t *testing.T) {
	parser := NewPDataParser()

	if parser == nil {
		t.Fatal("NewPDataParser returned nil")
	}
}

func TestPDataParser_Load_FileNotFound(t *testing.T) {
	parser := NewPDataParser()

	pdata, err := parser.Load("/nonexistent/file.csv")

	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}

	if pdata != nil {
		t.Errorf("Expected pdata to be nil, got %v", pdata)
	}
}

func TestPDataParser_Load_UnsupportedFormat(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create file with unsupported extension
	unsupportedFile := filepath.Join(tmpDir, "data.txt")
	if err := os.WriteFile(unsupportedFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	parser := NewPDataParser()

	pdata, err := parser.Load(unsupportedFile)

	if err == nil {
		t.Error("Expected error for unsupported format, got nil")
	}

	if pdata != nil {
		t.Errorf("Expected pdata to be nil, got %v", pdata)
	}

	// Check error message
	if !strings.Contains(err.Error(), "unsupported file format") {
		t.Errorf("Expected error message to contain 'unsupported file format', got: %v", err)
	}
}

func TestPDataParser_Load_ExcelNotSupported(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create Excel file
	excelFile := filepath.Join(tmpDir, "data.xlsx")
	if err := os.WriteFile(excelFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create Excel file: %v", err)
	}

	parser := NewPDataParser()

	pdata, err := parser.Load(excelFile)

	if err == nil {
		t.Error("Expected error for Excel file, got nil")
	}

	if pdata != nil {
		t.Errorf("Expected pdata to be nil, got %v", pdata)
	}

	// Check error message
	// Excel is now supported, so the error should be about invalid zip/format
	if !strings.Contains(err.Error(), "zip") && !strings.Contains(err.Error(), "valid") {
		t.Errorf("Expected error about invalid zip/format, got: %v", err)
	}
}

func TestPDataParser_LoadCSV_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty CSV file
	csvFile := filepath.Join(tmpDir, "empty.csv")
	if err := os.WriteFile(csvFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty CSV file: %v", err)
	}

	parser := NewPDataParser()

	pdata, err := parser.Load(csvFile)

	if err == nil {
		t.Error("Expected error for empty CSV file, got nil")
	}

	if pdata != nil {
		t.Errorf("Expected pdata to be nil, got %v", pdata)
	}
}

func TestPDataParser_LoadCSV_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid CSV file
	csvContent := `sampleid,inline_barcode_sequence,condition
sample1,ATCG,control
sample2,GCTA,treatment
sample3,TAGC,control
sample4,CGAT,treatment`

	csvFile := filepath.Join(tmpDir, "data.csv")
	if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
		t.Fatalf("Failed to create CSV file: %v", err)
	}

	parser := NewPDataParser()

	pdata, err := parser.Load(csvFile)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pdata == nil {
		t.Fatal("Expected pdata to be non-nil")
	}

	if len(pdata.Samples) != 4 {
		t.Errorf("Expected 4 samples, got %d", len(pdata.Samples))
	}

	// Check samples
	expectedSamples := []string{"sample1", "sample2", "sample3", "sample4"}
	for i, expected := range expectedSamples {
		if i >= len(pdata.Samples) {
			t.Errorf("Missing sample at index %d", i)
		} else if pdata.Samples[i] != expected {
			t.Errorf("Expected sample '%s' at index %d, got '%s'", expected, i, pdata.Samples[i])
		}
	}

	// Check data
	if len(pdata.Data) != 4 {
		t.Errorf("Expected 4 samples in data map, got %d", len(pdata.Data))
	}

	// Check sample1 data
	sample1Data, ok := pdata.Data["sample1"]
	if !ok {
		t.Error("sample1 not found in data")
	} else {
		if sample1Data["sampleid"] != "sample1" {
			t.Errorf("Expected sample1 'sampleid' to be 'sample1', got '%s'", sample1Data["sampleid"])
		}
		if sample1Data["inline_barcode_sequence"] != "ATCG" {
			t.Errorf("Expected sample1 'inline_barcode_sequence' to be 'ATCG', got '%s'", sample1Data["inline_barcode_sequence"])
		}
		if sample1Data["condition"] != "control" {
			t.Errorf("Expected sample1 'condition' to be 'control', got '%s'", sample1Data["condition"])
		}
	}
}

func TestPDataParser_LoadCSV_ChineseColumns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CSV file with Chinese headers
	csvContent := `样本编号,inline_barcode_sequence,条件
样本1,ATCG,对照组
样本2,GCTA,治疗组
样本3,TAGC,对照组`

	csvFile := filepath.Join(tmpDir, "data_chinese.csv")
	if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
		t.Fatalf("Failed to create CSV file: %v", err)
	}

	parser := NewPDataParser()

	pdata, err := parser.Load(csvFile)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pdata == nil {
		t.Fatal("Expected pdata to be non-nil")
	}

	if len(pdata.Samples) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(pdata.Samples))
	}

	// Check that both original and normalized column names are stored
	sample1Data, ok := pdata.Data["样本1"]
	if !ok {
		t.Error("样本1 not found in data")
	} else {
		// Should have both "样本编号" and "sampleid"
		if sample1Data["样本编号"] != "样本1" {
			t.Errorf("Expected '样本编号' to be '样本1', got '%s'", sample1Data["样本编号"])
		}
		if sample1Data["sampleid"] != "样本1" {
			t.Errorf("Expected 'sampleid' to be '样本1', got '%s'", sample1Data["sampleid"])
		}

		// Should have both "条件" and "condition"
		if sample1Data["条件"] != "对照组" {
			t.Errorf("Expected '条件' to be '对照组', got '%s'", sample1Data["条件"])
		}
		if sample1Data["condition"] != "对照组" {
			t.Errorf("Expected 'condition' to be '对照组', got '%s'", sample1Data["condition"])
		}
	}
}

func TestPDataParser_LoadCSV_NoSampleID(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CSV file without sampleid column
	csvContent := `barcode,condition
ATCG,control
GCTA,treatment`

	csvFile := filepath.Join(tmpDir, "data_no_sampleid.csv")
	if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
		t.Fatalf("Failed to create CSV file: %v", err)
	}

	parser := NewPDataParser()

	pdata, err := parser.Load(csvFile)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pdata == nil {
		t.Fatal("Expected pdata to be non-nil")
	}

	if len(pdata.Samples) != 2 {
		t.Errorf("Expected 2 samples, got %d", len(pdata.Samples))
	}

	// Should generate sample IDs automatically
	if pdata.Samples[0] != "Sample1" {
		t.Errorf("Expected first sample ID to be 'Sample1', got '%s'", pdata.Samples[0])
	}
	if pdata.Samples[1] != "Sample2" {
		t.Errorf("Expected second sample ID to be 'Sample2', got '%s'", pdata.Samples[1])
	}
}

func TestPDataParser_Validate_Valid(t *testing.T) {
	parser := NewPDataParser()

	pdata := &PData{
		Samples: []string{"sample1", "sample2", "sample3"},
		Data: map[string]map[string]string{
			"sample1": {"sampleid": "sample1"},
			"sample2": {"sampleid": "sample2"},
			"sample3": {"sampleid": "sample3"},
		},
	}

	errors := parser.Validate(pdata, []string{"sample1", "sample2", "sample3"})

	if len(errors) != 0 {
		t.Errorf("Expected no errors, got: %v", errors)
	}
}

func TestPDataParser_Validate_MissingSamples(t *testing.T) {
	parser := NewPDataParser()

	pdata := &PData{
		Samples: []string{"sample1", "sample2"},
		Data: map[string]map[string]string{
			"sample1": {"sampleid": "sample1"},
			"sample2": {"sampleid": "sample2"},
		},
	}

	// Request validation for a sample not in pdata
	errors := parser.Validate(pdata, []string{"sample1", "sample2", "sample3"})

	if len(errors) == 0 {
		t.Error("Expected errors for missing samples")
	}

	// Check error message
	found := false
	for _, err := range errors {
		if strings.Contains(err, "sample3") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error mentioning 'sample3', got: %v", errors)
	}
}

func TestPDataParser_Merge(t *testing.T) {
	parser := NewPDataParser()

	samples := []Sample{
		{Name: "sample1"},
		{Name: "sample2"},
		{Name: "sample3"},
	}

	pdata := &PData{
		Data: map[string]map[string]string{
			"sample1": {"sampleid": "sample1", "condition": "control"},
			"sample2": {"sampleid": "sample2", "condition": "treatment"},
			// sample3 not in pdata
		},
	}

	result := parser.Merge(samples, pdata)

	if len(result) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(result))
	}

	// Check sample1 has metadata
	if result[0].Metadata == nil {
		t.Error("sample1 metadata is nil")
	} else {
		if result[0].Metadata["condition"] != "control" {
			t.Errorf("Expected sample1 condition to be 'control', got '%s'", result[0].Metadata["condition"])
		}
	}

	// Check sample2 has metadata
	if result[1].Metadata == nil {
		t.Error("sample2 metadata is nil")
	} else {
		if result[1].Metadata["condition"] != "treatment" {
			t.Errorf("Expected sample2 condition to be 'treatment', got '%s'", result[1].Metadata["condition"])
		}
	}

	// Check sample3 doesn't have pdata metadata
	if result[2].Metadata != nil {
		// Should be empty or not have pdata keys
		if _, ok := result[2].Metadata["condition"]; ok {
			t.Error("sample3 should not have condition metadata")
		}
	}
}

func TestPDataParser_LoadCSV_MultipleColumns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CSV file with many columns
	csvContent := `sampleid,col1,col2,col3,col4
sample1,val1,val2,val3,val4
sample2,val1,val2,val3,val4
sample3,val1,val2,val3,val4`

	csvFile := filepath.Join(tmpDir, "data_multi_col.csv")
	if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
		t.Fatalf("Failed to create CSV file: %v", err)
	}

	parser := NewPDataParser()

	pdata, err := parser.Load(csvFile)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(pdata.Columns) != 5 {
		t.Errorf("Expected 5 columns, got %d", len(pdata.Columns))
	}

	if len(pdata.Data["sample1"]) != 5 {
		t.Errorf("Expected 5 fields in sample1 data, got %d", len(pdata.Data["sample1"]))
	}
}

func TestPDataParser_LoadCSV_RejectsEmptyAndDuplicateSampleIDs(t *testing.T) {
	parser := NewPDataParser()

	for testName, csvContent := range map[string]string{
		"empty":     "sampleid,condition\n,control\n",
		"duplicate": "sampleid,condition\nsample1,control\nsample1,treatment\n",
	} {
		t.Run(testName, func(t *testing.T) {
			csvFile := filepath.Join(t.TempDir(), "pdata.csv")
			if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := parser.Load(csvFile); err == nil {
				t.Fatalf("expected invalid sample ID error")
			}
		})
	}
}

func TestPDataParser_LoadCSV_EmptyValues(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CSV file with empty values
	csvContent := `sampleid,inline_barcode_sequence,condition
sample1,,control
sample2,GCTA,
sample3,,
`

	csvFile := filepath.Join(tmpDir, "data_empty_vals.csv")
	if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
		t.Fatalf("Failed to create CSV file: %v", err)
	}

	parser := NewPDataParser()

	pdata, err := parser.Load(csvFile)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(pdata.Samples) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(pdata.Samples))
	}

	// Check empty values are preserved
	sample1Data := pdata.Data["sample1"]
	if sample1Data["inline_barcode_sequence"] != "" {
		t.Errorf("Expected empty barcode, got '%s'", sample1Data["inline_barcode_sequence"])
	}
	if sample1Data["condition"] != "control" {
		t.Errorf("Expected condition 'control', got '%s'", sample1Data["condition"])
	}

	sample2Data := pdata.Data["sample2"]
	if sample2Data["inline_barcode_sequence"] != "GCTA" {
		t.Errorf("Expected barcode 'GCTA', got '%s'", sample2Data["inline_barcode_sequence"])
	}
	if sample2Data["condition"] != "" {
		t.Errorf("Expected empty condition, got '%s'", sample2Data["condition"])
	}
}
