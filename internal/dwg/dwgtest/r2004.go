// Package dwgtest writes DWG files for tests, so no third-party drawings
// need to live in the repository.
package dwgtest

// The R2004-family container writer follows the layout in the R2004 chapter
// of the ODA "Open Design Specification for .dwg files". It is the mirror
// of internal/dwg's reader and deliberately shares no code with it, so a
// misreading of the specification has to be made twice, the same way, to
// go unnoticed.

import (
	"encoding/binary"
	"hash/crc32"
	"maps"
	"slices"
)

// R2004 builds a file of the given version ("AC1018", "AC1024", "AC1027"
// or "AC1032") holding the named sections, in name order, compressed with
// back-references.
func R2004(version string, sections map[string][]byte) []byte {
	f := R2004File{Version: version}
	for _, name := range slices.Sorted(maps.Keys(sections)) {
		f.Sections = append(f.Sections, R2004Section{Name: name, Data: sections[name]})
	}
	return f.Bytes()
}

// R2004Section is one named section of an R2004File.
type R2004Section struct {
	Name string
	Data []byte
	// Stored writes the section uncompressed (compression type 1), as
	// AutoCAD does for small sections such as AcDb:AppInfo, padding the
	// last page with non-zero filler the way AutoCAD's leftover bytes do.
	Stored bool
	// Encrypted sets the section's encrypted flag; the data is written
	// in the clear.
	Encrypted bool
}

// R2004File describes an R2004-family file. The zero value of each field
// is the AutoCAD default.
type R2004File struct {
	Version     string
	Maintenance byte
	Codepage    uint16 // 0 writes 30, ANSI_1252
	Sections    []R2004Section

	// PageSize is the decompressed size of a section page; 0 means
	// AutoCAD's 0x7400.
	PageSize int
	// Literal compresses with literal runs only, which every decoder must
	// handle, instead of with back-references.
	Literal bool
	// SkipZeroPages leaves out pages that are entirely zero, as AutoCAD
	// does; the reader must zero-fill the holes.
	SkipZeroPages bool
	// Gap puts a freed (negative id) entry in the page map ahead of the
	// system pages.
	Gap bool

	// The hooks below change a structure before its checksum is taken, so
	// tests can build files that are wrong in one specific way yet pass
	// every integrity check. Each receives the plain (unencrypted,
	// uncompressed) bytes.

	// Header sees the 0x6C-byte R2004 header.
	Header func(h []byte)
	// PageMap and SectionMap see the decompressed system pages and return
	// their replacements.
	PageMap    func(m []byte) []byte
	SectionMap func(m []byte) []byte
	// DataPage sees each data page's 32-byte header; page counts from 0 in
	// file order.
	DataPage func(page int, h []byte)
}

const (
	firstPage        = 0x100
	pageTypePageMap  = 0x41630E3B
	pageTypeSecMap   = 0x4163003B
	pageTypeData     = 0x4163043B
	dataPageMask     = 0x4164536B
	defaultPageSize  = 0x7400
	defaultCodepage  = 30
	pageAlign        = 0x20
	sectionNameSize  = 64
	storedPadding    = 0xA5
	gapSize          = 0x40
	r2004HeaderSize  = 0x6C
	r2004HeaderStart = 0x80
)

type pageEntry struct {
	id   int32
	size int
}

// Bytes encodes the file.
func (f R2004File) Bytes() []byte {
	pageSize := f.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	compress := Compress
	if f.Literal {
		compress = CompressLiteral
	}
	out := make([]byte, firstPage)
	var pages []pageEntry
	nextID := int32(1)
	dataPage := 0

	// Section ids follow AutoCAD: the unnamed section is 0 and the named
	// ones count down from the total to 1.
	smap := le32(nil, u32(len(f.Sections)+1))
	smap = le32(smap, 2)
	smap = le32(smap, defaultPageSize)
	smap = le32(smap, 0)
	smap = le32(smap, u32(len(f.Sections)+1))
	smap = descriptor(smap, R2004Section{}, 0, pageSize, 0, nil)
	for i, s := range f.Sections {
		id := u32(len(f.Sections) - i)
		var entries []byte
		var written uint32
		for start := 0; start < len(s.Data); start += pageSize {
			chunk := make([]byte, pageSize)
			copy(chunk, s.Data[start:])
			var body []byte
			if s.Stored {
				for k := min(pageSize, len(s.Data)-start); k < pageSize; k++ {
					chunk[k] = storedPadding
				}
				body = chunk
			} else {
				if f.SkipZeroPages && allZero(chunk) {
					continue
				}
				body = compress(chunk)
			}
			off := len(out)
			out = f.writeDataPage(out, dataPage, id, body, u64(start))
			dataPage++
			pages = append(pages, pageEntry{id: nextID, size: len(out) - off})
			entries = le32(entries, u32(int(nextID)))
			entries = le32(entries, u32(len(body)))
			entries = le64(entries, u64(start))
			nextID++
			written++
		}
		smap = descriptor(smap, s, id, pageSize, written, entries)
	}

	if f.Gap {
		out = append(out, make([]byte, gapSize)...)
		pages = append(pages, pageEntry{id: -1, size: gapSize})
	}

	if f.SectionMap != nil {
		smap = f.SectionMap(smap)
	}
	sectionMapID := nextID
	off := len(out)
	out = systemPage(out, pageTypeSecMap, smap, compress)
	pages = append(pages, pageEntry{id: sectionMapID, size: len(out) - off})
	nextID++

	// The page map lists itself, and its own size depends on how well it
	// compresses; it is the last page, so iterate until the size settles.
	pageMapID := nextID
	pageMapAddr := len(out)
	var pmap []byte
	size := 0
	for range 8 {
		pmap = pageMap(append(pages, pageEntry{id: pageMapID, size: size}))
		if f.PageMap != nil {
			pmap = f.PageMap(pmap)
		}
		n := len(systemPage(nil, pageTypePageMap, pmap, compress))
		if n == size {
			break
		}
		size = n
	}
	out = systemPage(out, pageTypePageMap, pmap, compress)

	f.writeHeader(out, pageMapID, pageMapAddr, sectionMapID, len(pages)+1)
	return out
}

func pageMap(pages []pageEntry) []byte {
	var m []byte
	for _, p := range pages {
		m = le32(m, u32(int(p.id)))
		m = le32(m, u32(p.size))
		if p.id < 0 {
			// Parent, left, right and a zero: the free-space tree links.
			m = append(m, make([]byte, 16)...)
		}
	}
	return m
}

// descriptor appends s's description to the section map m.
func descriptor(m []byte, s R2004Section, id uint32, pageSize int, npages uint32,
	entries []byte,
) []byte {
	m = le64(m, u64(len(s.Data)))
	m = le32(m, npages)
	m = le32(m, u32(pageSize))
	m = le32(m, 1)
	method := uint32(2)
	if s.Stored {
		method = 1
	}
	m = le32(m, method)
	m = le32(m, id)
	encrypted := uint32(0)
	if s.Encrypted {
		encrypted = 1
	}
	m = le32(m, encrypted)
	var name [sectionNameSize]byte
	copy(name[:], s.Name)
	m = append(m, name[:]...)
	return append(m, entries...)
}

func (f R2004File) writeDataPage(out []byte, n int, section uint32, body []byte,
	start uint64,
) []byte {
	off := len(out)
	var h [32]byte
	binary.LittleEndian.PutUint32(h[0x00:], pageTypeData)
	binary.LittleEndian.PutUint32(h[0x04:], section)
	binary.LittleEndian.PutUint32(h[0x08:], u32(len(body)))
	binary.LittleEndian.PutUint32(h[0x0C:], u32(len(body)))
	binary.LittleEndian.PutUint64(h[0x10:], start)
	if f.DataPage != nil {
		f.DataPage(n, h[:])
	}
	dsum := Checksum(0, body)
	binary.LittleEndian.PutUint32(h[0x18:], 0)
	binary.LittleEndian.PutUint32(h[0x1C:], dsum)
	binary.LittleEndian.PutUint32(h[0x18:], Checksum(dsum, h[:]))
	mask := uint32(dataPageMask) ^ u32(off)
	for k := 0; k < len(h); k += 4 {
		binary.LittleEndian.PutUint32(h[k:], binary.LittleEndian.Uint32(h[k:])^mask)
	}
	out = append(out, h[:]...)
	out = append(out, body...)
	return pad(out, off)
}

func systemPage(out []byte, typ uint32, data []byte, compress func([]byte) []byte) []byte {
	off := len(out)
	body := compress(data)
	var h [20]byte
	binary.LittleEndian.PutUint32(h[0:], typ)
	binary.LittleEndian.PutUint32(h[4:], u32(len(data)))
	binary.LittleEndian.PutUint32(h[8:], u32(len(body)))
	binary.LittleEndian.PutUint32(h[12:], 2)
	binary.LittleEndian.PutUint32(h[16:], Checksum(Checksum(0, h[:]), body))
	out = append(out, h[:]...)
	out = append(out, body...)
	return pad(out, off)
}

// pad aligns a page that began at off to 0x20 bytes, as AutoCAD does.
func pad(out []byte, off int) []byte {
	for (len(out)-off)%pageAlign != 0 {
		out = append(out, 0)
	}
	return out
}

func (f R2004File) writeHeader(out []byte, pageMapID int32, pageMapAddr int,
	sectionMapID int32, npages int,
) {
	copy(out, f.Version)
	out[0x0B] = f.Maintenance
	out[0x0C] = 3
	out[0x11] = 33 // the writing application's version
	out[0x12] = f.Maintenance
	codepage := f.Codepage
	if codepage == 0 {
		codepage = defaultCodepage
	}
	binary.LittleEndian.PutUint16(out[0x13:], codepage)
	binary.LittleEndian.PutUint32(out[0x28:], 0x80)

	h := make([]byte, r2004HeaderSize)
	copy(h, "AcFssFcAJMB\x00")
	binary.LittleEndian.PutUint32(h[0x10:], r2004HeaderSize)
	binary.LittleEndian.PutUint32(h[0x14:], 4)
	binary.LittleEndian.PutUint32(h[0x24:], 1)
	binary.LittleEndian.PutUint32(h[0x28:], u32(int(pageMapID)))
	binary.LittleEndian.PutUint64(h[0x2C:], u64(len(out)-firstPage))
	binary.LittleEndian.PutUint32(h[0x40:], u32(npages))
	binary.LittleEndian.PutUint32(h[0x44:], 0x20)
	binary.LittleEndian.PutUint32(h[0x48:], 0x80)
	binary.LittleEndian.PutUint32(h[0x4C:], 0x40)
	binary.LittleEndian.PutUint32(h[0x50:], u32(int(pageMapID)))
	binary.LittleEndian.PutUint64(h[0x54:], u64(pageMapAddr-firstPage))
	binary.LittleEndian.PutUint32(h[0x5C:], u32(int(sectionMapID)))
	binary.LittleEndian.PutUint32(h[0x60:], u32(npages))
	if f.Header != nil {
		f.Header(h)
	}
	binary.LittleEndian.PutUint32(h[0x68:], 0)
	binary.LittleEndian.PutUint32(h[0x68:], crc32.ChecksumIEEE(h))
	Scramble(h)
	copy(out[r2004HeaderStart:], h)
}

// Scramble XORs b with the R2004 header's pseudo-random sequence; applying
// it twice restores b.
func Scramble(b []byte) {
	seed := uint32(1)
	for i := range b {
		seed = seed*0x343FD + 0x269EC3
		b[i] ^= byte(seed >> 16 & 0xFF)
	}
}

// Checksum is the R2004 page checksum.
func Checksum(seed uint32, b []byte) uint32 {
	sum1, sum2 := seed&0xFFFF, seed>>16
	for len(b) > 0 {
		n := min(len(b), 0x15B0)
		for _, c := range b[:n] {
			sum1 += uint32(c)
			sum2 += sum1
		}
		sum1 %= 0xFFF1
		sum2 %= 0xFFF1
		b = b[n:]
	}
	return sum2<<16 | sum1&0xFFFF
}

// CompressLiteral encodes b as a single literal run: valid, never smaller.
func CompressLiteral(b []byte) []byte {
	out := literals(nil, len(b), true)
	out = append(out, b...)
	return append(out, lzTerminator)
}

// Compress encodes b with the R2004 LZ77 variant, using every match form
// the format has (short, medium and the far 0x10–0x1F offsets) so a
// decoder is exercised on all of them. It is greedy and makes no attempt
// to compress well.
func Compress(b []byte) []byte {
	var out []byte
	var table [1 << 14]int32 // hash of 3 bytes → position + 1
	pending, pendingOff, pendingLen := false, 0, 0
	lit := 0 // start of the literals not yet written
	emit := func(end int) {
		if pending {
			out = match(out, pendingOff, pendingLen, end-lit)
		} else {
			out = literals(out, end-lit, true)
		}
		out = append(out, b[lit:end]...)
	}
	for i := 0; i+3 <= len(b); {
		h := hash3(b[i:])
		cand := int(table[h]) - 1
		table[h] = int32(i + 1)
		if cand < 0 {
			i++
			continue
		}
		off, n := i-cand, 0
		for i+n < len(b) && b[cand+n] == b[i+n] {
			n++
		}
		// Up to 0x4000 back a match may be 3 bytes; beyond, the short
		// 0x10 form for 3 bytes would be the 0x11 terminator.
		if (off > 0x4000 || n < 3) && (off > 0xBFFF || n < 4) {
			i++
			continue
		}
		emit(i)
		pending, pendingOff, pendingLen = true, off, n
		for k := i + 1; k < i+n && k+3 <= len(b); k++ {
			table[hash3(b[k:])] = int32(k + 1)
		}
		i += n
		lit = i
	}
	if pending || len(b) > 0 {
		emit(len(b))
	}
	return append(out, lzTerminator)
}

const lzTerminator = 0x11

func hash3(b []byte) uint32 {
	return (uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16) * 2654435761 >> 18
}

// literals writes the count for a run of n literal bytes. At the start of
// a stream a run of 1–3 uses the leading form (0x12–0x14), which the
// specification omits but internal/dwg and ACadSharp accept; after a match
// runs of 1–3 ride in the match's low bits and aren't written here.
func literals(out []byte, n int, first bool) []byte {
	switch {
	case n == 0:
		return out
	case n < 4 && first:
		return append(out, lo8(n+17))
	case n < 4:
		return out
	case n-3 <= 0x0F:
		return append(out, byte(n-3))
	default:
		return longCount(append(out, 0), n-3-0x0F)
	}
}

// longCount writes the tail of a long length: a 0x00 per 0xFF, then the
// non-zero remainder.
func longCount(out []byte, r int) []byte {
	for ; r > 0xFF; r -= 0xFF {
		out = append(out, 0)
	}
	return append(out, lo8(r))
}

// match writes a back-reference of n bytes from off bytes back, followed
// by the count of the lit literal bytes that come after it.
func match(out []byte, off, n, lit int) []byte {
	bits := byte(0)
	if lit < 4 {
		bits = lo8(lit)
	}
	switch {
	case off <= 0x400 && n <= 14:
		out = append(out, lo8((n+1)<<4|(off-1)&3<<2)|bits, lo8((off-1)>>2))
	case off <= 0x4000:
		if n-2 <= 0x1F {
			out = append(out, lo8(0x20|(n-2)))
		} else {
			out = longCount(append(out, 0x20), n-2-0x1F)
		}
		out = append(out, lo8((off-1)&0x3F<<2)|bits, lo8((off-1)>>6))
	default:
		o := off - 0x4000
		hi := lo8(o >> 14 & 1 << 3)
		// n ≥ 4 here, so the short form never collides with the 0x11
		// terminator.
		if n-2 <= 7 {
			out = append(out, 0x10|hi|lo8(n-2))
		} else {
			out = longCount(append(out, 0x10|hi), n-2-7)
		}
		lo := o & 0x3FFF
		out = append(out, lo8(lo&0x3F<<2)|bits, lo8(lo>>6))
	}
	return literals(out, lit, false)
}

// u32 and u64 convert sizes and offsets, which in a test file are small
// and non-negative; u32 keeps the low 32 bits, so gap ids wrap negative.
func u32(v int) uint32 { return uint32(v & 0xFFFFFFFF) }

func u64(v int) uint64 { return uint64(v) } //nolint:gosec // never negative here

// lo8 keeps the low byte; every caller has already range-limited v to it.
func lo8(v int) byte { return byte(v & 0xFF) }

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

func le32(b []byte, v uint32) []byte { return binary.LittleEndian.AppendUint32(b, v) }
func le64(b []byte, v uint64) []byte { return binary.LittleEndian.AppendUint64(b, v) }
