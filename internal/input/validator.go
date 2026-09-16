package input

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/otterlab-bio/otter/internal/logger"
)

// Validator validates input files and configurations
type Validator struct{}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateInput validates FASTQ files and pdata
func (v *Validator) ValidateInput(fastqDir, pdataFile string, pairedSamples []PairedSample, pdata *PData) *InputValidationResult {
	result := &InputValidationResult{
		Valid:         true,
		Errors:        []string{},
		Warnings:      []string{},
		PairedSamples: pairedSamples,
		PData:         pdata,
	}

	// Validate FASTQ directory
	if fastqDir != "" {
		if err := v.validateFastqDir(fastqDir); err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	// Validate pdata file
	if pdataFile != "" {
		if err := v.validatePdataFile(pdataFile); err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	// Validate paired samples
	if len(pairedSamples) == 0 {
		result.Errors = append(result.Errors, "no paired samples found")
	}

	// Validate each paired sample
	for _, sample := range pairedSamples {
		if !sample.Valid {
			result.Warnings = append(result.Warnings, fmt.Sprintf("sample '%s' is not valid", sample.Name))
		}

		// Check if R1 file exists
		if sample.R1Path != "" {
			if _, err := os.Stat(sample.R1Path); os.IsNotExist(err) {
				result.Errors = append(result.Errors, fmt.Sprintf("R1 file not found: %s", sample.R1Path))
			}
		}

		// Check if R2 file exists
		if sample.R2Path != "" {
			if _, err := os.Stat(sample.R2Path); os.IsNotExist(err) {
				result.Errors = append(result.Errors, fmt.Sprintf("R2 file not found: %s", sample.R2Path))
			}
		}
	}

	// Validate pdata vs samples
	if pdata != nil && len(pairedSamples) > 0 {
		pdataErrors := v.validatePdataSamples(pairedSamples, pdata)
		result.Errors = append(result.Errors, pdataErrors...)
	}

	// Set valid flag
	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

// validateFastqDir validates the FASTQ directory
func (v *Validator) validateFastqDir(dir string) error {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("FASTQ directory not found: %s", dir)
	}

	// Check if directory is readable
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("FASTQ directory is not readable: %s", dir)
	}
	defer f.Close()

	logger.Debug("FASTQ directory validation passed")
	return nil
}

// validatePdataFile validates the pdata file
func (v *Validator) validatePdataFile(filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("pdata file not found: %s", filePath)
	}

	// Check file extension
	ext := filepath.Ext(filePath)
	validExts := []string{".csv", ".xlsx", ".xls"}
	isValidExt := false
	for _, validExt := range validExts {
		if ext == validExt {
			isValidExt = true
			break
		}
	}

	if !isValidExt {
		return fmt.Errorf("pdata file has invalid extension: %s (supported: %v)", ext, validExts)
	}

	logger.Debug("pdata file validation passed")
	return nil
}

// validatePdataSamples validates that pdata contains all samples
func (v *Validator) validatePdataSamples(pairedSamples []PairedSample, pdata *PData) []string {
	var errors []string

	// Create a set of sample names from paired samples
	sampleSet := make(map[string]bool)
	for _, sample := range pairedSamples {
		sampleSet[sample.Name] = true
	}

	// Check each pdata sample against paired samples
	for _, sample := range pdata.Samples {
		if !sampleSet[sample] {
			errors = append(errors, fmt.Sprintf("sample '%s' in pdata not found in FASTQ files", sample))
		}
	}

	// Check for samples in FASTQ but not in pdata
	for _, sample := range pairedSamples {
		if _, ok := pdata.Data[sample.Name]; !ok {
			errors = append(errors, fmt.Sprintf("sample '%s' in FASTQ files not found in pdata", sample.Name))
		}
	}

	return errors
}

// GetSampleNames extracts sample names from paired samples
func GetSampleNames(pairedSamples []PairedSample) []string {
	sampleNames := make([]string, len(pairedSamples))
	for i, sample := range pairedSamples {
		sampleNames[i] = sample.Name
	}
	return sampleNames
}
