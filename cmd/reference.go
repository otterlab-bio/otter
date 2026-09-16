package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	refpkg "github.com/otterlab-bio/otter/internal/reference"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const referencePromotionAuditRelativePath = ".otter/reference-promotions.jsonl"

type referencePromotionAuditRecord struct {
	Timestamp    time.Time               `json:"timestamp"`
	Action       string                  `json:"action"`
	RunID        string                  `json:"run_id"`
	LockPath     string                  `json:"lock_path"`
	PreviousLock configv1.ReferencesLock `json:"previous_lock"`
	PromotedLock configv1.ReferencesLock `json:"promoted_lock"`
}

var referenceCmd = &cobra.Command{
	Use:   "reference",
	Short: "Manage reference registry assets",
}

var referenceBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build and atomically publish an immutable reference release",
	Long: `Copy a FASTA and GTF into a staged reference release, generate its FAI,
build real Bismark, Bowtie2, and STAR indexes by default, then write
reference.yaml, manifest.json, and checksums.sha256 before atomically publishing.

The default registry root is $OTTER_REFERENCE_ROOT when set, otherwise
~/.otter/references. A release that already exists is never overwritten.`,
	Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, args []string) error {
		return runReferenceBuild(command)
	},
}

var referencePromoteCmd = &cobra.Command{
	Use:   "promote <run-yaml-path>",
	Short: "Promote a run's effective reference selection to the project lock",
	Long: `Read a resolved run snapshot and promote the effective reference selection
into the project's references.lock.yaml.

Promotion is atomic: the lock file is written via a temp file + rename.
The run ID is recorded as promoted_from_run_id for audit.`,
	Args: cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		confirm, _ := command.Flags().GetBool("confirm")
		return runReferencePromote(command, args[0], confirm)
	},
}

func init() {
	referenceBuildCmd.Flags().String("registry-root", "", "Registry root (default: $OTTER_REFERENCE_ROOT or ~/.otter/references)")
	referenceBuildCmd.Flags().String("id", "", "Stable reference identifier, for example hg38")
	referenceBuildCmd.Flags().String("release", "", "Immutable reference release, for example GRCh38.p14")
	referenceBuildCmd.Flags().String("organism", "", "Scientific organism name")
	referenceBuildCmd.Flags().String("assembly", "", "Assembly name")
	referenceBuildCmd.Flags().String("fasta", "", "Source FASTA file")
	referenceBuildCmd.Flags().String("gtf", "", "Source GTF file")
	referenceBuildCmd.Flags().StringSlice("alias", nil, "Optional reference alias; may be repeated")
	referenceBuildCmd.Flags().StringSlice("indexes", nil, "Indexes to build: bismark,bowtie2,star (default: all)")
	referenceBuildCmd.Flags().StringSlice("scenario", nil, "Supported scenario; defaults from selected indexes")
	referenceBuildCmd.Flags().String("samtools", "samtools", "samtools executable used for FASTA indexing")
	referenceBuildCmd.Flags().String("bismark-genome-preparation", "bismark_genome_preparation", "Bismark genome-preparation executable")
	referenceBuildCmd.Flags().String("bowtie2-build", "bowtie2-build", "bowtie2-build executable")
	referenceBuildCmd.Flags().String("star", "STAR", "STAR executable")
	referenceBuildCmd.Flags().Int("star-sjdb-overhang", 149, "STAR sjdbOverhang used for genome generation")
	referenceBuildCmd.Flags().Int("index-build-threads", 1, "Threads for Bowtie2 and STAR; Bismark uses half per CT/GA indexer")

	referencePromoteCmd.Flags().Bool("confirm", false, "Apply the promotion (default is preview-only)")
	rootCmd.AddCommand(referenceCmd)
	referenceCmd.AddCommand(referenceBuildCmd)
	referenceCmd.AddCommand(referencePromoteCmd)
}

func runReferenceBuild(command *cobra.Command) error {
	registryRoot, _ := command.Flags().GetString("registry-root")
	referenceID, _ := command.Flags().GetString("id")
	release, _ := command.Flags().GetString("release")
	organism, _ := command.Flags().GetString("organism")
	assembly, _ := command.Flags().GetString("assembly")
	sourceFastaPath, _ := command.Flags().GetString("fasta")
	sourceGTFPath, _ := command.Flags().GetString("gtf")
	aliases, _ := command.Flags().GetStringSlice("alias")
	indexes, _ := command.Flags().GetStringSlice("indexes")
	scenarioValues, _ := command.Flags().GetStringSlice("scenario")
	scenarios := make([]configv1.Scenario, 0, len(scenarioValues))
	for _, scenarioValue := range scenarioValues {
		scenarios = append(scenarios, configv1.Scenario(scenarioValue))
	}
	samtoolsBinary, _ := command.Flags().GetString("samtools")
	bismarkBinary, _ := command.Flags().GetString("bismark-genome-preparation")
	bowtie2Binary, _ := command.Flags().GetString("bowtie2-build")
	starBinary, _ := command.Flags().GetString("star")
	starSJDBOverhang, _ := command.Flags().GetInt("star-sjdb-overhang")
	indexBuildThreads, _ := command.Flags().GetInt("index-build-threads")

	result, err := refpkg.BuildRelease(refpkg.BuildRequest{
		Context:           command.Context(),
		RegistryRoot:      registryRoot,
		ReferenceID:       referenceID,
		Release:           release,
		Organism:          organism,
		Assembly:          assembly,
		Aliases:           aliases,
		SourceFastaPath:   sourceFastaPath,
		SourceGTFPath:     sourceGTFPath,
		Scenarios:         scenarios,
		Indexes:           indexes,
		SamtoolsBinary:    samtoolsBinary,
		BismarkBinary:     bismarkBinary,
		Bowtie2Binary:     bowtie2Binary,
		STARBinary:        starBinary,
		STARSJDBOverhang:  starSJDBOverhang,
		IndexBuildThreads: indexBuildThreads,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(command.OutOrStdout(), "Reference release published: %s\n", result.ReleaseRoot)
	fmt.Fprintf(command.OutOrStdout(), "reference.yaml: %s\n", result.DefinitionPath)
	fmt.Fprintf(command.OutOrStdout(), "manifest.json: %s\n", result.ManifestPath)
	fmt.Fprintf(command.OutOrStdout(), "manifest digest: %s\n", result.ManifestDigest)
	return nil
}

func runReferencePromote(command *cobra.Command, runYamlPath string, confirm bool) error {
	runSnapshot, err := loadRunSnapshot(runYamlPath)
	if err != nil {
		return err
	}
	absoluteRunPath, err := filepath.Abs(runYamlPath)
	if err != nil {
		return fmt.Errorf("resolve run snapshot path: %w", err)
	}
	expectedRunPath := filepath.Join(runSnapshot.Paths.RunRoot, "run.yaml")
	if filepath.Clean(absoluteRunPath) != filepath.Clean(expectedRunPath) {
		return fmt.Errorf("run snapshot path %q does not match immutable paths.run_root %q", absoluteRunPath, runSnapshot.Paths.RunRoot)
	}
	lockPath := filepath.Join(runSnapshot.Project.Root, "references.lock.yaml")
	currentLock, err := configv1.LoadReferencesLock(lockPath)
	if err != nil {
		return fmt.Errorf("load current lock %q: %w", lockPath, err)
	}

	proposedLock := configv1.ReferencesLock{
		SchemaVersion:     currentLock.SchemaVersion,
		References:        make(map[string]configv1.LockedReference),
		PromotedFromRunID: runSnapshot.Run.ID,
	}
	for role, locked := range currentLock.References {
		proposedLock.References[role] = locked
	}
	for _, role := range []string{"primary", "secondary", "graft", "host"} {
		effectiveSel := selectionByRole(runSnapshot.References.EffectiveSelection, role)
		if effectiveSel == "" {
			continue
		}
		id, release, parseErr := configv1.ParseReferenceSelection(effectiveSel)
		if parseErr != nil {
			continue
		}
		resolvedRef := resolvedReferenceForRole(runSnapshot.References.Resolved, configv1.ReferenceRole(role))
		if resolvedRef == nil {
			continue
		}
		proposedLock.References[role] = configv1.LockedReference{
			ID:             id,
			Release:        release,
			ManifestDigest: resolvedRef.ManifestDigest,
		}
	}

	changes := buildLockDiff(&currentLock, runSnapshot)
	if len(changes) == 0 {
		fmt.Fprintln(command.OutOrStdout(), "No reference changes to promote.")
		return nil
	}

	if err := verifyResolvedReferences(runSnapshot); err != nil {
		return fmt.Errorf("reference verification failed before promotion: %w", err)
	}

	if !confirm {
		fmt.Fprintln(command.OutOrStdout(), "Preview: the following lock changes would be applied:")
		for _, change := range changes {
			fmt.Fprintf(command.OutOrStdout(), "  %s\n", change)
		}
		fmt.Fprintf(command.OutOrStdout(), "\nRun ID: %s\n", runSnapshot.Run.ID)
		fmt.Fprintln(command.OutOrStdout(), "\nUse --confirm to apply.")
		return nil
	}

	if err := configv1.ValidateReferencesLock(proposedLock); err != nil {
		return fmt.Errorf("validate proposed reference lock: %w", err)
	}
	lockData, err := yaml.Marshal(proposedLock)
	if err != nil {
		return fmt.Errorf("marshal proposed lock: %w", err)
	}
	if err := writeAtomicLock(lockPath, lockData); err != nil {
		return err
	}
	if err := appendReferencePromotionAudit(runSnapshot.Project.Root, lockPath, currentLock, proposedLock, runSnapshot.Run.ID); err != nil {
		return fmt.Errorf("reference lock was updated but promotion audit could not be persisted: %w", err)
	}
	fmt.Fprintln(command.OutOrStdout(), "Lock updated successfully.")
	fmt.Fprintf(command.OutOrStdout(), "Promoted from run: %s\n", runSnapshot.Run.ID)
	return nil
}

func appendReferencePromotionAudit(projectRoot, lockPath string, previousLock, promotedLock configv1.ReferencesLock, runID string) error {
	auditRecord := referencePromotionAuditRecord{
		Timestamp:    time.Now().UTC(),
		Action:       "reference_promote",
		RunID:        runID,
		LockPath:     lockPath,
		PreviousLock: previousLock,
		PromotedLock: promotedLock,
	}
	encodedRecord, err := json.Marshal(auditRecord)
	if err != nil {
		return fmt.Errorf("encode promotion audit: %w", err)
	}
	auditPath := filepath.Join(projectRoot, referencePromotionAuditRelativePath)
	if err := os.MkdirAll(filepath.Dir(auditPath), 0o755); err != nil {
		return fmt.Errorf("create promotion audit directory: %w", err)
	}
	auditFile, err := os.OpenFile(auditPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open promotion audit %q: %w", auditPath, err)
	}
	defer auditFile.Close()
	if _, err := auditFile.Write(append(encodedRecord, '\n')); err != nil {
		return fmt.Errorf("append promotion audit %q: %w", auditPath, err)
	}
	if err := auditFile.Sync(); err != nil {
		return fmt.Errorf("sync promotion audit %q: %w", auditPath, err)
	}
	return nil
}

func writeAtomicLock(path string, content []byte) error {
	temporaryFile, err := os.CreateTemp(filepath.Dir(path), ".references-lock-*")
	if err != nil {
		return fmt.Errorf("create temporary lock: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	if err := temporaryFile.Chmod(0o644); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("set temporary lock permissions: %w", err)
	}
	if _, err := temporaryFile.Write(content); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("write temporary lock: %w", err)
	}
	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("sync temporary lock: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("close temporary lock: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("atomically update lock %q: %w", path, err)
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("open lock directory: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync lock directory: %w", err)
	}
	return nil
}

func loadRunSnapshot(path string) (configv1.RunSnapshot, error) {
	return configv1.LoadRunSnapshot(path)
}

func buildLockDiff(currentLock *configv1.ReferencesLock, snapshot configv1.RunSnapshot) []string {
	var changes []string
	for _, role := range []string{"primary", "secondary", "graft", "host"} {
		effectiveSel := string(selectionByRole(snapshot.References.EffectiveSelection, role))
		projectSel := string(selectionByRole(snapshot.References.ProjectSelection, role))
		if effectiveSel == "" {
			continue
		}
		currentLocked, hasLock := currentLock.References[role]
		currentSel := ""
		if hasLock {
			currentSel = currentLocked.ID + "@" + currentLocked.Release
		}
		if projectSel != "" && effectiveSel != projectSel {
			changes = append(changes, fmt.Sprintf("~ %s: %s -> %s", role, projectSel, effectiveSel))
			continue
		}
		if effectiveSel != currentSel {
			resolvedRef := resolvedReferenceForRole(snapshot.References.Resolved, configv1.ReferenceRole(role))
			newLabel := effectiveSel
			if resolvedRef != nil {
				newLabel = fmt.Sprintf("%s (digest: %s...)", effectiveSel, resolvedRef.ManifestDigest[:16])
			}
			changes = append(changes, fmt.Sprintf("~ %s: %s -> %s", role, currentSel, newLabel))
		}
	}
	sort.Strings(changes)
	return changes
}

func selectionByRole(selections configv1.ReferenceSelections, role string) configv1.ReferenceSelection {
	switch role {
	case "primary":
		return selections.Primary
	case "secondary":
		return selections.Secondary
	case "graft":
		return selections.Graft
	case "host":
		return selections.Host
	}
	return ""
}

func resolvedReferenceForRole(references []configv1.ResolvedReference, role configv1.ReferenceRole) *configv1.ResolvedReference {
	for i := range references {
		if references[i].Role == role {
			return &references[i]
		}
	}
	return nil
}

func verifyResolvedReferences(snapshot configv1.RunSnapshot) error {
	for _, ref := range snapshot.References.Resolved {
		report, err := refpkg.VerifyRelease(ref.RegistryRoot, ref.ManifestDigest)
		if err != nil {
			return fmt.Errorf("verify %s (%s@%s): %w", ref.Role, ref.ID, ref.Release, err)
		}
		if !report.Passed {
			for _, issue := range report.Issues {
				fmt.Fprintf(os.Stderr, "  %s: %s (expected: %s, got: %s)\n", issue.Asset, issue.Path, issue.Expected, issue.Actual)
			}
			return fmt.Errorf("checksum verification failed for %s (%s@%s)", ref.Role, ref.ID, ref.Release)
		}
		fmt.Fprintf(os.Stderr, "  verified: %s (%s@%s)\n", ref.Role, ref.ID, ref.Release)
	}
	return nil
}
