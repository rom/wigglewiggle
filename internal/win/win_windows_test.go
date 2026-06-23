package win

import (
	"testing"
	"unsafe"
)

// TestInputLayout guards the INPUT memory layout that SendInput relies on.
// On 64-bit Windows the structure is 40 bytes, and the keyboard variant must
// fit within the mouse-sized union slot it is overlaid onto.
func TestInputLayout(t *testing.T) {
	if got := unsafe.Sizeof(input{}); got != 40 {
		t.Errorf("sizeof(input) = %d, want 40", got)
	}
	if unsafe.Sizeof(keybdInput{}) > unsafe.Sizeof(mouseInput{}) {
		t.Errorf("keybdInput (%d) does not fit in mouseInput union (%d)",
			unsafe.Sizeof(keybdInput{}), unsafe.Sizeof(mouseInput{}))
	}
}
