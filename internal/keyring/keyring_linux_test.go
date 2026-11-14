//go:build linux && keyring_integration

package keyring

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// Run inside a session with an unlocked Secret Service, for example:
//
//	dbus-run-session -- sh -c 'echo pw | gnome-keyring-daemon --unlock --components=secrets >/dev/null &&
//	    go test -tags keyring_integration ./internal/keyring'
func TestSecretServiceWithAnotherProgramsEntry(t *testing.T) {
	kr := System()
	it := Item{Service: "pm-test", Account: "alice"}
	t.Cleanup(func() { kr.Delete(it) })

	// Another program stores a secret the way libsecret tools do.
	cmd := exec.Command("secret-tool", "store", "--label=pm test", "service", "pm-test", "username", "alice")
	cmd.Stdin = strings.NewReader("from-other-program")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("secret-tool store: %v %s", err, out)
	}
	got, err := kr.Get(it)
	if err != nil || string(got) != "from-other-program" {
		t.Fatalf("Get = %q, %v", got, err)
	}

	// Replacing it must update the entry the other program reads.
	if err := kr.Set(it, []byte("switched")); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("secret-tool", "lookup", "service", "pm-test", "username", "alice").Output()
	if err != nil || string(out) != "switched" {
		t.Fatalf("secret-tool lookup = %q, %v", out, err)
	}

	// Custom attributes, for programs that don't use service/username.
	custom := Item{Attributes: map[string]string{"application": "pm-test-app", "account": "bob"}}
	if err := kr.Set(custom, []byte("attr-secret")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Delete(custom) })
	if got, err := kr.Get(custom); err != nil || string(got) != "attr-secret" {
		t.Fatalf("custom Get = %q, %v", got, err)
	}

	if err := kr.Delete(it); err != nil {
		t.Fatal(err)
	}
	if _, err := kr.Get(it); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after Delete: %v", err)
	}
	if err := kr.Delete(it); err != nil {
		t.Fatalf("deleting a missing entry: %v", err)
	}
}
