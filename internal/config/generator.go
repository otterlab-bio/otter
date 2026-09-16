package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/internal/logger"
	"gopkg.in/yaml.v3"
)

// SnakemakeConfig represents the configuration for Snakemake
type SnakemakeConfig struct {
	Mode     string   `yaml:"Mode"`
	UserID   string   `yaml:"userid"`
	JobID    string   `yaml:"jobid"`
	Species  []string `yaml:"species"`
	Graft    string   `yaml:"graft"`
	Host     string   `yaml:"host"`
	Suffix   string   `yaml:"suffix"`
	Suffix2  string   `yaml:"suffix2"`
	Error    float64  `yaml:"error"`
	TrimSeq1 string   `yaml:"trimSeq1"`
	TrimSeq2 string   `yaml:"trimSeq2"`
	C1       string   `yaml:"C1"`
	C2       string   `yaml:"C2"`
	T1       int      `yaml:"T1"`
	T2       int      `yaml:"T2"`

	// Directory paths
	WorkDir     string `yaml:"workDir"`
	WorkflowDir string `yaml:"workflowDir"`
	AnalysisDir string `yaml:"analysisDir"`
	SelfConfig  string `yaml:"selfconfig"`
	QCDir       string `yaml:"qcDir"`
	TrimDir     string `yaml:"trimDir"`
	BsmapDir    string `yaml:"bsmapDir"`
	OutDirMCall string `yaml:"outDir_mCall"`
	OutDirUmx   string `yaml:"outDir_umx"`
	LogDir      string `yaml:"SID_log"`

	// Reference files
	GenomeFile  []string    `yaml:"genomeFile"`
	GenomeFasta []string    `yaml:"genome_fasta"`
	GenomeAnno  []string    `yaml:"genomeAnno"`
	RNASEQGTF   interface{} `yaml:"rnaseq_gtf"`
	RNASEQRef   interface{} `yaml:"rnaseq_ref"`

	// Sample information
	SIDs []string `yaml:"SIDs"`

	// User info
	UserEmail string `yaml:"user_email"`
}

// GenerateSnakemakeConfig generates YAML configuration for Snakemake
func GenerateSnakemakeConfig(config *OtterConfig, samples []string, outputPath string) error {
	// Infer graft and host
	graft, host := inferGraftHost(config)

	snakeConfig := SnakemakeConfig{
		Mode:     strings.ToUpper(config.Workflow.Mode),
		UserID:   config.Workflow.UserID,
		JobID:    config.Workflow.JobID,
		Species:  getSpeciesSlice(config),
		Graft:    graft,
		Host:     host,
		Suffix:   config.Input.Suffix1,
		Suffix2:  config.Input.Suffix2,
		Error:    config.Workflow.Adapters.ErrorRate,
		TrimSeq1: config.Workflow.Adapters.Seq1[0],
		TrimSeq2: config.Workflow.Adapters.Seq2[0],
		C1:       config.Workflow.Alignment.C1,
		C2:       config.Workflow.Alignment.C2,
		T1:       config.Workflow.Alignment.T1,
		T2:       config.Workflow.Alignment.T2,

		WorkDir:     config.Output.BaseDir,
		WorkflowDir: config.Output.WorkflowDir,
		AnalysisDir: config.Output.AnalysisDir,
		SelfConfig:  config.Directories.Config,
		QCDir:       config.Directories.QC.Main,
		TrimDir:     config.Output.TrimDir,
		BsmapDir:    config.Directories.BSMAP.Main,
		OutDirMCall: config.Directories.MethylationCall,
		OutDirUmx:   config.Directories.UMX,
		LogDir:      config.Directories.SIDLog,

		GenomeFile:  config.Reference.Indices.Genome,
		GenomeFasta: config.Reference.Files.Fasta,
		GenomeAnno:  config.Reference.Annotations.Names,
		RNASEQGTF:   config.Reference.RNAseq.GTF,
		RNASEQRef:   config.Reference.RNAseq.Reference,

		SIDs:      samples,
		UserEmail: config.Metadata.UserEmail,
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(snakeConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	logger.Infof("Snakemake configuration generated: %s", outputPath)
	return nil
}

// inferGraftHost infers graft and host from species configuration
func inferGraftHost(config *OtterConfig) (graft, host string) {
	return config.Workflow.Species.Primary, config.Workflow.Species.Secondary
}

// getSpeciesSlice returns species as a slice
func getSpeciesSlice(config *OtterConfig) []string {
	if config.Workflow.Species.Secondary == "" {
		return []string{config.Workflow.Species.Primary}
	}
	return []string{config.Workflow.Species.Primary, config.Workflow.Species.Secondary}
}
