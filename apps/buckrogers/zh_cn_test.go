package buckrogers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate/translit"
)

// Buck repo spec 041 (zh-CN): the per-language glossary files, the
// character-map wrapper of the player-name transliterator and the zh-CN
// lane on the formal generated files.

type fixedTranslit map[string]string

func (f fixedTranslit) Transliterate(name string, _ translit.Gender) (string, translit.Tier, bool) {
	zh, ok := f[name]
	return zh, translit.TierDict, ok
}

func TestCharMapTransliterator(t *testing.T) {
	inner := fixedTranslit{"FLAVIUS": "弗拉維烏斯", "ANNE-MARIE": "安妮-瑪麗", "JO BO": "喬•博", "ODD": "奇字"}
	m := map[rune]rune{'弗': '弗', '拉': '拉', '維': '维', '烏': '乌', '斯': '斯', '安': '安', '妮': '妮', '瑪': '玛', '麗': '丽', '喬': '乔', '博': '博'}
	c := &charMapTransliterator{inner: inner, chars: m}
	for name, want := range map[string]string{"FLAVIUS": "弗拉维乌斯", "ANNE-MARIE": "安妮-玛丽", "JO BO": "乔•博"} {
		got, _, ok := c.Transliterate(name, translit.Male)
		if !ok || got != want {
			t.Errorf("%s = %q %v, want %q", name, got, ok, want)
		}
	}
	// A character outside the map: the name stays English.
	if got, tier, ok := c.Transliterate("ODD", translit.Male); ok || got != "" || tier != translit.TierNone {
		t.Errorf("表外字元應失敗：%q %v %v", got, tier, ok)
	}
	if _, _, ok := c.Transliterate("NOBODY", translit.Male); ok {
		t.Error("內層失敗應失敗")
	}
	p := NewPlayerNames(c, nil)
	if zh, ok := p.Chinese("FLAVIUS", translit.Male); !ok || zh != "弗拉维乌斯" {
		t.Errorf("PlayerNames = %q %v", zh, ok)
	}
}

func TestLoadTranslitCharMap(t *testing.T) {
	good := "tw\tcn\n維\t维\n烏\t乌\n"
	m, err := LoadTranslitCharMap("m", []byte(good))
	if err != nil || m['維'] != '维' || len(m) != 2 {
		t.Fatalf("%v %v", m, err)
	}
	for name, bad := range map[string]string{
		"header":    "a\tb\n維\t维\n",
		"multi":     "tw\tcn\n維\t维乌\n",
		"collision": "tw\tcn\n髮\t发\n發\t发\n",
		"dup":       "tw\tcn\n維\t维\n維\t维\n",
		"empty":     "tw\tcn\n",
		"columns":   "tw\tcn\n維\n",
	} {
		if _, err := LoadTranslitCharMap("m", []byte(bad)); err == nil {
			t.Errorf("%s: 應失敗", name)
		}
	}
}

func TestNameGlossaryLangFiles(t *testing.T) {
	text, lang := t.TempDir(), t.TempDir()
	hdr := "english\tenglish_mixed\tchinese\tkind\tperson\tbasis\tnote\n"
	write := func(dir, name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(text, "name-glossary.tsv", hdr+"JIM\tJim\t吉姆\tshort\tjim\txinhua\t\n")
	write(text, "name-glossary-exclude.tsv", "phrase\tscope\tnote\n吉姆博火箭酒吧\t*\tx\n")
	write(lang, "name-glossary.zh-CN.tsv", hdr+"JIM\tJim\t吉姆\tshort\tjim\txinhua\t\n")
	write(lang, "name-glossary-exclude.zh-CN.tsv", "phrase\tscope\tnote\n吉姆博火箭酒吧\t*\tx\n")
	g, err := loadNameGlossaryLang(text, lang, LangZhCN)
	if err != nil || g == nil {
		t.Fatal(err)
	}
	// The zh-CN exclusion phrase comes from the language directory.
	if n := len(g.Matches([]rune("吉姆博火箭酒吧"), "hmenu.x")); n != 0 {
		t.Errorf("例外片語應排除，得 %d 個命中", n)
	}
	if n := len(g.Matches([]rune("吉姆说"), "hmenu.x")); n != 1 {
		t.Errorf("吉姆 應命中，得 %d", n)
	}
	// Without the language's exclusion file nothing is excluded (the zh-TW
	// file is not borrowed).
	os.Remove(filepath.Join(lang, "name-glossary-exclude.zh-CN.tsv"))
	g, err = loadNameGlossaryLang(text, lang, LangZhCN)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(g.Matches([]rune("吉姆博火箭酒吧"), "hmenu.x")); n != 1 {
		t.Errorf("無例外表時應命中，得 %d", n)
	}
}

// zhCNInputs returns the formal text directory and the two fonts, or skips.
func zhCNInputs(t *testing.T) (text, twFont, cnFont string) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	cnFont = os.Getenv("BUCKROGERS_ZHCN_FONT")
	twFont = os.Getenv("BUCKROGERS_ZHTW_FONT")
	if root == "" || cnFont == "" || twFont == "" {
		t.Skip("BUCKROGERS_CHT_ROOT／BUCKROGERS_ZHCN_FONT／BUCKROGERS_ZHTW_FONT 未設定")
	}
	text = filepath.Join(root, "text")
	if _, err := os.Stat(filepath.Join(text, LangFile("menu", LangZhCN))); err != nil {
		t.Skip("text/ 沒有 zh-CN 產生檔")
	}
	return
}

func TestZhCNLaneOnFormalText(t *testing.T) {
	text, twFont, cnFont := zhCNInputs(t)
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: twFont, Langs: []string{LangZhCN},
		LangFonts: map[string]string{LangZhCN: cnFont}})
	if err != nil {
		t.Fatal(err)
	}
	var cycle []string
	for _, s := range r.Languages() {
		if s.Enabled {
			cycle = append(cycle, s.Code)
		}
	}
	if strings.Join(cycle, ",") != "zh-TW,zh-CN,en" {
		t.Fatalf("F4 循環 = %v（停用：%v）", cycle, r.off)
	}
	if err := r.SetLanguage(LangZhCN); err != nil {
		t.Fatal(err)
	}
	sum := r.DebugSummary()
	t.Logf("DebugSummary: %s", sum)
	if strings.Contains(sum, "語言停用") || strings.Contains(sum, "玩家名=off") || strings.Contains(sum, "rebuilds=") {
		t.Errorf("zh-CN 不應有停用、玩家名停用或重建：%s", sum)
	}
	if resets, skips, ok := r.LaneResets(LangZhCN); !ok || len(resets) != 0 || len(skips) != 0 {
		t.Errorf("zh-CN resets=%v skips=%v ok=%v", resets, skips, ok)
	}
	l := r.lanes[r.laneIndex(LangZhCN)]
	if l.players == nil {
		t.Fatalf("zh-CN 玩家名未啟用：%s", l.playersOff)
	}
	if zh, ok := l.players.Chinese("FLAVIUS", translit.Male); !ok || zh != "弗拉维乌斯" {
		t.Errorf("FLAVIUS = %q %v，預期 弗拉维乌斯", zh, ok)
	}
	if zh, ok := l.players.Chinese("BUCK", translit.Male); !ok || zh != "巴克" {
		t.Errorf("BUCK = %q %v（名字表優先）", zh, ok)
	}
	tw := r.lanes[0]
	if zh, ok := tw.players.Chinese("FLAVIUS", translit.Male); !ok || zh != "弗拉維烏斯" {
		t.Errorf("zh-TW FLAVIUS = %q %v", zh, ok)
	}
}

// A zh-CN character map that fails to load turns only zh-CN player names off.
func TestZhCNMapMissingOnlyDisablesPlayerNames(t *testing.T) {
	text, twFont, cnFont := zhCNInputs(t)
	dir := t.TempDir()
	entries, err := os.ReadDir(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == TranslitMapFile(LangZhCN) {
			continue
		}
		if err := os.Symlink(filepath.Join(text, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			t.Fatal(err)
		}
	}
	for _, sub := range []string{"cmudict"} {
		if _, err := os.Stat(filepath.Join(text, sub)); err == nil {
			os.Remove(filepath.Join(dir, sub))
			if err := os.Symlink(filepath.Join(text, sub), filepath.Join(dir, sub)); err != nil {
				t.Fatal(err)
			}
		}
	}
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: dir, FontPath: twFont, Langs: []string{LangZhCN},
		LangFonts: map[string]string{LangZhCN: cnFont}})
	if err != nil {
		t.Fatal(err)
	}
	i := r.laneIndex(LangZhCN)
	if i < 0 {
		t.Fatalf("zh-CN 應仍啟用：%v", r.off)
	}
	if l := r.lanes[i]; l.players != nil || !strings.HasPrefix(l.playersOff, "translit-map:") {
		t.Errorf("players=%v off=%q", l.players, l.playersOff)
	}
	if r.lanes[0].players == nil {
		t.Error("zh-TW 玩家名不應受影響")
	}
}

// Spec 041 §5.4: the rows that grow in zh-CN (「呎→英尺」 twice,
// 「新檔名→新文件名」, 「執行檔→可执行文件」) stay within the cells of the
// English they replace (2 half units per original character), so a
// horizontal-menu row cannot get wider than the English row it overlays;
// the hmenu item is also laid out at the right-most column the English
// "feet" fits in.
func TestZhCNWidenedRowsFitEnglishCells(t *testing.T) {
	text, _, _ := zhCNInputs(t)
	read := func(name string) map[string][]string {
		b, err := os.ReadFile(filepath.Join(text, name))
		if err != nil {
			t.Fatal(err)
		}
		out := map[string][]string{}
		for _, l := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")[1:] {
			f := strings.Split(l, "\t")
			out[f[0]] = f
		}
		return out
	}
	units := func(s string) int {
		n := 0
		for _, r := range s {
			n += runeUnits(r)
		}
		return n
	}
	for fam, keys := range map[string][]string{
		"engine-fragment": {"frag.2260006382e7", "frag.5250b5de07b3", "frag.4844cec572c9"},
		"hmenu":           {"hmenu.a5add1365529"},
	} {
		events := read(map[string]string{"engine-fragment": "engine-fragment-events.tsv", "hmenu": "hmenu-item-events.tsv"}[fam])
		tw, cn := read(LangFile(fam, LangZhTW)), read(LangFile(fam, LangZhCN))
		for _, k := range keys {
			var orig int
			if _, err := fmt.Sscan(events[k][1], &orig); err != nil {
				t.Fatal(err)
			}
			u := units(cn[k][1])
			if u <= units(tw[k][1]) || u > 2*orig {
				t.Errorf("%s: zh-CN %d 單位（zh-TW %d），英文 %d 格＝%d 單位", k, u, units(tw[k][1]), orig, 2*orig)
			}
		}
	}
	ev, err := os.ReadFile(filepath.Join(text, "hmenu-item-events.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	tr, err := os.ReadFile(filepath.Join(text, LangFile("hmenu", LangZhCN)))
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadHMenuCatalogLang(ev, tr, LangZhCN)
	if err != nil {
		t.Fatal(err)
	}
	w := NewHMenuWatcher(c)
	page, why := w.build(HMenuEntry{Text: []byte("feet"), Row: 24, Col: 36, Items: [][2]uint8{{1, 4}}, Normal: 15, Hot: 14})
	if page == nil {
		t.Fatalf("「feet」在第 36 欄放不下：%s", why)
	}
	if got := page.Rows[0].Units(); got != 8 {
		t.Errorf("列寬 %d 單位，預期補滿 8", got)
	}
}
