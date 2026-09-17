package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/otterlab-bio/otter/internal/reference"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

func TestConfigResolveGoldenPathProducesImmutableRunSnapshot(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")

	writeConfigResolveProjectFixture(t, projectRoot, referenceRoot)

	output := executeCommand(t, rootCmd, "config", "resolve",
		"--project", filepath.Join(projectRoot, "project.yaml"),
		"--reference-root", referenceRoot,
		"--executor", "craftmake",
		"--backend", "local",
	)
	if output.exitCode != 0 {
		t.Fatalf("config resolve failed (exit %d): %s\n%s", output.exitCode, output.stdout, output.stderr)
	}

	runYamlPath := ""
	entries, err := os.ReadDir(filepath.Join(projectRoot, "runs"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected at least one run directory under runs/: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			runYamlPath = filepath.Join(projectRoot, "runs", entry.Name(), "run.yaml")
			break
		}
	}
	if runYamlPath == "" {
		t.Fatal("expected a run.yaml under the created run directory")
	}

	runData, err := os.ReadFile(runYamlPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := yaml.Unmarshal(runData, &snapshot); err != nil {
		t.Fatal(err)
	}
	assertField(t, snapshot, "schema_version", "otter.run/v1")
	runBlock := mustGetBlock(t, snapshot, "run")
	assertField(t, runBlock, "immutable", true)
	projectBlock := mustGetBlock(t, snapshot, "project")
	assertField(t, projectBlock, "id", "cohort-a")
	executionBlock := mustGetBlock(t, snapshot, "execution")
	executorBlock := mustGetBlock(t, executionBlock, "executor")
	assertField(t, executorBlock, "value", "craftmake")
	backendBlock := mustGetBlock(t, executionBlock, "backend")
	assertField(t, backendBlock, "value", "local")

	info, err := os.Stat(runYamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o200 != 0 {
		t.Fatalf("run.yaml must be read-only (0o444), got 0o%o", info.Mode().Perm())
	}
}

func TestConfigResolveRejectsAutoAsResolvedBackendOverride(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")

	writeConfigResolveProjectFixture(t, projectRoot, referenceRoot)

	output := executeCommand(t, rootCmd, "config", "resolve",
		"--project", filepath.Join(projectRoot, "project.yaml"),
		"--reference-root", referenceRoot,
		"--backend", "auto",
	)
	if output.exitCode == 0 {
		t.Fatalf("expected --backend auto to be rejected as a resolved backend override; got stdout=%q stderr=%q", output.stdout, output.stderr)
	}
}

func TestConfigResolveRejectsMissingReferenceRoot(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")
	writeConfigResolveProjectFixture(t, projectRoot, referenceRoot)

	output := executeCommand(t, rootCmd, "config", "resolve",
		"--project", filepath.Join(projectRoot, "project.yaml"),
	)
	if output.exitCode == 0 {
		t.Fatal("expected config resolve without --reference-root to fail")
	}
}

func writeConfigResolveProjectFixture(t *testing.T, projectRoot string, referenceRoot string) {
	t.Helper()
	project := `schema_version: otter.project/v1
project:
  id: cohort-a
workflow:
  scenario: rrbs
execution:
  executor: craftmake
  backend: auto
  site: auto
samples:
  manifest: samples.tsv
references:
  primary: hg38@GRCh38.p14
observability:
  metrics: false
`
	writeTestFile(t, filepath.Join(projectRoot, "project.yaml"), project)
	writeTestFile(t, filepath.Join(projectRoot, "samples.tsv"), "sample_id\tr1\tr2\nS01\tdata/S01_R1.fastq.gz\tdata/S01_R2.fastq.gz\n")
	writeTestFile(t, filepath.Join(projectRoot, "data", "S01_R1.fastq.gz"), "@S01/1\nACGT\n+\nIIII\n")
	writeTestFile(t, filepath.Join(projectRoot, "data", "S01_R2.fastq.gz"), "@S01/2\nTGCA\n+\nIIII\n")
	for _, directoryName := range []string{"workflows", "environments", "schemas"} {
		if err := os.MkdirAll(filepath.Join(projectRoot, directoryName), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "workflows", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(projectRoot, "workflows", "rrbs.yaml"), "name: rrbs\n")
	writeTestFile(t, filepath.Join(projectRoot, "project.lock.yaml"), "schema_version: otter.project.lock/v1\n")
	releaseRoot := filepath.Join(referenceRoot, "genomes", "hg38", "GRCh38.p14")
	fastaContent := ">chr1\nACGT\n"
	fastaDigest := reference.ComputeDigest(fastaContent)
	indexContent := "index\n"
	indexDigest := reference.ComputeDigest(indexContent)
	indexManifestDigest := reference.ComputeDigest("index.bin:" + indexDigest)
	writeTestFile(t, filepath.Join(releaseRoot, "fasta", "genome.fa.gz"), fastaContent)
	writeTestFile(t, filepath.Join(releaseRoot, "fasta", "genome.fa.gz.fai"), "chr1\t4\t0\t4\t5\n")
	writeTestFile(t, filepath.Join(releaseRoot, "indexes", "bismark", "index.bin"), indexContent)
	definition := fmt.Sprintf(`schema_version: otter.reference/v1
reference:
  id: hg38
  release: GRCh38.p14
  organism: Homo sapiens
  assembly: test
assets:
  fasta:
    path: fasta/genome.fa.gz
    sha256: %s
    size_bytes: %d
    fai: fasta/genome.fa.gz.fai
  indexes:
    - type: bismark
      path: indexes/bismark
      reference_fasta_sha256: %s
      tool: bismark
      tool_version: test
      manifest_sha256: %s
compatibility:
  scenarios: [rrbs]
`, fastaDigest, len(fastaContent), fastaDigest, indexManifestDigest)
	writeTestFile(t, filepath.Join(releaseRoot, "reference.yaml"), definition)
	report, err := reference.BuildManifest(releaseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Write(""); err != nil {
		t.Fatal(err)
	}
	manifestData, err := os.ReadFile(filepath.Join(releaseRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	lock := `schema_version: otter.references.lock/v1
references:
  primary:
    id: hg38
    release: GRCh38.p14
    manifest_digest: ` + reference.ComputeDigest(string(manifestData)) + "\n"
	writeTestFile(t, filepath.Join(projectRoot, "references.lock.yaml"), lock)
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type commandOutput struct {
	exitCode int
	stdout   string
	stderr   string
}

func executeCommand(t *testing.T, root *cobra.Command, args ...string) commandOutput {
	t.Helper()
	resetCommandFlags(root)
	root.SetArgs(args)
	stdout := &testWriter{}
	stderr := &testWriter{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = append([]string{"otter"}, args...)
	exitCode := 0
	if err := root.Execute(); err != nil {
		exitCode = 1
	}
	return commandOutput{exitCode: exitCode, stdout: stdout.String(), stderr: stderr.String()}
}

// resetCommandFlags restores every flag in the command tree to its declared
// default and clears the "changed" bookkeeping.
//
// The CLI binds flags to package-level variables, and cobra records which flags
// a caller set. Both persist for the lifetime of the process, so without this a
// value set by one test would silently leak into the next one and produce
// failures that depend on test order.
func resetCommandFlags(command *cobra.Command) {
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		// Slice flags cannot round-trip through their printed default, so only
		// their changed bookkeeping is cleared.
		switch flag.Value.Type() {
		case "stringSlice", "stringArray", "intSlice", "boolSlice", "float64Slice":
		default:
			if flag.Changed {
				if err := flag.Value.Set(flag.DefValue); err == nil {
					flag.Changed = false
				}
			}
		}
		flag.Changed = false
	})
	for _, child := range command.Commands() {
		resetCommandFlags(child)
	}
}

type testWriter struct {
	buf []byte
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *testWriter) String() string {
	return string(w.buf)
}

func assertField(t *testing.T, block map[string]any, key string, expected any) {
	t.Helper()
	actual, exists := block[key]
	if !exists {
		t.Fatalf("expected field %q in block", key)
	}
	if actual != expected {
		t.Fatalf("field %q: got %v, expected %v", key, actual, expected)
	}
}

func mustGetBlock(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	raw, exists := parent[key]
	if !exists {
		t.Fatalf("expected block %q in parent", key)
	}
	block, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected %q to be a map, got %T", key, raw)
	}
	return block
}
