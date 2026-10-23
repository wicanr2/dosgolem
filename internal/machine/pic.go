package machine

import (
	"fmt"
	"strings"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 8259 的共用埠模型與保護模式剖面的在服務模型（docs/spec/459-mouse-irq12-v86-delivery §3.3、§3.3a）。
//
// 444 的 PIC 埠模型（20h、A0h、A1h）原本只在 -sb-dma8 開啟時由 sbDMA8 接管；459 把它的狀態（從片 IMR、
// OCW3 讀取選擇）搬到 Machine，讓保護模式剖面不開 -sb-dma8 時也用同一份。IRR7、ISR7 仍在 sbDMA8（444 不變）。
//
// 只開 -sb-dma8（剖面關閉）時，埠的行為與 444 逐位元組相同；剖面開啟時另有：
//   - 主片 ISR0、ISR1、ISR2 與從片 ISR4（§3.3a、§3.6），非特定 EOI 清編號最小的在服務位元，
//     特定 EOI 60h–62h、67h（主片）與 64h（從片）；
//   - IRQ 依在服務位元閘控（IRQ0 > IRQ1 > IRQ12（級聯 IRQ2）> IRQ7）；
//   - ISR 卡住偵測：待處理但被在服務位元擋住超過 L 步即失敗即關閉；
//   - EOI 計數（診斷）。

// IRQ 卡住偵測的索引（stuck 陣列）。
const (
	stuckIRQ0 = iota
	stuckIRQ1
	stuckIRQ12
	stuckIRQ7
	stuckN
)

// 擋住 IRQ 的在服務位元（stuckBy 的位元）。
const (
	byISR0 = 1 << iota
	byISR1
	byISR2
	byISR4
	byISR7
)

// picState 是 459 新增的機器狀態（§3.3 的表、§5.1、§5.4）：進 PM 版本 4 與 Snapshot。
// 卡住偵測的計數（stuckCnt、stuckLast）只進 Snapshot，不進 PM（§5.4）。
type picState struct {
	slaveIMR uint8   // A1h（從片 IMR）；-sb-dma8 或剖面開啟時使用
	readISR  [2]bool // OCW3 讀取選擇：[0] 主片（20h）、[1] 從片（A0h）；false＝IRR
	isr      uint8   // 主片在服務位元：bit0 ISR0、bit1 ISR1、bit2 ISR2（ISR7 在 sbDMA8）；只在剖面開啟時設起
	isr4     bool    // 從片 ISR4（IRQ12）
	uir      bool    // 使用者回呼執行中（DOSBox-X mouse.in_UIR）
	upgraded bool    // §5.3 升級血統（PM 旗標 bit 5）
	overflow bool    // §3.2 佇列溢位（PM 旗標 bit 6，黏著）
	// §3.3a 卡住偵測：stuckLast[i] 是上一次評估 IRQ i 那一步的 Steps+1（0＝目前不在「待處理且被在服務位元擋住」的一段內），
	// stuckCnt[i] 是這一段已計入的步數。IF=0、IMR 遮住的步不計入也不歸零；兩次評估之間沒有評估的步
	// （分段模式）計入。
	stuckCnt, stuckLast [stuckN]uint64
}

// picDiag 是 459 的診斷計數：不進狀態檔、快照與摘要（§3.3a、§3.8）。
type picDiag struct {
	direct bool // SetMouseIRQ12Direct（§3.8，N1 的負對照開關）

	rejected  uint64 // 溢位時被拒排入的筆數
	irq12     uint64 // IRQ12 送達次數
	discards  uint64 // 事件丟棄次數（§3.6 第 3 步、§3.7）
	upgradeIV string // 升級時 IVT[74h] 已被改過的紀錄（§5.3 第 3b 步）
	// upgradeFrames 是升級時在 ring 0 堆疊上找到的 V86 iretd 框架（§5.3 第 3 步，§10 (a) 收據用）。
	upgradeFrames string

	eoiMaster   [5]uint64 // 主片非特定 EOI：清掉 ISR0、ISR1、ISR2、ISR7、空 EOI
	eoiSlave    [2]uint64 // 從片非特定 EOI：清掉 ISR4、空 EOI
	eoiSpecific uint64    // 特定 EOI（主、從片合計）

	stuckMax      [stuckN]uint64 // 各 IRQ 被擋住的最長連續步數
	stuckMaxBy    [stuckN]uint8  // 最長那一段期間擋住它的位元聯集
	stuckEpisodes [stuckN][5]uint64
	stuckBy       [stuckN]uint8 // 目前這一段的擋住位元聯集
	isrSetAt      [5]uint64     // 最後一次設起 ISR0、ISR1、ISR2、ISR4、ISR7 的步數

	irq7DelayedByISR0 uint64 // §7.1b：IRQ7 被 ISR0 延後的段數
	irq7DelaySteps    uint64 // §7.1b：上一項各段的步數合計
	irq0InIRQ7        uint64 // §7.1b：IRQ7 處理期間（ISR7=1）IRQ0 插入的次數
	irq0InIRQ12       uint64 // IRQ12 處理期間（ISR2、ISR4 或 UIR 為 1）IRQ0 插入的次數
	irq7InIRQ12       uint64 // 同上，IRQ7

	a0Writes, a1Writes uint64
}

// stuckLimit 是 §3.3a 的 L＝10 × IRQ0Every 步（IRQ0Every 為 0 時取 DefaultIRQ0Every）。
func (m *Machine) stuckLimit() uint64 {
	if m.IRQ0Every == 0 {
		return 10 * DefaultIRQ0Every
	}
	return 10 * m.IRQ0Every
}

// StuckLimit 回 §3.3a 的 L 與來源（收據用）。
func (m *Machine) StuckLimit() (l uint64, source string) {
	if m.IRQ0Every == 0 {
		return m.stuckLimit(), "10 × DefaultIRQ0Every（IRQ0Every=0）"
	}
	return m.stuckLimit(), fmt.Sprintf("10 × IRQ0Every（%d）", m.IRQ0Every)
}

// isr7 回主片 ISR7（sbDMA8 的 isr7；選項關閉時為假）。
func (m *Machine) isr7() bool { return m.sb8 != nil && m.sb8.isr7 }

// irq12Pending 是「從片 IRR4」：佇列非空且不在使用者回呼中（§3.3）。只在剖面開啟、未設直接遠呼叫時成立。
func (m *Machine) irq12Pending() bool {
	return m.MouseIRQ12Active() && len(m.cbQueue) > 0 && !m.pic.uir
}

// irq12Unmasked 是 §3.4 條件 2：從片 IMR bit 4 與主片 IMR bit 2 都為 0。
func (m *Machine) irq12Unmasked() bool { return m.pic.slaveIMR&0x10 == 0 && m.picMask&0x04 == 0 }

// irq12Ready 是 §3.4 條件 1、2（segWait 的「有待處理 IRQ」的 IRQ12 部分）。
func (m *Machine) irq12Ready() bool {
	return m.irq12Pending() && m.pic.isr&7 == 0 && !m.pic.isr4 && m.irq12Unmasked()
}

// ---- 埠 ---------------------------------------------------------------------

// picIn 是 20h、A0h、A1h 的讀取（444 §2.6，459 §3.3 修訂）。呼叫端：sb8In（-sb-dma8 開啟）與 In8（只開剖面）。
func (m *Machine) picIn(port uint16) uint8 {
	switch port {
	case 0x20:
		var v uint8
		if m.pic.readISR[0] {
			if m.pmProfile {
				v = m.pic.isr & 7
			}
			if m.isr7() {
				v |= 0x80
			}
			return v
		}
		if m.irq0Pending {
			v |= 0x01
		}
		if m.irq12Pending() && m.pic.slaveIMR&0x10 == 0 && !m.pic.isr4 {
			v |= 0x04 // 級聯：從片 IRQ12 待處理、未遮、ISR4 為 0（pic.cpp:141-185）
		}
		if s := m.sb8; s != nil && s.irr7 {
			v |= 0x80
		}
		return v
	case 0xA0:
		if !m.pmProfile {
			return 0 // 444 §2.6
		}
		if m.pic.readISR[1] {
			if m.pic.isr4 {
				return 0x10
			}
			return 0
		}
		if m.irq12Pending() {
			return 0x10
		}
		return 0
	case 0xA1:
		return m.pic.slaveIMR
	}
	return 0xFF
}

// picFail 是 PIC 埠的失敗即關閉：-sb-dma8 開啟時照 444 寫 sb8Fail；只開剖面時寫同一個黏著欄位 sb8Err
// （§3.3：欄位語意擴大為 PIC 與 SB 的黏著錯誤），訊息寫 PIC。
func (m *Machine) picFail(format string, a ...any) {
	if m.sb8 != nil {
		m.sb8Fail(format, a...)
		return
	}
	if m.sb8Err != nil {
		return
	}
	m.sb8Err = fmt.Errorf("machine: %04X:%04X PIC 失敗即關閉：%s（step=%d，docs/spec/459 §3.3）",
		m.insnCS, m.insnIP, fmt.Sprintf(format, a...), m.Steps)
}

// picOut 是 20h、A0h、A1h 的寫入（444 §2.6，459 §3.3 修訂）。
func (m *Machine) picOut(p uint16, v uint8) {
	if p == 0xA1 {
		m.pic.slaveIMR = v
		m.picd.a1Writes++
		return
	}
	i := 0
	if p == 0xA0 {
		i = 1
		m.picd.a0Writes++
	}
	switch {
	case v&0x10 != 0:
		m.picFail("PIC %02Xh 寫入 ICW1（%02Xh）：不支援重新程式化 PIC", p, v)
	case v&0x18 == 0x08: // OCW3
		if (i == 0 || m.pmProfile) && v&0x44 != 0 {
			m.picFail("PIC %02Xh 寫入 OCW3 %02Xh：poll 或特殊遮罩未建模", p, v)
			return
		}
		switch v & 3 {
		case 2:
			m.pic.readISR[i] = false
		case 3:
			m.pic.readISR[i] = true
		}
	default: // OCW2
		if !m.pmProfile {
			m.picOCW2Legacy(i, p, v)
			return
		}
		switch v & 0xE0 {
		case 0x00, 0x40:
		case 0x20: // 非特定 EOI
			if i == 0 {
				m.masterEOI()
			} else {
				m.slaveEOI()
			}
		case 0x60: // 特定 EOI
			m.picd.eoiSpecific++
			lvl := v & 7
			switch {
			case i == 0 && lvl <= 2:
				m.pic.isr &^= 1 << lvl
			case i == 0 && lvl == 7:
				if m.isr7() {
					m.sb8EOI()
				}
			case i == 1 && lvl == 4:
				m.pic.isr4 = false
			}
		default:
			m.picFail("PIC %02Xh 寫入 OCW2 %02Xh：旋轉或設定優先序未建模", p, v)
		}
	}
}

// picOCW2Legacy 是只開 -sb-dma8 時 444 §2.6 的 OCW2（主片的非特定 EOI 與 67h 清 ISR7；從片只接受不動作）。
func (m *Machine) picOCW2Legacy(i int, p uint16, v uint8) {
	switch v & 0xE0 {
	case 0x00, 0x40:
	case 0x20: // 非特定 EOI
		if i == 0 {
			m.sb8EOI()
		}
	case 0x60: // 特定 EOI
		if i == 0 && v&7 == 7 {
			m.sb8EOI()
		}
	default:
		m.picFail("PIC %02Xh 寫入 OCW2 %02Xh：旋轉或設定優先序未建模", p, v)
	}
}

// masterEOI 是保護模式剖面主片的非特定 EOI：清 ISR0、ISR1、ISR2、ISR7 中編號最小且為 1 的那一位
// （pic.cpp:274-281、65-80）；都為 0 時不動作，記為空 EOI。
func (m *Machine) masterEOI() {
	switch {
	case m.pic.isr&1 != 0:
		m.pic.isr &^= 1
		m.picd.eoiMaster[0]++
	case m.pic.isr&2 != 0:
		m.pic.isr &^= 2
		m.picd.eoiMaster[1]++
	case m.pic.isr&4 != 0:
		m.pic.isr &^= 4
		m.picd.eoiMaster[2]++
	case m.isr7():
		m.sb8EOI()
		m.picd.eoiMaster[3]++
	default:
		m.picd.eoiMaster[4]++
	}
}

// slaveEOI 是保護模式剖面從片的非特定 EOI：ISR4 為 1 時清除，否則記為空 EOI。
func (m *Machine) slaveEOI() {
	if m.pic.isr4 {
		m.pic.isr4 = false
		m.picd.eoiSlave[0]++
		return
	}
	m.picd.eoiSlave[1]++
}

// ---- §3.3a 在服務設定與卡住偵測 ------------------------------------------------

// setISR 設起主片在服務位元（bit 0、1、2）並記步數（診斷）。
func (m *Machine) setISR(bit uint8) {
	m.pic.isr |= 1 << bit
	m.picd.isrSetAt[bit] = m.Steps
}

// irq0Accepted 是保護模式剖面 IRQ0 送達時的在服務設定（tick 在 raiseIRQ(08h) 之前呼叫）。
func (m *Machine) irq0Accepted() {
	if m.isr7() {
		m.picd.irq0InIRQ7++
	}
	if m.pic.isr&4 != 0 || m.pic.isr4 || m.pic.uir {
		m.picd.irq0InIRQ12++
	}
	m.setISR(0)
	m.stuckClear(stuckIRQ0)
}

// irq1Accepted 是保護模式剖面 IRQ1 送達時的在服務設定（keyTick、timedKeyTick 在 raiseIRQ(09h) 之前呼叫）。
func (m *Machine) irq1Accepted() {
	m.setISR(1)
	m.stuckClear(stuckIRQ1)
}

// stuckClear 結束 IRQ i 目前的「被擋住」段（送達或待處理消失時呼叫）。
func (m *Machine) stuckClear(i int) {
	if m.pic.stuckLast[i] == 0 {
		return
	}
	m.stuckEnd(i)
}

func (m *Machine) stuckEnd(i int) {
	n := m.pic.stuckCnt[i]
	if i == stuckIRQ7 && m.picd.stuckBy[i]&byISR0 != 0 {
		m.picd.irq7DelayedByISR0++
		m.picd.irq7DelaySteps += n
	}
	for b := 0; b < 5; b++ {
		if m.picd.stuckBy[i]&(1<<b) != 0 {
			m.picd.stuckEpisodes[i][b]++
		}
	}
	m.pic.stuckCnt[i], m.pic.stuckLast[i] = 0, 0
	m.picd.stuckBy[i] = 0
}

// stuckUpdate 更新 IRQ i 的「待處理但被在服務位元擋住」連續步數（§3.3a）。by 是擋住它的位元（0＝沒有待處理或
// 未被在服務位元擋住：這一段結束、歸零）；counted 為假（IF=0 或 IMR 遮住）時這一步不計入、也不歸零。
// 兩次評估之間沒有評估的步（分段模式）計入。連續步數超過 L 時把錯誤存進 irqErr 並回 true。
func (m *Machine) stuckUpdate(i int, by uint8, counted bool) bool {
	if by == 0 {
		m.stuckClear(i)
		return false
	}
	s := m.Steps + 1 // 這一步的序號
	last := m.pic.stuckLast[i]
	m.pic.stuckLast[i] = s
	m.picd.stuckBy[i] |= by
	if !counted || last == s {
		return false
	}
	if last == 0 {
		m.pic.stuckCnt[i] = 1
	} else {
		m.pic.stuckCnt[i] += s - last
	}
	n := m.pic.stuckCnt[i]
	if n > m.picd.stuckMax[i] {
		m.picd.stuckMax[i], m.picd.stuckMaxBy[i] = n, m.picd.stuckBy[i]
	}
	if l := m.stuckLimit(); n > l {
		if m.irqErr == nil {
			m.irqErr = m.stuckErr(i, n, l)
		}
		return true
	}
	return false
}

var stuckNames = [stuckN]string{"IRQ0", "IRQ1", "IRQ12", "IRQ7"}

func byNames(by uint8) string {
	var s []string
	for b, n := range []string{"ISR0", "ISR1", "ISR2", "ISR4", "ISR7"} {
		if by&(1<<b) != 0 {
			s = append(s, n)
		}
	}
	return strings.Join(s, "+")
}

func (m *Machine) stuckErr(i int, n, l uint64) error {
	_, src := m.StuckLimit()
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	return fmt.Errorf("machine: %s 待處理但被在服務位元 %s 擋住連續 %d 步，超過上限 L=%d（%s），失敗即關閉："+
		"ISR0=%d ISR1=%d ISR2=%d ISR4=%d ISR7=%d；待處理 IRQ0=%t IRQ1=%t IRQ12=%t IRQ7=%t；"+
		"最後設起 ISR0@%d ISR1@%d ISR2@%d ISR4@%d ISR7@%d（step=%d，docs/spec/459 §3.3a）",
		stuckNames[i], byNames(m.picd.stuckBy[i]), n, l, src,
		b(m.pic.isr&1 != 0), b(m.pic.isr&2 != 0), b(m.pic.isr&4 != 0), b(m.pic.isr4), b(m.isr7()),
		m.irq0Pending, m.irq1Pending(), m.irq12Pending(), m.sb8 != nil && m.sb8.irr7,
		m.picd.isrSetAt[0], m.picd.isrSetAt[1], m.picd.isrSetAt[2], m.picd.isrSetAt[3], m.picd.isrSetAt[4], m.Steps)
}

// irq1Pending 是 IRQ1 待處理：到期的定時鍵，或佇列非空且已過節流間隔。
func (m *Machine) irq1Pending() bool {
	return m.timedKeyDue() || (len(m.keyQueue) > 0 && m.KeyEvery != 0 && m.Steps >= m.nextKey)
}

// irq0Stuck 更新 IRQ0 的卡住計數：待處理時擋住它的在服務位元是 ISR0；IF=0 或主片 bit 0 遮住的步不計入。
func (m *Machine) irq0Stuck() bool {
	var by uint8
	if m.irq0Pending {
		by = m.pic.isr & byISR0
	}
	return m.stuckUpdate(stuckIRQ0, by, m.CPU.Flag(cpu.IF) && m.picMask&0x01 == 0)
}

// irq1Stuck：IRQ1 要 ISR0、ISR1 都為 0。
func (m *Machine) irq1Stuck() bool {
	var by uint8
	if m.irq1Pending() {
		by = m.pic.isr & (byISR0 | byISR1)
	}
	return m.stuckUpdate(stuckIRQ1, by, m.CPU.Flag(cpu.IF) && m.picMask&0x02 == 0)
}

// irq12Stuck：IRQ12 要 ISR0、ISR1、ISR2、ISR4 都為 0。
func (m *Machine) irq12Stuck() bool {
	var by uint8
	if m.irq12Pending() {
		by = m.pic.isr & (byISR0 | byISR1 | byISR2)
		if m.pic.isr4 {
			by |= byISR4
		}
	}
	return m.stuckUpdate(stuckIRQ12, by, m.CPU.Flag(cpu.IF) && m.irq12Unmasked())
}

// irq7Stuck：IRQ7 要 ISR0、ISR1、ISR2、ISR7 都為 0。
func (m *Machine) irq7Stuck() bool {
	var by uint8
	if s := m.sb8; s != nil && s.irr7 {
		by = m.pic.isr & (byISR0 | byISR1 | byISR2)
		if s.isr7 {
			by |= byISR7
		}
	}
	return m.stuckUpdate(stuckIRQ7, by, m.CPU.Flag(cpu.IF) && m.picMask&0x80 == 0)
}

// irq1Gate 是 keyTick、timedKeyTick 在保護模式剖面的在服務閘控（§3.4）：更新 IRQ1 的卡住計數；
// 回 true 表示這一步不送 IRQ1（被 ISR0／ISR1 擋住，或卡住錯誤已存進 irqErr）。
func (m *Machine) irq1Gate() bool {
	if m.irq1Stuck() {
		return true
	}
	return m.pic.isr&3 != 0
}

// ---- 診斷與查詢 ---------------------------------------------------------------

// PICStats 是 §3.3a、§3.8 的診斷（probe 報告與收據用）。不進狀態檔。
type PICStats struct {
	ISR0, ISR1, ISR2, ISR4, ISR7, UIR bool
	SlaveIMR, MasterIMR               uint8
	ReadISR                           [2]bool
	Queued                            int
	IRQ12, CallbacksMade, Discards    uint64
	Rejected                          uint64
	Overflow, LegacyMouseUpgrade      bool
	UpgradeNote, UpgradeFrames        string
	Direct                            bool
	// EOIMaster：清掉 ISR0、ISR1、ISR2、ISR7、空 EOI；EOISlave：清掉 ISR4、空 EOI。
	EOIMaster   [5]uint64
	EOISlave    [2]uint64
	EOISpecific uint64
	// StuckMax[i]、StuckMaxBy[i]：IRQ0、IRQ1、IRQ12、IRQ7 被擋住的最長連續步數與擋住位元（byNames 的格式）。
	StuckMax      [4]uint64
	StuckMaxBy    [4]string
	StuckEpisodes [4][5]uint64
	L             uint64
	LSource       string

	IRQ7DelayedByISR0, IRQ7DelaySteps, IRQ0InIRQ7 uint64
	IRQ0InIRQ12, IRQ7InIRQ12                      uint64
	A0Reads, A1Reads, A0Writes, A1Writes          uint64
}

// PICStats 回 §3.3a 的診斷與目前的 PIC 狀態。
func (m *Machine) PICStats() PICStats {
	l, src := m.StuckLimit()
	st := PICStats{
		ISR0: m.pic.isr&1 != 0, ISR1: m.pic.isr&2 != 0, ISR2: m.pic.isr&4 != 0, ISR4: m.pic.isr4, ISR7: m.isr7(), UIR: m.pic.uir,
		SlaveIMR: m.pic.slaveIMR, MasterIMR: m.picMask, ReadISR: m.pic.readISR, Queued: len(m.cbQueue),
		IRQ12: m.picd.irq12, CallbacksMade: m.cbMade, Discards: m.picd.discards, Rejected: m.picd.rejected,
		Overflow: m.pic.overflow, LegacyMouseUpgrade: m.pic.upgraded, UpgradeNote: m.picd.upgradeIV,
		UpgradeFrames: m.picd.upgradeFrames, Direct: m.picd.direct,
		EOIMaster: m.picd.eoiMaster, EOISlave: m.picd.eoiSlave, EOISpecific: m.picd.eoiSpecific,
		StuckMax: m.picd.stuckMax, StuckEpisodes: m.picd.stuckEpisodes, L: l, LSource: src,
		IRQ7DelayedByISR0: m.picd.irq7DelayedByISR0, IRQ7DelaySteps: m.picd.irq7DelaySteps, IRQ0InIRQ7: m.picd.irq0InIRQ7,
		IRQ0InIRQ12: m.picd.irq0InIRQ12, IRQ7InIRQ12: m.picd.irq7InIRQ12,
		A0Reads: m.PortsIn[0xA0], A1Reads: m.PortsIn[0xA1], A0Writes: m.picd.a0Writes, A1Writes: m.picd.a1Writes,
	}
	for i := range st.StuckMaxBy {
		st.StuckMaxBy[i] = byNames(m.picd.stuckMaxBy[i])
	}
	return st
}

// LegacyMouseUpgrade 回 §5.1 旗標 bit 5（升級血統）：這台機器的保護模式狀態曾由 PM 版本 1–3 升級而來。
func (m *Machine) LegacyMouseUpgrade() bool { return m.pic.upgraded }
