package config

import "testing"

func TestExampleIsValid(t *testing.T) {
	cfg := Example("trojan.example.com", "admin@example.com", "secret")
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid example config, got %v", err)
	}
}

func TestDNSChallengeRequiresProvider(t *testing.T) {
	cfg := Example("trojan.example.com", "admin@example.com", "secret")
	cfg.TLS.Challenge = ChallengeDNS01
	cfg.TLS.DNSProvider = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected dns-01 validation failure when dns_provider is empty")
	}
}

func TestAddUser(t *testing.T) {
	cfg := Example("trojan.example.com", "admin@example.com", "secret")
	if err := cfg.AddUser("secondary", "s3cret"); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if len(cfg.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(cfg.Users))
	}
	user, ok := cfg.User("secondary")
	if !ok {
		t.Fatal("expected to find newly added user")
	}
	if user.Password != "s3cret" {
		t.Fatalf("expected password s3cret, got %q", user.Password)
	}
}

func TestDeleteUser(t *testing.T) {
	cfg := Example("trojan.example.com", "admin@example.com", "secret")
	if err := cfg.AddUser("secondary", "s3cret"); err != nil {
		t.Fatalf("AddUser returned error: %v", err)
	}
	if err := cfg.DeleteUser("secondary"); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	if _, ok := cfg.User("secondary"); ok {
		t.Fatal("expected user to be removed")
	}
}

func TestDeleteLastUserFails(t *testing.T) {
	cfg := Example("trojan.example.com", "admin@example.com", "secret")
	if err := cfg.DeleteUser("primary"); err == nil {
		t.Fatal("expected deleting the last user to fail")
	}
}
