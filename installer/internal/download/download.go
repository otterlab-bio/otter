// Package download resolves GitHub release metadata and downloads release
// assets with optional proxy and token support.
package download

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client downloads release assets from GitHub.
type Client struct {
	HTTP      *http.Client
	Token     string
	Proxy     string
	DryRun    bool
	UserAgent string
}

// NewClient returns a Client with sane timeouts.
func NewClient(token, proxy string, dryRun bool) *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: 5 * time.Minute,
		},
		Token:     token,
		Proxy:     proxy,
		DryRun:    dryRun,
		UserAgent: "otter-installer/1.0",
	}
}

// Platform identifies the release asset family. macOS binaries use the
// system runtime, so only Linux builds carry the -static suffix.
type Platform struct {
	OS, Arch string
}

func NewPlatform(goos, goarch string) (Platform, error) {
	if (goos != "linux" && goos != "darwin") || (goarch != "amd64" && goarch != "arm64") {
		return Platform{}, fmt.Errorf("unsupported platform %s/%s (expected Linux or macOS on amd64 or arm64)", goos, goarch)
	}
	return Platform{OS: goos, Arch: goarch}, nil
}

func (p Platform) AssetName(stem string) string {
	name := stem + "-" + p.OS + "-" + p.Arch
	if p.OS == "linux" {
		name += "-static"
	}
	return name
}

type Release struct {
	Tag    string `json:"tag_name"`
	Draft  bool   `json:"draft"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"assets"`
}

func (release Release) AssetAPIURL(name string) string {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset.URL
		}
	}
	return ""
}

// FindRelease selects a release with all required assets. Older releases were
// Linux amd64 only, and a release still being uploaded can be incomplete.
func (client *Client) FindRelease(ctx context.Context, repo, tag string, required []string) (Release, error) {
	base := fmt.Sprintf("https://api.github.com/repos/%s/releases", repo)
	var releases []Release
	if tag == "" || tag == "latest" {
		body, err := client.get(ctx, base+"?per_page=100")
		if err != nil {
			return Release{}, err
		}
		if err := json.Unmarshal(body, &releases); err != nil {
			return Release{}, fmt.Errorf("parse releases for %s: %w", repo, err)
		}
	} else {
		body, err := client.get(ctx, base+"/tags/"+url.PathEscape(tag))
		if err != nil {
			return Release{}, err
		}
		var release Release
		if err := json.Unmarshal(body, &release); err != nil {
			return Release{}, fmt.Errorf("parse release %s for %s: %w", tag, repo, err)
		}
		releases = []Release{release}
	}
	for _, release := range releases {
		if release.Tag == "" || release.Draft {
			continue
		}
		present := make(map[string]bool, len(release.Assets))
		for _, asset := range release.Assets {
			present[asset.Name] = true
		}
		var missing []string
		for _, name := range required {
			if !present[name] {
				missing = append(missing, name)
			}
		}
		if len(missing) == 0 {
			return release, nil
		}
		if tag != "" && tag != "latest" {
			return Release{}, fmt.Errorf("%s release %s is missing assets: %s", repo, tag, strings.Join(missing, ", "))
		}
	}
	return Release{}, fmt.Errorf("%s has no matching published release with all required assets", repo)
}

// Download fetches a release asset to dest, honoring proxy and token.
func (client *Client) Download(ctx context.Context, url, dest string) error {
	if client.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	downloadURL := url
	if client.Proxy != "" {
		downloadURL = client.ApplyProxy(url)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", client.UserAgent)
	if client.Token != "" && client.Proxy == "" {
		request.Header.Set("Authorization", "Bearer "+client.Token)
		request.Header.Set("Accept", "application/octet-stream")
	}
	response, err := client.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", downloadURL, response.StatusCode)
	}
	output, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer output.Close()
	if _, err := io.Copy(output, response.Body); err != nil {
		return err
	}
	return output.Sync()
}

// ReleaseAssetURL returns the direct download URL for an asset in a release.
func ReleaseAssetURL(repo, tag, asset string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, asset)
}

// ApplyProxy prefixes a URL with the configured GitHub proxy.
func (client *Client) ApplyProxy(url string) string {
	if client.Proxy == "" {
		return url
	}
	prefix := strings.TrimSuffix(client.Proxy, "/")
	return prefix + "/" + url
}

// ErrResourceNotFound reports a resource that the host does not have.
var ErrResourceNotFound = errors.New("resource not found")

// FetchResource retrieves a small text resource such as a parts manifest.
//
// It differs from get in two ways that matter for dataset hosts: the proxy prefix is applied,
// and the GitHub authorization header is not sent, since the resource lives on a different
// service that would reject an unrelated token.
func (client *Client) FetchResource(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.ApplyProxy(url), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", client.UserAgent)
	response, err := client.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, ErrResourceNotFound
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, response.Status)
	}
	return io.ReadAll(response.Body)
}

func (client *Client) get(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", client.UserAgent)
	request.Header.Set("Accept", "application/vnd.github+json")
	if client.Token != "" {
		request.Header.Set("Authorization", "Bearer "+client.Token)
	}
	response, err := client.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, response.StatusCode)
	}
	return io.ReadAll(response.Body)
}
