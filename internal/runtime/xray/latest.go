package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const latestReleaseAPI = "https://api.github.com/repos/XTLS/Xray-core/releases/latest"

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
}

func LatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dtsw")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("query latest xray release failed with %s", resp.Status)
	}
	var release latestReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return normalizeVersion(release.TagName)
}

func normalizeVersion(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", fmt.Errorf("empty version")
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return version, nil
}
