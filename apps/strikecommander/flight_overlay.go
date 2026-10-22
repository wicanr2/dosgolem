package strikecommander

import (
	"github.com/wicanr2/dosgolem/xlate"
)

// 飛行字串的遮蓋、背景與出圖比對（strike_commander 規格 009 §3.6.5、§3.8）；版面與字見 flight_text.go（011 §3.6.4 到 §3.6.6）。
// 每個倍率一個 FlightOverlay；遮蓋範圍是原版像素，文字是輸出像素。

// FlightCover 是一筆字串的遮蓋範圍 [X0, X1) × [Y0, Y1)（原版像素，§3.11）。
type FlightCover struct {
	Key            string
	Kind           Kind
	X0, Y0, X1, Y1 int
}

// FlightEntry 是 Report 的一筆（009 §3.11、011 §3.8）：遮蓋範圍加版面、字色、外框與前後列、Ref。
// Px 以下是輸出像素；Ink 是最近一次 Draw 的文字框。
type FlightEntry struct {
	FlightCover
	CatKey                    string
	Text                      string // N() 前的譯文
	K                         int
	Xs                        []int
	TextX0, TextX1, Y, B0, BH int // 原版的 X0、X1、Y 與 band
	FG                        uint8
	Outline                   bool
	Hidden                    bool
	Before, After, Ref        []byte

	Display             string // N() 後實際畫的字串
	Px, TxOut, Baseline int
	Clip, Ink           xlate.AlphaRect
	E0, E1              int
}

type flightEntry struct {
	idx      int // 在最近一次 Sync 的 texts 中的索引
	t        FlightText
	text     []rune
	cx0, cx1 int
	fg       uint8
	outline  bool
	cover    *xlate.Stamp
	ti       *textItem
}

// FlightOverlay 依可見集合規劃飛行字串並出圖（NewFlightOverlayText）。
type FlightOverlay struct {
	cats  FlightCatalogs
	scale int

	gen     uint64
	built   bool
	entries []flightEntry
	lastBad []bool

	// Text 是 011 的圖集。lastInk 與 entries 平行。
	Text    *TextFonts
	lastInk []xlate.AlphaRect

	Hidden []string
	Misses []Miss
	Skips  []Skip
}

// Scale 回倍率。
func (o *FlightOverlay) Scale() int { return o.scale }

// Sync 在 generation 改變時依可見集合重建 stamp；generation 沒變時不重建（Misses、Skips 不再追加），
// 以這次的 texts（同索引、同種類與 catalog key）更新觀察字串、前後列、Ref、Xs、X1 與由它們算出的版面與字色。
func (o *FlightOverlay) Sync(texts []FlightText, gen uint64) {
	if o.built && gen == o.gen {
		for i := range o.entries {
			e := &o.entries[i]
			if e.idx >= len(texts) {
				continue
			}
			t := texts[e.idx]
			if t.Kind != e.t.Kind || t.CatKey != e.t.CatKey {
				continue
			}
			if ne, miss, skip := o.plan(e.idx, t); miss == nil && skip == nil {
				*e = ne
			}
		}
		return
	}
	o.built, o.gen = true, gen
	o.entries, o.lastBad, o.lastInk = nil, nil, nil
	for i, t := range texts {
		e, miss, skip := o.plan(i, t)
		switch {
		case miss != nil:
			o.Misses = append(o.Misses, *miss)
		case skip != nil:
			if skip.Reason != "" {
				o.Skips = append(o.Skips, *skip)
			}
		default:
			o.entries = append(o.entries, e)
		}
	}
}

// errNoStamp 表示不產生 stamp 且不記 Skip（1 倍、HUD 在 2 倍，§3.7.1）。
var errNoStamp = &Skip{}

// plan 規劃一筆的遮蓋 stamp 與文字項。不產生時回 miss 或 skip（skip.Reason 為空表示不記）。
func (o *FlightOverlay) plan(i int, t FlightText) (flightEntry, *Miss, *Skip) {
	return o.planText(i, t)
}

// hudRange 回 §3.7.1 表的左界 L、右界 R（含）。
func hudRange(t FlightText) (l, r int, ok bool) {
	switch t.CatKey {
	case "GEAR":
		return 183, 213, true
	case "FLAPS":
		return 183, 212, true
	case "BRAKE":
		return 183, 211, true
	case "DGFT", "STRF", "SRM":
		return 118, 149, true
	case "NONE":
		return t.Anchor(), 149, true
	}
	return 0, 0, false
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func clampInt(v, lo, hi int) int { return max(lo, min(v, hi)) }

// inkXRange 回 band 內前列 ≠ 後列的點的 x 範圍 [ix0, ix1]（§3.7.3）。
func inkXRange(t FlightText) (ix0, ix1 int, ok bool) {
	ix0, ix1 = 320, -1
	for i := range t.After {
		if i < len(t.Before) && t.Before[i] != t.After[i] {
			x := i % 320
			ix0, ix1 = min(ix0, x), max(ix1, x)
		}
	}
	return ix0, ix1, ix1 >= 0
}

// inkMode 是 §3.8 第 2 條的字色：[cx0, cx1) 內、前列 ≠ 後列的點在後列的色號眾數，同數取小。n 是墨跡點數。
func inkMode(t FlightText, cx0, cx1 int) (fg uint8, n int) {
	var hist [256]int
	for r := 0; r < t.BH; r++ {
		for x := max(cx0, 0); x < min(cx1, 320); x++ {
			i := r*320 + x
			if t.Before[i] != t.After[i] {
				hist[t.After[i]]++
				n++
			}
		}
	}
	best := -1
	for c := 0; c < 256; c++ {
		if hist[c] > 0 && (best < 0 || hist[c] > hist[best]) {
			best = c
		}
	}
	if best < 0 {
		return 0, 0
	}
	return uint8(best), n
}

// flightMismatch 是 §3.6.5 的逐筆比對：遮蓋範圍（裁到畫面）內 indexed 逐點等於 Ref 的對應列（列位移 y − b0）。
// 不改既有的 mismatched（它寫死 y − Y − 1）。
func (o *FlightOverlay) flightMismatch(indexed []byte) []bool {
	bad := make([]bool, len(o.entries))
	for i, e := range o.entries {
		t := e.t
		if len(indexed) < 64000 || t.Ref == nil || len(t.Ref) != t.BH*320 {
			bad[i] = true
			continue
		}
	scan:
		for y := max(t.B0, 0); y < min(t.B0+t.BH, 200); y++ {
			row := (y - t.B0) * 320
			for x := e.cx0; x < e.cx1; x++ {
				if indexed[y*320+x] != t.Ref[row+x] {
					bad[i] = true
					break scan
				}
			}
		}
	}
	return bad
}

// Match 回比對不符的觀察字串（不出圖，不改 Hidden）。
func (o *FlightOverlay) Match(indexed []byte) []string {
	var out []string
	for i, b := range o.flightMismatch(indexed) {
		if b {
			out = append(out, o.entries[i].t.Key)
		}
	}
	return out
}

// Draw 在 out（原版放大圖，已畫過 OPTTEST 的 Overlay.Draw）上就地畫飛行字串（011 §3.9 的合成順序）：
// 逐筆比對，不符的不畫、記入 Hidden；照畫的遮蓋先以背景圖換回 → 照畫 HUD 串的外框層 → 照畫串的填色層（drawText）。
func (o *FlightOverlay) Draw(out []byte, indexed []byte, palette [256][3]uint8) {
	bad := o.flightMismatch(indexed)
	o.lastBad = bad
	o.Hidden = nil
	if len(o.entries) == 0 || o.scale < 2 {
		o.lastInk = nil
		return
	}
	o.drawText(out, indexed, palette, bad)
}

// Covers 回目前每筆（含 Hidden）的遮蓋範圍，供 CheckComposite 的允許區。
func (o *FlightOverlay) Covers() []FlightCover {
	var out []FlightCover
	for _, e := range o.entries {
		out = append(out, FlightCover{Key: e.t.Key, Kind: e.t.Kind, X0: e.cx0, Y0: e.t.B0, X1: e.cx1, Y1: e.t.B0 + e.t.BH})
	}
	return out
}

// Report 依重建順序列出每筆（FlightEntry），Hidden 與 Ink 取最近一次 Draw。切片是複本。
func (o *FlightOverlay) Report() []FlightEntry {
	var out []FlightEntry
	for i, e := range o.entries {
		t, ti := e.t, e.ti
		fe := FlightEntry{
			FlightCover: FlightCover{Key: t.Key, Kind: t.Kind, X0: e.cx0, Y0: t.B0, X1: e.cx1, Y1: t.B0 + t.BH},
			CatKey:      t.CatKey, Text: string(e.text), K: t.K, Xs: append([]int(nil), t.Xs...),
			TextX0: t.X0, TextX1: t.X1, Y: t.Y, B0: t.B0, BH: t.BH, FG: e.fg, Outline: e.outline,
			Before: append([]byte(nil), t.Before...), After: append([]byte(nil), t.After...), Ref: append([]byte(nil), t.Ref...),
			Display: string(ti.display), Px: ti.px, TxOut: ti.tx, Baseline: ti.baseline, Clip: ti.clip, E0: ti.e0, E1: ti.e1,
		}
		if i < len(o.lastInk) {
			fe.Ink = o.lastInk[i]
		}
		if i < len(o.lastBad) {
			fe.Hidden = o.lastBad[i]
		}
		out = append(out, fe)
	}
	return out
}
