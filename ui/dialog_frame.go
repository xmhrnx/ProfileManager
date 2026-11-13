package ui

import (
	"github.com/xmhrnx/giode"
	props "github.com/xmhrnx/giode/styles/properties"
)

// dialogStyles is the panel of every dialog.
func dialogStyles(width int) giode.Styles {
	return giode.Styles{
		Width: props.Width(width), Background: colSurface, BorderRadius: 18, BorderWidth: 1, BorderColor: colBorderStrong,
		Padding: giode.UniformInset(24), BoxShadow: giode.BoxShadow{Y: 24, Blur: 60, Color: "#00000080"},
	}
}

// dialogBody lays out a dialog: an icon and heading, the content, and
// the buttons right-aligned at the bottom.
func dialogBody(iconName, heading, sub string, content []giode.Element, buttons ...giode.Element) giode.Element {
	head := giode.Row(giode.Styles{Gap: 14, Align: giode.AlignCenter, Width: -1, Wrap: giode.WrapNoWrap},
		giode.Box(giode.Styles{Width: 40, Height: 40, BorderRadius: 12, BackgroundGradient: brandGradient(), Align: giode.AlignCenter, Justify: giode.JustifyCenter},
			icon(iconName, 19, "#FFFFFF")),
		giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
			title(heading, 17),
			txt(sub, giode.Styles{FontSize: 12, Color: colMuted, LineHeight: 17})),
	)
	kids := append([]giode.Element{head}, content...)
	kids = append(kids, giode.Row(giode.Styles{Width: -1, Gap: 10, Justify: giode.JustifyEnd, Padding: giode.Inset{Top: 6}}, buttons...))
	return giode.Stack(giode.Styles{Width: -1, Gap: 18}, kids...)
}

// field is a labeled form control.
func field(label, hint string, control giode.Element) giode.Element {
	kids := []giode.Element{txt(label, giode.Styles{FontSize: 12, FontWeight: giode.FontWeightMedium, Color: colText}), control}
	if hint != "" {
		kids = append(kids, txt(hint, giode.Styles{FontSize: 11, Color: colSubtle, LineHeight: 16}))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 7}, kids...)
}

// withError appends an error line to a dialog's content when msg is set.
func withError(kids []giode.Element, msg string) []giode.Element {
	if msg == "" {
		return kids
	}
	return append(kids, errorLine(msg))
}

// errorLine shows a form error.
func errorLine(msg string) giode.Element {
	return giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter, Padding: giode.SymmetricInset(12, 9), Background: colDangerSoft, BorderRadius: 9, Width: -1, Wrap: giode.WrapNoWrap},
		icon("triangle-alert", 14, colDanger),
		giode.Box(giode.Styles{FlexGrow: 1}, txt(msg, giode.Styles{FontSize: 12, Color: colDanger, LineHeight: 17})))
}
