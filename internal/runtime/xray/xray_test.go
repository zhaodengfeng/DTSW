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
	if !ok || len(inbounds) != 1 {
		t.Fatalf("expected one inbound, got %#v", decoded["inbounds"])
	}
}
