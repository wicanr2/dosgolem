package strikecommander

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// strike_commander 規格 010 §4.1 第 2 項（結構表）與第 9 項（lint）。

const (
	scI022Env        = "DOSGOLEM_SC_I022"
	scOrigEnv        = "DOSGOLEM_SC_ORIG"
	scSpeechIndexEnv = "DOSGOLEM_SC_SPEECH_INDEX"
)

func readTSV(t *testing.T, p string) (header []string, rows [][]string) {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	ls := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	header = strings.Split(ls[0], "\t")
	for _, l := range ls[1:] {
		rows = append(rows, strings.Split(l, "\t"))
	}
	return header, rows
}

func col(h []string, name string) int {
	for i, c := range h {
		if c == name {
			return i
		}
	}
	return -1
}

// 第 2 項：以 i022 的 conv-ops.tsv、conv-lines.tsv 獨立算出整張表，與 text/speech.index.tsv 逐列相同；samples 以原版 VOCLIST.TRE 驗。
func TestSpeechIndexRebuilt(t *testing.T) {
	i022, orig, idxPath := os.Getenv(scI022Env), os.Getenv(scOrigEnv), os.Getenv(scSpeechIndexEnv)
	if i022 == "" || orig == "" || idxPath == "" {
		t.Skip(scI022Env + "、" + scOrigEnv + "、" + scSpeechIndexEnv + " 未全部設定")
	}
	// 非句操作碼的固定大小（convparse 的版面）。
	size := map[string]int{"00": 10, "01": 14, "0B": 3, "0C": 3, "0E": 3, "0F": 1, "10": 1, "11": 1}
	isLine := map[string]bool{"02": true, "03": true, "04": true, "0A": true}
	oh, ops := readTSV(t, filepath.Join(i022, "conv/conv-ops.tsv"))
	lh, lines := readTSV(t, filepath.Join(i022, "conv/conv-lines.tsv"))
	segLen := map[int]int{}
	// gaps[g][k]：第 k 句之後、下一句之前的非句操作碼大小總和。
	gaps := map[int][]int{}
	for _, r := range ops {
		if r[col(oh, "pak")] != "CONV.PAK" {
			continue
		}
		g, _ := strconv.Atoi(r[col(oh, "conv")])
		n, _ := strconv.Atoi(r[col(oh, "len")])
		segLen[g] = n
		var gs []int
		for _, tok := range strings.Fields(r[col(oh, "ops")]) {
			op := tok[:2]
			switch {
			case isLine[op]:
				gs = append(gs, 0)
			case size[op] != 0:
				if len(gs) > 0 {
					gs[len(gs)-1] += size[op]
				}
			default:
				t.Fatalf("段 %d 不認得的操作碼 %q", g, tok)
			}
		}
		gaps[g] = gs
	}
	vl := voclistSamples(t, filepath.Join(orig, "VOCLIST.TRE"))
	type ln struct {
		idx, pos         int
		op, speaker, voc string
		text             string
	}
	bySeg := map[int][]ln{}
	var segOrder []int
	for _, r := range lines {
		if r[col(lh, "pak")] != "CONV.PAK" {
			continue
		}
		g, _ := strconv.Atoi(r[col(lh, "conv")])
		i, _ := strconv.Atoi(r[col(lh, "idx")])
		pos, _ := strconv.ParseUint(r[col(lh, "pos")], 16, 32)
		if _, ok := bySeg[g]; !ok {
			segOrder = append(segOrder, g)
		}
		bySeg[g] = append(bySeg[g], ln{i, int(pos), r[col(lh, "op")], r[col(lh, "speaker")], r[col(lh, "voc")], r[col(lh, "text")]})
	}
	var want []string
	for _, g := range segOrder {
		ls := bySeg[g]
		if len(ls) != len(gaps[g]) {
			t.Fatalf("段 %d：conv-lines %d 句、conv-ops %d 句", g, len(ls), len(gaps[g]))
		}
		for k, l := range ls {
			var end int
			if k+1 < len(ls) {
				end = ls[k+1].pos - gaps[g][k]
			} else {
				end = segLen[g] - gaps[g][k]
			}
			if l.voc == "" {
				continue
			}
			spk := l.speaker
			if l.op == "0A" {
				v := strings.TrimLeft(strings.TrimLeft(l.voc, "zx"), "0123456789")
				spk = map[string]string{"st": "stern", "vi": "virgil", "me": "men"}[v[:2]]
			} else if v := strings.TrimLeft(strings.TrimLeft(l.voc, "zx"), "0123456789"); v[:2] != spk[:2] {
				t.Errorf("conv/%03d/%02d 語音字母 %q 與代碼 %q 前兩字母不符", g, l.idx, v[:2], spk)
			}
			en := map[string]string{"stern": "Stern", "virgil": "Virgil", "billy": "Billy", "tex": "Tex", "gwen": "Gwen", "miguel": "Miguel",
				"janet": "Janet", "janet1": "Janet", "beto": "Beto", "lyle": "Lyle", "tweedly": "Tweedly", "far": "Farhad", "muh": "Muhammed",
				"walt": "Walters", "pri": "Prideaux", "men": "Mendez", "air": "(player)", "airwing1": "(player)"}[spk]
			text := l.text
			if i := strings.Index(text, `\r`); i >= 0 {
				text = text[:i]
			}
			var sym []string
			for i := 0; i+1 < len(text); i++ {
				if text[i] == '$' && strings.IndexByte("SNCWPT", text[i+1]) >= 0 {
					sym = append(sym, text[i:i+2])
				}
			}
			s := ""
			if n, ok := vl[l.voc]; ok {
				s = strconv.Itoa(n)
			}
			want = append(want, fmt.Sprintf("conv/%03d/%02d\t%d\t%X\t%s\t%s\t%s\t%s\t%s\t%s", g, l.idx, segLen[g], end, l.op, spk, en, l.voc, s, strings.Join(sym, ",")))
		}
	}
	_, got := readTSV(t, idxPath)
	if len(got) != len(want) || len(got) != 2338 {
		t.Fatalf("結構表 %d 列，獨立計算 %d 列，要 2338", len(got), len(want))
	}
	for i := range want {
		if g := strings.Join(got[i], "\t"); g != want[i] {
			t.Fatalf("第 %d 列：\n表   %q\n期望 %q", i+2, g, want[i])
		}
	}
}

// voclistSamples 讀 VOCLIST.TRE：條目名（去路徑與 .VOC、小寫）→ 長度 − 33。
func voclistSamples(t *testing.T, p string) map[string]int {
	t.Helper()
	d, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	n := int(binary.LittleEndian.Uint32(d))
	out := map[string]int{}
	for k := 0; k < n; k++ {
		o := 8 + 74*k
		name := string(d[o+1 : o+66])
		if i := strings.IndexByte(name, 0); i >= 0 {
			name = name[:i]
		}
		if i := strings.LastIndexAny(name, `:\`); i >= 0 {
			name = name[i+1:]
		}
		ln := int(binary.LittleEndian.Uint32(d[o+70:]))
		out[strings.ToLower(strings.TrimSuffix(strings.ToUpper(name), ".VOC"))] = ln - 33
	}
	return out
}

// 第 2 項的固定值。
func TestSpeechIndexFixedValues(t *testing.T) {
	p := os.Getenv(scSpeechIndexEnv)
	if p == "" {
		t.Skip(scSpeechIndexEnv + " 未設定")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	x, err := ParseSpeechIndex(b)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint32{0x53, 0x88, 0xB8, 0xF8, 0x14F, 0x172, 0x1B8, 0x1F5, 0x25D, 0x298, 0x2E8, 0x32E}
	for i, e := range want {
		l, ok := x.Line(fmt.Sprintf("conv/011/%02d", i))
		if !ok || l.End != e || l.SegLen != 814 {
			t.Errorf("conv/011/%02d：%+v，要句尾 %X、段長 814", i, l, e)
		}
	}
	if _, ok := x.Line("conv/012/17"); ok {
		t.Error("conv/012/17（無聲格）在表內")
	}
	if l, ok := x.Line("conv/012/18"); !ok || l.End != 0x546 {
		t.Errorf("conv/012/18：%+v，要 end 546", l)
	}
	for _, c := range [][2]string{{"2st214", "stern"}, {"3me023", "men"}} {
		found := false
		for _, l := range x.Lines {
			if l.Voc == c[0] {
				found = true
				if l.Speaker != c[1] {
					t.Errorf("%s 說話人 %s，要 %s", c[0], l.Speaker, c[1])
				}
			}
		}
		if !found {
			t.Errorf("%s 不在表內", c[0])
		}
	}
}

// ---- 第 9 項：lint ----------------------------------------------------------

type lintFixture struct {
	index, en, glossary string
	cat                 []string // 每列 key\ttranslation\tsource（不含欄名）
}

func (f lintFixture) catalog() []byte {
	return []byte("key\ttranslation\tsource\n" + strings.Join(f.cat, "\n") + "\n")
}

func (f lintFixture) run(t *testing.T) SpeechLintReport {
	t.Helper()
	return LintSpeech([]byte(f.index), f.catalog(), []byte(f.en), []byte(f.glossary), subTestFont(t).Wide)
}

func baseLintFixture() lintFixture {
	idx := "key\tseg_len\tend\top\tspeaker\tspeaker_en\tvoc\tsamples\tsymbols\n" +
		"conv/011/00\t814\t53\t03\tstern\tStern\t2st139\t1000\t$N\n" +
		"conv/011/01\t814\t88\t04\tstern\tStern\t2st140\t500\t\n" +
		"conv/012/02\t900\t92\t03\tair\t(player)\t2ai431\t400\t\n" +
		"conv/057/02\t300\t60\t03\tlyle\tLyle\t2ly010\t400\t\n" +
		"conv/058/00\t300\t40\t03\tlyle\tLyle\t2ly011\t400\t\n" +
		"conv/059/00\t300\t40\t03\tlyle\tLyle\t2ly012\t400\t\n"
	en := "key\ttext\tsource\n" +
		"conv/011/00\tAll right, $N, the pilot.\t" + SpeechSourcePlain + "\n" +
		"conv/011/01\tStern here.\t" + SpeechSourcePlain + "\n" +
		"conv/012/02\tSo how are you, Miguel?\t" + SpeechSourcePlain + "\n" +
		"conv/057/02\tNO pay today.\t" + SpeechSourcePlain + "\n" +
		"conv/058/00\tTake the GBU-15.\t" + SpeechSourcePlain + "\n" +
		"conv/059/00\tTexas is big\\x0B.\t" + SpeechSourceCut + "\n"
	gl := "en\tzh-TW\tbasis\tnote\n"
	for _, r := range [][2]string{{"Stern", "史登"}, {"Gwen", "葛雯"}, {"Janet", "甲"}, {"Miguel", "乙"}, {"Tex", "丙"}, {"Billy", "比利"},
		{"Prideaux", "丁"}, {"Farhad", "戊"}, {"Muhammed", "己"}, {"Beto", "庚"}, {"Tweedly", "辛"}, {"Walters", "壬"}, {"Lyle", "萊爾"},
		{"Virgil", "癸"}, {"Ari", "一"}, {"Maxwell", "二"}, {"Wildcats", "三"}, {"Wildcat Base", "四"}, {"Jackals", "五"}, {"Selim's", "六"},
		{"Strike Base", "七"}, {"Mission Pool", "八"}, {"Prideaux's niche", "九"}, {"Maverick", "十"}, {"Durandal", "中"},
		{"GBU-15", "中文（GBU-15）"}, {"LAU-3", "字"}, {"NO", "否"}} {
		gl += r[0] + "\t" + r[1] + "\t攻略\t\n"
	}
	return lintFixture{index: idx, en: en, glossary: gl, cat: []string{
		"conv/011/00\t好，$N，測試一。\tCONV.PAK 第 11 段第 0 句；已觀察",
		"conv/011/01\t史登，測試二。\tCONV.PAK 第 11 段第 1 句；未觀察",
		"conv/012/02\t乙，測試三？\tCONV.PAK 第 12 段第 2 句；未觀察",
		"conv/057/02\t今天沒有錢。\tCONV.PAK 第 57 段第 2 句；未觀察",
		"conv/058/00\t拿中文（ＧＢＵ－１５）。\tCONV.PAK 第 58 段第 0 句；未觀察",
		"conv/059/00\t測試四。\tCONV.PAK 第 59 段第 0 句；未觀察",
		"speaker/stern\t史登\t說話人代碼 stern；glossary",
		"speaker/air\t$C\t說話人代碼 air；玩家呼號",
		"speaker/lyle\t萊爾\t說話人代碼 lyle；glossary",
	}}
}

func TestLintSpeechPositive(t *testing.T) {
	rep := baseLintFixture().run(t)
	if !rep.OK() {
		t.Fatalf("正向例報錯：\n%s", strings.Join(rep.Errors, "\n"))
	}
	if len(rep.Untranslated) != 0 {
		t.Fatalf("漏譯 %v", rep.Untranslated)
	}
}

func TestLintSpeechErrors(t *testing.T) {
	set := func(f *lintFixture, key, row string) {
		for i, r := range f.cat {
			if strings.HasPrefix(r, key+"\t") {
				f.cat[i] = row
				return
			}
		}
		f.cat = append(f.cat, row)
	}
	cases := []struct {
		name string
		mut  func(f *lintFixture)
		want string
	}{
		{"孤兒 key", func(f *lintFixture) {
			set(f, "conv/011/05", "conv/011/05\t測試\tCONV.PAK 第 11 段第 5 句；未觀察")
		}, "孤兒"},
		{"欄數不符", func(f *lintFixture) {
			set(f, "conv/011/01", "conv/011/01\t測試二\tCONV.PAK 第 11 段第 1 句；未觀察\t")
		}, "欄數"},
		{"尾欄空被吃掉", func(f *lintFixture) { f.index = strings.Replace(f.index, "2st140\t500\t\n", "2st140\t500\n", 1) }, "欄數"},
		{"speaker 代碼不在結構表", func(f *lintFixture) { set(f, "speaker/gwen", "speaker/gwen\t葛雯\t說話人代碼 gwen；glossary") }, "孤兒"},
		{"符號集合不符", func(f *lintFixture) {
			set(f, "conv/011/00", "conv/011/00\t好，測試一。\tCONV.PAK 第 11 段第 0 句；已觀察")
		}, "代換符號"},
		{"說話人 key 含 $C 以外的符號", func(f *lintFixture) {
			set(f, "speaker/stern", "speaker/stern\t史登$N\t說話人代碼 stern；glossary")
		}, "只允許單一 $C"},
		{"$L", func(f *lintFixture) {
			set(f, "conv/011/01", "conv/011/01\t$L測試二\tCONV.PAK 第 11 段第 1 句；未觀察")
		}, "只能接"},
		{"半形字", func(f *lintFixture) {
			set(f, "conv/011/01", "conv/011/01\t測試a\tCONV.PAK 第 11 段第 1 句；未觀察")
		}, "半形字"},
		{"反斜線", func(f *lintFixture) {
			set(f, "conv/011/01", "conv/011/01\t測試\\二\tCONV.PAK 第 11 段第 1 句；未觀察")
		}, "反斜線"},
		{"source 格式", func(f *lintFixture) {
			set(f, "conv/011/01", "conv/011/01\t測試二\tCONV.PAK 第 011 段第 1 句；未觀察")
		}, "source"},
		{"說話人譯名與 glossary 不同", func(f *lintFixture) {
			set(f, "speaker/stern", "speaker/stern\t史丹\t說話人代碼 stern；glossary")
		}, "不等於 glossary"},
		{"玩家代碼不是 $C", func(f *lintFixture) { set(f, "speaker/air", "speaker/air\t玩家\t說話人代碼 air；玩家呼號") }, "要是 $C"},
		{"譯名不一致", func(f *lintFixture) {
			set(f, "conv/011/01", "conv/011/01\t測試二。\tCONV.PAK 第 11 段第 1 句；未觀察")
		}, "英文含 \"Stern\""},
		{"超過 4 行", func(f *lintFixture) {
			set(f, "conv/011/01", "conv/011/01\t史登"+strings.Repeat("測", 160)+"\tCONV.PAK 第 11 段第 1 句；未觀察")
		}, "超過 4 行"},
		{"en key 集合不符", func(f *lintFixture) { f.en = strings.Replace(f.en, "conv/011/01\t", "conv/011/02\t", 1) }, "key 集合或列順序"},
		{"en 列順序不符", func(f *lintFixture) {
			ls := strings.Split(f.en, "\n")
			ls[1], ls[2] = ls[2], ls[1]
			f.en = strings.Join(ls, "\n")
		}, "key 集合或列順序"},
		{"en source 不在列舉", func(f *lintFixture) {
			f.en = strings.Replace(f.en, "Stern here.\t"+SpeechSourcePlain, "Stern here.\t人工聽寫", 1)
		}, "不在列舉"},
		{"en 非法跳脫", func(f *lintFixture) { f.en = strings.Replace(f.en, `\x0B`, `\q`, 1) }, "非法跳脫"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := baseLintFixture()
			f.cat = append([]string(nil), f.cat...)
			c.mut(&f)
			rep := f.run(t)
			hit := false
			for _, e := range rep.Errors {
				hit = hit || strings.Contains(e, c.want)
			}
			if !hit {
				t.Fatalf("沒有含 %q 的錯誤：%v", c.want, rep.Errors)
			}
		})
	}
	// 缺譯：列出、不判失敗。
	f := baseLintFixture()
	f.cat = f.cat[1:]
	rep := f.run(t)
	if !rep.OK() || len(rep.Untranslated) != 1 || rep.Untranslated[0] != "conv/011/00" {
		t.Fatalf("缺譯：%v %v", rep.Errors, rep.Untranslated)
	}
	// 超過 2 行：列出、不判失敗。
	f = baseLintFixture()
	f.cat = append([]string(nil), f.cat...)
	set(&f, "conv/011/01", "conv/011/01\t史登"+strings.Repeat("測", 90)+"\tCONV.PAK 第 11 段第 1 句；未觀察")
	rep = f.run(t)
	if !rep.OK() || len(rep.Over2) != 1 {
		t.Fatalf("超過 2 行：%v %v", rep.Errors, rep.Over2)
	}
}

// 正向例的三個要點各自單獨驗：NO pay（介面字排除）、GBU-15（全形轉寫後比對）、Texas（整字邊界）。
func TestLintSpeechNameRules(t *testing.T) {
	if !wordHit("Take the GBU-15.", "GBU-15") || wordHit("Texas is big", "Tex") || !wordHit("Stern, here", "Stern") || wordHit("Sterns", "Stern") {
		t.Fatal("整字邊界")
	}
	for _, term := range speechNameTerms {
		if term == "NO" || term == "SAVE" || term == "catalog" || term == "Package" {
			t.Fatalf("檢查集含介面字或普通名詞 %q", term)
		}
	}
	if FullwidthText("雷射導引炸彈（GBU-15）") != "雷射導引炸彈（ＧＢＵ－１５）" {
		t.Fatal("全形轉寫")
	}
}
