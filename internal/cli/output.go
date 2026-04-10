package cli

import (
	"fmt"
	"io"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func printInstallSummary(stdout io.Writer, configPath string, cfg config.Config) {
	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintf(stdout, "  dtsw status --config %q\n", configPath)
	fmt.Fprintf(stdout, "  sudo dtsw panel --config %q  # menu 1 upgrades Xray to the latest stable release\n", configPath)
	fmt.Fprintf(stdout, "  sudo dtsw runtime upgrade --config %q --latest\n", configPath)

	user, ok := preferredPanelUser(cfg)
	if !ok {
		return
	}

	fmt.Fprintf(stdout, "  dtsw users url --config %q --name %s\n", configPath, user.Name)
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Client configuration:")
	fmt.Fprintln(stdout, "  Protocol:  trojan")
	fmt.Fprintf(stdout, "  Address:   %s\n", cfg.Server.Domain)
	fmt.Fprintf(stdout, "  Port:      %d\n", cfg.Server.Port)
	fmt.Fprintf(stdout, "  Username:  %s\n", user.Name)
	fmt.Fprintf(stdout, "  Password:  %s\n", user.Password)
	fmt.Fprintln(stdout, "  Security:  tls")
	fmt.Fprintf(stdout, "  SNI:       %s\n", cfg.Server.Domain)
	fmt.Fprintf(stdout, "  ALPN:      %s\n", joinALPN(cfg.Server.ALPN))
	fmt.Fprintln(stdout, "  Import URL:")
	fmt.Fprintf(stdout, "    %s\n", trojanURL(cfg, user))
}

func joinALPN(values []string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for _, value := range values[1:] {
		out += "," + value
	}
	return out
}
