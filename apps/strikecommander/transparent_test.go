package strikecommander

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// strike_commander 規格 006 §4.1：熱點標籤去掉黑底（透明標籤）、中文字加黑外框。
// 沿用 observer_test.go 的合成 harness、labelframe_test.go 的 lfPal、lfFrame、refFromFrame。

// --- 測試字型：另加有亮點的全形空白，「一」是一條橫線 ---

// otRunes 是本檔的全形字；「一」以外每字每列左右兩緣各一點（同 buildTestFont），「一」只有第 7 列整列亮。
const otRunes = "測試譯文甲一　"

func otFont(t *testing.T) *LabelFont {
	t.Helper()
	rs := []rune(otRunes)
	b := []byte("GOLEMFNT")
	b = binary.LittleEndian.AppendUint16(b, 16)
	b = binary.LittleEndian.AppendUint16(b, 16)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(rs)))
	for _, r := range rs {
		b = binary.LittleEndian.AppendUint32(b, uint32(r))
		b = append(b, 0x81)
		for row := 0; row < 16; row++ {
			switch {
			case r == '一' && row == 7:
				b = append(b, 0xFF, 0xFF)
			case r == '一':
				b = append(b, 0, 0)
			default:
				b = append(b, 0x81, 0x81)
			}
		}
	}
	f, err := ParseLabelFont(b, "full16-outline-test")
	if err != nil {
		t.Fatal(err)
	}
	if g := f.Font.Glyphs['　']; g == nil || g[0] == 0 {
		t.Fatal("測試字型的全形空白沒有亮點")
	}
	return f
}

// expectGlyphs 在測試內獨立算字形點集（輸出像素）：跳過 i ≥ Cells 與空白，字模第 gy 列第 gx 欄亮時取
// ((X + i × CellW) × sc + GlyphX + gx, Y × sc + GlyphY + gy)，只留格內的點。GlyphScale 為 1。
func expectGlyphs(f *xlate.Font, s *xlate.Stamp, sc int) map[[2]int]bool {
	out := map[[2]int]bool{}
	for i, r := range s.Text {
		if i >= s.Cells || r == ' ' || r == '　' {
			continue
		}
		g := f.Glyphs[r]
		for gy := 0; gy < 16; gy++ {
			for gx := 0; gx < 16; gx++ {
				if g[gy*2+gx/8]&(0x80>>uint(gx%8)) == 0 {
					continue
				}
				lx, ly := s.GlyphX+gx, s.GlyphY+gy
				if lx < 0 || lx >= s.CellW*sc || ly < 0 || ly >= s.CellH*sc {
					continue
				}
				out[[2]int{(s.X+i*s.CellW)*sc + lx, s.Y*sc + ly}] = true
			}
		}
	}
	return out
}

// expectOutline 是 8 鄰域膨脹減字形，裁到遮蓋範圍（輸出像素）[cx0, cx1) × [cy0, cy1)。
func expectOutline(g map[[2]int]bool, cx0, cy0, cx1, cy1 int) map[[2]int]bool {
	out := map[[2]int]bool{}
	for p := range g {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				q := [2]int{p[0] + dx, p[1] + dy}
				if g[q] || q[0] < cx0 || q[0] >= cx1 || q[1] < cy0 || q[1] >= cy1 {
					continue
				}
				out[q] = true
			}
		}
	}
	return out
}

func maskSet(m []bool, sc int) map[[2]int]bool {
	out := map[[2]int]bool{}
	W := 320 * sc
	for i, b := range m {
		if b {
			out[[2]int{i % W, i / W}] = true
		}
	}
	return out
}

func setDiff(a, b map[[2]int]bool) (onlyA, onlyB int) {
	for p := range a {
		if !b[p] {
			onlyA++
		}
	}
	for p := range b {
		if !a[p] {
			onlyB++
		}
	}
	return
}

func px(img []byte, sc, x, y int) [3]uint8 {
	i := 4 * (y*320*sc + x)
	return [3]uint8{img[i], img[i+1], img[i+2]}
}

// coverItem 依 PlanLabel 的結果建一筆背景項。
func coverItem(p Plan, l Label, before, after []byte) bgItem {
	return bgItem{key: l.Key, kind: l.Kind, x0: p.Cover.X, x1: p.Cover.X + p.Cover.CellW, y0: p.Cover.Y, rows: refRows(l.Kind), before: before, after: after}
}

// --- 第 1 項：觀察器 ---

// labelDuring 模擬一筆熱點標籤：入口游標 y 為 yEntry，第一次 putc 時 y 為 yPutc；during 在 putc 之後、
// puts 返回之前畫字。
func (h *harness) labelDuring(s string, x0, x1, yEntry, yPutc int, during func()) {
	h.setString(s)
	h.setCursor(0xCC, uint16(yEntry))
	h.putsEntry()
	h.setCursor(uint16(x0), uint16(yPutc))
	h.putc(tObj)
	if during != nil {
		during()
	}
	h.setCursor(uint16(x1), uint16(yPutc))
	h.putsReturn()
}

func TestTransparentObserverRows(t *testing.T) {
	prep := func(h *harness) {
		h.bufPx(10, 175, 70) // 第 Y 列：不在前後列內
		h.bufPx(10, 176, 71) // 第 Y+1 列：前後列第 0 列
		h.bufPx(11, 189, 72) // 第 Y+14 列：前後列第 13 列
		h.bufPx(12, 190, 73) // 第 Y+15 列：不在前後列內
	}
	paint := func(h *harness) func() {
		return func() { h.bufPx(120, 180, 15) }
	}
	for _, ink := range []bool{false, true} {
		t.Run(fmt.Sprintf("InkCheck %t", ink), func(t *testing.T) {
			h := newHarness(t)
			h.o.InkCheck = ink
			prep(h)
			h.labelDuring("Talk to Gwen", 114, 204, 175, 175, paint(h))
			p := h.o.Pending()
			if len(p) != 1 {
				t.Fatalf("待提交 %d 筆，要 1", len(p))
			}
			l := p[0]
			if len(l.Before) != 14*320 || len(l.After) != 14*320 {
				t.Fatalf("前列 %d、後列 %d bytes，要各 %d", len(l.Before), len(l.After), 14*320)
			}
			at := func(b []byte, x, y int) byte { return b[(y-176)*320+x] }
			if at(l.Before, 10, 176) != 71 || at(l.Before, 11, 189) != 72 || at(l.After, 11, 189) != 72 {
				t.Errorf("前後列範圍不是 [Y+1, Y+15)：%d %d %d", at(l.Before, 10, 176), at(l.Before, 11, 189), at(l.After, 11, 189))
			}
			if at(l.Before, 120, 180) != 0 || at(l.After, 120, 180) != 15 {
				t.Errorf("墨跡像素前列 %d（要入口時的 0）、後列 %d（要返回時的 15）", at(l.Before, 120, 180), at(l.After, 120, 180))
			}
			if !reflect.DeepEqual(l.BoardInk, InkResult{}) {
				t.Errorf("label 的 BoardInk 要是零值：%+v", l.BoardInk)
			}
			if ink && len(l.ink.before) != 19*320 {
				t.Errorf("InkCheck 開時 ink.before %d bytes，要 %d", len(l.ink.before), 19*320)
			}
			if !ink && l.ink.before != nil {
				t.Errorf("InkCheck 關時不該讀 ink.before")
			}
		})
	}
	t.Run("Y 與 y′ 不同", func(t *testing.T) {
		for _, ink := range []bool{false, true} {
			h := newHarness(t)
			h.o.InkCheck = ink
			h.labelDuring("Talk to Gwen", 114, 204, 174, 175, paint(h))
			p := h.o.Pending()
			if len(p) != 1 {
				t.Fatalf("待提交 %d 筆，要 1", len(p))
			}
			if p[0].Before != nil || p[0].After != nil {
				t.Fatalf("InkCheck %t：前後列應作廢：前列 %d、後列 %d bytes", ink, len(p[0].Before), len(p[0].After))
			}
			if !reflect.DeepEqual(p[0].BoardInk, InkResult{}) {
				t.Errorf("作廢分支的 label BoardInk 要是零值：%+v", p[0].BoardInk)
			}
			if !hasLog(h.o, "label") {
				t.Error("作廢時沒有記種類為 label 的紀錄")
			}
		}
	})
	t.Run("board 仍是 8 列", func(t *testing.T) {
		h := newBoardHarness(t)
		h.boardPuts("GROUND", tBoardSeg, 0x1369, gX0, gX1, gY, gY, func() { paintGround(h) })
		p := h.o.Pending()
		if len(p) != 1 || p[0].Kind != KindBoard || len(p[0].Before) != 8*320 || len(p[0].After) != 8*320 || !p[0].BoardInk.OK() {
			t.Fatalf("board 前後列 %d、%d bytes，核對 %s", len(p[0].Before), len(p[0].After), p[0].BoardInk)
		}
	})
}

// --- 第 2 項：背景（墨跡換回前列、游標保留） ---

func TestLabelBackgroundInk(t *testing.T) {
	pal := lfPal()
	for _, sc := range []int{2, 3} {
		t.Run(strconv.Itoa(sc), func(t *testing.T) {
			base := lfFrame()
			l := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont, Kind: KindLabel}
			before := refFromFrame(base, l)
			after := append([]byte(nil), before...)
			set := func(b []byte, x, y int, c byte) { b[(y-176)*320+x] = c }
			get := func(b []byte, x, y int) byte { return b[(y-176)*320+x] }
			// 英文墨跡：A、B 兩點；C 是墨跡且被游標壓住。
			set(after, 120, 180, 15)
			set(after, 130, 181, 0)
			set(after, 140, 182, 15)
			// 畫格 = 後列，再疊游標（色號 33）：C 壓在墨跡上，D 在非墨跡處。
			frame := append([]byte(nil), base...)
			for y := 176; y < 190; y++ {
				copy(frame[y*320:(y+1)*320], after[(y-176)*320:(y-175)*320])
			}
			frame[182*320+140] = 33
			frame[183*320+150] = 33
			if get(before, 140, 182) == 33 || get(before, 150, 183) == 33 {
				t.Fatal("游標色與前列相同，測試無鑑別力")
			}
			p, err := PlanLabel(l, []rune("測試譯文甲"), sc, testLabelFont(t).Font)
			if err != nil {
				t.Fatal(err)
			}
			out := ScaleIndexedRGBA(frame, pal, sc)
			bg := labelBackground(out, frame, pal, sc, []bgItem{coverItem(p, l, before, after)}, []*xlate.Stamp{nil})
			check := func(name string, x, y int, want [3]uint8) {
				t.Helper()
				for dy := 0; dy < sc; dy++ {
					for dx := 0; dx < sc; dx++ {
						if c := px(bg, sc, x*sc+dx, y*sc+dy); c != want {
							t.Errorf("%s (%d, %d) 放大後 = %v，要 %v", name, x, y, c, want)
						}
					}
				}
			}
			check("墨跡 A 換回前列", 120, 180, pal[get(before, 120, 180)])
			check("墨跡 B 換回前列", 130, 181, pal[get(before, 130, 181)])
			check("壓在墨跡上的游標保留", 140, 182, pal[33])
			check("非墨跡處的游標保留", 150, 183, pal[33])
			// 墨跡以外的像素，以及遮蓋範圍外，都等於原版放大圖。
			ink := map[[2]int]bool{{120, 180}: true, {130, 181}: true}
			for y := 0; y < 200; y++ {
				for x := 0; x < 320; x++ {
					if ink[[2]int{x, y}] {
						continue
					}
					for dy := 0; dy < sc; dy++ {
						for dx := 0; dx < sc; dx++ {
							if px(bg, sc, x*sc+dx, y*sc+dy) != px(out, sc, x*sc+dx, y*sc+dy) {
								t.Fatalf("非墨跡像素 (%d, %d) 被改動", x, y)
							}
						}
					}
				}
			}
		})
	}
}

// --- 第 3 項：外框 ---

func TestGlyphOutline(t *testing.T) {
	f := otFont(t)
	pal := lfPal()
	for _, sc := range []int{2, 3} {
		t.Run(strconv.Itoa(sc), func(t *testing.T) {
			frame := lfFrame()
			l := Label{Key: "K", X0: 100, X1: 220, Y: 175, Font: measuredFont, Kind: KindLabel}
			l.Before, l.After, l.Ref = refFromFrame(frame, l), refFromFrame(frame, l), refFromFrame(frame, l)
			text := "測一　一測"
			p, err := PlanLabel(l, []rune(text), sc, f.Font)
			if err != nil {
				t.Fatal(err)
			}
			g := expectGlyphs(f.Font, p.Text, sc)
			if len(g) == 0 {
				t.Fatal("字形點集是空的")
			}
			if a, b := setDiff(maskSet(glyphMask(p.Text, sc), sc), g); a != 0 || b != 0 {
				t.Fatalf("glyphMask 與獨立算出的字形不同：多 %d、少 %d", a, b)
			}
			cx0, cy0, cx1, cy1 := p.Cover.X*sc, 176*sc, (p.Cover.X+p.Cover.CellW)*sc, 190*sc
			wantOL := expectOutline(g, cx0, cy0, cx1, cy1)
			// 全形空白格（第 2 格）內部：不是字形、也不在外框。
			sx0 := (p.Text.X + 2*p.Text.CellW) * sc
			for y := cy0; y < cy1; y++ {
				for x := sx0 + 1; x < sx0+p.Text.CellW*sc-1; x++ {
					if g[[2]int{x, y}] || wantOL[[2]int{x, y}] {
						t.Fatalf("全形空白格內 (%d, %d) 有字形或外框", x, y)
					}
				}
			}
			// 直接測 labelBackground：沒有墨跡，背景圖與原版放大圖只差在外框。
			out := ScaleIndexedRGBA(frame, pal, sc)
			bg := labelBackground(out, frame, pal, sc, []bgItem{coverItem(p, l, l.Before, l.After)}, []*xlate.Stamp{p.Text})
			gotOL := map[[2]int]bool{}
			W, H := 320*sc, 200*sc
			for y := 0; y < H; y++ {
				for x := 0; x < W; x++ {
					if px(bg, sc, x, y) == px(out, sc, x, y) {
						continue
					}
					gotOL[[2]int{x, y}] = true
					if c := px(bg, sc, x, y); c != pal[0] {
						t.Fatalf("外框 (%d, %d) = %v，要 0 號 %v", x, y, c, pal[0])
					}
				}
			}
			if a, b := setDiff(gotOL, wantOL); a != 0 || b != 0 {
				t.Fatalf("外框像素與 8 鄰域膨脹減字形不同：多 %d、少 %d（共 %d）", a, b, len(wantOL))
			}
			// 合成圖：字形 15 號、外框 0 號，遮蓋範圍內其他像素等於原版放大圖。
			ov, err := NewOverlay(f, map[string]string{"K": text}, nil, sc)
			if err != nil {
				t.Fatal(err)
			}
			ov.Sync([]Label{l}, 1)
			comp, miss := ov.Draw(frame, pal)
			if len(miss) != 0 || len(ov.Hidden) != 0 || len(ov.BGFallbacks) != 0 {
				t.Fatalf("缺字 %q、Hidden %v、BGFallbacks %+v", miss, ov.Hidden, ov.BGFallbacks)
			}
			for y := cy0; y < cy1; y++ {
				for x := cx0; x < cx1; x++ {
					c := px(comp, sc, x, y)
					switch q := [2]int{x, y}; {
					case g[q]:
						if c != pal[15] {
							t.Fatalf("字形 (%d, %d) = %v，要 15 號", x, y, c)
						}
					case wantOL[q]:
						if c != pal[0] {
							t.Fatalf("外框 (%d, %d) = %v，要 0 號", x, y, c)
						}
					default:
						if c != px(out, sc, x, y) {
							t.Fatalf("遮蓋內其他像素 (%d, %d) = %v，要原版 %v", x, y, c, px(out, sc, x, y))
						}
					}
				}
			}
		})
	}
	t.Run("i ≥ Cells", func(t *testing.T) {
		for _, sc := range []int{2, 3} {
			l := Label{Key: "K", X0: 100, X1: 220, Y: 175, Font: measuredFont, Kind: KindLabel}
			p, err := PlanLabel(l, []rune("測一"), sc, f.Font)
			if err != nil {
				t.Fatal(err)
			}
			s := *p.Text
			s.Text = []rune("測一測一") // 第 2、3 格超出 Cells
			if a, b := setDiff(maskSet(glyphMask(&s, sc), sc), expectGlyphs(f.Font, p.Text, sc)); a != 0 || b != 0 {
				t.Fatalf("%d 倍：i ≥ Cells 的格仍有字形：多 %d、少 %d", sc, a, b)
			}
		}
	})
}

// --- 第 4 項：外框裁切 ---

func TestOutlineClip(t *testing.T) {
	pal := lfPal()
	font := testLabelFont(t).Font
	for _, sc := range []int{2, 3} {
		frame := lfFrame()
		// 英文窄、譯文寬：文字 stamp 貼齊遮蓋左緣（cover.X = text.X > 0）。
		l := Label{Key: "K", X0: 100, X1: 110, Y: 175, Font: measuredFont, Kind: KindLabel}
		before := refFromFrame(frame, l)
		p, err := PlanLabel(l, []rune("測試譯文甲"), sc, font)
		if err != nil {
			t.Fatal(err)
		}
		if p.Cover.X <= 0 || p.Cover.X != p.Text.X {
			t.Fatalf("%d 倍：cover.X %d、text.X %d，要相等且 > 0", sc, p.Cover.X, p.Text.X)
		}
		out := ScaleIndexedRGBA(frame, pal, sc)
		bg := labelBackground(out, frame, pal, sc, []bgItem{coverItem(p, l, before, before)}, []*xlate.Stamp{p.Text})
		x := p.Cover.X*sc - 1
		for y := 0; y < 200*sc; y++ {
			if px(bg, sc, x, y) != px(out, sc, x, y) {
				t.Fatalf("%d 倍：遮蓋左緣外一欄 (%d, %d) 被改動", sc, x, y)
			}
		}
		if sc == 2 && !glyphMask(p.Text, sc)[(176*sc+p.Text.GlyphY)*320*sc+x+1] {
			// 2 倍 GlyphX 0：字模第 0 欄貼在遮蓋左緣，不裁切時外框會落在 x；確認情境成立。
			t.Fatal("2 倍：遮蓋左緣第一欄不是字形，測試無鑑別力")
		}
	}
}

// --- 第 5 項：退路 ---

func TestBGFallback(t *testing.T) {
	pal := lfPal()
	frame := lfFrame()
	tl := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont, Kind: KindLabel}
	tl.Before, tl.After, tl.Ref = refFromFrame(frame, tl), refFromFrame(frame, tl), refFromFrame(frame, tl)
	fb := Label{Key: "Fly Mission", X0: 124, X1: 195, Y: 140, Font: measuredFont, Kind: KindLabel}
	fb.After, fb.Ref = refFromFrame(frame, fb), refFromFrame(frame, fb) // 前列 nil
	ov, err := NewOverlay(testLabelFont(t), testCatalog(), nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	ov.Sync([]Label{tl, fb}, 1)
	if len(ov.BGFallbacks) != 1 || ov.BGFallbacks[0].Key != "Fly Mission" || len(ov.Skips) != 0 {
		t.Fatalf("BGFallbacks %+v、Skips %+v，要只有 Fly Mission、Skips 空", ov.BGFallbacks, ov.Skips)
	}
	comp, _ := ov.Draw(frame, pal)
	if len(ov.Hidden) != 0 {
		t.Fatalf("Hidden %v", ov.Hidden)
	}
	cv := coverOf(t, ov, fb.Key)
	var txt *xlate.Stamp
	for _, s := range ov.Stamps() {
		if s.Key == fb.Key+".text" {
			txt = s
		}
	}
	g := maskSet(glyphMask(txt, 2), 2)
	for y := cv.Y * 2; y < (cv.Y+cv.CellH)*2; y++ {
		for x := cv.X * 2; x < (cv.X+cv.CellW)*2; x++ {
			want := pal[0]
			if g[[2]int{x, y}] {
				want = pal[15]
			}
			if c := px(comp, 2, x, y); c != want {
				t.Fatalf("退路標籤遮蓋內 (%d, %d) = %v，要 %v（實心 0 號底）", x, y, c, want)
			}
		}
	}
	// 重建兩次仍恰一筆；Skips 不變。
	ov.Sync([]Label{tl, fb}, 2)
	ov.Sync([]Label{tl, fb}, 3)
	if len(ov.BGFallbacks) != 1 || len(ov.Skips) != 0 {
		t.Fatalf("重建後 BGFallbacks %+v、Skips %+v，要恰一筆、空", ov.BGFallbacks, ov.Skips)
	}
	rep := ov.BGReport()
	if len(rep) != 2 || rep[0].State != BGStateTransparent || rep[1].State != BGStateFallback || rep[1].Before != nil {
		t.Fatalf("BGReport %+v", rep)
	}
}

// --- 第 6 項：同畫格有透明標籤與 board ---

// oldBoardBackground 是 003 §3.4（006 之前）的 board 背景：在畫格複本上把墨跡換回前列，再整幅放大。
func oldBoardBackground(indexed []byte, pal [256][3]uint8, sc int, items []bgItem) []byte {
	idx := append([]byte(nil), indexed...)
	for _, b := range items {
		for y := b.y0; y < b.y0+8; y++ {
			row := (y - b.y0) * 320
			for x := b.x0; x < b.x1; x++ {
				if bf, af := b.before[row+x], b.after[row+x]; bf != af && idx[y*320+x] == af {
					idx[y*320+x] = bf
				}
			}
		}
	}
	return ScaleIndexedRGBA(idx, pal, sc)
}

func TestBoardWithTransparentLabel(t *testing.T) {
	pal := lfPal()
	base := lfFrame()
	g := Label{Key: "GROUND", X0: 177, X1: 250, Y: 45, Font: boardFont, Kind: KindBoard}
	before := refFromFrame(base, g)
	after := append([]byte(nil), before...)
	for _, p := range []Point{{180, 47}, {200, 49}, {239, 50}, {245, 52}} {
		after[(p.Y-46)*320+p.X] = 10
	}
	frame := append([]byte(nil), base...)
	for y := 46; y < 54; y++ {
		copy(frame[y*320:(y+1)*320], after[(y-46)*320:(y-45)*320])
	}
	frame[48*320+190] = 15 // 游標（非墨跡處）
	g.Before, g.After, g.Ref = before, after, refFromFrame(frame, g)
	lab := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont, Kind: KindLabel}
	lb := refFromFrame(frame, lab)
	la := append([]byte(nil), lb...)
	la[(180-176)*320+120] = 15
	for y := 176; y < 190; y++ {
		copy(frame[y*320:(y+1)*320], la[(y-176)*320:(y-175)*320])
	}
	lab.Before, lab.After, lab.Ref = lb, la, refFromFrame(frame, lab)
	tr := map[string]string{"GROUND": "地面", "Talk to Gwen": "測試譯文甲"}
	for _, sc := range []int{2, 3} {
		only, _ := NewOverlay(testLabelFont(t), tr, nil, sc)
		only.Sync([]Label{g}, 1)
		both, _ := NewOverlay(testLabelFont(t), tr, nil, sc)
		both.Sync([]Label{g, lab}, 1)
		a, _ := only.Draw(frame, pal)
		b, _ := both.Draw(frame, pal)
		if len(only.Hidden)+len(both.Hidden) != 0 {
			t.Fatalf("Hidden %v %v", only.Hidden, both.Hidden)
		}
		// board 的遮蓋範圍（與它之外、label 遮蓋之外的全部像素）逐位元組相同。
		lc := coverOf(t, both, lab.Key)
		W := 320 * sc
		for i := 0; i < len(a); i += 4 {
			x, y := (i/4)%W/sc, (i/4)/W/sc
			if x >= lc.X && x < lc.X+lc.CellW && y >= lc.Y && y < lc.Y+lc.CellH {
				continue
			}
			if !bytes.Equal(a[i:i+4], b[i:i+4]) {
				t.Fatalf("%d 倍：(%d, %d) 有 label 時與只有 board 時不同", sc, x, y)
			}
		}
		// board 背景與 006 之前的 boardBackground 逐位元組相同。
		items, texts := only.backgroundItems(make([]bool, 1))
		got := labelBackground(ScaleIndexedRGBA(frame, pal, sc), frame, pal, sc, items, texts)
		if !bytes.Equal(got, oldBoardBackground(frame, pal, sc, items)) {
			t.Fatalf("%d 倍：board 背景與 003 的 boardBackground 不同", sc)
		}
	}
}

// --- 第 7、11 項：同 generation 更新前後列；BGReport ---

func TestTransparentSameGenUpdate(t *testing.T) {
	pal := lfPal()
	base := lfFrame()
	l := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont, Kind: KindLabel}
	before := refFromFrame(base, l)
	after := append([]byte(nil), before...)
	const ix, iy = 120, 180
	after[(iy-176)*320+ix] = 77
	frame := append([]byte(nil), base...)
	frame[iy*320+ix] = 77
	l.Before, l.After, l.Ref = before, after, refFromFrame(frame, l)
	ov, _ := NewOverlay(testLabelFont(t), testCatalog(), nil, 2)
	ov.Sync([]Label{l}, 1)
	at := func(img []byte) [3]uint8 { return px(img, 2, 2*ix, 2*iy) }
	out, _ := ov.Draw(frame, pal)
	if at(out) != pal[before[(iy-176)*320+ix]] {
		t.Fatalf("第一對前後列：墨跡 (%d, %d) = %v，要前列色", ix, iy, at(out))
	}
	// 同 generation 新的一對：該點不是墨跡，保留畫格。
	after2 := append([]byte(nil), before...)
	l2 := l
	l2.Before, l2.After = append([]byte(nil), before...), after2
	first := ov.Stamps()[1]
	ov.Sync([]Label{l2}, 1)
	if ov.Stamps()[1] != first {
		t.Fatal("generation 相同時重建了 stamp")
	}
	out, _ = ov.Draw(frame, pal)
	if at(out) != pal[77] {
		t.Fatalf("新的一對前後列：(%d, %d) = %v，要畫格的 77 號", ix, iy, at(out))
	}
	// 新的一對有 nil：沿用上一對。
	l3 := l
	l3.Before, l3.After = before, nil
	ov.Sync([]Label{l3}, 1)
	out, _ = ov.Draw(frame, pal)
	if at(out) != pal[77] {
		t.Fatalf("新後列 nil 時應沿用上一對：(%d, %d) = %v", ix, iy, at(out))
	}
	rep := ov.BGReport()
	if len(rep) != 1 || !bytes.Equal(rep[0].After, after2) || !bytes.Equal(rep[0].Before, before) || rep[0].State != BGStateTransparent || rep[0].Hidden {
		t.Fatalf("BGReport 不是沿用的那一對：%+v", rep)
	}
	e, tx := rep[0], ov.Stamps()[1]
	cv := ov.Stamps()[0]
	if e.Text != "測試譯文甲" || e.TextX != tx.X || e.TextY != 176 || e.Cells != 5 || e.CellW != tx.CellW || e.CellH != 14 ||
		e.GlyphX != tx.GlyphX || e.GlyphY != tx.GlyphY || e.CoverX != cv.X || e.CoverY != 176 || e.CoverW != cv.CellW || e.CoverH != 14 ||
		!bytes.Equal(e.Ref, l.Ref) || e.Kind != KindLabel || e.Key != l.Key {
		t.Fatalf("BGReport 欄位與 stamp 不符：%+v", e)
	}
	// Hidden 取最近一次 Draw。
	bad := append([]byte(nil), frame...)
	bad[178*320+150] ^= 0x40
	ov.Draw(bad, pal)
	if r := ov.BGReport(); !r[0].Hidden {
		t.Fatal("比對不符的 Draw 之後 BGReport.Hidden 要為真")
	}
}

// --- 第 8 項：比對不符的標籤不列入背景清單 ---

func TestBackgroundItemsSkipBad(t *testing.T) {
	frame := lfFrame()
	a := lfLabel(frame)
	a.Before, a.After = refFromFrame(frame, a), refFromFrame(frame, a)
	b := Label{Key: "Fly Mission", X0: 124, X1: 195, Y: 140, Font: measuredFont, Kind: KindLabel}
	b.Before, b.After, b.Ref = refFromFrame(frame, b), refFromFrame(frame, b), refFromFrame(frame, b)
	ov := lfOverlay(t, []Label{a, b})
	items, texts := ov.backgroundItems([]bool{true, false})
	if len(items) != 1 || items[0].key != "Fly Mission" || len(texts) != 1 || texts[0] == nil || texts[0].Key != "Fly Mission.text" {
		var ks []string
		for _, it := range items {
			ks = append(ks, it.key)
		}
		t.Fatalf("背景清單 %q，要只有 Fly Mission", ks)
	}
	if items, _ := ov.backgroundItems([]bool{false, false}); len(items) != 2 {
		t.Fatalf("都相符時背景清單 %d 筆，要 2", len(items))
	}
}

// --- 第 9 項：圖面相交 ---

func TestLabelBackgroundArt(t *testing.T) {
	pal := lfPal()
	for _, sc := range []int{2, 3} {
		base := lfFrame()
		l := Label{Key: "Talk to Gwen", X0: 114, X1: 204, Y: 175, Font: measuredFont, Kind: KindLabel}
		before := refFromFrame(base, l)
		after := append([]byte(nil), before...)
		after[(180-176)*320+120] = 15 // P1：在圖面內
		after[(181-176)*320+190] = 15 // P2：在圖面外
		frame := append([]byte(nil), base...)
		for y := 176; y < 190; y++ {
			copy(frame[y*320:(y+1)*320], after[(y-176)*320:(y-175)*320])
		}
		p, err := PlanLabel(l, []rune("測試譯文甲"), sc, testLabelFont(t).Font)
		if err != nil {
			t.Fatal(err)
		}
		// 模擬圖面：原版矩形 [110, 160) × [178, 186) 的輸出像素改成 50 號色。
		out := ScaleIndexedRGBA(frame, pal, sc)
		art := pal[50]
		if art == pal[15] || art == pal[before[(180-176)*320+120]] {
			t.Fatal("圖面色與墨跡前後列相同，測試無鑑別力")
		}
		W := 320 * sc
		for y := 178 * sc; y < 186*sc; y++ {
			for x := 110 * sc; x < 160*sc; x++ {
				i := 4 * (y*W + x)
				out[i], out[i+1], out[i+2] = art[0], art[1], art[2]
			}
		}
		bg := labelBackground(out, frame, pal, sc, []bgItem{coverItem(p, l, before, after)}, []*xlate.Stamp{nil})
		for dy := 0; dy < sc; dy++ {
			for dx := 0; dx < sc; dx++ {
				if c := px(bg, sc, 120*sc+dx, 180*sc+dy); c != art {
					t.Errorf("%d 倍：圖面內的墨跡 P1 = %v，要保留圖面 %v", sc, c, art)
				}
				if c, want := px(bg, sc, 190*sc+dx, 181*sc+dy), pal[before[(181-176)*320+190]]; c != want {
					t.Errorf("%d 倍：圖面外的墨跡 P2 = %v，要前列 %v", sc, c, want)
				}
			}
		}
	}
}

// --- 收據的 TSV 與 rows.bin ---

func TestBGTSV(t *testing.T) {
	entries := []BGEntry{
		{Key: "A", Kind: KindLabel, State: BGStateTransparent, Text: "測", Before: []byte{1, 2}, After: []byte{3}, Ref: []byte{4, 5, 6}},
		{Key: "B", Kind: KindLabel, State: BGStateFallback, Text: "試", After: []byte{7}, Ref: []byte{8}, Hidden: true},
	}
	tsv, rows := BGTSV(entries)
	lines := strings.Split(strings.TrimSuffix(string(tsv), "\n"), "\n")
	if len(lines) != 3 || lines[0] != strings.Join(bgTSVHeader, "\t") {
		t.Fatalf("TSV %q", tsv)
	}
	for i, e := range entries {
		f := strings.Split(lines[i+1], "\t")
		if len(f) != len(bgTSVHeader) || f[0] != e.Key || f[2] != e.State || f[15] != strconv.FormatBool(e.Hidden) {
			t.Fatalf("第 %d 列 %q", i, f)
		}
		n := func(k int) int { v, _ := strconv.Atoi(f[k]); return v }
		for k, want := range [][]byte{e.Before, e.After, e.Ref} {
			off, ln := n(16+2*k), n(17+2*k)
			if want == nil {
				if off != 0 || ln != 0 {
					t.Fatalf("第 %d 列 nil 的列記 %d、%d", i, off, ln)
				}
				continue
			}
			if !bytes.Equal(rows[off:off+ln], want) {
				t.Fatalf("第 %d 列第 %d 段 %v，要 %v", i, k, rows[off:off+ln], want)
			}
		}
	}
}
