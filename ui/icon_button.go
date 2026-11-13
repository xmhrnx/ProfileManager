package ui

import (
	"github.com/xmhrnx/giode"
	props "github.com/xmhrnx/giode/styles/properties"
)

// IconButton is a square, quiet button holding one icon, with a
// tooltip naming what it does.
type IconButton struct {
	icon   string
	size   int
	danger bool
	active bool
	tip    giode.Element
	press  *giode.PressableElement
}

func newIconButton(iconName, label string, onClick func(e giode.ClickEvent)) *IconButton {
	b := &IconButton{icon: iconName, size: 32}
	b.press = giode.Pressable(b.content).OnClick(onClick).Label(label)
	b.tip = giode.Tooltip(b.press, label, giode.Styles{FontSize: 12})
	return b
}

// Danger tints the hover state red.
func (b *IconButton) Danger() *IconButton { b.danger = true; return b }

// Size sets the button's side in dp.
func (b *IconButton) Size(dp int) *IconButton { b.size = dp; return b }

// SetIcon swaps the icon.
func (b *IconButton) SetIcon(name string) { b.icon = name }

// SetActive highlights the button (a toggled state).
func (b *IconButton) SetActive(v bool) { b.active = v }

func (b *IconButton) Layout(gtx giode.Context) giode.Dimensions { return b.tip.Layout(gtx) }

func (b *IconButton) content(s giode.PressState) giode.Element {
	fg, bg := colMuted, giode.Color("transparent")
	if b.active {
		fg, bg = colPrimary, colPrimarySoft
	}
	switch {
	case s.Pressed:
		bg, fg = colPressed, colText
	case s.Hovered && b.danger:
		bg, fg = colDangerSoft, colDanger
	case s.Hovered:
		bg, fg = colHover, colText
	}
	return giode.Box(giode.Styles{
		Width: props.Width(b.size), Height: props.Height(b.size), Align: giode.AlignCenter, Justify: giode.JustifyCenter,
		BorderRadius: 8, Background: bg, Cursor: giode.CursorPointer,
	}, icon(b.icon, b.size/2, fg))
}
