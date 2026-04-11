package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExampleIsValid(t *testing.T) {
	cfg := Example("trojan.example.com", "admin@example.com", "secret")
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid example config, got %v", err)
	}
	if cfg.Fallback.Mode != FallbackCaddyStatic {
		t.Fatalf("expected caddy-static fallback mode, got %s", cfg.Fallback.Mode)
	}
	if cfg.Fallback.SiteRoot == "" {
		t.Fatal("expected site root to be populated")
	}
}

func TestDNSChallengeRequiresProvider(t *testing.T) {
	cfg := Example("trojan.example.com", "admin@example.com", "secret")
	cfg.TLS.Challenge = ChallengeDNS01
	cfg.TLS.DNSProvider = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected dns-01 validation failure when dns_provider is empty")
	}
}

func TestAddUser(t *testing.T) {
	cfg := Example("trojan.example.com", "admin@example.com", "secret")
	if err := cfg.AddUser("secondary", "s3cret"); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if len(cfg.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(cfg.Users))
	}
	user, ok := cfg.User("secondary")
	if !ok {
		t.Fatal("expected to find newly added user")
	}
	if user.Password != "s3cret" {
		t.Fatalf("expected password s3cret, got %q", user.Password)
	}
}

func TestDeleteUser(t *testing.T) {
	cfg := Example("trojan.example.com", "admin@example.com", "secret")
	if err := cfg.AddUser("secondary", "s3cret"); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if err := cfg.DeleteUser("secondary"); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	if _, ok := cfg.User("secondary"); ok {
		t.Fatal("expected user to be removed")
	}
}

func TestDeleteLastUserFails(t *testing.T) {
	cfg := Example("trojan.example.com", "admin@example.com", "secret")
	if err := cfg.DeleteUser("primary"); err == nil {
		t.Fatal("expected deleting the last user to fail")
	}
}

func TestLoadAppliesDefaultsForLegacyBuiltinFallback(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.json")
	data := []byte(`{
  "name": "dtsw",
  "runtime": {"type": "xray", "version": "v26.1.13"},
  "server": {"domain": "trojan.example.com", "listen_host": "0.0.0.0", "port": 443, "alpn": ["h2", "http/1.1"]},
  "tls": {"email": "admin@example.com", "issuer": "letsencrypt", "challenge": "http-01", "renew_before_days": 30},
  "fallback": {"listen_address": "127.0.0.1:8080", "site_title": "Legacy", "site_message": "Legacy page", "status_code": 404},
  "users": [{"name": "primary", "password": "secret"}],
  "paths": {"config_dir": "/etc/dtsw", "data_dir": "/var/lib/dtsw", "runtime_config_file": "/etc/dtsw/xray.json", "dtsw_config_file": "/etc/dtsw/config.json", "dtsw_binary": "/usr/local/bin/dtsw", "xray_binary": "/usr/local/bin/xray", "acme_binary": "/usr/local/bin/acme.sh", "acme_env_file": "/etc/dtsw/acme.env", "systemd_dir": "/etc/systemd/system", "runtime_service": "dtsw-xray.service", "fallback_service": "dtsw-fallback.service", "renew_service": "dtsw-renew.service", "renew_timer": "dtsw-renew.timer"}
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Fallback.Mode != FallbackBuiltin {
		t.Fatalf("expected builtin fallback mode for legacy config, got %s", cfg.Fallback.Mode)
	}
	if cfg.Paths.CaddyBinary == "" || cfg.Paths.CaddyConfigFile == "" {
		t.Fatal("expected Caddy defaults to be populated")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("legacy config should validate after defaults, got %v", err)
	}
}
