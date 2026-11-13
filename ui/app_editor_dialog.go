package ui

import (
	"fmt"
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/dialog"
	"github.com/xmhrnx/giode/components/input"
	"github.com/xmhrnx/giode/components/scroll"
	"github.com/xmhrnx/giode/components/segmented"
	"github.com/xmhrnx/giode/components/toggle"
	props "github.com/xmhrnx/giode/styles/properties"

	"github.com/xmhrnx/profilemanager/internal/adapter"
)

// AppEditorDialog adds a user's own app, or customizes one: as a form
// for the common case, or as JSON for everything (per-OS paths,
// keychain attributes, identity, native output parsing).
type AppEditorDialog struct {
	ui    *UI
	shown bool
	base  adapter.App
	isNew bool
	err   string

	dlg    *dialog.Dialog
	tab    *segmented.Control
	scroll *scroll.Scroll

	name, id, category, color, glyph, desc, hint *input.Input
	files, keychain, env, processes              *input.Input
	nList, nCurrent, nSwitch, nCreate            *input.Input
	closeFirst                                   *toggle.Switch
	json                                         *input.Input

	cancel, save *ActionButton
}

const exampleJSON = `{
  "id": "my-tool",
  "name": "My Tool",
  "files": [
    {"id": "auth", "paths": {"default": ["{home}/.my-tool/auth.json"]}}
  ]
}`

func newAppEditorDialog(u *UI) *AppEditorDialog {
	d := &AppEditorDialog{ui: u}
	text := func(ph string) *input.Input { return giode.Input.Text(ph, inputStyles()) }
	monoText := func(ph string) *input.Input { return giode.Input.Text(ph, monoInputStyles()) }
	area := func(ph string, lines int) *input.Input {
		return giode.Input.Text(ph, monoInputStyles()).Lines(lines, 8)
	}

	d.name = text("e.g. My Tool")
	d.id = monoText("my-tool (from the name if empty)")
	d.category = text("e.g. Developer tools")
	d.color = monoText("#7C5CFF")
	d.glyph = text("e.g. MT")
	d.desc = text("e.g. Swaps the session token")
	d.hint = text("e.g. Run my-tool login in a terminal.")
	d.files = area("~/.my-tool/credentials\n{xdg_config}/my-tool/config.json keys=user,token env=config.json", 3)
	d.keychain = area("service | account", 1)
	d.env = area("MY_TOOL_HOME={profile_dir}", 1)
	d.processes = text("e.g. my-tool, My Tool")
	d.closeFirst = giode.Input.Switch("Must be closed before switching (apps holding files open)", giode.Styles{FontSize: 13, Color: colText})
	d.nList = monoText("tool profiles list")
	d.nCurrent = monoText("tool profiles current")
	d.nSwitch = monoText("tool profiles use {profile}")
	d.nCreate = monoText("tool profiles create {profile}")
	d.json = giode.Input.Text(exampleJSON, monoInputStyles()).Lines(18, 18)
	d.tab = giode.Input.Segmented("Form", "JSON").Styles(giode.Styles{Width: 200}).OnChange(d.switchTab)
	d.scroll = giode.Scroll(d.fields, giode.Styles{ScrollBar: giode.ScrollBar{Width: 6, TrackColor: "transparent", ThumbColor: colBorderStrong, AutoHide: true}}).FitContent(true)
	d.cancel = newActionButton("Cancel", "", kindSecondary, func() { d.shown = false })
	d.save = newActionButton("Save app", "check", kindPrimary, d.submit)
	d.dlg = giode.Dialog(&d.shown, d.content, dialogStyles(720)).Dismissible(false)
	return d
}

func (d *AppEditorDialog) Layout(gtx giode.Context) giode.Dimensions { return d.dlg.Layout(gtx) }

// openNew starts a blank definition.
func (d *AppEditorDialog) openNew() {
	d.base, d.isNew, d.err = adapter.App{}, true, ""
	d.fill(adapter.Form{})
	d.json.SetText(exampleJSON)
	d.tab.SetSelected(0)
	d.save.SetLabel("Add app")
	d.shown = true
	d.name.Focus()
}

// openEdit edits a definition; built-ins are saved as a customized copy.
func (d *AppEditorDialog) openEdit(a adapter.App) {
	d.base, d.isNew, d.err = a, false, ""
	d.fill(adapter.FormFrom(a))
	data, _ := adapter.Marshal(a)
	d.json.SetText(string(data))
	d.tab.SetSelected(0)
	if !adapter.Representable(a) {
		d.tab.SetSelected(1)
	}
	d.save.SetLabel("Save changes")
	d.shown = true
}

func (d *AppEditorDialog) fill(f adapter.Form) {
	d.name.SetText(f.Name)
	d.id.SetText(f.ID)
	d.category.SetText(f.Category)
	d.color.SetText(f.Color)
	d.glyph.SetText(f.Glyph)
	d.desc.SetText(f.Description)
	d.hint.SetText(f.LoginHint)
	d.files.SetText(f.Files)
	d.keychain.SetText(f.Keychain)
	d.env.SetText(f.Env)
	d.processes.SetText(f.Processes)
	d.closeFirst.SetChecked(f.CloseBeforeSwitch)
	d.nList.SetText(f.NativeList)
	d.nCurrent.SetText(f.NativeCurrent)
	d.nSwitch.SetText(f.NativeSwitch)
	d.nCreate.SetText(f.NativeCreate)
}

func (d *AppEditorDialog) form() adapter.Form {
	return adapter.Form{
		Name: d.name.Text(), ID: d.id.Text(), Category: d.category.Text(), Color: d.color.Text(),
		Glyph: d.glyph.Text(), Description: d.desc.Text(), LoginHint: d.hint.Text(),
		Files: d.files.Text(), Keychain: d.keychain.Text(), Env: d.env.Text(), Processes: d.processes.Text(),
		CloseBeforeSwitch: d.closeFirst.Checked(),
		NativeList:        d.nList.Text(), NativeCurrent: d.nCurrent.Text(), NativeSwitch: d.nSwitch.Text(), NativeCreate: d.nCreate.Text(),
	}
}

// switchTab carries edits across: form to JSON, or JSON to form when
// the JSON fits in the form.
func (d *AppEditorDialog) switchTab(i int) {
	d.err = ""
	if i == 1 {
		a, err := d.form().App(d.base)
		if err != nil {
			d.err = "The form has a problem, so the JSON shows the last saved version: " + err.Error()
			return
		}
		data, _ := adapter.Marshal(a)
		d.json.SetText(string(data))
		return
	}
	a, err := adapter.Parse([]byte(d.json.Text()))
	if err != nil {
		d.err = "Fix the JSON first: " + err.Error()
		d.tab.SetSelected(1)
		return
	}
	if !adapter.Representable(a) {
		d.err = "This definition uses per-system paths or other options the form can't show; keep editing it as JSON."
		d.tab.SetSelected(1)
		return
	}
	d.base = a
	d.fill(adapter.FormFrom(a))
}

func (d *AppEditorDialog) content() giode.Element {
	heading, sub := "Add your own app", "Describe where a program keeps its sign-in; Profile Manager does the rest."
	if !d.isNew {
		heading = "Edit " + d.base.Name
		sub = "Saved profiles keep working as long as item IDs stay the same."
		if d.base.Builtin || d.base.Overridden {
			sub = "Saving makes a customized copy; “Reset to the built-in definition” undoes it."
		}
	}
	var main giode.Element = giode.Box(giode.Styles{Width: -1, MaxHeight: 460}, d.scroll)
	if d.tab.Selected() == 1 {
		main = field("Definition", "The full format: per-system paths (linux, darwin, windows, default), json_keys, exclude, env_path, keychain attributes, identity, native list_regex/current_regex.", d.json)
	}
	return dialogBody("wand-sparkles", heading, sub, withError([]giode.Element{d.tab, main}, d.err), d.cancel, d.save)
}

func (d *AppEditorDialog) fields() giode.Element {
	row := func(kids ...giode.Element) giode.Element {
		return giode.Row(giode.Styles{Width: -1, Gap: 12, Wrap: giode.WrapNoWrap, Align: giode.AlignStart}, kids...)
	}
	grow := func(el giode.Element) giode.Element { return giode.Box(giode.Styles{FlexGrow: 1}, el) }
	fixed := func(w int, el giode.Element) giode.Element { return giode.Box(giode.Styles{Width: props.Width(w)}, el) }
	return giode.Stack(giode.Styles{Width: -1, Gap: 16, Padding: giode.Inset{Right: 10}},
		row(grow(field("Name", "", d.name)), fixed(200, field("ID", "", d.id))),
		row(grow(field("Category", "", d.category)), fixed(130, field("Color", "", d.color)), fixed(90, field("Letters", "", d.glyph))),
		field("Description", "", d.desc),
		sectionDivider("Swapped sign-in"),
		field("Files and folders", "One per line. Placeholders: ~, {home}, {config}, {xdg_config}, {xdg_data}, {appdata}, {localappdata}, {user}, {env:NAME}. Options: keys=a,b swaps only those keys of a JSON file · exclude=Cache/**,*.log · env=path places it in the per-command folder.", d.files),
		field("Keychain entries", "One per line: service | account.", d.keychain),
		row(grow(field("Process names", "Comma-separated, without .exe.", d.processes))),
		d.closeFirst,
		field("Environment for per-command profiles", "NAME=value per line; {profile_dir} is the temporary folder holding the profile.", d.env),
		sectionDivider("Or: the program's own profiles"),
		row(grow(field("List profiles", "Prints one name per line.", d.nList)), grow(field("Show the active one", "", d.nCurrent))),
		row(grow(field("Switch", "{profile} is the name.", d.nSwitch)), grow(field("Create (optional)", "", d.nCreate))),
		field("How to sign in", "Shown for a new, empty profile.", d.hint),
	)
}

func sectionDivider(label string) giode.Element {
	return giode.Row(giode.Styles{Width: -1, Gap: 10, Align: giode.AlignCenter, Padding: giode.Inset{Top: 6}},
		overline(label), giode.Box(giode.Styles{FlexGrow: 1, Height: 1, Background: colBorder}))
}

func (d *AppEditorDialog) submit() {
	var a adapter.App
	var err error
	if d.tab.Selected() == 1 {
		a, err = adapter.Parse([]byte(d.json.Text()))
	} else {
		a, err = d.form().App(d.base)
	}
	if err != nil {
		d.err = err.Error()
		return
	}
	if d.isNew {
		if _, exists := d.ui.view(a.ID); exists {
			d.err = fmt.Sprintf("An app with the ID %q exists already; choose another ID, or edit that app.", a.ID)
			return
		}
	} else if a.ID != d.base.ID {
		d.err = "The ID can't change: saved profiles belong to it. Add a new app instead."
		return
	}
	d.save.SetBusy(true)
	u := d.ui
	u.do(func() error { return u.m.SaveApp(a) }, func(err error) {
		d.save.SetBusy(false)
		if err != nil {
			d.err = strings.TrimSpace(err.Error())
			return
		}
		d.shown = false
		u.selectApp(a.ID)
		u.toasts.Show(giode.ToastSuccess, a.Name+" saved")
	})
}
