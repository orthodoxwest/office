// Command genicons renders the PWA icon PNGs from the same cross design as
// internal/web/static/favicon.svg. The generated files are checked in under
// internal/web/static/icons/; rerun this tool only if the design changes:
//
//	go run ./tools/genicons
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

var (
	bg    = color.RGBA{R: 0xfd, G: 0xf9, B: 0xf2, A: 0xff} // site --bg cream
	brown = color.RGBA{R: 0x6b, G: 0x3a, B: 0x1f, A: 0xff} // site accent
)

// drawIcon renders the favicon cross (defined on a 32-unit grid) centered on a
// full-bleed cream background. The design is scaled to 60% of the canvas so it
// stays inside the maskable-icon safe zone (center 80%).
func drawIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	scale := float64(size) * 0.6 / 32.0
	offset := float64(size) * 0.2
	rect := func(x, y, w, h float64) image.Rectangle {
		return image.Rect(
			int(offset+x*scale+0.5), int(offset+y*scale+0.5),
			int(offset+(x+w)*scale+0.5), int(offset+(y+h)*scale+0.5),
		)
	}

	// Same geometry as favicon.svg: vertical bar then horizontal bar.
	draw.Draw(img, rect(13, 3, 6, 26), &image.Uniform{C: brown}, image.Point{}, draw.Src)
	draw.Draw(img, rect(5, 10, 22, 6), &image.Uniform{C: brown}, image.Point{}, draw.Src)
	return img
}

func main() {
	outDir := filepath.Join("internal", "web", "static", "icons")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "genicons: %v\n", err)
		os.Exit(1)
	}

	files := map[string]int{
		"icon-192.png":         192,
		"icon-512.png":         512,
		"apple-touch-icon.png": 180,
	}
	for name, size := range files {
		path := filepath.Join(outDir, name)
		f, err := os.Create(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "genicons: %v\n", err)
			os.Exit(1)
		}
		if err := png.Encode(f, drawIcon(size)); err != nil {
			fmt.Fprintf(os.Stderr, "genicons: encoding %s: %v\n", name, err)
			os.Exit(1)
		}
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "genicons: closing %s: %v\n", name, err)
			os.Exit(1)
		}
		fmt.Println("wrote", path)
	}
}
