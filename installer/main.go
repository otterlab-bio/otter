// Command otter-install is a statically compiled installer for the otter
// bioinformatics toolchain. It downloads pre-built static binaries from GitHub
// Releases, creates conda environments, verifies pinned tool versions, and can
// run the Craftmake ReferenceBuild workflow.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/otterlab-bio/otter/installer/internal/assets"
	"github.com/otterlab-bio/otter/installer/internal/config"
	"github.com/otterlab-bio/otter/installer/internal/download"
	"github.com/otterlab-bio/otter/installer/internal/envs"
	"github.com/otterlab-bio/otter/installer/internal/reference"
	"github.com/otterlab-bio/otter/installer/internal/verify"
)

var (
	version = "1.2.0"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	options, err := config.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, options); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, options *config.Options) error {
	client := download.NewClient(options.GitHubToken, options.GitHubProxy, options.DryRun)

	fmt.Printf("otter-installer %s+%s (%s)\n", version, commit, date)
	if options.DryRun {
		fmt.Println("DRY-RUN MODE - no changes will be made")
	}
	fmt.Printf("Install directory: %s\n", options.InstallDir)
	fmt.Printf("Primary releases repo: %s\n", options.ReleasesRepo)
	if options.FallbackReleasesRepo != "" && options.FallbackReleasesRepo != options.ReleasesRepo {
		fmt.Printf("Fallback releases repo: %s\n", options.FallbackReleasesRepo)
	}
	fmt.Printf("GitHub proxy prefix: %s\n", orNone(options.GitHubProxy))
	fmt.Println()

	// Step 1: Resolve version and download binaries.
	tag, err := resolveTag(ctx, client, options)
	if err != nil {
		return err
	}
	fmt.Printf("Version: %s\n", tag)
	fmt.Printf("Selected release repo: %s\n", options.ReleasesRepo)
	fmt.Println()

	if err := os.MkdirAll(options.InstallDir, 0o755); err != nil {
		return err
	}
	for _, tool := range config.Tools {
		asset := download.AssetName(tool.AssetStem, tool.Linkage)
		dest := filepath.Join(options.InstallDir, tool.Name)
		url := download.ReleaseAssetURL(options.ReleasesRepo, tag, asset)
		if options.DryRun {
			fmt.Printf("  [DRY-RUN] would download %s -> %s\n", url, dest)
			continue
		}
		fmt.Printf("Downloading %s ...\n", tool.Name)
		if err := client.Download(ctx, url, dest); err != nil {
			if _, statErr := os.Stat(dest); statErr == nil {
				fmt.Printf("  ⚠ %s download failed (%v); keeping existing binary\n", tool.Name, err)
			} else {
				fmt.Printf("  ⚠ %s download failed (%v); asset may not exist for this release\n", tool.Name, err)
			}
			continue
		}
		if err := os.Chmod(dest, 0o755); err != nil {
			return err
		}
		fmt.Printf("  ✓ %s installed\n", tool.Name)
	}
	fmt.Println()

	// Step 2: Deploy Craftmake workflow catalog and configurations.
	fmt.Println("Step 2: Deploying Craftmake workflow catalog and configurations")
	if err := assets.ExtractAll(options.InstallDir, options.DryRun); err != nil {
		return fmt.Errorf("deploy workflows and configs: %w", err)
	}
	if !options.DryRun {
		shareDir := filepath.Join(filepath.Dir(options.InstallDir), "share", "craftmake")
		fmt.Printf("  ✓ workflows deployed to %s\n", filepath.Join(shareDir, "workflows"))
		fmt.Printf("  ✓ configs deployed to %s\n", filepath.Join(shareDir, "configs"))
	}

	// The environment manager reads its YAML from the install directory, so the
	// environment definitions have to be fetched into it. They are published as
	// release assets alongside the binaries, which is why nothing staged them
	// locally.
	if !options.SkipEnvs {
		fmt.Println("  Fetching environment definitions ...")
		for _, envFile := range config.EnvFiles {
			url := download.ReleaseAssetURL(options.ReleasesRepo, tag, envFile)
			destination := filepath.Join(options.InstallDir, envFile)
			if options.DryRun {
				fmt.Printf("  [DRY-RUN] would download %s -> %s\n", url, destination)
				continue
			}
			if err := client.Download(ctx, url, destination); err != nil {
				return fmt.Errorf("download environment definition %s: %w", envFile, err)
			}
			fmt.Printf("  ✓ %s\n", envFile)
		}
	}
	fmt.Println()

	// Step 3: Verify installed tools.
	fmt.Println("Step 3: Verifying installed tools")
	for _, tool := range config.Tools {
		output, err := verify.Binary(ctx, options.InstallDir, tool.Name, options.DryRun)
		if err != nil {
			fmt.Printf("  ⚠ %s: %v\n", tool.Name, err)
			continue
		}
		if options.DryRun {
			fmt.Printf("  [DRY-RUN] would verify: %s --version\n", tool.Name)
			continue
		}
		fmt.Printf("  ✓ %s  %s\n", tool.Name, output)
	}
	fmt.Println()

	// Step 4: Create conda environments.
	envaPath := envs.ResolveEnva(options.InstallDir)
	packageManager := envs.ResolvePackageManager()
	manager := &envs.Manager{
		DryRun:         options.DryRun,
		EnvaPath:       envaPath,
		PackageManager: packageManager,
		EnvsDir:        options.InstallDir,
		Choice:         "core",
	}
	if !options.SkipEnvs {
		fmt.Println("Step 4: Creating conda environments")
		if err := manager.Create(ctx); err != nil {
			return err
		}
		fmt.Println()
	}

	// Step 5: Verify pinned methylation tools.
	if !options.SkipEnvs {
		fmt.Println("Step 5: Verifying pinned methylation tools")
		if err := manager.VerifyPinnedTools(ctx); err != nil {
			return err
		}
		fmt.Println()
	}

	// Step 6: ReferenceBuild.
	if options.ReferenceBuild {
		if err := runReferenceBuild(ctx, options, envaPath, packageManager); err != nil {
			return err
		}
	}

	// Step 7: Fetch published reference index builds.
	if options.ReferenceFetch {
		if err := runReferenceFetch(ctx, options, client); err != nil {
			return err
		}
	}

	printCompletionSummary(options)
	return nil
}

// runReferenceFetch downloads index builds that were published once and extracts them into the
// registry, so a machine does not have to rebuild STAR, bowtie2 and Bismark indexes locally.
func runReferenceFetch(ctx context.Context, options *config.Options, client *download.Client) error {
	fetchConfig := options.ReferenceFetchConfig
	selections, err := reference.ParseSelections(fetchConfig.Releases)
	if err != nil {
		return err
	}
	fmt.Printf("Step 7: Fetching reference release from %s\n", fetchConfig.Repo)
	fetcher := &reference.Fetcher{
		BaseURL:      fetchConfig.BaseURL,
		Repo:         fetchConfig.Repo,
		Revision:     fetchConfig.Revision,
		RegistryRoot: fetchConfig.RegistryRoot,
		AssetNames:   fetchConfig.Assets,
		Selections:   selections,
		Client:       client,
		DryRun:       options.DryRun,
	}
	if err := fetcher.Fetch(ctx); err != nil {
		return err
	}
	fmt.Printf("  registry root: %s\n\n", fetchConfig.RegistryRoot)
	return nil
}

func resolveTag(ctx context.Context, client *download.Client, options *config.Options) (string, error) {
	if options.Version != "" && options.Version != "latest" {
		return options.Version, nil
	}
	tag, err := client.LatestTag(ctx, options.ReleasesRepo)
	if err == nil && tag != "" {
		return tag, nil
	}
	if options.FallbackReleasesRepo != "" {
		if fallbackTag, fallbackErr := client.LatestTag(ctx, options.FallbackReleasesRepo); fallbackErr == nil && fallbackTag != "" {
			return fallbackTag, nil
		}
	}
	return "", fmt.Errorf("could not determine latest release version from %s or %s", options.ReleasesRepo, options.FallbackReleasesRepo)
}

func runReferenceBuild(ctx context.Context, options *config.Options, envaPath, packageManager string) error {
	fmt.Println("ReferenceBuild: downloading and publishing reference genome")
	craftmakeBinary, err := resolveCraftmake(options)
	if err != nil {
		return err
	}
	workflowCatalog, err := resolveWorkflowCatalog(options, craftmakeBinary)
	if err != nil {
		return err
	}
	configPath := filepath.Join(options.Home, ".otter", "reference-build", "reference-build.yaml")

	referenceConfig := options.ReferenceBuildConfig
	referenceConfig.SamtoolsBinary, err = reference.ResolveToolPath(ctx, referenceConfig.SamtoolsBinary, "samtools", envaPath, packageManager, options.DryRun)
	if err != nil {
		return err
	}
	referenceConfig.BismarkBinary, err = reference.ResolveToolPath(ctx, referenceConfig.BismarkBinary, "bismark_genome_preparation", envaPath, packageManager, options.DryRun)
	if err != nil {
		return err
	}
	referenceConfig.Bowtie2Binary, err = reference.ResolveToolPath(ctx, referenceConfig.Bowtie2Binary, "bowtie2-build", envaPath, packageManager, options.DryRun)
	if err != nil {
		return err
	}
	referenceConfig.STARBinary, err = reference.ResolveToolPath(ctx, referenceConfig.STARBinary, "STAR", envaPath, packageManager, options.DryRun)
	if err != nil {
		return err
	}

	builder := &reference.Builder{
		DryRun:          options.DryRun,
		CraftmakeBinary: craftmakeBinary,
		WorkflowCatalog: workflowCatalog,
		ConfigPath:      configPath,
		Config:          referenceConfig,
	}
	if err := builder.WriteConfiguration(); err != nil {
		return err
	}
	if err := builder.Run(ctx); err != nil {
		return err
	}
	fmt.Println("  ✓ ReferenceBuild published the immutable registry release")
	return nil
}

func resolveCraftmake(options *config.Options) (string, error) {
	if options.DryRun {
		return filepath.Join(options.InstallDir, "craftmake"), nil
	}
	candidates := []string{
		filepath.Join(options.InstallDir, "craftmake"),
	}
	if path, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(path), "craftmake"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if path, err := execLookPath("craftmake"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("craftmake binary was not found; install it or place it in %s", options.InstallDir)
}

func resolveWorkflowCatalog(options *config.Options, craftmakeBinary string) (string, error) {
	if options.DryRun {
		return filepath.Join(options.InstallDir, "craftmake-workflows"), nil
	}
	candidates := []string{
		filepath.Join(filepath.Dir(craftmakeBinary), "..", "share", "craftmake", "workflows"),
		filepath.Join(filepath.Dir(craftmakeBinary), "workflows"),
	}
	for _, candidate := range candidates {
		workflowPath := filepath.Join(candidate, "ReferenceBuild", "build.yaml")
		if info, err := os.Stat(workflowPath); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Craftmake ReferenceBuild workflow was not found")
}

func execLookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func printCompletionSummary(options *config.Options) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Installation complete!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Quick start (canonical v1 project):")
	fmt.Println()
	fmt.Println("    otter init my_project")
	fmt.Println("    otter build --project-root my_project --fastq data/fastq --mode RRBS \\")
	fmt.Println("      --reference-root <registry> --reference-primary <id@release> --backend local")
	fmt.Println()
	fmt.Println("    build is the shortcut over init, create, config validate, and config")
	fmt.Println("    resolve. Run those individually when you need to inspect each step:")
	fmt.Println()
	fmt.Println("    otter create --output my_project --fastq data/fastq --mode RRBS \\")
	fmt.Println("      --reference-root <registry> --reference-primary <id@release>")
	fmt.Println("    otter config resolve --project my_project/project.yaml --backend local")
	fmt.Println()
	fmt.Println("    Then execute the resolved snapshot:")
	fmt.Println()
	fmt.Println("    otter run --config my_project/runs/<run-id>/run.yaml \\")
	fmt.Println("      --executor craftmake --phase step1 --backend local")
	fmt.Println()
	fmt.Println("Pre-existing legacy projects (config/otter.yaml) are not accepted by any")
	fmt.Println("executor directly. Migrate and resolve them before running:")
	fmt.Println()
	fmt.Println("    otter init migrated")
	fmt.Println("    otter config migrate --input old_project/userspace/demo_rrbs/config/otter.yaml \\")
	fmt.Println("      --output migrated/project.yaml \\")
	fmt.Println("      --reference-root <registry> --reference-primary <id@release>")
	fmt.Println("    otter config resolve --project migrated/project.yaml --backend local")
	fmt.Println("    otter run --config migrated/runs/<run-id>/run.yaml \\")
	fmt.Println("      --executor snakemake --dry-run --foreground")
	fmt.Println()
	fmt.Println("Reference genomes are managed in a shared immutable registry.")
	fmt.Printf("Default registry root: %s\n", filepath.Join(options.Home, ".otter", "references"))
	fmt.Println("Build or select a release there before creating a canonical project.")
}

func orNone(value string) string {
	if value == "" {
		return "<none>"
	}
	return value
}
