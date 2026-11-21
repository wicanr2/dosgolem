package main

// 批次 078 項 4：結局接力（docs/spec/062、001）。觀測 MAIN.EXE 的 sub_2764F、KOUKAI2.COM 的 INT 65h 服務與 END.EXE 的參數解碼、播放入口。
// 只觀測，不改被觀測程式的記憶體。

import (
	"encoding/hex"
	"fmt"

	"github.com/wicanr2/dosgolem/oracle"
)

// endDS 是 END.EXE 的 dseg：IDA 線性 0x18DD0 + 偏移（docs/spec/062）。
func endDS(b Bind, off uint16) oracle.Addr {
	return oracle.Addr{Seg: b.Seg(uint16(0x18DD0 >> 4)), Off: off}
}

// installRelay 掛在 MAIN.EXE 執行期間：sub_2764F（結束處理）與 INT 65h 處理器（KOUKAI2.COM 的 102E7）。
func (e *env) installRelay() {
	o := e.o
	b, _ := CurrentOrMain(o)
	st := &HookStat78{}
	e.stats["main_2764F"] = st
	d := &dedup{}
	o.OnCall(b.Code(0x2764F), func(o *oracle.Oracle) {
		if d.again(o) {
			st.Dup++
			return
		}
		st.Total++
		st.Logged++
		e.log.Emit(Ev78{Kind: "relay", Label: "main_2764F", Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o),
			Extra: map[string]any{"AL": int(o.Regs().AX & 0xFF), "byte_49405": int(u8(o, b, 0x12B5)), "word_5441E": int(u16(o, b, 0xC2CE)),
				"byte_48DF4": int(u8(o, b, 0x0CA4)), "hero_nation_flags": int(u8(o, b, u16(o, b, 0xC2CE)+0x31)), "end_state_4853A": int(u8(o, b, 0x03EA))}})
	})
	// 27786..2779E：MAIN.EXE 以 AH=1、3、4、5 呼叫 INT 65h。在 INT 65h 處理器入口記錄，兩端都會經過。
	d2 := &dedup{}
	st2 := &HookStat78{}
	e.stats["int65"] = st2
	o.OnCall(o.IDA(0x102E7), func(o *oracle.Oracle) {
		if d2.again(o) {
			st2.Dup++
			return
		}
		st2.Total++
		st2.Logged++
		r := o.Regs()
		x := map[string]any{"AH": int(r.AX >> 8), "AL": int(r.AX & 0xFF), "BX": int(r.BX), "CX": int(r.CX), "DX": int(r.DX),
			"caller_ip": int(o.StackWord(0)), "caller_cs": int(o.StackWord(1)), "flags": int(o.StackWord(2)), "prog": func() string { c, _ := Current(o); return c.Name }()}
		if r.AX>>8 == 1 {
			// AH=1：CX:BX 指向 21 bytes（名、空白、姓）
			x["name21"] = hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.CX, Off: r.BX}, 21))
		}
		e.log.Emit(Ev78{Kind: "relay", Label: "int65", Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o), Extra: x})
	})
}

// installEnd 在 END.EXE 載入後掛 hook。位址是 END.EXE 的 IDA 線性位址（docs/spec/062）。
func (e *env) installEnd(psp uint16) {
	o := e.o
	b, ok := Current(o)
	if psp != 0 {
		b, ok = Bind{Name: "END.EXE", PSP: psp, Running: true}, true
	}
	if !ok || b.Name != "END.EXE" {
		fmt.Println("installEnd：目前不是 END.EXE")
		return
	}
	snap := func(o *oracle.Oracle) map[string]any {
		return map[string]any{
			"word_1AD92": int(o.Word(endDS(b, 0x1FC2))), "word_1ADA4": int(o.Word(endDS(b, 0x1FD4))),
			"word_18E12": int(o.Word(endDS(b, 0x0042))), "byte_18E15": int(o.Byte(endDS(b, 0x0045))),
			"name_1FC6": hex.EncodeToString(o.Bytes(endDS(b, 0x1FC6), 12)), "name_202E": hex.EncodeToString(o.Bytes(endDS(b, 0x202E), 12)),
		}
	}
	points := []struct {
		ida   uint32
		label string
	}{
		{0x10206, "end_params"},   // INT 65h AX=0301h、0401h 讀 v
		{0x1025D, "end_decode"},   // 解碼開始
		{0x102FB, "end_verify"},   // 驗證
		{0x1030E, "end_reject"},   // 驗證失敗
		{0x10329, "end_accept"},   // 驗證通過，切名姓
		{0x103CC, "end_hibyte"},   // 結局碼高位元組處理
		{0x103EF, "end_c5"},       // 結局碼 5 設 byte_18E15 = 0x78
		{0x103F8, "end_open_grp"}, // 開 ENDGRP.DAT
		{0x10998, "end_body"},     // 結局本體
		{0x109AB, "end_part1"},    // 第一部分：對白
		{0x109C5, "end_part2"},    // 第二部分：回憶
		{0x10A14, "end_part3"},    // 第三部分：片尾
		{0x10F2B, "end_wait_key"}, // 結束前等任一鍵
	}
	for _, p := range points {
		p := p
		d := &dedup{}
		st := &HookStat78{}
		e.stats[p.label] = st
		o.OnCall(b.Code(p.ida), func(o *oracle.Oracle) {
			if d.again(o) {
				st.Dup++
				return
			}
			st.Total++
			st.Logged++
			x := snap(o)
			r := o.Regs()
			x["AX"], x["BX"], x["CX"], x["DX"] = int(r.AX), int(r.BX), int(r.CX), int(r.DX)
			e.log.Emit(Ev78{Kind: "relay", Label: p.label, IDA: fmt.Sprintf("%05X", p.ida), Step: o.Steps(), Tick: o.Ticks(), Extra: x})
		})
	}
}
