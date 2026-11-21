package dkk

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// Inventory 是 G3b 印字路徑盤點（issue #11）的觀測器：記錄每次路徑 A、B 呼叫的字串與查表結果，
// 以及不在 A、B 呼叫內的逐字呼叫（05F4:01F5），供判斷畫面是否落在既有家族、哪些字串還沒有譯文。
// 只觀測，不改畫面；輸出含原文，只放工作區。由 Live 在綁定後建立（Live.Inventory 設為 true）。
type Inventory struct {
	Events []InvEvent

	a, b, g uint32
	frames  []invFrame
	run     int // 進行中的逐字串索引（同路徑、同呼叫端段、同列、欄連續）；-1 表示沒有
	runCol  int
	seen    map[string]int // 段＋前 64 bytes → snaps 索引
	snaps   [][]byte
}

// InvEvent 是一次印字。Path：A、B 為字串呼叫；C 為選單逐字；G 為不在 A、B 呼叫內、也不是選單的逐字
// （路徑 D 05F4:0361 是 near 函式，不另攔，其逐字以呼叫端 05F4 出現在 G）。
type InvEvent struct {
	Step      uint64
	Path      string
	Caller    string // 返回位址 段:位移
	CallerSeg uint16
	Row, Col  int
	Text      string
	Key       string
	Rejected  string
	snap      int // 呼叫端段開頭的快照（overlay 會換載入同一段，事後讀會對錯檔案位置）
}

type invFrame struct {
	ret uint32
	ss  uint16
	sp  uint16
}

const ovrSnapLen = 512

func newInventory(l *Live) *Inventory {
	inv := &Inventory{a: l.A.Entry, g: l.C.Entry, run: -1, seen: map[string]int{}}
	if l.B != nil {
		inv.b = l.B.Entry
	}
	return inv
}

func readPascal(mem CPUView, seg, off uint16) string {
	base := linear(seg, off)
	b := make([]byte, mem.Read8(base))
	for i := range b {
		b[i] = mem.Read8(base + 1 + uint32(i))
	}
	return string(b)
}

func (inv *Inventory) add(e InvEvent, mem CPUView) int {
	base := uint32(e.CallerSeg) << 4
	b := make([]byte, ovrSnapLen)
	for k := range b {
		b[k] = mem.Read8(base + uint32(k))
	}
	key := fmt.Sprintf("%04X:%x", e.CallerSeg, b[:64])
	j, ok := inv.seen[key]
	if !ok {
		j = len(inv.snaps)
		inv.snaps = append(inv.snaps, b)
		inv.seen[key] = j
	}
	e.snap = j
	inv.Events = append(inv.Events, e)
	return len(inv.Events) - 1
}

// before 在路徑 A、B、C 的攔截之前呼叫。
func (inv *Inventory) before(step uint64, cs, ip, ss, sp uint16, mem CPUView) {
	at := linear(cs, ip)
	for len(inv.frames) > 0 {
		f := inv.frames[len(inv.frames)-1]
		if at != f.ret || ss != f.ss || sp <= f.sp {
			break
		}
		inv.frames = inv.frames[:len(inv.frames)-1]
	}
	if at != inv.a && at != inv.b && at != inv.g {
		return
	}
	retOff, retSeg := mem.Read16(linear(ss, sp)), mem.Read16(linear(ss, sp+2))
	arg := func(i int) uint16 { return mem.Read16(linear(ss, sp+4+uint16(2*i))) }
	e := InvEvent{Step: step, Caller: fmt.Sprintf("%04X:%04X", retSeg, retOff), CallerSeg: retSeg}
	switch at {
	case inv.a:
		e.Path, e.Row, e.Col, e.Text = "A", int(arg(4)&0xFF), int(arg(5)&0xFF), readPascal(mem, arg(1), arg(0))
	case inv.b:
		e.Path, e.Row, e.Col, e.Text = "B", int(arg(7)&0xFF), int(arg(8)&0xFF), readPascal(mem, arg(1), arg(0))
	default: // 逐字程序
		if len(inv.frames) > 0 {
			return
		}
		ch, count := rune(arg(1)&0xFF), max(1, int(arg(2)&0xFF))
		row, col := int(arg(5)&0xFF), int(arg(6)&0xFF)
		path := "G"
		if menuSite(mem, linear(retSeg, retOff)) != "" {
			path = "C"
		}
		i := inv.run
		if i < 0 || inv.Events[i].Path != path || inv.Events[i].CallerSeg != retSeg || inv.Events[i].Row != row || col != inv.runCol {
			e.Path, e.Row, e.Col = path, row, col
			i = inv.add(e, mem)
			inv.run = i
		}
		inv.Events[i].Text += strings.Repeat(string(ch), count)
		inv.runCol = col + count
		return
	}
	inv.run = -1
	inv.add(e, mem)
	inv.frames = append(inv.frames, invFrame{linear(retSeg, retOff), ss, sp})
}

// attach 把 Live 這一步新增的命中結果（key、拒絕原因）補到最後一個同路徑事件。
func (inv *Inventory) attach(path string, h Hit) {
	for i := len(inv.Events) - 1; i >= 0; i-- {
		if inv.Events[i].Path == path {
			inv.Events[i].Key, inv.Events[i].Rejected = h.Key, h.Rejected
			if !h.Stamped && h.Rejected == "" {
				inv.Events[i].Rejected = "未覆繪"
			}
			return
		}
	}
}

// ovrBest 與 tools/ovr_match.py 同法：前 256 bytes 每 8 bytes 一個錨點找候選起點，再以前 512 bytes 相同比例評分。
func ovrBest(ovr, seg []byte) (int, float64) {
	cands := map[int]bool{}
	for a := 0; a+8 <= 256; a += 8 {
		for p := 0; ; {
			q := bytes.Index(ovr[p:], seg[a:a+8])
			if q < 0 {
				break
			}
			cands[p+q-a] = true
			p += q + 1
		}
	}
	best, score := -1, 0.0
	for c := range cands {
		if c < 0 || c+len(seg) > len(ovr) {
			continue
		}
		same := 0
		for i := range seg {
			if ovr[c+i] == seg[i] {
				same++
			}
		}
		if r := float64(same) / float64(len(seg)); r > score || r == score && c < best {
			best, score = c, r
		}
	}
	return best, score
}

// WriteTSV 把事件寫成 TSV（step、path、caller、ovr、row、col、key、rejected、text）。
// ovr 是 GAME.OVR 內容（可為 nil），呼叫端段以 ovrBest 對回檔案位置，相同比例低於 0.8 記為 -。
func (inv *Inventory) WriteTSV(path string, ovr []byte) error {
	names := make([]string, len(inv.snaps))
	for i, s := range inv.snaps {
		names[i] = "-"
		if c, r := ovrBest(ovr, s); ovr != nil && c >= 0 && r >= 0.8 {
			names[i] = fmt.Sprintf("ovr@0x%05X:%.3f", c, r)
		}
	}
	var b strings.Builder
	b.WriteString("step\tpath\tcaller\tovr\trow\tcol\tkey\trejected\ttext\n")
	clean := strings.NewReplacer("\t", `\t`, "\n", `\n`, "\r", `\r`)
	for _, e := range inv.Events {
		fmt.Fprintf(&b, "%d\t%s\t%s\t%s\t%d\t%d\t%s\t%s\t%s\n", e.Step, e.Path, e.Caller, names[e.snap],
			e.Row, e.Col, e.Key, e.Rejected, clean.Replace(e.Text))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
