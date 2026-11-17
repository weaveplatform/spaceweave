package dwg

// The paged container shared by R2004 (AC1018), R2010 (AC1024), R2013
// (AC1027) and R2018 (AC1032), from the R2004 chapter of the ODA
// specification. Where the specification is vague (the gap entries in the
// page map, the zero-page rules) ACadSharp's DwgReader (MIT,
// github.com/DomCR/ACadSharp) was consulted for how real files behave.
//
// A file is a 0x100-byte header followed by pages. The page map lists every
// page's id and size in file order; the section map names each logical
// section and lists the pages, by id, that make it up. Each data page is
// LZ77-compressed separately and placed at its own offset in the section.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const (
	r2004HeaderOffset = 0x80
	r2004HeaderSize   = 0x6C
	r2004FileID       = "AcFssFcAJMB\x00"
	// Page-map offsets count from here: the file header occupies the
	// first 0x100 bytes and isn't itself a page.
	r2004FirstPage = 0x100

	r2004PageMap    = 0x41630E3B // system page types
	r2004SectionMap = 0x4163003B
	r2004DataPage   = 0x4163043B
	r2004DataMask   = 0x4164536B

	r2004SystemHeader = 20 // bytes in a system page's header
	r2004DataHeader   = 32 // bytes in a data page's header
	r2004MapPreamble  = 20 // section map bytes before the first description
	r2004DescSize     = 0x60
	r2004NameSize     = 64
	r2004PageEntry    = 16 // a description's per-page entry

	r2004Stored     = 1 // section compression types
	r2004Compressed = 2

	// r2004MaxBytes caps what one file may decompress to. The largest real
	// drawings reach tens of MiB; anything past this is hostile.
	r2004MaxBytes = 512 << 20
	// lzMaxRatio bounds how far LZ77 can expand: the long length form adds
	// at most 0xFF output bytes per input byte, so a page claiming more
	// than this many times its compressed size is lying.
	lzMaxRatio = 256
	lzSlack    = 1 << 10
)

// readR2004 opens an R2004-family container and returns its sections,
// decompressed and reassembled. It verifies the header's CRC-32 and the
// checksums of every system and data page: a mismatch means the bytes
// about to be decompressed are not the ones AutoCAD wrote, and decoding
// them would yield plausible-looking garbage rather than an error.
func readR2004(data []byte) (*container, error) {
	if len(data) < r2004FirstPage {
		return nil, r2004Errorf("file is too short for a DWG header")
	}
	version := string(data[:6])
	switch version {
	case "AC1018", "AC1024", "AC1027", "AC1032":
	default:
		return nil, fmt.Errorf("%w: %q is not an R2004-family file", ErrVersion, version)
	}
	// Allocation follows from what the file could plausibly decompress to,
	// so a small hostile file can't demand hundreds of MiB.
	r := &r2004Reader{
		data:   data,
		budget: min(r2004MaxBytes, lzMaxRatio*uint64(len(data))+lzSlack),
	}
	h, err := decryptR2004Header(data)
	if err != nil {
		return nil, err
	}
	if err := r.readPageMap(h); err != nil {
		return nil, err
	}
	descs, err := r.readSectionMap(h)
	if err != nil {
		return nil, err
	}
	c := &container{
		Version:     version,
		Maintenance: data[0x0B],
		Codepage:    binary.LittleEndian.Uint16(data[0x13:]),
		Sections:    make(map[string][]byte, len(descs)),
	}
	for i := range descs {
		d := &descs[i]
		if d.name == "" {
			// The unnamed section 0 has no pages and no meaning.
			continue
		}
		if _, dup := c.Sections[d.name]; dup {
			return nil, r2004Errorf("section %q appears twice", d.name)
		}
		buf, err := r.readSection(d)
		if err != nil {
			return nil, err
		}
		c.Sections[d.name] = buf
	}
	return c, nil
}

// r2004Errorf reports damage, wrapping ErrCorrupt.
func r2004Errorf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrCorrupt, fmt.Sprintf(format, args...))
}

// r2004ID reads a signed little-endian long; page ids are signed so that
// negative ones can mark gaps.
func r2004ID(b []byte) int32 {
	return int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16 | int32(b[3])<<24
}

// r2004Header is the part of the encrypted header the reader needs.
type r2004Header struct {
	pageMapAddr  uint64 // absolute
	sectionMapID int32
}

func decryptR2004Header(data []byte) (r2004Header, error) {
	var b [r2004HeaderSize]byte
	copy(b[:], data[r2004HeaderOffset:])
	r2004Scramble(b[:])
	if string(b[:len(r2004FileID)]) != r2004FileID {
		return r2004Header{}, r2004Errorf("R2004 header id is wrong")
	}
	// The CRC covers all 0x6C bytes with its own field zeroed.
	want := binary.LittleEndian.Uint32(b[0x68:])
	binary.LittleEndian.PutUint32(b[0x68:], 0)
	if got := crc32.ChecksumIEEE(b[:]); got != want {
		return r2004Header{}, r2004Errorf("R2004 header CRC is %#08x, want %#08x", got, want)
	}
	addr := binary.LittleEndian.Uint64(b[0x54:])
	if addr > uint64(len(data)) {
		return r2004Header{}, r2004Errorf("page map address is past the end of the file")
	}
	return r2004Header{
		pageMapAddr:  addr + r2004FirstPage,
		sectionMapID: r2004ID(b[0x5C:]),
	}, nil
}

// r2004Scramble XORs b with the header's pseudo-random sequence (the
// Microsoft C rand() recurrence seeded with 1). It is its own inverse.
func r2004Scramble(b []byte) {
	seed := uint32(1)
	for i := range b {
		seed = seed*0x343FD + 0x269EC3
		b[i] ^= byte(seed >> 16 & 0xFF)
	}
}

// r2004Checksum is the specification's page checksum, an Adler-32
// relative.
func r2004Checksum(seed uint32, b []byte) uint32 {
	sum1, sum2 := seed&0xFFFF, seed>>16
	for len(b) > 0 {
		// 0x15B0 bytes is the most that can be summed before sum2 could
		// overflow 32 bits; the modulus must be applied at exactly these
		// boundaries to agree with AutoCAD.
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

type r2004Page struct {
	offset uint64 // absolute file offset
	size   uint64 // bytes on disk, header included
}

type r2004Reader struct {
	data   []byte
	pages  map[int32]r2004Page
	budget uint64 // decompressed bytes still allowed
}

// take reserves n bytes of decompression buffer against the file's
// budget, before they are allocated.
func (r *r2004Reader) take(n uint64, what string) error {
	if n > r.budget {
		return r2004Errorf("%s needs more memory than this file could fill", what)
	}
	r.budget -= n
	return nil
}

// systemPage reads and decompresses the page map or section map at off.
func (r *r2004Reader) systemPage(off uint64, wantType uint32) ([]byte, error) {
	name := "page map"
	if wantType == r2004SectionMap {
		name = "section map"
	}
	end := uint64(len(r.data))
	if off > end || end-off < r2004SystemHeader {
		return nil, r2004Errorf("%s is past the end of the file", name)
	}
	hdr := r.data[off : off+r2004SystemHeader]
	typ := binary.LittleEndian.Uint32(hdr[0:])
	size := binary.LittleEndian.Uint32(hdr[4:])
	csize := binary.LittleEndian.Uint32(hdr[8:])
	method := binary.LittleEndian.Uint32(hdr[12:])
	sum := binary.LittleEndian.Uint32(hdr[16:])
	if typ != wantType {
		return nil, r2004Errorf("%s page has type %#08x", name, typ)
	}
	body := off + r2004SystemHeader
	if uint64(csize) > end-body {
		return nil, r2004Errorf("%s runs past the end of the file", name)
	}
	comp := r.data[body : body+uint64(csize)]
	// Seed 0 over the header with its checksum zeroed, then on over the
	// compressed bytes.
	var zeroed [r2004SystemHeader]byte
	copy(zeroed[:], hdr)
	binary.LittleEndian.PutUint32(zeroed[16:], 0)
	if got := r2004Checksum(r2004Checksum(0, zeroed[:]), comp); got != sum {
		return nil, r2004Errorf("%s checksum is %#08x, want %#08x", name, got, sum)
	}
	if method != r2004Compressed {
		return nil, r2004Errorf("%s has compression type %d", name, method)
	}
	if uint64(size) > min(lzMaxRatio*uint64(csize)+lzSlack, r.budget) {
		return nil, r2004Errorf("%s claims an impossible expansion", name)
	}
	r.budget -= uint64(size)
	out := make([]byte, size)
	n, err := decompressR2004(out, comp)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if n != len(out) {
		return nil, r2004Errorf("%s decompresses to %d bytes, want %d", name, n, len(out))
	}
	return out, nil
}

func (r *r2004Reader) readPageMap(h r2004Header) error {
	m, err := r.systemPage(h.pageMapAddr, r2004PageMap)
	if err != nil {
		return err
	}
	r.pages = make(map[int32]r2004Page, len(m)/8)
	addr := uint64(r2004FirstPage)
	for len(m) > 0 {
		if len(m) < 8 {
			return r2004Errorf("page map has a partial entry")
		}
		id := r2004ID(m[0:])
		size := uint64(binary.LittleEndian.Uint32(m[4:]))
		m = m[8:]
		if id < 0 {
			// A gap (freed space) carries four more longs of tree links
			// the reader has no use for.
			if len(m) < 16 {
				return r2004Errorf("page map has a partial gap entry")
			}
			m = m[16:]
		} else {
			if _, dup := r.pages[id]; dup {
				return r2004Errorf("page %d is mapped twice", id)
			}
			// A page past a truncated end only matters if a section
			// uses it, and then it reads as missing.
			if addr+size <= uint64(len(r.data)) {
				r.pages[id] = r2004Page{offset: addr, size: size}
			}
		}
		addr += size
	}
	return nil
}

type r2004Desc struct {
	name      string
	size      uint64 // decompressed size of the whole section
	maxPage   uint32 // decompressed size of a full page
	compress  uint32
	id        uint32
	encrypted uint32
	pages     []r2004DescPage
}

type r2004DescPage struct {
	id    int32
	csize uint32
	start uint64
}

func (r *r2004Reader) readSectionMap(h r2004Header) ([]r2004Desc, error) {
	p, ok := r.pages[h.sectionMapID]
	if !ok {
		return nil, r2004Errorf("section map page %d is missing", h.sectionMapID)
	}
	m, err := r.systemPage(p.offset, r2004SectionMap)
	if err != nil {
		return nil, err
	}
	if len(m) < r2004MapPreamble {
		return nil, r2004Errorf("section map is too short")
	}
	count := binary.LittleEndian.Uint32(m[0:])
	m = m[r2004MapPreamble:]
	// Each description takes at least r2004DescSize bytes; checking the
	// count against that first keeps a hostile count from sizing the slice.
	if uint64(count) > uint64(len(m)/r2004DescSize) {
		return nil, r2004Errorf("section map lists %d sections but holds fewer", count)
	}
	descs := make([]r2004Desc, count)
	for i := range descs {
		if len(m) < r2004DescSize {
			return nil, r2004Errorf("section map is truncated")
		}
		d := &descs[i]
		d.size = binary.LittleEndian.Uint64(m[0:])
		npages := binary.LittleEndian.Uint32(m[8:])
		d.maxPage = binary.LittleEndian.Uint32(m[12:])
		d.compress = binary.LittleEndian.Uint32(m[20:])
		d.id = binary.LittleEndian.Uint32(m[24:])
		d.encrypted = binary.LittleEndian.Uint32(m[28:])
		name := m[32 : 32+r2004NameSize]
		if k := bytes.IndexByte(name, 0); k >= 0 {
			name = name[:k]
		}
		d.name = string(name)
		m = m[r2004DescSize:]
		if uint64(npages) > uint64(len(m)/r2004PageEntry) {
			return nil, r2004Errorf("section %q lists more pages than the map holds", d.name)
		}
		d.pages = make([]r2004DescPage, npages)
		for j := range d.pages {
			d.pages[j] = r2004DescPage{
				id:    r2004ID(m[0:]),
				csize: binary.LittleEndian.Uint32(m[4:]),
				start: binary.LittleEndian.Uint64(m[8:]),
			}
			m = m[r2004PageEntry:]
		}
	}
	return descs, nil
}

// readSection reassembles one section. Pages are placed at their own start
// offsets rather than concatenated: AutoCAD omits pages that are all
// zeros, and the holes they leave must stay zero-filled.
func (r *r2004Reader) readSection(d *r2004Desc) ([]byte, error) {
	if d.encrypted == 1 {
		return nil, r2004Errorf("encrypted DWG files aren't supported")
	}
	if d.compress != r2004Stored && d.compress != r2004Compressed {
		return nil, r2004Errorf("section %q has compression type %d", d.name, d.compress)
	}
	what := fmt.Sprintf("section %q", d.name)
	if err := r.take(d.size, what); err != nil {
		return nil, err
	}
	out := make([]byte, d.size)
	var scratch []byte
	if len(d.pages) > 0 && d.compress == r2004Compressed {
		// One page buffer per section rather than per page; pages
		// decompress to the full page size even at the section's end.
		if err := r.take(uint64(d.maxPage), what); err != nil {
			return nil, err
		}
		scratch = make([]byte, d.maxPage)
	}
	for _, dp := range d.pages {
		if err := r.readDataPage(d, dp, out, scratch); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *r2004Reader) readDataPage(d *r2004Desc, dp r2004DescPage, out, scratch []byte) error {
	p, ok := r.pages[dp.id]
	if !ok {
		return r2004Errorf("section %q uses missing page %d", d.name, dp.id)
	}
	if p.size < r2004DataHeader {
		return r2004Errorf("page %d is too small for its header", dp.id)
	}
	var hdr [r2004DataHeader]byte
	copy(hdr[:], r.data[p.offset:])
	// The mask depends on where the page sits, so a page copied elsewhere
	// in the file won't decode. Only the offset's low 32 bits take part.
	mask := r2004DataMask ^ uint32(p.offset&0xFFFFFFFF)
	for k := 0; k < r2004DataHeader; k += 4 {
		binary.LittleEndian.PutUint32(hdr[k:], binary.LittleEndian.Uint32(hdr[k:])^mask)
	}
	typ := binary.LittleEndian.Uint32(hdr[0x00:])
	secID := binary.LittleEndian.Uint32(hdr[0x04:])
	csize := uint64(binary.LittleEndian.Uint32(hdr[0x08:]))
	start := binary.LittleEndian.Uint64(hdr[0x10:])
	hsum := binary.LittleEndian.Uint32(hdr[0x18:])
	dsum := binary.LittleEndian.Uint32(hdr[0x1C:])
	switch {
	case typ != r2004DataPage:
		return r2004Errorf("page %d has type %#08x", dp.id, typ)
	case secID != d.id:
		return r2004Errorf("page %d belongs to section %d, not %q (%d)", dp.id, secID, d.name, d.id)
	case csize != uint64(dp.csize) || start != dp.start:
		return r2004Errorf("page %d disagrees with the section map", dp.id)
	case csize > p.size-r2004DataHeader:
		return r2004Errorf("page %d's data overruns the page", dp.id)
	case start >= d.size:
		return r2004Errorf("page %d starts past the end of section %q", dp.id, d.name)
	}
	body := r.data[p.offset+r2004DataHeader:][:csize]
	if got := r2004Checksum(0, body); got != dsum {
		return r2004Errorf("page %d data checksum is %#08x, want %#08x", dp.id, got, dsum)
	}
	// The header checksum covers the unmasked header with its own field
	// zeroed, seeded with the data checksum.
	binary.LittleEndian.PutUint32(hdr[0x18:], 0)
	if got := r2004Checksum(dsum, hdr[:]); got != hsum {
		return r2004Errorf("page %d header checksum is %#08x, want %#08x", dp.id, got, hsum)
	}
	// Pages are written whole, so the last one overhangs the section's end.
	// What lies past it is padding, not always zeros (stored pages in
	// AutoCAD files carry leftover bytes), and is dropped.
	dst := out[start:]
	if d.compress == r2004Stored {
		if csize > uint64(d.maxPage) {
			return r2004Errorf("page %d is larger than section %q's pages", dp.id, d.name)
		}
		copy(dst, body)
		return nil
	}
	n, err := decompressR2004(scratch, body)
	if err != nil {
		return fmt.Errorf("section %q page %d: %w", d.name, dp.id, err)
	}
	copy(dst, scratch[:n])
	return nil
}
