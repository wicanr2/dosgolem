package buckrogers

import (
	"strings"
	"testing"
)

// Buck repo spec 055: the 3× manual presenter of the zh-CN, ja and ko lanes
// draws E1 with the lane's own font and catalog.  These tests load the real
// catalogs and the release fonts (BUCKROGERS_CHT_ROOT and the per-language
// font variables) and skip without them.

var e1LaneInputs = []struct {
	lang   string
	inputs func(t *testing.T) (string, string, string)
}{
	{LangZhCN, zhCNInputs}, {LangJa, jaInputs}, {LangKo, koInputs},
}

// e1LaneRuntime loads a runtime with the zh-TW lane and the named language.
func e1LaneRuntime(t *testing.T, lang string, inputs func(*testing.T) (string, string, string)) (*LiveRuntime, *liveLane) {
	t.Helper()
	text, twFont, font := inputs(t)
	r := loadJkRuntime(t, lang, text, twFont, font)
	i := r.laneIndex(lang)
	if i < 0 {
		t.Fatalf("%s 沒有通道：%v", lang, r.off)
	}
	return r, r.lanes[i]
}

// laneBlock is the DebugSummary block of lane[<lang>] (blocks are cut at
// " lane[": they hold {…} of their own).
func laneBlock(summary, lang string) string {
	for _, b := range strings.Split(summary, " lane[")[1:] {
		if strings.HasPrefix(b, lang+"]=") {
			return b
		}
	}
	return ""
}

func TestManualE1LaneFormalPreflight(t *testing.T) {
	for _, c := range e1LaneInputs {
		c := c
		t.Run(c.lang, func(t *testing.T) {
			r, l := e1LaneRuntime(t, c.lang, c.inputs)
			if l.manE1 != manualE1On || len(l.manE1Rows) != 39 {
				t.Fatalf("status=%q 題數=%d", l.manE1, len(l.manE1Rows))
			}
			for i, scale := range liveScales {
				if on := l.manPres[i].e1Base != nil; on != (scale == 3) {
					t.Errorf("%d×：e1Base=%v", scale, on)
				}
			}
			sum := r.DebugSummary()
			if b := laneBlock(sum, c.lang); !strings.Contains(b, " manual-e1=on") {
				t.Errorf("通道區塊沒有 manual-e1=on：%.100s", b)
			}
			// Only the zh-TW lane has keyword rows (spec 055 §3.3).
			if l.manualE1Active() {
				t.Error("manualE1Active 只屬於 zh-TW 通道")
			}
			same, shorter, longer := 0, 0, 0
			for _, e := range l.manCatalog.byIdentity {
				paragraph, err := manualRows(e.translation, manualEnglishColumns, manualEnglishLastRow+1)
				if err != nil {
					t.Fatal(err)
				}
				k, e1 := manualUsedRows(paragraph), l.manE1Rows[e.eventKey]
				switch {
				case e1 == k:
					same++
				case e1 < k:
					shorter++
				default:
					longer++
				}
			}
			t.Logf("%s：E1 列數與固定格 相同 %d、較少 %d、較多 %d", c.lang, same, shorter, longer)
			if longer != 0 {
				t.Errorf("%d 題的 E1 列數大於固定格", longer)
			}
		})
	}
}

// Each lane's E1 switch leaves 2× alone, differs from the fixed cells only
// inside the manual rectangle, and does differ for some questions.
func TestManualE1LaneFormalScaleAlternation(t *testing.T) {
	for _, c := range e1LaneInputs {
		c := c
		t.Run(c.lang, func(t *testing.T) {
			r, l := e1LaneRuntime(t, c.lang, c.inputs)
			if err := r.SetLanguage(c.lang); err != nil {
				t.Fatal(err)
			}
			p2, p3 := l.manPres[0], l.manPres[1]
			var palette [256][3]uint8
			palette[10] = [3]uint8{85, 255, 85}
			indexed := make([]byte, 320*200)
			r.Frame(indexed, palette)
			apply := func(p *RuntimeManualOverlay, gen uint64, e catalogEntry) {
				if err := p.SetStyle(ManualTextStyle{Background: 0, Foreground: 10}); err != nil {
					t.Fatal(err)
				}
				for _, ev := range []ManualPresentationEvent{
					{Kind: ManualPresentationBegin, Generation: gen},
					{Kind: ManualPresentationRequest, Generation: gen, Request: DisplayRequest{Generation: gen, EventKey: e.eventKey, TextKey: e.textKey}},
				} {
					if err := p.Apply(ev); err != nil {
						t.Fatal(err)
					}
				}
			}
			compose := func(scale int) []byte {
				out, _, err := r.ComposeWith(indexed, palette, scale)
				if err != nil {
					t.Fatal(err)
				}
				return append([]byte(nil), out...)
			}
			differing, same := 0, 0
			gen := uint64(0)
			for _, entry := range l.manCatalog.byIdentity {
				gen++
				apply(p2, gen, entry)
				apply(p3, gen, entry)
				r.Frame(indexed, palette)
				e2, e3 := compose(2), compose(3)
				if string(e3) == string(ScaleIndexedRGBA(indexed, palette, 3)) {
					t.Fatalf("%s：3× 沒有畫出手冊層", entry.textKey)
				}
				saved := p3.e1Base
				p3.e1Base = nil
				gen++
				apply(p3, gen, entry)
				r.Frame(indexed, palette)
				f2, f3 := compose(2), compose(3)
				p3.e1Base = saved
				if string(e2) != string(f2) {
					t.Errorf("%s：E1 開關改變了 2× 位元組", entry.textKey)
				}
				if string(e3) == string(f3) {
					same++
					continue
				}
				differing++
				const w = 960
				for i := 0; i < len(e3); i += 4 {
					if string(e3[i:i+4]) != string(f3[i:i+4]) {
						x, y := (i/4)%w, (i/4)/w
						if x < 21 || x >= 21+915 || y < 216 || y >= 216+336 {
							t.Fatalf("%s：矩形外有差異 (%d,%d)", entry.textKey, x, y)
						}
					}
				}
			}
			t.Logf("%s：E1 與固定格相同 %d 段、不同 %d 段", c.lang, same, differing)
			if differing == 0 {
				t.Error("沒有任何一段與固定格不同（驗收會是真空）")
			}
		})
	}
}

// A broken derived font after the plan was sealed: Compose(3) neither panics
// nor draws the manual layer, and counts the skip (spec 053 §3.2).
func TestManualE1LaneFormalComposeSurvivesBrokenDraw(t *testing.T) {
	for _, c := range e1LaneInputs {
		c := c
		t.Run(c.lang, func(t *testing.T) {
			r, l := e1LaneRuntime(t, c.lang, c.inputs)
			if err := r.SetLanguage(c.lang); err != nil {
				t.Fatal(err)
			}
			p := l.manPres[1]
			var entry catalogEntry
			for _, e := range p.catalog.byIdentity {
				entry = e
				break
			}
			if err := p.SetStyle(ManualTextStyle{Background: 0, Foreground: 10}); err != nil {
				t.Fatal(err)
			}
			for _, ev := range []ManualPresentationEvent{
				{Kind: ManualPresentationBegin, Generation: 1},
				{Kind: ManualPresentationRequest, Generation: 1, Request: DisplayRequest{Generation: 1, EventKey: entry.eventKey, TextKey: entry.textKey}},
			} {
				if err := p.Apply(ev); err != nil {
					t.Fatal(err)
				}
			}
			var palette [256][3]uint8
			palette[10] = [3]uint8{85, 255, 85}
			indexed := make([]byte, 320*200)
			r.Frame(indexed, palette)
			if out, drawn, err := r.ComposeWith(indexed, palette, 3); err != nil || !drawn || len(out) != 960*600*4 {
				t.Fatalf("compose err=%v drawn=%v len=%d", err, drawn, len(out))
			}
			p.font.Name = "changed"
			before := l.skips["manual"]
			if out, _, err := r.ComposeWith(indexed, palette, 3); err != nil || len(out) != 960*600*4 {
				t.Fatalf("compose err=%v len=%d", err, len(out))
			}
			if l.skips["manual"] != before+1 {
				t.Errorf("skips[manual] = %d，應加一", l.skips["manual"])
			}
		})
	}
}
