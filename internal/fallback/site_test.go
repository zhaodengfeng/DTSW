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

func TestCaddyChecksumFromFilePrefersSHA256Entry(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "checksums.txt")
	const asset = "caddy_2.10.2_linux_amd64.tar.gz"
	data := "" +
		"747df7ee74de188485157a383633a1a963fd9233b71fbb4a69ddcbcc589ce4e2cc82dacf5dbbe136cb51d17e14c59daeb5d9bc92487610b0f3b93680b2646546 " + asset + "\n" +
		"5c218bc34c9197369263da7e9317a83acdbd80ef45d94dca5eff76e727c67cdd " + asset + "\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write checksums file: %v", err)
	}

	sum, err := caddyChecksumFromFile(path, asset)
	if err != nil {
		t.Fatalf("caddyChecksumFromFile returned error: %v", err)
	}
	if sum != "5c218bc34c9197369263da7e9317a83acdbd80ef45d94dca5eff76e727c67cdd" {
		t.Fatalf("caddyChecksumFromFile = %q, want sha256 entry", sum)
	}
}
