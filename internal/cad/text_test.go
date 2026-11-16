package cad

import (
	"slices"
	"testing"
)

func TestPlainText(t *testing.T) {
	tests := []struct{ in, want string }{
		{in: "", want: ""},
		{in: "Kitchen", want: "Kitchen"},
		{in: "45%%d", want: "45°"},
		{in: "45%%D", want: "45°"},
		{in: "%%c50", want: "⌀50"},
		{in: "%%C50", want: "⌀50"},
		{in: "%%p5", want: "±5"},
		{in: "%%P5", want: "±5"},
		{in: "%%uUnder%%U and %%oOver%%O", want: "Under and Over"},
		{in: "100%%%", want: "100%"},
		// %%nnn reads at most three digits, so a fourth is literal.
		{in: "%%065BC", want: "ABC"},
		{in: "%%0651", want: "A1"},
		{in: "%%9x", want: "\tx"},
		{in: "%%176C", want: "°C"},
		// Unknown codes and percent signs too near the end stay as written.
		{in: "%%x", want: "%%x"},
		{in: "50%", want: "50%"},
		{in: "a%%", want: "a%%"},
		{in: "a%b", want: "a%b"},
		{in: `20\U+00B0C`, want: "20°C"},
		{in: `caf\u+00e9`, want: "café"},
		{in: `\U+00B0%%c`, want: "°⌀"},
		// Malformed escapes are kept rather than half-decoded.
		{in: `\U+ZZZZ`, want: `\U+ZZZZ`},
		{in: `x\U+00`, want: `x\U+00`},
	}
	for _, tt := range tests {
		if got := PlainText(tt.in); got != tt.want {
			t.Errorf("PlainText(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMTextLines(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{in: "Room", want: []string{"Room"}},
		{in: `Line one\PLine two`, want: []string{"Line one", "Line two"}},
		{in: `{\fArial|b1|i0;Bold} text`, want: []string{"Bold text"}},
		{in: `\H2.5;Big\W0.8;Narrow`, want: []string{"BigNarrow"}},
		{in: `{\C1;red {\H2x;big}} end`, want: []string{"red big end"}},
		{in: `\A1;\Q15;\T1.1;\cFF0000;x`, want: []string{"x"}},
		{in: `1\S1^2; in`, want: []string{"11/2 in"}},
		{in: `\S3#4;`, want: []string{"3/4"}},
		{in: `\S1/8;`, want: []string{"1/8"}},
		// A stack with no closing semicolon runs to the end.
		{in: `\S1^2`, want: []string{"1/2"}},
		{in: `a\~b`, want: []string{"a b"}},
		{in: `\\ \{ \}`, want: []string{`\ { }`}},
		{in: `\Lunder\l \Oover\o \Kstrike\k`, want: []string{"under over strike"}},
		{in: `a  \Pb  `, want: []string{"a", "b"}},
		{in: `\P`, want: []string{"", ""}},
		{in: `90%%d\P\U+00B1`, want: []string{"90°", "±"}},
		// An argument code with no semicolon drops only the code.
		{in: `\Wfoo`, want: []string{"foo"}},
		{in: `a\`, want: []string{`a\`}},
	}
	for _, tt := range tests {
		if got := MTextLines(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("MTextLines(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
