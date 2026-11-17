package dxftest

import (
	"reflect"
	"testing"
)

func TestFurnished(t *testing.T) {
	doc, n := count(t, Furnished())
	want := map[string]int{"Insert": 17, "Text": 7}
	if !reflect.DeepEqual(n, want) {
		t.Errorf("Furnished model %v, want %v", n, want)
	}
	if len(doc.Blocks) != 16 || len(doc.Blocks["A$C0ZQC"].Entities) != 41 {
		t.Errorf("%d blocks; chair %d strokes, want 41", len(doc.Blocks), len(doc.Blocks["A$C0ZQC"].Entities))
	}
	for _, l := range []string{FurnLayer, SanitaryLayer, DeskNumLayer} {
		if doc.Layers[l] == nil {
			t.Errorf("no layer %q", l)
		}
	}
}
