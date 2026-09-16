package input

import (
	"testing"
)

func TestAdapterGenerator_GenerateAdapters_NoBarcode(t *testing.T) {
	gen := NewAdapterGenerator(AdapterGeneratorOptions{
		BaseAdapter1: "AGATCGGAAGAGC",
		BaseAdapter2: "AGATCGGAAGAGC",
		Mode:         "RRBS",
	})

	samples := []string{"sample1", "sample2"}
	pdata := &PData{
		Data: map[string]map[string]string{
			"sample1": {"sampleid": "sample1", "inline_barcode_sequence": ""},
			"sample2": {"sampleid": "sample2", "inline_barcode_sequence": ""},
		},
	}

	adapter1, adapter2, err := gen.GenerateAdapters(samples, pdata)
	if err != nil {
		t.Fatalf("GenerateAdapters failed: %v", err)
	}

	// No barcode should result in NO_ADAPTER_CAL_USE_DEFAULT
	if len(adapter1) != 2 || len(adapter2) != 2 {
		t.Fatalf("Expected 2 adapters each, got adapter1=%d, adapter2=%d", len(adapter1), len(adapter2))
	}

	for i, adapter := range adapter1 {
		if adapter != "NO_ADAPTER_CAL_USE_DEFAULT" {
			t.Errorf("Sample %d: expected NO_ADAPTER_CAL_USE_DEFAULT, got %s", i, adapter)
		}
	}

	for i, adapter := range adapter2 {
		if adapter != "NO_ADAPTER_CAL_USE_DEFAULT" {
			t.Errorf("Sample %d: expected NO_ADAPTER_CAL_USE_DEFAULT, got %s", i, adapter)
		}
	}
}

func TestAdapterGenerator_GenerateAdapters_WithBarcode(t *testing.T) {
	gen := NewAdapterGenerator(AdapterGeneratorOptions{
		BaseAdapter1: "AGATCGGAAGAGC",
		BaseAdapter2: "AGATCGGAAGAGC",
		Mode:         "RRBS",
	})

	samples := []string{"sample1"}
	pdata := &PData{
		Data: map[string]map[string]string{
			"sample1": {"sampleid": "sample1", "inline_barcode_sequence": "ATCG"},
		},
	}

	adapter1, adapter2, err := gen.GenerateAdapters(samples, pdata)
	if err != nil {
		t.Fatalf("GenerateAdapters failed: %v", err)
	}

	// ATCG -> reverse complement -> CGAT
	// RRBS mode: TGA + CGAT + AGATCGGAAGAGC
	expectedAdapter1 := "TGACGATAGATCGGAAGAGC"
	expectedAdapter2 := "ACGATAGATCGGAAGAGC"

	if len(adapter1) != 1 || len(adapter2) != 1 {
		t.Fatalf("Expected 1 adapter each, got adapter1=%d, adapter2=%d", len(adapter1), len(adapter2))
	}

	if adapter1[0] != expectedAdapter1 {
		t.Errorf("Expected adapter1[0]=%s, got %s", expectedAdapter1, adapter1[0])
	}

	if adapter2[0] != expectedAdapter2 {
		t.Errorf("Expected adapter2[0]=%s, got %s", expectedAdapter2, adapter2[0])
	}
}

func TestAdapterGenerator_GenerateAdapters_WGBSMode(t *testing.T) {
	gen := NewAdapterGenerator(AdapterGeneratorOptions{
		BaseAdapter1: "AGATCGGAAGAGC",
		BaseAdapter2: "AGATCGGAAGAGC",
		Mode:         "WGBS",
	})

	samples := []string{"sample1"}
	pdata := &PData{
		Data: map[string]map[string]string{
			"sample1": {"sampleid": "sample1", "inline_barcode_sequence": "ATCG"},
		},
	}

	adapter1, adapter2, err := gen.GenerateAdapters(samples, pdata)
	if err != nil {
		t.Fatalf("GenerateAdapters failed: %v", err)
	}

	// WGBS mode: no TGA/A prefix, just barcode + adapter
	expectedAdapter1 := "CGATAGATCGGAAGAGC"
	expectedAdapter2 := "CGATAGATCGGAAGAGC"

	if adapter1[0] != expectedAdapter1 {
		t.Errorf("WGBS mode: expected adapter1[0]=%s, got %s", expectedAdapter1, adapter1[0])
	}

	if adapter2[0] != expectedAdapter2 {
		t.Errorf("WGBS mode: expected adapter2[0]=%s, got %s", expectedAdapter2, adapter2[0])
	}
}

func TestAdapterGenerator_GenerateAdapters_NoPData(t *testing.T) {
	gen := NewAdapterGenerator(AdapterGeneratorOptions{
		BaseAdapter1: "AGATCGGAAGAGC",
		BaseAdapter2: "AGATCGGAAGAGC",
		Mode:         "RRBS",
	})

	samples := []string{"sample1"}

	adapter1, adapter2, err := gen.GenerateAdapters(samples, nil)
	if err != nil {
		t.Fatalf("GenerateAdapters failed: %v", err)
	}

	// Without pdata, should treat as no barcode
	if adapter1[0] != "NO_ADAPTER_CAL_USE_DEFAULT" {
		t.Errorf("Expected NO_ADAPTER_CAL_USE_DEFAULT without pdata, got %s", adapter1[0])
	}

	// adapter2 should also be NO_ADAPTER_CAL_USE_DEFAULT
	if adapter2[0] != "NO_ADAPTER_CAL_USE_DEFAULT" {
		t.Errorf("Expected NO_ADAPTER_CAL_USE_DEFAULT without pdata, got %s", adapter2[0])
	}
}

func TestAdapterGenerator_MultipleSamples(t *testing.T) {
	gen := NewAdapterGenerator(AdapterGeneratorOptions{
		BaseAdapter1: "AGATCGGAAGAGC",
		BaseAdapter2: "AGATCGGAAGAGC",
		Mode:         "RRBS",
	})

	samples := []string{"sample1", "sample2", "sample3"}
	pdata := &PData{
		Data: map[string]map[string]string{
			"sample1": {"sampleid": "sample1", "inline_barcode_sequence": "ATCG"},
			"sample2": {"sampleid": "sample2", "inline_barcode_sequence": ""},
			"sample3": {"sampleid": "sample3", "inline_barcode_sequence": "GCTA"},
		},
	}

	adapter1, adapter2, err := gen.GenerateAdapters(samples, pdata)
	if err != nil {
		t.Fatalf("GenerateAdapters failed: %v", err)
	}

	// Multiple samples should not have placeholders
	if len(adapter1) != 3 || len(adapter2) != 3 {
		t.Fatalf("Expected 3 adapters each for 3 samples, got adapter1=%d, adapter2=%d", len(adapter1), len(adapter2))
	}

	// sample1: ATCG -> CGAT -> TGACGATAGATCGGAAGAGC
	if adapter1[0] != "TGACGATAGATCGGAAGAGC" {
		t.Errorf("Sample1: expected TGACGATAGATCGGAAGAGC, got %s", adapter1[0])
	}

	// sample2: no barcode -> NO_ADAPTER_CAL_USE_DEFAULT
	if adapter1[1] != "NO_ADAPTER_CAL_USE_DEFAULT" {
		t.Errorf("Sample2: expected NO_ADAPTER_CAL_USE_DEFAULT, got %s", adapter1[1])
	}

	// sample3: GCTA -> TAGC -> TGATAGCAGATCGGAAGAGC
	if adapter1[2] != "TGATAGCAGATCGGAAGAGC" {
		t.Errorf("Sample3: expected TGATAGCAGATCGGAAGAGC, got %s", adapter1[2])
	}
}

func TestReverseComplement(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ATCG", "CGAT"},
		{"AATTGGCC", "GGCCAATT"},
		{"atcg", "cgat"},
		{"AAA", "TTT"},
		{"CCC", "GGG"},
	}

	for _, test := range tests {
		result := reverseComplement(test.input)
		if result != test.expected {
			t.Errorf("reverseComplement(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}
