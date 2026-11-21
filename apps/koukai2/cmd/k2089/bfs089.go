package main

// 批次 089：NPC 視窗內的區域路徑（060 第 6.3、6.4 節：33872 加權廣度優先、33A97 縮成直線段、340A7 視窗內步驟目標）。全部只觀測。
//   bfshooks   33872 入口（A、B 兩個座標位址）、33885（成本陣列段值）、33A91（返回：AX 與整個 72 x 72 u16 成本陣列 10,368 bytes）；
//              33A97 入口（P、Q、turn）與 33D75（返回：AX 與 P 的新值）；340A7 入口（F 的 133 bytes）。

import (
	"encoding/hex"

	"github.com/wicanr2/dosgolem/oracle"
)

func (e *env) installBFS089() {
	o := e.o
	b, _ := CurrentOrMain(o)
	var seg uint16
	var aptr, bptr uint16
	var ppP, ppQ uint16
	win := func(o *oracle.Oracle) map[string]any {
		return map[string]any{"L": 720*int(u8(o, b, 0x06CA)) + int(u16(o, b, 0xBDA6)), "T": int(u16(o, b, 0xBDAA))}
	}
	e.hookAt(0x33872, "bfs_in", func(o *oracle.Oracle) map[string]any {
		r := o.Regs()
		aptr, bptr = r.AX, r.BX
		x := win(o)
		x["A"] = hx(o, b, aptr, 4)
		x["B"] = hx(o, b, bptr, 4)
		return x
	})
	o.OnCall(b.Code(0x33885), func(o *oracle.Oracle) { seg = o.Regs().AX })
	e.hookAt(0x33A91, "bfs_out", func(o *oracle.Oracle) map[string]any {
		x := win(o)
		x["ret"] = int(o.Regs().AX)
		x["A"] = hx(o, b, aptr, 4)
		x["B"] = hx(o, b, bptr, 4)
		x["cost"] = hex.EncodeToString(o.Bytes(oracle.Addr{Seg: seg, Off: 0}, 72*72*2))
		return x
	})
	e.hookAt(0x33A97, "pp_in", func(o *oracle.Oracle) map[string]any {
		r := o.Regs()
		ppP, ppQ = r.AX, r.BX
		x := win(o)
		x["P"] = hx(o, b, ppP, 4)
		x["Q"] = hx(o, b, ppQ, 4)
		x["turn"] = int(r.DX)
		return x
	})
	e.hookAt(0x33D75, "pp_out", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"ret": int(o.Regs().AX), "P": hx(o, b, ppP, 4), "Q": hx(o, b, ppQ, 4)}
	})
	e.hookAt(0x340A7, "s340A7", func(o *oracle.Oracle) map[string]any {
		bx := o.Regs().BX
		x := win(o)
		x["fptr"] = int(bx)
		x["F"] = hx(o, b, bx, 0x85)
		return x
	})
}
