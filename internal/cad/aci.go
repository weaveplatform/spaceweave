package cad

import "github.com/weaveplatform/spaceweave/internal/vector"

// aciBase are colours 1 to 9. Colour 7 is "white on a dark screen, black on
// paper"; we draw on paper.
var aciBase = [10]vector.Color{
	{},
	rgb(255, 0, 0), rgb(255, 255, 0), rgb(0, 255, 0), rgb(0, 255, 255),
	rgb(0, 0, 255), rgb(255, 0, 255), rgb(0, 0, 0), rgb(128, 128, 128), rgb(192, 192, 192),
}

func rgb(r, g, b uint8) vector.Color { return vector.Color{R: r, G: g, B: b} }

// aciGreys are colours 250 to 255.
var aciGreys = [6]uint8{51, 91, 132, 173, 214, 255}

// aciValues are the brightness steps within each hue of colours 10 to 249.
var aciValues = [5]float64{255, 204, 153, 127, 76}

// ACI converts an AutoCAD Color Index number to RGB. Colours 10 to 249 are
// 24 hues 15° apart; within a hue, even numbers are saturated and odd
// numbers half-saturated, at five brightness steps.
func ACI(i int) vector.Color {
	switch {
	case i >= 1 && i <= 9:
		return aciBase[i]
	case i >= 250 && i <= 255:
		g := aciGreys[i-250]
		return vector.Color{R: g, G: g, B: g}
	case i >= 10 && i <= 249:
		hue := float64(i/10-1) * 15
		k := i % 10
		v := aciValues[k/2]
		lo := 0.0
		if k%2 == 1 {
			lo = v / 2
		}
		return hsv(hue, v, lo)
	default:
		return vector.Black
	}
}

// hsv is a colour of the given hue whose strongest channel is v and weakest lo.
func hsv(hue, v, lo float64) vector.Color {
	sector := int(hue / 60)
	f := (hue - float64(sector)*60) / 60
	rise := lo + (v-lo)*f
	fall := v - (v-lo)*f
	var r, g, b float64
	switch sector {
	case 0:
		r, g, b = v, rise, lo
	case 1:
		r, g, b = fall, v, lo
	case 2:
		r, g, b = lo, v, rise
	case 3:
		r, g, b = lo, fall, v
	case 4:
		r, g, b = rise, lo, v
	default:
		r, g, b = v, lo, fall
	}
	return vector.Color{R: uint8(r + 0.5), G: uint8(g + 0.5), B: uint8(b + 0.5)}
}
