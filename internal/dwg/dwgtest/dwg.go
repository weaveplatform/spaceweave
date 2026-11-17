package dwgtest

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/weaveplatform/spaceweave/internal/cad"
	"github.com/weaveplatform/spaceweave/internal/dxf"
	"github.com/weaveplatform/spaceweave/internal/dxf/dxftest"
)

// Section names, as the R2004-family container stores them.
const (
	SectionHeader  = "AcDb:Header"
	SectionClasses = "AcDb:Classes"
	SectionHandles = "AcDb:Handles"
	SectionObjects = "AcDb:AcDbObjects"
)

// Versions are the DWG versions the writer produces, oldest first.
var Versions = []string{"AC1015", "AC1018", "AC1024", "AC1027", "AC1032"}

// Drawing describes a DWG file to write.
type Drawing struct {
	// Version is "AC1015" (R2000) to "AC1032" (R2018).
	Version string
	// Maintenance is the maintenance release, which from R2010 decides
	// whether section sizes have a high word.
	Maintenance byte
	Doc         *cad.Document
	// Extras adds what a reader must step over: entities it doesn't draw
	// (a 3DFACE, a polyface mesh, a class entity, a POINT), a non-entity
	// class object, and two objects it can't decode (a LINE cut short and
	// an object of an undeclared class), which count as skipped.
	Extras bool
}

// File writes doc as a DWG file of the given version.
func File(version string, doc *cad.Document) []byte {
	return Drawing{Version: version, Doc: doc}.Bytes()
}

// R2000 writes doc as an AC1015 file.
func R2000(doc *cad.Document) []byte { return File("AC1015", doc) }

// Office is dxftest.Office(4), read by the DXF reader and written as a DWG
// file of the given version, so tests can check that both formats read
// the same drawing the same way.
func Office(version string) []byte { return File(version, FromDXF(dxftest.Office(4))) }

// FromDXF reads a dxftest drawing with the DXF reader. It panics on a
// fixture the reader rejects, which is a broken fixture.
func FromDXF(d *dxftest.Doc) *cad.Document {
	doc, err := dxf.Read(bytes.NewReader(d.Bytes()))
	if err != nil {
		panic(fmt.Sprintf("dwgtest: fixture doesn't read: %v", err))
	}
	return doc
}

// Bytes encodes the file. R2000 is written here; later versions are
// wrapped by R2004File.
func (dr Drawing) Bytes() []byte {
	if dr.Version == "AC1015" {
		return dr.r2000()
	}
	f := R2004File{Version: dr.Version, Maintenance: dr.Maintenance}
	secs := dr.Sections()
	for _, name := range []string{SectionClasses, SectionHandles, SectionHeader, SectionObjects} {
		f.Sections = append(f.Sections, R2004Section{Name: name, Data: secs[name]})
	}
	return f.Bytes()
}

// Sections encodes an R2004-family drawing's sections, for R2004 or
// R2004File to wrap. Object map offsets count from the objects section's
// start, which holds the long 0x0DCA as AutoCAD's does.
func (dr Drawing) Sections() map[string][]byte {
	e := newEncoder(dr)
	objs := binary.LittleEndian.AppendUint32(nil, 0x0DCA)
	objs, offsets := e.objects(objs)
	return map[string][]byte{
		SectionHeader:  e.header(),
		SectionClasses: e.classSection(),
		SectionHandles: objectMap(offsets),
		SectionObjects: objs,
	}
}

// r2000CRCMagic is what the R2000 file header's CRC is XORed with, by the
// number of section locators (ODA specification 3.2.6).
var r2000CRCMagic = map[int]uint16{3: 0xA598, 4: 0x8101, 5: 0x3CC4, 6: 0x8461}

var r2000HeaderEnd = []byte{
	0x95, 0xA0, 0x4E, 0x28, 0x99, 0x82, 0x1A, 0xE5,
	0x5E, 0x41, 0xE0, 0x5F, 0x9D, 0x3A, 0x4D, 0x00,
}

// r2000 lays the sections out one after another behind a file header whose
// locators give each one's offset and size. Object map offsets are file
// offsets.
func (dr Drawing) r2000() []byte {
	e := newEncoder(dr)
	const locators = 3
	headerSize := 0x19 + locators*9 + 2 + len(r2000HeaderEnd)
	hdr, classes := e.header(), e.classSection()
	out := make([]byte, headerSize, headerSize+len(hdr)+len(classes))
	out = append(out, hdr...)
	out = append(out, classes...)
	out, offsets := e.objects(out)
	mapStart := len(out)
	out = append(out, objectMap(offsets)...)

	copy(out, dr.Version)
	out[0x0B] = dr.Maintenance
	out[0x0C] = 1
	binary.LittleEndian.PutUint16(out[0x13:], codepage)
	binary.LittleEndian.PutUint32(out[0x15:], locators)
	locs := []struct{ off, size int }{
		{headerSize, len(hdr)},
		{headerSize + len(hdr), len(classes)},
		{mapStart, len(out) - mapStart},
	}
	for i, l := range locs {
		rec := out[0x19+i*9:]
		rec[0] = byte(i)
		binary.LittleEndian.PutUint32(rec[1:], u32(l.off))
		binary.LittleEndian.PutUint32(rec[5:], u32(l.size))
	}
	end := 0x19 + locators*9
	binary.LittleEndian.PutUint16(out[end:], crc16(0, out[:end])^r2000CRCMagic[locators])
	copy(out[end+2:], r2000HeaderEnd)
	return out
}

// codepage is ANSI_1252, the one strings are written in.
const codepage = 30

// objectMap writes the handle-to-offset map: runs of at most 2032 bytes of
// handle and offset deltas, each restarting from zero, behind a big-endian
// size and ahead of a big-endian CRC, then an empty run.
func objectMap(offsets []mapEntry) []byte {
	const maxRun = 2032
	var out []byte
	run := &Bits{}
	var handle uint64
	var offset int64
	flush := func() {
		size := run.Len()/8 + 2
		chunk := make([]byte, 0, size)
		chunk = append(chunk, byte(size>>8&0xff), byte(size&0xff))
		chunk = append(chunk, run.Bytes()...)
		crc := crc16(0xC0C1, chunk)
		out = append(out, chunk...)
		out = binary.BigEndian.AppendUint16(out, crc)
		run, handle, offset = &Bits{}, 0, 0
	}
	for _, m := range offsets {
		if run.Len()/8 > maxRun-20 {
			flush()
		}
		run.MC(m.handle - handle)
		run.MCs(m.offset - offset)
		handle, offset = m.handle, m.offset
	}
	if run.Len() > 0 {
		flush()
	}
	flush()
	return out
}

type mapEntry struct {
	handle uint64
	offset int64
}

// crc16 is CRC-16/ARC, which the specification calls its 8-bit CRC.
func crc16(seed uint16, b []byte) uint16 {
	for _, c := range b {
		seed ^= uint16(c)
		for range 8 {
			if seed&1 != 0 {
				seed = seed>>1 ^ 0xA001
			} else {
				seed >>= 1
			}
		}
	}
	return seed
}
