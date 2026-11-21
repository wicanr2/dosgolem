package main

// 批次 089：海戰敵方 AI 的目的地選擇（058 第十節）。全部只觀測。
//   aihooks   掛 1A238（分派入口）與各戰術函式、19EB3 的入口與 1A25F（分派返回）：
//             ai_in   1A238 入口：現行艦記錄 DS:9BC2、旗標、三層（DS:97D4 / 97D8 / 97DC 的 far 指標，各 52 x 48 = 2,496 bytes）、占用圖 DS:8E14、組記錄與艦記錄、分派表 DS:8C96
//             t_xxxxx 各戰術函式入口（暫存器 AX、DX = 19EB3 的目標座標）
//             ai_out  1A25F：word_51D14（DS:9BC4）= 選到的格（52 * y + x）

import (
	"github.com/wicanr2/dosgolem/oracle"
)

func (e *env) installAI089() {
	b, _ := CurrentOrMain(e.o)
	e.hookAt(0x1A238, "ai_in", func(o *oracle.Oracle) map[string]any {
		rec := u16(o, b, 0x9BC2)
		grp := u16(o, b, 0x9BC0)
		x := map[string]any{
			"rec_ptr": int(rec), "rec": hx(o, b, rec, 0x18), "grp_ptr": int(grp),
			"L0": farMem(o, b, 0x97D4, 2496), "L1": farMem(o, b, 0x97D8, 2496), "L2": farMem(o, b, 0x97DC, 2496),
			"occ": hx(o, b, 0x8E14, 2496), "groups": hx(o, b, 0x97E0, 32), "ships": hx(o, b, 0x9800, 0x3C0),
			"disp": hx(o, b, 0x8C96, 0x40), "wind": hx(o, b, 0x9BF8, 2),
		}
		if grp != 0 {
			x["grp"] = hx(o, b, grp, 8)
		}
		return x
	})
	for _, p := range []struct {
		ida   uint32
		label string
	}{{0x1A17E, "t1A17E"}, {0x1A145, "t1A145"}, {0x19EB3, "t19EB3"}, {0x1A008, "t1A008"}, {0x19FE7, "t19FE7"}, {0x19DC9, "t19DC9"}} {
		e.hookAt(p.ida, p.label, func(o *oracle.Oracle) map[string]any {
			return map[string]any{"ax": int(o.Regs().AX), "dx": int(o.Regs().DX)}
		})
	}
	e.hookAt(0x1A25F, "ai_out", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"target": int(u16(o, b, 0x9BC4))}
	})
}
