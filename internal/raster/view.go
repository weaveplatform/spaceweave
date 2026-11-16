package raster

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// MaxEdge is the longest side of an image sent to the model. Larger images are
// downscaled by the API anyway, which would silently break the pixel rulers.
const MaxEdge = 1568

// MaxZoom caps upscaling of small crops: beyond 4× the model sees blocky
// pixels, not more detail.
const MaxZoom = 4.0

const margin = 34 // ruler band, room for "12345" in basicfont

// View is an annotated crop: the region of the source it shows and the scale
// it was drawn at, so tool results can tell the model how to read it.
type View struct {
	Image  *image.RGBA
	Region image.Rectangle // in source pixels
	Scale  float64         // output pixels per source pixel
}

// Crop cuts region out of img, scales it to fit MaxEdge (enlarging small
// regions up to MaxZoom), and draws source-pixel rulers along the top and left
// so positions can be read straight off the picture. gridPx > 0 also draws a
// light grid at that spacing in source pixels.
func Crop(img image.Image, region image.Rectangle, gridPx int) (*View, error) {
	r := region.Canon().Intersect(img.Bounds())
	if r.Empty() {
		return nil, ErrEmptyRegion
	}
	scale := math.Min(MaxZoom, float64(MaxEdge-margin)/float64(max(r.Dx(), r.Dy())))
	w := max(1, int(math.Round(float64(r.Dx())*scale)))
	h := max(1, int(math.Round(float64(r.Dy())*scale)))

	out := image.NewRGBA(image.Rect(0, 0, w+margin, h+margin))
	draw.Draw(out, out.Bounds(), image.NewUniform(white), image.Point{}, draw.Src)
	body := image.Rect(margin, margin, margin+w, margin+h)
	// Enlarged crops keep hard pixel edges so wall faces can be measured;
	// reduced ones are filtered so thin lines don't vanish.
	var scaler xdraw.Interpolator = xdraw.CatmullRom
	if scale >= 1 {
		scaler = xdraw.NearestNeighbor
	}
	scaler.Scale(out, body, img, r, draw.Over, nil)

	v := &View{Image: out, Region: r, Scale: scale}
	if gridPx > 0 {
		v.grid(gridPx)
	}
	v.rulers()
	return v, nil
}

// At maps a source pixel to the output image.
func (v *View) At(x, y float64) (float64, float64) {
	return margin + (x-float64(v.Region.Min.X))*v.Scale, margin + (y-float64(v.Region.Min.Y))*v.Scale
}

// tickStep picks a round source-pixel spacing that lands ticks at least
// minGap output pixels apart.
func tickStep(scale float64) int {
	const minGap = 70
	for _, step := range []int{1, 2, 5, 10, 20, 25, 50, 100, 200, 250, 500, 1000, 2000, 5000} {
		if float64(step)*scale >= minGap {
			return step
		}
	}
	return 10000
}

func (v *View) rulers() {
	step := tickStep(v.Scale)
	minor := max(1, step/5)
	b := v.Image.Bounds()
	for x := ceilTo(v.Region.Min.X, minor); x <= v.Region.Max.X; x += minor {
		ox, _ := v.At(float64(x), 0)
		length := 5
		if x%step == 0 {
			length = 12
			label(v.Image, int(ox)+2, 12, strconv.Itoa(x))
		}
		vline(v.Image, int(ox), margin-length, margin-1, inkGrey)
	}
	for y := ceilTo(v.Region.Min.Y, minor); y <= v.Region.Max.Y; y += minor {
		_, oy := v.At(0, float64(y))
		length := 5
		if y%step == 0 {
			length = 12
			label(v.Image, 1, int(oy)-2, strconv.Itoa(y))
		}
		hline(v.Image, margin-length, margin-1, int(oy), inkGrey)
	}
	hline(v.Image, margin, b.Max.X-1, margin-1, inkGrey)
	vline(v.Image, margin-1, margin, b.Max.Y-1, inkGrey)
}

func (v *View) grid(spacing int) {
	c := color.RGBA{224, 17, 95, 255}
	b := v.Image.Bounds()
	for x := ceilTo(v.Region.Min.X, spacing); x <= v.Region.Max.X; x += spacing {
		ox, _ := v.At(float64(x), 0)
		for y := margin; y < b.Max.Y; y += 2 { // dotted, so ink underneath stays legible
			blend(v.Image, int(ox), y, c, 0.5)
		}
	}
	for y := ceilTo(v.Region.Min.Y, spacing); y <= v.Region.Max.Y; y += spacing {
		_, oy := v.At(0, float64(y))
		for x := margin; x < b.Max.X; x += 2 {
			blend(v.Image, x, int(oy), c, 0.5)
		}
	}
}

func scaleDown(dst *image.RGBA, src image.Image) {
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
}

func ceilTo(v, step int) int {
	if r := v % step; r != 0 {
		if v < 0 {
			return v - r
		}
		return v + step - r
	}
	return v
}

func hline(img *image.RGBA, x0, x1, y int, c color.RGBA) {
	for x := x0; x <= x1; x++ {
		img.SetRGBA(x, y, c)
	}
}

func vline(img *image.RGBA, x, y0, y1 int, c color.RGBA) {
	for y := y0; y <= y1; y++ {
		img.SetRGBA(x, y, c)
	}
}

// blend mixes c into the pixel at x, y with opacity a.
func blend(img *image.RGBA, x, y int, c color.RGBA, a float64) {
	if !(image.Point{X: x, Y: y}).In(img.Bounds()) {
		return
	}
	o := img.RGBAAt(x, y)
	mix := func(p, q uint8) uint8 { return uint8(float64(p)*(1-a) + float64(q)*a) }
	img.SetRGBA(x, y, color.RGBA{mix(o.R, c.R), mix(o.G, c.G), mix(o.B, c.B), 255})
}

func label(img *image.RGBA, x, y int, s string) {
	labelColor(img, x, y, s, inkGrey)
}

func labelColor(img *image.RGBA, x, y int, s string, c color.Color) {
	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}
