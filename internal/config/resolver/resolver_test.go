package resolver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	referencepkg "github.com/otterlab-bio/otter/internal/reference"
	runstate "github.com/otterlab-bio/otter/internal/run"
	"github.com/otterlab-bio/otter/internal/site"
)

const fixtureDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

func TestResolveIsDeterministicAndTracksOverrides(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")
	writeProjectFixture(t, projectRoot)
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	writeReferenceFixture(t, referenceRoot, "hg19", "GRCh37.p13", true)
	runDirectory := runstate.Directory{
		ID:        "run-20260726T013245Z-abcdef",
		CreatedAt: time.Date(2026, 7, 26, 1, 32, 45, 0, time.UTC),
		Root:      filepath.Join(projectRoot, "runs", "run-20260726T013245Z-abcdef"),
	}
	options := Options{
		ProjectPath:      filepath.Join(projectRoot, "project.yaml"),
		RunDirectory:     runDirectory,
		ReferenceRoot:    referenceRoot,
		BackendOverride:  configv1.BackendLocal,
		ExecutorOverride: configv1.ExecutorSnakemake,
		ReferenceOverride: map[configv1.ReferenceRole]configv1.ReferenceSelection{
			configv1.ReferenceRolePrimary: "hg19@GRCh37.p13",
		},
	}
	first, err := (Resolver{}).Resolve(options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := (Resolver{}).Resolve(options)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digests != second.Digests || first.Run != second.Run {
		t.Fatal("same inputs did not resolve deterministically")
	}
	if first.Execution.Executor.Source != configv1.SourceCLI || first.Execution.Backend.Source != configv1.SourceCLI {
		t.Fatalf("CLI source tracking failed: %+v", first.Execution)
	}
	if !first.References.Override || first.References.EffectiveSelection.Primary != "hg19@GRCh37.p13" {
		t.Fatalf("reference override was not captured: %+v", first.References)
	}
	if first.References.ProjectSelection.Primary != "hg38@GRCh38.p14" {
		t.Fatalf("project reference selection was mutated: %+v", first.References.ProjectSelection)
	}
	if first.Samples[0].R1SHA256 == "" || first.Samples[0].R1Size == 0 || first.Samples[0].R2SHA256 == "" || first.Samples[0].R2Size == 0 {
		t.Fatalf("sample input metadata was not frozen: %+v", first.Samples[0])
	}
	if first.Paths.ProjectConfig == "" || first.Paths.SamplesManifest == "" || first.Paths.ReferencesLock == "" || len(first.Paths.WorkflowAssets) == 0 {
		t.Fatalf("source paths were not frozen: %+v", first.Paths)
	}
}

func TestResolveExcludesMutableSnakemakeStateFromWorkflowAssets(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")
	writeProjectFixture(t, projectRoot)
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	writeFile(t, filepath.Join(projectRoot, "BeaverRNA.snakemake"), "rule all:\n")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".snakemake", "locks"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(projectRoot, ".snakemake", "locks", "active.lock"), "mutable state\n")

	runDirectory := runstate.Directory{
		ID:        "run-20260726T013245Z-abcdef",
		CreatedAt: time.Date(2026, 7, 26, 1, 32, 45, 0, time.UTC),
		Root:      filepath.Join(projectRoot, "runs", "run-20260726T013245Z-abcdef"),
	}
	snapshot, err := (Resolver{}).Resolve(Options{
		ProjectPath:      filepath.Join(projectRoot, "project.yaml"),
		RunDirectory:     runDirectory,
		ReferenceRoot:    referenceRoot,
		BackendOverride:  configv1.BackendLocal,
		ExecutorOverride: configv1.ExecutorSnakemake,
	})
	if err != nil {
		t.Fatal(err)
	}

	mutableStatePath := filepath.Join(projectRoot, ".snakemake")
	rootSnakefilePath := filepath.Join(projectRoot, "BeaverRNA.snakemake")
	foundRootSnakefile := false
	for _, workflowAssetPath := range snapshot.Paths.WorkflowAssets {
		if workflowAssetPath == mutableStatePath {
			t.Fatalf("mutable Snakemake state must not be a workflow asset: %q", workflowAssetPath)
		}
		if workflowAssetPath == rootSnakefilePath {
			foundRootSnakefile = true
		}
	}
	if !foundRootSnakefile {
		t.Fatalf("root Snakemake workflow must remain a workflow asset: %#v", snapshot.Paths.WorkflowAssets)
	}

	writeFile(t, filepath.Join(projectRoot, ".snakemake", "locks", "active.lock"), "new mutable state\n")
	if err := runstate.RevalidateSnapshot(snapshot); err != nil {
		t.Fatalf("mutable Snakemake state must not invalidate immutable workflow assets: %v", err)
	}
}

func TestResolvedSnapshotRejectsInputDrift(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")
	writeProjectFixture(t, projectRoot)
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	runDirectory := runstate.Directory{
		ID:        "run-20260726T013245Z-abcdef",
		CreatedAt: time.Date(2026, 7, 26, 1, 32, 45, 0, time.UTC),
		Root:      filepath.Join(projectRoot, "runs", "run-20260726T013245Z-abcdef"),
	}
	snapshot, err := (Resolver{}).Resolve(Options{
		ProjectPath:      filepath.Join(projectRoot, "project.yaml"),
		RunDirectory:     runDirectory,
		ReferenceRoot:    referenceRoot,
		BackendOverride:  configv1.BackendLocal,
		ExecutorOverride: configv1.ExecutorCraftmake,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runstate.RevalidateSnapshot(snapshot); err != nil {
		t.Fatalf("fresh snapshot should revalidate: %v", err)
	}
	writeFile(t, filepath.Join(projectRoot, "data", "S01_R1.fastq.gz"), "@S01/1\nTTTT\n+\nIIII\n")
	if err := runstate.RevalidateSnapshot(snapshot); err == nil {
		t.Fatal("expected modified FASTQ input to be rejected")
	}
}

func TestResolveAutoDetectsLocalBackend(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")
	writeProjectFixture(t, projectRoot)
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	runDirectory := runstate.Directory{
		ID:        "run-20260726T013245Z-abcdef",
		CreatedAt: time.Date(2026, 7, 26, 1, 32, 45, 0, time.UTC),
		Root:      filepath.Join(projectRoot, "runs", "run-20260726T013245Z-abcdef"),
	}
	mockDetector := site.NewDetector(
		site.WithToolChecker(func(tool string) bool { return false }),
	)
	options := Options{
		ProjectPath:   filepath.Join(projectRoot, "project.yaml"),
		RunDirectory:  runDirectory,
		ReferenceRoot: referenceRoot,
		Detector:      mockDetector,
	}
	snapshot, err := (Resolver{}).Resolve(options)
	if err != nil {
		t.Fatalf("auto-detection should succeed with no SLURM tools: %v", err)
	}
	if snapshot.Execution.Backend.Value != configv1.BackendLocal {
		t.Fatalf("expected backend local from auto-detection, got %q", snapshot.Execution.Backend.Value)
	}
	if snapshot.Execution.Backend.Source != configv1.SourceDetection {
		t.Fatalf("expected source detection, got %q", snapshot.Execution.Backend.Source)
	}
}

func TestResolveAutoDetectionFailsOnPartialSlurm(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")
	writeProjectFixture(t, projectRoot)
	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	runDirectory := runstate.Directory{
		ID:        "run-20260726T013245Z-abcdef",
		CreatedAt: time.Now().UTC(),
		Root:      filepath.Join(projectRoot, "runs", "run-20260726T013245Z-abcdef"),
	}
	mockDetector := site.NewDetector(
		site.WithToolChecker(func(tool string) bool { return tool == "sbatch" || tool == "squeue" }),
	)
	options := Options{
		ProjectPath:   filepath.Join(projectRoot, "project.yaml"),
		RunDirectory:  runDirectory,
		ReferenceRoot: referenceRoot,
		Detector:      mockDetector,
	}
	_, err := (Resolver{}).Resolve(options)
	if err == nil {
		t.Fatal("expected partial SLURM toolchain to fail closed")
	}
}

func TestValidateSlurmComputePathsFailsClosedOnReferenceError(t *testing.T) {
	referenceRoot := t.TempDir()
	computePathError := errors.New("mount is absent")
	err := validateSlurmComputePaths(
		Options{
			ComputePathValidator: func(path, partition, account string) error {
				if path != referenceRoot || partition != "cpu" || account != "genomics" {
					t.Fatalf("unexpected effective reference compute preflight: path=%q partition=%q account=%q", path, partition, account)
				}
				return computePathError
			},
		},
		backendResolution{
			Backend: configv1.BackendSlurm,
			SlurmResources: configv1.ResolvedSlurmResources{
				Partition: configv1.ResolvedString{Value: "cpu"},
				Account:   configv1.ResolvedString{Value: "genomics"},
			},
		},
		referenceRoot,
		referenceRoot,
	)
	if !errors.Is(err, computePathError) {
		t.Fatalf("expected compute-node reference root preflight error, got %v", err)
	}
}

func TestValidateSlurmComputePathsSkipsImplicitComputeSubmission(t *testing.T) {
	temporaryRoot := t.TempDir()
	runRoot := filepath.Join(temporaryRoot, "runs", "run-20260726T013245Z-abcdef")
	if err := os.MkdirAll(runRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	err := validateSlurmComputePaths(
		Options{},
		backendResolution{
			Backend: configv1.BackendSlurm,
			SlurmResources: configv1.ResolvedSlurmResources{
				Partition: configv1.ResolvedString{Value: "cpu"},
				Account:   configv1.ResolvedString{Value: "genomics"},
			},
		},
		temporaryRoot,
		runRoot,
	)
	if err != nil {
		t.Fatalf("login-node resolution should not submit an implicit compute preflight: %v", err)
	}
}

func TestResolveAutoBackendWithNamedSiteProfile(t *testing.T) {
	projectRoot := t.TempDir()
	referenceRoot := filepath.Join(projectRoot, "reference-registry")

	writeProjectFixture(t, projectRoot)
	sitePath := filepath.Join(projectRoot, "sites", "hpc-cluster-1.yaml")
	siteYAML := fmt.Sprintf("schema_version: otter.site/v1\nsite:\n  id: hpc-cluster-1\n  backend: slurm\nslurm:\n  partition: cpu\n  account: genomics\n  qos: normal\n  default_time: 24:00:00\npaths:\n  reference_root: %s\n", referenceRoot)
	writeFile(t, sitePath, siteYAML)
	t.Setenv("OTTER_SITE_PROFILE", sitePath)

	writeReferenceFixture(t, referenceRoot, "hg38", "GRCh38.p14", false)
	runDirectory := runstate.Directory{
		ID:        "run-20260726T013245Z-abcdef",
		CreatedAt: time.Date(2026, 7, 26, 1, 32, 45, 0, time.UTC),
		Root:      filepath.Join(projectRoot, "runs", "run-20260726T013245Z-abcdef"),
	}
	mockDetector := site.NewDetector(
		site.WithToolChecker(func(tool string) bool {
			return tool == "sbatch" || tool == "squeue" || tool == "sacct" || tool == "scancel"
		}),
		site.WithClusterNameGetter(func() (string, bool) {
			return "test-cluster", true
		}),
		site.WithSlurmValidator(func(profile *site.SiteProfile) error { return nil }),
	)
	if err := os.MkdirAll(runDirectory.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	computePathValidated := false
	outputPathValidated := false
	options := Options{
		ProjectPath:  filepath.Join(projectRoot, "project.yaml"),
		RunDirectory: runDirectory,
		SiteOverride: "hpc-cluster-1",
		Detector:     mockDetector,
		ComputePathValidator: func(path, partition, account string) error {
			computePathValidated = true
			if path != referenceRoot || partition != "cpu" || account != "genomics" {
				t.Fatalf("unexpected effective reference compute preflight: path=%q partition=%q account=%q", path, partition, account)
			}
			return nil
		},
		ComputeWritablePathValidator: func(path, partition, account string) error {
			outputPathValidated = true
			if path != runDirectory.Root || partition != "cpu" || account != "genomics" {
				t.Fatalf("unexpected run output compute preflight: path=%q partition=%q account=%q", path, partition, account)
			}
			return nil
		},
	}
	snapshot, err := (Resolver{}).Resolve(options)
	if err != nil {
		t.Fatalf("named site profile resolve should succeed with mocked SLURM: %v", err)
	}
	if snapshot.Execution.Backend.Value != configv1.BackendSlurm {
		t.Fatalf("expected backend slurm from named site profile, got %q", snapshot.Execution.Backend.Value)
	}
	if snapshot.Execution.Site.Value != "hpc-cluster-1" {
		t.Fatalf("expected site hpc-cluster-1, got %q", snapshot.Execution.Site.Value)
	}
	if !computePathValidated {
		t.Fatal("expected effective reference root compute-node preflight")
	}
	if !outputPathValidated {
		t.Fatal("expected run output compute-node writability preflight")
	}
}

func writeProjectFixture(t *testing.T, projectRoot string) {
	t.Helper()
	content := "schema_version: otter.project/v1\nproject:\n  id: cohort-a\nworkflow:\n  scenario: rrbs\nexecution:\n  executor: craftmake\n  backend: auto\n  site: auto\nsamples:\n  manifest: samples.tsv\nreferences:\n  primary: hg38@GRCh38.p14\nobservability:\n  metrics: true\n  retain_logs: true\n"
	writeFile(t, filepath.Join(projectRoot, "project.yaml"), content)
	writeFile(t, filepath.Join(projectRoot, "samples.tsv"), "sample_id\tr1\tr2\nS01\tdata/S01_R1.fastq.gz\tdata/S01_R2.fastq.gz\n")
	writeFile(t, filepath.Join(projectRoot, "data", "S01_R1.fastq.gz"), "@S01/1\nACGT\n+\nIIII\n")
	writeFile(t, filepath.Join(projectRoot, "data", "S01_R2.fastq.gz"), "@S01/2\nTGCA\n+\nIIII\n")
	lock := "schema_version: otter.references.lock/v1\nreferences:\n  primary:\n    id: hg38\n    release: GRCh38.p14\n    manifest_digest: " + fixtureDigest + "\n"
	writeFile(t, filepath.Join(projectRoot, "references.lock.yaml"), lock)
	for _, directoryName := range []string{"workflows", "rules", "environments", "schemas"} {
		if err := os.MkdirAll(filepath.Join(projectRoot, directoryName), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(projectRoot, "workflows", "rrbs.yaml"), "name: rrbs\n")
	writeFile(t, filepath.Join(projectRoot, "project.lock.yaml"), "schema_version: otter.project.lock/v1\n")
}

func writeReferenceFixture(t *testing.T, referenceRoot string, id string, release string, _ bool) {
	t.Helper()
	releaseRoot := filepath.Join(referenceRoot, "genomes", id, release)
	fastaContent := ">chr1\nACGT\n"
	fastaDigest := referencepkg.ComputeDigest(fastaContent)
	indexContent := "index\n"
	indexDigest := referencepkg.ComputeDigest(indexContent)
	indexManifestDigest := referencepkg.ComputeDigest("index.bin:" + indexDigest)
	writeFile(t, filepath.Join(releaseRoot, "fasta", "genome.fa.gz"), fastaContent)
	writeFile(t, filepath.Join(releaseRoot, "fasta", "genome.fa.gz.fai"), "chr1\t4\t0\t4\t5\n")
	writeFile(t, filepath.Join(releaseRoot, "indexes", "bismark", "index.bin"), indexContent)
	definition := fmt.Sprintf("schema_version: otter.reference/v1\nreference:\n  id: %s\n  release: %s\n  organism: Homo sapiens\n  assembly: test\nassets:\n  fasta:\n    path: fasta/genome.fa.gz\n    sha256: %s\n    size_bytes: %d\n    fai: fasta/genome.fa.gz.fai\n  indexes:\n    - type: bismark\n      path: indexes/bismark\n      reference_fasta_sha256: %s\n      tool: bismark\n      tool_version: test\n      manifest_sha256: %s\ncompatibility:\n  scenarios: [rrbs]\n", id, release, fastaDigest, len(fastaContent), fastaDigest, indexManifestDigest)
	writeFile(t, filepath.Join(releaseRoot, "reference.yaml"), definition)
	report, err := referencepkg.BuildManifest(releaseRoot)
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
	if id == "hg38" {
		lock := "schema_version: otter.references.lock/v1\nreferences:\n  primary:\n    id: hg38\n    release: GRCh38.p14\n    manifest_digest: " + referencepkg.ComputeDigest(string(manifestData)) + "\n"
		writeFile(t, filepath.Join(filepath.Dir(referenceRoot), "references.lock.yaml"), lock)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
