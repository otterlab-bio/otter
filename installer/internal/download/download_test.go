package download

import (
	"fmt"
	"runtime"
	"testing"
)

func TestAssetName(t *testing.T) {
	arch := runtime.GOARCH
	if arch == "x86_64" {
		arch = "amd64"
	}
	if arch == "aarch64" {
		arch = "arm64"
	}
	testCases := []struct {
		assetStem string
		linkage   string
		expected  string
	}{
		{assetStem: "otter", linkage: "static", expected: fmt.Sprintf("otter-linux-%s-static", arch)},
		{assetStem: "enva", linkage: "static", expected: fmt.Sprintf("enva-linux-%s-static", arch)},
		{assetStem: "methx", linkage: "static", expected: fmt.Sprintf("methx-linux-%s-static", arch)},
		{assetStem: "otter", linkage: "dynamic", expected: fmt.Sprintf("otter-linux-%s", arch)},
	}
	for _, testCase := range testCases {
		if actual := AssetName(testCase.assetStem, testCase.linkage); actual != testCase.expected {
			t.Fatalf("AssetName(%q, %q) = %q, expected %q", testCase.assetStem, testCase.linkage, actual, testCase.expected)
		}
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
