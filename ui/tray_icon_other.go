//go:build !darwin

package ui

import "github.com/xmhrnx/giode"

// setTrayIcon uses the full-color app mark.
func setTrayIcon(t *giode.Tray, logo []byte) { t.SetIcon(logo) }
