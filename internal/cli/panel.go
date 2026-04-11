package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/install"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
	"github.com/zhaodengfeng/dtsw/internal/systemd"
	"github.com/zhaodengfeng/dtsw/internal/tlscfg"
)

type panelState struct {
	InstalledVersion string
	InstalledError   error
	LatestVersion    string
	LatestError      error
}

func runPanel(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("panel", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", config.DefaultPaths().DTSWConfigFile, "Path to config.json")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	return openPanelSession(*configPath, stdout, stderr)
}

func openPanelSession(configPath string, stdout, stderr io.Writer) int {
	input, cleanup, err := openSetupInput()
	if err != nil {
		fmt.Fprintf(stderr, "panel requires an interactive terminal: %v\n", err)
		return 1
	}
	defer cleanup()
	return runPanelWithInput(configPath, input, stdout, stderr)
}

func runPanelWithInput(configPath string, input io.Reader, stdout, stderr io.Writer) int {
	reader := bufio.NewReader(input)
	for {
		cfg, err := config.Load(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(stderr, "invalid config: %v\n", err)
			return 1
		}
		state := gatherPanelState(cfg)
		renderPanel(stdout, cfg, state)

		choice, err := panelPromptDefault(reader, stdout, "Select an action", "1")
		if err != nil {
			fmt.Fprintf(stderr, "read selection: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "")

		switch strings.TrimSpace(choice) {
		case "0", "q", "quit", "exit":
			fmt.Fprintln(stdout, "Leaving DTSW panel.")
			return 0
		case "1":
			printClientConfiguration(stdout, cfg)
			waitForEnter(reader, stdout)
		case "2":
			printStatusReport(stdout, cfg)
			waitForEnter(reader, stdout)
		case "3":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			if err := install.Execute(context.Background(), cfg, install.Options{Stdout: stdout, Stderr: stderr}); err != nil {
				fmt.Fprintf(stderr, "install or repair: %v\n", err)
			} else {
				fmt.Fprintln(stdout, "Server installation or repair completed.")
				fmt.Fprintln(stdout, "")
				printClientConfiguration(stdout, cfg)
			}
			waitForEnter(reader, stdout)
		case "4":
			if state.LatestError != nil {
				fmt.Fprintf(stderr, "latest xray version: %v\n", state.LatestError)
				waitForEnter(reader, stdout)
				continue
			}
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			if err := upgradeRuntime(context.Background(), configPath, cfg, state.LatestVersion, install.Options{SkipIssue: true, Stdout: stdout, Stderr: stderr}); err != nil {
				fmt.Fprintf(stderr, "upgrade runtime: %v\n", err)
			} else {
				fmt.Fprintf(stdout, "Xray upgraded to %s\n", state.LatestVersion)
			}
			waitForEnter(reader, stdout)
		case "5":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			if err := upgradeRuntime(context.Background(), configPath, cfg, cfg.Runtime.Version, install.Options{SkipIssue: true, Stdout: stdout, Stderr: stderr}); err != nil {
				fmt.Fprintf(stderr, "sync runtime: %v\n", err)
			} else {
				fmt.Fprintf(stdout, "Xray restored to configured state using %s\n", cfg.Runtime.Version)
			}
			waitForEnter(reader, stdout)
		case "6":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			if err := tlscfg.Renew(context.Background(), cfg, tlscfg.Options{Stdout: stdout, Stderr: stderr}); err != nil {
				fmt.Fprintf(stderr, "renew certificate: %v\n", err)
			} else {
				fmt.Fprintln(stdout, "Certificate renewal completed.")
			}
			waitForEnter(reader, stdout)
		case "7":
			openUserPanel(configPath, cfg, reader, stdout, stderr)
		case "8":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			removed, err := uninstallFromPanel(cfg, reader, stdout, stderr)
			if err != nil {
				fmt.Fprintf(stderr, "uninstall: %v\n", err)
				waitForEnter(reader, stdout)
				continue
			}
			if removed {
				fmt.Fprintln(stdout, "DTSW uninstall completed.")
				return 0
			}
			waitForEnter(reader, stdout)
		default:
			fmt.Fprintf(stderr, "unknown selection %q\n", choice)
			waitForEnter(reader, stdout)
		}
	}
}

func gatherPanelState(cfg config.Config) panelState {
	state := panelState{}
	if version, err := xray.CurrentVersion(cfg.Paths.XrayBinary); err == nil {
		state.InstalledVersion = version
	} else {
		state.InstalledError = err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if version, err := xray.LatestVersion(ctx); err == nil {
		state.LatestVersion = version
	} else {
		state.LatestError = err
	}
	return state
}

func renderPanel(stdout io.Writer, cfg config.Config, state panelState) {
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║          DTSW Management Panel       ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "  Domain:            %s\n", cfg.Server.Domain)
	fmt.Fprintf(stdout, "  Configured Xray:   %s\n", cfg.Runtime.Version)
	if state.InstalledError != nil {
		fmt.Fprintf(stdout, "  Installed Xray:    unavailable (%v)\n", state.InstalledError)
	} else {
		fmt.Fprintf(stdout, "  Installed Xray:    %s\n", state.InstalledVersion)
	}
	if state.LatestError != nil {
		fmt.Fprintf(stdout, "  Latest Xray:       unavailable (%v)\n", state.LatestError)
	} else {
		fmt.Fprintf(stdout, "  Latest Xray:       %s\n", state.LatestVersion)
	}
	fmt.Fprintf(stdout, "  Users:             %d\n", len(cfg.Users))
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "  1) Show client configuration")
	fmt.Fprintln(stdout, "  2) Show runtime and certificate status")
	fmt.Fprintln(stdout, "  3) Install or repair this server")
	fmt.Fprintln(stdout, "  4) Upgrade Xray to latest stable")
	fmt.Fprintln(stdout, "  5) Restore Xray to configured state")
	fmt.Fprintln(stdout, "  6) Renew TLS certificate now")
	fmt.Fprintln(stdout, "  7) Manage users")
	fmt.Fprintln(stdout, "  8) Uninstall DTSW")
	fmt.Fprintln(stdout, "  0) Exit")
	fmt.Fprintln(stdout, "")
}

func openUserPanel(configPath string, cfg config.Config, reader *bufio.Reader, stdout, stderr io.Writer) {
	for {
		renderUserPanel(stdout, cfg)
		choice, err := panelPromptDefault(reader, stdout, "Select a user action", "1")
		if err != nil {
			fmt.Fprintf(stderr, "read selection: %v\n", err)
			return
		}
		fmt.Fprintln(stdout, "")

		switch strings.TrimSpace(choice) {
		case "0", "q", "quit", "exit", "back":
			return
		case "1":
			printUserList(stdout, cfg)
			waitForEnter(reader, stdout)
		case "2":
			user, ok, err := selectUserFromConfig(reader, stdout, cfg, "Show configuration for which user")
			if err != nil {
				fmt.Fprintf(stderr, "select user: %v\n", err)
			} else if ok {
				printClientConfigurationForUser(stdout, cfg, user)
			}
			waitForEnter(reader, stdout)
		case "3":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			updated, changed, err := addUserFromPanel(configPath, cfg, reader, stdout, stderr)
			if err != nil {
				fmt.Fprintf(stderr, "add user: %v\n", err)
			} else if changed {
				cfg = updated
			}
			waitForEnter(reader, stdout)
		case "4":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			updated, changed, err := deleteUserFromPanel(configPath, cfg, reader, stdout, stderr)
			if err != nil {
				fmt.Fprintf(stderr, "delete user: %v\n", err)
			} else if changed {
				cfg = updated
			}
			waitForEnter(reader, stdout)
		default:
			fmt.Fprintf(stderr, "unknown selection %q\n", choice)
			waitForEnter(reader, stdout)
		}
	}
}

func renderUserPanel(stdout io.Writer, cfg config.Config) {
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║           DTSW User Manager          ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "  Config path:       %s\n", cfg.Paths.DTSWConfigFile)
	fmt.Fprintf(stdout, "  Total users:       %d\n", len(cfg.Users))
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "  1) List users")
	fmt.Fprintln(stdout, "  2) Show a user's client configuration")
	fmt.Fprintln(stdout, "  3) Add user")
	fmt.Fprintln(stdout, "  4) Delete user")
	fmt.Fprintln(stdout, "  0) Back")
	fmt.Fprintln(stdout, "")
}

func addUserFromPanel(configPath string, cfg config.Config, reader *bufio.Reader, stdout, stderr io.Writer) (config.Config, bool, error) {
	defaultName := fmt.Sprintf("user%d", len(cfg.Users)+1)
	name, err := promptRequiredText(reader, stdout, "User name", defaultName)
	if err != nil {
		return cfg, false, err
	}
	password, err := panelPromptDefault(reader, stdout, "Trojan password", generatePanelPassword())
	if err != nil {
		return cfg, false, err
	}
	if err := cfg.AddUser(name, password); err != nil {
		return cfg, false, err
	}
	if err := savePanelUserChanges(configPath, cfg, stdout, stderr); err != nil {
		return cfg, false, err
	}
	user, _ := cfg.User(name)
	fmt.Fprintf(stdout, "Added user %s and reloaded Xray.\n\n", name)
	printClientConfigurationForUser(stdout, cfg, user)
	return cfg, true, nil
}

func deleteUserFromPanel(configPath string, cfg config.Config, reader *bufio.Reader, stdout, stderr io.Writer) (config.Config, bool, error) {
	user, ok, err := selectUserFromConfig(reader, stdout, cfg, "Delete which user")
	if err != nil || !ok {
		return cfg, false, err
	}
	confirm, err := panelPromptDefault(reader, stdout, fmt.Sprintf("Delete user %s? (y/n)", user.Name), "n")
	if err != nil {
		return cfg, false, err
	}
	if !isAffirmative(confirm) {
		fmt.Fprintln(stdout, "User deletion cancelled.")
		return cfg, false, nil
	}
	if err := cfg.DeleteUser(user.Name); err != nil {
		return cfg, false, err
	}
	if err := savePanelUserChanges(configPath, cfg, stdout, stderr); err != nil {
		return cfg, false, err
	}
	fmt.Fprintf(stdout, "Deleted user %s and reloaded Xray.\n", user.Name)
	return cfg, true, nil
}

func savePanelUserChanges(configPath string, cfg config.Config, stdout, stderr io.Writer) error {
	if configPath != "" {
		cfg.Paths.DTSWConfigFile = configPath
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Paths.DTSWConfigFile), 0o755); err != nil {
		return err
	}
	if err := writeSourceAndRuntimeConfig(cfg.Paths.DTSWConfigFile, cfg, false, stdout, stderr); err != nil {
		return err
	}
	return systemd.Reload(context.Background(), systemd.CommandOptions{Stdout: stdout, Stderr: stderr}, cfg.Paths.RuntimeService)
}

func selectUserFromConfig(reader *bufio.Reader, stdout io.Writer, cfg config.Config, prompt string) (config.User, bool, error) {
	if len(cfg.Users) == 0 {
		fmt.Fprintln(stdout, "No users are configured yet.")
		return config.User{}, false, nil
	}
	fmt.Fprintln(stdout, "Users:")
	for i, user := range cfg.Users {
		fmt.Fprintf(stdout, "  %d) %s\n", i+1, user.Name)
	}
	selection, err := panelPromptDefault(reader, stdout, prompt, "1")
	if err != nil {
		return config.User{}, false, err
	}
	idx, err := strconv.Atoi(strings.TrimSpace(selection))
	if err != nil || idx < 1 || idx > len(cfg.Users) {
		return config.User{}, false, fmt.Errorf("invalid user selection %q", selection)
	}
	return cfg.Users[idx-1], true, nil
}

func promptRequiredText(reader *bufio.Reader, stdout io.Writer, label, defaultValue string) (string, error) {
	for {
		value, err := panelPromptDefault(reader, stdout, label, defaultValue)
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(value)
		if value != "" {
			return value, nil
		}
		fmt.Fprintln(stdout, "    ↳ This field is required.")
	}
}

func generatePanelPassword() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "change-me"
	}
	return hex.EncodeToString(b)
}

func isAffirmative(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "y" || value == "yes"
}

func uninstallFromPanel(cfg config.Config, reader *bufio.Reader, stdout, stderr io.Writer) (bool, error) {
	fmt.Fprintln(stdout, "Uninstall options:")
	fmt.Fprintln(stdout, "  1) Remove services only and keep config, certificates, and binaries")
	fmt.Fprintln(stdout, "  2) Remove services and DTSW-managed data, keep DTSW binary")
	fmt.Fprintln(stdout, "  3) Remove everything including Xray, Caddy, data, and DTSW binary")
	fmt.Fprintln(stdout, "  0) Cancel")

	choice, err := panelPromptDefault(reader, stdout, "Select uninstall mode", "0")
	if err != nil {
		return false, err
	}
	mode := strings.TrimSpace(choice)
	if mode == "0" || strings.EqualFold(mode, "cancel") {
		fmt.Fprintln(stdout, "Uninstall cancelled.")
		return false, nil
	}

	opts := install.RemoveOptions{
		Stdout: stdout,
		Stderr: stderr,
	}
	var confirmLabel string
	switch mode {
	case "1":
		confirmLabel = "Remove DTSW services only? (y/n)"
	case "2":
		opts.PurgeData = true
		confirmLabel = "Remove DTSW services and managed data? (y/n)"
	case "3":
		opts.PurgeData = true
		opts.PurgeXray = true
		opts.PurgeCaddy = true
		opts.RemoveDTSW = true
		confirmLabel = "Remove DTSW completely, including data and binaries? (y/n)"
	default:
		return false, fmt.Errorf("unknown uninstall selection %q", choice)
	}

	confirm, err := panelPromptDefault(reader, stdout, confirmLabel, "n")
	if err != nil {
		return false, err
	}
	if !isAffirmative(confirm) {
		fmt.Fprintln(stdout, "Uninstall cancelled.")
		return false, nil
	}

	if err := install.Remove(context.Background(), cfg, opts); err != nil {
		return false, err
	}
	return true, nil
}

func preferredPanelUser(cfg config.Config) (config.User, bool) {
	if user, ok := cfg.User("primary"); ok {
		return user, true
	}
	if len(cfg.Users) == 0 {
		return config.User{}, false
	}
	return cfg.Users[0], true
}

func waitForEnter(reader *bufio.Reader, stdout io.Writer) {
	fmt.Fprintln(stdout, "")
	fmt.Fprint(stdout, "Press Enter to continue...")
	_, _ = reader.ReadString('\n')
}

func ensureRootForPanelAction() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this action requires root; restart DTSW with root privileges and choose it again")
	}
	return nil
}

func panelPromptDefault(reader *bufio.Reader, stdout io.Writer, label, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(stdout, "  %s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(stdout, "  %s: ", label)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}
