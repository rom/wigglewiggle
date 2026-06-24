//go:build windows

// Command wigglewiggle is a tiny Windows program that keeps your screen from
// locking or going to sleep while it is running.
//
// THE BIG PICTURE (for non-programmers)
//
// When you start the program, nothing opens on screen. Instead a small icon
// appears in the "system tray" — the cluster of little icons next to the
// Windows clock, in the bottom-right corner. Clicking that icon opens a menu
// where you can choose how it works, pause it, or quit.
//
// The real "keep the screen awake" work is done by a separate helper (the
// internal/keepawake package). This file is only about the tray icon and its
// menu: it builds the menu, and when you click something it tells the helper
// what to do.
//
// The program never needs administrator rights — it only ever affects your
// own signed-in Windows session.
package main

import (
	"time"

	// systray is a third-party library that draws the tray icon and menu for
	// us, so we don't have to talk to the Windows tray system by hand.
	"fyne.io/systray"

	// These are our own packages, each handling one job:
	"github.com/rom/wigglewiggle/internal/autostart" // the "start at login" setting
	"github.com/rom/wigglewiggle/internal/keepawake" // the actual keep-awake logic
	"github.com/rom/wigglewiggle/internal/ui"        // the embedded tray icon picture
)

// defaultInterval is how long you must sit idle before "input simulation"
// mode does anything, unless you pick a different value from the menu.
// (time.Minute means one minute.)
const defaultInterval = time.Minute

// engine is the single shared keep-awake helper. The menu-click handlers
// below all talk to this one object to change behaviour or shut it down.
// (A "package-level variable" like this is simply something the whole file
// can see.)
var engine *keepawake.Engine

// main is where the program starts. systray.Run takes over for the entire
// life of the program: it calls onReady once the tray icon is ready, and
// calls onExit when the program is shutting down. It does not return until
// the user quits.
func main() {
	systray.Run(onReady, onExit)
}

// onReady builds everything the moment the tray icon becomes available: it
// sets the icon picture, starts the keep-awake engine, and assembles the
// menu one item at a time.
func onReady() {
	// Set the picture shown in the tray, and the text that appears when you
	// hover the mouse over it.
	systray.SetIcon(ui.Icon)
	systray.SetTooltip("wigglewiggle")

	// Create and start the keep-awake engine. It begins in "OS keep-awake"
	// mode with the default idle interval. Start() kicks off its background
	// work; from here on it runs on its own until we Stop() it in onExit.
	engine = keepawake.New(keepawake.OSKeepAwake, defaultInterval)
	engine.Start()

	// The first menu line is a non-clickable status label (Disable makes it
	// look greyed-out). refresh() updates that label and the hover tooltip to
	// describe what the engine is currently doing. We call it now, and again
	// after every change the user makes.
	status := systray.AddMenuItem("", "")
	status.Disable()
	refresh := func() {
		s := engine.Status()
		status.SetTitle(s)
		systray.SetTooltip("wigglewiggle — " + s)
	}
	refresh()

	// AddSeparator draws a thin dividing line to group menu items. Each
	// setup* helper below adds one group of items.
	systray.AddSeparator()
	setupModeMenu(refresh)
	setupIntervalMenu(refresh)

	systray.AddSeparator()
	setupPause(refresh)
	setupStartup()

	systray.AddSeparator()

	// The final item: Quit. Clicking it tells systray to shut down, which
	// ends the program (and triggers onExit).
	//
	// "go func() { ... }()" starts a tiny background task (Go calls it a
	// "goroutine"). The "<-quit.ClickedCh" line means "wait here until a
	// click on Quit arrives", then we quit.
	quit := systray.AddMenuItem("Quit", "Exit wigglewiggle")
	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()
}

// setupModeMenu builds the "Mode" submenu, which lets the user choose how the
// screen is kept awake. The two choices behave like radio buttons: only one
// can be ticked at a time.
func setupModeMenu(refresh func()) {
	mode := systray.AddMenuItem("Mode", "How the screen is kept awake")
	// AddSubMenuItemCheckbox adds a tickable item under "Mode". The final
	// argument is whether it starts ticked — "OS keep-awake" does, matching
	// the engine's starting mode.
	os := mode.AddSubMenuItemCheckbox("OS keep-awake", "Hold a native Windows stay-awake assertion", true)
	input := mode.AddSubMenuItemCheckbox("Input simulation", "Simulate input when idle; also keeps chat presence active", false)

	// apply switches the engine to the chosen mode, ticks the chosen item,
	// un-ticks the other, and refreshes the status line.
	apply := func(m keepawake.Mechanism, chosen, other *systray.MenuItem) {
		engine.SetMechanism(m)
		chosen.Check()
		other.Uncheck()
		refresh()
	}

	// One background task per choice, each waiting for clicks on its item.
	// "for range os.ClickedCh" runs the body once per click, forever.
	go func() {
		for range os.ClickedCh {
			apply(keepawake.OSKeepAwake, os, input)
		}
	}()
	go func() {
		for range input.ClickedCh {
			apply(keepawake.InputSimulation, input, os)
		}
	}()
}

// setupIntervalMenu builds the "Interval" submenu: how long you must be idle
// before "input simulation" mode acts. Like Mode, the choices act as radio
// buttons. The interval only matters in input-simulation mode.
func setupIntervalMenu(refresh func()) {
	every := systray.AddMenuItem("Interval", "How often to act when idle (input simulation)")

	// The four tickable choices, paired with the actual length of time each
	// represents. "1 minute" starts ticked to match the engine's default.
	items := []*systray.MenuItem{
		every.AddSubMenuItemCheckbox("30 seconds", "", false),
		every.AddSubMenuItemCheckbox("1 minute", "", true),
		every.AddSubMenuItemCheckbox("2 minutes", "", false),
		every.AddSubMenuItemCheckbox("5 minutes", "", false),
	}
	durs := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute}

	// Start one background watcher per choice. When a choice is clicked, tell
	// the engine the new interval, tick only that choice, and refresh.
	for i := range items {
		i := i // give each background task its own copy of the index
		go func() {
			for range items[i].ClickedCh {
				engine.SetInterval(durs[i])
				for j, it := range items {
					if j == i {
						it.Check()
					} else {
						it.Uncheck()
					}
				}
				refresh()
			}
		}()
	}
}

// setupPause builds the Pause/Resume item. Clicking it toggles between
// pausing all keep-awake activity and resuming it, and relabels the item to
// show what the next click will do.
func setupPause(refresh func()) {
	pause := systray.AddMenuItem("Pause", "Temporarily stop keeping the screen awake")
	go func() {
		paused := false // remembers the on/off state between clicks
		for range pause.ClickedCh {
			paused = !paused // flip true<->false
			engine.SetPaused(paused)
			if paused {
				pause.SetTitle("Resume")
			} else {
				pause.SetTitle("Pause")
			}
			refresh()
		}
	}()
}

// setupStartup builds the "Start at login" checkbox. When ticked, Windows
// launches wigglewiggle automatically each time you sign in; un-ticking
// removes that. The checkbox opens in whatever state it's already in, which
// autostart.IsEnabled() reports.
func setupStartup() {
	startup := systray.AddMenuItemCheckbox("Start at login", "Launch wigglewiggle when you sign in to Windows", autostart.IsEnabled())
	go func() {
		for range startup.ClickedCh {
			// If it's currently ticked, a click means "turn it off".
			if startup.Checked() {
				if err := autostart.Disable(); err == nil {
					startup.Uncheck()
				}
				continue
			}
			// Otherwise a click means "turn it on". Only update the tick if
			// the change actually succeeded.
			if err := autostart.Enable(); err == nil {
				startup.Check()
			}
		}
	}()
}

// onExit runs when the program is shutting down. It tells the engine to stop
// cleanly, which also cancels the Windows "stay awake" request so normal
// sleep/lock behaviour returns.
func onExit() {
	if engine != nil {
		engine.Stop()
	}
}
