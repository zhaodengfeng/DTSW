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
		Clients   []user     `json:"clients"`
		Fallbacks []fallback `json:"fallbacks"`
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
		Settings       inboundSettings `json:"settings"`
		StreamSettings streamSettings  `json:"streamSettings"`
		Sniffing       sniffing        `json:"sniffing"`
	}
	type outbound struct {
		Protocol string `json:"protocol"`
		Tag      string `json:"tag,omitempty"`
	}
	type root struct {
		Log       logConfig  `json:"log"`
		Inbounds  []inbound  `json:"inbounds"`
		Outbounds []outbound `json:"outbounds"`
	}

	_, portText, err := net.SplitHostPort(cfg.Fallback.ListenAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portText)
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
		Inbounds: []inbound{
			{
				Listen:   cfg.Server.ListenHost,
				Port:     cfg.Server.Port,
				Protocol: "trojan",
				Settings: inboundSettings{
					Clients: clients,
					Fallbacks: []fallback{
						{Dest: port},
					},
				},
				StreamSettings: streamSettings{
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
				Sniffing: sniffing{
					Enabled:      true,
					DestOverride: []string{"http", "tls"},
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
