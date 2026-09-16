// Package config defines the installer options and their resolution from
// command-line flags and environment variables.
package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Tool describes a release asset to download and install.
type Tool struct {
	// Name is the installed binary name.
	Name string
	// AssetStem is the release asset stem before the platform suffix.
	AssetStem string
	// Linkage is "static" or "dynamic".
	Linkage string
}

// Tools is the canonical set of binaries the installer manages.
var Tools = []Tool{
	{Name: "otter", AssetStem: "otter", Linkage: "static"},
	{Name: "craftmake", AssetStem: "craftmake", Linkage: "static"},
	{Name: "enva", AssetStem: "enva", Linkage: "static"},
	{Name: "xenofilx", AssetStem: "xenofilx", Linkage: "static"},
	{Name: "pairbam", AssetStem: "pairbam", Linkage: "static"},
	{Name: "seq2mat", AssetStem: "seq2mat", Linkage: "static"},
	{Name: "methx", AssetStem: "methx", Linkage: "static"},
	{Name: "qctb", AssetStem: "qctb", Linkage: "static"},
	{Name: "fastqcx", AssetStem: "fastqcx", Linkage: "static"},
	{Name: "matsrun", AssetStem: "matsrun", Linkage: "static"},
}

// EnvFiles are the conda environment YAML assets staged with releases.
var EnvFiles = []string{
	"otter-core.yaml",
	"otter-snakemake.yaml",
	"otter-extra.yaml",
}

// Options holds the fully resolved installer configuration.
type Options struct {
	InstallDir           string
	SkipEnvs             bool
	NonInteractive       bool
	DryRun               bool
	Version              string
	ReleasesRepo         string
	FallbackReleasesRepo string
	GitHubProxy          string
	Lang                 string
	ReferenceBuild       bool

	// ReferenceBuild overrides (OTTER_REFERENCE_BUILD_*).
	ReferenceBuildConfig ReferenceBuildConfig

	// ReferenceFetch downloads published index builds rather than compiling them.
	ReferenceFetch bool

	// ReferenceFetchConfig holds the OTTER_REFERENCE_FETCH_* overrides.
	ReferenceFetchConfig ReferenceFetchConfig

	// GitHubToken is the optional authentication token for private releases.
	GitHubToken string

	// Home is the resolved user home directory.
	Home string
}

// ReferenceBuildConfig holds the OTTER_REFERENCE_BUILD_* overrides.
type ReferenceBuildConfig struct {
	RunID                  string
	Backend                string
	Partition              string
	ReferenceID            string
	Release                string
	Organism               string
	Assembly               string
	Aliases                string
	CacheDir               string
	WorkDir                string
	RegistryRoot           string
	EvidenceDir            string
	FastaURL               string
	FastaFilename          string
	FastaChecksumAlgorithm string
	FastaChecksumValue     string
	GTFURL                 string
	GTFFilename            string
	GTFChecksumAlgorithm   string
	GTFChecksumValue       string
	Contigs                string
	STARSJDBOverhang       string
	IndexBuildThreads      string
	OtterBinary            string
	SamtoolsBinary         string
	BismarkBinary          string
	Bowtie2Binary          string
	STARBinary             string
}

// ReferenceFetchConfig holds the OTTER_REFERENCE_FETCH_* overrides.
type ReferenceFetchConfig struct {
	// Repo is the dataset repository holding the published index archives.
	Repo string
	// Releases is a comma-separated list of <id>@<release> selections.
	Releases string
	// Assets is a comma-separated subset of bismark,bowtie2,star,fasta,annotations.
	Assets string
	// BaseURL is the dataset host; a mirror can be substituted where the default is blocked.
	BaseURL string
	// Revision is the dataset revision to resolve.
	Revision string
	// RegistryRoot is the directory containing genomes/.
	RegistryRoot string
}

// DefaultReleasesRepo is the primary public release repository.
const DefaultReleasesRepo = "otterlab-bio/otter"

// DefaultReferenceFetchRepo is the dataset that mirrors the reference registry layout.
const DefaultReferenceFetchRepo = "fallingstar10/xdxtools-genomes"

// DefaultReferenceFetchBaseURL is the public dataset host.
const DefaultReferenceFetchBaseURL = "https://huggingface.co"

// DefaultFallbackReleasesRepo is the fallback release repository.
const DefaultFallbackReleasesRepo = "otterlab-bio/otter"

// DefaultCraftmakeReleasesRepo is the Craftmake release repository.
const DefaultCraftmakeReleasesRepo = "otterlab-bio/craftmake"

// Parse builds Options from command-line flags and environment variables.
func Parse(arguments []string) (*Options, error) {
	options := &Options{}

	flagSet := flag.NewFlagSet("otter-install", flag.ContinueOnError)
	flagSet.SetOutput(os.Stderr)

	flagSet.StringVar(&options.InstallDir, "install-dir", "", "Override binary installation directory")
	flagSet.BoolVar(&options.SkipEnvs, "skip-envs", false, "Skip conda environment creation")
	flagSet.BoolVar(&options.NonInteractive, "non-interactive", false, "Use all defaults without prompting")
	flagSet.BoolVar(&options.DryRun, "dry-run", false, "Print all actions without executing")
	flagSet.StringVar(&options.Version, "version", "latest", "Release version (e.g. v0.3.0)")
	flagSet.StringVar(&options.ReleasesRepo, "releases-repo", "", "Primary GitHub release repo (owner/name)")
	flagSet.StringVar(&options.FallbackReleasesRepo, "fallback-releases-repo", "", "Fallback GitHub release repo (owner/name)")
	flagSet.StringVar(&options.GitHubProxy, "github-proxy", "", "Optional GitHub proxy prefix")
	flagSet.StringVar(&options.Lang, "lang", "", "Interface language: en or zh")
	flagSet.BoolVar(&options.ReferenceBuild, "reference-build", false, "Download and run the Craftmake ReferenceBuild workflow")
	flagSet.BoolVar(&options.ReferenceFetch, "reference-fetch", false, "Download published reference index builds from the dataset")

	if err := flagSet.Parse(arguments); err != nil {
		return nil, err
	}
	if flagSet.NArg() > 0 {
		return nil, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flagSet.Args(), " "))
	}

	options.Home = resolveHome()
	options.GitHubToken = firstEnv("GITHUB_TOKEN", "GH_TOKEN", "GITHUB_PAT")

	if options.ReleasesRepo == "" {
		options.ReleasesRepo = firstEnv("GITHUB_RELEASES_REPO", "OTTER_RELEASES_REPO")
	}
	if options.ReleasesRepo == "" {
		options.ReleasesRepo = DefaultReleasesRepo
	}
	if options.FallbackReleasesRepo == "" {
		options.FallbackReleasesRepo = firstEnv("GITHUB_FALLBACK_RELEASES_REPO", "OTTER_FALLBACK_RELEASES_REPO")
	}
	if options.FallbackReleasesRepo == "" {
		options.FallbackReleasesRepo = DefaultFallbackReleasesRepo
	}
	if options.GitHubProxy == "" {
		options.GitHubProxy = firstEnv("GITHUB_PROXY_PREFIX", "OTTER_GITHUB_PROXY")
	}
	if options.Lang == "" {
		options.Lang = firstEnv("OTTER_INSTALL_LANG")
	}
	if options.Lang == "" {
		options.Lang = detectDefaultLanguage()
	}
	options.Lang = normalizeLanguage(options.Lang)

	if options.InstallDir == "" {
		options.InstallDir = filepath.Join(options.Home, ".cargo", "bin")
	}

	options.ReferenceBuildConfig = readReferenceBuildConfig(options.Home, options.InstallDir)
	options.ReferenceFetchConfig = readReferenceFetchConfig(options.Home)
	return options, nil
}

// readReferenceFetchConfig resolves the dataset fetch settings. The registry root is shared
// with ReferenceBuild so a fetched release lands wherever a built one would.
func readReferenceFetchConfig(home string) ReferenceFetchConfig {
	registryRoot := envOr("OTTER_REFERENCE_FETCH_REGISTRY_ROOT", "")
	if registryRoot == "" {
		registryRoot = envOr("OTTER_REFERENCE_BUILD_REGISTRY_ROOT", filepath.Join(home, ".otter", "references"))
	}
	return ReferenceFetchConfig{
		Repo:         envOr("OTTER_REFERENCE_FETCH_REPO", DefaultReferenceFetchRepo),
		Releases:     envOr("OTTER_REFERENCE_FETCH_RELEASES", ""),
		Assets:       envOr("OTTER_REFERENCE_FETCH_ASSETS", ""),
		BaseURL:      envOr("OTTER_REFERENCE_FETCH_BASE_URL", DefaultReferenceFetchBaseURL),
		Revision:     envOr("OTTER_REFERENCE_FETCH_REVISION", "main"),
		RegistryRoot: registryRoot,
	}
}

func readReferenceBuildConfig(home, installDir string) ReferenceBuildConfig {
	config := ReferenceBuildConfig{
		Backend:                envOr("OTTER_REFERENCE_BUILD_BACKEND", "local"),
		Partition:              envOr("OTTER_REFERENCE_BUILD_PARTITION", os.Getenv("CRAFTMAKE_SLURM_PARTITION")),
		Aliases:                envOr("OTTER_REFERENCE_BUILD_ALIASES", ""),
		FastaChecksumAlgorithm: envOr("OTTER_REFERENCE_BUILD_FASTA_CHECKSUM_ALGORITHM", "md5"),
		GTFChecksumAlgorithm:   envOr("OTTER_REFERENCE_BUILD_GTF_CHECKSUM_ALGORITHM", "md5"),
		Contigs:                envOr("OTTER_REFERENCE_BUILD_CONTIGS", "*"),
		STARSJDBOverhang:       envOr("OTTER_REFERENCE_BUILD_STAR_SJDB_OVERHANG", "149"),
		IndexBuildThreads:      envOr("OTTER_REFERENCE_BUILD_INDEX_BUILD_THREADS", "16"),
		RegistryRoot:           envOr("OTTER_REFERENCE_BUILD_REGISTRY_ROOT", filepath.Join(home, ".otter", "references")),
		OtterBinary:            envOr("OTTER_REFERENCE_BUILD_OTTER_BINARY", filepath.Join(installDir, "otter")),
	}
	config.ReferenceID = envOr("OTTER_REFERENCE_BUILD_REFERENCE_ID", "")
	config.Release = envOr("OTTER_REFERENCE_BUILD_RELEASE", "")
	config.Organism = envOr("OTTER_REFERENCE_BUILD_ORGANISM", "")
	config.Assembly = envOr("OTTER_REFERENCE_BUILD_ASSEMBLY", "")
	config.FastaURL = envOr("OTTER_REFERENCE_BUILD_FASTA_URL", "")
	config.GTFURL = envOr("OTTER_REFERENCE_BUILD_GTF_URL", "")
	config.FastaChecksumValue = envOr("OTTER_REFERENCE_BUILD_FASTA_CHECKSUM_VALUE", "")
	config.GTFChecksumValue = envOr("OTTER_REFERENCE_BUILD_GTF_CHECKSUM_VALUE", "")
	config.FastaFilename = envOr("OTTER_REFERENCE_BUILD_FASTA_FILENAME", "")
	config.GTFFilename = envOr("OTTER_REFERENCE_BUILD_GTF_FILENAME", "")
	config.CacheDir = envOr("OTTER_REFERENCE_BUILD_CACHE_DIR", filepath.Join(home, ".otter", "reference-build", "cache", config.ReferenceID))
	config.WorkDir = envOr("OTTER_REFERENCE_BUILD_WORK_DIR", filepath.Join(home, ".otter", "reference-build", "work", config.ReferenceID))
	config.EvidenceDir = envOr("OTTER_REFERENCE_BUILD_EVIDENCE_DIR", filepath.Join(home, ".otter", "reference-build", "evidence", config.ReferenceID))
	config.SamtoolsBinary = envOr("OTTER_REFERENCE_BUILD_SAMTOOLS_BINARY", "")
	config.BismarkBinary = envOr("OTTER_REFERENCE_BUILD_BISMARK_BINARY", "")
	config.Bowtie2Binary = envOr("OTTER_REFERENCE_BUILD_BOWTIE2_BINARY", "")
	config.STARBinary = envOr("OTTER_REFERENCE_BUILD_STAR_BINARY", "")
	config.RunID = envOr("OTTER_REFERENCE_BUILD_RUN_ID", "")
	if config.Aliases == "" {
		config.Aliases = config.ReferenceID
	}
	return config
}

func resolveHome() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if runtime.GOOS == "windows" {
		if home := os.Getenv("USERPROFILE"); home != "" {
			return home
		}
	}
	return "."
}

func detectDefaultLanguage() string {
	lang := os.Getenv("LANG")
	if strings.HasPrefix(strings.ToLower(lang), "zh") {
		return "zh"
	}
	return "en"
}

func normalizeLanguage(lang string) string {
	switch strings.ToLower(lang) {
	case "zh", "zh-cn", "zh_tw", "cn", "中文":
		return "zh"
	case "en", "en-us", "en_us", "english":
		return "en"
	default:
		return "en"
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
