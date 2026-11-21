package dkk

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Lookuper 以原字串 bytes 查譯文。Catalog 與 Translator 都滿足。
type Lookuper interface {
	Lookup(original []byte) (Entry, bool)
}

// Translator 在 catalog 未命中之後依規格 004 組出譯文：名稱、數字模板、標籤、物品名。
type Translator struct {
	Exact     []Lookuper   // 依序查的 catalog
	Names     Lookuper     // 怪物、人物、地名（可為 nil）
	Words     Lookuper     // 物品名詞組來源（介面 catalog；可為 nil）
	Menus     *MenuMasters // 底列選單母串（規格 003 §2.2.1；可為 nil）
	templates map[string]string
}

var digits = regexp.MustCompile(`[0-9]+`)
var modifier = regexp.MustCompile(`^([+-][0-9]+|vs\.)$`)
var placeholder = regexp.MustCompile(`\{([0-9]*)n?\}`)

// LoadTemplates 讀 text/composed-templates.tsv（template、translation、note）。
func (t *Translator) LoadTemplates(b []byte) error {
	rows, err := readTSV(b, []string{"template", "translation"})
	if err != nil {
		return fmt.Errorf("dkk: 模板 TSV：%w", err)
	}
	t.templates = make(map[string]string, len(rows))
	for _, r := range rows {
		if _, dup := t.templates[r[0]]; dup {
			return fmt.Errorf("dkk: 模板重複：%q", r[0])
		}
		t.templates[r[0]] = r[1]
	}
	return nil
}

// MenuWords 回選單詞列在母串表中的逐詞譯文（規格 003 §2.2.1）。
func (t *Translator) MenuWords(words []string) []string { return t.Menus.Words(words) }

// Lookup 實作 Lookuper（規格 004 §2）。
func (t *Translator) Lookup(original []byte) (Entry, bool) {
	if e, ok := t.exact(original); ok {
		return e, true
	}
	if e, ok := t.template(string(original)); ok {
		return e, true
	}
	if e, ok := t.label(string(original)); ok {
		return e, true
	}
	if e, ok := t.readyRow(string(original)); ok {
		return e, true
	}
	if e, ok := t.item(string(original)); ok {
		return e, true
	}
	// 詞組：不以空白結尾、兩詞以上、每段都是介面詞組（例 `Paladine Cleric` 神祇＋職業；規格 004 §2.2）。
	if s := string(original); !strings.HasSuffix(s, " ") && len(strings.Fields(s)) >= 2 {
		if e, ok := t.item(s + " "); ok {
			return e, true
		}
	}
	// 反白列不帶清單列的前導空白，catalog 收的是帶空白的寫法（例 ` <No Spell Effects>`）。
	if len(original) > 0 && original[0] != ' ' {
		return t.exact(append([]byte{' '}, original...))
	}
	return Entry{}, false
}

var readyFlag = regexp.MustCompile(`^( *)(Yes|No) +(\S.*?) *$`)

// readyRow：物品清單列「Yes／No（備妥旗標）＋物品名」（規格 004 §2.6）。旗標譯為「是」「否」，物品名走 item。
func (t *Translator) readyRow(s string) (Entry, bool) {
	m := readyFlag.FindStringSubmatch(s)
	if m == nil {
		return Entry{}, false
	}
	it, ok := t.item(m[3] + " ")
	if !ok {
		return Entry{}, false
	}
	flag := map[string]string{"Yes": "是", "No": "否"}[m[2]]
	return Entry{Key: "ready", Translation: m[1] + flag + " " + it.Translation}, true // 前導空白保留，與反白列對齊
}

func (t *Translator) exact(b []byte) (Entry, bool) {
	for _, c := range t.Exact {
		if e, ok := c.Lookup(b); ok {
			return e, true
		}
	}
	if t.Names != nil {
		if e, ok := t.Names.Lookup(b); ok {
			return e, true
		}
		if s := strings.TrimSpace(string(b)); s != string(b) && s != "" {
			if e, ok := t.Names.Lookup([]byte(s)); ok {
				return e, true
			}
		}
	}
	return Entry{}, false
}

// template：連續數字換成 {n} 後整串比對；譯文的 {n} 依序代回，{k} 取第 k 個數字。
func (t *Translator) template(s string) (Entry, bool) {
	if t.templates == nil {
		return Entry{}, false
	}
	nums := digits.FindAllString(s, -1)
	if len(nums) == 0 {
		// 沒有數字的模板是串接片段的整句譯法（規格 004 §2.1，例 `Spells in Grimoire`）。
		zh, ok := t.templates[s]
		return Entry{Key: "template", Translation: zh}, ok
	}
	zh, ok := t.templates[digits.ReplaceAllString(s, "{n}")]
	if !ok {
		return Entry{}, false
	}
	next := 0
	out := placeholder.ReplaceAllStringFunc(zh, func(m string) string {
		k := strings.TrimSuffix(strings.TrimPrefix(m, "{"), "}")
		if k == "n" {
			if next >= len(nums) {
				return m
			}
			next++
			return nums[next-1]
		}
		if i, err := strconv.Atoi(k); err == nil && i >= 1 && i <= len(nums) {
			return nums[i-1]
		}
		return m
	})
	return Entry{Key: "template", Translation: out}, true
}

// label：「前段:後段」兩段各自查表（後段可走名稱或物品名），都命中才組成。
func (t *Translator) label(s string) (Entry, bool) {
	i := strings.IndexByte(s, ':')
	if i <= 0 || i == len(s)-1 {
		return Entry{}, false
	}
	pre, ok := t.exact([]byte(s[:i+1]))
	if !ok {
		return Entry{}, false
	}
	rest, ok := t.exact([]byte(s[i+1:]))
	if !ok {
		if rest, ok = t.item(s[i+1:] + " "); !ok {
			return Entry{}, false
		}
	}
	p := strings.TrimRight(pre.Translation, ":：")
	return Entry{Key: "label", Translation: p + "：" + rest.Translation}, true
}

// item：只處理以空白結尾的字串（物品名的印法）；從頭貪婪取最長命中詞組，加值照原樣置後。
func (t *Translator) item(s string) (Entry, bool) {
	if t.Words == nil || !strings.HasSuffix(s, " ") {
		return Entry{}, false
	}
	tok := strings.Fields(s)
	if len(tok) == 0 {
		return Entry{}, false
	}
	var b strings.Builder
	phrases := 0
	for i := 0; i < len(tok); {
		if modifier.MatchString(tok[i]) {
			b.WriteString(" " + tok[i])
			i++
			continue
		}
		found := false
		for j := len(tok); j > i; j-- {
			if e, ok := t.Words.Lookup([]byte(strings.Join(tok[i:j], " "))); ok {
				b.WriteString(e.Translation)
				phrases++
				i, found = j, true
				break
			}
		}
		if !found {
			return Entry{}, false
		}
	}
	if phrases == 0 {
		return Entry{}, false
	}
	return Entry{Key: "item", Translation: strings.TrimSpace(b.String())}, true
}
