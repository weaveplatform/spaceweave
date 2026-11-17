package rasterfront

import (
	"bytes"
	"image"
	"reflect"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/synth"
	"github.com/weaveplatform/spaceweave/internal/raster"
)

// FuzzRead feeds Read arbitrary small images: whatever the pixels, it must
// not panic, and two reads of one image must agree, error or not. The seeds are a plan
// with a slanted wall, outlined and hatched, so the fuzzer starts from the
// paths that turn and fill the image.
func FuzzRead(f *testing.F) {
	tr := synth.New(20).
		Box(20, 20, 140, 100).
		Wall("interior", [2]float64{20, 100}, [2]float64{140, 20}).
		Trace()
	for _, style := range []synth.WallStyle{synth.Solid, synth.Outline, synth.Hatch} {
		g := synth.Render(tr, synth.Options{Walls: style})
		f.Add(uint16(g.Rect.Dx()), g.Pix)
	}
	f.Add(uint16(3), []byte{0, 255, 0, 255, 0, 255})
	f.Fuzz(func(t *testing.T, w uint16, pix []byte) {
		width := int(w%256) + 1
		height := min(len(pix)/width, 256)
		if height == 0 {
			return
		}
		g := image.NewGray(image.Rect(0, 0, width, height))
		copy(g.Pix, pix)
		src := raster.NewSource(g)
		a, errA := Read(src, Options{})
		b, errB := Read(raster.NewSource(&image.Gray{Pix: bytes.Clone(g.Pix), Stride: g.Stride, Rect: g.Rect}), Options{})
		if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(errA, errB) {
			t.Fatal("two reads of one image differ")
		}
	})
}
