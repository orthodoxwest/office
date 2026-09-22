package render

import (
	"html/template"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// elidedWords begin with an apostrophe that stands for dropped letters, not an
// opening quotation mark: "'Mid the Twelve", "'Tis". Compared lower-case.
var elidedWords = []string{"mid", "midst", "tis", "twas", "twere", "gainst", "neath", "tween", "twixt"}

// Typeset turns the corpus's typewriter punctuation into the printed forms a
// service book uses: ' and " become curly apostrophes and quotation marks, and
// a hyphen between two digits (a verse range, "Luke 1:46-55") becomes an en
// dash. It works on display text only; corpus keys, attestation hashes and the
// LaTeX booklet keep the source bytes.
func Typeset(s string) string {
	if !strings.ContainsAny(s, `'"-`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	var prev rune // previous rune written, 0 at the start
	for i, r := range s {
		next, _ := utf8.DecodeRuneInString(s[i+utf8.RuneLen(r):])
		switch r {
		case '\'':
			switch {
			case unicode.IsLetter(prev) || unicode.IsDigit(prev) || isClosingPunct(prev):
				// God's, angels', th' eternal, 'the Watchful'.
				b.WriteRune('’')
			case unicode.IsLetter(next) && startsElision(s[i+1:]):
				b.WriteRune('’')
			default:
				b.WriteRune('‘')
			}
		case '"':
			if prev == 0 || unicode.IsSpace(prev) || isOpeningPunct(prev) {
				b.WriteRune('“')
			} else {
				b.WriteRune('”')
			}
		case '-':
			if unicode.IsDigit(prev) && unicode.IsDigit(next) {
				b.WriteRune('–')
			} else {
				b.WriteRune(r)
			}
		default:
			b.WriteRune(r)
		}
		prev = r
	}
	return b.String()
}

func startsElision(rest string) bool {
	end := strings.IndexFunc(rest, func(r rune) bool { return !unicode.IsLetter(r) })
	if end < 0 {
		end = len(rest)
	}
	return slices.Contains(elidedWords, strings.ToLower(rest[:end]))
}

func isOpeningPunct(r rune) bool {
	return strings.ContainsRune("([{‘“—–-/", r)
}

func isClosingPunct(r rune) bool {
	return strings.ContainsRune(".,;:!?)]}’”", r)
}

// escText is the HTML escape for every run of display text in an hour.
func escText(s string) string {
	return template.HTMLEscapeString(Typeset(s))
}
