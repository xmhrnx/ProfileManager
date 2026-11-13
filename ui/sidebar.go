package ui

import (
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/scroll"

	"github.com/xmhrnx/profilemanager/internal/engine"
)

const sidebarWidth = 276

// Sidebar lists the apps: the user's own first, then every other
// supported app, filtered by the title bar's search field.
type Sidebar struct {
	ui      *UI
	list    *scroll.Scroll
	addApp  *ActionButton
	showAll bool
	more    *giode.PressableElement
}

func newSidebar(u *UI) *Sidebar {
	s := &Sidebar{ui: u}
	s.list = giode.Scroll(s.rows, giode.Styles{ScrollBar: giode.ScrollBar{Width: 6, TrackColor: "transparent", ThumbColor: colBorderStrong, AutoHide: true, Inset: 3}})
	s.addApp = newActionButton("Add your own app", "plus", kindSecondary, func() { u.editor.openNew() }).Grow()
	s.more = giode.Pressable(func(st giode.PressState) giode.Element {
		label, ic := "Show all supported apps", "chevron-down"
		if s.showAll {
			label, ic = "Show fewer", "arrow-up"
		}
		fg := colMuted
		if st.Hovered {
			fg = colText
		}
		return giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter, Padding: giode.SymmetricInset(12, 8), Cursor: giode.CursorPointer},
			icon(ic, 14, fg), txt(label, giode.Styles{FontSize: 12, Color: fg}))
	}).OnClick(func(giode.ClickEvent) { s.showAll = !s.showAll })
	return s
}

func (s *Sidebar) Layout(gtx giode.Context) giode.Dimensions {
	return giode.Stack(giode.Styles{Width: sidebarWidth, Height: -1, Background: colSidebar},
		giode.Box(giode.Styles{FlexGrow: 1, Width: -1}, s.list),
		giode.Box(giode.Styles{Width: -1, Padding: giode.UniformInset(12), BorderColor: colBorder}, s.addApp),
	).Layout(gtx)
}

func (s *Sidebar) rows() giode.Element {
	q := strings.ToLower(strings.TrimSpace(s.ui.search.Text()))
	var mine, other []engine.AppView
	for _, v := range s.ui.apps {
		if q != "" && !matches(v, q) {
			continue
		}
		if v.State.Pinned {
			mine = append(mine, v)
		} else {
			other = append(other, v)
		}
	}
	kids := []giode.Element{}
	if len(mine) > 0 {
		kids = append(kids, sectionLabel("Your apps", len(mine)))
		for _, v := range mine {
			kids = append(kids, s.row(v))
		}
	}
	if len(other) > 0 {
		kids = append(kids, sectionLabel("Supported apps", len(other)))
		limit := len(other)
		if !s.showAll && q == "" && len(mine) > 0 && limit > 6 {
			limit = 6
		}
		for _, v := range other[:limit] {
			kids = append(kids, s.row(v))
		}
		if q == "" && len(mine) > 0 && len(other) > 6 {
			kids = append(kids, s.more)
		}
	}
	if len(kids) == 0 {
		kids = append(kids, giode.Box(giode.Styles{Padding: giode.UniformInset(20)}, muted("No app matches “"+q+"”.", 12)))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 2, Padding: giode.Inset{Top: 10, Left: 10, Right: 10, Bottom: 10}}, kids...)
}

func matches(v engine.AppView, q string) bool {
	return strings.Contains(strings.ToLower(v.App.Name), q) ||
		strings.Contains(strings.ToLower(v.App.Category), q) ||
		strings.Contains(v.App.ID, q)
}

func sectionLabel(s string, n int) giode.Element {
	return giode.Row(giode.Styles{Padding: giode.Inset{Top: 12, Bottom: 6, Left: 10, Right: 10}, Align: giode.AlignCenter, Gap: 6},
		overline(s), spacer(), txt(itoa(n), giode.Styles{FontSize: 11, Color: colSubtle}))
}

func (s *Sidebar) row(v engine.AppView) giode.Element {
	id := v.App.ID
	return giode.Keyed("sidebar-row", id, func() *AppRow { return newAppRow(s.ui, id) })
}
