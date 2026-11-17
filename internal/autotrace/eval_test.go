package autotrace

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/autotrace/score"
	"github.com/weaveplatform/spaceweave/internal/autotrace/synth"
	"github.com/weaveplatform/spaceweave/internal/plan"
	"github.com/weaveplatform/spaceweave/internal/raster"
	"github.com/weaveplatform/spaceweave/internal/trace"
)

// evalPlans are the hand-traced plans the accuracy table is drawn from.
var evalPlans = []string{"office", "clinic", "suites", "corporate"}

// variants are the ways a published plan is drawn. Every one keeps the room
// names and furniture on, as published plans do.
var variants = []struct {
	name string
	o    synth.Options
}{
	{"clean", synth.Options{}},
	{"scan", synth.Options{AntiAlias: true, Noise: 24, Seed: 7, JPEG: 60}},
	{"thin", synth.Options{AntiAlias: true, WallScale: 0.7}},
	{"bold", synth.Options{AntiAlias: true, WallScale: 1.5}},
	{"outline", synth.Options{AntiAlias: true, Walls: synth.Outline}},
	{"hatch", synth.Options{AntiAlias: true, Walls: synth.Hatch}},
	{"outline-scan", synth.Options{AntiAlias: true, Walls: synth.Outline, Noise: 24, Seed: 7, JPEG: 60}},
}

// floor is the least a row of the table may score: wall recall and
// precision, openings matched by kind and position, rooms covered, and the
// most the scale may be off by on either axis.
type floor struct {
	recall, precision float64
	openings, rooms   int
	scale             float64
}

// floors sit a little under what this version measures (2 points on the
// wall ratios, 5% of the openings and rooms, 2 points of scale), so a
// change that loses accuracy fails while one that moves a pixel doesn't.
var floors = map[[2]string]floor{
	{"office", "clean"}:           {0.98, 0.94, 20, 12, 0.037},
	{"office", "scan"}:            {0.98, 0.91, 20, 12, 0.037},
	{"office", "thin"}:            {0.74, 0.88, 10, 5, 0.037},
	{"office", "bold"}:            {0.98, 0.94, 18, 11, 0.045},
	{"office", "outline"}:         {0.98, 0.86, 10, 12, 0.122},
	{"office", "hatch"}:           {0.98, 0.87, 10, 12, 0.122},
	{"office", "outline-scan"}:    {0.98, 0.89, 10, 12, 0.105},
	{"clinic", "clean"}:           {0.98, 0.95, 50, 34, 0.038},
	{"clinic", "scan"}:            {0.98, 0.94, 50, 34, 0.038},
	{"clinic", "thin"}:            {0.98, 0.93, 49, 34, 0.038},
	{"clinic", "bold"}:            {0.97, 0.97, 50, 34, 0.038},
	{"clinic", "outline"}:         {0.98, 0.80, 33, 33, 0.038},
	{"clinic", "hatch"}:           {0.98, 0.81, 33, 33, 0.038},
	{"clinic", "outline-scan"}:    {0.98, 0.81, 33, 33, 0.038},
	{"suites", "clean"}:           {0.95, 0.84, 46, 30, 0.028},
	{"suites", "scan"}:            {0.95, 0.84, 45, 30, 0.024},
	{"suites", "thin"}:            {0.96, 0.87, 46, 30, 0.024},
	{"suites", "bold"}:            {0.75, 0.66, 37, 32, 0.024},
	{"suites", "outline"}:         {0.78, 0.48, 20, 16, 0.024},
	{"suites", "hatch"}:           {0.94, 0.83, 42, 27, 0.024},
	{"suites", "outline-scan"}:    {0.78, 0.45, 20, 16, 0.024},
	{"corporate", "clean"}:        {0.82, 0.83, 69, 22, 0.130},
	{"corporate", "scan"}:         {0.76, 0.72, 66, 23, 0.117},
	{"corporate", "thin"}:         {0.71, 0.78, 51, 8, 0.095},
	{"corporate", "bold"}:         {0.86, 0.68, 38, 14, 0.077},
	{"corporate", "outline"}:      {0.69, 0.49, 18, 11, 0.077},
	{"corporate", "hatch"}:        {0.83, 0.70, 32, 36, 0.130},
	{"corporate", "outline-scan"}: {0.70, 0.51, 20, 3, 0.077},
}

// measured is one row of the accuracy table.
type measured struct {
	plan, variant     string
	recall, precision float64
	ops               score.OpeningMatch // summed over kinds
	doors, singles    int                // matched doors (single and double), single doors
	rooms             score.AreaMatch
	scaleErr          float64 // the worse axis, as a fraction
	confidence        float64
	err               error
}

func evaluate(t *testing.T, name string, o synth.Options) measured {
	t.Helper()
	want := readTrace(t, name)
	o.Text, o.Furniture = true, true
	src := raster.NewSource(synth.Render(want, o))
	w, h := src.Size()
	job := trace.Job{
		Image: src,
		Title: name,
		Meta:  plan.Meta{URL: "generated/x/source.png", Credit: "test", SizePx: [2]int{w, h}},
	}
	m := measured{plan: name}
	res, err := Run(context.Background(), trace.Options{Validator: verdict{}}, job)
	if err != nil {
		m.err = err
		return m
	}
	m.recall, m.precision = score.Walls(res.Trace, want, 3)
	for kind, k := range score.Openings(res.Trace, want, 8) {
		m.ops.Matched += k.Matched
		m.ops.Want += k.Want
		m.ops.Got += k.Got
		m.ops.Located += k.Located
		m.ops.Swung += k.Swung
		m.ops.Hinged += k.Hinged
		switch kind {
		case "door":
			m.doors += k.Matched
			m.singles += k.Matched
		case "double-door":
			m.doors += k.Matched
		}
	}
	m.rooms = score.RoomAreas(res.Trace, want)
	for i := range 2 {
		e := res.Trace.Calibration.PxPerMetre[i]/want.Calibration.PxPerMetre[i] - 1
		if math.Abs(e) > math.Abs(m.scaleErr) {
			m.scaleErr = e
		}
	}
	m.confidence = res.Confidence
	return m
}

func (m measured) check(t *testing.T, f floor) {
	t.Helper()
	if m.err != nil {
		t.Fatalf("trace: %v", m.err)
	}
	if m.recall < f.recall || m.precision < f.precision {
		t.Errorf("walls %.3f/%.3f, floor %.2f/%.2f", m.recall, m.precision, f.recall, f.precision)
	}
	if m.ops.Matched < f.openings {
		t.Errorf("openings %d matched, floor %d", m.ops.Matched, f.openings)
	}
	if m.rooms.Covered < f.rooms {
		t.Errorf("rooms %d covered, floor %d", m.rooms.Covered, f.rooms)
	}
	if math.Abs(m.scaleErr) > f.scale {
		t.Errorf("scale off by %+.1f%%, ceiling %.1f%%", 100*m.scaleErr, 100*f.scale)
	}
}

func (m measured) row() string {
	if m.err != nil {
		return fmt.Sprintf("%-10s %-13s failed: %v", m.plan, m.variant, m.err)
	}
	return fmt.Sprintf("%-10s %-13s %4.2f %4.2f  %3d/%-3d %3d/%-3d %3d/%-3d %3d/%-3d  %3d %3d/%-3d %3d/%-3d %4.2f %4.2f %2d/%-3d %+5.1f%%  %.2f",
		m.plan, m.variant, m.recall, m.precision,
		m.ops.Matched, m.ops.Want, m.ops.Located, m.ops.Want, m.ops.Swung, m.doors, m.ops.Hinged, m.singles,
		m.rooms.Got, m.rooms.Exact, m.rooms.Want, m.rooms.Covered, m.rooms.Want,
		m.rooms.AreaRecall, m.rooms.AreaPrecision, m.rooms.Named, m.rooms.Want,
		100*m.scaleErr, m.confidence)
}

const tableHead = `plan       variant       walls      openings                          rooms                                     scale   conf
                         rec  prec  kind+pos found   swing   hinge   got exact   covered area rec/prec named
`

// tableKey explains the columns of the table.
const tableKey = `
walls: share of the hand trace's wall length within 3 px of a traced wall (rec), and of the traced length within 3 px of the hand trace's (prec).
openings: kind+pos = hand-traced openings with a traced one of the same kind within 8 px on a parallel wall; found = with one of any kind;
  swing = matched doors and double doors opening to the same side, of those matched; hinge = matched single doors hung from the same end.
rooms: got = rooms traced; exact = hand-traced rooms with a traced room at IoU >= 0.9; covered = exact plus rooms matched as a group
  (one traced room = several hand-traced ones where the hand trace split an open area with no wall, or the reverse) at IoU >= 0.9;
  area rec/prec = share of the hand trace's floor inside traced rooms / of the traced floor inside hand-traced rooms; named = exact matches with the same name.
scale: traced px/m against the hand trace's, the worse axis.
`

// TestAccuracy scores the image tracer against the hand traces the test
// images were drawn from, in every drawing style, and logs the table
// `make eval` prints. The floors sit a little under what this version
// reaches, so a change that loses accuracy fails.
//
// Under the race detector the whole table takes over a minute, so there
// every plan is read clean and every style is read on the office alone,
// which keeps each code path under the detector; the full table and its
// floors run without it (`make eval`, or go test without -race).
func TestAccuracy(t *testing.T) {
	type key struct{ plan, variant string }
	rows := make(map[key]measured)
	var mu sync.Mutex
	t.Run("plans", func(t *testing.T) {
		for _, name := range evalPlans {
			for _, v := range variants {
				t.Run(name+"/"+v.name, func(t *testing.T) {
					if raceDetector && name != "office" && v.name != "clean" {
						t.Skip("the full table runs without the race detector")
					}
					t.Parallel()
					m := evaluate(t, name, v.o)
					m.variant = v.name
					m.check(t, floors[[2]string{name, v.name}])
					mu.Lock()
					rows[key{name, v.name}] = m
					mu.Unlock()
				})
			}
		}
	})
	var b strings.Builder
	b.WriteString(tableHead)
	for _, name := range evalPlans {
		for _, v := range variants {
			if m, ok := rows[key{name, v.name}]; ok {
				b.WriteString(m.row() + "\n")
			}
		}
	}
	b.WriteString(tableKey)
	t.Logf("synthetic renders of the hand traces:\n%s", b.String())
}
