package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/profilemanager/internal/fsutil"
	"github.com/xmhrnx/profilemanager/internal/vault"
)

const keyName = "vault-master-key"

// keySource picks where the vault's master key lives: the OS credential
// store when there is one, else a private key file. The choice is
// remembered, so a credential store that is briefly unavailable (a
// locked keyring) never makes Profile Manager fall back silently.
func keySource(dataDir string) vault.KeySource {
	giode.SecureStore.Service = "com.xmhrnx.profilemanager"
	osStore := vault.FuncKeySource{
		Label:    "the OS credential store",
		Get:      func() ([]byte, error) { return giode.SecureStore.Get(keyName) },
		Set:      func(v []byte) error { return giode.SecureStore.Set(keyName, v) },
		NotFound: giode.ErrNotFound,
	}
	file := vault.FileKeySource{Path: filepath.Join(dataDir, "vault.key")}
	marker := filepath.Join(dataDir, "keystore")
	switch choice, _ := os.ReadFile(marker); strings.TrimSpace(string(choice)) {
	case "os":
		return osStore
	case "file":
		return file
	}
	// First run: probe the credential store.
	choice := "os"
	if _, err := osStore.Load(); err != nil && !errors.Is(err, vault.ErrNoKey) {
		choice = "file"
	}
	os.MkdirAll(dataDir, 0o700)
	fsutil.WriteFileAtomic(marker, []byte(choice+"\n"), 0o600)
	if choice == "file" {
		return file
	}
	return osStore
}
