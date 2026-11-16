package dwg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/text/encoding"

	"github.com/weaveplatform/spaceweave/internal/cad"
)

// ErrTooLarge is returned for a file bigger than MaxSize.
var ErrTooLarge = errors.New("dwg: file too large")

// MaxSize is the largest file Read takes. Floor plans run to tens of
// megabytes; the whole file is held in memory while it is read.
const MaxSize = 256 << 20

// maxSize is MaxSize, smaller in tests.
var maxSize int64 = MaxSize

// ReadFile reads a DWG file from disk.
func ReadFile(path string) (*cad.Document, error) {
	f, err := os.Open(path) //nolint:gosec // the caller names the file to read
	if err != nil {
		return nil, fmt.Errorf("dwg: %w", err)
	}
	defer func() { _ = f.Close() }()
	return Read(f)
}

// Read reads a DWG drawing. Objects it can't decode are left out and
// counted in the document's Skipped; only a file whose structure can't be
// read at all is an error.
func Read(r io.Reader) (*cad.Document, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return nil, fmt.Errorf("dwg: %w", err)
	}
	if int64(len(data)) > maxSize {
		return nil, ErrTooLarge
	}
	doc, _, err := decode(data)
	return doc, err
}

// stats breaks down what a read found, for tests and diagnostics: entities
// by type, and what was skipped and why.
type stats struct {
	types   map[string]int
	skipped map[string]int
}

func decode(data []byte) (*cad.Document, *stats, error) {
	if len(data) < 6 || !strings.HasPrefix(string(data[:6]), "AC") {
		return nil, nil, ErrNotDWG
	}
	version := string(data[:6])
	v, ok := versions[version]
	if !ok {
		if strings.HasPrefix(version, "AC10") || strings.HasPrefix(version, "AC1.") ||
			strings.HasPrefix(version, "AC2.") {
			return nil, nil, fmt.Errorf("%w: %s", ErrVersion, version)
		}
		return nil, nil, ErrNotDWG
	}
	var c *container
	var err error
	if v == v2000 {
		c, err = readR2000(data)
	} else {
		c, err = readR2004(data)
	}
	if err != nil {
		return nil, nil, err
	}
	d := &decoder{
		c: c, ver: v, objs: c.Sections[SectionObjects], classes: map[int]class{},
		doc: cad.NewDocument(),
		st:  &stats{types: map[string]int{}, skipped: map[string]int{}},
	}
	d.doc.Version = version
	if v < v2007 {
		d.enc = codepage(c.Codepage)
	}
	if err := d.run(); err != nil {
		return nil, d.st, err
	}
	return d.doc, d.st, nil
}

// decoder holds a read in progress.
type decoder struct {
	c       *container
	ver     ver
	objs    []byte
	enc     encoding.Encoding
	classes map[int]class
	doc     *cad.Document
	st      *stats

	hdr      header
	layers   map[uint64]*cad.Layer
	blocks   map[uint64]*blockRec
	recs     map[uint64]*rec
	order    []uint64 // entity handles in object map order
	model    uint64   // block records of model and paper space
	paper    uint64
	notDrawn map[string]int
}
