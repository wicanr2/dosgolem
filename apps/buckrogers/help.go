package buckrogers

// 前端說明頁（`docs/spec/241` §3.3）：文字來自 Buck repo 的
// text/host-ui.zh-TW.tsv（key 以 `help.` 開頭、source 為 frontend-help），
// 依 key 排序成行。載入時逐字核對字模，缺字即失敗。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wicanr2/dosgolem/xlate"
)

const (
	helpColumns = 38 // 每行最多格數（每格 8 邏輯像素）
	helpX       = 8
	helpY       = 12
	helpPitch   = 12 // 行距（邏輯像素）
	helpMaxRows = 15
)

// LoadHelp 讀說明頁的行。
func LoadHelp(textDir string, font *xlate.Font) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(textDir, "host-ui.zh-TW.tsv"))
	if err != nil {
		return nil, fmt.Errorf("buckrogers: 讀取 host-ui.zh-TW.tsv：%w", err)
	}
	rows, err := readTSV("host-ui.zh-TW.tsv", data, []string{"key", "translation", "source"})
	if err != nil {
		return nil, err
	}
	type line struct{ key, text string }
	var lines []line
	for _, r := range rows {
		if !strings.HasPrefix(r[0], "help.") {
			continue
		}
		if r[2] != "frontend-help" {
			return nil, fmt.Errorf("host-ui.zh-TW.tsv: %s 的 source 須為 frontend-help", r[0])
		}
		if n := len([]rune(r[1])); n > helpColumns {
			return nil, fmt.Errorf("host-ui.zh-TW.tsv: %s 超過 %d 格（%d）", r[0], helpColumns, n)
		}
		for _, ch := range r[1] {
			if _, ok := font.Glyphs[ch]; !ok && ch != ' ' {
				return nil, fmt.Errorf("host-ui.zh-TW.tsv: %s 缺字模 %q", r[0], ch)
			}
		}
		lines = append(lines, line{r[0], r[1]})
	}
	if len(lines) == 0 || len(lines) > helpMaxRows {
		return nil, fmt.Errorf("host-ui.zh-TW.tsv: 說明頁須有 1–%d 行，得 %d", helpMaxRows, len(lines))
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].key < lines[j].key })
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.text
	}
	return out, nil
}

// DrawHelp 把說明頁畫在已合成的 RGBA（320×scale × 200×scale）上：
// 先把整個畫面壓暗，再逐行畫字。
func DrawHelp(rgba []byte, scale int, font *xlate.Font, lines []string) {
	for i := 0; i+3 < len(rgba); i += 4 {
		rgba[i], rgba[i+1], rgba[i+2] = rgba[i]/5, rgba[i+1]/5, rgba[i+2]/5
	}
	off := (8*scale - font.W) / 2
	if off < 0 {
		off = 0
	}
	layer := &xlate.Layer{W: 320, H: 200}
	for i, l := range lines {
		text := []rune(l)
		layer.Add(&xlate.Stamp{
			Key: fmt.Sprintf("help.%d", i), X: helpX, Y: helpY + i*helpPitch,
			Cells: len(text), CellW: 8, CellH: 8, Font: font, GlyphX: off, GlyphY: off,
			GlyphScale: 1, Text: text, State: xlate.Shown,
			FG: [3]uint8{0xF0, 0xE8, 0x90}, BG: [3]uint8{0x10, 0x10, 0x18},
		})
	}
	layer.Draw(rgba, scale, nil)
}

// HelpLines 回說明頁的行；Font 回覆繪用的字型。
func (r *LiveRuntime) HelpLines() []string { return r.help }
func (r *LiveRuntime) Font() *xlate.Font   { return r.font }
