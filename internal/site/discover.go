package site

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultReferenceRootEnvironmentVariable is the environment variable that
// overrides the registry root, shared with the reference package.
const DefaultReferenceRootEnvironmentVariable = "OTTER_REFERENCE_ROOT"

// DefaultScratchRootEnvironmentVariable overrides the scratch root candidate.
const DefaultScratchRootEnvironmentVariable = "OTTER_SCRATCH_ROOT"

// Discovery records what the local environment can actually be probed for.
//
// It deliberately separates observable facts from site policy: the toolchain,
// cluster name, and partitions can be read off the machine, while the account,
// QOS, and the correct shared reference root are decisions the operator makes.
// "otter site generate" therefore emits observed values and refuses to invent
// the rest.
type Discovery struct {
	// SlurmToolsPresent lists which of sbatch/squeue/sacct/scancel were found.
	SlurmToolsPresent []string
	// SlurmToolchainComplete is true only when all four were found.
	SlurmToolchainComplete bool
	// ClusterName is read from `scontrol show config` when available.
	ClusterName string
	// Partitions lists partition names offered by the cluster.
	Partitions []string
	// ReferenceRootCandidate is the registry root implied by the environment.
	ReferenceRootCandidate string
	// ScratchRootCandidate is the scratch root implied by the environment.
	ScratchRootCandidate string
}

// DiscoverProbes lets tests replace the environment probes.
type DiscoverProbes struct {
	ListPartitions func() ([]string, error)
	Environment    func(string) string
	UserHomeDir    func() (string, error)
}

// Discover probes the current environment.
func (detector *Detector) Discover(probes DiscoverProbes) Discovery {
	if probes.Environment == nil {
		probes.Environment = os.Getenv
	}
	if probes.UserHomeDir == nil {
		probes.UserHomeDir = os.UserHomeDir
	}
	if probes.ListPartitions == nil {
		probes.ListPartitions = listSlurmPartitions
	}

	available := detector.availableSlurmTools()
	discovery := Discovery{
		SlurmToolsPresent:      available,
		SlurmToolchainComplete: len(available) == slurmToolchainToolCount,
	}
	if discovery.SlurmToolchainComplete {
		if clusterName, found := detector.getClusterName(); found {
			discovery.ClusterName = clusterName
		}
		if partitions, err := probes.ListPartitions(); err == nil {
			discovery.Partitions = partitions
		}
	}
	discovery.ReferenceRootCandidate = referenceRootCandidate(probes)
	discovery.ScratchRootCandidate = strings.TrimSpace(probes.Environment(DefaultScratchRootEnvironmentVariable))
	return discovery
}

// referenceRootCandidate mirrors the reference package default: the environment
// variable when set, otherwise ~/.otter/references.
func referenceRootCandidate(probes DiscoverProbes) string {
	if configured := strings.TrimSpace(probes.Environment(DefaultReferenceRootEnvironmentVariable)); configured != "" {
		if absolute, err := filepath.Abs(configured); err == nil {
			return absolute
		}
		return configured
	}
	home, err := probes.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".otter", "references")
}

// listSlurmPartitions reads the partition names the cluster advertises.
func listSlurmPartitions() ([]string, error) {
	output, err := exec.Command("sinfo", "-h", "-o", "%P").Output()
	if err != nil {
		return nil, fmt.Errorf("list partitions with sinfo: %w", err)
	}
	seen := make(map[string]bool)
	partitions := make([]string, 0)
	for _, line := range strings.Split(string(output), "\n") {
		name := strings.TrimSuffix(strings.TrimSpace(line), "*")
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		partitions = append(partitions, name)
	}
	sort.Strings(partitions)
	return partitions, nil
}

// GenerateOptions carries the site policy a human must decide.
type GenerateOptions struct {
	SiteID        string
	Backend       string
	Partition     string
	Account       string
	QOS           string
	MaxJobs       int
	DefaultTime   string
	ReferenceRoot string
	ScratchRoot   string
}

// BuildProfile turns discovery plus explicit policy into a validated profile.
//
// The backend resolves as follows: an explicit local/slurm wins; "auto" or empty
// selects slurm only when the toolchain is complete AND both partition and
// account were supplied, because a slurm profile without them cannot pass the
// cluster probes.
func BuildProfile(discovery Discovery, options GenerateOptions) (*SiteProfile, error) {
	siteID := strings.TrimSpace(options.SiteID)
	if siteID == "" {
		return nil, fmt.Errorf("--id is required")
	}
	if strings.ContainsAny(siteID, `/\`) || siteID == "." || siteID == ".." {
		return nil, fmt.Errorf("--id %q must be a plain name without path separators", siteID)
	}

	backend := strings.ToLower(strings.TrimSpace(options.Backend))
	switch backend {
	case "", "auto":
		if discovery.SlurmToolchainComplete && strings.TrimSpace(options.Partition) != "" && strings.TrimSpace(options.Account) != "" {
			backend = "slurm"
		} else {
			backend = "local"
		}
	case "local", "slurm":
	default:
		return nil, fmt.Errorf("--backend %q is unsupported; expected auto, local, or slurm", options.Backend)
	}

	referenceRoot := strings.TrimSpace(options.ReferenceRoot)
	if referenceRoot == "" {
		referenceRoot = discovery.ReferenceRootCandidate
	}
	if referenceRoot != "" && !filepath.IsAbs(referenceRoot) {
		return nil, fmt.Errorf("--reference-root %q must be an absolute path", referenceRoot)
	}
	scratchRoot := strings.TrimSpace(options.ScratchRoot)
	if scratchRoot == "" {
		scratchRoot = discovery.ScratchRootCandidate
	}
	if scratchRoot != "" && !filepath.IsAbs(scratchRoot) {
		return nil, fmt.Errorf("--scratch-root %q must be an absolute path", scratchRoot)
	}

	profile := &SiteProfile{
		SchemaVersion: profileSchemaVersion,
		Site:          SiteIdentity{ID: siteID, Backend: backend},
		Paths:         SitePaths{ReferenceRoot: referenceRoot, ScratchRoot: scratchRoot},
	}
	if backend == "slurm" {
		if strings.TrimSpace(options.Partition) == "" {
			return nil, slurmFieldRequired("--partition", discovery)
		}
		if strings.TrimSpace(options.Account) == "" {
			return nil, slurmFieldRequired("--account", discovery)
		}
		profile.Slurm = &SlurmConfig{
			Partition:   strings.TrimSpace(options.Partition),
			Account:     strings.TrimSpace(options.Account),
			QOS:         strings.TrimSpace(options.QOS),
			MaxJobs:     options.MaxJobs,
			DefaultTime: strings.TrimSpace(options.DefaultTime),
		}
		if referenceRoot == "" {
			return nil, fmt.Errorf("a slurm profile requires --reference-root (or $%s)", DefaultReferenceRootEnvironmentVariable)
		}
	}

	if err := ValidateProfile(profile); err != nil {
		return nil, fmt.Errorf("generated site profile is invalid: %w", err)
	}
	return profile, nil
}

// slurmFieldRequired reports a missing slurm field and names the candidates
// discovery observed, so the operator does not have to go and look.
func slurmFieldRequired(flagName string, discovery Discovery) error {
	hint := ""
	if flagName == "--partition" && len(discovery.Partitions) > 0 {
		hint = fmt.Sprintf("; observed partitions: %s", strings.Join(discovery.Partitions, ", "))
	}
	return fmt.Errorf(
		"a slurm profile requires %s%s.\nDiscovery cannot decide this for you; pass %s explicitly, or use --backend local for a contract-only profile",
		flagName, hint, flagName)
}

// DefaultGeneratedProfilePath returns the user-scoped path a generated profile
// lands in when --output is omitted.
func DefaultGeneratedProfilePath(siteID string) string {
	return filepath.Join(DefaultLocator().UserConfigDir, siteID+".yaml")
}

// DiscoverySummary renders the observed facts for human output.
func DiscoverySummary(discovery Discovery) []string {
	lines := []string{
		fmt.Sprintf("slurm toolchain: %s", describeToolchain(discovery)),
	}
	if discovery.ClusterName != "" {
		lines = append(lines, fmt.Sprintf("cluster: %s", discovery.ClusterName))
	}
	if len(discovery.Partitions) > 0 {
		lines = append(lines, fmt.Sprintf("observed partitions: %s", strings.Join(discovery.Partitions, ", ")))
	}
	if discovery.ReferenceRootCandidate != "" {
		lines = append(lines, fmt.Sprintf("reference root candidate: %s", discovery.ReferenceRootCandidate))
	}
	if discovery.ScratchRootCandidate != "" {
		lines = append(lines, fmt.Sprintf("scratch root candidate: %s", discovery.ScratchRootCandidate))
	}
	return lines
}

func describeToolchain(discovery Discovery) string {
	if discovery.SlurmToolchainComplete {
		return fmt.Sprintf("complete (%s)", strings.Join(discovery.SlurmToolsPresent, ", "))
	}
	if len(discovery.SlurmToolsPresent) == 0 {
		return "absent; the generated profile will use the local backend"
	}
	return fmt.Sprintf("partial (%s); the generated profile will use the local backend",
		strings.Join(discovery.SlurmToolsPresent, ", "))
}
