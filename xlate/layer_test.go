package xlate

import (
	"bytes"
	"testing"
)

// spec 202 §3 第 1 項，搬自 psychic_war_cht apps/psychicwar/overlay：捲動。
func TestScroll(t *testing.T) {
	var dropped []string
	l := &Layer{OnDrop: func(s *Stamp, why string) { dropped = append(dropped, s.Key+":"+why) }}
	in := &Stamp{Key: "in", X: 8, Y: 112, Cells: 16, CellW: 8, CellH: 8}
	top := &Stamp{Key: "top", X: 8, Y: 88, Cells: 16, CellW: 8, CellH: 8}
	out := &Stamp{Key: "out", X: 200, Y: 112, Cells: 4, CellW: 8, CellH: 8}
	wide := &Stamp{Key: "wide", X: 100, Y: 96, Cells: 20, CellW: 6, CellH: 7} // 左上角在框內、右緣超出框：不搬
	l.Stamps = []*Stamp{in, top, out, wide}
	l.Scroll(8, 88, 136, 120, -2)
	if in.Y != 110 || out.Y != 112 || wide.Y != 96 || len(l.Stamps) != 3 || len(dropped) != 1 || dropped[0] != "top:scroll" {
		t.Errorf("in.Y=%d out.Y=%d 剩 %d 筆 移除 %v", in.Y, out.Y, len(l.Stamps), dropped)
	}
}

// spec 202 §3 第 1 項：3 幀失效與恢復計數。
func TestInvalidateAfterThreeFrames(t *testing.T) {
	idx := make([]uint8, DefaultScreenW*DefaultScreenH)
	rgb := make([]uint8, 3*DefaultScreenW*DefaultScreenH)
	idx[112*DefaultScreenW+9] = 11
	l := &Layer{}
	s := &Stamp{Key: "k", X: 8, Y: 112, Cells: 2, CellW: 8, CellH: 8, State: Pending}
	l.Stamps = []*Stamp{s}
	l.Frame(idx, rgb)
	if s.State != Shown {
		t.Fatal("定色後應該顯示")
	}
	changed := append([]uint8(nil), idx...)
	changed[113*DefaultScreenW+10] = 5
	l.Frame(changed, rgb)
	l.Frame(changed, rgb)
	if len(l.Stamps) != 1 {
		t.Fatal("連續 2 幀不同就被移除")
	}
	l.Frame(idx, rgb) // 恢復一次，重新計數
	l.Frame(changed, rgb)
	l.Frame(changed, rgb)
	if len(l.Stamps) != 1 {
		t.Fatal("恢復後又 2 幀不同就被移除")
	}
	l.Frame(changed, rgb)
	if len(l.Stamps) != 0 {
		t.Fatal("連續 3 幀不同應該移除")
	}
}

// spec 202 §3 第 1 項：定色。
func TestColors(t *testing.T) {
	cell := make([]uint8, 64)
	for i := 0; i < 14; i++ {
		cell[i*3] = 11
	}
	if bg, fg := Colors(cell); bg != 0 || fg != 11 {
		t.Errorf("背景 %d 前景 %d，要 0、11", bg, fg)
	}
	if bg, fg := Colors(make([]uint8, 64)); bg != 0 || fg != 0 {
		t.Errorf("只有一種色號：背景 %d 前景 %d", bg, fg)
	}
}

// spec 202 §3 第 1 項：重疊移除。
func TestAddReplacesOverlap(t *testing.T) {
	l := &Layer{}
	l.Add(&Stamp{Key: "a", X: 8, Y: 112, Cells: 16, CellW: 8, CellH: 8})
	l.Add(&Stamp{Key: "b", X: 8, Y: 104, Cells: 16, CellW: 8, CellH: 8})
	l.Add(&Stamp{Key: "c", X: 16, Y: 112, Cells: 3, CellW: 8, CellH: 8})
	if len(l.Stamps) != 2 || l.Stamps[0].Key != "b" || l.Stamps[1].Key != "c" {
		t.Errorf("剩 %d 筆", len(l.Stamps))
	}
}

// spec 202 §3 第 1 項：畫字與缺字回呼（字型換算成通用 24×24，等同原本 8 像素字格 × 放大 3 倍）。
func TestDrawGlyph(t *testing.T) {
	g := make([]byte, 3*24) // 24 列 × 3 bytes/列
	g[0] = 0x80             // 左上角一點
	font := &Font{W: 24, H: 24, Glyphs: map[rune][]byte{'一': g}}
	l := &Layer{Stamps: []*Stamp{{
		X: 8, Y: 112, Cells: 2, CellW: 8, CellH: 8,
		Font: font, Text: []rune("一x"), State: Shown,
		FG: [3]uint8{1, 2, 3}, BG: [3]uint8{9, 9, 9},
	}}}
	dst := make([]uint8, 4*960*600)
	var miss []rune
	l.Draw(dst, 3, func(r rune) { miss = append(miss, r) })
	at := func(x, y int) [3]uint8 { i := 4 * (y*960 + x); return [3]uint8{dst[i], dst[i+1], dst[i+2]} }
	if at(24, 336) != [3]uint8{1, 2, 3} || at(25, 336) != [3]uint8{9, 9, 9} || at(24+47, 336+23) != [3]uint8{9, 9, 9} {
		t.Errorf("字模點 %v、旁邊 %v、角落 %v", at(24, 336), at(25, 336), at(71, 359))
	}
	if len(miss) != 1 || miss[0] != 'x' {
		t.Errorf("缺字 %q", string(miss))
	}
}

// spec 202 §3 第 2 項：16×15 字型、6×7 字格、scale 3、GlyphScale 1、偏移 (1,3)：
// 字模左上點畫在格左上 ＋(1,3)。
func TestDrawGlyphOffset(t *testing.T) {
	rowBytes := 2 // (16+7)/8
	g := make([]byte, 15*rowBytes)
	g[0] = 0x80 // 左上角一點 (gx=0, gy=0)
	font := &Font{W: 16, H: 15, Glyphs: map[rune][]byte{'字': g}}
	s := &Stamp{
		X: 0, Y: 0, Cells: 1, CellW: 6, CellH: 7,
		Font: font, GlyphX: 1, GlyphY: 3, GlyphScale: 1,
		Text: []rune("字"), State: Shown,
		FG: [3]uint8{1, 2, 3}, BG: [3]uint8{9, 9, 9},
	}
	l := &Layer{W: 6, H: 7, Stamps: []*Stamp{s}} // 畫面尺寸設成剛好一格，Draw 內部的列寬才對得上 at() 假設的 W
	W := 6 * 3
	H := 7 * 3
	dst := make([]uint8, 4*W*H)
	l.Draw(dst, 3, nil)
	at := func(x, y int) [3]uint8 { i := 4 * (y*W + x); return [3]uint8{dst[i], dst[i+1], dst[i+2]} }
	if at(1, 3) != s.FG {
		t.Errorf("字模左上點該在格左上+(1,3)：%v", at(1, 3))
	}
}

// spec 202 §3 第 2 項：一格右緣之外不畫。用寬到能容納「沒 clip 也不會被
// set() 的陣列邊界擋下」的畫布，確定 clip 是規則本身擋下，不是邊界檢查的副作用。
func TestDrawGlyphClipsBeyondCell(t *testing.T) {
	g := []byte{0x01} // W=8 字型，最右一欄 (gx=7) 有一點
	font := &Font{W: 8, H: 1, Glyphs: map[rune][]byte{'字': g}}
	s := &Stamp{
		X: 0, Y: 0, Cells: 1, CellW: 2, CellH: 1, // 格寬(放大後) 2*3=6，字寬 8*1=8，超出格緣
		Font: font, GlyphX: 0, GlyphY: 0, GlyphScale: 1,
		Text: []rune("字"), State: Shown,
		FG: [3]uint8{1, 2, 3}, BG: [3]uint8{9, 9, 9},
	}
	l := &Layer{W: 8, H: 1, Stamps: []*Stamp{s}} // 畫面寬 8（4 格），遠比一格寬（2）大，且與 at() 假設的 W 對齊
	W := 8 * 3
	dst := make([]uint8, 4*W*3)
	sentinel := [3]uint8{0xAA, 0xAA, 0xAA}
	for i := 0; i < len(dst); i += 4 {
		dst[i], dst[i+1], dst[i+2], dst[i+3] = sentinel[0], sentinel[1], sentinel[2], 0xFF
	}
	l.Draw(dst, 3, nil)
	at := func(x, y int) [3]uint8 { i := 4 * (y*W + x); return [3]uint8{dst[i], dst[i+1], dst[i+2]} }
	if at(7, 0) != sentinel {
		t.Errorf("超出格緣（x=7 ≥ 格寬 6）的點不該被畫：%v", at(7, 0))
	}
}

// spec 202 §3 第 4 項：Snapshot／Restore 往返後 Draw 出的 RGBA 逐位元組相同。
func TestSnapshotRestoreRoundTrip(t *testing.T) {
	g := make([]byte, 3*24)
	g[0] = 0x80
	font := &Font{W: 24, H: 24, Name: "cjk24", Glyphs: map[rune][]byte{'一': g}}
	l := &Layer{W: 320, H: 200, Stamps: []*Stamp{{
		Key: "k1", X: 8, Y: 112, Cells: 2, CellW: 8, CellH: 8,
		Font: font, GlyphX: 1, GlyphY: 2, GlyphScale: 2,
		Text: []rune("一x"), State: Shown,
		FG: [3]uint8{1, 2, 3}, BG: [3]uint8{9, 9, 9},
	}}}
	before := make([]uint8, 4*960*600)
	l.Draw(before, 3, nil)

	data, err := l.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	restored := &Layer{}
	if err := restored.Restore(data, map[string]*Font{"cjk24": font}); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored.W != 320 || restored.H != 200 {
		t.Errorf("W/H 沒還原：%d %d", restored.W, restored.H)
	}
	after := make([]uint8, 4*960*600)
	restored.Draw(after, 3, nil)

	if !bytes.Equal(before, after) {
		t.Error("Snapshot/Restore 往返後 Draw 結果不同")
	}
}

// Restore 找不到字型名稱時回錯，不是靜靜地把 Stamp 的 Font 留成 nil。
func TestRestoreMissingFont(t *testing.T) {
	font := &Font{Name: "missing-in-restore"}
	l := &Layer{Stamps: []*Stamp{{Key: "k", Font: font, State: Shown}}}
	data, err := l.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if err := (&Layer{}).Restore(data, map[string]*Font{}); err == nil {
		t.Error("字型名對不上應該回錯")
	}
}

// Restore 之後 OnDrop 維持呼叫端原本接的那個，不被 JSON 清掉。
func TestRestoreKeepsOnDrop(t *testing.T) {
	called := false
	l := &Layer{OnDrop: func(s *Stamp, why string) { called = true }}
	data, err := l.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if err := l.Restore(data, nil); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	l.Add(&Stamp{Key: "a", Cells: 1, CellW: 1, CellH: 1})
	l.Add(&Stamp{Key: "b", Cells: 1, CellW: 1, CellH: 1}) // 與 a 重疊（都在原點），觸發 drop
	if !called {
		t.Error("Restore 之後 OnDrop 應該還在")
	}
}

// spec 202 §3 第 6 項：透明格不填背景、不畫字、不列入定色與指紋。
func TestTransparentCells(t *testing.T) {
	idx := make([]uint8, 320*200)
	rgb := make([]uint8, 3*320*200)
	for i := range rgb {
		rgb[i] = 7
	}
	// 第 0 格有一點前景色 11；第 1 格（透明）有很多色號 5
	idx[112*320+9] = 11
	for y := 112; y < 120; y++ {
		for x := 16; x < 24; x++ {
			idx[y*320+x] = 5
		}
	}
	font := &Font{W: 24, H: 24, Glyphs: map[rune][]byte{'甲': make([]byte, 72)}}
	font.Glyphs['甲'][0] = 0x80
	s := &Stamp{X: 8, Y: 112, Cells: 2, CellW: 8, CellH: 8, Font: font, Text: []rune("甲甲"), Transparent: []bool{false, true}, State: Pending}
	l := &Layer{Stamps: []*Stamp{s}}
	l.Frame(idx, rgb)
	if s.State != Shown {
		t.Fatal("定色後應該顯示")
	}
	// 透明格色號 5 最多，但不列入定色：背景應該是 0 對應的 RGB、前景是 11 對應的 RGB（全部 7）
	changed := append([]uint8(nil), idx...)
	changed[115*320+18] = 9 // 只改透明格
	for i := 0; i < 5; i++ {
		l.Frame(changed, rgb)
	}
	if len(l.Stamps) != 1 {
		t.Fatal("透明格變動不該讓疊字失效")
	}
	// 反向對照：改非透明格，3 幀後失效
	changed[115*320+10] = 9
	for i := 0; i < 3; i++ {
		l.Frame(changed, rgb)
	}
	if len(l.Stamps) != 0 {
		t.Fatal("非透明格變動應該失效")
	}
	s.State = Shown
	s.BG, s.FG = [3]uint8{1, 1, 1}, [3]uint8{2, 2, 2}
	l.Stamps = []*Stamp{s}
	dst := make([]uint8, 4*960*600)
	l.Draw(dst, 3, nil)
	at := func(x, y int) [4]uint8 { i := 4 * (y*960 + x); return [4]uint8{dst[i], dst[i+1], dst[i+2], dst[i+3]} }
	if at(24, 336) != [4]uint8{2, 2, 2, 255} || at(30, 340) != [4]uint8{1, 1, 1, 255} {
		t.Errorf("非透明格：字點 %v、背景 %v", at(24, 336), at(30, 340))
	}
	if at(48, 336) != [4]uint8{} || at(60, 350) != [4]uint8{} {
		t.Errorf("透明格應該完全不畫：%v %v", at(48, 336), at(60, 350))
	}
}

// spec 202 §3 第 7 項：凍結期間指紋不同不累計失效。
func TestFrozenSkipsInvalidation(t *testing.T) {
	idx := make([]uint8, 320*200)
	rgb := make([]uint8, 3*320*200)
	idx[112*320+9] = 11
	for _, freeze := range []bool{true, false} {
		s := &Stamp{X: 8, Y: 112, Cells: 2, CellW: 8, CellH: 8, State: Pending}
		frozen := false
		l := &Layer{Stamps: []*Stamp{s}, Frozen: func(*Stamp) bool { return frozen }}
		l.Frame(idx, rgb)
		changed := append([]uint8(nil), idx...)
		changed[113*320+10] = 5
		frozen = freeze
		for i := 0; i < 5; i++ {
			l.Frame(changed, rgb)
		}
		frozen = false
		if freeze && len(l.Stamps) != 1 {
			t.Error("凍結期間不該失效")
		}
		if !freeze && len(l.Stamps) != 0 {
			t.Error("反向對照：不凍結時應該失效")
		}
		if freeze {
			l.Frame(idx, rgb) // 解除後內容恢復
			if len(l.Stamps) != 1 || s.State != Shown {
				t.Error("解除凍結、內容相同時應該繼續顯示")
			}
		}
	}
}
