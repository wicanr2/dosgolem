package buckrogers

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Buck repo spec 046: how the translations of the pieces of one sentence are
// joined in the languages that need more than "put them next to each other"
// (Korean separates words by spaces; Japanese does not leave a space between
// a Latin name and a particle).  Everything here is only reached for LangKo
// and LangJa; the other languages keep the pre-046 code paths.

// koPunct are the punctuation marks that stick to the text before them.
const koPunct = ",.!?:;)」』"

// koWordMarks are the particles, copulas and endings that stick to the word
// before them (spec 046 §3.2).  The bare 이 is not in the list: at the start
// of a fragment it is the demonstrative "this" (it is in KO_LATIN_SUFFIXES of
// tools/lang_check.py, which checks static adjacency instead).
var koWordMarks = []string{
	"은(는)", "이(가)", "을(를)", "와(과)", "(으)로",
	"의", "에", "에서", "에게", "에게는", "에게서", "에는", "에서의", "까지", "도", "만",
	"들", "들이", "들은", "들을", "들에게",
	"입니다", "이다", "이며", "이라고", "이라는", "이라면", "이에요", "형",
	"은", "는", "을", "를", "과", "와", "로", "으로", "가",
}

func isHangulSyllable(r rune) bool { return r >= 0xAC00 && r <= 0xD7A3 }

// koWordSide reports whether r, the last character before a fragment, is one
// a particle can attach to.
func koWordSide(r rune) bool {
	return isHangulSyllable(r) || r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r)) || r == ')' || r == '」' || r == '』'
}

// koMarkEnds reports whether rest, what follows a mark, ends the word: the
// end of the output, a space or a punctuation mark.
func koMarkEnds(rest string) bool {
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return r == ' ' || strings.ContainsRune(koPunct, r)
}

// koGlue is the spec 046 §3.2 test: a is the last character before out (the
// output of the next piece); true means no space goes between them.
func koGlue(a rune, out string) bool {
	if out == "" {
		return false
	}
	// A punctuation mark followed by the end or a space sticks to anything
	// ("..." does not: the mark is followed by another ".").
	if r, n := utf8.DecodeRuneInString(out); strings.ContainsRune(koPunct, r) {
		if rest := out[n:]; rest == "" || rest[0] == ' ' {
			return true
		}
	}
	if !koWordSide(a) {
		return false
	}
	for _, m := range koWordMarks {
		if strings.HasPrefix(out, m) && koMarkEnds(out[len(m):]) {
			return true
		}
	}
	return false
}

func lastRuneOf(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

func isHiragana(r rune) bool { return r >= 0x3041 && r <= 0x309F }

// jaOrdinaryWords start with a single-kana particle but are ordinary words.
var jaOrdinaryWords = []string{"でき", "とき", "ところ", "とても"}

// jaParticleStart reports whether a Japanese fragment output starts with a
// particle (spec 046 §3.2): は が を に の と で へ always (except the ordinary
// words), も や から まで より only when no hiragana follows.
func jaParticleStart(out string) bool {
	if out == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(out)
	if strings.ContainsRune("はがをにのとでへ", r) {
		for _, w := range jaOrdinaryWords {
			if strings.HasPrefix(out, w) {
				return false
			}
		}
		return true
	}
	for _, m := range []string{"も", "や", "から", "まで", "より"} {
		if strings.HasPrefix(out, m) {
			rest := out[len(m):]
			if rest == "" {
				return true
			}
			next, _ := utf8.DecodeRuneInString(rest)
			if !isHiragana(next) {
				return true
			}
		}
	}
	return false
}

// joinKo joins the outputs of the parts of a decomposed string for Korean
// (spec 046 §3.2): a space at a boundary where the original has one, unless
// the output already has it or the next piece sticks to the text before it.
func joinKo(parts []EnginePart, zh []string) string {
	var b strings.Builder
	for i, p := range parts {
		z := zh[i]
		if i > 0 && z != "" && zh[i-1] != "" {
			prev := parts[i-1]
			space := strings.HasSuffix(prev.Text, " ") || strings.HasPrefix(p.Text, " ")
			switch {
			case !space:
			case strings.HasSuffix(zh[i-1], " ") || strings.HasPrefix(z, " "):
			case p.Kind == 'F' && koGlue(lastRuneOf(zh[i-1]), z):
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteString(z)
	}
	return b.String()
}

// joinJa is the pre-046 join with one change (spec 046 §3.2): a fragment that
// starts with a particle after a name ending in a Latin letter gets no space.
func joinJa(parts []EnginePart, zh []string) string {
	var b strings.Builder
	for i, p := range parts {
		z := zh[i]
		if p.Kind == 'F' {
			if strings.HasPrefix(p.Text, " ") && i > 0 && parts[i-1].Kind == '_' && len(zh[i-1]) > 0 && engineAlnum(zh[i-1][len(zh[i-1])-1]) {
				last := zh[i-1][len(zh[i-1])-1]
				if !(engineAlpha(last) && jaParticleStart(z)) {
					z = " " + z
				}
			}
			if strings.HasSuffix(p.Text, " ") && i+1 < len(parts) && parts[i+1].Kind == '_' && len(zh[i+1]) > 0 && engineAlnum(zh[i+1][0]) {
				z += " "
			}
		}
		b.WriteString(z)
	}
	return b.String()
}

var (
	templatePlaceholder = regexp.MustCompile(`\{[0-9]+\}`)
	digitsDot           = regexp.MustCompile(`^[0-9]+\.$`)
)

// templateRefsUnique reports whether no placeholder of a template appears
// twice (checked at load for ko and ja, spec 046 §3.3).
func templateRefsUnique(t string) bool {
	seen := map[string]bool{}
	for _, m := range templatePlaceholder.FindAllString(t, -1) {
		if seen[m] {
			return false
		}
		seen[m] = true
	}
	return true
}

// fillTemplateKoJa fills the {i} of a template with the outputs of the parts
// slots names (spec 046 §3.3).  A value that comes from a plain slot and is
// digits plus one period ("38.") loses the period when text follows the
// placeholder, and the sentence then ends with a terminator of the language.
func fillTemplateKoJa(lang, t string, slots []int, parts []EnginePart, zh []string) string {
	var b strings.Builder
	stripped := false
	for i := 0; i < len(t); {
		loc := templatePlaceholder.FindStringIndex(t[i:])
		if loc == nil {
			b.WriteString(t[i:])
			break
		}
		b.WriteString(t[i : i+loc[0]])
		ph := t[i+loc[0] : i+loc[1]]
		end := i + loc[1]
		idx := 0
		for _, c := range ph[1 : len(ph)-1] {
			idx = idx*10 + int(c-'0')
		}
		switch {
		case idx >= len(slots):
			b.WriteString(ph) // out of range: stays literal, as before
		default:
			v := strings.TrimSpace(zh[slots[idx]])
			if parts[slots[idx]].Kind == '_' && digitsDot.MatchString(v) && end < len(t) {
				v = strings.TrimSuffix(v, ".")
				stripped = true
			}
			b.WriteString(v)
		}
		i = end
	}
	out := b.String()
	if !stripped {
		return out
	}
	terminators, add := ".!?」』", "."
	if lang == LangJa {
		terminators, add = "。！？」』…", "。"
	}
	if r := lastRuneOf(out); !strings.ContainsRune(terminators, r) {
		out += add
	}
	return out
}
