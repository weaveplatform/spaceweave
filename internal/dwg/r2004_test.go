package dwg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/weaveplatform/spaceweave/internal/dwg/dwgtest"
)

// TestReadR2004RealFiles runs over real drawings when SPACEWEAVE_DWG_DIR
// names a directory of them. They aren't committed: customer drawings
// can't live in the repository, and synthetic files can't prove the reader
// agrees with AutoCAD.
func TestReadR2004RealFiles(t *testing.T) {
	dir := os.Getenv("SPACEWEAVE_DWG_DIR")
	if dir == "" {
		t.Skip("set SPACEWEAVE_DWG_DIR to a directory of R2004+ .dwg files")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.dwg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no .dwg files in %s", dir)
	}
	// The start sentinels of the header and classes sections; finding them
	// proves the pages were decompressed and placed right, not merely that
	// the container parsed.
	sentinels := map[string][]byte{
		SectionHeader: {
			0xCF, 0x7B, 0x1F, 0x23, 0xFD, 0xDE, 0x38, 0xA9,
			0x5F, 0x7C, 0x68, 0xB8, 0x4E, 0x6D, 0x33, 0x5F,
		},
		SectionClasses: {
			0x8D, 0xA1, 0xC4, 0xB8, 0xC4, 0xA9, 0xF8, 0xC5,
			0xC0, 0xDC, 0xF4, 0x5F, 0xE7, 0xCF, 0xB6, 0x8A,
		},
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path) //nolint:gosec // the runner chose the directory
			if err != nil {
				t.Fatal(err)
			}
			if v := string(data[:min(6, len(data))]); v == "AC1015" || v == "AC1021" {
				t.Skipf("%s is not an R2004-family file", v)
			}
			began := time.Now()
			c, err := readR2004(data)
			elapsed := time.Since(began)
			if err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			for _, name := range slices.Sorted(maps.Keys(c.Sections)) {
				b.WriteString("\n  " + name + ": " + strconv.Itoa(len(c.Sections[name])))
			}
			t.Logf("%s maintenance %d codepage %d, %d bytes read in %v:%s",
				c.Version, c.Maintenance, c.Codepage, len(data), elapsed, b.String())
			for _, name := range []string{
				SectionHeader, SectionClasses, SectionHandles, SectionObjects,
			} {
				if len(c.Sections[name]) == 0 {
					t.Errorf("section %s is missing or empty", name)
				}
			}
			for name, sentinel := range sentinels {
				if !bytes.HasPrefix(c.Sections[name], sentinel) {
					t.Errorf("%s doesn't start with its sentinel", name)
				}
			}
		})
	}
}

// r2004Sample is a file's worth of sections, with enough repetition to
// compress.
func r2004Sample() map[string][]byte {
	return map[string][]byte{
		SectionHeader:  []byte("header variables"),
		SectionClasses: bytes.Repeat([]byte{1, 2, 3, 4, 5}, 50),
		SectionHandles: {0x00, 0x02, 0x01, 0x00},
		SectionObjects: bytes.Repeat([]byte("object data \x00\x01\x02 "), 300),
		"AcDb:Empty":   {},
	}
}

func TestReadR2004RoundTrip(t *testing.T) {
	want := r2004Sample()
	for _, v := range []string{"AC1018", "AC1024", "AC1027", "AC1032"} {
		c, err := readR2004(dwgtest.R2004(v, want))
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		if c.Version != v || c.Codepage != 30 || c.Maintenance != 0 {
			t.Errorf("%s: read version %q codepage %d maintenance %d",
				v, c.Version, c.Codepage, c.Maintenance)
		}
		if !maps.EqualFunc(c.Sections, want, bytes.Equal) {
			t.Errorf("%s: sections differ", v)
		}
	}
}

func TestReadR2004Layouts(t *testing.T) {
	objects := make([]byte, 64*7+10)
	copy(objects[64:], "the first page is zero, this one is not")
	copy(objects[64*4:], "pages two and three are zero; five and six are too")
	stored := bytes.Repeat([]byte("stored "), 30) // not a multiple of the page size
	want := map[string][]byte{
		SectionHeader:  []byte("header"),
		SectionObjects: objects,
		"AcDb:AppInfo": stored,
	}
	tests := map[string]dwgtest.R2004File{
		"literal":         {Literal: true},
		"small pages":     {PageSize: 64},
		"zero pages":      {PageSize: 64, SkipZeroPages: true},
		"gap":             {Gap: true},
		"stored":          {PageSize: 40},
		"maintenance":     {Maintenance: 105, Codepage: 1252},
		"literal & small": {Literal: true, PageSize: 32},
	}
	for name, f := range tests {
		f.Version = "AC1032"
		f.Sections = []dwgtest.R2004Section{
			{Name: SectionHeader, Data: want[SectionHeader]},
			{Name: SectionObjects, Data: objects},
			{Name: "AcDb:AppInfo", Data: stored, Stored: true},
		}
		c, err := readR2004(f.Bytes())
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !maps.EqualFunc(c.Sections, want, bytes.Equal) {
			t.Errorf("%s: sections differ", name)
		}
		if c.Maintenance != f.Maintenance || (f.Codepage != 0 && c.Codepage != f.Codepage) {
			t.Errorf("%s: maintenance %d codepage %d", name, c.Maintenance, c.Codepage)
		}
	}
}

func TestReadR2004Version(t *testing.T) {
	data := dwgtest.R2004("AC1032", r2004Sample())
	for _, v := range []string{"AC1021", "AC1015", "AC1014", "NOTDWG"} {
		bad := bytes.Clone(data)
		copy(bad, v)
		if _, err := readR2004(bad); !errors.Is(err, ErrVersion) {
			t.Errorf("%s: err=%v, want ErrVersion", v, err)
		}
	}
}

// Offsets into a one-section file's decompressed section map: the
// preamble, the unnamed section's description, then ours and its single
// page entry.
const (
	r2004DescOff      = r2004MapPreamble + r2004DescSize
	r2004DescPagesOff = r2004DescOff + r2004DescSize
)

// r2004SetDesc returns a SectionMap hook that writes v at off.
func r2004SetDesc(off int, v uint32) func([]byte) []byte {
	return func(m []byte) []byte {
		binary.LittleEndian.PutUint32(m[off:], v)
		return m
	}
}

// r2004PageMapAt finds the page map through the file's R2004 header.
func r2004PageMapAt(data []byte) int {
	h := bytes.Clone(data[r2004HeaderOffset : r2004HeaderOffset+r2004HeaderSize])
	dwgtest.Scramble(h)
	return int(binary.LittleEndian.Uint32(h[0x54:])) + r2004FirstPage
}

// r2004ResealPageMap edits the page map's header and compressed bytes,
// then fixes its checksum so only the edit is wrong.
func r2004ResealPageMap(data []byte, edit func(h, body []byte)) []byte {
	off := r2004PageMapAt(data)
	h := data[off : off+r2004SystemHeader]
	body := data[off+r2004SystemHeader:][:binary.LittleEndian.Uint32(h[8:])]
	edit(h, body)
	binary.LittleEndian.PutUint32(h[16:], 0)
	binary.LittleEndian.PutUint32(h[16:], dwgtest.Checksum(dwgtest.Checksum(0, h), body))
	return data
}

func TestReadR2004Corrupt(t *testing.T) {
	one := func() dwgtest.R2004File {
		return dwgtest.R2004File{Version: "AC1032", Sections: []dwgtest.R2004Section{
			{Name: SectionHeader, Data: []byte("header variables to compress: aaaaaaaaaaaa")},
		}}
	}
	with := func(edit func(f *dwgtest.R2004File)) []byte {
		f := one()
		edit(&f)
		return f.Bytes()
	}
	after := func(edit func(data []byte) []byte) []byte { return edit(one().Bytes()) }
	header := func(edit func(h []byte)) []byte {
		return with(func(f *dwgtest.R2004File) { f.Header = edit })
	}
	pageMap := func(edit func(m []byte) []byte) []byte {
		return with(func(f *dwgtest.R2004File) { f.PageMap = edit })
	}
	sectionMap := func(edit func(m []byte) []byte) []byte {
		return with(func(f *dwgtest.R2004File) { f.SectionMap = edit })
	}
	dataPage := func(edit func(h []byte)) []byte {
		return with(func(f *dwgtest.R2004File) { f.DataPage = func(_ int, h []byte) { edit(h) } })
	}
	resealed := func(edit func(h, body []byte)) []byte {
		return after(func(d []byte) []byte { return r2004ResealPageMap(d, edit) })
	}
	flip := func(at func(d []byte) int) []byte {
		return after(func(d []byte) []byte { d[at(d)] ^= 0x40; return d })
	}
	cut := func(at func(d []byte) int) []byte {
		return after(func(d []byte) []byte { return d[:at(d)] })
	}
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"short", cut(func([]byte) int { return 0x80 }), "too short"},
		{"header id", header(func(h []byte) { h[0] = 'X' }), "header id"},
		{"header CRC", flip(func([]byte) int { return r2004HeaderOffset + 0x30 }), "header CRC"},
		{
			"page map address",
			header(func(h []byte) { binary.LittleEndian.PutUint64(h[0x54:], 1<<40) }),
			"page map address",
		},
		{
			"page map header cut",
			cut(func(d []byte) int { return r2004PageMapAt(d) + 10 }),
			"page map is past the end",
		},
		{
			"page map body cut",
			cut(func(d []byte) int { return r2004PageMapAt(d) + r2004SystemHeader + 2 }),
			"page map runs past",
		},
		{"page map type", resealed(func(h, _ []byte) { h[0] ^= 1 }), "page map page has type"},
		{
			"page map checksum",
			flip(func(d []byte) int { return r2004PageMapAt(d) + r2004SystemHeader }),
			"page map checksum",
		},
		{"page map method", resealed(func(h, _ []byte) { h[12] = 1 }), "compression type 1"},
		{
			"page map expansion",
			resealed(func(h, _ []byte) { binary.LittleEndian.PutUint32(h[4:], 1<<30) }),
			"impossible expansion",
		},
		{"page map size", resealed(func(h, _ []byte) { h[4]++ }), "page map decompresses to"},
		{
			"page map stream",
			resealed(func(_, body []byte) { body[0] = 0x10 }),
			"page map: dwg: file is damaged: compressed",
		},
		{
			"page map partial entry",
			pageMap(func(m []byte) []byte { return append(m, 1, 2, 3) }),
			"partial entry",
		},
		{
			"page map partial gap",
			pageMap(func(m []byte) []byte {
				return append(m, 0xFF, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0, 1, 2, 3)
			}),
			"partial gap",
		},
		{
			"page map duplicate",
			pageMap(func(m []byte) []byte { return append(m, m[:8]...) }),
			"mapped twice",
		},
		{
			// Page 1 claims the rest of the address space, so every later
			// page, the section map among them, is lost.
			"page past the end",
			pageMap(func(m []byte) []byte {
				binary.LittleEndian.PutUint32(m[4:], 1<<31)
				return m
			}),
			"section map page 2 is missing",
		},
		{
			"section map id",
			header(func(h []byte) { binary.LittleEndian.PutUint32(h[0x5C:], 999) }),
			"section map page 999 is missing",
		},
		{
			// Point the section map id at the page map.
			"section map type",
			header(func(h []byte) { copy(h[0x5C:0x60], h[0x50:0x54]) }),
			"section map page has type",
		},
		{
			"section map short",
			sectionMap(func(m []byte) []byte { return m[:10] }),
			"section map is too short",
		},
		{"section count", sectionMap(r2004SetDesc(0, 1000)), "lists 1000 sections"},
		{
			// Three descriptions' worth of bytes, but the second's page
			// entry eats into the third.
			"section map cut",
			sectionMap(func(m []byte) []byte {
				binary.LittleEndian.PutUint32(m, 3)
				return append(m, make([]byte, r2004DescSize-r2004PageEntry)...)
			}),
			"section map is truncated",
		},
		{
			"page count",
			sectionMap(r2004SetDesc(r2004DescOff+8, 1000)),
			"more pages than the map holds",
		},
		{
			"duplicate section",
			with(func(f *dwgtest.R2004File) { f.Sections = append(f.Sections, f.Sections[0]) }),
			"appears twice",
		},
		{
			"encrypted",
			with(func(f *dwgtest.R2004File) { f.Sections[0].Encrypted = true }),
			"encrypted DWG files aren't supported",
		},
		{"compression type", sectionMap(r2004SetDesc(r2004DescOff+20, 3)), "compression type 3"},
		{
			"section size",
			sectionMap(func(m []byte) []byte {
				binary.LittleEndian.PutUint64(m[r2004DescOff:], 1<<40)
				return m
			}),
			"needs more memory",
		},
		{"page size", sectionMap(r2004SetDesc(r2004DescOff+12, 0xFFFFFFFF)), "needs more memory"},
		{"missing page", sectionMap(r2004SetDesc(r2004DescPagesOff, 999)), "missing page 999"},
		{
			// Shrink page 1 to 16 bytes and account for the rest as a gap,
			// so every later page stays where it is.
			"page too small",
			pageMap(func(m []byte) []byte {
				size := binary.LittleEndian.Uint32(m[4:])
				binary.LittleEndian.PutUint32(m[4:], 16)
				gap := binary.LittleEndian.AppendUint32(nil, 0xFFFFFFFF)
				gap = binary.LittleEndian.AppendUint32(gap, size-16)
				gap = append(gap, make([]byte, 16)...)
				return slices.Concat(m[:8], gap, m[8:])
			}),
			"too small for its header",
		},
		{"data page type", dataPage(func(h []byte) { h[0] ^= 1 }), "page 1 has type"},
		{"data page section", dataPage(func(h []byte) { h[4] = 9 }), "belongs to section 9"},
		{"data page size", dataPage(func(h []byte) { h[8]++ }), "disagrees with the section"},
		{"data page start", dataPage(func(h []byte) { h[0x10] = 1 }), "disagrees with the section"},
		{
			"data page overrun",
			with(func(f *dwgtest.R2004File) {
				f.DataPage = func(_ int, h []byte) { binary.LittleEndian.PutUint32(h[8:], 1<<20) }
				f.SectionMap = r2004SetDesc(r2004DescPagesOff+4, 1<<20)
			}),
			"overruns the page",
		},
		{
			"data page past section",
			with(func(f *dwgtest.R2004File) {
				f.DataPage = func(_ int, h []byte) { h[0x10] = 0xFF }
				f.SectionMap = r2004SetDesc(r2004DescPagesOff+8, 0xFF)
			}),
			"starts past the end",
		},
		{
			"data checksum",
			flip(func([]byte) int { return r2004FirstPage + r2004DataHeader }),
			"data checksum",
		},
		{
			// The page size field is checked by nothing but the checksum.
			"data page header checksum",
			flip(func([]byte) int { return r2004FirstPage + 0x0C }),
			"header checksum",
		},
		{
			"stored page size",
			with(func(f *dwgtest.R2004File) {
				f.Sections[0].Stored = true
				f.SectionMap = r2004SetDesc(r2004DescOff+12, 8)
			}),
			"larger than section",
		},
		{
			"data page stream",
			sectionMap(r2004SetDesc(r2004DescOff+12, 16)),
			"page 1: dwg: file is damaged: compressed data expands",
		},
	}
	for _, tt := range tests {
		c, err := readR2004(tt.data)
		if c != nil || !errors.Is(err, ErrCorrupt) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err=%v, want ErrCorrupt containing %q", tt.name, err, tt.want)
		}
	}
}

func TestR2004Checksum(t *testing.T) {
	// Long enough to cross the 0x15B0-byte modulus boundary several times.
	b := bytes.Repeat([]byte{0xFF, 0x80, 0x01}, 0x15B0*2)
	for _, seed := range []uint32{0, 1, 0xFFF0FFF0} {
		if got, want := r2004Checksum(seed, b), dwgtest.Checksum(seed, b); got != want {
			t.Errorf("seed %#x: %#08x, want %#08x", seed, got, want)
		}
	}
}

func FuzzReadR2004(f *testing.F) {
	seeds := [][]byte{
		dwgtest.R2004("AC1032", map[string][]byte{
			SectionHeader:  []byte("hdr"),
			SectionObjects: bytes.Repeat([]byte("ab"), 40),
		}),
		dwgtest.R2004File{
			Version: "AC1018", Literal: true, PageSize: 32, Gap: true,
			Sections: []dwgtest.R2004Section{
				{Name: SectionHeader, Data: []byte("header")},
				{Name: "AcDb:AppInfo", Data: []byte("stored data"), Stored: true},
			},
		}.Bytes(),
		dwgtest.R2004File{
			Version: "AC1024", PageSize: 16, SkipZeroPages: true,
			Sections: []dwgtest.R2004Section{
				{Name: SectionObjects, Data: append(make([]byte, 40), 1)},
			},
		}.Bytes(),
	}
	for _, s := range seeds {
		f.Add(s)
		f.Add(s[:len(s)/2])
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		c, err := readR2004(data)
		if (c == nil) == (err == nil) {
			t.Fatalf("readR2004 = %v, %v: want exactly one of a container and an error", c, err)
		}
		if err != nil {
			if !errors.Is(err, ErrCorrupt) && !errors.Is(err, ErrVersion) {
				t.Fatalf("error %v wraps neither ErrCorrupt nor ErrVersion", err)
			}
			return
		}
		total := 0
		for _, s := range c.Sections {
			total += len(s)
		}
		if uint64(total) > lzMaxRatio*uint64(len(data))+lzSlack {
			t.Fatalf("%d bytes became %d", len(data), total)
		}
	})
}
