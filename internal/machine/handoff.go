package machine

import (
	"errors"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 實模式→分段模式（docs/spec/434-legacy-core-segmented-mode §3.1；取代 433 §3.2、§5 的交棒接收端）。
//
// 既有核心執行 mov cr0／lmsw 使 CR0.PE 由 0 變 1 時，CPU.Step() 完成該道指令後回 cpu.ErrModeSwitch
// （433 §5 的 PE 邊緣判定保留）。Machine.Step 只在 CPU.Step() 回錯誤（或有分頁紀錄）時才進 finishStep，
// 正常步沒有額外成本。同一顆核心接著以分段模式執行，不換核心（431 §1.1）。

// finishStep 是 Machine.Step 三個回傳點共用的收尾，只在 CPU.Step() 回錯誤或該步有分頁紀錄時呼叫。
//
// 收到 ErrModeSwitch 時先進分段模式，再看分頁紀錄：同一步有紀錄時回 *PageFaultError（此時已進分段模式），
// 否則回 nil。這取代 432 §10 第 1 項「哨兵不處理」對 ErrModeSwitch 的處置：CR0.PE 已是 1，
// 不進分段模式反而會以實模式語意執行保護模式碼（434 §3.1）。
func (m *Machine) finishStep(err error) error {
	if errors.Is(err, cpu.ErrModeSwitch) {
		if e := m.enterSegmented(); e != nil {
			return e
		}
		if m.pfOK {
			return m.stepFault(err)
		}
		return nil
	}
	if m.pfOK {
		return m.stepFault(err)
	}
	return err
}

// enterSegmented 讓既有核心進分段模式（434 §3.1 第 1 項）：六個快取設為實模式推得值（基底＝段值×16、
// 界限 FFFFh；CS 屬性 009Bh（present、DPL0、可執行可讀、accessed、16 位元），其餘 0093h（present、DPL0、
// 可讀寫、accessed、16 位元），同 433 §5），SetSegmented(true)，SetPMCPL(0)。
//
// 段值以具名欄位逐一取：FS、GS 不在 Seg 陣列裡（cpu.go 的段索引常數說明）。
func (m *Machine) enterSegmented() error {
	c := m.CPU
	vals := [6]uint16{}
	vals[cpu.ES], vals[cpu.CS], vals[cpu.SS], vals[cpu.DS] = c.Seg[cpu.ES], c.Seg[cpu.CS], c.Seg[cpu.SS], c.Seg[cpu.DS]
	vals[cpu.FS], vals[cpu.GS] = c.FSeg, c.GSeg
	for i, v := range vals {
		attr := uint16(0x0093)
		if i == cpu.CS {
			attr = 0x009B
		}
		c.SetSegCache(i, SegCache{Base: uint32(v) << 4, Limit: 0xFFFF, Attr: attr})
	}
	c.SetSegmented(true)
	m.resetSegWait()
	return m.SetPMCPL(0)
}
