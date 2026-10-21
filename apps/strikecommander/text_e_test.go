package strikecommander

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// strike_commander 規格 011 §4.1 的 (e) 部分：第 5、7 項的計分板與飛行、第 8、12、15 項、第 9 項的繪製部分。
// 期望值在測試內獨立寫出，字型用 testTextFonts。

// ---- 第 5 項：計分板（§3.6.2） ----

func TestBoardTextExample(t *testing.T) {
	fonts := testTextFonts(t)
	for _, c := range []struct {
		key        string
		x0, x1, tx int
		tr         string
	}{
		{"GROUND", 177, 250, 411, "地面"}, // ⌊(854 − 32) / 2⌋
		{"AIR", 259, 290, 533, "空中"},
	} {
		l := Label{Key: c.key, X0: c.x0, X1: c.x1, Y: 100, Kind: KindBoard, Font: boardFont}
		p, err := PlanLabelText(l, []rune(c.tr), 2, fonts)
		if err != nil {
			t.Fatal(err)
		}
		if p.Text.tx != c.tx || p.Text.px != 16 || p.Text.outline || p.Text.fgIdx != boardFGIndex || p.Kind != KindBoard {
			t.Errorf("%s：tx %d px %d outline %v fg %d，要 tx %d px 16 無外框 fg 10", c.key, p.Text.tx, p.Text.px, p.Text.outline, p.Text.fgIdx, c.tx)
		}
		// 字帶 [Y+1, Y+9)：2 倍 H 16、px 16，ey = 202，baseline 216；遮蓋就是游標範圍。
		if p.Text.baseline != 216 || p.Cover.X != c.x0 || p.Cover.CellW != c.x1-c.x0 || p.Cover.CellH != 8 {
			t.Errorf("%s：baseline %d 遮蓋 %d+%d×%d", c.key, p.Text.baseline, p.Cover.X, p.Cover.CellW, p.Cover.CellH)
		}
	}
	// 可用範圍是游標：放不下時 Skip。
	if _, err := PlanLabelText(Label{Key: "AIR", X0: 259, X1: 290, Y: 100, Kind: KindBoard}, []rune("空中空中"), 2, fonts); err == nil {
		t.Fatal("4 字 64 > 62 應 Skip")
	}
}

// ---- 飛行的測試資料 ----

// hudText 回一筆 3 倍 HUD 字串：band [Y+1, Y+6)，Before 全 0；After 在 ink 各欄設墨跡色。
func flightFixture(kind Kind, catKey string, xs []int, x1, y int, ink map[int]uint8) FlightText {
	b0, bh := flightBand(kind, y)
	t := FlightText{Key: catKey, CatKey: catKey, Kind: kind, Xs: xs, X1: x1, Y: y, B0: b0, BH: bh}
	if len(xs) > 0 {
		t.X0 = xs[0]
	}
	t.Before, t.After = make([]byte, bh*320), make([]byte, bh*320)
	for x, c := range ink {
		t.After[2*320+x] = c
	}
	return t
}

func flightTextOverlay(t *testing.T, fonts *TextFonts, scale int) *FlightOverlay {
	t.Helper()
	cats := FlightCatalogs{
		HUD:    map[string]string{"SRM": "短程飛彈", "GEAR": "短"},
		MFD:    map[string]string{"request status": "回報狀態", "Comm Mode:": "通訊模式："},
		Plaque: map[string]string{"time burst": "測試"},
	}
	fo, err := NewFlightOverlayText(fonts, cats, scale)
	if err != nil {
		t.Fatal(err)
	}
	return fo
}

// ---- 第 5 項：HUD、MFD（§3.6.4、§3.6.5） ----

func TestFlightTextExamples(t *testing.T) {
	fonts := testTextFonts(t)
	fo := flightTextOverlay(t, fonts, 3)
	// SRM：Xs[k] 118、X1 130，「短程飛彈」W 60，置中 342，推回到 lo = 355（可用 [118, 150)，hi = 449）。
	e, miss, skip := fo.planText(0, flightFixture(KindHUD, "SRM", []int{118}, 130, 50, map[int]uint8{120: 223}))
	if miss != nil || skip != nil {
		t.Fatalf("SRM：%+v %+v", miss, skip)
	}
	if e.ti.tx != 355 || e.ti.px != 15 || !e.ti.outline || e.ti.baseline != 51*3+13 {
		t.Errorf("SRM：tx %d px %d outline %v baseline %d，要 355、15、有外框、%d", e.ti.tx, e.ti.px, e.ti.outline, e.ti.baseline, 51*3+13)
	}
	// 2 倍不畫 HUD。
	if _, _, skip := flightTextOverlay(t, fonts, 2).planText(0, flightFixture(KindHUD, "SRM", []int{118}, 130, 50, map[int]uint8{120: 223})); skip != errNoStamp {
		t.Errorf("2 倍 HUD 應不產生（%+v）", skip)
	}
	// MFD request status（k > 0）：可用範圍左界 Xs[k] 230；英文 [230, 286)，置中 tx = ⌊(1032 − 64) / 2⌋ = 484（靠左會是 460）。
	fo2 := flightTextOverlay(t, fonts, 2)
	mt := flightFixture(KindMFD, "request status", []int{214, 222, 230}, 286, 60, map[int]uint8{240: 5, 232: 5})
	mt.K = 2
	e, miss, skip = fo2.planText(0, mt)
	if miss != nil || skip != nil {
		t.Fatalf("MFD：%+v %+v", miss, skip)
	}
	if e.ti.tx != 484 || e.ti.clip.X0 != 460 || e.ti.clip.X1 != 612 || e.ti.baseline != 59*2+14 || e.ti.outline {
		t.Errorf("MFD：tx %d clip %+v baseline %d", e.ti.tx, e.ti.clip, e.ti.baseline)
	}
	// 英文範圍很窄時推回到左界 Xs[k]，中文不會蓋到數字前綴。
	mt.X1 = 236
	e, _, _ = fo2.planText(0, mt)
	if e.ti.tx != 460 || e.cx0 != 230 {
		t.Errorf("MFD 推回：tx %d 遮蓋左界 %d，要 460、230", e.ti.tx, e.cx0)
	}
	// MFD 3 倍：ey = (Y−1)·3 + 1，baseline + 19。
	e, _, _ = flightTextOverlay(t, fonts, 3).planText(0, flightFixture(KindMFD, "Comm Mode:", []int{20}, 70, 60, map[int]uint8{30: 5}))
	if e.ti == nil || e.ti.baseline != 59*3+1+19 {
		t.Errorf("MFD 3 倍 baseline %v", e.ti)
	}
	// 橫幅：英文範圍與可用範圍都是英文墨跡 [ix0, ix1 + 1)；band [y−2, y+7)，2 倍 ey = (y−2)·2 + 1。
	pt := flightFixture(KindPlaque, "time burst", nil, 0, 80, map[int]uint8{100: 5, 140: 5})
	pt.X0 = 100
	e, miss, skip = fo2.planText(0, pt)
	if miss != nil || skip != nil || e.ti.tx != floorDiv((100+141)*2-32, 2) || e.ti.baseline != 78*2+1+14 || e.ti.clip.X0 != 200 || e.ti.clip.X1 != 282 {
		t.Errorf("橫幅：%+v %+v %+v", e.ti, miss, skip)
	}
}

// ---- 第 7 項：遮蓋與 E′（飛行） ----

func TestFlightTextCoverAndColor(t *testing.T) {
	fonts := testTextFonts(t)
	fo := flightTextOverlay(t, fonts, 3)
	// GEAR 範圍 [183, 214)；英文 [184, 200)、「短」W 15：置中 tx = ⌊(384·3 − 15) / 2⌋ = 568 > Xs[k]·3 = 552，
	// 遮蓋仍從 Xs[k] 起（§3.5：min(E′0, ⌊T0 / s⌋)）。
	e, miss, skip := fo.planText(0, flightFixture(KindHUD, "GEAR", []int{184}, 200, 50, map[int]uint8{190: 223}))
	if miss != nil || skip != nil {
		t.Fatalf("%+v %+v", miss, skip)
	}
	if e.ti.tx != 568 || e.cx0 != 184 || e.cx1 != 200 {
		t.Fatalf("tx %d 遮蓋 [%d, %d)，要 568、[184, 200)", e.ti.tx, e.cx0, e.cx1)
	}
	// 英文範圍窄於中文：遮蓋往右延到 ⌈T1 / s⌉；E′ 外、遮蓋內的非墨跡色像素不影響字色。
	// SRM：英文 [118, 122)，「短程飛彈」推回到 355，遮蓋 [118, ⌈416 / 3⌉ = 139)。E′ 內 118 是 223，E′ 外 130..132 是 9。
	ft := flightFixture(KindHUD, "SRM", []int{118}, 122, 50, map[int]uint8{118: 223, 130: 9, 131: 9, 132: 9})
	e, _, skip = fo.planText(0, ft)
	if skip != nil {
		t.Fatalf("skip %+v", skip)
	}
	if e.cx0 != 118 || e.cx1 <= 122 || e.fg != 223 {
		t.Fatalf("遮蓋 [%d, %d) 字色 %d，要從 118 起、右緣 > 122、字色 223（E′ 內）", e.cx0, e.cx1, e.fg)
	}
	// E′ 內沒有墨跡（只有遮蓋內有）：Skip。
	ft.After[2*320+118] = 0
	if _, _, skip = fo.planText(0, ft); skip == nil || !strings.Contains(skip.Reason, "沒有墨跡") {
		t.Fatalf("E′ 無墨跡應 Skip，得 %+v", skip)
	}
}

// ---- 第 8 項：圖面（§3.6.3） ----

// artGlyph 回 px 16 時墨跡在 em 列 [top, bot) 的字，其他字級用預設字形。
func artGlyph(top, bot int) testGlyphFunc {
	return func(px, asc int) *xlate.AlphaGlyph {
		if px != 16 {
			return boxGlyph(px, 1, -asc+1, px-2, px-2)
		}
		return boxGlyph(px, 1, -asc+top, px-2, bot-top)
	}
}

func TestArtTextGeom(t *testing.T) {
	fonts := testTextFontsWith(t, map[rune]testGlyphFunc{
		'是': artGlyph(0, 17), // 重現 2 倍 k17「是」的墨跡列 em 0..16
		'取': artGlyph(1, 16), '消': artGlyph(1, 16),
		'高': artGlyph(0, 22), // 墨跡高 22：主選單字帶 11 列 × 2 放得下 dy 0，放不下 ±1
	})
	f := fonts.Font(16)
	geom := func(file ArtFile, k int, passes []glyphPass, s string) (int, int, error) {
		ev := ArtEvent{File: file, K: k}
		sp, ok := artSpecFor(ev)
		if !ok {
			t.Fatalf("artSpecFor %v k%d", file, k)
		}
		return artTextGeom(ev, sp, passes, f, []rune(s), 2)
	}
	plain := []glyphPass{{15, -1, 0}, {20, 0, 0}}
	// k17／k18「是」2 倍：一般 0（第 5 步保底）、按下 1。
	_, n, _ := geom(ArtSaveLoad, 17, plain, "是")
	_, p, _ := geom(ArtSaveLoad, 18, plain, "是")
	if n != 0 || p != 1 {
		t.Errorf("k17 一般 %d、按下 %d，要 0、1", n, p)
	}
	// k6「取消」2 倍不觸發保底：一般 1、按下 2（按下被 hi 夾住）。
	_, n, _ = geom(ArtSaveLoad, 6, plain, "取消")
	_, p, _ = geom(ArtSaveLoad, 7, plain, "取消")
	if n != 1 || p != 2 {
		t.Errorf("k6 一般 %d、按下 %d，要 1、2", n, p)
	}
	// 主選單（k0 一般、k5 按下）與 TOMENU（k0、k2）按下態 ey 等於一般態。
	mm := func(k int) []glyphPass { return menuPasses(ArtEvent{File: ArtMainMenuButtons, K: k}) }
	_, n, _ = geom(ArtMainMenuButtons, 0, mm(0), "測試")
	_, p, _ = geom(ArtMainMenuButtons, 5, mm(5), "測試")
	if n != p {
		t.Errorf("主選單一般 %d、按下 %d，要相同", n, p)
	}
	tmp := menuPasses(ArtEvent{File: ArtTomenuButtons})
	_, n, _ = geom(ArtTomenuButtons, 0, tmp, "測試")
	_, p, _ = geom(ArtTomenuButtons, 2, tmp, "測試")
	if n != p {
		t.Errorf("TOMENU 一般 %d、按下 %d，要相同", n, p)
	}
	// 墨跡加 dy 超出字帶：Skip（主選單趟含 ±1）；同一字在只有 dy 0 的存讀檔 k1 放得下。
	if _, _, err := geom(ArtMainMenuButtons, 0, mm(0), "高"); err == nil {
		t.Error("主選單「高」應 Skip")
	}
	if _, _, err := geom(ArtSaveLoad, 1, []glyphPass{{20, 0, 0}}, "高"); err != nil {
		t.Errorf("存讀檔 k1「高」不應 Skip：%v", err)
	}
	// 分子為負時以 floorDiv：⌊(1·3 − 22) / 2⌋ = −10（向零取整是 −9）。
	if got := artEy(box{0, 0, 10, 0}, box{0, 0, 10, 10}, 3, 22, -12, 30); got != -10 {
		t.Errorf("負分子 ey %d，要 −10", got)
	}
	// 水平：置中框先減字帶起始欄；m = 最大 |dx|。
	tx, _, _ := geom(ArtSaveLoad, 2, plain, "測試")
	// k2 字帶 x 20..59、置中框 x 24..51：x0 4、x1 31，⌊((4 + 31 + 1)·2 − 32) / 2⌋ = 20。
	if tx != 20 {
		t.Errorf("k2 tx %d，要 20", tx)
	}
}

func TestArtTextPix(t *testing.T) {
	fonts := testTextFonts(t)
	f := fonts.Font(16)
	var pal [256][3]uint8
	pal[3], pal[20], pal[15] = [3]uint8{3, 3, 3}, [3]uint8{200, 200, 200}, [3]uint8{100, 100, 100}
	e := &artEntry{rect: ArtRect{Key: "k", W: 40, H: 9}, idx: bytes.Repeat([]byte{3}, 80*18),
		text: &artText{display: []rune("測"), font: f, px: 16, tx: 10, ey: 1, baseline: 1 + 14, passes: []glyphPass{{15, -1, 0}, {20, 0, 0}}}}
	s := e.stamp(pal, 2)
	n := 0
	for i := 0; i < len(s.Pix); i += 4 {
		if s.Pix[i+3] != 255 {
			t.Fatalf("Pix 第 %d 像素 alpha %d", i/4, s.Pix[i+3])
		}
		if s.Pix[i] == 200 {
			n++
		}
	}
	if n == 0 || e.text.ink.Empty() {
		t.Fatalf("字色趟沒畫：%d 點，ink %+v", n, e.text.ink)
	}
	// 高光趟在左 1 像素：(tx − 1 + 1, ...) 是高光色。
	if c := s.Pix[4*((1+3)*80+10)]; c != 100 && c != 200 {
		t.Fatalf("字形左緣 %d", c)
	}
}

// ---- 第 12 項：跨平台合成雜湊 ----

func TestTextCompositeHash(t *testing.T) {
	fonts := testTextFonts(t)
	var pal [256][3]uint8
	for i := range pal {
		pal[i] = [3]uint8{uint8(i), uint8(255 - i), uint8(i * 7)}
	}
	idx := make([]byte, 64000)
	for i := range idx {
		idx[i] = byte(i % 251)
	}
	out := ScaleIndexedRGBA(idx, pal, 2)
	mk := func(s string, px, tx, base int, outline bool, fg uint8) *textItem {
		f := fonts.Font(px)
		return &textItem{display: NarrowAlnum([]rune(s)), font: f, px: px, tx: tx, baseline: base,
			clip: xlate.AlphaRect{X0: 0, Y0: 0, X1: 640, Y1: 400}, outline: outline, outlineIdx: 0, fgIdx: fg}
	}
	drawTextItems(out, 2, pal, []*textItem{
		mk("與葛雯交談", 22, 263, 374, true, 15),
		mk("響尾蛇（ＡＩＭ－９Ｊ）", 16, 10, 30, false, 10),
		mk("回報狀態", 22, 400, 380, true, 223),
	})
	sum := sha256.Sum256(out)
	// 寫死的值：Linux（dosgolem-go-ebiten:1.24-r1）首次執行得出；Windows、macOS 在 M7 跨平台驗證跑同一測試。
	const want = "eaf93aee0b223facda3ecbbbdf2eb509d392c754de43b16738aa679140e9979d"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("合成雜湊 %s，要 %s", got, want)
	}
}

// ---- 第 15 項：CheckSubtitleRegion ----

func TestCheckSubtitleRegion(t *testing.T) {
	pre := make([]byte, 640*400*4)
	after := append([]byte(nil), pre...)
	ink := []xlate.AlphaRect{{X0: 100, Y0: 320, X1: 120, Y1: 340}}
	set := func(b []byte, x, y int) { b[4*(y*640+x)] = 9 }
	set(after, 105, 325) // 在 ink 內
	c := CheckSubtitleRegion(after, pre, 2, ink, true)
	if c.Diffs != 1 || c.Outside != 0 || c.InkPixels != 400 || !c.Visible {
		t.Fatalf("ink 內：%+v", c)
	}
	set(after, 200, 330) // 字幕區內、不在 ink 內
	if c = CheckSubtitleRegion(after, pre, 2, ink, true); c.Outside != 1 {
		t.Fatalf("字幕區內框外：%+v", c)
	}
	after2 := append([]byte(nil), pre...)
	set(after2, 105, 100) // 字幕區外
	if c = CheckSubtitleRegion(after2, pre, 2, []xlate.AlphaRect{{X0: 100, Y0: 90, X1: 120, Y1: 110}}, true); c.Outside != 1 || c.InkPixels != 0 {
		t.Fatalf("字幕區外：%+v", c)
	}
	if c = CheckSubtitleRegion(pre, pre, 2, ink, false); c.Diffs != 0 || c.Outside != 0 {
		t.Fatalf("相同：%+v", c)
	}
}

// ---- 第 9 項：字幕的繪製部分 ----

func TestSubtitleTextLayout(t *testing.T) {
	fonts := testTextFonts(t)
	// baseline 表（§3.6.7）：n 行第 0 行的 baseline。
	for _, c := range []struct{ s, n, want int }{{2, 1, 360}, {2, 2, 346}, {2, 3, 332}, {3, 1, 540}, {3, 2, 519}, {3, 3, 498}} {
		lines := make([][]rune, c.n)
		for i := range lines {
			lines[i] = []rune("測")
		}
		ls := subTextLayout(lines, fonts.Font(pxSub[c.s]), c.s)
		if ls[0].Baseline != c.want || (c.n > 1 && ls[1].Baseline-ls[0].Baseline != 14*c.s) {
			t.Errorf("%d 倍 %d 行：baseline %d（行距 %d），要 %d", c.s, c.n, ls[0].Baseline, ls[min(1, c.n-1)].Baseline-ls[0].Baseline, c.want)
		}
	}
	// 行尾半形空白不計寬、不畫、不影響置中。
	f := fonts.Font(22)
	a, _, _, _ := subTextLines([]rune("甲乙 "), f, 2)
	b, _, _, _ := subTextLines([]rune("甲乙"), f, 2)
	if len(a) != 1 || string(a[0].Text) != "甲乙" || a[0].TxOut != b[0].TxOut || a[0].TxOut != (640-44)/2 {
		t.Fatalf("行尾空白：%+v / %+v", a, b)
	}
	// 只有空白：0 行。
	if ls, _, _, empty := subTextLines([]rune("  "), f, 2); len(ls) != 0 || !empty {
		t.Fatalf("只有空白：%+v", ls)
	}
	// 4 行截成 3 行：第 3 行末 …，寬 ≤ 可用寬；第 3 行剛好放得下 … 時只附加，不刪字。
	long := []rune(strings.Repeat("測", 27*3+5))
	ls, _, cut, _ := subTextLines(long, f, 2)
	if !cut || len(ls) != 3 {
		t.Fatalf("截斷：cut %v %d 行", cut, len(ls))
	}
	w, _ := f.Advance(ls[2].Text)
	if w > 600 || !strings.HasSuffix(string(ls[2].Text), "…") || string(ls[2].Text) != strings.Repeat("測", 26)+"…" {
		t.Fatalf("第 3 行 %q 寬 %d", string(ls[2].Text), w)
	}
	short := [][]rune{[]rune("甲"), []rune("乙"), []rune("丙丁"), []rune("戊")}
	out, cut := truncateLines(short, f, 600)
	if !cut || string(out[2]) != "丙丁…" {
		t.Fatalf("附加截斷 %q", string(out[2]))
	}
	// 代換：$N 為 sam 得 sam（半形，不轉全形）；80h 以上位元組得 ？ 並記一筆。
	s := &Subtitles{Catalog: map[string]string{"speaker/billy": "比利"}}
	got, bad := s.substituteHalf("好$N，$C$P$W", subValues{N: []byte("sam"), C: []byte{'A', 0x81, ' '}, P: -32768, W: []byte("Billy")})
	if string(got) != "好sam，A？ -32768比利" || bad != 1 {
		t.Fatalf("代換 %q bad %d", string(got), bad)
	}
}
