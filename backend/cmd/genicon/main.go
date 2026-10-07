// genicon generates the app icon (cmd/soundboard/icon.ico): a dark square with 2×2 colorful tiles.
// Usage (in the backend folder): go run ./cmd/genicon
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

func pixel(x, y, size int) color.NRGBA {
	// normalized coordinates 0..1
	fx, fy := (float64(x)+0.5)/float64(size), (float64(y)+0.5)/float64(size)
	// rounded square
	const r = 0.2
	dx, dy := max(0, max(r-fx, fx-(1-r))), max(0, max(r-fy, fy-(1-r)))
	if dx*dx+dy*dy > r*r && (fx < r || fx > 1-r) && (fy < r || fy > 1-r) {
		return color.NRGBA{}
	}
	bg := color.NRGBA{0x1e, 0x22, 0x2b, 0xff}
	tiles := []color.NRGBA{{0xff, 0x6b, 0x6b, 0xff}, {0x4d, 0xab, 0xf7, 0xff}, {0x51, 0xcf, 0x66, 0xff}, {0xff, 0xd4, 0x3b, 0xff}}
	const m, g = 0.14, 0.08 // margin, gap
	w := (1 - 2*m - g) / 2
	for i, c := range tiles {
		x0 := m + float64(i%2)*(w+g)
		y0 := m + float64(i/2)*(w+g)
		if fx >= x0 && fx < x0+w && fy >= y0 && fy < y0+w {
			return c
		}
	}
	return bg
}

func dib(size int) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, struct {
		Size                  uint32
		W, H                  int32
		Planes, Bits          uint16
		Comp, ImgSize         uint32
		XPPM, YPPM            int32
		ClrUsed, ClrImportant uint32
	}{40, int32(size), int32(size * 2), 1, 32, 0, 0, 0, 0, 0, 0})
	for y := size - 1; y >= 0; y-- { // bottom-up
		for x := 0; x < size; x++ {
			c := pixel(x, y, size)
			b.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	b.Write(make([]byte, ((size+31)/32*4)*size)) // AND mask (unused, alpha applies)
	return b.Bytes()
}

func main() {
	writePNGs()
	sizes := []int{16, 32, 48}
	var imgs [][]byte
	for _, s := range sizes {
		imgs = append(imgs, dib(s))
	}
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	off := 6 + 16*len(sizes)
	for i, s := range sizes {
		out.Write([]byte{byte(s), byte(s), 0, 0})
		binary.Write(&out, binary.LittleEndian, [2]uint16{1, 32})
		binary.Write(&out, binary.LittleEndian, [2]uint32{uint32(len(imgs[i])), uint32(off)})
		off += len(imgs[i])
	}
	for _, im := range imgs {
		out.Write(im)
	}
	if err := os.WriteFile("cmd/soundboard/icon.ico", out.Bytes(), 0o644); err != nil {
		panic(err)
	}
}

// writePNGs generates the PNG variants for go-winres (exe icon).
func writePNGs() {
	for _, s := range []int{16, 32, 48, 256} {
		img := image.NewNRGBA(image.Rect(0, 0, s, s))
		for y := 0; y < s; y++ {
			for x := 0; x < s; x++ {
				img.SetNRGBA(x, y, pixel(x, y, s))
			}
		}
		f, err := os.Create(fmt.Sprintf("cmd/soundboard/winres/icon%d.png", s))
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, img); err != nil {
			panic(err)
		}
		f.Close()
	}
}
