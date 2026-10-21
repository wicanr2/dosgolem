package xlate

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"slices"
	"testing"
)

// 453 §4 驗收：合成圖集，在測試內以程式組出 bytes，不讀字型檔。

// 公開簽名不變（453 §4 第 8 項）。
var (
	_ func(*Layer, []uint8, int, func(rune)) bool                                                                     = (*Layer).Draw
	_ func(*Layer, []uint8, int, func(rune), []uint8, func(*Stamp) bool) bool                                         = (*Layer).DrawWithBackground
	_ func([]byte) (*Font, error)                                                                                     = ParseFont
	_ func(*Layer, *Stamp)                                                                                            = (*Layer).Add
	_ func(*Layer, []uint8, []uint8)                                                                                  = (*Layer).Frame
	_ func([]byte) (*AlphaFont, error)                                                                                = ParseAlphaFont
	_ func([]byte, int, int, *AlphaFont, []rune, int, int, AlphaLayer, [3]uint8, AlphaRect) (AlphaRect, []rune, bool) = DrawAlphaText
)

type testAAFGlyph struct {
	cp             rune
	adv            int
	fx, fy, fw, fh int
	fill           []byte
	sx, sy, sw, sh int
	stroke         []byte
}

type testAAFHeader struct {
	magic           string
	version, px     uint16
	ascent, descent int16
	flags, stroke   uint16
	count           uint32
}

func goodHeader(px int, withStroke bool, n int) testAAFHeader {
	asc := int16(px * 880 / 1000)
	if asc < 1 {
		asc = 1
	}
	h := testAAFHeader{magic: "GOLEMAAF", version: 1, px: uint16(px), ascent: asc, descent: int16(px) - asc, count: uint32(n)}
	if withStroke {
		h.flags, h.stroke = 1, 1
	}
	return h
}

// buildAAF 依 453 §2 組出圖集 bytes；withStroke 決定每字是否寫外框欄位（與檔頭 flags 無關，供錯誤例使用）。
func buildAAF(h testAAFHeader, withStroke bool, gs []testAAFGlyph) []byte {
	var b bytes.Buffer
	b.WriteString(h.magic)
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	w(h.version)
	w(h.px)
	w(h.ascent)
	w(h.descent)
	w(h.flags)
	w(h.stroke)
	w(h.count)
	src := make([]byte, 32)
	for i := range src {
		src[i] = byte(i * 7)
	}
	b.Write(src)
	for _, g := range gs {
		w(uint32(g.cp))
		w(int16(g.adv))
		w(int16(g.fx))
		w(int16(g.fy))
		w(uint16(g.fw))
		w(uint16(g.fh))
		b.Write(g.fill)
		if withStroke {
			w(int16(g.sx))
			w(int16(g.sy))
			w(uint16(g.sw))
			w(uint16(g.sh))
			b.Write(g.stroke)
		}
	}
	return b.Bytes()
}

func mustParse(t *testing.T, b []byte) *AlphaFont {
	t.Helper()
	f, err := ParseAlphaFont(b)
	if err != nil {
		t.Fatalf("ParseAlphaFont: %v", err)
	}
	return f
}

func filled(n int, v byte) []byte { return bytes.Repeat([]byte{v}, n) }

// strokeFont：「A」填色 2×2 在 (0,−2)，外框 4×4 在 (−1,−3)，形狀與位置都不同；「B」填色 1×3。
func strokeFont(t *testing.T) *AlphaFont {
	t.Helper()
	gs := []testAAFGlyph{
		{cp: 'A', adv: 3, fx: 0, fy: -2, fw: 2, fh: 2, fill: []byte{255, 200, 100, 50}, sx: -1, sy: -3, sw: 4, sh: 4, stroke: filled(16, 255)},
		{cp: 'B', adv: 2, fx: 0, fy: -3, fw: 1, fh: 3, fill: []byte{10, 20, 30}, sx: -1, sy: -4, sw: 3, sh: 5, stroke: filled(15, 128)},
	}
	return mustParse(t, buildAAF(goodHeader(4, true, len(gs)), true, gs))
}

func TestParseAlphaFontValid(t *testing.T) {
	// 最小合法檔：1 字、無外框。
	f := mustParse(t, buildAAF(goodHeader(16, false, 1), false, []testAAFGlyph{{cp: 'x', adv: 8, fx: 1, fy: -9, fw: 2, fh: 1, fill: []byte{1, 2}}}))
	g := f.Glyphs['x']
	if f.Px != 16 || f.Ascent != 14 || f.Descent != 2 || f.HasStroke || f.StrokeRadius != 0 || len(f.Glyphs) != 1 {
		t.Fatalf("檔頭解析錯：%+v", f)
	}
	if g.Adv != 8 || g.FX != 1 || g.FY != -9 || g.FW != 2 || g.FH != 1 || !bytes.Equal(g.Fill, []byte{1, 2}) || g.Stroke != nil || g.SW != 0 {
		t.Fatalf("字形解析錯：%+v", g)
	}
	if f.Source[1] != 7 || f.Source[31] != byte(31*7) {
		t.Fatalf("source 未原樣保留：%x", f.Source)
	}
	// 有外框。
	sf := strokeFont(t)
	if !sf.HasStroke || sf.StrokeRadius != 1 || sf.Glyphs['A'].SX != -1 || sf.Glyphs['A'].SH != 4 || len(sf.Glyphs['B'].Stroke) != 15 {
		t.Fatalf("外框解析錯：%+v %+v", sf, sf.Glyphs['A'])
	}
	// fw > 0、fh = 0 的空白字；外框 sw × 2 全 0 的空白字。
	bf := mustParse(t, buildAAF(goodHeader(22, true, 2), true, []testAAFGlyph{
		{cp: ' ', adv: 5, fw: 5, fh: 0, sx: -1, sy: -1, sw: 7, sh: 2, stroke: filled(14, 0)},
		{cp: 0x3000, adv: 22, fw: 22, fh: 0, sx: -1, sy: -1, sw: 24, sh: 2, stroke: filled(48, 0)},
	}))
	sp := bf.Glyphs[' ']
	if sp.FW != 5 || sp.FH != 0 || len(sp.Fill) != 0 || sp.SW != 7 || sp.SH != 2 || len(sp.Stroke) != 14 {
		t.Fatalf("空白字解析錯：%+v", sp)
	}
	if bf.Glyphs[0x3000].SW != 24 {
		t.Fatalf("全形空白解析錯")
	}
}

func TestParseAlphaFontErrors(t *testing.T) {
	one := []testAAFGlyph{{cp: 'a', adv: 4, fw: 1, fh: 1, fill: []byte{9}}}
	two := []testAAFGlyph{{cp: 'a', adv: 4, fw: 1, fh: 1, fill: []byte{9}}, {cp: 'b', adv: 4, fw: 1, fh: 1, fill: []byte{9}}}
	good := buildAAF(goodHeader(16, false, 1), false, one)
	if _, err := ParseAlphaFont(good); err != nil {
		t.Fatalf("基準檔應合法：%v", err)
	}
	mod := func(f func(h *testAAFHeader)) []byte {
		h := goodHeader(16, false, 1)
		f(&h)
		return buildAAF(h, false, one)
	}
	cases := map[string][]byte{
		"magic":             mod(func(h *testAAFHeader) { h.magic = "GOLEMFNT" }),
		"version 2":         mod(func(h *testAAFHeader) { h.version = 2 }),
		"ascent+descent≠px": mod(func(h *testAAFHeader) { h.descent++ }),
		"ascent 0":          mod(func(h *testAAFHeader) { h.ascent, h.descent = 0, 16 }),
		"px 0":              mod(func(h *testAAFHeader) { h.px, h.ascent, h.descent = 0, 0, 0 }),
		"flags 未定義位元":       mod(func(h *testAAFHeader) { h.flags = 2 }),
		"無外框但 stroke≠0":     mod(func(h *testAAFHeader) { h.stroke = 1 }),
		"count 0":           mod(func(h *testAAFHeader) { h.count = 0 }),
		"碼點未遞增":             buildAAF(goodHeader(16, false, 2), false, []testAAFGlyph{two[1], two[0]}),
		"碼點重複":              buildAAF(goodHeader(16, false, 2), false, []testAAFGlyph{two[0], two[0]}),
		"代理對碼點":             buildAAF(goodHeader(16, false, 1), false, []testAAFGlyph{{cp: 0xD800, adv: 1}}),
		"碼點超出":              buildAAF(goodHeader(16, false, 1), false, []testAAFGlyph{{cp: 0x110000, adv: 1}}),
		"adv 負":             buildAAF(goodHeader(16, false, 1), false, []testAAFGlyph{{cp: 'a', adv: -1}}),
		"長度少 1":             good[:len(good)-1],
		"尾端多 1":             append(slices.Clone(good), 0),
		"count 多":           buildAAF(goodHeader(16, false, 2), false, one),
		"count 少":           buildAAF(goodHeader(16, false, 1), false, two),
		"檔頭不足":              good[:40],
	}
	for name, b := range cases {
		if f, err := ParseAlphaFont(b); err == nil || f != nil {
			t.Errorf("%s：應回錯，得 %v %v", name, f, err)
		}
	}
	// 宣告寬高乘積大於剩餘長度：65535 × 65535（約 4 GB），不得先配置。
	big := buildAAF(goodHeader(16, false, 1), false, []testAAFGlyph{{cp: 'a', adv: 1, fw: 65535, fh: 65535, fill: []byte{1, 2, 3}}})
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	if _, err := ParseAlphaFont(big); err == nil {
		t.Fatalf("宣告超長遮罩應回錯")
	}
	runtime.ReadMemStats(&after)
	if d := after.TotalAlloc - before.TotalAlloc; d > 1<<20 {
		t.Fatalf("回錯前配置了 %d bytes", d)
	}
	// 外框遮罩超長同樣回錯。
	bigS := buildAAF(goodHeader(16, true, 1), true, []testAAFGlyph{{cp: 'a', adv: 1, sw: 65535, sh: 65535}})
	if _, err := ParseAlphaFont(bigS); err == nil {
		t.Fatalf("宣告超長外框遮罩應回錯")
	}
}

func TestAlphaAdvance(t *testing.T) {
	f := strokeFont(t)
	if w, miss := f.Advance([]rune("ABA")); w != 8 || miss != nil {
		t.Fatalf("Advance = %d %v，要 8 nil", w, miss)
	}
	w, miss := f.Advance([]rune("AzBzy"))
	if w != 5 || !slices.Equal(miss, []rune("zzy")) {
		t.Fatalf("缺字 Advance = %d %q，要 5 \"zzy\"", w, string(miss))
	}
}

// px1 回 1×1 的遮罩與 box。
func px1(x, y int, a byte) ([]byte, AlphaRect) { return []byte{a}, AlphaRect{x, y, x + 1, y + 1} }

func TestBlendFormula(t *testing.T) {
	// a = 0 不改：alpha 位元組初值 0。
	dst := []byte{10, 20, 30, 0}
	m, box := px1(0, 0, 0)
	BlendMask(dst, 1, 1, m, box, [3]uint8{255, 255, 255})
	if !bytes.Equal(dst, []byte{10, 20, 30, 0}) {
		t.Fatalf("a = 0 改寫了 dst：%v", dst)
	}
	// a = 255 等於 c。
	m, box = px1(0, 0, 255)
	BlendMask(dst, 1, 1, m, box, [3]uint8{1, 2, 3})
	if !bytes.Equal(dst, []byte{1, 2, 3, 255}) {
		t.Fatalf("a = 255 得 %v", dst)
	}
	// a = 128、底 0、c = 255 得 128。
	dst = []byte{0, 0, 0, 0}
	m, box = px1(0, 0, 128)
	BlendMask(dst, 1, 1, m, box, [3]uint8{255, 255, 255})
	if !bytes.Equal(dst, []byte{128, 128, 128, 255}) {
		t.Fatalf("a = 128 得 %v", dst)
	}
	// c = 255、全部 65536 組 (底, a)：與獨立寫出的式子逐值相同。
	bad := 0
	for d := 0; d < 256; d++ {
		for a := 0; a < 256; a++ {
			want := d
			if a > 0 {
				want = (255*a + d*(255-a) + 127) / 255
			}
			px := []byte{byte(d), byte(d), byte(d), 0}
			m, box = px1(0, 0, byte(a))
			BlendMask(px, 1, 1, m, box, [3]uint8{255, 255, 255})
			if int(px[0]) != want || int(px[1]) != want || int(px[2]) != want {
				bad++
			}
		}
	}
	if bad != 0 {
		t.Fatalf("混色式 %d 組不符", bad)
	}
}

func TestStringCompositeOverlay(t *testing.T) {
	// 兩字遮罩在同一像素重疊，兩字 alpha 都是 3（adv 0），字色 255、底 128。
	gs := []testAAFGlyph{
		{cp: 'p', adv: 0, fx: 0, fy: -1, fw: 1, fh: 1, fill: []byte{3}},
		{cp: 'q', adv: 0, fx: 0, fy: -1, fw: 1, fh: 1, fill: []byte{3}},
	}
	f := mustParse(t, buildAAF(goodHeader(4, false, 2), false, gs))
	dst := []byte{128, 128, 128, 0}
	ink, miss, ns := DrawAlphaText(dst, 1, 1, f, []rune("pq"), 0, 1, AlphaFill, [3]uint8{255, 255, 255}, AlphaRect{0, 0, 1, 1})
	if miss != nil || ns || ink != (AlphaRect{0, 0, 1, 1}) {
		t.Fatalf("DrawAlphaText 回傳 %v %v %v", ink, miss, ns)
	}
	blend := func(d, a int) int { return (255*a + d*(255-a) + 127) / 255 }
	over := func(m, a int) int { return (a*255 + m*(255-a) + 127) / 255 }
	perGlyph := blend(blend(128, 3), 3)
	maxed := blend(128, 3)
	overlay := blend(128, over(over(0, 3), 3))
	if perGlyph != 130 || maxed != 129 || overlay != 131 {
		t.Fatalf("測試內算式：逐字 %d、最大值 %d、疊合 %d，要 130、129、131", perGlyph, maxed, overlay)
	}
	if dst[0] != 131 {
		t.Fatalf("字串合成得 %d，要 131（逐字 130、最大值 129）", dst[0])
	}
	m, box, _, _ := StringMask(f, []rune("pq"), 0, 1, AlphaFill, AlphaRect{0, 0, 1, 1})
	if !bytes.Equal(m, []byte{6}) || box != (AlphaRect{0, 0, 1, 1}) {
		t.Fatalf("字串遮罩 %v %v，要 [6]", m, box)
	}
}

func TestAlphaStrokeLayer(t *testing.T) {
	f := strokeFont(t)
	clip := AlphaRect{0, 0, 10, 10}
	m, box, miss, ns := StringMask(f, []rune("A"), 3, 5, AlphaStroke, clip)
	if miss != nil || ns {
		t.Fatalf("外框層 %v %v", miss, ns)
	}
	// 外框 4×4 全 255 在 (3−1, 5−3) = (2,2)。
	if box != (AlphaRect{2, 2, 6, 6}) || !bytes.Equal(m, filled(16, 255)) {
		t.Fatalf("外框層 box %v m %v", box, m)
	}
	m, box, _, _ = StringMask(f, []rune("A"), 3, 5, AlphaFill, clip)
	if box != (AlphaRect{3, 3, 5, 5}) || !bytes.Equal(m, []byte{255, 200, 100, 50}) {
		t.Fatalf("填色層 box %v m %v", box, m)
	}
	// 經 DrawAlphaText：外框層寫到 (2,2)，填色層不寫 (2,2)。
	dst := make([]byte, 4*10*10)
	ink, _, _ := DrawAlphaText(dst, 10, 10, f, []rune("A"), 3, 5, AlphaStroke, [3]uint8{9, 9, 9}, clip)
	if ink != (AlphaRect{2, 2, 6, 6}) || dst[4*(2*10+2)] != 9 {
		t.Fatalf("外框層繪製 ink %v", ink)
	}
	// 沒有外框層：noStroke 為真、dst 不變。
	nf := mustParse(t, buildAAF(goodHeader(4, false, 1), false, []testAAFGlyph{{cp: 'A', adv: 3, fy: -2, fw: 2, fh: 2, fill: filled(4, 255)}}))
	dst = make([]byte, 4*10*10)
	ink, miss, ns = DrawAlphaText(dst, 10, 10, nf, []rune("A"), 3, 5, AlphaStroke, [3]uint8{9, 9, 9}, clip)
	if !ns || miss != nil || ink != (AlphaRect{}) || !bytes.Equal(dst, make([]byte, 400)) {
		t.Fatalf("無外框層：ns %v ink %v", ns, ink)
	}
	if m, box, _, ns := StringMask(nf, []rune("A"), 3, 5, AlphaStroke, clip); m != nil || !box.Empty() || !ns {
		t.Fatalf("無外框層 StringMask %v %v %v", m, box, ns)
	}
}

// clipFont：「C」填色 4×4，外圈一圈 0、內部 2×2 為 200；「D」填色 3×1 全 255。
func clipFont(t *testing.T) *AlphaFont {
	t.Helper()
	c := []byte{
		0, 0, 0, 0,
		0, 200, 200, 0,
		0, 200, 200, 0,
		0, 0, 0, 0,
	}
	gs := []testAAFGlyph{
		{cp: 'C', adv: 4, fx: 0, fy: -4, fw: 4, fh: 4, fill: c},
		{cp: 'D', adv: 3, fx: 0, fy: -1, fw: 3, fh: 1, fill: filled(3, 255)},
	}
	return mustParse(t, buildAAF(goodHeader(4, false, len(gs)), false, gs))
}

func TestAlphaClip(t *testing.T) {
	f := clipFont(t)
	// box 縮到 alpha > 0 的外接框：「C」在 (2,6) 起，墨跡 (3..4, 7..8)。
	m, box, _, _ := StringMask(f, []rune("C"), 2, 10, AlphaFill, AlphaRect{0, 0, 20, 20})
	if box != (AlphaRect{3, 7, 5, 9}) || !bytes.Equal(m, filled(4, 200)) {
		t.Fatalf("外接框 %v m %v，要 {3 7 5 9}", box, m)
	}
	// clip 切掉一部分：只留 x = 4 那一欄。
	clip := AlphaRect{4, 0, 20, 20}
	m, box, _, _ = StringMask(f, []rune("C"), 2, 10, AlphaFill, clip)
	if box != (AlphaRect{4, 7, 5, 9}) || !bytes.Equal(m, []byte{200, 200}) {
		t.Fatalf("裁切後 box %v m %v", box, m)
	}
	dst := make([]byte, 4*20*20)
	ink, _, _ := DrawAlphaText(dst, 20, 20, f, []rune("C"), 2, 10, AlphaFill, [3]uint8{255, 0, 0}, clip)
	if ink != (AlphaRect{4, 7, 5, 9}) {
		t.Fatalf("ink %v", ink)
	}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			inside := x == 4 && (y == 7 || y == 8)
			if got := dst[4*(y*20+x)+3]; (got != 0) != inside {
				t.Fatalf("(%d,%d) alpha %d，clip 內墨跡 %v", x, y, got, inside)
			}
		}
	}
	// clip 與 dst 範圍不交：不寫入、ink 空框。
	dst = make([]byte, 4*20*20)
	ink, _, _ = DrawAlphaText(dst, 20, 20, f, []rune("C"), 2, 10, AlphaFill, [3]uint8{255, 0, 0}, AlphaRect{30, 0, 40, 20})
	if ink != (AlphaRect{}) || !bytes.Equal(dst, make([]byte, 1600)) {
		t.Fatalf("clip 不交仍寫入，ink %v", ink)
	}
	// 字串一部分落在 dst 外（clip 很大）：ink 只含 dst 內的像素。
	dst = make([]byte, 4*4*4)
	ink, _, _ = DrawAlphaText(dst, 4, 4, f, []rune("CD"), 0, 3, AlphaFill, [3]uint8{255, 0, 0}, AlphaRect{-100, -100, 100, 100})
	// 「C」墨跡 (1..2, 0..1)，「D」在 x 4..6、y 2，全在 dst 外。
	if ink != (AlphaRect{1, 0, 3, 2}) {
		t.Fatalf("部分在 dst 外的 ink %v，要 {1 0 3 2}", ink)
	}
	// BlendMask：len(m) 與 box 不符、len(dst) 不足時不寫入；空 box 時 m 必須為空。
	dst = make([]byte, 4*4*4)
	BlendMask(dst, 4, 4, []byte{255, 255, 255}, AlphaRect{0, 0, 2, 2}, [3]uint8{1, 1, 1})
	BlendMask(dst, 4, 4, []byte{255, 255, 255, 255, 255}, AlphaRect{0, 0, 2, 2}, [3]uint8{1, 1, 1})
	BlendMask(dst[:63], 4, 4, filled(4, 255), AlphaRect{0, 0, 2, 2}, [3]uint8{1, 1, 1})
	BlendMask(dst, 4, 4, []byte{255}, AlphaRect{}, [3]uint8{1, 1, 1})
	if !bytes.Equal(dst, make([]byte, 64)) {
		t.Fatalf("BlendMask 輸入不符仍寫入：%v", dst)
	}
	// H 由參數給：dst 較長時不寫超出 H 的列。
	dst = make([]byte, 4*4*8)
	BlendMask(dst, 4, 2, filled(4*4, 255), AlphaRect{0, 0, 4, 4}, [3]uint8{1, 1, 1})
	if dst[4*(2*4)+3] != 0 || dst[4*(1*4)+3] != 255 {
		t.Fatalf("BlendMask 未依參數 H 裁切")
	}
}

func TestAlphaMissing(t *testing.T) {
	f := clipFont(t)
	dst := make([]byte, 4*20*20)
	for i := range dst {
		dst[i] = 77
	}
	orig := slices.Clone(dst)
	ink, miss, ns := DrawAlphaText(dst, 20, 20, f, []rune("CxDy"), 2, 10, AlphaFill, [3]uint8{255, 0, 0}, AlphaRect{0, 0, 20, 20})
	if !bytes.Equal(dst, orig) {
		t.Fatalf("缺字時 dst 被改寫")
	}
	if !slices.Equal(miss, []rune("xy")) || ns || ink != (AlphaRect{}) {
		t.Fatalf("缺字回傳 miss %q ns %v ink %v，要 \"xy\"", string(miss), ns, ink)
	}
	if m, box, miss, _ := StringMask(f, []rune("Cx"), 0, 10, AlphaFill, AlphaRect{0, 0, 20, 20}); m != nil || !box.Empty() || string(miss) != "x" {
		t.Fatalf("StringMask 缺字 %v %v %q", m, box, string(miss))
	}
}

func TestAlphaNoInk(t *testing.T) {
	// 只有空白：box 空、m nil、ink 空。
	f := mustParse(t, buildAAF(goodHeader(22, false, 1), false, []testAAFGlyph{{cp: ' ', adv: 5, fw: 5, fh: 0}}))
	m, box, miss, ns := StringMask(f, []rune("  "), 0, 10, AlphaFill, AlphaRect{0, 0, 50, 50})
	if m != nil || box != (AlphaRect{}) || miss != nil || ns {
		t.Fatalf("空白字串 %v %v %v %v", m, box, miss, ns)
	}
}
