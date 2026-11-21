package main

// 批次 094：NPC 路徑、NPC 其他、航海、海戰分類的額外動作。全部只觀測；pokes 與強制呼叫是測試條件，一律寫進事件檔。
//
//	autoret:on|off          在 247AB（等鍵）每次進入時送一個 Return（強制呼叫會顯示訊息窗時用）。
//	dsbase                  記下 DS:0000..EFFF 當比較基線。
//	dsdiff:標籤             與基線比較，事件 x.d = [[偏移, hex]…]（相鄰差異間隔小於 8 合併），並把基線更新成現在。
//	callk:段.偏移:標籤:暫存器[:參數…]   同 callr，但預算 200M 道，呼叫期間 autoret 有效。
//	win:標籤                記一筆視窗 L、T、頁 DS:06CA、BDA6、BDAA、06C8。
//	pokeF:艦隊:偏移hex:HEX:標籤   寫艦隊記錄（DS:2E76 + 133 * 艦隊 + 偏移），測試條件。
//	seqpokeF:IDA:艦隊:偏移hex:標籤:HEX1+HEX2…   IDA 每被觸發一次（去重）寫下一組進艦隊記錄。

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

var autoRet bool
var autoRetInstalled bool
var dsBase []byte

// callGen：每次強制呼叫（callz、callk）加 1。fnpair 的去重只在同一次強制呼叫內有效（連續強制呼叫的暫存器相同，不是中斷返回的重複觸發）。
var callGen int

func (e *env) installAutoRet() {
	if autoRetInstalled {
		return
	}
	autoRetInstalled = true
	o := e.o
	b, _ := CurrentOrMain(o)
	// 247AB 入口會呼叫 231E3 清鍵盤緩衝，所以鍵要等到迴圈頂端 24809（15A1B 輪詢之前）才送。
	d := &dedup{}
	pending := false
	o.OnCall(b.Code(0x247AB), func(o *oracle.Oracle) {
		if d.again(o) {
			return
		}
		pending = autoRet
	})
	o.OnCall(b.Code(0x24809), func(o *oracle.Oracle) {
		if !pending {
			return
		}
		pending = false
		_ = o.SendKeys("Return")
		e.log.Emit(Ev78{Kind: "note", Label: "autoret", IDA: "24809", Step: o.Steps(), Tick: o.Ticks()})
	})
}

func (e *env) act094(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	hexn := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 16, 64) }
	switch f[0] {
	case "autoret":
		e.installAutoRet()
		autoRet = f[1] == "on"
		return nil
	case "dsbase":
		dsBase = append([]byte(nil), o.Bytes(b.DS(0), vmDSSize)...)
		return nil
	case "dsdiff":
		cur := append([]byte(nil), o.Bytes(b.DS(0), vmDSSize)...)
		d := diffExact(dsBase, cur)
		dsBase = cur
		e.log.Emit(Ev78{Kind: "b", Label: "dsdiff:" + f[1], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"d": d}})
		return nil
	case "callk":
		e.installAutoRet()
		autoRet = true
		defer func() { autoRet = false }()
		return e.callrBud(f, 200_000_000)
	case "jset":
		return e.installJSet094(f[1])
	case "callz":
		return e.callrBud(f, 20_000_000)
	case "fnpair":
		return e.fnpair(f)
	case "script":
		return e.script(f[1])
	case "win":
		L := 720*int(u8(o, b, 0x06CA)) + int(u16(o, b, 0xBDA6))
		T := int(u16(o, b, 0xBDAA))
		e.log.Emit(Ev78{Kind: "b", Label: "win:" + f[1], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"L": L, "T": T, "page": int(u8(o, b, 0x06CA)),
			"bda6": int(u16(o, b, 0xBDA6)), "bdaa": int(u16(o, b, 0xBDAA)), "w06c8": int(u16(o, b, 0x06C8))}})
		return nil
	case "pokeF":
		fi, err := strconv.Atoi(f[1])
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
		a := uint16(0x2E76 + 133*fi + int(off))
		old := o.Bytes(b.DS(a), len(data))
		o.SetBytes(b.DS(a), data)
		e.log.Emit(Ev78{Kind: "poke", Label: f[4], Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"fleet": fi, "off": int(a), "old": hex.EncodeToString(old), "new": hex.EncodeToString(data)}})
		return nil
	case "seqpokeF":
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		fi, err := strconv.Atoi(f[2])
		if err != nil {
			return err
		}
		off, err := hexn(3)
		if err != nil {
			return err
		}
		return e.installSeqPoke(uint32(lin), uint16(0x2E76+133*fi+int(off)), strings.Split(f[5], "+"), f[4])
	}
	return e.act092(f)
}

// callrBud 同 callr，預算可指定。
func (e *env) callrBud(f []string, bud uint64) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	seg, off, ok := strings.Cut(f[1], ".")
	if !ok {
		return fmt.Errorf("callk 要用 段.偏移")
	}
	sv, err := strconv.ParseUint(seg, 16, 32)
	if err != nil {
		return err
	}
	ov, err := strconv.ParseUint(off, 16, 32)
	if err != nil {
		return err
	}
	target := oracle.Addr{Seg: b.Seg(uint16(sv)), Off: uint16(ov)}
	regs, err := parseRegs(f[3])
	if err != nil {
		return err
	}
	var args []uint16
	for i := 4; i < len(f); i++ {
		v, err := strconv.ParseUint(f[i], 16, 16)
		if err != nil {
			return err
		}
		args = append(args, uint16(v))
	}
	callGen++
	saved := o.Regs()
	o.SetRegs(regs)
	v, err := o.CallBudget(bud, target, args...)
	o.SetRegs(oracle.CallRegs{AX: saved.AX, BX: saved.BX, CX: saved.CX, DX: saved.DX, SI: saved.SI, DI: saved.DI, BP: saved.BP, DS: saved.DS, ES: saved.ES,
		SetAX: true, SetBX: true, SetCX: true, SetDX: true, SetSI: true, SetDI: true, SetBP: true, SetDS: true, SetES: true})
	if err != nil {
		return err
	}
	e.log.Emit(Ev78{Kind: "call", Label: f[2], IDA: f[1], Step: o.Steps(), Extra: map[string]any{"ret": v, "regs": f[3], "args": f[4:]}})
	return nil
}

var _ = os.Stderr

// diffExact 回傳 cur 相對 prev 逐位元組不同的連續區段（不合併間隔）。
func diffExact(prev, cur []byte) [][2]any {
	var out [][2]any
	n := len(cur)
	i := 0
	for i < n {
		if prev[i] == cur[i] {
			i++
			continue
		}
		s := i
		for i < n && prev[i] != cur[i] {
			i++
		}
		out = append(out, [2]any{s, hex.EncodeToString(cur[s:i])})
	}
	return out
}

// ---------------------------------------------------------------------------------------------------------------------
// fnpair:標籤:入口IDA:出口IDA:AX長度:BX長度:DX長度[:堆疊字數]
//   入口與出口成對的 hook（出口放在函式的 retf 指令位址，AX 是回傳值）。入口記暫存器、堆疊、視窗變數，並把 AX / BX / DX 當作 DS 指標各讀「長度」個 bytes（0 不讀）；
//   出口記 AX（ret），並用入口記下的同一組指標再讀一次（函式內部改過的結構、out 參數）。事件 lab = 標籤_in / 標籤_out，x.m = {AX, BX, DX 的 hex}。
//   取代不可靠的出口暫存器（函式尾端 BX、SI、DI 已被重用）。去重同 hookAt。
func (e *env) fnpair(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	if len(f) < 7 {
		return fmt.Errorf("fnpair:標籤:入口:出口:AX長度:BX長度:DX長度[:堆疊字數]")
	}
	label := f[1]
	ein, err := strconv.ParseUint(f[2], 16, 32)
	if err != nil {
		return err
	}
	eout, err := strconv.ParseUint(f[3], 16, 32)
	if err != nil {
		return err
	}
	var lens [3]int
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseUint(f[4+i], 0, 32)
		if err != nil {
			return err
		}
		lens[i] = int(v)
	}
	stkN := 0
	if len(f) > 7 {
		v, _ := strconv.Atoi(f[7])
		stkN = v
	}
	var ptrs [3]uint16
	var have bool
	win := func(o *oracle.Oracle) map[string]any {
		return map[string]any{"L": 720*int(u8(o, b, 0x06CA)) + int(u16(o, b, 0xBDA6)), "T": int(u16(o, b, 0xBDAA)), "page": int(u8(o, b, 0x06CA)),
			"bda6": int(u16(o, b, 0xBDA6)), "bdaa": int(u16(o, b, 0xBDAA)), "wind": hx(o, b, 0x0CB1, 2)}
	}
	mem := func(o *oracle.Oracle) map[string]string {
		m := map[string]string{}
		names := [3]string{"AX", "BX", "DX"}
		for i := 0; i < 3; i++ {
			if lens[i] > 0 {
				m[names[i]] = hx(o, b, ptrs[i], lens[i])
			}
		}
		return m
	}
	d1, d2 := &dedup{}, &dedup{}
	gen := callGen
	o.OnCall(b.Code(uint32(ein)), func(o *oracle.Oracle) {
		if gen != callGen {
			gen = callGen
			d1, d2 = &dedup{}, &dedup{}
		}
		if d1.again(o) {
			return
		}
		r := o.Regs()
		ptrs = [3]uint16{r.AX, r.BX, r.DX}
		have = true
		x := map[string]any{"win": win(o), "m": mem(o), "ptr": []int{int(r.AX), int(r.BX), int(r.DX)}}
		if stkN > 0 {
			var s []int
			for i := 0; i < stkN; i++ {
				s = append(s, int(o.StackWord(i)))
			}
			x["stk"] = s
		}
		e.log.Emit(Ev78{Kind: "b", Label: label + "_in", IDA: fmt.Sprintf("%05X", ein), Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o), Extra: x})
	})
	o.OnCall(b.Code(uint32(eout)), func(o *oracle.Oracle) {
		if d2.again(o) || !have {
			return
		}
		x := map[string]any{"ret": int(o.Regs().AX), "m": mem(o), "win": win(o)}
		e.log.Emit(Ev78{Kind: "b", Label: label + "_out", IDA: fmt.Sprintf("%05X", eout), Step: o.Steps(), Tick: o.Ticks(), Extra: x})
	})
	return nil
}

// script:路徑
//   讀檔，每行一組動作（以逗號分隔），依序執行；# 開頭的行略過。解決 -do 參數長度的限制。
func (e *env) script(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, ln := range strings.Split(string(data), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		for _, a := range strings.Split(ln, ",") {
			if a == "" {
				continue
			}
			if err := e.act(a); err != nil {
				return fmt.Errorf("script %s：%q：%w", path, a, err)
			}
		}
	}
	return nil
}
