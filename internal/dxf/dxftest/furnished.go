package dxftest

import "math"

// Layers of Furnished, named the BS 1192 / Uniclass 2015 way: the code
// says what the layer holds, the words are free.
const (
	FurnLayer     = "A-Ss_40_15-Furniture"
	SanitaryLayer = "A-Pr_40_20-Sanitary"
	DeskNumLayer  = "A-Zz_20_10_90-Desk Numbers"
)

// Furnished is a furniture layout in millimetres whose blocks are named
// the way furniture libraries and exploded vendor files leave them:
// anonymous ("A$C…") or vendor codes that say nothing, so only what each
// block draws, and the desk numbers on their own layer, say what it is.
// Every block's front faces its -y, as symbol libraries draw them.
//
//   - a bench of six 1200 × 800 desks (three a side, each its own
//     outline) at (2000, 2000), turned 30°, its desks numbered; another,
//     unnumbered, at (2000, 8000);
//   - a bench of four 1400 × 800 desks named only by its count and
//     module, at (8000, 8000);
//   - a single 1600 × 800 desk, numbered, at (8000, 2000), with a chair
//     drawn facing away from it in front (y 1300);
//   - a workstation (a desk and a chair nested in one block) at
//     (14000, 2000);
//   - a meeting set (a 1200 mm round table and four chairs, nested) at
//     (14000, 8000);
//   - a cupboard drawn with its two doors open, at (20000, 2000); a tall
//     unit marked with a cross, at (20000, 5000); a 400 mm stool and an
//     800 mm round table, at (20000, 8000) and (22000, 8000); a 500 mm
//     pouffe at (24000, 8000);
//   - a WC, a basin and a hand dryer on the sanitary layer, at y 12000;
//   - a coat stand and an acoustic panel, which the plan has no type for.
func Furnished() *Doc {
	d := New(4).Layer(FurnLayer, 3, 0).Layer(SanitaryLayer, 5, 0).Layer(DeskNumLayer, 1, 0)
	d.Block("A$C0ZQC", 0, 0, chairSymbol()...)

	bench := make([]Ent, 0, 6)
	for row := range 2 {
		for col := range 3 {
			x, y := float64(col)*1200-1800, float64(row)*800-800
			bench = append(bench, Rect("0", x, y, x+1200, y+800))
		}
	}
	d.Block("A$C0ZQB", 0, 0, bench...)
	d.Block("XQ_BB_4_1400", 0, 0,
		Rect("0", -1400, -800, 1400, 800), Line("0", -1400, 0, 1400, 0), Line("0", 0, -800, 0, 800))
	d.Block("A$C0ZQD", 0, 0, Rect("0", -800, -400, 800, 400))
	// The chair turned to face the desk behind it.
	d.Block("A$C0ZQW", 0, 0,
		Insert("0", "A$C0ZQD", 0, 400, 0), Insert("0", "A$C0ZQC", 0, -300, 180))
	meeting := make([]Ent, 0, 5)
	meeting = append(meeting, E("CIRCLE", 8, "0", 10, 0.0, 20, 0.0, 40, 600.0))
	for i := range 4 {
		a := float64(i) * 90
		s, c := math.Sincos(a * math.Pi / 180)
		// Each chair faces the table: its -y towards the centre.
		meeting = append(meeting, Insert("0", "A$C0ZQC", 900*c, 900*s, a-90))
	}
	d.Block("A$C0ZQM", 0, 0, meeting...)
	d.Block("A$C0ZQK", 0, 0,
		Rect("0", 0, 0, 800, 450),
		Arc("0", 0, 0, 400, 270, 360), Line("0", 0, 0, 0, -400),
		Arc("0", 800, 0, 400, 180, 270), Line("0", 800, 0, 800, -400),
	)
	d.Block("A$C0ZQT", 0, 0,
		Rect("0", 0, 0, 1000, 600), Line("0", 0, 0, 1000, 600), Line("0", 0, 600, 1000, 0))
	d.Block("A$C0ZQS", 0, 0, E("CIRCLE", 8, "0", 10, 0.0, 20, 0.0, 40, 200.0))
	d.Block("A$C0ZQR", 0, 0, E("CIRCLE", 8, "0", 10, 0.0, 20, 0.0, 40, 400.0))
	d.Block("A$C0ZQP", 0, 0, Rect("0", -250, -250, 250, 250))
	d.Block("A$C0ZQL", 0, 0, Rect("0", -200, 0, 200, 180), ellipse(0, -280, 0, 280, 0.68))
	d.Block("A$C0ZQN", 0, 0,
		ellipse(0, 0, 250, 0, 0.8), E("CIRCLE", 8, "0", 10, 0.0, 20, 0.0, 40, 20.0))
	d.Block("A$C0ZQH", 0, 0, Rect("0", -150, 0, 150, 150))
	d.Block("COAT_STAND", 0, 0, E("CIRCLE", 8, "0", 10, 0.0, 20, 0.0, 40, 190.0))
	d.Block("A$C0ZQF", 0, 0, Rect("0", -800, 0, 800, 50))

	d.Add(Insert(FurnLayer, "A$C0ZQB", 2000, 2000, 30), Insert(FurnLayer, "A$C0ZQB", 2000, 8000, 0))
	for row := range 2 {
		for col := range 3 {
			s, c := math.Sincos(30 * math.Pi / 180)
			x, y := float64(col)*1200-1200, float64(row)*800-400
			d.Add(Text(DeskNumLayer, 2000+x*c-y*s-150, 2000+x*s+y*c-50, 100,
				"D-"+string(rune('1'+row*3+col))))
		}
	}
	d.Add(
		Insert(FurnLayer, "XQ_BB_4_1400", 8000, 8000, 0),
		Insert(FurnLayer, "A$C0ZQD", 8000, 2000, 0), Text(DeskNumLayer, 7850, 1950, 100, "D-7"),
		// Drawn facing away from its desk, which is above it.
		Insert(FurnLayer, "A$C0ZQC", 8000, 1300, 0),
		Insert(FurnLayer, "A$C0ZQW", 14000, 2000, 0),
		Insert(FurnLayer, "A$C0ZQM", 14000, 8000, 0),
		Insert(FurnLayer, "A$C0ZQK", 20000, 2000, 0),
		Insert(FurnLayer, "A$C0ZQT", 20000, 5000, 0),
		Insert(FurnLayer, "A$C0ZQS", 20000, 8000, 0),
		Insert(FurnLayer, "A$C0ZQR", 22000, 8000, 0),
		Insert(FurnLayer, "A$C0ZQP", 24000, 8000, 0),
		Insert(SanitaryLayer, "A$C0ZQL", 2000, 12000, 0),
		Insert(SanitaryLayer, "A$C0ZQN", 4000, 12000, 0),
		Insert(SanitaryLayer, "A$C0ZQH", 6000, 12000, 0),
		Insert(FurnLayer, "COAT_STAND", 8000, 12000, 0),
		Insert(FurnLayer, "A$C0ZQF", 10000, 12000, 0),
	)
	return d
}

// ellipse is a whole ELLIPSE on layer 0 centred at (cx, cy), its major
// axis (mx, my) from the centre.
func ellipse(cx, cy, mx, my, ratio float64) Ent {
	return E("ELLIPSE", 8, "0", 10, cx, 20, cy, 11, mx, 21, my, 40, ratio, 41, 0.0, 42, 2*math.Pi)
}

// chairSymbol is a task chair about 560 mm square, drawn in detail as
// libraries draw them: a seat of short strokes, a curved back at +y,
// arms and five castors.
func chairSymbol() []Ent {
	const n = 24
	out := make([]Ent, 0, n+4+8+5)
	pt := func(i int) (float64, float64) {
		s, c := math.Sincos(2 * math.Pi * float64(i) / n)
		return 230 * c, -30 + 210*s
	}
	for i := range n {
		x0, y0 := pt(i)
		x1, y1 := pt(i + 1)
		out = append(out, Line("0", x0, y0, x1, y1))
	}
	out = append(out,
		Arc("0", 0, -30, 260, 30, 150), Arc("0", 0, -30, 300, 30, 150),
		Line("0", 225, 100, 260, 120), Line("0", -225, 100, -260, 120),
	)
	for _, x := range []float64{-280, 250} {
		out = append(out,
			Line("0", x, -150, x+30, -150), Line("0", x+30, -150, x+30, 100),
			Line("0", x+30, 100, x, 100), Line("0", x, 100, x, -150))
	}
	for i := range 5 {
		s, c := math.Sincos(2*math.Pi*float64(i)/5 - math.Pi/2)
		out = append(out, Line("0", 0, -30, 280*c, -30+280*s))
	}
	return out
}
