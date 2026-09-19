package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/internal/input"
	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/spf13/cobra"
)

var (
	createFastqDir  string
	createPdataFile string
	createMode      string
	createOutputDir string
	createJobID     string
	createSuffix1   string
	createSuffix2   string
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new analysis project",
	Long: `Create a new analysis project after validating paired samples.

This writes a canonical v1 project into --output (default "."): it verifies
every declared reference against the reference registry, then writes
project.yaml, samples.tsv, and references.lock.yaml so "otter config resolve"
can produce an immutable run immediately.

It scans the FASTQ directory for paired samples (R1/R2), validates sample
pairing, loads pdata when provided, and derives per-sample adapters.

PDX scenarios are selected by naming both reference roles with
--reference-graft and --reference-host; non-PDX scenarios use
--reference-primary.

Examples:
  otter init my_project
  otter create --output my_project --fastq data/fastq --mode RRBS \
    --reference-root /shared/otter/references --reference-primary hg38@GRCh38.p14
  otter create --output my_project --fastq data/fastq --mode RNASEQ \
    --reference-graft hg38@GRCh38.p14 --reference-host mm10@GRCm38.p6`,
	RunE: runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)

	// Basic parameters
	createCmd.Flags().StringVarP(&createFastqDir, "fastq", "f", "", "FASTQ files directory (required)")
	createCmd.Flags().StringVarP(&createPdataFile, "pdata", "p", "", "Phenotype data file (Excel/CSV)")
	createCmd.Flags().StringVarP(&createMode, "mode", "m", "RRBS", "Workflow mode (RRBS/WGBS/RNASEQ)")
	createCmd.Flags().StringVarP(&createOutputDir, "output", "o", ".", "Project root, initialised by \"otter init\"")
	createCmd.Flags().StringVar(&createJobID, "jobid", "", "Project id (default: derived from the project directory name)")
	createCmd.Flags().StringVar(&createSuffix1, "suffix1", "_R1.fastq.gz", "R1 file suffix")
	createCmd.Flags().StringVar(&createSuffix2, "suffix2", "", "R2 file suffix (auto-derived if empty)")

	// Reference registry flags
	registerCanonicalCreateFlags(createCmd)

	createCmd.MarkFlagRequired("fastq")
}

func validateCreateOutputDir(outputDir string) error {
	trimmed := strings.TrimSpace(outputDir)
	if trimmed == "" {
		return fmt.Errorf("--output must be a directory")
	}

	cleaned := filepath.Clean(trimmed)
	switch strings.ToLower(filepath.Ext(cleaned)) {
	case ".yaml", ".yml", ".json", ".toml":
		return fmt.Errorf("--output must be a directory, got file-like path %q; use a directory such as my_project", outputDir)
	}

	return nil
}

// sampleIntakeRequest carries the inputs that decide which FASTQ files become
// samples. It exists so "otter create" and "otter build" run one intake routine
// instead of each reading the package-level flags, which would let the two
// entry points drift in how they pair or reject samples.
type sampleIntakeRequest struct {
	FastqDir  string
	Suffix1   string
	Suffix2   string
	PdataFile string
}

// sampleIntake is the validated result of scanning a FASTQ directory. Sample
// names are preserved in scan order so every downstream record keeps the same
// ordering as the input directory.
type sampleIntake struct {
	PairedSamples []input.PairedSample
	PData         *input.PData
	SampleNames   []string
}

// collectSampleIntake scans, pairs, loads pdata, and validates. Every rejection
// that "otter create" reports for a malformed directory originates here.
func collectSampleIntake(request sampleIntakeRequest) (sampleIntake, error) {
	if _, err := os.Stat(request.FastqDir); os.IsNotExist(err) {
		return sampleIntake{}, fmt.Errorf("FASTq directory does not exist: %s", request.FastqDir)
	}

	logger.Infof("Scanning FASTQ directory: %s", request.FastqDir)
	scanner := input.NewScanner(&input.ScanOptions{
		FastqDir: request.FastqDir,
		Suffix1:  request.Suffix1,
		Suffix2:  request.Suffix2,
	})

	samples, err := scanner.Scan()
	if err != nil {
		return sampleIntake{}, fmt.Errorf("failed to scan FASTQ files: %w", err)
	}
	if len(samples) == 0 {
		return sampleIntake{}, fmt.Errorf("no FASTQ files found in directory")
	}
	logger.Infof("Found %d FASTQ files", len(samples))

	pairedSamples, err := scanner.PairSamples(samples, nil)
	if err != nil {
		return sampleIntake{}, fmt.Errorf("failed to pair samples: %w", err)
	}

	validPairs := make([]input.PairedSample, 0, len(pairedSamples))
	for _, pairedSample := range pairedSamples {
		if pairedSample.Valid {
			validPairs = append(validPairs, pairedSample)
		}
	}
	if len(validPairs) == 0 {
		return sampleIntake{}, fmt.Errorf("no valid paired samples found. Check file naming conventions")
	}
	logger.Infof("Found %d valid paired samples", len(validPairs))

	var pdata *input.PData
	if request.PdataFile != "" {
		logger.Infof("Loading pdata file: %s", request.PdataFile)
		parser := input.NewPDataParser()
		pdata, err = parser.Load(request.PdataFile)
		if err != nil {
			return sampleIntake{}, fmt.Errorf("failed to load pdata: %w", err)
		}
		logger.Infof("Loaded pdata with %d samples", len(pdata.Samples))
	}

	validator := input.NewValidator()
	validationResult := validator.ValidateInput(request.FastqDir, request.PdataFile, validPairs, pdata)
	for _, warning := range validationResult.Warnings {
		logger.Warn(warning)
	}
	if !validationResult.Valid {
		for _, validationError := range validationResult.Errors {
			logger.Error(validationError)
		}
		return sampleIntake{}, fmt.Errorf("validation failed. Please fix the errors above")
	}

	sampleNames := make([]string, len(validPairs))
	for index, pairedSample := range validPairs {
		sampleNames[index] = pairedSample.Name
	}

	return sampleIntake{
		PairedSamples: validPairs,
		PData:         pdata,
		SampleNames:   sampleNames,
	}, nil
}

// generateSampleAdapters derives the per-sample adapters a project records.
// Both "otter create" and "otter build" call it so a project never pins an
// adapter that disagrees with its own samples.
func generateSampleAdapters(modeStr string, sampleNames []string, pdata *input.PData) ([]string, []string, error) {
	adapterGenerator := input.NewAdapterGenerator(input.AdapterGeneratorOptions{
		BaseAdapter1: "AGATCGGAAGAGC",
		BaseAdapter2: "AGATCGGAAGAGC",
		Mode:         modeStr,
	})
	adapter1, adapter2, err := adapterGenerator.GenerateAdapters(sampleNames, pdata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate adapters: %w", err)
	}
	logger.Infof("Generated adapters for %d samples", len(sampleNames))
	return adapter1, adapter2, nil
}

func runCreate(cmd *cobra.Command, args []string) error {
	logger.Info("Creating analysis project...")

	if err := validateCreateOutputDir(createOutputDir); err != nil {
		return err
	}

	intake, err := collectSampleIntake(sampleIntakeRequest{
		FastqDir:  createFastqDir,
		Suffix1:   createSuffix1,
		Suffix2:   createSuffix2,
		PdataFile: createPdataFile,
	})
	if err != nil {
		return err
	}

	modeStr := strings.ToUpper(createMode)

	adapter1, adapter2, err := generateSampleAdapters(modeStr, intake.SampleNames, intake.PData)
	if err != nil {
		return err
	}

	return runCreateCanonical(cmd, modeStr, intake.PairedSamples, intake.PData, adapter1, adapter2)
}

// runCreateCanonical authorises a canonical v1 project at --output.
//
// --output is the project root (default "." so create augments the directory
// initialised by "otter init"), while --jobid becomes the canonical project id.
func runCreateCanonical(
	cmd *cobra.Command,
	modeStr string,
	validPairs []input.PairedSample,
	pdata *input.PData,
	adapter1, adapter2 []string,
) error {
	flags, err := readCanonicalCreateFlags(cmd)
	if err != nil {
		return err
	}

	// The reference roles carry the PDX intent: naming graft and host references
	// is what selects a PDX scenario.
	pdxMode := strings.TrimSpace(flags.Graft) != "" || strings.TrimSpace(flags.Host) != ""

	scenario, err := createScenarioForMode(modeStr, pdxMode)
	if err != nil {
		return err
	}
	if err := flags.validateForScenario(scenario); err != nil {
		return err
	}
	if err := flags.validateCompleteness(scenario); err != nil {
		return err
	}
	projectID := createJobID
	if projectID == "" {
		projectID = defaultCanonicalProjectID(createOutputDir)
	}
	return runCreateCanonicalTrack(
		createOutputDir,
		projectID,
		scenario,
		flags.ReferenceRoot,
		flags.referenceRoleValues(),
		validPairs,
		pdata,
		adapter1,
		adapter2,
	)
}
