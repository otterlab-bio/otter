package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otterlab-bio/otter/internal/logger"
)

func init() {
	// Initialize logger for tests
	logger.Init(false)
}

func TestNewScanner(t *testing.T) {
	// Test with nil options
	options := &ScanOptions{
		FastqDir: "/tmp/test",
	}
	scanner := NewScanner(options)

	if scanner == nil {
		t.Fatal("NewScanner returned nil")
	}

	if scanner.options.FastqDir != "/tmp/test" {
		t.Errorf("Expected FastqDir to be '/tmp/test', got '%s'", scanner.options.FastqDir)
	}

	// Test default extensions are set
	if len(scanner.options.Extensions) == 0 {
		t.Error("Default extensions not set")
	}

	// Test default suffix1 is set
	if scanner.options.Suffix1 == "" {
		t.Error("Default suffix1 not set")
	}

	// Test default suffix2 is empty (will be auto-derived)
	if scanner.options.Suffix2 != "" {
		t.Errorf("Expected suffix2 to be empty, got '%s'", scanner.options.Suffix2)
	}

	// Test with custom options
	customOptions := &ScanOptions{
		FastqDir:   "/custom/path",
		Suffix1:    "_R1.fq.gz",
		Suffix2:    "_R2.fq.gz",
		Extensions: []string{".fq.gz"},
	}
	customScanner := NewScanner(customOptions)

	if customScanner.options.FastqDir != "/custom/path" {
		t.Errorf("Expected FastqDir to be '/custom/path', got '%s'", customScanner.options.FastqDir)
	}

	if customScanner.options.Suffix1 != "_R1.fq.gz" {
		t.Errorf("Expected Suffix1 to be '_R1.fq.gz', got '%s'", customScanner.options.Suffix1)
	}

	if customScanner.options.Suffix2 != "_R2.fq.gz" {
		t.Errorf("Expected Suffix2 to be '_R2.fq.gz', got '%s'", customScanner.options.Suffix2)
	}
}

func TestScanner_isFastqFile(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir:   "/tmp",
		Extensions: []string{".fastq.gz", ".fastq", ".fq.gz", ".fq"},
	})

	tests := []struct {
		fileName string
		expected bool
	}{
		// Valid FASTQ files
		{"sample1_R1.fastq.gz", true},
		{"sample1_R2.fastq.gz", true},
		{"sample1.fastq", true},
		{"sample1.fq.gz", true},
		{"sample1.fq", true},
		{"Sample1_R1.FASTQ.GZ", true}, // Case insensitive

		// Invalid files
		{"sample1.txt", false},
		{"sample1.bam", false},
		{"sample1", false},
		{"sample1_R1.fastq.gz.bak", false},
	}

	for _, test := range tests {
		result := scanner.isFastqFile(test.fileName)
		if result != test.expected {
			t.Errorf("isFastqFile(%s) = %v, expected %v", test.fileName, result, test.expected)
		}
	}
}

func TestScanner_Scan_DirectoryNotFound(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir: "/nonexistent/directory/12345",
	})

	samples, err := scanner.Scan()

	if err == nil {
		t.Error("Expected error for nonexistent directory, got nil")
	}

	if samples != nil {
		t.Errorf("Expected samples to be nil, got %v", samples)
	}
}

func TestScanner_Scan_Success(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create test FASTQ files
	testFiles := []string{
		"sample1_R1.fastq.gz",
		"sample1_R2.fastq.gz",
		"sample2_R1.fastq.gz",
		"sample2_R2.fastq.gz",
		"sample3_R1.fastq.gz",
		"sample3_R2.fastq.gz",
	}

	for _, file := range testFiles {
		filePath := filepath.Join(tmpDir, file)
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Create non-FASTQ file (should be ignored)
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create subdirectory (should be ignored)
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "file.fastq.gz"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	scanner := NewScanner(&ScanOptions{
		FastqDir: tmpDir,
	})

	samples, err := scanner.Scan()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(samples) != 3 {
		t.Errorf("Expected 3 samples, got %d", len(samples))
	}

	// Verify sample names
	expectedSamples := map[string]bool{
		"sample1": false,
		"sample2": false,
		"sample3": false,
	}

	for _, sample := range samples {
		if _, ok := expectedSamples[sample.Name]; ok {
			expectedSamples[sample.Name] = true
		} else {
			t.Errorf("Unexpected sample name: %s", sample.Name)
		}
	}

	// Check all samples were found
	for name, found := range expectedSamples {
		if !found {
			t.Errorf("Expected sample not found: %s", name)
		}
	}
}

func TestScanner_Scan_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	scanner := NewScanner(&ScanOptions{
		FastqDir: tmpDir,
	})

	samples, err := scanner.Scan()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(samples) != 0 {
		t.Errorf("Expected 0 samples, got %d", len(samples))
	}
}

func TestScanner_groupFiles(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir: "/tmp",
		Suffix1:  "_R1.fastq.gz",
		Suffix2:  "_R2.fastq.gz",
	})

	files := []string{
		"/tmp/sample1_R1.fastq.gz",
		"/tmp/sample1_R2.fastq.gz",
		"/tmp/sample2_R1.fastq.gz",
		"/tmp/sample2_R2.fastq.gz",
	}

	samples, err := scanner.groupFiles(files)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(samples) != 2 {
		t.Errorf("Expected 2 samples, got %d", len(samples))
	}

	// Check sample1
	sample1 := findSample(samples, "sample1")
	if sample1 == nil {
		t.Fatal("sample1 not found")
	}
	if sample1.FastqR1 == "" {
		t.Error("sample1 FastqR1 is empty")
	}
	if sample1.FastqR2 == "" {
		t.Error("sample1 FastqR2 is empty")
	}

	// Check sample2
	sample2 := findSample(samples, "sample2")
	if sample2 == nil {
		t.Fatal("sample2 not found")
	}
	if sample2.FastqR1 == "" {
		t.Error("sample2 FastqR1 is empty")
	}
	if sample2.FastqR2 == "" {
		t.Error("sample2 FastqR2 is empty")
	}
}

func TestScanner_groupFiles_AutoDeriveSuffix(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir: "/tmp",
		Suffix1:  "_R1.fastq.gz",
		// Suffix2 is empty, should be auto-derived
	})

	files := []string{
		"/tmp/sample1_R1.fastq.gz",
		"/tmp/sample1_R2.fastq.gz",
	}

	samples, err := scanner.groupFiles(files)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(samples) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(samples))
	}

	if samples[0].FastqR1 == "" {
		t.Error("FastqR1 is empty")
	}
	if samples[0].FastqR2 == "" {
		t.Error("FastqR2 is empty")
	}
}

func TestScanner_groupFiles_AlternativeNaming(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir: "/tmp",
		Suffix1:  "_1.fastq.gz",
		Suffix2:  "_2.fastq.gz",
	})

	files := []string{
		"/tmp/sample1_1.fastq.gz",
		"/tmp/sample1_2.fastq.gz",
	}

	samples, err := scanner.groupFiles(files)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(samples) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(samples))
	}

	if samples[0].Name != "sample1" {
		t.Errorf("Expected sample name 'sample1', got '%s'", samples[0].Name)
	}
}

func TestScanner_groupFiles_SingleEnd(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir: "/tmp",
		Suffix1:  "_R1.fastq.gz",
		Suffix2:  "_R2.fastq.gz",
	})

	files := []string{
		"/tmp/sample1_R1.fastq.gz",
		// No R2 file for sample1
		"/tmp/sample2_R1.fastq.gz",
		"/tmp/sample2_R2.fastq.gz",
	}

	samples, err := scanner.groupFiles(files)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(samples) != 2 {
		t.Errorf("Expected 2 samples, got %d", len(samples))
	}

	// sample1 should have only R1
	sample1 := findSample(samples, "sample1")
	if sample1 == nil {
		t.Fatal("sample1 not found")
	}
	if sample1.FastqR1 == "" {
		t.Error("sample1 FastqR1 is empty")
	}
	if sample1.FastqR2 != "" {
		t.Error("sample1 should not have FastqR2")
	}
}

func TestScanner_PairSamples(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir: "/tmp",
	})

	samples := []Sample{
		{Name: "sample1", FastqR1: "/tmp/sample1_R1.fastq.gz", FastqR2: "/tmp/sample1_R2.fastq.gz"},
		{Name: "sample2", FastqR1: "/tmp/sample2_R1.fastq.gz", FastqR2: "/tmp/sample2_R2.fastq.gz"},
	}

	paired, err := scanner.PairSamples(samples, nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(paired) != 2 {
		t.Errorf("Expected 2 paired samples, got %d", len(paired))
	}

	for _, ps := range paired {
		if !ps.Valid {
			t.Errorf("Sample %s should be valid", ps.Name)
		}
		if ps.R1Path == "" {
			t.Errorf("Sample %s R1Path is empty", ps.Name)
		}
		if ps.R2Path == "" {
			t.Errorf("Sample %s R2Path is empty", ps.Name)
		}
	}
}

func TestScanner_PairSamples_SingleEndAllowed(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir: "/tmp",
	})

	samples := []Sample{
		{Name: "sample1", FastqR1: "/tmp/sample1_R1.fastq.gz", FastqR2: ""},
	}

	options := &PairOptions{
		StrictMatching: false,
		AllowSingleEnd: true,
		MinFiles:       1,
	}

	paired, err := scanner.PairSamples(samples, options)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(paired) != 1 {
		t.Errorf("Expected 1 paired sample, got %d", len(paired))
	}

	if !paired[0].Valid {
		t.Error("Sample should be valid with AllowSingleEnd=true")
	}
}

func TestScanner_PairSamples_StrictMatching(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir: "/tmp",
	})

	samples := []Sample{
		{Name: "sample1", FastqR1: "/tmp/sample1_R1.fastq.gz", FastqR2: ""},
	}

	options := &PairOptions{
		StrictMatching: true,
		AllowSingleEnd: false,
		MinFiles:       1,
	}

	paired, err := scanner.PairSamples(samples, options)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(paired) != 0 {
		t.Errorf("Expected 0 paired samples with strict matching, got %d", len(paired))
	}
}

func TestDeriveSuffix2(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"_R1.fastq.gz", "_R2.fastq.gz"},
		{"_1.fastq.gz", "_2.fastq.gz"},
		{"_R1.fq", "_R2.fq"},
		{"_R1", "_R2"},
		{"_1", "_2"},
		{"sample_R1.fastq.gz", "sample_R2.fastq.gz"},
		{"sample_1.fastq.gz", "sample_2.fastq.gz"},
	}

	for _, test := range tests {
		result := deriveSuffix2(test.input)
		if result != test.expected {
			t.Errorf("deriveSuffix2(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestScanner_groupFiles_RejectsAmbiguousInput(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir: "/tmp",
		Suffix1:  "_R1.fastq.gz",
		Suffix2:  "_R2.fastq.gz",
	})

	_, err := scanner.groupFiles([]string{
		"/tmp/sample_R1.fastq.gz",
		"/other/sample_R1.fastq.gz",
	})
	if err == nil || !strings.Contains(err.Error(), "multiple R1 files") {
		t.Fatalf("expected duplicate R1 error, got %v", err)
	}

	_, err = scanner.groupFiles([]string{"/tmp/sample.fastq.gz"})
	if err == nil || !strings.Contains(err.Error(), "matches neither") {
		t.Fatalf("expected unrecognized suffix error, got %v", err)
	}
}

func TestScanner_groupFiles_RejectsIdenticalMateSuffixes(t *testing.T) {
	scanner := NewScanner(&ScanOptions{
		FastqDir: "/tmp",
		Suffix1:  ".fastq.gz",
		Suffix2:  ".fastq.gz",
	})

	_, err := scanner.groupFiles([]string{"/tmp/sample.fastq.gz"})
	if err == nil || !strings.Contains(err.Error(), "must be distinct") {
		t.Fatalf("expected distinct suffix error, got %v", err)
	}
}

// Helper function to find a sample by name
func findSample(samples []Sample, name string) *Sample {
	for i := range samples {
		if samples[i].Name == name {
			return &samples[i]
		}
	}
	return nil
}
