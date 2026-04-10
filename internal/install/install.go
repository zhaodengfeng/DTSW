package install

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/ioutil"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
	"github.com/zhaodengfeng/dtsw/internal/systemd"
	"github.com/zhaodengfeng/dtsw/internal/tlscfg"
)

type Options struct {
	DryRun     bool
	SkipIssue  bool
	SkipEnable bool
	Stdout     io.Writer
	Stderr     io.Writer
}

func Execute(ctx context.Context, cfg config.Config, opts Options) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := requireTargetEnvironment(opts); err != nil {
		return err
	}
	if cfg.TLS.Challenge == config.ChallengeHTTP01 {
		if err := ensurePackage(ctx, "socat", opts); err != nil {
			return err
		}
	}
	if err := ensureDirectories(cfg, opts); err != nil {
		return err
	}
	if err := installSelfBinary(cfg.Paths.DTSWBinary, opts); err != nil {
		return err
	}
	if err := ensureACMEEnvFile(cfg.Paths.ACMEEnvFile, opts); err != nil {
		return err
	}
	if err := ensureACMESh(ctx, cfg, opts); err != nil {
		return err
	}
	if err := ensureXray(ctx, cfg, opts); err != nil {
		return err
	}
	if err := WriteManagedConfig(cfg, opts); err != nil {
		return err
	}
	if err := WriteUnits(cfg, opts); err != nil {
		return err
	}
	if err := systemd.DaemonReload(ctx, systemd.CommandOptions{DryRun: opts.DryRun, Stdout: opts.Stdout, Stderr: opts.Stderr}); err != nil {
		return err
	}
	if !opts.SkipIssue {
		if err := ensureCertificate(ctx, cfg, opts); err != nil {
			return err
		}
	}
	if !opts.SkipEnable {
		systemdOpts := systemd.CommandOptions{DryRun: opts.DryRun, Stdout: opts.Stdout, Stderr: opts.Stderr}
		if err := systemd.Enable(ctx, systemdOpts, cfg.Paths.FallbackService, cfg.Paths.RuntimeService, cfg.Paths.RenewTimer); err != nil {
			return err
		}
		if err := systemd.RestartOrStart(ctx, systemdOpts, cfg.Paths.FallbackService, cfg.Paths.RuntimeService, cfg.Paths.RenewTimer); err != nil {
			return err
		}
	}
	return nil
}

func WriteManagedConfig(cfg config.Config, opts Options) error {
	if err := writeJSON(cfg.Paths.DTSWConfigFile, cfg, opts); err != nil {
		return err
	}
	renderer := xray.Renderer{}
	data, err := renderer.Render(cfg)
	if err != nil {
		return err
	}
	return writeBytes(cfg.Paths.RuntimeConfigFile, data, 0o644, opts)
}

func WriteUnits(cfg config.Config, opts Options) error {
	type unit struct {
		path    string
		content string
	}
	units := []unit{
		{systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.FallbackService), systemd.RenderFallbackService(cfg)},
		{systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.RuntimeService), systemd.RenderXrayService(cfg)},
		{systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.RenewService), systemd.RenderRenewService(cfg)},
		{systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.RenewTimer), systemd.RenderRenewTimer(cfg)},
	}
	for _, u := range units {
		if err := writeBytes(u.path, []byte(u.content), 0o644, opts); err != nil {
			return err
		}
	}
	return nil
}

func requireTargetEnvironment(opts Options) error {
	if opts.DryRun {
		return nil
	}
	if goruntime.GOOS != "linux" {
		return fmt.Errorf("install must run on Linux, got %s", goruntime.GOOS)
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("install must run as root")
	}
	return nil
}

func ensureDirectories(cfg config.Config, opts Options) error {
	paths := []string{
		cfg.Paths.ConfigDir,
		cfg.Paths.DataDir,
		filepath.Dir(cfg.Paths.DTSWBinary),
		filepath.Dir(cfg.Paths.XrayBinary),
		filepath.Dir(cfg.TLS.CertificateFile),
		cfg.TLS.ACMEHome,
		cfg.Paths.SystemdDir,
	}
	for _, path := range paths {
		if opts.DryRun {
			if opts.Stdout != nil {
				fmt.Fprintf(opts.Stdout, "mkdir -p %s\n", path)
			}
			continue
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func installSelfBinary(target string, opts Options) error {
	current, err := os.Executable()
	if err != nil {
		return err
	}
	if current == target {
		return nil
	}
	if opts.DryRun {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "install %s %s\n", current, target)
		}
		return nil
	}
	return ioutil.CopyFile(current, target, 0o755)
}

func ensureACMEEnvFile(path string, opts Options) error {
	const template = "# DTSW ACME environment variables\n# Add DNS provider credentials here when using dns-01.\n# Example:\n# CF_Token=replace-me\n# CF_Account_ID=replace-me\n"
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return writeBytes(path, []byte(template), 0o600, opts)
}

func ensureACMESh(ctx context.Context, cfg config.Config, opts Options) error {
	if _, err := os.Stat(cfg.Paths.ACMEBinary); err == nil {
		return nil
	}
	url := acmeDownloadURL(config.DefaultACMEShVersion)
	if opts.DryRun {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "download %s\n", url)
			fmt.Fprintf(opts.Stdout, "install acme.sh to %s\n", cfg.Paths.ACMEBinary)
		}
		return nil
	}
	tmpDir, err := os.MkdirTemp("", "dtsw-acme-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	scriptPath := filepath.Join(tmpDir, "acme.sh")
	if err := ioutil.DownloadToFile(ctx, url, scriptPath); err != nil {
		return err
	}
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Paths.ACMEBinary), 0o755); err != nil {
		return err
	}
	return ioutil.CopyFile(scriptPath, cfg.Paths.ACMEBinary, 0o755)
}

func ensureXray(ctx context.Context, cfg config.Config, opts Options) error {
	if version, err := xray.CurrentVersion(cfg.Paths.XrayBinary); err == nil && version == cfg.Runtime.Version {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "xray %s already installed at %s\n", version, cfg.Paths.XrayBinary)
		}
		return nil
	}
	return xray.Install(ctx, cfg.Paths.XrayBinary, cfg.Runtime.Version, xray.InstallOptions{
		DryRun: opts.DryRun,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
		GOOS:   "linux",
		GOARCH: goruntime.GOARCH,
	})
}

func ensureCertificate(ctx context.Context, cfg config.Config, opts Options) error {
	if _, err := os.Stat(cfg.TLS.CertificateFile); err == nil {
		return tlscfg.Renew(ctx, cfg, tlscfg.Options{DryRun: opts.DryRun, Stdout: opts.Stdout, Stderr: opts.Stderr})
	}
	return tlscfg.Issue(ctx, cfg, tlscfg.Options{DryRun: opts.DryRun, Stdout: opts.Stdout, Stderr: opts.Stderr})
}

func ensurePackage(ctx context.Context, name string, opts Options) error {
	if _, err := exec.LookPath(name); err == nil {
		return nil
	}
	var manager string
	var args [][]string
	switch {
	case hasCommand("apt-get"):
		manager = "apt-get"
		args = [][]string{{"update"}, {"install", "-y", name}}
	case hasCommand("dnf"):
		manager = "dnf"
		args = [][]string{{"install", "-y", name}}
	case hasCommand("yum"):
		manager = "yum"
		args = [][]string{{"install", "-y", name}}
	default:
		if opts.DryRun {
			if opts.Stdout != nil {
				fmt.Fprintf(opts.Stdout, "ensure package %s\n", name)
			}
			return nil
		}
		return fmt.Errorf("required package %s is missing and no supported package manager was found", name)
	}
	for _, argList := range args {
		if opts.DryRun {
			if opts.Stdout != nil {
				fmt.Fprintf(opts.Stdout, "%s %s\n", manager, ioutil.JoinArgs(argList))
			}
			continue
		}
		cmd := exec.CommandContext(ctx, manager, argList...)
		cmd.Stdout = opts.Stdout
		cmd.Stderr = opts.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, cfg config.Config, opts Options) error {
	if opts.DryRun {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "write %s\n", path)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return config.Write(path, cfg)
}

func writeBytes(path string, data []byte, mode os.FileMode, opts Options) error {
	if opts.DryRun {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "write %s\n", path)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, mode)
}

func acmeDownloadURL(version string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/acmesh-official/acme.sh/%s/acme.sh", version)
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
