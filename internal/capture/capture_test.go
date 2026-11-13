package capture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/keyring"
)

func testApp() *adapter.App {
	return &adapter.App{
		ID: "t", Name: "T",
		Files: []adapter.FileSpec{
			{ID: "token", Paths: map[string][]string{"default": {"{home}/.t/token"}}, EnvPath: "token"},
			{ID: "dir", Paths: map[string][]string{"default": {"{home}/.t/profile"}}, Exclude: []string{"Cache/**"}, EnvPath: "profile"},
			{ID: "cfg", Paths: map[string][]string{"default": {"{home}/.t.json"}}, JSONKeys: []string{"account"}, EnvPath: "t.json"},
		},
		Keychain: []adapter.KeychainSpec{{ID: "k", Service: "svc", Account: "{user}"}},
		Identity: &adapter.IdentitySpec{File: "cfg", JSON: "account.email"},
	}
}

func vars(home string) adapter.Vars {
	return adapter.Vars{GOOS: "linux", Values: map[string]string{"home": home, "user": "me"}}
}

func write(t *testing.T, p, s string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o700)
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCaptureRestoreRoundTrip(t *testing.T) {
	home := t.TempDir()
	app := testApp()
	kr := &keyring.Memory{}
	tgt := Live(app, vars(home), kr)

	write(t, filepath.Join(home, ".t/token"), "A-token")
	write(t, filepath.Join(home, ".t/profile/Cookies"), "A-cookies")
	write(t, filepath.Join(home, ".t/profile/Cache/big"), "cache")
	write(t, filepath.Join(home, ".t.json"), `{"theme":"dark","account":{"email":"a@x.com"},"projects":{"p":1}}`)
	kr.Set(keyring.Item{Service: "svc", Account: "me"}, []byte("A-secret"))

	a, err := Capture(tgt)
	if err != nil {
		t.Fatal(err)
	}
	if got := Identity(app, a); got != "a@x.com" {
		t.Fatalf("identity %q", got)
	}
	for _, it := range a.Items {
		for _, e := range it.Entries {
			if strings.HasPrefix(e.Path, "Cache") {
				t.Fatalf("excluded entry captured: %s", e.Path)
			}
		}
	}

	// Sign in as B: other content everywhere, a setting changed too.
	os.RemoveAll(filepath.Join(home, ".t"))
	write(t, filepath.Join(home, ".t/token"), "B-token")
	write(t, filepath.Join(home, ".t/profile/Cookies"), "B-cookies")
	write(t, filepath.Join(home, ".t.json"), `{"theme":"light","account":{"email":"b@x.com"},"projects":{"p":2}}`)
	kr.Set(keyring.Item{Service: "svc", Account: "me"}, []byte("B-secret"))
	b, err := Capture(tgt)
	if err != nil {
		t.Fatal(err)
	}
	if a.Hash() == b.Hash() {
		t.Fatal("different sign-ins hash the same")
	}

	if err := Restore(tgt, a); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(home, ".t/token")); got != "A-token" {
		t.Errorf("token %q", got)
	}
	if got := read(t, filepath.Join(home, ".t/profile/Cookies")); got != "A-cookies" {
		t.Errorf("cookies %q", got)
	}
	cfg := read(t, filepath.Join(home, ".t.json"))
	if !strings.Contains(cfg, `"a@x.com"`) || !strings.Contains(cfg, `"light"`) || !strings.Contains(cfg, `"p": 2`) {
		t.Errorf("JSON keys not merged: %s", cfg)
	}
	if strings.Index(cfg, "theme") > strings.Index(cfg, "projects") {
		t.Errorf("key order lost: %s", cfg)
	}
	if v, _ := kr.Get(keyring.Item{Service: "svc", Account: "me"}); string(v) != "A-secret" {
		t.Errorf("secret %q", v)
	}
	again, _ := Capture(tgt)
	if again.Hash() != a.Hash() {
		t.Error("restored state doesn't hash like the snapshot")
	}
}

func TestBlankRemovesSignIn(t *testing.T) {
	home := t.TempDir()
	app := testApp()
	kr := &keyring.Memory{}
	tgt := Live(app, vars(home), kr)
	write(t, filepath.Join(home, ".t/token"), "A")
	write(t, filepath.Join(home, ".t.json"), `{"theme":"dark","account":{"email":"a@x.com"}}`)
	kr.Set(keyring.Item{Service: "svc", Account: "me"}, []byte("s"))

	blank := Blank("t", []string{"token", "dir", "cfg"}, map[string]bool{"cfg": true}, []string{"k"})
	if !blank.Empty() {
		t.Fatal("blank not empty")
	}
	if err := Restore(tgt, blank); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".t/token")); !os.IsNotExist(err) {
		t.Error("token still there")
	}
	if cfg := read(t, filepath.Join(home, ".t.json")); strings.Contains(cfg, "account") || !strings.Contains(cfg, "theme") {
		t.Errorf("cfg %s", cfg)
	}
	if _, err := kr.Get(keyring.Item{Service: "svc", Account: "me"}); err != keyring.ErrNotFound {
		t.Error("secret still there")
	}
	now, _ := Capture(tgt)
	if !now.Empty() {
		t.Error("capture after blank isn't empty")
	}
}

func TestSymlinkedFileIsWrittenThrough(t *testing.T) {
	home := t.TempDir()
	app := &adapter.App{ID: "n", Name: "N", Files: []adapter.FileSpec{{ID: "rc", Paths: map[string][]string{"default": {"{home}/.npmrc"}}}}}
	real := filepath.Join(home, "dotfiles", "npmrc")
	write(t, real, "one")
	os.Symlink(real, filepath.Join(home, ".npmrc"))
	tgt := Live(app, vars(home), nil)
	s, _ := Capture(tgt)
	write(t, real, "two")
	if err := Restore(tgt, s); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(filepath.Join(home, ".npmrc")); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink replaced by a file")
	}
	if got := read(t, real); got != "one" {
		t.Fatalf("target %q", got)
	}
}

func TestFingerprintNotices(t *testing.T) {
	home := t.TempDir()
	app := testApp()
	tgt := Live(app, vars(home), nil)
	write(t, filepath.Join(home, ".t/token"), "A")
	os.MkdirAll(filepath.Join(home, ".t/profile/Cache"), 0o700)
	f1, _ := Fingerprint(tgt)
	write(t, filepath.Join(home, ".t/profile/Cache/x"), "ignored")
	f2, _ := Fingerprint(tgt)
	if f1 != f2 {
		t.Error("excluded file changed the fingerprint")
	}
	write(t, filepath.Join(home, ".t/token"), "AB")
	f3, _ := Fingerprint(tgt)
	if f3 == f1 {
		t.Error("change not noticed")
	}
}

func TestEncodeDecode(t *testing.T) {
	s := Blank("x", []string{"a"}, nil, nil)
	s.Items[0] = Item{ID: "a", Kind: KindFile, Data: []byte("hello")}
	data, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	d, err := Decode(data)
	if err != nil || d.Hash() != s.Hash() {
		t.Fatalf("decode: %v", err)
	}
}
