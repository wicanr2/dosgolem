package main

// 批次 082 的觀測輔助：事件紀錄（hook 觸發時抓暫存器、堆疊與記憶體並去重）。
//
// 用 `oracle.OnCall` 掛 hook，輸出 JSON lines 給 koukai2 專案的比對腳本。
// 這一層只觀測，不改被觀測程式的記憶體。

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/wicanr2/dosgolem/oracle"
)

// Ev82 是一筆事件紀錄。
type Ev82 struct {
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

// EvLog82 把事件寫成 JSON lines。
type EvLog82 struct {
	W   io.Writer
	Seq int
	Cnt map[string]int
}

// NewEvLog82 建立紀錄器。
func NewEvLog82(w io.Writer) *EvLog82 { return &EvLog82{W: w, Cnt: map[string]int{}} }

// Emit 寫一筆事件。
func (l *EvLog82) Emit(e Ev82) {
	l.Seq++
	e.Seq = l.Seq
	l.Cnt[e.Kind+":"+e.Label]++
	b, _ := json.Marshal(e)
	b = append(b, '\n')
	_, _ = l.W.Write(b)
}

// Item82 描述 hook 要抓的一段記憶體。'f' DS:(BX+偏移)、'g' DS:(SI+偏移)、'h' DS:(DI+偏移)。
type Item82 struct {
	Kind byte // 'd' DS 偏移；'e' ES 偏移；'b' ES:(BX+偏移)；'p' SS:(BP+偏移)；'s' 堆疊 N 個字
	Off  uint16
	N    int
}

// ParseItems82 解析 `d0535.1+d0CB1.3+e0.10+b90.4+s6`。偏移與長度都是十六進位。
func ParseItems82(spec string) ([]Item82, error) {
	var out []Item82
	for _, tok := range splitPlus82(spec) {
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
			out = append(out, Item82{Kind: 's', N: n})
		case 'd', 'e', 'b', 'p', 'f', 'g', 'h':
			var off, n int
			if _, err := fmt.Sscanf(tok[1:], "%x.%x", &off, &n); err != nil {
				return nil, fmt.Errorf("項目 %q：%w", tok, err)
			}
			out = append(out, Item82{Kind: k, Off: uint16(off), N: n})
		default:
			return nil, fmt.Errorf("項目 %q：不認得的種類", tok)
		}
	}
	return out, nil
}

func splitPlus82(s string) []string {
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

// Capture82 依項目抓值，填進事件。
func Capture82(o *oracle.Oracle, b Bind, items []Item82, e *Ev82) {
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
		case 'f', 'g', 'h':
			if e.ES == nil {
				e.ES = map[string]string{}
			}
			base := map[byte]uint16{'f': r.BX, 'g': r.SI, 'h': r.DI}[it.Kind]
			off := base + it.Off
			e.ES[fmt.Sprintf("%c%04X@%04X.%X", it.Kind, it.Off, off, it.N)] = hex.EncodeToString(o.Bytes(b.DS(off), it.N))
		case 's':
			for i := 0; i < it.N; i++ {
				e.Stack = append(e.Stack, o.StackWord(i))
			}
		}
	}
}

func regMap82(o *oracle.Oracle) map[string]uint16 {
	r := o.Regs()
	return map[string]uint16{
		"AX": r.AX, "BX": r.BX, "CX": r.CX, "DX": r.DX, "SI": r.SI, "DI": r.DI,
		"BP": r.BP, "SP": r.SP, "DS": r.DS, "ES": r.ES, "SS": r.SS, "CS": r.CS, "IP": r.IP,
	}
}

type hookKey82 struct {
	step   uint64
	r      oracle.Regs
	ticks  uint64
	irq1   uint64
	hasPrv bool
}

// HookEv82 在 MAIN.EXE 的 IDA 位址掛事件 hook。
//
// `OnCall` 在中斷返回時會重複觸發（dosgolem 規格 443 第 4 節）。去重條件：
// 全部暫存器與 SP 都與上一次觸發相同，而且兩次之間有 IRQ0 或 IRQ1 被送達。
// 去重不影響迴圈：迴圈內的暫存器通常會變，而且沒有中斷穿插時不會被判成重複。
// limit 為 0 表示不限筆數；達到上限後不再記錄，但仍計入 Dup82 與 total。
func HookEv82(o *oracle.Oracle, b Bind, log *EvLog82, ida uint32, label string, items []Item82, limit int) *HookStat82 {
	st := &HookStat82{}
	var prev hookKey82
	o.OnCall(b.Code(ida), func(o *oracle.Oracle) {
		cur := hookKey82{step: o.Steps(), r: o.Regs(), ticks: o.Ticks(), irq1: o.IRQ1Delivered(), hasPrv: true}
		if prev.hasPrv && prev.r == cur.r && cur.step-prev.step < Dup82Window && (prev.ticks != cur.ticks || prev.irq1 != cur.irq1) {
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
		e := Ev82{Kind: "hook", Label: label, IDA: fmt.Sprintf("%05X", ida), Step: o.Steps(), Tick: o.Ticks(), Regs: regMap82(o)}
		Capture82(o, b, items, &e)
		log.Emit(e)
	})
	return st
}

// HookStat82 是 hook 的計數。
type HookStat82 struct {
	Total  int // 去重後的觸發次數
	Logged int
	Dup    int // 中斷返回造成的重複觸發
}

// Snap82 在現在這一刻記一筆記憶體快照。
func Snap82(o *oracle.Oracle, b Bind, log *EvLog82, label string, items []Item82) {
	e := Ev82{Kind: "snap", Label: label, Step: o.Steps(), Tick: o.Ticks()}
	Capture82(o, b, items, &e)
	log.Emit(e)
}


// Dup82Window 是判定「中斷返回造成的重複觸發」的最大指令間距。
// 兩次觸發之間只跑了中斷處理常式，間距是 ISR 的長度（實測在百道以內）；
// 一般迴圈或逐時段的再次經過，間距至少是一圈的長度。
const Dup82Window = 3000
