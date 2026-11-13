package ui

import "github.com/xmhrnx/giode"

// CommandSnippet shows a command in a code well with a copy button.
type CommandSnippet struct {
	ui   *UI
	text string
	copy *IconButton
}

func newCommandSnippet(u *UI, text string) *CommandSnippet {
	c := &CommandSnippet{ui: u, text: text}
	c.copy = newIconButton("copy", "Copy", func(giode.ClickEvent) { u.copyText(c.text, "Command") }).Size(28)
	return c
}

func (c *CommandSnippet) Layout(gtx giode.Context) giode.Dimensions {
	return giode.Row(giode.Styles{
		Width: -1, Gap: 8, Align: giode.AlignCenter, Padding: giode.Inset{Left: 12, Right: 4, Top: 4, Bottom: 4},
		Background: colBackground, BorderRadius: 9, BorderWidth: 1, BorderColor: colBorder, Wrap: giode.WrapNoWrap,
	},
		txt("$", giode.Styles{FontFamily: fontMono, FontSize: 12, Color: colPrimary}),
		giode.Box(giode.Styles{FlexGrow: 1}, txt(c.text, giode.Styles{FontFamily: fontMono, FontSize: 12, Color: colText, TextOverflow: giode.TextOverflowEllipsis})),
		c.copy,
	).Layout(gtx)
}

// snippet returns the snippet for text, kept across frames.
func snippet(u *UI, text string) giode.Element {
	return giode.Keyed("snippet", text, func() *CommandSnippet { return newCommandSnippet(u, text) })
}
