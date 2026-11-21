package main

// 批次 092：劇本 VM 逐道 DS 差異與按鍵餵送。全部只觀測；pokes 與強制呼叫是測試條件，一律寫進事件檔。
//
//	vmhooks               掛劇本 VM 的差異 hook：433DB（vd_in，整個 DS 0..EFFF 當基線）、433F8（vd_op，每道指令前的 DS 差異）、
//	                      4347F（vd_out，433DB 返回）、43510（vd_end，43480 返回，SNR0 狀態重設之後）、43651（vd_hend，435D6 返回）。
//	                      每筆事件 x.pc = DS:0418、x.gs = DS:040C 兩個 byte、x.d = [[偏移, hex]…]（與上一筆的差異，間隔小於 8 的合併）。
//	msg92                 掛 428A8（MSG 展開）與 4377E 的 hook，記堆疊上的訊息編號。
//	vmdrive:結束次數:預算:答案…  讓遊戲自己跑，直到 VM 結束次數（43480 或 435D6 返回）達到或預算用完。
//	                      等鍵時的按鍵：247AB（WAIT_KEY）送 Return；242CD（ASK_YN、MENU 的選擇迴圈）依答案序列取下一個：
//	                      Y = Return、N = Right+Return、M<k> = Down*k+Return。答案用完後送 Return。
//	vmtrig:type:arg:標籤[:預算]  強制呼叫原版的 43511（觸發搜尋）與命中時的 43480（執行 SNR0），不經設施入口與 UI；
//	                      只用在沒有視窗的程式碼。回傳值記成 call 事件。
//	vmtrigh:type:arg:標籤      同上，英雄劇本 43652 與 435D6。
//	setgold / dumpds:…     見各自說明

import (
	"os"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

const vmDSSize = 0xF000

type vmDiffState struct {
	prev []byte
	n    int
}

// diffRuns 回傳 cur 相對 prev 的差異區段（相鄰差異間隔小於 8 的合併）。prev 為 nil 時整段回傳。
func diffRuns(prev, cur []byte) [][2]any {
	var out [][2]any
	if prev == nil {
		return [][2]any{{0, hex.EncodeToString(cur)}}
	}
	n := len(cur)
	i := 0
	for i < n {
		if prev[i] == cur[i] {
			i++
			continue
		}
		s := i
		last := i
		for j := i + 1; j < n && j-last < 8; j++ {
			if prev[j] != cur[j] {
				last = j
			}
		}
		out = append(out, [2]any{s, hex.EncodeToString(cur[s : last+1])})
		i = last + 1
	}
	return out
}

func (e *env) installVMHooks() {
	o := e.o
	b, _ := CurrentOrMain(o)
	st := &vmDiffState{}
	var lastPC uint16 = 0xFFFF
	var lastRegs oracle.Regs
	var lastStep uint64
	var haveLast bool
	hook := func(ida uint32, label string) {
		stt := &HookStat78{}
		e.stats[label] = stt
		o.OnCall(b.Code(ida), func(o *oracle.Oracle) {
			pc := o.Word(b.DS(0x0418))
			r := o.Regs()
			if haveLast && lastPC == pc && lastRegs == r && o.Steps()-lastStep < Dup78Window {
				stt.Dup++
				return
			}
			haveLast = true
			lastPC, lastRegs, lastStep = pc, r, o.Steps()
			stt.Total++
			stt.Logged++
			cur := append([]byte(nil), o.Bytes(b.DS(0), vmDSSize)...)
			d := diffRuns(st.prev, cur)
			st.prev = cur
			st.n++
			e.log.Emit(Ev78{Kind: "b", Label: label, IDA: fmt.Sprintf("%05X", ida), Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o),
				Extra: map[string]any{"pc": int(pc), "gs": hex.EncodeToString(o.Bytes(b.DS(0x040C), 2)), "d": d}})
		})
	}
	hook(0x433DB, "vd_in")
	hook(0x433F8, "vd_op")
	hook(0x4347F, "vd_out")
	hook(0x43510, "vd_end")
	hook(0x43651, "vd_hend")
}

// installMsg92 掛 428A8（MSG 展開）：堆疊 [ip, cs?]… 的訊息編號。428A8 是 near 呼叫（431AB call near ptr sub_428A8），AX = 訊息編號？
// 428AF 把 ax 存起來，4280B 以 bx、AX 取訊息，所以在入口記 AX。
func (e *env) installMsg92() {
	e.hookAt(0x428A8, "m428A8", func(o *oracle.Oracle) map[string]any {
		return map[string]any{"ax": int(o.Regs().AX)}
	})
}

func (e *env) act092(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	num := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 0, 64) }
	hexn := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 16, 64) }
	switch f[0] {
	case "vmhooks":
		e.installVMHooks()
		return nil
	case "msg92":
		e.installMsg92()
		return nil
	case "sel92":
		// sel92：選擇迴圈 242CD 的進入（AL = 模式）與返回（24363，AL = 選項或 0xFF）、23DF0（輸入）的返回。
		e.hookAt(0x242CD, "s_in", func(o *oracle.Oracle) map[string]any {
			return map[string]any{"al": int(o.Regs().AX & 0xFF), "cur": int(o.Word(b.DS(0xA160))), "dis": int(o.Word(b.DS(0xA14C))), "n": int(o.Word(b.DS(0xA158)))}
		})
		e.hookAt(0x24363, "s_out", func(o *oracle.Oracle) map[string]any {
			return map[string]any{"al": int(o.Regs().AX & 0xFF), "cur": int(o.Word(b.DS(0xA160)))}
		})
		return nil
	case "vmdrive":
		return e.vmdrive(f)
	case "setport":
		// setport:變數編號:標籤：DS:0CAE（所在港）低位元組 = SNR0 變數 v[n]（DS:0D68 + 2n）的低位元組。測試條件：不實際航行到該港，
		// 直接把「所在港」改成劇本指定的港，之後從設施入口進入，劇本程式碼與觸發全是原版。記成 poke 事件。
		vn, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			return err
		}
		v := o.Word(b.DS(uint16(0x0D68 + 2*vn)))
		old := o.Bytes(b.DS(0x0CAE), 2)
		o.SetBytes(b.DS(0x0CAE), []byte{byte(v), 0})
		e.log.Emit(Ev78{Kind: "poke", Label: f[2], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": 0x0CAE, "old": hex.EncodeToString(old), "new": hex.EncodeToString([]byte{byte(v), 0}), "from_var": int(vn)}})
		return nil
	case "addday":
		// addday:天數:標籤：DS:0534（日）加上天數（不處理進位到月：日不超過 29）。測試條件：推進日期使期限逾期。
		n, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			return err
		}
		d := o.Byte(b.DS(0x0534))
		old := d
		d += byte(n)
		o.SetByte(b.DS(0x0534), d)
		e.log.Emit(Ev78{Kind: "poke", Label: f[2], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": 0x0534, "old": hex.EncodeToString([]byte{old}), "new": hex.EncodeToString([]byte{d})}})
		return nil
	case "givecargo":
		// givecargo:商品變數:數量變數:差額:標籤：把 ship_load[0] 的第 0 格改成（商品 = v[item] 低位元組、數量 = v[qty] + 差額）。測試條件：
		// 模擬「已經買到的貨」。差額用有號十進位。記成 poke 事件。
		iv, _ := strconv.ParseUint(f[1], 10, 64)
		qv, _ := strconv.ParseUint(f[2], 10, 64)
		delta, _ := strconv.ParseInt(f[3], 10, 64)
		item := byte(o.Word(b.DS(uint16(0x0D68 + 2*iv))))
		q := uint16(int64(o.Word(b.DS(uint16(0x0D68+2*qv)))) + delta)
		old := o.Bytes(b.DS(0x52D4+0x0C), 2)
		oldg := o.Byte(b.DS(0x52D4 + 0x16))
		o.SetByte(b.DS(0x52D4+0x16), item)
		o.SetWord(b.DS(0x52D4+0x0C), q)
		e.log.Emit(Ev78{Kind: "poke", Label: f[4], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": 0x52D4 + 0x0C, "old": hex.EncodeToString(old), "new": hex.EncodeToString([]byte{byte(q), byte(q >> 8)}), "good_old": int(oldg), "good_new": int(item)}})
		return nil
	case "collect":
		// collect:數量變數:v18 新值:每單位金幣:標籤：討債的酒館收款結果（061 第 4.4 節：付款時 v18 = 0、金幣 += 50000 * v19）。
		// 測試條件，不跑酒館的 36246。v18 = SNR0 的 DS:0D8C。
		qv, _ := strconv.ParseUint(f[1], 10, 64)
		nv, _ := strconv.ParseUint(f[2], 10, 64)
		per, _ := strconv.ParseInt(f[3], 10, 64)
		q := int64(o.Word(b.DS(uint16(0x0D68 + 2*qv))))
		g := int64(int32(uint32(o.Word(b.DS(0x12AE))) | uint32(o.Word(b.DS(0x12B0)))<<16))
		oldv18 := o.Word(b.DS(0x0D8C))
		g2 := g + per*q
		o.SetWord(b.DS(0x0D8C), uint16(nv))
		o.SetWord(b.DS(0x12AE), uint16(uint32(g2)))
		o.SetWord(b.DS(0x12B0), uint16(uint32(g2)>>16))
		e.log.Emit(Ev78{Kind: "poke", Label: f[4], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"v18_old": int(oldv18), "v18_new": int(nv), "gold_old": g, "gold_new": g2}})
		return nil
	case "addmonth":
		// addmonth:月數:標籤：DS:0533（月，原始值）加上月數（不處理進位到年：月不超過 11）。測試條件：推進日期使期限逾期。
		n, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			return err
		}
		mo := o.Byte(b.DS(0x0533))
		old := mo
		mo += byte(n)
		o.SetByte(b.DS(0x0533), mo)
		e.log.Emit(Ev78{Kind: "poke", Label: f[2], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": 0x0533, "old": hex.EncodeToString([]byte{old}), "new": hex.EncodeToString([]byte{mo})}})
		return nil
	case "setport_nation":
		// setport_nation:變數:標籤：DS:0CAE 低位元組 = 國家表 nation[v[n]] 的首都港（DS:117A + 32 * 國 + 10）。測試條件（王命 7、8 的目標國首都）。
		vn, _ := strconv.ParseUint(f[1], 10, 64)
		nat := o.Word(b.DS(uint16(0x0D68 + 2*vn))) & 0xFF
		cap := o.Byte(b.DS(uint16(0x117A + 32*uint32(nat) + 10)))
		old := o.Bytes(b.DS(0x0CAE), 2)
		o.SetBytes(b.DS(0x0CAE), []byte{cap, 0})
		e.log.Emit(Ev78{Kind: "poke", Label: f[2], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": 0x0CAE, "old": hex.EncodeToString(old), "new": hex.EncodeToString([]byte{cap, 0}), "nation": int(nat)}})
		return nil
	case "ownports":
		// ownports:目標變數:差額:國:標籤：把港口表 DS:5F54 + 22 * i + 21 的低 3 位元（所屬國）改成：前 K 個港 = 國 f[3]，其餘 = 國 + 1（K = v[目標變數] + 差額，i = 0..99）。
		// 測試條件（王命 9 的同盟港數）。其他位元不動。
		vn, _ := strconv.ParseUint(f[1], 10, 64)
		delta, _ := strconv.ParseInt(f[2], 10, 64)
		nat, _ := strconv.ParseUint(f[3], 10, 64)
		K := int64(o.Word(b.DS(uint16(0x0D68+2*vn)))) + delta
		for i := int64(0); i < 100; i++ {
			a := b.DS(uint16(0x5F54 + 22*i + 21))
			v := o.Byte(a) &^ 7
			if i < K {
				v |= byte(nat)
			} else {
				v |= byte((nat + 1) % 6)
			}
			o.SetByte(a, v)
		}
		e.log.Emit(Ev78{Kind: "poke", Label: f[4], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"own_ports": K, "nation": int(nat)}})
		return nil
	case "giveitem":
		// giveitem:變數:格:標籤：背包第 [格] 格（DS:2E57 + 格）= v[n] 低位元組（王命 11 的指定寶物）。測試條件。
		vn, _ := strconv.ParseUint(f[1], 10, 64)
		slot, _ := strconv.ParseUint(f[2], 10, 64)
		it := byte(o.Word(b.DS(uint16(0x0D68 + 2*vn))))
		a := b.DS(uint16(0x2E57 + slot))
		old := o.Byte(a)
		o.SetByte(a, it)
		e.log.Emit(Ev78{Kind: "poke", Label: f[3], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": 0x2E57 + int(slot), "old": int(old), "new": int(it)}})
		return nil
	case "setfleet":
		// setfleet:人物:變數:標籤：人物 [n] 的 +44（艦隊編號，DS:12B6 + 50 * n + 44）= 10 * v[變數] + 2（該國的艦隊編號 10 * 國 .. 10 * 國 + 9）。測試條件（王命 12 的遭遇對象）。
		pn, _ := strconv.ParseUint(f[1], 10, 64)
		vn, _ := strconv.ParseUint(f[2], 10, 64)
		nat := o.Word(b.DS(uint16(0x0D68 + 2*vn)))
		fid := byte(10*nat + 2)
		a := b.DS(uint16(0x12B6 + 50*pn + 44))
		old := o.Byte(a)
		o.SetByte(a, fid)
		e.log.Emit(Ev78{Kind: "poke", Label: f[3], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": 0x12B6 + 50*int(pn) + 44, "old": int(old), "new": int(fid)}})
		return nil
	case "navhooks":
		// navhooks：海戰 1D62E（選敵方主組）進出、1E5FC（離場）進出的觀測（058）。
		//   sel_in 1D62E 入口：BX = 現行艦記錄、側 = [BX + 0xD] >> 4；sel_out 1D640：AX = 組記錄位址。
		//   leave_in 1E5FC 入口、leave_out 1E615（retf）：現行艦記錄 DS:9BC2、艦記錄 DS:9800 起 0x3C0 bytes、組記錄 DS:97E0 起 32 bytes、占用圖 DS:8E14 起 2496 bytes。
		e.hookAt(0x1D62E, "sel_in", func(o *oracle.Oracle) map[string]any {
			bx := o.Regs().BX
			return map[string]any{"bx": int(bx), "b0d": int(u8(o, b, bx+0x0D)), "cur": int(u16(o, b, 0x9BC2))}
		})
		e.hookAt(0x1D640, "sel_out", func(o *oracle.Oracle) map[string]any {
			return map[string]any{"ax": int(o.Regs().AX)}
		})
		snap := func(o *oracle.Oracle) map[string]any {
			return map[string]any{"cur": int(u16(o, b, 0x9BC2)), "ships": hx(o, b, 0x9800, 0x3C0), "groups": hx(o, b, 0x97E0, 32), "occ": hx(o, b, 0x8E14, 2496)}
		}
		e.hookAt(0x1E5FC, "leave_in", snap)
		e.hookAt(0x1E615, "leave_out", snap)
		return nil
	case "jonly":
		// jonly:L:T（十進位）：只處理視窗原點 = (L, T) 的 2A1E0 進入（見 npc089.go）。
		l, _ := strconv.Atoi(f[1])
		t, _ := strconv.Atoi(f[2])
		jOnlyL, jOnlyT = l, t
		return nil
	case "fgrp":
		return e.fgrp092(f)
	case "setlock":
		// setlock:艦記錄:目標記錄：艦記錄 +0x0C |= 0x80、+6 = 目標記錄位址（十六進位 DS 偏移）。測試條件（鎖定目標）。
		rec, _ := hexn(1)
		tg, _ := hexn(2)
		fl := o.Byte(b.DS(uint16(rec) + 0x0C))
		old6 := o.Word(b.DS(uint16(rec) + 6))
		o.SetByte(b.DS(uint16(rec)+0x0C), fl|0x80)
		o.SetWord(b.DS(uint16(rec)+6), uint16(tg))
		e.log.Emit(Ev78{Kind: "poke", Label: "setlock", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"rec": int(rec), "flags_old": int(fl), "flags_new": int(fl | 0x80), "t6_old": int(old6), "t6_new": int(tg)}})
		return nil
	case "escif":
		return e.escif(f)
	case "esc":
		// esc:次數：送 Escape 若干次（每次之後跑 1M 道指令）。收掉設施預設功能開出來的畫面。
		n, _ := strconv.ParseUint(f[1], 10, 64)
		for i := uint64(0); i < n; i++ {
			if err := o.SendKeys("Escape"); err != nil {
				return err
			}
			if err := o.Run(1_000_000); err != nil {
				return err
			}
		}
		return nil
	case "vmtrig", "vmtrigh":
		ty, err := hexn(1)
		if err != nil {
			return err
		}
		ar, err := hexn(2)
		if err != nil {
			return err
		}
		bud := uint64(200_000_000)
		if len(f) > 4 {
			bud, _ = num(4)
		}
		var srch, exec oracle.Addr
		if f[0] == "vmtrig" {
			srch = oracle.Addr{Seg: b.Seg(0x3E09), Off: 0x5481}
			exec = oracle.Addr{Seg: b.Seg(0x3E09), Off: 0x53F0}
		} else {
			srch = oracle.Addr{Seg: b.Seg(0x3E09), Off: 0x55C2}
			exec = oracle.Addr{Seg: b.Seg(0x3E09), Off: 0x5546}
		}
		saved := o.Regs()
		restore := func() {
			o.SetRegs(oracle.CallRegs{AX: saved.AX, BX: saved.BX, CX: saved.CX, DX: saved.DX, SI: saved.SI, DI: saved.DI, BP: saved.BP, DS: saved.DS, ES: saved.ES,
				SetAX: true, SetBX: true, SetCX: true, SetDX: true, SetSI: true, SetDI: true, SetBP: true, SetDS: true, SetES: true})
		}
		o.SetRegs(oracle.CallRegs{AX: uint16(ty), SetAX: true, DX: uint16(ar), SetDX: true})
		r1, err := o.CallBudget(bud, srch)
		if err != nil {
			restore()
			return err
		}
		var r2 uint32
		hit := (r1 & 0xFFFF) != 0
		if hit {
			r2, err = o.CallBudget(bud, exec)
			if err != nil {
				restore()
				return err
			}
		}
		restore()
		e.log.Emit(Ev78{Kind: "call", Label: f[3], IDA: f[0], Step: o.Steps(), Extra: map[string]any{"type": int(ty), "arg": int(ar), "search": int(r1 & 0xFFFF), "exec": int(r2 & 0xFFFF), "hit": hit}})
		return nil
	}
	return e.act089(f)
}

// vmdrive 的持久狀態（hook 只裝一次）。
type drvState struct {
	installed bool
	waitKind  string
	armStep   uint64
	lastSend  uint64
	ends      uint64
	starts    uint64
	sent      int
}

func (e *env) drvInstall() *drvState {
	if e.drv != nil {
		return e.drv
	}
	o := e.o
	b, _ := CurrentOrMain(o)
	d := &drvState{installed: true}
	e.drv = d
	arm := func(kind string) func(o *oracle.Oracle) {
		return func(o *oracle.Oracle) {
			if d.waitKind != "" {
				return
			}
			d.waitKind = kind
			d.armStep = o.Steps()
		}
	}
	d1, d2, dend := &dedup{}, &dedup{}, &dedup{}
	o.OnCall(b.Code(0x247AB), func(o *oracle.Oracle) {
		if d1.again(o) {
			return
		}
		arm("K")(o)
	})
	o.OnCall(b.Code(0x242CD), func(o *oracle.Oracle) {
		if d2.again(o) {
			return
		}
		arm("S")(o)
	})
	for _, x := range []struct {
		ida  uint32
		kind string
	}{{0x2488F, "K"}, {0x24363, "S"}, {0x15994, "N"}} {
		kind := x.kind
		dx := &dedup{}
		o.OnCall(b.Code(x.ida), func(o *oracle.Oracle) {
			if dx.again(o) {
				return
			}
			if d.waitKind == kind {
				d.waitKind = ""
			}
		})
	}
	dst := &dedup{}
	o.OnCall(b.Code(0x433DB), func(o *oracle.Oracle) {
		if dst.again(o) {
			return
		}
		d.starts++
	})
	d3 := &dedup{}
	o.OnCall(b.Code(0x1592A), func(o *oracle.Oracle) {
		if d3.again(o) {
			return
		}
		arm("N")(o)
	})
	for _, ida := range []uint32{0x43510, 0x43651} {
		o.OnCall(b.Code(ida), func(o *oracle.Oracle) {
			if dend.again(o) {
				return
			}
			d.ends++
		})
	}
	return d
}

// vmdrive:結束次數:預算:答案[:settle]：見檔頭。結束次數是「從現在起再結束幾次」。
func (e *env) vmdrive(f []string) error {
	o := e.o
	ends, err := strconv.ParseUint(f[1], 0, 64)
	if err != nil {
		return err
	}
	bud, err := strconv.ParseUint(f[2], 0, 64)
	if err != nil {
		return err
	}
	var answers []string
	if len(f) > 3 && f[3] != "" && f[3] != "-" {
		answers = strings.Split(f[3], "+")
	}
	settle := uint64(1_200_000)
	if len(f) > 4 {
		settle, _ = strconv.ParseUint(f[4], 0, 64)
	}
	const chunk = 100_000
	d := e.drvInstall()
	target := d.ends + ends
	d.waitKind = ""
	if len(f) > 6 && f[6] != "" && f[6] != "-" {
		// 進場：寫入玩家位置（設施入口正下方一格）、按 Up。市民 NPC 可能站在入口格上擋住移動（座標沒變），
		// 此時等一段時間讓 NPC 走開再按 Up，最多 40 次。位置有變而劇本還沒起來就多等一會兒。每次輸入都相同（Up）。
		posHex, perr := hex.DecodeString(f[6])
		if perr != nil {
			return perr
		}
		b, _ := CurrentOrMain(o)
		o.SetBytes(b.DS(0x0CB4), posHex)
		e.log.Emit(Ev78{Kind: "poke", Label: "place", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": 0x0CB4, "new": f[6]}})
		s0 := d.starts
		tries := 0
		moved := false
		for d.starts == s0 && tries < 40 {
			if err := o.Run(2_000_000); err != nil {
				return err
			}
			before := o.Word(b.DS(0x0CBA))
			if err := o.SendKeys("Up"); err != nil {
				return err
			}
			if err := o.Run(4_000_000); err != nil {
				return err
			}
			tries++
			if d.starts != s0 {
				break
			}
			if o.Word(b.DS(0x0CBA)) != before {
				moved = true
				if err := o.Run(30_000_000); err != nil {
					return err
				}
				break
			}
		}
		e.log.Emit(Ev78{Kind: "note", Label: "enter_tries", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"tries": tries, "moved": moved, "started": d.starts != s0}})
	}
	ai := 0
	start := o.Steps()
	sent := 0
	untilHit := false
	useUntil := false
	if len(f) > 5 && f[5] != "" && f[5] != "-" {
		ua, perr := strconv.ParseUint(f[5], 16, 64)
		if perr != nil {
			return perr
		}
		useUntil = true
		b, _ := CurrentOrMain(o)
		o.OnCall(b.Code(uint32(ua)), func(o *oracle.Oracle) { untilHit = true })
	}
	finished := func() bool {
		if useUntil {
			return untilHit || (ends > 0 && d.ends >= target)
		}
		return d.ends >= target
	}
	for !finished() {
		if o.Steps()-start > bud {
			e.log.Emit(Ev78{Kind: "note", Label: "vmdrive_budget", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"ends": int(d.ends + ends - target), "want": int(ends), "sent": sent, "where": Describe(o)}})
			fmt.Fprintf(os.Stderr, "vmdrive：預算用完，VM 結束 %d / %d 次\n", d.ends+ends-target, ends)
			return nil
		}
		if err := o.Run(chunk); err != nil {
			return err
		}
		if d.waitKind != "" && o.Steps()-d.armStep >= settle {
			var keys []string
			ans := ""
			switch d.waitKind {
			case "N":
				ans = "T0"
				if ai < len(answers) {
					ans = answers[ai]
					ai++
				}
				if strings.HasPrefix(ans, "T") {
					if err := o.TypeKeys(ans[1:]); err != nil {
						return err
					}
					if err := o.Run(300_000); err != nil {
						return err
					}
				}
				keys = []string{"Return"}
			case "K":
				keys = []string{"Return"}
			case "S":
				ans = "Y"
				if ai < len(answers) {
					ans = answers[ai]
					ai++
				}
				switch {
				case ans == "Y":
					keys = []string{"Return"}
				case ans == "N":
					keys = []string{"Right", "Return"}
				case ans == "E":
					keys = []string{"Escape"}
				case strings.HasPrefix(ans, "M"):
					k, _ := strconv.Atoi(ans[1:])
					for i := 0; i < k; i++ {
						keys = append(keys, "Down")
					}
					keys = append(keys, "Return")
				default:
					keys = []string{"Return"}
				}
			}
			kind := d.waitKind
			armS := d.armStep
			d.waitKind = ""
			d.lastSend = o.Steps()
			for _, k := range keys {
				if err := o.SendKeys(k); err != nil {
					return err
				}
				if err := o.Run(300_000); err != nil {
					return err
				}
			}
			e.log.Emit(Ev78{Kind: "note", Label: "vmdrive_key", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"kind": kind, "ans": ans, "keys": strings.Join(keys, "+"), "arm": armS}})
			sent++
		}
	}
	e.log.Emit(Ev78{Kind: "note", Label: "vmdrive_done", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"ends": int(ends), "sent": sent, "answers_used": ai, "answers": len(answers)}})
	return nil
}

// escif:最多次數：VM 結束後設施的預設功能可能開出選單或等鍵（vmdrive 的最後一輪已經掛起等待）。仍有等待時送 Escape 離開（K 型等待送 Return），
// 每次之後跑 3M 道指令；沒有等待就不送。回報送了幾次。
func (e *env) escif(f []string) error {
	o := e.o
	n, _ := strconv.ParseUint(f[1], 10, 64)
	d := e.drvInstall()
	sent := 0
	if err := o.Run(3_000_000); err != nil {
		return err
	}
	for i := uint64(0); i < n && d.waitKind != ""; i++ {
		k := "Escape"
		if d.waitKind == "K" {
			k = "Return"
		}
		kind := d.waitKind
		d.waitKind = ""
		if err := o.SendKeys(k); err != nil {
			return err
		}
		sent++
		if err := o.Run(3_000_000); err != nil {
			return err
		}
		e.log.Emit(Ev78{Kind: "note", Label: "escif_key", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"kind": kind, "key": k}})
	}
	e.log.Emit(Ev78{Kind: "note", Label: "escif", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"sent": sent}})
	return nil
}
