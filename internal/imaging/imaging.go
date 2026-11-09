// Package imaging does binning, a minimal FITS writer and preview JPEGs.
package imaging

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// Image is a mono image, row-major (Pix[y*W+x]).
type Image struct {
	W, H int
	Pix  []float32
}

// FromAlpaca converts Alpaca's [x][y] element order (y varies fastest) to row-major.
func FromAlpaca(data []float32, dimX, dimY int) Image {
	img := Image{W: dimX, H: dimY, Pix: make([]float32, dimX*dimY)}
	for x := 0; x < dimX; x++ {
		col := data[x*dimY : (x+1)*dimY]
		for y, v := range col {
			img.Pix[y*dimX+x] = v
		}
	}
	return img
}

// Bin2x2 averages 2x2 blocks. On a raw Bayer frame each block holds one R, two
// G and one B, so the solver never sees the colour pattern, and ASTAP has a
// quarter of the pixels to chew through.
func Bin2x2(img Image) Image {
	w, h := img.W/2, img.H/2
	out := Image{W: w, H: h, Pix: make([]float32, w*h)}
	for y := 0; y < h; y++ {
		r0 := img.Pix[(2*y)*img.W:]
		r1 := img.Pix[(2*y+1)*img.W:]
		for x := 0; x < w; x++ {
			out.Pix[y*w+x] = (r0[2*x] + r0[2*x+1] + r1[2*x] + r1[2*x+1]) / 4
		}
	}
	return out
}

// FITSMeta are optional header values.
type FITSMeta struct {
	Exposure        float64
	DateObs         time.Time
	PixelUM         float64
	FocalMM         float64
	RAHint, DecHint *float64
}

func card(key string, value any) string {
	var s string
	switch v := value.(type) {
	case bool:
		b := "F"
		if v {
			b = "T"
		}
		s = fmt.Sprintf("%-8s= %20s", key, b)
	case int:
		s = fmt.Sprintf("%-8s= %20d", key, v)
	case float64:
		s = fmt.Sprintf("%-8s= %20s", key, strings.ToUpper(fmt.Sprintf("%.10G", v)))
	case string:
		s = fmt.Sprintf("%-8s= '%-8s'", key, v)
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return s + strings.Repeat(" ", 80-len(s))
}

// WriteFITS writes a 16-bit unsigned (BZERO=32768) mono FITS that ASTAP reads.
func WriteFITS(path string, img Image, meta FITSMeta) error {
	cards := []string{
		card("SIMPLE", true), card("BITPIX", 16), card("NAXIS", 2),
		card("NAXIS1", img.W), card("NAXIS2", img.H),
		card("BZERO", 32768.0), card("BSCALE", 1.0),
	}
	if meta.Exposure > 0 {
		cards = append(cards, card("EXPTIME", meta.Exposure))
	}
	if !meta.DateObs.IsZero() {
		cards = append(cards, card("DATE-OBS", meta.DateObs.UTC().Format("2006-01-02T15:04:05.000")))
	}
	if meta.PixelUM > 0 {
		cards = append(cards, card("XPIXSZ", meta.PixelUM), card("YPIXSZ", meta.PixelUM))
	}
	if meta.FocalMM > 0 {
		cards = append(cards, card("FOCALLEN", meta.FocalMM))
	}
	if meta.RAHint != nil && meta.DecHint != nil {
		cards = append(cards, card("RA", *meta.RAHint), card("DEC", *meta.DecHint))
	}
	cards = append(cards, "END"+strings.Repeat(" ", 77))

	var buf bytes.Buffer
	for _, c := range cards {
		buf.WriteString(c)
	}
	pad(&buf, ' ')
	// FITS rows go bottom-up. Mirroring wouldn't change the solved centre, but
	// stay standard anyway.
	row := make([]byte, 2*img.W)
	for y := img.H - 1; y >= 0; y-- {
		for x := 0; x < img.W; x++ {
			v := math.Round(float64(img.Pix[y*img.W+x]))
			v = math.Max(0, math.Min(65535, v))
			binary.BigEndian.PutUint16(row[2*x:], uint16(int32(v)-32768))
		}
		buf.Write(row)
	}
	pad(&buf, 0)
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func pad(buf *bytes.Buffer, b byte) {
	if n := buf.Len() % 2880; n != 0 {
		buf.Write(bytes.Repeat([]byte{b}, 2880-n))
	}
}

// PreviewJPEG renders an auto-stretched, downscaled JPEG for the browser.
func PreviewJPEG(img Image, maxSize int) ([]byte, error) {
	f := 1
	for img.W/f > maxSize || img.H/f > maxSize {
		f++
	}
	w, h := img.W/f, img.H/f
	small := make([]float32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var s float32
			for dy := 0; dy < f; dy++ {
				row := img.Pix[(y*f+dy)*img.W+x*f:]
				for dx := 0; dx < f; dx++ {
					s += row[dx]
				}
			}
			small[y*w+x] = s / float32(f*f)
		}
	}
	// Robust background statistics: median and MAD-based sigma. Star fields
	// are mostly sky, so percentiles alone would stretch the noise.
	sample := make([]float32, 0, len(small)/7+1)
	for i := 0; i < len(small); i += 7 {
		sample = append(sample, small[i])
	}
	sort.Slice(sample, func(i, j int) bool { return sample[i] < sample[j] })
	med := float64(sample[len(sample)/2])
	dev := make([]float64, len(sample))
	for i, v := range sample {
		dev[i] = math.Abs(float64(v) - med)
	}
	sort.Float64s(dev)
	sigma := math.Max(1.4826*dev[len(dev)/2], 1e-3)
	lo, hi := med-1.5*sigma, med+80*sigma
	if top := float64(sample[len(sample)-1]); top < hi {
		hi = math.Max(top, lo+1)
	}
	g := image.NewGray(image.Rect(0, 0, w, h))
	const stretch = 6.0
	k := math.Asinh(stretch)
	for i, v := range small {
		t := math.Max(0, math.Min(1, (float64(v)-lo)/(hi-lo)))
		g.Pix[i] = uint8(255 * math.Asinh(t*stretch) / k)
	}
	var out bytes.Buffer
	err := jpeg.Encode(&out, g, &jpeg.Options{Quality: 75})
	return out.Bytes(), err
}
