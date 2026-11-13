package adapter

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Form is an app definition as plain text fields, for the editor. It
// covers definitions whose paths are the same on every system; others
// are edited as JSON (Representable reports which).
type Form struct {
	Name, ID, Category, Color, Glyph, Description string
	// Files has one item per line: a path, then optional
	// "keys=a,b", "exclude=x,y" and "env=relative/path" options.
	Files string
	// Keychain has one "service | account" per line.
	Keychain string
	// Env has one NAME=value per line.
	Env string
	// Processes is a comma-separated list.
	Processes         string
	CloseBeforeSwitch bool
	// Native commands, each a command line.
	NativeList, NativeCurrent, NativeSwitch, NativeCreate string
	LoginHint                                             string
}

// Representable reports whether a can be edited as a Form without
// losing anything.
func Representable(a App) bool {
	for _, f := range a.Files {
		if len(f.Paths) != 1 || len(f.Paths["default"]) != 1 {
			return false
		}
		if strings.ContainsAny(f.Paths["default"][0], " \t") {
			return false
		}
	}
	for _, k := range a.Keychain {
		if len(k.OS) > 0 || len(k.Attributes) > 0 || k.Label != "" {
			return false
		}
	}
	for _, e := range a.Env {
		if len(e.OS) > 0 {
			return false
		}
	}
	if n := a.Native; n != nil && (n.ListRegex != "" || n.CurrentRegex != "") {
		return false
	}
	return true
}

// FormFrom fills a Form from a definition.
func FormFrom(a App) Form {
	f := Form{
		Name: a.Name, ID: a.ID, Category: a.Category, Color: a.Color, Glyph: a.Glyph,
		Description: a.Description, Processes: strings.Join(a.Processes, ", "),
		CloseBeforeSwitch: a.CloseBeforeSwitch, LoginHint: a.LoginHint,
	}
	var lines []string
	for _, fs := range a.Files {
		line := strings.Join(fs.Paths["default"], " ")
		if len(fs.JSONKeys) > 0 {
			line += " keys=" + strings.Join(fs.JSONKeys, ",")
		}
		if len(fs.Exclude) > 0 {
			line += " exclude=" + strings.Join(fs.Exclude, ",")
		}
		if fs.EnvPath != "" {
			line += " env=" + fs.EnvPath
		}
		lines = append(lines, line)
	}
	f.Files = strings.Join(lines, "\n")
	lines = nil
	for _, k := range a.Keychain {
		lines = append(lines, k.Service+" | "+k.Account)
	}
	f.Keychain = strings.Join(lines, "\n")
	lines = nil
	for _, e := range a.Env {
		lines = append(lines, e.Name+"="+e.Value)
	}
	f.Env = strings.Join(lines, "\n")
	if n := a.Native; n != nil {
		f.NativeList = joinArgs(n.List)
		f.NativeCurrent = joinArgs(n.Current)
		f.NativeSwitch = joinArgs(n.Switch)
		f.NativeCreate = joinArgs(n.Create)
	}
	return f
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slug turns a name into an app ID ("My Tool" -> "my-tool").
func Slug(name string) string {
	s := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(s) > 48 {
		s = s[:48]
	}
	if s == "" {
		s = "app"
	}
	return s
}

// App builds the definition, starting from base so fields the form
// doesn't show (identity, website, notes) are kept, as are the IDs of
// existing items, which saved profiles refer to.
func (f Form) App(base App) (App, error) {
	a := base.Clone()
	a.Name = strings.TrimSpace(f.Name)
	a.ID = strings.TrimSpace(f.ID)
	if a.ID == "" {
		a.ID = Slug(a.Name)
	}
	a.Category = strings.TrimSpace(f.Category)
	a.Color = strings.TrimSpace(f.Color)
	a.Glyph = strings.TrimSpace(f.Glyph)
	a.Description = strings.TrimSpace(f.Description)
	a.LoginHint = strings.TrimSpace(f.LoginHint)
	a.CloseBeforeSwitch = f.CloseBeforeSwitch
	a.Processes = splitList(f.Processes)

	files, err := parseFileLines(f.Files, base.Files)
	if err != nil {
		return App{}, err
	}
	a.Files = files

	a.Keychain = nil
	used := map[string]bool{}
	for _, fs := range a.Files {
		used[fs.ID] = true
	}
	for i, line := range lines(f.Keychain) {
		svc, acct, _ := strings.Cut(line, "|")
		k := KeychainSpec{Service: strings.TrimSpace(svc), Account: strings.TrimSpace(acct)}
		if i < len(base.Keychain) && !used[base.Keychain[i].ID] {
			k.ID = base.Keychain[i].ID
		} else {
			k.ID = uniqueID(used, "keychain")
		}
		used[k.ID] = true
		a.Keychain = append(a.Keychain, k)
	}

	a.Env = nil
	for _, line := range lines(f.Env) {
		name, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(name) == "" {
			return App{}, fmt.Errorf("environment variable %q: write NAME=value", line)
		}
		a.Env = append(a.Env, EnvSpec{Name: strings.TrimSpace(name), Value: strings.TrimSpace(value)})
	}

	a.Native = nil
	if strings.TrimSpace(f.NativeList+f.NativeSwitch+f.NativeCurrent+f.NativeCreate) != "" {
		n := &NativeSpec{}
		var errs []error
		n.List, errs = appendArgs(errs, f.NativeList)
		n.Current, errs = appendArgs(errs, f.NativeCurrent)
		n.Switch, errs = appendArgs(errs, f.NativeSwitch)
		n.Create, errs = appendArgs(errs, f.NativeCreate)
		if len(errs) > 0 {
			return App{}, errs[0]
		}
		a.Native = n
	}
	if err := a.Validate(); err != nil {
		return App{}, err
	}
	return a, nil
}

func parseFileLines(text string, base []FileSpec) ([]FileSpec, error) {
	var out []FileSpec
	used := map[string]bool{}
	for i, line := range lines(text) {
		fields := strings.Fields(line)
		fs := FileSpec{Paths: map[string][]string{"default": {fields[0]}}}
		for _, opt := range fields[1:] {
			k, v, ok := strings.Cut(opt, "=")
			if !ok {
				return nil, fmt.Errorf("%q: options look like keys=a,b, exclude=x,y or env=path (paths with spaces need the JSON editor)", opt)
			}
			switch k {
			case "keys":
				fs.JSONKeys = splitList(v)
			case "exclude":
				fs.Exclude = splitList(v)
			case "env":
				fs.EnvPath = v
			default:
				return nil, fmt.Errorf("unknown option %q on %s", k, fields[0])
			}
		}
		if i < len(base) && !used[base[i].ID] {
			fs.ID = base[i].ID
		} else {
			fs.ID = uniqueID(used, Slug(strings.TrimPrefix(filepath.Base(fields[0]), ".")))
		}
		used[fs.ID] = true
		out = append(out, fs)
	}
	return out, nil
}

func uniqueID(used map[string]bool, base string) string {
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		id := fmt.Sprintf("%s-%d", base, i)
		if !used[id] {
			return id
		}
	}
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func appendArgs(errs []error, s string) ([]string, []error) {
	args, err := SplitArgs(s)
	if err != nil {
		errs = append(errs, err)
	}
	return args, errs
}

// SplitArgs splits a command line into arguments, honoring double and
// single quotes.
func SplitArgs(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inArg := false
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inArg = r, true
		case r == ' ' || r == '\t':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote in %q", s)
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}

func joinArgs(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\"") {
			a = "'" + a + "'"
		}
		out[i] = a
	}
	return strings.Join(out, " ")
}
