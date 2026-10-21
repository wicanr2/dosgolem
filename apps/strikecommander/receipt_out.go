package strikecommander

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// 收據的寫檔（strike_commander 規格 006 §3.3 第 2 條）：離線工具可由這些檔重算 Match 與期望圖，不需讀產品程式。
//
//   - <前綴>-frame.idx：出圖用的原版色號畫格（320×200 bytes）；
//   - <前綴>-palette.bin：色盤（256 × RGB，768 bytes）；
//   - <前綴>-s<倍率>-labels.tsv：該倍率 BGReport 的每筆一列（第一列是欄名），欄位見 bgTSVHeader；
//   - <前綴>-s<倍率>-rows.bin：依 TSV 的位移與長度串接前列、後列、Ref；nil 的列位移與長度記 0；
//   - <前綴>-composite-<倍率>x.png、<前綴>-plain-<倍率>x.png：合成圖與原版放大圖。

// bgTSVHeader 是 labels.tsv 的欄名。座標是原版像素；state 是「透明」或「退路」；hidden 是該倍率這次 Draw 是否不畫。
var bgTSVHeader = []string{
	"key", "kind", "state", "text",
	"cover_x", "cover_y", "cover_w", "cover_h",
	"text_x", "text_y", "cells", "cell_w", "cell_h", "glyph_x", "glyph_y",
	"hidden",
	"before_off", "before_len", "after_off", "after_len", "ref_off", "ref_len",
	// strike_commander 規格 011 §3.8：(d) 到 (f) 在現行欄之後新增；新路徑的列舊幾何欄填 -，舊路徑的列新欄填 -。
	"display", "px", "tx_out", "baseline", "clip_x0", "clip_y0", "clip_x1", "clip_y1",
	"ink_x0", "ink_y0", "ink_x1", "ink_y1", "outline", "e0", "e1", "path",
}

// BGTSV 把一份 BGReport 編成 labels.tsv 與 rows.bin 的內容。
func BGTSV(entries []BGEntry) (tsv, rows []byte) {
	var t strings.Builder
	var r bytes.Buffer
	t.WriteString(strings.Join(bgTSVHeader, "\t") + "\n")
	put := func(b []byte) (int, int) {
		if b == nil {
			return 0, 0
		}
		off := r.Len()
		r.Write(b)
		return off, len(b)
	}
	for _, e := range entries {
		bo, bl := put(e.Before)
		ao, al := put(e.After)
		ro, rl := put(e.Ref)
		geom := fmt.Sprintf("%d\t%d\t%d\t%d\t%d\t%d\t%d", e.TextX, e.TextY, e.Cells, e.CellW, e.CellH, e.GlyphX, e.GlyphY)
		extra := strings.Repeat("-\t", 15) + "old"
		if e.Path == "new" {
			geom = strings.TrimSuffix(strings.Repeat("-\t", 7), "\t")
			ink := e.Ink
			if ink.Empty() {
				ink.X0, ink.Y0, ink.X1, ink.Y1 = 0, 0, 0, 0
			}
			extra = fmt.Sprintf("%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%t\t%d\t%d\tnew", e.Display, e.Px, e.TxOut, e.Baseline,
				e.Clip.X0, e.Clip.Y0, e.Clip.X1, e.Clip.Y1, ink.X0, ink.Y0, ink.X1, ink.Y1, e.Outline, e.E0, e.E1)
		}
		fmt.Fprintf(&t, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%s\t%t\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n",
			e.Key, e.Kind, e.State, e.Text,
			e.CoverX, e.CoverY, e.CoverW, e.CoverH,
			geom,
			e.Hidden, bo, bl, ao, al, ro, rl, extra)
	}
	return []byte(t.String()), r.Bytes()
}

// WriteReceiptFiles 在 dir 寫一個 shot 或停止點的全部收據檔（006 §3.3 第 2 條）。scales 決定寫哪些倍率；
// bg 某倍率缺時仍寫只有欄名的 TSV 與空的 rows.bin。
func WriteReceiptFiles(dir, prefix string, indexed []byte, palette [256][3]uint8, scales []int,
	composites, plain map[int][]byte, bg map[int][]BGEntry) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, prefix+"-frame.idx"), indexed, 0o644); err != nil {
		return err
	}
	pal := make([]byte, 0, 768)
	for _, c := range palette {
		pal = append(pal, c[0], c[1], c[2])
	}
	if err := os.WriteFile(filepath.Join(dir, prefix+"-palette.bin"), pal, 0o644); err != nil {
		return err
	}
	for _, sc := range scales {
		tsv, rows := BGTSV(bg[sc])
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-s%d-labels.tsv", prefix, sc)), tsv, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-s%d-rows.bin", prefix, sc)), rows, 0o644); err != nil {
			return err
		}
		if err := WriteRGBAPNG(filepath.Join(dir, fmt.Sprintf("%s-composite-%dx.png", prefix, sc)), composites[sc], sc); err != nil {
			return err
		}
		if err := WriteRGBAPNG(filepath.Join(dir, fmt.Sprintf("%s-plain-%dx.png", prefix, sc)), plain[sc], sc); err != nil {
			return err
		}
	}
	return nil
}

// ShotPrefix 回 shot 的前綴 shot-<步數>-<名稱>；沒有名稱時是 shot-<步數>（同 cmd/strikecommander-label-receipt）。
func ShotPrefix(s *ShotResult) string {
	if s.Name == "" {
		return fmt.Sprintf("shot-%d", s.Steps)
	}
	return fmt.Sprintf("shot-%d-%s", s.Steps, s.Name)
}

// WriteShotFiles 以 ShotResult 呼叫 WriteReceiptFiles。
func WriteShotFiles(dir, prefix string, s *ShotResult, scales []int) error {
	if err := WriteReceiptFiles(dir, prefix, s.Indexed, s.Palette, scales, s.Composites, s.Plain, s.BG); err != nil {
		return err
	}
	return writeTextExtras(dir, prefix, scales, s.Arts, s.SubtitleRegion)
}

// WriteStopFiles 以 ReceiptResult 的停止點呼叫 WriteReceiptFiles，前綴 stop-<步數>。
func WriteStopFiles(dir string, r *ReceiptResult, scales []int) error {
	prefix := fmt.Sprintf("stop-%d", r.Digest.Steps)
	if err := WriteReceiptFiles(dir, prefix, r.Frame, r.Palette, scales, r.Composites, r.Plain, r.BG); err != nil {
		return err
	}
	return writeTextExtras(dir, prefix, scales, r.StopArts, r.StopSubtitleRegion)
}

// writeTextExtras 寫 strike_commander 規格 011 §3.8 的新檔：圖面新路徑開時的 arts.tsv；字幕新路徑開時的 subregion.tsv
// （欄 scale diffs outside ink_pixels visible）。對應路徑沒開時不寫（舊路徑的收據檔不變）。
func writeTextExtras(dir, prefix string, scales []int, arts map[int][]ArtReportEntry, region map[int]SubtitleRegionCheck) error {
	if err := writeArtsTSV(dir, prefix, scales, arts); err != nil {
		return err
	}
	if region == nil {
		return nil
	}
	var b strings.Builder
	b.WriteString("scale\tdiffs\toutside\tink_pixels\tvisible\n")
	for _, sc := range scales {
		c := region[sc]
		fmt.Fprintf(&b, "%d\t%d\t%d\t%d\t%t\n", sc, c.Diffs, c.Outside, c.InkPixels, c.Visible)
	}
	return os.WriteFile(filepath.Join(dir, prefix+"-subregion.tsv"), []byte(b.String()), 0o644)
}

// WriteRGBAPNG 把 (320 × scale) × (200 × scale) 的 RGBA 寫成 PNG。
func WriteRGBAPNG(path string, rgba []byte, scale int) error {
	img := &image.RGBA{Pix: rgba, Stride: 320 * scale * 4, Rect: image.Rect(0, 0, 320*scale, 200*scale)}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
