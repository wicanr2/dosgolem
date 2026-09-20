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
	// 兩格都要有墨跡（都是錨定格），否則第 0 格失效時整筆就會被移除（spec 202 §2.3）
	idx[112*DefaultScreenW+9] = 11
	idx[112*DefaultScreenW+17] = 11
	l := &Layer{}
	s := &Stamp{Key: "k", X: 8, Y: 112, Cells: 2, CellW: 8, CellH: 8, State: Pending}
	l.Stamps = []*Stamp{s}
	l.Frame(idx, rgb)
	if s.State != Shown {
		t.Fatal("定色後應該顯示")
	}
	// 只有第 0 格變：3 幀後那一格變透明，整筆還在（spec 202 §2.3）
	one := append([]uint8(nil), idx...)
	one[113*DefaultScreenW+10] = 5
	l.Frame(one, rgb)
	l.Frame(one, rgb)
	if s.transparent(0) {
		t.Fatal("2 幀就把格子遮掉了")
	}
	l.Frame(one, rgb)
	if !s.transparent(0) || s.transparent(1) || len(l.Stamps) != 1 {
		t.Fatalf("第 0 格應該遮掉、第 1 格留著：%v 剩 %d 筆", s.Transparent, len(l.Stamps))
	}
	// 反向對照：恢復原狀時不再累計（拿一筆新的重來，改回原內容後 5 幀都不遮）
	s2 := &Stamp{Key: "k2", X: 8, Y: 112, Cells: 2, CellW: 8, CellH: 8, State: Pending}
	l.Stamps = []*Stamp{s2}
	l.Frame(idx, rgb)
	l.Frame(one, rgb)
	l.Frame(one, rgb)
	l.Frame(idx, rgb) // 恢復
	for i := 0; i < 2; i++ {
		l.Frame(one, rgb)
	}
	if s2.transparent(0) {
		t.Error("恢復之後應該重新計數")
	}
	// 每一格都變 → 整筆移除
	all := append([]uint8(nil), idx...)
	for y := 112; y < 120; y++ {
		for x := 8; x < 24; x++ {
			all[y*DefaultScreenW+x] = 3
		}
	}
	for i := 0; i < 3; i++ {
		l.Frame(all, rgb)
	}
	if len(l.Stamps) != 0 {
		t.Fatal("每一格都變應該移除")
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
// spec 202 §2.3：整個被蓋住才移除；只蓋到一部分時被蓋的格子變透明，其餘照常顯示。
func TestAddCoversOrMasks(t *testing.T) {
	var dropped []string
	l := &Layer{OnDrop: func(s *Stamp, why string) { dropped = append(dropped, s.Key+":"+why) }}
	a := &Stamp{Key: "a", X: 8, Y: 112, Cells: 16, CellW: 8, CellH: 8, State: Shown}
	l.Add(a)
	l.Add(&Stamp{Key: "b", X: 8, Y: 104, Cells: 16, CellW: 8, CellH: 8}) // 不同列，不重疊
	l.Add(&Stamp{Key: "c", X: 16, Y: 112, Cells: 3, CellW: 8, CellH: 8}) // 蓋住 a 的第 1–3 格
	if len(l.Stamps) != 3 {
		t.Fatalf("部分重疊不該移除，剩 %d 筆", len(l.Stamps))
	}
	for i := 0; i < 16; i++ {
		want := i >= 1 && i <= 3
		if a.transparent(i) != want {
			t.Errorf("第 %d 格透明 %v，要 %v", i, a.transparent(i), want)
		}
	}
	if a.State != Pending {
		t.Error("剩下的格子要重新定色")
	}
	l.Add(&Stamp{Key: "d", X: 8, Y: 112, Cells: 16, CellW: 8, CellH: 8}) // 整個蓋住 a 與 c
	if len(l.Stamps) != 2 || l.Stamps[0].Key != "b" || l.Stamps[1].Key != "d" {
		t.Errorf("整個蓋住要移除：剩 %d 筆 %v", len(l.Stamps), dropped)
	}
	if len(dropped) != 2 || dropped[0] != "a:overlap" || dropped[1] != "c:overlap" {
		t.Errorf("移除紀錄不對：%v", dropped)
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
	// 反向對照：改非透明格，3 幀後那一格也被遮掉；兩格都透明就整筆移除
	changed[115*320+10] = 9
	for i := 0; i < 3; i++ {
		l.Frame(changed, rgb)
	}
	if len(l.Stamps) != 0 {
		t.Fatal("唯一的非透明格變動後應該整筆移除")
	}
	s = &Stamp{X: 8, Y: 112, Cells: 2, CellW: 8, CellH: 8, Font: font, Text: []rune("甲甲"),
		Transparent: []bool{false, true}, State: Shown}
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
		if freeze && s.transparent(0) {
			t.Error("凍結期間不該把格子遮掉")
		}
		if !freeze && !s.transparent(0) {
			t.Error("反向對照：不凍結時那一格應該被遮掉")
		}
		if freeze {
			l.Frame(idx, rgb) // 解除後內容恢復
			if len(l.Stamps) != 1 || s.State != Shown {
				t.Error("解除凍結、內容相同時應該繼續顯示")
			}
		}
	}
}

// 舊快照（misses 是單一數值）要能讀回來：那幾筆重新定色，不整個失敗。
func TestRestoreOldSnapshot(t *testing.T) {
	old := `{"w":320,"h":200,"stamps":[{"key":"k","x":8,"y":112,"cells":2,"cell_w":8,"cell_h":8,` +
		`"glyph_x":0,"glyph_y":0,"glyph_scale":1,"text":"甲甲","state":2,"fg":[1,1,1],"bg":[0,0,0],` +
		`"hash":123,"misses":2}]}`
	l := &Layer{}
	if err := l.Restore([]byte(old), nil); err != nil {
		t.Fatalf("舊快照應該讀得回來：%v", err)
	}
	if len(l.Stamps) != 1 {
		t.Fatalf("剩 %d 筆", len(l.Stamps))
	}
	idx := make([]uint8, DefaultScreenW*DefaultScreenH)
	rgb := make([]uint8, 3*DefaultScreenW*DefaultScreenH)
	l.Frame(idx, rgb) // 這一幀發現指紋陣列長度不對，改回 Pending
	l.Frame(idx, rgb)
	if l.Stamps[0].State != Shown || len(l.Stamps[0].hashes) != 2 {
		t.Errorf("應該重新定色：state=%v hashes=%v", l.Stamps[0].State, l.Stamps[0].hashes)
	}
}

// spec 202 §2.3 錨定格：原版把整段文字清掉之後，壓在純色背景上的格子指紋不會變，
// 不整筆移除就會在畫面上留下孤字（實例：道具頁翻頁後留著「上」「經」）。
func TestAnchorsGoneDropsWholeStamp(t *testing.T) {
	const W, H = DefaultScreenW, DefaultScreenH
	idx := make([]uint8, W*H)
	rgb := make([]uint8, 3*W*H)
	// 整塊底色 1（藍），原文墨跡只在第 1、2 格
	for y := 40; y < 48; y++ {
		for x := 16; x < 40; x++ {
			idx[y*W+x] = 1
		}
	}
	idx[42*W+25], idx[42*W+33] = 15, 15
	var dropped []string
	l := &Layer{OnDrop: func(s *Stamp, why string) { dropped = append(dropped, s.Key+":"+why) }}
	s := &Stamp{Key: "上限", X: 16, Y: 40, Cells: 3, CellW: 8, CellH: 8, State: Pending}
	l.Stamps = []*Stamp{s}
	l.Frame(idx, rgb)
	if !s.anchors[1] || !s.anchors[2] || s.anchors[0] {
		t.Fatalf("錨定格應該是第 1、2 格：%v", s.anchors)
	}
	// 原版把整塊清成純藍：第 0 格的指紋不變（本來就是純藍），錨定格都變了
	clean := append([]uint8(nil), idx...)
	clean[42*W+25], clean[42*W+33] = 1, 1
	for i := 0; i < 3; i++ {
		l.Frame(clean, rgb)
	}
	if len(l.Stamps) != 0 {
		t.Fatalf("錨定格全部失效應該整筆移除，卻剩 %d 筆（transparent=%v）", len(l.Stamps), s.Transparent)
	}
	if len(dropped) != 1 || dropped[0] != "上限:anchors" {
		t.Fatalf("移除原因應該是 anchors：%v", dropped)
	}
	// 反向對照：只清掉一個錨定格時整筆留著（原版在旁邊開框只蓋住一部分）
	s2 := &Stamp{Key: "半", X: 16, Y: 40, Cells: 3, CellW: 8, CellH: 8, State: Pending}
	l.Stamps = []*Stamp{s2}
	l.Frame(idx, rgb)
	half := append([]uint8(nil), idx...)
	half[42*W+33] = 1
	for i := 0; i < 4; i++ {
		l.Frame(half, rgb)
	}
	if len(l.Stamps) != 1 || !s2.transparent(2) || s2.transparent(1) {
		t.Fatalf("只有一個錨定格失效時應該只遮那一格：剩 %d 筆 transparent=%v", len(l.Stamps), s2.Transparent)
	}
}

// 沒有任何錨定格（整塊純色）的疊字照舊逐格判斷，不會因為「沒有錨定格」就被移除。
func TestNoAnchorsKeepsPerCellRule(t *testing.T) {
	const W, H = DefaultScreenW, DefaultScreenH
	idx := make([]uint8, W*H)
	rgb := make([]uint8, 3*W*H)
	l := &Layer{}
	s := &Stamp{Key: "純色", X: 8, Y: 8, Cells: 2, CellW: 8, CellH: 8, State: Pending}
	l.Stamps = []*Stamp{s}
	l.Frame(idx, rgb)
	for i := 0; i < 5; i++ {
		l.Frame(idx, rgb)
	}
	if len(l.Stamps) != 1 {
		t.Fatal("畫面沒變就不該移除")
	}
}

// spec 202 §3 第 8 項：SwapColors——字比底密的區塊，「最多的當背景」會反過來。
func TestSwapColors(t *testing.T) {
	const w, h = 32, 8
	indexed := make([]uint8, w*h)
	rgb := make([]uint8, 3*w*h)
	// 一塊 16×8：色號 15 佔多數（字比底密），色號 0 少數。
	for y := 0; y < h; y++ {
		for x := 0; x < 16; x++ {
			c := uint8(15)
			if x%4 == 0 {
				c = 0
			}
			indexed[y*w+x] = c
			i := 3 * (y*w + x)
			rgb[i], rgb[i+1], rgb[i+2] = c*17, c*17, c*17
		}
	}
	mk := func(swap bool) *Stamp {
		return &Stamp{Key: "k", X: 0, Y: 0, Cells: 2, CellW: 8, CellH: 8,
			Text: []rune("甲乙"), State: Pending, SwapColors: swap}
	}
	plain, swapped := mk(false), mk(true)
	l := &Layer{W: w, H: h}
	l.Add(plain)
	l.Frame(indexed, rgb)
	if plain.BG != [3]uint8{255, 255, 255} || plain.FG != [3]uint8{0, 0, 0} {
		t.Fatalf("不互換：背景 %v 前景 %v，要白底黑字", plain.BG, plain.FG)
	}
	l2 := &Layer{W: w, H: h}
	l2.Add(swapped)
	l2.Frame(indexed, rgb)
	if swapped.BG != plain.FG || swapped.FG != plain.BG {
		t.Fatalf("互換：背景 %v 前景 %v，要與不互換的對調", swapped.BG, swapped.FG)
	}
}

// spec 202 §3 第 4 項：SwapColors 要跟著快照走（還原後重新定色時仍然互換）。
func TestSwapColorsSurvivesSnapshot(t *testing.T) {
	l := &Layer{W: 32, H: 8}
	l.Add(&Stamp{Key: "k", Cells: 2, CellW: 8, CellH: 8, Text: []rune("甲乙"), SwapColors: true})
	b, err := l.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var l2 Layer
	if err := l2.Restore(b, nil); err != nil {
		t.Fatal(err)
	}
	if len(l2.Stamps) != 1 || !l2.Stamps[0].SwapColors {
		t.Fatalf("還原後 SwapColors 沒了：%+v", l2.Stamps)
	}
}
