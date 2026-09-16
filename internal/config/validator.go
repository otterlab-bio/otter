package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/internal/logger"
)

// ValidateConfig validates the workflow configuration
func ValidateConfig(config *OtterConfig) error {
	// Validate mode
	mode := strings.ToUpper(config.Workflow.Mode)
	switch mode {
	case "RRBS", "WGBS", "BSSEQ", "RNASEQ":
	default:
		return fmt.Errorf("invalid mode: %s. Valid modes: %v", config.Workflow.Mode, []string{"RRBS", "WGBS", "BSSEQ", "RNASEQ"})
	}

	// Detect PDX mode
	pdxMode := DetectPDXMode(config)
	if pdxMode {
		logger.Info("PDX mode detected (species1 and species2 specified)")
	}

	// Validate species
	if config.Workflow.Species.Primary == "" {
		return fmt.Errorf("species1 must be specified")
	}

	// Validate FASTQ directories
	if config.Input.FastqDir != "" {
		if absPath, err := filepath.Abs(config.Input.FastqDir); err == nil {
			logger.Debugf("FASTQ directory: %s", absPath)
		}
	}

	// Validate suffix patterns
	if config.Input.Suffix1 == "" {
		return fmt.Errorf("suffix1 must be specified")
	}
	if config.Input.Suffix2 == "" {
		logger.Warn("suffix2 is empty, will be auto-derived from suffix1")
	}

	// Validate parallel settings
	if config.Parallel.Workers <= 0 {
		logger.Warn("Invalid workers setting, using default: 4")
		config.Parallel.Workers = 4
	}

	// Validate reference configuration
	if len(config.Reference.Files.Fasta) > 0 {
		for _, fasta := range config.Reference.Files.Fasta {
			if !fileExists(fasta) {
				logger.Warnf("Reference FASTA not found: %s", fasta)
			}
		}
	}

	// Validate alignment parameters
	if config.Workflow.Adapters.ErrorRate < 0 || config.Workflow.Adapters.ErrorRate > 1 {
		return fmt.Errorf("error_rate must be between 0 and 1, got: %f", config.Workflow.Adapters.ErrorRate)
	}

	logger.Debug("Configuration validation passed")
	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

// DetectPDXMode detects if PDX mode should be enabled
func DetectPDXMode(config *OtterConfig) bool {
	return config.Workflow.Species.Primary != "" && config.Workflow.Species.Secondary != ""
}

// GetWorkflowName returns the workflow name based on mode and PDX flag
func GetWorkflowName(mode string, pdx bool) string {
	switch {
	case pdx && mode == "RNASEQ":
		return "BeaverRNASEQPDX"
	case pdx:
		return "BeaverPDX"
	case mode == "RNASEQ":
		return "BeaverRNA"
	default: // RRBS/WGBS/BSSEQ
		return "BeaverBS"
	}
}

// GetStepCount returns the number of steps for the workflow
func GetStepCount(mode string, pdx bool) int {
	if mode == "RNASEQ" && !pdx {
		return 2 // RNASEQ non-PDX only has 2 steps
	}
	return 3 // All other modes have 3 steps
}
