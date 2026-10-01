package translitjk

import "strings"

// Buck repo spec 044 §3.4: syllables to katakana (rules J1 to J9).

type tracer struct{ ids []string }

func (t *tracer) add(id string) {
	if t != nil {
		t.ids = append(t.ids, id)
	}
}

var jaAlone = [5]string{"ア", "イ", "ウ", "エ", "オ"}

// jaHead is the J2 table: head consonant and vowel column (ア イ ウ エ オ).
var jaHead = map[string][5]string{
	"K": {"カ", "キ", "ク", "ケ", "コ"}, "G": {"ガ", "ギ", "グ", "ゲ", "ゴ"},
	"S": {"サ", "シ", "ス", "セ", "ソ"}, "Z": {"ザ", "ジ", "ズ", "ゼ", "ゾ"},
	"T": {"タ", "ティ", "トゥ", "テ", "ト"}, "D": {"ダ", "ディ", "ドゥ", "デ", "ド"},
	"N": {"ナ", "ニ", "ヌ", "ネ", "ノ"}, "HH": {"ハ", "ヒ", "フ", "ヘ", "ホ"},
	"B": {"バ", "ビ", "ブ", "ベ", "ボ"}, "P": {"パ", "ピ", "プ", "ペ", "ポ"},
	"M": {"マ", "ミ", "ム", "メ", "モ"}, "R": {"ラ", "リ", "ル", "レ", "ロ"}, "L": {"ラ", "リ", "ル", "レ", "ロ"},
	"F": {"ファ", "フィ", "フ", "フェ", "フォ"}, "V": {"バ", "ビ", "ブ", "ベ", "ボ"},
	"W": {"ワ", "ウィ", "ウ", "ウェ", "ウォ"}, "Y": {"ヤ", "イ", "ユ", "イェ", "ヨ"},
	"SH": {"シャ", "シ", "シュ", "シェ", "ショ"}, "CH": {"チャ", "チ", "チュ", "チェ", "チョ"},
	"JH": {"ジャ", "ジ", "ジュ", "ジェ", "ジョ"}, "ZH": {"ジャ", "ジ", "ジュ", "ジェ", "ジョ"},
	"TH": {"サ", "シ", "ス", "セ", "ソ"}, "DH": {"ザ", "ジ", "ズ", "ゼ", "ゾ"},
}

// jaIcol is the J3 table: the イ-column character of the consonant of C+Y+V.
var jaIcol = map[string]string{
	"K": "キ", "G": "ギ", "HH": "ヒ", "B": "ビ", "P": "ピ", "M": "ミ", "N": "ニ", "L": "リ", "R": "リ", "V": "ビ", "F": "フ",
	"T": "テ", "D": "デ", "S": "シ", "Z": "ジ", "TH": "シ", "SH": "シ", "CH": "チ", "JH": "ジ", "ZH": "ジ", "DH": "ジ",
}

// jaTail is the J4 table: a consonant that is not followed by a vowel.
var jaTail = map[string]string{
	"T": "ト", "D": "ド", "CH": "チ", "JH": "ジ", "ZH": "ジ", "SH": "シュ", "F": "フ", "HH": "フ", "V": "ブ",
	"TH": "ス", "DH": "ズ", "R": "ル", "L": "ル", "W": "ウ", "Y": "イ", "TS": "ツ", "DZ": "ズ",
	"K": "ク", "G": "グ", "S": "ス", "Z": "ズ", "B": "ブ", "P": "プ",
}

// jaVowel is J1: the column and the suffix of a vowel phoneme.
func jaVowelCol(s Syl) (col int, suffix string) {
	switch s.Nuc.Sym {
	case "IY":
		return 1, "ー"
	case "IH":
		col = 1
		if s.Nuc.Stress == 0 {
			switch s.letterOf() {
			case "e":
				col = 3
			case "a", "u":
				col = 0
			case "o":
				col = 4
			}
		}
		return col, ""
	case "EY":
		return 3, "イ"
	case "EH":
		return 3, ""
	case "AE":
		return 0, ""
	case "AA":
		if s.startsWithO() {
			return 4, ""
		}
		return 0, ""
	case "AH":
		if s.Nuc.Stress > 0 {
			return 0, ""
		}
		switch s.letterOf() {
		case "e":
			return 3, ""
		case "i", "y":
			return 1, ""
		case "o":
			return 4, ""
		}
		return 0, ""
	case "AO":
		return 4, "ー"
	case "OW":
		if s.Nuc.Stress > 0 {
			return 4, "ー"
		}
		return 4, ""
	case "UW":
		return 2, "ー"
	case "UH":
		return 2, ""
	case "AW":
		return 0, "ウ"
	case "AY":
		return 0, "イ"
	case "OY":
		return 4, "イ"
	case "ER":
		return 0, "ー"
	}
	return 0, ""
}

func jaTrace(s Syl) string {
	switch s.Nuc.Sym {
	case "AW", "AY", "OY":
		return "J1.AWAYOY"
	}
	return "J1." + s.Nuc.Sym
}

// jaRender renders the syllables of one word.
func jaRender(syls []Syl, tr *tracer) (string, bool) {
	r := jaClean(jaRenderRaw(syls, tr))
	return r, r != ""
}

// jaRenderRaw is jaRender before the J8 clean-up.
func jaRenderRaw(syls []Syl, tr *tracer) string {
	var out strings.Builder
	ngOnset := false // an NG before a vowel was written as ン; the vowel takes the G row
	for i, s := range syls {
		for j, c := range s.Pre {
			next := nextAfterPre(s, j)
			initial := i == 0 && j == 0
			if c.Sym == "NG" && next == "" {
				out.WriteString("ン")
				tr.add("J4.NG.vowel")
				ngOnset = true
				continue
			}
			out.WriteString(jaStandalone(c.Sym, next, initial, tr))
		}
		onset := s.Onset
		if ngOnset && onset == "" {
			onset = "G"
		}
		ngOnset = false
		out.WriteString(jaSyllable(syls, i, onset, tr))
		if i == len(syls)-1 {
			out.WriteString(jaPost(s, tr))
		}
	}
	return out.String()
}

// jaSyllable writes the onset and the vowel of syllable i (J1, J2, J3, J5, J7).
func jaSyllable(syls []Syl, i int, onset string, tr *tracer) string {
	s := syls[i]
	col, suffix := jaVowelCol(s)
	tr.add(jaTrace(s))
	if s.Nuc.Sym == "AA" && s.startsWithO() {
		tr.add("J1.AA.o")
	}
	if s.Nuc.Sym == "IH" && s.Nuc.Stress == 0 && s.letterOf() != "" && s.letterOf() != "i" && s.letterOf() != "y" {
		tr.add("J1.IH.letter")
	}
	if s.Nuc.Sym == "AH" && s.Nuc.Stress == 0 {
		tr.add("J1.AH.0")
	}
	if s.Nuc.Sym == "OW" && s.Nuc.Stress == 0 {
		tr.add("J1.OW.0")
	}
	// EH spelled with a before R+vowel reads as AE (J1).
	asAE := s.Nuc.Sym == "AE"
	if s.Nuc.Sym == "EH" && s.letterOf() == "a" && i+1 < len(syls) &&
		len(syls[i+1].Pre) == 0 && syls[i+1].Onset == "R" && len(s.Post) == 0 {
		col, asAE = 0, true
		tr.add("J1.EH.a")
	}
	// J7: the long vowel mark only before a consonant or the end of the word.
	if suffix == "ー" {
		switch {
		case s.Nuc.Sym == "IY" && s.Nuc.Stress == 0 && (onset == "T" || onset == "D") && i == len(syls)-1 && len(s.Post) == 0:
			suffix = ""
			tr.add("J1.IY.TD")
		case vowelDirectlyAfter(syls, i):
			suffix = ""
			tr.add("J7.nolong")
		case s.Nuc.Sym == "IY" && vowelDirectlyBefore(syls, i):
			suffix = ""
			tr.add("J7.nolong")
		}
	}
	// J5: an absorbed R.
	if s.R {
		suffix = jaRSuffix(s, suffix, tr)
	}
	if s.Pal && onset != "" {
		return jaPal(s, onset, col, suffix, tr)
	}
	var base string
	switch {
	case onset == "":
		base = jaAlone[col]
	default:
		row, ok := jaHead[onset]
		if !ok {
			base = jaAlone[col]
			break
		}
		base = row[col]
		tr.add("J2." + jaHeadRule(onset))
		if asAE && (onset == "K" || onset == "G") {
			base = map[string]string{"K": "キャ", "G": "ギャ"}[onset]
			tr.add("J1.AE.KG")
		}
		if onset == "Y" && s.Nuc.Sym == "IY" {
			tr.add("J2.YIY")
		}
		if onset == "Y" && s.Nuc.Sym == "UW" {
			tr.add("J2.YUW")
		}
		if onset == "W" && s.Nuc.Sym == "UW" {
			tr.add("J2.WUW")
		}
	}
	return base + suffix
}

func jaHeadRule(c string) string {
	switch c {
	case "R", "L":
		return "RL"
	case "JH", "ZH":
		return "JHZH"
	}
	return c
}

func jaRSuffix(s Syl, suffix string, tr *tracer) string {
	switch s.Nuc.Sym {
	case "AA", "AO", "OW", "AE", "AH":
		tr.add("J5.long")
		return "ー"
	case "EH":
		tr.add("J5.EH")
		return "ア"
	case "IH", "IY":
		tr.add("J5.IH")
		return "ア"
	case "UH", "UW":
		tr.add("J5.UH")
		return "ア"
	case "AY", "AW", "EY", "OY":
		tr.add("J5.diph")
		return suffix + "ア"
	}
	return suffix
}

// jaPal is J3: C+Y+V.
func jaPal(s Syl, onset string, col int, suffix string, tr *tracer) string {
	ic, ok := jaIcol[onset]
	if !ok {
		ic = jaHead["Y"][1]
	}
	small := ""
	v := s.Nuc.Sym
	switch {
	case v == "UW" || v == "UH":
		small = "ュ"
		tr.add("J3.UW")
	case v == "AH" && s.Nuc.Stress == 0:
		switch s.letterOf() {
		case "u":
			small = "ュ"
			tr.add("J3.AHu")
		case "i", "y":
			tr.add("J3.AH0")
		default:
			ic += jaAlone[col]
			tr.add("J3.AH0")
		}
		col = -1
	case v == "AA" || v == "AE" || v == "AH" || v == "ER" || v == "AY" || v == "AW":
		small = "ャ"
		tr.add("J3.ya")
	case v == "EH" || v == "EY":
		small = "ェ"
		tr.add("J3.ye")
	case v == "AO" || v == "OW" || v == "OY":
		small = "ョ"
		tr.add("J3.yo")
	default:
		tr.add("J3.yi")
	}
	if onset == "F" && small == "ュ" {
		return "フュ" + suffix
	}
	if col == -1 {
		// unstressed AH: no suffix of its own
		return ic + small
	}
	return ic + small + suffix
}

// jaStandalone is J4 for a consonant of Pre.
func jaStandalone(c, next string, initial bool, tr *tracer) string {
	switch c {
	case "N":
		if initial && next != "" {
			tr.add("J4.N.initial")
			return "ヌ"
		}
		tr.add("J4.N")
		return "ン"
	case "M":
		if !initial && (next == "B" || next == "P" || next == "F") {
			tr.add("J4.M.BPF")
			return "ン"
		}
		tr.add("J4.M")
		return "ム"
	case "NG":
		if next == "K" || next == "G" || next == "T" {
			tr.add("J4.NG.KGT")
			return "ン"
		}
		tr.add("J4.NG")
		return "ング"
	}
	if t, ok := jaTail[c]; ok {
		tr.add("J4." + c)
		return t
	}
	return ""
}

// jaPost writes the final cluster with the geminate of J6.
func jaPost(s Syl, tr *tracer) string {
	var out strings.Builder
	gem := false
	if len(s.Post) > 0 && !s.R {
		p0 := s.Post[0].Sym
		if s.stressed() && shortVowel(s.Nuc.Sym) {
			switch p0 {
			case "K", "P", "T", "CH", "JH", "D", "TS", "DZ":
				gem = true
				tr.add("J6.a")
			}
		} else if !s.stressed() && (s.Nuc.Sym == "AH" || s.Nuc.Sym == "IH") {
			if len(s.Post) == 1 && (p0 == "K" || p0 == "P" || p0 == "T" || p0 == "D") ||
				len(s.Post) == 2 && p0 == "K" && s.Post[1].Sym == "S" {
				gem = true
				tr.add("J6.b")
			}
		}
	}
	for k, c := range s.Post {
		next := nextAfterPost(s, k)
		if k == 0 && gem {
			out.WriteString("ッ")
		}
		out.WriteString(jaStandalone(c.Sym, next, false, tr))
	}
	return out.String()
}

// jaClean enforces J8.
func jaClean(s string) string {
	for strings.Contains(s, "ーー") {
		s = strings.ReplaceAll(s, "ーー", "ー")
	}
	for _, bad := range []string{"ンー", "ッー", "ーッ"} {
		for strings.Contains(s, bad) {
			s = strings.ReplaceAll(s, bad, string([]rune(bad)[0]))
		}
	}
	for {
		r := []rune(s)
		if len(r) == 0 || (r[0] != 'ッ' && r[0] != 'ー' && r[0] != 'ン') {
			break
		}
		s = string(r[1:])
	}
	return s
}

// jaRules lists the rule ids of the Japanese back end.
func jaRules() []string {
	r := []string{
		"J1.IY", "J1.IH", "J1.EY", "J1.EH", "J1.AE", "J1.AA", "J1.AH", "J1.AO", "J1.OW", "J1.UW", "J1.UH", "J1.AWAYOY", "J1.ER",
		"J1.IY.TD", "J1.IH.letter", "J1.EH.a", "J1.AE.KG", "J1.AA.o", "J1.AH.0", "J1.OW.0",
		"J2.YIY", "J2.YUW", "J2.WUW",
		"J3.UW", "J3.AHu", "J3.AH0", "J3.ya", "J3.ye", "J3.yo", "J3.yi",
		"J4.N", "J4.N.initial", "J4.M", "J4.M.BPF", "J4.NG", "J4.NG.KGT", "J4.NG.vowel",
		"J5.long", "J5.EH", "J5.IH", "J5.UH", "J5.diph",
		"J6.a", "J6.b", "J7.nolong", "J9",
	}
	for c := range jaHead {
		r = append(r, "J2."+jaHeadRule(c))
	}
	for c := range jaTail {
		r = append(r, "J4."+c)
	}
	return dedupSort(r)
}
