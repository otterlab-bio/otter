package reference

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

func TestBuildReleasePublishesVerifiedImmutableRegistryRelease(t *testing.T) {
	temporaryDirectory := t.TempDir()
	registryRoot := filepath.Join(temporaryDirectory, "registry")
	sourceFastaPath := writeBuildFixtureFile(t, temporaryDirectory, "source/genome.fa", ">chr1\nACGT\n")
	sourceGTFPath := writeBuildFixtureFile(t, temporaryDirectory, "source/genes.gtf", "chr1\ttest\tgene\t1\t4\t.\t+\t.\tgene_id \"gene1\";\n")
	toolDirectory := filepath.Join(temporaryDirectory, "tools")

	result, err := BuildRelease(BuildRequest{
		Context:           context.Background(),
		RegistryRoot:      registryRoot,
		ReferenceID:       "testgenome",
		Release:           "v1",
		Organism:          "Test organism",
		Assembly:          "TestAssembly",
		Aliases:           []string{"test", "testgenome"},
		SourceFastaPath:   sourceFastaPath,
		SourceGTFPath:     sourceGTFPath,
		SamtoolsBinary:    writeFakeSamtools(t, toolDirectory),
		BismarkBinary:     writeFakeBismark(t, toolDirectory, false),
		Bowtie2Binary:     writeFakeBowtie2(t, toolDirectory),
		STARBinary:        writeFakeSTAR(t, toolDirectory, false),
		STARSJDBOverhang:  99,
		IndexBuildThreads: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		unsealBuildTestRelease(result.ReleaseRoot)
	})

	expectedReleaseRoot := filepath.Join(registryRoot, "genomes", "testgenome", "v1")
	if result.ReleaseRoot != expectedReleaseRoot {
		t.Fatalf("unexpected release root: %q", result.ReleaseRoot)
	}
	definition, err := configv1.LoadReferenceDefinition(result.DefinitionPath)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Reference.ID != "testgenome" || definition.Reference.Release != "v1" {
		t.Fatalf("unexpected reference identity: %#v", definition.Reference)
	}
	if definition.Assets.Fasta.Path != "fasta/genome.fa" || definition.Assets.Fasta.FAI != "fasta/genome.fa.fai" {
		t.Fatalf("unexpected FASTA declaration: %#v", definition.Assets.Fasta)
	}
	if len(definition.Assets.Annotations) != 1 || definition.Assets.Annotations[0].Path != "annotations/genes.gtf" {
		t.Fatalf("unexpected annotation declaration: %#v", definition.Assets.Annotations)
	}
	if len(definition.Assets.Indexes) != 3 {
		t.Fatalf("expected three default indexes, got %#v", definition.Assets.Indexes)
	}
	if definition.Assets.Indexes[2].Type != ReferenceIndexSTAR || definition.Assets.Indexes[2].Parameters.SJDBOverhang != 99 {
		t.Fatalf("STAR index configuration was not recorded: %#v", definition.Assets.Indexes[2])
	}
	expectedIndexArguments := [][]string{
		{"--bowtie2", "--parallel", "4"},
		{"--threads", "8", "genome"},
		{"--runMode", "genomeGenerate", "--runThreadN", "8"},
	}
	for indexPosition, expectedArguments := range expectedIndexArguments {
		actualArguments := definition.Assets.Indexes[indexPosition].Parameters.Arguments
		if strings.Join(actualArguments, " ") != strings.Join(expectedArguments, " ") {
			t.Fatalf(
				"index %s recorded arguments %q, expected %q",
				definition.Assets.Indexes[indexPosition].Type,
				actualArguments,
				expectedArguments,
			)
		}
	}
	for _, requiredPath := range []string{
		"fasta/genome.fa",
		"fasta/genome.fa.fai",
		"annotations/genes.gtf",
		"indexes/bismark/genome/Bisulfite_Genome/CT_conversion/genome.1.bt2",
		"indexes/bowtie2/genome.1.bt2",
		"indexes/star/Genome",
		"reference.yaml",
		"manifest.json",
		"checksums.sha256",
	} {
		if _, err := os.Stat(filepath.Join(result.ReleaseRoot, requiredPath)); err != nil {
			t.Fatalf("expected generated asset %q: %v", requiredPath, err)
		}
	}
	checksumsData, err := os.ReadFile(result.ChecksumsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(checksumsData), "  manifest.json\n") || !strings.Contains(string(checksumsData), "  fasta/genome.fa\n") {
		t.Fatalf("generated checksums do not cover required assets: %s", checksumsData)
	}
	verification, err := VerifyRelease(result.ReleaseRoot, result.ManifestDigest)
	if err != nil {
		t.Fatal(err)
	}
	if !verification.Passed {
		t.Fatalf("generated release did not verify: %#v", verification.Issues)
	}
	if releaseInfo, err := os.Stat(result.ReleaseRoot); err != nil || releaseInfo.Mode().Perm() != 0o555 {
		t.Fatalf("release directory was not sealed read-only: info=%#v err=%v", releaseInfo, err)
	}
}

func TestBuildReleaseRejectsMissingExpectedIndexOutput(t *testing.T) {
	temporaryDirectory := t.TempDir()
	registryRoot := filepath.Join(temporaryDirectory, "registry")
	sourceFastaPath := writeBuildFixtureFile(t, temporaryDirectory, "source/genome.fa", ">chr1\nACGT\n")
	sourceGTFPath := writeBuildFixtureFile(t, temporaryDirectory, "source/genes.gtf", "chr1\ttest\tgene\t1\t4\t.\t+\t.\tgene_id \"gene1\";\n")
	toolDirectory := filepath.Join(temporaryDirectory, "tools")
	noOutputStar := writeFakeTool(t, toolDirectory, "STAR-no-output", `
if [ "$1" = "--version" ]; then
  echo "STAR 2.7.11b"
fi
`)

	_, err := BuildRelease(BuildRequest{
		RegistryRoot:    registryRoot,
		ReferenceID:     "testgenome",
		Release:         "empty-star-v1",
		Organism:        "Test organism",
		Assembly:        "TestAssembly",
		SourceFastaPath: sourceFastaPath,
		SourceGTFPath:   sourceGTFPath,
		Indexes:         []string{ReferenceIndexSTAR},
		SamtoolsBinary:  writeFakeSamtools(t, toolDirectory),
		STARBinary:      noOutputStar,
	})
	if err == nil || !strings.Contains(err.Error(), "STAR Genome output") {
		t.Fatalf("expected missing STAR output rejection, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(registryRoot, "genomes", "testgenome", "empty-star-v1")); !os.IsNotExist(statErr) {
		t.Fatalf("missing index output published a release: %v", statErr)
	}
}

func TestDefaultRegistryRootUsesConfiguredEnvironment(t *testing.T) {
	expectedRegistryRoot := filepath.Join(t.TempDir(), "shared-registry")
	t.Setenv(DefaultReferenceRootEnvironmentVariable, expectedRegistryRoot)

	actualRegistryRoot, err := DefaultRegistryRoot()
	if err != nil {
		t.Fatal(err)
	}
	if actualRegistryRoot != expectedRegistryRoot {
		t.Fatalf("expected configured registry root %q, got %q", expectedRegistryRoot, actualRegistryRoot)
	}
}

func TestBuildReleaseDoesNotPublishPartialReleaseWhenIndexBuildFails(t *testing.T) {
	temporaryDirectory := t.TempDir()
	registryRoot := filepath.Join(temporaryDirectory, "registry")
	sourceFastaPath := writeBuildFixtureFile(t, temporaryDirectory, "source/genome.fa", ">chr1\nACGT\n")
	sourceGTFPath := writeBuildFixtureFile(t, temporaryDirectory, "source/genes.gtf", "chr1\ttest\tgene\t1\t4\t.\t+\t.\tgene_id \"gene1\";\n")
	toolDirectory := filepath.Join(temporaryDirectory, "tools")

	_, err := BuildRelease(BuildRequest{
		RegistryRoot:    registryRoot,
		ReferenceID:     "testgenome",
		Release:         "broken-v1",
		Organism:        "Test organism",
		Assembly:        "TestAssembly",
		SourceFastaPath: sourceFastaPath,
		SourceGTFPath:   sourceGTFPath,
		Indexes:         []string{ReferenceIndexSTAR},
		SamtoolsBinary:  writeFakeSamtools(t, toolDirectory),
		STARBinary:      writeFakeSTAR(t, toolDirectory, true),
	})
	if err == nil || !strings.Contains(err.Error(), "build STAR index") {
		t.Fatalf("expected STAR build failure, got %v", err)
	}
	finalReleaseRoot := filepath.Join(registryRoot, "genomes", "testgenome", "broken-v1")
	if _, statErr := os.Stat(finalReleaseRoot); !os.IsNotExist(statErr) {
		t.Fatalf("failed build published a partial release: %v", statErr)
	}
	stagingEntries, globErr := filepath.Glob(filepath.Join(registryRoot, "genomes", "testgenome", ".staging-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(stagingEntries) != 0 {
		t.Fatalf("failed build left staging directories: %v", stagingEntries)
	}
}

func unsealBuildTestRelease(releaseRoot string) {
	_ = filepath.WalkDir(releaseRoot, func(path string, directoryEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if directoryEntry.IsDir() {
			_ = os.Chmod(path, 0o755)
			return nil
		}
		_ = os.Chmod(path, 0o644)
		return nil
	})
}

func writeBuildFixtureFile(t *testing.T, rootDirectory, relativePath, content string) string {
	t.Helper()
	path := filepath.Join(rootDirectory, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFakeSamtools(t *testing.T, toolDirectory string) string {
	t.Helper()
	return writeFakeTool(t, toolDirectory, "samtools", `
if [ "$1" = "--version" ]; then
  echo "samtools 1.20"
  exit 0
fi
if [ "$1" = "faidx" ]; then
  printf 'chr1\t4\t6\t4\t5\n' > "$2.fai"
  exit 0
fi
exit 1
`)
}

func writeFakeBismark(t *testing.T, toolDirectory string, shouldFail bool) string {
	t.Helper()
	failureCommand := ""
	if shouldFail {
		failureCommand = "exit 1"
	}
	return writeFakeTool(t, toolDirectory, "bismark_genome_preparation", `
if [ "$1" = "--version" ]; then
  echo "bismark 0.24.2"
  exit 0
fi
`+failureCommand+`
genome_directory="${!#}"
mkdir -p "$genome_directory/Bisulfite_Genome/CT_conversion"
printf 'bismark index\n' > "$genome_directory/Bisulfite_Genome/CT_conversion/genome.1.bt2"
`)
}

func writeFakeBowtie2(t *testing.T, toolDirectory string) string {
	t.Helper()
	return writeFakeTool(t, toolDirectory, "bowtie2-build", `
if [ "$1" = "--version" ]; then
  echo "bowtie2 2.5.4"
  exit 0
fi
output_prefix="${!#}"
printf 'bowtie index\n' > "$output_prefix.1.bt2"
`)
}

func writeFakeSTAR(t *testing.T, toolDirectory string, shouldFail bool) string {
	t.Helper()
	failureCommand := ""
	if shouldFail {
		failureCommand = "exit 1"
	}
	return writeFakeTool(t, toolDirectory, "STAR", `
if [ "$1" = "--version" ]; then
  echo "STAR 2.7.11b"
  exit 0
fi
`+failureCommand+`
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--genomeDir" ]; then
    genome_directory="$2"
    break
  fi
  shift
done
mkdir -p "$genome_directory"
printf 'star index\n' > "$genome_directory/Genome"
`)
}

func writeFakeTool(t *testing.T, toolDirectory, name, scriptBody string) string {
	t.Helper()
	if err := os.MkdirAll(toolDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(toolDirectory, name)
	script := "#!/usr/bin/env bash\nset -eu\n" + scriptBody
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
