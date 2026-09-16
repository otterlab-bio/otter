package config

import (
	"path/filepath"
	"testing"
)

func TestParseResolvesDefaults(t *testing.T) {
	t.Setenv("HOME", "/home/testuser")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_RELEASES_REPO", "")
	t.Setenv("GITHUB_FALLBACK_RELEASES_REPO", "")
	t.Setenv("GITHUB_PROXY_PREFIX", "")
	t.Setenv("OTTER_INSTALL_LANG", "")

	options, err := Parse([]string{"--dry-run", "--skip-envs"})
	if err != nil {
		t.Fatal(err)
	}
	if !options.DryRun {
		t.Fatal("expected dry-run to be set")
	}
	if !options.SkipEnvs {
		t.Fatal("expected skip-envs to be set")
	}
	if options.ReleasesRepo != DefaultReleasesRepo {
		t.Fatalf("unexpected default releases repo %q", options.ReleasesRepo)
	}
	if options.ReleasesRepo != "otterlab-bio/otter" {
		t.Fatalf("expected DefaultReleasesRepo to be otterlab-bio/otter, got %q", options.ReleasesRepo)
	}
	if options.FallbackReleasesRepo != DefaultFallbackReleasesRepo {
		t.Fatalf("unexpected default fallback repo %q", options.FallbackReleasesRepo)
	}
	if options.InstallDir != filepath.Join("/home/testuser", ".cargo", "bin") {
		t.Fatalf("unexpected install dir %q", options.InstallDir)
	}
}

func TestParseRejectsPositionalArguments(t *testing.T) {
	_, err := Parse([]string{"unexpected"})
	if err == nil {
		t.Fatal("expected error for positional arguments")
	}
}

func TestParseReadsEnvironmentOverrides(t *testing.T) {
	t.Setenv("HOME", "/home/testuser")
	t.Setenv("GITHUB_RELEASES_REPO", "owner/custom")
	t.Setenv("GITHUB_FALLBACK_RELEASES_REPO", "owner/fallback")
	t.Setenv("GITHUB_PROXY_PREFIX", "https://proxy.example/")
	t.Setenv("OTTER_INSTALL_LANG", "zh")

	options, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if options.ReleasesRepo != "owner/custom" {
		t.Fatalf("unexpected releases repo %q", options.ReleasesRepo)
	}
	if options.FallbackReleasesRepo != "owner/fallback" {
		t.Fatalf("unexpected fallback repo %q", options.FallbackReleasesRepo)
	}
	if options.GitHubProxy != "https://proxy.example/" {
		t.Fatalf("unexpected proxy %q", options.GitHubProxy)
	}
	if options.Lang != "zh" {
		t.Fatalf("unexpected language %q", options.Lang)
	}
}

func TestParseReadsReferenceBuildOverrides(t *testing.T) {
	t.Setenv("HOME", "/home/testuser")
	t.Setenv("OTTER_REFERENCE_BUILD_BACKEND", "slurm")
	t.Setenv("OTTER_REFERENCE_BUILD_PARTITION", "compute")
	t.Setenv("OTTER_REFERENCE_BUILD_REFERENCE_ID", "hg38")
	t.Setenv("OTTER_REFERENCE_BUILD_RELEASE", "GRCh38.p14")

	options, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	referenceConfig := options.ReferenceBuildConfig
	if referenceConfig.Backend != "slurm" {
		t.Fatalf("unexpected backend %q", referenceConfig.Backend)
	}
	if referenceConfig.Partition != "compute" {
		t.Fatalf("unexpected partition %q", referenceConfig.Partition)
	}
	if referenceConfig.ReferenceID != "hg38" {
		t.Fatalf("unexpected reference id %q", referenceConfig.ReferenceID)
	}
	if referenceConfig.Aliases != "hg38" {
		t.Fatalf("aliases should default to reference id, got %q", referenceConfig.Aliases)
	}
}

func TestNormalizeLanguage(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{input: "zh", expected: "zh"},
		{input: "zh-CN", expected: "zh"},
		{input: "中文", expected: "zh"},
		{input: "en", expected: "en"},
		{input: "en-US", expected: "en"},
		{input: "fr", expected: "en"},
		{input: "", expected: "en"},
	}
	for _, testCase := range testCases {
		if actual := normalizeLanguage(testCase.input); actual != testCase.expected {
			t.Fatalf("normalizeLanguage(%q) = %q, expected %q", testCase.input, actual, testCase.expected)
		}
	}
}

func TestToolsCoverExpectedBinaries(t *testing.T) {
	expected := []string{"otter", "craftmake", "enva", "xenofilx", "pairbam", "seq2mat", "methx", "qctb", "fastqcx", "matsrun"}
	if len(Tools) != len(expected) {
		t.Fatalf("expected %d tools, got %d", len(expected), len(Tools))
	}
	for index, name := range expected {
		if Tools[index].Name != name {
			t.Fatalf("tool %d = %q, expected %q", index, Tools[index].Name, name)
		}
	}
}
