package xray

import (
	"encoding/json"
	"net"
	"strconv"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

type Renderer struct{}

func (Renderer) Name() string {
	return config.RuntimeXray
}

func (Renderer) Render(cfg config.Config) ([]byte, error) {
	type logConfig struct {
		LogLevel string `json:"loglevel"`
	}
	type user struct {
		Email    string `json:"email,omitempty"`
		Password string `json:"password"`
	}
	type fallback struct {
		Dest int `json:"dest"`
	}
	type inboundSettings struct {
		Clients   []user     `json:"clients,omitempty"`
		Fallbacks []fallback `json:"fallbacks,omitempty"`
		Address   string     `json:"address,omitempty"`
	}
	type certificate struct {
		CertificateFile string `json:"certificateFile"`
		KeyFile         string `json:"keyFile"`
	}
	type tlsSettings struct {
		ALPN         []string      `json:"alpn"`
		MinVersion   string        `json:"minVersion"`
		Certificates []certificate `json:"certificates"`
	}
	type streamSettings struct {
		Network     string      `json:"network"`
		Security    string      `json:"security"`
		TLSSettings tlsSettings `json:"tlsSettings"`
	}
	type sniffing struct {
		Enabled      bool     `json:"enabled"`
		DestOverride []string `json:"destOverride"`
	}
	type inbound struct {
		Listen         string          `json:"listen"`
		Port           int             `json:"port"`
		Protocol       string          `json:"protocol"`
		Tag            string          `json:"tag,omitempty"`
		Settings       inboundSettings `json:"settings"`
		StreamSettings *streamSettings `json:"streamSettings,omitempty"`
		Sniffing       *sniffing       `json:"sniffing,omitempty"`
	}
	type outbound struct {
		Protocol string `json:"protocol"`
		Tag      string `json:"tag,omitempty"`
	}
	type apiConfig struct {
		Tag      string   `json:"tag"`
		Services []string `json:"services"`
	}
	type systemPolicy struct {
		StatsInboundUplink   bool `json:"statsInboundUplink"`
		StatsInboundDownlink bool `json:"statsInboundDownlink"`
	}
	type policyConfig struct {
		System systemPolicy `json:"system"`
	}
	type routingRule struct {
		Type        string   `json:"type"`
		InboundTag  []string `json:"inboundTag"`
		OutboundTag string   `json:"outboundTag"`
	}
	type routingConfig struct {
		Rules []routingRule `json:"rules"`
	}
	type root struct {
		Log       logConfig     `json:"log"`
		Stats     struct{}      `json:"stats"`
		API       apiConfig     `json:"api"`
		Policy    policyConfig  `json:"policy"`
		Routing   routingConfig `json:"routing"`
		Inbounds  []inbound     `json:"inbounds"`
		Outbounds []outbound    `json:"outbounds"`
	}

	_, portText, err := net.SplitHostPort(cfg.Fallback.ListenAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, err
	}

	apiHost, apiPortText, err := net.SplitHostPort(cfg.Stats.APIListen)
	if err != nil {
		return nil, err
	}
	apiPort, err := strconv.Atoi(apiPortText)
	if err != nil {
		return nil, err
	}

	clients := make([]user, 0, len(cfg.Users))
	for _, u := range cfg.Users {
		clients = append(clients, user{
			Email:    u.Name,
			Password: u.Password,
		})
	}

	xrayConfig := root{
		Log: logConfig{
			LogLevel: "warning",
		},
		Stats: struct{}{},
		API: apiConfig{
			Tag:      "api",
			Services: []string{"StatsService"},
		},
		Policy: policyConfig{
			System: systemPolicy{
				StatsInboundUplink:   true,
				StatsInboundDownlink: true,
			},
		},
		Routing: routingConfig{
			Rules: []routingRule{
				{
					Type:        "field",
					InboundTag:  []string{"api"},
					OutboundTag: "api",
				},
			},
		},
		Inbounds: []inbound{
			{
				Listen:   cfg.Server.ListenHost,
				Port:     cfg.Server.Port,
				Protocol: "trojan",
				Tag:      "trojan-in",
				Settings: inboundSettings{
					Clients: clients,
					Fallbacks: []fallback{
						{Dest: port},
					},
				},
				StreamSettings: &streamSettings{
					Network:  "tcp",
					Security: "tls",
					TLSSettings: tlsSettings{
						ALPN:       cfg.Server.ALPN,
						MinVersion: "1.2",
						Certificates: []certificate{
							{
								CertificateFile: cfg.TLS.CertificateFile,
								KeyFile:         cfg.TLS.PrivateKeyFile,
							},
						},
					},
				},
				Sniffing: &sniffing{
					Enabled:      true,
					DestOverride: []string{"http", "tls"},
				},
			},
			{
				Listen:   apiHost,
				Port:     apiPort,
				Protocol: "dokodemo-door",
				Tag:      "api",
				Settings: inboundSettings{
					Address: apiHost,
				},
			},
		},
		Outbounds: []outbound{
			{Protocol: "freedom", Tag: "direct"},
			{Protocol: "blackhole", Tag: "block"},
		},
	}

	data, err := json.MarshalIndent(xrayConfig, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
