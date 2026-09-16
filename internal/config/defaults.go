package config

import (
	"os"
)

// LoadDefaults loads default configuration values
func LoadDefaults() *OtterConfig {
	// Get user info
	userID := os.Getenv("USER")
	if userID == "" {
		userID = "unknown"
	}

	userEmail := os.Getenv("USER") + "@unknown"
	if os.Getenv("EMAIL") != "" {
		userEmail = os.Getenv("EMAIL")
	}

	return &OtterConfig{
		Workflow: WorkflowConfig{
			Mode:   "RRBS",
			UserID: userID,
			JobID:  userID,
			Species: SpeciesConfig{
				Primary:   "human",
				Secondary: "",
				Graft:     "human",
				Host:      "",
				Name:      []string{"human"},
			},
			Adapters: AdapterConfig{
				Seq1:      []string{"AGATCGGAAGAGC"},
				Seq2:      []string{"AGATCGGAAGAGC"},
				ErrorRate: 0.2,
			},
			Trim: TrimConfig{
				Read1Five:  0.0,
				Read1Three: 0.0,
				Read2Five:  0.0,
				Read2Three: 0.0,
				SeqDepth:   10.0,
				Fixed:      "",
			},
			Alignment: AlignmentConfig{
				C1: "7",
				C2: "9",
				T1: 0,
				T2: 0,
			},
			Samples: []SampleConfig{},
		},
		Input: InputConfig{
			FastqDir:  "./data/fastq",
			PdataFile: "",
			Suffix1:   "_R1.fastq.gz",
			Suffix2:   "_R2.fastq.gz",
		},
		Output: OutputConfig{
			BaseDir:     "./results",
			WorkflowDir: "/workflow",
			AnalysisDir: "/analysis",
			RawDir:      "/data",
			LogDir:      "/log",
			TrimDir:     "/trim",
		},
		Reference: ReferenceConfig{
			Genome: "hg19",
			Files: ReferenceFiles{
				Fasta:    []string{},
				Genome:   []string{},
				CGI:      "",
				CpGSites: "",
			},
			Indices: ReferenceIndices{
				Genome: []string{},
			},
			Annotations: AnnotationConfig{
				Names: []string{},
			},
			RNAseq: RNAseqConfig{
				GTF:         nil,
				Reference:   nil,
				Chromosomes: []string{},
			},
		},
		Directories: DirectoryConfig{
			Base:     "./results",
			Work:     "./results",
			Workflow: "/workflow",
			Analysis: "/analysis",
			Config:   "/config",
			QC: QCConfig{
				Main:   "/QC",
				Before: "/fastqc_raw",
				After:  "/fastqc_clean",
			},
			SIDLog: "/log",
			BSMAP: BSMAPConfig{
				Main:     "/bsmap",
				Temp:     "/tmp",
				Filtered: "/Filtered_bams",
			},
			MethylationCall: "/mCall",
			UMX:             "/uxm",
			Qualimap:        "/QC/qualimap",
			MHAP:            "/mhap",
			RData:           "/RData",
			DMR:             "/DMR",
			BetaMatrix:      "/betaM",
			QCSummary:       "/QC_summary",
			LogSummary:      "/logsummary",
			UXMSummary:      "/uxm_summary",
			ClubCpG: ClubCpGConfig{
				Main:     "/clubcpg",
				Coverage: "/clubcpg_coverage_before",
				Model:    "/clubcpg_model",
				Impute:   "/clubcpg_coverage_impute",
			},
			MethrixH5: "/methrixh5",
			GCBias:    "/GCbias",
		},
		Parallel: ParallelConfig{
			Workers:      4,
			DwarfWorkers: 1,
		},
		Metadata: MetadataConfig{
			SampleIDs:   []string{},
			UserEmail:   userEmail,
			PDXPipeline: "no",
			GroupLevels: 0,
		},
	}
}

// defaultStepResources defines resource defaults that apply to all workflow modes.
// All modes (RRBS, WGBS, BSSEQ, RNASEQ) share the same per-step resource requirements.
var defaultStepResources = map[int]*StepResource{
	1: {Cores: 20, Memory: "100G", Partition: "cpu", Threads: 10, JobArray: false},
	2: {Cores: 40, Memory: "200G", Partition: "cpu", Threads: 20, JobArray: true, MaxJobs: 10},
	3: {Cores: 10, Memory: "300G", Partition: "cpu", Threads: 5, JobArray: true, MaxJobs: 10},
}

// GetDefaultStepResource returns default resource configuration for a given step.
// The mode and isPDX parameters are retained for API compatibility but do not affect the result.
func GetDefaultStepResource(step int, mode string, isPDX bool) *StepResource {
	if r, ok := defaultStepResources[step]; ok {
		copy := *r
		return &copy
	}
	return &StepResource{Cores: 20, Memory: "100G"}
}
