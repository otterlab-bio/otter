package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/otterlab-bio/otter/internal/assets"
	"github.com/otterlab-bio/otter/internal/e2esupport"
	"github.com/otterlab-bio/otter/internal/projectlayout"
)

// TestMain installs a synthetic embedded asset filesystem so "otter init"
// exercises the real pinning code path without depending on the packaged
// assets. The directory names must match assets.V1AssetSets, because those
// names are the contract the canonical resolver digests.
func TestMain(m *testing.M) {
	assets.SetEmbeddedAssets(fstest.MapFS{
		"inst/snakefiles/BeaverBS_step1.snakemake": &fstest.MapFile{Data: []byte("rule step1:\n  shell: 'echo step1'\n")},
		"inst/snakefiles/BeaverBS_step2.snakemake": &fstest.MapFile{Data: []byte("rule step2:\n  shell: 'echo step2'\n")},
		"inst/rules/01fastqcxAtfirst.smk":          &fstest.MapFile{Data: []byte("rule fastqcxAtfirst:\n  shell: 'fastqcx {input}'\n")},
		"inst/rules_legacy/legacy_rule.smk":        &fstest.MapFile{Data: []byte("rule legacy:\n  shell: 'echo legacy'\n")},
		"inst/Rscripts/test.R":                     &fstest.MapFile{Data: []byte("# Test R script\nprint('hello')\n")},
		"inst/envs/otter-snakemake.yaml":           &fstest.MapFile{Data: []byte("name: otter-snakemake\n")},
		"inst/data/gene_mapping.csv":               &fstest.MapFile{Data: []byte("ensembl_id,symbol\nENSG001,BRCA1\n")},
		"docs/schema/otter-run-v1.schema.json":     &fstest.MapFile{Data: []byte("{}\n")},
	})
	os.Exit(m.Run())
}

// craftmakeEnvelopeStub is a shell stand-in for the real Craftmake binary. It
// answers "plan" with a protocol-conformant envelope so the Otter-side
// executor boundary, argument construction, and envelope parsing are all
// exercised without building or running Craftmake.
const craftmakeEnvelopeStub = `#!/bin/sh
set -eu
command_name="$1"
shift
case "$command_name" in
  plan)
    printf '{"protocol_version":"otter.craftmake/v1","command":"plan","ok":true,"run_id":"stub-run","data":{"workflow":"BeaverBS","phase":"step1","name":"stub plan","tasks":[]}}\n'
    ;;
  run)
    printf '{"protocol_version":"otter.craftmake/v1","command":"run","ok":true,"run_id":"stub-run","state_path":"stub/state.sqlite","data":{"status":"ok"}}\n'
    ;;
  *)
    printf '{"protocol_version":"otter.craftmake/v1","command":"%s","ok":false,"data":{"error":"unsupported"}}\n' "$command_name"
    exit 64
    ;;
esac
`

// writeCraftmakeEnvelopeStub installs the stub binary and returns its path.
func writeCraftmakeEnvelopeStub(t *testing.T) string {
	t.Helper()
	stubPath := filepath.Join(t.TempDir(), "craftmake")
	if err := os.WriteFile(stubPath, []byte(craftmakeEnvelopeStub), 0o755); err != nil {
		t.Fatal(err)
	}
	return stubPath
}

// writeCanonicalProjectFixture initialises a canonical project with pinned
// assets and a single stub reference registry release, then returns the project
// root, the registry root, and the reference selection.
func writeCanonicalProjectFixture(t *testing.T, fastqNames []string) (projectRoot, registryRoot, selection string) {
	t.Helper()
	projectRoot = t.TempDir()
	registryRoot = filepath.Join(t.TempDir(), "registry")

	// A stub release keeps the test offline while still passing production
	// reference verification.
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

	// The canonical track marker must exist before create will author into the
	// directory.
	writeCommandAssetFixture(t, projectRoot)
	writeTestFile(t, filepath.Join(projectRoot, "project.lock.yaml"), "schema_version: otter.project.lock/v1\nproject_id: fixture\nassets: []\n")

	fastqDirectory := filepath.Join(projectRoot, "data")
	if err := os.MkdirAll(fastqDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	pdataLines := []string{"sampleid,inline_barcode_sequence,condition"}
	for index, name := range fastqNames {
		// Non-empty content is required: the resolver rejects zero-byte inputs.
		content := "@" + name + "/1\nACGTACGT\n+\nIIIIIIII\n"
		writeTestFile(t, filepath.Join(fastqDirectory, name+"_R1.fastq.gz"), content)
		writeTestFile(t, filepath.Join(fastqDirectory, name+"_R2.fastq.gz"), content)
		condition := "case"
		if index%2 == 1 {
			condition = "control"
		}
		pdataLines = append(pdataLines, name+",,"+condition)
	}
	writeTestFile(t, filepath.Join(projectRoot, "pdata.csv"), strings.Join(pdataLines, "\n")+"\n")
	return projectRoot, registryRoot, selection
}

// writeCommandAssetFixture creates the directories the canonical resolver
// digests, so "otter create" does not have to pin real packaged assets in a
// unit test.
func writeCommandAssetFixture(t *testing.T, projectRoot string) {
	t.Helper()
	for _, directory := range []string{"workflows", "rules", "environments", "schemas", "runs"} {
		if err := os.MkdirAll(filepath.Join(projectRoot, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(projectRoot, "workflows", "rrbs.yaml"), "name: rrbs\n")
}

// TestCanonicalChainCreateResolveRunDryRun is the primary end-to-end contract
// for the canonical track: create writes a resolvable v1 project, resolve
// writes an immutable snapshot, and run --dry-run drives the executor boundary.
func TestCanonicalChainCreateResolveRunDryRun(t *testing.T) {
	projectRoot, registryRoot, selection := writeCanonicalProjectFixture(t, []string{"S01", "S02"})

	createOutput := executeCommand(t, rootCmd, "create",
		"--output", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--pdata", filepath.Join(projectRoot, "pdata.csv"),
		"--mode", "RRBS",
		"--jobid", "cohort-a",
		"--reference-root", registryRoot,
		"--reference-primary", selection,
	)
	if createOutput.exitCode != 0 {
		t.Fatalf("canonical create failed (exit %d):\n%s\n%s", createOutput.exitCode, createOutput.stdout, createOutput.stderr)
	}
	for _, expectedArtifact := range []string{"project.yaml", "samples.tsv", "references.lock.yaml"} {
		if _, err := os.Stat(filepath.Join(projectRoot, expectedArtifact)); err != nil {
			t.Fatalf("create did not write %s: %v", expectedArtifact, err)
		}
	}

	validateOutput := executeCommand(t, rootCmd, "config", "validate",
		"--config", filepath.Join(projectRoot, "project.yaml"), "--schema", "v1")
	if validateOutput.exitCode != 0 {
		t.Fatalf("generated project failed v1 validation: %s%s", validateOutput.stdout, validateOutput.stderr)
	}

	resolveOutput := executeCommand(t, rootCmd, "config", "resolve",
		"--project", filepath.Join(projectRoot, "project.yaml"),
		"--reference-root", registryRoot,
		"--backend", "local",
	)
	if resolveOutput.exitCode != 0 {
		t.Fatalf("config resolve failed (exit %d):\n%s\n%s", resolveOutput.exitCode, resolveOutput.stdout, resolveOutput.stderr)
	}
	runYAMLPath := strings.TrimSpace(resolveOutput.stdout)
	if _, err := os.Stat(runYAMLPath); err != nil {
		t.Fatalf("resolve did not print a readable run.yaml (%q): %v", runYAMLPath, err)
	}
	runInfo, err := os.Stat(runYAMLPath)
	if err != nil {
		t.Fatal(err)
	}
	if runInfo.Mode().Perm()&0o200 != 0 {
		t.Fatalf("run.yaml must be immutable (0o444), got 0o%o", runInfo.Mode().Perm())
	}

	craftmakeStub := writeCraftmakeEnvelopeStub(t)
	runOutput := executeCommand(t, rootCmd, "run",
		"--config", runYAMLPath,
		"--executor", "craftmake",
		"--phase", "step1",
		"--dry-run",
		"--foreground",
		"--craftmake-binary", craftmakeStub,
	)
	if runOutput.exitCode != 0 {
		t.Fatalf("canonical dry-run failed (exit %d):\n%s\n%s", runOutput.exitCode, runOutput.stdout, runOutput.stderr)
	}
	if !strings.Contains(runOutput.stdout, `"command":"plan"`) {
		t.Fatalf("craftmake plan envelope was not forwarded to stdout:\n%s", runOutput.stdout)
	}
}

// TestCanonicalCreateResolvesInputsTheSameWayAsResolve covers a real authoring
// trap found by following the manual by hand:
//
//   - "otter create" reads --fastq relative to the WORKING DIRECTORY.
//   - every later reader anchors a relative path in samples.tsv to the PROJECT ROOT.
//
// When the FASTQ directory sits outside the project root, recording the scanned
// path verbatim made the two commands disagree about what "relative" means:
// create reported success, and "otter config resolve" then failed with
// "no such file or directory" for a file that plainly existed.
//
// The rehearsal missed this because it only ever passes absolute --fastq paths,
// so this test deliberately mirrors the manual: a working directory holding
// fastq/ and samples.csv, and a sibling project root.
func TestCanonicalCreateResolvesInputsTheSameWayAsResolve(t *testing.T) {
	workingDirectory := t.TempDir()
	t.Chdir(workingDirectory)

	fastqDirectory := filepath.Join(workingDirectory, "fastq")
	if err := os.MkdirAll(fastqDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(fastqDirectory, "S01_R1.fastq.gz"), "@S01/1\nACGTACGT\n+\nIIIIIIII\n")
	writeTestFile(t, filepath.Join(fastqDirectory, "S01_R2.fastq.gz"), "@S01/2\nACGTACGT\n+\nIIIIIIII\n")
	writeTestFile(t, filepath.Join(workingDirectory, "samples.csv"),
		"sampleid,inline_barcode_sequence,condition\nS01,,case\n")

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

	projectRoot := filepath.Join(workingDirectory, "my_project")
	if output := executeCommand(t, rootCmd, "init", projectRoot); output.exitCode != 0 {
		t.Fatalf("init failed: %s%s", output.stdout, output.stderr)
	}

	// The manual's commands: relative input path, project root as a sibling
	// directory.
	createOutput := executeCommand(t, rootCmd, "create",
		"--output", "my_project",
		"--fastq", "./fastq",
		"--pdata", "./samples.csv",
		"--mode", "RRBS",
		"--jobid", "relative-inputs",
		"--reference-root", registryRoot,
		"--reference-primary", "hg19@hg19-legacy",
	)
	if createOutput.exitCode != 0 {
		t.Fatalf("create with a relative --fastq failed: %s%s", createOutput.stdout, createOutput.stderr)
	}

	// The recorded input, read the way every later command reads it, must name a
	// real file. This is the assertion that fails when create records the path
	// relative to the working directory instead of anchoring it.
	manifest, err := os.ReadFile(filepath.Join(projectRoot, "samples.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 || fields[0] == "sample_id" {
			continue
		}
		for _, recordedPath := range fields[1:3] {
			anchored := recordedPath
			if !filepath.IsAbs(anchored) {
				anchored = filepath.Join(projectRoot, recordedPath)
			}
			if _, statErr := os.Stat(anchored); statErr != nil {
				t.Fatalf("create reported success but %q does not resolve under the project root (%q): %v",
					recordedPath, anchored, statErr)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no sample inputs were checked")
	}

	resolveOutput := executeCommand(t, rootCmd, "config", "resolve",
		"--project", filepath.Join(projectRoot, "project.yaml"),
		"--reference-root", registryRoot,
		"--backend", "local",
	)
	if resolveOutput.exitCode != 0 {
		t.Fatalf("resolve rejected the project create just wrote (exit %d):\n%s\n%s",
			resolveOutput.exitCode, resolveOutput.stdout, resolveOutput.stderr)
	}
}

// TestCanonicalCreateScenariosForEveryMode covers the create/reference contract
// for the four scenarios that have distinct fixture expectations. WGBS shares
// the RRBS reference contract and is asserted here as a contract case only.
func TestCanonicalCreateScenariosForEveryMode(t *testing.T) {
	testCases := []struct {
		name          string
		mode          string
		pdx           bool
		expected      string
		expectedRoles []string
	}{
		{name: "rrbs", mode: "RRBS", expected: "rrbs", expectedRoles: []string{"primary"}},
		{name: "wgbs", mode: "WGBS", expected: "wgbs", expectedRoles: []string{"primary"}},
		{name: "rnaseq", mode: "RNASEQ", expected: "rnaseq", expectedRoles: []string{"primary"}},
		{name: "bs-pdx", mode: "RRBS", pdx: true, expected: "bs-pdx", expectedRoles: []string{"graft", "host"}},
		{name: "rna-pdx", mode: "RNASEQ", pdx: true, expected: "rna-pdx", expectedRoles: []string{"graft", "host"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot, registryRoot, selection := writeCanonicalProjectFixture(t, []string{"S01"})
			arguments := []string{"create",
				"--output", projectRoot,
				"--fastq", filepath.Join(projectRoot, "data"),
				"--pdata", filepath.Join(projectRoot, "pdata.csv"),
				"--mode", testCase.mode,
				"--jobid", "cohort-" + testCase.name,
				"--reference-root", registryRoot,
			}
			if testCase.pdx {
				arguments = append(arguments, "--reference-graft", selection, "--reference-host", selection)
			} else {
				arguments = append(arguments, "--reference-primary", selection)
			}
			output := executeCommand(t, rootCmd, arguments...)
			if output.exitCode != 0 {
				t.Fatalf("create %s failed (exit %d):\n%s\n%s", testCase.name, output.exitCode, output.stdout, output.stderr)
			}

			projectData, err := os.ReadFile(filepath.Join(projectRoot, "project.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(projectData), "scenario: "+testCase.expected) {
				t.Fatalf("expected scenario %q in project.yaml:\n%s", testCase.expected, projectData)
			}
			lockData, err := os.ReadFile(filepath.Join(projectRoot, "references.lock.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			for _, role := range testCase.expectedRoles {
				if !strings.Contains(string(lockData), role+":") {
					t.Fatalf("expected reference role %q in references.lock.yaml:\n%s", role, lockData)
				}
			}
		})
	}
}

// TestCanonicalCreateRejectsWGBSPDX documents that WGBS-PDX is refused rather
// than silently reinterpreted as BS-PDX.
func TestCanonicalCreateRejectsWGBSPDX(t *testing.T) {
	projectRoot, registryRoot, selection := writeCanonicalProjectFixture(t, []string{"S01"})
	output := executeCommand(t, rootCmd, "create",
		"--output", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--mode", "WGBS",
		"--species2", "mouse",
		"--jobid", "wgbs-pdx",
		"--reference-root", registryRoot,
		"--reference-graft", selection,
		"--reference-host", selection,
	)
	if output.exitCode == 0 {
		t.Fatal("expected WGBS with a second species to be rejected")
	}
	if !strings.Contains(output.stderr, "WGBS has no PDX scenario") {
		t.Fatalf("expected an explicit WGBS-PDX refusal, got:\n%s", output.stderr)
	}
}

// TestCanonicalCreateRequiresScenarioMatchingReferenceFlag pins the flag/role
// correspondence so a wrong flag cannot silently produce a half-locked project.
func TestCanonicalCreateRequiresScenarioMatchingReferenceFlag(t *testing.T) {
	projectRoot, registryRoot, selection := writeCanonicalProjectFixture(t, []string{"S01"})

	// A primary flag alongside a PDX selector is contradictory and must fail.
	primaryOnPDX := executeCommand(t, rootCmd, "create",
		"--output", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--mode", "RRBS",
		"--species2", "mouse",
		"--jobid", "wrong-flag",
		"--reference-root", registryRoot,
		"--reference-primary", selection,
		"--reference-graft", selection,
		"--reference-host", selection,
	)
	if primaryOnPDX.exitCode == 0 || !strings.Contains(primaryOnPDX.stderr, "--reference-graft and --reference-host") {
		t.Fatalf("expected a PDX flag mismatch error, got exit=%d stderr=%s", primaryOnPDX.exitCode, primaryOnPDX.stderr)
	}

	// Graft alone implies PDX, so the missing host must be named rather than
	// silently producing a non-PDX project.
	graftOnly := executeCommand(t, rootCmd, "create",
		"--output", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--mode", "RRBS",
		"--jobid", "wrong-flag-2",
		"--reference-root", registryRoot,
		"--reference-graft", selection,
	)
	if graftOnly.exitCode == 0 || !strings.Contains(graftOnly.stderr, "--reference-host") {
		t.Fatalf("expected a missing-host error, got exit=%d stderr=%s", graftOnly.exitCode, graftOnly.stderr)
	}
}

// TestCanonicalCreateRejectsMissingReferenceSelection documents that a canonical
// authoring pass cannot produce an unresolved reference lock.
func TestCanonicalCreateRejectsMissingReferenceSelection(t *testing.T) {
	projectRoot, registryRoot, _ := writeCanonicalProjectFixture(t, []string{"S01"})
	output := executeCommand(t, rootCmd, "create",
		"--output", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--mode", "RRBS",
		"--jobid", "missing-ref",
		"--reference-root", registryRoot,
	)
	if output.exitCode == 0 {
		t.Fatal("expected create without --reference-primary to fail")
	}
	if !strings.Contains(output.stderr, "--reference-primary") {
		t.Fatalf("expected the missing selection to name the required flag, got:\n%s", output.stderr)
	}
}

// TestCanonicalCreateRejectsUnknownReference documents that the registry is the
// authority: a syntactically valid but absent release must not be locked.
func TestCanonicalCreateRejectsUnknownReference(t *testing.T) {
	projectRoot, registryRoot, _ := writeCanonicalProjectFixture(t, []string{"S01"})
	output := executeCommand(t, rootCmd, "create",
		"--output", projectRoot,
		"--fastq", filepath.Join(projectRoot, "data"),
		"--mode", "RRBS",
		"--jobid", "unknown-ref",
		"--reference-root", registryRoot,
		"--reference-primary", "hg38@GRCh38.p14",
	)
	if output.exitCode == 0 {
		t.Fatal("expected create to reject a reference absent from the registry")
	}
	if !strings.Contains(output.stderr, "reference-primary") {
		t.Fatalf("expected the failure to name the offending flag, got:\n%s", output.stderr)
	}
}

// TestCanonicalCreateRefusesLegacyProjectDirectory covers the mixed-track guard:
// a legacy project must not receive canonical authoring artifacts.
func TestCanonicalCreateRefusesLegacyProjectDirectory(t *testing.T) {
	projectRoot := t.TempDir()
	writeTestFile(t, filepath.Join(projectRoot, ".otter", "assets.manifest.json"), "{}\n")

	fastqDirectory := filepath.Join(projectRoot, "data")
	if err := os.MkdirAll(fastqDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(fastqDirectory, "S01_R1.fastq.gz"), "@S01/1\nACGT\n+\nIIII\n")
	writeTestFile(t, filepath.Join(fastqDirectory, "S01_R2.fastq.gz"), "@S01/2\nACGT\n+\nIIII\n")

	output := executeCommand(t, rootCmd, "create",
		"--output", projectRoot,
		"--fastq", fastqDirectory,
		"--mode", "RRBS",
		"--jobid", "mixed",
		"--reference-primary", "hg19@hg19-legacy",
	)
	if output.exitCode == 0 {
		t.Fatal("expected canonical create to refuse a legacy project directory")
	}
	if !strings.Contains(output.stderr, "--legacy") {
		t.Fatalf("expected the error to point at --legacy, got:\n%s", output.stderr)
	}
}

// TestCreateLegacyTrackStillWritesOtterYAML is the characterization test for the
// compatibility track: --legacy must keep producing config/otter.yaml under
// userspace/<jobid>, and the legacy marker must remain the track evidence.
func TestCreateLegacyTrackStillWritesOtterYAML(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	fastqDirectory := filepath.Join(projectRoot, "fastq")
	if err := os.MkdirAll(fastqDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sampleName := range []string{"sample1", "sample2"} {
		writeTestFile(t, filepath.Join(fastqDirectory, sampleName+"_R1.fastq.gz"), "@"+sampleName+"/1\nACGT\n+\nIIII\n")
		writeTestFile(t, filepath.Join(fastqDirectory, sampleName+"_R2.fastq.gz"), "@"+sampleName+"/2\nACGT\n+\nIIII\n")
	}
	writeTestFile(t, filepath.Join(projectRoot, "pdata.csv"),
		"sampleid,inline_barcode_sequence,condition\nsample1,,case\nsample2,,control\n")

	userspaceRoot := filepath.Join(projectRoot, "userspace")
	output := executeCommand(t, rootCmd, "create",
		"--legacy",
		"--fastq", fastqDirectory,
		"--pdata", filepath.Join(projectRoot, "pdata.csv"),
		"--mode", "RRBS",
		"--output", userspaceRoot,
		"--jobid", "legacy-demo",
	)
	if output.exitCode != 0 {
		t.Fatalf("legacy create failed (exit %d):\n%s\n%s", output.exitCode, output.stdout, output.stderr)
	}

	configPath := filepath.Join(userspaceRoot, "legacy-demo", "config", "otter.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("legacy create did not write %s: %v", configPath, err)
	}
	configText := string(configData)
	// These keys are the legacy contract the Snakemake assets index into.
	for _, expectedKey := range []string{"SIDs:", "directories:", "reference:", "workflow:", "species:"} {
		if !strings.Contains(configText, expectedKey) {
			t.Errorf("legacy otter.yaml is missing %q", expectedKey)
		}
	}
	// species.name must stay a sequence: the Snakemake assets call .index() on it.
	if !strings.Contains(configText, "name:\n            - human") {
		t.Errorf("legacy otter.yaml must write workflow.species.name as a sequence:\n%s", configText)
	}
	// The canonical artifacts must not appear on the legacy track.
	for _, mustNotExist := range []string{"project.yaml", "samples.tsv", "references.lock.yaml"} {
		if _, err := os.Stat(filepath.Join(userspaceRoot, "legacy-demo", mustNotExist)); err == nil {
			t.Errorf("legacy create must not write %s", mustNotExist)
		}
	}
}

// TestInitCanonicalTrackWritesProjectLock covers the canonical init contract:
// the pinned asset directories are exactly the ones the resolver digests, and
// the lock records a digest per set.
func TestInitCanonicalTrackWritesProjectLock(t *testing.T) {
	projectRoot := filepath.Join(t.TempDir(), "canonical-project")
	output := executeCommand(t, rootCmd, "init", projectRoot)
	if output.exitCode != 0 {
		t.Fatalf("canonical init failed (exit %d):\n%s\n%s", output.exitCode, output.stdout, output.stderr)
	}

	for _, expectedDirectory := range []string{"workflows", "rules", "environments", "schemas", "runs"} {
		info, err := os.Stat(filepath.Join(projectRoot, expectedDirectory))
		if err != nil {
			t.Errorf("init did not create %s/: %v", expectedDirectory, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", expectedDirectory)
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "project.lock.yaml")); err != nil {
		t.Fatalf("init did not write project.lock.yaml: %v", err)
	}
	lock, err := projectlayout.ReadProjectLock(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Assets) == 0 {
		t.Fatal("project.lock.yaml recorded no pinned asset sets")
	}
	for _, assetSet := range lock.Assets {
		if !strings.HasPrefix(assetSet.Digest, "sha256:") {
			t.Errorf("asset set %s has no content digest: %q", assetSet.Role, assetSet.Digest)
		}
		if assetSet.Files == 0 {
			t.Errorf("asset set %s pinned no files", assetSet.Role)
		}
	}
	// The legacy track must not be marked on a canonical project.
	if _, err := os.Stat(filepath.Join(projectRoot, ".otter", "assets.manifest.json")); err == nil {
		t.Error("canonical init must not write the legacy assets manifest")
	}
}

// TestInitLegacyTrackKeepsCompatibilityLayout covers the legacy init contract.
func TestInitLegacyTrackKeepsCompatibilityLayout(t *testing.T) {
	projectRoot := filepath.Join(t.TempDir(), "legacy-project")
	output := executeCommand(t, rootCmd, "init", projectRoot, "--legacy")
	if output.exitCode != 0 {
		t.Fatalf("legacy init failed (exit %d):\n%s\n%s", output.exitCode, output.stdout, output.stderr)
	}
	for _, expectedDirectory := range []string{"config", "data", "envs", "inst", "R", "rules", "saveRDS", "temp", "userspace", "www"} {
		if _, err := os.Stat(filepath.Join(projectRoot, expectedDirectory)); err != nil {
			t.Errorf("legacy init did not create %s/: %v", expectedDirectory, err)
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".otter", "assets.manifest.json")); err != nil {
		t.Fatalf("legacy init did not stamp the assets manifest: %v", err)
	}
	// The legacy layout creates an empty workflows/ directory. It must stay
	// unpopulated: populating it is what the canonical track does.
	workflowEntries, err := os.ReadDir(filepath.Join(projectRoot, "workflows"))
	if err != nil {
		t.Fatalf("legacy init did not create workflows/: %v", err)
	}
	if len(workflowEntries) != 0 {
		t.Errorf("legacy init must leave workflows/ empty, found %d entries", len(workflowEntries))
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "project.lock.yaml")); err == nil {
		t.Error("legacy init must not write the canonical project.lock.yaml")
	}
}

// TestInitTrackMismatchIsRefused covers both mixed-track directions.
func TestInitTrackMismatchIsRefused(t *testing.T) {
	baseDirectory := t.TempDir()
	canonicalRoot := filepath.Join(baseDirectory, "canonical")
	legacyRoot := filepath.Join(baseDirectory, "legacy")

	if output := executeCommand(t, rootCmd, "init", canonicalRoot); output.exitCode != 0 {
		t.Fatalf("canonical init failed: %s%s", output.stdout, output.stderr)
	}
	if output := executeCommand(t, rootCmd, "init", legacyRoot, "--legacy"); output.exitCode != 0 {
		t.Fatalf("legacy init failed: %s%s", output.stdout, output.stderr)
	}

	legacyOverCanonical := executeCommand(t, rootCmd, "init", canonicalRoot, "--legacy")
	if legacyOverCanonical.exitCode == 0 || !strings.Contains(legacyOverCanonical.stderr, "canonical v1 project") {
		t.Fatalf("expected legacy init over a canonical project to be refused, got exit=%d stderr=%s",
			legacyOverCanonical.exitCode, legacyOverCanonical.stderr)
	}

	canonicalOverLegacy := executeCommand(t, rootCmd, "init", legacyRoot)
	if canonicalOverLegacy.exitCode == 0 || !strings.Contains(canonicalOverLegacy.stderr, "legacy compatibility project") {
		t.Fatalf("expected canonical init over a legacy project to be refused, got exit=%d stderr=%s",
			canonicalOverLegacy.exitCode, canonicalOverLegacy.stderr)
	}

	legacyRulesWithoutLegacy := executeCommand(t, rootCmd, "init", filepath.Join(baseDirectory, "x"), "--legacy-rules")
	if legacyRulesWithoutLegacy.exitCode == 0 || !strings.Contains(legacyRulesWithoutLegacy.stderr, "--legacy-rules only applies") {
		t.Fatalf("expected --legacy-rules without --legacy to be refused, got exit=%d stderr=%s",
			legacyRulesWithoutLegacy.exitCode, legacyRulesWithoutLegacy.stderr)
	}
}
