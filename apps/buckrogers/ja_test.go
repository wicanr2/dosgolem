package buckrogers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Buck repo spec 042: the ja lane on the formal files.  The inputs are the
// Buck repo root (text/*.ja.tsv) and two fonts; the test skips without them.

func jaInputs(t *testing.T) (text, twFont, jaFont string) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	jaFont = os.Getenv("BUCKROGERS_JA_FONT")
	twFont = os.Getenv("BUCKROGERS_ZHTW_FONT")
	if root == "" || jaFont == "" || twFont == "" {
		t.Skip("BUCKROGERS_CHT_ROOT／BUCKROGERS_JA_FONT／BUCKROGERS_ZHTW_FONT 未設定")
	}
	text = filepath.Join(root, "text")
	if _, err := os.Stat(filepath.Join(text, LangFile("menu", LangJa))); err != nil {
		t.Skip("text/ 沒有 ja 檔")
	}
	return
}

func TestJaLaneOnFormalText(t *testing.T) {
	text, twFont, jaFont := jaInputs(t)
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: twFont, Langs: []string{LangJa},
		LangFonts: map[string]string{LangJa: jaFont}})
	if err != nil {
		t.Fatal(err)
	}
	var cycle []string
	for _, s := range r.Languages() {
		if s.Enabled {
			cycle = append(cycle, s.Code)
		}
	}
	if strings.Join(cycle, ",") != "zh-TW,en,ja" && strings.Join(cycle, ",") != "zh-TW,ja,en" {
		t.Fatalf("F4 循環 = %v（停用：%v）", cycle, r.off)
	}
	if err := r.SetLanguage(LangJa); err != nil {
		t.Fatal(err)
	}
	sum := r.DebugSummary()
	t.Logf("DebugSummary: %s", sum)
	if strings.Contains(sum, "語言停用") || strings.Contains(sum, "rebuilds=") {
		t.Errorf("ja 不應有停用或重建：%s", sum)
	}
	if resets, skips, ok := r.LaneResets(LangJa); !ok || len(resets) != 0 || len(skips) != 0 {
		t.Errorf("ja resets=%v skips=%v ok=%v", resets, skips, ok)
	}
	l := r.lanes[r.laneIndex(LangJa)]
	// Spec 042 §3.8: without the ja transliterator (spec 044) the player names are English.
	if l.players != nil || l.playersOff != "no-transliterator" {
		t.Errorf("ja 玩家名應停用（no-transliterator）：players=%v off=%q", l.players != nil, l.playersOff)
	}
	if l.ecl == nil || l.ecl.layout != layoutJa {
		t.Error("ja 的 ECL watcher 應使用日文排版設定")
	}
	if l.engDisp == nil || l.engDisp.layout != layoutJa {
		t.Error("ja 的引擎片段 watcher 應使用日文排版設定")
	}
	tw := r.lanes[0]
	if tw.ecl.layout != nil || tw.engDisp.layout != nil {
		t.Error("zh-TW 不應有排版設定")
	}
}

// Spec 042 §3.5 static pre-check (a report, not a gate): every ECL row laid
// out in the standard narrative window (spec 036 name-scan window) with the
// ja profile versus the default profile.
func TestJaEclStaticPrecheck(t *testing.T) {
	text, twFont, jaFont := jaInputs(t)
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: twFont, Langs: []string{LangJa},
		LangFonts: map[string]string{LangJa: jaFont}})
	if err != nil {
		t.Fatal(err)
	}
	l := r.lanes[r.laneIndex(LangJa)]
	var rows, overJa, overDefault, startsJa, startsDefault int
	for _, key := range l.ecl.catalog.Keys() {
		s := []rune(l.ecl.catalog.Text(key))
		rows++
		for i, prof := range []*LayoutProfile{layoutJa, nil} {
			lines, _, _, ok := layoutEclTextP(prof, s, nil, 17, eclUnitLeft(1), eclUnitLeft(1), eclUnitRight(38), 22)
			if !ok {
				if i == 0 {
					overJa++
				} else {
					overDefault++
				}
				continue
			}
			for _, ln := range lines[min(1, len(lines)):] {
				if len(ln.Text) > 0 && (isEclClosing(ln.Text[0]) || (prof == nil && layoutJa.closing(ln.Text[0]))) {
					if i == 0 {
						startsJa++
					} else {
						startsDefault++
					}
				}
			}
		}
	}
	t.Logf("ECL %d 列：標準窗溢出 ja %d／預設 %d；列首禁則字元 ja %d／預設 %d", rows, overJa, overDefault, startsJa, startsDefault)
	if startsJa != 0 {
		t.Errorf("ja 排版後仍有 %d 行以禁則字元開頭", startsJa)
	}
}
