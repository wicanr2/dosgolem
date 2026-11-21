package main

// 批次 084：畫面差異擷取。
//
// capcall:ENTRY:RET1[+RET2…]:名稱:最多筆數[:堆疊字數]
//
//	ENTRY 被執行時（去重）記畫面、暫存器與堆疊；RET 之一被執行時（函式的 retf 或 retn 位址，
//	程式碼已經畫完）記畫面並寫出差異檔 <capdir>/<名稱>.<NNN>.bin，同時寫一筆 kind=cap 事件。
//	差異檔格式（小端）：x0 y0 w h（各 u16，畫面座標，列 0..479；整個畫面的變動外框，沒有變動 w=h=0），
//	接著 w*h bytes 呼叫前色號、w*h bytes 呼叫後色號。
//	只觀測，不改任何狀態。位址是 MAIN.EXE 的 IDA 線性位址。
//
// capdir:路徑   設定差異檔與 capshot 的輸出目錄（預設 /out/oracle/084/cap）。
// capshot:名稱  存目前整個畫面（640x480 色號）到 <capdir>/<名稱>.idx。
// capoff:名稱   停止該名稱的擷取（之後的觸發不記）。
// capstat       印每個名稱的進入、返回與寫出筆數。

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

type capState struct {
	dir   string
	hooks map[string]*capHook
}

type capHook struct {
	name    string
	max     int
	nstk    int
	enter   int
	ret     int
	written int
	off     bool
	fidx    int    // 篩選：堆疊第 fidx 個字等於 DS:foff 的字才記錄（fidx < 0 不篩）
	foff    uint16
	cur     *capPend
}

type capPend struct {
	screen []byte
	regs   oracle.Regs
	stack  []uint16
	step   uint64
}

func (e *env) capInit() {
	if e.cap == nil {
		e.cap = &capState{dir: "/out/oracle/084/cap", hooks: map[string]*capHook{}}
	}
}

func screenCopy(o *oracle.Oracle) (int, int, []byte) {
	w, h, px := o.Screen()
	return w, h, append([]byte(nil), px...)
}

// act084 處理 084 新增的動作。
func (e *env) act084(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	e.capInit()
	switch f[0] {
	case "capmem", "capmemds":
		return e.capMem(f)
	case "capshotat":
		return e.capShotAt(f)
	case "capregion":
		return e.capRegion(f)
	case "worldwin":
		return e.worldWin(f)
	case "capdir":
		e.cap.dir = f[1]
		return os.MkdirAll(f[1], 0o755)
	case "capshot":
		if err := os.MkdirAll(e.cap.dir, 0o755); err != nil {
			return err
		}
		_, _, px := o.Screen()
		return os.WriteFile(filepath.Join(e.cap.dir, f[1]+".idx"), px, 0o644)
	case "capoff":
		if h := e.cap.hooks[f[1]]; h != nil {
			h.off = true
		}
		return nil
	case "capstat":
		for n, h := range e.cap.hooks {
			e.log.Emit(Ev78{Kind: "capstat", Label: n, Step: o.Steps(), Extra: map[string]any{"enter": h.enter, "ret": h.ret, "written": h.written}})
		}
		return nil
	case "capcall":
		if len(f) < 5 {
			return fmt.Errorf("capcall:ENTRY:RET:名稱:最多筆數[:堆疊字數]")
		}
		ent, err := strconv.ParseUint(f[1], 16, 64)
		if err != nil {
			return err
		}
		var rets []uint64
		for _, r := range strings.Split(f[2], "+") {
			v, err := strconv.ParseUint(r, 16, 64)
			if err != nil {
				return err
			}
			rets = append(rets, v)
		}
		mx, err := strconv.Atoi(f[4])
		if err != nil {
			return err
		}
		nstk := 12
		if len(f) > 5 {
			nstk, _ = strconv.Atoi(f[5])
		}
		h := &capHook{name: f[3], max: mx, nstk: nstk, fidx: -1}
		if len(f) > 6 {
			// 篩選 w<索引>=<DS偏移hex>，例 w8=3EE
			var fi, fo int
			if _, err := fmt.Sscanf(f[6], "w%d=%x", &fi, &fo); err != nil {
				return err
			}
			h.fidx, h.foff = fi, uint16(fo)
		}
		e.cap.hooks[h.name] = h
		if err := os.MkdirAll(e.cap.dir, 0o755); err != nil {
			return err
		}
		d := &dedup{}
		o.OnCall(b.Code(uint32(ent)), func(o *oracle.Oracle) {
			if h.off || h.written >= h.max {
				return
			}
			if d.again(o) {
				return
			}
			if h.fidx >= 0 && o.StackWord(h.fidx) != o.Word(b.DS(h.foff)) {
				return
			}
			h.enter++
			_, _, scr := screenCopy(o)
			p := &capPend{screen: scr, regs: o.Regs(), step: o.Steps()}
			for i := 0; i < h.nstk; i++ {
				p.stack = append(p.stack, o.StackWord(i))
			}
			h.cur = p
		})
		for _, rv := range rets {
			rd := &dedup{}
			o.OnCall(b.Code(uint32(rv)), func(o *oracle.Oracle) {
				if h.off || h.cur == nil || h.written >= h.max {
					return
				}
				if rd.again(o) {
					return
				}
				h.ret++
				p := h.cur
				h.cur = nil
				w, hh, after := screenCopy(o)
				x0, y0, x1, y1 := w, hh, -1, -1
				for y := 0; y < hh; y++ {
					row := y * w
					if string(p.screen[row:row+w]) == string(after[row:row+w]) {
						continue
					}
					for x := 0; x < w; x++ {
						if p.screen[row+x] != after[row+x] {
							if x < x0 {
								x0 = x
							}
							if x > x1 {
								x1 = x
							}
							if y < y0 {
								y0 = y
							}
							if y > y1 {
								y1 = y
							}
						}
					}
				}
				bw, bh := 0, 0
				if x1 >= 0 {
					bw, bh = x1-x0+1, y1-y0+1
				} else {
					x0, y0 = 0, 0
				}
				buf := make([]byte, 8, 8+2*bw*bh)
				binary.LittleEndian.PutUint16(buf[0:], uint16(x0))
				binary.LittleEndian.PutUint16(buf[2:], uint16(y0))
				binary.LittleEndian.PutUint16(buf[4:], uint16(bw))
				binary.LittleEndian.PutUint16(buf[6:], uint16(bh))
				for y := 0; y < bh; y++ {
					buf = append(buf, p.screen[(y0+y)*w+x0:(y0+y)*w+x0+bw]...)
				}
				for y := 0; y < bh; y++ {
					buf = append(buf, after[(y0+y)*w+x0:(y0+y)*w+x0+bw]...)
				}
				fn := fmt.Sprintf("%s.%03d.bin", h.name, h.written)
				if err := os.WriteFile(filepath.Join(e.cap.dir, fn), buf, 0o644); err != nil {
					fmt.Fprintln(os.Stderr, "capcall 寫檔:", err)
				}
				rg := p.regs
				e.log.Emit(Ev78{Kind: "cap", Label: h.name, IDA: fmt.Sprintf("%05X", ent), Step: p.step, Tick: o.Ticks(),
					Regs: map[string]uint16{"AX": rg.AX, "BX": rg.BX, "CX": rg.CX, "DX": rg.DX, "SI": rg.SI, "DI": rg.DI, "BP": rg.BP, "SP": rg.SP, "DS": rg.DS, "ES": rg.ES, "SS": rg.SS},
					Stack: p.stack,
					Extra: map[string]any{"n": h.written, "file": fn, "ret_step": o.Steps(), "bbox": []int{x0, y0, bw, bh}}})
				h.written++
			})
		}
		return nil
	}
	return e.act093(f)
}

// capmem:IDA:暫存器:長度hex:名稱:最多筆數
//
//	IDA 被執行時（去重），把「暫存器值當段、偏移 0」起的長度 bytes 寫到 <capdir>/<名稱>.<NNN>.mem，
//	並寫一筆 kind=capmem 事件（步數、暫存器、堆疊 12 字）。暫存器：ax bx cx dx si di es ds。
//	capmemds:IDA:DS偏移hex:長度hex:名稱:最多筆數  同上，但取 DS:偏移 起的 bytes。
func (e *env) capMem(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	e.capInit()
	ent, err := strconv.ParseUint(f[1], 16, 64)
	if err != nil {
		return err
	}
	var regName string
	var dsOff uint64
	isDS := f[0] == "capmemds"
	if isDS {
		dsOff, err = strconv.ParseUint(f[2], 16, 64)
		if err != nil {
			return err
		}
	} else {
		regName = f[2]
	}
	n, err := strconv.ParseUint(f[3], 16, 64)
	if err != nil {
		return err
	}
	name := f[4]
	mx, err := strconv.Atoi(f[5])
	if err != nil {
		return err
	}
	if err := os.MkdirAll(e.cap.dir, 0o755); err != nil {
		return err
	}
	dir := e.cap.dir
	cnt := 0
	d := &dedup{}
	o.OnCall(b.Code(uint32(ent)), func(o *oracle.Oracle) {
		if cnt >= mx || d.again(o) {
			return
		}
		r := o.Regs()
		var a oracle.Addr
		if isDS {
			a = b.DS(uint16(dsOff))
		} else {
			var seg uint16
			switch regName {
			case "ax":
				seg = r.AX
			case "bx":
				seg = r.BX
			case "cx":
				seg = r.CX
			case "dx":
				seg = r.DX
			case "si":
				seg = r.SI
			case "di":
				seg = r.DI
			case "es":
				seg = r.ES
			case "ds":
				seg = r.DS
			}
			a = oracle.Addr{Seg: seg, Off: 0}
		}
		fn := fmt.Sprintf("%s.%03d.mem", name, cnt)
		if err := os.WriteFile(filepath.Join(dir, fn), o.Bytes(a, int(n)), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "capmem 寫檔:", err)
		}
		var stk []uint16
		for i := 0; i < 12; i++ {
			stk = append(stk, o.StackWord(i))
		}
		e.log.Emit(Ev78{Kind: "capmem", Label: name, IDA: fmt.Sprintf("%05X", ent), Step: o.Steps(), Tick: o.Ticks(),
			Regs: map[string]uint16{"AX": r.AX, "BX": r.BX, "CX": r.CX, "DX": r.DX, "SI": r.SI, "DI": r.DI, "BP": r.BP, "SP": r.SP, "DS": r.DS, "ES": r.ES, "SS": r.SS},
			Stack: stk, Extra: map[string]any{"n": cnt, "file": fn, "seg": int(a.Seg), "off": int(a.Off), "len": int(n)}})
		cnt++
	})
	return nil
}

// capshotat:IDA:名稱:最多筆數
//
//	IDA 被執行時（去重）存整個畫面（640x480 色號）到 <capdir>/<名稱>.<NNN>.idx，並寫一筆 kind=capshot 事件（步數）。
func (e *env) capShotAt(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	e.capInit()
	ent, err := strconv.ParseUint(f[1], 16, 64)
	if err != nil {
		return err
	}
	name := f[2]
	mx, err := strconv.Atoi(f[3])
	if err != nil {
		return err
	}
	if err := os.MkdirAll(e.cap.dir, 0o755); err != nil {
		return err
	}
	dir := e.cap.dir
	cnt := 0
	d := &dedup{}
	o.OnCall(b.Code(uint32(ent)), func(o *oracle.Oracle) {
		if cnt >= mx || d.again(o) {
			return
		}
		_, _, px := o.Screen()
		fn := fmt.Sprintf("%s.%03d.idx", name, cnt)
		if err := os.WriteFile(filepath.Join(dir, fn), px, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "capshotat 寫檔:", err)
		}
		e.log.Emit(Ev78{Kind: "capshot", Label: name, IDA: fmt.Sprintf("%05X", ent), Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"n": cnt, "file": fn}})
		cnt++
	})
	return nil
}

// capregion:IDA:x:y:w:h:名稱:最多筆數
//
//	IDA 被執行時（去重）存畫面矩形（畫面座標，列 0..479，十進位）的色號到 <capdir>/<名稱>.<NNN>.reg（w*h bytes），並寫 kind=capreg 事件。
func (e *env) capRegion(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	e.capInit()
	ent, err := strconv.ParseUint(f[1], 16, 64)
	if err != nil {
		return err
	}
	var v [4]int
	for i := 0; i < 4; i++ {
		v[i], err = strconv.Atoi(f[2+i])
		if err != nil {
			return err
		}
	}
	name := f[6]
	mx, err := strconv.Atoi(f[7])
	if err != nil {
		return err
	}
	if err := os.MkdirAll(e.cap.dir, 0o755); err != nil {
		return err
	}
	dir := e.cap.dir
	cnt := 0
	d := &dedup{}
	o.OnCall(b.Code(uint32(ent)), func(o *oracle.Oracle) {
		if cnt >= mx || d.again(o) {
			return
		}
		w, _, px := o.Screen()
		buf := make([]byte, 0, v[2]*v[3])
		for y := 0; y < v[3]; y++ {
			buf = append(buf, px[(v[1]+y)*w+v[0]:(v[1]+y)*w+v[0]+v[2]]...)
		}
		fn := fmt.Sprintf("%s.%03d.reg", name, cnt)
		if err := os.WriteFile(filepath.Join(dir, fn), buf, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "capregion 寫檔:", err)
		}
		r := o.Regs()
		e.log.Emit(Ev78{Kind: "capreg", Label: name, IDA: fmt.Sprintf("%05X", ent), Step: o.Steps(), Tick: o.Ticks(),
			Regs: map[string]uint16{"AX": r.AX, "DX": r.DX}, Extra: map[string]any{"n": cnt, "file": fn, "rect": v[:]}})
		cnt++
	})
	return nil
}

// worldwin:輸出路徑[:known]
//
//	照 k2world：對世界圖的 3 項 x 43 列 x 28 欄視窗各呼叫原版 sub_32D62，把 72x72 緩衝依序寫到輸出檔。
//	known 給了就先把全部港口記錄（DS:5F54 起，每筆 22 bytes，130 筆）的 +0x15 bit4（known）與地點記錄（DS:7F8C 起，每筆 7 bytes，98 筆）的 +6 bit6（found）設起
//	（測試條件：只改輸入旗標，不跳過 sub_32D62 的任何程式）。要先 at:310F1，避免在 FMDRV 的 INT 處理器內強制呼叫。
func (e *env) worldWin(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	const (
		offLoadItem uint16 = 0x849A
		offWindow   uint16 = 0x2FD2
		dsItem      uint16 = 0x06CA
		dsBuf       uint16 = 0x03F4
		stride             = 0x60
		size               = 72
	)
	if len(f) > 2 {
		for i := 0; i < 130; i++ {
			a := b.DS(uint16(0x5F54 + 22*i + 0x15))
			o.SetByte(a, o.Byte(a)|0x10)
		}
		for j := 0; j < 98; j++ {
			a := b.DS(uint16(0x7F8C + 7*j + 6))
			o.SetByte(a, o.Byte(a)|0x40)
		}
	}
	off, seg := o.Word(b.DS(dsBuf)), o.Word(b.DS(dsBuf+2))
	buf := oracle.Addr{Seg: seg, Off: off}
	fh, err := os.Create(f[1])
	if err != nil {
		return err
	}
	defer fh.Close()
	dgroup := b.DS(0).Seg
	regs := func(ax uint16) oracle.CallRegs {
		return oracle.CallRegs{AX: ax, SetAX: true, DS: dgroup, SetDS: true}
	}
	for item := 0; item < 3; item++ {
		o.SetByte(b.DS(dsItem), uint8(item))
		o.SetRegs(regs(0))
		if _, err := o.CallBudget(200_000_000, oracle.Addr{Seg: b.Seg(0x1000), Off: offLoadItem}); err != nil {
			return fmt.Errorf("載入項 %d：%w", item, err)
		}
		for row := 0; row <= 42; row++ {
			for col := 0; col <= 27; col++ {
				o.SetRegs(regs(uint16(30*row + col)))
				if _, err := o.CallBudget(200_000_000, oracle.Addr{Seg: b.Seg(0x2FD9), Off: offWindow}); err != nil {
					return fmt.Errorf("項 %d 列 %d 欄 %d：%w", item, row, col, err)
				}
				raw := o.Bytes(buf, stride*size)
				win := make([]byte, 0, size*size)
				for r := 0; r < size; r++ {
					win = append(win, raw[r*stride:r*stride+size]...)
				}
				if _, err := fh.Write(win); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
