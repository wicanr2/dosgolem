package buckrogers

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/xlate"
)

// Spec 040 (Buck repo) §5.1 unit tests.  The formal text comes from
// BUCKROGERS_CHT_ROOT; the test-only language zz from BUCKROGERS_ZZ_DIR
// (generated in the Buck workplace, never in text/).  Missing inputs skip.

func liveLangInputs(t *testing.T) (textDir, fontPath string) {
	t.Helper()
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定")
	}
	fontPath = filepath.Join(root, "workplace/current-font/buckrogers-eten-top-pad.golemfnt")
	if _, err := os.Stat(fontPath); err != nil {
		t.Skip("本機工作字型缺席")
	}
	return filepath.Join(root, "text"), fontPath
}

func zzDir(t *testing.T) string {
	t.Helper()
	d := os.Getenv("BUCKROGERS_ZZ_DIR")
	if d == "" {
		t.Skip("BUCKROGERS_ZZ_DIR 未設定")
	}
	return d
}

func loadWithZZ(t *testing.T) *LiveRuntime {
	t.Helper()
	text, font := liveLangInputs(t)
	dir := zzDir(t)
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: font, Langs: []string{LangTest},
		LangDirs: map[string]string{LangTest: dir}, LangFonts: map[string]string{LangTest: font}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := r.LaneResets(LangTest); !ok {
		t.Fatalf("zz 未啟用：%v", r.off)
	}
	return r
}

func TestLangFileNamesDefaultZhTW(t *testing.T) {
	if LangFile("menu", LangZhTW) != "menu.zh-TW.tsv" || LangFile("ecl-text", "ja") != "ecl-text.ja.tsv" {
		t.Fatal("LangFile")
	}
	if LangFontPath("/r", "ko") != filepath.Join("/r", "font", "buckrogers-ko.golemfnt") {
		t.Fatal("LangFontPath")
	}
	for _, c := range []string{"zh-TW", "zh-CN", "en", "ja", "ko", "zz"} {
		if !KnownLang(c) {
			t.Fatalf("%s 應為已知語言", c)
		}
	}
	if KnownLang("fr") || KnownLang("") {
		t.Fatal("未知語言代碼被接受")
	}
	// Default loaders keep the zh-TW names in their messages.
	_, err := LoadSkillExitCatalog(nil, nil, []byte("key\ttranslation\tsource\n"))
	if err == nil || !strings.Contains(err.Error(), "skill-exit-confirmation.zh-TW.tsv") {
		t.Fatalf("zh-TW 檔名：%v", err)
	}
	_, err = LoadSkillExitCatalogLang(nil, nil, []byte("key\ttranslation\n"), "ja")
	if err == nil || !strings.Contains(err.Error(), "skill-exit-confirmation.ja.tsv") {
		t.Fatalf("ja 檔名：%v", err)
	}
}

func TestLiveRuntimeTestLanguageLoads(t *testing.T) {
	r := loadWithZZ(t)
	if r.Language() != LangZhTW || len(r.lanes) != 2 || r.lanes[1].lang != LangTest {
		t.Fatalf("lanes %d lang %s", len(r.lanes), r.Language())
	}
	st := r.Languages()
	if len(st) != 6 || st[5].Code != LangTest || !st[5].Enabled || !st[0].Enabled || !st[2].Enabled || st[1].Enabled {
		t.Fatalf("languages %+v", st)
	}
	// F4 cycle skips languages that are off and never visits zz.
	if n := r.NextLanguage(); n != LangEn {
		t.Fatalf("next %s", n)
	}
	if err := r.SetLanguage(LangEn); err != nil || r.NextLanguage() != LangZhTW {
		t.Fatal("en → zh-TW")
	}
	if err := r.SetLanguage("ja"); err == nil {
		t.Fatal("未啟用語言不得切換")
	}
	if err := r.SetLanguage("fr"); err == nil {
		t.Fatal("未知語言不得切換")
	}
	if err := r.SetLanguage(LangTest); err != nil || r.Language() != LangTest {
		t.Fatal(err)
	}
}

// Spec 040 §3.3: a language whose files fail the load check is off; zh-TW
// failing is a start failure.
func TestLiveRuntimeLanguageLoadFailureIsIsolated(t *testing.T) {
	text, font := liveLangInputs(t)
	src := zzDir(t)
	dir := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if e.Name() == "skill-exit-confirmation.zz.tsv" {
			// A body far wider than the original row: capacity check fails.
			b = []byte("key\ttranslation\tsource\ncareer.skill.exit.exit_confirmation_prompt.001\t" + strings.Repeat("點", 60) + "\truntime-interface\n")
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: font, Langs: []string{LangTest},
		LangDirs: map[string]string{LangTest: dir}, LangFonts: map[string]string{LangTest: font}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := r.LaneResets(LangTest); ok || r.off[LangTest] == "" {
		t.Fatalf("zz 應停用：%v", r.off)
	}
	if !strings.Contains(r.DebugSummary(), "語言停用=zz(") {
		t.Fatal(r.DebugSummary())
	}
	// Missing files and missing fonts are reasons too.
	r, err = LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: font, Langs: []string{"ja", LangTest},
		LangDirs: map[string]string{LangTest: src}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.off["ja"], "缺語言檔") || r.off[LangTest] != "缺字型" {
		t.Fatalf("off %v", r.off)
	}
	// zh-TW failure keeps the runtime from starting.
	zh := t.TempDir()
	tentries, _ := os.ReadDir(text)
	for _, e := range tentries {
		if e.IsDir() {
			continue
		}
		b, _ := os.ReadFile(filepath.Join(text, e.Name()))
		if e.Name() == "skill-exit-confirmation.zh-TW.tsv" {
			b = []byte("key\ttranslation\tsource\ncareer.skill.exit.exit_confirmation_prompt.001\t" + strings.Repeat("點", 60) + "\truntime-interface\ntechnical.skill.exit.exit_confirmation_prompt.001\t離開\truntime-interface\n")
		}
		if err := os.WriteFile(filepath.Join(zh, e.Name()), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if sub, err := os.ReadDir(filepath.Join(text, "cmudict")); err == nil && len(sub) > 0 {
		_ = os.Symlink(filepath.Join(text, "cmudict"), filepath.Join(zh, "cmudict"))
	}
	if _, err := LoadLiveRuntime(zh, font); err == nil {
		t.Fatal("zh-TW 驗證失敗必須是啟動失敗")
	}
}

// Spec 040 §3.1: the shared watchers output keys only; the manual receipt
// keeps translation_runes from the zh-TW catalog.
func TestSharedWatchersOutputKeysOnly(t *testing.T) {
	r := loadWithZZ(t)
	for _, e := range r.menu.watcher.catalog.byIdentity {
		if e.translation != "" {
			t.Fatal("選單共用 recorder 帶了譯文")
		}
	}
	for _, q := range r.action.resolver.byIdentity {
		if q.Translation != "" {
			t.Fatal("動作列共用 watcher 帶了譯文")
		}
	}
	for _, e := range r.manual.catalog.byIdentity {
		if e.translation != "" {
			t.Fatal("手冊共用 watcher 帶了譯文")
		}
	}
	// Observation runes come from the zh-TW text by key.
	var ev, tk string
	for _, e := range r.manCatalog.byIdentity {
		ev, tk = e.eventKey, e.textKey
		break
	}
	zh, _ := r.manCatalog.translationFor(ev, tk)
	runes := r.manual.runes
	got, _ := runes.translationFor(ev, tk)
	if got != zh || zh == "" {
		t.Fatal("translation_runes 來源不是 zh-TW catalog")
	}
}

// Spec 040 §3.1 動作列: zh-TW derives exactly the established colouring;
// another language must keep the same letter in one "(X)".
func TestActionBarStyleDerivation(t *testing.T) {
	for _, zh := range []string{"加點(A)", "減點(S)", "上頁(P)", "下頁(N)", "完成(D)"} {
		st, err := DeriveActionBarNormalStyle(zh, zh, 15, 10)
		if err != nil || !equalUint8s(st.RuneForegrounds, HotkeyPreservingActionBarNormalStyle().RuneForegrounds) {
			t.Fatalf("%s: %v %v", zh, st.RuneForegrounds, err)
		}
	}
	st, err := DeriveActionBarNormalStyle("點(A)", "加點(A)", 15, 10)
	if err != nil || !equalUint8s(st.RuneForegrounds, []uint8{10, 10, 15, 10}) {
		t.Fatalf("zz 推導 %v %v", st.RuneForegrounds, err)
	}
	for _, bad := range []string{"點(B)", "點", "(A)(A)", "點（A）"} {
		if _, err := DeriveActionBarNormalStyle(bad, "加點(A)", 15, 10); err == nil {
			t.Fatalf("%q 應失敗", bad)
		}
	}
}

// Spec 040 §3.1 post_join: zh-TW keeps its hash pin; others 0 or 7 rows.
func TestPostJoinMenuLanguageRows(t *testing.T) {
	text, _ := liveLangInputs(t)
	ev, _ := os.ReadFile(filepath.Join(text, "post-join-menu-events.tsv"))
	va, _ := os.ReadFile(filepath.Join(text, "post-join-menu-variants.tsv"))
	zh, _ := os.ReadFile(filepath.Join(text, "post-join-menu.zh-TW.tsv"))
	if _, err := LoadPostJoinMenuCatalog(ev, va, zh); err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(zh, []byte("刪除角色"), []byte("刪除角"), 1)
	if _, err := LoadPostJoinMenuCatalog(ev, va, changed); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("zh-TW 雜湊釘選：%v", err)
	}
	if c, err := LoadPostJoinMenuCatalogLang(ev, va, changed, "ja"); err != nil || len(c.base.byIdentity) != 7 {
		t.Fatalf("7 列：%v", err)
	}
	empty, err := LoadPostJoinMenuCatalogLang(ev, va, headerOnly(), "ja")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(zh), "\n")
	if _, err := LoadPostJoinMenuCatalogLang(ev, va, []byte(strings.Join(lines[:4], "")), "ja"); err == nil {
		t.Fatal("3 列應失敗")
	}
	// 0 rows: the whole menu is left to the original, yielding its seven cells.
	font := &xlate.Font{W: 16, H: 16, Name: "t", Glyphs: map[rune][]byte{}}
	p, err := NewRuntimePostJoinMenuOverlay(empty, font, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(PostJoinMenuGeneration{Generation: 1, Selected: 0}, [256][3]uint8{}); err != nil {
		t.Fatal(err)
	}
	if len(p.ActiveKeys()) != 0 || len(p.MissingRects()) != 7 || p.Missing != 1 {
		t.Fatalf("keys %v rects %d", p.ActiveKeys(), len(p.MissingRects()))
	}
}

// Spec 040 §3.2 讓位矩形 for the four families when a language lacks the text.
func TestMissingTranslationYieldRects(t *testing.T) {
	r := loadWithZZ(t)
	l := r.lanes[1]
	// Menu presenter: a missing key clears its own rectangle.
	var id menuIdentity
	var e catalogEntry
	for x, y := range r.menuShared.full.byIdentity {
		if y.eventKey == "menu.transition.old.create_new_character" {
			id, e = x, y
		}
	}
	if e.eventKey == "" {
		t.Skip("menu.transition.old.create_new_character 不在 catalog")
	}
	ev := TextEvent{EntryStep: 1, PostCallStep: 2, OriginalLength: id.length, OriginalSHA256: id.hash, Caller: id.caller,
		Background: id.background, Foreground: id.foreground, Row: id.row, Column: id.column}
	req := DisplayRequest{EventKey: e.eventKey, TextKey: e.textKey}
	p := l.menuPres[0]
	if err := p.Apply(ev, req, [256][3]uint8{}); err != nil || len(p.ActiveKeys()) != 1 {
		t.Fatalf("zz 應畫出：%v %v", err, p.ActiveKeys())
	}
	saved := l.menuTexts[e.textKey]
	delete(l.menuTexts, e.textKey)
	defer func() { l.menuTexts[e.textKey] = saved }()
	if err := p.Apply(ev, req, [256][3]uint8{}); err != nil || len(p.ActiveKeys()) != 0 || p.Missing != 1 {
		t.Fatalf("缺譯應清掉自己的矩形：%v %v", err, p.ActiveKeys())
	}
	// Skill exit: the row-24 band.
	o := l.skill[0]
	for _, x := range o.Watcher.c.byIdentity {
		ge := TextEvent{EntryStep: 10, OriginalLength: x.id.length, OriginalSHA256: x.id.hash, Caller: x.id.caller,
			Background: x.id.background, Foreground: x.id.foreground, Row: x.id.row, Column: x.id.column}
		saved := x
		x.text = ""
		o.Watcher.c.byIdentity[x.id] = x
		if err := o.ObserveEntry(ge); err != nil {
			t.Fatal(err)
		}
		ge.PostCallStep = 11
		if err := o.ObserveReturn(ge, [256][3]uint8{}); err != nil {
			t.Fatal(err)
		}
		lo, hi := skillExitBody(x.page)
		if m := o.MissingRect(); m == nil || *m != (PixelRect{0, 192, int(hi - lo), 8}) || !o.Watcher.Active() {
			t.Fatalf("技能離開讓位 %v", m)
		}
		o.Watcher.c.byIdentity[x.id] = saved
		break
	}
	// Body icon: its safe rectangles.
	b, err := NewRuntimeBodyIconOverlay(&BodyIconCatalog{byKey: map[string]BodyIconIdentity{}, save: l.bodyCatalog.save}, l.font, 2, BodyIconLive)
	if err != nil {
		t.Fatal(err)
	}
	b.catalog = &BodyIconCatalog{byKey: map[string]BodyIconIdentity{}, save: l.bodyCatalog.save}
	for k, v := range l.bodyCatalog.byKey {
		if k == "body.icon.confirmation" {
			v.Translation = ""
		}
		b.catalog.byKey[k] = v
	}
	tr := BodyIconTransition{Generation: 1, Group: "confirmation", Events: []BodyIconEvent{{EventKey: "body.icon.confirmation", EntryStep: 1, PostCallStep: 2, Generation: 1, Group: "confirmation"}}}
	if err := b.Apply(tr, [256][3]uint8{}); err != nil {
		t.Fatal(err)
	}
	if want := l.bodyCatalog.byKey["body.icon.confirmation"].Rect; len(b.MissingRects()) != 1 || b.MissingRects()[0] != want || len(b.ActiveKeys()) != 0 {
		t.Fatalf("身體圖示讓位 %v", b.MissingRects())
	}
}

// Spec 040 §3.4: English composes the original byte for byte; the logbook
// keys belong to the game in English.
func TestEnglishComposeIsOriginal(t *testing.T) {
	r := loadWithZZ(t)
	indexed := make([]byte, 320*200)
	var pal [256][3]uint8
	for i := range indexed {
		indexed[i] = uint8(i * 7)
	}
	for i := range pal {
		pal[i] = [3]uint8{uint8(i), uint8(255 - i), uint8(i / 2)}
	}
	r.Frame(indexed, pal)
	if err := r.SetLanguage(LangEn); err != nil {
		t.Fatal(err)
	}
	for _, scale := range []int{2, 3} {
		got, ok, err := r.Compose(scale)
		if err != nil || !ok || !bytes.Equal(got, ScaleIndexedRGBA(indexed, pal, scale)) {
			t.Fatalf("%d×：英文模式必須等於原版", scale)
		}
	}
	for _, l := range r.lanes {
		if l.logbook != nil {
			l.logbook.open = 1
		}
	}
	if r.LogbookTurn(1) {
		t.Fatal("英文模式 LogbookTurn 應回 false")
	}
}

// Spec 040 §3.4 手札翻頁: the composed language decides; each lane turns
// within its own pages.
func TestLogbookTurnClampsPerLane(t *testing.T) {
	mk := func(pages int) *LogbookWatcher {
		c := &LogbookCatalog{entries: map[int]LogbookEntry{3: {Pages: make([][]string, pages)}}}
		w := NewLogbookWatcher(c, nil)
		w.open = 3
		return w
	}
	a, b := mk(3), mk(1)
	r := &LiveRuntime{lanes: []*liveLane{{lang: LangZhTW, logbook: a}, {lang: LangTest, logbook: b}}}
	if !r.LogbookTurn(1) || a.page != 1 || b.page != 0 {
		t.Fatalf("pages %d %d", a.page, b.page)
	}
	r.cur = 1
	if !r.LogbookTurn(1) || a.page != 2 || b.page != 0 {
		t.Fatalf("pages %d %d", a.page, b.page)
	}
	b.open = 0
	if r.LogbookTurn(-1) || a.page != 2 {
		t.Fatal("目前語言面板未開啟時任何語言都不翻頁")
	}
	r.cur = -1
	if r.LogbookTurn(-1) {
		t.Fatal("英文模式不吃鍵")
	}
}

// Spec 040 §3.2: a draw-time missing glyph skips the layer and counts; the
// menu family then uses the original frame as its base.
func TestComposeMissingGlyphSkipsLayer(t *testing.T) {
	r := loadWithZZ(t)
	l := r.lanes[1]
	indexed := make([]byte, 320*200)
	var pal [256][3]uint8
	r.Frame(indexed, pal)
	if err := r.SetLanguage(LangTest); err != nil {
		t.Fatal(err)
	}
	// Put a stamp with a glyph the font lacks into the zz menu layer.
	empty := &xlate.Font{W: 16, H: 16, Name: "empty", Glyphs: map[rune][]byte{}}
	l.menuPres[0].layer.Add(&xlate.Stamp{Key: "x#0", X: 0, Y: 0, Cells: 1, CellW: 8, CellH: 8, Font: empty, Text: []rune{'點'}, State: xlate.Shown})
	got, ok, err := r.Compose(2)
	if err != nil || !ok || !bytes.Equal(got, ScaleIndexedRGBA(indexed, pal, 2)) || l.skips["menu"] != 1 {
		t.Fatalf("缺字底圖：err=%v skips=%v", err, l.skips)
	}
	// zh-TW is untouched by zz's state.
	if err := r.SetLanguage(LangZhTW); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.Compose(2); err != nil || !ok || len(r.lanes[0].skips) != 0 {
		t.Fatalf("zh-TW 受影響：%v %v", err, r.lanes[0].skips)
	}
}

// Spec 040 §3.2: B-family sync runs for every lane at both time points, and
// a missing glyph in one lane only resets that lane.
func TestBSyncAllLanesAndMissingGlyphIsolated(t *testing.T) {
	r := loadWithZZ(t)
	if r.lanes[0].hmenu == nil || r.lanes[1].hmenu == nil {
		t.Skip("hmenu catalog 缺席")
	}
	for _, l := range r.lanes {
		l.hmenu.gen++ // pretend a new generation with no page
	}
	var pal [256][3]uint8
	r.Frame(make([]byte, 320*200), pal)
	for _, l := range r.lanes {
		if l.hmenuGen != l.hmenu.Generation() {
			t.Fatalf("%s 未在 retrace 同步", l.lang)
		}
		l.hmenu.gen++
	}
	if err := r.SetLanguage(LangEn); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.ComposeWith(make([]byte, 320*200), pal, 2); err != nil {
		t.Fatal(err)
	}
	for _, l := range r.lanes {
		if l.hmenuGen != l.hmenu.Generation() {
			t.Fatalf("%s 未在合成前同步（英文模式也要）", l.lang)
		}
	}
	// A zz page the font cannot draw: only zz sees the discontinuity.
	empty := &xlate.Font{W: 16, H: 16, Name: "empty", Glyphs: map[rune][]byte{}}
	zz := r.lanes[1]
	for i, scale := range liveScales {
		var err error
		if zz.hmenuPres[i], err = NewHMenuOverlay(empty, scale); err != nil {
			t.Fatal(err)
		}
	}
	page := &HMenuPage{Rows: []HMenuRow{newHMenuRow(3, 1, []HMenuCell{{Rune: '點', FG: 15}})}}
	for _, l := range r.lanes {
		l.hmenu.page, l.hmenu.gen = page, l.hmenu.gen+1
	}
	r.Frame(make([]byte, 320*200), pal)
	if zz.resets["hmenu"] != 1 || zz.hmenu.Page() != nil || r.lanes[0].resets["hmenu"] != 0 || r.lanes[0].hmenu.Page() == nil {
		t.Fatalf("zz=%v zh=%v", zz.resets, r.lanes[0].resets)
	}
}

// Spec 040 §3.3: a lane's runtime error rebuilds only that lane's family,
// never the shared recorder, and another lane keeps drawing.
func TestRuntimeLaneErrorIsolated(t *testing.T) {
	r := loadWithZZ(t)
	zz, zh := r.lanes[1], r.lanes[0]
	bad := [4]uint8{30, 1, 0, 0} // bottom ≥ 25: presenter error
	for _, l := range r.lanes {
		l.menuClear(bad)
	}
	if zz.resets["menu"] != 1 || zh.resets["menu"] != 1 || r.menu.fault != nil {
		t.Fatalf("menu resets zz=%v zh=%v fault=%v", zz.resets, zh.resets, r.menu.fault)
	}
	// Post-join: a zz presenter failure rebuilds only zz.
	zz.postPres[1] = &RuntimePostJoinMenuOverlay{layer: &xlate.Layer{W: 320, H: 200}, c: zz.postCatalog, font: &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}, scale: 3, active: map[string]bool{}}
	for _, l := range r.lanes {
		if err := l.postApply(PostJoinMenuGeneration{Generation: 1, Selected: 0}, [256][3]uint8{}); err != nil {
			t.Fatal(err)
		}
	}
	if zz.resets["post-join"] != 1 || zh.resets["post-join"] != 0 || len(zh.postPres[0].ActiveKeys()) != 7 || r.resets["post-join"] != 0 {
		t.Fatalf("post-join zz=%v zh=%v shared=%v", zz.resets, zh.resets, r.resets)
	}
}

// Spec 040 §3.2: the composed language after a switch equals a runtime that
// used it from the start, when both saw the same observations (here: the
// same menu request and frame).
func TestSwitchEqualsStartedInLanguage(t *testing.T) {
	a := loadWithZZ(t)
	b := loadWithZZ(t)
	if err := b.SetLanguage(LangTest); err != nil {
		t.Fatal(err)
	}
	var id menuIdentity
	var e catalogEntry
	for x, y := range a.menuShared.full.byIdentity {
		if y.eventKey == "menu.transition.old.create_new_character" {
			id, e = x, y
		}
	}
	if e.eventKey == "" {
		t.Skip("menu.transition.old.create_new_character 不在 catalog")
	}
	ev := TextEvent{EntryStep: 1, PostCallStep: 2, OriginalLength: id.length, OriginalSHA256: id.hash, Caller: id.caller,
		Background: id.background, Foreground: id.foreground, Row: id.row, Column: id.column}
	req := DisplayRequest{EventKey: e.eventKey, TextKey: e.textKey}
	var pal [256][3]uint8
	for i := range pal {
		pal[i] = [3]uint8{uint8(i), uint8(i), uint8(i)}
	}
	indexed := make([]byte, 320*200)
	for i := range indexed {
		indexed[i] = uint8(i % 16)
	}
	for _, r := range []*LiveRuntime{a, b} {
		for _, l := range r.lanes {
			l.menuApply(ev, req, pal)
		}
		r.Frame(indexed, pal)
		r.Frame(indexed, pal)
	}
	if err := a.SetLanguage(LangTest); err != nil {
		t.Fatal(err)
	}
	for _, scale := range []int{2, 3} {
		x, _, err1 := a.Compose(scale)
		y, _, err2 := b.Compose(scale)
		if err1 != nil || err2 != nil || !bytes.Equal(x, y) || bytes.Equal(x, ScaleIndexedRGBA(indexed, pal, scale)) {
			t.Fatalf("%d×：切換後畫面不同或沒有疊字", scale)
		}
	}
}

// Spec 040 §3.2: the post-join prompt yields the original cell range of an
// untranslated body (TextEvent Row／Column／OriginalLength).
func TestExitPromptMissingYieldsOriginalCells(t *testing.T) {
	r := loadWithZZ(t)
	o := r.lanes[1].exit[0]
	for id, x := range o.Watcher.c.byIdentity {
		if x.key != postJoinExitQ1 {
			continue
		}
		x.text = ""
		o.Watcher.c.byIdentity[id] = x
		e := TextEvent{EntryStep: 5, OriginalLength: id.length, OriginalSHA256: id.hash, Caller: id.caller,
			Background: id.background, Foreground: id.foreground, Row: id.row, Column: id.column}
		if err := o.ObserveEntry(e); err != nil {
			t.Fatal(err)
		}
		e.PostCallStep = 9
		if err := o.ObserveReturn(e, [256][3]uint8{}); err != nil {
			t.Fatal(err)
		}
		want := PixelRect{int(id.column) * 8, int(id.row) * 8, int(id.length) * 8, 8}
		if m := o.MissingRect(); m == nil || *m != want || !o.Watcher.Active() || len(o.Presenter.ActiveKeys()) != 0 {
			t.Fatalf("加入後提示讓位 %v", m)
		}
		// The watcher still tracks the body: the original writer clears it.
		if err := o.Prewrite(machine.VideoWrite{Step: 6, CS: 0x0763, IP: 0x184D, Offset: 192 * 320}); err != nil || o.Watcher.Active() {
			t.Fatalf("prewrite %v", err)
		}
		return
	}
	t.Fatal("q1 不在 catalog")
}

// Spec 040 §3.3: an untranslated manual paragraph is accepted and cleared,
// so the consumer never retries it.
func TestManualMissingParagraphIsConsumed(t *testing.T) {
	r := loadWithZZ(t)
	l := r.lanes[1]
	var ev, tk string
	for id, e := range l.manCatalog.byIdentity {
		ev, tk = e.eventKey, e.textKey
		e.translation = ""
		l.manCatalog.byIdentity[id] = e
		break
	}
	p := l.manPres[0]
	c, err := NewManualPresentationConsumer(p)
	if err != nil {
		t.Fatal(err)
	}
	events := []ManualPresentationEvent{
		{Step: 1, Kind: ManualPresentationBegin, Generation: 1},
		{Step: 2, Kind: ManualPresentationRequest, Generation: 1, Request: DisplayRequest{Generation: 1, EventKey: ev, TextKey: tk}},
		{Step: 3, Kind: ManualPresentationClear, Generation: 1},
	}
	if n, err := c.Consume(events); err != nil || n != 3 || p.Missing != 1 || len(p.ActiveKeys()) != 0 {
		t.Fatalf("consumed %d err %v missing %d", n, err, p.Missing)
	}
}

// Spec 040 §5.5 start-up time: load and validate zh-TW alone and with zz.
func BenchmarkLiveLoad(b *testing.B) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	zz := os.Getenv("BUCKROGERS_ZZ_DIR")
	if root == "" || zz == "" {
		b.Skip("BUCKROGERS_CHT_ROOT／BUCKROGERS_ZZ_DIR 未設定")
	}
	text := filepath.Join(root, "text")
	font := filepath.Join(root, "workplace/current-font/buckrogers-eten-top-pad.golemfnt")
	for _, c := range []struct {
		name  string
		langs []string
	}{{"zh-TW", nil}, {"zh-TW+zz", []string{LangTest}}} {
		b.Run(c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: font, Langs: c.langs,
					LangDirs: map[string]string{LangTest: zz}, LangFonts: map[string]string{LangTest: font}}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
