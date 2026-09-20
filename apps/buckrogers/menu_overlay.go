package buckrogers

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wicanr2/dosgolem/xlate"
)

// PixelRect is an output-pixel rectangle with an exclusive right and bottom edge.
type PixelRect struct {
	X, Y, Width, Height int
}

// MenuOverlayEntry is one exact, catalog-backed menu translation plus its
// proven logical text-safe rectangle.
type MenuOverlayEntry struct {
	EventKey, TextKey, Translation string
	Background, Foreground         uint8
	X, Y, Width, Height            int
	DrawX, DrawY                   int
	Capacity, LineCount            int
	Overflow                       string
}

// MenuOverlayGeometry is content-safe receipt metadata for one built stamp.
type MenuOverlayGeometry struct {
	EventKey, TextKey        string
	TranslationRunes         int
	Background, Foreground   uint8
	ClearRect                PixelRect
	DrawAnchorX, DrawAnchorY int
	InkRect                  PixelRect
	Contained                bool
}

// MenuOverlay is an all-or-nothing, scale-explicit set of menu stamps.
type MenuOverlay struct {
	Layer  *xlate.Layer
	Events []MenuOverlayGeometry
}

// BuildMenuOverlay implements docs/spec/015-buck-rogers-menu-overlay-core.md.
// It validates the complete batch before returning a usable layer.
func BuildMenuOverlay(entries []MenuOverlayEntry, font *xlate.Font, palette [256][3]uint8, scale int) (*MenuOverlay, error) {
	if scale <= 0 || scale > int(^uint(0)>>1)/320 {
		return nil, fmt.Errorf("buckrogers: 無效覆繪倍率 %d", scale)
	}
	if font == nil || font.W != 16 || font.H != 16 {
		return nil, fmt.Errorf("buckrogers: 功能選單字型必須是 16x16")
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("buckrogers: 功能選單覆繪不得是空批次")
	}

	layer := &xlate.Layer{W: 320, H: 200}
	events := make([]MenuOverlayGeometry, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.EventKey == "" || entry.TextKey == "" || entry.Translation == "" || !utf8.ValidString(entry.Translation) {
			return nil, fmt.Errorf("buckrogers: 覆繪識別或 UTF-8 譯文無效")
		}
		if seen[entry.EventKey] {
			return nil, fmt.Errorf("buckrogers: 重複覆繪事件 %q", entry.EventKey)
		}
		seen[entry.EventKey] = true
		if entry.X < 0 || entry.Y < 0 || entry.Width <= 0 || entry.Height != 8 ||
			entry.X+entry.Width > 320 || entry.Y+entry.Height > 200 || entry.Width%8 != 0 ||
			entry.DrawX < entry.X || (entry.DrawX-entry.X)%8 != 0 || entry.DrawY != entry.Y ||
			entry.LineCount != 1 || entry.Overflow != "single-line-reject" {
			return nil, fmt.Errorf("buckrogers: %s 不符合單列 8x8 安全矩形契約", entry.EventKey)
		}
		prefix := (entry.DrawX - entry.X) / 8
		cells := entry.Width / 8
		translation := []rune(entry.Translation)
		if entry.Capacity < 0 || cells-prefix != entry.Capacity || prefix+len(translation) > cells {
			return nil, fmt.Errorf("buckrogers: %s 容量不符", entry.EventKey)
		}
		for _, r := range translation {
			glyph, ok := font.Glyphs[r]
			if !ok || len(glyph) != 32 {
				return nil, fmt.Errorf("buckrogers: %s 缺少有效字模 U+%04X", entry.EventKey, r)
			}
		}

		offset := (8*scale - 16) / 2
		if offset < 0 {
			offset = 0
		}
		stamp := &xlate.Stamp{
			Key: entry.EventKey, X: entry.X, Y: entry.Y, Cells: cells, CellW: 8, CellH: 8,
			Font: font, GlyphX: offset, GlyphY: offset,
			Text:  append([]rune(strings.Repeat("　", prefix)), translation...),
			State: xlate.Shown, BG: palette[entry.Background], FG: palette[entry.Foreground],
		}
		clear := PixelRect{entry.X * scale, entry.Y * scale, entry.Width * scale, entry.Height * scale}
		ink, err := menuInkRect(stamp, scale)
		if err != nil {
			return nil, fmt.Errorf("buckrogers: %s: %w", entry.EventKey, err)
		}
		contained := ink.Width > 0 && ink.Height > 0 && ink.X >= clear.X && ink.Y >= clear.Y &&
			ink.X+ink.Width <= clear.X+clear.Width && ink.Y+ink.Height <= clear.Y+clear.Height
		if !contained {
			return nil, fmt.Errorf("buckrogers: %s 字模墨跡超出安全矩形", entry.EventKey)
		}
		geometry := MenuOverlayGeometry{
			EventKey: entry.EventKey, TextKey: entry.TextKey, TranslationRunes: len(translation),
			Background: entry.Background, Foreground: entry.Foreground,
			ClearRect: clear, DrawAnchorX: entry.DrawX * scale, DrawAnchorY: entry.DrawY * scale,
			InkRect: ink, Contained: true,
		}
		for _, prior := range events {
			if pixelRectsOverlap(prior.ClearRect, clear) {
				return nil, fmt.Errorf("buckrogers: %s 的安全矩形與 %s 重疊", entry.EventKey, prior.EventKey)
			}
		}
		layer.Add(stamp)
		events = append(events, geometry)
	}
	return &MenuOverlay{Layer: layer, Events: events}, nil
}

func menuInkRect(stamp *xlate.Stamp, scale int) (PixelRect, error) {
	k := stamp.GlyphScale
	if k == 0 {
		k = scale / 3
		if k < 1 {
			k = 1
		}
	}
	minX, minY, maxX, maxY := int(^uint(0)>>1), int(^uint(0)>>1), -1, -1
	rowBytes := (stamp.Font.W + 7) / 8
	for i, r := range stamp.Text {
		if r == ' ' || r == '　' {
			continue
		}
		glyph, ok := stamp.Font.Glyphs[r]
		if !ok || len(glyph) != stamp.Font.H*rowBytes {
			return PixelRect{}, fmt.Errorf("缺少有效字模 U+%04X", r)
		}
		for gy := 0; gy < stamp.Font.H; gy++ {
			for gx := 0; gx < stamp.Font.W; gx++ {
				if glyph[gy*rowBytes+gx/8]&(0x80>>uint(gx%8)) == 0 {
					continue
				}
				x0 := (stamp.X+i*stamp.CellW)*scale + stamp.GlyphX + gx*k
				y0 := stamp.Y*scale + stamp.GlyphY + gy*k
				x1, y1 := x0+k, y0+k
				if x0 < minX {
					minX = x0
				}
				if y0 < minY {
					minY = y0
				}
				if x1 > maxX {
					maxX = x1
				}
				if y1 > maxY {
					maxY = y1
				}
			}
		}
	}
	if maxX < minX || maxY < minY {
		return PixelRect{}, fmt.Errorf("譯文沒有可見字模墨跡")
	}
	return PixelRect{minX, minY, maxX - minX, maxY - minY}, nil
}

func pixelRectsOverlap(a, b PixelRect) bool {
	return a.X < b.X+b.Width && b.X < a.X+a.Width && a.Y < b.Y+b.Height && b.Y < a.Y+a.Height
}
