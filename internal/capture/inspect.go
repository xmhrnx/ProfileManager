package capture

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/xmhrnx/profilemanager/internal/adapter"
)

// Fingerprint cheaply summarizes the target's files (names, sizes,
// times, permissions) without reading them, so a background check can
// tell whether a full capture is worth doing. Keychain entries aren't
// included: reading them is the expensive part.
func Fingerprint(t Target) (string, error) {
	h := sha256.New()
	for i := range t.App.Files {
		f := &t.App.Files[i]
		path, ok := t.Locate(f)
		if !ok {
			continue
		}
		fmt.Fprintf(h, "#%s\n", f.ID)
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintln(h, "absent")
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			fmt.Fprintf(h, "%d %d %o\n", info.Size(), info.ModTime().UnixNano(), info.Mode())
			continue
		}
		ex := adapter.NewExcluder(f.Exclude)
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			rel, _ := filepath.Rel(path, p)
			rel = filepath.ToSlash(rel)
			if p != path && ex.Excluded(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			fmt.Fprintf(h, "%s %d %d %o\n", rel, info.Size(), info.ModTime().UnixNano(), info.Mode())
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Identity reads the signed-in account's name from a snapshot, or ""
// when the app defines no identity or the snapshot has none.
func Identity(app *adapter.App, s *Snapshot) string {
	spec := app.Identity
	if spec == nil || s == nil {
		return ""
	}
	var data []byte
	for _, it := range s.Items {
		if it.ID != spec.File {
			continue
		}
		switch it.Kind {
		case KindFile:
			data = it.Data
		case KindJSON:
			data, _ = json.Marshal(it.Keys)
		case KindDir:
			for _, e := range it.Entries {
				if e.Path == spec.Path {
					data = e.Data
				}
			}
		}
	}
	if len(data) == 0 {
		return ""
	}
	if spec.JSON != "" {
		return jsonLookup(data, spec.JSON)
	}
	re, err := regexp.Compile(spec.Regex)
	if err != nil {
		return ""
	}
	if m := re.FindSubmatch(data); len(m) > 1 {
		return strings.TrimSpace(string(m[1]))
	}
	return ""
}

// jsonLookup follows a dotted path ("a.b.0.c") into a JSON document and
// returns the string or number found there.
func jsonLookup(data []byte, path string) string {
	var v any
	if err := json.Unmarshal(bytes.TrimPrefix(data, bom), &v); err != nil {
		return ""
	}
	for _, part := range strings.Split(path, ".") {
		switch x := v.(type) {
		case map[string]any:
			v = x[part]
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(x) {
				return ""
			}
			v = x[i]
		default:
			return ""
		}
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}
