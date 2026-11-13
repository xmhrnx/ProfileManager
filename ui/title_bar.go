package ui

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/op/clip"
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/kit"
	props "github.com/xmhrnx/giode/styles/properties"
)

const titleBarHeight = 48

// TitleBar is the custom window frame's top edge: the logo and name, a
// search field, global actions and the window controls. Empty space
// drags the window; a double click maximizes it.
type TitleBar struct {
	ui       *UI
	dbl      gesture.Click
	logo     giode.Element
	sync     *IconButton
	theme    *IconButton
	settings *IconButton
	controls *WindowControls
}

func newTitleBar(u *UI) *TitleBar {
	t := &TitleBar{ui: u, controls: newWindowControls(u.app)}
	t.logo = giode.Image(u.logo, giode.Styles{Width: 22, Height: 22})
	t.sync = newIconButton("refresh-cw", "Sync all profiles now (F5)", func(giode.ClickEvent) { u.syncAll() })
	t.theme = newIconButton("sun", "Light or dark appearance", func(giode.ClickEvent) { u.toggleTheme() })
	t.settings = newIconButton("settings", "Settings (Ctrl+,)", func(giode.ClickEvent) { u.settings.open() })
	return t
}

func (t *TitleBar) Layout(gtx giode.Context) giode.Dimensions {
	for {
		ev, ok := t.dbl.Update(gtx.Source)
		if !ok {
			break
		}
		if ev.Kind == gesture.KindClick && ev.NumClicks == 2 {
			t.ui.app.ToggleMaximize()
		}
	}
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(titleBarHeight))
	// The double-click listener and the drag handle go first; the
	// controls laid out after them carve themselves out.
	cl := clip.Rect{Max: size}.Push(gtx.Ops)
	t.dbl.Add(gtx.Ops)
	cl.Pop()
	kit.DragArea(gtx, size)

	if giode.CurrentMode() == giode.ModeDark {
		t.theme.SetIcon("sun")
	} else {
		t.theme.SetIcon("moon")
	}
	t.sync.SetActive(t.ui.syncing)
	dim := 1.0
	if !t.ui.app.Focused() {
		dim = 0.6
	}
	gtx.Constraints.Min, gtx.Constraints.Max = size, size
	return giode.Row(giode.Styles{
		Width: -1, Height: titleBarHeight, Align: giode.AlignCenter, Gap: 6, Wrap: giode.WrapNoWrap,
		Padding: giode.Inset{Left: 14}, Background: colSidebar, BorderColor: colBorder,
	},
		giode.Row(giode.Styles{Gap: 10, Align: giode.AlignCenter, Width: 252, Opacity: props.Opacity(dim)},
			t.logo,
			txt("Profile Manager", giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold}),
		),
		giode.Box(giode.Styles{Width: 340}, t.ui.search),
		spacer(),
		t.sync, t.theme, t.settings,
		giode.Box(giode.Styles{Width: 1, Height: 20, Background: colBorder, Margin: giode.SymmetricInset(6, 0)}),
		t.controls,
	).Layout(gtx)
}
