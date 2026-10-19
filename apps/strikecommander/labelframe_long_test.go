package strikecommander

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// strike_commander 規格 005 §4.2（案例 F 的補充斷言，見 saveload_long_test.go）、§4.3 案例 H 與 §4.4 的量測。
//
// 案例 H 是長測試（約 4.6 億步，開、關各一次）。只在下列環境變數都設定時執行，否則 t.Skip：
//   - DOSGOLEM_SC_ROOT、DOSGOLEM_SC_STATE_DIR：同 trigger_long_test.go（狀態檔 p486/s600.state）
//   - DOSGOLEM_SC_FONT：font-test（正式字型的超集）
//   - DOSGOLEM_SC_CATALOG：正式 catalog text/opttest_labels.zh-TW.tsv
//
// DOSGOLEM_SC_OUT 有設定時，把每個 shot 寫到 <OUT>/case-h（同 saveload_long_test.go 的格式）。
//
// 腳本 testdata/case_h.script 取自 strike_commander workplace/i016/h.script（RE 016 之後的探針，005 §2.1）。
// 舊規則（001 §2.5 計步重置）的對照數字取自同一探針的 h.log：a30、a40 的 2 倍合成圖差異 0、3 倍 0。

const (
	caseHState = "p486/s600.state"
	caseHSHA   = "2774488e041642b91a4feadc38b2c6b2045dd06b0046071c2c39226fd5901577"
	caseHKey   = "Fly mission"
)

// checkNoSig 斷言 log 沒有 "sig" 紀錄（005 §4.4：案例 A 到 G）。
func checkNoSig(t *testing.T, res *ReceiptResult) {
	t.Helper()
	for _, e := range res.Log {
		if e.Kind == "sig" {
			t.Errorf("log 有 sig 紀錄：第 %d 步 %s", e.Steps, e.Detail)
		}
	}
}

// logMeasure 印出 005 §4.4 的量測。
func logMeasure(t *testing.T, name string, res *ReceiptResult) {
	t.Helper()
	m := res.Measure
	for _, sc := range []int{2, 3} {
		for _, s := range []struct {
			tag string
			mc  *MeasureCount
		}{{"cur", m.Cur[sc]}, {"old", m.Old[sc]}} {
			if s.mc == nil {
				continue
			}
			t.Logf("量測 %s %d 倍 %s：可見非空回掃 %d，不符 %d，分類 (1) %d (2) %d (3) %d",
				name, sc, s.tag, m.Frames, s.mc.Mismatch, s.mc.Class[0], s.mc.Class[1], s.mc.Class[2])
			for _, e := range s.mc.Others {
				t.Logf("  量測 %s %d 倍 %s (3)：第 %d 步 %v", name, sc, s.tag, e.Steps, e.Keys)
			}
		}
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// coverDiffs 回 shot 在 key 的遮蓋 stamp 矩形（比對框）內合成圖與原版放大圖的差異像素數；沒有遮蓋 stamp 回 -1。
func coverDiffs(s ShotResult, sc int, key string) int {
	r, ok := s.Covers[sc][key+".cover"]
	if !ok {
		return -1
	}
	return diffsIn(s.Composites[sc], s.Plain[sc], sc, r[0], r[1], r[2], r[3])
}

// checkShotShown 斷言 shot 時 key 在可見集合、各倍率 Hidden 為空、比對框內合成圖與原版放大圖不同（照畫）。
func checkShotShown(t *testing.T, s ShotResult, key string) {
	t.Helper()
	if !contains(s.Visible, key) {
		t.Errorf("shot %s：Visible %q 不含 %q", s.Name, s.Visible, key)
		return
	}
	for _, sc := range []int{2, 3} {
		n := coverDiffs(s, sc, key)
		t.Logf("shot %s %d 倍：Visible %q Hidden %q，%q 比對框差 %d", s.Name, sc, s.Visible, s.Hidden[sc], key, n)
		if len(s.Hidden[sc]) != 0 {
			t.Errorf("shot %s %d 倍：Hidden %q，要空", s.Name, sc, s.Hidden[sc])
		}
		if n <= 0 {
			t.Errorf("shot %s %d 倍：%q 比對框內合成圖與原版放大圖差 %d，要有差異", s.Name, sc, key, n)
		}
	}
}

// checkCaseF005 是 005 §4.2 對案例 F 的斷言。
func checkCaseF005(t *testing.T, res *ReceiptResult) {
	t.Helper()
	shown := map[string]string{"open": "Load Game", "pre": "Load Game", "hover": "Load Game", "pressed": "Load Game", "dragout": "Load Game", "save": "Save Game"}
	for _, s := range res.Shots {
		if key, ok := shown[s.Name]; ok {
			checkShotShown(t, s, key)
		}
		if s.Name == "released" || s.Name == "closed" {
			t.Logf("shot %s：Visible %q Hidden %v", s.Name, s.Visible, s.Hidden)
			if contains(s.Visible, "Load Game") {
				t.Errorf("shot %s：Visible %q 不得含 Load Game", s.Name, s.Visible)
			}
		}
	}
	checkNoSig(t, res)
}

func runCaseH(t *testing.T, overlay bool) *ReceiptResult {
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
	raw, err := os.ReadFile(filepath.Join(os.Getenv(scStateDirEnv), caseHState))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(caseHSHA)); err != nil {
		t.Fatal(err)
	}
	font, err := LoadLabelFont(os.Getenv(scFontEnv), "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	cat, err := LoadCatalog(os.Getenv(scCatalogEnv))
	if err != nil {
		t.Fatal(err)
	}
	sb, err := os.ReadFile("testdata/case_h.script")
	if err != nil {
		t.Fatal(err)
	}
	script, err := ParseScript(sb, 0)
	if err != nil {
		t.Fatal(err)
	}
	res, err := RunReceipt(ReceiptConfig{
		SaveRoot: root, State: raw, StateSHA256: want, RebaseFrom: []string{"/scratch"},
		StopSteps: script.Stop, Scales: []int{2, 3}, Catalog: cat, Font: font, Overlay: overlay, CursorY: -1,
		Inputs: script.Inputs, Shots: script.Shots,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("BootReceipt Steps=%d；停止 %d", res.Boot.Steps, res.Digest.Steps)
	return res
}

func checkCaseH(t *testing.T, res *ReceiptResult) {
	t.Helper()
	n := len(res.Commits)
	if n == 0 {
		t.Fatal("沒有提交")
	}
	t.Logf("提交 %d 次；最後一次第 %d 步，標籤 %q", n, res.Commits[n-1].BlitReturn, keys(res.Commits[n-1].Labels))
	var sigSteps []uint64
	for _, e := range res.Log {
		if e.Kind == "sig" || e.Kind == "base" || e.Kind == "art" {
			t.Logf("紀錄 第 %d 步 %s：%s", e.Steps, e.Kind, e.Detail)
		}
		if e.Kind == "sig" {
			sigSteps = append(sigSteps, e.Steps)
		}
	}
	t.Logf("sig 紀錄步數 %v", sigSteps)
	for _, s := range res.Shots {
		for _, sc := range []int{2, 3} {
			t.Logf("shot %s（第 %d 步）%d 倍：Visible %q Hidden %q，全畫面差 %d、框外 %d，%q 比對框差 %d",
				s.Name, s.Steps, sc, s.Visible, s.Hidden[sc], s.Check[sc].Diffs, s.Check[sc].Outside, caseHKey, coverDiffs(s, sc, caseHKey))
			if s.Check[sc].Outside != 0 {
				t.Errorf("shot %s %d 倍：遮蓋框外差 %d", s.Name, sc, s.Check[sc].Outside)
			}
		}
		switch s.Name {
		case "hover", "a25":
			checkShotShown(t, s, caseHKey)
		case "a30", "a40":
			if contains(s.Visible, caseHKey) {
				t.Logf("shot %s：可見集合仍含 %q（簽章層未清），新規則照畫；舊規則（h.log）此處合成圖差 0", s.Name, caseHKey)
				checkShotShown(t, s, caseHKey)
			} else {
				t.Logf("shot %s：可見集合 %q（簽章層已清）", s.Name, s.Visible)
				if len(s.Visible) != 0 {
					t.Errorf("shot %s：Visible %q，要含 %q 或為空", s.Name, s.Visible, caseHKey)
				}
			}
		case "a60", "a100", "a200", "a300", "a450":
			for _, sc := range []int{2, 3} {
				var drawn []string
				for _, k := range s.Visible {
					if !contains(s.Hidden[sc], k) {
						drawn = append(drawn, k)
					}
				}
				if len(drawn) != 0 || s.Check[sc].Diffs != 0 {
					t.Errorf("shot %s %d 倍：照畫 %q、全畫面差 %d，要都沒有", s.Name, sc, drawn, s.Check[sc].Diffs)
				}
			}
		}
	}
	if dir := os.Getenv(scOutEnv); dir != "" {
		out := filepath.Join(dir, "case-h")
		for i := range res.Shots {
			if err := writeSaveLoadShot(out, &res.Shots[i]); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("shot 輸出寫到 %s", out)
	}
}

func TestLabelFrameCaseH(t *testing.T) {
	for _, e := range []string{scRootEnv, scStateDirEnv, scFontEnv, scCatalogEnv} {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定：005 §4.3 案例 H 需要設定樹、狀態檔、字型與正式 catalog")
		}
	}
	on := runCaseH(t, true)
	checkCaseH(t, on)
	writeStop(t, "case-h", on) // 006 §3.3
	logMeasure(t, "H", on)
	off := runCaseH(t, false)
	a, b := on.Digest, off.Digest
	t.Logf("開：Memory %x CPU %x Indexed %x Palette %x", a.MemorySHA256, a.CPUSHA256, a.IndexedSHA256, a.PaletteSHA256)
	t.Logf("關：Memory %x CPU %x Indexed %x Palette %x", b.MemorySHA256, b.CPUSHA256, b.IndexedSHA256, b.PaletteSHA256)
	if a.Steps != b.Steps || a.MemorySHA256 != b.MemorySHA256 || a.CPUSHA256 != b.CPUSHA256 ||
		a.IndexedSHA256 != b.IndexedSHA256 || a.PaletteSHA256 != b.PaletteSHA256 || a.ExtSHA256 != b.ExtSHA256 {
		t.Fatal("觀察器與 stamp 開、關的 Digest 不同")
	}
}
