package strikecommander

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// strike_commander 規格 007 §4.1 的單元測試：存讀檔對話框確認框 k15 到 k20 的繁中覆繪。
// 第 1、2、5、9 項併在 saveload_test.go（TestDialogEventAccept、TestBandCoverage、TestArtGlyphPlacement、
// TestArtTextColors 等）；本檔是第 3、4、5（Pix 色號）、6、7、8 項。
//
// 原版衍生物：DOSGOLEM_SC_I015（SAVELOAD.SHP）與 DOSGOLEM_SC_I017（strike_commander workplace/i017 的畫格
// q1/q10.idx、q2/nopressed.idx、k5/ow.idx、k5/yespressed.idx、tq/tq.idx、tq/tnopressed.idx）。讀不到時 t.Skip
// 並印原因；主代理驗收時必須提供。期望數值都在測試內獨立寫出（007 §3.2 到 §3.6）。

const i017Env = "DOSGOLEM_SC_I017"

func i017Dir(t *testing.T) string {
	t.Helper()
	d := os.Getenv(i017Env)
	if d == "" {
		t.Skip(i017Env + " 未設定：007 §4.1 需要 workplace/i017 的畫格")
	}
	return d
}

func loadIdx17(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(i017Dir(t), rel))
	if err != nil {
		t.Skip("讀不到 " + rel + "：" + err.Error())
	}
	if len(b) != 64000 {
		t.Fatalf("%s 長度 %d，要 64000", rel, len(b))
	}
	return b
}

// tDraw 是一次繪圖：條目 e 畫在 (x, y)。
type tDraw struct{ e, x, y int }

// --- 第 3 項：解碼對真畫格 ---

// TestConfirmDecodeMatchesFrames：依事件順序把各元素疊起來（後畫蓋前畫），每個元素只比它在最上層的像素；
// 與畫格不同的像素，外接矩形不超過 11×11（滑鼠游標）。
func TestConfirmDecodeMatchesFrames(t *testing.T) {
	file := loadSHP(t)
	cases := []struct {
		frame string
		draws []tDraw
		check []int // 要斷言的條目（其餘只疊層）
	}{
		{"q1/q10.idx", []tDraw{{15, 101, 85}, {17, 119, 110}, {19, 170, 110}}, []int{15, 17, 19}},
		{"k5/ow.idx", []tDraw{{15, 101, 85}, {17, 119, 110}, {19, 170, 110}, {16, 119, 96}}, []int{15, 17, 19, 16}},
		{"k5/yespressed.idx", []tDraw{{15, 101, 85}, {17, 119, 110}, {19, 170, 110}, {16, 119, 96}, {18, 119, 110}}, []int{18}},
		{"tq/tq.idx", []tDraw{{36, 101, 85}, {38, 119, 110}, {40, 170, 110}}, []int{36, 38, 40}},
	}
	for _, c := range cases {
		frame := loadIdx17(t, c.frame)
		owner := make([]int, 64000)
		for i := range owner {
			owner[i] = -1
		}
		for i, d := range c.draws {
			s := mustDecode(t, file, d.e)
			for y := 0; y < s.H; y++ {
				for x := 0; x < s.W; x++ {
					owner[(d.y+y)*320+d.x+x] = i
				}
			}
		}
		for i, d := range c.draws {
			want := false
			for _, e := range c.check {
				want = want || e == d.e
			}
			s := mustDecode(t, file, d.e)
			x0, y0, x1, y1, n, own := 1<<30, 1<<30, -1, -1, 0, 0
			for y := 0; y < s.H; y++ {
				for x := 0; x < s.W; x++ {
					p := (d.y+y)*320 + d.x + x
					if owner[p] != i {
						continue
					}
					own++
					if frame[p] != s.Pix[y*s.W+x] {
						n++
						x0, y0, x1, y1 = min(x0, d.x+x), min(y0, d.y+y), max(x1, d.x+x), max(y1, d.y+y)
					}
				}
			}
			if n == 0 {
				t.Logf("%s 條目 %d：最上層 %d 像素，不同 0", c.frame, d.e, own)
			} else {
				t.Logf("%s 條目 %d：最上層 %d 像素，不同 %d，外接 (%d,%d)–(%d,%d)", c.frame, d.e, own, n, x0, y0, x1, y1)
			}
			if want && (own == 0 || n != 0 && (x1-x0+1 > 11 || y1-y0+1 > 11)) {
				t.Errorf("%s 條目 %d：與畫格不同的像素外接 (%d,%d)–(%d,%d) 超過 11×11（%d 像素）", c.frame, d.e, x0, y0, x1, y1, n)
			}
		}
	}
	// 007 §3.9：k2 字帶（y 2 到 10）在確認框開著的畫格 40 欄全相同；原本的第 1 列（絕對 y 128）被 k15 底列蓋住。
	for _, c := range []struct {
		frame string
		e     int
	}{{"q1/q10.idx", 2}, {"k5/ow.idx", 2}, {"tq/tq.idx", 23}} {
		frame := loadIdx17(t, c.frame)
		s := mustDecode(t, file, c.e)
		same, row1 := 0, 0
		for x := 20; x <= 59; x++ {
			ok := true
			for y := 2; y <= 10; y++ {
				if frame[(127+y)*320+134+x] != s.Pix[y*s.W+x] {
					ok = false
				}
			}
			if ok {
				same++
			}
			if frame[128*320+134+x] == s.Pix[1*s.W+x] {
				row1++
			}
		}
		t.Logf("%s 條目 %d：字帶 y 2 到 10 相同 %d 欄；舊字帶第 1 列相同 %d 欄", c.frame, c.e, same, row1)
		if same != 40 {
			t.Errorf("%s 條目 %d：k2 字帶 40 欄只有 %d 欄與畫格相同", c.frame, c.e, same)
		}
	}
}

// --- 第 4 項：墨跡與回填 ---

func TestConfirmInk(t *testing.T) {
	file := loadSHP(t)
	// 007 §3.3：六個態（0 組）的墨跡數，判定框等於該態墨跡框。
	want := map[int]int{15: 260, 16: 224, 17: 74, 18: 74, 19: 55, 20: 55}
	for _, k := range []int{15, 16, 17, 18, 19, 20} {
		el := tElemFor(t, k)
		jb := tJudge(t, k)
		if jb != el.ink {
			t.Errorf("k%d：判定框 %v 不等於墨跡框 %v", k, jb, el.ink)
		}
		bg := tBG(k)
		s := mustDecode(t, file, k)
		nF, nB := 0, 0
		for y := jb[1]; y <= jb[3]; y++ {
			for x := jb[0]; x <= jb[2]; x++ {
				c := s.Pix[y*s.W+x]
				isF, notB := tInk[c], !bg[c]
				if isF {
					nF++
				}
				if notB {
					nB++
				}
				if isF != notB {
					t.Errorf("k%d (%d,%d) 色號 %d：屬於 F %t、不屬於 B %t", k, x, y, c, isF, notB)
				}
			}
		}
		if nF != want[k] || nB != want[k] {
			t.Errorf("k%d：F %d、非 B %d，要 %d", k, nF, nB, want[k])
		}
		// 產品的墨跡遮罩（兩組共用 0 組位置）與獨立判定的數目相同。
		for _, g := range []int{0, 21} {
			ev := mkEvent(t, file, g+k, el.x, el.y, 1)
			_, _, mask := dialogMask(ev)
			n := 0
			for _, on := range mask {
				if on {
					n++
				}
			}
			if n != want[k] {
				t.Errorf("組 %d k%d：產品的墨跡像素 %d，要 %d", g, k, n, want[k])
			}
		}
	}
}

// --- 第 5 項：k19、k20 的 Pix 色號（沒有高光）---

func TestConfirmNoHighlightPix(t *testing.T) {
	file := loadSHP(t)
	for _, sc := range []int{2, 3} {
		for _, e := range []int{19, 20, 40, 41} {
			k := e % 21
			el := tElemFor(t, k)
			jb := tJudge(t, k)
			ov := newArtOverlay(t, sc)
			ev := mkEvent(t, file, e, el.x, el.y, 1)
			ov.AddDialogArt(ev)
			if len(ov.arts) != 1 {
				t.Fatalf("%d 倍 e %d：沒有圖面（misses %+v skips %+v）", sc, e, ov.Misses, ov.Skips)
			}
			c, _, _ := dialogColors(ev)
			bw := el.band[2] - el.band[0] + 1
			bgPix := dialogBackground(ev)
			allowed := map[uint8]bool{c: true}
			for y := jb[1]; y <= jb[3]; y++ {
				for x := jb[0]; x <= jb[2]; x++ {
					allowed[bgPix[(y-el.band[1])*bw+x-el.band[0]]] = true
				}
			}
			ow := bw * sc
			bad := map[uint8]int{}
			for y := (jb[1] - el.band[1]) * sc; y < (jb[3]-el.band[1]+1)*sc; y++ {
				for x := (jb[0] - el.band[0]) * sc; x < (jb[2]-el.band[0]+1)*sc; x++ {
					if v := ov.arts[0].idx[y*ow+x]; !allowed[v] {
						bad[v]++
					}
				}
			}
			if len(bad) != 0 {
				t.Errorf("%d 倍 e %d：判定框輸出範圍內有回填底色與字色 %d 以外的色號 %v", sc, e, c, bad)
			}
		}
	}
}

// --- 第 6 項：生命期 ---

func confirmLayerKeys(ov *Overlay) []string {
	var out []string
	for _, s := range ov.artLayer.Art {
		out = append(out, s.Key)
	}
	sort.Strings(out)
	return out
}

func sortedKeys(ks []string) []string {
	out := append([]string(nil), ks...)
	sort.Strings(out)
	return out
}

func TestConfirmLifecycle(t *testing.T) {
	file := loadSHP(t)
	k0 := "saveload/80,30/0"
	k15, k16, k17, k19 := "saveload/101,85/15", "saveload/119,96/16", "saveload/119,110/17", "saveload/170,110/19"
	send := func(ov *Overlay, order int, ks ...int) int {
		for _, k := range ks {
			el := tElemFor(t, k)
			ov.AddDialogArt(mkEvent(t, file, k, el.x, el.y, order))
			order++
		}
		return order
	}
	draw := func(ov *Overlay) { ov.Draw(make([]byte, 64000), tPal()) }
	eq := func(a, b []string) bool { return strings.Join(a, "|") == strings.Join(b, "|") }

	t.Run("k0 清確認框", func(t *testing.T) {
		ov := newArtOverlay(t, 2)
		order := send(ov, 1, 15, 17, 19, 16)
		draw(ov)
		all := sortedKeys([]string{k15, k16, k17, k19})
		if !eq(sortedKeys(ov.ArtKeys()), all) || !eq(confirmLayerKeys(ov), all) {
			t.Fatalf("ArtKeys %q、layer %q，要 %q", ov.ArtKeys(), confirmLayerKeys(ov), all)
		}
		s15, _ := artStampOf(t, ov, k15)
		s16, _ := artStampOf(t, ov, k16)
		if s15 == nil || s16 == nil || s16.Order <= s15.Order {
			t.Fatalf("Order(k16) 要大於 Order(k15)")
		}
		send(ov, order, 0)
		draw(ov)
		if !eq(ov.ArtKeys(), []string{k0}) || !eq(confirmLayerKeys(ov), []string{k0}) {
			t.Fatalf("k0 之後 ArtKeys %q、layer %q，要只剩 %q", ov.ArtKeys(), confirmLayerKeys(ov), k0)
		}
	})
	t.Run("k0 清按下態", func(t *testing.T) {
		// k18、k20 的 Key 是 n 17、19，同樣被規則 1 移除。
		ov := newArtOverlay(t, 2)
		order := send(ov, 1, 15, 18, 20)
		draw(ov)
		send(ov, order, 0)
		draw(ov)
		if !eq(ov.ArtKeys(), []string{k0}) || !eq(confirmLayerKeys(ov), []string{k0}) {
			t.Fatalf("ArtKeys %q、layer %q", ov.ArtKeys(), confirmLayerKeys(ov))
		}
	})
	t.Run("k15 清 k16", func(t *testing.T) {
		ov := newArtOverlay(t, 2)
		order := send(ov, 1, 17, 16)
		draw(ov)
		send(ov, order, 15)
		draw(ov)
		want := sortedKeys([]string{k17, k15})
		if !eq(sortedKeys(ov.ArtKeys()), want) || !eq(confirmLayerKeys(ov), want) {
			t.Fatalf("ArtKeys %q、layer %q，要 %q", ov.ArtKeys(), confirmLayerKeys(ov), want)
		}
	})
	t.Run("k0 缺譯文仍清", func(t *testing.T) {
		gfx := testGfx()
		delete(gfx, "SAVE GAME")
		ov, err := NewOverlay(testLabelFont(t), testCatalog(), gfx, 2)
		if err != nil {
			t.Fatal(err)
		}
		order := send(ov, 1, 15, 17, 19, 16)
		draw(ov)
		if len(confirmLayerKeys(ov)) != 4 {
			t.Fatalf("layer %q", confirmLayerKeys(ov))
		}
		send(ov, order, 0)
		draw(ov)
		if len(ov.ArtKeys()) != 0 || len(confirmLayerKeys(ov)) != 0 {
			t.Fatalf("ArtKeys %q、layer %q，要都空", ov.ArtKeys(), confirmLayerKeys(ov))
		}
		if len(ov.Misses) != 1 || ov.Misses[0].Key != "SAVE GAME" || ov.Misses[0].Reason != MissTranslation {
			t.Fatalf("k0 要記 Miss：%+v", ov.Misses)
		}
	})
	t.Run("其他事件不清", func(t *testing.T) {
		ov := newArtOverlay(t, 2)
		order := send(ov, 1, 15, 17, 19, 16)
		send(ov, order, 2, 6, 8, 18, 20)
		want := sortedKeys([]string{k15, k16, k17, k19, "saveload/134,127/2", "saveload/90,142/6", "saveload/174,142/8"})
		if !eq(sortedKeys(ov.ArtKeys()), want) {
			t.Fatalf("ArtKeys %q，要 %q", ov.ArtKeys(), want)
		}
	})
}

// --- 第 7 項：逐欄 ---

func TestConfirmPerColumn(t *testing.T) {
	file := loadSHP(t)
	pal := tPal()
	for _, c := range []struct {
		frame string
		e     int
	}{{"q2/nopressed.idx", 20}, {"tq/tnopressed.idx", 41}} {
		frame := loadIdx17(t, c.frame)
		el := tElemFor(t, c.e%21)
		ov := newArtOverlay(t, 2)
		ov.AddDialogArt(mkEvent(t, file, c.e, el.x, el.y, 1))
		out, _ := ov.Draw(frame, pal)
		s, ok := artStampOf(t, ov, el.key)
		if !ok {
			t.Fatalf("%s：沒有圖面", c.frame)
		}
		sh, hi, ot := columnState(t, out, ScaleIndexedRGBA(frame, pal, 2), s, 2)
		t.Logf("%s e %d：顯示 %d、停畫 %d、其他 %d", c.frame, c.e, sh, hi, ot)
		if s.Cells != 36 || sh != 25 || hi != 11 || ot != 0 {
			t.Errorf("%s e %d：字帶 %d 欄，顯示 %d、停畫 %d、其他 %d，要 36 欄、25、11、0", c.frame, c.e, s.Cells, sh, hi, ot)
		}
	}
}

// --- 第 8 項：Miss 與 lint ---

func TestConfirmMissLint(t *testing.T) {
	file := loadSHP(t)
	el := tElemFor(t, 17)
	mk := func(gfx map[string]string) *Overlay {
		ov, err := NewOverlay(testLabelFont(t), testCatalog(), gfx, 2)
		if err != nil {
			t.Fatal(err)
		}
		ov.AddDialogArt(mkEvent(t, file, 17, el.x, el.y, 1))
		ov.Draw(make([]byte, 64000), tPal())
		return ov
	}
	gfx := testGfx()
	delete(gfx, "YES")
	ov := mk(gfx)
	if len(ov.Art()) != 0 || len(ov.artLayer.Art) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Key != "YES" || ov.Misses[0].Reason != MissTranslation {
		t.Fatalf("缺譯文：圖面 %d、misses %+v", len(ov.Art()), ov.Misses)
	}
	gfx = testGfx()
	gfx["YES"] = "乂"
	ov = mk(gfx)
	if len(ov.Art()) != 0 || len(ov.Misses) != 1 || ov.Misses[0].Reason != MissGlyph || string(ov.Misses[0].Runes) != "乂" {
		t.Fatalf("缺字：圖面 %d、misses %+v", len(ov.Art()), ov.Misses)
	}
	// 對照：有譯文時產生圖面。
	if ov = mk(testGfx()); len(ov.Art()) != 1 {
		t.Fatalf("對照組沒有圖面：misses %+v skips %+v", ov.Misses, ov.Skips)
	}
	wide := testLabelFont(t).Wide
	b, err := os.ReadFile("testdata/menu_gfx.test.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]string{
		"缺 YES":   strings.Replace(string(b), "YES\t甲\t測試（007 §4.1）\n", "", 1),
		"YES 缺字":  strings.Replace(string(b), "YES\t甲\t", "YES\t乂\t", 1),
		"缺 NO":    strings.Replace(string(b), "NO\t乙\t測試（007 §4.1）\n", "", 1),
		"缺 ARE…":  strings.Replace(string(b), "ARE YOU SURE\t一二三\t測試（007 §4.1）\n", "", 1),
		"缺 OVER…": strings.Replace(string(b), "OVERWRITE\t飛彈卸下\t測試（007 §4.1）\n", "", 1),
	} {
		if s == string(b) {
			t.Fatalf("%s：替換沒有生效", name)
		}
		rep := LintArt([]byte(s), wide)
		if rep.OK() {
			t.Errorf("%s 應報錯", name)
		}
	}
	if keys := ArtCatalogKeys(); len(keys) != 10 {
		t.Errorf("ArtCatalogKeys %q，要 10 個", keys)
	}
}
