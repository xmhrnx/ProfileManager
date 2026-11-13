package adapter

import (
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Vars are the values {name} placeholders in paths, keychain entries
// and environment variables are replaced with.
type Vars struct {
	GOOS string
	// Values maps placeholder names to values: home, config,
	// xdg_config, xdg_data, appdata, localappdata, user, and the
	// per-use profile and profile_dir.
	Values map[string]string
	// Getenv reads environment variables for {env:NAME}.
	Getenv func(string) string
}

// SystemVars returns the placeholders for this computer.
func SystemVars() Vars {
	home, _ := os.UserHomeDir()
	cfg, _ := os.UserConfigDir()
	v := map[string]string{
		"home":         home,
		"config":       cfg,
		"xdg_config":   envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config")),
		"xdg_data":     envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share")),
		"appdata":      envOr("APPDATA", filepath.Join(home, "AppData", "Roaming")),
		"localappdata": envOr("LOCALAPPDATA", filepath.Join(home, "AppData", "Local")),
		"user":         currentUser(),
	}
	return Vars{GOOS: runtime.GOOS, Values: v, Getenv: os.Getenv}
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func currentUser() string {
	if u, err := user.Current(); err == nil {
		name := u.Username
		// Windows reports DOMAIN\name.
		if i := strings.LastIndex(name, `\`); i >= 0 {
			name = name[i+1:]
		}
		return name
	}
	return envOr("USER", os.Getenv("USERNAME"))
}

// With returns a copy with extra placeholder values.
func (v Vars) With(kv ...string) Vars {
	m := make(map[string]string, len(v.Values)+len(kv)/2)
	for k, val := range v.Values {
		m[k] = val
	}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	v.Values = m
	return v
}

var placeholder = regexp.MustCompile(`\{([a-z_]+|env:[A-Za-z_][A-Za-z0-9_]*)\}`)

// Expand replaces the placeholders in s. Unknown ones are left as is.
func (v Vars) Expand(s string) string {
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		name := m[1 : len(m)-1]
		if env, ok := strings.CutPrefix(name, "env:"); ok {
			if v.Getenv != nil {
				return v.Getenv(env)
			}
			return ""
		}
		if val, ok := v.Values[name]; ok {
			return val
		}
		return m
	})
}

// ExpandPath expands s and turns a leading ~ into the home folder.
func (v Vars) ExpandPath(s string) string {
	s = v.Expand(s)
	if s == "~" || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`) {
		s = v.Values["home"] + s[1:]
	}
	return filepath.Clean(filepath.FromSlash(s))
}

// Candidates returns the expanded candidate paths of f on v.GOOS: the
// OS's own list, then the "default" list.
func (f *FileSpec) Candidates(v Vars) []string {
	var out []string
	seen := map[string]bool{}
	for _, key := range []string{v.GOOS, "default"} {
		for _, p := range f.Paths[key] {
			e := v.ExpandPath(p)
			if !seen[e] {
				seen[e] = true
				out = append(out, e)
			}
		}
	}
	return out
}

// Resolve picks the path to use for f: the first candidate that
// exists, else the first candidate. ok is false when f has no path on
// this OS.
func (f *FileSpec) Resolve(v Vars) (path string, ok bool) {
	c := f.Candidates(v)
	if len(c) == 0 {
		return "", false
	}
	for _, p := range c {
		if _, err := os.Lstat(p); err == nil {
			return p, true
		}
	}
	return c[0], true
}

// Expanded returns k with placeholders replaced.
func (k KeychainSpec) Expanded(v Vars) KeychainSpec {
	k.Service = v.Expand(k.Service)
	k.Account = v.Expand(k.Account)
	k.Label = v.Expand(k.Label)
	if k.Attributes != nil {
		m := make(map[string]string, len(k.Attributes))
		for key, val := range k.Attributes {
			m[key] = v.Expand(val)
		}
		k.Attributes = m
	}
	return k
}
