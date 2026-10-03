package phantasie

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// 定色與有效性閘門的單元測試（docs/spec/001 §8、§10 第 1 項與第 5 項，002 §4）。
// 字型是測試自己合成的 2032 bytes（127 字模 × 16 bytes，每像素 2 位元，值只有 0 與 3），
// 不使用也不讀取任何原版素材。期望值全部是依規格推導的字面值。

// rcPat 是合成字型的字模：8 列 8 欄，'#' 是墨（像素值 3），'.' 是背景（0）。
// 'H' 是 heavy（36 個墨點，每列 5 或 4 個），'l' 是 light（10 個墨點），空白與其他字元全 0。
var rcPat = map[byte][8]string{
	'H': {"#####...", "####....", "#####...", "####....", "#####...", "####....", "#####...", "####...."},
	'l': {"...##...", "...##...", "...##...", "...##...", "...##...", "........", "........", "........"},
	'-': {"........", "........", "........", "########", "########", "........", "........", "........"},
	'+': {"...##...", "...##...", "...##...", "########", "########", "...##...", "...##...", "........"},
}

// rcFont 把 rcPat 編成 2032 bytes：字模 g 的掃描線 r 是 FONT[g×16+2r]、FONT[g×16+2r+1]，高位元在左。
func rcFont() []byte {
	f := make([]byte, 2032)
	for g, pat := range rcPat {
		for r, row := range pat {
			for c := 0; c < 8; c++ {
				if row[c] == '#' {
					f[int(g)*16+2*r+c/4] |= 3 << (uint(3-c%4) * 2)
				}
			}
		}
	}
	return f
}

// rcPal 是 002 §6 的預設調色盤（設模式後）：色號 0 黑、1 淺青、2 淺洋紅、3 白。
var rcPal = [4][3]uint8{{0, 0, 0}, {85, 255, 255}, {255, 85, 255}, {255, 255, 255}}

func rcScreen(bg uint8) []uint8 {
	s := make([]uint8, screenW*screenH)
	for i := range s {
		s[i] = bg
	}
	return s
}

func rcRGB(idx []uint8) []uint8 {
	rgb := make([]uint8, 3*len(idx))
	for i, c := range idx {
		copy(rgb[3*i:], rcPal[c][:])
	}
	return rgb
}

// rcPaint 在 (x0, y0) 起逐字元畫 text：墨像素用 ink，其餘（含不在 rcPat 的字元，例如空白）用 bg。
// 超出畫面下緣的掃描線略過。
func rcPaint(idx []uint8, x0, y0 int, text string, ink, bg uint8) {
	for k := 0; k < len(text); k++ {
		pat, ok := rcPat[text[k]]
		for r := 0; r < 8; r++ {
			if y0+r >= screenH {
				break
			}
			for c := 0; c < 8; c++ {
				v := bg
				if ok && pat[r][c] == '#' {
					v = ink
				}
				idx[(y0+r)*screenW+x0+k*8+c] = v
			}
		}
	}
}

func rcFill(idx []uint8, x0, y0, x1, y1 int, v uint8) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			idx[y*screenW+x] = v
		}
	}
}

func rcRegion(idx []uint8, x0, y0, x1, y1 int) []uint8 {
	var out []uint8
	for y := y0; y < y1; y++ {
		out = append(out, idx[y*screenW+x0:y*screenW+x1]...)
	}
	return out
}

// rcCellPixels 回字元 ch 的字模中，墨（ink 為真）或背景（ink 為假）像素的 (欄, 列)，依列優先順序。
func rcCellPixels(ch byte, ink bool) [][2]int {
	var out [][2]int
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			isInk := false
			if pat, ok := rcPat[ch]; ok {
				isInk = pat[r][c] == '#'
			}
			if isInk == ink {
				out = append(out, [2]int{c, r})
			}
		}
	}
	return out
}

// rcZh 回 n 個互異的全形字。
func rcZh(n int) string { return string([]rune("甲乙丙丁戊己庚辛壬癸")[:n]) }

func rcNew(t *testing.T, pairs ...string) *Overlay {
	t.Helper()
	ui := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		ui[pairs[i]] = pairs[i+1]
	}
	o := NewOverlay()
	o.AddLanguage(&Language{
		Name:    "zh-TW",
		Cat:     NewCatalog(ui, nil, nil),
		Font:    &xlate.Font{Name: "rcfont", W: 16, H: 16, Glyphs: map[rune][]byte{}},
		Wide:    func(r rune) bool { return r >= 0x2E80 },
		Enabled: true,
	})
	o.SetFont(rcFont())
	return o
}

func rcRec(id string, col, row int, text string) *EventRecord {
	return &EventRecord{ID: id, Col: col, Row: row, Text: text, Format: text, FmtKind: KindStatic, Cells: []byte(text)}
}

// rcDraw 以 Begin、End 提交一般字面事件（一般提交的 Pending 疊字，不進閘門）。
func rcDraw(t *testing.T, o *Overlay, col, row int, text string) string {
	t.Helper()
	rec := rcRec("", col, row, text)
	o.Begin(rec, false)
	o.End()
	if o.records[rec.ID] == nil {
		t.Fatalf("事件 %q 沒有提交疊字", text)
	}
	return rec.ID
}

// rcRestore 以 restoreShadow 產生疊字（影子還原的 Pending 疊字，會進閘門）。hidden 是影子的 Hidden。
func rcRestore(t *testing.T, o *Overlay, rec *EventRecord, hidden ...xrange) {
	t.Helper()
	o.records[rec.ID] = rec
	o.restoreShadow(Shadow{{ID: rec.ID, Hidden: hidden}})
	if len(o.Layer.Stamps) == 0 {
		t.Fatalf("restoreShadow 沒有產生疊字（%q）", rec.Text)
	}
}

func rcFrame(o *Overlay, idx []uint8) { o.Frame(idx, rcRGB(idx)) }

// rcViews 把疊字寫成一行一筆：鍵、位置、格數乘格寬、文字、狀態，有透明格時附 T=位元串。
func rcViews(o *Overlay) []string {
	var out []string
	for _, s := range o.Layer.Stamps {
		st := map[xlate.State]string{xlate.Printing: "-", xlate.Pending: "P", xlate.Shown: "S"}[s.State]
		tr := ""
		for i := 0; i < s.Cells; i++ {
			if i < len(s.Transparent) && s.Transparent[i] {
				tr += "1"
			} else {
				tr += "0"
			}
		}
		line := fmt.Sprintf("%s X%d Y%d %dx%d [%s] %s", s.Key, s.X, s.Y, s.Cells, s.CellW, string(s.Text), st)
		if strings.Contains(tr, "1") {
			line += " T=" + tr
		}
		out = append(out, line)
	}
	return out
}

func rcEq(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s\n got:\n  %s\nwant:\n  %s", what, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// rcWant 檢查事件組每筆疊字的 BG、FG 是調色盤的 bg、fg 號。
func rcWant(t *testing.T, o *Overlay, key string, bg, fg uint8) {
	t.Helper()
	n := 0
	for _, s := range o.Layer.Stamps {
		if s.Key != key {
			continue
		}
		n++
		if s.BG != rcPal[bg] || s.FG != rcPal[fg] {
			t.Errorf("%s X%d：BG %v FG %v，要 BG 色號 %d %v、FG 色號 %d %v", key, s.X, s.BG, s.FG, bg, rcPal[bg], fg, rcPal[fg])
		}
	}
	if n == 0 {
		t.Errorf("沒有 %s 的疊字", key)
	}
}

func rcCount(t *testing.T, o *Overlay, name string, want uint64) {
	t.Helper()
	if got := o.C.Get(name); got != want {
		t.Errorf("%s = %d，要 %d", name, got, want)
	}
}

// ---- glyphPixel ----

func TestRecolorGlyphPixelDecode(t *testing.T) {
	f := make([]byte, 2032)
	f[5*16+4], f[5*16+5] = 0xC3, 0x3C   // 字模 5 的掃描線 2：11 00 00 11 | 00 11 11 00
	f[6*16+14], f[6*16+15] = 0x1B, 0xE4 // 字模 6 的掃描線 7：00 01 10 11 | 11 10 01 00（高位元在左）
	f[126*16+15] = 0xC0                 // 最後一個字模的最後一個 byte：欄 4 為 3
	o := NewOverlay()
	o.SetFont(f)
	row := func(g byte, r int) [8]uint8 {
		var out [8]uint8
		for c := range out {
			out[c] = o.glyphPixel(g, r, c)
		}
		return out
	}
	if got, want := row(5, 2), [8]uint8{3, 0, 0, 3, 0, 3, 3, 0}; got != want {
		t.Errorf("字模 5 掃描線 2 = %v，要 %v", got, want)
	}
	if got, want := row(6, 7), [8]uint8{0, 1, 2, 3, 3, 2, 1, 0}; got != want {
		t.Errorf("字模 6 掃描線 7 = %v，要 %v（高位元在左、每像素 2 位元）", got, want)
	}
	if got, want := row(126, 7), [8]uint8{0, 0, 0, 0, 3, 0, 0, 0}; got != want {
		t.Errorf("字模 126 掃描線 7 = %v，要 %v", got, want)
	}
	if got := row(5, 1); got != [8]uint8{} {
		t.Errorf("字模 5 掃描線 1 = %v，要全 0（掃描線之間不互相滲入）", got)
	}
	if got := row(5, 3); got != [8]uint8{} {
		t.Errorf("字模 5 掃描線 3 = %v，要全 0", got)
	}
	// 超出範圍一律 0：字模編號 127 以上、掃描線與欄不在 0 至 7。
	for _, c := range []struct {
		g    byte
		r, c int
	}{{127, 0, 0}, {255, 7, 7}, {5, 8, 0}, {5, -1, 0}, {5, 2, 8}, {5, 2, -1}} {
		if got := o.glyphPixel(c.g, c.r, c.c); got != 0 {
			t.Errorf("glyphPixel(%d, %d, %d) = %d，要 0", c.g, c.r, c.c, got)
		}
	}
}

func TestRecolorSetFontLengthMustBe2032(t *testing.T) {
	o := NewOverlay()
	o.SetFont(rcFont())
	if o.font == nil {
		t.Fatalf("2032 bytes 應接受")
	}
	o.SetFont(make([]byte, 2031))
	if o.font != nil {
		t.Errorf("長度不是 2032 視為沒有字模")
	}
}

// ---- recolor：字模遮罩 ----

func TestRecolorFrameTurnsPendingIntoShownAndRecolors(t *testing.T) {
	o := rcNew(t, "HH", "甲乙")
	rcDraw(t, o, 2, 3, "HH")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HH", 3, 0)
	s := o.Layer.Stamps[0]
	if s.State != xlate.Pending || s.BG != [3]uint8{} || s.FG != [3]uint8{} {
		t.Fatalf("Frame 之前應是 Pending 且未定色：state %d BG %v FG %v", s.State, s.BG, s.FG)
	}
	rcFrame(o, idx)
	rcEq(t, "Frame 之後", rcViews(o), []string{"g1 X16 Y24 2x8 [甲乙] S"})
	rcWant(t, o, "g1", 0, 3)
	rcCount(t, o, "recolor_fallback", 0)
	// 對照：xlate 的多數色規則在這個全形段上判反（墨 72 個像素多於底 56 個）。
	if bg, fg := xlate.Colors(rcRegion(idx, 16, 24, 32, 32)); bg != 3 || fg != 0 {
		t.Errorf("xlate.Colors = (%d, %d)，對照用的期望是判反的 (3, 0)", bg, fg)
	}
}

// 墨色與底色（含反白後互換）在各種字模組合下，recolor 都依遮罩判對；多數色規則在墨點多於一半時判反。
func TestRecolorMaskBeatsMajorityRule(t *testing.T) {
	cases := []struct {
		text     string
		ink, bg  uint8
		majBG    uint8 // xlate.Colors 回的「背景」
		majFG    uint8
		inkCount int
	}{
		// 墨 3、底 0。
		{"H", 3, 0, 3, 0, 36},    // 36 墨對 28 底：判反
		{"HH", 3, 0, 3, 0, 72},   // 72 對 56：判反
		{"HHH", 3, 0, 3, 0, 108}, // 108 對 84：判反
		{"Hl", 3, 0, 0, 3, 46},   // 46 對 82：不反
		{"lHl", 3, 0, 0, 3, 56},
		{"HlHl", 3, 0, 0, 3, 92},
		{"ll", 3, 0, 0, 3, 20},
		{"H  l", 3, 0, 0, 3, 46}, // 中間兩個空白格全是背景
		// 反白後：墨 0、底 3。
		{"H", 0, 3, 0, 3, 36}, // 色號 0 有 36 個像素、色號 3 有 28 個：多數色把 0 當背景，判反
		{"HH", 0, 3, 0, 3, 72},
		{"Hl", 0, 3, 3, 0, 46}, // 色號 3 有 82 個像素：不反
		{"HlHl", 0, 3, 3, 0, 92},
		// 其他色號。
		{"HH", 2, 1, 2, 1, 72},
		{"Hl", 2, 1, 1, 2, 46},
	}
	for _, c := range cases {
		name := fmt.Sprintf("%q 墨%d 底%d", c.text, c.ink, c.bg)
		o := rcNew(t, c.text, rcZh(len(c.text)))
		rcDraw(t, o, 2, 3, c.text)
		idx := rcScreen(c.bg)
		rcPaint(idx, 16, 24, c.text, c.ink, c.bg)
		// 以字面值確認墨像素數與多數色規則的結果（對照組）。
		region := rcRegion(idx, 16, 24, 16+8*len(c.text), 32)
		nInk := 0
		for _, v := range region {
			if v == c.ink {
				nInk++
			}
		}
		if nInk != c.inkCount {
			t.Fatalf("%s：畫面有 %d 個墨像素，要 %d（字模定義錯誤）", name, nInk, c.inkCount)
		}
		if bg, fg := xlate.Colors(region); bg != c.majBG || fg != c.majFG {
			t.Fatalf("%s：xlate.Colors = (%d, %d)，對照用的期望是 (%d, %d)", name, bg, fg, c.majBG, c.majFG)
		}
		rcFrame(o, idx)
		rcWant(t, o, "g1", c.bg, c.ink)
		rcCount(t, o, "recolor_fallback", 0)
	}
}

// 反白前後：墨 3 底 0 與墨 0 底 3 的 fgIdx、bgIdx 對調，不需要追蹤反白狀態。
func TestRecolorFollowsInvert(t *testing.T) {
	o := rcNew(t, "HlHl", "甲乙丙丁")
	rcDraw(t, o, 2, 3, "HlHl")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HlHl", 3, 0)
	rcFrame(o, idx)
	rcWant(t, o, "g1", 0, 3)
	if got := o.Layer.Stamps[0]; got.BG != [3]uint8{0, 0, 0} || got.FG != [3]uint8{255, 255, 255} {
		t.Errorf("反白前 BG %v FG %v，要 BG 黑 FG 白", got.BG, got.FG)
	}

	idx = rcScreen(3)
	rcPaint(idx, 16, 24, "HlHl", 0, 3)
	o.OnInvert(3, 2, 4, false)
	if o.Layer.Stamps[0].State != xlate.Pending {
		t.Fatalf("反白之後應是 Pending")
	}
	rcFrame(o, idx)
	rcWant(t, o, "g1", 3, 0)
	if got := o.Layer.Stamps[0]; got.BG != [3]uint8{255, 255, 255} || got.FG != [3]uint8{0, 0, 0} {
		t.Errorf("反白後 BG %v FG %v，要 BG 白 FG 黑", got.BG, got.FG)
	}
	rcCount(t, o, "recolor_fallback", 0)
}

// 補白用的半形段整組統一：半形段單獨看全是背景，xlate 會把 FG 與 BG 定成同一色。
func TestRecolorUnifiesPaddingSegment(t *testing.T) {
	o := rcNew(t, "H", "甲") // 原文 4 格，譯文 1 個全形字加 6 個半形空白補白
	rcDraw(t, o, 2, 3, "H   ")
	rcEq(t, "疊字", rcViews(o), []string{
		"g1 X16 Y24 1x8 [甲] P",
		"g1 X24 Y24 6x4 [      ] P",
	})
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "H   ", 3, 0)
	rcFrame(o, idx)
	rcWant(t, o, "g1", 0, 3)
	rcCount(t, o, "recolor_fallback", 0)
}

// 前景直方圖為空（沒有任何墨像素）：退回 xlate 定的色並計 recolor_fallback。
func TestRecolorNoInkFallsBack(t *testing.T) {
	o := rcNew(t, "ABCD", "甲乙丙丁") // 合成字型沒有 A、B、C、D 的字模：遮罩全是背景
	rcDraw(t, o, 2, 3, "ABCD")
	rcFrame(o, rcScreen(0))
	rcCount(t, o, "recolor_fallback", 1)
	rcWant(t, o, "g1", 0, 0) // xlate 只看到一種色號：BG、FG 同色

	// 畫面有三種色號：色號 2 最多、1 次之、0 最少。xlate 定 BG=2、FG=1；
	// 沒有墨像素時前景直方圖是空的，不能把「空直方圖的眾數 0」當成前景。
	o = rcNew(t, "ABCD", "甲乙丙丁")
	rcDraw(t, o, 2, 3, "ABCD")
	idx := rcScreen(0)
	rcFill(idx, 16, 24, 48, 32, 2)
	for i := 0; i < 20; i++ {
		idx[24*screenW+16+i] = 1
	}
	for i := 0; i < 5; i++ {
		idx[27*screenW+16+i] = 0
	}
	rcFrame(o, idx)
	rcCount(t, o, "recolor_fallback", 1)
	rcWant(t, o, "g1", 2, 1)
}

// 部分格被覆蓋（Transparent）：只計入非透明格的像素。
func TestRecolorSkipsTransparentCells(t *testing.T) {
	o := rcNew(t, "HHHH", "甲乙丙丁")
	rcDraw(t, o, 2, 3, "HHHH")
	idx := rcScreen(3)
	rcPaint(idx, 16, 24, "H", 3, 0)   // 第 0 格：墨 3 底 0
	rcPaint(idx, 24, 24, "HHH", 0, 3) // 第 1 至 3 格：反白（墨 0 底 3），之後被別的繪製蓋過
	o.Layer.Clear(24, 24, 48, 32)     // 第 1 至 3 格轉透明，剩第 0 格
	rcEq(t, "Clear 之後", rcViews(o), []string{"g1 X16 Y24 4x8 [甲乙丙丁] P T=0111"})
	rcFrame(o, idx)
	// 只看第 0 格：bg 0、fg 3。若把透明格也算進去，3 格反白的像素會讓 bg 判成 3。
	rcWant(t, o, "g1", 0, 3)
}

// 平手取較小色號：兩格各用不同的墨色與底色，每個色號的像素數相同。
func TestRecolorTieTakesSmallerIndex(t *testing.T) {
	o := rcNew(t, "HH", "甲乙")
	rcDraw(t, o, 2, 3, "HH")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "H", 3, 0) // 墨色 3（36）、底色 0（28）
	rcPaint(idx, 24, 24, "H", 2, 1) // 墨色 2（36）、底色 1（28）
	rcFrame(o, idx)
	rcWant(t, o, "g1", 0, 2) // 底：0 與 1 各 28，取 0；前景：3 與 2 各 36，取 2
	rcCount(t, o, "recolor_fallback", 0)
}

// BG、FG 取 rgb 中該色號第一次出現的位置（依疊字、格、列、欄的掃描順序）。
func TestRecolorTakesFirstOccurrenceRGB(t *testing.T) {
	o := rcNew(t, "HH", "甲乙")
	rcDraw(t, o, 2, 3, "HH")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HH", 3, 0)
	rgb := rcRGB(idx)
	set := func(x, y int, c [3]uint8) { copy(rgb[3*(y*screenW+x):], c[:]) }
	// 第 0 格第 0 列第 0 欄是墨（'H' 第 0 列 "#####..." 的第一個像素）。
	set(16, 24, [3]uint8{11, 22, 33})
	// 第 0 格第 0 列第 5 欄是第一個背景像素（"#####..." 的第 6 個字元）。
	set(21, 24, [3]uint8{44, 55, 66})
	o.Frame(idx, rgb)
	s := o.Layer.Stamps[0]
	if s.FG != [3]uint8{11, 22, 33} || s.BG != [3]uint8{44, 55, 66} {
		t.Errorf("FG %v BG %v，要 FG {11 22 33} BG {44 55 66}", s.FG, s.BG)
	}
}

// 4 像素半形段與 8 像素原版格錯位：以像素為單位判定，與疊字格是否對齊原版格無關。
func TestRecolorHalfWidthSegmentsMisalignedWithOriginalCells(t *testing.T) {
	o := rcNew(t, "HlHl", "甲a乙") // 5 h 加 3 個半形空白補到 8 h
	rcDraw(t, o, 2, 3, "HlHl")
	rcEq(t, "疊字", rcViews(o), []string{
		"g1 X16 Y24 1x8 [甲] P",
		"g1 X24 Y24 1x4 [a] P",
		"g1 X28 Y24 1x8 [乙] P", // 跨原版格 1 與 2
		"g1 X36 Y24 3x4 [   ] P",
	})
	rec := o.records["g1"]
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HlHl", 3, 0)

	type px struct{ x, y int }
	scan := func() map[px][2]uint8 {
		got := map[px][2]uint8{}
		ok := o.scanGroup(rec, o.groupStamps("g1"), idx, func(k int, m, c uint8, x, y int) {
			if _, dup := got[px{x, y}]; dup {
				t.Errorf("像素 (%d,%d) 被走訪兩次", x, y)
			}
			got[px{x, y}] = [2]uint8{uint8(k), m}
		})
		if !ok {
			t.Fatalf("scanGroup 回 false")
		}
		return got
	}
	check := func(name string, got map[px][2]uint8, skipX0, skipX1 int) {
		want := 0
		for y := 24; y < 32; y++ {
			for x := 16; x < 48; x++ {
				if x >= skipX0 && x < skipX1 {
					if _, seen := got[px{x, y}]; seen {
						t.Errorf("%s：透明格內的像素 (%d,%d) 不該走訪", name, x, y)
					}
					continue
				}
				want++
				k := (x - 16) / 8
				var m uint8
				if pat, ok := rcPat["HlHl"[k]]; ok && pat[y-24][(x-16)%8] == '#' {
					m = 3
				}
				if v, seen := got[px{x, y}]; !seen || v != [2]uint8{uint8(k), m} {
					t.Errorf("%s：像素 (%d,%d) = %v（%v），要 [原版格 %d, 遮罩 %d]", name, x, y, v, seen, k, m)
					return
				}
			}
		}
		if len(got) != want {
			t.Errorf("%s：走訪 %d 個像素，要 %d", name, len(got), want)
		}
	}
	check("全部不透明", scan(), 0, 0)
	// 跨格的全形字（X28..36）轉透明：這 8 欄的像素不計入。
	o.groupStamps("g1")[2].Transparent = []bool{true}
	check("跨格全形字透明", scan(), 28, 36)
	o.groupStamps("g1")[2].Transparent = nil

	rcFrame(o, idx)
	rcWant(t, o, "g1", 0, 3)
	rcCount(t, o, "recolor_fallback", 0)
}

// ---- 捲動序列 ----

// 疊字經 Layer.Scroll 上移一列後再反白：recolor 取疊字的 Y 為 y0，顏色仍由遮罩決定。
func TestRecolorScrollThenInvertUsesStampY(t *testing.T) {
	o := rcNew(t, "HH", "甲乙")
	rcDraw(t, o, 2, 6, "HH") // Y=48
	idx := rcScreen(0)
	rcPaint(idx, 16, 48, "HH", 3, 0)
	rcFrame(o, idx)
	rcWant(t, o, "g1", 0, 3)

	o.OnInt10(0x0601, 0, 5<<8|2, 9<<8|17) // 第 5 至 9 列、第 2 至 17 欄上捲 1 列
	rcEq(t, "上捲之後", rcViews(o), []string{"g1 X16 Y40 2x8 [甲乙] S"})

	// 原版把畫面上捲後，在第 5 列以反白畫出；事件記錄的第 6 列（Y=48）現在是下一列的內容（色號 1 的純色）。
	idx2 := rcScreen(0)
	rcFill(idx2, 16, 48, 32, 56, 1)
	rcFill(idx2, 16, 40, 32, 48, 3)
	rcPaint(idx2, 16, 40, "HH", 0, 3)
	o.OnInvert(5, 2, 2, false)
	rcFrame(o, idx2)
	rcWant(t, o, "g1", 3, 0) // 反白：底 3、墨 0；xlate 的多數色規則在這裡判反（色號 0 有 72 個像素）
	rcCount(t, o, "recolor_fallback", 0)
}

// 同組各疊字的 Y 不一致：不做遮罩判定，沿用 xlate 定的色並計 recolor_fallback。
func TestRecolorInconsistentStampYFallsBack(t *testing.T) {
	o := rcNew(t, "H", "甲")
	rcDraw(t, o, 2, 5, "H   ") // 全形段 X16 1 格，半形段 X24 6 格；Y=40
	// 視窗只涵蓋 x16..24：全形段上移到 Y=32，半形段不動。
	o.OnInt10(0x0601, 0, 3<<8|2, 7<<8|2)
	rcEq(t, "部分疊字被捲動", rcViews(o), []string{
		"g1 X16 Y32 1x8 [甲] P",
		"g1 X24 Y40 6x4 [      ] P",
	})
	idx := rcScreen(0)
	rcPaint(idx, 16, 32, "H", 3, 0)
	o.OnInvert(4, 2, 1, false)
	rcFrame(o, idx)
	rcCount(t, o, "recolor_fallback", 1)
	s := o.Layer.Stamps
	if len(s) != 2 {
		t.Fatalf("疊字有 %d 筆，要 2 筆", len(s))
	}
	// 沿用 xlate 各疊字自己的定色：全形段只有 'H' 一格，墨多於底而判反；半形段全是背景，BG 與 FG 同色。
	if s[0].BG != rcPal[3] || s[0].FG != rcPal[0] {
		t.Errorf("全形段 BG %v FG %v，要 xlate 的判反結果 BG 白 FG 黑", s[0].BG, s[0].FG)
	}
	if s[1].BG != rcPal[0] || s[1].FG != rcPal[0] {
		t.Errorf("半形段 BG %v FG %v，要 BG、FG 都是黑", s[1].BG, s[1].FG)
	}
}

// ---- Frame 與字型 ----

func TestRecolorNoFontKeepsXlateColors(t *testing.T) {
	o := rcNew(t, "HH", "甲乙")
	o.SetFont(make([]byte, 2031)) // 長度不符：視為沒有字模
	rcDraw(t, o, 2, 3, "HH")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HH", 3, 0)
	rcFrame(o, idx)
	rcEq(t, "Frame 之後", rcViews(o), []string{"g1 X16 Y24 2x8 [甲乙] S"})
	// 沒有字模時不 recolor：留下 xlate 判反的結果（BG 白、FG 黑），也不計 recolor_fallback。
	rcWant(t, o, "g1", 3, 0)
	rcCount(t, o, "recolor_fallback", 0)
}

func TestRecolorOnlyGroupsNewlyShown(t *testing.T) {
	o := rcNew(t, "HH", "甲乙")
	rcDraw(t, o, 2, 3, "HH")  // g1
	rcDraw(t, o, 10, 3, "HH") // g2：同一個譯文，不同位置
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HH", 3, 0)
	rcPaint(idx, 80, 24, "HH", 3, 0)
	rcFrame(o, idx)
	rcWant(t, o, "g1", 0, 3)
	rcWant(t, o, "g2", 0, 3)
	sentinel := [3]uint8{9, 9, 9}
	for _, s := range o.Layer.Stamps {
		s.BG, s.FG = sentinel, sentinel
	}
	o.OnInvert(3, 10, 2, false) // 只有 g2 改 Pending
	rcFrame(o, idx)
	for _, s := range o.Layer.Stamps {
		switch s.Key {
		case "g1":
			if s.BG != sentinel || s.FG != sentinel {
				t.Errorf("g1 沒有轉成 Shown，不該重新定色：BG %v FG %v", s.BG, s.FG)
			}
		case "g2":
			if s.BG != rcPal[0] || s.FG != rcPal[3] {
				t.Errorf("g2 轉成 Shown，應重新定色：BG %v FG %v", s.BG, s.FG)
			}
		}
	}
}

// ---- 有效性閘門（只對 restoreShadow 的 Pending 疊字執行） ----

func TestGateMarksUnrelatedCellTransparent(t *testing.T) {
	o := rcNew(t, "HlHl", "甲乙丙丁")
	rcRestore(t, o, rcRec("g1", 2, 3, "HlHl"))
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HlHl", 3, 0)
	rcFill(idx, 32, 24, 40, 32, 2) // 第 2 格（'H'）被與字模無關的圖樣蓋掉
	rcFrame(o, idx)
	rcEq(t, "第 2 格轉透明", rcViews(o), []string{"g1 X16 Y24 4x8 [甲乙丙丁] S T=0010"})
	rcCount(t, o, "inconsistent_cells", 1)
	rcCount(t, o, "inconsistent_groups", 0)
	rcWant(t, o, "g1", 0, 3)
	if len(o.gate) != 0 {
		t.Errorf("Frame 之後閘門集合應清空，還有 %d 筆", len(o.gate))
	}
	// 閘門只在還原後的第一個 Frame 執行：之後別的格被蓋掉，交給指紋偵測，不再被閘門標透明。
	rcFill(idx, 24, 24, 32, 32, 2)
	rcFrame(o, idx)
	rcEq(t, "第二次 Frame", rcViews(o), []string{"g1 X16 Y24 4x8 [甲乙丙丁] S T=0010"})
	rcCount(t, o, "inconsistent_cells", 1)
}

// 一般提交的 Pending 疊字與畫面由構造一致，不執行閘門：同樣的畫面、同樣的損壞，格仍不透明。
func TestGateNotRunForNormalCommit(t *testing.T) {
	o := rcNew(t, "HlHl", "甲乙丙丁")
	rcDraw(t, o, 2, 3, "HlHl")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HlHl", 3, 0)
	rcFill(idx, 32, 24, 40, 32, 2)
	rcFrame(o, idx)
	rcEq(t, "一般提交", rcViews(o), []string{"g1 X16 Y24 4x8 [甲乙丙丁] S"})
	rcCount(t, o, "inconsistent_cells", 0)
	rcCount(t, o, "inconsistent_groups", 0)
}

// 語言切換重建的 Pending 疊字不進閘門。
func TestGateNotRunForRebuiltStamps(t *testing.T) {
	o := rcNew(t, "HlHl", "甲乙丙丁")
	o.AddLanguage(&Language{
		Name:    "zh-CN",
		Cat:     NewCatalog(map[string]string{"HlHl": "子丑寅卯"}, nil, nil),
		Font:    &xlate.Font{Name: "rcfont-cn", W: 16, H: 16, Glyphs: map[rune][]byte{}},
		Wide:    func(r rune) bool { return r >= 0x2E80 },
		Enabled: true,
	})
	rcDraw(t, o, 2, 3, "HlHl")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HlHl", 3, 0)
	rcFrame(o, idx)
	rcFill(idx, 32, 24, 40, 32, 2)
	if err := o.SetDisplay("zh-CN"); err != nil {
		t.Fatal(err)
	}
	rcEq(t, "重建之後", rcViews(o), []string{"g1 X16 Y24 4x8 [子丑寅卯] P"})
	rcFrame(o, idx)
	rcEq(t, "重建之後的 Frame", rcViews(o), []string{"g1 X16 Y24 4x8 [子丑寅卯] S"})
	rcCount(t, o, "inconsistent_cells", 0)
}

// P 類重建（選項開關符號）的 Pending 疊字不進閘門：%c 事件之後、反白之前畫面是暫態。
func TestGateNotRunForPatchRebuild(t *testing.T) {
	o := rcNew(t, "+Sound", "+音效", "-Sound", "-音效")
	old := rcRec("", 2, 3, "+Sound ")
	old.Format = "%s"
	old.ArgStrs = []ArgStr{{Ptr: 0x1234, Content: "+Sound ", Kind: KindStatic}}
	o.Begin(old, false)
	o.End()
	rcEq(t, "提交之後", rcViews(o), []string{
		"g1 X16 Y24 1x4 [+] P",
		"g1 X20 Y24 2x8 [音效] P",
		"g1 X36 Y24 9x4 [         ] P",
	})
	idx := rcScreen(0)
	rcFrame(o, idx)
	// %c 事件把第 0 格改成 '-'；畫面是暫態：第 0 格先被別的圖樣蓋過。
	sym := rcRec("", 2, 3, "-")
	o.Begin(sym, false)
	o.End()
	rcCount(t, o, "patched", 1)
	rcEq(t, "P 類重建", rcViews(o), []string{
		"g2 X16 Y24 1x4 [-] P",
		"g2 X20 Y24 2x8 [音效] P",
		"g2 X36 Y24 9x4 [         ] P",
	})
	rcPaint(idx, 16, 24, "-Sound ", 3, 0)
	rcFill(idx, 16, 24, 24, 32, 2)
	rcFrame(o, idx)
	rcEq(t, "Frame 之後符號格仍不透明", rcViews(o), []string{
		"g2 X16 Y24 1x4 [-] S",
		"g2 X20 Y24 2x8 [音效] S",
		"g2 X36 Y24 9x4 [         ] S",
	})
	rcCount(t, o, "inconsistent_cells", 0)
	rcCount(t, o, "inconsistent_groups", 0)
}

// 整組被同一色號填滿 90% 以上（bgIdx = fgIdx）：整組移除；剛好 90% 以下則不移除，退回 xlate 的色。
func TestGateRemovesGroupFilledWithOneColor(t *testing.T) {
	// 'H' 第 0 格的背景像素（28 個）中，前 k 個畫成色號 1，其餘全畫成色號 2。
	// 256 個像素中同色號（2）的有 256-k 個：10×(256-k) ≥ 9×256 當且僅當 k ≤ 25。
	build := func(k int) (*Overlay, []uint8) {
		o := rcNew(t, "HlHl", "甲乙丙丁")
		rcRestore(t, o, rcRec("g1", 2, 3, "HlHl"))
		idx := rcScreen(0)
		rcFill(idx, 16, 24, 48, 32, 2)
		for i, p := range rcCellPixels('H', false) {
			if i < k {
				idx[(24+p[1])*screenW+16+p[0]] = 1
			}
		}
		return o, idx
	}
	for _, k := range []int{0, 25} {
		o, idx := build(k)
		rcFrame(o, idx)
		if len(o.Layer.Stamps) != 0 {
			t.Errorf("k=%d：同色號 %d%% 以上應整組移除，還剩 %v", k, 90, rcViews(o))
		}
		rcCount(t, o, "inconsistent_groups", 1)
		rcCount(t, o, "recolor_fallback", 0)
	}
	o, idx := build(26)
	rcFrame(o, idx)
	rcEq(t, "k=26（同色號 230/256 不足 90%）不移除", rcViews(o), []string{"g1 X16 Y24 4x8 [甲乙丙丁] S"})
	rcCount(t, o, "inconsistent_groups", 0)
	rcCount(t, o, "recolor_fallback", 1)

	// 對照：一般提交的同一個畫面不執行閘門，疊字不移除（k=0：整格同一色號）。
	o = rcNew(t, "HlHl", "甲乙丙丁")
	rcDraw(t, o, 2, 3, "HlHl")
	rcFrame(o, func() []uint8 { s := rcScreen(0); rcFill(s, 16, 24, 48, 32, 2); return s }())
	rcEq(t, "一般提交不執行閘門", rcViews(o), []string{"g1 X16 Y24 4x8 [甲乙丙丁] S"})
	rcCount(t, o, "inconsistent_groups", 0)
	rcCount(t, o, "recolor_fallback", 1)
}

// 反白前後混合的組（bgIdx = fgIdx 但同色號不足 90%）：不移除，計 recolor_fallback。
func TestGateKeepsMixedInvertedGroup(t *testing.T) {
	o := rcNew(t, "HH", "甲乙")
	rcRestore(t, o, rcRec("g1", 2, 3, "HH"))
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "H", 3, 0) // 第 0 格正常：墨 3 底 0
	rcPaint(idx, 24, 24, "H", 0, 3) // 第 1 格反白：墨 0 底 3
	rcFrame(o, idx)
	// 底：色號 0 與 3 各 28，取 0；前景：3 與 0 各 36，取 0。整組取最多數時 bgIdx = fgIdx = 0，
	// 改取各格的 (底, 墨) 配對：(0, 3) 與 (3, 0) 各一格，同數取最左格者為組色，第 1 格是反白狀態。
	// 兩格狀態不同，疊字切成兩片各自定色；組不移除、不退回 xlate 的色。
	rcEq(t, "混合的組不移除，依狀態切開", rcViews(o), []string{
		"g1 X16 Y24 1x8 [甲] S",
		"g1 X24 Y24 1x8 [乙] S",
	})
	if len(o.Layer.Stamps) != 2 {
		t.Fatalf("疊字有 %d 片，要 2", len(o.Layer.Stamps))
	}
	if s := o.Layer.Stamps[0]; s.BG != rcPal[0] || s.FG != rcPal[3] {
		t.Errorf("正常片 BG %v FG %v，要 BG 黑 FG 白", s.BG, s.FG)
	}
	if s := o.Layer.Stamps[1]; s.BG != rcPal[3] || s.FG != rcPal[0] {
		t.Errorf("反白片 BG %v FG %v，要 BG 白 FG 黑", s.BG, s.FG)
	}
	rcCount(t, o, "recolor_split", 1)
	rcCount(t, o, "recolor_fallback", 0)
	rcCount(t, o, "inconsistent_groups", 0)
	rcCount(t, o, "inconsistent_cells", 0)
}

// 每一格都與 (bgIdx, fgIdx) 不一致：整組全部格轉透明，移除。
func TestGateRemovesGroupWhenEveryCellInconsistent(t *testing.T) {
	o := rcNew(t, "HH", "甲乙")
	rcRestore(t, o, rcRec("g1", 2, 3, "HH"))
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "H", 3, 0) // 墨 3 底 0
	rcPaint(idx, 24, 24, "H", 2, 1) // 墨 2 底 1
	rcFrame(o, idx)
	// bgIdx = 0（0 與 1 各 28，取小）、fgIdx = 2（3 與 2 各 36，取小）：
	// 第 0 格配對 28/64，第 1 格配對 36/64，都低於 70%。
	if len(o.Layer.Stamps) != 0 {
		t.Errorf("整組應移除，還剩 %v", rcViews(o))
	}
	rcCount(t, o, "inconsistent_cells", 2)
	rcCount(t, o, "inconsistent_groups", 1)
	rcCount(t, o, "recolor_fallback", 0)
}

// 墨像素不少於 6 個時，色號等於 fgIdx 的比例不得低於 30%（整數比較 10×保留 ≥ 3×墨）。
func TestGateInkRetentionBoundary(t *testing.T) {
	for _, c := range []struct {
		keep int
		bad  bool
	}{{3, false}, {2, true}, {0, true}} {
		o := rcNew(t, "HlHl", "甲乙丙丁")
		rcRestore(t, o, rcRec("g1", 2, 3, "HlHl"))
		idx := rcScreen(0)
		rcPaint(idx, 16, 24, "HlHl", 3, 0)
		// 第 1 格（'l'，10 個墨像素）被空白蓋掉，只保留前 keep 個墨像素。
		rcPaint(idx, 24, 24, " ", 3, 0)
		for i, p := range rcCellPixels('l', true) {
			if i < c.keep {
				idx[(24+p[1])*screenW+24+p[0]] = 3
			}
		}
		rcFrame(o, idx)
		want := "g1 X16 Y24 4x8 [甲乙丙丁] S"
		cells := uint64(0)
		if c.bad {
			want += " T=0100"
			cells = 1
		}
		rcEq(t, fmt.Sprintf("保留 %d/10", c.keep), rcViews(o), []string{want})
		rcCount(t, o, "inconsistent_cells", cells)
	}
}

// 配對比例不得低於 70%（整數比較 10×配對 ≥ 7×像素，64 個像素需 45 個）。
func TestGateMatchRatioBoundary(t *testing.T) {
	for _, c := range []struct {
		wrong int
		bad   bool
	}{{19, false}, {20, true}} {
		o := rcNew(t, "HlHl", "甲乙丙丁")
		rcRestore(t, o, rcRec("g1", 2, 3, "HlHl"))
		idx := rcScreen(0)
		rcPaint(idx, 16, 24, "HlHl", 3, 0)
		// 第 1 格（'l'）的背景像素中前 wrong 個畫成色號 2：配對 64-wrong，墨像素 10 個全部保留。
		for i, p := range rcCellPixels('l', false) {
			if i < c.wrong {
				idx[(24+p[1])*screenW+24+p[0]] = 2
			}
		}
		rcFrame(o, idx)
		want := "g1 X16 Y24 4x8 [甲乙丙丁] S"
		cells := uint64(0)
		if c.bad {
			want += " T=0100"
			cells = 1
		}
		rcEq(t, fmt.Sprintf("錯 %d 個像素（配對 %d/64）", c.wrong, 64-c.wrong), rcViews(o), []string{want})
		rcCount(t, o, "inconsistent_cells", cells)
	}
}

// 全是空白字元的格要求同一色號像素不少於 85%（64 個像素需 55 個）。
func TestGateSpaceCellBoundary(t *testing.T) {
	for _, c := range []struct {
		other int // 空白格內畫成色號 2 的像素數
		bad   bool
	}{{9, false}, {10, true}} {
		o := rcNew(t, "H l", "甲乙丙")
		rcRestore(t, o, rcRec("g1", 2, 3, "H l"))
		idx := rcScreen(0)
		rcPaint(idx, 16, 24, "H l", 3, 0)
		for i := 0; i < c.other; i++ {
			idx[(24+i/8)*screenW+24+i%8] = 2 // 第 1 格（空白）由左上起、列優先的 other 個像素
		}
		rcFrame(o, idx)
		want := "g1 X16 Y24 3x8 [甲乙丙] S"
		cells := uint64(0)
		if c.bad {
			want += " T=010"
			cells = 1
		}
		rcEq(t, fmt.Sprintf("空白格 %d/64 個像素被蓋", c.other), rcViews(o), []string{want})
		rcCount(t, o, "inconsistent_cells", cells)
	}
}

// 停用項目變暗（格內第 2、4 條掃描線歸零）不被誤判：整格配對率與墨保留率都高於門檻。
func TestGateDimmedCellsNotMisjudged(t *testing.T) {
	o := rcNew(t, "HlHl", "甲乙丙丁")
	rcRestore(t, o, rcRec("g1", 2, 3, "HlHl"))
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HlHl", 3, 0)
	for _, r := range []int{1, 3} { // 第 2、4 條掃描線
		rcFill(idx, 16, 24+r, 48, 24+r+1, 0)
	}
	// 'H' 配對 56/64、墨保留 28/36；'l' 配對 60/64、墨保留 6/10。
	rcFrame(o, idx)
	rcEq(t, "變暗", rcViews(o), []string{"g1 X16 Y24 4x8 [甲乙丙丁] S"})
	rcCount(t, o, "inconsistent_cells", 0)
	rcCount(t, o, "inconsistent_groups", 0)
	rcWant(t, o, "g1", 0, 3)
}

// 半格：原版格只有一半被非透明格覆蓋（32 個像素，不少於 24 個）時仍評估。
func TestGateHalfCell(t *testing.T) {
	setup := func() (*Overlay, []uint8) {
		o := rcNew(t, "HlHl", "abcdefgh") // 8 個半形字：單一疊字 8 格，每格 4 像素
		// 影子的 Hidden 蓋住 x20..24：疊字格 1 透明，原版格 0（x16..24）只剩左半被覆蓋。
		rcRestore(t, o, rcRec("g1", 2, 3, "HlHl"), xrange{20, 24})
		rcEq(t, "還原", rcViews(o), []string{"g1 X16 Y24 8x4 [abcdefgh] P T=01000000"})
		idx := rcScreen(0)
		rcPaint(idx, 16, 24, "HlHl", 3, 0)
		return o, idx
	}
	// 一致：左半（欄 0 至 3）的 'H' 全是墨，配對 32/32。
	o, idx := setup()
	rcFrame(o, idx)
	rcEq(t, "半格一致", rcViews(o), []string{"g1 X16 Y24 8x4 [abcdefgh] S T=01000000"})
	rcCount(t, o, "inconsistent_cells", 0)
	// 左半被與字模無關的圖樣蓋掉：該原版格（只剩 32 個像素）仍被評估，兩個疊字格（含已透明的）都為透明。
	o, idx = setup()
	rcFill(idx, 16, 24, 20, 32, 2)
	rcFrame(o, idx)
	rcEq(t, "半格被蓋", rcViews(o), []string{"g1 X16 Y24 8x4 [abcdefgh] S T=11000000"})
	rcCount(t, o, "inconsistent_cells", 1)
	// 變暗：左半的墨保留 24/32，配對 24/32，不誤判。
	o, idx = setup()
	for _, r := range []int{1, 3} {
		rcFill(idx, 16, 24+r, 20, 24+r+1, 0)
	}
	rcFrame(o, idx)
	rcEq(t, "半格變暗", rcViews(o), []string{"g1 X16 Y24 8x4 [abcdefgh] S T=01000000"})
	rcCount(t, o, "inconsistent_cells", 0)
}

// 原版格被非透明格覆蓋的像素少於 24 個時略過（視為一致）：疊字的 Y 有位移而下緣被畫面截掉，
// 半格只剩 5 條掃描線（20 個像素）時略過，剩 6 條（24 個像素）時照常評估。
func TestGateMinPixelsBoundary(t *testing.T) {
	for _, c := range []struct {
		dy   int // 疊字 Y = 192 + dy，畫面內可見的掃描線數 = 8 - dy
		bad  bool
		rows int
	}{{3, false, 5}, {2, true, 6}} {
		o := rcNew(t, "HlHl", "abcdefgh")
		rec := rcRec("g1", 2, 24, "HlHl")
		o.records[rec.ID] = rec
		o.restoreShadow(Shadow{{ID: rec.ID, DY: c.dy, Hidden: []xrange{{20, 24}}}})
		y := 192 + c.dy
		idx := rcScreen(0)
		rcPaint(idx, 16, y, "HlHl", 3, 0)
		rcFill(idx, 16, y, 20, screenH, 2) // 原版格 0 的左半（唯一被覆蓋的一半）被與字模無關的圖樣蓋掉
		rcFrame(o, idx)
		want := fmt.Sprintf("g1 X16 Y%d 8x4 [abcdefgh] S T=01000000", y)
		cells := uint64(0)
		if c.bad {
			want = fmt.Sprintf("g1 X16 Y%d 8x4 [abcdefgh] S T=11000000", y)
			cells = 1
		}
		rcEq(t, fmt.Sprintf("半格可見 %d 條掃描線（%d 個像素）", c.rows, 4*c.rows), rcViews(o), []string{want})
		rcCount(t, o, "inconsistent_cells", cells)
	}
}

// ---- 事件只有一部分被反白 ----

// 原版以 invert 只反白一個事件的前幾格（例如旅店分配畫面的名字與職業，同一列的數字不反白）。
// 疊字依各格反白狀態切開，各片以自己的狀態定色；稽核不把未反白的格判成殘字。
func TestRecolorSplitsPartiallyInvertedGroup(t *testing.T) {
	o := rcNew(t, "HHHHl", "甲乙丙丁戊")
	rcDraw(t, o, 2, 3, "HHHHl")
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HHH", 0, 3) // 第 0 至 2 格反白：墨 0 底 3
	rcPaint(idx, 40, 24, "Hl", 3, 0)  // 第 3、4 格正常：墨 3 底 0
	rcFrame(o, idx)
	rcEq(t, "依狀態切開", rcViews(o), []string{
		"g1 X16 Y24 3x8 [甲乙丙] S",
		"g1 X40 Y24 2x8 [丁戊] S",
	})
	rcCount(t, o, "recolor_split", 1)
	rcCount(t, o, "recolor_fallback", 0)
	if len(o.Layer.Stamps) != 2 {
		t.Fatalf("疊字有 %d 片，要 2", len(o.Layer.Stamps))
	}
	if s := o.Layer.Stamps[0]; s.BG != rcPal[3] || s.FG != rcPal[0] {
		t.Errorf("反白片 BG %v FG %v，要 BG 白 FG 黑", s.BG, s.FG)
	}
	if s := o.Layer.Stamps[1]; s.BG != rcPal[0] || s.FG != rcPal[3] {
		t.Errorf("正常片 BG %v FG %v，要 BG 黑 FG 白", s.BG, s.FG)
	}
	if got := o.AuditStale(idx); got != 0 {
		t.Errorf("AuditStale = %d，要 0（未反白的格不是殘字）", got)
	}
	if ex, st := o.AuditEvents(idx); ex != 0 || st != 0 {
		t.Errorf("AuditEvents = (%d, %d)，要 (0, 0)", ex, st)
	}

	// 反白解除：全部回到正常色，已切開的疊字維持兩片。
	idx = rcScreen(0)
	rcPaint(idx, 16, 24, "HHHHl", 3, 0)
	o.OnInvert(3, 2, 5, false)
	rcFrame(o, idx)
	rcEq(t, "解除後", rcViews(o), []string{
		"g1 X16 Y24 3x8 [甲乙丙] S",
		"g1 X40 Y24 2x8 [丁戊] S",
	})
	for _, s := range o.Layer.Stamps {
		if s.BG != rcPal[0] || s.FG != rcPal[3] {
			t.Errorf("X%d 解除後 BG %v FG %v，要 BG 黑 FG 白", s.X, s.BG, s.FG)
		}
	}
	rcCount(t, o, "recolor_split", 1)
}

// 切開處落在同一個疊字內的透明格（玩家輸入）不另外切；透明格沿用前一格的狀態。
func TestRecolorSplitKeepsTransparentCells(t *testing.T) {
	o := rcNew(t, "HHHHl", "甲乙丙丁戊")
	rcDraw(t, o, 2, 3, "HHHHl")
	o.Layer.Stamps[0].Transparent = []bool{false, false, true, false, false}
	idx := rcScreen(0)
	rcPaint(idx, 16, 24, "HHH", 0, 3)
	rcPaint(idx, 40, 24, "Hl", 3, 0)
	rcFrame(o, idx)
	rcEq(t, "透明格留在前一片", rcViews(o), []string{
		"g1 X16 Y24 3x8 [甲乙丙] S T=001",
		"g1 X40 Y24 2x8 [丁戊] S",
	})
}
