package buckrogers

// Buck repo spec 042 §3.4: per-language line-breaking rules.  A nil profile
// is the pre-042 behaviour (zh-TW and zh-CN): only the fixed closing set of
// isEclClosing never starts a line and there is no line-end rule.  The ja
// profile adds Japanese kinsoku: extra characters that never start a line
// (small kana, the long-vowel mark, iteration marks, the middle dot,
// closing brackets, the em dash of a "——" pair) and opening brackets that
// never end a line.

type LayoutProfile struct {
	noLineStart map[rune]bool // never start a line; attach to the token before
	noLineEnd   map[rune]bool // never end a line; join the token after
}

func newLayoutProfile(noStart, noEnd string) *LayoutProfile {
	p := &LayoutProfile{noLineStart: map[rune]bool{}, noLineEnd: map[rune]bool{}}
	for _, r := range noStart {
		p.noLineStart[r] = true
	}
	for _, r := range noEnd {
		p.noLineEnd[r] = true
	}
	return p
}

// layoutJa is the spec 042 §3.4 table.  The pair "……" needs no entry: … is
// already in isEclClosing; the em dash is listed so a "——" pair sticks to
// the character before it as one token.
var layoutJa = newLayoutProfile(
	"ぁぃぅぇぉっゃゅょゎゕゖ"+"ァィゥェォッャュョヮヵヶ"+"ーゝゞヽヾ々〻・"+"】〕］｝〉〙〗’”．—",
	"「『（【〔［｛〈《‘“")

// LayoutFor returns the layout profile of a language; nil means the
// default rules.
func LayoutFor(lang string) *LayoutProfile {
	if lang == LangJa {
		return layoutJa
	}
	return nil
}

// closing reports whether r never starts a line.
func (p *LayoutProfile) closing(r rune) bool {
	return isEclClosing(r) || (p != nil && p.noLineStart[r])
}

// opening reports whether r never ends a line.
func (p *LayoutProfile) opening(r rune) bool {
	return p != nil && p.noLineEnd[r]
}

// joinOpening merges every token that ends with an opening bracket into the
// token after it, unless the merged token would be wider than a line (then
// the old behaviour stays rather than failing the layout).
func (p *LayoutProfile) joinOpening(tokens [][]rune, width int) [][]rune {
	if p == nil || len(p.noLineEnd) == 0 {
		return tokens
	}
	out := make([][]rune, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		for i+1 < len(tokens) && len(t) > 0 && p.opening(t[len(t)-1]) && textUnits(t)+textUnits(tokens[i+1]) <= width {
			merged := make([]rune, 0, len(t)+len(tokens[i+1]))
			merged = append(append(merged, t...), tokens[i+1]...)
			t = merged
			i++
		}
		out = append(out, t)
	}
	return out
}

// adjustBreak moves a two-row split point k (the first rune of the second
// row is r[k]) earlier while the second row would start with a closing
// character or the first row would end with an opening bracket.
func (p *LayoutProfile) adjustBreak(r []rune, k int) int {
	if p == nil {
		return k
	}
	for k > 0 && k < len(r) && (p.closing(r[k]) || p.opening(r[k-1])) {
		k--
	}
	return k
}
