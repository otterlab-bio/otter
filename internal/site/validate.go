package site

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

func ValidateSlurmPartition(partition string) error {
	if strings.TrimSpace(partition) == "" {
		return fmt.Errorf("partition name cannot be empty")
	}
	cmd := exec.Command("sinfo", "-h", "-p", partition)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("partition %q is not accessible: %w", partition, err)
	}
	if len(strings.TrimSpace(string(output))) == 0 {
		return fmt.Errorf("partition %q does not exist", partition)
	}
	return nil
}

func ValidateSlurmAccount(account string) error {
	if strings.TrimSpace(account) == "" {
		return fmt.Errorf("account name cannot be empty")
	}
	cmd := exec.Command(
		"sacctmgr", "-s", "-n", "-P",
		"show", "assoc", "where", "account="+account,
		"format=Account",
	)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("account %q validation failed: %w", account, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		associatedAccount := strings.TrimSpace(strings.Split(line, "|")[0])
		if associatedAccount == account {
			return nil
		}
	}
	return fmt.Errorf("account %q does not exist", account)
}

func ValidateSlurmAccountPartition(account string, partition string) error {
	if strings.TrimSpace(account) == "" || strings.TrimSpace(partition) == "" {
		return fmt.Errorf("account and partition must both be provided")
	}
	cmd := exec.Command(
		"sacctmgr", "-s", "-n", "-P",
		"show", "assoc", "where", "account="+account,
		"format=Account,Partition",
	)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("account-partition association check failed: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Split(line, "|")
		if len(fields) < 2 || strings.TrimSpace(fields[0]) != account {
			continue
		}
		associatedPartition := strings.TrimSpace(fields[1])
		if associatedPartition == "" || associatedPartition == partition {
			return nil
		}
	}
	return fmt.Errorf("account %q cannot access partition %q", account, partition)
}

func ValidateSlurmQOS(qos string) error {
	if strings.TrimSpace(qos) == "" {
		return fmt.Errorf("qos name cannot be empty")
	}
	cmd := exec.Command(
		"sacctmgr", "-s", "-n", "-P",
		"show", "qos", "where", "name="+qos,
		"format=Name",
	)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("qos %q validation failed: %w", qos, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(strings.Split(line, "|")[0]) == qos {
			return nil
		}
	}
	return fmt.Errorf("qos %q does not exist", qos)
}

var slurmTimePattern = regexp.MustCompile(`^(?:(\d+)-)?(\d{1,2}):(\d{2}):(\d{2})$`)

func ValidateSlurmTimeFormat(timeSpec string) error {
	if strings.TrimSpace(timeSpec) == "" {
		return nil
	}
	if !slurmTimePattern.MatchString(timeSpec) {
		return fmt.Errorf("invalid SLURM time format %q; expected D-HH:MM:SS or HH:MM:SS", timeSpec)
	}
	return nil
}

func ValidateSlurmMaxJobs(maxJobs int) error {
	if maxJobs < 0 {
		return fmt.Errorf("max_jobs cannot be negative, got %d", maxJobs)
	}
	if maxJobs == 0 {
		return nil
	}
	return nil
}

func CheckLoginNodePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if err := exec.Command("test", "-d", path).Run(); err != nil {
		return fmt.Errorf("path %q is not a directory on the login node", path)
	}
	if err := exec.Command("test", "-r", path).Run(); err != nil {
		return fmt.Errorf("path %q is not readable on the login node", path)
	}
	return nil
}

func CheckLoginNodeWritableDirectory(path string) error {
	if err := CheckLoginNodePath(path); err != nil {
		return err
	}
	if err := exec.Command("test", "-w", path).Run(); err != nil {
		return fmt.Errorf("path %q is not writable on the login node", path)
	}
	return nil
}

func ValidateComputeNodePath(path string, partition string, account string) error {
	return validateComputeNodeDirectory(path, partition, account, false)
}

func ValidateComputeNodeWritableDirectory(path string, partition string, account string) error {
	return validateComputeNodeDirectory(path, partition, account, true)
}

func validateComputeNodeDirectory(path string, partition string, account string, requireWritable bool) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path cannot be empty")
	}
	checkCommand := `test -d "$1" && test -r "$1"`
	if requireWritable {
		checkCommand += ` && test -w "$1"`
	}
	args := []string{
		"--ntasks=1",
		"--partition=" + partition,
		"--account=" + account,
		"bash", "-c", checkCommand, "--", path,
	}
	cmd := exec.Command("srun", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		access := "accessible"
		if requireWritable {
			access = "readable and writable"
		}
		return fmt.Errorf("path %q is not %s on compute nodes (partition %q, account %q): %w\n%s",
			path, access, partition, account, err, string(output))
	}
	return nil
}
