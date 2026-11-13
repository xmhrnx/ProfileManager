package ui

import (
	"strconv"

	"github.com/xmhrnx/profilemanager/internal/adapter"
	"github.com/xmhrnx/profilemanager/internal/engine"
)

// profilesOf returns the profiles of the app's current mode: its own
// in native mode, saved ones otherwise.
func profilesOf(v engine.AppView) []engine.Profile {
	var out []engine.Profile
	for _, p := range v.State.Profiles {
		if p.Native == (v.Mode == adapter.ModeNative) {
			out = append(out, p)
		}
	}
	return out
}

// isActive reports whether p is signed in now.
func isActive(v engine.AppView, p engine.Profile) bool {
	if v.Mode == adapter.ModeNative {
		return p.ID == v.State.NativeActive
	}
	return p.ID == v.State.Active
}

// activeProfile returns the signed-in profile.
func activeProfile(v engine.AppView) (engine.Profile, bool) {
	for _, p := range profilesOf(v) {
		if isActive(v, p) {
			return p, true
		}
	}
	return engine.Profile{}, false
}

func itoa(n int) string { return strconv.Itoa(n) }
