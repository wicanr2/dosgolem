package translit

import "strings"

// render 把一個字的音素串轉成漢字（規格 037 §3.3）。letters 是去掉撇號、連字號的小寫拼寫。
func (t *Transliterator) render(letters string, phones []ph, female bool) string {
	align := alignVowels(letters, phones)
	p := normalize(letters, phones)
	r := renderer{t: t, letters: letters, female: female, align: align}
	r.syllabify(p)
	return r.out.String()
}

// alignVowels 把每個元音音素對到拼寫中的一組元音字母；對不齊回 nil（規則 1、9 與 AA 判讀用）。
func alignVowels(letters string, phones []ph) []string {
	nv := 0
	for _, p := range phones {
		if p.vowel {
			nv++
		}
	}
	groups := vowelGroups(letters)
	// 詞尾不發音的 e（celeste、pierre）與 -es／-ed 的 e（james、charles）。
	if len(groups) > nv && len(groups) > 0 {
		last := groups[len(groups)-1]
		if last.text == "e" && (last.end == len(letters) ||
			(last.end == len(letters)-1 && strings.ContainsRune("sd", rune(letters[len(letters)-1])))) {
			groups = groups[:len(groups)-1]
		}
	}
	// 元音字母組少於元音音素時，拆開 i／y／e／u 起頭的組（pierre 的 ie、flavius 的 iu）。
	for len(groups) < nv {
		idx := -1
		for _, lead := range "iyeuoa" {
			for k, g := range groups {
				if len(g.text) >= 2 && rune(g.text[0]) == lead {
					idx = k
					break
				}
			}
			if idx >= 0 {
				break
			}
		}
		if idx < 0 {
			return nil
		}
		g := groups[idx]
		a := vgroup{g.text[:1], g.start, g.start + 1}
		b := vgroup{g.text[1:], g.start + 1, g.end}
		groups = append(groups[:idx], append([]vgroup{a, b}, groups[idx+1:]...)...)
	}
	if len(groups) != nv {
		return nil
	}
	out := make([]string, nv)
	for i, g := range groups {
		out[i] = g.text
	}
	return out
}

type vgroup struct {
	text       string
	start, end int
}

func isVowelLetter(letters string, i int) bool {
	switch letters[i] {
	case 'a', 'e', 'i', 'o', 'u':
		// qu 的 u 是 [w]。
		return !(letters[i] == 'u' && i > 0 && letters[i-1] == 'q')
	case 'y':
		// y 在元音前是輔音 [j]。
		return !(i+1 < len(letters) && strings.ContainsRune("aeiou", rune(letters[i+1])))
	case 'w':
		// aw、ew、ow 的 w 屬前面的元音組。
		return i > 0 && strings.ContainsRune("aeo", rune(letters[i-1])) &&
			!(i+1 < len(letters) && strings.ContainsRune("aeiouy", rune(letters[i+1])))
	}
	return false
}

func vowelGroups(letters string) []vgroup {
	var out []vgroup
	for i := 0; i < len(letters); {
		if !isVowelLetter(letters, i) || (letters[i] == 'w') {
			i++
			continue
		}
		j := i + 1
		for j < len(letters) && isVowelLetter(letters, j) {
			j++
		}
		out = append(out, vgroup{letters[i:j], i, j})
		i = j
	}
	return out
}

// normalize 在音節切分前把 CMUdict（美式）讀音調成譯音表（英式音位）能用的形式。
func normalize(letters string, in []ph) []ph {
	var p []ph
	// ER 後接元音：[ər] 的 r 成為下一音節的聲母（everett、maria）。
	// 非重讀的 ER0 拆成 [ə]（AH0）＋r，讓附註規則 9 能按拼寫譯（maria → 馬里亞）。
	for i, x := range in {
		if x.sym == "ER" && i+1 < len(in) && in[i+1].vowel {
			if x.stress == 0 {
				x.sym = "AH"
			}
			p = append(p, x, ph{"R", -1, false})
			continue
		}
		p = append(p, x)
	}
	// 元音後、輔音前的 r 在英式不發音（carter、port、roarke）；ER 後的詞尾 r 也併入 ER。
	// 元音後的詞尾 r 保留，按單獨 [ɹ] 譯「爾」（pierre、moore；附註規則 4 的同一結果）。
	var q []ph
	for i, x := range p {
		if x.sym == "R" && i > 0 && p[i-1].vowel {
			last := i == len(p)-1
			if (!last && !p[i+1].vowel) || (last && p[i-1].sym == "ER") {
				continue
			}
		}
		q = append(q, x)
	}
	p = q
	n := len(p)
	// 頁面註腳 voiced：濁輔音清化或清輔音濁化一般仍按形譯。只處理詞尾 -s 讀 [z]（charles、james）。
	if n >= 2 && p[n-1].sym == "Z" && strings.HasSuffix(letters, "s") && p[n-2].sym != "D" && p[n-2].sym != "T" {
		p[n-1].sym = "S"
	}
	// 詞尾 [ts]、[dz] 合讀，按 ts、dz 欄（roberts、edwards）。
	if n >= 2 && p[n-2].sym == "T" && p[n-1].sym == "S" {
		p = append(p[:n-2], ph{"TS", -1, false})
	} else if n >= 2 && p[n-2].sym == "D" && p[n-1].sym == "Z" {
		p = append(p[:n-2], ph{"DZ", -1, false})
	}
	// [ɡʷ]、[kʷ]、[hʷ]：G／K／HH 後接 W 再接元音時合成一個聲母。
	q = q[:0:0]
	for i := 0; i < len(p); i++ {
		if i+2 < len(p) && p[i+1].sym == "W" && p[i+2].vowel {
			if lab, ok := map[string]string{"G": "GW", "K": "KW", "HH": "HW"}[p[i].sym]; ok {
				q = append(q, ph{lab, -1, false})
				i++
				continue
			}
		}
		q = append(q, p[i])
	}
	p = q
	// 規則 6：m 在 b、p 前按 [n] 譯寫（CMUdict 只列發音的 b，所以不發音的 b 不會出現）。
	for i := 0; i+1 < len(p); i++ {
		if p[i].sym == "M" && (p[i+1].sym == "B" || p[i+1].sym == "P") {
			p[i].sym = "N"
		}
	}
	return p
}

type renderer struct {
	t       *Transliterator
	letters string
	female  bool
	align   []string
	out     strings.Builder
	units   int
}

func (r *renderer) cell(row, col string) (cell, bool) {
	c, ok := r.t.table[[2]string{row, col}]
	return c, ok
}

func (r *renderer) put(c cell) {
	s := c.zh
	if r.female {
		// 規則 10：女名用字；「西」音作「茜」。
		if c.female != "" {
			s = c.female
		} else if s == xiChar {
			s = xiFemaleChar
		}
	}
	r.out.WriteString(s)
	r.units++
}

// standalone 輸出單獨輔音；final 表示它是這個字的最後一個音。
func (r *renderer) standalone(sym string, final bool) {
	if sym == "TH" {
		// 規則 8：th 在輔音前或詞尾（即單獨出現時）譯「思」。
		r.out.WriteString(thChar)
		r.units++
		return
	}
	c, ok := r.cell(standaloneRow, r.t.consCol[sym])
	if !ok {
		return
	}
	if r.units == 0 && c.initial != "" {
		// 規則 7：詞首 [f]（表中 v、w、f 欄同格）用「弗」。
		c.zh = c.initial
	}
	if !final && sym != "" && r.t.consCol[sym] == r.t.consCol["S"] {
		// 單獨 [s] 的女名用字「絲」只用在詞尾（agnes、doris 型）；詞中照「斯」（steele、celeste）。
		c.female = ""
	}
	r.put(c)
}

// syllable 輸出「聲母＋元音列」；表中無此格時拆成單獨輔音＋無輔音格。
func (r *renderer) syllable(onset, row string) {
	col := noConsonant
	if onset != "" {
		col = r.t.consCol[onset]
	}
	if c, ok := r.cell(row, col); ok {
		if row == r.t.letterRow['a'] && onset != "" && col == r.t.consCol["F"] {
			// [f]＋[ɑː] 格在來源表標女名用字「娃」，那是 [v]／[w] 欄的字，疑為來源表疏漏；
			// 表檔照原樣保留，程式略過這一格的女名替換，女名仍用「法」（stephanie → 斯特法妮）。
			c.female = ""
		}
		r.put(c)
		return
	}
	r.standalone(onset, false)
	if c, ok := r.cell(row, noConsonant); ok {
		r.put(c)
	}
}

func (r *renderer) vowelRow(v ph, letter string) string {
	if v.sym == "AA" && strings.HasPrefix(letter, "o") {
		return r.t.vowelRow["AA_O"]
	}
	if v.sym == "AH" && v.stress == 0 {
		return r.t.vowelRow["AH0"]
	}
	return r.t.vowelRow[v.sym]
}

func (r *renderer) nasalRow(v ph, nasal string) string {
	if v.stress == 0 {
		if row, ok := r.t.nasalRow[v.sym+"0 "+nasal]; ok {
			return row
		}
	}
	return r.t.nasalRow[v.sym+" "+nasal]
}

func (r *renderer) syllabify(p []ph) {
	var pend []ph
	vi := 0
	for i := 0; i < len(p); i++ {
		v := p[i]
		if !v.vowel {
			pend = append(pend, v)
			continue
		}
		letter := ""
		if r.align != nil {
			letter = r.align[vi]
		}
		first := vi == 0
		vi++
		// 聲母只取緊鄰元音的一個輔音，其餘作單獨輔音（§3.3）。
		onset := ""
		pre := pend
		if n := len(pend); n > 0 && pend[n-1].sym != "NG" {
			onset, pre = pend[n-1].sym, pend[:n-1]
		}
		pend = nil
		yc := "" // 「輔音＋[j]＋元音」的那個輔音
		if onset == "Y" && len(pre) > 0 && pre[len(pre)-1].sym != "NG" {
			yc, pre = pre[len(pre)-1].sym, pre[:len(pre)-1]
		}
		for _, c := range pre {
			r.standalone(c.sym, false)
		}
		wordStart := first && onset == "" && yc == "" && r.units == 0
		// 鼻音韻尾：N／NG 後面不是元音時，與元音合為表中 -n／-ŋ 列。
		nasal := ""
		// N 後接 [j] 時歸下一音節（daniel → 達尼爾、sonia → 索尼亞）。
		if i+1 < len(p) && (p[i+1].sym == "N" || p[i+1].sym == "NG") &&
			(i+2 == len(p) || (!p[i+2].vowel && p[i+2].sym != "Y")) {
			nasal = p[i+1].sym
		}
		lastVowelPhone := i == len(p)-1

		isU := v.sym == "UW" || v.sym == "UH"
		if yc != "" {
			if isU {
				// [Cjuː] 按 juː 列（比尤、繆）。
				r.syllable(yc, r.t.vowelRow["YUW"])
				continue
			}
			rule2 := lastVowelPhone && v.sym == "AH" && v.stress == 0 && strings.HasSuffix(r.letters, "ia")
			if v.sym == "AH" && v.stress == 0 && !rule2 {
				// 表頭「iː ɪ (j)」與「ɪən jən」：[Cjə] 按 C＋[ɪ] 譯，[ə] 不另譯（daniel → 尼爾）。
				row := r.t.vowelRow["IY"]
				if nasal != "" {
					if nr, ok := r.t.nasalRow["IH N"]; ok {
						if _, ok := r.cell(nr, r.t.consCol[yc]); ok {
							row = nr
							i++
						}
					}
				}
				r.syllable(yc, row)
				continue
			}
			r.syllable(yc, r.t.vowelRow["IY"])
		} else if onset == "Y" && isU {
			r.syllable("", r.t.vowelRow["YUW"])
			continue
		}

		// 規則 4：r、re 在詞尾讀 [ə]（無聲母的詞尾 ER）時譯「爾」。
		if lastVowelPhone && v.sym == "ER" && onset == "" &&
			(strings.HasSuffix(r.letters, "r") || strings.HasSuffix(r.letters, "re")) {
			r.standalone("R", true)
			continue
		}

		row := r.vowelRow(v, letter)
		if nasal != "" {
			if nr := r.nasalRow(v, nasal); nr != "" {
				col := noConsonant
				if onset != "" {
					col = r.t.consCol[onset]
				}
				if _, ok := r.cell(nr, col); ok {
					row = nr
					i++
				} else {
					nasal = ""
				}
			} else {
				nasal = ""
			}
		}
		if nasal == "" {
			// 規則 9：非重音節元音按拼寫譯（只處理對得到單一元音字母的開音節）。
			// [ə]（AH0）一律按拼寫；[ɪ]（IH0）只在拼作 a、o、u 時改（flavius 的 u → 烏），
			// 拼作 e、i、y 時 CMUdict 的 [ɪ] 已對應 i 列（agnes → 尼絲）。
			if v.stress == 0 && len(letter) == 1 &&
				(v.sym == "AH" || (v.sym == "IH" && strings.ContainsRune("aou", rune(letter[0])))) {
				if lr, ok := r.t.letterRow[letter[0]]; ok {
					row = lr
				}
			}
			// 規則 1：詞首 a 讀 [ə] 時按 [ɑː] 列。
			if wordStart && v.sym == "AH" && v.stress == 0 && strings.HasPrefix(r.letters, "a") {
				row = r.t.letterRow['a']
			}
			// 規則 3：詞首 ai、ay 按「艾」列。
			if wordStart && (strings.HasPrefix(r.letters, "ai") || strings.HasPrefix(r.letters, "ay")) {
				row = r.t.vowelRow["AY"]
			}
			// 規則 2：詞尾 ia 的 a 譯「亞」（[j] 欄＋[ɑː] 列）。
			if lastVowelPhone && v.sym == "AH" && v.stress == 0 && strings.HasSuffix(r.letters, "ia") &&
				(onset == "" || onset == "Y") {
				onset, row = "Y", r.t.letterRow['a']
			}
		}
		r.syllable(onset, row)
		if v.sym == "OY" {
			// [ɔɪ] 的後半按無輔音 [ɪ]（伊）。
			r.syllable("", r.t.vowelRow["IY"])
		}
	}
	for k, c := range pend {
		r.standalone(c.sym, k == len(pend)-1)
	}
}
