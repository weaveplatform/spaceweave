package dwg

// The R2004-family LZ77 variant, from the R2004 chapter of the ODA
// specification. The leading-literal shortcut (a first byte above 0x11)
// follows ACadSharp's DwgLZ77AC18Decompressor (MIT,
// github.com/DomCR/ACadSharp); the specification doesn't mention it, but
// no valid stream can start with a match, so accepting it costs nothing.
//
// Every read and write is bounds-checked: the input is untrusted, and a
// stream that runs off either end is reported as ErrCorrupt rather than
// allowed to panic or grow its output.

import "fmt"

var (
	errLZTruncated = fmt.Errorf("%w: compressed data ends mid-instruction", ErrCorrupt)
	errLZOverflow  = fmt.Errorf("%w: compressed data expands past its page", ErrCorrupt)
	errLZOffset    = fmt.Errorf("%w: compressed data refers back before its start", ErrCorrupt)
	errLZOpcode    = fmt.Errorf("%w: compressed data has a reserved opcode", ErrCorrupt)
)

// lzTerminator ends a compressed stream.
const lzTerminator = 0x11

// decompressR2004 expands src into dst and returns the number of bytes
// written. dst's length is the most the stream may produce: a page that
// claims more is corrupt, and failing beats silently truncating it.
func decompressR2004(dst, src []byte) (int, error) {
	z := lzState{src: src, dst: dst}
	op, ok := z.next()
	if !ok {
		return 0, errLZTruncated
	}
	if op > lzTerminator {
		if err := z.literal(int(op) - 17); err != nil {
			return z.o, err
		}
		if op, ok = z.next(); !ok {
			return z.o, nil
		}
	}
	if op&0xF0 == 0 {
		n, err := z.runLength(op, 0x0F)
		if err != nil {
			return z.o, err
		}
		if err = z.literal(n + 3); err != nil {
			return z.o, err
		}
		if op, ok = z.next(); !ok {
			return z.o, nil
		}
	}
	for op != lzTerminator {
		var err error
		if op, err = z.instruction(op); err != nil {
			return z.o, err
		}
		if z.done {
			break
		}
	}
	return z.o, nil
}

type lzState struct {
	src  []byte
	i    int
	dst  []byte
	o    int
	done bool // input ended cleanly where an opcode was due
}

func (z *lzState) next() (byte, bool) {
	if z.i >= len(z.src) {
		return 0, false
	}
	b := z.src[z.i]
	z.i++
	return b, true
}

// instruction runs one match opcode and the literal run that follows it,
// returning the next opcode.
func (z *lzState) instruction(op byte) (byte, error) {
	var length, offset int
	var lit byte // the low two bits that carry a short literal count
	switch {
	case op < 0x10:
		// Reserved: after a literal run a 0x00–0x0F byte would read as
		// another literal length, so encoders never emit one here.
		return 0, errLZOpcode
	case op >= 0x40:
		b, ok := z.next()
		if !ok {
			return 0, errLZTruncated
		}
		length = int(op>>4) - 1
		offset = (int(op>>2&3) | int(b)<<2) + 1
		lit = op & 3
	case op < 0x20: // 0x10, 0x12–0x1F
		n, err := z.runLength(op, 0x07)
		if err != nil {
			return 0, err
		}
		b1, b2, err := z.twoBytes()
		if err != nil {
			return 0, err
		}
		length = n + 2
		// Bit 3 of the opcode is offset bit 14; this form reaches the
		// distances 0x4000–0xBFFF that the 0x20 forms can't.
		offset = (int(op&8)<<11 | int(b1>>2) | int(b2)<<6) + 0x4000
		lit = b1 & 3
	default: // 0x20–0x3F
		n, err := z.runLength(op, 0x1F)
		if err != nil {
			return 0, err
		}
		b1, b2, err := z.twoBytes()
		if err != nil {
			return 0, err
		}
		length = n + 2
		offset = (int(b1>>2) | int(b2)<<6) + 1
		lit = b1 & 3
	}
	if err := z.match(offset, length); err != nil {
		return 0, err
	}
	n := int(lit)
	if n == 0 {
		next, ok := z.next()
		if !ok {
			z.done = true
			return 0, nil
		}
		if next&0xF0 != 0 {
			return next, nil
		}
		ln, err := z.runLength(next, 0x0F)
		if err != nil {
			return 0, err
		}
		n = ln + 3
	}
	if err := z.literal(n); err != nil {
		return 0, err
	}
	next, ok := z.next()
	if !ok {
		z.done = true
	}
	return next, nil
}

// runLength decodes the length packed into op's low bits (mask). Zero
// bits mean "long form": each following 0x00 byte adds 0xFF and the first
// non-zero byte adds itself plus mask and ends the count.
func (z *lzState) runLength(op, mask byte) (int, error) {
	n := int(op & mask)
	if n != 0 {
		return n, nil
	}
	n = int(mask)
	for {
		b, ok := z.next()
		if !ok {
			return 0, errLZTruncated
		}
		if b != 0 {
			return n + int(b), nil
		}
		n += 0xFF
	}
}

func (z *lzState) twoBytes() (byte, byte, error) {
	if z.i+2 > len(z.src) {
		return 0, 0, errLZTruncated
	}
	b1, b2 := z.src[z.i], z.src[z.i+1]
	z.i += 2
	return b1, b2, nil
}

func (z *lzState) literal(n int) error {
	if n > len(z.src)-z.i {
		return errLZTruncated
	}
	if n > len(z.dst)-z.o {
		return errLZOverflow
	}
	copy(z.dst[z.o:], z.src[z.i:z.i+n])
	z.i += n
	z.o += n
	return nil
}

func (z *lzState) match(offset, length int) error {
	if offset > z.o {
		return errLZOffset
	}
	if length > len(z.dst)-z.o {
		return errLZOverflow
	}
	// Copy forward one byte at a time when the source overlaps the
	// destination: an offset shorter than the length repeats the last
	// offset bytes, which copy() (memmove semantics) would not do.
	from := z.o - offset
	if offset >= length {
		copy(z.dst[z.o:z.o+length], z.dst[from:from+length])
	} else {
		for k := range length {
			z.dst[z.o+k] = z.dst[from+k]
		}
	}
	z.o += length
	return nil
}
