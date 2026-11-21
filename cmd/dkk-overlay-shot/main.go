// Command dkk-overlay-shot 是《Death Knights of Krynn》路徑 A 覆繪的 headless A/B 收據
// （death-knights-of-krynn-cht 規格 001 §3）。
//
// 從 probe 狀態檔接著跑，照指令數送 BIOS 鍵；每道指令前交給 apps/dkk 的攔截點，
// 每個畫格送進 xlate 定色。在 -shots 指定的步數各輸出原版與覆繪的 2× PNG，
// 最後印 JSON：每張圖的 SHA-256、差異是否落在 Stamp 矩形內、每次印字呼叫的命中紀錄
// （只有長度與 key，不含原文）。
//
//	tools/go.sh run ./cmd/dkk-overlay-shot -state /st/start-200m.state -root /orig \
//	  -scratch /out/scratch -font /font/dkk-eten.golemfnt \
//	  -events /text/ui-text-events.tsv -translations /text/ui-text.zh-TW.tsv \
//	  -bios-keys ' p' -bios-key-from 210000000 -bios-key-every 50000000 \
//	  -shots 399000000:menu -until 400000000 -out /out
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/dkk"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
	"github.com/wicanr2/dosgolem/xlate"
)

var scale = 2 // 輸出倍率，-scale 設定（2 或 3）

type timedKey struct {
	at  uint64
	key dos.Key
}

type shot struct {
	step uint64
	name string
}

type shotResult struct {
	Name          string `json:"name"`
	Step          uint64 `json:"step"`
	IndexedSHA256 string `json:"indexed_sha256"` // 原版色號陣列，與 probe -dump-at 的 .bin 同一份資料
	BaseSHA256    string `json:"base_sha256"`
	OverlaySHA256 string `json:"overlay_sha256"`
	DiffPixels    int    `json:"diff_pixels"`
	DiffOutside   int    `json:"diff_outside_stamps"`
	Stamps        int    `json:"stamps"`
	StampsShown   int    `json:"stamps_shown"`
	Missing       string `json:"missing_glyphs,omitempty"`
	Journal       bool   `json:"journal_panel,omitempty"`
	StampList     []string `json:"stamp_list,omitempty"` // 每筆疊字：key、位置、狀態、透明格數、文字（查殘字用）
}

func main() {
	statePath := flag.String("state", "", "probe 狀態檔")
	root := flag.String("root", "", "原版素材目錄（唯讀）")
	scratch := flag.String("scratch", "", "可寫暫存層（存檔落這裡）")
	fontPath := flag.String("font", "", "16×16 GOLEMFNT")
	events := flag.String("events", "", "ui-text-events.tsv")
	translations := flag.String("translations", "", "ui-text.zh-TW.tsv")
	eclEvents := flag.String("ecl-events", "", "ecl-text-events.tsv（給了才啟用路徑 B）")
	eclTranslations := flag.String("ecl-translations", "", "ecl-text.zh-TW.tsv")
	textDir := flag.String("text-dir", "", "譯文目錄：給了就以 dkk.LoadTextDir 載入全部（含規格 004 組句），忽略 events／translations／ecl-* 旗標")
	biosKeys := flag.String("bios-keys", "", "逐個送進 BIOS 鍵盤緩衝區的字元")
	biosKeyNames := flag.String("bios-key-names", "", "接在 -bios-keys 後送的具名鍵，逗號分隔")
	keyFrom := flag.Uint64("bios-key-from", 0, "第一個鍵的步數")
	keyEvery := flag.Uint64("bios-key-every", 50_000_000, "兩鍵間隔步數")
	shotsSpec := flag.String("shots", "", "步數:名稱，逗號分隔")
	keysAt := flag.String("keys-at", "", "依步數送鍵 `步數:鍵名`，逗號分隔（重播 dkk-explore 的送鍵紀錄；與 -bios-keys 併用時各自排程）")
	until := flag.Uint64("until", 0, "絕對步數上限")
	outDir := flag.String("out", "", "輸出目錄")
	hideAnswers := flag.Bool("hide-answers", false, "隱藏查驗題答案（規格 006）")
	journalPreview := flag.Int("journal-preview", 0, "以翻閱模式開啟這一則手札（規格 007 版面檢查；需 -text-dir）")
	guidePreview := flag.String("guide-preview", "", "以 `章:頁` 開啟攻略面板（規格 008 版面檢查；0 章是目錄；需 -text-dir）")
	mapPreview := flag.String("map-preview", "", "開地圖疊層（規格 009）：`走過` 只畫走過的格子、`全圖` 全畫（需 -text-dir）")
	noOverlay := flag.Bool("no-overlay", false, "不掛攔截點（同一套送鍵跑出原版畫面，比對色號雜湊用）")
	invPath := flag.String("inventory", "", "印字路徑盤點（issue #11）寫到這個 TSV；含原文，只放工作區")
	hdDir := flag.String("hd", "", "HD 主題目錄（theme.json；主專案規格 010，只在 3× 顯示）")
	flag.IntVar(&scale, "scale", 2, "輸出倍率 2 或 3（3× 用 22×22 變體字型，xlate spec 202 §2.4.1）")
	flag.Parse()
	if scale != 2 && scale != 3 {
		fmt.Fprintln(os.Stderr, "dkk-overlay-shot: -scale 須為 2 或 3")
		os.Exit(2)
	}
	if *statePath == "" || *root == "" || *fontPath == "" || *until == 0 || *outDir == "" {
		fail(fmt.Errorf("state、root、font、until、out 均為必填"))
	}
	if *textDir == "" && (*events == "" || *translations == "") {
		fail(fmt.Errorf("沒給 text-dir 時 events、translations 必填"))
	}
	shots, err := parseShots(*shotsSpec)
	if err != nil {
		fail(err)
	}
	keys, err := parseKeys(*biosKeys, *biosKeyNames)
	if err != nil {
		fail(err)
	}
	var timed []timedKey
	if strings.TrimSpace(*keysAt) != "" {
		for _, it := range strings.Split(*keysAt, ",") {
			at, name, ok := strings.Cut(strings.TrimSpace(it), ":")
			n, err := strconv.ParseUint(at, 10, 64)
			if !ok || err != nil {
				fail(fmt.Errorf("-keys-at %q 不是 步數:鍵名", it))
			}
			k, err := parseKeys("", name)
			if err != nil {
				fail(err)
			}
			timed = append(timed, timedKey{n, k[0]})
		}
	}
	font, err := xlate.LoadFont(*fontPath)
	if err != nil {
		fail(err)
	}
	font.Name = filepath.Base(*fontPath)

	m := machine.New()
	d := dos.New(m, *root)
	d.Install()
	d.Scratch = *scratch
	if err := state.Load(*statePath, m, d); err != nil {
		fail(err)
	}
	var ui dkk.Lookuper
	var ecl *dkk.Catalog
	entries := 0
	if *textDir != "" {
		t, e, err := dkk.LoadTextDir(*textDir)
		if err != nil {
			fail(err)
		}
		ui, ecl = t, e
		for _, c := range t.Exact {
			if c, ok := c.(*dkk.Catalog); ok {
				entries += c.Len()
			}
		}
	} else {
		cat, err := dkk.LoadCatalog(mustRead(*events), mustRead(*translations))
		if err != nil {
			fail(err)
		}
		ui, entries = cat, cat.Len()
		if *eclEvents != "" {
			if ecl, err = dkk.LoadCatalog(mustRead(*eclEvents), mustRead(*eclTranslations)); err != nil {
				fail(err)
			}
		}
	}
	live := dkk.NewLive(font, ui, ecl)
	if *hdDir != "" {
		if err := live.AddHD(*hdDir); err != nil {
			fail(err)
		}
	}
	if *textDir != "" {
		if live.Answers, live.QuizFormat, err = dkk.LoadQuizDir(*textDir); err != nil {
			fail(err)
		}
	}
	live.SetHideAnswers(*hideAnswers)
	if *textDir != "" {
		if live.Journal, err = dkk.LoadJournalDir(*textDir); err != nil {
			fail(err)
		}
		if *journalPreview != 0 {
			live.Journal.Seen[*journalPreview] = true
			live.Journal.ToggleBrowse()
		}
	}
	if *textDir != "" && *guidePreview != "" {
		if live.Guide, err = dkk.LoadGuideDir(*textDir); err != nil || live.Guide == nil {
			fail(fmt.Errorf("攻略面板：%v（缺 walkthrough.zh-TW.tsv？）", err))
		}
		var ch, page int
		if _, err := fmt.Sscanf(*guidePreview, "%d:%d", &ch, &page); err != nil {
			fail(fmt.Errorf("-guide-preview %q 要寫 章:頁", *guidePreview))
		}
		live.Guide.SetPos(ch, page)
		live.Guide.Toggle()
	}
	if *textDir != "" && *mapPreview != "" {
		if live.Map, err = dkk.LoadAutoMapDir(*textDir); err != nil || live.Map == nil {
			fail(fmt.Errorf("地圖疊層：%v（缺 map-geo.tsv？）", err))
		}
		live.Map.Toggle()
		if *mapPreview == "全圖" {
			live.Map.ToggleReveal()
		}
	}
	live.Inventory = *invPath != ""
	live.Arm() // 狀態檔已在遊戲中
	cpuView := machineCPU{m}
	m.ObserveVideoWrites(func(w machine.VideoWrite) { live.VideoWrite(w.Offset) })
	m.ObserveModeChanges(func(machine.ModeChange) { live.ModeChange() })
	m.SetOnFrame(func() { live.Frame(m.Indexed(), m.FramePalette()) }) // planar 以 FramePalette 取色（#12）

	var results []shotResult
	ki, si := 0, 0
	for m.Steps < *until && !d.Exited {
		if ki < len(keys) && m.Steps >= *keyFrom+uint64(ki)*(*keyEvery) {
			if _, pending := m.PeekKey(); !pending {
				d.PushKey(keys[ki])
				ki++
			}
		}
		if len(timed) > 0 && m.Steps >= timed[0].at {
			if _, pending := m.PeekKey(); !pending {
				d.PushKey(timed[0].key)
				timed = timed[1:]
				ki++
			}
		}
		if si < len(shots) && m.Steps >= shots[si].step {
			results = append(results, capture(m, live, shots[si], *outDir))
			si++
		}
		if !*noOverlay {
			if err := live.BeforeStep(cpuView); err != nil {
				fail(err)
			}
		}
		if err := m.Step(); err != nil {
			fail(err)
		}
	}
	if live.Inv != nil {
		ovr, _ := os.ReadFile(filepath.Join(*root, "GAME.OVR"))
		if err := live.Inv.WriteTSV(*invPath, ovr); err != nil {
			fail(err)
		}
	}
	out := struct {
		Entry   string         `json:"path_a_entry"`
		Catalog int            `json:"catalog_entries"`
		KeysOut int            `json:"keys_sent"`
		Stopped uint64         `json:"stopped_at"`
		Shots   []shotResult   `json:"shots"`
		Hits    []dkk.Hit      `json:"hits"`
		HitsB   []dkk.Hit      `json:"hits_b,omitempty"`
		MenuC   []dkk.MenuWord `json:"menu_words"`
	}{Catalog: entries, KeysOut: ki, Stopped: m.Steps, Shots: results}
	if live.A != nil { // -no-overlay 時沒有綁定
		out.Entry, out.Hits, out.MenuC = fmt.Sprintf("%05X", live.A.Entry), live.A.Hits, live.C.Words
	}
	if live.B != nil {
		out.HitsB = live.B.Hits
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(out); err != nil {
		fail(err)
	}
}

func capture(m *machine.Machine, live *dkk.Live, s shot, dir string) shotResult {
	// 截圖用此刻的畫面（不是最後一個 retrace），與 probe -dump-at 同一份資料。
	idx := m.Indexed()
	if len(idx) != 320*200 {
		fail(fmt.Errorf("#%d 畫面不是 320×200（%d 點）", m.Steps, len(idx)))
	}
	layer := live.Layer
	pal := m.FramePalette()
	base := upscale(idx, pal)
	over := append([]uint8(nil), base...)
	var missing []rune
	layer.Draw(over, scale, func(r rune) { missing = append(missing, r) })
	journal := live.Journal.Open()
	if journal {
		live.Journal.Draw(over, scale, live.Font, func(r rune) { missing = append(missing, r) })
	}
	guide := live.Guide.Open()
	if guide {
		live.Guide.Draw(over, scale, live.Font, func(r rune) { missing = append(missing, r) })
	}
	if live.Map.Open() {
		guide = true // 面板範圍同攻略面板（[0,320)×[0,192)）
		live.Map.Draw(over, scale, live.Font, func(r rune) { missing = append(missing, r) })
	}
	isum := sha256.Sum256(idx)
	r := shotResult{Name: s.name, Step: m.Steps, IndexedSHA256: hex.EncodeToString(isum[:]), Stamps: len(layer.Stamps)}
	for _, st := range layer.Stamps {
		if st.State == xlate.Shown {
			r.StampsShown++
		}
		nt := 0
		for _, t := range st.Transparent {
			if t {
				nt++
			}
		}
		r.StampList = append(r.StampList, fmt.Sprintf("%s @%d,%d cells=%d state=%d transparent=%d %q", st.Key, st.X, st.Y, st.Cells, st.State, nt, string(st.Text)))
	}
	for i := 0; i < len(base); i += 4 {
		if base[i] == over[i] && base[i+1] == over[i+1] && base[i+2] == over[i+2] {
			continue
		}
		r.DiffPixels++
		x, y := (i/4)%(320*scale)/scale, (i/4)/(320*scale)/scale
		inside := journal && dkk.InJournalPanel(x, y) || guide && dkk.InGuidePanel(x, y)
		for _, st := range layer.Stamps {
			x0, y0, x1, y1 := st.Rect()
			if x >= x0 && x < x1 && y >= y0 && y < y1 {
				inside = true
				break
			}
		}
		if !inside {
			r.DiffOutside++
		}
	}
	r.BaseSHA256 = writePNG(filepath.Join(dir, s.name+"-base.png"), base)
	r.OverlaySHA256 = writePNG(filepath.Join(dir, s.name+"-overlay.png"), over)
	r.Missing = string(missing)
	r.Journal = journal
	return r
}

// machineCPU 把 machine.Machine 包成 dkk.StepCPU。
type machineCPU struct{ m *machine.Machine }

func (c machineCPU) Read8(a uint32) uint8   { return c.m.Peek8(a) }
func (c machineCPU) Read16(a uint32) uint16 { return c.m.Peek16(a) }
func (c machineCPU) Steps() uint64          { return c.m.Steps }
func (c machineCPU) CS() uint16             { return c.m.CPU.Seg[cpu.CS] }
func (c machineCPU) IP() uint16             { return c.m.CPU.IP }
func (c machineCPU) SS() uint16             { return c.m.CPU.Seg[cpu.SS] }
func (c machineCPU) SP() uint16             { return c.m.CPU.R[cpu.SP] }
func (c machineCPU) DS() uint16             { return c.m.CPU.Seg[cpu.DS] }

func rgbOf(idx []uint8, pal [256][3]uint8) []uint8 {
	out := make([]uint8, len(idx)*3)
	for i, c := range idx {
		out[i*3], out[i*3+1], out[i*3+2] = pal[c][0], pal[c][1], pal[c][2]
	}
	return out
}

func upscale(idx []uint8, pal [256][3]uint8) []uint8 {
	w := 320 * scale
	out := make([]uint8, w*200*scale*4)
	for y := 0; y < 200*scale; y++ {
		for x := 0; x < w; x++ {
			c := pal[idx[(y/scale)*320+x/scale]]
			o := (y*w + x) * 4
			out[o], out[o+1], out[o+2], out[o+3] = c[0], c[1], c[2], 0xFF
		}
	}
	return out
}

func writePNG(path string, rgba []uint8) string {
	img := &image.RGBA{Pix: rgba, Stride: 320 * scale * 4, Rect: image.Rect(0, 0, 320*scale, 200*scale)}
	f, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	if err := png.Encode(f, img); err != nil {
		fail(err)
	}
	if err := f.Close(); err != nil {
		fail(err)
	}
	sum := sha256.Sum256(rgba)
	return hex.EncodeToString(sum[:])
}

func parseShots(spec string) ([]shot, error) {
	var out []shot
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	for _, part := range strings.Split(spec, ",") {
		i := strings.Index(part, ":")
		if i < 0 {
			return nil, fmt.Errorf("-shots %q 不是 步數:名稱", part)
		}
		n, err := strconv.ParseUint(part[:i], 10, 64)
		if err != nil || part[i+1:] == "" || strings.ContainsAny(part[i+1:], "/\\") {
			return nil, fmt.Errorf("-shots %q 不合法", part)
		}
		if len(out) > 0 && n <= out[len(out)-1].step {
			return nil, fmt.Errorf("-shots 步數要遞增")
		}
		out = append(out, shot{n, part[i+1:]})
	}
	return out, nil
}

func parseKeys(text, names string) ([]dos.Key, error) {
	var q []dos.Key
	for _, r := range text {
		if k, ok := dos.KeyForRune(r); ok {
			q = append(q, k)
			continue
		}
		switch r {
		case '\r', '\n':
			q = append(q, dos.Key{Scan: 0x1C, ASCII: '\r'})
		default:
			q = append(q, dos.Key{ASCII: uint8(r)})
		}
	}
	if strings.TrimSpace(names) != "" {
		for _, n := range strings.Split(names, ",") {
			n = strings.TrimSpace(n)
			k, ok := dos.KeyNamed(n)
			if r := []rune(n); !ok && len(r) == 1 {
				k, ok = dos.KeyForRune(r[0]) // 單一字元可與具名鍵交錯（例 Esc,V,E）
			}
			if !ok {
				return nil, fmt.Errorf("不認得的鍵名 %q", n)
			}
			q = append(q, k)
		}
	}
	return q, nil
}

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	return b
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "dkk-overlay-shot：", err)
	os.Exit(1)
}
