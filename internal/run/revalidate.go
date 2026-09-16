package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/reference"
)

type DriftIssue struct {
	Field    string
	Path     string
	Expected string
	Actual   string
}

type SnapshotDriftError struct {
	Issues []DriftIssue
}

func (driftError *SnapshotDriftError) Error() string {
	if len(driftError.Issues) == 0 {
		return "run snapshot drift detected"
	}
	issue := driftError.Issues[0]
	return fmt.Sprintf("run snapshot drift detected for %s at %q: expected %s, got %s", issue.Field, issue.Path, issue.Expected, issue.Actual)
}

func EnrichSampleInputMetadata(records []configv1.SampleRecord) ([]configv1.SampleRecord, error) {
	enrichedRecords := append([]configv1.SampleRecord(nil), records...)
	for recordIndex := range enrichedRecords {
		record := &enrichedRecords[recordIndex]
		r1Digest, r1Size, err := digestInputFile(record.R1)
		if err != nil {
			return nil, fmt.Errorf("sample %q R1: %w", record.ID, err)
		}
		r2Digest, r2Size, err := digestInputFile(record.R2)
		if err != nil {
			return nil, fmt.Errorf("sample %q R2: %w", record.ID, err)
		}
		record.R1SHA256 = r1Digest
		record.R1Size = r1Size
		record.R2SHA256 = r2Digest
		record.R2Size = r2Size
	}
	return enrichedRecords, nil
}

func RevalidateSnapshot(snapshot configv1.RunSnapshot) error {
	issues := make([]DriftIssue, 0)

	projectPath := snapshot.Paths.ProjectConfig
	if projectPath == "" {
		projectPath = filepath.Join(snapshot.Project.Root, "project.yaml")
	}
	issues = appendDigestIssue(issues, "project", projectPath, snapshot.Digests.Project)

	samplesPath := snapshot.Paths.SamplesManifest
	if samplesPath != "" {
		issues = appendDigestIssue(issues, "samples", samplesPath, snapshot.Digests.Samples)
	}

	workflowAssets := snapshot.Paths.WorkflowAssets
	if len(workflowAssets) == 0 {
		workflowAssets = defaultWorkflowAssetPaths(snapshot.Project.Root)
	}
	if snapshot.Digests.WorkflowAssets != "" {
		actualWorkflowDigest, err := DigestPaths(workflowAssets)
		if err != nil {
			issues = append(issues, DriftIssue{Field: "workflow_assets", Path: strings.Join(workflowAssets, ","), Expected: snapshot.Digests.WorkflowAssets, Actual: "error: " + err.Error()})
		} else if actualWorkflowDigest != snapshot.Digests.WorkflowAssets {
			issues = append(issues, DriftIssue{Field: "workflow_assets", Path: strings.Join(workflowAssets, ","), Expected: snapshot.Digests.WorkflowAssets, Actual: actualWorkflowDigest})
		}
	}

	referencesLockPath := snapshot.Paths.ReferencesLock
	if referencesLockPath == "" {
		referencesLockPath = filepath.Join(snapshot.Project.Root, "references.lock.yaml")
	}
	if snapshot.Digests.References != "" {
		issues = appendDigestIssue(issues, "references", referencesLockPath, snapshot.Digests.References)
	}

	for _, sample := range snapshot.Samples {
		issues = appendInputIssue(issues, sample.ID+".r1", sample.R1, sample.R1SHA256, sample.R1Size)
		issues = appendInputIssue(issues, sample.ID+".r2", sample.R2, sample.R2SHA256, sample.R2Size)
	}

	for _, resolvedReference := range snapshot.References.Resolved {
		if err := reference.VerifyReleaseIdentity(resolvedReference.RegistryRoot, resolvedReference.ManifestDigest); err != nil {
			issues = append(issues, DriftIssue{
				Field:    "reference." + string(resolvedReference.Role),
				Path:     resolvedReference.RegistryRoot,
				Expected: resolvedReference.ManifestDigest,
				Actual:   "error: " + err.Error(),
			})
		}
	}

	if len(issues) > 0 {
		return &SnapshotDriftError{Issues: issues}
	}
	return nil
}

func appendDigestIssue(issues []DriftIssue, field, path, expected string) []DriftIssue {
	if expected == "" || path == "" {
		return issues
	}
	actual, err := DigestFile(path)
	if err != nil {
		return append(issues, DriftIssue{Field: field, Path: path, Expected: expected, Actual: "error: " + err.Error()})
	}
	if actual != expected {
		return append(issues, DriftIssue{Field: field, Path: path, Expected: expected, Actual: actual})
	}
	return issues
}

func appendInputIssue(issues []DriftIssue, field, path, expectedDigest string, expectedSize int64) []DriftIssue {
	if expectedDigest == "" && expectedSize == 0 {
		return issues
	}
	actualDigest, actualSize, err := digestInputFile(path)
	if err != nil {
		return append(issues, DriftIssue{Field: field, Path: path, Expected: expectedDigest, Actual: "error: " + err.Error()})
	}
	if actualDigest != expectedDigest || actualSize != expectedSize {
		return append(issues, DriftIssue{
			Field:    field,
			Path:     path,
			Expected: fmt.Sprintf("%s size=%d", expectedDigest, expectedSize),
			Actual:   fmt.Sprintf("%s size=%d", actualDigest, actualSize),
		})
	}
	return issues
}

func digestInputFile(path string) (string, int64, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", 0, fmt.Errorf("inspect input %q: %w", path, err)
	}
	if !fileInfo.Mode().IsRegular() {
		return "", 0, fmt.Errorf("input %q is not a regular file", path)
	}
	digest, err := DigestFile(path)
	if err != nil {
		return "", 0, err
	}
	return digest, fileInfo.Size(), nil
}

func defaultWorkflowAssetPaths(projectRoot string) []string {
	return []string{
		filepath.Join(projectRoot, "workflows"),
		filepath.Join(projectRoot, "rules"),
		filepath.Join(projectRoot, "environments"),
		filepath.Join(projectRoot, "schemas"),
		filepath.Join(projectRoot, "project.lock.yaml"),
	}
}
