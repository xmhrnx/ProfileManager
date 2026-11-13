package ui

import (
	"strings"

	"github.com/xmhrnx/giode"
	props "github.com/xmhrnx/giode/styles/properties"
)

// appTile is an app's square badge: its letters on a gradient of its
// brand color.
func appTile(glyph, color string, size int) giode.Element {
	if color == "" {
		color = "#8B6CFF"
	}
	if glyph == "" {
		glyph = "?"
	}
	c := giode.Color(color)
	return giode.Box(giode.Styles{
		Width: props.Width(size), Height: props.Height(size), BorderRadius: props.BorderRadius(size * 3 / 10),
		Align: giode.AlignCenter, Justify: giode.JustifyCenter,
		BackgroundGradient: giode.Linear(145, giode.Mix(c, "#FFFFFF", 0.82), giode.Mix(c, "#000000", 0.78)),
		BoxShadow:          giode.BoxShadow{Y: size / 12, Blur: size / 4, Color: "#00000040"},
	}, txt(glyph, giode.Styles{
		FontSize: props.FontSize(size * 38 / 100), FontWeight: giode.FontWeightBold, Color: "#FFFFFF",
	}))
}

// glyphFor returns a tile's letters: the definition's own, or the
// initials of its name.
func glyphFor(glyph, name string) string {
	if glyph != "" {
		return glyph
	}
	var out []rune
	for _, w := range strings.Fields(name) {
		for _, r := range w {
			out = append(out, r)
			break
		}
		if len(out) == 2 {
			break
		}
	}
	return strings.ToUpper(string(out))
}
