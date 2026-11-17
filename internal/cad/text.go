package cad

import (
	"strconv"
	"strings"
)

// Special characters written as %% codes in TEXT and MTEXT.
var percent = map[byte]string{'d': "°", 'D': "°", 'c': "⌀", 'C': "⌀", 'p': "±", 'P': "±"}

// PlainText decodes a TEXT value: %%d, %%c and %%p become °, ⌀ and ±,
// %%nnn is a character code, and the underline and overline toggles
// (%%u, %%o) are dropped.
func PlainText(s string) string {
	s = unicodeEscapes(s)
	if !strings.Contains(s, "%%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+2 >= len(s) || s[i+1] != '%' {
			b.WriteByte(s[i])
			continue
		}
		c := s[i+2]
		switch {
		case percent[c] != "":
			b.WriteString(percent[c])
			i += 2
		case c == 'u' || c == 'U' || c == 'o' || c == 'O':
			i += 2
		case c == '%':
			b.WriteByte('%')
			i += 2
		case c >= '0' && c <= '9':
			j := i + 2
			for j < len(s) && j < i+5 && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			n, _ := strconv.Atoi(s[i+2 : j])
			b.WriteRune(rune(n)) //nolint:gosec // three digits at most, so ≤ 999
			i = j - 1
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// unicodeEscapes decodes \U+XXXX, which drawings before 2007 use for
// characters outside their code page.
func unicodeEscapes(s string) string {
	if !strings.Contains(s, `\U+`) && !strings.Contains(s, `\u+`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i+7 <= len(s) && s[i] == '\\' && (s[i+1] == 'U' || s[i+1] == 'u') && s[i+2] == '+' {
			if n, err := strconv.ParseUint(s[i+3:i+7], 16, 32); err == nil {
				b.WriteRune(rune(n)) //nolint:gosec // four hex digits, so ≤ 0xFFFF
				i += 6
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// mtextArgCodes take an argument running to the next semicolon. \p is
// paragraph indents and alignment ("\pxql;" left-aligns), which shows as
// "xql;" in front of the words when it isn't stripped.
const mtextArgCodes = "ACcFfHQTWp"

// MTextLines strips MTEXT formatting and splits the text into lines. Font,
// height, colour and width changes are dropped (the tracer reads the words,
// not their styling), and a stacked fraction a^b or a#b becomes a/b.
func MTextLines(s string) []string {
	s = PlainText(s)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '{' || c == '}':
		case c != '\\' || i+1 >= len(s):
			b.WriteByte(c)
		default:
			i++
			code := s[i]
			switch {
			case code == 'P':
				b.WriteByte('\n')
			case code == '~':
				b.WriteByte(' ')
			case code == '\\' || code == '{' || code == '}':
				b.WriteByte(code)
			case code == 'S':
				end := strings.IndexByte(s[i:], ';')
				if end < 0 {
					end = len(s) - i
				}
				stack := s[i+1 : i+end]
				b.WriteString(strings.NewReplacer("^", "/", "#", "/").Replace(stack))
				i += end
			case strings.IndexByte(mtextArgCodes, code) >= 0:
				if end := strings.IndexByte(s[i:], ';'); end >= 0 {
					i += end
				}
			default:
				// \L, \l, \O, \o, \K, \k: underline, overline and strike
				// toggles with nothing to show here.
			}
		}
	}
	lines := strings.Split(b.String(), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}
