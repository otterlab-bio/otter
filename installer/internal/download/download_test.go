package download

import "testing"

func TestAssetName(t *testing.T) {
	testCases := []struct {
		assetStem string
		linkage   string
		expected  string
	}{
		{assetStem: "otter", linkage: "static", expected: "otter-linux-amd64-static"},
		{assetStem: "enva", linkage: "static", expected: "enva-linux-amd64-static"},
		{assetStem: "methx", linkage: "static", expected: "methx-linux-amd64-static"},
		{assetStem: "otter", linkage: "dynamic", expected: "otter-linux-amd64"},
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
