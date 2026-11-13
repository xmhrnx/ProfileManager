package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/scroll"
)

// WelcomePage greets a user with no app selected: what Profile Manager
// does, and the supported apps to start with.
type WelcomePage struct {
	ui     *UI
	scroll *scroll.Scroll
	addApp *ActionButton
}

func newWelcomePage(u *UI) *WelcomePage {
	w := &WelcomePage{ui: u}
	w.scroll = giode.Scroll(w.content, giode.Styles{ScrollBar: giode.ScrollBar{Width: 8, TrackColor: "transparent", ThumbColor: colBorderStrong, AutoHide: true}})
	w.addApp = newActionButton("Add your own app", "plus", kindSecondary, func() { u.editor.openNew() })
	return w
}

func (w *WelcomePage) Layout(gtx giode.Context) giode.Dimensions { return w.scroll.Layout(gtx) }

func (w *WelcomePage) content() giode.Element {
	tiles := []giode.Element{}
	for _, v := range w.ui.apps {
		id := v.App.ID
		tiles = append(tiles, giode.Keyed("welcome-tile", id, func() *giode.PressableElement { return w.tile(id) }))
	}
	return giode.Stack(giode.Styles{Width: -1, MaxWidth: 1100, Gap: 28, Padding: giode.Inset{Top: 32, Left: 32, Right: 32, Bottom: 40}},
		w.hero(),
		giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter},
			giode.Stack(giode.Styles{Gap: 4, FlexGrow: 1},
				title("Pick an app to start", 16),
				muted("Each saved profile is one account. Switch with one click, from here or the tray.", 12)),
			spacer(),
			w.addApp),
		giode.Row(giode.Styles{Width: -1, Gap: 12, Wrap: giode.WrapWrap}, tiles...),
	)
}

func (w *WelcomePage) hero() giode.Element {
	feature := func(iconName, head, text string) giode.Element {
		return giode.Row(giode.Styles{Gap: 12, Align: giode.AlignStart, FlexGrow: 1, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 34, Height: 34, BorderRadius: 10, Background: "#FFFFFF1F", Align: giode.AlignCenter, Justify: giode.JustifyCenter},
				icon(iconName, 17, "#FFFFFF")),
			giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
				txt(head, giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold, Color: "#FFFFFF"}),
				txt(text, giode.Styles{FontSize: 12, Color: "#FFFFFFB8", LineHeight: 17})))
	}
	return giode.Stack(giode.Styles{
		Width: -1, Gap: 26, Padding: giode.UniformInset(36), BorderRadius: 22,
		BackgroundGradient: giode.Linear(125, "#5B3FD9", "#3D2FA8", "#1F4FB8"),
		BoxShadow:          giode.BoxShadow{Y: 18, Blur: 50, Color: "#3B2BA855"},
	},
		giode.Row(giode.Styles{Gap: 18, Align: giode.AlignCenter},
			giode.Image(w.ui.logo, giode.Styles{Width: 56, Height: 56}),
			giode.Stack(giode.Styles{Gap: 6},
				txt("Every account, one click away", giode.Styles{FontSize: 26, FontWeight: giode.FontWeightBold, Color: "#FFFFFF"}),
				txt("Keep work and personal sign-ins for your CLIs and apps side by side, and switch without signing out.", giode.Styles{FontSize: 14, Color: "#FFFFFFCC"}))),
		giode.Row(giode.Styles{Width: -1, Gap: 24, Wrap: giode.WrapNoWrap},
			feature("shield-check", "Encrypted at rest", "AES-256-GCM, with the key in your OS keychain."),
			feature("refresh-cw", "Always up to date", "Refreshed tokens are saved back automatically."),
			feature("wand-sparkles", "Any software", "Built-in definitions, or describe your own."),
		),
	)
}

func (w *WelcomePage) tile(id string) *giode.PressableElement {
	return giode.Pressable(func(s giode.PressState) giode.Element {
		v, ok := w.ui.view(id)
		if !ok {
			return giode.Box(giode.Styles{})
		}
		st := giode.Styles{
			Width: 200, Gap: 12, Align: giode.AlignCenter, Padding: giode.UniformInset(14), Wrap: giode.WrapNoWrap,
			Background: colSurface, BorderRadius: 14, BorderWidth: 1, BorderColor: colBorder, Cursor: giode.CursorPointer,
		}
		if s.Hovered {
			st.BorderColor, st.Background = colPrimaryLine, colRaised
		}
		return giode.Row(st,
			appTile(glyphFor(v.App.Glyph, v.App.Name), v.App.Color, 36),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2},
				txt(v.App.Name, giode.Styles{FontSize: 13, FontWeight: giode.FontWeightMedium, TextOverflow: giode.TextOverflowEllipsis}),
				txt(v.App.Category, giode.Styles{FontSize: 11, Color: colSubtle})))
	}).OnClick(func(giode.ClickEvent) { w.ui.selectApp(id) })
}
