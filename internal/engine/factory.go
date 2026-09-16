package engine

import (
	"fmt"
	"os"
	"time"

	"github.com/otterlab-bio/otter/internal/config"
	"github.com/otterlab-bio/otter/internal/logger"
)

// EngineFactory creates engines based on type and configuration
type EngineFactory struct{}

func buildSlurmEngineConfig(source config.SlurmConfig) (*SlurmConfig, error) {
	waitTimeout := time.Duration(0)
	if source.WaitTimeout != "" {
		parsedWaitTimeout, err := time.ParseDuration(source.WaitTimeout)
		if err != nil {
			return nil, fmt.Errorf("parse Slurm wait_timeout %q: %w", source.WaitTimeout, err)
		}
		if parsedWaitTimeout < 0 {
			return nil, fmt.Errorf("Slurm wait_timeout must not be negative: %s", source.WaitTimeout)
		}
		waitTimeout = parsedWaitTimeout
	}

	return &SlurmConfig{
		Partition:   source.Partition,
		Cores:       source.Cores,
		Memory:      source.Memory,
		Time:        source.Time,
		JobName:     source.JobName,
		MaxRetries:  source.MaxRetries,
		WaitTimeout: waitTimeout,
	}, nil
}

// NewEngine creates an engine based on the specified type and configuration
func (f *EngineFactory) NewEngine(engineType EngineType, cfg *config.EngineConfig) (Engine, error) {
	switch engineType {
	case EngineSlurm:
		slurmConfig, err := buildSlurmEngineConfig(cfg.Slurm)
		if err != nil {
			return nil, err
		}
		return NewSlurmEngine(slurmConfig), nil

	case EngineLocal:
		localConfig := &LocalConfig{
			MaxCores:  cfg.Local.MaxCores,
			MaxMemory: cfg.Local.MaxMemory,
		}
		return NewLocalEngine(localConfig), nil

	case EngineSlurmArray:
		slurmConfig, err := buildSlurmEngineConfig(cfg.Slurm)
		if err != nil {
			return nil, err
		}
		// For array engine, samples and stepResource need to be set later
		return NewSlurmArrayEngine(slurmConfig, nil, nil), nil

	default:
		return nil, fmt.Errorf("unsupported engine type: %s", engineType)
	}
}

// NewSlurmArrayEngineWithResources creates a SlurmArrayEngine with samples and step resources
func (f *EngineFactory) NewSlurmArrayEngineWithResources(
	cfg *config.EngineConfig,
	samples []string,
	stepResource *config.StepResource,
	maxBatchSize int,
	loadRatio float64,
) (*SlurmArrayEngine, error) {
	// Use stepResource values if available, otherwise fallback to cfg.Slurm values
	cores := cfg.Slurm.Cores
	memory := cfg.Slurm.Memory
	partition := cfg.Slurm.Partition
	timeLimit := cfg.Slurm.Time

	logger.Debugf("NewSlurmArrayEngineWithResources: cfg.Slurm.Cores=%d, cfg.Slurm.Memory=%s", cores, memory)

	if stepResource != nil {
		logger.Debugf("stepResource.Cores=%d, stepResource.Memory=%s", stepResource.Cores, stepResource.Memory)
		if stepResource.Cores > 0 {
			cores = stepResource.Cores
		}
		if stepResource.Memory != "" {
			memory = stepResource.Memory
		}
		if stepResource.Time != "" {
			timeLimit = stepResource.Time
		}
		if stepResource.Partition != "" {
			partition = stepResource.Partition
		}
	}

	logger.Debugf("Final partition=%s, cores=%d, memory=%s", partition, cores, memory)

	slurmConfig, err := buildSlurmEngineConfig(cfg.Slurm)
	if err != nil {
		return nil, err
	}
	slurmConfig.Partition = partition
	slurmConfig.Cores = cores
	slurmConfig.Memory = memory
	slurmConfig.Time = timeLimit

	eng := NewSlurmArrayEngine(slurmConfig, samples, stepResource)
	eng.maxBatchSize = maxBatchSize
	eng.loadRatio = loadRatio
	return eng, nil
}

// DetectEngine automatically detects the execution environment
func DetectEngine() EngineType {
	// Check for Slurm environment variables
	if os.Getenv("SLURM_JOB_ID") != "" || os.Getenv("SLURM_CLUSTER_NAME") != "" {
		return EngineSlurm
	}

	// Default to local
	return EngineLocal
}

// CreateEngineFromConfig creates an engine from configuration
// It handles auto-detection if engine type is "auto"
func CreateEngineFromConfig(cfg *config.OtterConfig) (Engine, error) {
	factory := &EngineFactory{}

	engineType := EngineType(cfg.Engine.Type)

	// Auto-detect if specified
	if engineType == "auto" {
		engineType = DetectEngine()
	}

	return factory.NewEngine(engineType, &cfg.Engine)
}
