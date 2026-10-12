// Package icon draws the app icon (a small TV) so no image files are needed.
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

// Draw renders the icon at size×size. on=false greys the screen out.
func Draw(size int, on bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	s := float64(size) / 32
	frame := color.NRGBA{0x2b, 0x2f, 0x36, 0xff}
	screen := color.NRGBA{0x3b, 0x9c, 0xff, 0xff}
	glow := color.NRGBA{0x8f, 0xd0, 0xff, 0xff}
	if !on {
		screen = color.NRGBA{0x6b, 0x72, 0x80, 0xff}
		glow = color.NRGBA{0x9a, 0xa1, 0xad, 0xff}
	}
	roundRect(img, 1*s, 4*s, 31*s, 24*s, 3*s, frame)
	roundRect(img, 3*s, 6*s, 29*s, 22*s, 1.5*s, screen)
	roundRect(img, 5*s, 8*s, 13*s, 10*s, 1*s, glow)
	roundRect(img, 14*s, 24*s, 18*s, 27*s, 0, frame)
	roundRect(img, 9*s, 27*s, 23*s, 29*s, 1*s, frame)
	return img
}

// roundRect fills a rounded rectangle with simple 4x supersampled edges.
func roundRect(img *image.NRGBA, x0, y0, x1, y1, r float64, c color.NRGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			hits := 0
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					px := float64(x) + (float64(sx)+0.5)/4
					py := float64(y) + (float64(sy)+0.5)/4
					if inside(px, py, x0, y0, x1, y1, r) {
						hits++
					}
				}
			}
			if hits == 0 {
				continue
			}
			a := float64(hits) / 16
			dst := img.NRGBAAt(x, y)
			blend := func(s, d uint8) uint8 { return uint8(float64(s)*a + float64(d)*(1-a)) }
			img.SetNRGBA(x, y, color.NRGBA{
				blend(c.R, dst.R), blend(c.G, dst.G), blend(c.B, dst.B),
				uint8(math.Min(255, float64(dst.A)+a*255)),
			})
		}
	}
}

func inside(px, py, x0, y0, x1, y1, r float64) bool {
	if px < x0 || px > x1 || py < y0 || py > y1 {
		return false
	}
	cx := math.Max(x0+r, math.Min(px, x1-r))
	cy := math.Max(y0+r, math.Min(py, y1-r))
	return (px-cx)*(px-cx)+(py-cy)*(py-cy) <= r*r+1e-9
}

func PNG(size int, on bool) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, Draw(size, on))
	return buf.Bytes()
}

// ICO packs PNG images of the given sizes into a .ico file.
func ICO(sizes ...int) []byte {
	var imgs [][]byte
	for _, s := range sizes {
		imgs = append(imgs, PNG(s, true))
	}
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, [3]uint16{0, 1, uint16(len(imgs))})
	offset := 6 + 16*len(imgs)
	for i, data := range imgs {
		dim := uint8(sizes[i])
		if sizes[i] >= 256 {
			dim = 0
		}
		binary.Write(&buf, binary.LittleEndian, struct {
			W, H, Colors, Reserved uint8
			Planes, BPP            uint16
			Size, Offset           uint32
		}{dim, dim, 0, 0, 1, 32, uint32(len(data)), uint32(offset)})
		offset += len(data)
	}
	for _, data := range imgs {
		buf.Write(data)
	}
	return buf.Bytes()
}
