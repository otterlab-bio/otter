package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMarshalWithMapstructureTagsUsesLegacyKeys(t *testing.T) {
	configuration := &OtterConfig{
		Workflow: WorkflowConfig{UserID: "cohort", JobID: "run-1"},
		Output:   OutputConfig{BaseDir: "/results", WorkflowDir: "/work"},
		StepResources: map[int]*StepResource{
			2: {Cores: 8, Memory: "16GiB"},
		},
	}
	encoded, err := MarshalWithMapstructureTags(configuration)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "userid: cohort") || !strings.Contains(string(encoded), "base_dir: /results") || !strings.Contains(string(encoded), "step_resources:") {
		t.Fatalf("legacy mapstructure keys were not encoded:\n%s", encoded)
	}
	var decoded map[string]interface{}
	if err := yaml.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["workflow"]; !ok {
		t.Fatal("workflow key missing from compatibility YAML")
	}
}
