// Command genicon renders the wigglewiggle tray icon and writes it to
// internal/ui/icon.ico as a multi-resolution 32-bit (BGRA) .ico file.
//
// # FOR NON-PROGRAMMERS
//
// This is a small, separate helper program — not part of wigglewiggle itself.
// Its only job is to draw the app's icon (a friendly teal face) and save it as
// an .ico file, which the main program then embeds. You only run this when you
// want to change how the icon looks.
//
// Regenerate the icon with:
//
//	go run ./tools/genicon
package main

import (
	"bytes"           // an in-memory buffer we build the file up in
	"encoding/binary" // writes numbers as raw bytes in the order .ico expects
	"image"           // in-memory picture types
	"image/color"     // colours
	"math"            // distance and rounding helpers used while drawing
	"os"              // writing the finished file to disk
	"path/filepath"   // building the output path in an OS-independent way
)

// main draws the icon at several sizes and saves them together in one .ico
// file. Windows then picks whichever size it needs for a given spot.
func main() {
	sizes := []int{16, 32, 48, 64} // common icon sizes, in pixels
	imgs := make([]*image.RGBA, 0, len(sizes))
	for _, s := range sizes {
		imgs = append(imgs, draw(s)) // draw the face at each size
	}
	out := filepath.Join("internal", "ui", "icon.ico")
	if err := os.WriteFile(out, encodeICO(imgs), 0o644); err != nil {
		panic(err) // stop with an error if the file can't be written
	}
	println("wrote", out)
}

// clamp keeps a value within the range 0..1. We use it so colour and opacity
// strengths never dip below 0 or rise above 1.
func clamp(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// lerp ("linear interpolate") blends between two numbers a and b: t=0 gives a,
// t=1 gives b, and values in between mix the two. We use it to blend the white
// facial features onto the teal background for smooth edges.
func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// draw renders a friendly teal "awake" face: a filled disc with two eyes and a
// smile, anti-aliased and with a transparent background.
//
// In plain terms: it visits every pixel, works out how far "inside" the circle
// that pixel is and whether it lands on an eye or the smile, and picks a colour
// to match. Edges are softened (anti-aliased) by using partial opacity right at
// the boundaries, which avoids a jagged, blocky look.
func draw(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	c := float64(size-1) / 2                       // centre point of the square
	rOuter := float64(size)/2 - float64(size)*0.06 // radius of the face disc
	eyeOff := float64(size) * 0.18                 // how far each eye sits from centre
	eyeR := float64(size) * 0.095                  // eye radius
	smileR := float64(size) * 0.27                 // radius of the smile arc
	smileW := float64(size) * 0.05                 // thickness of the smile
	bg := color.RGBA{0x1f, 0x9e, 0x8a, 0xff}       // teal

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x), float64(y)
			// How strongly this pixel belongs to the disc (0 outside, 1 well
			// inside, partial right at the edge for a smooth outline).
			disc := clamp(rOuter - math.Hypot(fx-c, fy-c) + 0.5)
			if disc <= 0 {
				continue // outside the face entirely; leave it transparent
			}
			r, g, b := float64(bg.R), float64(bg.G), float64(bg.B)

			// Eyes: take the stronger of the two eye shapes at this pixel.
			eyeY := c - eyeOff*0.15
			eye := clamp(eyeR - math.Hypot(fx-(c-eyeOff), fy-eyeY) + 0.5)
			if e := clamp(eyeR - math.Hypot(fx-(c+eyeOff), fy-eyeY) + 0.5); e > eye {
				eye = e
			}

			// Smile: a thin arc, only in the lower half of the face.
			var smile float64
			if fy > c {
				d := math.Abs(math.Hypot(fx-c, fy-c) - smileR)
				smile = clamp(smileW - d + 0.5)
			}

			// Wherever an eye or the smile is present, blend toward white.
			ink := math.Max(eye, smile)
			if ink > 0 {
				r = lerp(r, 0xff, ink)
				g = lerp(g, 0xff, ink)
				b = lerp(b, 0xff, ink)
			}
			// Store the final colour. Opacity follows the disc strength, so the
			// outer edge fades smoothly to transparent.
			img.SetRGBA(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), uint8(disc * 255)})
		}
	}
	return img
}

// encodeICO packs one or more rendered images into a single .ico file and
// returns its bytes. An .ico begins with a small directory that lists each
// image's size and where its data sits in the file, followed by the image data
// itself. This function writes that directory, then appends each image.
func encodeICO(imgs []*image.RGBA) []byte {
	var buf bytes.Buffer
	put := func(v any) { binary.Write(&buf, binary.LittleEndian, v) } // write a number as raw bytes

	// File header: reserved (always 0), type 1 = icon, then the image count.
	put(uint16(0)) // reserved
	put(uint16(1)) // type: icon
	put(uint16(len(imgs)))

	// Encode each image's pixel data first, so we know how big each one is.
	blobs := make([][]byte, len(imgs))
	for i, im := range imgs {
		blobs[i] = bmpForICO(im)
	}

	// Directory: one 16-byte entry per image, recording its size and the byte
	// offset where its data begins.
	offset := 6 + 16*len(imgs)
	for i, im := range imgs {
		w, h := im.Bounds().Dx(), im.Bounds().Dy()
		buf.WriteByte(dim(w))
		buf.WriteByte(dim(h))
		buf.WriteByte(0) // palette size (unused for full colour)
		buf.WriteByte(0) // reserved
		put(uint16(1))   // color planes
		put(uint16(32))  // bits per pixel
		put(uint32(len(blobs[i])))
		put(uint32(offset))
		offset += len(blobs[i])
	}
	// Finally, the image data blocks, in the same order as the directory.
	for _, b := range blobs {
		buf.Write(b)
	}
	return buf.Bytes()
}

// dim encodes an icon dimension into the single byte the .ico directory uses.
// The format stores 256 as 0 (since one byte can't hold the number 256).
func dim(v int) byte {
	if v >= 256 {
		return 0
	}
	return byte(v)
}

// bmpForICO encodes a single image as the pixel-data block used inside an .ico.
// Historically this is a "BMP" layout: a short header describing the image,
// then the colour of every pixel, then a 1-bit-per-pixel transparency mask.
// A few quirks of the format are worth knowing:
//   - the stored height is doubled (it counts both the colour block and the
//     transparency mask);
//   - rows are stored bottom-to-top;
//   - colours are stored in Blue-Green-Red-Alpha order.
func bmpForICO(im *image.RGBA) []byte {
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	var buf bytes.Buffer
	put := func(v any) { binary.Write(&buf, binary.LittleEndian, v) }

	// Header (BITMAPINFOHEADER) describing the image that follows.
	put(uint32(40))   // biSize: header length
	put(int32(w))     // biWidth
	put(int32(2 * h)) // biHeight (doubled: colour block + transparency mask)
	put(uint16(1))    // biPlanes
	put(uint16(32))   // biBitCount: 32 bits per pixel (full colour + alpha)
	put(uint32(0))    // biCompression = BI_RGB (none)
	put(uint32(0))    // biSizeImage
	put(int32(0))     // biXPelsPerMeter
	put(int32(0))     // biYPelsPerMeter
	put(uint32(0))    // biClrUsed
	put(uint32(0))    // biClrImportant

	// Colour block: every pixel, bottom row first, as Blue, Green, Red, Alpha.
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := im.RGBAAt(x, y)
			buf.WriteByte(c.B)
			buf.WriteByte(c.G)
			buf.WriteByte(c.R)
			buf.WriteByte(c.A)
		}
	}

	// Transparency mask: one bit per pixel (each row padded to a multiple of 4
	// bytes). We set a bit where the pixel is mostly transparent. Modern
	// Windows mainly relies on the Alpha above, but the mask is part of the
	// format, so we include it.
	rowBytes := ((w + 31) / 32) * 4 // 1 bpp, rows padded to 32 bits
	for y := h - 1; y >= 0; y-- {
		row := make([]byte, rowBytes)
		for x := 0; x < w; x++ {
			if im.RGBAAt(x, y).A < 128 {
				row[x/8] |= 0x80 >> (uint(x) % 8)
			}
		}
		buf.Write(row)
	}
	return buf.Bytes()
}
