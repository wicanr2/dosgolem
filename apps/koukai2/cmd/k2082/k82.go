package main

// 批次 082 的新增動作：rand_n 呼叫紀錄（rnd）、保留暫存器的強制呼叫（callr）。
// 只觀測，不改被觀測程式的記憶體（callr 與 poke 除外，都寫進事件檔）。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

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
	dup := d.hasPrv && d.prev == r && cur.step-d.step < Dup82Window && (d.ticks != cur.ticks || d.irq1 != cur.irq1)
	*d = cur
	return dup
}

func seed32(o *oracle.Oracle, b Bind) uint32 {
	return uint32(o.Word(b.DS(0xBD88))) | uint32(o.Word(b.DS(0xBD8A)))<<16
}

// installRnd 掛 sub_14DC4（rand_n）的入口與返回。每次呼叫記一筆 rnd 事件：
// n（AX）、ret（呼叫端返回位址的 IDA 線性位址）、res（回傳 AX）、s0、s1（呼叫前後的種子）。
// excl 內的返回位址不記（輸入輪詢每圈抽一次，量太大）。
func (e *env) installRnd(excl map[uint32]bool) {
	o := e.o
	b, _ := CurrentOrMain(o)
	var pend struct {
		ok   bool
		n    uint16
		ret  uint32
		seed uint32
	}
	d1, d2 := &dedup{}, &dedup{}
	skipped := 0
	e.rndSkipped = &skipped
	o.OnCall(b.Code(0x14DC4), func(o *oracle.Oracle) {
		if d1.again(o) {
			return
		}
		r := o.Regs()
		ret := oracle.Addr{Seg: o.StackWord(1), Off: o.StackWord(0)}
		pend.ok, pend.n, pend.ret, pend.seed = true, r.AX, b.ToIDA(ret), seed32(o, b)
	})
	o.OnCall(b.Code(0x14DDD), func(o *oracle.Oracle) {
		if d2.again(o) || !pend.ok {
			return
		}
		pend.ok = false
		if !e.rndOn {
			return
		}
		if excl[pend.ret] {
			skipped++
			return
		}
		e.log.Emit(Ev82{Kind: "rnd", Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"n": int(pend.n), "ret": fmt.Sprintf("%05X", pend.ret), "res": int(o.Regs().AX), "s0": int(pend.seed), "s1": int(seed32(o, b))}})
	})
}

// parseRegs 解析 "BX=2EFB/AX=0" 成 CallRegs。
func parseRegs(spec string) (oracle.CallRegs, error) {
	var r oracle.CallRegs
	if spec == "" || spec == "-" {
		return r, nil
	}
	for _, kv := range strings.Split(spec, "/") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return r, fmt.Errorf("暫存器格式 %q", kv)
		}
		n, err := strconv.ParseUint(v, 16, 16)
		if err != nil {
			return r, err
		}
		switch k {
		case "AX":
			r.AX, r.SetAX = uint16(n), true
		case "BX":
			r.BX, r.SetBX = uint16(n), true
		case "CX":
			r.CX, r.SetCX = uint16(n), true
		case "DX":
			r.DX, r.SetDX = uint16(n), true
		case "SI":
			r.SI, r.SetSI = uint16(n), true
		case "DI":
			r.DI, r.SetDI = uint16(n), true
		default:
			return r, fmt.Errorf("不支援的暫存器 %s", k)
		}
	}
	return r, nil
}

// callr：callr:段.偏移:標籤:暫存器設定:參數…
// 暫存器設定如 BX=2EFB/AX=0（"-" 表示不設）。呼叫前記下全部暫存器，呼叫後寫回，所以遊戲接著跑不受影響。
func (e *env) callr(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	seg, off, ok := strings.Cut(f[1], ".")
	if !ok {
		return fmt.Errorf("callr 要用 段.偏移")
	}
	sv, err := strconv.ParseUint(seg, 16, 32)
	if err != nil {
		return err
	}
	ov, err := strconv.ParseUint(off, 16, 32)
	if err != nil {
		return err
	}
	target := oracle.Addr{Seg: b.Seg(uint16(sv)), Off: uint16(ov)}
	regs, err := parseRegs(f[3])
	if err != nil {
		return err
	}
	var args []uint16
	for i := 4; i < len(f); i++ {
		v, err := strconv.ParseUint(f[i], 16, 16)
		if err != nil {
			return err
		}
		args = append(args, uint16(v))
	}
	saved := o.Regs()
	o.SetRegs(regs)
	v, err := o.CallBudget(20_000_000, target, args...)
	o.SetRegs(oracle.CallRegs{AX: saved.AX, BX: saved.BX, CX: saved.CX, DX: saved.DX, SI: saved.SI, DI: saved.DI, BP: saved.BP, DS: saved.DS, ES: saved.ES,
		SetAX: true, SetBX: true, SetCX: true, SetDX: true, SetSI: true, SetDI: true, SetBP: true, SetDS: true, SetES: true})
	if err != nil {
		return err
	}
	e.log.Emit(Ev82{Kind: "call", Label: f[2], IDA: f[1], Step: o.Steps(), Extra: map[string]any{"ret": v, "regs": f[3], "args": f[4:]}})
	return nil
}

// hexNote 供 note 類動作使用。
func hexNote(b []byte) string { return hex.EncodeToString(b) }

// sail：sail:天數[:每天補給。HEX（寫到 DS:52D4）或 偏移=HEX/偏移=HEX]。
// 以 2M 指令為一段讓遊戲自己跑，日期（DS:0532..0534）變一次算一天，每天補一次水糧（測試條件，寫進事件檔）。
// 時段（DS:0535）連續 10 段（20M 指令）沒變就送一個 Return 並記一筆 unstick（訊息窗在等鍵）。
func (e *env) sail(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	days, err := strconv.Atoi(f[1])
	if err != nil {
		return err
	}
	type refillItem struct {
		off  uint16
		data []byte
	}
	var refills []refillItem
	if len(f) > 2 && f[2] != "" && f[2] != "-" {
		// 格式：HEX（寫到 DS:52D4）或 偏移=HEX/偏移=HEX
		for _, part := range strings.Split(f[2], "/") {
			off := uint64(0x52D4)
			hx := part
			if o_, h_, ok := strings.Cut(part, "="); ok {
				off, err = strconv.ParseUint(o_, 16, 16)
				if err != nil {
					return err
				}
				hx = h_
			}
			d, err := hex.DecodeString(hx)
			if err != nil {
				return err
			}
			refills = append(refills, refillItem{uint16(off), d})
		}
	}
	start := o.Steps()
	prev := o.Bytes(b.DS(0x0532), 4)
	count, stuck := 0, 0
	for count < days {
		if o.Steps()-start > e.max {
			return fmt.Errorf("sail：預算用完，已過 %d 天", count)
		}
		if err := o.Run(2_000_000); err != nil {
			return err
		}
		cur := o.Bytes(b.DS(0x0532), 4)
		if string(cur) == string(prev) {
			stuck++
			if stuck >= 10 {
				stuck = 0
				if err := o.SendKeys("Return"); err != nil {
					return err
				}
				sh := o.Screen
				_, _, px := sh()
				e.log.Emit(Ev82{Kind: "note", Label: "unstick", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"day": hex.EncodeToString(cur), "cb3": int(o.Byte(b.DS(0x0CB3))), "scr": fmt.Sprintf("%x", sha256sum(px))}})
			}
			continue
		}
		stuck = 0
		newDay := string(cur[:3]) != string(prev[:3])
		prev = cur
		if newDay {
			count++
			for _, rf := range refills {
				o.SetBytes(b.DS(rf.off), rf.data)
				e.log.Emit(Ev82{Kind: "poke", Label: "refill", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": int(rf.off), "new": hex.EncodeToString(rf.data)}})
			}
			e.log.Emit(Ev82{Kind: "note", Label: "day", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"n": count, "date": hex.EncodeToString(cur), "cb3": int(o.Byte(b.DS(0x0CB3)))}})
		}
	}
	return nil
}

func sha256sum(b []byte) [32]byte { return sha256.Sum256(b) }
