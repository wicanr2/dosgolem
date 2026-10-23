package strikecommander

import (
	"fmt"
	"slices"
	"strings"
)

// MFD 字串的裁切窗核對（strike_commander 規格 009 第 5 版 §3.2.1 第 1、3 條、§3.2.4、§3.5；RE 031）。
//
// 字形方法依字串物件的裁切窗 [obj+6, obj+0Ah] × [obj+8, obj+0Ch]（四邊含等號、畫面絕對座標）逐列逐像素裁切，
// 整串落在窗外時原版什麼都不畫（RE 031 §2、§3）。新路徑以 SHP 字形算出預期墨跡像素 E，與窗、band、前後列差異分類。
// 只有 mfd 走新路徑；hud、plaque 與 font = 0、窗全 0 的 mfd 走舊路徑（flightInk 加 inkInBox，不改）。

// 字串物件的窗與字型物件的欄位位移（RE 031 §3；規格 009 §2 字型物件列）。
const (
	objWinX0 = 0x06
	objWinY0 = 0x08
	objWinX1 = 0x0A
	objWinY1 = 0x0C

	fontVTable  = 0x00 // 方法表
	fontPtrOff  = 0x02 // SHP（PAK）遠指標：位移
	fontPtrSeg  = 0x04 // SHP（PAK）遠指標：段
	fontKind    = 0x06 // byte
	fontYOffset = 0x0E // 字形方法加到 y 的值（有號）

	fontVTableWant = 0x3FDA
	fontKindWant   = 0x02

	shpTotalMin = 8
	shpTotalMax = 0x10000

	// MFD 螢幕內區的上、下界列（§2 左、右 MFD 內區列；§3.5 T、Bt）。
	mfdInnerTop    = 116
	mfdInnerBottom = 188

	maxClipped = 4096
)

// 新路徑的分類（§3.2.4.2）。
const (
	ClipPass    = "pass"
	ClipClipped = "clipped"
	ClipInk     = "ink"
)

// FlightClip 是一筆 mfd 新路徑字串的核對結果（§3.11 第 5 版）。舊路徑的字串為 nil。
type FlightClip struct {
	Window        [4]int // x0, y0, x1, y1（四邊含等號）
	FY            int    // [font+0Eh]
	NE, NEW, NEWB int    // |E|、|E ∩ W|、|E ∩ W ∩ band|
	Class         string // ClipPass、ClipClipped、ClipInk
	Shifted       bool   // B0 經 §3.5 往內推過
}

// FlightClipEvent 是一筆整串裁切（§3.11 第 5 版）。Steps 是 puts 返回時的步數（與 log 相同）。
type FlightClipEvent struct {
	Steps     uint64
	Key       string
	Y, YPrime int
	Window    [4]int
	FY        int
	NE, NEW   int
	Diffs     int
	NXs       int
	X0, X1    int
}

// flightGlyph 是一張 shape 寫入過的點，座標是 (dx + k, dy)（§3.2.4.1）。
type flightGlyph struct {
	pts []Point
	err error
}

// flightShp 是一份 SHP（PAK）的位元組與已解碼字形（§3.2.4.3 的快取值）。鍵是 (l, total)。
type flightShp struct {
	l      uint32
	total  int
	raw    []byte
	ents   [][2]int
	glyphs map[int]flightGlyph
}

// parseFlightShp 解析一份 SHP（PAK 外層）。字形在第一次用到時才解碼。
func parseFlightShp(raw []byte) (*flightShp, error) {
	ents, err := pakEntries(raw)
	if err != nil {
		return nil, err
	}
	return &flightShp{total: len(raw), raw: raw, ents: ents, glyphs: map[int]flightGlyph{}}, nil
}

// glyph 回第 ch 張 shape 寫入過的點。超出條目數的字回 nil（貢獻 0 點，§3.2.4.1）。
func (s *flightShp) glyph(ch int) ([]Point, error) {
	if ch < 0 || ch >= len(s.ents) {
		return nil, nil
	}
	if g, ok := s.glyphs[ch]; ok {
		return g.pts, g.err
	}
	b := s.raw[s.ents[ch][0]:s.ents[ch][1]]
	sh, mask, err := decodeShapeMask(b)
	var pts []Point
	if err == nil {
		left, top := shapeOrigin(b)
		for i, m := range mask {
			if m {
				pts = append(pts, Point{i%sh.W - left, i/sh.W - top})
			}
		}
	}
	s.glyphs[ch] = flightGlyph{pts, err}
	return pts, err
}

// same 比對記憶體與快取：PAK 前 8 bytes 與字串 str 用到的每個條目（§3.2.4.3）。
func (s *flightShp) same(v View, str string) bool {
	for i := 0; i < 8; i++ {
		if v.Read8(s.l+uint32(i)) != s.raw[i] {
			return false
		}
	}
	var seen [256]bool
	for i := 0; i < len(str); i++ {
		ch := int(str[i])
		if seen[ch] || ch >= len(s.ents) {
			continue
		}
		seen[ch] = true
		for j := s.ents[ch][0]; j < s.ents[ch][1]; j++ {
			if v.Read8(s.l+uint32(j)) != s.raw[j] {
				return false
			}
		}
	}
	return true
}

// readFlightShp 讀線性位址 l 的 SHP（§3.2.4.3）：快取相符就沿用，否則整份重讀重解。失敗回原因。
func (o *FlightObserver) readFlightShp(v View, l uint32, str string) (*flightShp, string) {
	total := int(uint32(v.Read8(l)) | uint32(v.Read8(l+1))<<8 | uint32(v.Read8(l+2))<<16 | uint32(v.Read8(l+3))<<24)
	if total < shpTotalMin || total > shpTotalMax {
		return nil, fmt.Sprintf("SHP（線性 %05X）總長 %d 不在 [%d, %d]", l, total, shpTotalMin, shpTotalMax)
	}
	if c := o.shp; c != nil && c.l == l && c.total == total && c.same(v, str) {
		return c, ""
	}
	raw := make([]byte, total)
	for i := range raw {
		raw[i] = v.Read8(l + uint32(i))
	}
	c, err := parseFlightShp(raw)
	if err != nil {
		return nil, fmt.Sprintf("SHP（線性 %05X）解析失敗：%v", l, err)
	}
	c.l = l
	o.shp = c
	return c, ""
}

// flightClipIn 是新路徑在 puts 入口記下的窗、字型與逐字字形（§3.2.1 第 1 條）。
type flightClipIn struct {
	raw     [4]uint16 // +6..+0Ch 原值（返回時重讀比對）
	win     [4]int    // 有號
	fy      int
	glyphs  [][]Point // 與字串逐字平行
	shifted bool
}

func readWinRaw(v View, ds, obj uint16) [4]uint16 {
	return [4]uint16{rd16(v, ds, obj+objWinX0), rd16(v, ds, obj+objWinY0), rd16(v, ds, obj+objWinX1), rd16(v, ds, obj+objWinY1)}
}

// mfdClipEntry 是 §3.2.1 第 1 條的路徑判定（只對 mfd、解析通過之後）。回 nil 表示舊路徑：font = 0 或窗全 0 時靜默；
// 字型物件、窗或 SHP 不合法時計 InkGlyphFallback、記 log。
func (o *FlightObserver) mfdClipEntry(v View, steps uint64, ds, obj uint16, key string) *flightClipIn {
	raw := readWinRaw(v, ds, obj)
	font := rd16(v, ds, obj+objFont)
	if font == 0 || raw == [4]uint16{} {
		return nil
	}
	fallback := func(format string, a ...any) *flightClipIn {
		o.counts.InkGlyphFallback++
		o.note(steps, "inkfb", "%q 改用舊規則：%s", key, fmt.Sprintf(format, a...))
		return nil
	}
	vt, kind := rd16(v, ds, font+fontVTable), v.Read8(lin(ds, font+fontKind))
	off, seg := rd16(v, ds, font+fontPtrOff), rd16(v, ds, font+fontPtrSeg)
	if vt != fontVTableWant || kind != fontKindWant || (off == 0 && seg == 0) {
		return fallback("字型物件 DS:%04X 不合法（+0 %04X、+6 %02X、+4:+2 %04X:%04X）", font, vt, kind, seg, off)
	}
	win := [4]int{int(int16(raw[0])), int(int16(raw[1])), int(int16(raw[2])), int(int16(raw[3]))}
	if !(0 <= win[0] && win[0] <= win[2] && win[2] <= 319 && 0 <= win[1] && win[1] <= win[3] && win[3] <= 199) {
		return fallback("窗 (%d,%d)-(%d,%d) 不合法", win[0], win[1], win[2], win[3])
	}
	fy := int(int16(rd16(v, ds, font+fontYOffset)))
	shp, why := o.readFlightShp(v, lin(seg, off), key)
	if why != "" {
		return fallback("%s", why)
	}
	c := &flightClipIn{raw: raw, win: win, fy: fy, glyphs: make([][]Point, len(key))}
	for i := 0; i < len(key); i++ {
		pts, err := shp.glyph(int(key[i]))
		if err != nil {
			return fallback("SHP 第 %d 張 shape 解析失敗：%v", key[i], err)
		}
		c.glyphs[i] = pts
	}
	return c
}

// mfdClipBand 是 §3.5 第 5 版的 mfd band：E 的列集合（裁到畫面）與 [y0, y1] 相交但不是子集時，
// B0 = clamp(Y − 1, T, Bt − BH + 1)；其他照表。y 是有號的 y′。列集合只依字串、字形、Y、fy，不需要 Xs。
func mfdClipBand(c *flightClipIn, y int) (b0, bh int, shifted bool) {
	b0, bh = flightBand(KindMFD, y)
	in, out := false, false
	for _, g := range c.glyphs {
		for _, p := range g {
			r := y + c.fy + p.Y
			if r < 0 || r >= 200 {
				continue
			}
			if r >= c.win[1] && r <= c.win[3] {
				in = true
			} else {
				out = true
			}
		}
	}
	if in && out {
		nb := clampInt(y-1, mfdInnerTop, mfdInnerBottom-bh+1)
		shifted = nb != b0
		b0 = nb
	}
	return b0, bh, shifted
}

// flightExpectedInk 回 §3.2.4.1 的 E：第 i 字的點平移到 (xs[i], y + fy)，裁到畫面 [0, 320) × [0, 200)，去重。
// 回傳依畫面位址遞增排列。
func flightExpectedInk(glyphs [][]Point, xs []int, y, fy int) []Point {
	set := map[int]bool{}
	for i, g := range glyphs {
		if i >= len(xs) {
			break
		}
		for _, p := range g {
			x, yy := xs[i]+p.X, y+fy+p.Y
			if x < 0 || x >= 320 || yy < 0 || yy >= 200 {
				continue
			}
			set[yy*320+x] = true
		}
	}
	addrs := make([]int, 0, len(set))
	for a := range set {
		addrs = append(addrs, a)
	}
	slices.Sort(addrs)
	out := make([]Point, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, Point{a % 320, a / 320})
	}
	return out
}

// flightClipInk 是 §3.2.4.2 的分類（|E| > 0 時；|E| = 0 由呼叫端改用舊規則）。band 是 [b0, b0 + len/320)。
// InkResult 的 Diffs 是 |D|，Outside 是 D 不在 E ∩ W ∩ band 的點數；三緣不使用。
func flightClipInk(key string, before, after []byte, b0 int, e []Point, win [4]int) (InkResult, FlightClip) {
	r := InkResult{Key: key}
	var c FlightClip
	if before == nil || after == nil || len(before) != len(after) {
		r.Skipped = "前後列缺或長度不同"
		c.Class = ClipInk
		return r, c
	}
	bh := len(after) / 320
	c.NE = len(e)
	ewb := map[int]bool{}
	for _, p := range e {
		if p.X < win[0] || p.X > win[2] || p.Y < win[1] || p.Y > win[3] {
			continue
		}
		c.NEW++
		if p.Y >= b0 && p.Y < b0+bh {
			c.NEWB++
			ewb[p.Y*320+p.X] = true
		}
	}
	for i := range after {
		if after[i] == before[i] {
			continue
		}
		x, y := i%320, b0+i/320
		r.Diffs++
		if !ewb[y*320+x] {
			r.Outside++
			if len(r.OutsideAt) < 64 {
				r.OutsideAt = append(r.OutsideAt, Point{x, y})
			}
		}
	}
	switch {
	case c.NEW == 0 && r.Diffs == 0:
		c.Class = ClipClipped
	case c.NEW == 0, c.NEWB == 0, r.Diffs == 0, r.Outside != 0:
		c.Class = ClipInk
	default:
		c.Class = ClipPass
	}
	return r, c
}

// putsReturnClip 是 §3.2.1 第 3 條第 3 款（新路徑）：重讀窗、算 E、分類。前後列已讀，作廢檢查已過。
func (o *FlightObserver) putsReturnClip(v FlightView, steps uint64, c *flightPuts, t FlightText) {
	ci := c.clip
	if w := readWinRaw(v, v.DS(), c.obj); w != ci.raw {
		o.counts.SkipVoid++
		o.note(steps, "void", "%q 的窗在返回時 %04X 不等於入口 %04X，作廢", t.Key, w, ci.raw)
		return
	}
	e := flightExpectedInk(ci.glyphs, t.Xs, int(int16(uint16(t.Y))), ci.fy)
	if len(e) == 0 {
		o.counts.InkGlyphFallback++
		o.note(steps, "inkfb", "%q 改用舊規則：預期墨跡像素 0 點", t.Key)
		x0, y0, x1, y1 := t.InkBox()
		t.Ink = flightInk(t.Key, t.Before, t.After, t.B0, x0, y0, x1, y1)
		if !inkInBox(t.Ink) {
			o.counts.SkipInk++
			o.note(steps, "ink", "墨跡框核對不通過：%s", t.Ink)
			return
		}
		o.addPending(steps, t)
		return
	}
	ink, cl := flightClipInk(t.Key, t.Before, t.After, t.B0, e, ci.win)
	cl.Window, cl.FY, cl.Shifted = ci.win, ci.fy, ci.shifted
	t.Ink = ink
	switch cl.Class {
	case ClipClipped:
		o.counts.SkipClipped++
		ev := FlightClipEvent{Steps: steps, Key: t.Key, Y: t.Y, YPrime: c.yPrime, Window: ci.win, FY: ci.fy,
			NE: cl.NE, NEW: cl.NEW, Diffs: ink.Diffs, NXs: len(t.Xs), X0: t.X0, X1: t.X1}
		if len(o.clipped) >= maxClipped {
			o.clipDropped++
		} else {
			o.clipped = append(o.clipped, ev)
		}
		o.note(steps, "clip", "%q 整串在窗 (%d,%d)-(%d,%d) 外（y′ %d、fy %d、預期 %d 點、差異 0），原版沒有畫", t.Key,
			ci.win[0], ci.win[1], ci.win[2], ci.win[3], c.yPrime, ci.fy, cl.NE)
		return
	case ClipInk:
		o.counts.SkipInk++
		o.note(steps, "ink", "墨跡框核對不通過（窗 (%d,%d)-(%d,%d)、預期 %d 點、窗內 %d、band 內 %d）：%s", ci.win[0], ci.win[1], ci.win[2], ci.win[3],
			cl.NE, cl.NEW, cl.NEWB, ink)
		return
	}
	if cl.NEW != cl.NE {
		o.counts.ClipPartialWindow++
	}
	if cl.NEWB != cl.NEW {
		o.counts.ClipPartialBand++
	}
	if cl.Shifted {
		o.counts.BandShifted++
	}
	t.Clip = &cl
	o.addPending(steps, t)
}

// Clipped 回整串裁切事件的複本與因上限（4096 筆）捨棄的筆數（§3.11 第 5 版）。不與 log 共用上限。
func (o *FlightObserver) Clipped() ([]FlightClipEvent, int) {
	return append([]FlightClipEvent(nil), o.clipped...), o.clipDropped
}

// flightClippedHeader 是 flight-clipped.tsv 的欄名，順序照 FlightClipEvent；window 以逗號分隔 x0,y0,x1,y1。
var flightClippedHeader = []string{"steps", "key", "y", "yprime", "window", "fy", "ne", "new", "diffs", "nxs", "x0", "x1"}

// FlightClippedTSV 是 flight-clipped.tsv 的內容（§3.11 第 5 版）：每筆整串裁切一列，第一列是欄名。
func FlightClippedTSV(es []FlightClipEvent) []byte {
	var b strings.Builder
	b.WriteString(strings.Join(flightClippedHeader, "\t") + "\n")
	for _, e := range es {
		fmt.Fprintf(&b, "%d\t%s\t%d\t%d\t%d,%d,%d,%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n", e.Steps, e.Key, e.Y, e.YPrime,
			e.Window[0], e.Window[1], e.Window[2], e.Window[3], e.FY, e.NE, e.NEW, e.Diffs, e.NXs, e.X0, e.X1)
	}
	return []byte(b.String())
}
