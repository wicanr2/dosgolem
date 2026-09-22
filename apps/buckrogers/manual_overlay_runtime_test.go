package buckrogers

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

const manualLayoutFixture = "layout_key\tclear_x\tclear_y\tclear_width\tclear_height\ttext_x\ttext_y\tcolumns\trows\tcapacity\tline_height\tline_spacing\toverflow_policy\tevidence_level\n" +
	"manual.paragraph.body\t7\t72\t305\t112\t16\t72\t36\t14\t504\t8\t0\tsingle-page-reject\tconfirmed\n"

func manualOverlayCatalog(translation string) *Catalog {
	return &Catalog{byIdentity: map[identity]catalogEntry{
		{page: 1, heading: "Fixture", ordinal: 1}: {
			eventKey: "manual.fixture.word1", textKey: "manual.fixture", translation: translation,
		},
	}}
}

func manualOverlayFont(catalog *Catalog) *xlate.Font {
	glyph := bytes.Repeat([]byte{0xff}, 32)
	glyphs := map[rune][]byte{}
	for _, entry := range catalog.byIdentity {
		for _, r := range entry.translation {
			glyphs[r] = append([]byte(nil), glyph...)
		}
	}
	return &xlate.Font{W: 16, H: 16, Glyphs: glyphs}
}

func manualOverlayRequest(catalog *Catalog, generation uint64) DisplayRequest {
	for _, entry := range catalog.byIdentity {
		return DisplayRequest{Generation: generation, EventKey: entry.eventKey, TextKey: entry.textKey, Translation: entry.translation}
	}
	panic("fixture catalog empty")
}

func loadManualOverlayLayout(t *testing.T) *ManualOverlayLayout {
	t.Helper()
	layout, err := LoadManualOverlayLayout("manual-overlay-layout.tsv", []byte(manualLayoutFixture))
	if err != nil {
		t.Fatal(err)
	}
	return layout
}

func manualOverlayPaletteAndFrame() ([256][3]uint8, []byte) {
	var palette [256][3]uint8
	palette[10] = [3]uint8{10, 20, 30}
	palette[15] = [3]uint8{150, 160, 170}
	indexed := bytes.Repeat([]byte{10}, 320*200)
	for y := 72; y < 184; y++ {
		indexed[y*320+7] = 15
		indexed[y*320+311] = 15
		for x := 16; x < 304; x += 8 {
			indexed[y*320+x] = 15
		}
	}
	return palette, indexed
}

func rgbaAt(rgba []byte, width, x, y int) [4]uint8 {
	i := 4 * (y*width + x)
	return [4]uint8{rgba[i], rgba[i+1], rgba[i+2], rgba[i+3]}
}

func assertDiffOnlyInside(t *testing.T, before, after []byte, width, height int, rect PixelRect) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatal("RGBA 長度不同")
	}
	changes := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := 4 * (y*width + x)
			if bytes.Equal(before[i:i+4], after[i:i+4]) {
				continue
			}
			changes++
			if x < rect.X || x >= rect.X+rect.Width || y < rect.Y || y >= rect.Y+rect.Height {
				t.Fatalf("矩形外變動 at %d,%d", x, y)
			}
		}
	}
	if changes == 0 {
		t.Fatal("手冊 layer 應改變已核准矩形內像素")
	}
}

func TestLoadManualOverlayLayoutRejectsDrift(t *testing.T) {
	if _, err := LoadManualOverlayLayout("manual.tsv", []byte(manualLayoutFixture)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data string
	}{
		{"schema", "bad\n"},
		{"geometry", strings.Replace(manualLayoutFixture, "\t305\t112", "\t304\t112", 1)},
		{"capacity", strings.Replace(manualLayoutFixture, "\t504\t8\t0", "\t503\t8\t0", 1)},
		{"policy", strings.Replace(manualLayoutFixture, "single-page-reject", "truncate", 1)},
		{"two rows", manualLayoutFixture + strings.SplitN(manualLayoutFixture, "\n", 2)[1]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadManualOverlayLayout("bad.tsv", []byte(tc.data)); err == nil {
				t.Fatal("layout 漂移必須拒絕")
			}
		})
	}
}

func TestRuntimeManualOverlayRejectsInvalidInputs(t *testing.T) {
	layout := loadManualOverlayLayout(t)
	catalog := manualOverlayCatalog("繁中段落")
	font := manualOverlayFont(catalog)
	for _, tc := range []struct {
		name    string
		layout  *ManualOverlayLayout
		catalog *Catalog
		font    *xlate.Font
		scale   int
	}{
		{"nil layout", nil, catalog, font, 2},
		{"nil catalog", layout, nil, font, 2},
		{"wrong font", layout, catalog, &xlate.Font{W: 8, H: 8}, 2},
		{"bad scale", layout, catalog, font, 1},
		{"bad scale high", layout, catalog, font, 4},
		{"over capacity", layout, manualOverlayCatalog(strings.Repeat("字", 505)), &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{'字': bytes.Repeat([]byte{0xff}, 32)}}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := NewRuntimeManualOverlay(tc.layout, tc.catalog, tc.font, tc.scale); got != nil || err == nil {
				t.Fatalf("無效 constructor 必須回 nil,error；got=%#v err=%v", got, err)
			}
		})
	}
	delete(font.Glyphs, '繁')
	if got, err := NewRuntimeManualOverlay(layout, catalog, font, 2); got != nil || err == nil {
		t.Fatal("正式 catalog 缺 glyph 必須拒絕整個 presenter")
	}
}

func TestRuntimeManualOverlayBuildsFourteenRowsAndContainsRGBA(t *testing.T) {
	for _, scale := range []int{2, 3} {
		t.Run("scale", func(t *testing.T) {
			layout := loadManualOverlayLayout(t)
			catalog := manualOverlayCatalog("繁中段落")
			o, err := NewRuntimeManualOverlay(layout, catalog, manualOverlayFont(catalog), scale)
			if err != nil {
				t.Fatal(err)
			}
			if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: 1}); err != nil {
				t.Fatal(err)
			}
			request := manualOverlayRequest(catalog, 1)
			if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 1, Request: request}); err != nil {
				t.Fatal(err)
			}
			if len(o.background.Stamps) != 14 || len(o.text.Stamps) != 14 || len(o.ActiveKeys()) != 14 {
				t.Fatalf("background=%d text=%d keys=%d", len(o.background.Stamps), len(o.text.Stamps), len(o.ActiveKeys()))
			}
			for row, stamp := range o.text.Stamps {
				if stamp.X != 16 || stamp.Y != 72+row*8 || stamp.Cells != 36 || stamp.CellW != 8 || stamp.CellH != 8 {
					t.Fatalf("row %d geometry=%#v", row, stamp)
				}
			}
			palette, indexed := manualOverlayPaletteAndFrame()
			baseline := ScaleIndexedRGBA(indexed, palette, scale)
			o.Frame(indexed, palette)
			rgba, missing, drew := o.Draw(indexed, palette)
			if !drew || len(missing) != 0 || len(rgba) != 320*scale*200*scale*4 {
				t.Fatalf("drew=%v missing=%q rgba=%d", drew, string(missing), len(rgba))
			}
			action := o.Actions()[0]
			assertDiffOnlyInside(t, baseline, rgba, 320*scale, 200*scale, action.ClearRect)
			if got := rgbaAt(rgba, 320*scale, 7*scale, 72*scale); got != [4]uint8{10, 20, 30, 255} {
				t.Fatalf("左側 clear margin=%v，應由背景 layer 清除", got)
			}
			if action.ClearRect != (PixelRect{7 * scale, 72 * scale, 305 * scale, 112 * scale}) ||
				action.TextRect != (PixelRect{16 * scale, 72 * scale, 288 * scale, 112 * scale}) {
				t.Fatalf("action geometry=%#v", action)
			}
		})
	}
}

func TestManualThreeXEnlargesOnlyChineseGlyphs(t *testing.T) {
	catalog := manualOverlayCatalog("中文字A")
	source := manualOverlayFont(catalog)
	before := append([]byte(nil), source.Glyphs['中']...)
	for _, scale := range []int{2, 3} {
		o, err := NewRuntimeManualOverlay(loadManualOverlayLayout(t), catalog, source, scale)
		if err != nil {
			t.Fatal(err)
		}
		if scale == 2 && (o.font != source || o.font.W != 16 || manualGlyphOffset(scale) != 0) {
			t.Fatal("2x 必須沿用原始 16x16 字型與位置")
		}
		if scale == 3 {
			if o.font == source || o.font.W != 22 || o.font.H != 22 || manualGlyphOffset(scale) != 1 {
				t.Fatal("3x 中文須使用 22x22 衍生字型，置於 24px 字格")
			}
			if len(o.font.Glyphs['中']) != 66 || len(o.font.Glyphs['A']) != 66 {
				t.Fatal("3x 衍生字模長度無效")
			}
			// ASCII retains a 16x16 ink box centred in the derived bitmap.
			if o.font.Glyphs['A'][3*3+3/8] == 0 {
				t.Fatal("ASCII 應被保留在 3x 衍生字模內")
			}
		}
	}
	if !bytes.Equal(source.Glyphs['中'], before) || source.W != 16 {
		t.Fatal("不得修改來源字型")
	}
}

func TestRuntimeManualOverlayRowMajorCapacityAndLifecycle(t *testing.T) {
	layout := loadManualOverlayLayout(t)
	catalog := manualOverlayCatalog(strings.Repeat("字", 504))
	o, err := NewRuntimeManualOverlay(layout, catalog, manualOverlayFont(catalog), 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: 2}); err != nil {
		t.Fatal(err)
	}
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationClear, Generation: 2}); err != nil {
		t.Fatal("pending clear 必須保留 pending：", err)
	}
	request := manualOverlayRequest(catalog, 2)
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 2, Request: request}); err != nil {
		t.Fatal(err)
	}
	for row, stamp := range o.text.Stamps {
		if len(stamp.Text) != 36 || string(stamp.Text) != strings.Repeat("字", 36) {
			t.Fatalf("row %d=%q", row, string(stamp.Text))
		}
	}
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationClear, Generation: 2}); err != nil {
		t.Fatal(err)
	}
	if len(o.ActiveKeys()) != 0 {
		t.Fatal("visible clear 必須移除全部手冊文字 stamps")
	}
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 2, Request: request}); err == nil {
		t.Fatal("cleared generation 不得重新 request")
	}
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: 3}); err != nil {
		t.Fatal(err)
	}
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 2, Request: request}); err == nil {
		t.Fatal("上一代 request 不得在新 generation 顯示")
	}
	if o.ActiveGeneration() != 3 || len(o.ActiveKeys()) != 0 {
		t.Fatalf("new generation=%d keys=%v", o.ActiveGeneration(), o.ActiveKeys())
	}
}

func TestRuntimeManualOverlayFailsClosedAndCopiesActions(t *testing.T) {
	layout := loadManualOverlayLayout(t)
	catalog := manualOverlayCatalog("繁中段落")
	o, err := NewRuntimeManualOverlay(layout, catalog, manualOverlayFont(catalog), 2)
	if err != nil {
		t.Fatal(err)
	}
	request := manualOverlayRequest(catalog, 1)
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 1, Request: request}); err == nil {
		t.Fatal("沒有 begin 的 request 必須拒絕")
	}
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: 1}); err != nil {
		t.Fatal(err)
	}
	bad := request
	bad.TextKey = "mutated"
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 1, Request: bad}); err == nil {
		t.Fatal("catalog 不相符 request 必須拒絕")
	}
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 1, Request: request}); err != nil {
		t.Fatal(err)
	}
	actions := o.Actions()
	actions[0].TextKey = "mutated"
	if o.Actions()[0].TextKey != request.TextKey {
		t.Fatal("Actions 回傳值不得污染 presenter")
	}
	for _, event := range []ManualPresentationEvent{
		{Kind: ManualPresentationBegin, Generation: 1},
		{Kind: ManualPresentationClear, Generation: 0},
		{Kind: ManualPresentationClear, Generation: 2},
		{Kind: ManualPresentationKind("unknown"), Generation: 1},
	} {
		if err := o.Apply(event); err == nil {
			t.Fatalf("無效 event=%#v 必須拒絕", event)
		}
	}
}

func TestRuntimeManualOverlayUsesObservedStyleWithoutFrameSampling(t *testing.T) {
	catalog := manualOverlayCatalog("中")
	o, err := NewRuntimeManualOverlay(loadManualOverlayLayout(t), catalog, manualOverlayFont(catalog), 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := o.SetStyle(ManualTextStyle{Background: 10, Foreground: 15, Row: 2, Column: 3}); err != nil {
		t.Fatal(err)
	}
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationBegin, Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := o.Apply(ManualPresentationEvent{Kind: ManualPresentationRequest, Generation: 1, Request: manualOverlayRequest(catalog, 1)}); err != nil {
		t.Fatal(err)
	}
	palette, indexed := manualOverlayPaletteAndFrame()
	// The body has deliberately no foreground index 15. Frame must still use
	// the observed prompt style, not infer black-on-black from this rectangle.
	for y := 72; y < 184; y++ {
		for x := 7; x < 312; x++ {
			indexed[y*320+x] = 10
		}
	}
	o.Frame(indexed, palette)
	if got := o.text.Stamps[0].FG; got != palette[15] {
		t.Fatalf("FG=%v，要原版觀測 palette[15]=%v", got, palette[15])
	}
	if got := o.text.Stamps[0].BG; got != palette[10] {
		t.Fatalf("BG=%v，要原版觀測 palette[10]=%v", got, palette[10])
	}
	palette[15] = [3]uint8{1, 2, 3}
	o.Frame(indexed, palette)
	if got := o.text.Stamps[0].FG; got != palette[15] {
		t.Fatalf("調色盤更新後 FG=%v，要 %v", got, palette[15])
	}
	if _, missing, drew := o.Draw(indexed, palette); !drew || len(missing) != 0 {
		t.Fatalf("drew=%v missing=%q", drew, string(missing))
	}
}

func TestRuntimeManualOverlayFreshAfterRestoreHasNoDerivedLayer(t *testing.T) {
	catalog := manualOverlayCatalog("中")
	o, err := NewRuntimeManualOverlay(loadManualOverlayLayout(t), catalog, manualOverlayFont(catalog), 2)
	if err != nil {
		t.Fatal(err)
	}
	palette, indexed := manualOverlayPaletteAndFrame()
	baseline := ScaleIndexedRGBA(indexed, palette, 2)
	rgba, missing, drew := o.Draw(indexed, palette)
	if o.HasStyle() || drew || len(missing) != 0 || len(o.ActiveKeys()) != 0 {
		t.Fatalf("新 restore presenter 不得保有衍生 layer: style=%v drew=%v missing=%q keys=%v", o.HasStyle(), drew, string(missing), o.ActiveKeys())
	}
	if !bytes.Equal(baseline, rgba) {
		t.Fatal("新 restore presenter 必須回傳未覆繪 baseline")
	}
}
