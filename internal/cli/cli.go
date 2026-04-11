package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/doctor"
	"github.com/zhaodengfeng/dtsw/internal/fallback"
	"github.com/zhaodengfeng/dtsw/internal/install"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
	"github.com/zhaodengfeng/dtsw/internal/systemd"
	"github.com/zhaodengfeng/dtsw/internal/tlscfg"
	"github.com/zhaodengfeng/dtsw/internal/wizard"
)

const Version = "0.2.9"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		return runLauncher(stdout, stderr)
	}

	switch args[1] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "init":
		return runInit(args[2:], stdout, stderr)
	case "setup":
		return runSetup(stdout, stderr)
	case "panel":
		return runPanel(args[2:], stdout, stderr)
	case "validate":
		return runValidate(args[2:], stdout, stderr)
	case "render":
		return runRender(args[2:], stdout, stderr)
	case "plan":
		return runPlan(args[2:], stdout, stderr)
	case "install":
		return runInstall(args[2:], stdout, stderr)
	case "runtime":
		return runRuntime(args[2:], stdout, stderr)
	case "status":
		return runStatus(args[2:], stdout, stderr)
	case "doctor":
		return runDoctor(args[2:], stdout, stderr)
	case "users":
		return runUsers(args[2:], stdout, stderr)
	case "fallback-serve":
		return runFallbackServe(args[2:], stdout, stderr)
	case "tls":
		return runTLS(args[2:], stdout, stderr)
	case "issue":
		return runIssue(args[2:], stdout, stderr)
	case "renew":
		return runRenew(args[2:], stdout, stderr)
	case "uninstall":
		return runUninstall(args[2:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[1])
		printUsage(stderr)
		return 1
	}
}

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	domain := fs.String("domain", "trojan.example.com", "Fully-qualified Trojan domain")
	email := fs.String("email", "admin@example.com", "ACME account email")
	password := fs.String("password", "change-me", "Initial Trojan password")
	out := fs.String("out", "configs/dtsw.example.json", "Where to write the generated config")
	overwrite := fs.Bool("overwrite", false, "Overwrite the output file if it already exists")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !*overwrite {
		if _, err := os.Stat(*out); err == nil {
			fmt.Fprintf(stderr, "%s already exists; rerun with --overwrite to replace it\n", *out)
			return 1
		}
	}
	runtimeVersion, runtimeNote := resolveInitialRuntimeVersion()
	cfg := config.ExampleWithVersion(*domain, *email, *password, runtimeVersion)
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "generated config is invalid: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintf(stderr, "create output directory: %v\n", err)
		return 1
	}
	if err := config.Write(*out, cfg); err != nil {
		fmt.Fprintf(stderr, "write config: %v\n", err)
		return 1
	}
	if runtimeNote != "" {
		fmt.Fprintln(stdout, runtimeNote)
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	return 0
}

func runSetup(stdout, stderr io.Writer) int {
	input, cleanup, err := openSetupInput()
	if err != nil {
		fmt.Fprintf(stderr, "setup requires an interactive terminal: %v\n", err)
		return 1
	}
	defer cleanup()

	result, err := wizard.Run(input, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "setup wizard failed: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(result.ConfigPath), 0o755); err != nil {
		fmt.Fprintf(stderr, "create config directory: %v\n", err)
		return 1
	}
	if err := config.Write(result.ConfigPath, result.Config); err != nil {
		fmt.Fprintf(stderr, "write config: %v\n", err)
		return 1
	}

	displayConfigPath := result.ConfigPath
	if abs, err := filepath.Abs(result.ConfigPath); err == nil {
		displayConfigPath = abs
	}
	fmt.Fprintf(stdout, "\n✓ Configuration saved to %s\n", displayConfigPath)

	if !result.AutoStart {
		fmt.Fprintln(stdout, "\nConfiguration saved.")
		fmt.Fprintln(stdout, "Restart DTSW later and choose the install or repair option from the menu.")
		return 0
	}

	if os.Geteuid() != 0 {
		fmt.Fprintln(stdout, "\nAutomatic installation requires root.")
		fmt.Fprintln(stdout, "Restart DTSW as root later and choose the install or repair option from the menu.")
		return 0
	}

	fmt.Fprintln(stdout, "\nStarting installation...")
	fmt.Fprintln(stdout, "")
	if err := install.Execute(context.Background(), result.Config, install.Options{
		Stdout: stdout,
		Stderr: stderr,
	}); err != nil {
		fmt.Fprintf(stderr, "install failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "✓ Installation completed successfully!")
	return completeInstallFlow(displayConfigPath, result.Config, stdout, stderr)
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	cfg, _, ok := loadConfigFlags("validate", args, stderr)
	if !ok {
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "invalid config: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "config is valid")
	return 0
}

func runRender(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "render requires a target: xray or systemd")
		return 1
	}

	switch args[0] {
	case "xray":
		cfg, _, ok := loadConfigFlags("render xray", args[1:], stderr)
		if !ok {
			return 1
		}
		renderer := xray.Renderer{}
		data, err := renderer.Render(cfg)
		if err != nil {
			fmt.Fprintf(stderr, "render xray: %v\n", err)
			return 1
		}
		_, _ = stdout.Write(data)
		return 0
	case "systemd":
		fs := flag.NewFlagSet("render systemd", flag.ContinueOnError)
		fs.SetOutput(stderr)
		configPath := fs.String("config", "", "Path to config.json")
		unit := fs.String("unit", "xray", "Unit to render: xray, fallback, renew-service, renew-timer")
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
		switch *unit {
		case "xray":
			fmt.Fprint(stdout, systemd.RenderXrayService(cfg))
		case "fallback":
			fmt.Fprint(stdout, systemd.RenderFallbackService(cfg))
		case "renew-service":
			fmt.Fprint(stdout, systemd.RenderRenewService(cfg))
		case "renew-timer":
			fmt.Fprint(stdout, systemd.RenderRenewTimer(cfg))
		default:
			fmt.Fprintf(stderr, "unknown systemd unit %q\n", *unit)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown render target %q\n", args[0])
		return 1
	}
}

func runPlan(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "install" {
		fmt.Fprintln(stderr, "plan requires the subcommand install")
		return 1
	}
	cfg, _, ok := loadConfigFlags("plan install", args[1:], stderr)
	if !ok {
		return 1
	}
	fmt.Fprint(stdout, install.Render(install.Build(cfg)))
	return 0
}

func runInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to config.json")
	dryRun := fs.Bool("dry-run", false, "Print installation actions instead of running them")
	skipIssue := fs.Bool("skip-issue", false, "Skip the initial certificate issuance step")
	skipEnable := fs.Bool("skip-enable", false, "Write files but do not enable/start services")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	if err := install.Execute(context.Background(), cfg, install.Options{
		DryRun:     *dryRun,
		SkipIssue:  *skipIssue,
		SkipEnable: *skipEnable,
		Stdout:     stdout,
		Stderr:     stderr,
	}); err != nil {
		fmt.Fprintf(stderr, "install failed: %v\n", err)
		return 1
	}
	if !*dryRun {
		fmt.Fprintln(stdout, "install completed")
		return completeInstallFlow(*configPath, cfg, stdout, stderr)
	}
	return 0
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	cfg, _, ok := loadConfigFlags("status", args, stderr)
	if !ok {
		return 1
	}
	printStatusReport(stdout, cfg)
	return 0
}
func runDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to config.json")
	expectedIP := fs.String("expected-ip", "", "Warn if the domain does not resolve to this address")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	results := doctor.Run(cfg, *expectedIP)
	exitCode := 0
	for _, result := range results {
		fmt.Fprintf(stdout, "[%s] %s: %s\n", result.Severity, result.Title, result.Detail)
		if result.Severity == doctor.SeverityFail {
			exitCode = 1
		}
	}
	return exitCode
}

func runUsers(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "users requires a subcommand: list, add, del, url")
		return 1
	}
	switch args[0] {
	case "list":
		cfg, _, ok := loadConfigFlags("users list", args[1:], stderr)
		if !ok {
			return 1
		}
		for _, user := range cfg.Users {
			fmt.Fprintf(stdout, "%s\n", user.Name)
		}
		return 0
	case "add":
		fs := flag.NewFlagSet("users add", flag.ContinueOnError)
		fs.SetOutput(stderr)
		configPath := fs.String("config", "", "Path to config.json")
		name := fs.String("name", "", "User name")
		password := fs.String("password", "", "Trojan password")
		dryRun := fs.Bool("dry-run", false, "Print actions instead of writing files")
		reload := fs.Bool("reload", true, "Reload the runtime service after updating the config")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		cfg, err := config.Load(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		if err := cfg.AddUser(*name, *password); err != nil {
			fmt.Fprintf(stderr, "add user: %v\n", err)
			return 1
		}
		if err := writeSourceAndRuntimeConfig(*configPath, cfg, *dryRun, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "write config: %v\n", err)
			return 1
		}
		if *reload {
			if err := systemd.Reload(context.Background(), systemd.CommandOptions{DryRun: *dryRun, Stdout: stdout, Stderr: stderr}, cfg.Paths.RuntimeService); err != nil {
				fmt.Fprintf(stderr, "reload runtime: %v\n", err)
				return 1
			}
		}
		fmt.Fprintf(stdout, "added user %s\n", *name)
		return 0
	case "del":
		fs := flag.NewFlagSet("users del", flag.ContinueOnError)
		fs.SetOutput(stderr)
		configPath := fs.String("config", "", "Path to config.json")
		name := fs.String("name", "", "User name")
		dryRun := fs.Bool("dry-run", false, "Print actions instead of writing files")
		reload := fs.Bool("reload", true, "Reload the runtime service after updating the config")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		cfg, err := config.Load(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		if err := cfg.DeleteUser(*name); err != nil {
			fmt.Fprintf(stderr, "delete user: %v\n", err)
			return 1
		}
		if err := writeSourceAndRuntimeConfig(*configPath, cfg, *dryRun, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "write config: %v\n", err)
			return 1
		}
		if *reload {
			if err := systemd.Reload(context.Background(), systemd.CommandOptions{DryRun: *dryRun, Stdout: stdout, Stderr: stderr}, cfg.Paths.RuntimeService); err != nil {
				fmt.Fprintf(stderr, "reload runtime: %v\n", err)
				return 1
			}
		}
		fmt.Fprintf(stdout, "deleted user %s\n", *name)
		return 0
	case "url":
		fs := flag.NewFlagSet("users url", flag.ContinueOnError)
		fs.SetOutput(stderr)
		configPath := fs.String("config", "", "Path to config.json")
		name := fs.String("name", "", "User name")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		cfg, err := config.Load(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		user, ok := cfg.User(*name)
		if !ok {
			fmt.Fprintf(stderr, "user %q not found\n", *name)
			return 1
		}
		fmt.Fprintln(stdout, trojanURL(cfg, user))
		return 0
	default:
		fmt.Fprintf(stderr, "unknown users subcommand %q\n", args[0])
		return 1
	}
}

func runFallbackServe(args []string, stdout, stderr io.Writer) int {
	cfg, _, ok := loadConfigFlags("fallback-serve", args, stderr)
	if !ok {
		return 1
	}
	fmt.Fprintf(stdout, "starting fallback server on %s\n", cfg.Fallback.ListenAddress)
	if err := fallback.Serve(context.Background(), cfg); err != nil {
		fmt.Fprintf(stderr, "fallback server: %v\n", err)
		return 1
	}
	return 0
}

func runTLS(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "issuers" {
		fmt.Fprintln(stderr, "tls requires the subcommand issuers")
		return 1
	}
	for _, issuer := range tlscfg.SupportedIssuers() {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", issuer.ID, issuer.DisplayName, issuer.Notes)
	}
	return 0
}

func runIssue(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("issue", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to config.json")
	dryRun := fs.Bool("dry-run", false, "Print the ACME commands instead of running them")
	if err := fs.Parse(args); err != nil {
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
	if err := tlscfg.Issue(context.Background(), cfg, tlscfg.Options{DryRun: *dryRun, Stdout: stdout, Stderr: stderr}); err != nil {
		fmt.Fprintf(stderr, "issue certificate: %v\n", err)
		return 1
	}
	return 0
}

func runRenew(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("renew", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to config.json")
	dryRun := fs.Bool("dry-run", false, "Print the ACME commands instead of running them")
	if err := fs.Parse(args); err != nil {
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
	if err := tlscfg.Renew(context.Background(), cfg, tlscfg.Options{DryRun: *dryRun, Stdout: stdout, Stderr: stderr}); err != nil {
		fmt.Fprintf(stderr, "renew certificate: %v\n", err)
		return 1
	}
	return 0
}

func runUninstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to config.json")
	dryRun := fs.Bool("dry-run", false, "Print actions instead of removing files")
	purgeData := fs.Bool("purge-data", false, "Remove /etc/dtsw and /var/lib/dtsw content")
	purgeXray := fs.Bool("purge-xray", false, "Remove the Xray binary")
	removeDTSW := fs.Bool("remove-dtsw", false, "Remove the DTSW binary")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	if err := install.Remove(context.Background(), cfg, install.RemoveOptions{DryRun: *dryRun, PurgeData: *purgeData, PurgeXray: *purgeXray, RemoveDTSW: *removeDTSW, Stdout: stdout, Stderr: stderr}); err != nil {
		fmt.Fprintf(stderr, "uninstall failed: %v\n", err)
		return 1
	}
	return 0
}

func loadConfigFlags(name string, args []string, stderr io.Writer) (config.Config, string, bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "Path to config.json")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, "", false
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return config.Config{}, "", false
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "invalid config: %v\n", err)
		return config.Config{}, "", false
	}
	return cfg, *configPath, true
}

func writeSourceAndRuntimeConfig(sourcePath string, cfg config.Config, dryRun bool, stdout, stderr io.Writer) error {
	if dryRun {
		fmt.Fprintf(stdout, "write %s\n", sourcePath)
	} else {
		if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
			return err
		}
		if err := config.Write(sourcePath, cfg); err != nil {
			return err
		}
	}
	renderer := xray.Renderer{}
	data, err := renderer.Render(cfg)
	if err != nil {
		return err
	}
	if dryRun {
		fmt.Fprintf(stdout, "write %s\n", cfg.Paths.RuntimeConfigFile)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Paths.RuntimeConfigFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cfg.Paths.RuntimeConfigFile, data, 0o644)
}

func trojanURL(cfg config.Config, user config.User) string {
	u := &url.URL{
		Scheme: "trojan",
		User:   url.User(user.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Server.Domain, cfg.Server.Port),
	}
	q := u.Query()
	q.Set("security", "tls")
	q.Set("sni", cfg.Server.Domain)
	q.Set("alpn", strings.Join(cfg.Server.ALPN, ","))
	u.RawQuery = q.Encode()
	u.Fragment = user.Name
	return u.String()
}

func openSetupInput() (io.Reader, func(), error) {
	if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return os.Stdin, func() {}, nil
	}
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return nil, nil, err
	}
	return tty, func() { _ = tty.Close() }, nil
}

func resolveInitialRuntimeVersion() (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	version, err := xray.LatestVersion(ctx)
	if err != nil {
		return config.DefaultXrayVersion, fmt.Sprintf("Latest Xray lookup failed, using bundled fallback %s.", config.DefaultXrayVersion)
	}
	return version, fmt.Sprintf("Latest stable Xray %s selected and written into the config.", version)
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, strings.TrimSpace(`
DTSW (Does Trojan still work?)

Usage:
  dtsw                                  Open the interactive launcher
  dtsw setup                            Interactive setup wizard
  dtsw panel --config path              Interactive management panel
  dtsw init [flags]                     Generate config from flags
  dtsw validate --config path
  dtsw render xray --config path
  dtsw render systemd --config path --unit xray
  dtsw plan install --config path
  dtsw install --config path [--dry-run]
  dtsw runtime current --config path
  dtsw runtime latest
  dtsw runtime upgrade --config path [--latest|--version vX]
  dtsw status --config path
  dtsw doctor --config path [--expected-ip ip]
  dtsw users list --config path
  dtsw users add --config path --name user --password pass
  dtsw users del --config path --name user
  dtsw users url --config path --name user
  dtsw tls issuers
  dtsw issue --config path [--dry-run]
  dtsw renew --config path [--dry-run]
  dtsw uninstall --config path [--dry-run]
  dtsw fallback-serve --config path
  dtsw version
`))
}
