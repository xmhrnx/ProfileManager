package adapter

import (
	"fmt"
	"path"
	"strings"
)

// Glob matches slash-separated relative paths. "**" matches any
// number of folders; other segments follow path.Match. A pattern
// without a slash matches the name at any depth ("*.log").
type Glob struct {
	segs []string
	any  bool
}

func compileGlob(p string) (Glob, error) {
	p = strings.Trim(strings.ReplaceAll(p, `\`, "/"), "/")
	if p == "" {
		return Glob{}, fmt.Errorf("empty exclude pattern")
	}
	segs := strings.Split(p, "/")
	for _, s := range segs {
		if s == "**" {
			continue
		}
		if _, err := path.Match(s, ""); err != nil {
			return Glob{}, fmt.Errorf("exclude pattern %q: %w", p, err)
		}
	}
	return Glob{segs: segs, any: len(segs) == 1}, nil
}

// Match reports whether rel (slash-separated) matches.
func (g Glob) Match(rel string) bool {
	parts := strings.Split(strings.Trim(rel, "/"), "/")
	if g.any {
		ok, _ := path.Match(g.segs[0], parts[len(parts)-1])
		return ok
	}
	return matchSegs(g.segs, parts)
}

func matchSegs(pat, parts []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			rest := pat[1:]
			for i := 0; i <= len(parts); i++ {
				if matchSegs(rest, parts[i:]) {
					return true
				}
			}
			return false
		}
		if len(parts) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], parts[0]); !ok {
			return false
		}
		pat, parts = pat[1:], parts[1:]
	}
	return len(parts) == 0
}

// Excluder reports whether a relative path inside a folder is left out.
type Excluder []Glob

// NewExcluder compiles patterns; invalid ones are reported by Validate.
func NewExcluder(patterns []string) Excluder {
	var e Excluder
	for _, p := range patterns {
		if g, err := compileGlob(p); err == nil {
			e = append(e, g)
		}
	}
	return e
}

// Excluded reports whether rel, or a folder containing it, is left out.
func (e Excluder) Excluded(rel string) bool {
	rel = strings.ReplaceAll(rel, `\`, "/")
	for _, g := range e {
		if g.Match(rel) {
			return true
		}
	}
	return false
}
