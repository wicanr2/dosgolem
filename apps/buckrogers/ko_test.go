package buckrogers

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Buck repo spec 043: the ko lane on the formal files.  The inputs are the
// Buck repo root (text/*.ko.tsv) and two fonts; every test here skips with
// its reason when they are missing and prints the row counts it covered
// when they are there (a test that covers nothing must not turn green
// silently).

func koInputs(t *testing.T) (text, twFont, koFont string) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	koFont = os.Getenv("BUCKROGERS_KO_FONT")
	twFont = os.Getenv("BUCKROGERS_ZHTW_FONT")
	if root == "" || koFont == "" || twFont == "" {
		t.Skip("BUCKROGERS_CHT_ROOT／BUCKROGERS_KO_FONT／BUCKROGERS_ZHTW_FONT 未設定")
	}
	text = filepath.Join(root, "text")
	if _, err := os.Stat(filepath.Join(text, LangFile("menu", LangKo))); err != nil {
		t.Skip("text/ 沒有 ko 檔")
	}
	return
}

func koRuntime(t *testing.T) (*LiveRuntime, *liveLane) {
	text, twFont, koFont := koInputs(t)
	r, err := LoadLiveRuntimeOptions(LiveOptions{TextDir: text, FontPath: twFont, Langs: []string{LangKo},
		LangFonts: map[string]string{LangKo: koFont}})
	if err != nil {
		t.Fatal(err)
	}
	return r, r.lanes[r.laneIndex(LangKo)]
}

func TestKoLaneOnFormalText(t *testing.T) {
	r, l := koRuntime(t)
	var cycle []string
	for _, s := range r.Languages() {
		if s.Enabled {
			cycle = append(cycle, s.Code)
		}
	}
	sort.Strings(cycle)
	if strings.Join(cycle, ",") != "en,ko,zh-TW" {
		t.Fatalf("F4 循環 = %v（停用：%v）", cycle, r.off)
	}
	if err := r.SetLanguage(LangKo); err != nil {
		t.Fatal(err)
	}
	sum := r.DebugSummary()
	t.Logf("DebugSummary: %s", sum)
	if strings.Contains(sum, "語言停用") || strings.Contains(sum, "rebuilds=") || !strings.Contains(sum, "lane[ko]") {
		t.Errorf("ko 不應有停用或重建，且應有 lane[ko]：%s", sum)
	}
	if resets, skips, ok := r.LaneResets(LangKo); !ok || len(resets) != 0 || len(skips) != 0 {
		t.Errorf("ko resets=%v skips=%v ok=%v", resets, skips, ok)
	}
	// Spec 045: the ko player names are on (translit_jk_lane_test.go has the negatives).
	if l.players == nil || l.playersOff != "" {
		t.Errorf("ko 玩家名應啟用：players=%v off=%q", l.players != nil, l.playersOff)
	}
	if l.names == nil {
		t.Error("ko 的名字表未載入")
	}
	if l.ecl == nil || l.ecl.layout != layoutKo {
		t.Error("ko 的 ECL watcher 應使用韓文排版設定")
	}
	if l.engDisp == nil || l.engDisp.layout != layoutKo {
		t.Error("ko 的引擎片段 watcher 應使用韓文排版設定")
	}
	if l.logbook == nil {
		t.Error("ko 的手札未載入")
	}
	if tw := r.lanes[0]; tw.ecl.layout != nil || tw.engDisp.layout != nil {
		t.Error("zh-TW 不應有排版設定")
	}
}

// midWordBreaks counts the line ends of a laid-out call that fall inside a
// word: the next line continues with a non-space after a non-space.
func midWordBreaks(t *testing.T, text []rune, lines []EclTextLine) int {
	t.Helper()
	pos, n := 0, 0
	for i, ln := range lines {
		for i > 0 && pos < len(text) && text[pos] == ' ' {
			pos++
		}
		if pos+len(ln.Text) > len(text) || string(text[pos:pos+len(ln.Text)]) != string(ln.Text) {
			t.Fatalf("排版結果不是原文的連續片段：%q 行 %d %q", string(text), i, string(ln.Text))
		}
		pos += len(ln.Text)
		if i < len(lines)-1 && pos < len(text) && text[pos-1] != ' ' && text[pos] != ' ' {
			n++
		}
	}
	return n
}

// Spec 043 §3.5 static pre-check (a report; the hard lines are the
// invariant of §3.4 and zero overflow at the standard windows): every ECL
// row, plain and with the first (fullest) name annotation, laid out in
// windows of 76, 56, 40 and 30 half units and six rows.
func TestKoEclStaticPrecheck(t *testing.T) {
	_, l := koRuntime(t)
	keys := l.ecl.catalog.Keys()
	if len(keys) != 2542 {
		t.Fatalf("ECL 列數 = %d，應為 2542", len(keys))
	}
	text, _, _ := koInputs(t)
	zhBytes, err := os.ReadFile(filepath.Join(text, LangFile("ecl-text", LangZhTW)))
	if err != nil {
		t.Fatal(err)
	}
	zhRows, err := ReadCatalogRows("ecl-text.zh-TW.tsv", zhBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []int{76, 56, 40, 30} {
		var st struct {
			calls, overKo, overChar, koOnly               int
			overKoPlain, overZhTW                         int
			linesKo, linesChar, midKo, midChar, fallbacks int
			longWords, extraLines                         int
		}
		for _, key := range keys {
			plain := AnnotatedText{Tier: NameTierNone, Text: []rune(l.ecl.catalog.Text(key))}
			variants := []AnnotatedText{plain}
			if v := l.names.Variants(string(plain.Text), key, NameCaseUpper); len(v) > 0 && len(v[0].Units) > 0 {
				variants = append(variants, v[0])
			}
			if _, _, _, ok := layoutEclTextP(nil, []rune(zhRows[key]), nil, 17, 0, 0, uint8(w-1), 22); !ok {
				st.overZhTW++
			}
			if _, _, _, ok := layoutEclTextP(layoutKo, plain.Text, nil, 17, 0, 0, uint8(w-1), 22); !ok {
				st.overKoPlain++
			}
			for _, a := range variants {
				st.calls++
				ko, _, _, okKo := layoutEclTextP(layoutKo, a.Text, a.Units, 17, 0, 0, uint8(w-1), 22)
				ch, _, _, okCh := layoutEclTextP(layoutKoChars, a.Text, a.Units, 17, 0, 0, uint8(w-1), 22)
				if !okKo {
					st.overKo++
				}
				if !okCh {
					st.overChar++
				}
				if okKo != okCh {
					if okKo {
						st.koOnly++
					}
					t.Errorf("窗寬 %d %s：ko 與字級的 fits 不同（ko=%v 字級=%v）", w, key, okKo, okCh)
				}
				if !okKo || !okCh {
					continue
				}
				st.linesKo += len(ko)
				st.linesChar += len(ch)
				st.extraLines += len(ko) - len(ch)
				st.midKo += midWordBreaks(t, a.Text, ko)
				st.midChar += midWordBreaks(t, a.Text, ch)
				if _, _, _, ok := layoutEclTextTokens(layoutKo, true, a.Text, a.Units, 17, 0, 0, uint8(w-1), 22); !ok {
					st.fallbacks++
				}
				for _, f := range strings.Fields(string(a.Text)) {
					if textUnits([]rune(f)) > w {
						st.longWords++
					}
				}
			}
		}
		t.Logf("窗寬 %2d：%d 次排版；溢出 ko %d（未加註 %d）／字級 %d／zh-TW 同窗 %d；行數 ko %d／字級 %d（多 %d）；詞中斷行 ko %d／字級 %d；整次退回字級 %d；寬於一列的詞 %d",
			w, st.calls, st.overKo, st.overKoPlain, st.overChar, st.overZhTW, st.linesKo, st.linesChar, st.extraLines, st.midKo, st.midChar, st.fallbacks, st.longWords)
		if w == 76 && st.overKo != 0 {
			t.Errorf("標準窗 76 單位不應有溢出：%d", st.overKo)
		}
		if st.midKo > st.midChar {
			t.Errorf("窗寬 %d：詞中斷行不應比字級多（%d > %d）", w, st.midKo, st.midChar)
		}
	}
}

// Every logbook entry fits three pages in the annotation tier the catalog
// keeps, and the page count of the Korean profile is reported against
// character level.
func TestKoLogbookStaticPrecheck(t *testing.T) {
	_, l := koRuntime(t)
	text, _, _ := koInputs(t)
	b, err := os.ReadFile(filepath.Join(text, LangFile("logbook", LangKo)))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ReadCatalogRows("logbook.ko.tsv", b)
	if err != nil {
		t.Fatal(err)
	}
	var entries, fallbacks, maxLines, maxPages, maxPagesChar, morePages int
	for key, body := range rows {
		if strings.HasSuffix(key, ".title") {
			continue
		}
		entries++
		// The tier the catalog keeps: the first annotation that fits three pages.
		var a AnnotatedText
		for _, v := range l.names.Variants(body, key, NameCaseMixed) {
			if _, err := layoutLogbookUnitsP(layoutKo, v.Text, v.Units); err == nil {
				a = v
				break
			}
		}
		if a.Text == nil {
			t.Errorf("%s 的所有加註段都排不進 %d 頁", key, logbookMaxPages)
			continue
		}
		byWord, err := layoutLogbookPages(layoutKo, a.Text, a.Units)
		if err != nil {
			t.Fatalf("%s：%v", key, err)
		}
		byChar, err := layoutLogbookPages(layoutKoChars, a.Text, a.Units)
		if err != nil {
			t.Fatalf("%s：%v", key, err)
		}
		if len(byWord) > logbookMaxPages {
			fallbacks++
		}
		if len(byWord) > len(byChar) {
			morePages++
		}
		if _, err := layoutLogbookUnitsP(layoutKo, a.Text, a.Units); err != nil {
			t.Errorf("%s 排不進 %d 頁：%v", key, logbookMaxPages, err)
		}
		n := 0
		for _, p := range byWord {
			n += len(p)
		}
		maxLines, maxPages, maxPagesChar = max(maxLines, n), max(maxPages, len(byWord)), max(maxPagesChar, len(byChar))
	}
	if entries != 71 {
		t.Errorf("手札列數 = %d，應為 71", entries)
	}
	t.Logf("手札 %d 則：最長 %d 行；頁數最大 ko %d／字級 %d；詞級比字級多頁 %d 則；需退回字級 %d 則", entries, maxLines, maxPages, maxPagesChar, morePages, fallbacks)
	if fallbacks != 0 || maxPages > logbookMaxPages {
		t.Errorf("手札詞級不應超過 %d 頁（退回 %d 則）", logbookMaxPages, fallbacks)
	}
}

// The layout of the languages without word level is the layout of the code
// before spec 043: on every formal ECL row and logbook entry of zh-TW,
// zh-CN and ja, in several windows and with every name annotation tier.
func TestLayoutUnchangedOnFormalTexts(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定")
	}
	text := filepath.Join(root, "text")
	for _, lang := range []string{LangZhTW, LangZhCN, LangJa} {
		eclBytes, err := os.ReadFile(filepath.Join(text, LangFile("ecl-text", lang)))
		if os.IsNotExist(err) {
			t.Skipf("text/ 沒有 %s 檔", lang)
		} else if err != nil {
			t.Fatal(err)
		}
		ecl, err := ReadCatalogRows(LangFile("ecl-text", lang), eclBytes)
		if err != nil {
			t.Fatal(err)
		}
		lbBytes, err := os.ReadFile(filepath.Join(text, LangFile("logbook", lang)))
		if err != nil {
			t.Fatal(err)
		}
		lb, err := ReadCatalogRows(LangFile("logbook", lang), lbBytes)
		if err != nil {
			t.Fatal(err)
		}
		names, err := loadNameGlossaryLang(text, text, lang)
		if err != nil {
			t.Fatal(err)
		}
		prof := LayoutFor(lang)
		var eclRows, eclCalls, logRows, logCalls int
		for key, s := range ecl {
			eclRows++
			for _, v := range names.Variants(s, key, NameCaseUpper) {
				for _, w := range []int{76, 56, 40, 30} {
					for _, col := range []uint8{0, 7} {
						for _, bottom := range []uint8{22, 255} {
							eclCalls++
							a1, r1, e1, ok1 := layoutEclTextP(prof, v.Text, v.Units, 17, col, 0, uint8(w-1), bottom)
							a2, r2, e2, ok2 := frozenLayoutEclTextP(prof, v.Text, v.Units, 17, col, 0, uint8(w-1), bottom)
							if ok1 != ok2 || r1 != r2 || e1 != e2 || !reflect.DeepEqual(a1, a2) {
								t.Fatalf("%s %s 窗寬 %d 起點 %d 底 %d：與 88f391f 的排法不同", lang, key, w, col, bottom)
							}
						}
					}
				}
			}
		}
		for key, s := range lb {
			if strings.HasSuffix(key, ".title") {
				continue
			}
			logRows++
			for _, v := range names.Variants(s, key, NameCaseMixed) {
				logCalls++
				p1, e1 := layoutLogbookUnitsP(prof, v.Text, v.Units)
				p2, e2 := frozenLayoutLogbookUnitsP(prof, v.Text, v.Units)
				if (e1 == nil) != (e2 == nil) || !reflect.DeepEqual(p1, p2) {
					t.Fatalf("%s %s：手札排法與 88f391f 不同", lang, key)
				}
			}
		}
		t.Logf("%s：ECL %d 列（%d 次排版）、手札 %d 則（%d 次排版）排法與 88f391f 相同", lang, eclRows, eclCalls, logRows, logCalls)
		if eclRows != 2542 || logRows != 71 {
			t.Errorf("%s：ECL %d 列、手札 %d 則，應為 2542、71", lang, eclRows, logRows)
		}
	}
}

// Go's Matches on the ko glossary against the zh-TW glossary, per row of
// every family file: the persons found must agree, except the row-level
// exemptions of text/ko-name-exemptions.tsv (the Python check reads the
// same file; Go applies the runtime exclusions).
func TestKoNameMatchesAgreeWithZhTW(t *testing.T) {
	root := os.Getenv("BUCKROGERS_CHT_ROOT")
	if root == "" {
		t.Skip("BUCKROGERS_CHT_ROOT 未設定")
	}
	text := filepath.Join(root, "text")
	ko, err := loadNameGlossaryLang(text, text, LangKo)
	if err != nil || ko == nil {
		t.Skipf("沒有 ko 名字表：%v", err)
	}
	zh, err := loadNameGlossaryLang(text, text, LangZhTW)
	if err != nil || zh == nil {
		t.Fatalf("zh-TW 名字表：%v", err)
	}
	exBytes, err := os.ReadFile(filepath.Join(text, "ko-name-exemptions.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	exRows, err := readPlainTSV("ko-name-exemptions.tsv", exBytes, []string{"key", "person", "reason"})
	if err != nil {
		t.Fatal(err)
	}
	exempt := map[[2]string]bool{}
	for _, r := range exRows {
		exempt[[2]string{r[0], r[1]}] = true
	}
	used := map[[2]string]bool{}
	files, _ := filepath.Glob(filepath.Join(text, "*.ko.tsv"))
	persons := func(g *NameGlossary, s, key string) map[string]bool {
		out := map[string]bool{}
		for _, m := range g.Matches([]rune(s), key) {
			out[m.Entry.Person] = true
		}
		return out
	}
	var rowsSeen, diffs int
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasPrefix(base, "name-glossary") {
			continue
		}
		zhFile := strings.Replace(f, ".ko.tsv", ".zh-TW.tsv", 1)
		kb, err1 := os.ReadFile(f)
		zb, err2 := os.ReadFile(zhFile)
		if err1 != nil || err2 != nil {
			continue
		}
		koRows, err := readTSVAllowEmpty(base, kb, langTextHeader)
		if err != nil {
			t.Fatal(err)
		}
		zhRows, err := readTSVAllowEmpty(filepath.Base(zhFile), zb, langTextHeader)
		if err != nil {
			t.Fatal(err)
		}
		zhText := map[string]string{}
		for _, r := range zhRows {
			zhText[r[0]] = r[1]
		}
		for _, r := range koRows {
			z, ok := zhText[r[0]]
			if !ok {
				continue
			}
			rowsSeen++
			pz, pk := persons(zh, z, r[0]), persons(ko, r[1], r[0])
			for _, side := range []struct {
				a, b map[string]bool
				what string
			}{{pz, pk, "缺人物"}, {pk, pz, "多人物"}} {
				for p := range side.a {
					if side.b[p] {
						continue
					}
					if exempt[[2]string{r[0], p}] {
						used[[2]string{r[0], p}] = true
						continue
					}
					diffs++
					t.Errorf("%s %s：%s %s", base, r[0], side.what, p)
				}
			}
		}
	}
	for k := range exempt {
		if !used[k] {
			t.Errorf("豁免 %v 未被使用", k)
		}
	}
	t.Logf("Go Matches 對 %d 列（zh-TW 對 ko 逐列）人物集合差異 %d 處（已扣 %d 條列級豁免）", rowsSeen, diffs, len(exempt))
	if rowsSeen != 5414 {
		t.Errorf("比對列數 = %d，應為 5414", rowsSeen)
	}
}
