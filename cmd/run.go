package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/internal/assets"
	"github.com/otterlab-bio/otter/internal/config"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/engine"
	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/spf13/cobra"
)

// defaultRunConfigFile is the implicit --config value, used to tell "the user
// passed no config" apart from "the user passed this exact default".
const defaultRunConfigFile = "otter.yaml"

var (
	runConfigFile string
	runEngine     string
	runIDFlag     string
	dryRun        bool
	verboseRun    bool
	runCondaEnv   string
	copyFastq     bool
	moveFastq     bool

	// SLURM global settings (acts as unified partition)
	slurmPartition string
	slurmCores     int
	slurmMemory    string

	// Per-step resources
	step1Cores     int
	step1Memory    string
	step1Partition string
	step2Cores     int
	step2Memory    string
	step2Partition string
	step3Cores     int
	step3Memory    string
	step3Partition string

	// Checker resources
	step2CheckerCores  int
	step2CheckerMemory string
	step3CheckerCores  int
	step3CheckerMemory string

	// Parallel control
	parallelJobs int

	// Dynamic pool load ratio (replaces fixed batching for SLURM)
	loadRatio float64

	// Legacy unified partition parameter (kept for backward compatibility)
	slurmUnifiedPartition string

	// FASTQ compression option
	compressFastq bool

	// Resume option
	resumeFlag bool

	// Asset integrity options
	runProjectDir  string
	verifyAssets   bool
	strictAssets   bool
	foregroundRun  bool
	internalWorker bool
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a workflow",
	Long: `Execute a bioinformatics workflow with the specified configuration.

Examples:
  otter run --config otter.yaml
  otter run --config otter.yaml --engine slurm
  otter run --config otter.yaml --dry-run`,
	RunE: runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&runConfigFile, "config", "c", defaultRunConfigFile, "Immutable run.yaml snapshot path, or a resolved run directory")
	runCmd.Flags().StringVar(&runIDFlag, "run-id", "", "Resolved run ID under <project-dir>/runs/, for example run-20260915T094657Z-eghubq")
	runCmd.Flags().StringVarP(&runEngine, "engine", "e", "auto", "Execution engine (auto/slurm/local)")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Perform a dry run without executing")
	runCmd.Flags().BoolVarP(&verboseRun, "verbose", "v", false, "Verbose output")
	runCmd.Flags().StringVar(&runCondaEnv, "conda-env", "", "Conda environment for Snakemake")
	runCmd.Flags().BoolVar(&copyFastq, "copy-fastq", false, "Deprecated: rejected for immutable runs; stage FASTQ before resolving a run")
	runCmd.Flags().BoolVar(&moveFastq, "move-fastq", false, "Deprecated: rejected for immutable runs; stage FASTQ before resolving a run")
	runCmd.Flags().StringVar(&runProjectDir, "project-dir", "", "Project directory (defaults to directory containing --config)")
	runCmd.Flags().BoolVar(&verifyAssets, "verify-assets", true, "Verify workflow assets manifest before running (if present)")
	runCmd.Flags().BoolVar(&strictAssets, "strict-assets", false, "Fail fast if workflow assets differ from the manifest")

	// SLURM global settings
	runCmd.Flags().StringVar(&slurmPartition, "slurm-partition", "", "Default SLURM partition (overrides config)")
	runCmd.Flags().IntVar(&slurmCores, "slurm-cores", 0, "Default SLURM CPU cores")
	runCmd.Flags().StringVar(&slurmMemory, "slurm-memory", "", "Default SLURM memory (e.g., 16G)")

	// Per-step resources
	runCmd.Flags().IntVar(&step1Cores, "step1-cores", 0, "Step 1 CPU cores")
	runCmd.Flags().StringVar(&step1Memory, "step1-memory", "", "Step 1 memory (e.g., 8G)")
	runCmd.Flags().StringVar(&step1Partition, "step1-partition", "", "Step 1 partition")

	runCmd.Flags().IntVar(&step2Cores, "step2-cores", 0, "Step 2 CPU cores")
	runCmd.Flags().StringVar(&step2Memory, "step2-memory", "", "Step 2 memory (e.g., 32G)")
	runCmd.Flags().StringVar(&step2Partition, "step2-partition", "", "Step 2 partition")

	runCmd.Flags().IntVar(&step3Cores, "step3-cores", 0, "Step 3 CPU cores")
	runCmd.Flags().StringVar(&step3Memory, "step3-memory", "", "Step 3 memory (e.g., 16G)")
	runCmd.Flags().StringVar(&step3Partition, "step3-partition", "", "Step 3 partition")

	// Checker resources
	runCmd.Flags().IntVar(&step2CheckerCores, "step2-checker-cores", 0, "Step 2 checker CPU cores")
	runCmd.Flags().StringVar(&step2CheckerMemory, "step2-checker-memory", "", "Step 2 checker memory")
	runCmd.Flags().IntVar(&step3CheckerCores, "step3-checker-cores", 0, "Step 3 checker CPU cores")
	runCmd.Flags().StringVar(&step3CheckerMemory, "step3-checker-memory", "", "Step 3 checker memory")

	// Parallel control
	runCmd.Flags().IntVar(&parallelJobs, "parallel-jobs", 2, "Max parallel jobs (local/Snakemake)")

	// Dynamic pool load ratio for SLURM
	runCmd.Flags().Float64Var(&loadRatio, "load-ratio", 1.0,
		"Job pool load ratio (0.1-1.0). SLURM: slot_limit=floor(min(parallel-jobs, MaxSubmitJobs-1)×ratio). "+
			"Local: slot_limit=floor(parallel-jobs×ratio). Set to 0 to disable dynamic pool.")

	// Legacy unified partition parameter (kept for backward compatibility)
	runCmd.Flags().StringVar(&slurmUnifiedPartition, "slurm-unified-partition", "", "Unified SLURM partition for all steps (overrides individual step partitions)")

	// FASTQ compression option
	runCmd.Flags().BoolVar(&compressFastq, "compress-fastq", false, "Deprecated: rejected for immutable runs; compress FASTQ before resolving a run")

	// Resume option
	runCmd.Flags().BoolVarP(&resumeFlag, "resume", "r", false, "Resume from last completed step")
	runCmd.Flags().BoolVarP(&foregroundRun, "foreground", "F", false, "Run in the foreground instead of creating a background task")
	runCmd.Flags().BoolVar(&internalWorker, "internal-worker", false, "Run as an internal background worker")
	_ = runCmd.Flags().MarkHidden("internal-worker")
}

func runRun(cmd *cobra.Command, args []string) error {
	executorName, err := selectedRunExecutor()
	if err != nil {
		return err
	}
	if executorName == runExecutorCraftmake {
		if cmd.Flags().Changed("engine") {
			return fmt.Errorf("--engine is a Snakemake compatibility flag; use --backend with --executor craftmake")
		}
		if !dryRun && !foregroundRun && !internalWorker {
			return submitBackgroundCraftmakeRun(cmd)
		}
		return executeCraftmakeRun(cmd)
	}
	if cmd.Flags().Changed("backend") {
		if cmd.Flags().Changed("engine") && runEngine != runBackend {
			return fmt.Errorf("--engine %s conflicts with --backend %s", runEngine, runBackend)
		}
		runEngine = runBackend
	}
	if !dryRun && !foregroundRun && !internalWorker {
		return submitBackgroundSnakemakeSnapshotRun(cmd)
	}
	return executeSnakemakeSnapshotRun(cmd)
}

// buildBackgroundWorkerArguments rebuilds the worker command line for a
// background task: it drops foreground/worker/rebinding flags and pins the
// resolved config and project directory.
func buildBackgroundWorkerArguments(arguments []string, configPath, projectDir string) []string {
	workerArguments := make([]string, 0, len(arguments)+5)
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--foreground" || argument == "-F" || argument == "--internal-worker" {
			continue
		}
		if argument == "--config" || argument == "-c" || argument == "--project-dir" {
			index++
			continue
		}
		if strings.HasPrefix(argument, "--config=") || strings.HasPrefix(argument, "--project-dir=") {
			continue
		}
		workerArguments = append(workerArguments, argument)
	}
	workerArguments = append(workerArguments,
		"--config", configPath,
		"--project-dir", projectDir,
		"--internal-worker",
	)
	return workerArguments
}

// resolveRunPaths turns the configured flags into an absolute snapshot path and
// project directory.
//
// --run-id is a convenience for the canonical layout: it expands to
// <project-dir>/runs/<run-id>/run.yaml so a user does not have to spell the run
// directory out. It never resolves project.yaml into a snapshot; the snapshot
// must already exist, which is what keeps "run" a pure consumer of run.yaml.
func resolveRunPaths(configPath, projectDirOverride string) (string, string, error) {
	if strings.TrimSpace(runIDFlag) != "" {
		if strings.TrimSpace(configPath) != "" && configPath != defaultRunConfigFile {
			return "", "", fmt.Errorf("--run-id and --config are mutually exclusive; pass one of them")
		}
		return resolveRunPathsFromRunID(projectDirOverride)
	}
	if strings.TrimSpace(configPath) == "" {
		return "", "", fmt.Errorf("config file path is required")
	}

	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve config path %s: %w", configPath, err)
	}

	if strings.TrimSpace(projectDirOverride) != "" {
		absProjectDir, err := filepath.Abs(projectDirOverride)
		if err != nil {
			return "", "", fmt.Errorf("failed to resolve project directory %s: %w", projectDirOverride, err)
		}
		return absConfigPath, filepath.Clean(absProjectDir), nil
	}

	return absConfigPath, discoverProjectDirFromConfig(absConfigPath), nil
}

// resolveRunPathsFromRunID expands --run-id into the canonical snapshot path.
func resolveRunPathsFromRunID(projectDirOverride string) (string, string, error) {
	if !configv1.IsValidRunID(runIDFlag) {
		return "", "", fmt.Errorf("--run-id %q must match run-YYYYMMDDTHHMMSSZ-abcdef", runIDFlag)
	}
	projectRoot := projectDirOverride
	if strings.TrimSpace(projectRoot) == "" {
		projectRoot = "."
	}
	absoluteProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve project directory %s: %w", projectRoot, err)
	}
	snapshotPath := filepath.Join(absoluteProjectRoot, "runs", runIDFlag, "run.yaml")
	if _, statErr := os.Stat(snapshotPath); statErr != nil {
		return "", "", fmt.Errorf("run %q is not resolved in %s: %w (run `otter config resolve --project project.yaml` first)", runIDFlag, absoluteProjectRoot, statErr)
	}
	return snapshotPath, absoluteProjectRoot, nil
}

func discoverProjectDirFromConfig(configPath string) string {
	configDir := filepath.Dir(configPath)
	if projectDir, ok := findAncestorWithManifest(configDir); ok {
		return projectDir
	}
	return configDir
}

func findAncestorWithManifest(startDir string) (string, bool) {
	current := filepath.Clean(startDir)
	for {
		if _, err := os.Stat(assets.ManifestPath(current)); err == nil {
			return current, true
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

func shouldRelaxLocalDryRunResourceValidation(step int, stepResources map[int]*config.StepResource, dryRun bool) bool {
	if !dryRun {
		return false
	}

	customRes, hasCustom := stepResources[step]
	return !hasCustom || (customRes.Cores == 0 && customRes.Memory == "")
}

func shouldPreflightRNAsplicing(cfg *config.OtterConfig) bool {
	if strings.ToUpper(strings.TrimSpace(cfg.Workflow.Mode)) != "RNASEQ" {
		return false
	}
	return cfg.Metadata.GroupLevels >= 2
}

func validateRNAsplicingDependencies(cfg *config.OtterConfig) error {
	if !shouldPreflightRNAsplicing(cfg) {
		return nil
	}

	// Determine the environment name, preferring the configured conda environment.
	rnaEnv := cfg.Engine.CondaEnv
	if rnaEnv == "" {
		rnaEnv = "otter-core-bismark-rust-3.1.0-r2"
	}

	if _, err := exec.LookPath("enva"); err != nil {
		return fmt.Errorf(
			"RNA splicing preflight failed: enva not found in PATH: %w\n"+
				"Required for rnaseq_splicing: enva + %s environment with matsrun and rmats.py.\n"+
				"Try: enva run %s -- matsrun --help\n"+
				"Try: enva run %s -- rmats.py --help", err, rnaEnv, rnaEnv, rnaEnv)
	}

	checks := [][]string{
		{"enva", "run", rnaEnv, "--", "matsrun", "--help"},
		{"enva", "run", rnaEnv, "--", "rmats.py", "--help"},
	}

	for _, check := range checks {
		cmd := exec.Command(check[0], check[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf(
				"RNA splicing preflight failed while running `%s`: %w\noutput:\n%s\n"+
					"Required for rnaseq_splicing: enva + otter-core-bismark-rust-3.1.0-r2 with matsrun and rmats.py.\n"+
					"Try: enva run otter-core-bismark-rust-3.1.0-r2 -- matsrun --help\n"+
					"Try: enva run otter-core-bismark-rust-3.1.0-r2 -- rmats.py --help",
				strings.Join(check, " "), err, string(output))
		}
	}

	return nil
}

// validateResources validates local and SLURM resources
func validateResources(engineType string, cfg *config.OtterConfig, stepResources map[int]*config.StepResource, parallelJobs int, dryRun bool) error {
	logger.Info("Validating resources...")

	// Get workflow mode and PDX status
	mode := cfg.Workflow.Mode
	pdxMode := config.DetectPDXMode(cfg)

	// Validate each step
	for step := 1; step <= 3; step++ {
		// Get default step resource
		defaultRes := config.GetDefaultStepResource(step, mode, pdxMode)

		// Get final step resource (command line overrides defaults)
		stepRes := &config.StepResource{
			Cores:     defaultRes.Cores,
			Memory:    defaultRes.Memory,
			Partition: defaultRes.Partition,
		}

		if customRes, exists := stepResources[step]; exists {
			if customRes.Cores > 0 {
				stepRes.Cores = customRes.Cores
			}
			if customRes.Memory != "" {
				stepRes.Memory = customRes.Memory
			}
			if customRes.Partition != "" {
				stepRes.Partition = customRes.Partition
			}
		}

		// Validate based on engine type
		switch engineType {
		case "local":
			// Validate local resources
			if err := engine.ValidateLocalResources(stepRes.Cores, stepRes.Memory); err != nil {
				if shouldRelaxLocalDryRunResourceValidation(step, stepResources, dryRun) {
					logger.Warnf(
						"Step %d default local resources exceed this machine (%v); continuing because --dry-run only validates workflow structure. Override with --step%d-cores/--step%d-memory for a realistic local smoke test.",
						step, err, step, step,
					)
					continue
				}
				return fmt.Errorf("step %d local resource validation failed: %w", step, err)
			}

		case "slurm", "slurm_array":
			// Validate SLURM resources
			partition := stepRes.Partition
			if partition == "" {
				partition = "cpu" // Default partition
			}

			// Validate partition exists
			if err := engine.ValidateSlurmPartition(partition); err != nil {
				return fmt.Errorf("step %d SLURM partition validation failed: %w", step, err)
			}

			// Validate node resources (check if any node meets requirements)
			if err := engine.ValidateSlurmNodeResources(partition, stepRes.Cores, stepRes.Memory); err != nil {
				return fmt.Errorf("step %d SLURM node resource validation failed: %w", step, err)
			}

		default:
			logger.Debugf("Engine type %s, skipping resource validation", engineType)
		}
	}

	// Validate parallel jobs
	if parallelJobs > 0 {
		if engineType == "local" {
			if err := engine.ValidateParallelJobs(parallelJobs); err != nil {
				return fmt.Errorf("parallel jobs validation failed: %w", err)
			}
		} else if parallelJobs > 20 {
			logger.Warnf("High parallel jobs (%d) may cause SLURM queue congestion", parallelJobs)
		}
	}

	logger.Info("Resource validation completed successfully")
	return nil
}
