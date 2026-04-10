package ioutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFileAtomicallyReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "dest.txt")

	if err := os.WriteFile(src, []byte("new content"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(dest, []byte("old content"), 0o600); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	if err := CopyFile(src, dest, 0o755); err != nil {
		t.Fatalf("CopyFile returned error: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "new content" {
		t.Fatalf("unexpected dest content: %q", string(data))
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("unexpected dest mode: %v", info.Mode().Perm())
	}
}
