package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/e2esupport"
)

// The tests here cover the executor boundary and the run-resolution
// convenience flag. They are separate from create_v1_test.go because they
// exercise "otter run", not the authoring commands.

// TestLegacyConfigMigrateProducesResolvableProject covers the supported path
// for pre-existing legacy projects: a config/otter.yaml that already exists in
// the wild must survive "otter config migrate" and resolve to a snakemake
// snapshot. Legacy authoring has been removed, so the fixture is written
// directly, the way an old checkout already carries it.
func TestLegacyConfigMigrateProducesResolvableProject(t *testing.T) {
	projectRoot := t.TempDir()
	fastqDirectory := filepath.Join(projectRoot, "fastq")
	if err := os.MkdirAll(fastqDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sampleName := range []string{"sample1"} {
		writeTestFile(t, filepath.Join(fastqDirectory, sampleName+"_R1.fastq.gz"), "@"+sampleName+"/1\nACGT\n+\nIIII\n")
		writeTestFile(t, filepath.Join(fastqDirectory, sampleName+"_R2.fastq.gz"), "@"+sampleName+"/2\nACGT\n+\nIIII\n")
	}

	legacyConfigPath := filepath.Join(projectRoot, "userspace", "legacy-demo", "config", "otter.yaml")
	writeTestFile(t, legacyConfigPath, "# otter Analysis Project Configuration (pre-existing legacy fixture)\n"+
		"workflow:\n"+
		"    jobid: legacy-demo\n"+
		"    species:\n"+
		"        graft: human\n"+
		"        host: \"\"\n"+
		"        name:\n"+
		"            - human\n"+
		"    samples:\n"+
		"        - name: sample1\n"+
		"          r1: "+filepath.Join(fastqDirectory, "sample1_R1.fastq.gz")+"\n"+
		"          r2: "+filepath.Join(fastqDirectory, "sample1_R2.fastq.gz")+"\n"+
		"input:\n"+
		"    fastq_dir: "+fastqDirectory+"\n"+
		"    suffix: _R1.fastq.gz\n"+
		"    suffix2: _R2.fastq.gz\n"+
		"metadata:\n"+
		"    sample_ids:\n"+
		"        - sample1\n"+
		"SIDs:\n"+
		"    - sample1\n"+
		"mode: RRBS\n")

	// Migration must leave behind a resolvable project, so it needs a registry
	// to lock the declared references against.
	registryRoot := filepath.Join(projectRoot, "registry")
	if _, err := e2esupport.WriteStubRelease(e2esupport.StubReleaseRequest{
		RegistryRoot: registryRoot,
		ReferenceID:  "hg19",
		Release:      "hg19-legacy",
		Organism:     "Homo sapiens",
		Assembly:     "hg19",
	}); err != nil {
		t.Fatalf("write stub release: %v", err)
	}

	// Migration produces project intent; it needs a canonical project root for
	// the pinned workflow assets that the resolver digests.
	migratedRoot := filepath.Join(projectRoot, "migrated-project")
	if initOutput := executeCommand(t, rootCmd, "init", migratedRoot); initOutput.exitCode != 0 {
		t.Fatalf("init of the migration target failed: %s%s", initOutput.stdout, initOutput.stderr)
	}
	migrateOutput := executeCommand(t, rootCmd, "config", "migrate",
		"--input", legacyConfigPath,
		"--output", filepath.Join(migratedRoot, "project.yaml"),
		"--reference-primary", "hg19@hg19-legacy",
		"--reference-root", registryRoot,
	)
	if migrateOutput.exitCode != 0 {
		t.Fatalf("config migrate failed (exit %d):\n%s\n%s", migrateOutput.exitCode, migrateOutput.stdout, migrateOutput.stderr)
	}
	migratedProject, err := os.ReadFile(filepath.Join(migratedRoot, "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migratedProject), "schema_version: otter.project/v1") {
		t.Fatalf("migrated project is not canonical v1:\n%s", migratedProject)
	}
	if !strings.Contains(string(migratedProject), "scenario: rrbs") {
		t.Fatalf("migrated project lost the scenario:\n%s", migratedProject)
	}

	// The migration must write the reference lock, or the project it declares
	// complete cannot be resolved.
	if _, err := os.Stat(filepath.Join(migratedRoot, "references.lock.yaml")); err != nil {
		t.Fatalf("migration did not write references.lock.yaml: %v", err)
	}

	// The migrated snapshot must select the snakemake executor: the legacy
	// adapter maps the compatibility track onto Snakemake. This is the
	// "legacy config pairs with snakemake" half of the executor contract.
	resolveOutput := executeCommand(t, rootCmd, "config", "resolve",
		"--project", filepath.Join(migratedRoot, "project.yaml"),
		"--reference-root", registryRoot,
		"--backend", "local",
	)
	if resolveOutput.exitCode != 0 {
		t.Fatalf("resolving the migrated project failed (exit %d):\n%s\n%s", resolveOutput.exitCode, resolveOutput.stdout, resolveOutput.stderr)
	}
	migratedRunYAML := strings.TrimSpace(resolveOutput.stdout)
	migratedSnapshot, err := configv1.LoadRunSnapshot(migratedRunYAML)
	if err != nil {
		t.Fatal(err)
	}
	if migratedSnapshot.Execution.Executor.Value != configv1.ExecutorSnakemake {
		t.Fatalf("migrated snapshot selected executor %q, expected %q",
			migratedSnapshot.Execution.Executor.Value, configv1.ExecutorSnakemake)
	}

	// And the craftmake executor must refuse that snakemake-selected snapshot.
	craftmakeStub := writeCraftmakeEnvelopeStub(t)
	crossExecutor := executeCommand(t, rootCmd, "run",
		"--config", migratedRunYAML,
		"--executor", "craftmake",
		"--phase", "step1",
		"--dry-run", "--foreground",
		"--craftmake-binary", craftmakeStub,
	)
	if crossExecutor.exitCode == 0 {
		t.Fatal("expected craftmake to refuse a snakemake-selected snapshot")
	}
	if !strings.Contains(crossExecutor.stderr, "resolves executor") {
		t.Fatalf("expected executor-mismatch guidance, got:\n%s", crossExecutor.stderr)
	}
}

// TestRunAcceptsRunIDConvenience documents that --run-id expands to the
// canonical snapshot path and refuses an unresolved run.
func TestRunAcceptsRunIDConvenience(t *testing.T) {
	projectRoot, registryRoot, selection := writeCanonicalProjectFixture(t, []string{"S01"})

	createOutput := executeCommand(t, rootCmd, "create",
		"--output", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--mode", "RRBS",
		"--jobid", "cohort-a",
		"--reference-root", registryRoot,
		"--reference-primary", selection,
	)
	if createOutput.exitCode != 0 {
		t.Fatalf("create failed: %s%s", createOutput.stdout, createOutput.stderr)
	}
	resolveOutput := executeCommand(t, rootCmd, "config", "resolve",
		"--project", filepath.Join(projectRoot, "project.yaml"),
		"--reference-root", registryRoot,
		"--backend", "local",
	)
	if resolveOutput.exitCode != 0 {
		t.Fatalf("resolve failed: %s%s", resolveOutput.stdout, resolveOutput.stderr)
	}
	runYAMLPath := strings.TrimSpace(resolveOutput.stdout)
	runID := filepath.Base(filepath.Dir(runYAMLPath))

	craftmakeStub := writeCraftmakeEnvelopeStub(t)
	runOutput := executeCommand(t, rootCmd, "run",
		"--run-id", runID,
		"--project-dir", projectRoot,
		"--executor", "craftmake",
		"--phase", "step1",
		"--dry-run",
		"--foreground",
		"--craftmake-binary", craftmakeStub,
	)
	if runOutput.exitCode != 0 {
		t.Fatalf("run --run-id failed (exit %d):\n%s\n%s", runOutput.exitCode, runOutput.stdout, runOutput.stderr)
	}

	unresolved := executeCommand(t, rootCmd, "run",
		"--run-id", "run-20260101T000000Z-abcdef",
		"--project-dir", projectRoot,
		"--dry-run", "--foreground",
		"--craftmake-binary", craftmakeStub,
	)
	if unresolved.exitCode == 0 {
		t.Fatal("expected an unresolved --run-id to fail")
	}
	if !strings.Contains(unresolved.stderr, "is not resolved in") {
		t.Fatalf("expected guidance about resolving the run first, got:\n%s", unresolved.stderr)
	}

	malformed := executeCommand(t, rootCmd, "run",
		"--run-id", "not-a-run-id",
		"--project-dir", projectRoot,
		"--dry-run", "--foreground",
		"--craftmake-binary", craftmakeStub,
	)
	if malformed.exitCode == 0 || !strings.Contains(malformed.stderr, "run-YYYYMMDDTHHMMSSZ-abcdef") {
		t.Fatalf("expected a run-id format error, got exit=%d stderr=%s", malformed.exitCode, malformed.stderr)
	}
}

// TestCraftmakeRejectsLegacyConfigWithGuidance covers the boundary error: the
// raw snapshot decode failure must be replaced by actionable guidance.
func TestCraftmakeRejectsLegacyConfigWithGuidance(t *testing.T) {
	projectRoot := t.TempDir()
	legacyConfigPath := filepath.Join(projectRoot, "otter.yaml")
	writeTestFile(t, legacyConfigPath, "# legacy\nSIDs:\n  - sample1\nworkflow:\n  jobid: demo\nmode: RRBS\n")

	output := executeCommand(t, rootCmd, "run",
		"--config", legacyConfigPath,
		"--executor", "craftmake",
		"--dry-run", "--foreground",
	)
	if output.exitCode == 0 {
		t.Fatal("expected a legacy config to be rejected by the craftmake boundary")
	}
	for _, expectedGuidance := range []string{
		"legacy compatibility configuration",
		"otter config migrate",
		"--executor snakemake",
	} {
		if !strings.Contains(output.stderr, expectedGuidance) {
			t.Errorf("boundary error is missing guidance %q:\n%s", expectedGuidance, output.stderr)
		}
	}
}
