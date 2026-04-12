package wizard

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/runtime/xray"
	"github.com/zhaodengfeng/dtsw/internal/tlscfg"
)

var latestVersionLookup = xray.LatestVersion

// Result holds the wizard output.
type Result struct {
	Config     config.Config
	ConfigPath string
	AutoStart  bool
}

// Run executes the interactive setup wizard.
func Run(stdin io.Reader, stdout, stderr io.Writer) (Result, error) {
	return run(stdin, stdout, stderr, os.Geteuid() == 0)
}

func run(stdin io.Reader, stdout, stderr io.Writer, isRoot bool) (Result, error) {
	r := bufio.NewReader(stdin)
	var result Result

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║         DTSW 引导安装向导             ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "本向导将引导您完成 DTSW 的配置。")
	fmt.Fprintln(stdout, "按回车键接受 [方括号] 中的默认值。")
	if !isRoot {
		fmt.Fprintln(stdout, "当前非 root 运行，配置将默认保存在当前目录。")
		fmt.Fprintln(stdout, "安装服务仍需以 root 身份运行 DTSW。")
	}
	fmt.Fprintln(stdout, "")

	domain, err := promptRequired(r, stdout, "域名 (例如 trojan.example.com)", "")
	if err != nil {
		return result, err
	}

	email, err := promptValidated(r, stdout, "ACME 邮箱地址", "", func(v string) error {
		if v == "" {
			return fmt.Errorf("邮箱为必填项")
		}
		if _, err := mail.ParseAddress(v); err != nil {
			return fmt.Errorf("邮箱格式无效: %v", err)
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	defaultPass := generatePassword()
	password, err := promptDefault(r, stdout, "Trojan 密码", defaultPass)
	if err != nil {
		return result, err
	}

	portStr, err := promptDefault(r, stdout, "监听端口", "443")
	if err != nil {
		return result, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return result, fmt.Errorf("无效的端口: %s", portStr)
	}

	issuers := tlscfg.SupportedIssuers()
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "可用的证书颁发者:")
	for i, iss := range issuers {
		fmt.Fprintf(stdout, "  %d) %s - %s\n", i+1, iss.DisplayName, iss.Notes)
	}
	issuerStr, err := promptDefault(r, stdout, "选择颁发者", "1")
	if err != nil {
		return result, err
	}
	issuerIdx, err := strconv.Atoi(issuerStr)
	if err != nil || issuerIdx < 1 || issuerIdx > len(issuers) {
		return result, fmt.Errorf("无效的颁发者选择: %s", issuerStr)
	}
	selectedIssuer := issuers[issuerIdx-1].ID

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "ACME 验证方式:")
	fmt.Fprintln(stdout, "  1) HTTP-01 - 需要 TCP 80 端口可访问 (推荐)")
	fmt.Fprintln(stdout, "  2) DNS-01  - 需要 DNS 服务商 API 凭证")
	challengeStr, err := promptDefault(r, stdout, "选择验证方式", "1")
	if err != nil {
		return result, err
	}
	var challenge, dnsProvider string
	switch challengeStr {
	case "1":
		challenge = config.ChallengeHTTP01
	case "2":
		challenge = config.ChallengeDNS01
		dnsProvider, err = promptRequired(r, stdout, "DNS 服务商 (例如 dns_cf, dns_ali)", "")
		if err != nil {
			return result, err
		}
	default:
		return result, fmt.Errorf("无效的验证方式选择: %s", challengeStr)
	}

	configPath, err := promptDefault(r, stdout, "配置文件路径", defaultConfigPath(isRoot))
	if err != nil {
		return result, err
	}
	runtimeVersion, runtimeNote := resolveInitialRuntimeVersion()

	paths := config.DefaultPaths()
	paths.DTSWConfigFile = configPath
	cfg := config.Config{
		Name: "dtsw",
		Runtime: config.RuntimeConfig{
			Type:    config.RuntimeXray,
			Version: runtimeVersion,
		},
		Server: config.ServerConfig{
			Domain:     domain,
			ListenHost: "0.0.0.0",
			Port:       port,
			ALPN:       []string{"h2", "http/1.1"},
		},
		TLS: config.TLSConfig{
			Email:           email,
			Issuer:          selectedIssuer,
			Challenge:       challenge,
			DNSProvider:     dnsProvider,
			RenewBeforeDays: 30,
			ACMEHome:        paths.DataDir + "/acme",
			CertificateFile: paths.DataDir + "/tls/fullchain.pem",
			PrivateKeyFile:  paths.DataDir + "/tls/privkey.pem",
		},
		Fallback: config.FallbackConfig{
			ListenAddress: "127.0.0.1:8080",
			Mode:          config.FallbackCaddyStatic,
			SiteRoot:      paths.DataDir + "/site",
			SiteTitle:     "Service Unavailable",
			SiteMessage:   "DTSW is online, but this endpoint does not accept direct web traffic.",
			StatusCode:    404,
		},
		Users: []config.User{{
			Name:     "primary",
			Password: password,
		}},
		Paths: paths,
	}

	if err := cfg.Validate(); err != nil {
		return result, fmt.Errorf("配置校验失败: %v", err)
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║           配置摘要                   ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "  域名:          %s\n", domain)
	fmt.Fprintf(stdout, "  邮箱:          %s\n", email)
	fmt.Fprintf(stdout, "  密码:          %s\n", password)
	fmt.Fprintf(stdout, "  端口:          %d\n", port)
	fmt.Fprintf(stdout, "  颁发者:        %s\n", selectedIssuer)
	fmt.Fprintf(stdout, "  验证方式:      %s\n", challenge)
	if dnsProvider != "" {
		fmt.Fprintf(stdout, "  DNS 服务商:    %s\n", dnsProvider)
	}
	fmt.Fprintf(stdout, "  配置文件:      %s\n", configPath)
	fmt.Fprintf(stdout, "  运行时:        Xray %s\n", runtimeVersion)
	fmt.Fprintln(stdout, "  回落:          真实网站 (Caddy 静态站点)")
	fmt.Fprintf(stdout, "  站点根目录:    %s\n", cfg.Fallback.SiteRoot)
	if runtimeNote != "" {
		fmt.Fprintf(stdout, "  运行时备注:    %s\n", runtimeNote)
	}
	fmt.Fprintln(stdout, "")

	if !isRoot {
		fmt.Fprintln(stdout, "自动安装服务需要 root 权限。")
		fmt.Fprintln(stdout, "您可以先保存配置，之后使用 sudo 运行安装命令。")
		fmt.Fprintln(stdout, "")
	}

	confirm, err := promptDefault(r, stdout, "是否立即开始安装？(y/n)", defaultInstallAnswer(isRoot))
	if err != nil {
		return result, err
	}
	autoStart := strings.EqualFold(confirm, "y") || strings.EqualFold(confirm, "yes")

	result.Config = cfg
	result.ConfigPath = configPath
	result.AutoStart = autoStart
	return result, nil
}

func defaultConfigPath(isRoot bool) string {
	if isRoot {
		return "/etc/dtsw/config.json"
	}
	return "./dtsw.config.json"
}

func defaultInstallAnswer(isRoot bool) string {
	if isRoot {
		return "y"
	}
	return "n"
}

func promptDefault(r *bufio.Reader, w io.Writer, label, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Fprintf(w, "  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Fprintf(w, "  %s: ", label)
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

func promptRequired(r *bufio.Reader, w io.Writer, label, defaultVal string) (string, error) {
	for {
		val, err := promptDefault(r, w, label, defaultVal)
		if err != nil {
			return "", err
		}
		if val != "" {
			return val, nil
		}
		fmt.Fprintln(w, "    ↳ 此项为必填。")
	}
}

func promptValidated(r *bufio.Reader, w io.Writer, label, defaultVal string, validate func(string) error) (string, error) {
	for {
		val, err := promptDefault(r, w, label, defaultVal)
		if err != nil {
			return "", err
		}
		if verr := validate(val); verr != nil {
			fmt.Fprintf(w, "    ↳ %v\n", verr)
			continue
		}
		return val, nil
	}
}

func generatePassword() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "change-me-please"
	}
	return hex.EncodeToString(b)
}

func resolveInitialRuntimeVersion() (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	version, err := latestVersionLookup(ctx)
	if err != nil {
		return config.DefaultXrayVersion, fmt.Sprintf("获取最新版本失败，使用内置版本 %s (%v)", config.DefaultXrayVersion, err)
	}
	return version, "已获取最新稳定版本并写入配置。"
}
