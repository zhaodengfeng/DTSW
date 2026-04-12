package stats

import "testing"

func TestParseStatsOutputBraceFormat(t *testing.T) {
	output := `stat: {
  name: "user>>>alice>>>traffic>>>uplink"
  value: 12345
}
stat: {
  name: "user>>>alice>>>traffic>>>downlink"
  value: 67890
}
stat: {
  name: "user>>>bob>>>traffic>>>uplink"
  value: 111
}
stat: {
  name: "user>>>bob>>>traffic>>>downlink"
  value: 222
}
`
	results := ParseStatsOutput(output)
	if len(results) != 2 {
		t.Fatalf("expected 2 users, got %d", len(results))
	}
	byName := make(map[string]UserTraffic)
	for _, r := range results {
		byName[r.Name] = r
	}
	alice := byName["alice"]
	if alice.Upload != 12345 || alice.Download != 67890 {
		t.Fatalf("alice = %d/%d, want 12345/67890", alice.Upload, alice.Download)
	}
	bob := byName["bob"]
	if bob.Upload != 111 || bob.Download != 222 {
		t.Fatalf("bob = %d/%d, want 111/222", bob.Upload, bob.Download)
	}
}

func TestParseStatsOutputAngleFormat(t *testing.T) {
	output := `stat: <
  name: "user>>>alice>>>traffic>>>uplink"
  value: 100
>
stat: <
  name: "user>>>alice>>>traffic>>>downlink"
  value: 200
>
`
	results := ParseStatsOutput(output)
	if len(results) != 1 {
		t.Fatalf("expected 1 user, got %d", len(results))
	}
	if results[0].Upload != 100 || results[0].Download != 200 {
		t.Fatalf("alice = %d/%d, want 100/200", results[0].Upload, results[0].Download)
	}
}

func TestParseStatsOutputEmpty(t *testing.T) {
	results := ParseStatsOutput("")
	if len(results) != 0 {
		t.Fatalf("expected 0 users for empty output, got %d", len(results))
	}
}

func TestParseStatsOutputIgnoresNonUserStats(t *testing.T) {
	output := `stat: {
  name: "inbound>>>trojan-in>>>traffic>>>uplink"
  value: 99999
}
stat: {
  name: "user>>>alice>>>traffic>>>uplink"
  value: 50
}
stat: {
  name: "user>>>alice>>>traffic>>>downlink"
  value: 60
}
`
	results := ParseStatsOutput(output)
	if len(results) != 1 {
		t.Fatalf("expected 1 user, got %d", len(results))
	}
	if results[0].Upload != 50 || results[0].Download != 60 {
		t.Fatalf("alice = %d/%d, want 50/60", results[0].Upload, results[0].Download)
	}
}
