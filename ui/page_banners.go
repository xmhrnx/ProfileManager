package ui

import (
	"fmt"
	"strings"

	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/engine"
)

// banners are the notices that need the user: a sign-in held back, an
// app that must close, errors, and what to do with a fresh profile.
func (p *AppPage) banners(v engine.AppView) []giode.Element {
	var out []giode.Element
	active, hasActive := activeProfile(v)
	name := active.Name
	if c := v.State.Conflict; c != nil && hasActive {
		p.keepNew.SetLabel("Update “" + name + "”")
		p.restore.SetLabel("Restore “" + name + "”")
		if c.Kind == engine.ConflictSignedOut {
			out = append(out, banner(toneWarning, "log-out",
				v.App.Name+" was signed out outside Profile Manager",
				fmt.Sprintf("“%s” still holds its sign-in. Put it back, or keep the app signed out and clear the profile.", name),
				p.restore, p.keepNew.SetLabel("Clear “"+name+"”")))
		} else {
			out = append(out, banner(toneWarning, "user-round",
				fmt.Sprintf("Now signed in as %s, not %s", c.Identity, active.Identity),
				fmt.Sprintf("This sign-in wasn't saved into “%s”, so that account stays safe. Choose what to keep.", name),
				p.saveNew, p.keepNew, p.restore))
		}
	}
	if len(v.Running) > 0 && v.App.CloseBeforeSwitch {
		out = append(out, banner(toneInfo, "circle-pause",
			v.App.Name+" is running",
			"Its files are in use, so changes are saved after it closes. Switching asks it to quit first.",
			p.quit))
	}
	if v.State.SyncError != "" {
		out = append(out, banner(toneDanger, "triangle-alert", "Couldn't read the current sign-in", v.State.SyncError))
	}
	if v.Mode == adapter.ModeNative && v.State.NativeError != "" {
		out = append(out, banner(toneDanger, "terminal", "The "+v.App.Native.List[0]+" command failed", shorten(v.State.NativeError), p.retry))
	}
	if hasActive && active.Empty && v.Mode == adapter.ModeSwap {
		hint := v.App.LoginHint
		if hint == "" {
			hint = "Sign in with " + v.App.Name + " as usual."
		}
		out = append(out, banner(toneSuccess, "sparkles", "Sign in to “"+name+"” now",
			plain(hint)+" Profile Manager saves the new sign-in into this profile automatically."))
	}
	if v.Mode == adapter.ModeNative && len(profilesOf(v)) == 0 && v.State.NativeError == "" && v.App.LoginHint != "" {
		out = append(out, banner(toneInfo, "info", "No profiles yet", plain(v.App.LoginHint)))
	}
	return out
}

func shorten(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 240 {
		return s[:240] + "…"
	}
	return s
}
