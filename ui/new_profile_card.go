package ui

import "github.com/xmhrnx/giode"

// newProfileCard is the dashed "add" tile at the end of the profiles.
func addProfileTile(u *UI) *giode.PressableElement {
	return giode.Pressable(func(s giode.PressState) giode.Element {
		border, fg, bg := colBorderStrong, colMuted, giode.Color("transparent")
		if s.Hovered {
			border, fg, bg = colPrimaryLine, colPrimary, colPrimarySoft
		}
		return giode.Stack(giode.Styles{
			Width: cardWidth, Height: cardHeight, BorderRadius: 14, BorderWidth: 1, BorderColor: border, Background: bg,
			Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: 10, Cursor: giode.CursorPointer,
		},
			giode.Box(giode.Styles{Width: 40, Height: 40, BorderRadius: 40, Background: colRaised, Align: giode.AlignCenter, Justify: giode.JustifyCenter},
				icon("plus", 18, fg)),
			txt("New profile", giode.Styles{FontSize: 13, FontWeight: giode.FontWeightMedium, Color: fg}),
			txt("Save this sign-in or add another", giode.Styles{FontSize: 11, Color: colSubtle}),
		)
	}).Label("New profile").OnClick(func(giode.ClickEvent) {
		if v, ok := u.view(u.selected); ok {
			u.newProfile.open(v)
		}
	})
}
