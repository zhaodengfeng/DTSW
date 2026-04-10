package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/domain"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
	"github.com/zhaodengfeng/dtsw/internal/systemd"
	"github.com/zhaodengfeng/dtsw/internal/tlscfg"
)

type Severity string

const (
	SeverityPass Severity = "PASS"
	SeverityWarn Severity = "WARN"
	SeverityFail Severity = "FAIL"
)

type Result struct {
	Severity Severity
	Title    string
	Detail   string
}

func Run(cfg config.Config, expectedIP string) []Result {
	results := []Result{}

	if err := cfg.Validate(); err != nil {
		results = append(results, Result{Severity: SeverityFail, Title: "Config validation", Detail: err.Error()})
		return results
	}

	ips, err := domain.Lookup(cfg.Server.Domain)
	if err != nil {
		results = append(results, Result{Severity: SeverityFail, Title: "DNS lookup", Detail: fmt.Sprintf("failed to resolve %s: %v", cfg.Server.Domain, err)})
	} else {
		detail := fmt.Sprintf("%s resolves to %v", cfg.Server.Domain, ips)
		severity := SeverityPass
		if expectedIP != "" && !contains(ips, expectedIP) {
			severity = SeverityWarn
			detail = fmt.Sprintf("%s; expected %s to be present", detail, expectedIP)
		}
		results = append(results, Result{Severity: severity, Title: "DNS lookup", Detail: detail})
	}

	results = append(results, fileCheck(cfg.Paths.ACMEBinary, "ACME binary"))
	results = append(results, fileCheck(cfg.Paths.XrayBinary, "Xray binary"))
	results = append(results, fileCheck(cfg.Paths.DTSWBinary, "DTSW binary"))
	results = append(results, dirCheck(filepath.Dir(cfg.TLS.CertificateFile), "TLS directory"))
	results = append(results, fileCheck(cfg.Paths.ACMEEnvFile, "ACME env file"))

	if version, err := xray.CurrentVersion(cfg.Paths.XrayBinary); err == nil {
		severity := SeverityPass
		detail := fmt.Sprintf("installed version %s", version)
		if version != cfg.Runtime.Version {
			severity = SeverityWarn
			detail = fmt.Sprintf("installed version %s differs from configured %s", version, cfg.Runtime.Version)
		}
		results = append(results, Result{Severity: severity, Title: "Xray version", Detail: detail})
	} else {
		results = append(results, Result{Severity: SeverityWarn, Title: "Xray version", Detail: err.Error()})
	}

	if needsRenewal, notAfter, err := tlscfg.CertificateNeedsRenewal(cfg.TLS.CertificateFile, cfg.TLS.RenewBeforeDays, time.Now()); err == nil {
		severity := SeverityPass
		detail := fmt.Sprintf("certificate expires at %s", notAfter.Format(time.RFC3339))
		if needsRenewal {
			severity = SeverityWarn
			detail = fmt.Sprintf("certificate expires at %s and is inside the renewal window", notAfter.Format(time.RFC3339))
		}
		results = append(results, Result{Severity: severity, Title: "Certificate", Detail: detail})
	} else {
		results = append(results, Result{Severity: SeverityWarn, Title: "Certificate", Detail: err.Error()})
	}

	ctx := context.Background()
	for _, service := range []string{cfg.Paths.FallbackService, cfg.Paths.RuntimeService, cfg.Paths.RenewTimer} {
		active := systemd.IsActive(ctx, service)
		enabled := systemd.IsEnabled(ctx, service)
		severity := SeverityWarn
		detail := fmt.Sprintf("enabled=%t active=%t", enabled, active)
		if active || enabled {
			severity = SeverityPass
		}
		results = append(results, Result{Severity: severity, Title: service, Detail: detail})
	}

	return results
}

func fileCheck(path, title string) Result {
	if _, err := os.Stat(path); err == nil {
		return Result{Severity: SeverityPass, Title: title, Detail: path}
	}
	return Result{Severity: SeverityWarn, Title: title, Detail: fmt.Sprintf("%s is not present yet", path)}
}

func dirCheck(path, title string) Result {
	if _, err := os.Stat(path); err == nil {
		return Result{Severity: SeverityPass, Title: title, Detail: path}
	}
	return Result{Severity: SeverityWarn, Title: title, Detail: fmt.Sprintf("%s does not exist yet", path)}
}

func contains(items []string, expected string) bool {
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}
