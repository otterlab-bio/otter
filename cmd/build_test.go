package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/e2esupport"
	"github.com/otterlab-bio/otter/internal/projectlayout"
)

// TestBuildMatchesManualChain is the contract this command exists to satisfy:
// "otter build" must produce exactly the project and snapshot that the manual
// init/create/resolve chain produces, so the shortcut cannot become a second,
// divergent authoring path.
func TestBuildMatchesManualChain(t *testing.T) {
	manualRoot, registryRoot, selection := writeCanonicalProjectFixture(t, []string{"S01", "S02"})

	// The manual chain, using the same flags the build leg will use.
	if output := executeCommand(t, rootCmd, "create",
		"--output", manualRoot,
		"--fastq", filepath.Join(manualRoot, "data"),
		"--pdata", filepath.Join(manualRoot, "pdata.csv"),
		"--mode", "RRBS",
		"--jobid", "cohort-a",
		"--reference-root", registryRoot,
		"--reference-primary", selection,
	); output.exitCode != 0 {
		t.Fatalf("manual create failed (exit %d):\n%s\n%s", output.exitCode, output.stdout, output.stderr)
	}
	manualResolve := executeCommand(t, rootCmd, "config", "resolve",
		"--project", filepath.Join(manualRoot, "project.yaml"),
		"--reference-root", registryRoot,
		"--backend", "local",
	)
	if manualResolve.exitCode != 0 {
		t.Fatalf("manual resolve failed (exit %d):\n%s\n%s", manualResolve.exitCode, manualResolve.stdout, manualResolve.stderr)
	}
	manualSnapshot := readSnapshot(t, strings.TrimSpace(manualResolve.stdout))

	// The build chain, into its own directory with its own FASTQ copy.
	buildRoot := t.TempDir()
	copyFixtureInputs(t, manualRoot, buildRoot)
	buildOutput := executeCommand(t, rootCmd, "build",
		"--project-root", buildRoot,
		"--fastq", filepath.Join(buildRoot, "data"),
		"--pdata", filepath.Join(buildRoot, "pdata.csv"),
		"--mode", "RRBS",
		"--jobid", "cohort-a",
		"--reference-root", registryRoot,
		"--reference-primary", selection,
		"--backend", "local",
	)
	if buildOutput.exitCode != 0 {
		t.Fatalf("build failed (exit %d):\n%s\n%s", buildOutput.exitCode, buildOutput.stdout, buildOutput.stderr)
	}
	buildSnapshotPath := lastNonEmptyLine(buildOutput.stdout)
	if !strings.HasSuffix(buildSnapshotPath, "run.yaml") {
		t.Fatalf("build did not report a run.yaml path, got %q\nstdout:\n%s", buildSnapshotPath, buildOutput.stdout)
	}
	buildSnapshot := readSnapshot(t, buildSnapshotPath)

	// The authoring artifacts must be byte-identical: same project intent, same
	// locked reference digest, same samples. A difference here means the two
	// paths disagree about what a project is.
	for _, artifact := range []string{"project.yaml", "samples.tsv", "references.lock.yaml"} {
		manualBytes, err := os.ReadFile(filepath.Join(manualRoot, artifact))
		if err != nil {
			t.Fatalf("read manual %s: %v", artifact, err)
		}
		buildBytes, err := os.ReadFile(filepath.Join(buildRoot, artifact))
		if err != nil {
			t.Fatalf("read build %s: %v", artifact, err)
		}
		if string(manualBytes) != string(buildBytes) {
			t.Fatalf("%s differs between the manual chain and otter build:\n--- manual ---\n%s\n--- build ---\n%s", artifact, manualBytes, buildBytes)
		}
	}

	// The snapshot must agree on everything except the run directory, which is
	// generated per run and therefore cannot match.
	if manualSnapshot.Project.ID != buildSnapshot.Project.ID {
		t.Fatalf("project id differs: manual %q build %q", manualSnapshot.Project.ID, buildSnapshot.Project.ID)
	}
	if manualSnapshot.Workflow.Scenario != buildSnapshot.Workflow.Scenario {
		t.Fatalf("scenario differs: manual %q build %q", manualSnapshot.Workflow.Scenario, buildSnapshot.Workflow.Scenario)
	}
	if manualSnapshot.Execution.Executor.Value != buildSnapshot.Execution.Executor.Value {
		t.Fatalf("executor differs: manual %q build %q", manualSnapshot.Execution.Executor.Value, buildSnapshot.Execution.Executor.Value)
	}
	if manualSnapshot.Run.Immutable != buildSnapshot.Run.Immutable {
		t.Fatalf("immutability differs: manual %v build %v", manualSnapshot.Run.Immutable, buildSnapshot.Run.Immutable)
	}
}

// TestBuildDefaultsToCraftmake pins the executor default. Craftmake is the
// execution layer under development, so the shortcut must not silently record
// the compatibility executor for a new project.
func TestBuildDefaultsToCraftmake(t *testing.T) {
	projectRoot, registryRoot, selection := writeCanonicalProjectFixture(t, []string{"S01"})

	output := executeCommand(t, rootCmd, "build",
		"--project-root", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--pdata", filepath.Join(projectRoot, "pdata.csv"),
		"--mode", "RRBS",
		"--reference-root", registryRoot,
		"--reference-primary", selection,
		"--backend", "local",
	)
	if output.exitCode != 0 {
		t.Fatalf("build failed (exit %d):\n%s\n%s", output.exitCode, output.stdout, output.stderr)
	}

	snapshot := readSnapshot(t, lastNonEmptyLine(output.stdout))
	if snapshot.Execution.Executor.Value != configv1.ExecutorCraftmake {
		t.Fatalf("build defaulted to executor %q, want %q", snapshot.Execution.Executor.Value, configv1.ExecutorCraftmake)
	}
}

// TestBuildAcceptsSnakemakeExecutor covers the documented escape hatch. The
// Snakemake executor is a compatibility surface for the canonical snapshot, so
// selecting it must change only the recorded executor.
func TestBuildAcceptsSnakemakeExecutor(t *testing.T) {
	projectRoot, registryRoot, selection := writeCanonicalProjectFixture(t, []string{"S01"})

	output := executeCommand(t, rootCmd, "build",
		"--project-root", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--pdata", filepath.Join(projectRoot, "pdata.csv"),
		"--mode", "RRBS",
		"--reference-root", registryRoot,
		"--reference-primary", selection,
		"--executor", "snakemake",
		"--backend", "local",
	)
	if output.exitCode != 0 {
		t.Fatalf("build --executor snakemake failed (exit %d):\n%s\n%s", output.exitCode, output.stdout, output.stderr)
	}

	snapshot := readSnapshot(t, lastNonEmptyLine(output.stdout))
	if snapshot.Execution.Executor.Value != configv1.ExecutorSnakemake {
		t.Fatalf("snapshot executor is %q, want %q", snapshot.Execution.Executor.Value, configv1.ExecutorSnakemake)
	}
	// The snapshot is the same boundary for both executors, so selecting
	// snakemake must not change which project the snapshot describes.
	project, err := configv1.LoadProject(filepath.Join(projectRoot, "project.yaml"))
	if err != nil {
		t.Fatalf("load authored project: %v", err)
	}
	if project.Execution.Executor != configv1.ExecutorCraftmake {
		t.Fatalf(
			"project.yaml records executor %q; the executor choice belongs to the run snapshot and must not be written into the project",
			project.Execution.Executor,
		)
	}
}

// TestBuildPinsRulesUnderWorkflows is the layout contract for the include
// resolution: the pinned rules must sit beside the Snakefiles so a Snakefile's
// own include: directives resolve without writing anything to the project root.
func TestBuildPinsRulesUnderWorkflows(t *testing.T) {
	projectRoot, registryRoot, selection := writeBuildInputFixture(t, []string{"S01"})

	output := executeCommand(t, rootCmd, "build",
		"--project-root", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--pdata", filepath.Join(projectRoot, "pdata.csv"),
		"--mode", "RRBS",
		"--reference-root", registryRoot,
		"--reference-primary", selection,
		"--backend", "local",
	)
	if output.exitCode != 0 {
		t.Fatalf("build failed (exit %d):\n%s\n%s", output.exitCode, output.stdout, output.stderr)
	}

	rulesDirectory := filepath.Join(projectRoot, "workflows", "rules")
	if info, err := os.Stat(rulesDirectory); err != nil || !info.IsDir() {
		t.Fatalf("build did not pin workflows/rules: %v", err)
	}
	// The whole point of the move: nothing named rules/ may reappear at the
	// project root, because that is what pollutes a canonical project.
	if _, err := os.Stat(filepath.Join(projectRoot, "rules")); !os.IsNotExist(err) {
		t.Fatalf("build wrote a project-root rules/ directory (err=%v); rules must stay under workflows/", err)
	}

	// The lock must digest the new path, or a resolve would checksum a
	// directory that no longer holds the rules.
	lock, err := projectlayout.ReadProjectLock(projectRoot)
	if err != nil {
		t.Fatalf("read project lock: %v", err)
	}
	roles := make(map[string]bool, len(lock.Assets))
	for _, assetSet := range lock.Assets {
		roles[assetSet.Role] = true
	}
	if !roles[filepath.Join("workflows", "rules")] {
		t.Fatalf("project.lock.yaml does not pin the workflows/rules role; roles: %v", roles)
	}
	if roles["rules"] {
		t.Fatalf("project.lock.yaml still pins a project-root rules role; roles: %v", roles)
	}
}

// TestBuildRefusesSecondAuthoring covers a second build in an already-authored
// directory.
//
// "otter create" never overwrites project.yaml, because rewriting project
// intent in place would silently invalidate every snapshot already resolved
// from it. The pipeline must name that up front and leave the existing project
// untouched, rather than failing two stages in with a freshly re-pinned lock.
func TestBuildRefusesSecondAuthoring(t *testing.T) {
	projectRoot, registryRoot, selection := writeBuildInputFixture(t, []string{"S01"})

	buildArguments := []string{
		"build",
		"--project-root", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--pdata", filepath.Join(projectRoot, "pdata.csv"),
		"--mode", "RRBS",
		"--reference-root", registryRoot,
		"--reference-primary", selection,
		"--backend", "local",
	}

	first := executeCommand(t, rootCmd, buildArguments...)
	if first.exitCode != 0 {
		t.Fatalf("first build failed (exit %d):\n%s\n%s", first.exitCode, first.stdout, first.stderr)
	}

	lockPath := filepath.Join(projectRoot, "project.lock.yaml")
	lockBefore, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	projectBefore, err := os.ReadFile(filepath.Join(projectRoot, "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	second := executeCommand(t, rootCmd, buildArguments...)
	if second.exitCode == 0 {
		t.Fatalf("second build re-authored an existing project:\n%s\n%s", second.stdout, second.stderr)
	}
	if !strings.Contains(second.stderr, "already exists") {
		t.Fatalf("refusal did not explain that the project is already authored:\n%s", second.stderr)
	}
	// The refusal must point at the command that does what the caller wants.
	if !strings.Contains(second.stderr, "otter config resolve") {
		t.Fatalf("refusal did not point at `otter config resolve` for a further run:\n%s", second.stderr)
	}

	// Nothing may have been rewritten before the refusal.
	lockAfter, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(lockBefore) != string(lockAfter) {
		t.Fatal("the refused second build rewrote project.lock.yaml; it must check before re-pinning assets")
	}
	projectAfter, err := os.ReadFile(filepath.Join(projectRoot, "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(projectBefore) != string(projectAfter) {
		t.Fatal("the refused second build rewrote project.yaml")
	}

	// The refusal must arrive before the second run directory is created, so a
	// rejected invocation leaves no half-resolved run behind.
	entries, err := os.ReadDir(filepath.Join(projectRoot, "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		runDirectories := make([]string, 0, len(entries))
		for _, entry := range entries {
			runDirectories = append(runDirectories, entry.Name())
		}
		t.Fatalf("second build left %d run directories (%v), want the first build's single run", len(entries), runDirectories)
	}
}

// TestBuildRefusesLegacyProject keeps the tracks mutually exclusive. A legacy
// directory carries config/otter.yaml, not project.yaml, so authoring a
// canonical project into it would produce a directory that neither track can
// resolve.
func TestBuildRefusesLegacyProject(t *testing.T) {
	projectRoot := t.TempDir()
	// The legacy track marker is what makes a directory legacy; write it
	// directly so this test does not depend on the legacy init path.
	writeTestFile(t, filepath.Join(projectRoot, ".otter", "assets.manifest.json"), "{}\n")

	output := executeCommand(t, rootCmd, "build",
		"--project-root", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--mode", "RRBS",
	)
	if output.exitCode == 0 {
		t.Fatalf("build accepted a legacy compatibility project:\n%s\n%s", output.stdout, output.stderr)
	}
	if !strings.Contains(output.stderr, "legacy compatibility project") {
		t.Fatalf("refusal did not explain the track mismatch:\n%s", output.stderr)
	}
}

// TestBuildRefusesMissingFastq proves the pipeline fails at the authoring stage
// rather than leaving a half-initialised project behind with a snapshot that
// describes samples it never read.
func TestBuildRefusesMissingFastq(t *testing.T) {
	projectRoot := t.TempDir()
	registryRoot := filepath.Join(t.TempDir(), "registry")
	if _, err := e2esupport.WriteStubRelease(e2esupport.StubReleaseRequest{
		RegistryRoot: registryRoot,
		ReferenceID:  "hg19",
		Release:      "hg19-legacy",
		Organism:     "Homo sapiens",
		Assembly:     "hg19",
	}); err != nil {
		t.Fatalf("write stub release: %v", err)
	}
	selection := "hg19@hg19-legacy"

	output := executeCommand(t, rootCmd, "build",
		"--project-root", projectRoot,
		"--fastq", filepath.Join(projectRoot, "absent-fastq"),
		"--mode", "RRBS",
		"--reference-root", registryRoot,
		"--reference-primary", selection,
	)
	if output.exitCode == 0 {
		t.Fatalf("build accepted a missing FASTQ directory:\n%s\n%s", output.stdout, output.stderr)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "project.yaml")); !os.IsNotExist(err) {
		t.Fatalf("build wrote project.yaml despite failing sample intake (err=%v)", err)
	}
}

// writeBuildInputFixture lays out FASTQ inputs, a pdata file, and a stub
// reference registry in a directory that is NOT yet a canonical project.
//
// It deliberately does not write the track marker or the pinned assets: these
// tests need "otter build" to run its real init stage, which is the only way to
// observe the artifact layout and the lock digests the command produces.
func writeBuildInputFixture(t *testing.T, fastqNames []string) (projectRoot, registryRoot, selection string) {
	t.Helper()
	projectRoot = t.TempDir()
	registryRoot = filepath.Join(t.TempDir(), "registry")

	if _, err := e2esupport.WriteStubRelease(e2esupport.StubReleaseRequest{
		RegistryRoot: registryRoot,
		ReferenceID:  "hg19",
		Release:      "hg19-legacy",
		Organism:     "Homo sapiens",
		Assembly:     "hg19",
	}); err != nil {
		t.Fatalf("write stub release: %v", err)
	}
	selection = "hg19@hg19-legacy"

	fastqDirectory := filepath.Join(projectRoot, "data")
	if err := os.MkdirAll(fastqDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	pdataLines := []string{"sampleid,inline_barcode_sequence,condition"}
	for _, name := range fastqNames {
		content := "@" + name + "/1\nACGTACGT\n+\nIIIIIIII\n"
		writeTestFile(t, filepath.Join(fastqDirectory, name+"_R1.fastq.gz"), content)
		writeTestFile(t, filepath.Join(fastqDirectory, name+"_R2.fastq.gz"), content)
		pdataLines = append(pdataLines, name+",,case")
	}
	writeTestFile(t, filepath.Join(projectRoot, "pdata.csv"), strings.Join(pdataLines, "\n")+"\n")
	return projectRoot, registryRoot, selection
}

// readSnapshot loads a run snapshot from the path a command printed.
func readSnapshot(t *testing.T, snapshotPath string) configv1.RunSnapshot {
	t.Helper()
	snapshot, err := configv1.LoadRunSnapshot(snapshotPath)
	if err != nil {
		t.Fatalf("load run snapshot %q: %v", snapshotPath, err)
	}
	return snapshot
}

// lastNonEmptyLine returns the final non-blank stdout line, which is where the
// resolve stage reports the snapshot it wrote.
func lastNonEmptyLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if trimmed := strings.TrimSpace(lines[index]); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// copyFixtureInputs gives the build leg its own FASTQ and pdata inputs so the
// two chains never read from the same project directory.
func copyFixtureInputs(t *testing.T, sourceRoot, destinationRoot string) {
	t.Helper()
	if err := os.MkdirAll(destinationRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"data", "pdata.csv"} {
		sourcePath := filepath.Join(sourceRoot, name)
		destinationPath := filepath.Join(destinationRoot, name)
		if info, err := os.Stat(sourcePath); err == nil && info.IsDir() {
			entries, err := os.ReadDir(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(destinationPath, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				content, err := os.ReadFile(filepath.Join(sourcePath, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(destinationPath, entry.Name()), content, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			continue
		}
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destinationPath, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
