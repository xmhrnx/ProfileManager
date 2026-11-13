// Package fsutil holds the careful file operations the rest of the app
// relies on: writes that never leave a half-written file behind.
package fsutil

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
)

// WriteFileAtomic writes data to a temporary file next to name, flushes
// it to disk and renames it over name, so readers see the old content
// or the new, never a mix. A symlink at name is written through: the
// file it points to is replaced and the link is kept.
func WriteFileAtomic(name string, data []byte, perm os.FileMode) error {
	if target, err := filepath.EvalSymlinks(name); err == nil {
		name = target
	}
	dir := filepath.Dir(name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(name)+".pm-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()
	// Windows has no Unix permissions; the file inherits the folder's ACL.
	if err := tmp.Chmod(perm); err != nil && runtime.GOOS != "windows" {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, name); err != nil {
		return err
	}
	ok = true
	syncDir(dir)
	return nil
}

// TempSibling returns an unused name next to path with the given tag.
func TempSibling(path, tag string) string {
	var b [6]byte
	rand.Read(b[:])
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".pm-"+tag+"-"+hex.EncodeToString(b[:]))
}

// syncDir flushes a folder's entries, so a rename survives a crash.
// Not every system supports it; failures are harmless.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
}
