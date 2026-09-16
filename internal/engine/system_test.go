package engine

import (
	"os"
	"testing"
)

func TestParseMemory(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		hasError bool
	}{
		{"GB format", "100G", 102400, false},
		{"GB format with GB", "100GB", 102400, false},
		{"Gibibyte format", "8GiB", 8192, false},
		{"MB format", "8000M", 8000, false},
		{"MB format with MB", "8000MB", 8000, false},
		{"Empty string", "", 0, false},
		{"Invalid format", "invalid", 0, true},
		{"Mixed case", "50g", 51200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseMemory(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("ParseMemory(%s) expected error but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("ParseMemory(%s) unexpected error: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("ParseMemory(%s) = %d, expected %d", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestFormatSlurmMemoryUsesMebibyteDirectives(t *testing.T) {
	formattedMemory, err := formatSlurmMemory("16GiB")
	if err != nil {
		t.Fatal(err)
	}
	if formattedMemory != "16384M" {
		t.Fatalf("unexpected Slurm memory directive: %q", formattedMemory)
	}
}

func TestGetSystemInfo(t *testing.T) {
	info, err := GetSystemInfo()
	if err != nil {
		t.Fatalf("GetSystemInfo() failed: %v", err)
	}

	// 验证字段不为零
	if info.TotalCores <= 0 {
		t.Errorf("TotalCores should be > 0, got %d", info.TotalCores)
	}

	if info.TotalMem <= 0 {
		t.Errorf("TotalMem should be > 0, got %d", info.TotalMem)
	}

	if info.AvailableMem <= 0 {
		t.Errorf("AvailableMem should be > 0, got %d", info.AvailableMem)
	}

	// 验证可用内存不超过总内存
	if info.AvailableMem > info.TotalMem {
		t.Errorf("AvailableMem (%d) should not exceed TotalMem (%d)", info.AvailableMem, info.TotalMem)
	}

	t.Logf("System Info: %+v", info)
}

func TestValidateLocalResources(t *testing.T) {
	info, err := GetSystemInfo()
	if err != nil {
		t.Fatalf("GetSystemInfo() failed: %v", err)
	}

	// Test with invalid cores
	err = ValidateLocalResources(info.TotalCores+10, "1G")
	if err == nil {
		t.Errorf("ValidateLocalResources() expected error for too many cores")
	}

	// Test with valid resources
	err = ValidateLocalResources(info.TotalCores/2, "1G")
	if err != nil {
		t.Errorf("ValidateLocalResources() unexpected error: %v", err)
	}
}

func TestValidateParallelJobs(t *testing.T) {
	info, err := GetSystemInfo()
	if err != nil {
		t.Fatalf("GetSystemInfo() failed: %v", err)
	}

	tests := []struct {
		name     string
		jobs     int
		expected bool
	}{
		{"Valid small number", 2, true},
		{"Valid within limits", info.TotalCores / 2, true},
		{"Invalid too many", info.TotalCores + 10, false},
		{"Invalid zero", 0, false},
		{"Valid maximum recommended", int(float64(info.TotalCores) * 0.75), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParallelJobs(tt.jobs)
			if tt.expected && err != nil {
				t.Errorf("ValidateParallelJobs() expected no error but got: %v", err)
			}
			if !tt.expected && err == nil {
				t.Errorf("ValidateParallelJobs() expected error but got none")
			}
		})
	}
}

// TestValidateSlurmNodeResources tests SLURM node resource validation
func TestValidateSlurmNodeResources(t *testing.T) {
	// 这个测试需要实际的SLURM环境
	// 在没有SLURM的情况下，我们跳过测试
	if os.Getenv("SKIP_SLURM_TESTS") == "1" {
		t.Skip("Skipping SLURM tests")
	}

	// 测试非存在的分区（应该失败）
	err := ValidateSlurmNodeResources("non_existent_partition", 1, "1G")
	if err == nil {
		t.Errorf("ValidateSlurmNodeResources() expected error for non-existent partition")
	} else {
		t.Logf("Expected error for non-existent partition: %v", err)
	}
}

// Benchmark tests for performance
func BenchmarkParseMemory(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseMemory("100G")
	}
}

func BenchmarkGetSystemInfo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetSystemInfo()
	}
}

func BenchmarkValidateLocalResources(b *testing.B) {
	info, _ := GetSystemInfo()
	for i := 0; i < b.N; i++ {
		ValidateLocalResources(info.TotalCores/2, "10G")
	}
}
