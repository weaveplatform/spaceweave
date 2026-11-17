package dwgtest

import "strings"

// headerLayout lists the header variables of R2000 and later in file order
// (ODA specification chapter 9), as from-version, to-version and their bit
// codes. A ":name" suffix marks a variable the writer gives a value; the
// rest are written as zeros. It is a second transcription of the
// specification's list, not a copy of the reader's, so the two can only
// agree by both being right.
var headerLayout = []struct {
	from, to ver
	fields   string
}{
	{v2013, v2018, "BLL"},
	{v2000, v2018, "BD BD BD BD TV TV TV TV BL BL"},
	{v2000, v2000, "H"},
	{v2000, v2018, "B B B B B B B B B"},
	{v2004, v2018, "B"},
	{v2000, v2018, "B B B B B B B B B B B BS BS BS BS BS BS BS BS"},
	{v2004, v2018, "BL BL BL"},
	{v2000, v2018, "BS BS BS BS BS BS BS BS BS BS BS BS BS BS BS BS BS BS BS"},
	{v2000, v2018, "BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD BD"},
	{v2000, v2018, "TV BL BL BL BL"},
	{v2004, v2018, "BL BL BL"},
	{v2000, v2018, "BL BL BL BL CMC HS H H H"},
	{v2007, v2018, "H"},
	{v2000, v2018, "H H BD"},
	{v2000, v2018, "3BD 3BD 3BD 2RD 2RD BD 3BD 3BD 3BD H H BS H 3BD 3BD 3BD 3BD 3BD 3BD"},
	{v2000, v2018, "3BD 3BD 3BD 2RD 2RD BD 3BD 3BD 3BD H H BS H"},
	{v2000, v2018, "3BD 3BD 3BD 3BD 3BD 3BD TV TV BD BD BD BD BD BD BD BD BD"},
	{v2007, v2018, "BD BD BS CMC"},
	{v2000, v2018, "B B B B B B BS BS BS"},
	{v2007, v2018, "BS"},
	{v2000, v2018, "BD BD BD BD BD BD BD BD BD B BS B B B B CMC CMC CMC"},
	{v2000, v2018, "BS BS BS BS BS BS BS BS BS BS BS B B BS BS BS BS B BS"},
	{v2007, v2018, "B"},
	{v2010, v2018, "B BD TV BD TV"},
	{v2000, v2018, "H H H H H"},
	{v2007, v2018, "H H H"},
	{v2000, v2018, "BS BS H:blockcontrol H:layercontrol H H H H H H H"},
	{v2000, v2000, "H"},
	{v2000, v2018, "H H H BS BS TV TV H H H"},
	{v2004, v2018, "H H"},
	{v2007, v2018, "H"},
	{v2013, v2018, "H"},
	{v2000, v2018, "BL BS:insunits BS:psntype H:psn TV TV"},
	{v2004, v2018, "RC RC RC RC RC RC BS BS RC RC TV"},
	{v2000, v2018, "H:paperspace H:modelspace H H H"},
}

// header writes the header variables section.
func (e *encoder) header() []byte {
	d, s, h := &Bits{}, &Bits{}, &Bits{}
	if e.v < v2007 {
		s, h = d, d
	}
	// CEPSNTYPE 3 ("by object") is the case with a CPSNID handle after
	// it; later versions use it so both cases are written.
	psnType := int16(0)
	if e.v >= v2010 {
		psnType = 3
	}
	for _, step := range headerLayout {
		if e.v < step.from || e.v > step.to {
			continue
		}
		for f := range strings.FieldsSeq(step.fields) {
			kind, name, _ := strings.Cut(f, ":")
			switch kind {
			case "B":
				d.B(false)
			case "BS":
				switch name {
				case "insunits":
					d.BS(i16(int(e.dr.Doc.Units)))
				case "psntype":
					d.BS(psnType)
				default:
					d.BS(0)
				}
			case "BL":
				d.BL(0)
			case "BLL":
				d.BLL(0)
			case "BD":
				d.BD(0)
			case "RC":
				d.RC(0)
			case "2RD":
				d.RD2(0, 0)
			case "3BD":
				d.BD3(0, 0, 0)
			case "TV":
				if e.v >= v2007 {
					s.TU("")
				} else {
					s.T("")
				}
			case "CMC":
				d.BS(256)
				if e.v >= v2004 {
					bl(d, 0xC0<<24)
					d.RC(0)
				}
			case "HS":
				d.H(0, e.next)
			case "H":
				if name == "psn" && psnType != 3 {
					continue
				}
				h.H(5, map[string]uint64{
					"blockcontrol": hBlockControl, "layercontrol": hLayerControl,
					"paperspace": e.paper, "modelspace": e.model, "psn": hStyle,
				}[name])
			}
		}
	}
	return e.section(sentinelHeader, e.streams(d, s, h))
}
