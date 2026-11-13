package ui

import "github.com/xmhrnx/giode"

// WindowControls are the minimize, maximize/restore and close buttons
// of the custom frame, drawn as thin glyphs.
type WindowControls struct {
	app            *giode.App
	min, max, quit *giode.PressableElement
}

func newWindowControls(app *giode.App) *WindowControls {
	w := &WindowControls{app: app}
	w.min = giode.Pressable(func(s giode.PressState) giode.Element { return w.button(s, glyphMinimize, false) }).
		OnClick(func(giode.ClickEvent) { app.Minimize() }).Label("Minimize")
	w.max = giode.Pressable(func(s giode.PressState) giode.Element {
		if app.Maximized() {
			return w.button(s, glyphRestore, false)
		}
		return w.button(s, glyphMaximize, false)
	}).OnClick(func(giode.ClickEvent) { app.ToggleMaximize() }).Label("Maximize")
	w.quit = giode.Pressable(func(s giode.PressState) giode.Element { return w.button(s, glyphClose, true) }).
		OnClick(func(giode.ClickEvent) { app.RequestClose() }).Label("Close")
	return w
}

func (w *WindowControls) Layout(gtx giode.Context) giode.Dimensions {
	return giode.Row(giode.Styles{Gap: 2, Align: giode.AlignCenter, Padding: giode.Inset{Right: 8}}, w.min, w.max, w.quit).Layout(gtx)
}

func (w *WindowControls) button(s giode.PressState, draw func(p *giode.Painter, c giode.Color), danger bool) giode.Element {
	fg, bg := colMuted, giode.Color("transparent")
	switch {
	case s.Hovered && danger:
		fg, bg = "#FFFFFF", "#E5484D"
	case s.Pressed:
		fg, bg = colText, colPressed
	case s.Hovered:
		fg, bg = colText, colHover
	}
	if !w.app.Focused() && !s.Hovered {
		fg = colSubtle
	}
	return giode.Box(giode.Styles{
		Width: 40, Height: 30, BorderRadius: 8, Background: bg,
		Align: giode.AlignCenter, Justify: giode.JustifyCenter,
	}, giode.Canvas(giode.Styles{Width: 12, Height: 12}, func(p *giode.Painter) { draw(p, fg) }))
}

func glyphMinimize(p *giode.Painter, c giode.Color) { p.Line(1, 6.5, 11, 6.5, 1.2, c) }

func glyphMaximize(p *giode.Painter, c giode.Color) { square(p, 1.5, 1.5, 10.5, 10.5, c) }

func glyphRestore(p *giode.Painter, c giode.Color) {
	square(p, 1.5, 3.5, 8.5, 10.5, c)
	p.Line(3.5, 3.5, 3.5, 1.5, 1.2, c)
	p.Line(3.5, 1.5, 10.5, 1.5, 1.2, c)
	p.Line(10.5, 1.5, 10.5, 8.5, 1.2, c)
	p.Line(10.5, 8.5, 8.5, 8.5, 1.2, c)
}

func square(p *giode.Painter, x1, y1, x2, y2 float32, c giode.Color) {
	pa := p.Path()
	pa.MoveTo(x1, y1)
	pa.LineTo(x2, y1)
	pa.LineTo(x2, y2)
	pa.LineTo(x1, y2)
	pa.Close()
	pa.Stroke(1.2, c)
}

func glyphClose(p *giode.Painter, c giode.Color) {
	p.Line(1.5, 1.5, 10.5, 10.5, 1.2, c)
	p.Line(10.5, 1.5, 1.5, 10.5, 1.2, c)
}
