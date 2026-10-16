package main

import (
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// pcGuard 是「程式碼不該跑進 A0000 以上」的護欄（docs/spec/438）。
//
// 沒有分頁時那段是視訊記憶體與 BIOS，跑進去代表飛掉。開了分頁之後線性位址可能對到
// 一般 RAM（JEMM 把線性 B0000–B7FFF 映到 XMS），所以改看實體位址。轉換關閉時
// PCPhys() 就是 PCLinear()，行為與原本相同。頁不存在時不攔：下一步取指令會留下
// 分頁紀錄，Step 以 *PageFaultError 回報。
func pcGuard(m *machine.Machine) error {
	p, ok := m.PCPhys()
	if !ok || p < machine.VideoSeg*16 || p >= machine.MemSize {
		return nil
	}
	if lin := m.PCLinear(); lin != p {
		return fmt.Errorf("跑出可用記憶體：CS:IP ＝ %04X:%04X（線性 %05X，實體 %05X）",
			m.CPU.Seg[cpu.CS], m.CPU.IP, lin, p)
	}
	return fmt.Errorf("跑出可用記憶體：CS:IP ＝ %04X:%04X（線性 %05X）",
		m.CPU.Seg[cpu.CS], m.CPU.IP, p)
}
