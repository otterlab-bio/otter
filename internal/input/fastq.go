package input

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/otterlab-bio/otter/internal/logger"
)

// Scanner scans directories for FASTQ files
type Scanner struct {
	options *ScanOptions
}

// NewScanner creates a new FASTQ scanner
func NewScanner(options *ScanOptions) *Scanner {
	// Set default extensions if not provided
	if options.Extensions == nil || len(options.Extensions) == 0 {
		options.Extensions = []string{".fastq.gz", ".fastq", ".fq.gz", ".fq"}
	}

	// Set default suffix if not provided
	if options.Suffix1 == "" {
		options.Suffix1 = "_R1.fastq.gz"
	}
	if options.Suffix2 == "" {
		options.Suffix2 = "" // Will be auto-derived
	}

	return &Scanner{
		options: options,
	}
}

// Scan scans the directory for FASTQ files
func (s *Scanner) Scan() ([]Sample, error) {
	logger.Infof("Scanning FASTQ directory: %s", s.options.FastqDir)

	entries, err := os.ReadDir(s.options.FastqDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("FASTQ directory not found: %s", s.options.FastqDir)
		}
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	fastqFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		file := filepath.Join(s.options.FastqDir, entry.Name())
		if s.isFastqFile(file) {
			fastqFiles = append(fastqFiles, file)
		}
	}

	logger.Debugf("Found %d FASTQ files", len(fastqFiles))

	// Group files by sample name
	samples, err := s.groupFiles(fastqFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to group files: %w", err)
	}

	logger.Infof("Identified %d samples", len(samples))

	return samples, nil
}

// isFastqFile checks if a file is a FASTQ file based on extension
func (s *Scanner) isFastqFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	lowerName := strings.ToLower(fileName)
	for _, validExt := range s.options.Extensions {
		if strings.HasSuffix(lowerName, validExt) {
			return true
		}
	}
	return false
}

// groupFiles groups FASTQ files by exact configured mate suffix.
func (s *Scanner) groupFiles(files []string) ([]Sample, error) {
	suffix1 := s.options.Suffix1
	suffix2 := s.options.Suffix2
	if suffix2 == "" {
		suffix2 = deriveSuffix2(suffix1)
	}
	if suffix1 == "" || suffix2 == "" || suffix1 == suffix2 {
		return nil, fmt.Errorf("FASTQ mate suffixes must be distinct and non-empty: suffix1=%q suffix2=%q", suffix1, suffix2)
	}

	logger.Debugf("Using suffix1=%s, suffix2=%s", suffix1, suffix2)

	samplesByName := make(map[string]*Sample)
	now := time.Now()
	for _, file := range files {
		fileName := filepath.Base(file)
		mate := 0
		sampleName := ""
		switch {
		case strings.HasSuffix(fileName, suffix1):
			mate = 1
			sampleName = strings.TrimSuffix(fileName, suffix1)
		case strings.HasSuffix(fileName, suffix2):
			mate = 2
			sampleName = strings.TrimSuffix(fileName, suffix2)
		default:
			return nil, fmt.Errorf("FASTQ file %q matches neither configured mate suffix %q nor %q", fileName, suffix1, suffix2)
		}
		if strings.TrimSpace(sampleName) == "" {
			return nil, fmt.Errorf("FASTQ file %q produces an empty sample name", fileName)
		}

		sample := samplesByName[sampleName]
		if sample == nil {
			sample = &Sample{
				Name:      sampleName,
				Metadata:  make(map[string]string),
				CreatedAt: now,
			}
			samplesByName[sampleName] = sample
		}

		if mate == 1 {
			if sample.FastqR1 != "" {
				return nil, fmt.Errorf("sample %q has multiple R1 files: %q and %q", sampleName, sample.FastqR1, file)
			}
			sample.FastqR1 = file
		} else {
			if sample.FastqR2 != "" {
				return nil, fmt.Errorf("sample %q has multiple R2 files: %q and %q", sampleName, sample.FastqR2, file)
			}
			sample.FastqR2 = file
		}
	}

	sampleNames := make([]string, 0, len(samplesByName))
	for sampleName := range samplesByName {
		sampleNames = append(sampleNames, sampleName)
	}
	sort.Strings(sampleNames)

	result := make([]Sample, 0, len(sampleNames))
	for _, sampleName := range sampleNames {
		result = append(result, *samplesByName[sampleName])
	}
	return result, nil
}

// PairSamples pairs R1 and R2 files for each sample
func (s *Scanner) PairSamples(samples []Sample, options *PairOptions) ([]PairedSample, error) {
	if options == nil {
		options = &PairOptions{
			StrictMatching: true,
			AllowSingleEnd: false,
			MinFiles:       1,
		}
	}

	var pairedSamples []PairedSample

	for _, sample := range samples {
		paired := PairedSample{
			Name:   sample.Name,
			R1Path: sample.FastqR1,
			R2Path: sample.FastqR2,
			Valid:  false,
		}

		// Check if sample is valid
		if paired.R1Path != "" && paired.R2Path != "" {
			// Both R1 and R2 exist
			paired.Valid = true
		} else if paired.R1Path != "" && options.AllowSingleEnd {
			// Only R1 exists and single-end is allowed
			paired.Valid = true
		} else if options.StrictMatching {
			// Strict matching requires both R1 and R2
			logger.Warnf("Sample %s is missing R2 file", sample.Name)
			continue
		}

		pairedSamples = append(pairedSamples, paired)
	}

	logger.Infof("Successfully paired %d/%d samples", len(pairedSamples), len(samples))

	return pairedSamples, nil
}

// deriveSuffix2 auto-derives suffix2 from suffix1
func deriveSuffix2(suffix1 string) string {
	// Try replacing '1' with '2'
	if strings.Contains(suffix1, "1") {
		return strings.Replace(suffix1, "1", "2", 1)
	}

	// Try replacing "R1" with "R2"
	if strings.Contains(suffix1, "R1") {
		return strings.Replace(suffix1, "R1", "R2", 1)
	}

	// Fallback: return original
	return suffix1
}
