package ui

import (
	"fmt"

	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/profilemanager/internal/engine"
)

// AppRow is one app in the sidebar: its tile, name, the signed-in
// profile and a status light.
type AppRow struct {
	ui    *UI
	id    string
	press *giode.PressableElement
}

func newAppRow(u *UI, id string) *AppRow {
	r := &AppRow{ui: u, id: id}
	r.press = giode.Pressable(r.content).OnClick(func(giode.ClickEvent) { u.selectApp(id) })
	return r
}

func (r *AppRow) Layout(gtx giode.Context) giode.Dimensions {
	if v, ok := r.ui.view(r.id); ok {
		r.press.Label(v.App.Name)
	}
	return r.press.Layout(gtx)
}

func (r *AppRow) content(s giode.PressState) giode.Element {
	v, ok := r.ui.view(r.id)
	if !ok {
		return giode.Box(giode.Styles{})
	}
	selected := r.ui.selected == r.id
	bg := giode.Color("transparent")
	switch {
	case selected:
		bg = colPrimarySoft
	case s.Pressed:
		bg = colPressed
	case s.Hovered:
		bg = colHover
	}
	sub, subColor := rowSubtitle(v)
	nameColor := colText
	if !v.State.Pinned && !selected {
		nameColor = giode.Mix("theme.text", "theme.surface", 0.8)
	}
	return giode.Row(giode.Styles{
		Width: -1, Gap: 12, Align: giode.AlignCenter, Padding: giode.SymmetricInset(10, 8),
		BorderRadius: 10, Background: bg, Cursor: giode.CursorPointer, Wrap: giode.WrapNoWrap,
	},
		appTile(glyphFor(v.App.Glyph, v.App.Name), v.App.Color, 32),
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: 1},
			txt(v.App.Name, giode.Styles{FontSize: 13, FontWeight: giode.FontWeightMedium, Color: nameColor, TextOverflow: giode.TextOverflowEllipsis}),
			txt(sub, giode.Styles{FontSize: 11, Color: subColor, TextOverflow: giode.TextOverflowEllipsis}),
		),
		statusLight(v),
	)
}

// rowSubtitle says who's signed in, or what the app is.
func rowSubtitle(v engine.AppView) (string, giode.Color) {
	if p, ok := activeProfile(v); ok {
		return p.Name, colMuted
	}
	if n := len(profilesOf(v)); n > 0 {
		return fmt.Sprintf("%d profiles · none active", n), colSubtle
	}
	return v.App.Category, colSubtle
}

// statusLight shows the app's state at a glance.
func statusLight(v engine.AppView) giode.Element {
	switch {
	case v.State.Conflict != nil || v.State.SyncError != "":
		return dot(colDanger, 8)
	case len(v.Running) > 0 && v.App.CloseBeforeSwitch:
		return dot(colWarning, 8)
	}
	if _, ok := activeProfile(v); ok {
		return dot(colSuccess, 8)
	}
	return giode.Box(giode.Styles{Width: 8})
}
