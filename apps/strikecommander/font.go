// Package strikecommander 是《Strike Commander》（陸空戰將）的中文化 adapter。
//
// 目前是 OPTTEST 熱點標籤的覆繪（strike_commander repo 的 docs/spec/001、002）：唯讀觀察
// 標籤常式在 0CF2:082A 印出的那一行，於畫格 blit 完成後提交，規劃 xlate stamp；
// 以及計分板的白板欄名與底部標籤（docs/spec/003），呼叫端在 overlay 區的計分板程式。
// 遊戲位址、簽章與呼叫端規則只放這個套件，不加進 xlate、session 或 internal（001 §2.1）。
package strikecommander

import (
	"encoding/binary"
	"fmt"
	"os"
	"unicode"

	"github.com/wicanr2/dosgolem/xlate"
)

// GOLEMFNT 檔頭與每字的來源位元組（001 §2.7；phantasie 規格 004 §3）。
//
// tools/font/build_font.py 依 Unifont .hex 的行長把每字寬度寫進來源位元組：
// bit 7 為 1 是全形（16 像素寬），低 7 位元是來源編號（Unifont 為 1）。
// xlate.ParseFont 讀過即丟，這裡對同一份位元組再解析一次。
const (
	fntMagic     = "GOLEMFNT"
	fntHeaderLen = 8 + 2 + 2 + 4
	fntWideBit   = 0x80
)

// WideTable 記字型收入的碼點與各自是否全形。nil 是空表。
type WideTable struct {
	wide map[rune]bool
}

// ParseWide 解析 GOLEMFNT 位元組的寬度表。長度檢查與 xlate.ParseFont 相同。
func ParseWide(b []byte) (*WideTable, error) {
	if len(b) < fntHeaderLen || string(b[:8]) != fntMagic {
		return nil, fmt.Errorf("strikecommander: 輸入不是 %s 字型", fntMagic)
	}
	w := int(binary.LittleEndian.Uint16(b[8:10]))
	h := int(binary.LittleEndian.Uint16(b[10:12]))
	declared := binary.LittleEndian.Uint32(b[12:16])
	if w == 0 || h == 0 {
		return nil, fmt.Errorf("strikecommander: 字型尺寸 %dx%d 不合法", w, h)
	}
	glyphSize := 4 + 1 + h*((w+7)/8)
	remaining := len(b) - fntHeaderLen
	if remaining%glyphSize != 0 || uint64(declared) != uint64(remaining/glyphSize) {
		return nil, fmt.Errorf("strikecommander: 字型長度 %d，宣告 %d 字 %dx%d 不相符", len(b), declared, w, h)
	}
	n := remaining / glyphSize
	t := &WideTable{wide: make(map[rune]bool, n)}
	for i := 0; i < n; i++ {
		o := fntHeaderLen + i*glyphSize
		cp := binary.LittleEndian.Uint32(b[o : o+4])
		if cp > unicode.MaxRune {
			return nil, fmt.Errorf("strikecommander: 第 %d 字碼點 0x%X 超出範圍", i, cp)
		}
		t.wide[rune(cp)] = b[o+4]&fntWideBit != 0
	}
	return t, nil
}

// Wide 回字型收入了 r 且 r 是全形。
func (t *WideTable) Wide(r rune) bool { return t != nil && t.wide[r] }

// Has 回字型收入了 r（不論寬度）。
func (t *WideTable) Has(r rune) bool {
	if t == nil {
		return false
	}
	_, ok := t.wide[r]
	return ok
}

// Len 回收入的字數。
func (t *WideTable) Len() int {
	if t == nil {
		return 0
	}
	return len(t.wide)
}

// LabelFont 是熱點標籤用的 16×16 字型與它的全形表。
type LabelFont struct {
	Font *xlate.Font
	Wide *WideTable
}

// ParseLabelFont 以同一份位元組建出 xlate.Font 與全形表。字型必須是 16×16。
func ParseLabelFont(b []byte, name string) (*LabelFont, error) {
	f, err := xlate.ParseFont(b)
	if err != nil {
		return nil, err
	}
	if f.W != 16 || f.H != 16 {
		return nil, fmt.Errorf("strikecommander: 字型 %dx%d，要 16x16", f.W, f.H)
	}
	wt, err := ParseWide(b)
	if err != nil {
		return nil, err
	}
	f.Name = name
	return &LabelFont{Font: f, Wide: wt}, nil
}

// LoadLabelFont 讀檔後呼叫 ParseLabelFont。
func LoadLabelFont(path, name string) (*LabelFont, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseLabelFont(b, name)
}

// HUDFont 是飛行 HUD 狀態字用的 12×12 字型（HUD12，Fusion Pixel 12px，strike_commander 規格 009 §3.9）與它的寬度表。
type HUDFont struct {
	Font *xlate.Font
	Wide *WideTable
}

// ParseHUDFont 以同一份位元組建出 xlate.Font 與寬度表。字型必須是 12×12。
func ParseHUDFont(b []byte, name string) (*HUDFont, error) {
	f, err := xlate.ParseFont(b)
	if err != nil {
		return nil, err
	}
	if f.W != 12 || f.H != 12 {
		return nil, fmt.Errorf("strikecommander: HUD 字型 %dx%d，要 12x12", f.W, f.H)
	}
	wt, err := ParseWide(b)
	if err != nil {
		return nil, err
	}
	f.Name = name
	return &HUDFont{Font: f, Wide: wt}, nil
}

// LoadHUDFont 讀檔後呼叫 ParseHUDFont。
func LoadHUDFont(path, name string) (*HUDFont, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseHUDFont(b, name)
}
