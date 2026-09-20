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
	Owner string // 建立它的 watcher 的 Key（spec 203）；一般疊字是空字串
	X, Y  int
	Cells int
	CellW int
	CellH int

	Font       *Font
	GlyphX     int
	GlyphY     int
	GlyphScale int // 0 表示 max(1, scale/3)（見 §2.4，Draw 裡展開）

	Text []rune
	// Transparent 標出不蓋的格（長度可以短於 Cells，缺的當 false）：不填背景、不畫字、不列入定色與指紋
	// （spec 202 §2.3）。用途：原版在這幾格畫玩家輸入的字，疊字不能蓋掉，也不能因為輸入變動而失效。
	Transparent []bool
	// SwapColors 把定色的背景與前景對調（spec 202 §2.3）。用途：字比底密的區塊
	// （字模填滿整條橫幅時，字的像素多於底），「最多的當背景」在那裡會反過來。
	SwapColors bool
	State      State
	FG         [3]uint8
	BG         [3]uint8

	hashes  []uint64 // 每一格的指紋（Frame 定色時記），Shown 之後用來判斷哪幾格還有效
	misses  []int    // 每一格連續指紋不同的次數
	anchors []bool   // 每一格定色時是否壓在原文墨跡上（spec 202 §2.3）；全部失效就整筆移除
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
	// Frozen 回 true 的疊字，這次 Frame 不定色也不檢查指紋（可為 nil，spec 202 §2.3）。
	// 用途：原版正在搬動這一塊（例如訊息框逐步捲動、顯存複製到一半），中間狀態不能拿來判斷失效。
	Frozen func(s *Stamp) bool
	W, H   int

	watchers []*Watcher // spec 203：以畫面內容當觸發點
}

func overlap(a, b *Stamp) bool {
	ax0, ay0, ax1, ay1 := a.Rect()
	bx0, by0, bx1, by1 := b.Rect()
	return ax0 < bx1 && bx0 < ax1 && ay0 < by1 && by0 < ay1
}

// Add 加一筆。舊疊字被新的**整個蓋住**才移除（原因 overlap）；
// 只重疊一部分時，把被蓋到的那幾格標成透明（不畫、不列入指紋），其餘照常顯示（spec 202 §2.3）。
//
// 為什麼不整筆移除：原版會在訊息框旁邊開別的框（例如輸入檔名的面板），只蓋住訊息的右半。
// 整筆移除的話，沒被蓋到的左半會露出原版英文。
func (l *Layer) Add(s *Stamp) {
	keep := l.Stamps[:0]
	nx0, ny0, nx1, ny1 := s.Rect()
	for _, old := range l.Stamps {
		if !overlap(old, s) {
			keep = append(keep, old)
			continue
		}
		ox0, oy0, ox1, oy1 := old.Rect()
		// watcher 加的疊字（Owner 非空）整筆移除，不遮格子：它的有效性由整塊區域定義，
		// 遮一半會永遠留著半行原版英文（spec 203 §2.1）。移除之後 watcher 會在畫面回到原樣時重新蓋。
		if old.Owner != "" || (nx0 <= ox0 && nx1 >= ox1 && ny0 <= oy0 && ny1 >= oy1) {
			l.drop(old, "overlap")
			continue
		}
		if old.CellW > 0 {
			for i := 0; i < old.Cells; i++ {
				cx0 := ox0 + i*old.CellW
				if cx0 < nx1 && nx0 < cx0+old.CellW {
					old.setTransparent(i)
				}
			}
		}
		if old.allTransparent() {
			l.drop(old, "overlap")
			continue
		}
		old.State = Pending // 剩下的格子要重新定色與重算指紋
		keep = append(keep, old)
	}
	l.Stamps = append(keep, s)
}

// cellHashes 算每一格的指紋。
func (s *Stamp) cellHashes(indexed []uint8, w, h int) []uint64 {
	out := make([]uint64, s.Cells)
	for i := range out {
		out[i] = fnv(s.cellRegion(indexed, w, h, i))
	}
	return out
}

// cellAnchors 算每一格是不是「錨定格」：定色時該格的原版像素不只一種色號，
// 也就是那一格壓在原文的墨跡上（spec 202 §2.3）。中文比原文寬時多出來的格子
// 壓在純色背景上，指紋永遠不變，不能拿來判斷原文還在不在。
func (s *Stamp) cellAnchors(indexed []uint8, w, h int) []bool {
	out := make([]bool, s.Cells)
	for i := range out {
		reg := s.cellRegion(indexed, w, h, i)
		for _, v := range reg {
			if v != reg[0] {
				out[i] = true
				break
			}
		}
	}
	return out
}

// anchorsGone 回「有錨定格，而且全部都失效了」。
func (s *Stamp) anchorsGone() bool {
	if len(s.anchors) != s.Cells {
		return false
	}
	any := false
	for i := 0; i < s.Cells; i++ {
		if !s.anchors[i] {
			continue
		}
		any = true
		if !s.transparent(i) {
			return false
		}
	}
	return any
}

// setTransparent 把第 i 格標成透明（陣列不夠長就補）。
func (s *Stamp) setTransparent(i int) {
	if len(s.Transparent) < s.Cells {
		t := make([]bool, s.Cells)
		copy(t, s.Transparent)
		s.Transparent = t
	}
	s.Transparent[i] = true
}

// allTransparent 回這一筆是不是每一格都透明（沒有東西可畫）。
func (s *Stamp) allTransparent() bool {
	if s.Cells <= 0 {
		return true
	}
	for i := 0; i < s.Cells; i++ {
		if !s.transparent(i) {
			return false
		}
	}
	return true
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
		if s.X >= x0 && s.X+s.Cells*s.CellW <= x1 && s.Y >= y0 && s.Y < y1 {
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

// cellRegion 取第 i 格的色號（超出畫面的部分跳過）。
func (s *Stamp) cellRegion(indexed []uint8, w, h, i int) []uint8 {
	x0 := s.X + i*s.CellW
	out := make([]uint8, 0, s.CellW*s.CellH)
	for y := s.Y; y < s.Y+s.CellH; y++ {
		for x := x0; x < x0+s.CellW; x++ {
			if x >= 0 && x < w && y >= 0 && y < h {
				out = append(out, indexed[y*w+x])
			}
		}
	}
	return out
}

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
	l.checkWatchers(indexed)
	w, h := l.width(), l.height()
	keep := l.Stamps[:0]
	for _, s := range l.Stamps {
		if l.Frozen != nil && l.Frozen(s) {
			keep = append(keep, s)
			continue
		}
		switch s.State {
		case Pending:
			reg := s.region(indexed, w, h)
			bg, fg := Colors(reg)
			if s.SwapColors {
				bg, fg = fg, bg
			}
			s.BG, s.FG = pick(s, indexed, rgb, bg, w, h), pick(s, indexed, rgb, fg, w, h)
			s.hashes, s.misses, s.State = s.cellHashes(indexed, w, h), make([]int, s.Cells), Shown
			s.anchors = s.cellAnchors(indexed, w, h)
		case Shown:
			// 以**格**為單位判斷失效：原版在旁邊開另一個框只蓋住這一行的一部分時，
			// 沒被蓋到的格子照常顯示中文（spec 202 §2.3）。
			if len(s.hashes) != s.Cells || len(s.misses) != s.Cells {
				s.State = Pending // 舊快照或格數變了：下一幀重新定色
				keep = append(keep, s)
				continue
			}
			if len(s.anchors) != s.Cells {
				// 舊快照還原的疊字沒有錨定格：拿這一幀補算。原文已經被蓋掉時算出來會是空的，
				// 那時 anchorsGone 回 false，行為退回逐格判斷（spec 202 §2.3）。
				s.anchors = s.cellAnchors(indexed, w, h)
			}
			now := s.cellHashes(indexed, w, h)
			for i := 0; i < s.Cells; i++ {
				if s.transparent(i) {
					continue
				}
				if now[i] != s.hashes[i] {
					s.misses[i]++
					if s.misses[i] >= 3 {
						if s.Owner != "" { // watcher 的疊字整筆失效（同上）
							s.State = Printing // 標記為要移除
							break
						}
						s.setTransparent(i)
					}
				} else {
					s.misses[i] = 0
				}
			}
			if s.State == Printing || s.allTransparent() {
				l.drop(s, "changed")
				continue
			}
			// 錨定格全部失效 ＝ 原版那段文字已經不在畫面上；留下的格子壓在純色背景上，
			// 指紋永遠不變，不移除就會變成孤字（spec 202 §2.3）。
			if s.anchorsGone() {
				l.drop(s, "anchors")
				continue
			}
		}
		keep = append(keep, s)
	}
	l.Stamps = keep
}

// Draw 把顯示中的疊字畫進放大後的 RGBA（寬 l.width()×scale）。scale 必須是正整數。
// missing 對字型沒有的字呼叫（可為 nil）。回有沒有畫任何東西。
func (l *Layer) Draw(dst []uint8, scale int, missing func(r rune)) bool {
	if scale <= 0 {
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
// 字模每個點畫成 k×k（k = GlyphScale，0 表示 max(1, scale/3)，spec 202 §2.4）。
// 超出格緣（寬或高）的點不畫——呼叫端保證「格寬 × scale ≥ GlyphX ＋ 字寬 × GlyphScale」，
// 這裡的邊界檢查是最後一道防線，也同樣套用在高度方向。
func drawGlyphs(dst []uint8, W int, s *Stamp, scale int, missing func(r rune)) {
	k := s.GlyphScale
	if k == 0 {
		k = scale / 3
		if k < 1 {
			k = 1
		}
	}
	if k < 0 {
		return
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
	Key        string          `json:"key"`
	Owner      string          `json:"owner,omitempty"`
	X          int             `json:"x"`
	Y          int             `json:"y"`
	Cells      int             `json:"cells"`
	CellW      int             `json:"cell_w"`
	CellH      int             `json:"cell_h"`
	Font       string          `json:"font,omitempty"`
	GlyphX     int             `json:"glyph_x"`
	GlyphY     int             `json:"glyph_y"`
	GlyphScale int             `json:"glyph_scale"`
	Text       string          `json:"text"`
	Transp     []bool          `json:"transparent,omitempty"`
	Swap       bool            `json:"swap_colors,omitempty"`
	State      State           `json:"state"`
	FG         [3]uint8        `json:"fg"`
	BG         [3]uint8        `json:"bg"`
	Hashes     json.RawMessage `json:"hashes,omitempty"` // 舊快照是單一數值：讀不成陣列就重新定色
	Misses     json.RawMessage `json:"misses,omitempty"`
	Anchors    []bool          `json:"anchors,omitempty"` // 舊快照沒有：還原後當「沒有錨定格」，照舊逐格判斷
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
			Key: s.Key, Owner: s.Owner, X: s.X, Y: s.Y, Cells: s.Cells, CellW: s.CellW, CellH: s.CellH,
			Font: name, GlyphX: s.GlyphX, GlyphY: s.GlyphY, GlyphScale: s.GlyphScale,
			Text: string(s.Text), Transp: s.Transparent, Swap: s.SwapColors, State: s.State, FG: s.FG, BG: s.BG,
			Hashes: mustJSON(s.hashes), Misses: mustJSON(s.misses), Anchors: s.anchors,
		}
	}
	return json.Marshal(snap)
}

// mustJSON 把切片轉成 JSON（失敗回 nil，快照少一個欄位不影響還原）。
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func decodeHashes(raw json.RawMessage) []uint64 {
	var out []uint64
	if len(raw) == 0 || json.Unmarshal(raw, &out) != nil {
		return nil
	}
	return out
}

func decodeMisses(raw json.RawMessage) []int {
	var out []int
	if len(raw) == 0 || json.Unmarshal(raw, &out) != nil {
		return nil
	}
	return out
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
			Key: ss.Key, Owner: ss.Owner, X: ss.X, Y: ss.Y, Cells: ss.Cells, CellW: ss.CellW, CellH: ss.CellH,
			Font: font, GlyphX: ss.GlyphX, GlyphY: ss.GlyphY, GlyphScale: ss.GlyphScale,
			Text: []rune(ss.Text), Transparent: ss.Transp, SwapColors: ss.Swap, State: ss.State, FG: ss.FG, BG: ss.BG,
			hashes: decodeHashes(ss.Hashes), misses: decodeMisses(ss.Misses), anchors: ss.Anchors,
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
