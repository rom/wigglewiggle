package ui

import (
	"encoding/binary"
	"testing"
)

// TestIconIsValidICO checks that the embedded asset is a well-formed,
// non-empty .ico whose directory entries point within the file.
func TestIconIsValidICO(t *testing.T) {
	if len(Icon) < 6 {
		t.Fatalf("icon too small: %d bytes", len(Icon))
	}
	if got := binary.LittleEndian.Uint16(Icon[0:2]); got != 0 {
		t.Errorf("reserved = %d, want 0", got)
	}
	if got := binary.LittleEndian.Uint16(Icon[2:4]); got != 1 {
		t.Errorf("type = %d, want 1 (icon)", got)
	}
	count := int(binary.LittleEndian.Uint16(Icon[4:6]))
	if count == 0 {
		t.Fatal("icon contains no images")
	}
	if len(Icon) < 6+count*16 {
		t.Fatalf("truncated directory for %d entries", count)
	}
	for i := 0; i < count; i++ {
		e := Icon[6+i*16 : 6+i*16+16]
		size := binary.LittleEndian.Uint32(e[8:12])
		off := binary.LittleEndian.Uint32(e[12:16])
		if int(off+size) > len(Icon) {
			t.Errorf("entry %d points past end: off=%d size=%d len=%d", i, off, size, len(Icon))
		}
	}
}
