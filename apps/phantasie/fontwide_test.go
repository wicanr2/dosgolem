package phantasie

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode"

	"github.com/wicanr2/dosgolem/xlate"
)

// fwEnt 是測試字型的一個字：碼點與來源位元組（bit 7 = 全形，低 7 位元 = 來源編號）。
type fwEnt struct {
	cp  rune
	src byte
}

// fwBuild 組出一份 GOLEMFNT：每字的位元圖是 H×((W+7)/8) bytes 的確定性非零內容。
func fwBuild(w, h int, ents []fwEnt) []byte {
	b := []byte(fwMagic)
	b = binary.LittleEndian.AppendUint16(b, uint16(w))
	b = binary.LittleEndian.AppendUint16(b, uint16(h))
	b = binary.LittleEndian.AppendUint32(b, uint32(len(ents)))
	for i, e := range ents {
		b = binary.LittleEndian.AppendUint32(b, uint32(e.cp))
		b = append(b, e.src)
		for j := 0; j < h*((w+7)/8); j++ {
			b = append(b, byte(i*7+j+1))
		}
	}
	return b
}

// fwSample 含半形與全形字（W=H=16，每字 32 bytes 位元圖）。
// 全形與否只看來源位元組的 bit 7，所以刻意放入與碼點範圍相反的字：
// U+2026 全形（碼點範圍會判成半形）、U+4E00 半形（碼點範圍會判成全形）。
var fwSample = []fwEnt{
	{'A', 0x01},
	{' ', 0x01},
	{'~', 0x01},
	{'中', 0x81},
	{'…', 0x81}, // Unifont 的 U+2026 是 16 px 寬
	{'一', 0x01}, // CJK 碼點，但字型裡是半形
	{'全', 0x80}, // 只有 bit 7：來源編號 0
	{'Ａ', 0x7F}, // 全形碼點，低 7 位元全 1 但 bit 7 為 0：半形
	{'☆', 0xFF}, // 低 7 位元全 1 且 bit 7 為 1：全形
}

var fwSampleWide = map[rune]bool{
	'A': false, ' ': false, '~': false, '中': true, '…': true, '一': false, '全': true, 'Ａ': false, '☆': true,
}

// fwAbsent 是字型沒有收入的字元。
var fwAbsent = []rune{'B', '龍', '①', 0, 0x10FFFF, -1}

func TestFontWideTable(t *testing.T) {
	tb, err := ParseFontWide(fwBuild(16, 16, fwSample))
	if err != nil {
		t.Fatal(err)
	}
	if tb.Len() != len(fwSample) {
		t.Errorf("Len = %d，要 %d", tb.Len(), len(fwSample))
	}
	f := tb.Func()
	for r, want := range fwSampleWide {
		if !tb.Has(r) {
			t.Errorf("Has(%q) 應為真", r)
		}
		if got := tb.Wide(r); got != want {
			t.Errorf("Wide(%q) = %v，要 %v", r, got, want)
		}
		if f(r) != want {
			t.Errorf("Func()(%q) 與 Wide 不同", r)
		}
	}
	for _, r := range fwAbsent {
		if tb.Has(r) {
			t.Errorf("Has(%q) 應為假（字型沒有收入）", r)
		}
		if tb.Wide(r) || f(r) {
			t.Errorf("未收入的 %q 應為半形（false）", r)
		}
	}
	// 接進格式引擎：寬度由字型決定，不是碼點範圍。
	if got := HalfWidth("…A一", f); got != 4 {
		t.Errorf("HalfWidth(…A一) = %d，要 4（… 全形 2，A 與 一 各 1）", got)
	}
}

func TestFontWideNilAndZeroTable(t *testing.T) {
	var nilTable *WideTable
	var zero WideTable
	for name, tb := range map[string]*WideTable{"nil": nilTable, "零值": &zero} {
		if tb.Wide('中') || tb.Has('中') || tb.Len() != 0 {
			t.Errorf("%s 表應為空表", name)
		}
		if f := tb.Func(); f == nil || f('中') {
			t.Errorf("%s 表的 Func 應非 nil 且回 false", name)
		}
	}
}

func TestFontWideLoad(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.golemfnt")
	if err := os.WriteFile(good, fwBuild(16, 16, fwSample), 0o644); err != nil {
		t.Fatal(err)
	}
	tb, err := LoadFontWide(good)
	if err != nil {
		t.Fatal(err)
	}
	if !tb.Wide('中') || tb.Wide('A') || !tb.Has('A') {
		t.Error("LoadFontWide 載入的表內容不對")
	}
	if _, err := LoadFontWide(filepath.Join(dir, "missing.golemfnt")); err == nil {
		t.Error("檔案不存在應回 error")
	}
	bad := filepath.Join(dir, "bad.golemfnt")
	if err := os.WriteFile(bad, []byte("not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFontWide(bad); err == nil {
		t.Error("內容損毀應回 error")
	}
}

func fwHeader(w, h uint16, count uint32) []byte {
	b := []byte(fwMagic)
	b = binary.LittleEndian.AppendUint16(b, w)
	b = binary.LittleEndian.AppendUint16(b, h)
	return binary.LittleEndian.AppendUint32(b, count)
}

func TestFontWideCorrupt(t *testing.T) {
	good := fwBuild(16, 16, fwSample)
	mut := func(f func(b []byte) []byte) []byte { return f(append([]byte(nil), good...)) }

	cases := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"空", []byte{}},
		{"magic 太短", []byte("GOLEMFN")},
		{"只有 magic", []byte(fwMagic)},
		{"magic 不符", mut(func(b []byte) []byte { b[7] = 'X'; return b })},
		{"magic 大小寫不符", mut(func(b []byte) []byte { b[0] = 'g'; return b })},
		{"檔頭不完整", good[:fwHeaderLen-1]},
		{"宣告 1 字但沒有內容", fwHeader(16, 16, 1)},
		{"宣告字數多一個", mut(func(b []byte) []byte { b[12]++; return b })},
		{"宣告字數少一個", mut(func(b []byte) []byte { b[12]--; return b })},
		{"宣告 0 字但有內容", mut(func(b []byte) []byte { b[12], b[13], b[14], b[15] = 0, 0, 0, 0; return b })},
		{"截在最後一字中間", good[:len(good)-1]},
		{"少一整字", good[:len(good)-37]},
		{"尾端多一個位元組", append(append([]byte(nil), good...), 0)},
		{"W = 0", mut(func(b []byte) []byte { b[8], b[9] = 0, 0; return b })},
		{"H = 0", mut(func(b []byte) []byte { b[10], b[11] = 0, 0; return b })},
		{"W、H 與資料長度不符", mut(func(b []byte) []byte { b[8] = 24; return b })},
		{"巨大 W、H 與字數", fwHeader(0xFFFF, 0xFFFF, 0xFFFFFFFF)},
		{"巨大 W、H，宣告 1 字", fwHeader(0xFFFF, 0xFFFF, 1)},
		{"W = 0 的最小字型", append(fwHeader(0, 0, 1), 'A', 0, 0, 0, 1)},
		{"碼點超過 U+10FFFF", fwBuild(16, 16, []fwEnt{{0x110000, 0x01}})},
		{"碼點是 0xFFFFFFFF", append(fwHeader(8, 1, 1), 0xFF, 0xFF, 0xFF, 0xFF, 0x01, 0xAA)},
	}
	for _, tc := range cases {
		tb, err := ParseFontWide(tc.data)
		if err == nil {
			t.Errorf("%s：應回 error，得 %+v", tc.name, tb)
		}
	}

	// 所有真前綴都不是合法字型（字數與長度對不上）。
	for n := 0; n < len(good); n++ {
		if _, err := ParseFontWide(good[:n]); err == nil {
			t.Errorf("長度 %d 的前綴應回 error", n)
		}
	}

	// 合法但為空的字型（0 字）可解析，表是空的。
	if tb, err := ParseFontWide(fwHeader(16, 16, 0)); err != nil || tb.Len() != 0 || tb.Wide('A') {
		t.Errorf("0 字字型：%+v %v", tb, err)
	}
	// 其他尺寸的合法字型：W = 8（每列 1 byte）、W = 9（每列 2 bytes）、H = 1。
	for _, wh := range [][2]int{{8, 16}, {9, 3}, {16, 1}, {1, 1}, {24, 24}} {
		tb, err := ParseFontWide(fwBuild(wh[0], wh[1], fwSample))
		if err != nil || tb.Len() != len(fwSample) || !tb.Wide('中') || tb.Wide('A') {
			t.Errorf("W=%d H=%d：%+v %v", wh[0], wh[1], tb, err)
		}
	}
}

func TestFontWideDuplicateCodepointLastWins(t *testing.T) {
	data := fwBuild(16, 16, []fwEnt{{'A', 0x01}, {'B', 0x81}, {'A', 0x81}, {'B', 0x01}})
	tb, err := ParseFontWide(data)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Len() != 2 || !tb.Wide('A') || tb.Wide('B') {
		t.Errorf("重複碼點以後出現者為準：Len=%d Wide(A)=%v Wide(B)=%v", tb.Len(), tb.Wide('A'), tb.Wide('B'))
	}
	xf, err := xlate.ParseFont(data)
	if err != nil || len(xf.Glyphs) != tb.Len() {
		t.Errorf("xlate.ParseFont 對重複碼點的字數 = %d（%v），要與 Len 相同 %d", len(xf.Glyphs), err, tb.Len())
	}
}

// fwAgree 以同一份位元組呼叫 xlate.ParseFont 與 ParseFontWide，要求兩邊對「是不是合法字型」與
// 「收入哪些字」的看法一致。ParseFontWide 比 xlate.ParseFont 嚴格的情形只有兩種（見 ParseFontWide 的說明）：
// W 或 H 為 0、碼點超出 Unicode 範圍。
func fwAgree(t *testing.T, name string, data []byte) {
	t.Helper()
	xf, xerr := xlate.ParseFont(data)
	tb, terr := ParseFontWide(data)
	switch {
	case xerr != nil && terr == nil:
		t.Errorf("%s：xlate.ParseFont 拒絕（%v），ParseFontWide 卻接受", name, xerr)
	case xerr == nil && terr != nil:
		stricter := xf.W == 0 || xf.H == 0
		for cp := range xf.Glyphs {
			if cp < 0 || cp > unicode.MaxRune {
				stricter = true
			}
		}
		if !stricter {
			t.Errorf("%s：xlate.ParseFont 接受，ParseFontWide 卻拒絕（%v），且不屬於已知的較嚴格情形", name, terr)
		}
	case xerr == nil && terr == nil:
		if tb.Len() != len(xf.Glyphs) {
			t.Errorf("%s：收入字數 xlate=%d ParseFontWide=%d", name, len(xf.Glyphs), tb.Len())
		}
		for cp := range xf.Glyphs {
			if !tb.Has(cp) {
				t.Errorf("%s：xlate 收入 U+%04X，ParseFontWide.Has 為假", name, cp)
			}
		}
	}
}

func TestFontWideAgreesWithXlate(t *testing.T) {
	// 合法字型：兩邊收入同一組字，且 Wide 等於來源位元組的 bit 7。
	for _, wh := range [][2]int{{16, 16}, {8, 16}, {9, 3}, {24, 24}} {
		data := fwBuild(wh[0], wh[1], fwSample)
		xf, err := xlate.ParseFont(data)
		if err != nil {
			t.Fatalf("W=%d H=%d：xlate.ParseFont：%v", wh[0], wh[1], err)
		}
		tb, err := ParseFontWide(data)
		if err != nil {
			t.Fatalf("W=%d H=%d：ParseFontWide：%v", wh[0], wh[1], err)
		}
		if len(xf.Glyphs) != tb.Len() || len(xf.Glyphs) != len(fwSample) {
			t.Errorf("W=%d H=%d：字數 xlate=%d ParseFontWide=%d 樣本=%d", wh[0], wh[1], len(xf.Glyphs), tb.Len(), len(fwSample))
		}
		for _, e := range fwSample {
			if _, ok := xf.Glyphs[e.cp]; !ok || !tb.Has(e.cp) {
				t.Errorf("W=%d H=%d：U+%04X 的收入，xlate=%v ParseFontWide=%v", wh[0], wh[1], e.cp, ok, tb.Has(e.cp))
			}
			if got, want := tb.Wide(e.cp), e.src&0x80 != 0; got != want {
				t.Errorf("W=%d H=%d：Wide(U+%04X) = %v，要 %v", wh[0], wh[1], e.cp, got, want)
			}
		}
		for _, r := range fwAbsent {
			if _, ok := xf.Glyphs[r]; ok || tb.Has(r) {
				t.Errorf("W=%d H=%d：U+%04X 兩邊都不該收入", wh[0], wh[1], r)
			}
		}
	}

	// 損毀與邊界輸入：ParseFontWide 的錯誤集合包含 xlate.ParseFont 的錯誤集合，
	// 兩邊都接受時收入的字相同。
	bases := [][]byte{
		fwBuild(16, 16, fwSample),
		fwBuild(8, 16, fwSample[:3]),
		fwBuild(9, 3, fwSample[:2]),
		fwBuild(16, 16, nil),
	}
	for _, base := range bases {
		fwAgree(t, "原樣", base)
		for n := 0; n < len(base); n++ {
			fwAgree(t, "截斷", base[:n])
		}
		fwAgree(t, "尾端多 1 byte", append(append([]byte(nil), base...), 0))
		fwAgree(t, "尾端多一個字長", append(append([]byte(nil), base...), make([]byte, 37)...))
		// 檔頭與第一字前幾個位元組（碼點、來源）逐位元組代入幾個值。
		for pos := 0; pos < len(base) && pos < fwHeaderLen+6; pos++ {
			for _, v := range []byte{0x00, 0x01, 0x7F, 0x80, 0xFF} {
				m := append([]byte(nil), base...)
				m[pos] = v
				fwAgree(t, "改 byte", m)
			}
		}
	}
}
