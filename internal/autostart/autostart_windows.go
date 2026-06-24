// Package autostart controls a single Windows setting: whether wigglewiggle
// starts automatically when you sign in.
//
// # FOR NON-PROGRAMMERS
//
// Windows keeps a large settings database called the "registry". Inside it is
// a well-known list named "Run": every program listed there is launched at
// sign-in. We add or remove our own entry in the *per-user* copy of that list
// (under "HKEY_CURRENT_USER"), which affects only you and needs no
// administrator rights. (There is a computer-wide copy too, but changing that
// would require admin, so we deliberately don't.)
package autostart

import (
	"os" // to find the path of our own program file

	"golang.org/x/sys/windows/registry" // read/write the Windows registry
)

const (
	// runKey is the location of the per-user "launch at sign-in" list.
	runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	// valueName is the name of our own entry within that list.
	valueName = "wigglewiggle"
)

// IsEnabled reports whether our "start at login" entry currently exists. The
// menu uses this to show the checkbox in the correct state at startup.
func IsEnabled() bool {
	// Open the Run list for reading. If we can't even open it, treat that as
	// "not enabled".
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close() // always close the key when we're done
	// Try to read our entry. If reading succeeds (no error), it exists.
	_, _, err = k.GetStringValue(valueName)
	return err == nil
}

// Enable adds (or updates) the entry so Windows launches wigglewiggle at
// sign-in, pointing it at this program's own location on disk.
func Enable() error {
	// Find the full path to the currently running .exe.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Open the Run list for writing, creating it if it doesn't exist yet.
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	// Store the path wrapped in quotes, so a path that contains spaces (such
	// as one under "Program Files") is still read as a single location.
	return k.SetStringValue(valueName, `"`+exe+`"`)
}

// Disable removes our entry so Windows no longer launches us at sign-in. If
// the entry already doesn't exist, that's fine — we treat it as success.
func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	// Delete our entry. The only error we ignore is "it wasn't there anyway".
	if err := k.DeleteValue(valueName); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}
