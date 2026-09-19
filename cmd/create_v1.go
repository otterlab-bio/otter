package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/input"
	"github.com/otterlab-bio/otter/internal/input/samples"
	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/otterlab-bio/otter/internal/projectlayout"
	refpkg "github.com/otterlab-bio/otter/internal/reference"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// canonicalCreateRequest carries the resolved inputs for a canonical v1 project
// authoring pass.
type canonicalCreateRequest struct {
	ProjectRoot    string
	ProjectID      string
	Scenario       configv1.Scenario
	ReferenceRoot  string
	ReferenceFlags map[configv1.ReferenceRole]string
	PairedSamples  []input.PairedSample
	PData          *input.PData
	Adapter1       []string
	Adapter2       []string
}

// createScenarioForMode maps the workflow mode onto a canonical scenario.
//
// WGBS combined with a PDX reference selection is rejected rather than guessed:
// there is no WGBS-PDX scenario, and silently promoting it to BS-PDX would
// change the meaning of the run.
func createScenarioForMode(mode string, pdx bool) (configv1.Scenario, error) {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "RRBS":
		if pdx {
			return configv1.ScenarioBSPDX, nil
		}
		return configv1.ScenarioRRBS, nil
	case "WGBS":
		if pdx {
			return "", fmt.Errorf("WGBS has no PDX scenario; use --mode RRBS with --reference-graft/--reference-host for BS-PDX, or drop the second species")
		}
		return configv1.ScenarioWGBS, nil
	case "RNASEQ":
		if pdx {
			return configv1.ScenarioRNAPDX, nil
		}
		return configv1.ScenarioRNASeq, nil
	case "BSSEQ":
		return "", fmt.Errorf("BSSEQ does not distinguish RRBS from WGBS; specify --mode RRBS or --mode WGBS")
	default:
		return "", fmt.Errorf("unsupported mode %q; expected RRBS, WGBS, or RNASEQ", mode)
	}
}

// requiredReferenceRoles lists the roles a scenario must declare.
func requiredReferenceRoles(scenario configv1.Scenario) []configv1.ReferenceRole {
	if scenario == configv1.ScenarioBSPDX || scenario == configv1.ScenarioRNAPDX {
		return []configv1.ReferenceRole{configv1.ReferenceRoleGraft, configv1.ReferenceRoleHost}
	}
	return []configv1.ReferenceRole{configv1.ReferenceRolePrimary}
}

// referenceFlagName maps a role onto the flag a user supplies for it.
func referenceFlagName(role configv1.ReferenceRole) string {
	return "--reference-" + string(role)
}

// resolveCanonicalReferences validates every declared reference against the
// registry and returns both the project declaration and the lock entries.
//
// Validation runs the production resolver path (identity, manifest digest,
// scenario compatibility, declared asset presence) so a stub or real release is
// held to the same contract a later "otter config resolve" will enforce.
func resolveCanonicalReferences(request canonicalCreateRequest) (configv1.ProjectReferences, configv1.ReferencesLock, error) {
	registryRoot, err := filepath.Abs(request.ReferenceRoot)
	if err != nil {
		return configv1.ProjectReferences{}, configv1.ReferencesLock{}, fmt.Errorf("resolve reference root %q: %w", request.ReferenceRoot, err)
	}
	if info, statErr := os.Stat(registryRoot); statErr != nil {
		return configv1.ProjectReferences{}, configv1.ReferencesLock{}, fmt.Errorf("reference registry root %q is not readable: %w", registryRoot, statErr)
	} else if !info.IsDir() {
		return configv1.ProjectReferences{}, configv1.ReferencesLock{}, fmt.Errorf("reference registry root %q is not a directory", registryRoot)
	}

	requiredRoles := requiredReferenceRoles(request.Scenario)
	resolver := refpkg.Resolver{RegistryRoot: registryRoot}
	selections := make(map[configv1.ReferenceRole]configv1.ReferenceSelection, len(requiredRoles))
	lockEntries := make(map[string]configv1.LockedReference, len(requiredRoles))

	for _, role := range requiredRoles {
		rawSelection := strings.TrimSpace(request.ReferenceFlags[role])
		if rawSelection == "" {
			return configv1.ProjectReferences{}, configv1.ReferencesLock{}, fmt.Errorf(
				"scenario %q requires %s as id@release", request.Scenario, referenceFlagName(role))
		}
		selection := configv1.ReferenceSelection(rawSelection)
		referenceID, release, parseErr := configv1.ParseReferenceSelection(selection)
		if parseErr != nil {
			return configv1.ProjectReferences{}, configv1.ReferencesLock{}, fmt.Errorf("%s: %w", referenceFlagName(role), parseErr)
		}
		resolvedReference, resolveErr := resolver.ResolveOverride(role, selection, request.Scenario)
		if resolveErr != nil {
			return configv1.ProjectReferences{}, configv1.ReferencesLock{}, fmt.Errorf(
				"%s %s: %w", referenceFlagName(role), selection, resolveErr)
		}
		logger.Infof("Reference %s verified: %s@%s (%s, manifest %s)",
			role, referenceID, release, resolvedReference.Organism, resolvedReference.ManifestDigest)
		selections[role] = selection
		lockEntries[string(role)] = configv1.LockedReference{
			ID:             referenceID,
			Release:        release,
			ManifestDigest: resolvedReference.ManifestDigest,
		}
	}

	projectReferences := configv1.ProjectReferences{}
	if request.Scenario == configv1.ScenarioBSPDX || request.Scenario == configv1.ScenarioRNAPDX {
		projectReferences.Species = []configv1.SpeciesReference{
			{Role: configv1.ReferenceRoleGraft, Selection: selections[configv1.ReferenceRoleGraft]},
			{Role: configv1.ReferenceRoleHost, Selection: selections[configv1.ReferenceRoleHost]},
		}
	} else {
		projectReferences.Primary = selections[configv1.ReferenceRolePrimary]
	}

	return projectReferences, configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References:    lockEntries,
	}, nil
}

// buildCanonicalSampleRecords converts the scanned pairs into canonical sample
// records, carrying pdata group and generated adapters where available.
//
// Input paths are made absolute before they are recorded. "otter create" reads
// --fastq relative to the working directory, but every later reader anchors a
// relative path in samples.tsv to the project root, so recording the path
// verbatim would point resolve at a file that does not exist.
func buildCanonicalSampleRecords(request canonicalCreateRequest) ([]configv1.SampleRecord, error) {
	records := make([]configv1.SampleRecord, 0, len(request.PairedSamples))
	for index, paired := range request.PairedSamples {
		read1, err := filepath.Abs(paired.R1Path)
		if err != nil {
			return nil, fmt.Errorf("resolve R1 path %q: %w", paired.R1Path, err)
		}
		read2, err := filepath.Abs(paired.R2Path)
		if err != nil {
			return nil, fmt.Errorf("resolve R2 path %q: %w", paired.R2Path, err)
		}
		record := configv1.SampleRecord{
			ID:    paired.Name,
			R1:    read1,
			R2:    read2,
			Group: sampleGroupFromPData(request.PData, paired.Name),
		}
		if index < len(request.Adapter1) {
			record.AdapterR1 = normalizedAdapter(request.Adapter1[index])
		}
		if index < len(request.Adapter2) {
			record.AdapterR2 = normalizedAdapter(request.Adapter2[index])
		}
		records = append(records, record)
	}
	return records, nil
}

// normalizedAdapter drops the generator's "no adapter" sentinel so samples.tsv
// carries an empty cell instead of a placeholder token.
func normalizedAdapter(adapter string) string {
	trimmed := strings.TrimSpace(adapter)
	if strings.EqualFold(trimmed, "NO_ADAPTER_CAL_USE_DEFAULT") {
		return ""
	}
	return trimmed
}

// sampleGroupFromPData prefers the explicit group column, then the condition
// column.
func sampleGroupFromPData(pdata *input.PData, sampleID string) string {
	if pdata == nil || pdata.Data == nil {
		return ""
	}
	sampleData, exists := pdata.Data[sampleID]
	if !exists {
		return ""
	}
	for _, candidateColumn := range []string{"sample_group", "group", "condition"} {
		if value := strings.TrimSpace(sampleData[candidateColumn]); value != "" {
			return value
		}
	}
	return ""
}

// writeCanonicalProject writes project.yaml, samples.tsv, and
// references.lock.yaml for a canonical v1 project.
func writeCanonicalProject(request canonicalCreateRequest, projectReferences configv1.ProjectReferences, referenceLock configv1.ReferencesLock) (string, string, string, error) {
	project := configv1.ProjectConfig{
		SchemaVersion: configv1.ProjectSchemaVersion,
		Project:       configv1.ProjectMetadata{ID: request.ProjectID},
		Workflow: configv1.ProjectWorkflow{
			Scenario:  request.Scenario,
			Toolchain: configv1.ToolchainModern,
		},
		Execution: configv1.ProjectExecution{
			Executor: configv1.ExecutorCraftmake,
			Backend:  configv1.BackendAuto,
			Site:     "auto",
		},
		Samples:       configv1.SamplesDeclaration{Manifest: "samples.tsv"},
		References:    projectReferences,
		Observability: configv1.ObservabilityConfig{Metrics: true, RetainLogs: true},
	}
	if err := configv1.ValidateProject(project); err != nil {
		return "", "", "", fmt.Errorf("generated project configuration is invalid: %w", err)
	}
	projectBytes, err := configv1.MarshalProject(project)
	if err != nil {
		return "", "", "", err
	}

	projectPath := filepath.Join(request.ProjectRoot, "project.yaml")
	samplesPath := filepath.Join(request.ProjectRoot, "samples.tsv")
	lockPath := filepath.Join(request.ProjectRoot, "references.lock.yaml")
	for _, target := range []string{projectPath, samplesPath, lockPath} {
		if err := requireAbsent(target); err != nil {
			return "", "", "", err
		}
	}

	if err := os.WriteFile(projectPath, projectBytes, 0o644); err != nil {
		return "", "", "", fmt.Errorf("write project configuration %s: %w", projectPath, err)
	}

	records, err := buildCanonicalSampleRecords(request)
	if err != nil {
		_ = os.Remove(projectPath)
		return "", "", "", err
	}
	if err := samples.Write(samplesPath, records, request.ProjectRoot); err != nil {
		_ = os.Remove(projectPath)
		return "", "", "", fmt.Errorf("write samples manifest: %w", err)
	}

	if err := configv1.ValidateReferencesLock(referenceLock); err != nil {
		_ = os.Remove(projectPath)
		_ = os.Remove(samplesPath)
		return "", "", "", fmt.Errorf("generated reference lock is invalid: %w", err)
	}
	lockBytes, err := marshalLock(referenceLock)
	if err != nil {
		_ = os.Remove(projectPath)
		_ = os.Remove(samplesPath)
		return "", "", "", err
	}
	if err := os.WriteFile(lockPath, lockBytes, 0o644); err != nil {
		_ = os.Remove(projectPath)
		_ = os.Remove(samplesPath)
		return "", "", "", fmt.Errorf("write reference lock %s: %w", lockPath, err)
	}

	return projectPath, samplesPath, lockPath, nil
}

func marshalLock(lock configv1.ReferencesLock) ([]byte, error) {
	encoded, err := yaml.Marshal(lock)
	if err != nil {
		return nil, fmt.Errorf("marshal reference lock: %w", err)
	}
	return encoded, nil
}

// requireAbsent refuses to overwrite an existing authoring artifact so a rerun
// cannot silently drop a previously locked reference or sample set.
func requireAbsent(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; delete it explicitly or use a new project directory", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	return nil
}

// runCreateCanonicalTrack is the default "otter create" behaviour: it writes a
// canonical v1 project that "otter config resolve" can consume immediately.
func runCreateCanonicalTrack(
	projectRoot string,
	projectID string,
	scenario configv1.Scenario,
	referenceRoot string,
	referenceFlags map[configv1.ReferenceRole]string,
	pairedSamples []input.PairedSample,
	pdata *input.PData,
	adapter1, adapter2 []string,
) error {
	absoluteRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root %q: %w", projectRoot, err)
	}
	if err := projectlayout.RequireTrack(absoluteRoot, projectlayout.TrackV1); err != nil {
		return err
	}
	absoluteRegistryRoot, err := filepath.Abs(referenceRoot)
	if err != nil {
		return fmt.Errorf("resolve reference root %q: %w", referenceRoot, err)
	}

	request := canonicalCreateRequest{
		ProjectRoot:    absoluteRoot,
		ProjectID:      projectID,
		Scenario:       scenario,
		ReferenceRoot:  referenceRoot,
		ReferenceFlags: referenceFlags,
		PairedSamples:  pairedSamples,
		PData:          pdata,
		Adapter1:       adapter1,
		Adapter2:       adapter2,
	}

	logger.Info("Verifying declared references against the registry...")
	projectReferences, referenceLock, err := resolveCanonicalReferences(request)
	if err != nil {
		return err
	}

	projectPath, samplesPath, lockPath, err := writeCanonicalProject(request, projectReferences, referenceLock)
	if err != nil {
		return err
	}

	logger.Info("===========================================")
	logger.Info("Canonical project created successfully!")
	logger.Info("===========================================")
	logger.Infof("  Project ID: %s", projectID)
	logger.Infof("  Scenario:   %s", scenario)
	logger.Infof("  Samples:    %d paired samples", len(pairedSamples))
	logger.Infof("  Project:    %s", projectPath)
	logger.Infof("  Samples:    %s", samplesPath)
	logger.Infof("  References: %s", lockPath)
	logger.Info("")
	logger.Info("Next steps:")
	logger.Infof("  otter config validate --config %s --schema v1", projectPath)
	// resolve needs the registry root unless a site profile supplies one, so
	// print the value this create already validated rather than a command that
	// fails for want of a flag.
	logger.Infof("  otter config resolve --project %s --reference-root %s --backend local", projectPath, absoluteRegistryRoot)
	logger.Info("  otter run --config <printed run.yaml> --executor craftmake --phase step1 --backend local")
	return nil
}

// canonicalCreateFlags holds the reference flags a canonical create accepts.
type canonicalCreateFlags struct {
	ReferenceRoot string
	Primary       string
	Graft         string
	Host          string
}

// referenceRoleValues exposes the flags as a role map for the shared resolver.
func (flags canonicalCreateFlags) referenceRoleValues() map[configv1.ReferenceRole]string {
	return map[configv1.ReferenceRole]string{
		configv1.ReferenceRolePrimary: flags.Primary,
		configv1.ReferenceRoleGraft:   flags.Graft,
		configv1.ReferenceRoleHost:    flags.Host,
	}
}

// validateCompleteness rejects a partially declared reference set with a message
// that explains what the flags implied. A lone --reference-graft silently
// implies PDX, which is surprising, so it is named explicitly.
func (flags canonicalCreateFlags) validateCompleteness(scenario configv1.Scenario) error {
	if scenario != configv1.ScenarioBSPDX && scenario != configv1.ScenarioRNAPDX {
		return nil
	}
	hasGraft := strings.TrimSpace(flags.Graft) != ""
	hasHost := strings.TrimSpace(flags.Host) != ""
	switch {
	case hasGraft && hasHost:
		return nil
	case hasGraft:
		return fmt.Errorf("--reference-graft selects a PDX scenario, which also requires --reference-host as id@release")
	case hasHost:
		return fmt.Errorf("--reference-host selects a PDX scenario, which also requires --reference-graft as id@release")
	default:
		return fmt.Errorf("PDX scenarios require both --reference-graft and --reference-host as id@release")
	}
}

// readCanonicalCreateFlags reads the canonical reference flags, defaulting the
// registry root to the site-independent default when the user omits it.
func readCanonicalCreateFlags(command *cobra.Command) (canonicalCreateFlags, error) {
	flags := canonicalCreateFlags{}
	var err error
	if flags.ReferenceRoot, err = command.Flags().GetString("reference-root"); err != nil {
		return flags, err
	}
	if flags.Primary, err = command.Flags().GetString("reference-primary"); err != nil {
		return flags, err
	}
	if flags.Graft, err = command.Flags().GetString("reference-graft"); err != nil {
		return flags, err
	}
	if flags.Host, err = command.Flags().GetString("reference-host"); err != nil {
		return flags, err
	}
	if strings.TrimSpace(flags.ReferenceRoot) == "" {
		defaultRoot, defaultErr := refpkg.DefaultRegistryRoot()
		if defaultErr != nil {
			return flags, defaultErr
		}
		flags.ReferenceRoot = defaultRoot
	}
	return flags, nil
}

// validateForScenario rejects a primary flag on a PDX scenario and vice versa,
// so a mismatched flag cannot silently produce a half-locked project.
func (flags canonicalCreateFlags) validateForScenario(scenario configv1.Scenario) error {
	if scenario == configv1.ScenarioBSPDX || scenario == configv1.ScenarioRNAPDX {
		if strings.TrimSpace(flags.Primary) != "" {
			return fmt.Errorf("scenario %q uses --reference-graft and --reference-host; --reference-primary does not apply", scenario)
		}
		return nil
	}
	if strings.TrimSpace(flags.Graft) != "" || strings.TrimSpace(flags.Host) != "" {
		return fmt.Errorf(
			"scenario %q uses --reference-primary; --reference-graft/--reference-host only apply to PDX scenarios", scenario)
	}
	return nil
}

// defaultCanonicalProjectID derives a project id from the project directory
// name when --jobid is omitted.
func defaultCanonicalProjectID(projectRoot string) string {
	absoluteRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "otter-project"
	}
	base := filepath.Base(absoluteRoot)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "otter-project"
	}
	return base
}

// registerCanonicalCreateFlags registers the canonical authoring flags on the
// create command.
func registerCanonicalCreateFlags(command *cobra.Command) {
	command.Flags().String("reference-root", "", "Reference registry root for canonical projects (default: $OTTER_REFERENCE_ROOT or ~/.otter/references)")
	command.Flags().String("reference-primary", "", "Primary reference as id@release for RRBS/WGBS/RNA-seq")
	command.Flags().String("reference-graft", "", "Graft (primary) reference as id@release for BS-PDX/RNA-PDX")
	command.Flags().String("reference-host", "", "Host (secondary) reference as id@release for BS-PDX/RNA-PDX")
}
