// strikecommander-label-receipt 是 strike_commander 規格 001 §2.8 的收據工具：
// 從 probe 狀態檔以 session.BootFromState 啟動，觀察 OPTTEST 熱點標籤的提交，
// 輸出每次提交、墨跡框核對、指定步數的合成 PNG，以及觀察器與 stamp 開、關各一份 Digest。
//
//	strikecommander-label-receipt -root <SB16 設定樹> -state <s449.state> -state-sha256 <hex> \
//	    -steps 4492700000 -tsv <catalog.tsv> -font-dir <圖集目錄> [-font-expected <預期表>] -out <輸出目錄>
//
// -script 給規格 002 §2.4 的腳本（move、down、up、kdown、kup、shot、stop；kdown、kup 見規格 007 §3.8）時，
// 依腳本送滑鼠與鍵盤事件，每個 shot 另出各倍率的合成 PNG；-steps 可省，給了就必須等於腳本的 stop。
//
// -root 是唯讀來源；每次執行另外實體複製一份到 -work 底下當 SaveRoot（遊戲會寫入），結束時刪除。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wicanr2/dosgolem/apps/strikecommander"
	"github.com/wicanr2/dosgolem/session"
)

func main() {
	root := flag.String("root", "", "SB16 設定樹（唯讀來源）")
	statePath := flag.String("state", "", "probe -save-state 狀態檔")
	stateSHA := flag.String("state-sha256", "", "狀態檔 SHA-256（十六進位）")
	rebase := flag.String("rebase-from", "/scratch", "狀態檔開檔路徑的舊前綴，逗號分隔（狀態檔的 Root 自動包含）")
	steps := flag.Uint64("steps", 0, "停止步數（絕對）；有 -script 時可省")
	script := flag.String("script", "", "腳本檔（strike_commander 規格 002 §2.4；鍵盤 kdown、kup 見規格 007 §3.8）")
	scales := flag.String("scales", "2,3", "合成圖倍率，逗號分隔（2、3）")
	tsv := flag.String("tsv", "", "譯文 catalog（key、translation、source）")
	gfxTSV := flag.String("gfx-tsv", "", "圖形字 catalog text/menu_gfx.<lang>.tsv（存讀檔對話框、主選單、MENU 片頭選單，規格 004 §3.6、008 §3.7）；可省")
	out := flag.String("out", "", "輸出目錄（PNG 與 RGBA 原始檔）")
	work := flag.String("work", os.TempDir(), "放 SaveRoot 複本的目錄")
	cursorY := flag.Int("cursor-y", -1, "滑鼠游標中心的原版列；負值為未知（墨跡框核對不略過）")
	ab := flag.Bool("ab", true, "另跑一次觀察器與 stamp 關閉，比較 Digest")
	flag.StringVar(&sub.index, "speech-index", "", "語音字幕結構表 text/speech.index.tsv（strike_commander 規格 010 §3.5）；給了 -subtitles 時必要")
	flag.StringVar(&sub.catalog, "speech-tsv", "", "語音字幕譯文 text/speech.<lang>.tsv（規格 010 §3.5）")
	flag.BoolVar(&sub.on, "subtitles", false, "掛上語音字幕（規格 010），收據另寫 receipt-subs.tsv、receipt-subskips.tsv、receipt-subsmeta.tsv 與各 shot 的 substamps.tsv")
	flag.Uint64Var(&sub.e, "sub-e", 0, "字幕顯示延長 E（步數，規格 010 §3.8）")
	flag.Uint64Var(&sub.t, "sub-t", strikecommander.SubtitleDefaultT, "等待續段的逾時 T（步數，規格 010 §3.4）")
	flag.BoolVar(&sub.sbdmaLog, "sbdma-log", false, "記下全部 SB16 DMA 事件寫成 receipt-sbdma.tsv（欄位同 probe -sb-dma-log；dosgolem 452）")
	flag.StringVar(&fl.text, "flight-text", "", "text 目錄：讀 strike_hud、strike_mfd、strike_plaque 三個 catalog（規格 009 §3.10）；沒給時不裝飛行字串觀察器")
	flag.StringVar(&tx.dir, "font-dir", "", "反鋸齒圖集目錄（zh-TW.cjkb.manifest.tsv 所在，strike_commander 規格 011 §3.9）；必要")
	flag.StringVar(&tx.expected, "font-expected", "", "圖集預期表 font/zh-TW.cjkb.expected.tsv；正式圖集必要，測試圖集不給")
	flag.Parse()
	if err := run(*root, *statePath, *stateSHA, *rebase, *steps, *script, *scales, *tsv, *gfxTSV, *out, *work, *cursorY, *ab); err != nil {
		fmt.Fprintln(os.Stderr, "錯誤：", err)
		os.Exit(1)
	}
}

// sub 是規格 010 的旗標（語音字幕）。
var sub struct {
	index, catalog string
	on, sbdmaLog   bool
	e, t           uint64
}

// tx 是規格 011 的旗標（反鋸齒圖集）。
var tx struct {
	dir, expected string
}

// fl 是規格 009 的旗標（飛行 HUD、MFD、訊息橫幅）。
var fl struct {
	text string
}

// loadFlight 依 -flight-text 設定 cfg（009 §3.11 接點第 6a 條；字型取 -font-dir 的圖集，011 §3.9）；沒給時不裝。
func loadFlight(cfg *strikecommander.ReceiptConfig) error {
	if fl.text == "" {
		return nil
	}
	var cats strikecommander.FlightCatalogs
	for _, c := range []struct {
		name string
		dst  *map[string]string
	}{{"strike_hud", &cats.HUD}, {"strike_mfd", &cats.MFD}, {"strike_plaque", &cats.Plaque}} {
		m, err := strikecommander.LoadCatalog(filepath.Join(fl.text, c.name+".zh-TW.tsv"))
		if err != nil {
			return err
		}
		*c.dst = m
	}
	cfg.FlightCatalogs = &cats
	return nil
}

func run(root, statePath, stateSHA, rebase string, steps uint64, script, scalesArg, tsv, gfxTSV, out, work string, cursorY int, ab bool) error {
	if root == "" || statePath == "" || stateSHA == "" || (steps == 0 && script == "") || tsv == "" || out == "" || tx.dir == "" {
		return fmt.Errorf("缺必要參數（-root -state -state-sha256 -steps 或 -script、-tsv -font-dir -out）")
	}
	var sc strikecommander.Script
	if script != "" {
		b, err := os.ReadFile(script)
		if err != nil {
			return err
		}
		// 狀態檔步數要等啟動後才知道；RunReceipt 會以 BootReceipt.Steps 再驗一次。
		if sc, err = strikecommander.ParseScript(b, 0); err != nil {
			return err
		}
		if steps != 0 && steps != sc.Stop {
			return fmt.Errorf("-steps %d 不等於腳本的 stop %d", steps, sc.Stop)
		}
		steps = sc.Stop
	}
	var want [32]byte
	if b, err := hex.DecodeString(stateSHA); err != nil || len(b) != 32 {
		return fmt.Errorf("-state-sha256 不是 64 位十六進位")
	} else {
		copy(want[:], b)
	}
	var scales []int
	for _, s := range strings.Split(scalesArg, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || (n != 2 && n != 3) {
			return fmt.Errorf("倍率 %q 不支援", s)
		}
		scales = append(scales, n)
	}
	var rebaseFrom []string
	for _, p := range strings.Split(rebase, ",") {
		if p = strings.TrimSpace(p); p != "" {
			rebaseFrom = append(rebaseFrom, p)
		}
	}
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	cat, err := strikecommander.LoadCatalog(tsv)
	if err != nil {
		return err
	}
	var gfx map[string]string
	if gfxTSV != "" {
		if gfx, err = strikecommander.LoadCatalog(gfxTSV); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	cfg := strikecommander.ReceiptConfig{
		State: raw, StateSHA256: want, RebaseFrom: rebaseFrom, StopSteps: steps,
		Scales: scales, Catalog: cat, GfxCatalog: gfx, Overlay: true, CursorY: cursorY,
		Inputs: sc.Inputs, Shots: sc.Shots,
	}
	if cfg.Fonts, err = strikecommander.LoadTextFonts(tx.dir, tx.expected); err != nil {
		return err
	}
	fmt.Printf("圖集 %s，anchor %s\n", tx.dir, cfg.Fonts.Anchor)
	var meta strikecommander.SubtitleMeta
	if sub.on {
		if sub.index == "" || sub.catalog == "" {
			return fmt.Errorf("-subtitles 要 -speech-index、-speech-tsv")
		}
		ib, err := os.ReadFile(sub.index)
		if err != nil {
			return err
		}
		if cfg.SpeechIndex, err = strikecommander.ParseSpeechIndex(ib); err != nil {
			return err
		}
		cb, err := os.ReadFile(sub.catalog)
		if err != nil {
			return err
		}
		if cfg.SpeechCatalog, err = strikecommander.ParseCatalog(cb); err != nil {
			return err
		}
		cfg.Subtitles, cfg.SubtitleE, cfg.SubtitleT = true, sub.e, sub.t
		meta = strikecommander.NewSubtitleMetaText(cfg.Fonts, ib, cb, sub.e, sub.t) // 規格 011 §3.8
	}
	cfg.RecordSBDMA = sub.sbdmaLog
	if err := loadFlight(&cfg); err != nil {
		return err
	}
	on, err := runOnce(root, work, cfg)
	if err != nil {
		return err
	}
	if sub.on || sub.sbdmaLog {
		// 規格 010 §3.11：字幕條、skip、中繼資料與各 shot 的 substamps.tsv（shot 的 PNG 等由下方照舊寫出）。
		subOnly := *on
		subOnly.Shots, subOnly.SubShots = nil, nil
		if err := strikecommander.WriteSubtitleReceipt(out, "receipt", &subOnly, meta, scales); err != nil {
			return err
		}
		for i := range on.Shots {
			if i < len(on.SubShots) {
				for _, s := range scales {
					p := filepath.Join(out, fmt.Sprintf("%s-s%d-substamps.tsv", strikecommander.ShotPrefix(&on.Shots[i]), s))
					if err := os.WriteFile(p, strikecommander.SubStampsTSV(on.SubShots[i], s), 0o644); err != nil {
						return err
					}
				}
			}
		}
		for _, s := range on.Subs {
			fmt.Printf("字幕條 %s 起點 %d parts %d 原因 %s：%s\n", s.Key, s.Start, s.Parts, s.Reason, s.Text)
		}
	}
	fmt.Printf("狀態檔 %s SHA-256 %x，啟動步數 %d，停止步數 %d\n", statePath, on.Boot.StateSHA256, on.Boot.Steps, on.Digest.Steps)
	for i, c := range on.Commits {
		fmt.Printf("提交 %d：blit 入口 %d、返回 %d，接受 %d、忽略 %d，generation %d，stamp %v\n",
			i, c.BlitEntry, c.BlitReturn, c.Accepted, c.Ignored, c.Generation, on.Stamps[i])
		for j, l := range c.Labels {
			text, rule, reason := strikecommander.ResolveLabel(l.Key, cat)
			fmt.Printf("  %q 種類 %s X0 %d X1 %d Y %d 字型 %04X 段 %04X puts 第 %d 步，有譯文 %t（%s%s）%s\n",
				l.Key, l.Kind, l.X0, l.X1, l.Y, l.Font, l.Seg, l.Steps, text != nil, rule, reason, string(text))
			if j < len(c.Ink) {
				fmt.Printf("  墨跡框核對：%s\n", c.Ink[j])
			}
		}
	}
	for _, a := range on.Arts {
		fmt.Printf("圖形字繪圖事件 第 %d 步：%v、組 %d、k %d、位置 (%d,%d)、Order %d\n", a.Steps, a.File, a.G, a.K, a.X, a.Y, a.Order)
	}
	for _, e := range on.Log {
		fmt.Printf("紀錄 第 %d 步 %s：%s\n", e.Steps, e.Kind, e.Detail)
	}
	if on.LogDropped != 0 {
		fmt.Printf("紀錄超過上限，捨棄 %d 筆\n", on.LogDropped)
	}
	fmt.Printf("非全螢幕 blit（未壓到可見墨跡框）%d 次\n", on.PartialBlits)
	fmt.Printf("最後回掃畫格與停止時 Indexed 相同：%t\n", on.FrameMatches)
	for _, sc := range scales {
		cc := on.CompositeCheck[sc]
		ov := on.Overlays[sc]
		fmt.Printf("%d 倍：合成圖差異 %d、遮蓋框外 %d；misses %+v；skips %+v\n", sc, cc.Diffs, cc.Outside, ov.Misses, ov.Skips)
		if err := writePNG(filepath.Join(out, fmt.Sprintf("composite-%dx.png", sc)), on.Composites[sc], sc); err != nil {
			return err
		}
		if err := writePNG(filepath.Join(out, fmt.Sprintf("plain-%dx.png", sc)), on.Plain[sc], sc); err != nil {
			return err
		}
		fmt.Printf("  BGFallbacks %+v\n", ov.BGFallbacks)
	}
	// 規格 006 §3.3：停止點的 frame.idx、palette.bin、labels.tsv、rows.bin 與 PNG（前綴 stop-<步數>）。
	if err := strikecommander.WriteStopFiles(out, on, scales); err != nil {
		return err
	}
	// 規格 011 §3.8：fonts.tsv（圖集的 file 列與 anchor）。
	if err := strikecommander.WriteFontsTSV(out, fmt.Sprintf("stop-%d", on.Digest.Steps), cfg.Fonts, tx.expected); err != nil {
		return err
	}
	if cfg.FlightCatalogs != nil {
		// 規格 009 §3.11 接點第 7 條：停止點與每個 shot 的 flight.tsv、flight-rows.bin、digest.txt。
		if err := strikecommander.WriteFlightFiles(out, fmt.Sprintf("stop-%d", on.Digest.Steps), scales, on.FlightReport, on.Digest); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, "flight-commits.tsv"), strikecommander.FlightCommitsTSV(on.FlightCommits), 0o644); err != nil {
			return err
		}
		// 規格 009 第 5 版 §3.11：整串裁切事件。
		if err := os.WriteFile(filepath.Join(out, "flight-clipped.tsv"), strikecommander.FlightClippedTSV(on.FlightClipped), 0o644); err != nil {
			return err
		}
		if on.FlightClippedDrop != 0 {
			fmt.Printf("整串裁切事件超過上限，捨棄 %d 筆\n", on.FlightClippedDrop)
		}
		fmt.Printf("飛行字串：升格 %d 次，計數 %+v\n", len(on.FlightCommits), on.FlightCounts)
		for _, e := range on.FlightLog {
			fmt.Printf("飛行紀錄 第 %d 步 %s：%s\n", e.Steps, e.Kind, e.Detail)
		}
		for _, sc := range scales {
			fov := on.FlightOverlays[sc]
			fmt.Printf("%d 倍飛行 overlay：misses %+v；skips %+v\n", sc, fov.Misses, fov.Skips)
		}
	}
	for i := range on.Shots {
		s := &on.Shots[i]
		// 規格 006 §3.3：shot 的收據檔（前綴 shot-<步數>-<名稱>，含 PNG）。
		if err := strikecommander.WriteShotFiles(out, strikecommander.ShotPrefix(s), s, scales); err != nil {
			return err
		}
		if cfg.FlightCatalogs != nil {
			if err := strikecommander.WriteFlightFiles(out, strikecommander.ShotPrefix(s), scales, s.FlightReport, s.Digest); err != nil {
				return err
			}
			fmt.Printf("  飛行 Visible %q Hidden %v\n", s.FlightVisible, s.FlightHidden)
		}
		for _, sc := range scales {
			cc := s.Check[sc]
			fmt.Printf("shot 第 %d 步 %s，%d 倍：合成圖差異 %d、遮蓋框外 %d\n", s.Steps, s.Name, sc, cc.Diffs, cc.Outside)
			if len(s.ArtKeys[sc]) != 0 {
				fmt.Printf("  圖面 %v\n", s.ArtKeys[sc])
			}
			// 規格 005 §4.2：shot 時的可見集合與比對不符而不畫的標籤。
			fmt.Printf("  Visible %q Hidden %q\n", s.Visible, s.Hidden[sc])
		}
	}
	// 規格 005 §4.4 的量測。
	for _, sc := range scales {
		for _, m := range []struct {
			tag string
			mc  *strikecommander.MeasureCount
		}{{"cur", on.Measure.Cur[sc]}, {"old", on.Measure.Old[sc]}} {
			if m.mc == nil {
				continue
			}
			fmt.Printf("量測 %d 倍 %s：可見非空回掃 %d，不符 %d，分類 (1) %d (2) %d (3) %d\n",
				sc, m.tag, on.Measure.Frames, m.mc.Mismatch, m.mc.Class[0], m.mc.Class[1], m.mc.Class[2])
			for _, e := range m.mc.Others {
				fmt.Printf("  (3) 第 %d 步 %v\n", e.Steps, e.Keys)
			}
		}
	}
	printDigest("開", on.Digest)
	if !ab {
		return nil
	}
	cfg.Overlay, cfg.Subtitles, cfg.RecordSBDMA, cfg.FlightCatalogs = false, false, false, nil
	off, err := runOnce(root, work, cfg)
	if err != nil {
		return err
	}
	printDigest("關", off.Digest)
	a, b := on.Digest, off.Digest
	same := a.Steps == b.Steps && a.MemorySHA256 == b.MemorySHA256 && a.CPUSHA256 == b.CPUSHA256 &&
		a.IndexedSHA256 == b.IndexedSHA256 && a.PaletteSHA256 == b.PaletteSHA256 && a.ExtSHA256 == b.ExtSHA256
	fmt.Printf("同狀態 A/B：Memory、CPU、Indexed、Palette、Ext 全同 %t\n", same)
	if !same {
		return fmt.Errorf("觀察器與 stamp 開、關的 Digest 不同")
	}
	return nil
}

func runOnce(root, work string, cfg strikecommander.ReceiptConfig) (*strikecommander.ReceiptResult, error) {
	dir, err := os.MkdirTemp(work, "sc-label-receipt-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if err := strikecommander.CopyTree(root, dir); err != nil {
		return nil, err
	}
	cfg.SaveRoot = dir
	return strikecommander.RunReceipt(cfg)
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func printDigest(name string, d session.StateDigest) {
	fmt.Printf("Digest（%s）：Steps %d Memory %x CPU %x Indexed %x Palette %x Ext %s\n",
		name, d.Steps, d.MemorySHA256, d.CPUSHA256, d.IndexedSHA256, d.PaletteSHA256, d.ExtSHA256)
}

func writePNG(path string, rgba []byte, scale int) error {
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
