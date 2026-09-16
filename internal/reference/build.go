package reference

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"gopkg.in/yaml.v3"
)

// DefaultRegistryRoot returns the user-scoped registry unless the caller has
// explicitly configured OTTER_REFERENCE_ROOT for a shared registry mount.
func DefaultRegistryRoot() (string, error) {
	if configuredRoot := strings.TrimSpace(os.Getenv(DefaultReferenceRootEnvironmentVariable)); configuredRoot != "" {
		absoluteRoot, err := filepath.Abs(configuredRoot)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", DefaultReferenceRootEnvironmentVariable, err)
		}
		return absoluteRoot, nil
	}

	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home for default reference registry: %w", err)
	}
	return filepath.Join(homeDirectory, DefaultReferenceRootDirectory), nil
}

// BuildRelease creates one immutable release below
// <registry-root>/genomes/<reference-id>/<release>. It stages every generated
// file alongside the final release so the last directory rename is atomic.
func BuildRelease(request BuildRequest) (*BuildResult, error) {
	preparedRequest, err := prepareBuildRequest(request)
	if err != nil {
		return nil, err
	}

	releaseParentDirectory := filepath.Join(preparedRequest.RegistryRoot, "genomes", preparedRequest.ReferenceID)
	finalReleaseDirectory := filepath.Join(releaseParentDirectory, preparedRequest.Release)
	if _, statErr := os.Stat(finalReleaseDirectory); statErr == nil {
		return nil, fmt.Errorf("reference release already exists: %q", finalReleaseDirectory)
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("inspect reference release %q: %w", finalReleaseDirectory, statErr)
	}
	if err := os.MkdirAll(releaseParentDirectory, 0o755); err != nil {
		return nil, fmt.Errorf("create registry release directory: %w", err)
	}

	stagingReleaseDirectory, err := os.MkdirTemp(releaseParentDirectory, ".staging-")
	if err != nil {
		return nil, fmt.Errorf("create reference staging directory: %w", err)
	}
	defer os.RemoveAll(stagingReleaseDirectory)

	definition, err := buildStagedRelease(preparedRequest, stagingReleaseDirectory)
	if err != nil {
		return nil, err
	}
	if err := configv1.ValidateReferenceDefinition(definition); err != nil {
		return nil, fmt.Errorf("validate generated reference definition: %w", err)
	}
	if err := writeReferenceDefinition(stagingReleaseDirectory, definition); err != nil {
		return nil, err
	}

	manifestReport, err := PublishRelease(stagingReleaseDirectory)
	if err != nil {
		return nil, fmt.Errorf("publish staged reference manifest: %w", err)
	}
	if err := writeChecksums(stagingReleaseDirectory, manifestReport); err != nil {
		return nil, err
	}
	manifestDigest, err := digestFile(manifestReport.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("digest staged manifest: %w", err)
	}
	verification, err := VerifyRelease(stagingReleaseDirectory, manifestDigest)
	if err != nil {
		return nil, fmt.Errorf("verify staged reference release: %w", err)
	}
	if !verification.Passed {
		return nil, fmt.Errorf("generated reference release failed verification: %s", formatVerificationIssues(verification.Issues))
	}
	if err := sealRelease(stagingReleaseDirectory); err != nil {
		return nil, err
	}
	if err := os.Rename(stagingReleaseDirectory, finalReleaseDirectory); err != nil {
		return nil, fmt.Errorf("atomically publish reference release: %w", err)
	}
	if err := syncDirectory(releaseParentDirectory); err != nil {
		return nil, err
	}

	return &BuildResult{
		RegistryRoot:   preparedRequest.RegistryRoot,
		ReleaseRoot:    finalReleaseDirectory,
		DefinitionPath: filepath.Join(finalReleaseDirectory, "reference.yaml"),
		ManifestPath:   filepath.Join(finalReleaseDirectory, "manifest.json"),
		ChecksumsPath:  filepath.Join(finalReleaseDirectory, "checksums.sha256"),
		ManifestDigest: manifestDigest,
	}, nil
}

func prepareBuildRequest(request BuildRequest) (BuildRequest, error) {
	if request.Context == nil {
		request.Context = context.Background()
	}
	if strings.TrimSpace(request.RegistryRoot) == "" {
		defaultRegistryRoot, err := DefaultRegistryRoot()
		if err != nil {
			return BuildRequest{}, err
		}
		request.RegistryRoot = defaultRegistryRoot
	}
	absoluteRegistryRoot, err := filepath.Abs(request.RegistryRoot)
	if err != nil {
		return BuildRequest{}, fmt.Errorf("resolve registry root: %w", err)
	}
	request.RegistryRoot = absoluteRegistryRoot

	for fieldName, fieldValue := range map[string]string{
		"reference id": request.ReferenceID,
		"release":      request.Release,
		"organism":     request.Organism,
		"assembly":     request.Assembly,
	} {
		if strings.TrimSpace(fieldValue) == "" {
			return BuildRequest{}, fmt.Errorf("%s is required", fieldName)
		}
	}
	if !isSafeReferenceIdentifier(request.ReferenceID) || !isSafeReferenceIdentifier(request.Release) {
		return BuildRequest{}, fmt.Errorf("reference id and release may contain only letters, digits, dots, underscores, and hyphens")
	}
	if err := validateBuildSourceFile("FASTA", request.SourceFastaPath); err != nil {
		return BuildRequest{}, err
	}
	absoluteFastaPath, err := filepath.Abs(request.SourceFastaPath)
	if err != nil {
		return BuildRequest{}, fmt.Errorf("resolve FASTA path: %w", err)
	}
	request.SourceFastaPath = absoluteFastaPath
	if err := validateBuildSourceFile("GTF", request.SourceGTFPath); err != nil {
		return BuildRequest{}, err
	}
	absoluteGTFPath, err := filepath.Abs(request.SourceGTFPath)
	if err != nil {
		return BuildRequest{}, fmt.Errorf("resolve GTF path: %w", err)
	}
	request.SourceGTFPath = absoluteGTFPath

	request.Indexes, err = normalizeIndexes(request.Indexes)
	if err != nil {
		return BuildRequest{}, err
	}
	request.Scenarios, err = normalizeScenarios(request.Scenarios, request.Indexes)
	if err != nil {
		return BuildRequest{}, err
	}
	request.Aliases = normalizeStrings(request.Aliases)
	if request.STARSJDBOverhang < 1 {
		request.STARSJDBOverhang = 149
	}
	if request.IndexBuildThreads < 1 {
		request.IndexBuildThreads = 1
	}
	request.SamtoolsBinary = defaultBinary(request.SamtoolsBinary, "samtools")
	request.BismarkBinary = defaultBinary(request.BismarkBinary, "bismark_genome_preparation")
	request.Bowtie2Binary = defaultBinary(request.Bowtie2Binary, "bowtie2-build")
	request.STARBinary = defaultBinary(request.STARBinary, "STAR")
	return request, nil
}

func buildStagedRelease(request BuildRequest, stagingReleaseDirectory string) (configv1.ReferenceDefinition, error) {
	fastaRelativePath := filepath.ToSlash(filepath.Join("fasta", filepath.Base(request.SourceFastaPath)))
	gtfRelativePath := filepath.ToSlash(filepath.Join("annotations", filepath.Base(request.SourceGTFPath)))
	stagedFastaPath := filepath.Join(stagingReleaseDirectory, filepath.FromSlash(fastaRelativePath))
	stagedGTFPath := filepath.Join(stagingReleaseDirectory, filepath.FromSlash(gtfRelativePath))
	if err := copyRegularFile(request.SourceFastaPath, stagedFastaPath); err != nil {
		return configv1.ReferenceDefinition{}, fmt.Errorf("copy FASTA into staging: %w", err)
	}
	if err := copyRegularFile(request.SourceGTFPath, stagedGTFPath); err != nil {
		return configv1.ReferenceDefinition{}, fmt.Errorf("copy GTF into staging: %w", err)
	}
	if err := runTool(request.Context, request.SamtoolsBinary, "faidx", stagedFastaPath); err != nil {
		return configv1.ReferenceDefinition{}, fmt.Errorf("build FASTA index with samtools: %w", err)
	}
	stagedFAIPath := stagedFastaPath + ".fai"
	if err := validateBuildSourceFile("generated FAI", stagedFAIPath); err != nil {
		return configv1.ReferenceDefinition{}, err
	}

	fastaDigest, err := digestFile(stagedFastaPath)
	if err != nil {
		return configv1.ReferenceDefinition{}, fmt.Errorf("digest staged FASTA: %w", err)
	}
	fastaInfo, err := os.Stat(stagedFastaPath)
	if err != nil {
		return configv1.ReferenceDefinition{}, fmt.Errorf("inspect staged FASTA: %w", err)
	}
	gtfDigest, err := digestFile(stagedGTFPath)
	if err != nil {
		return configv1.ReferenceDefinition{}, fmt.Errorf("digest staged GTF: %w", err)
	}

	indexes, err := buildIndexes(request, stagingReleaseDirectory, stagedFastaPath, stagedGTFPath, fastaDigest)
	if err != nil {
		return configv1.ReferenceDefinition{}, err
	}
	return configv1.ReferenceDefinition{
		SchemaVersion: configv1.ReferenceSchemaVersion,
		Reference: configv1.ReferenceIdentity{
			ID:       request.ReferenceID,
			Release:  request.Release,
			Organism: request.Organism,
			Assembly: request.Assembly,
			Aliases:  request.Aliases,
		},
		Assets: configv1.ReferenceAssets{
			Fasta: configv1.ReferenceFasta{
				Path:      fastaRelativePath,
				SHA256:    fastaDigest,
				SizeBytes: fastaInfo.Size(),
				FAI:       fastaRelativePath + ".fai",
			},
			Annotations: []configv1.ReferenceAnnotation{{
				ID:     "primary-gtf",
				Type:   "gtf",
				Path:   gtfRelativePath,
				SHA256: gtfDigest,
			}},
			Indexes: indexes,
		},
		Compatibility: configv1.ReferenceCompatibility{
			Scenarios: request.Scenarios,
			Workflows: workflowsForScenarios(request.Scenarios),
		},
	}, nil
}

func buildIndexes(request BuildRequest, stagingReleaseDirectory, stagedFastaPath, stagedGTFPath, fastaDigest string) ([]configv1.ReferenceIndex, error) {
	indexes := make([]configv1.ReferenceIndex, 0, len(request.Indexes))
	for _, indexType := range request.Indexes {
		indexRelativePath := filepath.ToSlash(filepath.Join("indexes", indexType))
		indexDirectory := filepath.Join(stagingReleaseDirectory, filepath.FromSlash(indexRelativePath))
		var toolBinary string
		var parameters configv1.IndexBuildParameters
		switch indexType {
		case ReferenceIndexBismark:
			toolBinary = request.BismarkBinary
			bismarkThreads := max(1, request.IndexBuildThreads/2)
			parameters.Arguments = []string{
				"--bowtie2",
				"--parallel", fmt.Sprintf("%d", bismarkThreads),
			}
			if err := buildBismarkIndex(
				request.Context,
				toolBinary,
				stagedFastaPath,
				indexDirectory,
				bismarkThreads,
			); err != nil {
				return nil, err
			}
			if err := requireNonEmptyDirectory(filepath.Join(indexDirectory, "genome", "Bisulfite_Genome", "CT_conversion"), "Bismark converted genome output"); err != nil {
				return nil, err
			}
		case ReferenceIndexBowtie2:
			toolBinary = request.Bowtie2Binary
			parameters.Arguments = []string{
				"--threads", fmt.Sprintf("%d", request.IndexBuildThreads), "genome",
			}
			if err := os.MkdirAll(indexDirectory, 0o755); err != nil {
				return nil, fmt.Errorf("create Bowtie2 index directory: %w", err)
			}
			if err := runTool(
				request.Context,
				toolBinary,
				"--threads", fmt.Sprintf("%d", request.IndexBuildThreads),
				stagedFastaPath,
				filepath.Join(indexDirectory, "genome"),
			); err != nil {
				return nil, fmt.Errorf("build Bowtie2 index: %w", err)
			}
			if err := requireOneRegularFile([]string{
				filepath.Join(indexDirectory, "genome.1.bt2"),
				filepath.Join(indexDirectory, "genome.1.bt2l"),
			}, "Bowtie2 index output"); err != nil {
				return nil, err
			}
		case ReferenceIndexSTAR:
			toolBinary = request.STARBinary
			parameters.SJDBOverhang = request.STARSJDBOverhang
			parameters.Arguments = []string{
				"--runMode", "genomeGenerate",
				"--runThreadN", fmt.Sprintf("%d", request.IndexBuildThreads),
			}
			if err := os.MkdirAll(indexDirectory, 0o755); err != nil {
				return nil, fmt.Errorf("create STAR index directory: %w", err)
			}
			if err := runTool(request.Context, toolBinary,
				"--runMode", "genomeGenerate",
				"--genomeDir", indexDirectory,
				"--genomeFastaFiles", stagedFastaPath,
				"--sjdbGTFfile", stagedGTFPath,
				"--sjdbOverhang", fmt.Sprintf("%d", request.STARSJDBOverhang),
				"--runThreadN", fmt.Sprintf("%d", request.IndexBuildThreads),
			); err != nil {
				return nil, fmt.Errorf("build STAR index: %w", err)
			}
			if err := requireRegularFile(filepath.Join(indexDirectory, "Genome"), "STAR Genome output"); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported index type %q", indexType)
		}
		indexDigest, err := digestDirectory(indexDirectory)
		if err != nil {
			return nil, fmt.Errorf("digest %s index: %w", indexType, err)
		}
		toolVersion, err := probeToolVersion(request.Context, toolBinary)
		if err != nil {
			return nil, fmt.Errorf("record %s tool version: %w", indexType, err)
		}
		indexes = append(indexes, configv1.ReferenceIndex{
			Type:                 indexType,
			Path:                 indexRelativePath,
			ReferenceFastaSHA256: fastaDigest,
			Tool:                 filepath.Base(toolBinary),
			ToolVersion:          toolVersion,
			Parameters:           parameters,
			ManifestSHA256:       indexDigest,
		})
	}
	return indexes, nil
}

func buildBismarkIndex(ctx context.Context, bismarkBinary, stagedFastaPath, indexDirectory string, indexBuildThreads int) error {
	genomeDirectory := filepath.Join(indexDirectory, "genome")
	if err := os.MkdirAll(genomeDirectory, 0o755); err != nil {
		return fmt.Errorf("create Bismark genome directory: %w", err)
	}
	bismarkFastaPath := filepath.Join(genomeDirectory, filepath.Base(stagedFastaPath))
	if err := copyRegularFile(stagedFastaPath, bismarkFastaPath); err != nil {
		return fmt.Errorf("copy FASTA for Bismark preparation: %w", err)
	}
	if err := runTool(
		ctx,
		bismarkBinary,
		"--bowtie2",
		"--parallel", fmt.Sprintf("%d", indexBuildThreads),
		genomeDirectory,
	); err != nil {
		return fmt.Errorf("build Bismark index: %w", err)
	}
	return nil
}

func writeReferenceDefinition(releaseRoot string, definition configv1.ReferenceDefinition) error {
	definitionData, err := yaml.Marshal(definition)
	if err != nil {
		return fmt.Errorf("marshal generated reference definition: %w", err)
	}
	definitionData = append(definitionData, '\n')
	definitionPath := filepath.Join(releaseRoot, "reference.yaml")
	if err := os.WriteFile(definitionPath, definitionData, 0o644); err != nil {
		return fmt.Errorf("write generated reference definition: %w", err)
	}
	return nil
}

func writeChecksums(releaseRoot string, manifestReport *ManifestReport) error {
	entries := append([]ManifestEntry(nil), manifestReport.Entries...)
	manifestDigest, err := digestFile(manifestReport.ManifestPath)
	if err != nil {
		return fmt.Errorf("digest manifest for checksums: %w", err)
	}
	entries = append(entries, ManifestEntry{Path: "manifest.json", SHA256: manifestDigest})
	sort.Slice(entries, func(leftIndex, rightIndex int) bool {
		return entries[leftIndex].Path < entries[rightIndex].Path
	})

	var checksumLines strings.Builder
	for _, entry := range entries {
		checksumLines.WriteString(strings.TrimPrefix(entry.SHA256, "sha256:"))
		checksumLines.WriteString("  ")
		checksumLines.WriteString(entry.Path)
		checksumLines.WriteByte('\n')
	}
	checksumPath := filepath.Join(releaseRoot, "checksums.sha256")
	if err := os.WriteFile(checksumPath, []byte(checksumLines.String()), 0o644); err != nil {
		return fmt.Errorf("write checksums: %w", err)
	}
	return nil
}

func sealRelease(releaseRoot string) error {
	if err := filepath.WalkDir(releaseRoot, func(path string, directoryEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if directoryEntry.IsDir() {
			if err := os.Chmod(path, 0o555); err != nil {
				return fmt.Errorf("make release directory read-only %q: %w", path, err)
			}
			return nil
		}
		if !directoryEntry.Type().IsRegular() {
			return fmt.Errorf("generated reference release contains non-regular file %q", path)
		}
		if err := os.Chmod(path, 0o444); err != nil {
			return fmt.Errorf("make release file read-only %q: %w", path, err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open release parent for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync release parent: %w", err)
	}
	return nil
}

func copyRegularFile(sourcePath, destinationPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source %q: %w", sourcePath, err)
	}
	defer sourceFile.Close()
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	destinationFile, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create destination %q: %w", destinationPath, err)
	}
	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		_ = destinationFile.Close()
		return fmt.Errorf("copy source to %q: %w", destinationPath, err)
	}
	if err := destinationFile.Sync(); err != nil {
		_ = destinationFile.Close()
		return fmt.Errorf("sync destination %q: %w", destinationPath, err)
	}
	if err := destinationFile.Close(); err != nil {
		return fmt.Errorf("close destination %q: %w", destinationPath, err)
	}
	return nil
}

func runTool(ctx context.Context, binary string, arguments ...string) error {
	command := exec.CommandContext(ctx, binary, arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", binary, strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func probeToolVersion(ctx context.Context, binary string) (string, error) {
	command := exec.CommandContext(ctx, binary, "--version")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s --version: %w\n%s", binary, err, strings.TrimSpace(string(output)))
	}
	for _, outputLine := range strings.Split(string(output), "\n") {
		trimmedLine := strings.TrimSpace(outputLine)
		if trimmedLine != "" {
			return trimmedLine, nil
		}
	}
	return "", fmt.Errorf("%s --version returned no version text", binary)
}

func requireRegularFile(path, label string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s is missing at %q: %w", label, path, err)
	}
	if !fileInfo.Mode().IsRegular() || fileInfo.Size() == 0 {
		return fmt.Errorf("%s at %q must be a non-empty regular file", label, path)
	}
	return nil
}

func requireOneRegularFile(candidatePaths []string, label string) error {
	for _, candidatePath := range candidatePaths {
		if err := requireRegularFile(candidatePath, label); err == nil {
			return nil
		}
	}
	return fmt.Errorf("%s is missing; expected one of %s", label, strings.Join(candidatePaths, ", "))
}

func requireNonEmptyDirectory(path, label string) error {
	directoryInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s is missing at %q: %w", label, path, err)
	}
	if !directoryInfo.IsDir() {
		return fmt.Errorf("%s at %q must be a directory", label, path)
	}
	hasRegularFile := false
	walkErr := filepath.WalkDir(path, func(_ string, directoryEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !directoryEntry.IsDir() && directoryEntry.Type().IsRegular() {
			hasRegularFile = true
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("inspect %s at %q: %w", label, path, walkErr)
	}
	if !hasRegularFile {
		return fmt.Errorf("%s at %q contains no regular files", label, path)
	}
	return nil
}

func validateBuildSourceFile(label, sourcePath string) error {
	if strings.TrimSpace(sourcePath) == "" {
		return fmt.Errorf("%s path is required", label)
	}
	absoluteSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolve %s path: %w", label, err)
	}
	sourceInfo, err := os.Stat(absoluteSourcePath)
	if err != nil {
		return fmt.Errorf("inspect %s %q: %w", label, absoluteSourcePath, err)
	}
	if !sourceInfo.Mode().IsRegular() || sourceInfo.Size() == 0 {
		return fmt.Errorf("%s %q must be a non-empty regular file", label, absoluteSourcePath)
	}
	return nil
}

func normalizeIndexes(indexes []string) ([]string, error) {
	if len(indexes) == 0 {
		indexes = []string{ReferenceIndexBismark, ReferenceIndexBowtie2, ReferenceIndexSTAR}
	}
	seenIndexes := make(map[string]bool, len(indexes))
	for _, indexType := range indexes {
		normalizedIndexType := strings.ToLower(strings.TrimSpace(indexType))
		switch normalizedIndexType {
		case ReferenceIndexBismark, ReferenceIndexBowtie2, ReferenceIndexSTAR:
			seenIndexes[normalizedIndexType] = true
		default:
			return nil, fmt.Errorf("unsupported reference index %q", indexType)
		}
	}
	normalizedIndexes := make([]string, 0, len(seenIndexes))
	for indexType := range seenIndexes {
		normalizedIndexes = append(normalizedIndexes, indexType)
	}
	sort.Strings(normalizedIndexes)
	return normalizedIndexes, nil
}

func normalizeScenarios(scenarios []configv1.Scenario, indexes []string) ([]configv1.Scenario, error) {
	supportedScenarios := map[configv1.Scenario]bool{}
	for _, indexType := range indexes {
		switch indexType {
		case ReferenceIndexBismark:
			supportedScenarios[configv1.ScenarioRRBS] = true
			supportedScenarios[configv1.ScenarioWGBS] = true
			supportedScenarios[configv1.ScenarioBSPDX] = true
		case ReferenceIndexSTAR:
			supportedScenarios[configv1.ScenarioRNASeq] = true
			supportedScenarios[configv1.ScenarioRNAPDX] = true
		}
	}
	if len(scenarios) == 0 {
		for scenario := range supportedScenarios {
			scenarios = append(scenarios, scenario)
		}
	}
	seenScenarios := make(map[configv1.Scenario]bool, len(scenarios))
	for _, scenario := range scenarios {
		if !supportedScenarios[scenario] {
			return nil, fmt.Errorf("scenario %q is not supported by the selected reference indexes", scenario)
		}
		seenScenarios[scenario] = true
	}
	normalizedScenarios := make([]configv1.Scenario, 0, len(seenScenarios))
	for scenario := range seenScenarios {
		normalizedScenarios = append(normalizedScenarios, scenario)
	}
	sort.Slice(normalizedScenarios, func(leftIndex, rightIndex int) bool {
		return normalizedScenarios[leftIndex] < normalizedScenarios[rightIndex]
	})
	if len(normalizedScenarios) == 0 {
		return nil, fmt.Errorf("at least one supported scenario is required")
	}
	return normalizedScenarios, nil
}

func workflowsForScenarios(scenarios []configv1.Scenario) []string {
	workflowByScenario := map[configv1.Scenario]string{
		configv1.ScenarioRRBS:   "BeaverBS",
		configv1.ScenarioWGBS:   "BeaverBS",
		configv1.ScenarioRNASeq: "BeaverRNA",
		configv1.ScenarioBSPDX:  "BeaverPDX",
		configv1.ScenarioRNAPDX: "BeaverRNASEQPDX",
	}
	seenWorkflows := make(map[string]bool, len(scenarios))
	for _, scenario := range scenarios {
		seenWorkflows[workflowByScenario[scenario]] = true
	}
	workflows := make([]string, 0, len(seenWorkflows))
	for workflow := range seenWorkflows {
		workflows = append(workflows, workflow)
	}
	sort.Strings(workflows)
	return workflows
}

func normalizeStrings(values []string) []string {
	seenValues := make(map[string]bool, len(values))
	for _, value := range values {
		normalizedValue := strings.TrimSpace(value)
		if normalizedValue != "" {
			seenValues[normalizedValue] = true
		}
	}
	normalizedValues := make([]string, 0, len(seenValues))
	for value := range seenValues {
		normalizedValues = append(normalizedValues, value)
	}
	sort.Strings(normalizedValues)
	return normalizedValues
}

func defaultBinary(configuredBinary, defaultName string) string {
	if trimmedBinary := strings.TrimSpace(configuredBinary); trimmedBinary != "" {
		return trimmedBinary
	}
	return defaultName
}

func isSafeReferenceIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z') && !(character >= 'A' && character <= 'Z') && !(character >= '0' && character <= '9') && character != '.' && character != '_' && character != '-' {
			return false
		}
	}
	return true
}
