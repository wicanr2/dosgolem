package buckrogers

import (
	"sort"
	"strings"
	"testing"
)

// Buck repo spec 054: Korean particles by final consonant (§3.1, §3.2 first
// item) and the party member's reading in the plain slot of a sentence (§3.3).

func TestKoReadingTail(t *testing.T) {
	cases := []struct {
		s    string
		want rune
		why  string
	}{
		{"설레스트", '트', "韓文音節結尾"},
		{"설레스트(CELESTE)", '트', "英文加註之後取括號前的音節"},
		{"설레스트(R2-D2'S J.R)", '트', "加註可含 ' . - 數字與空白"},
		{"FLAVIUS", 0, "拉丁字母"},
		{"스3", 0, "數字"},
		{"스.", 0, "標點"},
		{"스 ", 0, "空白"},
		{"", 0, "空字串"},
		{"(CELESTE)", 0, "括號前沒有韓文音節"},
		{"플라(FLA", 0, "沒有收尾括號"},
		{"스()", 0, "空的加註不算"},
		{"스(A(B))", 0, "加註內含括號不算"},
		{"스(한국)", 0, "加註內含非 ASCII 不算"},
		{"스(A\x01B)", 0, "加註內含控制字元不算"},
		{"가(A) (B)", 0, "括號前是空白"},
	}
	for _, c := range cases {
		if got := koReadingTail(c.s); got != c.want {
			t.Errorf("koReadingTail(%q) = %q，應為 %q（%s）", c.s, got, c.want, c.why)
		}
	}
}

func TestKoPickParticle(t *testing.T) {
	// 가 no final, 각 final ㄱ, 갈 final ㄹ.
	cases := map[string][3]string{
		"은(는)": {"는", "은", "은"},
		"이(가)": {"가", "이", "이"},
		"을(를)": {"를", "을", "을"},
		"와(과)": {"와", "과", "과"},
		"(으)로": {"로", "으로", "로"},
	}
	for mark, want := range cases {
		k, ok := koMarkerAt(mark + " 뒤")
		if !ok {
			t.Fatalf("標記 %q 不被認得", mark)
		}
		for i, syl := range []rune{'가', '각', '갈'} {
			if got := koPickParticle(k, syl); got != want[i] {
				t.Errorf("%s 接 %c：%q，應為 %q", mark, syl, got, want[i])
			}
		}
	}
	if len(cases) != len(koParticleMarkers) {
		t.Errorf("標記表有 %d 項，測試涵蓋 %d 項", len(koParticleMarkers), len(cases))
	}
}

func TestKoResolveMarkers(t *testing.T) {
	cases := []struct{ in, want, why string }{
		{"플라비우스은(는) 자신의 전술", "플라비우스는 자신의 전술", "無尾音：는"},
		{"테린 전사은(는)", "테린 전사는", "字串結尾的標記"},
		{"로앙은(는)", "로앙은", "有尾音：은"},
		{"설레스트(CELESTE)이(가) 서서히", "설레스트(CELESTE)가 서서히", "加註之後取括號前的音節"},
		{"리온(R2-D2'S J.R)은(는)", "리온(R2-D2'S J.R)은", "加註含 ' . - 數字與空白"},
		{"물을(를) 마신다", "물을 마신다", "ㄹ 尾音對 을(를) 與有尾音相同"},
		{"책와(과) 칼", "책과 칼", "와(과) 有尾音取 과"},
		{"책(으)로", "책으로", "(으)로 有尾音取 으로"},
		{"길(으)로", "길로", "(으)로 ㄹ 尾音取 로"},
		{"사과(으)로", "사과로", "(으)로 無尾音取 로"},
		{"FLAVIUS은(는) 자신의", "FLAVIUS은(는) 자신의", "拉丁字母：保留"},
		{"3이(가)", "3이(가)", "數字：保留"},
		{"끝!은(는)", "끝!은(는)", "標點：保留"},
		{"은(는) 있다", "은(는) 있다", "字串開頭：保留"},
		{" 은(는)", " 은(는)", "空白後：保留"},
		{"스의 에서", "스의 에서", "其他助詞不動"},
		{"스에서은(는)", "스에서는", "他助詞後的標記照前一個音節"},
		{"스은(는)과 팀이(가)", "스는과 팀이", "同一字串內多個標記各自解析"},
		{"스은(는)이(가)", "스는이", "連續標記依已解析的前文"},
		{"한()은(는)", "한()은(는)", "空的加註不算"},
		{"스\xff은(는)", "스\xff은(는)", "非法位元組不吞字也不誤判"},
		{"스\xff팀은(는)", "스\xff팀은", "非法位元組保留，後面照常解析"},
		{"", "", "空字串"},
	}
	for _, c := range cases {
		if got := koResolveMarkers(c.in); got != c.want {
			t.Errorf("%s：%q → %q，應為 %q", c.why, c.in, got, c.want)
		}
	}
	// A string without a mark comes back as the same bytes.
	if s := "이 방은 비어 있다."; koResolveMarkers(s) != s {
		t.Errorf("無標記的字串被改動")
	}
}

func TestKoResolveLeading(t *testing.T) {
	cases := []struct {
		in   string
		syl  rune
		want string
		ok   bool
	}{
		{"이(가) 서서히", '트', "가 서서히", true},
		{"이(가) 서서히", '각', "이 서서히", true},
		{"은(는) 공격을 받아", '가', "는 공격을 받아", true},
		{"(으)로 간다", '갈', "로 간다", true},
		{"이(가) 서서히", 0, "이(가) 서서히", false},
		{"의 이(가)", '트', "의 이(가)", false},
		{" 이(가)", '트', " 이(가)", false},
		{"서서히", '트', "서서히", false},
		{"", '트', "", false},
	}
	for _, c := range cases {
		got, ok := koResolveLeading(c.in, c.syl)
		if got != c.want || ok != c.ok {
			t.Errorf("koResolveLeading(%q, %q) = %q %v，應為 %q %v", c.in, c.syl, got, ok, c.want, c.ok)
		}
	}
}

// --- sentences with the party member's name ---------------------------------

// inlineCatalog is a Korean or Japanese engine catalog with fragments that
// carry particle marks and monsters and items that end with and without a
// final consonant.
func inlineCatalog(t *testing.T, lang string, frags, monsters, items map[string]string) *EngineTextCatalog {
	t.Helper()
	var fo, fp, mo, mp, io, ip []string
	for o, z := range frags {
		fo, fp = append(fo, o), append(fp, o, z)
	}
	for o, z := range monsters {
		mo, mp = append(mo, o), append(mp, o, z)
	}
	for o, z := range items {
		io, ip = append(io, o), append(ip, o, z)
	}
	c, err := LoadEngineTextCatalog(EngineTextFiles{
		Lang:           lang,
		FragmentEvents: engineTSV("frag", fo...), FragmentText: engineZh("frag", fp...),
		ItemEvents: engineTSV("item", io...), ItemText: engineZh("item", ip...),
		MonsterEvents: engineTSV("monster", mo...), MonsterText: engineZh("monster", mp...),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var (
	inlineKoMonsters = map[string]string{"NEO WARRIOR": "NEO 전사", "TERRINE WARRIOR": "테린 전사", "GIANT": "거인", "WOLF": "늑대"}
	inlineKoItems    = map[string]string{"Bolt": "볼트", "Gun": "건", "Laser": "레이저", "Pistol": "권총"}
	inlineJaFrags    = map[string]string{" makes ": "は", "his": "自分の", " tactics roll.": "戦術判定を行う。", " is hit FOR ": "は", " points of Damage.": "点のダメージを受けた。", " points ": "点", " leads ": "率いる"}
)

func partyReadings(m map[string]string) PartyNameFunc {
	return func(core string) (string, bool) { z, ok := m[core]; return z, ok }
}

func TestTranslatePartyKo(t *testing.T) {
	c := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	names := partyReadings(map[string]string{"FLAVIUS": "플라비우스", "ROARKE": "로앙", "CELESTE": "설레스트", "NEO WARRIOR": "錯"})
	cases := []struct {
		in, want string
		inline   int
		why       string
	}{
		{"FLAVIUS makes his tactics roll.", "플라비우스는 자신의 전술 판정을 한다.", 1, "名字換音譯，無尾音取 는"},
		{"ROARKE makes his tactics roll.", "로앙은 자신의 전술 판정을 한다.", 1, "有尾音取 은"},
		{"ROARKE is hit FOR 3 points of Damage.", "로앙은 공격을 받아 3 점의 피해.", 1, "名字加助詞片段"},
		{"  FLAVIUS is hit FOR 6 points of Damage.", "  플라비우스는 공격을 받아 6 점의 피해.", 1, "名字前的空白照舊保留"},
		{"MARCUS makes his tactics roll.", "MARCUS은(는) 자신의 전술 판정을 한다.", 0, "不是隊員：保留英文與標記"},
		{"NEO WARRIOR makes his tactics roll.", "NEO 전사는 자신의 전술 판정을 한다.", 0, "怪物名優先於隊員名"},
		{"GIANT makes his tactics roll.", "거인은 자신의 전술 판정을 한다.", 0, "怪物名有尾音取 은（無快照也解析）"},
		{"WOLF makes his tactics roll.", "늑대는 자신의 전술 판정을 한다.", 0, "怪物名無尾音取 는"},
		{"FLAVIUS. makes his tactics roll.", "플라비우스. 은(는) 자신의 전술 판정을 한다.", 1, "句尾標點保留，標點後的標記無法定音節：保留"},
		{"FLAVIUSX makes his tactics roll.", "FLAVIUSX은(는) 자신의 전술 판정을 한다.", 0, "名字多一個字就不是隊員"},
	}
	for _, tc := range cases {
		got, ok, inline := c.TranslateParty(tc.in, names)
		if !ok || got != tc.want || inline != tc.inline {
			t.Errorf("%s：%q → %q %v %d，應為 %q %d", tc.why, tc.in, got, ok, inline, tc.want, tc.inline)
		}
		// Without names, and through Translate: the marks are resolved all the
		// same, the name stays English.
		plain, pok := c.Translate(tc.in)
		none, nok, ninline := c.TranslateParty(tc.in, nil)
		if plain != none || pok != nok || ninline != 0 {
			t.Errorf("%s：Translate %q 與 TranslateParty(nil) %q 不同", tc.why, plain, none)
		}
		if tc.inline == 0 && plain != got {
			t.Errorf("%s：沒有換名時結果應與 Translate 相同：%q 對 %q", tc.why, got, plain)
		}
	}
}

func TestTranslatePartyJa(t *testing.T) {
	c := inlineCatalog(t, LangJa, inlineJaFrags, map[string]string{"NEO WARRIOR": "NEO 戦士"}, map[string]string{"Bolt": "ボルト"})
	names := partyReadings(map[string]string{"FLAVIUS": "フラビウス", "ROARKE": "ロアーク"})
	cases := []struct {
		in, want string
		inline   int
		why      string
	}{
		{"FLAVIUS makes his tactics roll.", "フラビウスは自分の戦術判定を行う。", 1, "片假名名字加助詞：不補空白"},
		{"MARCUS makes his tactics roll.", "MARCUSは自分の戦術判定を行う。", 0, "不是隊員：與現行相同"},
		{"ROARKE is hit FOR 3 points of Damage.", "ロアークは 3 点のダメージを受けた。", 1, "助詞片段：與英文名字的結果相同"},
		{"ROARKE leads NEO WARRIOR", "ロアーク率いる NEO 戦士", 1, "非助詞片段：名字後不再補半形空白（engineAlnum，規格 054 §3.4 第 3 點）"},
	}
	for _, tc := range cases {
		got, ok, inline := c.TranslateParty(tc.in, names)
		if !ok || got != tc.want || inline != tc.inline {
			t.Errorf("%s：%q → %q %v %d，應為 %q %d", tc.why, tc.in, got, ok, inline, tc.want, tc.inline)
		}
		if tc.inline == 0 {
			if plain, _ := c.Translate(tc.in); plain != got {
				t.Errorf("%s：沒有換名時應與 Translate 相同：%q 對 %q", tc.why, got, plain)
			}
		}
	}
}

// A name function has no effect outside Korean and Japanese: the answers of
// every other language are those of Translate, string by string.
func TestTranslatePartyLanguageGate(t *testing.T) {
	names := partyReadings(map[string]string{"FLAVIUS": "弗拉維烏斯", "ROARKE": "羅克", "CELESTE": "塞萊斯特", "PIERRE": "皮埃爾"})
	for _, lang := range []string{LangZhTW, LangZhCN, LangTest, ""} {
		c := engineFreezeCatalog(t, lang)
		var changed int
		for _, s := range engineFreezeCorpus() {
			want, wok := c.Translate(s)
			got, ok, inline := c.TranslateParty(s, names)
			if got != want || ok != wok || inline != 0 {
				t.Fatalf("lang %q：%q → %q %v %d，Translate 為 %q %v", lang, s, got, ok, inline, want, wok)
			}
			changed++
		}
		if changed == 0 {
			t.Fatal("語料是空的")
		}
	}
}

// A name function never turns a failed translation into a success, and a
// nil catalog stays nil-safe.
func TestTranslatePartyNilAndEmpty(t *testing.T) {
	var c *EngineTextCatalog
	if z, ok, n := c.TranslateParty("X", partyReadings(nil)); z != "" || ok || n != 0 {
		t.Errorf("nil catalog：%q %v %d", z, ok, n)
	}
	ko := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	names := partyReadings(map[string]string{"FLAVIUS": "플라비우스"})
	for _, s := range []string{"", "FLAVIUS", "UNKNOWN SENTENCE"} {
		if z, ok, n := ko.TranslateParty(s, names); ok || n != 0 {
			t.Errorf("%q：不應譯成 %q %v %d", s, z, ok, n)
		}
	}
}

// The slot is compared as a whole after its spaces and its sentence-final
// punctuation are taken off: a name with a space inside is a name.
func TestTranslatePartySlotMatching(t *testing.T) {
	c := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	names := partyReadings(map[string]string{"NICOLE STEELE": "니콜 스틸", "JO": "조"})
	for _, tc := range []struct{ in, want string }{
		{"NICOLE STEELE makes his tactics roll.", "니콜 스틸은 자신의 전술 판정을 한다."},
		{"JO makes his tactics roll.", "조는 자신의 전술 판정을 한다."},
		{"JOE makes his tactics roll.", "JOE은(는) 자신의 전술 판정을 한다."},
		{"NICOLE makes his tactics roll.", "NICOLE은(는) 자신의 전술 판정을 한다."},
	} {
		if got, ok, _ := c.TranslateParty(tc.in, names); !ok || got != tc.want {
			t.Errorf("%q → %q %v，應為 %q", tc.in, got, ok, tc.want)
		}
	}
}

// --- the Korean answers without names (spec 054 §3.4 item 2) ----------------

// markersOnlyDiffer reports whether got is want with some particle marks
// replaced by one of their two forms, byte for byte otherwise.
func markersOnlyDiffer(want, got string) bool {
	for i, j := 0, 0; ; {
		if i == len(want) {
			return j == len(got)
		}
		if k, ok := koMarkerAt(want[i:]); ok {
			m := koParticleMarkers[k]
			switch {
			case strings.HasPrefix(got[j:], m.mark):
				j += len(m.mark)
			case strings.HasPrefix(got[j:], m.with):
				j += len(m.with)
			case strings.HasPrefix(got[j:], m.without):
				j += len(m.without)
			default:
				return false
			}
			i += len(m.mark)
			continue
		}
		if j >= len(got) || want[i] != got[j] {
			return false
		}
		i, j = i+1, j+1
	}
}

func inlineKoCorpus() []string {
	seed := uint64(0x2545f4914f6cdd1d)
	next := func(n int) int {
		seed = seed*6364136223846793005 + 1442695040888963407
		return int((seed >> 33) % uint64(n))
	}
	pieces := []string{"FLAVIUS", "ROARKE", "CELESTE ", " PIERRE", "3", "38.", "12 ", "NEO WARRIOR.", "TERRINE WARRIOR", "GIANT", "WOLF!", "Bolt Gun", "Laser Pistol", "Bolt Gun (72)", " (5)", "IT", " "}
	keys := make([]string, 0, len(koFragments))
	for o := range koFragments {
		keys = append(keys, o)
	}
	sort.Strings(keys) // map order would change the corpus from run to run
	for _, o := range keys {
		pieces = append(pieces, o, o, o)
	}
	var out []string
	for n := 0; n < 4000; n++ {
		var b strings.Builder
		for p, np := 0, 1+next(6); p < np; p++ {
			b.WriteString(pieces[next(len(pieces))])
		}
		out = append(out, b.String())
	}
	return append(out, "GIANT makes his tactics roll.", "WOLF is hit FOR 3 points of Damage.", "Fires Bolt Gun at GIANT",
		"FLAVIUS makes his tactics roll.", "(from behind) Hitting for 3 points of damage")
}

// Without names the Korean answer is the one of spec 046 (the frozen copy)
// with the marks resolved, and only the marks differ.  The set of strings
// that differ is not empty: the test would otherwise compare nothing.
func TestTranslateKoWithoutNamesOnlyMarksDiffer(t *testing.T) {
	c := inlineCatalog(t, LangKo, koFragments, inlineKoMonsters, inlineKoItems)
	var total, differ, resolvedFinal, resolvedOpen int
	for _, s := range inlineKoCorpus() {
		frozen, fok := frozenTranslateLine046(c, s)
		got, ok := c.translateLine(s)
		if ok != fok {
			t.Fatalf("%q：能否翻譯與凍結副本不同（%v 對 %v）", s, ok, fok)
		}
		if !ok {
			continue
		}
		total++
		if want := koResolveMarkers(frozen); got != want {
			t.Fatalf("%q：%q，應為 %q", s, got, want)
		}
		if got != frozen {
			differ++
			if !markersOnlyDiffer(frozen, got) {
				t.Fatalf("%q：差異不只在標記：%q 對 %q", s, frozen, got)
			}
			if strings.Contains(got, "은 ") || strings.HasSuffix(got, "은") {
				resolvedFinal++
			}
			if strings.Contains(got, "는 ") || strings.HasSuffix(got, "는") {
				resolvedOpen++
			}
		}
		// The public entry points agree with translateLine on a plain line.
		if z, zok := c.Translate(s); engineTableRow.MatchString(s) == false && (z != got || zok != ok) {
			t.Fatalf("%q：Translate %q 與 translateLine %q 不同", s, z, got)
		}
	}
	t.Logf("語料 %d 筆可譯，%d 筆與凍結副本不同（無尾音 %d、有尾音 %d）", total, differ, resolvedOpen, resolvedFinal)
	if differ == 0 || resolvedFinal == 0 || resolvedOpen == 0 {
		t.Fatalf("差集需包含有尾音與無尾音：differ=%d 有尾音=%d 無尾音=%d", differ, resolvedFinal, resolvedOpen)
	}
}

// The table row path pads with the width of the resolved front: the mark is
// replaced before the padding is computed, and the last column stays at the
// original right edge.
func TestTranslateKoTableRowUsesResolvedFront(t *testing.T) {
	c := inlineCatalog(t, LangKo, map[string]string{"Hits": "대상은(는)", "Shots": "샷은(는)"}, inlineKoMonsters, map[string]string{"Bolt": "볼트"})
	for _, tc := range []struct{ row, front string }{
		{"Hits   12", "대상은"},
		{"Shots  7", "샷은"},
	} {
		got, ok := c.Translate(tc.row)
		if !ok {
			t.Fatalf("%q 譯不出", tc.row)
		}
		if !strings.HasPrefix(got, tc.front) || strings.Contains(got, "(") {
			t.Errorf("%q：前段應為解析後的 %q，得到 %q", tc.row, tc.front, got)
		}
		if u := stringUnits(got); u != 2*len(tc.row) {
			t.Errorf("%q：共 %d 單位，應為原版的 %d", tc.row, u, 2*len(tc.row))
		}
	}
}
