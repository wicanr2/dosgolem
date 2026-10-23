package strikecommander

import "fmt"

// STRIKE 飛行中 HUD 狀態字、MFD 與訊息橫幅的觀察（strike_commander 規格 009 §3.1 到 §3.6）。
//
// 位址都寫成相對 STRIKE 主映像載入段 base 的段偏移與段內位移（規格 009 用語「+段偏移」）。
// 觀察器只讀 View，不寫入任何原版狀態。每畫格由 JEMM 的 CU（int 15h AX=1209h BX=4355h）把畫外緩衝
// 複製到 VGA；字串在 CU 入口成為候選，CU 返回之後的第一個 Frame 升格為可見集合（§3.6.3）。

// FlightView 是 FlightObserver 讀取的唯讀介面：View 加 AX、CX（CU 的 CH、MU 的 CX、橫幅返回的 AX）。
// session.StepView 直接滿足它。只在一次 Observe 呼叫內有效，不得保留。
type FlightView interface {
	View
	AX() uint16
	CX() uint16
}

const (
	// KindHUD、KindMFD、KindPlaque 是飛行字串的種類（規格 009 §3.11）。
	KindHUD    Kind = "hud"
	KindMFD    Kind = "mfd"
	KindPlaque Kind = "plaque"
)

// 段偏移與段內位移（規格 009 §2.1）。
const (
	segFText  = 0x5F7A // printf、格式化函式、puts、putc
	segFGlyph = 0x5F47 // 字形方法
	segFJemm  = 0x1D3A // JEMM 包裝：一般 CU、差分 CU、MU
	segFHUD   = 0x1462 // HUD 呼叫端
	segFMFD   = 0x1563 // MFD 呼叫端

	ipFPuts      = 0x0345
	ipFPutsRet   = 0x03CD
	ipFPutc      = 0x02AC
	ipFFmtRet    = 0x0618 // 格式化函式返回到 printf 的位址
	ipFHUDRet    = 0x01A6
	ipFMFDRet    = 0x0138
	ipCUPlain    = 0x00A5
	ipCUPlainRet = 0x00A7
	ipCUDiff     = 0x00D6
	ipCUDiffRet  = 0x00D8
	ipMU         = 0x00EE
	ipPlaque     = 0x0182 // 橫幅 overlay 函式入口（段每次不同）
	ipPlaqueLoop = 0x01DC
	ipPlaqueRet  = 0x01F2

	maxFlightPending    = 32
	flightReverifySteps = 500_000   // 同 base 驗證失敗後的重驗節流（§3.1 第 3 條）
	flightStaleSteps    = 1_500_000 // 待提交過期（§3.6.2 動作第 1 條）
	cuDupSteps          = 200_000   // 重複入口的步數界（§3.6.2）
	cuTimeoutSteps      = 1_000_000 // 「CU 進行中」逾時（§3.6.2）
)

var (
	sigFGlyph = []int{0x55, 0x8B, 0xEC, 0x56, 0x57, 0x8B, 0x76, 0x06, 0x8B, 0x7E, 0x08, 0x8A, 0x56, 0x0A, 0x80, 0x7C}
	// sigJemm 是 +1D3A:0090 到 00F1 共 98 bytes：一般 CU（0090–00A9）、差分 CU（00AA–00DA）、MU（00DB–00F1）。
	sigJemm = []int{
		0x55, 0x8B, 0xEC, 0x56, 0x8B, 0x76, 0x06, 0x33, 0xD2, 0x8A, 0x4E, 0x08, 0x8A, 0x6E, 0x0A, 0xBB, 0x55, 0x43, 0xB8, 0x09, 0x12, 0xCD, 0x15, 0x5E, 0x5D, 0xCB,
		0x55, 0x8B, 0xEC, 0x56, 0x8B, 0x76, 0x06, 0x83, 0x7C, 0x15, 0x00, 0x74, 0x10, 0xFF, 0x76, 0x0A, 0xFF, 0x76, 0x08, 0x56, 0x0E, 0xE8, 0xCE, 0xFF, 0x83, 0xC4, 0x06,
		0xEB, 0x11, 0xBA, 0x01, 0x00, 0x8A, 0x4E, 0x08, 0x8A, 0x6E, 0x0A, 0xBB, 0x55, 0x43, 0xB8, 0x09, 0x12, 0xCD, 0x15, 0x5E, 0x5D, 0xCB,
		0x55, 0x8B, 0xEC, 0x56, 0x8B, 0x76, 0x06, 0x8B, 0x54, 0x13, 0x8B, 0x4E, 0x08, 0xBB, 0x55, 0x4D, 0xB8, 0x09, 0x12, 0xCD, 0x15, 0x0B, 0xC0,
	}
	// 橫幅 overlay 函式：+0182 起 32 bytes、+01DC 起 23 bytes（§2.1）。
	sigPlaqueHead = []int{0x55, 0x8B, 0xEC, 0x83, 0xEC, 0x08, 0x56, 0x57, 0x8B, 0x76, 0x06, 0x8B, 0x7E, 0x0A, 0x89, 0x7E,
		0xFE, 0x8B, 0x44, 0x17, 0x40, 0x89, 0x46, 0xFC, 0xC7, 0x46, 0xF8, 0x00, 0x00, 0xEB, 0x3B, 0x83}
	sigPlaqueLoop = []int{0xC4, 0x5E, 0x0E, 0x03, 0x5E, 0xF8, 0x26, 0x8A, 0x07, 0x98, 0x89, 0x46, 0xFA, 0x0B, 0xC0, 0x75,
		0xB4, 0x8B, 0xC7, 0x5F, 0x5E, 0xC9, 0xCB}
	// 呼叫端 printf 之前的位元組（§2.1），-1 的兩個位置依序是 base + 5F7Ah 的低、高位元組。
	sigHUDCall = []int{0x8D, 0x46, 0xEC, 0x50, 0xFF, 0x76, 0x0C, 0x9A, 0xF9, 0x05, -1, -1}
	sigMFDCall = []int{0x8D, 0x46, 0x80, 0x50, 0x56, 0x9A, 0xF9, 0x05, -1, -1}
)

// flightBaseRows 是 §3.1 第 2 條的驗證列（puts 入口另比）。
var flightBaseRows = []sigRow{
	{"puts 返回", segFText, 0x03CA, []int{0x5F, 0x5E, 0xC9, 0xCB}},
	{"putc 入口", segFText, ipFPutc, []int{0x55, 0x8B, 0xEC, 0x83, 0xEC, 0x02, 0x56, 0x8B, 0x76, 0x06, 0x80, 0x7E, 0x08, 0x1F, 0x7E, 0x54}},
	{"格式化函式返回點", segFText, 0x0613, []int{0x56, 0x0E, 0xE8, 0xB6, 0xFD}},
	{"格式化函式序言", segFText, 0x03CE, []int{0x55, 0x8B, 0xEC}},
	{"printf 序言", segFText, 0x05F9, []int{0x55, 0x8B, 0xEC}},
	{"字形方法", segFGlyph, 0x0184, sigFGlyph},
	{"JEMM 包裝", segFJemm, 0x0090, sigJemm},
}

// FlightText 是一筆飛行字串（§3.11）。座標是原版像素。
type FlightText struct {
	Key    string // 觀察字串（記憶體讀到的整行）
	CatKey string // catalog key
	Kind   Kind
	K      int   // catalog key 在觀察字串中的起點
	X0     int   // 第一字的游標 x（橫幅為入口參數 x）
	Xs     []int // 每次 putc 入口的游標 x（含空白）；橫幅為 nil
	X1     int   // puts 返回時的游標 x；橫幅為返回時的 AX
	Y      int   // 第一次 putc 的游標 y（橫幅為入口參數 y）
	B0, BH int   // band [B0, B0+BH)
	DS     uint16
	Seg    uint16
	List   uint16
	// Before、After 是入口、返回時讀下的 band；Ref 是 CU 入口時讀下的 band。只用於繪製與比對，不寫入原版狀態。
	Before, After, Ref []byte
	Steps              uint64 // 入口步數
	Ink                InkResult
}

// Anchor 回去重與 generation 比的錨點：Xs[K]（橫幅為 X0）。
func (t FlightText) Anchor() int {
	if t.Kind == KindPlaque || t.K >= len(t.Xs) {
		return t.X0
	}
	return t.Xs[t.K]
}

// stripped 回去掉前後列與 Ref 的複本（FlightCommit 保存用，§3.11）。
func (t FlightText) stripped() FlightText {
	t.Before, t.After, t.Ref = nil, nil, nil
	t.Xs = append([]int(nil), t.Xs...)
	return t
}

// FlightCommit 是一次升格（§3.6.3）。
type FlightCommit struct {
	CUSteps    uint64 // 候選所屬 CU 的入口步數
	FrameSteps uint64 // 升格的 Frame 收到的步數
	Texts      []FlightText
	Generation uint64
}

// FlightCounts 是觀察器的計數（§3.11）。
type FlightCounts struct {
	HitPuts, HitPlaque, HitCUDiff, HitCUPlain, HitMU int // 0345、0182、00D6、00A5、00EE
	HitRetDiff, HitRetPlain                          int // 00D8、00A7
	CUCHNonZero                                      int // CH ≠ 0 的 CU
	ReturnsAccepted                                  int
	ReturnsIgnored                                   int // 「CU 進行中」為假時的 00D8、00A7
	ReturnsSSMismatch                                int // SS:SP（或配對的 IP）不符的返回
	EntriesDuplicate                                 int // 被忽略的重複入口
	EntriesVoided                                    int // 作廢的入口（SS:SP 不同或超過 20 萬步）
	Timeouts                                         int
	CandidatesReplaced                               int // 被取代的候選集合
	CandidatesReplacedAwaiting                       int // 其中返回後、升格前就被新 CU 取代
	CUFrames                                         int // CU 期間回掃
	CUFrameMismatch                                  int // CU 期間比對不符筆數（Frame 鉤子回填）
	// 略過（§3.6.1 的各檢查）。
	SkipBase, SkipVGA, SkipString, SkipDesc, SkipCaller, SkipResolve int
	SkipVoid, SkipInk, SkipDedup, SkipStale, SkipOverflow           int
	Promotions                                                      int
	FirstFrameBlocked                                               int // 升格後第一個 Frame 被擋下的筆數（Frame 鉤子回填）
	FirstFrameBlockedMaxRun                                         int // 連續幾次升格各至少擋下 1 筆，最長的次數
}

type flightPuts struct {
	t      FlightText
	obj    uint16
	ss, sp uint16
	yPrime int
}

type flightPlaque struct {
	t      FlightText
	cs     uint16
	ss, sp uint16
}

// FlightObserver 依規格 009 §3.1 到 §3.6 觀察 STRIKE 的飛行字串。只讀 View。
type FlightObserver struct {
	// OnCommit 在每次升格時呼叫（在 Frame 回呼內）。可為 nil。
	OnCommit func(FlightCommit)
	// OnAccepted 在字串通過墨跡框核對、加入待提交之前呼叫（strike_commander 規格 014 §4.2.2 墨跡台帳，只供曝光收據）。
	// 可為 nil；字串的陣列不得修改。
	OnAccepted func(FlightText)

	resolve func(Kind, string) (string, int, bool)

	baseKnown, baseOK bool
	base              uint16
	lastVerify        uint64

	cur *flightPuts
	plq *flightPlaque

	pending   []FlightText
	cands     []FlightText
	candValid bool
	candCU    uint64
	visible   []FlightText
	gen       uint64
	lastSig   uint64

	vga bool

	// cu 是 CU 狀態機（規格 012 §3.2.2）；cuh 是它的回呼，第一次用到時建立。
	cu  cuTracker
	cuh *cuHooks
	// onCU 在 CU 追蹤器各回呼的既有處理之後呼叫（strike_commander 規格 014 §3.4.2，只供呈現層）；nil 時行為不變。
	// 事件的 V 只在這次呼叫內有效（逾時為 nil）。
	onCU func(cuEvent)

	firstRun   int
	counts     FlightCounts
	log        []Event
	logDropped int
}

// NewFlightObserver 建一個觀察器。resolve 是 ResolveFlight 包好 catalog 的閉包（§3.4 第 1 條）。
func NewFlightObserver(resolve func(Kind, string) (string, int, bool)) *FlightObserver {
	return &FlightObserver{resolve: resolve}
}

// Base 回目前的載入段與是否啟用。
func (o *FlightObserver) Base() (uint16, bool) { return o.base, o.baseKnown && o.baseOK }

// Visible 回可見集合的複本。
func (o *FlightObserver) Visible() []FlightText { return append([]FlightText(nil), o.visible...) }

// Generation 在可見集合的（種類、catalog key、Y、錨點）序列改變時加 1。
func (o *FlightObserver) Generation() uint64 { return o.gen }

// Pending 回待提交清單的複本。
func (o *FlightObserver) Pending() []FlightText { return append([]FlightText(nil), o.pending...) }

// Candidates 回候選集合的複本。
func (o *FlightObserver) Candidates() []FlightText { return append([]FlightText(nil), o.cands...) }

// CUState 回「CU 進行中」與「CU 後待升格」。
func (o *FlightObserver) CUState() (inProgress, awaitingPromotion bool) {
	e := o.cu.Entry()
	return e.InProgress, e.Awaiting
}

// VGAWindow 回 VGA 視窗旗標（§3.6.4）。
func (o *FlightObserver) VGAWindow() bool { return o.vga }

// Log 回紀錄的複本與因上限捨棄的筆數。
func (o *FlightObserver) Log() ([]Event, int) { return append([]Event(nil), o.log...), o.logDropped }

// Counts 回計數的快照。
func (o *FlightObserver) Counts() FlightCounts { return o.counts }

// NoteFirstFrameBlocked 由 Frame 鉤子回填升格後第一個 Frame 被比對擋下的筆數（§3.11 接點第 2 條）。
func (o *FlightObserver) NoteFirstFrameBlocked(n int) {
	o.counts.FirstFrameBlocked += n
	if n > 0 {
		o.firstRun++
		if o.firstRun > o.counts.FirstFrameBlockedMaxRun {
			o.counts.FirstFrameBlockedMaxRun = o.firstRun
		}
	} else {
		o.firstRun = 0
	}
}

// NoteCUFrameMismatch 由 Frame 鉤子回填 CU 期間 Frame 的比對不符筆數。
func (o *FlightObserver) NoteCUFrameMismatch(n int) { o.counts.CUFrameMismatch += n }

func (o *FlightObserver) note(steps uint64, kind, format string, a ...any) {
	if len(o.log) >= maxLog {
		o.logDropped++
		return
	}
	o.log = append(o.log, Event{Steps: steps, Kind: kind, Detail: fmt.Sprintf(format, a...)})
}

// Observe 在每一步執行前呼叫。熱路徑只比 IP。
func (o *FlightObserver) Observe(v FlightView) {
	switch v.IP() {
	case ipFPuts:
		o.putsEntry(v)
	case ipFPutc:
		if o.cur != nil {
			o.putc(v)
		}
	case ipFPutsRet:
		if o.cur != nil {
			o.putsReturn(v)
		}
	case ipPlaque:
		if o.baseOK {
			o.plaqueEntry(v)
		}
	case ipPlaqueRet:
		if o.plq != nil {
			o.plaqueReturn(v)
		}
	case ipCUDiff, ipCUPlain, ipCUDiffRet, ipCUPlainRet, ipMU:
		if o.baseOK {
			o.cu.Observe(v, o.cuHooks())
		}
	}
	// 簽章層（§3.1 第 5 條）：可見集合非空、上次升格或檢查後經過 SigCheckSteps。
	if len(o.visible) != 0 && o.baseOK {
		if s := v.Steps(); s-o.lastSig >= SigCheckSteps {
			o.lastSig = s
			if why := o.checkBase(v, o.base); why != "" {
				o.note(s, "sig", "base %04X 的 %s 簽章不符，可見集合、候選、待提交與 CU 狀態清空，停用", o.base, why)
				o.baseOK = false
				o.cu.SetBase(o.base, o.baseOK)
				o.lastVerify = s
				o.reset()
			}
		}
	}
}

// reset 是 §3.1 第 5、6 條與 base 停用時的清空：待提交、候選、可見集合（非空時 generation 加 1）、
// VGA 視窗旗標、全部 CU 狀態與進行中的字串事件。
func (o *FlightObserver) reset() {
	if len(o.visible) != 0 {
		o.gen++
	}
	o.visible, o.pending, o.cands, o.candValid = nil, nil, nil, false
	o.cur, o.plq = nil, nil
	o.vga = false
	o.cu.Reset()
}

func (o *FlightObserver) verifyRows(v View, base uint16) string {
	for _, r := range flightBaseRows {
		if !match(v, base+r.seg, r.off, r.bytes, 0) {
			return r.name
		}
	}
	return ""
}

func (o *FlightObserver) checkBase(v View, base uint16) string {
	if !match(v, base+segFText, ipFPuts, sigPutsEntry, 0) {
		return "puts 入口"
	}
	return o.verifyRows(v, base)
}

func (o *FlightObserver) putsEntry(v FlightView) {
	cs := v.CS()
	if !match(v, cs, ipFPuts, sigPutsEntry, 0) {
		return
	}
	o.counts.HitPuts++
	steps := v.Steps()
	base := cs - segFText
	switch {
	case !o.baseKnown || base != o.base:
		// 定位到不同的 base：立即驗證；base 改變一律重設（§3.1 第 3、6 條）。
		if o.baseKnown {
			o.note(steps, "base", "puts 入口定位到不同的 base %04X（原 %04X）", base, o.base)
		}
		o.reset()
		o.base, o.baseKnown, o.lastVerify = base, true, steps
		o.cu.SetBase(o.base, o.baseOK)
		if why := o.verifyRows(v, base); why != "" {
			o.baseOK = false
			o.cu.SetBase(o.base, o.baseOK)
			o.note(steps, "base", "base %04X 停用：%s 簽章不符", base, why)
		} else {
			o.baseOK = true
			o.cu.SetBase(o.base, o.baseOK)
			o.note(steps, "base", "base %04X 啟用", base)
		}
	case !o.baseOK && steps-o.lastVerify >= flightReverifySteps:
		// 同 base 的重驗節流（§3.1 第 3 條）：Peek8 在頁不存在時回 FFh，暫時讀不到不會永久停用。
		o.lastVerify = steps
		if why := o.verifyRows(v, base); why == "" {
			o.baseOK = true
			o.cu.SetBase(o.base, o.baseOK)
			o.reset()
			o.note(steps, "base", "base %04X 重驗成功，啟用", base)
		}
	}
	if !o.baseOK {
		o.counts.SkipBase++
		return
	}
	if o.vga {
		o.counts.SkipVGA++
		return
	}
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	kind, ok := o.classify(v, ss)
	if !ok {
		o.counts.SkipCaller++
		return
	}
	obj := rd16(v, ss, sp+4)
	str := rd16(v, ss, sp+6)
	key, why := readLabelString(v, ds, str)
	if why != "" {
		o.counts.SkipString++
		return
	}
	catKey, k, ok := o.resolve(kind, key)
	if !ok {
		o.counts.SkipResolve++
		return
	}
	if o.cur != nil {
		o.note(steps, "puts", "上一筆 %q 的 puts 沒有返回，捨棄", o.cur.t.Key)
	}
	yp := int(rd16(v, ds, obj+objY))
	t := FlightText{Key: key, CatKey: catKey, Kind: kind, K: k, DS: ds, Steps: steps,
		Seg: rd16(v, ds, obj+objSeg), List: rd16(v, ds, obj+objList)}
	t.B0, t.BH = flightBand(kind, yp)
	t.Before = readRows(v, ds, t.Seg, t.List, t.B0, t.B0+t.BH)
	o.cur = &flightPuts{t: t, obj: obj, ss: ss, sp: sp, yPrime: yp}
}

// flightBand 回種類的 band 起點與列數（§3.5）；y 是 HUD、MFD 的游標 y 或橫幅的入口 y。
func flightBand(k Kind, y int) (b0, bh int) {
	switch k {
	case KindHUD:
		return y + 1, 5
	case KindMFD:
		return y - 1, 8
	}
	return y - 2, 9
}

// classify 是 §3.3 的呼叫端判定：puts 入口時 BP 是格式化函式的框架 P，[P] 是 printf 的框架 Q，
// [Q+2]、[Q+4] 是 printf 的返回位址 r、s；s:r 之前的位元組要等於 §2.1 的呼叫端列。
func (o *FlightObserver) classify(v View, ss uint16) (Kind, bool) {
	p := v.BP()
	text := o.base + segFText
	if rd16(v, ss, p+2) != ipFFmtRet || rd16(v, ss, p+4) != text {
		return "", false
	}
	q := rd16(v, ss, p)
	r, s := rd16(v, ss, q+2), rd16(v, ss, q+4)
	switch {
	case s == o.base+segFHUD && r == ipFHUDRet:
		if match(v, s, r-uint16(len(sigHUDCall)), sigHUDCall, text) {
			return KindHUD, true
		}
	case s == o.base+segFMFD && r == ipFMFDRet:
		if match(v, s, r-uint16(len(sigMFDCall)), sigMFDCall, text) {
			return KindMFD, true
		}
	}
	return "", false
}

func (o *FlightObserver) putc(v FlightView) {
	if v.CS() != o.base+segFText {
		return
	}
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	if rd16(v, ss, sp+4) != o.cur.obj {
		return
	}
	if len(o.cur.t.Xs) == 0 {
		o.cur.t.Y = int(rd16(v, ds, o.cur.obj+objY))
	}
	o.cur.t.Xs = append(o.cur.t.Xs, int(rd16(v, ds, o.cur.obj+objX)))
}

func (o *FlightObserver) putsReturn(v FlightView) {
	if v.CS() != o.base+segFText || v.SS() != o.cur.ss || v.SP() != o.cur.sp {
		return
	}
	c := o.cur
	o.cur = nil
	steps := v.Steps()
	t := c.t
	t.X1 = int(rd16(v, v.DS(), c.obj+objX))
	if len(t.Xs) != len(t.Key) {
		o.counts.SkipVoid++
		o.note(steps, "void", "%q 的 putc %d 次，字串 %d 字，作廢", t.Key, len(t.Xs), len(t.Key))
		return
	}
	t.X0 = t.Xs[0]
	if t.Y != c.yPrime {
		o.counts.SkipVoid++
		o.note(steps, "void", "%q 第一次 putc 的 Y %d 不等於 puts 入口的 y′ %d，作廢", t.Key, t.Y, c.yPrime)
		return
	}
	t.After = readRows(v, v.DS(), t.Seg, t.List, t.B0, t.B0+t.BH)
	x0, y0, x1, y1 := t.InkBox()
	t.Ink = flightInk(t.Key, t.Before, t.After, t.B0, x0, y0, x1, y1)
	if !inkInBox(t.Ink) {
		o.counts.SkipInk++
		o.note(steps, "ink", "墨跡框核對不通過：%s", t.Ink)
		return
	}
	o.addPending(steps, t)
}

// InkBox 回 §3.2.4 的預期框 [x0, x1) × [y0, y1)。
func (t FlightText) InkBox() (x0, y0, x1, y1 int) {
	switch t.Kind {
	case KindHUD:
		return t.X0, t.Y + 1, t.X1, t.Y + 5
	case KindMFD:
		return t.X0, t.Y + 1, t.X1, t.Y + 7
	}
	return t.X0, t.Y, t.X1, t.Y + 5
}

// flightInk 比較 band 的前後列（列 rowY0 起），以框 [x0, x1) × [y0, y1) 分框內外（§3.2.4）。
func flightInk(key string, before, after []byte, rowY0, x0, y0, x1, y1 int) InkResult {
	r := InkResult{Key: key}
	if before == nil || after == nil || len(before) != len(after) {
		r.Skipped = "前後列缺或長度不同"
		return r
	}
	for i := range after {
		if after[i] == before[i] {
			continue
		}
		x, y := i%320, rowY0+i/320
		r.Diffs++
		if x < x0 || x >= x1 || y < y0 || y >= y1 {
			r.Outside++
			if len(r.OutsideAt) < 64 {
				r.OutsideAt = append(r.OutsideAt, Point{x, y})
			}
			continue
		}
		if y == y0 {
			r.Top = true
		}
		if x == x0 {
			r.Left = true
		}
		if x == x1-1 {
			r.Right = true
		}
	}
	return r
}

// inkInBox 是 §3.2.4 的通過判定：差異點數 > 0 且框外差異 0（不要求三緣，與 InkResult.OK 不同）。
func inkInBox(r InkResult) bool { return r.Skipped == "" && r.Diffs > 0 && r.Outside == 0 }

// addPending 照 §3.6.1：去重（種類、catalog key、Y、錨點相同保留後到的）、上限 32 筆捨棄最舊。
func (o *FlightObserver) addPending(steps uint64, t FlightText) {
	if o.OnAccepted != nil {
		o.OnAccepted(t)
	}
	for i := 0; i < len(o.pending); i++ {
		p := o.pending[i]
		if p.Kind == t.Kind && p.CatKey == t.CatKey && p.Y == t.Y && p.Anchor() == t.Anchor() {
			o.pending = append(o.pending[:i], o.pending[i+1:]...)
			o.counts.SkipDedup++
			i--
		}
	}
	if len(o.pending) >= maxFlightPending {
		o.counts.SkipOverflow++
		o.note(steps, "pending", "待提交清單滿 %d 筆，捨棄最舊的 %q", maxFlightPending, o.pending[0].Key)
		o.pending = append(o.pending[:0], o.pending[1:]...)
	}
	o.pending = append(o.pending, t)
}

func (o *FlightObserver) plaqueEntry(v FlightView) {
	cs := v.CS()
	if v.Read8(lin(cs, ipPlaque)) != 0x55 {
		return
	}
	if !match(v, cs, ipPlaque, sigPlaqueHead, 0) || !match(v, cs, ipPlaqueLoop, sigPlaqueLoop, 0) {
		return
	}
	o.counts.HitPlaque++
	steps := v.Steps()
	if o.plq != nil {
		o.note(steps, "plaque", "上一筆橫幅 %q 沒有返回，捨棄", o.plq.t.Key)
		o.plq = nil
	}
	if o.vga {
		o.counts.SkipVGA++
		return
	}
	ss, sp, ds := v.SS(), v.SP(), v.DS()
	desc := rd16(v, ss, sp+6)
	x := int(rd16(v, ss, sp+8))
	y := int(int16(rd16(v, ss, sp+0x0A)))
	off, seg := rd16(v, ss, sp+0x0C), rd16(v, ss, sp+0x0E)
	key, why := readLabelString(v, seg, off)
	if why != "" {
		o.counts.SkipString++
		return
	}
	// §3.2.2 第 4 條：段是畫外緩衝 FFFFh，rect 在畫面內（橫幅描述元的 rect 是底板裁切窗，不是全畫面）。
	if r := readRect(v, ds, desc); rd16(v, ds, desc) != 0xFFFF || r.x0 > r.x1 || r.x1 > 319 || r.y0 > r.y1 || r.y1 > 199 {
		o.counts.SkipDesc++
		o.note(steps, "plaque", "橫幅 %q 的描述元 DS:%04X 不是畫外緩衝或 rect 超出畫面，略過", key, desc)
		return
	}
	catKey, k, ok := o.resolve(KindPlaque, key)
	if !ok {
		o.counts.SkipResolve++
		return
	}
	t := FlightText{Key: key, CatKey: catKey, Kind: KindPlaque, K: k, X0: x, Y: y, DS: ds, Steps: steps,
		Seg: rd16(v, ds, desc), List: rd16(v, ds, desc+2)}
	t.B0, t.BH = flightBand(KindPlaque, y)
	t.Before = readRows(v, ds, t.Seg, t.List, t.B0, t.B0+t.BH)
	o.plq = &flightPlaque{t: t, cs: cs, ss: ss, sp: sp}
}

func (o *FlightObserver) plaqueReturn(v FlightView) {
	if v.CS() != o.plq.cs || v.SS() != o.plq.ss || v.SP() != o.plq.sp {
		return
	}
	c := o.plq
	o.plq = nil
	steps := v.Steps()
	t := c.t
	t.X1 = int(v.AX())
	t.After = readRows(v, t.DS, t.Seg, t.List, t.B0, t.B0+t.BH)
	x0, y0, x1, y1 := t.InkBox()
	t.Ink = flightInk(t.Key, t.Before, t.After, t.B0, x0, y0, x1, y1)
	if !inkInBox(t.Ink) {
		o.counts.SkipInk++
		o.note(steps, "ink", "墨跡框核對不通過：%s", t.Ink)
		return
	}
	o.addPending(steps, t)
}

// cuHooks 回 CU 追蹤器的回呼（第一次呼叫時建立）。計數、日誌、待提交與候選的處理都在回呼內，
// 次序與日誌字串照抽出前的 cuEntry、cuReturn、mu（規格 012 §3.2.2）。
func (o *FlightObserver) cuHooks() *cuHooks {
	if o.cuh != nil {
		return o.cuh
	}
	o.cuh = &cuHooks{
		OnEntryHit: func(_ FlightView, ip uint16) {
			if ip == ipCUDiff {
				o.counts.HitCUDiff++
			} else {
				o.counts.HitCUPlain++
			}
		},
		// 目的不是 VGA：只計數，不動任何集合（009 §3.6.2）。
		OnCHNonZero: func(v FlightView, ip uint16) {
			o.counts.CUCHNonZero++
			o.emitCU(cuEvent{Kind: cuEvCHNonZero, IP: ip, V: v})
		},
		OnTimeout: func(steps uint64, old cuEntry) {
			o.counts.Timeouts++
			o.note(steps, "cu", "第 %d 步的 CU 入口超過 %d 步沒有返回，逾時", old.Steps, cuTimeoutSteps)
			o.emitCU(cuEvent{Kind: cuEvTimeout, Steps: steps, Entry: old})
		},
		OnDuplicate: func(v FlightView, ip uint16, cur cuEntry) {
			o.counts.EntriesDuplicate++
			o.emitCU(cuEvent{Kind: cuEvDuplicate, IP: ip, Entry: cur, V: v})
		},
		OnVoided: func(v FlightView, ip uint16, old cuEntry) {
			o.counts.EntriesVoided++
			o.note(v.Steps(), "cu", "第 %d 步的 CU 入口（SS:SP %04X:%04X）沒有返回就有新入口（%04X:%04X），舊的作廢", old.Steps, old.SS, old.SP, v.SS(), v.SP())
			o.emitCU(cuEvent{Kind: cuEvVoided, IP: ip, Entry: old, V: v})
		},
		OnEntryAccepted: func(v FlightView, steps uint64, ip uint16) {
			o.cuAccept(v, steps, ip)
			o.emitCU(cuEvent{Kind: cuEvAccepted, IP: ip, Steps: steps, V: v})
		},
		OnReturn: func(v FlightView, ip uint16, e cuEntry, out cuReturnOutcome) {
			if ip == ipCUDiffRet {
				o.counts.HitRetDiff++
			} else {
				o.counts.HitRetPlain++
			}
			switch out {
			case cuReturnIgnored:
				o.counts.ReturnsIgnored++
			case cuReturnMismatch:
				o.counts.ReturnsSSMismatch++
				o.note(v.Steps(), "cu", "CU 返回 %04X SS:SP %04X:%04X 與入口 %04X %04X:%04X 不配對", ip, v.SS(), v.SP(), e.IP, e.SS, e.SP)
			case cuReturnAccepted:
				o.counts.ReturnsAccepted++
			}
			o.emitCU(cuEvent{Kind: cuEvReturn, IP: ip, Entry: e, Outcome: out, V: v})
		},
		OnMU: o.muHit,
	}
	return o.cuh
}

// cuEventKind 是轉送給呈現層的 CU 追蹤器事件種類（strike_commander 規格 014 §3.4.2）。
type cuEventKind uint8

const (
	cuEvAccepted  cuEventKind = iota // 被接受的入口（cuAccept 已把待提交轉成候選）
	cuEvDuplicate                    // 重複入口（不受理）
	cuEvCHNonZero                    // CH ≠ 0 的入口（不受理）
	cuEvVoided                       // 進行中的入口被新入口作廢（Entry 是舊入口）
	cuEvTimeout                      // 進行中的入口逾時（Entry 是舊入口；V 為 nil）
	cuEvReturn                       // 返回（Outcome 是追蹤器的處理結果）
)

// cuEvent 是一筆 CU 追蹤器事件。V 只在這次回呼內有效，不得保留。
type cuEvent struct {
	Kind    cuEventKind
	IP      uint16
	Steps   uint64
	Entry   cuEntry
	Outcome cuReturnOutcome
	V       FlightView
}

func (o *FlightObserver) emitCU(e cuEvent) {
	if o.onCU != nil {
		o.onCU(e)
	}
}

// cuAccept 是被接受的 CU 入口（追蹤器改狀態之前呼叫）：009 §3.6.2 動作第 1 到 4 條；第 5 條「記入口」由追蹤器做。
func (o *FlightObserver) cuAccept(v FlightView, steps uint64, _ uint16) {
	// 1. 過期的待提交捨棄。
	kept := o.pending[:0:0]
	for _, t := range o.pending {
		if steps-t.Steps >= flightStaleSteps {
			o.counts.SkipStale++
			o.note(steps, "stale", "%q（第 %d 步入口）早於 CU %d 步以上，捨棄", t.Key, t.Steps, flightStaleSteps)
			continue
		}
		// 2. Ref：以自己記下的 DS、段、列表讀 band。
		t.Ref = readRows(v, t.DS, t.Seg, t.List, t.B0, t.B0+t.BH)
		kept = append(kept, t)
	}
	// 3. 待提交 → 候選，取代尚未升格的舊候選。
	if o.candValid {
		o.counts.CandidatesReplaced++
		if o.cu.Entry().Awaiting {
			o.counts.CandidatesReplacedAwaiting++
		}
		o.note(steps, "cu", "第 %d 步 CU 的候選 %d 筆尚未升格，被取代", o.candCU, len(o.cands))
	}
	o.cands, o.candValid, o.candCU = kept, true, steps
	o.pending = nil
	// 4. VGA 視窗旗標設為假。
	o.vga = false
}

// muHit 是 sigJemm 相符的 MU（009 §3.6.4）。
func (o *FlightObserver) muHit(v FlightView, cx uint16) {
	o.counts.HitMU++
	if cx == 0 {
		if len(o.pending) != 0 {
			o.note(v.Steps(), "vga", "MU CX=0：HMA 視窗指向 VGA，待提交 %d 筆清空", len(o.pending))
		}
		o.vga = true
		o.pending = nil
		return
	}
	o.vga = false
}

// Frame 在每次回掃呼叫（session 的 Frame 回呼內）。steps 是最後一次 BeforeStep 的步數（§3.6.3）。
// 追蹤器先檢查逾時；沒有升格且仍在進行中時計 CU 期間回掃。
func (o *FlightObserver) Frame(indexed []byte, steps uint64) {
	promote, _ := o.cu.Frame(steps, o.cuHooks())
	if !promote {
		if o.cu.Entry().InProgress {
			o.counts.CUFrames++
		}
		return
	}
	prev := o.visible
	o.visible = o.cands
	o.cands, o.candValid = nil, false
	if !sameFlight(prev, o.visible) {
		o.gen++
	}
	o.lastSig = steps
	o.counts.Promotions++
	if o.OnCommit != nil {
		c := FlightCommit{CUSteps: o.candCU, FrameSteps: steps, Generation: o.gen}
		for _, t := range o.visible {
			c.Texts = append(c.Texts, t.stripped())
		}
		o.OnCommit(c)
	}
}

// sameFlight 比（種類、catalog key、Y、錨點）序列（§3.6.3 第 3 條）。
func sameFlight(a, b []FlightText) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].CatKey != b[i].CatKey || a[i].Y != b[i].Y || a[i].Anchor() != b[i].Anchor() {
			return false
		}
	}
	return true
}
