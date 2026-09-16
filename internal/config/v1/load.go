package v1

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type yamlDocument interface {
	ProjectConfig | ReferencesLock | ReferenceDefinition | RunSnapshot
}

func LoadProject(path string) (ProjectConfig, error) {
	project, _, err := LoadProjectWithSources(path)
	return project, err
}

func LoadProjectWithSources(path string) (ProjectConfig, ProjectFieldSources, error) {
	project, err := decodeStrictYAML[ProjectConfig](path)
	if err != nil {
		return ProjectConfig{}, ProjectFieldSources{}, fmt.Errorf("load project configuration: %w", err)
	}
	sources := ProjectFieldSources{
		Toolchain: sourceForPresence(project.Workflow.Toolchain != ""),
		Executor:  sourceForPresence(project.Execution.Executor != ""),
		Backend:   sourceForPresence(project.Execution.Backend != ""),
		Site:      sourceForPresence(project.Execution.Site != ""),
	}
	ApplyProjectDefaults(&project)
	if err := ValidateProject(project); err != nil {
		return ProjectConfig{}, ProjectFieldSources{}, fmt.Errorf("validate project configuration: %w", err)
	}
	return project, sources, nil
}

func sourceForPresence(present bool) ValueSource {
	if present {
		return SourceProject
	}
	return SourceDefault
}

func LoadReferencesLock(path string) (ReferencesLock, error) {
	lock, err := decodeStrictYAML[ReferencesLock](path)
	if err != nil {
		return ReferencesLock{}, fmt.Errorf("load references lock: %w", err)
	}
	if err := ValidateReferencesLock(lock); err != nil {
		return ReferencesLock{}, fmt.Errorf("validate references lock: %w", err)
	}
	return lock, nil
}

func LoadReferenceDefinition(path string) (ReferenceDefinition, error) {
	definition, err := decodeStrictYAML[ReferenceDefinition](path)
	if err != nil {
		return ReferenceDefinition{}, fmt.Errorf("load reference definition: %w", err)
	}
	if err := ValidateReferenceDefinition(definition); err != nil {
		return ReferenceDefinition{}, fmt.Errorf("validate reference definition: %w", err)
	}
	return definition, nil
}

func LoadRunSnapshot(path string) (RunSnapshot, error) {
	snapshot, err := decodeStrictYAML[RunSnapshot](path)
	if err != nil {
		return RunSnapshot{}, fmt.Errorf("load run snapshot: %w", err)
	}
	if err := ValidateRunSnapshot(snapshot); err != nil {
		return RunSnapshot{}, fmt.Errorf("validate run snapshot: %w", err)
	}
	return snapshot, nil
}

func MarshalProject(project ProjectConfig) ([]byte, error) {
	return marshalYAML(project)
}

func MarshalRunSnapshot(snapshot RunSnapshot) ([]byte, error) {
	return marshalYAML(snapshot)
}

func marshalYAML[T yamlDocument](document T) ([]byte, error) {
	encoded, err := yaml.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("marshal YAML: %w", err)
	}
	return encoded, nil
}

func decodeStrictYAML[T yamlDocument](path string) (T, error) {
	var document T
	file, err := os.Open(path)
	if err != nil {
		return document, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return document, fmt.Errorf("decode %s: %w", path, err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return document, fmt.Errorf("decode %s: multiple YAML documents are not supported", path)
		}
		return document, fmt.Errorf("decode %s: %w", path, err)
	}
	return document, nil
}
