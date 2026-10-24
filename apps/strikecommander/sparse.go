package strikecommander

import "github.com/wicanr2/dosgolem/session"

// 規格 460（dosgolem-sc docs/spec/460-session-sparse-observe）：SessionObserver 實作 session.SparseObserver。
// session 只在觸發 IP、步數期限與單次喚醒的步驟呼叫 BeforeStep；觀察器的判定規則不變，只改呼叫時機與步數來源。

// 各觀察器 Observe 內 switch v.IP() 的 case（§2.1）。O2 的測試以 go/types 雙向比對這幾張表與原始碼的 case。
var (
	labelTriggerIPs    = [...]uint16{ipPutsEntry, ipPutsRet, ipPutc, ipBlitEntry, ipBlitRet, ipMenuShape}
	flightTriggerIPs   = [...]uint16{ipFPuts, ipFPutc, ipFPutsRet, ipPlaque, ipPlaqueRet, ipCUDiff, ipCUPlain, ipCUDiffRet, ipCUPlainRet, ipMU}
	subtitleTriggerIPs = [...]uint16{ipConvEntry, ipConvDispatch, ipConvFast}
)

// sparseDefault 是陸空戰將 SessionConfig 的稀疏觀察預設（460 §3.5）。§5 全部通過之前為假（§7 第 5 步才翻）。
const sparseDefault = false

// sparseOverride 由測試依環境變數 DOSGOLEM_SC_SPARSE 設定（sparse_env_test.go）：0 未設、1 開、-1 關。非測試碼不讀環境變數。
var sparseOverride int8

// sparseOn 回 SessionConfig 的 SparseObserve：測試覆寫優先，其次 want（ReceiptConfig.sparse），最後 sparseDefault。
func sparseOn(want bool) bool {
	switch sparseOverride {
	case 1:
		return true
	case -1:
		return false
	}
	return want || sparseDefault
}

var (
	_ session.SparseObserver = (*SessionObserver)(nil)
	_ session.SparseObserver = presentObserver{}
)

// TriggerIPs 回已掛觀察器的觸發 IP 聯集（460 §3.4）：Labels 恆掛；Flight、Subtitles 非 nil 才併入；
// 呈現層非 nil 時併入它以實際窗表建的 ipMask。每次呼叫現算（session 每回合讀一次）。
func (s *SessionObserver) TriggerIPs() session.IPSet {
	var set session.IPSet
	for _, ip := range labelTriggerIPs {
		set.Add(ip)
	}
	if s.Flight != nil {
		for _, ip := range flightTriggerIPs {
			set.Add(ip)
		}
	}
	if s.Subtitles != nil {
		for _, ip := range subtitleTriggerIPs {
			set.Add(ip)
		}
	}
	if s.present != nil {
		m := session.IPSet(s.present.ipMask)
		set.Union(&m)
	}
	return set
}

// NextWake 回各觀察器 nextWake() 的最小值（460 §3.4）。
func (s *SessionObserver) NextWake() uint64 {
	w := s.Labels.nextWake()
	if s.Flight != nil {
		w = min(w, s.Flight.nextWake())
	}
	if s.Subtitles != nil {
		w = min(w, s.Subtitles.nextWake())
	}
	if s.present != nil {
		w = min(w, s.present.nextWake())
	}
	return w
}

// FrameAt 在每次 Frame 之前由 session 呼叫：Frame 回呼讀的步數改由回掃當下的 machine.Steps 供給（460 §2.4、§3.4）。
func (s *SessionObserver) FrameAt(steps uint64) {
	s.steps = steps
	if s.present != nil {
		s.present.steps = steps
	}
}

// nextWake：簽章層（observer.go Observe 末段）的期限。
func (o *LabelObserver) nextWake() uint64 {
	if len(o.visible) != 0 && o.baseKnown {
		return o.lastSig + SigCheckSteps
	}
	return session.NoWake
}

// nextWake：入口重看（pendOn，下一步）與簽章層（flight.go Observe 末段）。
func (o *FlightObserver) nextWake() uint64 {
	if o.pendOn {
		return 0
	}
	if len(o.visible) != 0 && o.baseOK {
		return o.lastSig + SigCheckSteps
	}
	return session.NoWake
}

// nextWake：最後一條字幕在等待時的逾時（subtitle.go Observe：Steps() > LastDue+T）。pending 不進（460 §2.2，由 SBDMA 之後的 Force 喚醒）。
func (s *Subtitles) nextWake() uint64 {
	if n := len(s.Subs); n != 0 {
		if c := s.Subs[n-1]; c.State == SubWaiting {
			return c.LastDue + s.T + 1
		}
	}
	return session.NoWake
}

// nextWake：recheck、afterRet 為下一步；窗堆疊非空時最早的逾時（present.go checkTimeouts）。cuPend、clearPend 是同步驟內消費，不進。
func (p *Presenter) nextWake() uint64 {
	if p.recheck != nil || p.afterRet {
		return 0
	}
	w := session.NoWake
	for _, win := range p.stack {
		w = min(w, win.entry+win.r.timeout)
	}
	return w
}
