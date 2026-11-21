package main

// 批次 078 的觀測輔助：事件紀錄（hook 觸發時抓暫存器、堆疊與記憶體並去重）。
//
// 用 `oracle.OnCall` 掛 hook，輸出 JSON lines 給 koukai2 專案的比對腳本。
// 這一層只觀測，不改被觀測程式的記憶體。

import (
	"unicode"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/wicanr2/dosgolem/oracle"
)

// Ev78 是一筆事件紀錄。
type Ev78 struct {
	Seq   int               `json:"i"`
	Kind  string            `json:"k"` // hook、snap、write、call
	Label string            `json:"lab"`
	IDA   string            `json:"ida,omitempty"`
	Step  uint64            `json:"step"`
	Tick  uint64            `json:"tick"`
	Regs  map[string]uint16 `json:"r,omitempty"`
	Stack []uint16          `json:"stk,omitempty"`
	DS    map[string]string `json:"ds,omitempty"` // 鍵 "偏移.長度"，值十六進位
	ES    map[string]string `json:"es,omitempty"` // 鍵 "偏移.長度"（相對 ES），值十六進位
	Extra map[string]any    `json:"x,omitempty"`
}

// EvLog78 把事件寫成 JSON lines。
type EvLog78 struct {
	W   io.Writer
	Seq int
	Cnt map[string]int
}

// NewEvLog78 建立紀錄器。
func NewEvLog78(w io.Writer) *EvLog78 { return &EvLog78{W: w, Cnt: map[string]int{}} }

// Emit 寫一筆事件。
func (l *EvLog78) Emit(e Ev78) {
	l.Seq++
	e.Seq = l.Seq
	l.Cnt[e.Kind+":"+e.Label]++
	b, _ := json.Marshal(e)
	b = append(b, '\n')
	_, _ = l.W.Write(b)
}

// Item78 描述 hook 要抓的一段記憶體。
type Item78 struct {
	Kind byte // 'd' DS 偏移；'e' ES 偏移；'b' ES:(BX+偏移)；'p' SS:(BP+偏移)；'s' 堆疊 N 個字
	Off  uint16
	Off2 uint16 // 項目 u 的第二個偏移
	N    int
}

// ParseItems78 解析 `d0535.1+d0CB1.3+e0.10+b90.4+s6`。偏移與長度都是十六進位。
func ParseItems78(spec string) ([]Item78, error) {
	var out []Item78
	for _, tok := range splitPlus78(spec) {
		if tok == "" {
			continue
		}
		k := tok[0]
		switch k {
		case 's':
			var n int
			if _, err := fmt.Sscanf(tok[1:], "%d", &n); err != nil {
				return nil, fmt.Errorf("項目 %q：%w", tok, err)
			}
			out = append(out, Item78{Kind: 's', N: n})
		case 'q', 'x', 'y':
			var off, n int
			if _, err := fmt.Sscanf(tok[1:], "%x.%x", &off, &n); err != nil {
				return nil, fmt.Errorf("項目 %q：%w", tok, err)
			}
			out = append(out, Item78{Kind: k, Off: uint16(off), N: n})
		case 'u':
			var o1, o2, n int
			if _, err := fmt.Sscanf(tok[1:], "%x.%x.%x", &o1, &o2, &n); err != nil {
				return nil, fmt.Errorf("項目 %q：%w", tok, err)
			}
			out = append(out, Item78{Kind: 'u', Off: uint16(o1), Off2: uint16(o2), N: n})
		case 'l', 'i':
			var off, n int
			if _, err := fmt.Sscanf(tok[1:], "%x.%x", &off, &n); err != nil {
				return nil, fmt.Errorf("項目 %q：%w", tok, err)
			}
			out = append(out, Item78{Kind: k, Off: uint16(off & 0xFFFF), Off2: uint16(off >> 16), N: n})
		case 'd', 'e', 'b', 'p':
			var off, n int
			if _, err := fmt.Sscanf(tok[1:], "%x.%x", &off, &n); err != nil {
				return nil, fmt.Errorf("項目 %q：%w", tok, err)
			}
			out = append(out, Item78{Kind: k, Off: uint16(off), N: n})
		default:
			return nil, fmt.Errorf("項目 %q：不認得的種類", tok)
		}
	}
	return out, nil
}

func splitPlus78(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == '+' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	return append(out, cur)
}

// Capture78 依項目抓值，填進事件。
func Capture78(o *oracle.Oracle, b Bind, items []Item78, e *Ev78) {
	r := o.Regs()
	for _, it := range items {
		switch it.Kind {
		case 'd':
			if e.DS == nil {
				e.DS = map[string]string{}
			}
			e.DS[fmt.Sprintf("%04X.%X", it.Off, it.N)] = hex.EncodeToString(o.Bytes(b.DS(it.Off), it.N))
		case 'e':
			if e.ES == nil {
				e.ES = map[string]string{}
			}
			e.ES[fmt.Sprintf("%04X.%X", it.Off, it.N)] = hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.ES, Off: it.Off}, it.N))
		case 'b':
			if e.ES == nil {
				e.ES = map[string]string{}
			}
			off := r.BX + it.Off
			e.ES[fmt.Sprintf("B%04X@%04X.%X", it.Off, off, it.N)] = hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.ES, Off: off}, it.N))
		case 'p':
			if e.ES == nil {
				e.ES = map[string]string{}
			}
			off := r.BP + it.Off
			e.ES[fmt.Sprintf("P%04X@%04X.%X", it.Off, off, it.N)] = hex.EncodeToString(o.Bytes(oracle.Addr{Seg: r.SS, Off: off}, it.N))
		case 'l', 'i':
			// 線性位址（l：實體線性；i：目前程式的 IDA 線性位址，以 Bind.Code 換算），鍵 L/I<hex>.<長度>
			if e.ES == nil {
				e.ES = map[string]string{}
			}
			idaOrLin := uint32(it.Off2)<<16 | uint32(it.Off)
			lin := idaOrLin
			if it.Kind == 'i' {
				lin = b.Code(idaOrLin).Linear()
			}
			e.ES[fmt.Sprintf("%c%X.%X", unicode.ToUpper(rune(it.Kind)), idaOrLin, it.N)] = hex.EncodeToString(o.Bytes(oracle.Addr{Seg: uint16(lin >> 4), Off: uint16(lin & 0xF)}, it.N))
		case 'x', 'y':
			// DS:(AX+偏移) 或 DS:(DX+偏移)，鍵 X/Y<偏移>@<實際偏移>.<長度>
			if e.DS == nil {
				e.DS = map[string]string{}
			}
			base := r.AX
			if it.Kind == 'y' {
				base = r.DX
			}
			off := base + it.Off
			e.DS[fmt.Sprintf("%c%04X@%04X.%X", unicode.ToUpper(rune(it.Kind)), it.Off, off, it.N)] = hex.EncodeToString(o.Bytes(b.DS(off), it.N))
		case 'q':
			// DS:(BX+偏移)，鍵 Q<偏移>@<實際偏移>.<長度>
			if e.DS == nil {
				e.DS = map[string]string{}
			}
			off := r.BX + it.Off
			e.DS[fmt.Sprintf("Q%04X@%04X.%X", it.Off, off, it.N)] = hex.EncodeToString(o.Bytes(b.DS(off), it.N))
		case 'u':
			// DS:([DS:BX+Off] + Off2)，鍵 U<Off>.<Off2>@<實際偏移>.<長度>
			if e.DS == nil {
				e.DS = map[string]string{}
			}
			ptr := o.Word(b.DS(r.BX + it.Off))
			off := ptr + it.Off2
			e.DS[fmt.Sprintf("U%04X.%04X@%04X.%X", it.Off, it.Off2, off, it.N)] = hex.EncodeToString(o.Bytes(b.DS(off), it.N))
		case 's':
			for i := 0; i < it.N; i++ {
				e.Stack = append(e.Stack, o.StackWord(i))
			}
		}
	}
}

func regMap78(o *oracle.Oracle) map[string]uint16 {
	r := o.Regs()
	return map[string]uint16{
		"AX": r.AX, "BX": r.BX, "CX": r.CX, "DX": r.DX, "SI": r.SI, "DI": r.DI,
		"BP": r.BP, "SP": r.SP, "DS": r.DS, "ES": r.ES, "SS": r.SS, "CS": r.CS, "IP": r.IP,
	}
}

type hookKey78 struct {
	step   uint64
	r      oracle.Regs
	ticks  uint64
	irq1   uint64
	hasPrv bool
}

// HookEv78 在 MAIN.EXE 的 IDA 位址掛事件 hook。
//
// `OnCall` 在中斷返回時會重複觸發（dosgolem 規格 443 第 4 節）。去重條件：
// 全部暫存器與 SP 都與上一次觸發相同，而且兩次之間有 IRQ0 或 IRQ1 被送達。
// 去重不影響迴圈：迴圈內的暫存器通常會變，而且沒有中斷穿插時不會被判成重複。
// limit 為 0 表示不限筆數；達到上限後不再記錄，但仍計入 Dup78 與 total。
func HookEv78(o *oracle.Oracle, b Bind, log *EvLog78, ida uint32, label string, items []Item78, limit int) *HookStat78 {
	st := &HookStat78{}
	var prev hookKey78
	o.OnCall(b.Code(ida), func(o *oracle.Oracle) {
		cur := hookKey78{step: o.Steps(), r: o.Regs(), ticks: o.Ticks(), irq1: o.IRQ1Delivered(), hasPrv: true}
		if prev.hasPrv && prev.r == cur.r && cur.step-prev.step < Dup78Window && (prev.ticks != cur.ticks || prev.irq1 != cur.irq1) {
			st.Dup++
			prev = cur
			return
		}
		prev = cur
		st.Total++
		if limit > 0 && st.Logged >= limit {
			return
		}
		st.Logged++
		e := Ev78{Kind: "hook", Label: label, IDA: fmt.Sprintf("%05X", ida), Step: o.Steps(), Tick: o.Ticks(), Regs: regMap78(o)}
		Capture78(o, b, items, &e)
		log.Emit(e)
	})
	return st
}

// HookStat78 是 hook 的計數。
type HookStat78 struct {
	Total  int // 去重後的觸發次數
	Logged int
	Dup    int // 中斷返回造成的重複觸發
}

// Snap78 在現在這一刻記一筆記憶體快照。
func Snap78(o *oracle.Oracle, b Bind, log *EvLog78, label string, items []Item78) {
	e := Ev78{Kind: "snap", Label: label, Step: o.Steps(), Tick: o.Ticks()}
	Capture78(o, b, items, &e)
	log.Emit(e)
}


// Dup78Window 是判定「中斷返回造成的重複觸發」的最大指令間距。
// 兩次觸發之間只跑了中斷處理常式，間距是 ISR 的長度（實測在百道以內）；
// 一般迴圈或逐時段的再次經過，間距至少是一圈的長度。
const Dup78Window = 3000
