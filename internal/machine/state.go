package machine

import (
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"sort"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// PortState／PortCount 是埠統計的排序線上版本（map 編 gob 順序隨機）。
type PortState struct {
	Port uint16
	Val  uint8
}

type PortCount struct {
	Port  uint16
	Count uint64
}

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
//
// v3 加了 A20 與 HMA（`docs/spec/191-bios-rom-tail` §4）。**v2 仍然讀**：
// 那一版沒存這兩項，讀成 A20 關、HMA 全 0——與 v2 執行器讀回來的結果
// 相同，所以舊檢查點展開的行為不變；只有「存檔那一刻 A20 開著」的 v2 檔
// 會錯，那種檔要從開機重錄。讀到 v2 時 LegacyState 會記下來。
const (
	stateMagic      = "DOSGOLEM-M"
	stateVersion    = 3
	stateVersionMin = 2
	// stateVersionMax 是 LoadState 接受的最高版本：4 只在 444 的選項開啟時寫（stateVersionSB8）。
	stateVersionMax = stateVersionSB8
)

// machineState 是機器狀態的線上格式。**欄位要匯出**，gob 才看得到。
type machineState struct {
	Magic   string
	Version int

	Mem []uint8
	// A20／HMA：v3 起才有（v2 解出來是零值）。
	A20 bool
	HMA []uint8

	R      [8]uint16
	Seg    [4]uint16
	IP     uint16
	Flags  uint16
	Halted bool
	Model  int

	// 80386 的暫存器高半與 FS／GS（`docs/spec/197`）。**漏存的話還原之後
	// 32 位元運算的高半歸零、FS／GS 段值消失**，而機器照常跑。
	// gob 對新增欄位相容：舊狀態檔讀進來是零值，不必升版號。
	Hi         [8]uint16
	FSeg, GSeg uint16

	Steps       uint64
	Ticks       uint64
	PortTicks   uint64
	NextIRQ0    uint64
	IRQ0Pending bool
	IRQ0Every   uint64
	IRQ0Base    uint64
	IRQ0Pinned  bool
	CPUHz       uint64
	CycleClock  bool
	DOSBoxCost  bool // `docs/spec/198`；舊狀態檔讀進來是 false
	Cycles      uint64
	NextIRQ0Cyc uint64
	PITDiv      uint32
	PITAccess   uint8
	PITPhase    uint8
	PITLo       uint8

	// `docs/spec/242`：242 之前的存檔沒有這幾個欄位（ClockPercent 為 0）。
	VGAFrameEvery uint64
	NextFrame     uint64
	KeyEvery      uint64
	NextKey       uint64
	ClockPercent  int

	// 埠寫入統計用**排序過的 slice**（PortList／PortInList），不用 map——gob 編
	// map 的順序隨機，兩次一模一樣的執行會存出不同 bytes，狀態檔就沒辦法
	// 用雜湊驗決定性（`docs/spec/002` §5 的精神）。
	//
	// Ports／PortsIn 只留著讀舊檔：同名欄位換型別的話 gob 解不開，
	// 既有的狀態檔會全部讀不回來。存檔時兩個 map 都是 nil，gob 不寫它們。
	Ports      map[uint16]uint8
	PortsIn    map[uint16]uint64
	PortList   []PortState
	PortInList []PortCount

	DAC            []uint8
	DACIndex       uint8
	DACPhase       uint8
	DACReadIndex   uint8
	DACReadPhase   uint8
	DACReadKnown   bool
	DACMaskPresent bool // 舊 v2 缺此欄位；遮罩 0 本身是合法值。
	DACMask        uint8

	Planes []uint8
	Latch  [4]uint8
	SeqIdx uint8
	Seq    [8]uint8
	GCIdx  uint8
	GC     [16]uint8
	// 屬性控制器：調色盤暫存器、模式控制與色彩選擇。**漏掉它的話
	// 還原之後畫面的形狀對、顏色錯**——而且看起來像是程式自己畫錯的。
	AC       [32]uint8
	ACIdx    uint8
	ACFlip   bool
	PlanarOn bool
	// CRTC 與 Misc Output（顯示起點、列距、分割線；`docs/spec/201`）。
	// 舊狀態檔沒有它們：讀進來是零值時沿用目前這一台的值。
	CRTC    [32]uint8
	CRTCIdx uint8
	Misc    uint8
	HasCRTC bool

	// ProgramPath 是環境區塊裡的程式全路徑（MSC 的 argv[0]）。
	ProgramPath string

	FreeSeg   uint16
	ImageBase uint32
	ImageLen  int

	// 430（docs/spec/430-physical-memory-and-xms-unification §3）：
	// Ext 是 1 MB 之上的延伸記憶體（含 HMA），kb == 0 時為空；EMSAbsent 是
	// SetEMSPresent(false)；KbdAwaitOutport 是 8042 的 D1h 狀態機旗標。
	// 都標 omitempty，零值不進 StateDigest 的 JSON。
	Ext             []uint8 `json:",omitempty"`
	EMSAbsent       bool    `json:",omitempty"`
	KbdAwaitOutport bool    `json:",omitempty"`

	// PM 是保護模式狀態的不透明欄位（docs/spec/431-hybrid-cpu-protected-mode §4.2、
	// docs/spec/432-linear-memory-and-paging §7）。內容自帶版本號與編碼；profile 關閉時為 nil，
	// 不進 StateDigest 的 JSON。
	//
	// ⚠ gob 位元組的政策：語意錨點（記憶體雜湊、StateDigest）永不變；gob 型別描述
	// （錨點測試的位元組常數）在 430、432 各變一次，之後 433–435 的保護模式狀態只改
	// PM 的內部格式，不得再增減或改型別機器狀態欄位。
	PM []byte `json:",omitempty"`
}

// SaveState 把整台機器寫出去。
func (m *Machine) SaveState(w io.Writer) error {
	s := machineState{
		Magic: stateMagic, Version: stateVersion,
		Mem:           append([]uint8(nil), m.Mem...),
		R:             m.CPU.R,
		Hi:            m.CPU.Hi,
		FSeg:          m.CPU.FSeg,
		GSeg:          m.CPU.GSeg,
		Seg:           m.CPU.Seg,
		IP:            m.CPU.IP,
		Flags:         m.CPU.Flags,
		Halted:        m.CPU.Halted,
		Model:         int(m.CPU.Model),
		Steps:         m.Steps,
		Ticks:         m.Ticks,
		PortTicks:     m.portTicks,
		NextIRQ0:      m.nextIRQ0,
		IRQ0Pending:   m.irq0Pending,
		IRQ0Every:     m.IRQ0Every,
		IRQ0Base:      m.IRQ0Base,
		CPUHz:         m.CPUHz,
		CycleClock:    m.CycleClock,
		DOSBoxCost:    m.CPU.DOSBoxCost,
		Cycles:        m.CPU.Cycles,
		NextIRQ0Cyc:   m.nextIRQ0Cyc,
		PITDiv:        m.PITDiv,
		PITAccess:     m.pitAccess,
		PITPhase:      m.pitPhase,
		PITLo:         m.pitLo,
		VGAFrameEvery: m.VGAFrameEvery, NextFrame: m.nextFrame, KeyEvery: m.KeyEvery, NextKey: m.nextKey,
		ClockPercent:   m.ClockPercent(),
		DAC:            append([]uint8(nil), m.DAC[:]...),
		DACMaskPresent: true,
		DACMask:        m.dacMask,
		DACIndex:       m.dacIndex,
		DACPhase:       m.dacPhase,
		DACReadIndex:   m.dacReadIndex,
		DACReadPhase:   m.dacReadPhase,
		DACReadKnown:   true,
		Planes:         append([]uint8(nil), m.VGA.Raw()...),
		Latch:          m.VGA.latch,
		SeqIdx:         m.VGA.seqIdx,
		Seq:            m.VGA.seq,
		GCIdx:          m.VGA.gcIdx,
		GC:             m.VGA.gc,
		AC:             m.VGA.ac,
		ACIdx:          m.VGA.acIdx,
		ACFlip:         m.VGA.acFlip,
		PlanarOn:       m.planarOn,
		CRTC:           m.VGA.crtc,
		CRTCIdx:        m.VGA.crtcIdx,
		Misc:           m.VGA.misc,
		HasCRTC:        true,
		ProgramPath:    m.ProgramPath,
		FreeSeg:        m.FreeSeg,
		ImageBase:      m.ImageBase,
		ImageLen:       m.ImageLen,
		A20:            m.a20,
		HMA:            append([]uint8(nil), m.hma[:]...),
		IRQ0Pinned:     m.IRQ0Pinned,

		EMSAbsent:       m.emsAbsent,
		KbdAwaitOutport: m.kbdAwaitOutport,
		PM:              m.encodePM(), // profile 關閉時為 nil
	}
	// kb > 0：HMA 就是 Ext[0:HMASize]，不另存 HMA 欄位（規格 430 §3）。
	if m.Ext != nil {
		s.HMA = nil
		s.Ext = append([]uint8(nil), m.Ext...)
	}
	for k, v := range m.Ports {
		s.PortList = append(s.PortList, PortState{Port: k, Val: v})
	}
	sort.Slice(s.PortList, func(i, j int) bool { return s.PortList[i].Port < s.PortList[j].Port })
	for k, v := range m.PortsIn {
		s.PortInList = append(s.PortInList, PortCount{Port: k, Count: v})
	}
	sort.Slice(s.PortInList, func(i, j int) bool { return s.PortInList[i].Port < s.PortInList[j].Port })
	// 444 §2.9：選項開啟時寫版本 4，以同一個 Encoder 緊接著編碼 sbDMA8State；關閉時與 444 之前逐位元組相同。
	if m.sb8 != nil {
		s.Version = stateVersionSB8
	}
	enc := gob.NewEncoder(w)
	if err := enc.Encode(&s); err != nil {
		return err
	}
	if m.sb8 != nil {
		return enc.Encode(m.sb8Save())
	}
	return nil
}

// LoadState 把機器倒回存檔的狀態。
func (m *Machine) LoadState(r io.Reader) error {
	var s machineState
	dec := gob.NewDecoder(r)
	if err := dec.Decode(&s); err != nil {
		return fmt.Errorf("machine: 讀不開狀態檔：%w", err)
	}
	if s.Magic != stateMagic || s.Version < stateVersionMin || s.Version > stateVersionMax {
		return fmt.Errorf("machine: 狀態檔不認得（%q v%d）", s.Magic, s.Version)
	}
	// 444 §2.9：選項開關與「版本是否為 4」要一致；版本 4 以同一個 Decoder 再解 sbDMA8State。都在任何寫入之前。
	var sb8 *sbDMA8State
	if is4 := s.Version == stateVersionSB8; is4 != (m.sb8 != nil) {
		return fmt.Errorf("machine: 狀態檔版本 %d 與 SB16 DMA8／IRQ7 選項（%t）不一致（版本 4 ⇔ 選項開啟；先用 SetSBDMA8／-sb-dma8 設成相同，docs/spec/444 §2.9）",
			s.Version, m.sb8 != nil)
	} else if is4 {
		sb8 = new(sbDMA8State)
		if err := dec.Decode(sb8); err != nil {
			return fmt.Errorf("machine: 讀不開狀態檔的 sbDMA8State：%w", err)
		}
		if sb8.V != sbDMA8StateV {
			return fmt.Errorf("machine: sbDMA8State 版本 %d 不認得（docs/spec/444 §2.9）", sb8.V)
		}
		if err := sb8Validate(sb8); err != nil { // 445 §2.10
			return err
		}
	}
	if len(s.Mem) != len(m.Mem) {
		return fmt.Errorf("machine: 狀態檔的記憶體是 %d bytes，這台是 %d", len(s.Mem), len(m.Mem))
	}
	// 430：Ext 長度與 EMS 旗標在載入後仍影響執行，必須在載入前就設好；
	// 比對放在任何寫入之前，失敗時機器維持原狀。
	if len(s.Ext) != len(m.Ext) {
		return fmt.Errorf("machine: 狀態檔的延伸記憶體是 %d KB，這台是 %d KB（先用 SetExtendedMemoryKB／-ext-mem-kb 設成相同）",
			len(s.Ext)/1024, len(m.Ext)/1024)
	}
	if s.EMSAbsent != m.emsAbsent {
		return fmt.Errorf("machine: 狀態檔的 EMM 呈現是 %t，這台是 %t（先用 SetEMSPresent／-no-ems 設成相同）",
			!s.EMSAbsent, !m.emsAbsent)
	}
	// 432：PM 與機器設定（profile 開關）要一致，且內容要通過驗證；同樣在任何寫入之前，
	// 失敗時機器維持原狀（docs/spec/432-linear-memory-and-paging §7、§10 第 6 項）。
	// profile 關閉時套用初值（433 §2.1：不用零值，避免 IDTR 界限、DR6 出現 0）。
	pm := pmFields{sysRegs: sysRegsInit()}
	if hasPM := len(s.PM) != 0; hasPM != m.pmProfile {
		return fmt.Errorf("machine: 狀態檔的保護模式 profile 是 %t，這台是 %t（先用 SetProtectedModeProfile 設成相同）",
			hasPM, m.pmProfile)
	} else if hasPM {
		var err error
		if pm, err = decodePM(s.PM); err != nil {
			return err
		}
		// 434 §2.1：版本 1、2 帶 PE=1、VM=0 的存檔只可能來自 433 的停擺狀態，不能可靠恢復。
		// 拒絕放在這裡，不在 decodePM（直接呼叫 decodePM 的測試不受影響）。
		if v := binary.LittleEndian.Uint16(s.PM); v < pmVersion3 && pm.cr0&cr0PE != 0 && pm.eflagsHi&eflagsHiVM == 0 {
			return fmt.Errorf("machine: PM 版本 %d 帶 PE=1、VM=0（433 的停擺狀態），不能可靠恢復（docs/spec/434 §2.1）", v)
		}
	}
	// 新檔的埠統計在 slice 裡，舊檔在 map 裡；統一倒進 map 再用。
	for _, pv := range s.PortList {
		if s.Ports == nil {
			s.Ports = map[uint16]uint8{}
		}
		s.Ports[pv.Port] = pv.Val
	}
	for _, pc := range s.PortInList {
		if s.PortsIn == nil {
			s.PortsIn = map[uint16]uint64{}
		}
		s.PortsIn[pc.Port] = pc.Count
	}
	if !s.DACReadKnown && s.PortsIn[0x3C9] != 0 {
		return fmt.Errorf("machine: 舊狀態檔已讀 VGA DAC，卻沒有讀取索引與分量相位")
	}
	copy(m.Mem, s.Mem)
	// ROM 不是狀態：舊版執行器存下來的那一段是 0，照這一版的 ROM 蓋回去。
	m.initROM()
	m.LegacyState = s.Version < 3
	m.hma = [HMASize]uint8{}
	copy(m.hma[:], s.HMA)
	if m.Ext != nil {
		copy(m.Ext, s.Ext)
	}
	m.kbdAwaitOutport = s.KbdAwaitOutport
	m.a20 = s.A20
	m.syncCodeFastPath()
	m.CPU.R, m.CPU.Seg, m.CPU.IP = s.R, s.Seg, s.IP
	m.CPU.Hi, m.CPU.FSeg, m.CPU.GSeg = s.Hi, s.FSeg, s.GSeg
	m.CPU.SetFlags(s.Flags)
	m.CPU.Halted = s.Halted
	m.CPU.Model = cpu.Model(s.Model)

	m.Steps, m.Ticks = s.Steps, s.Ticks
	m.portTicks, m.nextIRQ0, m.irq0Pending = s.PortTicks, s.NextIRQ0, s.IRQ0Pending
	m.IRQ0Every = s.IRQ0Every
	// 舊的狀態檔沒有這幾個欄位；讀到零值就退回開機預設，
	// 不要讓分頻變成 0（那會讓 IRQ0 間隔算成 0）。
	m.IRQ0Base, m.PITDiv = s.IRQ0Base, s.PITDiv
	m.IRQ0Pinned = s.IRQ0Pinned
	if m.IRQ0Base == 0 {
		m.IRQ0Base = DefaultIRQ0Every
	}
	if m.PITDiv == 0 {
		m.PITDiv = PITDefaultDivisor
	}
	m.pitAccess, m.pitPhase, m.pitLo = s.PITAccess, s.PITPhase, s.PITLo
	if m.pitAccess == 0 {
		m.pitAccess = 3
	}
	m.CPUHz, m.CPU.Cycles, m.nextIRQ0Cyc = s.CPUHz, s.Cycles, s.NextIRQ0Cyc
	m.CycleClock = s.CycleClock
	m.CPU.DOSBoxCost = s.DOSBoxCost
	if m.CPUHz == 0 {
		m.CPUHz = DefaultCPUHz
	}
	m.recalcIRQ0()
	// 存檔決定時脈（`docs/spec/242` §2.1）。舊存檔（ClockPercent 缺席）是 100% 的存檔：
	// 還原前機器若縮放過，間隔回 100% 預設；否則保留當下值（例如 probe 在載入前自訂的
	// KeyEvery），與既有行為相同。nextFrame／nextKey 維持當下值。
	if s.ClockPercent == 0 {
		if m.ClockPercent() != 100 {
			m.VGAFrameEvery, m.KeyEvery = DefaultVGAFrameEvery, DefaultKeyIRQEvery
		}
		m.clockPct = 0
	} else {
		m.VGAFrameEvery, m.nextFrame, m.KeyEvery, m.nextKey = s.VGAFrameEvery, s.NextFrame, s.KeyEvery, s.NextKey
		m.clockPct = s.ClockPercent
	}

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
	m.dacReadIndex, m.dacReadPhase = s.DACReadIndex, s.DACReadPhase
	m.dacMask = 0xff
	if s.DACMaskPresent {
		m.dacMask = s.DACMask
	}
	copy(m.VGA.Raw(), s.Planes)
	m.VGA.latch, m.VGA.seqIdx, m.VGA.seq = s.Latch, s.SeqIdx, s.Seq
	m.VGA.gcIdx, m.VGA.gc = s.GCIdx, s.GC
	m.VGA.ac, m.VGA.acIdx, m.VGA.acFlip = s.AC, s.ACIdx, s.ACFlip
	if s.HasCRTC { // 舊狀態檔沒有 CRTC：沿用目前的值，不要清成零
		m.VGA.crtc, m.VGA.crtcIdx, m.VGA.misc = s.CRTC, s.CRTCIdx, s.Misc
	}
	m.planarOn = s.PlanarOn
	m.ProgramPath = s.ProgramPath

	m.FreeSeg, m.ImageBase, m.ImageLen = s.FreeSeg, s.ImageBase, s.ImageLen
	m.programLoaded = true
	m.applyPM(pm) // TLB 世代加一、重算衍生欄位與取指令快路徑
	if sb8 != nil {
		m.sb8Apply(sb8)
	}
	m.sb8Err = nil
	return nil
}
