package strikecommander

import (
	"testing"
)

// strike_commander 規格 003 §4.1：計分板的呼叫端、前後列與核對、stamp 規劃、字型檢查與繪製。
// 沿用 observer_test.go 的合成 harness；計分板程式放在 overlay 段 tBoardSeg。

const tBoardSeg = 0xED80

// realSiteBytes 是計分板程式呼叫輸出函式的六個呼叫點，返回位址之前的真實位元組
// （strike_commander workplace/i013/kb2/ed80.bin，ED80:0000 起；段字 83 3B 是本次執行的 base + 3942h）。
var realSiteBytes = map[uint16][]int{
	0x1369: {0x68, 0x68, 0x10, 0x8D, 0x86, 0xFE, 0xFD, 0x50, 0x9A, 0xFF, 0x05, 0x83, 0x3B},                               // GROUND（ED80:135C 起）
	0x1385: {0x68, 0x6F, 0x10, 0x8D, 0x86, 0xFE, 0xFD, 0x50, 0x9A, 0xFF, 0x05, 0x83, 0x3B},                               // AIR（1378 起）
	0x1407: {0x6A, 0x53, 0x68, 0x76, 0x10, 0x8D, 0x86, 0xFE, 0xFD, 0x50, 0x9A, 0xFF, 0x05, 0x83, 0x3B},                   // 呼號（13F8 起）
	0x1499: {0x68, 0x7D, 0x10, 0x8D, 0x86, 0xFE, 0xFD, 0x50, 0x9A, 0xFF, 0x05, 0x83, 0x3B},                               // 地面擊落數（148C 起）
	0x14B5: {0x68, 0x84, 0x10, 0x8D, 0x86, 0xFE, 0xFD, 0x50, 0x9A, 0xFF, 0x05, 0x83, 0x3B},                               // 空中擊落數（14A8 起）
	0x14E4: {0x68, 0xAF, 0x00, 0x6A, 0x02, 0x68, 0x8B, 0x10, 0x8D, 0x86, 0xE4, 0xFD, 0x50, 0x9A, 0xFF, 0x05, 0x83, 0x3B}, // 底部標籤（14D2 起）
}

// installBoard 把六個呼叫點的真實位元組寫到 seg 段。
func (h *harness) installBoard(seg uint16) {
	for ret, b := range realSiteBytes {
		h.f.w8(seg, ret-uint16(len(b)), b...)
	}
}

// boardPuts 模擬計分板經輸出函式的一筆 puts：輸出函式返回到 seg:ret；入口游標 (x0, yEntry)，
// 第一次 putc 時游標 (x0, y)，返回時 x1。during 在 putc 之後、puts 返回之前呼叫（畫字）。
func (h *harness) boardPuts(s string, seg, ret uint16, x0, x1, y, yEntry int, during func()) {
	h.setChain(ipFmtRet, tBase+segText, ret, seg)
	h.setString(s)
	h.setCursor(uint16(x0), uint16(yEntry))
	h.putsEntry()
	h.setCursor(uint16(x0), uint16(y))
	h.putc(tObj)
	if during != nil {
		during()
	}
	h.setCursor(uint16(x1), uint16(y))
	h.putsReturn()
}

// newBoardHarness 建 harness，計分板程式在 tBoardSeg，輸出物件的字型是 FB12，緩衝的白板帶（y 40 到 59）
// 填白板底色 164。
func newBoardHarness(t *testing.T) *harness {
	h := newHarness(t)
	h.installBoard(tBoardSeg)
	h.f.w16(tDS, tObj+objFont, boardFont)
	for y := 40; y < 60; y++ {
		for x := 0; x < 320; x++ {
			h.bufPx(x, y, 164)
		}
	}
	return h
}

func (h *harness) bufPx(x, y int, c byte) { h.f.w8(tBufSeg, uint16(y*320+4+x), int(c)) }

// --- 第 1 項：呼叫端 ---

func TestBoardCaller(t *testing.T) {
	accept := []struct {
		ret  uint16
		key  string
		kind Kind
	}{
		{0x1369, "GROUND", KindBoard},
		{0x1385, "AIR", KindBoard},
		{0x14E4, "Return to office", KindLabel},
	}
	for _, c := range accept {
		t.Run("接受 "+c.key, func(t *testing.T) {
			h := newBoardHarness(t)
			h.boardPuts(c.key, tBoardSeg, c.ret, 177, 250, 45, 45, nil)
			p := h.o.Pending()
			if len(p) != 1 || p[0].Key != c.key || p[0].Kind != c.kind {
				t.Fatalf("待提交 %+v，要 %q 種類 %s", p, c.key, c.kind)
			}
			h.blit()
			if len(h.commits) != 1 || h.commits[0].Accepted != 1 || h.commits[0].Ignored != 0 {
				t.Fatalf("提交 %+v，要接受 1、忽略 0", h.commits)
			}
		})
		t.Run("換段 "+c.key, func(t *testing.T) {
			// 段 s 不寫死：程式換到另一段、位元組照搬，仍接受（ED80 段沒有程式）。
			h := newHarness(t)
			h.installBoard(0xD400)
			h.boardPuts(c.key, 0xD400, c.ret, 177, 250, 45, 45, nil)
			if p := h.o.Pending(); len(p) != 1 || p[0].Kind != c.kind {
				t.Fatalf("換段後待提交 %+v，要接受、種類 %s", p, c.kind)
			}
		})
	}
	reject := []struct {
		name string
		ret  uint16
		mod  func(h *harness)
		bp2  uint16
		bp4  uint16
	}{
		{name: "呼號 1407", ret: 0x1407},
		{name: "地面擊落數 1499", ret: 0x1499},
		{name: "空中擊落數 14B5", ret: 0x14B5},
		// 三列各改一個 push 常數。
		{name: "GROUND 格式 1068h 改 1069h", ret: 0x1369, mod: func(h *harness) { h.f.w8(tBoardSeg, 0x1369-13+1, 0x69) }},
		{name: "AIR 格式 106Fh 改 1070h", ret: 0x1385, mod: func(h *harness) { h.f.w8(tBoardSeg, 0x1385-13+1, 0x70) }},
		{name: "底部標籤 y AFh 改 B0h", ret: 0x14E4, mod: func(h *harness) { h.f.w8(tBoardSeg, 0x14E4-18+1, 0xB0) }},
		// 段字不等於 base + 3942h。
		{name: "GROUND 段字 3B84", ret: 0x1369, mod: func(h *harness) { h.f.w8(tBoardSeg, 0x1369-2, 0x84, 0x3B) }},
		{name: "底部標籤段字 3C83", ret: 0x14E4, mod: func(h *harness) { h.f.w8(tBoardSeg, 0x14E4-2, 0x83, 0x3C) }},
		// [BP+2]、[BP+4] 不符（計分板的 Q 鏈照常）。
		{name: "[BP+2] 不是 061E", ret: 0x1369, bp2: 0x0620},
		{name: "[BP+4] 不是 base+3942", ret: 0x1369, bp4: tBase + segText + 1},
	}
	for _, c := range reject {
		t.Run("拒絕 "+c.name, func(t *testing.T) {
			h := newBoardHarness(t)
			if c.mod != nil {
				c.mod(h)
			}
			h.setString("X")
			h.setCursor(177, 45)
			p2, p4 := uint16(ipFmtRet), uint16(tBase+segText)
			if c.bp2 != 0 {
				p2 = c.bp2
			}
			if c.bp4 != 0 {
				p4 = c.bp4
			}
			h.setChain(p2, p4, c.ret, tBoardSeg)
			h.putsEntry()
			h.putc(tObj)
			h.setCursor(250, 45)
			h.putsReturn()
			if p := h.o.Pending(); len(p) != 0 {
				t.Fatalf("%s 被接受：%+v", c.name, p)
			}
			h.blit()
			if len(h.commits) != 1 || h.commits[0].Accepted != 0 || h.commits[0].Ignored != 1 {
				t.Fatalf("提交 %+v，要接受 0、忽略 1", h.commits)
			}
		})
	}
	t.Run("noBoardCaller", func(t *testing.T) {
		h := newBoardHarness(t)
		h.o.noBoardCaller = true
		for _, c := range accept {
			h.boardPuts(c.key, tBoardSeg, c.ret, 177, 250, 45, 45, nil)
		}
		if p := h.o.Pending(); len(p) != 0 {
			t.Fatalf("noBoardCaller 時仍接受 %q", keys(p))
		}
		// 082A 照常接受。
		h.f.w16(tDS, tObj+objFont, measuredFont)
		h.setChain(ipFmtRet, tBase+segText, ipLabelRet, tBase+segLabel)
		h.label("Talk to Gwen", 114, 204)
		h.blit()
		if len(h.commits) != 1 || h.commits[0].Accepted != 1 || h.commits[0].Ignored != 3 ||
			keys(h.commits[0].Labels) != "Talk to Gwen" || h.commits[0].Labels[0].Kind != KindLabel {
			t.Fatalf("提交 %+v，要只接受 Talk to Gwen（label）、忽略 3", h.commits)
		}
	})
}

// --- 第 2 項：前後列與核對 ---

// GROUND 的幾何（RE 014）：x0 177、x1 250、y 45，墨跡框 [177, 250) × [46, 53)。
const (
	gX0, gX1, gY = 177, 250, 45
)

// paintGround 只在框內畫字：上緣列 46 整列、左緣行 177 與右緣行 249 的 [46, 53)。
func paintGround(h *harness) {
	for x := gX0; x < gX1; x++ {
		h.bufPx(x, gY+1, 10)
	}
	for y := gY + 1; y < gY+8; y++ {
		h.bufPx(gX0, y, 88)
		h.bufPx(gX1-1, y, 32)
	}
}

// runBoard 跑一筆 GROUND（入口 y 為 yEntry），during 在 puts 期間畫，after 在 puts 返回後、blit 前畫。
// 回 puts 返回後的待提交紀錄與 blit 提交的核對結果。
func runBoard(t *testing.T, yEntry int, prep, during, after func(h *harness), cursorY int) (Label, InkResult) {
	t.Helper()
	h := newBoardHarness(t)
	h.o.InkCheck = true
	h.o.CursorY = cursorY
	if prep != nil {
		prep(h)
	}
	h.boardPuts("GROUND", tBoardSeg, 0x1369, gX0, gX1, gY, yEntry, func() {
		if during != nil {
			during(h)
		}
	})
	p := h.o.Pending()
	if len(p) != 1 {
		t.Fatalf("待提交 %d 筆，要 1", len(p))
	}
	if after != nil {
		after(h)
	}
	h.blit()
	if len(h.commits) != 1 || len(h.commits[0].Ink) != 1 || len(h.commits[0].Labels) != 1 {
		t.Fatalf("提交 %+v，要一筆標籤與一筆核對", h.commits)
	}
	return p[0], h.commits[0].Ink[0]
}

func TestBoardRows(t *testing.T) {
	// 入口前：第 45 列（y）、第 53 列（y+8）、第 54 列（y+9）各放一個標記，(203, 50) 是白板細線。
	prep := func(h *harness) {
		h.bufPx(10, gY, 78)
		h.bufPx(10, gY+8, 77)
		h.bufPx(10, gY+9, 79)
		h.bufPx(203, 50, 154)
	}
	l, r := runBoard(t, gY, prep, paintGround, nil, -1)
	if l.Kind != KindBoard || len(l.Before) != 8*320 || len(l.After) != 8*320 {
		t.Fatalf("種類 %s、前列 %d bytes、後列 %d bytes，要 board、各 2560", l.Kind, len(l.Before), len(l.After))
	}
	at := func(b []byte, x, y int) byte { return b[(y-gY-1)*320+x] }
	if at(l.Before, 10, gY+1) != 164 || at(l.Before, 10, gY+8) != 77 || at(l.After, 10, gY+8) != 77 {
		t.Errorf("列範圍不是 [y+1, y+9)：前列第 0 列 %d（要 164）、第 7 列 %d（要 77）", at(l.Before, 10, gY+1), at(l.Before, 10, gY+8))
	}
	if at(l.Before, 200, gY+1) != 164 || at(l.After, 200, gY+1) != 10 {
		t.Errorf("前列 (200, %d) = %d（要入口時的 164）、後列 = %d（要返回時的 10）", gY+1, at(l.Before, 200, gY+1), at(l.After, 200, gY+1))
	}
	if at(l.Before, 203, 50) != 154 || at(l.After, 203, 50) != 154 {
		t.Errorf("細線像素前後列 %d、%d，要 154", at(l.Before, 203, 50), at(l.After, 203, 50))
	}
	if !r.OK() || !l.BoardInk.OK() {
		t.Fatalf("只在框內畫字應通過：提交 %s、Label %s", r, l.BoardInk)
	}
}

func TestBoardInk(t *testing.T) {
	cases := []struct {
		name    string
		yEntry  int
		during  func(h *harness)
		after   func(h *harness)
		cursorY int
		ok      bool
	}{
		{"只在框內畫字", gY, paintGround, nil, -1, true},
		{"第 y+8 列框內 x 有一點", gY, func(h *harness) { paintGround(h); h.bufPx(200, gY+8, 10) }, nil, -1, false},
		{"第 y+1 列 x = x1 有一點", gY, func(h *harness) { paintGround(h); h.bufPx(gX1, gY+1, 10) }, nil, -1, false},
		{"返回後、blit 前同一帶框外另畫一筆", gY, paintGround, func(h *harness) {
			for x := 259; x < 290; x++ {
				h.bufPx(x, gY+1, 10)
			}
		}, -1, true},
		{"沒有畫任何字", gY, nil, nil, -1, false},
		{"只畫框內中央、不碰三緣", gY, func(h *harness) { h.bufPx(200, 49, 10) }, nil, -1, false},
		{"CursorY 落在 46 到 53 照常核對", gY, paintGround, nil, 50, true},
		{"CursorY 落在 46 到 53 照常核對（失敗例）", gY, func(h *harness) { paintGround(h); h.bufPx(gX1, gY+1, 10) }, nil, 50, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l, r := runBoard(t, c.yEntry, nil, c.during, c.after, c.cursorY)
			if r.Skipped != "" {
				t.Fatalf("核對被略過：%s", r)
			}
			if r.OK() != c.ok {
				t.Fatalf("核對 %s，要通過 %t", r, c.ok)
			}
			if l.BoardInk.OK() != c.ok {
				t.Fatalf("Label.BoardInk %s，要通過 %t", l.BoardInk, c.ok)
			}
		})
	}
	t.Run("返回後另畫一筆不進後列", func(t *testing.T) {
		l, _ := runBoard(t, gY, nil, paintGround, func(h *harness) { h.bufPx(270, gY+1, 10) }, -1)
		if l.After[270] != 164 {
			t.Fatalf("後列 (270, %d) = %d，要 puts 返回時的 164", gY+1, l.After[270])
		}
	})
	t.Run("第一次 putc 的 y 與入口不同", func(t *testing.T) {
		l, r := runBoard(t, gY-1, nil, paintGround, nil, -1)
		if l.Before != nil || l.After != nil || r.Skipped == "" || r.OK() {
			t.Fatalf("前後列應作廢：前列 %d、後列 %d bytes，核對 %s", len(l.Before), len(l.After), r)
		}
		ov := textOverlay(t, testTextFonts(t), map[string]string{"GROUND": "地面"}, 2)
		ov.Sync([]Label{l}, 1)
		if len(ov.Stamps()) != 0 || len(ov.Skips) != 1 {
			t.Fatalf("前後列作廢時仍產生 stamp：%d 個，skips %+v", len(ov.Stamps()), ov.Skips)
		}
	})
}

// --- 第 3 項：規劃 ---

// TestBoardPlan：003 §3.4 的舊幾何（右對齊，legacyLabelGeom）照數值例；PlanLabelText 的 board 種類、色號與遮蓋列
// （水平置中的數值例見 text_e_test.go 的 TestBoardTextExample）。
func TestBoardPlan(t *testing.T) {
	cases := []struct {
		name             string
		x0, x1, n, scale int
		x, cells, cellW  int
		gx, gy           int
		cx0, cx1         int
	}{
		{"GROUND 2 字 2 倍", 177, 250, 2, 2, 234, 2, 8, 0, 0, 177, 250},
		{"GROUND 2 字 3 倍", 177, 250, 2, 3, 238, 2, 6, 1, 4, 177, 250},
		{"AIR 5 字 2 倍", 259, 290, 5, 2, 250, 5, 8, 0, 0, 250, 290},
		{"AIR 40 字 2 倍（tx 夾到 0）", 259, 290, 40, 2, 0, 40, 8, 0, 0, 0, 320},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := Label{Key: "K", X0: c.x0, X1: c.x1, Y: 45, Font: boardFont, Kind: KindBoard}
			g, err := legacyLabelGeom(l, c.n, c.scale)
			if err != nil {
				t.Fatal(err)
			}
			if g.X != c.x || g.Y != 46 || g.CellW != c.cellW || g.CellH != 8 || g.GlyphX != c.gx || g.GlyphY != c.gy || g.CX0 != c.cx0 || g.CX1 != c.cx1 {
				t.Errorf("舊幾何 %+v，要 X %d Y 46 CellW %d CellH 8 Glyph (%d,%d) 遮蓋 [%d, %d)", g, c.x, c.cellW, c.gx, c.gy, c.cx0, c.cx1)
			}
		})
	}
	t.Run("新路徑", func(t *testing.T) {
		l := Label{Key: "K", X0: 177, X1: 250, Y: 45, Font: boardFont, Kind: KindBoard}
		p, err := PlanLabelText(l, []rune("地面"), 2, testTextFonts(t))
		if err != nil {
			t.Fatal(err)
		}
		cv := p.Cover
		if p.Kind != KindBoard || p.BG != 164 || p.FG != 10 || p.Text.outline || p.Text.fgIdx != 10 {
			t.Errorf("種類 %s、色號 %d、%d、外框 %t，要 board、164、10、無外框", p.Kind, p.BG, p.FG, p.Text.outline)
		}
		if cv.Font != nil || cv.Cells != 1 || cv.X != 177 || cv.CellW != 73 || cv.Y != 46 || cv.CellH != 8 || cv.Key != "K.cover" {
			t.Errorf("遮蓋 [%d, %d) × [%d, %d)，要 [177, 250) × [46, 54)", cv.X, cv.X+cv.CellW, cv.Y, cv.Y+cv.CellH)
		}
	})
	t.Run("label 不變", func(t *testing.T) {
		l := Label{Key: "K", X0: 114, X1: 204, Y: 175, Font: measuredFont, Kind: KindLabel}
		g, err := legacyLabelGeom(l, 5, 2)
		if err != nil || g.X != 139 || g.CellH != 14 || g.GlyphY != 6 || g.CX0 != 114 || g.CX1 != 204 {
			t.Errorf("label 舊幾何 %+v（%v）", g, err)
		}
		p, err := PlanLabelText(l, []rune("測試譯文甲"), 2, testTextFonts(t))
		if err != nil || p.Kind != KindLabel || p.BG != 0 || p.FG != 15 || !p.Text.outline || p.Cover.CellH != 14 {
			t.Errorf("label 規劃 %+v（%v）", p, err)
		}
	})
}

// --- 第 4 項：字型檢查 ---

func boardLabel(key string, font uint16, kind Kind) Label {
	l := Label{Key: key, X0: 177, X1: 250, Y: 45, Font: font, Kind: kind}
	if kind == KindLabel {
		l.Y = 175
	}
	if kind == KindBoard {
		l.Before, l.After = make([]byte, 8*320), make([]byte, 8*320)
	}
	return l
}

func TestBoardFontCheck(t *testing.T) {
	cases := []struct {
		name  string
		kind  Kind
		font  uint16
		stamp bool
	}{
		{"board 配 FB12", KindBoard, boardFont, true},
		{"board 配 D7E6", KindBoard, measuredFont, false},
		{"label 配 FB12", KindLabel, boardFont, false},
		{"label 配 D7E6", KindLabel, measuredFont, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ov := textOverlay(t, testTextFonts(t), map[string]string{"GROUND": "地面"}, 2)
			ov.Sync([]Label{boardLabel("GROUND", c.font, c.kind)}, 1)
			if c.stamp && (len(ov.Stamps()) != 1 || len(ov.Skips) != 0) {
				t.Fatalf("要產生 stamp：%d 個，skips %+v", len(ov.Stamps()), ov.Skips)
			}
			if !c.stamp && (len(ov.Stamps()) != 0 || len(ov.Skips) != 1) {
				t.Fatalf("要 skip：%d 個 stamp，skips %+v", len(ov.Stamps()), ov.Skips)
			}
			for _, s := range ov.Stamps() {
				if ov.StampKind(s) != c.kind {
					t.Fatalf("stamp %q 種類 %s，要 %s", s.Key, ov.StampKind(s), c.kind)
				}
			}
		})
	}
}

// --- 第 5 項：Draw ---

func TestBoardDraw(t *testing.T) {
	var pal [256][3]uint8
	for i := range pal {
		pal[i] = [3]uint8{uint8(i), uint8(255 - i), 77}
	}
	// 畫格：全 7 號，白板帶 [170, 300) × [46, 54) 是 164。
	idx := make([]byte, 64000)
	for i := range idx {
		idx[i] = 7
	}
	for y := 46; y < 54; y++ {
		for x := 170; x < 300; x++ {
			idx[y*320+x] = 164
		}
	}
	set := func(b []byte, x, y int, c byte) { b[(y-46)*320+x] = c }
	before := make([]byte, 8*320)
	for y := 46; y < 54; y++ {
		copy(before[(y-46)*320:(y-45)*320], idx[y*320:(y+1)*320])
	}
	// 白板細線：(240, 50) 在文字格內、字模以外；(203, 52) 在畫字後被墨跡蓋住。
	set(before, 240, 50, 154)
	set(before, 203, 52, 159)
	after := append([]byte(nil), before...)
	set(after, 180, 47, 10)
	set(after, 203, 52, 10)
	set(after, 239, 49, 88)
	set(after, 185, 48, 10)
	// 目前畫格 = 後列，再疊人工游標：(190, 50) 原本不是墨跡，(185, 48) 原本是墨跡。
	for y := 46; y < 54; y++ {
		copy(idx[y*320:(y+1)*320], after[(y-46)*320:(y-45)*320])
	}
	idx[50*320+190] = 15
	idx[48*320+185] = 15

	g := Label{Key: "GROUND", X0: 177, X1: 250, Y: 45, Font: boardFont, Kind: KindBoard, Before: before, After: after}
	r := Label{Key: "Return to office", X0: 100, X1: 160, Y: 175, Font: measuredFont, Kind: KindLabel}
	// 005 §4.1：Ref 取自已疊上人工游標像素的畫格（游標靜止時在 blit 前已畫進緩衝，003 §2.3）。
	g.Ref, r.Ref = refFromFrame(idx, g), refFromFrame(idx, r)
	ov := textOverlay(t, testTextFonts(t), map[string]string{"GROUND": "地面", "Return to office": "測試"}, 2)
	ov.Sync([]Label{g, r}, 1)
	if len(ov.Stamps()) != 2 {
		t.Fatalf("stamp %d 個，要 2（遮蓋）", len(ov.Stamps()))
	}
	out, miss := ov.Draw(idx, pal)
	if len(miss) != 0 {
		t.Fatalf("缺字 %q", miss)
	}
	at := func(x, y int) [3]uint8 { i := 4 * (y*640 + x); return [3]uint8{out[i], out[i+1], out[i+2]} }
	// 原版像素 (x, y) 放大後的 2×2 都要等於 want。
	check := func(name string, x, y int, want uint8) {
		t.Helper()
		for dy := 0; dy < 2; dy++ {
			for dx := 0; dx < 2; dx++ {
				if c := at(2*x+dx, 2*y+dy); c != pal[want] {
					t.Errorf("%s (%d, %d) 放大後 (%d, %d) = %v，要 %d 號 %v", name, x, y, 2*x+dx, 2*y+dy, c, want, pal[want])
				}
			}
		}
	}
	check("墨跡換回前列", 180, 47, 164)
	check("被墨跡蓋住的細線還原", 203, 52, 159)
	check("文字格內、字模以外的墨跡換回前列", 239, 49, 164)
	check("文字格內、字模以外的細線", 240, 50, 154)
	check("游標（非墨跡處）", 190, 50, 15)
	check("游標（蓋在墨跡上）", 185, 48, 15)
	check("遮蓋內的白板", 200, 46, 164)
	check("遮蓋外", 176, 47, 164)
	check("遮蓋外的列", 200, 54, 7)
	// 字（測試圖集的實心方塊）：board「地面」2 倍 16 像素，tx 411、baseline 106，填色 x 412..425、428..441，列 93..106。
	if c := at(415, 100); c != pal[10] {
		t.Errorf("board 字 %v，要 10 號 %v", c, pal[10])
	}
	if c := at(430, 100); c != pal[10] {
		t.Errorf("board 第二字 %v，要 10 號 %v", c, pal[10])
	}
	// label（退路）：遮蓋是 0 號；「測試」2 倍 22 像素，tx 238、baseline 374，第一字填色 x 239..258、列 356..375。
	if c := at(2*100, 2*176); c != pal[0] {
		t.Errorf("label 遮蓋 %v，要 0 號 %v", c, pal[0])
	}
	if c := at(240, 360); c != pal[15] {
		t.Errorf("label 字 %v，要 15 號 %v", c, pal[15])
	}
	if cc := CheckCompositeText(out, ScaleIndexedRGBA(idx, pal, 2), 2, ov.Stamps(), nil, ov.TextBoxes()); cc.Diffs == 0 || cc.Outside != 0 {
		t.Errorf("合成圖比較 %+v", cc)
	}

	// generation 沒變時仍更新前後列：新的後列在 (180, 47) 沒有墨跡，畫格的 10 號就保留。
	after2 := append([]byte(nil), after...)
	set(after2, 180, 47, 164)
	g2 := g
	g2.After = after2
	first := ov.Stamps()[0]
	ov.Sync([]Label{g2, r}, 1)
	if ov.Stamps()[0] != first {
		t.Fatal("generation 相同時重建了 stamp")
	}
	out, _ = ov.Draw(idx, pal)
	if c := at(2*180, 2*47); c != pal[10] {
		t.Errorf("前後列更新後 (180, 47) = %v，要畫格的 10 號 %v", c, pal[10])
	}
}

// --- 第 6 項：lint（缺 GROUND 列入漏譯、ExeKeys 與 LABL 交集失敗見 lint_test.go） ---

func TestExeKeysDisjointFromLABL(t *testing.T) {
	// LABL 的 Return to Office 是大寫 O，與 ExeKeys 的 Return to office 不同 key。
	labl := testLABL("Talk to Gwen", "Fly Mission", "Return to Office", "Return to Tent")
	if rep := LintLabels([]byte(lintTSV(nil, nil)), testTextFonts(t), labl); !rep.OK() {
		t.Fatalf("大寫 O 的 LABL key 不算交集：%v", rep.Errors)
	}
}
