package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	legacyconfig "github.com/otterlab-bio/otter/internal/config"
	"github.com/otterlab-bio/otter/internal/config/legacy"
	"github.com/otterlab-bio/otter/internal/config/resolver"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/input/samples"
	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/otterlab-bio/otter/internal/projectlayout"
	refpkg "github.com/otterlab-bio/otter/internal/reference"
	runstate "github.com/otterlab-bio/otter/internal/run"
	"github.com/spf13/cobra"
)

type configResolveFlags struct {
	ProjectPath      string
	ReferenceRoot    string
	Executor         string
	Backend          string
	Site             string
	PrimaryReference string
	GraftReference   string
	HostReference    string
	RunID            string
	ParentRunID      string
}

type configMigrateFlags struct {
	Input              string
	Output             string
	From               string
	To                 string
	ProjectID          string
	ReferenceRoot      string
	PrimaryReference   string
	SecondaryReference string
	GraftReference     string
	HostReference      string
}

func newConfigResolveCommand() *cobra.Command {
	flags := configResolveFlags{}
	command := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve a canonical project into an immutable run snapshot",
		RunE: func(command *cobra.Command, args []string) error {
			return runConfigResolve(command, flags)
		},
	}
	command.Flags().StringVar(&flags.ProjectPath, "project", "project.yaml", "Canonical project configuration")
	command.Flags().StringVar(&flags.ReferenceRoot, "reference-root", "", "Absolute shared reference registry root; defaults to the selected site profile")
	command.Flags().StringVar(&flags.Executor, "executor", "", "Override executor: craftmake or snakemake")
	command.Flags().StringVar(&flags.Backend, "backend", "", "Resolved backend override: local or slurm")
	command.Flags().StringVar(&flags.Site, "site", "", "Resolved site override")
	command.Flags().StringVar(&flags.PrimaryReference, "reference-primary", "", "Run-level primary reference override as id@release")
	command.Flags().StringVar(&flags.GraftReference, "reference-graft", "", "Run-level graft reference override as id@release")
	command.Flags().StringVar(&flags.HostReference, "reference-host", "", "Run-level host reference override as id@release")
	command.Flags().StringVar(&flags.RunID, "run-id", "", "Explicit run ID matching run-YYYYMMDDTHHMMSSZ-abcdef")
	command.Flags().StringVar(&flags.ParentRunID, "parent-run-id", "", "Optional lineage parent run ID")
	return command
}

func runConfigResolve(command *cobra.Command, flags configResolveFlags) error {
	projectPath, err := filepath.Abs(flags.ProjectPath)
	if err != nil {
		return fmt.Errorf("resolve project path: %w", err)
	}
	projectRoot := filepath.Dir(projectPath)
	var directory runstate.Directory
	if flags.RunID == "" {
		directory, err = runstate.CreateDirectory(projectRoot)
	} else {
		createdAt, parseErr := runstate.CreatedAtFromID(flags.RunID)
		if parseErr != nil {
			return parseErr
		}
		directory, err = runstate.CreateDirectoryWithID(projectRoot, flags.RunID, createdAt)
	}
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(directory.Root)
		}
	}()

	referenceOverrides := make(map[configv1.ReferenceRole]configv1.ReferenceSelection)
	if flags.PrimaryReference != "" {
		referenceOverrides[configv1.ReferenceRolePrimary] = configv1.ReferenceSelection(flags.PrimaryReference)
	}
	if flags.GraftReference != "" {
		referenceOverrides[configv1.ReferenceRoleGraft] = configv1.ReferenceSelection(flags.GraftReference)
	}
	if flags.HostReference != "" {
		referenceOverrides[configv1.ReferenceRoleHost] = configv1.ReferenceSelection(flags.HostReference)
	}
	snapshot, err := (resolver.Resolver{}).Resolve(resolver.Options{
		ProjectPath:       projectPath,
		RunDirectory:      directory,
		ReferenceRoot:     flags.ReferenceRoot,
		ExecutorOverride:  configv1.Executor(flags.Executor),
		BackendOverride:   configv1.Backend(flags.Backend),
		SiteOverride:      flags.Site,
		ReferenceOverride: referenceOverrides,
		ParentRunID:       flags.ParentRunID,
	})
	if err != nil {
		return err
	}
	snapshotPath, err := runstate.WriteSnapshot(directory, snapshot)
	if err != nil {
		return err
	}
	cleanup = false
	fmt.Fprintln(command.OutOrStdout(), snapshotPath)
	return nil
}

func newConfigMigrateCommand() *cobra.Command {
	flags := configMigrateFlags{}
	command := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate a legacy Otter configuration to canonical v1",
		RunE: func(command *cobra.Command, args []string) error {
			return runConfigMigrate(command, flags)
		},
	}
	command.Flags().StringVar(&flags.Input, "input", "otter.yaml", "Legacy configuration file")
	command.Flags().StringVar(&flags.Output, "output", "project.yaml", "Canonical project output file")
	command.Flags().StringVar(&flags.From, "from", "legacy", "Source configuration format")
	command.Flags().StringVar(&flags.To, "to", "v1", "Target configuration format")
	command.Flags().StringVar(&flags.ProjectID, "project-id", "", "Canonical project ID override")
	command.Flags().StringVar(&flags.ReferenceRoot, "reference-root", "", "Reference registry root used to lock every declared reference (default: $OTTER_REFERENCE_ROOT or ~/.otter/references)")
	command.Flags().StringVar(&flags.PrimaryReference, "reference-primary", "", "Primary reference as id@release")
	command.Flags().StringVar(&flags.SecondaryReference, "reference-secondary", "", "Secondary reference as id@release")
	command.Flags().StringVar(&flags.GraftReference, "reference-graft", "", "Graft reference as id@release")
	command.Flags().StringVar(&flags.HostReference, "reference-host", "", "Host reference as id@release")
	return command
}

func runConfigMigrate(command *cobra.Command, flags configMigrateFlags) error {
	if strings.ToLower(flags.From) != "legacy" || strings.ToLower(flags.To) != "v1" {
		return fmt.Errorf("only --from legacy --to v1 is supported")
	}
	legacyLoader := legacyconfig.NewLoader(flags.Input)
	configuration, err := legacyLoader.LoadConfigWithoutEnvironmentOverrides()
	if err != nil {
		return fmt.Errorf("load legacy configuration: %w", err)
	}
	result := legacy.Adapt(*configuration, legacy.MigrationOptions{
		ProjectID:          flags.ProjectID,
		PrimaryReference:   configv1.ReferenceSelection(flags.PrimaryReference),
		SecondaryReference: configv1.ReferenceSelection(flags.SecondaryReference),
		GraftReference:     configv1.ReferenceSelection(flags.GraftReference),
		HostReference:      configv1.ReferenceSelection(flags.HostReference),
	})
	if err := writeMigrationReport(command, result.Report); err != nil {
		return err
	}
	if result.Report.HasConflicts() {
		return fmt.Errorf("legacy migration has %d conflict(s); no canonical files were written", len(result.Report.Conflicts))
	}

	outputPath, err := filepath.Abs(flags.Output)
	if err != nil {
		return fmt.Errorf("resolve migration output path: %w", err)
	}
	outputRoot := filepath.Dir(outputPath)
	inputPath, err := filepath.Abs(flags.Input)
	if err != nil {
		return fmt.Errorf("resolve legacy input path: %w", err)
	}
	inputRoot := filepath.Dir(inputPath)
	for index := range result.Samples {
		result.Samples[index].R1 = resolveLegacySamplePath(inputRoot, result.Samples[index].R1)
		result.Samples[index].R2 = resolveLegacySamplePath(inputRoot, result.Samples[index].R2)
	}
	projectBytes, err := configv1.MarshalProject(result.Project)
	if err != nil {
		return err
	}
	if err := writeExclusiveFile(outputPath, projectBytes); err != nil {
		return err
	}
	samplesPath := filepath.Join(outputRoot, result.Project.Samples.Manifest)
	if err := samples.Write(samplesPath, result.Samples, outputRoot); err != nil {
		_ = os.Remove(outputPath)
		return err
	}

	// A canonical project is only resolvable together with its reference lock,
	// so migration writes one. Without it "otter config resolve" would fail on
	// the migrated project that this command just declared complete.
	lockPath, err := writeMigratedReferenceLock(result.Project, flags.ReferenceRoot, outputRoot)
	if err != nil {
		_ = os.Remove(outputPath)
		_ = os.Remove(samplesPath)
		return fmt.Errorf("write reference lock for migrated project: %w", err)
	}

	fmt.Fprintf(command.OutOrStdout(), "Migrated project: %s\nSamples manifest: %s\nReferences lock: %s\n", outputPath, samplesPath, lockPath)

	// Migration writes project intent, not the pinned asset skeleton. The
	// canonical resolver digests workflows/, rules/, environments/, schemas/,
	// and project.lock.yaml, so a project.yaml dropped into a bare directory
	// cannot be resolved. Say so here rather than letting resolve fail later.
	if detectedTrack, detectErr := projectlayout.Detect(outputRoot); detectErr == nil && detectedTrack != projectlayout.TrackV1 {
		logger.Warnf(
			"%s is not a canonical project root, so the migrated project cannot be resolved yet; run `otter init <project>` and migrate into that project root to get the pinned workflow assets",
			outputRoot,
		)
	}
	return nil
}

// writeMigratedReferenceLock resolves every declared reference against the
// registry and writes references.lock.yaml into the migrated project root.
func writeMigratedReferenceLock(project configv1.ProjectConfig, registryRoot, outputRoot string) (string, error) {
	if strings.TrimSpace(registryRoot) == "" {
		defaultRoot, err := refpkg.DefaultRegistryRoot()
		if err != nil {
			return "", err
		}
		registryRoot = defaultRoot
	}
	referenceFlags := make(map[configv1.ReferenceRole]string)
	if project.References.Primary != "" {
		referenceFlags[configv1.ReferenceRolePrimary] = string(project.References.Primary)
	}
	if project.References.Secondary != "" {
		referenceFlags[configv1.ReferenceRoleSecondary] = string(project.References.Secondary)
	}
	for _, speciesReference := range project.References.Species {
		referenceFlags[speciesReference.Role] = string(speciesReference.Selection)
	}

	_, referenceLock, err := resolveCanonicalReferences(canonicalCreateRequest{
		ProjectRoot:    outputRoot,
		Scenario:       project.Workflow.Scenario,
		ReferenceRoot:  registryRoot,
		ReferenceFlags: referenceFlags,
	})
	if err != nil {
		return "", err
	}
	lockBytes, err := marshalLock(referenceLock)
	if err != nil {
		return "", err
	}
	lockPath := filepath.Join(outputRoot, "references.lock.yaml")
	if err := writeExclusiveFile(lockPath, lockBytes); err != nil {
		return "", err
	}
	return lockPath, nil
}

func writeMigrationReport(command *cobra.Command, report legacy.MigrationReport) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal migration report: %w", err)
	}
	fmt.Fprintln(command.ErrOrStderr(), string(encoded))
	return nil
}

func resolveLegacySamplePath(inputRoot string, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(inputRoot, path)
}

func writeExclusiveFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("sync %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}
