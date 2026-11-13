package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/dialog"
	"github.com/xmhrnx/giode/components/input"
	"github.com/xmhrnx/giode/components/segmented"
	"github.com/xmhrnx/giode/components/toggle"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/engine"
)

// profileDialogMode is what the profile dialog is doing.
type profileDialogMode int

const (
	dialogCreate profileDialogMode = iota
	dialogRename
	// dialogConflict saves a sign-in made outside the app as a profile.
	dialogConflict
)

// NewProfileDialog creates, renames and recolors profiles.
type NewProfileDialog struct {
	ui    *UI
	shown bool
	mode  profileDialogMode
	app   engine.AppView
	edit  engine.Profile
	color string
	err   string

	dlg       *dialog.Dialog
	name      *input.Input
	source    *segmented.Control
	switchNow *toggle.Switch
	swatches  []*giode.PressableElement
	cancel    *ActionButton
	save      *ActionButton
}

func newNewProfileDialog(u *UI) *NewProfileDialog {
	d := &NewProfileDialog{ui: u}
	d.name = giode.Input.Text("Work, Personal, Client…", inputStyles()).Submit(func(string) { d.submit() })
	d.source = giode.Input.Segmented("Use the current sign-in", "Start signed out").Styles(giode.Styles{Width: -1})
	d.switchNow = giode.Input.Switch("Switch to it now so I can sign in", giode.Styles{FontSize: 13, Color: colText})
	d.switchNow.SetChecked(true)
	for _, c := range engine.ProfileColors {
		c := c
		d.swatches = append(d.swatches, giode.Pressable(func(s giode.PressState) giode.Element {
			ring := giode.Color("transparent")
			if d.color == c {
				ring = colText
			} else if s.Hovered {
				ring = colBorderStrong
			}
			return giode.Box(giode.Styles{Width: 30, Height: 30, BorderRadius: 30, BorderWidth: 2, BorderColor: ring, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Cursor: giode.CursorPointer},
				giode.Box(giode.Styles{Width: 20, Height: 20, BorderRadius: 20, Background: giode.Color(c)}))
		}).Label("Color "+c).OnClick(func(giode.ClickEvent) { d.color = c }))
	}
	d.cancel = newActionButton("Cancel", "", kindSecondary, func() { d.shown = false })
	d.save = newActionButton("Create profile", "check", kindPrimary, d.submit)
	d.dlg = giode.Dialog(&d.shown, d.content, dialogStyles(520))
	return d
}

func (d *NewProfileDialog) Layout(gtx giode.Context) giode.Dimensions { return d.dlg.Layout(gtx) }

func (d *NewProfileDialog) reset(v engine.AppView, mode profileDialogMode) {
	d.app, d.mode, d.err = v, mode, ""
	d.save.SetBusy(false)
	d.color = engine.ProfileColors[len(profilesOf(v))%len(engine.ProfileColors)]
	d.shown = true
	d.name.Focus()
}

// open starts creating a profile for v.
func (d *NewProfileDialog) open(v engine.AppView) {
	d.reset(v, dialogCreate)
	d.name.SetText("")
	if len(profilesOf(v)) == 0 {
		d.source.SetSelected(0)
		d.name.SetText("Work")
	} else {
		d.source.SetSelected(1)
	}
	d.save.SetLabel("Create profile")
}

// openRename edits an existing profile's name and color.
func (d *NewProfileDialog) openRename(v engine.AppView, p engine.Profile) {
	d.reset(v, dialogRename)
	d.edit = p
	d.name.SetText(p.Name)
	if p.Color != "" {
		d.color = p.Color
	}
	d.save.SetLabel("Save")
}

// openForConflict saves a stray sign-in as a new profile.
func (d *NewProfileDialog) openForConflict(v engine.AppView) {
	d.reset(v, dialogConflict)
	name := ""
	if v.State.Conflict != nil {
		name = v.State.Conflict.Identity
	}
	d.name.SetText(name)
	d.save.SetLabel("Save profile")
}

func (d *NewProfileDialog) content() giode.Element {
	v := d.app
	native := v.Mode == adapter.ModeNative
	heading, sub := "New "+v.App.Name+" profile", "A profile is one account's saved sign-in."
	switch {
	case d.mode == dialogRename:
		heading, sub = "Edit profile", v.App.Name
	case d.mode == dialogConflict:
		heading, sub = "Save this sign-in", "It becomes a new profile and the active one; the others keep their sign-ins."
	case native:
		sub = "Created with " + v.App.Native.List[0] + "'s own command."
	}
	kids := []giode.Element{field("Name", "", d.name)}
	if !native {
		kids = append(kids, field("Color", "", giode.Row(giode.Styles{Gap: 6}, elements(d.swatches)...)))
	}
	if d.mode == dialogCreate && !native {
		hint := "Saves what the app is signed in to right now as this profile."
		if d.source.Selected() == 1 {
			hint = "Creates an empty profile. Switching to it signs the app out, so you can sign in to another account; the new sign-in is saved into this profile automatically."
		}
		kids = append(kids, field("Start from", hint, d.source))
		if d.source.Selected() == 1 {
			kids = append(kids, d.switchNow)
		}
	}
	return dialogBody("user-round", heading, sub, withError(kids, d.err), d.cancel, d.save)
}

func elements[T giode.Element](xs []T) []giode.Element {
	out := make([]giode.Element, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

func (d *NewProfileDialog) submit() {
	if d.save.Busy() {
		return
	}
	name := strings.TrimSpace(d.name.Text())
	if name == "" {
		d.err = "Give the profile a name."
		return
	}
	u, appID, color, mode := d.ui, d.app.App.ID, d.color, d.mode
	src := engine.FromCurrent
	if d.mode == dialogCreate && d.source.Selected() == 1 {
		src = engine.Blank
	}
	switchNow := src == engine.Blank && d.switchNow.Checked()
	d.save.SetBusy(true)
	d.err = ""
	go func() {
		var created engine.Profile
		var err error
		switch mode {
		case dialogRename:
			err = u.m.UpdateProfile(appID, d.edit.ID, name, color)
		case dialogConflict:
			err = u.m.Resolve(context.Background(), appID, engine.SaveAsNew, name)
		default:
			created, err = u.m.AddProfile(context.Background(), appID, name, color, src)
		}
		u.app.Post(func() {
			d.save.SetBusy(false)
			if err != nil {
				d.err = friendly(err)
				return
			}
			d.shown = false
			switch {
			case mode == dialogRename:
			case switchNow:
				u.switchProfile(appID, created.ID, false)
			default:
				u.toasts.Show(giode.ToastSuccess, fmt.Sprintf("Profile “%s” saved", name))
			}
		})
	}()
}
