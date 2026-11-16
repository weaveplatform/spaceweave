package cadfront

import (
	"slices"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
	"github.com/weaveplatform/spaceweave/internal/floorgraph"
	"github.com/weaveplatform/spaceweave/internal/plan"
)

// TestBuildOffice runs the office through the shared core: the evidence
// must be enough for a trace that passes plan.Check, with the drawing's
// room names.
func TestBuildOffice(t *testing.T) {
	in, _ := read(t, dxftest.Office(4))
	in.Title = "Office"
	res, err := floorgraph.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if problems := plan.Check(res.Trace); len(problems) > 0 {
		t.Errorf("plan.Check: %q", problems)
	}
	var names []string
	for _, r := range res.Trace.Rooms {
		names = append(names, r.Name)
	}
	for name := range dxftest.OfficeRooms {
		if !slices.Contains(names, name) {
			t.Errorf("rooms %q, want %q among them", names, name)
		}
	}
	t.Logf("confidence %.2f, %s; %d walls, %d rooms %q, %d furniture; caveats %q",
		res.Confidence, res.Summary, len(res.Trace.Walls), len(res.Trace.Rooms), names,
		len(res.Trace.Furniture), res.Caveats)
}

func TestBuildUnlayered(t *testing.T) {
	in, _ := read(t, dxftest.Unlayered(4))
	in.Title = "Two rooms"
	res, err := floorgraph.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if problems := plan.Check(res.Trace); len(problems) > 0 {
		t.Errorf("plan.Check: %q", problems)
	}
	var names []string
	for _, r := range res.Trace.Rooms {
		names = append(names, r.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"East", "West"}) {
		t.Errorf("rooms %q", names)
	}
	t.Logf("confidence %.2f, %s; caveats %q", res.Confidence, res.Summary, res.Caveats)
}
