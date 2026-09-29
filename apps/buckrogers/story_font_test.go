package buckrogers

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"hash/crc32"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

// 規格 031 §5 第 4 條（§3.4 修訂）單元測試。全部使用合成字型與合成字形表
// （syntheticTable），不含原版字模。

// storyFontBase 是合成 16×16 底字型：ASCII 與中文字模都不是原版。
func storyFontBase() *xlate.Font {
	f := &xlate.Font{W: 16, H: 16, Name: "synthetic", Glyphs: map[rune][]byte{}}
	for _, r := range "中文A(N" {
		g := make([]byte, 32)
		for i := range g {
			g[i] = byte(r) ^ byte(i*7) | 0x18
		}
		f.Glyphs[r] = g
	}
	return f
}

const storyFontText = "中A(文N"

func storyFontTexts(page string, n int) map[string]string {
	t := map[string]string{}
	for i := 1; i <= n; i++ {
		t[fmt.Sprintf("story.%s.line.%03d", page, i)] = storyFontText
	}
	return t
}

var storyFontPalette = func() (p [256][3]uint8) {
	p[0] = [3]uint8{1, 2, 3}
	p[10] = [3]uint8{200, 210, 220}
	return
}()

// storyPresenterUT adapts one page presenter type for the table tests.
type storyPresenterUT struct {
	page  string
	lines int
	build func(map[string]string, *xlate.Font, int) (any, error)
	apply func(any) error
	layer func(any) *xlate.Layer
	font  func(any) *xlate.Font
	set   func(any, *xlate.Font) error
	draw  func(any) []byte
}

func storyEventKey(page string, i int) string { return fmt.Sprintf("story.%s.line.%03d", page, i+1) }

func storyPresentersUT() []storyPresenterUT {
	ix := make([]byte, 320*200)
	drawOf := func(d func([]byte, [256][3]uint8) ([]byte, []rune, bool)) []byte {
		out, miss, _ := d(ix, storyFontPalette)
		if len(miss) != 0 {
			panic(fmt.Sprintf("missing %q", string(miss)))
		}
		return out
	}
	return []storyPresenterUT{
		{page: "opening", lines: 5,
			build: func(t map[string]string, f *xlate.Font, s int) (any, error) {
				return NewRuntimeStoryOpeningOverlay(t, f, s)
			},
			apply: func(p any) error {
				var es []StoryOpeningEvent
				for i := 0; i < 5; i++ {
					es = append(es, StoryOpeningEvent{Generation: 1, EntryStep: uint64(10*i + 1), PostCallStep: uint64(10*i + 5), EventKey: storyEventKey("opening", i), Row: uint8(17 + i), Column: 1})
				}
				return p.(*RuntimeStoryOpeningOverlay).Apply(es, storyFontPalette)
			},
			layer: func(p any) *xlate.Layer { return p.(*RuntimeStoryOpeningOverlay).layer },
			font:  func(p any) *xlate.Font { return p.(*RuntimeStoryOpeningOverlay).font },
			set:   func(p any, f *xlate.Font) error { return p.(*RuntimeStoryOpeningOverlay).SetFont(f) },
			draw:  func(p any) []byte { return drawOf(p.(*RuntimeStoryOpeningOverlay).Draw) }},
		{page: "page2", lines: 4,
			build: func(t map[string]string, f *xlate.Font, s int) (any, error) {
				return NewRuntimeStoryPage2Overlay(t, f, s)
			},
			apply: func(p any) error {
				var es []StoryPage2Event
				for i := 0; i < 4; i++ {
					es = append(es, StoryPage2Event{Generation: 1, EntryStep: uint64(10*i + 1), PostCallStep: uint64(10*i + 5), EventKey: storyEventKey("page2", i), Row: uint8(17 + i), Column: 1})
				}
				return p.(*RuntimeStoryPage2Overlay).Apply(es, storyFontPalette)
			},
			layer: func(p any) *xlate.Layer { return p.(*RuntimeStoryPage2Overlay).layer },
			font:  func(p any) *xlate.Font { return p.(*RuntimeStoryPage2Overlay).font },
			set:   func(p any, f *xlate.Font) error { return p.(*RuntimeStoryPage2Overlay).SetFont(f) },
			draw:  func(p any) []byte { return drawOf(p.(*RuntimeStoryPage2Overlay).Draw) }},
		{page: "page3", lines: 5,
			build: func(t map[string]string, f *xlate.Font, s int) (any, error) {
				return NewRuntimeStoryPage3Overlay(t, f, s)
			},
			apply: func(p any) error {
				var es []StoryPage3Event
				for i := 0; i < 5; i++ {
					es = append(es, StoryPage3Event{Generation: 1, EntryStep: uint64(10*i + 1), PostCallStep: uint64(10*i + 5), EventKey: storyEventKey("page3", i), Row: uint8(17 + i), Column: 1})
				}
				return p.(*RuntimeStoryPage3Overlay).Apply(es, storyFontPalette)
			},
			layer: func(p any) *xlate.Layer { return p.(*RuntimeStoryPage3Overlay).layer },
			font:  func(p any) *xlate.Font { return p.(*RuntimeStoryPage3Overlay).font },
			set:   func(p any, f *xlate.Font) error { return p.(*RuntimeStoryPage3Overlay).SetFont(f) },
			draw:  func(p any) []byte { return drawOf(p.(*RuntimeStoryPage3Overlay).Draw) }},
		{page: "page4", lines: 6,
			build: func(t map[string]string, f *xlate.Font, s int) (any, error) {
				return NewRuntimeStoryPage4Overlay(t, f, s)
			},
			apply: func(p any) error {
				var es []StoryPage4Event
				for i := 0; i < 6; i++ {
					es = append(es, StoryPage4Event{Generation: 1, EntryStep: uint64(10*i + 1), PostCallStep: uint64(10*i + 5), EventKey: storyEventKey("page4", i), Row: uint8(17 + i), Column: 1})
				}
				return p.(*RuntimeStoryPage4Overlay).Apply(es, storyFontPalette)
			},
			layer: func(p any) *xlate.Layer { return p.(*RuntimeStoryPage4Overlay).layer },
			font:  func(p any) *xlate.Font { return p.(*RuntimeStoryPage4Overlay).font },
			set:   func(p any, f *xlate.Font) error { return p.(*RuntimeStoryPage4Overlay).SetFont(f) },
			draw:  func(p any) []byte { return drawOf(p.(*RuntimeStoryPage4Overlay).Draw) }},
		{page: "page5", lines: 5,
			build: func(t map[string]string, f *xlate.Font, s int) (any, error) {
				return NewRuntimeStoryPage5Overlay(t, f, s)
			},
			apply: func(p any) error {
				var es []StoryPage5Event
				for i := 0; i < 5; i++ {
					es = append(es, StoryPage5Event{Generation: 1, EntryStep: uint64(10*i + 1), PostCallStep: uint64(10*i + 5), EventKey: storyEventKey("page5", i), Row: uint8(17 + i), Column: 1})
				}
				return p.(*RuntimeStoryPage5Overlay).Apply(es, storyFontPalette)
			},
			layer: func(p any) *xlate.Layer { return p.(*RuntimeStoryPage5Overlay).layer },
			font:  func(p any) *xlate.Font { return p.(*RuntimeStoryPage5Overlay).font },
			set:   func(p any, f *xlate.Font) error { return p.(*RuntimeStoryPage5Overlay).SetFont(f) },
			draw:  func(p any) []byte { return drawOf(p.(*RuntimeStoryPage5Overlay).Draw) }},
		{page: "page6", lines: 6,
			build: func(t map[string]string, f *xlate.Font, s int) (any, error) {
				return NewRuntimeStoryPage6Overlay(t, f, s)
			},
			apply: func(p any) error {
				var es []StoryPage6Event
				for i := 0; i < 6; i++ {
					es = append(es, StoryPage6Event{Generation: 1, EntryStep: uint64(10*i + 1), PostCallStep: uint64(10*i + 5), EventKey: storyEventKey("page6", i), Row: uint8(17 + i), Column: 1})
				}
				return p.(*RuntimeStoryPage6Overlay).Apply(es, storyFontPalette)
			},
			layer: func(p any) *xlate.Layer { return p.(*RuntimeStoryPage6Overlay).layer },
			font:  func(p any) *xlate.Font { return p.(*RuntimeStoryPage6Overlay).font },
			set:   func(p any, f *xlate.Font) error { return p.(*RuntimeStoryPage6Overlay).SetFont(f) },
			draw:  func(p any) []byte { return drawOf(p.(*RuntimeStoryPage6Overlay).Draw) }},
		{page: "page7", lines: 6,
			build: func(t map[string]string, f *xlate.Font, s int) (any, error) {
				return NewRuntimeStoryPage7Overlay(t, f, s)
			},
			apply: func(p any) error {
				var es []StoryPage7Event
				for i := 0; i < 6; i++ {
					es = append(es, StoryPage7Event{Generation: 1, EntryStep: uint64(10*i + 1), PostCallStep: uint64(10*i + 5), EventKey: storyEventKey("page7", i), Row: uint8(17 + i), Column: 1})
				}
				return p.(*RuntimeStoryPage7Overlay).Apply(es, storyFontPalette)
			},
			layer: func(p any) *xlate.Layer { return p.(*RuntimeStoryPage7Overlay).layer },
			font:  func(p any) *xlate.Font { return p.(*RuntimeStoryPage7Overlay).font },
			set:   func(p any, f *xlate.Font) error { return p.(*RuntimeStoryPage7Overlay).SetFont(f) },
			draw:  func(p any) []byte { return drawOf(p.(*RuntimeStoryPage7Overlay).Draw) }},
		{page: "page8", lines: 4,
			build: func(t map[string]string, f *xlate.Font, s int) (any, error) {
				return NewRuntimeStoryPage8Overlay(t, f, s)
			},
			apply: func(p any) error {
				var es []StoryPage8Event
				for i := 0; i < 4; i++ {
					es = append(es, StoryPage8Event{Generation: 1, EntryStep: uint64(10*i + 1), PostCallStep: uint64(10*i + 5), EventKey: storyEventKey("page8", i), Row: uint8(17 + i), Column: 1})
				}
				return p.(*RuntimeStoryPage8Overlay).Apply(es, storyFontPalette)
			},
			layer: func(p any) *xlate.Layer { return p.(*RuntimeStoryPage8Overlay).layer },
			font:  func(p any) *xlate.Font { return p.(*RuntimeStoryPage8Overlay).font },
			set:   func(p any, f *xlate.Font) error { return p.(*RuntimeStoryPage8Overlay).SetFont(f) },
			draw:  func(p any) []byte { return drawOf(p.(*RuntimeStoryPage8Overlay).Draw) }},
		{page: "page9", lines: 1,
			build: func(t map[string]string, f *xlate.Font, s int) (any, error) {
				return NewRuntimeStoryPage9Overlay(t, f, s)
			},
			apply: func(p any) error {
				return p.(*RuntimeStoryPage9Overlay).Apply(StoryPage9Event{Generation: 1, EntryStep: 1, PostCallStep: 5, EventKey: storyEventKey("page9", 0), Row: 17, Column: 1}, storyFontPalette)
			},
			layer: func(p any) *xlate.Layer { return p.(*RuntimeStoryPage9Overlay).layer },
			font:  func(p any) *xlate.Font { return p.(*RuntimeStoryPage9Overlay).font },
			set:   func(p any, f *xlate.Font) error { return p.(*RuntimeStoryPage9Overlay).SetFont(f) },
			draw:  func(p any) []byte { return drawOf(p.(*RuntimeStoryPage9Overlay).Draw) }},
	}
}

// §5.4：SetFont 後 layer 指標、stamp 數不變，stamp 改用新字型，Draw 等同以新字型建構並
// Apply 的 presenter；3× 依各頁既有規則衍生（第 4 頁不衍生）。
func TestStorySetFontKeepsLayerAndDrawsNewFont(t *testing.T) {
	base := storyFontBase()
	orig := OriginalASCIIFont(base, syntheticTable())
	for _, ut := range storyPresentersUT() {
		for _, scale := range liveScales {
			name := fmt.Sprintf("%s/%dx", ut.page, scale)
			text := storyFontTexts(ut.page, ut.lines)
			p, err := ut.build(text, base, scale)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if err := ut.apply(p); err != nil {
				t.Fatalf("%s apply: %v", name, err)
			}
			// 取得前：presenter 用原字型（ASCII 為底字型字模）。
			before := ut.draw(p)
			wantBefore := ut.font(p).Glyphs['A']
			if scale == 2 || ut.page == "page4" {
				if !bytes.Equal(wantBefore, base.Glyphs['A']) {
					t.Fatalf("%s: 取得前字型不是底字型", name)
				}
			}
			layer := ut.layer(p)
			stamps := append([]*xlate.Stamp(nil), layer.Stamps...)
			if err := ut.set(p, orig); err != nil {
				t.Fatalf("%s SetFont: %v", name, err)
			}
			if ut.layer(p) != layer || len(layer.Stamps) != len(stamps) || len(stamps) != ut.lines {
				t.Fatalf("%s: layer 或 stamp 數改變", name)
			}
			nf := ut.font(p)
			for i, s := range layer.Stamps {
				if s != stamps[i] || s.Font != nf {
					t.Fatalf("%s: stamp %d 身分或字型不對", name, i)
				}
			}
			// 各頁倍率規則。
			switch {
			case scale == 2 || ut.page == "page4":
				if nf.W != 16 || !bytes.Equal(nf.Glyphs['A'], orig.Glyphs['A']) || !bytes.Equal(nf.Glyphs['中'], base.Glyphs['中']) {
					t.Fatalf("%s: 16×16 字型不是 .orig-ascii 基底", name)
				}
			default:
				want := manualThreeXFont(orig)
				if nf.W != 22 || !bytes.Equal(nf.Glyphs['A'], want.Glyphs['A']) || !bytes.Equal(nf.Glyphs['中'], want.Glyphs['中']) {
					t.Fatalf("%s: 3× 衍生不符既有規則", name)
				}
			}
			if scale == 3 && strings.Contains("opening page2 page3 page5 page6", ut.page) && !strings.HasPrefix(nf.Name, "synthetic.orig-ascii.") {
				t.Fatalf("%s: 3× 名稱 %q 未沿用既有規則", name, nf.Name)
			}
			after := ut.draw(p)
			fresh, err := ut.build(text, orig, scale)
			if err != nil {
				t.Fatal(err)
			}
			if err := ut.apply(fresh); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, ut.draw(fresh)) {
				t.Fatalf("%s: SetFont 後 Draw 與新字型 presenter 不同", name)
			}
			if bytes.Equal(before, after) {
				t.Fatalf("%s: SetFont 後畫面沒有改變", name)
			}
			if name := ut.font(fresh).Name; name != nf.Name {
				t.Fatalf("%s: 名稱 %q 與建構子 %q 不同", ut.page, nf.Name, name)
			}
		}
	}
}

// SetFont 驗證失敗（缺字）時保留原字型與 stamp。
func TestStorySetFontFailureKeepsFont(t *testing.T) {
	base := storyFontBase()
	bad := &xlate.Font{W: 16, H: 16, Name: "bad", Glyphs: map[rune][]byte{'A': make([]byte, 32)}}
	for _, ut := range storyPresentersUT() {
		for _, scale := range liveScales {
			p, err := ut.build(storyFontTexts(ut.page, ut.lines), base, scale)
			if err != nil {
				t.Fatal(err)
			}
			if err := ut.apply(p); err != nil {
				t.Fatal(err)
			}
			old := ut.font(p)
			if err := ut.set(p, bad); err == nil {
				t.Fatalf("%s/%dx: 缺字字型被接受", ut.page, scale)
			}
			if ut.font(p) != old || ut.layer(p).Stamps[0].Font != old {
				t.Fatalf("%s/%dx: 失敗後字型被改", ut.page, scale)
			}
		}
	}
}

// storyLiveFamiliesUT builds every live story family from synthetic text;
// watchers are not needed for setFont.
func storyLiveFamiliesUT(t *testing.T, base *xlate.Font) []storyFamily {
	t.Helper()
	var out []storyFamily
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	op := &storyOpeningLive{gen: 7}
	p2 := &storyPage2Live{gen: 7}
	p3 := &storyPage3Live{gen: 7}
	p4 := &storyPage4Live{gen: 7}
	p5 := &storyPage5Live{gen: 7}
	p6 := &storyPage6Live{gen: 7}
	p7 := &storyPage7Live{gen: 7}
	p8 := &storyPage8Live{gen: 7}
	p9 := &storyPage9Live{gen: [2]uint64{7, 7}}
	var err error
	for i, s := range liveScales {
		op.pres[i], err = NewRuntimeStoryOpeningOverlay(storyFontTexts("opening", 5), base, s)
		must(err)
		p2.pres[i], err = NewRuntimeStoryPage2Overlay(storyFontTexts("page2", 4), base, s)
		must(err)
		p3.pres[i], err = NewRuntimeStoryPage3Overlay(storyFontTexts("page3", 5), base, s)
		must(err)
		p4.pres[i], err = NewRuntimeStoryPage4Overlay(storyFontTexts("page4", 6), base, s)
		must(err)
		p5.pres[i], err = NewRuntimeStoryPage5Overlay(storyFontTexts("page5", 5), base, s)
		must(err)
		p6.pres[i], err = NewRuntimeStoryPage6Overlay(storyFontTexts("page6", 6), base, s)
		must(err)
		p7.pres[i], err = NewRuntimeStoryPage7Overlay(storyFontTexts("page7", 6), base, s)
		must(err)
		p8.pres[i], err = NewRuntimeStoryPage8Overlay(storyFontTexts("page8", 4), base, s)
		must(err)
		pr, err := NewRuntimeStoryPage9Overlay(storyFontTexts("page9", 1), base, s)
		must(err)
		p9.owner[i] = &StoryPage9Owner{Watcher: &StoryPage9Watcher{}, Presenter: pr}
	}
	return append(out, op, p2, p3, p4, p5, p6, p7, p8, p9)
}

// storyFamilyFonts lists (presenter pointer, font) for every scale.
func storyFamilyFonts(f storyFamily) (ps []any, fs []*xlate.Font) {
	for i := range liveScales {
		switch v := f.(type) {
		case *storyOpeningLive:
			ps, fs = append(ps, v.pres[i]), append(fs, v.pres[i].font)
		case *storyPage2Live:
			ps, fs = append(ps, v.pres[i]), append(fs, v.pres[i].font)
		case *storyPage3Live:
			ps, fs = append(ps, v.pres[i]), append(fs, v.pres[i].font)
		case *storyPage4Live:
			ps, fs = append(ps, v.pres[i]), append(fs, v.pres[i].font)
		case *storyPage5Live:
			ps, fs = append(ps, v.pres[i]), append(fs, v.pres[i].font)
		case *storyPage6Live:
			ps, fs = append(ps, v.pres[i]), append(fs, v.pres[i].font)
		case *storyPage7Live:
			ps, fs = append(ps, v.pres[i]), append(fs, v.pres[i].font)
		case *storyPage8Live:
			ps, fs = append(ps, v.pres[i]), append(fs, v.pres[i].font)
		case *storyPage9Live:
			ps, fs = append(ps, v.owner[i].Presenter), append(fs, v.owner[i].Presenter.font)
		}
	}
	return
}

func storyFamilyGen(f storyFamily) [2]uint64 {
	switch v := f.(type) {
	case *storyOpeningLive:
		return [2]uint64{v.gen, v.gen}
	case *storyPage2Live:
		return [2]uint64{v.gen, v.gen}
	case *storyPage3Live:
		return [2]uint64{v.gen, v.gen}
	case *storyPage4Live:
		return [2]uint64{v.gen, v.gen}
	case *storyPage5Live:
		return [2]uint64{v.gen, v.gen}
	case *storyPage6Live:
		return [2]uint64{v.gen, v.gen}
	case *storyPage7Live:
		return [2]uint64{v.gen, v.gen}
	case *storyPage8Live:
		return [2]uint64{v.gen, v.gen}
	case *storyPage9Live:
		return v.gen
	}
	return [2]uint64{}
}

// §5.4：family 層 setFont 不換 presenter、不改 generation；第 9 頁只換 Presenter 字型，
// owner 與 watcher 不變。
func TestLiveStorySetFontKeepsPresenterAndGeneration(t *testing.T) {
	base := storyFontBase()
	orig := OriginalASCIIFont(base, syntheticTable())
	for _, f := range storyLiveFamiliesUT(t, base) {
		ps, fs := storyFamilyFonts(f)
		var p9 [2]StoryPage9Owner
		if v, ok := f.(*storyPage9Live); ok {
			for i := range liveScales {
				p9[i] = *v.owner[i]
			}
		}
		if err := f.setFont(orig); err != nil {
			t.Fatalf("%s: %v", f.name(), err)
		}
		ps2, fs2 := storyFamilyFonts(f)
		for i := range ps {
			if ps[i] != ps2[i] || fs[i] == fs2[i] {
				t.Fatalf("%s[%d]: presenter 被換或字型未換", f.name(), i)
			}
		}
		if g := storyFamilyGen(f); g != [2]uint64{7, 7} {
			t.Fatalf("%s: generation 改變 %v", f.name(), g)
		}
		if v, ok := f.(*storyPage9Live); ok {
			for i := range liveScales {
				if v.owner[i].Watcher != p9[i].Watcher || v.owner[i].Presenter != p9[i].Presenter {
					t.Fatal("第 9 頁 owner 的 watcher／presenter 被換")
				}
			}
		}
	}
}

// fakeStepView is a synthetic StepReader over fakeMem.
type fakeStepView struct {
	fakeMem
	steps uint64
}

func (v fakeStepView) Steps() uint64        { return v.steps }
func (fakeStepView) CS() uint16             { return 0 }
func (fakeStepView) IP() uint16             { return 0 }
func (fakeStepView) SS() uint16             { return 0 }
func (fakeStepView) SP() uint16             { return 0 }
func (fakeStepView) DS() uint16             { return 0 }
func (fakeStepView) ES() uint16             { return 0 }
func (fakeStepView) DI() uint16             { return 0 }
func (fakeStepView) CX() uint16             { return 0 }
func (fakeStepView) Palette() [256][3]uint8 { return storyFontPalette }

// withSyntheticFinder swaps the production hash for the synthetic table's.
func withSyntheticFinder(t *testing.T) {
	t.Helper()
	tab := syntheticTable()
	sum := sha256.Sum256(tab)
	old := origASCIIFinder
	origASCIIFinder = func(m MemReader, limit uint32) ([]byte, bool) {
		return findGlyphTable(m, limit, crc32.ChecksumIEEE(tab[:16]), sum[:])
	}
	t.Cleanup(func() { origASCIIFinder = old })
}

func syntheticMemory(withTable bool) fakeStepView {
	m := make(fakeMem, origASCIIScanLimit)
	if withTable {
		copy(m[0x3211:], syntheticTable())
	}
	return fakeStepView{fakeMem: m, steps: 4242}
}

func storyFontNames(t *testing.T, fams []storyFamily) []string {
	t.Helper()
	var out []string
	for _, f := range fams {
		_, fs := storyFamilyFonts(f)
		for _, x := range fs {
			out = append(out, x.Name)
		}
	}
	return out
}

func assertStoryFontsDerived(t *testing.T, fams []storyFamily, derived bool) {
	t.Helper()
	for _, f := range fams {
		_, fs := storyFamilyFonts(f)
		for i, x := range fs {
			// 2×（與第 4 頁 3×）的字型就是 16×16：原字型的 'A' 是底字型，衍生後是合成表。
			if x.W != 16 {
				continue
			}
			has := bytes.Equal(x.Glyphs['A'], OriginalASCIIFont(storyFontBase(), syntheticTable()).Glyphs['A'])
			if has != derived {
				t.Fatalf("%s[%d]: derived=%v want %v", f.name(), i, has, derived)
			}
		}
	}
}

// §5.4：通用路徑先取得時，劇情家族也換字型；取得前用原字型。
func TestLiveGenericAcquisitionSwitchesStoryFonts(t *testing.T) {
	withSyntheticFinder(t)
	base := storyFontBase()
	c, err := LoadLogbookCatalog([]byte("key\ttranslation\tsource\nlogbook.5\t正文。\tx\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	w := NewLogbookWatcher(c, nil)
	w.open = 5
	fams := storyLiveFamiliesUT(t, base)
	r := &LiveRuntime{resets: map[string]int{}, font: base, logbook: w, stories: fams}
	assertStoryFontsDerived(t, fams, false)
	if err := r.findOriginalASCII(syntheticMemory(true), false); err != nil {
		t.Fatal(err)
	}
	if !r.asciiFound || r.asciiScans != 1 || r.asciiStep != 4242 || r.asciiStoryErrs != 0 {
		t.Fatalf("found=%v scans=%d step=%d errs=%d", r.asciiFound, r.asciiScans, r.asciiStep, r.asciiStoryErrs)
	}
	assertStoryFontsDerived(t, fams, true)
	for _, n := range storyFontNames(t, fams) {
		if n != "" && !strings.HasPrefix(n, "synthetic.orig-ascii") {
			t.Fatalf("字型名稱 %q", n)
		}
	}
	if r.logbookPres[0] == nil || !strings.HasPrefix(r.logbookPres[0].font.Name, "synthetic.orig-ascii") {
		t.Fatal("通用家族未依 §3.1 重建")
	}
	if !strings.Contains(r.DebugSummary(), "orig-ascii=true/1 orig-ascii-step=4242") {
		t.Fatalf("summary: %s", r.DebugSummary())
	}
}

// fakeStoryFamily records the order of setFont and apply.
type fakeStoryFamily struct {
	want  bool
	calls []string
}

func (*fakeStoryFamily) name() string                                                          { return "fake" }
func (*fakeStoryFamily) glyphEntry(Address, uint16, uint16, [7]uint16, uint64)                 {}
func (*fakeStoryFamily) verifiedReturn(uint64, Address, Address, byte, uint16, uint16, uint64) {}
func (*fakeStoryFamily) discontinuity()                                                        {}
func (*fakeStoryFamily) clearWrite(Address, uint16, uint16, uint16) bool                       { return false }
func (*fakeStoryFamily) prewrite(machine.VideoWrite)                                           {}
func (*fakeStoryFamily) frame([]byte, [256][3]uint8)                                           {}
func (*fakeStoryFamily) draw(int, []byte, [256][3]uint8) ([]byte, []rune, bool) {
	return nil, nil, false
}
func (*fakeStoryFamily) clear()             {}
func (f *fakeStoryFamily) needsApply() bool { return f.want }
func (f *fakeStoryFamily) apply([256][3]uint8) error {
	f.calls = append(f.calls, "apply")
	f.want = false
	return nil
}
func (f *fakeStoryFamily) setFont(*xlate.Font) error {
	f.calls = append(f.calls, "setFont")
	return nil
}

// §3.4：通用家族沒有內容時，劇情 needsApply 會觸發搜尋，而且在 apply 迴圈之前。
func TestLiveStoryNeedsApplyTriggersSearchBeforeApply(t *testing.T) {
	withSyntheticFinder(t)
	base := storyFontBase()
	fake := &fakeStoryFamily{}
	fams := append(storyLiveFamiliesUT(t, base), fake)
	r := &LiveRuntime{resets: map[string]int{}, font: base, stories: fams}
	v := syntheticMemory(true)
	// 通用家族沒有內容、劇情也不需要 apply：不搜尋。
	if err := r.findOriginalASCII(v, false); err != nil || r.asciiScans != 0 {
		t.Fatalf("generic-only search ran: scans=%d err=%v", r.asciiScans, err)
	}
	if err := r.applyStories(v); err != nil || r.asciiScans != 0 {
		t.Fatalf("idle stories searched: scans=%d err=%v", r.asciiScans, err)
	}
	fake.want = true
	if err := r.applyStories(v); err != nil {
		t.Fatal(err)
	}
	if !r.asciiFound || r.asciiScans != 1 || r.asciiStep != 4242 {
		t.Fatalf("found=%v scans=%d step=%d", r.asciiFound, r.asciiScans, r.asciiStep)
	}
	if strings.Join(fake.calls, ",") != "setFont,apply" {
		t.Fatalf("順序 %v", fake.calls)
	}
	assertStoryFontsDerived(t, fams[:len(fams)-1], true)
	// 取得後不再搜尋。
	fake.want = true
	if err := r.applyStories(v); err != nil || r.asciiScans != 1 {
		t.Fatalf("searched again: %d", r.asciiScans)
	}
}

// §3.4：取得失敗沿用原字型、不中止、不增加重設次數；節流每 60 個回掃最多一次。
func TestLiveStoryAcquisitionMissKeepsFont(t *testing.T) {
	withSyntheticFinder(t)
	base := storyFontBase()
	fake := &fakeStoryFamily{want: true}
	fams := append(storyLiveFamiliesUT(t, base), fake)
	r := &LiveRuntime{resets: map[string]int{}, font: base, stories: fams}
	v := syntheticMemory(false)
	if err := r.applyStories(v); err != nil {
		t.Fatal(err)
	}
	if r.asciiFound || r.asciiScans != 1 || r.asciiStep != 0 || len(r.resets) != 0 {
		t.Fatalf("found=%v scans=%d step=%d resets=%v", r.asciiFound, r.asciiScans, r.asciiStep, r.resets)
	}
	if strings.Join(fake.calls, ",") != "apply" {
		t.Fatalf("calls %v", fake.calls)
	}
	assertStoryFontsDerived(t, fams[:len(fams)-1], false)
	for i := 0; i < 59; i++ {
		fake.want = true
		r.frameSeen++
		if err := r.applyStories(v); err != nil {
			t.Fatal(err)
		}
	}
	if r.asciiScans != 1 {
		t.Fatalf("throttle: %d scans within 60 frames", r.asciiScans)
	}
	fake.want = true
	r.frameSeen++
	if err := r.applyStories(syntheticMemory(true)); err != nil {
		t.Fatal(err)
	}
	if !r.asciiFound || r.asciiScans != 2 || len(r.resets) != 0 {
		t.Fatalf("after 60 frames: found=%v scans=%d resets=%v", r.asciiFound, r.asciiScans, r.resets)
	}
	assertStoryFontsDerived(t, fams[:len(fams)-1], true)
	if !strings.Contains(r.DebugSummary(), "orig-ascii=true/2 orig-ascii-step=4242") {
		t.Fatalf("summary: %s", r.DebugSummary())
	}
}
