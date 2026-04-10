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
	return []Step{
		{
			Title:       "Create directories",
			Description: "Prepare DTSW config, state, ACME, and certificate directories.",
			Command: fmt.Sprintf(
				"mkdir -p %s %s %s %s",
				cfg.Paths.ConfigDir,
				cfg.Paths.DataDir,
				cfg.TLS.ACMEHome,
				filepath.Dir(cfg.TLS.CertificateFile),
			),
		},
		{
			Title:       "Install acme.sh",
			Description: "Download a pinned acme.sh script used for Let's Encrypt and ZeroSSL automation.",
			Command:     fmt.Sprintf("curl -fsSL %s -o %s && chmod 0755 %s", acmeDownloadURL(config.DefaultACMEShVersion), cfg.Paths.ACMEBinary, cfg.Paths.ACMEBinary),
		},
		{
			Title:       "Copy DTSW files",
			Description: "Place the DTSW binary, config, and rendered Xray config on the target server.",
			Command:     fmt.Sprintf("install -m 0755 ./bin/dtsw /usr/local/bin/dtsw && install -m 0600 ./config.json %s", cfg.Paths.DTSWConfigFile),
		},
		{
			Title:       "Issue certificate",
			Description: "Issue the initial certificate before starting the Trojan runtime.",
			Command:     fmt.Sprintf("/usr/local/bin/dtsw issue --config %s", cfg.Paths.DTSWConfigFile),
		},
		{
			Title:       "Enable services",
			Description: "Start fallback HTTP, Xray runtime, and automatic renewal.",
			Command:     fmt.Sprintf("systemctl enable --now %s %s %s", cfg.Paths.FallbackService, cfg.Paths.RuntimeService, cfg.Paths.RenewTimer),
		},
		{
			Title:       "Open firewall",
			Description: "Allow HTTP-01 validation and Trojan traffic.",
			Command:     "Open TCP 80 and 443 in both the OS firewall and your cloud security group.",
		},
	}
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
