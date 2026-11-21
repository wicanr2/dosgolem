// Command k2078 是批次 078（規則對拍第二批）的觀測工具：k2step 的動作加上事件 hook、
// 寫入監看、條件等待與強制呼叫。事件寫成 JSON lines，由 koukai2 專案的
// `tools/oracle/078_*.py` 比對。
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
	log     *EvLog78
	stats   map[string]*HookStat78
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

	o, err := Load(*root)
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
	e := &env{o: o, log: NewEvLog78(w), stats: map[string]*HookStat78{}, hits: map[uint32]*int{}, max: *budget}
	for _, a := range strings.Split(*do, ",") {
		if a == "" {
			continue
		}
		if err := e.act(a); err != nil {
			fmt.Fprintln(os.Stderr, Describe(o))
			e.log.Emit(Ev78{Kind: "fail", Label: a, Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"err": err.Error(), "where": Describe(o), "missing": o.Missing(), "console": o.Console()}})
			w.Flush()
			die(fmt.Errorf("動作 %q：%w", a, err))
		}
	}
	// 寫入監看的傾印。
	b, _ := CurrentOrMain(o)
	for _, ww := range e.watches {
		for _, m := range *ww.log {
			e.log.Emit(Ev78{Kind: "write", Label: ww.label, IDA: fmt.Sprintf("%05X", b.ToIDA(m.IP)), Step: m.Step,
				Extra: map[string]any{"off": int(ww.lo) + int(m.Off), "old": m.Old, "new": m.New}})
		}
	}
	for _, ww := range e.lwatches {
		for _, m := range *ww.log {
			e.log.Emit(Ev78{Kind: "write", Label: ww.label, IDA: fmt.Sprintf("%05X", b.ToIDA(m.IP)), Step: m.Step,
				Extra: map[string]any{"lin": int(ww.lo) + int(m.Off), "old": m.Old, "new": m.New}})
		}
	}
	for k, st := range e.stats {
		e.log.Emit(Ev78{Kind: "stat", Label: k, Step: o.Steps(), Extra: map[string]any{"total": st.Total, "logged": st.Logged, "dup": st.Dup}})
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
	e.log.Emit(Ev78{Kind: "end", Label: "final", Step: o.Steps(), Tick: o.Ticks(),
		Extra: map[string]any{"where": Describe(o), "mode": int(o.VideoMode()), "w": wd, "h": h,
			"screen_sha256": fmt.Sprintf("%x", sha256.Sum256(px)), "keys_consumed": o.KeysConsumed(), "keys_pending": o.KeysPending(), "missing": o.Missing(), "console": o.Console()}})
}

func (e *env) act(a string) error {
	o := e.o
	f := strings.Split(a, ":")
	num := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 0, 64) }
	hexn := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 16, 64) }
	b, _ := CurrentOrMain(o)
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
		e.log.Emit(Ev78{Kind: "poke", Label: f[3], Step: o.Steps(), Tick: o.Ticks(),
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
				if err := o.Run(KeyGap); err != nil {
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
		return o.RunUntil(oracle.NewCond("MAIN.EXE 載入", IsMain), oracle.Budget(e.max))
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
	case "retuntil":
		// retuntil:IDA:最多按幾次[:每次間隔指令數]：每次送 Return、跑到間隔用完；一到 IDA 位址就停（不再多送鍵）。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		maxN, _ := num(2)
		gap := uint64(20_000_000)
		if len(f) > 3 {
			gap, _ = num(3)
		}
		target := b.Code(uint32(lin)).Linear()
		cond := oracle.NewCond("到達 "+f[1], func(o *oracle.Oracle) bool { return o.IP().Linear() == target })
		presses := 0
		reached := false
		for i := uint64(0); i < maxN; i++ {
			if err := o.SendKeys("Return"); err != nil {
				return err
			}
			presses++
			err := o.RunUntil(cond, oracle.Budget(gap))
			if err == nil {
				reached = true
				break
			}
			if _, ok := err.(*oracle.BudgetError); !ok {
				return err
			}
		}
		e.log.Emit(Ev78{Kind: "note", Label: "retuntil", IDA: f[1], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"reached": reached, "presses": presses}})
		return nil
	case "tryat":
		// tryat:IDA[:預算]：同 at，但跑滿預算沒到也不算錯，記一筆 note（reached=false）。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		bud := e.max
		if len(f) > 2 {
			bud, _ = num(2)
		}
		target := b.Code(uint32(lin)).Linear()
		err = o.RunUntil(oracle.NewCond("到達 "+f[1], func(o *oracle.Oracle) bool { return o.IP().Linear() == target }), oracle.Budget(bud))
		reached := err == nil
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		e.log.Emit(Ev78{Kind: "note", Label: "tryat", IDA: f[1], Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"reached": reached, "err": msg, "where": Describe(o)}})
		return nil
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
			if !first && r == prev && o.Steps()-prevS < Dup78Window && (prevT != o.Ticks() || prevK != o.IRQ1Delivered()) {
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
		e.log.Emit(Ev78{Kind: "poke", Label: lab, Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"off": int(off), "old": hex.EncodeToString(old), "new": hex.EncodeToString(data)}})
		fmt.Fprintf(os.Stderr, "  步數 %d poke DS:%04X = % X（原 % X）\n", o.Steps(), off, data, old)
		return nil
	case "snap":
		items, err := ParseItems78(f[2])
		if err != nil {
			return err
		}
		Snap78(o, b, e.log, f[1], items)
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
		items, err := ParseItems78(itemSpec)
		if err != nil {
			return err
		}
		lim := 0
		if len(f) > 4 {
			v, _ := num(4)
			lim = int(v)
		}
		e.stats[f[2]] = HookEv78(o, b, e.log, uint32(lin), f[2], items, lim)
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
		e.watches = append(e.watches, writeWatch{label: f[3], lo: uint16(lo), log: o.WatchWritesAt(b.DS(uint16(lo)).Linear(), b.DS(uint16(hi)).Linear())})
		return nil
	case "dgseg":
		// dgseg：印 Oracle.DS(0) 的段與 MAIN.EXE 實際 DS，檢查 WatchWrites（以 o.DS 換算）是否監看到 MAIN.EXE 的資料段。
		e.log.Emit(Ev78{Kind: "note", Label: "dgseg", Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"oracle_DS0_seg": int(o.DS(0).Seg), "main_DS_seg": int(b.DS(0).Seg), "cpu_DS": int(o.Regs().DS)}})
		return nil
	case "minihooks":
		e.installMini()
		return nil
	case "bhooks":
		// bhooks：掛海戰、白刃的 hook（battle.go）。
		e.installBattle()
		return nil
	case "relay":
		e.installRelay()
		return nil
	case "toend":
		// toend：跑到 END.EXE 被 EXEC 載入（最後一支未結束的程式是 END.EXE）。
		err := o.RunUntil(oracle.NewCond("END.EXE 載入", func(o *oracle.Oracle) bool {
			c, ok := Current(o)
			return ok && c.Name == "END.EXE"
		}), oracle.Budget(e.max))
		e.log.Emit(Ev78{Kind: "note", Label: "toend", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"where": Describe(o), "err": fmt.Sprint(err)}})
		return err
	case "endhooks":
		var psp uint64
		if len(f) > 1 {
			psp, _ = hexn(1)
		}
		e.installEnd(uint16(psp))
		return nil
	case "endrun":
		// endrun:最多按幾次:每次間隔指令數：每次送 Return、跑一段；整個程式（KOUKAI2.COM）結束就停並記錄結束碼。
		maxN, _ := num(1)
		gap, _ := num(2)
		for i := uint64(0); i < maxN; i++ {
			if err := o.SendKeys("Return"); err != nil {
				return err
			}
			if err := o.Run(gap); err != nil {
				if ee, ok := err.(*oracle.ExitError); ok {
					e.log.Emit(Ev78{Kind: "note", Label: "program_exit", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"code": int(ee.Code), "console": ee.Console, "presses": int(i + 1)}})
					return nil
				}
				return err
			}
		}
		e.log.Emit(Ev78{Kind: "note", Label: "endrun_limit", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"presses": int(maxN), "where": Describe(o)}})
		return nil
	case "opl":
		// opl:路徑：OPL 埠寫入，每行 步數 埠 值。
		var sb strings.Builder
		for _, w := range o.PortWritesIn(0x388, 0x389) {
			fmt.Fprintf(&sb, "%d %03X %02X\n", w.Step, w.Port, w.Val)
		}
		return os.WriteFile(f[1], []byte(sb.String()), 0o644)
	case "wstop":
		// wstop：停止寫入監看（已記下的筆數保留，結束時傾印）。
		o.StopWatchingWrites()
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
		e.log.Emit(Ev78{Kind: "call", Label: f[2], IDA: f[1], Step: o.Steps(),
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
		e.log.Emit(Ev78{Kind: "snap", Label: f[2], Step: o.Steps(), Tick: o.Ticks(),
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
		e.log.Emit(Ev78{Kind: "note", Label: strings.Join(f[1:], ":"), Step: o.Steps(), Tick: o.Ticks()})
		return nil
	case "rclick":
		err := o.Press(oracle.Right())
		if _, none := err.(*oracle.NoResponseError); none {
			return nil
		}
		return err
	case "dump":
		// dump:DS偏移hex:長度hex:路徑
		off, err := hexn(1)
		if err != nil {
			return err
		}
		n, err := hexn(2)
		if err != nil {
			return err
		}
		return os.WriteFile(f[3], o.Bytes(b.DS(uint16(off)), int(n)), 0o644)
	case "dseg":
		// dseg:DS偏移hex（內含段值）:長度hex:路徑。傾印該段 0 偏移起的記憶體，並在事件檔記段值。
		off, err := hexn(1)
		if err != nil {
			return err
		}
		n, err := hexn(2)
		if err != nil {
			return err
		}
		seg := o.Word(b.DS(uint16(off)))
		e.log.Emit(Ev78{Kind: "note", Label: "dseg", Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"ptr_off": int(off), "seg": int(seg), "len": int(n)}})
		return os.WriteFile(f[3], o.Bytes(oracle.Addr{Seg: seg, Off: 0}, int(n)), 0o644)
	case "dax":
		// dax:長度hex:路徑：傾印 AX:0 起的記憶體（AX 是段值，用在 sub_45D44 返回之後）。
		n, err := hexn(1)
		if err != nil {
			return err
		}
		seg := o.Regs().AX
		e.log.Emit(Ev78{Kind: "note", Label: "dax", Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"seg": int(seg), "len": int(n)}})
		return os.WriteFile(f[2], o.Bytes(oracle.Addr{Seg: seg, Off: 0}, int(n)), 0o644)
	case "lin":
		// lin:線性hex:長度hex:路徑
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		n, err := hexn(2)
		if err != nil {
			return err
		}
		return os.WriteFile(f[3], o.Bytes(oracle.Addr{Seg: uint16(lin >> 4), Off: uint16(lin & 0xF)}, int(n)), 0o644)
	case "fops":
		// fops[:起始步數]：把檔案操作記成事件（只記步數不小於起始步數者）。
		var from uint64
		if len(f) > 1 {
			from, _ = num(1)
		}
		for _, op := range o.FileOps() {
			if op.Step < from {
				continue
			}
			e.log.Emit(Ev78{Kind: "fop", Label: op.Op, Step: op.Step, Extra: map[string]any{
				"name": op.Name, "fn": int(op.Fn), "handle": int(op.Handle), "arg": op.Arg,
				"pos": op.Pos, "len": op.Len, "whence": int(op.Whence), "failed": op.Failed}})
		}
		return nil
	case "missing":
		// missing：開檔失敗（找不到檔）的名稱清單。
		e.log.Emit(Ev78{Kind: "missing", Label: "files", Step: o.Steps(), Extra: map[string]any{"names": o.Missing(), "console": o.Console()}})
		return nil
	case "wrote":
		// wrote：程式的寫檔紀錄（Oracle.Wrote，依序，沒有步數；FileOps 不含寫入）。
		for i, w := range o.Wrote() {
			e.log.Emit(Ev78{Kind: "wrote", Label: w.Name, Step: o.Steps(), Extra: map[string]any{"i": i, "n": w.N}})
		}
		return nil
	case "exec":
		for _, r := range o.ExecLog() {
			e.log.Emit(Ev78{Kind: "exec", Label: r.Base, Step: o.Steps(), Extra: map[string]any{
				"psp": int(r.PSP), "exit": r.Exit, "ended": r.Ended, "tsr": r.TSR}})
		}
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
