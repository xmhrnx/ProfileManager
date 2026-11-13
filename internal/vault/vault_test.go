package vault

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAndTamper(t *testing.T) {
	dir := t.TempDir()
	ks := &MemoryKeySource{}
	v, err := Open(dir, ks)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Put("app", "p1", []byte("secret token")); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "app", "p1.pmv"))
	if len(raw) == 0 || string(raw) == "secret token" {
		t.Fatal("not encrypted")
	}
	got, err := v.Get("app", "p1")
	if err != nil || string(got) != "secret token" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	// A file moved to another profile fails authentication.
	os.WriteFile(filepath.Join(dir, "app", "p2.pmv"), raw, 0o600)
	if _, err := v.Get("app", "p2"); !errors.Is(err, ErrTampered) {
		t.Fatalf("moved file: %v", err)
	}
	raw[len(raw)-1] ^= 1
	os.WriteFile(filepath.Join(dir, "app", "p1.pmv"), raw, 0o600)
	if _, err := v.Get("app", "p1"); !errors.Is(err, ErrTampered) {
		t.Fatalf("tampered file: %v", err)
	}
	if _, err := v.Get("app", "../x"); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestKeyLostAndWrongKey(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir, &MemoryKeySource{}); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir, &MemoryKeySource{}); !errors.Is(err, ErrKeyLost) {
		t.Fatalf("missing key: %v", err)
	}
	other := &MemoryKeySource{Key: make([]byte, 32)}
	if _, err := Open(dir, other); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("wrong key: %v", err)
	}
}

func TestFileKeySource(t *testing.T) {
	dir := t.TempDir()
	ks := FileKeySource{Path: filepath.Join(dir, "k")}
	v, err := Open(filepath.Join(dir, "v"), ks)
	if err != nil {
		t.Fatal(err)
	}
	v.Put("a", "b", []byte("x"))
	v2, err := Open(filepath.Join(dir, "v"), ks)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := v2.Get("a", "b"); string(got) != "x" {
		t.Fatal("reopen failed")
	}
	if info, _ := os.Stat(ks.Path); info.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode %v", info.Mode())
	}
}
