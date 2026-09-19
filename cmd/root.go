package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	userLevel bool
	verbose   bool

	// buildVersion is the version of the binary
	buildVersion = "1.2.0"
	// buildCommit is the git commit hash
	buildCommit = "unknown"
	// buildDate is the build date
	buildDate = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "otter",
	Short: "Bioinformatics workflow CLI (Orchestrated Transcriptomic, Tumor-xenograft, and Epigenomic Reporting)",
	Long: `OTTER: Orchestrated Transcriptomic, Tumor-xenograft (PDX/CDX), and Epigenomic Reporting Workflow.
A reproducible bioinformatics workflow CLI for RRBS, WGBS, RNA-seq, and PDX analysis.
It integrates with Snakemake and Craftmake, supporting SLURM and local execution environments.`,
	Version: fmt.Sprintf("%s+%s (%s)", buildVersion, buildCommit, buildDate),
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// ExecuteContext runs the root command with a context for cancellation support.
func ExecuteContext(ctx context.Context) error {
	rootCmd.SetContext(ctx)
	return rootCmd.Execute()
}

// ExitCode returns a classified executor status when the command error provides one.
func ExitCode(commandError error) int {
	if commandError == nil {
		return 0
	}
	var classifiedError interface{ ExitCode() int }
	if errors.As(commandError, &classifiedError) {
		if exitCode := classifiedError.ExitCode(); exitCode > 0 {
			return exitCode
		}
	}
	return 1
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is otter.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&userLevel, "user-level", false, "run at user level (for PDX)")
}

// initConfig initializes the logger based on verbose flag.
func initConfig() {
	// Initialize logger
	logger.Init(verbose)
}
