package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
	"github.com/zhaodengfeng/dtsw/internal/systemd"
	"github.com/zhaodengfeng/dtsw/internal/tlscfg"
)

func printClientConfiguration(stdout io.Writer, cfg config.Config) bool {
	user, ok := preferredPanelUser(cfg)
	if !ok {
		fmt.Fprintln(stdout, "No Trojan users are configured yet.")
		return false
	}

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
	return true
}

func printStatusReport(stdout io.Writer, cfg config.Config) {
	fmt.Fprintf(stdout, "Name: %s\n", cfg.Name)
	fmt.Fprintf(stdout, "Domain: %s\n", cfg.Server.Domain)
	fmt.Fprintf(stdout, "Port: %d\n", cfg.Server.Port)
	fmt.Fprintf(stdout, "Runtime: %s %s\n", cfg.Runtime.Type, cfg.Runtime.Version)
	fmt.Fprintf(stdout, "Issuer: %s\n", cfg.TLS.Issuer)
	fmt.Fprintf(stdout, "Challenge: %s\n", cfg.TLS.Challenge)
	fmt.Fprintf(stdout, "Users: %d\n", len(cfg.Users))
	if version, err := xray.CurrentVersion(cfg.Paths.XrayBinary); err == nil {
		fmt.Fprintf(stdout, "Installed Xray: %s\n", version)
	} else {
		fmt.Fprintf(stdout, "Installed Xray: unavailable (%v)\n", err)
	}
	if needsRenewal, notAfter, err := tlscfg.CertificateNeedsRenewal(cfg.TLS.CertificateFile, cfg.TLS.RenewBeforeDays, time.Now()); err == nil {
		fmt.Fprintf(stdout, "Certificate Expires: %s\n", notAfter.Format(time.RFC3339))
		fmt.Fprintf(stdout, "Certificate Renewal Needed: %t\n", needsRenewal)
	} else {
		fmt.Fprintf(stdout, "Certificate: unavailable (%v)\n", err)
	}
	ctx := context.Background()
	fmt.Fprintf(stdout, "Fallback Service: enabled=%t active=%t\n", systemd.IsEnabled(ctx, cfg.Paths.FallbackService), systemd.IsActive(ctx, cfg.Paths.FallbackService))
	fmt.Fprintf(stdout, "Runtime Service: enabled=%t active=%t\n", systemd.IsEnabled(ctx, cfg.Paths.RuntimeService), systemd.IsActive(ctx, cfg.Paths.RuntimeService))
	fmt.Fprintf(stdout, "Renew Timer: enabled=%t active=%t\n", systemd.IsEnabled(ctx, cfg.Paths.RenewTimer), systemd.IsActive(ctx, cfg.Paths.RenewTimer))
}

func printUserList(stdout io.Writer, cfg config.Config) {
	if len(cfg.Users) == 0 {
		fmt.Fprintln(stdout, "No users are configured yet.")
		return
	}
	fmt.Fprintln(stdout, "Users:")
	for _, user := range cfg.Users {
		fmt.Fprintf(stdout, "  - %s\n", user.Name)
	}
}

func joinALPN(values []string) string {
	return strings.Join(values, ",")
}
