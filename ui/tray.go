package ui

import (
	"bytes"
	"fmt"
	"image/png"

	"github.com/xmhrnx/giode"
)

// trayMenu is the tray icon's menu: one submenu per app with its
// profiles, so switching doesn't need the window.
type trayMenu struct {
	ui        *UI
	installed bool
	built     bool
	last      string
}

func newTrayMenu(u *UI) *trayMenu { return &trayMenu{ui: u} }

func (t *trayMenu) install() {
	if !giode.TraySupported() {
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, t.ui.logo); err == nil {
		setTrayIcon(t.ui.app.Tray(), buf.Bytes())
	}
	t.ui.app.Tray().SetTooltip("Profile Manager")
	t.installed = true
	t.update()
}

// update rebuilds the menu when what it shows has changed.
func (t *trayMenu) update() {
	if !t.installed {
		return
	}
	var items []giode.TrayItem
	sig := ""
	for _, v := range t.ui.apps {
		profiles := profilesOf(v)
		if !v.State.Pinned || len(profiles) == 0 {
			continue
		}
		var sub []giode.TrayItem
		for _, p := range profiles {
			appID, id := v.App.ID, p.ID
			active := isActive(v, p)
			sub = append(sub, giode.TrayCheckbox(p.Name, active, func() {
				if !active {
					t.ui.switchProfile(appID, id, false)
				}
			}))
			sig += fmt.Sprintf("%s/%s/%s/%v;", appID, id, p.Name, active)
		}
		label := v.App.Name
		if a, ok := activeProfile(v); ok {
			label += " — " + a.Name
		}
		items = append(items, giode.TraySubmenu(label, sub...))
	}
	if t.built && sig == t.last {
		return
	}
	t.built, t.last = true, sig
	app := t.ui.app
	if len(items) > 0 {
		items = append(items, giode.TraySeparator())
	}
	items = append(items,
		giode.TrayAction("Sync all now", t.ui.syncAll),
		giode.TrayAction("Open Profile Manager", app.ShowWindow),
		giode.TraySeparator(),
		giode.TrayAction("Quit", app.Quit),
	)
	app.Tray().SetMenu(items...)
}
