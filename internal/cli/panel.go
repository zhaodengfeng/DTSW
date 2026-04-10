package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/install"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
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

	input, cleanup, err := openSetupInput()
	if err != nil {
		fmt.Fprintf(stderr, "panel requires an interactive terminal: %v\n", err)
		return 1
	}
	defer cleanup()

	reader := bufio.NewReader(input)
	for {
		cfg, err := config.Load(*configPath)
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
			if err := upgradeRuntime(context.Background(), *configPath, cfg, state.LatestVersion, install.Options{SkipIssue: true, Stdout: stdout, Stderr: stderr}); err != nil {
				fmt.Fprintf(stderr, "upgrade runtime: %v\n", err)
			} else {
				fmt.Fprintf(stdout, "Xray upgraded to %s\n", state.LatestVersion)
			}
			waitForEnter(reader, stdout)
		case "2":
			if err := ensureRootForPanelAction(); err != nil {
				fmt.Fprintln(stderr, err)
				waitForEnter(reader, stdout)
				continue
			}
			if err := upgradeRuntime(context.Background(), *configPath, cfg, cfg.Runtime.Version, install.Options{SkipIssue: true, Stdout: stdout, Stderr: stderr}); err != nil {
				fmt.Fprintf(stderr, "sync runtime: %v\n", err)
			} else {
				fmt.Fprintf(stdout, "Runtime synced to configured version %s\n", cfg.Runtime.Version)
			}
			waitForEnter(reader, stdout)
		case "3":
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
		case "4":
			user, ok := preferredPanelUser(cfg)
			if !ok {
				fmt.Fprintln(stderr, "no users found in the config")
			} else {
				fmt.Fprintln(stdout, trojanURL(cfg, user))
			}
			waitForEnter(reader, stdout)
		case "5":
			for _, user := range cfg.Users {
				fmt.Fprintf(stdout, "- %s\n", user.Name)
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
	fmt.Fprintln(stdout, "  1) Upgrade Xray to latest stable")
	fmt.Fprintln(stdout, "  2) Sync installed Xray to configured version")
	fmt.Fprintln(stdout, "  3) Renew TLS certificate now")
	fmt.Fprintln(stdout, "  4) Show primary Trojan URL")
	fmt.Fprintln(stdout, "  5) List users")
	fmt.Fprintln(stdout, "  0) Exit")
	fmt.Fprintln(stdout, "")
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
		return fmt.Errorf("this action requires root; rerun the panel with sudo")
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
