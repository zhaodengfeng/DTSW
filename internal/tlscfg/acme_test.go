package tlscfg

import (
	"strings"
	"testing"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func TestBuildIssueCommandsHTTP01(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	issue, install, err := BuildIssueCommands(cfg)
	if err != nil {
		t.Fatalf("BuildIssueCommands failed: %v", err)
	}
	if !strings.Contains(strings.Join(issue, " "), "--standalone") {
		t.Fatalf("expected --standalone in issue command: %v", issue)
	}
	if !strings.Contains(strings.Join(install, " "), cfg.TLS.CertificateFile) {
		t.Fatalf("expected certificate path in install command: %v", install)
	}
}
