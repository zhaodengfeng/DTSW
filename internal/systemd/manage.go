package systemd

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

type CommandOptions struct {
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
}

func DaemonReload(ctx context.Context, opts CommandOptions) error {
	return run(ctx, opts, "systemctl", "daemon-reload")
}

func EnableNow(ctx context.Context, opts CommandOptions, units ...string) error {
	args := append([]string{"enable", "--now"}, units...)
	return run(ctx, opts, "systemctl", args...)
}

func Reload(ctx context.Context, opts CommandOptions, unit string) error {
	return run(ctx, opts, "systemctl", "reload", unit)
}

func StopDisable(ctx context.Context, opts CommandOptions, units ...string) error {
	if err := run(ctx, opts, "systemctl", append([]string{"stop"}, units...)...); err != nil {
		return err
	}
	return run(ctx, opts, "systemctl", append([]string{"disable"}, units...)...)
}

func IsActive(ctx context.Context, unit string) bool {
	return exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unit).Run() == nil
}

func IsEnabled(ctx context.Context, unit string) bool {
	return exec.CommandContext(ctx, "systemctl", "is-enabled", "--quiet", unit).Run() == nil
}

func run(ctx context.Context, opts CommandOptions, name string, args ...string) error {
	if opts.DryRun {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "%s %s\n", name, joinArgs(args))
		}
		return nil
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	return cmd.Run()
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
