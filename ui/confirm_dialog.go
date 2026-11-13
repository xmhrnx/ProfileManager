package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/dialog"
)

// ConfirmDialog asks before something that can't be undone.
type ConfirmDialog struct {
	shown          bool
	heading, text  string
	onConfirm      func()
	dlg            *dialog.Dialog
	cancel, accept *ActionButton
}

func newConfirmDialog() *ConfirmDialog {
	c := &ConfirmDialog{}
	c.cancel = newActionButton("Cancel", "", kindSecondary, func() { c.shown = false })
	c.accept = newActionButton("Delete", "trash-2", kindDanger, func() {
		c.shown = false
		if c.onConfirm != nil {
			c.onConfirm()
		}
	})
	c.dlg = giode.Dialog(&c.shown, func() giode.Element {
		return dialogBody("triangle-alert", c.heading, "Please confirm", []giode.Element{body(c.text)}, c.cancel, c.accept)
	}, dialogStyles(460))
	return c
}

// ask shows the dialog; fn runs if the user confirms.
func (c *ConfirmDialog) ask(heading, text, confirm string, fn func()) {
	c.heading, c.text, c.onConfirm = heading, text, fn
	c.accept.SetLabel(confirm)
	c.shown = true
}

func (c *ConfirmDialog) Layout(gtx giode.Context) giode.Dimensions { return c.dlg.Layout(gtx) }
