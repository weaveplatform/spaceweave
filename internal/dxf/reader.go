// Package dxf reads ASCII and binary DXF files into a cad.Document.
//
// A DXF file is a flat list of (group code, value) pairs; meaning comes from
// the codes and the order. The reader keeps what a floor plan needs (units,
// layers, blocks and the drawable entities) and skips the rest without
// failing: a drawing with one entity type we don't draw is still a drawing.
package dxf

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Errors.
var (
	ErrNotDXF    = errors.New("not a DXF file")
	ErrTruncated = errors.New("the DXF file ends early")
)

// BinarySentinel opens every binary DXF file.
const BinarySentinel = "AutoCAD Binary DXF\r\n\x1a\x00"

// maxLine bounds one line of an ASCII DXF; real values are short, but an
// embedded binary chunk can run to a few kilobytes of hex.
const maxLine = 1 << 20

// pair is one group code and its value. Binary values are converted to text
// so the rest of the reader handles both encodings alike.
type pair struct {
	code int
	val  string
}

func (p pair) float() float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(p.val), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

func (p pair) int() int {
	s := strings.TrimSpace(p.val)
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	// Some writers put a float where an integer belongs.
	return int(p.float())
}

// source yields pairs.
type source interface {
	next() (pair, error)
}

func newSource(r io.Reader) (source, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	head, err := br.Peek(len(BinarySentinel))
	if err == nil && string(head) == BinarySentinel {
		_, _ = br.Discard(len(BinarySentinel))
		return &binarySource{r: br}, nil
	}
	sc := bufio.NewScanner(br)
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)
	return &asciiSource{sc: sc}, nil
}

type asciiSource struct {
	sc   *bufio.Scanner
	line int
}

func (s *asciiSource) scan() (string, error) {
	if !s.sc.Scan() {
		if err := s.sc.Err(); err != nil {
			return "", fmt.Errorf("read DXF: %w", err)
		}
		return "", io.EOF
	}
	s.line++
	return strings.TrimSuffix(s.sc.Text(), "\r"), nil
}

func (s *asciiSource) next() (pair, error) {
	c, err := s.scan()
	if err != nil {
		return pair{}, err
	}
	// The first line may carry a UTF-8 byte-order mark.
	c = strings.TrimSpace(strings.TrimPrefix(c, "\xef\xbb\xbf"))
	code, err := strconv.Atoi(c)
	if err != nil {
		return pair{}, fmt.Errorf(
			"%w: line %d is %q, not a group code",
			ErrNotDXF,
			s.line,
			truncate(c),
		)
	}
	v, err := s.scan()
	if errors.Is(err, io.EOF) {
		return pair{}, ErrTruncated
	}
	if err != nil {
		return pair{}, err
	}
	return pair{code: code, val: v}, nil
}

func truncate(s string) string {
	if len(s) > 20 {
		return s[:20] + "…"
	}
	return s
}

// binarySource reads binary DXF (AutoCAD R13 and later: two-byte codes).
type binarySource struct{ r *bufio.Reader }

func (s *binarySource) next() (pair, error) {
	var code uint16
	if err := binary.Read(s.r, binary.LittleEndian, &code); err != nil {
		if errors.Is(err, io.EOF) {
			return pair{}, io.EOF
		}
		return pair{}, ErrTruncated
	}
	c := int(code)
	v, err := s.value(c)
	if err != nil {
		return pair{}, ErrTruncated
	}
	return pair{code: c, val: v}, nil
}

func (s *binarySource) value(code int) (string, error) {
	switch kind(code) {
	case kindFloat:
		var f float64
		err := binary.Read(s.r, binary.LittleEndian, &f)
		return strconv.FormatFloat(
			f,
			'g',
			-1,
			64,
		), err //nolint:wrapcheck // ErrTruncated replaces it
	case kindInt16:
		var n int16
		err := binary.Read(s.r, binary.LittleEndian, &n)
		return strconv.Itoa(int(n)), err //nolint:wrapcheck // as above
	case kindInt32:
		var n int32
		err := binary.Read(s.r, binary.LittleEndian, &n)
		return strconv.Itoa(int(n)), err //nolint:wrapcheck // as above
	case kindInt64:
		var n int64
		err := binary.Read(s.r, binary.LittleEndian, &n)
		return strconv.FormatInt(n, 10), err //nolint:wrapcheck // as above
	case kindBool:
		b, err := s.r.ReadByte()
		return strconv.Itoa(int(b)), err //nolint:wrapcheck // as above
	case kindChunk:
		n, err := s.r.ReadByte()
		if err != nil {
			return "", err //nolint:wrapcheck // as above
		}
		_, err = s.r.Discard(int(n))
		return "", err //nolint:wrapcheck // as above
	default:
		b, err := s.r.ReadBytes(0)
		return string(bytes.TrimSuffix(b, []byte{0})), err //nolint:wrapcheck // as above
	}
}

type valueKind uint8

const (
	kindString valueKind = iota
	kindFloat
	kindInt16
	kindInt32
	kindInt64
	kindBool
	kindChunk
)

// kind is the type of a group code's value, from the DXF reference's table
// of group code ranges.
func kind(c int) valueKind {
	switch {
	case c >= 10 && c <= 59, c >= 110 && c <= 149, c >= 210 && c <= 239,
		c >= 460 && c <= 469, c >= 1010 && c <= 1059:
		return kindFloat
	case c >= 60 && c <= 79, c >= 170 && c <= 179, c >= 270 && c <= 289,
		c >= 370 && c <= 389, c >= 400 && c <= 409, c >= 1060 && c <= 1070:
		return kindInt16
	case c >= 90 && c <= 99, c >= 420 && c <= 429, c >= 440 && c <= 459, c == 1071:
		return kindInt32
	case c >= 160 && c <= 169:
		return kindInt64
	case c >= 290 && c <= 299:
		return kindBool
	case c >= 310 && c <= 319, c == 1004:
		return kindChunk
	default:
		return kindString
	}
}
