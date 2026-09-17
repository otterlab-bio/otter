package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/otterlab-bio/otter/internal/projectlayout"
	refpkg "github.com/otterlab-bio/otter/internal/reference"
	"github.com/spf13/cobra"
)

// buildFlags carries every input "otter build" needs to author a canonical
// project and freeze its first run.
//
// It mirrors the flags of the three commands the pipeline chains, so a caller
// migrating from manual invocation can move each flag across unchanged.
type buildFlags struct {
	// ProjectRoot is the directory to author into. It defaults to "." because
	// the canonical track authors in place.
	ProjectRoot string

	// Authoring inputs, as accepted by "otter create".
	FastqDir       string
	PdataFile      string
	Mode           string
	Suffix1        string
	Suffix2        string
	ProjectID      string
	ReferenceRoot  string
	PrimaryRef     string
	GraftRef       string
	HostRef        string
	SkipValidation bool

	// Run selection, as accepted by "otter config resolve".
	Executor      string
	Backend       string
	Site          string
	RunID         string
	ParentRunID   string
	PrimaryRunRef string
	GraftRunRef   string
	HostRunRef    string
}

// newBuildCommand wires the authoring pipeline into one verb: initialise the
// canonical layout, author the project, then freeze a run snapshot.
//
// It deliberately stops before execution. Resolving produces the immutable
// runs/<run-id>/run.yaml that both executors consume, so the caller can inspect
// or hand off that snapshot; executing it stays a separate, explicit step.
func newBuildCommand() *cobra.Command {
	flags := buildFlags{}
	command := &cobra.Command{
		Use:   "build",
		Short: "Author a canonical project and freeze its first run in one step",
		Long: `Author a canonical project and freeze its first run.

This chains the canonical authoring steps in order:

  1. otter init            pin the packaged workflow assets and write project.lock.yaml
  2. otter create          verify declared references, write project.yaml and references.lock.yaml
  3. otter config resolve  write the immutable runs/<run-id>/run.yaml

The default executor is the craftmake execution layer. Pass --executor snakemake
to record the Snakemake compatibility executor in the snapshot instead; the
snapshot is the same either way, so the choice does not change the project on
disk and can be revisited when resolving a later run.

The command stops after resolve. It does not execute the run, so nothing is
submitted or launched until you invoke "otter run" against the printed snapshot.

Examples:
  otter build --fastq data/fastq --mode RRBS \
    --reference-root /shared/otter/references --reference-primary mm39@GRCm39-gencode-M39

  otter build --fastq data/fastq --mode RNASEQ \
    --reference-graft hg38@GRCh38.p14 --reference-host mm10@GRCm38.p6 \
    --executor snakemake --backend slurm`,
		RunE: func(command *cobra.Command, args []string) error {
			return runBuild(command, flags)
		},
	}

	command.Flags().StringVarP(&flags.ProjectRoot, "project-root", "o", ".", "Directory to author the canonical project into")

	command.Flags().StringVarP(&flags.FastqDir, "fastq", "f", "", "FASTQ files directory (required)")
	command.Flags().StringVarP(&flags.PdataFile, "pdata", "p", "", "Phenotype data file (Excel/CSV)")
	command.Flags().StringVarP(&flags.Mode, "mode", "m", "RRBS", "Workflow mode (RRBS/WGBS/RNASEQ)")
	command.Flags().StringVar(&flags.Suffix1, "suffix1", "_R1.fastq.gz", "R1 file suffix")
	command.Flags().StringVar(&flags.Suffix2, "suffix2", "", "R2 file suffix (auto-derived if empty)")
	command.Flags().StringVar(&flags.ProjectID, "jobid", "", "Canonical project id (default: derived from the project root name)")
	command.Flags().StringVar(&flags.ReferenceRoot, "reference-root", "", "Reference registry root (default: $OTTER_REFERENCE_ROOT or ~/.otter/references)")
	command.Flags().StringVar(&flags.PrimaryRef, "reference-primary", "", "Primary reference as id@release for RRBS/WGBS/RNA-seq")
	command.Flags().StringVar(&flags.GraftRef, "reference-graft", "", "Graft (primary) reference as id@release for BS-PDX/RNA-PDX")
	command.Flags().StringVar(&flags.HostRef, "reference-host", "", "Host (secondary) reference as id@release for BS-PDX/RNA-PDX")
	command.Flags().BoolVar(&flags.SkipValidation, "skip-validate", false, "Skip the standalone project validation step (resolve still validates)")

	command.Flags().StringVar(&flags.Executor, "executor", "craftmake", "Executor recorded in the run snapshot: craftmake or snakemake")
	command.Flags().StringVar(&flags.Backend, "backend", "", "Resolved backend override: local or slurm")
	command.Flags().StringVar(&flags.Site, "site", "", "Resolved site override")
	command.Flags().StringVar(&flags.RunID, "run-id", "", "Explicit run ID matching run-YYYYMMDDTHHMMSSZ-abcdef")
	command.Flags().StringVar(&flags.ParentRunID, "parent-run-id", "", "Optional lineage parent run ID")
	command.Flags().StringVar(&flags.PrimaryRunRef, "run-reference-primary", "", "Run-level primary reference override as id@release")
	command.Flags().StringVar(&flags.GraftRunRef, "run-reference-graft", "", "Run-level graft reference override as id@release")
	command.Flags().StringVar(&flags.HostRunRef, "run-reference-host", "", "Run-level host reference override as id@release")

	command.MarkFlagRequired("fastq")

	return command
}

func init() {
	rootCmd.AddCommand(newBuildCommand())
}

// runBuild is the pipeline itself. Each stage is a function shared with the
// standalone command it mirrors, so the two entry points cannot diverge.
func runBuild(command *cobra.Command, flags buildFlags) error {
	projectRoot, err := filepath.Abs(flags.ProjectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root %q: %w", flags.ProjectRoot, err)
	}
	if strings.TrimSpace(flags.Executor) == "" {
		flags.Executor = string(configv1.ExecutorCraftmake)
	}
	if err := ensureBuildProjectRootExists(projectRoot); err != nil {
		return err
	}

	// Resolve the registry root once, before either stage runs. The authoring
	// stage locks references against it and the resolve stage reopens the same
	// releases, so if the two stages picked different defaults the resolve would
	// look for a release the project never locked.
	referenceRoot := strings.TrimSpace(flags.ReferenceRoot)
	if referenceRoot == "" {
		referenceRoot, err = refpkg.DefaultRegistryRoot()
		if err != nil {
			return err
		}
	}
	flags.ReferenceRoot = referenceRoot
	logger.Infof("Reference registry: %s", referenceRoot)

	skipped, err := reportExistingBuild(projectRoot, filepath.Join(projectRoot, "project.yaml"))
	if err != nil {
		return err
	}

	logger.Info("===========================================")
	logger.Info("Stage 1/3: initialising the canonical project")
	logger.Info("===========================================")
	if !skipped {
		projectName := defaultCanonicalProjectID(projectRoot)
		if err := runInitCanonicalTrack(projectName, projectRoot); err != nil {
			return err
		}
	} else {
		logger.Infof("%s is already initialised; reusing the pinned assets", projectRoot)
	}

	logger.Info("")
	logger.Info("===========================================")
	logger.Info("Stage 2/3: authoring the project")
	logger.Info("===========================================")
	if err := runBuildAuthoring(flags, projectRoot); err != nil {
		return err
	}

	if flags.SkipValidation {
		logger.Info("Skipping project validation (--skip-validate); resolve validates the same project")
	} else {
		projectPath := filepath.Join(projectRoot, "project.yaml")
		project, err := configv1.LoadProject(projectPath)
		if err != nil {
			return err
		}
		logger.Infof("Valid canonical project: %s (%s)", project.Project.ID, project.Workflow.Scenario)
	}

	logger.Info("")
	logger.Info("===========================================")
	logger.Info("Stage 3/3: resolving the run snapshot")
	logger.Info("===========================================")
	snapshotPath, err := resolveRunSnapshot(configResolveFlags{
		ProjectPath:      filepath.Join(projectRoot, "project.yaml"),
		ReferenceRoot:    flags.ReferenceRoot,
		Executor:         flags.Executor,
		Backend:          flags.Backend,
		Site:             flags.Site,
		PrimaryReference: flags.PrimaryRunRef,
		GraftReference:   flags.GraftRunRef,
		HostReference:    flags.HostRunRef,
		RunID:            flags.RunID,
		ParentRunID:      flags.ParentRunID,
	})
	if err != nil {
		return err
	}

	logger.Info("")
	logger.Info("===========================================")
	logger.Info("Build complete")
	logger.Info("===========================================")
	logger.Infof("  Project root: %s", projectRoot)
	logger.Infof("  Snapshot:     %s", snapshotPath)
	logger.Infof("  Executor:     %s", flags.Executor)
	logger.Info("")
	logger.Info("Next step:")
	logger.Infof("  otter run --config %s --executor %s", snapshotPath, flags.Executor)

	// The path goes to stdout on its own line so the command stays scriptable
	// exactly like "otter config resolve", which is the stage it ends on.
	fmt.Fprintln(command.OutOrStdout(), snapshotPath)
	return nil
}

// reportExistingBuild decides whether the init stage can be skipped.
//
// Re-running init would rewrite project.lock.yaml and re-pin the assets, which
// is exactly the state a later run snapshot is checksummed against, so an
// already-initialised directory reuses its existing lock.
//
// It also refuses an already-authored project. "otter create" never overwrites
// project.yaml, because rewriting project intent in place would silently
// invalidate every snapshot already resolved from it. A second build is
// therefore a mistake worth naming before the pipeline starts, not an error
// discovered two stages in with a freshly re-pinned lock behind it.
func reportExistingBuild(projectRoot, projectPath string) (bool, error) {
	if _, err := os.Stat(projectPath); err == nil {
		return false, fmt.Errorf(
			"%s already exists in %s; `otter build` authors a project once. Delete it explicitly to re-author, or resolve another run from it with `otter config resolve --project %s`",
			filepath.Base(projectPath), projectRoot, projectPath,
		)
	}

	detected, err := projectlayout.Detect(projectRoot)
	if err != nil {
		return false, err
	}
	switch detected {
	case projectlayout.TrackV1:
		return true, nil
	case projectlayout.TrackLegacy:
		return false, fmt.Errorf(
			"%s is a legacy compatibility project; `otter build` authors canonical projects only. Run `otter config migrate` to convert it, or build into a new directory",
			projectRoot,
		)
	default:
		return false, nil
	}
}

// runBuildAuthoring runs the create stage against explicit values rather than
// the create command's package-level flags, so "otter build" and
// "otter create" cannot drift.
func runBuildAuthoring(flags buildFlags, projectRoot string) error {
	if err := validateCreateOutputDir(projectRoot); err != nil {
		return err
	}

	intake, err := collectSampleIntake(sampleIntakeRequest{
		FastqDir:  flags.FastqDir,
		Suffix1:   flags.Suffix1,
		Suffix2:   flags.Suffix2,
		PdataFile: flags.PdataFile,
	})
	if err != nil {
		return err
	}

	modeStr := strings.ToUpper(strings.TrimSpace(flags.Mode))
	// In the canonical track the reference roles carry the PDX intent: naming
	// graft and host references is enough to select a PDX scenario.
	pdxMode := strings.TrimSpace(flags.GraftRef) != "" || strings.TrimSpace(flags.HostRef) != ""
	if pdxMode {
		logger.Infof("PDX mode selected by the graft/host reference roles for mode %s", modeStr)
	}

	scenario, err := createScenarioForMode(modeStr, pdxMode)
	if err != nil {
		return err
	}

	referenceRoot := strings.TrimSpace(flags.ReferenceRoot)
	if referenceRoot == "" {
		// runBuild resolves the default before either stage runs; reaching here
		// means the two would disagree, which is the drift this pipeline exists
		// to prevent.
		return fmt.Errorf("internal error: reference root was not resolved before authoring")
	}

	createFlags := canonicalCreateFlags{
		ReferenceRoot: referenceRoot,
		Primary:       flags.PrimaryRef,
		Graft:         flags.GraftRef,
		Host:          flags.HostRef,
	}
	if err := createFlags.validateForScenario(scenario); err != nil {
		return err
	}
	if err := createFlags.validateCompleteness(scenario); err != nil {
		return err
	}

	projectID := strings.TrimSpace(flags.ProjectID)
	if projectID == "" {
		projectID = defaultCanonicalProjectID(projectRoot)
	}

	adapter1, adapter2, err := generateSampleAdapters(modeStr, intake.SampleNames, intake.PData)
	if err != nil {
		return err
	}

	return runCreateCanonicalTrack(
		projectRoot,
		projectID,
		scenario,
		referenceRoot,
		createFlags.referenceRoleValues(),
		intake.PairedSamples,
		intake.PData,
		adapter1,
		adapter2,
	)
}

// ensureBuildProjectRootExists keeps the error for a missing directory
// actionable rather than letting the first os.MkdirAll create a parent the
// caller mistyped.
func ensureBuildProjectRootExists(projectRoot string) error {
	if _, err := os.Stat(projectRoot); os.IsNotExist(err) {
		parent := filepath.Dir(projectRoot)
		if _, parentErr := os.Stat(parent); os.IsNotExist(parentErr) {
			return fmt.Errorf("parent directory %s does not exist; create it or point --project-root elsewhere", parent)
		}
	}
	return nil
}
