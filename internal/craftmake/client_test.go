package craftmake

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteParsesVersionedEnvelope(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "craftmake")
	script := `#!/bin/sh
printf '%s\n' '{"protocol_version":"otter.craftmake/v1","command":"status","ok":true,"run_id":"run-20260726T013245Z-kxqjrm","state_path":"/tmp/state.sqlite","data":{"status":"succeeded"}}'
`
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), Request{Binary: binaryPath, Command: CommandStatus})
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed() || result.Envelope.RunID != "run-20260726T013245Z-kxqjrm" || result.Envelope.Command != "status" {
		t.Fatalf("unexpected Craftmake result: %#v", result)
	}
}

func TestExecuteRejectsUnexpectedCommand(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "craftmake")
	script := `#!/bin/sh
printf '%s\n' '{"protocol_version":"otter.craftmake/v1","command":"status","ok":true}'
`
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Execute(context.Background(), Request{Binary: binaryPath, Command: CommandRun})
	if err == nil {
		t.Fatal("expected mismatched command to fail")
	}
}

func TestExecutePreservesCraftmakeFailureExitCode(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "craftmake")
	script := `#!/bin/sh
printf '%s\n' '{"protocol_version":"otter.craftmake/v1","command":"run","ok":false}'
exit 6
`
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), Request{Binary: binaryPath, Command: CommandRun})
	if err == nil {
		t.Fatal("expected Craftmake failure")
	}
	var classifiedError *ExitCodeError
	if !errors.As(err, &classifiedError) {
		t.Fatalf("expected classified Craftmake error, got %T: %v", err, err)
	}
	if result.ExitCode != 6 || classifiedError.ExitCode() != 6 {
		t.Fatalf("Craftmake exit code was not preserved: result=%d error=%d", result.ExitCode, classifiedError.ExitCode())
	}
}

func TestResolveBinaryRejectsDirectory(t *testing.T) {
	_, err := ResolveBinary(t.TempDir())
	if err == nil {
		t.Fatal("expected a directory to be rejected as a Craftmake binary")
	}
}

func TestExecuteRejectsIncompatibleProtocol(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "craftmake")
	script := `#!/bin/sh
printf '%s\n' '{"protocol_version":"otter.craftmake/v2","command":"status","ok":true}'
`
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Execute(context.Background(), Request{Binary: binaryPath, Command: CommandStatus})
	if err == nil {
		t.Fatal("expected incompatible protocol to fail")
	}
}
