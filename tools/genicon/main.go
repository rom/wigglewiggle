// Command genicon renders the wigglewiggle tray icon and writes it to
// internal/ui/icon.ico as a multi-resolution 32-bit (BGRA) .ico file.
//
// It is a developer tool, not part of the application. Regenerate with:
//
//	go run ./tools/genicon
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
)

func main() {
	sizes := []int{16, 32, 48, 64}
	imgs := make([]*image.RGBA, 0, len(sizes))
	for _, s := range sizes {
		imgs = append(imgs, draw(s))
	}
	out := filepath.Join("internal", "ui", "icon.ico")
	if err := os.WriteFile(out, encodeICO(imgs), 0o644); err != nil {
		panic(err)
	}
	println("wrote", out)
}

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

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// draw renders a friendly teal "awake" face: a filled disc with two eyes
// and a smile, anti-aliased and with a transparent background.
func draw(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	c := float64(size-1) / 2
	rOuter := float64(size)/2 - float64(size)*0.06
	eyeOff := float64(size) * 0.18
	eyeR := float64(size) * 0.095
	smileR := float64(size) * 0.27
	smileW := float64(size) * 0.05
	bg := color.RGBA{0x1f, 0x9e, 0x8a, 0xff} // teal

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x), float64(y)
			disc := clamp(rOuter - math.Hypot(fx-c, fy-c) + 0.5)
			if disc <= 0 {
				continue
			}
			r, g, b := float64(bg.R), float64(bg.G), float64(bg.B)

			eyeY := c - eyeOff*0.15
			eye := clamp(eyeR - math.Hypot(fx-(c-eyeOff), fy-eyeY) + 0.5)
			if e := clamp(eyeR - math.Hypot(fx-(c+eyeOff), fy-eyeY) + 0.5); e > eye {
				eye = e
			}

			var smile float64
			if fy > c {
				d := math.Abs(math.Hypot(fx-c, fy-c) - smileR)
				smile = clamp(smileW - d + 0.5)
			}

			ink := math.Max(eye, smile)
			if ink > 0 {
				r = lerp(r, 0xff, ink)
				g = lerp(g, 0xff, ink)
				b = lerp(b, 0xff, ink)
			}
			img.SetRGBA(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), uint8(disc * 255)})
		}
	}
	return img
}

func encodeICO(imgs []*image.RGBA) []byte {
	var buf bytes.Buffer
	put := func(v any) { binary.Write(&buf, binary.LittleEndian, v) }

	put(uint16(0)) // reserved
	put(uint16(1)) // type: icon
	put(uint16(len(imgs)))

	blobs := make([][]byte, len(imgs))
	for i, im := range imgs {
		blobs[i] = bmpForICO(im)
	}

	offset := 6 + 16*len(imgs)
	for i, im := range imgs {
		w, h := im.Bounds().Dx(), im.Bounds().Dy()
		buf.WriteByte(dim(w))
		buf.WriteByte(dim(h))
		buf.WriteByte(0) // palette size
		buf.WriteByte(0) // reserved
		put(uint16(1))   // color planes
		put(uint16(32))  // bits per pixel
		put(uint32(len(blobs[i])))
		put(uint32(offset))
		offset += len(blobs[i])
	}
	for _, b := range blobs {
		buf.Write(b)
	}
	return buf.Bytes()
}

// dim encodes an icon dimension; 256 is stored as 0.
func dim(v int) byte {
	if v >= 256 {
		return 0
	}
	return byte(v)
}

// bmpForICO encodes a single image as the BMP (DIB) payload used inside an
// .ico: a BITMAPINFOHEADER with doubled height, a bottom-up 32-bit BGRA
// color block, and a 1-bpp AND mask derived from the alpha channel.
func bmpForICO(im *image.RGBA) []byte {
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	var buf bytes.Buffer
	put := func(v any) { binary.Write(&buf, binary.LittleEndian, v) }

	put(uint32(40))   // biSize
	put(int32(w))     // biWidth
	put(int32(2 * h)) // biHeight (XOR + AND)
	put(uint16(1))    // biPlanes
	put(uint16(32))   // biBitCount
	put(uint32(0))    // biCompression = BI_RGB
	put(uint32(0))    // biSizeImage
	put(int32(0))     // biXPelsPerMeter
	put(int32(0))     // biYPelsPerMeter
	put(uint32(0))    // biClrUsed
	put(uint32(0))    // biClrImportant

	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := im.RGBAAt(x, y)
			buf.WriteByte(c.B)
			buf.WriteByte(c.G)
			buf.WriteByte(c.R)
			buf.WriteByte(c.A)
		}
	}

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
