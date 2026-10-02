package buckrogers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// Buck repo spec 044 §3.7: the release package, loaded the way the player
// loads it.  BUCKROGERS_PKG_ROOT is the package's common/ directory (text/
// and font/ in it); BUCKROGERS_PKG_LANGS lists the languages it carries
// (default zh-TW zh-CN ja ko).  tools/package.sh runs this test and fails
// the package unless it ran and passed, so a missing CMU dictionary or a
// font without the allowed transliteration characters cannot ship.
func TestPackagedLanes(t *testing.T) {
	root := os.Getenv("BUCKROGERS_PKG_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_PKG_ROOT 未設定：發行包的通道冒煙未檢查")
	}
	langs := strings.Fields(os.Getenv("BUCKROGERS_PKG_LANGS"))
	if len(langs) == 0 {
		langs = []string{LangZhTW, LangZhCN, LangJa, LangKo}
	}
	opts := LiveOptions{
		TextDir:   filepath.Join(root, "text"),
		FontPath:  filepath.Join(root, "font", "buckrogers-unifont.golemfnt"),
		LangFonts: map[string]string{},
	}
	for _, l := range langs {
		if l == LangZhTW {
			continue
		}
		opts.Langs = append(opts.Langs, l)
		opts.LangFonts[l] = filepath.Join(root, "font", "buckrogers-"+l+".golemfnt")
	}
	r, err := LoadLiveRuntimeOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.off) != 0 {
		t.Fatalf("語言停用：%v", r.off)
	}
	// Spec 053 §5.5: the zh-TW 3× presenter of the packaged runtime draws E1.
	if sum := r.DebugSummary(); !strings.Contains(sum, " manual-e1=on") {
		t.Fatalf("zh-TW 手冊 E1 未啟用：%s", sum)
	}
	for _, l := range langs {
		if err := r.SetLanguage(l); err != nil {
			t.Fatalf("%s：%v", l, err)
		}
		sum := r.DebugSummary()
		if strings.Contains(sum, "語言停用") || strings.Contains(sum, "玩家名=off") {
			t.Errorf("%s：DebugSummary 有停用：%s", l, sum)
		}
		i := r.laneIndex(l)
		if i < 0 {
			t.Errorf("%s：沒有通道", l)
			continue
		}
		lane := r.lanes[i]
		for _, p := range lane.manPres {
			if (l != LangZhTW) && p.e1Base != nil {
				t.Errorf("%s：只有 zh-TW 可以啟用手冊 E1", l)
			}
		}
		if lane.players == nil {
			t.Errorf("%s：玩家名未啟用（%s）", l, lane.playersOff)
			continue
		}
		out, ok := lane.players.Chinese("NICOLE STEELE", translit.Female)
		if !ok || out == "" {
			t.Errorf("%s：NICOLE STEELE 沒有音譯結果", l)
		}
		t.Logf("%s：NICOLE STEELE → %s", l, out)
	}
}
