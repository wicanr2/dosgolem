// Command k2077 是批次 077（規則對拍第二批）的觀測工具：k2step 的動作加上事件 hook、
// 寫入監看、條件等待與強制呼叫。事件寫成 JSON lines，由 koukai2 專案的
// `tools/oracle/077_*.py` 比對。
//
// 所有時間都是指令數，同一份腳本、同一個起點永遠得到同一個結果。
//
// 動作（逗號分隔，欄位用冒號）：
//
//	run:N                 跑 N 道指令
//	key:名字              送一個 BIOS 鍵
//	keys:名字+名字+…      連續送鍵，每鍵之間跑 KeyGap
//	move:X:Y  click:X:Y   滑鼠
//	main                  跑到 MAIN.EXE 被載入
//	at:IDA                跑到 IDA 線性位址（第一次）
//	until:IDA:N           跑到該位址被觸發第 N 次（去重後）
//	untilb:OFF:HEX        跑到 DS:OFF 的位元組等於 HEX
//	untilmax:N            （上限）之後的 until 動作最多跑 N 道指令
//	peek:OFF:LEN          印 DS 內容
//	poke:OFF:HEX          改 DS 內容（收據要明列），同時寫進事件檔
//	snap:標籤:項目        記一筆記憶體快照（項目格式見 ev）
//	ev:IDA:標籤:項目[:上限]   掛事件 hook。項目用 + 連接：dOFF.LEN（DS）、eOFF.LEN（ES）、
//	                      bOFF.LEN（ES:BX+OFF）、sN（堆疊 N 個字），數字都是十六進位（sN 是十進位）
//	ww:LO:HI:標籤         監看 DS 偏移範圍的寫入（結束時傾印，IP 換成 IDA 位址）
//	callf:IDA:標籤:參數…   強制呼叫 far 常式（參數十六進位，由右到左推入），記回傳值
//	shot:路徑  idx:路徑   畫面
//	note:文字             在事件檔留一筆註記
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/koukai2"
	"github.com/wicanr2/dosgolem/oracle"
)

type writeWatch struct {
	label string
	lo    uint16
	log   *[]oracle.MemWrite
}

type linWatch struct {
	label string
	lo    uint32
	log   *[]oracle.MemWrite
}

type env struct {
	lwatches []linWatch
	o       *oracle.Oracle
	log     *koukai2.EvLog77
	stats   map[string]*koukai2.HookStat77
	hits    map[uint32]*int
	watches []writeWatch
	max     uint64
}

func main() {
	root := flag.String("root", "/orig", "原版素材目錄")
	scratch := flag.String("scratch", "", "可寫的暫存層")
	loadState := flag.String("load-state", "", "從這個狀態檔接著跑")
	saveState := flag.String("save-state", "", "跑完存狀態")
	shot := flag.String("shot", "", "跑完存 PNG")
	do := flag.String("do", "", "動作腳本")
	evPath := flag.String("ev", "", "事件檔（JSON lines）")
	budget := flag.Uint64("budget", 2_000_000_000, "條件動作的指令數上限")
	flag.Parse()

	o, err := koukai2.Load(*root)
	if err != nil {
		die(err)
	}
	defer o.Close()
	if *scratch != "" {
		o.SetScratch(*scratch)
	}
	if *loadState != "" {
		if err := o.LoadStateFile(*loadState); err != nil {
			die(err)
		}
	}
	var w *bufio.Writer
	if *evPath != "" {
		f, err := os.Create(*evPath)
		if err != nil {
			die(err)
		}
		defer f.Close()
		w = bufio.NewWriterSize(f, 1<<20)
		defer w.Flush()
	} else {
		w = bufio.NewWriter(os.Stdout)
		defer w.Flush()
	}
	e := &env{o: o, log: koukai2.NewEvLog77(w), stats: map[string]*koukai2.HookStat77{}, hits: map[uint32]*int{}, max: *budget}
	for _, a := range strings.Split(*do, ",") {
		if a == "" {
			continue
		}
		if err := e.act(a); err != nil {
			fmt.Fprintln(os.Stderr, koukai2.Describe(o))
			w.Flush()
			die(fmt.Errorf("動作 %q：%w", a, err))
		}
	}
	// 寫入監看的傾印。
	b, _ := koukai2.CurrentOrMain(o)
	for _, ww := range e.watches {
		for _, m := range *ww.log {
			e.log.Emit(koukai2.Ev77{Kind: "write", Label: ww.label, IDA: fmt.Sprintf("%05X", b.ToIDA(m.IP)), Step: m.Step,
				Extra: map[string]any{"off": int(ww.lo) + int(m.Off), "old": m.Old, "new": m.New, "ip_next": fmt.Sprintf("%04X:%04X", m.IP.Seg, m.IP.Off)}})
		}
	}
	for _, ww := range e.lwatches {
		for _, m := range *ww.log {
			e.log.Emit(koukai2.Ev77{Kind: "write", Label: ww.label, IDA: fmt.Sprintf("%05X", b.ToIDA(m.IP)), Step: m.Step,
				Extra: map[string]any{"lin": int(ww.lo) + int(m.Off), "old": m.Old, "new": m.New, "ip_next": fmt.Sprintf("%04X:%04X", m.IP.Seg, m.IP.Off)}})
		}
	}
	for k, st := range e.stats {
		e.log.Emit(koukai2.Ev77{Kind: "stat", Label: k, Step: o.Steps(), Extra: map[string]any{"total": st.Total, "logged": st.Logged, "dup": st.Dup}})
	}
	if *shot != "" {
		if err := o.WritePNG(*shot); err != nil {
			die(err)
		}
	}
	if *saveState != "" {
		if err := o.SaveStateFile(*saveState); err != nil {
			die(err)
		}
	}
	wd, h, px := o.Screen()
	e.log.Emit(koukai2.Ev77{Kind: "end", Label: "final", Step: o.Steps(), Tick: o.Ticks(),
		Extra: map[string]any{"where": koukai2.Describe(o), "mode": int(o.VideoMode()), "w": wd, "h": h,
			"screen_sha256": fmt.Sprintf("%x", sha256.Sum256(px)), "keys_consumed": o.KeysConsumed(), "keys_pending": o.KeysPending()}})
}

func (e *env) act(a string) error {
	o := e.o
	f := strings.Split(a, ":")
	num := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 0, 64) }
	hexn := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 16, 64) }
	b, _ := koukai2.CurrentOrMain(o)
	switch f[0] {
	case "run":
		n, err := num(1)
		if err != nil {
			return err
		}
		return o.Run(n)
	case "key":
		return o.SendKeys(f[1])
	case "wmem":
		// wmem:段.偏移:HEX:標籤：改實體記憶體（測試條件，會寫進事件檔）。
		parts := strings.Split(f[1], ".")
		if len(parts) != 2 {
			return fmt.Errorf("格式 wmem:段.偏移:HEX:標籤")
		}
		sv, err1 := strconv.ParseUint(parts[0], 16, 32)
		ov, err2 := strconv.ParseUint(parts[1], 16, 32)
		data, err3 := hex.DecodeString(f[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return fmt.Errorf("wmem 參數錯誤")
		}
		o.SetBytes(oracle.Addr{Seg: uint16(sv), Off: uint16(ov)}, data)
		e.log.Emit(koukai2.Ev77{Kind: "poke", Label: f[3], Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"seg": int(sv), "off": int(ov), "new": f[2]}})
		return nil
	case "tomain":
		// tomain：跑到 CS:IP 在 MAIN.EXE 的程式碼段（段值不小於映像段）且 IF=1，避開 FMDRV 的 INT/IRQ 處理器內。
		return o.RunUntil(oracle.NewCond("在 MAIN.EXE 碼段且 IF=1", func(o *oracle.Oracle) bool {
			r := o.Regs()
			return r.CS >= b.Seg(0x1000) && r.Flags&0x200 != 0
		}), oracle.Budget(e.max))
	case "type":
		return o.TypeKeys(f[1])
	case "keys":
		for i, k := range strings.Split(f[1], "+") {
			if i > 0 {
				if err := o.Run(koukai2.KeyGap); err != nil {
					return err
				}
			}
			if err := o.SendKeys(k); err != nil {
				return err
			}
		}
		return nil
	case "move", "click":
		x, _ := num(1)
		y, _ := num(2)
		if f[0] == "move" {
			return o.MoveMouse(int(x), int(y))
		}
		err := o.Click(int(x), int(y), oracle.NoCursorWait())
		if _, none := err.(*oracle.NoResponseError); none {
			return nil
		}
		return err
	case "main":
		return o.RunUntil(oracle.NewCond("MAIN.EXE 載入", koukai2.IsMain), oracle.Budget(e.max))
	case "untilmax":
		n, err := num(1)
		if err != nil {
			return err
		}
		e.max = n
		return nil
	case "at":
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		target := b.Code(uint32(lin)).Linear()
		return o.RunUntil(oracle.NewCond("到達 "+f[1], func(o *oracle.Oracle) bool { return o.IP().Linear() == target }), oracle.Budget(e.max))
	case "until":
		// until:IDA:N：該位址被觸發第 N 次（去重）。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		n, err := num(2)
		if err != nil {
			return err
		}
		cnt := 0
		var prev oracle.Regs
		var prevT, prevK, prevS uint64
		first := true
		target := b.Code(uint32(lin)).Linear()
		return o.RunUntil(oracle.NewCond("到達 "+f[1]+" 第 "+f[2]+" 次", func(o *oracle.Oracle) bool {
			if o.IP().Linear() != target {
				return false
			}
			r := o.Regs()
			if !first && r == prev && o.Steps()-prevS < koukai2.Dup77Window && (prevT != o.Ticks() || prevK != o.IRQ1Delivered()) {
				prev, prevT, prevK, prevS = r, o.Ticks(), o.IRQ1Delivered(), o.Steps()
				return false
			}
			first = false
			prev, prevT, prevK, prevS = r, o.Ticks(), o.IRQ1Delivered(), o.Steps()
			cnt++
			return uint64(cnt) >= n
		}), oracle.Budget(e.max))
	case "untilb":
		off, err := hexn(1)
		if err != nil {
			return err
		}
		want, err := hex.DecodeString(f[2])
		if err != nil {
			return err
		}
		return o.RunUntil(oracle.NewCond("DS:"+f[1]+"="+f[2], func(o *oracle.Oracle) bool {
			got := o.Bytes(b.DS(uint16(off)), len(want))
			for i := range want {
				if got[i] != want[i] {
					return false
				}
			}
			return true
		}), oracle.Budget(e.max))
	case "peek":
		off, _ := hexn(1)
		n, _ := hexn(2)
		fmt.Fprintf(os.Stderr, "  步數 %d DS:%04X = % X\n", o.Steps(), off, o.Bytes(b.DS(uint16(off)), int(n)))
		return nil
	case "poke":
		off, err := hexn(1)
		if err != nil {
			return err
		}
		data, err := hex.DecodeString(f[2])
		if err != nil {
			return err
		}
		old := o.Bytes(b.DS(uint16(off)), len(data))
		o.SetBytes(b.DS(uint16(off)), data)
		lab := f[1]
		if len(f) > 3 {
			lab = f[3]
		}
		e.log.Emit(koukai2.Ev77{Kind: "poke", Label: lab, Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"off": int(off), "old": hex.EncodeToString(old), "new": hex.EncodeToString(data)}})
		fmt.Fprintf(os.Stderr, "  步數 %d poke DS:%04X = % X（原 % X）\n", o.Steps(), off, data, old)
		return nil
	case "snap":
		items, err := koukai2.ParseItems77(f[2])
		if err != nil {
			return err
		}
		koukai2.Snap77(o, b, e.log, f[1], items)
		return nil
	case "ev":
		// ev:IDA:標籤:項目[:上限]
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		itemSpec := ""
		if len(f) > 3 {
			itemSpec = f[3]
		}
		items, err := koukai2.ParseItems77(itemSpec)
		if err != nil {
			return err
		}
		lim := 0
		if len(f) > 4 {
			v, _ := num(4)
			lim = int(v)
		}
		e.stats[f[2]] = koukai2.HookEv77(o, b, e.log, uint32(lin), f[2], items, lim)
		return nil
	case "ww":
		lo, err := hexn(1)
		if err != nil {
			return err
		}
		hi, err := hexn(2)
		if err != nil {
			return err
		}
		// 不用 o.WatchWrites：它以 o.DS(0) 換算（KOUKAI2.COM 的段），看不到 MAIN.EXE 的寫入。
		e.watches = append(e.watches, writeWatch{label: f[3], lo: uint16(lo), log: o.WatchWritesAt(b.DS(uint16(lo)).Linear(), b.DS(uint16(hi)).Linear())})
		return nil
	case "wl":
		// wl:線性LO:線性HI:標籤（十六進位）：監看實體位址範圍的寫入（EMS 頁框 D000 等），結束時傾印。
		lo, err := hexn(1)
		if err != nil {
			return err
		}
		hi, err := hexn(2)
		if err != nil {
			return err
		}
		e.lwatches = append(e.lwatches, linWatch{label: f[3], lo: uint32(lo), log: o.WatchWritesAt(uint32(lo), uint32(hi))})
		return nil
	case "callf":
		// callf:IDA 或 callf:IDA段.偏移（用 listing 的段:位移，避免 IP 繞回）
		var target oracle.Addr
		if seg, off, ok := strings.Cut(f[1], "."); ok {
			sv, err := strconv.ParseUint(seg, 16, 32)
			if err != nil {
				return err
			}
			ov, err := strconv.ParseUint(off, 16, 32)
			if err != nil {
				return err
			}
			target = oracle.Addr{Seg: b.Seg(uint16(sv)), Off: uint16(ov)}
		} else {
			lin, err := hexn(1)
			if err != nil {
				return err
			}
			target = b.Code(uint32(lin))
		}
		var args []uint16
		for i := 3; i < len(f); i++ {
			v, err := hexn(i)
			if err != nil {
				return err
			}
			args = append(args, uint16(v))
		}
		v, err := o.CallBudget(5_000_000, target, args...)
		if err != nil {
			return err
		}
		e.log.Emit(koukai2.Ev77{Kind: "call", Label: f[2], IDA: f[1], Step: o.Steps(),
			Extra: map[string]any{"ret": v, "args": f[3:]}})
		return nil
	case "mem":
		// mem:段.偏移.長度:標籤（段是實際段值，十六進位）：記一筆記憶體快照，放進事件的 es 欄。
		parts := strings.Split(f[1], ".")
		if len(parts) != 3 {
			return fmt.Errorf("格式 mem:段.偏移.長度:標籤")
		}
		sv, err1 := strconv.ParseUint(parts[0], 16, 32)
		ov, err2 := strconv.ParseUint(parts[1], 16, 32)
		nv, err3 := strconv.ParseUint(parts[2], 16, 32)
		if err1 != nil || err2 != nil || err3 != nil {
			return fmt.Errorf("mem 參數不是十六進位")
		}
		data := o.Bytes(oracle.Addr{Seg: uint16(sv), Off: uint16(ov)}, int(nv))
		e.log.Emit(koukai2.Ev77{Kind: "snap", Label: f[2], Step: o.Steps(), Tick: o.Ticks(),
			ES: map[string]string{fmt.Sprintf("%04X:%04X.%X", sv, ov, nv): hex.EncodeToString(data)}})
		return nil
	case "setreg":
		// setreg:AX:HEX（可接多組 AX:1,BX:2 以空白分隔不行，一次一個暫存器）
		v, err := hexn(2)
		if err != nil {
			return err
		}
		var r oracle.CallRegs
		switch f[1] {
		case "AX":
			r.AX, r.SetAX = uint16(v), true
		case "BX":
			r.BX, r.SetBX = uint16(v), true
		case "CX":
			r.CX, r.SetCX = uint16(v), true
		case "DX":
			r.DX, r.SetDX = uint16(v), true
		default:
			return fmt.Errorf("不支援的暫存器")
		}
		o.SetRegs(r)
		return nil
	case "note":
		e.log.Emit(koukai2.Ev77{Kind: "note", Label: strings.Join(f[1:], ":"), Step: o.Steps(), Tick: o.Ticks()})
		return nil
	case "idx":
		_, _, px := o.Screen()
		return os.WriteFile(f[1], px, 0o644)
	case "shot":
		return o.WritePNG(f[1])
	}
	return fmt.Errorf("不認得的動作")
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
