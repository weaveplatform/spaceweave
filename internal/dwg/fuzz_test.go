package dwg

import (
	"bytes"
	"reflect"
	"runtime"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dwg/dwgtest"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
)

// FuzzRead feeds the reader damaged drawings: it must return a document or
// an error, never panic, and allocate in proportion to its input.
//
// The seeds are tens of kilobytes, so run it with -fuzzminimizetime 1s or
// so: minimizing a new input from one at the default takes minutes, during
// which the fuzzer reports no executions.
func FuzzRead(f *testing.F) {
	small := dwgtest.FromDXF(dxftest.Plan())
	for _, v := range dwgtest.Versions {
		f.Add(dwgtest.File(v, small))
		f.Add(dwgtest.Drawing{Version: v, Doc: sink(v), Extras: true}.Bytes())
	}
	f.Add(office2000()[:600])
	f.Add([]byte("AC1015"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		d, err := Read(bytes.NewReader(data))
		runtime.ReadMemStats(&after)
		// Every count is checked against the bits left to hold it, so a
		// read allocates in step with its input, whatever the input says.
		if n := after.TotalAlloc - before.TotalAlloc; n > 64<<20+uint64(len(data))*200 {
			t.Errorf("read of %d bytes allocated %d", len(data), n)
		}
		if (d == nil) == (err == nil) {
			t.Fatalf("Read = %v, %v: want exactly one of a document and an error", d, err)
		}
		if err != nil {
			return
		}
		if d.Layers["0"] == nil || d.Blocks == nil {
			t.Fatalf("document lost its defaults: %+v", d)
		}
		check := func(es []cad.Entity) {
			for _, e := range es {
				if e == nil || reflect.ValueOf(e).IsNil() {
					t.Fatal("nil entity in the document")
				}
				if in, ok := e.(*cad.Insert); ok {
					for _, a := range in.Attribs {
						if a == nil {
							t.Fatal("nil attribute")
						}
					}
				}
			}
		}
		check(d.Model)
		check(d.Paper)
		for _, b := range d.Blocks {
			if b == nil {
				t.Fatal("nil block")
			}
			check(b.Entities)
		}
	})
}
