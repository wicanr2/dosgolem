package strikecommander

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// 熱點標籤的 stamp 規劃（001 §2.6），計分板 board 標籤的規劃（003 §3.4）。
// 幾何都是原版像素，整數運算，除法向下取整。
const (
	stampCellH = 14 // label 墨跡框高 [Y+1, Y+15)
	boardCellH = 8  // board 格高：前後列 [Y+1, Y+9)
	glyphPx    = 16
	// 原版色盤的 0 號是黑外框、15 號是白字（RE 012 §5）。
	bgIndex = 0
	fgIndex = 15
	// 白板底色是 164 號 (215, 215, 215)，墨跡最深的是 10 號 (60, 60, 60)（003 §2.3）。
	// board 的背景實際取自畫格加墨跡還原，164 號只在背景圖缺時當底色。
	boardBGIndex = 164
	boardFGIndex = 10
)

// Miss 是缺譯文或缺字的紀錄；畫面維持英文。
type Miss struct {
	Key    string
	Reason string // MissTranslation、MissTemplate（002 §2.2）或 MissGlyph
	Runes  []rune // 缺的字（只對 glyph）
}

// Skip 是其他不產生 stamp 的原因（倍率、字型、超寬、前後列作廢）。
type Skip struct {
	Key    string
	Reason string
}

// Plan 是一筆標籤的兩個 stamp：遮蓋 stamp 在前、文字 stamp 在後。
// Kind 與色號隨 stamp 帶出（xlate.Stamp 沒有種類欄位），由 Overlay 記住，Draw 時依色盤設色。
type Plan struct {
	Cover, Text *xlate.Stamp
	Kind        Kind
	BG, FG      uint8 // 背景、前景色號
}

// PlanLabel 依種類規劃一筆標籤的 stamp：label 照 001 §2.6，board 照 003 §3.4。
// scale 只接受 2、3；text 是譯文。字模檢查、字型檢查由 Overlay 做；這裡只算幾何。
func PlanLabel(l Label, text []rune, scale int, font *xlate.Font) (Plan, error) {
	if scale != 2 && scale != 3 {
		return Plan{}, fmt.Errorf("%d 倍輸出不產生 stamp（框高放不下 16 像素字模）", scale)
	}
	n := len(text)
	if n == 0 {
		return Plan{}, fmt.Errorf("譯文是空的")
	}
	cellW := (glyphPx + scale - 1) / scale // ceil(16 / scale)：2 倍 8、3 倍 6
	w := n * cellW
	if w > 320 {
		return Plan{}, fmt.Errorf("譯文 %d 字寬 %d 超過 320", n, w)
	}
	y := l.Y + 1
	if l.Kind == KindBoard {
		// 右對齊到執行期的 x1（擊落數也右對齊到同一處，003 §2.1）；左伸超出畫面時夾到 0。
		x := max(0, l.X1-w)
		txt := &xlate.Stamp{
			Key: l.Key + ".text", X: x, Y: y, Cells: n, CellW: cellW, CellH: boardCellH,
			Font: font, GlyphX: (cellW*scale - glyphPx) / 2, GlyphY: (boardCellH*scale - glyphPx) / 2, GlyphScale: 1,
			Text: append([]rune(nil), text...), State: xlate.Shown,
		}
		cx0, cx1 := min(l.X0, x), max(l.X1, x+w)
		cx0, cx1 = max(cx0, 0), min(cx1, 320)
		cover := &xlate.Stamp{
			Key: l.Key + ".cover", X: cx0, Y: y, Cells: 1, CellW: cx1 - cx0, CellH: boardCellH, State: xlate.Shown,
		}
		return Plan{Cover: cover, Text: txt, Kind: KindBoard, BG: boardBGIndex, FG: boardFGIndex}, nil
	}
	x := (l.X0+l.X1)/2 - w/2
	if x < 0 {
		x = 0
	}
	if x+w > 320 {
		x = 320 - w
	}
	txt := &xlate.Stamp{
		Key: l.Key + ".text", X: x, Y: y, Cells: n, CellW: cellW, CellH: stampCellH,
		Font: font, GlyphX: (cellW*scale - glyphPx) / 2, GlyphY: (stampCellH*scale - glyphPx) / 2, GlyphScale: 1,
		Text: append([]rune(nil), text...), State: xlate.Shown,
	}
	cx0, cx1 := min(l.X0, x), max(l.X1, x+w)
	cx0, cx1 = max(cx0, 0), min(cx1, 320)
	cover := &xlate.Stamp{
		Key: l.Key + ".cover", X: cx0, Y: y, Cells: 1, CellW: cx1 - cx0, CellH: stampCellH, State: xlate.Shown,
	}
	return Plan{Cover: cover, Text: txt, Kind: KindLabel, BG: bgIndex, FG: fgIndex}, nil
}

// stampInfo 是 Overlay 為每個 stamp 記的種類與色號。
type stampInfo struct {
	kind   Kind
	bg, fg uint8
}

// boardBG 是一筆 board 標籤繪製背景所需的資料（003 §3.4）：遮蓋範圍與前後列。
type boardBG struct {
	label         int // 在最近一次 Sync 的 labels 中的索引
	key           string
	x0, x1, y0    int // 遮蓋範圍 [x0, x1) × [y0, y0+8)，y0 = Y+1 也是前後列的第一列
	before, after []byte
}

// labelMatch 是一筆產生了 stamp 的標籤的比對資料（005 §3.2 第 1 條）。比對框是
// [cover.X, cover.X + cover.Cells × cover.CellW) × [Y+1, Y+1+h) 與畫面的交集。
type labelMatch struct {
	label          int // 在最近一次 Sync 的 labels 中的索引
	key            string
	kind           Kind
	h              int
	y              int // 標籤的 Y；參考列的第一列是 Y+1
	x0, y0, x1, y1 int // 比對框 [x0, x1) × [y0, y1)
	ref            []byte
	cover, text    *xlate.Stamp
}

// Overlay 自有一個 xlate.Layer，依可見集合與 generation 重建 stamp（同 apps/buckrogers/hmenu_overlay.go）。
// 不呼叫 Layer.Frame、Add、Replace：Add 遇到部分重疊會把遮蓋 stamp 的格子設成透明而丟掉它。
type Overlay struct {
	layer *xlate.Layer
	font  *LabelFont
	tr    map[string]string
	scale int
	gen   uint64
	built bool
	// exactOnly 讓解析只留第 1 步（只供 002 §3.2 案例 D 的負對照）。
	exactOnly bool
	// info 記每個 stamp 的種類與色號；boards 記 board 標籤的背景資料。重建 stamp 時清空。
	info   map[*xlate.Stamp]stampInfo
	boards []boardBG
	// matches 記每筆產生了 stamp 的標籤的比對資料（005 §3.2 第 1 條），依 labels 順序。重建 stamp 時清空。
	matches []labelMatch
	// Hidden 是最近一次 Draw 因比對不符而不畫的標籤 Key（005 §3.2 第 3 條），每次 Draw 開頭清空。
	Hidden []string

	// 存讀檔對話框的圖面（004 §3.3）：獨立的 layer，不與文字 stamp 共用。
	artLayer *xlate.Layer
	gfx      map[string]string // 圖形字 catalog（text/menu_gfx），不併進 tr（004 §3.6）
	arts     []*artEntry
	artPal   [256][3]uint8
	artPalOK bool

	Misses []Miss
	Skips  []Skip
}

// NewOverlay 建一個 overlay。scale 接受 1、2、3；1 倍時永遠不產生 stamp（§2.6）。
// tr 是熱點標籤 catalog（text/opttest_labels），gfx 是圖形字 catalog（text/menu_gfx，004 §3.6；可為 nil）。
func NewOverlay(font *LabelFont, tr, gfx map[string]string, scale int) (*Overlay, error) {
	if font == nil || font.Font == nil || scale < 1 || scale > 3 {
		return nil, fmt.Errorf("strikecommander: overlay 輸入無效")
	}
	return &Overlay{layer: &xlate.Layer{W: 320, H: 200}, artLayer: &xlate.Layer{W: 320, H: 200}, font: font, tr: tr, gfx: gfx, scale: scale}, nil
}

// Scale 回輸出倍率。
func (o *Overlay) Scale() int { return o.scale }

// SetScale 換倍率並強制下次 Sync 重建（stamp 幾何依倍率而定）。
func (o *Overlay) SetScale(scale int) error {
	if scale < 1 || scale > 3 {
		return fmt.Errorf("strikecommander: 倍率 %d 不支援", scale)
	}
	o.scale, o.built = scale, false
	o.replanArt()
	return nil
}

// Stamps 回 layer 上的 stamp（呼叫端不得修改）。
func (o *Overlay) Stamps() []*xlate.Stamp { return o.layer.Stamps }

// StampKind 回 stamp 的種類；不是這個 overlay 的 stamp 時回空字串。
func (o *Overlay) StampKind(s *xlate.Stamp) Kind { return o.info[s].kind }

// labelFont 回種類要求的字型：board 是 FB12，label 是 D7E6（003 §3.2）。只有這兩組量過墨跡框。
func labelFont(k Kind) uint16 {
	if k == KindBoard {
		return boardFont
	}
	return measuredFont
}

// Sync 在 generation 改變（或倍率改變）時依可見集合重建 stamp。generation 沒變時不重建，
// 但仍以這次的 labels 更新各 board 標籤的前後列（003 §3.3：sameLabels 不比前後列），
// 以及各標籤的參考列 Ref（005 §3.2 第 1 條：同索引、同 Key；新 Ref 為 nil 時沿用舊值）。
func (o *Overlay) Sync(labels []Label, gen uint64) {
	if o.built && gen == o.gen {
		for i := range o.boards {
			b := &o.boards[i]
			if b.label < len(labels) {
				if l := labels[b.label]; l.Kind == KindBoard && l.Key == b.key && l.Before != nil && l.After != nil {
					b.before, b.after = l.Before, l.After
				}
			}
		}
		for i := range o.matches {
			m := &o.matches[i]
			if m.label < len(labels) {
				if l := labels[m.label]; l.Key == m.key && l.Ref != nil {
					m.ref = l.Ref
				}
			}
		}
		return
	}
	o.built, o.gen = true, gen
	o.layer.Stamps = nil
	o.info = map[*xlate.Stamp]stampInfo{}
	o.boards = nil
	o.matches = nil
	for i, l := range labels {
		if o.scale != 2 && o.scale != 3 {
			o.Skips = append(o.Skips, Skip{l.Key, fmt.Sprintf("%d 倍輸出不覆繪", o.scale)})
			continue
		}
		if want := labelFont(l.Kind); l.Font != want {
			o.Skips = append(o.Skips, Skip{l.Key, fmt.Sprintf("%s 標籤的字型 %04X 不是 %04X，墨跡框沒量過", l.Kind, l.Font, want)})
			continue
		}
		if l.Kind == KindBoard && (l.Before == nil || l.After == nil) {
			o.Skips = append(o.Skips, Skip{l.Key, "board 標籤的前後列作廢"})
			continue
		}
		// 組合標籤由解析代入譯文（002 §2.2）；Label.Key 仍是觀察到的原文。
		text, _, reason := resolveLabel(l.Key, o.tr, o.exactOnly)
		if text == nil {
			o.Misses = append(o.Misses, Miss{Key: l.Key, Reason: reason})
			continue
		}
		var miss []rune
		for _, r := range text {
			if !o.font.Wide.Wide(r) || o.font.Font.Glyphs[r] == nil {
				miss = append(miss, r)
			}
		}
		if len(miss) != 0 {
			o.Misses = append(o.Misses, Miss{Key: l.Key, Reason: MissGlyph, Runes: miss})
			continue
		}
		p, err := PlanLabel(l, text, o.scale, o.font.Font)
		if err != nil {
			o.Skips = append(o.Skips, Skip{l.Key, err.Error()})
			continue
		}
		o.layer.Stamps = append(o.layer.Stamps, p.Cover, p.Text)
		si := stampInfo{kind: p.Kind, bg: p.BG, fg: p.FG}
		o.info[p.Cover], o.info[p.Text] = si, si
		h := refRows(l.Kind)
		o.matches = append(o.matches, labelMatch{
			label: i, key: l.Key, kind: l.Kind, h: h, y: l.Y,
			x0: max(p.Cover.X, 0), x1: min(p.Cover.X+p.Cover.Cells*p.Cover.CellW, 320),
			y0: max(l.Y+1, 0), y1: min(l.Y+1+h, 200),
			ref: l.Ref, cover: p.Cover, text: p.Text,
		})
		if p.Kind == KindBoard {
			o.boards = append(o.boards, boardBG{
				label: i, key: l.Key, x0: p.Cover.X, x1: p.Cover.X + p.Cover.CellW, y0: p.Cover.Y,
				before: l.Before, after: l.After,
			})
		}
	}
}

// Draw 把原版畫格放大成 RGBA 並畫上 stamp。每次繪製前依各 stamp 記下的色號從目前色盤設 BG、FG：
// label 是 0、15 號，board 是 164、10 號。board 的兩個 stamp 以 boardBackground 當底（003 §3.4），
// label 照 001 填實心底色。回傳合成圖與 Layer 回報的缺字。
//
// 順序（004 §3.3）：1. 原版畫格放大；2. 存讀檔對話框的圖面（獨立 layer，逐格比對原版基準）；
// 3. 001 到 003 的文字 stamp。
//
// 第 3 步之前以 Match 逐筆比對（005 §3.2 第 3 條）：不符的標籤兩個 stamp 都不畫，Key 記入 Hidden；
// 照畫的 stamp 組成暫時的 layer 交給 DrawWithBackground。不改 Stamp.State。
func (o *Overlay) Draw(indexed []byte, palette [256][3]uint8) ([]byte, []rune) {
	out := ScaleIndexedRGBA(indexed, palette, o.scale)
	o.drawArt(out, indexed, palette)
	for _, s := range o.layer.Stamps {
		si := o.info[s]
		s.BG, s.FG = palette[si.bg], palette[si.fg]
	}
	bad := o.mismatched(indexed)
	o.Hidden = nil
	hidden := map[int]bool{}
	shown := &xlate.Layer{W: 320, H: 200}
	for i, m := range o.matches {
		if bad[i] {
			o.Hidden = append(o.Hidden, m.key)
			hidden[m.label] = true
			continue
		}
		shown.Stamps = append(shown.Stamps, m.cover, m.text)
	}
	var boards []boardBG
	for _, b := range o.boards {
		if !hidden[b.label] {
			boards = append(boards, b)
		}
	}
	var bg []byte
	if len(boards) != 0 {
		bg = o.boardBackground(indexed, palette, boards)
	}
	var miss []rune
	shown.DrawWithBackground(out, o.scale, func(r rune) { miss = append(miss, r) }, bg,
		func(s *xlate.Stamp) bool { return o.info[s].kind == KindBoard })
	return out, miss
}

// Match 回比對不符的標籤 Key（005 §3.2 第 2 條），不出圖。比對框內每個 (x, y)：indexed[y*320+x]
// 對 Ref[(y−Y−1)*320+x]。len(indexed) < 64000、Ref 為 nil、或 len(Ref) != h × 320 視為不符。
func (o *Overlay) Match(indexed []byte) []string {
	var out []string
	for i, b := range o.mismatched(indexed) {
		if b {
			out = append(out, o.matches[i].key)
		}
	}
	return out
}

// mismatched 回與 o.matches 平行的不符旗標。
func (o *Overlay) mismatched(indexed []byte) []bool {
	bad := make([]bool, len(o.matches))
	for i, m := range o.matches {
		if len(indexed) < 64000 || m.ref == nil || len(m.ref) != m.h*320 {
			bad[i] = true
			continue
		}
	scan:
		for y := m.y0; y < m.y1; y++ {
			row := (y - m.y - 1) * 320
			for x := m.x0; x < m.x1; x++ {
				if indexed[y*320+x] != m.ref[row+x] {
					bad[i] = true
					break scan
				}
			}
		}
	}
	return bad
}

// MatchBox 回 Key 的比對框 [x0, x1) × [y0, y1)；沒有這筆時 ok 為 false（量測與收據用）。
func (o *Overlay) MatchBox(key string) (x0, y0, x1, y1 int, ok bool) {
	for _, m := range o.matches {
		if m.key == key {
			return m.x0, m.y0, m.x1, m.y1, true
		}
	}
	return 0, 0, 0, 0, false
}

// boardBackground 依 003 §3.4 建 board stamp 的背景（放大後的 RGBA）：
//  1. 從原版畫格開始；
//  2. 每筆 board 標籤遮蓋範圍內的每個像素，若是墨跡像素（前列 ≠ 後列）而且目前畫格在該點仍等於後列，
//     換成前列的顏色；其他像素保留目前畫格（滑鼠游標、白板細線都留著）；
//  3. 依色盤換成 RGBA、放大。
//
// boards 是這次照畫的 board 標籤（005 §3.2 第 3 條：比對不符的不列入）。
func (o *Overlay) boardBackground(indexed []byte, palette [256][3]uint8, boards []boardBG) []byte {
	idx := append([]byte(nil), indexed...)
	for _, b := range boards {
		for y := b.y0; y < b.y0+boardRows; y++ {
			if y < 0 || y >= 200 {
				continue
			}
			row := (y - b.y0) * 320
			for x := max(b.x0, 0); x < min(b.x1, 320); x++ {
				bf, af := b.before[row+x], b.after[row+x]
				if bf != af && idx[y*320+x] == af {
					idx[y*320+x] = bf
				}
			}
		}
	}
	return ScaleIndexedRGBA(idx, palette, o.scale)
}

// ScaleIndexedRGBA 把 320×200 的色號畫格依色盤放大成 RGBA。
func ScaleIndexedRGBA(indexed []byte, palette [256][3]uint8, scale int) []byte {
	w := 320 * scale
	rgba := make([]byte, w*200*scale*4)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			c := palette[indexed[y*320+x]]
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					i := ((y*scale+dy)*w + x*scale + dx) * 4
					rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = c[0], c[1], c[2], 255
				}
			}
		}
	}
	return rgba
}
