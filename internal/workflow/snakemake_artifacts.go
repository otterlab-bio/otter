package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/otterlab-bio/otter/internal/artifact"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	runstate "github.com/otterlab-bio/otter/internal/run"
)

const snakemakeArtifactDeclarationFileName = "snakemake-artifacts.json"

type SnakemakeArtifactPublicationRequest struct {
	Context                         context.Context
	SnapshotPath                    string
	Snapshot                        configv1.RunSnapshot
	SamtoolsBinary                  string
	DeclarationPath                 string
	afterStagedDirectoryPublication func(string)
	afterManifestPublication        func()
}

type SnakemakeArtifactPublicationResult struct {
	ManifestPath     string
	DeclarationPath  string
	ArtifactCount    int
	AlreadyPublished bool
}

type snakemakeArtifactPublicationPlan struct {
	Declarations      []artifact.Declaration
	StagedDirectories []snakemakeStagedDirectory
}

type snakemakeStagedDirectory struct {
	RelativePath string
	Populate     func(context.Context, string) error
}

func PublishSnakemakeArtifacts(request SnakemakeArtifactPublicationRequest) (SnakemakeArtifactPublicationResult, error) {
	if request.Context == nil {
		request.Context = context.Background()
	}
	if request.Snapshot.Execution.Executor.Value != configv1.ExecutorSnakemake {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("Snakemake artifact publication requires executor %q", configv1.ExecutorSnakemake)
	}
	if err := configv1.ValidateRunSnapshot(request.Snapshot); err != nil {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("validate immutable run snapshot: %w", err)
	}
	if err := runstate.RevalidateSnapshot(request.Snapshot); err != nil {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("immutable run snapshot drift detected before artifact publication: %w", err)
	}
	absoluteSnapshotPath, err := filepath.Abs(request.SnapshotPath)
	if err != nil {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("resolve run snapshot path: %w", err)
	}
	expectedSnapshotPath := filepath.Join(request.Snapshot.Paths.RunRoot, "run.yaml")
	if filepath.Clean(absoluteSnapshotPath) != filepath.Clean(expectedSnapshotPath) {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("run snapshot path %q does not match immutable paths.run_root %q", absoluteSnapshotPath, request.Snapshot.Paths.RunRoot)
	}
	if request.SamtoolsBinary == "" {
		request.SamtoolsBinary = "samtools"
	}
	if request.DeclarationPath == "" {
		request.DeclarationPath = filepath.Join(request.Snapshot.Paths.Work, "publish", snakemakeArtifactDeclarationFileName)
	}
	absoluteDeclarationPath, err := filepath.Abs(request.DeclarationPath)
	if err != nil {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("resolve artifact declaration path: %w", err)
	}
	manifestPath := filepath.Join(request.Snapshot.Paths.Results, artifact.DefaultManifestFileName)
	snapshotDigest, err := runstate.DigestFile(absoluteSnapshotPath)
	if err != nil {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("digest immutable run snapshot: %w", err)
	}
	identity := artifact.PublicationIdentity{
		RunID:             request.Snapshot.Run.ID,
		RunSnapshotDigest: snapshotDigest,
		Scenario:          request.Snapshot.Workflow.Scenario,
		Toolchain:         request.Snapshot.Workflow.Toolchain,
		Executor:          request.Snapshot.Execution.Executor.Value,
		Backend:           request.Snapshot.Execution.Backend.Value,
	}
	if existingManifest, err := artifact.Load(manifestPath); err == nil {
		if err := verifySnakemakeManifestIdentity(identity, existingManifest); err != nil {
			return SnakemakeArtifactPublicationResult{}, err
		}
		verification, err := artifact.Verify(request.Snapshot.Paths.Results, existingManifest)
		if err != nil {
			return SnakemakeArtifactPublicationResult{}, fmt.Errorf("verify existing artifact manifest: %w", err)
		}
		if !verification.Passed {
			return SnakemakeArtifactPublicationResult{}, fmt.Errorf("existing immutable artifact manifest does not verify")
		}
		return SnakemakeArtifactPublicationResult{
			ManifestPath:     manifestPath,
			DeclarationPath:  absoluteDeclarationPath,
			ArtifactCount:    len(existingManifest.Artifacts),
			AlreadyPublished: true,
		}, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("inspect existing artifact manifest: %w", err)
	}

	plan, err := buildSnakemakeArtifactPublicationPlan(request.Snapshot, request.SamtoolsBinary)
	if err != nil {
		return SnakemakeArtifactPublicationResult{}, err
	}
	stagingParent := filepath.Join(request.Snapshot.Paths.Results, ".publish-staging")
	if err := os.MkdirAll(stagingParent, 0o755); err != nil {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("create results-local artifact staging parent: %w", err)
	}
	stagingRoot, err := os.MkdirTemp(stagingParent, "snakemake.")
	if err != nil {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("create results-local artifact staging directory: %w", err)
	}
	defer os.RemoveAll(stagingRoot)
	for _, stagedDirectory := range plan.StagedDirectories {
		stagedPath := filepath.Join(stagingRoot, filepath.FromSlash(stagedDirectory.RelativePath))
		if err := stagedDirectory.Populate(request.Context, stagedPath); err != nil {
			return SnakemakeArtifactPublicationResult{}, fmt.Errorf("stage %s: %w", stagedDirectory.RelativePath, err)
		}
	}
	if err := writeArtifactDeclarations(absoluteDeclarationPath, plan.Declarations); err != nil {
		return SnakemakeArtifactPublicationResult{}, err
	}
	for _, stagedDirectory := range plan.StagedDirectories {
		stagedPath := filepath.Join(stagingRoot, filepath.FromSlash(stagedDirectory.RelativePath))
		publishedPath := filepath.Join(request.Snapshot.Paths.Results, filepath.FromSlash(stagedDirectory.RelativePath))
		if err := publishStagedDirectory(stagedPath, publishedPath); err != nil {
			return SnakemakeArtifactPublicationResult{}, fmt.Errorf("publish %s: %w", stagedDirectory.RelativePath, err)
		}
		if request.afterStagedDirectoryPublication != nil {
			request.afterStagedDirectoryPublication(stagedDirectory.RelativePath)
		}
	}
	manifest, err := artifact.Publish(identity, request.Snapshot.Paths.Results, plan.Declarations)
	if err != nil {
		return SnakemakeArtifactPublicationResult{}, err
	}
	if request.afterManifestPublication != nil {
		request.afterManifestPublication()
	}
	verification, err := artifact.Verify(request.Snapshot.Paths.Results, manifest)
	if err != nil {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("verify published artifact manifest: %w", err)
	}
	if !verification.Passed {
		return SnakemakeArtifactPublicationResult{}, fmt.Errorf("published artifact manifest did not verify")
	}
	return SnakemakeArtifactPublicationResult{
		ManifestPath:    manifestPath,
		DeclarationPath: absoluteDeclarationPath,
		ArtifactCount:   len(manifest.Artifacts),
	}, nil
}

func buildSnakemakeArtifactPublicationPlan(snapshot configv1.RunSnapshot, samtoolsBinary string) (snakemakeArtifactPublicationPlan, error) {
	primaryReference, err := snakemakePublicationPrimaryReference(snapshot)
	if err != nil {
		return snakemakeArtifactPublicationPlan{}, err
	}
	qualityControlSummary := filepath.Join(snapshot.Paths.Results, "qc", "qc_summary.xlsx")
	methylationDirectory := filepath.Join(snapshot.Paths.Work, "mCall", "methrixh5")
	bismarkSummary := filepath.Join(snapshot.Paths.Work, "bsmap", primaryReference.ID, "bismark_summary_report.html")
	switch snapshot.Workflow.Scenario {
	case configv1.ScenarioRRBS, configv1.ScenarioWGBS:
		return snakemakeArtifactPublicationPlan{
			Declarations: []artifact.Declaration{
				methylationMatrixDeclaration("methylation-matrix", "methylation/methrix_data.h5"),
				htmlDeclaration("bismark-summary", "methylation/bismark_summary_report.html"),
				qualityControlSummaryDeclaration(),
			},
			StagedDirectories: []snakemakeStagedDirectory{{
				RelativePath: "methylation",
				Populate: func(_ context.Context, destinationDirectory string) error {
					if err := copyRegularFile(filepath.Join(methylationDirectory, "methrix_data.h5"), filepath.Join(destinationDirectory, "methrix_data.h5")); err != nil {
						return err
					}
					if err := copyRegularFile(bismarkSummary, filepath.Join(destinationDirectory, "bismark_summary_report.html")); err != nil {
						return err
					}
					return requireRegularFile(qualityControlSummary)
				},
			}},
		}, nil
	case configv1.ScenarioRNASeq:
		return buildSnakemakeRNAPublicationPlan(snapshot, qualityControlSummary, false, samtoolsBinary)
	case configv1.ScenarioBSPDX:
		return buildSnakemakeBSPDXPublicationPlan(snapshot, qualityControlSummary, methylationDirectory, bismarkSummary, samtoolsBinary)
	case configv1.ScenarioRNAPDX:
		return buildSnakemakeRNAPublicationPlan(snapshot, qualityControlSummary, true, samtoolsBinary)
	default:
		return snakemakeArtifactPublicationPlan{}, fmt.Errorf("unsupported Snakemake publication scenario %q", snapshot.Workflow.Scenario)
	}
}

func buildSnakemakeRNAPublicationPlan(snapshot configv1.RunSnapshot, qualityControlSummary string, pdx bool, samtoolsBinary string) (snakemakeArtifactPublicationPlan, error) {
	countMatrix := filepath.Join(snapshot.Paths.Results, "methylation", "matrix_count.txt")
	normalizedMatrix := filepath.Join(snapshot.Paths.Results, "methylation", "matrix_norm.txt")
	splicingOutcome := filepath.Join(snapshot.Paths.Work, "bsmap", "RNASplicing", "splicing-outcome.json")
	if err := requireRegularFile(countMatrix); err != nil {
		return snakemakeArtifactPublicationPlan{}, fmt.Errorf("validate expression count matrix: %w", err)
	}
	if err := requireRegularFile(normalizedMatrix); err != nil {
		return snakemakeArtifactPublicationPlan{}, fmt.Errorf("validate normalized expression matrix: %w", err)
	}
	if err := requireRegularFile(qualityControlSummary); err != nil {
		return snakemakeArtifactPublicationPlan{}, fmt.Errorf("validate quality-control workbook: %w", err)
	}
	splicingDeclarations, splicingPopulate, err := buildSplicingPublication(splicingOutcome)
	if err != nil {
		return snakemakeArtifactPublicationPlan{}, err
	}
	declarations := []artifact.Declaration{
		expressionMatrixDeclaration("expression-count-matrix", "methylation/matrix_count.txt"),
		expressionMatrixDeclaration("expression-normalized-matrix", "methylation/matrix_norm.txt"),
		qualityControlSummaryDeclaration(),
	}
	declarations = append(declarations, splicingDeclarations...)
	stagedDirectories := []snakemakeStagedDirectory{{RelativePath: "splicing", Populate: splicingPopulate}}
	if pdx {
		pdxDeclarations, graftPopulate, err := buildPDXGraftPublication(snapshot, samtoolsBinary)
		if err != nil {
			return snakemakeArtifactPublicationPlan{}, err
		}
		declarations = append(pdxDeclarations, declarations...)
		stagedDirectories = append([]snakemakeStagedDirectory{{RelativePath: "pdx/graft", Populate: graftPopulate}}, stagedDirectories...)
	}
	return snakemakeArtifactPublicationPlan{Declarations: declarations, StagedDirectories: stagedDirectories}, nil
}

func buildSnakemakeBSPDXPublicationPlan(
	snapshot configv1.RunSnapshot,
	qualityControlSummary string,
	methylationDirectory string,
	bismarkSummary string,
	samtoolsBinary string,
) (snakemakeArtifactPublicationPlan, error) {
	pdxDeclarations, graftPopulate, err := buildPDXGraftPublication(snapshot, samtoolsBinary)
	if err != nil {
		return snakemakeArtifactPublicationPlan{}, err
	}
	declarations := append([]artifact.Declaration{}, pdxDeclarations...)
	declarations = append(declarations,
		methylationMatrixDeclaration("methylation-matrix", "methylation/methrix_data.h5"),
		htmlDeclaration("graft-bismark-summary", "pdx/graft/bismark_summary_report.html"),
		qualityControlSummaryDeclaration(),
	)
	return snakemakeArtifactPublicationPlan{
		Declarations: declarations,
		StagedDirectories: []snakemakeStagedDirectory{
			{RelativePath: "pdx/graft", Populate: func(ctx context.Context, destinationDirectory string) error {
				if err := graftPopulate(ctx, destinationDirectory); err != nil {
					return err
				}
				return copyRegularFile(bismarkSummary, filepath.Join(destinationDirectory, "bismark_summary_report.html"))
			}},
			{RelativePath: "methylation", Populate: func(_ context.Context, destinationDirectory string) error {
				if err := copyRegularFile(filepath.Join(methylationDirectory, "methrix_data.h5"), filepath.Join(destinationDirectory, "methrix_data.h5")); err != nil {
					return err
				}
				return requireRegularFile(qualityControlSummary)
			}},
		},
	}, nil
}

func buildPDXGraftPublication(snapshot configv1.RunSnapshot, samtoolsBinary string) ([]artifact.Declaration, func(context.Context, string) error, error) {
	filteredBAMs, err := filepath.Glob(filepath.Join(snapshot.Paths.Work, "bsmap", "Filtered_bams", "*_Filtered.bam"))
	if err != nil {
		return nil, nil, fmt.Errorf("enumerate filtered PDX BAMs: %w", err)
	}
	if len(filteredBAMs) == 0 {
		return nil, nil, fmt.Errorf("no graft filtered BAMs were found")
	}
	sort.Strings(filteredBAMs)
	declarations := make([]artifact.Declaration, 0, len(filteredBAMs)*2+1)
	for artifactIndex, bamPath := range filteredBAMs {
		baiPath := bamPath + ".bai"
		if err := requireRegularFile(bamPath); err != nil {
			return nil, nil, fmt.Errorf("validate filtered graft BAM %q: %w", bamPath, err)
		}
		if err := requireRegularFile(baiPath); err != nil {
			return nil, nil, fmt.Errorf("validate filtered graft BAM index %q: %w", baiPath, err)
		}
		artifactNumber := artifactIndex + 1
		baseName := filepath.Base(bamPath)
		declarations = append(declarations,
			bamDeclaration(fmt.Sprintf("graft-alignment-bam-%04d", artifactNumber), filepath.ToSlash(filepath.Join("pdx/graft", baseName))),
			baiDeclaration(fmt.Sprintf("graft-alignment-bai-%04d", artifactNumber), filepath.ToSlash(filepath.Join("pdx/graft", baseName+".bai"))),
		)
	}
	declarations = append(declarations, artifact.Declaration{
		ID:        "graft-classification-summary",
		Path:      "pdx/graft/classification.tsv",
		MediaType: "text/tab-separated-values",
		Schema:    "otter.pdx-graft-classification/v1",
		Comparison: artifact.Comparison{
			Tier:       artifact.ComparisonTierScientific,
			Comparator: artifact.ComparatorPDXGraftReadCounts,
		},
	})
	return declarations, func(ctx context.Context, destinationDirectory string) error {
		if err := os.MkdirAll(destinationDirectory, 0o755); err != nil {
			return fmt.Errorf("create graft staging directory: %w", err)
		}
		classificationPath := filepath.Join(destinationDirectory, "classification.tsv")
		classificationFile, err := os.OpenFile(classificationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("create graft classification summary: %w", err)
		}
		defer classificationFile.Close()
		if _, err := fmt.Fprintln(classificationFile, "artifact_index\tsource_bam\tsource_bai\tmapped_reads"); err != nil {
			return err
		}
		for artifactIndex, sourceBAM := range filteredBAMs {
			targetBAM := filepath.Join(destinationDirectory, filepath.Base(sourceBAM))
			if err := copyRegularFile(sourceBAM, targetBAM); err != nil {
				return err
			}
			if err := copyRegularFile(sourceBAM+".bai", targetBAM+".bai"); err != nil {
				return err
			}
			if err := validateBAMWithSamtools(ctx, samtoolsBinary, targetBAM); err != nil {
				return err
			}
			mappedReads, err := countMappedReads(ctx, samtoolsBinary, targetBAM)
			if err != nil {
				return err
			}
			if mappedReads < 1 {
				return fmt.Errorf("filtered graft BAM has no mapped reads: %s", targetBAM)
			}
			if _, err := fmt.Fprintf(classificationFile, "%04d\t%s\t%s\t%d\n", artifactIndex+1, filepath.Base(targetBAM), filepath.Base(targetBAM)+".bai", mappedReads); err != nil {
				return err
			}
		}
		return classificationFile.Sync()
	}, nil
}

func buildSplicingPublication(outcomePath string) ([]artifact.Declaration, func(context.Context, string) error, error) {
	if err := requireRegularFile(outcomePath); err != nil {
		return nil, nil, fmt.Errorf("validate RNA splicing outcome: %w", err)
	}
	outcomeBytes, err := os.ReadFile(outcomePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read RNA splicing outcome: %w", err)
	}
	var outcome struct {
		SchemaVersion string   `json:"schema_version"`
		Status        string   `json:"status"`
		SourceRoot    string   `json:"source_root"`
		ArtifactPaths []string `json:"artifact_paths"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(outcomeBytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&outcome); err != nil {
		return nil, nil, fmt.Errorf("parse RNA splicing outcome: %w", err)
	}
	var trailingValue any
	if err := decoder.Decode(&trailingValue); err != io.EOF {
		if err == nil {
			return nil, nil, fmt.Errorf("parse RNA splicing outcome: multiple JSON values are not supported")
		}
		return nil, nil, fmt.Errorf("parse RNA splicing outcome: %w", err)
	}
	if outcome.SchemaVersion != "otter.rna-splicing-outcome/v1" {
		return nil, nil, fmt.Errorf("unsupported RNA splicing outcome schema %q", outcome.SchemaVersion)
	}
	if outcome.Status != "produced" && outcome.Status != "not_applicable" {
		return nil, nil, fmt.Errorf("invalid RNA splicing outcome status %q", outcome.Status)
	}
	if outcome.Status == "produced" && len(outcome.ArtifactPaths) == 0 {
		return nil, nil, fmt.Errorf("produced RNA splicing outcome requires artifact paths")
	}
	if outcome.Status == "not_applicable" && len(outcome.ArtifactPaths) != 0 {
		return nil, nil, fmt.Errorf("not_applicable RNA splicing outcome must not declare artifact paths")
	}
	if strings.TrimSpace(outcome.SourceRoot) == "" || !filepath.IsAbs(outcome.SourceRoot) {
		return nil, nil, fmt.Errorf("RNA splicing source root must be a non-empty absolute directory")
	}
	sourceRoot := filepath.Clean(outcome.SourceRoot)
	if fileInfo, err := os.Lstat(sourceRoot); err != nil || !fileInfo.IsDir() || fileInfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("RNA splicing source root must be a non-symbolic-link directory: %s", sourceRoot)
	}
	relativeArtifacts := append([]string(nil), outcome.ArtifactPaths...)
	sort.Strings(relativeArtifacts)
	declarations := []artifact.Declaration{{
		ID:        "splicing-outcome",
		Path:      "splicing/outcome.json",
		MediaType: "application/json",
		Schema:    "otter.rna-splicing-outcome/v1",
		Comparison: artifact.Comparison{
			Tier:       artifact.ComparisonTierStructural,
			Comparator: artifact.ComparatorRNASplicingOutcome,
		},
	}}
	for artifactIndex, relativeArtifactPath := range relativeArtifacts {
		cleanedRelativePath, err := safeSplicingRelativePath(relativeArtifactPath)
		if err != nil {
			return nil, nil, err
		}
		sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(cleanedRelativePath))
		if err := requirePathWithinRoot(sourceRoot, sourcePath); err != nil {
			return nil, nil, err
		}
		if err := requireRegularFile(sourcePath); err != nil {
			return nil, nil, fmt.Errorf("validate RNA splicing artifact %q: %w", relativeArtifactPath, err)
		}
		declarations = append(declarations, artifact.Declaration{
			ID:        fmt.Sprintf("splicing-output-%04d", artifactIndex+1),
			Path:      "splicing/files/" + cleanedRelativePath,
			MediaType: "application/octet-stream",
			Schema:    "otter.rna-splicing-output/v1",
			Comparison: artifact.Comparison{
				Tier:       artifact.ComparisonTierExact,
				Comparator: artifact.ComparatorExactFile,
			},
		})
	}
	return declarations, func(_ context.Context, destinationDirectory string) error {
		if err := copyRegularFile(outcomePath, filepath.Join(destinationDirectory, "outcome.json")); err != nil {
			return err
		}
		for _, relativeArtifactPath := range relativeArtifacts {
			cleanedRelativePath, err := safeSplicingRelativePath(relativeArtifactPath)
			if err != nil {
				return err
			}
			sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(cleanedRelativePath))
			destinationPath := filepath.Join(destinationDirectory, "files", filepath.FromSlash(cleanedRelativePath))
			if err := copyRegularFile(sourcePath, destinationPath); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

func snakemakePublicationPrimaryReference(snapshot configv1.RunSnapshot) (configv1.ResolvedReference, error) {
	for _, resolvedReference := range snapshot.References.Resolved {
		if resolvedReference.Role == configv1.ReferenceRolePrimary || resolvedReference.Role == configv1.ReferenceRoleGraft {
			return resolvedReference, nil
		}
	}
	return configv1.ResolvedReference{}, fmt.Errorf("Snakemake artifact publication requires a primary or graft reference")
}

func writeArtifactDeclarations(path string, declarations []artifact.Declaration) error {
	if err := artifact.ValidateDeclarations(artifact.DeclarationDocument{
		SchemaVersion: artifact.DeclarationSchemaVersion,
		Artifacts:     declarations,
	}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create artifact declaration directory: %w", err)
	}
	encodedDeclarations, err := json.MarshalIndent(artifact.DeclarationDocument{
		SchemaVersion: artifact.DeclarationSchemaVersion,
		Artifacts:     declarations,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode artifact declarations: %w", err)
	}
	temporaryPath := path + ".tmp"
	if err := os.WriteFile(temporaryPath, append(encodedDeclarations, '\n'), 0o600); err != nil {
		return fmt.Errorf("write temporary artifact declarations: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish artifact declarations: %w", err)
	}
	return nil
}

func publishStagedDirectory(stagedPath string, publishedPath string) error {
	if err := requireDirectory(stagedPath); err != nil {
		return err
	}
	if _, err := os.Lstat(publishedPath); err == nil {
		if err := compareRegularFileTrees(stagedPath, publishedPath); err != nil {
			return fmt.Errorf("existing publication differs from the staged retry payload: %w", err)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing publication: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(publishedPath), 0o755); err != nil {
		return fmt.Errorf("create publication parent directory: %w", err)
	}
	if err := os.Rename(stagedPath, publishedPath); err != nil {
		return fmt.Errorf("atomically publish staged directory: %w", err)
	}
	return nil
}

func compareRegularFileTrees(leftRoot string, rightRoot string) error {
	leftEntries, err := digestRegularFileTree(leftRoot)
	if err != nil {
		return err
	}
	rightEntries, err := digestRegularFileTree(rightRoot)
	if err != nil {
		return err
	}
	if len(leftEntries) != len(rightEntries) {
		return fmt.Errorf("file counts differ")
	}
	for path, leftDigest := range leftEntries {
		rightDigest, found := rightEntries[path]
		if !found || rightDigest != leftDigest {
			return fmt.Errorf("file %q differs", path)
		}
	}
	return nil
}

func digestRegularFileTree(root string) (map[string]string, error) {
	entries := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, directoryEntry os.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if path == root {
			return nil
		}
		if directoryEntry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("publication tree contains a symbolic link: %s", path)
		}
		if directoryEntry.IsDir() {
			return nil
		}
		if !directoryEntry.Type().IsRegular() {
			return fmt.Errorf("publication tree contains a non-regular file: %s", path)
		}
		digest, err := sha256File(path)
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries[filepath.ToSlash(relativePath)] = digest
		return nil
	})
	return entries, err
}

func copyRegularFile(sourcePath string, destinationPath string) error {
	if err := requireRegularFile(sourcePath); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source artifact: %w", err)
	}
	defer sourceFile.Close()
	destinationFile, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create destination artifact: %w", err)
	}
	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		destinationFile.Close()
		return fmt.Errorf("copy artifact: %w", err)
	}
	if err := destinationFile.Sync(); err != nil {
		destinationFile.Close()
		return fmt.Errorf("sync artifact: %w", err)
	}
	if err := destinationFile.Close(); err != nil {
		return fmt.Errorf("close destination artifact: %w", err)
	}
	return nil
}

func requireRegularFile(path string) error {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect file %q: %w", path, err)
	}
	if !fileInfo.Mode().IsRegular() || fileInfo.Size() == 0 {
		return fmt.Errorf("file %q must be a non-empty regular file", path)
	}
	return nil
}

func requireDirectory(path string) error {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect directory %q: %w", path, err)
	}
	if !fileInfo.IsDir() || fileInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("path %q must be a directory", path)
	}
	return nil
}

func safeSplicingRelativePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") {
		return "", fmt.Errorf("unsafe RNA splicing artifact path: %q", path)
	}
	cleanedPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if cleanedPath == "." || cleanedPath == ".." || strings.HasPrefix(cleanedPath, "../") {
		return "", fmt.Errorf("unsafe RNA splicing artifact path: %q", path)
	}
	return cleanedPath, nil
}

func requirePathWithinRoot(root string, path string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve RNA splicing source root: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve RNA splicing artifact: %w", err)
	}
	relativePath, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return err
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("RNA splicing artifact escapes source root: %s", path)
	}
	return nil
}

func validateBAMWithSamtools(ctx context.Context, samtoolsBinary string, bamPath string) error {
	command := exec.CommandContext(ctx, samtoolsBinary, "quickcheck", "-v", bamPath)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("samtools quickcheck %q: %w: %s", bamPath, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func countMappedReads(ctx context.Context, samtoolsBinary string, bamPath string) (int64, error) {
	command := exec.CommandContext(ctx, samtoolsBinary, "view", "-c", "-F", "4", bamPath)
	output, err := command.Output()
	if err != nil {
		return 0, fmt.Errorf("count mapped reads in %q: %w", bamPath, err)
	}
	var mappedReads int64
	if _, err := fmt.Sscan(strings.TrimSpace(string(output)), &mappedReads); err != nil {
		return 0, fmt.Errorf("parse mapped read count for %q: %w", bamPath, err)
	}
	return mappedReads, nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func methylationMatrixDeclaration(identifier string, path string) artifact.Declaration {
	return artifact.Declaration{ID: identifier, Path: path, MediaType: "application/x-hdf5", Schema: "methrix-se/v1", Comparison: artifact.Comparison{Tier: artifact.ComparisonTierScientific, Comparator: artifact.ComparatorMethrixSemantics}}
}

func htmlDeclaration(identifier string, path string) artifact.Declaration {
	return artifact.Declaration{ID: identifier, Path: path, MediaType: "text/html", Schema: "otter.bismark-summary/v1", Comparison: artifact.Comparison{Tier: artifact.ComparisonTierInformational, Comparator: "bismark-summary/v1"}}
}

func qualityControlSummaryDeclaration() artifact.Declaration {
	return artifact.Declaration{ID: "qc-summary", Path: "qc/qc_summary.xlsx", MediaType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Schema: "otter.qc-summary/v1", Comparison: artifact.Comparison{Tier: artifact.ComparisonTierStructural, Comparator: artifact.ComparatorXLSXStructure}}
}

func expressionMatrixDeclaration(identifier string, path string) artifact.Declaration {
	schema := "otter.expression-count-matrix/v1"
	if identifier == "expression-normalized-matrix" {
		schema = "otter.expression-normalized-matrix/v1"
	}
	return artifact.Declaration{ID: identifier, Path: path, MediaType: "text/tab-separated-values", Schema: schema, Comparison: artifact.Comparison{Tier: artifact.ComparisonTierScientific, Comparator: artifact.ComparatorExpressionMatrix}}
}

func bamDeclaration(identifier string, path string) artifact.Declaration {
	return artifact.Declaration{ID: identifier, Path: path, MediaType: "application/x-bam", Schema: "sam-bam/v1", Comparison: artifact.Comparison{Tier: artifact.ComparisonTierStructural, Comparator: artifact.ComparatorBAMPairStructure}}
}

func baiDeclaration(identifier string, path string) artifact.Declaration {
	return artifact.Declaration{ID: identifier, Path: path, MediaType: "application/x-bam-index", Schema: "sam-bai/v1", Comparison: artifact.Comparison{Tier: artifact.ComparisonTierStructural, Comparator: artifact.ComparatorBAMPairStructure}}
}

func verifySnakemakeManifestIdentity(identity artifact.PublicationIdentity, manifest artifact.Manifest) error {
	if manifest.RunID != identity.RunID || manifest.RunSnapshotDigest != identity.RunSnapshotDigest || manifest.Scenario != identity.Scenario || manifest.Toolchain != identity.Toolchain || manifest.Executor != identity.Executor || manifest.Backend != identity.Backend {
		return fmt.Errorf("existing artifact manifest identity does not match immutable Snakemake run snapshot")
	}
	return nil
}
