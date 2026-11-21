package dkk

import (
	"fmt"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// GlyphSignature 是 resident 逐字程序 05F4:01F5 的進入點（規格 003 §2.1，START.EXE 檔案 0x504D，唯一）。
var GlyphSignature = sig(
	0x55, 0x89, 0xE5, 0x83, 0xEC, 0x12, 0x80, 0x7E, 0x12, 0x27, 0x77, 0x0F, 0x80, 0x7E, 0x10, 0x18,
)

const (
	glyphArgBytes = 14 // 7 個 word
	pathCKey      = "dkk.pathc:"
	menuCols      = 40
)

// menuCallSites 是選單模組呼叫 01F5 的返回位址後 bytes（規格 003 §2.1）。
var menuCallSites = []struct {
	name  string
	after []byte
}{
	{"selected", []byte{0xE9, 0x96, 0x00}},
	{"hotkey", []byte{0xEB, 0x3C}},
	{"normal", []byte{0xFE, 0x46, 0xFF}},
	{"pad", []byte{0xC4, 0x7E, 0xFC, 0x26, 0x8B, 0x45, 0x2A}},
}

type menuCell struct {
	r      byte
	bg, fg uint8
	set    bool
}

type gCall struct {
	ret    uint32
	ss, sp uint16
	row    int
}

// PathC 依規格 003 呈現底列水平選單的中文。
type PathC struct {
	Entry   uint32
	Catalog Lookuper
	Font    *xlate.Font
	Layer   *xlate.Layer
	Words   []MenuWord // 最近一次重組的結果（收據用）

	pal    [256][3]uint8
	rows   map[int]*[menuCols]menuCell
	dirty  map[int]bool
	stamps map[int][]*xlate.Stamp
	call   *gCall
}

// MenuWord 記錄一個詞的重組結果。原文只存長度。
type MenuWord struct {
	Row, Col, Length int
	Key              string
	Drawn            bool
}

// NewPathC 搜尋逐字程序；命中數不是 1 就回錯。
func NewPathC(mem CPUView, cat Lookuper, font *xlate.Font, layer *xlate.Layer) (*PathC, error) {
	hits := GlyphSignature.Find(mem.Read8, 0, 0xA0000)
	if len(hits) != 1 {
		return nil, fmt.Errorf("dkk: 路徑 C 逐字程序簽章命中 %d 處，要恰好 1 處", len(hits))
	}
	if font == nil || font.W != 16 || font.H != 16 {
		return nil, fmt.Errorf("dkk: 路徑 C 需要 16×16 字型")
	}
	prev := layer.Frozen
	layer.Frozen = func(s *xlate.Stamp) bool {
		return strings.HasPrefix(s.Key, pathCKey) || (prev != nil && prev(s))
	}
	return &PathC{Entry: hits[0], Catalog: cat, Font: font, Layer: layer,
		rows: map[int]*[menuCols]menuCell{}, dirty: map[int]bool{}, stamps: map[int][]*xlate.Stamp{}}, nil
}

func menuSite(mem CPUView, ret uint32) string {
	if mem.Read8(ret-5) != 0x9A || mem.Read8(ret-4) != 0xF5 || mem.Read8(ret-3) != 0x01 {
		return ""
	}
	for _, s := range menuCallSites {
		ok := true
		for i, b := range s.after {
			if mem.Read8(ret+uint32(i)) != b {
				ok = false
				break
			}
		}
		if ok {
			return s.name
		}
	}
	return ""
}

// Before 在每道指令前呼叫。
func (c *PathC) Before(cs, ip, ss, sp uint16, mem CPUView) {
	at := linear(cs, ip)
	if c.call != nil && at == c.call.ret && ss == c.call.ss && sp == c.call.sp {
		c.call = nil
	}
	if at != c.Entry {
		return
	}
	retOff, retSeg := mem.Read16(linear(ss, sp)), mem.Read16(linear(ss, sp+2))
	ret := linear(retSeg, retOff)
	if menuSite(mem, ret) == "" {
		return
	}
	arg := func(i int) int { return int(mem.Read16(linear(ss, sp+4+uint16(2*i))) & 0xFF) }
	ch, count, bg, fg, row, col := byte(arg(1)), arg(2), uint8(arg(3)), uint8(arg(4)), arg(5), arg(6)
	if row > 0x18 || col >= menuCols {
		return
	}
	c.call = &gCall{ret: ret, ss: ss, sp: sp + 4 + glyphArgBytes, row: row}
	cells := c.rows[row]
	if cells == nil {
		cells = &[menuCols]menuCell{}
		c.rows[row] = cells
	}
	for i := 0; i < count && col+i < menuCols; i++ {
		cells[col+i] = menuCell{r: ch, bg: bg, fg: fg, set: true}
	}
	c.dirty[row] = true
}

// VideoWrite 處理呼叫期間外的 A000 寫入（規格 003 §2.4）。
func (c *PathC) VideoWrite(offset uint32) {
	if offset >= planarPage || len(c.rows) == 0 {
		return
	}
	row, col := int(offset/planarStride)/8, int(offset%planarStride)
	if c.call != nil && c.call.row == row {
		return
	}
	cells := c.rows[row]
	if cells == nil || !cells[col].set {
		return
	}
	cells[col] = menuCell{}
	c.dirty[row] = true
}

// Reset 清除全部呈現。
func (c *PathC) Reset() {
	for row := range c.stamps {
		c.clearStamps(row)
	}
	c.rows = map[int]*[menuCols]menuCell{}
	c.dirty = map[int]bool{}
	c.call = nil
}

func (c *PathC) clearStamps(row int) {
	old := c.stamps[row]
	if len(old) == 0 {
		return
	}
	drop := map[*xlate.Stamp]bool{}
	for _, s := range old {
		drop[s] = true
	}
	keep := c.Layer.Stamps[:0]
	for _, s := range c.Layer.Stamps {
		if !drop[s] {
			keep = append(keep, s)
		}
	}
	c.Layer.Stamps = keep
	delete(c.stamps, row)
}

type menuWord struct {
	col   int
	cells []menuCell
}

func (w menuWord) text() string {
	b := make([]byte, len(w.cells))
	for i, x := range w.cells {
		b[i] = x.r
	}
	return string(b)
}

// colors 回詞的多數 bg／fg，與熱鍵格（fg 與多數不同者；沒有回 -1）。
func (w menuWord) colors() (bg, fg uint8, hot int) {
	count := map[[2]uint8]int{}
	for _, x := range w.cells {
		count[[2]uint8{x.bg, x.fg}]++
	}
	best, n := [2]uint8{}, -1
	for k, v := range count {
		if v > n || (v == n && (k[0] < best[0] || (k[0] == best[0] && k[1] < best[1]))) {
			best, n = k, v
		}
	}
	hot = -1
	for i, x := range w.cells {
		if x.fg != best[1] || x.bg != best[0] {
			hot = i
			break
		}
	}
	return best[0], best[1], hot
}

func splitWords(cells *[menuCols]menuCell) []menuWord {
	var ws []menuWord
	for i := 0; i < menuCols; {
		if !cells[i].set || cells[i].r == ' ' {
			i++
			continue
		}
		j := i
		for j < menuCols && cells[j].set && cells[j].r != ' ' {
			j++
		}
		ws = append(ws, menuWord{col: i, cells: append([]menuCell(nil), cells[i:j]...)})
		i = j
	}
	return ws
}

// SetPalette 更新顏色並在畫格前重組有變動的列。
func (c *PathC) SetPalette(pal [256][3]uint8) {
	c.pal = pal
	if len(c.dirty) > 0 {
		c.Words = c.Words[:0]
	}
	for row := range c.dirty {
		c.rebuild(row)
	}
	c.dirty = map[int]bool{}
}

// menuWordser 由查表提供選單母串的逐詞譯文（規格 003 §2.2.1；Translator 滿足）。
type menuWordser interface {
	MenuWords(words []string) []string
}

func (c *PathC) rebuild(row int) {
	c.clearStamps(row)
	cells := c.rows[row]
	if cells == nil {
		return
	}
	words := splitWords(cells)
	// 整列查表，詞數相同時逐詞對應（規格 003 §2.2）。
	var lineWords []string
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = w.text()
	}
	if e, ok := c.Catalog.Lookup([]byte(strings.Join(parts, " "))); ok && len(words) > 1 {
		if f := strings.Fields(e.Translation); len(f) == len(words) {
			lineWords = f
		} else if c.wholeLine(row, words, e) {
			// 整句提示（例：換頁的 Press button or enter to continue 是單一選項）：
			// 詞數對不上時，整句中文畫在第一詞到最後一詞的範圍內（規格 003 §2.2）。
			return
		}
	}
	var masterWords []string
	if mm, ok := c.Catalog.(menuWordser); ok && lineWords == nil {
		masterWords = mm.MenuWords(parts)
	}
	for i, w := range words {
		mw := MenuWord{Row: row, Col: w.col, Length: len(w.cells)}
		zh := ""
		if e, ok := c.Catalog.Lookup([]byte(parts[i])); ok {
			zh, mw.Key = e.Translation, e.Key
		} else if lineWords != nil {
			zh, mw.Key = lineWords[i], "line"
		} else if masterWords != nil && masterWords[i] != "" {
			zh, mw.Key = masterWords[i], "master"
		}
		runes := PackHalfWidth([]rune(zh))
		if zh == "" || len(runes) > len(w.cells) {
			c.Words = append(c.Words, mw)
			continue
		}
		bg, fg, hot := w.colors()
		text := append([]rune(nil), runes...)
		for len(text) < len(w.cells) {
			text = append(text, ' ')
		}
		s := &xlate.Stamp{Key: fmt.Sprintf("%s%d.%d", pathCKey, row, w.col), X: w.col * 8, Y: row * 8,
			Cells: len(w.cells), CellW: 8, CellH: 8, Font: c.Font, Text: text, State: xlate.Shown,
			BG: c.pal[bg], FG: c.pal[fg]}
		c.Layer.Stamps = append(c.Layer.Stamps, s)
		c.stamps[row] = append(c.stamps[row], s)
		if hot >= 0 && len(runes) < len(w.cells) {
			h := w.cells[hot]
			hs := &xlate.Stamp{Key: fmt.Sprintf("%s%d.%d.hot", pathCKey, row, w.col), X: (w.col + len(runes)) * 8, Y: row * 8,
				Cells: 1, CellW: 8, CellH: 8, Font: c.Font, Text: []rune{rune(h.r)}, State: xlate.Shown,
				BG: c.pal[h.bg], FG: c.pal[h.fg]}
			c.Layer.Stamps = append(c.Layer.Stamps, hs)
			c.stamps[row] = append(c.stamps[row], hs)
		}
		mw.Drawn = true
		c.Words = append(c.Words, mw)
	}
}

// wholeLine 把整列譯文畫在 words 的總範圍；放不下回 false。
func (c *PathC) wholeLine(row int, words []menuWord, e Entry) bool {
	first, last := words[0], words[len(words)-1]
	from, to := first.col, last.col+len(last.cells)
	runes := PackHalfWidth([]rune(e.Translation))
	if len(runes) > to-from {
		return false
	}
	text := append([]rune(nil), runes...)
	for len(text) < to-from {
		text = append(text, ' ')
	}
	bg, fg, _ := first.colors()
	s := &xlate.Stamp{Key: fmt.Sprintf("%s%d.%d.line", pathCKey, row, from), X: from * 8, Y: row * 8,
		Cells: to - from, CellW: 8, CellH: 8, Font: c.Font, Text: text, State: xlate.Shown,
		BG: c.pal[bg], FG: c.pal[fg]}
	c.Layer.Stamps = append(c.Layer.Stamps, s)
	c.stamps[row] = append(c.stamps[row], s)
	c.Words = append(c.Words, MenuWord{Row: row, Col: from, Length: to - from, Key: e.Key, Drawn: true})
	return true
}
