package ui

import (
	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/engine"
)

// AppHeader is the top of an app's page: its tile, name, description
// and the page's main actions.
type AppHeader struct {
	ui   *UI
	add  *ActionButton
	sync *ActionButton
	more *IconButton
}

func newAppHeader(u *UI) *AppHeader {
	h := &AppHeader{ui: u}
	h.add = newActionButton("Add to Your apps", "pin", kindPrimary, func() {
		id := u.selected
		u.do(func() error { return u.m.Pin(id, true) }, nil)
	})
	h.sync = newActionButton("Sync now", "refresh-cw", kindSecondary, func() { u.syncApp(u.selected) })
	h.more = newIconButton("ellipsis", "More", func(e giode.ClickEvent) {
		if v, ok := u.view(u.selected); ok {
			u.page.openAppMenu(v, e)
		}
	}).Size(38)
	return h
}

func (h *AppHeader) Layout(gtx giode.Context) giode.Dimensions {
	v, ok := h.ui.view(h.ui.selected)
	if !ok {
		return giode.Dimensions{}
	}
	return h.view(v).Layout(gtx)
}

func (h *AppHeader) view(v engine.AppView) giode.Element {
	tags := []giode.Element{}
	if v.App.Category != "" {
		tags = append(tags, pill(v.App.Category, colMuted, colRaised, ""))
	}
	if v.Mode == adapter.ModeNative {
		tags = append(tags, pill("Native profiles", colAccent, colInfoSoft, "layers"))
	} else {
		tags = append(tags, pill("Encrypted vault", colSuccess, colSuccessSoft, "shield-check"))
	}
	if v.App.CloseBeforeSwitch {
		tags = append(tags, pill("Closes before switching", colWarning, colWarningSoft, "power"))
	}
	if !v.App.Builtin {
		tags = append(tags, pill("Custom", colPrimary, colPrimarySoft, "wand-sparkles"))
	} else if v.App.Overridden {
		tags = append(tags, pill("Customized", colPrimary, colPrimarySoft, "wand-sparkles"))
	}

	actions := []giode.Element{}
	if !v.State.Pinned {
		actions = append(actions, h.add)
	} else if v.Mode == adapter.ModeSwap {
		_, hasActive := activeProfile(v)
		h.sync.SetDisabled(!hasActive)
		actions = append(actions, h.sync)
	}
	actions = append(actions, h.more)

	desc := plain(v.App.Description)
	return giode.Row(giode.Styles{
		Width: -1, Gap: 20, Align: giode.AlignCenter, Padding: giode.UniformInset(24), Wrap: giode.WrapNoWrap,
		BorderRadius: 18, BorderWidth: 1, BorderColor: colBorder,
		BackgroundGradient: giode.Linear(120, giode.Mix(giode.Color(tileColor(v)), "theme.surface", 0.16), colSurface, colSurface),
	},
		appTile(glyphFor(v.App.Glyph, v.App.Name), v.App.Color, 64),
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: 8},
			title(v.App.Name, 22),
			giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter}, tags...),
			txt(desc, giode.Styles{FontSize: 13, Color: colMuted, LineHeight: 19, MaxWidth: 640}),
		),
		giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter}, actions...),
	)
}

func tileColor(v engine.AppView) string {
	if v.App.Color != "" {
		return v.App.Color
	}
	return "#8B6CFF"
}
