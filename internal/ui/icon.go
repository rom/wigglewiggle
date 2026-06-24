// Package ui holds the program's user-interface assets. Right now that's just
// the tray icon picture.
package ui

import _ "embed" // enables the //go:embed feature used just below

// Icon is the picture shown in the system tray, stored in Windows' ".ico"
// image format.
//
// The "//go:embed icon.ico" line just below is a build-time instruction: it
// bakes the contents of the icon.ico file (which sits next to this source
// file) directly into the program. That way the finished .exe carries its own
// icon and needs no separate image file shipped alongside it.
//
// To change the icon, regenerate it with: go run ./tools/genicon
//
//go:embed icon.ico
var Icon []byte
