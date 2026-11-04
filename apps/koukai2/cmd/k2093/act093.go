package main

// 批次 093 新增的動作（複製自 k2084 之後只增不改別的目錄）。
//
//	dac:名稱                    把目前 DAC（256 色，每色 R G B 各 8 位元，dosgolem Palette() 的輸出）寫到 <capdir>/<名稱>.dac。
//	dacat:IDA:名稱:最多筆數      IDA 被執行時（去重）同上，檔名 <名稱>.<NNN>.dac，同時寫一筆 kind=dac 事件（含步數）。
//	pokeat:IDA:DS偏移:HEX:標籤[:最多次數]
//	                            IDA 被執行時（去重）把 DS:偏移 改成 HEX（測試條件，每次寫一筆 poke 事件，含舊值）。
//	sail:天數[:補給]             以 2M 指令為一段讓遊戲自己跑，日期（DS:0532..0534）每變一天補一次水糧（同 k2082 的 sail），
//	                            每段結束若 DS:0CB3 有變記一筆 note（weather）。
//	oplat:IDA:標籤:最多筆數      IDA 被執行時記 OPL 寫入計數（步數、埠 0x388/0x389 的累計筆數），用來切分音樂的起點。
//
// 事件項目另有：l<線性hex>.<長度hex>（實體線性位址），i<IDA hex>.<長度hex>（目前程式的 IDA 線性位址，END.EXE 載入後換算）。

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/oracle"
)

func (e *env) act093(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	hexn := func(i int) (uint64, error) { return strconv.ParseUint(f[i], 16, 64) }
	switch f[0] {
	case "dac":
		return e.writeDAC(filepath.Join(e.cap.dir, f[1]+".dac"))
	case "dacat":
		if len(f) < 4 {
			return fmt.Errorf("dacat:IDA:名稱:最多筆數")
		}
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		mx, _ := strconv.Atoi(f[3])
		name := f[2]
		n := 0
		d := &dedup{}
		if err := os.MkdirAll(e.cap.dir, 0o755); err != nil {
			return err
		}
		o.OnCall(b.Code(uint32(lin)), func(o *oracle.Oracle) {
			if n >= mx || d.again(o) {
				return
			}
			n++
			p := filepath.Join(e.cap.dir, fmt.Sprintf("%s.%03d.dac", name, n))
			_ = e.writeDAC(p)
			e.log.Emit(Ev78{Kind: "dac", Label: name, IDA: f[1], Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o), Extra: map[string]any{"file": filepath.Base(p)}})
		})
		return nil
	case "pokeat":
		if len(f) < 5 {
			return fmt.Errorf("pokeat:IDA:DS偏移:HEX:標籤[:最多次數]")
		}
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
		label := f[4]
		mx := 1 << 30
		if len(f) > 5 {
			mx, _ = strconv.Atoi(f[5])
		}
		n := 0
		d := &dedup{}
		o.OnCall(b.Code(uint32(lin)), func(o *oracle.Oracle) {
			if n >= mx || d.again(o) {
				return
			}
			n++
			old := o.Bytes(b.DS(uint16(off)), len(data))
			o.SetBytes(b.DS(uint16(off)), data)
			e.log.Emit(Ev78{Kind: "poke", Label: label, IDA: f[1], Step: o.Steps(), Tick: o.Ticks(),
				Extra: map[string]any{"off": int(off), "old": hex.EncodeToString(old), "new": f[3], "n": n}})
		})
		return nil
	case "oplat":
		if len(f) < 4 {
			return fmt.Errorf("oplat:IDA:標籤:最多筆數")
		}
		lin, err := hexn(1)
		if err != nil {
			return err
		}
		mx, _ := strconv.Atoi(f[3])
		n := 0
		d := &dedup{}
		o.OnCall(b.Code(uint32(lin)), func(o *oracle.Oracle) {
			if n >= mx || d.again(o) {
				return
			}
			n++
			e.log.Emit(Ev78{Kind: "oplat", Label: f[2], IDA: f[1], Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o),
				Extra: map[string]any{"opl_writes": len(o.PortWritesIn(0x388, 0x389))}})
		})
		return nil
	case "sail":
		return e.sail093(f)
	case "capcell":
		return e.capCell(f)
	case "callcell":
		return e.callCell(f)
	case "bind":
		return e.bindAction(f)
	}
	return fmt.Errorf("不認得的動作")
}

func (e *env) writeDAC(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	pal := e.o.Palette()
	buf := make([]byte, 0, 768)
	for i := 0; i < 256; i++ {
		buf = append(buf, pal[i][0], pal[i][1], pal[i][2])
	}
	return os.WriteFile(path, buf, 0o644)
}

// sail093：sail:天數[:補給]。補給格式 HEX（寫到 DS:52D4）或 偏移=HEX/偏移=HEX。
func (e *env) sail093(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	days, err := strconv.Atoi(f[1])
	if err != nil {
		return err
	}
	type refillItem struct {
		off  uint16
		data []byte
	}
	var refills []refillItem
	if len(f) > 2 && f[2] != "" && f[2] != "-" {
		for _, part := range strings.Split(f[2], "/") {
			off := uint64(0x52D4)
			hx := part
			if o_, h_, ok := strings.Cut(part, "="); ok {
				off, err = strconv.ParseUint(o_, 16, 16)
				if err != nil {
					return err
				}
				hx = h_
			}
			d, err := hex.DecodeString(hx)
			if err != nil {
				return err
			}
			refills = append(refills, refillItem{uint16(off), d})
		}
	}
	start := o.Steps()
	prev := o.Bytes(b.DS(0x0532), 4)
	prevW := o.Byte(b.DS(0x0CB3))
	count, stuck := 0, 0
	for count < days {
		if o.Steps()-start > e.max {
			return fmt.Errorf("sail：預算用完，已過 %d 天", count)
		}
		if err := o.Run(2_000_000); err != nil {
			return err
		}
		if w := o.Byte(b.DS(0x0CB3)); w != prevW {
			e.log.Emit(Ev78{Kind: "note", Label: "weather", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"old": int(prevW), "new": int(w)}})
			prevW = w
		}
		cur := o.Bytes(b.DS(0x0532), 4)
		if string(cur) == string(prev) {
			stuck++
			if stuck >= 10 {
				stuck = 0
				if err := o.SendKeys("Return"); err != nil {
					return err
				}
				e.log.Emit(Ev78{Kind: "note", Label: "unstick", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"day": hex.EncodeToString(cur), "cb3": int(o.Byte(b.DS(0x0CB3)))}})
			}
			continue
		}
		stuck = 0
		newDay := string(cur[:3]) != string(prev[:3])
		prev = cur
		if newDay {
			count++
			for _, rf := range refills {
				o.SetBytes(b.DS(rf.off), rf.data)
				e.log.Emit(Ev78{Kind: "poke", Label: "refill", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"off": int(rf.off), "new": hex.EncodeToString(rf.data)}})
			}
			e.log.Emit(Ev78{Kind: "note", Label: "day", Step: o.Steps(), Tick: o.Ticks(), Extra: map[string]any{"n": count, "date": hex.EncodeToString(cur), "cb3": int(o.Byte(b.DS(0x0CB3)))}})
		}
	}
	return nil
}

// forcedBind 讓 hook 在程式載入之前就以該程式的位址換算安裝（END.EXE 載入前裝 END.EXE 的 hook）。
// 動作 bind:程式名:PSP（十六進位）設定，bind:off 清除。只影響之後安裝 hook 時的位址換算，不改任何狀態。
var forcedBind *Bind

func (e *env) bindAction(f []string) error {
	if f[1] == "off" {
		forcedBind = nil
		return nil
	}
	if len(f) < 3 {
		return fmt.Errorf("bind:程式名:PSP | bind:off")
	}
	psp, err := strconv.ParseUint(f[2], 16, 16)
	if err != nil {
		return err
	}
	forcedBind = &Bind{Name: strings.ToUpper(f[1]), PSP: uint16(psp), Running: true}
	return nil
}

// capcell:ENTRY:RET:名稱:最多筆數[:w:h]
//
//	ENTRY 被執行時（去重）記畫面與暫存器；RET 被執行時存「AX、DX 指定的圖塊格」整塊 w x h（預設 32x32）的呼叫前與呼叫後色號。
//	格的畫面座標 = (96 + 16 * AX, 8 + 16 * DX + 40)（港口地圖視窗，AX = 視窗欄、DX = 視窗列）。
//	檔案格式同 capcall：x0 y0 w h（u16 小端），接 w*h bytes 呼叫前、w*h bytes 呼叫後（外框固定是整塊，不是變動外框）。
//	事件 kind=cap，Regs 為進入時暫存器，Extra 有 file、ret_step。只觀測。
func (e *env) capCell(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	e.capInit()
	ent, err := strconv.ParseUint(f[1], 16, 64)
	if err != nil {
		return err
	}
	ret, err := strconv.ParseUint(f[2], 16, 64)
	if err != nil {
		return err
	}
	name := f[3]
	mx, _ := strconv.Atoi(f[4])
	cw, ch := 32, 32
	if len(f) > 6 {
		cw, _ = strconv.Atoi(f[5])
		ch, _ = strconv.Atoi(f[6])
	}
	if err := os.MkdirAll(e.cap.dir, 0o755); err != nil {
		return err
	}
	dir := e.cap.dir
	var pend *capPend
	written := 0
	d := &dedup{}
	rd := &dedup{}
	crop := func(px []byte, w, x0, y0 int) []byte {
		buf := make([]byte, 0, cw*ch)
		for y := 0; y < ch; y++ {
			buf = append(buf, px[(y0+y)*w+x0:(y0+y)*w+x0+cw]...)
		}
		return buf
	}
	o.OnCall(b.Code(uint32(ent)), func(o *oracle.Oracle) {
		if written >= mx || d.again(o) {
			return
		}
		w, _, scr := screenCopy(o)
		r := o.Regs()
		x0, y0 := 96+16*int(r.AX), 8+16*int(r.DX)+40
		if x0 < 0 || y0 < 0 || x0+cw > w || y0+ch > 480 {
			pend = nil
			return
		}
		p := &capPend{regs: r, step: o.Steps()}
		p.screen = crop(scr, w, x0, y0)
		for i := 0; i < 4; i++ {
			p.stack = append(p.stack, o.StackWord(i))
		}
		pend = p
	})
	o.OnCall(b.Code(uint32(ret)), func(o *oracle.Oracle) {
		if pend == nil || written >= mx || rd.again(o) {
			return
		}
		p := pend
		pend = nil
		w, _, scr := screenCopy(o)
		x0, y0 := 96+16*int(p.regs.AX), 8+16*int(p.regs.DX)+40
		after := crop(scr, w, x0, y0)
		buf := make([]byte, 8, 8+2*cw*ch)
		binary.LittleEndian.PutUint16(buf[0:], uint16(x0))
		binary.LittleEndian.PutUint16(buf[2:], uint16(y0))
		binary.LittleEndian.PutUint16(buf[4:], uint16(cw))
		binary.LittleEndian.PutUint16(buf[6:], uint16(ch))
		buf = append(buf, p.screen...)
		buf = append(buf, after...)
		fn := fmt.Sprintf("%s.%03d.bin", name, written)
		if err := os.WriteFile(filepath.Join(dir, fn), buf, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "capcell 寫檔:", err)
		}
		rg := p.regs
		e.log.Emit(Ev78{Kind: "cap", Label: name, IDA: fmt.Sprintf("%05X", ent), Step: p.step, Tick: o.Ticks(),
			Regs:  map[string]uint16{"AX": rg.AX, "BX": rg.BX, "CX": rg.CX, "DX": rg.DX, "SI": rg.SI, "DI": rg.DI, "BP": rg.BP, "SP": rg.SP, "DS": rg.DS, "ES": rg.ES, "SS": rg.SS},
			Stack: p.stack,
			Extra: map[string]any{"n": written, "file": fn, "ret_step": o.Steps(), "bbox": []int{x0, y0, cw, ch}}})
		written++
	})
	return nil
}

// callcell:AX:DX:BX:名稱
//
//	測試條件：直接呼叫原版 sub_1BD3F（海戰棋盤格圖樣繪製，1BD3F..1BE1F）。AX = 地圖欄（十進位）、DX = 地圖列（十進位）、BX = 格碼（十六進位）。
//	呼叫前以 OnCall 一次性把 AX、DX、BX 設好（常式讀到的就是呼叫端會給的暫存器），其餘全是原版程式。
//	格的畫面矩形（讀碼，1BD53..1BE11）：x = 32 * (欄 - DS:9BEF)，y = 32 * (列 - DS:9BF0) + (欄為奇數 ? 16 : 0) + 40，32x32。
//	存呼叫前後該矩形的色號（格式同 capcall：x0 y0 w h u16 小端，接 w*h 前、w*h 後）到 <capdir>/<名稱>.bin，並寫 kind=cell 事件。
func (e *env) callCell(f []string) error {
	o := e.o
	b, _ := CurrentOrMain(o)
	e.capInit()
	if len(f) < 5 {
		return fmt.Errorf("callcell:AX:DX:BX(hex):名稱")
	}
	col, err := strconv.Atoi(f[1])
	if err != nil {
		return err
	}
	row, err := strconv.Atoi(f[2])
	if err != nil {
		return err
	}
	code, err := strconv.ParseUint(f[3], 16, 16)
	if err != nil {
		return err
	}
	name := f[4]
	ox, oy := int(o.Byte(b.DS(0x9BEF))), int(o.Byte(b.DS(0x9BF0)))
	col, row = col+ox, row+oy // 參數是相對視窗原點的欄、列偏移（視窗原點 DS:9BEF、9BF0 在執行時才知道）
	X := 32 * (col - ox)
	Y := 32*(row-oy) + 40
	if col%2 == 1 {
		Y += 16
	}
	if X < 0 || Y < 40 || X+32 > 416 || Y+32 > 480 {
		return fmt.Errorf("callcell：格 (%d,%d) 在視窗外（原點 %d,%d）", col, row, ox, oy)
	}
	crop := func() []byte {
		w, _, px := o.Screen()
		buf := make([]byte, 0, 1024)
		for y := 0; y < 32; y++ {
			buf = append(buf, px[(Y+y)*w+X:(Y+y)*w+X+32]...)
		}
		return append([]byte(nil), buf...)
	}
	before := crop()
	// listing 的段:位移（1000:BD3F）：near call 以 CS:IP 相對，段值不能用 IDA 線性位址拆出來的別名，否則 call near 會跳到錯的位置。
	target := oracle.Addr{Seg: b.Seg(0x1000), Off: 0xBD3F}
	// Call 保留目前暫存器（只換 SP、CS、IP），所以呼叫前直接設 AX、DX、BX；呼叫後把原值寫回，不改被中斷程式的狀態。
	r0 := o.Regs()
	s0 := o.Steps()
	o.SetRegs(oracle.CallRegs{AX: uint16(col), SetAX: true, DX: uint16(row), SetDX: true, BX: uint16(code), SetBX: true, DS: b.DS(0).Seg, SetDS: true})
	_, cerr := o.CallBudget(5_000_000, target)
	o.SetRegs(oracle.CallRegs{AX: r0.AX, SetAX: true, BX: r0.BX, SetBX: true, DX: r0.DX, SetDX: true, DS: r0.DS, SetDS: true})
	if cerr != nil {
		return cerr
	}
	after := crop()
	buf := make([]byte, 8, 8+2048)
	binary.LittleEndian.PutUint16(buf[0:], uint16(X))
	binary.LittleEndian.PutUint16(buf[2:], uint16(Y))
	binary.LittleEndian.PutUint16(buf[4:], 32)
	binary.LittleEndian.PutUint16(buf[6:], 32)
	buf = append(buf, before...)
	buf = append(buf, after...)
	fn := name + ".bin"
	if err := os.MkdirAll(e.cap.dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(e.cap.dir, fn), buf, 0o644); err != nil {
		return err
	}
	e.log.Emit(Ev78{Kind: "cell", Label: name, IDA: "1BD3F", Step: o.Steps(), Tick: o.Ticks(),
		Extra: map[string]any{"steps": o.Steps() - s0, "ds_before": int(r0.DS), "ds_main": int(b.DS(0).Seg), "cs_before": int(r0.CS), "col": col, "row": row, "code": int(code), "origin": []int{ox, oy}, "rect": []int{X, Y, 32, 32}, "file": fn}})
	return nil
}
