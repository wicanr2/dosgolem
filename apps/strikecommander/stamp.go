package strikecommander

import (
	"fmt"

	"github.com/wicanr2/dosgolem/xlate"
)

// 熱點標籤的 stamp 規劃（001 §2.6）。幾何都是原版像素，整數運算，除法向下取整。
const (
	stampCellH = 14 // 墨跡框高 [Y+1, Y+15)
	glyphPx    = 16
	// 原版色盤的 0 號是黑外框、15 號是白字（RE 012 §5）。
	bgIndex = 0
	fgIndex = 15
)

// Miss 是缺譯文或缺字的紀錄；畫面維持英文。
type Miss struct {
	Key    string
	Reason string // MissTranslation、MissTemplate（002 §2.2）或 MissGlyph
	Runes  []rune // 缺的字（只對 glyph）
}

// Skip 是其他不產生 stamp 的原因（倍率、字型、超寬）。
type Skip struct {
	Key    string
	Reason string
}

// Plan 是一筆熱點標籤的兩個 stamp：遮蓋 stamp 在前、文字 stamp 在後。
type Plan struct {
	Cover, Text *xlate.Stamp
}

// PlanLabel 依 §2.6 規劃一筆熱點標籤的 stamp。scale 只接受 2、3；text 是譯文。
// 字模檢查、字型 D7E6 檢查由 Overlay 做；這裡只算幾何。
func PlanLabel(l Label, text []rune, scale int, font *xlate.Font) (Plan, error) {
	if scale != 2 && scale != 3 {
		return Plan{}, fmt.Errorf("%d 倍輸出不產生 stamp（框高 14 放不下 16 像素字模）", scale)
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
	x := (l.X0+l.X1)/2 - w/2
	if x < 0 {
		x = 0
	}
	if x+w > 320 {
		x = 320 - w
	}
	y := l.Y + 1
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
	return Plan{Cover: cover, Text: txt}, nil
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

	Misses []Miss
	Skips  []Skip
}

// NewOverlay 建一個 overlay。scale 接受 1、2、3；1 倍時永遠不產生 stamp（§2.6）。
func NewOverlay(font *LabelFont, tr map[string]string, scale int) (*Overlay, error) {
	if font == nil || font.Font == nil || scale < 1 || scale > 3 {
		return nil, fmt.Errorf("strikecommander: overlay 輸入無效")
	}
	return &Overlay{layer: &xlate.Layer{W: 320, H: 200}, font: font, tr: tr, scale: scale}, nil
}

// Scale 回輸出倍率。
func (o *Overlay) Scale() int { return o.scale }

// SetScale 換倍率並強制下次 Sync 重建（stamp 幾何依倍率而定）。
func (o *Overlay) SetScale(scale int) error {
	if scale < 1 || scale > 3 {
		return fmt.Errorf("strikecommander: 倍率 %d 不支援", scale)
	}
	o.scale, o.built = scale, false
	return nil
}

// Stamps 回 layer 上的 stamp（呼叫端不得修改）。
func (o *Overlay) Stamps() []*xlate.Stamp { return o.layer.Stamps }

// Sync 在 generation 改變（或倍率改變）時依可見集合重建 stamp。
func (o *Overlay) Sync(labels []Label, gen uint64) {
	if o.built && gen == o.gen {
		return
	}
	o.built, o.gen = true, gen
	o.layer.Stamps = nil
	for _, l := range labels {
		if o.scale != 2 && o.scale != 3 {
			o.Skips = append(o.Skips, Skip{l.Key, fmt.Sprintf("%d 倍輸出不覆繪", o.scale)})
			continue
		}
		if l.Font != measuredFont {
			o.Skips = append(o.Skips, Skip{l.Key, fmt.Sprintf("字型 %04X 的墨跡框沒量過", l.Font)})
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
	}
}

// Draw 把原版畫格放大成 RGBA 並畫上 stamp。每次繪製前把 BG、FG 設成目前色盤的 0、15 號。
// 回傳合成圖與 Layer.Draw 回報的缺字。
func (o *Overlay) Draw(indexed []byte, palette [256][3]uint8) ([]byte, []rune) {
	out := ScaleIndexedRGBA(indexed, palette, o.scale)
	for _, s := range o.layer.Stamps {
		s.BG, s.FG = palette[bgIndex], palette[fgIndex]
	}
	var miss []rune
	o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss
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
