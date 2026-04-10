package xray

import "testing"

func TestAssetName(t *testing.T) {
	asset, err := AssetName("linux", "amd64")
	if err != nil {
		t.Fatalf("AssetName returned error: %v", err)
	}
	if asset != "Xray-linux-64.zip" {
		t.Fatalf("unexpected asset name: %s", asset)
	}
}

func TestDownloadURL(t *testing.T) {
	url := DownloadURL("v26.1.13", "Xray-linux-64.zip")
	want := "https://github.com/XTLS/Xray-core/releases/download/v26.1.13/Xray-linux-64.zip"
	if url != want {
		t.Fatalf("unexpected download url: %s", url)
	}
}
