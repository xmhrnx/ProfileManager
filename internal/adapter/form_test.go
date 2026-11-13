package adapter

import (
	"reflect"
	"testing"
)

func TestFormRoundTrip(t *testing.T) {
	f := Form{
		Name:              "My Tool",
		Files:             "~/.mytool/auth.json env=auth.json\n# a comment\n{home}/.mytool.json keys=user,token\n~/.config/mytool exclude=cache/**,*.log",
		Keychain:          "mytool | {user}",
		Env:               "MYTOOL_HOME={profile_dir}",
		Processes:         "mytool, MyTool",
		CloseBeforeSwitch: true,
	}
	a, err := f.App(App{})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "my-tool" || len(a.Files) != 3 || a.Files[1].JSONKeys[1] != "token" || a.Files[2].Exclude[0] != "cache/**" {
		t.Fatalf("%+v", a)
	}
	if a.Files[0].ID == a.Files[1].ID {
		t.Fatal("duplicate item IDs")
	}
	if !Representable(a) {
		t.Fatal("form-made app not representable")
	}
	back, err := FormFrom(a).App(a)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, a) {
		t.Fatalf("round trip changed the app:\n%+v\n%+v", a, back)
	}
}

func TestFormKeepsItemIDs(t *testing.T) {
	base := App{ID: "x", Name: "X", Files: []FileSpec{{ID: "credentials", Paths: map[string][]string{"default": {"~/.x/creds"}}}}}
	f := FormFrom(base)
	f.Files = "~/.x/creds-moved"
	a, err := f.App(base)
	if err != nil {
		t.Fatal(err)
	}
	if a.Files[0].ID != "credentials" {
		t.Fatalf("item ID changed to %q", a.Files[0].ID)
	}
}

func TestBuiltinsWithPerOSPathsUseJSON(t *testing.T) {
	apps, _ := Builtins()
	for _, a := range apps {
		if a.ID == "wrangler" && Representable(a) {
			t.Fatal("wrangler has per-OS paths and shouldn't be form-editable")
		}
	}
}

func TestSplitArgs(t *testing.T) {
	got, err := SplitArgs(`gcloud config list --format="value(name)" 'a b'`)
	want := []string{"gcloud", "config", "list", "--format=value(name)", "a b"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := SplitArgs(`a "b`); err == nil {
		t.Fatal("unclosed quote accepted")
	}
}
