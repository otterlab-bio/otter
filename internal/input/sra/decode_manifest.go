package sra

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

const DecodeManifestSchemaVersion = "otter.sra-fastq-decode/v1"

var sraToolsVersionPattern = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)+`)

type DecodeManifest struct {
	SchemaVersion string                `json:"schema_version"`
	Accession     string                `json:"accession"`
	Archive       configv1.SRAArchive   `json:"archive"`
	Outputs       DecodeManifestOutputs `json:"outputs"`
	Tool          DecodeManifestTool    `json:"tool"`
}

type DecodeManifestOutputs struct {
	R1 configv1.SRAAcquiredFASTQ `json:"r1"`
	R2 configv1.SRAAcquiredFASTQ `json:"r2"`
}

type DecodeManifestTool struct {
	Name               string `json:"name"`
	FasterqDump        string `json:"fasterq_dump"`
	FasterqDumpVersion string `json:"fasterq_dump_version"`
	Compression        string `json:"compression"`
	PigzVersion        string `json:"pigz_version"`
}

type ProvenanceOptions struct {
	AcquisitionID string
	CreatedAt     time.Time
	Scenario      configv1.Scenario
	Reference     configv1.SRAAcquisitionReference
}

func LoadDecodeManifest(path string) (DecodeManifest, error) {
	manifestFile, err := os.Open(path)
	if err != nil {
		return DecodeManifest{}, fmt.Errorf("open SRA decode manifest %q: %w", path, err)
	}
	defer manifestFile.Close()

	decoder := json.NewDecoder(manifestFile)
	decoder.DisallowUnknownFields()
	var manifest DecodeManifest
	if err := decoder.Decode(&manifest); err != nil {
		return DecodeManifest{}, fmt.Errorf("decode SRA decode manifest %q: %w", path, err)
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		return DecodeManifest{}, fmt.Errorf("decode SRA decode manifest %q: %w", path, err)
	}
	if err := validateDecodeManifest(manifest); err != nil {
		return DecodeManifest{}, fmt.Errorf("validate SRA decode manifest %q: %w", path, err)
	}
	return manifest, nil
}

func BuildProvenanceManifest(decodeManifestPath string, options ProvenanceOptions) (configv1.SRAAcquisitionManifest, error) {
	absoluteDecodeManifestPath, err := filepath.Abs(decodeManifestPath)
	if err != nil {
		return configv1.SRAAcquisitionManifest{}, fmt.Errorf("resolve SRA decode manifest path: %w", err)
	}
	decodeManifest, err := LoadDecodeManifest(absoluteDecodeManifestPath)
	if err != nil {
		return configv1.SRAAcquisitionManifest{}, err
	}

	createdAt := options.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	acquisitionRoot := filepath.Dir(filepath.Dir(absoluteDecodeManifestPath))
	manifest := configv1.SRAAcquisitionManifest{
		SchemaVersion: configv1.SRAAcquisitionSchemaVersion,
		Acquisition: configv1.SRAAcquisitionIdentity{
			ID:        options.AcquisitionID,
			CreatedAt: configv1.Timestamp(createdAt.Format(time.RFC3339)),
			Immutable: true,
			Root:      acquisitionRoot,
		},
		Entries: []configv1.SRAAcquisitionEntry{{
			Scenario:  options.Scenario,
			Accession: decodeManifest.Accession,
			Download:  decodeManifest.Archive,
			Output: configv1.SRAAcquiredFASTQPair{
				R1: decodeManifest.Outputs.R1,
				R2: decodeManifest.Outputs.R2,
			},
			Reference: options.Reference,
			Tool: configv1.SRAAcquisitionTool{
				Name:    "sra-tools",
				Version: normalizeSRAToolsVersion(decodeManifest.Tool.FasterqDumpVersion),
				Command: buildDecodeCommand(decodeManifest),
			},
		}},
	}
	if err := ValidateManifest(manifest); err != nil {
		return configv1.SRAAcquisitionManifest{}, fmt.Errorf("build SRA acquisition provenance: %w", err)
	}
	return manifest, nil
}

func validateDecodeManifest(manifest DecodeManifest) error {
	if manifest.SchemaVersion != DecodeManifestSchemaVersion {
		return fmt.Errorf("schema_version must be %q", DecodeManifestSchemaVersion)
	}
	if !accessionPattern.MatchString(manifest.Accession) {
		return fmt.Errorf("accession %q is invalid", manifest.Accession)
	}
	if err := validateArchive(manifest.Archive); err != nil {
		return fmt.Errorf("archive: %w", err)
	}
	if err := validateFASTQPair(configv1.SRAAcquiredFASTQPair{R1: manifest.Outputs.R1, R2: manifest.Outputs.R2}); err != nil {
		return fmt.Errorf("outputs: %w", err)
	}
	if manifest.Outputs.R1.PairedRecordCount != manifest.Outputs.R2.PairedRecordCount {
		return fmt.Errorf("output R1/R2 paired_record_count values differ")
	}
	if strings.TrimSpace(manifest.Tool.Name) != "sra-tools" || strings.TrimSpace(manifest.Tool.FasterqDump) == "" || strings.TrimSpace(manifest.Tool.FasterqDumpVersion) == "" || strings.TrimSpace(manifest.Tool.Compression) == "" {
		return fmt.Errorf("tool must identify sra-tools, fasterq-dump, version, and compression")
	}
	return nil
}

func normalizeSRAToolsVersion(versionOutput string) string {
	if matchedVersion := sraToolsVersionPattern.FindString(versionOutput); matchedVersion != "" {
		return matchedVersion
	}
	return strings.TrimSpace(versionOutput)
}

func buildDecodeCommand(manifest DecodeManifest) string {
	return fmt.Sprintf(
		"%s --split-files -e 8 <verified-archive:%s>; %s",
		manifest.Tool.FasterqDump,
		manifest.Archive.Path,
		manifest.Tool.Compression,
	)
}
