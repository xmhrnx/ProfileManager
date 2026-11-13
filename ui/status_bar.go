package ui

import (
	"strings"

	"github.com/xmhrnx/giode"
)

// StatusBar is the frame's bottom edge: how the vault is protected and
// what auto-sync is doing.
type StatusBar struct{ ui *UI }

func newStatusBar(u *UI) *StatusBar { return &StatusBar{ui: u} }

func (s *StatusBar) Layout(gtx giode.Context) giode.Dimensions {
	set := s.ui.m.Settings()
	key := s.ui.m.KeyLocation()
	keyColor, keyIcon := colMuted, "shield-check"
	if strings.HasPrefix(key, "a key file") {
		keyColor, keyIcon = colWarning, "triangle-alert"
	}
	sync := "Auto-sync off"
	syncColor := colSubtle
	if set.AutoSync {
		sync = "Auto-sync every " + interval(set.IntervalSeconds)
		syncColor = colMuted
	}
	if s.ui.syncing {
		sync = "Syncing…"
	}
	item := func(iconName, label string, c giode.Color) giode.Element {
		return giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter}, icon(iconName, 12, c), txt(label, giode.Styles{FontSize: 11, Color: c}))
	}
	return giode.Row(giode.Styles{
		Width: -1, Height: 30, Gap: 18, Align: giode.AlignCenter, Padding: giode.SymmetricInset(14, 0),
		Background: colSidebar, Wrap: giode.WrapNoWrap,
	},
		item(keyIcon, "Vault encrypted (AES-256-GCM) · key in "+key, keyColor),
		item("refresh-cw", sync, syncColor),
		spacer(),
		txt(s.ui.m.DataDir(), giode.Styles{FontFamily: fontMono, FontSize: 10, Color: colSubtle, TextOverflow: giode.TextOverflowEllipsis, MaxWidth: 420}),
	).Layout(gtx)
}
