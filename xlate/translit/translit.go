// Package translit 把英文姓名音譯成繁體中文（新華社英語姓名譯名風格）。
//
// 規格：拯救地球 docs/spec/037-player-name-transliteration-draft.md（READY）。
// 查找順序：專案詞典 → CMU 發音詞典 → 拼寫後備；音素到漢字依中文維基
// 「Wikipedia:外語譯音表/英語」人名表（CC BY-SA 4.0，revid 86761742）。
//
// 本套件是純函式庫：不認識任何遊戲、不寫檔、不進存檔；同一組輸入永遠同一輸出。
// 資料檔由呼叫端提供目錄（Load），本套件不內嵌任何表格。
package translit

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Gender 是呼叫端已知的角色性別。
type Gender int

const (
	GenderUnknown Gender = iota
	Male
	Female
)

// Tier 是音譯結果的來源等級，數字越大越可靠。
type Tier int

const (
	TierNone     Tier = iota // ok=false 時
	TierSpelling             // 拼寫後備
	TierCMUdict              // CMU 發音詞典
	TierDict                 // 專案人工詞典
)

func (t Tier) String() string {
	switch t {
	case TierSpelling:
		return "spelling"
	case TierCMUdict:
		return "cmudict"
	case TierDict:
		return "dict"
	}
	return "none"
}

// Separator 是多字名之間的連接號（U+2022）。
const Separator = "•"

// Hyphen 連接連字號名字各段的音譯（新華社慣例：讓-保羅）。字型子集固定含可列印 ASCII。
const (
	Hyphen     = "-"
	hyphenRune = '-'
)

const (
	standaloneRow = "單獨輔音"
	noConsonant   = "無輔音"
	// 規則 8：th 在輔音前或詞尾譯「思」；規則 10：女名「西」音作「茜」（頁面「說明」段落）。
	thChar       = "思"
	xiChar       = "西"
	xiFemaleChar = "茜"
)

type cell struct{ zh, female, initial string }

// Transliterator 是載入後的唯讀資料；可同時給多個 goroutine 使用。
type Transliterator struct {
	table     map[[2]string]cell
	vowelRow  map[string]string // ARPAbet 元音 → 元音列
	consCol   map[string]string // ARPAbet 輔音 → 輔音欄
	nasalRow  map[string]string // "AE N" → 鼻音韻列
	letterRow map[byte]string   // 規則 9：拼寫字母 → 元音列
	names     map[string]map[string]string
	cmu       map[string][]string
	allowed   map[rune]bool
}

// Load 讀 textDir 底下的 translit-table.tsv、translit-arpabet.tsv、translit-names.tsv、
// translit-chars.zh-TW.tsv（允許字集）與 cmudict/cmudict.dict。
func Load(textDir string) (*Transliterator, error) {
	t := &Transliterator{
		table:     map[[2]string]cell{},
		vowelRow:  map[string]string{},
		consCol:   map[string]string{},
		nasalRow:  map[string]string{},
		letterRow: map[byte]string{},
		names:     map[string]map[string]string{},
		cmu:       map[string][]string{},
		allowed:   map[rune]bool{},
	}
	rows, err := readTSV(filepath.Join(textDir, "translit-table.tsv"),
		[]string{"vowel_row", "consonant", "zh", "zh_female", "zh_initial", "note"})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		t.table[[2]string{r[0], r[1]}] = cell{r[2], r[3], r[4]}
	}
	rows, err = readTSV(filepath.Join(textDir, "translit-arpabet.tsv"),
		[]string{"arpabet", "kind", "table_key", "note"})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		switch r[1] {
		case "vowel":
			t.vowelRow[r[0]] = r[2]
		case "consonant":
			t.consCol[r[0]] = r[2]
		case "nasal":
			t.nasalRow[r[0]] = r[2]
		case "letter":
			if len(r[0]) != 1 {
				return nil, fmt.Errorf("translit-arpabet.tsv: letter 必須是單一字母：%q", r[0])
			}
			t.letterRow[r[0][0]] = r[2]
		default:
			return nil, fmt.Errorf("translit-arpabet.tsv: 未知 kind %q", r[1])
		}
	}
	rows, err = readTSV(filepath.Join(textDir, "translit-names.tsv"),
		[]string{"english", "gender", "chinese", "basis"})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		m := t.names[r[0]]
		if m == nil {
			m = map[string]string{}
			t.names[r[0]] = m
		}
		m[r[1]] = r[2]
	}
	rows, err = readTSV(filepath.Join(textDir, "translit-chars.zh-TW.tsv"),
		[]string{"key", "translation", "source"})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		for _, ch := range r[1] {
			t.allowed[ch] = true
		}
	}
	if err := t.loadCMU(filepath.Join(textDir, "cmudict", "cmudict.dict")); err != nil {
		return nil, err
	}
	return t, t.selfCheck()
}

// selfCheck 確認 arpabet 對照指到的列、欄都存在於譯音表，避免資料改名後靜默失效。
func (t *Transliterator) selfCheck() error {
	rowsSeen, colsSeen := map[string]bool{}, map[string]bool{}
	for k := range t.table {
		rowsSeen[k[0]], colsSeen[k[1]] = true, true
	}
	for k, v := range t.vowelRow {
		if !rowsSeen[v] {
			return fmt.Errorf("translit-arpabet.tsv: %s 指到不存在的元音列 %q", k, v)
		}
	}
	for k, v := range t.nasalRow {
		if !rowsSeen[v] {
			return fmt.Errorf("translit-arpabet.tsv: %s 指到不存在的元音列 %q", k, v)
		}
	}
	for k, v := range t.letterRow {
		if !rowsSeen[v] {
			return fmt.Errorf("translit-arpabet.tsv: %c 指到不存在的元音列 %q", k, v)
		}
	}
	for k, v := range t.consCol {
		if !colsSeen[v] {
			return fmt.Errorf("translit-arpabet.tsv: %s 指到不存在的輔音欄 %q", k, v)
		}
	}
	for _, need := range []string{"AA", "AA_O", "AH", "AH0", "AY", "IY", "YUW"} {
		if t.vowelRow[need] == "" {
			return fmt.Errorf("translit-arpabet.tsv: 缺少 %s", need)
		}
	}
	for _, need := range []string{"a", "e", "i", "o", "u"} {
		if t.letterRow[need[0]] == "" {
			return fmt.Errorf("translit-arpabet.tsv: 缺少字母 %s", need)
		}
	}
	return nil
}

func readTSV(path string, header []string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = len(header)
	r.LazyQuotes = false
	all, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(all) == 0 || strings.Join(all[0], "\t") != strings.Join(header, "\t") {
		return nil, fmt.Errorf("%s: 標頭必須為 %s", path, strings.Join(header, "/"))
	}
	return all[1:], nil
}

func (t *Transliterator) loadCMU(path string) error {
	m, err := SharedCMU(path)
	if err != nil {
		return err
	}
	t.cmu = m
	return nil
}

// parseCMU 解析 CMU 發音詞典檔，每個詞只取第一個讀音。
func parseCMU(path string) (map[string][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cmu := map[string][]string{}
	br := bufio.NewReader(f)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			if i := strings.IndexByte(line, '#'); i >= 0 {
				line = line[:i]
			}
			fs := strings.Fields(line)
			// 只取第一個讀音：word(2)、word(3)… 是其他讀音。
			if len(fs) >= 2 && !strings.Contains(fs[0], "(") {
				if _, dup := cmu[fs[0]]; !dup {
					cmu[fs[0]] = fs[1:]
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	if len(cmu) == 0 {
		return nil, fmt.Errorf("%s: 沒有任何詞條", path)
	}
	return cmu, nil
}

// Transliterate 把 name 音譯成繁體中文。字與字以 Separator 連接；tier 是各字來源的最低等級。
// 名字含 A–Z、撇號、連字號、空白以外的字元，或任一字只有一個字母，回 ok=false。
func (t *Transliterator) Transliterate(name string, gender Gender) (zh string, tier Tier, ok bool) {
	words := strings.Fields(strings.ToUpper(name))
	if len(words) == 0 {
		return "", TierNone, false
	}
	parts := make([]string, 0, len(words))
	tier = TierDict
	for _, w := range words {
		letters := 0
		for i := 0; i < len(w); i++ {
			c := w[i]
			switch {
			case c >= 'A' && c <= 'Z':
				letters++
			case c == '\'' || c == '-':
			default:
				return "", TierNone, false
			}
		}
		if letters < 2 {
			return "", TierNone, false
		}
		s, wt := t.word(w, gender)
		if s == "" {
			return "", TierNone, false
		}
		for _, ch := range s {
			if ch != hyphenRune && !t.allowed[ch] {
				return "", TierNone, false
			}
		}
		parts = append(parts, s)
		if wt < tier {
			tier = wt
		}
	}
	return strings.Join(parts, Separator), tier, true
}

// word 音譯一個以空白切出的字。整字先查詞典與 CMUdict（保留撇號、連字號）；
// 含連字號而整字查無時，各段分別走完整三層查找，結果以 Hyphen 連接（mary-jane → 瑪麗-珍）；
// 其餘走拼寫後備。回傳空字串表示無法音譯。
func (t *Transliterator) word(w string, g Gender) (string, Tier) {
	if s, ok := t.lookupDict(w, g); ok {
		return s, TierDict
	}
	if s, ok := t.lookupCMU(w, g); ok {
		return s, TierCMUdict
	}
	if strings.Contains(w, "-") {
		var parts []string
		tier := TierDict
		for _, seg := range strings.Split(w, "-") {
			if seg == "" {
				continue
			}
			if countLetters(seg) < 2 {
				return "", TierNone
			}
			s, st := t.word(seg, g)
			if s == "" {
				return "", TierNone
			}
			parts = append(parts, s)
			if st < tier {
				tier = st
			}
		}
		if len(parts) == 0 {
			return "", TierNone
		}
		return strings.Join(parts, Hyphen), tier
	}
	return t.render(lettersOf(w), spellingPhones(lettersOf(w)), g == Female), TierSpelling
}

// lookupDict 查專案詞典：性別相符者優先，其次不分性別；性別未知時再依 M、F 順序取有標性別的條目。
// 性別已知但只有異性條目時不命中（交給 CMUdict）。
func (t *Transliterator) lookupDict(w string, g Gender) (string, bool) {
	m := t.names[w]
	if m == nil {
		return "", false
	}
	switch g {
	case Male:
		if s, ok := m["M"]; ok {
			return s, true
		}
	case Female:
		if s, ok := m["F"]; ok {
			return s, true
		}
	}
	if s, ok := m[""]; ok {
		return s, true
	}
	if g == GenderUnknown {
		for _, k := range []string{"M", "F"} {
			if s, ok := m[k]; ok {
				return s, true
			}
		}
	}
	return "", false
}

func (t *Transliterator) lookupCMU(w string, g Gender) (string, bool) {
	pron, ok := t.cmu[strings.ToLower(w)]
	if !ok {
		return "", false
	}
	p, ok := parseARPAbet(pron)
	if !ok {
		return "", false
	}
	s := t.render(lettersOf(w), p, g == Female)
	return s, s != ""
}

// lettersOf 去掉撇號、連字號並轉小寫（輸入已驗證只含 A–Z ' -）。
func lettersOf(w string) string {
	return strings.Map(func(r rune) rune {
		if r == '\'' || r == '-' {
			return -1
		}
		return r + ('a' - 'A')
	}, w)
}

func countLetters(w string) int {
	n := 0
	for i := 0; i < len(w); i++ {
		if w[i] >= 'A' && w[i] <= 'Z' {
			n++
		}
	}
	return n
}

// ph 是一個音素；stress 在輔音為 -1。
type ph struct {
	sym    string
	stress int
	vowel  bool
}

var arpaVowels = map[string]bool{
	"AA": true, "AE": true, "AH": true, "AO": true, "AW": true, "AY": true, "EH": true, "ER": true,
	"EY": true, "IH": true, "IY": true, "OW": true, "OY": true, "UH": true, "UW": true,
}

func parseARPAbet(pron []string) ([]ph, bool) {
	out := make([]ph, 0, len(pron))
	for _, p := range pron {
		if n := len(p); n > 0 && p[n-1] >= '0' && p[n-1] <= '2' {
			sym := p[:n-1]
			if !arpaVowels[sym] {
				return nil, false
			}
			st, _ := strconv.Atoi(p[n-1:])
			out = append(out, ph{sym, st, true})
			continue
		}
		if arpaVowels[p] {
			return nil, false
		}
		out = append(out, ph{p, -1, false})
	}
	return out, true
}
