package dxf

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"github.com/weaveplatform/spaceweave/internal/cad"
)

// Read parses a DXF file.
func Read(r io.Reader) (*cad.Document, error) {
	src, err := newSource(r)
	if err != nil {
		return nil, err
	}
	p := &parser{src: src, doc: cad.NewDocument(), unknown: map[string]int{}}
	if err := p.file(); err != nil {
		return nil, err
	}
	p.doc.Notes = append(p.doc.Notes, p.unknownNote()...)
	return p.doc, nil
}

type parser struct {
	src     source
	back    *pair
	doc     *cad.Document
	unknown map[string]int
	// legacy is set for drawings before AutoCAD 2007, whose text is in a
	// Windows code page rather than UTF-8.
	legacy bool
}

func (p *parser) read() (pair, error) {
	if p.back != nil {
		pr := *p.back
		p.back = nil
		return pr, nil
	}
	return p.src.next()
}

func (p *parser) unread(pr pair) { p.back = &pr }

// file reads the sections. Anything before the first section other than a
// comment means this isn't a DXF file.
func (p *parser) file() error {
	sections := 0
	for {
		pr, err := p.read()
		if errors.Is(err, io.EOF) {
			if sections == 0 {
				return ErrNotDXF
			}
			// A file without its closing EOF is common and harmless.
			return nil
		}
		if err != nil {
			return err
		}
		switch {
		case pr.code == 999:
		case sections == 0 && (pr.code != 0 || pr.val != "SECTION"):
			// Even "0 / EOF" alone: anything but comments before the first
			// section means this isn't a drawing.
			return fmt.Errorf("%w: it doesn't start with a section", ErrNotDXF)
		case pr.code != 0:
		case pr.val == "EOF":
			return nil
		case pr.val == "SECTION":
			sections++
			if err := p.section(); err != nil {
				return err
			}
		}
	}
}

func (p *parser) section() error {
	name, err := p.read()
	if err != nil {
		return eof(err)
	}
	switch strings.TrimSpace(name.val) {
	case "HEADER":
		return p.header()
	case "TABLES":
		return p.tables()
	case "BLOCKS":
		return p.blocks()
	case "ENTITIES":
		model, paper, err := p.entities("ENDSEC")
		p.doc.Model, p.doc.Paper = append(p.doc.Model, model...), append(p.doc.Paper, paper...)
		return err
	default:
		return p.skipTo("ENDSEC")
	}
}

// eof turns a clean end of file in the middle of a structure into an error.
func eof(err error) error {
	if errors.Is(err, io.EOF) {
		return ErrTruncated
	}
	return err
}

func (p *parser) skipTo(end string) error {
	for {
		pr, err := p.read()
		if err != nil {
			return eof(err)
		}
		if pr.code == 0 && pr.val == end {
			return nil
		}
	}
}

// group reads the pairs of one object after its code-0 type, up to the next
// code 0.
func (p *parser) group() (fields, error) {
	var g fields
	for {
		pr, err := p.read()
		if err != nil {
			return nil, eof(err)
		}
		if pr.code == 0 {
			p.unread(pr)
			return g, nil
		}
		g = append(g, pr)
	}
}

func (p *parser) header() error {
	for {
		pr, err := p.read()
		if err != nil {
			return eof(err)
		}
		if pr.code == 0 && pr.val == "ENDSEC" {
			return nil
		}
		if pr.code != 9 {
			continue
		}
		v, err := p.read()
		if err != nil {
			return eof(err)
		}
		switch pr.val {
		case "$INSUNITS":
			p.doc.Units = cad.Units(v.int())
		case "$ACADVER":
			p.doc.Version = strings.TrimSpace(v.val)
			p.legacy = p.doc.Version < "AC1021"
		}
	}
}

func (p *parser) tables() error {
	for {
		pr, err := p.read()
		if err != nil {
			return eof(err)
		}
		if pr.code != 0 {
			continue
		}
		switch pr.val {
		case "ENDSEC":
			return nil
		case "LAYER":
			g, err := p.group()
			if err != nil {
				return err
			}
			p.layer(g)
		}
	}
}

func (p *parser) layer(g fields) {
	name := p.text(g.str(2))
	if name == "" {
		return
	}
	l := &cad.Layer{Name: name, Color: 7}
	if c, ok := g.int(62); ok {
		// A negative colour number means the layer is off.
		l.Color, l.Off = c, c < 0
		if c < 0 {
			l.Color = -c
		}
	}
	if f, ok := g.int(70); ok {
		l.Frozen = f&1 != 0
	}
	if c, ok := g.int(420); ok {
		l.RGB = trueColor(c)
	}
	p.doc.Layers[name] = l
}

func (p *parser) blocks() error {
	for {
		pr, err := p.read()
		if err != nil {
			return eof(err)
		}
		if pr.code != 0 {
			continue
		}
		switch pr.val {
		case "ENDSEC":
			return nil
		case "BLOCK":
			g, err := p.group()
			if err != nil {
				return err
			}
			b := &cad.Block{Name: p.text(g.str(2)), Base: g.point(10)}
			b.Entities, _, err = p.entities("ENDBLK")
			if err != nil {
				return err
			}
			if b.Name != "" {
				p.doc.Blocks[b.Name] = b
			}
		}
	}
}

// entities reads entities up to the code-0 marker end, sorting them into
// model and paper space.
func (p *parser) entities(end string) (model, paper []cad.Entity, err error) {
	for {
		pr, err := p.read()
		if err != nil {
			return model, paper, eof(err)
		}
		if pr.code != 0 {
			continue
		}
		if pr.val == end {
			return model, paper, nil
		}
		g, err := p.group()
		if err != nil {
			return model, paper, err
		}
		e, err := p.entity(pr.val, g)
		if err != nil {
			return model, paper, err
		}
		if e == nil {
			continue
		}
		if v, _ := g.int(67); v == 1 {
			paper = append(paper, e)
		} else {
			model = append(model, e)
		}
	}
}

// children reads the VERTEX or ATTRIB groups that follow a POLYLINE or an
// INSERT, through the closing SEQEND.
func (p *parser) children() ([]fields, error) {
	var out []fields
	for {
		pr, err := p.read()
		if err != nil {
			return nil, eof(err)
		}
		if pr.code != 0 {
			continue
		}
		if pr.val != "SEQEND" && pr.val != "VERTEX" && pr.val != "ATTRIB" {
			// No SEQEND: this is already the next entity.
			p.unread(pr)
			return out, nil
		}
		g, err := p.group()
		if err != nil {
			return nil, err
		}
		if pr.val == "SEQEND" {
			return out, nil
		}
		out = append(out, g)
	}
}

// text decodes a string from the file: UTF-8 from AutoCAD 2007 on, the
// Windows code page before it (Latin-1 Western covers the plans we see).
func (p *parser) text(s string) string {
	if p.legacy && !utf8.ValidString(s) {
		if d, err := charmap.Windows1252.NewDecoder().String(s); err == nil {
			return d
		}
	}
	return strings.ToValidUTF8(s, "�")
}

// notDrawn are entity types that carry nothing a plan view should show.
var notDrawn = []string{"VIEWPORT", "POINT", "ATTDEF", "SEQEND", "VERTEX", "XLINE", "RAY"}

func (p *parser) unknownNote() []string {
	if len(p.unknown) == 0 {
		return nil
	}
	names := make([]string, 0, len(p.unknown))
	for n := range p.unknown {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%d %s", p.unknown[n], n))
	}
	return []string{"Not drawn: " + strings.Join(parts, ", ") + "."}
}

func (p *parser) skip(kind string) {
	if !slices.Contains(notDrawn, kind) {
		p.unknown[kind]++
	}
}
