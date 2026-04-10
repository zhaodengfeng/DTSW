package wizard

import (
	"bytes"
	"strings"
	"testing"
)

func TestWizardProducesValidConfig(t *testing.T) {
	input := strings.Join([]string{
		"trojan.example.com", // domain
		"admin@example.com",  // email
		"",                   // password (accept default)
		"",                   // port (accept 443)
		"",                   // issuer (accept 1)
		"",                   // challenge (accept 1 = http-01)
		"",                   // config path (accept default)
		"n",                  // don't auto-start
	}, "\n") + "\n"

	var stdout, stderr bytes.Buffer
	result, err := Run(strings.NewReader(input), &stdout, &stderr)
	if err != nil {
		t.Fatalf("wizard failed: %v", err)
	}
	if err := result.Config.Validate(); err != nil {
		t.Fatalf("wizard produced invalid config: %v", err)
	}
	if result.Config.Server.Domain != "trojan.example.com" {
		t.Fatalf("unexpected domain: %s", result.Config.Server.Domain)
	}
	if result.AutoStart {
		t.Fatal("expected AutoStart to be false")
	}
}

func TestWizardDNS01RequiresProvider(t *testing.T) {
	input := strings.Join([]string{
		"trojan.example.com", // domain
		"admin@example.com",  // email
		"mypassword",         // password
		"443",                // port
		"1",                  // issuer
		"2",                  // challenge = dns-01
		"dns_cf",             // dns provider
		"/etc/dtsw/cfg.json", // config path
		"y",                  // auto-start
	}, "\n") + "\n"

	var stdout, stderr bytes.Buffer
	result, err := Run(strings.NewReader(input), &stdout, &stderr)
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
