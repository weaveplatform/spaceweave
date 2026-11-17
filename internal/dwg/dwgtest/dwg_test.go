package dwgtest

import (
	"bytes"
	"testing"

	"github.com/weaveplatform/spaceweave/internal/cad"
)

func TestR2000(t *testing.T) {
	doc := cad.NewDocument()
	doc.Layers["BY BLOCK"] = &cad.Layer{Name: "BY BLOCK", Color: cad.ByBlock}
	doc.Layers["BY LAYER"] = &cad.Layer{Name: "BY LAYER", Color: cad.ByLayer}
	if !bytes.Equal(R2000(doc), File("AC1015", doc)) {
		t.Error("R2000 isn't an AC1015 file")
	}
	for _, v := range Versions {
		if got := File(v, doc); string(got[:6]) != v {
			t.Errorf("%s: starts %q", v, got[:6])
		}
	}
}

func TestUnknownVersion(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("an unknown version didn't panic")
		}
	}()
	File("AC1021", cad.NewDocument())
}

func TestBits(t *testing.T) {
	w := &Bits{}
	w.T("Ω") // not in Windows-1252: written as its UTF-8 bytes
	if w.Len() != 2+8+2*8 {
		t.Errorf("%d bits", w.Len())
	}
	w = &Bits{}
	w.B(true)
	w.Pad()
	if w.Len() != 8 || w.Bytes()[0] != 0x80 {
		t.Errorf("pad: %d bits, %x", w.Len(), w.Bytes())
	}
}
