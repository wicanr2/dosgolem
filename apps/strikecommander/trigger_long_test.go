package strikecommander

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// strike_commander 規格 001 §3.3 的觸發案例：從 probe 狀態檔啟動 session，觀察熱點標籤的提交。
//
// 長測試。只在下列環境變數都設定時執行，否則 t.Skip：
//   - DOSGOLEM_SC_ROOT：448 的 SB16 設定樹（每個子測試實體複製到 t.TempDir() 當 SaveRoot）
//   - DOSGOLEM_SC_STATE_DIR：strike_commander 的 workplace/scratch（底下有 p485、p530、p539）
//   - DOSGOLEM_SC_FONT：tools/font/build.sh 建的 zh-TW.golemfnt
// 譯文是 testdata/opttest_labels.test.tsv（不是正式譯文），字型要含測試字（strike_commander font/README.md 的 font-test）。
//
// 子測試：
//   - A、B、C：§3.3 的三個案例，斷言提交、墨跡框核對、每次提交的 stamp 數與合成圖差異範圍。
//   - ANoCaller：關掉 §2.3 重跑 A，只 t.Log 每次提交的接受數（預期從第二次提交起為 2）。
//   - AOverlayOff：A 的同狀態 A/B，觀察器與 stamp 開、關各跑一次，Digest 的 Memory、CPU、Indexed、Palette 必須相同。

const (
	scRootEnv     = "DOSGOLEM_SC_ROOT"
	scStateDirEnv = "DOSGOLEM_SC_STATE_DIR"
	scFontEnv     = "DOSGOLEM_SC_FONT"
	scCatalogEnv  = "DOSGOLEM_SC_CATALOG"
	// 448 §3.2 的 SB16 設定（含 -ma -ds）。
	scCFGSHA256 = "317b0abdbf538f52d95e13b7358712e6da30a1a6d6e312e78d4943254b34e36f"
)

type wantCommit struct {
	entry     uint64
	key       string
	x0, x1, y int
	accepted  int
	ignored   int // -1：不斷言
	seg       uint16
}

type triggerCase struct {
	state  string
	sha    string
	stop   uint64
	cursor int
	want   []wantCommit
}

var triggerCases = map[string]triggerCase{
	"A": {"p485/s449.state", "7dff6dd268ef38d04086aeaa3ba22d5e2b23c5dc953de97dfe73fc351b1e47a0", 4492700000, -1, []wantCommit{
		{4490362413, "Talk to Gwen", 114, 204, 175, 1, -1, 0},
		{4491518424, "Talk to Gwen", 114, 204, 175, 1, 1, 0},
		{4492676586, "Talk to Gwen", 114, 204, 175, 1, 1, 0},
	}},
	"B": {"p530/s4495b.state", "34c5b2e5a50131cda4537428ad0c1e4d999a3302a6bc6e60c34ba177fc3b48cd", 4496200000, 75, []wantCommit{
		{4496160311, "Transport to Strike Base", 68, 251, 175, 1, 0, 0},
	}},
	"C": {"p539/s500b.state", "0fbe712170ff91b9d83a5346fcaac9f98a5f09ef10b6295d4e6914b51f0d4ee6", 5000900000, 75, []wantCommit{
		{5000842631, "Fly Mission", 124, 195, 175, 1, 0, 0xAD22},
	}},
}

func runTrigger(t *testing.T, tc triggerCase, overlay, noCaller bool) *ReceiptResult {
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
	raw, err := os.ReadFile(filepath.Join(os.Getenv(scStateDirEnv), tc.state))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(tc.sha)); err != nil {
		t.Fatal(err)
	}
	font, err := LoadLabelFont(os.Getenv(scFontEnv), "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := LoadCatalog("testdata/opttest_labels.test.tsv")
	if err != nil {
		t.Fatal(err)
	}
	res, err := RunReceipt(ReceiptConfig{
		SaveRoot: root, State: raw, StateSHA256: want, RebaseFrom: []string{"/scratch"},
		StopSteps: tc.stop, Scales: []int{2, 3}, Catalog: cat, Font: font, Overlay: overlay, CursorY: tc.cursor,
		noCallerCheck: noCaller,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("BootReceipt Steps=%d StateSHA256=%x；停止 %d", res.Boot.Steps, res.Boot.StateSHA256, res.Digest.Steps)
	for i, c := range res.Commits {
		t.Logf("提交 %d：blit 入口 %d 返回 %d，接受 %d、忽略 %d，gen %d，stamp %v", i, c.BlitEntry, c.BlitReturn, c.Accepted, c.Ignored, c.Generation, res.Stamps[i])
		for j, l := range c.Labels {
			text, rule, reason := ResolveLabel(l.Key, cat)
			t.Logf("  %q X0 %d X1 %d Y %d 字型 %04X 段 %04X 列表 %04X puts 第 %d 步，有譯文 %t（%s%s）",
				l.Key, l.X0, l.X1, l.Y, l.Font, l.Seg, l.List, l.Steps, text != nil, rule, reason)
			if j < len(c.Ink) {
				t.Logf("  墨跡框核對：%s", c.Ink[j])
			}
		}
	}
	for _, e := range res.Log {
		t.Logf("紀錄 第 %d 步 %s：%s", e.Steps, e.Kind, e.Detail)
	}
	return res
}

func checkTrigger(t *testing.T, tc triggerCase, res *ReceiptResult) {
	t.Helper()
	if len(res.Commits) != len(tc.want) {
		t.Fatalf("提交 %d 次，要 %d", len(res.Commits), len(tc.want))
	}
	for i, w := range tc.want {
		c := res.Commits[i]
		if c.BlitEntry != w.entry {
			t.Errorf("提交 %d 的 blit 入口第 %d 步，要 %d", i, c.BlitEntry, w.entry)
		}
		if len(c.Labels) != 1 {
			t.Errorf("提交 %d 有 %d 筆標籤，要 1", i, len(c.Labels))
			continue
		}
		l := c.Labels[0]
		if l.Key != w.key || l.X0 != w.x0 || l.X1 != w.x1 || l.Y != w.y {
			t.Errorf("提交 %d：%q X0 %d X1 %d Y %d，要 %q %d %d %d", i, l.Key, l.X0, l.X1, l.Y, w.key, w.x0, w.x1, w.y)
		}
		if w.seg != 0 && l.Seg != w.seg {
			t.Errorf("提交 %d 的緩衝段 %04X，要 %04X", i, l.Seg, w.seg)
		}
		if c.Accepted != w.accepted {
			t.Errorf("提交 %d 接受 %d，要 %d", i, c.Accepted, w.accepted)
		}
		if w.ignored >= 0 && c.Ignored != w.ignored {
			t.Errorf("提交 %d 忽略 %d，要 %d", i, c.Ignored, w.ignored)
		}
		for _, ink := range c.Ink {
			if !ink.OK() {
				t.Errorf("提交 %d 墨跡框核對未通過：%s", i, ink)
			}
		}
		if len(c.Ink) != len(c.Labels) {
			t.Errorf("提交 %d 的核對 %d 筆，標籤 %d 筆", i, len(c.Ink), len(c.Labels))
		}
		for _, s := range res.Stamps[i] {
			if s.Stamps != 2*len(c.Labels) {
				t.Errorf("提交 %d 的 %d 倍 stamp %d 個，要 %d", i, s.Scale, s.Stamps, 2*len(c.Labels))
			}
		}
	}
	t.Logf("最後回掃畫格與停止時 Indexed 相同：%t", res.FrameMatches)
	for _, sc := range []int{2, 3} {
		cc := res.CompositeCheck[sc]
		t.Logf("%d 倍合成圖：差異 %d、框外 %d；misses %+v skips %+v", sc, cc.Diffs, cc.Outside, res.Overlays[sc].Misses, res.Overlays[sc].Skips)
		if cc.Outside != 0 || cc.Diffs == 0 {
			t.Errorf("%d 倍合成圖差異 %d、框外 %d", sc, cc.Diffs, cc.Outside)
		}
	}
}

func TestLabelOverlayTrigger(t *testing.T) {
	for _, e := range []string{scRootEnv, scStateDirEnv, scFontEnv} {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定：001 §3.3 觸發案例需要設定樹、狀態檔與字型")
		}
	}
	for _, name := range []string{"A", "B", "C"} {
		t.Run(name, func(t *testing.T) {
			tc := triggerCases[name]
			checkTrigger(t, tc, runTrigger(t, tc, true, false))
		})
	}
	t.Run("ANoCaller", func(t *testing.T) {
		res := runTrigger(t, triggerCases["A"], true, true)
		for i, c := range res.Commits {
			t.Logf("負對照（關掉 §2.3）提交 %d：接受 %d", i, c.Accepted)
		}
	})
	t.Run("AOverlayOff", func(t *testing.T) {
		on := runTrigger(t, triggerCases["A"], true, false)
		off := runTrigger(t, triggerCases["A"], false, false)
		a, b := on.Digest, off.Digest
		t.Logf("開：Memory %x CPU %x Indexed %x Palette %x", a.MemorySHA256, a.CPUSHA256, a.IndexedSHA256, a.PaletteSHA256)
		t.Logf("關：Memory %x CPU %x Indexed %x Palette %x", b.MemorySHA256, b.CPUSHA256, b.IndexedSHA256, b.PaletteSHA256)
		if a.Steps != b.Steps || a.MemorySHA256 != b.MemorySHA256 || a.CPUSHA256 != b.CPUSHA256 ||
			a.IndexedSHA256 != b.IndexedSHA256 || a.PaletteSHA256 != b.PaletteSHA256 || a.ExtSHA256 != b.ExtSHA256 {
			t.Fatal("觀察器與 stamp 開、關的 Digest 不同")
		}
	})
}

// TestBuiltFont 以 Go 重讀 tools/font/build.sh 的產物（獨立於 build_font.py 的核對）：
// 測試 TSV 的每個譯文字都在全形表裡，lint 通過。只在 DOSGOLEM_SC_FONT 設定時執行。
func TestBuiltFont(t *testing.T) {
	p := os.Getenv(scFontEnv)
	if p == "" {
		t.Skip(scFontEnv + " 未設定")
	}
	f, err := LoadLabelFont(p, "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("testdata/opttest_labels.test.tsv")
	if err != nil {
		t.Fatal(err)
	}
	labl := testLABL("Talk to Gwen", "Transport to Strike Base", "Fly Mission")
	if rep := Lint(b, f.Wide, labl); !rep.OK() {
		t.Fatalf("lint：%v", rep.Errors)
	}
	half := 0
	for r := rune(0x20); r <= 0x7E; r++ {
		if f.Wide.Has(r) && !f.Wide.Wide(r) {
			half++
		}
	}
	t.Logf("字數 %d，ASCII 半形 %d", f.Wide.Len(), half)
	if half != 95 {
		t.Fatalf("ASCII 20h 到 7Eh 應全部以半形收入，得 %d", half)
	}
}

// TestFormalCatalog 以 lint 檢查正式譯文：DOSGOLEM_SC_CATALOG 是 text/opttest_labels.<lang>.tsv，
// DOSGOLEM_SC_LABL 是 OPTIONS.IFF 與 OPTIONS2.IFF 的 LABL 去重清單（每列一個 key，原始位元組），
// 字型取 DOSGOLEM_SC_FONT，DOSGOLEM_SC_FONT_EXTRA 是 strike_commander font/zh-TW.extra.txt。
// 四者都設定時才執行。斷言 lint 通過、漏譯為 0（002 §2.3）、extra 檔內容等於 FontExtraRunes，
// 且字型含這 12 字（002 §2.5）。
func TestFormalCatalog(t *testing.T) {
	cp, lp, fp, xp := os.Getenv(scCatalogEnv), os.Getenv("DOSGOLEM_SC_LABL"), os.Getenv(scFontEnv), os.Getenv("DOSGOLEM_SC_FONT_EXTRA")
	if cp == "" || lp == "" || fp == "" || xp == "" {
		t.Skip(scCatalogEnv + "、DOSGOLEM_SC_LABL、" + scFontEnv + "、DOSGOLEM_SC_FONT_EXTRA 未全部設定")
	}
	f, err := LoadLabelFont(fp, "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cp)
	if err != nil {
		t.Fatal(err)
	}
	lb, err := os.ReadFile(lp)
	if err != nil {
		t.Fatal(err)
	}
	labl := map[string]bool{}
	for _, k := range strings.Split(strings.TrimSuffix(string(lb), "\n"), "\n") {
		labl[k] = true
	}
	rep := Lint(b, f.Wide, labl)
	t.Logf("LABL %d 個，漏譯 %d：%v", len(labl), len(rep.Untranslated), rep.Untranslated)
	if !rep.OK() {
		t.Fatalf("lint：\n%s", strings.Join(rep.Errors, "\n"))
	}
	if len(rep.Untranslated) != 0 {
		t.Fatalf("正式 catalog 漏譯 %d：%v", len(rep.Untranslated), rep.Untranslated)
	}
	xb, err := os.ReadFile(xp)
	if err != nil {
		t.Fatal(err)
	}
	if string(xb) != FontExtraRunes {
		t.Fatalf("%s 的內容 %q，要等於 FontExtraRunes %q", xp, xb, FontExtraRunes)
	}
	if err := CheckFontExtra(f.Wide); err != nil {
		t.Fatal(err)
	}
}
