package ui

import (
	"fmt"
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/dialog"

	"github.com/xmhrnx/profilemanager/internal/engine"
)

// RunningDialog offers to close an app that's in the way of a switch.
type RunningDialog struct {
	ui           *UI
	shown        bool
	appID, pid   string
	err          *engine.RunningError
	dlg          *dialog.Dialog
	cancel, quit *ActionButton
}

func newRunningDialog(u *UI) *RunningDialog {
	r := &RunningDialog{ui: u}
	r.cancel = newActionButton("Not now", "", kindSecondary, func() { r.shown = false })
	r.quit = newActionButton("Quit and switch", "power", kindPrimary, func() {
		r.shown = false
		u.switchProfile(r.appID, r.pid, true)
	})
	r.dlg = giode.Dialog(&r.shown, r.content, dialogStyles(480))
	return r
}

func (r *RunningDialog) open(appID, pid string, err *engine.RunningError) {
	r.appID, r.pid, r.err = appID, pid, err
	r.quit.SetLabel("Quit " + err.App + " and switch")
	r.shown = true
}

func (r *RunningDialog) content() giode.Element {
	if r.err == nil {
		return giode.Box(giode.Styles{})
	}
	text := fmt.Sprintf("%s keeps its sign-in in files it has open, so it must close before switching to “%s”. It will be asked to quit normally, saving its state, and can be reopened right after.",
		r.err.App, r.ui.profileName(r.appID, r.pid))
	return dialogBody("circle-pause", r.err.App+" is running", "Running as: "+strings.Join(r.err.Processes, ", "),
		[]giode.Element{body(text)}, r.cancel, r.quit)
}

func (r *RunningDialog) Layout(gtx giode.Context) giode.Dimensions { return r.dlg.Layout(gtx) }
