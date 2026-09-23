package ebiten

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/dosgolem/xlate"
)

// HostFont3 keeps native 24-point wide and ASCII glyphs separate. The source
// files remain caller-owned local inputs; this type never embeds font bytes.
type HostFont3 struct {
	Wide  *xlate.Font // native 24x24 Chinese and full-width symbols
	ASCII *xlate.Font // native 16x24 ASCII digits
}

func (f *HostFont3) glyph(r rune) (*xlate.Font, []byte, error) {
	if f == nil {
		return nil, nil, fmt.Errorf("frontend/ebiten: 3× host 字型不得為 nil")
	}
	var face *xlate.Font
	switch {
	case r >= '0' && r <= '9':
		face = f.ASCII
	case r > 127:
		face = f.Wide
	default:
		return nil, nil, fmt.Errorf("frontend/ebiten: 3× host 標籤不支援字元 %q", r)
	}
	if face == nil || face.W <= 0 || face.H <= 0 {
		return nil, nil, fmt.Errorf("frontend/ebiten: 3× host 字型來源缺失：%q", r)
	}
	glyph, ok := face.Glyphs[r]
	if !ok || len(glyph) != face.H*((face.W+7)/8) {
		return nil, nil, fmt.Errorf("frontend/ebiten: 3× host 字型缺少或損壞字元 %q", r)
	}
	hasInk := false
	for _, b := range glyph {
		hasInk = hasInk || b != 0
	}
	if !hasInk {
		return nil, nil, fmt.Errorf("frontend/ebiten: 3× host 字型 %q 沒有可見墨跡", r)
	}
	return face, glyph, nil
}

func validateHostFont3(font *HostFont3, labels HostLabels) error {
	if font == nil || font.Wide == nil || font.ASCII == nil ||
		font.Wide.W != 24 || font.Wide.H != 24 || font.ASCII.W != 16 || font.ASCII.H != 24 {
		return fmt.Errorf("frontend/ebiten: 3× host 字型必須有 24x24 Wide 與 16x24 ASCII")
	}
	for _, item := range []struct {
		text string
		at   image.Point
		safe image.Rectangle
	}{
		{labels.Settings, image.Pt(744, 8), image.Rect(732, 6, 948, 45)},
		{labels.Scale2, image.Pt(36, 114), image.Rect(24, 105, 186, 174)},
		{labels.Scale3, image.Pt(216, 114), image.Rect(204, 105, 366, 174)},
		{labels.Apply, image.Pt(450, 198), image.Rect(435, 189, 639, 261)},
		{labels.Cancel, image.Pt(675, 198), image.Rect(660, 189, 864, 261)},
	} {
		if item.text == "" {
			return fmt.Errorf("frontend/ebiten: 3× host label 為空")
		}
		advance, ink, err := measureHostText3(font, item.at, item.text)
		if err != nil {
			return err
		}
		if !insideHostRect(item.safe, advance) || !insideHostRect(item.safe, ink) {
			return fmt.Errorf("frontend/ebiten: 3× host 字型不容於 %q 安全矩形", item.text)
		}
	}
	return nil
}

func insideHostRect(safe, got image.Rectangle) bool {
	return got.Min.X >= safe.Min.X && got.Min.Y >= safe.Min.Y &&
		got.Max.X <= safe.Max.X && got.Max.Y <= safe.Max.Y
}

// measureHostText3 and drawText3 select glyphs through the same typed gate.
// Rectangles use exclusive Max coordinates, like image.Rectangle.
func measureHostText3(font *HostFont3, at image.Point, label string) (image.Rectangle, image.Rectangle, error) {
	x := at.X
	ink := image.Rectangle{}
	hasInk := false
	for _, r := range label {
		face, glyph, err := font.glyph(r)
		if err != nil {
			return image.Rectangle{}, image.Rectangle{}, err
		}
		rowBytes := (face.W + 7) / 8
		for y := 0; y < face.H; y++ {
			for gx := 0; gx < face.W; gx++ {
				if glyph[y*rowBytes+gx/8]&(0x80>>uint(gx%8)) == 0 {
					continue
				}
				px, py := x+gx, at.Y+y
				if !hasInk {
					ink = image.Rect(px, py, px+1, py+1)
					hasInk = true
				} else {
					if px < ink.Min.X {
						ink.Min.X = px
					}
					if py < ink.Min.Y {
						ink.Min.Y = py
					}
					if px+1 > ink.Max.X {
						ink.Max.X = px + 1
					}
					if py+1 > ink.Max.Y {
						ink.Max.Y = py + 1
					}
				}
			}
		}
		x += face.W
	}
	return image.Rect(at.X, at.Y, x, at.Y+24), ink, nil
}

func (g *Game) drawText3(screen *ebiten.Image, x, y int, label string, c color.Color) error {
	for _, r := range label {
		face, glyph, err := g.font3.glyph(r)
		if err != nil {
			return err
		}
		rowBytes := (face.W + 7) / 8
		for gy := 0; gy < face.H; gy++ {
			for gx := 0; gx < face.W; gx++ {
				if glyph[gy*rowBytes+gx/8]&(0x80>>uint(gx%8)) != 0 {
					screen.Set(x+gx, y+gy, c)
				}
			}
		}
		x += face.W
	}
	return nil
}
