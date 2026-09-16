package legacy

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	legacyconfig "github.com/otterlab-bio/otter/internal/config"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

type MigrationOptions struct {
	ProjectID          string
	PrimaryReference   configv1.ReferenceSelection
	SecondaryReference configv1.ReferenceSelection
	GraftReference     configv1.ReferenceSelection
	HostReference      configv1.ReferenceSelection
}

type MigrationIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type MigrationReport struct {
	Adopted    []string         `json:"adopted"`
	Deprecated []string         `json:"deprecated"`
	Warnings   []MigrationIssue `json:"warnings"`
	Conflicts  []MigrationIssue `json:"conflicts"`
}

func (report MigrationReport) HasConflicts() bool {
	return len(report.Conflicts) > 0
}

type Result struct {
	Project configv1.ProjectConfig
	Samples []configv1.SampleRecord
	Report  MigrationReport
}

func Adapt(configuration legacyconfig.OtterConfig, options MigrationOptions) Result {
	report := MigrationReport{
		Adopted: []string{
			"workflow.mode -> workflow.scenario",
			"engine.type -> execution.backend",
			"legacy samples -> samples.tsv",
			"explicit reference releases -> references",
		},
		Deprecated: []string{
			"legacy OtterConfig layout",
			"implicit species/reference array pairing",
			"engine.type executor/backend coupling",
			"runtime environment overrides",
		},
	}
	projectID := options.ProjectID
	if projectID == "" {
		projectID = configuration.Workflow.JobID
	}
	projectID = normalizeProjectID(projectID)
	if projectID == "" {
		projectID = "migrated-project"
		report.Warnings = append(report.Warnings, MigrationIssue{
			Field:   "project.id",
			Code:    "generated-default",
			Message: "legacy configuration did not provide a valid project ID; used migrated-project",
		})
	}

	scenario, scenarioIssue := mapScenario(configuration)
	if scenarioIssue != nil {
		report.Conflicts = append(report.Conflicts, *scenarioIssue)
	}
	backend := configv1.Backend(strings.ToLower(configuration.Engine.Type))
	if backend == "" {
		backend = configv1.BackendAuto
	}
	if backend != configv1.BackendAuto && backend != configv1.BackendLocal && backend != configv1.BackendSlurm {
		report.Conflicts = append(report.Conflicts, MigrationIssue{
			Field:   "engine.type",
			Code:    "unsupported-value",
			Message: fmt.Sprintf("legacy engine type %q cannot be mapped", configuration.Engine.Type),
		})
		backend = configv1.BackendAuto
	}

	project := configv1.ProjectConfig{
		SchemaVersion: configv1.ProjectSchemaVersion,
		Project:       configv1.ProjectMetadata{ID: projectID},
		Workflow: configv1.ProjectWorkflow{
			Scenario:  scenario,
			Toolchain: configv1.ToolchainLegacyEquivalent,
		},
		Execution: configv1.ProjectExecution{
			Executor: configv1.ExecutorSnakemake,
			Backend:  backend,
			Site:     "auto",
		},
		Samples:       configv1.SamplesDeclaration{Manifest: "samples.tsv"},
		Observability: configv1.ObservabilityConfig{Metrics: true, RetainLogs: true},
	}
	project.References = adaptReferences(configuration, scenario, options, &report)
	sampleRecords := adaptSamples(configuration, &report)

	if !report.HasConflicts() {
		if err := configv1.ValidateProject(project); err != nil {
			report.Conflicts = append(report.Conflicts, MigrationIssue{
				Field:   "project",
				Code:    "canonical-validation",
				Message: err.Error(),
			})
		}
	}
	return Result{Project: project, Samples: sampleRecords, Report: report}
}

func mapScenario(configuration legacyconfig.OtterConfig) (configv1.Scenario, *MigrationIssue) {
	mode := strings.ToUpper(strings.TrimSpace(configuration.Workflow.Mode))
	isPDX := legacyconfig.DetectPDXMode(&configuration) || strings.TrimSpace(configuration.Workflow.Species.Host) != ""
	switch mode {
	case "RRBS":
		if isPDX {
			return configv1.ScenarioBSPDX, nil
		}
		return configv1.ScenarioRRBS, nil
	case "WGBS":
		if isPDX {
			return configv1.ScenarioBSPDX, nil
		}
		return configv1.ScenarioWGBS, nil
	case "RNASEQ":
		if isPDX {
			return configv1.ScenarioRNAPDX, nil
		}
		return configv1.ScenarioRNASeq, nil
	case "BSSEQ":
		return configv1.ScenarioWGBS, &MigrationIssue{
			Field:   "workflow.mode",
			Code:    "ambiguous-mode",
			Message: "legacy BSSEQ does not distinguish RRBS from WGBS; choose a canonical scenario explicitly",
		}
	default:
		return configv1.ScenarioRRBS, &MigrationIssue{
			Field:   "workflow.mode",
			Code:    "unsupported-value",
			Message: fmt.Sprintf("legacy mode %q cannot be mapped", configuration.Workflow.Mode),
		}
	}
}

func adaptReferences(configuration legacyconfig.OtterConfig, scenario configv1.Scenario, options MigrationOptions, report *MigrationReport) configv1.ProjectReferences {
	isPDX := scenario == configv1.ScenarioBSPDX || scenario == configv1.ScenarioRNAPDX
	if isPDX {
		graftSelection := firstReferenceSelection(options.GraftReference, configuration.Workflow.Species.Graft, configuration.Workflow.Species.Primary)
		hostSelection := firstReferenceSelection(options.HostReference, configuration.Workflow.Species.Host, configuration.Workflow.Species.Secondary)
		if graftSelection == "" {
			report.Conflicts = append(report.Conflicts, missingReferenceIssue("graft"))
		}
		if hostSelection == "" {
			report.Conflicts = append(report.Conflicts, missingReferenceIssue("host"))
		}
		return configv1.ProjectReferences{Species: []configv1.SpeciesReference{
			{Role: configv1.ReferenceRoleGraft, Selection: graftSelection},
			{Role: configv1.ReferenceRoleHost, Selection: hostSelection},
		}}
	}

	primarySelection := firstReferenceSelection(options.PrimaryReference, configuration.Workflow.Species.Primary, configuration.Workflow.Species.Graft, configuration.Reference.Genome)
	if primarySelection == "" {
		report.Conflicts = append(report.Conflicts, missingReferenceIssue("primary"))
	}
	secondarySelection := firstReferenceSelection(options.SecondaryReference, configuration.Workflow.Species.Secondary)
	return configv1.ProjectReferences{Primary: primarySelection, Secondary: secondarySelection}
}

func firstReferenceSelection(explicit configv1.ReferenceSelection, legacyValues ...string) configv1.ReferenceSelection {
	if explicit != "" {
		return explicit
	}
	for _, value := range legacyValues {
		selection := configv1.ReferenceSelection(strings.TrimSpace(value))
		if _, _, err := configv1.ParseReferenceSelection(selection); err == nil {
			return selection
		}
	}
	return ""
}

func missingReferenceIssue(role string) MigrationIssue {
	return MigrationIssue{
		Field:   "references." + role,
		Code:    "release-required",
		Message: "legacy paths/species do not identify an immutable registry release; provide " + role + " as id@release",
	}
}

func adaptSamples(configuration legacyconfig.OtterConfig, report *MigrationReport) []configv1.SampleRecord {
	if len(configuration.Workflow.Samples) > 0 {
		records := make([]configv1.SampleRecord, 0, len(configuration.Workflow.Samples))
		for _, sample := range configuration.Workflow.Samples {
			if sample.Name == "" || sample.R1 == "" || sample.R2 == "" {
				report.Conflicts = append(report.Conflicts, MigrationIssue{
					Field:   "workflow.samples",
					Code:    "incomplete-sample",
					Message: "every legacy workflow sample must include name, r1, and r2",
				})
				continue
			}
			records = append(records, configv1.SampleRecord{ID: sample.Name, R1: sample.R1, R2: sample.R2})
		}
		return records
	}
	if len(configuration.Metadata.SampleIDs) == 0 {
		report.Conflicts = append(report.Conflicts, MigrationIssue{
			Field:   "samples",
			Code:    "missing-samples",
			Message: "legacy configuration has no explicit samples or sample IDs",
		})
		return nil
	}
	if configuration.Input.FastqDir == "" || configuration.Input.Suffix1 == "" || configuration.Input.Suffix2 == "" {
		report.Conflicts = append(report.Conflicts, MigrationIssue{
			Field:   "input",
			Code:    "incomplete-fastq-pattern",
			Message: "sample IDs require fastq_dir, suffix, and suffix2 to construct samples.tsv",
		})
		return nil
	}
	records := make([]configv1.SampleRecord, 0, len(configuration.Metadata.SampleIDs))
	for _, sampleID := range configuration.Metadata.SampleIDs {
		records = append(records, configv1.SampleRecord{
			ID: sampleID,
			R1: filepath.Join(configuration.Input.FastqDir, sampleID+configuration.Input.Suffix1),
			R2: filepath.Join(configuration.Input.FastqDir, sampleID+configuration.Input.Suffix2),
		})
	}
	return records
}

func normalizeProjectID(value string) string {
	value = strings.TrimSpace(value)
	invalidCharacters := regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	value = invalidCharacters.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-._")
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}
