// Package ui holds embedded UI assets for wigglewiggle.
package ui

import _ "embed"

// Icon is the system-tray icon in Windows .ico format.
// Regenerate it with: go run ./tools/genicon
//
//go:embed icon.ico
var Icon []byte
