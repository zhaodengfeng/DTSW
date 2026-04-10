package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func TestFindLauncherStateReady(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	if err := config.Write(configPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	state := findLauncherState([]string{configPath})
	if !state.Ready {
		t.Fatalf("state.Ready = false, want true (problem: %v)", state.Problem)
	}
	if state.ConfigPath != configPath {
		t.Fatalf("state.ConfigPath = %q, want %q", state.ConfigPath, configPath)
	}
	if state.Config.Server.Domain != cfg.Server.Domain {
		t.Fatalf("state.Config.Server.Domain = %q, want %q", state.Config.Server.Domain, cfg.Server.Domain)
	}
}

func TestFindLauncherStateProblem(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	state := findLauncherState([]string{configPath})
	if state.Ready {
		t.Fatal("state.Ready = true, want false")
	}
	if state.Problem == nil {
		t.Fatal("state.Problem = nil, want validation error")
	}
	if state.ConfigPath != configPath {
		t.Fatalf("state.ConfigPath = %q, want %q", state.ConfigPath, configPath)
	}
}
