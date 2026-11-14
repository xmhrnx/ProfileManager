package ui

import (
	"fmt"
	"strings"

	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/engine"
)

// DetailsPanel explains, for the selected app, how to run a command
// under a profile and where the sign-in lives on this computer.
type DetailsPanel struct{ ui *UI }

func newDetailsPanel(u *UI) *DetailsPanel { return &DetailsPanel{ui: u} }

func (d *DetailsPanel) Layout(gtx giode.Context) giode.Dimensions {
	v, ok := d.ui.view(d.ui.selected)
	if !ok {
		return giode.Dimensions{}
	}
	cols := []giode.Element{}
	if v.Mode == adapter.ModeSwap {
		cols = append(cols, d.runCard(v))
	}
	cols = append(cols, d.whereCard(v))
	return giode.Stack(giode.Styles{Width: -1, Gap: 16}, cols...).Layout(gtx)
}

func panelHeading(iconName, s string) giode.Element {
	return giode.Row(giode.Styles{Gap: 10, Align: giode.AlignCenter},
		giode.Box(giode.Styles{Width: 28, Height: 28, BorderRadius: 8, Background: colPrimarySoft, Align: giode.AlignCenter, Justify: giode.JustifyCenter},
			icon(iconName, 15, colPrimary)),
		title(s, 14))
}

func (d *DetailsPanel) runCard(v engine.AppView) giode.Element {
	st := cardStyles()
	st.FlexDirection, st.Gap, st.Width, st.Padding = giode.FlexDirectionColumn, 12, -1, giode.UniformInset(20)
	kids := []giode.Element{panelHeading("terminal", "Use a profile in one terminal")}
	goos := d.ui.m.GOOS()
	if !v.App.SupportsEnv(goos) {
		reason := "This app can't be pointed at another folder with environment variables."
		if len(v.App.KeychainFor(goos)) > 0 {
			reason = "On this system the app keeps its sign-in in the keychain, which can't be redirected per command."
		}
		kids = append(kids, body(reason+" Switch profiles instead; the change applies everywhere."))
		return giode.Stack(st, kids...)
	}
	kids = append(kids, muted("Runs one command, or a whole shell, signed in to a profile while everything else keeps the current sign-in. The profile is decrypted to a private folder for the run and wiped afterwards; refreshed tokens are saved back.", 12))
	profiles := profilesOf(v)
	if len(profiles) == 0 {
		kids = append(kids, muted("Create a profile first.", 12))
		return giode.Stack(st, kids...)
	}
	p := profiles[0]
	if a, ok := activeProfile(v); ok {
		for _, x := range profiles {
			if x.ID != a.ID {
				p = x
				break
			}
		}
	}
	tool := v.App.ID
	if len(v.App.Processes) > 0 {
		tool = v.App.Processes[0]
	}
	kids = append(kids,
		snippet(d.ui, shellCommand(v.App.ID, p.Name)),
		snippet(d.ui, fmt.Sprintf("profilemanager exec %s %q -- %s", v.App.ID, p.Name, tool)),
	)
	return giode.Stack(st, kids...)
}

func (d *DetailsPanel) whereCard(v engine.AppView) giode.Element {
	st := cardStyles()
	st.FlexDirection, st.Gap, st.Width, st.Padding = giode.FlexDirectionColumn, 12, -1, giode.UniformInset(20)
	kids := []giode.Element{panelHeading("folder", "Where the sign-in lives")}
	if v.Mode == adapter.ModeNative {
		n := v.App.Native
		kids = append(kids,
			muted("Profiles are read and switched with:", 12),
			mono(strings.Join(n.List, " "), 12, colText),
			mono(strings.Join(n.Switch, " "), 12, colText),
		)
	} else {
		files, keys := d.ui.m.LivePaths(&v.App)
		for _, f := range files {
			kids = append(kids, pathRow("file-json", f))
		}
		for _, k := range keys {
			kids = append(kids, pathRow("key-round", "Keychain: "+k))
		}
		if len(files) == 0 && len(keys) == 0 {
			kids = append(kids, muted("Nothing on this operating system.", 12))
		}
		var excl []string
		for _, f := range v.App.Files {
			excl = append(excl, f.Exclude...)
			if len(f.JSONKeys) > 0 {
				kids = append(kids, muted("Only these keys of "+f.ID+" are switched: "+strings.Join(f.JSONKeys, ", "), 11))
			}
		}
		if len(excl) > 0 {
			kids = append(kids, muted("Left out: "+strings.Join(excl, "  "), 11))
		}
	}
	if v.App.Notes != "" {
		kids = append(kids, giode.Box(giode.Styles{Width: -1, Height: 1, Background: colBorder}),
			giode.Row(giode.Styles{Gap: 8, Width: -1, Wrap: giode.WrapNoWrap},
				icon("info", 14, colMuted),
				giode.Box(giode.Styles{FlexGrow: 1}, muted(plain(v.App.Notes), 12))))
	}
	return giode.Stack(st, kids...)
}

func pathRow(iconName, s string) giode.Element {
	return giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter, Width: -1, Wrap: giode.WrapNoWrap},
		icon(iconName, 13, colMuted),
		giode.Box(giode.Styles{FlexGrow: 1}, txt(s, giode.Styles{FontFamily: fontMono, FontSize: 12, Color: colText, TextOverflow: giode.TextOverflowEllipsis})))
}
