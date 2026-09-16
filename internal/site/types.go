package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"gopkg.in/yaml.v3"
)

const profileSchemaVersion = "otter.site/v1"

const defaultLocalSiteID = "local-default"

type SiteProfile struct {
	SchemaVersion string       `yaml:"schema_version"`
	Site          SiteIdentity `yaml:"site"`
	Slurm         *SlurmConfig `yaml:"slurm,omitempty"`
	Paths         SitePaths    `yaml:"paths"`
}

type SiteIdentity struct {
	ID      string `yaml:"id"`
	Backend string `yaml:"backend"`
}

type SlurmConfig struct {
	Partition   string `yaml:"partition"`
	Account     string `yaml:"account,omitempty"`
	QOS         string `yaml:"qos,omitempty"`
	MaxJobs     int    `yaml:"max_jobs,omitempty"`
	DefaultTime string `yaml:"default_time,omitempty"`
}

type SitePaths struct {
	ReferenceRoot string `yaml:"reference_root"`
	ScratchRoot   string `yaml:"scratch_root,omitempty"`
}

type DetectionResult struct {
	Backend        configv1.Backend
	SiteID         string
	Evidence       configv1.BackendEvidence
	Source         configv1.ValueSource
	SitePaths      SitePaths
	SiteResources  configv1.ProjectResources
	SlurmResources configv1.ResolvedSlurmResources
}

type Locator struct {
	UserConfigDir   string
	SystemConfigDir string
}

func DefaultLocator() Locator {
	return Locator{
		UserConfigDir:   filepath.Join(userHomeDir(), ".config", "otter", "sites"),
		SystemConfigDir: "/etc/otter/sites",
	}
}

func (locator Locator) Find(siteID string) (*SiteProfile, error) {
	if envPath := os.Getenv("OTTER_SITE_PROFILE"); envPath != "" {
		return LoadProfile(envPath)
	}
	for _, dir := range locator.directories() {
		candidatePath := filepath.Join(dir, siteID+".yaml")
		profile, err := LoadProfile(candidatePath)
		if err != nil {
			continue
		}
		return profile, nil
	}
	return nil, fmt.Errorf("site profile %q not found in search paths: %v", siteID, locator.directories())
}

func (locator Locator) List() ([]string, error) {
	var discovered []string
	for _, dir := range locator.directories() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
				continue
			}
			discovered = append(discovered, strings.TrimSuffix(entry.Name(), ".yaml"))
		}
	}
	if len(discovered) == 0 {
		return nil, fmt.Errorf("no site profiles found in search paths: %v", locator.directories())
	}
	return discovered, nil
}

func (locator Locator) FindByBackend(backend string) (*SiteProfile, error) {
	candidates, err := locator.List()
	if err != nil {
		return nil, err
	}
	for _, id := range candidates {
		profile, err := locator.Find(id)
		if err != nil {
			continue
		}
		if profile.Site.Backend == backend {
			return profile, nil
		}
	}
	return nil, fmt.Errorf("no site profile with backend %q found", backend)
}

func (locator Locator) directories() []string {
	dirs := make([]string, 0, 2)
	if locator.UserConfigDir != "" {
		dirs = append(dirs, locator.UserConfigDir)
	}
	if locator.SystemConfigDir != "" {
		dirs = append(dirs, locator.SystemConfigDir)
	}
	return dirs
}

func LoadProfile(path string) (*SiteProfile, error) {
	data, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read site profile %q: %w", path, err)
	}
	defer data.Close()

	var profile SiteProfile
	decoder := yaml.NewDecoder(data)
	decoder.KnownFields(true)
	if err := decoder.Decode(&profile); err != nil {
		return nil, fmt.Errorf("parse site profile %q: %w", path, err)
	}
	if err := ValidateProfile(&profile); err != nil {
		return nil, fmt.Errorf("site profile %q: %w", path, err)
	}
	return &profile, nil
}

// ValidateProfile normalises and checks a profile in place.
//
// "otter site generate" calls this before writing so a generated profile can
// never be one the loader would reject.
func ValidateProfile(profile *SiteProfile) error {
	if profile.SchemaVersion != profileSchemaVersion {
		return fmt.Errorf("schema_version %q is not %q", profile.SchemaVersion, profileSchemaVersion)
	}
	if strings.TrimSpace(profile.Site.ID) == "" {
		return fmt.Errorf("site.id is empty")
	}
	backend := strings.ToLower(strings.TrimSpace(profile.Site.Backend))
	if backend != "local" && backend != "slurm" {
		return fmt.Errorf("unsupported backend %q", profile.Site.Backend)
	}
	profile.Site.Backend = backend
	if backend == "slurm" && profile.Slurm == nil {
		return fmt.Errorf("backend slurm requires a slurm configuration")
	}
	if strings.TrimSpace(profile.Paths.ReferenceRoot) != "" && !filepath.IsAbs(profile.Paths.ReferenceRoot) {
		return fmt.Errorf("paths.reference_root must be absolute")
	}
	if strings.TrimSpace(profile.Paths.ScratchRoot) != "" && !filepath.IsAbs(profile.Paths.ScratchRoot) {
		return fmt.Errorf("paths.scratch_root must be absolute")
	}
	if backend == "slurm" && strings.TrimSpace(profile.Paths.ReferenceRoot) == "" {
		return fmt.Errorf("backend slurm requires paths.reference_root")
	}
	return nil
}

// WriteProfile writes a profile, creating the parent directory.
//
// It refuses to overwrite unless force is set, because an existing profile may
// carry a partition/account the user has already validated against the cluster.
func WriteProfile(path string, profile SiteProfile, force bool) error {
	if err := ValidateProfile(&profile); err != nil {
		return err
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists; pass --force to overwrite it", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
	}
	encoded, err := yaml.Marshal(profile)
	if err != nil {
		return fmt.Errorf("marshal site profile: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create site profile directory: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("write site profile %s: %w", path, err)
	}
	return nil
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
