package strikecommander

import (
	"encoding/binary"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// testFontRunes：全形字（來源位元組 bit 7 = 1）；testHalfRunes：半形字。
// 數字刻意缺 ７（002 §3.1：缺字時 "… (17)" 記 glyph miss、CheckFontExtra 失敗）。
const (
	testFontRunes = "測試譯文甲乙丙一二三ＡＩＭ－９Ｊ尾追式飛彈卸下０１２３４５６８（）"
	testHalfRunes = "a -"
)

// buildTestFont 合成一份 16×16 GOLEMFNT。
func buildTestFont(wide, half string) []byte {
	var rs []rune
	src := map[rune]byte{}
	for _, r := range wide {
		rs = append(rs, r)
		src[r] = 0x81
	}
	for _, r := range half {
		rs = append(rs, r)
		src[r] = 0x01
	}
	b := []byte("GOLEMFNT")
	b = binary.LittleEndian.AppendUint16(b, 16)
	b = binary.LittleEndian.AppendUint16(b, 16)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(rs)))
	for _, r := range rs {
		b = binary.LittleEndian.AppendUint32(b, uint32(r))
		b = append(b, src[r])
		for i := 0; i < 32; i++ {
			b = append(b, 0x81) // 每列左右兩緣各一點
		}
	}
	return b
}

func testLabelFont(t *testing.T) *LabelFont {
	t.Helper()
	f, err := ParseLabelFont(buildTestFont(testFontRunes, testHalfRunes), "full16-test")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// testLABL 回測試用的 LABL 集合：九個武器長名稱（002 §2.3 防呆要求它們在 LABL 裡）加上 extra。
func testLABL(extra ...string) map[string]bool {
	m := map[string]bool{}
	for _, k := range WeaponLongNames {
		m[k] = true
	}
	for _, k := range extra {
		m[k] = true
	}
	return m
}

func testCatalog() map[string]string {
	return map[string]string{
		"Talk to Gwen":             "測試譯文甲",
		"Transport to Strike Base": "測試譯文乙乙乙",
		"Fly Mission":              "測試丙",
	}
}

func TestWideTable(t *testing.T) {
	f := testLabelFont(t)
	if !f.Wide.Wide('測') || f.Wide.Wide('a') || !f.Wide.Has('a') || f.Wide.Has('乂') {
		t.Fatal("全形表解析錯誤")
	}
	if f.Wide.Len() != len([]rune(testFontRunes+testHalfRunes)) {
		t.Fatalf("字數 %d", f.Wide.Len())
	}
	if _, err := ParseWide([]byte("GOLEMFNT\x10\x00\x10\x00\x02\x00\x00\x00")); err == nil {
		t.Fatal("宣告字數不符應回錯")
	}
}

func TestTestdataCatalog(t *testing.T) {
	b, err := os.ReadFile("testdata/opttest_labels.test.tsv")
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseCatalog(b)
	if err != nil {
		t.Fatal(err)
	}
	want := testCatalog()
	if len(c) != len(want) {
		t.Fatalf("testdata catalog %d 列", len(c))
	}
	for k, v := range want {
		if c[k] != v {
			t.Fatalf("testdata %q = %q，要 %q", k, c[k], v)
		}
	}
	labl := testLABL("Talk to Gwen", "Transport to Strike Base", "Fly Mission", "Talk to Janet")
	rep := Lint(b, testLabelFont(t).Wide, labl)
	// 漏譯：Talk to Janet、九個長名稱、兩個範本、九個短代號（002 §2.3 的 key 集合）。
	wantU := []string{"Talk to Janet", TemplateCount, TemplateRemove}
	wantU = append(wantU, WeaponLongNames[:]...)
	wantU = append(wantU, WeaponShortNames[:]...)
	sort.Strings(wantU)
	if !rep.OK() || strings.Join(rep.Untranslated, "|") != strings.Join(wantU, "|") {
		t.Fatalf("testdata lint：%+v", rep)
	}
}

// --- 第 5 項：stamp ---

func syncOne(t *testing.T, l Label, tr string, scale int) *Overlay {
	t.Helper()
	ov, err := NewOverlay(testLabelFont(t), map[string]string{l.Key: tr}, scale)
	if err != nil {
		t.Fatal(err)
	}
	ov.Sync([]Label{l}, 1)
	return ov
}

func TestStampExamples(t *testing.T) {
	cases := []struct {
		name          string
		x0, x1        int
		n, scale      int
		x, w          int
		cx0, cx1      int
		cellW, gx, gy int
	}{
		{"樣本 A 5 字 2 倍", 114, 204, 5, 2, 139, 40, 114, 204, 8, 0, 6},
		{"樣本 A 5 字 3 倍", 114, 204, 5, 3, 144, 30, 114, 204, 6, 1, 13},
		{"樣本 A 14 字 2 倍", 114, 204, 14, 2, 103, 112, 103, 215, 8, 0, 6},
		{"左緣 10 字 2 倍", 0, 40, 10, 2, 0, 80, 0, 80, 8, 0, 6},
		{"右緣 10 字 2 倍", 300, 320, 10, 2, 240, 80, 240, 320, 8, 0, 6},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := Label{Key: "K", X0: c.x0, X1: c.x1, Y: 175, Font: measuredFont}
			ov := syncOne(t, l, strings.Repeat("測", c.n), c.scale)
			st := ov.Stamps()
			if len(st) != 2 {
				t.Fatalf("stamp %d 個，要 2（遮蓋、文字）", len(st))
			}
			cv, tx := st[0], st[1]
			if cv.Key != "K.cover" || cv.Font != nil || len(cv.Text) != 0 || cv.Cells != 1 || cv.CellH != 14 || cv.Y != 176 ||
				cv.X != c.cx0 || cv.CellW != c.cx1-c.cx0 || cv.State != xlate.Shown {
				t.Errorf("遮蓋 stamp %+v，要 [%d, %d) × [176, 190)", *cv, c.cx0, c.cx1)
			}
			if tx.Key != "K.text" || tx.Font == nil || len(tx.Text) != c.n || tx.Cells != c.n || tx.CellW != c.cellW || tx.CellH != 14 ||
				tx.GlyphScale != 1 || tx.GlyphX != c.gx || tx.GlyphY != c.gy || tx.X != c.x || tx.Cells*tx.CellW != c.w || tx.Y != 176 ||
				tx.State != xlate.Shown {
				t.Errorf("文字 stamp X %d W %d CellW %d Glyph (%d,%d) scale %d Y %d，要 X %d W %d CellW %d Glyph (%d,%d)",
					tx.X, tx.Cells*tx.CellW, tx.CellW, tx.GlyphX, tx.GlyphY, tx.GlyphScale, tx.Y, c.x, c.w, c.cellW, c.gx, c.gy)
			}
		})
	}
}

func TestStampOrderMultiple(t *testing.T) {
	ov, err := NewOverlay(testLabelFont(t), testCatalog(), 2)
	if err != nil {
		t.Fatal(err)
	}
	ov.Sync([]Label{
		{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont},
		{Key: "Fly Mission", X0: 124, X1: 195, Y: 160, Font: measuredFont},
	}, 1)
	var ks []string
	for _, s := range ov.Stamps() {
		ks = append(ks, s.Key)
	}
	if got := strings.Join(ks, "|"); got != "Talk to Gwen.cover|Talk to Gwen.text|Fly Mission.cover|Fly Mission.text" {
		t.Fatalf("順序 %s", got)
	}
}

func TestStampNotProduced(t *testing.T) {
	l := Label{Key: "K", X0: 114, X1: 204, Y: 175, Font: measuredFont}
	t.Run("超寬", func(t *testing.T) {
		ov := syncOne(t, l, strings.Repeat("測", 41), 2)
		if len(ov.Stamps()) != 0 || len(ov.Skips) != 1 {
			t.Fatalf("W 328 > 320 仍產生 stamp：%d", len(ov.Stamps()))
		}
	})
	t.Run("1 倍", func(t *testing.T) {
		ov := syncOne(t, l, "測試", 1)
		if len(ov.Stamps()) != 0 || len(ov.Skips) != 1 {
			t.Fatalf("1 倍仍產生 stamp：%d", len(ov.Stamps()))
		}
	})
	t.Run("缺譯文", func(t *testing.T) {
		ov, _ := NewOverlay(testLabelFont(t), map[string]string{}, 2)
		ov.Sync([]Label{l}, 1)
		if len(ov.Stamps()) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Reason != "translation" {
			t.Fatalf("缺譯文：stamps %d misses %+v", len(ov.Stamps()), ov.Misses)
		}
	})
	for _, tr := range []string{"測試乂", "測a試"} {
		t.Run("缺字 "+tr, func(t *testing.T) {
			ov := syncOne(t, l, tr, 2)
			if len(ov.Stamps()) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Reason != "glyph" || len(ov.Misses[0].Runes) != 1 {
				t.Fatalf("缺字：stamps %d misses %+v", len(ov.Stamps()), ov.Misses)
			}
		})
	}
}

func TestStampGenerationAndScale(t *testing.T) {
	l := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont}
	ov, _ := NewOverlay(testLabelFont(t), testCatalog(), 2)
	ov.Sync([]Label{l}, 1)
	first := ov.Stamps()[1]
	ov.Sync(nil, 1) // generation 相同：不重建
	if len(ov.Stamps()) != 2 || ov.Stamps()[1] != first {
		t.Fatal("generation 相同時重建了")
	}
	ov.Sync(nil, 2)
	if len(ov.Stamps()) != 0 {
		t.Fatal("generation 改變後未清空")
	}
	ov.Sync([]Label{l}, 3)
	if err := ov.SetScale(3); err != nil {
		t.Fatal(err)
	}
	ov.Sync([]Label{l}, 3)
	if ov.Stamps()[1].CellW != 6 {
		t.Fatal("換倍率後未重建")
	}
}

func TestStampDrawColors(t *testing.T) {
	l := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont}
	ov := syncOne(t, l, "測試譯文甲", 2)
	idx := make([]byte, 64000)
	for i := range idx {
		idx[i] = 7
	}
	var pal [256][3]uint8
	pal[0], pal[7], pal[15] = [3]uint8{1, 2, 3}, [3]uint8{70, 70, 70}, [3]uint8{250, 251, 252}
	out, miss := ov.Draw(idx, pal)
	if len(miss) != 0 {
		t.Fatalf("缺字 %q", miss)
	}
	at := func(x, y int) [3]uint8 { i := 4 * (y*640 + x); return [3]uint8{out[i], out[i+1], out[i+2]} }
	if c := at(2*114, 2*176); c != pal[0] {
		t.Errorf("遮蓋左上 %v，要 0 號色 %v", c, pal[0])
	}
	if c := at(2*114-1, 2*176); c != pal[7] {
		t.Errorf("遮蓋框外 %v，要原畫格 %v", c, pal[7])
	}
	// 文字 stamp 第一格 X 139，GlyphY 6：字模第 0 列最左點（0x81 的 bit 7）在 (278, 352+6)
	if c := at(2*139, 2*176+6); c != pal[15] {
		t.Errorf("字模點 %v，要 15 號色 %v", c, pal[15])
	}
	// 色盤改變後重畫，跟著改
	pal[15] = [3]uint8{9, 9, 9}
	out, _ = ov.Draw(idx, pal)
	if c := at(2*139, 2*176+6); c != pal[15] {
		t.Errorf("色盤改變後字模點 %v，要 %v", c, pal[15])
	}
	cc := CheckComposite(out, ScaleIndexedRGBA(idx, pal, 2), 2, ov.Stamps())
	if cc.Diffs == 0 || cc.Outside != 0 {
		t.Errorf("合成圖比較 %+v", cc)
	}
}
