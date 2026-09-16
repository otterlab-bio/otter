package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	assetspkg "github.com/otterlab-bio/otter/internal/assets"
	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/spf13/cobra"
)

var (
	assetsProjectDir string
)

var assetsCmd = &cobra.Command{
	Use:   "assets",
	Short: "Manage workflow assets manifest",
}

var assetsStampCmd = &cobra.Command{
	Use:   "stamp",
	Short: "Generate workflow assets manifest",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir := filepath.Clean(assetsProjectDir)
		m, err := assetspkg.StampWorkflowAssets(projectDir, buildVersion, buildCommit, buildDate)
		if err != nil {
			return err
		}
		deprecated := 0
		for _, e := range m.Entries {
			if e.Deprecated {
				deprecated++
			}
		}
		if deprecated > 0 {
			logger.Warnf("Deprecated assets detected: %d files under R/ (still verified for integrity)", deprecated)
		}
		logger.Infof("Assets manifest written: %s (%d files)", assetspkg.ManifestPath(projectDir), len(m.Entries))
		return nil
	},
}

var assetsVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify workflow assets against the manifest",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir := filepath.Clean(assetsProjectDir)
		m, err := assetspkg.LoadManifest(projectDir)
		if err != nil {
			return err
		}

		strict, _ := cmd.Flags().GetBool("strict")
		diff, err := assetspkg.VerifyWorkflowAssets(projectDir, m)
		if err != nil {
			return err
		}
		if diff.IsClean() {
			logger.Infof("Assets verified against manifest: %s", assetspkg.ManifestPath(projectDir))
			return nil
		}

		msg := fmt.Sprintf("Assets differ from manifest (missing=%d extra=%d modified=%d)", len(diff.Missing), len(diff.Extra), len(diff.Modified))
		if strict {
			return fmt.Errorf("%s", msg)
		}
		logger.Warn(msg)
		return nil
	},
}

var assetsPrintCmd = &cobra.Command{
	Use:   "print",
	Short: "Print the current assets manifest",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir := filepath.Clean(assetsProjectDir)
		m, err := assetspkg.LoadManifest(projectDir)
		if err != nil {
			return err
		}
		b, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(append(b, '\n'))
		return err
	},
}

func init() {
	rootCmd.AddCommand(assetsCmd)

	assetsCmd.PersistentFlags().StringVar(&assetsProjectDir, "project", ".", "Project directory")

	assetsCmd.AddCommand(assetsStampCmd)
	assetsCmd.AddCommand(assetsVerifyCmd)
	assetsCmd.AddCommand(assetsPrintCmd)

	assetsVerifyCmd.Flags().Bool("strict", false, "Fail if assets differ from the manifest")
}
