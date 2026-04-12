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

	if !strings.Contains(out.String(), "恢复 Xray 到配置版本") {
		t.Fatalf("panel output did not contain restore label: %q", out.String())
	}
	if !strings.Contains(out.String(), "卸载 DTSW") {
		t.Fatalf("panel output did not contain uninstall entry: %q", out.String())
	}
	if !strings.Contains(out.String(), "用户与流量管理") {
		t.Fatalf("panel output did not contain user/traffic entry: %q", out.String())
	}
}

func TestRenderUserPanelContainsStats(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	var out bytes.Buffer
	renderUserPanel(&out, cfg)

	if !strings.Contains(out.String(), "查看全部用户流量统计") {
		t.Fatalf("user panel did not contain all-stats entry: %q", out.String())
	}
	if !strings.Contains(out.String(), "查看单个用户流量统计") {
		t.Fatalf("user panel did not contain single-stats entry: %q", out.String())
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
	if !strings.Contains(text, "用户名:    secondary") {
		t.Fatalf("expected selected username in output, got %q", text)
	}
	if !strings.Contains(text, "密码:      secret-2") {
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
	if !strings.Contains(out.String(), "已取消卸载。") {
		t.Fatalf("expected cancel message, got %q", out.String())
	}
}
