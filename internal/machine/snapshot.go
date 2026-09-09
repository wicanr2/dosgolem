package machine

import "github.com/wicanr2/dosgolem/internal/cpu"

// Snapshot 是一份完整的機器狀態。
//
// ⚠ **「完整」是字面意思，包含所有內部時鐘。** 只存記憶體與 CPU 的話，
// 還原之後 `nextIRQ0` 還停在未來的某個大值——**計時器中斷從此不再送**，
// 於是遊戲的動畫停住、輪詢變少，而畫面看起來完全正常。
//
// 實測症狀：從快照展開四個變體，第一個（沒還原過）跑得好好的，
// 後面三個「點下去遊戲沒輪詢到」。查到最後是這裡漏了三個欄位。
type Snapshot struct {
	mem   []uint8
	regs  [8]uint16
	segs  [4]uint16
	ip    uint16
	flags uint16

	steps     uint64
	ticks     uint64
	portTicks uint64
	nextIRQ0  uint64
	pending   bool

	// 硬體鍵盤的整份狀態。**佇列與 nextIRQ1 漏掉會讓還原之後的鍵永遠送不出去**：
	// Restore 把 Steps 倒回過去，而 nextIRQ1 還停在未來，`keyTick` 的
	// 「時間還沒到」於是永遠成立。症狀是第一個變體收得到鍵、後面每一個都
	// 「按了沒反應」——看起來像那些送法不對，其實是送鍵這條路已經死了。
	keyQueue  []KeyEvent
	nextIRQ1  uint64
	irq1Count int
	irq1Own   int
	kbdData   uint8
	kbdPortB  uint8

	ports   map[uint16]uint8
	portsIn map[uint16]uint64

	dac      [256 * 3]uint8
	dacIndex uint8
	dacPhase uint8

	// 平面式 VRAM。**線性檢視與平面是兩份資料**，只還原前者的話
	// 畫面會是兩次執行混在一起的，而那張圖看起來完全正常（見 egaState）。
	ega *egaState

	// PC 喇叭與 8253 通道 0（`docs/spec/016`）。
	// 分頻值不還原的話，還原之後的中斷頻率是**上一次跑到最後**的那個值。
	speaker     []SpeakerSample
	pit         pit
	irq0Every   uint64
	irq0Clamped int
	picMask     uint8
}

// Mem 回快照裡的記憶體，給差分比對用。**不要改它。**
func (s *Snapshot) Mem() []uint8 { return s.mem }

// Snapshot 拍一份快照。1 MB 記憶體，約 1 毫秒。
func (m *Machine) Snapshot() *Snapshot {
	s := &Snapshot{
		mem:       make([]uint8, len(m.Mem)),
		regs:      m.CPU.R,
		segs:      m.CPU.Seg,
		ip:        m.CPU.IP,
		flags:     m.CPU.Flags,
		steps:     m.Steps,
		ticks:     m.Ticks,
		portTicks: m.portTicks,
		nextIRQ0:  m.nextIRQ0,
		pending:   m.irq0Pending,
		ports:     map[uint16]uint8{},
		portsIn:   map[uint16]uint64{},
		dac:       m.DAC,
		dacIndex:  m.dacIndex,
		dacPhase:  m.dacPhase,

		keyQueue:  append([]KeyEvent(nil), m.keyQueue...),
		nextIRQ1:  m.nextIRQ1,
		irq1Count: m.irq1Count,
		irq1Own:   m.irq1Own,
		kbdData:   m.kbdData,
		kbdPortB:  m.kbdPortB,

		ega:         m.ega.snapshot(),
		speaker:     append([]SpeakerSample(nil), m.Speaker...),
		pit:         m.pit,
		irq0Every:   m.IRQ0Every,
		irq0Clamped: m.IRQ0Clamped,
		picMask:     m.picMask,
	}
	copy(s.mem, m.Mem)
	for k, v := range m.Ports {
		s.ports[k] = v
	}
	for k, v := range m.PortsIn {
		s.portsIn[k] = v
	}
	return s
}

// Restore 把機器倒回快照。
func (m *Machine) Restore(s *Snapshot) {
	copy(m.Mem, s.mem)
	m.CPU.R, m.CPU.Seg, m.CPU.IP = s.regs, s.segs, s.ip
	m.CPU.SetFlags(s.flags)
	m.CPU.Halted = false

	m.Steps, m.Ticks = s.steps, s.ticks
	m.portTicks, m.nextIRQ0, m.irq0Pending = s.portTicks, s.nextIRQ0, s.pending

	m.Ports = map[uint16]uint8{}
	for k, v := range s.ports {
		m.Ports[k] = v
	}
	m.PortsIn = map[uint16]uint64{}
	for k, v := range s.portsIn {
		m.PortsIn[k] = v
	}
	m.PortLog = m.PortLog[:0]

	m.DAC, m.dacIndex, m.dacPhase = s.dac, s.dacIndex, s.dacPhase

	m.keyQueue = append([]KeyEvent(nil), s.keyQueue...)
	m.nextIRQ1, m.irq1Count, m.irq1Own = s.nextIRQ1, s.irq1Count, s.irq1Own
	m.kbdData, m.kbdPortB = s.kbdData, s.kbdPortB

	m.ega.restore(s.ega)
	m.Speaker = append(m.Speaker[:0], s.speaker...)
	m.pit, m.picMask = s.pit, s.picMask
	m.IRQ0Every, m.IRQ0Clamped = s.irq0Every, s.irq0Clamped
}

// 讓 cpu 這個 import 有用途（Snapshot 裡的暫存器型別來自它）。
var _ = cpu.AX
