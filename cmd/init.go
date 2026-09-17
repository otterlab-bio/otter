package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/otterlab-bio/otter/internal/assets"
	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/otterlab-bio/otter/internal/projectlayout"
	runstate "github.com/otterlab-bio/otter/internal/run"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize a new otter project",
	Long: `Initialize a new otter project.

This writes a canonical v1 project: packaged workflow assets pinned into
workflows/, rules/, environments/, and schemas/, plus a project.lock.yaml
that records their digests. Add project.yaml, samples.tsv, and
references.lock.yaml with "otter create", then resolve a run with
"otter config resolve".

A directory carrying the legacy compatibility marker
(.otter/assets.manifest.json) is refused: convert such a project with
"otter config migrate" instead.

Canonical project directories:
  workflows/     # Packaged workflow assets (pinned)
  rules/         # Snakemake compatibility rules (pinned)
  environments/  # Packaged environment declarations (pinned)
  schemas/       # Packaged JSON schemas (pinned)
  runs/          # Immutable run snapshots, created by "otter config resolve"

Example:
  otter init my_project`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	projectName := "otter-project"
	if len(args) > 0 {
		projectName = args[0]
	}
	projectDir := projectName

	if _, err := os.Stat(projectDir); err == nil {
		logger.Warnf("Directory %s already exists", projectDir)
	}

	return runInitCanonicalTrack(projectName, projectDir)
}

// runInitCanonicalTrack writes a canonical v1 project: pinned assets plus a
// project.lock.yaml that later commands use to prove the project track.
func runInitCanonicalTrack(projectName, projectDir string) error {
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return fmt.Errorf("failed to create project directory %s: %w", projectDir, err)
	}
	if existingTrack, err := projectlayout.Detect(projectDir); err != nil {
		return err
	} else if existingTrack == projectlayout.TrackLegacy {
		return fmt.Errorf(
			"%s is already a legacy compatibility project; migrate it with `otter config migrate`, or initialise a new directory",
			projectDir,
		)
	}

	copier := assets.NewAssetCopier(projectDir)

	logger.Info("Creating canonical project directory structure...")
	if err := copier.CreateV1DirectoryStructure(); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	logger.Info("Pinning packaged workflow assets...")
	copiedSets, err := copier.CopyV1ProjectAssets()
	if err != nil {
		return fmt.Errorf("failed to pin workflow assets: %w", err)
	}

	assetSets := make([]projectlayout.AssetSet, 0, len(copiedSets))
	for _, copiedSet := range copiedSets {
		digest, err := runstate.DigestPaths([]string{copiedSet.DestDir})
		if err != nil {
			return fmt.Errorf("failed to digest pinned assets in %s: %w", copiedSet.DestDir, err)
		}
		assetSets = append(assetSets, projectlayout.AssetSet{
			Role:   copiedSet.Role,
			Source: copiedSet.Source,
			Digest: digest,
			Files:  copiedSet.Files,
		})
	}

	lockPath, err := projectlayout.WriteProjectLock(projectDir, projectlayout.ProjectLock{
		ProjectID: projectName,
		Assets:    assetSets,
	})
	if err != nil {
		return err
	}
	logger.Infof("Project lock written: %s", lockPath)

	readmePath := filepath.Join(projectDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(canonicalReadme(projectName)), 0o644); err != nil {
		logger.Warnf("Failed to create README: %v", err)
	}

	checkEnvSupport()

	logger.Info("===========================================")
	logger.Infof("Canonical project '%s' initialized successfully!", projectName)
	logger.Infof("Location: %s", projectDir)
	logger.Info("===========================================")
	logger.Info("Next steps:")
	logger.Info("1. Place paired FASTQ files in a data directory")
	logger.Info("2. Build or select a reference registry release (see docs/reference-registry.md)")
	logger.Infof("3. Create the project: otter create --fastq <fastq-dir> --mode RRBS --reference-root <registry> --reference-primary <id@release>")
	logger.Info("4. Resolve a run:     otter config resolve --project project.yaml --backend local")
	logger.Info("5. Execute:           otter run --config runs/<run-id>/run.yaml --executor craftmake --phase step1")

	return nil
}

func canonicalReadme(projectName string) string {
	return fmt.Sprintf(`# %s

This is a canonical otter v1 project.

## Directory structure

- workflows/     - Packaged workflow assets, pinned and digested
- rules/         - Snakemake compatibility rules, pinned and digested
- environments/  - Packaged environment declarations, pinned and digested
- schemas/       - Packaged JSON schemas, pinned and digested
- runs/          - One immutable run.yaml snapshot per resolved run
- project.lock.yaml - Track marker and pinned asset digests

## Usage

1. Place paired FASTQ files in a data directory.
2. Build or select a reference registry release (see docs/reference-registry.md).
3. Create the project intent:

   otter create --fastq <fastq-dir> --mode RRBS \
     --reference-root <registry-root> --reference-primary <id@release>

4. Validate and resolve an immutable run:

   otter config validate --config project.yaml --schema v1
   otter config resolve --project project.yaml --backend local

5. Execute the resolved snapshot:

   otter run --config runs/<run-id>/run.yaml \
     --executor craftmake --phase step1 --backend local

Reference genomes are never copied into the project. The project locks logical
reference ids and release digests; the registry supplies the bytes.
`, projectName)
}

// checkEnvSupport checks if enva is available and provides installation guidance
func checkEnvSupport() {
	logger.Info("")
	logger.Info("Checking package manager support...")

	if _, err := exec.LookPath("enva"); err != nil {
		// enva not found
		logger.Warn("────────────────────────────────────────────────────────")
		logger.Warn("enva not found in PATH")
		logger.Warn("")
		logger.Warn("For the best environment workflow, install enva:")
		logger.Warn("  enva is rattler-first and can interoperate with existing conda/mamba/micromamba environments")
		logger.Warn("")
		logger.Warn("Installation:")
		logger.Warn("  wget https://github.com/otterlab-bio/enva/releases/latest/download/enva-linux-x86_64")
		logger.Warn("  chmod +x enva-linux-x86_64")
		logger.Warn("  sudo mv enva-linux-x86_64 /usr/local/bin/enva")
		logger.Warn("")
		logger.Warn("Or build from source:")
		logger.Warn("  git clone https://github.com/otterlab-bio/enva")
		logger.Warn("  cd enva && cargo build --release")
		logger.Warn("  cp target/release/enva /usr/local/bin/enva")
		logger.Warn("")
		logger.Warn("Falling back to conda run (slower)")
		logger.Warn("────────────────────────────────────────────────────────")
	} else {
		// enva found
		logger.Info("✓ enva detected - rattler-first environment management is available")
		logger.Info("")
	}
}
