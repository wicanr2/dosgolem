package phantasie

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// catParse 以 \n 接合各列後解析，方便寫合成資料。
func catParse(lines ...string) (map[string]string, error) {
	return ParseCatalogTSV([]byte(strings.Join(lines, "\n") + "\n"))
}

const catHdr = "key\ttranslation\tsource"

func TestCatalogParseLegal(t *testing.T) {
	cm := string(CenterMark)
	cases := []struct {
		name string
		rows []string
		want map[string]string
	}{
		{"只有欄名列", nil, map[string]string{}},
		{"一般列", []string{"FIRE\t火\tres:2FAC"}, map[string]string{"FIRE": "火"}},
		{"置中標記變成 CenterMark 前綴", []string{`%s HITS` + "\t" + `\c%s 命中` + "\tov2:B963"}, map[string]string{"%s HITS": cm + "%s 命中"}},
		{"只有置中標記", []string{"X\t" + `\c` + "\ts"}, map[string]string{"X": cm}},
		{"置中標記後的尾端空白 rstrip", []string{"X\t" + `\c` + "  \ts"}, map[string]string{"X": cm}},
		{"字面的反斜線加 c 不是標記", []string{"X\t" + `\\c` + "\ts"}, map[string]string{"X": `\c`}},
		{"<blank> 原樣保留", []string{"   X\t<blank>\ts"}, map[string]string{"   X": "<blank>"}},
		{"鍵尾端空白 rstrip", []string{"AB  \t甲\ts"}, map[string]string{"AB": "甲"}},
		{"譯文尾端空白 rstrip", []string{"AB\t甲  \ts"}, map[string]string{"AB": "甲"}},
		{"鍵開頭空白保留", []string{" EARTH\t 土\ts"}, map[string]string{" EARTH": " 土"}},
		{"譯文開頭空白保留", []string{"AB\t  甲\ts"}, map[string]string{"AB": "  甲"}},
		{"空譯文保留為空字串", []string{"AB\t\ts"}, map[string]string{"AB": ""}},
		{"全空白譯文成為空字串", []string{"AB\t   \ts"}, map[string]string{"AB": ""}},
		{"\\t 與 \\\\ 跳脫", []string{`A\tB\\C` + "\t" + `x\ty\\z` + "\ts"}, map[string]string{"A\tB\\C": "x\ty\\z"}},
		{"尾端是 TAB 字元不被 rstrip", []string{`AB\t` + "\t甲\ts"}, map[string]string{"AB\t": "甲"}},
		{"source 欄多來源", []string{"AB\t甲\tres:1;ov1:2"}, map[string]string{"AB": "甲"}},
		{"source 欄可為空", []string{"AB\t甲\t"}, map[string]string{"AB": "甲"}},
	}
	for _, c := range cases {
		got, err := catParse(append([]string{catHdr}, c.rows...)...)
		if err != nil {
			t.Errorf("%s：不應出錯：%v", c.name, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("%s：筆數 %d，要 %d（%v）", c.name, len(got), len(c.want), got)
			continue
		}
		for k, v := range c.want {
			if g, ok := got[k]; !ok || g != v {
				t.Errorf("%s：鍵 %q 得到 %q（存在 %v），要 %q", c.name, k, g, ok, v)
			}
		}
	}
	// 沒有結尾換行
	got, err := ParseCatalogTSV([]byte(catHdr + "\nAB\t甲\ts"))
	if err != nil || got["AB"] != "甲" || len(got) != 1 {
		t.Errorf("沒有結尾換行：%v %v", got, err)
	}
}

func TestCatalogParseErrors(t *testing.T) {
	cases := []struct {
		name string
		data string
		line int
	}{
		{"BOM", "\xEF\xBB\xBF" + catHdr + "\nA\tB\ts\n", 1},
		{"CR 在第 3 列", catHdr + "\nA\tB\ts\nC\tD\ts\r\n", 3},
		{"CR 在欄名列", catHdr + "\r\nA\tB\ts\n", 1},
		{"空檔案缺欄名", "", 1},
		{"只有換行", "\n", 1},
		{"欄名不符", "key\ttranslation\n", 1},
		{"欄名順序不符", "translation\tkey\tsource\n", 1},
		{"欄數 2", catHdr + "\nA\tB\ts\nC\tD\n", 3},
		{"欄數 4", catHdr + "\nA\tB\ts\tx\n", 2},
		{"中間有空白列", catHdr + "\nA\tB\ts\n\nC\tD\ts\n", 3},
		{"結尾多一個空白列", catHdr + "\nA\tB\ts\n\n", 3},
		{"空鍵", catHdr + "\n\tB\ts\n", 2},
		{"鍵全是空白", catHdr + "\n   \tB\ts\n", 2},
		{"重複鍵", catHdr + "\nA\tB\ts\nC\tD\ts\nA\tE\ts\n", 4},
		{"尾端空白後重複", catHdr + "\nA\tB\ts\nA  \tE\ts\n", 3},
		{"含跳脫的鍵重複", catHdr + "\nA\\tB\tX\ts\nC\tY\ts\nA\\tB\tZ\ts\n", 4},
		{"譯文壞跳脫 \\x", catHdr + "\nA\t\\xZZ\ts\n", 2},
		{"鍵壞跳脫 \\q", catHdr + "\nA\\q\tB\ts\n", 2},
		{"譯文尾端反斜線", catHdr + "\nA\tB\\\ts\n", 2},
		{"鍵尾端反斜線", catHdr + "\nA\\\tB\ts\n", 2},
		{"\\c 不在開頭", catHdr + "\nA\tB\\cC\ts\n", 2},
		{"\\c 出現兩次", catHdr + "\nA\t\\c\\cB\ts\n", 2},
		{"鍵內的 \\c", catHdr + "\n\\cA\tB\ts\n", 2},
		{"不合法的 UTF-8", catHdr + "\nA\t\xff\xfe\ts\n", 2},
	}
	for _, c := range cases {
		got, err := ParseCatalogTSV([]byte(c.data))
		if err == nil {
			t.Errorf("%s：應回 error，得到 %v", c.name, got)
			continue
		}
		var te *TSVError
		if !errors.As(err, &te) {
			t.Errorf("%s：error 不是 *TSVError：%v", c.name, err)
			continue
		}
		if te.Line != c.line {
			t.Errorf("%s：行號 %d，要 %d（%v）", c.name, te.Line, c.line, err)
		}
		if got != nil {
			t.Errorf("%s：出錯時 map 應為 nil", c.name)
		}
	}
}

func TestCatalogNormalizeText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"ABC", "ABC"},
		{"ABC  ", "ABC"},
		{"  ABC", "  ABC"},
		{"  ABC  ", "  ABC"},
		{"   ", ""},
		{"A\t", "A\t"},    // 只去 0x20
		{"A \t ", "A \t"}, // 只去尾端的 0x20
		{"A　", "A　"},      // 全形空白不算
	}
	for _, c := range cases {
		if got := NormalizeText(c.in); got != c.want {
			t.Errorf("NormalizeText(%q) = %q，要 %q", c.in, got, c.want)
		}
	}
}

// 期望值由 sha256sum 在主機算出（不用被測函式）：printf '<文字>' | sha256sum。
func TestCatalogDigest(t *testing.T) {
	cases := []struct{ in, want string }{
		{"HELLO THERE", "h:58c688a0f443"}, // 58c688a0f443...
		{"", "h:e3b0c44298fc"},            // 空字串的 sha256 為 e3b0c44298fc...
		{"RO", "h:0049aa51a630"},          // 0049aa51a630b79127...；與真實 prose 的鍵相同
		{"佩爾諾", "h:2ec6b4db7be1"},         // UTF-8 位元組
		{"  AT RANK", "h:abd147df7d2a"},   // 開頭空白保留
	}
	for _, c := range cases {
		if got := Digest(c.in); got != c.want {
			t.Errorf("Digest(%q) = %q，要 %q", c.in, got, c.want)
		}
		if len(Digest(c.in)) != 14 {
			t.Errorf("Digest(%q) 長度不是 14", c.in)
		}
	}
	// Digest 不自行規範化：尾端空白會改變摘要。
	if Digest("RO ") == Digest("RO") {
		t.Error("Digest 不應對尾端空白規範化（呼叫端負責）")
	}
}

func TestCatalogNewAndLookup(t *testing.T) {
	ui := map[string]string{"FIRE": "火", "NEW": ""}
	prose := map[string]string{"h:0049aa51a630": "RO"}
	c := NewCatalog(ui, prose, []string{"SECRET  ", " LEAD", "SECRET"})
	var _ Lookup = c

	if v, ok := c.UI("FIRE"); !ok || v != "火" {
		t.Errorf("UI(FIRE) = %q %v", v, ok)
	}
	if v, ok := c.UI("NEW"); !ok || v != "" {
		t.Errorf("空譯文要回 \"\" 與 ok=true：%q %v", v, ok)
	}
	if _, ok := c.UI("fire"); ok {
		t.Error("鍵區分大小寫")
	}
	if _, ok := c.UI("h:0049aa51a630"); ok {
		t.Error("ui 不得查到 prose 的鍵")
	}
	if v, ok := c.Prose("h:0049aa51a630"); !ok || v != "RO" {
		t.Errorf("Prose = %q %v", v, ok)
	}
	if _, ok := c.Prose("FIRE"); ok {
		t.Error("prose 不得查到 ui 的鍵")
	}
	// 保護清單：鍵先規範化；查詢也規範化（冪等），開頭空白保留。
	for _, s := range []string{"SECRET", "SECRET  ", " LEAD", " LEAD   "} {
		if !c.Protected(s) {
			t.Errorf("Protected(%q) 應為真", s)
		}
	}
	for _, s := range []string{"secret", "LEAD", "", "SECRET2"} {
		if c.Protected(s) {
			t.Errorf("Protected(%q) 應為假", s)
		}
	}
	if u, p, pr := c.Stats(); u != 2 || p != 1 || pr != 2 {
		t.Errorf("Stats = %d %d %d，要 2 1 2", u, p, pr)
	}
	// 複製：改動來源 map 不影響 Catalog。
	ui["FIRE"] = "改"
	ui["ADDED"] = "x"
	if v, _ := c.UI("FIRE"); v != "火" {
		t.Error("NewCatalog 應複製 ui")
	}
	if _, ok := c.UI("ADDED"); ok {
		t.Error("NewCatalog 應複製 ui")
	}
	// nil 參數可用。
	e := NewCatalog(nil, nil, nil)
	if u, p, pr := e.Stats(); u+p+pr != 0 {
		t.Error("空 Catalog 的 Stats 應全為 0")
	}
	if _, ok := e.UI("x"); ok || e.Protected("x") {
		t.Error("空 Catalog 不應命中")
	}
}

func catWrite(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCatalogLoad(t *testing.T) {
	dir := t.TempDir()
	ui := catWrite(t, dir, "ui.tsv", catHdr+"\nFIRE\t火\tres:1\n EARTH\t 土\tres:2\n")
	prose := catWrite(t, dir, "prose.tsv", catHdr+"\nh:0049aa51a630\t"+`\c`+"RO\tmess2:90\n")
	prot := catWrite(t, dir, "protected.tsv", "key\tnote\nPROT A  \t說明一\n\n PROT B\t說明二\nPROT A\t重複\n")

	c, err := LoadCatalog(ui, prose, prot)
	if err != nil {
		t.Fatal(err)
	}
	if u, p, pr := c.Stats(); u != 2 || p != 1 || pr != 2 {
		t.Errorf("Stats = %d %d %d，要 2 1 2", u, p, pr)
	}
	if v, _ := c.UI(" EARTH"); v != " 土" {
		t.Errorf("開頭空白應保留：%q", v)
	}
	if v, _ := c.Prose("h:0049aa51a630"); v != string(CenterMark)+"RO" {
		t.Errorf("prose 置中標記：%q", v)
	}
	if !c.Protected("PROT A") || !c.Protected(" PROT B") || c.Protected("PROT B") {
		t.Error("保護清單載入不符")
	}

	// prosePath、protectedPath 可為空。
	c2, err := LoadCatalog(ui, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if u, p, pr := c2.Stats(); u != 2 || p != 0 || pr != 0 {
		t.Errorf("只有 ui：Stats = %d %d %d", u, p, pr)
	}

	// 錯誤：uiPath 空、檔案不存在、格式錯誤帶行號與檔名。
	if _, err := LoadCatalog("", "", ""); err == nil {
		t.Error("uiPath 為空應出錯")
	}
	if _, err := LoadCatalog(filepath.Join(dir, "nope.tsv"), "", ""); err == nil {
		t.Error("ui 檔不存在應出錯")
	}
	if _, err := LoadCatalog(ui, filepath.Join(dir, "nope.tsv"), ""); err == nil {
		t.Error("prose 檔不存在應出錯")
	}
	if _, err := LoadCatalog(ui, "", filepath.Join(dir, "nope.tsv")); err == nil {
		t.Error("protected 檔不存在應出錯")
	}
	bad := catWrite(t, dir, "bad.tsv", catHdr+"\nA\tB\ts\nA\tC\ts\n")
	_, err = LoadCatalog(bad, "", "")
	var te *TSVError
	if !errors.As(err, &te) || te.Line != 3 || !strings.Contains(err.Error(), "bad.tsv") {
		t.Errorf("重複鍵的 error 要帶檔名與行號 3：%v", err)
	}
	_, err = LoadCatalog(ui, bad, "")
	if !errors.As(err, &te) || te.Line != 3 {
		t.Errorf("prose 的錯誤也要帶行號：%v", err)
	}
	badProt := []struct{ name, body string }{
		{"壞欄名", "name\tnote\nA\tn\n"},
		{"空檔", ""},
		{"空鍵", "key\tnote\n  \tn\n"},
		{"CR", "key\tnote\r\nA\tn\r\n"},
		{"BOM", "\xEF\xBB\xBFkey\tnote\nA\tn\n"},
	}
	for i, b := range badProt {
		p := catWrite(t, dir, "bp"+string(rune('0'+i))+".tsv", b.body)
		if _, err := LoadCatalog(ui, "", p); err == nil {
			t.Errorf("保護清單 %s 應出錯", b.name)
		}
	}
}

// 真實檔案：/text 由 DOSGOLEM_EXTRA_MOUNT 掛入（唯讀）；不存在就跳過。
// 保護清單只載入並比筆數，不讀、不印其內容。
func TestCatalogRealFiles(t *testing.T) {
	const dir = "/text"
	uiPath := filepath.Join(dir, "ui.zh-TW.tsv")
	prosePath := filepath.Join(dir, "prose.zh-TW.tsv")
	protPath := filepath.Join(dir, "protected.tsv")
	for _, p := range []string{uiPath, prosePath, protPath} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("沒有真實 catalog（%s 不存在）：以 DOSGOLEM_EXTRA_MOUNT=<phantasie>/text:/text:ro 掛入後才會執行", p)
		}
	}
	c, err := LoadCatalog(uiPath, prosePath, protPath)
	if err != nil {
		t.Fatal(err)
	}
	if u, p, pr := c.Stats(); u != 677 || p != 981 || pr != 4 {
		t.Errorf("Stats = %d %d %d，要 677 981 4", u, p, pr)
	}
	if v, ok := c.UI("PELNOR"); !ok || v != "佩爾諾" {
		t.Errorf("UI(PELNOR) = %q %v，要 佩爾諾", v, ok)
	}
	if v, ok := c.UI("%s HITS"); !ok || !strings.HasPrefix(v, string(CenterMark)) || v != string(CenterMark)+"%s 命中" {
		t.Errorf("UI(%%s HITS) = %q %v，要 CenterMark 開頭的「%%s 命中」", v, ok)
	}
	// 開頭空白保留（ui 的選單標籤）。
	if v, ok := c.UI(" EARTH"); !ok || v != " 土" {
		t.Errorf("UI( EARTH) = %q %v，要 \" 土\"", v, ok)
	}
	if v, ok := c.UI(" FIRE"); !ok || v != " 火" {
		t.Errorf("UI( FIRE) = %q %v，要 \" 火\"", v, ok)
	}
	// prose 鍵形，且 Digest 與真實鍵吻合（"RO" 的摘要由主機 sha256sum 算出）。
	re := regexp.MustCompile(`^h:[0-9a-f]{12}$`)
	for k := range c.prose {
		if !re.MatchString(k) {
			t.Fatalf("prose 鍵形不符：%q", k)
		}
	}
	if v, ok := c.Prose("h:0049aa51a630"); !ok || v != "RO" {
		t.Errorf("Prose(h:0049aa51a630) = %q %v", v, ok)
	}
	if _, ok := c.Prose(Digest("RO")); !ok {
		t.Error("Digest(RO) 應命中真實 prose 鍵")
	}
	// 沒有誤判：空字串與不存在的文字不在保護清單。
	if c.Protected("") || c.Protected("THIS TEXT IS NOT IN ANY LIST") {
		t.Error("Protected 誤判")
	}
}
