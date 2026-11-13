package ui

import (
	"github.com/xmhrnx/giode"
	props "github.com/xmhrnx/giode/styles/properties"
)

// buttonKind is an ActionButton's look.
type buttonKind int

const (
	// kindPrimary is the brand gradient with a soft glow.
	kindPrimary buttonKind = iota
	// kindSecondary is a raised neutral button.
	kindSecondary
	// kindGhost shows only its label until hovered.
	kindGhost
	// kindDanger is for destructive actions.
	kindDanger
)

// ActionButton is the app's button: an optional icon and a label, in
// one of a few looks, with hover, press, busy and disabled states.
type ActionButton struct {
	label    string
	icon     string
	kind     buttonKind
	busy     bool
	disabled bool
	compact  bool
	grow     bool
	press    *giode.PressableElement
}

func newActionButton(label, iconName string, kind buttonKind, onClick func()) *ActionButton {
	b := &ActionButton{label: label, icon: iconName, kind: kind}
	b.press = giode.Pressable(b.content).OnClick(func(giode.ClickEvent) {
		if !b.busy && !b.disabled {
			onClick()
		}
	}).Label(label)
	return b
}

// SetLabel replaces the label.
func (b *ActionButton) SetLabel(s string) *ActionButton { b.label = s; b.press.Label(s); return b }

// SetBusy shows a spinner and ignores clicks.
func (b *ActionButton) SetBusy(v bool) *ActionButton { b.busy = v; return b }

// SetDisabled dims the button and ignores clicks.
func (b *ActionButton) SetDisabled(v bool) *ActionButton {
	b.disabled = v
	b.press.Disabled(v)
	return b
}

// Compact makes the button smaller, for cards and banners.
func (b *ActionButton) Compact() *ActionButton { b.compact = true; return b }

// Grow makes the button fill the width it's given.
func (b *ActionButton) Grow() *ActionButton {
	b.grow = true
	b.press.Styles(giode.Styles{Width: -1})
	return b
}

// Busy reports whether the button shows a spinner.
func (b *ActionButton) Busy() bool { return b.busy }

func (b *ActionButton) Layout(gtx giode.Context) giode.Dimensions {
	return b.press.Layout(gtx)
}

func (b *ActionButton) content(s giode.PressState) giode.Element {
	st := giode.Styles{
		Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: 8, BorderRadius: 10,
		Padding: giode.SymmetricInset(16, 9), Cursor: giode.CursorPointer, BorderWidth: 1,
		BorderColor: "transparent",
	}
	if b.compact {
		st.Padding = giode.SymmetricInset(12, 6)
		st.BorderRadius = 8
	}
	if b.grow {
		st.Width = -1
	}
	fg := colText
	switch b.kind {
	case kindPrimary:
		fg = "#FFFFFF"
		st.BackgroundGradient = brandGradient()
		st.BoxShadow = giode.BoxShadow{Y: 4, Blur: 16, Color: "#6D4BFF55"}
		if s.Hovered {
			st.BackgroundGradient = giode.Linear(135, "#9B74FF", "#4B8EF8")
			st.BoxShadow = giode.BoxShadow{Y: 6, Blur: 22, Color: "#6D4BFF77"}
		}
		if s.Pressed {
			st.BackgroundGradient = giode.Linear(135, "#7A4EF0", "#3273E0")
			st.BoxShadow = giode.BoxShadow{Y: 2, Blur: 8, Color: "#6D4BFF55"}
		}
	case kindSecondary:
		st.Background, st.BorderColor = colRaised, colBorder
		if s.Hovered {
			st.Background, st.BorderColor = colHover, colBorderStrong
		}
		if s.Pressed {
			st.Background = colPressed
		}
	case kindGhost:
		fg = colMuted
		if s.Hovered {
			st.Background, fg = colHover, colText
		}
		if s.Pressed {
			st.Background = colPressed
		}
	case kindDanger:
		fg = colDanger
		st.Background, st.BorderColor = colDangerSoft, giode.Mix("theme.danger", "theme.surface", 0.3)
		if s.Hovered {
			st.Background, fg = colDanger, "#FFFFFF"
		}
	}
	if b.disabled {
		st.Opacity = 0.45
		st.Cursor = giode.CursorDefault
	}
	st.Color = fg
	size := 13
	if b.compact {
		size = 12
	}
	var kids []giode.Element
	switch {
	case b.busy:
		kids = append(kids, giode.Spinner(giode.Styles{Width: props.Width(size + 2), Height: props.Height(size + 2), Color: fg}))
	case b.icon != "":
		kids = append(kids, icon(b.icon, size+3, fg))
	}
	if b.label != "" {
		kids = append(kids, txt(b.label, giode.Styles{FontSize: props.FontSize(size), FontWeight: giode.FontWeightMedium, Color: fg}))
	}
	return giode.Row(st, kids...)
}
