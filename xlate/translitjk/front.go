package translitjk

import (
	"strings"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// Buck repo spec 044 §3.3: the front end shared by the Japanese and the
// Korean back ends, from the phonemes of one word to syllables.

// Pos says where a stand-alone consonant is in the word.
type Pos int

const (
	PosInitial Pos = iota // in the cluster before the first vowel
	PosMedial             // in a cluster between two vowels
	PosFinal              // in the cluster after the last vowel
)

// Cons is a consonant that does not combine with a vowel.
type Cons struct {
	Sym string
	Pos Pos
}

// Syl is one syllable: the vowel nucleus with the single consonant in front
// of it.
type Syl struct {
	Pre    []Cons         // stand-alone consonants before Onset
	Onset  string         // consonant right before the vowel ("" none; never NG)
	Pal    bool           // Onset is followed by Y and then the vowel (C+Y+V)
	Nuc    translit.Phone // the vowel
	Group  string         // the letters the vowel is spelled with ("" unknown)
	FromER bool           // an unstressed ER before a stressed vowel that F1 rewrote as AH (Marie, Maria)
	R      bool           // an R after the vowel (before a consonant or the end) is absorbed
	Post   []Cons         // last syllable only: the final cluster
}

// letterOf is the letter of the vowel group the back ends read: the last
// letter, the one before it when that is w, "" when it is not a vowel letter.
func (s Syl) letterOf() string {
	g := s.Group
	if g == "" {
		return ""
	}
	l := g[len(g)-1]
	if l == 'w' && len(g) > 1 {
		l = g[len(g)-2]
	}
	if strings.IndexByte("aeiouy", l) < 0 {
		return ""
	}
	return string(l)
}

// startsWithO reports whether the vowel group starts with o (F2: AA).
func (s Syl) startsWithO() bool { return strings.HasPrefix(s.Group, "o") }

func (s Syl) stressed() bool { return s.Nuc.Stress >= 1 }

// shortVowel lists the short vowels (spec 044 J6, spec 045 K6).
func shortVowel(sym string) bool {
	switch sym {
	case "AE", "EH", "IH", "AH", "AA", "UH":
		return true
	}
	return false
}

// Syllabify implements F1 to F8.  letters is the lower case spelling without
// apostrophes and hyphens.  It reports false for a word without a vowel (F8).
func Syllabify(phones []translit.Phone, letters string) ([]Syl, bool) {
	groups := translit.AlignVowels(letters, phones) // nil: no letters (F2)
	type item struct {
		p     translit.Phone
		group string
		er    bool
	}
	var items []item
	vi := 0
	for i, x := range phones {
		g := ""
		if x.Vowel {
			if groups != nil && vi < len(groups) {
				g = groups[vi]
			}
			vi++
		}
		items = append(items, item{p: x, group: g})
		// F1: ER before a vowel keeps its r as the head of the next syllable.
		if x.Sym == "ER" && i+1 < len(phones) && phones[i+1].Vowel {
			if x.Stress == 0 {
				items[len(items)-1].p.Sym = "AH"
				// Marie, Maria: the vowel after the r is stressed (before an
				// unstressed one, Barbara and Margaret, the ER stays a schwa)
				items[len(items)-1].er = phones[i+1].Stress >= 1
			}
			items = append(items, item{p: translit.Phone{Sym: "R", Stress: -1}})
		}
	}
	// F6: a final T S or D Z reads as one sound.
	if n := len(items); n >= 2 {
		a, b := items[n-2].p, items[n-1].p
		if !a.Vowel && !b.Vowel {
			switch {
			case a.Sym == "T" && b.Sym == "S":
				items = append(items[:n-2], item{p: translit.Phone{Sym: "TS", Stress: -1}})
			case a.Sym == "D" && b.Sym == "Z":
				items = append(items[:n-2], item{p: translit.Phone{Sym: "DZ", Stress: -1}})
			}
		}
	}
	hasVowel := false
	for _, it := range items {
		if it.p.Vowel {
			hasVowel = true
		}
	}
	if !hasVowel {
		return nil, false
	}

	var syls []Syl
	var pend []Cons
	for i := 0; i < len(items); i++ {
		it := items[i]
		if !it.p.Vowel {
			pend = append(pend, Cons{Sym: it.p.Sym})
			continue
		}
		s := Syl{Nuc: it.p, Group: it.group, FromER: it.er}
		pos := PosMedial
		if len(syls) == 0 {
			pos = PosInitial
		}
		pre := pend
		if n := len(pend); n > 0 && pend[n-1].Sym != "NG" {
			last := pend[n-1].Sym
			if last == "Y" && n >= 2 && pend[n-2].Sym != "NG" && pend[n-2].Sym != "Y" {
				s.Onset, s.Pal, pre = pend[n-2].Sym, true, pend[:n-2]
			} else {
				s.Onset, pre = last, pend[:n-1]
			}
		}
		for _, c := range pre {
			s.Pre = append(s.Pre, Cons{c.Sym, pos})
		}
		pend = nil
		// F5: an R after the vowel, before a consonant or the end, is absorbed.
		if i+1 < len(items) && !items[i+1].p.Vowel && items[i+1].p.Sym == "R" &&
			(i+2 >= len(items) || !items[i+2].p.Vowel) {
			s.R = true
			i++
		}
		syls = append(syls, s)
	}
	for _, c := range pend {
		syls[len(syls)-1].Post = append(syls[len(syls)-1].Post, Cons{c.Sym, PosFinal})
	}
	return syls, true
}

// nextConsonant is the consonant that follows the stand-alone consonant
// pre[j] of syllable i, "" when a vowel follows (spec 044 §3.3).
func nextAfterPre(s Syl, j int) string {
	if j+1 < len(s.Pre) {
		return s.Pre[j+1].Sym
	}
	return s.Onset
}

func nextAfterPost(s Syl, k int) string {
	if k+1 < len(s.Post) {
		return s.Post[k+1].Sym
	}
	return ""
}

// vowelDirectlyAfter reports whether a vowel follows syllable i with no
// consonant between.
func vowelDirectlyAfter(syls []Syl, i int) bool {
	if i+1 >= len(syls) {
		return false
	}
	n := syls[i+1]
	return len(n.Pre) == 0 && n.Onset == "" && len(syls[i].Post) == 0
}

// vowelDirectlyBefore reports whether syllable i follows a vowel with no
// consonant between.
func vowelDirectlyBefore(syls []Syl, i int) bool {
	return i > 0 && len(syls[i].Pre) == 0 && syls[i].Onset == ""
}
