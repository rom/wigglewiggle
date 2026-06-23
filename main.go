//go:build windows

// Command wigglewiggle is a tiny Windows tray utility that keeps the screen
// from locking. It runs entirely as a normal user — no administrator or
// SYSTEM rights required.
package main

import (
	"time"

	"fyne.io/systray"

	"github.com/rom/wigglewiggle/internal/autostart"
	"github.com/rom/wigglewiggle/internal/keepawake"
	"github.com/rom/wigglewiggle/internal/ui"
)

const defaultInterval = time.Minute

var engine *keepawake.Engine

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(ui.Icon)
	systray.SetTooltip("wigglewiggle")

	engine = keepawake.New(keepawake.OSKeepAwake, defaultInterval)
	engine.Start()

	status := systray.AddMenuItem("", "")
	status.Disable()
	refresh := func() {
		s := engine.Status()
		status.SetTitle(s)
		systray.SetTooltip("wigglewiggle — " + s)
	}
	refresh()

	systray.AddSeparator()
	setupModeMenu(refresh)
	setupIntervalMenu(refresh)

	systray.AddSeparator()
	setupPause(refresh)
	setupStartup()

	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "Exit wigglewiggle")
	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()
}

// setupModeMenu wires the radio-style mechanism submenu.
func setupModeMenu(refresh func()) {
	mode := systray.AddMenuItem("Mode", "How the screen is kept awake")
	os := mode.AddSubMenuItemCheckbox("OS keep-awake", "Hold a native Windows stay-awake assertion", true)
	input := mode.AddSubMenuItemCheckbox("Input simulation", "Simulate input when idle; also keeps chat presence active", false)

	apply := func(m keepawake.Mechanism, chosen, other *systray.MenuItem) {
		engine.SetMechanism(m)
		chosen.Check()
		other.Uncheck()
		refresh()
	}
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

// setupIntervalMenu wires the radio-style idle-interval submenu.
func setupIntervalMenu(refresh func()) {
	every := systray.AddMenuItem("Interval", "How often to act when idle (input simulation)")
	items := []*systray.MenuItem{
		every.AddSubMenuItemCheckbox("30 seconds", "", false),
		every.AddSubMenuItemCheckbox("1 minute", "", true),
		every.AddSubMenuItemCheckbox("2 minutes", "", false),
		every.AddSubMenuItemCheckbox("5 minutes", "", false),
	}
	durs := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute}

	for i := range items {
		i := i
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

func setupPause(refresh func()) {
	pause := systray.AddMenuItem("Pause", "Temporarily stop keeping the screen awake")
	go func() {
		paused := false
		for range pause.ClickedCh {
			paused = !paused
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

func setupStartup() {
	startup := systray.AddMenuItemCheckbox("Start at login", "Launch wigglewiggle when you sign in to Windows", autostart.IsEnabled())
	go func() {
		for range startup.ClickedCh {
			if startup.Checked() {
				if err := autostart.Disable(); err == nil {
					startup.Uncheck()
				}
				continue
			}
			if err := autostart.Enable(); err == nil {
				startup.Check()
			}
		}
	}()
}

func onExit() {
	if engine != nil {
		engine.Stop()
	}
}
