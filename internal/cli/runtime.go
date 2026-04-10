package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/install"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
)

func runRuntime(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "runtime requires a subcommand: current, latest, upgrade")
		return 1
	}

	switch args[0] {
	case "current":
		cfg, _, ok := loadConfigFlags("runtime current", args[1:], stderr)
		if !ok {
			return 1
		}
		fmt.Fprintf(stdout, "Configured Runtime: %s %s\n", cfg.Runtime.Type, cfg.Runtime.Version)
		version, err := xray.CurrentVersion(cfg.Paths.XrayBinary)
		if err != nil {
			fmt.Fprintf(stdout, "Installed Runtime: unavailable (%v)\n", err)
			return 0
		}
		fmt.Fprintf(stdout, "Installed Runtime: %s %s\n", cfg.Runtime.Type, version)
		return 0
	case "latest":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		version, err := xray.LatestVersion(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "latest runtime version: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, version)
		return 0
	case "upgrade":
		fs := flag.NewFlagSet("runtime upgrade", flag.ContinueOnError)
		fs.SetOutput(stderr)
		configPath := fs.String("config", config.DefaultPaths().DTSWConfigFile, "Path to config.json")
		version := fs.String("version", "", "Explicit Xray version to install, for example v26.1.13")
		latest := fs.Bool("latest", false, "Install the latest stable Xray release and update the config")
		dryRun := fs.Bool("dry-run", false, "Print actions instead of modifying the host")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		cfg, err := config.Load(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(stderr, "invalid config: %v\n", err)
			return 1
		}
		targetVersion, err := resolveRuntimeTargetVersion(cfg, *version, *latest)
		if err != nil {
			fmt.Fprintf(stderr, "resolve target version: %v\n", err)
			return 1
		}
		if err := upgradeRuntime(context.Background(), *configPath, cfg, targetVersion, install.Options{DryRun: *dryRun, SkipIssue: true, Stdout: stdout, Stderr: stderr}); err != nil {
			fmt.Fprintf(stderr, "upgrade runtime: %v\n", err)
			return 1
		}
		if !*dryRun {
			fmt.Fprintf(stdout, "runtime upgraded to %s\n", targetVersion)
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown runtime subcommand %q\n", args[0])
		return 1
	}
}

func resolveRuntimeTargetVersion(cfg config.Config, explicitVersion string, latest bool) (string, error) {
	if cfg.Runtime.Type != config.RuntimeXray {
		return "", fmt.Errorf("runtime upgrades currently support only xray")
	}
	if explicitVersion != "" && latest {
		return "", fmt.Errorf("choose either --version or --latest")
	}
	if explicitVersion != "" {
		return normalizeRuntimeVersion(explicitVersion)
	}
	if latest {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return xray.LatestVersion(ctx)
	}
	return normalizeRuntimeVersion(cfg.Runtime.Version)
}

func upgradeRuntime(ctx context.Context, sourcePath string, cfg config.Config, targetVersion string, opts install.Options) error {
	normalizedVersion, err := normalizeRuntimeVersion(targetVersion)
	if err != nil {
		return err
	}
	cfg.Runtime.Version = normalizedVersion
	if sourcePath != "" {
		cfg.Paths.DTSWConfigFile = sourcePath
	}
	return install.Execute(ctx, cfg, opts)
}

func normalizeRuntimeVersion(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", fmt.Errorf("runtime version is required")
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return version, nil
}
