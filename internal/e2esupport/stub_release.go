// Package e2esupport builds offline test fixtures for Otter's end-to-end
// contracts. Nothing here downloads a genome or invokes a genome index builder.
package e2esupport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	refpkg "github.com/otterlab-bio/otter/internal/reference"
	"gopkg.in/yaml.v3"
)

// StubReleaseRequest describes one simulated reference registry release.
type StubReleaseRequest struct {
	// RegistryRoot is the registry root; the release lands at
	// <RegistryRoot>/genomes/<ReferenceID>/<Release>.
	RegistryRoot string
	// ReferenceID is the logical reference id used in selections, for example hg19.
	ReferenceID string
	// Release is the immutable release label, for example hg19-legacy.
	Release string
	// Organism is the scientific organism name.
	Organism string
	// Assembly is the assembly label.
	Assembly string
	// Aliases are optional alternate ids that resolve to this release.
	Aliases []string
	// Indexes selects which index directories to materialize. Supported values
	// are bismark, bowtie2, and star; empty means all three.
	Indexes []string
	// Scenarios overrides the compatibility scenarios. Empty means derive from
	// the selected indexes, exactly as "otter reference build" does.
	Scenarios []configv1.Scenario
	// FASTA and GTF are the stub payloads.
	FASTA string
	GTF   string
}

// StubReleaseResult reports where the simulated release landed.
type StubReleaseResult struct {
	ReleaseRoot    string
	DefinitionPath string
	ManifestPath   string
	ManifestDigest string
	Scenarios      []configv1.Scenario
}

// stubIndexFiles maps an index type onto the files that must exist for the
// production verifier to accept the release.
//
// The names intentionally mirror real tool output so a stub passes the same
// checks a real release does:
//   - Bismark requires indexes/bismark/genome/Bisulfite_Genome/CT_conversion/
//     to be a non-empty directory (see buildBismarkIndex).
//   - Bowtie2 requires one of genome.1.bt2 / genome.1.bt2l (see buildIndexes).
//   - STAR requires an indexes/star/Genome file (see buildIndexes).
var stubIndexFiles = map[string][]string{
	refpkg.ReferenceIndexBismark: {
		"genome/Bisulfite_Genome/CT_conversion/BS_CT.1.bt2",
		"genome/Bisulfite_Genome/GA_conversion/BS_GA.1.bt2",
	},
	refpkg.ReferenceIndexBowtie2: {
		"genome.1.bt2",
		"genome.2.bt2",
		"genome.rev.1.bt2",
	},
	refpkg.ReferenceIndexSTAR: {
		"Genome",
		"SA",
		"SAindex",
	},
}

// stubToolVersions records a plausible builder version per index type. These are
// fixture metadata, not measurements, and are labelled as stubs in the file.
var stubToolVersions = map[string]struct {
	Tool    string
	Version string
}{
	refpkg.ReferenceIndexBismark: {Tool: "bismark_genome_preparation", Version: "stub-e2e"},
	refpkg.ReferenceIndexBowtie2: {Tool: "bowtie2-build", Version: "stub-e2e"},
	refpkg.ReferenceIndexSTAR:    {Tool: "STAR", Version: "stub-e2e"},
}

// DefaultStubFASTA is a minimal two-contig FASTA. It is valid enough for the
// production FAI relationship check and small enough to commit as a literal.
const DefaultStubFASTA = ">chr1\nACGTACGTACGTACGTACGTACGTACGTACGT\n>chr2\nTTTTGGGGCCCCAAAATTTTGGGGCCCCAAAA\n"

// DefaultStubGTF is a minimal two-feature GTF.
const DefaultStubGTF = "chr1\tstub\texon\t1\t12\t.\t+\t.\tgene_id \"g1\"; transcript_id \"t1\";\nchr1\tstub\texon\t13\t24\t.\t+\t.\tgene_id \"g1\"; transcript_id \"t2\";\n"

// WriteStubRelease materializes one simulated registry release and then verifies
// it with the production verifier.
//
// Verifying here is the point: if the real reference contract changes shape (for
// example a new required field or a renamed index artifact), this fixture starts
// failing instead of silently diverging from production.
func WriteStubRelease(request StubReleaseRequest) (*StubReleaseResult, error) {
	if strings.TrimSpace(request.RegistryRoot) == "" {
		return nil, fmt.Errorf("registry root is required")
	}
	if strings.TrimSpace(request.ReferenceID) == "" || strings.TrimSpace(request.Release) == "" {
		return nil, fmt.Errorf("reference id and release are required")
	}
	if strings.TrimSpace(request.Organism) == "" || strings.TrimSpace(request.Assembly) == "" {
		return nil, fmt.Errorf("organism and assembly are required")
	}
	registryRoot, err := filepath.Abs(request.RegistryRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve registry root: %w", err)
	}

	indexTypes, err := normalizeStubIndexes(request.Indexes)
	if err != nil {
		return nil, err
	}
	scenarios, err := normalizeStubScenarios(request.Scenarios, indexTypes)
	if err != nil {
		return nil, err
	}

	fastaContent := request.FASTA
	if fastaContent == "" {
		fastaContent = DefaultStubFASTA
	}
	gtfContent := request.GTF
	if gtfContent == "" {
		gtfContent = DefaultStubGTF
	}

	releaseRoot := filepath.Join(registryRoot, "genomes", request.ReferenceID, request.Release)
	if _, statErr := os.Stat(releaseRoot); statErr == nil {
		return nil, fmt.Errorf("reference release already exists: %q", releaseRoot)
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("inspect release root %q: %w", releaseRoot, statErr)
	}

	fastaRelativePath := filepath.ToSlash(filepath.Join("fasta", request.ReferenceID+".fa"))
	gtfRelativePath := filepath.ToSlash(filepath.Join("annotations", request.ReferenceID+".gtf"))
	fastaPath := filepath.Join(releaseRoot, filepath.FromSlash(fastaRelativePath))
	gtfPath := filepath.Join(releaseRoot, filepath.FromSlash(gtfRelativePath))

	if err := writeStubFile(fastaPath, fastaContent); err != nil {
		return nil, err
	}
	if err := writeStubFile(gtfPath, gtfContent); err != nil {
		return nil, err
	}
	if err := writeStubFile(fastaPath+".fai", buildStubFAI(fastaContent)); err != nil {
		return nil, err
	}

	fastaDigest := refpkg.ComputeDigest(fastaContent)
	gtfDigest := refpkg.ComputeDigest(gtfContent)

	indexes := make([]configv1.ReferenceIndex, 0, len(indexTypes))
	for _, indexType := range indexTypes {
		indexRelativePath := filepath.ToSlash(filepath.Join("indexes", indexType))
		indexDirectory := filepath.Join(releaseRoot, filepath.FromSlash(indexRelativePath))
		for _, relativeFile := range stubIndexFiles[indexType] {
			target := filepath.Join(indexDirectory, filepath.FromSlash(relativeFile))
			if err := writeStubFile(target, "stub index data for "+indexType+"\n"); err != nil {
				return nil, err
			}
		}
		indexDigest, digestErr := digestStubDirectory(indexDirectory)
		if digestErr != nil {
			return nil, digestErr
		}
		tool := stubToolVersions[indexType]
		indexes = append(indexes, configv1.ReferenceIndex{
			Type:                 indexType,
			Path:                 indexRelativePath,
			ReferenceFastaSHA256: fastaDigest,
			Tool:                 tool.Tool,
			ToolVersion:          tool.Version,
			ManifestSHA256:       indexDigest,
		})
	}

	definition := configv1.ReferenceDefinition{
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
				SizeBytes: int64(len(fastaContent)),
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
		Compatibility: configv1.ReferenceCompatibility{Scenarios: scenarios},
	}
	if err := configv1.ValidateReferenceDefinition(definition); err != nil {
		return nil, fmt.Errorf("generated stub reference definition is invalid: %w", err)
	}
	definitionBytes, err := marshalStubYAML(definition)
	if err != nil {
		return nil, err
	}
	if err := writeStubFile(filepath.Join(releaseRoot, "reference.yaml"), string(definitionBytes)); err != nil {
		return nil, err
	}

	// BuildManifest + Write produce the same manifest.json bytes the production
	// publisher writes, so the manifest digest is computed identically.
	manifestReport, err := refpkg.BuildManifest(releaseRoot)
	if err != nil {
		return nil, fmt.Errorf("build stub manifest: %w", err)
	}
	if err := manifestReport.Write(""); err != nil {
		return nil, fmt.Errorf("write stub manifest: %w", err)
	}
	manifestPath := filepath.Join(releaseRoot, "manifest.json")
	manifestDigest, err := digestStubFile(manifestPath)
	if err != nil {
		return nil, err
	}

	if err := refpkg.VerifyReleaseIdentity(releaseRoot, manifestDigest); err != nil {
		return nil, fmt.Errorf("stub release failed production identity verification: %w", err)
	}

	return &StubReleaseResult{
		ReleaseRoot:    releaseRoot,
		DefinitionPath: filepath.Join(releaseRoot, "reference.yaml"),
		ManifestPath:   manifestPath,
		ManifestDigest: manifestDigest,
		Scenarios:      scenarios,
	}, nil
}

// normalizeStubIndexes validates and sorts the requested index types.
func normalizeStubIndexes(requested []string) ([]string, error) {
	if len(requested) == 0 {
		requested = []string{refpkg.ReferenceIndexBismark, refpkg.ReferenceIndexBowtie2, refpkg.ReferenceIndexSTAR}
	}
	seen := make(map[string]bool, len(requested))
	for _, indexType := range requested {
		normalized := strings.ToLower(strings.TrimSpace(indexType))
		if _, supported := stubIndexFiles[normalized]; !supported {
			return nil, fmt.Errorf("unsupported stub index type %q; expected bismark, bowtie2, or star", indexType)
		}
		seen[normalized] = true
	}
	indexTypes := make([]string, 0, len(seen))
	for indexType := range seen {
		indexTypes = append(indexTypes, indexType)
	}
	sort.Strings(indexTypes)
	return indexTypes, nil
}

// normalizeStubScenarios derives scenarios from indexes the same way
// "otter reference build" does, unless the caller states them explicitly.
func normalizeStubScenarios(requested []configv1.Scenario, indexTypes []string) ([]configv1.Scenario, error) {
	derived := make(map[configv1.Scenario]bool)
	for _, indexType := range indexTypes {
		switch indexType {
		case refpkg.ReferenceIndexBismark:
			derived[configv1.ScenarioRRBS] = true
			derived[configv1.ScenarioWGBS] = true
			derived[configv1.ScenarioBSPDX] = true
		case refpkg.ReferenceIndexSTAR:
			derived[configv1.ScenarioRNASeq] = true
			derived[configv1.ScenarioRNAPDX] = true
		}
	}
	effective := requested
	if len(effective) == 0 {
		for scenario := range derived {
			effective = append(effective, scenario)
		}
	}
	if len(effective) == 0 {
		return nil, fmt.Errorf("stub release supports no scenarios; include a bismark or star index")
	}
	seen := make(map[configv1.Scenario]bool, len(effective))
	scenarios := make([]configv1.Scenario, 0, len(effective))
	for _, scenario := range effective {
		if !derived[scenario] {
			return nil, fmt.Errorf("scenario %q is not supported by the selected stub indexes", scenario)
		}
		if seen[scenario] {
			continue
		}
		seen[scenario] = true
		scenarios = append(scenarios, scenario)
	}
	sort.Slice(scenarios, func(left, right int) bool { return scenarios[left] < scenarios[right] })
	return scenarios, nil
}

// buildStubFAI derives a samtools-faidx-shaped index from the FASTA payload so
// the production FASTA/FAI relationship check has consistent input.
//
// The byte offset of each record is tracked from the real payload rather than
// guessed, because the reference verifier reads the FAI to confirm it points at
// the matching FASTA.
func buildStubFAI(fastaContent string) string {
	var builder strings.Builder
	lines := strings.SplitAfter(fastaContent, "\n")
	byteOffset := 0
	for index := 0; index < len(lines); index++ {
		headerLine := lines[index]
		if !strings.HasPrefix(headerLine, ">") {
			byteOffset += len(headerLine)
			continue
		}
		contigName := strings.TrimRight(strings.TrimPrefix(headerLine, ">"), "\n")
		sequenceOffset := byteOffset + len(headerLine)
		sequenceLength := 0
		lineBases := 0
		lineWidth := 0
		consumedBytes := sequenceOffset
		for probe := index + 1; probe < len(lines); probe++ {
			sequenceLine := lines[probe]
			trimmedLine := strings.TrimRight(sequenceLine, "\n")
			if trimmedLine == "" {
				break
			}
			if lineBases == 0 {
				lineBases = len(trimmedLine)
				lineWidth = len(sequenceLine)
			}
			sequenceLength += len(trimmedLine)
			consumedBytes += len(sequenceLine)
		}
		builder.WriteString(fmt.Sprintf("%s\t%d\t%d\t%d\t%d\n", contigName, sequenceLength, sequenceOffset, lineBases, lineWidth))
		byteOffset = consumedBytes
	}
	return builder.String()
}

func writeStubFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create stub directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write stub file %s: %w", path, err)
	}
	return nil
}

// marshalStubYAML encodes the reference definition with the same marshaler the
// production builder uses.
func marshalStubYAML(definition configv1.ReferenceDefinition) ([]byte, error) {
	encoded, err := yaml.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("marshal stub reference definition: %w", err)
	}
	return encoded, nil
}

// digestStubFile hashes a single file the way the reference package does.
func digestStubFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

// digestStubDirectory reproduces the reference package's directory digest so
// index manifest digests match production semantics exactly.
func digestStubDirectory(directory string) (string, error) {
	var digests []string
	if err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		fileDigest, err := digestStubFile(path)
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(directory, path)
		if err != nil {
			return fmt.Errorf("resolve relative index path: %w", err)
		}
		digests = append(digests, relativePath+":"+fileDigest)
		return nil
	}); err != nil {
		return "", fmt.Errorf("walk stub index directory %q: %w", directory, err)
	}
	sort.Strings(digests)
	hasher := sha256.New()
	for _, entry := range digests {
		hasher.Write([]byte(entry))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}
