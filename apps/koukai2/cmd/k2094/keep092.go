package main

// 批次 092：戰後 20CCE 迴圈多艦隊（058 第十三節第 5 點，1FF62 的呼叫端）。
//   fgrp:組:艦隊:現役:水手:耐久:旗標
//      組 1：沿用現有敵方主組（DS:97E8），只改旗艦槽與船長。
//      組 2、3：新造一個只有一艘船的組（DS:97E0 + 8 * 組），艦隊 = 艦隊表（DS:2E76 + 133 * 艦隊）的第 n 筆，
//                 船記錄放在 DS:9968 + 0x18 * (組 - 2)：+0 = 船長人物（DS:12B6 + 50 * 艦隊 +0x2A）、+2 = 艦隊 +0x2B（第 0 槽），其餘清零；
//                 組記錄 +0、+2 = 船記錄、+4 = 艦隊、+6 = 組 1 的 +6。
//      旗艦槽 +8 的 0x30 位元：現役 1 = 0x10、0 = 清；槽 +0 = 水手（u16）；槽 +2 = 耐久；船長人物 +0x31 的 0x20：旗標 1 設起、0 清除。
//      全部是測試條件，記 poke 事件（含改前改後）。

import (
	"fmt"
	"strconv"
)

func (e *env) fgrp092(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	if len(f) < 7 {
		return fmt.Errorf("fgrp:組:艦隊:現役:水手:耐久:旗標")
	}
	var n [6]int
	for i := range n {
		v, err := strconv.Atoi(f[i+1])
		if err != nil {
			return err
		}
		n[i] = v
	}
	g, fl, act, sail, dur, flag := n[0], n[1], n[2], n[3], n[4], n[5]
	if g < 1 || g > 3 {
		return fmt.Errorf("組要 1..3")
	}
	ga := uint16(0x97E0 + 8*g)
	var fptr, rec uint16
	made := false
	if g == 1 {
		fptr = o.Word(b.DS(ga + 4))
		rec = o.Word(b.DS(ga))
		if rec == 0 {
			return fmt.Errorf("組 1 沒有旗艦")
		}
	} else {
		fptr = uint16(0x2E76 + 133*fl)
		rec = uint16(0x9968 + 0x18*(g-2))
		cap := o.Byte(b.DS(fptr + 0x2A))
		person := uint16(0x12B6 + 50*int(cap))
		for i := uint16(0); i < 0x18; i++ {
			o.SetByte(b.DS(rec+i), 0)
		}
		o.SetWord(b.DS(rec), person)
		o.SetWord(b.DS(rec+2), fptr+0x2B)
		w6 := o.Word(b.DS(0x97E8 + 6))
		o.SetWord(b.DS(ga), rec)
		o.SetWord(b.DS(ga+2), rec)
		o.SetWord(b.DS(ga+4), fptr)
		o.SetWord(b.DS(ga+6), w6)
		made = true
	}
	slot := o.Word(b.DS(rec + 2))
	person := o.Word(b.DS(rec))
	oldSlot := o.Bytes(b.DS(slot), 9)
	oldFlag := o.Byte(b.DS(person + 0x31))
	st := o.Byte(b.DS(slot+8)) & 0xCF
	if act != 0 {
		st |= 0x10
	}
	o.SetByte(b.DS(slot+8), st)
	o.SetWord(b.DS(slot), uint16(sail))
	o.SetByte(b.DS(slot+2), byte(dur))
	nf := oldFlag &^ 0x20
	if flag != 0 {
		nf |= 0x20
	}
	o.SetByte(b.DS(person+0x31), nf)
	e.log.Emit(Ev78{Kind: "poke", Label: fmt.Sprintf("fgrp%d", g), Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{
		"group": g, "made": made, "fleet_ptr": int(fptr), "rec": int(rec), "slot": int(slot), "person": int(person), "slot_old": fmt.Sprintf("%x", oldSlot),
		"state": int(st), "sailors": sail, "dur": dur, "flag_old": int(oldFlag), "flag_new": int(nf)}})
	return nil
}
