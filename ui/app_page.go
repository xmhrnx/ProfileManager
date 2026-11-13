package ui

import (
	"context"
	"fmt"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/menu"
	"github.com/xmhrnx/giode/components/scroll"
	"github.com/xmhrnx/giode/components/segmented"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/engine"
)

// AppPage is the selected app: its header, notices, profiles and the
// details of where its sign-in lives.
type AppPage struct {
	ui     *UI
	scroll *scroll.Scroll
	header *AppHeader
	mode   *segmented.Control
	// modeFor is the app the mode control was last set for.
	modeFor string

	newCard *giode.PressableElement
	details *DetailsPanel

	saveNew, keepNew, restore, quit, retry *ActionButton

	menu *menu.Menu
}

func newAppPage(u *UI) *AppPage {
	p := &AppPage{ui: u}
	p.scroll = giode.Scroll(p.content, giode.Styles{ScrollBar: giode.ScrollBar{Width: 8, TrackColor: "transparent", ThumbColor: colBorderStrong, AutoHide: true, Inset: 3}})
	p.header = newAppHeader(u)
	p.mode = giode.Input.Segmented("Swap sign-in files", "Use its own profiles").OnChange(func(i int) {
		mode := adapter.ModeSwap
		if i == 1 {
			mode = adapter.ModeNative
		}
		id := u.selected
		u.do(func() error { return u.m.SetMode(id, mode) }, nil)
	})
	p.newCard = addProfileTile(u)
	p.details = newDetailsPanel(u)
	p.saveNew = newActionButton("Save as new profile", "plus", kindPrimary, func() {
		if v, ok := u.view(u.selected); ok {
			u.newProfile.openForConflict(v)
		}
	}).Compact()
	p.keepNew = newActionButton("Update profile", "download", kindSecondary, func() { p.resolve(engine.KeepNew) }).Compact()
	p.restore = newActionButton("Restore profile", "rotate-ccw", kindSecondary, func() { p.resolve(engine.RestoreProfile) }).Compact()
	p.quit = newActionButton("Quit app", "power", kindSecondary, func() { p.quitApp() }).Compact()
	p.retry = newActionButton("Retry", "refresh-cw", kindSecondary, func() {
		id := u.selected
		u.do(func() error { return u.m.RefreshNative(context.Background(), id) }, nil)
	}).Compact()
	p.menu = giode.Menu()
	return p
}

func (p *AppPage) Layout(gtx giode.Context) giode.Dimensions {
	return giode.Box(giode.Styles{Width: -1, Height: -1}, p.scroll, p.menu).Layout(gtx)
}

func (p *AppPage) content() giode.Element {
	v, ok := p.ui.view(p.ui.selected)
	if !ok {
		return giode.Box(giode.Styles{})
	}
	if p.modeFor != v.App.ID || (p.mode.Selected() == 1) != (v.Mode == adapter.ModeNative) {
		p.modeFor = v.App.ID
		if v.Mode == adapter.ModeNative {
			p.mode.SetSelected(1)
		} else {
			p.mode.SetSelected(0)
		}
	}
	kids := []giode.Element{p.header}
	if len(v.App.Modes()) > 1 {
		kids = append(kids, p.modeRow(v))
	}
	kids = append(kids, p.banners(v)...)
	kids = append(kids, p.profilesSection(v), p.details)
	return giode.Stack(giode.Styles{
		Width: -1, MaxWidth: 1180, Gap: 22, Padding: giode.Inset{Top: 28, Left: 32, Right: 32, Bottom: 40},
	}, kids...)
}

func (p *AppPage) modeRow(v engine.AppView) giode.Element {
	tool := v.App.Name
	if v.App.Native != nil {
		tool = v.App.Native.Switch[0]
	}
	help := "Profile Manager keeps each account's sign-in encrypted and swaps it in."
	if v.Mode == adapter.ModeNative {
		help = fmt.Sprintf("Profiles come from %s itself; switching runs its own command.", tool)
	}
	return giode.Row(giode.Styles{Width: -1, Gap: 14, Align: giode.AlignCenter},
		giode.Box(giode.Styles{Width: 380}, p.mode),
		muted(help, 12),
	)
}

func (p *AppPage) profilesSection(v engine.AppView) giode.Element {
	profiles := profilesOf(v)
	cards := make([]giode.Element, 0, len(profiles)+1)
	for _, pr := range profiles {
		appID, id := v.App.ID, pr.ID
		cards = append(cards, giode.Keyed("profile-card", busyKey(appID, id), func() *ProfileCard { return newProfileCard(p.ui, appID, id) }))
	}
	cards = append(cards, p.newCard)
	heading := giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 10},
		title("Profiles", 16),
		pill(itoa(len(profiles)), colMuted, colRaised, ""),
		spacer(),
	)
	return giode.Stack(giode.Styles{Width: -1, Gap: 14},
		heading,
		giode.Row(giode.Styles{Width: -1, Gap: 16, Wrap: giode.WrapWrap}, cards...),
	)
}

func (p *AppPage) resolve(r engine.Resolution) {
	id := p.ui.selected
	p.ui.do(func() error { return p.ui.m.Resolve(context.Background(), id, r, "") }, func(err error) {
		if err == nil {
			p.ui.toasts.Show(giode.ToastSuccess, "Done")
		}
	})
}

func (p *AppPage) quitApp() {
	id := p.ui.selected
	p.quit.SetBusy(true)
	p.ui.do(func() error { return p.ui.m.QuitApp(id) }, func(error) { p.quit.SetBusy(false) })
}

// openProfileMenu shows the actions for one profile.
func (p *AppPage) openProfileMenu(appID, profileID string, e giode.ClickEvent) {
	v, ok := p.ui.view(appID)
	if !ok {
		return
	}
	var prof engine.Profile
	for _, x := range profilesOf(v) {
		if x.ID == profileID {
			prof = x
		}
	}
	u := p.ui
	m := giode.Menu(menuStyles()).
		IconItem(menuIcon("pencil"), "Rename…", func() { u.newProfile.openRename(v, prof) })
	if !isActive(v, prof) {
		m.IconItem(menuIcon("arrow-left-right"), "Switch to this profile", func() { u.switchProfile(appID, profileID, false) })
	} else {
		m.IconItem(menuIcon("refresh-cw"), "Save the current sign-in now", func() { u.syncApp(appID) })
	}
	if v.App.SupportsEnv(u.m.GOOS()) {
		m.IconItem(menuIcon("terminal"), "Copy “open a shell” command", func() {
			u.copyText(shellCommand(v.App.ID, prof.Name), "Command")
		})
	}
	m.Separator().
		IconItem(menuIcon("arrow-up"), "Move earlier", func() { u.do(func() error { return u.m.MoveProfile(appID, profileID, -1) }, nil) }).
		IconItem(menuIcon("arrow-down"), "Move later", func() { u.do(func() error { return u.m.MoveProfile(appID, profileID, 1) }, nil) }).
		Separator().
		IconItem(menuIcon("trash-2"), "Delete profile…", func() {
			text := fmt.Sprintf("The saved sign-in for “%s” is deleted from the vault. This can't be undone.", prof.Name)
			if isActive(v, prof) {
				text += " The app stays signed in as it is now."
			}
			u.confirm.ask("Delete “"+prof.Name+"”?", text, "Delete profile", func() {
				u.do(func() error { return u.m.DeleteProfile(appID, profileID) }, nil)
			})
		})
	p.menu = m
	m.OpenAtClick(e)
}

// openAppMenu shows the actions for the app itself.
func (p *AppPage) openAppMenu(v engine.AppView, e giode.ClickEvent) {
	u := p.ui
	id := v.App.ID
	m := giode.Menu(menuStyles())
	if v.State.Pinned {
		m.IconItem(menuIcon("pin-off"), "Remove from Your apps", func() { u.do(func() error { return u.m.Pin(id, false) }, nil) })
	} else {
		m.IconItem(menuIcon("pin"), "Add to Your apps", func() { u.do(func() error { return u.m.Pin(id, true) }, nil) })
	}
	m.IconItem(menuIcon("pencil"), "Edit definition…", func() { u.editor.openEdit(v.App) }).
		IconItem(menuIcon("copy"), "Copy definition as JSON", func() {
			data, err := u.m.ExportApp(id)
			if err != nil {
				u.toasts.Show(giode.ToastError, friendly(err))
				return
			}
			u.copyText(string(data), "Definition")
		})
	if v.App.Website != "" {
		m.IconItem(menuIcon("info"), "Open documentation", func() { go u.app.OpenURL(context.Background(), v.App.Website) })
	}
	switch {
	case v.App.Overridden:
		m.Separator().IconItem(menuIcon("rotate-ccw"), "Reset to the built-in definition", func() {
			u.do(func() error { return u.m.DeleteApp(id) }, nil)
		})
	case !v.App.Builtin:
		m.Separator().IconItem(menuIcon("trash-2"), "Delete app…", func() {
			u.confirm.ask("Delete "+v.App.Name+"?", "Its definition and every saved profile are deleted. The app itself stays signed in as it is now.", "Delete app", func() {
				u.do(func() error { return u.m.DeleteApp(id) }, func(err error) {
					if err == nil {
						u.selected = ""
					}
				})
			})
		})
	}
	p.menu = m
	m.OpenAtClick(e)
}

// menuStyles is the look of popup menus.
func menuStyles() giode.Styles {
	return giode.Styles{Background: colRaised, Color: colText, BorderRadius: 12, BorderColor: colBorderStrong, BorderWidth: 1}
}

// menuIcon returns a registered icon for a menu item.
func menuIcon(name string) giode.IconSource {
	src, _ := giode.LookupIcon(name)
	return src
}

// shellCommand is the command line that opens a shell signed in to a
// profile.
func shellCommand(appID, profile string) string {
	return fmt.Sprintf("profilemanager shell %s %q", appID, profile)
}
