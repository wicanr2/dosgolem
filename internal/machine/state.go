package machine

import (
	"encoding/gob"
	"fmt"
	"io"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 存檔／讀檔：把整台機器寫成檔案，下一支行程直接從那裡接著跑。
//
// **為什麼要有它**：要觀測的畫面常常在幾億道指令之後（源平合戰跑到玩家
// 能下指令的命令面板要八億五千萬道，約八分鐘）。每問一個問題就從頭跑一次，
// 迴圈慢到只能用猜的。存一次、之後每次實驗從那裡展開，迴圈是秒級的。
//
// ⚠ **`Snapshot`（記憶體內）與這裡（檔案）存的是同一組欄位**，
// 新增機器狀態時兩邊都要補。漏掉的欄位不會報錯——還原之後機器看起來
// 正常，只是某個時鐘或某個 plane 停在別的時間點。

// stateMagic 認檔用；版本不合就拒絕，不要讓舊檔悄悄餵出錯的狀態。
const (
	stateMagic   = "DOSGOLEM-M"
	stateVersion = 1
)

// machineState 是機器狀態的線上格式。**欄位要匯出**，gob 才看得到。
type machineState struct {
	Magic   string
	Version int

	Mem []uint8

	R      [8]uint16
	Seg    [4]uint16
	IP     uint16
	Flags  uint16
	Halted bool
	Model  int

	Steps       uint64
	Ticks       uint64
	PortTicks   uint64
	NextIRQ0    uint64
	IRQ0Pending bool
	IRQ0Every   uint64

	Ports   map[uint16]uint8
	PortsIn map[uint16]uint64

	DAC      []uint8
	DACIndex uint8
	DACPhase uint8

	Planes   []uint8
	Latch    [4]uint8
	SeqIdx   uint8
	Seq      [8]uint8
	GCIdx    uint8
	GC       [9]uint8
	PlanarOn bool

	FreeSeg   uint16
	ImageBase uint32
	ImageLen  int
}

// SaveState 把整台機器寫出去。
func (m *Machine) SaveState(w io.Writer) error {
	s := machineState{
		Magic: stateMagic, Version: stateVersion,
		Mem:         append([]uint8(nil), m.Mem...),
		R:           m.CPU.R,
		Seg:         m.CPU.Seg,
		IP:          m.CPU.IP,
		Flags:       m.CPU.Flags,
		Halted:      m.CPU.Halted,
		Model:       int(m.CPU.Model),
		Steps:       m.Steps,
		Ticks:       m.Ticks,
		PortTicks:   m.portTicks,
		NextIRQ0:    m.nextIRQ0,
		IRQ0Pending: m.irq0Pending,
		IRQ0Every:   m.IRQ0Every,
		Ports:       map[uint16]uint8{},
		PortsIn:     map[uint16]uint64{},
		DAC:         append([]uint8(nil), m.DAC[:]...),
		DACIndex:    m.dacIndex,
		DACPhase:    m.dacPhase,
		Planes:      append([]uint8(nil), m.vga.planes[:]...),
		Latch:       m.vga.latch,
		SeqIdx:      m.vga.seqIdx,
		Seq:         m.vga.seq,
		GCIdx:       m.vga.gcIdx,
		GC:          m.vga.gc,
		PlanarOn:    m.planarOn,
		FreeSeg:     m.FreeSeg,
		ImageBase:   m.ImageBase,
		ImageLen:    m.ImageLen,
	}
	for k, v := range m.Ports {
		s.Ports[k] = v
	}
	for k, v := range m.PortsIn {
		s.PortsIn[k] = v
	}
	return gob.NewEncoder(w).Encode(&s)
}

// LoadState 把機器倒回存檔的狀態。
func (m *Machine) LoadState(r io.Reader) error {
	var s machineState
	if err := gob.NewDecoder(r).Decode(&s); err != nil {
		return fmt.Errorf("machine: 讀不開狀態檔：%w", err)
	}
	if s.Magic != stateMagic || s.Version != stateVersion {
		return fmt.Errorf("machine: 狀態檔不認得（%q v%d）", s.Magic, s.Version)
	}
	if len(s.Mem) != len(m.Mem) {
		return fmt.Errorf("machine: 狀態檔的記憶體是 %d bytes，這台是 %d", len(s.Mem), len(m.Mem))
	}
	copy(m.Mem, s.Mem)
	m.CPU.R, m.CPU.Seg, m.CPU.IP = s.R, s.Seg, s.IP
	m.CPU.SetFlags(s.Flags)
	m.CPU.Halted = s.Halted
	m.CPU.Model = cpu.Model(s.Model)

	m.Steps, m.Ticks = s.Steps, s.Ticks
	m.portTicks, m.nextIRQ0, m.irq0Pending = s.PortTicks, s.NextIRQ0, s.IRQ0Pending
	m.IRQ0Every = s.IRQ0Every

	m.Ports = map[uint16]uint8{}
	for k, v := range s.Ports {
		m.Ports[k] = v
	}
	m.PortsIn = map[uint16]uint64{}
	for k, v := range s.PortsIn {
		m.PortsIn[k] = v
	}
	m.PortLog = m.PortLog[:0]

	copy(m.DAC[:], s.DAC)
	m.dacIndex, m.dacPhase = s.DACIndex, s.DACPhase
	copy(m.vga.planes[:], s.Planes)
	m.vga.latch, m.vga.seqIdx, m.vga.seq = s.Latch, s.SeqIdx, s.Seq
	m.vga.gcIdx, m.vga.gc = s.GCIdx, s.GC
	m.planarOn = s.PlanarOn

	m.FreeSeg, m.ImageBase, m.ImageLen = s.FreeSeg, s.ImageBase, s.ImageLen
	return nil
}
