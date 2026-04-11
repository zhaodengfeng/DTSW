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

func TestRenderFallbackServiceUsesCaddyWhenConfigured(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	unit := RenderFallbackService(cfg)
	if !strings.Contains(unit, cfg.Paths.CaddyBinary) {
		t.Fatalf("expected caddy binary path in fallback unit: %s", unit)
	}
	if !strings.Contains(unit, cfg.Paths.CaddyConfigFile) {
		t.Fatalf("expected caddy config path in fallback unit: %s", unit)
	}
	if strings.Contains(unit, "fallback-serve") {
		t.Fatalf("expected caddy-backed fallback service, got: %s", unit)
	}
}
