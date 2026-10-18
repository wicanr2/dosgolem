// strikecommander-label-receipt 是 strike_commander 規格 001 §2.8 的收據工具：
// 從 probe 狀態檔以 session.BootFromState 啟動，觀察 OPTTEST 熱點標籤的提交，
// 輸出每次提交、墨跡框核對、指定步數的合成 PNG，以及觀察器與 stamp 開、關各一份 Digest。
//
//	strikecommander-label-receipt -root <SB16 設定樹> -state <s449.state> -state-sha256 <hex> \
//	    -steps 4492700000 -tsv <catalog.tsv> -font <zh-TW.golemfnt> -out <輸出目錄>
//
// -script 給規格 002 §2.4 的腳本（move、down、up、shot、stop）時，依腳本送滑鼠事件，
// 每個 shot 另出各倍率的合成 PNG；-steps 可省，給了就必須等於腳本的 stop。
//
// -root 是唯讀來源；每次執行另外實體複製一份到 -work 底下當 SaveRoot（遊戲會寫入），結束時刪除。
package main

import (
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
	script := flag.String("script", "", "腳本檔（strike_commander 規格 002 §2.4）")
	scales := flag.String("scales", "2,3", "合成圖倍率，逗號分隔（2、3）")
	tsv := flag.String("tsv", "", "譯文 catalog（key、translation、source）")
	gfxTSV := flag.String("gfx-tsv", "", "圖形字 catalog text/menu_gfx.<lang>.tsv（存讀檔對話框，規格 004 §3.6）；可省")
	fontPath := flag.String("font", "", "GOLEMFNT 字型")
	out := flag.String("out", "", "輸出目錄（PNG 與 RGBA 原始檔）")
	work := flag.String("work", os.TempDir(), "放 SaveRoot 複本的目錄")
	cursorY := flag.Int("cursor-y", -1, "滑鼠游標中心的原版列；負值為未知（墨跡框核對不略過）")
	ab := flag.Bool("ab", true, "另跑一次觀察器與 stamp 關閉，比較 Digest")
	flag.Parse()
	if err := run(*root, *statePath, *stateSHA, *rebase, *steps, *script, *scales, *tsv, *gfxTSV, *fontPath, *out, *work, *cursorY, *ab); err != nil {
		fmt.Fprintln(os.Stderr, "錯誤：", err)
		os.Exit(1)
	}
}

func run(root, statePath, stateSHA, rebase string, steps uint64, script, scalesArg, tsv, gfxTSV, fontPath, out, work string, cursorY int, ab bool) error {
	if root == "" || statePath == "" || stateSHA == "" || (steps == 0 && script == "") || tsv == "" || fontPath == "" || out == "" {
		return fmt.Errorf("缺必要參數（-root -state -state-sha256 -steps 或 -script、-tsv -font -out）")
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
	font, err := strikecommander.LoadLabelFont(fontPath, "full16-zh-TW")
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
		Scales: scales, Catalog: cat, GfxCatalog: gfx, Font: font, Overlay: true, CursorY: cursorY,
		Inputs: sc.Inputs, Shots: sc.Shots,
	}
	on, err := runOnce(root, work, cfg)
	if err != nil {
		return err
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
		fmt.Printf("對話框繪圖事件 第 %d 步：組 %d、k %d、位置 (%d,%d)、Order %d\n", a.Steps, a.G, a.K, a.X, a.Y, a.Order)
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
	}
	for _, s := range on.Shots {
		name := fmt.Sprintf("shot-%d", s.Steps)
		if s.Name != "" {
			name += "-" + s.Name
		}
		for _, sc := range scales {
			cc := s.Check[sc]
			fmt.Printf("shot 第 %d 步 %s，%d 倍：合成圖差異 %d、遮蓋框外 %d\n", s.Steps, s.Name, sc, cc.Diffs, cc.Outside)
			if len(s.ArtKeys[sc]) != 0 {
				fmt.Printf("  圖面 %v\n", s.ArtKeys[sc])
			}
			if err := writePNG(filepath.Join(out, fmt.Sprintf("%s-composite-%dx.png", name, sc)), s.Composites[sc], sc); err != nil {
				return err
			}
			if err := writePNG(filepath.Join(out, fmt.Sprintf("%s-plain-%dx.png", name, sc)), s.Plain[sc], sc); err != nil {
				return err
			}
		}
	}
	printDigest("開", on.Digest)
	if !ab {
		return nil
	}
	cfg.Overlay = false
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
