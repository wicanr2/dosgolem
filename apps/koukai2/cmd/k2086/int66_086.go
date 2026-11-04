package main

// 批次 086 項 7：INT 66h（FMDRV.COM 常駐驅動）的呼叫紀錄，自 k2082 搬來。掛在 IVT 向量目標，記 AH、AL 與呼叫者返回位址。
// 等待迴圈每約 20 步以 AH = F1h 輪詢一次，AH = F1h 只記前 3 筆。

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

func (e *env) installInt66(label string) {
	o := e.o
	b, _ := CurrentOrMain(o)
	vec := oracle.Addr{Seg: o.Word(oracle.Addr{Seg: 0, Off: 0x19A}), Off: o.Word(oracle.Addr{Seg: 0, Off: 0x198})}
	e.log.Emit(Ev78{Kind: "note", Label: "int66_vector", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"seg": int(vec.Seg), "off": int(vec.Off), "linear": int(vec.Linear())}})
	d := &dedup{}
	counts := map[int]int{}
	o.OnCall(vec, func(o *oracle.Oracle) {
		if d.again(o) {
			return
		}
		r := o.Regs()
		ah := int(r.AX >> 8)
		counts[ah]++
		if ah == 0xF1 && counts[ah] > 3 {
			return
		}
		ret := oracle.Addr{Seg: o.StackWord(1), Off: o.StackWord(0)}
		e.log.Emit(Ev78{Kind: "int66", Label: label, Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"ah": ah, "al": int(r.AX & 0xFF), "bx": int(r.BX), "dx": int(r.DX), "n": counts[ah],
				"caller_ida": fmt.Sprintf("%05X", b.ToIDA(ret)), "last_idx": int(u8(o, b, 0x8C1E)), "playing_flag": int(u8(o, b, 0x8C1F))}})
	})
}
