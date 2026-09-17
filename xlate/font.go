package xlate

import (
	"encoding/binary"
	"fmt"
	"os"
)

// golemFontMagic 是 GOLEMFNT 字型檔的檔頭識別字串（spec 202 §2.1）。
const golemFontMagic = "GOLEMFNT"

// golemFontHeaderLen 是檔頭長度：magic(8) + W(2) + H(2) + 字數(4)。
const golemFontHeaderLen = 8 + 2 + 2 + 4

// Font 是一組點陣字模。每字 H 列，每列 (W+7)/8 bytes，MSB 在左（spec 202 §2.1）。
//
// Name 不是檔案格式的一部分，是呼叫端載入後自己標記的識別名——Snapshot／Restore
// 靠它把 Stamp 引用的字型記成一個名字，而不是把整份字模也存進快照（§2.3）。
// 沒設 Name 的字型不能被 Snapshot（Stamp 會被記成沒有字型）。
type Font struct {
	W, H   int
	Glyphs map[rune][]byte
	Name   string
}

// rowBytes 回這個字型一列字模佔幾個 byte。
func (f *Font) rowBytes() int { return (f.W + 7) / 8 }

// LoadFont 讀一份 GOLEMFNT 格式的字型檔。
//
// 檔頭：magic「GOLEMFNT」8 bytes、u16 W、u16 H（小端）、u32 字數。
// 之後每字：u32 碼點 ＋ u8 來源（呼叫端自訂，這裡不解讀、不保留）＋ 字模
// （H×(W+7)/8 bytes）。長度對不上、magic 不對都回錯。
//
// 回傳的 Font.Name 是空字串；要參與 Snapshot／Restore 的話由呼叫端自行設定。
func LoadFont(path string) (*Font, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) < golemFontHeaderLen || string(b[:8]) != golemFontMagic {
		return nil, fmt.Errorf("xlate: %s 不是 %s 字型", path, golemFontMagic)
	}
	w := int(binary.LittleEndian.Uint16(b[8:10]))
	h := int(binary.LittleEndian.Uint16(b[10:12]))
	n := int(binary.LittleEndian.Uint32(b[12:16]))

	f := &Font{W: w, H: h, Glyphs: make(map[rune][]byte, n)}
	rowBytes := f.rowBytes()
	glyphSize := 4 + 1 + h*rowBytes
	want := golemFontHeaderLen + n*glyphSize
	if len(b) != want {
		return nil, fmt.Errorf("xlate: %s 長度 %d，%d 字 %dx%d 應該是 %d", path, len(b), n, w, h, want)
	}

	for i := 0; i < n; i++ {
		o := golemFontHeaderLen + i*glyphSize
		cp := rune(binary.LittleEndian.Uint32(b[o : o+4]))
		// b[o+4] 是來源標記，呼叫端自訂，這裡不解讀。
		glyph := make([]byte, h*rowBytes)
		copy(glyph, b[o+5:o+5+h*rowBytes])
		f.Glyphs[cp] = glyph
	}
	return f, nil
}
