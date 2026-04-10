package systemd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func UnitPath(systemdDir, name string) string {
	return filepath.Join(systemdDir, name)
}

func RenderXrayService(cfg config.Config) string {
	return strings.TrimSpace(fmt.Sprintf(`
[Unit]
Description=DTSW Xray Runtime
After=network-online.target %s
Wants=network-online.target
Requires=%s

[Service]
Type=simple
ExecStart=%s run -config %s
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
`, cfg.Paths.FallbackService, cfg.Paths.FallbackService, cfg.Paths.XrayBinary, cfg.Paths.RuntimeConfigFile)) + "\n"
}

func RenderFallbackService(cfg config.Config) string {
	return strings.TrimSpace(fmt.Sprintf(`
[Unit]
Description=DTSW Fallback HTTP Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s fallback-serve --config %s
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
`, cfg.Paths.DTSWBinary, cfg.Paths.DTSWConfigFile)) + "\n"
}

func RenderRenewService(cfg config.Config) string {
	return strings.TrimSpace(fmt.Sprintf(`
[Unit]
Description=DTSW Certificate Renewal
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
EnvironmentFile=-%s
WorkingDirectory=%s
ExecStart=%s renew --config %s
`, cfg.Paths.ACMEEnvFile, cfg.Paths.ConfigDir, cfg.Paths.DTSWBinary, cfg.Paths.DTSWConfigFile)) + "\n"
}

func RenderRenewTimer(cfg config.Config) string {
	return strings.TrimSpace(fmt.Sprintf(`
[Unit]
Description=Run DTSW certificate renewal twice daily

[Timer]
OnCalendar=*-*-* 00,12:00:00
Persistent=true
Unit=%s

[Install]
WantedBy=timers.target
`, cfg.Paths.RenewService)) + "\n"
}
