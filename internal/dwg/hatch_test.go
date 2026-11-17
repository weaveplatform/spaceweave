package dwg

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dwg/dwgtest"
	"github.com/weaveplatform/spaceweave/internal/dxf"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// TestEdgeLoop checks a boundary of every edge type reads as the DXF
// reader reads the same boundary, which is the behaviour the two share.
func TestEdgeLoop(t *testing.T) {
	// DXF stores the angles in degrees; DWG in radians.
	pairs := []any{
		8,
		"0",
		2,
		"SOLID",
		70,
		1,
		91,
		1,
		92,
		0,
		93,
		6,
		72,
		1,
		10,
		0.0,
		20,
		0.0,
		11,
		10.0,
		21,
		0.0,
		72,
		2,
		10,
		10.0,
		20,
		5.0,
		40,
		5.0,
		50,
		270.0,
		51,
		90.0,
		73,
		1,
		72,
		2,
		10,
		5.0,
		20,
		10.0,
		40,
		5.0,
		50,
		0.0,
		51,
		90.0,
		73,
		0,
		72,
		3,
		10,
		0.0,
		20,
		5.0,
		11,
		0.0,
		21,
		5.0,
		40,
		0.5,
		50,
		90.0,
		51,
		270.0,
		73,
		1,
		72,
		4,
		94,
		3,
		73,
		1,
		74,
		0,
		95,
		8,
		96,
		4,
		40,
		0.0,
		40,
		0.0,
		40,
		0.0,
		40,
		0.0,
		40,
		1.0,
		40,
		1.0,
		40,
		1.0,
		40,
		1.0,
		10,
		0.0,
		20,
		0.0,
		42,
		1.0,
		10,
		1.0,
		20,
		-1.0,
		42,
		0.5,
		10,
		2.0,
		20,
		-1.0,
		42,
		0.5,
		10,
		3.0,
		20,
		0.0,
		42,
		1.0,
		97,
		2,
		11,
		0.0,
		21,
		0.0,
		11,
		3.0,
		21,
		0.0,
		12,
		1.0,
		22,
		0.0,
		13,
		1.0,
		23,
		0.0,
		72,
		4,
		94,
		2,
		73,
		0,
		74,
		0,
		95,
		0,
		96,
		0,
		97,
		3,
		11,
		3.0,
		21,
		0.0,
		11,
		4.0,
		21,
		1.0,
		11,
		5.0,
		21,
		0.0,
		97,
		0,
	}
	doc, err := dxf.Read(bytes.NewReader(dxftest.New(0).Add(dxftest.E("HATCH", pairs...)).Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	want := doc.Model[0].(*cad.Hatch).Boundary
	if len(want.Segs) < 20 {
		t.Fatalf("the DXF reader read only %d segments", len(want.Segs))
	}

	w := &dwgtest.Bits{}
	w.BL(6)
	w.RC(1)
	w.RD2(0, 0)
	w.RD2(10, 0)
	w.RC(2)
	w.RD2(10, 5)
	w.BD(5)
	w.BD(3 * math.Pi / 2)
	w.BD(math.Pi / 2)
	w.B(true)
	w.RC(2)
	w.RD2(5, 10)
	w.BD(5)
	w.BD(0)
	w.BD(math.Pi / 2)
	w.B(false)
	w.RC(3)
	w.RD2(0, 5)
	w.RD2(0, 5)
	w.BD(0.5)
	w.BD(math.Pi / 2)
	w.BD(3 * math.Pi / 2)
	w.B(true)
	w.RC(4)
	w.BL(3)
	w.B(true)
	w.B(false)
	w.BL(8)
	w.BL(4)
	for _, k := range []float64{0, 0, 0, 0, 1, 1, 1, 1} {
		w.BD(k)
	}
	for _, p := range [][3]float64{{0, 0, 1}, {1, -1, 0.5}, {2, -1, 0.5}, {3, 0, 1}} {
		w.RD2(p[0], p[1])
		w.BD(p[2])
	}
	w.BL(2)
	w.RD2(0, 0)
	w.RD2(3, 0)
	w.RD2(1, 0)
	w.RD2(1, 0)
	w.RC(4)
	w.BL(2)
	w.B(false)
	w.B(false)
	w.BL(0)
	w.BL(0)
	w.BL(3)
	w.RD2(3, 0)
	w.RD2(4, 1)
	w.RD2(5, 0)
	w.RD2(0, 0)
	w.RD2(0, 0)

	var got vector.Path
	r := newBits(w.Bytes())
	edgeLoop(&got, r, v2010)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if !equal(got.Segs, want.Segs) {
		t.Errorf("edges\n got %+v\nwant %+v", got.Segs, want.Segs)
	}

	// Before R2010 a spline edge has no fit points.
	w = &dwgtest.Bits{}
	w.BL(1)
	w.RC(4)
	w.BL(1)
	w.B(false)
	w.B(false)
	w.BL(2)
	w.BL(2)
	w.BD(0)
	w.BD(1)
	w.RD2(0, 0)
	w.RD2(1, 1)
	got = vector.Path{}
	r = newBits(w.Bytes())
	edgeLoop(&got, r, v2004)
	if r.err != nil || len(got.Segs) < 2 || got.Segs[len(got.Segs)-1].Op != vector.Close {
		t.Errorf("old spline edge: %v %+v", r.err, got.Segs)
	}
}

func TestEdgeLoopBadType(t *testing.T) {
	w := &dwgtest.Bits{}
	w.BL(2)
	w.RC(9)
	w.RC(1)
	var p vector.Path
	r := newBits(w.Bytes())
	edgeLoop(&p, r, v2000)
	if !errors.Is(r.err, ErrCorrupt) || len(p.Segs) != 0 {
		t.Errorf("err %v, segs %v", r.err, p.Segs)
	}
}

func TestSweep(t *testing.T) {
	for _, c := range []struct {
		a0, a1 float64
		ccw    bool
		t0, t1 float64
	}{
		{0, math.Pi / 2, true, 0, math.Pi / 2},
		{1, 1, true, 1, 1 + 2*math.Pi},
		{0, math.Pi / 2, false, 0, -math.Pi / 2},
		{math.Inf(1), 0, true, math.Inf(1), math.Inf(1)},
	} {
		t0, t1 := sweep(c.a0, c.a1, c.ccw)
		if !equal([]float64{t0, t1}, []float64{c.t0, c.t1}) {
			t.Errorf(
				"sweep(%v, %v, %v) = %v, %v; want %v, %v",
				c.a0,
				c.a1,
				c.ccw,
				t0,
				t1,
				c.t0,
				c.t1,
			)
		}
	}
}
