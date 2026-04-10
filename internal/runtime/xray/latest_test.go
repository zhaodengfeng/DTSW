package xray

import "testing"

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "v26.1.13", want: "v26.1.13"},
		{in: "26.1.13", want: "v26.1.13"},
		{in: " 26.1.13 ", want: "v26.1.13"},
	}
	for _, tt := range tests {
		got, err := normalizeVersion(tt.in)
		if err != nil {
			t.Fatalf("normalizeVersion(%q) returned error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("normalizeVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeVersionRejectsEmptyInput(t *testing.T) {
	if _, err := normalizeVersion(""); err == nil {
		t.Fatal("normalizeVersion returned nil error for empty input")
	}
}
