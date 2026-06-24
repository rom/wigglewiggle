package win

import (
	"testing"
	"unsafe" // lets us measure the exact byte-size of our structures
)

// TestInputLayout is an automated safety check on the memory shapes we hand to
// Windows' SendInput. Windows is very particular here: our INPUT structure
// must be exactly 40 bytes on 64-bit Windows, and the keyboard variant must
// fit inside the (larger) mouse-shaped slot we reuse for it. If a future edit
// accidentally changed these sizes, faking input could misbehave — this test
// catches that early.
func TestInputLayout(t *testing.T) {
	if got := unsafe.Sizeof(input{}); got != 40 {
		t.Errorf("sizeof(input) = %d, want 40", got)
	}
	if unsafe.Sizeof(keybdInput{}) > unsafe.Sizeof(mouseInput{}) {
		t.Errorf("keybdInput (%d) does not fit in mouseInput union (%d)",
			unsafe.Sizeof(keybdInput{}), unsafe.Sizeof(mouseInput{}))
	}
}
