// Package convert knows which files spaceweave reads, identifies them from
// their first bytes, and turns drawings (DXF, SVG) into the PNG the tracer
// reads and the SVG people look at.
//
// Formats is the one list of what is accepted: the server serves it to the
// page (which builds its file picker and its pre-upload check from it), and
// the inbox watcher and the upload API check files against it.
package convert

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// Format is a kind of file spaceweave reads.
type Format struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Exts   []string `json:"exts"`
	MIME   []string `json:"mime"`
	Detail string   `json:"detail,omitempty"`
	// Drawing formats are converted to an image before tracing.
	Drawing bool `json:"drawing"`
}

// The formats. Their IDs are stored in catalog.json and read by the site.
var (
	PNG  = Format{ID: "png", Label: "PNG", Exts: []string{".png"}, MIME: []string{"image/png"}}
	JPEG = Format{
		ID:    "jpeg",
		Label: "JPEG",
		Exts:  []string{".jpg", ".jpeg"},
		MIME:  []string{"image/jpeg"},
	}
	WebP = Format{ID: "webp", Label: "WebP", Exts: []string{".webp"}, MIME: []string{"image/webp"}}
	DXF  = Format{
		ID:    "dxf",
		Label: "DXF",
		Exts:  []string{".dxf"},
		MIME: []string{
			"image/vnd.dxf",
			"application/dxf",
			"image/x-dxf",
		},
		Detail:  "ASCII or binary",
		Drawing: true,
	}
	SVG = Format{
		ID:      "svg",
		Label:   "SVG",
		Exts:    []string{".svg"},
		MIME:    []string{"image/svg+xml"},
		Drawing: true,
	}
)

// Formats are the accepted formats, in the order the page lists them.
var Formats = []Format{PNG, JPEG, WebP, DXF, SVG}

// SniffLen is how much of a file Sniff needs. SVG is the greedy one: an
// exporter's comment and doctype can come before the <svg> tag.
const SniffLen = 8 << 10

// The kinds of refusal.
var (
	ErrUnsupported = errors.New("unsupported file")
	ErrMismatch    = errors.New("file content doesn't match its name")
)

// Refusal is why a file is refused, worded for the person who sent it. It
// unwraps to ErrUnsupported or ErrMismatch.
type Refusal struct {
	kind error
	msg  string
}

func (r *Refusal) Error() string { return r.msg }

func (r *Refusal) Unwrap() error { return r.kind }

func refuse(kind error, format string, a ...any) error {
	return &Refusal{kind: kind, msg: fmt.Sprintf(format, a...)}
}

// Names lists the accepted formats for messages: "PNG, JPEG, WebP, DXF or SVG".
func Names() string {
	labels := make([]string, len(Formats))
	for i, f := range Formats {
		labels[i] = f.Label
	}
	return strings.Join(labels[:len(labels)-1], ", ") + " or " + labels[len(labels)-1]
}

// ByExt finds the format for a file name's extension.
func ByExt(name string) (Format, bool) {
	ext := strings.ToLower(filepath.Ext(name))
	for _, f := range Formats {
		if slices.Contains(f.Exts, ext) {
			return f, true
		}
	}
	return Format{}, false
}

// ByID finds a format by its ID.
func ByID(id string) (Format, bool) {
	for _, f := range Formats {
		if f.ID == id {
			return f, true
		}
	}
	return Format{}, false
}

// Eligible reports whether a file name looks like something spaceweave
// reads. Dotfiles are skipped: they are usually partial downloads or OS
// metadata.
func Eligible(name string) bool {
	if strings.HasPrefix(filepath.Base(name), ".") {
		return false
	}
	_, ok := ByExt(name)
	return ok
}

// other are formats people try that spaceweave doesn't read, so a rejection
// can say what the file is and what to do instead.
var other = []struct {
	name, advice string
	match        func([]byte) bool
}{
	{"DWG", "Save it as DXF from your CAD program.", isDWG},
	{"PDF", "Export the page as PNG, or the drawing as DXF or SVG.", func(b []byte) bool {
		i := bytes.Index(b, []byte("%PDF-"))
		return i >= 0 && i < 1024
	}},
	{"DWF", "Export the drawing as DXF instead.", prefix("(DWF V")},
	{"TIFF", "Export it as PNG.", func(b []byte) bool {
		return bytes.HasPrefix(b, []byte("II*\x00")) || bytes.HasPrefix(b, []byte("MM\x00*"))
	}},
	{"GIF", "Export it as PNG.", prefix("GIF8")},
	{"BMP", "Export it as PNG.", prefix("BM")},
	{"HEIC", "Export it as JPEG.", func(b []byte) bool {
		return len(b) >= 12 && string(b[4:8]) == "ftyp" && strings.HasPrefix(string(b[8:12]), "hei")
	}},
	{"ZIP archive", "Unzip it and upload the drawing inside.", prefix("PK\x03\x04")},
}

func prefix(p string) func([]byte) bool {
	return func(b []byte) bool { return bytes.HasPrefix(b, []byte(p)) }
}

// isDWG matches AutoCAD's "AC10nn" version string, and the older "AC1.40"
// style.
func isDWG(b []byte) bool {
	return len(b) >= 6 && string(b[:2]) == "AC" && (b[2] == '1' || b[2] == '2') &&
		(isDigit(b[3]) || b[3] == '.') && isDigit(b[4]) && isDigit(b[5])
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// Sniff identifies a file from its first SniffLen bytes.
func Sniff(head []byte) (Format, error) {
	f, known, err := identify(head)
	if known {
		return f, nil
	}
	return Format{}, err
}

// identify is Sniff, also saying whether the content was recognised at all
// (a PDF is recognised and refused; random bytes aren't recognised).
func identify(head []byte) (Format, bool, error) {
	switch {
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return PNG, true, nil
	case bytes.HasPrefix(head, []byte{0xff, 0xd8, 0xff}):
		return JPEG, true, nil
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return WebP, true, nil
	case bytes.HasPrefix(head, []byte("AutoCAD Binary DXF\r\n\x1a\x00")), asciiDXF(head):
		return DXF, true, nil
	case isSVG(head):
		return SVG, true, nil
	}
	for _, o := range other {
		if o.match(head) {
			return Format{}, false, refuse(
				ErrUnsupported,
				"%s files aren't supported. %s",
				o.name,
				o.advice,
			)
		}
	}
	return Format{}, false, refuse(
		ErrUnsupported,
		"This isn't a file spaceweave reads. Use %s.",
		Names(),
	)
}

// recognised reports whether identify knew what the content was, accepted
// or not.
func recognised(f Format, err error) bool {
	var r *Refusal
	return f.ID != "" || (errors.As(err, &r) && !strings.HasPrefix(r.msg, "This isn't"))
}

// article puts "a" or "an" before a format's name, by how an acronym is
// spoken: "an SVG", "a PNG".
func article(label string) string {
	if label != strings.ToUpper(label) || !strings.ContainsRune("AEFHILMNORSX", rune(label[0])) {
		return "a " + label
	}
	return "an " + label
}

func trimBOM(b []byte) []byte { return bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")) }

// asciiDXF matches a "0 / SECTION" pair, after any 999 comment pairs.
func asciiDXF(head []byte) bool {
	lines := strings.Split(string(trimBOM(head)), "\n")
	for i := 0; i+1 < len(lines); i += 2 {
		code, val := strings.TrimSpace(lines[i]), strings.TrimSpace(lines[i+1])
		switch code {
		case "999":
			continue
		case "0":
			return val == "SECTION"
		default:
			return false
		}
	}
	return false
}

// isSVG matches an <svg> root after any XML declaration, comments, doctype
// and processing instructions.
func isSVG(head []byte) bool {
	s := strings.TrimSpace(string(trimBOM(head)))
	for {
		switch {
		case strings.HasPrefix(s, "<?"):
			s = after(s, "?>")
		case strings.HasPrefix(s, "<!--"):
			s = after(s, "-->")
		case strings.HasPrefix(s, "<!"):
			s = afterDoctype(s)
		default:
			return strings.HasPrefix(s, "<svg") && len(s) > 4 &&
				strings.ContainsRune(" \t\r\n>/", rune(s[4]))
		}
		if s == "" {
			return false
		}
	}
}

func after(s, end string) string {
	_, rest, ok := strings.Cut(s, end)
	if !ok {
		return ""
	}
	return strings.TrimSpace(rest)
}

// afterDoctype skips a doctype, which may hold an internal subset in [...].
func afterDoctype(s string) string {
	if b := strings.IndexByte(s, '['); b >= 0 && b < strings.IndexByte(s, '>') {
		s = after(s, "]")
	}
	return after(s, ">")
}

// Check identifies a file and checks that its name agrees with its content,
// returning a message for people when it doesn't.
func Check(name string, head []byte) (Format, error) {
	byName, known := ByExt(name)
	byContent, _, err := identify(head)
	if !known {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		switch {
		case ext == "":
			return Format{}, refuse(
				ErrUnsupported,
				"%s has no file extension. Use %s.",
				name,
				Names(),
			)
		case byContent.ID != "":
			return Format{}, refuse(ErrMismatch, "%s is %s file. Rename it to end in %s.",
				name, article(byContent.Label), byContent.Exts[0])
		case recognised(byContent, err):
			// The content says what it is: "PDF files aren't supported" is
			// more use than ".pdf files aren't".
			return Format{}, err
		}
		return Format{}, refuse(ErrUnsupported, ".%s files aren't supported. Use %s.", ext, Names())
	}
	if err != nil {
		return Format{}, refuse(
			ErrMismatch,
			"%s doesn't look like %s file. %s",
			name,
			article(byName.Label),
			err,
		)
	}
	if byContent.ID != byName.ID {
		return Format{}, refuse(
			ErrMismatch,
			"%s is named like %s file but is %s file. Rename it to end in %s.",
			name,
			article(byName.Label),
			article(byContent.Label),
			byContent.Exts[0],
		)
	}
	return byName, nil
}
