package main

// 批次 078 項 2、3 的戰鬥觀測：海戰（058）、白刃與決鬥（059）。
// 只觀測，不改被觀測程式的記憶體。每個 hook 去重（中斷返回造成的重複觸發，dosgolem 規格 443 第 4 節）。

import (
	"encoding/hex"
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

func u16(o *oracle.Oracle, b Bind, off uint16) uint16 { return o.Word(b.DS(off)) }
func u8(o *oracle.Oracle, b Bind, off uint16) uint8  { return o.Byte(b.DS(off)) }

func hx(o *oracle.Oracle, b Bind, off uint16, n int) string {
	return hex.EncodeToString(o.Bytes(b.DS(off), n))
}

// shipSnap 取一艘艦的艦記錄（24 bytes）、船槽（9 bytes）、船長人物記錄（50 bytes）與兩個指標。
func shipSnap(o *oracle.Oracle, b Bind, rec uint16) map[string]any {
	if rec == 0 {
		return map[string]any{"rec_ptr": 0}
	}
	slot := u16(o, b, rec+2)
	person := u16(o, b, rec)
	m := map[string]any{
		"rec_ptr": int(rec), "rec": hx(o, b, rec, 0x18),
		"slot_ptr": int(slot), "person_ptr": int(person),
	}
	if slot != 0 {
		m["slot"] = hx(o, b, slot, 9)
	}
	if person != 0 {
		m["person"] = hx(o, b, person, 50)
	}
	return m
}

type dedup struct {
	prev   oracle.Regs
	ticks  uint64
	irq1   uint64
	step   uint64
	hasPrv bool
}

// again 回傳這次觸發是否是中斷返回造成的重複（暫存器與 SP 全同，且兩次之間有 IRQ 被送達）。
func (d *dedup) again(o *oracle.Oracle) bool {
	r := o.Regs()
	cur := dedup{prev: r, ticks: o.Ticks(), irq1: o.IRQ1Delivered(), step: o.Steps(), hasPrv: true}
	dup := d.hasPrv && d.prev == r && cur.step-d.step < Dup78Window && (d.ticks != cur.ticks || d.irq1 != cur.irq1)
	*d = cur
	return dup
}

// installBattle 掛海戰與白刃的 hook。事件標籤見各 emit。
func (e *env) installBattle() {
	o := e.o
	b, _ := CurrentOrMain(o)
	hook := func(ida uint32, label string, fn func(o *oracle.Oracle) map[string]any) {
		d := &dedup{}
		st := &HookStat78{}
		e.stats[label] = st
		o.OnCall(b.Code(ida), func(o *oracle.Oracle) {
			if d.again(o) {
				st.Dup++
				return
			}
			st.Total++
			st.Logged++
			x := fn(o)
			e.log.Emit(Ev78{Kind: "b", Label: label, IDA: fmt.Sprintf("%05X", ida), Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o), Extra: x})
		})
	}
	ssWord := func(o *oracle.Oracle, off uint16) uint16 {
		r := o.Regs()
		return o.Word(oracle.Addr{Seg: r.SS, Off: r.BP + off})
	}
	ssByte := func(o *oracle.Oracle, off uint16) uint8 {
		r := o.Regs()
		return o.Byte(oracle.Addr{Seg: r.SS, Off: r.BP + off})
	}

	// 20F7E：開戰入口。堆疊第 3 個字起是 [堆疊] 參數（守方援軍）。
	hook(0x20F7E, "start", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"stk": []int{int(o.StackWord(0)), int(o.StackWord(1)), int(o.StackWord(2))}}
	})
	// 1FE9A：回合迴圈入口。傾印戰場狀態另用 dump 動作。
	hook(0x1FE9A, "loop_start", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"slot": int(u8(o, b, 0x0535)), "result": int(u8(o, b, 0x9BFB)), "wind": hx(o, b, 0x9BF8, 2), "wind_eff": hx(o, b, 0x9BF2, 6),
			"groups": hx(o, b, 0x97E0, 32)}
	})
	// 1FDB8：每一次「某組某艦行動」的入口。
	hook(0x1FDB8, "turn", func(o *oracle.Oracle) map[string]any {
		rec := u16(o, b, 0x9BC2)
		x := map[string]any{"grp_ptr": int(u16(o, b, 0x9BC0)), "rec_ptr": int(rec), "slot": int(u8(o, b, 0x0535)), "result": int(u8(o, b, 0x9BFB)), "end_state": int(u8(o, b, 0x03EA))}
		if rec != 0 {
			x["rec"] = hx(o, b, rec, 0x18)
			sp := u16(o, b, rec+2)
			if sp != 0 {
				x["slot_bytes"] = hx(o, b, sp, 9)
			}
		}
		return x
	})
	// 1A3CE：非玩家艦的回合。1B8B1：玩家艦的回合（選單 50）。
	hook(0x1A3CE, "npc_turn", func(o *oracle.Oracle) map[string]any {
		rec := u16(o, b, 0x9BC2)
		return map[string]any{"rec_ptr": int(rec), "ship": shipSnap(o, b, rec), "flags": int(u8(o, b, rec+0x0C))}
	})
	hook(0x1B8B1, "player_turn", func(o *oracle.Oracle) map[string]any {
		rec := u16(o, b, 0x9BC2)
		return map[string]any{"rec_ptr": int(rec), "ship": shipSnap(o, b, rec)}
	})
	// 1E8CC：一次砲擊。BX = 目標艦記錄。
	hook(0x1E8CC, "fire", func(o *oracle.Oracle) map[string]any {
		bx := o.Regs().BX
		att := u16(o, b, 0x9BC2)
		return map[string]any{"att": shipSnap(o, b, att), "tgt": shipSnap(o, b, bx), "grp": int(u16(o, b, 0x9BC0)),
			"fire_vals": hx(o, b, att+0x10, 5), "cannon_left": int(u16(o, b, att+0x0E))}
	})
	// 1E735 入口：傷害套用前。BX = 目標艦記錄。
	hook(0x1E735, "hit_pre", func(o *oracle.Oracle) map[string]any {
		bx := o.Regs().BX
		att := u16(o, b, 0x9BC2)
		return map[string]any{"att": shipSnap(o, b, att), "tgt": shipSnap(o, b, bx), "wind": hx(o, b, 0x9BF8, 2)}
	})
	// 1E75A：1D9DE 返回，AX = 單發傷害 D。[BP-2] = 距離，[BP-4] = 目標艦記錄。
	hook(0x1E75A, "hit_D", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"D": int(o.Regs().AX), "dist": int(ssByte(o, 0xFFFE)), "tgt_ptr": int(ssWord(o, 0xFFFC))}
	})
	// 1E8D5：1E735 返回、動畫之前。目標狀態 = 傷害套用後。
	hook(0x1E8D5, "hit_post", func(o *oracle.Oracle) map[string]any {
		tgt := ssWord(o, 0xFFFE)
		return map[string]any{"tgt": shipSnap(o, b, tgt)}
	})
	// 1E8DA：動畫之前；1E8E0：1CDCD 返回。用來界定動畫內的亂數。
	hook(0x1E8DA, "anim_start", func(o *oracle.Oracle) map[string]any { return map[string]any{} })
	hook(0x1E8DD, "anim_end", func(o *oracle.Oracle) map[string]any { return map[string]any{} })
	// 1E91F：白刃雙方損失入口（BX = 目標艦記錄 T，DS:9BC2 = 攻方 S）。1E9FE：損失套用後（SI = T）。
	hook(0x1E91F, "loss_pre", func(o *oracle.Oracle) map[string]any {
		bx := o.Regs().BX
		att := u16(o, b, 0x9BC2)
		return map[string]any{"att": shipSnap(o, b, att), "tgt": shipSnap(o, b, bx)}
	})
	hook(0x1E9FE, "loss_post", func(o *oracle.Oracle) map[string]any {
		att := u16(o, b, 0x9BC2)
		return map[string]any{"att": shipSnap(o, b, att), "tgt": shipSnap(o, b, o.Regs().SI)}
	})
	// 1E95E：白刃（相鄰攻擊）。1D95E：白刃單方計算。1E616：經驗。
	hook(0x1E95E, "melee", func(o *oracle.Oracle) map[string]any {
		bx := o.Regs().BX
		att := u16(o, b, 0x9BC2)
		return map[string]any{"att": shipSnap(o, b, att), "tgt": shipSnap(o, b, bx)}
	})
	hook(0x1E616, "exp", func(o *oracle.Oracle) map[string]any {
		r := o.Regs()
		return map[string]any{"person_ptr": int(r.BX), "person": hx(o, b, r.BX, 50), "n": int(r.AX)}
	})
	// ---- 決鬥（059）----
	duelState := func(o *oracle.Oracle) map[string]any {
		p0, p1 := u16(o, b, 0x9D44), u16(o, b, 0x9D46)
		return map[string]any{
			"p0_ptr": int(p0), "p1_ptr": int(p1), "p0": hx(o, b, p0, 50), "p1": hx(o, b, p1, 50),
			"player_side": int(u16(o, b, 0x9D48)), "side": int(u16(o, b, 0x9D4E)),
			"hist0": int(u8(o, b, 0x9D4A)), "hist1": int(u8(o, b, 0x9D4B)), "meter": int(u8(o, b, 0x9C2A)),
			"equip": hx(o, b, 0xA01E, 14), "last_opp": int(u16(o, b, 0x9C2C)),
		}
	}
	hook(0x22BBA, "duel_enter", func(o *oracle.Oracle) map[string]any {
		r := o.Regs()
		x := duelState(o)
		x["mode"] = int(r.AX & 0xFF)
		x["tgt"] = shipSnap(o, b, r.BX)
		x["att"] = shipSnap(o, b, u16(o, b, 0x9BC2))
		return x
	})
	hook(0x22998, "duel_start", func(o *oracle.Oracle) map[string]any { return duelState(o) })
	hook(0x2231F, "duel_dmg_pre", func(o *oracle.Oracle) map[string]any { return duelState(o) })
	hook(0x22A04, "duel_dmg", func(o *oracle.Oracle) map[string]any {
		r := o.Regs()
		return map[string]any{"dmg": int(r.AX), "round": int(r.SI), "meter": int(u8(o, b, 0x9C2A)), "side": int(u16(o, b, 0x9D4E))}
	})
	hook(0x22A12, "duel_apply_ret", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"ended": int(o.Regs().AX), "round": int(o.Regs().SI), "meter": int(u8(o, b, 0x9C2A)), "hist0": int(u8(o, b, 0x9D4A)), "hist1": int(u8(o, b, 0x9D4B))}
	})
	hook(0x22A93, "duel_exp", func(o *oracle.Oracle) map[string]any {
		r := o.Regs()
		x := duelState(o)
		x["mode"] = int(r.AX & 0xFF)
		x["hero"] = hx(o, b, u16(o, b, 0xC2CE), 50)
		return x
	})
	// 20BDF：戰後入口；20679：結果結算；20984：國家關係。
	hook(0x20BDF, "after_battle", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"result": int(u8(o, b, 0x9BFB)), "slot": int(u8(o, b, 0x0535))}
	})
	hook(0x20679, "settle", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"result": int(u8(o, b, 0x9BFB)), "stk": []int{int(o.StackWord(0)), int(o.StackWord(1)), int(o.StackWord(2))}, "AX": int(o.Regs().AX)}
	})
	hook(0x20984, "relations", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"result": int(u8(o, b, 0x9BFB))}
	})
	// 14DC4：rand_n，只記 n != 100（輸入輪詢每圈 rand_n(100)，見 051）。
	rd := &dedup{}
	rst := &HookStat78{}
	e.stats["rand"] = rst
	o.OnCall(b.Code(0x14DC4), func(o *oracle.Oracle) {
		if rd.again(o) {
			rst.Dup++
			return
		}
		rst.Total++
		n := o.Regs().AX
		ret := b.ToIDA(oracle.Addr{Seg: o.StackWord(1), Off: o.StackWord(0)})
		if n == 100 && ret != 0x22473 && ret != 0x22471 {
			// 輸入輪詢每圈 rand_n(100)；只記決鬥格擋判定 2246E（far call 5 bytes，返回位址 22473）。
			return
		}
		rst.Logged++
		seed := o.Bytes(b.DS(0xBD88), 4)
		e.log.Emit(Ev78{Kind: "rand", Label: "n", Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"n": int(n), "seed": hex.EncodeToString(seed), "ret_ip": int(o.StackWord(0)), "ret_cs": int(o.StackWord(1)), "ret_ida": int(ret)}})
	})
}

// installMini 掛酒店賭博（二十一點，docs/spec/064）的 hook。
func (e *env) installMini() {
	o := e.o
	b, _ := CurrentOrMain(o)
	gold := func(o *oracle.Oracle) int { return int(int32(uint32(u16(o, b, 0x12AE)) | uint32(u16(o, b, 0x12B0))<<16)) }
	hook := func(ida uint32, label string, fn func(o *oracle.Oracle) map[string]any) {
		d := &dedup{}
		st := &HookStat78{}
		e.stats[label] = st
		o.OnCall(b.Code(ida), func(o *oracle.Oracle) {
			if d.again(o) {
				st.Dup++
				return
			}
			st.Total++
			st.Logged++
			e.log.Emit(Ev78{Kind: "b", Label: label, IDA: fmt.Sprintf("%05X", ida), Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o), Extra: fn(o)})
		})
	}
	// 3E118：洗牌函式 3E0B6 把前 24 張寫到 [bp-0x38] 指向的緩衝之後（pop si 之前）。
	hook(0x3E118, "bj_deck", func(o *oracle.Oracle) map[string]any {
		r := o.Regs()
		ptr := o.Word(oracle.Addr{Seg: r.SS, Off: r.BP - 0x38})
		return map[string]any{"ptr": int(ptr), "deck": hx(o, b, ptr, 24)}
	})
	hook(0x3E725, "bj_round", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"gold": gold(o), "stk": []int{int(o.StackWord(0)), int(o.StackWord(1)), int(o.StackWord(2))}}
	})
	hook(0x3E427, "bj_settle", func(o *oracle.Oracle) map[string]any {
		s := []int{}
		for i := 0; i < 10; i++ {
			s = append(s, int(o.StackWord(i)))
		}
		return map[string]any{"gold": gold(o), "stk": s}
	})
	hook(0x3E278, "bj_bet", func(o *oracle.Oracle) map[string]any { return map[string]any{"gold": gold(o)} })
}
