// Command genplaster derives the limewash texture behind the threshold pages
// (home, ordo, reminders) from a photograph of the parish's nave plaster. The
// generated file is checked in as internal/web/static/plaster.jpg; rerun this
// tool only to change the source photograph or the processing:
//
//	go run ./tools/genplaster -src ../resources/design/parish/nave-wall-plaster.jpg
//
// The photograph is not used as a picture. Its brightness is divided by a
// heavily blurred copy of itself, which removes the camera's light falloff and
// leaves only the wall's own mottling and trowel marks, centred on mid-grey.
// Colour is supplied by the stylesheet's theme tokens, so one grey field
// serves Nave and Apse alike. The field is sized for a single viewport and is
// never tiled: repetition was what defeated earlier texture attempts.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
)

func main() {
	src := flag.String("src", "../resources/design/parish/nave-wall-plaster.jpg", "source photograph of the plaster")
	dst := flag.String("out", "internal/web/static/plaster.jpg", "output grey texture")
	width := flag.Int("width", 800, "output width in pixels (height follows the photograph's aspect)")
	radius := flag.Float64("radius", 0.06, "blur radius for the lighting estimate, as a fraction of the width")
	gain := flag.Float64("gain", 5, "contrast applied to the wall's deviation from its local mean")
	crop := flag.Float64("crop", 0.03, "border trimmed from each edge, as a fraction of the size")
	soften := flag.Int("soften", 1, "box radius in output pixels that removes sensor noise before contrast")
	quality := flag.Int("quality", 72, "JPEG quality")
	flag.Parse()

	if err := run(*src, *dst, *width, *radius, *gain, *crop, *soften, *quality); err != nil {
		fmt.Fprintln(os.Stderr, "genplaster:", err)
		os.Exit(1)
	}
}

func run(src, dst string, width int, radius, gain, crop float64, soften, quality int) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	photo, err := jpeg.Decode(f)
	f.Close()
	if err != nil {
		return err
	}

	b := photo.Bounds()
	cx, cy := int(float64(b.Dx())*crop), int(float64(b.Dy())*crop)
	b = image.Rect(b.Min.X+cx, b.Min.Y+cy, b.Max.X-cx, b.Max.Y-cy)
	height := int(math.Round(float64(width) * float64(b.Dy()) / float64(b.Dx())))

	lum := downscaleLuminance(photo, b, width, height)
	if soften > 0 {
		// Sensor noise is not plaster, and it is what JPEG pays most to keep.
		lum = blur(lum, width, height, soften)
	}
	mean := blur(lum, width, height, int(math.Round(radius*float64(width))))

	out := image.NewGray(image.Rect(0, 0, width, height))
	var sum, sum2 float64
	for i, l := range lum {
		detail := l / math.Max(mean[i], 1e-3)
		sum += detail
		sum2 += detail * detail
		// Darker than the local mean is more wash; mid-grey is the mean wall.
		v := 0.5 + (detail-1)*gain*0.5
		out.Pix[i] = uint8(math.Round(255 * math.Max(0, math.Min(1, v))))
	}
	n := float64(len(lum))
	mu := sum / n
	fmt.Printf("detail mean %.4f sd %.4f\n", mu, math.Sqrt(sum2/n-mu*mu))

	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(w, out, &jpeg.Options{Quality: quality}); err != nil {
		w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	st, err := os.Stat(dst)
	if err != nil {
		return err
	}
	fmt.Printf("%s %dx%d %d bytes\n", dst, width, height, st.Size())
	return nil
}

// downscaleLuminance box-averages the photograph's linear-light luminance
// into a w×h grid, so the reduction itself introduces no aliasing.
func downscaleLuminance(img image.Image, b image.Rectangle, w, h int) []float64 {
	var toLinear [256]float64
	for i := range toLinear {
		c := float64(i) / 255
		if c <= 0.04045 {
			toLinear[i] = c / 12.92
		} else {
			toLinear[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	sums := make([]float64, w*h)
	counts := make([]float64, w*h)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		oy := (y - b.Min.Y) * h / b.Dy()
		for x := b.Min.X; x < b.Max.X; x++ {
			ox := (x - b.Min.X) * w / b.Dx()
			r, g, bl, _ := color.RGBAModel.Convert(img.At(x, y)).RGBA()
			l := 0.2126*toLinear[r>>8] + 0.7152*toLinear[g>>8] + 0.0722*toLinear[bl>>8]
			sums[oy*w+ox] += l
			counts[oy*w+ox]++
		}
	}
	for i := range sums {
		sums[i] /= counts[i]
	}
	return sums
}

// blur approximates a Gaussian with three separable box passes, mirroring at
// the edges so the lighting estimate does not sag at the borders.
func blur(src []float64, w, h, r int) []float64 {
	a := append([]float64(nil), src...)
	tmp := make([]float64, len(a))
	for range 3 {
		boxPass(a, tmp, w, h, r, true)
		boxPass(tmp, a, w, h, r, false)
	}
	return a
}

func boxPass(src, dst []float64, w, h, r int, horizontal bool) {
	lines, length := h, w
	if !horizontal {
		lines, length = w, h
	}
	at := func(line, i int) int {
		if horizontal {
			return line*w + i
		}
		return i*w + line
	}
	mirror := func(i int) int {
		for i < 0 || i >= length {
			if i < 0 {
				i = -i - 1
			} else {
				i = 2*length - i - 1
			}
		}
		return i
	}
	norm := 1 / float64(2*r+1)
	for line := range lines {
		var acc float64
		for i := -r; i <= r; i++ {
			acc += src[at(line, mirror(i))]
		}
		for i := range length {
			dst[at(line, i)] = acc * norm
			acc += src[at(line, mirror(i+r+1))] - src[at(line, mirror(i-r))]
		}
	}
}
