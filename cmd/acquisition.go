package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	inputSRA "github.com/otterlab-bio/otter/internal/input/sra"
	"github.com/spf13/cobra"
)

type acquisitionPublishOptions struct {
	DecodeManifestPath string
	OutputPath         string
	Scenario           string
	AcquisitionID      string
	PrimaryID          string
	PrimaryRelease     string
	PrimaryManifest    string
	GraftID            string
	GraftRelease       string
	GraftManifest      string
	HostID             string
	HostRelease        string
	HostManifest       string
}

func newAcquisitionCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "acquisition",
		Short: "Publish verified input acquisition provenance",
	}
	command.AddCommand(newAcquisitionPublishCommand())
	return command
}

func newAcquisitionPublishCommand() *cobra.Command {
	options := acquisitionPublishOptions{}
	command := &cobra.Command{
		Use:   "publish",
		Short: "Convert a verified Craftmake SRA decode manifest into immutable Otter provenance",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, arguments []string) error {
			return runAcquisitionPublish(command, options)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.DecodeManifestPath, "decode-manifest", "", "Craftmake otter.sra-fastq-decode/v1 manifest")
	flags.StringVar(&options.OutputPath, "output", "", "Create-only otter.sra-acquisition/v1 output path")
	flags.StringVar(&options.Scenario, "scenario", "", "Scenario: rrbs, wgbs, rnaseq, bs-pdx, or rna-pdx")
	flags.StringVar(&options.AcquisitionID, "acquisition-id", "", "Acquisition ID; defaults to a UTC accession-derived ID")
	flags.StringVar(&options.PrimaryID, "primary-id", "", "Primary reference ID")
	flags.StringVar(&options.PrimaryRelease, "primary-release", "", "Primary reference release")
	flags.StringVar(&options.PrimaryManifest, "primary-manifest-sha256", "", "Primary reference manifest SHA-256")
	flags.StringVar(&options.GraftID, "graft-id", "", "PDX graft reference ID")
	flags.StringVar(&options.GraftRelease, "graft-release", "", "PDX graft reference release")
	flags.StringVar(&options.GraftManifest, "graft-manifest-sha256", "", "PDX graft reference manifest SHA-256")
	flags.StringVar(&options.HostID, "host-id", "", "PDX host reference ID")
	flags.StringVar(&options.HostRelease, "host-release", "", "PDX host reference release")
	flags.StringVar(&options.HostManifest, "host-manifest-sha256", "", "PDX host reference manifest SHA-256")
	return command
}

func runAcquisitionPublish(command *cobra.Command, options acquisitionPublishOptions) error {
	if strings.TrimSpace(options.DecodeManifestPath) == "" || strings.TrimSpace(options.OutputPath) == "" || strings.TrimSpace(options.Scenario) == "" {
		return fmt.Errorf("--decode-manifest, --output, and --scenario are required")
	}
	absoluteOutputPath, err := filepath.Abs(options.OutputPath)
	if err != nil {
		return fmt.Errorf("resolve acquisition output path: %w", err)
	}
	decodeManifest, err := inputSRA.LoadDecodeManifest(options.DecodeManifestPath)
	if err != nil {
		return err
	}
	createdAt := time.Now().UTC()
	acquisitionID := strings.TrimSpace(options.AcquisitionID)
	if acquisitionID == "" {
		acquisitionID = "sra-" + createdAt.Format("20060102T150405Z") + "-" + strings.ToLower(decodeManifest.Accession)
	}
	reference, err := acquisitionReference(configv1.Scenario(strings.TrimSpace(options.Scenario)), options)
	if err != nil {
		return err
	}
	manifest, err := inputSRA.BuildProvenanceManifest(options.DecodeManifestPath, inputSRA.ProvenanceOptions{
		AcquisitionID: acquisitionID,
		CreatedAt:     createdAt,
		Scenario:      configv1.Scenario(strings.TrimSpace(options.Scenario)),
		Reference:     reference,
	})
	if err != nil {
		return err
	}
	if err := inputSRA.VerifyManifestFiles(manifest); err != nil {
		return fmt.Errorf("verify acquisition files: %w", err)
	}
	if err := inputSRA.WriteManifest(absoluteOutputPath, manifest); err != nil {
		return fmt.Errorf("publish acquisition provenance: %w", err)
	}
	fmt.Fprintln(command.OutOrStdout(), absoluteOutputPath)
	return nil
}

func acquisitionReference(scenario configv1.Scenario, options acquisitionPublishOptions) (configv1.SRAAcquisitionReference, error) {
	if scenario == configv1.ScenarioBSPDX || scenario == configv1.ScenarioRNAPDX {
		if anyReferenceValue(options.PrimaryID, options.PrimaryRelease, options.PrimaryManifest) {
			return configv1.SRAAcquisitionReference{}, fmt.Errorf("PDX scenarios do not accept primary reference flags")
		}
		graft, err := acquisitionReferenceSelection(configv1.ReferenceRoleGraft, options.GraftID, options.GraftRelease, options.GraftManifest)
		if err != nil {
			return configv1.SRAAcquisitionReference{}, err
		}
		host, err := acquisitionReferenceSelection(configv1.ReferenceRoleHost, options.HostID, options.HostRelease, options.HostManifest)
		if err != nil {
			return configv1.SRAAcquisitionReference{}, err
		}
		return configv1.SRAAcquisitionReference{Species: []configv1.SRAReferenceSelection{graft, host}}, nil
	}
	if scenario != configv1.ScenarioRRBS && scenario != configv1.ScenarioWGBS && scenario != configv1.ScenarioRNASeq {
		return configv1.SRAAcquisitionReference{}, fmt.Errorf("unsupported scenario %q", scenario)
	}
	if anyReferenceValue(options.GraftID, options.GraftRelease, options.GraftManifest, options.HostID, options.HostRelease, options.HostManifest) {
		return configv1.SRAAcquisitionReference{}, fmt.Errorf("non-PDX scenarios do not accept graft or host reference flags")
	}
	primary, err := acquisitionReferenceSelection(configv1.ReferenceRolePrimary, options.PrimaryID, options.PrimaryRelease, options.PrimaryManifest)
	if err != nil {
		return configv1.SRAAcquisitionReference{}, err
	}
	return configv1.SRAAcquisitionReference{Primary: &primary}, nil
}

func acquisitionReferenceSelection(role configv1.ReferenceRole, id string, release string, manifestSHA256 string) (configv1.SRAReferenceSelection, error) {
	selection := configv1.SRAReferenceSelection{
		Role:           role,
		ID:             strings.TrimSpace(id),
		Release:        strings.TrimSpace(release),
		ManifestSHA256: strings.TrimSpace(manifestSHA256),
	}
	if selection.ID == "" || selection.Release == "" || selection.ManifestSHA256 == "" {
		return configv1.SRAReferenceSelection{}, fmt.Errorf("%s reference ID, release, and manifest SHA-256 are required", role)
	}
	return selection, nil
}

func anyReferenceValue(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(newAcquisitionCommand())
}
