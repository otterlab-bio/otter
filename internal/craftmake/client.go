package craftmake

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const ProtocolVersion = "otter.craftmake/v1"

type Command string

const (
	CommandValidate Command = "validate"
	CommandPlan     Command = "plan"
	CommandRun      Command = "run"
	CommandResume   Command = "resume"
	CommandStatus   Command = "status"
	CommandLogs     Command = "logs"
	CommandReport   Command = "report"
	CommandCancel   Command = "cancel"
)

type Envelope struct {
	ProtocolVersion string          `json:"protocol_version"`
	Command         string          `json:"command"`
	OK              bool            `json:"ok"`
	RunID           string          `json:"run_id,omitempty"`
	StatePath       string          `json:"state_path,omitempty"`
	ControllerLog   string          `json:"controller_log,omitempty"`
	Data            json.RawMessage `json:"data,omitempty"`
}

type Request struct {
	Binary    string
	Command   Command
	Arguments []string
}

type Result struct {
	Envelope Envelope
	Stdout   string
	Stderr   string
	ExitCode int
}

// ExitCodeError preserves Craftmake's classified process status through Otter's executor boundary.
type ExitCodeError struct {
	Code    int
	Command Command
	Cause   error
}

func (errorValue *ExitCodeError) Error() string {
	if errorValue.Cause != nil {
		return fmt.Sprintf("Craftmake %s failed with exit code %d: %v", errorValue.Command, errorValue.Code, errorValue.Cause)
	}
	return fmt.Sprintf("Craftmake %s failed with exit code %d", errorValue.Command, errorValue.Code)
}

func (errorValue *ExitCodeError) Unwrap() error {
	return errorValue.Cause
}

func (errorValue *ExitCodeError) ExitCode() int {
	return errorValue.Code
}

func NewExitCodeError(command Command, exitCode int, cause error) error {
	if exitCode <= 0 {
		exitCode = 1
	}
	return &ExitCodeError{Code: exitCode, Command: command, Cause: cause}
}

func (result Result) Failed() bool {
	return result.ExitCode != 0 || !result.Envelope.OK
}

func Execute(ctx context.Context, request Request) (Result, error) {
	binary := strings.TrimSpace(request.Binary)
	if binary == "" {
		binary = "craftmake"
	}
	standardOutput := &bytes.Buffer{}
	standardError := &bytes.Buffer{}
	command := exec.Command(binary, append([]string{string(request.Command)}, request.Arguments...)...)
	command.Stdout = standardOutput
	command.Stderr = standardError
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return Result{}, fmt.Errorf("start Craftmake: %w", err)
	}

	processDone := make(chan error, 1)
	go func() { processDone <- command.Wait() }()
	select {
	case <-ctx.Done():
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		<-processDone
		return Result{ExitCode: 8, Stdout: standardOutput.String(), Stderr: standardError.String()}, NewExitCodeError(request.Command, 8, ctx.Err())
	case waitErr := <-processDone:
		result := Result{Stdout: standardOutput.String(), Stderr: standardError.String(), ExitCode: exitCode(waitErr)}
		if parseErr := parseEnvelope(result.Stdout, request.Command, &result.Envelope); parseErr != nil {
			if waitErr != nil {
				return result, NewExitCodeError(request.Command, result.ExitCode, fmt.Errorf("failed without a valid envelope: %w: %s", waitErr, parseErr))
			}
			return result, fmt.Errorf("Craftmake %s returned invalid JSON envelope: %w", request.Command, parseErr)
		}
		if waitErr != nil || result.Failed() {
			return result, NewExitCodeError(request.Command, result.ExitCode, waitErr)
		}
		return result, nil
	}
}

func parseEnvelope(output string, expectedCommand Command, envelope *Envelope) error {
	trimmedOutput := strings.TrimSpace(output)
	if trimmedOutput == "" {
		return errors.New("empty stdout")
	}
	if err := json.Unmarshal([]byte(trimmedOutput), envelope); err != nil {
		return err
	}
	if envelope.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported protocol version %q", envelope.ProtocolVersion)
	}
	if envelope.Command == "" {
		return errors.New("missing command")
	}
	if envelope.Command != string(expectedCommand) {
		return fmt.Errorf("unexpected command %q, expected %q", envelope.Command, expectedCommand)
	}
	return nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return 1
}

func ResolveBinary(configuredPath string) (string, error) {
	if strings.TrimSpace(configuredPath) != "" {
		fileInfo, err := os.Stat(configuredPath)
		if err != nil {
			return "", fmt.Errorf("inspect Craftmake binary %q: %w", configuredPath, err)
		}
		if fileInfo.IsDir() {
			return "", fmt.Errorf("Craftmake binary path %q is a directory", configuredPath)
		}
		return configuredPath, nil
	}
	resolvedPath, err := exec.LookPath("craftmake")
	if err != nil {
		return "", fmt.Errorf("resolve Craftmake binary: %w", err)
	}
	return resolvedPath, nil
}
