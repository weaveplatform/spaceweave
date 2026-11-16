package dwg

import (
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// defaultCodepage is Windows-1252: what an unknown or unsupported code page
// falls back to, and what most Western drawings use anyway.
var defaultCodepage encoding.Encoding = charmap.Windows1252

// codepages maps the DWGCODEPAGE number in an R2000–R2004 file to its
// encoding. The numbering is AutoCAD's own (30 is ANSI_1252); the order
// follows ACadSharp's code page table. Pages x/text lacks are left out
// and fall back to the default, which garbles only their accented letters.
var codepages = map[uint16]encoding.Encoding{
	2: charmap.ISO8859_1, 3: charmap.ISO8859_2, 4: charmap.ISO8859_3, 5: charmap.ISO8859_4,
	6: charmap.ISO8859_5, 7: charmap.ISO8859_6, 8: charmap.ISO8859_7, 9: charmap.ISO8859_8,
	10: charmap.ISO8859_9, 11: charmap.CodePage437, 12: charmap.CodePage850,
	13: charmap.CodePage852, 14: charmap.CodePage855, 16: charmap.CodePage860,
	18: charmap.CodePage863, 20: charmap.CodePage865, 22: japanese.ShiftJIS,
	23: charmap.Macintosh, 24: traditionalchinese.Big5, 25: korean.EUCKR,
	27: charmap.CodePage866, 28: charmap.Windows1250, 29: charmap.Windows1251,
	30: charmap.Windows1252, 31: simplifiedchinese.GBK, 32: charmap.Windows1253,
	33: charmap.Windows1254, 34: charmap.Windows1255, 35: charmap.Windows1256,
	36: charmap.Windows1257, 37: charmap.Windows874, 38: japanese.ShiftJIS,
	39: simplifiedchinese.GBK, 40: korean.EUCKR, 41: traditionalchinese.Big5,
	44: charmap.Windows1258,
}

func codepage(n uint16) encoding.Encoding {
	if e, ok := codepages[n]; ok {
		return e
	}
	return defaultCodepage
}
