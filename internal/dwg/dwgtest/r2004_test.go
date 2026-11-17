package dwgtest

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

// The encoders' output is checked byte for byte against hand-assembled
// streams here; internal/dwg round-trips them through the real decoder.
func TestCompressGolden(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []byte
	}{
		{"empty", "", []byte{0x11}},
		{"leading literals", "abcabc", []byte{0x14, 'a', 'b', 'c', 0x48, 0x00, 0x11}},
		{"near match", "abcdabcd", []byte{0x01, 'a', 'b', 'c', 'd', 0x5C, 0x00, 0x11}},
		{
			"two literals after", "abcdabcdxy",
			[]byte{0x01, 'a', 'b', 'c', 'd', 0x5E, 0x00, 'x', 'y', 0x11},
		},
		{
			"five literals after", "abcdabcdvwxyz",
			[]byte{0x01, 'a', 'b', 'c', 'd', 0x5C, 0x00, 0x02, 'v', 'w', 'x', 'y', 'z', 0x11},
		},
		{"overlapping run", "aaaaaaaaaaaaaaaaaaaa", []byte{0x12, 'a', 0x31, 0x00, 0x00, 0x11}},
	}
	for _, tt := range tests {
		if got := Compress([]byte(tt.in)); !bytes.Equal(got, tt.want) {
			t.Errorf("Compress(%s) = % x, want % x", tt.name, got, tt.want)
		}
	}
	lit := map[string][]byte{
		"":     {0x11},
		"a":    {0x12, 'a', 0x11},
		"abcd": {0x01, 'a', 'b', 'c', 'd', 0x11},
	}
	for in, want := range lit {
		if got := CompressLiteral([]byte(in)); !bytes.Equal(got, want) {
			t.Errorf("CompressLiteral(%q) = % x, want % x", in, got, want)
		}
	}
	// 0x0F+3 literals is the most the short count holds; one more needs
	// the long form, and 0xFF more needs a zero byte in it.
	heads := map[int][]byte{18: {0x0F}, 19: {0x00, 0x01}, 19 + 0xFF: {0x00, 0x00, 0x01}}
	for n, head := range heads {
		got := CompressLiteral(make([]byte, n))
		if !bytes.HasPrefix(got, head) || len(got) != len(head)+n+1 {
			t.Errorf("CompressLiteral(%d bytes) starts % x, want % x", n, got[:len(head)], head)
		}
	}
}

func TestMatchForms(t *testing.T) {
	tests := []struct {
		off, n, lit int
		want        []byte
	}{
		{0x400, 14, 1, []byte{0xFD, 0xFF}},
		{0x401, 3, 0, []byte{0x21, 0x00, 0x10}},
		{0x4000, 40, 3, []byte{0x20, 0x07, 0xFF, 0xFF}},
		{0x4001, 4, 2, []byte{0x12, 0x06, 0x00}},
		{0xBFFF, 9, 0, []byte{0x1F, 0xFC, 0xFF}},
		{0xBFFF, 10, 0, []byte{0x18, 0x01, 0xFC, 0xFF}},
		{0x400, 3, 4, []byte{0x4C, 0xFF, 0x01}},
	}
	for _, tt := range tests {
		if got := match(nil, tt.off, tt.n, tt.lit); !bytes.Equal(got, tt.want) {
			t.Errorf("match(%#x, %d, %d) = % x, want % x", tt.off, tt.n, tt.lit, got, tt.want)
		}
	}
}

func TestR2004Structure(t *testing.T) {
	var hooks []string
	f := R2004File{
		Version:       "AC1018",
		Maintenance:   6,
		Codepage:      1252,
		PageSize:      32,
		SkipZeroPages: true,
		Gap:           true,
		Sections: []R2004Section{
			{Name: "A", Data: append(make([]byte, 40), 1)},
			{Name: "B", Data: []byte("stored"), Stored: true, Encrypted: true},
		},
		Header:     func([]byte) { hooks = append(hooks, "header") },
		PageMap:    func(m []byte) []byte { hooks = append(hooks, "page map"); return m },
		SectionMap: func(m []byte) []byte { hooks = append(hooks, "section map"); return m },
		DataPage:   func(int, []byte) { hooks = append(hooks, "data page") },
	}
	data := f.Bytes()
	// The page map is compressed twice: once to learn its size, then for
	// real.
	if got := len(hooks); got < 6 {
		t.Errorf("hooks ran %v", hooks)
	}
	codepage := binary.LittleEndian.Uint16(data[0x13:])
	if string(data[:6]) != "AC1018" || data[0x0B] != 6 || codepage != 1252 {
		t.Errorf("plain header % x", data[:0x20])
	}
	h := bytes.Clone(data[r2004HeaderStart : r2004HeaderStart+r2004HeaderSize])
	Scramble(h)
	if string(h[:12]) != "AcFssFcAJMB\x00" {
		t.Errorf("header id %q", h[:12])
	}
	sum := binary.LittleEndian.Uint32(h[0x68:])
	binary.LittleEndian.PutUint32(h[0x68:], 0)
	if crc32.ChecksumIEEE(h) != sum {
		t.Error("header CRC is wrong")
	}
	addr := binary.LittleEndian.Uint64(h[0x54:]) + firstPage
	if typ := binary.LittleEndian.Uint32(data[addr:]); typ != pageTypePageMap {
		t.Errorf("page map type %#x", typ)
	}
	if (len(data)-firstPage)%pageAlign != 0 {
		t.Errorf("pages aren't 0x20-aligned: %d bytes", len(data))
	}
	// A's first page is all zeros and skipped; its second and B's one
	// stored page are written, then the gap and the two system pages.
	if got := binary.LittleEndian.Uint32(h[0x40:]); got != 5 {
		t.Errorf("page count %d, want 5", got)
	}
}

func TestR2004Sorted(t *testing.T) {
	a := R2004("AC1032", map[string][]byte{"x": []byte("1"), "y": []byte("2"), "z": nil})
	b := R2004("AC1032", map[string][]byte{"z": nil, "y": []byte("2"), "x": []byte("1")})
	if !bytes.Equal(a, b) {
		t.Error("R2004 depends on map order")
	}
}
