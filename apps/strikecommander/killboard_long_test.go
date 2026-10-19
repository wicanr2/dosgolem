package strikecommander

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// strike_commander 規格 003 §4.2 的案例 E：從基地的狀態檔依腳本進辦公室、點計分板，再把游標移到底部標籤，
// 觀察白板欄名 GROUND、AIR（board）與底部標籤 Return to office（label）的提交與 stamp。
//
// 長測試（約 5 億步，開、關、noBoardCaller 各一次）。只在下列環境變數都設定時執行，否則 t.Skip：
//   - DOSGOLEM_SC_ROOT、DOSGOLEM_SC_STATE_DIR：同 trigger_long_test.go
//   - DOSGOLEM_SC_FONT：font-test（正式字型的超集）
//   - DOSGOLEM_SC_CATALOG：正式 catalog text/opttest_labels.zh-TW.tsv
//
// DOSGOLEM_SC_OUT 有設定時，把第 4989000000 步 shot 的合成圖、原版放大圖、原版色號畫格與色盤寫到該目錄，
// 供主代理對 workplace/i013/kb6/b0.bin 做 §4.2 的獨立比對。
//
// 腳本是 testdata/case_e.script。期望值取自盤點工具 strike_commander workplace/i013/kb3/trace.tsv
// （同一腳本步數），獨立於本規格的實作。
//
// 子測試：
//   - E：觀察器開、關各跑一次；斷言視窗 [4980000000, 4990000000) 內的提交數、各 key 的筆數與第一筆
//     Label.Steps、接受數、忽略數、stamp 文字與種類、墨跡核對，shot 的合成圖差異只在遮蓋框內，
//     停止時開、關的 Digest 全同。印出 GROUND、AIR 的 x1（003 §2.3 的強推論是 250、290）。
//   - ENoBoardCaller：負對照，只關掉 003 §3.1 條件 2；地面、空中、返回辦公室都不出現。

const (
	caseEState = "p485/s449.state"
	caseESHA   = "7dff6dd268ef38d04086aeaa3ba22d5e2b23c5dc953de97dfe73fc351b1e47a0"
	// 視窗與游標移動（腳本的 move 4984000000 8 156）。
	caseEFrom, caseETo = 4980000000, 4990000000
	caseEMove          = 4984000000
	caseEShot          = 4989000000
	scOutEnv           = "DOSGOLEM_SC_OUT"
)

type caseEWant struct {
	key, text string
	kind      Kind
	first     uint64 // 視窗內第一筆的 Label.Steps
	n         int    // 視窗內的筆數
}

var caseEWants = []caseEWant{
	{"GROUND", "地面", KindBoard, 4980293261, 17},
	{"AIR", "空中", KindBoard, 4980297642, 17},
	{"Return to office", "返回辦公室", KindLabel, 4984419359, 10},
}

func runCaseE(t *testing.T, overlay, noBoard bool) *ReceiptResult {
	t.Helper()
	root := t.TempDir()
	if err := CopyTree(os.Getenv(scRootEnv), root); err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(filepath.Join(root, "SC.CFG"))
	if err != nil {
		t.Fatal(err)
	}
	if s := sha256.Sum256(cfg); hex.EncodeToString(s[:]) != scCFGSHA256 {
		t.Fatalf("SC.CFG SHA-256 %x，要 %s", s, scCFGSHA256)
	}
	raw, err := os.ReadFile(filepath.Join(os.Getenv(scStateDirEnv), caseEState))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(caseESHA)); err != nil {
		t.Fatal(err)
	}
	fontBytes, err := os.ReadFile(os.Getenv(scFontEnv))
	if err != nil {
		t.Fatal(err)
	}
	font, err := ParseLabelFont(fontBytes, "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	catBytes, err := os.ReadFile(os.Getenv(scCatalogEnv))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := ParseCatalog(catBytes)
	if err != nil {
		t.Fatal(err)
	}
	fs, cs := sha256.Sum256(fontBytes), sha256.Sum256(catBytes)
	t.Logf("字型 %s SHA-256 %x；catalog %s SHA-256 %x", os.Getenv(scFontEnv), fs, os.Getenv(scCatalogEnv), cs)
	sb, err := os.ReadFile("testdata/case_e.script")
	if err != nil {
		t.Fatal(err)
	}
	script, err := ParseScript(sb, 0) // 起始步數由 RunReceipt 以 BootReceipt.Steps 再驗
	if err != nil {
		t.Fatal(err)
	}
	res, err := RunReceipt(ReceiptConfig{
		SaveRoot: root, State: raw, StateSHA256: want, RebaseFrom: []string{"/scratch"},
		StopSteps: script.Stop, Scales: []int{2, 3}, Catalog: cat, Font: font, Overlay: overlay, CursorY: -1,
		Inputs: script.Inputs, Shots: script.Shots, noBoardCaller: noBoard,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("BootReceipt Steps=%d StateSHA256=%x；停止 %d", res.Boot.Steps, res.Boot.StateSHA256, res.Digest.Steps)
	if !overlay {
		return res
	}
	// 視窗外的提交只摘要（基地、辦公室的熱點標籤），視窗內逐筆記錄。
	outside := map[string]int{}
	for i, c := range res.Commits {
		if c.BlitEntry < caseEFrom || c.BlitEntry >= caseETo {
			for _, l := range c.Labels {
				outside[fmt.Sprintf("%s/%s", l.Key, l.Kind)]++
			}
			continue
		}
		t.Logf("提交 %d：blit 入口 %d 返回 %d，接受 %d、忽略 %d，gen %d，stamp %+v", i, c.BlitEntry, c.BlitReturn, c.Accepted, c.Ignored, c.Generation, res.Stamps[i])
		for j, l := range c.Labels {
			t.Logf("  %q 種類 %s X0 %d X1 %d Y %d 字型 %04X 段 %04X puts 第 %d 步", l.Key, l.Kind, l.X0, l.X1, l.Y, l.Font, l.Seg, l.Steps)
			if j < len(c.Ink) {
				t.Logf("  墨跡核對：%s", c.Ink[j])
			}
		}
	}
	var ks []string
	for k, n := range outside {
		ks = append(ks, fmt.Sprintf("%s ×%d", k, n))
	}
	sort.Strings(ks)
	t.Logf("視窗外的提交標籤：%s", strings.Join(ks, "、"))
	for _, e := range res.Log {
		t.Logf("紀錄 第 %d 步 %s：%s", e.Steps, e.Kind, e.Detail)
	}
	for _, s := range res.Shots {
		for _, sc := range []int{2, 3} {
			t.Logf("shot 第 %d 步 %d 倍：差異 %d、框外 %d", s.Steps, sc, s.Check[sc].Diffs, s.Check[sc].Outside)
		}
	}
	for sc, ov := range res.Overlays {
		t.Logf("%d 倍 misses %+v skips %+v", sc, ov.Misses, ov.Skips)
	}
	return res
}

// checkCaseE 斷言案例 E 視窗內的提交、x1、stamp 與 shot。
func checkCaseE(t *testing.T, res *ReceiptResult) {
	t.Helper()
	wants := map[string]caseEWant{}
	for _, w := range caseEWants {
		wants[w.key] = w
	}
	type seen struct {
		first uint64
		n     int
		x1    map[int]int
	}
	got := map[string]*seen{}
	commits := 0
	for i, c := range res.Commits {
		if c.BlitEntry < caseEFrom || c.BlitEntry >= caseETo {
			continue
		}
		commits++
		// 游標移到 (8, 156) 之後，底部標籤加入：每畫格接受 3，之前 2；忽略 24（8 個呼號、16 個擊落數）。
		var expect []caseEWant
		expect = append(expect, wants["GROUND"], wants["AIR"])
		if c.BlitEntry > caseEMove {
			expect = append(expect, wants["Return to office"])
		}
		if c.Accepted != len(expect) || c.Ignored != 24 {
			t.Errorf("提交 %d（blit 入口 %d）接受 %d、忽略 %d，要 %d、24", i, c.BlitEntry, c.Accepted, c.Ignored, len(expect))
		}
		if len(c.Labels) != len(expect) {
			t.Errorf("提交 %d 有 %d 筆標籤，要 %d", i, len(c.Labels), len(expect))
		}
		var texts []string
		var kinds []Kind
		for j, l := range c.Labels {
			w, ok := wants[l.Key]
			if !ok {
				t.Errorf("提交 %d 有預期外的 key %q（第 %d 步）", i, l.Key, l.Steps)
				continue
			}
			if l.Kind != w.kind {
				t.Errorf("提交 %d 的 %q 種類 %s，要 %s", i, l.Key, l.Kind, w.kind)
			}
			if j < len(expect) && expect[j].key != l.Key {
				t.Errorf("提交 %d 第 %d 筆是 %q，要 %q", i, j, l.Key, expect[j].key)
			}
			texts = append(texts, w.text)
			kinds = append(kinds, w.kind)
			s := got[l.Key]
			if s == nil {
				s = &seen{first: l.Steps, x1: map[int]int{}}
				got[l.Key] = s
			}
			s.n++
			s.x1[l.X1]++
			if l.Kind == KindBoard && (len(l.Before) != 8*320 || len(l.After) != 8*320) {
				t.Errorf("提交 %d 的 %q 前後列 %d、%d bytes，要各 2560", i, l.Key, len(l.Before), len(l.After))
			}
		}
		for _, s := range res.Stamps[i] {
			if s.Stamps != 2*len(texts) || strings.Join(s.Texts, "|") != strings.Join(texts, "|") || fmt.Sprint(s.Kinds) != fmt.Sprint(kinds) {
				t.Errorf("提交 %d 的 %d 倍 stamp %d 個、文字 %q、種類 %v，要 %d 個 %q %v", i, s.Scale, s.Stamps, s.Texts, s.Kinds, 2*len(texts), texts, kinds)
			}
		}
		if len(c.Ink) != len(c.Labels) {
			t.Errorf("提交 %d 的核對 %d 筆，標籤 %d 筆", i, len(c.Ink), len(c.Labels))
		}
		for _, ink := range c.Ink {
			if !ink.OK() {
				t.Errorf("提交 %d 墨跡核對未通過：%s", i, ink)
			}
		}
	}
	if commits != 17 {
		t.Errorf("視窗內提交 %d 次，要 17", commits)
	}
	for _, w := range caseEWants {
		s := got[w.key]
		if s == nil {
			t.Errorf("%q 在視窗內沒有提交", w.key)
			continue
		}
		t.Logf("%q：視窗內 %d 筆，第一筆第 %d 步，x1 分布 %v", w.key, s.n, s.first, s.x1)
		if s.first != w.first || s.n != w.n {
			t.Errorf("%q：第一筆 %d、筆數 %d，要 %d、%d", w.key, s.first, s.n, w.first, w.n)
		}
	}
	// shot：差異只在遮蓋框內。
	var shot *ShotResult
	for i := range res.Shots {
		if res.Shots[i].Steps == caseEShot {
			shot = &res.Shots[i]
		}
	}
	if shot == nil {
		t.Fatalf("沒有第 %d 步的 shot", caseEShot)
	}
	for _, sc := range []int{2, 3} {
		if cc := shot.Check[sc]; cc.Outside != 0 || cc.Diffs == 0 {
			t.Errorf("shot %d 倍合成圖差異 %d、框外 %d", sc, cc.Diffs, cc.Outside)
		}
	}
	if dir := os.Getenv(scOutEnv); dir != "" {
		if err := writeCaseEShot(dir, shot); err != nil {
			t.Fatal(err)
		}
		t.Logf("shot 輸出寫到 %s", dir)
	}
}

// writeCaseEShot 把 shot 的 2、3 倍合成圖、原版放大圖、原版色號畫格（320×200）與色盤（768 bytes）寫到 dir。
func writeCaseEShot(dir string, s *ShotResult) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("shot-%d", s.Steps)
	for _, sc := range []int{2, 3} {
		if err := writeRGBAPNG(filepath.Join(dir, fmt.Sprintf("%s-composite-%dx.png", name, sc)), s.Composites[sc], sc); err != nil {
			return err
		}
		if err := writeRGBAPNG(filepath.Join(dir, fmt.Sprintf("%s-plain-%dx.png", name, sc)), s.Plain[sc], sc); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, name+"-frame.idx"), s.Indexed, 0o644); err != nil {
		return err
	}
	pal := make([]byte, 0, 768)
	for _, c := range s.Palette {
		pal = append(pal, c[0], c[1], c[2])
	}
	return os.WriteFile(filepath.Join(dir, name+"-palette.bin"), pal, 0o644)
}

// writeRGBAPNG 同 cmd/strikecommander-label-receipt 的 writePNG。
func writeRGBAPNG(path string, rgba []byte, scale int) error {
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

func TestKillBoardCaseE(t *testing.T) {
	for _, e := range []string{scRootEnv, scStateDirEnv, scFontEnv, scCatalogEnv} {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定：003 §4.2 案例 E 需要設定樹、狀態檔、字型與正式 catalog")
		}
	}
	t.Run("E", func(t *testing.T) {
		on := runCaseE(t, true, false)
		checkCaseE(t, on)
		checkNoSig(t, on) // 005 §4.4
		logMeasure(t, "E", on)
		off := runCaseE(t, false, false)
		a, b := on.Digest, off.Digest
		t.Logf("開：Memory %x CPU %x Indexed %x Palette %x", a.MemorySHA256, a.CPUSHA256, a.IndexedSHA256, a.PaletteSHA256)
		t.Logf("關：Memory %x CPU %x Indexed %x Palette %x", b.MemorySHA256, b.CPUSHA256, b.IndexedSHA256, b.PaletteSHA256)
		if a.Steps != b.Steps || a.MemorySHA256 != b.MemorySHA256 || a.CPUSHA256 != b.CPUSHA256 ||
			a.IndexedSHA256 != b.IndexedSHA256 || a.PaletteSHA256 != b.PaletteSHA256 || a.ExtSHA256 != b.ExtSHA256 {
			t.Fatal("觀察器與 stamp 開、關的 Digest 不同")
		}
	})
	t.Run("ENoBoardCaller", func(t *testing.T) {
		res := runCaseE(t, true, true)
		banned := map[string]bool{"地面": true, "空中": true, "返回辦公室": true}
		for i, cs := range res.Stamps {
			for _, s := range cs {
				for _, x := range s.Texts {
					if banned[x] {
						t.Errorf("提交 %d（blit 入口 %d）的 %d 倍 stamp 有 %q", i, res.Commits[i].BlitEntry, s.Scale, x)
					}
				}
			}
		}
		n := 0
		for _, c := range res.Commits {
			if c.BlitEntry >= caseEFrom && c.BlitEntry < caseETo {
				n++
				if len(c.Labels) != 0 {
					t.Errorf("blit 入口 %d 的提交有標籤 %q，計分板應維持英文", c.BlitEntry, keys(c.Labels))
				}
			}
		}
		if n != 17 {
			t.Errorf("視窗內提交 %d 次，要 17", n)
		}
	})
}
