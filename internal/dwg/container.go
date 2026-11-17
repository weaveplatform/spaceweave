// Package dwg reads AutoCAD DWG drawings natively into a cad.Document, the
// same model the DXF reader produces, so everything downstream (drawing,
// tracing) works on either.
//
// Supported: R2000 (AC1015) and the paged R2004 family, R2004 (AC1018),
// R2010 (AC1024), R2013 (AC1027) and R2018 (AC1032). R2007 (AC1021) uses a
// container of its own and R14 and older predate the object model here;
// both are refused with ErrVersion.
//
// Written from the Open Design Alliance's "Open Design Specification for
// .dwg files" (free to read). ACadSharp (MIT) was consulted; nothing is
// taken from GPL projects such as LibreDWG.
package dwg

import "errors"

// Section names. An R2004-family file stores its data in named sections;
// an R2000 file's are found through section locators and given the same
// names, so the object decoder doesn't care which container it came from.
const (
	SectionHeader  = "AcDb:Header"
	SectionClasses = "AcDb:Classes"
	SectionHandles = "AcDb:Handles" // the object map: handle → offset
	SectionObjects = "AcDb:AcDbObjects"
)

// Errors.
var (
	ErrNotDWG  = errors.New("dwg: not a DWG file")
	ErrVersion = errors.New("dwg: unsupported DWG version")
	ErrCorrupt = errors.New("dwg: file is damaged")
)

// container is a file's sections, decompressed and reassembled.
//
// Offsets in the object map index Sections[SectionObjects]. For R2000 that
// section is the whole file, because R2000's map holds absolute file
// offsets; for R2004 and later it is the AcDb:AcDbObjects section, whose
// map offsets are relative to it.
type container struct {
	Version  string // "AC1015", "AC1018", "AC1024", "AC1027", "AC1032"
	Codepage uint16 // for pre-R2007 8-bit strings
	// Maintenance is the maintenance release byte, which R2010+ object
	// decoding needs in a few places.
	Maintenance byte
	Sections    map[string][]byte
}
