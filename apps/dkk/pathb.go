package dkk

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

// PathBSignature 是 resident 文字視窗印字程序的進入點（規格 002 §2.1，START.EXE 檔案 0x5303，唯一）。
var PathBSignature = sig(
	0x55, 0x89, 0xE5, 0x81, 0xEC, 0x26, 0x01, 0xC4, 0x7E, 0x06, 0x06, 0x57, 0x8D, 0xBE, 0x00, 0xFF,
)

const (
	pathBArgBytes = 0x12   // 9 個 word
	cursorColAddr = 0xB43A // DS:B43A 欄
	cursorRowAddr = 0xB43B // DS:B43B 列
	pathBKey      = "dkk.pathb:"
	planarStride  = 40 // mode 0Dh 一列 40 bytes（8 像素一格）
	planarPage    = 8000
)

// Rect 是文字格座標的視窗矩形，含端點。
type Rect struct{ Top, Left, Bottom, Right int }

func (r Rect) contains(row, col int) bool {
	return row >= r.Top && row <= r.Bottom && col >= r.Left && col <= r.Right
}

type bPage struct {
	rect               Rect
	bg, fg             uint8
	startRow, startCol int
	curRow, curCol     int
	lastRow            int
	grid               map[int][]rune // 列 → 左欄到右欄的格子，0 為空
	stamps             map[int]*xlate.Stamp
}

func (p *bPage) maskFrom(row int) int {
	if row == p.startRow {
		return p.startCol
	}
	return p.rect.Left
}

type bCall struct {
	ret    uint32
	ss, sp uint16
	rect   Rect
}

// PathB 依規格 002 呈現文字視窗的中文。
type PathB struct {
	Entry   uint32
	Lookups []Lookuper // 依序查：ECL、介面（含規格 004 組句）
	Font    *xlate.Font
	Layer   *xlate.Layer
	Hits    []Hit
	// OnEntry 在每次合法進入時以原字串呼叫（規格 007 手札面板用）；可為 nil。
	OnEntry func(text []byte, mem CPUView)

	pal   [256][3]uint8
	pages []*bPage
	call  *bCall
}

// NewPathB 搜尋進入點；命中數不是 1 就回錯。也設定 Layer.Frozen 讓 xlate 不碰路徑 B 的 Stamp。
func NewPathB(mem CPUView, font *xlate.Font, layer *xlate.Layer, lookups ...Lookuper) (*PathB, error) {
	hits := PathBSignature.Find(mem.Read8, 0, 0xA0000)
	if len(hits) != 1 {
		return nil, fmt.Errorf("dkk: 路徑 B 簽章命中 %d 處，要恰好 1 處", len(hits))
	}
	if font == nil || font.W != 16 || font.H != 16 {
		return nil, fmt.Errorf("dkk: 路徑 B 需要 16×16 字型")
	}
	prev := layer.Frozen
	layer.Frozen = func(s *xlate.Stamp) bool {
		return strings.HasPrefix(s.Key, pathBKey) || (prev != nil && prev(s))
	}
	return &PathB{Entry: hits[0], Lookups: lookups, Font: font, Layer: layer}, nil
}

// SetPalette 更新顏色（畫格的 FramePalette），並刷新既有 Stamp。
func (b *PathB) SetPalette(pal [256][3]uint8) {
	b.pal = pal
	for _, p := range b.pages {
		for _, s := range p.stamps {
			s.BG, s.FG = pal[p.bg], pal[p.fg]
		}
	}
}

func (b *PathB) lookup(text []byte) (Entry, bool) {
	for _, c := range b.Lookups {
		if e, ok := c.Lookup(text); ok {
			return e, true
		}
	}
	trimmed := bytes.TrimSpace(text)
	if len(trimmed) > 0 && len(trimmed) < len(text) {
		for _, c := range b.Lookups {
			if e, ok := c.Lookup(trimmed); ok {
				return e, true
			}
		}
	}
	return Entry{}, false
}

// Before 在每道指令前呼叫。
func (b *PathB) Before(step uint64, cs, ip, ss, sp, ds uint16, mem CPUView) {
	at := linear(cs, ip)
	if b.call != nil && at == b.call.ret && ss == b.call.ss && sp == b.call.sp {
		b.call = nil
	}
	if at != b.Entry {
		return
	}
	arg := func(i int) int { return int(mem.Read16(linear(ss, sp+4+uint16(2*i))) & 0xFF) }
	retOff, retSeg := mem.Read16(linear(ss, sp)), mem.Read16(linear(ss, sp+2))
	strOff, strSeg := mem.Read16(linear(ss, sp+4)), mem.Read16(linear(ss, sp+6))
	flag, bg, fg := arg(2), arg(3), arg(4)
	r := Rect{Top: arg(7), Left: arg(8), Bottom: arg(5), Right: arg(6)}
	h := Hit{Step: step, Caller: fmt.Sprintf("%04X:%04X", retSeg, retOff), Row: r.Top, Col: r.Left, BG: bg, FG: fg}
	defer func() { b.Hits = append(b.Hits, h) }()
	if r.Left > 0x27 || r.Right > 0x27 || r.Bottom > 0x27 || r.Top > 0x18 || r.Left > r.Right || r.Top > r.Bottom || r.Bottom > 24 {
		h.Rejected = "視窗參數不合法"
		return
	}
	b.call = &bCall{ret: linear(retSeg, retOff), ss: ss, sp: sp + 4 + pathBArgBytes, rect: r}
	base := linear(strSeg, strOff)
	n := int(mem.Read8(base))
	text := make([]byte, n)
	for i := range text {
		text[i] = mem.Read8(base + 1 + uint32(i))
	}
	h.Length = n
	if b.OnEntry != nil {
		b.OnEntry(text, mem)
	}
	oc, or := int(mem.Read8(linear(ds, cursorColAddr))), int(mem.Read8(linear(ds, cursorRowAddr)))
	e, hit := b.lookup(text)
	if hit {
		h.Key = e.Key
	}
	page := b.pageAt(r)
	newPage := flag != 0 || !r.contains(or, oc)
	var runes []rune
	switch {
	case newPage:
		b.removeIntersecting(r)
		if !hit {
			h.Rejected = "未命中（新頁）"
			return
		}
		page = b.newPage(r, uint8(bg), uint8(fg), r.Top, r.Left)
		runes = PackHalfWidth([]rune(e.Translation))
	case page == nil:
		if !hit {
			h.Rejected = "未命中（無有效頁）"
			return
		}
		page = b.newPage(r, uint8(bg), uint8(fg), or, oc)
		runes = PackHalfWidth([]rune(e.Translation))
	default:
		if oc == r.Left && page.curCol != r.Left {
			page.curRow, page.curCol = page.curRow+1, r.Left
		}
		if hit {
			runes = PackHalfWidth([]rune(e.Translation))
		} else {
			runes = []rune(string(text)) // 照原文接入（名字、數字）
			h.Rejected = "未命中（照原文接入）"
		}
	}
	if !layout(page, runes) {
		b.removePage(page)
		h.Rejected = "溢出"
		return
	}
	b.rebuild(page)
	if h.Rejected == "" {
		h.Stamped = true
	}
}

func (b *PathB) pageAt(r Rect) *bPage {
	for _, p := range b.pages {
		if p.rect == r {
			return p
		}
	}
	return nil
}

func (b *PathB) newPage(r Rect, bg, fg uint8, row, col int) *bPage {
	p := &bPage{rect: r, bg: bg, fg: fg, startRow: row, startCol: col, curRow: row, curCol: col, lastRow: row,
		grid: map[int][]rune{}, stamps: map[int]*xlate.Stamp{}}
	b.pages = append(b.pages, p)
	return p
}

// isClosePunct 是不放行首的全形句讀。
func isClosePunct(r rune) bool { return strings.ContainsRune("，。、；：！？」』）…", r) }

func isWordRune(r rune) bool { return r < 0x80 && r != ' ' || isPair(r) } // 組合字不從中間斷行（規格 005）

// layout 把 runes 從頁的中文游標排進格子；溢出回 false（不改格子）。
func layout(p *bPage, runes []rune) bool {
	width := p.rect.Right - p.rect.Left + 1
	row, col := p.curRow, p.curCol
	type cell struct {
		row, col int
		r        rune
	}
	var placed []cell
	newline := func() {
		row, col = row+1, p.rect.Left
	}
	for i := 0; i < len(runes); {
		r := runes[i]
		if r == ' ' && col == p.rect.Left && len(placed) > 0 && placed[len(placed)-1].row != row {
			i++ // 行首空白略過
			continue
		}
		// 拉丁詞不拆行
		j := i + 1
		if isWordRune(r) {
			for j < len(runes) && isWordRune(runes[j]) {
				j++
			}
		}
		n := j - i
		if col+n-1 > p.rect.Right && col != p.rect.Left && n <= width {
			newline()
			if r == ' ' {
				i++
				continue
			}
		}
		if isClosePunct(r) && col == p.rect.Left && len(placed) > 0 && placed[len(placed)-1].row == row-1 {
			last := placed[len(placed)-1]
			placed = placed[:len(placed)-1]
			placed = append(placed, cell{row, col, last.r})
			col++
		}
		for k := i; k < j; k++ {
			if col > p.rect.Right {
				newline()
			}
			placed = append(placed, cell{row, col, runes[k]})
			col++
		}
		i = j
	}
	for _, c := range placed {
		if c.row > p.rect.Bottom {
			return false
		}
	}
	for _, c := range placed {
		g := p.grid[c.row]
		if g == nil {
			g = make([]rune, width)
			p.grid[c.row] = g
		}
		g[c.col-p.rect.Left] = c.r
		if c.row > p.lastRow {
			p.lastRow = c.row
		}
	}
	p.curRow, p.curCol = row, col
	return true
}

// rebuild 依格子重建該頁每一列的 Stamp：起始列到視窗底列都遮罩（規格 002 §2.4，同拯救地球 027 §3.4）。
// 原版後續接續呼叫的英文印在下方列，中文較短時只遮到中文最後一列會露出英文。
func (b *PathB) rebuild(p *bPage) {
	for row := p.startRow; row <= p.rect.Bottom; row++ {
		from := p.maskFrom(row)
		g := p.grid[row]
		text := make([]rune, p.rect.Right-from+1)
		for c := from; c <= p.rect.Right; c++ {
			text[c-from] = ' '
			if g != nil && g[c-p.rect.Left] != 0 {
				text[c-from] = g[c-p.rect.Left]
			}
		}
		if old := p.stamps[row]; old != nil {
			old.Text = text
			continue
		}
		s := &xlate.Stamp{
			Key: fmt.Sprintf("%s%d.%d.%d", pathBKey, p.rect.Top, p.rect.Left, row), X: from * 8, Y: row * 8,
			Cells: len(text), CellW: 8, CellH: 8, Font: b.Font, Text: text, State: xlate.Shown,
			BG: b.pal[p.bg], FG: b.pal[p.fg],
		}
		p.stamps[row] = s
		b.Layer.Stamps = append(b.Layer.Stamps, s)
	}
}

func (b *PathB) removeRow(p *bPage, row int) {
	s := p.stamps[row]
	if s == nil {
		return
	}
	delete(p.stamps, row)
	b.dropStamp(s)
	if len(p.stamps) == 0 {
		b.removePage(p)
	}
}

func (b *PathB) dropStamp(s *xlate.Stamp) {
	keep := b.Layer.Stamps[:0]
	for _, x := range b.Layer.Stamps {
		if x != s {
			keep = append(keep, x)
		}
	}
	b.Layer.Stamps = keep
}

func (b *PathB) removePage(p *bPage) {
	for _, s := range p.stamps {
		b.dropStamp(s)
	}
	p.stamps = map[int]*xlate.Stamp{}
	keep := b.pages[:0]
	for _, x := range b.pages {
		if x != p {
			keep = append(keep, x)
		}
	}
	b.pages = keep
}

// removeIntersecting 移除各頁中列落在 r 內、遮罩欄範圍與 r 相交的列（規格 002 §2.5 第 1 條）。
func (b *PathB) removeIntersecting(r Rect) {
	for _, p := range append([]*bPage(nil), b.pages...) {
		for row := range p.stamps {
			if row < r.Top || row > r.Bottom {
				continue
			}
			if p.maskFrom(row) <= r.Right && p.rect.Right >= r.Left {
				b.removeRow(p, row)
			}
		}
	}
}

// VideoWrite 處理 A000 寫入（規格 002 §2.5 第 2 條）。offset 是相對 A000 的位移。
func (b *PathB) VideoWrite(offset uint32) {
	if offset >= planarPage || len(b.pages) == 0 {
		return
	}
	row, col := int(offset/planarStride)/8, int(offset%planarStride)
	if b.call != nil && b.call.rect.contains(row, col) {
		return // 呼叫期間這個視窗的輸出
	}
	for _, p := range append([]*bPage(nil), b.pages...) {
		if p.stamps[row] != nil && col >= p.maskFrom(row) && col <= p.rect.Right {
			b.removeRow(p, row)
		}
	}
}

// Reset 移除全部呈現（視訊模式改變等不連續點）。
func (b *PathB) Reset() {
	for _, p := range append([]*bPage(nil), b.pages...) {
		b.removePage(p)
	}
	b.call = nil
}
