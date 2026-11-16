package dwg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dwg/dwgtest"
)

func office2000() []byte { return dwgtest.Office("AC1015") }

func TestReadErrors(t *testing.T) {
	for name, c := range map[string]struct {
		data []byte
		want error
	}{
		"empty":      {nil, ErrNotDWG},
		"text":       {[]byte("hello, world"), ErrNotDWG},
		"AC but not": {[]byte("ACME corp"), ErrNotDWG},
		"R12":        {[]byte("AC1009" + strings.Repeat("\x00", 100)), ErrVersion},
		"R2007":      {[]byte("AC1021" + strings.Repeat("\x00", 100)), ErrVersion},
		"R2000 cut":  {office2000()[:20], ErrCorrupt},
		"R2004 cut":  {dwgtest.Office("AC1018")[:200], ErrCorrupt},
	} {
		if _, err := Read(bytes.NewReader(c.data)); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
	if _, err := Read(iotest.ErrReader(errors.New("boom"))); err == nil {
		t.Error("a failing reader read")
	}
	defer func(n int64) { maxSize = n }(maxSize)
	maxSize = 10
	if _, err := Read(bytes.NewReader(office2000())); !errors.Is(err, ErrTooLarge) {
		t.Errorf("too large: %v", err)
	}
}

func TestReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "office.dwg")
	if err := os.WriteFile(path, office2000(), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := ReadFile(path)
	if err != nil || len(doc.Model) == 0 {
		t.Fatalf("%v, %+v", err, doc)
	}
	if _, err := ReadFile(
		filepath.Join(t.TempDir(), "missing.dwg"),
	); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Errorf("missing file: %v", err)
	}
}

// r2000Locator returns the offset and size of an R2000 file's section.
func r2000Locator(data []byte, n int) (int, int) {
	rec := data[0x19+n*9:]
	return int(binary.LittleEndian.Uint32(rec[1:])), int(binary.LittleEndian.Uint32(rec[5:]))
}

func TestR2000Errors(t *testing.T) {
	good := office2000()
	damage := map[string]func(b []byte) []byte{
		"no locators":  func(b []byte) []byte { binary.LittleEndian.PutUint32(b[0x15:], 1); return b },
		"cut header":   func(b []byte) []byte { return b[:0x30] },
		"no sentinel":  func(b []byte) []byte { b[0x19+3*9+2] ^= 0xff; return b },
		"past the end": func(b []byte) []byte { binary.LittleEndian.PutUint32(b[0x19+5:], 1<<30); return b },
		"no handles":   func(b []byte) []byte { b[0x19+2*9] = 7; return b },
	}
	for name, f := range damage {
		b := f(append([]byte{}, good...))
		if _, err := Read(bytes.NewReader(b)); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestDamagedSections checks a drawing whose header, classes or object
// map is damaged still reads, with a note saying what is missing.
func TestDamagedSections(t *testing.T) {
	good := office2000()
	for name, c := range map[string]struct {
		section int
		at      int // byte to damage, from the section's start
		note    string
	}{
		"header":  {0, 0, "header couldn't be read"},
		"classes": {1, 0, "class list couldn't be read"},
		"map":     {2, 0, "object map is damaged"},
	} {
		b := append([]byte{}, good...)
		off, size := r2000Locator(b, c.section)
		if c.section == 2 {
			// A second run whose size claims more than there is.
			b = append(b[:off+size-4], 0x7f, 0xff, 0, 0)
		} else {
			b[off+c.at] ^= 0xff
		}
		doc, err := Read(bytes.NewReader(b))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !strings.Contains(strings.Join(doc.Notes, " "), c.note) {
			t.Errorf("%s: notes %q", name, doc.Notes)
		}
		if len(doc.Model) == 0 {
			t.Errorf("%s: model space is empty", name)
		}
	}

	// An object map with no entries leaves nothing to read.
	b := append([]byte{}, good...)
	off, _ := r2000Locator(b, 2)
	b[off], b[off+1] = 0, 2
	if _, err := Read(bytes.NewReader(b)); !errors.Is(err, ErrCorrupt) {
		t.Errorf("empty map: %v", err)
	}
	b[off] = 0xff
	if _, err := Read(bytes.NewReader(b)); !errors.Is(err, ErrCorrupt) {
		t.Errorf("bad map: %v", err)
	}
}

// objectAt finds an object's offset in an R2000 file.
func objectAt(t *testing.T, data []byte, handle uint64) int {
	t.Helper()
	off, size := r2000Locator(data, 2)
	entries, err := readObjectMap(data[off : off+size])
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.handle == handle {
			return int(e.offset)
		}
	}
	t.Fatalf("no object %#x", handle)
	return 0
}

// TestFindSpaces checks model and paper space are found without the block
// control object: from the header, and failing that by name.
func TestFindSpaces(t *testing.T) {
	good := office2000()
	want, err := Read(bytes.NewReader(good))
	if err != nil {
		t.Fatal(err)
	}
	b := append([]byte{}, good...)
	at := objectAt(t, b, 1)
	b[at], b[at+1] = 0, 0 // a size of 0: unreadable
	doc, st, err := decode(b)
	if err != nil || st.skipped["unreadable"] != 1 || len(doc.Model) != len(want.Model) {
		t.Fatalf("from the header: %v, %v, %d entities", err, st.skipped, len(doc.Model))
	}
	hdr, _ := r2000Locator(b, 0)
	b[hdr] ^= 0xff
	doc, err = Read(bytes.NewReader(b))
	if err != nil || len(doc.Model) != len(want.Model) || len(doc.Blocks) != len(want.Blocks) {
		t.Fatalf("by name: %v, %d entities, %d blocks", err, len(doc.Model), len(doc.Blocks))
	}
}

func TestStringStream(t *testing.T) {
	// A size with its top bit set has a high short before it.
	w := &dwgtest.Bits{}
	w.TU("hi")
	n := w.Len()
	w.RS(0)
	w.RS(uint16(n) | 0x8000) //nolint:gosec // small
	w.B(true)
	r := newBits(w.Bytes())
	s := stringStream(r, w.Len(), 0)
	if s == nil || s.TU() != "hi" || s.err != nil {
		t.Errorf("high size: %+v", s)
	}
	// A size reaching before the object.
	w = &dwgtest.Bits{}
	w.RS(100)
	w.B(true)
	s = stringStream(newBits(w.Bytes()), w.Len(), 0)
	if s == nil || !errors.Is(s.err, ErrCorrupt) {
		t.Errorf("size before the object: %+v", s)
	}
}

func TestOpenErrors(t *testing.T) {
	objs := func(f func(w *dwgtest.Bits)) *decoder {
		w := &dwgtest.Bits{}
		f(w)
		return &decoder{ver: v2000, objs: w.Bytes()}
	}
	for name, d := range map[string]*decoder{
		"no size":       objs(func(w *dwgtest.Bits) { w.RC(0x80) }),
		"zero size":     objs(func(w *dwgtest.Bits) { w.MS(0) }),
		"past the end":  objs(func(w *dwgtest.Bits) { w.MS(50) }),
		"handles after": objs(func(w *dwgtest.Bits) { w.MS(8); w.BS(1); w.RL(1000); w.RL(0) }),
	} {
		if _, err := d.open(0); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: %v", name, err)
		}
	}
	d := objs(func(w *dwgtest.Bits) { w.MS(1) })
	for _, off := range []int64{-1, 5} {
		if _, err := d.open(off); !errors.Is(err, ErrCorrupt) {
			t.Errorf("offset %d: %v", off, err)
		}
	}
	// R2010 handle stream bigger than the object.
	w := &dwgtest.Bits{}
	w.MS(2)
	w.MC(100)
	w.RS(0)
	if _, err := (&decoder{ver: v2010, objs: w.Bytes()}).open(0); !errors.Is(err, ErrCorrupt) {
		t.Errorf("R2010 handles: %v", err)
	}
	// An R2007+ object without strings reads empty ones.
	o := &obj{ver: v2010}
	if o.tv() != "" {
		t.Error("no string stream, but a string")
	}
}

func TestTrueColor(t *testing.T) {
	for v, want := range map[uint32]int{0xC0000000: cad.ByLayer, 0xC1000000: cad.ByBlock, 0xC3000005: 5} {
		if c, rgb := trueColor(v); c != want || rgb != nil {
			t.Errorf("%#x: %d %v", v, c, rgb)
		}
	}
	if c, rgb := trueColor(0xC2010203); c != cad.ByLayer || rgb == nil || rgb.B != 3 {
		t.Errorf("RGB: %d %v", c, rgb)
	}
}

func TestObjectMapErrors(t *testing.T) {
	// A run whose modular char never ends keeps what came before.
	run := []byte{0, 7, 1, 10, 0xff, 0xff, 0xff, 0, 0}
	got, err := readObjectMap(run)
	if !errors.Is(err, ErrCorrupt) || len(got) != 1 {
		t.Errorf("%v, %v", got, err)
	}
	if _, err := readObjectMap([]byte{0, 1, 0, 0}); !errors.Is(err, ErrCorrupt) {
		t.Errorf("short run: %v", err)
	}
	// No closing run: the map simply ends.
	if got, err := readObjectMap([]byte{0, 4, 1, 10, 0, 0}); err != nil || len(got) != 1 {
		t.Errorf("unclosed: %v, %v", got, err)
	}
}
