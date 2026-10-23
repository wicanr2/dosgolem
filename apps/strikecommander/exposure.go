package strikecommander

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wicanr2/dosgolem/session"
)

// strike_commander 規格 014 §4.2 的英文曝光收據（分期 (b)）：墨跡台帳（§4.2.2）、合成點（§4.2.1）與判定。
// 分期 (b) 只有基線：呈現用的是「現行合成」（畫格取最後一次回掃、覆繪狀態取合成當下），以一組自有的 Overlay、
// FlightOverlay 實例重現，不碰收據的觀察用實例；台帳另有第三組實例（§4.2.2 判定第 2 條）。
//
// 台帳只由觀察事件（puts 返回、shape 入口、飛行字串通過核對、提交與升格、繪圖事件）與原版資料（自備的 TRE／PAK
// 讀取器）建立，不讀呈現用實例。共用的只有收件過濾與圖形字墨跡遮罩（§4.2.2 範圍說明）。

const (
	// expoInterval 是合成點間隔：c_n = boot + φ + n × 192,500，φ 取 0、48,125、96,250、144,375（§4.2.1）。
	// 四個相位合在同一次執行：合成點只切分推進，同狀態（§4.4 的 A′）由既有收據逐位元組相同驗證。
	expoInterval = 48_125
	expoPhases   = 4

	// OPTTEST 游標繪製呼叫 shape 的返回位址 +3745:0649（RE 015 §6，規格 014 §2 游標列）。
	segCursor   = 0x3745
	ipCursorRet = 0x0649
)

// expoMut 是負對照的突變旗標（規格 014 §4.5）。零值是正確的收據。
type expoMut uint8

const (
	// expoMutN1：合成點改成每個狀態改變事件之後（提交、繪圖事件、清除、升格、可見集合世代改變），以現行合成出圖（N1）。
	expoMutN1 expoMut = 1 << iota
	// expoMutLedgerFromVisible：台帳的標籤條目只看呈現用 Overlay 此刻的可見集合（N8 的第一個突變）。
	expoMutLedgerFromVisible
	// expoMutObsStamps：台帳改用觀察用實例取 stamp 像素（N29）。
	expoMutObsStamps
)

func (m expoMut) String() string {
	var s []string
	if m&expoMutN1 != 0 {
		s = append(s, "N1")
	}
	if m&expoMutLedgerFromVisible != 0 {
		s = append(s, "台帳讀可見集合")
	}
	if m&expoMutObsStamps != 0 {
		s = append(s, "台帳用觀察用實例")
	}
	if len(s) == 0 {
		return "無"
	}
	return strings.Join(s, "+")
}

// ExpoConfig 是曝光收據的設定。Root 是台帳 TRE 讀取器的 C: 根目錄；空字串時用 ReceiptConfig.SaveRoot。
type ExpoConfig struct {
	Root string
	mut  expoMut
	// DumpAt、DumpDir 只供審查：在第一個不早於 DumpAt[i]、且有條目在畫面上的合成點，把 3 倍的現行合成圖、
	// 台帳合成圖與原版放大圖寫成 PNG（dump-<步數>-{comp,ledger,plain}.png）。
	DumpAt  []uint64
	DumpDir string
}

// ExpoCapture 是一個合成點（基線的「呈現像」即現行合成）。Phase −1 表示 N1 模式的事件合成點。
type ExpoCapture struct {
	Phase      int
	Steps      uint64
	Frames     int
	OnScreen   int         // 在畫面上的台帳條目數
	Exposed    map[int]int // 各倍率有露出的條目數
	Cached     bool        // 與上一個合成點的輸入完全相同，沿用判定
	NoFrame    bool
	FrameSteps uint64 // 最後一次回掃的步數（呈現層：呈現像的取像步數）
	// Seq、Kind 是呈現層的呈現像（基線為 0、空字串）。
	Seq  uint64
	Kind PresentKind
}

// ExpoExposure 是一個合成點、一個倍率、一個台帳條目的露出。
type ExpoExposure struct {
	Phase      int
	Steps      uint64
	Scale      int
	Entry      int
	Kind, Key  string
	EntrySteps uint64
	Class      string // 游標、未覆繪、退路、其他（§4.2.2 分類，依序）
	Ink        int    // 條目的墨跡點數
	Visible    int    // 呈現像色號等於記錄色的點數
	Exposed    int    // 在畫面上、不照畫的點數
	Cursor     int    // 其中落在游標範圍的點數
	Trail      int    // 其中不在游標範圍、落在游標殘跡欄的點數（§4.2.2 分類 3，第 10 版）
	// ExposedLit 是照字面規則（區塊內合成圖與原版放大圖至少一點不同）的露出點數。文字 stamp 的筆畫色常與英文墨跡同色
	// （標籤白字 15、飛行字色取英文墨跡眾數），s×s 區塊整塊被同色筆畫蓋住時字面規則會誤判露出；Exposed 另把「台帳自有
	// 實例回報照畫的遮蓋範圍內」的區塊算進「與原版不同」（見 judgeEntry）。兩者並列，供主代理審查。
	ExposedLit int
	// Contrast 是露出點中、在呈現像色盤下與八鄰點至少一點 RGB 不同（看得見）的點數。0 時分類記「色盤不可見」
	// （規格以外的補充分類：色號仍是英文墨跡、但色盤淡出等使墨跡與周圍同色）。
	Contrast int
	// Gone 只對標籤、計分板：條目的 Key 不在觀察器此刻的可見集合（「標籤消失」的露出，N8 的量測）。
	Gone bool
	// ExX0..ExY1 是露出點的外接框（含端點，1 倍）；CurX、CurY 是分類用的游標位置（−1 為未知）。供審查。
	ExX0, ExY0, ExX1, ExY1 int
	CurX, CurY             int
}

// LedgerEntry 是台帳一筆條目的中繼資料（ledger.tsv）。
type LedgerEntry struct {
	ID             int
	Kind, Key      string
	Steps          uint64
	X0, Y0, X1, Y1 int // 墨跡外接框 [X0, X1) × [Y0, Y1)
	Points         int
	Group2         bool
	Replaced       uint64 // 被同位置新條目取代的步數；0 為停止時仍在台帳
	Retired        uint64 // 取代後第一個不在畫面上的合成點（移出台帳）；0 為未移出
}

// ExpoResult 是曝光收據。
type ExpoResult struct {
	Mut         string
	Present     bool // 判定用呈現層的呈現像（分期 (c)）；假為基線（現行合成）
	Captures    []ExpoCapture
	Exposures   []ExpoExposure
	Ledger      []LedgerEntry
	PlateEvents int    // 主選單底板 shape 入口數
	ReaderTRE   string // 台帳讀取器採用的 TRE；空字串為沒讀或失敗
	ReaderErr   string // 台帳讀取器失敗原因（有底板入口時才讀；非空即驗收失敗，§3.6.4 第 3 項）
	Errors      []string
	Digest      session.StateDigest
}

// ---- 台帳 ----

// 台帳條目的種類。
const (
	ledLabel   = "label"
	ledBoard   = "board"
	ledArt     = "art"    // 圖形字（SAVELOAD、MAINMENU 按鈕、TOMENU）
	ledConfirm = "k15-yn" // k15 框內的 YES、NO
	ledK0      = "k0"     // k0 內建字（獨立算法）
	ledPlate   = "plate"  // 主選單底板灰字（獨立讀取器）
	ledHUD     = "hud"    // HUD 狀態字
	ledMFD     = "mfd"    // MFD
	ledPlaque  = "plaque" // 訊息橫幅
)

type ledgerEntry struct {
	id       int
	kind     string
	key      string
	steps    uint64
	pts      []int32 // y*320+x
	cols     []uint8
	x0, y0   int
	x1, y1   int
	art      bool // 判定以「p 所在欄在字帶內的整條」為區塊
	by0, by1 int  // art：字帶列 [by0, by1)（畫面座標）
	group2   bool
	hud      bool
}

func (e *ledgerEntry) finish() {
	e.x0, e.y0, e.x1, e.y1 = 320, 200, -1, -1
	for _, p := range e.pts {
		x, y := int(p%320), int(p/320)
		e.x0, e.y0, e.x1, e.y1 = min(e.x0, x), min(e.y0, y), max(e.x1, x+1), max(e.y1, y+1)
	}
}

func (e *ledgerEntry) addPt(x, y int, c uint8) {
	if x < 0 || x >= 320 || y < 0 || y >= 200 {
		return
	}
	e.pts = append(e.pts, int32(y*320+x))
	e.cols = append(e.cols, c)
}

func sameInk(a, b *ledgerEntry) bool {
	if a.kind != b.kind || a.key != b.key || a.group2 != b.group2 || a.by0 != b.by0 || a.by1 != b.by1 || len(a.pts) != len(b.pts) {
		return false
	}
	for i := range a.pts {
		if a.pts[i] != b.pts[i] || a.cols[i] != b.cols[i] {
			return false
		}
	}
	return true
}

// expoLedger 是墨跡台帳（§4.2.2）與它自有的第三組 Overlay、FlightOverlay（判定第 2 條）。
type expoLedger struct {
	entries  []*ledgerEntry
	retiring []*ledgerEntry // 已被取代、還沒確認離開畫面的條目
	recs     []LedgerEntry
	ver      uint64

	ovs  map[int]*Overlay
	fovs map[int]*FlightOverlay
	// 台帳自有實例的集合：由提交與升格事件收下、同位置取代，不因可見集合縮小而移除（顯示由 Ref 比對決定）。
	labels    []Label
	labelGen  uint64
	flights   []FlightText
	flightGen uint64

	// 最近一次 shape 入口的檔（k0 衍生列用）。
	lastKind ArtFile
	lastFile []byte
	lastE    int

	root        string
	readerTried bool
	menu        *ledgerMenuFiles
	readerErr   string
	readerTRE   string
	plates      int

	cursor *Point
	// trail 是游標殘跡欄（§4.2.2 分類 3，第 10 版）：自上次對話框重畫（k0、主選單底板的 shape 入口）以來，
	// 游標繪製（OnCursor）的 11×11 矩形覆蓋過的欄。TOMENU 底板沒有觀察事件（只有按鈕檔有雜湊），不重設。
	trail [320]bool
}

// cursorAt 記游標位置並把它覆蓋的欄加入游標殘跡欄。
func (l *expoLedger) cursorAt(x, y int) {
	l.cursor = &Point{x, y}
	for c := max(x-5, 0); c < min(x+6, 320); c++ {
		l.trail[c] = true
	}
}

// add 加入一筆條目：已有逐點相同的條目時不變；否則移除墨跡外接框重疊的舊條目（被同位置新條目取代），再加入。
func (l *expoLedger) add(e *ledgerEntry) {
	if len(e.pts) == 0 {
		return
	}
	e.finish()
	for _, o := range l.entries {
		if sameInk(o, e) {
			return
		}
	}
	revived := false
	for i, o := range l.retiring {
		if sameInk(o, e) { // 同一串又畫回原位：從退場清單收回，不另建條目
			l.retiring = append(l.retiring[:i], l.retiring[i+1:]...)
			l.recs[o.id].Replaced = 0
			e, revived = o, true
			break
		}
	}
	keep := l.entries[:0]
	for _, o := range l.entries {
		if o.x0 < e.x1 && e.x0 < o.x1 && o.y0 < e.y1 && e.y0 < o.y1 {
			// 被同位置新條目取代。新條目在 puts 返回或 shape 入口就到（畫進畫外緩衝或正要畫），舊英文仍在畫面上直到
			// blit／CU 或繪製完成：舊條目移到退場清單，照常判定，直到某個合成點不在畫面上才移出（見 judge 的退場清單）。
			l.recs[o.id].Replaced = e.steps
			l.retiring = append(l.retiring, o)
			continue
		}
		keep = append(keep, o)
	}
	for i := len(keep); i < len(l.entries); i++ {
		l.entries[i] = nil
	}
	l.entries = append(keep, e)
	if !revived {
		e.id = len(l.recs)
		l.recs = append(l.recs, LedgerEntry{ID: e.id, Kind: e.kind, Key: e.key, Steps: e.steps, X0: e.x0, Y0: e.y0, X1: e.x1, Y1: e.y1, Points: len(e.pts), Group2: e.group2})
	}
	l.ver++
}

// onPutsReturn：標籤、計分板（§4.2.2 表第 1 列）。前後列有效時，Box 內 After ≠ Before 的點，色 After。
func (l *expoLedger) onPutsReturn(lb Label) {
	h := refRows(lb.Kind)
	if !validRows(lb.Before, h) || !validRows(lb.After, h) {
		return
	}
	kind := ledLabel
	if lb.Kind == KindBoard {
		kind = ledBoard
	}
	e := &ledgerEntry{kind: kind, key: lb.Key, steps: lb.Steps}
	x0, y0, x1, y1 := lb.Box()
	for y := y0; y < y1; y++ {
		r := (y - (lb.Y + 1)) * 320
		if r < 0 || r+320 > len(lb.After) {
			continue
		}
		for x := max(x0, 0); x < min(x1, 320); x++ {
			if lb.After[r+x] != lb.Before[r+x] {
				e.addPt(x, y, lb.After[r+x])
			}
		}
	}
	l.add(e)
}

// onAccepted：HUD、MFD、橫幅（§4.2.2 表最後一列）。InkBox 內 After ≠ Before 的點，色 After。HUD、MFD 只取 catalog key
// 的範圍 [Anchor, X1)（數字前綴等保留英文的部分不記，009 §1.2）；橫幅取整個 InkBox。
func (l *expoLedger) onAccepted(t FlightText) {
	kind := ledPlaque
	switch t.Kind {
	case KindHUD:
		kind = ledHUD
	case KindMFD:
		kind = ledMFD
	}
	e := &ledgerEntry{kind: kind, key: t.Key, steps: t.Steps, hud: t.Kind == KindHUD}
	x0, y0, x1, y1 := t.InkBox()
	if t.Kind != KindPlaque {
		x0 = max(x0, t.Anchor())
	}
	if len(t.Before) != len(t.After) {
		return
	}
	for y := y0; y < y1; y++ {
		r := (y - t.B0) * 320
		if r < 0 || r+320 > len(t.After) {
			continue
		}
		for x := max(x0, 0); x < min(x1, 320); x++ {
			if t.After[r+x] != t.Before[r+x] {
				e.addPt(x, y, t.After[r+x])
			}
		}
	}
	l.add(e)
}

// onCommit：台帳自有 Overlay 的標籤集合（同位置取代）。Overlay 以 Ref 比對決定每筆是否照畫。
func (l *expoLedger) onCommit(c Commit) {
	prev := l.labels
	next := append([]Label(nil), prev...)
	for _, lb := range c.Labels {
		x0, y0, x1, y1 := lb.Box()
		keep := next[:0]
		for _, o := range next {
			a0, b0, a1, b1 := o.Box()
			if a0 < x1 && x0 < a1 && b0 < y1 && y0 < b1 {
				continue
			}
			keep = append(keep, o)
		}
		next = append(keep, lb)
	}
	l.labels = next
	if !sameLabels(prev, next) {
		l.labelGen++
	}
}

// onFlightCommit：台帳自有 FlightOverlay 的集合（升格時的可見集合，同位置取代）。
func (l *expoLedger) onFlightCommit(vis []FlightText) {
	prev := l.flights
	next := append([]FlightText(nil), prev...)
	for _, t := range vis {
		x0, x1, y0, y1 := t.Anchor(), t.X1, t.B0, t.B0+t.BH
		if t.Kind == KindPlaque {
			x0 = t.X0
		}
		keep := next[:0]
		for _, o := range next {
			a0, a1, b0, b1 := o.Anchor(), o.X1, o.B0, o.B0+o.BH
			if o.Kind == KindPlaque {
				a0 = o.X0
			}
			if a0 < x1 && x0 < a1 && b0 < y1 && y0 < b1 {
				continue
			}
			keep = append(keep, o)
		}
		next = append(keep, t)
	}
	l.flights = next
	if !sameFlight(prev, next) {
		l.flightGen++
	}
}

// k0Derived 是 k0 內建三塊的位移與條目（規格 014 §3.3.2 第 1 條；探針報告 P11）。
var k0Derived = [3]struct{ k, dx, dy int }{{2, 54, 97}, {6, 10, 112}, {8, 94, 112}}

// onArt：台帳自有 Overlay 收被接受的繪圖事件；k0 另由台帳自己規劃三個衍生列（§4.2.2 判定第 2 條）：位置照 RE 015 §5，
// 條目由事件帶來的檔案（shape 入口時讀下的 SAVELOAD.SHP）解碼同組 k2、k6、k8 一般態。不經呈現層。
// 台帳不收 ClearArt：圖面以逐欄比對原版基準決定顯示，留著的列只在畫面仍是原版英文時照畫。
func (l *expoLedger) onArt(ev ArtEvent) {
	evs := []ArtEvent{ev}
	if ev.File == ArtSaveLoad && ev.K == 0 && l.lastKind == ArtSaveLoad && l.lastFile != nil && l.lastE == ev.G*saveLoadGroup {
		for _, d := range k0Derived {
			el := artElements[d.k]
			ent, err1 := decodeEntry(l.lastFile, ev.G*saveLoadGroup+d.k)
			b0, err2 := decodeEntry(l.lastFile, d.k)
			if err1 != nil || err2 != nil || ent.W != el.w || ent.H != el.h || b0.W != el.w || b0.H != el.h {
				continue
			}
			evs = append(evs, ArtEvent{Steps: ev.Steps, File: ArtSaveLoad, G: ev.G, K: d.k, X: ev.X + d.dx, Y: ev.Y + d.dy,
				Order: ev.Order, W: el.w, H: el.h, Entry: ent.Pix, Base0: b0.Pix})
		}
	}
	for _, ov := range l.ovs {
		for _, e := range evs {
			ov.AddDialogArt(e)
		}
	}
}

// onShapeArt：圖形字的墨跡（§4.2.2 表第 2 到 4 列）。
func (l *expoLedger) onShapeArt(steps uint64, kind ArtFile, file []byte, e, x, y int) {
	l.lastKind, l.lastFile, l.lastE = kind, file, e
	if (kind == ArtSaveLoad && e%saveLoadGroup == 0 && e/saveLoadGroup < 3) || (kind == ArtMainMenuPlate && e == 0) {
		l.trail = [320]bool{} // 對話框重畫：游標殘跡欄重設
	}
	switch kind {
	case ArtSaveLoad:
		l.saveLoadInk(steps, file, e, x, y)
	case ArtMainMenuButtons:
		if e < 0 || e >= 2*mmLines {
			return
		}
		normal, pressed, err := mmPair(file, e%mmLines)
		if err != nil {
			return
		}
		ent, err := decodeEntry(file, e)
		if err != nil || ent.W != mmW || ent.H != mmH {
			return
		}
		m, _ := ruleP(normal, pressed) // 008 §3.3 規則 P（共用遮罩）
		le := &ledgerEntry{kind: ledArt, key: mmKeys[e%mmLines], steps: steps, art: true, by0: y + mmBand.y0, by1: y + mmBand.y1 + 1}
		for i, on := range m {
			if on && !bMainMenu[ent.Pix[i]] {
				le.addPt(x+i%mmW, y+i/mmW, ent.Pix[i])
			}
		}
		l.add(le)
	case ArtMainMenuPlate:
		if e != 0 {
			return
		}
		l.plates++
		l.plateInk(steps, x, y)
	case ArtTomenuButtons:
		if e < 0 || e >= len(tmSizes) {
			return
		}
		ent, err := decodeEntry(file, e)
		if err != nil || ent.W != tmSizes[e][0] || ent.H != tmSizes[e][1] {
			return
		}
		sp, ok := artSpecFor(ArtEvent{File: ArtTomenuButtons, K: e})
		if !ok {
			return
		}
		m := tomenuMask(ent.Pix, sp)
		le := &ledgerEntry{kind: ledArt, key: sp.key, steps: steps, art: true, by0: y + sp.band.y0, by1: y + sp.band.y1 + 1}
		for i, on := range m {
			if on {
				le.addPt(x+i%sp.w, y+i/sp.w, ent.Pix[i])
			}
		}
		l.add(le)
	}
}

// saveLoadInk：SAVELOAD 三組（含第 2 組）。k0 只走獨立算法（k0Ink）；其他有字的元素依 004 §3.4 的判定框與底色集合
// （dialogMask，共用）在條目上算，色 Entry；k15 另記框內 YES、NO（RE 017 §3：與 k17、k19 同位置，位移 (18,25)、(69,25)）。
func (l *expoLedger) saveLoadInk(steps uint64, file []byte, e, x, y int) {
	g, k := e/saveLoadGroup, e%saveLoadGroup
	if g >= 3 || (k >= 10 && k <= 14) {
		return
	}
	ent, err := decodeEntry(file, e)
	if err != nil {
		return
	}
	if normalK(k) == 0 {
		if ent.W == k0W && ent.H == k0H {
			for _, le := range k0Ink(ent.Pix, x, y, steps) {
				le.group2 = g == 2
				l.add(le)
			}
		}
		return
	}
	el, _, ok := elementFor(k)
	if !ok {
		return
	}
	b0, err := decodeEntry(file, k)
	if err != nil || ent.W != el.w || ent.H != el.h || b0.W != el.w || b0.H != el.h {
		return
	}
	ev := ArtEvent{G: g, K: k, X: x, Y: y, W: el.w, H: el.h, Entry: ent.Pix, Base0: b0.Pix}
	_, _, mask := dialogMask(ev)
	le := &ledgerEntry{kind: ledArt, key: el.key, steps: steps, art: true, by0: y + el.band.y0, by1: y + el.band.y1 + 1, group2: g == 2}
	for i, on := range mask {
		if on {
			le.addPt(x+i%el.w, y+i/el.w, ent.Pix[i])
		}
	}
	l.add(le)
	if k != 15 {
		return
	}
	for _, yn := range [2]struct{ k, dx, dy int }{{17, 18, 25}, {19, 69, 25}} {
		sub := artElements[yn.k]
		jb := judgeBox(yn.k)
		jb = box{jb.x0 + yn.dx, jb.y0 + yn.dy, jb.x1 + yn.dx, jb.y1 + yn.dy}
		m := inkMask(b0.Pix, el.w, el.h, jb, &bgConfirm)
		c := &ledgerEntry{kind: ledConfirm, key: sub.key, steps: steps, art: true,
			by0: y + yn.dy + sub.band.y0, by1: y + yn.dy + sub.band.y1 + 1, group2: g == 2}
		for i, on := range m {
			if on {
				c.addPt(x+i%el.w, y+i/el.w, ent.Pix[i])
			}
		}
		l.add(c)
	}
}

// k0 的尺寸（RE 015 §5：對話框 (80,30)–(261,160)）。
const k0W, k0H = 182, 131

// k0Text 是 k0 內建字的獨立算法常數（規格 014 §4.2.2 表第 3 列）：RE 015 §5 的四個字面矩形（營房絕對座標換成
// 相對 k0 左上角 (80,30)，含端點），每框往下延 1 列含陰影；band 是判定區塊用的字帶（相對 k0、含端點：標題取 k0 的字帶，
// 三個按鈕取 k2、k6、k8 的字帶平移到 P11 的位移）。底色集合標題 {9, 13}、按鈕 {8, 9, 13}（RE 015 §5 的刷紋色）。
// 這些常數另抄，不引用 artElements、judgeBox、bgTitle、bgButton。
var k0Text = [4]struct {
	key                string
	x0, y0, x1, y1     int
	title              bool
	bx0, by0, bx1, by1 int
}{
	{"SAVE GAME", 127 - 80, 39 - 30, 213 - 80, 44 - 30 + 1, true, 44, 6, 139, 17},
	{"SAVE", 158 - 80, 130 - 30, 185 - 80, 134 - 30 + 1, false, 54 + 20, 97 + 2, 54 + 59, 97 + 10},
	{"CANCEL", 105 - 80, 145 - 30, 153 - 80, 149 - 30 + 1, false, 10 + 12, 112 + 1, 10 + 67, 112 + 9},
	{"QUIT TO DOS", 177 - 80, 145 - 30, 248 - 80, 149 - 30 + 1, false, 94 + 2, 112 + 1, 94 + 77, 112 + 9},
}

var (
	k0BgTitle  = [256]bool{9: true, 13: true}
	k0BgButton = [256]bool{8: true, 9: true, 13: true}
)

// k0Ink 是 k0 內建字的獨立算法：每個字面矩形內、k0 Entry 色號不屬於底色集合的點為墨跡，色為 Entry。
func k0Ink(entry []uint8, x, y int, steps uint64) []*ledgerEntry {
	var out []*ledgerEntry
	for _, r := range k0Text {
		bg := &k0BgButton
		if r.title {
			bg = &k0BgTitle
		}
		e := &ledgerEntry{kind: ledK0, key: r.key, steps: steps, art: true, by0: y + r.by0, by1: y + r.by1 + 1}
		for yy := r.y0; yy <= r.y1; yy++ {
			for xx := r.x0; xx <= r.x1; xx++ {
				if c := entry[yy*k0W+xx]; !bg[c] {
					e.addPt(x+xx, y+yy, c)
				}
			}
		}
		out = append(out, e)
	}
	return out
}

// plateInk：主選單底板灰字（§4.2.2 表第 4 列）。台帳自備讀取器讀 MAINMENU.PAK，依 008 §3.3 規則 Q（共用 ruleP、ruleQ）
// 算五行：規則 Q 回填後與底板不同的點為墨跡，色為底板條目；平移到底板位置。
func (l *expoLedger) plateInk(steps uint64, x, y int) {
	if !l.readerTried {
		l.readerTried = true
		m, err := ledgerReadMainMenu(l.root)
		if err != nil {
			l.readerErr = err.Error()
		} else {
			l.menu, l.readerTRE = m, m.tre
		}
	}
	if l.menu == nil {
		return
	}
	plate := l.menu.plate
	for n := 0; n < mmLines; n++ {
		normal, pressed, err := mmPair(l.menu.buttons, n)
		if err != nil {
			l.readerErr = err.Error()
			return
		}
		m, ref := ruleP(normal, pressed)
		bg, _ := ruleQ(plate, n, m, ref)
		pb := plateBand(n)
		bw := pb.x1 - pb.x0 + 1
		e := &ledgerEntry{kind: ledPlate, key: mmKeys[n], steps: steps, art: true, by0: y + pb.y0, by1: y + pb.y1 + 1}
		for py := pb.y0; py <= pb.y1; py++ {
			for px := pb.x0; px <= pb.x1; px++ {
				c := plate[py*plateW+px]
				if bg[(py-pb.y0)*bw+px-pb.x0] != c {
					e.addPt(x+px, y+py, c)
				}
			}
		}
		l.add(e)
	}
}

// ---- 台帳自備的 TRE 與外層 PAK 讀取器（規格 014 §4.2.2 表第 4 列；格式與門檻照 §3.6.2，程式碼與 §3.6 的預載分開） ----

const (
	ledgerPakName   = `..\..\DATA\GAMEFLOW\MAINMENU.PAK`
	ledgerPakLen    = 24603
	ledgerTreRecLen = 74
)

var (
	ledgerPakSHA    = ledgerHex("081e1d3f3028f90784040ccfbc881fa529b12eb5b28ab4a7755b56ed0aa2baba")
	ledgerSubSHA    = [2][32]byte{ledgerHex("3729dfc27900368bbe27e7fb1e7deaa2e16e5666476fbbf715b719139208d0af"), ledgerHex("abebe6c16aee0408fa27e93881f3a8ade463745c68ce3a1bdabcedef1210d380")}
	ledgerSubRange  = [2][2]int{{16, 15622}, {15622, 24215}}
	ledgerSubFirst  = [2]uint32{15606, 8593}
	ledgerTRENames  = []string{"BIGTRE.TRE", "TOBIGTRE.TRE"}
	ledgerPlateSize = [2]int{233, 104}
)

func ledgerHex(s string) [32]byte {
	var out [32]byte
	if _, err := hex.Decode(out[:], []byte(s)); err != nil {
		panic(err)
	}
	return out
}

// ledgerMenuFiles 是台帳讀到的 MAINMENU.PAK 子條目：按鈕檔（子 PAK）與底板條目 0 的解碼。
type ledgerMenuFiles struct {
	tre     string
	buttons []byte
	plate   []uint8
}

// ledgerReadMainMenu 依序試 root 下的 BIGTRE.TRE、TOBIGTRE.TRE，第一個通過名稱與雜湊門檻的就用；都不通過回各份的原因。
func ledgerReadMainMenu(root string) (*ledgerMenuFiles, error) {
	if root == "" {
		return nil, errors.New("台帳讀取器沒有根目錄")
	}
	var why []string
	for _, n := range ledgerTRENames {
		m, err := ledgerReadTRE(filepath.Join(root, n))
		if err == nil {
			m.tre = n
			return m, nil
		}
		why = append(why, n+"："+err.Error())
	}
	return nil, errors.New(strings.Join(why, "；"))
}

func ledgerReadTRE(path string) (*ledgerMenuFiles, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var hdr [8]byte
	if _, err := f.ReadAt(hdr[:], 0); err != nil {
		return nil, fmt.Errorf("讀目錄頭：%w", err)
	}
	n := binary.LittleEndian.Uint32(hdr[:]) // 第二個 u32 不使用（§3.6.2 第 2 條）
	if n == 0 || n > 1<<16 {
		return nil, fmt.Errorf("目錄筆數 %d 不合理", n)
	}
	dir := make([]byte, int(n)*ledgerTreRecLen)
	if _, err := f.ReadAt(dir, 8); err != nil {
		return nil, fmt.Errorf("讀目錄：%w", err)
	}
	hits := 0
	var off, ln uint32
	for k := 0; k < int(n); k++ {
		r := dir[k*ledgerTreRecLen : (k+1)*ledgerTreRecLen]
		name := r[1:66]
		if i := bytes.IndexByte(name, 0); i >= 0 {
			name = name[:i] // NUL 之後的殘留忽略
		}
		if strings.EqualFold(string(name), ledgerPakName) {
			hits++
			off, ln = binary.LittleEndian.Uint32(r[66:]), binary.LittleEndian.Uint32(r[70:])
		}
	}
	if hits != 1 {
		return nil, fmt.Errorf("名稱 %s 命中 %d 筆，要 1", ledgerPakName, hits)
	}
	if ln != ledgerPakLen {
		return nil, fmt.Errorf("外層長度 %d，要 %d", ln, ledgerPakLen)
	}
	pak := make([]byte, ln)
	if _, err := f.ReadAt(pak, int64(off)); err != nil {
		return nil, fmt.Errorf("讀外層：%w", err)
	}
	if sha256.Sum256(pak) != ledgerPakSHA {
		return nil, errors.New("外層 SHA-256 不符")
	}
	subs, err := ledgerOuterPak(pak)
	if err != nil {
		return nil, err
	}
	for i := range subs {
		if subs[i] != ledgerSubRange[i] {
			return nil, fmt.Errorf("子條目 %d 範圍 %v，要 %v", i, subs[i], ledgerSubRange[i])
		}
		b := pak[subs[i][0]:subs[i][1]]
		if binary.LittleEndian.Uint32(b) != ledgerSubFirst[i] {
			return nil, fmt.Errorf("子條目 %d 首 u32 %d，要 %d", i, binary.LittleEndian.Uint32(b), ledgerSubFirst[i])
		}
		if sha256.Sum256(b) != ledgerSubSHA[i] {
			return nil, fmt.Errorf("子條目 %d SHA-256 不符", i)
		}
	}
	buttons := append([]byte(nil), pak[subs[0][0]:subs[0][1]]...)
	plate, err := decodeEntry(pak[subs[1][0]:subs[1][1]], 0) // 子 PAK 是型別 0，交共用解碼器
	if err != nil {
		return nil, fmt.Errorf("底板解碼：%w", err)
	}
	if plate.W != ledgerPlateSize[0] || plate.H != ledgerPlateSize[1] {
		return nil, fmt.Errorf("底板 %d×%d，要 %d×%d", plate.W, plate.H, ledgerPlateSize[0], ledgerPlateSize[1])
	}
	return &ledgerMenuFiles{buttons: buttons, plate: plate.Pix}, nil
}

// ledgerOuterPak 解外層 PAK（§3.6.2 第 4 條）：u32 總長等於檔長；第一個位移的低 24 位 ÷ 4 − 1 為條目數；
// 每個位移低 24 位為起點、高 8 位為型別，只收 E0（未壓縮）；終點為下一個起點或總長。回前兩個條目的範圍。
func ledgerOuterPak(pak []byte) ([2][2]int, error) {
	var out [2][2]int
	if len(pak) < 8 {
		return out, errors.New("外層太短")
	}
	total := int(binary.LittleEndian.Uint32(pak))
	if total != len(pak) {
		return out, fmt.Errorf("外層總長 %d 不等於 %d", total, len(pak))
	}
	first := int(binary.LittleEndian.Uint32(pak[4:]) & 0xFFFFFF)
	n := first/4 - 1
	if first%4 != 0 || n < 2 || 4+4*n > len(pak) {
		return out, fmt.Errorf("外層第一個位移 %d 不合法", first)
	}
	starts := make([]int, n)
	for i := 0; i < n; i++ {
		raw := binary.LittleEndian.Uint32(pak[4+4*i:])
		if t := raw >> 24; t != 0xE0 {
			return out, fmt.Errorf("外層條目 %d 型別 %02X 不是 E0", i, t)
		}
		starts[i] = int(raw & 0xFFFFFF)
	}
	for i := 0; i < 2; i++ {
		end := total
		if i+1 < n {
			end = starts[i+1]
		}
		if starts[i] < 4+4*n || end < starts[i] || end > total {
			return out, fmt.Errorf("外層條目 %d 範圍 [%d, %d) 不合法", i, starts[i], end)
		}
		out[i] = [2]int{starts[i], end}
	}
	return out, nil
}

// ---- 收據接線與判定 ----

type expoKey struct {
	frames           int
	events           uint64
	lgen, fgen       uint64
	commits          int
	promotions       int
	ver, lsg, lfg    uint64
	cursorX, cursorY int
	moveX, moveY     int
	seq              uint64
	trail            [320]bool
}

// usePres 回判定是否用呈現層的呈現像（掛了呈現層且不是 N1）。
func (x *expoRun) usePres() bool { return x.pres != nil && x.mut&expoMutN1 == 0 }

// expoRun 是一次收據執行的曝光收據狀態。
type expoRun struct {
	mut      expoMut
	so       *SessionObserver
	lo       *LabelObserver
	flight   *FlightObserver
	scales   []int
	obsOvs   map[int]*Overlay
	obsFovs  map[int]*FlightOverlay
	presOvs  map[int]*Overlay
	presFovs map[int]*FlightOverlay
	led      *expoLedger
	lastMove **Point

	started    bool
	boot, next uint64
	frameSteps uint64

	events  uint64
	dirty   bool
	lastLG  uint64
	lastFG  uint64
	lastKey expoKey
	haveKey bool
	lastCap ExpoCapture
	lastExp []ExpoExposure

	res *ExpoResult

	dumpAt  []uint64
	dumpDir string

	// pres 非 nil 時（規格 014 分期 (c)）判定改用呈現層的呈現像（§4.2.1）：合成點判目前呈現像，另在每個呈現像產生時
	// 判一次（窮舉模式，phase −2）。N1 模式不用呈現層（照現行合成）。
	pres *Presenter
}

// attachPresenter 讓曝光收據改判呈現層的呈現像（§4.2.1 窮舉模式）。
func (x *expoRun) attachPresenter(p *Presenter) {
	x.pres = p
	x.res.Present = true
	if x.mut&expoMutN1 != 0 {
		return
	}
	// 呈現層在 CU 入口以候選集合取像（§3.3.1 第 3 條），早於升格。台帳自有的 FlightOverlay 原本在升格時才收集合，
	// CU 入口的呈現像會因台帳實例缺新字串而把照畫的中文誤判成露出；所以呈現層模式下台帳另在 CU 入口收候選
	// （觀察事件：cuAccept 剛轉入的 Candidates()，同位置取代），升格時照舊。基線（現行合成）不收，數字不變。
	if f := x.flight; f != nil {
		prev := f.onCU
		f.onCU = func(e cuEvent) {
			if e.Kind == cuEvAccepted {
				x.led.onFlightCommit(f.Candidates())
			}
			if prev != nil {
				prev(e)
			}
		}
	}
	p.OnCapture(func(info PresentInfo) { x.judge(info.CaptureSteps, expoPhaseCapture) })
}

// expoPhaseCapture 是窮舉模式（呈現像產生時）的判定在 ExpoCapture.Phase 的值。
const expoPhaseCapture = -2

// attachExpo 建呈現用（現行合成）與台帳的實例，串接既有回呼（不取代）。
func attachExpo(cfg ReceiptConfig, so *SessionObserver, obsOvs map[int]*Overlay, obsFovs map[int]*FlightOverlay, lastMove **Point) (*expoRun, error) {
	x := &expoRun{mut: cfg.expo.mut, so: so, lo: so.Labels, flight: so.Flight, scales: cfg.Scales, obsOvs: obsOvs, obsFovs: obsFovs,
		presOvs: map[int]*Overlay{}, presFovs: map[int]*FlightOverlay{}, lastMove: lastMove,
		res: &ExpoResult{Mut: cfg.expo.mut.String()}, dumpAt: append([]uint64(nil), cfg.expo.DumpAt...), dumpDir: cfg.expo.DumpDir}
	sort.Slice(x.dumpAt, func(i, j int) bool { return x.dumpAt[i] < x.dumpAt[j] })
	root := cfg.expo.Root
	if root == "" {
		root = cfg.SaveRoot
	}
	x.led = &expoLedger{ovs: map[int]*Overlay{}, fovs: map[int]*FlightOverlay{}, root: root}
	for _, sc := range cfg.Scales {
		for _, m := range []map[int]*Overlay{x.presOvs, x.led.ovs} {
			ov, err := NewOverlayText(cfg.Fonts, cfg.Catalog, cfg.GfxCatalog, sc)
			if err != nil {
				return nil, err
			}
			ov.exactOnly = cfg.exactOnly
			m[sc] = ov
		}
		if cfg.FlightCatalogs != nil {
			for _, m := range []map[int]*FlightOverlay{x.presFovs, x.led.fovs} {
				fov, err := NewFlightOverlayText(cfg.Fonts, *cfg.FlightCatalogs, sc)
				if err != nil {
					return nil, err
				}
				m[sc] = fov
			}
		}
	}
	lo := x.lo
	art, clear, commit := lo.OnArt, lo.OnClearArt, lo.OnCommit
	lo.OnArt = func(ev ArtEvent) {
		if art != nil {
			art(ev)
		}
		x.events++
		x.dirty = true
		for _, sc := range x.scales {
			x.presOvs[sc].AddDialogArt(ev)
		}
		x.led.onArt(ev)
	}
	lo.OnClearArt = func(steps uint64) {
		if clear != nil {
			clear(steps)
		}
		x.events++
		x.dirty = true
		for _, sc := range x.scales {
			x.presOvs[sc].ClearArt()
		}
	}
	lo.OnCommit = func(c Commit) {
		if commit != nil {
			commit(c)
		}
		x.events++
		x.dirty = true
		x.led.onCommit(c)
	}
	lo.OnPutsReturn = func(l Label) {
		x.events++
		x.led.onPutsReturn(l)
	}
	lo.OnShapeArt = func(steps uint64, kind ArtFile, file []byte, e, xx, yy int) {
		x.events++
		x.led.onShapeArt(steps, kind, file, e, xx, yy)
	}
	lo.OnCursor = func(_ uint64, xx, yy int) {
		x.led.cursorAt(xx, yy)
	}
	if f := x.flight; f != nil {
		fc := f.OnCommit
		f.OnCommit = func(c FlightCommit) {
			if fc != nil {
				fc(c)
			}
			x.events++
			x.dirty = true
			x.led.onFlightCommit(f.Visible())
		}
		f.OnAccepted = func(t FlightText) {
			x.events++
			x.led.onAccepted(t)
		}
	}
	prevFrame := so.OnFrame
	so.OnFrame = func(old, cur []byte, steps uint64) {
		if prevFrame != nil {
			prevFrame(old, cur, steps)
		}
		x.frameSteps = steps
	}
	if x.mut&expoMutN1 != 0 {
		so.afterStep = x.afterStep
	}
	return x, nil
}

// nextPoint 回 (steps, target] 內的下一個合成點。第一次呼叫時的 steps 是開機步數。N1 模式沒有固定合成點。
func (x *expoRun) nextPoint(steps, target uint64) (uint64, bool) {
	if x.mut&expoMutN1 != 0 {
		return 0, false
	}
	if !x.started {
		x.started, x.boot, x.next = true, steps, steps+expoInterval
	}
	for x.next <= steps {
		x.next += expoInterval
	}
	if x.next > target {
		return 0, false
	}
	return x.next, true
}

func (x *expoRun) checkpoint(c uint64) {
	phase := int(((c - x.boot) / expoInterval) % expoPhases)
	x.next = c + expoInterval
	x.judge(c, phase)
}

// afterStep 是 N1：每個狀態改變事件（或可見集合世代改變）之後的那一步出圖判定。
func (x *expoRun) afterStep(steps uint64) {
	lg := x.lo.Generation()
	var fg uint64
	if x.flight != nil {
		fg = x.flight.Generation()
	}
	if !x.dirty && lg == x.lastLG && fg == x.lastFG {
		return
	}
	x.dirty, x.lastLG, x.lastFG = false, lg, fg
	x.judge(steps, -1)
}

func (x *expoRun) key() expoKey {
	k := expoKey{frames: x.so.Frames, events: x.events, lgen: x.lo.Generation(), commits: x.lo.Commits(),
		ver: x.led.ver, lsg: x.led.labelGen, lfg: x.led.flightGen, cursorX: -1, cursorY: -1, moveX: -1, moveY: -1, trail: x.led.trail}
	if x.usePres() {
		k.seq = x.pres.Current().Seq
	}
	if x.flight != nil {
		k.fgen, k.promotions = x.flight.Generation(), x.flight.Counts().Promotions
	}
	if c := x.led.cursor; c != nil {
		k.cursorX, k.cursorY = c.X, c.Y
	}
	if m := *x.lastMove; m != nil {
		k.moveX, k.moveY = m.X, m.Y
	}
	return k
}

// judge 在一個合成點以現行合成出 2、3 倍圖並逐條目判定（§4.2.2 判定、分類）。
func (x *expoRun) judge(steps uint64, phase int) {
	cp := ExpoCapture{Phase: phase, Steps: steps, Frames: x.so.Frames, Exposed: map[int]int{}, FrameSteps: x.frameSteps}
	idx, pal := x.so.LastIndexed, x.so.LastPalette
	var img *presentImage
	if x.usePres() {
		img = x.pres.cur
		idx = nil
		if img != nil {
			idx, pal = img.indexed, img.palette
			cp.FrameSteps, cp.Seq, cp.Kind = img.info.CaptureSteps, img.info.Seq, img.info.Kind
		}
	}
	if idx == nil {
		cp.NoFrame = true
		x.res.Captures = append(x.res.Captures, cp)
		return
	}
	k := x.key()
	if x.haveKey && k == x.lastKey {
		c := x.lastCap
		c.Phase, c.Steps, c.Cached = phase, steps, true
		c.Exposed = map[int]int{}
		for sc, n := range x.lastCap.Exposed {
			c.Exposed[sc] = n
		}
		x.res.Captures = append(x.res.Captures, c)
		for _, e := range x.lastExp {
			e.Phase, e.Steps = phase, steps
			x.res.Exposures = append(x.res.Exposures, e)
		}
		return
	}
	x.lastKey, x.haveKey = k, true
	vis := map[string]bool{}
	for _, l := range x.lo.Visible() {
		vis[l.Key] = true
	}
	var on []*ledgerEntry
	for _, e := range x.led.entries {
		if x.mut&expoMutLedgerFromVisible != 0 && (e.kind == ledLabel || e.kind == ledBoard) && !vis[e.key] {
			continue
		}
		if onScreen(e, idx) {
			on = append(on, e)
		}
	}
	// 退場清單：在畫面上的照常判定；不在畫面上的移出（被同位置新條目取代、且已離開畫面）。
	keep := x.led.retiring[:0]
	for _, e := range x.led.retiring {
		if !onScreen(e, idx) {
			x.led.recs[e.id].Retired = steps
			x.led.ver++
			continue
		}
		keep = append(keep, e)
		if x.mut&expoMutLedgerFromVisible != 0 && (e.kind == ledLabel || e.kind == ledBoard) && !vis[e.key] {
			continue
		}
		on = append(on, e)
	}
	for i := len(keep); i < len(x.led.retiring); i++ {
		x.led.retiring[i] = nil
	}
	x.led.retiring = keep
	cp.OnScreen = len(on)
	var exps []ExpoExposure
	if len(on) != 0 {
		var cur *Point
		if x.led.cursor != nil {
			cur = x.led.cursor
		} else if m := *x.lastMove; m != nil {
			cur = m
		}
		for _, sc := range x.scales {
			var comp, plain []byte
			if img != nil {
				// 呈現層：以呈現像凍結的集合合成（不畫字幕：字幕只在全黑的字幕帶，不影響墨跡判定）。
				comp, plain = x.pres.composeImage(img, sc, false), ScaleIndexedRGBA(idx, pal, sc)
			} else {
				comp, plain = x.presentCompose(sc, idx, pal)
			}
			lc, cover := x.ledgerCompose(sc, idx, pal)
			if sc == 3 && len(x.dumpAt) != 0 && steps >= x.dumpAt[0] && x.dumpDir != "" {
				x.dumpAt = x.dumpAt[1:]
				for n, img := range map[string][]byte{"comp": comp, "ledger": lc, "plain": plain} {
					p := filepath.Join(x.dumpDir, fmt.Sprintf("dump-%d-%s.png", steps, n))
					if err := os.MkdirAll(x.dumpDir, 0o755); err == nil {
						if err := WriteRGBAPNG(p, img, 3); err != nil {
							x.res.Errors = append(x.res.Errors, err.Error())
						}
					}
				}
			}
			miss := x.missKeys(sc)
			for _, e := range on {
				if e.hud && sc != 3 {
					continue // HUD 中文只在 3 倍（AGENTS §2 2026-10-20）
				}
				ex := judgeEntryTrail(e, idx, &pal, sc, comp, plain, lc, cur, cover, &x.led.trail)
				if ex.Exposed == 0 && ex.ExposedLit == 0 {
					continue
				}
				ex.Phase, ex.Steps, ex.Scale = phase, steps, sc
				ex.CurX, ex.CurY = -1, -1
				if cur != nil {
					ex.CurX, ex.CurY = cur.X, cur.Y
				}
				ex.Gone = (e.kind == ledLabel || e.kind == ledBoard) && !vis[e.key]
				ex.Class = expoClass(ex, e.group2 || miss[e.key], img != nil && img.info.Kind == PresFallback)
				exps = append(exps, ex)
				if ex.Exposed > 0 {
					cp.Exposed[sc]++
				}
			}
		}
	}
	x.res.Captures = append(x.res.Captures, cp)
	x.res.Exposures = append(x.res.Exposures, exps...)
	x.lastCap, x.lastExp = cp, exps
}

// expoClass 是 §4.2.2 的分類（第 10 版，依序）：色盤不可見 → 游標 → 游標殘跡 → 未覆繪 → 退路 → 其他；
// 只有字面規則判露出的記「僅字面」。
//
// 「色盤不可見」仍以露出點的八鄰點判（Contrast == 0）。§4.2.2「待 (c) 動工前定案的改進規則」（看墨跡點與標籤框外
// 背景的對比）只寫了方向，沒有寫背景取哪些像素與門檻，分期 (c) 未實作，照規格「定案前照實列出」。
func expoClass(ex ExpoExposure, unpainted, fallback bool) string {
	switch {
	case ex.Exposed == 0:
		return "僅字面"
	case ex.Contrast == 0:
		return "色盤不可見"
	case ex.Cursor == ex.Exposed:
		return "游標"
	case ex.Trail > 0 && ex.Cursor+ex.Trail == ex.Exposed:
		return "游標殘跡"
	case unpainted:
		return "未覆繪"
	case fallback:
		return "退路"
	}
	return "其他"
}

// onScreen 是判定第 1 條：墨跡點中呈現像色號等於記錄色的點數 ≥ max(4, 墨跡點數的一半)。
func onScreen(e *ledgerEntry, idx []byte) bool {
	n := 0
	for i, p := range e.pts {
		if idx[p] == e.cols[i] {
			n++
		}
	}
	return n >= 4 && 2*n >= len(e.pts)
}

// judgeEntry 是判定第 2、3 條：呈現像上仍是記錄色的墨跡點 p，對應區塊（文字 s×s；圖面為 p 所在欄在字帶內的 s 欄寬整條）
// 合成圖與原版放大圖至少一點不同、且等於台帳合成圖（台帳自有實例的 stamp 像素），算照畫；否則露出（ExposedLit）。
// cover（nil 或長度 64000）是台帳自有實例照畫的範圍（1 倍像素，ledgerCover）：p 落在其中時「與原版不同」改為成立
// （文字：同色筆畫整塊蓋住英文墨跡；圖面：該欄 stamp 的無字底與原版同色，墨跡點本來就與周圍同色），其餘照字面（Exposed）。
func judgeEntry(e *ledgerEntry, idx []byte, pal *[256][3]uint8, sc int, comp, plain, lc []byte, cur *Point, cover []bool) ExpoExposure {
	return judgeEntryTrail(e, idx, pal, sc, comp, plain, lc, cur, cover, nil)
}

// judgeEntryTrail 同 judgeEntry；trail（可為 nil）是游標殘跡欄：圖面條目的露出點不在游標範圍、而所在欄屬於它時記入 Trail。
func judgeEntryTrail(e *ledgerEntry, idx []byte, pal *[256][3]uint8, sc int, comp, plain, lc []byte, cur *Point, cover []bool, trail *[320]bool) ExpoExposure {
	ex := ExpoExposure{Entry: e.id, Kind: e.kind, Key: e.key, EntrySteps: e.steps, Ink: len(e.pts)}
	W := 320 * sc
	type cv struct{ same, differ bool }
	colDone := map[int]cv{}
	for i, p := range e.pts {
		if idx[p] != e.cols[i] {
			continue
		}
		ex.Visible++
		x, y := int(p%320), int(p/320)
		var r cv
		if e.art {
			var ok bool
			if r, ok = colDone[x]; !ok {
				r.same, r.differ = blockCompare(comp, plain, lc, W, x*sc, e.by0*sc, sc, (e.by1-e.by0)*sc)
				colDone[x] = r
			}
		} else {
			r.same, r.differ = blockCompare(comp, plain, lc, W, x*sc, y*sc, sc, sc)
		}
		if !(r.same && r.differ) {
			ex.ExposedLit++
		}
		if r.same && (r.differ || (cover != nil && cover[p])) {
			continue
		}
		if ex.Exposed == 0 {
			ex.ExX0, ex.ExY0, ex.ExX1, ex.ExY1 = x, y, x, y
		}
		ex.ExX0, ex.ExY0, ex.ExX1, ex.ExY1 = min(ex.ExX0, x), min(ex.ExY0, y), max(ex.ExX1, x), max(ex.ExY1, y)
		ex.Exposed++
		if pal != nil && contrast(idx, pal, x, y) {
			ex.Contrast++
		}
		inCur := false
		if cur != nil && x >= cur.X-5 && x < cur.X+6 {
			if (e.art && cur.Y-5 < e.by1 && e.by0 < cur.Y+6) || (!e.art && y >= cur.Y-5 && y < cur.Y+6) {
				ex.Cursor++
				inCur = true
			}
		}
		if !inCur && e.art && trail != nil && trail[x] {
			ex.Trail++
		}
	}
	return ex
}

// contrast 回 (x, y) 在呈現像色盤下與八鄰點中至少一點 RGB 不同（色盤淡出成全黑等情形下墨跡看不見）。
func contrast(idx []byte, pal *[256][3]uint8, x, y int) bool {
	c := pal[idx[y*320+x]]
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := x+dx, y+dy
			if (dx != 0 || dy != 0) && nx >= 0 && nx < 320 && ny >= 0 && ny < 200 && pal[idx[ny*320+nx]] != c {
				return true
			}
		}
	}
	return false
}

// blockCompare 回區塊 [ox, ox+w) × [oy, oy+h)（輸出像素）合成圖是否等於台帳合成圖（same）、是否與原版放大圖至少一點不同（differ）。
func blockCompare(comp, plain, lc []byte, W, ox, oy, w, h int) (same, differ bool) {
	for y := max(oy, 0); y < oy+h; y++ {
		i := 4 * (y*W + ox)
		j := i + 4*w
		if j > len(comp) {
			return false, false
		}
		if !bytes.Equal(comp[i:j], lc[i:j]) {
			return false, differ
		}
		if !differ && !bytes.Equal(comp[i:j], plain[i:j]) {
			differ = true
		}
	}
	return true, differ
}

// ledgerCover 回台帳（或 N29 時觀察用）實例最近一次 Draw 照畫的範圍（1 倍像素，長度 64000）：標籤、計分板的
// BGReport 與飛行字串的 Report（Hidden 的不算），以及圖面各列在這張畫格逐欄比對原版基準相符（照畫）的欄
// （同 xlate frameArt 的規則，以該實例自己的規劃與基準算）。
func ledgerCover(ov *Overlay, fov *FlightOverlay, idx []byte) []bool {
	m := make([]bool, 64000)
	if ov != nil {
		for _, e := range ov.arts {
			r := e.rect
			if len(e.ref) != r.W*r.H || r.X < 0 || r.Y < 0 || r.X+r.W > 320 || r.Y+r.H > 200 {
				continue
			}
		col:
			for cx := 0; cx < r.W; cx++ {
				for cy := 0; cy < r.H; cy++ {
					if idx[(r.Y+cy)*320+r.X+cx] != e.ref[cy*r.W+cx] {
						continue col
					}
				}
				for cy := 0; cy < r.H; cy++ {
					m[(r.Y+cy)*320+r.X+cx] = true
				}
			}
		}
	}
	set := func(x0, y0, x1, y1 int) {
		for y := max(y0, 0); y < min(y1, 200); y++ {
			for x := max(x0, 0); x < min(x1, 320); x++ {
				m[y*320+x] = true
			}
		}
	}
	if ov != nil {
		for _, b := range ov.BGReport() {
			if !b.Hidden {
				set(b.CoverX, b.CoverY, b.CoverX+b.CoverW, b.CoverY+b.CoverH)
			}
		}
	}
	if fov != nil {
		for _, f := range fov.Report() {
			if !f.Hidden {
				set(f.X0, f.Y0, f.X1, f.Y1)
			}
		}
	}
	return m
}

// presentCompose 是基線的呈現：現行合成（畫格取最後一次回掃，覆繪狀態取當下），以呈現用的自有實例。
func (x *expoRun) presentCompose(sc int, idx []byte, pal [256][3]uint8) (comp, plain []byte) {
	in := scaleIn{Overlay: x.presOvs[sc], LabelsV: x.lo.Visible(), LabelsG: x.lo.Generation(), Scale: sc, Frame: idx, Palette: pal}
	if fov := x.presFovs[sc]; fov != nil && x.flight != nil {
		in.Flight, in.FlightV, in.FlightG = fov, x.flight.Visible(), x.flight.Generation()
	}
	o := composeScale(in)
	return o.Comp, o.Plain
}

// ledgerCompose 是台帳的 stamp 像素：台帳自有實例在同一張呈現像上重畫（§4.2.2 判定第 2 條）。
// expoMutObsStamps（N29）改用觀察用實例與觀察器的即時集合。
func (x *expoRun) ledgerCompose(sc int, idx []byte, pal [256][3]uint8) ([]byte, []bool) {
	if x.mut&expoMutObsStamps != 0 {
		in := scaleIn{Overlay: x.obsOvs[sc], LabelsV: x.lo.Visible(), LabelsG: x.lo.Generation(), Scale: sc, Frame: idx, Palette: pal}
		if fov := x.obsFovs[sc]; fov != nil && x.flight != nil {
			in.Flight, in.FlightV, in.FlightG = fov, x.flight.Visible(), x.flight.Generation()
		}
		return composeScale(in).Comp, ledgerCover(in.Overlay, in.Flight, idx)
	}
	in := scaleIn{Overlay: x.led.ovs[sc], LabelsV: x.led.labels, LabelsG: x.led.labelGen, Scale: sc, Frame: idx, Palette: pal}
	if fov := x.led.fovs[sc]; fov != nil {
		in.Flight, in.FlightV, in.FlightG = fov, x.led.flights, x.led.flightGen
	}
	return composeScale(in).Comp, ledgerCover(in.Overlay, in.Flight, idx)
}

// missKeys 回觀察用實例在倍率 sc 的 Miss、Skip 的 Key（分類「未覆繪」）。
func (x *expoRun) missKeys(sc int) map[string]bool {
	m := map[string]bool{}
	if ov := x.obsOvs[sc]; ov != nil {
		for _, v := range ov.Misses {
			m[v.Key] = true
		}
		for _, v := range ov.Skips {
			m[v.Key] = true
		}
	}
	if fov := x.obsFovs[sc]; fov != nil {
		for _, v := range fov.Misses {
			m[v.Key] = true
		}
		for _, v := range fov.Skips {
			m[v.Key] = true
		}
	}
	return m
}

func (x *expoRun) finish(d session.StateDigest) {
	r := x.res
	r.Digest = d
	r.Ledger = x.led.recs
	r.PlateEvents = x.led.plates
	r.ReaderTRE, r.ReaderErr = x.led.readerTRE, x.led.readerErr
	if r.PlateEvents > 0 && x.led.menu == nil && r.ReaderErr == "" {
		r.ReaderErr = "有底板入口但讀取器沒有結果"
	}
}

// ---- 輸出（§4.2.5） ----

// ExpoSummary 是一個相位（−1 為 N1）、一個倍率、一個分類的露出數。
type ExpoSummary struct {
	Phase, Scale int
	Class        string
	Exposures    int // 露出筆數（合成點 × 條目）
	Points       int // 有露出的合成點數（同一合成點多筆只算一次）
}

// Summary 依相位、倍率、分類彙整（快取沿用的合成點照算）。
func (r *ExpoResult) Summary() []ExpoSummary {
	type k struct {
		ph, sc int
		cl     string
	}
	n := map[k]int{}
	pts := map[k]map[uint64]bool{}
	for _, e := range r.Exposures {
		kk := k{e.Phase, e.Scale, e.Class}
		n[kk]++
		if pts[kk] == nil {
			pts[kk] = map[uint64]bool{}
		}
		pts[kk][e.Steps] = true
	}
	var out []ExpoSummary
	for kk, v := range n {
		out = append(out, ExpoSummary{kk.ph, kk.sc, kk.cl, v, len(pts[kk])})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Phase != b.Phase {
			return a.Phase < b.Phase
		}
		if a.Scale != b.Scale {
			return a.Scale < b.Scale
		}
		return a.Class < b.Class
	})
	return out
}

// WriteExpoFiles 寫 §4.2.5 的檔到 dir（新目錄）：exposure.tsv、captures.tsv、ledger.tsv、counts.txt、digest.txt；
// windows.tsv、unattributed.tsv、stale.tsv 在基線（p 為 nil）不適用，只寫說明列；p 非 nil 時由呈現層寫，另加
// captures-present.tsv、cu.tsv、present-counts.txt。
func WriteExpoFiles(dir string, r *ExpoResult, p *Presenter) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b bytes.Buffer
	b.WriteString("phase\tsteps\tscale\tentry\tkind\tkey\tentry_steps\tclass\tink\tvisible\texposed\tcursor\tgone\texposed_lit\tcontrast\tex_x0\tex_y0\tex_x1\tex_y1\tcur_x\tcur_y\ttrail\n")
	for _, e := range r.Exposures {
		fmt.Fprintf(&b, "%d\t%d\t%d\t%d\t%s\t%s\t%d\t%s\t%d\t%d\t%d\t%d\t%t\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n", e.Phase, e.Steps, e.Scale, e.Entry, e.Kind, tsvSafe(e.Key), e.EntrySteps, e.Class, e.Ink, e.Visible, e.Exposed, e.Cursor, e.Gone, e.ExposedLit,
			e.Contrast, e.ExX0, e.ExY0, e.ExX1, e.ExY1, e.CurX, e.CurY, e.Trail)
	}
	if err := os.WriteFile(filepath.Join(dir, "exposure.tsv"), b.Bytes(), 0o644); err != nil {
		return err
	}
	b.Reset()
	b.WriteString("phase\tsteps\tframes\tframe_steps\ton_screen\texposed_2x\texposed_3x\tcached\tno_frame\tseq\tkind\n")
	for _, c := range r.Captures {
		fmt.Fprintf(&b, "%d\t%d\t%d\t%d\t%d\t%d\t%d\t%t\t%t\t%d\t%s\n", c.Phase, c.Steps, c.Frames, c.FrameSteps, c.OnScreen, c.Exposed[2], c.Exposed[3], c.Cached, c.NoFrame, c.Seq, c.Kind)
	}
	if err := os.WriteFile(filepath.Join(dir, "captures.tsv"), b.Bytes(), 0o644); err != nil {
		return err
	}
	b.Reset()
	b.WriteString("id\tkind\tkey\tsteps\tx0\ty0\tx1\ty1\tpoints\tgroup2\treplaced\tretired\n")
	for _, e := range r.Ledger {
		fmt.Fprintf(&b, "%d\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%t\t%d\t%d\n", e.ID, e.Kind, tsvSafe(e.Key), e.Steps, e.X0, e.Y0, e.X1, e.Y1, e.Points, e.Group2, e.Replaced, e.Retired)
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.tsv"), b.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "counts.txt"), r.CountsText(), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "digest.txt"), DigestText(r.Digest), 0o644); err != nil {
		return err
	}
	if p != nil {
		return p.WriteFiles(dir, func(name string, b []byte) error { return os.WriteFile(filepath.Join(dir, name), b, 0o644) })
	}
	na := "# 基線（現行合成）不適用：窗、未歸屬寫入、陳舊斷言在呈現層（分期 (c)、(f)）才有\n"
	for _, n := range []string{"windows.tsv", "unattributed.tsv", "stale.tsv"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(na), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func tsvSafe(s string) string { return strings.NewReplacer("\t", " ", "\n", " ").Replace(s) }

// CountsText 是 counts.txt 的內容。
func (r *ExpoResult) CountsText() []byte {
	var b bytes.Buffer
	pres := "基線（現行合成）"
	if r.Present {
		pres = "呈現層（分期 (c)；N1 時仍為現行合成）"
	}
	fmt.Fprintf(&b, "呈現：%s；突變：%s\n", pres, r.Mut)
	fmt.Fprintf(&b, "合成點間隔 %d 步、%d 相位（φ = 0、48125、96250、144375；phase −1 為 N1 事件合成點，−2 為呈現像產生時的窮舉判定）\n", expoInterval, expoPhases)
	byPh := map[int][3]int{} // 合成點數、有條目在畫面上、沿用
	for _, c := range r.Captures {
		v := byPh[c.Phase]
		v[0]++
		if c.OnScreen > 0 {
			v[1]++
		}
		if c.Cached {
			v[2]++
		}
		byPh[c.Phase] = v
	}
	var phs []int
	for p := range byPh {
		phs = append(phs, p)
	}
	sort.Ints(phs)
	for _, p := range phs {
		v := byPh[p]
		fmt.Fprintf(&b, "phase %d：合成點 %d、有條目在畫面上 %d、沿用 %d\n", p, v[0], v[1], v[2])
	}
	b.WriteString("露出（phase、倍率、分類：筆數／合成點數）：\n")
	for _, s := range r.Summary() {
		fmt.Fprintf(&b, "  phase %d %d 倍 %s：%d／%d\n", s.Phase, s.Scale, s.Class, s.Exposures, s.Points)
	}
	kinds := map[string]int{}
	for _, e := range r.Exposures {
		kinds[fmt.Sprintf("%d 倍 %s %s", e.Scale, e.Class, e.Kind)]++
	}
	var ks []string
	for k := range kinds {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	b.WriteString("露出依種類（倍率、分類、種類：筆數，四相位合計）：\n")
	for _, k := range ks {
		fmt.Fprintf(&b, "  %s：%d\n", k, kinds[k])
	}
	lit := map[[2]int][2]int{} // (phase, 倍率) → 字面規則的露出筆數、合成點數
	litPts := map[[2]int]map[uint64]bool{}
	for _, e := range r.Exposures {
		if e.ExposedLit == 0 {
			continue
		}
		k := [2]int{e.Phase, e.Scale}
		v := lit[k]
		v[0]++
		lit[k] = v
		if litPts[k] == nil {
			litPts[k] = map[uint64]bool{}
		}
		litPts[k][e.Steps] = true
	}
	var lks [][2]int
	for k := range lit {
		lks = append(lks, k)
	}
	sort.Slice(lks, func(i, j int) bool { return lks[i][0] < lks[j][0] || (lks[i][0] == lks[j][0] && lks[i][1] < lks[j][1]) })
	b.WriteString("字面規則（區塊內至少一點與原版不同；不分類）的露出（phase、倍率：筆數／合成點數）：\n")
	for _, k := range lks {
		fmt.Fprintf(&b, "  phase %d %d 倍：%d／%d\n", k[0], k[1], lit[k][0], len(litPts[k]))
	}
	gone := 0
	for _, e := range r.Exposures {
		if e.Gone && e.Exposed > 0 {
			gone++
		}
	}
	fmt.Fprintf(&b, "標籤消失的露出（標籤、計分板條目的 Key 不在可見集合；N8 的量測）：%d\n", gone)
	fmt.Fprintf(&b, "台帳條目 %d；主選單底板入口 %d；讀取器 TRE %q；讀取器錯誤 %q\n", len(r.Ledger), r.PlateEvents, r.ReaderTRE, r.ReaderErr)
	if r.Present {
		b.WriteString("呈現層的陳舊斷言與異常計數見 present-counts.txt\n")
	} else {
		b.WriteString("不適用（基線）：退路、陳舊、RecaptureMismatch、QueueOverflow、WindowTimeout、ModeTimeout、DuplicateEntry、WindowSkipped、CUResidue、StaleAfterCUVoid（呈現層分期 (c) 以後才有）\n")
	}
	for _, e := range r.Errors {
		fmt.Fprintf(&b, "錯誤：%s\n", e)
	}
	return b.Bytes()
}
