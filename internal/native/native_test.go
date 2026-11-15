package native

import (
	"context"
	"strings"
	"testing"

	"github.com/xmhrnx/profilemanager/internal/adapter"
)

const ghStatus = `github.com
  ✓ Logged in to github.com account octo (keyring)
  - Active account: false
  - Git operations protocol: https
  ✓ Logged in to github.com account work-octo (keyring)
  - Active account: true
`

func TestParseGhStatus(t *testing.T) {
	apps, _ := adapter.Builtins()
	var gh adapter.App
	for _, a := range apps {
		if a.ID == "gh" {
			gh = a
		}
	}
	d := Driver{Spec: gh.Native, Run: func(ctx context.Context, args []string) (string, error) {
		for _, a := range args {
			if a == "--active" {
				return "github.com\n  ✓ Logged in to github.com account work-octo (keyring)\n", nil
			}
		}
		return ghStatus, nil
	}}
	list, err := d.List(context.Background())
	if err != nil || len(list) != 2 || list[0] != "octo" || list[1] != "work-octo" {
		t.Fatalf("list %v %v", list, err)
	}
	cur, _ := d.Current(context.Background())
	if cur != "work-octo" {
		t.Fatalf("current %q", cur)
	}
}

func TestLinesAndExpand(t *testing.T) {
	if got := parse("a\n\n b \na\n", "", true); len(got) != 2 || got[1] != "b" {
		t.Fatalf("%v", got)
	}
	if got := expand([]string{"use", "{profile}"}, "x"); got[1] != "x" {
		t.Fatal(got)
	}
}

func builtin(t *testing.T, id string) adapter.App {
	t.Helper()
	apps, _ := adapter.Builtins()
	for _, a := range apps {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("no built-in %s", id)
	return adapter.App{}
}

func TestParseBuiltinOutputs(t *testing.T) {
	cases := []struct {
		app, out string
		list     []string
		current  string
	}{
		{"doctl", "default\nwork (current)\nclient\n", []string{"default", "work", "client"}, "work"},
		{"tailscale", "ID    Tailnet             Account\n4a1b  example.com         me@example.com*\n7c2d  corp.example        me@corp.example\n",
			[]string{"me@example.com", "me@corp.example"}, "me@example.com"},
		{"kubectl", "dev\nprod\n", []string{"dev", "prod"}, "dev\n"},
	}
	for _, c := range cases {
		a := builtin(t, c.app)
		d := Driver{Spec: a.Native, Run: func(context.Context, []string) (string, error) { return c.out, nil }}
		list, err := d.List(context.Background())
		if err != nil || strings.Join(list, ",") != strings.Join(c.list, ",") {
			t.Errorf("%s list = %v, %v", c.app, list, err)
		}
		if c.app == "kubectl" {
			continue
		}
		if cur, _ := d.Current(context.Background()); cur != c.current {
			t.Errorf("%s current = %q", c.app, cur)
		}
	}
}
