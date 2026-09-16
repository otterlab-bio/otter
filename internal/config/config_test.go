package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOtterConfig_Structure(t *testing.T) {
	// 测试配置结构体的基本字段存在性
	cfg := OtterConfig{
		Workflow: WorkflowConfig{
			Mode:   "RRBS",
			UserID: "test_user",
			JobID:  "test_job",
			Species: SpeciesConfig{
				Primary:   "human",
				Secondary: "mouse",
				Name:      []string{"human", "mouse"},
			},
			Adapters: AdapterConfig{
				Seq1:      []string{"AGATCGGAAGAGC"},
				Seq2:      []string{"AGATCGGAAGAGC"},
				ErrorRate: 0.2,
			},
		},
		Reference: ReferenceConfig{
			Genome: "hg19",
		},
	}

	// 验证 Workflow 配置
	assert.Equal(t, "RRBS", cfg.Workflow.Mode)
	assert.Equal(t, "test_user", cfg.Workflow.UserID)
	assert.Equal(t, "test_job", cfg.Workflow.JobID)

	// 验证 Species 配置
	assert.Equal(t, "human", cfg.Workflow.Species.Primary)
	assert.Equal(t, "mouse", cfg.Workflow.Species.Secondary)
	assert.Equal(t, []string{"human", "mouse"}, cfg.Workflow.Species.Name)

	// 验证 Adapters 配置
	assert.Equal(t, []string{"AGATCGGAAGAGC"}, cfg.Workflow.Adapters.Seq1)
	assert.Equal(t, []string{"AGATCGGAAGAGC"}, cfg.Workflow.Adapters.Seq2)
	assert.Equal(t, 0.2, cfg.Workflow.Adapters.ErrorRate)

	// 验证 Reference 配置
	assert.Equal(t, "hg19", cfg.Reference.Genome)
}

func TestNestedConfig_Marshaling(t *testing.T) {
	cfg := OtterConfig{
		Workflow: WorkflowConfig{
			Mode:   "WGBS",
			UserID: "user123",
			JobID:  "job456",
			Species: SpeciesConfig{
				Primary:   "mouse",
				Secondary: "",
				Graft:     "mouse",
				Host:      "",
				Name:      []string{"mouse"},
			},
		},
		Output: OutputConfig{
			BaseDir:     "./userspace",
			WorkflowDir: "./userspace/job456/workflow",
			AnalysisDir: "./userspace/job456/analysis",
			RawDir:      "./userspace/job456/data",
			LogDir:      "./userspace/job456/log",
			TrimDir:     "./userspace/job456/workflow/trim",
		},
	}

	// 验证嵌套结构可以正确创建
	assert.Equal(t, "WGBS", cfg.Workflow.Mode)
	assert.Equal(t, "mouse", cfg.Workflow.Species.Primary)
	assert.Equal(t, "./userspace", cfg.Output.BaseDir)
	assert.Equal(t, "./userspace/job456/workflow", cfg.Output.WorkflowDir)
}

func TestWorkflowConfig_EmptySamples(t *testing.T) {
	cfg := WorkflowConfig{
		Mode:    "RRBS",
		UserID:  "test",
		JobID:   "123",
		Samples: nil,
	}

	// 验证 Samples 可以为空（使用 omitempty 标签）
	assert.NotNil(t, cfg)
	assert.Equal(t, "RRBS", cfg.Mode)
	assert.Nil(t, cfg.Samples)
}

func TestAdapterConfig_ErrorRate(t *testing.T) {
	cfg := AdapterConfig{
		Seq1:      []string{"AGATCGGAAGAGC"},
		Seq2:      []string{"AGATCGGAAGAGC"},
		ErrorRate: 0.2,
	}

	assert.Equal(t, 0.2, cfg.ErrorRate)
	assert.Equal(t, 1, len(cfg.Seq1))
	assert.Equal(t, 1, len(cfg.Seq2))
}

func TestTrimConfig_DefaultValues(t *testing.T) {
	trim := TrimConfig{
		Read1Five:  0.0,
		Read1Three: 0.0,
		Read2Five:  0.0,
		Read2Three: 0.0,
		SeqDepth:   10.0,
		Fixed:      "",
	}

	assert.Equal(t, 0.0, trim.Read1Five)
	assert.Equal(t, 0.0, trim.Read1Three)
	assert.Equal(t, 10.0, trim.SeqDepth)
	assert.Equal(t, "", trim.Fixed)
}

func TestAlignmentConfig_DefaultValues(t *testing.T) {
	align := AlignmentConfig{
		C1: "7",
		C2: "9",
		T1: 0,
		T2: 0,
	}

	assert.Equal(t, "7", align.C1)
	assert.Equal(t, "9", align.C2)
	assert.Equal(t, 0, align.T1)
	assert.Equal(t, 0, align.T2)
}

func TestInputConfig_Suffixes(t *testing.T) {
	input := InputConfig{
		FastqDir:  "/data/fastq",
		PdataFile: "",
		Suffix1:   "_R1.fastq.gz",
		Suffix2:   "_R2.fastq.gz",
	}

	assert.Equal(t, "/data/fastq", input.FastqDir)
	assert.Equal(t, "_R1.fastq.gz", input.Suffix1)
	assert.Equal(t, "_R2.fastq.gz", input.Suffix2)
}

func TestDirectoryConfig_QCDir(t *testing.T) {
	dirs := DirectoryConfig{
		Base:     "./userspace",
		Work:     "./userspace",
		Workflow: "./userspace/workflow",
		QC: QCConfig{
			Main:   "./userspace/workflow/QC",
			Before: "./userspace/workflow/fastqc_raw",
			After:  "./userspace/workflow/fastqc_clean",
		},
	}

	assert.Equal(t, "./userspace", dirs.Base)
	assert.Equal(t, "./userspace/workflow", dirs.Workflow)
	assert.Equal(t, "./userspace/workflow/QC", dirs.QC.Main)
	assert.Equal(t, "./userspace/workflow/fastqc_raw", dirs.QC.Before)
	assert.Equal(t, "./userspace/workflow/fastqc_clean", dirs.QC.After)
}

func TestBSMAPConfig_Directories(t *testing.T) {
	bsmap := BSMAPConfig{
		Main:     "./userspace/workflow/bsmap",
		Temp:     "./userspace/workflow/bsmap/tmp",
		Filtered: "./userspace/workflow/bsmap/Filtered_bams",
	}

	assert.Equal(t, "./userspace/workflow/bsmap", bsmap.Main)
	assert.Equal(t, "./userspace/workflow/bsmap/tmp", bsmap.Temp)
	assert.Equal(t, "./userspace/workflow/bsmap/Filtered_bams", bsmap.Filtered)
}

func TestClubCpGConfig_Directories(t *testing.T) {
	club := ClubCpGConfig{
		Main:     "./userspace/workflow/clubcpg",
		Coverage: "./userspace/workflow/clubcpg/clubcpg_coverage_before",
		Model:    "./userspace/workflow/clubcpg/clubcpg_model",
		Impute:   "./userspace/workflow/clubcpg/clubcpg_coverage_impute",
	}

	assert.Equal(t, "./userspace/workflow/clubcpg", club.Main)
	assert.Equal(t, "./userspace/workflow/clubcpg/clubcpg_coverage_before", club.Coverage)
	assert.Equal(t, "./userspace/workflow/clubcpg/clubcpg_model", club.Model)
	assert.Equal(t, "./userspace/workflow/clubcpg/clubcpg_coverage_impute", club.Impute)
}

func TestReferenceConfig_RNAseq(t *testing.T) {
	ref := ReferenceConfig{
		Genome: "hg19",
		Files: ReferenceFiles{
			Fasta:    []string{"inst/pdx/homo_sapiens/hg19.fasta"},
			Genome:   []string{"inst/pdx/homo_sapiens/"},
			CGI:      "inst/hg19/hg19_cpgIsland.bed",
			CpGSites: "inst/hg19/hg19_CpG_sites.gz",
		},
		Indices: ReferenceIndices{
			Genome: []string{"inst/pdx/homo_sapiens/"},
		},
		Annotations: AnnotationConfig{
			Names: []string{"hg19"},
		},
		RNAseq: RNAseqConfig{
			GTF:         "",
			Reference:   "",
			Chromosomes: []string{"chr1", "chr2", "chr3"},
		},
	}

	assert.Equal(t, "hg19", ref.Genome)
	assert.Equal(t, 1, len(ref.Files.Fasta))
	assert.Equal(t, "inst/pdx/homo_sapiens/hg19.fasta", ref.Files.Fasta[0])
	assert.Equal(t, 3, len(ref.RNAseq.Chromosomes))
}

func TestParallelConfig_Workers(t *testing.T) {
	parallel := ParallelConfig{
		Workers:      4,
		DwarfWorkers: 1,
	}

	assert.Equal(t, 4, parallel.Workers)
	assert.Equal(t, 1, parallel.DwarfWorkers)
}

func TestMetadataConfig_SampleIDs(t *testing.T) {
	meta := MetadataConfig{
		SampleIDs:   []string{"sample1", "sample2", "sample3"},
		UserEmail:   "user@example.com",
		PDXPipeline: "no",
		GroupLevels: 0,
	}

	assert.Equal(t, 3, len(meta.SampleIDs))
	assert.Equal(t, "user@example.com", meta.UserEmail)
	assert.Equal(t, "no", meta.PDXPipeline)
	assert.Equal(t, 0, meta.GroupLevels)
}

func TestSampleConfig_PairedReads(t *testing.T) {
	sample := SampleConfig{
		Name: "sample1",
		R1:   "sample1_R1.fastq.gz",
		R2:   "sample1_R2.fastq.gz",
	}

	assert.Equal(t, "sample1", sample.Name)
	assert.Equal(t, "sample1_R1.fastq.gz", sample.R1)
	assert.Equal(t, "sample1_R2.fastq.gz", sample.R2)
}

func TestValidateConfig_InvalidMode(t *testing.T) {
	cfg := LoadDefaults()
	cfg.Workflow.Mode = "INVALID_MODE"
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestValidateConfig_ValidModes(t *testing.T) {
	for _, mode := range []string{"RRBS", "WGBS", "BSSEQ", "RNASEQ"} {
		t.Run(mode, func(t *testing.T) {
			cfg := LoadDefaults()
			cfg.Workflow.Mode = mode
			err := ValidateConfig(cfg)
			if err != nil {
				t.Errorf("mode %s should be valid: %v", mode, err)
			}
		})
	}
}

func TestValidateConfig_MissingSuffix1(t *testing.T) {
	cfg := LoadDefaults()
	cfg.Input.Suffix1 = ""
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for missing suffix1")
	}
}

func TestValidateConfig_InvalidErrorRate(t *testing.T) {
	cfg := LoadDefaults()
	cfg.Workflow.Adapters.ErrorRate = 1.5
	err := ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for error_rate > 1")
	}

	cfg.Workflow.Adapters.ErrorRate = -0.1
	err = ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for error_rate < 0")
	}
}

func TestDetectPDXMode(t *testing.T) {
	cfg := LoadDefaults()
	cfg.Workflow.Species.Primary = "human"
	cfg.Workflow.Species.Secondary = ""
	if DetectPDXMode(cfg) {
		t.Error("not PDX when secondary is empty")
	}

	cfg.Workflow.Species.Secondary = "mouse"
	if !DetectPDXMode(cfg) {
		t.Error("should detect PDX with secondary set")
	}
}

func TestGetWorkflowName(t *testing.T) {
	tests := []struct {
		mode, expected string
		pdx            bool
	}{
		{"RNASEQ", "BeaverRNASEQPDX", true},
		{"RNASEQ", "BeaverRNA", false},
		{"RRBS", "BeaverPDX", true},
		{"WGBS", "BeaverPDX", true},
		{"BSSEQ", "BeaverPDX", true},
		{"RRBS", "BeaverBS", false},
	}
	for _, tc := range tests {
		result := GetWorkflowName(tc.mode, tc.pdx)
		if result != tc.expected {
			t.Errorf("GetWorkflowName(%s, %v) = %s, want %s", tc.mode, tc.pdx, result, tc.expected)
		}
	}
}

func TestGetStepCount(t *testing.T) {
	if GetStepCount("RNASEQ", false) != 2 {
		t.Error("RNASEQ non-PDX should have 2 steps")
	}
	if GetStepCount("RNASEQ", true) != 3 {
		t.Error("RNASEQ PDX should have 3 steps")
	}
	if GetStepCount("RRBS", false) != 3 {
		t.Error("RRBS should have 3 steps")
	}
}

func TestEngineConfig_Slurm(t *testing.T) {
	engine := EngineConfig{
		Type: "slurm",
		Slurm: SlurmConfig{
			Partition:  "compute",
			Cores:      8,
			Memory:     "32GB",
			JobName:    "otter_job",
			MaxRetries: 3,
		},
	}

	assert.Equal(t, "slurm", engine.Type)
	assert.Equal(t, "compute", engine.Slurm.Partition)
	assert.Equal(t, 8, engine.Slurm.Cores)
	assert.Equal(t, "32GB", engine.Slurm.Memory)
}
