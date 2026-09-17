package xlate

import "encoding/json"

// State 是一筆疊字的狀態（spec 202 §2.3）。
type State int

const (
	Printing State = iota // 原版還在畫這一行
	Pending               // 畫完了，等下一幀定色
	Shown                 // 已定色、顯示中
)

// Stamp 是一行要蓋在原版畫面上的譯文（spec 202 §2.3）。
//
// X、Y 是左上角，原版像素座標；Cells 個字格，每格 CellW×CellH 原版像素。
// Font、GlyphX、GlyphY、GlyphScale 決定字模怎麼畫進放大後的格子（§2.4）。
type Stamp struct {
	Key   string // 呼叫端的識別（例：文本檔 key）
	X, Y  int
	Cells int
	CellW int
	CellH int

	Font       *Font
	GlyphX     int
	GlyphY     int
	GlyphScale int // 0 表示 scale/3（見 §2.4，Draw 裡展開）

	Text  []rune
	// Transparent 標出不蓋的格（長度可以短於 Cells，缺的當 false）：不填背景、不畫字、不列入定色與指紋
	// （spec 202 §2.3）。用途：原版在這幾格畫玩家輸入的字，疊字不能蓋掉，也不能因為輸入變動而失效。
	Transparent []bool
	State       State
	FG    [3]uint8
	BG    [3]uint8

	hash   uint64 // Frame 定色時記的指紋，Shown 之後用來判斷是否還有效
	misses int    // 連續指紋不同的次數
}

// Rect 回這一筆蓋住的原版像素範圍 [x0,x1)×[y0,y1)。
func (s *Stamp) Rect() (x0, y0, x1, y1 int) {
	return s.X, s.Y, s.X + s.Cells*s.CellW, s.Y + s.CellH
}

// Layer 是目前所有疊字。W、H 是原版畫面大小，0 當 320×200（spec 202 §2.3）。
type Layer struct {
	Stamps []*Stamp
	// OnDrop 在一筆被移除時呼叫（原因：overlap、scroll、changed）。可為 nil。
	// 不會被 Snapshot／Restore 保存——那是呼叫端接上去的 hook，不是狀態。
	OnDrop func(s *Stamp, why string)
	W, H   int
}

func overlap(a, b *Stamp) bool {
	ax0, ay0, ax1, ay1 := a.Rect()
	bx0, by0, bx1, by1 := b.Rect()
	return ax0 < bx1 && bx0 < ax1 && ay0 < by1 && by0 < ay1
}

// Add 加一筆；與它重疊的舊疊字移除（原因 overlap）。
func (l *Layer) Add(s *Stamp) {
	keep := l.Stamps[:0]
	for _, old := range l.Stamps {
		if overlap(old, s) {
			l.drop(old, "overlap")
			continue
		}
		keep = append(keep, old)
	}
	l.Stamps = append(keep, s)
}

func (l *Layer) drop(s *Stamp, why string) {
	if l.OnDrop != nil {
		l.OnDrop(s, why)
	}
}

// Scroll 把左上角落在 [x0,x1)×[y0,y1) 的疊字移 dy 像素；移出框的移除（原因 scroll）。
func (l *Layer) Scroll(x0, y0, x1, y1, dy int) {
	keep := l.Stamps[:0]
	for _, s := range l.Stamps {
		if s.X >= x0 && s.X < x1 && s.Y >= y0 && s.Y < y1 {
			s.Y += dy
			if s.Y < y0 || s.Y+s.CellH > y1 {
				l.drop(s, "scroll")
				continue
			}
		}
		keep = append(keep, s)
	}
	l.Stamps = keep
}

// Colors 回一塊色號裡出現最多（背景）與第二多（前景）的色號；只有一種時兩者相同。
func Colors(idx []uint8) (bg, fg uint8) {
	var count [256]int
	for _, v := range idx {
		count[v]++
	}
	best, second := -1, -1
	for c := 0; c < 256; c++ {
		if count[c] == 0 {
			continue
		}
		if best < 0 || count[c] > count[best] {
			best, second = c, best
		} else if second < 0 || count[c] > count[second] {
			second = c
		}
	}
	if best < 0 {
		return 0, 0
	}
	if second < 0 {
		second = best
	}
	return uint8(best), uint8(second)
}

// transparent 回第 i 格是不是透明格。
func (s *Stamp) transparent(i int) bool { return i >= 0 && i < len(s.Transparent) && s.Transparent[i] }

// covered 回原版像素 (x, y) 是否在這一筆的非透明格內（x 已知在矩形內）。
func (s *Stamp) covered(x int) bool { return !s.transparent((x - s.X) / s.CellW) }

// region 取這一筆矩形內非透明格的色號，w、h 是畫面尺寸（超出畫面的部分跳過）。
func (s *Stamp) region(indexed []uint8, w, h int) []uint8 {
	x0, y0, x1, y1 := s.Rect()
	out := make([]uint8, 0, (x1-x0)*(y1-y0))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x >= 0 && x < w && y >= 0 && y < h && s.covered(x) {
				out = append(out, indexed[y*w+x])
			}
		}
	}
	return out
}

func fnv(b []uint8) uint64 {
	h := uint64(14695981039346656037)
	for _, v := range b {
		h ^= uint64(v)
		h *= 1099511628211
	}
	return h
}

// pick 從 rgb 取色號 c 在這一筆範圍內第一次出現的位置的顏色。
func pick(s *Stamp, indexed, rgb []uint8, c uint8, w, h int) [3]uint8 {
	x0, y0, x1, y1 := s.Rect()
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x >= 0 && x < w && y >= 0 && y < h && s.covered(x) && indexed[y*w+x] == c {
				i := 3 * (y*w + x)
				return [3]uint8{rgb[i], rgb[i+1], rgb[i+2]}
			}
		}
	}
	return [3]uint8{}
}

// Frame 在機器停下來之後呼叫一次：定色、檢查失效（spec 202 §2.3）。
// indexed 是原版色號畫面（寬 l.width()），rgb 是同一幀的 RGB。
func (l *Layer) Frame(indexed, rgb []uint8) {
	w, h := l.width(), l.height()
	keep := l.Stamps[:0]
	for _, s := range l.Stamps {
		switch s.State {
		case Pending:
			reg := s.region(indexed, w, h)
			bg, fg := Colors(reg)
			s.BG, s.FG = pick(s, indexed, rgb, bg, w, h), pick(s, indexed, rgb, fg, w, h)
			s.hash, s.misses, s.State = fnv(reg), 0, Shown
		case Shown:
			if fnv(s.region(indexed, w, h)) != s.hash {
				s.misses++
				if s.misses >= 3 {
					l.drop(s, "changed")
					continue
				}
			} else {
				s.misses = 0
			}
		}
		keep = append(keep, s)
	}
	l.Stamps = keep
}

// Draw 把顯示中的疊字畫進放大後的 RGBA（寬 l.width()×scale）。scale 必須是 3 的倍數。
// missing 對字型沒有的字呼叫（可為 nil）。回有沒有畫任何東西。
func (l *Layer) Draw(dst []uint8, scale int, missing func(r rune)) bool {
	if scale%3 != 0 {
		return false
	}
	W := l.width() * scale
	drew := false
	for _, s := range l.Stamps {
		if s.State != Shown {
			continue
		}
		drew = true
		x0, y0, x1, y1 := s.Rect()
		for y := y0 * scale; y < y1*scale; y++ {
			for x := x0 * scale; x < x1*scale; x++ {
				if s.covered(x / scale) {
					set(dst, W, x, y, s.BG)
				}
			}
		}
		if s.Font == nil {
			continue
		}
		drawGlyphs(dst, W, s, scale, missing)
	}
	return drew
}

// drawGlyphs 畫 s 的字模。每格的字模以前景色畫在「格左上 ＋ (GlyphX, GlyphY)」；
// 字模每個點畫成 k×k（k = GlyphScale，0 表示 scale/3，spec 202 §2.4）。
// 超出格緣（寬或高）的點不畫——呼叫端保證「格寬 × scale ≥ GlyphX ＋ 字寬 × GlyphScale」，
// 這裡的邊界檢查是最後一道防線，也同樣套用在高度方向。
func drawGlyphs(dst []uint8, W int, s *Stamp, scale int, missing func(r rune)) {
	k := s.GlyphScale
	if k == 0 {
		k = scale / 3
	}
	rowBytes := s.Font.rowBytes()
	cellW := s.CellW * scale
	cellH := s.CellH * scale
	for i, r := range s.Text {
		if i >= s.Cells || r == ' ' || r == '　' || s.transparent(i) {
			continue
		}
		g, ok := s.Font.Glyphs[r]
		if !ok {
			if missing != nil {
				missing(r)
			}
			continue
		}
		cx, cy := (s.X+i*s.CellW)*scale, s.Y*scale
		for gy := 0; gy < s.Font.H; gy++ {
			for gx := 0; gx < s.Font.W; gx++ {
				if g[gy*rowBytes+gx/8]&(0x80>>uint(gx%8)) == 0 {
					continue
				}
				for dy := 0; dy < k; dy++ {
					ly := s.GlyphY + gy*k + dy
					if ly < 0 || ly >= cellH {
						continue
					}
					for dx := 0; dx < k; dx++ {
						lx := s.GlyphX + gx*k + dx
						if lx < 0 || lx >= cellW {
							continue
						}
						set(dst, W, cx+lx, cy+ly, s.FG)
					}
				}
			}
		}
	}
}

func set(dst []uint8, w, x, y int, c [3]uint8) {
	i := 4 * (y*w + x)
	if x < 0 || x >= w || i < 0 || i+3 >= len(dst) {
		return
	}
	dst[i], dst[i+1], dst[i+2], dst[i+3] = c[0], c[1], c[2], 0xFF
}

// stampSnapshot、layerSnapshot 是 Snapshot／Restore 的 JSON 落地格式（spec 202 §2.3）。
// 字型以 Font.Name 記，不重複存字模——Restore 時由呼叫端透過 fonts 參數換回指標。
type stampSnapshot struct {
	Key        string   `json:"key"`
	X          int      `json:"x"`
	Y          int      `json:"y"`
	Cells      int      `json:"cells"`
	CellW      int      `json:"cell_w"`
	CellH      int      `json:"cell_h"`
	Font       string   `json:"font,omitempty"`
	GlyphX     int      `json:"glyph_x"`
	GlyphY     int      `json:"glyph_y"`
	GlyphScale int      `json:"glyph_scale"`
	Text       string   `json:"text"`
	Transp     []bool   `json:"transparent,omitempty"`
	State      State    `json:"state"`
	FG         [3]uint8 `json:"fg"`
	BG         [3]uint8 `json:"bg"`
	Hash       uint64   `json:"hash"`
	Misses     int      `json:"misses"`
}

type layerSnapshot struct {
	W      int             `json:"w"`
	H      int             `json:"h"`
	Stamps []stampSnapshot `json:"stamps"`
}

// Snapshot 把疊字層存成 JSON，供逐步操作（spec 201）跨步保留（spec 202 §2.3）。
// 不含 OnDrop（那是呼叫端接的 hook，不是狀態）。
func (l *Layer) Snapshot() ([]byte, error) {
	snap := layerSnapshot{W: l.W, H: l.H, Stamps: make([]stampSnapshot, len(l.Stamps))}
	for i, s := range l.Stamps {
		name := ""
		if s.Font != nil {
			name = s.Font.Name
		}
		snap.Stamps[i] = stampSnapshot{
			Key: s.Key, X: s.X, Y: s.Y, Cells: s.Cells, CellW: s.CellW, CellH: s.CellH,
			Font: name, GlyphX: s.GlyphX, GlyphY: s.GlyphY, GlyphScale: s.GlyphScale,
			Text: string(s.Text), Transp: s.Transparent, State: s.State, FG: s.FG, BG: s.BG,
			Hash: s.hash, Misses: s.misses,
		}
	}
	return json.Marshal(snap)
}

// Restore 從 Snapshot 存的 JSON 還原疊字層。fonts 是名稱→字型的對照表——
// 每筆疊字存的是 Font.Name，Restore 時用這張表換回 *Font 指標；
// 找不到對應名稱回錯。只換 Stamps、W、H，OnDrop 維持呼叫端原本接的那個。
func (l *Layer) Restore(data []byte, fonts map[string]*Font) error {
	var snap layerSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	stamps := make([]*Stamp, len(snap.Stamps))
	for i, ss := range snap.Stamps {
		var font *Font
		if ss.Font != "" {
			f, ok := fonts[ss.Font]
			if !ok {
				return &fontNotFoundError{name: ss.Font}
			}
			font = f
		}
		stamps[i] = &Stamp{
			Key: ss.Key, X: ss.X, Y: ss.Y, Cells: ss.Cells, CellW: ss.CellW, CellH: ss.CellH,
			Font: font, GlyphX: ss.GlyphX, GlyphY: ss.GlyphY, GlyphScale: ss.GlyphScale,
			Text: []rune(ss.Text), Transparent: ss.Transp, State: ss.State, FG: ss.FG, BG: ss.BG,
			hash: ss.Hash, misses: ss.Misses,
		}
	}
	l.W, l.H = snap.W, snap.H
	l.Stamps = stamps
	return nil
}

type fontNotFoundError struct{ name string }

func (e *fontNotFoundError) Error() string {
	return "xlate: restore 找不到字型 " + e.name
}
