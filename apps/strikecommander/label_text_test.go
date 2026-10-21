package strikecommander

import (
	"bytes"
	"slices"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// strike_commander 規格 011 §4.1 第 3 到 7、13、16 項的標籤部分（分期 (d)）。期望值在測試內獨立寫出，字型用 testTextFonts。

// textOverlay 建一個 label 走新路徑的 Overlay（舊字型照舊必要，計分板等仍走舊路徑）。
func textOverlay(t *testing.T, fonts *TextFonts, tr map[string]string, scale int) *Overlay {
	t.Helper()
	ov, err := NewOverlay(testLabelFont(t), tr, nil, scale)
	if err != nil {
		t.Fatal(err)
	}
	ov.Text, ov.TextLabel = fonts, true
	return ov
}

func filledFrame(c byte) []byte { return bytes.Repeat([]byte{c}, 64000) }

// ---- 第 3 項：水平 ----

func TestTextHorizontal(t *testing.T) {
	if got := floorDiv(-3, 2); got != -2 {
		t.Fatalf("⌊−3 / 2⌋ = %d，要 −2", got)
	}
	for _, c := range []struct {
		name            string
		e0, e1, s, w, m int
		want            int
		skip            bool
	}{
		{"置中", 114, 204, 2, 110, 1, 263, false},
		{"左推回", 0, 10, 2, 100, 1, 1, false},      // ⌊(20 − 100) / 2⌋ = −40 → lo 1
		{"右推回", 310, 320, 2, 100, 1, 539, false}, // 580 → hi − W = 639 − 100
		{"負分子向下取整", 0, 0, 2, 3, 0, 0, false},     // ⌊−3 / 2⌋ = −2 → lo 0
		{"Skip", 0, 320, 2, 639, 1, 0, true},     // 639 > 638
		{"剛好放得下", 0, 320, 2, 638, 1, 1, false},
	} {
		got, err := textHorizontal(c.e0, c.e1, 0, 320, c.s, c.w, c.m)
		if (err != nil) != c.skip || (!c.skip && got != c.want) {
			t.Errorf("%s：tx %d err %v，要 %d skip %v", c.name, got, err, c.want, c.skip)
		}
	}
	// 無推回時的負分子：⌊((0 + 0)·2 − 3) / 2⌋ = −2（可用範圍放寬到負數）。
	if got, _ := textHorizontal(0, 0, -10, 320, 2, 3, 0); got != -2 {
		t.Fatalf("負分子 tx %d，要 −2", got)
	}
}

// ---- 第 4 項：垂直 ----

func TestTextVertical(t *testing.T) {
	for _, c := range []struct{ b0, b1, s, px, ey, base int }{
		{176, 190, 2, 22, 355, 374}, // 標籤 2 倍（§3.6.1 樣本 A）
		{176, 190, 3, 32, 533, 561}, // 標籤 3 倍
		{101, 109, 2, 16, 202, 216}, // 計分板 2 倍：H 16、px 16
	} {
		ey, base, err := textVertical(c.b0, c.b1, c.s, c.px)
		if err != nil || ey != c.ey || base != c.base {
			t.Errorf("textVertical(%d,%d,%d,%d) = %d %d %v，要 %d %d", c.b0, c.b1, c.s, c.px, ey, base, err, c.ey, c.base)
		}
	}
	if _, _, err := textVertical(0, 5, 2, 16); err == nil {
		t.Fatal("px 16 > H 10 應 Skip")
	}
	// 合成圖集的「一」：墨跡只在 em 第 2、3 列。依 em 置中 baseline 374；依墨跡置中會是 382。
	fonts := testTextFontsWith(t, map[rune]testGlyphFunc{
		'一': func(px, asc int) *xlate.AlphaGlyph { return boxGlyph(px, 1, -asc+2, px-2, 2) },
	})
	p, err := PlanLabelText(Label{Key: "k", X0: 114, X1: 204, Y: 175}, []rune("一"), 2, fonts)
	if err != nil {
		t.Fatal(err)
	}
	inkCentered := (176*2+190*2)/2 - 3 + TextAscent(22) // 字帶中心 366 減墨跡中心 3，再加 asc
	if p.Text.baseline != 374 || p.Text.baseline == inkCentered {
		t.Fatalf("baseline %d，要依 em 置中的 374（依墨跡置中為 %d）", p.Text.baseline, inkCentered)
	}
}

// ---- 第 5 項：版面例（§3.6.1） ----

func TestLabelTextExample(t *testing.T) {
	fonts := testTextFonts(t)
	l := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Kind: KindLabel}
	for _, c := range []struct{ s, w, tx, base int }{{2, 110, 263, 374}, {3, 160, 397, 561}} {
		p, err := PlanLabelText(l, []rune("與葛雯交談"), c.s, fonts)
		if err != nil {
			t.Fatal(err)
		}
		w, _ := fonts.Font(pxLabel[c.s]).Advance(p.Text.display)
		cv := p.Cover
		if w != c.w || p.Text.tx != c.tx || p.Text.baseline != c.base || cv.X != 114 || cv.CellW != 90 || cv.Cells != 1 || cv.Y != 176 || cv.CellH != 14 {
			t.Errorf("%d 倍：W %d tx %d baseline %d 遮蓋 (%d,%d) %d×%d，要 W %d tx %d baseline %d 遮蓋 (114,176) 90×14",
				c.s, w, p.Text.tx, p.Text.baseline, cv.X, cv.Y, cv.CellW, cv.CellH, c.w, c.tx, c.base)
		}
		wantClip := xlate.AlphaRect{X0: 0, Y0: 175 * c.s, X1: 320 * c.s, Y1: 191 * c.s}
		if p.Text.clip != wantClip || p.Kind != KindLabel || p.BG != 0 || p.FG != 15 || !p.Text.outline {
			t.Errorf("%d 倍：clip %+v kind %s BG %d FG %d", c.s, p.Text.clip, p.Kind, p.BG, p.FG)
		}
	}
}

// ---- 第 6 項：繪製 ----

// pixelFont 回只有一字 'P' 的字型：填色與外框都是 (0, −1) 的 1×1，alpha 分別為 fill、stroke。
func pixelFont(fill, stroke byte) *xlate.AlphaFont {
	return &xlate.AlphaFont{Px: 4, Ascent: 3, Descent: 1, HasStroke: true, StrokeRadius: 1, Glyphs: map[rune]*xlate.AlphaGlyph{
		'P': {Adv: 1, FX: 0, FY: -1, FW: 1, FH: 1, Fill: []byte{fill}, SX: 0, SY: -1, SW: 1, SH: 1, Stroke: []byte{stroke}},
	}}
}

func TestDrawTextItemsOrder(t *testing.T) {
	var pal [256][3]uint8
	pal[0], pal[15] = [3]uint8{0, 0, 0}, [3]uint8{255, 255, 255}
	clip := xlate.AlphaRect{X0: 0, Y0: 0, X1: 640, Y1: 400}
	item := func(f *xlate.AlphaFont, x, base int) *textItem {
		return &textItem{display: []rune("P"), font: f, tx: x, baseline: base, clip: clip, outline: true, outlineIdx: 0, fgIdx: 15}
	}
	at := func(out []byte, x, y int) byte { return out[4*(y*640+x)] }
	// 白字 alpha 128、黑框 alpha 255：先框後字得 128；順序顛倒得 0。
	out := ScaleIndexedRGBA(filledFrame(7), pal, 2)
	drawTextItems(out, 2, pal, []*textItem{item(pixelFont(128, 255), 10, 11)})
	if v := at(out, 10, 10); v != 128 {
		t.Fatalf("外框層先、填色層後應得 128，得 %d", v)
	}
	// 兩串上下相接：第二串的外框壓在第一串的填色位置，先畫全部外框層，第一串的填色仍在。
	out = ScaleIndexedRGBA(filledFrame(7), pal, 2)
	f1 := pixelFont(255, 0)
	f2 := &xlate.AlphaFont{Px: 4, Ascent: 3, Descent: 1, HasStroke: true, StrokeRadius: 1, Glyphs: map[rune]*xlate.AlphaGlyph{
		'P': {Adv: 1, FX: 0, FY: 0, FW: 1, FH: 1, Fill: []byte{255}, SX: 0, SY: -2, SW: 1, SH: 3, Stroke: []byte{255, 255, 255}},
	}}
	ink := drawTextItems(out, 2, pal, []*textItem{item(f1, 20, 21), item(f2, 20, 21)})
	if v := at(out, 20, 20); v != 255 {
		t.Fatalf("第一串填色被第二串外框蓋掉：%d", v)
	}
	if ink[0] != (xlate.AlphaRect{X0: 20, Y0: 20, X1: 21, Y1: 21}) || ink[1] != (xlate.AlphaRect{X0: 20, Y0: 19, X1: 21, Y1: 22}) {
		t.Fatalf("文字框 %+v", ink)
	}
}

func TestLabelTextDraw(t *testing.T) {
	// 「高」：墨跡在輸出列 198..201（字帶 [202, 230) 上方 2 個原版列），沒有外框層。2 倍 baseline = 202 + 3 + 19 = 224。
	fonts := testTextFontsWith(t, map[rune]testGlyphFunc{
		'高': func(px, asc int) *xlate.AlphaGlyph {
			return &xlate.AlphaGlyph{Adv: px, FX: 0, FY: 198 - 224, FW: px, FH: 4, Fill: bytes.Repeat([]byte{255}, px*4)}
		},
	})
	var pal [256][3]uint8
	pal[0], pal[7], pal[15] = [3]uint8{1, 2, 3}, [3]uint8{70, 70, 70}, [3]uint8{250, 251, 252}
	idx := filledFrame(7)
	l := Label{Key: "Talk to Gwen", X0: 100, X1: 140, Y: 100, Font: measuredFont, Kind: KindLabel}
	l.Ref = refFromFrame(idx, l)
	ov := textOverlay(t, fonts, map[string]string{l.Key: "高"}, 2)
	ov.Sync([]Label{l}, 1)
	if len(ov.Stamps()) != 1 || len(ov.Misses)+len(ov.Skips) != 0 {
		t.Fatalf("stamps %d misses %+v skips %+v", len(ov.Stamps()), ov.Misses, ov.Skips)
	}
	out, _ := ov.Draw(idx, pal)
	at := func(x, y int) [3]uint8 { i := 4 * (y*640 + x); return [3]uint8{out[i], out[i+1], out[i+2]} }
	tx := ov.matches[0].textItem.tx
	// 延伸列（字帶上方 1 原版列 = 輸出列 200、201）有寫入；2 列外（198、199）在裁切框外，不寫。
	if at(tx, 200) != pal[15] || at(tx, 201) != pal[15] {
		t.Errorf("延伸列沒寫入：%v %v", at(tx, 200), at(tx, 201))
	}
	if at(tx, 199) != pal[7] || at(tx, 198) != pal[7] {
		t.Errorf("裁切框外被寫入：%v %v", at(tx, 199), at(tx, 198))
	}
	// 文字框等於兩層 ink 的聯集（這裡只有填色層，裁到 clip）。
	boxes := ov.TextBoxes()
	want := xlate.AlphaRect{X0: tx, Y0: 200, X1: tx + 22, Y1: 202}
	if len(boxes) != 1 || boxes[0] != want {
		t.Errorf("TextBoxes %+v，要 [%+v]", boxes, want)
	}
	if bg := ov.BGReport(); len(bg) != 1 || bg[0].Path != "new" || bg[0].Ink != want || bg[0].Display != "高" || bg[0].Px != 22 {
		t.Errorf("BGReport %+v", bg)
	}

	// 一般字：外框層與填色層 ink 的聯集，獨立以 StringMask 重算。
	fonts = testTextFonts(t)
	ov = textOverlay(t, fonts, map[string]string{l.Key: "測試"}, 2)
	ov.Sync([]Label{l}, 1)
	ov.Draw(idx, pal)
	ti := ov.matches[0].textItem
	f := fonts.Font(22)
	_, fb, _, _ := xlate.StringMask(f, []rune("測試"), ti.tx, ti.baseline, xlate.AlphaFill, ti.clip)
	_, sb, _, _ := xlate.StringMask(f, []rune("測試"), ti.tx, ti.baseline, xlate.AlphaStroke, ti.clip)
	if b := ov.TextBoxes(); len(b) != 1 || b[0] != unionRect(fb, sb) || b[0] == fb {
		t.Errorf("TextBoxes %+v，要外框層 %+v 與填色層 %+v 的聯集", b, sb, fb)
	}

	// 缺字：整串不畫，Runes 依序列出全部缺字。
	ov = textOverlay(t, fonts, map[string]string{l.Key: "測鑫試鑪"}, 2)
	ov.Sync([]Label{l}, 1)
	if len(ov.Stamps()) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Reason != MissGlyph || string(ov.Misses[0].Runes) != "鑫鑪" {
		t.Fatalf("缺字：stamps %d misses %+v", len(ov.Stamps()), ov.Misses)
	}
	out, _ = ov.Draw(idx, pal)
	if !bytes.Equal(out, ScaleIndexedRGBA(idx, pal, 2)) || len(ov.TextBoxes()) != 0 {
		t.Fatal("缺字時畫面應維持原版")
	}

	// 繪製路徑的半形化：testTextFonts 只收半形 A、不收全形 Ａ；ＡＩＭ－９Ｊ 照畫，像素等於直接畫 AIM-9J。
	ovFull := textOverlay(t, fonts, map[string]string{l.Key: "ＡＩＭ－９Ｊ"}, 2)
	ovFull.Sync([]Label{l}, 1)
	ovHalf := textOverlay(t, fonts, map[string]string{l.Key: "AIM-9J"}, 2)
	ovHalf.Sync([]Label{l}, 1)
	if len(ovFull.Misses) != 0 || len(ovFull.Stamps()) != 1 {
		t.Fatalf("全形英數：misses %+v stamps %d", ovFull.Misses, len(ovFull.Stamps()))
	}
	a, _ := ovFull.Draw(idx, pal)
	b, _ := ovHalf.Draw(idx, pal)
	if !bytes.Equal(a, b) || bytes.Equal(a, ScaleIndexedRGBA(idx, pal, 2)) {
		t.Fatal("ＡＩＭ－９Ｊ 畫出的像素不等於 AIM-9J")
	}
}

// ---- 第 7 項：遮蓋右緣 ----

func TestLabelTextCoverRight(t *testing.T) {
	// 文字框右緣落在原版像素中間：2 倍 T1 = 201，遮蓋欄右界 ⌈201 / 2⌉ = 101，不是 100。
	fonts := testTextFontsWith(t, map[rune]testGlyphFunc{
		'高': func(px, asc int) *xlate.AlphaGlyph {
			if px != 22 {
				return boxGlyph(px, 1, -asc+1, px-2, px-2)
			}
			return &xlate.AlphaGlyph{Adv: 100, FX: 0, FY: -asc + 1, FW: 101, FH: 4, Fill: bytes.Repeat([]byte{255}, 101*4)}
		},
	})
	l := Label{Key: "k", X0: 50, X1: 100, Y: 100, Kind: KindLabel}
	p, err := PlanLabelText(l, []rune("高"), 2, fonts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Text.tx != 100 || p.Text.box.X1 != 201 || p.Cover.X != 50 || p.Cover.X+p.Cover.CellW != 101 {
		t.Fatalf("tx %d 文字框 %+v 遮蓋 [%d, %d)，要 tx 100、T1 201、遮蓋 [50, 101)", p.Text.tx, p.Text.box, p.Cover.X, p.Cover.X+p.Cover.CellW)
	}
	// 文字框左緣在 E0 左邊時遮蓋往左延：⌊T0 / s⌋。
	l2 := Label{Key: "k", X0: 60, X1: 61, Y: 100, Kind: KindLabel}
	p, _ = PlanLabelText(l2, []rune("測試"), 2, testTextFonts(t))
	if p.Cover.X != floorDiv(p.Text.box.X0, 2) || p.Cover.X+p.Cover.CellW != ceilDiv(p.Text.box.X1, 2) {
		t.Fatalf("遮蓋 [%d, %d)，文字框 %+v", p.Cover.X, p.Cover.X+p.Cover.CellW, p.Text.box)
	}
}

// ---- 第 13 項：CheckCompositeText ----

func TestCheckCompositeText(t *testing.T) {
	var pal [256][3]uint8
	pal[7] = [3]uint8{70, 70, 70}
	plain := ScaleIndexedRGBA(filledFrame(7), pal, 2)
	comp := slices.Clone(plain)
	cover := &xlate.Stamp{X: 100, Y: 101, Cells: 1, CellW: 40, CellH: 14}
	// 差異在延伸列：輸出 (210, 200)，原版列 100，在遮蓋列 [101, 115) 之外。
	i := 4 * (200*640 + 210)
	comp[i] = 1
	box := xlate.AlphaRect{X0: 205, Y0: 200, X1: 230, Y1: 230}
	if c := CheckCompositeText(comp, plain, 2, []*xlate.Stamp{cover}, nil, []xlate.AlphaRect{box}); c.Diffs != 1 || c.Outside != 0 {
		t.Fatalf("文字框內的延伸列差異應允許：%+v", c)
	}
	// 同一位置若屬 Hidden 的串（文字框不列入）算框外。
	if c := CheckCompositeText(comp, plain, 2, []*xlate.Stamp{cover}, nil, nil); c.Outside != 1 {
		t.Fatalf("沒有文字框時應算框外：%+v", c)
	}
	if c := CheckComposite(comp, plain, 2, []*xlate.Stamp{cover}, nil); c.Outside != 1 {
		t.Fatalf("CheckComposite 行為改變：%+v", c)
	}
	// Overlay：比對不符（Hidden）的串不列入 TextBoxes。
	fonts := testTextFonts(t)
	idx := filledFrame(7)
	l := Label{Key: "Talk to Gwen", X0: 100, X1: 140, Y: 100, Font: measuredFont, Kind: KindLabel}
	l.Ref = refFromFrame(idx, l)
	ov := textOverlay(t, fonts, map[string]string{l.Key: "測試"}, 2)
	ov.Sync([]Label{l}, 1)
	other := filledFrame(7)
	other[105*320+110] = 9 // 比對框內改一點
	out, _ := ov.Draw(other, pal)
	if len(ov.Hidden) != 1 || len(ov.TextBoxes()) != 0 || !bytes.Equal(out, ScaleIndexedRGBA(other, pal, 2)) {
		t.Fatalf("Hidden %q TextBoxes %+v", ov.Hidden, ov.TextBoxes())
	}
}

// ---- 第 16 項：commitStamps ----

func TestCommitStampsOldNew(t *testing.T) {
	labels := []Label{
		{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont, Kind: KindLabel},
		{Key: "Fly Mission", X0: 124, X1: 195, Y: 160, Font: measuredFont, Kind: KindLabel},
	}
	tr := map[string]string{"Talk to Gwen": "測試譯文甲", "Fly Mission": "尾（ＡＩＭ－９Ｊ）"}
	old, err := NewOverlay(testLabelFont(t), tr, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	old.Sync(labels, 1)
	nw := textOverlay(t, testTextFonts(t), tr, 2)
	nw.Sync(labels, 1)
	a, b := old.commitStamps(2), nw.commitStamps(2)
	want := CommitStamps{Scale: 2, Stamps: 4, Texts: []string{"測試譯文甲", "尾（ＡＩＭ－９Ｊ）"}, Kinds: []Kind{KindLabel, KindLabel}}
	eq := func(x, y CommitStamps) bool {
		return x.Scale == y.Scale && x.Stamps == y.Stamps && slices.Equal(x.Texts, y.Texts) && slices.Equal(x.Kinds, y.Kinds)
	}
	if !eq(a, want) || !eq(b, want) {
		t.Fatalf("舊 %+v、新 %+v，要 %+v", a, b, want)
	}
	if len(old.Stamps()) != 4 || len(nw.Stamps()) != 2 {
		t.Fatalf("Stamps()：舊 %d、新 %d，要 4、2（新路徑只回遮蓋 stamp）", len(old.Stamps()), len(nw.Stamps()))
	}
}

// TestTextPathsGuard：(d) 只有 Label 可開；Fonts 為 nil 而開關為真時回錯。
func TestTextPathsGuard(t *testing.T) {
	if _, err := RunReceipt(ReceiptConfig{TextPaths: TextPaths{Label: true}}); err == nil {
		t.Fatal("Fonts 為 nil 應回錯")
	}
	if _, err := RunReceipt(ReceiptConfig{Fonts: testTextFonts(t), TextPaths: TextPaths{Board: true}}); err == nil {
		t.Fatal("(d) 開 Board 應回錯")
	}
	p, err := ParseTextPaths("label, subtitle")
	if err != nil || !p.Label || !p.Subtitle || p.Board {
		t.Fatalf("ParseTextPaths %+v %v", p, err)
	}
	if _, err := ParseTextPaths("labels"); err == nil {
		t.Fatal("不認得的路徑應回錯")
	}
}

// TestBGTSVNewPath：labels.tsv 新路徑的列舊幾何欄填 -、新欄照填；舊路徑的列新欄填 -（011 §3.8）。
func TestBGTSVNewPath(t *testing.T) {
	es := []BGEntry{
		{Key: "a", Kind: KindLabel, State: BGStateFallback, Text: "測", CoverW: 10, CoverH: 14, TextX: 3, Cells: 1},
		{Key: "b", Kind: KindLabel, State: BGStateFallback, Text: "ＡＢ", Path: "new", Display: "AB", Px: 22, TxOut: 263, Baseline: 374,
			Clip: xlate.AlphaRect{X0: 0, Y0: 350, X1: 640, Y1: 382}, Ink: xlate.AlphaRect{X0: 1, Y0: 2, X1: 3, Y1: 4}, Outline: true, E0: 114, E1: 204},
	}
	tsv, _ := BGTSV(es)
	lines := bytes.Split(bytes.TrimSuffix(tsv, []byte("\n")), []byte("\n"))
	want := []string{
		"key\tkind\tstate\ttext\tcover_x\tcover_y\tcover_w\tcover_h\ttext_x\ttext_y\tcells\tcell_w\tcell_h\tglyph_x\tglyph_y\thidden\tbefore_off\tbefore_len\tafter_off\tafter_len\tref_off\tref_len\tdisplay\tpx\ttx_out\tbaseline\tclip_x0\tclip_y0\tclip_x1\tclip_y1\tink_x0\tink_y0\tink_x1\tink_y1\toutline\te0\te1\tpath",
		"a\tlabel\t退路\t測\t0\t0\t10\t14\t3\t0\t1\t0\t0\t0\t0\tfalse\t0\t0\t0\t0\t0\t0\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\told",
		"b\tlabel\t退路\tＡＢ\t0\t0\t0\t0\t-\t-\t-\t-\t-\t-\t-\tfalse\t0\t0\t0\t0\t0\t0\tAB\t22\t263\t374\t0\t350\t640\t382\t1\t2\t3\t4\ttrue\t114\t204\tnew",
	}
	if len(lines) != 3 {
		t.Fatalf("%d 列", len(lines))
	}
	for i, w := range want {
		if string(lines[i]) != w {
			t.Errorf("第 %d 列\n得 %q\n要 %q", i+1, lines[i], w)
		}
	}
}
