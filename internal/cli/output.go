package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
	"github.com/zhaodengfeng/dtsw/internal/stats"
	"github.com/zhaodengfeng/dtsw/internal/systemd"
	"github.com/zhaodengfeng/dtsw/internal/tlscfg"
)

func printClientConfiguration(stdout io.Writer, cfg config.Config) bool {
	user, ok := preferredPanelUser(cfg)
	if !ok {
		fmt.Fprintln(stdout, "尚未配置任何 Trojan 用户。")
		return false
	}
	return printClientConfigurationForUser(stdout, cfg, user)
}

func printClientConfigurationForUser(stdout io.Writer, cfg config.Config, user config.User) bool {
	fmt.Fprintln(stdout, "客户端配置:")
	fmt.Fprintln(stdout, "  协议:      trojan")
	fmt.Fprintf(stdout, "  地址:      %s\n", cfg.Server.Domain)
	fmt.Fprintf(stdout, "  端口:      %d\n", cfg.Server.Port)
	fmt.Fprintf(stdout, "  用户名:    %s\n", user.Name)
	fmt.Fprintf(stdout, "  密码:      %s\n", user.Password)
	fmt.Fprintln(stdout, "  安全:      tls")
	fmt.Fprintf(stdout, "  SNI:       %s\n", cfg.Server.Domain)
	fmt.Fprintf(stdout, "  ALPN:      %s\n", joinALPN(cfg.Server.ALPN))
	fmt.Fprintln(stdout, "  导入链接:")
	fmt.Fprintf(stdout, "    %s\n", trojanURL(cfg, user))
	return true
}

func printStatusReport(stdout io.Writer, cfg config.Config) {
	fmt.Fprintf(stdout, "名称: %s\n", cfg.Name)
	fmt.Fprintf(stdout, "域名: %s\n", cfg.Server.Domain)
	fmt.Fprintf(stdout, "端口: %d\n", cfg.Server.Port)
	fmt.Fprintf(stdout, "运行时: %s %s\n", cfg.Runtime.Type, cfg.Runtime.Version)
	fmt.Fprintf(stdout, "回落模式: %s\n", cfg.Fallback.Mode)
	if cfg.Fallback.Mode == config.FallbackCaddyStatic {
		fmt.Fprintf(stdout, "网站根目录: %s\n", cfg.Fallback.SiteRoot)
	}
	fmt.Fprintf(stdout, "证书颁发者: %s\n", cfg.TLS.Issuer)
	fmt.Fprintf(stdout, "验证方式: %s\n", cfg.TLS.Challenge)
	fmt.Fprintf(stdout, "用户数: %d\n", len(cfg.Users))
	if version, err := xray.CurrentVersion(cfg.Paths.XrayBinary); err == nil {
		fmt.Fprintf(stdout, "已安装 Xray: %s\n", version)
	} else {
		fmt.Fprintf(stdout, "已安装 Xray: 不可用 (%v)\n", err)
	}
	if needsRenewal, notAfter, err := tlscfg.CertificateNeedsRenewal(cfg.TLS.CertificateFile, cfg.TLS.RenewBeforeDays, time.Now()); err == nil {
		fmt.Fprintf(stdout, "证书到期时间: %s\n", notAfter.Format(time.RFC3339))
		fmt.Fprintf(stdout, "是否需要续签: %t\n", needsRenewal)
	} else {
		fmt.Fprintf(stdout, "证书: 不可用 (%v)\n", err)
	}
	ctx := context.Background()
	fmt.Fprintf(stdout, "回落服务: enabled=%t active=%t\n", systemd.IsEnabled(ctx, cfg.Paths.FallbackService), systemd.IsActive(ctx, cfg.Paths.FallbackService))
	fmt.Fprintf(stdout, "运行时服务: enabled=%t active=%t\n", systemd.IsEnabled(ctx, cfg.Paths.RuntimeService), systemd.IsActive(ctx, cfg.Paths.RuntimeService))
	fmt.Fprintf(stdout, "续签定时器: enabled=%t active=%t\n", systemd.IsEnabled(ctx, cfg.Paths.RenewTimer), systemd.IsActive(ctx, cfg.Paths.RenewTimer))
}

func printUserList(stdout io.Writer, cfg config.Config) {
	if len(cfg.Users) == 0 {
		fmt.Fprintln(stdout, "尚未配置任何用户。")
		return
	}
	fmt.Fprintln(stdout, "用户列表:")
	for _, user := range cfg.Users {
		fmt.Fprintf(stdout, "  - %s\n", user.Name)
	}
}

func joinALPN(values []string) string {
	return strings.Join(values, ",")
}

func printAllUserStats(stdout io.Writer, cfg config.Config, store *stats.Store) {
	fmt.Fprintln(stdout, "流量统计:")
	fmt.Fprintln(stdout, "")
	if len(cfg.Users) == 0 {
		fmt.Fprintln(stdout, "  尚未配置任何用户。")
		return
	}
	if !store.UpdatedAt.IsZero() {
		fmt.Fprintf(stdout, "  最后更新: %s\n\n", store.UpdatedAt.Format(time.RFC3339))
	}
	for _, u := range cfg.Users {
		printUserStats(stdout, u.Name, store)
		fmt.Fprintln(stdout, "")
	}
}

func printUserStats(stdout io.Writer, userName string, store *stats.Store) {
	now := time.Now()
	u, ok := store.Users[userName]
	if !ok {
		fmt.Fprintf(stdout, "  %s: 暂无流量记录\n", userName)
		return
	}
	totalUp := u.TotalUpload()
	totalDown := u.TotalDownload()
	monthUp, monthDown := u.CurrentMonthTraffic(now)

	fmt.Fprintf(stdout, "  用户: %s\n", userName)
	if !u.TrackedSince.IsZero() {
		fmt.Fprintf(stdout, "    统计起始:        %s\n", u.TrackedSince.Format("2006-01-02"))
	}
	fmt.Fprintf(stdout, "    总上传:          %s\n", stats.FormatBytes(totalUp))
	fmt.Fprintf(stdout, "    总下载:          %s\n", stats.FormatBytes(totalDown))
	fmt.Fprintf(stdout, "    总流量:          %s\n", stats.FormatBytes(totalUp+totalDown))
	fmt.Fprintf(stdout, "    本月上传:        %s  (%s)\n", stats.FormatBytes(monthUp), now.Format("2006-01"))
	fmt.Fprintf(stdout, "    本月下载:        %s  (%s)\n", stats.FormatBytes(monthDown), now.Format("2006-01"))
	fmt.Fprintf(stdout, "    本月流量:        %s  (%s)\n", stats.FormatBytes(monthUp+monthDown), now.Format("2006-01"))
}
