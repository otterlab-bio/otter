package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testProbes returns probes with a deterministic environment.
func testProbes(environment map[string]string, partitions []string) DiscoverProbes {
	return DiscoverProbes{
		Environment: func(key string) string { return environment[key] },
		UserHomeDir: func() (string, error) { return "/home/tester", nil },
		ListPartitions: func() ([]string, error) {
			return partitions, nil
		},
	}
}

func TestDiscoverReportsAbsentToolchainAndCandidates(t *testing.T) {
	detector := NewDetector(WithToolChecker(func(string) bool { return false }))
	discovery := detector.Discover(testProbes(map[string]string{
		DefaultReferenceRootEnvironmentVariable: "/shared/otter/references",
	}, nil))

	if discovery.SlurmToolchainComplete {
		t.Fatal("toolchain must not be reported complete when no tool is present")
	}
	if len(discovery.SlurmToolsPresent) != 0 {
		t.Fatalf("expected no tools, got %v", discovery.SlurmToolsPresent)
	}
	if discovery.ReferenceRootCandidate != "/shared/otter/references" {
		t.Fatalf("reference root candidate = %q", discovery.ReferenceRootCandidate)
	}
	if len(discovery.Partitions) != 0 {
		t.Fatalf("partitions must not be probed without a complete toolchain, got %v", discovery.Partitions)
	}
}

func TestDiscoverFallsBackToHomeRegistryRoot(t *testing.T) {
	detector := NewDetector(WithToolChecker(func(string) bool { return false }))
	discovery := detector.Discover(testProbes(map[string]string{}, nil))
	if discovery.ReferenceRootCandidate != "/home/tester/.otter/references" {
		t.Fatalf("reference root candidate = %q", discovery.ReferenceRootCandidate)
	}
}

func TestDiscoverReadsClusterAndPartitionsWhenComplete(t *testing.T) {
	detector := NewDetector(
		WithToolChecker(func(string) bool { return true }),
		WithClusterNameGetter(func() (string, bool) { return "cluster-a", true }),
	)
	discovery := detector.Discover(testProbes(map[string]string{}, []string{"cpu", "gpu"}))
	if !discovery.SlurmToolchainComplete {
		t.Fatal("toolchain should be complete")
	}
	if discovery.ClusterName != "cluster-a" {
		t.Fatalf("cluster = %q", discovery.ClusterName)
	}
	if len(discovery.Partitions) != 2 {
		t.Fatalf("partitions = %v", discovery.Partitions)
	}
}

func TestBuildProfileWithoutToolchainProducesLocal(t *testing.T) {
	discovery := Discovery{ReferenceRootCandidate: "/shared/refs"}
	profile, err := BuildProfile(discovery, GenerateOptions{SiteID: "dev"})
	if err != nil {
		t.Fatalf("BuildProfile failed: %v", err)
	}
	if profile.Site.Backend != "local" {
		t.Fatalf("backend = %q, want local", profile.Site.Backend)
	}
	if profile.Slurm != nil {
		t.Fatal("a local profile must not carry a slurm block")
	}
	if profile.Paths.ReferenceRoot != "/shared/refs" {
		t.Fatalf("reference root = %q", profile.Paths.ReferenceRoot)
	}
	if err := ValidateProfile(profile); err != nil {
		t.Fatalf("generated profile must pass validation: %v", err)
	}
}

func TestBuildProfileAutoSelectsSlurmOnlyWithPolicy(t *testing.T) {
	discovery := Discovery{
		SlurmToolchainComplete: true,
		Partitions:             []string{"cpu"},
		ReferenceRootCandidate: "/shared/refs",
	}

	// Auto without partition/account must not invent a slurm profile.
	localProfile, err := BuildProfile(discovery, GenerateOptions{SiteID: "auto-local"})
	if err != nil {
		t.Fatal(err)
	}
	if localProfile.Site.Backend != "local" {
		t.Fatalf("auto without policy selected %q, want local", localProfile.Site.Backend)
	}

	slurmProfile, err := BuildProfile(discovery, GenerateOptions{
		SiteID: "auto-slurm", Partition: "cpu", Account: "genomics",
	})
	if err != nil {
		t.Fatal(err)
	}
	if slurmProfile.Site.Backend != "slurm" || slurmProfile.Slurm == nil {
		t.Fatalf("expected a slurm profile, got %+v", slurmProfile)
	}
	if slurmProfile.Slurm.Partition != "cpu" || slurmProfile.Slurm.Account != "genomics" {
		t.Fatalf("slurm block = %+v", slurmProfile.Slurm)
	}
	if err := ValidateProfile(slurmProfile); err != nil {
		t.Fatalf("generated slurm profile must pass structural validation: %v", err)
	}
}

func TestBuildProfileSlurmRequiresPolicyAndNamesCandidates(t *testing.T) {
	discovery := Discovery{
		SlurmToolchainComplete: true,
		Partitions:             []string{"cpu112c", "gpu"},
		ReferenceRootCandidate: "/shared/refs",
	}

	_, err := BuildProfile(discovery, GenerateOptions{SiteID: "prod", Backend: "slurm"})
	if err == nil {
		t.Fatal("expected --partition to be required")
	}
	if !strings.Contains(err.Error(), "--partition") {
		t.Fatalf("error should name the missing flag: %v", err)
	}
	if !strings.Contains(err.Error(), "cpu112c") {
		t.Fatalf("error should name the observed partitions: %v", err)
	}

	_, err = BuildProfile(discovery, GenerateOptions{SiteID: "prod", Backend: "slurm", Partition: "cpu112c"})
	if err == nil || !strings.Contains(err.Error(), "--account") {
		t.Fatalf("expected --account to be required: %v", err)
	}
}

func TestBuildProfileRequiresAbsolutePathsAndPlainID(t *testing.T) {
	discovery := Discovery{}
	if _, err := BuildProfile(discovery, GenerateOptions{SiteID: ""}); err == nil {
		t.Fatal("expected an empty --id to be rejected")
	}
	if _, err := BuildProfile(discovery, GenerateOptions{SiteID: "a/b"}); err == nil {
		t.Fatal("expected a path-separator --id to be rejected")
	}
	if _, err := BuildProfile(discovery, GenerateOptions{SiteID: "ok", ReferenceRoot: "relative/refs"}); err == nil {
		t.Fatal("expected a relative --reference-root to be rejected")
	}
	if _, err := BuildProfile(discovery, GenerateOptions{SiteID: "ok", Backend: "k8s"}); err == nil {
		t.Fatal("expected an unsupported backend to be rejected")
	}
}

func TestWriteProfileRefusesOverwriteAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sites", "dev.yaml")
	profile := SiteProfile{
		SchemaVersion: profileSchemaVersion,
		Site:          SiteIdentity{ID: "dev", Backend: "local"},
		Paths:         SitePaths{ReferenceRoot: "/shared/refs"},
	}
	if err := WriteProfile(path, profile, false); err != nil {
		t.Fatalf("WriteProfile failed: %v", err)
	}

	loaded, err := LoadProfile(path)
	if err != nil {
		t.Fatalf("the generated profile must load: %v", err)
	}
	if loaded.Site.ID != "dev" || loaded.Paths.ReferenceRoot != "/shared/refs" {
		t.Fatalf("profile did not round-trip: %+v", loaded)
	}

	if err := WriteProfile(path, profile, false); err == nil {
		t.Fatal("expected overwrite to be refused without force")
	}
	if err := WriteProfile(path, profile, true); err != nil {
		t.Fatalf("--force should permit overwrite: %v", err)
	}
}

func TestWriteProfileRejectsInvalidProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	// A slurm profile with no slurm block cannot load, so it must not be written.
	profile := SiteProfile{
		SchemaVersion: profileSchemaVersion,
		Site:          SiteIdentity{ID: "bad", Backend: "slurm"},
		Paths:         SitePaths{ReferenceRoot: "/shared/refs"},
	}
	if err := WriteProfile(path, profile, false); err == nil {
		t.Fatal("expected an unloadable profile to be refused")
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("no file should have been written")
	}
}

func TestDetectReadsAnExplicitlyNamedLocalSite(t *testing.T) {
	// The bug this covers: a machine without SLURM used to ignore the named
	// profile and report its own detection instead.
	sitesDir := t.TempDir()
	profilePath := filepath.Join(sitesDir, "dev-local.yaml")
	if err := WriteProfile(profilePath, SiteProfile{
		SchemaVersion: profileSchemaVersion,
		Site:          SiteIdentity{ID: "dev-local", Backend: "local"},
		Paths:         SitePaths{ReferenceRoot: "/shared/refs"},
	}, false); err != nil {
		t.Fatal(err)
	}

	detector := NewDetector(WithToolChecker(func(string) bool { return false }))
	locator := Locator{UserConfigDir: sitesDir}
	result, err := detector.Detect(locator, "dev-local")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if result.SiteID != "dev-local" {
		t.Fatalf("site = %q, want dev-local", result.SiteID)
	}
	if result.Backend != "local" {
		t.Fatalf("backend = %q, want local", result.Backend)
	}
	if result.SitePaths.ReferenceRoot != "/shared/refs" {
		t.Fatalf("reference root was not taken from the profile: %q", result.SitePaths.ReferenceRoot)
	}
	if result.Source != "profile" {
		t.Fatalf("source = %q, want profile", result.Source)
	}
}

func TestDetectAutoStillUsesDetection(t *testing.T) {
	detector := NewDetector(WithToolChecker(func(string) bool { return false }))
	result, err := detector.Detect(Locator{UserConfigDir: t.TempDir()}, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Backend != "local" || result.SiteID != defaultLocalSiteID {
		t.Fatalf("auto detection should report the local default, got %+v", result)
	}
}

func TestDetectRejectsAnUnknownNamedSite(t *testing.T) {
	detector := NewDetector(WithToolChecker(func(string) bool { return false }))
	if _, err := detector.Detect(Locator{UserConfigDir: t.TempDir()}, "missing-site"); err == nil {
		t.Fatal("expected an unknown site id to be rejected")
	}
}
