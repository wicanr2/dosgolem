package main

// 批次 086：海戰玩家移動（1A899）等的額外觀測。全部只觀測，不改被觀測程式的記憶體。

import (
	"encoding/hex"
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// farMem 讀 DS:ptrOff 的 far 指標（偏移在前、段在後）指向的 n bytes。
func farMem(o *oracle.Oracle, b Bind, ptrOff uint16, n int) string {
	off := o.Word(b.DS(ptrOff))
	seg := o.Word(b.DS(ptrOff + 2))
	return hex.EncodeToString(o.Bytes(oracle.Addr{Seg: seg, Off: off}, n))
}

func (e *env) act086(f []string) error {
	switch f[0] {
	case "int66":
		e.installInt66(f[1])
		return nil
	case "msghooks":
		e.installMsg()
		return nil
	case "sail":
		return e.sail(f)
	case "callr":
		return e.callr(f)
	case "tiphooks":
		e.installTip086()
		return nil
	case "movehooks":
		e.installMove086()
		return nil
	}
	return fmt.Errorf("不認得的動作")
}

// installMove086 掛玩家「移動」指令（1A899）的 hook。
func (e *env) installMove086() {
	b, _ := CurrentOrMain(e.o)
	frame := func(o *oracle.Oracle) string {
		r := o.Regs()
		// 1A899 的區域變數：BP-0x2C .. BP+0（var_22 = 各步分數，var_E = 各步剩餘移動點，var_20 = 下一步分數）
		return hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.SS, Off: r.BP - 0x2C}, 0x2C))
	}
	cur := func(o *oracle.Oracle) map[string]any {
		return map[string]any{
			"step": int(u8(o, b, 0x9BEE)), "pos": hx(o, b, 0x9BD0, 22), "dirs": hx(o, b, 0x9BE4, 12),
			"target": int(u16(o, b, 0x9BC4)), "path": hx(o, b, 0x9BC6, 10), "frame": frame(o),
		}
	}
	e.hookAt(0x1A899, "mv_enter", func(o *oracle.Oracle) map[string]any {
		x := cur(o)
		rec := u16(o, b, 0x9BC2)
		x["ship"] = shipSnap(o, b, rec)
		x["wind"] = hx(o, b, 0x9BF8, 2)
		x["wind_eff"] = hx(o, b, 0x9BF2, 6)
		x["occ"] = hx(o, b, 0x8E14, 2496)
		x["layer0"] = farMem(o, b, 0x97D4, 2496)
		return x
	})
	// 1A8D1：1D792 返回，AL = 移動點 M
	e.hookAt(0x1A8D1, "mv_M", func(o *oracle.Oracle) map[string]any {
		x := cur(o)
		x["M"] = int(o.Regs().AX & 0xFF)
		return x
	})
	// 1A8F7：1DC3C 返回（可達格已標好）
	e.hookAt(0x1A8F7, "mv_reach", func(o *oracle.Oracle) map[string]any {
		x := cur(o)
		x["layer0"] = farMem(o, b, 0x97D4, 2496)
		x["rec"] = hx(o, b, u16(o, b, 0x9BC2), 0x18)
		return x
	})
	// 1A8FB：1A6F2 返回，AX 非 0 才繼續選方向
	e.hookAt(0x1A8FB, "mv_6F2", func(o *oracle.Oracle) map[string]any {
		x := cur(o)
		x["ax"] = int(o.Regs().AX)
		return x
	})
	// 1A916：1BA28 返回（AL = 方向 0..5、0x80 = 退一步、0xFF = 取消）
	e.hookAt(0x1A916, "mv_dir", func(o *oracle.Oracle) map[string]any {
		x := cur(o)
		x["al"] = int(o.Regs().AX & 0xFF)
		x["cursor"] = int(u16(o, b, 0xA160))
		return x
	})
	// 1AA1C：1D6F6 返回，AL = 單步耗費；[BP-1] = 新方向
	e.hookAt(0x1AA1C, "mv_cost", func(o *oracle.Oracle) map[string]any {
		x := cur(o)
		r := o.Regs()
		x["cost"] = int(r.AX & 0xFF)
		x["newdir"] = int(o.Byte(oracle.Addr{Seg: r.SS, Off: r.BP - 1}))
		x["dl"] = int(r.DX & 0xFF)
		return x
	})
	// 1AA4E：離開選方向迴圈
	e.hookAt(0x1AA4E, "mv_done", func(o *oracle.Oracle) map[string]any { return cur(o) })
	// 1A2D5：執行路徑
	e.hookAt(0x1A2D5, "mv_exec", func(o *oracle.Oracle) map[string]any {
		x := cur(o)
		x["ship"] = shipSnap(o, b, u16(o, b, 0x9BC2))
		x["occ"] = hx(o, b, 0x8E14, 2496)
		return x
	})
	// 1AA91：1A2D5 返回
	e.hookAt(0x1AA91, "mv_exec_ret", func(o *oracle.Oracle) map[string]any {
		x := cur(o)
		x["ship"] = shipSnap(o, b, u16(o, b, 0x9BC2))
		x["occ"] = hx(o, b, 0x8E14, 2496)
		return x
	})
	// 1C611：前進一格（AX = 是否移動）
	e.hookAt(0x1C611, "mv_adv", func(o *oracle.Oracle) map[string]any {
		x := map[string]any{"ax": int(o.Regs().AX), "bx": int(o.Regs().BX)}
		x["rec"] = hx(o, b, u16(o, b, 0x9BC2), 0x18)
		return x
	})

	e.hookAt(0x1BCA4, "mv_confirm", func(o *oracle.Oracle) map[string]any { return cur(o) })
	e.hookAt(0x1A80A, "mv_post", func(o *oracle.Oracle) map[string]any { return cur(o) })
}

// installTip086 掛原地轉向（1AAAC）的 hook。
func (e *env) installTip086() {
	b, _ := CurrentOrMain(e.o)
	rec := func(o *oracle.Oracle) string { return hx(o, b, u16(o, b, 0x9BC2), 0x18) }
	e.hookAt(0x1AAAC, "tip_enter", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"rec": rec(o), "occ": hx(o, b, 0x8E14, 2496)}
	})
	e.hookAt(0x1AB92, "tip_reach", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"rec": rec(o), "layer0": farMem(o, b, 0x97D4, 2496)}
	})
	e.hookAt(0x1ABAA, "tip_dir", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"al": int(o.Regs().AX & 0xFF), "rec": rec(o)}
	})
	e.hookAt(0x1ABE9, "tip_ret", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"ax": int(o.Regs().AX), "rec": rec(o)}
	})
}
