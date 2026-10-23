package strikecommander

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

// strike_commander 規格 014（2D 分期 READY 第 10 版）分期 (c)：覆蓋模式的呈現層 Presenter。
//
// 呈現層在「遊戲完成畫面」的那一步取像（完成點），把畫面色號、色盤與覆繪狀態凍結成一個呈現像；
// Compose 只由目前的呈現像決定（§3.1 I1）。分期 (c) 的窗只有 OPTTEST 的 blit（全螢幕與非全螢幕）與 STRIKE 的 CU；
// shape、puts、游標、MENU 與 STRIKE 的常式複本是分期 (d)，未歸屬寫入的退路取像是分期 (f)。窗表是資料（presRoutines），
// (d) 只加條目。
//
// 呈現層只讀觀察器與 session 的複本，不寫原版記憶體（§3.1 I4）；呈現用 Overlay、FlightOverlay 與收據的觀察用實例分開。

// PresentView 是呈現層在觀察點讀取的唯讀介面。*session.StepView 滿足它；只在一次 Observe 呼叫內有效，不得保留。
type PresentView interface {
	FlightView
	Deliveries() uint64
	Palette() [256][3]uint8
	ReadBlock(a uint32, dst []byte) bool
	IndexedLayout() machine.IndexedLayout
	IndexedInto(dst []byte) (machine.IndexedLayout, bool)
}

var _ PresentView = (*session.StepView)(nil)

// PresentKind 是呈現像的種類（§3.3.1 第 4 條）。
type PresentKind string

const (
	PresBlit     PresentKind = "blit"
	PresPartial  PresentKind = "partial"
	PresShape    PresentKind = "shape"
	PresPuts     PresentKind = "puts"
	PresRoutine  PresentKind = "routine"
	PresCU       PresentKind = "cu"
	PresCUReturn PresentKind = "cu-return"
	PresFallback PresentKind = "fallback"
	PresPalette  PresentKind = "palette"
	PresSubtitle PresentKind = "subtitle"
	// PresFrame 只在負對照 N2（回掃取像）出現。
	PresFrame PresentKind = "frame"
)

const (
	presQueueMax       = 256     // §3.3.1 第 2 條
	presTimeoutDefault = 200_000 // §3.2.1 第 4 條
	presRecheckEvery   = 16      // §4.2.2 重讀抽查
	presMaxRecords     = 8192    // 逐筆紀錄的上限（計數不受限）

	// cuViaPlainSPDelta 是 CUReturnViaPlain 的堆疊位移（§3.4.2）：差分包裝 +1D3A:00AA 在 [si+15] ≠ 0 時
	// 走 00B7 push [bp+0A]、00BA push [bp+08]、00BD push si、00BE push cs、00BF call 0090，
	// 一般包裝的 00A7 返回之後 pop si、pop bp、retf（4 bytes），回到 00C2 add sp,6、00C5 jmp 00D8。
	// 00D8 的 SP 因此等於 00A7 的 SP + 2 + 2 + 4 + 6 = +14（機械事實：flight.go sigJemm 的位元組解碼）。
	cuViaPlainSPDelta = 14
)

// presentMut 是負對照的突變旗標（§4.5）。零值是正確的呈現層。
type presentMut uint32

const (
	presMutN2   presentMut = 1 << iota // 回掃取像、在回掃凍結狀態
	presMutN6                          // 飛行在 CU 入口改讀 VGA
	presMutN9                          // 非全螢幕 blit 不開窗
	presMutN22                         // 非 CU 取像一律即時讀 Candidates()
	presMutN22b                        // 非 CU 取像一律用 Visible()
	presMutN24                         // 重複入口不看 Deliveries()
	presMutN27                         // CUResidue 不另取 cu-return（只計數）
	presMutN30a                        // 本次候選為空時走 cu-return（第 7 版行為）
	presMutN30b                        // 本次候選非空時仍用較早的（退用）描述元
	presMutN33                         // CUReturnDelivered 併入 CUReturnNoWindow
	presMutN34                         // 退用描述元指向錯位的列表
	presMutN35                         // CUReturnViaPlain 併入 CUReturnNoWindow
)

var presMutNames = []struct {
	m presentMut
	n string
}{
	{presMutN2, "n2"}, {presMutN6, "n6"}, {presMutN9, "n9"}, {presMutN22, "n22"}, {presMutN22b, "n22b"},
	{presMutN24, "n24"}, {presMutN27, "n27"}, {presMutN30a, "n30a"}, {presMutN30b, "n30b"}, {presMutN33, "n33"},
	{presMutN34, "n34"}, {presMutN35, "n35"},
}

// parsePresentMut 解析逗號分隔的突變名稱（例：n33,n34）。
func parsePresentMut(s string) (presentMut, error) {
	var m presentMut
	if s == "" {
		return 0, nil
	}
	for _, f := range bytes.Split([]byte(s), []byte(",")) {
		ok := false
		for _, e := range presMutNames {
			if string(f) == e.n {
				m |= e.m
				ok = true
			}
		}
		if !ok {
			return 0, fmt.Errorf("strikecommander: 呈現層突變 %q 不支援", f)
		}
	}
	return m, nil
}

func (m presentMut) String() string {
	var out []string
	for _, e := range presMutNames {
		if m&e.m != 0 {
			out = append(out, e.n)
		}
	}
	if len(out) == 0 {
		return "無"
	}
	return fmt.Sprint(out)
}

// ---- 窗表（§3.2） ----

type presProg uint8

const (
	progOPTTEST presProg = iota // 段是相對 LabelObserver base 的段偏移
	progAbs                     // 段是絕對 CS（只供單元測試注入條目）
)

// presRoutine 是窗表的一列。
type presRoutine struct {
	name       string
	prog       presProg
	seg        uint16
	entryIP    uint16
	retIP      uint16
	ipLo, ipHi uint16 // 歸屬範圍（含端點），CS 等於窗的入口 CS
	allCS      bool   // 窗內全部寫入都歸屬、不比 CS（puts，分期 (d)）
	completion bool   // 返回是否為完成點（游標擦除為否，分期 (d)）
	timeout    uint64
	kind       PresentKind
	blit       bool // 入口讀來源、目的矩形，分全螢幕（blit）與非全螢幕（partial）
}

// presRoutines 是分期 (c) 的窗表：OPTTEST blit sub_49FB0（§3.2 表第 1 列）。CU 不進 2D 窗堆疊（§3.2.1 第 6 條）。
var presRoutines = []presRoutine{
	{name: "blit", prog: progOPTTEST, seg: segBlit, entryIP: ipBlitEntry, retIP: ipBlitRet, ipLo: 0x0000, ipHi: ipBlitRet,
		completion: true, timeout: presTimeoutDefault, kind: PresBlit, blit: true},
}

type presWindow struct {
	r          *presRoutine
	cs, ss, sp uint16
	entry      uint64
	deliv      uint64
	kind       PresentKind
	wrote      bool
	firstWrite uint64
	closed     bool
}

// ---- 呈現像 ----

// PresentInfo 是呈現像的中繼資料（§3.8 Current）。
type PresentInfo struct {
	Seq          uint64
	CaptureSteps uint64
	Kind         PresentKind
	SourceCS     uint16
	SourceIP     uint16
}

type presentImage struct {
	info     PresentInfo
	indexed  []byte // 64000，不可變（呈現像之間可共用）
	palette  [256][3]uint8
	labels   []Label
	labelGen uint64
	flight   []FlightText
	presGen  uint64
	// 字幕（§3.3.4）：current() 的值複本與阻擋狀態（以呈現像色號判）。
	subPtr     *Subtitle
	sub        Subtitle
	subBlocked bool
}

type presItem struct {
	art   bool
	ev    ArtEvent
	win   *presWindow // nil：發生時沒有開著的窗
	await bool        // ClearArt：等同一步的全螢幕 blit 窗開窗後歸屬
	steps uint64
}

type presDesc struct {
	valid          bool
	ds, seg, list  uint16
	base           uint16
	fromCandidates bool
}

type presCU struct {
	valid      bool
	entry      uint64
	captured   bool
	buf        []byte
	desc       presDesc
	fromFB     bool
	needReturn bool
}

type presRet struct {
	valid  bool
	ip     uint16
	ss, sp uint16
	deliv  uint64
	steps  uint64
}

type presRecheck struct {
	img []byte
	vw  uint64
	seq uint64
}

// PresentCounts 是各種取像數與異常計數（§4.2.3）。
type PresentCounts struct {
	Frames   int
	Captures map[PresentKind]int

	QueueOverflow, WindowTimeout, ModeTimeout, DuplicateEntry, DuplicateDelivered, WindowSkipped int
	ReturnWithoutEntry, ReturnDuplicateDelivered                                                 int
	RecaptureChecked, RecaptureSkipped, RecaptureMismatch                                        int
	CUReturnNoWindow, CUReturnDelivered, CUReturnCHNonZero, CUReturnViaPlain                     int
	CUResidue, CUResidueFallbackDesc, CUCapturedNotCopied, StaleAfterCUVoid                      int
	CUNoDesc, CUFallbackDescUsed, BufCUReadFail                                                  int
	CUDuplicate, CUCHNonZero, CUVoided, CUTimeout                                                int
	StaleChecked, Stale, StaleNoImage                                                            int
	Unattributed, UnattributedOffscreen                                                          int
	IndexedIntoShort, NonLinearLayout, ModeChanges                                               int
	Fallback, CursorEraseFlush                                                                   int
	WindowsOpened                                                                                int
	MaxDepth                                                                                     int
}

// PresentStale 是一筆陳舊（或無呈現像、CU 作廢後）的回掃。
type PresentStale struct {
	Steps          uint64
	Class          string // stale、no-image、after-cu-void
	Seq            uint64
	Kind           PresentKind
	Diff           int
	X0, Y0, X1, Y1 int // 差異外接框（含端點）
}

// PresentUnattr 是未歸屬寫入依 CS:IP 的彙整。
type PresentUnattr struct {
	CS, IP      uint16
	Ctx         machine.VideoWriteCtx
	Count       int
	First, Last uint64
}

// PresentWindowRec 是一筆窗的開關紀錄。
type PresentWindowRec struct {
	Steps      uint64
	Event      string // open、close、skip、timeout、dup-delivered、dup-entry、ret-no-entry、ret-dup-delivered
	Routine    string
	CS, SS, SP uint16
	Depth      int
	Wrote      bool
	Len        uint64
	Kind       PresentKind
}

// PresentCURec 是一筆 CU 異常（CUResidue、CU 返回無窗、CU 作廢後取像等）。
type PresentCURec struct {
	Steps          uint64
	Class          string
	CUEntry        uint64
	IP, SS, SP     uint16
	Diff           int
	X0, Y0, X1, Y1 int
	FallbackDesc   bool
}

// PresentShot 是收據 shot（或停止點）的呈現合成與差異斷言（§3.9）。
type PresentShot struct {
	Info       PresentInfo
	Composites map[int][]byte
	Cond       bool     // 三個條件全部成立
	Why        []string // 不成立的條件
	Equal      map[int]bool
}

// PresenterConfig 是 NewPresenter 的輸入（§3.8）。
type PresenterConfig struct {
	Fonts          *TextFonts
	Catalog        map[string]string
	GfxCatalog     map[string]string
	FlightCatalogs *FlightCatalogs // nil：不建呈現用 FlightOverlay
	Scales         []int
	Subtitles      bool // 前端字幕開關的初值
	Receipt        bool // 收據模式：做重讀抽查（§4.2.2）
	exactOnly      bool
	mut            presentMut
	routines       []presRoutine // nil 用 presRoutines（單元測試注入）
}

type composeCacheKey struct {
	seq   uint64
	scale int
	subs  bool
}

// Presenter 是覆蓋模式的呈現層（§3.8）。單一 goroutine 使用。
type Presenter struct {
	cfg      PresenterConfig
	mut      presentMut
	ovs      map[int]*Overlay
	fovs     map[int]*FlightOverlay
	labels   *LabelObserver
	flight   *FlightObserver
	subs     *Subtitles
	routines []presRoutine
	ipMask   [1024]uint64

	queue      []presItem
	clearPend  bool
	stack      []*presWindow
	epWrote    bool
	lastClosed *presWindow
	lastRetWin presWindow // 上一個關閉的窗（同 PC 的重複返回判定）
	haveRetWin bool

	cur  *presentImage
	seq  uint64
	vga  []byte
	tmp  []byte
	bufC []byte

	presGen    uint64
	presSet    []FlightText
	cuOpen     bool
	cuSnap     []FlightText
	snapValid  bool
	cuPend     bool
	cuPendIP   uint16
	cu         presCU
	fbDesc     presDesc
	lastRet    presRet
	lastCH     presRet
	afterRet   bool
	afterRetAt uint64
	staleVoid  bool

	needFallback bool // 未歸屬寫入或模式設定之後（退路取像在分期 (f)）

	steps         uint64
	vwTotal       uint64
	vwSinceCap    uint64
	stateSinceCap uint64
	frameSinceCap bool

	recheck *presRecheck
	capN    int

	subsOn bool
	cache  map[int]struct {
		key  composeCacheKey
		rgba []byte
	}

	counts    PresentCounts
	captures  []PresentInfo
	stale     []PresentStale
	unattr    map[[2]uint16]*PresentUnattr
	windows   []PresentWindowRec
	cuRecs    []PresentCURec
	onCapture []func(PresentInfo)
}

// NewPresenter 建呈現層與它自有的呈現用實例（每倍率一個）。
func NewPresenter(cfg PresenterConfig) (*Presenter, error) {
	if cfg.Fonts == nil {
		return nil, fmt.Errorf("strikecommander: 呈現層缺反鋸齒圖集")
	}
	p := &Presenter{cfg: cfg, mut: cfg.mut, ovs: map[int]*Overlay{}, fovs: map[int]*FlightOverlay{}, subsOn: cfg.Subtitles,
		unattr: map[[2]uint16]*PresentUnattr{}}
	p.counts.Captures = map[PresentKind]int{}
	p.cache = map[int]struct {
		key  composeCacheKey
		rgba []byte
	}{}
	for _, sc := range cfg.Scales {
		ov, err := NewOverlayText(cfg.Fonts, cfg.Catalog, cfg.GfxCatalog, sc)
		if err != nil {
			return nil, err
		}
		ov.exactOnly = cfg.exactOnly
		p.ovs[sc] = ov
		if cfg.FlightCatalogs != nil {
			fov, err := NewFlightOverlayText(cfg.Fonts, *cfg.FlightCatalogs, sc)
			if err != nil {
				return nil, err
			}
			p.fovs[sc] = fov
		}
	}
	p.routines = cfg.routines
	if p.routines == nil {
		p.routines = presRoutines
	}
	for _, r := range p.routines {
		for _, ip := range []uint16{r.entryIP, r.retIP} {
			p.ipMask[ip>>6] |= 1 << (ip & 63)
		}
	}
	return p, nil
}

// Attach 串接既有回呼（§8，不取代）：圖面事件入佇列、狀態改變計數、CU 事件；SessionObserver 轉送 BeforeStep、
// VideoWrite、Frame。在 AttachOverlays、收據與曝光收據的接線之後呼叫。
func (p *Presenter) Attach(so *SessionObserver) {
	p.labels, p.flight, p.subs = so.Labels, so.Flight, so.Subtitles
	lo := so.Labels
	art, clear, commit := lo.OnArt, lo.OnClearArt, lo.OnCommit
	lo.OnArt = func(ev ArtEvent) {
		if art != nil {
			art(ev)
		}
		p.stateSinceCap++
		p.enqueue(presItem{art: true, ev: ev, steps: ev.Steps})
	}
	lo.OnClearArt = func(steps uint64) {
		if clear != nil {
			clear(steps)
		}
		p.stateSinceCap++
		p.enqueue(presItem{await: true, steps: steps})
	}
	lo.OnCommit = func(c Commit) {
		if commit != nil {
			commit(c)
		}
		p.stateSinceCap++
	}
	if f := so.Flight; f != nil {
		fc := f.OnCommit
		f.OnCommit = func(c FlightCommit) {
			if fc != nil {
				fc(c)
			}
			p.stateSinceCap++
		}
		f.onCU = p.onCU
	}
	so.present = p
}

// Current 回目前呈現像的中繼資料；還沒有呈現像時 Seq 為 0。
func (p *Presenter) Current() PresentInfo {
	if p.cur == nil {
		return PresentInfo{}
	}
	return p.cur.info
}

// Counts 回計數的快照。
func (p *Presenter) Counts() PresentCounts {
	c := p.counts
	c.Captures = map[PresentKind]int{}
	for k, v := range p.counts.Captures {
		c.Captures[k] = v
	}
	return c
}

// OnCapture 登記每次產生呈現像時的回呼（只供收據，§4.2.1 窮舉模式）。
func (p *Presenter) OnCapture(fn func(PresentInfo)) { p.onCapture = append(p.onCapture, fn) }

// SetSubtitles 是前端字幕開關（§3.3.4：在 Compose 時套用，不產生呈現像）。
func (p *Presenter) SetSubtitles(on bool) { p.subsOn = on }

// Compose 回目前呈現像在倍率 scale 的合成圖（§3.8）。依 Seq、倍率與字幕開關快取；回傳的切片呼叫端不得修改。
// 還沒有呈現像時 ok 為假。
func (p *Presenter) Compose(scale int) ([]byte, bool, error) {
	if p.cur == nil {
		return nil, false, nil
	}
	if p.ovs[scale] == nil {
		return nil, false, fmt.Errorf("strikecommander: 呈現層沒有 %d 倍的實例", scale)
	}
	k := composeCacheKey{p.cur.info.Seq, scale, p.subsOn}
	if c, ok := p.cache[scale]; ok && c.key == k {
		return c.rgba, true, nil
	}
	out := p.composeImage(p.cur, scale, p.subsOn)
	p.cache[scale] = struct {
		key  composeCacheKey
		rgba []byte
	}{k, out}
	return out, true, nil
}

// composeImage 以呈現像凍結的集合與世代合成（§3.3.1 第 5 條），subs 為真時畫字幕（§3.3.4）。
// 呈現用實例的狀態屬於最近一次取像，所以只能合成目前的呈現像。
func (p *Presenter) composeImage(img *presentImage, sc int, subs bool) []byte {
	in := scaleIn{Overlay: p.ovs[sc], LabelsV: img.labels, LabelsG: img.labelGen, Scale: sc, Frame: img.indexed, Palette: img.palette}
	if fov := p.fovs[sc]; fov != nil {
		in.Flight, in.FlightV, in.FlightG = fov, img.flight, img.presGen
	}
	o := composeScale(in)
	if subs && p.subs != nil && img.subPtr != nil && !img.subBlocked && img.sub.Text != "" && sc >= 2 && sc <= 3 {
		s := img.sub
		p.subs.drawText(&s, o.Comp, sc)
	}
	return o.Comp
}

// ---- 佇列（§3.3.1） ----

func (p *Presenter) enqueue(it presItem) {
	if n := len(p.stack); n != 0 && !it.await {
		it.win = p.stack[n-1]
	}
	if it.await {
		p.clearPend = true
	}
	if len(p.queue) >= presQueueMax {
		p.counts.QueueOverflow++
		p.applyItem(p.queue[0])
		copy(p.queue, p.queue[1:])
		p.queue = p.queue[:len(p.queue)-1]
	}
	p.queue = append(p.queue, it)
}

func (p *Presenter) applyItem(it presItem) {
	for _, sc := range p.cfg.Scales {
		if it.art {
			p.ovs[sc].AddDialogArt(it.ev)
		} else {
			p.ovs[sc].ClearArt()
		}
	}
}

// applyQueue 依序套用所屬窗已關閉（或沒有所屬窗）的項目；所屬窗仍開著的留在佇列（§3.1 I5）。
func (p *Presenter) applyQueue() {
	keep := p.queue[:0]
	for _, it := range p.queue {
		if it.await || (it.win != nil && !it.win.closed) {
			keep = append(keep, it)
			continue
		}
		p.applyItem(it)
	}
	for i := len(keep); i < len(p.queue); i++ {
		p.queue[i] = presItem{}
	}
	p.queue = keep
}

func (p *Presenter) closedItems() bool {
	for _, it := range p.queue {
		if it.win != nil && it.win.closed {
			return true
		}
	}
	return false
}

// ---- 觀察點 ----

// Observe 在 BeforeStep 內、Labels、Flight、Subtitles 的 Observe 之後呼叫（§3.2.1 第 7 條）。
func (p *Presenter) Observe(v PresentView) {
	steps := v.Steps()
	p.steps = steps
	if p.flight != nil {
		p.cuOpen, _ = p.flight.CUState()
	}
	// 上一步排下的「下一個觀察點」工作。
	if p.recheck != nil {
		p.doRecheck(v)
	}
	if p.afterRet && steps != p.afterRetAt {
		// CU 返回（配對成功）後的下一個觀察點（返回那一步的 Flight.Observe 已設旗標，這一步不算）。
		p.afterRet = false
		p.cuAfterReturn(v)
	}
	if p.cuPend {
		p.cuPend = false
		p.cuEntryCapture(v)
	}
	if len(p.stack) != 0 {
		p.checkTimeouts(steps)
	}
	if ip := v.IP(); p.ipMask[ip>>6]&(1<<(ip&63)) != 0 {
		p.dispatch(v, ip)
	}
	if p.clearPend {
		// ClearArt 沒有在同一步開窗的全螢幕 blit：當作沒有所屬窗。
		p.clearPend = false
		for i := range p.queue {
			p.queue[i].await = false
		}
	}
}

func (p *Presenter) dispatch(v PresentView, ip uint16) {
	cs := v.CS()
	for i := range p.routines {
		r := &p.routines[i]
		if ip != r.entryIP && ip != r.retIP {
			continue
		}
		want := r.seg
		if r.prog == progOPTTEST {
			base, ok := p.labels.Base()
			if !ok {
				continue
			}
			want = base + r.seg
		}
		if cs != want {
			continue
		}
		if ip == r.entryIP {
			p.entry(v, r)
		} else {
			p.ret(v, r)
		}
	}
}

func (p *Presenter) logWindow(rec PresentWindowRec) {
	if len(p.windows) < presMaxRecords {
		p.windows = append(p.windows, rec)
	}
}

func (p *Presenter) entry(v PresentView, r *presRoutine) {
	cs, ss, sp := v.CS(), v.SS(), v.SP()
	steps := v.Steps()
	if n := len(p.stack); n != 0 {
		top := p.stack[n-1]
		if top.r == r && top.cs == cs && top.ss == ss && top.sp == sp {
			// §3.2.1 第 2 條：同常式、同 PC、同 SS:SP 的重複入口不另開窗；以送達計數分合法與異常。
			d := v.Deliveries()
			ev := "dup-entry"
			if d > top.deliv && p.mut&presMutN24 == 0 {
				p.counts.DuplicateDelivered++
				top.deliv = d
				ev = "dup-delivered"
			} else {
				p.counts.DuplicateEntry++
			}
			p.logWindow(PresentWindowRec{Steps: steps, Event: ev, Routine: r.name, CS: cs, SS: ss, SP: sp, Depth: n})
			return
		}
	}
	w := &presWindow{r: r, cs: cs, ss: ss, sp: sp, entry: steps, deliv: v.Deliveries(), kind: r.kind}
	if r.blit {
		ds := v.DS()
		src, dst := rd16(v, ss, sp+4), rd16(v, ss, sp+6)
		if readRect(v, ds, src) != fullScreen || readRect(v, ds, dst) != fullScreen {
			w.kind = PresPartial
			if p.mut&presMutN9 != 0 {
				return
			}
		} else {
			for i := range p.queue {
				if p.queue[i].await {
					p.queue[i].await, p.queue[i].win = false, w // ClearArt 屬全螢幕 blit 窗（§3.3.1 第 1 條）
				}
			}
			p.clearPend = false
		}
	}
	p.stack = append(p.stack, w)
	p.counts.WindowsOpened++
	p.counts.MaxDepth = max(p.counts.MaxDepth, len(p.stack))
	p.logWindow(PresentWindowRec{Steps: steps, Event: "open", Routine: r.name, CS: cs, SS: ss, SP: sp, Depth: len(p.stack), Kind: w.kind})
}

func (p *Presenter) closeWin(w *presWindow, steps uint64, ev string, depth int) {
	w.closed = true
	if w.wrote {
		p.lastClosed = w
	}
	p.logWindow(PresentWindowRec{Steps: steps, Event: ev, Routine: w.r.name, CS: w.cs, SS: w.ss, SP: w.sp, Depth: depth, Wrote: w.wrote, Len: steps - w.entry, Kind: w.kind})
}

func (p *Presenter) ret(v PresentView, r *presRoutine) {
	cs, ss, sp := v.CS(), v.SS(), v.SP()
	steps := v.Steps()
	for i := len(p.stack) - 1; i >= 0; i-- {
		w := p.stack[i]
		if w.r != r || w.cs != cs || w.ss != ss || w.sp != sp {
			continue
		}
		for j := len(p.stack) - 1; j > i; j-- {
			p.counts.WindowSkipped++
			p.closeWin(p.stack[j], steps, "skip", j+1)
		}
		p.closeWin(w, steps, "close", i+1)
		p.stack = p.stack[:i]
		p.lastRetWin, p.haveRetWin = *w, true
		p.lastRetWin.deliv = v.Deliveries()
		if len(p.stack) == 0 {
			p.complete(v, w)
		}
		return
	}
	// 沒有對應的窗：IRQ 在返回點送達造成同一 PC 的重複觀察，或開機時窗已開（§3.2.1 第 4a 條）。
	if lw := p.lastRetWin; p.haveRetWin && lw.r == r && lw.cs == cs && lw.ss == ss && lw.sp == sp && v.Deliveries() > lw.deliv {
		p.counts.ReturnDuplicateDelivered++
		p.lastRetWin.deliv = v.Deliveries()
		p.logWindow(PresentWindowRec{Steps: steps, Event: "ret-dup-delivered", Routine: r.name, CS: cs, SS: ss, SP: sp})
		return
	}
	p.counts.ReturnWithoutEntry++
	p.logWindow(PresentWindowRec{Steps: steps, Event: "ret-no-entry", Routine: r.name, CS: cs, SS: ss, SP: sp})
}

// checkTimeouts 是 §3.2.1 第 4 條：開著超過該常式逾時的窗（與壓在它上面的窗）關閉並記 WindowTimeout。不取像。
func (p *Presenter) checkTimeouts(steps uint64) {
	for i, w := range p.stack {
		if steps-w.entry < w.r.timeout {
			continue
		}
		for j := len(p.stack) - 1; j >= i; j-- {
			p.counts.WindowTimeout++
			p.closeWin(p.stack[j], steps, "timeout", j+1)
		}
		p.stack = p.stack[:i]
		if len(p.stack) == 0 {
			p.epWrote, p.lastClosed = false, nil
		}
		return
	}
}

// complete 是 §3.2.1 第 5 條：堆疊由非空變空的那一步。期間有歸屬的 VGA 寫入或有已關閉窗的狀態項目，
// 且最後關閉的寫 VGA 窗不是游標擦除窗，就取像。
func (p *Presenter) complete(v PresentView, last *presWindow) {
	wrote := p.epWrote
	lc := p.lastClosed
	p.epWrote, p.lastClosed = false, nil
	if !wrote && !p.closedItems() {
		return
	}
	if lc != nil && !lc.r.completion {
		return // 游標擦除窗（分期 (d) 的條目）：延到下一個游標繪製返回
	}
	if p.mut&presMutN2 != 0 {
		return
	}
	kind := last.kind
	if lc != nil {
		kind = lc.kind
	}
	if p.capture(v, kind, v.CS(), v.IP(), nil, nil, p.flightSet()) {
		p.scheduleRecheck()
	}
}

// ---- 取像（§3.3.1 第 3 條、§3.4.1） ----

// capture 建一個呈現像。idx 為 nil 時以 IndexedInto 讀實體 VGA（只在 mode 13h 鏈結時）；pal 為 nil 時讀 v.Palette()。
// v 只在 idx 或 pal 為 nil 時使用；在 Frame 內（沒有開著的視圖）呼叫時 v 為 nil，idx 與 pal 都必須給。
// 同一步依序：套用佇列 → Sync(Labels) → 飛行 Sync(F, presGen) → 色號與色盤 → 字幕狀態 → 建呈現像。
func (p *Presenter) capture(v PresentView, kind PresentKind, cs, ip uint16, idx []byte, pal *[256][3]uint8, F []FlightText) bool {
	if idx == nil {
		l := v.IndexedLayout()
		if l.Kind != machine.IndexedLinear || l.Mode != 0x13 {
			p.counts.NonLinearLayout++
			p.needFallback = true
			return false
		}
		if n := l.W * l.H; len(p.vga) < n {
			p.vga = make([]byte, n)
		}
		if _, ok := v.IndexedInto(p.vga); !ok {
			p.counts.IndexedIntoShort++
			return false
		}
		idx = p.vga[:64000]
	}
	img := &presentImage{indexed: append([]byte(nil), idx...)}
	if pal != nil {
		img.palette = *pal
	} else {
		img.palette = v.Palette()
	}
	p.applyQueue()
	img.labels, img.labelGen = p.labels.Visible(), p.labels.Generation()
	for _, sc := range p.cfg.Scales {
		p.ovs[sc].Sync(img.labels, img.labelGen)
	}
	if p.flight != nil {
		if !sameFlight(p.presSet, F) {
			p.presGen++
		}
		p.presSet = F
		img.flight, img.presGen = F, p.presGen
		for _, sc := range p.cfg.Scales {
			if fov := p.fovs[sc]; fov != nil {
				fov.Sync(F, p.presGen)
			}
		}
	}
	img.subPtr, img.subBlocked = p.subState(img.indexed)
	if img.subPtr != nil {
		img.sub = *img.subPtr
	}
	p.seq++
	img.info = PresentInfo{Seq: p.seq, CaptureSteps: p.steps, Kind: kind, SourceCS: cs, SourceIP: ip}
	p.cur = img
	p.vwSinceCap, p.stateSinceCap, p.frameSinceCap = 0, 0, false
	p.publish(img)
	return true
}

// derive 是只換色盤或字幕狀態的呈現像（§3.3.4、§3.4.3）：色號與覆繪狀態沿用，不重設未歸屬旗標。
func (p *Presenter) derive(kind PresentKind, pal [256][3]uint8, sub *Subtitle, blocked bool) {
	img := *p.cur
	img.palette = pal
	img.subPtr, img.subBlocked = sub, blocked
	img.sub = Subtitle{}
	if sub != nil {
		img.sub = *sub
	}
	p.seq++
	img.info = PresentInfo{Seq: p.seq, CaptureSteps: p.steps, Kind: kind}
	p.cur = &img
	p.publish(&img)
}

func (p *Presenter) publish(img *presentImage) {
	p.counts.Captures[img.info.Kind]++
	if len(p.captures) < 4*presMaxRecords {
		p.captures = append(p.captures, img.info)
	}
	for _, fn := range p.onCapture {
		fn(img.info)
	}
}

// subState 回 current() 與以 idx 判的阻擋狀態（§3.3.4）。
func (p *Presenter) subState(idx []byte) (*Subtitle, bool) {
	if p.subs == nil {
		return nil, false
	}
	c, ok := p.subs.current()
	if !ok {
		return nil, false
	}
	return c, !subRegionBlack(idx)
}

// flightSet 是非 CU 入口取像的飛行集合（§3.3.1 第 3 條）：最近被接受的 CU 已返回、尚未升格時用返回當下的快照，
// 否則用 Visible()。
func (p *Presenter) flightSet() []FlightText {
	if p.flight == nil {
		return nil
	}
	switch {
	case p.mut&presMutN22 != 0:
		return p.flight.Candidates()
	case p.mut&presMutN22b != 0:
		return p.flight.Visible()
	}
	if _, await := p.flight.CUState(); await && p.snapValid {
		return append([]FlightText(nil), p.cuSnap...)
	}
	return p.flight.Visible()
}

func (p *Presenter) scheduleRecheck() {
	if !p.cfg.Receipt {
		return
	}
	p.capN++
	if p.capN%presRecheckEvery == 0 {
		p.recheck = &presRecheck{img: p.cur.indexed, vw: p.vwTotal, seq: p.cur.info.Seq}
	}
}

// doRecheck 是重讀抽查（§4.2.2）：完成點取像後下一個觀察點、期間沒有 VideoWrite 時再讀一次，兩次必須相同。
func (p *Presenter) doRecheck(v PresentView) {
	r := p.recheck
	p.recheck = nil
	if p.vwTotal != r.vw {
		p.counts.RecaptureSkipped++
		return
	}
	l := v.IndexedLayout()
	if l.Kind != machine.IndexedLinear || l.Mode != 0x13 {
		p.counts.RecaptureSkipped++
		return
	}
	if n := l.W * l.H; len(p.tmp) < n {
		p.tmp = make([]byte, n)
	}
	if _, ok := v.IndexedInto(p.tmp); !ok {
		p.counts.RecaptureSkipped++
		return
	}
	p.counts.RecaptureChecked++
	if !bytes.Equal(p.tmp[:64000], r.img) {
		p.counts.RecaptureMismatch++
	}
}

// ---- CU（§3.4.2） ----

func (p *Presenter) onCU(e cuEvent) {
	switch e.Kind {
	case cuEvAccepted:
		p.cuPend, p.cuPendIP = true, e.IP
		p.cuOpen = true
		p.snapValid, p.cuSnap = false, nil
		p.lastRet, p.lastCH = presRet{}, presRet{}
	case cuEvDuplicate:
		p.counts.CUDuplicate++
	case cuEvCHNonZero:
		p.counts.CUCHNonZero++
		p.lastCH = presRet{valid: true, ip: e.IP, ss: e.V.SS(), sp: e.V.SP(), steps: e.V.Steps()}
	case cuEvVoided, cuEvTimeout:
		if e.Kind == cuEvVoided {
			p.counts.CUVoided++
		} else {
			p.counts.CUTimeout++
			p.cuOpen = false
		}
		if p.cu.valid && p.cu.entry == e.Entry.Steps {
			if p.cu.captured {
				p.counts.CUCapturedNotCopied++
				p.staleVoid = true
				steps := e.Steps
				if e.V != nil {
					steps = e.V.Steps()
				}
				p.cuRec(PresentCURec{Steps: steps, Class: "CUCapturedNotCopied", CUEntry: e.Entry.Steps, IP: e.Entry.IP, SS: e.Entry.SS, SP: e.Entry.SP})
			}
			p.cu = presCU{}
		}
	case cuEvReturn:
		if e.Outcome == cuReturnAccepted {
			p.cuOpen = false
			p.cuSnap, p.snapValid = p.flight.Candidates(), true
			p.afterRet, p.afterRetAt = true, e.V.Steps()
			p.lastRet = presRet{valid: true, ip: e.IP, ss: e.V.SS(), sp: e.V.SP(), deliv: viewDeliveries(e.V), steps: e.V.Steps()}
			p.lastCH = presRet{}
			return
		}
		p.classifyNoWindow(e)
	}
}

func viewDeliveries(v FlightView) uint64 {
	if d, ok := v.(interface{ Deliveries() uint64 }); ok {
		return d.Deliveries()
	}
	return 0
}

func cuReturnFor(ip uint16) uint16 {
	if ip == ipCUPlain {
		return ipCUPlainRet
	}
	return ipCUDiffRet
}

// classifyNoWindow 是 CU 返回時找不到開著的 CU 窗的四類（§3.4.2）。
func (p *Presenter) classifyNoWindow(e cuEvent) {
	v := e.V
	ip, ss, sp, d := e.IP, v.SS(), v.SP(), viewDeliveries(v)
	rec := PresentCURec{Steps: v.Steps(), CUEntry: e.Entry.Steps, IP: ip, SS: ss, SP: sp}
	lr := p.lastRet
	switch {
	case p.mut&presMutN33 == 0 && lr.valid && ip == lr.ip && ss == lr.ss && sp == lr.sp && d > lr.deliv:
		p.counts.CUReturnDelivered++
		p.lastRet.deliv = d
		return
	case p.lastCH.valid && ip == cuReturnFor(p.lastCH.ip) && ss == p.lastCH.ss && sp == p.lastCH.sp:
		p.counts.CUReturnCHNonZero++
		p.lastCH = presRet{}
		return
	case p.mut&presMutN35 == 0 && lr.valid && lr.ip == ipCUPlainRet && ip == ipCUDiffRet && ss == lr.ss && sp == lr.sp+cuViaPlainSPDelta:
		p.counts.CUReturnViaPlain++
		rec.Class = "CUReturnViaPlain"
		p.cuRec(rec)
		return
	}
	p.counts.CUReturnNoWindow++
	rec.Class = "CUReturnNoWindow"
	p.cuRec(rec)
}

func (p *Presenter) cuRec(r PresentCURec) {
	if len(p.cuRecs) < presMaxRecords {
		p.cuRecs = append(p.cuRecs, r)
	}
}

// cuEntryCapture 是被接受的 CU 入口（完成點，取 BufCU）。描述元：本次候選任一筆的 DS、Seg、List；本次候選為空時用
// 退用描述元；都沒有時等 CU 返回後的觀察點以 cu-return 取像。
func (p *Presenter) cuEntryCapture(v PresentView) {
	p.cu = presCU{valid: true, entry: v.Steps()}
	if p.mut&presMutN2 != 0 {
		return
	}
	cands := p.flight.Candidates()
	if p.mut&presMutN6 != 0 {
		if p.capture(v, PresCU, v.CS(), p.cuPendIP, nil, nil, cands) {
			p.cu.captured, p.cu.buf = true, p.cur.indexed
		}
		return
	}
	if b, ok := p.flight.Base(); !ok || !p.fbDesc.valid || b != p.fbDesc.base {
		p.fbDesc = presDesc{} // base 改變或停用時清除（§3.4.2）
	}
	var d presDesc
	fromFB := false
	switch {
	case len(cands) != 0 && (p.mut&presMutN30b == 0 || !p.fbDesc.valid):
		t := cands[0]
		d = presDesc{valid: true, ds: t.DS, seg: t.Seg, list: t.List, fromCandidates: true}
	case p.fbDesc.valid && (len(cands) != 0 || p.mut&presMutN30a == 0):
		d, fromFB = p.fbDesc, true
		p.counts.CUFallbackDescUsed++
	}
	if !d.valid {
		p.counts.CUNoDesc++
		p.cu.needReturn = true
		return
	}
	rd := d
	if fromFB && p.mut&presMutN34 != 0 {
		rd.list += 2 // 列表指標錯位一列
	}
	if !p.readBufCU(v, rd) {
		p.counts.BufCUReadFail++
		p.cu.needReturn = true
		return
	}
	if p.capture(v, PresCU, v.CS(), p.cuPendIP, p.bufC, nil, cands) {
		p.cu.captured, p.cu.buf, p.cu.desc, p.cu.fromFB = true, p.cur.indexed, d, fromFB
	}
}

// readSeg 以 段:位移 讀 len(dst) bytes（位移在 64 KiB 內環繞時切成兩段，454 §2.2）。
func readSeg(v PresentView, seg, off uint16, dst []byte) bool {
	if n := 0x10000 - int(off); n < len(dst) {
		a := v.ReadBlock(lin(seg, off), dst[:n])
		b := v.ReadBlock(lin(seg, 0), dst[n:])
		return a && b
	}
	return v.ReadBlock(lin(seg, off), dst)
}

// readBufCU 照描述元的列表逐列讀 200 列（不用描述元的矩形，§3.4.2）。
func (p *Presenter) readBufCU(v PresentView, d presDesc) bool {
	if len(p.bufC) != 64000 {
		p.bufC = make([]byte, 64000)
	}
	var tab [400]byte
	if !readSeg(v, d.ds, d.list, tab[:]) {
		return false
	}
	for y := 0; y < 200; y++ {
		off := uint16(tab[2*y]) | uint16(tab[2*y+1])<<8
		if !readSeg(v, d.seg, off, p.bufC[y*320:(y+1)*320]) {
			return false
		}
	}
	return true
}

// cuAfterReturn 是 CU 返回（配對成功）後的下一個觀察點：沒有 BufCU 時以 cu-return 取像；有時以 IndexedInto 讀 VGA
// 與 BufCU 比較（CUResidue），不同就另取 cu-return 呈現像；相等且 Seg = FFFF 時該描述元成為退用描述元。
func (p *Presenter) cuAfterReturn(v PresentView) {
	c := p.cu
	p.cu = presCU{}
	if p.mut&presMutN2 != 0 {
		return
	}
	if c.needReturn {
		if p.capture(v, PresCUReturn, v.CS(), v.IP(), nil, nil, p.flightSet()) {
			p.scheduleRecheck()
		}
		return
	}
	if !c.captured {
		return
	}
	l := v.IndexedLayout()
	if l.Kind != machine.IndexedLinear || l.Mode != 0x13 {
		p.counts.NonLinearLayout++
		return
	}
	if n := l.W * l.H; len(p.vga) < n {
		p.vga = make([]byte, n)
	}
	if _, ok := v.IndexedInto(p.vga); !ok {
		p.counts.IndexedIntoShort++
		return
	}
	vga := p.vga[:64000]
	if bytes.Equal(vga, c.buf) {
		// 退用描述元的資格（§3.4.2）：最近一次比對相等的 CU 所用的描述元，且 Seg = FFFF。
		if c.desc.valid && c.desc.seg == 0xFFFF {
			if b, ok := p.flight.Base(); ok {
				d := c.desc
				d.base = b
				p.fbDesc = d
			}
		}
		return
	}
	n, x0, y0, x1, y1 := diffBox(vga, c.buf)
	p.counts.CUResidue++
	rec := PresentCURec{Steps: v.Steps(), Class: "CUResidue", CUEntry: c.entry, Diff: n, X0: x0, Y0: y0, X1: x1, Y1: y1, FallbackDesc: c.fromFB}
	if c.fromFB {
		p.counts.CUResidueFallbackDesc++
		rec.Class = "CUResidueFallbackDesc"
	}
	p.cuRec(rec)
	if p.mut&presMutN27 == 0 {
		if p.capture(v, PresCUReturn, v.CS(), v.IP(), vga, nil, p.flightSet()) {
			p.scheduleRecheck()
		}
	}
}

// diffBox 回兩張 320×200 色號的差異點數與外接框（含端點）。
func diffBox(a, b []byte) (n, x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = 320, 200, -1, -1
	for i := range a {
		if i < len(b) && a[i] == b[i] {
			continue
		}
		n++
		x, y := i%320, i/320
		x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
	}
	return
}

// ---- VideoWrite、Frame、模式設定 ----

// VideoWrite 依 §3.2 歸屬（分期 (c)：CU 窗內非 V86 的寫入、開著的 blit 窗範圍內的寫入）。熱路徑不配置
// （未歸屬寫入的彙整只在新的 CS:IP 第一次出現時配置）。
func (p *Presenter) VideoWrite(w machine.VideoWrite) {
	p.vwTotal++
	p.vwSinceCap++
	if p.cuOpen && !w.Ctx.V86() {
		return
	}
	for i := len(p.stack) - 1; i >= 0; i-- {
		win := p.stack[i]
		if win.r.allCS || (w.CS == win.cs && w.IP >= win.r.ipLo && w.IP <= win.r.ipHi) {
			if !win.wrote {
				win.wrote, win.firstWrite = true, w.Step
			}
			p.epWrote = true
			return
		}
	}
	if w.Offset >= 64000 {
		p.counts.UnattributedOffscreen++
		return
	}
	p.counts.Unattributed++
	p.needFallback = true
	k := [2]uint16{w.CS, w.IP}
	if u := p.unattr[k]; u != nil {
		u.Count++
		u.Last = w.Step
	} else if len(p.unattr) < 4096 {
		p.unattr[k] = &PresentUnattr{CS: w.CS, IP: w.IP, Ctx: w.Ctx, Count: 1, First: w.Step, Last: w.Step}
	}
}

// VideoModeChange 是模式設定（§3.4.2 清除退用描述元；§3.5.1 的退路標記在分期 (f) 取像）。
func (p *Presenter) VideoModeChange(machine.ModeChange) {
	p.counts.ModeChanges++
	p.fbDesc = presDesc{}
	p.needFallback = true
}

// Frame 在 SessionObserver.Frame 的最後呼叫（Subtitles.Frame 之後）：丟升格後的快照、色盤與字幕呈現像、陳舊斷言。
func (p *Presenter) Frame(indexed []byte, palette [256][3]uint8) {
	p.counts.Frames++
	p.frameSinceCap = true
	if p.flight != nil {
		in, await := p.flight.CUState()
		p.cuOpen = in
		if p.snapValid && !await {
			p.snapValid, p.cuSnap = false, nil
		}
	}
	if p.mut&presMutN2 != 0 {
		// 負對照 N2：回掃取像，覆繪狀態取回掃當下。
		F := []FlightText(nil)
		if p.flight != nil {
			F = p.flight.Visible()
		}
		p.capture(nil, PresFrame, 0, 0, indexed, &palette, F)
	}
	if p.cur != nil && palette != p.cur.palette {
		sub, blocked := p.cur.subPtr, p.cur.subBlocked
		p.derive(PresPalette, palette, sub, blocked)
	}
	if p.cur != nil && p.subs != nil {
		sub, blocked := p.subState(p.cur.indexed)
		if sub != p.cur.subPtr || blocked != p.cur.subBlocked {
			p.derive(PresSubtitle, p.cur.palette, sub, blocked)
		}
	}
	p.staleCheck(indexed)
}

// staleCheck 是陳舊斷言（§4.2.2）：寫 VGA 的 2D 窗堆疊為空、不在 CU 窗內時，目前呈現像的色號必須等於該次回掃。
func (p *Presenter) staleCheck(indexed []byte) {
	for _, w := range p.stack {
		if w.wrote {
			return
		}
	}
	if p.cuOpen {
		return
	}
	rec := PresentStale{Steps: p.steps}
	if p.cur != nil {
		rec.Seq, rec.Kind = p.cur.info.Seq, p.cur.info.Kind
	}
	if p.staleVoid {
		p.staleVoid = false
		if p.cur == nil || !bytes.Equal(p.cur.indexed, indexed) {
			p.counts.StaleAfterCUVoid++
			rec.Class = "after-cu-void"
			if p.cur != nil {
				rec.Diff, rec.X0, rec.Y0, rec.X1, rec.Y1 = diffBox(p.cur.indexed, indexed)
			}
			p.staleRec(rec)
		}
		return
	}
	p.counts.StaleChecked++
	if p.cur == nil {
		p.counts.StaleNoImage++
		rec.Class = "no-image"
		p.staleRec(rec)
		return
	}
	if bytes.Equal(p.cur.indexed, indexed) {
		return
	}
	p.counts.Stale++
	rec.Class = "stale"
	rec.Diff, rec.X0, rec.Y0, rec.X1, rec.Y1 = diffBox(p.cur.indexed, indexed)
	p.staleRec(rec)
}

func (p *Presenter) staleRec(r PresentStale) {
	if len(p.stale) < presMaxRecords {
		p.stale = append(p.stale, r)
	}
}

// ---- 收據（§3.9） ----

// shotCheck 出 shot（或停止點）的呈現合成與差異斷言：三個條件全部成立時，呈現合成與現行合成要逐位元組相同。
// cur 是現行合成（已畫字幕）；subs 是現行合成畫字幕的開關。
func (p *Presenter) shotCheck(cur map[int][]byte) *PresentShot {
	ps := &PresentShot{Info: p.Current(), Composites: map[int][]byte{}, Equal: map[int]bool{}}
	if p.cur == nil {
		ps.Why = append(ps.Why, "沒有呈現像")
		return ps
	}
	for _, sc := range p.cfg.Scales {
		c, _, _ := p.Compose(sc)
		ps.Composites[sc] = append([]byte(nil), c...)
	}
	if !p.frameSinceCap {
		ps.Why = append(ps.Why, "最後一次取像不在最後一次回掃之前")
	}
	if p.vwSinceCap != 0 {
		ps.Why = append(ps.Why, fmt.Sprintf("取像後有 %d 筆 VideoWrite", p.vwSinceCap))
	}
	if p.stateSinceCap != 0 {
		ps.Why = append(ps.Why, fmt.Sprintf("取像後有 %d 次狀態改變", p.stateSinceCap))
	}
	if sub, blocked := p.subState(p.cur.indexed); sub != p.cur.subPtr || blocked != p.cur.subBlocked {
		ps.Why = append(ps.Why, "取像後字幕狀態改變")
	}
	if k := p.cur.info.Kind; k == PresPalette || k == PresSubtitle {
		ps.Why = append(ps.Why, "最後一次取像是 "+string(k))
	}
	ps.Cond = len(ps.Why) == 0
	for _, sc := range p.cfg.Scales {
		ps.Equal[sc] = bytes.Equal(ps.Composites[sc], cur[sc])
	}
	return ps
}

// ---- 輸出 ----

// CountsText 是 counts.txt 的內容。
func (p *Presenter) CountsText() []byte {
	var b bytes.Buffer
	c := p.counts
	fmt.Fprintf(&b, "呈現層（規格 014 分期 (c)）；突變：%s\n", p.mut)
	var ks []string
	for k := range c.Captures {
		ks = append(ks, string(k))
	}
	sort.Strings(ks)
	fmt.Fprintf(&b, "回掃 %d；呈現像", c.Frames)
	for _, k := range ks {
		fmt.Fprintf(&b, " %s %d", k, c.Captures[PresentKind(k)])
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "陳舊斷言：檢查 %d、陳舊 %d、無呈現像 %d、CU 作廢後 StaleAfterCUVoid %d\n", c.StaleChecked, c.Stale, c.StaleNoImage, c.StaleAfterCUVoid)
	fmt.Fprintf(&b, "正式組要求 0：QueueOverflow %d、WindowTimeout %d、ModeTimeout %d、DuplicateEntry %d、WindowSkipped %d、RecaptureMismatch %d、CUReturnNoWindow %d、CUResidueFallbackDesc %d\n",
		c.QueueOverflow, c.WindowTimeout, c.ModeTimeout, c.DuplicateEntry, c.WindowSkipped, c.RecaptureMismatch, c.CUReturnNoWindow, c.CUResidueFallbackDesc)
	fmt.Fprintf(&b, "只記錄：CUResidue %d、StaleAfterCUVoid %d、CUCapturedNotCopied %d、CUReturnDelivered %d、CUReturnCHNonZero %d、CUReturnViaPlain %d、DuplicateDelivered %d、ReturnWithoutEntry %d、ReturnDuplicateDelivered %d\n",
		c.CUResidue, c.StaleAfterCUVoid, c.CUCapturedNotCopied, c.CUReturnDelivered, c.CUReturnCHNonZero, c.CUReturnViaPlain, c.DuplicateDelivered, c.ReturnWithoutEntry, c.ReturnDuplicateDelivered)
	fmt.Fprintf(&b, "重讀抽查：比對 %d、略過（期間有 VideoWrite 等）%d、不符 %d\n", c.RecaptureChecked, c.RecaptureSkipped, c.RecaptureMismatch)
	fmt.Fprintf(&b, "CU：無描述元 %d、用退用描述元 %d、BufCU 讀取失敗 %d、重複入口 %d、CH≠0 %d、作廢 %d、逾時 %d\n",
		c.CUNoDesc, c.CUFallbackDescUsed, c.BufCUReadFail, c.CUDuplicate, c.CUCHNonZero, c.CUVoided, c.CUTimeout)
	fmt.Fprintf(&b, "窗：開窗 %d、最大深度 %d；未歸屬寫入 %d（畫面外 %d，退路取像在分期 (f)）；模式設定 %d；非 13h 鏈結 %d；IndexedIntoShort %d\n",
		c.WindowsOpened, c.MaxDepth, c.Unattributed, c.UnattributedOffscreen, c.ModeChanges, c.NonLinearLayout, c.IndexedIntoShort)
	fmt.Fprintf(&b, "退路 %d、CursorEraseFlush %d（分期 (c) 不做退路取像）；佇列剩 %d 項\n", c.Fallback, c.CursorEraseFlush, len(p.queue))
	return b.Bytes()
}

// WriteFiles 寫 captures.tsv、stale.tsv、unattributed.tsv、windows.tsv、cu.tsv、present-counts.txt 到 dir。
func (p *Presenter) WriteFiles(dir string, write func(name string, b []byte) error) error {
	var b bytes.Buffer
	b.WriteString("seq\tsteps\tkind\tsource\n")
	for _, c := range p.captures {
		fmt.Fprintf(&b, "%d\t%d\t%s\t%04X:%04X\n", c.Seq, c.CaptureSteps, c.Kind, c.SourceCS, c.SourceIP)
	}
	if err := write("captures-present.tsv", b.Bytes()); err != nil {
		return err
	}
	b.Reset()
	b.WriteString("steps\tclass\tseq\tkind\tdiff\tx0\ty0\tx1\ty1\n")
	for _, s := range p.stale {
		fmt.Fprintf(&b, "%d\t%s\t%d\t%s\t%d\t%d\t%d\t%d\t%d\n", s.Steps, s.Class, s.Seq, s.Kind, s.Diff, s.X0, s.Y0, s.X1, s.Y1)
	}
	if err := write("stale.tsv", b.Bytes()); err != nil {
		return err
	}
	b.Reset()
	b.WriteString("cs\tip\tctx\tcount\tfirst\tlast\n")
	var us []*PresentUnattr
	for _, u := range p.unattr {
		us = append(us, u)
	}
	sort.Slice(us, func(i, j int) bool { return us[i].First < us[j].First })
	for _, u := range us {
		fmt.Fprintf(&b, "%04X\t%04X\t%d\t%d\t%d\t%d\n", u.CS, u.IP, u.Ctx, u.Count, u.First, u.Last)
	}
	if err := write("unattributed.tsv", b.Bytes()); err != nil {
		return err
	}
	b.Reset()
	b.WriteString("steps\tevent\troutine\tcs\tss\tsp\tdepth\twrote\tlen\tkind\n")
	for _, w := range p.windows {
		fmt.Fprintf(&b, "%d\t%s\t%s\t%04X\t%04X\t%04X\t%d\t%t\t%d\t%s\n", w.Steps, w.Event, w.Routine, w.CS, w.SS, w.SP, w.Depth, w.Wrote, w.Len, w.Kind)
	}
	if err := write("windows.tsv", b.Bytes()); err != nil {
		return err
	}
	b.Reset()
	b.WriteString("steps\tclass\tcu_entry\tip\tss\tsp\tdiff\tx0\ty0\tx1\ty1\tfallback_desc\n")
	for _, r := range p.cuRecs {
		fmt.Fprintf(&b, "%d\t%s\t%d\t%04X\t%04X\t%04X\t%d\t%d\t%d\t%d\t%d\t%t\n", r.Steps, r.Class, r.CUEntry, r.IP, r.SS, r.SP, r.Diff, r.X0, r.Y0, r.X1, r.Y1, r.FallbackDesc)
	}
	if err := write("cu.tsv", b.Bytes()); err != nil {
		return err
	}
	return write("present-counts.txt", p.CountsText())
}
