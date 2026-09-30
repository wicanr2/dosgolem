package buckrogers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// Buck repo spec 043 §3.4: the pre-043 layout code, frozen.
//
// frozenLayoutEclTextP, frozenLayoutLogbookUnitsP and frozenSplit are copies
// of layoutEclTextP, layoutLogbookUnitsP and the joinWrapped split as they
// were at commit 88f391f (before word-level wrapping).  Nothing here may be
// edited to follow later changes: the expected digests below were computed
// from the real functions at 88f391f on this same corpus, so a matching
// digest shows the copies are faithful.  Other tests then require the
// current code to give the same answers as the copies for every profile
// without word level.

type eclLayoutFn func(prof *LayoutProfile, text []rune, units []NameUnit, row, col, left, right, bottom uint8) ([]EclTextLine, uint8, uint8, bool)

func frozenLayoutEclTextP(prof *LayoutProfile, text []rune, units []NameUnit, row, col, left, right, bottom uint8) ([]EclTextLine, uint8, uint8, bool) {
	width := int(right) - int(left) + 1
	var tokens [][]rune
	for i, u := 0, 0; i < len(text); {
		for u < len(units) && units[u].End <= i {
			u++
		}
		if u < len(units) && units[u].Start == i {
			j := units[u].End
			for j < len(text) && prof.closing(text[j]) {
				j++
			}
			tok := text[i:j]
			if textUnits(tok) <= width {
				tokens = append(tokens, tok)
			} else {
				from := 0
				for k := 1; k < units[u].End-i; k++ {
					if tok[k] == ' ' {
						tokens = append(tokens, tok[from:k])
						from = k
					}
				}
				tokens = append(tokens, tok[from:])
			}
			i = j
			continue
		}
		j := i + 1
		if isEclLatin(text[i]) {
			for j < len(text) && isEclLatin(text[j]) {
				j++
			}
		}
		for j < len(text) && prof.closing(text[j]) {
			j++
		}
		if u < len(units) && j > units[u].Start {
			j = max(units[u].Start, i+1)
		}
		tokens = append(tokens, text[i:j])
		i = j
	}
	tokens = prof.joinOpening(tokens, width)
	var lines []EclTextLine
	cur := EclTextLine{Row: row, Col: col}
	used := int(col) - int(left)
	for _, t := range tokens {
		if used+textUnits(t) > width && used > 0 {
			for len(cur.Text) > 0 && cur.Text[len(cur.Text)-1] == ' ' {
				cur.Text = cur.Text[:len(cur.Text)-1]
			}
			if len(cur.Text) > 0 {
				lines = append(lines, cur)
			}
			row++
			cur = EclTextLine{Row: row, Col: left}
			used = 0
			if t[0] == ' ' {
				t = t[1:]
			}
		}
		if row > bottom || textUnits(t) > width {
			return nil, 0, 0, false
		}
		cur.Text = append(cur.Text, t...)
		used += textUnits(t)
	}
	if len(cur.Text) > 0 {
		lines = append(lines, cur)
	}
	end := uint8(int(left) + used)
	if row > bottom {
		return nil, 0, 0, false
	}
	return lines, row, end, true
}

func frozenLayoutLogbookUnitsP(prof *LayoutProfile, body []rune, units []NameUnit) ([][]string, error) {
	var lines []string
	for start := 0; start <= len(body); {
		end := start
		for end < len(body) && !(body[end] == '\\' && end+1 < len(body) && body[end+1] == 'n') {
			end++
		}
		a, b := start, end
		for a < b && isLogbookSpace(body[a]) {
			a++
		}
		for b > a && isLogbookSpace(body[b-1]) {
			b--
		}
		start = end + 2
		if a == b {
			continue
		}
		var us []NameUnit
		for _, u := range units {
			if u.Start >= a && u.End <= b {
				us = append(us, NameUnit{u.Start - a, u.End - a})
			}
		}
		ls, _, _, ok := frozenLayoutEclTextP(prof, body[a:b], us, 0, 0, 0, logbookBodyUnits-1, 255)
		if !ok {
			return nil, fmt.Errorf("buckrogers: 手札段落無法排版")
		}
		for _, l := range ls {
			lines = append(lines, string(l.Text))
		}
	}
	var pages [][]string
	for len(lines) > 0 {
		n := logbookBodyRows
		if n > len(lines) {
			n = len(lines)
		}
		pages = append(pages, lines[:n])
		lines = lines[n:]
	}
	if len(pages) == 0 || len(pages) > logbookMaxPages {
		return nil, fmt.Errorf("buckrogers: 手札頁數 %d 超出範圍", len(pages))
	}
	return pages, nil
}

// frozenSplit is the two-row split of joinWrapped as it was: break at the
// last character that fits, adjustBreak, and the second row must fit.
func frozenSplit(p *LayoutProfile, r []rune, n1, n2 int) (first, second []rune, ok bool) {
	k := p.adjustBreak(r, fitUnits(r, 2*n1))
	return r[:k], r[k:], textUnits(r[k:]) <= 2*n2
}

// freezeProfileMap holds the profiles whose behaviour must not change: the
// default (zh-TW, zh-CN), ja, and the Korean character-level profile.
var freezeProfileMap = map[string]*LayoutProfile{
	"nil":    nil,
	"ja":     layoutJa,
	"koChar": newLayoutProfile("", "「『"),
}

var freezeProfileOrder = []string{"nil", "ja", "koChar"}

type freezeCase struct {
	text                          []rune
	units                         []NameUnit
	row, col, left, right, bottom uint8
}

// freezeCorpus is a fixed pseudo-random set of layout calls.
func freezeCorpus() []freezeCase {
	seed := uint64(0x9e3779b97f4a7c15)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int((seed >> 33) % uint64(n))
	}
	pieces := []string{"안녕하세요", "여러분", "반갑습니다", "가", "나다", "NEO", "RAM", "x.y", "it's", "a-b", "12345",
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ", "かな", "ー", "・", "ぁ", "漢字", "拯救地球", "。", "，", "「", "」", "『", "』",
		"（", "）", "...", "…", "——", ".", ",", "!", "?", ":", ";", ")", "가나다라마바사아자차카타파하", "벅 로저스(BUCK ROGERS)"}
	seps := []string{"", "", " ", " ", " ", "  "}
	widths := []int{4, 6, 8, 10, 14, 20, 30, 40, 56, 72, 76}
	var out []freezeCase
	for n := 0; n < 4000; n++ {
		var text []rune
		for p, np := 0, 1+next(14); p < np; p++ {
			text = append(text, []rune(pieces[next(len(pieces))])...)
			text = append(text, []rune(seps[next(len(seps))])...)
		}
		var units []NameUnit
		for pos := next(6); pos < len(text) && len(units) < 3; {
			l := 2 + next(9)
			if pos+l > len(text) {
				break
			}
			if next(2) == 0 {
				units = append(units, NameUnit{pos, pos + l})
			}
			pos += l + next(10)
		}
		left := uint8(next(4))
		right := left + uint8(widths[next(len(widths))]-1)
		col := left + uint8(next(int(right-left)/2+1))
		row := uint8(next(6))
		bottoms := []uint8{row, row + 1, row + 2, row + 5, 255}
		out = append(out, freezeCase{text, units, row, col, left, right, bottoms[next(len(bottoms))]})
	}
	return out
}

// freezeLogbookCorpus is a fixed set of logbook bodies, some near the
// three-page limit.
func freezeLogbookCorpus() [][]rune {
	seed := uint64(0x1234567887654321)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int((seed >> 33) % uint64(n))
	}
	words := []string{"안녕하세요", "여러분", "가나다라마바사아자차카타파하", "NEO", "it's", "漢字の", "「이름」", "...", "!", "벅 로저스"}
	var out []([]rune)
	for n := 0; n < 300; n++ {
		var b strings.Builder
		for p, np := 0, 1+next(4)*next(40); p < np; p++ {
			b.WriteString(words[next(len(words))])
			switch next(9) {
			case 0:
				b.WriteString(`\n`)
			case 1: // the next word follows without a separator
			default:
				b.WriteString(" ")
			}
		}
		out = append(out, []rune(b.String()))
	}
	// Long ones: 35-syllable words, 58 of them, around the three-page edge.
	w := strings.Repeat("가", 35)
	for k := 50; k <= 62; k++ {
		out = append(out, []rune(strings.TrimSpace(strings.Repeat(w+" ", k))))
	}
	return out
}

func freezeEclDigest(fn eclLayoutFn) string {
	h := sha256.New()
	for ci, c := range freezeCorpus() {
		for _, name := range freezeProfileOrder {
			lines, row, end, ok := fn(freezeProfileMap[name], c.text, c.units, c.row, c.col, c.left, c.right, c.bottom)
			fmt.Fprintf(h, "%d|%s|%v|%d|%d|", ci, name, ok, row, end)
			for _, l := range lines {
				fmt.Fprintf(h, "%d:%d:%q;", l.Row, l.Col, string(l.Text))
			}
			fmt.Fprintln(h)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func freezeLogbookDigest(fn func(*LayoutProfile, []rune, []NameUnit) ([][]string, error)) string {
	h := sha256.New()
	for bi, b := range freezeLogbookCorpus() {
		for _, name := range freezeProfileOrder {
			pages, err := fn(freezeProfileMap[name], b, nil)
			fmt.Fprintf(h, "%d|%s|%v|", bi, name, err != nil)
			for _, p := range pages {
				fmt.Fprintf(h, "%q;", p)
			}
			fmt.Fprintln(h)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func freezeSplitDigest(fn func(*LayoutProfile, []rune, int, int) ([]rune, []rune, bool)) string {
	h := sha256.New()
	seed := uint64(0xdeadbeefcafef00d)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int((seed >> 33) % uint64(n))
	}
	pieces := []string{"안녕", "하세요", "여러분", "가", "NEO", "RAM", "漢字", "かな", "ー", "「", "」", ".", "!", " ", " ", " ", "...", "）", "（"}
	for n := 0; n < 3000; n++ {
		var r []rune
		for p, np := 0, 2+next(10); p < np; p++ {
			r = append(r, []rune(pieces[next(len(pieces))])...)
		}
		n1, n2 := 1+next(14), 1+next(14)
		for _, name := range freezeProfileOrder {
			a, b, ok := fn(freezeProfileMap[name], r, n1, n2)
			fmt.Fprintf(h, "%d|%s|%q|%q|%v\n", n, name, string(a), string(b), ok)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Digests of the real pre-043 functions (layoutEclTextP, layoutLogbookUnitsP,
// adjustBreak with the joinWrapped check) at commit 88f391f on the corpora
// above.  Do not edit.
const (
	freezeEclWant     = "737b436e6ad14ada83fc1eba2eaedfbaf1cb61e69cbc962086d4883047e59128"
	freezeLogbookWant = "1ba5978486f43628ca54a8c37667770e5be072e215de15d12fc06e7fdf58dd36"
	freezeSplitWant   = "0589e18c6f21ee00493471630afbe1177828f734412c5e8fb24c3ee0531cbf6f"
)

func TestFrozenCopiesMatchPre043(t *testing.T) {
	cases, books := len(freezeCorpus()), len(freezeLogbookCorpus())
	t.Logf("語料：ECL %d 組、手札 %d 則、拆分 3000 組；每組 %d 個 profile", cases, books, len(freezeProfileOrder))
	if got := freezeEclDigest(frozenLayoutEclTextP); got != freezeEclWant {
		t.Errorf("ECL 凍結副本摘要 %s，應為 %s", got, freezeEclWant)
	}
	if got := freezeLogbookDigest(frozenLayoutLogbookUnitsP); got != freezeLogbookWant {
		t.Errorf("手札凍結副本摘要 %s，應為 %s", got, freezeLogbookWant)
	}
	if got := freezeSplitDigest(frozenSplit); got != freezeSplitWant {
		t.Errorf("拆分凍結副本摘要 %s，應為 %s", got, freezeSplitWant)
	}
}
