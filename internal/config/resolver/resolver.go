package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/input/samples"
	"github.com/otterlab-bio/otter/internal/reference"
	runstate "github.com/otterlab-bio/otter/internal/run"
	"github.com/otterlab-bio/otter/internal/site"
)

type Options struct {
	ProjectPath                  string
	RunDirectory                 runstate.Directory
	ReferenceRoot                string
	ExecutorOverride             configv1.Executor
	BackendOverride              configv1.Backend
	SiteOverride                 string
	ReferenceOverride            map[configv1.ReferenceRole]configv1.ReferenceSelection
	ParentRunID                  string
	Detector                     *site.Detector
	ComputePathValidator         func(path, partition, account string) error
	ComputeWritablePathValidator func(path, partition, account string) error
}

type Resolver struct{}

type backendResolution struct {
	Backend        configv1.Backend
	BackendSource  configv1.ValueSource
	Evidence       configv1.BackendEvidence
	SiteID         string
	SiteSource     configv1.ValueSource
	SitePaths      site.SitePaths
	SiteResources  configv1.ProjectResources
	SlurmResources configv1.ResolvedSlurmResources
}

func (Resolver) Resolve(options Options) (configv1.RunSnapshot, error) {
	projectPath, projectRoot, err := resolveProjectPath(options.ProjectPath)
	if err != nil {
		return configv1.RunSnapshot{}, err
	}
	project, sources, err := configv1.LoadProjectWithSources(projectPath)
	if err != nil {
		return configv1.RunSnapshot{}, err
	}
	if options.RunDirectory.ID == "" || options.RunDirectory.Root == "" || options.RunDirectory.CreatedAt.IsZero() {
		return configv1.RunSnapshot{}, fmt.Errorf("allocated run directory is required")
	}
	if filepath.Dir(filepath.Dir(options.RunDirectory.Root)) != projectRoot {
		return configv1.RunSnapshot{}, fmt.Errorf("run directory must belong to project root")
	}

	samplesPath := resolveProjectRelative(projectRoot, project.Samples.Manifest)
	sampleRecords, err := samples.Load(samplesPath, projectRoot)
	if err != nil {
		return configv1.RunSnapshot{}, err
	}
	sampleRecords, err = runstate.EnrichSampleInputMetadata(sampleRecords)
	if err != nil {
		return configv1.RunSnapshot{}, err
	}
	lockPath := filepath.Join(projectRoot, "references.lock.yaml")
	referenceLock, err := reference.LoadLock(lockPath)
	if err != nil {
		return configv1.RunSnapshot{}, err
	}
	executor, executorSource := project.Execution.Executor, sources.Executor
	if options.ExecutorOverride != "" {
		if options.ExecutorOverride != configv1.ExecutorCraftmake && options.ExecutorOverride != configv1.ExecutorSnakemake {
			return configv1.RunSnapshot{}, fmt.Errorf("executor override %q is invalid", options.ExecutorOverride)
		}
		executor, executorSource = options.ExecutorOverride, configv1.SourceCLI
	}

	detector := options.Detector
	if detector == nil {
		detector = site.NewDetector()
	}
	backendResolution, err := resolveBackendAndSite(
		project.Execution.Backend, sources.Backend,
		project.Execution.Site, sources.Site,
		options.BackendOverride,
		options.SiteOverride,
		detector,
	)
	if err != nil {
		return configv1.RunSnapshot{}, err
	}
	referenceRoot := strings.TrimSpace(options.ReferenceRoot)
	if referenceRoot == "" {
		referenceRoot = strings.TrimSpace(backendResolution.SitePaths.ReferenceRoot)
	}
	if !filepath.IsAbs(referenceRoot) {
		return configv1.RunSnapshot{}, fmt.Errorf("reference root must be an absolute path or supplied by the selected site profile")
	}
	if err := validateSlurmComputePaths(
		options,
		backendResolution,
		referenceRoot,
		options.RunDirectory.Root,
	); err != nil {
		return configv1.RunSnapshot{}, err
	}
	resolvedResources := mergeResources(project.Resources, backendResolution.SiteResources)

	projectSelections, roles := selectionsFromProject(project.References)
	effectiveSelections := projectSelections
	overriddenRoles := make(map[configv1.ReferenceRole]bool, len(options.ReferenceOverride))
	for role, selection := range options.ReferenceOverride {
		if _, _, err := configv1.ParseReferenceSelection(selection); err != nil {
			return configv1.RunSnapshot{}, err
		}
		if err := setSelection(&effectiveSelections, role, selection); err != nil {
			return configv1.RunSnapshot{}, err
		}
		if !containsRole(roles, role) {
			return configv1.RunSnapshot{}, fmt.Errorf("cannot override reference role %q because it is not present in the project", role)
		}
		overriddenRoles[role] = true
	}
	referenceResolver := reference.Resolver{RegistryRoot: referenceRoot, Lock: referenceLock}
	resolvedReferences := make([]configv1.ResolvedReference, 0, len(roles))
	for _, role := range roles {
		selection := selectionForRole(effectiveSelections, role)
		if selection == "" {
			return configv1.RunSnapshot{}, fmt.Errorf("effective reference selection for role %q is empty", role)
		}
		var resolved configv1.ResolvedReference
		if overriddenRoles[role] {
			resolved, err = referenceResolver.ResolveOverride(role, selection, project.Workflow.Scenario)
		} else {
			resolved, err = referenceResolver.Resolve(role, selection, project.Workflow.Scenario)
		}
		if err != nil {
			return configv1.RunSnapshot{}, err
		}
		resolvedReferences = append(resolvedReferences, resolved)
	}

	projectDigest, err := runstate.DigestFile(projectPath)
	if err != nil {
		return configv1.RunSnapshot{}, err
	}
	samplesDigest, err := runstate.DigestFile(samplesPath)
	if err != nil {
		return configv1.RunSnapshot{}, err
	}
	workflowAssets := []string{
		filepath.Join(projectRoot, "workflows"),
		filepath.Join(projectRoot, "rules"),
		filepath.Join(projectRoot, "environments"),
		filepath.Join(projectRoot, "schemas"),
		filepath.Join(projectRoot, "project.lock.yaml"),
	}
	for _, rootWorkflowAsset := range []string{"snakefile"} {
		rootWorkflowAssetPath := filepath.Join(projectRoot, rootWorkflowAsset)
		if info, statErr := os.Stat(rootWorkflowAssetPath); statErr == nil && !info.IsDir() {
			workflowAssets = append(workflowAssets, rootWorkflowAssetPath)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return configv1.RunSnapshot{}, fmt.Errorf("inspect workflow asset %q: %w", rootWorkflowAssetPath, statErr)
		}
	}
	rootSnakefiles, err := filepath.Glob(filepath.Join(projectRoot, "*.snakemake"))
	if err != nil {
		return configv1.RunSnapshot{}, fmt.Errorf("discover root Snakemake assets: %w", err)
	}
	for _, rootSnakefilePath := range rootSnakefiles {
		fileInfo, statErr := os.Stat(rootSnakefilePath)
		if statErr != nil {
			return configv1.RunSnapshot{}, fmt.Errorf("inspect root Snakemake asset %q: %w", rootSnakefilePath, statErr)
		}
		if !fileInfo.Mode().IsRegular() {
			continue
		}
		workflowAssets = append(workflowAssets, rootSnakefilePath)
	}
	workflowDigest, err := runstate.DigestPaths(workflowAssets)
	if err != nil {
		return configv1.RunSnapshot{}, err
	}
	referencesDigest, err := runstate.DigestFile(lockPath)
	if err != nil {
		return configv1.RunSnapshot{}, err
	}

	override := len(overriddenRoles) > 0
	overrideSource := configv1.OverrideSourceNone
	if override {
		overrideSource = configv1.OverrideSourceCLI
	}
	runRoot := options.RunDirectory.Root
	parity := configv1.ParityConfig{}
	if backendResolution.Backend == configv1.BackendSlurm && len(resolvedResources.Phases) > 0 {
		parity.Policy = configv1.ParityPolicyExecutorPhaseEnvelope
	}
	snapshot := configv1.RunSnapshot{
		SchemaVersion: configv1.RunSchemaVersion,
		Run: configv1.RunMetadata{
			ID:          options.RunDirectory.ID,
			CreatedAt:   configv1.Timestamp(options.RunDirectory.CreatedAt.UTC().Truncate(time.Second).Format(time.RFC3339)),
			Immutable:   true,
			ParentRunID: options.ParentRunID,
		},
		Project: configv1.ResolvedProject{ID: project.Project.ID, Root: projectRoot},
		Workflow: configv1.ResolvedWorkflow{
			Scenario:         project.Workflow.Scenario,
			Toolchain:        project.Workflow.Toolchain,
			LegacyExtensions: append([]string(nil), project.Workflow.LegacyExtensions...),
			AssetRoot:        filepath.Join(projectRoot, "workflows"),
		},
		Execution: configv1.ResolvedExecution{
			Executor: configv1.ResolvedExecutor{Value: executor, Source: executorSource},
			Backend: configv1.ResolvedBackend{
				Value:    backendResolution.Backend,
				Source:   backendResolution.BackendSource,
				Evidence: backendResolution.Evidence,
			},
			Site:      configv1.ResolvedString{Value: backendResolution.SiteID, Source: backendResolution.SiteSource},
			Resources: resolvedResources,
			Slurm:     backendResolution.SlurmResources,
		},
		Samples: sampleRecords,
		References: configv1.ResolvedReferences{
			ProjectSelection:   projectSelections,
			EffectiveSelection: effectiveSelections,
			Override:           override,
			OverrideSource:     overrideSource,
			Resolved:           resolvedReferences,
		},
		Paths: configv1.RunPaths{
			RunRoot:         runRoot,
			Work:            filepath.Join(runRoot, "work"),
			Results:         filepath.Join(runRoot, "results"),
			Logs:            filepath.Join(runRoot, "logs"),
			State:           filepath.Join(runRoot, "state"),
			Metrics:         filepath.Join(runRoot, "metrics"),
			ProjectConfig:   projectPath,
			SamplesManifest: samplesPath,
			ReferencesLock:  lockPath,
			WorkflowAssets:  append([]string(nil), workflowAssets...),
		},
		Digests: configv1.RunDigests{
			Project:        projectDigest,
			Samples:        samplesDigest,
			WorkflowAssets: workflowDigest,
			References:     referencesDigest,
		},
		Observability: project.Observability,
		Parity:        parity,
	}
	if err := configv1.ValidateRunSnapshot(snapshot); err != nil {
		return configv1.RunSnapshot{}, err
	}
	return snapshot, nil
}

func validateSlurmComputePaths(
	options Options,
	resolution backendResolution,
	referenceRoot string,
	runRoot string,
) error {
	if resolution.Backend != configv1.BackendSlurm {
		return nil
	}
	partition := resolution.SlurmResources.Partition.Value
	account := resolution.SlurmResources.Account.Value
	if err := site.CheckLoginNodePath(referenceRoot); err != nil {
		return fmt.Errorf("effective reference root %q is not accessible on the login node: %w", referenceRoot, err)
	}
	computePathValidator := options.ComputePathValidator
	if computePathValidator != nil {
		if err := computePathValidator(referenceRoot, partition, account); err != nil {
			return fmt.Errorf("effective reference root %q failed the requested compute-node preflight: %w", referenceRoot, err)
		}
	}
	if err := site.CheckLoginNodeWritableDirectory(runRoot); err != nil {
		return fmt.Errorf("run output root %q is not writable on the login node: %w", runRoot, err)
	}
	computeWritablePathValidator := options.ComputeWritablePathValidator
	if computeWritablePathValidator != nil {
		if err := computeWritablePathValidator(runRoot, partition, account); err != nil {
			return fmt.Errorf("run output root %q failed the requested compute-node preflight: %w", runRoot, err)
		}
	}
	return nil
}

func resolveBackendAndSite(
	projectBackend configv1.Backend,
	projectBackendSource configv1.ValueSource,
	projectSite string,
	projectSiteSource configv1.ValueSource,
	backendOverride configv1.Backend,
	siteOverride string,
	detector *site.Detector,
) (backendResolution, error) {
	backend, backendSource := projectBackend, projectBackendSource
	if backendOverride != "" {
		if backendOverride != configv1.BackendLocal && backendOverride != configv1.BackendSlurm {
			return backendResolution{}, fmt.Errorf("backend override must be local or slurm")
		}
		backend, backendSource = backendOverride, configv1.SourceCLI
	}

	siteID, siteSource := projectSite, projectSiteSource
	if siteOverride != "" {
		siteID, siteSource = siteOverride, configv1.SourceCLI
	}

	locator := site.DefaultLocator()
	if backend == configv1.BackendAuto {
		result, err := detector.Detect(locator, siteID)
		if err != nil {
			return backendResolution{}, fmt.Errorf("backend auto-detection failed: %w", err)
		}
		return backendResolution{
			Backend:        result.Backend,
			BackendSource:  configv1.SourceDetection,
			Evidence:       result.Evidence,
			SiteID:         result.SiteID,
			SiteSource:     result.Source,
			SitePaths:      result.SitePaths,
			SiteResources:  result.SiteResources,
			SlurmResources: result.SlurmResources,
		}, nil
	}

	result, err := detector.Validate(backend, locator, siteID)
	if err != nil {
		return backendResolution{}, fmt.Errorf("backend validation failed: %w", err)
	}
	if siteID == "auto" {
		siteID = result.SiteID
		siteSource = result.Source
	}
	return backendResolution{
		Backend:        backend,
		BackendSource:  backendSource,
		Evidence:       result.Evidence,
		SiteID:         siteID,
		SiteSource:     siteSource,
		SitePaths:      result.SitePaths,
		SiteResources:  result.SiteResources,
		SlurmResources: result.SlurmResources,
	}, nil
}

func mergeResources(projectResources configv1.ProjectResources, siteResources configv1.ProjectResources) configv1.ProjectResources {
	resolved := configv1.ProjectResources{
		Defaults: projectResources.Defaults,
		Phases:   make(map[string]configv1.ResourceSpec, len(projectResources.Phases)+len(siteResources.Phases)),
	}
	for phase, resourceSpec := range projectResources.Phases {
		resolved.Phases[phase] = resourceSpec
	}
	resolved.Defaults = mergeResourceSpec(resolved.Defaults, siteResources.Defaults)
	for phase, siteResourceSpec := range siteResources.Phases {
		resolved.Phases[phase] = mergeResourceSpec(resolved.Phases[phase], siteResourceSpec)
	}
	if len(resolved.Phases) == 0 {
		resolved.Phases = nil
	}
	return resolved
}

func mergeResourceSpec(projectResource configv1.ResourceSpec, siteResource configv1.ResourceSpec) configv1.ResourceSpec {
	resolved := projectResource
	if siteResource.Cores != 0 {
		resolved.Cores = siteResource.Cores
	}
	if siteResource.Memory != "" {
		resolved.Memory = siteResource.Memory
	}
	if siteResource.Time != "" {
		resolved.Time = siteResource.Time
	}
	if siteResource.Partition != "" {
		resolved.Partition = siteResource.Partition
	}
	return resolved
}

func resolveProjectPath(path string) (string, string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve project path: %w", err)
	}
	return filepath.Clean(absolutePath), filepath.Dir(filepath.Clean(absolutePath)), nil
}

func resolveProjectRelative(projectRoot string, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(projectRoot, path)
}

func selectionsFromProject(references configv1.ProjectReferences) (configv1.ReferenceSelections, []configv1.ReferenceRole) {
	selections := configv1.ReferenceSelections{}
	var roles []configv1.ReferenceRole
	if references.Primary != "" {
		selections.Primary = references.Primary
		roles = append(roles, configv1.ReferenceRolePrimary)
	}
	if references.Secondary != "" {
		selections.Secondary = references.Secondary
		roles = append(roles, configv1.ReferenceRoleSecondary)
	}
	for _, species := range references.Species {
		_ = setSelection(&selections, species.Role, species.Selection)
		roles = append(roles, species.Role)
	}
	return selections, roles
}

func setSelection(selections *configv1.ReferenceSelections, role configv1.ReferenceRole, selection configv1.ReferenceSelection) error {
	switch role {
	case configv1.ReferenceRolePrimary:
		selections.Primary = selection
	case configv1.ReferenceRoleSecondary:
		selections.Secondary = selection
	case configv1.ReferenceRoleGraft:
		selections.Graft = selection
	case configv1.ReferenceRoleHost:
		selections.Host = selection
	default:
		return fmt.Errorf("reference role %q is invalid", role)
	}
	return nil
}

func selectionForRole(selections configv1.ReferenceSelections, role configv1.ReferenceRole) configv1.ReferenceSelection {
	switch role {
	case configv1.ReferenceRolePrimary:
		return selections.Primary
	case configv1.ReferenceRoleSecondary:
		return selections.Secondary
	case configv1.ReferenceRoleGraft:
		return selections.Graft
	case configv1.ReferenceRoleHost:
		return selections.Host
	default:
		return ""
	}
}

func containsRole(roles []configv1.ReferenceRole, expected configv1.ReferenceRole) bool {
	for _, role := range roles {
		if role == expected {
			return true
		}
	}
	return false
}
