package systemd

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRestartOrStartDryRunPrintsPseudoCommand(t *testing.T) {
	var out bytes.Buffer
	if err := RestartOrStart(context.Background(), CommandOptions{DryRun: true, Stdout: &out}, "dtsw-xray.service", "dtsw-renew.timer"); err != nil {
		t.Fatalf("RestartOrStart returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"systemctl restart-or-start dtsw-xray.service",
		"systemctl restart-or-start dtsw-renew.timer",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, text)
		}
	}
}
