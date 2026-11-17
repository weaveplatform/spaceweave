package profile

import (
	"strings"
	"unicode"
)

// Words splits a layer or block name into upper-case words: at anything
// that isn't a letter or digit, between letters and digits, and where a
// lower-case letter meets an upper-case one ("TaskChair", "DOOR900",
// "Ss40"). Rules' words and codes are matched against these, which is why
// "A-WALL", "a_wall" and "AWall" all have the word WALL.
func Words(name string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToUpper(string(cur)))
			cur = cur[:0]
		}
	}
	prev := rune(0)
	for _, r := range name {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case len(cur) > 0 && (unicode.IsDigit(r) != unicode.IsDigit(prev) ||
			unicode.IsUpper(r) && unicode.IsLower(prev)):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
		prev = r
	}
	flush()
	return out
}

// AnyWord reports whether any word fits any pattern: a word, or a prefix
// ending in *. Patterns are upper case, as Words returns.
func AnyWord(words, patterns []string) bool {
	for _, w := range words {
		for _, p := range patterns {
			if pre, ok := strings.CutSuffix(p, "*"); ok {
				if strings.HasPrefix(w, pre) {
					return true
				}
			} else if w == p {
				return true
			}
		}
	}
	return false
}

// hasCode reports whether the words hold a classification code starting
// with one of codes, as BS 1192 and ISO 19650 layer names carry a Uniclass
// 2015 one ("AR_EF_25_10-X_Walls": EF_25_10). At each word the code is
// that word and the ones after it joined by "_", so "EF_25_10" matches
// "Ef_25_10_30" and "EF_25_100" alike, and "Zz_" matches the Zz table
// with any number after it. A code needs two words: a name's last word is
// never one on its own.
func hasCode(words, codes []string) bool {
	for i := 0; i+1 < len(words); i++ {
		code := strings.Join(words[i:], "_")
		for _, c := range codes {
			if strings.HasPrefix(code, c) {
				return true
			}
		}
	}
	return false
}
