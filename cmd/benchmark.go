package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	runstate "github.com/otterlab-bio/otter/internal/run"
	"github.com/spf13/cobra"
)

const benchmarkEvidenceSchemaVersion = "otter.executor-benchmark-evidence/v1"

type benchmarkCell struct {
	RunID             string                       `json:"run_id"`
	RunSnapshotPath   string                       `json:"run_snapshot_path"`
	RunSnapshotDigest string                       `json:"run_snapshot_digest"`
	Executor          configv1.Executor            `json:"executor"`
	Scenario          configv1.Scenario            `json:"scenario"`
	Toolchain         configv1.Toolchain           `json:"toolchain"`
	Backend           configv1.Backend             `json:"backend"`
	Site              string                       `json:"site"`
	Phase             string                       `json:"phase"`
	Resources         configv1.ResourceSpec        `json:"resources"`
	SamplesDigest     string                       `json:"samples_digest"`
	ReferencesDigest  string                       `json:"references_digest"`
	WorkflowDigest    string                       `json:"workflow_digest"`
	References        []benchmarkReferenceIdentity `json:"references"`
	Jobs              []benchmarkSlurmJob          `json:"jobs"`
	Metrics           benchmarkAggregateMetrics    `json:"metrics"`
}

type benchmarkReferenceIdentity struct {
	Role           configv1.ReferenceRole `json:"role"`
	ID             string                 `json:"id"`
	Release        string                 `json:"release"`
	Organism       string                 `json:"organism"`
	ManifestDigest string                 `json:"manifest_digest"`
}

type benchmarkSlurmJob struct {
	JobID         string `json:"job_id"`
	State         string `json:"state"`
	ExitCode      string `json:"exit_code"`
	AllocationCPU int    `json:"allocation_cpu"`
	RequestedMem  string `json:"requested_memory"`
	ElapsedRawSec int64  `json:"elapsed_raw_seconds"`
	TotalCPU      string `json:"total_cpu"`
	MaxRSS        string `json:"max_rss"`
	MaxDiskRead   string `json:"max_disk_read"`
	MaxDiskWrite  string `json:"max_disk_write"`
}

type benchmarkAggregateMetrics struct {
	WallClockSeconds int64 `json:"wall_clock_seconds"`
	AllocatedCPU     int   `json:"allocated_cpu"`
	JobCount         int   `json:"job_count"`
}

type benchmarkEvidence struct {
	SchemaVersion string        `json:"schema_version"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Comparison    string        `json:"comparison"`
	Phase         string        `json:"phase"`
	Left          benchmarkCell `json:"left"`
	Right         benchmarkCell `json:"right"`
}

var runSacctForBenchmark = runSacctForBenchmarkCommand

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Collect immutable executor benchmark evidence",
}

var benchmarkCollectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Collect Slurm accounting for a matched executor pair",
	RunE:  runBenchmarkCollect,
}

var (
	benchmarkLeftRunPath  string
	benchmarkRightRunPath string
	benchmarkLeftJobs     string
	benchmarkRightJobs    string
	benchmarkPhase        string
	benchmarkOutputPath   string
)

func init() {
	rootCmd.AddCommand(benchmarkCmd)
	benchmarkCmd.AddCommand(benchmarkCollectCmd)
	benchmarkCollectCmd.Flags().StringVar(&benchmarkLeftRunPath, "left-run", "", "Immutable left run.yaml path")
	benchmarkCollectCmd.Flags().StringVar(&benchmarkRightRunPath, "right-run", "", "Immutable right run.yaml path")
	benchmarkCollectCmd.Flags().StringVar(&benchmarkLeftJobs, "left-jobs", "", "Comma-separated completed Slurm job IDs for the left executor")
	benchmarkCollectCmd.Flags().StringVar(&benchmarkRightJobs, "right-jobs", "", "Comma-separated completed Slurm job IDs for the right executor")
	benchmarkCollectCmd.Flags().StringVar(&benchmarkPhase, "phase", "", "Immutable phase name represented by both benchmark cells")
	benchmarkCollectCmd.Flags().StringVar(&benchmarkOutputPath, "output", "", "Create-only JSON evidence output path")
	_ = benchmarkCollectCmd.MarkFlagRequired("left-run")
	_ = benchmarkCollectCmd.MarkFlagRequired("right-run")
	_ = benchmarkCollectCmd.MarkFlagRequired("left-jobs")
	_ = benchmarkCollectCmd.MarkFlagRequired("right-jobs")
	_ = benchmarkCollectCmd.MarkFlagRequired("phase")
	_ = benchmarkCollectCmd.MarkFlagRequired("output")
}

func runBenchmarkCollect(command *cobra.Command, _ []string) error {
	evidence, err := collectExecutorBenchmarkEvidence(
		command.Context(),
		benchmarkLeftRunPath,
		benchmarkRightRunPath,
		benchmarkPhase,
		parseBenchmarkJobIDs(benchmarkLeftJobs),
		parseBenchmarkJobIDs(benchmarkRightJobs),
	)
	if err != nil {
		return err
	}
	if err := writeCreateOnlyBenchmarkEvidence(benchmarkOutputPath, evidence); err != nil {
		return err
	}
	fmt.Fprintln(command.OutOrStdout(), benchmarkOutputPath)
	return nil
}

func collectExecutorBenchmarkEvidence(
	ctx context.Context,
	leftRunPath string,
	rightRunPath string,
	phase string,
	leftJobIDs []string,
	rightJobIDs []string,
) (benchmarkEvidence, error) {
	if strings.TrimSpace(phase) == "" {
		return benchmarkEvidence{}, fmt.Errorf("benchmark phase is required")
	}
	if len(leftJobIDs) == 0 || len(rightJobIDs) == 0 {
		return benchmarkEvidence{}, fmt.Errorf("both benchmark cells require at least one Slurm job ID")
	}

	leftCell, err := loadBenchmarkCell(ctx, leftRunPath, phase, leftJobIDs)
	if err != nil {
		return benchmarkEvidence{}, fmt.Errorf("collect left benchmark cell: %w", err)
	}
	rightCell, err := loadBenchmarkCell(ctx, rightRunPath, phase, rightJobIDs)
	if err != nil {
		return benchmarkEvidence{}, fmt.Errorf("collect right benchmark cell: %w", err)
	}
	if err := validateBenchmarkPair(leftCell, rightCell); err != nil {
		return benchmarkEvidence{}, err
	}

	return benchmarkEvidence{
		SchemaVersion: benchmarkEvidenceSchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Comparison:    "executor-parity",
		Phase:         phase,
		Left:          leftCell,
		Right:         rightCell,
	}, nil
}

func loadBenchmarkCell(ctx context.Context, runPath string, phase string, jobIDs []string) (benchmarkCell, error) {
	absoluteRunPath, err := filepath.Abs(runPath)
	if err != nil {
		return benchmarkCell{}, fmt.Errorf("resolve benchmark run path: %w", err)
	}
	snapshot, err := configv1.LoadRunSnapshot(absoluteRunPath)
	if err != nil {
		return benchmarkCell{}, fmt.Errorf("load immutable run snapshot: %w", err)
	}
	resources, found := snapshot.Execution.Resources.Phases[phase]
	if !found {
		return benchmarkCell{}, fmt.Errorf("run %s has no resource envelope for phase %s", snapshot.Run.ID, phase)
	}
	snapshotDigest, err := runstate.DigestFile(absoluteRunPath)
	if err != nil {
		return benchmarkCell{}, fmt.Errorf("digest immutable run snapshot: %w", err)
	}
	jobs, err := runSacctForBenchmark(ctx, jobIDs)
	if err != nil {
		return benchmarkCell{}, err
	}
	metrics, err := aggregateBenchmarkMetrics(jobs)
	if err != nil {
		return benchmarkCell{}, err
	}

	return benchmarkCell{
		RunID:             snapshot.Run.ID,
		RunSnapshotPath:   absoluteRunPath,
		RunSnapshotDigest: snapshotDigest,
		Executor:          snapshot.Execution.Executor.Value,
		Scenario:          snapshot.Workflow.Scenario,
		Toolchain:         snapshot.Workflow.Toolchain,
		Backend:           snapshot.Execution.Backend.Value,
		Site:              snapshot.Execution.Site.Value,
		Phase:             phase,
		Resources:         resources,
		SamplesDigest:     snapshot.Digests.Samples,
		ReferencesDigest:  snapshot.Digests.References,
		WorkflowDigest:    snapshot.Digests.WorkflowAssets,
		References:        benchmarkReferenceIdentities(snapshot.References.Resolved),
		Jobs:              jobs,
		Metrics:           metrics,
	}, nil
}

func validateBenchmarkPair(leftCell benchmarkCell, rightCell benchmarkCell) error {
	if leftCell.Executor == rightCell.Executor {
		return fmt.Errorf("benchmark executor pair must differ; both cells use %q", leftCell.Executor)
	}
	if leftCell.Scenario != rightCell.Scenario || leftCell.Toolchain != rightCell.Toolchain || leftCell.Backend != rightCell.Backend || leftCell.Site != rightCell.Site {
		return fmt.Errorf("benchmark cells differ in scenario, toolchain, backend, or site")
	}
	if leftCell.Phase != rightCell.Phase || leftCell.Resources != rightCell.Resources {
		return fmt.Errorf("benchmark cells differ in immutable phase resource envelope")
	}
	if leftCell.WorkflowDigest != rightCell.WorkflowDigest {
		return fmt.Errorf("benchmark cells differ in immutable workflow assets")
	}
	if leftCell.SamplesDigest != rightCell.SamplesDigest || leftCell.ReferencesDigest != rightCell.ReferencesDigest {
		return fmt.Errorf("benchmark cells differ in immutable sample or reference identity")
	}
	if !sameBenchmarkReferences(leftCell.References, rightCell.References) {
		return fmt.Errorf("benchmark cells differ in resolved reference identity")
	}
	return nil
}

func benchmarkReferenceIdentities(references []configv1.ResolvedReference) []benchmarkReferenceIdentity {
	identities := make([]benchmarkReferenceIdentity, 0, len(references))
	for _, reference := range references {
		identities = append(identities, benchmarkReferenceIdentity{
			Role:           reference.Role,
			ID:             reference.ID,
			Release:        reference.Release,
			Organism:       reference.Organism,
			ManifestDigest: reference.ManifestDigest,
		})
	}
	return identities
}

func sameBenchmarkReferences(leftReferences []benchmarkReferenceIdentity, rightReferences []benchmarkReferenceIdentity) bool {
	if len(leftReferences) != len(rightReferences) {
		return false
	}
	for referenceIndex := range leftReferences {
		if leftReferences[referenceIndex] != rightReferences[referenceIndex] {
			return false
		}
	}
	return true
}

func parseBenchmarkJobIDs(value string) []string {
	var jobIDs []string
	seenJobIDs := map[string]bool{}
	for _, valuePart := range strings.Split(value, ",") {
		jobID := strings.TrimSpace(valuePart)
		if jobID == "" || seenJobIDs[jobID] {
			continue
		}
		seenJobIDs[jobID] = true
		jobIDs = append(jobIDs, jobID)
	}
	return jobIDs
}

func runSacctForBenchmarkCommand(ctx context.Context, jobIDs []string) ([]benchmarkSlurmJob, error) {
	command := exec.CommandContext(ctx,
		"sacct",
		"-X",
		"-n",
		"-P",
		"-j", strings.Join(jobIDs, ","),
		"--format=JobIDRaw,State,ExitCode,AllocCPUS,ReqMem,ElapsedRaw,TotalCPU,MaxRSS,MaxDiskRead,MaxDiskWrite",
	)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("query Slurm accounting: %w", err)
	}
	jobs, err := parseBenchmarkSacctOutput(string(output))
	if err != nil {
		return nil, err
	}
	if err := validateRequestedBenchmarkJobs(jobIDs, jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func validateRequestedBenchmarkJobs(requestedJobIDs []string, jobs []benchmarkSlurmJob) error {
	if len(jobs) != len(requestedJobIDs) {
		return fmt.Errorf("Slurm accounting returned %d primary jobs for %d requested IDs", len(jobs), len(requestedJobIDs))
	}
	requestedJobIDSet := make(map[string]struct{}, len(requestedJobIDs))
	for _, jobID := range requestedJobIDs {
		requestedJobIDSet[jobID] = struct{}{}
	}
	for _, job := range jobs {
		if _, found := requestedJobIDSet[job.JobID]; !found {
			return fmt.Errorf("Slurm accounting returned unexpected job ID %q", job.JobID)
		}
		delete(requestedJobIDSet, job.JobID)
	}
	if len(requestedJobIDSet) != 0 {
		return fmt.Errorf("Slurm accounting did not return every requested job ID")
	}
	return nil
}

func parseBenchmarkSacctOutput(output string) ([]benchmarkSlurmJob, error) {
	var jobs []benchmarkSlurmJob
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(strings.TrimSuffix(line, "|"), "|")
		const requiredBenchmarkSacctFields = 6
		const benchmarkSacctFields = 10
		if len(fields) < requiredBenchmarkSacctFields || len(fields) > benchmarkSacctFields {
			return nil, fmt.Errorf("unexpected sacct benchmark row %q", line)
		}
		for len(fields) < benchmarkSacctFields {
			fields = append(fields, "")
		}
		allocationCPU, err := strconv.Atoi(strings.TrimSpace(fields[3]))
		if err != nil {
			return nil, fmt.Errorf("parse benchmark AllocCPUS %q: %w", fields[3], err)
		}
		elapsedSeconds, err := strconv.ParseInt(strings.TrimSpace(fields[5]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse benchmark ElapsedRaw %q: %w", fields[5], err)
		}
		job := benchmarkSlurmJob{
			JobID:         strings.TrimSpace(fields[0]),
			State:         strings.TrimSpace(fields[1]),
			ExitCode:      strings.TrimSpace(fields[2]),
			AllocationCPU: allocationCPU,
			RequestedMem:  strings.TrimSpace(fields[4]),
			ElapsedRawSec: elapsedSeconds,
			TotalCPU:      strings.TrimSpace(fields[6]),
			MaxRSS:        strings.TrimSpace(fields[7]),
			MaxDiskRead:   strings.TrimSpace(fields[8]),
			MaxDiskWrite:  strings.TrimSpace(fields[9]),
		}
		if job.JobID == "" || job.State != "COMPLETED" || job.ExitCode != "0:0" {
			return nil, fmt.Errorf("benchmark job %q is not accepted: state=%s exit_code=%s", job.JobID, job.State, job.ExitCode)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func aggregateBenchmarkMetrics(jobs []benchmarkSlurmJob) (benchmarkAggregateMetrics, error) {
	if len(jobs) == 0 {
		return benchmarkAggregateMetrics{}, fmt.Errorf("benchmark metrics require at least one completed Slurm job")
	}
	metrics := benchmarkAggregateMetrics{JobCount: len(jobs)}
	for _, job := range jobs {
		metrics.WallClockSeconds += job.ElapsedRawSec
		metrics.AllocatedCPU += job.AllocationCPU
	}
	return metrics, nil
}

func writeCreateOnlyBenchmarkEvidence(outputPath string, evidence benchmarkEvidence) error {
	absoluteOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve benchmark evidence output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absoluteOutputPath), 0o755); err != nil {
		return fmt.Errorf("create benchmark evidence directory: %w", err)
	}
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return fmt.Errorf("encode benchmark evidence: %w", err)
	}
	file, err := os.OpenFile(absoluteOutputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o444)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("benchmark evidence already exists: %s", absoluteOutputPath)
		}
		return fmt.Errorf("create benchmark evidence: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write benchmark evidence: %w", err)
	}
	return nil
}
