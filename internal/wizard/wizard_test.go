package wizard

import (
	"bytes"
	"strings"
	"testing"
)

func TestWizardProducesValidConfig(t *testing.T) {
	input := strings.Join([]string{
		"trojan.example.com",
		"admin@example.com",
		"",
		"",
		"",
		"",
		"",
		"n",
	}, "\n") + "\n"

	var stdout, stderr bytes.Buffer
	result, err := run(strings.NewReader(input), &stdout, &stderr, true)
	if err != nil {
		t.Fatalf("wizard failed: %v", err)
	}
	if err := result.Config.Validate(); err != nil {
		t.Fatalf("wizard produced invalid config: %v", err)
	}
	if result.Config.Server.Domain != "trojan.example.com" {
		t.Fatalf("unexpected domain: %s", result.Config.Server.Domain)
	}
	if result.ConfigPath != "/etc/dtsw/config.json" {
		t.Fatalf("unexpected config path: %s", result.ConfigPath)
	}
	if result.AutoStart {
		t.Fatal("expected AutoStart to be false")
	}
}

func TestWizardNonRootDefaultsToLocalConfigPath(t *testing.T) {
	input := strings.Join([]string{
		"trojan.example.com",
		"admin@example.com",
		"mypassword",
		"443",
		"1",
		"1",
		"",
		"",
	}, "\n") + "\n"

	var stdout, stderr bytes.Buffer
	result, err := run(strings.NewReader(input), &stdout, &stderr, false)
	if err != nil {
		t.Fatalf("wizard failed: %v", err)
	}
	if result.ConfigPath != "./dtsw.config.json" {
		t.Fatalf("expected local config path, got %s", result.ConfigPath)
	}
	if result.AutoStart {
		t.Fatal("expected AutoStart to default to false for non-root setup")
	}
}

func TestWizardDNS01RequiresProvider(t *testing.T) {
	input := strings.Join([]string{
		"trojan.example.com",
		"admin@example.com",
		"mypassword",
		"443",
		"1",
		"2",
		"dns_cf",
		"/etc/dtsw/cfg.json",
		"y",
	}, "\n") + "\n"

	var stdout, stderr bytes.Buffer
	result, err := run(strings.NewReader(input), &stdout, &stderr, true)
	if err != nil {
		t.Fatalf("wizard failed: %v", err)
	}
	if result.Config.TLS.Challenge != "dns-01" {
		t.Fatalf("expected dns-01 challenge, got %s", result.Config.TLS.Challenge)
	}
	if result.Config.TLS.DNSProvider != "dns_cf" {
		t.Fatalf("expected dns_cf provider, got %s", result.Config.TLS.DNSProvider)
	}
	if !result.AutoStart {
		t.Fatal("expected AutoStart to be true")
	}
}
