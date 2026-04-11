package install

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

type Step struct {
	Title       string
	Description string
	Command     string
}

func Build(cfg config.Config) []Step {
	steps := []Step{
		{
			Title:       "Create directories",
			Description: "Prepare DTSW config, state, ACME, certificate, and fallback site directories.",
			Command: fmt.Sprintf(
				"mkdir -p %s %s %s %s %s",
				cfg.Paths.ConfigDir,
				cfg.Paths.DataDir,
				cfg.TLS.ACMEHome,
				filepath.Dir(cfg.TLS.CertificateFile),
				cfg.Fallback.SiteRoot,
			),
		},
		{
			Title:       "Install acme.sh",
			Description: "Download a pinned acme.sh script used for Let's Encrypt and ZeroSSL automation.",
			Command:     fmt.Sprintf("curl -fsSL %s -o %s && chmod 0755 %s", acmeDownloadURL(config.DefaultACMEShVersion), cfg.Paths.ACMEBinary, cfg.Paths.ACMEBinary),
		},
		{
			Title:       "Issue certificate",
			Description: "Issue the initial certificate before starting the Trojan runtime.",
			Command:     fmt.Sprintf("/usr/local/bin/dtsw issue --config %s", cfg.Paths.DTSWConfigFile),
		},
		{
			Title:       "Enable services",
			Description: "Enable the fallback website, Xray runtime, and automatic renewal, then restart running units or start inactive ones.",
			Command:     fmt.Sprintf("systemctl enable %s %s %s && systemctl restart %s %s %s  # DTSW starts inactive units automatically when needed", cfg.Paths.FallbackService, cfg.Paths.RuntimeService, cfg.Paths.RenewTimer, cfg.Paths.FallbackService, cfg.Paths.RuntimeService, cfg.Paths.RenewTimer),
		},
		{
			Title:       "Open firewall",
			Description: "Allow HTTP-01 validation and Trojan traffic.",
			Command:     "Open TCP 80 and 443 in both the OS firewall and your cloud security group.",
		},
	}
	if cfg.Fallback.Mode == config.FallbackCaddyStatic {
		steps = append([]Step{
			{
				Title:       "Install Caddy",
				Description: "Download the pinned Caddy binary used to serve the fallback website locally.",
				Command:     fmt.Sprintf("Automatic via dtsw install (downloads, verifies, and installs %s)", cfg.Paths.CaddyBinary),
			},
			{
				Title:       "Copy DTSW files",
				Description: "Place DTSW, the rendered Xray config, the generated Caddyfile, and the fallback site on the target server.",
				Command:     fmt.Sprintf("install -m 0755 ./bin/dtsw /usr/local/bin/dtsw && install -m 0600 ./config.json %s && install -m 0644 ./Caddyfile %s", cfg.Paths.DTSWConfigFile, cfg.Paths.CaddyConfigFile),
			},
		}, steps...)
	} else {
		steps = append([]Step{
			{
				Title:       "Copy DTSW files",
				Description: "Place the DTSW binary, config, and rendered Xray config on the target server.",
				Command:     fmt.Sprintf("install -m 0755 ./bin/dtsw /usr/local/bin/dtsw && install -m 0600 ./config.json %s", cfg.Paths.DTSWConfigFile),
			},
		}, steps...)
	}
	return steps
}

func Render(steps []Step) string {
	var b strings.Builder
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step.Title)
		fmt.Fprintf(&b, "   %s\n", step.Description)
		fmt.Fprintf(&b, "   %s\n\n", step.Command)
	}
	return strings.TrimSpace(b.String()) + "\n"
}
