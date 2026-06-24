package ui

import (
	"encoding/binary" // to read the small numbers in the .ico header
	"testing"         // Go's built-in testing framework
)

// TestIconIsValidICO is an automated check (a "test") that runs during
// development, not in the finished app. It confirms the icon we embed is a
// genuine, non-empty .ico file and that its internal directory doesn't point
// outside the file. If someone broke the icon generator, this test would fail
// and warn us before the change ever shipped.
func TestIconIsValidICO(t *testing.T) {
	// An .ico needs at least a 6-byte header.
	if len(Icon) < 6 {
		t.Fatalf("icon too small: %d bytes", len(Icon))
	}
	// The first two bytes are "reserved" and must be 0.
	if got := binary.LittleEndian.Uint16(Icon[0:2]); got != 0 {
		t.Errorf("reserved = %d, want 0", got)
	}
	// The next two bytes are the type, which must be 1 for an icon.
	if got := binary.LittleEndian.Uint16(Icon[2:4]); got != 1 {
		t.Errorf("type = %d, want 1 (icon)", got)
	}
	// The next two bytes are how many images are inside; need at least one.
	count := int(binary.LittleEndian.Uint16(Icon[4:6]))
	if count == 0 {
		t.Fatal("icon contains no images")
	}
	// The directory has one 16-byte entry per image; make sure they all fit.
	if len(Icon) < 6+count*16 {
		t.Fatalf("truncated directory for %d entries", count)
	}
	// For each entry, check that the image data it points to lies within the
	// file (its offset plus its size never runs past the end).
	for i := 0; i < count; i++ {
		e := Icon[6+i*16 : 6+i*16+16]
		size := binary.LittleEndian.Uint32(e[8:12])
		off := binary.LittleEndian.Uint32(e[12:16])
		if int(off+size) > len(Icon) {
			t.Errorf("entry %d points past end: off=%d size=%d len=%d", i, off, size, len(Icon))
		}
	}
}
