package fallback

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func TestRenderCaddyfileUsesConfiguredSiteRoot(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	rendered := RenderCaddyfile(cfg)
	if !strings.Contains(rendered, cfg.Fallback.ListenAddress) {
		t.Fatalf("expected listen address in caddyfile, got %s", rendered)
	}
	if !strings.Contains(rendered, cfg.Fallback.SiteRoot) {
		t.Fatalf("expected site root in caddyfile, got %s", rendered)
	}
	if !strings.Contains(rendered, "auto_https off") {
		t.Fatalf("expected auto_https off in caddyfile, got %s", rendered)
	}
}

func TestDefaultSiteFilesIncludeHomepage(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	files := DefaultSiteFiles(cfg)
	if _, ok := files["index.html"]; !ok {
		t.Fatal("expected default site to include index.html")
	}
	if _, ok := files["styles.css"]; !ok {
		t.Fatal("expected default site to include styles.css")
	}
	if _, ok := files["journal/index.html"]; !ok {
		t.Fatal("expected default site to include journal page")
	}
}

func TestCurrentVersionParsesCaddyOutput(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "caddy")
	script := "#!/bin/sh\nprintf 'v2.10.2 h1:h2:h3\\n'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake caddy: %v", err)
	}

	version, err := CurrentVersion(path)
	if err != nil {
		t.Fatalf("CurrentVersion returned error: %v", err)
	}
	if version != "v2.10.2" {
		t.Fatalf("CurrentVersion = %q, want %q", version, "v2.10.2")
	}
}
