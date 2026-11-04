// Command k2song 直接呼叫原版 FMDRV.COM 的 INT 66h，逐首播放 D2.MML 與 8 個音效，
// 把 OPL 埠寫入與開歌當下的驅動計時狀態存檔，給 koukai2 的 `tools/oracle/079_songs.py`
// 對 `tools/assets/mml.py` 的模擬。
//
// 用 SetTickRate 加快計時器：驅動以 IRQ 次數計時，寫入序列與指令數無關，
// 所以只改變跑完要幾道指令，不改變序列（koukai2 `docs/playtest/002-assets.md`）。
//
// 不含任何原版檔案，-root 與 -load-state 由玩家自備。
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/wicanr2/dosgolem/apps/koukai2"
	"github.com/wicanr2/dosgolem/oracle"
)

const (
	drvCounters uint32 = 0x1938 // FMDRV.COM PSP 0182：cs:0118
	drvGCount   uint32 = 0x1DEE // cs:05CE
	dsSE        uint16 = 0x0BC0 // 8 筆音效音色，每筆 22 bytes
	drvSeg      uint16 = 0x0182
	drvSong     uint16 = 0x05D7 // byte_105D7：目前曲號，0 = 停止
	drvBusy     uint16 = 0x05E5 // byte_105E5：sub_108CE 進行中，用驅動自己的堆疊
)

// 音高與 tools/assets/mml.py 的 cmd_se 相同。
var seNotes = []uint16{0x30, 0x30, 0x24, 0x30, 0x3C, 0x3C, 0x30, 0x30}

func main() {
	root := flag.String("root", "/orig", "原版素材目錄")
	scratch := flag.String("scratch", "", "可寫的暫存層")
	load := flag.String("load-state", "", "主迴圈的狀態檔")
	out := flag.String("o", "", "輸出目錄")
	every := flag.Uint64("every", 20000, "每幾道指令送一次計時器中斷")
	first := flag.Int("first", 1, "第一首")
	last := flag.Int("last", 22, "最後一首")
	se := flag.Bool("se", true, "也跑 8 個音效")
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
	o.SetTickRate(*every)
	entry := oracle.Addr{Seg: o.Word(oracle.Addr{Off: 0x19A}), Off: o.Word(oracle.Addr{Off: 0x198})}
	fmt.Printf("INT 66h 入口 %04X:%04X\n", entry.Seg, entry.Off)
	// 用真的 INT 指令呼叫，IF 才會和遊戲呼叫時一樣被清掉（BDA 0x4F0..0x4FF 是空的）。
	stub := oracle.Addr{Seg: 0, Off: 0x4F0}
	o.SetBytes(stub, []byte{0xCD, 0x66, 0xCB})
	b := koukai2.MainAt()
	dgroup := b.Seg(uint16(koukai2.MainDGroup >> 4))

	// 驅動的計時常式換到自己的堆疊並把舊 SS:SP 存在固定位置。在它執行中呼叫 INT 66h 會蓋掉
	// 那份紀錄，所以只在 CPU 離開驅動時才呼叫。
	safe := oracle.NewCond("離開驅動", func(o *oracle.Oracle) bool {
		return o.IP().Seg != drvSeg && o.Byte(oracle.Addr{Seg: drvSeg, Off: drvBusy}) == 0
	})
	int66 := func(ax, dx uint16) uint16 {
		if err := o.RunUntil(safe, oracle.Budget(5_000_000)); err != nil {
			die(err)
		}
		o.SetRegs(oracle.CallRegs{AX: ax, SetAX: true, DX: dx, SetDX: true, DS: dgroup, SetDS: true})
		v, err := o.CallBudget(5_000_000, stub)
		if err != nil {
			die(err)
		}
		return uint16(v)
	}
	phase := func() string {
		c := o.Bytes(oracle.Addr{Seg: uint16(drvCounters >> 4), Off: uint16(drvCounters & 0xF)}, 16)
		g := o.Bytes(oracle.Addr{Seg: uint16(drvGCount >> 4), Off: uint16(drvGCount & 0xF)}, 2)
		return fmt.Sprintf("%d %d %d\n", int(g[0])|int(g[1])<<8, c[8], c[9])
	}
	save := func(name string, mark int, ph string) {
		var sb strings.Builder
		all := o.PortWritesIn(0x388, 0x389)
		for _, w := range all[mark:] {
			fmt.Fprintf(&sb, "%d %03X %02X\n", w.Step, w.Port, w.Val)
		}
		if err := os.WriteFile(*out+"/"+name+".opl", []byte(sb.String()), 0o644); err != nil {
			die(err)
		}
		if err := os.WriteFile(*out+"/"+name+".phase", []byte(ph), 0o644); err != nil {
			die(err)
		}
		fmt.Printf("%s：%d 筆  步數 %d  IRQ0 %d\n", name, (len(all)-mark)/2, o.Steps(), o.Ticks())
	}

	for n := *first; n <= *last; n++ {
		int66(0x0200, 0) // AH=2 AL=0：立刻靜音停止
		if err := o.Run(200_000); err != nil {
			die(err)
		}
		int66(0x0400, 0) // AH=4 AL=0：不迴圈
		if err := o.RunUntil(safe, oracle.Budget(5_000_000)); err != nil {
			die(err)
		}
		ph := phase()
		mark := len(o.PortWritesIn(0x388, 0x389))
		int66(0x0100|uint16(n), 0) // AH=1
		var ran uint64
		for {
			if err := o.Run(1_000_000); err != nil {
				die(err)
			}
			ran += 1_000_000
			if ran%100_000_000 == 0 {
				fmt.Printf("  第 %d 首：跑了 %d 道，寫入 %d 筆，IRQ0 %d，曲號 %d，%s\n", n, ran, (len(o.PortWritesIn(0x388, 0x389))-mark)/2, o.Ticks(), o.Byte(oracle.Addr{Seg: drvSeg, Off: drvSong}), koukai2.Describe(o))
			}
			if o.Byte(oracle.Addr{Seg: drvSeg, Off: drvSong}) == 0 {
				break
			}
			if ran > 4_000_000_000 {
				die(fmt.Errorf("第 %d 首沒有結束", n))
			}
		}
		save(fmt.Sprintf("song%02d", n), mark, ph)
	}

	if *se {
		const irqsHold uint64 = 145 // mml.py：int(0.5 * PIT_HZ / IRQ_DIV)
		for k := 0; k < 8; k++ {
			int66(0x0200, 0)
			if err := o.Run(200_000); err != nil {
				die(err)
			}
			if err := o.RunUntil(safe, oracle.Budget(5_000_000)); err != nil {
				die(err)
			}
			ph := phase()
			mark := len(o.PortWritesIn(0x388, 0x389))
			int66(0x0500, dsSE+uint16(k)*22) // AH=5 AL=0，DS:DX = 音色
			int66(0x0600, seNotes[k])        // AH=6 AL=0，DL = 音高
			t := o.Ticks()
			if err := o.RunUntil(oracle.AtTick(t+irqsHold), oracle.Budget(100_000_000)); err != nil {
				die(err)
			}
			int66(0x0700, 0) // AH=7
			t = o.Ticks()
			if err := o.RunUntil(oracle.AtTick(t+irqsHold), oracle.Budget(100_000_000)); err != nil {
				die(err)
			}
			save(fmt.Sprintf("se%d", k), mark, ph)
		}
	}
	if u := o.Unimplemented(); len(u) > 0 {
		fmt.Printf("未實作：%v\n", u)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
