package workflow

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otterlab-bio/otter/internal/artifact"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/reference"
	runstate "github.com/otterlab-bio/otter/internal/run"
)

func TestPublishSnakemakeArtifactsStagesWGBSPayloadAndRetriesIdempotently(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioWGBS)
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "mCall", "methrixh5", "methrix_data.h5"), "hdf5 payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "bsmap", "hg38", "bismark_summary_report.html"), "<html>summary</html>\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "qc", "qc_summary.xlsx"), "xlsx payload\n")

	result, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyPublished || result.ArtifactCount != 3 {
		t.Fatalf("unexpected initial publication result: %#v", result)
	}
	for _, artifactPath := range []string{
		"methylation/methrix_data.h5",
		"methylation/bismark_summary_report.html",
		"qc/qc_summary.xlsx",
	} {
		if err := requireRegularFile(filepath.Join(snapshot.Paths.Results, artifactPath)); err != nil {
			t.Fatalf("missing published artifact %s: %v", artifactPath, err)
		}
	}
	manifest, err := artifact.Load(result.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	verification, err := artifact.Verify(snapshot.Paths.Results, manifest)
	if err != nil || !verification.Passed {
		t.Fatalf("published manifest did not verify: report=%#v error=%v", verification, err)
	}

	retryResult, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !retryResult.AlreadyPublished || retryResult.ArtifactCount != result.ArtifactCount {
		t.Fatalf("retry must verify the immutable publication, got %#v", retryResult)
	}
}

func TestPublishSnakemakeArtifactsStagesRRBSPayloadAndRetriesIdempotently(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioRRBS)
	writeWGBSPublicationInputs(t, snapshot)

	result, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyPublished || result.ArtifactCount != 3 {
		t.Fatalf("unexpected initial RRBS publication result: %#v", result)
	}

	retryResult, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !retryResult.AlreadyPublished || retryResult.ArtifactCount != result.ArtifactCount {
		t.Fatalf("RRBS retry must verify the immutable publication, got %#v", retryResult)
	}
}

func TestPublishSnakemakeArtifactsCompletesMatchingPartialWGBSPublication(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioWGBS)
	writeWGBSPublicationInputs(t, snapshot)
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "methylation", "methrix_data.h5"), "hdf5 payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "methylation", "bismark_summary_report.html"), "<html>summary</html>\n")

	result, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyPublished {
		t.Fatalf("partial publication without a manifest must finish publication, got %#v", result)
	}
	if _, err := artifact.Load(result.ManifestPath); err != nil {
		t.Fatalf("matching partial publication must receive an immutable manifest: %v", err)
	}
}

func TestPublishSnakemakeArtifactsRejectsMismatchedPartialWGBSPublication(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioWGBS)
	writeWGBSPublicationInputs(t, snapshot)
	partialMatrixPath := filepath.Join(snapshot.Paths.Results, "methylation", "methrix_data.h5")
	writeWorkflowArtifact(t, partialMatrixPath, "different payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "methylation", "bismark_summary_report.html"), "<html>summary</html>\n")

	_, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err == nil || !strings.Contains(err.Error(), "existing publication differs from the staged retry payload") {
		t.Fatalf("expected mismatched partial publication to fail closed, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName)); !os.IsNotExist(statErr) {
		t.Fatalf("mismatched partial publication must not create a manifest, stat error=%v", statErr)
	}
	partialContent, readErr := os.ReadFile(partialMatrixPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(partialContent) != "different payload\n" {
		t.Fatalf("mismatched partial payload must not be overwritten: %q", partialContent)
	}
}

func TestExpressionMatrixDeclarationUsesIdentifierSpecificSchema(t *testing.T) {
	countDeclaration := expressionMatrixDeclaration("expression-count-matrix", "methylation/matrix_count.txt")
	normalizedDeclaration := expressionMatrixDeclaration("expression-normalized-matrix", "methylation/matrix_norm.txt")

	if countDeclaration.Schema != "otter.expression-count-matrix/v1" {
		t.Fatalf("count matrix schema = %q", countDeclaration.Schema)
	}
	if normalizedDeclaration.Schema != "otter.expression-normalized-matrix/v1" {
		t.Fatalf("normalized matrix schema = %q", normalizedDeclaration.Schema)
	}
	if normalizedDeclaration.Comparison != countDeclaration.Comparison {
		t.Fatalf("normalized matrix comparison contract = %#v, want %#v", normalizedDeclaration.Comparison, countDeclaration.Comparison)
	}
}

func TestPublishSnakemakeArtifactsStagesRNAOutcomeArtifacts(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioRNASeq)
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "methylation", "matrix_count.txt"), "gene\tS01\nGeneA\t10\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "methylation", "matrix_norm.txt"), "gene\tS01\nGeneA\t9.5\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "qc", "qc_summary.xlsx"), "xlsx payload\n")
	splicingRoot := filepath.Join(snapshot.Paths.Work, "bsmap", "RNASplicing")
	writeWorkflowArtifact(t, filepath.Join(splicingRoot, "events.tsv"), "event\tvalue\nSE\t1\n")
	writeWorkflowArtifact(t, filepath.Join(splicingRoot, "splicing-outcome.json"), fmt.Sprintf(`{
  "schema_version": "otter.rna-splicing-outcome/v1",
  "status": "produced",
  "source_root": %q,
  "artifact_paths": ["events.tsv"]
}
`, splicingRoot))

	result, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ArtifactCount != 5 {
		t.Fatalf("unexpected RNA artifact count: %#v", result)
	}
	manifest, err := artifact.Load(result.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	foundNormalizedMatrix := false
	foundSplicingOutcome := false
	for _, entry := range manifest.Artifacts {
		switch entry.ID {
		case "expression-normalized-matrix":
			foundNormalizedMatrix = true
			if entry.Schema != "otter.expression-normalized-matrix/v1" {
				t.Fatalf("normalized matrix must use its dedicated schema: %#v", entry)
			}
		case "splicing-outcome":
			foundSplicingOutcome = true
			if entry.Comparison.Tier != artifact.ComparisonTierStructural || entry.Comparison.Comparator != artifact.ComparatorRNASplicingOutcome {
				t.Fatalf("splicing outcome must use its structural comparator: %#v", entry)
			}
		}
	}
	if !foundNormalizedMatrix {
		t.Fatal("published RNA manifest must declare a normalized expression matrix")
	}
	if !foundSplicingOutcome {
		t.Fatal("published RNA manifest must declare a splicing outcome")
	}
	for _, artifactPath := range []string{
		"splicing/outcome.json",
		"splicing/files/events.tsv",
		"methylation/matrix_count.txt",
		"methylation/matrix_norm.txt",
	} {
		if err := requireRegularFile(filepath.Join(snapshot.Paths.Results, artifactPath)); err != nil {
			t.Fatalf("missing published RNA artifact %s: %v", artifactPath, err)
		}
	}
}

func TestPublishSnakemakeArtifactsCompletesMatchingPartialRNAPublication(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioRNASeq)
	writeRNAPublicationInputs(t, snapshot, false)
	splicingRoot := filepath.Join(snapshot.Paths.Results, "splicing")
	writeWorkflowArtifact(t, filepath.Join(splicingRoot, "outcome.json"), readPublicationInput(t, filepath.Join(snapshot.Paths.Work, "bsmap", "RNASplicing", "splicing-outcome.json")))
	writeWorkflowArtifact(t, filepath.Join(splicingRoot, "files", "events.tsv"), "event\tvalue\nSE\t1\n")

	result, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyPublished {
		t.Fatalf("matching partial RNA publication without a manifest must finish publication, got %#v", result)
	}
	if _, err := artifact.Load(result.ManifestPath); err != nil {
		t.Fatalf("matching partial RNA publication must receive an immutable manifest: %v", err)
	}
}

func TestPublishSnakemakeArtifactsRejectsMismatchedPartialRNAPublication(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioRNASeq)
	writeRNAPublicationInputs(t, snapshot, false)
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "splicing", "outcome.json"), "different outcome\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "splicing", "files", "events.tsv"), "event\tvalue\nSE\t1\n")

	_, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err == nil || !strings.Contains(err.Error(), "existing publication differs from the staged retry payload") {
		t.Fatalf("expected mismatched partial RNA publication to fail closed, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName)); !os.IsNotExist(statErr) {
		t.Fatalf("mismatched partial RNA publication must not create a manifest, stat error=%v", statErr)
	}
}

func TestBuildSplicingPublicationRejectsEmptySourceRoot(t *testing.T) {
	outcomePath := filepath.Join(t.TempDir(), "splicing-outcome.json")
	writeWorkflowArtifact(t, outcomePath, `{
  "schema_version": "otter.rna-splicing-outcome/v1",
  "status": "not_applicable",
  "source_root": "",
  "artifact_paths": []
}
`)
	if _, _, err := buildSplicingPublication(outcomePath); err == nil {
		t.Fatal("expected empty RNA splicing source_root to fail")
	}
}

func TestPublishSnakemakeArtifactsStagesRNAPDXArtifactsAndRetriesIdempotently(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioRNAPDX)
	writeRNAPublicationInputs(t, snapshot, true)

	result, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:        context.Background(),
		SnapshotPath:   snapshotPath,
		Snapshot:       snapshot,
		SamtoolsBinary: writeFakeSamtools(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyPublished || result.ArtifactCount != 8 {
		t.Fatalf("unexpected initial RNA-PDX publication result: %#v", result)
	}
	manifest, err := artifact.Load(result.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	seenArtifactIDs := make(map[string]bool, len(manifest.Artifacts))
	for _, declaration := range manifest.Artifacts {
		seenArtifactIDs[declaration.ID] = true
	}
	for _, expectedArtifactID := range []string{
		"graft-alignment-bam-0001",
		"graft-alignment-bai-0001",
	} {
		if !seenArtifactIDs[expectedArtifactID] {
			t.Fatalf("RNA-PDX manifest is missing aligned PDX artifact ID %q: %#v", expectedArtifactID, seenArtifactIDs)
		}
	}
	if seenArtifactIDs["graft-rna-alignment-bam-0001"] || seenArtifactIDs["graft-rna-alignment-bai-0001"] {
		t.Fatalf("RNA-PDX manifest retained executor-specific graft artifact IDs: %#v", seenArtifactIDs)
	}

	for _, artifactPath := range []string{
		"pdx/graft/S01_graft_Filtered.bam",
		"pdx/graft/S01_graft_Filtered.bam.bai",
		"pdx/graft/classification.tsv",
		"splicing/outcome.json",
		"splicing/files/events.tsv",
		"methylation/matrix_count.txt",
		"methylation/matrix_norm.txt",
	} {
		if err := requireRegularFile(filepath.Join(snapshot.Paths.Results, artifactPath)); err != nil {
			t.Fatalf("missing published RNA-PDX artifact %s: %v", artifactPath, err)
		}
	}

	retryResult, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:        context.Background(),
		SnapshotPath:   snapshotPath,
		Snapshot:       snapshot,
		SamtoolsBinary: writeFakeSamtools(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !retryResult.AlreadyPublished || retryResult.ArtifactCount != result.ArtifactCount {
		t.Fatalf("RNA-PDX retry must verify the immutable publication, got %#v", retryResult)
	}
}

func TestPublishSnakemakeArtifactsCompletesPartialBSPDXPublication(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioBSPDX)
	writeBSPDXPublicationInputs(t, snapshot)
	partialGraftDirectory := filepath.Join(snapshot.Paths.Results, "pdx", "graft")
	writeWorkflowArtifact(t, filepath.Join(partialGraftDirectory, "S01_graft_Filtered.bam"), "bam payload\n")
	writeWorkflowArtifact(t, filepath.Join(partialGraftDirectory, "S01_graft_Filtered.bam.bai"), "bai payload\n")
	writeWorkflowArtifact(t, filepath.Join(partialGraftDirectory, "classification.tsv"), "artifact_index\tsource_bam\tsource_bai\tmapped_reads\n0001\tS01_graft_Filtered.bam\tS01_graft_Filtered.bam.bai\t7\n")
	writeWorkflowArtifact(t, filepath.Join(partialGraftDirectory, "bismark_summary_report.html"), "<html>graft summary</html>\n")

	result, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:        context.Background(),
		SnapshotPath:   snapshotPath,
		Snapshot:       snapshot,
		SamtoolsBinary: writeFakeSamtools(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyPublished {
		t.Fatalf("partial BS-PDX publication without a manifest must finish publication, got %#v", result)
	}
	for _, artifactPath := range []string{
		"pdx/graft/classification.tsv",
		"methylation/methrix_data.h5",
		"artifacts.json",
	} {
		if err := requireRegularFile(filepath.Join(snapshot.Paths.Results, artifactPath)); err != nil {
			t.Fatalf("missing recovered BS-PDX publication artifact %s: %v", artifactPath, err)
		}
	}
}

func TestPublishSnakemakeArtifactsRejectsMismatchedPartialBSPDXPublication(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioBSPDX)
	writeBSPDXPublicationInputs(t, snapshot)
	partialBAMPath := filepath.Join(snapshot.Paths.Results, "pdx", "graft", "S01_graft_Filtered.bam")
	writeWorkflowArtifact(t, partialBAMPath, "different bam payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "pdx", "graft", "S01_graft_Filtered.bam.bai"), "bai payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "pdx", "graft", "classification.tsv"), "artifact_index\tsource_bam\tsource_bai\tmapped_reads\n0001\tS01_graft_Filtered.bam\tS01_graft_Filtered.bam.bai\t7\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "pdx", "graft", "bismark_summary_report.html"), "<html>graft summary</html>\n")

	_, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:        context.Background(),
		SnapshotPath:   snapshotPath,
		Snapshot:       snapshot,
		SamtoolsBinary: writeFakeSamtools(t),
	})
	if err == nil || !strings.Contains(err.Error(), "existing publication differs from the staged retry payload") {
		t.Fatalf("expected mismatched partial BS-PDX publication to fail closed, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName)); !os.IsNotExist(statErr) {
		t.Fatalf("mismatched partial BS-PDX publication must not create a manifest, stat error=%v", statErr)
	}
	partialContent, readErr := os.ReadFile(partialBAMPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(partialContent) != "different bam payload\n" {
		t.Fatalf("mismatched partial PDX payload must not be overwritten: %q", partialContent)
	}
}

func TestPublishSnakemakeArtifactsRecoversAfterStagingProcessIsKilled(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioBSPDX)
	writeBSPDXPublicationInputs(t, snapshot)
	checkpointPath := filepath.Join(t.TempDir(), "staging-checkpoint")
	blockingSamtoolsPath := writeBlockingSamtools(t)

	publicationProcess := exec.Command(os.Args[0], "-test.run=^TestPublishSnakemakeArtifactsInterruptedProcessHelper$")
	publicationProcess.Env = append(os.Environ(),
		"OTTER_SNAKEMAKE_PUBLICATION_HELPER=1",
		"OTTER_SNAKEMAKE_PUBLICATION_SNAPSHOT="+snapshotPath,
		"OTTER_SNAKEMAKE_PUBLICATION_SAMTOOLS="+blockingSamtoolsPath,
		"OTTER_SNAKEMAKE_PUBLICATION_CHECKPOINT="+checkpointPath,
	)
	if err := publicationProcess.Start(); err != nil {
		t.Fatal(err)
	}
	checkpointDeadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(checkpointPath); err == nil {
			break
		}
		if time.Now().After(checkpointDeadline) {
			_ = publicationProcess.Process.Kill()
			_ = publicationProcess.Wait()
			t.Fatal("interrupted publication did not reach the staged BAM validation checkpoint")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := publicationProcess.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := publicationProcess.Wait(); err == nil {
		t.Fatal("interrupted publication process unexpectedly exited successfully")
	}

	stagingRoots, err := filepath.Glob(filepath.Join(snapshot.Paths.Results, ".publish-staging", "snakemake.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(stagingRoots) != 1 {
		t.Fatalf("expected one retained interrupted staging root, got %#v", stagingRoots)
	}
	for _, stagedArtifactPath := range []string{
		"pdx/graft/S01_graft_Filtered.bam",
		"pdx/graft/S01_graft_Filtered.bam.bai",
		"pdx/graft/classification.tsv",
	} {
		if err := requireRegularFile(filepath.Join(stagingRoots[0], stagedArtifactPath)); err != nil {
			t.Fatalf("interrupted process must have staged %s before termination: %v", stagedArtifactPath, err)
		}
	}
	if _, err := os.Stat(filepath.Join(snapshot.Paths.Results, "pdx", "graft")); !os.IsNotExist(err) {
		t.Fatalf("interrupted staging must not publish graft results, stat error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName)); !os.IsNotExist(err) {
		t.Fatalf("interrupted staging must not publish an immutable manifest, stat error=%v", err)
	}

	retryResult, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:        context.Background(),
		SnapshotPath:   snapshotPath,
		Snapshot:       snapshot,
		SamtoolsBinary: writeFakeSamtools(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if retryResult.AlreadyPublished {
		t.Fatalf("retry after an interrupted staging process must create publication, got %#v", retryResult)
	}
	manifest, err := artifact.Load(retryResult.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	verification, err := artifact.Verify(snapshot.Paths.Results, manifest)
	if err != nil || !verification.Passed {
		t.Fatalf("retry after interrupted staging must publish a verifiable manifest: report=%#v error=%v", verification, err)
	}
}

func TestPublishSnakemakeArtifactsInterruptedProcessHelper(t *testing.T) {
	if os.Getenv("OTTER_SNAKEMAKE_PUBLICATION_HELPER") != "1" {
		return
	}
	snapshotPath := os.Getenv("OTTER_SNAKEMAKE_PUBLICATION_SNAPSHOT")
	snapshot, err := configv1.LoadRunSnapshot(snapshotPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	publicationRequest := SnakemakeArtifactPublicationRequest{
		Context:        context.Background(),
		SnapshotPath:   snapshotPath,
		Snapshot:       snapshot,
		SamtoolsBinary: os.Getenv("OTTER_SNAKEMAKE_PUBLICATION_SAMTOOLS"),
	}
	interruptionPoint := os.Getenv("OTTER_SNAKEMAKE_PUBLICATION_INTERRUPT_AFTER")
	checkpointPath := os.Getenv("OTTER_SNAKEMAKE_PUBLICATION_CHECKPOINT")
	if interruptionPoint != "" && interruptionPoint != "manifest" {
		publicationRequest.afterStagedDirectoryPublication = func(relativePath string) {
			if relativePath != interruptionPoint {
				return
			}
			if err := os.WriteFile(checkpointPath, []byte("published\n"), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			select {}
		}
	}
	if interruptionPoint == "manifest" {
		publicationRequest.afterManifestPublication = func() {
			if err := os.WriteFile(checkpointPath, []byte("published\n"), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			select {}
		}
	}
	_, err = PublishSnakemakeArtifacts(publicationRequest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestPublishSnakemakeArtifactsRecoversAfterGraftDirectoryPublicationProcessIsKilled(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioBSPDX)
	writeBSPDXPublicationInputs(t, snapshot)
	checkpointPath := filepath.Join(t.TempDir(), "graft-publication-checkpoint")

	publicationProcess := exec.Command(os.Args[0], "-test.run=^TestPublishSnakemakeArtifactsInterruptedProcessHelper$")
	publicationProcess.Env = append(os.Environ(),
		"OTTER_SNAKEMAKE_PUBLICATION_HELPER=1",
		"OTTER_SNAKEMAKE_PUBLICATION_SNAPSHOT="+snapshotPath,
		"OTTER_SNAKEMAKE_PUBLICATION_SAMTOOLS="+writeFakeSamtools(t),
		"OTTER_SNAKEMAKE_PUBLICATION_CHECKPOINT="+checkpointPath,
		"OTTER_SNAKEMAKE_PUBLICATION_INTERRUPT_AFTER=pdx/graft",
	)
	if err := publicationProcess.Start(); err != nil {
		t.Fatal(err)
	}
	checkpointDeadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(checkpointPath); err == nil {
			break
		}
		if time.Now().After(checkpointDeadline) {
			_ = publicationProcess.Process.Kill()
			_ = publicationProcess.Wait()
			t.Fatal("interrupted publication did not publish the graft directory")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := publicationProcess.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := publicationProcess.Wait(); err == nil {
		t.Fatal("interrupted publication process unexpectedly exited successfully")
	}

	for _, publishedGraftArtifact := range []string{
		"pdx/graft/S01_graft_Filtered.bam",
		"pdx/graft/S01_graft_Filtered.bam.bai",
		"pdx/graft/classification.tsv",
		"pdx/graft/bismark_summary_report.html",
	} {
		if err := requireRegularFile(filepath.Join(snapshot.Paths.Results, publishedGraftArtifact)); err != nil {
			t.Fatalf("graft publication must survive interruption after its atomic move for %s: %v", publishedGraftArtifact, err)
		}
	}
	if _, err := os.Stat(filepath.Join(snapshot.Paths.Results, "methylation")); !os.IsNotExist(err) {
		t.Fatalf("methylation publication must not occur before the interruption checkpoint, stat error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName)); !os.IsNotExist(err) {
		t.Fatalf("interrupted partial publication must not create an immutable manifest, stat error=%v", err)
	}

	retryResult, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:        context.Background(),
		SnapshotPath:   snapshotPath,
		Snapshot:       snapshot,
		SamtoolsBinary: writeFakeSamtools(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if retryResult.AlreadyPublished {
		t.Fatalf("retry after first directory publication interruption must complete publication, got %#v", retryResult)
	}
	manifest, err := artifact.Load(retryResult.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	verification, err := artifact.Verify(snapshot.Paths.Results, manifest)
	if err != nil || !verification.Passed {
		t.Fatalf("retry after first directory publication interruption must verify: report=%#v error=%v", verification, err)
	}
}

func TestPublishSnakemakeArtifactsRecoversAfterRNASplicingDirectoryPublicationProcessIsKilled(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioRNASeq)
	writeRNAPublicationInputs(t, snapshot, false)
	checkpointPath := filepath.Join(t.TempDir(), "rna-splicing-publication-checkpoint")

	publicationProcess := exec.Command(os.Args[0], "-test.run=^TestPublishSnakemakeArtifactsInterruptedProcessHelper$")
	publicationProcess.Env = append(os.Environ(),
		"OTTER_SNAKEMAKE_PUBLICATION_HELPER=1",
		"OTTER_SNAKEMAKE_PUBLICATION_SNAPSHOT="+snapshotPath,
		"OTTER_SNAKEMAKE_PUBLICATION_CHECKPOINT="+checkpointPath,
		"OTTER_SNAKEMAKE_PUBLICATION_INTERRUPT_AFTER=splicing",
	)
	if err := publicationProcess.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPublicationCheckpoint(t, publicationProcess, checkpointPath, "interrupted RNA publication did not publish the splicing directory")
	if err := publicationProcess.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := publicationProcess.Wait(); err == nil {
		t.Fatal("interrupted RNA publication process unexpectedly exited successfully")
	}

	for _, publishedArtifactPath := range []string{"splicing/outcome.json", "splicing/files/events.tsv"} {
		if err := requireRegularFile(filepath.Join(snapshot.Paths.Results, publishedArtifactPath)); err != nil {
			t.Fatalf("RNA partial publication must retain %s: %v", publishedArtifactPath, err)
		}
	}
	if _, err := os.Stat(filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName)); !os.IsNotExist(err) {
		t.Fatalf("interrupted RNA partial publication must not create a manifest, stat error=%v", err)
	}

	retryResult, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if retryResult.AlreadyPublished {
		t.Fatalf("retry after RNA splicing publication interruption must finish publication, got %#v", retryResult)
	}
	manifest, err := artifact.Load(retryResult.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	verification, err := artifact.Verify(snapshot.Paths.Results, manifest)
	if err != nil || !verification.Passed {
		t.Fatalf("retry after RNA splicing interruption must verify: report=%#v error=%v", verification, err)
	}
}

func TestPublishSnakemakeArtifactsRecoversAfterRNAPDXSplicingDirectoryPublicationProcessIsKilled(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioRNAPDX)
	writeRNAPublicationInputs(t, snapshot, true)
	checkpointPath := filepath.Join(t.TempDir(), "splicing-publication-checkpoint")

	publicationProcess := exec.Command(os.Args[0], "-test.run=^TestPublishSnakemakeArtifactsInterruptedProcessHelper$")
	publicationProcess.Env = append(os.Environ(),
		"OTTER_SNAKEMAKE_PUBLICATION_HELPER=1",
		"OTTER_SNAKEMAKE_PUBLICATION_SNAPSHOT="+snapshotPath,
		"OTTER_SNAKEMAKE_PUBLICATION_SAMTOOLS="+writeFakeSamtools(t),
		"OTTER_SNAKEMAKE_PUBLICATION_CHECKPOINT="+checkpointPath,
		"OTTER_SNAKEMAKE_PUBLICATION_INTERRUPT_AFTER=splicing",
	)
	if err := publicationProcess.Start(); err != nil {
		t.Fatal(err)
	}
	waitForPublicationCheckpoint(t, publicationProcess, checkpointPath, "interrupted RNA-PDX publication did not publish the splicing directory")
	if err := publicationProcess.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := publicationProcess.Wait(); err == nil {
		t.Fatal("interrupted RNA-PDX publication process unexpectedly exited successfully")
	}

	for _, publishedArtifactPath := range []string{
		"pdx/graft/S01_graft_Filtered.bam",
		"pdx/graft/S01_graft_Filtered.bam.bai",
		"pdx/graft/classification.tsv",
		"splicing/outcome.json",
		"splicing/files/events.tsv",
	} {
		if err := requireRegularFile(filepath.Join(snapshot.Paths.Results, publishedArtifactPath)); err != nil {
			t.Fatalf("RNA-PDX partial publication must retain %s: %v", publishedArtifactPath, err)
		}
	}
	if _, err := os.Stat(filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName)); !os.IsNotExist(err) {
		t.Fatalf("interrupted RNA-PDX partial publication must not create a manifest, stat error=%v", err)
	}

	retryResult, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:        context.Background(),
		SnapshotPath:   snapshotPath,
		Snapshot:       snapshot,
		SamtoolsBinary: writeFakeSamtools(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if retryResult.AlreadyPublished {
		t.Fatalf("retry after RNA-PDX splicing publication interruption must finish publication, got %#v", retryResult)
	}
	manifest, err := artifact.Load(retryResult.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	verification, err := artifact.Verify(snapshot.Paths.Results, manifest)
	if err != nil || !verification.Passed {
		t.Fatalf("retry after RNA-PDX splicing interruption must verify: report=%#v error=%v", verification, err)
	}
}

func waitForPublicationCheckpoint(t *testing.T, publicationProcess *exec.Cmd, checkpointPath string, timeoutMessage string) {
	t.Helper()
	checkpointDeadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(checkpointPath); err == nil {
			return
		}
		if time.Now().After(checkpointDeadline) {
			_ = publicationProcess.Process.Kill()
			_ = publicationProcess.Wait()
			t.Fatal(timeoutMessage)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPublishSnakemakeArtifactsRecoversAfterManifestPublicationProcessIsKilled(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioWGBS)
	writeWGBSPublicationInputs(t, snapshot)
	checkpointPath := filepath.Join(t.TempDir(), "manifest-publication-checkpoint")

	publicationProcess := exec.Command(os.Args[0], "-test.run=^TestPublishSnakemakeArtifactsInterruptedProcessHelper$")
	publicationProcess.Env = append(os.Environ(),
		"OTTER_SNAKEMAKE_PUBLICATION_HELPER=1",
		"OTTER_SNAKEMAKE_PUBLICATION_SNAPSHOT="+snapshotPath,
		"OTTER_SNAKEMAKE_PUBLICATION_CHECKPOINT="+checkpointPath,
		"OTTER_SNAKEMAKE_PUBLICATION_INTERRUPT_AFTER=manifest",
	)
	if err := publicationProcess.Start(); err != nil {
		t.Fatal(err)
	}
	checkpointDeadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(checkpointPath); err == nil {
			break
		}
		if time.Now().After(checkpointDeadline) {
			_ = publicationProcess.Process.Kill()
			_ = publicationProcess.Wait()
			t.Fatal("interrupted publication did not create its immutable manifest")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := publicationProcess.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := publicationProcess.Wait(); err == nil {
		t.Fatal("interrupted publication process unexpectedly exited successfully")
	}

	manifestPath := filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName)
	manifest, err := artifact.Load(manifestPath)
	if err != nil {
		t.Fatalf("manifest must persist after post-manifest interruption: %v", err)
	}
	verification, err := artifact.Verify(snapshot.Paths.Results, manifest)
	if err != nil || !verification.Passed {
		t.Fatalf("manifest must already verify after post-manifest interruption: report=%#v error=%v", verification, err)
	}

	retryResult, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:      context.Background(),
		SnapshotPath: snapshotPath,
		Snapshot:     snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !retryResult.AlreadyPublished {
		t.Fatalf("retry after manifest interruption must verify existing immutable publication, got %#v", retryResult)
	}
}

func TestPublishSnakemakeArtifactsStagesBSPDXGraftArtifacts(t *testing.T) {
	snapshot, snapshotPath := writeSnakemakePublicationSnapshot(t, configv1.ScenarioBSPDX)
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "mCall", "methrixh5", "methrix_data.h5"), "hdf5 payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "bsmap", "graft", "bismark_summary_report.html"), "<html>graft summary</html>\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "bsmap", "Filtered_bams", "S01_graft_Filtered.bam"), "bam payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "bsmap", "Filtered_bams", "S01_graft_Filtered.bam.bai"), "bai payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "qc", "qc_summary.xlsx"), "xlsx payload\n")
	samtoolsPath := writeFakeSamtools(t)

	result, err := PublishSnakemakeArtifacts(SnakemakeArtifactPublicationRequest{
		Context:        context.Background(),
		SnapshotPath:   snapshotPath,
		Snapshot:       snapshot,
		SamtoolsBinary: samtoolsPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ArtifactCount != 6 {
		t.Fatalf("unexpected BS-PDX artifact count: %#v", result)
	}
	classificationPath := filepath.Join(snapshot.Paths.Results, "pdx", "graft", "classification.tsv")
	classificationContent, err := os.ReadFile(classificationPath)
	if err != nil {
		t.Fatal(err)
	}
	expectedClassification := "artifact_index\tsource_bam\tsource_bai\tmapped_reads\n0001\tS01_graft_Filtered.bam\tS01_graft_Filtered.bam.bai\t7\n"
	if string(classificationContent) != expectedClassification {
		t.Fatalf("unexpected PDX classification: %q", classificationContent)
	}
	for _, artifactPath := range []string{
		"pdx/graft/S01_graft_Filtered.bam",
		"pdx/graft/S01_graft_Filtered.bam.bai",
		"pdx/graft/bismark_summary_report.html",
		"methylation/methrix_data.h5",
	} {
		if err := requireRegularFile(filepath.Join(snapshot.Paths.Results, artifactPath)); err != nil {
			t.Fatalf("missing published PDX artifact %s: %v", artifactPath, err)
		}
	}
}

func writeWGBSPublicationInputs(t *testing.T, snapshot configv1.RunSnapshot) {
	t.Helper()
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "mCall", "methrixh5", "methrix_data.h5"), "hdf5 payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "bsmap", "hg38", "bismark_summary_report.html"), "<html>summary</html>\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "qc", "qc_summary.xlsx"), "xlsx payload\n")
}

func writeBSPDXPublicationInputs(t *testing.T, snapshot configv1.RunSnapshot) {
	t.Helper()
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "mCall", "methrixh5", "methrix_data.h5"), "hdf5 payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "bsmap", "graft", "bismark_summary_report.html"), "<html>graft summary</html>\n")
	writePDXFilteredAlignmentInputs(t, snapshot)
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "qc", "qc_summary.xlsx"), "xlsx payload\n")
}

func writeRNAPublicationInputs(t *testing.T, snapshot configv1.RunSnapshot, pdx bool) {
	t.Helper()
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "methylation", "matrix_count.txt"), "gene\tS01\nGeneA\t10\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "methylation", "matrix_norm.txt"), "gene\tS01\nGeneA\t9.5\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Results, "qc", "qc_summary.xlsx"), "xlsx payload\n")
	splicingRoot := filepath.Join(snapshot.Paths.Work, "bsmap", "RNASplicing")
	writeWorkflowArtifact(t, filepath.Join(splicingRoot, "events.tsv"), "event\tvalue\nSE\t1\n")
	writeWorkflowArtifact(t, filepath.Join(splicingRoot, "splicing-outcome.json"), fmt.Sprintf(`{
  "schema_version": "otter.rna-splicing-outcome/v1",
  "status": "produced",
  "source_root": %q,
  "artifact_paths": ["events.tsv"]
}
`, splicingRoot))
	if pdx {
		writePDXFilteredAlignmentInputs(t, snapshot)
	}
}

func writePDXFilteredAlignmentInputs(t *testing.T, snapshot configv1.RunSnapshot) {
	t.Helper()
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "bsmap", "Filtered_bams", "S01_graft_Filtered.bam"), "bam payload\n")
	writeWorkflowArtifact(t, filepath.Join(snapshot.Paths.Work, "bsmap", "Filtered_bams", "S01_graft_Filtered.bam.bai"), "bai payload\n")
}

func readPublicationInput(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func writeSnakemakePublicationSnapshot(t *testing.T, scenario configv1.Scenario) (configv1.RunSnapshot, string) {
	t.Helper()
	projectRoot := t.TempDir()
	runRoot := filepath.Join(projectRoot, "runs", "run-20260727T010203Z-abcdef")
	workRoot := filepath.Join(runRoot, "work")
	resultsRoot := filepath.Join(runRoot, "results")
	for _, directory := range []string{
		workRoot,
		resultsRoot,
		filepath.Join(projectRoot, "workflows"),
		filepath.Join(projectRoot, "rules"),
		filepath.Join(projectRoot, "environments"),
		filepath.Join(projectRoot, "schemas"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	projectConfigPath := filepath.Join(projectRoot, "project.yaml")
	samplesManifestPath := filepath.Join(projectRoot, "samples.tsv")
	referencesLockPath := filepath.Join(projectRoot, "references.lock.yaml")
	projectLockPath := filepath.Join(projectRoot, "project.lock.yaml")
	writeWorkflowArtifact(t, projectConfigPath, "schema_version: otter.project/v1\n")
	writeWorkflowArtifact(t, samplesManifestPath, "sample_id\tr1\tr2\n")
	writeWorkflowArtifact(t, referencesLockPath, "schema_version: otter.references.lock/v1\nreferences: {}\n")
	writeWorkflowArtifact(t, projectLockPath, "workflow assets\n")
	workflowAssets := []string{
		filepath.Join(projectRoot, "workflows"),
		filepath.Join(projectRoot, "rules"),
		filepath.Join(projectRoot, "environments"),
		filepath.Join(projectRoot, "schemas"),
		projectLockPath,
	}
	workflowDigest, err := runstate.DigestPaths(workflowAssets)
	if err != nil {
		t.Fatal(err)
	}
	projectDigest, err := runstate.DigestFile(projectConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	samplesDigest, err := runstate.DigestFile(samplesManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	referenceRoot, referenceDigest := writePublicationReference(t, projectRoot, scenario)
	resolvedReferences := []configv1.ResolvedReference{{
		Role:           configv1.ReferenceRolePrimary,
		ID:             "hg38",
		Release:        "test",
		RegistryRoot:   referenceRoot,
		ManifestDigest: referenceDigest,
		Fasta: configv1.ResolvedAsset{
			Type:   "fasta",
			Path:   filepath.Join(referenceRoot, "fasta", "genome.fa"),
			SHA256: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		},
		Indexes: []configv1.ResolvedAsset{{
			Type:   "bismark",
			Path:   filepath.Join(referenceRoot, "indexes", "bismark"),
			SHA256: "sha256:2222222222222222222222222222222222222222222222222222222222222222",
		}},
	}}
	if scenario == configv1.ScenarioBSPDX || scenario == configv1.ScenarioRNAPDX {
		resolvedReferences = []configv1.ResolvedReference{
			resolvedReferenceForPublicationTest(referenceRoot, referenceDigest, configv1.ReferenceRoleGraft, "graft"),
			resolvedReferenceForPublicationTest(referenceRoot, referenceDigest, configv1.ReferenceRoleHost, "host"),
		}
	}
	snapshot := configv1.RunSnapshot{
		SchemaVersion: configv1.RunSchemaVersion,
		Run: configv1.RunMetadata{
			ID:        "run-20260727T010203Z-abcdef",
			CreatedAt: "2026-07-27T01:02:03Z",
			Immutable: true,
		},
		Project: configv1.ResolvedProject{ID: "publication-test", Root: projectRoot},
		Workflow: configv1.ResolvedWorkflow{
			Scenario:  scenario,
			Toolchain: configv1.ToolchainModern,
			AssetRoot: filepath.Join(projectRoot, "workflows"),
		},
		Execution: configv1.ResolvedExecution{
			Executor: configv1.ResolvedExecutor{Value: configv1.ExecutorSnakemake, Source: configv1.SourceProject},
			Backend:  configv1.ResolvedBackend{Value: configv1.BackendLocal, Source: configv1.SourceProject},
			Site:     configv1.ResolvedString{Value: "local", Source: configv1.SourceProject},
		},
		Samples: []configv1.SampleRecord{{
			ID: "S01",
			R1: filepath.Join(projectRoot, "inputs", "S01_R1.fastq.gz"),
			R2: filepath.Join(projectRoot, "inputs", "S01_R2.fastq.gz"),
		}},
		References: configv1.ResolvedReferences{Resolved: resolvedReferences},
		Paths: configv1.RunPaths{
			RunRoot:         runRoot,
			Work:            workRoot,
			Results:         resultsRoot,
			Logs:            filepath.Join(runRoot, "logs"),
			State:           filepath.Join(runRoot, "state"),
			Metrics:         filepath.Join(runRoot, "metrics"),
			ProjectConfig:   projectConfigPath,
			SamplesManifest: samplesManifestPath,
			ReferencesLock:  referencesLockPath,
			WorkflowAssets:  workflowAssets,
		},
		Digests: configv1.RunDigests{
			Project:        projectDigest,
			Samples:        samplesDigest,
			WorkflowAssets: workflowDigest,
		},
	}
	if err := configv1.ValidateRunSnapshot(snapshot); err != nil {
		t.Fatalf("invalid publication test snapshot: %v", err)
	}
	snapshotPath := filepath.Join(runRoot, "run.yaml")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		t.Fatal(err)
	}
	encodedSnapshot, err := configv1.MarshalRunSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, encodedSnapshot, 0o644); err != nil {
		t.Fatal(err)
	}
	return snapshot, snapshotPath
}

func resolvedReferenceForPublicationTest(releaseRoot string, manifestDigest string, role configv1.ReferenceRole, identifier string) configv1.ResolvedReference {
	return configv1.ResolvedReference{
		Role:           role,
		ID:             identifier,
		Release:        "test",
		RegistryRoot:   releaseRoot,
		ManifestDigest: manifestDigest,
		Fasta: configv1.ResolvedAsset{
			Type:   "fasta",
			Path:   filepath.Join(releaseRoot, "fasta", "genome.fa"),
			SHA256: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		},
		Indexes: []configv1.ResolvedAsset{{
			Type:   "bismark",
			Path:   filepath.Join(releaseRoot, "indexes", "bismark"),
			SHA256: "sha256:2222222222222222222222222222222222222222222222222222222222222222",
		}},
	}
}

func writePublicationReference(t *testing.T, projectRoot string, scenario configv1.Scenario) (string, string) {
	t.Helper()
	releaseRoot := filepath.Join(projectRoot, "reference")
	fastaPath := filepath.Join(releaseRoot, "fasta", "genome.fa")
	indexPath := filepath.Join(releaseRoot, "indexes", "bismark", "index.bin")
	writeWorkflowArtifact(t, fastaPath, ">chr1\nACGT\n")
	writeWorkflowArtifact(t, filepath.Join(releaseRoot, "fasta", "genome.fa.fai"), "chr1\t4\t0\t4\t5\n")
	writeWorkflowArtifact(t, indexPath, "index\n")
	fastaDigest, err := runstate.DigestFile(fastaPath)
	if err != nil {
		t.Fatal(err)
	}
	indexDigest, err := runstate.DigestFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	indexManifestDigest := reference.ComputeDigest("index.bin:" + indexDigest)
	definition := fmt.Sprintf(`schema_version: otter.reference/v1
reference:
  id: hg38
  release: test
  organism: Homo sapiens
  assembly: test
assets:
  fasta:
    path: fasta/genome.fa
    sha256: %s
    size_bytes: 11
    fai: fasta/genome.fa.fai
  indexes:
    - type: bismark
      path: indexes/bismark
      reference_fasta_sha256: %s
      tool: bismark
      tool_version: test
      manifest_sha256: %s
compatibility:
  scenarios: [%s]
`, fastaDigest, fastaDigest, indexManifestDigest, scenario)
	writeWorkflowArtifact(t, filepath.Join(releaseRoot, "reference.yaml"), definition)
	manifest, err := reference.BuildManifest(releaseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.Write(""); err != nil {
		t.Fatal(err)
	}
	manifestDigest, err := runstate.DigestFile(filepath.Join(releaseRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	return releaseRoot, manifestDigest
}

func writeFakeSamtools(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "samtools")
	script := `#!/bin/sh
case "$1" in
  quickcheck) exit 0 ;;
  view) printf '7\n' ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeBlockingSamtools(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blocking-samtools")
	script := `#!/bin/sh
if [ "$1" != "quickcheck" ]; then
  exit 1
fi
printf 'staged\n' > "$OTTER_SNAKEMAKE_PUBLICATION_CHECKPOINT"
sleep 5
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeWorkflowArtifact(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
