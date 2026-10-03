package phantasie

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// 唯讀鉤子（docs/spec/001 §3、002 §3）。鉤子只讀原版記憶體與暫存器，把擷取到的資料送進 Overlay。
// 簽章驗證與安裝時機見 001 §3.1：安裝在鏈載入完成後立即進行，驗證在任一鉤子第一次於視訊模式 04h 觸發時一次完成。

const (
	// OffDrawDone 是 B：25A5 的共用結尾（視訊寫入已全部完成）。
	OffDrawDone = 0x277D
	// OffDrawEntry 是 25A5 入口，只作簽章檢查。
	OffDrawEntry = 0x25A5
	// OffStrcat 是 strcat(dest, src) 入口（T）。
	OffStrcat = 0x4DEF
	// ResidentEnd 是常駐碼結尾（映像偏移，不含）：image_hash 的範圍。
	ResidentEnd = 0x53EA
	// OffFont 與 FontSize：原版字模 FONT 在 DGROUP 內的位置（001 §8）。
	OffFont  = 0x3244
	FontSize = 2032

	maxFmtBytes = 64
	maxArgBytes = 80
	maxDestScan = 256
)

// DGROUP 內頁面緩衝區的描述（002 §2）。
const (
	offBufSeg = 0x5BAA
	offBuf1   = 0x5BAC
	offBuf2   = 0x5BAE
	pageBytes = 0x4000
	row24Len  = 0x140
)

// memView 是鉤子需要的唯讀記憶體存取。DGROUP 偏移；SS 等於 DS（001 §2，每個事件已驗證）所以堆疊也用它。
type memView interface {
	Bytes(off uint16, n int) []byte
	Word(off uint16) uint16
	Far(seg, off uint16, n int) []byte
}

type oracleMem struct {
	o  *oracle.Oracle
	dg uint16
}

func (m oracleMem) Bytes(off uint16, n int) []byte { return m.o.Bytes(oracle.Far(m.dg, off), n) }
func (m oracleMem) Word(off uint16) uint16         { return m.o.Word(oracle.Far(m.dg, off)) }
func (m oracleMem) Far(seg, off uint16, n int) []byte {
	return m.o.Bytes(oracle.Far(seg, off), n)
}

type sigCheck struct {
	name string
	off  uint16
	want []byte
}

// signatures 是 001 §3.1 與 002 §3 的位元組簽章（解壓後映像）。
var signatures = []sigCheck{
	{"A", OffDrawFormatted, []byte{0x8B, 0x46, 0x06}},
	{"B", OffDrawDone, []byte{0x8B, 0xC6, 0x5E, 0x5F}},
	{"L", OffOverlayLoad, []byte{0x8B, 0xDC, 0x8B, 0x47, 0x02}},
	{"S", OffSprintf, []byte{0x55, 0x8B, 0xEC}},
	{"T", OffStrcat, []byte{0x55, 0x8B, 0xEC, 0xBA, 0xFF, 0x7F}},
	{"25A5", OffDrawEntry, []byte{0x55, 0x8B, 0xEC}},
}

// opHook 是 002 §3 的一個畫面操作掛點：入口存參數，完成時套用。
type opHook struct {
	name        string
	entry, exit uint16
}

var opHooks = []opHook{
	{"int86", OffInt86, 0x4EE0},
	{"invert", 0x0672, 0x073F},
	{"invert2", 0x0740, 0x0799},
	{"save1", 0x0D30, 0x0D4F},
	{"copy12", 0x0D50, 0x0D70},
	{"load1", 0x0CF0, 0x0D0F},
	{"load2", 0x0D10, 0x0D2F},
	{"row24", 0x372D, 0x376D},
}

type opPending struct {
	intNo          uint8
	ax, bx, cx, dx uint16
	row, col, wid  int
	h, hNew        uint64
	pre            []byte
}

// Hooks 把原版的唯讀鉤子接到一個 Overlay。
type Hooks struct {
	Ov      *Overlay
	Regions *RegionTable
	Compose *ComposeStore

	Img, DG   uint16
	ImageHash string // 解壓後映像雜湊（SHA-256，img:0000 至 img:53EA），驗證成功後有值
	FontHash  string
	Diag      string // 空字串：尚未驗證；"ok"；或停用原因

	// Int10Probe 非 nil 時，每次 INT 10h AH=06h 或 07h 完成時收到入口參數與入口、完成時的視訊記憶體
	// （B800:0000 起 4000h bytes）。證據用（dosgolem 規格 250 §5 第 4 項的同狀態收據），不影響機器。
	Int10Probe func(ax, bx, cx, dx uint16, pre, post []byte)

	// NoStrcat 為真時 T 掛點（strcat）不處理：故障注入，001 §10 第 5 項的負對照（戰鬥指令列的組句應失敗）。
	NoStrcat bool

	mem   memView
	mode  func() uint8
	armed bool
	dead  bool

	overlay     string
	fontChecked bool
	writes      []VideoWrite // 開啟中事件的視訊寫入足跡（位置 oracle 用）
	pend        map[string]*opPending
}

// NewHooks 建立鉤子狀態（不安裝）。mem 與 mode 可由測試以假資料提供。
func NewHooks(ov *Overlay, regions *RegionTable, img, dg uint16, mem memView, mode func() uint8) *Hooks {
	return &Hooks{Ov: ov, Regions: regions, Compose: NewComposeStore(), Img: img, DG: dg, mem: mem, mode: mode,
		pend: map[string]*opPending{}}
}

// Busy 回是否有畫面操作在入口與完成之間（002 §3）：這段時間畫面是暫態，稽核抽樣要略過（005 §5.1）。
func (h *Hooks) Busy() bool { return len(h.pend) > 0 }

// Armed 回簽章驗證是否已通過；Failed 回是否已永久停用。
func (h *Hooks) Armed() bool  { return h.armed }
func (h *Hooks) Failed() bool { return h.dead }

// InstallHooks 在映像段 img 上安裝全部掛點，回傳 Hooks。掛點全部唯讀（不改暫存器與記憶體）。
func InstallHooks(o *oracle.Oracle, img uint16, ov *Overlay, regions *RegionTable) *Hooks {
	dg := img + DGroupParas
	h := NewHooks(ov, regions, img, dg, oracleMem{o, dg}, o.VideoMode)
	at := func(off uint16, fn func(*oracle.Oracle)) { o.OnCall(oracle.Far(img, off), fn) }
	at(OffOverlayLoad, func(o *oracle.Oracle) {
		if h.dead {
			return
		}
		h.overlay = string(cstr(o, dg, o.StackWord(1), 16))
	})
	at(OffDrawFormatted, func(o *oracle.Oracle) {
		h.handleA(aRegs{o.Steps(), o.SP(), o.BP(), o.DI(), o.DSReg() == dg})
	})
	at(OffDrawDone, func(o *oracle.Oracle) { h.handleB() })
	at(OffMemMove, func(o *oracle.Oracle) {
		if !h.armed || h.dead {
			return
		}
		if _, open := h.Ov.Open(); !open {
			return
		}
		if seg := o.StackWord(4); seg == 0xB800 || seg == 0xBA00 {
			h.writes = append(h.writes, VideoWrite{Step: o.Steps(), Caller: o.StackWord(0), Seg: seg, Off: o.StackWord(3), Count: o.StackWord(5)})
		}
	})
	at(OffSprintf, func(o *oracle.Oracle) { h.handleS(o.Steps(), o.SP()) })
	at(OffStrcat, func(o *oracle.Oracle) { h.handleT(o.SP()) })
	for _, op := range opHooks {
		op := op
		at(op.entry, func(o *oracle.Oracle) { h.opEntry(op.name, o.SP()) })
		at(op.exit, func(o *oracle.Oracle) { h.opExit(op.name) })
	}
	return h
}

// gate 判斷鉤子是否可處理：停用或未 armed 且視訊模式不是 04h 時忽略（計 prearm）；
// 第一次於模式 04h 觸發時做簽章驗證。just 為真表示這次觸發剛完成驗證。
func (h *Hooks) gate() (ok, just bool) {
	if h.dead {
		return false, false
	}
	if h.armed {
		return true, false
	}
	if h.mode() != 0x04 {
		h.Ov.C.Inc("prearm")
		return false, false
	}
	h.verify()
	if h.dead {
		return false, false
	}
	return true, true
}

func (h *Hooks) fail(msg string) {
	h.dead = true
	h.Diag = msg
}

// verify 一次性驗證 001 §3.1 與 002 §3 的所有掛點簽章，並記錄 image_hash 與快取 FONT。
func (h *Hooks) verify() {
	for _, s := range signatures {
		if got := h.mem.Far(h.Img, s.off, len(s.want)); !bytes.Equal(got, s.want) {
			h.fail(fmt.Sprintf("掛點 %s（映像 %04X）簽章不符：期望 % X，實際 % X", s.name, s.off, s.want, got))
			return
		}
	}
	for _, op := range opHooks {
		if got := h.mem.Far(h.Img, op.entry, 3); !bytes.Equal(got, []byte{0x55, 0x8B, 0xEC}) {
			h.fail(fmt.Sprintf("掛點 %s 入口（映像 %04X）簽章不符：期望 55 8B EC，實際 % X", op.name, op.entry, got))
			return
		}
		if got := h.mem.Far(h.Img, op.exit, 1); !bytes.Equal(got, []byte{0xC3}) {
			h.fail(fmt.Sprintf("掛點 %s 完成（映像 %04X）不是 retn：實際 % X", op.name, op.exit, got))
			return
		}
	}
	sum := sha256.Sum256(h.mem.Far(h.Img, 0, ResidentEnd))
	h.ImageHash = hex.EncodeToString(sum[:])
	h.armed = true
	h.Diag = "ok"
	h.loadFont()
}

func (h *Hooks) loadFont() {
	b := h.mem.Bytes(OffFont, FontSize)
	sum := sha256.Sum256(b)
	h.FontHash = hex.EncodeToString(sum[:])
	h.Ov.SetFont(b)
}

// readStr 讀 DGROUP 內以 NUL 結尾的字串：回內容與是否讀滿 max 仍無 NUL。
func (h *Hooks) readStr(ptr uint16, max int) (string, bool) {
	b := h.mem.Bytes(ptr, max)
	for i, c := range b {
		if c == 0 {
			return string(b[:i]), false
		}
	}
	return string(b), true
}

// stringArgWords 回格式字串中每個 %s 轉換對應的引數字組序（%ld、%lu 佔 2 個字組）。
func stringArgWords(segs []FmtSeg) []int {
	var out []int
	w := 0
	for _, s := range segs {
		if s.Conv == nil {
			continue
		}
		if s.Conv.Type == 's' {
			out = append(out, w)
		}
		w++
		if s.Conv.Long {
			w++
		}
	}
	return out
}

type aRegs struct {
	step      uint64
	sp, bp    uint16
	di        uint16
	dsMatches bool
}

// readArgStrs 讀格式字串所有 %s 引數：指標、內容、種類。引數字組不足時只回已有的（Resolve 回 badarg）。
func (h *Hooks) readArgStrs(format string, args [12]uint16, sp uint16, caller uint16) (strs []ArgStr, truncated bool) {
	segs, err := ParseFormat(format)
	if err != nil {
		return nil, false
	}
	for i, wi := range stringArgWords(segs) {
		if wi >= len(args) {
			break
		}
		p := args[wi]
		content, tr := h.readStr(p, maxArgBytes)
		if tr {
			truncated = true
		}
		kind := h.Regions.Classify(p, sp, h.overlay)
		if kind == KindOther {
			h.Ov.C.Inc("arg_unclassified")
			h.Ov.C.Key("arg_unclassified", fmt.Sprintf("%04X/%d/%02X", caller, i, p>>8))
		}
		strs = append(strs, ArgStr{Ptr: p, Content: content, Kind: kind})
	}
	return strs, truncated
}

// handleA 是 A：格式化完成、視訊寫入之前。擷取 EventRecord 並開啟事件（001 §3.2、§3.3）。
func (h *Hooks) handleA(r aRegs) {
	if ok, _ := h.gate(); !ok {
		return
	}
	if !h.fontChecked {
		h.fontChecked = true
		h.loadFont()
	}
	c := h.Ov.C
	if !r.dsMatches {
		c.Inc("ds_mismatch")
		return
	}
	w := func(off uint16) uint16 { return h.mem.Word(r.bp + off) }
	rec := &EventRecord{
		Step: r.step, SP: r.sp, BP: r.bp, Caller: w(2), Overlay: h.overlay,
		Col: int(w(4)), Row: int(w(6)), FmtPtr: w(8),
	}
	skip := false
	text, _ := h.readStr(DSTextBuf, MaxTextChars)
	if len(text) != int(r.di) {
		c.Inc("badlen")
		skip = true
	}
	rec.Text = text
	rec.Cells = []byte(text)
	format, tr := h.readStr(rec.FmtPtr, maxFmtBytes)
	if tr {
		c.Inc("truncated_input")
		skip = true
	}
	rec.Format = format
	rec.FmtKind = h.Regions.Classify(rec.FmtPtr, r.sp, h.overlay)
	for i := range rec.Args {
		rec.Args[i] = w(uint16(10 + 2*i))
	}
	if !skip {
		strs, trunc := h.readArgStrs(format, rec.Args, r.sp, rec.Caller)
		if trunc {
			c.Inc("truncated_input")
			skip = true
		}
		rec.ArgStrs = strs
	}
	var composed int
	var misses map[Miss]int
	if !skip {
		composed, misses = h.Compose.Associate(rec, h.readStr)
	}
	opened := h.Ov.Begin(rec, skip)
	if opened {
		h.writes = h.writes[:0]
	}
	if opened && !skip {
		c.Add("composed", uint64(composed))
		for m, k := range misses {
			c.Add(MissCounter(m), uint64(k))
			if m == MissPrefix {
				c.Key("composed_miss_prefix", fmt.Sprintf("%04X", rec.Caller))
			}
		}
	}
}

// handleB 是 B：25A5 的共用結尾，提交事件。
func (h *Hooks) handleB() {
	ok, just := h.gate()
	if !ok {
		return
	}
	if just {
		h.Ov.C.Inc("prearm_completion")
		return
	}
	h.Ov.End()
}

// handleS 是 S：sprintf(dest, fmt, ...) 入口。[SP+2] dest、[SP+4] fmt、其後是可變引數。
func (h *Hooks) handleS(step uint64, sp uint16) {
	if ok, _ := h.gate(); !ok {
		return
	}
	caller := h.mem.Word(sp)
	dest, fmtPtr := h.mem.Word(sp+2), h.mem.Word(sp+4)
	format, tr := h.readStr(fmtPtr, maxArgBytes)
	if tr {
		return
	}
	var args [12]uint16
	for i := range args {
		args[i] = h.mem.Word(sp + 6 + uint16(2*i))
	}
	strs, trunc := h.readArgStrs(format, args, sp, caller)
	if trunc {
		return
	}
	h.Compose.OnSprintf(step, dest, fmtPtr, h.Regions.Classify(fmtPtr, sp, h.overlay), format, args, strs)
}

// handleT 是 T：strcat(dest, src) 入口。[SP+2] dest、[SP+4] src。
func (h *Hooks) handleT(sp uint16) {
	if h.NoStrcat {
		return
	}
	if ok, _ := h.gate(); !ok {
		return
	}
	dest, src := h.mem.Word(sp+2), h.mem.Word(sp+4)
	cur, _ := h.readStr(dest, maxDestScan)
	content, tr := h.readStr(src, maxArgBytes)
	if tr {
		return
	}
	a := ArgStr{Ptr: src, Content: content, Kind: h.Regions.Classify(src, sp, h.overlay)}
	if h.Compose.OnStrcat(dest, cur, a) == AppendNoRec {
		h.Ov.C.Inc("no_rec")
	}
}

// ---- 畫面操作（002 §3） ----

func (h *Hooks) bufHash(off uint16, extra func([]byte)) uint64 {
	seg := h.mem.Word(offBufSeg)
	b := h.mem.Far(seg, off, pageBytes)
	if extra != nil {
		extra(b)
	}
	return fnv64(b)
}

func (h *Hooks) opEntry(name string, sp uint16) {
	if ok, _ := h.gate(); !ok {
		return
	}
	p := &opPending{}
	switch name {
	case "int86":
		p.intNo = uint8(h.mem.Word(sp + 2))
		in := h.mem.Word(sp + 4)
		p.ax, p.bx, p.cx, p.dx = h.mem.Word(in), h.mem.Word(in+2), h.mem.Word(in+4), h.mem.Word(in+6)
		if ah := p.ax >> 8; p.intNo == 0x10 && (ah == 0x06 || ah == 0x07) && h.Int10Probe != nil {
			p.pre = h.mem.Far(0xB800, 0, pageBytes)
		}
	case "invert", "invert2":
		p.row, p.col, p.wid = int(h.mem.Word(sp+2)), int(h.mem.Word(sp+4)), int(h.mem.Word(sp+6))
	case "load1":
		p.h = h.bufHash(h.mem.Word(offBuf1), nil)
	case "load2":
		p.h = h.bufHash(h.mem.Word(offBuf2), nil)
	case "row24":
		off := h.mem.Word(offBuf1)
		p.h = h.bufHash(off, nil)
		p.hNew = h.bufHash(off, func(b []byte) {
			copy(b[0x1E00:0x1E00+row24Len], h.mem.Far(0xB800, 0x1E00, row24Len))
			copy(b[0x3E00:0x3E00+row24Len], h.mem.Far(0xBA00, 0x1E00, row24Len))
		})
	}
	h.pend[name] = p
}

func (h *Hooks) opExit(name string) {
	ok, just := h.gate()
	if !ok {
		return
	}
	p := h.pend[name]
	if p == nil {
		if just {
			h.Ov.C.Inc("prearm_completion")
		} else {
			h.Ov.C.Inc("dup_close")
		}
		return
	}
	delete(h.pend, name)
	switch name {
	case "int86":
		if p.intNo == 0x10 {
			h.Ov.OnInt10(p.ax, p.bx, p.cx, p.dx)
			if p.pre != nil && h.Int10Probe != nil {
				h.Int10Probe(p.ax, p.bx, p.cx, p.dx, p.pre, h.mem.Far(0xB800, 0, pageBytes))
			}
		}
	case "invert":
		h.Ov.OnInvert(p.row, p.col, p.wid, false)
	case "invert2":
		h.Ov.OnInvert(p.row, p.col, p.wid, true)
	case "save1":
		h.Ov.C.Inc("op_save1")
		h.Ov.OnSave1(fnv64(h.mem.Far(0xB800, 0, pageBytes)))
	case "copy12":
		h.Ov.C.Inc("op_copy12")
	case "load1", "load2":
		h.Ov.C.Inc("op_" + name)
		h.Ov.OnLoad(p.h)
	case "row24":
		h.Ov.C.Inc("op_row24")
		h.Ov.OnRow24(p.h, p.hNew)
	}
}

// Writes 回目前開啟中的事件自 A 起的視訊寫入（B800、BA00 段的複製）；供位置 oracle 在提交時比對（CheckPosition）。
// 事件關閉後內容保留到下一個 A。
func (h *Hooks) Writes() []VideoWrite { return h.writes }

// VerifyNow 立即驗證簽章（不等視訊模式 04h）。故障注入用：在 LZEXE 解壓之前驗證會失敗，
// 證明 001 §3.1 的「驗證時機」不可省（001 §10 第 5 項的負對照）。
func (h *Hooks) VerifyNow() {
	if !h.armed && !h.dead {
		h.verify()
	}
}
