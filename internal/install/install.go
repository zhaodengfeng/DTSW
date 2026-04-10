package install

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
	"github.com/zhaodengfeng/dtsw/internal/systemd"
	"github.com/zhaodengfeng/dtsw/internal/tlscfg"
)

type Options struct {
	DryRun     bool
	SkipIssue  bool
	SkipEnable bool
	Force      bool
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
		if err := systemd.EnableNow(ctx, systemd.CommandOptions{DryRun: opts.DryRun, Stdout: opts.Stdout, Stderr: opts.Stderr}, cfg.Paths.FallbackService, cfg.Paths.RuntimeService, cfg.Paths.RenewTimer); err != nil {
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
	units := map[string]string{
		systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.RuntimeService):  systemd.RenderXrayService(cfg),
		systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.FallbackService): systemd.RenderFallbackService(cfg),
		systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.RenewService):    systemd.RenderRenewService(cfg),
		systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.RenewTimer):      systemd.RenderRenewTimer(cfg),
	}
	for path, content := range units {
		if err := writeBytes(path, []byte(content), 0o644, opts); err != nil {
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
	return copyFile(current, target, 0o755)
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
	if opts.DryRun {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "download https://get.acme.sh and install acme.sh for %s\n", cfg.TLS.Email)
		}
		return nil
	}
	tmpDir, err := os.MkdirTemp("", "dtsw-acme-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	scriptPath := filepath.Join(tmpDir, "get.acme.sh")
	if err := downloadToFile(ctx, "https://get.acme.sh", scriptPath); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", scriptPath, "email="+cfg.TLS.Email)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	cmd.Env = append(os.Environ(), "HOME=/root")
	return cmd.Run()
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
				fmt.Fprintf(opts.Stdout, "%s %s\n", manager, joinArgs(argList))
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

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func downloadToFile(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download %s failed with %s", url, resp.Status)
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	out := args[0]
	for _, arg := range args[1:] {
		out += " " + arg
	}
	return out
}
