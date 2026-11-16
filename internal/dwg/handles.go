package dwg

import "fmt"

// mapEntry is one object map entry: an object's handle and where its data
// starts in the objects section.
type mapEntry struct {
	handle uint64
	offset int64
}

// readObjectMap reads the object map (ODA specification chapter 23): runs
// of handle and offset deltas as modular chars, each run prefixed by its
// big-endian size and followed by a CRC, ending with a run of size 2.
//
// A map cut short keeps the entries read so far: the objects they find are
// as good as any, and the rest are simply missing.
func readObjectMap(sec []byte) ([]mapEntry, error) {
	r := newBits(sec)
	var out []mapEntry
	for r.left() >= 16 {
		size := int(r.RC())<<8 | int(r.RC())
		if size == 2 {
			return out, nil
		}
		if size < 2 || (size-2)*8 > r.left() {
			return out, fmt.Errorf("%w: object map run of %d bytes", ErrCorrupt, size)
		}
		run := r.sub(r.pos, r.pos+(size-2)*8)
		// Handles and offsets restart from zero in each run.
		var handle uint64
		var offset int64
		for run.left() > 0 && run.err == nil {
			dh := run.MC()
			offset += run.MCs()
			handle += dh
			if run.err == nil && dh > 0 {
				out = append(out, mapEntry{handle, offset})
			}
		}
		if run.err != nil {
			return out, run.err
		}
		r.skip((size-2)*8 + 16) // the run and its CRC
	}
	return out, nil
}
