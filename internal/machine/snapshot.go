package machine

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

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
	// 80386 暫存器高半與 FS／GS（`docs/spec/197`）；漏抄的話展開出來的變體
	// 高半歸零，而記憶體與 16 位元暫存器全對。
	hi         [8]uint16
	fseg, gseg uint16

	steps     uint64
	ticks     uint64
	portTicks uint64
	nextIRQ0  uint64
	pending   bool

	// PIT 通道 0 的分頻與寫入狀態機。**漏抄的話還原之後計時器
	// 會退回開機頻率**，而症狀是「同一個快照展開的變體跑得比原本慢」。
	irq0Every  uint64
	irq0Base   uint64
	cpuHz      uint64
	cycClock   bool
	dosboxCost bool
	irq0Pinned bool
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
	timedKeys []timedKey // `docs/spec/197`：同樣的理由，按住中的放開碼漏掉的話鍵會卡住
	timedSeq  uint64
	nextKey   uint64
	// 回掃與鍵盤間隔、時脈比例（`docs/spec/242`）。
	vgaEvery, nextFrame, keyEvery uint64
	clockPct                      int
	keyIRQs                       uint64
	keyStalls                     uint64
	kbdData                       uint8
	kbdPortB                      uint8

	ports   map[uint16]uint8
	portsIn map[uint16]uint64

	dac          [256 * 3]uint8
	dacIndex     uint8
	dacPhase     uint8
	dacReadIndex uint8
	dacReadPhase uint8
	dacMask      uint8

	// ⚠ **平面模式的畫面不在 mem 裡。** 漏了這一段，從快照展開的機器
	// 記憶體與 CPU 全對，畫面卻是還原之前的那一張——而那看起來像
	// 「遊戲沒重畫」，不像「快照少存東西」。
	planarOn bool
	// A20 與 HMA（`docs/spec/191-bios-rom-tail` §4）。HMA 是陣列，值複製。
	a20 bool
	hma [HMASize]uint8
	// 430：Ext（kb == 0 時為 nil）、8042 旗標與 EMS 旗標。
	ext             []uint8
	kbdAwaitOutport bool
	emsAbsent       bool

	// 432：保護模式 profile 與其狀態（衍生欄位、TLB 不存，Restore 時重算）。
	pmProfile bool
	pmFields

	// SetDOSFiles 的「已載入」旗標（`docs/spec/429-dos-sft-chain`）。
	programLoaded bool

	vga *VGA

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
//
// kb > 0（SetExtendedMemoryKB）時要複製整個 Ext，成本隨 kb 線性增加
// （63 MB 約數十毫秒，且每張快照多佔同樣大的記憶體）。
func (m *Machine) Snapshot() *Snapshot {
	s := &Snapshot{
		mem:          make([]uint8, len(m.Mem)),
		regs:         m.CPU.R,
		hi:           m.CPU.Hi,
		fseg:         m.CPU.FSeg,
		gseg:         m.CPU.GSeg,
		segs:         m.CPU.Seg,
		ip:           m.CPU.IP,
		flags:        m.CPU.Flags,
		steps:        m.Steps,
		ticks:        m.Ticks,
		portTicks:    m.portTicks,
		nextIRQ0:     m.nextIRQ0,
		pending:      m.irq0Pending,
		irq0Every:    m.IRQ0Every,
		irq0Base:     m.IRQ0Base,
		cpuHz:        m.CPUHz,
		cycClock:     m.CycleClock,
		dosboxCost:   m.CPU.DOSBoxCost,
		cycles:       m.CPU.Cycles,
		cycPer:       m.cycPerIRQ0,
		nextCyc:      m.nextIRQ0Cyc,
		pitDiv:       m.PITDiv,
		pitAccess:    m.pitAccess,
		pitPhase:     m.pitPhase,
		pitLo:        m.pitLo,
		ports:        map[uint16]uint8{},
		portsIn:      map[uint16]uint64{},
		dac:          m.DAC,
		dacMask:      m.dacMask,
		dacIndex:     m.dacIndex,
		dacPhase:     m.dacPhase,
		dacReadIndex: m.dacReadIndex,
		dacReadPhase: m.dacReadPhase,
		vga:          m.VGA.clone(),
		planarOn:     m.planarOn,
		irq0Pinned:   m.IRQ0Pinned,
		a20:          m.a20,
		hma:          m.hma,

		kbdAwaitOutport: m.kbdAwaitOutport,
		emsAbsent:       m.emsAbsent,

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
		timedKeys: append([]timedKey(nil), m.timedKeys...),
		timedSeq:  m.timedKeySeq,
		nextKey:   m.nextKey,
		vgaEvery:  m.VGAFrameEvery, nextFrame: m.nextFrame, keyEvery: m.KeyEvery, clockPct: m.clockPct,
		keyIRQs:   m.KeyIRQs,
		keyStalls: m.keyStalls,
		kbdData:   m.kbdData,
		kbdPortB:  m.kbdPortB,

		speaker:     append([]SpeakerSample(nil), m.Speaker...),
		pit:         m.pit,
		irq0Clamped: m.IRQ0Clamped,
		picMask:     m.picMask,

		programLoaded: m.programLoaded,

		pmProfile: m.pmProfile,
		pmFields:  pmFields{cpl: m.pmCPL, cr0: m.cr0, cr3: m.cr3, eflagsHi: m.eflagsHi},
	}
	s.vga = m.VGA.clone()
	if m.Ext != nil {
		s.ext = append([]uint8(nil), m.Ext...)
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

// Restore 把機器倒回快照。沒有錯誤回傳：快照的 Ext 長度與這台機器不同時 panic
// （Ext 在程式載入前就設好，同一台機器的長度不會變）。
func (m *Machine) Restore(s *Snapshot) {
	if s.pmProfile != m.pmProfile {
		panic(fmt.Sprintf("machine: Restore 的快照保護模式 profile 是 %t，這台是 %t", s.pmProfile, m.pmProfile))
	}
	if len(s.ext) != len(m.Ext) {
		panic(fmt.Sprintf("machine: Restore 的快照延伸記憶體是 %d bytes，這台是 %d bytes", len(s.ext), len(m.Ext)))
	}
	copy(m.Mem, s.mem)
	copy(m.Ext, s.ext)
	m.kbdAwaitOutport, m.emsAbsent = s.kbdAwaitOutport, s.emsAbsent
	m.CPU.R, m.CPU.Seg, m.CPU.IP = s.regs, s.segs, s.ip
	m.CPU.Hi, m.CPU.FSeg, m.CPU.GSeg = s.hi, s.fseg, s.gseg
	m.CPU.SetFlags(s.flags)
	m.CPU.Halted = false

	m.Steps, m.Ticks = s.steps, s.ticks
	m.portTicks, m.nextIRQ0, m.irq0Pending = s.portTicks, s.nextIRQ0, s.pending
	m.IRQ0Every, m.IRQ0Base, m.PITDiv = s.irq0Every, s.irq0Base, s.pitDiv
	m.IRQ0Pinned = s.irq0Pinned
	m.pitAccess, m.pitPhase, m.pitLo = s.pitAccess, s.pitPhase, s.pitLo
	m.CPUHz, m.CPU.Cycles, m.CycleClock = s.cpuHz, s.cycles, s.cycClock
	m.CPU.DOSBoxCost = s.dosboxCost
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
	m.dacReadIndex, m.dacReadPhase = s.dacReadIndex, s.dacReadPhase
	m.dacMask = s.dacMask
	m.VGA.restore(s.vga)
	m.planarOn = s.planarOn
	m.a20, m.hma = s.a20, s.hma
	m.programLoaded = s.programLoaded
	m.syncCodeFastPath()

	m.periodic.on, m.periodic.seg, m.periodic.off = s.periodicOn, s.periodicSeg, s.periodicOff
	m.periodic.every, m.periodic.next = s.periodicEvery, s.periodicNext
	m.periodic.Calls = s.periodicCalls
	m.cbQueue = append(m.cbQueue[:0], s.cbQueue...)
	m.cbSaved, m.cbActive, m.cbMade = s.cbSaved, s.cbActive, s.cbMade

	m.keyQueue = append([]KeyEvent(nil), s.keyQueue...)
	m.timedKeys, m.timedKeySeq = append([]timedKey(nil), s.timedKeys...), s.timedSeq
	m.nextKey, m.KeyIRQs, m.keyStalls = s.nextKey, s.keyIRQs, s.keyStalls
	m.VGAFrameEvery, m.nextFrame, m.KeyEvery, m.clockPct = s.vgaEvery, s.nextFrame, s.keyEvery, s.clockPct
	m.kbdData, m.kbdPortB = s.kbdData, s.kbdPortB

	m.Speaker = append(m.Speaker[:0], s.speaker...)
	m.syncSpeakerLast()
	m.pit, m.picMask = s.pit, s.picMask
	m.IRQ0Clamped = s.irq0Clamped

	// 432：保護模式狀態。TLB 世代加一，衍生欄位與 CPU.Code 依還原後的 CR0 重算
	// （上面 syncCodeFastPath 用的是還原前的 xlat）。
	m.applyPM(s.pmFields)
}

// 讓 cpu 這個 import 有用途（Snapshot 裡的暫存器型別來自它）。
var _ = cpu.AX
