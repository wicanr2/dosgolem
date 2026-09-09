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

	// PIT 通道 0 的分頻與寫入狀態機。**漏抄的話還原之後計時器
	// 會退回開機頻率**，而症狀是「同一個快照展開的變體跑得比原本慢」。
	irq0Every uint64
	irq0Base  uint64
	cpuHz     uint64
	cycles    uint64
	cycPer    uint64
	nextCyc   uint64
	pitDiv    uint32
	pitAccess uint8
	pitPhase  uint8
	pitLo     uint8

	ports   map[uint16]uint8
	portsIn map[uint16]uint64

	dac      [256 * 3]uint8
	dacIndex uint8
	dacPhase uint8

	// planar 的畫面與暫存器（`docs/spec/013`）。**四個 plane 不在 mem
	// 裡**，漏抄的話還原後畫面會是別的時間點的。
	vga      vga
	planarOn bool
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
		irq0Every: m.IRQ0Every,
		irq0Base:  m.IRQ0Base,
		cpuHz:     m.CPUHz,
		cycles:    m.CPU.Cycles,
		cycPer:    m.cycPerIRQ0,
		nextCyc:   m.nextIRQ0Cyc,
		pitDiv:    m.PITDiv,
		pitAccess: m.pitAccess,
		pitPhase:  m.pitPhase,
		pitLo:     m.pitLo,
		ports:     map[uint16]uint8{},
		portsIn:   map[uint16]uint64{},
		dac:       m.DAC,
		dacIndex:  m.dacIndex,
		dacPhase:  m.dacPhase,
		vga:       *m.vga,
		planarOn:  m.planarOn,
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
	m.IRQ0Every, m.IRQ0Base, m.PITDiv = s.irq0Every, s.irq0Base, s.pitDiv
	m.pitAccess, m.pitPhase, m.pitLo = s.pitAccess, s.pitPhase, s.pitLo
	m.CPUHz, m.CPU.Cycles = s.cpuHz, s.cycles
	m.cycPerIRQ0, m.nextIRQ0Cyc = s.cycPer, s.nextCyc

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
	*m.vga, m.planarOn = s.vga, s.planarOn
}

// 讓 cpu 這個 import 有用途（Snapshot 裡的暫存器型別來自它）。
var _ = cpu.AX
