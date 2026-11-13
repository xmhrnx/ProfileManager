package ui

import "github.com/xmhrnx/giode"

// bannerTone colors a banner.
type bannerTone int

const (
	toneInfo bannerTone = iota
	toneWarning
	toneDanger
	toneSuccess
)

// banner is a full-width notice with an icon, a message and actions.
func banner(tone bannerTone, iconName, heading, text string, actions ...giode.Element) giode.Element {
	fg, bg := colAccent, colInfoSoft
	switch tone {
	case toneWarning:
		fg, bg = colWarning, colWarningSoft
	case toneDanger:
		fg, bg = colDanger, colDangerSoft
	case toneSuccess:
		fg, bg = colSuccess, colSuccessSoft
	}
	textCol := []giode.Element{txt(heading, giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold})}
	if text != "" {
		textCol = append(textCol, txt(text, giode.Styles{FontSize: 12, Color: colMuted, LineHeight: 18}))
	}
	row := []giode.Element{
		giode.Box(giode.Styles{Width: 32, Height: 32, BorderRadius: 9, Background: bg, Align: giode.AlignCenter, Justify: giode.JustifyCenter},
			icon(iconName, 16, fg)),
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2}, textCol...),
	}
	if len(actions) > 0 {
		row = append(row, giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter}, actions...))
	}
	return giode.Row(giode.Styles{
		Width: -1, Gap: 14, Align: giode.AlignCenter, Padding: giode.UniformInset(14), Wrap: giode.WrapNoWrap,
		Background: colSurface, BorderRadius: 12, BorderWidth: 1, BorderColor: giode.Mix(fg, "theme.surface", 0.35),
	}, row...)
}
