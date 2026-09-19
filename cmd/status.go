package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/otterlab-bio/otter/internal/workflow"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [project-dir]",
	Short: "Show workflow status",
	Long: `Display the status of a running or completed workflow.

If no directory is specified, checks the current directory.
The status is read from the .otter_state.json file.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Determine directory to check
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory not found: %s", dir)
	}

	// Create state manager
	state := workflow.NewState(dir, "")

	// Check if state file exists
	if !state.Exists() {
		fmt.Println("No workflow state found")
		fmt.Println("Run 'otter run' to start a new workflow")
		return nil
	}

	// Load state
	if err := state.Load(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Print status
	printWorkflowStatus(state)

	return nil
}

func printWorkflowStatus(state *workflow.State) {
	printWorkflowStatusTo(os.Stdout, state)
}

func printWorkflowStatusTo(output io.Writer, state *workflow.State) {
	data := state.GetData()

	// Calculate duration
	var duration time.Duration
	if !data.LastUpdate.IsZero() && !data.StartTime.IsZero() {
		duration = data.LastUpdate.Sub(data.StartTime)
	}

	fmt.Fprintln(output)
	fmt.Fprintln(output, "Workflow Status")
	fmt.Fprintln(output, "================")
	fmt.Fprintf(output, "Job ID:     %s\n", data.JobID)
	fmt.Fprintf(output, "Status:     %s\n", formatStatus(data.Status))
	fmt.Fprintf(output, "Started:    %s\n", data.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(output, "Last Update: %s\n", data.LastUpdate.Format("2006-01-02 15:04:05"))
	if duration > 0 {
		fmt.Fprintf(output, "Duration:   %s\n", formatDuration(duration))
	}
	fmt.Fprintln(output)

	fmt.Fprintln(output, "Configuration:")
	fmt.Fprintf(output, "  Mode:      %s\n", data.Config.WorkflowMode)
	fmt.Fprintf(output, "  Species:   %s", data.Config.Species1)
	if data.Config.Species2 != "" {
		fmt.Fprintf(output, " + %s (PDX)", data.Config.Species2)
	}
	fmt.Fprintln(output)
	fmt.Fprintf(output, "  Samples:   %d\n", data.Config.SampleCount)
	fmt.Fprintf(output, "  Engine:    %s\n", data.Config.EngineType)
	if data.Config.Partition != "" {
		fmt.Fprintf(output, "  Partition: %s\n", data.Config.Partition)
	}
	fmt.Fprintln(output)

	fmt.Fprintln(output, "Steps:")
	for _, step := range data.Steps {
		statusIcon := "○"
		switch step.Status {
		case "completed":
			statusIcon = "✓"
		case "running":
			statusIcon = "→"
		case "failed":
			statusIcon = "✗"
		}

		fmt.Fprintf(output, "  %s Step %d (%s): %s", statusIcon, step.Step, step.Name, step.Status)
		if step.Status == "completed" && !step.EndTime.IsZero() && !step.StartTime.IsZero() {
			fmt.Fprintf(output, " [%s]", formatDuration(step.EndTime.Sub(step.StartTime)))
		} else if step.Status == "running" && !step.StartTime.IsZero() {
			fmt.Fprintf(output, " [%s running]", formatDuration(time.Since(step.StartTime)))
		}
		if step.JobID != "" {
			fmt.Fprintf(output, " (Job: %s)", step.JobID)
		}
		fmt.Fprintln(output)
	}
	fmt.Fprintln(output)

	completed, running, pending, failed := 0, 0, 0, 0
	for _, step := range data.Steps {
		switch step.Status {
		case "completed":
			completed++
		case "running":
			running++
		case "pending":
			pending++
		case "failed":
			failed++
		}
	}
	fmt.Fprintf(output, "Progress: %d completed, %d running, %d pending", completed, running, pending)
	if failed > 0 {
		fmt.Fprintf(output, ", %d failed", failed)
	}
	fmt.Fprintln(output)

	if running > 0 {
		for _, step := range data.Steps {
			if step.Status == "running" {
				fmt.Fprintf(output, "\nCurrently running: Step %d (%s)\n", step.Step, step.Name)
				fmt.Fprintln(output, "To resume after interruption, use:")
				fmt.Fprintln(output, "  otter run --config /absolute/project/runs/<run-id>/run.yaml --phase <phase> --resume")
				break
			}
		}
	} else if completed < len(data.Steps) && failed == 0 {
		for _, step := range data.Steps {
			if step.Status == "pending" {
				fmt.Fprintf(output, "\nNext step: Step %d (%s)\n", step.Step, step.Name)
				break
			}
		}
	} else if completed == len(data.Steps) {
		fmt.Fprintln(output, "\n✓ Workflow completed successfully!")
		fmt.Fprintln(output, "State file can be removed:")
		fmt.Fprintf(output, "  rm %s\n", state.GetFilePath())
	}
}

func formatStatus(status string) string {
	switch status {
	case "completed":
		return "Completed"
	case "running":
		return "Running"
	case "failed":
		return "Failed"
	case "pending":
		return "Pending"
	default:
		return status
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	} else {
		days := int(d.Hours() / 24)
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}
