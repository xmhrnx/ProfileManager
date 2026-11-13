// Package ui is Profile Manager's window: a custom-framed, two-pane
// interface over the engine.
package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"strings"
	"time"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/input"
	"github.com/xmhrnx/giode/components/toaster"

	"github.com/xmhrnx/profilemanager/assets"
	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/engine"
)

// UI is the root view and the state it owns. Everything here runs on
// the UI goroutine; engine calls run in goroutines and post back.
type UI struct {
	app  *giode.App
	m    *engine.Manager
	logo image.Image

	apps     []engine.AppView
	selected string
	syncing  bool
	// busy marks profiles being switched to, by app/profile key.
	busy map[string]bool

	search *input.Input
	toasts *toaster.Toaster

	titleBar  *TitleBar
	sidebar   *Sidebar
	page      *AppPage
	welcome   *WelcomePage
	statusBar *StatusBar

	newProfile *NewProfileDialog
	editor     *AppEditorDialog
	settings   *SettingsDialog
	confirm    *ConfirmDialog
	running    *RunningDialog
	tray       *trayMenu
}

// New builds the interface for app over m.
func New(app *giode.App, m *engine.Manager) (*UI, error) {
	if err := setupTheme(); err != nil {
		return nil, err
	}
	logo, err := loadLogo()
	if err != nil {
		return nil, err
	}
	u := &UI{app: app, m: m, logo: logo, busy: map[string]bool{}}
	u.applyTheme(m.Settings().Theme)
	u.search = giode.Input.Text("Search apps", inputStyles()).OnChange(func(string) {})
	u.toasts = giode.Toaster(giode.Styles{BorderRadius: 12}).Placement(giode.ToastBottomEnd).
		Inset(giode.Inset{Bottom: 36, Right: 8}).Duration(5 * time.Second)
	u.titleBar = newTitleBar(u)
	u.sidebar = newSidebar(u)
	u.page = newAppPage(u)
	u.welcome = newWelcomePage(u)
	u.statusBar = newStatusBar(u)
	u.newProfile = newNewProfileDialog(u)
	u.editor = newAppEditorDialog(u)
	u.settings = newSettingsDialog(u)
	u.confirm = newConfirmDialog()
	u.running = newRunningDialog(u)
	u.tray = newTrayMenu(u)
	u.refresh()
	for _, v := range u.apps {
		if v.State.Pinned {
			u.selected = v.App.ID
			break
		}
	}
	m.SetOnChange(func() { app.Post(u.refresh) })
	return u, nil
}

func loadLogo() (image.Image, error) {
	data, err := fs.ReadFile(assets.FS, "logo.png")
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(data))
}

// Start begins background work: auto-sync, native profile discovery
// and the tray icon. It returns when ctx ends, after a last sync.
func (u *UI) Start(ctx context.Context) {
	u.tray.install()
	for _, v := range u.apps {
		if v.State.Pinned && v.Mode == adapter.ModeNative {
			go u.m.RefreshNative(ctx, v.App.ID)
		}
	}
	go u.m.CheckRunning()
	u.m.AutoSync(ctx, func(appID string, err error) {
		u.app.Post(func() { u.toasts.Show(giode.ToastWarning, fmt.Sprintf("Couldn't sync %s: %v", u.appName(appID), err)) })
	})
}

// refresh rereads the engine's view of every app.
func (u *UI) refresh() {
	u.apps = u.m.Apps()
	u.tray.update()
}

func (u *UI) view(id string) (engine.AppView, bool) {
	for _, v := range u.apps {
		if v.App.ID == id {
			return v, true
		}
	}
	return engine.AppView{}, false
}

func (u *UI) appName(id string) string {
	if v, ok := u.view(id); ok {
		return v.App.Name
	}
	return id
}

// View is the whole window, called every frame.
func (u *UI) View() giode.Element {
	giode.Shortcuts(
		giode.Shortcut("Mod+K", u.search.Focus),
		giode.Shortcut("Mod+,", u.settings.open),
		giode.Shortcut("F5", u.syncAll),
		giode.Shortcut("Mod+N", func() {
			if v, ok := u.view(u.selected); ok {
				u.newProfile.open(v)
			}
		}),
	)
	var main giode.Element = u.welcome
	if _, ok := u.view(u.selected); ok {
		main = u.page
	}
	frame := giode.Styles{Width: -1, Height: -1, Background: colBackground}
	if !u.app.Maximized() {
		frame.BorderWidth, frame.BorderColor = 1, colBorderStrong
	}
	return giode.Stack(frame,
		u.titleBar,
		giode.Box(giode.Styles{Width: -1, Height: 1, Background: colBorder}),
		giode.Row(giode.Styles{Width: -1, FlexGrow: 1, Wrap: giode.WrapNoWrap, Align: giode.AlignStretch},
			u.sidebar,
			giode.Box(giode.Styles{Width: 1, Height: -1, Background: colBorder}),
			giode.Box(giode.Styles{FlexGrow: 1, Height: -1}, main),
		),
		u.statusBar,
		u.newProfile, u.editor, u.settings, u.confirm, u.running,
		u.toasts,
	)
}

// selectApp shows an app's page.
func (u *UI) selectApp(id string) {
	u.selected = id
	if v, ok := u.view(id); ok && v.Mode == adapter.ModeNative {
		go u.m.RefreshNative(context.Background(), id)
	}
}

// do runs fn off the UI goroutine and reports an error as a toast;
// then (on the UI goroutine) runs after with fn's error.
func (u *UI) do(fn func() error, after func(err error)) {
	go func() {
		err := fn()
		u.app.Post(func() {
			if err != nil {
				u.toasts.Show(giode.ToastError, friendly(err))
			}
			if after != nil {
				after(err)
			}
		})
	}()
}

// friendly shortens engine errors for a toast.
func friendly(err error) string {
	msg := err.Error()
	if len(msg) > 280 {
		msg = msg[:280] + "…"
	}
	return msg
}

func busyKey(appID, profileID string) string { return appID + "/" + profileID }

// switchProfile signs an app in to a profile.
func (u *UI) switchProfile(appID, profileID string, quit bool) {
	key := busyKey(appID, profileID)
	if u.busy[key] {
		return
	}
	u.busy[key] = true
	var res engine.SwitchResult
	go func() {
		var err error
		res, err = u.m.Switch(context.Background(), appID, profileID, engine.SwitchOptions{QuitRunning: quit})
		u.app.Post(func() {
			delete(u.busy, key)
			var re *engine.RunningError
			switch {
			case errors.As(err, &re):
				u.running.open(appID, profileID, re)
			case err != nil:
				u.toasts.Show(giode.ToastError, friendly(err))
			default:
				name := u.profileName(appID, profileID)
				u.toasts.Show(giode.ToastSuccess, fmt.Sprintf("%s is signed in as %s", u.appName(appID), name))
				if res.SavedAs != "" {
					u.toasts.Show(giode.ToastInfo, fmt.Sprintf("The previous sign-in belonged to no profile, so it was saved as “%s”.", res.SavedAs))
				}
			}
		})
	}()
}

func (u *UI) profileName(appID, profileID string) string {
	if v, ok := u.view(appID); ok {
		for _, p := range v.State.Profiles {
			if p.ID == profileID {
				return p.Name
			}
		}
	}
	return profileID
}

// syncAll saves every active profile's latest sign-in now.
func (u *UI) syncAll() {
	if u.syncing {
		return
	}
	u.syncing = true
	go func() {
		errs := u.m.SyncAll(true)
		u.app.Post(func() {
			u.syncing = false
			if len(errs) == 0 {
				u.toasts.Show(giode.ToastSuccess, "Every active profile is up to date")
				return
			}
			var names []string
			for id := range errs {
				names = append(names, u.appName(id))
			}
			u.toasts.Show(giode.ToastWarning, "Couldn't sync "+strings.Join(names, ", "))
		})
	}()
}

// syncApp saves one app's latest sign-in now.
func (u *UI) syncApp(appID string) {
	go func() {
		status, err := u.m.Sync(appID, true)
		u.app.Post(func() {
			switch {
			case err != nil:
				u.toasts.Show(giode.ToastError, friendly(err))
			case status == engine.SyncSaved:
				u.toasts.Show(giode.ToastSuccess, "Saved the latest sign-in")
			case status == engine.SyncWaiting:
				u.toasts.Show(giode.ToastInfo, u.appName(appID)+" is running; it'll be synced once it's closed")
			case status == engine.SyncConflict:
				u.toasts.Show(giode.ToastWarning, "The sign-in changed to another account; choose what to keep")
			default:
				u.toasts.Show(giode.ToastSuccess, "Already up to date")
			}
		})
	}()
}

// toggleTheme flips between light and dark and remembers it.
func (u *UI) toggleTheme() {
	theme := "light"
	if giode.CurrentMode() == giode.ModeLight {
		theme = "dark"
	}
	u.applyTheme(theme)
	u.do(func() error { return u.m.UpdateSettings(func(s *engine.Settings) { s.Theme = theme }) }, nil)
}

// applyTheme sets the color mode for a theme setting.
func (u *UI) applyTheme(theme string) {
	switch theme {
	case "light":
		giode.SetMode(giode.ModeLight)
	case "system":
		if dark, err := giode.SystemDarkMode(); err == nil && dark {
			giode.SetMode(giode.ModeDark)
		} else {
			giode.SetMode(giode.ModeLight)
		}
	default:
		giode.SetMode(giode.ModeDark)
	}
}

// copyText puts text on the clipboard and says so.
func (u *UI) copyText(text, what string) {
	u.app.SetClipboard(text)
	u.toasts.Show(giode.ToastSuccess, what+" copied")
}
