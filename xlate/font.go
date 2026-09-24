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

// ParseFont 解析一份已在呼叫端取得的 GOLEMFNT 位元組。
//
// 檔頭：magic「GOLEMFNT」8 bytes、u16 W、u16 H（小端）、u32 字數。
// 之後每字：u32 碼點 ＋ u8 來源（呼叫端自訂，這裡不解讀、不保留）＋ 字模
// （H×(W+7)/8 bytes）。長度對不上、magic 不對都回錯。
//
// 回傳的 Font.Name 是空字串；要參與 Snapshot／Restore 的話由呼叫端自行設定。
func ParseFont(b []byte) (*Font, error) {
	if len(b) < golemFontHeaderLen || string(b[:8]) != golemFontMagic {
		return nil, fmt.Errorf("xlate: 輸入不是 %s 字型", golemFontMagic)
	}
	w := int(binary.LittleEndian.Uint16(b[8:10]))
	h := int(binary.LittleEndian.Uint16(b[10:12]))
	declaredCount := binary.LittleEndian.Uint32(b[12:16])

	rowBytes := (w + 7) / 8
	glyphSize := 4 + 1 + h*rowBytes
	remaining := len(b) - golemFontHeaderLen
	if glyphSize <= 0 || remaining%glyphSize != 0 || uint64(declaredCount) != uint64(remaining/glyphSize) {
		return nil, fmt.Errorf("xlate: 字型長度 %d，宣告 %d 字 %dx%d 不相符", len(b), declaredCount, w, h)
	}
	n := remaining / glyphSize // Proven by the bounded byte slice before map allocation.
	f := &Font{W: w, H: h, Glyphs: make(map[rune][]byte, n)}

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

// LoadFont reads a GOLEMFNT file and delegates the format parsing to
// ParseFont. Callers that must bind a hash to the exact parsed bytes should
// read and verify those bytes themselves, then call ParseFont directly.
func LoadFont(path string) (*Font, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseFont(b)
}
