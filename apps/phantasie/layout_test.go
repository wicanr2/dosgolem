package phantasie

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// 本檔驗證版面與疊字產生（docs/spec/001 §6、§7）：layoutLine、splitRuns、mergeRanges、intersects、
// buildStamps、availH。期望值是依規格推導出來的字面值。測試輔助函式一律加前綴 ly。

// lyWide 是測試字型：CJK 統一表意文字（U+4E00 至 U+9FFF）與 U+2026 是 2 h，其餘 1 h。
// U+2026 不在 CJK 區間，U+FF71（半形片假名）在 U+2E80 以上卻是 1 h，
// 兩者合起來證明寬度來自傳進去的 WideFunc，不是碼點範圍。
func lyWide(r rune) bool { return (r >= 0x4E00 && r <= 0x9FFF) || r == 0x2026 }

// lyNoEllipsis 同 lyWide，但 U+2026 在這個字型是 1 h。
func lyNoEllipsis(r rune) bool { return r >= 0x4E00 && r <= 0x9FFF }

func ly(n int) string { return strings.Repeat(" ", n) }

// lyWidthOf 以測試自己的算法量字串的半格寬度（不用被測的 hw）。
func lyWidthOf(s string, wide WideFunc) int {
	n := 0
	for _, r := range s {
		if wide != nil && wide(r) {
			n += 2
		} else {
			n++
		}
	}
	return n
}

type lyLineCase struct {
	name    string
	in      string
	center  bool
	avail   int
	wide    WideFunc // nil 表示 lyWide
	nilWide bool     // 為真時傳 nil 給 layoutLine
	want    string
	trunc   bool
}

func TestLayoutLine(t *testing.T) {
	cases := []lyLineCase{
		// 照排：不足處尾端補 ASCII 空白，補到剛好 avail h。
		{name: "ASCII 補白", in: "AB", avail: 5, want: "AB" + ly(3)},
		{name: "全形補白", in: "新遊戲", avail: 16, want: "新遊戲" + ly(10)},
		{name: "剛好滿不補白不截斷", in: "新遊戲", avail: 6, want: "新遊戲"},
		{name: "ASCII 剛好滿不算截斷", in: "ABCDE", avail: 5, want: "ABCDE"},
		{name: "全形半形混排", in: "A戰B", avail: 6, want: "A戰B" + ly(2)},
		{name: "非置中保留譯文自己的開頭空白", in: "  戰士", avail: 10, want: "  戰士" + ly(4)},
		{name: "非置中不去尾端空白", in: "戰士  ", avail: 10, want: "戰士  " + ly(4)},
		{name: "<blank> 的空譯文補滿空白", in: "", avail: 6, want: ly(6)},
		{name: "avail 為 0 的空譯文", in: "", avail: 0, want: ""},

		// 置中：去掉頭尾空白後左補 floor((avail-w)/2) h。
		{name: "置中 floor：差 5 左補 2", in: "中文", center: true, avail: 9, want: ly(2) + "中文" + ly(3)},
		{name: "置中 floor：差 3 左補 1", in: "中文", center: true, avail: 7, want: ly(1) + "中文" + ly(2)},
		{name: "置中：差 4 左右各 2", in: "中文", center: true, avail: 8, want: ly(2) + "中文" + ly(2)},
		{name: "置中：剛好滿", in: "戰士", center: true, avail: 4, want: "戰士"},
		{name: "置中 ASCII", in: "A", center: true, avail: 4, want: ly(1) + "A" + ly(2)},
		{name: "置中去掉頭尾空白", in: "  戰士  ", center: true, avail: 10, want: ly(3) + "戰士" + ly(3)},
		{name: "置中去空白後 floor", in: "  戰士  ", center: true, avail: 9, want: ly(2) + "戰士" + ly(3)},
		{name: "置中的全空白", in: "     ", center: true, avail: 4, want: ly(4)},
		{name: "置中的空譯文", in: "", center: true, avail: 5, want: ly(5)},

		// 截斷：由左取不超過 avail h 的最長整字前綴，回 truncated。
		{name: "截斷取最長整字前綴並補白", in: "哥布林戰士", avail: 7, want: "哥布林" + ly(1), trunc: true},
		{name: "截斷剛好整字", in: "哥布林戰士", avail: 6, want: "哥布林", trunc: true},
		{name: "截斷在全形字中間放不下就退一字", in: "哥布林戰士", avail: 5, want: "哥布" + ly(1), trunc: true},
		{name: "放不下任何字：全補空白", in: "戰士", avail: 1, want: ly(1), trunc: true},
		{name: "avail 為 0 截斷成空", in: "AB", avail: 0, want: "", trunc: true},
		{name: "全形半形混排的截斷", in: "A戰士", avail: 3, want: "A戰", trunc: true},
		{name: "遇到第一個放不下的字就停，不撿後面較窄的字", in: "A戰B", avail: 2, want: "A" + ly(1), trunc: true},
		{name: "全形後接 ASCII 剛好放不下", in: "戰士A", avail: 4, want: "戰士", trunc: true},
		{name: "ASCII 截斷", in: "ABCDEFG", avail: 4, want: "ABCD", trunc: true},
		{name: "置中時截斷後左補 floor((avail-w)/2)", in: "中文字", center: true, avail: 5, want: "中文" + ly(1), trunc: true},
		{name: "置中先去空白再截斷", in: "  哥布林戰士  ", center: true, avail: 7, want: "哥布林" + ly(1), trunc: true},

		// 寬度由 WideFunc 決定。
		{name: "U+2026 是全形：a…b 剛好 4 h", in: "a…b", avail: 4, want: "a…b"},
		{name: "U+2026 是半形：a…b 是 3 h 補 1", in: "a…b", avail: 4, wide: lyNoEllipsis, want: "a…b" + ly(1)},
		{name: "U+2026 是全形的截斷", in: "a…b", avail: 2, want: "a" + ly(1), trunc: true},
		{name: "U+2026 是半形的截斷", in: "a…b", avail: 2, wide: lyNoEllipsis, want: "a…", trunc: true},
		{name: "U+FF71 在 U+2E80 以上但字型是半形", in: "ｱｱ", avail: 3, want: "ｱｱ" + ly(1)},
		{name: "wide 為 nil 時每字 1 h", in: "戰士", avail: 4, nilWide: true, want: "戰士" + ly(2)},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			wide := c.wide
			if wide == nil && !c.nilWide {
				wide = lyWide
			}
			out, trunc, err := layoutLine([]rune(c.in), c.center, c.avail, wide)
			if err != nil {
				t.Fatalf("不該有錯誤：%v", err)
			}
			if string(out) != c.want {
				t.Errorf("out = %q，要 %q", string(out), c.want)
			}
			if trunc != c.trunc {
				t.Errorf("truncated = %v，要 %v", trunc, c.trunc)
			}
			// 用測試自己的寬度算法確認：輸出恆為剛好 avail h。
			if w := lyWidthOf(string(out), wide); w != c.avail {
				t.Errorf("輸出寬度 %d h，要剛好 avail = %d h", w, c.avail)
			}
		})
	}
}

// 譯文含換行或控制字元：視為格式錯誤（001 §6 第 4 項）。
func TestLayoutLineControlCharsError(t *testing.T) {
	for _, in := range []string{"A\x01B", "A\nB", "A\tB", "A\rB", "\x1f", "A\x00", "A\x7fB", "ABC\n"} {
		for _, center := range []bool{false, true} {
			out, trunc, err := layoutLine([]rune(in), center, 10, lyWide)
			if err == nil {
				t.Errorf("layoutLine(%q, center=%v) 要回錯誤", in, center)
			}
			if out != nil || trunc {
				t.Errorf("layoutLine(%q, center=%v) 出錯時 out = %q、truncated = %v，要 nil 與 false", in, center, string(out), trunc)
			}
		}
	}
	// 錯誤優先於截斷：avail 很小時控制字元仍被抓到。
	if _, _, err := layoutLine([]rune("ABC\n"), false, 1, lyWide); err == nil {
		t.Error("avail 小於字串長度時仍要檢查控制字元")
	}
	// 可印字元與 U+00A0 以上不是控制字元。
	if _, _, err := layoutLine([]rune(" ~ "), false, 5, lyWide); err != nil {
		t.Errorf("空白、~ 與 U+00A0 不是控制字元：%v", err)
	}
}

func lySegs(segs []runSeg) []string {
	out := make([]string, 0, len(segs))
	for _, s := range segs {
		k := "n"
		if s.Wide {
			k = "w"
		}
		out = append(out, k+"|"+string(s.Runes))
	}
	return out
}

func TestLayoutSplitRuns(t *testing.T) {
	cases := []struct {
		name string
		in   string
		wide WideFunc
		want []string
	}{
		{"空字串沒有段", "", lyWide, nil},
		{"全半形", "ABC", lyWide, []string{"n|ABC"}},
		{"全全形", "戰士", lyWide, []string{"w|戰士"}},
		{"半形夾全形", "AB戰士C", lyWide, []string{"n|AB", "w|戰士", "n|C"}},
		{"全形開頭", "戰A", lyWide, []string{"w|戰", "n|A"}},
		{"交錯", "A戰B士", lyWide, []string{"n|A", "w|戰", "n|B", "w|士"}},
		{"空白是半形", "  戰", lyWide, []string{"n|  ", "w|戰"}},
		{"U+2026 是全形", "…A…", lyWide, []string{"w|…", "n|A", "w|…"}},
		{"U+2026 是半形時整串一段", "…A…", lyNoEllipsis, []string{"n|…A…"}},
		{"U+FF71 是半形", "ｱA", lyWide, []string{"n|ｱA"}},
		{"wide 為 nil 全是半形", "戰A", nil, []string{"n|戰A"}},
	}
	for _, c := range cases {
		got := lySegs(splitRuns([]rune(c.in), c.wide))
		if strings.Join(got, ",") != strings.Join(c.want, ",") || len(got) != len(c.want) {
			t.Errorf("%s：splitRuns(%q) = %q，要 %q", c.name, c.in, got, c.want)
		}
	}
}

func lyEqRanges(a, b []xrange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMergeRanges(t *testing.T) {
	cases := []struct {
		name     string
		in, want []xrange
	}{
		{"空", nil, nil},
		{"單一範圍", []xrange{{3, 9}}, []xrange{{3, 9}}},
		{"已排序且不相鄰", []xrange{{0, 10}, {20, 30}}, []xrange{{0, 10}, {20, 30}}},
		{"依 X0 排序", []xrange{{20, 30}, {0, 10}}, []xrange{{0, 10}, {20, 30}}},
		{"相鄰（半開範圍首尾相接）合併", []xrange{{0, 8}, {8, 16}}, []xrange{{0, 16}}},
		{"差 1 像素不合併", []xrange{{0, 8}, {9, 16}}, []xrange{{0, 8}, {9, 16}}},
		{"重疊合併", []xrange{{0, 10}, {5, 12}}, []xrange{{0, 12}}},
		{"被包含的範圍不縮短外層", []xrange{{0, 20}, {5, 10}}, []xrange{{0, 20}}},
		{"同 X0 取較大的 X1", []xrange{{0, 5}, {0, 9}}, []xrange{{0, 9}}},
		{"同 X0 取較大的 X1（反序）", []xrange{{0, 9}, {0, 5}}, []xrange{{0, 9}}},
		{"亂序加相鄰加間隔", []xrange{{20, 30}, {0, 10}, {10, 15}, {28, 35}}, []xrange{{0, 15}, {20, 35}}},
		{"亂序鏈", []xrange{{16, 24}, {0, 8}, {8, 16}, {30, 38}}, []xrange{{0, 24}, {30, 38}}},
		{"大範圍吞掉其餘", []xrange{{5, 6}, {0, 100}, {7, 8}}, []xrange{{0, 100}}},
		{"三個不相交且亂序", []xrange{{40, 48}, {0, 8}, {20, 28}}, []xrange{{0, 8}, {20, 28}, {40, 48}}},
	}
	for _, c := range cases {
		got := mergeRanges(c.in)
		if !lyEqRanges(got, c.want) {
			t.Errorf("%s：mergeRanges(%v) = %v，要 %v", c.name, c.in, got, c.want)
		}
	}
}

func TestLayoutIntersects(t *testing.T) {
	a := xrange{8, 16}
	cases := []struct {
		name string
		rs   []xrange
		want bool
	}{
		{"沒有範圍", nil, false},
		{"左側相接（半開）不相交", []xrange{{0, 8}}, false},
		{"右側相接（半開）不相交", []xrange{{16, 24}}, false},
		{"左側重疊 1 像素", []xrange{{0, 9}}, true},
		{"右側重疊 1 像素", []xrange{{15, 30}}, true},
		{"範圍在內", []xrange{{10, 12}}, true},
		{"範圍包住", []xrange{{0, 100}}, true},
		{"完全相同", []xrange{{8, 16}}, true},
		{"多個範圍其一相交", []xrange{{0, 4}, {100, 110}, {12, 13}}, true},
		{"多個範圍都不相交", []xrange{{0, 4}, {100, 110}, {16, 20}}, false},
	}
	for _, c := range cases {
		if got := intersects(a, c.rs); got != c.want {
			t.Errorf("%s：intersects(%v, %v) = %v，要 %v", c.name, a, c.rs, got, c.want)
		}
	}
}

// lyWantStamp 是一筆疊字的預期欄位；其餘欄位的預期值由 lyCheckStamps 統一檢查。
type lyWantStamp struct {
	x, cells, cellW int
	text            string
	transparent     []bool // nil 表示沒有任何透明格
}

func lyCheckStamps(t *testing.T, name string, got []*xlate.Stamp, key string, y int, font *xlate.Font, want []lyWantStamp) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s：疊字 %d 筆，要 %d 筆", name, len(got), len(want))
		return
	}
	for i, w := range want {
		s := got[i]
		tag := fmt.Sprintf("%s[%d]", name, i)
		if s.X != w.x || s.Cells != w.cells || s.CellW != w.cellW || string(s.Text) != w.text {
			t.Errorf("%s：X=%d Cells=%d CellW=%d Text=%q，要 X=%d Cells=%d CellW=%d Text=%q",
				tag, s.X, s.Cells, s.CellW, string(s.Text), w.x, w.cells, w.cellW, w.text)
		}
		if s.Key != key || s.Y != y || s.CellH != 8 {
			t.Errorf("%s：Key=%q Y=%d CellH=%d，要 Key=%q Y=%d CellH=8", tag, s.Key, s.Y, s.CellH, key, y)
		}
		if s.GlyphScale != 1 || s.GlyphX != 0 || s.GlyphY != 0 {
			t.Errorf("%s：GlyphScale=%d GlyphX=%d GlyphY=%d，要 1、0、0", tag, s.GlyphScale, s.GlyphX, s.GlyphY)
		}
		if s.State != xlate.Pending {
			t.Errorf("%s：State=%v，要 Pending", tag, s.State)
		}
		if s.Font != font {
			t.Errorf("%s：Font 不是傳進去的字型", tag)
		}
		if s.Owner != "" || s.SwapColors || s.PixelScale != 0 || len(s.PixelGlyphs) != 0 {
			t.Errorf("%s：Owner、SwapColors、PixelScale、PixelGlyphs 不應設定", tag)
		}
		for c := 0; c < s.Cells; c++ {
			gotT := c < len(s.Transparent) && s.Transparent[c]
			wantT := c < len(w.transparent) && w.transparent[c]
			if gotT != wantT {
				t.Errorf("%s：第 %d 格 Transparent = %v，要 %v（整體 %v）", tag, c, gotT, wantT, s.Transparent)
			}
		}
	}
}

func TestBuildStampsFields(t *testing.T) {
	font := &xlate.Font{Name: "ly"}
	got := buildStamps("g7", 16, 40, []rune("AB戰士C"), lyWide, font, nil)
	lyCheckStamps(t, "AB戰士C", got, "g7", 40, font, []lyWantStamp{
		{x: 16, cells: 2, cellW: 4, text: "AB"},
		{x: 24, cells: 2, cellW: 8, text: "戰士"},
		{x: 40, cells: 1, cellW: 4, text: "C"},
	})
	got = buildStamps("g8", 0, 8, []rune("戰士"), lyWide, font, nil)
	lyCheckStamps(t, "全形", got, "g8", 8, font, []lyWantStamp{{x: 0, cells: 2, cellW: 8, text: "戰士"}})
	got = buildStamps("g9", 8, 0, []rune("AB"), lyWide, font, nil)
	lyCheckStamps(t, "半形", got, "g9", 0, font, []lyWantStamp{{x: 8, cells: 2, cellW: 4, text: "AB"}})
	got = buildStamps("g10", 0, 0, []rune("a…b"), lyWide, font, nil)
	lyCheckStamps(t, "U+2026 是全形段", got, "g10", 0, font, []lyWantStamp{
		{x: 0, cells: 1, cellW: 4, text: "a"},
		{x: 4, cells: 1, cellW: 8, text: "…"},
		{x: 12, cells: 1, cellW: 4, text: "b"},
	})
	got = buildStamps("g11", 0, 0, []rune("a…b"), lyNoEllipsis, font, nil)
	lyCheckStamps(t, "U+2026 是半形時一段", got, "g11", 0, font, []lyWantStamp{{x: 0, cells: 3, cellW: 4, text: "a…b"}})
	if got := buildStamps("g12", 0, 0, nil, lyWide, font, nil); len(got) != 0 {
		t.Errorf("空字串要沒有疊字，得到 %d 筆", len(got))
	}
}

func TestBuildStampsHidden(t *testing.T) {
	font := &xlate.Font{Name: "ly"}
	line := []rune("AB戰士C") // x0=16：AB 在 [16,24)，戰士在 [24,40)，C 在 [40,44)
	cases := []struct {
		name   string
		hidden []xrange
		want   []lyWantStamp
	}{
		{"沒有 hidden", nil, []lyWantStamp{
			{x: 16, cells: 2, cellW: 4, text: "AB"}, {x: 24, cells: 2, cellW: 8, text: "戰士"}, {x: 40, cells: 1, cellW: 4, text: "C"}}},
		{"空 hidden", []xrange{}, []lyWantStamp{
			{x: 16, cells: 2, cellW: 4, text: "AB"}, {x: 24, cells: 2, cellW: 8, text: "戰士"}, {x: 40, cells: 1, cellW: 4, text: "C"}}},
		{"hidden 與任何格都不相交", []xrange{{100, 120}}, []lyWantStamp{
			{x: 16, cells: 2, cellW: 4, text: "AB"}, {x: 24, cells: 2, cellW: 8, text: "戰士"}, {x: 40, cells: 1, cellW: 4, text: "C"}}},
		{"只蓋到第二個半形格的最後 1 像素", []xrange{{23, 24}}, []lyWantStamp{
			{x: 16, cells: 2, cellW: 4, text: "AB", transparent: []bool{false, true}},
			{x: 24, cells: 2, cellW: 8, text: "戰士"}, {x: 40, cells: 1, cellW: 4, text: "C"}}},
		{"半開範圍：[24,28) 碰不到 [20,24)，只蓋全形段第一格", []xrange{{24, 28}}, []lyWantStamp{
			{x: 16, cells: 2, cellW: 4, text: "AB"},
			{x: 24, cells: 2, cellW: 8, text: "戰士", transparent: []bool{true, false}},
			{x: 40, cells: 1, cellW: 4, text: "C"}}},
		{"hidden 跨兩筆疊字", []xrange{{22, 26}}, []lyWantStamp{
			{x: 16, cells: 2, cellW: 4, text: "AB", transparent: []bool{false, true}},
			{x: 24, cells: 2, cellW: 8, text: "戰士", transparent: []bool{true, false}},
			{x: 40, cells: 1, cellW: 4, text: "C"}}},
		{"全部格都透明的疊字不加入，後面的 X 照常累加", []xrange{{0, 24}}, []lyWantStamp{
			{x: 24, cells: 2, cellW: 8, text: "戰士"}, {x: 40, cells: 1, cellW: 4, text: "C"}}},
		{"中間一筆全透明：前後的 X 不變", []xrange{{24, 40}}, []lyWantStamp{
			{x: 16, cells: 2, cellW: 4, text: "AB"}, {x: 40, cells: 1, cellW: 4, text: "C"}}},
		{"全透明以逐格相交判定，兩個範圍各蓋一格也算", []xrange{{16, 18}, {18, 24}}, []lyWantStamp{
			{x: 24, cells: 2, cellW: 8, text: "戰士"}, {x: 40, cells: 1, cellW: 4, text: "C"}}},
		{"多個 hidden 各蓋不同格", []xrange{{16, 17}, {32, 33}}, []lyWantStamp{
			{x: 16, cells: 2, cellW: 4, text: "AB", transparent: []bool{true, false}},
			{x: 24, cells: 2, cellW: 8, text: "戰士", transparent: []bool{false, true}},
			{x: 40, cells: 1, cellW: 4, text: "C"}}},
		{"全部都被蓋住就沒有疊字", []xrange{{0, 1000}}, nil},
	}
	for _, c := range cases {
		got := buildStamps("g1", 16, 40, line, lyWide, font, c.hidden)
		lyCheckStamps(t, c.name, got, "g1", 40, font, c.want)
	}
	// 四個半形字各自一格：兩個 hidden 範圍標第一與最後一格。
	got := buildStamps("g2", 0, 0, []rune("ABCD"), lyWide, font, []xrange{{0, 1}, {12, 13}})
	lyCheckStamps(t, "四格", got, "g2", 0, font, []lyWantStamp{
		{x: 0, cells: 4, cellW: 4, text: "ABCD", transparent: []bool{true, false, false, true}}})
}

func TestAvailH(t *testing.T) {
	cases := []struct{ col, n, want int }{
		{0, 10, 20},
		{0, 40, 80},
		{0, 0, 0},
		{35, 10, 10}, // 40 - 35 = 5 格
		{38, 5, 4},
		{39, 1, 2},
		{39, 2, 2},
		{36, 4, 8}, // 剛好貼右緣，不裁切
		{30, 10, 20},
		{40, 3, 0},
		{41, 2, 0}, // 起點已在畫布外：不得回負數
		{45, 9, 0},
	}
	for _, c := range cases {
		if got := availH(c.col, c.n); got != c.want {
			t.Errorf("availH(%d, %d) = %d，要 %d", c.col, c.n, got, c.want)
		}
	}
}
