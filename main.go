// Profile Manager switches programs between saved accounts.
package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/profilemanager/assets"
	"github.com/xmhrnx/profilemanager/internal/cli"
	"github.com/xmhrnx/profilemanager/internal/engine"
	"github.com/xmhrnx/profilemanager/ui"
)

func main() {
	if cli.IsCommand(os.Args[1:]) {
		os.Exit(cli.Run(os.Args[1:], openManager, os.Stdout, os.Stderr))
	}
	m, err := openManager()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Profile Manager can't open its data:", err)
		os.Exit(1)
	}
	iconPNG, _ := assets.FS.ReadFile("logo.png")
	app := giode.New(giode.Config{
		Title:          "Profile Manager",
		Size:           image.Pt(1240, 800),
		MinSize:        image.Pt(900, 600),
		Resizable:      true,
		Frameless:      true,
		Center:         true,
		Background:     "theme.background",
		Icon:           iconPNG,
		CloseToTray:    m.Settings().CloseToTray,
		MinimizeToTray: m.Settings().MinimizeToTray,
		// Started at login: only the tray icon, no window to dismiss.
		StartInTray: giode.StartInTrayAtLogin,
		SingleInstanceLock: &giode.SingleInstanceLock{
			UniqueID: "com.xmhrnx.profilemanager",
		},
	})
	view, err := ui.New(app, m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Profile Manager:", err)
		os.Exit(1)
	}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		view.Start(ctx)
		close(done)
	}()
	runErr := app.Run(view.View)
	// The final sync runs as the background loop ends.
	stop()
	<-done
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
}

// openManager opens the data folder: PROFILE_MANAGER_HOME, or
// "ProfileManager" in the user's config folder.
func openManager() (*engine.Manager, error) {
	dir := os.Getenv("PROFILE_MANAGER_HOME")
	if dir == "" {
		cfg, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(cfg, "ProfileManager")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return engine.New(engine.Options{DataDir: dir, Keys: keySource(dir)})
}
