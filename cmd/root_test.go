package cmd

import (
	"errors"
	"testing"

	craftmakeclient "github.com/otterlab-bio/otter/internal/craftmake"
)

func TestExitCodePreservesCraftmakeClassification(t *testing.T) {
	classifiedError := craftmakeclient.NewExitCodeError(craftmakeclient.CommandRun, 7, errors.New("resource failure"))
	if exitCode := ExitCode(classifiedError); exitCode != 7 {
		t.Fatalf("ExitCode() = %d, want 7", exitCode)
	}
}

func TestExitCodeFallsBackToOne(t *testing.T) {
	if exitCode := ExitCode(errors.New("unclassified failure")); exitCode != 1 {
		t.Fatalf("ExitCode() = %d, want 1", exitCode)
	}
}
