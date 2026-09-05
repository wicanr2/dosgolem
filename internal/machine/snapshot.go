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

	ports   map[uint16]uint8
	portsIn map[uint16]uint64

	dac      [256 * 3]uint8
	dacIndex uint8
	dacPhase uint8

	// ⚠ **平面模式的畫面不在 mem 裡。** 漏了這一段，從快照展開的機器
	// 記憶體與 CPU 全對，畫面卻是還原之前的那一張——而那看起來像
	// 「遊戲沒重畫」，不像「快照少存東西」。
	planar bool
	vga    VGA

	// ⚠ **回呼與週期時鐘也是狀態。** 漏了的話從快照展開的機器
	// 「時鐘不會走」或「卡在一個永遠回不來的回呼裡」，
	// 而記憶體與 CPU 全對——與 nextIRQ0 那個坑同一個形狀。
	periodicOn                   bool
	periodicSeg, periodicOff     uint16
	periodicEvery, periodicNext  uint64
	periodicCalls                uint64
	cbQueue                      []QueuedCall
	cbSaved                      callbackFrame
	cbActive                     bool
	cbMade                       uint64
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
		planar:    m.planar,

		periodicOn:    m.periodic.on,
		periodicSeg:   m.periodic.seg,
		periodicOff:   m.periodic.off,
		periodicEvery: m.periodic.every,
		periodicNext:  m.periodic.next,
		periodicCalls: m.periodic.Calls,
		cbQueue:       append([]QueuedCall(nil), m.cbQueue...),
		cbSaved:       m.cbSaved,
		cbActive:      m.cbActive,
		cbMade:        m.cbMade,
	}
	s.vga = m.VGA.clone()
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
	m.planar = s.planar
	m.VGA.restore(&s.vga)

	m.periodic.on, m.periodic.seg, m.periodic.off = s.periodicOn, s.periodicSeg, s.periodicOff
	m.periodic.every, m.periodic.next = s.periodicEvery, s.periodicNext
	m.periodic.Calls = s.periodicCalls
	m.cbQueue = append(m.cbQueue[:0], s.cbQueue...)
	m.cbSaved, m.cbActive, m.cbMade = s.cbSaved, s.cbActive, s.cbMade
}

// 讓 cpu 這個 import 有用途（Snapshot 裡的暫存器型別來自它）。
var _ = cpu.AX
