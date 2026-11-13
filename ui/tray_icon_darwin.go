//go:build darwin

package ui

import (
	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/profilemanager/assets"
)

// setTrayIcon uses the monochrome template, which macOS tints to match
// a light or dark menu bar.
func setTrayIcon(t *giode.Tray, _ []byte) {
	if data, err := assets.FS.ReadFile("tray-template.png"); err == nil {
		t.SetTemplateIcon(data)
	}
}
