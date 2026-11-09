// Package models holds Seestar optics (main/telephoto camera).
//
// Pixel size is 2.9 µm on all four; the Alpaca camera's own PixelSizeX/Y wins
// if it reports something sensible. Orientation doesn't matter for solving.
package models

import (
	"math"
	"regexp"
	"strings"
)

type Model struct {
	Key     string  `json:"key"`
	Name    string  `json:"name"`
	FocalMM float64 `json:"focal_mm"`
	PixelUM float64 `json:"pixel_um"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
}

var All = []Model{
	{"s30", "Seestar S30", 150, 2.9, 1080, 1920},
	{"s30pro", "Seestar S30 Pro", 160, 2.9, 2160, 3840},
	{"s50", "Seestar S50", 250, 2.9, 1080, 1920},
	{"s50pro", "Seestar S50 Pro", 260, 2.9, 2160, 3840},
}

func ByKey(key string) (Model, bool) {
	for _, m := range All {
		if m.Key == key {
			return m, true
		}
	}
	return Model{}, false
}

// ScaleArcsec is the image scale for a pixel size (0 = model default) and binning.
func (m Model) ScaleArcsec(pixelUM float64, binning int) float64 {
	if pixelUM <= 0 {
		pixelUM = m.PixelUM
	}
	return 206.265 * pixelUM * float64(binning) / m.FocalMM
}

// FOVDeg is the angular size of n pixels.
func (m Model) FOVDeg(n int, pixelUM float64, binning int) float64 {
	return float64(n) * m.ScaleArcsec(pixelUM, binning) / 3600
}

func (m Model) DiagDeg() float64 {
	return math.Hypot(float64(m.Width), float64(m.Height)) * m.ScaleArcsec(0, 1) / 3600
}

var modelRe = regexp.MustCompile(`s\s*-?\s*(30|50)\s*(pro)?`)

// Detect guesses the model from device names/descriptions ("Seestar S50 Pro Camera").
func Detect(texts ...string) (Model, bool) {
	m := modelRe.FindStringSubmatch(strings.ToLower(strings.Join(texts, " ")))
	if m == nil {
		return Model{}, false
	}
	key := "s" + m[1]
	if m[2] != "" {
		key += "pro"
	}
	return ByKey(key)
}

// Fallbacks lists models to try for plate-solve scale, most likely first:
// the given one, then the others with the same sensor size, then the rest.
func Fallbacks(first *Model, w, h int) []Model {
	var out []Model
	has := func(k string) bool {
		for _, m := range out {
			if m.Key == k {
				return true
			}
		}
		return false
	}
	if first != nil {
		out = append(out, *first)
	}
	for _, m := range All {
		if !has(m.Key) && ((m.Width == w && m.Height == h) || (m.Width == h && m.Height == w)) {
			out = append(out, m)
		}
	}
	for _, m := range All {
		if !has(m.Key) {
			out = append(out, m)
		}
	}
	return out
}
