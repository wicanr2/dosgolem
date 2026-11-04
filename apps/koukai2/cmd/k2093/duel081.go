package main

// 批次 081 項 1、2：決鬥（059）與海戰玩家方（058）的觀測與操作。
// hook 只觀測。動作 moveship、duelrounds、rets、openrect 是測試條件或送鍵序列，收據與腳本要列出。

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

// hookAt 在 MAIN.EXE 的 IDA 位址掛事件 hook，去重後寫成 b 類事件。
func (e *env) hookAt(ida uint32, label string, fn func(o *oracle.Oracle) map[string]any) {
	o := e.o
	b, _ := CurrentOrMain(o)
	d := &dedup{}
	st := &HookStat78{}
	e.stats[label] = st
	o.OnCall(b.Code(ida), func(o *oracle.Oracle) {
		if d.again(o) {
			st.Dup++
			return
		}
		st.Total++
		st.Logged++
		e.log.Emit(Ev78{Kind: "b", Label: label, IDA: fmt.Sprintf("%05X", ida), Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o), Extra: fn(o)})
	})
}

func gold32(o *oracle.Oracle, b Bind) int {
	return int(int32(uint32(u16(o, b, 0x12AE)) | uint32(u16(o, b, 0x12B0))<<16))
}

// installDuel081 掛決鬥與玩家主動攻擊相關的 hook（接在 bhooks 之後使用，bhooks 已掛 22BBA、22998、2231F、22A04、22A12、22A93）。
func (e *env) installDuel081() {
	b, _ := CurrentOrMain(e.o)
	ssWordAt := func(o *oracle.Oracle, off uint16) uint16 {
		r := o.Regs()
		return o.Word(oracle.Addr{Seg: r.SS, Off: r.BP + off})
	}
	// 21424：玩家側本輪出招函式入口（AX = 輪）。此時 AI 側（212B2）已出招，hist 已更新。
	e.hookAt(0x21424, "duel_pmove", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"round": int(o.Regs().AX), "side": int(u16(o, b, 0x9D4E)), "player_side": int(u16(o, b, 0x9D48)),
			"hist0": int(u8(o, b, 0x9D4A)), "hist1": int(u8(o, b, 0x9D4B)), "meter": int(u8(o, b, 0x9C2A))}
	})
	// 212B2：AI 出招函式入口。此時 word_51E9E 已設為本輪出手側，hist 尚未更新。
	e.hookAt(0x212B2, "duel_ai", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"side": int(u16(o, b, 0x9D4E)), "player_side": int(u16(o, b, 0x9D48)),
			"hist0": int(u8(o, b, 0x9D4A)), "hist1": int(u8(o, b, 0x9D4B)), "equip": hx(o, b, 0xA01E, 14)}
	})
	// 22B48：22A93（決鬥經驗）的 retf。
	e.hookAt(0x22B48, "duel_exp_ret", func(o *oracle.Oracle) map[string]any {
		hero := u16(o, b, 0xC2CE)
		return map[string]any{"hero_ptr": int(hero), "hero": hx(o, b, hero, 50), "meter": int(u8(o, b, 0x9C2A))}
	})
	// 22C28：22A93 返回之後、戰果碼判定之前（BP-2 是 22BBA 的模式）。
	e.hookAt(0x22C28, "duel_post_exp", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"mode": int(ssWordAt(o, 0xFFFE)), "meter": int(u8(o, b, 0x9C2A)), "result": int(u8(o, b, 0x9BFB)), "tgt_ptr": int(o.Regs().SI)}
	})
	// 22C66：戰果碼判定之後。
	e.hookAt(0x22C66, "duel_result", func(o *oracle.Oracle) map[string]any {
		tgt := o.Regs().SI
		return map[string]any{"mode": int(ssWordAt(o, 0xFFFE)), "meter": int(u8(o, b, 0x9C2A)), "result": int(u8(o, b, 0x9BFB)), "tgt_ptr": int(tgt),
			"tgt_sailors": int(u16(o, b, u16(o, b, tgt+2)))}
	})
	// 1E96F、1E9BB、1E9D9：1E95E 的決鬥觸發判定。
	e.hookAt(0x1E96F, "trig_flag", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"grp_flag": int(o.Word(oracle.Addr{Seg: b.DS(0).Seg, Off: u16(o, b, 0x9BC0)})), "cur": int(u16(o, b, 0x9BC2)), "tgt": int(o.Regs().SI)}
	})
	e.hookAt(0x1E9BB, "trig_sword", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"di": int(o.Regs().DI), "hero_sword": int(o.Regs().AX & 0xFF), "hero_ptr": int(o.Regs().BX)}
	})
	e.hookAt(0x1E9D9, "trig_luck", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"luck": int(o.Regs().AX & 0xFF), "rand_sailors": int(o.Regs().DI)}
	})
	// 1ABEF 玩家主動決鬥邀請。BX = 目標艦記錄。
	e.hookAt(0x1ABEF, "invite", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"tgt": shipSnap(o, b, o.Regs().BX), "att": shipSnap(o, b, u16(o, b, 0x9BC2))}
	})
	e.hookAt(0x1AC23, "invite_ans", func(o *oracle.Oracle) map[string]any { return map[string]any{"ax": int(o.Regs().AX)} })
	e.hookAt(0x1AC4B, "invite_cmp", func(o *oracle.Oracle) map[string]any {
		tslot := u16(o, b, ssWordAt(o, 0xFFFE)+2)
		return map[string]any{"lhs": int(o.Regs().AX), "tgt_sailors": int(u16(o, b, tslot)), "hero_luck": int(u8(o, b, u16(o, b, 0xC2CE)+0x23))}
	})
	// 1AC7D：玩家攻擊指令入口；1BB0B：目標選擇；1BCA4：是否確認。
	e.hookAt(0x1AC7D, "p_attack", func(o *oracle.Oracle) map[string]any { return map[string]any{} })
	e.hookAt(0x1BB0B, "p_pick", func(o *oracle.Oracle) map[string]any { return map[string]any{} })
	e.hookAt(0x1BB79, "p_pick_ok", func(o *oracle.Oracle) map[string]any { return map[string]any{"si": int(o.Regs().SI)} })
	e.hookAt(0x1AD04, "p_fire", func(o *oracle.Oracle) map[string]any { return map[string]any{"tgt": shipSnap(o, b, o.Regs().BX)} })
	e.hookAt(0x1AD17, "p_melee", func(o *oracle.Oracle) map[string]any { return map[string]any{"tgt": shipSnap(o, b, o.Regs().BX)} })
	// 1A35E：每回合結束檢查（旗艦存活與戰果碼）。
	e.hookAt(0x1A35E, "turn_end_check", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"result": int(u8(o, b, 0x9BFB))}
	})
}

// act081 處理 081 新增的動作。
func (e *env) act081(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	num := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 0, 64) }
	hexn := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 16, 64) }
	switch f[0] {
	case "setal":
		// setal:IDA:HEX：到該位址時把 AL 換成 HEX（一次）。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		v, err := hexn(2)
		if err != nil {
			return err
		}
		e.installSetAL(uint32(lin), uint8(v))
		return nil
	case "seqpoke":
		// seqpoke:IDA:DS偏移:標籤:HEX1+HEX2+…：IDA 每被觸發一次寫下一組（十六進位）進 DS:偏移。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		off, err := hexn(2)
		if err != nil {
			return err
		}
		return e.installSeqPoke(uint32(lin), uint16(off), strings.Split(f[4], "+"), f[3])
	case "duelhooks":
		e.installDuel081()
		return nil
	case "moveship":
		// moveship:艦記錄偏移(hex):x:y：把艦記錄 +4 的艦位改成 (x,y)，並同步占用圖 DS:8E14（舊格清 0xE0 位元，新格設舊格的占用位元）。
		// 這是測試條件：只改艦位與占用圖，不經過任何被驗證的函式。
		rec, err := hexn(1)
		if err != nil {
			return err
		}
		x, _ := num(2)
		y, _ := num(3)
		old := o.Word(b.DS(uint16(rec) + 4))
		npos := uint16(y*52 + x)
		occOld := o.Byte(b.DS(0x8E14 + old))
		occNew := o.Byte(b.DS(0x8E14 + npos))
		o.SetByte(b.DS(0x8E14+old), occOld&0x1F)
		o.SetByte(b.DS(0x8E14+npos), occNew|(occOld&0xE0))
		o.SetWord(b.DS(uint16(rec)+4), npos)
		e.log.Emit(Ev78{Kind: "poke", Label: "moveship", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{
			"rec": int(rec), "old_pos": int(old), "new_pos": int(npos), "occ_old_before": int(occOld), "occ_new_before": int(occNew)}})
		return nil
	case "rets":
		// rets:N:間隔：送 N 次 Return，每次之後跑間隔道指令（單純送鍵序列，不等條件）。
		n, _ := num(1)
		gap, _ := num(2)
		for i := uint64(0); i < n; i++ {
			if err := o.SendKeys("Return"); err != nil {
				return err
			}
			if err := o.Run(gap); err != nil {
				return err
			}
		}
		return nil
	case "keyseq":
		// keyseq:間隔:名字+名字+…：每個鍵之後跑間隔道指令。
		gap, _ := num(1)
		for _, k := range strings.Split(f[2], "+") {
			if err := o.SendKeys(k); err != nil {
				return err
			}
			if err := o.Run(gap); err != nil {
				return err
			}
		}
		return nil
	case "duelrounds":
		// duelrounds:最多輪數:招式字串[:畫面延遲]
		// 招式字串每個字元一輪（0..2 = 選單第幾項），不足補 0。流程：跑到 21424（玩家側本輪出招）→ 等畫面（預設 3M）→
		// Down x 招式 → Return；跑到 22A92（22998 的 retf）為止。
		maxN, _ := num(1)
		moves := f[2]
		delay := uint64(3_000_000)
		if len(f) > 3 {
			delay, _ = num(3)
		}
		t21424 := b.Code(0x21424).Linear()
		t22A92 := b.Code(0x22A92).Linear()
		cond := oracle.NewCond("到達 21424 或 22A92", func(o *oracle.Oracle) bool {
			l := o.IP().Linear()
			return l == t21424 || l == t22A92
		})
		rounds := 0
		ended := false
		for i := uint64(0); i < maxN; i++ {
			if err := o.RunUntil(cond, oracle.Budget(e.max)); err != nil {
				e.log.Emit(Ev78{Kind: "note", Label: "duelrounds_err", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"err": err.Error(), "rounds": rounds, "where": Describe(o)}})
				return err
			}
			if o.IP().Linear() == t22A92 {
				ended = true
				break
			}
			if err := o.Run(delay); err != nil {
				return err
			}
			m := 0
			if int(i) < len(moves) {
				m = int(moves[i] - '0')
			}
			for k := 0; k < m; k++ {
				if err := o.SendKeys("Down"); err != nil {
					return err
				}
				if err := o.Run(2_000_000); err != nil {
					return err
				}
			}
			if err := o.SendKeys("Return"); err != nil {
				return err
			}
			rounds++
			if err := o.Run(1000); err != nil {
				return err
			}
		}
		if !ended {
			// 最後一輪之後再等 22A92
			err := o.RunUntil(oracle.NewCond("到達 22A92", func(o *oracle.Oracle) bool { return o.IP().Linear() == t22A92 }), oracle.Budget(e.max))
			ended = err == nil
		}
		e.log.Emit(Ev78{Kind: "note", Label: "duelrounds", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"rounds": rounds, "ended": ended, "moves": moves}})
		return nil
	case "wait":
		// wait:IDA:預算：同 tryat 但不記 note（避免 note 太多）；到不了回傳錯誤。
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		bud, _ := num(2)
		target := b.Code(uint32(lin)).Linear()
		return o.RunUntil(oracle.NewCond("到達 "+f[1], func(o *oracle.Oracle) bool { return o.IP().Linear() == target }), oracle.Budget(bud))
	case "regs":
		// regs：記一筆目前暫存器與 IDA 位址。
		e.log.Emit(Ev78{Kind: "note", Label: "regs", Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o), Extra: map[string]any{"where": Describe(o)}})
		return nil
	}
	return e.act084(f)
}

// installSetAL 在 IDA 位址掛 hook，把 AL 改成 v（只改一次）。用在 sub_2764F 的參數：
// 港口迴圈以 poke DS:03EA 非 0 離開並呼叫 sub_2764F(byte_4853A)，這裡把參數換成要驗的結局碼（0..5 與 0x0F）。
func (e *env) installSetAL(ida uint32, v uint8) {
	o := e.o
	b, _ := CurrentOrMain(o)
	done := false
	o.OnCall(b.Code(ida), func(o *oracle.Oracle) {
		if done {
			return
		}
		done = true
		r := o.Regs()
		o.SetRegs(oracle.CallRegs{AX: (r.AX & 0xFF00) | uint16(v), SetAX: true})
		e.log.Emit(Ev78{Kind: "poke", Label: "setal", IDA: fmt.Sprintf("%05X", ida), Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"old_AL": int(r.AX & 0xFF), "new_AL": int(v)}})
	})
}

// installSeqPoke 在 IDA 位址每被觸發一次（去重）就把下一組 HEX 寫進 DS:OFF（寫完最後一組後不再動）。
// 用在二十一點：3E0B6（洗牌入口）寫種子 DS:BD88，使牌堆可預測；3E725（一局入口）寫金幣。是測試條件，每次都記 poke 事件。
func (e *env) installSeqPoke(ida uint32, off uint16, vals []string, label string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	var datas [][]byte
	for _, v := range vals {
		d, err := hex.DecodeString(v)
		if err != nil {
			return err
		}
		datas = append(datas, d)
	}
	d := &dedup{}
	idx := 0
	o.OnCall(b.Code(ida), func(o *oracle.Oracle) {
		if d.again(o) {
			return
		}
		if idx >= len(datas) {
			return
		}
		old := o.Bytes(b.DS(off), len(datas[idx]))
		o.SetBytes(b.DS(off), datas[idx])
		e.log.Emit(Ev78{Kind: "poke", Label: label, IDA: fmt.Sprintf("%05X", ida), Step: o.Steps(), Tick: o.Ticks(),
			Extra: map[string]any{"i": idx, "off": int(off), "old": hex.EncodeToString(old), "new": hex.EncodeToString(datas[idx])}})
		idx++
	})
	return nil
}
