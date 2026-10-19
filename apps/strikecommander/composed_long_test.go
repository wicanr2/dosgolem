package strikecommander

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// strike_commander 規格 002 §3.2 的案例 D：從停機坪的狀態檔依腳本點 Fly Mission 進武器掛載畫面，
// 游標移到兩處，觀察組合標籤（"%s (%d)"、"Remove %s (%d)"）經 082A 的提交與解析後的 stamp 文字。
//
// 長測試（約 6.3 億步，開、關各一次；DExactOnly 再一次）。只在下列環境變數都設定時執行，否則 t.Skip：
//   - DOSGOLEM_SC_ROOT、DOSGOLEM_SC_STATE_DIR：同 trigger_long_test.go
//   - DOSGOLEM_SC_FONT：002 §2.5 重建後的 font-test（正式字型的超集）
//   - DOSGOLEM_SC_CATALOG：正式 catalog text/opttest_labels.zh-TW.tsv
//
// 腳本是 testdata/case_d.script。期望值取自盤點工具 strike_commander workplace/i013/fly2/census.tsv
// （同一腳本步數），獨立於本規格的實作。
//
// 子測試：
//   - D：觀察器開、關各跑一次；斷言三個 key 的第一筆與最後一筆 Label.Steps、含該 key 的提交數、
//     沒有其他 key、每次提交各倍率的 stamp 文字、墨跡框核對、兩個 shot 的合成圖差異只在遮蓋框內、
//     停止時開、關的 Digest 全同。
//   - DExactOnly：負對照，解析只留第 1 步；兩組組合標籤記 miss（template）、stamp 數 0。

const (
	caseDState = "p539/s500b.state"
	caseDSHA   = "0fbe712170ff91b9d83a5346fcaac9f98a5f09ef10b6295d4e6914b51f0d4ee6"
	// caseDStart 是狀態檔的步數（001 §3.3：probe 在第 5000600000 步存的）；只供 TestCaseDScript 驗證腳本。
	caseDStart = 5000600000
)

type caseDWant struct {
	key, text   string
	first, last uint64 // 經 082A 的 Label.Steps
	commits     int    // 含該 key 的提交數
	composed    bool   // 組合標籤（DExactOnly 時 miss）
}

var caseDWants = []caseDWant{
	{"Fly Mission", "出擊", 5000833169, 5003149219, 3, false},
	{"AIM-9J rear aspect missile (42)", "尾追式飛彈（ＡＩＭ－９Ｊ）：４２", 5601146748, 5619673025, 17, true},
	{"Remove AIM-9J (2)", "卸下尾追式飛彈（ＡＩＭ－９Ｊ）：２", 5620831298, 5628940517, 8, true},
}

func runCaseD(t *testing.T, overlay, exactOnly bool) *ReceiptResult {
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
	raw, err := os.ReadFile(filepath.Join(os.Getenv(scStateDirEnv), caseDState))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(caseDSHA)); err != nil {
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
	sb, err := os.ReadFile("testdata/case_d.script")
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
		Inputs: script.Inputs, Shots: script.Shots, exactOnly: exactOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("BootReceipt Steps=%d StateSHA256=%x；停止 %d", res.Boot.Steps, res.Boot.StateSHA256, res.Digest.Steps)
	prev := map[string]uint64{}
	for i, c := range res.Commits {
		t.Logf("提交 %d：blit 入口 %d 返回 %d，接受 %d、忽略 %d，gen %d，stamp %+v", i, c.BlitEntry, c.BlitReturn, c.Accepted, c.Ignored, c.Generation, res.Stamps[i])
		for j, l := range c.Labels {
			text, rule, reason := resolveLabel(l.Key, cat, exactOnly)
			gap := ""
			if p, ok := prev[l.Key]; ok {
				gap = "，距上一筆 " + strconv.FormatUint(l.Steps-p, 10) + " 步"
			}
			prev[l.Key] = l.Steps
			t.Logf("  %q X0 %d X1 %d Y %d 字型 %04X 段 %04X puts 第 %d 步%s；解析 %q（%s%s）", l.Key, l.X0, l.X1, l.Y, l.Font, l.Seg, l.Steps, gap, string(text), rule, reason)
			if j < len(c.Ink) {
				t.Logf("  墨跡框核對：%s", c.Ink[j])
			}
		}
	}
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

// checkCaseD 斷言案例 D 的提交。exactOnly 時組合標籤的提交 stamp 數為 0。
func checkCaseD(t *testing.T, res *ReceiptResult, exactOnly bool) {
	t.Helper()
	wants := map[string]caseDWant{}
	for _, w := range caseDWants {
		wants[w.key] = w
	}
	type seen struct {
		first, last uint64
		commits     int
	}
	got := map[string]*seen{}
	for i, c := range res.Commits {
		var texts []string
		for _, l := range c.Labels {
			w, ok := wants[l.Key]
			if !ok {
				t.Errorf("提交 %d 有預期外的 key %q（第 %d 步）", i, l.Key, l.Steps)
				continue
			}
			if !exactOnly || !w.composed {
				texts = append(texts, w.text)
			}
		}
		for _, l := range c.Labels {
			s := got[l.Key]
			if s == nil {
				s = &seen{first: l.Steps}
				got[l.Key] = s
			}
			s.last = l.Steps
		}
		keys := map[string]bool{}
		for _, l := range c.Labels {
			keys[l.Key] = true
		}
		for k := range keys {
			got[k].commits++
		}
		for _, s := range res.Stamps[i] {
			if s.Stamps != 2*len(texts) || strings.Join(s.Texts, "|") != strings.Join(texts, "|") {
				t.Errorf("提交 %d 的 %d 倍 stamp %d 個、文字 %q，要 %d 個 %q", i, s.Scale, s.Stamps, s.Texts, 2*len(texts), texts)
			}
		}
		if len(c.Ink) != len(c.Labels) {
			t.Errorf("提交 %d 的核對 %d 筆，標籤 %d 筆", i, len(c.Ink), len(c.Labels))
		}
		for _, ink := range c.Ink {
			if !ink.OK() {
				t.Errorf("提交 %d 墨跡框核對未通過：%s", i, ink)
			}
		}
	}
	for _, w := range caseDWants {
		s := got[w.key]
		if s == nil {
			t.Errorf("%q 沒有提交", w.key)
			continue
		}
		if s.first != w.first || s.last != w.last || s.commits != w.commits {
			t.Errorf("%q：第一筆 %d、最後一筆 %d、提交 %d，要 %d、%d、%d", w.key, s.first, s.last, s.commits, w.first, w.last, w.commits)
		}
	}
	if exactOnly {
		for _, sc := range []int{2, 3} {
			for _, m := range res.Overlays[sc].Misses {
				if w, ok := wants[m.Key]; !ok || !w.composed || m.Reason != MissTemplate {
					t.Errorf("%d 倍 miss %+v，只該有組合標籤的 template miss", sc, m)
				}
			}
		}
		return
	}
	if len(res.Shots) != 2 {
		t.Fatalf("shot %d 個，要 2", len(res.Shots))
	}
	for _, s := range res.Shots {
		for _, sc := range []int{2, 3} {
			if cc := s.Check[sc]; cc.Outside != 0 || cc.Diffs == 0 {
				t.Errorf("shot 第 %d 步 %d 倍合成圖差異 %d、框外 %d", s.Steps, sc, cc.Diffs, cc.Outside)
			}
		}
	}
	for _, sc := range []int{2, 3} {
		if len(res.Overlays[sc].Misses) != 0 || len(res.Overlays[sc].Skips) != 0 {
			t.Errorf("%d 倍 misses %+v skips %+v，要都沒有", sc, res.Overlays[sc].Misses, res.Overlays[sc].Skips)
		}
		if cc := res.CompositeCheck[sc]; cc.Outside != 0 {
			t.Errorf("停止時 %d 倍合成圖框外差異 %d", sc, cc.Outside)
		}
	}
}

func TestComposedLabelCaseD(t *testing.T) {
	for _, e := range []string{scRootEnv, scStateDirEnv, scFontEnv, scCatalogEnv} {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定：002 §3.2 案例 D 需要設定樹、狀態檔、字型與正式 catalog")
		}
	}
	t.Run("D", func(t *testing.T) {
		on := runCaseD(t, true, false)
		checkCaseD(t, on, false)
		checkNoSig(t, on) // 005 §4.4
		logMeasure(t, "D", on)
		off := runCaseD(t, false, false)
		a, b := on.Digest, off.Digest
		t.Logf("開：Memory %x CPU %x Indexed %x Palette %x", a.MemorySHA256, a.CPUSHA256, a.IndexedSHA256, a.PaletteSHA256)
		t.Logf("關：Memory %x CPU %x Indexed %x Palette %x", b.MemorySHA256, b.CPUSHA256, b.IndexedSHA256, b.PaletteSHA256)
		if a.Steps != b.Steps || a.MemorySHA256 != b.MemorySHA256 || a.CPUSHA256 != b.CPUSHA256 ||
			a.IndexedSHA256 != b.IndexedSHA256 || a.PaletteSHA256 != b.PaletteSHA256 || a.ExtSHA256 != b.ExtSHA256 {
			t.Fatal("觀察器與 stamp 開、關的 Digest 不同")
		}
	})
	t.Run("DExactOnly", func(t *testing.T) {
		res := runCaseD(t, true, true)
		checkCaseD(t, res, true)
	})
}
