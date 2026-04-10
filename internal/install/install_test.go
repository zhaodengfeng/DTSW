package install

import (
	"bytes"
	"context"
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
