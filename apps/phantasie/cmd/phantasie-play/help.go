package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/wicanr2/dosgolem/apps/phantasie"
	"github.com/wicanr2/dosgolem/xlate"
)

var helpKeys = []string{"title", "pause", "open", "close", "language", "theme", "fullscreen", "arrows", "confirm", "letters", "context", "footer"}

type helpPage struct {
	lines []string
	font  *xlate.Font
	wide  phantasie.WideFunc
}

func parseHelp(data []byte, font *xlate.Font, wide phantasie.WideFunc, ascii bool) (*helpPage, error) {
	if len(data) > 72*1024 || !utf8.Valid(data) || !strings.HasSuffix(string(data), "\n") {
		return nil, fmt.Errorf("Help 須為有尾端換行的 UTF-8，限 72 KiB")
	}
	rows := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(rows) != len(helpKeys)+1 || rows[0] != "key\ttranslation\tsource" {
		return nil, fmt.Errorf("Help 欄名或列數不符")
	}
	values := map[string]string{}
	for _, row := range rows[1:] {
		fields := strings.Split(row, "\t")
		if len(fields) != 3 || fields[0] == "" || fields[1] == "" || fields[2] == "" {
			return nil, fmt.Errorf("Help 欄數或必要欄位不符")
		}
		for _, field := range fields {
			for _, c := range field {
				if unicode.IsControl(c) || c == '\\' || c == '\ufeff' {
					return nil, fmt.Errorf("Help 含控制碼或不接受的跳脫")
				}
			}
		}
		if _, exists := values[fields[0]]; exists {
			return nil, fmt.Errorf("Help 重複鍵")
		}
		values[fields[0]] = fields[1]
	}
	page := &helpPage{font: font, wide: wide}
	for _, key := range helpKeys {
		line, ok := values[key]
		if !ok {
			return nil, fmt.Errorf("Help 缺必要鍵 %s", key)
		}
		width := 0
		for _, c := range line {
			if ascii {
				if c < 32 || c > 126 {
					return nil, fmt.Errorf("English Help 須為 ASCII")
				}
				width += 8
			} else {
				if font == nil || font.W != 16 || font.H != 16 || wide == nil || len(font.Glyphs[c]) != 32 {
					return nil, fmt.Errorf("Help 缺完整字模 U+%04X", c)
				}
				width += 8
				if wide(c) {
					width += 8
				}
			}
		}
		if width > 576 {
			return nil, fmt.Errorf("Help 行寬超過 576 像素")
		}
		page.lines = append(page.lines, line)
	}
	return page, nil
}

func loadHelpPages(s *phantasie.Session, dir string) map[string]*helpPage {
	pages := map[string]*helpPage{}
	for _, name := range s.DisplayCycle() {
		var font *xlate.Font
		var wide phantasie.WideFunc
		if l := s.Ov.Language(name); l != nil {
			font, wide = l.Font, l.Wide
		}
		data, err := os.ReadFile(filepath.Join(dir, "help."+name+".tsv"))
		var p *helpPage
		if err == nil {
			p, err = parseHelp(data, font, wide, name == "en")
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s 操作說明未啟用，使用英文回退：%v\n", name, err)
			continue
		}
		pages[name] = p
	}
	return pages
}

func (g *game) drawHelp(screen *ebiten.Image) {
	p := g.help[g.s.Ov.Display()]
	fallback := p == nil
	if fallback {
		p = g.help["en"]
	}
	bg, fg := [4]byte{12, 20, 34, 255}, [4]byte{232, 237, 247, 255}
	accent := [4]byte{244, 215, 136, 255}
	if g.theme == "amber" {
		bg, fg, accent = [4]byte{16, 12, 4, 255}, [4]byte{240, 180, 60, 255}, [4]byte{255, 191, 63, 255}
	}
	if p == nil || p.font == nil {
		screen.Fill(color.RGBA{R: bg[0], G: bg[1], B: bg[2], A: 255})
		if fallback {
			ebitenutil.DebugPrintAt(screen, "Help unavailable in this language; English fallback.", 32, 12)
		}
		if p == nil {
			ebitenutil.DebugPrintAt(screen, "Help unavailable. F1 / Esc: close.", 32, 34)
			return
		}
		for i, line := range p.lines {
			ebitenutil.DebugPrintAt(screen, line, 32, 34+i*26)
		}
		return
	}
	for i := 0; i < len(g.rgba); i += 4 {
		copy(g.rgba[i:i+4], bg[:])
	}
	for i, line := range p.lines {
		x, y := 32, 34+i*26
		ink := fg
		if i == 0 || i == len(p.lines)-1 {
			ink = accent
		}
		for _, c := range line {
			glyph, width := p.font.Glyphs[c], 8
			if p.wide(c) {
				width = 16
			}
			for gy := 0; gy < 16; gy++ {
				for gx := 0; gx < width; gx++ {
					if glyph[gy*2+gx/8]&(0x80>>(gx%8)) != 0 {
						at := ((y+gy)*640 + x + gx) * 4
						copy(g.rgba[at:at+4], ink[:])
					}
				}
			}
			x += width
		}
	}
	g.img.WritePixels(g.rgba)
	screen.DrawImage(g.img, nil)
}
