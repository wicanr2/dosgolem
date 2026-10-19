package strikecommander

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// strike_commander 規格 004 §4.2 案例 F（營房讀檔、存檔）與 §4.3 案例 G（帳篷存檔），
// 以及規格 007 §4.2 到 §4.4 的案例 I（營房離開確認）、J（營房覆寫確認）、K（帳篷離開確認）；
// I、J、K 的補充斷言在 saveload_confirm_long_test.go 的 slExtra。
//
// 長測試（F 約 3.3 億步、G 約 2.6 億步、J 約 5.2 億步，各開、關一次）。只在下列環境變數都設定時執行，否則 t.Skip：
//   - DOSGOLEM_SC_ROOT、DOSGOLEM_SC_STATE_DIR：同 trigger_long_test.go
//   - DOSGOLEM_SC_FONT：正式字型（007 起含確認框的 7 個新字）
//   - DOSGOLEM_SC_CATALOG：正式 catalog text/opttest_labels.zh-TW.tsv
//   - DOSGOLEM_SC_GFX_CATALOG：正式圖形字 catalog text/menu_gfx.zh-TW.tsv
//   - DOSGOLEM_SC_I015：strike_commander workplace/i015（SAVELOAD.SHP，游標欄核對用）
//
// DOSGOLEM_SC_OUT 有設定時，把每個 shot 的 2、3 倍合成圖、原版放大圖、原版色號畫格與色盤寫到
// <OUT>/case-f、<OUT>/case-g（007 另有 case-i、case-j、case-k），供主代理做 §4.2 的獨立比對。
//
// 期望值取自盤點工具的 strike_commander workplace/i015/l7/ip.tsv、l8/ip.tsv、t1/ip.tsv，以及
// workplace/i017/q2/ip.tsv、k5/ip.tsv、tq/ip.tsv（同一輸入的步數），獨立於本規格的實作。

type slWant struct {
	steps uint64 // 0：不斷言步數（只斷言順序）
	g, k  int
}

type slCase struct {
	name, state, sha, script string
	want                     []slWant
	closed                   []string // 對話框不在畫面上的 shot：合成圖要等於原版放大圖
	open                     []string // 對話框在畫面上的 shot：合成圖要有差異
}

var slCases = map[string]slCase{
	"F": {"F", "p485/s449.state", "7dff6dd268ef38d04086aeaa3ba22d5e2b23c5dc953de97dfe73fc351b1e47a0", "testdata/case_f.script",
		[]slWant{
			// 讀檔開啟（l7/ip.tsv）：k0、k4、k6、k8、k1；k10、k12、k14 與裁切重畫不產生圖面。
			{4735308401, 0, 0}, {4735340008, 0, 4}, {4735341907, 0, 6}, {4735344082, 0, 8}, {4735372387, 0, 1},
			// 按下 CANCEL、在外放開（l8/ip.tsv）。
			{4755001638, 0, 7}, {4775001059, 0, 6},
			// 存檔開啟：k0、k2、k6、k8（s2/ip.tsv 的順序；步數不同，因為接在讀檔之後）。
			{0, 0, 0}, {0, 0, 2}, {0, 0, 6}, {0, 0, 8},
		},
		[]string{"closed"}, []string{"open", "pre", "hover", "pressed", "dragout", "save"}},
	"G": {"G", "p539/s500b.state", "0fbe712170ff91b9d83a5346fcaac9f98a5f09ef10b6295d4e6914b51f0d4ee6", "testdata/case_g.script",
		[]slWant{
			// t1/ip.tsv：e 21、23、27、29（k 0、2、6、8）。e 31、33、35 與裁切重畫不產生圖面。
			{5255492361, 1, 0}, {5255524794, 1, 2}, {5255526654, 1, 6}, {5255528829, 1, 8},
		},
		[]string{"tent"}, []string{"open", "tsave"}},
	// 007 §4.2 案例 I（q2/ip.tsv）：開啟 k0、k2、k6、k8 在 ip 範圍外（只斷言順序）；QUIT 按放 k9、k8；確認框 k15、k17、k19；
	// NO 按放 k20、k19；重畫 k0、k2、k6、k8（k10、k12、k14 不產生圖面）；SAVE 按下 k3（強推論，沒有原版收據）。
	"I": {"I", "p485/s449.state", "7dff6dd268ef38d04086aeaa3ba22d5e2b23c5dc953de97dfe73fc351b1e47a0", "testdata/case_i.script",
		[]slWant{
			{0, 0, 0}, {0, 0, 2}, {0, 0, 6}, {0, 0, 8},
			{4750001396, 0, 9}, {4760001044, 0, 8},
			{4760010772, 0, 15}, {4760021615, 0, 17}, {4760023288, 0, 19},
			{4800001549, 0, 20}, {4810001341, 0, 19},
			{4810005877, 0, 0}, {4810037484, 0, 2}, {4810039344, 0, 6}, {4810041519, 0, 8},
			{0, 0, 3},
		},
		nil, []string{"confirm", "nopressed", "no10", "savepressed"}},
	// 007 §4.3 案例 J（k5/ip.tsv，ip 從 4895000002 起）：第一次開啟 k0、k2、k6、k8 只斷言順序；打字存檔後關閉。
	"J": {"J", "p485/s449.state", "7dff6dd268ef38d04086aeaa3ba22d5e2b23c5dc953de97dfe73fc351b1e47a0", "testdata/case_j.script",
		[]slWant{
			{0, 0, 0}, {0, 0, 2}, {0, 0, 6}, {0, 0, 8},
			{4895642986, 0, 0}, {4895674593, 0, 2}, {4895676453, 0, 6}, {4895678628, 0, 8},
			{4913010230, 0, 15}, {4913021073, 0, 17}, {4913022746, 0, 19}, {4913025317, 0, 16},
			{4930001443, 0, 20}, {4940000953, 0, 19},
			{4940005490, 0, 0}, {4940037097, 0, 2}, {4940038957, 0, 6}, {4940041132, 0, 8},
			{4963010238, 0, 15}, {4963021081, 0, 17}, {4963022754, 0, 19}, {4963025325, 0, 16},
			{4985001443, 0, 18}, {4995001293, 0, 17},
			{4995005984, 0, 0}, {4995037591, 0, 2}, {4995039451, 0, 6}, {4995041626, 0, 8},
		},
		[]string{"yes10"}, []string{"bs", "c", "ow", "nopressed", "no10", "ow2", "yespressed"}},
	// 007 §4.4 案例 K（帳篷，tq/ip.tsv 從 5261000002 起；開啟的 e 21、23、27、29 步數取自 t1/ip.tsv，同狀態、同輸入）。
	"K": {"K", "p539/s500b.state", "0fbe712170ff91b9d83a5346fcaac9f98a5f09ef10b6295d4e6914b51f0d4ee6", "testdata/case_k.script",
		[]slWant{
			{5255492361, 1, 0}, {5255524794, 1, 2}, {5255526654, 1, 6}, {5255528829, 1, 8},
			{5265001517, 1, 9}, {5270001318, 1, 8},
			{5270011080, 1, 15}, {5270021979, 1, 17}, {5270023652, 1, 19},
			{5290001617, 1, 20}, {5300000963, 1, 19},
			{5300005513, 1, 0}, {5300037946, 1, 2}, {5300039806, 1, 6}, {5300041981, 1, 8},
		},
		nil, []string{"tsave", "tq", "tnopressed", "tno"}},
}

func runSaveLoadCase(t *testing.T, c slCase, overlay bool) *ReceiptResult {
	t.Helper()
	return runSaveLoadCaseRoot(t, c, overlay, scRootEnv)
}

// preserveMtimes 把 dst 樹中每個一般檔案的修改時間設成 src 樹同一路徑的修改時間。
func preserveMtimes(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil || !info.Mode().IsRegular() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		return os.Chtimes(filepath.Join(dst, rel), info.ModTime(), info.ModTime())
	})
}

// runSaveLoadCaseRoot 同 runSaveLoadCase，SaveRoot 由環境變數 rootEnv 指的樹複製（008 §4.2：每案例指定根目錄）。
func runSaveLoadCaseRoot(t *testing.T, c slCase, overlay bool, rootEnv string) *ReceiptResult {
	t.Helper()
	root := t.TempDir()
	if os.Getenv(rootEnv) == "" {
		t.Fatalf("%s 未設定", rootEnv)
	}
	if err := CopyTree(os.Getenv(rootEnv), root); err != nil {
		t.Fatal(err)
	}
	// 修改時間照來源樹：DOS findfirst 回傳檔案時間（internal/dos/find.go 的 dosDateTime），遊戲找 *.sav 時
	// 會把它讀進記憶體。CopyTree 的複本時間是複製當下，開、關兩次執行不同，Digest 的 Memory 就不同（008 案例 M1）。
	if err := preserveMtimes(os.Getenv(rootEnv), root); err != nil {
		t.Fatal(err)
	}
	t.Logf("SaveRoot 由 %s 複製（保留修改時間）", rootEnv)
	cfg, err := os.ReadFile(filepath.Join(root, "SC.CFG"))
	if err != nil {
		t.Fatal(err)
	}
	if s := sha256.Sum256(cfg); hex.EncodeToString(s[:]) != scCFGSHA256 {
		t.Fatalf("SC.CFG SHA-256 %x，要 %s", s, scCFGSHA256)
	}
	raw, err := os.ReadFile(filepath.Join(os.Getenv(scStateDirEnv), c.state))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(c.sha)); err != nil {
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
	cat, err := LoadCatalog(os.Getenv(scCatalogEnv))
	if err != nil {
		t.Fatal(err)
	}
	gfxBytes, err := os.ReadFile(os.Getenv(scGfxCatalogEnv))
	if err != nil {
		t.Fatal(err)
	}
	gfx, err := ParseCatalog(gfxBytes)
	if err != nil {
		t.Fatal(err)
	}
	fs, gs := sha256.Sum256(fontBytes), sha256.Sum256(gfxBytes)
	t.Logf("字型 SHA-256 %x；圖形字 catalog SHA-256 %x", fs, gs)
	sb, err := os.ReadFile(c.script)
	if err != nil {
		t.Fatal(err)
	}
	script, err := ParseScript(sb, 0)
	if err != nil {
		t.Fatal(err)
	}
	res, err := RunReceipt(ReceiptConfig{
		SaveRoot: root, State: raw, StateSHA256: want, RebaseFrom: []string{"/scratch"},
		StopSteps: script.Stop, Scales: []int{2, 3}, Catalog: cat, GfxCatalog: gfx, Font: font, Overlay: overlay, CursorY: -1,
		Inputs: script.Inputs, Shots: script.Shots,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("BootReceipt Steps=%d；停止 %d", res.Boot.Steps, res.Digest.Steps)
	if !overlay {
		return res
	}
	for _, a := range res.Arts {
		t.Logf("繪圖事件 第 %d 步：組 %d k %d (%d,%d) Order %d", a.Steps, a.G, a.K, a.X, a.Y, a.Order)
	}
	t.Logf("圖面清除（清除前有圖面）：%v", res.ArtClears)
	for _, e := range res.Log {
		if e.Kind == "art" || e.Kind == "base" {
			t.Logf("紀錄 第 %d 步 %s：%s", e.Steps, e.Kind, e.Detail)
		}
	}
	for sc, ov := range res.Overlays {
		var am, as []string
		for _, m := range ov.Misses {
			if isArtKey(m.Key) {
				am = append(am, fmt.Sprintf("%+v", m))
			}
		}
		for _, s := range ov.Skips {
			if isArtKey(s.Key) || strings.HasPrefix(s.Key, "saveload/") {
				as = append(as, fmt.Sprintf("%+v", s))
			}
		}
		t.Logf("%d 倍 對話框 misses %v skips %v", sc, am, as)
		if len(am) != 0 || len(as) != 0 {
			t.Errorf("%d 倍：對話框有 miss 或 skip", sc)
		}
	}
	return res
}

func isArtKey(k string) bool {
	for _, a := range ArtCatalogKeys() {
		if a == k {
			return true
		}
	}
	return false
}

// checkSaveLoadCase 斷言繪圖事件、每個 shot 的差異範圍與游標欄，並輸出 shot。
func checkSaveLoadCase(t *testing.T, c slCase, res *ReceiptResult) {
	t.Helper()
	if len(res.Arts) != len(c.want) {
		t.Errorf("繪圖事件 %d 筆，要 %d", len(res.Arts), len(c.want))
	}
	for i, w := range c.want {
		if i >= len(res.Arts) {
			break
		}
		a := res.Arts[i]
		if a.G != w.g || a.K != w.k || (w.steps != 0 && a.Steps != w.steps) {
			t.Errorf("第 %d 筆事件：第 %d 步 組 %d k %d，要第 %d 步 組 %d k %d", i, a.Steps, a.G, a.K, w.steps, w.g, w.k)
		}
	}
	file := loadSHP(t)
	closed, open := map[string]bool{}, map[string]bool{}
	for _, n := range c.closed {
		closed[n] = true
	}
	for _, n := range c.open {
		open[n] = true
	}
	for _, s := range res.Shots {
		for _, sc := range []int{2, 3} {
			cc := s.Check[sc]
			t.Logf("shot %s（第 %d 步）%d 倍：差異 %d、框外 %d，圖面 %v", s.Name, s.Steps, sc, cc.Diffs, cc.Outside, s.ArtKeys[sc])
			if cc.Outside != 0 {
				t.Errorf("shot %s %d 倍：字帶外有 %d 個差異像素", s.Name, sc, cc.Outside)
			}
			// 對話框不在畫面上：沒有圖面，對話框矩形 (80,30)-(261,160)（RE 015 §5）內與原版放大圖相同。
			// 矩形外可能有 001 的熱點標籤 stamp（帳篷的底部標籤），不在本規格範圍。
			if closed[s.Name] {
				if n := diffsIn(s.Composites[sc], s.Plain[sc], sc, 80, 30, 262, 161); n != 0 || len(s.ArtKeys[sc]) != 0 {
					t.Errorf("shot %s %d 倍：對話框不在畫面上，圖面 %v，對話框矩形內差 %d 像素", s.Name, sc, s.ArtKeys[sc], n)
				}
			}
			if open[s.Name] && cc.Diffs == 0 {
				t.Errorf("shot %s %d 倍：對話框在畫面上，合成圖卻沒有差異", s.Name, sc)
			}
			checkCursorColumns(t, file, res, s, sc)
		}
	}
	if dir := os.Getenv(scOutEnv); dir != "" {
		out := filepath.Join(dir, "case-"+strings.ToLower(c.name))
		for i := range res.Shots {
			if err := writeSaveLoadShot(out, &res.Shots[i]); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("shot 輸出寫到 %s", out)
	}
}

// checkCursorColumns：shot 時每列圖面，畫格與原版基準不同的欄（游標所在欄），合成圖要等於原版放大圖。
// 基準由 SAVELOAD.SHP 的解碼值與 shot 前最後一筆同 Key 的事件（組、k）算出。
func checkCursorColumns(t *testing.T, file []byte, res *ReceiptResult, s ShotResult, sc int) {
	t.Helper()
	W := 320 * sc
	// 每個 Key 取 shot 前最後一筆事件。
	evs := map[string]*ArtEvent{}
	for _, key := range s.ArtKeys[sc] {
		for i := range res.Arts {
			if res.Arts[i].Steps < s.Steps && artKey(res.Arts[i]) == key {
				evs[key] = &res.Arts[i]
			}
		}
	}
	// later 回 (x, y) 是否落在 Order 比 ev 大的另一列圖面的字帶內（例如 k0 標題被 k1 蓋住，004 §3.3）。
	later := func(ev *ArtEvent, x, y int) bool {
		for _, o := range evs {
			if o.Order <= ev.Order {
				continue
			}
			oe := tElemFor(t, o.K)
			if x >= o.X+oe.band[0] && x <= o.X+oe.band[2] && y >= o.Y+oe.band[1] && y <= o.Y+oe.band[3] {
				return true
			}
		}
		return false
	}
	for _, key := range s.ArtKeys[sc] {
		ev := evs[key]
		if ev == nil {
			t.Errorf("shot %s：圖面 %s 找不到事件", s.Name, key)
			continue
		}
		el := tElemFor(t, ev.K)
		ent := mustDecode(t, file, ev.G*21+ev.K)
		bx, by := ev.X+el.band[0], ev.Y+el.band[1]
		bw, bh := el.band[2]-el.band[0]+1, el.band[3]-el.band[1]+1
		cursorCols := 0
		for col := 0; col < bw; col++ {
			differs := false
			for y := 0; y < bh; y++ {
				if s.Indexed[(by+y)*320+bx+col] != ent.Pix[(el.band[1]+y)*ent.W+el.band[0]+col] {
					differs = true
					break
				}
			}
			if !differs {
				continue
			}
			cursorCols++
			for y := by * sc; y < (by+bh)*sc; y++ {
				for x := (bx + col) * sc; x < (bx+col+1)*sc; x++ {
					i := 4 * (y*W + x)
					if later(ev, x/sc, y/sc) {
						continue
					}
					if string(s.Composites[sc][i:i+4]) != string(s.Plain[sc][i:i+4]) {
						t.Errorf("shot %s %d 倍：圖面 %s 第 %d 欄與基準不同，合成圖卻不等於原版", s.Name, sc, key, col)
						y = 1 << 30
						break
					}
				}
			}
		}
		if cursorCols != 0 && sc == 2 {
			t.Logf("shot %s：圖面 %s 與基準不同 %d 欄（停畫）", s.Name, key, cursorCols)
		}
	}
}

// writeSaveLoadShot 寫 shot 的收據檔（006 §3.3 第 2 條），前綴 shot-<步數>-<名稱>（ShotPrefix）。
func writeSaveLoadShot(dir string, s *ShotResult) error {
	return WriteShotFiles(dir, ShotPrefix(s), s, []int{2, 3})
}

// checkTentColors 依 004 §3.5 的表檢查帳篷（21 組）圖面的字色與高光，用執行期事件的解碼陣列。
func checkTentColors(t *testing.T, res *ReceiptResult) {
	t.Helper()
	want := map[int][2]uint8{0: {55, 209}, 2: {31, 87}, 6: {31, 209}, 8: {31, 87}}
	for sc, ov := range res.Overlays {
		if len(ov.arts) != 4 {
			t.Errorf("%d 倍：停止時圖面 %d 列，要 4", sc, len(ov.arts))
		}
		for _, e := range ov.arts {
			c, h, hasH := dialogColors(e.ev)
			w := want[e.ev.K]
			if e.ev.G != 1 || c != w[0] || h != w[1] || !hasH {
				t.Errorf("%d 倍 %s：組 %d c %d h %d，要組 1 c %d h %d", sc, e.rect.Key, e.ev.G, c, h, w[0], w[1])
			}
		}
	}
}

func TestSaveLoadCase(t *testing.T) {
	for _, e := range []string{scRootEnv, scStateDirEnv, scFontEnv, scCatalogEnv, scGfxCatalogEnv, i015Env} {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定：004 §4.2、§4.3 需要設定樹、狀態檔、字型、兩份正式 catalog 與 i015")
		}
	}
	for _, name := range []string{"F", "G", "I", "J", "K"} {
		c := slCases[name]
		t.Run(name, func(t *testing.T) {
			t0 := time.Now()
			on := runSaveLoadCase(t, c, true)
			t.Logf("開：%d 步，耗時 %v", on.Digest.Steps-on.Boot.Steps, time.Since(t0))
			checkSaveLoadCase(t, c, on)
			writeStop(t, "case-"+strings.ToLower(name), on) // 006 §3.3
			if name == "G" || name == "K" {
				checkTentColors(t, on)
				checkNoSig(t, on) // 005 §4.4
			}
			if x := slExtra[name]; x != nil {
				x(t, on) // 007 §4.2 到 §4.4
			}
			if name == "F" {
				checkCaseF005(t, on) // 005 §4.2（含 sig 紀錄檢查）
			}
			for _, s := range on.Shots {
				t.Logf("005：shot %s Visible %q Hidden %v", s.Name, s.Visible, s.Hidden)
			}
			logMeasure(t, name, on)
			t1 := time.Now()
			off := runSaveLoadCase(t, c, false)
			t.Logf("關：%d 步，耗時 %v", off.Digest.Steps-off.Boot.Steps, time.Since(t1))
			a, b := on.Digest, off.Digest
			t.Logf("開：Memory %x CPU %x Indexed %x Palette %x", a.MemorySHA256, a.CPUSHA256, a.IndexedSHA256, a.PaletteSHA256)
			t.Logf("關：Memory %x CPU %x Indexed %x Palette %x", b.MemorySHA256, b.CPUSHA256, b.IndexedSHA256, b.PaletteSHA256)
			if a.Steps != b.Steps || a.MemorySHA256 != b.MemorySHA256 || a.CPUSHA256 != b.CPUSHA256 ||
				a.IndexedSHA256 != b.IndexedSHA256 || a.PaletteSHA256 != b.PaletteSHA256 || a.ExtSHA256 != b.ExtSHA256 {
				t.Fatal("圖面開、關的 Digest 不同")
			}
		})
	}
}

// diffsIn 回 [x0, x1) × [y0, y1)（1×）內合成圖與原版放大圖不同的輸出像素數。
func diffsIn(comp, plain []byte, sc, x0, y0, x1, y1 int) int {
	n, W := 0, 320*sc
	for y := y0 * sc; y < y1*sc; y++ {
		for x := x0 * sc; x < x1*sc; x++ {
			i := 4 * (y*W + x)
			if string(comp[i:i+4]) != string(plain[i:i+4]) {
				n++
			}
		}
	}
	return n
}
