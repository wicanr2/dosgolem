package xlate

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"slices"
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

// canonicalFontSHA256 is the stable identity used by physical-pixel snapshots.
// It deliberately hashes parsed glyph semantics, not a GOLEMFNT source file.
func canonicalFontSHA256(f *Font) ([32]byte, error) {
	var zero [32]byte
	if f == nil || f.W <= 0 || f.H <= 0 || uint64(f.W) > uint64(^uint32(0)) || uint64(f.H) > uint64(^uint32(0)) {
		return zero, fmt.Errorf("xlate: invalid font dimensions")
	}
	rb := f.rowBytes()
	if rb <= 0 || f.H > int(^uint(0)>>1)/rb {
		return zero, fmt.Errorf("xlate: invalid font bitmap dimensions")
	}
	want := f.H * rb
	keys := make([]rune, 0, len(f.Glyphs))
	for r, bitmap := range f.Glyphs {
		if uint64(r) > uint64(^uint32(0)) || len(bitmap) != want {
			return zero, fmt.Errorf("xlate: malformed font glyph U+%04X", r)
		}
		keys = append(keys, r)
	}
	slices.Sort(keys)
	h := sha256.New()
	h.Write([]byte("xlate-font-v1\x00"))
	var u32 [4]byte
	var u64 [8]byte
	binary.LittleEndian.PutUint32(u32[:], uint32(f.W))
	h.Write(u32[:])
	binary.LittleEndian.PutUint32(u32[:], uint32(f.H))
	h.Write(u32[:])
	binary.LittleEndian.PutUint64(u64[:], uint64(len(keys)))
	h.Write(u64[:])
	for _, r := range keys {
		binary.LittleEndian.PutUint32(u32[:], uint32(r))
		h.Write(u32[:])
		bitmap := f.Glyphs[r]
		binary.LittleEndian.PutUint64(u64[:], uint64(len(bitmap)))
		h.Write(u64[:])
		h.Write(bitmap)
	}
	copy(zero[:], h.Sum(nil))
	return zero, nil
}

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
