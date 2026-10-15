package machine

import (
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// 實模式→保護模式交棒（docs/spec/433-real-mode-system-instructions §3.2、§5）。
//
// 既有核心執行 mov cr0／lmsw 使 CR0.PE 由 0 變 1 時，CPU.Step() 完成該道指令後回 cpu.ErrModeSwitch。
// Machine.Step 只在 CPU.Step() 回錯誤（或有分頁紀錄）時才進 finishStep，正常步沒有額外成本。

// ErrPMCoreNotAvailable 是交棒失敗而停擺時的錯誤（errors.Is 可判）。停擺之後每次 Step 都回同一個值，
// 不讓既有核心在 PE=1 下繼續執行。
var ErrPMCoreNotAvailable = errors.New("machine: 保護模式核心不可用（docs/spec/433 §5；cpu386 分段模式在 434）")

// Handoff 是交棒時既有核心的狀態（cpu386 的順序）。
type Handoff struct {
	R      [8]uint32 // EAX…EDI（cpu386 的順序，與既有核心的 AX…DI 編碼順序相同）
	EIP    uint32
	EFlags uint32
	Seg    [6]uint16 // cpu386 順序：CS, DS, ES, FS, GS, SS
	Cache  [6]SegCache
}

// PMCore 是保護模式核心（434 填入 cpu386 的轉接；測試以替身注入）。
type PMCore interface {
	Enter(h Handoff) error // 接手執行
	Step() error
}

// stepMode 是預先算好的模式值（§5）：Machine.Step 最前面只讀它一次，停擺判斷先於分派。
const (
	modeNormal uint8 = iota // 實模式／V86：既有核心照舊
	modeHalt                // 停擺：回 m.halt
	modePM                  // 最小分派：entered 且 PE=1、VM=0，改呼叫 m.pm.Step()
)

// recalcMode 在 PE、VM、m.pm、entered 或停擺狀態改變時更新 m.mode。
func (m *Machine) recalcMode() {
	switch {
	case m.halt != nil:
		m.mode = modeHalt
	case m.entered && m.pm != nil && m.cr0&cr0PE != 0 && m.eflagsHi&eflagsHiVM == 0:
		m.mode = modePM
	default:
		m.mode = modeNormal
	}
}

// setPMCore 裝上（或以 nil 拿掉）保護模式核心。434 之前只供測試注入替身。
func (m *Machine) setPMCore(p PMCore) {
	m.pm = p
	m.recalcMode()
}

// setHalt 設停擺（err 一律包住 ErrPMCoreNotAvailable）。
func (m *Machine) setHalt(err error) {
	m.halt = err
	m.recalcMode()
}

// rejudgeHalt 是 Restore、LoadState 的處置（§5 最後一項）：清除 entered；停擺不單純清除，而是重新判定。
// Sys 非 nil、PE=1、VM=0 時設起（m.pm 非 nil 也設起：cpu386 的狀態不在 Snapshot 與狀態檔裡，
// 434 完成前不得以舊的 cpu386 狀態繼續執行），否則清除。
func (m *Machine) rejudgeHalt() {
	m.entered = false
	m.halt = nil
	if m.CPU != nil && m.CPU.Sys != nil && m.cr0&cr0PE != 0 && m.eflagsHi&eflagsHiVM == 0 {
		m.halt = fmt.Errorf("%w：還原的狀態在保護模式（PE=1、VM=0），沒有可接手的保護模式核心狀態", ErrPMCoreNotAvailable)
	}
	m.recalcMode()
}

// finishStep 是 Machine.Step 三個回傳點共用的收尾，只在 CPU.Step() 回錯誤或該步有分頁紀錄時呼叫。
// 順序（§5）：先依 432 §10 第 1 項處理分頁紀錄（回 *PageFaultError，哨兵不交棒，但若是哨兵仍設停擺）；
// 否則若是 ErrModeSwitch，組出 Handoff 交棒。
func (m *Machine) finishStep(err error) error {
	sentinel := errors.Is(err, cpu.ErrModeSwitch)
	if m.pfOK {
		if sentinel {
			m.setHalt(m.handoffHaltErr(nil))
		}
		return m.stepFault(err)
	}
	if !sentinel {
		return err
	}
	if m.pm == nil {
		m.setHalt(m.handoffHaltErr(nil))
		return m.halt
	}
	if e := m.pm.Enter(m.handoff()); e != nil {
		m.setHalt(m.handoffHaltErr(e))
		return m.halt
	}
	m.entered = true
	m.recalcMode()
	return nil
}

// handoffHaltErr 組交棒失敗的停擺錯誤：帶交棒指令的起點與下一道位址、CR0；Enter 的錯誤以 %w 一併帶上。
func (m *Machine) handoffHaltErr(enterErr error) error {
	ocs, oip := m.CPU.OpAddr()
	ncs, nip := m.CPU.Seg[cpu.CS], m.CPU.IP
	msg := fmt.Sprintf("交棒指令起點 %04X:%04X（線性 %05X）、下一道 %04X:%04X（線性 %05X）、CR0=%08X",
		ocs, oip, cpu.Linear(ocs, oip), ncs, nip, cpu.Linear(ncs, nip), m.cr0)
	if enterErr != nil {
		return fmt.Errorf("%w：%s；Enter 失敗：%w", ErrPMCoreNotAvailable, msg, enterErr)
	}
	return fmt.Errorf("%w：%s", ErrPMCoreNotAvailable, msg)
}

// handoff 由既有核心的狀態組出 Handoff。段快取設為實模式推得的值：基底＝段值×16、界限 FFFFh、
// CS 屬性 009Bh（present、DPL0、可執行可讀、accessed、16 位元）、其餘段 0093h（present、DPL0、可讀寫、accessed、16 位元）。
func (m *Machine) handoff() Handoff {
	c := m.CPU
	var h Handoff
	for i := range h.R {
		h.R[i] = uint32(c.Hi[i])<<16 | uint32(c.R[i])
	}
	h.EIP = uint32(c.IP)
	h.EFlags = uint32(m.eflagsHi)<<16 | uint32(c.Flags)
	h.Seg = segsToPM(c)
	for i, s := range h.Seg {
		h.Cache[i] = SegCache{Base: uint32(s) << 4, Limit: 0xFFFF, Attr: 0x0093}
	}
	h.Cache[cpu386.SegCS].Attr = 0x009B
	return h
}

// segsToPM／segsFromPM 是段索引的單一映射（431 §4、433 §3.2），以具名欄位逐一對應，不用迴圈：
// 既有核心 Seg[ES]、Seg[CS]、Seg[SS]、Seg[DS]、FSeg、GSeg 對 cpu386 的 SegES、SegCS、SegSS、SegDS、SegFS、SegGS。
// cpu.FS、cpu.GS 常數不是 Seg 的索引。
func segsToPM(c *cpu.CPU) [6]uint16 {
	var s [6]uint16
	s[cpu386.SegES] = c.Seg[cpu.ES]
	s[cpu386.SegCS] = c.Seg[cpu.CS]
	s[cpu386.SegSS] = c.Seg[cpu.SS]
	s[cpu386.SegDS] = c.Seg[cpu.DS]
	s[cpu386.SegFS] = c.FSeg
	s[cpu386.SegGS] = c.GSeg
	return s
}

func segsFromPM(s [6]uint16, c *cpu.CPU) {
	c.Seg[cpu.ES] = s[cpu386.SegES]
	c.Seg[cpu.CS] = s[cpu386.SegCS]
	c.Seg[cpu.SS] = s[cpu386.SegSS]
	c.Seg[cpu.DS] = s[cpu386.SegDS]
	c.FSeg = s[cpu386.SegFS]
	c.GSeg = s[cpu386.SegGS]
}
