package cad

import (
	"testing"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

func TestACI(t *testing.T) {
	c := func(r, g, b uint8) vector.Color { return vector.Color{R: r, G: g, B: b} }
	tests := []struct {
		i    int
		want vector.Color
	}{
		{i: 1, want: c(255, 0, 0)},
		{i: 2, want: c(255, 255, 0)},
		{i: 3, want: c(0, 255, 0)},
		{i: 4, want: c(0, 255, 255)},
		{i: 5, want: c(0, 0, 255)},
		{i: 6, want: c(255, 0, 255)},
		{i: 7, want: c(0, 0, 0)},
		{i: 8, want: c(128, 128, 128)},
		{i: 9, want: c(192, 192, 192)},
		{i: 250, want: c(51, 51, 51)},
		{i: 251, want: c(91, 91, 91)},
		{i: 252, want: c(132, 132, 132)},
		{i: 253, want: c(173, 173, 173)},
		{i: 254, want: c(214, 214, 214)},
		{i: 255, want: c(255, 255, 255)},
		// One index per 60° sector of the hue wheel, so every branch of
		// hsv is pinned to the AutoCAD palette.
		{i: 10, want: c(255, 0, 0)},
		{i: 30, want: c(255, 128, 0)},
		{i: 50, want: c(255, 255, 0)},
		{i: 70, want: c(128, 255, 0)},
		{i: 90, want: c(0, 255, 0)},
		{i: 130, want: c(0, 255, 255)},
		{i: 150, want: c(0, 128, 255)},
		{i: 170, want: c(0, 0, 255)},
		{i: 190, want: c(128, 0, 255)},
		{i: 210, want: c(255, 0, 255)},
		{i: 240, want: c(255, 0, 64)},
		// Odd numbers are the half-saturated tint of the even one below.
		{i: 11, want: c(255, 128, 128)},
		// The last two of each hue are the darkest step.
		{i: 18, want: c(76, 0, 0)},
		{i: 19, want: c(76, 38, 38)},
		{i: 172, want: c(0, 0, 204)},
		// Out of range, BYBLOCK and BYLAYER have no colour of their own.
		{i: 0, want: vector.Black},
		{i: 256, want: vector.Black},
		{i: -1, want: vector.Black},
		{i: 1000, want: vector.Black},
	}
	for _, tt := range tests {
		if got := ACI(tt.i); got != tt.want {
			t.Errorf("ACI(%d) = %v, want %v", tt.i, got, tt.want)
		}
	}
}
