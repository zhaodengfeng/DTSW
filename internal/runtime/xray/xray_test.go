package xray

import (
	"encoding/json"
	"testing"

	"github.com/zhaodengfeng/dtsw/internal/config"
)

func TestRenderIncludesUsersAndTLSPaths(t *testing.T) {
	cfg := config.Example("trojan.example.com", "admin@example.com", "secret")
	data, err := Renderer{}.Render(cfg)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("rendered config is not valid json: %v", err)
	}

	inbounds, ok := decoded["inbounds"].([]any)
	if !ok || len(inbounds) != 2 {
		t.Fatalf("expected two inbounds (trojan + api), got %d", len(inbounds))
	}

	if _, ok := decoded["stats"]; !ok {
		t.Fatal("rendered config missing stats section")
	}
	if _, ok := decoded["api"]; !ok {
		t.Fatal("rendered config missing api section")
	}
	if _, ok := decoded["policy"]; !ok {
		t.Fatal("rendered config missing policy section")
	}
	if _, ok := decoded["routing"]; !ok {
		t.Fatal("rendered config missing routing section")
	}
}
