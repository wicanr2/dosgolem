package strikecommander

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// 語音字幕的 lint（strike_commander 規格 010 §3.9）。

// SpeechLintReport 是 LintSpeech 的結果。Errors 非空即失敗；Untranslated（漏譯）與 Over2（超過 2 行，
// 第 4 版即剛好 3 行；超過 3 行是失敗）只列出。
type SpeechLintReport struct {
	Errors       []string
	Untranslated []string
	Over2        []string
}

// OK 回沒有失敗項。
func (r SpeechLintReport) OK() bool { return len(r.Errors) == 0 }

// 逐字稿的 source 只能是這兩種（§3.5）。
const (
	SpeechSourcePlain = "原版資料內的文字（`CONV.PAK`）"
	SpeechSourceCut   = "原版資料內的文字（`CONV.PAK`），截去開發標記"
)

// speechNameTerms 是譯名一致的檢查集（§3.9，glossary 的 en 值，固定清單）。
var speechNameTerms = []string{
	"Stern", "Gwen", "Janet", "Miguel", "Tex", "Billy", "Prideaux", "Farhad", "Muhammed", "Beto", "Tweedly", "Walters", "Lyle", "Virgil", "Ari", "Maxwell", "Mendez",
	"Wildcats", "Wildcat Base", "Jackals", "Selim's", "Strike Base", "Mission Pool", "Prideaux's niche", "Maverick", "Durandal", "GBU-15", "LAU-3",
}

// playerSpeakers 是玩家的兩個說話人代碼（§3.5、§7 第 2 項）。
var playerSpeakers = map[string]bool{"air": true, "airwing1": true}

var (
	reSentenceSource = regexp.MustCompile(`^CONV\.PAK 第 (\d+) 段第 (\d+) 句；(已觀察|未觀察)$`)
	reSpeakerSource  = regexp.MustCompile(`^說話人代碼 (\S+)；(glossary|玩家呼號)$`)
)

// FullwidthText 把字串中的 ASCII 21h–7Eh 換成全形、20h 換成 U+3000，其他字不變（§3.9 譯名比對、§3.6）。
func FullwidthText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == 0x20:
			b.WriteRune('　')
		case r >= 0x21 && r <= 0x7E:
			b.WriteRune(r + 0xFEE0)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParseGlossary 讀 text/glossary.tsv 成 en → zh-TW。
func ParseGlossary(b []byte) (map[string]string, error) {
	lines, err := splitTSVLines(b, "glossary")
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(lines[0], "en\tzh-TW\t") {
		return nil, fmt.Errorf("strikecommander: glossary 欄名不符")
	}
	out := map[string]string{}
	for i, ln := range lines[1:] {
		c := strings.Split(ln, "\t")
		if len(c) < 2 || c[0] == "" || c[1] == "" {
			return nil, fmt.Errorf("strikecommander: glossary 第 %d 列格式不對", i+2)
		}
		out[c[0]] = c[1]
	}
	return out, nil
}

// UnescapeSpeech 還原逐字稿的跳脫（§3.5）：\\、\t、\xNN。回原 bytes；非法跳脫回錯誤。
func UnescapeSpeech(s string) ([]byte, error) {
	var out []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x80 {
			return nil, fmt.Errorf("非 ASCII 字元")
		}
		if c != '\\' {
			out = append(out, c)
			continue
		}
		if i+1 >= len(s) {
			return nil, fmt.Errorf("結尾是單獨的反斜線")
		}
		switch s[i+1] {
		case '\\':
			out = append(out, '\\')
			i++
		case 't':
			out = append(out, '\t')
			i++
		case 'x':
			if i+4 > len(s) {
				return nil, fmt.Errorf("\\x 後不足兩位")
			}
			h := s[i+2 : i+4]
			if strings.ToUpper(h) != h {
				return nil, fmt.Errorf("\\x%s 不是大寫十六進位", h)
			}
			v, err := strconv.ParseUint(h, 16, 8)
			if err != nil || v == 9 || (v >= 0x20 && v < 0x7F) {
				return nil, fmt.Errorf("\\x%s 不合法", h)
			}
			out = append(out, byte(v))
			i += 3
		default:
			return nil, fmt.Errorf("不認得的跳脫 \\%c", s[i+1])
		}
	}
	return out, nil
}

// textSymbols 回英文台詞的代換符號（$ 後接數字為金額，略過）；$ 後接其他字元回錯誤。
func textSymbols(b []byte) ([]string, error) {
	var out []string
	for i, c := range b {
		if c != '$' {
			continue
		}
		if i+1 < len(b) && strings.IndexByte(subSymbols, b[i+1]) >= 0 {
			out = append(out, "$"+string(b[i+1]))
		} else if i+1 < len(b) && b[i+1] >= '0' && b[i+1] <= '9' {
			continue
		} else {
			return nil, fmt.Errorf("$ 後接 %q", string(b[i+1:min(i+2, len(b))]))
		}
	}
	return out, nil
}

// translationSymbols 回譯文的代換符號；$ 後不是 S N C W P T 時回錯誤（含 $L $O $A $B 與數字）。
func translationSymbols(s string) ([]string, error) {
	rs := []rune(s)
	var out []string
	for i, r := range rs {
		if r != '$' {
			continue
		}
		if i+1 < len(rs) && strings.ContainsRune(subSymbols, rs[i+1]) {
			out = append(out, "$"+string(rs[i+1]))
			continue
		}
		next := ""
		if i+1 < len(rs) {
			next = string(rs[i+1])
		}
		return nil, fmt.Errorf("$ 後接 %q（只能接 S N C W P T）", next)
	}
	return out, nil
}

func sameMultiset(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// wordHit 回 term 以 ASCII 整字邊界、大小寫區分出現在 text。
func wordHit(text, term string) bool {
	isAlnum := func(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }
	for from := 0; ; {
		i := strings.Index(text[from:], term)
		if i < 0 {
			return false
		}
		i += from
		j := i + len(term)
		if (i == 0 || !isAlnum(text[i-1])) && (j == len(text) || !isAlnum(text[j])) {
			return true
		}
		from = i + 1
	}
}

// lintSpeechCore 是 LintSpeechText（strike_commander 規格 011 §3.7）的結構、符號、
// source、譯名檢查；每列的字元檢查交給 chars，每句的行數檢查交給 lines（speaker 是說話人譯文，hasSpeaker 為有無）。
func lintSpeechCore(indexB, catalogB, enB, glossaryB []byte, chars func(bad badFunc, r catalogRow),
	lines func(rep *SpeechLintReport, bad badFunc, r catalogRow, speaker string, hasSpeaker bool)) SpeechLintReport {
	var rep SpeechLintReport
	bad := func(format string, a ...any) { rep.Errors = append(rep.Errors, fmt.Sprintf(format, a...)) }
	idx, err := ParseSpeechIndex(indexB)
	if err != nil {
		bad("結構表：%v", err)
		return rep
	}
	gl, err := ParseGlossary(glossaryB)
	if err != nil {
		bad("glossary：%v", err)
		return rep
	}
	for _, t := range speechNameTerms {
		if gl[t] == "" {
			bad("譯名一致檢查集的 %q 不在 glossary", t)
		}
	}
	speakers := idx.Speakers()
	// 逐字稿（§3.9 speech.en.tsv）。
	en := map[string][]byte{}
	if lines, err := splitTSVLines(enB, "逐字稿"); err != nil {
		bad("逐字稿：%v", err)
	} else {
		if lines[0] != "key\ttext\tsource" {
			bad("逐字稿第 1 列欄名要是 key、text、source")
		}
		if len(lines)-1 != len(idx.Lines) {
			bad("逐字稿 %d 列，結構表 %d 列", len(lines)-1, len(idx.Lines))
		}
		for n, ln := range lines[1:] {
			c := strings.Split(ln, "\t")
			if len(c) != 3 {
				bad("逐字稿第 %d 列欄數 %d，要 3", n+2, len(c))
				continue
			}
			if n >= len(idx.Lines) || c[0] != idx.Lines[n].Key {
				bad("逐字稿第 %d 列 key %q 與結構表的 key 集合或列順序不符", n+2, c[0])
				continue
			}
			if c[2] != SpeechSourcePlain && c[2] != SpeechSourceCut {
				bad("逐字稿第 %d 列 source %q 不在列舉", n+2, c[2])
			}
			raw, err := UnescapeSpeech(c[1])
			if err != nil {
				bad("逐字稿第 %d 列非法跳脫：%v", n+2, err)
				continue
			}
			sym, err := textSymbols(raw)
			if err != nil || !sameMultiset(sym, idx.Lines[n].Symbols) {
				bad("逐字稿第 %d 列 %s 的代換符號 %v 與結構表 %v 不符（%v）", n+2, c[0], sym, idx.Lines[n].Symbols, err)
			}
			en[c[0]] = raw
		}
	}
	// 譯文（§3.9 speech.zh-TW.tsv）。
	header, rows, err := splitCatalog(catalogB)
	if err != nil {
		bad("譯文：%v", err)
		return rep
	}
	if header != catalogHeader {
		bad("譯文第 1 列欄名要是 key、translation、source")
	}
	seen := map[string]int{}
	cat := map[string]string{}
	for _, r := range rows {
		if r.cols != 3 {
			bad("譯文第 %d 列欄數 %d，要 3", r.line, r.cols)
			continue
		}
		if first, dup := seen[r.key]; dup {
			bad("譯文第 %d 列 key %q 與第 %d 列重複", r.line, r.key, first)
			continue
		}
		seen[r.key] = r.line
		if r.tr == "" {
			bad("譯文第 %d 列 key %q 譯文是空的", r.line, r.key)
			continue
		}
		if strings.Contains(r.tr, `\`) || strings.Contains(r.source, `\`) {
			bad("譯文第 %d 列 key %q 的譯文或 source 含反斜線", r.line, r.key)
		}
		cat[r.key] = r.tr
	}
	for _, r := range rows {
		if r.cols != 3 || cat[r.key] != r.tr || r.tr == "" {
			continue
		}
		syms, serr := translationSymbols(r.tr)
		if serr != nil {
			bad("譯文第 %d 列 key %q：%v", r.line, r.key, serr)
		}
		chars(bad, r)
		if strings.HasPrefix(r.key, "speaker/") {
			code := strings.TrimPrefix(r.key, "speaker/")
			if !speakers[code] {
				bad("譯文第 %d 列 %q 的代碼不在結構表 speaker 欄（孤兒 key）", r.line, r.key)
				continue
			}
			m := reSpeakerSource.FindStringSubmatch(r.source)
			want := "glossary"
			if playerSpeakers[code] {
				want = "玩家呼號"
			}
			if m == nil || m[1] != code || m[2] != want {
				bad("譯文第 %d 列 %q 的 source %q 不符「說話人代碼 %s；%s」", r.line, r.key, r.source, code, want)
			}
			if serr == nil && !(len(syms) == 0 || len(syms) == 1 && syms[0] == "$C") {
				bad("譯文第 %d 列 %q 只允許單一 $C 或不含 $，得 %v", r.line, r.key, syms)
			}
			if playerSpeakers[code] {
				if r.tr != "$C" {
					bad("譯文第 %d 列 %q 的譯文要是 $C，得 %q", r.line, r.key, r.tr)
				}
				continue
			}
			var en string
			for _, l := range idx.Lines {
				if l.Speaker == code {
					en = l.SpeakerEN
					break
				}
			}
			if zh, ok := gl[en]; !ok || r.tr != FullwidthText(zh) {
				bad("譯文第 %d 列 %q 的譯文 %q 不等於 glossary %q 的 zh-TW %q", r.line, r.key, r.tr, en, FullwidthText(gl[en]))
			}
			continue
		}
		line, ok := idx.Line(r.key)
		if !ok {
			bad("譯文第 %d 列 key %q 不在結構表（孤兒 key）", r.line, r.key)
			continue
		}
		m := reSentenceSource.FindStringSubmatch(r.source)
		if m == nil || m[1] != strconv.Itoa(line.Seg) || m[2] != strconv.Itoa(line.Idx) {
			bad("譯文第 %d 列 key %q 的 source %q 不符「CONV.PAK 第 %d 段第 %d 句；已觀察|未觀察」", r.line, r.key, r.source, line.Seg, line.Idx)
		}
		if serr == nil && !sameMultiset(syms, line.Symbols) {
			bad("譯文第 %d 列 key %q 的代換符號 %v 與結構表 %v 不符", r.line, r.key, syms, line.Symbols)
		}
		// 譯名一致。
		if raw, ok := en[r.key]; ok {
			text := string(raw)
			for _, term := range speechNameTerms {
				if zh := gl[term]; zh != "" && wordHit(text, term) && !strings.Contains(r.tr, FullwidthText(zh)) {
					bad("譯文第 %d 列 key %q：英文含 %q，譯文缺 %q", r.line, r.key, term, FullwidthText(zh))
				}
			}
		}
		sp, ok := cat["speaker/"+line.Speaker]
		lines(&rep, bad, r, sp, ok)
	}
	for _, l := range idx.Lines {
		if _, ok := cat[l.Key]; !ok {
			rep.Untranslated = append(rep.Untranslated, l.Key)
		}
	}
	return rep
}

// withoutSymbols 回拿掉代換符號（$ 與後面的字母）後的字。
func withoutSymbols(s string) []rune {
	rs := []rune(s)
	var out []rune
	for i := 0; i < len(rs); i++ {
		if rs[i] == '$' && i+1 < len(rs) && strings.ContainsRune(subSymbols, rs[i+1]) {
			i++
			continue
		}
		out = append(out, rs[i])
	}
	return out
}
