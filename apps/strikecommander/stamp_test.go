package strikecommander

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

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
	rep := LintLabels(b, testTextFonts(t), labl)
	// 漏譯：Talk to Janet、九個長名稱、兩個範本、九個短代號（002 §2.3 的 key 集合）、四個 EXE 字串（003 §3.5）。
	wantU := []string{"Talk to Janet", TemplateCount, TemplateRemove}
	wantU = append(wantU, WeaponLongNames[:]...)
	wantU = append(wantU, WeaponShortNames[:]...)
	wantU = append(wantU, ExeKeys[:]...)
	sort.Strings(wantU)
	if !rep.OK() || strings.Join(rep.Untranslated, "|") != strings.Join(wantU, "|") {
		t.Fatalf("testdata lint：%+v", rep)
	}
}

// --- 第 5 項：stamp ---

func syncOne(t *testing.T, l Label, tr string, scale int) *Overlay {
	t.Helper()
	ov := textOverlay(t, testTextFonts(t), map[string]string{l.Key: tr}, scale)
	ov.Sync([]Label{l}, 1)
	return ov
}

// TestStampExamples：001 §2.6 的舊幾何（legacyLabelGeom）照數值例；Overlay 只產生遮蓋 stamp，
// 遮蓋範圍包住英文游標（011 §3.5），文字以圖集畫（版面值見 TestLabelTextExample）。
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
			g, err := legacyLabelGeom(l, c.n, c.scale)
			if err != nil || g.X != c.x || g.Y != 176 || g.CellW != c.cellW || g.CellH != 14 || g.GlyphX != c.gx || g.GlyphY != c.gy ||
				c.n*g.CellW != c.w || g.CX0 != c.cx0 || g.CX1 != c.cx1 {
				t.Errorf("舊幾何 %+v（%v），要 X %d W %d CellW %d Glyph (%d,%d) 遮蓋 [%d, %d)", g, err, c.x, c.w, c.cellW, c.gx, c.gy, c.cx0, c.cx1)
			}
			ov := syncOne(t, l, strings.Repeat("測", c.n), c.scale)
			st := ov.Stamps()
			if len(st) != 1 {
				t.Fatalf("stamp %d 個，要 1（遮蓋）", len(st))
			}
			cv := st[0]
			if cv.Key != "K.cover" || cv.Font != nil || len(cv.Text) != 0 || cv.Cells != 1 || cv.CellH != 14 || cv.Y != 176 ||
				cv.X > max(c.x0, 0) || cv.X+cv.CellW < min(c.x1, 320) || cv.State != xlate.Shown {
				t.Errorf("遮蓋 stamp %+v，要包住 [%d, %d) × [176, 190)", *cv, c.x0, c.x1)
			}
		})
	}
}

func TestStampOrderMultiple(t *testing.T) {
	ov := textOverlay(t, testTextFonts(t), testCatalog(), 2)
	ov.Sync([]Label{
		{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont},
		{Key: "Fly Mission", X0: 124, X1: 195, Y: 160, Font: measuredFont},
	}, 1)
	var ks []string
	for _, s := range ov.Stamps() {
		ks = append(ks, s.Key)
	}
	if got := strings.Join(ks, "|"); got != "Talk to Gwen.cover|Fly Mission.cover" {
		t.Fatalf("順序 %s", got)
	}
	if len(ov.matches) != 2 || string(ov.matches[0].textItem.raw) != "測試譯文甲" || string(ov.matches[1].textItem.raw) != "測試丙" {
		t.Fatal("文字項順序不符")
	}
}

func TestStampNotProduced(t *testing.T) {
	l := Label{Key: "K", X0: 114, X1: 204, Y: 175, Font: measuredFont}
	t.Run("超寬", func(t *testing.T) {
		ov := syncOne(t, l, strings.Repeat("測", 41), 2) // 41 × 22 = 902 > 640 − 2
		if len(ov.Stamps()) != 0 || len(ov.Skips) != 1 {
			t.Fatalf("超寬仍產生 stamp：%d", len(ov.Stamps()))
		}
	})
	t.Run("1 倍", func(t *testing.T) {
		ov := syncOne(t, l, "測試", 1)
		if len(ov.Stamps()) != 0 || len(ov.Skips) != 1 {
			t.Fatalf("1 倍仍產生 stamp：%d", len(ov.Stamps()))
		}
	})
	t.Run("缺譯文", func(t *testing.T) {
		ov := textOverlay(t, testTextFonts(t), map[string]string{}, 2)
		ov.Sync([]Label{l}, 1)
		if len(ov.Stamps()) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Reason != "translation" {
			t.Fatalf("缺譯文：stamps %d misses %+v", len(ov.Stamps()), ov.Misses)
		}
	})
	for _, tr := range []string{"測試乂", "測ゑ試"} {
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
	ov := textOverlay(t, testTextFonts(t), testCatalog(), 2)
	ov.Sync([]Label{l}, 1)
	first := ov.Stamps()[0]
	ov.Sync(nil, 1) // generation 相同：不重建
	if len(ov.Stamps()) != 1 || ov.Stamps()[0] != first {
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
	if len(ov.matches) != 1 || ov.matches[0].textItem.px != pxLabel[3] {
		t.Fatal("換倍率後未重建")
	}
}

func TestStampDrawColors(t *testing.T) {
	idx := make([]byte, 64000)
	for i := range idx {
		idx[i] = 7
	}
	l := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont}
	l.Ref = refFromFrame(idx, l) // 005 §4.1：畫格與提交時相同
	ov := syncOne(t, l, "測試譯文甲", 2)
	var pal [256][3]uint8
	pal[0], pal[7], pal[15] = [3]uint8{1, 2, 3}, [3]uint8{70, 70, 70}, [3]uint8{250, 251, 252}
	out, miss := ov.Draw(idx, pal)
	if len(miss) != 0 {
		t.Fatalf("缺字 %q", miss)
	}
	at := func(x, y int) [3]uint8 { i := 4 * (y*640 + x); return [3]uint8{out[i], out[i+1], out[i+2]} }
	// 前後列為 nil 的退路標籤：遮蓋以 0 號實心填格（006 §3.2 第 7 條）。
	if c := at(2*114, 2*176); c != pal[0] {
		t.Errorf("遮蓋左上 %v，要 0 號色 %v", c, pal[0])
	}
	if c := at(2*114-1, 2*176); c != pal[7] {
		t.Errorf("遮蓋框外 %v，要原畫格 %v", c, pal[7])
	}
	// 2 倍 22 像素、W 110：tx 263、ey 355、baseline 374；第一字填色方塊 x 264..283、y 356..375。
	ti := ov.matches[0].textItem
	if ti.tx != 263 || ti.baseline != 374 {
		t.Fatalf("tx %d baseline %d，要 263、374", ti.tx, ti.baseline)
	}
	if c := at(270, 360); c != pal[15] {
		t.Errorf("字的填色 %v，要 15 號色 %v", c, pal[15])
	}
	// 色盤改變後重畫，跟著改
	pal[15] = [3]uint8{9, 9, 9}
	out, _ = ov.Draw(idx, pal)
	if c := at(270, 360); c != pal[15] {
		t.Errorf("色盤改變後字的填色 %v，要 %v", c, pal[15])
	}
	cc := CheckCompositeText(out, ScaleIndexedRGBA(idx, pal, 2), 2, ov.Stamps(), nil, ov.TextBoxes())
	if cc.Diffs == 0 || cc.Outside != 0 {
		t.Errorf("合成圖比較 %+v", cc)
	}
}
