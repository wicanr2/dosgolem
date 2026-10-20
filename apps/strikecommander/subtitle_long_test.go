package strikecommander

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// strike_commander 規格 010 §4.2 的觸發案例（長測試）。只在下列環境變數都設定時執行，否則 t.Skip：
//   - DOSGOLEM_SC_ROOT、DOSGOLEM_SC_STATE_DIR（workplace/scratch）：設定樹與狀態檔；
//   - DOSGOLEM_SC_SUB_FONT：含測試字的字型（010 §4.2、001 §3.3 的做法）；
//   - DOSGOLEM_SC_SPEECH_INDEX、DOSGOLEM_SC_SPEECH_CATALOG：結構表與譯文（預設是測試 catalog testdata/speech.test.tsv）；
//   - DOSGOLEM_SC_I022：workplace/i022（T10、H12 的期望取自 timing-t10.tsv、timing-h12.tsv，獨立於本實作）。
//
// DOSGOLEM_SC_SUBCASES 以逗號列出要跑的案例（預設全部）；DOSGOLEM_SC_SUB_AB=1 時另跑同狀態 A/B 的甲（Overlay 關）、乙（Subtitles 為 nil）。
// DOSGOLEM_SC_OUT 有設定時把收據寫到 <OUT>/sub-<案例>（010 §3.11）。
// 「輸入重現的前置步驟」（§4.2 第 1、2 步）：每次執行都記下全部 SB16 DMA 事件，與 probe 收據比 start、complete、pause、resume、cancel。

const (
	scSubFontEnv       = "DOSGOLEM_SC_SUB_FONT"
	scSpeechCatalogEnv = "DOSGOLEM_SC_SPEECH_CATALOG"
	scSubCasesEnv      = "DOSGOLEM_SC_SUBCASES"
	scSubABEnv         = "DOSGOLEM_SC_SUB_AB"
	s500SHA            = "1e98714470a48b06ff6903dab39ec9a341b4f9925bb7cab3ce34ad3a1bd772e8"
	s510SHA            = "de7a367f93d745b6c83d85715af15bc51c82e7d44bd3cbf17fe34af809b7126b"
	s500bSHA           = "0fbe712170ff91b9d83a5346fcaac9f98a5f09ef10b6295d4e6914b51f0d4ee6"
	s390SHA            = "9da24923e55771610d3738dce58f46f1971dc5168b4e97147be4f4de0a013d57"
)

// wantSub 是一個期望字幕條。end 為 0 時不斷言訖點（播完時訖點等於 lastDue）。
type wantSub struct {
	key            string
	start, lastDue uint64
	parts          int
	end            uint64
	reason         string
}

type subLongCase struct {
	state, sha, script string
	probe              string // probe 收據（相對 STATE_DIR）；空為沒有
	want               func(t *testing.T) []wantSub
	shots              map[string]string // shot 名稱 → 可見的 key（空字串為不可見）
	blockedAfter       uint64            // 「字幕區非黑」只允許在這一步之後（0：最後一個字幕條的 last_due 之後）
}

func played(key string, start, due uint64) wantSub {
	return wantSub{key, start, due, 1, due, ReasonDone}
}

var c11Want = []wantSub{
	played("conv/011/00", 5032689784, 5063722744), played("conv/011/01", 5066534942, 5085408619),
	played("conv/011/02", 5087511261, 5112063300), played("conv/011/03", 5115122728, 5143010075),
	played("conv/011/04", 5145739718, 5187147572), played("conv/011/05", 5189368343, 5201637067),
	played("conv/011/06", 5205694580, 5224280586), played("conv/011/07", 5227691038, 5255664895),
	played("conv/011/08", 5260589760, 5291022365), played("conv/011/09", 5299234243, 5314943546),
	played("conv/011/10", 5319766815, 5347983524), played("conv/011/11", 5352409024, 5372928467),
}

var k1Want = []wantSub{
	played("conv/011/04", 5107909466, 5149317320), played("conv/011/05", 5151525053, 5163793777),
	{"conv/011/06", 5167868820, 5186454826, 1, 0, ReasonOpen},
}

var subLongCases = map[string]subLongCase{
	"C11": {"i019-a/s500.state", s500SHA, "testdata/case_sub_c11.script", "i023-c/sbdma.tsv",
		func(*testing.T) []wantSub { return c11Want },
		map[string]string{"n": "conv/011/00", "gap": "", "p": "conv/011/02"}, 5375329016},
	"T10": {"i022-t/s500b.state", s500bSHA, "testdata/case_sub_t10.script", "i022-t/sbdma.tsv",
		func(t *testing.T) []wantSub { return timingWant(t, "timing-t10.tsv", 10, 5710000000) }, nil, 0},
	"H12": {"i022-h/s390.state", s390SHA, "testdata/case_sub_h12.script", "i022-h/sbdma.tsv",
		func(t *testing.T) []wantSub { return timingWant(t, "timing-h12.tsv", 12, 4900000000) },
		map[string]string{"split": "conv/012/05", "oa": "conv/012/14"}, 0},
	"K1": {"i019-a/s510.state", s510SHA, "testdata/case_sub_k1.script", "i022-k/sbdma-k1.tsv",
		func(*testing.T) []wantSub { return k1Want }, nil, 0},
	"K2": {"i019-a/s510.state", s510SHA, "testdata/case_sub_k2.script", "i022-k/sbdma-k2.tsv",
		func(*testing.T) []wantSub { return nil }, nil, 0},
	"K1s": {"i019-a/s500.state", s500SHA, "testdata/case_sub_k1s.script", "",
		func(*testing.T) []wantSub {
			return append([]wantSub{c11Want[0], c11Want[1], {"conv/011/02", 5087511261, 5112063300, 1, 5104910275, ReasonPause}}, k1Want...)
		}, map[string]string{"paused": ""}, 0},
	"K2s": {"i019-a/s500.state", s500SHA, "testdata/case_sub_k2s.script", "",
		func(*testing.T) []wantSub {
			return []wantSub{c11Want[0], c11Want[1], {"conv/011/02", 5087511261, 5112063300, 1, 5103562392, ReasonPause}}
		}, nil, 0},
	"M": {"i019-a/s510.state", s510SHA, "testdata/case_sub_m.script", "",
		func(*testing.T) []wantSub {
			return []wantSub{{"conv/011/03", 5115122728, 5143010075, 1, 0, ReasonOpen}}
		}, map[string]string{"m5": "conv/011/03"}, 0},
}

// timingWant 由 RE 022 的 timing tsv（讀檔位置對回的條目名，與物件無關）與結構表的語音編號得期望字幕條。
func timingWant(t *testing.T, file string, seg int, stop uint64) []wantSub {
	t.Helper()
	dir := os.Getenv(scI022Env)
	if dir == "" {
		t.Fatal(scI022Env + " 未設定")
	}
	idx := loadSpeechIndex(t)
	h, rows := readTSV(t, filepath.Join(dir, file))
	var out []wantSub
	for _, r := range rows {
		voc := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(r[col(h, "entry")], "F:"), ".VOC"))
		key := ""
		for _, l := range idx.Lines {
			if l.Seg == seg && l.Voc == voc {
				if key != "" {
					t.Fatalf("第 %d 段語音 %s 出現兩次", seg, voc)
				}
				key = l.Key
			}
		}
		if key == "" {
			t.Fatalf("第 %d 段找不到語音 %s", seg, voc)
		}
		start, _ := strconv.ParseUint(r[col(h, "start_step")], 10, 64)
		due, _ := strconv.ParseUint(r[col(h, "due_step")], 10, 64)
		w := wantSub{key, start, due, strings.Count(r[col(h, "dma_samples")], "+") + 1, due, ReasonDone}
		if c := r[col(h, "complete_step")]; c == "" || due > stop {
			w.end, w.reason = 0, ReasonOpen
		}
		out = append(out, w)
	}
	return out
}

func loadSpeechIndex(t *testing.T) *SpeechIndex {
	t.Helper()
	b, err := os.ReadFile(os.Getenv(scSpeechIndexEnv))
	if err != nil {
		t.Fatal(err)
	}
	x, err := ParseSpeechIndex(b)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

type subRunMode int

const (
	modeOff  subRunMode = iota // 甲：Overlay 關，不裝觀察器
	modeNil                    // 乙：Overlay 開、Subtitles 為 nil
	modeSubs                   // 丙：掛上字幕
)

func shaHex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func runSubCase(t *testing.T, c subLongCase, mode subRunMode, meta *SubtitleMeta) *ReceiptResult {
	t.Helper()
	root := t.TempDir()
	if err := CopyTree(os.Getenv(scRootEnv), root); err != nil {
		t.Fatal(err)
	}
	cfgB, err := os.ReadFile(filepath.Join(root, "SC.CFG"))
	if err != nil || shaHex(cfgB) != scCFGSHA256 {
		t.Fatalf("SC.CFG SHA-256 %s：%v", shaHex(cfgB), err)
	}
	raw, err := os.ReadFile(filepath.Join(os.Getenv(scStateDirEnv), c.state))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(c.sha)); err != nil {
		t.Fatal(err)
	}
	fontB, err := os.ReadFile(os.Getenv(scSubFontEnv))
	if err != nil {
		t.Fatal(err)
	}
	font, err := ParseLabelFont(fontB, "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	idxB, err := os.ReadFile(os.Getenv(scSpeechIndexEnv))
	if err != nil {
		t.Fatal(err)
	}
	idx, err := ParseSpeechIndex(idxB)
	if err != nil {
		t.Fatal(err)
	}
	catB, err := os.ReadFile(os.Getenv(scSpeechCatalogEnv))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := ParseCatalog(catB)
	if err != nil {
		t.Fatal(err)
	}
	labels, err := LoadCatalog(os.Getenv(scCatalogEnv))
	if err != nil {
		t.Fatal(err)
	}
	if meta != nil {
		*meta = SubtitleMeta{FontSHA256: shaHex(fontB), IndexSHA256: shaHex(idxB), CatalogSHA256: shaHex(catB), E: 0, T: SubtitleDefaultT}
	}
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
		StopSteps: script.Stop, Scales: []int{2, 3}, Catalog: labels, Font: font, Overlay: mode != modeOff, CursorY: -1,
		Inputs: script.Inputs, Shots: script.Shots,
		SpeechIndex: idx, SpeechCatalog: cat, Subtitles: mode == modeSubs, RecordSBDMA: mode != modeOff,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// compareProbe 是 §4.2 前置步驟第 2 步：session 經 452 記下的事件與 probe 收據比 Kind、Step、Due，start 另比 Samples、SHA256。
func compareProbe(t *testing.T, name string, c subLongCase, res *ReceiptResult) {
	t.Helper()
	if c.probe == "" {
		t.Logf("%s：沒有 probe 收據（K1s、K2s、M 由主代理以 probe 重錄，§4.2 前置步驟第 2 步）；session 事件 %d 筆已寫進收據", name, len(res.SBDMA))
		return
	}
	h, rows := readTSV(t, filepath.Join(os.Getenv(scStateDirEnv), c.probe))
	keep := map[string]bool{"start": true, "complete": true, "pause": true, "resume": true, "cancel": true}
	lo, hi := res.Boot.Steps, res.Digest.Steps
	var probeMax uint64
	for _, r := range rows {
		if s, _ := strconv.ParseUint(r[col(h, "Step")], 10, 64); s > probeMax {
			probeMax = s
		}
	}
	if probeMax < hi {
		hi = probeMax
	}
	key := func(kind, step, due, samples, sha string) string {
		if kind != "start" {
			samples, sha = "", ""
		}
		return strings.Join([]string{kind, step, due, samples, sha}, " ")
	}
	var p, s []string
	for _, r := range rows {
		st, _ := strconv.ParseUint(r[col(h, "Step")], 10, 64)
		if keep[r[col(h, "Kind")]] && st > lo && st <= hi {
			p = append(p, key(r[col(h, "Kind")], r[col(h, "Step")], r[col(h, "Due")], r[col(h, "Samples")], r[col(h, "SHA256")]))
		}
	}
	for _, e := range res.SBDMA {
		if keep[e.Kind] && e.Step > lo && e.Step <= hi {
			s = append(s, key(e.Kind, fmt.Sprint(e.Step), fmt.Sprint(e.Due), fmt.Sprint(e.Samples), e.SHA256))
		}
	}
	t.Logf("%s 前置步驟：probe %s 在 (%d, %d] 的 start／complete／pause／resume／cancel %d 列，session %d 列", name, c.probe, lo, hi, len(p), len(s))
	n := len(p)
	if len(s) < n {
		n = len(s)
	}
	for i := 0; i < n; i++ {
		if p[i] != s[i] {
			t.Errorf("%s 前置步驟第 %d 列不同：probe %q，session %q（§4.2 第 3 步：以 probe 等價參數重錄，交主代理）", name, i, p[i], s[i])
			return
		}
	}
	if len(p) != len(s) {
		t.Errorf("%s 前置步驟列數不同：probe %d，session %d", name, len(p), len(s))
	}
}

func checkSubs(t *testing.T, name string, c subLongCase, res *ReceiptResult) {
	t.Helper()
	want := c.want(t)
	for _, s := range res.Subs {
		end := ""
		if s.HasEnd {
			end = fmt.Sprint(s.End)
		}
		t.Logf("%s 字幕條 %s 起點 %d parts %d samples %d last_due %d 訖點 %s 原因 %s：%s", name, s.Key, s.Start, s.Parts, s.Acc, s.LastDue, end, s.Reason, s.Text)
	}
	if len(res.Subs) != len(want) {
		t.Errorf("%s 字幕條 %d 條，要 %d", name, len(res.Subs), len(want))
	}
	starts := 0
	for i, w := range want {
		if i >= len(res.Subs) {
			break
		}
		g := res.Subs[i]
		starts += g.Parts
		if g.Key != w.key || g.Start != w.start || g.LastDue != w.lastDue || g.Parts != w.parts || g.Reason != w.reason {
			t.Errorf("%s 第 %d 條：%s 起點 %d last_due %d parts %d 原因 %s，要 %s %d %d %d %s", name, i, g.Key, g.Start, g.LastDue, g.Parts, g.Reason, w.key, w.start, w.lastDue, w.parts, w.reason)
		}
		if w.end != 0 && (!g.HasEnd || g.End != w.end) {
			t.Errorf("%s 第 %d 條 %s 訖點 %d（有 %t），要 %d", name, i, g.Key, g.End, g.HasEnd, w.end)
		}
		if w.reason == ReasonOpen && g.HasEnd {
			t.Errorf("%s 第 %d 條 %s 未結束卻有訖點 %d", name, i, g.Key, g.End)
		}
	}
	t.Logf("%s：start 次數（字幕條 parts 合計）%d", name, starts)
	if name == "H12" {
		if starts != 24 {
			t.Errorf("H12 start 次數 %d，要 24", starts)
		}
		idx := loadSpeechIndex(t)
		for _, s := range res.Subs {
			if l, _ := idx.Line(s.Key); l != nil && l.Op == "0A" && l.Speaker != "stern" {
				t.Errorf("H12 %s 是 0A，說話人 %s，要 stern", s.Key, l.Speaker)
			}
		}
	}
	// subskips：只允許停止未解、字幕區非黑、字幕區恢復、傳輸不符與 miss（§4.2）。
	after := c.blockedAfter
	if after == 0 && len(res.Subs) != 0 {
		after = res.Subs[len(res.Subs)-1].LastDue
	}
	for _, k := range res.SubSkips {
		switch {
		case k.Reason == SkipUnresolved || k.Reason == SkipUnblocked || k.Reason == SkipXfer || strings.HasPrefix(k.Reason, "miss "):
		case k.Reason == SkipBlocked:
			if k.Step <= after && len(res.Subs) != 0 {
				t.Errorf("%s 第 %d 步「字幕區非黑」在對話結束（%d）之前", name, k.Step, after)
			}
		default:
			t.Errorf("%s 不允許的 skip：%+v", name, k)
		}
	}
	t.Logf("%s subskips %d 筆", name, len(res.SubSkips))
}

// checkSubShots：每張 shot 的合成圖與原版放大圖的差異只在輸出列 [152s, 200s)；沒有字幕條可見的 shot 差異為 0。
func checkSubShots(t *testing.T, name string, c subLongCase, res *ReceiptResult) {
	t.Helper()
	if len(res.SubShots) != len(res.Shots) {
		t.Fatalf("%s SubShots %d，Shots %d", name, len(res.SubShots), len(res.Shots))
	}
	for i, s := range res.Shots {
		ss := res.SubShots[i]
		if want, ok := c.shots[s.Name]; ok && ss.Key != want {
			t.Errorf("%s shot %s：可見 %q，要 %q", name, s.Name, ss.Key, want)
		}
		for _, sc := range []int{2, 3} {
			in := diffsIn(s.Composites[sc], s.Plain[sc], sc, 0, 152, 320, 200)
			all := diffsIn(s.Composites[sc], s.Plain[sc], sc, 0, 0, 320, 200)
			t.Logf("%s shot %s（第 %d 步，t %d）%d 倍：可見 %q、阻擋 %t，字幕區差異 %d、全圖 %d，行 %d", name, s.Name, s.Steps, ss.FrameStep, sc, ss.Key, ss.Blocked, in, all, len(ss.Lines[sc]))
			if all != in {
				t.Errorf("%s shot %s %d 倍：字幕區外有 %d 個差異", name, s.Name, sc, all-in)
			}
			if ss.Key == "" && all != 0 {
				t.Errorf("%s shot %s %d 倍：沒有字幕可見，差異 %d", name, s.Name, sc, all)
			}
			if ss.Key != "" && in == 0 {
				t.Errorf("%s shot %s %d 倍：字幕可見卻沒有差異", name, s.Name, sc)
			}
		}
	}
	if name == "M" {
		s := shotNamed(t, res, "m5")
		zero := true
		for _, v := range s.Indexed[152*320:] {
			zero = zero && v == 0
		}
		if !zero {
			t.Error("M：shot m5 的字幕區 Indexed 不是全 0")
		}
		if res.DOSMouse.X != 300 || res.DOSMouse.Y != 195 {
			t.Errorf("M：停止時 DOSMouse (%d,%d)，要 (300,195)", res.DOSMouse.X, res.DOSMouse.Y)
		}
	}
}

func TestSubtitleCase(t *testing.T) {
	for _, e := range []string{scRootEnv, scStateDirEnv, scSubFontEnv, scSpeechIndexEnv, scSpeechCatalogEnv, scCatalogEnv, scI022Env} {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定：010 §4.2 需要設定樹、狀態檔、字型、結構表、譯文與 i022")
		}
	}
	if !SessionConfig(nil).SBDMA8 {
		t.Fatal("SessionConfig 的 SBDMA8 不是真（452 §4 第 1 項）")
	}
	names := []string{"C11", "T10", "H12", "K1", "K2", "K1s", "K2s", "M"}
	if v := os.Getenv(scSubCasesEnv); v != "" {
		names = strings.Split(v, ",")
	}
	for _, name := range names {
		c, ok := subLongCases[name]
		if !ok {
			t.Fatalf("沒有案例 %s", name)
		}
		t.Run(name, func(t *testing.T) {
			t0 := time.Now()
			var meta SubtitleMeta
			on := runSubCase(t, c, modeSubs, &meta)
			t.Logf("丙：%d 步，耗時 %v；SB16 事件 %d 筆", on.Digest.Steps-on.Boot.Steps, time.Since(t0), len(on.SBDMA))
			if dir := os.Getenv(scOutEnv); dir != "" {
				if err := WriteSubtitleReceipt(filepath.Join(dir, "sub-"+strings.ToLower(name)), "case", on, meta, []int{2, 3}); err != nil {
					t.Fatal(err)
				}
			}
			compareProbe(t, name, c, on)
			checkSubs(t, name, c, on)
			checkSubShots(t, name, c, on)
			if os.Getenv(scSubABEnv) != "1" {
				return
			}
			t1 := time.Now()
			nilRun := runSubCase(t, c, modeNil, nil)
			t.Logf("乙：耗時 %v", time.Since(t1))
			t2 := time.Now()
			off := runSubCase(t, c, modeOff, nil)
			t.Logf("甲：耗時 %v", time.Since(t2))
			for _, x := range []struct {
				n string
				r *ReceiptResult
			}{{"甲", off}, {"乙", nilRun}, {"丙", on}} {
				d := x.r.Digest
				t.Logf("%s：Steps %d Memory %x CPU %x Indexed %x Palette %x Ext %s", x.n, d.Steps, d.MemorySHA256, d.CPUSHA256, d.IndexedSHA256, d.PaletteSHA256, d.ExtSHA256)
			}
			for _, r := range []*ReceiptResult{off, nilRun} {
				a, b := r.Digest, on.Digest
				if a.Steps != b.Steps || a.MemorySHA256 != b.MemorySHA256 || a.CPUSHA256 != b.CPUSHA256 ||
					a.IndexedSHA256 != b.IndexedSHA256 || a.PaletteSHA256 != b.PaletteSHA256 || a.ExtSHA256 != b.ExtSHA256 {
					t.Error("甲、乙、丙的停止步 Digest 不同")
				}
			}
			if len(nilRun.Shots) != len(on.Shots) {
				t.Fatalf("乙 shot %d、丙 %d", len(nilRun.Shots), len(on.Shots))
			}
			for i := range on.Shots {
				a, b := nilRun.Shots[i], on.Shots[i]
				if string(a.Indexed) != string(b.Indexed) || a.Palette != b.Palette {
					t.Errorf("shot %s：乙、丙的 Indexed 或 Palette 不同", b.Name)
				}
				for _, sc := range []int{2, 3} {
					in := diffsIn(a.Composites[sc], b.Composites[sc], sc, 0, 152, 320, 200)
					all := diffsIn(a.Composites[sc], b.Composites[sc], sc, 0, 0, 320, 200)
					t.Logf("shot %s %d 倍：乙丙合成圖差異 %d（字幕區 %d）", b.Name, sc, all, in)
					if all != in {
						t.Errorf("shot %s %d 倍：乙、丙的合成圖在字幕區外不同", b.Name, sc)
					}
				}
			}
		})
	}
}

// TestFormalSpeechCatalog 以 lint 檢查正式譯文 text/speech.zh-TW.tsv（010 §3.9）。
func TestFormalSpeechCatalog(t *testing.T) {
	envs := []string{"DOSGOLEM_SC_SPEECH_FORMAL", scSpeechIndexEnv, "DOSGOLEM_SC_SPEECH_EN", "DOSGOLEM_SC_GLOSSARY", scSubFontEnv}
	var bs [][]byte
	for _, e := range envs {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定")
		}
		b, err := os.ReadFile(os.Getenv(e))
		if err != nil {
			t.Fatal(err)
		}
		bs = append(bs, b)
	}
	f, err := ParseLabelFont(bs[4], "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	rep := LintSpeech(bs[1], bs[0], bs[2], bs[3], f.Wide)
	t.Logf("漏譯 %d 句；超過 2 行 %v", len(rep.Untranslated), rep.Over2)
	if !rep.OK() {
		t.Fatalf("lint：\n%s", strings.Join(rep.Errors, "\n"))
	}
}
