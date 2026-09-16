package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	craftmakeclient "github.com/otterlab-bio/otter/internal/craftmake"
	taskruntime "github.com/otterlab-bio/otter/internal/task"
	"github.com/otterlab-bio/otter/internal/workflow"
	"github.com/spf13/cobra"
)

var (
	taskListAll       bool
	taskLogsFollow    bool
	taskLogsTail      int
	taskStopTimeout   time.Duration
	taskReportOutput  string
	taskReportRefresh bool
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage background workflow tasks",
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List background workflow tasks",
	RunE:  runTaskList,
}

var taskStatusCmd = &cobra.Command{
	Use:   "status <task-id>",
	Short: "Show a background task and workflow status",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskStatus,
}

var taskLogsCmd = &cobra.Command{
	Use:   "logs <task-id>",
	Short: "Show or follow a background task log",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskLogs,
}

var taskStopCmd = &cobra.Command{
	Use:   "stop <task-id>",
	Short: "Stop a background task and its active jobs",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskStop,
}

var taskReportCmd = &cobra.Command{
	Use:   "report <task-id>",
	Short: "Export a Craftmake task report",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskReport,
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskListCmd, taskStatusCmd, taskLogsCmd, taskStopCmd, taskReportCmd)

	taskListCmd.Flags().BoolVar(&taskListAll, "all", false, "Include completed and failed tasks")
	taskLogsCmd.Flags().BoolVarP(&taskLogsFollow, "follow", "f", false, "Follow new log output until the task finishes")
	taskLogsCmd.Flags().IntVarP(&taskLogsTail, "tail", "n", 100, "Number of existing log lines to show")
	taskStopCmd.Flags().DurationVar(&taskStopTimeout, "timeout", 10*time.Second, "Time to wait before forcing local processes to stop")
	taskReportCmd.Flags().StringVar(&taskReportOutput, "output", "", "Report output directory")
	taskReportCmd.Flags().BoolVar(&taskReportRefresh, "refresh-metrics", false, "Refresh unavailable SLURM metrics before export")
}

func runTaskList(cmd *cobra.Command, args []string) error {
	store, err := taskruntime.DefaultStore()
	if err != nil {
		return err
	}
	records, err := store.List()
	if err != nil {
		return err
	}

	for _, record := range records {
		if err := store.RefreshProcessStatus(record); err != nil {
			return err
		}
	}
	records, err = store.List()
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	defer writer.Flush()
	fmt.Fprintln(writer, "TASK ID\tSTATUS\tENGINE\tAGE\tPROJECT")
	shown := 0
	for _, record := range records {
		if !taskListAll && taskruntime.IsTerminalStatus(record.Status) && len(record.ActiveSlurmJobIDs) == 0 {
			continue
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n",
			record.ID,
			record.Status,
			record.Engine,
			formatTaskAge(record),
			record.ProjectDir,
		)
		shown++
	}
	if shown == 0 {
		if taskListAll {
			fmt.Fprintln(writer, "No background tasks found")
		} else {
			fmt.Fprintln(writer, "No active background tasks found; use --all to include finished tasks")
		}
	}
	return nil
}

func runTaskStatus(cmd *cobra.Command, args []string) error {
	store, err := taskruntime.DefaultStore()
	if err != nil {
		return err
	}
	record, err := store.Load(args[0])
	if err != nil {
		return err
	}
	if err := store.RefreshProcessStatus(record); err != nil {
		return err
	}
	record, err = store.Load(args[0])
	if err != nil {
		return err
	}

	output := cmd.OutOrStdout()
	fmt.Fprintf(output, "Task ID:      %s\n", record.ID)
	fmt.Fprintf(output, "Status:       %s\n", record.Status)
	fmt.Fprintf(output, "Engine:       %s\n", record.Engine)
	fmt.Fprintf(output, "Project:      %s\n", record.ProjectDir)
	fmt.Fprintf(output, "Config:       %s\n", record.ConfigPath)
	fmt.Fprintf(output, "PID:          %d\n", record.PID)
	fmt.Fprintf(output, "Started:      %s\n", formatTaskTime(record.StartedAt))
	fmt.Fprintf(output, "Last update:  %s\n", formatTaskTime(record.LastUpdate))
	fmt.Fprintf(output, "Duration:     %s\n", formatTaskAge(record))
	fmt.Fprintf(output, "Log:          %s\n", record.LogPath)
	if len(record.ActiveSlurmJobIDs) > 0 {
		fmt.Fprintf(output, "Active SLURM: %s\n", strings.Join(record.ActiveSlurmJobIDs, ", "))
	}
	if record.Error != "" {
		fmt.Fprintf(output, "Error:        %s\n", record.Error)
	}
	if record.Message != "" {
		fmt.Fprintf(output, "Message:      %s\n", record.Message)
	}
	if record.CraftmakeRunID != "" {
		result, executeErr := executeTaskCraftmakeCommand(cmd, record, craftmakeclient.CommandStatus)
		if executeErr != nil {
			return executeErr
		}
		fmt.Fprintf(output, "\nCraftmake status:\n%s", result.Stdout)
		return nil
	}

	if record.StatePath == "" {
		return nil
	}
	state := workflow.NewState(filepath.Dir(record.StatePath), "")
	if !state.Exists() {
		fmt.Fprintln(output, "\nWorkflow state has not been created yet.")
		return nil
	}
	if err := state.Load(); err != nil {
		return fmt.Errorf("load workflow state: %w", err)
	}
	printWorkflowStatusTo(output, state)
	return nil
}

func runTaskLogs(cmd *cobra.Command, args []string) error {
	store, err := taskruntime.DefaultStore()
	if err != nil {
		return err
	}
	record, err := store.Load(args[0])
	if err != nil {
		return err
	}
	if taskLogsTail < 0 {
		return fmt.Errorf("--tail must be zero or greater")
	}

	if err := printLastLogLines(cmd.OutOrStdout(), record.LogPath, taskLogsTail); err != nil {
		return err
	}
	if !taskLogsFollow {
		if record.CraftmakeRunID != "" {
			result, executeErr := executeTaskCraftmakeCommand(cmd, record, craftmakeclient.CommandLogs)
			if executeErr != nil {
				return executeErr
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nCraftmake logs:\n%s", result.Stdout)
		}
		return nil
	}
	return followTaskLog(cmd.OutOrStdout(), store, record.ID, record.LogPath)
}

func runTaskReport(cmd *cobra.Command, args []string) error {
	store, err := taskruntime.DefaultStore()
	if err != nil {
		return err
	}
	record, err := store.Load(args[0])
	if err != nil {
		return err
	}
	if record.CraftmakeRunID == "" {
		return fmt.Errorf("task %s is not a Craftmake task", record.ID)
	}
	extraArguments := make([]string, 0, 3)
	if taskReportOutput != "" {
		extraArguments = append(extraArguments, "--output", taskReportOutput)
	}
	if taskReportRefresh {
		extraArguments = append(extraArguments, "--refresh-metrics")
	}
	result, executeErr := executeTaskCraftmakeCommand(cmd, record, craftmakeclient.CommandReport, extraArguments...)
	if executeErr != nil {
		return executeErr
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), result.Stdout)
	return err
}

func runTaskStop(cmd *cobra.Command, args []string) error {
	store, err := taskruntime.DefaultStore()
	if err != nil {
		return err
	}
	record, err := store.Load(args[0])
	if err != nil {
		return err
	}
	if taskruntime.IsTerminalStatus(record.Status) && record.Status != taskruntime.StatusInterrupted && len(record.ActiveSlurmJobIDs) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Task %s is already %s.\n", record.ID, record.Status)
		return nil
	}

	if err := store.Update(record.ID, func(current *taskruntime.Record) error {
		current.Status = taskruntime.StatusStopping
		return nil
	}); err != nil {
		return err
	}

	var stopErrors []error
	failedJobIDs := make([]string, 0)
	if record.CraftmakeRunID != "" {
		result, cancelErr := executeTaskCraftmakeCommand(cmd, record, craftmakeclient.CommandCancel)
		if cancelErr != nil {
			stopErrors = append(stopErrors, cancelErr)
		} else if result.Failed() {
			stopErrors = append(stopErrors, fmt.Errorf("Craftmake cancel failed with exit code %d", result.ExitCode))
		}
	} else {
		for _, jobID := range record.ActiveSlurmJobIDs {
			cancelCommand := exec.Command("scancel", jobID)
			if output, cancelErr := cancelCommand.CombinedOutput(); cancelErr != nil {
				failedJobIDs = append(failedJobIDs, jobID)
				stopErrors = append(stopErrors, fmt.Errorf("cancel SLURM job %s: %w: %s", jobID, cancelErr, strings.TrimSpace(string(output))))
			}
		}
	}

	processGroupID := record.ProcessGroupID
	if processGroupID <= 0 {
		processGroupID = record.PID
	}
	if processGroupID > 0 && taskruntime.ProcessExists(record.PID) {
		if signalErr := syscall.Kill(-processGroupID, syscall.SIGTERM); signalErr != nil && !errors.Is(signalErr, syscall.ESRCH) {
			stopErrors = append(stopErrors, fmt.Errorf("signal task process group: %w", signalErr))
		}
		deadline := time.Now().Add(taskStopTimeout)
		for taskruntime.ProcessExists(record.PID) && time.Now().Before(deadline) {
			time.Sleep(200 * time.Millisecond)
		}
		if taskruntime.ProcessExists(record.PID) {
			if signalErr := syscall.Kill(-processGroupID, syscall.SIGKILL); signalErr != nil && !errors.Is(signalErr, syscall.ESRCH) {
				stopErrors = append(stopErrors, fmt.Errorf("force task process group to stop: %w", signalErr))
			}
		}
	}

	exitCode := 143
	if err := store.Update(record.ID, func(current *taskruntime.Record) error {
		current.Status = taskruntime.StatusStopped
		current.FinishedAt = time.Now()
		current.ExitCode = &exitCode
		current.ActiveSlurmJobIDs = failedJobIDs
		if len(stopErrors) > 0 {
			current.Error = errors.Join(stopErrors...).Error()
		} else {
			current.Error = ""
		}
		return nil
	}); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Task %s stopped.\n", record.ID)
	return errors.Join(stopErrors...)
}

func executeTaskCraftmakeCommand(command *cobra.Command, record *taskruntime.Record, requestedCommand craftmakeclient.Command, extraArguments ...string) (craftmakeclient.Result, error) {
	if record.StatePath == "" || record.CraftmakeRunID == "" {
		return craftmakeclient.Result{}, fmt.Errorf("task %s has incomplete Craftmake state correlation", record.ID)
	}
	binaryPath, err := craftmakeclient.ResolveBinary(record.CraftmakeBinary)
	if err != nil {
		return craftmakeclient.Result{}, err
	}
	arguments := []string{
		"--state", record.StatePath,
		"--run", record.CraftmakeRunID,
		"--format", "json",
	}
	arguments = append(arguments, extraArguments...)
	result, err := craftmakeclient.Execute(command.Context(), craftmakeclient.Request{
		Binary:    binaryPath,
		Command:   requestedCommand,
		Arguments: arguments,
	})
	if result.Stderr != "" {
		fmt.Fprint(command.ErrOrStderr(), result.Stderr)
	}
	return result, err
}

func printLastLogLines(output io.Writer, logPath string, lineCount int) error {
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("task log does not exist yet: %s", logPath)
		}
		return fmt.Errorf("open task log: %w", err)
	}
	defer file.Close()

	lines := make([]string, 0, lineCount)
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 4*1024*1024)
	for scanner.Scan() {
		if lineCount == 0 {
			continue
		}
		if len(lines) == lineCount {
			copy(lines, lines[1:])
			lines[len(lines)-1] = scanner.Text()
		} else {
			lines = append(lines, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read task log: %w", err)
	}
	for _, line := range lines {
		fmt.Fprintln(output, line)
	}
	return nil
}

func followTaskLog(output io.Writer, store *taskruntime.Store, taskID, logPath string) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("open task log: %w", err)
	}
	defer file.Close()
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek task log: %w", err)
	}

	reader := bufio.NewReader(file)
	idleAfterCompletion := 0
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			fmt.Fprint(output, line)
			idleAfterCompletion = 0
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("follow task log: %w", readErr)
		}
		if errors.Is(readErr, io.EOF) {
			record, loadErr := store.Load(taskID)
			if loadErr != nil {
				return loadErr
			}
			if taskruntime.IsTerminalStatus(record.Status) {
				idleAfterCompletion++
				if idleAfterCompletion >= 2 {
					return nil
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func formatTaskAge(record *taskruntime.Record) string {
	startTime := record.StartedAt
	if startTime.IsZero() {
		startTime = record.CreatedAt
	}
	endTime := time.Now()
	if !record.FinishedAt.IsZero() {
		endTime = record.FinishedAt
	}
	if startTime.IsZero() || endTime.Before(startTime) {
		return "-"
	}
	return formatDuration(endTime.Sub(startTime))
}

func formatTaskTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04:05")
}
