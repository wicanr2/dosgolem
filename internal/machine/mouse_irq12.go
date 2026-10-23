package machine

import (
	"errors"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 保護模式剖面的 int 33h 使用者回呼經 IRQ12 遞送（docs/spec/459-mouse-irq12-v86-delivery）。
//
// 實模式剖面的滑鼠事件照舊以直接遠呼叫插入（callback.go 的 startCallback）。保護模式剖面（JEMM 這類 V86 監控
// 程式常駐）下，直接遠呼叫不經 PIC、IDT 與監控程式的中斷反射，監控程式在反射期間才換入的頁面（陸空戰將的線性
// A0000）回呼看不到（strike_commander RE 030）。這裡照 DOSBox-X：事件拉 IRQ12，經 IDT 向量 74h（V86）或 IVT
// （實模式）進 IVT[74h] 的 BIOS 樁，樁內 `int FBh` 由服務層推入使用者回呼的返回鏈。
//
//	Mouse74Off   ：sti; push ds; push es; pushad; int FBh; push ax; mov al,20h; out A0h,al; out 20h,al; pop ax; cld; retf
//	Mouse74RetOff：cli; mov al,20h; out A0h,al; out 20h,al; popad; pop es; pop ds; iret（+7 跳過 EOI）
//	MouseUIROff  ：cli; int FCh; retf

const (
	// IntMouse74 是 IRQ12 樁內服務層的陷入號（§3.6），IntMouseUIR 是使用者回呼返回樁的陷入號。
	// F2h–FAh 已用（callback.go、internal/dos 的 font.go、int9.go）。兩者都只在樁內的那一道指令上被服務。
	IntMouse74  = 0xFB
	IntMouseUIR = 0xFC

	// cbQueueMax 是保護模式剖面的佇列上限（§3.2），與 PM 版本 4 的編碼上限相同（§5.1）。
	cbQueueMax = 4096
)

var (
	mouse74Code    = [...]byte{0xFB, 0x1E, 0x06, 0x66, 0x60, 0xCD, IntMouse74, 0x50, 0xB0, 0x20, 0xE6, 0xA0, 0xE6, 0x20, 0x58, 0xFC, 0xCB}
	mouse74RetCode = [...]byte{0xFA, 0xB0, 0x20, 0xE6, 0xA0, 0xE6, 0x20, 0x66, 0x61, 0x07, 0x1F, 0xCF}
	mouseUIRCode   = [...]byte{0xFA, 0xCD, IntMouseUIR, 0xCB}
)

// Mouse74TrapIP、MouseUIRTrapIP 是樁內 `int FBh`、`int FCh` 那一道指令的位移（§3.6 的接手條件）。
const (
	Mouse74TrapIP  = Mouse74Off + 5
	MouseUIRTrapIP = MouseUIROff + 1
	// mouse74RetUser 是使用者回呼返回路徑的入口（跳過 cli 與兩道 EOI，DOSBox-X mouse.cpp:2277 的 +7）。
	mouse74RetUser = Mouse74RetOff + 7
)

// pmLoadInit 是保護模式剖面在 LoadEXE、LoadCOM 設 programLoaded 的那一點做的初始化（§2.2、§3.7）：
// BIOS 計時器樁換成帶 EOI 的 40 bytes 版本（§3.3a）、寫入三段 IRQ12 樁、IVT[74h] 指向 Mouse74Off、
// 從片 IMR bit 4 清 0（DOSBox-X Mouse_ResetHardware，mouse.cpp:1394-1396）。LoadState 不做（§5.3 另管）。
func (m *Machine) pmLoadInit() {
	m.WriteBytes(StubSeg*16+biosTimerOff, biosTimerCodeEOI[:])
	m.writeMouseStubs(func(a uint32, b []byte) { m.WriteBytes(a, b) })
	m.Write16(0x74*4, Mouse74Off)
	m.Write16(0x74*4+2, StubSeg)
	m.pic.slaveIMR &^= 0x10
}

// writeMouseStubs 以 put 寫入三段樁（實體位址 StubSeg*16+位移）。
func (m *Machine) writeMouseStubs(put func(a uint32, b []byte)) {
	base := uint32(StubSeg) * 16
	put(base+Mouse74Off, mouse74Code[:])
	put(base+Mouse74RetOff, mouse74RetCode[:])
	put(base+MouseUIROff, mouseUIRCode[:])
}

// mouseIRQ12 是 tick 的 IRQ12 遞送點（§3.4）：條件 1、1a、2、3、4 全部成立才遞送。
// 事件本身不在這裡取出，留到樁內的 int FBh（ServiceMouse74）。
func (m *Machine) mouseIRQ12() {
	if len(m.cbQueue) == 0 || m.pic.uir || m.pic.isr&7 != 0 || m.pic.isr4 { // 條件 1
		return
	}
	if m.CPU.IntHook == nil { // 條件 1a
		if m.irqErr == nil {
			m.irqErr = errors.New("machine: IRQ12 待處理，但這台機器沒有服務層（CPU.IntHook 為 nil）：IRQ12 樁需要服務層處理 int FBh，" +
				"失敗即關閉（docs/spec/459 §3.4 條件 1a）")
		}
		return
	}
	if !m.irq12Unmasked() || !m.CPU.Flag(cpu.IF) { // 條件 2、3
		return
	}
	if m.ivtGuardIRQ(0x74) { // 條件 4
		return
	}
	m.pic.isr4 = true
	m.picd.isrSetAt[3] = m.Steps
	m.setISR(2)
	m.stuckClear(stuckIRQ12)
	m.picd.irq12++
	m.eventThisStep = true // 444 §2.7 的第七個設置點
	m.raiseIRQ(0x74)
}

// overflowErr 是 §3.2 的佇列溢位錯誤。
func (m *Machine) overflowErr() error {
	return fmt.Errorf("machine: 保護模式剖面的滑鼠事件佇列已滿（%d 筆），被拒排入 %d 筆，失敗即關閉；IVT[74h]=%04X:%04X"+
		"（遊戲改掛 int 74h 而沒有鏈回 %04X:%04X 時佇列不會被消耗；docs/spec/459 §3.2）",
		len(m.cbQueue), m.picd.rejected, m.ReadSup16(0x74*4+2), m.ReadSup16(0x74*4), StubSeg, Mouse74Off)
}

// ServiceMouse74 是 IRQ12 樁內 `int FBh` 的機器端操作（§3.6，對應 DOSBox-X INT74_Handler，mouse.cpp:2254-2296）。
// 由服務層在本道指令為 StubSeg:Mouse74TrapIP 且剖面開啟時呼叫；handler 依遞送當下的登記決定使用者回呼位址，
// ok 為假表示登記已失效（停用或遮罩不含該事件）。
//
//  1. UIR 為 1 或佇列空：CS:IP 改為 StubSeg:Mouse74RetOff（不應發生）。
//  2. 取出佇列頭；登記有效：以目前堆疊推入 StubSeg、Mouse74RetOff+7、StubSeg、MouseUIROff、Handler 段、位移
//     （與 FarCallWithReturn 相同的推入），設 AX…DI，UIR=1、CallbacksMade 加 1，CS:IP 不動（繼續執行樁尾）。
//  3. 登記失效：事件丟棄、丟棄計數加 1，CS:IP 改為 StubSeg:Mouse74RetOff（在返回樁送 EOI）。
func (m *Machine) ServiceMouse74(handler func(q QueuedCall) (seg, off uint16, ok bool)) {
	c := m.CPU
	if m.pic.uir || len(m.cbQueue) == 0 {
		c.Seg[cpu.CS], c.IP = StubSeg, Mouse74RetOff
		return
	}
	q := m.cbQueue[0]
	m.cbQueue = m.cbQueue[1:]
	m.stuckClear(stuckIRQ12) // 佇列可能變空，tick 不再進入時不留舊的起點
	seg, off, ok := handler(q)
	if !ok {
		m.picd.discards++
		c.Seg[cpu.CS], c.IP = StubSeg, Mouse74RetOff
		return
	}
	cs, ip := c.Seg[cpu.CS], c.IP
	c.FarCallWithReturn(cs, ip, StubSeg, mouse74RetUser)
	c.FarCallWithReturn(cs, ip, StubSeg, MouseUIROff)
	c.FarCallWithReturn(cs, ip, seg, off)
	c.Seg[cpu.CS], c.IP = cs, ip
	c.R[cpu.AX], c.R[cpu.BX] = q.AX, q.BX
	c.R[cpu.CX], c.R[cpu.DX] = q.CX, q.DX
	c.R[cpu.SI], c.R[cpu.DI] = q.SI, q.DI
	m.pic.uir = true
	m.cbMade++
}

// ServiceMouseUIR 是使用者回呼返回樁內 `int FCh` 的機器端操作（§3.6，DOSBox-X UIR_Handler，mouse.cpp:2308-2311）：
// UIR 清 0。UIR 本來為 0 時回 false（服務層記一筆 Unimplemented，不吞掉）。
func (m *Machine) ServiceMouseUIR() bool {
	if !m.pic.uir {
		return false
	}
	m.pic.uir = false
	return true
}

// MouseIRQ12Reset 是保護模式剖面 `int 33h AX=0000h` 的附加動作（§3.7）：清從片 IMR bit 4、清空佇列（清掉的筆數
// 計入丟棄計數）、UIR 清 0。ISR4、ISR2 不動。剖面關閉時不動作。
func (m *Machine) MouseIRQ12Reset() {
	if !m.pmProfile {
		return
	}
	m.pic.slaveIMR &^= 0x10
	m.picd.discards += uint64(len(m.cbQueue))
	m.cbQueue = m.cbQueue[:0]
	m.pic.uir = false
	m.stuckClear(stuckIRQ12)
}

// MouseIRQ12 回 §3.8 的唯讀查詢：IRQ12 待處理、ISR4、ISR2、UIR、佇列筆數與丟棄計數。不進狀態摘要。
func (m *Machine) MouseIRQ12() (pending bool, isr4, isr2, uir bool, queued int, discards uint64) {
	return m.irq12Pending(), m.pic.isr4, m.pic.isr&4 != 0, m.pic.uir, len(m.cbQueue), m.picd.discards
}

// MouseIRQ12Discards 回丟棄計數（§3.8 的 RunMouseEvent 用）。
func (m *Machine) MouseIRQ12Discards() uint64 { return m.picd.discards }

// MouseIRQ12Active 回滑鼠事件是否走 IRQ12 路徑：保護模式剖面且沒有設 SetMouseIRQ12Direct。
func (m *Machine) MouseIRQ12Active() bool { return m.pmProfile && !m.picd.direct }

// SetMouseIRQ12Direct 是 §3.8 的診斷開關（§6.4 N1 的負對照）：只在保護模式剖面有作用，為真時滑鼠事件照實模式
// 剖面直接遠呼叫。只能在載入程式（LoadEXE、LoadCOM、LoadState）之前設定。不進狀態檔。
func (m *Machine) SetMouseIRQ12Direct(on bool) error {
	if m.programLoaded {
		return errors.New("machine: SetMouseIRQ12Direct 只能在載入程式或讀狀態檔之前設定（docs/spec/459 §3.8）")
	}
	m.picd.direct = on
	return nil
}

// MouseIRQ12Direct 回 SetMouseIRQ12Direct 的值（probe 報告標記「負對照，非正式結果」用）。
func (m *Machine) MouseIRQ12Direct() bool { return m.picd.direct }
