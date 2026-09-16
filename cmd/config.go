package cmd

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/otterlab-bio/otter/internal/config"
	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/spf13/cobra"
)

var (
	validateConfigFile   string
	validateConfigSchema string
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `Configuration management commands for otter.

Note: Use 'otter create' to generate complete project configurations
with sample information, adapters, and directory structure.

Examples:
  otter config validate --config my_config.yaml
  otter create --fastq /data --mode RRBS  # Generates full config`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a configuration file",
	Long: `Validate a configuration file for correctness.

This command checks if a configuration file is properly formatted
and contains all required fields for workflow execution.

Examples:
  otter config validate --config my_config.yaml`,
	RunE: runConfigValidate,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(validateCmd)

	// Flags for validate command
	validateCmd.Flags().StringVarP(&validateConfigFile, "config", "c", "otter.yaml", "Configuration file to validate")
	validateCmd.Flags().StringVar(&validateConfigSchema, "schema", "auto", "Schema to validate: auto, legacy, or v1")
	configCmd.AddCommand(newConfigResolveCommand())
	configCmd.AddCommand(newConfigMigrateCommand())
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	schema := strings.ToLower(strings.TrimSpace(validateConfigSchema))
	if schema == "auto" {
		content, err := os.ReadFile(validateConfigFile)
		if err != nil {
			return fmt.Errorf("read configuration: %w", err)
		}
		header := struct {
			SchemaVersion string `yaml:"schema_version"`
		}{}
		if err := yaml.Unmarshal(content, &header); err != nil {
			return fmt.Errorf("inspect configuration schema: %w", err)
		}
		if header.SchemaVersion == configv1.ProjectSchemaVersion {
			schema = "v1"
		} else {
			schema = "legacy"
		}
	}
	switch schema {
	case "v1":
		project, err := configv1.LoadProject(validateConfigFile)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Valid canonical project: %s (%s)\n", project.Project.ID, project.Workflow.Scenario)
	case "legacy":
		loader := config.NewLoader(validateConfigFile)
		configuration, err := loader.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load legacy configuration: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Valid legacy configuration: mode=%s engine=%s\n", configuration.Workflow.Mode, configuration.Engine.Type)
	default:
		return fmt.Errorf("unsupported schema %q; expected auto, legacy, or v1", validateConfigSchema)
	}
	logger.Info("Configuration validation passed")
	return nil
}
