package convert

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "write convert.Formats into the shared sniff cases")

// casesFile is shared with the site's tests (test/formats.test.js), so the
// page's pre-upload check and the server's can't drift apart.
const casesFile = "../../web/floorplan-iso/test/sniff-cases.json"

type want struct {
	ID      string `json:"id,omitempty"`
	Refused string `json:"refused,omitempty"`
	OK      bool   `json:"ok,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type cases struct {
	Comment string   `json:"comment"`
	Formats []Format `json:"formats"`
	Sniff   []struct {
		Name string `json:"name"`
		Head string `json:"head"`
		Want want   `json:"want"`
	} `json:"sniff"`
	Check []struct {
		File string `json:"file"`
		Head string `json:"head"`
		Want want   `json:"want"`
	} `json:"check"`
}

// bytesOf turns a case's head back into bytes: one character per byte.
func bytesOf(s string) []byte {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		b = append(b, byte(r)) //nolint:gosec // the cases hold Latin-1 only
	}
	return b
}

func loadCases(t *testing.T) cases {
	t.Helper()
	raw, err := os.ReadFile(casesFile)
	if err != nil {
		t.Fatal(err)
	}
	var c cases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	if *update {
		c.Formats = Formats
		out, err := json.MarshalIndent(c, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(casesFile, append(out, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func TestSharedCasesListTheFormats(t *testing.T) {
	c := loadCases(t)
	if !reflect.DeepEqual(c.Formats, Formats) {
		t.Fatalf(
			"the shared cases list other formats than convert.Formats; run go test ./internal/convert -run Shared -update",
		)
	}
}

func TestSniff(t *testing.T) {
	for _, tc := range loadCases(t).Sniff {
		f, err := Sniff(bytesOf(tc.Head))
		switch {
		case tc.Want.ID != "":
			if err != nil || f.ID != tc.Want.ID {
				t.Errorf("%s: got %q, %v; want %s", tc.Name, f.ID, err, tc.Want.ID)
			}
		default:
			if !errors.Is(err, ErrUnsupported) || err.Error() != tc.Want.Refused {
				t.Errorf("%s: got %v\n want %s", tc.Name, err, tc.Want.Refused)
			}
		}
	}
}

func TestCheck(t *testing.T) {
	for _, tc := range loadCases(t).Check {
		f, err := Check(tc.File, bytesOf(tc.Head))
		if tc.Want.OK {
			if err != nil || f.ID != tc.Want.ID {
				t.Errorf("%s: got %q, %v", tc.File, f.ID, err)
			}
			continue
		}
		var r *Refusal
		if !errors.As(err, &r) || err.Error() != tc.Want.Reason {
			t.Errorf("%s: got %v\n want %s", tc.File, err, tc.Want.Reason)
		}
	}
}

func TestCheckKinds(t *testing.T) {
	_, err := Check("plan.jpg", bytesOf("\x89PNG\r\n\x1a\n"))
	if !errors.Is(err, ErrMismatch) {
		t.Errorf("a renamed image is a mismatch: %v", err)
	}
	_, err = Check("plan.txt", []byte("hello"))
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("an unknown extension is unsupported: %v", err)
	}
}

func TestRegistry(t *testing.T) {
	if got := Names(); got != "PNG, JPEG, WebP, DXF, DWG or SVG" {
		t.Errorf("Names() = %q", got)
	}
	for _, name := range []string{"a.png", "B.JPEG", "c.jpg", "d.webp", "e.DXF", "f.svg", "g.DWG"} {
		if !Eligible(name) {
			t.Errorf("%s should be eligible", name)
		}
	}
	for _, name := range []string{".hidden.png", "a.gif", "a.dwf", "noext"} {
		if Eligible(name) {
			t.Errorf("%s should not be eligible", name)
		}
	}
	if f, ok := ByID("svg"); !ok || !f.Drawing {
		t.Errorf("ByID(svg) = %+v, %v", f, ok)
	}
	if f, ok := ByID("dwg"); !ok || !f.Drawing {
		t.Errorf("ByID(dwg) = %+v, %v", f, ok)
	}
	if _, ok := ByID("dwf"); ok {
		t.Error("dwf isn't a format")
	}
	for _, f := range Formats {
		if f.ID == "" || f.Label == "" || len(f.Exts) == 0 || len(f.MIME) == 0 {
			t.Errorf("incomplete format %+v", f)
		}
		for _, e := range f.Exts {
			if e != strings.ToLower(e) || !strings.HasPrefix(e, ".") {
				t.Errorf("%s: extension %q must be lower case with a dot", f.ID, e)
			}
		}
	}
}

func TestArticle(t *testing.T) {
	for label, want := range map[string]string{"SVG": "an SVG", "PNG": "a PNG", "DXF": "a DXF", "WebP": "a WebP", "HEIC": "an HEIC", "DWG": "a DWG"} {
		if got := article(label); got != want {
			t.Errorf("article(%q) = %q", label, got)
		}
	}
}
