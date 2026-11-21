// Command k2trace 從狀態檔跑一段腳本，保留最後幾千道指令的位址與暫存器。
// 程式跑出可用記憶體時印出前面的軌跡，用來找控制流從哪裡壞掉。
//
// 不含任何原版檔案，-root 與 -load-state 由玩家自備。
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/koukai2"
	"github.com/wicanr2/dosgolem/oracle"
)

type rec struct {
	r    oracle.Regs
	step uint64
}

func main() {
	root := flag.String("root", "/orig", "原版素材目錄")
	scratch := flag.String("scratch", "", "可寫的暫存層")
	load := flag.String("load-state", "", "狀態檔")
	do := flag.String("do", "", "動作：at:IDA、poke:偏移:hex、run:N、key:名字，逗號分隔")
	n := flag.Int("n", 200, "失敗時印最後幾筆")
	watch := flag.String("watch", "", "監看線性範圍的寫入：LOhex-HIhex")
	flag.Parse()

	o, err := koukai2.Load(*root)
	if err != nil {
		die(err)
	}
	defer o.Close()
	if *scratch != "" {
		o.SetScratch(*scratch)
	}
	if err := o.LoadStateFile(*load); err != nil {
		die(err)
	}
	b := koukai2.MainAt()
	ring := make([]rec, 8192)
	var count uint64
	o.SetStepGuard(func(o *oracle.Oracle) error {
		ring[count%uint64(len(ring))] = rec{o.Regs(), o.Steps()}
		count++
		return nil
	})
	var wlog *[]oracle.MemWrite
	if *watch != "" {
		lo, hi, _ := strings.Cut(*watch, "-")
		l, _ := strconv.ParseUint(lo, 16, 32)
		h, _ := strconv.ParseUint(hi, 16, 32)
		wlog = o.WatchWritesAt(uint32(l), uint32(h))
	}
	dump := func() {
		if wlog != nil {
			w := *wlog
			if len(w) > 60 {
				w = w[len(w)-60:]
			}
			for _, m := range w {
				fmt.Printf("寫入 +%02X %02X->%02X 步數 %d 下一道 %04X:%04X IDA %05X\n", m.Off, m.Old, m.New, m.Step, m.IP.Seg, m.IP.Off, b.ToIDA(m.IP))
			}
		}
		start := uint64(0)
		if count > uint64(*n) {
			start = count - uint64(*n)
		}
		for i := start; i < count; i++ {
			e := ring[i%uint64(len(ring))]
			a := oracle.Addr{Seg: e.r.CS, Off: e.r.IP}
			fmt.Printf("%d %04X:%04X IDA %05X AX=%04X BX=%04X CX=%04X DX=%04X SP=%04X BP=%04X DS=%04X ES=%04X SI=%04X DI=%04X % X\n",
				e.step, e.r.CS, e.r.IP, b.ToIDA(a), e.r.AX, e.r.BX, e.r.CX, e.r.DX, e.r.SP, e.r.BP, e.r.DS, e.r.ES, e.r.SI, e.r.DI, o.Bytes(a, 6))
		}
	}
	for _, a := range strings.Split(*do, ",") {
		f := strings.Split(a, ":")
		var err error
		switch f[0] {
		case "at":
			lin, _ := strconv.ParseUint(f[1], 16, 32)
			t := b.Code(uint32(lin)).Linear()
			err = o.RunUntil(oracle.NewCond("到達 "+f[1], func(o *oracle.Oracle) bool { return o.IP().Linear() == t }), oracle.Budget(2_000_000_000))
		case "poke":
			off, _ := strconv.ParseUint(f[1], 16, 32)
			data, _ := hex.DecodeString(f[2])
			o.SetBytes(b.DS(uint16(off)), data)
		case "run":
			v, _ := strconv.ParseUint(f[1], 10, 64)
			err = o.Run(v)
		case "key":
			err = o.SendKeys(f[1])
		}
		if err != nil {
			fmt.Println("錯誤：", err)
			dump()
			fmt.Printf("堆疊 SS:SP %04X:%04X\n", o.Regs().SS, o.Regs().SP)
			for i := 0; i < 24; i++ {
				fmt.Printf(" %04X", o.StackWord(i))
			}
			fmt.Println()
			return
		}
	}
	fmt.Println("腳本跑完，沒有錯誤", koukai2.Describe(o))
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
