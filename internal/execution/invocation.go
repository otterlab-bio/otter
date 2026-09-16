package execution

import (
	"fmt"
	"path/filepath"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	runstate "github.com/otterlab-bio/otter/internal/run"
)

// RunInvocation is the executor-neutral, immutable execution boundary for one run.
type RunInvocation struct {
	SnapshotPath     string
	ProjectDirectory string
	RunRoot          string
	StateDirectory   string
	ResultsDirectory string
	Snapshot         configv1.RunSnapshot
}

// LoadRunInvocation verifies immutable snapshot identity and all frozen inputs before an executor starts.
func LoadRunInvocation(snapshotPath string, expectedExecutor configv1.Executor) (RunInvocation, error) {
	absoluteSnapshotPath, err := filepath.Abs(snapshotPath)
	if err != nil {
		return RunInvocation{}, fmt.Errorf("resolve run snapshot path: %w", err)
	}
	snapshot, err := configv1.LoadRunSnapshot(absoluteSnapshotPath)
	if err != nil {
		return RunInvocation{}, fmt.Errorf("load immutable otter.run/v1 run.yaml: %w", err)
	}
	if snapshot.Execution.Executor.Value != expectedExecutor {
		return RunInvocation{}, fmt.Errorf(
			"run snapshot selects executor %q, not %q",
			snapshot.Execution.Executor.Value,
			expectedExecutor,
		)
	}
	if err := runstate.RevalidateSnapshot(snapshot); err != nil {
		return RunInvocation{}, fmt.Errorf("revalidate immutable run snapshot: %w", err)
	}
	expectedSnapshotPath := filepath.Join(snapshot.Paths.RunRoot, "run.yaml")
	if filepath.Clean(absoluteSnapshotPath) != filepath.Clean(expectedSnapshotPath) {
		return RunInvocation{}, fmt.Errorf(
			"run snapshot path %q does not match immutable paths.run_root %q",
			absoluteSnapshotPath,
			snapshot.Paths.RunRoot,
		)
	}
	return RunInvocation{
		SnapshotPath:     absoluteSnapshotPath,
		ProjectDirectory: snapshot.Project.Root,
		RunRoot:          snapshot.Paths.RunRoot,
		StateDirectory:   snapshot.Paths.State,
		ResultsDirectory: snapshot.Paths.Results,
		Snapshot:         snapshot,
	}, nil
}
