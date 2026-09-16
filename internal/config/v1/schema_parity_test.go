package v1

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func TestProjectAndRunSchemasMatchStrictGoValidationFixtures(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	testCases := []struct {
		name          string
		schemaPath    string
		fixturePath   string
		fixture       string
		loadWithGo    func(string) error
		shouldBeValid bool
	}{
		{
			name:        "valid project example",
			schemaPath:  filepath.Join(repositoryRoot, "docs", "schema", "otter-config-v1.schema.json"),
			fixturePath: filepath.Join(repositoryRoot, "docs", "examples", "project.v1.yaml"),
			loadWithGo: func(path string) error {
				_, err := LoadProject(path)
				return err
			},
			shouldBeValid: true,
		},
		{
			name:       "PDX project requires graft and host",
			schemaPath: filepath.Join(repositoryRoot, "docs", "schema", "otter-config-v1.schema.json"),
			fixture: `schema_version: otter.project/v1
project:
  id: pdx-cohort
workflow:
  scenario: bs-pdx
execution: {}
samples:
  manifest: samples.tsv
references:
  species:
    - role: primary
      selection: hg38@GRCh38.p14
    - role: secondary
      selection: mm39@GRCm39
`,
			loadWithGo: func(path string) error {
				_, err := LoadProject(path)
				return err
			},
			shouldBeValid: false,
		},
		{
			name:        "valid immutable run example",
			schemaPath:  filepath.Join(repositoryRoot, "docs", "schema", "otter-run-v1.schema.json"),
			fixturePath: filepath.Join(repositoryRoot, "docs", "examples", "run.v1.yaml"),
			loadWithGo: func(path string) error {
				_, err := LoadRunSnapshot(path)
				return err
			},
			shouldBeValid: true,
		},
		{
			name:       "executor phase envelope requires all resource dimensions",
			schemaPath: filepath.Join(repositoryRoot, "docs", "schema", "otter-run-v1.schema.json"),
			fixture: strings.NewReplacer(
				"  resources: {}",
				"  resources:\n    phases:\n      align:\n        cores: 8\n        memory: 32GiB\n        time: \"04:00:00\"\n        partition: compute",
				"parity: {}",
				"parity:\n  policy: executor-phase-envelope/v1",
			).Replace(mustReadFile(t, filepath.Join(repositoryRoot, "docs", "examples", "run.v1.yaml"))),
			loadWithGo: func(path string) error {
				_, err := LoadRunSnapshot(path)
				return err
			},
			shouldBeValid: true,
		},
		{
			name:       "executor phase envelope rejects missing time",
			schemaPath: filepath.Join(repositoryRoot, "docs", "schema", "otter-run-v1.schema.json"),
			fixture: strings.NewReplacer(
				"  resources: {}",
				"  resources:\n    phases:\n      align:\n        cores: 8\n        memory: 32GiB\n        partition: compute",
				"parity: {}",
				"parity:\n  policy: executor-phase-envelope/v1",
			).Replace(mustReadFile(t, filepath.Join(repositoryRoot, "docs", "examples", "run.v1.yaml"))),
			loadWithGo: func(path string) error {
				_, err := LoadRunSnapshot(path)
				return err
			},
			shouldBeValid: false,
		},
		{
			name:       "run rejects malformed identifier",
			schemaPath: filepath.Join(repositoryRoot, "docs", "schema", "otter-run-v1.schema.json"),
			fixture: strings.ReplaceAll(
				mustReadFile(t, filepath.Join(repositoryRoot, "docs", "examples", "run.v1.yaml")),
				"run-20260726T013245Z-kxqjrm",
				"not-a-run-id",
			),
			loadWithGo: func(path string) error {
				_, err := LoadRunSnapshot(path)
				return err
			},
			shouldBeValid: false,
		},
		{
			name:       "run rejects unknown resolved resource field",
			schemaPath: filepath.Join(repositoryRoot, "docs", "schema", "otter-run-v1.schema.json"),
			fixture: strings.Replace(
				mustReadFile(t, filepath.Join(repositoryRoot, "docs", "examples", "run.v1.yaml")),
				"  resources: {}",
				"  resources:\n    defaults:\n      unsupported: true",
				1,
			),
			loadWithGo: func(path string) error {
				_, err := LoadRunSnapshot(path)
				return err
			},
			shouldBeValid: false,
		},
		{
			name:       "run rejects unknown backend evidence field",
			schemaPath: filepath.Join(repositoryRoot, "docs", "schema", "otter-run-v1.schema.json"),
			fixture: strings.Replace(
				mustReadFile(t, filepath.Join(repositoryRoot, "docs", "examples", "run.v1.yaml")),
				"      cluster: production",
				"      cluster: production\n      unsupported: true",
				1,
			),
			loadWithGo: func(path string) error {
				_, err := LoadRunSnapshot(path)
				return err
			},
			shouldBeValid: false,
		},
		{
			name:       "run rejects malformed resolved reference selection",
			schemaPath: filepath.Join(repositoryRoot, "docs", "schema", "otter-run-v1.schema.json"),
			fixture: strings.Replace(
				mustReadFile(t, filepath.Join(repositoryRoot, "docs", "examples", "run.v1.yaml")),
				"    primary: hg38@GRCh38.p14",
				"    primary: malformed-selection",
				1,
			),
			loadWithGo: func(path string) error {
				_, err := LoadRunSnapshot(path)
				return err
			},
			shouldBeValid: false,
		},
		{
			name:       "SLURM run requires resolved SLURM resources",
			schemaPath: filepath.Join(repositoryRoot, "docs", "schema", "otter-run-v1.schema.json"),
			fixture: strings.Replace(
				mustReadFile(t, filepath.Join(repositoryRoot, "docs", "examples", "run.v1.yaml")),
				"  slurm:\n    partition:\n      value: compute\n      source: profile\n    account:\n      value: genomics\n      source: profile\n    qos:\n      value: normal\n      source: profile\n    max_jobs:\n      value: 64\n      source: profile\n    default_time:\n      value: \"24:00:00\"\n      source: profile\n    scratch_root:\n      value: /scratch/otter\n      source: profile\n\n",
				"",
				1,
			),
			loadWithGo: func(path string) error {
				_, err := LoadRunSnapshot(path)
				return err
			},
			shouldBeValid: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := testCase.fixture
			if testCase.fixturePath != "" {
				fixture = mustReadFile(t, testCase.fixturePath)
			}
			fixturePath := filepath.Join(t.TempDir(), "fixture.yaml")
			if err := os.WriteFile(fixturePath, []byte(fixture), 0o600); err != nil {
				t.Fatal(err)
			}

			schema := compileSchema(t, testCase.schemaPath)
			fixtureDocument := decodeYAMLFixture(t, fixture)
			schemaError := schema.Validate(fixtureDocument)
			goError := testCase.loadWithGo(fixturePath)
			if (schemaError == nil) != testCase.shouldBeValid {
				t.Fatalf("schema validation error = %v, want valid=%t", schemaError, testCase.shouldBeValid)
			}
			if (goError == nil) != testCase.shouldBeValid {
				t.Fatalf("Go validation error = %v, want valid=%t", goError, testCase.shouldBeValid)
			}
		})
	}
}

func compileSchema(t *testing.T, schemaPath string) *jsonschema.Schema {
	t.Helper()
	schemaContent, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var schemaDocument any
	if err := json.Unmarshal(schemaContent, &schemaDocument); err != nil {
		t.Fatalf("decode JSON Schema: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	schemaURL := "file://" + schemaPath
	if err := compiler.AddResource(schemaURL, schemaDocument); err != nil {
		t.Fatalf("add JSON Schema resource: %v", err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		t.Fatalf("compile JSON Schema: %v", err)
	}
	return schema
}

func decodeYAMLFixture(t *testing.T, fixture string) any {
	t.Helper()
	var document any
	decoder := yaml.NewDecoder(bytes.NewBufferString(fixture))
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return document
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
