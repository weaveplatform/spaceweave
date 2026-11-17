package dwg

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// The R2000 file header's end sentinel (ODA specification 3.2.6).
var r2000HeaderEnd = []byte{
	0x95, 0xA0, 0x4E, 0x28, 0x99, 0x82, 0x1A, 0xE5,
	0x5E, 0x41, 0xE0, 0x5F, 0x9D, 0x3A, 0x4D, 0x00,
}

// Section locator record numbers.
const (
	locHeader  = 0
	locClasses = 1
	locHandles = 2
)

// r2000Locators is the most section locators read. Files have 3 to 6; the
// cap keeps a corrupt count from walking off into the header data.
const r2000Locators = 16

// readR2000 reads the R13–R2000 container: a short file header whose
// section locators give each section's absolute offset and size. The
// sections aren't compressed, so this only slices them out. The object map
// holds absolute file offsets, so the objects "section" is the whole file.
func readR2000(data []byte) (*container, error) {
	const fixed = 0x19 // through the locator count
	if len(data) < fixed {
		return nil, fmt.Errorf("%w: the file header is cut short", ErrCorrupt)
	}
	n := binary.LittleEndian.Uint32(data[0x15:])
	if n < 3 || n > r2000Locators {
		return nil, fmt.Errorf("%w: %d section locators", ErrCorrupt, n)
	}
	end := fixed + int(n)*9
	if len(data) < end+2+len(r2000HeaderEnd) {
		return nil, fmt.Errorf("%w: the file header is cut short", ErrCorrupt)
	}
	if !bytes.Equal(data[end+2:end+2+len(r2000HeaderEnd)], r2000HeaderEnd) {
		return nil, fmt.Errorf("%w: the file header's sentinel is missing", ErrCorrupt)
	}
	c := &container{
		Version:     string(data[:6]),
		Maintenance: data[0x0B],
		Codepage:    binary.LittleEndian.Uint16(data[0x13:]),
		Sections:    map[string][]byte{SectionObjects: data},
	}
	// The file header's CRC isn't checked: one that doesn't match is left
	// for the sections to trip over, since an old writer's wrong magic
	// number shouldn't cost a readable drawing.
	names := map[byte]string{
		locHeader:  SectionHeader,
		locClasses: SectionClasses,
		locHandles: SectionHandles,
	}
	for i := range int(n) {
		rec := data[fixed+i*9:]
		name, ok := names[rec[0]]
		if !ok {
			continue
		}
		off, size := uint64(
			binary.LittleEndian.Uint32(rec[1:]),
		), uint64(
			binary.LittleEndian.Uint32(rec[5:]),
		)
		if off+size > uint64(len(data)) {
			return nil, fmt.Errorf(
				"%w: section %d lies past the end of the file",
				ErrCorrupt,
				rec[0],
			)
		}
		c.Sections[name] = data[off : off+size]
	}
	for _, name := range []string{SectionHeader, SectionClasses, SectionHandles} {
		if _, ok := c.Sections[name]; !ok {
			return nil, fmt.Errorf("%w: no %s section", ErrCorrupt, name)
		}
	}
	return c, nil
}
