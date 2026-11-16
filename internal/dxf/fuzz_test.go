package dxf

import (
	"bytes"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
)

func FuzzRead(f *testing.F) {
	plan := dxftest.Plan()
	ascii, bin := plan.Bytes(), plan.Binary()
	seeds := [][]byte{
		ascii,
		bin,
		ascii[:len(ascii)/2],
		bin[:len(bin)/2],
		bin[:len(BinarySentinel)+3],
		dxftest.New(4).Bytes(),
		dxftest.New(-1).Binary(),
		dxftest.New(4).Add(dxftest.E("HATCH", 91, 1, 92, 1, 93, 1,
			72, 4, 94, 3, 73, 1, 74, 0, 95, 8, 96, 4,
			40, 0.0, 40, 0.0, 40, 0.0, 40, 0.0, 40, 1.0, 40, 1.0, 40, 1.0, 40, 1.0,
			10, 0.0, 20, 0.0, 42, 1.0, 10, 1.0, 20, 2.0, 42, 2.0,
			10, 3.0, 20, 2.0, 42, 1.0, 10, 4.0, 20, 0.0, 42, 1.0, 97, 0)).Bytes(),
		[]byte(
			"0\nSECTION\n2\nENTITIES\n0\nPOLYLINE\n66\n1\n0\nVERTEX\n10\n1\n0\nSEQEND\n0\nENDSEC\n",
		),
		[]byte(""),
		[]byte("999\n"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Read(bytes.NewReader(data))
		if (d == nil) == (err == nil) {
			t.Fatalf("Read = %v, %v: want exactly one of a document and an error", d, err)
		}
		if err != nil {
			return
		}
		// Every document keeps layer 0 and has maps a caller can index.
		if d.Layers["0"] == nil || d.Blocks == nil {
			t.Fatalf("document lost its defaults: %+v", d)
		}
		// A nil entity, typed or not, would panic in every consumer.
		for _, space := range [][]cad.Entity{d.Model, d.Paper} {
			for _, e := range space {
				if e == nil || reflect.ValueOf(e).IsNil() {
					t.Fatal("nil entity in the document")
				}
			}
		}
	})
}

// TestFuzzSeedsErrors keeps the fuzz seeds honest: the truncated ones fail
// the way a truncated file should.
func TestFuzzSeedsErrors(t *testing.T) {
	bin := dxftest.Plan().Binary()
	_, err := Read(bytes.NewReader(bin[:len(BinarySentinel)+3]))
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("binary cut inside the first value: %v, want ErrTruncated", err)
	}
}

// FuzzASCIIMatchesBinary checks the two encodings agree on arbitrary text
// and coordinates, the values a real drawing varies most.
func FuzzASCIIMatchesBinary(f *testing.F) {
	f.Add("WALLS", "Room 1", 0.0, 1.5)
	f.Add("", "", -1e300, 1e-300)
	f.Add("Ebene\xb0", "  padded  ", math.NaN(), math.Inf(1))
	f.Fuzz(func(t *testing.T, layer, content string, x, y float64) {
		// A line break or NUL can't be written in an ASCII or binary value
		// respectively; such a file is malformed, not a disagreement.
		if strings.ContainsAny(layer+content, "\r\n\x00") {
			t.Skip()
		}
		d := dxftest.New(4).Add(
			dxftest.E("TEXT", 8, layer, 10, x, 20, y, 1, content),
			dxftest.E("LINE", 8, layer, 10, x, 20, y, 11, y, 21, x),
		)
		readBoth(t, d)
	})
}
