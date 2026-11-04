package main

// 批次 086 自 k2082 搬來的動作：sail（自然推進時間）、callr（保留暫存器的強制呼叫）、msg（訊息編號 hook）。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

func parseRegs(spec string) (oracle.CallRegs, error) {
	var r oracle.CallRegs
	if spec == "" || spec == "-" {
		return r, nil
	}
	for _, part := range strings.Split(spec, "/") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			return r, fmt.Errorf("暫存器格式 %q", part)
		}
		n, err := strconv.ParseUint(v, 16, 16)
		if err != nil {
			return r, err
		}
		switch strings.ToUpper(k) {
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
			return r, fmt.Errorf("不支援的暫存器 %q", k)
		}
	}
	return r, nil
}

// callr:段.偏移:標籤:暫存器[:參數…]：保留暫存器的強制呼叫（呼叫後寫回呼叫前的暫存器）。
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
	e.log.Emit(Ev78{Kind: "call", Label: f[2], IDA: f[1], Step: o.Steps(), Extra: map[string]any{"ret": v, "regs": f[3], "args": f[4:]}})
	return nil
}

// sail:天數[:每天補給]：以 2M 指令為一段讓遊戲自己跑，日期（DS:0532..0534）變一次算一天。補給格式：HEX（寫到 DS:52D4）或 偏移=HEX/偏移=HEX。
// 時段 DS:0535 連續 10 段沒變就送一個 Return（訊息窗在等鍵）並記一筆 unstick。
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
				_, _, px := o.Screen()
				e.log.Emit(Ev78{Kind: "note", Label: "unstick", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"day": hex.EncodeToString(cur), "scr": fmt.Sprintf("%x", sha256.Sum256(px))}})
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
				e.log.Emit(Ev78{Kind: "poke", Label: "refill", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": int(rf.off), "new": hex.EncodeToString(rf.data)}})
			}
			e.log.Emit(Ev78{Kind: "note", Label: "day", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"n": count, "date": hex.EncodeToString(cur)}})
		}
	}
	return nil
}

// installMsg 掛訊息函式 4377E（far，參數是堆疊上的訊息編號）。
func (e *env) installMsg() {
	b, _ := CurrentOrMain(e.o)
	e.hookAt(0x4377E, "msg", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"msg": int(o.StackWord(2)), "ret_ida": int(b.ToIDA(oracle.Addr{Seg: o.StackWord(1), Off: o.StackWord(0)}))}
	})
}
