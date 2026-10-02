package buckrogers

import (
	"strings"
	"unicode/utf8"
)

// Buck repo spec 054 §3.1: Korean particles that depend on the last sound of
// the word before them.  The translations leave the parenthesised form
// (은(는), 이(가), 을(를), 와(과), (으)로) where the word is only known at run
// time; when the word before the mark is Korean, the final consonant decides.
// Everything here is only reached for LangKo.

// koParticleMarkers are the five marks.  With is the form after a syllable
// that has a final consonant, without the one after a syllable that has none.
var koParticleMarkers = []struct{ mark, with, without string }{
	{"은(는)", "은", "는"},
	{"이(가)", "이", "가"},
	{"을(를)", "을", "를"},
	{"와(과)", "과", "와"},
	{"(으)로", "으로", "로"},
}

// koMarkerAt reports the mark s starts with.
func koMarkerAt(s string) (int, bool) {
	for i, m := range koParticleMarkers {
		if strings.HasPrefix(s, m.mark) {
			return i, true
		}
	}
	return 0, false
}

// koPickParticle is the form of mark i after the syllable syl.  The final
// consonant is (code-0xAC00)%28: 0 is none, 8 is ㄹ, which takes the vowel
// form of 로 (and of nothing else).
func koPickParticle(i int, syl rune) string {
	m := koParticleMarkers[i]
	final := int(syl-0xAC00) % 28
	if i == len(koParticleMarkers)-1 { // (으)로
		if final == 0 || final == 8 {
			return m.without
		}
		return m.with
	}
	if final == 0 {
		return m.without
	}
	return m.with
}

// koReadingTail is the syllable the end of s is read as (spec 054 §3.1): the
// last character when it is a Hangul syllable, or, when it is the ")" that
// closes an English annotation (non-empty printable ASCII without
// parentheses, spec 038 §3.3) right after a Hangul syllable, that syllable.
// Latin letters, digits, punctuation and the empty string give 0.
func koReadingTail(s string) rune {
	r, n := utf8.DecodeLastRuneInString(s)
	if isHangulSyllable(r) {
		return r
	}
	if r != ')' {
		return 0
	}
	body := s[:len(s)-n]
	open := strings.LastIndexByte(body, '(')
	if open < 0 || open == len(body)-1 {
		return 0
	}
	for i := open + 1; i < len(body); i++ {
		if c := body[i]; c < 0x20 || c > 0x7E || c == ')' {
			return 0
		}
	}
	if before, _ := utf8.DecodeLastRuneInString(body[:open]); isHangulSyllable(before) {
		return before
	}
	return 0
}

// koResolveMarkers replaces every particle mark of s whose preceding word has
// a readable last syllable; the others stay as they are.  A mark is resolved
// against the text before it as already resolved, so a chain stays correct.
func koResolveMarkers(s string) string {
	if !strings.Contains(s, ")") {
		return s
	}
	var b strings.Builder
	changed := false
	for i := 0; i < len(s); {
		if k, ok := koMarkerAt(s[i:]); ok {
			if syl := koReadingTail(b.String()); syl != 0 {
				b.WriteString(koPickParticle(k, syl))
				i += len(koParticleMarkers[k].mark)
				changed = true
				continue
			}
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		b.WriteString(s[i : i+n])
		i += n
	}
	if !changed {
		return s
	}
	return b.String()
}

// koResolveLeading resolves only a mark that starts s, against the syllable
// the text before the call was read as (ECL continuation calls, spec 054
// §3.2).  The second result reports a replacement.
func koResolveLeading(s string, syl rune) (string, bool) {
	if syl == 0 {
		return s, false
	}
	k, ok := koMarkerAt(s)
	if !ok {
		return s, false
	}
	return koPickParticle(k, syl) + s[len(koParticleMarkers[k].mark):], true
}
