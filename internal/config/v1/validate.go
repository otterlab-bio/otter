package v1

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	projectIDPattern          = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9._-]{0,63}\\z")
	referenceSelectionPattern = regexp.MustCompile("^[A-Za-z0-9._-]+@[A-Za-z0-9._-]+\\z")
	digestPattern             = regexp.MustCompile("^sha256:[a-f0-9]{64}\\z")
	runIDPattern              = regexp.MustCompile("^run-[0-9]{8}T[0-9]{6}Z-[a-z]{6}\\z")
	memoryPattern             = regexp.MustCompile("^[1-9][0-9]*(MiB|GiB)\\z")
	timePattern               = regexp.MustCompile("^(?:[0-9]+-)?[0-9]{1,2}:[0-5][0-9]:[0-5][0-9]\\z")
)

func ApplyProjectDefaults(project *ProjectConfig) {
	if project.Workflow.Toolchain == "" {
		project.Workflow.Toolchain = ToolchainModern
	}
	if project.Execution.Executor == "" {
		project.Execution.Executor = ExecutorCraftmake
	}
	if project.Execution.Backend == "" {
		project.Execution.Backend = BackendAuto
	}
	if project.Execution.Site == "" {
		project.Execution.Site = "auto"
	}
}

func ValidateProject(project ProjectConfig) error {
	if project.SchemaVersion != ProjectSchemaVersion {
		return fmt.Errorf("schema_version must be %q", ProjectSchemaVersion)
	}
	if !projectIDPattern.MatchString(project.Project.ID) {
		return fmt.Errorf("project.id %q is invalid", project.Project.ID)
	}
	if !isScenario(project.Workflow.Scenario) {
		return fmt.Errorf("workflow.scenario %q is invalid", project.Workflow.Scenario)
	}
	if project.Workflow.Toolchain != ToolchainModern && project.Workflow.Toolchain != ToolchainLegacyEquivalent {
		return fmt.Errorf("workflow.toolchain %q is invalid", project.Workflow.Toolchain)
	}
	if err := validateLegacyExtensions(project.Workflow.LegacyExtensions); err != nil {
		return err
	}
	if project.Execution.Executor != ExecutorCraftmake && project.Execution.Executor != ExecutorSnakemake {
		return fmt.Errorf("execution.executor %q is invalid", project.Execution.Executor)
	}
	if project.Execution.Backend != BackendAuto && project.Execution.Backend != BackendLocal && project.Execution.Backend != BackendSlurm {
		return fmt.Errorf("execution.backend %q is invalid", project.Execution.Backend)
	}
	if strings.TrimSpace(project.Execution.Site) == "" {
		return fmt.Errorf("execution.site is required")
	}
	if strings.TrimSpace(project.Samples.Manifest) == "" {
		return fmt.Errorf("samples.manifest is required")
	}
	if err := validateProjectReferences(project.Workflow.Scenario, project.References); err != nil {
		return err
	}
	if err := validateProjectResources(project.Resources); err != nil {
		return err
	}
	return nil
}

func ValidateReferencesLock(lock ReferencesLock) error {
	if lock.SchemaVersion != ReferencesLockSchemaVersion {
		return fmt.Errorf("schema_version must be %q", ReferencesLockSchemaVersion)
	}
	if len(lock.References) == 0 {
		return fmt.Errorf("references must not be empty")
	}
	for role, reference := range lock.References {
		if strings.TrimSpace(role) == "" || strings.TrimSpace(reference.ID) == "" || strings.TrimSpace(reference.Release) == "" {
			return fmt.Errorf("reference lock entry %q is incomplete", role)
		}
		if !digestPattern.MatchString(reference.ManifestDigest) {
			return fmt.Errorf("reference lock entry %q has invalid manifest digest", role)
		}
	}
	if lock.PromotedFromRunID != "" && !runIDPattern.MatchString(lock.PromotedFromRunID) {
		return fmt.Errorf("promoted_from_run_id %q is invalid", lock.PromotedFromRunID)
	}
	return nil
}

func ValidateReferenceDefinition(definition ReferenceDefinition) error {
	if definition.SchemaVersion != ReferenceSchemaVersion {
		return fmt.Errorf("schema_version must be %q", ReferenceSchemaVersion)
	}
	if definition.Reference.ID == "" || definition.Reference.Release == "" || definition.Reference.Organism == "" || definition.Reference.Assembly == "" {
		return fmt.Errorf("reference identity is incomplete")
	}
	if err := validateRelativeAssetPath(definition.Assets.Fasta.Path); err != nil {
		return fmt.Errorf("fasta path: %w", err)
	}
	if err := validateRelativeAssetPath(definition.Assets.Fasta.FAI); err != nil {
		return fmt.Errorf("fasta index path: %w", err)
	}
	if !digestPattern.MatchString(definition.Assets.Fasta.SHA256) {
		return fmt.Errorf("fasta sha256 is invalid")
	}
	if definition.Assets.Fasta.SizeBytes < 1 {
		return fmt.Errorf("fasta size_bytes must be positive")
	}
	if len(definition.Assets.Indexes) == 0 {
		return fmt.Errorf("at least one reference index is required")
	}
	for _, annotation := range definition.Assets.Annotations {
		if annotation.ID == "" || annotation.Type == "" {
			return fmt.Errorf("reference annotation identity is incomplete")
		}
		if err := validateRelativeAssetPath(annotation.Path); err != nil {
			return fmt.Errorf("annotation %q path: %w", annotation.ID, err)
		}
		if !digestPattern.MatchString(annotation.SHA256) {
			return fmt.Errorf("annotation %q sha256 is invalid", annotation.ID)
		}
	}
	for _, index := range definition.Assets.Indexes {
		if index.Type == "" || index.Tool == "" || index.ToolVersion == "" {
			return fmt.Errorf("reference index identity is incomplete")
		}
		if err := validateRelativeAssetPath(index.Path); err != nil {
			return fmt.Errorf("index %q path: %w", index.Type, err)
		}
		if !digestPattern.MatchString(index.ReferenceFastaSHA256) || !digestPattern.MatchString(index.ManifestSHA256) {
			return fmt.Errorf("index %q digest is invalid", index.Type)
		}
		if index.ReferenceFastaSHA256 != definition.Assets.Fasta.SHA256 {
			return fmt.Errorf("index %q was built from a different FASTA digest", index.Type)
		}
	}
	if len(definition.Compatibility.Scenarios) == 0 {
		return fmt.Errorf("reference compatibility scenarios must not be empty")
	}
	for _, scenario := range definition.Compatibility.Scenarios {
		if !isScenario(scenario) {
			return fmt.Errorf("reference compatibility scenario %q is invalid", scenario)
		}
	}
	return nil
}

func ValidateRunSnapshot(snapshot RunSnapshot) error {
	if snapshot.SchemaVersion != RunSchemaVersion {
		return fmt.Errorf("schema_version must be %q", RunSchemaVersion)
	}
	if !runIDPattern.MatchString(snapshot.Run.ID) {
		return fmt.Errorf("run.id %q is invalid", snapshot.Run.ID)
	}
	if _, err := time.Parse(time.RFC3339, string(snapshot.Run.CreatedAt)); err != nil {
		return fmt.Errorf("run.created_at must be RFC3339: %w", err)
	}
	if !snapshot.Run.Immutable {
		return fmt.Errorf("run metadata must include immutable=true")
	}
	if snapshot.Run.ParentRunID != "" && !runIDPattern.MatchString(snapshot.Run.ParentRunID) {
		return fmt.Errorf("run.parent_run_id %q is invalid", snapshot.Run.ParentRunID)
	}
	if !projectIDPattern.MatchString(snapshot.Project.ID) || !filepath.IsAbs(snapshot.Project.Root) {
		return fmt.Errorf("resolved project id and absolute root are required")
	}
	if !isScenario(snapshot.Workflow.Scenario) || !filepath.IsAbs(snapshot.Workflow.AssetRoot) {
		return fmt.Errorf("resolved workflow is invalid")
	}
	if snapshot.Workflow.Toolchain != ToolchainModern && snapshot.Workflow.Toolchain != ToolchainLegacyEquivalent {
		return fmt.Errorf("resolved workflow toolchain %q is invalid", snapshot.Workflow.Toolchain)
	}
	if err := validateLegacyExtensions(snapshot.Workflow.LegacyExtensions); err != nil {
		return fmt.Errorf("resolved workflow extensions: %w", err)
	}
	if snapshot.Execution.Executor.Value != ExecutorCraftmake && snapshot.Execution.Executor.Value != ExecutorSnakemake {
		return fmt.Errorf("resolved executor %q is invalid", snapshot.Execution.Executor.Value)
	}
	if snapshot.Execution.Backend.Value != BackendLocal && snapshot.Execution.Backend.Value != BackendSlurm {
		return fmt.Errorf("resolved backend must be local or slurm")
	}
	if !isValueSource(snapshot.Execution.Executor.Source) || !isValueSource(snapshot.Execution.Backend.Source) || !isValueSource(snapshot.Execution.Site.Source) {
		return fmt.Errorf("resolved execution source is invalid")
	}
	if strings.TrimSpace(snapshot.Execution.Site.Value) == "" {
		return fmt.Errorf("resolved site is required")
	}
	if err := validateProjectResources(snapshot.Execution.Resources); err != nil {
		return fmt.Errorf("resolved execution resources: %w", err)
	}
	if snapshot.Execution.Backend.Value == BackendSlurm {
		if err := validateResolvedSlurmResources(snapshot.Execution.Slurm); err != nil {
			return fmt.Errorf("resolved SLURM resources: %w", err)
		}
	}
	if err := validateParityConfig(snapshot.Parity, snapshot.Execution); err != nil {
		return err
	}
	if len(snapshot.Samples) == 0 || len(snapshot.References.Resolved) == 0 {
		return fmt.Errorf("resolved samples and references must not be empty")
	}
	if err := validateResolvedReferenceSelections("references.project_selection", snapshot.References.ProjectSelection); err != nil {
		return err
	}
	if err := validateResolvedReferenceSelections("references.effective_selection", snapshot.References.EffectiveSelection); err != nil {
		return err
	}
	if snapshot.References.OverrideSource != "" && snapshot.References.OverrideSource != OverrideSourceNone && snapshot.References.OverrideSource != OverrideSourceProject && snapshot.References.OverrideSource != OverrideSourceCLI && snapshot.References.OverrideSource != OverrideSourceOverrideFile {
		return fmt.Errorf("references.override_source %q is invalid", snapshot.References.OverrideSource)
	}
	seenSampleIDs := make(map[string]bool, len(snapshot.Samples))
	for _, sample := range snapshot.Samples {
		if strings.TrimSpace(sample.ID) == "" || !filepath.IsAbs(sample.R1) || !filepath.IsAbs(sample.R2) {
			return fmt.Errorf("sample %q must have absolute paired FASTQ paths", sample.ID)
		}
		if seenSampleIDs[sample.ID] {
			return fmt.Errorf("sample %q is duplicated", sample.ID)
		}
		if err := validateOptionalInputMetadata(sample.ID+" R1", sample.R1SHA256, sample.R1Size); err != nil {
			return err
		}
		if err := validateOptionalInputMetadata(sample.ID+" R2", sample.R2SHA256, sample.R2Size); err != nil {
			return err
		}
		seenSampleIDs[sample.ID] = true
	}
	seenReferenceRoles := make(map[ReferenceRole]bool, len(snapshot.References.Resolved))
	for _, resolvedReference := range snapshot.References.Resolved {
		if !isReferenceRole(resolvedReference.Role) || seenReferenceRoles[resolvedReference.Role] {
			return fmt.Errorf("resolved reference role %q is invalid or duplicated", resolvedReference.Role)
		}
		seenReferenceRoles[resolvedReference.Role] = true
		if resolvedReference.ID == "" || resolvedReference.Release == "" || !filepath.IsAbs(resolvedReference.RegistryRoot) {
			return fmt.Errorf("resolved reference %q is incomplete", resolvedReference.Role)
		}
		if !digestPattern.MatchString(resolvedReference.ManifestDigest) {
			return fmt.Errorf("resolved reference %q has an invalid manifest digest", resolvedReference.Role)
		}
		if err := validateResolvedAsset("fasta", resolvedReference.Fasta); err != nil {
			return fmt.Errorf("resolved reference %q: %w", resolvedReference.Role, err)
		}
		for _, annotation := range resolvedReference.Annotations {
			if err := validateResolvedAsset("annotation", annotation); err != nil {
				return fmt.Errorf("resolved reference %q: %w", resolvedReference.Role, err)
			}
		}
		for _, index := range resolvedReference.Indexes {
			if err := validateResolvedAsset("index", index); err != nil {
				return fmt.Errorf("resolved reference %q: %w", resolvedReference.Role, err)
			}
		}
	}
	if snapshot.Workflow.Scenario == ScenarioBSPDX || snapshot.Workflow.Scenario == ScenarioRNAPDX {
		if !seenReferenceRoles[ReferenceRoleGraft] || !seenReferenceRoles[ReferenceRoleHost] {
			return fmt.Errorf("PDX run snapshots require graft and host references")
		}
	} else if !seenReferenceRoles[ReferenceRolePrimary] {
		return fmt.Errorf("non-PDX run snapshots require a primary reference")
	}
	if !filepath.IsAbs(snapshot.Paths.RunRoot) || !filepath.IsAbs(snapshot.Paths.Work) || !filepath.IsAbs(snapshot.Paths.Results) || !filepath.IsAbs(snapshot.Paths.Logs) || !filepath.IsAbs(snapshot.Paths.State) || !filepath.IsAbs(snapshot.Paths.Metrics) {
		return fmt.Errorf("all run paths must be absolute")
	}
	for pathName, pathValue := range map[string]string{
		"project_config":   snapshot.Paths.ProjectConfig,
		"samples_manifest": snapshot.Paths.SamplesManifest,
		"references_lock":  snapshot.Paths.ReferencesLock,
	} {
		if pathValue != "" && !filepath.IsAbs(pathValue) {
			return fmt.Errorf("paths.%s must be absolute", pathName)
		}
	}
	for assetIndex, assetPath := range snapshot.Paths.WorkflowAssets {
		if !filepath.IsAbs(assetPath) {
			return fmt.Errorf("paths.workflow_assets[%d] must be absolute", assetIndex)
		}
	}
	for name, digest := range map[string]string{
		"project":         snapshot.Digests.Project,
		"samples":         snapshot.Digests.Samples,
		"workflow_assets": snapshot.Digests.WorkflowAssets,
	} {
		if !digestPattern.MatchString(digest) {
			return fmt.Errorf("%s digest is invalid", name)
		}
	}
	if snapshot.Digests.References != "" && !digestPattern.MatchString(snapshot.Digests.References) {
		return fmt.Errorf("references digest is invalid")
	}
	return nil
}

func validateResolvedReferenceSelections(field string, selections ReferenceSelections) error {
	selectionsByRole := []struct {
		role      string
		selection ReferenceSelection
	}{
		{role: "primary", selection: selections.Primary},
		{role: "secondary", selection: selections.Secondary},
		{role: "graft", selection: selections.Graft},
		{role: "host", selection: selections.Host},
	}
	for _, selectionByRole := range selectionsByRole {
		if selectionByRole.selection == "" {
			continue
		}
		if _, _, err := ParseReferenceSelection(selectionByRole.selection); err != nil {
			return fmt.Errorf("%s.%s: %w", field, selectionByRole.role, err)
		}
	}
	return nil
}

func ParseReferenceSelection(selection ReferenceSelection) (string, string, error) {
	value := string(selection)
	if !referenceSelectionPattern.MatchString(value) {
		return "", "", fmt.Errorf("reference selection %q must use id@release", value)
	}
	parts := strings.SplitN(value, "@", 2)
	return parts[0], parts[1], nil
}

func IsValidRunID(runID string) bool {
	return runIDPattern.MatchString(runID)
}

func isScenario(scenario Scenario) bool {
	return scenario == ScenarioRRBS || scenario == ScenarioWGBS || scenario == ScenarioRNASeq || scenario == ScenarioBSPDX || scenario == ScenarioRNAPDX
}

func isValueSource(source ValueSource) bool {
	return source == SourceDefault || source == SourceProject || source == SourceCLI || source == SourceProfile || source == SourceDetection
}

func isReferenceRole(role ReferenceRole) bool {
	return role == ReferenceRolePrimary || role == ReferenceRoleSecondary || role == ReferenceRoleGraft || role == ReferenceRoleHost
}

func validateResolvedAsset(assetName string, asset ResolvedAsset) error {
	if strings.TrimSpace(asset.Type) == "" || !filepath.IsAbs(asset.Path) || !digestPattern.MatchString(asset.SHA256) {
		return fmt.Errorf("%s asset is invalid", assetName)
	}
	return nil
}

func validateOptionalInputMetadata(inputName, digest string, sizeBytes int64) error {
	if digest == "" && sizeBytes == 0 {
		return nil
	}
	if !digestPattern.MatchString(digest) {
		return fmt.Errorf("%s digest is invalid", inputName)
	}
	if sizeBytes < 1 {
		return fmt.Errorf("%s size_bytes must be positive", inputName)
	}
	return nil
}

func validateResolvedSlurmResources(resources ResolvedSlurmResources) error {
	if strings.TrimSpace(resources.Partition.Value) == "" || !isValueSource(resources.Partition.Source) {
		return fmt.Errorf("partition value and source are required")
	}
	if strings.TrimSpace(resources.Account.Value) == "" || !isValueSource(resources.Account.Source) {
		return fmt.Errorf("account value and source are required")
	}
	if !isValueSource(resources.QOS.Source) {
		return fmt.Errorf("qos source is invalid")
	}
	if resources.MaxJobs.Value < 0 || !isValueSource(resources.MaxJobs.Source) {
		return fmt.Errorf("max_jobs value/source is invalid")
	}
	if resources.DefaultTime.Value != "" && !timePattern.MatchString(resources.DefaultTime.Value) {
		return fmt.Errorf("default_time %q has invalid format", resources.DefaultTime.Value)
	}
	if !isValueSource(resources.DefaultTime.Source) {
		return fmt.Errorf("default_time source is invalid")
	}
	if resources.ScratchRoot.Value != "" && !filepath.IsAbs(resources.ScratchRoot.Value) {
		return fmt.Errorf("scratch_root must be absolute")
	}
	if !isValueSource(resources.ScratchRoot.Source) {
		return fmt.Errorf("scratch_root source is invalid")
	}
	return nil
}

func validateProjectReferences(scenario Scenario, references ProjectReferences) error {
	hasPrimary := references.Primary != ""
	hasSpecies := len(references.Species) > 0
	if hasPrimary == hasSpecies {
		return fmt.Errorf("references must define exactly one of primary or species")
	}
	if hasPrimary {
		if _, _, err := ParseReferenceSelection(references.Primary); err != nil {
			return err
		}
		if scenario == ScenarioBSPDX || scenario == ScenarioRNAPDX {
			return fmt.Errorf("PDX scenarios require graft and host species references")
		}
		if references.Secondary != "" {
			if _, _, err := ParseReferenceSelection(references.Secondary); err != nil {
				return err
			}
		}
		return nil
	}

	seenRoles := make(map[ReferenceRole]bool, len(references.Species))
	for _, species := range references.Species {
		if seenRoles[species.Role] {
			return fmt.Errorf("duplicate reference role %q", species.Role)
		}
		seenRoles[species.Role] = true
		if _, _, err := ParseReferenceSelection(species.Selection); err != nil {
			return err
		}
	}
	if scenario == ScenarioBSPDX || scenario == ScenarioRNAPDX {
		if !seenRoles[ReferenceRoleGraft] || !seenRoles[ReferenceRoleHost] || len(seenRoles) != 2 {
			return fmt.Errorf("PDX scenarios require exactly graft and host references")
		}
	}
	return nil
}

func validateLegacyExtensions(extensions []string) error {
	allowed := map[string]bool{"clubcpg": true, "mhap": true, "ccgg": true, "insert-length": true}
	seen := make(map[string]bool, len(extensions))
	for _, extension := range extensions {
		if !allowed[extension] {
			return fmt.Errorf("workflow legacy extension %q is invalid", extension)
		}
		if seen[extension] {
			return fmt.Errorf("workflow legacy extension %q is duplicated", extension)
		}
		seen[extension] = true
	}
	return nil
}

func validateParityConfig(parity ParityConfig, execution ResolvedExecution) error {
	if parity.Policy == "" {
		return nil
	}
	if parity.Policy != ParityPolicyExecutorPhaseEnvelope {
		return fmt.Errorf("parity.policy %q is invalid", parity.Policy)
	}
	if execution.Backend.Value != BackendSlurm {
		return fmt.Errorf("executor phase envelope parity requires the slurm backend")
	}
	if len(execution.Resources.Phases) == 0 {
		return fmt.Errorf("executor phase envelope parity requires phase resources")
	}
	for phaseName, resource := range execution.Resources.Phases {
		if resource.Cores <= 0 ||
			resource.Memory == "" ||
			strings.TrimSpace(resource.Partition) == "" ||
			resource.Time == "" {
			return fmt.Errorf("executor phase envelope %q requires cores, memory, partition, and time", phaseName)
		}
	}
	return nil
}

func validateProjectResources(resources ProjectResources) error {
	if err := validateResourceSpec("resources.defaults", resources.Defaults); err != nil {
		return err
	}
	for phase, resource := range resources.Phases {
		if strings.TrimSpace(phase) == "" {
			return fmt.Errorf("resource phase name must not be empty")
		}
		if err := validateResourceSpec("resources.phases."+phase, resource); err != nil {
			return err
		}
	}
	return nil
}

func validateResourceSpec(field string, resource ResourceSpec) error {
	if resource.Cores < 0 {
		return fmt.Errorf("%s.cores must not be negative", field)
	}
	if resource.Memory != "" && !memoryPattern.MatchString(resource.Memory) {
		return fmt.Errorf("%s.memory %q is invalid", field, resource.Memory)
	}
	if resource.Time != "" && !timePattern.MatchString(resource.Time) {
		return fmt.Errorf("%s.time %q is invalid", field, resource.Time)
	}
	return nil
}

func validateRelativeAssetPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("path must be relative")
	}
	cleaned := filepath.Clean(path)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path must not escape the reference release")
	}
	return nil
}
