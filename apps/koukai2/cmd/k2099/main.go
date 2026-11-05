// Command k2099 是 koukai2 批次 099（亂數呼叫點補跑：新遊戲初始化、月結關係反應、殖民成長、失散、NPC 任務決策、砲擊動畫、繳獲船）的觀測工具，
// 複製自 k2096（只增不改別的目錄）。
//
// 099 新增動作（只增不改 k2096 的行為）：
//
//	pokeif:IDA:條件:DS偏移hex:位元組hex   到達該位址且暫存器條件全成立時寫入 DS 位元組（條件 REG=HEX，以 / 連接，REG 為 AX BX CX DX SI DI；
//	                                     成立時印 ACT pokeif 行）。測試條件，收據要明列。
//	pokeor:IDA:條件:DS偏移hex:位元組hex   同 pokeif，但 OR 進原值（設旗標位元），印 ACT pokeor
//	fnmif:IDA:條件:標籤:區域[+區域…]       同 fnm（k2096），但只在暫存器條件（REG=HEX/REG=HEX…）全成立時印（例如 BX = 艦隊指標）
//	moveship:艦記錄偏移:x:y               複製自 k2084：海戰艦記錄 +4 的艦位改成 (x, y)，同步占用圖 DS:8E14（測試條件）
//	fnm 新區域 eOFF.LEN                   ES:OFF 起 LEN bytes（十六進位），用在 es 指向遠緩衝時
//
// 以下是 k2096 的說明。
//
// 096 新增動作（只增不改 k2095 的行為）：
//
//	fnm:IDA:標籤:區域[+區域…]        在 hook 當下印暫存器與多個區域（dOFF.LEN = DS:OFF、bLEN = DS:BX、sLEN = DS:SI、iLEN = DS:DI、cOFF.LEN = CS:OFF、
//	                                kN = 堆疊頂起 N 個字、pN.LEN = 堆疊第 N 個字當 DS 偏移印 LEN bytes，十六進位），輸出行首碼 FM2
//	cellsnap:前綴                  每次 1BD3F 進入且 BL >= 0x80（箭頭碼）存畫面色號 <前綴>.<n>.pre，返回（1BE1F）再存 .post，印 CELLSNAP 行
//
// 以下是 k2095 的說明。
// 複製自 k2091（只增不改別的目錄）。
//
// 095 新增動作（只增不改 k2091 的行為）：
//
//	seedonce:IDA:種子[/種子…]       下一次（第 k 次）到達該位址時設亂數種子 DS:BD88，用完解除；seedoff 解除全部
//	drive:停止IDA:詢問IDA:答案組[:鍵]  反覆跑到停止位址或詢問位址（函式進入）；到詢問位址就等輪詢再送答案鍵（組內 + 連接，組間 / 分隔）；
//	                                   3M 道都沒到就送預設鍵（預設 Return）；詢問位址與答案可寫 -
//	keyin:IDA:鍵名 / keys:標籤 / mrel  到達該位址時把鍵（MB = 滑鼠左鍵按下）排進佇列；印佇列未讀與已讀的鍵數；放開滑鼠左鍵
//
// 以下是 k2091 的說明。
// 複製自 k2090（k2090 複製自 k2088 再複製自 k2085，說明見各檔頭），另加的動作寫在本檔 act 函式內標明「091」處。
// k2088 複製自 k2085，另加：
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

// 095：seedonce 的武裝狀態（每個位址只裝一次 hook）。
type seedArmState struct {
	armed bool
	seeds []uint32
	idx   int
}

var seedArm = map[uint32]*seedArmState{}

// 095：keyin 的狀態（每個位址只裝一次 hook）。
type keyInState struct{ key string }

var keyIn = map[uint32]*keyInState{}

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

	// 099：fnmif 設定的下一個 fnm 條件（暫存器 = 值，全部成立才印）。
	nextCond map[string]uint16
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
	case "mdown":
		// 091：mdown:x:y[:跑幾道]  移到 (x, y)、跑指定指令數（預設 2,000,000，讓程式輪詢到新位置）、按下左鍵（不放開）。
		x, _ := num(1)
		y, _ := num(2)
		n := uint64(2_000_000)
		if len(f) > 3 {
			n, _ = num(3)
		}
		if err := o.MoveMouse(int(x), int(y)); err != nil {
			return err
		}
		if err := o.Run(n); err != nil {
			return err
		}
		o.PressButton(0)
		return o.Run(n)
	case "mup":
		// 091：mup[:跑幾道]  放開左鍵，再跑指定指令數（預設 3,000,000）。
		n := uint64(3_000_000)
		if len(f) > 1 {
			n, _ = num(1)
		}
		o.ReleaseButton(0)
		return o.Run(n)
	case "mmove":
		// 091：mmove:x:y[:跑幾道]  移動滑鼠（按著或沒按著都可），跑指定指令數（預設 2,000,000）。
		x, _ := num(1)
		y, _ := num(2)
		n := uint64(2_000_000)
		if len(f) > 3 {
			n, _ = num(3)
		}
		if err := o.MoveMouse(int(x), int(y)); err != nil {
			return err
		}
		return o.Run(n)
	case "mpos":
		// 090：mpos:標籤  印滑鼠座標（驅動夾進範圍之後）、被輪詢幾次、按著幾次。
		x, y := o.Mouse()
		polls, pressed := o.MouseActivity()
		fmt.Printf("MPOS %s step=%d x=%d y=%d polls=%d pressed=%d\n", f[1], o.Steps(), x, y, polls, pressed)
		return nil
	case "mclick":
		// 090：mclick:x:y[:hover:hold:settle]  同 click，但可指定三段指令數（預設同 oracle 的預設）；不因畫面沒變而回錯。
		x, _ := num(1)
		y, _ := num(2)
		var opts []oracle.ClickOpt
		opts = append(opts, oracle.NoCursorWait())
		if len(f) > 5 {
			h, _ := num(3)
			ho, _ := num(4)
			se, _ := num(5)
			opts = append(opts, oracle.Hover(h), oracle.Hold(ho), oracle.Settle(se))
		}
		err := o.Click(int(x), int(y), opts...)
		if _, none := err.(*oracle.NoResponseError); none {
			fmt.Printf("ACT mclick-noresponse %v\n", err)
			return nil
		}
		return err
	case "pokeat":
		// 090：pokeat:IDA:DS偏移hex:位元組hex  每次到達該位址時寫入 DS 位元組（測試條件，收據要明列）。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		off, err := hexn(2)
		if err != nil {
			return err
		}
		data, err := hex.DecodeString(f[3])
		if err != nil {
			return err
		}
		o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			c.flush()
			o.SetBytes(c.b.DS(uint16(off)), data)
			fmt.Printf("ACT pokeat step=%d at=%X DS:%04X = % X\n", o.Steps(), lin, off, data)
		})
		return nil
	case "fnsp":
		// 090：fnsp:IDA:標籤:堆疊字索引:字數  在 hook 當下取堆疊第 i 個字當作 SS 內的指標，印該指標起的 N 個字（選單 2B46E 的項目指標陣列在 st[2]）。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		label := f[2]
		si, err := strconv.Atoi(f[3])
		if err != nil {
			return err
		}
		nw, err := strconv.Atoi(f[4])
		if err != nil {
			return err
		}
		o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			c.flush()
			r := o.Regs()
			p := o.StackWord(si)
			var ws []string
			for i := 0; i < nw; i++ {
				ws = append(ws, fmt.Sprintf("%04X", o.Word(oracle.Addr{Seg: r.SS, Off: p + uint16(2*i)})))
			}
			fmt.Printf("FSP %s step=%d ida=%X AX=%04X BX=%04X ptr=%04X words=%s\n", label, o.Steps(), lin, r.AX, r.BX, p, strings.Join(ws, ","))
		})
		return nil
	case "seedat":
		// 090：seedat:IDA:種子(十進位)  在該位址每次到達時把亂數種子 DS:BD88 設成指定值（rand 呼叫前一刻固定亂數，測試條件，收據要明列）。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		sd, err := strconv.ParseUint(f[2], 0, 32)
		if err != nil {
			return err
		}
		o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			c.flush()
			o.SetBytes(c.b.DS(koukai2.DSSeedLo), []byte{byte(sd), byte(sd >> 8), byte(sd >> 16), byte(sd >> 24)})
			fmt.Printf("ACT seedat step=%d at=%X seed=%08X\n", o.Steps(), lin, uint32(sd))
		})
		return nil
	case "pokeif", "pokeor":
		// 099：pokeif:IDA:條件:DS偏移hex:位元組hex  條件 REG=HEX/REG=HEX…；成立才寫入並印 ACT pokeif。
		// pokeor 同上，但把位元組 OR 進原值（設旗標位元用），印 ACT pokeor。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		type cnd struct {
			reg string
			val uint16
		}
		var conds []cnd
		if f[2] != "-" && f[2] != "" {
			for _, cs := range strings.Split(f[2], "/") {
				rg, vs, ok := strings.Cut(cs, "=")
				if !ok {
					return fmt.Errorf("條件格式 REG=HEX")
				}
				v, err := strconv.ParseUint(vs, 16, 16)
				if err != nil {
					return err
				}
				conds = append(conds, cnd{rg, uint16(v)})
			}
		}
		off, err := hexn(3)
		if err != nil {
			return err
		}
		data, err := hex.DecodeString(f[4])
		if err != nil {
			return err
		}
		o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			r := o.Regs()
			for _, cd := range conds {
				var got uint16
				switch cd.reg {
				case "AX":
					got = r.AX
				case "BX":
					got = r.BX
				case "CX":
					got = r.CX
				case "DX":
					got = r.DX
				case "SI":
					got = r.SI
				case "DI":
					got = r.DI
				default:
					return
				}
				if got != cd.val {
					return
				}
			}
			c.flush()
			if f[0] == "pokeor" {
				old := o.Bytes(c.b.DS(uint16(off)), len(data))
				nw := make([]byte, len(data))
				for i := range data {
					nw[i] = old[i] | data[i]
				}
				o.SetBytes(c.b.DS(uint16(off)), nw)
				fmt.Printf("ACT pokeor step=%d at=%X cond=%s DS:%04X = % X | % X\n", o.Steps(), lin, f[2], off, old, data)
				return
			}
			o.SetBytes(c.b.DS(uint16(off)), data)
			fmt.Printf("ACT pokeif step=%d at=%X cond=%s DS:%04X = % X\n", o.Steps(), lin, f[2], off, data)
		})
		return nil
	case "seedonce":
		// 095：seedonce:IDA:種子(十進位)  下一次到達該位址時把亂數種子 DS:BD88 設成指定值，之後解除（可重新武裝，hook 只裝一次）。
		// 多案例同一容器用：每個案例開頭呼叫一次。測試條件，收據要明列。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		// 種子可用 / 連接多個：第 k 次到達用第 k 個，用完解除。
		var seeds []uint32
		for _, s := range strings.Split(f[2], "/") {
			v, err := strconv.ParseUint(s, 0, 32)
			if err != nil {
				return err
			}
			seeds = append(seeds, uint32(v))
		}
		st, ok := seedArm[uint32(lin)]
		if !ok {
			st = &seedArmState{}
			seedArm[uint32(lin)] = st
			o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
				if !st.armed || st.idx >= len(st.seeds) {
					return
				}
				sd := st.seeds[st.idx]
				st.idx++
				if st.idx >= len(st.seeds) {
					st.armed = false
				}
				c.flush()
				o.SetBytes(c.b.DS(koukai2.DSSeedLo), []byte{byte(sd), byte(sd >> 8), byte(sd >> 16), byte(sd >> 24)})
				fmt.Printf("ACT seedonce step=%d at=%X seed=%08X\n", o.Steps(), lin, sd)
			})
		}
		st.seeds = seeds
		st.idx = 0
		st.armed = true
		return nil
	case "drive":
		// 095：drive:停止IDA:詢問IDA:答案鍵序列[:鍵][:跳過]  反覆「跑到停止位址或詢問位址」：
		// 到詢問位址（函式進入）-> 等輪詢 25 次 -> 依答案鍵序列（以 + 連接，例如 Down+Return）逐鍵送；
		// 3M 道內都沒到 -> 送 Return（鍵可用第 4 欄覆蓋）。到停止位址結束。最多 120 輪。詢問位址可用 - 表示沒有。
		stop, err := hexn(1)
		if err != nil {
			return err
		}
		askLin := uint32(0)
		hasAsk := f[2] != "-"
		if hasAsk {
			v, err := hexn(2)
			if err != nil {
				return err
			}
			askLin = c.b.Code(uint32(v)).Linear()
		}
		stopLin := c.b.Code(uint32(stop)).Linear()
		// 答案：第 k 次詢問用第 k 組（組間以 / 分隔，組內鍵以 + 分隔），用完重複最後一組。
		var answers [][]string
		if len(f) > 3 && f[3] != "" && f[3] != "-" {
			for _, g := range strings.Split(f[3], "/") {
				answers = append(answers, strings.Split(g, "+"))
			}
		}
		askIdx := 0
		key := "Return"
		if len(f) > 4 && f[4] != "" {
			key = f[4]
		}
		for i := 0; i < 120; i++ {
			which := 0
			err := o.RunUntil(oracle.NewCond("drive", func(o *oracle.Oracle) bool {
				l := o.IP().Linear()
				if l == stopLin {
					which = 1
					return true
				}
				if hasAsk && l == askLin {
					which = 2
					return true
				}
				return false
			}), oracle.Budget(3_000_000))
			if err == nil {
				if which == 1 {
					fmt.Printf("ACT drive reached %X round=%d step=%d\n", stop, i, o.Steps())
					return nil
				}
				var ans []string
				if len(answers) > 0 {
					j := askIdx
					if j >= len(answers) {
						j = len(answers) - 1
					}
					ans = answers[j]
				}
				askIdx++
				fmt.Printf("ACT drive ask step=%d n=%d answer=%v\n", o.Steps(), askIdx, ans)
				if err := o.RunUntil(oracle.MousePolled(25), oracle.Budget(c.budget)); err != nil {
					return err
				}
				for ki, k := range ans {
					if err := c.sendKey(k, 0, true); err != nil {
						return err
					}
					if ki < len(ans)-1 {
						if err := o.RunUntil(oracle.MousePolled(8), oracle.Budget(c.budget)); err != nil {
							return err
						}
					}
				}
				continue
			}
			if _, ok := err.(*oracle.BudgetError); !ok {
				return err
			}
			if err := o.SendKeys(key); err != nil {
				return err
			}
		}
		return fmt.Errorf("120 輪後仍未到達 %X", stop)
	case "keyin":
		// 095：keyin:IDA:鍵名  每次到達該位址時把鍵排進佇列（不等讀走、不跑指令）；鍵名 - 表示停用；MB 表示改按下滑鼠左鍵（不放開）。
		// hook 每個位址只裝一次，再次呼叫只改鍵名。用來測「等待中送鍵會不會被讀走」。測試條件，收據要明列。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		st, ok := keyIn[uint32(lin)]
		if !ok {
			st = &keyInState{}
			keyIn[uint32(lin)] = st
			o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
				if st.key == "" || st.key == "-" {
					return
				}
				c.flush()
				if st.key == "MB" {
					o.PressButton(0)
				} else {
					_ = o.SendKeys(st.key)
				}
				fmt.Printf("ACT keyin step=%d at=%X key=%s pending=%d\n", o.Steps(), lin, st.key, o.KeysPending())
			})
		}
		st.key = f[2]
		return nil
	case "fnstr":
		// 095：fnstr:IDA:標籤  在 hook 當下印 DS:BX 起 24 bytes（十六進位）、AX、呼叫端與 DS:0538 起 10 bytes（列表項目編號陣列）。
		// 用在 14B9C（BX = 要印的字串指標）。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		label := f[2]
		o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			c.flush()
			r := o.Regs()
			fmt.Printf("STR %s step=%d ida=%X AX=%04X BX=%04X ret=%05X s=%s list=%s\n", label, o.Steps(), lin, r.AX, r.BX, c.b.ToIDA(o.Caller()),
				hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.DS, Off: r.BX}, 24)), hex.EncodeToString(o.Bytes(c.b.DS(0x538), 10)))
		})
		return nil
	case "keys":
		// 095：keys:標籤  印佇列中還沒被讀走的鍵數與程式已讀走的鍵數。
		fmt.Printf("KEYS %s step=%d pending=%d consumed=%d\n", f[1], o.Steps(), o.KeysPending(), o.KeysConsumed())
		return nil
	case "mrel":
		// 095：mrel  放開滑鼠左鍵（不跑指令）。
		o.ReleaseButton(0)
		return nil
	case "seedoff":
		// 095：seedoff  解除所有 seedonce 的武裝（案例結束時用，避免殘留到下一案例）。
		for _, st := range seedArm {
			st.armed = false
		}
		return nil
	case "keyat":
		// 090：keyat:鍵名:IDA[:跳過次數]  反覆「跑到該位址（上限 3M 道），沒到就送指定的鍵」，最多 60 輪。
		// 同 escat，但鍵可選（訊息等待用 Return，不會誤選取消）。
		lin, err := hexn(2)
		if err != nil {
			return err
		}
		target := c.b.Code(uint32(lin)).Linear()
		skip := uint64(0)
		if len(f) > 3 {
			skip, _ = num(3)
		}
		hits := uint64(0)
		for i := 0; i < 60; i++ {
			err := o.RunUntil(oracle.NewCond("到達 "+f[2], func(o *oracle.Oracle) bool {
				if o.IP().Linear() == target {
					hits++
					return hits > skip
				}
				return false
			}), oracle.Budget(3_000_000))
			if err == nil {
				fmt.Printf("ACT keyat reached %s after %d %s step=%d\n", f[2], i, f[1], o.Steps())
				return nil
			}
			if _, ok := err.(*oracle.BudgetError); !ok {
				return err
			}
			if err := o.SendKeys(f[1]); err != nil {
				return err
			}
		}
		return fmt.Errorf("60 輪 %s 後仍未到達 %s", f[1], f[2])
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
		// 091：倒回後清掉「上一筆亂數記錄」，否則相同種子、相同堆疊的下一案例第一次 rand 會被當成中斷返回造成的重複而漏記。
		c.haveLast = false
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
	case "fnb":
		// 090：fnb:IDA:標籤[:字數]  在 hook 當下印暫存器，以及 SS:BX 起的 N 個字（4377E 轉進 4372A 時 BX 指向訊息參數，預設 6 個字），
		// 並對每個字 w 印 DS:w 與 SS:w 起的 16 bytes（參數是字串指標時可讀到內容）。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		label := f[2]
		nw := 6
		if len(f) > 3 {
			v, err := strconv.Atoi(f[3])
			if err != nil {
				return err
			}
			nw = v
		}
		o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			c.flush()
			r := o.Regs()
			var ws []string
			var dsb, ssb []string
			for i := 0; i < nw; i++ {
				w := o.Word(oracle.Addr{Seg: r.SS, Off: r.BX + uint16(2*i)})
				ws = append(ws, fmt.Sprintf("%04X", w))
				dsb = append(dsb, hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.DS, Off: w}, 16)))
				ssb = append(ssb, hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.SS, Off: w}, 16)))
			}
			fmt.Printf("FB %s step=%d ida=%X AX=%04X BX=%04X SP=%04X ret=%05X args=%s ds=%s ss=%s\n",
				label, o.Steps(), lin, r.AX, r.BX, r.SP, c.b.ToIDA(o.Caller()),
				strings.Join(ws, ","), strings.Join(dsb, "|"), strings.Join(ssb, "|"))
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
	case "moveship":
		// 099（複製自 k2084 的 moveship）：moveship:艦記錄偏移(hex):x:y  把海戰艦記錄 +4 的艦位改成 (x, y)（位置 = y * 52 + x），
		// 並同步占用圖 DS:8E14（舊格清 0xE0 位元，新格設舊格的占用位元）。測試條件，收據明列。
		rec, err := hexn(1)
		if err != nil {
			return err
		}
		x, _ := num(2)
		y, _ := num(3)
		old := o.Word(c.b.DS(uint16(rec) + 4))
		npos := uint16(y*52 + x)
		occOld := o.Byte(c.b.DS(0x8E14 + old))
		occNew := o.Byte(c.b.DS(0x8E14 + npos))
		o.SetByte(c.b.DS(0x8E14+old), occOld&0x1F)
		o.SetByte(c.b.DS(0x8E14+npos), occNew|(occOld&0xE0))
		o.SetWord(c.b.DS(uint16(rec)+4), npos)
		fmt.Printf("ACT moveship step=%d rec=%X old=%d new=%d occ_old=%02X occ_new=%02X\n", o.Steps(), rec, old, npos, occOld, occNew)
		return nil
	case "fnmif":
		// 099：fnmif:IDA:條件:標籤:區域…  同 fnm，但只在暫存器條件（REG=HEX/REG=HEX…）全成立時印。
		if len(f) < 5 {
			return fmt.Errorf("fnmif:IDA:條件:標籤:區域")
		}
		cm := map[string]uint16{}
		for _, cs := range strings.Split(f[2], "/") {
			rg, vs, ok := strings.Cut(cs, "=")
			if !ok {
				return fmt.Errorf("條件格式 REG=HEX")
			}
			v, err := strconv.ParseUint(vs, 16, 16)
			if err != nil {
				return err
			}
			cm[rg] = uint16(v)
		}
		c.nextCond = cm
		return c.act("fnm:" + f[1] + ":" + f[3] + ":" + f[4])
	case "fnm":
		// 096：fnm:IDA:標籤:區域[+區域…]  在 hook 當下印暫存器與多個區域。區域寫法：
		//   dOFF.LEN  DS:OFF 起 LEN bytes（十六進位）；bLEN  DS:BX 起 LEN bytes；sLEN  DS:SI 起；iLEN  DS:DI 起；
		//   cOFF.LEN  CS:OFF 起（程式碼 bytes，同 cs 段）。
		// 輸出行首碼 FM2，欄位 AX BX CX DX SI DI SP ret 與各區域 hex。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		label := f[2]
		regions := strings.Split(f[3], "+")
		cond := c.nextCond
		c.nextCond = nil
		o.OnCall(c.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			r := o.Regs()
			for rg, want := range cond {
				var got uint16
				switch rg {
				case "AX":
					got = r.AX
				case "BX":
					got = r.BX
				case "CX":
					got = r.CX
				case "DX":
					got = r.DX
				case "SI":
					got = r.SI
				case "DI":
					got = r.DI
				}
				if got != want {
					return
				}
			}
			c.flush()
			var parts []string
			for _, rg := range regions {
				if rg == "" {
					continue
				}
				kind := rg[0]
				rest := rg[1:]
				switch kind {
				case 'd', 'c', 'e':
					offs, lens, _ := strings.Cut(rest, ".")
					off, _ := strconv.ParseUint(offs, 16, 32)
					n, _ := strconv.ParseUint(lens, 16, 32)
					if kind == 'd' {
						parts = append(parts, fmt.Sprintf("%s=%s", rg, hex.EncodeToString(o.Bytes(c.b.DS(uint16(off)), int(n)))))
					} else if kind == 'e' {
						parts = append(parts, fmt.Sprintf("%s=%s", rg, hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.ES, Off: uint16(off)}, int(n)))))
					} else {
						parts = append(parts, fmt.Sprintf("%s=%s", rg, hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.CS, Off: uint16(off)}, int(n)))))
					}
				case 'k':
					// kN：堆疊頂起 N 個字（十六進位個數）
					n, _ := strconv.ParseUint(rest, 16, 32)
					var ws []string
					for i := 0; i < int(n); i++ {
						ws = append(ws, fmt.Sprintf("%04x", o.StackWord(i)))
					}
					parts = append(parts, fmt.Sprintf("%s=%s", rg, strings.Join(ws, ",")))
				case 'p':
					// pN.LEN：堆疊第 N 個字（十六進位）當 DS 偏移，印 DS 起 LEN bytes（十六進位）
					ns, lens, _ := strings.Cut(rest, ".")
					ni, _ := strconv.ParseUint(ns, 16, 32)
					n, _ := strconv.ParseUint(lens, 16, 32)
					w := o.StackWord(int(ni))
					parts = append(parts, fmt.Sprintf("%s=%04x:%s", rg, w, hex.EncodeToString(o.Bytes(c.b.DS(w), int(n)))))
				case 'b', 's', 'i':
					n, _ := strconv.ParseUint(rest, 16, 32)
					off := map[byte]uint16{'b': r.BX, 's': r.SI, 'i': r.DI}[kind]
					parts = append(parts, fmt.Sprintf("%s=%s", rg, hex.EncodeToString(o.Bytes(c.b.DS(off), int(n)))))
				}
			}
			fmt.Printf("FM2 %s step=%d ida=%X AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X SP=%04X ret=%05X %s\n",
				label, o.Steps(), lin, r.AX, r.BX, r.CX, r.DX, r.SI, r.DI, r.SP, c.b.ToIDA(o.Caller()), strings.Join(parts, " "))
		})
		return nil
	case "cellsnap":
		// 096：cellsnap:前綴  每次 1BD3F 進入且 BL >= 0x80（箭頭碼）時存當下畫面色號（640x480 每點 1 byte）為 <前綴>.<n>.pre，
		// 該次返回（1BE1F，retf）時再存 <前綴>.<n>.post，並印 CELLSNAP n step col row code。
		prefix := f[1]
		n := 0
		var pre []byte
		armed := false
		var col, row, code uint16
		var preStep uint64
		o.OnCall(c.b.Code(0x1BD3F), func(o *oracle.Oracle) {
			if armed {
				return
			}
			r := o.Regs()
			if r.BX&0xFF < 0x80 {
				return
			}
			_, _, px := o.Screen()
			pre = append(pre[:0], px...)
			armed, col, row, code, preStep = true, r.AX, r.DX, r.BX&0xFF, o.Steps()
		})
		o.OnCall(c.b.Code(0x1BE1F), func(o *oracle.Oracle) {
			if !armed {
				return
			}
			armed = false
			_, _, px := o.Screen()
			if err := os.WriteFile(fmt.Sprintf("%s.%d.pre", prefix, n), pre, 0o644); err != nil {
				fmt.Printf("ACT cellsnap-error %v\n", err)
				return
			}
			if err := os.WriteFile(fmt.Sprintf("%s.%d.post", prefix, n), px, 0o644); err != nil {
				fmt.Printf("ACT cellsnap-error %v\n", err)
				return
			}
			fmt.Printf("CELLSNAP %d step=%d end=%d col=%d row=%d code=%02X\n", n, preStep, o.Steps(), col, row, code)
			n++
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
	case "stubseq":
		// 091：stubseq:IDA:v1/v2/v3…  far 常式依序回 v1、v2、…，用完後一直回最後一個（十進位）。測試條件，收據要明列。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		var vals []uint32
		for _, s := range strings.Split(f[2], "/") {
			v, err := strconv.ParseUint(s, 0, 32)
			if err != nil {
				return err
			}
			vals = append(vals, uint32(v))
		}
		idx := 0
		o.Stub(c.b.Code(uint32(lin)), func(*oracle.Oracle) uint32 {
			v := vals[idx]
			if idx < len(vals)-1 {
				idx++
			}
			return v
		})
		fmt.Printf("ACT stubseq %X = %v\n", lin, vals)
		return nil
	case "unstub":
		// 091：unstub:IDA  移除該位址的 stub。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		o.Stub(c.b.Code(uint32(lin)), nil)
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
		v, err := o.CallBudget(200_000_000, target, args...)
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
