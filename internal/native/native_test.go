package native

import (
	"context"
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
