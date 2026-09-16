package engine

// EngineType represents the type of execution engine
type EngineType string

// Engine type constants
const (
	EngineSlurm      EngineType = "slurm"
	EngineLocal      EngineType = "local"
	EngineSlurmArray EngineType = "slurm_array"
)

// String returns the string representation of the engine type
func (e EngineType) String() string {
	return string(e)
}
