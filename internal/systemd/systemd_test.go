package systemd

import (
	"strings"
	"testing"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func TestRenderXrayServiceUsesConfiguredPaths(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	unit := RenderXrayService(cfg)
	if !strings.Contains(unit, cfg.Paths.XrayBinary) {
		t.Fatalf("expected xray binary path in unit: %s", unit)
	}
	if !strings.Contains(unit, cfg.Paths.RuntimeConfigFile) {
		t.Fatalf("expected runtime config path in unit: %s", unit)
	}
}
