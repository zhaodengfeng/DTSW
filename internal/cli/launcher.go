package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/install"
)

type launcherState struct {
	ConfigPath string
	Config     config.Config
	Ready      bool
	Problem    error
}

func runLauncher(stdout, stderr io.Writer) int {
	input, cleanup, err := openSetupInput()
	if err != nil {
		printUsage(stdout)
		return 0
	}
	defer cleanup()

	reader := bufio.NewReader(input)
	for {
		state := findLauncherState(defaultLauncherConfigPaths())
		renderLauncher(stdout, state)

		choice, err := panelPromptDefault(reader, stdout, "Select an action", launcherDefaultChoice(state))
		if err != nil {
			fmt.Fprintf(stderr, "read selection: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "")

		switch {
		case state.Ready:
			switch strings.TrimSpace(choice) {
			case "0", "q", "quit", "exit":
				fmt.Fprintln(stdout, "Leaving DTSW.")
				return 0
			case "1":
				return runPanelWithInput(state.ConfigPath, reader, stdout, stderr)
			case "2":
				if os.Geteuid() != 0 {
					fmt.Fprintln(stdout, "This action needs root. Restart DTSW with root privileges and choose this option again.")
					waitForEnter(reader, stdout)
					continue
				}
				if err := install.Execute(context.Background(), state.Config, install.Options{Stdout: stdout, Stderr: stderr}); err != nil {
					fmt.Fprintf(stderr, "install or repair: %v\n", err)
					waitForEnter(reader, stdout)
					continue
				}
				fmt.Fprintln(stdout, "Server installation or repair completed.")
				fmt.Fprintln(stdout, "")
				printClientConfiguration(stdout, state.Config)
				fmt.Fprintln(stdout, "")
				fmt.Fprintln(stdout, "Opening the DTSW management panel...")
				return runPanelWithInput(state.ConfigPath, reader, stdout, stderr)
			case "3":
				printClientConfiguration(stdout, state.Config)
				waitForEnter(reader, stdout)
			case "4":
				return runSetup(stdout, stderr)
			default:
				fmt.Fprintf(stderr, "unknown selection %q\n", choice)
				waitForEnter(reader, stdout)
			}
		default:
			switch strings.TrimSpace(choice) {
			case "0", "q", "quit", "exit":
				fmt.Fprintln(stdout, "Leaving DTSW.")
				return 0
			case "1":
				return runSetup(stdout, stderr)
			default:
				fmt.Fprintf(stderr, "unknown selection %q\n", choice)
				waitForEnter(reader, stdout)
			}
		}
	}
}

func renderLauncher(stdout io.Writer, state launcherState) {
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║         DTSW Interactive Menu        ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")

	if state.Ready {
		fmt.Fprintf(stdout, "  Saved configuration: %s\n", state.ConfigPath)
		fmt.Fprintf(stdout, "  Domain:              %s\n", state.Config.Server.Domain)
		fmt.Fprintf(stdout, "  Users:               %d\n", len(state.Config.Users))
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "  1) Open management panel")
		fmt.Fprintln(stdout, "  2) Install or repair with the saved configuration")
		fmt.Fprintln(stdout, "  3) Show client configuration")
		fmt.Fprintln(stdout, "  4) Run guided setup again")
		fmt.Fprintln(stdout, "  0) Exit")
		fmt.Fprintln(stdout, "")
		return
	}

	if state.Problem != nil {
		fmt.Fprintf(stdout, "  Saved configuration found at %s, but it needs attention:\n", state.ConfigPath)
		fmt.Fprintf(stdout, "  %v\n", state.Problem)
	} else {
		fmt.Fprintln(stdout, "  No saved DTSW configuration was found yet.")
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "  1) Start guided setup")
	fmt.Fprintln(stdout, "  0) Exit")
	fmt.Fprintln(stdout, "")
}

func launcherDefaultChoice(state launcherState) string {
	return "1"
}

func defaultLauncherConfigPaths() []string {
	paths := []string{config.DefaultPaths().DTSWConfigFile}
	if local, err := filepath.Abs("./dtsw.config.json"); err == nil {
		if local != paths[0] {
			paths = append(paths, local)
		}
	} else {
		paths = append(paths, "./dtsw.config.json")
	}
	return paths
}

func findLauncherState(paths []string) launcherState {
	for _, path := range paths {
		cfg, err := config.Load(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return launcherState{ConfigPath: path, Problem: err}
		}
		if err := cfg.Validate(); err != nil {
			return launcherState{ConfigPath: path, Problem: err}
		}
		return launcherState{ConfigPath: path, Config: cfg, Ready: true}
	}
	return launcherState{}
}
