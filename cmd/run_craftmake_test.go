package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	craftmakeclient "github.com/otterlab-bio/otter/internal/craftmake"
)

func TestSelectedRunExecutorDefaultsToCraftmake(t *testing.T) {
	originalExecutor := runExecutor
	t.Cleanup(func() { runExecutor = originalExecutor })
	runExecutor = runExecutorCraftmake
	selected, err := selectedRunExecutor()
	if err != nil {
		t.Fatal(err)
	}
	if selected != runExecutorCraftmake {
		t.Fatalf("unexpected default executor %q", selected)
	}
}

func TestCraftmakeRunArgumentsScopeControllerRunToPhase(t *testing.T) {
	originalPhase := runPhase
	originalWorkflowPath := runWorkflowPath
	originalCatalog := runWorkflowCatalog
	originalDryRun := dryRun
	originalResume := resumeFlag
	originalParallelJobs := parallelJobs
	t.Cleanup(func() {
		runPhase = originalPhase
		runWorkflowPath = originalWorkflowPath
		runWorkflowCatalog = originalCatalog
		dryRun = originalDryRun
		resumeFlag = originalResume
		parallelJobs = originalParallelJobs
	})
	runPhase = "step1"
	runWorkflowPath = ""
	runWorkflowCatalog = "/opt/craftmake/workflows"
	dryRun = false
	resumeFlag = false
	parallelJobs = 4
	snapshot := configv1.RunSnapshot{
		Run: configv1.RunMetadata{ID: "run-20260726T013245Z-kxqjrm"},
		Execution: configv1.ResolvedExecution{
			Backend: configv1.ResolvedBackend{Value: configv1.BackendSlurm},
		},
		Paths: configv1.RunPaths{
			RunRoot: "/shared/project/runs/run-20260726T013245Z-kxqjrm",
			State:   "/shared/project/runs/run-20260726T013245Z-kxqjrm/state",
		},
	}
	command, arguments, err := craftmakeRunArguments("/shared/project/runs/run-20260726T013245Z-kxqjrm/run.yaml", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if command != craftmakeclient.CommandRun {
		t.Fatalf("unexpected command %q", command)
	}
	expectedStateDirectory := filepath.Join(snapshot.Paths.RunRoot, "state")
	for index, argument := range arguments {
		if argument == "--run-id" {
			t.Fatalf("Otter must let Craftmake derive its phase-scoped controller ID: %#v", arguments[index:])
		}
	}
	assertArgumentPair(t, arguments, "--backend", "slurm")
	assertArgumentAbsent(t, arguments, "--legacy-config")
	assertArgumentPair(t, arguments, "--config", "/shared/project/runs/run-20260726T013245Z-kxqjrm/run.yaml")
	assertArgumentPair(t, arguments, "--state-dir", expectedStateDirectory)
	assertArgumentPair(t, arguments, "--phase", "step1")
	// --gate is what makes Craftmake scope run identity to the phase. See the
	// phase-run-id test below for why the suffix in task correlation and
	// --resume depends on it.
	assertArgumentPresent(t, arguments, "--gate")
	for _, immutableResourceFlag := range []string{"--partition", "--account", "--qos", "--time", "--scratch-root"} {
		for _, argument := range arguments {
			if argument == immutableResourceFlag {
				t.Fatalf("Otter must not override immutable Craftmake resource %s", immutableResourceFlag)
			}
		}
	}
}

func TestCraftmakeResumeScopesControllerRunToPhase(t *testing.T) {
	originalPhase := runPhase
	originalResume := resumeFlag
	t.Cleanup(func() {
		runPhase = originalPhase
		resumeFlag = originalResume
	})
	runPhase = "step2"
	resumeFlag = true
	snapshot := configv1.RunSnapshot{
		Run:   configv1.RunMetadata{ID: "run-20260726T013245Z-kxqjrm"},
		Paths: configv1.RunPaths{State: "/shared/project/runs/run-20260726T013245Z-kxqjrm/state"},
	}
	command, arguments, err := craftmakeRunArguments("/shared/project/run.yaml", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if command != craftmakeclient.CommandResume {
		t.Fatalf("unexpected command %q", command)
	}
	assertArgumentPair(t, arguments, "--run", "run-20260726T013245Z-kxqjrm--step2")
}

func TestCraftmakeRunArgumentsRequireExplicitCompilationEntry(t *testing.T) {
	originalPhase := runPhase
	originalWorkflowPath := runWorkflowPath
	originalDryRun := dryRun
	originalResume := resumeFlag
	t.Cleanup(func() {
		runPhase = originalPhase
		runWorkflowPath = originalWorkflowPath
		dryRun = originalDryRun
		resumeFlag = originalResume
	})
	runPhase = ""
	runWorkflowPath = ""
	dryRun = false
	resumeFlag = false
	_, _, err := craftmakeRunArguments("/tmp/run.yaml", configv1.RunSnapshot{})
	if err == nil {
		t.Fatal("expected missing phase or workflow to fail")
	}
}

func assertArgumentAbsent(t *testing.T, arguments []string, unexpectedArgument string) {
	t.Helper()
	for _, argument := range arguments {
		if argument == unexpectedArgument {
			t.Fatalf("argument %s must not be present in %#v", unexpectedArgument, arguments)
		}
	}
}

func assertArgumentPair(t *testing.T, arguments []string, name string, expectedValue string) {
	t.Helper()
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == name {
			if arguments[index+1] != expectedValue {
				t.Fatalf("argument %s: got %q, expected %q", name, arguments[index+1], expectedValue)
			}
			return
		}
	}
	t.Fatalf("argument %s was not present in %#v", name, arguments)
}

// assertArgumentPresent asserts a valueless flag is present.
func assertArgumentPresent(t *testing.T, arguments []string, flag string) {
	t.Helper()
	for _, argument := range arguments {
		if argument == flag {
			return
		}
	}
	t.Fatalf("expected %s in %#v", flag, arguments)
}

// TestCraftmakeRunIdentityIsPhaseScoped pins the cross-component identity
// contract: Otter correlates tasks and resumes by "<run-id>--<phase>", and
// Craftmake only records that identity when it is invoked with --gate. Without
// --gate Craftmake writes the plain run ID, so a second phase collides on the
// runs primary key and --resume looks up a run that was never written.
func TestCraftmakeRunIdentityIsPhaseScoped(t *testing.T) {
	originalPhase := runPhase
	originalDryRun := dryRun
	originalResume := resumeFlag
	t.Cleanup(func() {
		runPhase = originalPhase
		dryRun = originalDryRun
		resumeFlag = originalResume
	})
	snapshot := configv1.RunSnapshot{
		Run: configv1.RunMetadata{ID: "run-20260726T013245Z-kxqjrm"},
		Execution: configv1.ResolvedExecution{
			Backend: configv1.ResolvedBackend{Value: configv1.BackendLocal},
		},
		Paths: configv1.RunPaths{
			RunRoot: "/shared/project/runs/run-20260726T013245Z-kxqjrm",
			State:   "/shared/project/runs/run-20260726T013245Z-kxqjrm/state",
		},
	}

	// A real run must carry --gate so the phase-scoped identity is recorded.
	runPhase = "step2"
	dryRun = false
	resumeFlag = false
	command, arguments, err := craftmakeRunArguments("/shared/project/run.yaml", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if command != craftmakeclient.CommandRun {
		t.Fatalf("command = %q", command)
	}
	assertArgumentPresent(t, arguments, "--gate")

	// A dry run maps to `plan`, which records no run row, so it must not claim
	// the immutable identity.
	runPhase = "step2"
	dryRun = true
	command, arguments, err = craftmakeRunArguments("/shared/project/run.yaml", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if command != craftmakeclient.CommandPlan {
		t.Fatalf("command = %q", command)
	}
	for _, argument := range arguments {
		if argument == "--gate" {
			t.Fatal("a plan must not assert the immutable run identity")
		}
	}

	// Resume must address the phase-scoped identity --gate produces.
	runPhase = "step2"
	dryRun = false
	resumeFlag = true
	command, arguments, err = craftmakeRunArguments("/shared/project/run.yaml", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if command != craftmakeclient.CommandResume {
		t.Fatalf("command = %q", command)
	}
	assertArgumentPair(t, arguments, "--run", "run-20260726T013245Z-kxqjrm--step2")
}

// TestCraftmakeRunFailureGuidanceExplainsRepeatedExecution pins the recovery
// guidance for the one Craftmake failure Otter can explain: the immutable
// snapshot already executed for that phase, so Craftmake's state row collides.
// The raw SQLite error is unactionable, and the user reaching it has usually
// just fixed a failing step and retried.
func TestCraftmakeRunFailureGuidanceExplainsRepeatedExecution(t *testing.T) {
	snapshot := configv1.RunSnapshot{Run: configv1.RunMetadata{ID: "run-20260726T013245Z-kxqjrm"}}
	const collision = "create run: constraint failed: UNIQUE constraint failed: runs.run_id (1555)"

	guidance := craftmakeRunFailureGuidance(craftmakeclient.CommandRun, collision, "/shared/project/run.yaml", snapshot, "step1")
	if guidance == "" {
		t.Fatal("expected guidance for a repeated execution")
	}
	t.Log(guidance)
	for _, expected := range []string{"run-20260726T013245Z-kxqjrm--step1", "--resume", "/shared/project/run.yaml"} {
		if !strings.Contains(guidance, expected) {
			t.Fatalf("guidance %q does not mention %q", guidance, expected)
		}
	}

	// Unrelated failures and non-run commands must keep their own error.
	if guidance := craftmakeRunFailureGuidance(craftmakeclient.CommandRun, "exit status 1", "/shared/project/run.yaml", snapshot, "step1"); guidance != "" {
		t.Fatalf("unexpected guidance for an unrelated failure: %q", guidance)
	}
	if guidance := craftmakeRunFailureGuidance(craftmakeclient.CommandResume, collision, "/shared/project/run.yaml", snapshot, "step1"); guidance != "" {
		t.Fatalf("unexpected guidance for resume: %q", guidance)
	}
}
