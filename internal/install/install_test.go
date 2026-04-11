package install

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func TestExecuteDryRunPrintsInstallActions(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	cfg.Paths.DTSWBinary = "/usr/local/bin/dtsw"
	var out bytes.Buffer
	if err := Execute(context.Background(), cfg, Options{DryRun: true, Stdout: &out, Stderr: &out}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"install ",
		acmeDownloadURL(config.DefaultACMEShVersion),
		"download https://github.com/caddyserver/caddy",
		"write /etc/dtsw/Caddyfile",
		"write /var/lib/dtsw/site/index.html",
		"download https://github.com/XTLS/Xray-core",
		"write /etc/dtsw/config.json",
		"systemctl daemon-reload",
		"systemctl enable dtsw-fallback.service dtsw-xray.service dtsw-renew.timer",
		"systemctl restart-or-start dtsw-xray.service",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, text)
		}
	}
}

func TestWriteDefaultSiteRepairsMissingFiles(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	cfg.Fallback.SiteRoot = filepath.Join(t.TempDir(), "site")

	if err := os.MkdirAll(cfg.Fallback.SiteRoot, 0o755); err != nil {
		t.Fatalf("mkdir site root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Fallback.SiteRoot, "styles.css"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	if err := writeDefaultSite(cfg, Options{}); err != nil {
		t.Fatalf("writeDefaultSite returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfg.Fallback.SiteRoot, "index.html")); err != nil {
		t.Fatalf("expected index.html to be restored: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(cfg.Fallback.SiteRoot, "styles.css"))
	if err != nil {
		t.Fatalf("read styles.css: %v", err)
	}
	if string(data) != "existing" {
		t.Fatalf("expected existing file to stay untouched, got %q", string(data))
	}
}
