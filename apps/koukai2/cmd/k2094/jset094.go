package main

// 批次 094：2A1E0 的第 n 次進入（去重）改寫艦隊 F 與航點 p（絕對座標），可改 F.dest、任務與任務目標，用於抵達 p 之後的分支與底邊掃描。
//   jset:名稱=Fx;Fy;px;py;destx;desty;任務;任務目標;種子[+名稱=…]
//     任務、任務目標、種子用 -1 表示不改（種子 DS:BD88，注入當下寫入）。AX = p 的位址、BX = F 的位址（2AC19 的 lea ax,[bp+var_4]、mov bx,si）。全部是測試條件，每次記 poke 事件。
//     jonly 的視窗限制同 jcase（jonly:L:T 之後，視窗原點不是 (L, T) 的進入不記錄、不改）。
//   觀測：j_in（進入：F 133 bytes、p、視窗）、land（4135C 的 (x, y, 返回)）、pos_set（2A392）、arr（2A39A 之後：33164 回傳）、j_out（2A4A0 返回前）。
//   另外 2A3B0（rand(10)）、2A16D、43921、341C2、4407F 用 ev 掛。

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

type jset094 struct {
	name                                   string
	seed                                   int
	fx, fy, px, py, dx, dy, mission, tgt int
}

func (e *env) installJSet094(spec string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	var cases []jset094
	for _, c := range strings.Split(spec, "+") {
		name, v, ok := strings.Cut(c, "=")
		if !ok {
			return fmt.Errorf("jset 格式 名稱=Fx;Fy;px;py;destx;desty;任務;任務目標;種子")
		}
		p := strings.Split(v, ";")
		if len(p) != 9 {
			return fmt.Errorf("jset 要 9 個數字")
		}
		var n [9]int
		for i := range n {
			x, err := strconv.Atoi(p[i])
			if err != nil {
				return err
			}
			n[i] = x
		}
		cases = append(cases, jset094{name, n[8], n[0], n[1], n[2], n[3], n[4], n[5], n[6], n[7]})
	}
	d := &dedup{}
	count := 0
	active := false
	recording := false
	var fptr, pptr uint16
	var landX, landY uint16
	var landPending bool
	o.OnCall(b.Code(0x2A1E0), func(o *oracle.Oracle) {
		if d.again(o) {
			return
		}
		r := o.Regs()
		pptr, fptr = r.AX, r.BX
		L := 720*int(u8(o, b, 0x06CA)) + int(u16(o, b, 0xBDA6))
		T := int(u16(o, b, 0xBDAA))
		if jOnlyL >= 0 && (L != jOnlyL || T != jOnlyT) {
			return
		}
		name := ""
		if count < len(cases) {
			c := cases[count]
			name = c.name
			oldF := o.Bytes(b.DS(fptr), 8)
			oldP := o.Bytes(b.DS(pptr), 4)
			o.SetWord(b.DS(fptr), uint16(c.fx))
			o.SetWord(b.DS(fptr+2), uint16(c.fy))
			o.SetWord(b.DS(fptr+4), uint16(c.dx))
			o.SetWord(b.DS(fptr+6), uint16(c.dy))
			o.SetWord(b.DS(pptr), uint16(c.px))
			o.SetWord(b.DS(pptr+2), uint16(c.py))
			if c.mission >= 0 {
				o.SetByte(b.DS(fptr+0x1B), uint8(c.mission))
			}
			if c.tgt >= 0 {
				o.SetByte(b.DS(fptr+0x1C), uint8(c.tgt))
			}
			if c.seed >= 0 {
				o.SetBytes(b.DS(0xBD88), []byte{byte(c.seed), byte(c.seed >> 8), byte(c.seed >> 16), byte(c.seed >> 24)})
			}
			e.log.Emit(Ev78{Kind: "poke", Label: "jset:" + c.name, Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{
				"n": count, "F_old": hex.EncodeToString(oldF), "p_old": hex.EncodeToString(oldP),
				"F_new": []int{c.fx, c.fy}, "dest_new": []int{c.dx, c.dy}, "p_new": []int{c.px, c.py}, "mission": c.mission, "tgt": c.tgt, "seed": c.seed, "L": L, "T": T}})
		}
		x := map[string]any{"n": count, "case": name, "fptr": int(fptr), "pptr": int(pptr), "L": L, "T": T,
			"F": hx(o, b, fptr, 0x85), "p": hx(o, b, pptr, 4), "w06C8": int(u16(o, b, 0x06C8)), "w06CA": int(u8(o, b, 0x06CA)),
			"wBDA6": int(u16(o, b, 0xBDA6)), "wBDAA": int(u16(o, b, 0xBDAA)), "pf": int(u16(o, b, 0xC2D4)), "at_sea": int(u8(o, b, 0x0CAE))}
		e.log.Emit(Ev78{Kind: "b", Label: "j_in", IDA: "2A1E0", Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o), Extra: x})
		count++
		active, recording = true, true
	})
	dl := &dedup{}
	o.OnCall(b.Code(0x4135C), func(o *oracle.Oracle) {
		if !active || !recording || dl.again(o) {
			return
		}
		r := o.Regs()
		landX, landY, landPending = r.AX, r.DX, true
	})
	o.OnCall(b.Code(0x413B0), func(o *oracle.Oracle) {
		if !active || !recording || !landPending {
			return
		}
		landPending = false
		e.log.Emit(Ev78{Kind: "b", Label: "land", IDA: "4135C", Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"x": int(landX), "y": int(landY), "ret": int(o.Regs().AX)}})
	})
	o.OnCall(b.Code(0x2A392), func(o *oracle.Oracle) {
		if !active || !recording {
			return
		}
		recording = false
		e.log.Emit(Ev78{Kind: "b", Label: "pos_set", IDA: "2A392", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"F": hx(o, b, fptr, 0x85)}})
	})
	// 2A39A：33164（到達判定）返回後，AX = 是否到達
	o.OnCall(b.Code(0x2A39A), func(o *oracle.Oracle) {
		if !active {
			return
		}
		e.log.Emit(Ev78{Kind: "b", Label: "arr", IDA: "2A39A", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"ax": int(o.Regs().AX), "F": hx(o, b, fptr, 0x85)}})
	})
	o.OnCall(b.Code(0x2A48B), func(o *oracle.Oracle) {
		if !active {
			return
		}
		e.log.Emit(Ev78{Kind: "b", Label: "post_arr", IDA: "2A48B", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"F": hx(o, b, fptr, 0x85)}})
	})
	o.OnCall(b.Code(0x341C2), func(o *oracle.Oracle) {
		if !active {
			return
		}
		e.log.Emit(Ev78{Kind: "b", Label: "plan", IDA: "341C2", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"bx": int(o.Regs().BX)}})
	})
	o.OnCall(b.Code(0x2A4A0), func(o *oracle.Oracle) {
		if !active {
			return
		}
		active = false
		e.log.Emit(Ev78{Kind: "b", Label: "j_out", IDA: "2A4A0", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"F": hx(o, b, fptr, 0x85), "p": hx(o, b, pptr, 4)}})
	})
	return nil
}
