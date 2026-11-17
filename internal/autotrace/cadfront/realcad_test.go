package cadfront

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dwg"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
	"github.com/weaveplatform/spaceweave/internal/trace"
	"github.com/weaveplatform/spaceweave/internal/vector"
)

// TestRealCAD measures the CAD tracer against real drawings, which can't
// live in the repository: set SPACEWEAVE_DWG_DIR to a directory holding
// arex/ (architectural plans, one DWG per floor, Uniclass 2015 / BS 1192
// layers, millimetres) and space/ (the same floors' CAFM space plans, whose
// area polygons and numbers are the room truth). Floors pair up by the
// two-digit number ending each file name. SPACEWEAVE_EVAL_OUT, when set, is
// where a picture of each floor's trace over its truth is written.
//
// The truth is read from the drawings by layer, independently of the
// tracer's own layer rules, so a rule that misreads a layer shows up as an
// error rather than agreeing with itself. What each number means is in
// the table's legend (realcadLegend).
func TestRealCAD(t *testing.T) {
	dir := os.Getenv("SPACEWEAVE_DWG_DIR")
	if dir == "" {
		t.Skip("set SPACEWEAVE_DWG_DIR to a directory with arex/ and space/ drawings")
	}
	arex, _ := filepath.Glob(filepath.Join(dir, "arex", "*.dwg"))
	spaces, _ := filepath.Glob(filepath.Join(dir, "space", "*.dwg"))
	if len(arex) == 0 {
		t.Fatalf("no arex/*.dwg under %s", dir)
	}
	slices.Sort(arex)
	byFloor := map[string]string{}
	for _, s := range spaces {
		byFloor[floorOf(s)] = s
	}
	var rows []evalRow
	only := os.Getenv("SPACEWEAVE_EVAL_FLOORS") // e.g. "02,03"; every floor when empty
	pick := func(f string) bool { return only == "" || slices.Contains(strings.Split(only, ","), f) }
	for _, a := range arex {
		f := floorOf(a)
		if !pick(f) {
			continue
		}
		row := evalFloor(t, f, a, byFloor[f])
		rows = append(rows, row)
	}
	for _, s := range spaces {
		if !pick(floorOf(s)) {
			continue
		}
		rows = append(rows, evalSpace(t, floorOf(s), s))
	}
	t.Log("\n" + realcadLegend + "\n" + table(rows))
}

func floorOf(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if len(base) < 2 {
		return base
	}
	return base[len(base)-2:]
}

const realcadLegend = `Legend (all lengths in drawing millimetres unless said):
  wallR   recall: share of wall-ink length (wall, glazing, partition, structural-wall and curtain-wall layers; columns excluded) within half a traced wall's thickness + 50 mm of its centreline
  wallP   precision: share of traced wall centreline length (less its openings) with wall ink within half its thickness + 50 mm
  doorR   share of door swings (arcs on door layers or in door blocks; a double door counts once) matched by a traced door: an end within 150 mm of the hinge and a width within 25% of the leaf
  kind    of matched doors, share with the right kind (door / double-door)
  hinge   of matched single doors, share hinged at the right end
  swing   of matched doors, share swinging to the side the arc is on
  doorP   share of traced doors that match a swing
  encl    SPACE area polygons whose boundary lies >= 90% on wall, glazing or door ink (within 250 mm): enclosed rooms
  roomR   share of enclosed areas matched by a traced room with IoU >= 0.9 (in brackets: >= 0.75)
  roomB   share of enclosed areas whose boundary and the best traced room's lie, on average, within 200 mm of each other both ways (forgives face vs centreline)
  roomP   share of traced rooms matching an enclosed area with IoU >= 0.9
  openCov share of open (not enclosed) area covered by traced rooms
  nameOK  of rooms matched at IoU >= 0.75 that the drawing traced names (AREX: a room-name text inside the area; SPACE: its area number), share whose traced name holds that name or number or is its first line
  scale   traced px/m over the truth (AREX: from millimetres; SPACE: from stated square feet vs polygon areas)
  furn    share of top-level furniture and sanitary inserts with a plan item centred in their box, long side within 30% and depth 40–130% of the box's (a cupboard's box holds its doors' swing), or several items that between them cover it; type checks in the notes
  valid   trace.Finalize (plan.Check, plan.Convert and the site validator when node is installed)`

type evalRow struct {
	name                            string
	read, build                     time.Duration
	wallInk, wallR, wallP           float64
	doors                           int
	doorR, kind, hinge, swing, dP   float64
	areas, encl                     int
	roomR, roomR75, roomP, openCov  float64
	roomB                           float64
	traced                          int
	nameOK                          float64
	nameN                           int
	scale                           float64
	furn                            float64
	furnN                           int
	valid                           string
	notes                           []string
}

func table(rows []evalRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-10s %6s %6s %6s %6s | %5s %6s %5s %5s %5s %5s | %5s %5s %12s %5s %5s %6s %6s | %6s %7s | %10s %6s |\n",
		"floor", "read", "build", "wallR", "wallP", "doors", "doorR", "kind", "hinge", "swing", "doorP",
		"areas", "encl", "roomR(.75)", "roomB", "roomP", "rooms", "openCv", "nameOK", "scale", "furn", "valid")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-10s %6.1fs %5.1fs %6s %6s | %5d %6s %5s %5s %5s %5s | %5d %5d %5s (%5s) %5s %5s %6d %6s | %6s %7.4f | %10s %6s |\n",
			r.name, r.read.Seconds(), r.build.Seconds(), pc(r.wallR), pc(r.wallP), r.doors, pc(r.doorR),
			pc(r.kind), pc(r.hinge), pc(r.swing), pc(r.dP), r.areas, r.encl, pc(r.roomR), pc(r.roomR75),
			pc(r.roomB), pc(r.roomP), r.traced, pc(r.openCov), fmt.Sprintf("%s/%d", pc(r.nameOK), r.nameN), r.scale,
			fmt.Sprintf("%s/%d", pc(r.furn), r.furnN), r.valid)
		for _, n := range r.notes {
			fmt.Fprintf(&b, "    %s\n", n)
		}
	}
	return b.String()
}

func pc(v float64) string {
	if math.IsNaN(v) {
		return "-"
	}
	return fmt.Sprintf("%.1f", 100*v)
}

func ratio(n, d int) float64 {
	if d == 0 {
		return math.NaN()
	}
	return float64(n) / float64(d)
}

// Truth layers for Uniclass 2015 / BS 1192 drawings, by the codes and words
// the architects used: EF_25_10 walls and internal glazing, EF_25_20
// structure, EF_25_30 doors and windows (the windows layer holds the
// curtain walling), and the partition and screen layers older revisions
// left behind.
var (
	truthWall = regexp.MustCompile(`(?i)EF_25_10|EF_25_20|EF_25_30-X_Window|Partition|Glazed Screen`)
	truthDoor = regexp.MustCompile(`(?i)EF_25_30.*Door|A315`)
	truthFurn = regexp.MustCompile(`(?i)Ss_40_15|Pr_40_20|Furniture`)
	truthName = regexp.MustCompile(`(?i)Zz_20_10_70_65-T_Room|BOCA-TEXT`)
	truthDesk = regexp.MustCompile(`(?i)Desk Number`)
	// noPlanType names furniture the plan has no type for, whatever the
	// tracer makes of it: the denominator of the "could be" share.
	noPlanType = regexp.MustCompile(`(?i)coat|screen|wall|panel|acoustic|dryer|dispenser|booth|` +
		`pod\b|lamp|\btap\b|mixer|bracket|pilaster|hinge|indicator|pipe|divider|trolley|bin\b|recycl`)
)

type seg2 struct {
	a, b  point
	layer string
}

type truthDoorT struct {
	hinge, hinge2, mid point
	rad                float64
	double             bool
	ends               [2]point
}

type truthArea struct {
	poly     []point
	number   string
	sf       float64
	enclosed bool
	names    []string // AREX names and numbers inside
	box      vector.Rect
}

type furnBox struct {
	box  vector.Rect // in the drawing turned by turn
	name string
	turn vector.Affine
}

// has reports whether a drawing point is in the box.
func (f furnBox) has(p point) bool { return boxHas(f.box, f.turn.Apply(p)) }

type arexTruth struct {
	walls, doorInk []seg2
	doors          []truthDoorT
	furniture      []furnBox
	deskNums       []point
	labels         []labelAt
}

type labelAt struct {
	at   point
	text string
}

func evalFloor(t *testing.T, floor, arexPath, spacePath string) evalRow {
	t.Helper()
	row := evalRow{name: "AREX-" + floor}
	start := time.Now()
	doc, err := dwg.ReadFile(arexPath)
	if err != nil {
		t.Errorf("%s: %v", arexPath, err)
		return row
	}
	d, err := cad.Draw(doc)
	if err != nil {
		t.Errorf("draw: %v", err)
		return row
	}
	ras, err := vector.Rasterize(d, vector.Options{})
	if err != nil {
		t.Errorf("rasterise: %v", err)
		return row
	}
	o := Options{View: ras.View(), Size: [2]int{ras.Image.Bounds().Dx(), ras.Image.Bounds().Dy()}}
	if d.Unit > 0 && !d.Paper {
		o.PxPerMetre, o.Exact = ras.PxPerUnit/d.Unit, !d.Assumed
	}
	o.Title = "AREX-" + floor
	in, err := Read(doc, o)
	row.read = time.Since(start)
	if err != nil {
		t.Errorf("read: %v", err)
		return row
	}
	if os.Getenv("SPACEWEAVE_EVAL_TWICE") != "" {
		again, _ := Read(doc, o)
		row.notes = append(row.notes, fmt.Sprintf("read twice, identical: %v", reflect.DeepEqual(in, again)))
	}
	start = time.Now()
	res, err := floorgraph.Build(in)
	row.build = time.Since(start)
	if err != nil {
		t.Errorf("build: %v", err)
		row.valid = err.Error()
		return row
	}
	row.notes = append(row.notes, fmt.Sprintf("input: %d walls, %d openings, %d labels, %d furniture; coverage %.2f; conf %.2f; %s",
		len(in.Walls), len(in.Openings), len(in.Labels), len(in.Furniture), in.Coverage, res.Confidence, res.Summary))
	for _, c := range res.Caveats {
		row.notes = append(row.notes, "caveat: "+c)
	}
	row.valid = finalize(t, res.Trace, ras)

	inv, _ := inverse(o.View)
	truthPxPerM := ras.PxPerUnit * 1000 // AREX is drawn in millimetres
	row.scale = res.Trace.Calibration.PxPerMetre[0] / truthPxPerM

	at := readArexTruth(doc)
	tw := tracedWalls(res.Trace, inv)
	row.wallR, row.wallP = wallScores(at.walls, tw)
	scoreDoors(&row, at.doors, tw)
	inputMiss, dropped := 0, 0
	var wrong []string
	var missAt []string
	for _, d := range at.doors {
		near := func(p point) bool { return p.Sub(d.hinge).Len() <= 150 }
		inTrace := false
		for _, w := range tw {
			for _, o := range w.ops {
				inTrace = inTrace || near(o.a) || near(o.b)
			}
		}
		if inTrace {
			var ws []string
			matched := false
			for _, w := range tw {
				for _, o := range w.ops {
					if near(o.a) || near(o.b) {
						width := o.a.Sub(o.b).Len()
						ws = append(ws, fmt.Sprintf("%s w%.0f", o.kind, width))
						matched = matched || math.Abs(width-d.rad) <= 0.25*d.rad
					}
				}
			}
			if !matched {
				wrong = append(wrong, fmt.Sprintf("(%.0f,%.0f r%.0f double=%v: %v)", d.hinge.X, d.hinge.Y, d.rad, d.double, ws))
			}
			continue
		}
		found := false
		for _, o := range in.Openings {
			found = found || near(inv.Apply(o.A)) || near(inv.Apply(o.B))
		}
		if found {
			dropped++
		} else {
			inputMiss++
			missAt = append(missAt, fmt.Sprintf("(%.0f,%.0f r%.0f)", d.hinge.X, d.hinge.Y, d.rad))
		}
	}
	row.notes = append(row.notes, fmt.Sprintf("doors with no opening at the hinge: %d not found by cadfront %v, %d dropped by floorgraph",
		inputMiss, missAt, dropped), fmt.Sprintf("doors traced at the hinge with the wrong width: %v", wrong))

	var areas []truthArea
	if spacePath != "" {
		if sdoc, err := dwg.ReadFile(spacePath); err == nil {
			areas, _ = readSpaceAreas(sdoc)
		}
	}
	for i := range areas {
		a := &areas[i]
		a.enclosed = enclosedBy(a.poly, append(slices.Clone(at.walls), at.doorInk...))
		for _, l := range at.labels {
			if boxHas(a.box, l.at) && insidePoly(a.poly, l.at) {
				a.names = append(a.names, l.text)
			}
		}
	}
	scoreRooms(&row, areas, res.Trace, inv, false)
	scoreFurniture(&row, at, res.Trace, inv)
	row.notes = append(row.notes, scoreDefects(at, areas, res.Trace, inv).String())
	if out := os.Getenv("SPACEWEAVE_EVAL_OUT"); out != "" {
		drawEval(t, filepath.Join(out, "arex-"+floor+".png"), at, areas, tw, res.Trace, inv)
	}
	return row
}

// finalize puts the trace through what a published plan must pass.
func finalize(t *testing.T, tr *plan.Trace, ras *vector.Raster) string {
	t.Helper()
	var v plan.Validator = okValidator{}
	script, _ := filepath.Abs("../../../web/floorplan-iso/tools/validate.mjs")
	if _, err := exec.LookPath("node"); err == nil {
		if _, err := os.Stat(script); err == nil {
			v = plan.NodeValidator{Node: "node", Script: script}
		}
	}
	w, h := ras.Image.Bounds().Dx(), ras.Image.Bounds().Dy()
	job := trace.Job{
		Image: raster.NewSource(ras.Image),
		Meta:  plan.Meta{URL: "x.png", Credit: "eval", SizePx: [2]int{w, h}},
	}
	f, err := trace.Finalize(context.Background(), job, tr, v)
	switch {
	case err != nil:
		return "error: " + err.Error()
	case !f.Valid():
		return fmt.Sprintf("NO(%d): %s", len(f.Problems), f.Problems[0])
	}
	if _, ok := v.(okValidator); ok {
		return "ok(no node)"
	}
	return "ok"
}

type okValidator struct{}

func (okValidator) Validate(context.Context, []byte) ([]string, error) { return nil, nil }

func readArexTruth(doc *cad.Document) arexTruth {
	var at arexTruth
	var swingsFound []swing
	_, _ = cad.Walk(doc, func(p *cad.Placed) error {
		layer := layerName(p)
		if _, ok := p.Entity.(*cad.Dimension); ok {
			return cad.SkipBlock
		}
		if in, ok := p.Entity.(*cad.Insert); ok {
			if p.Parent == nil && truthFurn.MatchString(layer) {
				// The box in the insert's own axes: the floors are drawn
				// turned, and a turned desk's upright box is far bigger
				// than the desk.
				turn := vector.Rotate(-p.M.Angle())
				var box vector.Rect
				walkBox(doc, in, p.M.Then(turn), &box)
				if !box.Empty() {
					at.furniture = append(at.furniture, furnBox{box: box, name: in.Block, turn: turn})
				}
			}
			return nil
		}
		if truthDesk.MatchString(layer) {
			if ls := cad.TextLines(p.Entity, p.M); len(ls) > 0 {
				at.deskNums = append(at.deskNums, ls[0].Bounds().Center())
			}
		}
		if truthName.MatchString(layer) {
			if ls := cad.TextLines(p.Entity, p.M); len(ls) > 0 {
				var box vector.Rect
				var txt []string
				for _, l := range ls {
					box = box.Union(l.Bounds())
					txt = append(txt, strings.TrimSpace(l.Content))
				}
				at.labels = append(at.labels, labelAt{at: box.Center(), text: strings.Join(txt, " ")})
			}
			return nil
		}
		isWall, isDoor := truthWall.MatchString(layer), truthDoor.MatchString(layer)
		if !isDoor {
			for q := p.Parent; q != nil && !isDoor; q = q.Parent {
				isDoor = truthDoor.MatchString(q.Layer.Name)
			}
		}
		switch e := p.Entity.(type) {
		case *cad.Line:
			s := seg2{p.M.Apply(e.A), p.M.Apply(e.B), layer}
			if isDoor {
				at.doorInk = append(at.doorInk, s)
			} else if isWall {
				at.walls = append(at.walls, s)
			}
		case *cad.Polyline:
			if isWall && !isDoor && truthColumn(e, p.M) {
				return nil
			}
			for _, s := range polylineSegments(e) {
				g := seg2{p.M.Apply(s[0]), p.M.Apply(s[1]), layer}
				if isDoor {
					at.doorInk = append(at.doorInk, g)
				} else if isWall {
					at.walls = append(at.walls, g)
				}
			}
		}
		if isDoor {
			l := leaf{e: p.Entity, m: p.M}
			for _, s := range swings(&l, 0) {
				swingsFound = append(swingsFound, s)
			}
			if e, ok := p.Entity.(*cad.Ellipse); ok {
				if s, ok := ellipseSwing(e, p.M); ok {
					swingsFound = append(swingsFound, s)
				}
			}
		}
		return nil
	})
	var doors []swing
	for _, s := range swingsFound {
		if s.sweep < 70*math.Pi/180 || s.sweep > 110*math.Pi/180 || s.rad < 300 || s.rad > 1400 {
			continue
		}
		if slices.ContainsFunc(doors, func(d swing) bool {
			return d.hinge.Sub(s.hinge).Len() < 50 && math.Abs(d.rad-s.rad) < 50
		}) {
			continue
		}
		doors = append(doors, s)
	}
	used := make([]bool, len(doors))
	for i, a := range doors {
		if used[i] {
			continue
		}
		td := truthDoorT{hinge: a.hinge, mid: a.mid, rad: a.rad, ends: [2]point{a.e1, a.e2}}
		for j := i + 1; j < len(doors); j++ {
			b := doors[j]
			dd := a.hinge.Sub(b.hinge).Len()
			if used[j] || dd < 0.85*(a.rad+b.rad) || dd > 1.15*(a.rad+b.rad) {
				continue
			}
			// The leaves close onto each other: an arc end of each meets.
			meet := false
			for _, x := range []point{a.e1, a.e2} {
				for _, y := range []point{b.e1, b.e2} {
					meet = meet || x.Sub(y).Len() < 0.15*(a.rad+b.rad)
				}
			}
			if meet {
				td.double, td.hinge2, td.rad = true, b.hinge, a.rad+b.rad
				used[j] = true
				break
			}
		}
		if !td.double && td.rad < 450 {
			// A leaf this narrow is a door only as the small half of a
			// leaf-and-a-half pair; alone it is a hatch or a cupboard.
			continue
		}
		at.doors = append(at.doors, td)
	}
	for _, d := range at.doors {
		// The closed leaves, as ink a room boundary may run along.
		at.doorInk = append(at.doorInk, seg2{a: d.hinge, b: d.ends[0]}, seg2{a: d.hinge, b: d.ends[1]})
		if d.double {
			at.doorInk = append(at.doorInk, seg2{a: d.hinge, b: d.hinge2})
		}
	}
	return at
}

// truthColumn: a small closed outline on a wall layer is a column or a
// pier, which the recall leaves out (circles are left out by type).
func truthColumn(p *cad.Polyline, m vector.Affine) bool {
	if !p.Closed {
		return false
	}
	var box vector.Rect
	for _, v := range p.Vertices {
		box = box.Extend(m.Apply(v.P))
	}
	s := box.Size()
	return math.Max(s.X, s.Y) <= 1000
}

// ellipseSwing reads an elliptical arc as a door swing when it is nearly
// circular (a door block scaled a little unevenly, then exploded).
func ellipseSwing(e *cad.Ellipse, m vector.Affine) (swing, bool) {
	if e.Ratio < 0.8 || e.Ratio > 1.25 {
		return swing{}, false
	}
	minor := point{X: -e.Major.Y, Y: e.Major.X}.Mul(e.Ratio)
	at := func(t float64) point {
		s, c := math.Sincos(t)
		return m.Apply(e.Center.Add(e.Major.Mul(c)).Add(minor.Mul(s)))
	}
	end := ccwEnd(e.Start, e.End)
	h := m.Apply(e.Center)
	e1, e2, mid := at(e.Start), at(end), at((e.Start+end)/2)
	return swing{
		hinge: h, e1: e1, e2: e2, mid: mid,
		rad: (e1.Sub(h).Len() + e2.Sub(h).Len()) / 2, sweep: end - e.Start,
	}, true
}

func walkBox(doc *cad.Document, in *cad.Insert, m vector.Affine, box *vector.Rect) {
	b := doc.Blocks[in.Block]
	if b == nil {
		return
	}
	var visit func(ents []cad.Entity, m vector.Affine, depth int)
	visit = func(ents []cad.Entity, m vector.Affine, depth int) {
		for _, e := range ents {
			if e.Base().Invisible {
				continue
			}
			if l := doc.Layers[e.Base().Layer]; l != nil && (l.Off || l.Frozen) {
				continue
			}
			if ins, ok := e.(*cad.Insert); ok {
				if inner := doc.Blocks[ins.Block]; inner != nil && depth < 8 {
					sx, sy := ins.Scale.X, ins.Scale.Y
					if sx == 0 {
						sx = 1
					}
					if sy == 0 {
						sy = 1
					}
					mm := vector.Translate(-inner.Base.X, -inner.Base.Y).Then(vector.Scale(sx, sy)).
						Then(vector.Rotate(ins.Rotation)).Then(vector.Translate(ins.At.X, ins.At.Y))
					if ins.Mirrored {
						mm = mm.Then(vector.Scale(-1, 1))
					}
					visit(inner.Entities, mm.Then(m), depth+1)
				}
				continue
			}
			if _, ok := e.(*cad.Text); ok {
				continue
			}
			if _, ok := e.(*cad.MText); ok {
				continue
			}
			for _, p := range samples(e) {
				*box = box.Extend(m.Apply(p))
			}
		}
	}
	visit(b.Entities, m, 0)
}

// tracedWall is a plan wall back in drawing coordinates.
type tracedWall struct {
	a, b  point
	thick float64 // mm
	kind  string
	ops   []tracedOp
}

type tracedOp struct {
	a, b       point // drawing coordinates
	kind       string
	hinge      string
	swing      string
	wallFrom   [2]float64
	wallTo     [2]float64
	hingePoint point
}

func tracedWalls(tr *plan.Trace, inv vector.Affine) []tracedWall {
	def := map[string]float64{"exterior": tr.Thickness.Exterior, "interior": tr.Thickness.Interior, "glass": tr.Thickness.Glass}
	var out []tracedWall
	for _, w := range tr.Walls {
		th := def[w.Kind]
		if w.Thickness > 0 {
			th = w.Thickness
		}
		tw := tracedWall{
			a: inv.Apply(point{X: w.From[0], Y: w.From[1]}), b: inv.Apply(point{X: w.To[0], Y: w.To[1]}),
			thick: th * 1000, kind: w.Kind,
		}
		ax := 0
		if math.Abs(w.To[0]-w.From[0]) < math.Abs(w.To[1]-w.From[1]) {
			ax = 1
		}
		on := func(v float64) point {
			f := (v - w.From[ax]) / (w.To[ax] - w.From[ax])
			return point{X: w.From[0] + f*(w.To[0]-w.From[0]), Y: w.From[1] + f*(w.To[1]-w.From[1])}
		}
		for _, o := range w.Openings {
			pa, pb := on(o.A), on(o.B)
			op := tracedOp{a: inv.Apply(pa), b: inv.Apply(pb), kind: o.Kind, hinge: o.Hinge, swing: o.Swing,
				wallFrom: w.From, wallTo: w.To}
			op.hingePoint = op.a
			if o.Hinge == "b" {
				op.hingePoint = op.b
			}
			tw.ops = append(tw.ops, op)
		}
		out = append(out, tw)
	}
	return out
}

func segDist2(p, a, b point) float64 {
	d := b.Sub(a)
	l2 := dot(d, d)
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, dot(p.Sub(a), d)/l2))
	}
	return p.Sub(a.Add(d.Mul(t))).Len()
}

// wallScores samples the ink and the traced centrelines every 50 mm.
func wallScores(ink []seg2, tw []tracedWall) (recall, precision float64) {
	const step, tol = 50.0, 50.0
	g := newGrid(1000)
	for i, w := range tw {
		g.add(i, w.a, w.b)
	}
	var hit, all float64
	missBy := map[string]float64{}
	for _, s := range ink {
		l := s.b.Sub(s.a).Len()
		n := max(1, int(math.Ceil(l/step)))
		for k := range n {
			p := s.a.Add(s.b.Sub(s.a).Mul((float64(k) + 0.5) / float64(n)))
			all += l / float64(n)
			found := false
			for _, j := range g.near(p, 600) {
				if segDist2(p, tw[j].a, tw[j].b) <= tw[j].thick/2+tol {
					hit += l / float64(n)
					found = true
					break
				}
			}
			if !found {
				key := s.layer
				if l < 100 {
					key += " (<100mm)"
				}
				missBy[key] += l / float64(n)
				if k == 0 && os.Getenv("SPACEWEAVE_EVAL_MISS") == s.layer {
					fmt.Printf("MISSSEG %.0f,%.0f-%.0f,%.0f len %.0f\n", s.a.X, s.a.Y, s.b.X, s.b.Y, l)
				}
			}
		}
	}
	recall = hit / all
	if os.Getenv("SPACEWEAVE_EVAL_MISS") != "" {
		for k, v := range missBy {
			fmt.Printf("MISS %-50s %5.1f%%\n", k, 100*v/all)
		}
	}
	gi := newGrid(1000)
	for i, s := range ink {
		gi.add(i, s.a, s.b)
	}
	hit, all = 0, 0
	for _, w := range tw {
		l := w.b.Sub(w.a).Len()
		n := max(1, int(math.Ceil(l/step)))
		if os.Getenv("SPACEWEAVE_EVAL_FALSE") != "" {
			p := w.a.Add(w.b).Mul(0.5)
			if !slices.ContainsFunc(gi.near(p, 600), func(j int) bool { return segDist2(p, ink[j].a, ink[j].b) <= w.thick/2+tol }) && l > 500 {
				fmt.Printf("FALSE %.0f,%.0f-%.0f,%.0f len %.0f thick %.0f %s\n", w.a.X, w.a.Y, w.b.X, w.b.Y, l, w.thick, w.kind)
			}
		}
		for k := range n {
			p := w.a.Add(w.b.Sub(w.a).Mul((float64(k) + 0.5) / float64(n)))
			if slices.ContainsFunc(w.ops, func(o tracedOp) bool { return segDist2(p, o.a, o.b) < 1 }) {
				continue
			}
			all += l / float64(n)
			for _, j := range gi.near(p, 600) {
				if segDist2(p, ink[j].a, ink[j].b) <= w.thick/2+tol {
					hit += l / float64(n)
					break
				}
			}
		}
	}
	precision = hit / all
	return recall, precision
}

func scoreDoors(row *evalRow, doors []truthDoorT, tw []tracedWall) {
	var ops []tracedOp
	for _, w := range tw {
		for _, o := range w.ops {
			if o.kind == floorgraph.Door || o.kind == floorgraph.DoubleDoor {
				ops = append(ops, o)
			}
		}
	}
	row.doors = len(doors)
	usedOp := make([]bool, len(ops))
	matched, kindOK, singles, hingeOK, swingOK := 0, 0, 0, 0, 0
	for _, d := range doors {
		best, bestD := -1, math.Inf(1)
		for i, o := range ops {
			width := o.a.Sub(o.b).Len()
			if math.Abs(width-d.rad) > 0.25*d.rad {
				continue
			}
			dd := math.Min(o.a.Sub(d.hinge).Len(), o.b.Sub(d.hinge).Len())
			if d.double {
				dd = math.Max(math.Min(o.a.Sub(d.hinge).Len(), o.b.Sub(d.hinge).Len()),
					math.Min(o.a.Sub(d.hinge2).Len(), o.b.Sub(d.hinge2).Len()))
			}
			if dd <= 150 && dd < bestD {
				best, bestD = i, dd
			}
		}
		if best < 0 {
			continue
		}
		usedOp[best] = true
		matched++
		o := ops[best]
		if (o.kind == floorgraph.DoubleDoor) == d.double {
			kindOK++
		}
		if !d.double && o.kind == floorgraph.Door {
			singles++
			if o.hingePoint.Sub(d.hinge).Len() <= 150 {
				hingeOK++
			}
		}
		if swingSide(o, d.mid) == o.swing {
			swingOK++
		}
	}
	row.doorR = ratio(matched, len(doors))
	row.kind = ratio(kindOK, matched)
	row.hinge = ratio(hingeOK, singles)
	row.swing = ratio(swingOK, matched)
	n := 0
	for _, u := range usedOp {
		if u {
			n++
		}
	}
	row.dP = ratio(n, len(ops))
}

// swingSide is the compass side of the opening's wall that p (drawing
// coordinates) lies on, in the image, as plan.Opening.Swing reads.
func swingSide(o tracedOp, p point) string {
	// Work in drawing coordinates: image y is drawing -y.
	d := o.b.Sub(o.a)
	horizontal := math.Abs(o.wallTo[0]-o.wallFrom[0]) >= math.Abs(o.wallTo[1]-o.wallFrom[1])
	on := o.a.Add(unit(d).Mul(dot(p.Sub(o.a), unit(d))))
	if horizontal {
		if p.Y > on.Y {
			return "N"
		}
		return "S"
	}
	if p.X < on.X {
		return "W"
	}
	return "E"
}

func readSpaceAreas(doc *cad.Document) ([]truthArea, []labelAt) {
	var areas []truthArea
	var nums, sfs []labelAt
	_, _ = cad.Walk(doc, func(p *cad.Placed) error {
		layer := layerName(p)
		switch e := p.Entity.(type) {
		case *cad.Polyline:
			if strings.EqualFold(layer, "EC1 Area Polygons") {
				var ps []point
				for _, q := range polylineSamples(e) {
					ps = append(ps, p.M.Apply(q))
				}
				a := truthArea{poly: ps}
				for _, q := range ps {
					a.box = a.box.Extend(q)
				}
				areas = append(areas, a)
			}
		case *cad.Text:
			switch {
			case strings.EqualFold(layer, "EC1 Area Numbers"):
				nums = append(nums, labelAt{p.M.Apply(e.At), strings.TrimSpace(e.Content)})
			case strings.EqualFold(layer, "EC1 Area Square Footages"):
				sfs = append(sfs, labelAt{p.M.Apply(e.At), strings.TrimSpace(e.Content)})
			}
		}
		return nil
	})
	for i := range areas {
		a := &areas[i]
		for _, n := range nums {
			if insidePoly(a.poly, n.at) {
				a.number = n.text
			}
		}
		for _, s := range sfs {
			if insidePoly(a.poly, s.at) {
				fmt.Sscanf(s.text, "%g", &a.sf)
			}
		}
	}
	return areas, nums
}

func insidePoly(poly []point, p point) bool { return inside([][]point{poly}, p) }

func polyArea2(ps []point) float64 {
	a := 0.0
	for i := range ps {
		j := (i + 1) % len(ps)
		a += ps[i].X*ps[j].Y - ps[j].X*ps[i].Y
	}
	return math.Abs(a) / 2
}

// enclosedBy: at least 90% of the boundary, sampled every 100 mm, lies
// within 250 mm of ink.
func enclosedBy(poly []point, ink []seg2) bool {
	g := newGrid(1000)
	for i, s := range ink {
		g.add(i, s.a, s.b)
	}
	on, all := 0, 0
	for i := range poly {
		a, b := poly[i], poly[(i+1)%len(poly)]
		l := b.Sub(a).Len()
		n := max(1, int(math.Ceil(l/100)))
		for k := range n {
			p := a.Add(b.Sub(a).Mul((float64(k) + 0.5) / float64(n)))
			all++
			if slices.ContainsFunc(g.near(p, 300), func(j int) bool { return segDist2(p, ink[j].a, ink[j].b) <= 250 }) {
				on++
			}
		}
	}
	return all > 0 && float64(on) >= 0.9*float64(all)
}

type tracedRoom struct {
	poly []point
	name string
	box  vector.Rect
}

// iou rasterises both polygons on a 50 mm grid over their joint box.
func iou(a, b []point, ab, bb vector.Rect) float64 {
	u := ab.Union(bb)
	lo, hi := u.Min, u.Max
	var inter, union int
	for y := lo.Y + 25; y < hi.Y; y += 50 {
		for x := lo.X + 25; x < hi.X; x += 50 {
			p := point{X: x, Y: y}
			ia := boxHas(ab, p) && insidePoly(a, p)
			ib := boxHas(bb, p) && insidePoly(b, p)
			if ia && ib {
				inter++
			}
			if ia || ib {
				union++
			}
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func boxOverlap(a, b vector.Rect) bool {
	return a.Min.X <= b.Max.X && b.Min.X <= a.Max.X && a.Min.Y <= b.Max.Y && b.Min.Y <= a.Max.Y
}

func scoreRooms(row *evalRow, areas []truthArea, tr *plan.Trace, inv vector.Affine, spaceNames bool) {
	var rooms []tracedRoom
	for _, r := range tr.Rooms {
		tr := tracedRoom{name: r.Name}
		for _, q := range r.Polygon {
			p := inv.Apply(point{X: q[0], Y: q[1]})
			tr.poly = append(tr.poly, p)
			tr.box = tr.box.Extend(p)
		}
		rooms = append(rooms, tr)
	}
	row.areas, row.traced = len(areas), len(rooms)
	matchedRoom := make([]bool, len(rooms))
	m90, m75, nameOK, nameN := 0, 0, 0, 0
	mB, missing, merged, split, offset := 0, 0, 0, 0, 0
	var openArea, openCovered float64
	for _, a := range areas {
		if !a.enclosed {
			openArea += polyArea2(a.poly)
			for y := a.box.Min.Y + 100; y < a.box.Max.Y; y += 200 {
				for x := a.box.Min.X + 100; x < a.box.Max.X; x += 200 {
					p := point{X: x, Y: y}
					if !insidePoly(a.poly, p) {
						continue
					}
					if slices.ContainsFunc(rooms, func(r tracedRoom) bool { return boxHas(r.box, p) && insidePoly(r.poly, p) }) {
						openCovered += 200 * 200
					}
				}
			}
			continue
		}
		row.encl++
		best, bestI := 0.0, -1
		for i, r := range rooms {
			if !boxOverlap(r.box, a.box) {
				continue
			}
			if v := iou(a.poly, r.poly, a.box, r.box); v > best {
				best, bestI = v, i
			}
		}
		bnd := bestI >= 0 && boundaryMatch(a.poly, rooms[bestI].poly, 200)
		if bnd {
			mB++
		}
		switch {
		case best >= 0.9 || bnd:
		case bestI < 0:
			missing++
		case polyArea2(rooms[bestI].poly) > 1.3*polyArea2(a.poly):
			merged++
		case polyArea2(rooms[bestI].poly) < 0.77*polyArea2(a.poly):
			split++
		default:
			offset++
		}
		if best >= 0.9 {
			m90++
			matchedRoom[bestI] = true
		}
		if best >= 0.75 {
			m75++
			// A room nothing in the traced drawing names isn't scored.
			if len(a.names) > 0 || spaceNames {
				nameN++
			}
			if len(a.names) == 0 && !spaceNames {
			} else if nameMatches(rooms[bestI].name, a) {
				nameOK++
			} else if os.Getenv("SPACEWEAVE_EVAL_NAMES") != "" {
				fmt.Printf("NAME traced %q want %q / %q\n", rooms[bestI].name, a.names, a.number)
			}
		}
	}
	row.roomR, row.roomR75 = ratio(m90, row.encl), ratio(m75, row.encl)
	row.roomB = ratio(mB, row.encl)
	row.notes = append(row.notes, fmt.Sprintf(
		"enclosed areas not matched (IoU < 0.9 and boundary > 200 mm): %d inside a larger traced room, %d split, %d offset, %d in no room",
		merged, split, offset, missing))
	n := 0
	for _, m := range matchedRoom {
		if m {
			n++
		}
	}
	row.roomP = ratio(n, len(rooms))
	row.openCov = math.NaN()
	if openArea > 0 {
		row.openCov = openCovered / openArea
	}
	row.nameOK, row.nameN = ratio(nameOK, nameN), nameN
}

// boundaryMatch: the mean distance from each polygon's boundary, sampled
// every 100 mm, to the other's is at most tol both ways. It forgives the
// half wall between a room drawn to the wall's face (CAFM areas are) and
// one drawn to its centreline (traces are), which IoU on a small room
// does not.
func boundaryMatch(a, b []point, tol float64) bool {
	return meanDist(a, b) <= tol && meanDist(b, a) <= tol
}

func meanDist(from, to []point) float64 {
	sum, n := 0.0, 0
	for i := range from {
		p, q := from[i], from[(i+1)%len(from)]
		k := max(1, int(math.Ceil(q.Sub(p).Len()/100)))
		for j := range k {
			x := p.Add(q.Sub(p).Mul((float64(j) + 0.5) / float64(k)))
			best := math.Inf(1)
			for m := range to {
				best = math.Min(best, segDist2(x, to[m], to[(m+1)%len(to)]))
			}
			sum += best
			n++
		}
	}
	if n == 0 {
		return math.Inf(1)
	}
	return sum / float64(n)
}

func normName(s string) string {
	return strings.Join(strings.Fields(strings.ToUpper(s)), " ")
}

// nameMatches: the traced name holds the truth's name or number, or is the
// start of it (a room tag's first line, "MALE TOILET" of "MALE TOILET /
// CORE 1 / FOURTH FLOOR").
func nameMatches(traced string, a truthArea) bool {
	tn := normName(traced)
	if len(a.names) == 0 {
		return a.number != "" && strings.Contains(tn, normName(a.number))
	}
	for _, n := range a.names {
		nn := normName(n)
		if nn != "" && (strings.Contains(tn, nn) || len(tn) >= 3 && strings.HasPrefix(nn, tn)) {
			return true
		}
	}
	return false
}

// scoreFurniture scores the furniture two ways. Placement (furn): a truth
// insert is placed when an item centred in its box is its size, or when
// several are and between them cover it (a bench split into its desks).
// Types (in the notes): an item's type is plausible when its footprint, or
// the truth box it sits in, is a size that type comes in, which a desk
// typed as a chair or a cupboard typed as a desk is not; and a desk number
// should sit on a desk, never on something else. The ranges below are the
// eval's own, independent of the tracer's.
func scoreFurniture(row *evalRow, at arexTruth, tr *plan.Trace, inv vector.Affine) {
	fs := at.furniture
	row.furnN = len(fs)
	view, _ := inverse(inv)
	pxPerMM := 1 / inv.Factor()
	centre := func(it plan.Item) point { return inv.Apply(point{X: it.X, Y: it.Y}) }
	ok, placeable, okPlaceable := 0, 0, 0
	audit := map[string]*furnAudit{}
	for _, f := range fs {
		s := f.box.Size()
		long, short := math.Max(s.X, s.Y), math.Min(s.X, s.Y)
		// Hardware, panels and screens: no plan type could be them. Nor
		// could what these generic words name (the eval's own list).
		can := short >= 250 && long <= 5000 && long <= 8*short && !noPlanType.MatchString(f.name)
		var in []plan.Item
		for _, it := range tr.Furniture {
			if f.has(centre(it)) {
				in = append(in, it)
			}
		}
		hit := false
		area := 0.0
		sized := 0
		for _, it := range in {
			if it.W == nil || it.D == nil {
				hit = true // a chair, a toilet: no footprint to compare
				continue
			}
			w, d := *it.W*1000, *it.D*1000
			iw, id := math.Max(w, d), math.Min(w, d)
			// The depth may fall short of the box's: a cupboard's box
			// holds its doors' swings, its footprint doesn't.
			if math.Abs(iw-long) <= 0.3*long && id >= 0.4*short && id <= 1.3*short {
				hit = true
			}
			area += w * d
			sized++
		}
		if !hit && sized >= 2 && math.Abs(area-long*short) <= 0.3*long*short {
			hit = true
		}
		if hit {
			ok++
		}
		if can {
			placeable++
			if hit {
				okPlaceable++
			}
		}
		a := audit[f.name]
		if a == nil {
			a = &furnAudit{types: map[string]int{}, size: s}
			audit[f.name] = a
		}
		a.n++
		if hit {
			a.placed++
		}
		for _, it := range in {
			a.types[it.Type]++
		}
	}
	row.furn = ratio(ok, len(fs))

	plausible, judged := 0, 0
	byType := map[string][2]int{}
	for _, it := range tr.Furniture {
		c := centre(it)
		var box vector.Rect
		for _, f := range fs {
			if f.has(c) && (box.Empty() || f.box.Size().X*f.box.Size().Y < box.Size().X*box.Size().Y) {
				box = f.box
			}
		}
		good, known := plausibleType(it, box)
		if !known {
			continue
		}
		judged++
		bt := byType[it.Type]
		bt[1]++
		if good {
			plausible++
			bt[0]++
		}
		byType[it.Type] = bt
	}
	onDesk, onOther := 0, 0
	missed := map[string]int{}
	for _, p := range at.deskNums {
		q := view.Apply(p)
		desk, other := false, false
		for _, it := range tr.Furniture {
			if it.W == nil || it.D == nil {
				continue
			}
			rot := 0.0
			if it.Rot != nil {
				rot = *it.Rot
			}
			sn, cs := math.Sincos(rot * math.Pi / 180)
			rx, ry := q.X-it.X, q.Y-it.Y
			lx, ly := rx*cs+ry*sn, -rx*sn+ry*cs
			if math.Abs(lx) > *it.W*1000*pxPerMM/2 || math.Abs(ly) > *it.D*1000*pxPerMM/2 {
				continue
			}
			if it.Type == "desk" {
				desk = true
			} else {
				other = true
			}
		}
		if desk {
			onDesk++
			continue
		} else if other {
			onOther++
		}
		in := "no insert"
		for _, f := range fs {
			if f.has(p) {
				in = f.name
				break
			}
		}
		missed[in]++
	}
	var types []string
	for k, v := range byType {
		types = append(types, fmt.Sprintf("%s %d/%d", k, v[0], v[1]))
	}
	slices.Sort(types)
	row.notes = append(row.notes, fmt.Sprintf(
		"furniture: placed %d/%d (%s); of truth inserts a plan type could be (25 cm to 5 m, not a panel, not named as a thing with no type) %d/%d (%s); "+
			"types plausible for their size %d/%d (%s) [%s]; desk numbers on a desk %d/%d (%s), on something else %d",
		ok, len(fs), pc(row.furn), okPlaceable, placeable, pc(ratio(okPlaceable, placeable)),
		plausible, judged, pc(ratio(plausible, judged)), strings.Join(types, ", "),
		onDesk, len(at.deskNums), pc(ratio(onDesk, len(at.deskNums))), onOther))
	if os.Getenv("SPACEWEAVE_EVAL_FURN") != "" {
		names := make([]string, 0, len(audit))
		for k := range audit {
			names = append(names, k)
		}
		slices.SortFunc(names, func(a, b string) int { return audit[b].n - audit[a].n })
		row.notes = append(row.notes, fmt.Sprintf("  desk numbers off a desk, by the insert they are in: %v", missed))
		for _, k := range names {
			a := audit[k]
			row.notes = append(row.notes, fmt.Sprintf("  furn %4d placed %4d %5.2fx%-5.2f %v %s",
				a.n, a.placed, a.size.X/1000, a.size.Y/1000, a.types, k))
		}
	}
}

type furnAudit struct {
	n, placed int
	size      point
	types     map[string]int
}

// plausibleType says whether an item's type fits its size: its own
// footprint where it has one, else the truth box it sits in. known is
// false for an item outside every truth box with no footprint.
func plausibleType(it plan.Item, box vector.Rect) (good, known bool) {
	var long, short float64
	switch {
	case it.W != nil && it.D != nil:
		long, short = math.Max(*it.W, *it.D), math.Min(*it.W, *it.D)
	case it.R != nil && it.Type == "roundTable":
		long, short = 2**it.R, 2**it.R
	case it.Size != nil:
		long, short = *it.Size, *it.Size
	case !box.Empty():
		s := box.Size().Mul(0.001)
		long, short = math.Max(s.X, s.Y), math.Min(s.X, s.Y)
	default:
		return false, false
	}
	in := func(lo, hi, slo, shi float64) bool { return long >= lo && long <= hi && short >= slo && short <= shi }
	switch it.Type {
	case "desk":
		return in(0.6, 5, 0.4, 2.0), true
	case "roundTable":
		return in(0.5, 2.4, 0.5, 2.4), true
	case "chair":
		return in(0.35, 0.9, 0.3, 0.9), true
	case "armchair":
		return in(0.5, 1.3, 0.5, 1.3), true
	case "stool":
		return in(0.25, 0.65, 0.25, 0.65), true
	case "cabinet", "storage":
		return in(0.3, 6, 0.2, 1.1), true
	case "counter":
		return in(0.4, 8, 0.3, 1.2), true
	case "toilet":
		return in(0.45, 1.3, 0.3, 0.8), true
	case "basin", "sink":
		return in(0.25, 1.2, 0.2, 0.8), true
	case "plant":
		return in(0.2, 2.0, 0.1, 2.0), true
	case "sofa":
		return in(0.9, 4, 0.5, 1.6), true
	}
	return true, true
}

// evalSpace traces a CAFM space plan and scores what it can: the scale,
// against the true unit its stated square footages give.
func evalSpace(t *testing.T, floor, path string) evalRow {
	t.Helper()
	row := evalRow{name: "SPACE-" + floor}
	start := time.Now()
	doc, err := dwg.ReadFile(path)
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return row
	}
	d, err := cad.Draw(doc)
	if err != nil {
		t.Errorf("draw: %v", err)
		return row
	}
	ras, err := vector.Rasterize(d, vector.Options{})
	if err != nil {
		t.Errorf("rasterise: %v", err)
		return row
	}
	o := Options{View: ras.View(), Size: [2]int{ras.Image.Bounds().Dx(), ras.Image.Bounds().Dy()}, Title: row.name}
	if d.Unit > 0 && !d.Paper {
		o.PxPerMetre, o.Exact = ras.PxPerUnit/d.Unit, !d.Assumed
	}
	in, err := Read(doc, o)
	row.read = time.Since(start)
	areas, _ := readSpaceAreas(doc)
	// The true unit: stated square feet over polygon area in drawing units².
	var ratios []float64
	for _, a := range areas {
		if a.sf > 0 {
			ratios = append(ratios, math.Sqrt(a.sf*0.09290304/polyArea2(a.poly)))
		}
	}
	slices.Sort(ratios)
	truthPxPerM := math.NaN()
	if len(ratios) > 0 {
		truthPxPerM = ras.PxPerUnit / ratios[len(ratios)/2]
	}
	if err != nil {
		t.Errorf("read: %v", err)
		return row
	}
	start = time.Now()
	res, err := floorgraph.Build(in)
	row.build = time.Since(start)
	row.areas = len(areas)
	if err != nil {
		row.valid = err.Error()
		row.scale = in.Scale.PxPerMetre[0] / truthPxPerM
		return row
	}
	row.scale = res.Trace.Calibration.PxPerMetre[0] / truthPxPerM
	row.valid = finalize(t, res.Trace, ras)
	for i := range areas {
		areas[i].enclosed = true
	}
	inv, _ := inverse(o.View)
	scoreRooms(&row, areas, res.Trace, inv, true)
	if out := os.Getenv("SPACEWEAVE_EVAL_OUT"); out != "" {
		drawEval(t, filepath.Join(out, "space-"+row.name[len(row.name)-2:]+".png"), arexTruth{}, areas, tracedWalls(res.Trace, inv), res.Trace, inv)
	}
	row.notes = append(row.notes, fmt.Sprintf("input: %d walls, %d labels; %s", len(in.Walls), len(in.Labels), res.Summary))
	return row
}

// drawEval pictures a floor: wall ink grey, door ink green, SPACE areas
// orange (enclosed) or yellow (open), traced rooms filled light blue,
// traced walls red, traced openings magenta, true hinges green dots.
func drawEval(t *testing.T, path string, at arexTruth, areas []truthArea, tw []tracedWall, tr *plan.Trace, inv vector.Affine) {
	t.Helper()
	var box vector.Rect
	for _, s := range at.walls {
		box = box.Extend(s.a).Extend(s.b)
	}
	for _, a := range areas {
		for _, p := range a.poly {
			box = box.Extend(p)
		}
	}
	mmPx := 40.0
	if win := os.Getenv("SPACEWEAVE_EVAL_WIN"); win != "" {
		var x0, y0, x1, y1 float64
		_, _ = fmt.Sscanf(win, "%g,%g,%g,%g,%g", &x0, &y0, &x1, &y1, &mmPx)
		box = vector.Rect{}.Extend(point{X: x0, Y: y0}).Extend(point{X: x1, Y: y1})
	}
	w, h := int(box.Size().X/mmPx)+20, int(box.Size().Y/mmPx)+20
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	to := func(p point) (float64, float64) { return (p.X-box.Min.X)/mmPx + 10, float64(h) - ((p.Y-box.Min.Y)/mmPx + 10) }
	line := func(a, b point, c color.RGBA) {
		x0, y0 := to(a)
		x1, y1 := to(b)
		n := int(math.Max(math.Abs(x1-x0), math.Abs(y1-y0))) + 1
		for k := 0; k <= n; k++ {
			f := float64(k) / float64(n)
			if n < 100000 {
				img.Set(int(x0+f*(x1-x0)), int(y0+f*(y1-y0)), c)
			}
		}
	}
	for ri, r := range tr.Rooms {
		fill := color.RGBA{uint8(160 + (ri*53)%90), uint8(170 + (ri*97)%80), uint8(180 + (ri*31)%70), 255}
		var poly []point
		var rb vector.Rect
		for _, q := range r.Polygon {
			p := inv.Apply(point{X: q[0], Y: q[1]})
			poly = append(poly, p)
			rb = rb.Extend(p)
		}
		x0, y0 := math.Max(rb.Min.X, box.Min.X), math.Max(rb.Min.Y, box.Min.Y)
		x1, y1 := math.Min(rb.Max.X, box.Max.X), math.Min(rb.Max.Y, box.Max.Y)
		for y := y0; y < y1; y += mmPx {
			for x := x0; x < x1; x += mmPx {
				if insidePoly(poly, point{X: x, Y: y}) {
					px, py := to(point{X: x, Y: y})
					img.Set(int(px), int(py), fill)
				}
			}
		}
	}
	for _, s := range at.walls {
		line(s.a, s.b, color.RGBA{150, 150, 150, 255})
	}
	for _, s := range at.doorInk {
		line(s.a, s.b, color.RGBA{120, 200, 120, 255})
	}
	for _, a := range areas {
		c := color.RGBA{255, 140, 0, 255}
		if !a.enclosed {
			c = color.RGBA{230, 200, 0, 255}
		}
		for i := range a.poly {
			line(a.poly[i], a.poly[(i+1)%len(a.poly)], c)
		}
	}
	for _, w := range tw {
		line(w.a, w.b, color.RGBA{220, 0, 0, 255})
		for _, o := range w.ops {
			line(o.a, o.b, color.RGBA{230, 0, 230, 255})
		}
	}
	t.Logf("eval image: box %v, %g mm/px", box, mmPx)
	if seed := os.Getenv("SPACEWEAVE_EVAL_SEED"); seed != "" {
		var sx, sy float64
		_, _ = fmt.Sscanf(seed, "%g,%g", &sx, &sy)
		leakPath(t, img, w, h, tw, to, point{X: sx, Y: sy})
	}
	for _, d := range at.doors {
		x, y := to(d.hinge)
		for dx := -2; dx <= 2; dx++ {
			for dy := -2; dy <= 2; dy++ {
				img.Set(int(x)+dx, int(y)+dy, color.RGBA{0, 160, 0, 255})
			}
		}
	}
	f, err := os.Create(path) //nolint:gosec // the directory the caller named
	if err != nil {
		t.Log(err)
		return
	}
	defer f.Close()
	_ = png.Encode(f, img)
}


func boxHas(r vector.Rect, p point) bool {
	return !r.Empty() && p.X >= r.Min.X && p.X <= r.Max.X && p.Y >= r.Min.Y && p.Y <= r.Max.Y
}

// leakPath draws, in blue, the shortest way out of the building from seed
// between the traced walls (each as thick as traced, at least 1 px).
func leakPath(t *testing.T, img *image.RGBA, w, h int, tw []tracedWall, to func(point) (float64, float64), seed point) {
	block := make([]bool, w*h)
	for _, wl := range tw {
		x0, y0 := to(wl.a)
		x1, y1 := to(wl.b)
		fx, fy := to(wl.a.Add(point{X: wl.thick / 2}))
		rad := math.Max(0.75, math.Hypot(fx-x0, fy-y0))
		minX, maxX := int(math.Min(x0, x1)-rad-1), int(math.Max(x0, x1)+rad+1)
		minY, maxY := int(math.Min(y0, y1)-rad-1), int(math.Max(y0, y1)+rad+1)
		for y := max(0, minY); y <= min(h-1, maxY); y++ {
			for x := max(0, minX); x <= min(w-1, maxX); x++ {
				if math.Sqrt(segDist2(point{X: float64(x) + 0.5, Y: float64(y) + 0.5}, point{X: x0, Y: y0}, point{X: x1, Y: y1})) <= rad {
					block[y*w+x] = true
				}
			}
		}
	}
	sx, sy := to(seed)
	start := int(sy)*w + int(sx)
	if start < 0 || start >= len(block) || block[start] {
		t.Log("leak: seed outside the image or on a wall")
		return
	}
	prev := make([]int32, w*h)
	for i := range prev {
		prev[i] = -2
	}
	prev[start] = -1
	q := []int{start}
	end := -1
	for len(q) > 0 && end < 0 {
		c := q[0]
		q = q[1:]
		x, y := c%w, c/w
		if x == 0 || y == 0 || x == w-1 || y == h-1 {
			end = c
			break
		}
		for _, n := range []int{c - 1, c + 1, c - w, c + w} {
			if prev[n] == -2 && !block[n] {
				prev[n] = int32(c)
				q = append(q, n)
			}
		}
	}
	if end < 0 {
		t.Log("leak: none, the seed is enclosed")
		return
	}
	var pts []string
	k := 0
	for c := end; c >= 0; c = int(prev[c]) {
		img.Set(c%w, c/w, color.RGBA{0, 0, 255, 255})
		img.Set(c%w+1, c/w, color.RGBA{0, 0, 255, 255})
		if k%25 == 0 {
			pts = append(pts, fmt.Sprintf("(%d,%d)", c%w, c/w))
		}
		k++
	}
	t.Logf("leak path (image px, every 25th): %v", pts)
}
