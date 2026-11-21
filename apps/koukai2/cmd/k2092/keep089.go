package main

// 批次 089：戰後 20CCE 迴圈的艦隊保留條件（058 第十三節第 5 點，1FF62 的呼叫端）。
//   fkeep:現役:水手:耐久:旗標   把組 1（敵方主組，DS:97E8）旗艦的船槽與船長改成指定值（測試條件，記 poke 事件）：
//                              現役 1 = 槽 +8 的 0x30 位元設成 0x10，0 = 清成 0；水手 = 槽 +0（u16）；耐久 = 槽 +2；旗標 1 = 船長人物 +0x31 的 0x20 設起，0 = 清除。

import (
	"fmt"
	"strconv"

	"github.com/wicanr2/dosgolem/oracle"
)

func (e *env) fkeep089(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	if len(f) < 5 {
		return fmt.Errorf("fkeep:現役:水手:耐久:旗標")
	}
	act, _ := strconv.Atoi(f[1])
	sail, _ := strconv.Atoi(f[2])
	dur, _ := strconv.Atoi(f[3])
	flag, _ := strconv.Atoi(f[4])
	rec := o.Word(b.DS(0x97E8))
	if rec == 0 {
		return fmt.Errorf("組 1 沒有旗艦")
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
	e.log.Emit(Ev78{Kind: "poke", Label: "fkeep", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{
		"rec": int(rec), "slot": int(slot), "person": int(person), "slot_old": fmt.Sprintf("%x", oldSlot),
		"state": int(st), "sailors": sail, "dur": dur, "flag_old": int(oldFlag), "flag_new": int(nf)}})
	return nil
}

var _ = oracle.Addr{}
