// Command k2088 是 koukai2 批次 088（規則對拍：國家 AI、升級、造船廠、王宮、介面）的觀測工具。
// 複製自 k2085，另加：
//
//	callf:段.偏移:標籤[:參數hex…]  強制呼叫 far 常式（段是 listing 的段值，參數由右到左推入），印回傳值並還原暫存器
//	setreg:AX:HEX                  設定暫存器（呼叫前用；之後讓遊戲繼續跑要自己記下 regs 再寫回）
//	regs:標籤                      印目前暫存器與 IP
//	dsr:路徑:偏移:長度             傾印 DS 範圍
//	seg:路徑:偏移:長度             傾印 DS 內遠指標指向的段
//
// 以下是 k2085 的說明。
//
// 以 k2step 為底，多了：
//
//	keyc:名字[:N]        送一個鍵，跑到程式讀走它，再跑 N 道（預設 0）。
//	                     遊戲讀鍵後會清空鍵盤緩衝，連送多鍵只有第一個有效，所以一律用這個。
//	typec:文字[:N]       逐字 keyc。
//	rng:on / rng:off     記錄 14DC4（rand_n）呼叫：種子、n、呼叫端。輪詢迴圈的呼叫合併成一行。
//	fn:IDA:標籤          在 IDA 位址掛 hook，印暫存器、堆疊與呼叫端。
//	wr:偏移:長度:標籤     監看 DS 範圍的寫入（一次只有一個範圍）。
//	obs:標籤:偏移:長度    印 DS 內容，帶步數與標籤，供比對腳本解析。
//	p8/p16/p32:偏移:值   poke 十進位數值（小端），收據要明列。
//	save:路徑 / load:路徑  存或讀狀態檔。
//	mem:標籤 / back:標籤   在行程內存取記憶體快照。
//	stub:IDA:值          把 far 常式釘死成回傳值（測試條件，收據要明列）。
//	clr                  清掉鍵盤佇列。
//
// 輸出行首碼：R 亂數、P 輪詢亂數（合併）、F hook、W 寫入、OBS 觀測、ACT 動作。
package main

import (
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

// 輸入輪詢函式呼叫 rand_n(100) 的返回位址（遠呼叫 5 bytes，051）。
var pollRet = map[uint32]string{
	0x23429: "23424",
	0x23D98: "23D93",
	0x258DB: "258D6",
	0x25D8D: "25D88",
}

type ctx struct {
	o      *oracle.Oracle
	b      koukai2.Bind
	budget uint64

	rngOn    bool
	rngHook  bool
	last     rngRec
	haveLast bool
	pend     struct {
		on    bool
		step  uint64
		seed  uint32
		ret   uint32
		count int
	}
	rngCount  uint64
	repeats   uint64
	hooked    map[string]bool
	wrOn      bool
	counters     map[string]int
	counterOrder []string
	snaps     map[string]*oracle.State
	stdoutBuf strings.Builder
}

type rngRec struct {
	seed uint32
	n    uint16
	sp   uint16
	ret  uint32
}

func main() {
	root := flag.String("root", "/orig", "原版素材目錄")
	scratch := flag.String("scratch", "", "可寫的暫存層")
	loadState := flag.String("load-state", "", "從這個狀態檔接著跑")
	saveState := flag.String("save-state", "", "跑完存狀態")
	shot := flag.String("shot", "", "跑完存 PNG")
	do := flag.String("do", "", "動作腳本")
	budget := flag.Uint64("budget", 400_000_000, "條件動作的指令數上限")
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
	c := &ctx{o: o, budget: *budget, hooked: map[string]bool{}, snaps: map[string]*oracle.State{}, counters: map[string]int{}}
	c.b, _ = koukai2.CurrentOrMain(o)
	for _, a := range strings.Split(*do, ",") {
		if a == "" {
			continue
		}
		if err := c.act(a); err != nil {
			c.flush()
			fmt.Println(koukai2.Describe(o))
			die(fmt.Errorf("動作 %q：%w", a, err))
		}
		c.flush()
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
	w, h, px := o.Screen()
	polls, _ := o.MouseActivity()
	fmt.Printf("步數 %d  %s\n模式 %02Xh %dx%d 畫面 sha256 %x\n滑鼠輪詢 %d  讀走鍵 %d  待讀 %d\n",
		o.Steps(), koukai2.Describe(o), o.VideoMode(), w, h, sha256.Sum256(px),
		polls, o.KeysConsumed(), o.KeysPending())
	if c.rngHook {
		fmt.Printf("RNGSUM calls=%d repeats=%d\n", c.rngCount, c.repeats)
	}
	if u := o.Unimplemented(); len(u) > 0 {
		fmt.Printf("未實作：%v\n", u)
	}
	if m := o.Missing(); len(m) > 0 {
		fmt.Printf("缺檔：%v\n", m)
	}
}

func (c *ctx) flush() {
	if c.pend.on {
		fmt.Printf("P step=%d seed=%08X caller=%05X site=%s count=%d\n",
			c.pend.step, c.pend.seed, c.pend.ret, pollRet[c.pend.ret], c.pend.count)
		c.pend.on = false
	}
}

func (c *ctx) seed() uint32 {
	return uint32(c.o.Word(c.b.DS(koukai2.DSSeedLo))) | uint32(c.o.Word(c.b.DS(koukai2.DSSeedHi)))<<16
}

func (c *ctx) hookRNG() {
	if c.rngHook {
		return
	}
	c.rngHook = true
	c.o.OnCall(c.b.Code(koukai2.AddrRandN), func(o *oracle.Oracle) {
		if !c.rngOn {
			return
		}
		n := o.AX()
		seed := c.seed()
		sp := o.SP()
		ret := c.b.ToIDA(o.Caller())
		rec := rngRec{seed: seed, n: n, sp: sp, ret: ret}
		// 中斷返回造成的重複觸發：n 非 0 時真呼叫必改種子。
		if c.haveLast && n != 0 && rec == c.last {
			c.repeats++
			return
		}
		c.last, c.haveLast = rec, true
		c.rngCount++
		if _, isPoll := pollRet[ret]; isPoll && n == 100 {
			if c.pend.on && c.pend.ret == ret {
				c.pend.count++
				return
			}
			c.flush()
			c.pend.on, c.pend.step, c.pend.seed, c.pend.ret, c.pend.count = true, o.Steps(), seed, ret, 1
			return
		}
		c.flush()
		fmt.Printf("R step=%d seed=%08X n=%d caller=%05X\n", o.Steps(), seed, n, ret)
	})
}

func (c *ctx) act(a string) error {
	o := c.o
	f := strings.Split(a, ":")
	num := func(i int) (uint64, error) {
		if i >= len(f) {
			return 0, fmt.Errorf("缺參數 %d", i)
		}
		return strconv.ParseUint(f[i], 0, 64)
	}
	hexn := func(i int) (uint64, error) {
		if i >= len(f) {
			return 0, fmt.Errorf("缺參數 %d", i)
		}
		return strconv.ParseUint(f[i], 16, 32)
	}
	switch f[0] {
	case "run":
		n, err := num(1)
		if err != nil {
			return err
		}
		return o.Run(n)
	case "key", "keyc":
		extra := uint64(0)
		if len(f) > 2 {
			v, err := num(2)
			if err != nil {
				return err
			}
			extra = v
		}
		return c.sendKey(f[1], extra, f[0] == "keyc")
	case "typec":
		extra := uint64(0)
		if len(f) > 2 {
			v, err := num(2)
			if err != nil {
				return err
			}
			extra = v
		}
		for _, r := range f[1] {
			if err := c.sendKey(string(r), extra, true); err != nil {
				return err
			}
		}
		return nil
	case "clr":
		o.ClearInput()
		return nil
	case "move":
		x, _ := num(1)
		y, _ := num(2)
		return o.MoveMouse(int(x), int(y))
	case "click":
		x, _ := num(1)
		y, _ := num(2)
		err := o.Click(int(x), int(y), oracle.NoCursorWait())
		if _, none := err.(*oracle.NoResponseError); none {
			fmt.Printf("ACT click-noresponse %v\n", err)
			return nil
		}
		return err
	case "rclick":
		err := o.Press(oracle.Right())
		if _, none := err.(*oracle.NoResponseError); none {
			fmt.Printf("ACT rclick-noresponse %v\n", err)
			return nil
		}
		return err
	case "poll":
		n, err := num(1)
		if err != nil {
			return err
		}
		return o.RunUntil(oracle.MousePolled(int(n)), oracle.Budget(c.budget))
	case "at":
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		target := c.b.Code(uint32(lin)).Linear()
		skip := uint64(0)
		if len(f) > 2 {
			skip, _ = num(2)
		}
		hits := uint64(0)
		err = o.RunUntil(oracle.NewCond("到達 "+f[1], func(o *oracle.Oracle) bool {
			if o.IP().Linear() == target {
				hits++
				return hits > skip
			}
			return false
		}), oracle.Budget(c.budget))
		return err
	case "escat":
		// escat:IDA 反覆「跑到該位址（上限 3M 道），沒到就送 Esc」，最多 40 輪。
		// 用來離開設施（含道別訊息）後停在一天迴圈頂端（2B2F2），不靠固定的 Esc 次數。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		target := c.b.Code(uint32(lin)).Linear()
		skip := uint64(0)
		if len(f) > 2 {
			skip, _ = num(2)
		}
		hits := uint64(0)
		for i := 0; i < 60; i++ {
			err := o.RunUntil(oracle.NewCond("到達 "+f[1], func(o *oracle.Oracle) bool {
				if o.IP().Linear() == target {
					hits++
					return hits > skip
				}
				return false
			}), oracle.Budget(3_000_000))
			if err == nil {
				fmt.Printf("ACT escat reached %s after %d Esc step=%d\n", f[1], i, o.Steps())
				return nil
			}
			if _, ok := err.(*oracle.BudgetError); !ok {
				return err
			}
			if err := o.SendKeys("Escape"); err != nil {
				return err
			}
		}
		return fmt.Errorf("40 輪 Esc 後仍未到達 %s", f[1])
	case "shot":
		return o.WritePNG(f[1])
	case "idx":
		_, _, px := o.Screen()
		return os.WriteFile(f[1], px, 0o644)
	case "dsd":
		// dsd:路徑 傾印整個 DS（64 KB），並印步數與雜湊。
		data := o.Bytes(c.b.DS(0), 0x10000)
		fmt.Printf("DSD %s step=%d sha256=%x\n", f[1], o.Steps(), sha256.Sum256(data))
		return os.WriteFile(f[1], data, 0o644)
	case "obs":
		// obs:標籤:DS偏移hex:長度hex
		off, err := hexn(2)
		if err != nil {
			return err
		}
		n, err := hexn(3)
		if err != nil {
			return err
		}
		fmt.Printf("OBS %s step=%d off=%04X %s\n", f[1], o.Steps(), off, hex.EncodeToString(o.Bytes(c.b.DS(uint16(off)), int(n))))
		return nil
	case "peek":
		off, err := hexn(1)
		if err != nil {
			return err
		}
		n, err := hexn(2)
		if err != nil {
			return err
		}
		fmt.Printf("  步數 %d DS:%04X = % X\n", o.Steps(), off, o.Bytes(c.b.DS(uint16(off)), int(n)))
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
		o.SetBytes(c.b.DS(uint16(off)), data)
		fmt.Printf("ACT poke step=%d DS:%04X = % X\n", o.Steps(), off, data)
		return nil
	case "p8", "p16", "p32":
		off, err := hexn(1)
		if err != nil {
			return err
		}
		v, err := strconv.ParseInt(f[2], 0, 64)
		if err != nil {
			return err
		}
		sz := map[string]int{"p8": 1, "p16": 2, "p32": 4}[f[0]]
		data := make([]byte, sz)
		for i := 0; i < sz; i++ {
			data[i] = byte(uint64(v) >> (8 * i))
		}
		o.SetBytes(c.b.DS(uint16(off)), data)
		fmt.Printf("ACT poke step=%d DS:%04X = % X\n", o.Steps(), off, data)
		return nil
	case "pokebp":
		// pokebp:偏移(十進位，可負):值  寫 SS:(BP+偏移) 一個位元組。
		// 用在 at:2B2F2（一天迴圈頂端）把本時段變數 [bp-1] 設成測試時段。收據要明列。
		off, err := strconv.ParseInt(f[1], 0, 32)
		if err != nil {
			return err
		}
		v, err := strconv.ParseUint(f[2], 0, 8)
		if err != nil {
			return err
		}
		r := o.Regs()
		a := oracle.Addr{Seg: r.SS, Off: uint16(int(r.BP) + int(off))}
		o.SetByte(a, uint8(v))
		fmt.Printf("ACT pokebp step=%d ip=%05X SS:%04X BP=%04X%+d = %d\n", o.Steps(), c.b.ToIDA(o.IP()), r.SS, r.BP, off, v)
		return nil
	case "save":
		return o.SaveStateFile(f[1])
	case "load":
		if err := o.LoadStateFile(f[1]); err != nil {
			return err
		}
		return nil
	case "mem":
		c.snaps[f[1]] = o.Save()
		return nil
	case "back":
		s, ok := c.snaps[f[1]]
		if !ok {
			return fmt.Errorf("沒有快照 %s", f[1])
		}
		o.Restore(s)
		return nil
	case "rng":
		c.hookRNG()
		c.rngOn = len(f) > 1 && f[1] == "on"
		c.haveLast = false
		return nil
	case "fn":
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		label := f[1]
		if len(f) > 2 {
			label = f[2]
		}
		o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			c.flush()
			r := o.Regs()
			fmt.Printf("F %s step=%d ida=%X AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X BP=%04X SP=%04X ret=%05X st=%04X,%04X,%04X,%04X,%04X,%04X,%04X,%04X\n",
				label, o.Steps(), lin, r.AX, r.BX, r.CX, r.DX, r.SI, r.DI, r.BP, r.SP,
				c.b.ToIDA(o.Caller()),
				o.StackWord(0), o.StackWord(1), o.StackWord(2), o.StackWord(3),
				o.StackWord(4), o.StackWord(5), o.StackWord(6), o.StackWord(7))
		})
		return nil
	case "fno":
		// fno:IDA:標籤:DS偏移hex:長度hex[:筆數上限]  在 hook 當下印暫存器與 DS 區段。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		label := f[2]
		off, err := hexn(3)
		if err != nil {
			return err
		}
		n, err := hexn(4)
		if err != nil {
			return err
		}
		o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			c.flush()
			r := o.Regs()
			fmt.Printf("FM %s step=%d ida=%X AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X SP=%04X ret=%05X st=%04X,%04X,%04X,%04X off=%04X mem=%s\n",
				label, o.Steps(), lin, r.AX, r.BX, r.CX, r.DX, r.SI, r.DI, r.SP, c.b.ToIDA(o.Caller()),
				o.StackWord(0), o.StackWord(1), o.StackWord(2), o.StackWord(3),
				off, hex.EncodeToString(o.Bytes(c.b.DS(uint16(off)), int(n))))
		})
		return nil
	case "mapdump":
		// mapdump:路徑 讀 DS:03F4 的遠指標（dword_48544），傾印 0x2400 bytes 的港口地圖。
		seg := o.Word(c.b.DS(0x3F6))
		off := o.Word(c.b.DS(0x3F4))
		data := o.Bytes(oracle.Addr{Seg: seg, Off: off}, 0x2400)
		fmt.Printf("MAP %s step=%d ptr=%04X:%04X sha256=%x\n", f[1], o.Steps(), seg, off, sha256.Sum256(data))
		return os.WriteFile(f[1], data, 0o644)
	case "waitpoll":
		// waitpoll:N 跑到滑鼠再被輪詢 N 次（等程式回到讀輸入的迴圈）。
		n, err := num(1)
		if err != nil {
			return err
		}
		return o.RunUntil(oracle.MousePolled(int(n)), oracle.Budget(c.budget))
	case "wr":
		off, err := hexn(1)
		if err != nil {
			return err
		}
		n, err := hexn(2)
		if err != nil {
			return err
		}
		label := f[3]
		base := c.b.DS(uint16(off)).Linear()
		ds0 := c.b.DS(0).Linear()
		oa := o.ToIDA(oracle.Addr{})
		o.OnWrite(base, base+uint32(n), func(o *oracle.Oracle, h oracle.WriteHit) {
			c.flush()
			at := h.At - oa
			fmt.Printf("W %s step=%d off=%04X old=%02X new=%02X at=%05X\n", label, o.Steps(),
				h.Addr-ds0, h.Old, h.Val, at-uint32(c.b.Seg(0x1000))*16+0x10000)
		})
		return nil
	case "cnt":
		// cnt:IDA:標籤  在位址掛計數器（中斷返回造成的重複觸發以 SP 與連續性不去重，僅供量級比較）。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		label := f[2]
		c.counters[label] = 0
		c.counterOrder = append(c.counterOrder, label)
		o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) { c.counters[label]++ })
		return nil
	case "cntp":
		// cntp:標籤  印出並歸零計數器，同時印目前步數。
		fmt.Printf("CNT %s step=%d", f[1], o.Steps())
		for _, l := range c.counterOrder {
			fmt.Printf(" %s=%d", l, c.counters[l])
			c.counters[l] = 0
		}
		fmt.Println()
		return nil
	case "nowr":
		o.StopWrites()
		return nil
	case "stub":
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		v, err := num(2)
		if err != nil {
			return err
		}
		o.StubValue(c.b.Code(uint32(lin)), uint32(v))
		fmt.Printf("ACT stub %X = %d\n", lin, v)
		return nil
	case "say":
		fmt.Printf("ACT %s step=%d\n", strings.Join(f[1:], ":"), o.Steps())
		return nil
	case "callf":
		// callf:段.偏移:標籤[:參數…]（段是 listing 的段值；不用 b.Code 換算，避免 IP 繞回）
		seg, off, ok := strings.Cut(f[1], ".")
		if !ok {
			return fmt.Errorf("格式 callf:段.偏移:標籤:參數")
		}
		sv, err := strconv.ParseUint(seg, 16, 32)
		if err != nil {
			return err
		}
		ov, err := strconv.ParseUint(off, 16, 32)
		if err != nil {
			return err
		}
		target := oracle.Addr{Seg: c.b.Seg(uint16(sv)), Off: uint16(ov)}
		var args []uint16
		r0 := o.Regs()
		var pre oracle.CallRegs
		for i := 3; i < len(f); i++ {
			if reg, val, ok := strings.Cut(f[i], "="); ok {
				// AX=0005 之類：呼叫前設暫存器，呼叫後（含 o.Call 自己的還原之後）寫回原值
				x, err := strconv.ParseUint(val, 16, 32)
				if err != nil {
					return err
				}
				switch reg {
				case "AX":
					pre.AX, pre.SetAX = uint16(x), true
				case "BX":
					pre.BX, pre.SetBX = uint16(x), true
				case "CX":
					pre.CX, pre.SetCX = uint16(x), true
				case "DX":
					pre.DX, pre.SetDX = uint16(x), true
				case "SI":
					pre.SI, pre.SetSI = uint16(x), true
				case "DI":
					pre.DI, pre.SetDI = uint16(x), true
				default:
					return fmt.Errorf("不支援的暫存器 %s", reg)
				}
				continue
			}
			v, err := hexn(i)
			if err != nil {
				return err
			}
			args = append(args, uint16(v))
		}
		o.SetRegs(pre)
		v, err := o.CallBudget(20_000_000, target, args...)
		o.SetRegs(oracle.CallRegs{AX: r0.AX, BX: r0.BX, CX: r0.CX, DX: r0.DX, SI: r0.SI, DI: r0.DI, BP: r0.BP,
			SetAX: true, SetBX: true, SetCX: true, SetDX: true, SetSI: true, SetDI: true, SetBP: true})
		if err != nil {
			return err
		}
		fmt.Printf("CALL %s step=%d ret=%08X\n", f[2], o.Steps(), v)
		return nil
	case "setreg":
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
		case "SI":
			r.SI, r.SetSI = uint16(v), true
		case "DI":
			r.DI, r.SetDI = uint16(v), true
		default:
			return fmt.Errorf("不支援的暫存器")
		}
		o.SetRegs(r)
		return nil
	case "regs":
		r := o.Regs()
		fmt.Printf("REGS %s step=%d ida=%05X AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X BP=%04X SP=%04X SS=%04X DS=%04X ES=%04X\n",
			f[1], o.Steps(), c.b.ToIDA(o.IP()), r.AX, r.BX, r.CX, r.DX, r.SI, r.DI, r.BP, r.SP, r.SS, r.DS, r.ES)
		return nil
	case "dsr":
		off, err := hexn(2)
		if err != nil {
			return err
		}
		n, err := hexn(3)
		if err != nil {
			return err
		}
		data := o.Bytes(c.b.DS(uint16(off)), int(n))
		fmt.Printf("DSR %s step=%d off=%04X len=%X sha256=%x\n", f[1], o.Steps(), off, n, sha256.Sum256(data))
		return os.WriteFile(f[1], data, 0o644)
	case "seg":
		// seg:路徑:DS偏移hex（內含 off:seg 遠指標，低字偏移、高字段）:長度hex
		off, err := hexn(2)
		if err != nil {
			return err
		}
		n, err := hexn(3)
		if err != nil {
			return err
		}
		po := o.Word(c.b.DS(uint16(off)))
		ps := o.Word(c.b.DS(uint16(off + 2)))
		data := o.Bytes(oracle.Addr{Seg: ps, Off: po}, int(n))
		fmt.Printf("SEG %s step=%d ptr=%04X:%04X len=%X sha256=%x\n", f[1], o.Steps(), ps, po, n, sha256.Sum256(data))
		return os.WriteFile(f[1], data, 0o644)
	}
	return fmt.Errorf("不認得的動作")
}

// sendKey 送一個鍵。wait 為真時跑到程式讀走它，再多跑 extra 道。
func (c *ctx) sendKey(name string, extra uint64, wait bool) error {
	o := c.o
	before := o.KeysConsumed()
	if name == "*" {
		// 掃描碼表沒有 '*'（要 Shift+8）。遊戲的輸入正規化把小鍵盤 '*' 當 '*'，所以直接寫進 BIOS 鍵盤環：
		// 0040:001A 頭、001C 尾，緩衝 0040:001E..003D。
		if err := pushRaw(o, 0x37, '*'); err != nil {
			return err
		}
	} else if len(name) == 1 {
		if err := o.TypeKeys(name); err != nil {
			return err
		}
	} else if err := o.SendKeys(name); err != nil {
		return err
	}
	if wait {
		err := o.RunUntil(oracle.NewCond("鍵被讀走", func(o *oracle.Oracle) bool {
			return o.KeysConsumed() > before
		}), oracle.Budget(c.budget))
		if err != nil {
			return err
		}
	}
	if extra > 0 {
		return o.Run(extra)
	}
	return nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// pushRaw 把 (掃描碼, ASCII) 直接寫進 BIOS 鍵盤環（0040:001A 頭、001C 尾、緩衝 0040:001E..003D）。
// 環滿（尾 + 2 等於頭）時回錯。
func pushRaw(o *oracle.Oracle, scan, ascii uint8) error {
	const seg = 0x40
	head := o.Word(oracle.Addr{Seg: seg, Off: 0x1A})
	tail := o.Word(oracle.Addr{Seg: seg, Off: 0x1C})
	next := tail + 2
	if next >= 0x3E {
		next = 0x1E
	}
	if next == head {
		return fmt.Errorf("BIOS 鍵盤環已滿")
	}
	o.SetWord(oracle.Addr{Seg: seg, Off: tail}, uint16(scan)<<8|uint16(ascii))
	o.SetWord(oracle.Addr{Seg: seg, Off: 0x1C}, next)
	return nil
}
