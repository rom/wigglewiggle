// Package autostart manages whether wigglewiggle launches at sign-in. It
// uses the per-user Run key under HKEY_CURRENT_USER, which is writable
// without administrator rights (unlike the machine-wide HKLM equivalent).
package autostart

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey    = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName = "wigglewiggle"
)

// IsEnabled reports whether a run-at-login entry currently exists.
func IsEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(valueName)
	return err == nil
}

// Enable registers the current executable to start at sign-in.
func Enable() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	// Quote the path so a Program Files-style path with spaces still works.
	return k.SetStringValue(valueName, `"`+exe+`"`)
}

// Disable removes the run-at-login entry. It is a no-op if none exists.
func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(valueName); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}
