package buckrogers

import (
	"fmt"
	"github.com/wicanr2/dosgolem/xlate"
)

// RuntimeStoryOpeningOverlay is output-only; it never retains machine state.
type RuntimeStoryOpeningOverlay struct {
	layer  *xlate.Layer
	font   *xlate.Font
	text   map[string]string
	scale  int
	active bool
}

func NewRuntimeStoryOpeningOverlay(text map[string]string, font *xlate.Font, scale int) (*RuntimeStoryOpeningOverlay, error) {
	if font == nil || font.W != 16 || font.H != 16 || (scale != 2 && scale != 3) || len(text) != 5 {
		return nil, fmt.Errorf("buckrogers: 首屏 presenter 輸入無效")
	}
	if scale == 3 {
		baseName := font.Name
		font = manualThreeXFont(font)
		// Snapshot／Restore 的 font registry 以 Name 指向字型。3× 是一份
		// 22×22 的衍生輸出字模，不能與 2× 的 16×16 source 共用 registry
		// key，否則 host presenter restore 時會失敗即關閉或誤畫稀疏字。
		if baseName == "" {
			font.Name = "buckrogers-story-3x22"
		} else {
			font.Name = baseName + ".3x22"
		}
	}
	for _, s := range text {
		for _, r := range s {
			if g, ok := font.Glyphs[r]; !ok || len(g) != (font.H*((font.W+7)/8)) {
				return nil, fmt.Errorf("buckrogers: 首屏缺字")
			}
		}
	}
	return &RuntimeStoryOpeningOverlay{layer: &xlate.Layer{W: 320, H: 200}, font: font, text: text, scale: scale}, nil
}
func (o *RuntimeStoryOpeningOverlay) Apply(events []StoryOpeningEvent, p [256][3]uint8) error {
	if o == nil || o.active || len(events) != 5 {
		return fmt.Errorf("buckrogers: 首屏 apply 無效")
	}
	for i, e := range events {
		s := o.text[e.EventKey]
		if s == "" || e.Row != uint8(17+i) || e.Column != 1 {
			return fmt.Errorf("buckrogers: 首屏 event 無效")
		}
		off := manualGlyphOffset(o.scale)
		o.layer.Add(&xlate.Stamp{Key: e.EventKey, X: 8, Y: int(e.Row) * 8, Cells: 39, CellW: 8, CellH: 8, Font: o.font, GlyphX: off, GlyphY: off, GlyphScale: 1, Text: []rune(s), State: xlate.Shown, BG: p[0], FG: p[10]})
	}
	o.active = true
	return nil
}
func (o *RuntimeStoryOpeningOverlay) Clear() {
	if o != nil {
		// 保持 layer 身分穩定：host presenter 只持有這一個輸出端 layer，
		// 並複製其 stamp。若在這裡替換 pointer，已確認的第二頁視訊寫入後
		// presenter 會繪出過期 stamp。READY 的劇情矩形完整包含五個 stamp，
		// 所以 Layer.Clear 可在不觸及 DOS VRAM 下移除它們。
		o.layer.Clear(8, 136, 320, 176)
		o.active = false
	}
}
func (o *RuntimeStoryOpeningOverlay) Frame(indexed []byte, p [256][3]uint8) {
	if o != nil {
		for _, s := range o.layer.Stamps {
			s.BG = p[0]
			s.FG = p[10]
		}
	}
}
func (o *RuntimeStoryOpeningOverlay) Draw(indexed []byte, p [256][3]uint8) ([]byte, []rune, bool) {
	if o == nil {
		return nil, nil, false
	}
	out := ScaleIndexedRGBA(indexed, p, o.scale)
	var miss []rune
	drew := o.layer.Draw(out, o.scale, func(r rune) { miss = append(miss, r) })
	return out, miss, drew
}

// ActiveKeys returns the current, content-safe story event keys.  It is used
// by receipt validation; callers cannot mutate the overlay through it.
func (o *RuntimeStoryOpeningOverlay) ActiveKeys() []string {
	if o == nil {
		return nil
	}
	keys := make([]string, 0, len(o.layer.Stamps))
	for _, stamp := range o.layer.Stamps {
		keys = append(keys, stamp.Key)
	}
	return keys
}

// PresentationLayer 回傳給 host presenter 的 active 輸出端 layer。呼叫端只能
// 在擁有這個 overlay Apply、Clear、Frame lifecycle 的同一 goroutine 使用它；不得
// 寫入 layer 或藉此接觸 machine／DOS state。presentation.LayerSnapshotProvider
// 會在繪製前複製 stamp，維持這道邊界。
//
// 它刻意只限已 READY 的首屏劇情 overlay，不暴露原文、輸入或遊戲 state。
func (o *RuntimeStoryOpeningOverlay) PresentationLayer() *xlate.Layer {
	if o == nil {
		return nil
	}
	return o.layer
}
