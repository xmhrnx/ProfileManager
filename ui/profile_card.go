package ui

import (
	"github.com/xmhrnx/giode"
	props "github.com/xmhrnx/giode/styles/properties"

	"github.com/xmhrnx/profilemanager/internal/engine"
)

const (
	cardWidth  = 272
	cardHeight = 200
)

// ProfileCard is one saved account: who it is, when it was last saved,
// and a button to switch to it.
type ProfileCard struct {
	ui        *UI
	appID     string
	id        string
	switchBtn *ActionButton
	more      *IconButton
}

func newProfileCard(u *UI, appID, id string) *ProfileCard {
	c := &ProfileCard{ui: u, appID: appID, id: id}
	c.switchBtn = newActionButton("Switch", "arrow-left-right", kindPrimary, func() { u.switchProfile(appID, id, false) }).Compact()
	c.more = newIconButton("ellipsis", "More actions", func(e giode.ClickEvent) { u.page.openProfileMenu(appID, id, e) }).Size(28)
	return c
}

func (c *ProfileCard) Layout(gtx giode.Context) giode.Dimensions {
	v, ok := c.ui.view(c.appID)
	if !ok {
		return giode.Dimensions{}
	}
	var p engine.Profile
	found := false
	for _, x := range profilesOf(v) {
		if x.ID == c.id {
			p, found = x, true
		}
	}
	if !found {
		return giode.Dimensions{}
	}
	return c.view(v, p).Layout(gtx)
}

func (c *ProfileCard) view(v engine.AppView, p engine.Profile) giode.Element {
	active := isActive(v, p)
	color := giode.Color(p.Color)
	if color == "" {
		color = colPrimary
	}
	st := cardStyles()
	st.Width, st.Height = cardWidth, cardHeight
	st.Gap = 12
	st.FlexDirection = giode.FlexDirectionColumn
	st.Padding = giode.UniformInset(18)
	if active {
		st.BorderColor = giode.Mix(color, "theme.surface", 0.6)
		st.BorderWidth = 1
		st.BackgroundGradient = giode.Linear(160, giode.Mix(color, "theme.surface", 0.13), giode.Mix(color, "theme.surface", 0.03))
		st.BoxShadow = giode.BoxShadow{Y: 10, Blur: 30, Color: "#00000040"}
	}

	head := []giode.Element{avatar(p.Name, color, 40), spacer()}
	switch {
	case active && p.Empty:
		head = append(head, pill("Active", colWarning, colWarningSoft, "circle-pause"))
	case active:
		head = append(head, pill("Signed in", colSuccess, colSuccessSoft, "circle-check"))
	}
	if !p.Native {
		head = append(head, c.more)
	}

	who, whoColor := p.Identity, colMuted
	switch {
	case p.Native:
		who = nativeCaption(v)
	case p.Empty:
		who, whoColor = "Not signed in yet", colWarning
	case who == "" || who == p.Name:
		who = "Saved sign-in"
	}

	c.switchBtn.SetBusy(c.ui.busy[busyKey(v.App.ID, p.ID)])
	var action giode.Element = c.switchBtn
	if active {
		action = giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter, Padding: giode.SymmetricInset(4, 6)},
			icon("check", 14, colSuccess), txt("Active", giode.Styles{FontSize: 12, FontWeight: giode.FontWeightMedium, Color: colSuccess}))
	}
	return giode.Box(st,
		giode.Row(giode.Styles{Width: -1, Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, head...),
		giode.Stack(giode.Styles{Gap: 3, Width: -1},
			txt(p.Name, giode.Styles{FontSize: 15, FontWeight: giode.FontWeightSemiBold, TextOverflow: giode.TextOverflowEllipsis}),
			txt(who, giode.Styles{FontSize: 12, Color: whoColor, TextOverflow: giode.TextOverflowEllipsis}),
		),
		spacer(),
		giode.Box(giode.Styles{Width: -1, Height: 1, Background: colBorder}),
		giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 8, Wrap: giode.WrapNoWrap},
			cardFooter(v, p), spacer(), action),
	)
}

// cardFooter says how fresh the saved copy is.
func cardFooter(v engine.AppView, p engine.Profile) giode.Element {
	if p.Native {
		return txt(v.App.Name, giode.Styles{FontSize: 11, Color: colSubtle})
	}
	label := "Saved " + ago(p.Updated)
	if isActive(v, p) && !v.State.LastSync.IsZero() && v.State.Conflict == nil {
		label = "Synced " + ago(v.State.LastSync)
	}
	return giode.Row(giode.Styles{Gap: 5, Align: giode.AlignCenter},
		icon("lock", 11, colSubtle),
		txt(label, giode.Styles{FontSize: 11, Color: colSubtle}))
}

func nativeCaption(v engine.AppView) string {
	if v.App.Native != nil && len(v.App.Native.Switch) > 0 {
		return "Managed by " + v.App.Native.Switch[0]
	}
	return "Managed by " + v.App.Name
}

// avatar is a profile's initial on its color.
func avatar(name string, color giode.Color, size int) giode.Element {
	return giode.Box(giode.Styles{
		Width: props.Width(size), Height: props.Height(size), BorderRadius: props.BorderRadius(size),
		Align: giode.AlignCenter, Justify: giode.JustifyCenter,
		BackgroundGradient: giode.Linear(135, giode.Mix(color, "#FFFFFF", 0.85), giode.Mix(color, "#000000", 0.8)),
	}, txt(glyphFor("", name), giode.Styles{FontSize: props.FontSize(size * 2 / 5), FontWeight: giode.FontWeightSemiBold, Color: "#FFFFFF"}))
}
