package dkk

import (
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

const (
	gEntry   = 0x6135 // 05F4:01F5 的線性位址（合成）
	menuSeg  = 0x34CF
	retBase  = 0x0352
	cSS, cSP = 0x1CBD, 0x3E00
)

func plantC(m fakeMem) {
	for i, x := range GlyphSignature.Bytes {
		m[gEntry+uint32(i)] = x
	}
	// 四個呼叫點：返回位址前 5 bytes 是 call far 01F5，後面接 menuCallSites 的 bytes
	for i, s := range menuCallSites {
		ret := linear(menuSeg, uint16(retBase+0x40*i))
		copy(m[ret-5:], []byte{0x9A, 0xF5, 0x01, 0xE4, 0x04})
		copy(m[ret:], s.after)
	}
}

// glyphCall 依規格 003 §1 放一次 01F5 呼叫。site 是 menuCallSites 的索引；-1 表示非選單呼叫端。
func glyphCall(c *PathC, m fakeMem, site int, ch byte, count int, bg, fg uint8, row, col int) {
	retOff := uint16(0x0100)
	if site >= 0 {
		retOff = uint16(retBase + 0x40*site)
	}
	s := linear(cSS, cSP)
	m.put16(s, retOff)
	m.put16(s+2, menuSeg)
	for i, v := range []int{1, int(ch), count, int(bg), int(fg), row, col} {
		m.put16(s+4+uint32(2*i), 0x7200|uint16(v))
	}
	c.Before(0x05F4, 0x01F5, cSS, cSP, m)
	c.Before(menuSeg, retOff, cSS, cSP+4+glyphArgBytes, m) // 返回
}

func newC(t *testing.T, cat *Catalog) (*PathC, fakeMem, *xlate.Layer) {
	t.Helper()
	m := make(fakeMem, 0x100000)
	plantC(m)
	layer := &xlate.Layer{}
	c, err := NewPathC(m, cat, &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}, layer)
	if err != nil {
		t.Fatal(err)
	}
	var pal [256][3]uint8
	for i := range pal {
		pal[i] = [3]uint8{uint8(i), uint8(i), uint8(i)}
	}
	c.pal = pal
	return c, m, layer
}

func drawWord(c *PathC, m fakeMem, row, col int, word string, hot int, selected bool) {
	for i := 0; i < len(word); i++ {
		switch {
		case selected:
			glyphCall(c, m, 0, word[i], 1, 15, 0, row, col+i)
		case i == hot:
			glyphCall(c, m, 1, word[i], 1, 0, 15, row, col+i)
		default:
			glyphCall(c, m, 2, word[i], 1, 0, 10, row, col+i)
		}
	}
}

func TestPathCWordsHotkeyAndSelection(t *testing.T) {
	c, m, l := newC(t, catalogOf(t, "Select", "選擇", "Exit", "離開"))
	drawWord(c, m, 24, 0, "Select", 0, true)
	glyphCall(c, m, 2, ' ', 1, 0, 10, 24, 6)
	drawWord(c, m, 24, 7, "Exit", 0, false)
	c.SetPalette(c.pal)
	got := map[string]*xlate.Stamp{}
	for _, s := range l.Stamps {
		got[s.Key] = s
	}
	sel := got[pathCKey+"24.0"]
	if sel == nil || string(sel.Text[:2]) != "選擇" || sel.Cells != 6 || sel.BG != c.pal[15] || sel.FG != c.pal[0] {
		t.Fatalf("選中詞應整詞反白：%+v", sel)
	}
	if got[pathCKey+"24.0.hot"] != nil {
		t.Fatal("整詞反白時沒有獨立熱鍵格")
	}
	ex, hot := got[pathCKey+"24.7"], got[pathCKey+"24.7.hot"]
	if ex == nil || string(ex.Text[:2]) != "離開" || ex.FG != c.pal[10] {
		t.Fatalf("一般詞：%+v", ex)
	}
	if hot == nil || hot.X != (7+2)*8 || hot.Text[0] != 'E' || hot.FG != c.pal[15] {
		t.Fatalf("熱鍵字母應以熱鍵色接在中文之後：%+v", hot)
	}
}

func TestPathCIgnoresOtherCallersAndPadClears(t *testing.T) {
	c, m, l := newC(t, catalogOf(t, "Exit", "離開"))
	glyphCall(c, m, -1, 'X', 1, 0, 10, 24, 0) // 非選單呼叫端
	if len(c.rows) != 0 {
		t.Fatal("非選單呼叫端不該記錄")
	}
	drawWord(c, m, 24, 3, "Exit", 0, false)
	c.SetPalette(c.pal)
	if len(l.Stamps) != 2 {
		t.Fatalf("要 2 筆（詞＋熱鍵），得到 %d", len(l.Stamps))
	}
	glyphCall(c, m, 3, ' ', 10, 0, 0, 24, 0) // 空白補位
	c.SetPalette(c.pal)
	if len(l.Stamps) != 0 {
		t.Fatal("空白補位後該詞應消失")
	}
}

func TestPathCLineLookupAndOutsideWrite(t *testing.T) {
	c, m, l := newC(t, catalogOf(t, "Play Demo", "開始 示範"))
	drawWord(c, m, 24, 0, "Play", 0, true)
	glyphCall(c, m, 2, ' ', 1, 0, 10, 24, 4)
	drawWord(c, m, 24, 5, "Demo", 0, false)
	c.SetPalette(c.pal)
	if len(l.Stamps) != 3 {
		t.Fatalf("整列查表逐詞對應：要 3 筆（兩詞＋一熱鍵），得到 %d", len(l.Stamps))
	}
	c.VideoWrite(uint32(24*8*planarStride + 5)) // 期間外寫到 Demo 的第一格
	c.SetPalette(c.pal)
	for _, s := range l.Stamps {
		if s.X == 5*8 {
			t.Fatal("期間外寫入後 Demo 已不完整，整列查表不再命中，不該再畫")
		}
	}
}

func TestPathCOverlongTranslationKeepsOriginal(t *testing.T) {
	c, m, l := newC(t, catalogOf(t, "Fix", "修理裝備"))
	drawWord(c, m, 24, 0, "Fix", 0, false)
	c.SetPalette(c.pal)
	if len(l.Stamps) != 0 {
		t.Fatal("譯文超過原詞格數應顯示原版")
	}
}

func TestPathCWholeLinePrompt(t *testing.T) {
	c, m, l := newC(t, catalogOf(t, "Press button or enter to continue", "按任意鍵或 Enter 繼續"))
	col := 0
	for i, w := range strings.Fields("Press button or enter to continue") {
		if i > 0 {
			glyphCall(c, m, 0, ' ', 1, 15, 0, 24, col)
			col++
		}
		drawWord(c, m, 24, col, w, -1, true)
		col += len(w)
	}
	c.SetPalette(c.pal)
	if len(l.Stamps) != 1 || !strings.HasPrefix(string(l.Stamps[0].Text), string(PackHalfWidth([]rune("按任意鍵或 Enter 繼續")))) || l.Stamps[0].Cells != col {
		t.Fatalf("整句提示應畫成一筆：%d 筆 %+v", len(l.Stamps), l.Stamps)
	}
}
