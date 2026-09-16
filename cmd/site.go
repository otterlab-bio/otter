package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/otter/internal/site"
	"github.com/spf13/cobra"
)

var siteCmd = &cobra.Command{
	Use:   "site",
	Short: "Manage site profiles for backend auto-detection",
}

var siteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered site profiles",
	RunE: func(command *cobra.Command, args []string) error {
		locator := site.DefaultLocator()
		profiles, err := locator.List()
		if err != nil {
			return err
		}
		for _, id := range profiles {
			profile, err := locator.Find(id)
			if err != nil {
				fmt.Fprintf(command.OutOrStdout(), "%-24s (error: %v)\n", id, err)
				continue
			}
			backend := profile.Site.Backend
			details := ""
			if profile.Slurm != nil {
				details = fmt.Sprintf("partition=%s account=%s", profile.Slurm.Partition, profile.Slurm.Account)
				if profile.Slurm.QOS != "" {
					details += fmt.Sprintf(" qos=%s", profile.Slurm.QOS)
				}
			}
			fmt.Fprintf(command.OutOrStdout(), "%-24s backend=%-8s %s\n", id, backend, details)
		}
		return nil
	},
}

var siteValidateCmd = &cobra.Command{
	Use:   "validate [site-id]",
	Short: "Validate a site profile against the current environment",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		siteID := "auto"
		if len(args) > 0 {
			siteID = args[0]
		}
		locator := site.DefaultLocator()
		detector := site.NewDetector()
		result, err := detector.Detect(locator, siteID)
		if err != nil {
			return err
		}
		fmt.Fprintf(command.OutOrStdout(), "Backend:  %s\n", result.Backend)
		fmt.Fprintf(command.OutOrStdout(), "Site:    %s\n", result.SiteID)
		fmt.Fprintf(command.OutOrStdout(), "Source:  %s\n", result.Source)
		fmt.Fprintf(command.OutOrStdout(), "Reason:  %s\n", result.Evidence.Reason)
		if result.Evidence.Cluster != "" {
			fmt.Fprintf(command.OutOrStdout(), "Cluster: %s\n", result.Evidence.Cluster)
		}
		if len(result.Evidence.Commands) > 0 {
			fmt.Fprintf(command.OutOrStdout(), "Tools:   %v\n", result.Evidence.Commands)
		}
		return nil
	},
}

var (
	siteGenerateID            string
	siteGenerateOutput        string
	siteGenerateBackend       string
	siteGeneratePartition     string
	siteGenerateAccount       string
	siteGenerateQOS           string
	siteGenerateMaxJobs       int
	siteGenerateDefaultTime   string
	siteGenerateReferenceRoot string
	siteGenerateScratchRoot   string
	siteGenerateForce         bool
)

var siteGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Probe the environment and write a site profile",
	Long: `Probe the local environment and write a site profile.

Discovery reads the SLURM toolchain, the cluster name, the advertised
partitions, and the reference/scratch root candidates. It cannot decide site
policy: an account, a QOS, or which partition is approved for this project are
operator decisions, so they must be supplied explicitly.

The generated profile is validated with the same rules the loader applies, so
"otter site generate" can never write a profile that fails to load.

Backend selection:
  --backend local   always write a local profile
  --backend slurm   requires --partition and --account (and a reference root)
  --backend auto    slurm when the toolchain is complete and both --partition
                    and --account were given, otherwise local

Examples:
  otter site generate --id dev-local --reference-root /shared/otter/references
  otter site generate --id production --backend slurm \
    --partition cpu112c --account genomics \
    --reference-root /shared/otter/references --scratch-root /scratch/genomics`,
	Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, args []string) error {
		return runSiteGenerate(command)
	},
}

func runSiteGenerate(command *cobra.Command) error {
	detector := site.NewDetector()
	discovery := detector.Discover(site.DiscoverProbes{})

	fmt.Fprintln(command.OutOrStdout(), "Observed environment:")
	for _, line := range site.DiscoverySummary(discovery) {
		fmt.Fprintf(command.OutOrStdout(), "  %s\n", line)
	}
	fmt.Fprintln(command.OutOrStdout())

	profile, err := site.BuildProfile(discovery, site.GenerateOptions{
		SiteID:        siteGenerateID,
		Backend:       siteGenerateBackend,
		Partition:     siteGeneratePartition,
		Account:       siteGenerateAccount,
		QOS:           siteGenerateQOS,
		MaxJobs:       siteGenerateMaxJobs,
		DefaultTime:   siteGenerateDefaultTime,
		ReferenceRoot: siteGenerateReferenceRoot,
		ScratchRoot:   siteGenerateScratchRoot,
	})
	if err != nil {
		return err
	}

	outputPath := strings.TrimSpace(siteGenerateOutput)
	if outputPath == "" {
		outputPath = site.DefaultGeneratedProfilePath(profile.Site.ID)
	}
	absolutePath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output path %q: %w", outputPath, err)
	}
	if err := site.WriteProfile(absolutePath, *profile, siteGenerateForce); err != nil {
		return err
	}

	fmt.Fprintf(command.OutOrStdout(), "Wrote site profile: %s\n", absolutePath)
	fmt.Fprintf(command.OutOrStdout(), "  backend: %s\n", profile.Site.Backend)
	if profile.Slurm != nil {
		fmt.Fprintf(command.OutOrStdout(), "  partition: %s account: %s", profile.Slurm.Partition, profile.Slurm.Account)
		if profile.Slurm.QOS != "" {
			fmt.Fprintf(command.OutOrStdout(), " qos: %s", profile.Slurm.QOS)
		}
		fmt.Fprintln(command.OutOrStdout())
	}
	if profile.Paths.ReferenceRoot != "" {
		fmt.Fprintf(command.OutOrStdout(), "  reference root: %s\n", profile.Paths.ReferenceRoot)
	}
	fmt.Fprintln(command.OutOrStdout())
	fmt.Fprintln(command.OutOrStdout(), "Next steps:")
	fmt.Fprintf(command.OutOrStdout(), "  otter site validate %s\n", profile.Site.ID)
	if profile.Site.Backend == "slurm" {
		fmt.Fprintln(command.OutOrStdout(), "Validation re-runs the cluster probes (sinfo, sacctmgr, login-node paths) before you rely on this profile.")
	} else {
		fmt.Fprintln(command.OutOrStdout(), "A local profile needs no cluster probes; it pins the reference root for this machine.")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(siteCmd)
	siteCmd.AddCommand(siteListCmd)
	siteCmd.AddCommand(siteValidateCmd)
	siteCmd.AddCommand(siteGenerateCmd)

	siteGenerateCmd.Flags().StringVar(&siteGenerateID, "id", "", "Site id; also the profile file name (required)")
	siteGenerateCmd.Flags().StringVarP(&siteGenerateOutput, "output", "o", "", "Output path (default: ~/.config/otter/sites/<id>.yaml)")
	siteGenerateCmd.Flags().StringVar(&siteGenerateBackend, "backend", "auto", "Backend: auto, local, or slurm")
	siteGenerateCmd.Flags().StringVar(&siteGeneratePartition, "partition", "", "SLURM partition (required for a slurm profile)")
	siteGenerateCmd.Flags().StringVar(&siteGenerateAccount, "account", "", "SLURM account (required for a slurm profile)")
	siteGenerateCmd.Flags().StringVar(&siteGenerateQOS, "qos", "", "Optional SLURM quality of service")
	siteGenerateCmd.Flags().IntVar(&siteGenerateMaxJobs, "max-jobs", 0, "Optional concurrent submission limit")
	siteGenerateCmd.Flags().StringVar(&siteGenerateDefaultTime, "default-time", "", "Optional default wall time, e.g. 24:00:00")
	siteGenerateCmd.Flags().StringVar(&siteGenerateReferenceRoot, "reference-root", "", "Absolute shared reference registry root (default: $OTTER_REFERENCE_ROOT or ~/.otter/references)")
	siteGenerateCmd.Flags().StringVar(&siteGenerateScratchRoot, "scratch-root", "", "Absolute scratch root (default: $OTTER_SCRATCH_ROOT)")
	siteGenerateCmd.Flags().BoolVar(&siteGenerateForce, "force", false, "Overwrite an existing profile")
	_ = siteGenerateCmd.MarkFlagRequired("id")
}
