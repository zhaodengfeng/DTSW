package stats

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateAccumulatesTraffic(t *testing.T) {
	s := &Store{Users: make(map[string]*UserStats)}
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)

	s.Update("alice", 100, 200, now)
	u := s.Users["alice"]
	if u.TotalUpload() != 100 {
		t.Fatalf("total upload = %d, want 100", u.TotalUpload())
	}
	if u.TotalDownload() != 200 {
		t.Fatalf("total download = %d, want 200", u.TotalDownload())
	}

	s.Update("alice", 300, 500, now)
	if u.TotalUpload() != 300 {
		t.Fatalf("total upload = %d, want 300", u.TotalUpload())
	}
	if u.TotalDownload() != 500 {
		t.Fatalf("total download = %d, want 500", u.TotalDownload())
	}
}

func TestUpdateDetectsRestart(t *testing.T) {
	s := &Store{Users: make(map[string]*UserStats)}
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)

	s.Update("alice", 1000, 2000, now)
	// Simulate Xray restart (counters drop).
	s.Update("alice", 50, 80, now)

	u := s.Users["alice"]
	if u.TotalUpload() != 1050 {
		t.Fatalf("total upload = %d, want 1050", u.TotalUpload())
	}
	if u.TotalDownload() != 2080 {
		t.Fatalf("total download = %d, want 2080", u.TotalDownload())
	}
}

func TestMonthlyTraffic(t *testing.T) {
	s := &Store{Users: make(map[string]*UserStats)}
	march := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	april := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	s.Update("bob", 100, 200, march)
	s.Update("bob", 300, 500, march)
	s.Update("bob", 500, 900, april)

	u := s.Users["bob"]
	up, down := u.CurrentMonthTraffic(march)
	if up != 300 || down != 500 {
		t.Fatalf("march traffic = %d/%d, want 300/500", up, down)
	}
	up, down = u.CurrentMonthTraffic(april)
	if up != 200 || down != 400 {
		t.Fatalf("april traffic = %d/%d, want 200/400", up, down)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")

	s := &Store{Users: make(map[string]*UserStats)}
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	s.Update("alice", 100, 200, now)

	if err := s.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadStore(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	u := loaded.Users["alice"]
	if u == nil {
		t.Fatal("alice not in loaded store")
	}
	if u.TotalUpload() != 100 || u.TotalDownload() != 200 {
		t.Fatalf("loaded total = %d/%d, want 100/200", u.TotalUpload(), u.TotalDownload())
	}
}

func TestLoadStoreNonExistent(t *testing.T) {
	s, err := LoadStore(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("load non-existent: %v", err)
	}
	if len(s.Users) != 0 {
		t.Fatalf("expected empty store, got %d users", len(s.Users))
	}
}

func TestLoadStoreInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadStore(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
