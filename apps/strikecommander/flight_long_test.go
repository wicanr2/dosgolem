package strikecommander

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/session"
)

// strike_commander 規格 009 §4.1 第 1 項（OPTTEST、MENU 狀態不啟用）、§4.2 案例 Q、R 與 A/B、§4.5 的 TestFormalFlightCatalog。
// 長測試，下列環境變數都設定時才執行，否則 t.Skip：
//   - DOSGOLEM_SC_ROOT、DOSGOLEM_SC_STATE_DIR（workplace/scratch：p476/s800.state、p487/s760.state、i018-st/m242.state、i018-cold/c800.state）；
//   - DOSGOLEM_SC_I021（workplace/i021：k1.script、k7.script）；
//   - DOSGOLEM_SC_FONT（16×16 正式字型，含 strike_mfd、strike_plaque 的字）、DOSGOLEM_SC_HUD_FONT（HUD12）；
//   - DOSGOLEM_SC_CATALOG（text/opttest_labels.zh-TW.tsv；它的目錄就是 text，讀三個 strike_* catalog）。
//
// DOSGOLEM_SC_FLIGHT_CASES 以逗號列出案例（預設 Q,R）；DOSGOLEM_SC_FLIGHT_RUNS 列出 run（預設 off,noflight,on23,on3）。
// DOSGOLEM_SC_OUT 有設定時，run-on23 的收據寫到 <OUT>/flight-<案例>-on23（shot 與停止點的 frame.idx、palette.bin、
// flight.tsv、flight-rows.bin、digest.txt、合成圖，另有 flight-commits.tsv、flight-cuframes.tsv、flight-counts.txt）。

const (
	scI021Env    = "DOSGOLEM_SC_I021"
	scHUDFontEnv = "DOSGOLEM_SC_HUD_FONT"
	s800SHA      = "f0c0730ca05050371b1f614c117daff8910453e7311964d085b8539ccb3ab7a9"
	s760SHA      = "6337a0dcd05cf479aed90cab2b918170c4874ade910f2360924ecf0610491801"
)

// flightCatalogsFrom 讀 text 目錄下三個飛行 catalog。
func flightCatalogsFrom(t *testing.T, dir string) FlightCatalogs {
	t.Helper()
	var c FlightCatalogs
	for _, x := range []struct {
		name string
		dst  *map[string]string
	}{{"strike_hud", &c.HUD}, {"strike_mfd", &c.MFD}, {"strike_plaque", &c.Plaque}} {
		m, err := LoadCatalog(filepath.Join(dir, x.name+".zh-TW.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		*x.dst = m
	}
	return c
}

// TestFormalFlightCatalog 以 LintFlight 檢查三個正式飛行 catalog（§4.5；TestFormalCatalog 不改，另寫這支）。
// hud 用 DOSGOLEM_SC_HUD_FONT 的寬度表，mfd、plaque 用 DOSGOLEM_SC_FONT；glossary 取 text/glossary.tsv。
func TestFormalFlightCatalog(t *testing.T) {
	cp, fp, hp := os.Getenv(scCatalogEnv), os.Getenv(scFontEnv), os.Getenv(scHUDFontEnv)
	if cp == "" || fp == "" || hp == "" {
		t.Skip(scCatalogEnv + "、" + scFontEnv + "、" + scHUDFontEnv + " 未全部設定")
	}
	dir := filepath.Dir(cp)
	label, err := LoadLabelFont(fp, "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	hud, err := LoadHUDFont(hp, "hud12-zh-TW")
	if err != nil {
		t.Skip("HUD12 字型讀不到：", err)
	}
	gb, err := os.ReadFile(filepath.Join(dir, "glossary.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	gl, err := ParseGlossaryRows(gb)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		kind Kind
		file string
		wide *WideTable
	}{{KindHUD, "strike_hud", hud.Wide}, {KindMFD, "strike_mfd", label.Wide}, {KindPlaque, "strike_plaque", label.Wide}} {
		b, err := os.ReadFile(filepath.Join(dir, c.file+".zh-TW.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		if errs := LintFlight(c.kind, b, c.wide, gl); len(errs) != 0 {
			t.Errorf("%s：%v", c.file, errs)
		}
		cat, err := ParseCatalog(b)
		if err != nil {
			t.Fatal(err)
		}
		f := label.Font
		if c.kind == KindHUD {
			f = hud.Font
		}
		for k, tr := range cat {
			for _, r := range tr {
				if f.Glyphs[r] == nil {
					t.Errorf("%s %q：字型缺 %q", c.file, k, r)
				}
			}
		}
	}
}

// TestFlightNoActivation 是 §4.1 第 1 項：OPTTEST 主選單與 MENU 狀態各跑 1 億步，FlightObserver 不啟用、沒有任何事件。
func TestFlightNoActivation(t *testing.T) {
	for _, e := range []string{scRootEnv, scStateDirEnv} {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定")
		}
	}
	for _, c := range []struct{ state, sha string }{{"i018-st/m242.state", m242SHA}, {"i018-cold/c800.state", c800SHA}} {
		t.Run(c.state, func(t *testing.T) {
			cats := flightTestCats()
			fo := NewFlightObserver(func(k Kind, s string) (string, int, bool) { return ResolveFlight(k, s, cats) })
			so := &SessionObserver{Labels: NewLabelObserver(), Flight: fo}
			o, boot := bootSC(t, c.state, c.sha, so)
			advanceTo(t, o, nil, boot.Steps, boot.Steps+100_000_000)
			logs, _ := fo.Log()
			if _, known := fo.Base(); known || fo.baseKnown || fo.Counts() != (FlightCounts{}) || len(logs) != 0 || len(fo.Visible()) != 0 {
				t.Fatalf("FlightObserver 有動作：base 已知 %t、計數 %+v、紀錄 %v", fo.baseKnown, fo.Counts(), logs)
			}
		})
	}
}

// flightScript 依 §4.2 由 i021 的腳本產生案例 Q、R 的腳本：去掉 ip 列、取到 last 那一列為止；Q 另在
// kdown 8370000010 19 之後插 shot 8371000000 k1-13-AltP2-mid。
func flightScript(t *testing.T, name string) (Script, uint64) {
	t.Helper()
	file, last, stop := "k1.script", "shot 9342000000 k1-45-Esc-b", uint64(9345000000)
	if name == "R" {
		file, last, stop = "k7.script", "shot 8042000000 k7-15-F-b", 8045000000
	}
	b, err := os.ReadFile(filepath.Join(os.Getenv(scI021Env), file))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	done := false
	for _, ln := range strings.Split(string(b), "\n") {
		s := strings.TrimSpace(ln)
		if strings.HasPrefix(s, "ip ") || strings.HasPrefix(s, "stop ") {
			continue
		}
		out = append(out, s)
		if name == "Q" && s == "kdown 8370000010 19" {
			out = append(out, "shot 8371000000 k1-13-AltP2-mid")
		}
		if s == last {
			done = true
			break
		}
	}
	if !done {
		t.Fatalf("%s 沒有 %q", file, last)
	}
	// DOSGOLEM_SC_FLIGHT_STOP（只供負對照的長測試層）：截到這一步，之後的事件與 shot 不跑。
	if s := os.Getenv("DOSGOLEM_SC_FLIGHT_STOP"); s != "" {
		var cut uint64
		if _, err := fmt.Sscan(s, &cut); err != nil || cut >= stop {
			t.Fatalf("DOSGOLEM_SC_FLIGHT_STOP %q 不合法", s)
		}
		var kept []string
		for _, ln := range out {
			f := strings.Fields(ln)
			if len(f) >= 2 && !strings.HasPrefix(ln, "#") {
				var st uint64
				if _, err := fmt.Sscan(f[1], &st); err == nil && st >= cut {
					continue
				}
			}
			kept = append(kept, ln)
		}
		out, stop = kept, cut
	}
	out = append(out, fmt.Sprintf("stop %d", stop))
	sc, err := ParseScript([]byte(strings.Join(out, "\n")+"\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	return sc, stop
}

type flightRunMode struct {
	name    string
	overlay bool
	flight  bool
	scales  []int
}

var flightRunModes = map[string]flightRunMode{
	"off":      {"off", false, false, nil},
	"noflight": {"noflight", true, false, []int{2, 3}},
	"on23":     {"on23", true, true, []int{2, 3}},
	"on3":      {"on3", true, true, []int{3}},
}

// flightShotSum 是一張 shot 跨 run 比較用的摘要（合成圖本身不跨 run 保留）。
type flightShotSum struct {
	name     string
	steps    uint64
	digest   session.StateDigest
	idxSHA   string
	comp     map[int]string
	plainEq  map[int]bool // 合成圖與原版放大圖逐位元組相同
	outside  map[int]int
	region2x int // 2 倍 [110,220)×[0,100) 扣掉橫幅遮蓋範圍的差異像素
}

func flightEnv(t *testing.T) {
	t.Helper()
	for _, e := range []string{scRootEnv, scStateDirEnv, scI021Env, scFontEnv, scHUDFontEnv, scCatalogEnv} {
		if os.Getenv(e) == "" {
			t.Skip(e + " 未設定：009 §4.2 案例需要設定樹、狀態檔、腳本、字型與 catalog")
		}
	}
}

func runFlightCase(t *testing.T, name string, m flightRunMode) (*ReceiptResult, []flightShotSum) {
	t.Helper()
	sc, stop := flightScript(t, name)
	state, sha := "p476/s800.state", s800SHA
	if name == "R" {
		state, sha = "p487/s760.state", s760SHA
	}
	root := t.TempDir()
	if err := CopyTree(os.Getenv(scRootEnv), root); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(os.Getenv(scStateDirEnv), state))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(sha)); err != nil {
		t.Fatal(err)
	}
	font, err := LoadLabelFont(os.Getenv(scFontEnv), "full16-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	hud, err := LoadHUDFont(os.Getenv(scHUDFontEnv), "hud12-zh-TW")
	if err != nil {
		t.Fatal(err)
	}
	labels, err := LoadCatalog(os.Getenv(scCatalogEnv))
	if err != nil {
		t.Fatal(err)
	}
	cfg := ReceiptConfig{SaveRoot: root, State: raw, StateSHA256: want, RebaseFrom: []string{"/scratch"},
		StopSteps: stop, Scales: m.scales, Catalog: labels, Font: font, Overlay: m.overlay, CursorY: -1,
		Inputs: sc.Inputs, Shots: sc.Shots, HUDFont: hud}
	if m.flight {
		cats := flightCatalogsFrom(t, filepath.Dir(os.Getenv(scCatalogEnv)))
		cfg.FlightCatalogs = &cats
	}
	start := time.Now()
	res, err := RunReceipt(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("案例 %s run-%s：%s，停止 %d，shot %d", name, m.name, time.Since(start).Round(time.Second), res.Digest.Steps, len(res.ShotDigests))
	if len(res.ShotDigests) != len(sc.Shots) {
		t.Fatalf("ShotDigests %d，shot %d", len(res.ShotDigests), len(sc.Shots))
	}
	var sums []flightShotSum
	for i, s := range sc.Shots {
		sum := flightShotSum{name: s.Name, steps: s.Steps, digest: res.ShotDigests[i], comp: map[int]string{}, plainEq: map[int]bool{}, outside: map[int]int{}}
		if m.overlay {
			sr := res.Shots[i]
			sum.idxSHA = shaHex(sr.Indexed)
			for _, k := range m.scales {
				sum.comp[k] = shaHex(sr.Composites[k])
				sum.plainEq[k] = string(sr.Composites[k]) == string(sr.Plain[k])
				sum.outside[k] = sr.Check[k].Outside
			}
			if m.flight && containsInt(m.scales, 2) {
				sum.region2x = region2xDiff(sr.Composites[2], sr.Plain[2], sr.FlightCovers[2])
			}
		}
		sums = append(sums, sum)
	}
	if m.name == "on23" && os.Getenv("DOSGOLEM_SC_OUT") != "" {
		writeFlightReceipts(t, filepath.Join(os.Getenv("DOSGOLEM_SC_OUT"), "flight-"+strings.ToLower(name)+"-on23"), res, m.scales)
	}
	// 合成圖不跨 run 保留。
	for i := range res.Shots {
		res.Shots[i].Composites, res.Shots[i].Plain = nil, nil
	}
	res.Composites, res.Plain = nil, nil
	return res, sums
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// region2xDiff 是 §4.2 的 2 倍固定區域比較：[110, 220) × [0, 100)（原版像素）扣掉橫幅遮蓋範圍，合成圖對原版放大圖的差異像素數。
func region2xDiff(comp, plain []byte, covers []FlightCover) int {
	n := 0
	for y := 0; y < 100*2; y++ {
		for x := 110 * 2; x < 220*2; x++ {
			ox, oy := x/2, y/2
			skip := false
			for _, c := range covers {
				if c.Kind == KindPlaque && ox >= c.X0 && ox < c.X1 && oy >= c.Y0 && oy < c.Y1 {
					skip = true
				}
			}
			if skip {
				continue
			}
			i := 4 * (y*640 + x)
			if comp[i] != plain[i] || comp[i+1] != plain[i+1] || comp[i+2] != plain[i+2] {
				n++
			}
		}
	}
	return n
}

func writeFlightReceipts(t *testing.T, dir string, res *ReceiptResult, scales []int) {
	t.Helper()
	for i := range res.Shots {
		s := &res.Shots[i]
		p := ShotPrefix(s)
		if err := WriteShotFiles(dir, p, s, scales); err != nil {
			t.Fatal(err)
		}
		if err := WriteFlightFiles(dir, p, scales, s.FlightReport, s.Digest); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteStopFiles(dir, res, scales); err != nil {
		t.Fatal(err)
	}
	if err := WriteFlightFiles(dir, fmt.Sprintf("stop-%d", res.Digest.Steps), scales, res.FlightReport, res.Digest); err != nil {
		t.Fatal(err)
	}
	must := func(name string, b []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("flight-commits.tsv", FlightCommitsTSV(res.FlightCommits))
	var cf strings.Builder
	cf.WriteString("steps\tvisible\tmismatch2\tmismatch3\tkeys3\n")
	for _, f := range res.FlightCUFrames {
		fmt.Fprintf(&cf, "%d\t%d\t%d\t%d\t%s\n", f.Steps, f.Visible, f.Mismatch[2], f.Mismatch[3], strings.Join(f.Keys[3], "|"))
	}
	must("flight-cuframes.tsv", []byte(cf.String()))
	var fb strings.Builder
	fb.WriteString("cu_steps\tframe_steps\tblocked2\tblocked3\n")
	for i, c := range res.FlightCommits {
		if i < len(res.FlightFirstBlocked) {
			fmt.Fprintf(&fb, "%d\t%d\t%d\t%d\n", c.CUSteps, c.FrameSteps, res.FlightFirstBlocked[i][2], res.FlightFirstBlocked[i][3])
		}
	}
	must("flight-firstblocked.tsv", []byte(fb.String()))
	var lg strings.Builder
	for _, e := range res.FlightLog {
		fmt.Fprintf(&lg, "%d\t%s\t%s\n", e.Steps, e.Kind, e.Detail)
	}
	must("flight-log.tsv", []byte(lg.String()))
	must("flight-counts.txt", []byte(fmt.Sprintf("%+v\n", res.FlightCounts)))
	var ss strings.Builder
	ss.WriteString("shot\tsteps\tgen\tvga\tvisible\thidden2\thidden3\tcounts\n")
	for _, s := range res.Shots {
		fmt.Fprintf(&ss, "%s\t%d\t%d\t%t\t%s\t%s\t%s\t%+v\n", s.Name, s.Steps, s.FlightGen, s.FlightVGAWindow, strings.Join(s.FlightVisible, "|"),
			strings.Join(s.FlightHidden[2], "|"), strings.Join(s.FlightHidden[3], "|"), s.FlightCounts)
	}
	must("flight-shots.tsv", []byte(ss.String()))
}

// flightWant 是 §4.2 正向斷言表的一列。
type flightWant struct {
	cu      uint64   // 顯示的畫格（CU 步數）
	hud3    []string // 3 倍顯示的 HUD 字串
	shown   []string // 2、3 倍顯示的 MFD 或橫幅字串
	notVis  []string // 不在可見集合
	extra   func(t *testing.T, s *ShotResult, res *ReceiptResult, all map[string]*ShotResult)
	noHUD   bool // 可見集合沒有 HUD 字串
	empty   bool // 可見集合為空
	gunTx   bool // GUN 譯「機砲」、tx 276（不左移）
	commFG  bool // Comm Mode: 的 fg 220
}

var flightWantQ = map[string]flightWant{
	"k1-00-base":  {cu: 8000705049, hud3: []string{"000 NONE"}},
	"k1-01-G-a":   {cu: 8012779028, hud3: []string{"DGFT"}, notVis: []string{"999 Vulcan"}},
	"k1-03-W-a":   {cu: 8072934756, hud3: []string{"STRF"}, shown: []string{"NORM", "GUN"}, gunTx: true},
	"k1-07-C-a":   {cu: 8192583279, shown: []string{"Comm Mode:", "Select Frequency", "NO RECEIVERS"}, commFG: true},
	"k1-10-Tab-a": {cu: 8282384410, shown: []string{"time burst"}},
	"k1-11-A-a":   {cu: 8312634092, shown: []string{"enemies near"}},
	"k1-12-AltP-a": {cu: 8340667429, hud3: []string{"DGFT"}, extra: func(t *testing.T, s *ShotResult, res *ReceiptResult, _ map[string]*ShotResult) {
		hiddenBoth(t, s, "enemies near")
		if hasStr(s.FlightVisible, "game paused") || s.FlightCounts.SkipVGA == 0 || !s.FlightVGAWindow {
			t.Errorf("%s：game paused 可見 %t、VGA 略過 %d、VGA 旗標 %t", s.Name, hasStr(s.FlightVisible, "game paused"), s.FlightCounts.SkipVGA, s.FlightVGAWindow)
		}
	}},
	"k1-13-AltP2-mid": {cu: 8340667429, hud3: []string{"DGFT"}, extra: func(t *testing.T, s *ShotResult, res *ReceiptResult, all map[string]*ShotResult) {
		hiddenBoth(t, s, "enemies near")
		if s.FlightVGAWindow || s.FlightCounts.CUCHNonZero < 1 {
			t.Errorf("%s：VGA 旗標 %t、CH ≠ 0 的 CU %d", s.Name, s.FlightVGAWindow, s.FlightCounts.CUCHNonZero)
		}
		if p := all["k1-12-AltP-a"]; p != nil && (p.FlightGen != s.FlightGen || !eqStrs(p.FlightVisible, s.FlightVisible)) {
			t.Errorf("%s：可見集合自 k1-12-AltP-a 之後改變（gen %d → %d）", s.Name, p.FlightGen, s.FlightGen)
		}
	}},
	"k1-13-AltP2-a": {cu: 8372904681, hud3: []string{"DGFT"}, shown: []string{"enemies near"}, extra: func(t *testing.T, s *ShotResult, res *ReceiptResult, _ map[string]*ShotResult) {
		for _, c := range res.FlightCommits {
			for _, x := range c.Texts {
				if x.Key == "game paused" {
					t.Errorf("game paused 在第 %d 步的 CU 被提交", c.CUSteps)
				}
			}
		}
	}},
	"k1-14-F2-a":   {cu: 8402660718, noHUD: true, shown: []string{"chase plane"}},
	"k1-17-F6-a":   {cu: 8492657311, shown: []string{"external view"}},
	"k1-19-F9-a":   {cu: 8552615610, shown: []string{"victim view on"}},
	"k1-20-F10-a":  {cu: 8582894662, shown: []string{"weapon camera on"}},
	"k1-22-B-a":    {cu: 8642531345, hud3: []string{"BRAKE"}},
	"k1-23-F-a":    {cu: 8672351663, hud3: []string{"FLAPS", "BRAKE"}},
	"k1-24-D-a":    {cu: 8702579210, shown: []string{"FUEL:"}},
	"k1-25-R-a":    {cu: 8732373702, shown: []string{"AIR"}},
	"k1-43-AltV-a": {cu: 9272311432, notVis: []string{"version: f2.3 -- instant"}},
	"k1-44-N-b":    {cu: 9304733889, empty: true},
	"k1-45-Esc-a":  {cu: 9332325325, hud3: []string{"DGFT", "FLAPS", "BRAKE"}, shown: []string{"FUEL:"}},
}

var flightWantR = map[string]flightWant{
	"k7-00-base":  {cu: 7600133942, hud3: []string{"GEAR"}},
	"k7-01-C-b":   {cu: 7621726410, shown: []string{"Comm Mode:", "Select Frequency"}, notVis: []string{"1 Hawk"}},
	"k7-03-C2-a":  {cu: 7671724062, shown: []string{"1 request status"}},
	"k7-04-B-b":   {cu: 7710267002, hud3: []string{"GEAR", "BRAKE"}},
	"k7-05-W-a":   {cu: 7732589387, hud3: []string{"SRM"}, shown: []string{"NORM"}, notVis: []string{"SWDJ"}},
	"k7-10-D-b":   {cu: 7890814292, shown: []string{"FUEL:"}},
	"k7-11-R-b":   {cu: 7921443115, shown: []string{"GND"}},
	"k7-15-F-b":   {cu: 8041833846, hud3: []string{"GEAR", "FLAPS", "BRAKE"}},
}

func hasStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func reportEntry(s *ShotResult, sc int, key string) (FlightEntry, bool) {
	for _, e := range s.FlightReport[sc] {
		if e.Key == key {
			return e, true
		}
	}
	return FlightEntry{}, false
}

func hiddenBoth(t *testing.T, s *ShotResult, key string) {
	t.Helper()
	if !hasStr(s.FlightVisible, key) {
		t.Errorf("%s：%q 不在可見集合", s.Name, key)
		return
	}
	for _, sc := range []int{2, 3} {
		if e, ok := reportEntry(s, sc, key); !ok || !e.Hidden {
			t.Errorf("%s：%d 倍 %q 不是 Hidden（有 %t）", s.Name, sc, key, ok)
		}
	}
}

// checkShown 是「顯示」：可見集合含該觀察字串，該倍率的 Report 有這筆、不 Hidden。
func checkShown(t *testing.T, s *ShotResult, key string, scales []int) {
	t.Helper()
	if !hasStr(s.FlightVisible, key) {
		t.Errorf("%s：%q 不在可見集合 %q", s.Name, key, s.FlightVisible)
		return
	}
	for _, sc := range scales {
		if e, ok := reportEntry(s, sc, key); !ok || e.Hidden {
			t.Errorf("%s：%d 倍 %q 沒有顯示（Report 有 %t）", s.Name, sc, key, ok)
		}
	}
}

func checkFlightWants(t *testing.T, res *ReceiptResult, wants map[string]flightWant) {
	all := map[string]*ShotResult{}
	for i := range res.Shots {
		all[res.Shots[i].Name] = &res.Shots[i]
	}
	names := make([]string, 0, len(wants))
	for n := range wants {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		w := wants[n]
		s := all[n]
		if s == nil {
			if os.Getenv("DOSGOLEM_SC_FLIGHT_STOP") != "" {
				continue // 截短的腳本沒有這張 shot
			}
			t.Errorf("沒有 shot %s", n)
			continue
		}
		// 顯示的畫格：shot 前最後一次升格的候選所屬 CU。
		var cu uint64
		for _, c := range res.FlightCommits {
			if c.FrameSteps <= s.Steps {
				cu = c.CUSteps
			}
		}
		if cu != w.cu {
			t.Errorf("%s：顯示的畫格 CU %d，要 %d", n, cu, w.cu)
		}
		for _, k := range w.hud3 {
			checkShown(t, s, k, []int{3})
		}
		for _, k := range w.shown {
			checkShown(t, s, k, []int{2, 3})
		}
		for _, k := range w.notVis {
			if hasStr(s.FlightVisible, k) {
				t.Errorf("%s：%q 在可見集合", n, k)
			}
		}
		if w.empty && len(s.FlightVisible) != 0 {
			t.Errorf("%s：可見集合 %q，要空", n, s.FlightVisible)
		}
		if w.noHUD {
			for _, e := range s.FlightReport[3] {
				if e.Kind == KindHUD {
					t.Errorf("%s：可見集合有 HUD 字串 %q", n, e.Key)
				}
			}
			for _, k := range []string{"GEAR", "FLAPS", "BRAKE", "DGFT", "STRF", "SRM", "000 NONE"} {
				if hasStr(s.FlightVisible, k) {
					t.Errorf("%s：可見集合有 HUD 字串 %q", n, k)
				}
			}
		}
		if w.gunTx {
			// 使用者 2026-10-21：GUN 譯「機砲」，原位放得下，不左移。
			for _, sc := range []int{2, 3} {
				if e, ok := reportEntry(s, sc, "GUN"); !ok || e.TX != 276 || e.Text != "機砲" {
					t.Errorf("%s：%d 倍 GUN tx %d 譯文 %q，要 276「機砲」", n, sc, e.TX, e.Text)
				}
			}
		}
		if w.commFG {
			for _, sc := range []int{2, 3} {
				if e, ok := reportEntry(s, sc, "Comm Mode:"); !ok || e.FG != 220 {
					t.Errorf("%s：%d 倍 Comm Mode: fg %d，要 220", n, sc, e.FG)
				}
			}
		}
		if w.extra != nil {
			w.extra(t, s, res, all)
		}
		t.Logf("%s：可見 %q；2 倍 Hidden %q；3 倍 Hidden %q", n, s.FlightVisible, s.FlightHidden[2], s.FlightHidden[3])
	}
}

func TestFlightCase(t *testing.T) {
	flightEnv(t)
	cases := []string{"Q", "R"}
	if s := os.Getenv("DOSGOLEM_SC_FLIGHT_CASES"); s != "" {
		cases = strings.Split(s, ",")
	}
	runs := []string{"off", "noflight", "on23", "on3"}
	if s := os.Getenv("DOSGOLEM_SC_FLIGHT_RUNS"); s != "" {
		runs = strings.Split(s, ",")
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			sums := map[string][]flightShotSum{}
			stops := map[string]session.StateDigest{}
			var on23, on3 *ReceiptResult
			for _, r := range runs {
				m, ok := flightRunModes[r]
				if !ok {
					t.Fatalf("不認得的 run %q", r)
				}
				res, s := runFlightCase(t, name, m)
				sums[r], stops[r] = s, res.Digest
				switch r {
				case "on23":
					on23 = res
				case "on3":
					on3 = res
				}
				if r == "on23" || r == "on3" {
					checkFlightRun(t, name, r, res)
				}
			}
			compareFlightRuns(t, runs, sums, stops)
			if on23 != nil {
				checkFlightWants(t, on23, map[string]map[string]flightWant{"Q": flightWantQ, "R": flightWantR}[name])
				if name == "Q" && os.Getenv("DOSGOLEM_SC_FLIGHT_STOP") == "" {
					checkFlightQExtra(t, on23)
				}
			}
			_ = on3
		})
	}
}

// checkFlightRun 是 on23、on3 各自的斷言：計數不變式、核對不通過與作廢 0、逾時與 SS:SP 不符 0。
func checkFlightRun(t *testing.T, name, run string, res *ReceiptResult) {
	t.Helper()
	c := res.FlightCounts
	t.Logf("案例 %s run-%s：升格 %d、gen %d、計數 %+v", name, run, len(res.FlightCommits), genCount(res), c)
	pend := 0
	if len(res.FlightCommits) != c.Promotions {
		t.Errorf("FlightCommits %d ≠ 升格 %d", len(res.FlightCommits), c.Promotions)
	}
	// 結束時仍待升格（0 或 1）：不變式容許其一。
	diff := c.ReturnsAccepted - c.CandidatesReplacedAwaiting - c.Promotions
	if diff != 0 && diff != 1 {
		t.Errorf("不變式：接受返回 %d − 返回後被取代 %d − 升格 %d = %d，要 0 或 1", c.ReturnsAccepted, c.CandidatesReplacedAwaiting, c.Promotions, diff)
	}
	_ = pend
	if c.Timeouts != 0 || c.ReturnsSSMismatch != 0 || c.EntriesVoided != 0 {
		t.Errorf("逾時 %d、SS:SP 不符返回 %d、作廢入口 %d，都要 0", c.Timeouts, c.ReturnsSSMismatch, c.EntriesVoided)
	}
	if c.SkipInk != 0 || c.SkipVoid != 0 {
		t.Errorf("墨跡框核對不通過 %d、作廢 %d，要 0", c.SkipInk, c.SkipVoid)
		for _, e := range res.FlightLog {
			if e.Kind == "ink" || e.Kind == "void" {
				t.Logf("  第 %d 步 %s：%s", e.Steps, e.Kind, e.Detail)
			}
		}
	}
	for i, s := range res.Shots {
		for sc, cc := range s.Check {
			if cc.Outside != 0 {
				t.Errorf("shot %d %s %d 倍：CheckComposite.Outside %d", i, s.Name, sc, cc.Outside)
			}
		}
	}
	for sc, cc := range res.CompositeCheck {
		if cc.Outside != 0 {
			t.Errorf("停止點 %d 倍：CheckComposite.Outside %d", sc, cc.Outside)
		}
	}
	for sc, fov := range res.FlightOverlays {
		t.Logf("  %d 倍 misses %+v skips %+v", sc, fov.Misses, fov.Skips)
		if len(fov.Misses) != 0 {
			t.Errorf("%d 倍有 miss：%+v", sc, fov.Misses)
		}
	}
}

func genCount(res *ReceiptResult) uint64 {
	if len(res.FlightCommits) == 0 {
		return 0
	}
	return res.FlightCommits[len(res.FlightCommits)-1].Generation
}

// checkFlightQExtra 是案例 Q 的 CU 期間回掃、重複返回斷言（§4.2 比較表）。
func checkFlightQExtra(t *testing.T, res *ReceiptResult) {
	t.Helper()
	c := res.FlightCounts
	if c.CUFrames < 4 {
		t.Errorf("CU 期間回掃 %d，要 ≥ 4", c.CUFrames)
	}
	if c.ReturnsIgnored < 5 {
		t.Errorf("被忽略的重複返回 %d，要 ≥ 5", c.ReturnsIgnored)
	}
	seen := map[uint64]bool{}
	for _, f := range res.FlightCUFrames {
		seen[f.Steps] = true
		t.Logf("CU 期間回掃第 %d 步：可見 %d，不符 2 倍 %d、3 倍 %d %q", f.Steps, f.Visible, f.Mismatch[2], f.Mismatch[3], f.Keys[3])
		if f.Steps == 9331740001 && (f.Mismatch[2] != 0 || f.Mismatch[3] != 0) {
			t.Errorf("9331740001 的 CU 期間比對不符 %v，要 0", f.Mismatch)
		}
	}
	for _, s := range []uint64{9304845001, 9330255001, 9331080001, 9331740001} {
		if !seen[s] {
			t.Errorf("CU 期間回掃沒有第 %d 步", s)
		}
	}
	t.Logf("升格後第一個 Frame 被擋下 %d 筆、最長連續 %d", c.FirstFrameBlocked, c.FirstFrameBlockedMaxRun)
}

// compareFlightRuns 是 §4.2 的 A/B 比較。
func compareFlightRuns(t *testing.T, runs []string, sums map[string][]flightShotSum, stops map[string]session.StateDigest) {
	t.Helper()
	ref := runs[0]
	for _, r := range runs[1:] {
		if stops[r] != stops[ref] {
			t.Errorf("停止點 Digest：run-%s %+v ≠ run-%s %+v", r, stops[r], ref, stops[ref])
		}
		for i := range sums[r] {
			if sums[r][i].digest != sums[ref][i].digest {
				t.Errorf("shot %s 的 Digest：run-%s ≠ run-%s", sums[r][i].name, r, ref)
			}
		}
	}
	var drawn []string
	for _, r := range runs {
		if r != "off" {
			drawn = append(drawn, r)
		}
	}
	for _, r := range drawn[min(1, len(drawn)):] {
		for i := range sums[r] {
			if sums[r][i].idxSHA != sums[drawn[0]][i].idxSHA {
				t.Errorf("shot %s 的 frame.idx：run-%s ≠ run-%s", sums[r][i].name, r, drawn[0])
			}
		}
	}
	for _, s := range sums["noflight"] {
		for sc, eq := range s.plainEq {
			if !eq {
				t.Errorf("run-noflight shot %s %d 倍合成圖不等於原版放大圖", s.name, sc)
			}
		}
	}
	for i, s := range sums["on23"] {
		if s.region2x != 0 {
			t.Errorf("run-on23 shot %s 2 倍固定區域差異 %d", s.name, s.region2x)
		}
		if o3 := sums["on3"]; len(o3) > i && o3[i].comp[3] != s.comp[3] {
			t.Errorf("shot %s 3 倍合成圖：run-on3 ≠ run-on23", s.name)
		}
	}
	t.Logf("A/B：%v 的停止點與 shot Digest 比較完成", runs)
}

var _ = sha256.Sum256
