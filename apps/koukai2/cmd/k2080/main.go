// Command k2080 是批次 080（素材與畫面對拍）的工具：讀一份腳本，對原版呼叫它自己的
// 載入與繪圖函式，把畫面矩形或記憶體存成檔，給 koukai2 的 `tools/oracle/080_*.py` 逐項比對解碼素材。
//
// 腳本一行一個動作，欄位以空白分隔，`#` 開頭是註解。數字用 Go 的 ParseUint（0x 開頭是十六進位）。
//
//	prog MAIN|OPEN|END       換算位址用的程式（預設 MAIN；OPEN、END 的 DS 基底不同，PSP 預設 02CF）
//	psp HEX                  覆寫 PSP
//	at IDA                   跑到該程式的 IDA 線性位址（十六進位）
//	run N                    跑 N 道指令
//	key 名字 | type 文字
//	call SEG:OFF [ax=] [bx=] [cx=] [dx=] [si=] [di=] [bp=] [es=] [ds=] [args=a,b,c] [budget=N] [ret=名]
//	                         SEG:OFF 是 listing 的段:位移（IDA 段，十六進位），args 第一個離返回位址最近
//	clear                    sub_107F7 清畫面為 0
//	fill X Y W H C           sub_10906 矩形填色（像素座標，X、W 為 8 的倍數）
//	cap 名 X Y W H           存畫面矩形（遊戲座標，畫面列 = Y + 40）的色號到 <o>/名.bin
//	capfull 名                存 640x480 色號
//	dump 名 ds:OFF LEN       存該程式 DS 的記憶體
//	dump 名 seg:SEG:OFF LEN  存實際段值的記憶體
//	dump 名 lin:HEX LEN      存實際線性位址
//	dump 名 ptr:OFF LEN      DS:OFF 存著遠指標（偏移、段），存它指向的 LEN bytes
//	peek ds:OFF LEN | poke ds:OFF HEX | setw ds:OFF VAL | setb ds:OFF VAL
//	watch IDA 名              數一個位址被執行的次數（去重），`call` 結束時印出並清零
//	hook IDA 名 規格 [上限]    位址被執行時存記憶體。規格 seg:es:off:len 之類見 hookSpec
//	shot 路徑 | idx 路徑 | pal 路徑 | save 路徑 | echo 文字
//
// 不含任何原版檔案，-root 與 -load-state 由玩家自備。
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

const (
	linMain uint32 = 0x48150
	linOpen uint32 = 0x1A0B0
	linEnd  uint32 = 0x18DD0
)

type Bind struct {
	PSP    uint16
	DGroup uint32
}

func (b Bind) loadSeg() uint16 { return b.PSP + 0x10 }
func (b Bind) Code(linear uint32) oracle.Addr {
	off := linear - 0x10000
	return oracle.Addr{Seg: b.loadSeg() + uint16(off>>4), Off: uint16(off & 0xF)}
}
func (b Bind) Seg(idaSeg uint16) uint16 { return idaSeg - 0x1000 + b.loadSeg() }
func (b Bind) DSSeg() uint16            { return b.Seg(uint16(b.DGroup >> 4)) }
func (b Bind) DS(off uint16) oracle.Addr {
	return oracle.Addr{Seg: b.DSSeg(), Off: off}
}

type hookRec struct {
	name string
	n    int
}

type env struct {
	o      *oracle.Oracle
	b      Bind
	prog   string
	out    string
	counts map[string]*int
	order  []string
	hooks  map[string]*hookRec
	rets   map[string]uint32
}

func main() {
	root := flag.String("root", "/orig", "原版素材目錄")
	scratch := flag.String("scratch", "", "可寫的暫存層")
	load := flag.String("load-state", "", "從這個狀態檔接著跑")
	out := flag.String("o", "", "輸出目錄")
	script := flag.String("script", "", "腳本檔")
	do := flag.String("do", "", "以分號分隔的動作（與 -script 擇一或併用，先 -do）")
	flag.Parse()

	o, err := Load(*root)
	if err != nil {
		die(err)
	}
	defer o.Close()
	if *scratch != "" {
		o.SetScratch(*scratch)
	}
	if *load != "" {
		if err := o.LoadStateFile(*load); err != nil {
			die(err)
		}
	}
	if *out != "" {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			die(err)
		}
	}
	e := &env{o: o, b: Bind{PSP: 0x02CF, DGroup: linMain}, prog: "MAIN", out: *out,
		counts: map[string]*int{}, hooks: map[string]*hookRec{}, rets: map[string]uint32{}}
	var lines []string
	if *do != "" {
		lines = append(lines, strings.Split(*do, ";")...)
	}
	if *script != "" {
		f, err := os.Open(*script)
		if err != nil {
			die(err)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		f.Close()
	}
	for n, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if err := e.act(strings.Fields(ln)); err != nil {
			fmt.Printf("第 %d 行 %q：%v\n", n+1, ln, err)
			if m := o.Missing(); len(m) > 0 {
				fmt.Printf("缺檔：%v\n", m)
			}
			fmt.Printf("開過的檔：%v\n", o.Opened())
			os.Exit(1)
		}
	}
	w, h, px := o.Screen()
	fmt.Printf("結束 步數 %d  IP %s  畫面 %dx%d sha256 %x\n", o.Steps(), o.IP(), w, h, sha256.Sum256(px))
	if u := o.Unimplemented(); len(u) > 0 {
		fmt.Printf("未實作：%v\n", u)
	}
	if m := o.Missing(); len(m) > 0 {
		fmt.Printf("缺檔：%v\n", m)
	}
}

func num(s string) (uint64, error) { return strconv.ParseUint(s, 0, 64) }

func hexn(s string) (uint64, error) { return strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 64) }

// 解析 `SEG:OFF`，SEG 是 IDA 段（十六進位）。
func (e *env) farAddr(s string) (oracle.Addr, error) {
	sg, of, ok := strings.Cut(s, ":")
	if !ok {
		return oracle.Addr{}, fmt.Errorf("位址格式 SEG:OFF")
	}
	sv, err := hexn(sg)
	if err != nil {
		return oracle.Addr{}, err
	}
	ov, err := hexn(of)
	if err != nil {
		return oracle.Addr{}, err
	}
	return oracle.Addr{Seg: e.b.Seg(uint16(sv)), Off: uint16(ov)}, nil
}

// 記憶體位置：ds:OFF、seg:SEG:OFF、lin:HEX、ptr:OFF。
func (e *env) memAddr(s string) (oracle.Addr, error) {
	k, rest, _ := strings.Cut(s, ":")
	switch k {
	case "ds":
		v, err := hexn(rest)
		if err != nil {
			return oracle.Addr{}, err
		}
		return e.b.DS(uint16(v)), nil
	case "seg":
		sg, of, _ := strings.Cut(rest, ":")
		sv, err1 := hexn(sg)
		ov, err2 := hexn(of)
		if err1 != nil || err2 != nil {
			return oracle.Addr{}, fmt.Errorf("seg:SEG:OFF")
		}
		return oracle.Addr{Seg: uint16(sv), Off: uint16(ov)}, nil
	case "lin":
		v, err := hexn(rest)
		if err != nil {
			return oracle.Addr{}, err
		}
		return oracle.Addr{Seg: uint16(v >> 4), Off: uint16(v & 0xF)}, nil
	case "r":
		// r:名:OFF，段取先前 call 的回傳值（ret=名）
		nm, of, _ := strings.Cut(rest, ":")
		sv, ok := e.rets[nm]
		if !ok {
			return oracle.Addr{}, fmt.Errorf("沒有 %q", nm)
		}
		ov, err := hexn(of)
		if err != nil {
			return oracle.Addr{}, err
		}
		return oracle.Addr{Seg: uint16(sv), Off: uint16(ov)}, nil
	case "ptr":
		v, err := hexn(rest)
		if err != nil {
			return oracle.Addr{}, err
		}
		p := e.b.DS(uint16(v))
		return oracle.Addr{Seg: e.o.Word(oracle.Addr{Seg: p.Seg, Off: p.Off + 2}), Off: e.o.Word(p)}, nil
	}
	return oracle.Addr{}, fmt.Errorf("位置種類 %q", k)
}

func (e *env) write(name string, data []byte) error {
	p := filepath.Join(e.out, name+".bin")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("  %s %d bytes sha256 %x\n", name, len(data), sha256.Sum256(data))
	return nil
}

func (e *env) callRegs(f []string) (oracle.CallRegs, []uint16, uint64, string, error) {
	r := oracle.CallRegs{DS: e.b.DSSeg(), SetDS: true}
	var args []uint16
	budget := uint64(200_000_000)
	retName := ""
	for _, kv := range f {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return r, nil, 0, "", fmt.Errorf("參數 %q 要 k=v", kv)
		}
		switch k {
		case "args":
			for _, a := range strings.Split(v, ",") {
				if a == "" {
					continue
				}
				n, err := num(a)
				if err != nil {
					return r, nil, 0, "", err
				}
				args = append(args, uint16(n))
			}
			continue
		case "budget":
			n, err := num(v)
			if err != nil {
				return r, nil, 0, "", err
			}
			budget = n
			continue
		case "ret":
			retName = v
			continue
		}
		var n uint64
		var err error
		if strings.HasPrefix(v, "@") {
			rv, ok := e.rets[v[1:]]
			if !ok {
				return r, nil, 0, "", fmt.Errorf("沒有 %q", v)
			}
			n = uint64(rv)
		} else if n, err = num(v); err != nil {
			return r, nil, 0, "", fmt.Errorf("%s: %w", k, err)
		}
		u := uint16(n)
		switch k {
		case "ax":
			r.AX, r.SetAX = u, true
		case "bx":
			r.BX, r.SetBX = u, true
		case "cx":
			r.CX, r.SetCX = u, true
		case "dx":
			r.DX, r.SetDX = u, true
		case "si":
			r.SI, r.SetSI = u, true
		case "di":
			r.DI, r.SetDI = u, true
		case "bp":
			r.BP, r.SetBP = u, true
		case "es":
			r.ES, r.SetES = u, true
		case "ds":
			r.DS, r.SetDS = u, true
		default:
			return r, nil, 0, "", fmt.Errorf("不認得的暫存器 %q", k)
		}
	}
	return r, args, budget, retName, nil
}

// call 呼叫 far 常式。呼叫前記下暫存器，呼叫後寫回（Call 只還原呼叫當下的值，不含我們設的）。
func (e *env) call(a oracle.Addr, f []string) (uint32, error) {
	r, args, budget, retName, err := e.callRegs(f)
	if err != nil {
		return 0, err
	}
	o := e.o
	orig := o.Regs()
	o.SetRegs(r)
	before := o.Steps()
	v, err := o.CallBudget(budget, a, args...)
	o.SetRegs(oracle.CallRegs{
		AX: orig.AX, BX: orig.BX, CX: orig.CX, DX: orig.DX, SI: orig.SI, DI: orig.DI, BP: orig.BP, DS: orig.DS, ES: orig.ES,
		SetAX: true, SetBX: true, SetCX: true, SetDX: true, SetSI: true, SetDI: true, SetBP: true, SetDS: true, SetES: true,
	})
	if err != nil {
		return 0, err
	}
	if retName != "" {
		e.rets[retName] = v
	}
	var cs []string
	for _, n := range e.order {
		if c := *e.counts[n]; c != 0 {
			cs = append(cs, fmt.Sprintf("%s=%d", n, c))
		}
		*e.counts[n] = 0
	}
	fmt.Printf("  call %s 回 %08X 步數 %d %s\n", a, v, o.Steps()-before, strings.Join(cs, " "))
	return v, nil
}

func (e *env) act(f []string) error {
	o := e.o
	need := func(n int) error {
		if len(f) < n {
			return fmt.Errorf("參數不足")
		}
		return nil
	}
	switch f[0] {
	case "prog":
		if err := need(2); err != nil {
			return err
		}
		switch strings.ToUpper(f[1]) {
		case "MAIN":
			e.b.DGroup = linMain
		case "OPEN":
			e.b.DGroup = linOpen
		case "END":
			e.b.DGroup = linEnd
		default:
			return fmt.Errorf("prog MAIN|OPEN|END")
		}
		e.prog = strings.ToUpper(f[1])
	case "psp":
		v, err := hexn(f[1])
		if err != nil {
			return err
		}
		e.b.PSP = uint16(v)
	case "run":
		n, err := num(f[1])
		if err != nil {
			return err
		}
		return o.Run(n)
	case "at":
		lin, err := hexn(f[1])
		if err != nil {
			return err
		}
		target := e.b.Code(uint32(lin)).Linear()
		budget := uint64(2_000_000_000)
		if len(f) > 2 {
			if budget, err = num(f[2]); err != nil {
				return err
			}
		}
		err = o.RunUntil(oracle.NewCond("到達 "+f[1], func(o *oracle.Oracle) bool { return o.IP().Linear() == target }), oracle.Budget(budget))
		fmt.Printf("  at %s 步數 %d\n", f[1], o.Steps())
		return err
	case "key":
		return o.SendKeys(f[1])
	case "type":
		return o.TypeKeys(f[1])
	case "call":
		if err := need(2); err != nil {
			return err
		}
		a, err := e.farAddr(f[1])
		if err != nil {
			return err
		}
		_, err = e.call(a, f[2:])
		return err
	case "clear":
		_, err := e.call(oracle.Addr{Seg: e.b.Seg(0x1000), Off: 0x07F7}, nil)
		return err
	case "fill":
		if err := need(6); err != nil {
			return err
		}
		var v [5]uint64
		for i := 0; i < 5; i++ {
			n, err := num(f[1+i])
			if err != nil {
				return err
			}
			v[i] = n
		}
		_, err := e.call(oracle.Addr{Seg: e.b.Seg(0x1000), Off: 0x0906},
			[]string{fmt.Sprintf("args=%d,%d,%d,%d,%d", v[0]/8, v[1], v[2]/8, v[3], v[4])})
		return err
	case "cap":
		if err := need(6); err != nil {
			return err
		}
		var v [4]int
		for i := 0; i < 4; i++ {
			n, err := num(f[2+i])
			if err != nil {
				return err
			}
			v[i] = int(n)
		}
		w, h, px := o.Screen()
		if w != 640 || h < 440 {
			return fmt.Errorf("畫面 %dx%d 不是模式 12h", w, h)
		}
		data := make([]byte, 0, v[2]*v[3])
		for y := 0; y < v[3]; y++ {
			row := (v[1] + y + 40) * w
			data = append(data, px[row+v[0]:row+v[0]+v[2]]...)
		}
		return e.write(f[1], data)
	case "capfull":
		_, _, px := o.Screen()
		return e.write(f[1], px)
	case "dump":
		if err := need(4); err != nil {
			return err
		}
		a, err := e.memAddr(f[2])
		if err != nil {
			return err
		}
		n, err := num(f[3])
		if err != nil {
			return err
		}
		return e.write(f[1], o.Bytes(a, int(n)))
	case "peek":
		a, err := e.memAddr(f[1])
		if err != nil {
			return err
		}
		n, err := num(f[2])
		if err != nil {
			return err
		}
		fmt.Printf("  步數 %d %s:%04X = % X\n", o.Steps(), f[1], a.Off, o.Bytes(a, int(n)))
	case "poke":
		a, err := e.memAddr(f[1])
		if err != nil {
			return err
		}
		data, err := hex.DecodeString(f[2])
		if err != nil {
			return err
		}
		o.SetBytes(a, data)
		fmt.Printf("  步數 %d poke %s = % X\n", o.Steps(), f[1], data)
	case "setw", "setb":
		a, err := e.memAddr(f[1])
		if err != nil {
			return err
		}
		v, err := num(f[2])
		if err != nil {
			return err
		}
		if f[0] == "setw" {
			o.SetWord(a, uint16(v))
		} else {
			o.SetByte(a, uint8(v))
		}
		fmt.Printf("  步數 %d %s %s = %d\n", o.Steps(), f[0], f[1], v)
	case "getw":
		// getw ds:OFF 名：把字存進 rets，供 setw 之後的腳本回填用（印出）。
		a, err := e.memAddr(f[1])
		if err != nil {
			return err
		}
		v := o.Word(a)
		if len(f) > 2 {
			e.rets[f[2]] = uint32(v)
		}
		fmt.Printf("  getw %s = %04X\n", f[1], v)
	case "setr":
		// setr 名 ds:OFF：把先前 call 的回傳值（ret=名）寫入 DS 字。
		a, err := e.memAddr(f[2])
		if err != nil {
			return err
		}
		v, ok := e.rets[f[1]]
		if !ok {
			return fmt.Errorf("沒有 %q", f[1])
		}
		o.SetWord(a, uint16(v))
		fmt.Printf("  setr %s = %04X → %s\n", f[1], uint16(v), f[2])
	case "watch":
		lin, err := hexn(f[1])
		if err != nil {
			return err
		}
		c := new(int)
		e.counts[f[2]] = c
		e.order = append(e.order, f[2])
		var lastStep uint64 = ^uint64(0)
		o.OnCall(e.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			// 中斷返回會讓同一個位址再觸發一次：同一步數不重複數。
			if o.Steps() == lastStep {
				return
			}
			lastStep = o.Steps()
			*c++
		})
	case "hook":
		// hook IDA 名 規格 [上限]；規格見 hookSpec。
		if err := need(4); err != nil {
			return err
		}
		lin, err := hexn(f[1])
		if err != nil {
			return err
		}
		max := 1
		if len(f) > 4 {
			n, err := num(f[4])
			if err != nil {
				return err
			}
			max = int(n)
		}
		hr := &hookRec{name: f[2]}
		e.hooks[f[2]] = hr
		spec := f[3]
		var lastStep uint64 = ^uint64(0)
		o.OnCall(e.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			if o.Steps() == lastStep || hr.n >= max {
				return
			}
			lastStep = o.Steps()
			a, n, err := e.hookSpec(spec)
			if err != nil {
				fmt.Printf("  hook %s：%v\n", hr.name, err)
				return
			}
			r := o.Regs()
			fmt.Printf("  hook %s #%d 步數 %d AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X ES=%04X DS=%04X → %s %d bytes\n",
				hr.name, hr.n, o.Steps(), r.AX, r.BX, r.CX, r.DX, r.SI, r.DI, r.ES, r.DS, a, n)
			if err := e.write(fmt.Sprintf("%s.%d", hr.name, hr.n), o.Bytes(a, n)); err != nil {
				fmt.Printf("  hook %s：%v\n", hr.name, err)
			}
			hr.n++
		})
	case "pair":
		// pair ENTRY RET 名：OPEN.EXE 的影像解碼包裝（sub_14905、sub_1492D）。
		// 進入時（AX = 資料來源緩衝的基底段，DX = 目的段，堆疊 dword = 影像在項內的位移）算出串流位址，
		// 讀標頭 w、h；返回時（retf 4）傾印目的段 0 起的 4 平面逐 byte 交錯內容（每列 4*ceil(w/8) bytes）。
		if err := need(4); err != nil {
			return err
		}
		ent, err := hexn(f[1])
		if err != nil {
			return err
		}
		ret, err := hexn(f[2])
		if err != nil {
			return err
		}
		name := f[3]
		type pend struct {
			ax, dx, lo, hi, w, h uint16
			stream               oracle.Addr
			step                 uint64
		}
		var cur *pend
		seq := 0
		inOpen := func() bool {
			log := o.ExecLog()
			for i := len(log) - 1; i >= 0; i-- {
				if log[i].TSR || log[i].Ended {
					continue
				}
				return strings.EqualFold(log[i].Base, e.prog+".EXE")
			}
			return false
		}
		var lastE, lastR uint64 = ^uint64(0), ^uint64(0)
		o.OnCall(e.b.Code(uint32(ent)), func(o *oracle.Oracle) {
			if o.Steps() == lastE || !inOpen() {
				return
			}
			lastE = o.Steps()
			r := o.Regs()
			lo, hi := o.StackWord(2), o.StackWord(3)
			dw := uint32(hi)<<16 | uint32(lo)
			seg := uint16(dw>>4) + r.AX
			off := uint16(dw & 0xF)
			st := oracle.Addr{Seg: seg, Off: off}
			hd := o.Bytes(st, 4)
			cur = &pend{ax: r.AX, dx: r.DX, lo: lo, hi: hi, w: uint16(hd[0]) | uint16(hd[1])<<8, h: uint16(hd[2]) | uint16(hd[3])<<8, stream: st, step: o.Steps()}
		})
		o.OnCall(e.b.Code(uint32(ret)), func(o *oracle.Oracle) {
			if o.Steps() == lastR || cur == nil || !inOpen() {
				return
			}
			lastR = o.Steps()
			c := cur
			cur = nil
			rowBytes := 4 * ((int(c.w) + 7) / 8)
			n := rowBytes * int(c.h)
			if n > 0x10000 {
				n = 0x10000
			}
			buf := append([]byte{byte(c.w), byte(c.w >> 8), byte(c.h), byte(c.h >> 8)}, o.Bytes(oracle.Addr{Seg: c.dx, Off: 0}, n)...)
			fmt.Printf("  pair %s #%d 進入步數 %d 返回步數 %d AX=%04X DX=%04X 位移=%04X%04X 串流 %s %dx%d 傾印 %d bytes\n",
				name, seq, c.step, o.Steps(), c.ax, c.dx, c.hi, c.lo, c.stream, c.w, c.h, n)
			if err := e.write(fmt.Sprintf("%s.%03d", name, seq), buf); err != nil {
				fmt.Printf("  pair %s：%v\n", name, err)
			}
			seq++
		})
	case "regat":
		// regat IDA 暫存器=值…：該位址每次被執行前改暫存器（測試條件，收據明列）。
		lin, err := hexn(f[1])
		if err != nil {
			return err
		}
		var r oracle.CallRegs
		for _, kv := range f[2:] {
			k, v, _ := strings.Cut(kv, "=")
			n, err := num(v)
			if err != nil {
				return err
			}
			u := uint16(n)
			switch k {
			case "ax":
				r.AX, r.SetAX = u, true
			case "bx":
				r.BX, r.SetBX = u, true
			case "cx":
				r.CX, r.SetCX = u, true
			case "dx":
				r.DX, r.SetDX = u, true
			default:
				return fmt.Errorf("regat 不支援 %q", k)
			}
		}
		var lastStep uint64 = ^uint64(0)
		o.OnCall(e.b.Code(uint32(lin)), func(o *oracle.Oracle) {
			if o.Steps() == lastStep || !e.curIs("MAIN") {
				return
			}
			lastStep = o.Steps()
			o.SetRegs(r)
			fmt.Printf("  regat %s 步數 %d 設 %v\n", f[1], o.Steps(), f[2:])
		})
	case "untilexec":
		// untilexec 名 [上限]：跑到 EXEC 紀錄出現該程式（例 END.EXE），之後 `psp auto` 取它的 PSP。
		budget := uint64(1_000_000_000)
		if len(f) > 2 {
			n, err := num(f[2])
			if err != nil {
				return err
			}
			budget = n
		}
		err := o.RunUntil(oracle.NewCond("EXEC "+f[1], func(o *oracle.Oracle) bool {
			for _, r := range o.ExecLog() {
				if strings.EqualFold(r.Base, f[1]) && !r.Ended {
					return true
				}
			}
			return false
		}), oracle.Budget(budget))
		fmt.Printf("  untilexec %s 步數 %d\n", f[1], o.Steps())
		return err
	case "pspauto":
		// 取最後一支未結束、非 TSR 程式的 PSP。
		log := o.ExecLog()
		for i := len(log) - 1; i >= 0; i-- {
			if !log[i].TSR && !log[i].Ended {
				e.b.PSP = log[i].PSP
				fmt.Printf("  psp %04X（%s）\n", log[i].PSP, log[i].Base)
				return nil
			}
		}
		return fmt.Errorf("沒有執行中的程式")
	case "playend":
		// playend 鍵 間隔 上限 名：重複 [送鍵、跑 間隔 道]，直到該程式結束或步數用完。
		gap, err := num(f[2])
		if err != nil {
			return err
		}
		budget, err := num(f[3])
		if err != nil {
			return err
		}
		start := o.Steps()
		for o.Steps()-start < budget {
			if err := o.SendKeys(f[1]); err != nil {
				return err
			}
			err := o.Run(gap)
			if err != nil {
				if _, ok := err.(*oracle.ExitError); ok {
					fmt.Printf("  playend 程式結束 步數 %d（用 %d）\n", o.Steps(), o.Steps()-start)
					return nil
				}
				return err
			}
			for _, r := range o.ExecLog() {
				if strings.EqualFold(r.Base, f[4]) && r.Ended {
					fmt.Printf("  playend %s 結束 步數 %d（用 %d）\n", f[4], o.Steps(), o.Steps()-start)
					return nil
				}
			}
		}
		fmt.Printf("  playend 步數用完 %d\n", o.Steps()-start)
	case "spair":
		// spair ENTRY RET 名：進入時記暫存器（AX、BX、DX），返回時（retf）存整個畫面色號（640x480）到 名.NNN.bin。
		// END.EXE 的 sub_12AA4（ENDGRP 影像直接畫到 VRAM，AX = x、DX = y、BX = 項）。
		ent, err := hexn(f[1])
		if err != nil {
			return err
		}
		ret, err := hexn(f[2])
		if err != nil {
			return err
		}
		name := f[3]
		var cur *oracle.Regs
		var curStep uint64
		seq := 0
		var lastE, lastR uint64 = ^uint64(0), ^uint64(0)
		o.OnCall(e.b.Code(uint32(ent)), func(o *oracle.Oracle) {
			if o.Steps() == lastE || !e.curIs("END") {
				return
			}
			lastE = o.Steps()
			r := o.Regs()
			cur = &r
			curStep = o.Steps()
		})
		o.OnCall(e.b.Code(uint32(ret)), func(o *oracle.Oracle) {
			if o.Steps() == lastR || cur == nil || !e.curIs("END") {
				return
			}
			lastR = o.Steps()
			c := cur
			cur = nil
			_, _, px := o.Screen()
			fmt.Printf("  spair %s #%d 進入步數 %d 返回步數 %d AX=%04X BX=%04X DX=%04X\n", name, seq, curStep, o.Steps(), c.AX, c.BX, c.DX)
			if err := e.write(fmt.Sprintf("%s.%03d", name, seq), append([]byte(nil), px...)); err != nil {
				fmt.Printf("  spair %s：%v\n", name, err)
			}
			seq++
		})
	case "epair":
		// epair ENTRY RET 名：END.EXE 的字典式影像解碼（150FC，retf 在 15434）。
		// 進入時堆疊 arg_0 = 串流段、arg_2 = 串流位移、arg_4 = 目的段；讀標頭 w、h；返回時傾印目的段 0 起 4 平面逐 byte 交錯內容。
		ent, err := hexn(f[1])
		if err != nil {
			return err
		}
		ret, err := hexn(f[2])
		if err != nil {
			return err
		}
		name := f[3]
		type pend struct {
			seg, off, dst, w, h uint16
			step                uint64
		}
		var cur *pend
		seq := 0
		var lastE, lastR uint64 = ^uint64(0), ^uint64(0)
		o.OnCall(e.b.Code(uint32(ent)), func(o *oracle.Oracle) {
			if o.Steps() == lastE || !e.curIs("END") {
				return
			}
			lastE = o.Steps()
			seg, off, dst := o.StackWord(2), o.StackWord(3), o.StackWord(4)
			hd := o.Bytes(oracle.Addr{Seg: seg, Off: off}, 4)
			cur = &pend{seg: seg, off: off, dst: dst, w: uint16(hd[0]) | uint16(hd[1])<<8, h: uint16(hd[2]) | uint16(hd[3])<<8, step: o.Steps()}
		})
		o.OnCall(e.b.Code(uint32(ret)), func(o *oracle.Oracle) {
			if o.Steps() == lastR || cur == nil || !e.curIs("END") {
				return
			}
			lastR = o.Steps()
			c := cur
			cur = nil
			rb := 4 * ((int(c.w) + 7) / 8)
			n := rb * int(c.h)
			if n > 0x10000 {
				n = 0x10000
			}
			buf := append([]byte{byte(c.w), byte(c.w >> 8), byte(c.h), byte(c.h >> 8)}, o.Bytes(oracle.Addr{Seg: c.dst, Off: 0}, n)...)
			fmt.Printf("  epair %s #%d 進入步數 %d 返回步數 %d 串流 %04X:%04X 目的段 %04X %dx%d 傾印 %d bytes\n",
				name, seq, c.step, o.Steps(), c.seg, c.off, c.dst, c.w, c.h, n)
			if err := e.write(fmt.Sprintf("%s.%03d", name, seq), buf); err != nil {
				fmt.Printf("  epair %s：%v\n", name, err)
			}
			seq++
		})
	case "shot":
		return o.WritePNG(f[1])
	case "idx":
		_, _, px := o.Screen()
		return os.WriteFile(f[1], px, 0o644)
	case "pal":
		pal := o.Palette()
		raw := make([]byte, 0, 768)
		for _, c := range pal {
			raw = append(raw, c[0], c[1], c[2])
		}
		return os.WriteFile(f[1], raw, 0o644)
	case "save":
		return o.SaveStateFile(f[1])
	case "echo":
		fmt.Println(strings.Join(f[1:], " "))
	case "regs":
		r := o.Regs()
		fmt.Printf("  步數 %d AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X BP=%04X SP=%04X DS=%04X ES=%04X SS=%04X CS:IP=%04X:%04X\n",
			o.Steps(), r.AX, r.BX, r.CX, r.DX, r.SI, r.DI, r.BP, r.SP, r.DS, r.ES, r.SS, r.CS, r.IP)
	default:
		return fmt.Errorf("不認得的動作")
	}
	return nil
}

// hookSpec 解析 hook 的記憶體規格，在 hook 觸發時求值：
//
//	es:DI:LEN   ES:DI 暫存器所指，LEN 十六進位 bytes（也可寫 es:bx、es:si、ds:si、es:ax、es:0）
//	ds:OFF:LEN  DS:OFF
//	ptr:OFF:LEN DS:OFF 的遠指標
//	seg:SEG:OFF:LEN
func (e *env) hookSpec(s string) (oracle.Addr, int, error) {
	f := strings.Split(s, ":")
	r := e.o.Regs()
	regv := map[string]uint16{"ax": r.AX, "bx": r.BX, "cx": r.CX, "dx": r.DX, "si": r.SI, "di": r.DI, "bp": r.BP, "sp": r.SP}
	segv := map[string]uint16{"es": r.ES, "ds": r.DS, "ss": r.SS, "cs": r.CS}
	parse := func(x string) (uint64, error) { return hexn(x) }
	switch f[0] {
	case "es", "ds", "ss", "cs":
		if len(f) == 3 {
			if v, ok := regv[f[1]]; ok {
				n, err := parse(f[2])
				return oracle.Addr{Seg: segv[f[0]], Off: v}, int(n), err
			}
			if f[0] == "ds" {
				of, err := parse(f[1])
				if err != nil {
					return oracle.Addr{}, 0, err
				}
				n, err := parse(f[2])
				return e.b.DS(uint16(of)), int(n), err
			}
			of, err := parse(f[1])
			if err != nil {
				return oracle.Addr{}, 0, err
			}
			n, err := parse(f[2])
			return oracle.Addr{Seg: segv[f[0]], Off: uint16(of)}, int(n), err
		}
	case "ptr":
		if len(f) == 3 {
			a, err := e.memAddr("ptr:" + f[1])
			if err != nil {
				return a, 0, err
			}
			n, err := parse(f[2])
			return a, int(n), err
		}
	case "seg":
		if len(f) == 4 {
			sv, e1 := parse(f[1])
			ov, e2 := parse(f[2])
			n, e3 := parse(f[3])
			if e1 != nil || e2 != nil || e3 != nil {
				return oracle.Addr{}, 0, fmt.Errorf("seg 規格")
			}
			return oracle.Addr{Seg: uint16(sv), Off: uint16(ov)}, int(n), nil
		}
	}
	return oracle.Addr{}, 0, fmt.Errorf("規格 %q", s)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// curIs 回目前執行的程式（EXEC 紀錄最後一支未結束、非 TSR）是否是 name（MAIN、OPEN、END）。
// 沒有 EXEC 紀錄（從狀態檔接續）時只有 MAIN 成立。
func (e *env) curIs(name string) bool {
	log := e.o.ExecLog()
	if len(log) == 0 {
		return strings.EqualFold(name, "MAIN")
	}
	for i := len(log) - 1; i >= 0; i-- {
		if log[i].TSR || log[i].Ended {
			continue
		}
		return strings.EqualFold(log[i].Base, name+".EXE")
	}
	return false
}
