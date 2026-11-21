package main

// 批次 082 項 5：INT 66h（FMDRV.COM 常駐驅動）的呼叫紀錄。掛在 IVT 向量目標，記 AH、AL 與呼叫者返回位址。
// 等待迴圈（IDA 116BA）每約 20 步就以 AH = F1h 輪詢一次，全記會產生每次數億位元組的事件檔，所以 AH = F1h 只記前 3 筆，
// 其餘 AH 全記，每筆帶該 AH 的累計次數 n。

import (
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

func (e *env) installInt66(label string) {
	o := e.o
	b, _ := CurrentOrMain(o)
	vec := oracle.Addr{Seg: o.Word(oracle.Addr{Seg: 0, Off: 0x19A}), Off: o.Word(oracle.Addr{Seg: 0, Off: 0x198})}
	e.log.Emit(Ev82{Kind: "note", Label: "int66_vector", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"seg": int(vec.Seg), "off": int(vec.Off), "linear": int(vec.Linear())}})
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
			if counts[ah]%1000000 == 0 {
				e.log.Emit(Ev82{Kind: "note", Label: "int66_f1_count", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"count": counts[ah]}})
			}
			return
		}
		ret := oracle.Addr{Seg: o.StackWord(1), Off: o.StackWord(0)}
		e.log.Emit(Ev82{Kind: "int66", Label: label, Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"ah": ah, "al": int(r.AX & 0xFF), "bx": int(r.BX), "dx": int(r.DX), "n": counts[ah],
				"caller_ida": fmt.Sprintf("%05X", b.ToIDA(ret)), "last_idx": int(u8(o, b, 0x8C1E)), "playing_flag": int(u8(o, b, 0x8C1F))}})
	})
}
