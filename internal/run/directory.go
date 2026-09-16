package run

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const createAttempts = 32

type Directory struct {
	ID        string
	CreatedAt time.Time
	Root      string
}

func CreateDirectory(projectRoot string) (Directory, error) {
	return createDirectory(projectRoot, time.Now().UTC(), rand.Reader)
}

func CreateDirectoryWithID(projectRoot string, runID string, createdAt time.Time) (Directory, error) {
	if err := ValidateID(runID); err != nil {
		return Directory{}, err
	}
	return createExactDirectory(projectRoot, runID, createdAt.UTC())
}

func createDirectory(projectRoot string, createdAt time.Time, randomSource io.Reader) (Directory, error) {
	runsRoot := filepath.Join(projectRoot, "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return Directory{}, fmt.Errorf("create runs directory: %w", err)
	}
	for attempt := 0; attempt < createAttempts; attempt++ {
		runID, err := generateID(createdAt, randomSource)
		if err != nil {
			return Directory{}, err
		}
		directory, err := createExactDirectory(projectRoot, runID, createdAt)
		if err == nil {
			return directory, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return Directory{}, err
		}
	}
	return Directory{}, fmt.Errorf("could not allocate a unique run ID after %d attempts", createAttempts)
}

func createExactDirectory(projectRoot string, runID string, createdAt time.Time) (Directory, error) {
	runsRoot := filepath.Join(projectRoot, "runs")
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return Directory{}, fmt.Errorf("create runs directory: %w", err)
	}
	runRoot := filepath.Join(runsRoot, runID)
	if err := os.Mkdir(runRoot, 0o755); err != nil {
		return Directory{}, fmt.Errorf("create run directory %s: %w", runRoot, err)
	}
	for _, child := range []string{"input", "work", "results", "logs", "state", "metrics"} {
		if err := os.Mkdir(filepath.Join(runRoot, child), 0o755); err != nil {
			_ = os.RemoveAll(runRoot)
			return Directory{}, fmt.Errorf("create run child directory %s: %w", child, err)
		}
	}
	return Directory{ID: runID, CreatedAt: createdAt.UTC(), Root: runRoot}, nil
}
