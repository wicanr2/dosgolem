package strikecommander

// cuTracker 是 STRIKE 的 CU 狀態機（strike_commander 規格 009 §3.6.2、§3.6.3；抽出依規格 012 §3.2.2）。
// FlightObserver 與 HD 錄製器各持一個實例。追蹤器只管「CU 進行中」「CU 後待升格」與入口的步數、SS:SP、IP；
// 計數、日誌、待提交與候選都由持有者在回呼內處理。只讀 View。

// cuEntry 是進行中入口的狀態（Entry() 回傳，供日誌與配對）。
type cuEntry struct {
	Steps      uint64
	SS, SP, IP uint16
	InProgress bool
	Awaiting   bool
}

// cuReturnOutcome 是 CU 返回的處理結果。
type cuReturnOutcome int

const (
	cuReturnIgnored  cuReturnOutcome = iota // 不在進行中
	cuReturnMismatch                        // IP 或 SS:SP 與入口不配對
	cuReturnAccepted                        // 接受，之後待升格
)

// noCUHooks 是 h 為 nil 時用的空回呼（只讀，不寫）。
var noCUHooks cuHooks

// cuHooks 是追蹤器的回呼；各欄可為 nil，整個 h 也可為 nil。
type cuHooks struct {
	OnEntryHit      func(v FlightView, ip uint16) // sigJemm 相符的入口命中（計 HitCUDiff／HitCUPlain）
	OnCHNonZero     func(v FlightView, ip uint16)
	OnTimeout       func(steps uint64, old cuEntry)
	OnDuplicate     func(v FlightView, ip uint16, cur cuEntry)
	OnVoided        func(v FlightView, ip uint16, old cuEntry)
	OnEntryAccepted func(v FlightView, steps uint64, ip uint16) // 在追蹤器改狀態之前呼叫
	OnReturn        func(v FlightView, ip uint16, entry cuEntry, outcome cuReturnOutcome)
	OnMU            func(v FlightView, cx uint16)
}

type cuTracker struct {
	cuIn, cuAwait bool
	cuSteps       uint64
	cuSS, cuSP    uint16
	cuIP          uint16
	base          uint16
	baseOK        bool
}

// SetBase 設定 STRIKE 主映像載入段與是否啟用；未啟用時 Observe 不處理任何命中。
func (t *cuTracker) SetBase(base uint16, ok bool) { t.base, t.baseOK = base, ok }

// Reset 清除全部 CU 狀態（不動 base）。
func (t *cuTracker) Reset() {
	t.cuIn, t.cuAwait, t.cuSteps, t.cuSS, t.cuSP, t.cuIP = false, false, 0, 0, 0, 0
}

// Entry 回目前的入口狀態。
func (t *cuTracker) Entry() cuEntry {
	return cuEntry{Steps: t.cuSteps, SS: t.cuSS, SP: t.cuSP, IP: t.cuIP, InProgress: t.cuIn, Awaiting: t.cuAwait}
}

// Observe 在 IP 為 00A5、00D6（入口）、00A7、00D8（返回）、00EE（MU）時處理，其餘 IP 不讀任何東西。
func (t *cuTracker) Observe(v FlightView, h *cuHooks) {
	if h == nil {
		h = &noCUHooks
	}
	switch v.IP() {
	case ipCUDiff, ipCUPlain:
		if t.baseOK {
			t.entry(v, h)
		}
	case ipCUDiffRet, ipCUPlainRet:
		if t.baseOK {
			t.ret(v, h)
		}
	case ipMU:
		if t.baseOK && t.jemmOK(v) && h.OnMU != nil {
			h.OnMU(v, v.CX())
		}
	}
}

// jemmOK 比 +1D3A:0090 起 98 bytes（每次都比，不快取，009 §3.6.2）。
func (t *cuTracker) jemmOK(v View) bool {
	return v.CS() == t.base+segFJemm && match(v, t.base+segFJemm, 0x0090, sigJemm, 0)
}

// checkTimeout 是 009 §3.6.2 的逾時：「CU 進行中」持續 100 萬步沒有返回就清為假。
func (t *cuTracker) checkTimeout(steps uint64, h *cuHooks) {
	if t.cuIn && steps-t.cuSteps >= cuTimeoutSteps {
		old := t.Entry()
		t.cuIn = false
		if h.OnTimeout != nil {
			h.OnTimeout(steps, old)
		}
	}
}

func (t *cuTracker) entry(v FlightView, h *cuHooks) {
	if !t.jemmOK(v) {
		return
	}
	ip := v.IP()
	if h.OnEntryHit != nil {
		h.OnEntryHit(v, ip)
	}
	if v.CX()>>8 != 0 {
		// 目的不是 VGA：不動狀態，也不檢查逾時（009 §3.6.2）。
		if h.OnCHNonZero != nil {
			h.OnCHNonZero(v, ip)
		}
		return
	}
	steps := v.Steps()
	ss, sp := v.SS(), v.SP()
	t.checkTimeout(steps, h)
	if t.cuIn {
		if ss == t.cuSS && sp == t.cuSP && steps-t.cuSteps < cuDupSteps {
			if h.OnDuplicate != nil {
				h.OnDuplicate(v, ip, t.Entry())
			}
			return
		}
		old := t.Entry()
		t.cuIn = false
		if h.OnVoided != nil {
			h.OnVoided(v, ip, old)
		}
	}
	if h.OnEntryAccepted != nil {
		h.OnEntryAccepted(v, steps, ip)
	}
	t.cuAwait = false
	t.cuIn, t.cuSteps, t.cuSS, t.cuSP, t.cuIP = true, steps, ss, sp, ip
}

func (t *cuTracker) ret(v FlightView, h *cuHooks) {
	if v.CS() != t.base+segFJemm {
		return
	}
	ip := v.IP()
	t.checkTimeout(v.Steps(), h)
	e := t.Entry()
	if !t.cuIn {
		if h.OnReturn != nil {
			h.OnReturn(v, ip, e, cuReturnIgnored)
		}
		return
	}
	want := uint16(ipCUDiffRet)
	if t.cuIP == ipCUPlain {
		want = ipCUPlainRet
	}
	if ip != want || v.SS() != t.cuSS || v.SP() != t.cuSP {
		if h.OnReturn != nil {
			h.OnReturn(v, ip, e, cuReturnMismatch)
		}
		return
	}
	t.cuIn, t.cuAwait = false, true
	if h.OnReturn != nil {
		h.OnReturn(v, ip, e, cuReturnAccepted)
	}
}

// Frame 在每次回掃呼叫：先檢查逾時；不在進行中且待升格時回 promote = true、key = 入口步數，待升格清為假。
func (t *cuTracker) Frame(steps uint64, h *cuHooks) (promote bool, key uint64) {
	if h == nil {
		h = &noCUHooks
	}
	t.checkTimeout(steps, h)
	if t.cuIn || !t.cuAwait {
		return false, 0
	}
	t.cuAwait = false
	return true, t.cuSteps
}
