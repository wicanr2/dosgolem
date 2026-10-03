package phantasie

import (
	"encoding/binary"
	"fmt"
	"os"
	"unicode"
)

// 字型寬度表（docs/spec/004 §3、001 §6）。
//
// 全形或半形由字型決定，不由碼點範圍決定（例如 U+2026、U+2460、U+2606 在 Unifont 是 16 px 寬）。
// tools/build_font.py 把每個字的寬度記在 GOLEMFNT 每字的「來源」位元組：bit 7 為 1 是全形（16 px 寬），
// 低 7 位元是來源編號（Unifont 為 1）。xlate.ParseFont 不保留該位元組，所以這裡自己解析檔頭與每字的
// 來源位元組（同一份檔案再讀一次）。
//
// GOLEMFNT 檔頭：magic「GOLEMFNT」8 bytes、u16 W、u16 H、u32 字數（小端）；之後每字
// u32 碼點、u8 來源、H×((W+7)/8) bytes 的位元圖。

const (
	fwMagic     = "GOLEMFNT"
	fwHeaderLen = 8 + 2 + 2 + 4
	fwWideBit   = 0x80
)

// WideTable 記錄字型收入了哪些碼點，以及每個碼點是不是全形。零值與 nil 都是空表。
type WideTable struct {
	wide map[rune]bool // 碼點 -> 是否全形；鍵的集合就是字型收入的字
}

// ParseFontWide 解析一份 GOLEMFNT 位元組，取出每字的寬度。
//
// 長度檢查與 xlate.ParseFont 相同（宣告的字數、W、H 與實際長度必須吻合，沒有多餘的位元組）；
// 另外 W 或 H 為 0、碼點超過 U+10FFFF 也視為損毀，回 error。碼點重複時以後出現者為準
// （與 xlate.ParseFont 的 map 一致）。任何輸入都不會 panic。
func ParseFontWide(data []byte) (*WideTable, error) {
	if len(data) < fwHeaderLen || string(data[:8]) != fwMagic {
		return nil, fmt.Errorf("輸入不是 %s 字型", fwMagic)
	}
	w := int(binary.LittleEndian.Uint16(data[8:10]))
	h := int(binary.LittleEndian.Uint16(data[10:12]))
	declared := binary.LittleEndian.Uint32(data[12:16])
	if w == 0 || h == 0 {
		return nil, fmt.Errorf("字型尺寸異常：W=%d H=%d", w, h)
	}
	glyphSize := 4 + 1 + h*((w+7)/8)
	remaining := len(data) - fwHeaderLen
	if glyphSize <= 0 || remaining%glyphSize != 0 || uint64(declared) != uint64(remaining/glyphSize) {
		return nil, fmt.Errorf("字型長度 %d，宣告 %d 字 %dx%d 不相符", len(data), declared, w, h)
	}
	n := remaining / glyphSize // 已由位元組長度限定上界，配置 map 前不會失控
	t := &WideTable{wide: make(map[rune]bool, n)}
	for i := 0; i < n; i++ {
		o := fwHeaderLen + i*glyphSize
		cp := binary.LittleEndian.Uint32(data[o : o+4])
		if cp > unicode.MaxRune {
			return nil, fmt.Errorf("第 %d 字的碼點 0x%X 超過 U+10FFFF", i, cp)
		}
		t.wide[rune(cp)] = data[o+4]&fwWideBit != 0
	}
	return t, nil
}

// LoadFontWide 讀入 GOLEMFNT 檔並解析寬度表。
func LoadFontWide(path string) (*WideTable, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	t, err := ParseFontWide(data)
	if err != nil {
		return nil, fmt.Errorf("%s：%w", path, err)
	}
	return t, nil
}

// Wide 回字元是不是全形（16 像素寬，2 h）。字型沒有收入的字元回 false；缺字另用 Has 檢查。
func (t *WideTable) Wide(r rune) bool {
	if t == nil {
		return false
	}
	return t.wide[r]
}

// Has 回字型有沒有收入該字（缺字檢查用）。
func (t *WideTable) Has(r rune) bool {
	if t == nil {
		return false
	}
	_, ok := t.wide[r]
	return ok
}

// Len 回字型收入的字數（碼點去重後）。
func (t *WideTable) Len() int {
	if t == nil {
		return 0
	}
	return len(t.wide)
}

// Func 回 WideFunc，語意同 Wide。
func (t *WideTable) Func() WideFunc { return t.Wide }
