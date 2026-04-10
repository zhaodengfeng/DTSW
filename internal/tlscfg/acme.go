package tlscfg

import (
	"bufio"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

type IssuerInfo struct {
	ID          string
	DisplayName string
	ACMEServer  string
	Notes       string
}

type Options struct {
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
	Now    func() time.Time
}

func SupportedIssuers() []IssuerInfo {
	return []IssuerInfo{
		{
			ID:          config.IssuerLetsEncrypt,
			DisplayName: "Let's Encrypt",
			ACMEServer:  "letsencrypt",
			Notes:       "Best default for most DTSW installs.",
		},
		{
			ID:          config.IssuerZeroSSL,
			DisplayName: "ZeroSSL",
			ACMEServer:  "zerossl",
			Notes:       "Useful as an alternative CA and supported by acme.sh.",
		},
	}
}

func Issue(ctx context.Context, cfg config.Config, opts Options) error {
	if err := ensureTLSDirs(cfg, opts); err != nil {
		return err
	}
	issueArgs, installArgs, err := BuildIssueCommands(cfg)
	if err != nil {
		return err
	}
	if err := run(ctx, cfg.Paths.ACMEBinary, argsWithEnvHint(cfg, issueArgs), opts, cfg.Paths.ACMEEnvFile); err != nil {
		return err
	}
	return run(ctx, cfg.Paths.ACMEBinary, argsWithEnvHint(cfg, installArgs), opts, cfg.Paths.ACMEEnvFile)
}

func Renew(ctx context.Context, cfg config.Config, opts Options) error {
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	needsRenewal, notAfter, err := CertificateNeedsRenewal(cfg.TLS.CertificateFile, cfg.TLS.RenewBeforeDays, now())
	if errors.Is(err, os.ErrNotExist) {
		return Issue(ctx, cfg, opts)
	}
	if err == nil && !needsRenewal {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "certificate is valid until %s; renewal skipped\n", notAfter.Format(time.RFC3339))
		}
		return nil
	}
	if err := ensureTLSDirs(cfg, opts); err != nil {
		return err
	}
	renewArgs, installArgs, err := BuildRenewCommands(cfg)
	if err != nil {
		return err
	}
	if err := run(ctx, cfg.Paths.ACMEBinary, argsWithEnvHint(cfg, renewArgs), opts, cfg.Paths.ACMEEnvFile); err != nil {
		return err
	}
	return run(ctx, cfg.Paths.ACMEBinary, argsWithEnvHint(cfg, installArgs), opts, cfg.Paths.ACMEEnvFile)
}

func BuildIssueCommands(cfg config.Config) ([]string, []string, error) {
	issuer, err := issuerServer(cfg.TLS.Issuer)
	if err != nil {
		return nil, nil, err
	}
	args := []string{
		"--home", cfg.TLS.ACMEHome,
		"--issue",
		"-d", cfg.Server.Domain,
		"--server", issuer,
		"--accountemail", cfg.TLS.Email,
		"--keylength", "ec-256",
	}
	switch cfg.TLS.Challenge {
	case config.ChallengeHTTP01:
		args = append(args, "--standalone")
	case config.ChallengeDNS01:
		args = append(args, "--dns", cfg.TLS.DNSProvider)
	default:
		return nil, nil, fmt.Errorf("unsupported challenge %q", cfg.TLS.Challenge)
	}
	return args, buildInstallArgs(cfg), nil
}

func BuildRenewCommands(cfg config.Config) ([]string, []string, error) {
	issuer, err := issuerServer(cfg.TLS.Issuer)
	if err != nil {
		return nil, nil, err
	}
	args := []string{
		"--home", cfg.TLS.ACMEHome,
		"--renew",
		"-d", cfg.Server.Domain,
		"--server", issuer,
		"--ecc",
	}
	return args, buildInstallArgs(cfg), nil
}

func buildInstallArgs(cfg config.Config) []string {
	reloadCmd := fmt.Sprintf("/bin/sh -c 'systemctl is-active --quiet %s && systemctl reload %s || true'", cfg.Paths.RuntimeService, cfg.Paths.RuntimeService)
	return []string{
		"--home", cfg.TLS.ACMEHome,
		"--install-cert",
		"-d", cfg.Server.Domain,
		"--ecc",
		"--key-file", cfg.TLS.PrivateKeyFile,
		"--fullchain-file", cfg.TLS.CertificateFile,
		"--reloadcmd", reloadCmd,
	}
}

func CertificateNeedsRenewal(certFile string, renewBeforeDays int, now time.Time) (bool, time.Time, error) {
	data, err := os.ReadFile(certFile)
	if err != nil {
		return true, time.Time{}, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return true, time.Time{}, errors.New("failed to decode PEM certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true, time.Time{}, err
	}
	renewAt := cert.NotAfter.AddDate(0, 0, -renewBeforeDays)
	return !now.Before(renewAt), cert.NotAfter, nil
}

func issuerServer(issuer string) (string, error) {
	switch issuer {
	case config.IssuerLetsEncrypt:
		return "letsencrypt", nil
	case config.IssuerZeroSSL:
		return "zerossl", nil
	default:
		return "", fmt.Errorf("unsupported issuer %q", issuer)
	}
}

func run(ctx context.Context, binary string, args []string, opts Options, envFile string) error {
	if opts.DryRun {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "%s %s\n", binary, strings.Join(args, " "))
			if envFile != "" {
				fmt.Fprintf(opts.Stdout, "env file: %s\n", envFile)
			}
		}
		return nil
	}
	env, err := loadEnv(envFile)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	cmd.Env = env
	return cmd.Run()
}

func loadEnv(path string) ([]string, error) {
	env := append([]string{}, os.Environ()...)
	if path == "" {
		return env, nil
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return env, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		env = append(env, line)
	}
	return env, scanner.Err()
}

func ensureTLSDirs(cfg config.Config, opts Options) error {
	for _, path := range []string{filepath.Dir(cfg.TLS.CertificateFile), cfg.TLS.ACMEHome} {
		if opts.DryRun {
			if opts.Stdout != nil {
				fmt.Fprintf(opts.Stdout, "mkdir -p %s\n", path)
			}
			continue
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func argsWithEnvHint(_ config.Config, args []string) []string {
	return args
}
