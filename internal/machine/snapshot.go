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
	irq0Every  uint64
	irq0Base   uint64
	irq0Pinned bool
	cpuHz      uint64
	cycClock   bool
	cycles     uint64
	cycPer     uint64
	nextCyc    uint64
	pitDiv     uint32
	pitAccess  uint8
	pitPhase   uint8
	pitLo      uint8

	// 硬體鍵盤的整份狀態。**佇列與 nextIRQ1 漏掉會讓還原之後的鍵永遠送不出去**：
	// Restore 把 Steps 倒回過去，而 nextIRQ1 還停在未來，`keyTick` 的
	// 「時間還沒到」於是永遠成立。症狀是第一個變體收得到鍵、後面每一個都
	// 「按了沒反應」——看起來像那些送法不對，其實是送鍵這條路已經死了。
	keyQueue  []KeyEvent
	nextKey   uint64
	keyIRQs   uint64
	keyStalls uint64
	kbdData   uint8
	kbdPortB  uint8

	ports   map[uint16]uint8
	portsIn map[uint16]uint64

	dac      [256 * 3]uint8
	dacIndex uint8
	dacPhase uint8

	// ⚠ **平面模式的畫面不在 mem 裡。** 漏了這一段，從快照展開的機器
	// 記憶體與 CPU 全對，畫面卻是還原之前的那一張——而那看起來像
	// 「遊戲沒重畫」，不像「快照少存東西」。
	planarOn bool
	vga      *VGA

	// ⚠ **回呼與週期時鐘也是狀態。** 漏了的話從快照展開的機器
	// 「時鐘不會走」或「卡在一個永遠回不來的回呼裡」，
	// 而記憶體與 CPU 全對——與 nextIRQ0 那個坑同一個形狀。
	periodicOn                  bool
	periodicSeg, periodicOff    uint16
	periodicEvery, periodicNext uint64
	periodicCalls               uint64
	cbQueue                     []QueuedCall
	cbSaved                     callbackFrame
	cbActive                    bool
	cbMade                      uint64

	// PC 喇叭與 8253 通道 0（`docs/spec/016`）。
	// 分頻值不還原的話，還原之後的中斷頻率是**上一次跑到最後**的那個值。
	speaker     []SpeakerSample
	pit         pit
	irq0Clamped int
	picMask     uint8
}

// Mem 回快照裡的記憶體，給差分比對用。**不要改它。**
func (s *Snapshot) Mem() []uint8 { return s.mem }

// Snapshot 拍一份快照。1 MB 記憶體，約 1 毫秒。
func (m *Machine) Snapshot() *Snapshot {
	s := &Snapshot{
		mem:        make([]uint8, len(m.Mem)),
		regs:       m.CPU.R,
		segs:       m.CPU.Seg,
		ip:         m.CPU.IP,
		flags:      m.CPU.Flags,
		steps:      m.Steps,
		ticks:      m.Ticks,
		portTicks:  m.portTicks,
		nextIRQ0:   m.nextIRQ0,
		pending:    m.irq0Pending,
		irq0Every:  m.IRQ0Every,
		irq0Base:   m.IRQ0Base,
		irq0Pinned: m.IRQ0Pinned,
		cpuHz:      m.CPUHz,
		cycClock:   m.CycleClock,
		cycles:     m.CPU.Cycles,
		cycPer:     m.cycPerIRQ0,
		nextCyc:    m.nextIRQ0Cyc,
		pitDiv:     m.PITDiv,
		pitAccess:  m.pitAccess,
		pitPhase:   m.pitPhase,
		pitLo:      m.pitLo,
		ports:      map[uint16]uint8{},
		portsIn:    map[uint16]uint64{},
		dac:        m.DAC,
		dacIndex:   m.dacIndex,
		dacPhase:   m.dacPhase,
		vga:        m.VGA.clone(),
		planarOn:   m.planarOn,

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

		keyQueue:  append([]KeyEvent(nil), m.keyQueue...),
		nextKey:   m.nextKey,
		keyIRQs:   m.KeyIRQs,
		keyStalls: m.keyStalls,
		kbdData:   m.kbdData,
		kbdPortB:  m.kbdPortB,

		speaker:     append([]SpeakerSample(nil), m.Speaker...),
		pit:         m.pit,
		irq0Clamped: m.IRQ0Clamped,
		picMask:     m.picMask,
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
	m.IRQ0Every, m.IRQ0Base, m.PITDiv = s.irq0Every, s.irq0Base, s.pitDiv
	m.IRQ0Pinned = s.irq0Pinned
	m.pitAccess, m.pitPhase, m.pitLo = s.pitAccess, s.pitPhase, s.pitLo
	m.CPUHz, m.CPU.Cycles, m.CycleClock = s.cpuHz, s.cycles, s.cycClock
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
	m.VGA.restore(s.vga)
	m.planarOn = s.planarOn

	m.periodic.on, m.periodic.seg, m.periodic.off = s.periodicOn, s.periodicSeg, s.periodicOff
	m.periodic.every, m.periodic.next = s.periodicEvery, s.periodicNext
	m.periodic.Calls = s.periodicCalls
	m.cbQueue = append(m.cbQueue[:0], s.cbQueue...)
	m.cbSaved, m.cbActive, m.cbMade = s.cbSaved, s.cbActive, s.cbMade

	m.keyQueue = append([]KeyEvent(nil), s.keyQueue...)
	m.nextKey, m.KeyIRQs, m.keyStalls = s.nextKey, s.keyIRQs, s.keyStalls
	m.kbdData, m.kbdPortB = s.kbdData, s.kbdPortB

	m.Speaker = append(m.Speaker[:0], s.speaker...)
	m.pit, m.picMask = s.pit, s.picMask
	m.IRQ0Clamped = s.irq0Clamped
}

// 讓 cpu 這個 import 有用途（Snapshot 裡的暫存器型別來自它）。
var _ = cpu.AX
