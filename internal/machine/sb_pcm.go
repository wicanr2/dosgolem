package machine

// SBPCMEvent 是 462 §3.2 的唯讀音訊事件。Data 只在非靜音 start 配置，
// 所有權交給回呼，不與機器記憶體或 SBDMALog 共用。
type SBPCMEvent struct {
	Kind        string
	Step        uint64
	Clock       string
	Command     uint8
	Format      string
	Length      uint32
	Samples     uint64
	Start, Due  uint64
	Transferred uint32
	SHA256      string
	Data        []byte
}

// ObserveSBPCM 安裝音訊觀測器；nil 關閉。不補送載入當下的傳輸。
func (m *Machine) ObserveSBPCM(fn func(SBPCMEvent)) { m.onSBPCM = fn }

// SBPCMMutated 回傳觀測到的傳輸期間緩衝改寫次數。
func (m *Machine) SBPCMMutated() uint64 { return m.sb8pcmMutated }

func (m *Machine) sb8PCMReset() {
	m.sb8pcmData, m.sb8pcmStartSHA, m.sb8pcmMutated = nil, "", 0
}

func (m *Machine) sb8PCM(e SBDMAEvent) {
	if m.onSBPCM == nil {
		return
	}
	p := SBPCMEvent{Kind: e.Kind, Step: e.Step, Clock: e.Clock, Command: e.Command,
		Format: e.Format, Length: e.Length, Samples: e.Samples, Start: m.sb8.start,
		Due: e.Due, Transferred: e.Transferred}
	if e.Kind == "start" {
		p.SHA256, p.Data = e.SHA256, m.sb8pcmData
		m.sb8pcmData = nil
	}
	if e.Kind == "cancel" {
		m.sb8pcmStartSHA = ""
	}
	m.onSBPCM(p)
}
