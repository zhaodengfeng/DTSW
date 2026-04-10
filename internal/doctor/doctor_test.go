package doctor

import "testing"

func TestEvaluateServiceStatusActiveService(t *testing.T) {
	result := evaluateServiceStatus("dtsw-xray.service", true, true, true)
	if result.Severity != SeverityPass {
		t.Fatalf("expected PASS, got %s", result.Severity)
	}
}

func TestEvaluateServiceStatusEnabledButInactiveServiceFails(t *testing.T) {
	result := evaluateServiceStatus("dtsw-xray.service", true, false, true)
	if result.Severity != SeverityFail {
		t.Fatalf("expected FAIL, got %s", result.Severity)
	}
}

func TestEvaluateServiceStatusMissingServiceWarns(t *testing.T) {
	result := evaluateServiceStatus("dtsw-xray.service", false, false, true)
	if result.Severity != SeverityWarn {
		t.Fatalf("expected WARN, got %s", result.Severity)
	}
}

func TestEvaluateServiceStatusTimerRequiresEnabledAndActiveForPass(t *testing.T) {
	result := evaluateServiceStatus("dtsw-renew.timer", true, false, false)
	if result.Severity != SeverityWarn {
		t.Fatalf("expected WARN, got %s", result.Severity)
	}
}
