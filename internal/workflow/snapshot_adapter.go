package workflow

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/internal/config"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

func SnapshotToLegacyConfig(snapshot configv1.RunSnapshot) (*config.OtterConfig, error) {
	if snapshot.Execution.Executor.Value != configv1.ExecutorSnakemake && snapshot.Execution.Executor.Value != configv1.ExecutorCraftmake {
		return nil, fmt.Errorf("snapshot executor must be craftmake or snakemake, got %q", snapshot.Execution.Executor.Value)
	}
	primaryReference, graftReference, hostReference := selectSnapshotReferences(snapshot)
	if primaryReference == nil {
		return nil, fmt.Errorf("snapshot requires a primary reference for legacy runtime configuration")
	}

	if strings.TrimSpace(primaryReference.Organism) == "" {
		return nil, fmt.Errorf(
			"primary reference %s@%s has no organism metadata; resolve a new snapshot from a complete reference registry entry",
			primaryReference.ID,
			primaryReference.Release,
		)
	}

	mode, err := snapshotMode(snapshot.Workflow.Scenario)
	if err != nil {
		return nil, err
	}
	legacyMode := mode
	if snapshot.Workflow.Scenario == configv1.ScenarioBSPDX {
		legacyMode = "PDX"
	}
	expressionReference := primaryReference
	if snapshot.Workflow.Scenario == configv1.ScenarioRNAPDX {
		expressionReference = graftReference
	}
	if expressionReference == nil {
		return nil, fmt.Errorf("snapshot requires an expression reference for scenario %q", snapshot.Workflow.Scenario)
	}
	expressionSpecies, err := seq2matSpeciesForReference(expressionReference)
	if err != nil {
		return nil, err
	}
	fastqDirectory := filepath.Dir(snapshot.Samples[0].R1)
	configuration := &config.OtterConfig{
		Workflow: config.WorkflowConfig{
			Mode:    legacyMode,
			UserID:  snapshot.Project.ID,
			JobID:   snapshot.Run.ID,
			Samples: snapshotSamples(snapshot.Samples),
			Species: config.SpeciesConfig{
				Primary:    primaryReference.Organism,
				Name:       []string{primaryReference.Organism},
				Expression: expressionSpecies,
			},
			Adapters:  config.AdapterConfig{ErrorRate: 0.2},
			Alignment: config.AlignmentConfig{},
		},
		Input: config.InputConfig{FastqDir: fastqDirectory},
		Output: config.OutputConfig{
			BaseDir:     snapshot.Paths.RunRoot,
			WorkflowDir: snapshot.Paths.Work,
			AnalysisDir: snapshot.Paths.Results,
			RawDir:      fastqDirectory,
			LogDir:      snapshot.Paths.Logs,
			TrimDir:     filepath.Join(snapshot.Paths.Work, "trim"),
		},
		Reference: config.ReferenceConfig{
			Files: config.ReferenceFiles{
				Fasta: []string{primaryReference.Fasta.Path},
			},
			GenomeFasta: []string{primaryReference.Fasta.Path},
			Indices: config.ReferenceIndices{
				Genome: []string{selectReferenceIndex(primaryReference, mode)},
			},
		},
		Directories: snapshotDirectories(snapshot, primaryReference, graftReference, hostReference),
		Metadata: config.MetadataConfig{
			SampleIDs:   sampleIDs(snapshot.Samples),
			GroupLevels: snapshotGroupLevelCount(snapshot.Samples),
		},
		Engine: config.EngineConfig{Type: string(snapshot.Execution.Backend.Value)},
	}
	if primaryReference != nil {
		configuration.Reference.RNAseq.GTF = []string{selectReferenceAnnotation(primaryReference, "gtf")}
		configuration.Reference.RNAseq.Reference = []string{selectReferenceIndex(primaryReference, "RNASEQ")}
	}
	if graftReference != nil {
		configuration.Workflow.Species.Graft = graftReference.ID
		if snapshot.Workflow.Scenario == configv1.ScenarioBSPDX || snapshot.Workflow.Scenario == configv1.ScenarioRNAPDX {
			configuration.Reference.Files.Fasta = append(configuration.Reference.Files.Fasta, graftReference.Fasta.Path)
			configuration.Reference.Indices.Genome = append(configuration.Reference.Indices.Genome, selectReferenceIndex(graftReference, mode))
		}
	}
	if hostReference != nil {
		configuration.Workflow.Species.Host = hostReference.ID
		configuration.Workflow.Species.Secondary = hostReference.ID
		configuration.Reference.Files.Fasta = append(configuration.Reference.Files.Fasta, hostReference.Fasta.Path)
		configuration.Reference.Indices.Genome = append(configuration.Reference.Indices.Genome, selectReferenceIndex(hostReference, mode))
	}
	if graftReference != nil && snapshot.Workflow.Scenario != configv1.ScenarioBSPDX && snapshot.Workflow.Scenario != configv1.ScenarioRNAPDX {
		configuration.Workflow.Species.Graft = primaryReference.ID
	}
	if snapshot.Workflow.Scenario == configv1.ScenarioBSPDX || snapshot.Workflow.Scenario == configv1.ScenarioRNAPDX {
		if graftReference == nil || hostReference == nil {
			return nil, fmt.Errorf("PDX snapshot requires graft and host references")
		}
	}
	for _, sample := range snapshot.Samples {
		configuration.Workflow.Adapters.Seq1 = append(configuration.Workflow.Adapters.Seq1, sample.AdapterR1)
		configuration.Workflow.Adapters.Seq2 = append(configuration.Workflow.Adapters.Seq2, sample.AdapterR2)
	}
	configuration.StepResources = snapshotStepResources(snapshot.Execution.Resources)
	configuration.Engine.Slurm.Partition = snapshot.Execution.Resources.Defaults.Partition
	configuration.Engine.Slurm.Memory = snapshot.Execution.Resources.Defaults.Memory
	configuration.Engine.Slurm.Time = snapshot.Execution.Resources.Defaults.Time
	configuration.Engine.Slurm.Cores = snapshot.Execution.Resources.Defaults.Cores
	return configuration, nil
}

func snapshotMode(scenario configv1.Scenario) (string, error) {
	switch scenario {
	case configv1.ScenarioRRBS:
		return "RRBS", nil
	case configv1.ScenarioWGBS:
		return "WGBS", nil
	case configv1.ScenarioRNASeq, configv1.ScenarioRNAPDX:
		return "RNASEQ", nil
	case configv1.ScenarioBSPDX:
		return "RRBS", nil
	default:
		return "", fmt.Errorf("unsupported snapshot scenario %q", scenario)
	}
}

func selectSnapshotReferences(snapshot configv1.RunSnapshot) (*configv1.ResolvedReference, *configv1.ResolvedReference, *configv1.ResolvedReference) {
	var primaryReference *configv1.ResolvedReference
	var graftReference *configv1.ResolvedReference
	var hostReference *configv1.ResolvedReference
	for referenceIndex := range snapshot.References.Resolved {
		reference := &snapshot.References.Resolved[referenceIndex]
		switch reference.Role {
		case configv1.ReferenceRolePrimary:
			primaryReference = reference
		case configv1.ReferenceRoleGraft:
			graftReference = reference
		case configv1.ReferenceRoleHost, configv1.ReferenceRoleSecondary:
			hostReference = reference
		}
	}
	if primaryReference == nil {
		primaryReference = graftReference
	}
	if graftReference == nil {
		graftReference = primaryReference
	}
	return primaryReference, graftReference, hostReference
}

func snapshotDirectories(snapshot configv1.RunSnapshot, primaryReference, graftReference, hostReference *configv1.ResolvedReference) config.DirectoryConfig {
	qualityControlDirectory := filepath.Join(snapshot.Paths.Work, "QC")
	directories := config.DirectoryConfig{
		Base:            snapshot.Paths.Work,
		Work:            snapshot.Paths.Work,
		Workflow:        snapshot.Paths.Work,
		Analysis:        snapshot.Paths.Results,
		Config:          snapshot.Project.Root,
		SIDLog:          snapshot.Paths.Logs,
		MethylationCall: filepath.Join(snapshot.Paths.Work, "mCall"),
		Qualimap:        filepath.Join(qualityControlDirectory, "qualimap"),
		BetaMatrix:      filepath.Join(snapshot.Paths.Results, "methylation"),
		QCSummary:       filepath.Join(snapshot.Paths.Results, "qc"),
		QC: config.QCConfig{
			Main:   qualityControlDirectory,
			Before: filepath.Join(snapshot.Paths.Work, "fastqc_raw"),
			After:  filepath.Join(snapshot.Paths.Work, "fastqc_clean"),
		},
		BSMAP: config.BSMAPConfig{Main: filepath.Join(snapshot.Paths.Work, "bsmap")},
	}
	if primaryReference != nil && snapshot.Workflow.Scenario == configv1.ScenarioRNASeq {
		directories.MethylationCall = filepath.Join(snapshot.Paths.Work, "expression")
	}
	if graftReference != nil && hostReference != nil {
		directories.MethylationCall = filepath.Join(snapshot.Paths.Work, "mCall")
	}
	return directories
}

func snapshotStepResources(resources configv1.ProjectResources) map[int]*config.StepResource {
	stepResources := make(map[int]*config.StepResource, len(resources.Phases)+3)
	if resources.Defaults != (configv1.ResourceSpec{}) {
		defaultResource := resourceSpecToStepResource(resources.Defaults)
		for _, stepNumber := range []int{1, 2, 3} {
			stepResources[stepNumber] = cloneStepResource(defaultResource)
		}
	}
	for phase, resource := range resources.Phases {
		step, ok := compatibilityStepNumber(phase)
		if !ok {
			continue
		}
		stepResources[step] = resourceSpecToStepResource(resource)
	}
	return stepResources
}

func compatibilityStepNumber(phase string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "step1", "ingest", "qc_raw", "prepare":
		return 1, true
	case "step2", "separate", "align":
		return 2, true
	case "step2-check":
		return 102, true
	case "step3", "quantify":
		return 3, true
	case "step3-check", "qc_final", "publish":
		return 103, true
	default:
		return 0, false
	}
}

func resourceSpecToStepResource(resource configv1.ResourceSpec) *config.StepResource {
	return &config.StepResource{
		Cores:     resource.Cores,
		Memory:    resource.Memory,
		Time:      resource.Time,
		Partition: resource.Partition,
	}
}

func cloneStepResource(resource *config.StepResource) *config.StepResource {
	if resource == nil {
		return nil
	}
	copy := *resource
	return &copy
}

func sampleIDs(samples []configv1.SampleRecord) []string {
	ids := make([]string, 0, len(samples))
	for _, sample := range samples {
		ids = append(ids, sample.ID)
	}
	return ids
}

func snapshotGroupLevelCount(samples []configv1.SampleRecord) int {
	groupNames := make(map[string]struct{}, len(samples))
	for _, sample := range samples {
		groupName := strings.TrimSpace(sample.Group)
		if groupName != "" {
			groupNames[groupName] = struct{}{}
		}
	}
	return len(groupNames)
}

func seq2matSpeciesForReference(reference *configv1.ResolvedReference) (string, error) {
	normalizedOrganism := strings.ToLower(strings.TrimSpace(reference.Organism))
	switch normalizedOrganism {
	case "human", "homo sapiens":
		return "human", nil
	case "mouse", "mus musculus":
		return "mouse", nil
	default:
		return "", fmt.Errorf(
			"reference %s@%s has unsupported organism %q for seq2mat",
			reference.ID,
			reference.Release,
			reference.Organism,
		)
	}
}

func snapshotSamples(samples []configv1.SampleRecord) []config.SampleConfig {
	convertedSamples := make([]config.SampleConfig, 0, len(samples))
	for _, sample := range samples {
		convertedSamples = append(convertedSamples, config.SampleConfig{
			Name: sample.ID,
			R1:   sample.R1,
			R2:   sample.R2,
		})
	}
	return convertedSamples
}

func selectReferenceIndex(reference *configv1.ResolvedReference, mode string) string {
	if reference == nil {
		return ""
	}
	preferredType := "bismark"
	if mode == "RNASEQ" {
		preferredType = "star"
	}
	for _, index := range reference.Indexes {
		if strings.EqualFold(index.Type, preferredType) {
			if strings.EqualFold(index.Type, "bismark") {
				return bismarkExecutionGenomeDirectory(index.Path)
			}
			return index.Path
		}
	}
	if len(reference.Indexes) > 0 {
		return reference.Indexes[0].Path
	}
	return ""
}

func bismarkExecutionGenomeDirectory(indexRoot string) string {
	if strings.TrimSpace(indexRoot) == "" {
		return ""
	}
	return filepath.Join(indexRoot, "genome")
}

func selectReferenceAnnotation(reference *configv1.ResolvedReference, annotationType string) string {
	if reference == nil {
		return ""
	}
	for _, annotation := range reference.Annotations {
		if strings.EqualFold(annotation.Type, annotationType) {
			return annotation.Path
		}
	}
	return ""
}
