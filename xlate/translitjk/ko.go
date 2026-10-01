package translitjk

import "strings"

// Buck repo spec 045: syllables to Hangul (rules K1 to K10).

// Jamo indexes of the Unicode Hangul syllable block.
const (
	oG  = 0
	oN  = 2
	oD  = 3
	oR  = 5
	oM  = 6
	oB  = 7
	oS  = 9
	oNG = 11
	oJ  = 12
	oCH = 14
	oK  = 15
	oT  = 16
	oP  = 17
	oH  = 18

	vA   = 0
	vAE  = 1
	vYA  = 2
	vYAE = 3
	vEO  = 4
	vE   = 5
	vYEO = 6
	vYE  = 7
	vO   = 8
	vWA  = 9
	vWAE = 10
	vYO  = 12
	vU   = 13
	vWO  = 14
	vWE  = 15
	vWI  = 16
	vYU  = 17
	vEU  = 18
	vI   = 20

	cNone = 0
	cG    = 1
	cN    = 4
	cL    = 8
	cM    = 16
	cB    = 17
	cS    = 19
	cNG   = 21
)

type hsyl struct{ on, nu, co int }

type hangul struct{ s []hsyl }

func (h *hangul) add(on, nu int) { h.s = append(h.s, hsyl{on, nu, cNone}) }

func (h *hangul) lastFree() bool { return len(h.s) > 0 && h.s[len(h.s)-1].co == cNone }

func (h *hangul) setCoda(co int) { h.s[len(h.s)-1].co = co }

func (h *hangul) String() string {
	var b strings.Builder
	for _, x := range h.s {
		b.WriteRune(rune(0xAC00 + (x.on*21+x.nu)*28 + x.co))
	}
	return b.String()
}

// koOnset is K1: the head consonant.
var koOnset = map[string]int{
	"P": oP, "B": oB, "T": oT, "D": oD, "K": oK, "G": oG, "F": oP, "V": oB, "TH": oS, "DH": oD, "S": oS, "Z": oJ,
	"CH": oCH, "JH": oJ, "ZH": oJ, "M": oM, "N": oN, "L": oR, "R": oR, "HH": oH,
}

// koSHVowel is the K1 table of SH before a vowel.
func koSHVowel(s Syl) int {
	switch s.Nuc.Sym {
	case "AA", "AY", "AW":
		return vYA
	case "AE":
		return vYAE
	case "AH":
		if s.Nuc.Stress == 0 && s.letterOf() == "a" {
			return vYA
		}
		return vYEO
	case "ER":
		return vYEO
	case "EH", "EY":
		return vYE
	case "AO", "OW", "OY":
		return vYO
	case "UW", "UH":
		return vYU
	}
	return vI
}

// koNucleus is K2 for a vowel after an ordinary onset: the main vowel and the
// second syllable of a diphthong (-1 when there is none).
func koNucleus(s Syl, onset string, last bool, tr *tracer) (v, second int) {
	second = -1
	switch s.Nuc.Sym {
	case "IY", "IH":
		return vI, -1
	case "EH":
		return vE, -1
	case "AE":
		return vAE, -1
	case "AA":
		if s.startsWithO() {
			tr.add("K2.AA.o")
			return vO, -1
		}
		return vA, -1
	case "AH":
		if s.Nuc.Stress > 0 {
			return vEO, -1
		}
		tr.add("K2.AH.0")
		switch s.letterOf() {
		case "i":
			return vI, -1
		case "o":
			if onset == "S" && len(s.Post) > 0 && s.Post[0].Sym == "N" {
				tr.add("K2.AH.son")
				return vEU, -1
			}
		case "a":
			if last && len(s.Post) == 0 {
				return vA, -1
			}
		}
		return vEO, -1
	case "AO", "OW":
		return vO, -1
	case "UW", "UH":
		return vU, -1
	case "ER":
		return vEO, -1
	case "EY":
		return vE, vI
	case "AW":
		return vA, vU
	case "AY":
		return vA, vI
	case "OY":
		return vO, vI
	}
	return vEO, -1
}

func koYVowel(s Syl) (v, second int) {
	second = -1
	switch s.Nuc.Sym {
	case "AA", "AE":
		v = vYA
	case "AH", "ER":
		v = vYEO
	case "EH":
		v = vYE
	case "UW", "UH":
		v = vYU
	case "AO", "OW":
		v = vYO
	case "EY":
		v, second = vYE, vI
	case "AY":
		v, second = vYA, vI
	case "AW":
		v, second = vYA, vU
	case "OY":
		v, second = vYO, vI
	default:
		v = vI
	}
	return
}

func koWVowel(s Syl) (v, second int) {
	second = -1
	switch s.Nuc.Sym {
	case "AA":
		v = vWA
	case "AE":
		v = vWAE
	case "AH", "AO", "ER":
		v = vWO
	case "EH":
		v = vWE
	case "IH", "IY":
		v = vWI
	case "UW", "UH", "OW":
		v = vU
	case "EY":
		v, second = vWE, vI
	case "AY":
		v, second = vWA, vI
	case "AW":
		v, second = vWA, vU
	case "OY":
		v, second = vWO, vI
	default:
		v = vWO
	}
	return
}

// koRender renders the syllables of one word.  letters is the lower case
// spelling (K7: the final Z).
func koRender(syls []Syl, letters string, tr *tracer) (string, bool) {
	var h hangul
	for i, s := range syls {
		last := i == len(syls)-1
		for j, c := range s.Pre {
			next := nextAfterPre(s, j)
			afterShort := j == 0 && i > 0 && shortVowel(syls[i-1].Nuc.Sym) && !syls[i-1].R
			koStandalone(&h, c.Sym, next, afterShort, letters, false, tr)
		}
		koOnsetVowel(&h, syls, i, tr)
		if last {
			for k, c := range s.Post {
				next := nextAfterPost(s, k)
				afterShort := k == 0 && shortVowel(s.Nuc.Sym) && !s.R
				koStandalone(&h, c.Sym, next, afterShort, letters, next == "", tr)
			}
		}
	}
	r := h.String()
	return r, r != ""
}

// koOnsetVowel writes the onset and the vowel of syllable i (K1 to K5).
func koOnsetVowel(h *hangul, syls []Syl, i int, tr *tracer) {
	s := syls[i]
	last := i == len(syls)-1
	onset := s.Onset
	// K5: L before a vowel doubles the final of the syllable before it.
	if onset == "L" {
		wordInitial := i == 0 && len(s.Pre) == 0
		prevM := false
		if len(s.Pre) > 0 {
			ps := s.Pre[len(s.Pre)-1].Sym
			prevM = ps == "M" || ps == "N" || ps == "NG"
		}
		if !wordInitial && !prevM && h.lastFree() {
			h.setCoda(cL)
			tr.add("K5")
		}
	}
	switch {
	case onset == "" || onset == "NG":
		v, second := koNucleus(s, "", last, tr)
		h.add(oNG, v)
		koSecond(h, second)
		tr.add(koTrace(s))
	case onset == "Y":
		v, second := koYVowel(s)
		h.add(oNG, v)
		koSecond(h, second)
		tr.add("K2.Y")
	case onset == "W":
		v, second := koWVowel(s)
		h.add(oNG, v)
		koSecond(h, second)
		tr.add("K2.W")
	case onset == "SH":
		v := koSHVowel(s)
		second := -1
		switch s.Nuc.Sym {
		case "EY", "AY", "OY":
			second = vI
		case "AW":
			second = vU
		}
		h.add(oS, v)
		koSecond(h, second)
		tr.add("K1.SH")
	case s.Pal:
		koPal(h, s, onset, last, tr)
	default:
		on, ok := koOnset[onset]
		if !ok {
			on = oNG
		}
		v, second := koNucleus(s, onset, last, tr)
		h.add(on, v)
		koSecond(h, second)
		tr.add("K1." + koOnsetRule(onset))
		tr.add(koTrace(s))
	}
	if s.R {
		switch s.Nuc.Sym {
		case "EH", "IH", "IY", "UH", "UW":
			h.add(oNG, vEO)
			tr.add("K4.eo")
		default:
			tr.add("K4.drop")
		}
	}
}

func koSecond(h *hangul, second int) {
	if second >= 0 {
		h.add(oNG, second)
	}
}

func koOnsetRule(c string) string {
	switch c {
	case "JH", "ZH":
		return "JHZH"
	case "L", "R":
		return "LR"
	}
	return c
}

func koTrace(s Syl) string {
	switch s.Nuc.Sym {
	case "IY", "IH":
		return "K2.IY"
	case "AO", "OW":
		return "K2.AO"
	case "UW", "UH":
		return "K2.UW"
	case "AW", "AY", "OY":
		return "K2.AWAYOY"
	}
	return "K2." + s.Nuc.Sym
}

// koPal is K3: C+Y+V.
func koPal(h *hangul, s Syl, onset string, last bool, tr *tracer) {
	on, ok := koOnset[onset]
	if !ok {
		on = oNG
	}
	plain := on == oJ || on == oCH // after ㅈ ㅊ the y vowels read as plain vowels
	if s.Nuc.Sym == "UW" || s.Nuc.Sym == "UH" || (s.Nuc.Sym == "AH" && s.Nuc.Stress == 0 && s.letterOf() == "u") {
		v := vYU
		if plain {
			v = vU
		}
		h.add(on, v)
		tr.add("K3.UW")
		return
	}
	if s.Nuc.Sym == "AH" && s.Nuc.Stress == 0 {
		h.add(on, vI)
		v, _ := koNucleus(s, onset, last, nil)
		h.add(oNG, v)
		tr.add("K3.AH0")
		return
	}
	v, second := koYVowel(s)
	if plain {
		v = map[int]int{vYA: vA, vYEO: vEO, vYO: vO, vYU: vU, vYE: vE}[v]
		if v == 0 && s.Nuc.Sym != "AA" && s.Nuc.Sym != "AE" {
			v = vI
		}
	}
	h.add(on, v)
	koSecond(h, second)
	tr.add("K3.y")
}

// koStandalone writes a consonant that is not followed by a vowel (K6 to K8).
func koStandalone(h *hangul, c, next string, afterShort bool, letters string, wordFinal bool, tr *tracer) {
	switch c {
	case "P", "T", "K":
		coda := map[string]int{"P": cB, "T": cS, "K": cG}[c]
		if afterShort && h.lastFree() && next != "L" && next != "R" && next != "M" && next != "N" {
			h.setCoda(coda)
			tr.add("K6.coda")
			return
		}
		h.add(koOnset[c], vEU)
		tr.add("K6.eu")
	case "M", "N", "NG", "L":
		coda := map[string]int{"M": cM, "N": cN, "NG": cNG, "L": cL}[c]
		if h.lastFree() {
			h.setCoda(coda)
			tr.add("K8.coda")
			return
		}
		switch c {
		case "NG":
			h.add(oNG, vEU)
			h.setCoda(cNG)
		default:
			h.add(koOnset[c], vEU)
		}
		tr.add("K8.eu")
	case "SH":
		if next == "" {
			h.add(oS, vI)
		} else {
			h.add(oS, vYU)
		}
		tr.add("K7.SH")
	case "CH":
		h.add(oCH, vI)
		tr.add("K7.CH")
	case "JH", "ZH":
		h.add(oJ, vI)
		tr.add("K7.JH")
	case "TS":
		h.add(oCH, vEU)
		tr.add("K7.TS")
	case "DZ":
		h.add(oJ, vEU)
		tr.add("K7.DZ")
	case "Z":
		if next == "" && strings.HasSuffix(letters, "s") {
			h.add(oS, vEU)
			tr.add("K7.Z.s")
		} else {
			h.add(oJ, vEU)
			tr.add("K7.Z")
		}
	case "HH":
		h.add(oH, vEU)
		tr.add("K7.HH")
	case "Y":
		h.add(oNG, vI)
		tr.add("K2.Y.end")
	case "W":
		h.add(oNG, vU)
		tr.add("K2.W.end")
	case "R":
		h.add(oR, vEU)
		tr.add("K2.R.alone")
	default:
		if on, ok := koOnset[c]; ok {
			h.add(on, vEU)
			tr.add("K7.eu")
		}
	}
}

// koRules lists the rule ids of the Korean back end.
func koRules() []string {
	r := []string{
		"K1.SH", "K2.AA.o", "K2.AH.0", "K2.AH.son", "K2.IY", "K2.EH", "K2.AE", "K2.AA", "K2.AH", "K2.AO", "K2.UW", "K2.ER", "K2.EY", "K2.AWAYOY",
		"K2.Y", "K2.W", "K2.Y.end", "K2.W.end", "K2.R.alone",
		"K3.UW", "K3.AH0", "K3.y", "K4.eo", "K4.drop", "K5",
		"K6.coda", "K6.eu", "K7.SH", "K7.CH", "K7.JH", "K7.TS", "K7.DZ", "K7.Z", "K7.Z.s", "K7.HH", "K7.eu",
		"K8.coda", "K8.eu", "K10",
	}
	for c := range koOnset {
		r = append(r, "K1."+koOnsetRule(c))
	}
	return dedupSort(r)
}
