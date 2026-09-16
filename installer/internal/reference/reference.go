// Package reference implements the ReferenceBuild flow: it writes a
// reference-build.yaml configuration and invokes Craftmake to download, build,
// and publish an immutable reference genome release.
package reference

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/installer/internal/config"
)

// Builder runs the ReferenceBuild workflow through Craftmake.
type Builder struct {
	DryRun bool
	// CraftmakeBinary is the resolved craftmake executable.
	CraftmakeBinary string
	// WorkflowCatalog is the directory containing the ReferenceBuild workflow.
	WorkflowCatalog string
	// ConfigPath is where the generated reference-build.yaml is written.
	ConfigPath string
	// Config is the resolved reference build configuration.
	Config config.ReferenceBuildConfig
}

// ResolveToolPath finds a tool inside otter-core or on PATH.
func ResolveToolPath(ctx context.Context, configured, executable string, envaPath, packageManager string, dryRun bool) (string, error) {
	if configured != "" {
		return configured, nil
	}
	if dryRun {
		return executable, nil
	}
	var runner []string
	if envaPath != "" {
		runner = []string{envaPath, "run", "otter-core", "--"}
	} else if packageManager != "" {
		runner = []string{packageManager, "run", "-n", "otter-core", "--"}
	}
	if len(runner) > 0 {
		commandArgs := append(append([]string{}, runner...), "bash", "-lc", "command -v "+executable)
		command := exec.CommandContext(ctx, commandArgs[0], commandArgs[1:]...)
		output, err := command.CombinedOutput()
		if err == nil {
			path := strings.TrimSpace(string(output))
			if path != "" {
				return path, nil
			}
		}
	}
	if path, err := exec.LookPath(executable); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("could not resolve required ReferenceBuild tool: %s", executable)
}

// WriteConfiguration writes the reference-build.yaml file.
func (builder *Builder) WriteConfiguration() error {
	config := builder.Config
	required := map[string]string{
		"OTTER_REFERENCE_BUILD_FASTA_URL":            config.FastaURL,
		"OTTER_REFERENCE_BUILD_GTF_URL":              config.GTFURL,
		"OTTER_REFERENCE_BUILD_REFERENCE_ID":         config.ReferenceID,
		"OTTER_REFERENCE_BUILD_RELEASE":              config.Release,
		"OTTER_REFERENCE_BUILD_ORGANISM":             config.Organism,
		"OTTER_REFERENCE_BUILD_ASSEMBLY":             config.Assembly,
		"OTTER_REFERENCE_BUILD_FASTA_CHECKSUM_VALUE": config.FastaChecksumValue,
		"OTTER_REFERENCE_BUILD_GTF_CHECKSUM_VALUE":   config.GTFChecksumValue,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("missing ReferenceBuild setting: %s", name)
		}
	}
	if config.Backend != "local" && config.Backend != "slurm" {
		return fmt.Errorf("OTTER_REFERENCE_BUILD_BACKEND must be local or slurm")
	}
	if config.Backend == "slurm" && strings.TrimSpace(config.Partition) == "" {
		return fmt.Errorf("a Slurm partition is required when ReferenceBuild backend is slurm")
	}
	if config.FastaFilename == "" {
		config.FastaFilename = baseName(config.FastaURL)
	}
	if config.GTFFilename == "" {
		config.GTFFilename = baseName(config.GTFURL)
	}
	if config.RunID == "" {
		config.RunID = fmt.Sprintf("reference-%s-%s", config.ReferenceID, config.Release)
	}

	if err := os.MkdirAll(filepath.Dir(builder.ConfigPath), 0o755); err != nil {
		return err
	}
	if builder.DryRun {
		fmt.Printf("  [DRY-RUN] would write ReferenceBuild configuration to %s\n", builder.ConfigPath)
		return nil
	}
	var content strings.Builder
	content.WriteString("reference_build:\n")
	writeField(&content, "run_id", config.RunID)
	writeField(&content, "backend", config.Backend)
	writeField(&content, "partition", config.Partition)
	writeField(&content, "reference_id", config.ReferenceID)
	writeField(&content, "release", config.Release)
	writeField(&content, "organism", config.Organism)
	writeField(&content, "assembly", config.Assembly)
	writeField(&content, "aliases", config.Aliases)
	writeField(&content, "cache_dir", config.CacheDir)
	writeField(&content, "work_dir", config.WorkDir)
	writeField(&content, "registry_root", config.RegistryRoot)
	writeField(&content, "evidence_dir", config.EvidenceDir)
	writeField(&content, "fasta_url", config.FastaURL)
	writeField(&content, "fasta_filename", config.FastaFilename)
	writeField(&content, "fasta_checksum_algorithm", config.FastaChecksumAlgorithm)
	writeField(&content, "fasta_checksum_value", config.FastaChecksumValue)
	writeField(&content, "gtf_url", config.GTFURL)
	writeField(&content, "gtf_filename", config.GTFFilename)
	writeField(&content, "gtf_checksum_algorithm", config.GTFChecksumAlgorithm)
	writeField(&content, "gtf_checksum_value", config.GTFChecksumValue)
	writeField(&content, "contigs", config.Contigs)
	writeField(&content, "star_sjdb_overhang", config.STARSJDBOverhang)
	writeField(&content, "index_build_threads", config.IndexBuildThreads)
	writeField(&content, "otter_binary", config.OtterBinary)
	writeField(&content, "samtools_binary", config.SamtoolsBinary)
	writeField(&content, "bismark_binary", config.BismarkBinary)
	writeField(&content, "bowtie2_binary", config.Bowtie2Binary)
	writeField(&content, "star_binary", config.STARBinary)
	return os.WriteFile(builder.ConfigPath, []byte(content.String()), 0o644)
}

// Run invokes craftmake plan and run for the ReferenceBuild workflow.
func (builder *Builder) Run(ctx context.Context) error {
	workflowPath := filepath.Join(builder.WorkflowCatalog, "ReferenceBuild", "build.yaml")
	common := []string{
		"--reference-build-config",
		"--config", builder.ConfigPath,
		"--workflow", workflowPath,
		"--phase", "build",
		"--catalog", builder.WorkflowCatalog,
	}
	if builder.DryRun {
		fmt.Printf("  [DRY-RUN] %s plan %s\n", builder.CraftmakeBinary, strings.Join(common, " "))
		fmt.Printf("  [DRY-RUN] %s run %s --gate\n", builder.CraftmakeBinary, strings.Join(common, " "))
		return nil
	}
	if err := runCommand(ctx, builder.CraftmakeBinary, append([]string{"plan"}, common...)...); err != nil {
		return err
	}
	return runCommand(ctx, builder.CraftmakeBinary, append(append([]string{"run"}, common...), "--gate")...)
}

func runCommand(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func writeField(builder *strings.Builder, key, value string) {
	builder.WriteString("  ")
	builder.WriteString(key)
	builder.WriteString(": ")
	builder.WriteString(yamlQuote(value))
	builder.WriteString("\n")
}

func yamlQuote(value string) string {
	value = strings.ReplaceAll(value, "'", "''")
	return "'" + value + "'"
}

func baseName(url string) string {
	trimmed := strings.TrimSuffix(url, "/")
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		return trimmed[index+1:]
	}
	return trimmed
}
