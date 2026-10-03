package phantasie

import (
	"github.com/wicanr2/dosgolem/oracle"
)

// 映像偏移（CS = 映像段）與 DGROUP 偏移。證據：私有研究紀錄，靜態反組譯加動態軌跡。
const (
	// OffDrawFormatted 是繪字函式（`printf` 風格：欄、列、格式字串、可變引數）
	// 完成格式化、開始取字模之前的位置。此時 DS:DSTextBuf 起是以 00h 結尾的已格式化字串，
	// 欄、列、格式字串指標在 [BP+4]、[BP+6]、[BP+8]，近呼叫的返回位址在 [BP+2]。
	OffDrawFormatted = 0x26E9
	// OffOverlayLoad 是 overlay 載入器入口：`[SP+2]` 是 DGROUP 內檔名字串的指標。
	OffOverlayLoad = 0x3D0F
	// DSTextBuf 是已格式化字串的緩衝區（DGROUP 偏移）。
	DSTextBuf = 0x3A36
	// MaxTextChars 是繪字函式一次最多畫的字元數（一列 40 個字元格）。
	MaxTextChars = 40
	// DGroupParas 是 DGROUP 段比映像段多的段數（DGROUP 在映像之後 0C9Fh 段）。
	DGroupParas = 0xC9F
	// OverlayCodeStart 是 overlay 程式碼載入的映像偏移；小於它的呼叫端在常駐程式碼。
	OverlayCodeStart = 0x53EA
)

// TextEvent 是一次繪字：原版要把這個字串畫在第 Row 列、第 Col 欄。
type TextEvent struct {
	Step    uint64
	Col     int
	Row     int
	Text    []byte
	Format  []byte   // 格式字串（原版傳進來的，未代入）
	FmtPtr  uint16   // 格式字串指標（DGROUP 偏移）
	Args    []uint16 // 格式字串之後的 8 個堆疊字組（可變引數的原始值，語意由格式字串決定）
	Caller  uint16   // 近呼叫的返回位址（映像偏移）
	Overlay string   // 呼叫端在 overlay 程式碼時，目前載入的 overlay 名稱；否則空字串
}

// ImageSeg 回最後一支已載入程式的映像段（PSP + 10h）。還沒有任何程式載入時回 0。
func ImageSeg(o *oracle.Oracle) uint16 {
	log := o.ExecLog()
	if len(log) == 0 {
		return 0
	}
	return log[len(log)-1].PSP + 0x10
}

// cstr 讀 DGROUP 內以 00h 結尾的字串，最多 max 個位元組。
func cstr(o *oracle.Oracle, dg, off uint16, max int) []byte {
	b := o.Bytes(oracle.Far(dg, off), max)
	for i, c := range b {
		if c == 0 {
			return append([]byte(nil), b[:i]...)
		}
	}
	return append([]byte(nil), b...)
}

// Capture 在映像段 img 上掛兩個唯讀 hook：記錄每一次繪字，並追蹤目前載入的 overlay。
// 只讀記憶體，不改任何原版狀態。
func Capture(o *oracle.Oracle, img uint16, fn func(TextEvent)) {
	dg := img + DGroupParas
	var overlay string
	o.OnCall(oracle.Far(img, OffOverlayLoad), func(o *oracle.Oracle) {
		ptr := o.StackWord(1)
		overlay = string(cstr(o, dg, ptr, 16))
	})
	o.OnCall(oracle.Far(img, OffDrawFormatted), func(o *oracle.Oracle) {
		bp := o.BP()
		ds := o.DSReg()
		w := func(off uint16) uint16 { return o.Word(oracle.Far(ds, bp+off)) }
		caller := w(2)
		ev := TextEvent{
			Step:   o.Steps(),
			Col:    int(w(4)),
			Row:    int(w(6)),
			Text:   cstr(o, dg, DSTextBuf, MaxTextChars),
			Format: cstr(o, dg, w(8), 64),
			FmtPtr: w(8),
			Caller: caller,
		}
		for i := uint16(0); i < 8; i++ {
			ev.Args = append(ev.Args, w(10+2*i))
		}
		if caller >= OverlayCodeStart {
			ev.Overlay = overlay
		}
		fn(ev)
	})
}

// OffMemMove 是遠位址之間的複製函式（`rep movsw`）：簽章
// `(來源偏移, 來源段, 目的偏移, 目的段, 位元組數)`，近呼叫。所有寫視訊記憶體的路徑都經過它。
const OffMemMove = 0x4D65

// VideoWrite 是一次寫進 CGA 視訊記憶體（B800h 或 BA00h 段）的複製。
type VideoWrite struct {
	Step   uint64
	Caller uint16 // 近呼叫的返回位址（映像偏移）
	Seg    uint16 // 目的段：0B800h（偶數掃描線）或 0BA00h（奇數掃描線）
	Off    uint16 // 目的偏移
	Count  uint16 // 位元組數
}

// CaptureVideoWrites 掛唯讀 hook 記錄所有寫進視訊記憶體的複製，供盤點「誰在畫」。
func CaptureVideoWrites(o *oracle.Oracle, img uint16, fn func(VideoWrite)) {
	o.OnCall(oracle.Far(img, OffMemMove), func(o *oracle.Oracle) {
		seg := o.StackWord(4)
		if seg != 0xB800 && seg != 0xBA00 {
			return
		}
		fn(VideoWrite{Step: o.Steps(), Caller: o.StackWord(0), Seg: seg, Off: o.StackWord(3), Count: o.StackWord(5)})
	})
}

// ScreenOp 是一個會改動畫面的原版常式（映像偏移，近呼叫，C 呼叫慣例：
// 返回位址在 [SP]，引數依序在 [SP+2]、[SP+4]…）。證據探針用，引數語意以規格為準。
type ScreenOp struct {
	Name string
	Off  uint16
	Args int // 要讀幾個引數
}

// ScreenOps 是已知的畫面常式。名稱只是導覽，位址與引數才是證據。
var ScreenOps = []ScreenOp{
	{"invert", 0x0672, 3},           // 反相一段字元格（列、欄、寬）
	{"invert2", 0x0740, 3},          // 與 invert 同形，未讀
	{"restore-buf1", 0x0CF0, 0},     // 緩衝區 1 → 整頁（4000h bytes）
	{"restore-buf2", 0x0D10, 0},     // 緩衝區 2 → 整頁
	{"save-buf1", 0x0D30, 0},        // 整頁 → 緩衝區 1
	{"save-row24", 0x372D, 0},       // 第 24 列（兩個 bank）→ 緩衝區 1 的對應位置
	{"msgline", 0x1F1C, 0},          // 訊息列：存第 0 列、畫訊息、等鍵、還原
	{"window", 0x0DAD, 6},           // 視窗框（6 個引數）
	{"block", 0x30AC, 6},            // 圖形區塊
	{"bios-scroll-up", 0x38A7, 6},   // INT 10h AH=06：視窗上捲／清除（AL 行數、BH 屬性、列欄左上、列欄右下）
	{"bios-scroll-down", 0x38ED, 6}, // INT 10h AH=07
	{"clear-ish", 0x3AE8, 3},        // 清畫面（BIOS 捲動）
}

// OpEvent 是一次畫面常式的呼叫。
type OpEvent struct {
	Step   uint64
	Name   string
	Caller uint16
	Args   []uint16
}

// CaptureOps 在每個 ScreenOps 的入口掛唯讀 hook。
func CaptureOps(o *oracle.Oracle, img uint16, fn func(OpEvent)) {
	for _, op := range ScreenOps {
		op := op
		o.OnCall(oracle.Far(img, op.Off), func(o *oracle.Oracle) {
			ev := OpEvent{Step: o.Steps(), Name: op.Name, Caller: o.StackWord(0)}
			for i := 0; i < op.Args; i++ {
				ev.Args = append(ev.Args, o.StackWord(1+i))
			}
			fn(ev)
		})
	}
}

// OffInt86 是 `int86(intno, inregs, outregs)` 包裝函式的入口（映像偏移，近呼叫，C 慣例）。
// 原版所有 BIOS 與 DOS 軟體中斷都經過它；引數是 `union REGS` 的 DGROUP 指標，
// 欄位依序為 ax、bx、cx、dx、si、di、ds、es（各 2 bytes）。
const OffInt86 = 0x4E60

// Int86Event 是一次 int86 呼叫（入口時的輸入暫存器）。
type Int86Event struct {
	Step           uint64
	Caller         uint16
	IntNo          uint8
	AX, BX, CX, DX uint16
}

// CaptureInt86 在 int86 入口掛唯讀 hook，回報中斷號與輸入暫存器。
func CaptureInt86(o *oracle.Oracle, img uint16, fn func(Int86Event)) {
	o.OnCall(oracle.Far(img, OffInt86), func(o *oracle.Oracle) {
		ds := o.DSReg()
		in := o.StackWord(2)
		w := func(off uint16) uint16 { return o.Word(oracle.Far(ds, in+off)) }
		fn(Int86Event{Step: o.Steps(), Caller: o.StackWord(0), IntNo: uint8(o.StackWord(1)),
			AX: w(0), BX: w(2), CX: w(4), DX: w(6)})
	})
}
