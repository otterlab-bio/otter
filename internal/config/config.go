package config

// OtterConfig represents the complete otter configuration
type OtterConfig struct {
	Workflow      WorkflowConfig        `mapstructure:"workflow"`
	Input         InputConfig           `mapstructure:"input"`
	Output        OutputConfig          `mapstructure:"output"`
	Reference     ReferenceConfig       `mapstructure:"reference"`
	Directories   DirectoryConfig       `mapstructure:"directories"`
	Parallel      ParallelConfig        `mapstructure:"parallel"`
	Metadata      MetadataConfig        `mapstructure:"metadata"`
	Engine        EngineConfig          `mapstructure:"engine"`
	StepResources map[int]*StepResource `mapstructure:"step_resources,omitempty"`
}

// WorkflowConfig represents the main workflow configuration
type WorkflowConfig struct {
	Mode      string          `mapstructure:"mode"` // RRBS/WGBS/RNASEQ/PDX
	UserID    string          `mapstructure:"userid"`
	JobID     string          `mapstructure:"jobid"`
	Species   SpeciesConfig   `mapstructure:"species"`
	Adapters  AdapterConfig   `mapstructure:"adapters"`
	Trim      TrimConfig      `mapstructure:"trim"`
	Alignment AlignmentConfig `mapstructure:"alignment"`
	Samples   []SampleConfig  `mapstructure:"samples,omitempty"`
}

// SpeciesConfig represents species configuration.
//
// Name is a sequence because the Snakemake assets index it positionally:
// "config["workflow"]["species"]["name"].index(config["workflow"]["species"]["graft"])"
// (see inst/rules/methrix_object.smk and inst/rules/rnaseq_mapping.smk). The
// workflow species arrays, the Craftmake compatibility projection, and the
// records written by "otter create" therefore all use a list. Legacy scalar
// configurations still load because the decoder lifts a scalars into a
// one-element sequence.
type SpeciesConfig struct {
	Primary    string   `mapstructure:"primary"`
	Secondary  string   `mapstructure:"secondary"`
	Graft      string   `mapstructure:"graft"`
	Host       string   `mapstructure:"host"`
	Name       []string `mapstructure:"name"`
	Expression string   `mapstructure:"expression"`
}

// AdapterConfig represents adapter configuration
type AdapterConfig struct {
	Seq1      []string `mapstructure:"seq1"`
	Seq2      []string `mapstructure:"seq2"`
	ErrorRate float64  `mapstructure:"error"`
}

// TrimConfig represents trimming parameters
type TrimConfig struct {
	Read1Five  float64 `mapstructure:"read1_5"`
	Read1Three float64 `mapstructure:"read1_3"`
	Read2Five  float64 `mapstructure:"read2_5"`
	Read2Three float64 `mapstructure:"read2_3"`
	SeqDepth   float64 `mapstructure:"seq_deth"`
	Fixed      string  `mapstructure:"fixed"`
}

// AlignmentConfig represents alignment parameters
type AlignmentConfig struct {
	C1 string `mapstructure:"c1"` // e.g., "7"
	C2 string `mapstructure:"c2"` // e.g., "9"
	T1 int    `mapstructure:"t1"` // e.g., 0
	T2 int    `mapstructure:"t2"` // e.g., 0
}

// SampleConfig represents a sample
type SampleConfig struct {
	Name string `mapstructure:"name"`
	R1   string `mapstructure:"r1"`
	R2   string `mapstructure:"r2"`
}

// InputConfig represents input file configuration
type InputConfig struct {
	FastqDir  string `mapstructure:"fastq_dir"`
	PdataFile string `mapstructure:"pdata_file"`
	Suffix1   string `mapstructure:"suffix"`
	Suffix2   string `mapstructure:"suffix2"`
}

// OutputConfig represents output directory configuration
type OutputConfig struct {
	BaseDir     string `mapstructure:"base_dir"`
	WorkflowDir string `mapstructure:"workflow_dir"`
	AnalysisDir string `mapstructure:"analysis_dir"`
	RawDir      string `mapstructure:"raw_dir"`
	LogDir      string `mapstructure:"log_dir"`
	TrimDir     string `mapstructure:"trim_dir"`
}

// DirectoryConfig represents all directory paths
type DirectoryConfig struct {
	Base            string        `mapstructure:"base"`
	Work            string        `mapstructure:"workDir"`
	Workflow        string        `mapstructure:"workflowDir"`
	Analysis        string        `mapstructure:"analysisDir"`
	Config          string        `mapstructure:"selfconfig"`
	QC              QCConfig      `mapstructure:"qcDir"`
	SIDLog          string        `mapstructure:"SID_log"`
	BSMAP           BSMAPConfig   `mapstructure:"bsmapDir"`
	MethylationCall string        `mapstructure:"outDir_mCall"`
	UMX             string        `mapstructure:"ourDirUmx"`
	Qualimap        string        `mapstructure:"outdir_qualimap"`
	MHAP            string        `mapstructure:"outDir_mhap"`
	RData           string        `mapstructure:"RData_folder"`
	DMR             string        `mapstructure:"DMR_folder"`
	BetaMatrix      string        `mapstructure:"outDir_betaM"`
	QCSummary       string        `mapstructure:"qc_summary"`
	LogSummary      string        `mapstructure:"logsummary"`
	UXMSummary      string        `mapstructure:"uxm_summary"`
	ClubCpG         ClubCpGConfig `mapstructure:"clubcpg"`
	MethrixH5       string        `mapstructure:"methrixh5"`
	GCBias          string        `mapstructure:"GCbias"`
}

// QCConfig represents QC directories
type QCConfig struct {
	Main   string `mapstructure:"main"`
	Before string `mapstructure:"before"`
	After  string `mapstructure:"after"`
}

// BSMAPConfig represents BSMAP directories
type BSMAPConfig struct {
	Main     string `mapstructure:"main"`
	Temp     string `mapstructure:"bamtmp"`
	Filtered string `mapstructure:"Filtered_bams"`
}

// ClubCpGConfig represents ClubCpG directories
type ClubCpGConfig struct {
	Main     string `mapstructure:"main"`
	Coverage string `mapstructure:"coverage_before"`
	Model    string `mapstructure:"model"`
	Impute   string `mapstructure:"coverage_impute"`
}

// ReferenceConfig represents reference genome configuration
type ReferenceConfig struct {
	Genome      string           `mapstructure:"genome"`
	Files       ReferenceFiles   `mapstructure:"files"`
	Indices     ReferenceIndices `mapstructure:"indices"`
	Annotations AnnotationConfig `mapstructure:"annotations"`
	RNAseq      RNAseqConfig     `mapstructure:"rnaseq"`
	// Flat fields for backward compatibility with YAML
	GenomeFasta []string `mapstructure:"genome_fasta"`
}

// ReferenceFiles represents reference files
type ReferenceFiles struct {
	Fasta    []string `mapstructure:"fasta,genome_fasta"`
	Genome   []string `mapstructure:"genomeFile"`
	CGI      string   `mapstructure:"CGI"`
	CpGSites string   `mapstructure:"cgGR_gz"`
}

// ReferenceIndices represents reference indices
type ReferenceIndices struct {
	Genome []string `mapstructure:"genome_index"`
}

// AnnotationConfig represents annotations
type AnnotationConfig struct {
	Names []string `mapstructure:"genomeAnno"`
}

// RNAseqConfig represents RNA-seq configuration
type RNAseqConfig struct {
	GTF         interface{} `mapstructure:"gtf"`
	Reference   interface{} `mapstructure:"ref"`
	Chromosomes []string    `mapstructure:"chrs"`
}

// ParallelConfig represents parallel processing configuration
type ParallelConfig struct {
	Workers      int `mapstructure:"workers"`
	DwarfWorkers int `mapstructure:"dwarf_workers"`
}

// MetadataConfig represents metadata
type MetadataConfig struct {
	SampleIDs   []string `mapstructure:"SIDs"`
	UserEmail   string   `mapstructure:"user_email"`
	PDXPipeline string   `mapstructure:"pdx_pipeline"`
	GroupLevels int      `mapstructure:"group_levels"`
}

// EngineConfig represents execution engine configuration
type EngineConfig struct {
	Type        string      `mapstructure:"type"` // auto/slurm/local
	Slurm       SlurmConfig `mapstructure:"slurm,omitempty"`
	Local       LocalConfig `mapstructure:"local,omitempty"`
	CondaEnv    string      `mapstructure:"conda_env,omitempty"`    // Conda environment for Snakemake
	FallbackEnv string      `mapstructure:"fallback_env,omitempty"` // Fallback environment (default: otter-snakemake)
	NoFallback  bool        `mapstructure:"no_fallback,omitempty"`  // Disable automatic fallback
}

// SlurmConfig represents Slurm cluster configuration
type SlurmConfig struct {
	Partition   string `mapstructure:"partition"`
	Cores       int    `mapstructure:"cores"`
	Memory      string `mapstructure:"memory"`
	Time        string `mapstructure:"time"`
	JobName     string `mapstructure:"job_name"`
	MaxRetries  int    `mapstructure:"max_retries"`
	WaitTimeout string `mapstructure:"wait_timeout"`
}

// LocalConfig represents local execution configuration
type LocalConfig struct {
	MaxCores  int    `mapstructure:"max_cores"`
	MaxMemory string `mapstructure:"max_memory"`
}

// StepResource represents per-step resource configuration
type StepResource struct {
	Cores     int    `mapstructure:"cores"`
	Memory    string `mapstructure:"memory"`
	Time      string `mapstructure:"time"`
	Partition string `mapstructure:"partition"`
	Threads   int    `mapstructure:"threads"`
	JobArray  bool   `mapstructure:"job_array"`
	MaxJobs   int    `mapstructure:"max_jobs"`
}
