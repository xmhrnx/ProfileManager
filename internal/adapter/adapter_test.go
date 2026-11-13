package adapter

import (
	"path/filepath"
	"testing"
)

func TestBuiltinsValidate(t *testing.T) {
	apps, err := Builtins()
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) < 15 {
		t.Fatalf("only %d built-ins", len(apps))
	}
	ids := map[string]bool{}
	for _, a := range apps {
		if ids[a.ID] {
			t.Errorf("duplicate id %s", a.ID)
		}
		ids[a.ID] = true
		for _, goos := range []string{"linux", "darwin", "windows"} {
			v := Vars{GOOS: goos, Values: map[string]string{"home": "/h"}}
			for _, f := range a.Files {
				if len(f.Candidates(v)) == 0 {
					t.Logf("%s: %s has no path on %s", a.ID, f.ID, goos)
				}
			}
		}
	}
	for _, want := range []string{"wrangler", "claude-code", "gh", "gcloud", "aws", "kubectl"} {
		if !ids[want] {
			t.Errorf("missing built-in %s", want)
		}
	}
}

func TestExpand(t *testing.T) {
	v := Vars{GOOS: "linux", Values: map[string]string{"home": "/home/a", "user": "a"}, Getenv: func(k string) string {
		if k == "X" {
			return "/x"
		}
		return ""
	}}
	cases := map[string]string{
		"{home}/.npmrc":     "/home/a/.npmrc",
		"~/.aws/config":     "/home/a/.aws/config",
		"{env:X}/y":         "/x/y",
		"{unknown}/z":       "{unknown}/z",
		"{home}/a/../b.txt": "/home/a/b.txt",
	}
	for in, want := range cases {
		if got := v.ExpandPath(in); got != filepath.FromSlash(want) {
			t.Errorf("ExpandPath(%q) = %q, want %q", in, got, want)
		}
	}
	if got := v.Expand("{user}"); got != "a" {
		t.Errorf("Expand user = %q", got)
	}
}

func TestExcluder(t *testing.T) {
	e := NewExcluder([]string{"Cache/**", "*.log", "a/**/tmp", "Singleton*"})
	yes := []string{"Cache", "Cache/x/y", "logs/z.log", "x.log", "a/tmp", "a/b/c/tmp", "SingletonLock"}
	no := []string{"Local Storage/leveldb/000.ldb", "Cookies", "a/b", "CacheX/y"}
	for _, p := range yes {
		if !e.Excluded(p) {
			t.Errorf("%s should be excluded", p)
		}
	}
	for _, p := range no {
		if e.Excluded(p) {
			t.Errorf("%s should be kept", p)
		}
	}
}

func TestValidateRejects(t *testing.T) {
	bad := []App{
		{ID: "Bad ID", Name: "x", Files: []FileSpec{{ID: "f", Paths: map[string][]string{"default": {"~/x"}}}}},
		{ID: "ok", Name: "", Files: []FileSpec{{ID: "f", Paths: map[string][]string{"default": {"~/x"}}}}},
		{ID: "ok", Name: "x"},
		{ID: "ok", Name: "x", Files: []FileSpec{{ID: "f", Paths: map[string][]string{"default": {"~/x"}}}, {ID: "f", Paths: map[string][]string{"default": {"~/y"}}}}},
		{ID: "ok", Name: "x", Native: &NativeSpec{List: []string{"a"}}},
	}
	for i, a := range bad {
		if err := a.Validate(); err == nil {
			t.Errorf("case %d: expected an error", i)
		}
	}
}
