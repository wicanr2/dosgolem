package main

// 批次 087：骰子（3FC60 一族）的觀測與條件按鍵。全部只觀測，不改被觀測程式的記憶體。
//   ifat:IDA:預算:鍵+鍵…   跑到 IDA（最多預算道指令）；到了才送鍵（每鍵之後跑 2M 道），沒到不送。用在「議價才出現的對話框」。
//   dicehooks              掛 3F471（擲骰回合）的 hook：每次在 3F492（初始化後）、3F4E5（擲一顆）、3F522（排序後）、3F5BD（問「定勝負」之後）、
//                          3F61E（24524 返回，AL = 被點的骰子或 0xFF）、3F62B（3ECA8 切換旗標之後）、3F64C（旗標 1 -> 2 之後）、3F679（回合結束）記狀態。
//                          3F471 的區域變數：旗標 5 個 word 在 BP-0x10，座位 BYTE BP-0x18，passes BP-6，t BP-4，骰子陣列指標 [BP+6]（DS 內 5 bytes）。

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

func (e *env) act087(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	switch f[0] {
	case "ifat":
		lin, err := strconv.ParseUint(f[1], 16, 64)
		if err != nil {
			return err
		}
		bud, _ := strconv.ParseUint(f[2], 0, 64)
		target := b.Code(uint32(lin)).Linear()
		err = o.RunUntil(oracle.NewCond("到達 "+f[1], func(o *oracle.Oracle) bool { return o.IP().Linear() == target }), oracle.Budget(bud))
		reached := err == nil
		if err != nil {
			if _, ok := err.(*oracle.BudgetError); !ok {
				return err
			}
		}
		if reached {
			if err := o.Run(3_000_000); err != nil {
				return err
			}
			for _, k := range strings.Split(f[3], "+") {
				if err := o.SendKeys(k); err != nil {
					return err
				}
				if err := o.Run(2_000_000); err != nil {
					return err
				}
			}
		}
		e.log.Emit(Ev78{Kind: "note", Label: "ifat", IDA: f[1], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"reached": reached, "keys": f[3]}})
		return nil
	case "dicehooks":
		e.installDice087()
		return nil
	}
	return fmt.Errorf("不認得的動作")
}

func (e *env) installDice087() {
	b, _ := CurrentOrMain(e.o)
	frame := func(o *oracle.Oracle) map[string]any {
		r := o.Regs()
		ss := func(off int) oracle.Addr { return oracle.Addr{Seg: r.SS, Off: uint16(int(r.BP) + off)} }
		ptr := o.Word(ss(6))
		return map[string]any{
			"flags":  hex.EncodeToString(o.Bytes(ss(-0x10), 10)),
			"dice":   hex.EncodeToString(o.Bytes(b.DS(ptr), 5)),
			"seat":   int(o.Byte(ss(-0x18))),
			"passes": int(o.Word(ss(-6))),
			"t":      int(o.Word(ss(-4))),
			"si":     int(r.SI),
			"ax":     int(r.AX),
			"gold":   gold32(o, b),
		}
	}
	for _, p := range []struct {
		ida   uint32
		label string
	}{{0x3F492, "d_start"}, {0x3F4E5, "d_roll"}, {0x3F512, "d_presort"}, {0x3F522, "d_sorted"}, {0x3F5BD, "d_ask"},
		{0x3F61E, "d_click"}, {0x3F62B, "d_toggled"}, {0x3F64C, "d_conv"}, {0x3F679, "d_end"}} {
		e.hookAt(p.ida, p.label, frame)
	}
}
