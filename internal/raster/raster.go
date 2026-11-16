// Package raster is the image side of plan tracing: decoding a source plan,
// cutting annotated crops for the model to read coordinates off, measuring
// dark runs, proposing wall candidates and drawing a trace back over the
// source. It is pure Go so the worker cross-compiles with CGO disabled.
package raster

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // registered for image.Decode
	_ "image/jpeg"
	"image/png"
	"io"
	"os"

	_ "golang.org/x/image/webp"
)

// ErrEmptyRegion is returned when a requested region misses the image.
var ErrEmptyRegion = errors.New("region does not overlap the image")

// Source is a decoded plan image plus the greyscale copy and ink threshold
// that every measurement uses, computed once.
type Source struct {
	Image     image.Image
	Gray      *image.Gray
	Threshold uint8 // luminance at or below this is ink
}

// Load decodes a PNG, JPEG, GIF or WebP file.
func Load(path string) (*Source, error) {
	f, err := os.Open(path) //nolint:gosec // the path is the worker's own copy of an inbox file
	if err != nil {
		return nil, fmt.Errorf("open plan image: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only
	return Decode(f)
}

// Decode reads an image and prepares it for measurement.
func Decode(r io.Reader) (*Source, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode plan image: %w", err)
	}
	gray := toGray(img)
	return &Source{Image: img, Gray: gray, Threshold: otsu(gray)}, nil
}

// Size is the image's width and height in pixels.
func (s *Source) Size() (int, int) {
	b := s.Image.Bounds()
	return b.Dx(), b.Dy()
}

// Ink reports whether the pixel at x, y counts as drawn.
func (s *Source) Ink(x, y int) bool {
	return s.Gray.GrayAt(x, y).Y <= s.Threshold
}

// toGray flattens transparency onto white first: plan PNGs are often line art
// on a transparent background, which a plain conversion would turn black.
func toGray(img image.Image) *image.Gray {
	b := img.Bounds()
	flat := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(flat, flat.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), img, b.Min, draw.Over)
	gray := image.NewGray(flat.Bounds())
	draw.Draw(gray, gray.Bounds(), flat, image.Point{}, draw.Src)
	return gray
}

// otsu picks the luminance threshold that best separates ink from paper.
// Scanned and anti-aliased plans have no fixed "black", and colour-filled
// rooms would defeat a hard-coded cut-off.
func otsu(g *image.Gray) uint8 {
	var hist [256]int
	for _, v := range g.Pix {
		hist[v]++
	}
	total := len(g.Pix)
	var sum float64
	for i, n := range hist {
		sum += float64(i * n)
	}
	var sumB, best float64
	var wB int
	threshold := uint8(127)
	for t := range 256 {
		wB += hist[t]
		if wB == 0 {
			continue
		}
		wF := total - wB
		if wF == 0 {
			break
		}
		sumB += float64(t * hist[t])
		mB := sumB / float64(wB)
		mF := (sum - sumB) / float64(wF)
		between := float64(wB) * float64(wF) * (mB - mF) * (mB - mF)
		if between > best {
			best = between
			threshold = uint8(t) //nolint:gosec // t < 256
		}
	}
	return threshold
}

// EncodePNG encodes an image as PNG.
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// clip intersects a requested region with the image.
func (s *Source) clip(r image.Rectangle) (image.Rectangle, error) {
	r = r.Canon().Intersect(s.Image.Bounds())
	if r.Empty() {
		return r, ErrEmptyRegion
	}
	return r, nil
}

var (
	white   = color.RGBA{255, 255, 255, 255}
	inkGrey = color.RGBA{60, 60, 60, 255}
)
