package strikecommander

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// strike_commander 規格 011 §4.1 第 1、2、9（wrapPixels）、10（lint）、11、17 項；(c) 期。

// testTextRunes 是 testTextFonts 收入的非 ASCII 字：testdata 測試 catalog 的字、lint 測試夾具的字、§4.1 用到的字與全形標點。
// 不收全形英數（Ａ、９ 等）：繪製與 lint 都先經 N()，沒經 N() 就會缺字（N6）。
const testTextRunes = "。」一丁三下丙中之乙二卸史吉地尾己式彈戊文測爾甲登空米維蓋試譯追面飛，" +
	"好今天沒有錢拿字萊葛雯比利門德茲否庚辛壬癸四五六七八九十" +
	"雷射導引炸彈開格通訊模式響蛇與交談短程回報狀態儲存進度讀取消離遊戲確定嗎覆檔是繼續始新訓練任務檢視物件入陸將戰術行動" +
	"高玩家" +
	"（）：，！？－…「」『』、；　"

// testGlyphFunc 回字級 px（em 上緣 asc）的一個字形；nil 表示用預設字形。
type testGlyphFunc func(px, asc int) *xlate.AlphaGlyph

// boxGlyph 是預設字形：填色是 fw×fh 的實心方塊，外框是四周各加 1 的實心方塊。
func boxGlyph(adv, fx, fy, fw, fh int) *xlate.AlphaGlyph {
	g := &xlate.AlphaGlyph{Adv: adv, FX: fx, FY: fy, FW: fw, FH: fh, Fill: bytes.Repeat([]byte{255}, fw*fh)}
	if fh == 0 {
		g.SX, g.SY, g.SW, g.SH, g.Stroke = -1, -1, fw+2, 2, make([]byte, (fw+2)*2)
		return g
	}
	g.SX, g.SY, g.SW, g.SH = fx-1, fy-1, fw+2, fh+2
	g.Stroke = bytes.Repeat([]byte{255}, g.SW*g.SH)
	return g
}

// defaultTestGlyph：空白 adv px/4、fh 0；U+3000 adv px、fh 0；ASCII adv px/2，墨跡 em 列 1..asc−1；
// 其他 adv px，墨跡 em 列 1..px−2（都在 em 框內）。
func defaultTestGlyph(r rune, px, asc int) *xlate.AlphaGlyph {
	switch {
	case r == ' ':
		return boxGlyph(max(1, px/4), 0, 0, max(1, px/4), 0)
	case r == '　':
		return boxGlyph(px, 0, 0, px, 0)
	case r < 0x80:
		return boxGlyph(px/2, 0, -asc+1, px/2-1, asc-1)
	}
	return boxGlyph(px, 1, -asc+1, px-2, px-2)
}

func testTextCharset() []rune {
	var rs []rune
	for r := rune(0x20); r <= 0x7E; r++ {
		rs = append(rs, r)
	}
	rs = append(rs, []rune(testTextRunes)...)
	slices.Sort(rs)
	return slices.Compact(rs)
}

// testTextFonts 在測試內以程式組出五個字級（15、16、22、32、33）的合成圖集（011 §3.9），不讀檔。
func testTextFonts(t *testing.T) *TextFonts {
	t.Helper()
	return testTextFontsWith(t, nil)
}

// testTextFontsWith 同 testTextFonts，override 可覆寫或加入個別字的字形（每個字級各呼叫一次）。
func testTextFontsWith(t *testing.T, override map[rune]testGlyphFunc) *TextFonts {
	t.Helper()
	tf := &TextFonts{BySize: map[int]*xlate.AlphaFont{}, Chars: map[rune]bool{}, Anchor: TextAnchorTest}
	cs := testTextCharset()
	for r := range override {
		if !slices.Contains(cs, r) {
			cs = append(cs, r)
		}
	}
	for _, px := range TextSizes {
		asc := TextAscent(px)
		f := &xlate.AlphaFont{Name: TextFontName(px), Px: px, Ascent: asc, Descent: px - asc, HasStroke: true, StrokeRadius: 1,
			Glyphs: map[rune]*xlate.AlphaGlyph{}}
		for _, r := range cs {
			g := defaultTestGlyph(r, px, asc)
			if fn := override[r]; fn != nil {
				g = fn(px, asc)
			}
			f.Glyphs[r] = g
		}
		tf.BySize[px] = f
	}
	for _, r := range cs {
		tf.Chars[r] = true
	}
	return tf
}

// encodeAAF 依 453 §2 把 AlphaFont 寫成 GOLEMAAF 位元組（測試用寫入器）。
func encodeAAF(f *xlate.AlphaFont) []byte {
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("GOLEMAAF")
	flags, stroke := uint16(0), uint16(0)
	if f.HasStroke {
		flags, stroke = 1, uint16(f.StrokeRadius)
	}
	w(uint16(1))
	w(uint16(f.Px))
	w(int16(f.Ascent))
	w(int16(f.Descent))
	w(flags)
	w(stroke)
	w(uint32(len(f.Glyphs)))
	b.Write(f.Source[:])
	keys := make([]rune, 0, len(f.Glyphs))
	for r := range f.Glyphs {
		keys = append(keys, r)
	}
	slices.Sort(keys)
	for _, r := range keys {
		g := f.Glyphs[r]
		w(uint32(r))
		w(int16(g.Adv))
		w(int16(g.FX))
		w(int16(g.FY))
		w(uint16(g.FW))
		w(uint16(g.FH))
		b.Write(g.Fill)
		if f.HasStroke {
			w(int16(g.SX))
			w(int16(g.SY))
			w(uint16(g.SW))
			w(uint16(g.SH))
			b.Write(g.Stroke)
		}
	}
	return b.Bytes()
}

// ---- 第 1 項：N() ----------------------------------------------------------

func TestNarrowAlnum(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"雷射導引炸彈（ＧＢＵ－１５）", "雷射導引炸彈（GBU-15）"},
		{"開米格－２９，", "開米格-29，"},
		{"通訊模式：", "通訊模式："},
		{"響尾蛇（ＡＩＭ－９Ｊ）", "響尾蛇（AIM-9J）"},
		{"中－文", "中－文"},       // － 兩側都是中文：不換
		{"：（）　＜＞", "：（）　＜＞"}, // 全形標點、U+3000、＜ 不換
		{"ａｚＡＺ０９", "azAZ09"},
		{"－ｘ", "-x"}, // 後一字是英數
		{"－", "－"},
	} {
		in := []rune(c.in)
		orig := slices.Clone(in)
		got := NarrowAlnum(in)
		if string(got) != c.want || len(got) != len(in) {
			t.Errorf("N(%q) = %q，要 %q", c.in, string(got), c.want)
		}
		if !slices.Equal(in, orig) {
			t.Errorf("N(%q) 改了輸入", c.in)
		}
	}
}

// ---- 第 2 項：em 框 --------------------------------------------------------

func TestTextAscent(t *testing.T) {
	want := map[int][2]int{15: {13, 2}, 16: {14, 2}, 22: {19, 3}, 32: {28, 4}, 33: {29, 4}}
	for px, w := range want {
		if a := TextAscent(px); a != w[0] || px-a != w[1] {
			t.Errorf("asc(%d) = %d、descent %d，要 %d、%d", px, a, px-a, w[0], w[1])
		}
	}
	// 以 hhea（1160）算會不同：22 像素得 26。
	if hhea := (22*1160 + 500) / 1000; hhea != 26 || hhea == TextAscent(22) {
		t.Fatalf("hhea 22 像素得 %d", hhea)
	}
}

// ---- 第 9 項：wrapPixels ---------------------------------------------------

func TestWrapUnits(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"測C-130的", []string{"測", "C-130", "的"}},
		{"甲-32768", []string{"甲", "-32768"}},
		{"a-32768", []string{"a-32768"}},
		{"Lee's sam.", []string{"Lee's", " ", "sam", "."}},
		{"x--1", []string{"x", "-", "-1"}}, // 第二個 - 前一字不是英數、後接數字：與數字同一單位
		{"-x", []string{"-", "x"}},
	} {
		var got []string
		for _, u := range wrapUnits([]rune(c.in)) {
			got = append(got, string(u))
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("wrapUnits(%q) = %q，要 %q", c.in, got, c.want)
		}
	}
}

func TestWrapPixels(t *testing.T) {
	f := testTextFonts(t).Font(22) // CJK 22、ASCII 11、空白 5
	type wc struct {
		name   string
		in     string
		maxW   int
		want   []string
		forced bool
	}
	for _, c := range []wc{
		{"C-130 不拆", "測試C-130測試", 76, []string{"測試", "C-130", "測試"}, false},
		{"行首。禁則", "甲乙丙。", 66, []string{"甲乙", "丙。"}, false},
		{"行尾「禁則", "甲乙「丙丁", 66, []string{"甲乙", "「丙丁"}, false},
		{"行首半形空白丟掉", "  甲乙", 66, []string{"甲乙"}, false},
		{"行尾半形空白不計寬、不畫", "甲乙丙 ", 66, []string{"甲乙丙"}, false},
		{"換行處的空白", "甲乙丙 丁", 66, []string{"甲乙丙", "丁"}, false},
		{"退到 i+1 而禁則已滿足：收一字行", "甲「乙丙", 44, []string{"甲", "「乙", "丙"}, false},
		{"退到 i+1 仍違反：還原 j0", "甲。。乙", 44, []string{"甲。", "。乙"}, true},
		{"-32768 整段換行", "甲乙-32768", 88, []string{"甲乙", "-32768"}, false},
		{"只有空白", "   ", 66, nil, false},
		{"空字串", "", 66, nil, false},
		{"單字比行寬", "甲乙", 10, []string{"甲", "乙"}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			ls, forced := wrapPixels([]rune(c.in), f, c.maxW)
			var got []string
			for _, l := range ls {
				got = append(got, string(l))
			}
			if !slices.Equal(got, c.want) || forced != c.forced {
				t.Fatalf("wrapPixels(%q, %d) = %q forced %v，要 %q forced %v", c.in, c.maxW, got, forced, c.want, c.forced)
			}
		})
	}
	// 40 個 ASCII 字母是一個單位，比行寬寬：拆成單字並記 forced（每行 300 / 11 = 27 字）。
	ls, forced := wrapPixels([]rune(strings.Repeat("a", 40)), f, 300)
	if !forced || len(ls) != 2 || len(ls[0]) != 27 || len(ls[1]) != 13 {
		t.Fatalf("40 字母：%d 行 forced %v", len(ls), forced)
	}
	// 行寬以像素計：ASCII 11、CJK 22。字數換行（每字同寬）會得不同的行。
	ls, _ = wrapPixels([]rune("abcd甲乙"), f, 66)
	if len(ls) != 2 || string(ls[0]) != "abcd甲" || string(ls[1]) != "乙" {
		t.Fatalf("像素寬換行得 %q", ls)
	}
	ls, _ = wrapPixels([]rune("ab甲乙丙"), f, 66)
	if len(ls) != 2 || string(ls[0]) != "ab甲乙" || string(ls[1]) != "丙" {
		t.Fatalf("像素寬換行得 %q", ls)
	}
}

// ---- 第 10 項：lint --------------------------------------------------------

func lintHas(errs []string, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

func catalogBytes(rows ...string) []byte {
	return []byte(catalogHeader + "\n" + strings.Join(rows, "\n") + "\n")
}

// tallGlyph 回墨跡從 em 上緣 top 列起、高 h 的字（外框各加 1）。
func tallGlyph(top, h func(px int) int) testGlyphFunc {
	return func(px, asc int) *xlate.AlphaGlyph { return boxGlyph(px, 1, -asc+top(px), px-2, h(px)) }
}

func TestLintLabels(t *testing.T) {
	fonts := testTextFontsWith(t, map[rune]testGlyphFunc{
		// 墨跡從 em 上緣上方 6 列起：2 倍標籤 ey 3，3−6−1（外框）= −4 < −2，超出裁切框。
		'高': tallGlyph(func(int) int { return -6 }, func(px int) int { return px }),
	})
	labl := testLABL("Talk to Gwen", "Fly Mission", "Transport to Strike Base")
	base := []string{"Talk to Gwen\t測試譯文甲\ts", "Fly Mission\t測試丙\ts", "Transport to Strike Base\t測試譯文乙\ts"}
	if rep := LintLabels(catalogBytes(base...), fonts, labl); !rep.OK() {
		t.Fatalf("正向例：%v", rep.Errors)
	}
	// testdata 的測試 catalog 也通過。
	tb, err := os.ReadFile("testdata/opttest_labels.test.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if rep := LintLabels(tb, fonts, labl); !rep.OK() {
		t.Fatalf("testdata：%v", rep.Errors)
	}
	set := func(key, tr string) []byte {
		rows := slices.Clone(base)
		for i, r := range rows {
			if strings.HasPrefix(r, key+"\t") {
				rows[i] = key + "\t" + tr + "\ts"
				return catalogBytes(rows...)
			}
		}
		return catalogBytes(append(rows, key+"\t"+tr+"\ts")...)
	}
	cases := []struct {
		name, key, tr, want string
		ok                  bool
	}{
		{"標籤 29 字", "Talk to Gwen", strings.Repeat("測", 29), "", true},
		// 30 × 22 + 2 = 662 > 640；以 16 像素量（N25）是 480，不會報。
		{"標籤 30 字", "Talk to Gwen", strings.Repeat("測", 30), "2 倍寬 660 + 2 超過 640", false},
		{"半形字", "Talk to Gwen", "測試a", "含半形字", false},
		{"缺字", "Talk to Gwen", "測鑫", "缺字", false},
		// 全形英數經 N() 才在圖集內（N6）。
		{"全形英數經 N()", "Talk to Gwen", "響尾蛇（ＡＩＭ－９Ｊ）", "", true},
		{"GROUND 9 字", "GROUND", strings.Repeat("地", 9), "", true},
		{"GROUND 10 字", "GROUND", strings.Repeat("地", 10), "2 倍寬 160 超過 146", false},
		{"AIR 3 字", "AIR", "空中空", "", true},
		{"AIR 4 字", "AIR", "空中空中", "2 倍寬 64 超過 62", false},
		{"墨跡超出裁切框列", "Talk to Gwen", "高", "墨跡列", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep := LintLabels(set(c.key, c.tr), fonts, labl)
			if c.ok != rep.OK() || (!c.ok && !lintHas(rep.Errors, c.want)) {
				t.Fatalf("ok %v，錯誤 %v；要 ok %v、含 %q", rep.OK(), rep.Errors, c.ok, c.want)
			}
		})
	}
	// 組合標籤：名稱 25 字單獨放得下（552），代入「%s（%d）」後 25×22 + 22 + 55 + 22 + 2 = 651 > 640。
	long := strings.Repeat("測", 25)
	rows := append(slices.Clone(base), "AIM-9J rear aspect missile\t"+long+"\ts", TemplateCount+"\t%s（%d）\ts")
	rep := LintLabels(catalogBytes(rows...), fonts, labl)
	if rep.OK() || !lintHas(rep.Errors, "組合標籤") {
		t.Fatalf("組合標籤超寬應報：%v", rep.Errors)
	}
	rows[len(rows)-2] = "AIM-9J rear aspect missile\t" + strings.Repeat("測", 24) + "\ts"
	if rep := LintLabels(catalogBytes(rows...), fonts, labl); !rep.OK() {
		t.Fatalf("組合標籤 24 字應通過：%v", rep.Errors)
	}
}

func TestArtVerticalRange(t *testing.T) {
	// 墨跡高恰為字帶高：只有 dy 0 時放得下，含 ±1 的趟就放不下。
	if lo, hi := artVerticalRange(1, 23, 22, []int{0}); lo > hi {
		t.Fatalf("dy {0}：lo %d hi %d", lo, hi)
	}
	if lo, hi := artVerticalRange(1, 23, 22, []int{1, -1, 0}); lo <= hi {
		t.Fatalf("dy {±1, 0}：lo %d hi %d，應 lo > hi", lo, hi)
	}
	if lo, hi := artVerticalRange(-2, 15, 18, []int{1, 0}); lo != 2 || hi != 2 {
		t.Fatalf("lo %d hi %d，要 2、2", lo, hi)
	}
}

func TestLintArtText(t *testing.T) {
	fonts := testTextFontsWith(t, map[rune]testGlyphFunc{
		// 墨跡高恰為主選單字帶高（11 列 × s）：dy 0 放得下，主選單一般態的陰影（+1）與亮邊（−1）放不下。
		'高': tallGlyph(func(int) int { return 1 }, func(px int) int {
			if px == 16 {
				return 22
			}
			return 33
		}),
	})
	b, err := os.ReadFile("testdata/menu_gfx.test.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if rep := LintArtText(b, fonts); !rep.OK() {
		t.Fatalf("testdata：%v", rep.Errors)
	}
	set := func(key, tr string) []byte {
		s := string(b)
		i := strings.Index(s, "\n"+key+"\t")
		j := strings.Index(s[i+1:], "\n") + i + 1
		row := strings.Split(s[i+1:j], "\t")
		row[1] = tr
		return []byte(s[:i+1] + strings.Join(row, "\t") + s[j:])
	}
	for _, c := range []struct {
		name, key, tr, want string
		ok                  bool
	}{
		{"SAVE 4 字", "SAVE", "測試測試", "", true},
		// k2 字帶寬 40 × 2 = 80；5 × 16 + 2 = 82。
		{"SAVE 5 字", "SAVE", "測試測試測", "存讀檔 k2 一般 2 倍寬 80 + 2×1 超過字帶寬 80", false},
		{"半形字", "SAVE", "測a", "含半形字", false},
		{"墨跡含 dy 才超出", "CONTINUE GAME", "高", "主選單 n0 一般 2 倍墨跡列", false},
		{"LOAD GAME 兩處都查（主選單）", "LOAD GAME", "高", "主選單 n1 一般 2 倍墨跡列", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			rep := LintArtText(set(c.key, c.tr), fonts)
			if c.ok != rep.OK() || (!c.ok && !lintHas(rep.Errors, c.want)) {
				t.Fatalf("ok %v，錯誤 %v；要 ok %v、含 %q", rep.OK(), rep.Errors, c.ok, c.want)
			}
		})
	}
	// 「高」放在只有 dy 0 的存讀檔元素（k1 字帶 12 列）不報。
	if rep := LintArtText(set("SAVE GAME", "高"), fonts); !rep.OK() {
		t.Fatalf("dy 0 的元素不應報：%v", rep.Errors)
	}
	// LOAD GAME 兩處都查（存讀檔）：13 字 13 × 16 + 2 = 210 超過存讀檔 k1 字帶寬 96 × 2 = 192，主選單字帶 207 × 2 = 414 放得下。
	rep := LintArtText(set("LOAD GAME", strings.Repeat("測", 13)), fonts)
	if !lintHas(rep.Errors, "存讀檔 k1") || lintHas(rep.Errors, "主選單 n1") {
		t.Fatalf("LOAD GAME 13 字：%v", rep.Errors)
	}
	// 「高」只在主選單（趟含 ±1）放不下，存讀檔 k1（dy 0、字帶 12 列）放得下。
	rep = LintArtText(set("LOAD GAME", "高"), fonts)
	if lintHas(rep.Errors, "存讀檔 k1") {
		t.Fatalf("LOAD GAME「高」不應報存讀檔：%v", rep.Errors)
	}
}

func flightCatalogFor(kind Kind, override map[string]string) []byte {
	rows := []string{}
	for _, k := range flightAllowed[kind] {
		tr := "測"
		if v, ok := override[k]; ok {
			tr = v
		}
		rows = append(rows, k+"\t"+tr+"\ts")
	}
	return catalogBytes(rows...)
}

func TestLintFlightText(t *testing.T) {
	fonts := testTextFontsWith(t, map[rune]testGlyphFunc{
		// 墨跡從 em 上緣上方 5 列起：HUD 3 倍 ey 0、外框再上 1 列，−6 < −3。
		'高': tallGlyph(func(int) int { return -5 }, func(px int) int { return px }),
	})
	for _, kind := range []Kind{KindHUD, KindMFD, KindPlaque} {
		if errs := LintFlightText(kind, flightCatalogFor(kind, nil), fonts, nil); len(errs) != 0 {
			t.Fatalf("%s 正向例：%v", kind, errs)
		}
	}
	for _, c := range []struct {
		name string
		kind Kind
		ov   map[string]string
		want string
	}{
		// GEAR 可用 31 × 3 = 93：6 × 15 + 2 = 92 通過，7 字 107 失敗。
		{"HUD GEAR 7 字", KindHUD, map[string]string{"GEAR": strings.Repeat("測", 7)}, "3 倍寬 105 + 2 超過 93"},
		{"HUD 半形字", KindHUD, map[string]string{"SRM": "測a"}, "含半形字"},
		{"HUD 墨跡超出", KindHUD, map[string]string{"SRM": "高"}, "墨跡列"},
		{"MFD k0 11 字", KindMFD, map[string]string{"NORM": strings.Repeat("測", 11)}, "2 倍寬 176 超過 174"},
		{"MFD request status 10 字", KindMFD, map[string]string{"request status": strings.Repeat("測", 10)}, "2 倍寬 160 超過 152"},
		{"橫幅 time burst 9 字", KindPlaque, map[string]string{"time burst": strings.Repeat("測", 9)}, "2 倍寬 144 超過 138"},
	} {
		t.Run(c.name, func(t *testing.T) {
			errs := LintFlightText(c.kind, flightCatalogFor(c.kind, c.ov), fonts, nil)
			var ss []string
			for _, e := range errs {
				ss = append(ss, e.Error())
			}
			if !lintHas(ss, c.want) {
				t.Fatalf("沒有含 %q 的錯誤：%v", c.want, ss)
			}
		})
	}
	for _, c := range []struct {
		kind Kind
		ov   map[string]string
	}{
		{KindHUD, map[string]string{"GEAR": strings.Repeat("測", 6)}},
		{KindMFD, map[string]string{"NORM": strings.Repeat("測", 10), "request status": strings.Repeat("測", 9)}},
		{KindPlaque, map[string]string{"time burst": strings.Repeat("測", 8)}},
	} {
		if errs := LintFlightText(c.kind, flightCatalogFor(c.kind, c.ov), fonts, nil); len(errs) != 0 {
			t.Fatalf("%s 上限內應通過：%v", c.kind, errs)
		}
	}
}

func TestLintSpeechText(t *testing.T) {
	fonts := testTextFonts(t)
	run := func(f lintFixture) SpeechLintReport {
		return LintSpeechText([]byte(f.index), f.catalog(), []byte(f.en), []byte(f.glossary), fonts)
	}
	f := baseLintFixture()
	if rep := run(f); !rep.OK() || len(rep.Untranslated) != 0 {
		t.Fatalf("正向例：%v %v", rep.Errors, rep.Untranslated)
	}
	set := func(key, row string) lintFixture {
		g := baseLintFixture()
		g.cat = slices.Clone(g.cat)
		for i, r := range g.cat {
			if strings.HasPrefix(r, key+"\t") {
				g.cat[i] = row
			}
		}
		return g
	}
	// 半形字。
	if rep := run(set("conv/011/01", "conv/011/01\t史登測a\tCONV.PAK 第 11 段第 1 句；未觀察")); !lintHas(rep.Errors, "含半形字") {
		t.Fatalf("半形字：%v", rep.Errors)
	}
	// 缺字。
	if rep := run(set("conv/011/01", "conv/011/01\t史登鑫\tCONV.PAK 第 11 段第 1 句；未觀察")); !lintHas(rep.Errors, "缺字") {
		t.Fatalf("缺字：%v", rep.Errors)
	}
	// 最壞代換：「史登：」3 字 + $N 的 19 個 ？ + m 個「測」；每行 27 字（2 倍 600 / 22、3 倍 900 / 33）。
	row := func(m int) string {
		return "conv/011/00\t$N" + strings.Repeat("測", m) + "\tCONV.PAK 第 11 段第 0 句；已觀察"
	}
	rep := run(set("conv/011/00", row(59))) // 81 字：3 行，列出
	if !rep.OK() || len(rep.Over2) != 1 || !strings.HasPrefix(rep.Over2[0], "conv/011/00（3 行）") {
		t.Fatalf("3 行：%v %v", rep.Errors, rep.Over2)
	}
	rep = run(set("conv/011/00", row(60))) // 82 字：4 行，失敗
	if rep.OK() || !lintHas(rep.Errors, "2 倍換行後 4 行") || !lintHas(rep.Errors, "3 倍換行後 4 行") {
		t.Fatalf("4 行：%v", rep.Errors)
	}
	// $P 以 -32768 計（6 個 ASCII，寬 66）。
	if got := string(withWorstSubs("甲$P乙$C")); got != "甲-32768乙"+strings.Repeat("？", 19) {
		t.Fatalf("最壞代換 %q", got)
	}
}

func TestCheckTextExtra(t *testing.T) {
	fonts := testTextFonts(t)
	if err := CheckTextExtra(fonts); err != nil {
		t.Fatal(err)
	}
	delete(fonts.BySize[33].Glyphs, '（')
	if err := CheckTextExtra(fonts); err == nil || !strings.Contains(err.Error(), "（") {
		t.Fatalf("缺（應報：%v", err)
	}
	fonts = testTextFonts(t)
	delete(fonts.Chars, '7')
	if err := CheckTextExtra(fonts); err == nil {
		t.Fatal("字集缺 7 應報")
	}
}

// ---- 第 11 項：LoadTextFonts ----------------------------------------------

func smallAtlas(px int, chars []rune) *xlate.AlphaFont {
	asc := TextAscent(px)
	f := &xlate.AlphaFont{Px: px, Ascent: asc, Descent: px - asc, HasStroke: true, StrokeRadius: 1, Glyphs: map[rune]*xlate.AlphaGlyph{}}
	for _, r := range chars {
		f.Glyphs[r] = defaultTestGlyph(r, px, asc)
	}
	return f
}

// writeAtlasDir 在 dir 以程式寫出五個合成圖集、字集與 manifest（manifest 依寫出的內容記），回 manifest 的 file 列。
// content 可替換某個檔的內容（manifest 照替換後的內容記）。
func writeAtlasDir(t *testing.T, dir, kind string, chars []rune, content func(name string, def []byte) []byte) []string {
	t.Helper()
	var rows []string
	put := func(name string, px int, def []byte) []byte {
		b := def
		if content != nil {
			b = content(name, def)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
		count := len([]rune(strings.TrimSuffix(string(b), "\n")))
		if px != 0 {
			count = int(binary.LittleEndian.Uint32(b[20:24]))
		}
		rows = append(rows, fmt.Sprintf("file\t%s\t%d\t%d\t%d\t%s", name, px, count, len(b), shaHex(b)))
		return b
	}
	for _, px := range TextSizes {
		put(TextAtlasName(px), px, encodeAAF(smallAtlas(px, chars)))
	}
	cb := put(TextCharsName, 0, []byte(string(chars)+"\n"))
	lines := []string{textTableHeader}
	for i, v := range []string{kind, "11.3.0", "2.13.3", "sha256:" + strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), shaHex(cb)} {
		lines = append(lines, "meta\t"+textMetaKeys[i]+"\t-\t-\t-\t"+v)
	}
	lines = append(lines, rows...)
	if err := os.WriteFile(filepath.Join(dir, TextManifestName), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return rows
}

func writeExpected(t *testing.T, path string, rows []string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(textTableHeader+"\n"+strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rewriteManifest(t *testing.T, dir string, f func(s string) string) {
	t.Helper()
	p := filepath.Join(dir, TextManifestName)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(f(string(b))), 0o644); err != nil {
		t.Fatal(err)
	}
}

// flipLast 把字串最後一個字元換成另一個 hex 字元。
func flipLast(s string) string {
	if strings.HasSuffix(s, "0") {
		return s[:len(s)-1] + "1"
	}
	return s[:len(s)-1] + "0"
}

// TestLoadTextFonts 是 011 §4.1 第 11 項的表：測試在暫存目錄以程式寫出合成圖集與 manifest。
func TestLoadTextFonts(t *testing.T) {
	chars := []rune(" A中")
	type tc struct {
		name     string
		kind     string
		expected bool
		content  func(name string, def []byte) []byte
		setup    func(t *testing.T, dir, exp string, rows []string)
		anchor   string // 空字串表示要回錯
	}
	cases := []tc{
		{name: "formal、給預期表、全部相符", kind: "formal", expected: true, anchor: TextAnchorMatch},
		{name: "test、不給預期表", kind: "test", anchor: TextAnchorTest},
		{name: "formal、不給預期表", kind: "formal"},
		{name: "test、給預期表", kind: "test", expected: true},
		{name: "缺一個圖集檔", kind: "formal", expected: true, setup: func(t *testing.T, dir, _ string, _ []string) {
			if err := os.Remove(filepath.Join(dir, TextAtlasName(22))); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "多一個字級（24）", kind: "formal", expected: true, setup: func(t *testing.T, dir, exp string, rows []string) {
			b := encodeAAF(smallAtlas(24, chars))
			if err := os.WriteFile(filepath.Join(dir, TextAtlasName(24)), b, 0o644); err != nil {
				t.Fatal(err)
			}
			row := fmt.Sprintf("file\t%s\t24\t%d\t%d\t%s", TextAtlasName(24), len(chars), len(b), shaHex(b))
			rewriteManifest(t, dir, func(s string) string {
				return strings.Replace(s, "file\t"+TextAtlasName(32), row+"\nfile\t"+TextAtlasName(32), 1)
			})
			writeExpected(t, exp, append(append(slices.Clone(rows[:3]), row), rows[3:]...))
		}},
		{name: "某圖集檔內容改一 byte（manifest 未改）", kind: "formal", expected: true, setup: func(t *testing.T, dir, _ string, _ []string) {
			p := filepath.Join(dir, TextAtlasName(16))
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			b[len(b)-1] ^= 1
			if err := os.WriteFile(p, b, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "manifest 某 file 列 value 與預期表不同", kind: "formal", expected: true, setup: func(t *testing.T, _, exp string, rows []string) {
			r := slices.Clone(rows)
			r[1] = flipLast(r[1])
			writeExpected(t, exp, r)
		}},
		{name: "預期表多一列", kind: "formal", expected: true, setup: func(t *testing.T, _, exp string, rows []string) {
			writeExpected(t, exp, append(slices.Clone(rows), "file\tx.aaf\t1\t1\t1\t"+strings.Repeat("f", 64)))
		}},
		{name: "chars_sha256 與字集列不同", kind: "formal", expected: true, setup: func(t *testing.T, dir, _ string, _ []string) {
			rewriteManifest(t, dir, func(s string) string {
				k := "meta\tchars_sha256\t-\t-\t-\t"
				i := strings.Index(s, k) + len(k)
				return s[:i] + strings.Repeat("9", 64) + s[i+64:]
			})
		}},
		{name: "圖集碼點集合與 chars.txt 不同", kind: "formal", expected: true, content: func(name string, def []byte) []byte {
			if name == TextCharsName {
				return []byte(" A文\n")
			}
			return def
		}},
		{name: "兩個圖集碼點集合不同", kind: "formal", expected: true, content: func(name string, def []byte) []byte {
			if name == TextAtlasName(33) {
				return encodeAAF(smallAtlas(33, []rune(" A文")))
			}
			return def
		}},
		{name: "圖集 px 與 manifest px 不同", kind: "formal", expected: true, content: func(name string, def []byte) []byte {
			if name == TextAtlasName(15) {
				return encodeAAF(smallAtlas(17, chars))
			}
			return def
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			rows := writeAtlasDir(t, dir, c.kind, chars, c.content)
			exp := ""
			if c.expected {
				exp = filepath.Join(t.TempDir(), "expected.tsv")
				writeExpected(t, exp, rows)
			}
			if c.setup != nil {
				c.setup(t, dir, exp, rows)
			}
			tf, err := LoadTextFonts(dir, exp)
			if c.anchor == "" {
				if err == nil {
					t.Fatalf("應回錯，得 Anchor %q", tf.Anchor)
				}
				t.Logf("回錯：%v", err)
				return
			}
			if err != nil {
				t.Fatalf("應成功：%v", err)
			}
			if tf.Anchor != c.anchor || len(tf.BySize) != 5 || len(tf.Chars) != 3 || len(tf.Files) != 6 {
				t.Fatalf("Anchor %q、%d 字級、%d 字、%d 檔", tf.Anchor, len(tf.BySize), len(tf.Chars), len(tf.Files))
			}
			for _, px := range TextSizes {
				if f := tf.BySize[px]; f.Name != TextFontName(px) || f.Px != px {
					t.Fatalf("字級 %d 的字型 %q px %d", px, f.Name, f.Px)
				}
			}
			if ff := tf.Files[2]; ff.Name != TextAtlasName(22) || ff.Px != 22 || ff.Count != 3 || ff.SHA256 != strings.Split(rows[2], "\t")[5] {
				t.Fatalf("Files[2] = %+v", ff)
			}
		})
	}
}

// ---- 第 17 項：跨語言黃金向量 ----------------------------------------------

// TestGoldenAAF 以 xlate.ParseAlphaFont 讀 build_noto.py --golden 寫的 testdata/golden.aaf，逐欄與 golden.txt 比對
// （011 §3.2.3：Python 寫、Go 讀的格式理解一致）。
func TestGoldenAAF(t *testing.T) {
	b, err := os.ReadFile("testdata/golden.aaf")
	if err != nil {
		t.Fatal(err)
	}
	txt, err := os.ReadFile("testdata/golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	f, err := xlate.ParseAlphaFont(b)
	if err != nil {
		t.Fatal(err)
	}
	atoi := func(s string) int {
		v, err := strconv.Atoi(s)
		if err != nil {
			t.Fatalf("golden.txt 數字 %q", s)
		}
		return v
	}
	unhex := func(s string) []byte {
		if s == "-" {
			return nil
		}
		v, err := hex.DecodeString(s)
		if err != nil {
			t.Fatalf("golden.txt hex %q", s)
		}
		return v
	}
	glyphs := 0
	for _, ln := range strings.Split(strings.TrimSuffix(string(txt), "\n"), "\n") {
		c := strings.Split(ln, "\t")
		switch c[0] {
		case "magic":
			if string(b[:8]) != c[1] {
				t.Errorf("magic %q，要 %q", b[:8], c[1])
			}
		case "version":
			if int(binary.LittleEndian.Uint16(b[8:10])) != atoi(c[1]) {
				t.Errorf("version 不符")
			}
		case "px":
			if f.Px != atoi(c[1]) {
				t.Errorf("px %d，要 %s", f.Px, c[1])
			}
		case "ascent":
			if f.Ascent != atoi(c[1]) || f.Ascent != TextAscent(f.Px) {
				t.Errorf("ascent %d，要 %s", f.Ascent, c[1])
			}
		case "descent":
			if f.Descent != atoi(c[1]) {
				t.Errorf("descent %d，要 %s", f.Descent, c[1])
			}
		case "flags":
			if (atoi(c[1])&1 != 0) != f.HasStroke {
				t.Errorf("flags %s 與 HasStroke %v 不符", c[1], f.HasStroke)
			}
		case "stroke":
			if f.StrokeRadius != atoi(c[1]) {
				t.Errorf("stroke %d，要 %s", f.StrokeRadius, c[1])
			}
		case "count":
			if len(f.Glyphs) != atoi(c[1]) {
				t.Errorf("count %d，要 %s", len(f.Glyphs), c[1])
			}
		case "source":
			if hex.EncodeToString(f.Source[:]) != c[1] {
				t.Errorf("source 不符")
			}
		case "glyph":
			glyphs++
			cp, err := strconv.ParseUint(c[1], 16, 32)
			if err != nil || len(c) != 13 {
				t.Fatalf("golden.txt 列 %q", ln)
			}
			g := f.Glyphs[rune(cp)]
			if g == nil {
				t.Fatalf("缺 U+%04X", cp)
			}
			want := xlate.AlphaGlyph{Adv: atoi(c[2]), FX: atoi(c[3]), FY: atoi(c[4]), FW: atoi(c[5]), FH: atoi(c[6]), Fill: unhex(c[7]),
				SX: atoi(c[8]), SY: atoi(c[9]), SW: atoi(c[10]), SH: atoi(c[11]), Stroke: unhex(c[12])}
			if g.Adv != want.Adv || g.FX != want.FX || g.FY != want.FY || g.FW != want.FW || g.FH != want.FH || !bytes.Equal(g.Fill, want.Fill) ||
				g.SX != want.SX || g.SY != want.SY || g.SW != want.SW || g.SH != want.SH || !bytes.Equal(g.Stroke, want.Stroke) {
				t.Errorf("U+%04X：讀到 %+v，要 %+v", cp, *g, want)
			}
		default:
			t.Fatalf("golden.txt 不認得的列 %q", ln)
		}
	}
	if glyphs != 3 || glyphs != len(f.Glyphs) {
		t.Fatalf("golden.txt %d 字、圖集 %d 字", glyphs, len(f.Glyphs))
	}
	// 合成向量的必要成分：外框層、負偏移、fw > 0 且 fh = 0 的空白字。
	sp := f.Glyphs[' ']
	if !f.HasStroke || sp == nil || sp.FW == 0 || sp.FH != 0 || f.Glyphs['中'].FX >= 0 {
		t.Fatal("golden 缺外框層、空白字或負偏移")
	}
}

// ---- 環境變數：圖集目錄 ------------------------------------------------------

const (
	scFontDirEnv      = "DOSGOLEM_SC_FONT_DIR"
	scFontExpectedEnv = "DOSGOLEM_SC_FONT_EXPECTED"
)

// envTextFonts 讀 DOSGOLEM_SC_FONT_DIR 的圖集（預期表取 DOSGOLEM_SC_FONT_EXPECTED）。環境變數未設定時回 nil；
// 讀不到 manifest 時 t.Skip；雜湊或比對不符時失敗（011 §3.9）。
func envTextFonts(t *testing.T) *TextFonts {
	t.Helper()
	dir := os.Getenv(scFontDirEnv)
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dir, TextManifestName)); err != nil {
		t.Skip("讀不到圖集 manifest：", err)
	}
	tf, err := LoadTextFonts(dir, os.Getenv(scFontExpectedEnv))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("圖集 %s，anchor %s", dir, tf.Anchor)
	return tf
}

// TestBuiltAtlas 以 Go 重讀 build_noto.sh 以 testdata 測試 catalog 烘的測試圖集（kind = test），
// 對 testdata/opttest_labels.test.tsv 跑 LintLabels（011 §4.6，取代 TestBuiltFont）。正式圖集（anchor match）時 Skip。
func TestBuiltAtlas(t *testing.T) {
	tf := envTextFonts(t)
	if tf == nil {
		t.Skip(scFontDirEnv + " 未設定")
	}
	if tf.Anchor != TextAnchorTest {
		t.Skip("圖集不是測試圖集（anchor " + tf.Anchor + "）")
	}
	b, err := os.ReadFile("testdata/opttest_labels.test.tsv")
	if err != nil {
		t.Fatal(err)
	}
	labl := testLABL("Talk to Gwen", "Transport to Strike Base", "Fly Mission")
	if rep := LintLabels(b, tf, labl); !rep.OK() {
		t.Fatalf("lint：%v", rep.Errors)
	}
	for r := rune(0x20); r <= 0x7E; r++ {
		if !tf.Chars[r] {
			t.Fatalf("ASCII 20h 到 7Eh 應全部收入，缺 %q", r)
		}
	}
	t.Logf("字數 %d", len(tf.Chars))
}
