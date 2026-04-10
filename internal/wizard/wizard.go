package wizard

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/mail"
	"strconv"
	"strings"

	"github.com/zhaodengfeng/dtsw/internal/config"
	"github.com/zhaodengfeng/dtsw/internal/tlscfg"
)

// Result holds the wizard output.
type Result struct {
	Config     config.Config
	ConfigPath string
	AutoStart  bool
}

// Run executes the interactive setup wizard.
func Run(stdin io.Reader, stdout, stderr io.Writer) (Result, error) {
	r := bufio.NewReader(stdin)
	var result Result

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║       DTSW Interactive Setup         ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "This wizard will guide you through configuring DTSW.")
	fmt.Fprintln(stdout, "Press Enter to accept the default value shown in [brackets].")
	fmt.Fprintln(stdout, "")

	// 1. Domain
	domain, err := promptRequired(r, stdout, "Domain name (e.g. trojan.example.com)", "")
	if err != nil {
		return result, err
	}

	// 2. Email
	email, err := promptValidated(r, stdout, "ACME email address", "", func(v string) error {
		if v == "" {
			return fmt.Errorf("email is required")
		}
		if _, err := mail.ParseAddress(v); err != nil {
			return fmt.Errorf("invalid email: %v", err)
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	// 3. Password
	defaultPass := generatePassword()
	password, err := promptDefault(r, stdout, "Trojan password", defaultPass)
	if err != nil {
		return result, err
	}

	// 4. Port
	portStr, err := promptDefault(r, stdout, "Listen port", "443")
	if err != nil {
		return result, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return result, fmt.Errorf("invalid port: %s", portStr)
	}

	// 5. TLS issuer
	issuers := tlscfg.SupportedIssuers()
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Available certificate issuers:")
	for i, iss := range issuers {
		fmt.Fprintf(stdout, "  %d) %s - %s\n", i+1, iss.DisplayName, iss.Notes)
	}
	issuerStr, err := promptDefault(r, stdout, "Select issuer", "1")
	if err != nil {
		return result, err
	}
	issuerIdx, err := strconv.Atoi(issuerStr)
	if err != nil || issuerIdx < 1 || issuerIdx > len(issuers) {
		return result, fmt.Errorf("invalid issuer selection: %s", issuerStr)
	}
	selectedIssuer := issuers[issuerIdx-1].ID

	// 6. Challenge type
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "ACME challenge types:")
	fmt.Fprintln(stdout, "  1) HTTP-01 - Requires TCP port 80 to be reachable (recommended)")
	fmt.Fprintln(stdout, "  2) DNS-01  - Requires DNS provider API credentials")
	challengeStr, err := promptDefault(r, stdout, "Select challenge type", "1")
	if err != nil {
		return result, err
	}
	var challenge, dnsProvider string
	switch challengeStr {
	case "1":
		challenge = config.ChallengeHTTP01
	case "2":
		challenge = config.ChallengeDNS01
		dnsProvider, err = promptRequired(r, stdout, "DNS provider (e.g. dns_cf, dns_ali)", "")
		if err != nil {
			return result, err
		}
	default:
		return result, fmt.Errorf("invalid challenge selection: %s", challengeStr)
	}

	// 7. Config output path
	configPath, err := promptDefault(r, stdout, "Config file path", "/etc/dtsw/config.json")
	if err != nil {
		return result, err
	}

	// Build config
	paths := config.DefaultPaths()
	paths.DTSWConfigFile = configPath
	cfg := config.Config{
		Name: "dtsw",
		Runtime: config.RuntimeConfig{
			Type:    config.RuntimeXray,
			Version: config.DefaultXrayVersion,
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
		return result, fmt.Errorf("configuration is invalid: %v", err)
	}

	// Show summary
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "╔══════════════════════════════════════╗")
	fmt.Fprintln(stdout, "║         Configuration Summary        ║")
	fmt.Fprintln(stdout, "╚══════════════════════════════════════╝")
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "  Domain:      %s\n", domain)
	fmt.Fprintf(stdout, "  Email:       %s\n", email)
	fmt.Fprintf(stdout, "  Password:    %s\n", password)
	fmt.Fprintf(stdout, "  Port:        %d\n", port)
	fmt.Fprintf(stdout, "  Issuer:      %s\n", selectedIssuer)
	fmt.Fprintf(stdout, "  Challenge:   %s\n", challenge)
	if dnsProvider != "" {
		fmt.Fprintf(stdout, "  DNS Provider: %s\n", dnsProvider)
	}
	fmt.Fprintf(stdout, "  Config Path: %s\n", configPath)
	fmt.Fprintf(stdout, "  Runtime:     Xray %s\n", config.DefaultXrayVersion)
	fmt.Fprintln(stdout, "")

	// Confirm
	confirm, err := promptDefault(r, stdout, "Proceed with installation? (y/n)", "y")
	if err != nil {
		return result, err
	}
	autoStart := strings.EqualFold(confirm, "y") || strings.EqualFold(confirm, "yes")

	result.Config = cfg
	result.ConfigPath = configPath
	result.AutoStart = autoStart
	return result, nil
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
		fmt.Fprintln(w, "    ↳ This field is required.")
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
