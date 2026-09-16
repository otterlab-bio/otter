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
	"os"
	"path/filepath"
	"runtime"
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

// AssetName builds the release asset name for a tool on the current platform.
func AssetName(assetStem, linkage string) string {
	arch := runtime.GOARCH
	if arch == "x86_64" {
		arch = "amd64"
	}
	if arch == "aarch64" {
		arch = "arm64"
	}
	name := fmt.Sprintf("%s-linux-%s", assetStem, arch)
	if linkage == "static" {
		name += "-static"
	}
	return name
}

// LatestTag resolves the latest release tag for a repository, preferring the
// most recent release (including pre-releases like daily-YYYYMMDD) and falling
// back to the latest formal release.
func (client *Client) LatestTag(ctx context.Context, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=1", repo)
	body, err := client.get(ctx, url)
	if err == nil {
		var releases []struct {
			TagName string `json:"tag_name"`
		}
		if unmarshalErr := json.Unmarshal(body, &releases); unmarshalErr == nil && len(releases) > 0 && releases[0].TagName != "" {
			return releases[0].TagName, nil
		}
	}
	latestURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	latestBody, latestErr := client.get(ctx, latestURL)
	if latestErr != nil {
		return "", fmt.Errorf("resolve latest tag for %s: %w", repo, latestErr)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(latestBody, &release); err != nil {
		return "", fmt.Errorf("parse latest release for %s: %w", repo, err)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("latest release for %s has no tag", repo)
	}
	return release.TagName, nil
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
