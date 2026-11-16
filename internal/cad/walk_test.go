package cad

import (
	"errors"
	"math"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/vector"
)

func TestWalkSkipBlockAndParents(t *testing.T) {
	door := &Insert{Common: Common{Layer: "doors"}, Block: "D", At: Point{X: 10}, Columns: 2, ColumnGap: 5}
	door.Attribs = []*Text{{Common: Common{Layer: "0"}, Content: "D1", Height: 1}}
	doc := metricDoc(door, &Insert{Block: "W", At: Point{Y: 10}}, line(0, 0, 1, 0))
	doc.Layers["doors"] = &Layer{Name: "doors", Color: 2}
	doc.Blocks["D"] = &Block{Name: "D", Base: Point{X: 1}, Entities: []Entity{
		&Arc{Common: Common{Layer: "0", Mirrored: true}, Radius: 1, End: math.Pi / 2},
		line(0, 0, 1, 0),
	}}
	doc.Blocks["W"] = &Block{Name: "W", Entities: []Entity{line(0, 0, 2, 0)}}

	var cells, arcs, lines, attribs int
	_, err := Walk(doc, func(p *Placed) error {
		switch e := p.Entity.(type) {
		case *Insert:
			if e.Block == "W" {
				return SkipBlock
			}
			// The block's base point lands on the insertion point.
			if got := p.M.Apply(Point{X: 1}); !nearPt(got, Point{X: 10 + 5*float64(cells)}) {
				t.Errorf("cell %d: base point at %v", cells, got)
			}
			cells++
			return nil
		case *Arc:
			arcs++
			if p.Layer == nil || p.Layer.Name != "doors" || p.Parent == nil || p.Parent.Entity != door {
				t.Errorf("arc layer %v parent %v, want the insert's", p.Layer, p.Parent)
			}
			// Mirrored in object coordinates: its start lands left of the centre.
			if got := p.M.Apply(Point{X: 1}); !nearPt(got, Point{X: 8 + 5*float64(arcs-1)}) {
				t.Errorf("mirrored arc start at %v", got)
			}
		case *Line:
			lines++
		case *Text:
			attribs++
			if p.Owner != door || p.Parent != nil {
				t.Errorf("attribute owner %v parent %v", p.Owner, p.Parent)
			}
		}
		return SkipBlock // ignored for anything but a block reference
	})
	if err != nil {
		t.Fatal(err)
	}
	if cells != 2 || arcs != 2 || lines != 3 || attribs != 1 {
		t.Errorf("visited %d cells, %d arcs, %d lines, %d attributes; want 2, 2, 3, 1",
			cells, arcs, lines, attribs)
	}
}

var errStop = errors.New("stop")

func TestWalkStopsOnError(t *testing.T) {
	doc := metricDoc(&Insert{Block: "B"}, line(0, 0, 1, 0))
	doc.Blocks["B"] = &Block{Name: "B", Entities: []Entity{line(0, 0, 1, 0)}}
	n := 0
	stop := func(kind string) func(*Placed) error {
		return func(p *Placed) error {
			n++
			if _, ok := p.Entity.(*Insert); ok == (kind == "insert") {
				return errStop
			}
			return nil
		}
	}
	if _, err := Walk(doc, stop("insert")); !errors.Is(err, errStop) || n != 1 {
		t.Errorf("insert: err %v after %d visits", err, n)
	}
	n = 0
	if _, err := Walk(doc, stop("line")); !errors.Is(err, errStop) || n != 2 {
		t.Errorf("line: err %v after %d visits", err, n)
	}
	if _, err := Draw(doc); err != nil {
		t.Errorf("Draw: %v", err)
	}
}

func TestWalkVisitCap(t *testing.T) {
	defer func(n int) { maxVisits = n }(maxVisits)
	maxVisits = 10
	// Empty blocks draw nothing, so only the walk's own cap stops this.
	doc := metricDoc(&Insert{Block: "B", Columns: 100})
	doc.Blocks["B"] = &Block{Name: "B"}
	if _, err := Walk(doc, func(*Placed) error { return nil }); !errors.Is(err, ErrTooBig) {
		t.Errorf("err %v, want ErrTooBig", err)
	}
}

func TestWalkPaperAndStats(t *testing.T) {
	doc := NewDocument()
	doc.Paper = []Entity{line(0, 0, 1, 0)}
	w, err := Walk(doc, func(*Placed) error { return nil })
	if err != nil || !w.Paper {
		t.Errorf("Walk = %+v, %v; want paper", w, err)
	}
}

func TestTextLines(t *testing.T) {
	if got := TextLines(line(0, 0, 1, 1), vector.Identity); got != nil {
		t.Errorf("TextLines(line) = %v", got)
	}
	if got := TextLines(&Text{Content: " ", Height: 1}, vector.Identity); got != nil {
		t.Errorf("TextLines(blank) = %v", got)
	}
	got := TextLines(&MText{Content: `A\P\PB`, Height: 1}, vector.Translate(5, 0))
	if len(got) != 2 || got[0].Content != "A" || got[1].Content != "B" || !nearPt(got[0].At, Point{X: 5}) {
		t.Errorf("TextLines(mtext) = %+v", got)
	}
}
