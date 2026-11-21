package dkk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/xlate"
)

// fakeMem 是 1 MB 線性記憶體，只供合成測試；不含任何原版 bytes 以外的簽章片段。
type fakeMem []byte

func (m fakeMem) Read8(a uint32) uint8   { return m[a] }
func (m fakeMem) Read16(a uint32) uint16 { return uint16(m[a]) | uint16(m[a+1])<<8 }
func (m fakeMem) put16(a uint32, v uint16) {
	m[a], m[a+1] = byte(v), byte(v>>8)
}

func catalogFor(t *testing.T, original, key, zh string) *Catalog {
	t.Helper()
	sum := sha256.Sum256([]byte(original))
	events := fmt.Sprintf("event_key\toriginal_length\toriginal_sha256\tsources\n%s\t%d\t%s\tSTART.EXE:1\n", key, len(original), hex.EncodeToString(sum[:]))
	trans := fmt.Sprintf("key\ttranslation\tsource\n%s\t%s\ttest\n", key, zh)
	c, err := LoadCatalog([]byte(events), []byte(trans))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func plantSignature(m fakeMem, at uint32, wild byte) {
	for i, b := range PathASignature.Bytes {
		if PathASignature.Mask[i] {
			m[at+uint32(i)] = b
		} else {
			m[at+uint32(i)] = wild
		}
	}
}

func TestSignatureMaskIgnoresRelocatedWords(t *testing.T) {
	m := make(fakeMem, 0x100000)
	plantSignature(m, 0x62AA, 0x5A)
	// 前 24 bytes 相同、第 28 byte 不同的另一處（START.EXE 0x609B 的情形）不能命中。
	plantSignature(m, 0x7000, 0x11)
	m[0x7000+27] = 0x8A
	hits := PathASignature.Find(m.Read8, 0, 0xA0000)
	if len(hits) != 1 || hits[0] != 0x62AA {
		t.Fatalf("命中 %v，要只有 0x62AA", hits)
	}
}

func TestNewPathARejectsAmbiguousSignature(t *testing.T) {
	m := make(fakeMem, 0x100000)
	plantSignature(m, 0x62AA, 0)
	plantSignature(m, 0x8000, 0)
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	if _, err := NewPathA(m, nil, font, &xlate.Layer{}); err == nil {
		t.Fatal("兩處命中應該失敗，不能猜位址")
	}
}

// callFrame 依規格 001 §1 的版面在堆疊放一次呼叫：返回位址、字串遠指標、bg、fg、列、欄（高位元組填殘值）。
func callFrame(m fakeMem, ss, sp uint16, retSeg, retOff, strSeg, strOff uint16, bg, fg, row, col uint8) {
	base := linear(ss, sp)
	m.put16(base, retOff)
	m.put16(base+2, retSeg)
	m.put16(base+4, strOff)
	m.put16(base+6, strSeg)
	m.put16(base+8, 0x7200|uint16(bg))
	m.put16(base+10, 0x7200|uint16(fg))
	m.put16(base+12, 0x7200|uint16(row))
	m.put16(base+14, 0x7200|uint16(col))
}

func TestPathAStampGeometryAndReturn(t *testing.T) {
	m := make(fakeMem, 0x100000)
	plantSignature(m, 0x62AA, 0)
	text := "Exit to DOS"
	strBase := linear(0x5777, 8)
	m[strBase] = byte(len(text))
	copy(m[strBase+1:], text)
	callFrame(m, 0x1CBD, 0x3F00, 0x34CF, 0x1605, 0x5777, 8, 0, 10, 16, 9)
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	layer := &xlate.Layer{}
	p, err := NewPathA(m, catalogFor(t, text, "ui.test", "離開到 DOS"), font, layer)
	if err != nil {
		t.Fatal(err)
	}
	p.Before(1, 0x05F4, 0x036A, 0x1CBD, 0x3F00, m)
	if len(layer.Stamps) != 1 {
		t.Fatalf("要 1 筆 Stamp，得到 %d（hits %+v）", len(layer.Stamps), p.Hits)
	}
	s := layer.Stamps[0]
	if s.X != 72 || s.Y != 128 || s.Cells != len(text) || s.CellW != 8 || s.CellH != 8 {
		t.Fatalf("幾何不對：%+v", s)
	}
	if want := string(PackHalfWidth([]rune("離開到 DOS"))) + "      "; len(s.Text) != len(text) || string(s.Text) != want {
		t.Fatalf("Text 應為譯文補空白到 %d 格：%q", len(text), string(s.Text))
	}
	if s.State != xlate.Printing {
		t.Fatalf("進入時應為 Printing")
	}
	// SP 不對的同位址（遞迴或別的框）不能轉狀態。
	p.Before(2, 0x34CF, 0x1605, 0x1CBD, 0x3F00, m)
	if s.State != xlate.Printing {
		t.Fatal("SP 不符時不該轉 Pending")
	}
	p.Before(3, 0x34CF, 0x1605, 0x1CBD, 0x3F00+4+argBytes, m)
	if s.State != xlate.Pending {
		t.Fatal("返回後應轉 Pending")
	}
	h := p.Hits[0]
	if h.Row != 16 || h.Col != 9 || h.BG != 0 || h.FG != 10 || h.Key != "ui.test" || !h.Stamped {
		t.Fatalf("hit 紀錄不對：%+v", h)
	}
}

func TestPathARejectsLongTranslationAndMiss(t *testing.T) {
	m := make(fakeMem, 0x100000)
	plantSignature(m, 0x62AA, 0)
	text := "AC"
	strBase := linear(0x5777, 8)
	m[strBase] = byte(len(text))
	copy(m[strBase+1:], text)
	callFrame(m, 0x1CBD, 0x3F00, 0x34CF, 0x1605, 0x5777, 8, 0, 10, 2, 30)
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	layer := &xlate.Layer{}
	p, err := NewPathA(m, catalogFor(t, text, "ui.ac", "防護力"), font, layer)
	if err != nil {
		t.Fatal(err)
	}
	p.Before(1, 0x05F4, 0x036A, 0x1CBD, 0x3F00, m)
	if len(layer.Stamps) != 0 || p.Hits[0].Rejected != "譯文超過原文格數" {
		t.Fatalf("超長譯文應拒絕：%+v", p.Hits)
	}
	m[strBase+1] = 'X'
	p.Before(2, 0x05F4, 0x036A, 0x1CBD, 0x3F00, m)
	if p.Hits[1].Rejected != "未命中" {
		t.Fatalf("未命中應記錄：%+v", p.Hits[1])
	}
}

func TestLoadCatalogKeepsQuotes(t *testing.T) {
	orig := `"THE FAT MAN"`
	sum := sha256.Sum256([]byte(orig))
	events := fmt.Sprintf("event_key\toriginal_length\toriginal_sha256\tsources\nk1\t%d\t%s\tX:1\nk2\t2\t%064d\tX:2\n", len(orig), hex.EncodeToString(sum[:]), 0)
	trans := "key\ttranslation\tsource\nk1\t「胖子」\tt\nk2\t乙\tt\n"
	c, err := LoadCatalog([]byte(events), []byte(trans))
	if err != nil {
		t.Fatal(err)
	}
	if c.Len() != 2 {
		t.Fatalf("含引號的列不該吞掉下一列：收錄 %d 筆", c.Len())
	}
	if e, ok := c.Lookup([]byte(orig)); !ok || e.Translation != "「胖子」" {
		t.Fatalf("查不到含引號原文：%+v", e)
	}
}

func TestPathAPaddedListEntryUsesTrimmedKey(t *testing.T) {
	m := make(fakeMem, 0x100000)
	plantSignature(m, 0x62AA, 0)
	text := "  Human "
	strBase := linear(0x5777, 8)
	m[strBase] = byte(len(text))
	copy(m[strBase+1:], text)
	callFrame(m, 0x1CBD, 0x3F00, 0x34CF, 0x1605, 0x5777, 8, 0, 10, 8, 6)
	font := &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}
	layer := &xlate.Layer{}
	p, err := NewPathA(m, catalogFor(t, "Human", "ui.human", "人類"), font, layer)
	if err != nil {
		t.Fatal(err)
	}
	p.Before(1, 0x05F4, 0x036A, 0x1CBD, 0x3F00, m)
	if len(layer.Stamps) != 1 {
		t.Fatalf("縮排加補白的清單項應以去頭尾空白的 key 命中：%+v", p.Hits)
	}
	if s := layer.Stamps[0]; s.X != (6+2)*8 || s.Cells != len(text)-2 {
		t.Fatalf("Stamp 應從縮排後開始、涵蓋到原字串結尾：X=%d Cells=%d", s.X, s.Cells)
	}
}

func TestPrepareFrameSwapsWhenInkDominates(t *testing.T) {
	layer := &xlate.Layer{}
	st := &xlate.Stamp{X: 0, Y: 0, Cells: 2, CellW: 8, CellH: 8, State: xlate.Pending}
	layer.Add(st)
	p := &PathA{Layer: layer, bgOf: map[*xlate.Stamp]uint8{st: 0}}
	idx := make([]uint8, 320*200)
	for y := 0; y < 8; y++ {
		for x := 0; x < 16; x++ {
			if (x+y)%3 != 0 {
				idx[y*320+x] = 10 // 墨跡（fg 10）多於背景 0
			}
		}
	}
	p.PrepareFrame(idx, 320, 200)
	if !st.SwapColors {
		t.Fatal("墨跡多於底色時應設 SwapColors")
	}
	for i := range idx {
		idx[i] = 0
	}
	idx[0] = 10
	p.PrepareFrame(idx, 320, 200)
	if st.SwapColors {
		t.Fatal("底色多時不該反轉")
	}
}

func TestPathAUntranslatedPrintInvalidatesOldStamp(t *testing.T) {
	m := make(fakeMem, 0x100000)
	plantSignature(m, 0x62AA, 0)
	put := func(s string) {
		b := linear(0x5777, 8)
		m[b] = byte(len(s))
		copy(m[b+1:], s)
	}
	layer := &xlate.Layer{}
	p, err := NewPathA(m, catalogFor(t, "Name", "ui.name", "姓名"), &xlate.Font{W: 16, H: 16, Glyphs: map[rune][]byte{}}, layer)
	if err != nil {
		t.Fatal(err)
	}
	put("Name")
	callFrame(m, 0x1CBD, 0x3F00, 0x34CF, 0x1605, 0x5777, 8, 0, 15, 2, 17)
	p.Before(1, 0x05F4, 0x036A, 0x1CBD, 0x3F00, m)
	if len(layer.Stamps) != 1 {
		t.Fatalf("要 1 筆，得到 %d", len(layer.Stamps))
	}
	put(" 4   ") // 新畫面在舊「Name」範圍印未翻譯的數字（查驗題字序）
	callFrame(m, 0x1CBD, 0x3F00, 0x34CF, 0x1605, 0x5777, 8, 0, 10, 2, 16)
	p.Before(2, 0x05F4, 0x036A, 0x1CBD, 0x3F00, m)
	if len(layer.Stamps) != 0 {
		t.Fatalf("舊 Stamp 被原版字整個蓋住應移除，剩 %d 筆", len(layer.Stamps))
	}
}
