package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/otterlab-bio/otter/internal/artifact"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	runstate "github.com/otterlab-bio/otter/internal/run"
	"github.com/spf13/cobra"
)

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Validate published run artifacts",
}

var artifactPublishCmd = &cobra.Command{
	Use:   "publish <run-yaml-path> <declarations-document-path>",
	Short: "Build and immutably publish a run artifact manifest",
	Args:  cobra.ExactArgs(2),
	RunE: func(command *cobra.Command, args []string) error {
		return runArtifactPublish(command, args[0], args[1])
	},
}

var artifactVerifyCmd = &cobra.Command{
	Use:   "verify <run-yaml-path>",
	Short: "Verify a run's artifact manifest and artifact checksums",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		manifestPath, err := command.Flags().GetString("manifest")
		if err != nil {
			return err
		}
		return runArtifactVerify(command, args[0], manifestPath)
	},
}

var artifactCompareCmd = &cobra.Command{
	Use:   "compare <left-run-yaml-path> <right-run-yaml-path>",
	Short: "Compare two verified run artifact manifests",
	Args:  cobra.ExactArgs(2),
	RunE: func(command *cobra.Command, args []string) error {
		return runArtifactCompare(command, args[0], args[1])
	},
}

func init() {
	artifactVerifyCmd.Flags().String("manifest", "", "Artifact manifest path; defaults to <run>/results/artifacts.json")
	rootCmd.AddCommand(artifactCmd)
	artifactCmd.AddCommand(artifactPublishCmd, artifactVerifyCmd, artifactCompareCmd)
}

func runArtifactPublish(
	command *cobra.Command,
	runSnapshotPath string,
	declarationDocumentPath string,
) error {
	snapshot, absoluteRunSnapshotPath, err := loadArtifactRunSnapshot(runSnapshotPath)
	if err != nil {
		return err
	}
	absoluteDeclarationDocumentPath, err := filepath.Abs(declarationDocumentPath)
	if err != nil {
		return fmt.Errorf("resolve artifact declaration document path: %w", err)
	}
	declarations, err := artifact.LoadDeclarations(absoluteDeclarationDocumentPath)
	if err != nil {
		return err
	}
	runSnapshotDigest, err := runstate.DigestFile(absoluteRunSnapshotPath)
	if err != nil {
		return fmt.Errorf("digest immutable run snapshot: %w", err)
	}
	if _, err := artifact.Publish(
		artifact.PublicationIdentity{
			RunID:             snapshot.Run.ID,
			RunSnapshotDigest: runSnapshotDigest,
			Scenario:          snapshot.Workflow.Scenario,
			Toolchain:         snapshot.Workflow.Toolchain,
			Executor:          snapshot.Execution.Executor.Value,
			Backend:           snapshot.Execution.Backend.Value,
		},
		snapshot.Paths.Results,
		declarations,
	); err != nil {
		return err
	}
	fmt.Fprintln(command.OutOrStdout(), filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName))
	return nil
}

func runArtifactVerify(command *cobra.Command, runSnapshotPath string, manifestPath string) error {
	snapshot, absoluteRunSnapshotPath, err := loadArtifactRunSnapshot(runSnapshotPath)
	if err != nil {
		return err
	}
	if manifestPath == "" {
		manifestPath = filepath.Join(snapshot.Paths.Results, artifact.DefaultManifestFileName)
	}
	absoluteManifestPath, err := filepath.Abs(manifestPath)
	if err != nil {
		return fmt.Errorf("resolve artifact manifest path: %w", err)
	}
	manifest, err := artifact.Load(absoluteManifestPath)
	if err != nil {
		return err
	}
	if err := verifyArtifactManifestIdentity(snapshot, absoluteRunSnapshotPath, manifest); err != nil {
		return err
	}
	report, err := artifact.Verify(snapshot.Paths.Results, manifest)
	if err != nil {
		return err
	}
	encodedReport, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode artifact verification report: %w", err)
	}
	fmt.Fprintln(command.OutOrStdout(), string(encodedReport))
	if !report.Passed {
		return fmt.Errorf("artifact verification failed with %d issue(s)", len(report.Issues))
	}
	return nil
}

func runArtifactCompare(command *cobra.Command, leftRunSnapshotPath string, rightRunSnapshotPath string) error {
	leftSnapshot, leftSnapshotPath, err := loadArtifactRunSnapshot(leftRunSnapshotPath)
	if err != nil {
		return err
	}
	rightSnapshot, rightSnapshotPath, err := loadArtifactRunSnapshot(rightRunSnapshotPath)
	if err != nil {
		return err
	}
	leftManifest, err := artifact.Load(filepath.Join(leftSnapshot.Paths.Results, artifact.DefaultManifestFileName))
	if err != nil {
		return err
	}
	rightManifest, err := artifact.Load(filepath.Join(rightSnapshot.Paths.Results, artifact.DefaultManifestFileName))
	if err != nil {
		return err
	}
	if err := verifyArtifactManifestIdentity(leftSnapshot, leftSnapshotPath, leftManifest); err != nil {
		return fmt.Errorf("validate left artifact manifest: %w", err)
	}
	if err := verifyArtifactManifestIdentity(rightSnapshot, rightSnapshotPath, rightManifest); err != nil {
		return fmt.Errorf("validate right artifact manifest: %w", err)
	}
	leftVerification, err := artifact.Verify(leftSnapshot.Paths.Results, leftManifest)
	if err != nil {
		return err
	}
	rightVerification, err := artifact.Verify(rightSnapshot.Paths.Results, rightManifest)
	if err != nil {
		return err
	}
	if !leftVerification.Passed || !rightVerification.Passed {
		return fmt.Errorf("artifact comparison requires two checksum-verified manifests")
	}
	comparisonReport, err := artifact.NewComparatorRegistry().Compare(
		leftSnapshot.Paths.Results,
		leftManifest,
		rightSnapshot.Paths.Results,
		rightManifest,
	)
	if err != nil {
		return err
	}
	encodedReport, err := json.MarshalIndent(comparisonReport, "", "  ")
	if err != nil {
		return fmt.Errorf("encode artifact comparison report: %w", err)
	}
	fmt.Fprintln(command.OutOrStdout(), string(encodedReport))
	if !comparisonReport.Passed {
		return fmt.Errorf("artifact comparison failed with %d issue(s)", len(comparisonReport.Issues))
	}
	return nil
}

func loadArtifactRunSnapshot(runSnapshotPath string) (configv1.RunSnapshot, string, error) {
	snapshot, err := configv1.LoadRunSnapshot(runSnapshotPath)
	if err != nil {
		return configv1.RunSnapshot{}, "", err
	}
	if err := runstate.RevalidateSnapshot(snapshot); err != nil {
		return configv1.RunSnapshot{}, "", fmt.Errorf("immutable run snapshot drift detected: %w", err)
	}
	absoluteRunSnapshotPath, err := filepath.Abs(runSnapshotPath)
	if err != nil {
		return configv1.RunSnapshot{}, "", fmt.Errorf("resolve run snapshot path: %w", err)
	}
	expectedRunSnapshotPath := filepath.Join(snapshot.Paths.RunRoot, "run.yaml")
	if filepath.Clean(absoluteRunSnapshotPath) != filepath.Clean(expectedRunSnapshotPath) {
		return configv1.RunSnapshot{}, "", fmt.Errorf("run snapshot path %q does not match immutable paths.run_root %q", absoluteRunSnapshotPath, snapshot.Paths.RunRoot)
	}
	return snapshot, absoluteRunSnapshotPath, nil
}

func verifyArtifactManifestIdentity(
	snapshot configv1.RunSnapshot,
	runSnapshotPath string,
	manifest artifact.Manifest,
) error {
	if manifest.RunID != snapshot.Run.ID {
		return fmt.Errorf("artifact manifest run_id %q does not match run snapshot %q", manifest.RunID, snapshot.Run.ID)
	}
	actualSnapshotDigest, err := runstate.DigestFile(runSnapshotPath)
	if err != nil {
		return fmt.Errorf("digest run snapshot %q: %w", runSnapshotPath, err)
	}
	if manifest.RunSnapshotDigest != actualSnapshotDigest {
		return fmt.Errorf("artifact manifest run_snapshot_digest mismatch: expected %s, got %s", actualSnapshotDigest, manifest.RunSnapshotDigest)
	}
	if manifest.Scenario != snapshot.Workflow.Scenario ||
		manifest.Toolchain != snapshot.Workflow.Toolchain ||
		manifest.Executor != snapshot.Execution.Executor.Value ||
		manifest.Backend != snapshot.Execution.Backend.Value {
		return fmt.Errorf("artifact manifest execution identity does not match immutable run snapshot")
	}
	return nil
}
