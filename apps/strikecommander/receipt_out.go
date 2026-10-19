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
		fmt.Fprintf(&t, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%t\t%d\t%d\t%d\t%d\t%d\t%d\n",
			e.Key, e.Kind, e.State, e.Text,
			e.CoverX, e.CoverY, e.CoverW, e.CoverH,
			e.TextX, e.TextY, e.Cells, e.CellW, e.CellH, e.GlyphX, e.GlyphY,
			e.Hidden, bo, bl, ao, al, ro, rl)
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
	return WriteReceiptFiles(dir, prefix, s.Indexed, s.Palette, scales, s.Composites, s.Plain, s.BG)
}

// WriteStopFiles 以 ReceiptResult 的停止點呼叫 WriteReceiptFiles，前綴 stop-<步數>。
func WriteStopFiles(dir string, r *ReceiptResult, scales []int) error {
	return WriteReceiptFiles(dir, fmt.Sprintf("stop-%d", r.Digest.Steps), r.Frame, r.Palette, scales, r.Composites, r.Plain, r.BG)
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
