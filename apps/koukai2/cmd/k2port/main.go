// Command k2port 讓原版的 `sub_188E9` 依序載入每個港口編號（0..129），把圖塊組、通行分界
// 與地圖緩衝存檔，給 koukai2 的 `tools/oracle/079_ports.py` 對解碼素材比對。
//
// 不含任何原版檔案，-root 與 -load-state 由玩家自備。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/apps/koukai2"
	"github.com/wicanr2/dosgolem/oracle"
)

const (
	dsPort   uint16 = 0x0CAE // word_48DFE
	dsChipLo uint16 = 0x03EC // word_4853C
	dsChipHi uint16 = 0x03EE // word_4853E
	dsPass   uint16 = 0xBD78 // 通行分界 4 bytes
)

func main() {
	root := flag.String("root", "/orig", "原版素材目錄")
	scratch := flag.String("scratch", "", "可寫的暫存層")
	load := flag.String("load-state", "", "主迴圈的狀態檔")
	out := flag.String("o", "", "輸出目錄")
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
	dgroup := b.Seg(uint16(koukai2.MainDGroup >> 4))
	entry := oracle.Addr{Seg: b.Seg(0x1000), Off: 0x88E9}
	chip := oracle.Addr{Seg: o.Word(b.DS(dsChipHi)), Off: o.Word(b.DS(dsChipLo))}
	fmt.Printf("圖塊緩衝 %04X:%04X\n", chip.Seg, chip.Off)

	var dest oracle.Addr
	o.OnCall(b.Code(0x45284), func(o *oracle.Oracle) {
		// sub_45284：DX:BX = 目的地，AX = 檔案。最後一次呼叫是 PORTMAP。
		r := o.Regs()
		dest = oracle.Addr{Seg: r.DX, Off: r.BX}
	})
	for id := 0; id < 130; id++ {
		o.SetWord(b.DS(dsPort), uint16(id))
		o.SetRegs(oracle.CallRegs{DS: dgroup, SetDS: true})
		if _, err := o.CallBudget(200_000_000, entry); err != nil {
			die(fmt.Errorf("港口 %d：%w", id, err))
		}
		raw := append([]byte{}, o.Bytes(chip, 30720)...)
		raw = append(raw, o.Bytes(b.DS(dsPass), 4)...)
		raw = append(raw, o.Bytes(dest, 9216)...)
		if err := os.WriteFile(fmt.Sprintf("%s/%03d.bin", *out, id), raw, 0o644); err != nil {
			die(err)
		}
	}
	fmt.Printf("地圖緩衝 %04X:%04X  步數 %d\n", dest.Seg, dest.Off, o.Steps())
	if m := o.Missing(); len(m) > 0 {
		fmt.Printf("缺檔：%v\n", m)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
