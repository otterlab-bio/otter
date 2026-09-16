package input

import (
	"fmt"
	"strings"
)

// AdapterGenerator handles per-sample adapter generation
type AdapterGenerator struct {
	baseAdapter1 string
	baseAdapter2 string
	mode         string
}

// AdapterGeneratorOptions contains configuration for adapter generation
type AdapterGeneratorOptions struct {
	BaseAdapter1 string
	BaseAdapter2 string
	Mode         string
}

// NewAdapterGenerator creates a new AdapterGenerator
func NewAdapterGenerator(opts AdapterGeneratorOptions) *AdapterGenerator {
	return &AdapterGenerator{
		baseAdapter1: opts.BaseAdapter1,
		baseAdapter2: opts.BaseAdapter2,
		mode:         opts.Mode,
	}
}

// GenerateAdapters generates per-sample adapters
// Returns two slices: adapter1 and adapter2 for each sample
func (g *AdapterGenerator) GenerateAdapters(samples []string, pdata *PData) (adapter1, adapter2 []string, err error) {
	// Initialize arrays with one adapter per sample
	adapter1 = make([]string, len(samples))
	adapter2 = make([]string, len(samples))

	for i, sample := range samples {
		// Get the barcode for this sample from pdata
		barcode := g.getSampleBarcode(sample, pdata)

		// Build adapters for this sample
		adapter1[i], adapter2[i] = g.buildAdapterForSample(barcode)
	}

	return adapter1, adapter2, nil
}

// getSampleBarcode retrieves the inline barcode sequence for a sample from pdata
// Returns empty string if not found
func (g *AdapterGenerator) getSampleBarcode(sample string, pdata *PData) string {
	if pdata == nil || pdata.Data == nil {
		return ""
	}

	// Search for the sample in pdata.Data
	if sampleData, ok := pdata.Data[sample]; ok {
		// Try multiple possible column names for barcode
		barcodeColumns := []string{
			"inline_barcode_sequence", // Standard column name
			"barcode",                 // Common alias
			"barcode_sequence",        // Alternative name
		}

		for _, col := range barcodeColumns {
			if barcode, ok := sampleData[col]; ok && barcode != "" {
				return barcode
			}
		}
	}

	return ""
}

// buildAdapterForSample constructs adapters for a single sample
// Handles empty barcode case with NO_ADAPTER_CAL_USE_DEFAULT marker
func (g *AdapterGenerator) buildAdapterForSample(barcode string) (adapter1, adapter2 string) {
	// If no barcode is provided, return special marker that Snakemake recognizes
	// This causes trim_galore to enter automatic adapter detection mode
	if barcode == "" {
		return "NO_ADAPTER_CAL_USE_DEFAULT", "NO_ADAPTER_CAL_USE_DEFAULT"
	}

	// Get reverse complement of barcode
	revCompBarcode := reverseComplement(barcode)

	// Build adapters based on mode
	switch strings.ToUpper(g.mode) {
	case "RRBS":
		// RRBS mode: TGA + revComp(barcode) + adapter / A + revComp(barcode) + adapter
		// Matches R package logic in lines 617-618 of beavergandalf.R
		adapter1 = fmt.Sprintf("TGA%s%s", revCompBarcode, g.baseAdapter1)
		adapter2 = fmt.Sprintf("A%s%s", revCompBarcode, g.baseAdapter2)
	default:
		// Other modes: revComp(barcode) + adapter
		adapter1 = fmt.Sprintf("%s%s", revCompBarcode, g.baseAdapter1)
		adapter2 = fmt.Sprintf("%s%s", revCompBarcode, g.baseAdapter2)
	}

	return adapter1, adapter2
}

// reverseComplement computes the reverse complement of a DNA sequence
func reverseComplement(seq string) string {
	complement := map[byte]byte{
		'A': 'T', 'T': 'A', 'G': 'C', 'C': 'G',
		'a': 't', 't': 'a', 'g': 'c', 'c': 'g',
	}

	var result strings.Builder
	result.Grow(len(seq))

	for i := len(seq) - 1; i >= 0; i-- {
		r := seq[i]
		if comp, ok := complement[r]; ok {
			result.WriteByte(comp)
		} else {
			// Preserve non-DNA characters (e.g., N, -)
			result.WriteByte(r)
		}
	}

	return result.String()
}
