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

// stampInfo 是 Overlay 為每個 stamp 記的種類與色號。bgImage 為真的 stamp 以背景圖填格（006 §3.2 第 1 條）：
// 透明標籤（前後列皆非 nil 的 label）與 board；退路標籤（前後列任一為 nil 的 label）為假，照 001 填實心底色。
type stampInfo struct {
	kind    Kind
	bg, fg  uint8
	bgImage bool
}

// bgItem 是一筆以背景圖繪製的標籤（006 §3.2 第 3 條；003 §3.4 的 boardBG 改成通用）：遮蓋範圍與前後列。
type bgItem struct {
	label         int // 在最近一次 Sync 的 labels 中的索引
	key           string
	kind          Kind
	x0, x1, y0    int // 遮蓋範圍 [x0, x1) × [y0, y0+rows)，y0 = Y+1 也是前後列的第一列
	rows          int // label 14、board 8
	before, after []byte
}

// labelMatch 是一筆產生了 stamp 的標籤的比對資料（005 §3.2 第 1 條）。比對框是
// [cover.X, cover.X + cover.Cells × cover.CellW) × [Y+1, Y+1+h) 與畫面的交集。
// before、after 是繪製背景用的前後列（006 §3.2 第 2 條：同 generation 時成對更新），bgImage 只在重建時判定。
type labelMatch struct {
	label          int // 在最近一次 Sync 的 labels 中的索引
	key            string
	kind           Kind
	h              int
	y              int // 標籤的 Y；參考列的第一列是 Y+1
	x0, y0, x1, y1 int // 比對框 [x0, x1) × [y0, y1)
	ref            []byte
	cover, text    *xlate.Stamp
	before, after  []byte
	bgImage        bool
	// textItem 是新路徑（011 §3.9）的文字項；新路徑沒有文字 stamp（text 為 nil）。
	textItem *textItem
}

// BGEntry 是 BGReport 的一筆（006 §3.3 第 1 條）：Overlay 內部實際使用的資料。座標都是原版像素。
type BGEntry struct {
	Key   string
	Kind  Kind
	State string // "透明" 或 "退路"（board 一律以背景圖繪製，記 "透明"）
	Text  string // 文字 stamp 的 Text
	// 遮蓋範圍 [CoverX, CoverX+CoverW) × [CoverY, CoverY+CoverH)，CoverY = 標籤 Y + 1。
	CoverX, CoverY, CoverW, CoverH int
	// 文字 stamp 的幾何。
	TextX, TextY, Cells, CellW, CellH, GlyphX, GlyphY int
	Before, After, Ref                                []byte
	// Hidden 是最近一次 Draw 是否因比對不符而不畫這筆（重建後尚未 Draw 時為 false）。
	Hidden bool

	// 以下是 011 §3.8 新增的欄位；Path 為 "new" 時上面的文字 stamp 幾何欄不適用（labels.tsv 填 -），
	// 為 "old" 時這些欄不適用（填 -）。座標是輸出像素；Ink 是最近一次 Draw 的文字框（沒畫時為空）。
	Path                string
	Display             string
	Px, TxOut, Baseline int
	Clip, Ink           xlate.AlphaRect
	Outline             bool
	E0, E1              int
}

// 透明、退路的狀態字（006 §3.3 第 1 條）。
const (
	BGStateTransparent = "透明"
	BGStateFallback    = "退路"
)

// validRows 回前後列可用：非 nil 且長度是 h × 320。
func validRows(b []byte, h int) bool { return b != nil && len(b) == h*320 }

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
	// info 記每個 stamp 的種類、色號與是否以背景圖填格。重建 stamp 時清空。
	info map[*xlate.Stamp]stampInfo
	// matches 記每筆產生了 stamp 的標籤的比對資料（005 §3.2 第 1 條）與前後列（006 §3.2），依 labels 順序。
	// 重建 stamp 時清空。
	matches []labelMatch
	// lastBad 是最近一次 Draw 的比對不符旗標，與 matches 平行（BGReport 的 Hidden）。重建時清空。
	lastBad []bool
	// Hidden 是最近一次 Draw 因比對不符而不畫的標籤 Key（005 §3.2 第 3 條），每次 Draw 開頭清空。
	Hidden []string
	// BGFallbacks 是目前 stamp 集合中的退路標籤（006 §3.2 第 7 條）：前後列任一為 nil、照 001 畫實心底的 label。
	// 每次重建清空再填，不累加；不寫進 Skips。
	BGFallbacks []Skip

	// 存讀檔對話框、主選單與 MENU 片頭選單的圖面（004 §3.3、008 §3.6）：獨立的 layer，不與文字 stamp 共用。
	artLayer *xlate.Layer
	gfx      map[string]string // 圖形字 catalog（text/menu_gfx），不併進 tr（004 §3.6）
	arts     []*artEntry
	artPal   [256][3]uint8
	artPalOK bool
	// platePending 是主選單底板事件（008 §3.4）：等下一個 MAINMENU 按鈕事件帶來按鈕檔，才建五行灰字的圖面。
	platePending *ArtEvent

	// 011 §3.9 的路徑開關：Text 是圖集；TextLabel 為真時 label 種類走反鋸齒新路徑。TextBoard、TextArt 在 (e) 實作，
	// (d) 設為真時 Sync 照舊路徑處理並不使用。開關在建立後、第一次 Sync 前設定。
	Text      *TextFonts
	TextLabel bool
	TextBoard bool
	TextArt   bool
	// lastInk 是最近一次 Draw 各筆（與 matches 平行）新路徑串的文字框；沒畫的為空。重建時清空。
	lastInk []xlate.AlphaRect

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
// 但仍以這次的 labels（同索引、同 Key）更新各標籤的前後列（003 §3.3、006 §3.2 第 2 條：新前列與新後列
// 都可用才成對更新，否則沿用舊的一對；透明或退路的判定只在重建時做），以及參考列 Ref（005 §3.2 第 1 條：
// 新 Ref 為 nil 時沿用舊值）。
func (o *Overlay) Sync(labels []Label, gen uint64) {
	if o.built && gen == o.gen {
		for i := range o.matches {
			m := &o.matches[i]
			if m.label >= len(labels) {
				continue
			}
			l := labels[m.label]
			if l.Key != m.key {
				continue
			}
			if l.Ref != nil {
				m.ref = l.Ref
			}
			if validRows(l.Before, m.h) && validRows(l.After, m.h) {
				m.before, m.after = l.Before, l.After
			}
		}
		return
	}
	o.built, o.gen = true, gen
	o.layer.Stamps = nil
	o.info = map[*xlate.Stamp]stampInfo{}
	o.matches = nil
	o.lastBad = nil
	o.lastInk = nil
	o.BGFallbacks = nil
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
		if o.Text != nil && ((o.TextLabel && l.Kind != KindBoard) || (o.TextBoard && l.Kind == KindBoard)) {
			o.syncLabelText(i, l, text)
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
		h := refRows(l.Kind)
		// 透明標籤與 board 以背景圖填格；退路標籤照 001 畫實心底（006 §3.2 第 1、7 條）。
		// board 的前後列在上面已確認非 nil；長度另驗，不符的列當作不可用（勘誤候選：規格只寫 nil）。
		rowsOK := validRows(l.Before, h) && validRows(l.After, h)
		bgImage := p.Kind == KindBoard || rowsOK
		if p.Kind == KindBoard && !rowsOK {
			o.Skips = append(o.Skips, Skip{l.Key, fmt.Sprintf("board 標籤的前後列長度 %d、%d，要 %d", len(l.Before), len(l.After), h*320)})
			continue
		}
		if !bgImage {
			o.BGFallbacks = append(o.BGFallbacks, Skip{l.Key, rowsReason(l.Before, l.After, h)})
		}
		o.layer.Stamps = append(o.layer.Stamps, p.Cover, p.Text)
		si := stampInfo{kind: p.Kind, bg: p.BG, fg: p.FG, bgImage: bgImage}
		o.info[p.Cover], o.info[p.Text] = si, si
		m := labelMatch{
			label: i, key: l.Key, kind: l.Kind, h: h, y: l.Y,
			x0: max(p.Cover.X, 0), x1: min(p.Cover.X+p.Cover.Cells*p.Cover.CellW, 320),
			y0: max(l.Y+1, 0), y1: min(l.Y+1+h, 200),
			ref: l.Ref, cover: p.Cover, text: p.Text, bgImage: bgImage,
		}
		if rowsOK {
			m.before, m.after = l.Before, l.After
		}
		o.matches = append(o.matches, m)
	}
}

// syncLabelText 是 Sync 重建時 label 種類的新路徑（011 §3.4、§3.6.1）：N() 之後任一字不在圖集記 MissGlyph
// （Runes 依序列出全部缺字），排版失敗記 Skip；成功時只加遮蓋 stamp，文字項記在 labelMatch.textItem。
// 前後列、透明或退路的判定與舊路徑相同。
func (o *Overlay) syncLabelText(i int, l Label, text []rune) {
	px := pxLabel[o.scale]
	if l.Kind == KindBoard {
		px = pxBoard[o.scale] // 011 §3.6.2
	}
	f := o.Text.Font(px)
	if f == nil {
		o.Skips = append(o.Skips, Skip{l.Key, fmt.Sprintf("圖集缺字級 %d", px)})
		return
	}
	if _, miss := f.Advance(NarrowAlnum(text)); len(miss) != 0 {
		o.Misses = append(o.Misses, Miss{Key: l.Key, Reason: MissGlyph, Runes: miss})
		return
	}
	p, err := PlanLabelText(l, text, o.scale, o.Text)
	if err != nil {
		o.Skips = append(o.Skips, Skip{l.Key, err.Error()})
		return
	}
	h := refRows(l.Kind)
	rowsOK := validRows(l.Before, h) && validRows(l.After, h)
	bgImage := p.Kind == KindBoard || rowsOK
	if p.Kind == KindBoard && !rowsOK {
		o.Skips = append(o.Skips, Skip{l.Key, fmt.Sprintf("board 標籤的前後列長度 %d、%d，要 %d", len(l.Before), len(l.After), h*320)})
		return
	}
	if !bgImage {
		o.BGFallbacks = append(o.BGFallbacks, Skip{l.Key, rowsReason(l.Before, l.After, h)})
	}
	o.layer.Stamps = append(o.layer.Stamps, p.Cover)
	o.info[p.Cover] = stampInfo{kind: p.Kind, bg: p.BG, fg: p.FG, bgImage: bgImage}
	ti := p.Text
	m := labelMatch{
		label: i, key: l.Key, kind: l.Kind, h: h, y: l.Y,
		x0: max(p.Cover.X, 0), x1: min(p.Cover.X+p.Cover.Cells*p.Cover.CellW, 320),
		y0: max(l.Y+1, 0), y1: min(l.Y+1+h, 200),
		ref: l.Ref, cover: p.Cover, bgImage: bgImage, textItem: &ti,
	}
	if rowsOK {
		m.before, m.after = l.Before, l.After
	}
	o.matches = append(o.matches, m)
}

// rowsReason 回退路的原因（BGFallbacks）。
func rowsReason(before, after []byte, h int) string {
	switch {
	case before == nil && after == nil:
		return "前列與後列都是 nil"
	case before == nil:
		return "前列是 nil"
	case after == nil:
		return "後列是 nil"
	}
	return fmt.Sprintf("前後列長度 %d、%d，要 %d", len(before), len(after), h*320)
}

// Draw 把原版畫格放大成 RGBA 並畫上 stamp。每次繪製前依各 stamp 記下的色號從目前色盤設 BG、FG：
// label 是 0、15 號，board 是 164、10 號。透明標籤與 board 的 stamp 以 labelBackground 的背景圖當底
// （003 §3.4、006 §3.2），退路標籤照 001 填實心底色。回傳合成圖與 Layer 回報的缺字。
//
// 順序（004 §3.3）：1. 原版畫格放大；2. 存讀檔對話框的圖面（獨立 layer，逐格比對原版基準）；
// 3. 001 到 003 的文字 stamp。背景圖以第 2 步之後的圖為底（006 §3.2 第 4 條）。
//
// 第 3 步之前以 Match 逐筆比對（005 §3.2 第 3 條）：不符的標籤兩個 stamp 都不畫，Key 記入 Hidden，
// 也不列入背景清單（006 §3.2 第 8 條）；照畫的 stamp 組成暫時的 layer 交給 DrawWithBackground。不改 Stamp.State。
func (o *Overlay) Draw(indexed []byte, palette [256][3]uint8) ([]byte, []rune) {
	out := ScaleIndexedRGBA(indexed, palette, o.scale)
	o.drawArt(out, indexed, palette)
	for _, s := range o.layer.Stamps {
		si := o.info[s]
		s.BG, s.FG = palette[si.bg], palette[si.fg]
	}
	bad := o.mismatched(indexed)
	o.lastBad = bad
	o.Hidden = nil
	shown := &xlate.Layer{W: 320, H: 200}
	items := make([]*textItem, len(o.matches))
	for i, m := range o.matches {
		if bad[i] {
			o.Hidden = append(o.Hidden, m.key)
			continue
		}
		if m.textItem != nil {
			// 新路徑（011 §3.6.1）：遮蓋 stamp 照舊以 DrawWithBackground 填格，字在之後兩趟畫。
			shown.Stamps = append(shown.Stamps, m.cover)
			items[i] = m.textItem
			continue
		}
		shown.Stamps = append(shown.Stamps, m.cover, m.text)
	}
	var bg []byte
	if bitems, texts := o.backgroundItems(bad); len(bitems) != 0 {
		bg = labelBackground(out, indexed, palette, o.scale, bitems, texts)
	}
	var miss []rune
	shown.DrawWithBackground(out, o.scale, func(r rune) { miss = append(miss, r) }, bg,
		func(s *xlate.Stamp) bool { return o.info[s].bgImage })
	// 全部照畫標籤的外框層，再全部照畫字串的填色層（011 §3.4 第 4 條、§3.6.1）。
	o.lastInk = drawTextItems(out, o.scale, palette, items)
	return out, miss
}

// backgroundItems 回這次照畫、以背景圖繪製的標籤（006 §3.2 第 3、8 條）：依重建順序，透明標籤與 board，
// bad（與 matches 平行）為真的不列入。texts 與 items 平行：透明標籤是文字 stamp（畫外框），board 是 nil。
func (o *Overlay) backgroundItems(bad []bool) ([]bgItem, []*xlate.Stamp) {
	var items []bgItem
	var texts []*xlate.Stamp
	for i, m := range o.matches {
		if !m.bgImage || (i < len(bad) && bad[i]) {
			continue
		}
		items = append(items, bgItem{
			label: m.label, key: m.key, kind: m.kind,
			x0: m.cover.X, x1: m.cover.X + m.cover.CellW, y0: m.cover.Y, rows: m.h,
			before: m.before, after: m.after,
		})
		if m.kind == KindBoard || m.textItem != nil {
			// board 不畫外框；新路徑的外框在 drawTextItems 畫，不畫到背景圖（011 §3.9）。
			texts = append(texts, nil)
		} else {
			texts = append(texts, m.text)
		}
	}
	return items, texts
}

// BGReport 依重建順序列出每筆產生了 stamp 的標籤（006 §3.3 第 1 條），取 Overlay 內部實際使用的資料
// （同 generation 更新後的前後列與 Ref）。Hidden 取最近一次 Draw。切片是複本。
func (o *Overlay) BGReport() []BGEntry {
	var out []BGEntry
	for i, m := range o.matches {
		e := BGEntry{
			Key: m.key, Kind: m.kind, State: BGStateFallback,
			CoverX: m.cover.X, CoverY: m.cover.Y, CoverW: m.cover.Cells * m.cover.CellW, CoverH: m.cover.CellH,
			Before: append([]byte(nil), m.before...), After: append([]byte(nil), m.after...), Ref: append([]byte(nil), m.ref...),
			Path: "old",
		}
		if ti := m.textItem; ti != nil {
			// 新路徑（011 §3.8）：文字 stamp 幾何不適用，填新欄。
			e.Path, e.Text, e.Display = "new", string(ti.raw), string(ti.display)
			e.Px, e.TxOut, e.Baseline, e.Clip, e.Outline, e.E0, e.E1 = ti.px, ti.tx, ti.baseline, ti.clip, ti.outline, ti.e0, ti.e1
			if i < len(o.lastInk) {
				e.Ink = o.lastInk[i]
			}
		} else {
			e.Text = string(m.text.Text)
			e.TextX, e.TextY, e.Cells, e.CellW, e.CellH = m.text.X, m.text.Y, m.text.Cells, m.text.CellW, m.text.CellH
			e.GlyphX, e.GlyphY = m.text.GlyphX, m.text.GlyphY
		}
		if m.bgImage {
			e.State = BGStateTransparent
		}
		if m.before == nil {
			e.Before = nil
		}
		if m.after == nil {
			e.After = nil
		}
		if m.ref == nil {
			e.Ref = nil
		}
		if i < len(o.lastBad) {
			e.Hidden = o.lastBad[i]
		}
		out = append(out, e)
	}
	return out
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

// labelBackground 依 006 §3.2 第 4 條建透明標籤與 board 的背景圖（放大後的 RGBA，003 §3.4 的推廣）：
//  1. 複製 out（drawArt 之後的原版放大圖，含存讀檔圖面）；
//  2. 每項遮蓋範圍內每個原版像素 (x, y)：若是墨跡像素（前列 ≠ 後列）而且畫格在該點仍等於後列，
//     對應的 倍率 × 倍率 輸出像素中，目前仍等於畫格顏色（沒被圖面改過）的換成前列的顏色；其他保留
//     （滑鼠游標、白板細線、圖面都留著）；
//  3. 每筆透明標籤的文字 stamp 畫外框（glyphMask 的 8 鄰域膨脹減字形，裁到遮蓋範圍，0 號色）。
//
// items 是這次照畫的項（005 §3.2 第 3 條：比對不符的不列入），texts 與 items 平行，board 項為 nil。
// 各項的遮蓋範圍假設不重疊；重疊時依清單順序套用，後者看到前者的改動（畫格的複本隨之更新）。
func labelBackground(out []byte, indexed []byte, palette [256][3]uint8, scale int, items []bgItem, texts []*xlate.Stamp) []byte {
	bg := append([]byte(nil), out...)
	idx := append([]byte(nil), indexed...)
	W := 320 * scale
	for _, it := range items {
		for y := it.y0; y < it.y0+it.rows; y++ {
			if y < 0 || y >= 200 {
				continue
			}
			row := (y - it.y0) * 320
			for x := max(it.x0, 0); x < min(it.x1, 320); x++ {
				bf, af := it.before[row+x], it.after[row+x]
				p := y*320 + x
				if bf == af || idx[p] != af {
					continue
				}
				// 「沒被圖面改過」：輸出像素仍等於畫格在該點的顏色。
				cur, nc := palette[idx[p]], palette[bf]
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						i := 4 * ((y*scale+dy)*W + x*scale + dx)
						if bg[i] == cur[0] && bg[i+1] == cur[1] && bg[i+2] == cur[2] {
							bg[i], bg[i+1], bg[i+2] = nc[0], nc[1], nc[2]
						}
					}
				}
				idx[p] = bf
			}
		}
	}
	for k, it := range items {
		if k < len(texts) && texts[k] != nil {
			drawOutline(bg, scale, palette[bgIndex], it, texts[k])
		}
	}
	return bg
}

// drawOutline 是 006 §3.2 第 5 條的外框：字形遮罩 G 以 8 鄰域膨脹一個輸出像素得 D，D − G 中落在遮蓋範圍
// 輸出像素 [x0 × 倍率, x1 × 倍率) × [y0 × 倍率, (y0 + rows) × 倍率) 內者設成 c。
func drawOutline(bg []byte, scale int, c [3]uint8, it bgItem, s *xlate.Stamp) {
	W, H := 320*scale, 200*scale
	g := glyphMask(s, scale)
	for y := max(it.y0*scale, 0); y < min((it.y0+it.rows)*scale, H); y++ {
		for x := max(it.x0*scale, 0); x < min(it.x1*scale, W); x++ {
			if g[y*W+x] {
				continue
			}
			hit := false
			for dy := -1; dy <= 1 && !hit; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if (dx != 0 || dy != 0) && nx >= 0 && nx < W && ny >= 0 && ny < H && g[ny*W+nx] {
						hit = true
						break
					}
				}
			}
			if hit {
				i := 4 * (y*W + x)
				bg[i], bg[i+1], bg[i+2], bg[i+3] = c[0], c[1], c[2], 255
			}
		}
	}
}

// glyphMask 回文字 stamp 的字形遮罩（輸出像素座標，寬 320 × 倍率、高 200 × 倍率），照 xlate drawGlyphs 逐點算
// （006 §3.2 第 5 條）：跳過 i ≥ Cells、半形與全形空白、缺字、透明格；字模亮點畫在
// (X + i × CellW) × 倍率 + GlyphX + gx × k、Y × 倍率 + GlyphY + gy × k（k 是 GlyphScale，0 表示 max(1, 倍率 / 3)），
// 只取格內的點。字模列寬 (Font.W + 7) / 8 bytes 在這裡自算，不動 xlate。
func glyphMask(s *xlate.Stamp, scale int) []bool {
	W, H := 320*scale, 200*scale
	m := make([]bool, W*H)
	if s == nil || s.Font == nil {
		return m
	}
	k := s.GlyphScale
	if k == 0 {
		k = max(1, scale/3)
	}
	if k < 0 {
		return m
	}
	rb := (s.Font.W + 7) / 8
	cellW, cellH := s.CellW*scale, s.CellH*scale
	for i, r := range s.Text {
		if i >= s.Cells || r == ' ' || r == '　' || (i < len(s.Transparent) && s.Transparent[i]) {
			continue
		}
		g, ok := s.Font.Glyphs[r]
		if !ok {
			continue
		}
		cx, cy := (s.X+i*s.CellW)*scale, s.Y*scale
		for gy := 0; gy < s.Font.H; gy++ {
			for gx := 0; gx < s.Font.W; gx++ {
				if g[gy*rb+gx/8]&(0x80>>uint(gx%8)) == 0 {
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
						x, y := cx+lx, cy+ly
						if x >= 0 && x < W && y >= 0 && y < H {
							m[y*W+x] = true
						}
					}
				}
			}
		}
	}
	return m
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
