package assets

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/otterlab-bio/otter/internal/logger"
)

func init() {
	logger.Init(false)
}

func setupTestFS(t *testing.T) {
	t.Helper()
	EmbeddedAssets = fstest.MapFS{
		"inst/Rscripts/test.R": &fstest.MapFile{
			Data: []byte("# Test R script\nprint('hello')\n"),
		},
		"inst/snakefiles/BeaverBS_step1.snakemake": &fstest.MapFile{
			Data: []byte("rule step1:\n  shell: 'echo hello'\n"),
		},
		"inst/rules/01fastqcxAtfirst.smk": &fstest.MapFile{
			Data: []byte("rule fastqcxAtfirst:\n  shell: 'fastqcx {input}'\n"),
		},
		"inst/envs/otter-snakemake.yaml": &fstest.MapFile{
			Data: []byte("name: otter-snakemake\n"),
		},
		"inst/data/gene_mapping.csv": &fstest.MapFile{
			Data: []byte("ensembl_id,symbol\nENSG001,BRCA1\n"),
		},
	}
}

func TestCreateDirectoryStructure(t *testing.T) {
	tmpDir := t.TempDir()
	copier := NewAssetCopier(tmpDir, "rootless")

	err := copier.CreateDirectoryStructure()
	if err != nil {
		t.Fatalf("CreateDirectoryStructure failed: %v", err)
	}

	expectedDirs := []string{
		"config", "data", "envs", "inst", "R", "rules",
		"saveRDS", "temp", "userspace", "workflows", "www",
	}

	for _, dir := range expectedDirs {
		fullPath := filepath.Join(tmpDir, dir)
		if info, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", dir)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}
}

func TestCopyDir_FileContent(t *testing.T) {
	setupTestFS(t)
	tmpDir := t.TempDir()
	copier := NewAssetCopier(tmpDir, "rootless")

	// Copy Rscripts
	err := copier.copyDir("inst/Rscripts", "R")
	if err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify file was copied
	destPath := filepath.Join(tmpDir, "R", "test.R")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatalf("expected %s to exist after copy", destPath)
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}

	expected := "# Test R script\nprint('hello')\n"
	if string(content) != expected {
		t.Errorf("file content mismatch.\nExpected: %q\nGot: %q", expected, string(content))
	}
}

func TestCopyDir_CreatesParentDirs(t *testing.T) {
	setupTestFS(t)
	tmpDir := t.TempDir()
	copier := NewAssetCopier(tmpDir, "rootless")

	// Copy into a nested subdirectory that doesn't exist
	err := copier.copyDir("inst/rules", "rules")
	if err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify nested file exists
	destPath := filepath.Join(tmpDir, "rules", "01fastqcxAtfirst.smk")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatalf("expected %s to exist after copy", destPath)
	}
}

func TestCopyAll(t *testing.T) {
	setupTestFS(t)
	tmpDir := t.TempDir()
	copier := NewAssetCopier(tmpDir, "rootless")

	err := copier.CopyAll()
	if err != nil {
		t.Fatalf("CopyAll failed: %v", err)
	}

	// Verify key files were copied
	checks := []struct {
		path string
		desc string
	}{
		{filepath.Join(tmpDir, "R", "test.R"), "R script"},
		{filepath.Join(tmpDir, "BeaverBS_step1.snakemake"), "snakefile"},
		{filepath.Join(tmpDir, "rules", "01fastqcxAtfirst.smk"), "rule"},
		{filepath.Join(tmpDir, "envs", "otter-snakemake.yaml"), "env"},
		{filepath.Join(tmpDir, "data", "gene_mapping.csv"), "data"},
	}

	for _, check := range checks {
		if _, err := os.Stat(check.path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist (%s)", check.path, check.desc)
		}
	}
}

func TestListEmbeddedFiles(t *testing.T) {
	setupTestFS(t)

	files, err := ListEmbeddedFiles()
	if err != nil {
		t.Fatalf("ListEmbeddedFiles failed: %v", err)
	}

	if len(files) != 5 {
		t.Errorf("expected 5 embedded files, got %d: %v", len(files), files)
	}
}

func TestSetEmbeddedAssets(t *testing.T) {
	testFS := fstest.MapFS{
		"custom/path/file.txt": &fstest.MapFile{
			Data: []byte("custom content"),
		},
	}

	SetEmbeddedAssets(testFS)

	err := fs.WalkDir(EmbeddedAssets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}
}

func TestNewAssetCopier_Defaults(t *testing.T) {
	// Test with empty rulesType (should default to "rootless")
	copier := NewAssetCopier("/tmp/project", "")
	if copier.RulesType != "rootless" {
		t.Errorf("expected RulesType 'rootless', got '%s'", copier.RulesType)
	}

	// Test with explicit legacy type
	copier2 := NewAssetCopier("/tmp/project", "legacy")
	if copier2.RulesType != "legacy" {
		t.Errorf("expected RulesType 'legacy', got '%s'", copier2.RulesType)
	}
}

func TestPDXFilteringRulesDeclareValidatedBAMAndBAIOutputs(t *testing.T) {
	rulePath := filepath.Join("..", "..", "inst", "rules", "XenofilteR.smk")
	ruleContent, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("read xenofilx rule %s: %v", rulePath, err)
	}
	ruleText := string(ruleContent)
	for _, requiredSnippet := range []string{
		"graft_bams=expand(",
		"\"{sample}_\" + config",
		"filtered_bams=expand(",
		"filtered_bais=expand(",
		"--output-names",
		"--recalculate-nm",
		"bisulfite_flag=(\"\" if config[\"mode\"] == \"RNASEQ\" else \"--bisulfite\")",
		"graft_ref=config[\"reference\"][\"graft_fasta\"]",
		"host_ref=config[\"reference\"][\"host_fasta\"]",
		"--host-ref {params.host_ref:q} {params.bisulfite_flag}",
		"samtools quickcheck -v",
		"test -s \"$filtered_bai\"",
	} {
		if !strings.Contains(ruleText, requiredSnippet) {
			t.Fatalf("xenofilx rule %s is missing declared filtered BAM/BAI contract %q", rulePath, requiredSnippet)
		}
	}
	if strings.Contains(ruleText, "samtools index") {
		t.Fatalf("xenofilx rule %s must preserve the Xenofilx-produced BAI rather than regenerate it with samtools", rulePath)
	}
	for _, obsoleteMarker := range []string{"filtered_success.txt", "touch "} {
		if strings.Contains(ruleText, obsoleteMarker) {
			t.Fatalf("xenofilx rule %s still relies on marker-only completion %q", rulePath, obsoleteMarker)
		}
	}
}

func TestPDXPatchRulesDeclareFixedBAMOutputs(t *testing.T) {
	rulePaths := []string{
		filepath.Join("..", "..", "inst", "rules", "picard_pdx_patch.smk"),
		filepath.Join("..", "..", "inst", "rules_legacy", "picard_pdx_patch.smk"),
	}
	for _, rulePath := range rulePaths {
		ruleContent, err := os.ReadFile(rulePath)
		if err != nil {
			t.Fatalf("read Picard patch rule %s: %v", rulePath, err)
		}
		ruleText := string(ruleContent)
		for _, requiredSnippet := range []string{"fixed_bam=", "{sample}_fixed_{species}.bam", "test -s {output.fixed_bam:q}"} {
			if !strings.Contains(ruleText, requiredSnippet) {
				t.Fatalf("Picard patch rule %s is missing fixed BAM contract %q", rulePath, requiredSnippet)
			}
		}
		if strings.Contains(ruleText, "pdx_patch_success") || strings.Contains(ruleText, "touch ") {
			t.Fatalf("Picard patch rule %s still relies on marker-only completion", rulePath)
		}
	}
}

func TestPDXStep2CheckersTargetDeclaredFilterOutputs(t *testing.T) {
	checkerPaths := []string{
		filepath.Join("..", "..", "inst", "snakefiles", "BeaverPDX_step2_checker.snakemake"),
		filepath.Join("..", "..", "inst", "snakefiles", "BeaverRNASEQPDX_step2_checker.snakemake"),
		filepath.Join("..", "..", "inst", "snakefiles", "BeaverPDX.snakemake"),
		filepath.Join("..", "..", "inst", "snakefiles", "BeaverRNASEQPDX.snakemake"),
		filepath.Join("..", "..", "testdata", "e2e", "test_init", "BeaverPDX_step2_checker.snakemake"),
		filepath.Join("..", "..", "testdata", "e2e", "test_init", "BeaverRNASEQPDX_step2_checker.snakemake"),
		filepath.Join("..", "..", "testdata", "e2e", "test_init", "BeaverPDX.snakemake"),
		filepath.Join("..", "..", "testdata", "e2e", "test_init", "BeaverRNASEQPDX.snakemake"),
	}
	for _, checkerPath := range checkerPaths {
		checkerContent, err := os.ReadFile(checkerPath)
		if err != nil {
			t.Fatalf("read PDX step2 checker %s: %v", checkerPath, err)
		}
		checkerText := string(checkerContent)
		for _, requiredSnippet := range []string{
			"rules/XenofilteR.smk",
			"_fixed_",
			"_Filtered.bam.bai",
		} {
			if !strings.Contains(checkerText, requiredSnippet) {
				t.Fatalf("PDX step2 checker %s is missing producer contract %q", checkerPath, requiredSnippet)
			}
		}
		for _, obsoleteMarker := range []string{"pdx_patch_success", "filtered_success.txt", "step2_success.txt", "PDXseq_step2_checker.smk", "XenofilteR_RNA.smk"} {
			if strings.Contains(checkerText, obsoleteMarker) {
				t.Fatalf("PDX step2 checker %s still targets marker-only completion %q", checkerPath, obsoleteMarker)
			}
		}
	}
}

func TestRNASnakefilesTargetTypedSplicingOutcomes(t *testing.T) {
	snakefilePaths := []string{
		filepath.Join("..", "..", "inst", "snakefiles", "BeaverRNA.snakemake"),
		filepath.Join("..", "..", "inst", "snakefiles", "BeaverRNA_step2_checker.snakemake"),
		filepath.Join("..", "..", "inst", "snakefiles", "BeaverRNASEQPDX.snakemake"),
		filepath.Join("..", "..", "inst", "snakefiles", "BeaverRNASEQPDX_step3.snakemake"),
		filepath.Join("..", "..", "testdata", "e2e", "test_init", "BeaverRNA.snakemake"),
		filepath.Join("..", "..", "testdata", "e2e", "test_init", "BeaverRNA_step2.snakemake"),
		filepath.Join("..", "..", "testdata", "e2e", "test_init", "BeaverRNASEQPDX.snakemake"),
		filepath.Join("..", "..", "testdata", "e2e", "test_init", "BeaverRNASEQPDX_step3.snakemake"),
	}
	for _, snakefilePath := range snakefilePaths {
		snakefileContent, err := os.ReadFile(snakefilePath)
		if err != nil {
			t.Fatalf("read RNA snakefile %s: %v", snakefilePath, err)
		}
		snakefileText := string(snakefileContent)
		if !strings.Contains(snakefileText, "splicing-outcome.json") {
			t.Fatalf("RNA snakefile %s does not target the typed splicing outcome", snakefilePath)
		}
		for _, obsoleteMarker := range []string{"step2_success.txt", "RNASplicing_success.txt"} {
			if strings.Contains(snakefileText, obsoleteMarker) {
				t.Fatalf("RNA snakefile %s still targets marker-only completion %q", snakefilePath, obsoleteMarker)
			}
		}
	}
}

func TestRNAsplicingRulesPublishTypedOutcomes(t *testing.T) {
	rulePaths := []string{
		filepath.Join("..", "..", "inst", "rules", "rnaseq_splicing.smk"),
		filepath.Join("..", "..", "inst", "rules_legacy", "rnaseq_splicing.smk"),
	}
	for _, rulePath := range rulePaths {
		ruleContent, err := os.ReadFile(rulePath)
		if err != nil {
			t.Fatalf("read RNA splicing rule %s: %v", rulePath, err)
		}
		ruleText := string(ruleContent)
		for _, requiredSnippet := range []string{
			"splicing-outcome.json",
			"otter.rna-splicing-outcome/v1",
			"\"status\": \"produced\"",
			"\"status\": \"not_applicable\"",
		} {
			if !strings.Contains(ruleText, requiredSnippet) {
				t.Errorf("RNA splicing rule %s is missing typed outcome contract %q", rulePath, requiredSnippet)
			}
		}
		if strings.Contains(ruleText, "RNASplicing_success.txt") {
			t.Errorf("RNA splicing rule %s still exposes a marker-only output", rulePath)
		}
	}
}

func TestMethrixReferenceRuleStagesResolvedAnnotation(t *testing.T) {
	rulePath := filepath.Join("..", "..", "inst", "rules", "methrix_object.smk")
	ruleContent, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("read Methrix reference rule: %v", err)
	}
	ruleText := string(ruleContent)
	for _, requiredSnippet := range []string{
		"genome_annotation = lambda wildcards: config[\"reference\"][\"rnaseq\"][\"gtf\"]",
		"annotation=\"{input.genome_annotation}\"",
		"install -m 0644 \"$annotation\" \"$out_dir/$key.gtf\"",
		"Methrix annotation must be a regular resolved GTF",
		"extract_command=(methx extract-cp-gs)",
		"|| {{",
		"${{contig_arguments[@]}}",
		"cpgs: \\[\\]",
		"--contigs",
	} {
		if !strings.Contains(ruleText, requiredSnippet) {
			t.Fatalf("Methrix reference rule does not stage its resolved annotation input %q", requiredSnippet)
		}
	}
	for _, obsoleteReferenceDirectoryLookup := range []string{
		"$ref_dir/${{key}}.gtf",
		"gtf_candidates",
	} {
		if strings.Contains(ruleText, obsoleteReferenceDirectoryLookup) {
			t.Fatalf("Methrix reference rule retains implicit annotation discovery %q", obsoleteReferenceDirectoryLookup)
		}
	}
}

func TestBisulfiteStep3CheckersUseRustBismarkCompatibleInputsAndMemory(t *testing.T) {
	workflowPaths := []string{
		filepath.Join("..", "..", "craftmake", "workflows", "BeaverBS", "step3-check.yaml"),
		filepath.Join("..", "..", "craftmake", "workflows", "BeaverPDX", "step3-check.yaml"),
	}
	for _, workflowPath := range workflowPaths {
		workflowContent, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatalf("read bisulfite step3 checker workflow %s: %v", workflowPath, err)
		}
		workflowText := string(workflowContent)
		if strings.Contains(workflowText, "nucleotide_report") || strings.Contains(workflowText, "--nucleotide_report") {
			t.Fatalf("bisulfite step3 checker %s retains a Rust-incompatible nucleotide report input", workflowPath)
		}
		if !strings.Contains(workflowText, "memory: 34G") {
			t.Fatalf("bisulfite step3 checker %s does not allocate enough memory for hg19 CpG extraction", workflowPath)
		}
		if !strings.Contains(workflowText, "environment: otter-core") {
			t.Fatalf("bisulfite step3 checker %s does not select the Rust Bismark environment", workflowPath)
		}
	}
}

func TestBismarkReportRulesUseOnlyRustBismarkProducedReports(t *testing.T) {
	rulePaths := []string{
		filepath.Join("..", "..", "inst", "rules", "bismark_report2summary.smk"),
		filepath.Join("..", "..", "inst", "rules_legacy", "bismark_report2summary.smk"),
	}
	for _, rulePath := range rulePaths {
		ruleContent, err := os.ReadFile(rulePath)
		if err != nil {
			t.Fatalf("read Bismark report rule %s: %v", rulePath, err)
		}
		ruleText := string(ruleContent)
		for _, requiredSnippet := range []string{
			"--alignment_report {params.alignment_log}",
			"--splitting_report {params.split_log}",
			"--mbias_report {params.mbias_log}",
		} {
			if !strings.Contains(ruleText, requiredSnippet) {
				t.Fatalf("Bismark report rule %s is missing Rust Bismark-compatible input %q", rulePath, requiredSnippet)
			}
		}
		if strings.Contains(ruleText, "nucleotide_stats.txt") || strings.Contains(ruleText, "--nucleotide_report") {
			t.Fatalf("Bismark report rule %s requires a nucleotide stats artifact Rust Bismark does not produce", rulePath)
		}
	}
}

func TestBismarkMethylationRuleUsesBAMNamedPairbamTemporaryInput(t *testing.T) {
	rulePath := filepath.Join("..", "..", "inst", "rules", "build_methy_matrix_bismark.smk")
	ruleContent, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatalf("read Bismark methylation rule: %v", err)
	}
	ruleText := string(ruleContent)
	for _, requiredSnippet := range []string{
		"-o {params.bam_nsorted}.tmp.bam {input.bam_sorted}",
		"pairbam {params.bam_nsorted}.tmp.bam {params.bam_nsorted}",
		"rm -f {params.bam_nsorted}.tmp.bam",
	} {
		if !strings.Contains(ruleText, requiredSnippet) {
			t.Fatalf("Bismark methylation rule is missing pairbam-compatible temporary BAM path %q", requiredSnippet)
		}
	}
	if strings.Contains(ruleText, "{params.bam_nsorted}.bam.tmp") || strings.Contains(ruleText, "{params.bam_nsorted}.tmp ") {
		t.Fatal("Bismark methylation rule retains a non-BAM pairbam temporary input")
	}
}

func TestWorkflowCommandsQuotePathArguments(t *testing.T) {
	testCases := []struct {
		name              string
		rulePath          string
		requiredSnippets  []string
		forbiddenSnippets []string
	}{
		{
			name:     "RNA splicing",
			rulePath: filepath.Join("..", "..", "inst", "rules", "rnaseq_splicing.smk"),
			requiredSnippets: []string{
				"--root {params.run_dir:q}",
				"--pdata {input.pdata:q}",
				"--seqlengthQC {params.seqlengthQC:q}",
				"--gtf {input.gtf:q}",
				"splicing-outcome.json",
				"otter.rna-splicing-outcome/v1",
			},
			forbiddenSnippets: []string{
				"--root {params.run_dir} ",
				"--pdata {params.pdata}",
				"--gtf {params.gtf}",
			},
		},
		{
			name:     "bisulfite QC summary",
			rulePath: filepath.Join("..", "..", "inst", "rules", "bs_qc_summary.smk"),
			requiredSnippets: []string{
				"--config {input.config_file:q}",
				"--output {output.summary:q}",
			},
			forbiddenSnippets: []string{"--config {params.self_config}"},
		},
		{
			name:     "RNA QC summary",
			rulePath: filepath.Join("..", "..", "inst", "rules", "rna_qc_summary.smk"),
			requiredSnippets: []string{
				"--config {input.config_file:q}",
				"--output {output.summary:q}",
			},
			forbiddenSnippets: []string{"--config {params.self_config}"},
		},
		{
			name:     "raw FastQC",
			rulePath: filepath.Join("..", "..", "inst", "rules", "01fastqcxAtfirst.smk"),
			requiredSnippets: []string{
				"rule fastqcxAtfirst:",
				"fastqcx -q {input.R1:q} -s {params.R1_dir:q} --no-html",
				"fastqcx -q {input.R2:q} -s {params.R2_dir:q} --no-html",
				"{sample}_R1_fastqcx\", \"fastqc_data.txt",
				"{sample}_R2_fastqcx\", \"fastqc_data.txt",
			},
			forbiddenSnippets: []string{
				"fastqcx -q {input.R1} ",
				"fastqcx -q {input.R2} ",
			},
		},
		{
			name:     "clean FastQC",
			rulePath: filepath.Join("..", "..", "inst", "rules", "03-0-fastqcxAtclean.smk"),
			requiredSnippets: []string{
				"rule fastqcxAtclean:",
				"fastqcx -q {input.R1:q} -s {params.R1_dir:q} --no-html",
				"fastqcx -q {input.R2:q} -s {params.R2_dir:q} --no-html",
				"{sample}_val_1_fastqcx\", \"fastqc_data.txt",
				"{sample}_val_2_fastqcx\", \"fastqc_data.txt",
			},
			forbiddenSnippets: []string{
				"fastqcx -q {input.R1} ",
				"fastqcx -q {input.R2} ",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ruleContent, err := os.ReadFile(testCase.rulePath)
			if err != nil {
				t.Fatalf("read workflow rule %s: %v", testCase.rulePath, err)
			}
			ruleText := string(ruleContent)
			for _, requiredSnippet := range testCase.requiredSnippets {
				if !strings.Contains(ruleText, requiredSnippet) {
					t.Errorf("workflow rule %s is missing safely quoted command fragment %q", testCase.rulePath, requiredSnippet)
				}
			}
			for _, forbiddenSnippet := range testCase.forbiddenSnippets {
				if strings.Contains(ruleText, forbiddenSnippet) {
					t.Errorf("workflow rule %s still contains unsafe command fragment %q", testCase.rulePath, forbiddenSnippet)
				}
			}
		})
	}
}

func TestQCWorkflowDeclaresQCTBConsumedArtifacts(t *testing.T) {
	testCases := []struct {
		name             string
		rulePath         string
		requiredSnippets []string
	}{
		{
			name:     "bisulfite summary inputs",
			rulePath: filepath.Join("..", "..", "inst", "rules", "bs_qc_summary.smk"),
			requiredSnippets: []string{
				"{sample}_val_1_bismark_bt2_PE_report.txt",
				"genome_results.txt",
				"CpG_coverage.xlsx",
				"CpG_annotation_report.xlsx",
			},
		},
		{
			name:     "methrix producer outputs",
			rulePath: filepath.Join("..", "..", "inst", "rules", "methrix_object.smk"),
			requiredSnippets: []string{
				"assays.h5",
				"methrix_data.h5",
				"CpG_coverage.xlsx",
				"CpG_annotation_report.xlsx",
				"CpG_annotation_details.tsv.gz",
			},
		},
		{
			name:             "RNA summary inputs",
			rulePath:         filepath.Join("..", "..", "inst", "rules", "rna_qc_summary.smk"),
			requiredSnippets: []string{"{sample}Log.final.out"},
		},
		{
			name:             "Bismark producer outputs",
			rulePath:         filepath.Join("..", "..", "inst", "rules", "04bsmap2sort_bismark.smk"),
			requiredSnippets: []string{"alignment_report=", "{sample}_val_1_bismark_bt2_PE_report.txt"},
		},
		{
			name:             "Qualimap producer outputs",
			rulePath:         filepath.Join("..", "..", "inst", "rules", "05-3-qualimap.smk"),
			requiredSnippets: []string{"genome_results=", "genome_results.txt"},
		},
		{
			name:             "STAR producer outputs",
			rulePath:         filepath.Join("..", "..", "inst", "rules", "rnaseq_mapping.smk"),
			requiredSnippets: []string{"star_log=", "{sample}Log.final.out"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ruleContent, err := os.ReadFile(testCase.rulePath)
			if err != nil {
				t.Fatalf("read workflow rule %s: %v", testCase.rulePath, err)
			}
			for _, snippet := range testCase.requiredSnippets {
				if !strings.Contains(string(ruleContent), snippet) {
					t.Errorf("workflow rule %s does not declare qctb artifact %q", testCase.rulePath, snippet)
				}
			}
		})
	}
}

func TestCopyAll_EmptyEmbeddedAssets(t *testing.T) {
	// Reset to empty
	EmbeddedAssets = fstest.MapFS{}
	tmpDir := t.TempDir()
	copier := NewAssetCopier(tmpDir, "rootless")

	// CopyAll should fail because embedded assets are empty
	err := copier.CopyAll()
	if err == nil {
		t.Error("expected error with empty embedded assets")
	}
}
