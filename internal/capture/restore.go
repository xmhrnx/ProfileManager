package capture

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/fsutil"
	"github.com/xmhrnx/profilemanager/internal/keyring"
)

const defaultMode = 0o600

// Restore writes a snapshot over the target. Files are replaced
// atomically; folders are built next to the live one and swapped in
// with renames. Items the snapshot doesn't know about are left alone.
func Restore(t Target, s *Snapshot) error {
	byID := map[string]*Item{}
	for i := range s.Items {
		byID[s.Items[i].ID] = &s.Items[i]
	}
	for i := range t.App.Files {
		f := &t.App.Files[i]
		it, ok := byID[f.ID]
		if !ok {
			continue
		}
		path, ok := t.Locate(f)
		if !ok {
			continue
		}
		if err := restoreItem(f, path, it); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	secrets := map[string]*Secret{}
	for i := range s.Secrets {
		secrets[s.Secrets[i].ID] = &s.Secrets[i]
	}
	for _, k := range t.secrets() {
		sec, ok := secrets[k.ID]
		if !ok || sec.Unavailable {
			continue
		}
		it := item(k, t.Vars)
		var err error
		if sec.Present {
			err = t.Keyring.Set(it, sec.Value)
		} else {
			err = t.Keyring.Delete(it)
		}
		if errors.Is(err, keyring.ErrUnsupported) {
			continue
		}
		if err != nil {
			return fmt.Errorf("keychain %q: %w", it.Service, err)
		}
	}
	return nil
}

func restoreItem(f *adapter.FileSpec, path string, it *Item) error {
	// Write through a symlinked file or folder, keeping the link.
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	}
	switch it.Kind {
	case KindAbsent:
		return removeLive(path)
	case KindFile:
		return fsutil.WriteFileAtomic(path, it.Data, mode(it.Mode))
	case KindJSON:
		return restoreJSON(f, path, it)
	case KindDir:
		return restoreDir(path, it)
	}
	return fmt.Errorf("unknown item kind %q", it.Kind)
}

func mode(m uint32) os.FileMode {
	if m == 0 {
		return defaultMode
	}
	return os.FileMode(m) & os.ModePerm
}

func removeLive(path string) error {
	err := os.RemoveAll(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// restoreJSON sets the captured keys in the live file and removes the
// listed keys the snapshot doesn't have, keeping every other key.
func restoreJSON(f *adapter.FileSpec, path string, it *Item) error {
	data, err := os.ReadFile(path)
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if !exists && len(it.Keys) == 0 {
		return nil
	}
	obj, err := parseObject(data)
	if err != nil {
		return fmt.Errorf("reading JSON: %w", err)
	}
	for _, k := range f.JSONKeys {
		if v, ok := it.Keys[k]; ok {
			obj.set(k, v)
		} else {
			obj.del(k)
		}
	}
	out, err := obj.marshal()
	if err != nil {
		return err
	}
	perm := os.FileMode(defaultMode)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	return fsutil.WriteFileAtomic(path, out, perm)
}

// restoreDir builds the folder beside the live one, then swaps them, so
// a failure halfway leaves the live folder untouched.
func restoreDir(path string, it *Item) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	next := fsutil.TempSibling(path, "new")
	if err := os.Mkdir(next, mode(it.Mode)|0o700); err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(next)
		}
	}()
	for _, e := range it.Entries {
		if err := writeEntry(next, e); err != nil {
			return err
		}
	}
	if it.Mode != 0 {
		os.Chmod(next, os.FileMode(it.Mode)&os.ModePerm)
	}
	old := fsutil.TempSibling(path, "old")
	hadLive := true
	if err := os.Rename(path, old); errors.Is(err, fs.ErrNotExist) {
		hadLive = false
	} else if err != nil {
		return fmt.Errorf("moving the live folder aside (is the program still running?): %w", err)
	}
	if err := os.Rename(next, path); err != nil {
		if hadLive {
			os.Rename(old, path)
		}
		return err
	}
	ok = true
	if hadLive {
		os.RemoveAll(old)
	}
	return nil
}

func writeEntry(root string, e Entry) error {
	rel := filepath.FromSlash(e.Path)
	if !filepath.IsLocal(rel) {
		return fmt.Errorf("snapshot entry %q escapes its folder", e.Path)
	}
	p := filepath.Join(root, rel)
	switch {
	case e.Dir:
		return os.MkdirAll(p, mode(e.Mode)|0o700)
	case e.Link != "":
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		if err := os.Symlink(e.Link, p); err != nil && runtime.GOOS != "windows" {
			return err
		}
		return nil
	default:
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		return os.WriteFile(p, e.Data, mode(e.Mode))
	}
}

// Clear removes everything a target's folder-mode copy wrote, for the
// per-profile folders of commands run under a profile.
func Clear(dir string) error {
	if dir == "" || dir == "/" || strings.TrimSpace(dir) == "" {
		return fmt.Errorf("refusing to clear %q", dir)
	}
	return os.RemoveAll(dir)
}
