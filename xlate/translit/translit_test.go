package translit

import (
	"os"
	"path/filepath"
	"testing"
)

// testdata/text 是拯救地球 repo text/ 的副本：translit-table.tsv、translit-arpabet.tsv、
// translit-names.tsv、translit-chars.zh-TW.tsv 與授權檔原樣複製；cmudict.dict 只取測試用到的
// 詞（原樣逐行擷取），CMUdict 授權見 testdata/text/cmudict/LICENSE。
func load(t *testing.T) *Transliterator {
	t.Helper()
	tr, err := Load("testdata/text")
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// loadRaw 清空專案詞典，讓附註規則測試固定走 CMUdict 路徑。
func loadRaw(t *testing.T) *Transliterator {
	t.Helper()
	tr := load(t)
	tr.names = map[string]map[string]string{}
	return tr
}

type want struct {
	name   string
	gender Gender
	zh     string
	tier   Tier
}

func check(t *testing.T, tr *Transliterator, cases []want) {
	t.Helper()
	for _, c := range cases {
		zh, tier, ok := tr.Transliterate(c.name, c.gender)
		if !ok || zh != c.zh || tier != c.tier {
			t.Errorf("Transliterate(%q, %d) = %q %s %v，應為 %q %s", c.name, c.gender, zh, tier, ok, c.zh, c.tier)
		}
	}
}

// 規格 037 §5 第 2 項的固定測試名單。
func TestFixedNames(t *testing.T) {
	tr := load(t)
	check(t, tr, []want{
		{"ROARKE", Male, "羅克", TierCMUdict},
		{"CELESTE", Female, "塞萊斯特", TierCMUdict},
		{"FLAVIUS", Male, "弗拉維烏斯", TierDict}, // 拉丁慣譯；CMUdict 路徑為「弗萊維烏斯」
		{"JANELLE", Female, "賈內爾", TierDict},
		{"PIERRE", Male, "皮埃爾", TierCMUdict},
		{"NICOLE STEELE", Female, "妮可•斯蒂爾", TierCMUdict},
		{"BUCK", Male, "巴克", TierCMUdict},
		{"WILMA", Female, "威爾瑪", TierCMUdict},
		{"WILMA", GenderUnknown, "威爾馬", TierCMUdict},
		{"PORT", GenderUnknown, "波特", TierCMUdict},
		{"EXIT", GenderUnknown, "埃格齊特", TierCMUdict},
	})
}

// §3.3 附註規則，每條一組；註解寫出不套規則時會得到什麼。
func TestRule1InitialSchwaA(t *testing.T) {
	// ADELE = AH0 D EH1 L；詞首 a 讀 [ə] 按 [ɑː] 列（否則「厄德爾」）。
	check(t, loadRaw(t), []want{{"ADELE", Female, "阿德爾", TierCMUdict}})
}

func TestRule2FinalIA(t *testing.T) {
	// 詞尾 ia 的 a 譯「亞」；女名用「婭」。
	check(t, loadRaw(t), []want{
		{"JULIA", GenderUnknown, "朱利亞", TierCMUdict},
		{"JULIA", Female, "朱莉婭", TierCMUdict},
		{"MARIA", Female, "瑪麗婭", TierCMUdict},
		{"SONIA", Female, "索妮婭", TierCMUdict},
	})
}

func TestRule3InitialAI(t *testing.T) {
	// AIKEN = EY1 K IH0 N；詞首 ai 按「艾」列（否則「埃肯」）。
	check(t, loadRaw(t), []want{{"AIKEN", Male, "艾肯", TierCMUdict}})
}

func TestRule4FinalR(t *testing.T) {
	// DYER = D AY1 ER0：無聲母的詞尾 [ə] 拼作 r 時譯「爾」（否則「代厄」）；元音後詞尾 r 同。
	check(t, loadRaw(t), []want{
		{"DYER", Male, "代爾", TierCMUdict},
		{"MOORE", Male, "穆爾", TierCMUdict},
		{"CARTER", Male, "卡特", TierCMUdict}, // 有聲母的 [tə] 仍按表「特」
	})
}

func TestRule5TRDR(t *testing.T) {
	// [tr] 按 [t]＋[r] 譯，不合成塞擦音。
	check(t, loadRaw(t), []want{{"PATRICK", Male, "帕特里克", TierCMUdict}})
}

func TestRule6MBeforeBP(t *testing.T) {
	// m 在 b、p 前按 [n]：CAMPBELL 的 [æm] → 坎、THOMPSON 的 [ɒm] → 唐。
	check(t, loadRaw(t), []want{
		{"CAMPBELL", Male, "坎貝爾", TierCMUdict},
		{"THOMPSON", Male, "唐普森", TierCMUdict},
	})
}

func TestRule7InitialF(t *testing.T) {
	// 詞首單獨 [f] 用「弗」；詞尾用「夫」。
	check(t, loadRaw(t), []want{
		{"FRANK", Male, "弗蘭克", TierCMUdict},
		{"JEFF", Male, "傑夫", TierCMUdict},
	})
}

func TestRule8TH(t *testing.T) {
	check(t, loadRaw(t), []want{
		{"SMITH", Male, "斯米思", TierCMUdict},
		{"KEITH", Male, "基思", TierCMUdict},
		{"MATTHEW", Male, "馬休", TierCMUdict}, // th 在元音前仍按表
	})
}

func TestRule9Unstressed(t *testing.T) {
	// CELESTE = S AH0 L EH1 S T：非重讀 [ə] 拼作 e，按 e 列「塞」（否則「瑟」）。
	// ROBERT：AA 拼作 o 按 [ɒ] 列「羅」；ER0 有聲母按表「伯」。
	check(t, loadRaw(t), []want{
		{"CELESTE", Male, "塞萊斯特", TierCMUdict},
		{"ROBERT", Male, "羅伯特", TierCMUdict},
		{"DORIS", Female, "多麗絲", TierCMUdict}, // AH0 拼作 i → i 列「里／麗」
	})
}

func TestRule10Female(t *testing.T) {
	check(t, loadRaw(t), []want{
		{"LUCY", Female, "盧茜", TierCMUdict}, // 「西」音作「茜」
		{"LUCY", Male, "盧西", TierCMUdict},
		{"QUINCY", Female, "昆茜", TierCMUdict},
		{"AGNES", Female, "阿格妮絲", TierCMUdict}, // 詞尾 [s] 用「絲」
		{"AGNES", GenderUnknown, "阿格尼斯", TierCMUdict},
		{"STEELE", Female, "斯蒂爾", TierCMUdict}, // 詞首 [s] 不用「絲」
		{"CHARLES", Male, "查爾斯", TierCMUdict},  // 詞尾 -s 讀 [z] 仍按形譯「斯」
		{"ROARKE", Female, "蘿克", TierCMUdict},
		// [f]＋[ɑː] 格來源標女名「娃」，疑為疏漏，程式略過：女名仍用「法」。
		{"STEPHANIE", Female, "斯特法妮", TierCMUdict},
	})
}

func TestMiscPhonology(t *testing.T) {
	check(t, loadRaw(t), []want{
		{"DANIEL", Male, "達尼爾", TierCMUdict},   // [njə] → 尼
		{"ROBERTS", Male, "羅伯茨", TierCMUdict},  // 詞尾 [ts]
		{"EDWARDS", Male, "埃德沃茲", TierCMUdict}, // 詞尾 [dz]
		{"BOYD", Male, "博伊德", TierCMUdict},     // [ɔɪ]
		{"GWEN", Female, "古恩", TierCMUdict},    // [ɡʷ] 欄
		{"STEPHANIE", Male, "斯特法尼", TierCMUdict},
		{"FLAVIUS", Male, "弗萊維烏斯", TierCMUdict}, // IH0 拼作 u → 烏
	})
}

func TestTiersAndJoin(t *testing.T) {
	tr := load(t)
	check(t, tr, []want{
		{"NICOLE", GenderUnknown, "妮可", TierDict},
		{"nicole", GenderUnknown, "妮可", TierDict}, // 先轉大寫
		{"STEELE", GenderUnknown, "斯蒂爾", TierCMUdict},
		{"ZORBLAX", GenderUnknown, "佐布拉克斯", TierSpelling},
		{"NICOLE  ZORBLAX", GenderUnknown, "妮可•佐布拉克斯", TierSpelling}, // 取最低等級
		{"BUCK NICOLE", GenderUnknown, "巴克•妮可", TierCMUdict},
	})
}

func TestDictGenderPriority(t *testing.T) {
	tr := load(t)
	tr.names["TESTNAME"] = map[string]string{"F": "妮可", "": "尼科"}
	tr.names["ONLYF"] = map[string]string{"F": "妮可"}
	tr.names["BOTH"] = map[string]string{"F": "妮可", "M": "尼科"}
	check(t, tr, []want{
		{"TESTNAME", Female, "妮可", TierDict},
		{"TESTNAME", Male, "尼科", TierDict},
		{"TESTNAME", GenderUnknown, "尼科", TierDict},
		{"ONLYF", Female, "妮可", TierDict},
		{"ONLYF", GenderUnknown, "妮可", TierDict}, // 性別未知：不分性別 → M → F
		{"BOTH", GenderUnknown, "尼科", TierDict},
		{"JAMES", GenderUnknown, "詹姆斯", TierDict},
	})
	if _, tier, _ := tr.Transliterate("ONLYF", Male); tier == TierDict {
		t.Error("性別已知且只有異性條目時不應命中詞典")
	}
	if zh, tier, _ := tr.Transliterate("MARY", Male); zh != "梅里" || tier != TierCMUdict {
		t.Errorf("MARY（男）應走 CMUdict 得「梅里」，得到 %q %s", zh, tier)
	}
}

func TestRejected(t *testing.T) {
	tr := load(t)
	for _, n := range []string{"", "   ", "A 1.?", "R2D2", "Z", "BUCK Z", "O'", "-", "ÉMILE", "BUCK.", "J.R", "J-P", "MARY-J", "--"} {
		if zh, _, ok := tr.Transliterate(n, GenderUnknown); ok {
			t.Errorf("Transliterate(%q) = %q，應為 ok=false", n, zh)
		}
	}
}

func TestApostropheHyphen(t *testing.T) {
	check(t, load(t), []want{
		{"O'BRIEN", Male, "奧布賴恩", TierCMUdict}, // CMUdict 保留撇號查
		// 整字查無時各段分別走三層查找，以「-」連接。
		{"MARY-JANE", Female, "瑪麗-珍", TierDict},
		{"MARY-JANE", GenderUnknown, "瑪麗-珍", TierDict},
		{"MARY-JANE", Male, "梅里-傑恩", TierCMUdict}, // 男性不命中女名詞典條目
		{"JANE-ZORBLAX", Female, "珍-佐布拉克絲", TierSpelling},
		{"MARY--JANE", Female, "瑪麗-珍", TierDict}, // 空段略過
		{"BUCK O'BRIEN-SMITH", Male, "巴克•奧布賴恩-斯米思", TierCMUdict},
		{"JONES", GenderUnknown, "瓊斯", TierDict}, // CMUdict 路徑按表為「準斯」（[oʊn] 在 uːn 列）
	})
}

func TestAllowedSet(t *testing.T) {
	tr := load(t)
	if _, _, ok := tr.Transliterate("BUCK", Male); !ok {
		t.Fatal("BUCK 應可音譯")
	}
	delete(tr.allowed, '巴')
	if zh, _, ok := tr.Transliterate("BUCK", Male); ok {
		t.Errorf("允許字集外的字應回 ok=false，得到 %q", zh)
	}
}

func TestDeterministic(t *testing.T) {
	a, b := load(t), load(t)
	names := []string{"ROARKE", "MARY-JANE", "ZORBLAX QUUX", "NICOLE STEELE", "O'BRIEN"}
	for _, g := range []Gender{GenderUnknown, Male, Female} {
		for _, n := range names {
			z1, t1, o1 := a.Transliterate(n, g)
			for i := 0; i < 50; i++ {
				z2, t2, o2 := b.Transliterate(n, g)
				if z1 != z2 || t1 != t2 || o1 != o2 {
					t.Fatalf("%q 不決定：%q/%q", n, z1, z2)
				}
			}
		}
	}
}

func TestSpellingPhones(t *testing.T) {
	cases := map[string]string{
		"krang":   "K R AE1 NG",
		"jane":    "JH EY1 N",
		"quux":    "K W AH1 K S",
		"cecily":  "S EH1 S IH1 L IY1",
		"zorblax": "Z AO1 R B L AE1 K S",
	}
	for w, exp := range cases {
		got := ""
		for i, p := range spellingPhones(w) {
			if i > 0 {
				got += " "
			}
			got += p.sym
			if p.vowel {
				got += "1"
			}
		}
		if got != exp {
			t.Errorf("spellingPhones(%q) = %q，應為 %q", w, got, exp)
		}
	}
}

func TestAlignVowels(t *testing.T) {
	cases := []struct {
		word   string
		nv     int
		expect []string
	}{
		{"celeste", 2, []string{"e", "e"}},
		{"pierre", 2, []string{"ie", "e"}},
		{"flavius", 3, []string{"a", "i", "u"}},
		{"charles", 1, []string{"a"}},
		{"maria", 3, []string{"a", "i", "a"}},
		{"xyz", 3, nil},
	}
	for _, c := range cases {
		ph := make([]ph, c.nv)
		for i := range ph {
			ph[i].vowel = true
		}
		got := alignVowels(c.word, ph)
		if len(got) != len(c.expect) {
			t.Errorf("alignVowels(%q) = %v，應為 %v", c.word, got, c.expect)
			continue
		}
		for i := range got {
			if got[i] != c.expect[i] {
				t.Errorf("alignVowels(%q) = %v，應為 %v", c.word, got, c.expect)
				break
			}
		}
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("空目錄應回錯誤")
	}
	// arpabet 指到不存在的列要在 Load 時被攔下。
	dir := t.TempDir()
	copyTree(t, "testdata/text", dir)
	f, err := os.OpenFile(filepath.Join(dir, "translit-arpabet.tsv"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("QQ\tvowel\t不存在\t\n")
	f.Close()
	if _, err := Load(dir); err == nil {
		t.Error("指到不存在的元音列應回錯誤")
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
