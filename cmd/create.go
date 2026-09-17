package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/internal/config"
	"github.com/otterlab-bio/otter/internal/input"
	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// generateReferencePaths generates reference file paths based on mode and species.
// Custom paths provided by the user take priority over defaults.
func generateReferencePaths(mode, species2, genome1Fasta, genome1Index, genome2Fasta, genome2Index, gtf1, gtf2, starIndex1, starIndex2 string) (fasta, index, gtf, ref []string, err error) {
	mode = strings.ToUpper(mode)

	// If user provided custom paths, use them; otherwise use default paths.

	// Species1 (graft/primary)
	if genome1Fasta != "" {
		fasta = append(fasta, genome1Fasta)
	} else {
		// Use default path
		fasta = append(fasta, fmt.Sprintf("inst/pdx/homo_sapiens/hg19.fasta"))
	}

	if genome1Index != "" {
		index = append(index, genome1Index)
	} else {
		index = append(index, "inst/pdx/homo_sapiens/")
	}

	// Species2 (host/secondary) - PDX mode only
	if species2 != "" {
		if genome2Fasta != "" {
			fasta = append(fasta, genome2Fasta)
		} else {
			fasta = append(fasta, "inst/pdx/mouse/GRCm38.fasta")
		}

		if genome2Index != "" {
			index = append(index, genome2Index)
		} else {
			index = append(index, "inst/pdx/mouse/")
		}
	}

	// RNA-seq requires GTF and STAR indices
	if mode == "RNASEQ" {
		if gtf1 != "" {
			gtf = append(gtf, gtf1)
		} else {
			gtf = append(gtf, "inst/rnaseq/homo_sapiens/hg19.ensGene_sorted.gtf")
		}

		if starIndex1 != "" {
			ref = append(ref, starIndex1)
		} else {
			ref = append(ref, "inst/rnaseq/homo_sapiens/")
		}

		// In PDX mode, add host (species2) GTF and indices
		if species2 != "" {
			if gtf2 != "" {
				gtf = append(gtf, gtf2)
			} else {
				gtf = append(gtf, "inst/rnaseq/mouse/GRCm38.ensGene_sorted.gtf")
			}

			if starIndex2 != "" {
				ref = append(ref, starIndex2)
			} else {
				ref = append(ref, "inst/rnaseq/mouse/")
			}
		}
	}

	return
}

var (
	createFastqDir  string
	createPdataFile string
	createMode      string
	createSpecies1  string
	createSpecies2  string
	createOutputDir string
	createJobID     string
	createSuffix1   string
	createSuffix2   string
	createCondaEnv  string

	// createLegacyTrack selects the legacy compatibility authoring contract.
	createLegacyTrack bool

	// Reference genome files
	createGenome1Fasta string
	createGenome1Index string
	createGenome2Fasta string
	createGenome2Index string
	createGTF1         string
	createGTF2         string
	createStarIndex1   string
	createStarIndex2   string
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new analysis project",
	Long: `Create a new analysis project after validating paired samples.

By default this writes a canonical v1 project into --output (default "."): it
verifies every declared reference against the reference registry, then writes
project.yaml, samples.tsv, and references.lock.yaml so "otter config resolve"
can produce an immutable run immediately.

With --legacy it writes the established compatibility artifact instead:
config/otter.yaml under --output/--jobid, executed through the explicit
Snakemake compatibility executor.

Both steps scan the FASTQ directory for paired samples (R1/R2), validate sample
pairing, load pdata when provided, and derive per-sample adapters.

Canonical examples:
  otter init my_project
  otter create --output my_project --fastq data/fastq --mode RRBS \
    --reference-root /shared/otter/references --reference-primary hg38@GRCh38.p14
  otter create --output my_project --fastq data/fastq --mode RNASEQ \
    --reference-graft hg38@GRCh38.p14 --reference-host mm10@GRCm38.p6

Legacy examples:
  otter init my_project --legacy
  otter create --legacy --fastq data/fastq --mode RRBS --output my_project/userspace --jobid demo_rrbs`,
	RunE: runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)

	// Basic parameters
	createCmd.Flags().StringVarP(&createFastqDir, "fastq", "f", "", "FASTQ files directory (required)")
	createCmd.Flags().StringVarP(&createPdataFile, "pdata", "p", "", "Phenotype data file (Excel/CSV)")
	createCmd.Flags().StringVarP(&createMode, "mode", "m", "RRBS", "Workflow mode (RRBS/WGBS/RNASEQ)")
	createCmd.Flags().StringVar(&createSpecies1, "species1", "human", "Primary species")
	createCmd.Flags().StringVar(&createSpecies2, "species2", "", "Secondary species (enables PDX mode)")
	createCmd.Flags().StringVarP(&createOutputDir, "output", "o", "userspace", "Project root for canonical projects (default \".\"), or the parent of userspace/<jobid> for legacy projects")
	createCmd.Flags().StringVar(&createJobID, "jobid", "", "Project id for canonical projects, or the job directory name for legacy projects (default: auto)")
	createCmd.Flags().StringVar(&createSuffix1, "suffix1", "_R1.fastq.gz", "R1 file suffix")
	createCmd.Flags().StringVar(&createSuffix2, "suffix2", "", "R2 file suffix (auto-derived if empty)")
	createCmd.Flags().StringVar(&createCondaEnv, "conda-env", "", "Conda environment for Snakemake")

	// Canonical authoring flags
	registerCanonicalCreateFlags(createCmd)

	// Reference genome files (legacy track only; uses defaults if not specified)
	createCmd.Flags().StringVar(&createGenome1Fasta, "genome1-fasta", "", "Legacy track: primary species genome FASTA file (e.g., inst/hg19/hg19.fasta)")
	createCmd.Flags().StringVar(&createGenome1Index, "genome1-index", "", "Legacy track: primary species genome index directory (e.g., inst/hg19/)")
	createCmd.Flags().StringVar(&createGenome2Fasta, "genome2-fasta", "", "Legacy track: secondary species genome FASTA file for PDX (e.g., inst/mm10/mm10.fasta)")
	createCmd.Flags().StringVar(&createGenome2Index, "genome2-index", "", "Legacy track: secondary species genome index directory for PDX (e.g., inst/mm10/)")
	createCmd.Flags().StringVar(&createGTF1, "gtf1", "", "Legacy track: primary species GTF annotation file for RNA-seq (e.g., inst/rnaseq/hg19/hg19.ensGene_sorted.gtf)")
	createCmd.Flags().StringVar(&createGTF2, "gtf2", "", "Legacy track: secondary species GTF annotation file for PDX RNA-seq (e.g., inst/rnaseq/mm10/mm10.ensGene_sorted.gtf)")
	createCmd.Flags().StringVar(&createStarIndex1, "star-index1", "", "Legacy track: primary species STAR index directory for RNA-seq (e.g., inst/rnaseq/hg19/)")
	createCmd.Flags().StringVar(&createStarIndex2, "star-index2", "", "Legacy track: secondary species STAR index directory for PDX RNA-seq (e.g., inst/rnaseq/mm10/)")

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
		return fmt.Errorf("--output must be a directory, got file-like path %q; use a directory such as my_project/userspace", outputDir)
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
// Both tracks call it so a project never pins an adapter that disagrees with
// its own samples.
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

	if !createLegacyTrack {
		// The canonical track augments the directory "otter init" created, so it
		// defaults to the current directory instead of the legacy userspace/.
		if !cmd.Flags().Changed("output") {
			createOutputDir = "."
		}
		if err := validateCreateOutputDir(createOutputDir); err != nil {
			return err
		}
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
	validPairs := intake.PairedSamples
	pdata := intake.PData
	sampleNames := intake.SampleNames

	// Detect PDX mode
	pdxMode := createSpecies2 != ""
	modeStr := strings.ToUpper(createMode)
	if pdxMode {
		logger.Infof("PDX mode enabled: %s + %s", createSpecies1, createSpecies2)
	}

	// Derive adapters once; both tracks record them for their samples.
	adapter1, adapter2, err := generateSampleAdapters(modeStr, sampleNames, pdata)
	if err != nil {
		return err
	}

	if !createLegacyTrack {
		return runCreateCanonical(cmd, modeStr, pdxMode, validPairs, pdata, adapter1, adapter2)
	}

	// 8. Validate reference genome files (if custom paths are provided)
	logger.Info("Validating reference genome files...")
	if err := validateReferenceGenomeFiles(
		modeStr, createSpecies1, createSpecies2,
		createGenome1Fasta, createGenome1Index,
		createGenome2Fasta, createGenome2Index,
		createGTF1, createGTF2,
		createStarIndex1, createStarIndex2,
	); err != nil {
		return fmt.Errorf("reference genome validation failed: %w", err)
	}

	// 9. Generate job ID
	jobID := createJobID
	if jobID == "" {
		jobID = generateJobID()
	}

	// Ensure jobID doesn't already exist
	projectDir := filepath.Join(createOutputDir, jobID)
	if _, err := os.Stat(projectDir); err == nil {
		if createJobID != "" {
			return fmt.Errorf("project with jobid '%s' already exists", jobID)
		}
		// Regenerate if auto-generated
		jobID = generateJobID()
		projectDir = filepath.Join(createOutputDir, jobID)
	}

	// 10. Create project directory structure
	logger.Infof("Creating project directory: %s", projectDir)
	if err := createProjectStructure(projectDir, modeStr, createSpecies1, createSpecies2); err != nil {
		return fmt.Errorf("failed to create project structure: %w", err)
	}

	// 11. Generate otter.yaml
	configPath := filepath.Join(projectDir, "config", "otter.yaml")
	if err := generateProjectConfig(configPath, modeStr, createSpecies1, createSpecies2,
		createFastqDir, createPdataFile, sampleNames, projectDir, adapter1, adapter2, pdata, jobID); err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// 12. Print summary
	logger.Info("===========================================")
	logger.Infof("Analysis project created successfully!")
	logger.Info("===========================================")
	logger.Infof("  Job ID:    %s", jobID)
	logger.Infof("  Mode:      %s%s", modeStr, func() string {
		if pdxMode {
			return " (PDX)"
		}
		return ""
	}())
	logger.Infof("  Samples:   %d paired samples", len(validPairs))
	logger.Infof("  Project:   %s", projectDir)
	logger.Infof("  Config:    %s", configPath)
	logger.Info("")
	logger.Info("Sample list:")
	for i, name := range sampleNames {
		if i < 5 {
			logger.Infof("  - %s", name)
		} else if i == 5 {
			logger.Infof("  ... and %d more", len(sampleNames)-5)
			break
		}
	}
	logger.Info("")
	logger.Info("Next step:")
	logger.Infof("  otter run --config %s --executor snakemake", configPath)

	return nil
}

// runCreateCanonical authorises a canonical v1 project at --output.
//
// --output is the project root for this track (default "." so create augments
// the directory initialised by "otter init"), while --jobid becomes the
// canonical project id.
func runCreateCanonical(
	cmd *cobra.Command,
	modeStr string,
	species2PDX bool,
	validPairs []input.PairedSample,
	pdata *input.PData,
	adapter1, adapter2 []string,
) error {
	flags, err := readCanonicalCreateFlags(cmd)
	if err != nil {
		return err
	}

	// In the canonical track the reference roles carry the PDX intent, so naming
	// graft and host references is enough to select a PDX scenario. The legacy
	// --species2 toggle stays supported for callers that already use it.
	pdxMode := species2PDX || strings.TrimSpace(flags.Graft) != "" || strings.TrimSpace(flags.Host) != ""

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

func optionalString(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

// generateJobID generates a random job ID (similar to R's openssl::rand_bytes)
func generateJobID() string {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("job_%d", os.Getpid())
	}
	return hex.EncodeToString(b)
}

// validateReferenceGenomeFiles validates that all specified reference genome files exist
func validateReferenceGenomeFiles(mode, species1, species2, genome1Fasta, genome1Index, genome2Fasta, genome2Index, gtf1, gtf2, starIndex1, starIndex2 string) error {
	mode = strings.ToUpper(mode)
	var errors []string

	// Validate species1 files
	if genome1Fasta != "" {
		if _, err := os.Stat(genome1Fasta); os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("Species1 genome FASTA not found: %s", genome1Fasta))
		}
	}

	if genome1Index != "" {
		if _, err := os.Stat(genome1Index); os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("Species1 genome index not found: %s", genome1Index))
		}
	}

	// Validate species2 files (for PDX mode)
	if species2 != "" {
		if genome2Fasta != "" {
			if _, err := os.Stat(genome2Fasta); os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("Species2 genome FASTA not found: %s", genome2Fasta))
			}
		}

		if genome2Index != "" {
			if _, err := os.Stat(genome2Index); os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("Species2 genome index not found: %s", genome2Index))
			}
		}
	}

	// Validate RNA-seq specific files
	if mode == "RNASEQ" {
		if gtf1 != "" {
			if _, err := os.Stat(gtf1); os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("Species1 GTF file not found: %s", gtf1))
			}
		}

		if starIndex1 != "" {
			if _, err := os.Stat(starIndex1); os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("Species1 STAR index not found: %s", starIndex1))
			}
		}

		// Validate species2 files for PDX RNA-seq
		if species2 != "" {
			if gtf2 != "" {
				if _, err := os.Stat(gtf2); os.IsNotExist(err) {
					errors = append(errors, fmt.Sprintf("Species2 GTF file not found: %s", gtf2))
				}
			}

			if starIndex2 != "" {
				if _, err := os.Stat(starIndex2); os.IsNotExist(err) {
					errors = append(errors, fmt.Sprintf("Species2 STAR index not found: %s", starIndex2))
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("reference genome validation failed:\n%s", strings.Join(errors, "\n"))
	}

	logger.Info("Reference genome files validation passed")
	return nil
}

// createProjectStructure creates the complete project directory structure
func createProjectStructure(projectDir, mode, species1, species2 string) error {
	// Base directories
	baseDirs := []string{
		"data",
		"workflow",
		"analysis",
		"config",
		"log",
	}

	// Workflow subdirectories
	workflowDirs := []string{
		"workflow/QC",
		"workflow/fastqc_raw",
		"workflow/fastqc_clean",
		"workflow/trim",
		"workflow/bsmap",
		"workflow/bsmap/tmp",
		"workflow/mCall",
		"workflow/umx",
		"workflow/qualimap",
		"workflow/mhap",
	}

	// Analysis subdirectories
	analysisDirs := []string{
		"analysis/methrixh5",
		"analysis/GCbias",
		"analysis/clubcpg",
		"analysis/clubcpg/coverage_before",
		"analysis/clubcpg/model",
		"analysis/clubcpg/coverage_impute",
		"analysis/RData",
		"analysis/qc_summary",
		"analysis/betaM",
		"analysis/DMR",
		"analysis/uxm_summary",
		"analysis/logsummary",
	}

	// PDX-specific directories
	pdxMode := species2 != ""
	if pdxMode {
		workflowDirs = append(workflowDirs,
			fmt.Sprintf("workflow/bsmap/tmp/%s", species1),
			fmt.Sprintf("workflow/bsmap/tmp/%s", species2),
			fmt.Sprintf("workflow/bsmap/%s", species1),
			fmt.Sprintf("workflow/bsmap/%s", species2),
			"workflow/bsmap/Filtered_bams",
		)
	}

	// RNA-seq specific directories
	if strings.ToUpper(mode) == "RNASEQ" {
		workflowDirs = append(workflowDirs,
			"workflow/star",
			"workflow/htseq",
			"workflow/splicing",
		)
		analysisDirs = append(analysisDirs,
			"analysis/counts",
			"analysis/DEG",
		)
	}

	// Combine all directories
	allDirs := append(baseDirs, workflowDirs...)
	allDirs = append(allDirs, analysisDirs...)

	// Create all directories
	for _, dir := range allDirs {
		fullPath := filepath.Join(projectDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
		}
	}

	logger.Debugf("Created %d directories", len(allDirs))
	return nil
}

// calculateGroupLevels calculates the number of unique groups in pdata
// Returns 0 if no pdata or no group information available
func calculateGroupLevels(pdata *input.PData, samples []string) int {
	if pdata == nil || pdata.Data == nil {
		logger.Debug("No pdata provided, group_levels = 0")
		return 0
	}

	// Use a map to track unique groups
	groups := make(map[string]bool)

	for _, sample := range samples {
		if sampleData, ok := pdata.Data[sample]; ok {
			// Priority: sample_group > condition
			var group string
			var exists bool

			if group, exists = sampleData["sample_group"]; exists && group != "" {
				groups[group] = true
			} else if group, exists = sampleData["condition"]; exists && group != "" {
				groups[group] = true
			}
		}
	}

	groupCount := len(groups)
	logger.Debugf("Found %d unique groups: %v", groupCount, groups)

	return groupCount
}

// generateProjectConfig generates the otter.yaml file with pure nested structure
func generateProjectConfig(configPath, mode, species1, species2,
	fastqDir, pdataFile string, samples []string, projectDir string,
	adapter1, adapter2 []string, pdata *input.PData, jobID string) error {

	// Build configuration
	pdxMode := species2 != ""
	speciesNames := append([]string{species1}, optionalString(species2)...)
	// workflowName := config.GetWorkflowName(mode, pdxMode)
	// stepCount := config.GetStepCount(mode, pdxMode)

	// Calculate group levels
	groupLevels := calculateGroupLevels(pdata, samples)

	// Generate reference paths dynamically
	fasta, index, gtf, ref, err := generateReferencePaths(
		mode, species2,
		createGenome1Fasta, createGenome1Index,
		createGenome2Fasta, createGenome2Index,
		createGTF1, createGTF2,
		createStarIndex1, createStarIndex2,
	)
	if err != nil {
		return fmt.Errorf("failed to generate reference paths: %w", err)
	}

	// Prepare genome_anno array
	genomeAnno := []string{species1}
	if species2 != "" {
		genomeAnno = append(genomeAnno, species2)
	}

	// Prepare rnaseq fields based on mode
	// Always use list format for snakemake compatibility (index operation requires list)
	var rnaseqGTF interface{}
	var rnaseqRef interface{}
	if len(gtf) == 0 {
		rnaseqGTF = ""
		rnaseqRef = []string{}
	} else if len(gtf) == 1 {
		// Wrap in list for snakemake index operation
		rnaseqGTF = []string{gtf[0]}
		rnaseqRef = []string{ref[0]}
	} else {
		rnaseqGTF = gtf
		rnaseqRef = ref
	}

	// Auto-derive suffix2 from suffix1 if empty
	suffix2Value := createSuffix2
	if suffix2Value == "" {
		// Derive R2 suffix from R1 suffix
		if strings.HasSuffix(createSuffix1, "_R1.fastq.gz") {
			suffix2Value = strings.Replace(createSuffix1, "_R1.fastq.gz", "_R2.fastq.gz", 1)
		} else if strings.HasSuffix(createSuffix1, "_R1.fastq") {
			suffix2Value = strings.Replace(createSuffix1, "_R1.fastq", "_R2.fastq", 1)
		} else if strings.HasSuffix(createSuffix1, "_R1") {
			suffix2Value = strings.Replace(createSuffix1, "_R1", "_R2", 1)
		} else {
			suffix2Value = "_R2.fastq.gz" // Default fallback
		}
	}

	// Calculate directory paths
	workflowDir := filepath.Join(projectDir, "workflow")
	analysisDir := filepath.Join(projectDir, "analysis")
	selfconfig := filepath.Join(projectDir, "config")
	qcDir := filepath.Join(projectDir, "workflow", "QC")
	qcDirBefore := filepath.Join(projectDir, "workflow", "fastqc_raw")
	qcDirAfter := filepath.Join(projectDir, "workflow", "fastqc_clean")
	sidLog := filepath.Join(projectDir, "workflow", "log")
	trimDir := filepath.Join(projectDir, "workflow", "trim")
	bsmapDir := filepath.Join(projectDir, "workflow", "bsmap")
	bsmapDirBamtmp := filepath.Join(projectDir, "workflow", "bsmap", "tmp")
	outDirMCall := filepath.Join(projectDir, "workflow", "mCall")
	ourDirUmx := filepath.Join(projectDir, "workflow", "uxm")
	outdirQualimap := filepath.Join(projectDir, "workflow", "QC", "qualimap")
	outDirMhap := filepath.Join(projectDir, "workflow", "mhap")
	rdataFolder := filepath.Join(projectDir, "workflow", "RData")
	dmrFolder := filepath.Join(projectDir, "analysis", "DMR")
	rawDir := filepath.Join(projectDir, "data")
	outDirBetaM := filepath.Join(projectDir, "analysis", "betaM")
	qcSummary := filepath.Join(projectDir, "QC")
	logsummary := filepath.Join(projectDir, "log")
	uxmSummary := filepath.Join(projectDir, "analysis", "uxm")
	clubcpg := filepath.Join(projectDir, "workflow", "clubcpg")
	clubcpgCoverageBefore := filepath.Join(projectDir, "workflow", "clubcpg", "clubcpg_coverage_before")
	clubcpgModel := filepath.Join(projectDir, "workflow", "clubcpg", "clubcpg_model")
	clubcpgCoverageImpute := filepath.Join(projectDir, "workflow", "clubcpg", "clubcpg_coverage_impute")
	methrixh5 := filepath.Join(projectDir, "workflow", "mCall", "methrixh5")
	gcbias := filepath.Join(projectDir, "workflow", "QC", "GCbias")

	// Prepare sample configs
	sampleConfigs := make([]config.SampleConfig, len(samples))
	for i, sample := range samples {
		sampleConfigs[i] = config.SampleConfig{
			Name: sample,
			R1:   fmt.Sprintf("%s/%s%s", fastqDir, sample, createSuffix1),
			R2:   fmt.Sprintf("%s/%s%s", fastqDir, sample, suffix2Value),
		}
	}

	// Build the nested configuration structure
	cfg := config.OtterConfig{
		Workflow: config.WorkflowConfig{
			Mode:   mode,
			UserID: jobID,
			JobID:  jobID,
			Species: config.SpeciesConfig{
				Primary:   species1,
				Secondary: species2,
				Graft:     species1,
				Host:      species2,
				Name:      speciesNames,
			},
			Adapters: config.AdapterConfig{
				Seq1:      adapter1,
				Seq2:      adapter2,
				ErrorRate: 0.2,
			},
			Trim: config.TrimConfig{
				Read1Five:  0.0,
				Read1Three: 0.0,
				Read2Five:  0.0,
				Read2Three: 0.0,
				SeqDepth:   10.0,
				Fixed:      "",
			},
			Alignment: config.AlignmentConfig{
				C1: "7",
				C2: "9",
				T1: 0,
				T2: 0,
			},
			Samples: sampleConfigs,
		},
		Input: config.InputConfig{
			FastqDir:  fastqDir,
			PdataFile: pdataFile,
			Suffix1:   createSuffix1,
			Suffix2:   suffix2Value,
		},
		Output: config.OutputConfig{
			BaseDir:     projectDir,
			WorkflowDir: workflowDir,
			AnalysisDir: analysisDir,
			RawDir:      rawDir,
			LogDir:      filepath.Join(projectDir, "log"),
			TrimDir:     trimDir,
		},
		Reference: config.ReferenceConfig{
			Genome: strings.ToLower(species1),
			Files: config.ReferenceFiles{
				Fasta: fasta,
			},
			Indices: config.ReferenceIndices{
				Genome: index,
			},
			Annotations: config.AnnotationConfig{
				Names: genomeAnno,
			},
			RNAseq: config.RNAseqConfig{
				GTF:         rnaseqGTF,
				Reference:   rnaseqRef,
				Chromosomes: []string{},
			},
		},
		Directories: config.DirectoryConfig{
			Base:     projectDir,
			Work:     projectDir,
			Workflow: workflowDir,
			Analysis: analysisDir,
			Config:   selfconfig,
			QC: config.QCConfig{
				Main:   qcDir,
				Before: qcDirBefore,
				After:  qcDirAfter,
			},
			SIDLog: sidLog,
			BSMAP: config.BSMAPConfig{
				Main:     bsmapDir,
				Temp:     bsmapDirBamtmp,
				Filtered: filepath.Join(bsmapDir, "Filtered_bams"),
			},
			MethylationCall: outDirMCall,
			UMX:             ourDirUmx,
			Qualimap:        outdirQualimap,
			MHAP:            outDirMhap,
			RData:           rdataFolder,
			DMR:             dmrFolder,
			BetaMatrix:      outDirBetaM,
			QCSummary:       qcSummary,
			LogSummary:      logsummary,
			UXMSummary:      uxmSummary,
			ClubCpG: config.ClubCpGConfig{
				Main:     clubcpg,
				Coverage: clubcpgCoverageBefore,
				Model:    clubcpgModel,
				Impute:   clubcpgCoverageImpute,
			},
			MethrixH5: methrixh5,
			GCBias:    gcbias,
		},
		Parallel: config.ParallelConfig{
			Workers:      4,
			DwarfWorkers: 1,
		},
		Metadata: config.MetadataConfig{
			SampleIDs: samples,
			UserEmail: "",
			PDXPipeline: func() string {
				if pdxMode {
					return "yes"
				}
				return "no"
			}(),
			GroupLevels: groupLevels,
		},
	}

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	// Build nested configuration structure for rootless_rules compatibility
	// This ensures Snakemake can access nested fields like config["workflow.jobid"]
	nestedConfig := map[string]interface{}{
		// Workflow section
		"workflow": map[string]interface{}{
			"jobid": cfg.Workflow.JobID,
			"species": map[string]interface{}{
				"graft": cfg.Workflow.Species.Graft,
				"host":  cfg.Workflow.Species.Host,
				"name":  speciesNames,
			},
			"adapters": map[string]interface{}{
				"seq1":  cfg.Workflow.Adapters.Seq1,
				"seq2":  cfg.Workflow.Adapters.Seq2,
				"error": cfg.Workflow.Adapters.ErrorRate,
			},
			"trim": map[string]interface{}{
				"read1_5":  cfg.Workflow.Trim.Read1Five,
				"read1_3":  cfg.Workflow.Trim.Read1Three,
				"read2_5":  cfg.Workflow.Trim.Read2Five,
				"read2_3":  cfg.Workflow.Trim.Read2Three,
				"seq_deth": cfg.Workflow.Trim.SeqDepth,
				"fixed":    cfg.Workflow.Trim.Fixed,
			},
			"alignment": map[string]interface{}{
				"c1": cfg.Workflow.Alignment.C1,
				"c2": cfg.Workflow.Alignment.C2,
				"t1": cfg.Workflow.Alignment.T1,
				"t2": cfg.Workflow.Alignment.T2,
			},
		},

		// Input section
		"input": map[string]interface{}{
			"fastq_dir":  cfg.Input.FastqDir,
			"pdata_file": cfg.Input.PdataFile,
			"suffix":     cfg.Input.Suffix1,
			"suffix2":    cfg.Input.Suffix2,
		},

		// Output section
		"output": map[string]interface{}{
			"base_dir":     cfg.Output.BaseDir,
			"workflow_dir": cfg.Output.WorkflowDir,
			"analysis_dir": cfg.Output.AnalysisDir,
			"raw_dir":      cfg.Output.RawDir,
			"log_dir":      cfg.Output.LogDir,
			"trim_dir":     cfg.Output.TrimDir,
		},

		// Reference section
		"reference": map[string]interface{}{
			"genome": cfg.Reference.Genome,
			"files": map[string]interface{}{
				"fasta": cfg.Reference.Files.Fasta,
			},
			"indices": map[string]interface{}{
				"genome": cfg.Reference.Indices.Genome,
			},
			"annotations": map[string]interface{}{
				"names": cfg.Reference.Annotations.Names,
			},
			"rnaseq": map[string]interface{}{
				"gtf": cfg.Reference.RNAseq.GTF,
				"ref": cfg.Reference.RNAseq.Reference,
			},
		},

		// Directories section
		"directories": map[string]interface{}{
			"work":       cfg.Directories.Work,
			"selfconfig": cfg.Directories.Config,
			"qc": map[string]interface{}{
				"main":   cfg.Directories.QC.Main,
				"before": cfg.Directories.QC.Before,
				"after":  cfg.Directories.QC.After,
			},
			"bsmap": map[string]interface{}{
				"main":     cfg.Directories.BSMAP.Main,
				"temp":     cfg.Directories.BSMAP.Temp,
				"filtered": cfg.Directories.BSMAP.Filtered,
			},
			"methylation_call": cfg.Directories.MethylationCall,
			"mhap":             cfg.Directories.MHAP,
			"qualimap":         cfg.Directories.Qualimap,
			"clubcpg": map[string]interface{}{
				"main":     cfg.Directories.ClubCpG.Main,
				"coverage": cfg.Directories.ClubCpG.Coverage,
				"model":    cfg.Directories.ClubCpG.Model,
				"impute":   cfg.Directories.ClubCpG.Impute,
			},
			"beta_matrix": cfg.Directories.BetaMatrix,
			"qc_summary":  cfg.Directories.QCSummary,
			"sid_log":     cfg.Directories.SIDLog,
			"uxm_summary": cfg.Directories.UXMSummary,
			"methrix_h5":  cfg.Directories.MethrixH5,
			"gc_bias":     cfg.Directories.GCBias,
		},

		// Metadata section
		"metadata": map[string]interface{}{
			"sample_ids":   samples,
			"user_email":   cfg.Metadata.UserEmail,
			"pdx_pipeline": cfg.Metadata.PDXPipeline,
			"group_levels": cfg.Metadata.GroupLevels,
		},

		// Parallel section
		"parallel": map[string]interface{}{
			"workers": cfg.Parallel.Workers,
		},

		// Engine section
		"engine": map[string]interface{}{
			"type":      "auto",
			"conda_env": createCondaEnv,
		},

		// Add flat fields for backward compatibility with old Snakefiles
		// These are not used by rootless_rules but kept for compatibility
		"SIDs":    samples,
		"mode":    cfg.Workflow.Mode,
		"species": speciesNames,
		"workers": cfg.Parallel.Workers,
	}

	// Write YAML file with nested structure
	data, err := yaml.Marshal(nestedConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add header comment
	header := `# otter Analysis Project Configuration
# Generated automatically by: otter create
#
# This file uses nested structure for rootless_rules compatibility.
# Edit this file to customize your analysis parameters.
# Then run: otter run --config <this-file>
#

`
	content := header + string(data)

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
