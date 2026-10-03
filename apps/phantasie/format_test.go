package phantasie

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// 共用向量（docs/spec/003 §7.3）放在 phantasie 專案的 tests/vectors，不在本 repo。
// 以 tools/go.sh 執行時用 DOSGOLEM_EXTRA_MOUNT 唯讀掛到容器的 /vectors：
//
//	DOSGOLEM_EXTRA_MOUNT=<phantasie>/tests/vectors:/vectors:ro tools/go.sh test ./apps/phantasie -run Format -v
//
// 沒掛載時向量測試 SKIP（SKIP 不算驗收）；其餘 Go 專屬單元測試不依賴向量。
const feVectorDir = "/vectors"

// feWideTest 是向量測試用的全形判定（碼點範圍）。向量的中文都在 U+2E80 以上，
// 正式程式的判定來自字型寬度表（fontwide.go），不用碼點範圍。
func feWideTest(r rune) bool { return r >= 0x2E80 }

// feReadVectors 讀一個向量檔，回傳資料列（欄以 TAB 切，不含欄名列）。
// 檔案不存在時 SKIP 並說明；欄數不符、沒有資料列都是失敗。
func feReadVectors(t *testing.T, name string, cols int) [][]string {
	t.Helper()
	path := feVectorDir + "/" + name
	if _, err := os.Stat(feVectorDir + "/parse.tsv"); err != nil {
		t.Skipf("沒有共用向量（%s 不存在：%v）；以 DOSGOLEM_EXTRA_MOUNT=<phantasie>/tests/vectors:/vectors:ro 掛載後重跑。SKIP 不算驗收", feVectorDir+"/parse.tsv", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀 %s：%v", path, err)
	}
	var rows [][]string
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 || line == "" {
			continue // 欄名列、檔尾空行
		}
		f := strings.Split(line, "\t")
		if len(f) != cols {
			t.Fatalf("%s 第 %d 列有 %d 欄，要 %d 欄：%q", name, i+1, len(f), cols, line)
		}
		rows = append(rows, f)
	}
	if len(rows) == 0 {
		t.Fatalf("%s 沒有資料列", name)
	}
	return rows
}

// feVecArgs 解析向量的 args 欄（以 | 分隔）：w:XXXX 是 16 位元字組（十六進位），
// s:文字 是原樣保留的 ASCII 字串引數，t:文字 是譯文引數；值內的 _ 代表半形空白。
//
// 向量裡的 %s 只寫字串、沒有指標字組，但 FormatEnglish、FormatTarget 的 %s 會從 words 取走 1 個字組
// （原版堆疊上 %s 的引數是指標，003 §5.2「引數讀取」），所以每個 s:、t: 引數在 words 補一個佔位字組 0。
func feVecArgs(t *testing.T, cell string, target bool) (words []uint16, en []string, tg []TargetStr) {
	t.Helper()
	for _, tok := range strings.Split(cell, "|") {
		if tok == "" {
			continue
		}
		kind, val, ok := strings.Cut(tok, ":")
		if !ok {
			t.Fatalf("向量引數 %q 沒有種類前綴", tok)
		}
		switch kind {
		case "w":
			v, err := strconv.ParseUint(val, 16, 16)
			if err != nil {
				t.Fatalf("向量引數 %q：%v", tok, err)
			}
			words = append(words, uint16(v))
		case "s", "t":
			val = strings.ReplaceAll(val, "_", " ")
			words = append(words, 0)
			if target {
				tg = append(tg, TargetStr{Text: val, Translated: kind == "t"})
			} else {
				if kind == "t" {
					t.Fatalf("英文向量不應有譯文引數 %q", tok)
				}
				en = append(en, val)
			}
		default:
			t.Fatalf("不認得的向量引數 %q", tok)
		}
	}
	return
}

// feConvAbbrev 是 parse.tsv 的轉換序縮寫：旗標 - 在最前，寬度、精度（.P，P 可為 0）、長度 l、類型。
func feConvAbbrev(c *Conv) string {
	var b strings.Builder
	if c.Left {
		b.WriteByte('-')
	}
	if c.Width > 0 {
		b.WriteString(strconv.Itoa(c.Width))
	}
	if c.Prec >= 0 {
		b.WriteByte('.')
		b.WriteString(strconv.Itoa(c.Prec))
	}
	if c.Long {
		b.WriteByte('l')
	}
	b.WriteByte(c.Type)
	return b.String()
}

func feBracket(s string) string { return "[" + strings.ReplaceAll(s, " ", "_") + "]" }

func TestFormatVectorsParse(t *testing.T) {
	rows := feReadVectors(t, "parse.tsv", 2)
	for _, r := range rows {
		fmtStr, want := r[0], r[1]
		var got string
		segs, err := ParseFormat(fmtStr)
		switch {
		case err != nil:
			got = "ERR"
		default:
			var parts []string
			for _, s := range segs {
				if s.Conv != nil {
					parts = append(parts, feConvAbbrev(s.Conv))
				}
			}
			if got = strings.Join(parts, ","); got == "" {
				got = "(none)"
			}
			if HasConversions(segs) != (got != "(none)") {
				t.Errorf("parse %q：HasConversions 與轉換序 %q 不一致", fmtStr, got)
			}
		}
		if got != want {
			t.Errorf("parse %q：得 %q，要 %q", fmtStr, got, want)
		}
	}
	t.Logf("parse.tsv：%d 列", len(rows))
}

func TestFormatVectorsEnglish(t *testing.T) {
	rows := feReadVectors(t, "english.tsv", 3)
	for _, r := range rows {
		fmtStr, args, want := r[0], r[1], r[2]
		words, strs, _ := feVecArgs(t, args, false)
		got, err := FormatEnglish(fmtStr, words, strs)
		if err != nil {
			t.Errorf("english %q %q：%v", fmtStr, args, err)
			continue
		}
		if g := feBracket(got); g != want {
			t.Errorf("english %q %q：得 %s，要 %s", fmtStr, args, g, want)
		}
	}
	t.Logf("english.tsv：%d 列", len(rows))
}

func TestFormatVectorsTarget(t *testing.T) {
	rows := feReadVectors(t, "target.tsv", 4)
	for _, r := range rows {
		tpl, args, want := r[0], r[1], r[2]
		wantH, err := strconv.Atoi(r[3])
		if err != nil {
			t.Fatalf("target %q 的 h 欄 %q：%v", tpl, r[3], err)
		}
		words, _, strs := feVecArgs(t, args, true)
		got, err := FormatTarget(tpl, words, strs, feWideTest)
		if err != nil {
			t.Errorf("target %q %q：%v", tpl, args, err)
			continue
		}
		if g := feBracket(got); g != want {
			t.Errorf("target %q %q：得 %s，要 %s", tpl, args, g, want)
		}
		if h := HalfWidth(got, feWideTest); h != wantH {
			t.Errorf("target %q %q：HalfWidth 得 %d，要 %d", tpl, args, h, wantH)
		}
	}
	t.Logf("target.tsv：%d 列", len(rows))
}

// 以下是 Go 專屬的單元測試，不依賴向量檔，期望值都是字面值。

func TestFormatParseDetails(t *testing.T) {
	segs, err := ParseFormat("100%%%d")
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 2 || segs[0].Conv != nil || segs[0].Lit != "100%" || segs[1].Conv == nil {
		t.Fatalf("100%%%%%%d 應得字面 \"100%%\" 加一個轉換，得 %+v", segs)
	}

	segs, err = ParseFormat("")
	if err != nil || len(segs) != 0 || HasConversions(segs) {
		t.Errorf("空字串應得零個片段：%+v %v", segs, err)
	}

	segs, err = ParseFormat("%%")
	if err != nil || len(segs) != 1 || segs[0].Lit != "%" || HasConversions(segs) {
		t.Errorf("%%%% 應得字面 %%：%+v %v", segs, err)
	}

	// UTF-8 譯文字面原樣保留，轉換規格正確讀出。
	segs, err = ParseFormat("攻擊%3u點")
	if err != nil || len(segs) != 3 {
		t.Fatalf("攻擊%%3u點：%+v %v", segs, err)
	}
	if segs[0].Lit != "攻擊" || segs[2].Lit != "點" {
		t.Errorf("字面片段錯：%+v", segs)
	}
	if c := *segs[1].Conv; c != (Conv{Width: 3, Prec: -1, Type: 'u'}) {
		t.Errorf("%%3u 的 Conv = %+v", c)
	}

	// .0 的精度是 0（截成空字串），不是「沒寫」。
	segs, err = ParseFormat("%.0s%-d%ld")
	if err != nil || len(segs) != 3 {
		t.Fatalf("%v %v", segs, err)
	}
	want := []Conv{{Prec: 0, Type: 's'}, {Left: true, Prec: -1, Type: 'd'}, {Long: true, Prec: -1, Type: 'd'}}
	for i, w := range want {
		if *segs[i].Conv != w {
			t.Errorf("第 %d 個轉換：得 %+v，要 %+v", i, *segs[i].Conv, w)
		}
	}
	if segs[0].Conv == segs[1].Conv {
		t.Error("各片段的 Conv 不應共用同一個指標")
	}

	segs, _ = ParseFormat("%s%d%-5s%%s%c")
	if n := CountS(segs); n != 2 {
		t.Errorf("CountS = %d，要 2（%%%%s 是字面）", n)
	}
	if !HasConversions(segs) {
		t.Error("HasConversions 應為真")
	}
}

func TestFormatParseErrors(t *testing.T) {
	bad := []string{
		"%", "100%", "%-", "%5", "%1", "%.", "%l",
		"%.s", "%--d", "%l%", "%ld%", "%ls", "%lc", "%lld",
		"%x", "%X", "%o", "%e", "%f", "%g", "%p", "%n", "%i",
		"%0d", "%05d", "%-05d", "%+d", "% d", "%#x", "%*d", "%hd", "%2$s", "%5%", "%-%", "%.3%",
		"%1000d", "%.1000d", "%99999999999999999999d", "%.99999999999999999999d",
	}
	for _, f := range bad {
		if segs, err := ParseFormat(f); err == nil {
			t.Errorf("ParseFormat(%q) 應回 error，得 %+v", f, segs)
		}
		if _, err := FormatEnglish(f, []uint16{1, 2, 3}, []string{"x"}); err == nil {
			t.Errorf("FormatEnglish(%q) 應回 error", f)
		}
		if _, err := FormatTarget(f, []uint16{1, 2, 3}, []TargetStr{{Text: "x"}}, feWideTest); err == nil {
			t.Errorf("FormatTarget(%q) 應回 error", f)
		}
	}
	// 上限本身（999）可接受。
	if _, err := ParseFormat("%999d%.999s"); err != nil {
		t.Errorf("寬度、精度 999 應可接受：%v", err)
	}
}

func TestFormatEnglishSpecExamples(t *testing.T) {
	// 003 §7.1 的字面期望值。
	for _, tc := range []struct {
		w    uint16
		want string
	}{{5, "5   "}, {1234, "123 "}, {0xFFF4, "-12 "}} {
		got, err := FormatEnglish("%-4.3d", []uint16{tc.w}, nil)
		if err != nil || got != tc.want {
			t.Errorf("%%-4.3d 對 %d：得 %q（%v），要 %q", int16(tc.w), got, err, tc.want)
		}
	}
	// %s 取走 1 個字組：指標字組之後才是 %d。
	got, err := FormatEnglish("%s%d", []uint16{0x1234, 7}, []string{"X"})
	if err != nil || got != "X7" {
		t.Errorf("%%s%%d：得 %q（%v），要 \"X7\"", got, err)
	}
	// 多餘的字組與字串不是錯誤（引數區固定 12 個字組）。
	if got, err = FormatEnglish("%d", []uint16{1, 2, 3}, []string{"x"}); err != nil || got != "1" {
		t.Errorf("多餘引數：得 %q（%v）", got, err)
	}
	// 沒有轉換的格式字串不需要任何引數。
	if got, err = FormatEnglish("a%%b", nil, nil); err != nil || got != "a%b" {
		t.Errorf("純字面：得 %q（%v）", got, err)
	}
}

func TestFormatEnglishCharKeepsNUL(t *testing.T) {
	// %c 取低位元組；為 0 時保留 NUL，不在此函式截斷（呼叫端截到第一個 NUL）。
	got, err := FormatEnglish("a%cb", []uint16{0x0100}, nil)
	if err != nil || got != "a\x00b" {
		t.Errorf("a%%cb（低位元組 0）：得 %q（%v），要 %q", got, err, "a\x00b")
	}
	// NUL 佔一個位元組的寬度。
	if got, err = FormatEnglish("%3c", []uint16{0}, nil); err != nil || got != "  \x00" {
		t.Errorf("%%3c（0）：得 %q（%v）", got, err)
	}
	// 高位元組不參與；%c 忽略精度。
	if got, err = FormatEnglish("%.0c", []uint16{0x0141}, nil); err != nil || got != "A" {
		t.Errorf("%%.0c：得 %q（%v），要 \"A\"", got, err)
	}
	// 位元組串：>= 0x80 的 %c 原樣保留為單一位元組，不是 UTF-8 編碼。
	if got, err = FormatEnglish("%c", []uint16{0x00E9}, nil); err != nil || got != "\xE9" {
		t.Errorf("%%c（0xE9）：得 %q（%v）", got, err)
	}
}

func TestFormatEnglishShortArgs(t *testing.T) {
	cases := []struct {
		name  string
		f     string
		words []uint16
		strs  []string
	}{
		{"%d 沒有字組", "%d", nil, nil},
		{"第二個 %d 不足", "%d%d", []uint16{1}, nil},
		{"%ld 只有一個字組", "%ld", []uint16{1}, nil},
		{"%lu 沒有字組", "%lu", nil, nil},
		{"%c 沒有字組", "%c", nil, nil},
		{"%s 沒有指標字組", "%s", nil, []string{"A"}},
		{"%s 沒有字串", "%s", []uint16{0}, nil},
		{"第二個 %s 沒有字串", "%s%s", []uint16{0, 0}, []string{"A"}},
		{"%s 之後的 %d 不足", "%s%d", []uint16{0}, []string{"A"}},
	}
	for _, tc := range cases {
		if got, err := FormatEnglish(tc.f, tc.words, tc.strs); err == nil {
			t.Errorf("%s：應回 error，得 %q", tc.name, got)
		}
	}
}

func TestFormatEnglishLongWordOrder(t *testing.T) {
	// 兩個字組，低字組在前。
	for _, tc := range []struct {
		f     string
		words []uint16
		want  string
	}{
		{"%ld", []uint16{0x0001, 0x0002}, "131073"}, // 0x00020001
		{"%lu", []uint16{0x0001, 0x0002}, "131073"}, // 0x00020001
		{"%ld", []uint16{0x0002, 0x0001}, "65538"},  // 0x00010002，高低對調會得到另一個值
		{"%ld", []uint16{0x0000, 0x8000}, "-2147483648"},
		{"%lu", []uint16{0x0000, 0x8000}, "2147483648"},
		{"%ld", []uint16{0xFFFF, 0xFFFF}, "-1"},
		{"%lu", []uint16{0xFFFF, 0x0000}, "65535"},
		{"%ld%d", []uint16{0x0001, 0x0000, 0x0007}, "17"}, // %ld 用兩個字組，%d 接著取第三個
	} {
		got, err := FormatEnglish(tc.f, tc.words, nil)
		if err != nil || got != tc.want {
			t.Errorf("%s %v：得 %q（%v），要 %q", tc.f, tc.words, got, err, tc.want)
		}
	}
	// 16 位元有號與無號。
	if got, _ := FormatEnglish("%d|%u", []uint16{0x8000, 0x8000}, nil); got != "-32768|32768" {
		t.Errorf("%%d|%%u：得 %q", got)
	}
}

func TestFormatEnglishPadAndPrecision(t *testing.T) {
	for _, tc := range []struct {
		f     string
		words []uint16
		strs  []string
		want  string
	}{
		{"%-5s|", []uint16{0}, []string{"AB"}, "AB   |"},
		{"%5s|", []uint16{0}, []string{"AB"}, "   AB|"},
		{"%3.1s", []uint16{0}, []string{"XYZ"}, "  X"},
		{"%.0s|", []uint16{0}, []string{"XYZ"}, "|"},
		{"%2s", []uint16{0}, []string{"XYZ"}, "XYZ"}, // 寬度不截斷
		{"%.0d|", []uint16{5}, nil, "|"},
		{"%6.3u", []uint16{1234}, nil, "   123"},
		{"%.2d", []uint16{0xFFF4}, nil, "-1"}, // 精度含負號
	} {
		got, err := FormatEnglish(tc.f, tc.words, tc.strs)
		if err != nil || got != tc.want {
			t.Errorf("%s：得 %q（%v），要 %q", tc.f, got, err, tc.want)
		}
	}
}

func TestFormatTargetCharTakesOneCell(t *testing.T) {
	for _, tc := range []struct {
		f     string
		words []uint16
		want  string
		h     int
	}{
		{"%c", []uint16{0x0041}, "A ", 2},             // 003 §7.2 字面期望值
		{"%c|", []uint16{0x0041}, "A |", 3},           // 其後欄位的位置依賴這一格
		{"%c%c", []uint16{0x0041, 0x0042}, "A B ", 4}, // 兩個 %c 兩格
		{"%3c", []uint16{0x0041}, "     A", 6},        // 有寬度：2W h，預設補左側
		{"%-3c", []uint16{0x0041}, "A     ", 6},       // - 補右側
		{"%-c", []uint16{0x0041}, "A ", 2},            // 無寬度時一律靠左
		{"%c", []uint16{0x0000}, "\x00 ", 2},          // NUL 也佔一格，截斷由呼叫端負責
		{"%.0c", []uint16{0x0041}, "A ", 2},           // %c 忽略精度
		{"%1c", []uint16{0x0141}, " A", 2},            // 寬度 1 = 2 h，補左側；高位元組不參與
		{"HP %c%d", []uint16{0x002B, 5}, "HP + 5", 6}, // 一格之後接數字
		{"+%2c-", []uint16{0x0058}, "+   X-", 6},      // 寬度 2 = 4 h
		{"%c", []uint16{0x007E}, "~ ", 2},             // 上限的可印字元
	} {
		got, err := FormatTarget(tc.f, tc.words, nil, feWideTest)
		if err != nil || got != tc.want {
			t.Errorf("%s：得 %q（%v），要 %q", tc.f, got, err, tc.want)
			continue
		}
		if h := HalfWidth(got, feWideTest); h != tc.h {
			t.Errorf("%s：HalfWidth 得 %d，要 %d", tc.f, h, tc.h)
		}
	}
	// 低位元組 >= 0x80 不能成為譯文字串中的 ASCII 字元。
	if got, err := FormatTarget("%c", []uint16{0x00E9}, nil, feWideTest); err == nil {
		t.Errorf("%%c（0xE9）應回 error，得 %q", got)
	}
}

func TestFormatTargetStringPrecision(t *testing.T) {
	tr := func(s string) []TargetStr { return []TargetStr{{Text: s, Translated: true}} }
	orig := func(s string) []TargetStr { return []TargetStr{{Text: s}} }
	one := []uint16{0}
	for _, tc := range []struct {
		name string
		f    string
		strs []TargetStr
		want string
		h    int
	}{
		// 003 §7.2 的字面期望值：譯文與原樣保留的 ASCII 對同一個格式的差異。
		{"譯文 %-10.9s", "%-10.9s", tr("哥布林"), "哥布林" + strings.Repeat(" ", 14), 20},
		{"原樣 %-10.9s", "%-10.9s", orig("ABCDEFGHIJ"), "ABCDEFGHI" + strings.Repeat(" ", 11), 20},
		// 譯文的精度是 2P h：ASCII 字元的譯文也一樣，原樣保留的 ASCII 則是 P 個字元。
		{"譯文 ASCII %.2s", "%.2s", tr("ABCDEFG"), "ABCD", 4},
		{"原樣 ASCII %.2s", "%.2s", orig("ABCDEFG"), "AB", 2},
		{"譯文 %.2s", "%.2s", tr("哥布林"), "哥布", 4},
		{"譯文 %.1s 放得下一個全形字", "%.1s", tr("哥布林"), "哥", 2},
		// 整字前綴：放不下下一個字就停，不跳過它去收後面較窄的字。
		{"譯文 %.1s 全形字放不下", "%.1s", tr("A哥B"), "A", 1},
		{"譯文 %.0s", "%.0s", tr("哥布林"), "", 0},
		{"原樣 %.0s", "%.0s", orig("ABC"), "", 0},
		// 003 §7.2 的 %-2.1s|：精度 1 = 2 h 保留「哥」，寬度 2 = 4 h 補 2 h。
		{"譯文 %-2.1s|", "%-2.1s|", tr("哥布林"), "哥  |", 5},
		{"譯文 %2s", "%2s", tr("哥"), "  哥", 4},
		// 補白方向與寬度（寬度 W = 2W h）。
		{"譯文 %-6s", "%-6s", tr("哥布林"), "哥布林" + strings.Repeat(" ", 6), 12},
		{"譯文 %6s", "%6s", tr("哥布林"), strings.Repeat(" ", 6) + "哥布林", 12},
		{"原樣 %-5s|", "%-5s|", orig("AB"), "AB" + strings.Repeat(" ", 8) + "|", 11},
		{"超寬不截斷 %2s", "%2s", tr("哥布林"), "哥布林", 6},
		{"精度大於長度不補", "%.9s|", orig("AB"), "AB|", 3},
		// 譯文可含半形字，寬度照字型判定。
		{"混合譯文 %-4s", "%-4s", tr("A哥B"), "A哥B" + "    ", 8},
	} {
		got, err := FormatTarget(tc.f, one, tc.strs, feWideTest)
		if err != nil || got != tc.want {
			t.Errorf("%s：得 %q（%v），要 %q", tc.name, got, err, tc.want)
			continue
		}
		if h := HalfWidth(got, feWideTest); h != tc.h {
			t.Errorf("%s：HalfWidth 得 %d，要 %d", tc.name, h, tc.h)
		}
	}
}

func TestFormatTargetNumbers(t *testing.T) {
	for _, tc := range []struct {
		f     string
		words []uint16
		want  string
		h     int
	}{
		{"%4u", []uint16{5}, strings.Repeat(" ", 7) + "5", 8},         // 003 §7.2：7 個空白加 5
		{"%-4.3d", []uint16{1234}, "123" + strings.Repeat(" ", 5), 8}, // 003 §7.2：精度是字元數，寬度是 2W h
		{"%-4.3d", []uint16{5}, "5" + strings.Repeat(" ", 7), 8},
		{"%.2d", []uint16{0xFFF4}, "-1", 2}, // 精度含負號
		{"%3.2d", []uint16{123}, strings.Repeat(" ", 4) + "12", 6},
		{"%d", []uint16{0xFFFF}, "-1", 2}, // 沒有寬度：不補白
		{"%lu", []uint16{0xFFFF, 0xFFFF}, "4294967295", 10},
		{"%5ld", []uint16{0x0000, 0x8000}, "-2147483648", 11}, // 11 h 超過 2W = 10 h，不截斷也不補白
		{"攻擊%3u", []uint16{5}, "攻擊" + strings.Repeat(" ", 5) + "5", 10},
	} {
		got, err := FormatTarget(tc.f, tc.words, nil, feWideTest)
		if err != nil || got != tc.want {
			t.Errorf("%s %v：得 %q（%v），要 %q", tc.f, tc.words, got, err, tc.want)
			continue
		}
		if h := HalfWidth(got, feWideTest); h != tc.h {
			t.Errorf("%s %v：HalfWidth 得 %d，要 %d", tc.f, tc.words, h, tc.h)
		}
	}
}

func TestFormatTargetConsumesWordsInOrder(t *testing.T) {
	// %s 的指標字組、%ld 的兩個字組、%c 的字組依序消耗；%s 字串依出現順序取。
	got, err := FormatTarget("%s %ld %c %s", []uint16{0xAAAA, 1, 2, 0x0042, 0xBBBB},
		[]TargetStr{{Text: "生命", Translated: true}, {Text: "KOBOLD"}}, feWideTest)
	if err != nil || got != "生命 131073 B  KOBOLD" {
		t.Errorf("得 %q（%v）", got, err)
	}
}

func TestFormatTargetErrors(t *testing.T) {
	s := []TargetStr{{Text: "A"}}
	if _, err := FormatTarget("%d", []uint16{1}, nil, nil); err == nil {
		t.Error("wide 為 nil 應回 error")
	}
	for _, tc := range []struct {
		name  string
		f     string
		words []uint16
		strs  []TargetStr
	}{
		{"字組不足", "%d%d", []uint16{1}, nil},
		{"%ld 字組不足", "%ld", []uint16{1}, nil},
		{"%c 字組不足", "%c", nil, nil},
		{"%s 沒有指標字組", "%s", nil, s},
		{"%s 沒有字串", "%s", []uint16{0}, nil},
		{"第二個 %s 沒有字串", "%s%s", []uint16{0, 0}, s},
		{"未支援的規格", "%x", []uint16{1}, nil},
	} {
		if got, err := FormatTarget(tc.f, tc.words, tc.strs, feWideTest); err == nil {
			t.Errorf("%s：應回 error，得 %q", tc.name, got)
		}
	}
	// 沒有轉換的模板不需要任何引數。
	if got, err := FormatTarget("生命100%%", nil, nil, feWideTest); err != nil || got != "生命100%" {
		t.Errorf("純字面：得 %q（%v）", got, err)
	}
}

func TestFormatHalfWidth(t *testing.T) {
	if got := HalfWidth("A中", feWideTest); got != 3 {
		t.Errorf("A中 = %d，要 3", got)
	}
	if got := HalfWidth("", feWideTest); got != 0 {
		t.Errorf("空字串 = %d，要 0", got)
	}
	if got := HalfWidth("A中", nil); got != 2 {
		t.Errorf("wide 為 nil：A中 = %d，要 2（每字元 1 h）", got)
	}
	// 寬度由傳入的判定決定，不看碼點範圍：這個判定只認 U+2026 與「中」。
	only := func(r rune) bool { return r == '…' || r == '中' }
	for _, tc := range []struct {
		s    string
		want int
	}{{"…", 2}, {"…A", 3}, {"哥", 1}, {"中中", 4}} {
		if got := HalfWidth(tc.s, only); got != tc.want {
			t.Errorf("HalfWidth(%q) = %d，要 %d", tc.s, got, tc.want)
		}
	}
	// FormatTarget 的補白與截斷也走同一個判定。
	got, err := FormatTarget("%-2s|", []uint16{0}, []TargetStr{{Text: "…", Translated: true}}, only)
	if err != nil || got != "…  |" {
		t.Errorf("得 %q（%v）", got, err)
	}
}
