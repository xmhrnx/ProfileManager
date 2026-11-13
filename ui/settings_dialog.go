package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/dialog"
	"github.com/xmhrnx/giode/components/segmented"
	"github.com/xmhrnx/giode/components/toggle"

	"github.com/xmhrnx/profilemanager/internal/engine"
)

var (
	themes    = []string{"dark", "light", "system"}
	intervals = []int{15, 30, 60, 300}
)

// SettingsDialog holds the preferences.
type SettingsDialog struct {
	ui    *UI
	shown bool
	dlg   *dialog.Dialog

	theme    *segmented.Control
	every    *segmented.Control
	autoSync *toggle.Switch
	keychain *toggle.Switch
	tray     *toggle.Switch
	login    *toggle.Switch
	done     *ActionButton
	copyDir  *IconButton
}

func newSettingsDialog(u *UI) *SettingsDialog {
	s := &SettingsDialog{ui: u}
	set := func(fn func(*engine.Settings)) { u.do(func() error { return u.m.UpdateSettings(fn) }, nil) }
	sw := func(label string) *toggle.Switch {
		return giode.Input.Switch(label, giode.Styles{FontSize: 13, Color: colText})
	}

	s.theme = giode.Input.Segmented("Dark", "Light", "System").Styles(giode.Styles{Width: -1}).OnChange(func(i int) {
		u.applyTheme(themes[i])
		set(func(st *engine.Settings) { st.Theme = themes[i] })
	})
	s.every = giode.Input.Segmented("15 s", "30 s", "1 min", "5 min").Styles(giode.Styles{Width: -1}).OnChange(func(i int) {
		set(func(st *engine.Settings) { st.IntervalSeconds = intervals[i] })
	})
	s.autoSync = sw("Save refreshed sign-ins automatically").OnChange(func(on bool) {
		set(func(st *engine.Settings) { st.AutoSync = on })
	})
	s.keychain = sw("Include keychain entries in background checks").OnChange(func(on bool) {
		set(func(st *engine.Settings) { st.SyncKeychain = on })
	})
	s.tray = sw("Keep running in the tray when the window closes").OnChange(func(on bool) {
		u.app.SetCloseToTray(on)
		set(func(st *engine.Settings) { st.CloseToTray = on })
	})
	s.login = sw("Start at login").OnChange(func(on bool) {
		if err := u.app.SetLaunchAtLogin(on); err != nil {
			u.toasts.Show(giode.ToastError, friendly(err))
		}
	})
	s.done = newActionButton("Done", "check", kindPrimary, func() { s.shown = false })
	s.copyDir = newIconButton("copy", "Copy path", func(giode.ClickEvent) { u.copyText(u.m.DataDir(), "Path") }).Size(28)
	s.dlg = giode.Dialog(&s.shown, s.content, dialogStyles(560))
	return s
}

func (s *SettingsDialog) open() {
	set := s.ui.m.Settings()
	for i, t := range themes {
		if t == set.Theme {
			s.theme.SetSelected(i)
		}
	}
	s.every.SetSelected(1)
	for i, n := range intervals {
		if n == set.IntervalSeconds {
			s.every.SetSelected(i)
		}
	}
	s.autoSync.SetChecked(set.AutoSync)
	s.keychain.SetChecked(set.SyncKeychain)
	s.tray.SetChecked(set.CloseToTray)
	s.login.SetChecked(s.ui.app.LaunchAtLogin())
	s.shown = true
}

func (s *SettingsDialog) Layout(gtx giode.Context) giode.Dimensions { return s.dlg.Layout(gtx) }

func (s *SettingsDialog) content() giode.Element {
	group := func(kids ...giode.Element) giode.Element {
		return giode.Stack(giode.Styles{Width: -1, Gap: 14, Padding: giode.UniformInset(16), Background: colRaised, BorderRadius: 12, BorderWidth: 1, BorderColor: colBorder}, kids...)
	}
	return dialogBody("settings", "Settings", "Preferences are kept in the data folder.", []giode.Element{
		field("Appearance", "", s.theme),
		group(s.autoSync,
			field("Check every", "Only changed files are read; tokens refreshed by your tools land in the vault within this time.", s.every),
			s.keychain),
		group(s.tray, s.login),
		field("Vault", "Profiles are sealed with AES-256-GCM; the key is kept in "+s.ui.m.KeyLocation()+".",
			giode.Row(giode.Styles{Width: -1, Gap: 8, Align: giode.AlignCenter, Padding: giode.Inset{Left: 12, Right: 4, Top: 4, Bottom: 4}, Background: colBackground, BorderRadius: 9, BorderWidth: 1, BorderColor: colBorder, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{FlexGrow: 1}, txt(s.ui.m.DataDir(), giode.Styles{FontFamily: fontMono, FontSize: 12, TextOverflow: giode.TextOverflowEllipsis})),
				s.copyDir)),
	}, s.done)
}
