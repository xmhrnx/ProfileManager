package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/titlebar"
)

const titleBarHeight = 48

// TitleBar is the custom window frame's top edge: Giode's title bar
// (drag handle, double-click to maximize, window buttons) carrying the
// logo, the search field and the global actions.
type TitleBar struct {
	ui    *UI
	bar   *titlebar.TitleBar
	sync  *IconButton
	theme *IconButton
}

func newTitleBar(u *UI) *TitleBar {
	t := &TitleBar{ui: u}
	t.sync = newIconButton("refresh-cw", "Sync all profiles now (F5)", func(giode.ClickEvent) { u.syncAll() })
	t.theme = newIconButton("sun", "Light or dark appearance", func(giode.ClickEvent) { u.toggleTheme() })
	settings := newIconButton("settings", "Settings (Ctrl+,)", func(giode.ClickEvent) { u.settings.open() })
	t.bar = giode.TitleBar(u.app, "Profile Manager", giode.Styles{
		Background: colSidebar, Color: colText, Height: titleBarHeight, FontSize: 13,
		FontWeight: giode.FontWeightSemiBold, Padding: giode.SymmetricInset(14, 0),
	}).
		Icon(giode.Image(u.logo, giode.Styles{Width: 26, Height: 26})).
		Leading(giode.Box(giode.Styles{Width: 340, Margin: giode.Inset{Left: 96}}, u.search)).
		Trailing(t.sync, t.theme, settings)
	return t
}

func (t *TitleBar) Layout(gtx giode.Context) giode.Dimensions {
	if giode.CurrentMode() == giode.ModeDark {
		t.theme.SetIcon("sun")
	} else {
		t.theme.SetIcon("moon")
	}
	t.sync.SetActive(t.ui.syncing)
	return t.bar.Layout(gtx)
}
