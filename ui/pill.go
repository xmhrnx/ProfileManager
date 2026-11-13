package ui

import (
	"github.com/xmhrnx/giode"
	props "github.com/xmhrnx/giode/styles/properties"
)

// pill is a small rounded label: a status or a tag.
func pill(label string, fg, bg giode.Color, iconName string) giode.Element {
	kids := []giode.Element{}
	if iconName != "" {
		kids = append(kids, icon(iconName, 12, fg))
	}
	kids = append(kids, txt(label, giode.Styles{FontSize: 11, FontWeight: giode.FontWeightMedium, Color: fg}))
	return giode.Row(giode.Styles{
		Gap: 5, Align: giode.AlignCenter, Padding: giode.SymmetricInset(8, 3),
		BorderRadius: 999, Background: bg, Wrap: giode.WrapNoWrap,
	}, kids...)
}

// dot is a small status light.
func dot(color giode.Color, size int) giode.Element {
	return giode.Box(giode.Styles{Width: props.Width(size), Height: props.Height(size), BorderRadius: props.BorderRadius(size), Background: color})
}
