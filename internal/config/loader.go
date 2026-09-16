package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/internal/logger"
	"github.com/spf13/viper"
)

// Loader handles configuration loading
type Loader struct {
	configPath string
	viper      *viper.Viper
}

// NewLoader creates a new configuration loader
func NewLoader(configPath string) *Loader {
	v := viper.New()
	v.SetConfigFile(configPath)

	return &Loader{
		configPath: configPath,
		viper:      v,
	}
}

// LoadConfig loads configuration from file with legacy environment overrides.
func (l *Loader) LoadConfig() (*OtterConfig, error) {
	return l.loadConfig(true)
}

// LoadConfigWithoutEnvironmentOverrides loads a legacy configuration deterministically for migration.
func (l *Loader) LoadConfigWithoutEnvironmentOverrides() (*OtterConfig, error) {
	return l.loadConfig(false)
}

func (l *Loader) loadConfig(applyEnvironmentOverrides bool) (*OtterConfig, error) {
	config := LoadDefaults()

	// Load from file if it exists
	err := l.viper.ReadInConfig()
	switch {
	case err == nil:
		logger.Debugf("Loading config from %s", l.configPath)
		if err := l.viper.Unmarshal(&config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	case os.IsNotExist(err):
		logger.Debugf("Config file not found at %s, using defaults", l.configPath)
	default:
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Auto-derive suffix2 if not set
	if config.Input.Suffix2 == "" {
		config.Input.Suffix2 = deriveSuffix2(config.Input.Suffix1)
		logger.Debugf("Auto-derived suffix2: %s", config.Input.Suffix2)
	}

	if applyEnvironmentOverrides {
		l.mergeEnvOverrides(config)
	}

	// Merge flat reference fields into nested structure
	l.mergeReferenceFields(config)
	// Merge flat compatibility fields into nested structure
	l.mergeFlatCompatFields(config)

	// Validate configuration
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to file
func (l *Loader) SaveConfig(config *OtterConfig) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(l.configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write to file
	if err := l.viper.WriteConfigAs(l.configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	logger.Infof("Configuration saved to %s", l.configPath)
	return nil
}

// mergeEnvOverrides merges environment variable overrides
func (l *Loader) mergeEnvOverrides(config *OtterConfig) {
	// Mode
	if mode := os.Getenv("OTTER_MODE"); mode != "" {
		config.Workflow.Mode = mode
	}

	// Species
	if species1 := os.Getenv("OTTER_SPECIES1"); species1 != "" {
		config.Workflow.Species.Primary = species1
	}
	if species2 := os.Getenv("OTTER_SPECIES2"); species2 != "" {
		config.Workflow.Species.Secondary = species2
	}

	// FASTQ
	if fastqDir := os.Getenv("OTTER_FASTQ_DIR"); fastqDir != "" {
		config.Input.FastqDir = fastqDir
	}

	logger.Debug("Environment variable overrides applied")
}

// mergeReferenceFields merges flat reference fields into nested structure
func (l *Loader) mergeReferenceFields(config *OtterConfig) {
	// Merge GenomeFasta into Files.Fasta for backward compatibility
	if len(config.Reference.GenomeFasta) > 0 && len(config.Reference.Files.Fasta) == 0 {
		config.Reference.Files.Fasta = config.Reference.GenomeFasta
		logger.Debugf("Merged genome_fasta into files.fasta")
	}
}

// mergeFlatCompatFields merges flat top-level fields into nested structure for compatibility.
func (l *Loader) mergeFlatCompatFields(config *OtterConfig) {
	// mode -> workflow.mode (used by Go runtime config validation and execution)
	if config.Workflow.Mode == "" {
		if mode := l.viper.GetString("mode"); mode != "" {
			config.Workflow.Mode = mode
			logger.Debugf("Merged top-level mode into workflow.mode")
		}
	}
	if len(config.Metadata.SampleIDs) == 0 {
		for _, key := range []string{"metadata.sample_ids", "metadata.SIDs", "SIDs"} {
			if sampleIDs := l.viper.GetStringSlice(key); len(sampleIDs) > 0 {
				config.Metadata.SampleIDs = sampleIDs
				logger.Debugf("Merged %s into metadata.SIDs", key)
				break
			}
		}
	}
}

// deriveSuffix2 auto-derives suffix2 from suffix1
func deriveSuffix2(suffix1 string) string {
	if strings.Contains(suffix1, "R1") {
		return strings.Replace(suffix1, "R1", "R2", 1)
	}
	if strings.Contains(suffix1, "1") {
		return strings.Replace(suffix1, "1", "2", 1)
	}
	return suffix1
}
