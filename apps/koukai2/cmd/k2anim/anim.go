package main

// M4n 新增動作（只觀測，不改狀態）：
//
//	shotat:IDA[+IDA…]:目錄:最多張   任一位址被執行時（去重：中斷返回造成的重複觸發不算）存整個畫面色號（640x480）到
//	                               <目錄>/<NNNN>.idx，印 SHOT 行（序號、位址、步數、計時器 tick、AX）。位址放 15998 的呼叫指令，
//	                               存到的就是這一格等待期間顯示的畫面。
//	shotoff                        停止 shotat。

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

type shotDedup struct {
	prev   oracle.Regs
	ticks  uint64
	irq1   uint64
	step   uint64
	hasPrv bool
}

// again：暫存器全同、步數差在 3000 道內且兩次之間有中斷，就是中斷返回造成的重複（同 k2094 的 dedup）。
func (d *shotDedup) again(o *oracle.Oracle) bool {
	r := o.Regs()
	cur := shotDedup{prev: r, ticks: o.Ticks(), irq1: o.IRQ1Delivered(), step: o.Steps(), hasPrv: true}
	dup := d.hasPrv && d.prev == r && cur.step-d.step < 3000 && (d.ticks != cur.ticks || d.irq1 != cur.irq1)
	*d = cur
	return dup
}

var shotStop bool

func (c *ctx) actAnim(f []string) error {
	o := c.o
	switch f[0] {
	case "shotat":
		if len(f) < 4 {
			return fmt.Errorf("shotat:IDA[+IDA…]:目錄:最多張")
		}
		dir := f[2]
		mx, err := strconv.Atoi(f[3])
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		n := 0
		for _, s := range strings.Split(f[1], "+") {
			lin, err := strconv.ParseUint(s, 16, 32)
			if err != nil {
				return err
			}
			ida := uint32(lin)
			d := &shotDedup{}
			o.OnCall(c.b.Code(ida), func(o *oracle.Oracle) {
				if shotStop || n >= mx || d.again(o) {
					return
				}
				_, _, px := o.Screen()
				fn := filepath.Join(dir, fmt.Sprintf("%04d.idx", n))
				if err := os.WriteFile(fn, px, 0o644); err != nil {
					fmt.Fprintln(os.Stderr, "shotat 寫檔:", err)
				}
				fmt.Printf("SHOT n=%d ida=%05X step=%d tick=%d AX=%04X\n", n, ida, o.Steps(), o.Ticks(), o.Regs().AX)
				n++
			})
		}
		return nil
	case "shotoff":
		shotStop = true
		return nil
	}
	return fmt.Errorf("不認得的動作")
}
