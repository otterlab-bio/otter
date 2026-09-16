package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	StateFileVersion = "1.0"
	StateFileName    = ".otter_state.json"
)

// StateFile represents the persistent workflow state
type StateFile struct {
	Version    string      `json:"version"`
	JobID      string      `json:"job_id"`
	StartTime  time.Time   `json:"start_time"`
	LastUpdate time.Time   `json:"last_update"`
	Status     string      `json:"status"` // pending, running, completed, failed
	Config     StateConfig `json:"config"`
	Steps      []StepState `json:"steps"`
	Samples    SampleState `json:"samples"`
}

// StateConfig represents critical configuration for validation
type StateConfig struct {
	WorkflowMode string `json:"workflow_mode"`
	Species1     string `json:"species1"`
	Species2     string `json:"species2,omitempty"`
	SampleCount  int    `json:"sample_count"`
	EngineType   string `json:"engine_type"`
	Partition    string `json:"partition,omitempty"`
}

// StepState represents the state of a workflow step
type StepState struct {
	Step      int       `json:"step"`
	Name      string    `json:"name"`
	Status    string    `json:"status"` // pending, running, completed, failed
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
	JobID     string    `json:"slurm_job_id,omitempty"`
}

// SampleState represents the state of samples
type SampleState struct {
	Completed []string `json:"completed"`
	Running   []string `json:"running"`
	Pending   []string `json:"pending"`
}

// State manages workflow state persistence
type State struct {
	filePath string
	data     *StateFile
}

// NewState creates a new state manager
func NewState(outputDir, jobID string) *State {
	return &State{
		filePath: filepath.Join(outputDir, StateFileName),
		data: &StateFile{
			Version:   StateFileVersion,
			JobID:     jobID,
			StartTime: time.Now(),
			Status:    "pending",
			Steps:     make([]StepState, 0),
			Samples: SampleState{
				Completed: make([]string, 0),
				Running:   make([]string, 0),
				Pending:   make([]string, 0),
			},
		},
	}
}

// Initialize initializes the state with workflow configuration
func (s *State) Initialize(jobID string, mode, species1, species2 string, stepCount int, samples []string, engineType, partition string) error {
	now := time.Now()
	s.data.JobID = jobID
	s.data.StartTime = now
	s.data.LastUpdate = now
	s.data.Status = "running"

	// Store configuration for validation
	s.data.Config = StateConfig{
		WorkflowMode: mode,
		Species1:     species1,
		Species2:     species2,
		SampleCount:  len(samples),
		EngineType:   engineType,
		Partition:    partition,
	}

	// Initialize sample states
	s.data.Samples = SampleState{
		Completed: make([]string, 0),
		Running:   make([]string, 0),
		Pending:   make([]string, 0, len(samples)),
	}

	// Initialize step states
	if stepCount <= 0 {
		stepCount = 3
	}
	s.data.Steps = make([]StepState, 0, stepCount)
	for step := 1; step <= stepCount; step++ {
		s.data.Steps = append(s.data.Steps, StepState{
			Step:   step,
			Name:   defaultStepName(step),
			Status: "pending",
		})
	}

	return s.Save()
}

// Load loads state from disk
func (s *State) Load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	if err := json.Unmarshal(data, &s.data); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	// Validate version
	if s.data.Version != StateFileVersion {
		return fmt.Errorf("incompatible state file version: %s (expected %s)",
			s.data.Version, StateFileVersion)
	}

	s.normalizeLegacySteps()

	return nil
}

// Save saves state to disk
func (s *State) Save() error {
	s.data.LastUpdate = time.Now()

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	stateDir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	temporaryFile, err := os.CreateTemp(stateDir, ".otter-state-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary state file: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	if err := temporaryFile.Chmod(0o644); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("failed to set state file permissions: %w", err)
	}
	if _, err := temporaryFile.Write(data); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("failed to write state file: %w", err)
	}
	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("failed to sync state file: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("failed to close state file: %w", err)
	}
	if err := os.Rename(temporaryPath, s.filePath); err != nil {
		return fmt.Errorf("failed to replace state file: %w", err)
	}

	return nil
}

// Exists checks if state file exists
func (s *State) Exists() bool {
	_, err := os.Stat(s.filePath)
	return err == nil
}

// GetLastCompletedStep returns the last completed step number
func (s *State) GetLastCompletedStep() int {
	for i := len(s.data.Steps) - 1; i >= 0; i-- {
		if s.data.Steps[i].Status == "completed" {
			return s.data.Steps[i].Step
		}
	}
	return 0
}

// IsStepCompleted checks if a step is completed
func (s *State) IsStepCompleted(step int) bool {
	for _, stepState := range s.data.Steps {
		if stepState.Step == step && stepState.Status == "completed" {
			return true
		}
	}
	return false
}

// UpdateStepStatus updates the status of a step
func (s *State) UpdateStepStatus(step int, status string, startTime time.Time, jobID string) error {
	for i := range s.data.Steps {
		if s.data.Steps[i].Step == step {
			s.data.Steps[i].Status = status

			if !startTime.IsZero() && s.data.Steps[i].StartTime.IsZero() {
				s.data.Steps[i].StartTime = startTime
			}

			if status == "completed" || status == "failed" {
				s.data.Steps[i].EndTime = time.Now()
			}

			if jobID != "" {
				s.data.Steps[i].JobID = jobID
			}

			return nil
		}
	}
	return fmt.Errorf("step %d not found", step)
}

// MarkCompleted marks the workflow as completed
func (s *State) MarkCompleted() error {
	s.data.Status = "completed"
	return s.Save()
}

// MarkFailed marks the workflow as failed
func (s *State) MarkFailed() error {
	s.data.Status = "failed"
	return s.Save()
}

// GetJobID returns the SLURM job ID for a step
func (s *State) GetJobID(step int) string {
	for _, stepState := range s.data.Steps {
		if stepState.Step == step {
			return stepState.JobID
		}
	}
	return ""
}

// GetStatus returns the overall workflow status
func (s *State) GetStatus() string {
	return s.data.Status
}

// GetData returns the underlying state data
func (s *State) GetData() *StateFile {
	return s.data
}

// GetFilePath returns the state file path
func (s *State) GetFilePath() string {
	return s.filePath
}

func defaultStepName(step int) string {
	switch step {
	case 1:
		return "quality_control"
	case 2:
		return "alignment"
	case 3:
		return "methylation_calling"
	default:
		return fmt.Sprintf("step_%d", step)
	}
}

func expectedStepCount(mode string, species2 string) int {
	modeUpper := strings.ToUpper(strings.TrimSpace(mode))
	if modeUpper == "RNASEQ" && strings.TrimSpace(species2) == "" {
		return 2
	}
	return 3
}

// normalizeLegacySteps keeps old state files resumable after step-count logic changes.
func (s *State) normalizeLegacySteps() {
	expected := expectedStepCount(s.data.Config.WorkflowMode, s.data.Config.Species2)
	if expected <= 0 {
		return
	}

	switch {
	case len(s.data.Steps) > expected:
		s.data.Steps = s.data.Steps[:expected]
	case len(s.data.Steps) < expected:
		for step := len(s.data.Steps) + 1; step <= expected; step++ {
			s.data.Steps = append(s.data.Steps, StepState{
				Step:   step,
				Name:   defaultStepName(step),
				Status: "pending",
			})
		}
	}

	for i := range s.data.Steps {
		s.data.Steps[i].Step = i + 1
		if strings.TrimSpace(s.data.Steps[i].Name) == "" {
			s.data.Steps[i].Name = defaultStepName(i + 1)
		}
	}
}
