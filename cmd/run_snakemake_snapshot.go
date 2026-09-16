package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/otterlab-bio/otter/internal/assets"
	"github.com/otterlab-bio/otter/internal/config"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/engine"
	execution "github.com/otterlab-bio/otter/internal/execution"
	"github.com/otterlab-bio/otter/internal/logger"
	taskruntime "github.com/otterlab-bio/otter/internal/task"
	"github.com/otterlab-bio/otter/internal/workflow"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type snakemakeSnapshotInvocation struct {
	SnapshotPath        string
	ProjectDirectory    string
	Snapshot            configv1.RunSnapshot
	Configuration       *config.OtterConfig
	CompatibilityConfig string
}

func snakemakeStepForPhase(phase string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "step1":
		return 1, nil
	case "step2":
		return 2, nil
	case "step2-check":
		return 102, nil
	case "step3":
		return 3, nil
	case "step3-check":
		return 103, nil
	default:
		return 0, fmt.Errorf("unsupported Snakemake compatibility phase %q; expected step1, step2, step2-check, step3, or step3-check", phase)
	}
}

func executeSnakemakeSnapshotRun(command *cobra.Command) (runErr error) {
	if internalWorker {
		defer finishCurrentSnakemakeTask(&runErr)
	}
	invocation, err := loadSnakemakeSnapshotInvocation(command)
	if err != nil {
		return err
	}
	if err := verifyProjectAssets(invocation.ProjectDirectory); err != nil {
		return err
	}

	sampleNames := make([]string, 0, len(invocation.Snapshot.Samples))
	for _, sample := range invocation.Snapshot.Samples {
		sampleNames = append(sampleNames, sample.ID)
	}
	workflowInstance := workflow.NewWorkflow(invocation.Configuration, sampleNames)
	workflowInstance.Options = &workflow.WorkflowOptions{DryRun: dryRun, Resume: resumeFlag}
	manager := workflow.NewManager(workflowInstance)
	manager.SetSamples(sampleNames)
	manager.SetStepResources(invocation.Configuration.StepResources)
	manager.SetParallelJobs(parallelJobs)
	if loadRatio > 0 {
		manager.SetLoadRatio(loadRatio)
	}
	if err := manager.SetConfigFile(invocation.CompatibilityConfig); err != nil {
		return err
	}

	selectedStep := 0
	if runPhase != "" {
		selectedStep, err = snakemakeStepForPhase(runPhase)
		if err != nil {
			return err
		}
		phaseResource, found := invocation.Configuration.StepResources[selectedStep]
		if !found || phaseResource == nil {
			return fmt.Errorf("Snakemake compatibility phase %s has no resolved resource envelope", runPhase)
		}
		invocation.Configuration.Engine.Slurm.Cores = phaseResource.Cores
		invocation.Configuration.Engine.Slurm.Memory = phaseResource.Memory
		invocation.Configuration.Engine.Slurm.Time = phaseResource.Time
		invocation.Configuration.Engine.Slurm.Partition = phaseResource.Partition
	}

	executionEngine, err := engine.CreateEngineFromConfig(invocation.Configuration)
	if err != nil {
		return fmt.Errorf("create Snakemake execution engine: %w", err)
	}
	workflowInstance.SetEngine(executionEngine)
	if err := validateResources(executionEngine.GetName().String(), invocation.Configuration, invocation.Configuration.StepResources, parallelJobs, dryRun); err != nil {
		return fmt.Errorf("resource validation failed: %w", err)
	}
	if dryRun {
		manager.SetDryRun(true)
	}
	condaEnvironment := runCondaEnv
	if condaEnvironment == "" {
		condaEnvironment = invocation.Configuration.Engine.CondaEnv
	}
	if condaEnvironment != "" {
		manager.SetCondaEnv(condaEnvironment)
	}
	fallbackEnvironment := invocation.Configuration.Engine.FallbackEnv
	if fallbackEnvironment == "" {
		fallbackEnvironment = workflow.DefaultFallbackEnv
	}
	manager.SetFallbackConfig(fallbackEnvironment, invocation.Configuration.Engine.NoFallback)
	if err := validateRNAsplicingDependencies(invocation.Configuration); err != nil {
		return err
	}

	clearInheritedJavaHome := clearSnakemakeInheritedJavaHome()
	defer clearInheritedJavaHome()

	originalDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current working directory: %w", err)
	}
	if err := os.Chdir(invocation.ProjectDirectory); err != nil {
		return fmt.Errorf("switch to project directory %q: %w", invocation.ProjectDirectory, err)
	}
	defer func() {
		if restoreErr := os.Chdir(originalDirectory); restoreErr != nil {
			logger.Warnf("Failed to restore working directory %s: %v", originalDirectory, restoreErr)
		}
	}()
	if resumeFlag && !dryRun {
		if err := unlockSnakemakeProjectDirectory(invocation.ProjectDirectory); err != nil {
			return err
		}
	}
	if runPhase == "" {
		if err := manager.ExecuteAll(); err != nil {
			return fmt.Errorf("Snakemake workflow execution failed: %w", err)
		}
	} else {
		if err := manager.ExecuteSelectedSteps([]int{selectedStep}); err != nil {
			return fmt.Errorf("Snakemake phase %s execution failed: %w", runPhase, err)
		}
	}
	if dryRun || runPhase != "" {
		return nil
	}
	publicationResult, err := workflow.PublishSnakemakeArtifacts(workflow.SnakemakeArtifactPublicationRequest{
		Context:      command.Context(),
		SnapshotPath: invocation.SnapshotPath,
		Snapshot:     invocation.Snapshot,
	})
	if err != nil {
		return fmt.Errorf("publish Snakemake compatibility artifacts: %w", err)
	}
	if publicationResult.AlreadyPublished {
		fmt.Fprintf(command.OutOrStdout(), "Verified existing immutable artifact manifest: %s\n", publicationResult.ManifestPath)
		return nil
	}
	fmt.Fprintf(command.OutOrStdout(), "Published %d immutable artifacts: %s\n", publicationResult.ArtifactCount, publicationResult.ManifestPath)
	return nil
}

func clearSnakemakeInheritedJavaHome() func() {
	javaHome, wasDefined := os.LookupEnv("JAVA_HOME")
	_ = os.Unsetenv("JAVA_HOME")

	return func() {
		if wasDefined {
			_ = os.Setenv("JAVA_HOME", javaHome)
			return
		}
		_ = os.Unsetenv("JAVA_HOME")
	}
}

func unlockSnakemakeProjectDirectory(projectDirectory string) error {
	unlockCommand := exec.Command("snakemake", "--unlock")
	unlockCommand.Dir = projectDirectory
	output, err := unlockCommand.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unlock Snakemake working directory: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func submitBackgroundSnakemakeSnapshotRun(command *cobra.Command) error {
	invocation, err := loadSnakemakeSnapshotInvocation(command)
	if err != nil {
		return err
	}
	store, err := taskruntime.DefaultStore()
	if err != nil {
		return err
	}
	taskID, err := taskruntime.GenerateID()
	if err != nil {
		return err
	}
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve otter executable: %w", err)
	}
	workerArguments := buildBackgroundWorkerArguments(os.Args[1:], invocation.SnapshotPath, invocation.ProjectDirectory)
	record := taskruntime.NewRecord(taskID, invocation.ProjectDirectory, invocation.SnapshotPath, runExecutorSnakemake, append([]string{executablePath}, workerArguments...))
	record.LogPath = store.LogPath(taskID)
	record.StatePath = workflow.NewState(invocation.Snapshot.Paths.RunRoot, invocation.Snapshot.Run.ID).GetFilePath()
	if err := store.Create(record); err != nil {
		return err
	}
	logFile, err := os.OpenFile(record.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open background task log: %w", err)
	}
	defer logFile.Close()
	workerCommand := exec.Command(executablePath, workerArguments...)
	workerCommand.Dir = invocation.ProjectDirectory
	workerCommand.Stdout = logFile
	workerCommand.Stderr = logFile
	workerCommand.Env = append(os.Environ(),
		taskruntime.EnvironmentTaskID+"="+taskID,
		taskruntime.EnvironmentTaskStateDir+"="+store.RootDir(),
	)
	workerCommand.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := workerCommand.Start(); err != nil {
		return fmt.Errorf("start background Snakemake task: %w", err)
	}
	processID := workerCommand.Process.Pid
	if err := store.Update(taskID, func(current *taskruntime.Record) error {
		current.PID = processID
		current.ProcessGroupID = processID
		current.Status = taskruntime.StatusRunning
		current.StartedAt = time.Now()
		return nil
	}); err != nil {
		_ = syscall.Kill(-processID, syscall.SIGTERM)
		return err
	}
	if err := workerCommand.Process.Release(); err != nil {
		return fmt.Errorf("release background Snakemake worker: %w", err)
	}
	fmt.Printf("Task submitted: %s\n", taskID)
	fmt.Printf("Run:            %s\n", invocation.Snapshot.Run.ID)
	fmt.Printf("Executor:       snakemake\n")
	fmt.Printf("Status:         otter task status %s\n", taskID)
	fmt.Printf("Logs:           otter task logs %s --follow\n", taskID)
	return nil
}

func loadSnakemakeSnapshotInvocation(command *cobra.Command) (snakemakeSnapshotInvocation, error) {
	configPath, _, err := resolveRunPaths(runConfigFile, runProjectDir)
	if err != nil {
		return snakemakeSnapshotInvocation{}, err
	}
	invocation, err := execution.LoadRunInvocation(configPath, configv1.ExecutorSnakemake)
	if err != nil {
		return snakemakeSnapshotInvocation{}, fmt.Errorf("%s", snakemakeSnapshotBoundaryError(configPath, err))
	}
	if err := rejectSnakemakeRuntimeOverrides(command, invocation.Snapshot); err != nil {
		return snakemakeSnapshotInvocation{}, err
	}
	configuration, err := workflow.SnapshotToLegacyConfig(invocation.Snapshot)
	if err != nil {
		return snakemakeSnapshotInvocation{}, err
	}
	compatibilityConfigPath, err := writeLegacyRuntimeConfig(invocation.Snapshot, configuration)
	if err != nil {
		return snakemakeSnapshotInvocation{}, err
	}
	return snakemakeSnapshotInvocation{
		SnapshotPath:        invocation.SnapshotPath,
		ProjectDirectory:    invocation.ProjectDirectory,
		Snapshot:            invocation.Snapshot,
		Configuration:       configuration,
		CompatibilityConfig: compatibilityConfigPath,
	}, nil
}

// snakemakeSnapshotBoundaryError turns a snapshot load failure into guidance.
//
// Two cases dominate, and the raw error reads as a decode failure in both:
// a legacy otter.yaml reaching the executor, and a valid v1 snapshot that
// selected a different executor.
func snakemakeSnapshotBoundaryError(configPath string, cause error) string {
	const boundary = "The Snakemake compatibility executor consumes an immutable otter.run/v1 run.yaml"
	if looksLikeLegacyOtterConfig(configPath) {
		return fmt.Sprintf(
			"%s; %s is a legacy compatibility configuration.\n"+
				"Migrate it to the canonical track first:\n"+
				"  otter config migrate --input %s --output project.yaml \\\n"+
				"    --reference-primary <id@release> --reference-root <registry>\n"+
				"  otter config resolve --project project.yaml --backend local\n"+
				"  otter run --config runs/<run-id>/run.yaml --executor snakemake",
			boundary, configPath, configPath)
	}
	if selectedExecutor := snapshotExecutorValue(configPath); selectedExecutor != "" && selectedExecutor != runExecutorSnakemake {
		return fmt.Sprintf(
			"%s, but %s resolves executor %q.\n"+
				"Choose one:\n"+
				"  - run the snapshot with its own executor:  --executor %s\n"+
				"  - resolve a compatibility snapshot:       set `execution.executor: snakemake` in project.yaml, then `otter config resolve`",
			boundary, configPath, selectedExecutor, selectedExecutor)
	}
	return fmt.Sprintf("%s: %v", boundary, cause)
}

// snapshotExecutorValue reports the executor a run snapshot selects, or an
// empty string when the file is not a readable v1 snapshot.
func snapshotExecutorValue(configPath string) string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	var document struct {
		SchemaVersion string `yaml:"schema_version"`
		Execution     struct {
			Executor struct {
				Value string `yaml:"value"`
			} `yaml:"executor"`
		} `yaml:"execution"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return ""
	}
	if document.SchemaVersion != configv1.RunSchemaVersion {
		return ""
	}
	return document.Execution.Executor.Value
}

func rejectSnakemakeRuntimeOverrides(command *cobra.Command, snapshot configv1.RunSnapshot) error {
	resolvedBackend := string(snapshot.Execution.Backend.Value)
	if command.Flags().Changed("backend") && runBackend != "auto" && runBackend != resolvedBackend {
		return fmt.Errorf("--backend %s conflicts with immutable run backend %s", runBackend, resolvedBackend)
	}
	if command.Flags().Changed("engine") && runEngine != "auto" && runEngine != resolvedBackend {
		return fmt.Errorf("--engine %s conflicts with immutable run backend %s", runEngine, resolvedBackend)
	}
	immutableFlags := []string{
		"slurm-partition", "slurm-cores", "slurm-memory", "slurm-unified-partition",
		"step1-cores", "step1-memory", "step1-partition",
		"step2-cores", "step2-memory", "step2-partition",
		"step3-cores", "step3-memory", "step3-partition",
		"step2-checker-cores", "step2-checker-memory",
		"step3-checker-cores", "step3-checker-memory",
		"copy-fastq", "move-fastq", "compress-fastq",
	}
	for _, flagName := range immutableFlags {
		if command.Flags().Changed(flagName) {
			return fmt.Errorf("--%s cannot override an immutable run snapshot; resolve a new run.yaml instead", flagName)
		}
	}
	return nil
}

func legacyRuntimeSpeciesIdentifier(snapshot configv1.RunSnapshot, fallback string) string {
	for _, preferredRole := range []configv1.ReferenceRole{
		configv1.ReferenceRolePrimary,
		configv1.ReferenceRoleGraft,
	} {
		for _, reference := range snapshot.References.Resolved {
			if reference.Role == preferredRole && strings.TrimSpace(reference.ID) != "" {
				return reference.ID
			}
		}
	}
	return fallback
}

func legacyRuntimeSpeciesIdentifiers(snapshot configv1.RunSnapshot, fallback string) []string {
	identifiers := make([]string, 0, 3)
	seenIdentifiers := make(map[string]struct{})
	for _, reference := range snapshot.References.Resolved {
		switch reference.Role {
		case configv1.ReferenceRolePrimary, configv1.ReferenceRoleGraft, configv1.ReferenceRoleHost, configv1.ReferenceRoleSecondary:
		default:
			continue
		}
		identifier := strings.TrimSpace(reference.ID)
		if identifier == "" {
			continue
		}
		if _, alreadyAdded := seenIdentifiers[identifier]; alreadyAdded {
			continue
		}
		seenIdentifiers[identifier] = struct{}{}
		identifiers = append(identifiers, identifier)
	}
	if len(identifiers) == 0 {
		identifiers = append(identifiers, fallback)
	}
	return identifiers
}

func setPDXReferenceFASTAMappings(referenceNode *yaml.Node, snapshot configv1.RunSnapshot) error {
	graftReference, hostReference := legacyRuntimeReferenceByRole(snapshot, configv1.ReferenceRoleGraft), legacyRuntimeReferenceByRole(snapshot, configv1.ReferenceRoleHost)
	if graftReference != nil {
		setYAMLMappingValue(referenceNode, "graft_fasta", yamlStringScalar(graftReference.Fasta.Path))
	}
	if hostReference != nil {
		setYAMLMappingValue(referenceNode, "host_fasta", yamlStringScalar(hostReference.Fasta.Path))
	}
	if snapshot.Workflow.Scenario == configv1.ScenarioBSPDX || snapshot.Workflow.Scenario == configv1.ScenarioRNAPDX {
		if graftReference == nil || hostReference == nil {
			return fmt.Errorf("PDX snapshot requires graft and host FASTA references")
		}
	}
	return nil
}

func legacyRuntimeReferenceByRole(snapshot configv1.RunSnapshot, role configv1.ReferenceRole) *configv1.ResolvedReference {
	for referenceIndex := range snapshot.References.Resolved {
		reference := &snapshot.References.Resolved[referenceIndex]
		if reference.Role == role && strings.TrimSpace(reference.Fasta.Path) != "" {
			return reference
		}
	}
	return nil
}

func writeLegacyRuntimeConfig(snapshot configv1.RunSnapshot, configuration *config.OtterConfig) (string, error) {
	encoded, err := config.MarshalWithMapstructureTags(configuration)
	if err != nil {
		return "", err
	}
	compatibilityDocument := yaml.Node{}
	if err := yaml.Unmarshal(encoded, &compatibilityDocument); err != nil {
		return "", fmt.Errorf("decode legacy runtime config: %w", err)
	}
	if len(compatibilityDocument.Content) != 1 || compatibilityDocument.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("legacy runtime config must be a YAML mapping")
	}

	rootMapping := compatibilityDocument.Content[0]
	setYAMLMappingValue(rootMapping, "mode", yamlStringScalar(configuration.Workflow.Mode))
	runtimeSpeciesIdentifier := legacyRuntimeSpeciesIdentifier(snapshot, configuration.Workflow.Species.Primary)
	runtimeSpeciesIdentifiers := legacyRuntimeSpeciesIdentifiers(snapshot, runtimeSpeciesIdentifier)
	setYAMLMappingValue(rootMapping, "SIDs", yamlStringSequence(configuration.Metadata.SampleIDs))
	setYAMLMappingValue(rootMapping, "species", yamlStringScalar(runtimeSpeciesIdentifier))
	setYAMLMappingValue(rootMapping, "outdir_qualimap", yamlStringScalar(configuration.Directories.Qualimap))
	setYAMLMappingValue(rootMapping, "outDir_mCall", yamlStringScalar(configuration.Directories.MethylationCall))
	setYAMLMappingValue(rootMapping, "graft", yamlStringScalar(configuration.Workflow.Species.Graft))
	setYAMLMappingValue(rootMapping, "qc_summary", yamlStringScalar(configuration.Directories.QCSummary))

	workflowNode, found := yamlMappingValue(rootMapping, "workflow")
	if !found || workflowNode.Kind != yaml.MappingNode {
		return "", fmt.Errorf("legacy runtime config has no workflow mapping")
	}
	workflowSpeciesNode, found := yamlMappingValue(workflowNode, "species")
	if !found || workflowSpeciesNode.Kind != yaml.MappingNode {
		return "", fmt.Errorf("legacy runtime config has no workflow.species mapping")
	}
	setYAMLMappingValue(workflowSpeciesNode, "primary", yamlStringScalar(runtimeSpeciesIdentifier))
	setYAMLMappingValue(workflowSpeciesNode, "expression", yamlStringScalar(configuration.Workflow.Species.Expression))
	setYAMLMappingValue(workflowSpeciesNode, "name", yamlStringSequence(runtimeSpeciesIdentifiers))

	directoriesNode, found := yamlMappingValue(rootMapping, "directories")
	if !found || directoriesNode.Kind != yaml.MappingNode {
		return "", fmt.Errorf("legacy runtime config has no directories mapping")
	}
	setYAMLMappingValue(directoriesNode, "qc", yamlQCMapping(configuration.Directories.QC))
	setYAMLMappingValue(directoriesNode, "bsmap", yamlBSMAPMapping(configuration.Directories.BSMAP))
	setYAMLMappingValue(directoriesNode, "qualimap", yamlStringScalar(configuration.Directories.Qualimap))
	setYAMLMappingValue(directoriesNode, "work", yamlStringScalar(configuration.Directories.Work))
	setYAMLMappingValue(directoriesNode, "selfconfig", yamlStringScalar(snapshot.Paths.State))
	setYAMLMappingValue(
		directoriesNode,
		"qctb_config",
		yamlStringScalar(filepath.Join(snapshot.Paths.RunRoot, "run.yaml")),
	)
	setYAMLMappingValue(directoriesNode, "methylation_call", yamlStringScalar(configuration.Directories.MethylationCall))

	referenceNode, found := yamlMappingValue(rootMapping, "reference")
	if !found || referenceNode.Kind != yaml.MappingNode {
		return "", fmt.Errorf("legacy runtime config has no reference mapping")
	}
	referenceIndicesNode, found := yamlMappingValue(referenceNode, "indices")
	if !found || referenceIndicesNode.Kind != yaml.MappingNode {
		return "", fmt.Errorf("legacy runtime config has no reference.indices mapping")
	}
	setYAMLMappingValue(referenceIndicesNode, "genome", yamlStringSequence(configuration.Reference.Indices.Genome))
	if err := setPDXReferenceFASTAMappings(referenceNode, snapshot); err != nil {
		return "", err
	}

	metadataNode, found := yamlMappingValue(rootMapping, "metadata")
	if !found || metadataNode.Kind != yaml.MappingNode {
		return "", fmt.Errorf("legacy runtime config has no metadata mapping")
	}
	setYAMLMappingValue(metadataNode, "sample_ids", yamlStringSequence(configuration.Metadata.SampleIDs))
	setYAMLMappingValue(metadataNode, "group_levels", yamlIntegerScalar(configuration.Metadata.GroupLevels))

	encoded, err = yaml.Marshal(&compatibilityDocument)
	if err != nil {
		return "", fmt.Errorf("encode legacy runtime config: %w", err)
	}
	path := filepath.Join(snapshot.Paths.State, "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create legacy runtime config directory: %w", err)
	}
	if existingContent, err := os.ReadFile(path); err == nil {
		equivalent, comparisonErr := equivalentYAMLDocuments(existingContent, encoded)
		if comparisonErr != nil {
			return "", fmt.Errorf("compare existing legacy runtime config %s: %w", path, comparisonErr)
		}
		if equivalent {
			return path, nil
		}
		matchesSnapshot, validationErr := legacyRuntimeConfigMatchesSnapshot(existingContent, snapshot)
		if validationErr != nil {
			return "", fmt.Errorf("validate existing legacy runtime config %s: %w", path, validationErr)
		}
		if !matchesSnapshot {
			return "", fmt.Errorf("legacy runtime config already exists with different content: %s", path)
		}
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect legacy runtime config: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o444); err != nil {
		return "", fmt.Errorf("write legacy runtime config: %w", err)
	}
	return path, nil
}

func equivalentYAMLDocuments(leftDocument, rightDocument []byte) (bool, error) {
	var leftValue interface{}
	if err := yaml.Unmarshal(leftDocument, &leftValue); err != nil {
		return false, fmt.Errorf("decode existing YAML: %w", err)
	}
	var rightValue interface{}
	if err := yaml.Unmarshal(rightDocument, &rightValue); err != nil {
		return false, fmt.Errorf("decode generated YAML: %w", err)
	}
	return reflect.DeepEqual(leftValue, rightValue), nil
}

func legacyRuntimeConfigMatchesSnapshot(document []byte, snapshot configv1.RunSnapshot) (bool, error) {
	compatibilityDocument := yaml.Node{}
	if err := yaml.Unmarshal(document, &compatibilityDocument); err != nil {
		return false, fmt.Errorf("decode YAML: %w", err)
	}
	if len(compatibilityDocument.Content) != 1 || compatibilityDocument.Content[0].Kind != yaml.MappingNode {
		return false, fmt.Errorf("legacy runtime config must be a YAML mapping")
	}

	rootMapping := compatibilityDocument.Content[0]
	workflowNode, found := yamlMappingValue(rootMapping, "workflow")
	if !found || workflowNode.Kind != yaml.MappingNode {
		return false, nil
	}
	workflowJobID, found := yamlMappingValue(workflowNode, "jobid")
	if !found || workflowJobID.Value != snapshot.Run.ID {
		return false, nil
	}

	outputNode, found := yamlMappingValue(rootMapping, "output")
	if !found || outputNode.Kind != yaml.MappingNode {
		return false, nil
	}
	outputBaseDirectory, found := yamlMappingValue(outputNode, "base_dir")
	if !found || outputBaseDirectory.Value != snapshot.Paths.RunRoot {
		return false, nil
	}

	directoriesNode, found := yamlMappingValue(rootMapping, "directories")
	if !found || directoriesNode.Kind != yaml.MappingNode {
		return false, nil
	}
	selfConfigDirectory, found := yamlMappingValue(directoriesNode, "selfconfig")
	if !found || selfConfigDirectory.Value != snapshot.Paths.State {
		return false, nil
	}
	qctbConfigurationPath, found := yamlMappingValue(directoriesNode, "qctb_config")
	if !found || qctbConfigurationPath.Value != filepath.Join(snapshot.Paths.RunRoot, "run.yaml") {
		return false, nil
	}
	return true, nil
}

func yamlMappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	for nodeIndex := 0; nodeIndex < len(mapping.Content); nodeIndex += 2 {
		if mapping.Content[nodeIndex].Value == key {
			return mapping.Content[nodeIndex+1], true
		}
	}
	return nil, false
}

func setYAMLMappingValue(mapping *yaml.Node, key string, value *yaml.Node) {
	for nodeIndex := 0; nodeIndex < len(mapping.Content); nodeIndex += 2 {
		if mapping.Content[nodeIndex].Value == key {
			mapping.Content[nodeIndex+1] = value
			return
		}
	}
	mapping.Content = append(mapping.Content, yamlStringScalar(key), value)
}

func yamlStringScalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func yamlIntegerScalar(value int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("%d", value)}
}

func yamlStringSequence(values []string) *yaml.Node {
	sequence := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range values {
		sequence.Content = append(sequence.Content, yamlStringScalar(value))
	}
	return sequence
}

func yamlQCMapping(qualityControl config.QCConfig) *yaml.Node {
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	setYAMLMappingValue(mapping, "main", yamlStringScalar(qualityControl.Main))
	setYAMLMappingValue(mapping, "before", yamlStringScalar(qualityControl.Before))
	setYAMLMappingValue(mapping, "after", yamlStringScalar(qualityControl.After))
	return mapping
}

func yamlBSMAPMapping(bsmap config.BSMAPConfig) *yaml.Node {
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	setYAMLMappingValue(mapping, "main", yamlStringScalar(bsmap.Main))
	setYAMLMappingValue(mapping, "bamtmp", yamlStringScalar(bsmap.Temp))
	setYAMLMappingValue(mapping, "Filtered_bams", yamlStringScalar(bsmap.Filtered))
	return mapping
}

func verifyProjectAssets(projectDirectory string) error {
	if !verifyAssets {
		return nil
	}
	manifest, err := assets.LoadManifest(projectDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warnf("Assets manifest not found: %s", assets.ManifestPath(projectDirectory))
			return nil
		}
		return fmt.Errorf("load assets manifest: %w", err)
	}
	difference, err := assets.VerifyWorkflowAssets(projectDirectory, manifest)
	if err != nil {
		return fmt.Errorf("verify workflow assets: %w", err)
	}
	if difference.IsClean() {
		return nil
	}
	message := fmt.Sprintf("assets differ from manifest (missing=%d extra=%d modified=%d)", len(difference.Missing), len(difference.Extra), len(difference.Modified))
	if strictAssets {
		return fmt.Errorf("%s", message)
	}
	logger.Warn(message)
	return nil
}

func finishCurrentSnakemakeTask(runError *error) func() {
	return func() {
		exitCode := 0
		status := taskruntime.StatusCompleted
		errorMessage := ""
		if *runError != nil {
			exitCode = 1
			status = taskruntime.StatusFailed
			errorMessage = (*runError).Error()
		}
		_ = taskruntime.UpdateCurrent(func(record *taskruntime.Record) error {
			if record.Status == taskruntime.StatusStopping {
				status = taskruntime.StatusStopped
			}
			record.Status = status
			record.ExitCode = &exitCode
			record.Error = errorMessage
			record.FinishedAt = time.Now()
			return nil
		})
	}
}
