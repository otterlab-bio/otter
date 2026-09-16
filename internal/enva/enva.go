package enva

import (
	"fmt"
	"os/exec"
	"strings"
)

// IsAvailable 检查 enva 是否在 PATH 中
func IsAvailable() bool {
	_, err := exec.LookPath("enva")
	return err == nil
}

// BuildCondaCommand builds a conda/enva command.
// If enva is available, returns "enva run <env> -- <command> <args...>".
// Otherwise returns "conda run --no-capture-output -n <env> <command> <args...>".
func BuildCondaCommand(envName string, command string, args ...string) []string {
	if IsAvailable() {
		result := []string{"enva", "run", envName, "--", command}
		result = append(result, args...)
		return result
	}

	result := []string{"conda", "run", "--no-capture-output", "-n", envName}
	result = append(result, command)
	result = append(result, args...)
	return result
}

// BuildCondaCommandWithFlags is like BuildCondaCommand but supports extra flags after the command.
func BuildCondaCommandWithFlags(envName string, command string, flags []string, args ...string) []string {
	if IsAvailable() {
		result := []string{"enva", "run", envName, "--", command}
		result = append(result, flags...)
		result = append(result, args...)
		return result
	}

	result := []string{"conda", "run", "--no-capture-output", "-n", envName, command}
	result = append(result, flags...)
	result = append(result, args...)
	return result
}

// ValidateEnvironment validates if a conda environment can run snakemake
func ValidateEnvironment(envName string) error {
	if envName == "" {
		// Empty means use system snakemake, just check if snakemake exists
		_, err := exec.LookPath("snakemake")
		if err != nil {
			return fmt.Errorf("snakemake not found in PATH")
		}
		return nil
	}

	// Check if snakemake --version works in the specified environment
	var checkCmd []string
	if IsAvailable() {
		checkCmd = []string{"enva", "run", envName, "--", "snakemake", "--version"}
	} else {
		checkCmd = []string{"conda", "run", "-n", envName, "--no-capture-output", "snakemake", "--version"}
	}

	cmd := exec.Command(checkCmd[0], checkCmd[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("environment '%s' validation failed: %w (output: %s)", envName, err, strings.TrimSpace(string(output)))
	}

	return nil
}
