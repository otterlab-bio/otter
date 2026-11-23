package download

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPlatformAssetNames(t *testing.T) {
	for _, test := range []struct{ os, arch, expected string }{
		{"linux", "amd64", "otter-linux-amd64-static"},
		{"linux", "arm64", "otter-linux-arm64-static"},
		{"darwin", "amd64", "otter-darwin-amd64"},
		{"darwin", "arm64", "otter-darwin-arm64"},
	} {
		p, err := NewPlatform(test.os, test.arch)
		if err != nil || p.AssetName("otter") != test.expected {
			t.Errorf("NewPlatform(%s, %s) = %v, %v; expected %s", test.os, test.arch, p, err, test.expected)
		}
	}
	for _, test := range [][2]string{{"windows", "amd64"}, {"darwin", "386"}, {"linux", "arm"}} {
		if _, err := NewPlatform(test[0], test[1]); err == nil {
			t.Errorf("accepted unsupported platform %v", test)
		}
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFindRelease(t *testing.T) {
	const incompatible = `{"tag_name":"v2.0.0","assets":[{"name":"otter-linux-amd64-static"}]}`
	const complete = `{"tag_name":"v1.0.0","assets":[{"name":"otter-darwin-arm64"},{"name":"craftmake-darwin-arm64","url":"https://api.github.com/repos/owner/repo/releases/assets/1"}]}`
	client := NewClient("", "", false)
	client.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Path {
		case "/repos/owner/repo/releases":
			body = `[{"tag_name":"draft","draft":true,"assets":[{"name":"otter-darwin-arm64"},{"name":"craftmake-darwin-arm64"}]},` + incompatible + `,{"tag_name":"partial","assets":[{"name":"otter-darwin-arm64"}]},` + complete + `]`
		case "/repos/owner/repo/releases/tags/v2.0.0":
			body = incompatible
		default:
			t.Fatalf("unexpected request: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	assets := []string{"otter-darwin-arm64", "craftmake-darwin-arm64"}
	release, err := client.FindRelease(context.Background(), "owner/repo", "latest", assets)
	if err != nil || release.Tag != "v1.0.0" || release.AssetAPIURL(assets[1]) == "" {
		t.Fatalf("expected complete release with asset URL; got %+v, %v", release, err)
	}
	if _, err = client.FindRelease(context.Background(), "owner/repo", "v2.0.0", assets); err == nil || !strings.Contains(err.Error(), "missing assets: otter-darwin-arm64") {
		t.Fatalf("expected missing-platform error, got %v", err)
	}
}

func TestReleaseAssetURL(t *testing.T) {
	expected := "https://github.com/owner/repo/releases/download/v1.2.3/otter-linux-amd64-static"
	if actual := ReleaseAssetURL("owner/repo", "v1.2.3", "otter-linux-amd64-static"); actual != expected {
		t.Fatalf("ReleaseAssetURL = %q, expected %q", actual, expected)
	}
}

func TestApplyProxy(t *testing.T) {
	client := NewClient("", "https://proxy.example/", false)
	url := "https://github.com/owner/repo/releases/download/v1/asset"
	expected := "https://proxy.example/https://github.com/owner/repo/releases/download/v1/asset"
	if actual := client.ApplyProxy(url); actual != expected {
		t.Fatalf("ApplyProxy = %q, expected %q", actual, expected)
	}

	noProxy := NewClient("", "", false)
	if actual := noProxy.ApplyProxy(url); actual != url {
		t.Fatalf("ApplyProxy without proxy should return the original URL, got %q", actual)
	}
}
