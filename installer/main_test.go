package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otterlab-bio/otter/installer/internal/config"
	"github.com/otterlab-bio/otter/installer/internal/download"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestInstallToolsKeepsPreviousToolsOnFailure(t *testing.T) {
	p, err := download.NewPlatform("darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	old := filepath.Join(dir, "otter")
	if err := os.WriteFile(old, []byte("old tool"), 0o755); err != nil {
		t.Fatal(err)
	}
	client := download.NewClient("", "", false)
	requests := 0
	client.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		switch path := r.URL.Path; {
		case strings.HasSuffix(path, "/otter-darwin-arm64"):
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("#!/bin/sh\necho 'otter v1.0.0'\n"))}, nil
		case strings.HasSuffix(path, "/craftmake-darwin-arm64"):
			return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody}, nil
		default:
			return nil, fmt.Errorf("unexpected download URL %s", r.URL)
		}
	})
	err = installTools(context.Background(), client, dir, "owner/repo", download.Release{Tag: "v1.0.0"}, p)
	if err == nil || !strings.Contains(err.Error(), "craftmake-darwin-arm64") || requests != 2 {
		t.Fatalf("expected a failure on the second download; got %d downloads, %v", requests, err)
	}
	if got, err := os.ReadFile(old); err != nil || string(got) != "old tool" {
		t.Fatalf("existing install was changed: %q, %v", got, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Fatalf("staging directory was not cleaned: %v, %v", entries, err)
	}
}

func TestResolveReleaseUsesCompatibleFallbackRepository(t *testing.T) {
	p, err := download.NewPlatform("linux", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	var assets []map[string]string
	for _, name := range config.Tools {
		assets = append(assets, map[string]string{"name": p.AssetName(name)})
	}
	body, err := json.Marshal(map[string]any{"tag_name": "v1.2.3", "assets": assets})
	if err != nil {
		t.Fatal(err)
	}
	client := download.NewClient("", "", false)
	client.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/repos/primary/otter/releases/tags/v1.2.3":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v1.2.3","assets":[]}`))}, nil
		case "/repos/fallback/otter/releases/tags/v1.2.3":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
		default:
			return nil, fmt.Errorf("unexpected request %s", r.URL)
		}
	})
	options := &config.Options{ReleasesRepo: "primary/otter", FallbackReleasesRepo: "fallback/otter", Version: "v1.2.3", SkipEnvs: true}
	repo, release, err := resolveRelease(context.Background(), client, options, p)
	if err != nil || repo != options.FallbackReleasesRepo || release.Tag != options.Version {
		t.Fatalf("wrong release selected: %q tag=%q (%v)", repo, release.Tag, err)
	}
	url := releaseURL(client, repo, release, p.AssetName("otter"))
	if url != "https://github.com/fallback/otter/releases/download/v1.2.3/otter-linux-arm64-static" {
		t.Fatalf("wrong fallback download URL: %s", url)
	}
}

func TestPinnedDryRunIsOfflineAndReadOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin")
	p, err := download.NewPlatform("linux", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	client := download.NewClient("", "", true)
	client.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s", r.URL)
		return nil, nil
	})
	options := &config.Options{DryRun: true, ReleasesRepo: "owner/repo", Version: "v1.0.0"}
	repo, release, err := resolveRelease(context.Background(), client, options, p)
	if err != nil || repo != "owner/repo" || release.Tag != "v1.0.0" {
		t.Fatalf("wrong pinned release: %s, %+v, %v", repo, release, err)
	}
	if err := installTools(context.Background(), client, dir, repo, release, p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("dry run unexpectedly touched install directory: %v", err)
	}
}
