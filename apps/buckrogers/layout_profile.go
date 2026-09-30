package buckrogers

// Buck repo spec 042 §3.4: per-language line-breaking rules.  A nil profile
// is the pre-042 behaviour (zh-TW and zh-CN): only the fixed closing set of
// isEclClosing never starts a line and there is no line-end rule.  The ja
// profile adds Japanese kinsoku: extra characters that never start a line
// (small kana, the long-vowel mark, iteration marks, the middle dot,
// closing brackets, the em dash of a "——" pair) and opening brackets that
// never end a line.
//
// Buck repo spec 043 §3.4 adds word-level wrapping for Korean: with word
// set, a maximal run of non-space characters is one token, a name unit and
// the particle glued to it merge when they fit a line, and a line or a
// whole call that cannot be laid out that way falls back to the character
// level of this file's earlier profiles.

type LayoutProfile struct {
	noLineStart map[rune]bool  // never start a line; attach to the token before
	noLineEnd   map[rune]bool  // never end a line; join the token after
	word        bool           // word-level tokens (spec 043 §3.4)
	chars       *LayoutProfile // the same profile at character level (set with word)
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

// layoutKoChars is the Korean profile at character level: only the opening
// quotation marks never end a line (spec 043 §3.4).  The closing set of
// isEclClosing already covers the ASCII punctuation and 」 』 that Korean
// text uses.
var layoutKoChars = newLayoutProfile("", "「『")

// layoutKo is the Korean word-level profile; the fallbacks use layoutKoChars.
var layoutKo = func() *LayoutProfile {
	p := newLayoutProfile("", "「『")
	p.word = true
	p.chars = layoutKoChars
	return p
}()

// LayoutFor returns the layout profile of a language; nil means the
// default rules.
func LayoutFor(lang string) *LayoutProfile {
	switch lang {
	case LangJa:
		return layoutJa
	case LangKo:
		return layoutKo
	}
	return nil
}

// charLevel is the profile the word-level fallbacks use: the same rules
// with character tokens.  A profile without word level is its own fallback.
func (p *LayoutProfile) charLevel() *LayoutProfile {
	if p == nil || !p.word || p.chars == nil {
		return p
	}
	return p.chars
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

// splitSearch is how far back (in half units, as a fraction of the first
// row's n1 cells) splitRows looks for a space when the split falls inside a
// word (spec 043 §3.4).
func splitSearch(n1 int) int { return n1 / 2 }

// splitRows divides the Chinese or translated text r of a fragment that the
// original broke over two rows of n1 and n2 cells (spec 029 §2.10).  ok is
// false when the second part does not fit its row.  A profile without word
// level (nil, ja, zh-CN) keeps the pre-043 rule exactly: break at the last
// character that fits, then adjustBreak.  With word level a break inside a
// word moves back to the nearest space within splitSearch units and the
// space itself is shown on neither row; a break that falls on a space drops
// that space.  If the moved split leaves the second part too wide the
// original split stays.
func (p *LayoutProfile) splitRows(r []rune, n1, n2 int) (first, second []rune, ok bool) {
	k := p.adjustBreak(r, fitUnits(r, 2*n1))
	if p == nil || !p.word || k <= 0 || k >= len(r) {
		return r[:k], r[k:], textUnits(r[k:]) <= 2*n2
	}
	switch {
	case r[k] == ' ':
		rest := r[k+1:]
		return r[:k], rest, textUnits(rest) <= 2*n2
	case r[k-1] == ' ':
		return r[:k], r[k:], textUnits(r[k:]) <= 2*n2
	}
	depth := splitSearch(n1)
	for s := k - 1; s >= 0 && textUnits(r[s:k]) <= depth; s-- {
		if r[s] == ' ' {
			if rest := r[s+1:]; textUnits(rest) <= 2*n2 {
				return r[:s], rest, true
			}
			break
		}
	}
	return r[:k], r[k:], textUnits(r[k:]) <= 2*n2
}
