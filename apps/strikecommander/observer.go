package strikecommander

import "fmt"

// View 是觀察器讀取的唯讀介面。session.StepView 以值接收者實作這些方法，直接滿足它；
// 單元測試以合成的記憶體與暫存器實作。View 只在一次 Observe 呼叫內有效，不得保留。
type View interface {
	Steps() uint64
	CS() uint16
	IP() uint16
	SS() uint16
	SP() uint16
	BP() uint16
	DS() uint16
	Read8(a uint32) uint8
}

// OPTTEST 的位址都寫成相對載入段 base 的段偏移與段內位移（001 §2.2）。
const (
	segText  = 0x3942 // puts、putc、格式化函式、輸出函式所在的段
	segBlit  = 0x39FB // blit
	segLabel = 0x0AB1 // 標籤常式

	ipPutsEntry = 0x034B
	ipPutsRet   = 0x03D3
	ipPutc      = 0x02B2
	ipBlitEntry = 0x0000
	ipBlitRet   = 0x008B
	ipFmtRet    = 0x061E // 格式化函式返回到輸出函式的位址
	ipLabelRet  = 0x082A // 輸出函式返回到標籤常式的位址
	ipLabelCall = 0x0819 // 標籤呼叫指令序列的起點

	// 字型 DS:D7E6 是唯一量過墨跡框的字型（001 §2.4）。
	measuredFont = 0xD7E6

	maxLabelBytes = 64
	maxPending    = 16
	// ResetSteps：可見集合非空、上次提交後經過這麼多步仍沒有配對的 blit 返回就清空（001 §2.5）。
	ResetSteps = 5_000_000
	maxLog     = 4096
)

// 物件 DS:obj 的欄位位移（RE 012 §2.1）。
const (
	objSeg   = 0x02 // 緩衝描述元：段
	objList  = 0x04 // 緩衝描述元：列表指標
	objX     = 0x10
	objY     = 0x12
	objFont  = 0x14
	descRect = 0x04 // 描述元 +4 起：x0、y0、x1、y1（含端點）
)

// sigRow 是一列簽章：段偏移 seg、段內位移 off 起的位元組，-1 表示由 base 決定（labelCallSeg）。
type sigRow struct {
	name  string
	seg   uint16
	off   uint16
	bytes []int
}

var sigPutsEntry = []int{0x55, 0x8B, 0xEC, 0x83, 0xEC, 0x06, 0x56, 0x57, 0x8B, 0x76, 0x06, 0x8B, 0x7E, 0x08, 0xFF, 0x74}

// baseRows 是 001 §2.2 表中除 puts 入口外的 6 列，加上 §2.3 前提的兩個 `push bp; mov bp, sp` 序言
// （格式化函式 3942:03D4、輸出函式 3942:05FF；檔案位移 3F3F4h、3F61Fh）。
var baseRows = []sigRow{
	{"puts 返回", segText, ipPutsRet - 3, []int{0x5F, 0x5E, 0xC9, 0xCB}},
	{"putc 入口", segText, ipPutc, []int{0x55, 0x8B, 0xEC, 0x83, 0xEC, 0x02, 0x56, 0x8B, 0x76, 0x06, 0x80, 0x7E, 0x08, 0x1F, 0x7E, 0x54}},
	{"blit 入口", segBlit, ipBlitEntry, []int{0x55, 0x8B, 0xEC, 0x83, 0xEC, 0x06, 0x56, 0x57, 0x1E, 0x06, 0xFC, 0x8B, 0x5E, 0x06, 0x8B, 0x76}},
	{"blit 返回", segBlit, ipBlitRet - 2, []int{0xE5, 0x5D, 0xCB}},
	{"格式化函式返回點", segText, ipFmtRet - 5, []int{0x56, 0x0E, 0xE8, 0xB6, 0xFD}},
	{"標籤呼叫點", segLabel, ipLabelCall, []int{0x68, 0xAF, 0x00, 0x57, 0x6A, 0x02, 0x68, 0x5D, 0x19, 0xFF, 0x74, 0x2D, 0x9A, 0xFF, 0x05, -1, -1, 0x83, 0xC4, 0x0A}},
	{"格式化函式序言", segText, 0x03D4, []int{0x55, 0x8B, 0xEC}},
	{"輸出函式序言", segText, 0x05FF, []int{0x55, 0x8B, 0xEC}},
}

// lin 把 段:位移 換成線性位址；位移以 16 位元環繞。
func lin(seg, off uint16) uint32 { return uint32(seg)<<4 + uint32(off) }

func rd16(v View, seg, off uint16) uint16 {
	return uint16(v.Read8(lin(seg, off))) | uint16(v.Read8(lin(seg, off+1)))<<8
}

// match 比對 seg:off 起的位元組；-1 的兩個位置依序等於 fill 的低、高位元組。
func match(v View, seg, off uint16, want []int, fill uint16) bool {
	wild := 0
	for i, w := range want {
		got := v.Read8(lin(seg, off+uint16(i)))
		if w < 0 {
			w = int(fill >> (8 * wild) & 0xFF)
			wild++
		}
		if int(got) != w {
			return false
		}
	}
	return true
}

// Label 是一筆熱點標籤（001 §2.4）。
type Label struct {
	Key   string // 字串的原始位元組（ASCII）
	X0    int    // 第一次相符 putc 入口時的游標 x
	X1    int    // puts 返回時的游標 x
	Y     int    // 第一次相符 putc 入口時的游標 y
	Font  uint16 // [DS:obj+14h]
	Seg   uint16 // [DS:obj+2]，緩衝段
	List  uint16 // [DS:obj+4]，緩衝描述元的列表指標
	Steps uint64 // puts 入口的步數

	ink inkCapture
}

// Box 回墨跡框 [X0, X1) × [Y+1, Y+15)。
func (l Label) Box() (x0, y0, x1, y1 int) { return l.X0, l.Y + 1, l.X1, l.Y + 15 }

func sameLabels(a, b []Label) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key != b[i].Key || a[i].X0 != b[i].X0 || a[i].X1 != b[i].X1 || a[i].Y != b[i].Y {
			return false
		}
	}
	return true
}

// Commit 是一次配對 blit 返回的提交（001 §2.5）。
type Commit struct {
	BlitEntry  uint64 // 配對的全螢幕 blit 入口步數
	BlitReturn uint64 // 返回步數
	Accepted   int    // 上次 blit 返回到這次 blit 入口之間接受的標籤呼叫數
	Ignored    int    // 同一區間忽略的非標籤 puts 呼叫數
	Labels     []Label
	Generation uint64
	Ink        []InkResult // 只在 LabelObserver.InkCheck 時有值，與 Labels 平行
}

// Event 是一筆紀錄（捨棄、停用、清空等原因）。
type Event struct {
	Steps  uint64
	Kind   string
	Detail string
}

type inflight struct {
	label    Label
	obj      uint16
	ss, sp   uint16
	gotFirst bool
}

// LabelObserver 依 001 §2.2 到 §2.5 觀察 OPTTEST 的熱點標籤。只讀 View，不寫入任何原版狀態。
type LabelObserver struct {
	// InkCheck 開啟 §2.8 的墨跡框核對（收據工具用；每筆標籤多讀約 6 KB）。
	InkCheck bool
	// CursorY 非負時是滑鼠游標中心的原版列；游標（11×11 十字）落在核對列範圍內時略過核對（§2.8）。
	// 觀察器看不到滑鼠位置，由呼叫端提供；負值表示未知（不略過）。
	CursorY int
	// OnCommit 在每次提交後呼叫（同一個 Observe 呼叫之內）。可為 nil。
	OnCommit func(Commit)

	// noCallerCheck 關掉 §2.3（只供 §3.3 的負對照）。
	noCallerCheck bool

	baseKnown bool
	base      uint16
	baseOK    bool

	cur     *inflight
	pending []Label
	visible []Label
	gen     uint64

	paired     bool
	pairSP     uint16
	pairEntry  uint64
	candidates []Label
	candInk    []InkResult
	snapAcc    int
	snapIgn    int
	accepted   int
	ignored    int
	lastCommit uint64

	commits      int
	partialBlits int
	log          []Event
	logDropped   int
}

// NewLabelObserver 建一個觀察器。
func NewLabelObserver() *LabelObserver { return &LabelObserver{CursorY: -1} }

// Base 回目前定位到的載入段與是否啟用。
func (o *LabelObserver) Base() (base uint16, ok bool) { return o.base, o.baseKnown && o.baseOK }

// Visible 回目前可見集合的複本。
func (o *LabelObserver) Visible() []Label { return append([]Label(nil), o.visible...) }

// Generation 在可見集合改變時加 1。
func (o *LabelObserver) Generation() uint64 { return o.gen }

// Pending 回待提交清單的複本。
func (o *LabelObserver) Pending() []Label { return append([]Label(nil), o.pending...) }

// Commits 回提交次數。
func (o *LabelObserver) Commits() int { return o.commits }

// PartialBlits 回沒有壓到可見墨跡框的非全螢幕 blit 次數。
func (o *LabelObserver) PartialBlits() int { return o.partialBlits }

// Log 回紀錄的複本與因上限捨棄的筆數。
func (o *LabelObserver) Log() ([]Event, int) { return append([]Event(nil), o.log...), o.logDropped }

func (o *LabelObserver) note(steps uint64, kind, format string, a ...any) {
	if len(o.log) >= maxLog {
		o.logDropped++
		return
	}
	o.log = append(o.log, Event{Steps: steps, Kind: kind, Detail: fmt.Sprintf(format, a...)})
}

// Observe 在每一步執行前呼叫。熱路徑只比較 IP。
func (o *LabelObserver) Observe(v View) {
	switch v.IP() {
	case ipPutsEntry:
		o.putsEntry(v)
	case ipPutsRet:
		if o.cur != nil {
			o.putsReturn(v)
		}
	case ipPutc:
		if o.cur != nil {
			o.putc(v)
		}
	case ipBlitEntry:
		if o.baseKnown && o.baseOK {
			o.blitEntry(v)
		}
	case ipBlitRet:
		if o.baseKnown && o.baseOK && o.paired {
			o.blitReturn(v)
		}
	}
	if len(o.visible) != 0 {
		if s := v.Steps(); s-o.lastCommit >= ResetSteps {
			o.visible = nil
			o.gen++
			o.note(s, "reset", "上次提交（第 %d 步）後 %d 步沒有配對的 blit 返回，可見集合清空", o.lastCommit, s-o.lastCommit)
		}
	}
}

func (o *LabelObserver) verifyBase(v View, base uint16) bool {
	for _, r := range baseRows {
		if !match(v, base+r.seg, r.off, r.bytes, base+segText) {
			o.note(v.Steps(), "base", "base %04X 停用：%s（%04X:%04X）簽章不符", base, r.name, base+r.seg, r.off)
			return false
		}
	}
	return true
}

func (o *LabelObserver) putsEntry(v View) {
	cs := v.CS()
	// 每次都讀簽章：SCCD 依序執行的程式可能重用同一段，不以 CS 快取（001 §2.2）。
	if !match(v, cs, ipPutsEntry, sigPutsEntry, 0) {
		if o.baseKnown && cs == o.base+segText {
			o.note(v.Steps(), "base", "base %04X 的 puts 入口簽章已不符，停用", o.base)
			o.baseKnown, o.baseOK = false, false
			o.cur, o.pending, o.paired = nil, nil, false
		}
		return
	}
	base := cs - segText
	if !o.baseKnown || base != o.base {
		o.base, o.baseKnown = base, true
		o.baseOK = o.verifyBase(v, base)
	}
	if !o.baseOK {
		return
	}
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	if !o.noCallerCheck && !o.isLabelCall(v, ss) {
		o.ignored++
		return
	}
	o.accepted++
	steps := v.Steps()
	obj := rd16(v, ss, sp+4)
	str := rd16(v, ss, sp+6)
	key, why := readLabelString(v, ds, str)
	if why != "" {
		o.note(steps, "string", "捨棄 DS:%04X 的字串：%s", str, why)
		return
	}
	if o.cur != nil {
		o.note(steps, "puts", "上一筆 %q 的 puts 沒有返回，捨棄", o.cur.label.Key)
	}
	l := Label{
		Key: key, Steps: steps,
		Font: rd16(v, ds, obj+objFont), Seg: rd16(v, ds, obj+objSeg), List: rd16(v, ds, obj+objList),
	}
	if o.InkCheck {
		l.ink.yPrime = int(rd16(v, ds, obj+objY))
		l.ink.before = readRows(v, ds, l.Seg, l.List, l.ink.yPrime-2, l.ink.yPrime+17)
	}
	o.cur = &inflight{label: l, obj: obj, ss: ss, sp: sp}
}

// isLabelCall 是 001 §2.3 的四個比較。puts 入口時 BP 是格式化函式的框架。
func (o *LabelObserver) isLabelCall(v View, ss uint16) bool {
	p := v.BP()
	if rd16(v, ss, p+2) != ipFmtRet {
		return false
	}
	if rd16(v, ss, p+4) != o.base+segText {
		return false
	}
	q := rd16(v, ss, p)
	if rd16(v, ss, q+2) != ipLabelRet {
		return false
	}
	if rd16(v, ss, q+4) != o.base+segLabel {
		return false
	}
	return true
}

// readLabelString 從 DS:str 起最多讀 64 bytes，第一個 00h 之前是字串；只接受 20h 到 7Eh。
func readLabelString(v View, ds, str uint16) (string, string) {
	buf := make([]byte, 0, maxLabelBytes)
	for i := 0; i < maxLabelBytes; i++ {
		c := v.Read8(lin(ds, str+uint16(i)))
		if c == 0 {
			for _, b := range buf {
				if b < 0x20 || b > 0x7E {
					return "", fmt.Sprintf("含 %02Xh", b)
				}
			}
			return string(buf), ""
		}
		buf = append(buf, c)
	}
	return "", "64 bytes 內沒有 00h"
}

func (o *LabelObserver) putc(v View) {
	if v.CS() != o.base+segText || o.cur.gotFirst {
		return
	}
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	if rd16(v, ss, sp+4) != o.cur.obj {
		return
	}
	o.cur.gotFirst = true
	o.cur.label.X0 = int(rd16(v, ds, o.cur.obj+objX))
	o.cur.label.Y = int(rd16(v, ds, o.cur.obj+objY))
}

func (o *LabelObserver) putsReturn(v View) {
	if v.CS() != o.base+segText || v.SS() != o.cur.ss || v.SP() != o.cur.sp {
		return
	}
	c := o.cur
	o.cur = nil
	if !c.gotFirst {
		o.note(v.Steps(), "empty", "%q 沒有相符的 putc，不記", c.label.Key)
		return
	}
	c.label.X1 = int(rd16(v, v.DS(), c.obj+objX))
	if len(o.pending) >= maxPending {
		o.note(v.Steps(), "pending", "待提交清單滿 %d 筆，捨棄最舊的 %q", maxPending, o.pending[0].Key)
		o.pending = append(o.pending[:0], o.pending[1:]...)
	}
	o.pending = append(o.pending, c.label)
}

type rect struct{ x0, y0, x1, y1 int } // 含端點

func readRect(v View, ds, desc uint16) rect {
	return rect{
		int(rd16(v, ds, desc+descRect)), int(rd16(v, ds, desc+descRect+2)),
		int(rd16(v, ds, desc+descRect+4)), int(rd16(v, ds, desc+descRect+6)),
	}
}

var fullScreen = rect{0, 0, 319, 199}

func (o *LabelObserver) blitEntry(v View) {
	if v.CS() != o.base+segBlit {
		return
	}
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	steps := v.Steps()
	src := rd16(v, ss, sp+4)
	dst := rd16(v, ss, sp+6)
	sr, dr := readRect(v, ds, src), readRect(v, ds, dst)
	if sr != fullScreen || dr != fullScreen {
		// 非全螢幕：不設配對旗標，待提交清單保留。
		for _, l := range o.visible {
			bx0, by0, bx1, by1 := l.Box()
			if dr.x0 < bx1 && bx0 < dr.x1+1 && dr.y0 < by1 && by0 < dr.y1+1 {
				o.visible = nil
				o.gen++
				o.note(steps, "partial", "非全螢幕 blit 目的 (%d,%d)-(%d,%d) 壓到可見墨跡框，可見集合清空", dr.x0, dr.y0, dr.x1, dr.y1)
				return
			}
		}
		o.partialBlits++
		return
	}
	if o.paired {
		o.note(steps, "blit", "前一次全螢幕 blit（第 %d 步）沒有配對的返回", o.pairEntry)
	}
	o.paired, o.pairSP, o.pairEntry = true, sp, steps
	bufSeg := rd16(v, ds, src)
	o.candidates = o.candidates[:0:0]
	for _, l := range o.pending {
		if l.Seg != bufSeg {
			o.note(steps, "seg", "%q 的緩衝段 %04X 不是 blit 來源 %04X，刪除", l.Key, l.Seg, bufSeg)
			continue
		}
		o.candidates = append(o.candidates, l)
	}
	o.pending = nil
	o.snapAcc, o.snapIgn = o.accepted, o.ignored
	o.accepted, o.ignored = 0, 0
	o.candInk = nil
	if o.InkCheck {
		for _, l := range o.candidates {
			o.candInk = append(o.candInk, o.inkCheck(v, ds, l))
		}
	}
}

func (o *LabelObserver) blitReturn(v View) {
	if v.CS() != o.base+segBlit || v.SP() != o.pairSP {
		return
	}
	steps := v.Steps()
	prev := o.visible
	o.visible = o.candidates
	o.candidates = nil
	if !sameLabels(prev, o.visible) {
		o.gen++
	}
	o.paired = false
	o.lastCommit = steps
	o.commits++
	if o.OnCommit != nil {
		o.OnCommit(Commit{
			BlitEntry: o.pairEntry, BlitReturn: steps, Accepted: o.snapAcc, Ignored: o.snapIgn,
			Labels: append([]Label(nil), o.visible...), Generation: o.gen, Ink: o.candInk,
		})
	}
	o.candInk = nil
}
