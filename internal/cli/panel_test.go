package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func TestRenderPanelUsesRestoreLabel(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	state := panelState{
		InstalledVersion: "v1.0.0",
		LatestVersion:    "v1.1.0",
	}

	var out bytes.Buffer
	renderPanel(&out, cfg, state)

	if !strings.Contains(out.String(), "Restore Xray to configured state") {
		t.Fatalf("panel output did not contain updated restore label: %q", out.String())
	}
	if strings.Contains(out.String(), "Sync installed Xray to configured version") {
		t.Fatalf("panel output still contains old sync label: %q", out.String())
	}
	if !strings.Contains(out.String(), "Uninstall DTSW") {
		t.Fatalf("panel output did not contain uninstall entry: %q", out.String())
	}
}

func TestSelectUserFromConfigReturnsSelectedUser(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	if err := cfg.AddUser("secondary", "secret-2"); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}

	reader := bufio.NewReader(strings.NewReader("2\n"))
	var out bytes.Buffer
	user, ok, err := selectUserFromConfig(reader, &out, cfg, "Select user")
	if err != nil {
		t.Fatalf("selectUserFromConfig returned error: %v", err)
	}
	if !ok {
		t.Fatal("selectUserFromConfig returned ok=false")
	}
	if user.Name != "secondary" {
		t.Fatalf("expected secondary user, got %s", user.Name)
	}
}

func TestSelectUserFromConfigRejectsInvalidSelection(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")

	reader := bufio.NewReader(strings.NewReader("8\n"))
	var out bytes.Buffer
	_, _, err := selectUserFromConfig(reader, &out, cfg, "Select user")
	if err == nil {
		t.Fatal("expected invalid selection error")
	}
}

func TestPrintClientConfigurationForUserUsesSelectedUser(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	user := config.User{Name: "secondary", Password: "secret-2"}

	var out bytes.Buffer
	ok := printClientConfigurationForUser(&out, cfg, user)
	if !ok {
		t.Fatal("printClientConfigurationForUser returned false")
	}
	text := out.String()
	if !strings.Contains(text, "Username:  secondary") {
		t.Fatalf("expected selected username in output, got %q", text)
	}
	if !strings.Contains(text, "Password:  secret-2") {
		t.Fatalf("expected selected password in output, got %q", text)
	}
}

func TestUninstallFromPanelCancel(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	reader := bufio.NewReader(strings.NewReader("0\n"))
	var out, errOut bytes.Buffer
	removed, err := uninstallFromPanel(cfg, reader, &out, &errOut)
	if err != nil {
		t.Fatalf("uninstallFromPanel returned error: %v", err)
	}
	if removed {
		t.Fatal("expected uninstall to be cancelled")
	}
	if !strings.Contains(out.String(), "Uninstall cancelled.") {
		t.Fatalf("expected cancel message, got %q", out.String())
	}
}
