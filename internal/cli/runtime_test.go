package cli

import (
	"testing"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func TestNormalizeRuntimeVersion(t *testing.T) {
	got, err := normalizeRuntimeVersion("26.1.13")
	if err != nil {
		t.Fatalf("normalizeRuntimeVersion returned error: %v", err)
	}
	if got != "v26.1.13" {
		t.Fatalf("normalizeRuntimeVersion = %q, want %q", got, "v26.1.13")
	}
}

func TestPreferredPanelUser(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	user, ok := preferredPanelUser(cfg)
	if !ok {
		t.Fatal("preferredPanelUser returned false")
	}
	if user.Name != "primary" {
		t.Fatalf("preferredPanelUser = %q, want primary", user.Name)
	}
}
