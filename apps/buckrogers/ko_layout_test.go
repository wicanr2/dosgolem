package buckrogers

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Buck repo spec 043 §3.4: word-level wrapping, its fallbacks, the two-row
// split, and the proof that every other profile is unchanged.

func rowTexts(lines []EclTextLine) []string {
	var out []string
	for _, l := range lines {
		out = append(out, string(l.Text))
	}
	return out
}

// wordLayout lays text out with the Korean profile from the top-left of a
// window `width` units wide and `rows` rows high.
func wordLayout(prof *LayoutProfile, text string, units []NameUnit, width, rows int) ([]string, bool) {
	lines, _, _, ok := layoutEclTextP(prof, []rune(text), units, 0, 0, 0, uint8(width-1), uint8(rows-1))
	return rowTexts(lines), ok
}

func TestKoProfileRegistered(t *testing.T) {
	if LayoutFor(LangKo) != layoutKo || !layoutKo.word || layoutKo.charLevel() != layoutKoChars || layoutKoChars.word {
		t.Fatal("ko 的排版設定未正確註冊")
	}
	if LayoutFor(LangZhTW) != nil || LayoutFor(LangZhCN) != nil || LayoutFor(LangJa) != layoutJa || LayoutFor(LangEn) != nil {
		t.Error("其他語言的排版設定不應改變")
	}
	if !KnownLang(LangKo) {
		t.Error("ko 應在語言循環內")
	}
	if layoutJa.charLevel() != layoutJa || (*LayoutProfile)(nil).charLevel() != nil {
		t.Error("沒有詞級的 profile，字級退回就是它自己")
	}
}

func TestKoWordTokens(t *testing.T) {
	const text = "안녕하세요 여러분 반갑습니다"
	got, ok := wordLayout(layoutKo, text, nil, 12, 6)
	if want := []string{"안녕하세요", "여러분", "반갑습니다"}; !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("詞級 = %q ok=%v，應為 %q", got, ok, want)
	}
	// Character level splits 반갑습니다 in the middle; that is the defect.
	got, ok = wordLayout(layoutKoChars, text, nil, 12, 6)
	if want := []string{"안녕하세요", "여러분 반갑", "습니다"}; !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("字級 = %q ok=%v，應為 %q", got, ok, want)
	}
	// A word wrapped to a new line loses the space before it, the last line
	// keeps what fits, and closing punctuation stays with its word.
	got, ok = wordLayout(layoutKo, "가나 다라마바 사아.", nil, 14, 6)
	if want := []string{"가나 다라마바", "사아."}; !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("收尾標點 = %q ok=%v，應為 %q", got, ok, want)
	}
}

func TestKoNameUnitWithParticle(t *testing.T) {
	const name = "벅 로저스(BUCK ROGERS)"
	units := []NameUnit{{0, len([]rune(name))}}
	const text = name + "는 갔다"
	// 22 units of name plus a 2-unit particle fit a 24-unit line: one token.
	got, ok := wordLayout(layoutKo, text, units, 24, 6)
	if want := []string{name + "는", "갔다"}; !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("窗寬 24 = %q ok=%v，應為 %q", got, ok, want)
	}
	// On a 22-unit line the particle cannot join; it starts the next line.
	got, ok = wordLayout(layoutKo, text, units, 22, 6)
	if want := []string{name, "는 갔다"}; !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("窗寬 22 = %q ok=%v，應為 %q", got, ok, want)
	}
	// A name unit wider than the line still breaks at its own spaces.
	got, ok = wordLayout(layoutKo, text, units, 12, 6)
	if want := []string{"벅", "로저스(BUCK", "ROGERS)는", "갔다"}; !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("窗寬 12 = %q ok=%v，應為 %q", got, ok, want)
	}
}

func TestKoOpeningQuoteNeverEndsLine(t *testing.T) {
	const name = "벅 로저스(BUCK ROGERS)"
	text := "그는 「" + name + "」라고 말했다"
	start := len([]rune("그는 「"))
	units := []NameUnit{{start, start + len([]rune(name))}}
	got, ok := wordLayout(layoutKo, text, units, 30, 6)
	if want := []string{"그는", "「" + name + "」라고", "말했다"}; !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("「 與名字單位 = %q ok=%v，應為 %q", got, ok, want)
	}
	for _, l := range got {
		if strings.HasSuffix(l, "「") || strings.HasSuffix(l, "『") {
			t.Errorf("行尾不該是開引號：%q", l)
		}
	}
}

func TestKoLongWordFallsBackToCharacters(t *testing.T) {
	text := "가 " + strings.Repeat("나", 20) + " 다"
	got, ok := wordLayout(layoutKo, text, nil, 30, 6)
	want := []string{"가 " + strings.Repeat("나", 13), strings.Repeat("나", 7) + " 다"}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("長詞 = %q ok=%v，應為 %q", got, ok, want)
	}
	// `...` at the start of an over-wide word is one token: not split.
	got, ok = wordLayout(layoutKo, "...그는 갔다", nil, 6, 6)
	if !ok || len(got) == 0 || !strings.HasPrefix(got[0], "...") {
		t.Errorf("列首 ... 被拆開：%q ok=%v", got, ok)
	}
}

func TestKoWholeCallFallsBackToCharacterLevel(t *testing.T) {
	// Words waste space: by words this needs 3 rows, by characters 2.
	const text = "가 나다라마바 사"
	if _, _, _, ok := layoutEclTextTokens(layoutKo, true, []rune(text), nil, 0, 0, 0, 9, 1); ok {
		t.Fatal("測試前提不成立：詞級應在兩列內放不下")
	}
	got, ok := wordLayout(layoutKo, text, nil, 10, 2)
	if want := []string{"가 나다라", "마바 사"}; !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("退回字級 = %q ok=%v，應為 %q", got, ok, want)
	}
	if _, ok := wordLayout(layoutKo, text, nil, 10, 1); ok {
		t.Error("一列放不下時不應成功")
	}
	if ref, ok2 := wordLayout(layoutKoChars, text, nil, 10, 2); !ok2 || !reflect.DeepEqual(ref, got) {
		t.Errorf("退回結果應等於字級：%q", ref)
	}
}

func TestKoSplitRows(t *testing.T) {
	type c struct {
		name       string
		r          string
		n1, n2     int
		first, sec string
		ok         bool
		prof       *LayoutProfile
	}
	cases := []c{
		{"拆點在空白之後", "안녕하세요 여러분", 6, 6, "안녕하세요 ", "여러분", true, layoutKo},
		{"拆點剛好落在空白", "가나다라마바사 아자", 7, 7, "가나다라마바사", "아자", true, layoutKo},
		{"拆點在詞中往前找空白", "가나다라마바 사아자", 8, 8, "가나다라마바", "사아자", true, layoutKo},
		{"空白太遠不找", "가나 다라마바사아자차카", 6, 8, "가나 다라마", "바사아자차카", true, layoutKo},
		{"移動後第二列放不下就維持原拆點", "가나다라마바 사아자차카타파", 8, 3, "가나다라마바 사", "아자차카타파", false, layoutKo},
		{"落在空白但第二列放不下", "가나다라마바사 아자차카타", 7, 2, "가나다라마바사", "아자차카타", false, layoutKo},
		{"整段放得下第一列", "가나다", 4, 4, "가나다", "", true, layoutKo},
		{"字級 profile 不丟空白", "가나다라마바사 아자", 7, 7, "가나다라마바사", " 아자", true, layoutKoChars},
		{"nil 不找空白", "가나다라마바 사아자", 8, 8, "가나다라마바 사", "아자", true, nil},
		{"ja 不找空白", "가나다라마바 사아자", 8, 8, "가나다라마바 사", "아자", true, layoutJa},
	}
	for _, tc := range cases {
		first, sec, ok := tc.prof.splitRows([]rune(tc.r), tc.n1, tc.n2)
		if string(first) != tc.first || string(sec) != tc.sec || ok != tc.ok {
			t.Errorf("%s：%q | %q ok=%v，應為 %q | %q ok=%v", tc.name, string(first), string(sec), ok, tc.first, tc.sec, tc.ok)
		}
	}
}

// The profiles without word level give exactly the answers of the code
// before word-level wrapping (digests computed from the real functions at
// commit 88f391f, see layout_freeze_test.go).
func TestLayoutWithoutWordLevelUnchanged(t *testing.T) {
	if got := freezeEclDigest(layoutEclTextP); got != freezeEclWant {
		t.Errorf("layoutEclTextP 摘要 %s，應為 %s", got, freezeEclWant)
	}
	if got := freezeLogbookDigest(layoutLogbookUnitsP); got != freezeLogbookWant {
		t.Errorf("layoutLogbookUnitsP 摘要 %s，應為 %s", got, freezeLogbookWant)
	}
	if got := freezeSplitDigest(func(p *LayoutProfile, r []rune, n1, n2 int) ([]rune, []rune, bool) {
		return p.splitRows(r, n1, n2)
	}); got != freezeSplitWant {
		t.Errorf("splitRows 摘要 %s，應為 %s", got, freezeSplitWant)
	}
}

// On the synthetic corpus the Korean profile never fails where character
// level fits, and a word-level answer is used only when it fits.
func TestKoNeverWorseThanCharacterLevel(t *testing.T) {
	var cases, koOnly, differ int
	for _, c := range freezeCorpus() {
		cases++
		a, _, _, okA := layoutEclTextP(layoutKo, c.text, c.units, c.row, c.col, c.left, c.right, c.bottom)
		b, _, _, okB := layoutEclTextP(layoutKoChars, c.text, c.units, c.row, c.col, c.left, c.right, c.bottom)
		if okB && !okA {
			t.Fatalf("字級放得下但 ko 失敗：%q 窗 [%d,%d] 底 %d", string(c.text), c.left, c.right, c.bottom)
		}
		if okA && !okB {
			koOnly++
		}
		if okA && okB && !reflect.DeepEqual(a, b) {
			differ++
		}
	}
	t.Logf("合成語料 %d 組：詞級放得下而字級放不下 %d 組；兩者都放得下但排法不同 %d 組", cases, koOnly, differ)
	if differ == 0 {
		t.Error("語料沒有涵蓋詞級與字級排法不同的情況")
	}
}

func TestKoLogbookFallsBackToCharacterLevel(t *testing.T) {
	// 34-syllable words fill 68 of 72 units: by words every word is a row, by
	// characters the text flows over the row ends and needs fewer rows.
	w := strings.Repeat("가", 34)
	var found int
	for k := 40; k <= 70; k++ {
		body := []rune(strings.TrimSpace(strings.Repeat(w+" ", k)))
		byWord, err := layoutLogbookPages(layoutKo, body, nil)
		if err != nil {
			t.Fatal(err)
		}
		byChar, err := layoutLogbookPages(layoutKoChars, body, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, gerr := layoutLogbookUnitsP(layoutKo, body, nil)
		switch {
		case len(byWord) <= logbookMaxPages:
			if gerr != nil || !reflect.DeepEqual(got, byWord) {
				t.Errorf("k=%d：詞級 %d 頁應直接採用", k, len(byWord))
			}
		case len(byChar) <= logbookMaxPages:
			found++
			if gerr != nil || !reflect.DeepEqual(got, byChar) {
				t.Errorf("k=%d：詞級 %d 頁超過上限，應退回字級的 %d 頁", k, len(byWord), len(byChar))
			}
		case gerr == nil:
			t.Errorf("k=%d：兩種排法都超過 %d 頁卻沒有失敗", k, logbookMaxPages)
		}
	}
	if found == 0 {
		t.Error("沒有找到詞級超過 3 頁、退回字級後放得下的手札長度")
	}
	t.Logf("詞級超過 %d 頁而退回字級後放得下：%d 種長度", logbookMaxPages, found)
}

func TestKoLoadNameGlossaryAllowsInnerSpaces(t *testing.T) {
	head := "english\tenglish_mixed\tchinese\tkind\tperson\tbasis\tnote\n"
	row := func(zh string) []byte {
		return []byte(head + "BUCK ROGERS\tBuck Rogers\t" + zh + "\tfull\tbuck\tko-glossary\tn\n")
	}
	for _, zh := range []string{"벅 로저스", "벅  로저스", " 벅 로저스", "벅 로저스 ", "벅 로저스(x)", "벅 로저스（x）"} {
		_, errKo := loadNameGlossaryBytes(row(zh), nil, true)
		_, errOther := LoadNameGlossary(row(zh), nil)
		okKo := zh == "벅 로저스"
		if (errKo == nil) != okKo {
			t.Errorf("ko 載入 %q：err=%v，應%s", zh, errKo, map[bool]string{true: "成功", false: "失敗"}[okKo])
		}
		if errOther == nil {
			t.Errorf("其他語言不得接受含空白或括號的 %q", zh)
		}
	}
	// A single-word name is accepted by both.
	if _, err := LoadNameGlossary(row("벅"), nil); err != nil {
		t.Error(err)
	}
	// By language: only ko reads a spaced name from name-glossary.<lang>.tsv.
	dir := t.TempDir()
	for _, lang := range []string{LangKo, LangJa, LangZhCN} {
		if err := os.WriteFile(filepath.Join(dir, LangFile("name-glossary", lang)), row("벅 로저스"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := loadNameGlossaryLang(dir, dir, lang)
		if (err == nil) != (lang == LangKo) {
			t.Errorf("%s：err=%v", lang, err)
		}
	}
}

func TestKoNameWithSpaceAnnotatesAsOneUnit(t *testing.T) {
	head := "english\tenglish_mixed\tchinese\tkind\tperson\tbasis\tnote\n"
	g, err := loadNameGlossaryBytes([]byte(head+
		"BUCK ROGERS\tBuck Rogers\t벅 로저스\tfull\tbuck\tko-glossary\tn\n"+
		"BUCK\tBuck\t벅\tshort\tbuck\tko-glossary\tn\n"+
		"JIM\t\t짐\tfull\tjim\tko-glossary\tn\n"), []byte("phrase\tscope\tnote\n짐작\t*\t普通詞\n짐\tecl.6.97.*\t太陽王自稱\n"), true)
	if err != nil {
		t.Fatal(err)
	}
	a := g.Annotate("벅 로저스는 짐작했다", "ecl.1", NameCaseUpper, NameTierAll)
	if got, want := string(a.Text), "벅 로저스(BUCK ROGERS)는 짐작했다"; got != want {
		t.Errorf("加註 = %q，應為 %q", got, want)
	}
	if len(a.Units) != 1 || string(a.Text[a.Units[0].Start:a.Units[0].End]) != "벅 로저스(BUCK ROGERS)" {
		t.Errorf("名字單位 = %v", a.Units)
	}
	if got := string(g.Annotate("짐이 왔다", "ecl.6.97.02158", NameCaseUpper, NameTierAll).Text); got != "짐이 왔다" {
		t.Errorf("限定 key 的排除失效：%q", got)
	}
	if got := string(g.Annotate("짐이 왔다", "ecl.1", NameCaseUpper, NameTierAll).Text); got != "짐(JIM)이 왔다" {
		t.Errorf("其他 key 應加註：%q", got)
	}
}
