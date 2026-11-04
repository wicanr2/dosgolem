// Command k2world 讓原版的 `sub_32D62` 處理世界圖的每一個視窗，把 72x72 世界單位的
// 緩衝全部存下來，給 koukai2 的 `tools/oracle/079_world.py` 與解碼工具逐格比對。
//
// 視窗：3 項，每項 43 列 x 28 欄（`sub_16CFC` 把單位欄夾在 0..27、單位列夾在 0..42）。
// 輸出每個視窗 5,184 bytes，順序是項、列、欄。
//
// 不含任何原版檔案，-root 與 -load-state 由玩家自備。
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"

	"github.com/wicanr2/dosgolem/apps/koukai2"
	"github.com/wicanr2/dosgolem/oracle"
)

const (
	// 要用原本的段值：這些函式內部有 `push cs; call near`，正規化成別的段會跑飛。
	segLoadItem uint16 = 0x1000 // sub_1849A：依 byte_4881A 解壓 WORLDMAP.LZW 的一項
	offLoadItem uint16 = 0x849A
	segWindow   uint16 = 0x2FD9 // sub_32D62：AX = 30*單位列 + 單位欄，展開到 dword_48544
	offWindow   uint16 = 0x2FD2
	dsItem       uint16 = 0x06CA  // byte_4881A
	dsBuf        uint16 = 0x03F4  // dword_48544
	stride              = 0x60
	size                = 72
)

func main() {
	root := flag.String("root", "/orig", "原版素材目錄")
	scratch := flag.String("scratch", "", "可寫的暫存層")
	load := flag.String("load-state", "", "海上畫面的狀態檔")
	out := flag.String("o", "", "視窗緩衝的輸出路徑")
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
	off, seg := o.Word(b.DS(dsBuf)), o.Word(b.DS(dsBuf+2))
	buf := oracle.Addr{Seg: seg, Off: off}
	fmt.Printf("緩衝 %04X:%04X  步數 %d\n", seg, off, o.Steps())

	f, err := os.Create(*out)
	if err != nil {
		die(err)
	}
	defer f.Close()
	h := sha256.New()
	regs := func(ax uint16) oracle.CallRegs {
		return oracle.CallRegs{AX: ax, SetAX: true, DS: dgroup, SetDS: true}
	}
	var windows, maxSteps uint64
	for item := 0; item < 3; item++ {
		o.SetByte(b.DS(dsItem), uint8(item))
		o.SetRegs(regs(0))
		if _, err := o.CallBudget(200_000_000, oracle.Addr{Seg: b.Seg(segLoadItem), Off: offLoadItem}); err != nil {
			die(fmt.Errorf("載入項 %d：%w", item, err))
		}
		for row := 0; row <= 42; row++ {
			for col := 0; col <= 27; col++ {
				o.SetRegs(regs(uint16(30*row + col)))
				before := o.Steps()
				if _, err := o.CallBudget(200_000_000, oracle.Addr{Seg: b.Seg(segWindow), Off: offWindow}); err != nil {
					die(fmt.Errorf("項 %d 列 %d 欄 %d：%w", item, row, col, err))
				}
				if d := o.Steps() - before; d > maxSteps {
					maxSteps = d
				}
				raw := o.Bytes(buf, stride*size)
				win := make([]byte, 0, size*size)
				for r := 0; r < size; r++ {
					win = append(win, raw[r*stride:r*stride+size]...)
				}
				if _, err := f.Write(win); err != nil {
					die(err)
				}
				h.Write(win)
				windows++
			}
		}
	}
	fmt.Printf("視窗 %d  單一視窗最多 %d 道  sha256 %x  步數 %d\n", windows, maxSteps, h.Sum(nil), o.Steps())
	if u := o.Unimplemented(); len(u) > 0 {
		fmt.Printf("未實作：%v\n", u)
	}
	if m := o.Missing(); len(m) > 0 {
		fmt.Printf("缺檔：%v\n", m)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
