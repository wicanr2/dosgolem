package strikecommander

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// strike_commander 規格 009 第 5 版 §4.1 第 16 到 18 項（第 11 項的新斷言在 flight_draw_test.go）。
// 合成 SHP 由測試手寫（PAK 與字形都自製，不含原版資料）；期望值在測試內獨立寫出。
// 合成世界的「原版」繪製由測試自己依字形定義與窗（四邊含等號）逐點裁切，不呼叫產品的解碼。

// tRun 是合成字形的一段原始色號片段：起點 (dx, dy)，n 個像素。
type tRun struct{ dx, dy, n int }

// tGlyph 是合成字形：標頭 right、left、top、bottom 與片段。
type tGlyph struct {
	right, left, top, bottom int
	runs                     []tRun
}

// pts 回字形寫入的點 (dx + k, dy)。
func (g tGlyph) pts() []Point {
	var out []Point
	for _, r := range g.runs {
		for k := 0; k < r.n; k++ {
			out = append(out, Point{r.dx + k, r.dy})
		}
	}
	return out
}

// block 是 w 欄、dy0..dy1 列的實心字形，標頭剛好包住（bottom 另加 extra 列空間）。
func block(w, dy0, dy1, extra int) tGlyph {
	g := tGlyph{right: w - 1, left: 0, top: -dy0, bottom: dy1 + extra}
	for dy := dy0; dy <= dy1; dy++ {
		g.runs = append(g.runs, tRun{0, dy, w})
	}
	return g
}

// letters 回 A 到 Z 都是 g 的字形表。
func letters(g tGlyph) map[byte]tGlyph {
	m := map[byte]tGlyph{}
	for c := byte('A'); c <= 'Z'; c++ {
		m[c] = g
	}
	return m
}

const tShpEntries = 128

func encGlyph(g tGlyph) []byte {
	var b bytes.Buffer
	for _, x := range []int{g.right, g.left, g.top, g.bottom} {
		binary.Write(&b, binary.LittleEndian, int16(x))
	}
	for _, r := range g.runs {
		binary.Write(&b, binary.LittleEndian, uint16(r.n<<1))
		binary.Write(&b, binary.LittleEndian, int16(r.dx))
		binary.Write(&b, binary.LittleEndian, int16(r.dy))
		for k := 0; k < r.n; k++ {
			b.WriteByte(15)
		}
	}
	binary.Write(&b, binary.LittleEndian, uint16(0))
	return b.Bytes()
}

// buildSHP 編一份 128 條目的 PAK；沒定義的條目是空 shape（標頭全 0、只有結尾 0）。
func buildSHP(glyphs map[byte]tGlyph) []byte {
	var data [][]byte
	for i := 0; i < tShpEntries; i++ {
		data = append(data, encGlyph(glyphs[byte(i)]))
	}
	off := 4 + 4*tShpEntries
	out := make([]byte, off)
	for i, d := range data {
		binary.LittleEndian.PutUint32(out[4+4*i:], uint32(off))
		off += len(d)
		out = append(out, d...)
	}
	binary.LittleEndian.PutUint32(out, uint32(len(out)))
	return out
}

// 合成字型物件與 SHP 的位址（SHP 線性 E96E4，同 SM-FONT 的實際位置，只為好讀）。
const (
	cFont   = 0xD86E
	cShpSeg = 0xE96E
	cShpOff = 0x0004
	cInk    = 220
)

func installClipFont(v *fv, fy int, shp []byte) {
	v.w16(qDS, cFont+fontVTable, fontVTableWant)
	v.w16(qDS, cFont+fontPtrOff, cShpOff)
	v.w16(qDS, cFont+fontPtrSeg, cShpSeg)
	v.w8(qDS, cFont+fontKind, fontKindWant)
	v.w16(qDS, cFont+fontYOffset, uint16(int16(fy)))
	for i, b := range shp {
		v.mem[lin(cShpSeg, cShpOff)+uint32(i)] = b
	}
}

func setWin(v *fv, obj uint16, win [4]int) {
	for i, o := range []uint16{objWinX0, objWinY0, objWinX1, objWinY1} {
		v.w16(qDS, obj+o, uint16(int16(win[i])))
	}
}

var leftWin = [4]int{17, 121, 96, 186}

type clipWorld struct {
	*world
	fy     int
	win    [4]int
	glyphs map[byte]tGlyph
}

func newClipWorld(t *testing.T, fy int, glyphs map[byte]tGlyph, win [4]int) *clipWorld {
	w := newWorld(t, 0x0241)
	installClipFont(w.v, fy, buildSHP(glyphs))
	setWin(w.v, tObjMFD, win)
	w.v.w16(qDS, tObjMFD+objFont, cFont)
	return &clipWorld{world: w, fy: fy, win: win, glyphs: glyphs}
}

// cOpt 是一次合成的 mfd puts：「原版」逐字以 drawWin（nil 用物件的窗）裁切畫字形；extra 在最後一字後多畫；
// hook 在每字畫完後呼叫（i 是字的索引，第一字時 puts 入口已觀察過）。
type cOpt struct {
	s       string
	x, y    int
	drawWin *[4]int
	noDraw  bool
	extra   []Point
	hook    func(i int)
}

func (cw *clipWorld) mfd(op cOpt) {
	win := cw.win
	if op.drawWin != nil {
		win = *op.drawWin
	}
	last := -1
	for i := 0; i < len(op.s); i++ {
		if op.s[i] != ' ' {
			last = i
		}
	}
	cw.puts(putsOpt{kind: KindMFD, s: op.s, x: op.x, y: op.y, color: cInk, inkFn: func(i, cx int) {
		if !op.noDraw {
			for _, p := range cw.glyphs[op.s[i]].pts() {
				x, y := cx+p.X, op.y+cw.fy+p.Y
				if x >= win[0] && x <= win[2] && y >= win[1] && y <= win[3] && x >= 0 && x < 320 && y >= 0 && y < 200 {
					cw.setPx(x, y, cInk)
				}
			}
		}
		if i == last {
			for _, p := range op.extra {
				cw.setPx(p.X, p.Y, cInk)
			}
		}
		if op.hook != nil {
			op.hook(i)
		}
	}})
}

// v5Counts 回第 5 版五個計數。
func v5Counts(c FlightCounts) [5]int {
	return [5]int{c.SkipClipped, c.ClipPartialWindow, c.ClipPartialBand, c.BandShifted, c.InkGlyphFallback}
}

func logKinds(o *FlightObserver) map[string]int {
	logs, _ := o.Log()
	m := map[string]int{}
	for _, e := range logs {
		m[e.Kind]++
	}
	return m
}

// checkV5Quiet 斷言觀察器沒有任何第 5 版的痕跡：計數全 0、log 沒有 clip、inkfb、沒有整串裁切事件。
func checkV5Quiet(t *testing.T, o *FlightObserver, where string) {
	t.Helper()
	if c := v5Counts(o.Counts()); c != [5]int{} {
		t.Fatalf("%s：第 5 版計數 %v，要全 0", where, c)
	}
	if k := logKinds(o); k["clip"] != 0 || k["inkfb"] != 0 {
		t.Fatalf("%s：log 有 clip %d、inkfb %d", where, k["clip"], k["inkfb"])
	}
	if ev, drop := o.Clipped(); len(ev) != 0 || drop != 0 {
		t.Fatalf("%s：整串裁切事件 %d 筆、捨棄 %d", where, len(ev), drop)
	}
}

// want 是一次 puts 之後的計數期望（只列出相關的）。
type clipWant struct {
	pending                          int
	skipInk, skipVoid, skipClipped   int
	partialWin, partialBand, shifted int
	fallback                         int
}

func checkClip(t *testing.T, cw *clipWorld, where string, w clipWant) {
	t.Helper()
	c := cw.o.Counts()
	got := clipWant{len(cw.o.Pending()), c.SkipInk, c.SkipVoid, c.SkipClipped, c.ClipPartialWindow, c.ClipPartialBand, c.BandShifted, c.InkGlyphFallback}
	if got != w {
		logs, _ := cw.o.Log()
		t.Fatalf("%s：得 %+v，要 %+v\nlog %+v", where, got, w, logs)
	}
}

var blockAIR = letters(block(4, -4, 0, 0)) // 4 欄、dy −4..0：fy 5 時墨跡在 Y+1..Y+5，同 SM-FONT 大寫

// ---- 第 16 項：新路徑分類 ----

func TestFlightClipTable(t *testing.T) {
	// |E| = 0：G、N、D 都是空 shape → 改用舊規則（原版照舊規則的框畫方塊，通過），計 InkGlyphFallback。
	t.Run("E 為空", func(t *testing.T) {
		cw := newClipWorld(t, 5, map[byte]tGlyph{'A': block(4, -4, 0, 0)}, leftWin)
		cw.puts(putsOpt{kind: KindMFD, s: "GND", x: 29, y: 140, color: cInk})
		checkClip(t, cw, "E 為空", clipWant{pending: 1, fallback: 1})
		if p := cw.o.Pending(); p[0].Clip != nil {
			t.Fatalf("改用舊規則的 Clip 不是 nil")
		}
		if logKinds(cw.o)["inkfb"] != 1 {
			t.Fatalf("log 沒有 inkfb：%v", logKinds(cw.o))
		}
	})
	// EW = ∅ 且 Diffs = 0：整串裁切（y′ 187，E 列 188..192 在窗下緣 186 之外）。
	t.Run("整串裁切", func(t *testing.T) {
		cw := newClipWorld(t, 5, blockAIR, leftWin)
		cw.mfd(cOpt{s: "AIR", x: 29, y: 187})
		checkClip(t, cw, "整串裁切", clipWant{skipClipped: 1})
		ev, drop := cw.o.Clipped()
		if len(ev) != 1 || drop != 0 {
			t.Fatalf("事件 %d 筆、捨棄 %d", len(ev), drop)
		}
		e := ev[0]
		want := FlightClipEvent{Steps: e.Steps, Key: "AIR", Y: 187, YPrime: 187, Window: leftWin, FY: 5, NE: 60, NEW: 0, Diffs: 0, NXs: 3, X0: 29, X1: 47}
		if e != want || e.Steps == 0 {
			t.Fatalf("事件 %+v，要 %+v", e, want)
		}
		if logKinds(cw.o)["clip"] != 1 || logKinds(cw.o)["ink"] != 0 {
			t.Fatalf("log kind %v", logKinds(cw.o))
		}
		if tsv := string(FlightClippedTSV(ev)); !strings.Contains(tsv, "\tAIR\t187\t187\t17,121,96,186\t5\t60\t0\t0\t3\t29\t47\n") ||
			!strings.HasPrefix(tsv, "steps\tkey\ty\typrime\twindow\tfy\tne\tnew\tdiffs\tnxs\tx0\tx1\n") {
			t.Fatalf("flight-clipped.tsv：%q", tsv)
		}
	})
	// EW = ∅ 且 Diffs > 0：不通過。
	t.Run("窗外有差異", func(t *testing.T) {
		cw := newClipWorld(t, 5, blockAIR, leftWin)
		cw.mfd(cOpt{s: "AIR", x: 29, y: 187, extra: []Point{{50, 188}}})
		checkClip(t, cw, "窗外有差異", clipWant{skipInk: 1})
	})
	// EW ≠ ∅、EWB = ∅、Diffs = 0：fy 20，E 列 Y+16..Y+20 全在窗內、全在 band 外（不推），不通過（F56）。
	t.Run("band 沒讀到窗內墨跡", func(t *testing.T) {
		cw := newClipWorld(t, 20, blockAIR, leftWin)
		cw.mfd(cOpt{s: "AIR", x: 29, y: 130})
		checkClip(t, cw, "EWB 空", clipWant{skipInk: 1})
	})
	// Diffs = 0、EW 非空：讀錯頁（前後列相同），不通過（F51）。
	t.Run("讀錯頁", func(t *testing.T) {
		cw := newClipWorld(t, 5, blockAIR, leftWin)
		cw.mfd(cOpt{s: "AIR", x: 29, y: 140, noDraw: true})
		checkClip(t, cw, "讀錯頁", clipWant{skipInk: 1})
	})
	// D ⊄ EWB：band 內多一點非字形的差異，不通過。
	t.Run("差異不在預期內", func(t *testing.T) {
		cw := newClipWorld(t, 5, blockAIR, leftWin)
		cw.mfd(cOpt{s: "AIR", x: 29, y: 140, extra: []Point{{60, 141}}})
		checkClip(t, cw, "D 不在 EWB", clipWant{skipInk: 1})
	})
	// 通過。
	t.Run("通過", func(t *testing.T) {
		cw := newClipWorld(t, 5, blockAIR, leftWin)
		cw.mfd(cOpt{s: "AIR", x: 29, y: 140})
		checkClip(t, cw, "通過", clipWant{pending: 1})
		p := cw.o.Pending()[0]
		want := FlightClip{Window: leftWin, FY: 5, NE: 60, NEW: 60, NEWB: 60, Class: ClipPass}
		if p.Clip == nil || *p.Clip != want || p.B0 != 139 || p.BH != 8 {
			t.Fatalf("通過：Clip %+v、B0 %d", p.Clip, p.B0)
		}
	})
	// 整字外框與窗相交、字形像素全在窗外（F52）：字形標頭 3 列、只有最上一列有墨跡，窗 y0 = Y+4。
	t.Run("外框相交像素在窗外", func(t *testing.T) {
		dash := tGlyph{right: 3, left: 0, top: 2, bottom: 0, runs: []tRun{{0, -2, 4}}}
		cw := newClipWorld(t, 5, letters(dash), [4]int{17, 144, 96, 186})
		cw.mfd(cOpt{s: "GUN", x: 29, y: 140})
		checkClip(t, cw, "外框相交", clipWant{skipClipped: 1})
	})
	// fy 由字型物件讀：fy = 4（F54）與 fy = −1（F58，有號）都通過。
	t.Run("fy 4", func(t *testing.T) {
		cw := newClipWorld(t, 4, blockAIR, leftWin)
		cw.mfd(cOpt{s: "AIR", x: 29, y: 140})
		checkClip(t, cw, "fy 4", clipWant{pending: 1})
	})
	t.Run("fy −1", func(t *testing.T) {
		cw := newClipWorld(t, -1, blockAIR, leftWin)
		cw.mfd(cOpt{s: "AIR", x: 29, y: 140}) // E 列 135..139，band 139..146 只含 139
		checkClip(t, cw, "fy −1", clipWant{pending: 1, partialBand: 1})
		if c := cw.o.Pending()[0].Clip; c == nil || c.FY != -1 || c.NEWB != 12 || c.NEW != 60 {
			t.Fatalf("fy −1：Clip %+v", c)
		}
	})
}

// 返回時窗與入口不同 → SkipVoid（F65）。
func TestFlightClipWindowChanged(t *testing.T) {
	cw := newClipWorld(t, 5, blockAIR, leftWin)
	cw.mfd(cOpt{s: "AIR", x: 29, y: 140, hook: func(i int) {
		if i == 2 {
			cw.v.w16(qDS, tObjMFD+objWinX0, 18)
		}
	}})
	checkClip(t, cw, "窗改變", clipWant{skipVoid: 1})
}

// 差分相容（明文測試）：合成世界的 hud、mfd 與 mfd 的 font = 0、窗全 0 各一筆，與凍結複本逐欄相同，沒有第 5 版痕跡。
func TestFlightClipSilentDiff(t *testing.T) {
	d := newCUDual(t)
	installClipFont(d.v, 5, buildSHP(blockAIR))
	// hud 的字型與窗都合法，仍走舊路徑（不讀窗與字型）。
	setWin(d.v, tObjHUD, [4]int{183, 11, 213, 12})
	d.v.w16(qDS, tObjHUD+objFont, cFont)
	d.hudAt("GEAR", 10)
	d.check(t, "hud")
	// mfd：font = 0、窗全 0。
	d.fillBand(120, 140, 0)
	d.puts(putsOpt{kind: KindMFD, s: "NORM", x: 222, y: 126, color: cInk})
	d.check(t, "mfd 兩者全 0")
	// mfd：font = 0、窗非 0。
	setWin(d.v, tObjMFD, [4]int{222, 121, 301, 186})
	d.fillBand(120, 140, 0)
	d.puts(putsOpt{kind: KindMFD, s: "GUN", x: 222, y: 131, color: cInk})
	d.check(t, "mfd font 0")
	// mfd：窗全 0、font 合法。
	setWin(d.v, tObjMFD, [4]int{})
	d.v.w16(qDS, tObjMFD+objFont, cFont)
	d.fillBand(120, 140, 0)
	d.puts(putsOpt{kind: KindMFD, s: "AIR", x: 230, y: 121, color: cInk})
	d.check(t, "mfd 窗全 0")
	if len(d.o.Pending()) != 4 {
		t.Fatalf("待提交 %v，要 4 筆", keysOf(d.o.Pending()))
	}
	d.cuFrame()
	d.check(t, "升格")
	for _, x := range append(d.o.Visible(), d.o.Pending()...) {
		if x.Clip != nil {
			t.Fatalf("%s 的 Clip 不是 nil", x.Key)
		}
	}
	checkV5Quiet(t, d.o, "差分相容")
}

// 改用舊規則的各種不合法（§3.2.4.5）：都計 InkGlyphFallback、記 inkfb，以舊規則判定。
func TestFlightClipFallback(t *testing.T) {
	type mod func(cw *clipWorld)
	shpPatch := func(f func(b []byte) []byte) mod {
		return func(cw *clipWorld) {
			b := f(buildSHP(blockAIR))
			for i, x := range b {
				cw.v.mem[lin(cShpSeg, cShpOff)+uint32(i)] = x
			}
		}
	}
	cases := []struct {
		name string
		m    mod
	}{
		{"y0 > y1", func(cw *clipWorld) { setWin(cw.v, tObjMFD, [4]int{17, 186, 96, 121}) }},
		{"x1 = 320", func(cw *clipWorld) { setWin(cw.v, tObjMFD, [4]int{17, 121, 320, 186}) }},
		{"x0 負", func(cw *clipWorld) { setWin(cw.v, tObjMFD, [4]int{-1, 121, 96, 186}) }},
		{"+0 不是 3FDA", func(cw *clipWorld) { cw.v.w16(qDS, cFont, 0x064E) }},
		{"+6 不是 2", func(cw *clipWorld) { cw.v.w8(qDS, cFont+fontKind, 3) }},
		{"遠指標 0", func(cw *clipWorld) { cw.v.w16(qDS, cFont+fontPtrOff, 0); cw.v.w16(qDS, cFont+fontPtrSeg, 0) }},
		{"total < 8", shpPatch(func(b []byte) []byte { binary.LittleEndian.PutUint32(b, 4); return b })},
		{"total 超過 10000h", shpPatch(func(b []byte) []byte { binary.LittleEndian.PutUint32(b, 0x10001); return b })},
		{"條目範圍超出 total", shpPatch(func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[4+4*int('B'):], uint32(len(b)+10))
			return b
		})},
		{"條目型別非 0", shpPatch(func(b []byte) []byte { b[4+4*int('A')+3] = 0x01; return b })},
		{"字形解碼失敗", shpPatch(func(b []byte) []byte {
			// A 的第一段 dx 改成 9：像素超出 4×5 標頭。
			off := int(binary.LittleEndian.Uint32(b[4+4*int('A'):]))
			binary.LittleEndian.PutUint16(b[off+8+2:], 9)
			return b
		})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cw := newClipWorld(t, 5, blockAIR, leftWin)
			c.m(cw)
			// 原版照舊規則的框畫方塊（世界預設的 MFD 墨跡），舊規則通過。
			cw.puts(putsOpt{kind: KindMFD, s: "AIR", x: 29, y: 140, color: cInk})
			checkClip(t, cw, c.name, clipWant{pending: 1, fallback: 1})
			if cw.o.Pending()[0].Clip != nil || logKinds(cw.o)["inkfb"] != 1 {
				t.Fatalf("%s：Clip %v、log %v", c.name, cw.o.Pending()[0].Clip, logKinds(cw.o))
			}
		})
	}
	// x0 > x1 且 Diffs = 0：舊規則 SkipInk 加 InkGlyphFallback（F57：不檢查合法性時會判成整串裁切）。
	t.Run("x0 > x1", func(t *testing.T) {
		cw := newClipWorld(t, 5, blockAIR, [4]int{96, 121, 17, 186})
		cw.mfd(cOpt{s: "AIR", x: 29, y: 140, noDraw: true})
		checkClip(t, cw, "x0 > x1", clipWant{skipInk: 1, fallback: 1})
	})
}

// 四邊含等號（F55a 到 F55d）：墨跡恰在邊上通過；把該邊內縮 1，那一點成為窗外差異，不通過。
func TestFlightClipEdges(t *testing.T) {
	// G、U、N 都是 1 個像素 (0, −2)：fy 5、Y 140、x 29 → (29,143)、(35,143)、(41,143)。
	dot := letters(tGlyph{right: 0, left: 0, top: 2, bottom: 0, runs: []tRun{{0, -2, 1}}})
	cases := []struct {
		name         string
		edge, shrunk [4]int
	}{
		{"x0", [4]int{29, 121, 96, 186}, [4]int{30, 121, 96, 186}},
		{"x1", [4]int{17, 121, 41, 186}, [4]int{17, 121, 40, 186}},
		{"y0", [4]int{17, 143, 96, 186}, [4]int{17, 144, 96, 186}},
		{"y1", [4]int{17, 121, 96, 143}, [4]int{17, 121, 96, 142}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cw := newClipWorld(t, 5, dot, c.edge)
			cw.mfd(cOpt{s: "GUN", x: 29, y: 140})
			checkClip(t, cw, c.name+" 恰在邊上", clipWant{pending: 1})
			cw2 := newClipWorld(t, 5, dot, c.shrunk)
			edge := c.edge
			cw2.mfd(cOpt{s: "GUN", x: 29, y: 140, drawWin: &edge})
			checkClip(t, cw2, c.name+" 內縮 1", clipWant{skipInk: 1})
		})
	}
}

// E ∩ W 有點在 band 外：通過，計 ClipPartialBand。
func TestFlightClipPartialBand(t *testing.T) {
	cw := newClipWorld(t, 5, letters(block(4, -4, 4, 0)), leftWin) // E 列 Y+1..Y+9，band Y−1..Y+6
	cw.mfd(cOpt{s: "AIR", x: 29, y: 140})
	checkClip(t, cw, "band 外", clipWant{pending: 1, partialBand: 1})
	if c := cw.o.Pending()[0].Clip; c == nil || c.NE != 108 || c.NEW != 108 || c.NEWB != 72 || c.Shifted {
		t.Fatalf("Clip %+v", c)
	}
}

// 快取：兩次 puts 之間改掉一個用到的條目，第二次重解；解析不通過的字串不讀 SHP。
func TestFlightClipCache(t *testing.T) {
	g := letters(block(4, -4, 0, 1)) // 標頭下方多 1 列，改 dy 之後仍在標頭內
	cw := newClipWorld(t, 5, g, leftWin)
	cw.mfd(cOpt{s: "AIR", x: 29, y: 140})
	checkClip(t, cw, "第一次", clipWant{pending: 1})
	if cw.o.shp == nil {
		t.Fatal("第一次之後沒有快取")
	}
	// A 的最後一段 dy 0 → 1：原版改畫在 Y+6，舊快取的 E 沒有這列，若不重解就會判成 D ⊄ EWB。
	shp := buildSHP(g)
	off := int(binary.LittleEndian.Uint32(shp[4+4*int('A'):]))
	runLen := 6 + 4
	a := lin(cShpSeg, cShpOff) + uint32(off+8+4*runLen+4)
	cw.v.mem[a], cw.v.mem[a+1] = 1, 0
	ng := map[byte]tGlyph{}
	for k, v := range g {
		ng[k] = v
	}
	na := block(4, -4, 0, 1)
	na.runs[4].dy = 1
	ng['A'] = na
	cw.glyphs = ng
	cw.fillBand(120, 160, 0)
	cw.mfd(cOpt{s: "AIR", x: 29, y: 150})
	checkClip(t, cw, "改條目後", clipWant{pending: 2})
	if c := cw.o.Pending()[1].Clip; c == nil || c.NE != 60 {
		t.Fatalf("重解後 Clip %+v", c)
	}
	// 解析不通過的字串不讀 SHP：新世界的 Hawk 不在 mfd catalog，快取仍是 nil、沒有 InkGlyphFallback；
	// SHP 總長改成 4（若被讀就會改用舊規則）。
	cw3 := newClipWorld(t, 5, g, leftWin)
	cw3.v.mem[lin(cShpSeg, cShpOff)] = 4
	cw3.v.mem[lin(cShpSeg, cShpOff)+1] = 0
	cw3.mfd(cOpt{s: "Hawk", x: 29, y: 140})
	if cw3.o.shp != nil || cw3.o.Counts().InkGlyphFallback != 0 || cw3.o.Counts().SkipResolve != 1 {
		t.Fatalf("解析不通過：快取 %v、fallback %d、解析略過 %d", cw3.o.shp != nil, cw3.o.Counts().InkGlyphFallback, cw3.o.Counts().SkipResolve)
	}
}

// ---- 第 17 項：列向往內推 ----

// rowBG 是逐列不同的背景色（避開墨跡色），讓列位移錯時比對得出來。
func rowBG(y int) uint8 { return uint8(1 + y%50) }

func newPushWorld(t *testing.T, win [4]int) *clipWorld {
	cw := newClipWorld(t, 5, blockAIR, win)
	for y := 100; y < 200; y++ {
		cw.fillBand(y, y+1, rowBG(y))
	}
	return cw
}

// pushCase 跑一筆，回入口時決定的 B0（第一字的 hook 讀 o.cur）。
func pushCase(t *testing.T, cw *clipWorld, s string, x, y int) int {
	t.Helper()
	b0 := -999
	cw.mfd(cOpt{s: s, x: x, y: y, hook: func(i int) {
		if i == 0 && cw.o.cur != nil {
			b0 = cw.o.cur.t.B0
		}
	}})
	return b0
}

func TestFlightClipPush(t *testing.T) {
	rightWin := [4]int{222, 121, 301, 186}
	for _, side := range []struct {
		name string
		win  [4]int
		x    int
	}{{"左 MFD", leftWin, 29}, {"右 MFD", rightWin, 250}} {
		t.Run(side.name, func(t *testing.T) {
			// 第 1 款：y′ 183，E 列 184..188 部分在窗內 → B0 181，前列從 181 起讀，通過後 BandShifted 1、Shifted 真。
			cw := newPushWorld(t, side.win)
			if b0 := pushCase(t, cw, "NORM", side.x, 183); b0 != 181 {
				t.Fatalf("y′ 183 入口 B0 %d，要 181", b0)
			}
			checkClip(t, cw, "y′ 183", clipWant{pending: 1, partialWin: 1, shifted: 1})
			p := cw.o.Pending()[0]
			if p.B0 != 181 || p.Clip == nil || !p.Clip.Shifted || p.Before[0] != rowBG(181) || p.Before[7*320] != rowBG(188) {
				t.Fatalf("y′ 183：B0 %d、Clip %+v、前列首列 %d", p.B0, p.Clip, p.Before[0])
			}
			// 第 2 款：y′ 182 → 181（不變，BandShifted 0）；y′ 121 → 120。
			for _, c := range []struct{ y, b0 int }{{182, 181}, {121, 120}} {
				cw := newPushWorld(t, side.win)
				if b0 := pushCase(t, cw, "NORM", side.x, c.y); b0 != c.b0 {
					t.Fatalf("y′ %d 入口 B0 %d，要 %d", c.y, b0, c.b0)
				}
				pw := 0
				if c.y == 182 {
					pw = 1
				}
				checkClip(t, cw, "不推", clipWant{pending: 1, partialWin: pw})
				if p := cw.o.Pending()[0]; p.B0 != c.b0 || p.Clip.Shifted {
					t.Fatalf("y′ %d：B0 %d、Shifted %t", c.y, p.B0, p.Clip.Shifted)
				}
			}
			// 第 3 款：窗 y1 = 199、Y 185，E 列全在窗內 → B0 184（不套 clamp）。
			w3 := side.win
			w3[3] = 199
			cw = newPushWorld(t, w3)
			if b0 := pushCase(t, cw, "NORM", side.x, 185); b0 != 184 {
				t.Fatalf("窗 y1 199、Y 185：B0 %d，要 184", b0)
			}
			checkClip(t, cw, "y1 199", clipWant{pending: 1})
			// 第 5 款：T 下界，Y 116、E 列 117..121 部分在窗內 → B0 116（不是 115）。
			cw = newPushWorld(t, side.win)
			if b0 := pushCase(t, cw, "NORM", side.x, 116); b0 != 116 {
				t.Fatalf("Y 116：B0 %d，要 116", b0)
			}
			checkClip(t, cw, "T 下界", clipWant{pending: 1, partialWin: 1, shifted: 1})
			// 第 6 款：不相交不推，Y 187、E 列 188..192 → B0 186，返回整串裁切，BandShifted 0。
			cw = newPushWorld(t, side.win)
			if b0 := pushCase(t, cw, "NORM", side.x, 187); b0 != 186 {
				t.Fatalf("Y 187：B0 %d，要 186", b0)
			}
			checkClip(t, cw, "不相交", clipWant{skipClipped: 1})
			// 第 7 款：列部分在窗內、x 全在窗外 → B0 已推（181），返回整串裁切，BandShifted 0（F67）。
			cw = newPushWorld(t, side.win)
			xOut := side.win[2] + 3
			if b0 := pushCase(t, cw, "NORM", xOut, 183); b0 != 181 {
				t.Fatalf("x 在窗外：B0 %d，要 181", b0)
			}
			checkClip(t, cw, "x 在窗外", clipWant{skipClipped: 1})
		})
	}
	// 第 4 款：hud 的列部分在窗外（字型與窗都合法）→ 舊路徑，B0 = Y + 1。
	t.Run("hud", func(t *testing.T) {
		cw := newPushWorld(t, leftWin)
		setWin(cw.v, tObjHUD, [4]int{183, 11, 213, 12})
		cw.v.w16(qDS, tObjHUD+objFont, cFont)
		cw.hud("GEAR", 190, 10)
		p := cw.o.Pending()
		if len(p) != 1 || p[0].B0 != 11 || p[0].Clip != nil {
			t.Fatalf("hud：待提交 %d、B0 %v", len(p), p)
		}
		checkV5Quiet(t, cw.o, "hud")
	})
}

// 第 8 款：推過的那筆，遮蓋 stamp 的 Y 與 flightMismatch 的列位移起點都是 181（F66）。
func TestFlightClipPushOverlay(t *testing.T) {
	cw := newPushWorld(t, leftWin)
	pushCase(t, cw, "NORM", 29, 183)
	cw.cuFrame()
	vis := cw.o.Visible()
	if len(vis) != 1 || vis[0].B0 != 181 || vis[0].Ref == nil || vis[0].Ref[0] != rowBG(181) {
		t.Fatalf("可見集合 %+v", keysOf(vis))
	}
	for _, sc := range []int{2, 3} {
		fo, err := NewFlightOverlayText(flightTextFonts(t), flightTestCats(), sc)
		if err != nil {
			t.Fatal(err)
		}
		fo.Sync(vis, cw.o.Generation())
		if len(fo.entries) != 1 {
			t.Fatalf("%d 倍：筆數 %d，misses %+v skips %+v", sc, len(fo.entries), fo.Misses, fo.Skips)
		}
		e := fo.entries[0]
		if e.cover.Y != 181 || e.cover.CellH != 8 {
			t.Fatalf("%d 倍：遮蓋 stamp Y %d、CellH %d，要 181、8", sc, e.cover.Y, e.cover.CellH)
		}
		if r := fo.Report()[0]; r.Y0 != 181 || r.Y1 != 189 || r.B0 != 181 {
			t.Fatalf("%d 倍：Report %+v", sc, r.FlightCover)
		}
		idx := cw.indexed()
		if bad := fo.Match(idx); len(bad) != 0 {
			t.Fatalf("%d 倍：畫面未變仍不符 %v", sc, bad)
		}
		i1 := append([]byte(nil), idx...)
		i1[181*320+e.cx0] = 99
		if bad := fo.Match(i1); !eqStrs(bad, []string{"NORM"}) {
			t.Fatalf("%d 倍：列 181 改一點，不符 %v", sc, bad)
		}
		i2 := append([]byte(nil), idx...)
		i2[189*320+e.cx0] = 99
		if bad := fo.Match(i2); len(bad) != 0 {
			t.Fatalf("%d 倍：列 189（band 外）改一點就不符 %v", sc, bad)
		}
	}
}

// ---- 第 18 項：實際字型 ----

// readTREFile 照 TRE 目錄格式（u32 筆數、u32、每筆 74 bytes：旗標、名稱 65 bytes、u32 位移、u32 長度）取一個檔。
func readTREFile(t *testing.T, path, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("讀不到 %s：%v", path, err)
	}
	n := int(binary.LittleEndian.Uint32(b))
	for k := 0; k < n; k++ {
		r := b[8+74*k : 8+74*(k+1)]
		nm := r[1:66]
		if i := bytes.IndexByte(nm, 0); i >= 0 {
			nm = nm[:i]
		}
		if strings.EqualFold(string(nm), name) {
			off, ln := binary.LittleEndian.Uint32(r[66:]), binary.LittleEndian.Uint32(r[70:])
			return b[off : off+ln]
		}
	}
	t.Fatalf("%s 沒有 %s", path, name)
	return nil
}

func TestFlightClipRealFont(t *testing.T) {
	root := os.Getenv("DOSGOLEM_SC_ROOT")
	if root == "" {
		t.Skip("DOSGOLEM_SC_ROOT 未設")
	}
	raw := readTREFile(t, filepath.Join(root, "BIGTRE.TRE"), `..\..\DATA\FONTS\SM-FONT.SHP`)
	if len(raw) != 5612 {
		t.Fatalf("SM-FONT 長 %d，要 5612", len(raw))
	}
	shp, err := parseFlightShp(raw)
	if err != nil || len(shp.ents) != 127 {
		t.Fatalf("SM-FONT 解析：%v、條目 %d", err, len(shp.ents))
	}
	var glyphs [][]Point
	for _, ch := range []byte("AIR") {
		g, err := shp.glyph(int(ch))
		if err != nil {
			t.Fatal(err)
		}
		glyphs = append(glyphs, g)
	}
	// 案例 Q 的 AIR：Xs 29、34、36，fy 5，左 MFD 窗 (17,121)-(96,186)（§2 MFD 墨跡列、RE 031 §2）。
	xs := []int{29, 34, 36}
	for _, c := range []struct{ y, ne, new int }{{177, 29, 29}, {182, 29, 24}, {183, 29, 19}, {187, 29, 0}} {
		e := flightExpectedInk(glyphs, xs, c.y, 5)
		b0, _ := flightBand(KindMFD, c.y)
		rows := make([]byte, 8*320)
		_, cl := flightClipInk("AIR", rows, rows, b0, e, leftWin)
		if len(e) != c.ne || cl.NE != c.ne || cl.NEW != c.new {
			t.Errorf("y′ %d：|E| %d、窗內 %d，要 %d、%d", c.y, len(e), cl.NEW, c.ne, c.new)
		}
	}
}
