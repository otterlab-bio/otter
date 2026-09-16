package task

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	EnvironmentTaskID       = "OTTER_TASK_ID"
	EnvironmentTaskStateDir = "OTTER_TASK_STATE_DIR"

	StatusQueued      = "queued"
	StatusRunning     = "running"
	StatusStopping    = "stopping"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusStopped     = "stopped"
	StatusInterrupted = "interrupted"
)

type Record struct {
	Version           string    `json:"version"`
	ID                string    `json:"id"`
	Status            string    `json:"status"`
	PID               int       `json:"pid,omitempty"`
	ProcessGroupID    int       `json:"process_group_id,omitempty"`
	ProjectDir        string    `json:"project_dir"`
	ConfigPath        string    `json:"config_path"`
	Engine            string    `json:"engine"`
	CraftmakeRunID    string    `json:"craftmake_run_id,omitempty"`
	CraftmakeBinary   string    `json:"craftmake_binary,omitempty"`
	Command           []string  `json:"command,omitempty"`
	LogPath           string    `json:"log_path"`
	StatePath         string    `json:"state_path,omitempty"`
	CurrentStep       int       `json:"current_step,omitempty"`
	Message           string    `json:"message,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	StartedAt         time.Time `json:"started_at,omitempty"`
	FinishedAt        time.Time `json:"finished_at,omitempty"`
	LastUpdate        time.Time `json:"last_update"`
	ExitCode          *int      `json:"exit_code,omitempty"`
	Error             string    `json:"error,omitempty"`
	ActiveSlurmJobIDs []string  `json:"active_slurm_job_ids,omitempty"`
	SlurmJobIDs       []string  `json:"slurm_job_ids,omitempty"`
}

type Store struct {
	rootDir string
}

func NewStore(rootDir string) *Store {
	return &Store{rootDir: filepath.Clean(rootDir)}
}

func DefaultStore() (*Store, error) {
	rootDir, err := DefaultStateDir()
	if err != nil {
		return nil, err
	}
	return NewStore(rootDir), nil
}

func DefaultStateDir() (string, error) {
	if configuredDir := strings.TrimSpace(os.Getenv(EnvironmentTaskStateDir)); configuredDir != "" {
		absoluteDir, err := filepath.Abs(configuredDir)
		if err != nil {
			return "", fmt.Errorf("resolve task state directory: %w", err)
		}
		return absoluteDir, nil
	}

	if xdgStateHome := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); xdgStateHome != "" {
		return filepath.Join(xdgStateHome, "otter", "tasks"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".local", "state", "otter", "tasks"), nil
}

func GenerateID() (string, error) {
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate task ID: %w", err)
	}
	return fmt.Sprintf("%s-%s", time.Now().Format("20060102-150405"), hex.EncodeToString(randomBytes)), nil
}

func NewRecord(taskID, projectDir, configPath, engineName string, command []string) *Record {
	now := time.Now()
	return &Record{
		Version:    "1.0",
		ID:         taskID,
		Status:     StatusQueued,
		ProjectDir: projectDir,
		ConfigPath: configPath,
		Engine:     engineName,
		Command:    append([]string(nil), command...),
		CreatedAt:  now,
		LastUpdate: now,
	}
}

func (store *Store) RootDir() string {
	return store.rootDir
}

func (store *Store) RecordPath(taskID string) string {
	return filepath.Join(store.rootDir, taskID+".json")
}

func (store *Store) LogPath(taskID string) string {
	return filepath.Join(store.rootDir, taskID+".log")
}

func (store *Store) Create(record *Record) error {
	if record == nil || strings.TrimSpace(record.ID) == "" {
		return fmt.Errorf("task record requires an ID")
	}
	if err := store.ensureRootDir(); err != nil {
		return err
	}

	return store.withLock(record.ID, func() error {
		if _, err := os.Stat(store.RecordPath(record.ID)); err == nil {
			return fmt.Errorf("task %s already exists", record.ID)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check task record: %w", err)
		}
		if record.LogPath == "" {
			record.LogPath = store.LogPath(record.ID)
		}
		return store.saveUnlocked(record)
	})
}

func (store *Store) Load(taskID string) (*Record, error) {
	data, err := os.ReadFile(store.RecordPath(taskID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task %s not found", taskID)
		}
		return nil, fmt.Errorf("read task %s: %w", taskID, err)
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parse task %s: %w", taskID, err)
	}
	return &record, nil
}

func (store *Store) Update(taskID string, update func(record *Record) error) error {
	if err := store.ensureRootDir(); err != nil {
		return err
	}
	return store.withLock(taskID, func() error {
		record, err := store.Load(taskID)
		if err != nil {
			return err
		}
		if err := update(record); err != nil {
			return err
		}
		record.LastUpdate = time.Now()
		return store.saveUnlocked(record)
	})
}

func (store *Store) List() ([]*Record, error) {
	if err := store.ensureRootDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(store.rootDir)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	records := make([]*Record, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		taskID := strings.TrimSuffix(entry.Name(), ".json")
		record, loadErr := store.Load(taskID)
		if loadErr != nil {
			continue
		}
		records = append(records, record)
	}

	sort.Slice(records, func(firstIndex, secondIndex int) bool {
		return records[firstIndex].CreatedAt.After(records[secondIndex].CreatedAt)
	})
	return records, nil
}

func (store *Store) RefreshProcessStatus(record *Record) error {
	if record == nil || !IsActiveStatus(record.Status) || record.PID <= 0 {
		return nil
	}
	if ProcessExists(record.PID) {
		return nil
	}
	return store.Update(record.ID, func(current *Record) error {
		if IsActiveStatus(current.Status) && !ProcessExists(current.PID) {
			current.Status = StatusInterrupted
			current.FinishedAt = time.Now()
			current.Error = "background worker is no longer running"
		}
		return nil
	})
}

func IsActiveStatus(status string) bool {
	return status == StatusQueued || status == StatusRunning || status == StatusStopping
}

func IsTerminalStatus(status string) bool {
	return status == StatusCompleted || status == StatusFailed || status == StatusStopped || status == StatusInterrupted
}

func ProcessExists(processID int) bool {
	if processID <= 0 {
		return false
	}
	err := syscall.Kill(processID, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func CurrentID() string {
	return strings.TrimSpace(os.Getenv(EnvironmentTaskID))
}

func UpdateCurrent(update func(record *Record) error) error {
	taskID := CurrentID()
	if taskID == "" {
		return nil
	}
	store, err := DefaultStore()
	if err != nil {
		return err
	}
	return store.Update(taskID, update)
}

func RegisterCurrentSlurmJob(jobID string) error {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil
	}
	return UpdateCurrent(func(record *Record) error {
		record.ActiveSlurmJobIDs = appendUnique(record.ActiveSlurmJobIDs, jobID)
		record.SlurmJobIDs = appendUnique(record.SlurmJobIDs, jobID)
		return nil
	})
}

func CompleteCurrentSlurmJob(jobID string) error {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil
	}
	return UpdateCurrent(func(record *Record) error {
		filteredJobIDs := make([]string, 0, len(record.ActiveSlurmJobIDs))
		for _, activeJobID := range record.ActiveSlurmJobIDs {
			if activeJobID != jobID {
				filteredJobIDs = append(filteredJobIDs, activeJobID)
			}
		}
		record.ActiveSlurmJobIDs = filteredJobIDs
		return nil
	})
}

func (store *Store) ensureRootDir() error {
	if err := os.MkdirAll(store.rootDir, 0o700); err != nil {
		return fmt.Errorf("create task state directory: %w", err)
	}
	return nil
}

func (store *Store) saveUnlocked(record *Record) error {
	record.LastUpdate = time.Now()
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode task %s: %w", record.ID, err)
	}

	temporaryFile, err := os.CreateTemp(store.rootDir, record.ID+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary task record: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	if err := temporaryFile.Chmod(0o600); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("set task record permissions: %w", err)
	}
	if _, err := temporaryFile.Write(data); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("write task record: %w", err)
	}
	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("sync task record: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("close task record: %w", err)
	}
	if err := os.Rename(temporaryPath, store.RecordPath(record.ID)); err != nil {
		return fmt.Errorf("replace task record: %w", err)
	}
	return nil
}

func (store *Store) withLock(taskID string, operation func() error) error {
	lockPath := filepath.Join(store.rootDir, taskID+".lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open task lock: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock task record: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	return operation()
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}
