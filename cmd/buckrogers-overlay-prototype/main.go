// Command buckrogers-overlay-prototype produces offline 2x/3x Traditional Chinese menu A/B receipts.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/buckrogers"
	"github.com/wicanr2/dosgolem/xlate"
)

type menuEvent struct {
	TextKey    string
	Background uint8
	Foreground uint8
}

type safeRect struct {
	X, Y, Width, Height int
	DrawX, DrawY        int
	Capacity            int
	LineCount           int
	Overflow            string
}

type rectJSON struct {
	X, Y, Width, Height int
}

type eventJSON struct {
	EventKey         string   `json:"event_key"`
	TextKey          string   `json:"text_key"`
	TranslationRunes int      `json:"translation_runes"`
	Background       uint8    `json:"background"`
	Foreground       uint8    `json:"foreground"`
	ClearRect        rectJSON `json:"clear_rect"`
	DrawAnchorX      int      `json:"draw_anchor_x"`
	DrawAnchorY      int      `json:"draw_anchor_y"`
	InkRect          rectJSON `json:"ink_rect"`
	Contained        bool     `json:"contained"`
}

type receiptJSON struct {
	Tool                 string            `json:"tool"`
	Screen               string            `json:"screen"`
	Scale                int               `json:"scale"`
	CanvasWidth          int               `json:"canvas_width"`
	CanvasHeight         int               `json:"canvas_height"`
	Inputs               map[string]string `json:"inputs_sha256"`
	Events               []eventJSON       `json:"events"`
	MissingGlyphs        []string          `json:"missing_glyphs"`
	DiffOutsideSafeRects int               `json:"diff_outside_safe_rects"`
	OverlappingRects     int               `json:"overlapping_rects"`
	BasePNG              string            `json:"base_png_sha256"`
	OutputPNG            string            `json:"output_png_sha256"`
}

func main() {
	indexedPath := flag.String("indexed", "", "320x200 indexed framebuffer")
	palettePath := flag.String("palette", "", "256x3 VGA DAC bytes")
	fontPath := flag.String("font", "", "GOLEMFNT font")
	eventsPath := flag.String("events", "", "menu-events.tsv")
	rectsPath := flag.String("rects", "", "menu-text-safe-rects.tsv")
	translationsPath := flag.String("translations", "", "menu.zh-TW.tsv")
	screen := flag.String("screen", "", "steady 或 down")
	scale := flag.Int("scale", 0, "2 或 3")
	outPath := flag.String("out", "", "輸出 PNG")
	baseOutPath := flag.String("base-out", "", "輸出未覆繪 PNG")
	receiptPath := flag.String("receipt", "", "輸出 JSON")
	flag.Parse()
	if *indexedPath == "" || *palettePath == "" || *fontPath == "" || *eventsPath == "" ||
		*rectsPath == "" || *translationsPath == "" || *outPath == "" || *baseOutPath == "" || *receiptPath == "" ||
		(*screen != "steady" && *screen != "down") || (*scale != 2 && *scale != 3) {
		fail(fmt.Errorf("所有路徑必填；screen=steady|down，scale=2|3"))
	}

	indexed := mustRead(*indexedPath)
	paletteRaw := mustRead(*palettePath)
	if len(indexed) != 320*200 || len(paletteRaw) != 256*3 {
		fail(fmt.Errorf("indexed=%d（要 64000），palette=%d（要 768）", len(indexed), len(paletteRaw)))
	}
	font, err := xlate.LoadFont(*fontPath)
	if err != nil {
		fail(err)
	}
	events := loadEvents(*eventsPath)
	rects := loadRects(*rectsPath)
	translations := loadTranslations(*translationsPath)
	keys := screenEvents(*screen)
	palette := decodePalette(paletteRaw)
	base := scaleIndexed(indexed, palette, *scale)
	rendered := append([]byte(nil), base...)
	overlayEntries := make([]buckrogers.MenuOverlayEntry, 0, len(keys))

	for _, key := range keys {
		event, ok := events[key]
		if !ok {
			fail(fmt.Errorf("事件不存在：%s", key))
		}
		r, ok := rects[key]
		if !ok {
			fail(fmt.Errorf("安全矩形不存在：%s", key))
		}
		translation, ok := translations[event.TextKey]
		if !ok {
			fail(fmt.Errorf("譯文不存在：%s", event.TextKey))
		}
		overlayEntries = append(overlayEntries, buckrogers.MenuOverlayEntry{
			EventKey: key, TextKey: event.TextKey, Translation: translation,
			Background: event.Background, Foreground: event.Foreground,
			X: r.X, Y: r.Y, Width: r.Width, Height: r.Height,
			DrawX: r.DrawX, DrawY: r.DrawY, Capacity: r.Capacity,
			LineCount: r.LineCount, Overflow: r.Overflow,
		})
	}
	overlay, err := buckrogers.BuildMenuOverlay(overlayEntries, font, palette, *scale)
	if err != nil {
		fail(err)
	}
	missingSet := map[rune]bool{}
	if !overlay.Layer.Draw(rendered, *scale, func(r rune) { missingSet[r] = true }) {
		fail(fmt.Errorf("xlate.Draw 沒有畫出任何 stamp"))
	}
	resultEvents := make([]eventJSON, 0, len(overlay.Events))
	for _, event := range overlay.Events {
		resultEvents = append(resultEvents, eventJSON{
			EventKey: event.EventKey, TextKey: event.TextKey, TranslationRunes: event.TranslationRunes,
			Background: event.Background, Foreground: event.Foreground,
			ClearRect: fromPixelRect(event.ClearRect), DrawAnchorX: event.DrawAnchorX,
			DrawAnchorY: event.DrawAnchorY, InkRect: fromPixelRect(event.InkRect), Contained: event.Contained,
		})
	}
	missing := make([]string, 0, len(missingSet))
	for r := range missingSet {
		missing = append(missing, string(r))
	}
	if len(missing) != 0 {
		fail(fmt.Errorf("GOLEMFNT 缺字：%s", strings.Join(missing, "")))
	}
	overlap := overlapping(resultEvents)
	outside := diffOutside(base, rendered, resultEvents, 320**scale, 200**scale)
	for _, e := range resultEvents {
		if !e.Contained {
			fail(fmt.Errorf("%s 字模超出安全矩形", e.EventKey))
		}
	}
	if overlap != 0 || outside != 0 {
		fail(fmt.Errorf("幾何驗證失敗：overlap=%d outside=%d", overlap, outside))
	}
	pngBytes := encodePNG(rendered, 320**scale, 200**scale)
	basePNGBytes := encodePNG(base, 320**scale, 200**scale)
	if err := os.WriteFile(*outPath, pngBytes, 0o644); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*baseOutPath, basePNGBytes, 0o644); err != nil {
		fail(err)
	}
	receipt := receiptJSON{
		Tool: "dosgolem/cmd/buckrogers-overlay-prototype", Screen: *screen, Scale: *scale,
		CanvasWidth: 320 * *scale, CanvasHeight: 200 * *scale,
		Inputs: map[string]string{
			"indexed": hash(indexed), "palette": hash(paletteRaw), "font": hash(mustRead(*fontPath)),
			"events": hash(mustRead(*eventsPath)), "rects": hash(mustRead(*rectsPath)),
			"translations": hash(mustRead(*translationsPath)),
		},
		Events: resultEvents, MissingGlyphs: missing, DiffOutsideSafeRects: outside,
		OverlappingRects: overlap, BasePNG: hash(basePNGBytes), OutputPNG: hash(pngBytes),
	}
	b, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		fail(err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(*receiptPath, b, 0o644); err != nil {
		fail(err)
	}
}

func screenEvents(screen string) []string {
	if screen == "steady" {
		return []string{"race.screen.prompt", "race.heading.terran", "race.option.martian", "race.option.venusian", "race.option.mercurian", "race.option.tinker", "race.option.desert_runner"}
	}
	return []string{"race.screen.prompt", "race.selection.normal.terran", "race.selection.selected.martian", "race.option.venusian", "race.option.mercurian", "race.option.tinker", "race.option.desert_runner"}
}

func loadEvents(path string) map[string]menuEvent {
	rows := readTSV(path)
	requireHeader(rows[0], []string{"event_key", "sequence", "text_key", "original_length", "original_sha256", "caller", "background", "foreground", "row", "column"})
	out := map[string]menuEvent{}
	for _, row := range rows[1:] {
		out[row[0]] = menuEvent{TextKey: row[2], Background: byteInt(row[6]), Foreground: byteInt(row[7])}
	}
	return out
}

func loadRects(path string) map[string]safeRect {
	rows := readTSV(path)
	requireHeader(rows[0], []string{"event_key", "x", "y", "width", "height", "draw_x", "draw_y", "capacity_cells", "line_count", "overflow_policy"})
	out := map[string]safeRect{}
	for _, row := range rows[1:] {
		out[row[0]] = safeRect{intField(row[1]), intField(row[2]), intField(row[3]), intField(row[4]), intField(row[5]), intField(row[6]), intField(row[7]), intField(row[8]), row[9]}
	}
	return out
}

func loadTranslations(path string) map[string]string {
	rows := readTSV(path)
	requireHeader(rows[0], []string{"key", "translation", "source"})
	out := map[string]string{}
	for _, row := range rows[1:] {
		out[row[0]] = row[1]
	}
	return out
}

func readTSV(path string) [][]string {
	r := csv.NewReader(bytes.NewReader(mustRead(path)))
	r.Comma, r.FieldsPerRecord = '\t', -1
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		fail(fmt.Errorf("讀 TSV %s：%v", path, err))
	}
	return rows
}

func requireHeader(got, want []string) {
	if strings.Join(got, "\t") != strings.Join(want, "\t") {
		fail(fmt.Errorf("TSV header 不符：%q", strings.Join(got, "\t")))
	}
}

func intField(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		fail(err)
	}
	return n
}

func byteInt(s string) uint8 {
	n := intField(s)
	if n < 0 || n > 255 {
		fail(fmt.Errorf("色號超界：%d", n))
	}
	return uint8(n)
}

func decodePalette(raw []byte) [256][3]uint8 {
	var out [256][3]uint8
	for i := 0; i < 256; i++ {
		for c := 0; c < 3; c++ {
			// cmd/probe 的 .pal 已經是 machine.Palette() 輸出的 8-bit RGB，不能再次做 6→8 轉換。
			out[i][c] = raw[i*3+c]
		}
	}
	return out
}

func scaleIndexed(indexed []byte, pal [256][3]uint8, scale int) []byte {
	w := 320 * scale
	out := make([]byte, w*200*scale*4)
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			c := pal[indexed[y*320+x]]
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					i := ((y*scale+dy)*w + x*scale + dx) * 4
					out[i], out[i+1], out[i+2], out[i+3] = c[0], c[1], c[2], 255
				}
			}
		}
	}
	return out
}

func fromPixelRect(r buckrogers.PixelRect) rectJSON {
	return rectJSON{r.X, r.Y, r.Width, r.Height}
}

func overlapping(events []eventJSON) int {
	n := 0
	for i := range events {
		for j := i + 1; j < len(events); j++ {
			a, b := events[i].ClearRect, events[j].ClearRect
			if a.X < b.X+b.Width && b.X < a.X+a.Width && a.Y < b.Y+b.Height && b.Y < a.Y+a.Height {
				n++
			}
		}
	}
	return n
}

func diffOutside(base, rendered []byte, events []eventJSON, w, h int) int {
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			inside := false
			for _, e := range events {
				r := e.ClearRect
				if x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height {
					inside = true
					break
				}
			}
			i := (y*w + x) * 4
			if !inside && !bytes.Equal(base[i:i+4], rendered[i:i+4]) {
				n++
			}
		}
	}
	return n
}

func encodePNG(rgba []byte, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, rgba)
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&b, img); err != nil {
		fail(err)
	}
	return b.Bytes()
}

func hash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	return b
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
