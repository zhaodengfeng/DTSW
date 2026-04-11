package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
)

const (
	RuntimeXray = "xray"

	IssuerLetsEncrypt = "letsencrypt"
	IssuerZeroSSL     = "zerossl"

	ChallengeHTTP01 = "http-01"
	ChallengeDNS01  = "dns-01"

	DefaultXrayVersion   = "v26.1.13"
	DefaultCaddyVersion  = "v2.10.2"
	DefaultACMEShVersion = "3.1.2"

	FallbackBuiltin     = "builtin"
	FallbackCaddyStatic = "caddy-static"
)

type Config struct {
	Name     string         `json:"name"`
	Runtime  RuntimeConfig  `json:"runtime"`
	Server   ServerConfig   `json:"server"`
	TLS      TLSConfig      `json:"tls"`
	Fallback FallbackConfig `json:"fallback"`
	Users    []User         `json:"users"`
	Paths    PathsConfig    `json:"paths"`
}

type RuntimeConfig struct {
	Type    string `json:"type"`
	Version string `json:"version"`
}

type ServerConfig struct {
	Domain     string   `json:"domain"`
	ListenHost string   `json:"listen_host"`
	Port       int      `json:"port"`
	ALPN       []string `json:"alpn"`
}

type TLSConfig struct {
	Email           string `json:"email"`
	Issuer          string `json:"issuer"`
	Challenge       string `json:"challenge"`
	DNSProvider     string `json:"dns_provider,omitempty"`
	RenewBeforeDays int    `json:"renew_before_days"`
	ACMEHome        string `json:"acme_home"`
	CertificateFile string `json:"certificate_file"`
	PrivateKeyFile  string `json:"private_key_file"`
}

type FallbackConfig struct {
	ListenAddress string `json:"listen_address"`
	Mode          string `json:"mode,omitempty"`
	SiteRoot      string `json:"site_root,omitempty"`
	SiteTitle     string `json:"site_title"`
	SiteMessage   string `json:"site_message"`
	StatusCode    int    `json:"status_code"`
}

type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type PathsConfig struct {
	ConfigDir         string `json:"config_dir"`
	DataDir           string `json:"data_dir"`
	RuntimeConfigFile string `json:"runtime_config_file"`
	DTSWConfigFile    string `json:"dtsw_config_file"`
	DTSWBinary        string `json:"dtsw_binary"`
	XrayBinary        string `json:"xray_binary"`
	CaddyBinary       string `json:"caddy_binary"`
	CaddyConfigFile   string `json:"caddy_config_file"`
	ACMEBinary        string `json:"acme_binary"`
	ACMEEnvFile       string `json:"acme_env_file"`
	SystemdDir        string `json:"systemd_dir"`
	RuntimeService    string `json:"runtime_service"`
	FallbackService   string `json:"fallback_service"`
	RenewService      string `json:"renew_service"`
	RenewTimer        string `json:"renew_timer"`
}

func DefaultPaths() PathsConfig {
	return PathsConfig{
		ConfigDir:         "/etc/dtsw",
		DataDir:           "/var/lib/dtsw",
		RuntimeConfigFile: "/etc/dtsw/xray.json",
		DTSWConfigFile:    "/etc/dtsw/config.json",
		DTSWBinary:        "/usr/local/bin/dtsw",
		XrayBinary:        "/usr/local/bin/xray",
		CaddyBinary:       "/usr/local/bin/caddy",
		CaddyConfigFile:   "/etc/dtsw/Caddyfile",
		ACMEBinary:        "/usr/local/bin/acme.sh",
		ACMEEnvFile:       "/etc/dtsw/acme.env",
		SystemdDir:        "/etc/systemd/system",
		RuntimeService:    "dtsw-xray.service",
		FallbackService:   "dtsw-fallback.service",
		RenewService:      "dtsw-renew.service",
		RenewTimer:        "dtsw-renew.timer",
	}
}

func Example(domain, email, password string) Config {
	return ExampleWithVersion(domain, email, password, DefaultXrayVersion)
}

func ExampleWithVersion(domain, email, password, runtimeVersion string) Config {
	paths := DefaultPaths()
	cfg := Config{
		Name: "dtsw",
		Runtime: RuntimeConfig{
			Type:    RuntimeXray,
			Version: runtimeVersion,
		},
		Server: ServerConfig{
			Domain:     domain,
			ListenHost: "0.0.0.0",
			Port:       443,
			ALPN:       []string{"h2", "http/1.1"},
		},
		TLS: TLSConfig{
			Email:           email,
			Issuer:          IssuerLetsEncrypt,
			Challenge:       ChallengeHTTP01,
			RenewBeforeDays: 30,
			ACMEHome:        filepath.Join(paths.DataDir, "acme"),
			CertificateFile: filepath.Join(paths.DataDir, "tls", "fullchain.pem"),
			PrivateKeyFile:  filepath.Join(paths.DataDir, "tls", "privkey.pem"),
		},
		Fallback: FallbackConfig{
			ListenAddress: "127.0.0.1:8080",
			Mode:          FallbackCaddyStatic,
			SiteRoot:      filepath.Join(paths.DataDir, "site"),
			SiteTitle:     "Service Unavailable",
			SiteMessage:   "DTSW is online, but this endpoint does not accept direct web traffic.",
			StatusCode:    404,
		},
		Users: []User{{
			Name:     "primary",
			Password: password,
		}},
		Paths: paths,
	}
	cfg.ApplyDefaults()
	return cfg
}

func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.ApplyDefaults()
	return cfg, nil
}

func (c *Config) ApplyDefaults() {
	paths := DefaultPaths()

	if c.Paths.ConfigDir == "" {
		c.Paths.ConfigDir = paths.ConfigDir
	}
	if c.Paths.DataDir == "" {
		c.Paths.DataDir = paths.DataDir
	}
	if c.Paths.RuntimeConfigFile == "" {
		c.Paths.RuntimeConfigFile = paths.RuntimeConfigFile
	}
	if c.Paths.DTSWConfigFile == "" {
		c.Paths.DTSWConfigFile = paths.DTSWConfigFile
	}
	if c.Paths.DTSWBinary == "" {
		c.Paths.DTSWBinary = paths.DTSWBinary
	}
	if c.Paths.XrayBinary == "" {
		c.Paths.XrayBinary = paths.XrayBinary
	}
	if c.Paths.CaddyBinary == "" {
		c.Paths.CaddyBinary = paths.CaddyBinary
	}
	if c.Paths.CaddyConfigFile == "" {
		c.Paths.CaddyConfigFile = paths.CaddyConfigFile
	}
	if c.Paths.ACMEBinary == "" {
		c.Paths.ACMEBinary = paths.ACMEBinary
	}
	if c.Paths.ACMEEnvFile == "" {
		c.Paths.ACMEEnvFile = paths.ACMEEnvFile
	}
	if c.Paths.SystemdDir == "" {
		c.Paths.SystemdDir = paths.SystemdDir
	}
	if c.Paths.RuntimeService == "" {
		c.Paths.RuntimeService = paths.RuntimeService
	}
	if c.Paths.FallbackService == "" {
		c.Paths.FallbackService = paths.FallbackService
	}
	if c.Paths.RenewService == "" {
		c.Paths.RenewService = paths.RenewService
	}
	if c.Paths.RenewTimer == "" {
		c.Paths.RenewTimer = paths.RenewTimer
	}

	if c.TLS.ACMEHome == "" {
		c.TLS.ACMEHome = filepath.Join(c.Paths.DataDir, "acme")
	}
	if c.TLS.CertificateFile == "" {
		c.TLS.CertificateFile = filepath.Join(c.Paths.DataDir, "tls", "fullchain.pem")
	}
	if c.TLS.PrivateKeyFile == "" {
		c.TLS.PrivateKeyFile = filepath.Join(c.Paths.DataDir, "tls", "privkey.pem")
	}

	if c.Fallback.ListenAddress == "" {
		c.Fallback.ListenAddress = "127.0.0.1:8080"
	}
	if c.Fallback.Mode == "" {
		c.Fallback.Mode = FallbackBuiltin
	}
	if c.Fallback.SiteRoot == "" {
		c.Fallback.SiteRoot = filepath.Join(c.Paths.DataDir, "site")
	}
	if c.Fallback.SiteTitle == "" {
		c.Fallback.SiteTitle = "Service Unavailable"
	}
	if c.Fallback.SiteMessage == "" {
		c.Fallback.SiteMessage = "DTSW is online, but this endpoint does not accept direct web traffic."
	}
	if c.Fallback.StatusCode == 0 {
		c.Fallback.StatusCode = 404
	}
}

func Write(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func (c Config) Validate() error {
	var problems []string

	if c.Name == "" {
		problems = append(problems, "name is required")
	}

	switch c.Runtime.Type {
	case RuntimeXray:
	default:
		problems = append(problems, "runtime.type must be xray")
	}
	if c.Runtime.Version == "" {
		problems = append(problems, "runtime.version is required")
	}

	if err := validateDomain(c.Server.Domain); err != nil {
		problems = append(problems, err.Error())
	}
	if c.Server.ListenHost == "" {
		problems = append(problems, "server.listen_host is required")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		problems = append(problems, "server.port must be between 1 and 65535")
	}
	if len(c.Server.ALPN) == 0 {
		problems = append(problems, "server.alpn must include at least one value")
	}

	if _, err := mail.ParseAddress(c.TLS.Email); err != nil {
		problems = append(problems, "tls.email must be a valid email address")
	}
	switch c.TLS.Issuer {
	case IssuerLetsEncrypt, IssuerZeroSSL:
	default:
		problems = append(problems, "tls.issuer must be letsencrypt or zerossl")
	}
	switch c.TLS.Challenge {
	case ChallengeHTTP01:
	case ChallengeDNS01:
		if c.TLS.DNSProvider == "" {
			problems = append(problems, "tls.dns_provider is required when tls.challenge is dns-01")
		}
	default:
		problems = append(problems, "tls.challenge must be http-01 or dns-01")
	}
	if c.TLS.RenewBeforeDays < 1 || c.TLS.RenewBeforeDays > 60 {
		problems = append(problems, "tls.renew_before_days must be between 1 and 60")
	}
	if c.TLS.ACMEHome == "" {
		problems = append(problems, "tls.acme_home is required")
	}
	if c.TLS.CertificateFile == "" {
		problems = append(problems, "tls.certificate_file is required")
	}
	if c.TLS.PrivateKeyFile == "" {
		problems = append(problems, "tls.private_key_file is required")
	}

	if _, _, err := net.SplitHostPort(c.Fallback.ListenAddress); err != nil {
		problems = append(problems, "fallback.listen_address must be host:port")
	}
	switch c.Fallback.Mode {
	case FallbackBuiltin:
		if c.Fallback.StatusCode < 100 || c.Fallback.StatusCode > 599 {
			problems = append(problems, "fallback.status_code must be between 100 and 599")
		}
		if c.Fallback.SiteTitle == "" {
			problems = append(problems, "fallback.site_title is required")
		}
		if c.Fallback.SiteMessage == "" {
			problems = append(problems, "fallback.site_message is required")
		}
	case FallbackCaddyStatic:
		if c.Fallback.SiteRoot == "" {
			problems = append(problems, "fallback.site_root is required when fallback.mode is caddy-static")
		}
	default:
		problems = append(problems, "fallback.mode must be builtin or caddy-static")
	}

	if len(c.Users) == 0 {
		problems = append(problems, "users must include at least one account")
	}
	seen := map[string]struct{}{}
	for i, user := range c.Users {
		if user.Name == "" {
			problems = append(problems, fmt.Sprintf("users[%d].name is required", i))
		}
		if user.Password == "" {
			problems = append(problems, fmt.Sprintf("users[%d].password is required", i))
		}
		if _, ok := seen[user.Name]; ok {
			problems = append(problems, fmt.Sprintf("users[%d].name duplicates %q", i, user.Name))
		}
		seen[user.Name] = struct{}{}
	}

	if c.Paths.ConfigDir == "" {
		problems = append(problems, "paths.config_dir is required")
	}
	if c.Paths.DataDir == "" {
		problems = append(problems, "paths.data_dir is required")
	}
	if c.Paths.RuntimeConfigFile == "" {
		problems = append(problems, "paths.runtime_config_file is required")
	}
	if c.Paths.DTSWConfigFile == "" {
		problems = append(problems, "paths.dtsw_config_file is required")
	}
	if c.Paths.DTSWBinary == "" {
		problems = append(problems, "paths.dtsw_binary is required")
	}
	if c.Paths.XrayBinary == "" {
		problems = append(problems, "paths.xray_binary is required")
	}
	if c.Paths.CaddyBinary == "" {
		problems = append(problems, "paths.caddy_binary is required")
	}
	if c.Paths.CaddyConfigFile == "" {
		problems = append(problems, "paths.caddy_config_file is required")
	}
	if c.Paths.ACMEBinary == "" {
		problems = append(problems, "paths.acme_binary is required")
	}
	if c.Paths.ACMEEnvFile == "" {
		problems = append(problems, "paths.acme_env_file is required")
	}
	if c.Paths.SystemdDir == "" {
		problems = append(problems, "paths.systemd_dir is required")
	}
	if c.Paths.RuntimeService == "" || c.Paths.FallbackService == "" || c.Paths.RenewService == "" || c.Paths.RenewTimer == "" {
		problems = append(problems, "all service names in paths must be set")
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (c *Config) AddUser(name, password string) error {
	if name == "" {
		return errors.New("user name is required")
	}
	if password == "" {
		return errors.New("user password is required")
	}
	for _, user := range c.Users {
		if user.Name == name {
			return fmt.Errorf("user %q already exists", name)
		}
	}
	c.Users = append(c.Users, User{Name: name, Password: password})
	return nil
}

func (c *Config) DeleteUser(name string) error {
	if len(c.Users) == 1 && c.Users[0].Name == name {
		return errors.New("cannot remove the last user")
	}
	for i, user := range c.Users {
		if user.Name == name {
			c.Users = append(c.Users[:i], c.Users[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("user %q does not exist", name)
}

func (c Config) User(name string) (User, bool) {
	for _, user := range c.Users {
		if user.Name == name {
			return user, true
		}
	}
	return User{}, false
}

func validateDomain(domain string) error {
	if domain == "" {
		return errors.New("server.domain is required")
	}
	if net.ParseIP(domain) != nil {
		return errors.New("server.domain must be a DNS name, not an IP address")
	}
	if strings.Contains(domain, "://") {
		return errors.New("server.domain must not include a scheme")
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return errors.New("server.domain must be a fully qualified domain name")
	}
	for _, label := range labels {
		if label == "" {
			return errors.New("server.domain contains an empty label")
		}
	}
	return nil
}
