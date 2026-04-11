package install

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/systemd"
)

type RemoveOptions struct {
	DryRun     bool
	PurgeData  bool
	PurgeXray  bool
	PurgeCaddy bool
	RemoveDTSW bool
	Stdout     io.Writer
	Stderr     io.Writer
}

func Remove(ctx context.Context, cfg config.Config, opts RemoveOptions) error {
	if err := requireTargetEnvironment(Options{DryRun: opts.DryRun}); err != nil {
		return err
	}
	units := []string{cfg.Paths.FallbackService, cfg.Paths.RuntimeService, cfg.Paths.RenewTimer, cfg.Paths.RenewService}
	for _, unit := range units {
		if opts.DryRun {
			if opts.Stdout != nil {
				fmt.Fprintf(opts.Stdout, "systemctl stop %s\n", unit)
				fmt.Fprintf(opts.Stdout, "systemctl disable %s\n", unit)
			}
			continue
		}
		_ = systemd.StopDisable(ctx, systemd.CommandOptions{DryRun: false, Stdout: opts.Stdout, Stderr: opts.Stderr}, unit)
	}

	paths := []string{
		systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.FallbackService),
		systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.RuntimeService),
		systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.RenewService),
		systemd.UnitPath(cfg.Paths.SystemdDir, cfg.Paths.RenewTimer),
		cfg.Paths.RuntimeConfigFile,
		cfg.Paths.DTSWConfigFile,
	}
	if opts.PurgeData {
		paths = append(paths, cfg.Paths.ConfigDir, cfg.Paths.DataDir)
	}
	if opts.PurgeXray {
		paths = append(paths, cfg.Paths.XrayBinary)
	}
	if opts.PurgeCaddy {
		paths = append(paths, cfg.Paths.CaddyBinary)
	}
	if opts.RemoveDTSW {
		paths = append(paths, cfg.Paths.DTSWBinary)
	}
	for _, path := range paths {
		if path == "" {
			continue
		}
		if opts.DryRun {
			if opts.Stdout != nil {
				fmt.Fprintf(opts.Stdout, "remove %s\n", path)
			}
			continue
		}
		_ = os.RemoveAll(path)
	}
	return systemd.DaemonReload(ctx, systemd.CommandOptions{DryRun: opts.DryRun, Stdout: opts.Stdout, Stderr: opts.Stderr})
}
