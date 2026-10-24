package strikecommander

// 規格 013 第 1 期：原版選配錄製、自檢與中立性驗收。
// 只在 SC_MODEL_ROOT、SC_MODEL_STATE、SC_MODEL_OUT 都設定時執行。
// SC_MODEL_CASE=s760/s800f2/n1/n2；SC_MODEL_GROUPS=20/30；
// SC_MODEL_MAX_STEPS 是從載入步起算的上限，預設 200,000,000。
// SC_MODEL_RASTER=1 改用 012 的場景錄製與 1 倍光柵化自檢。
// SC_MODEL_PALETTE=1 保存回掃後色盤；SC_MODEL_PRESENT=1 以 /project 的正式
// catalog、圖集與字幕觀察器做 2/3 倍 HD 關合成，同時保存色盤。
// SC_MODEL_OUT 必須不存在，內容含玩家原版資料，只能留在本機。
// 本期不渲染 HD；不比較未公開的 Machine/DOS SaveState 原始 bytes。

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/session"
)

type modelOriginalCase struct {
	sha          string
	start, first uint64
	objects      int
	xyz, screens uint64
}

// 起點與數量取規格 013 §3.7、RE 028；first 是探針第一個 R7。
var modelOriginalCases = map[string]modelOriginalCase{
	"s760":   {"6337a0dcd05cf479aed90cab2b918170c4874ade910f2360924ecf0610491801", 7600000000, 7600218725, 5, 6480, 6270},
	"s800f2": {"f0c0730ca05050371b1f614c117daff8910453e7311964d085b8539ccb3ab7a9", 8000000000, 8001101151, 1, 7147, 7147},
	"n1":     {"6c88fc02cb183f1faceea3e027e5c6020af11add2d0d15c3b533dbb71fbbc89e", 7608000000, 7609747998, 7, 16578, 16578},
	"n2":     {"306c9153bd1c416ac474381728b77dfc1bccdf4def1e56f164f9e44bd53902d6", 7840000000, 7840228958, 4, 15090, 15090},
}

// 所有值都在合法 BeforeStep 內讀取，不持有傳入的 StepView。
// LastDeliveries 是最後一個步驟嘗試之前的值，不冒稱停止後的精確計數。
type modelOriginalObserver struct {
	*SessionObserver
	LastBeforeStep, LastDeliveries uint64
	SampleFrom, RawFirstR7         uint64
}

func (o *modelOriginalObserver) VideoModeChange(c machine.ModeChange) {
	presentObserver{o.SessionObserver}.VideoModeChange(c)
}

func (o *modelOriginalObserver) BeforeStep(v session.StepView) error {
	o.LastBeforeStep, o.LastDeliveries = v.Steps(), v.Deliveries()
	// 獨立原始命中點，不使用錄製器的 base、規則或已錄群組。
	if o.RawFirstR7 == 0 && v.Steps() >= o.SampleFrom && v.CS() == 0x2af8 && v.IP() == 0x0102 {
		o.RawFirstR7 = v.Steps()
	}
	return o.SessionObserver.BeforeStep(v)
}

func modelOriginalSampleFrom(name string, c modelOriginalCase) uint64 {
	if name == "s800f2" {
		return 8001000000
	}
	return c.start
}

type modelOriginalR7Comparison struct {
	LegacyFirst, SampleFrom, FirstGroup, RawA, RawB, RawB2 uint64
	LegacyOffset                                           int64
	Interpretation                                         string
}

type modelOriginalSnapshot struct {
	Steps                          uint64
	Digest                         session.StateDigest
	MachineDigest                  session.FullDigest
	Frames                         int
	LastBeforeStep, LastDeliveries uint64
	FlightCommits                  []json.RawMessage
	Palette                        *[256][3]uint8   `json:",omitempty"`
	PaletteFrameStep               uint64           `json:",omitempty"`
	CompositeSHA256, PlainSHA256   map[int][32]byte `json:",omitempty"`
	Subtitles, Composition         json.RawMessage  `json:",omitempty"`
}

type modelOriginalPresent struct {
	Fonts               *TextFonts
	Labels, Gfx, Speech map[string]string
	Flight              FlightCatalogs
	Index               *SpeechIndex
	ResourceSHA256      map[string]string
}

func modelOriginalFormalPresent(t *testing.T) *modelOriginalPresent {
	t.Helper()
	p := &modelOriginalPresent{ResourceSHA256: map[string]string{}}
	read := func(rel string) []byte {
		b, err := os.ReadFile(filepath.Join("/project", rel))
		if err != nil {
			t.Fatal(err)
		}
		p.ResourceSHA256[rel] = fmt.Sprintf("%x", sha256.Sum256(b))
		return b
	}
	// LoadTextFonts 同時核對 manifest、expected 與每個真實 .aaf 的 SHA。
	var err error
	p.Fonts, err = LoadTextFonts("/project/workplace/s011impl/font-formal", "/project/font/zh-TW.cjkb.expected.tsv")
	if err != nil {
		t.Fatal(err)
	}
	read("font/zh-TW.cjkb.expected.tsv")
	read("workplace/s011impl/font-formal/zh-TW.cjkb.manifest.tsv")
	for _, file := range p.Fonts.Files {
		read(filepath.Join("workplace/s011impl/font-formal", file.Name))
	}
	cat := func(rel string) map[string]string {
		c, err := ParseCatalog(read(rel))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	p.Labels = cat("text/opttest_labels.zh-TW.tsv")
	p.Gfx = cat("text/menu_gfx.zh-TW.tsv")
	p.Speech = cat("text/speech.zh-TW.tsv")
	p.Flight = FlightCatalogs{HUD: cat("text/strike_hud.zh-TW.tsv"), MFD: cat("text/strike_mfd.zh-TW.tsv"), Plaque: cat("text/strike_plaque.zh-TW.tsv")}
	p.Index, err = ParseSpeechIndex(read("text/speech.index.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type modelOriginalResult struct {
	Stop                          uint64
	RawFirstR7, FirstGroupR7      uint64
	Checkpoints                   []modelOriginalSnapshot
	GroupJSON                     [][]byte
	GroupSteps, GroupDeliveries   []uint64
	FlightBaseValid               []bool
	Counts                        ModelRecorderCounts
	Logs                          []string
	Failures                      []string
	RasterMode                    bool
	RawJSONBytes, StoredJSONBytes uint64
}

func modelOriginalJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func modelOriginalWrite(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
}

// 完整群組 JSON 可壓縮儲存，B/B2 仍比較原始 bytes。
func modelOriginalWriteGroup(t *testing.T, p string, b []byte, compressed bool) uint64 {
	t.Helper()
	if !compressed {
		modelOriginalWrite(t, p, b)
		return uint64(len(b))
	}
	// 陣列完整保存；決定性比較仍使用未壓縮 JSON bytes。
	var buf bytes.Buffer
	z := gzip.NewWriter(&buf)
	if _, err := z.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	modelOriginalWrite(t, p+".gz", buf.Bytes())
	return uint64(buf.Len())
}

// CopyTree 不保存 mtime；複製後逐檔復原，三模式共用同一 SaveRoot。
func modelOriginalRestore(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.RemoveAll(dst); err != nil {
		t.Fatal(err)
	}
	if err := CopyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		return os.Chtimes(filepath.Join(dst, rel), info.ModTime(), info.ModTime())
	}); err != nil {
		t.Fatal(err)
	}
}

func TestModelOriginal(t *testing.T) {
	root, state, out := os.Getenv("SC_MODEL_ROOT"), os.Getenv("SC_MODEL_STATE"), os.Getenv("SC_MODEL_OUT")
	if root == "" || state == "" || out == "" {
		t.Skip("SC_MODEL_ROOT、SC_MODEL_STATE、SC_MODEL_OUT 未設定")
	}
	name := os.Getenv("SC_MODEL_CASE")
	c, ok := modelOriginalCases[name]
	if !ok {
		t.Fatalf("SC_MODEL_CASE %q：須為 s760/s800f2/n1/n2", name)
	}
	for _, p := range []string{state, filepath.Join(root, "STRIKE.EXE"), filepath.Join(root, "SC.CFG"), filepath.Join(root, "BIGTRE.TRE"), filepath.Join(root, "LILTRE.TRE"), filepath.Join(root, "TOBIGTRE.TRE")} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Skipf("缺素材 %s", p)
		} else if err != nil {
			t.Fatal(err)
		}
	}
	groups := 20
	if s := os.Getenv("SC_MODEL_GROUPS"); s != "" {
		var err error
		groups, err = strconv.Atoi(s)
		if err != nil || (groups != 20 && groups != 30) {
			t.Fatal("SC_MODEL_GROUPS 須為 20 或 30")
		}
	}
	maxSteps := uint64(200000000)
	if s := os.Getenv("SC_MODEL_MAX_STEPS"); s != "" {
		var err error
		maxSteps, err = strconv.ParseUint(s, 10, 64)
		if err != nil || maxSteps == 0 || maxSteps > ^uint64(0)-c.start {
			t.Fatal("SC_MODEL_MAX_STEPS 不合法")
		}
	}
	if _, err := os.Lstat(out); err == nil {
		t.Fatal("SC_MODEL_OUT 已存在，不覆寫")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	sha := sha256.Sum256(raw)
	if fmt.Sprintf("%x", sha) != c.sha {
		t.Fatalf("%s state SHA-256 %x，要 %s", name, sha, c.sha)
	}
	exe, err := os.ReadFile(filepath.Join(root, "STRIKE.EXE"))
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(exe); fmt.Sprintf("%x", sum) != "883270cd0038fe571376202233b8abf5efb77889e27f4f0caf97dfab7e529cb5" {
		t.Fatalf("STRIKE.EXE 非規格 013 oracle：%x", sum)
	}
	lib, err := realspace.LoadLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	var tables *hd3d.Tables
	switch os.Getenv("SC_MODEL_RASTER") {
	case "":
	case "1":
		tab, err := hd3d.TablesFromEXE(exe)
		if err != nil {
			t.Fatal(err)
		}
		tables = &tab
	default:
		t.Fatal("SC_MODEL_RASTER 只接受未設定或 1")
	}
	var present *modelOriginalPresent
	switch os.Getenv("SC_MODEL_PRESENT") {
	case "":
	case "1":
		present = modelOriginalFormalPresent(t)
	default:
		t.Fatal("SC_MODEL_PRESENT 只接受未設定或 1")
	}
	palette := present != nil
	switch os.Getenv("SC_MODEL_PALETTE") {
	case "":
	case "1":
		palette = true
	default:
		t.Fatal("SC_MODEL_PALETTE 只接受未設定或 1")
	}
	if err := os.Mkdir(out, 0700); err != nil {
		t.Fatal(err)
	}
	saveRoot := filepath.Join(t.TempDir(), "save")
	input := struct {
		Case, State, StateSHA256, OracleEXESHA256, SaveRoot, GoVersion, DeliveriesScope, RawSaveState string
		Groups                                                                                        int
		Present, Palette                                                                              bool
		FontAnchor                                                                                    string
		ResourceSHA256                                                                                map[string]string
		MaxSteps, TurnSteps                                                                           uint64
		Library                                                                                       realspace.LibraryStats
		R7                                                                                            modelOriginalR7Comparison
		Raster                                                                                        bool
		GroupEncoding                                                                                 string
	}{
		Case: name, State: state, StateSHA256: c.sha, OracleEXESHA256: fmt.Sprintf("%x", sha256.Sum256(exe)),
		SaveRoot: saveRoot, GoVersion: runtime.Version(), DeliveriesScope: "最後一次合法 BeforeStep；非停止後精確計數",
		RawSaveState: "本輪未比較，依正式契約比較完整 Digest/MachineDigest", Groups: groups, MaxSteps: maxSteps, TurnSteps: 100000, Library: lib.Stats(),
		R7:     modelOriginalR7Comparison{LegacyFirst: c.first, SampleFrom: modelOriginalSampleFrom(name, c), Interpretation: "459後runner讀取舊state；跨版本步數差異僅供診斷，原因未逐指令追證，未定。"},
		Raster: tables != nil, GroupEncoding: "JSON；光柵化模式以 gzip 完整保存，B/B2 比未壓縮 JSON bytes",
		Present: present != nil, Palette: palette,
	}
	if present != nil {
		input.FontAnchor = present.Fonts.Anchor
		input.ResourceSHA256 = present.ResourceSHA256
	}
	modelOriginalWrite(t, filepath.Join(out, "input.json"), modelOriginalJSON(t, input))
	b := modelOriginalRun(t, root, saveRoot, out, "B", raw, sha, lib, tables, present, palette, name, c, groups, c.start+maxSteps, nil)
	input.R7.FirstGroup, input.R7.RawB = b.FirstGroupR7, b.RawFirstR7
	if b.FirstGroupR7 != 0 {
		input.R7.LegacyOffset = int64(b.FirstGroupR7) - int64(c.first)
	}
	modelOriginalWrite(t, filepath.Join(out, "input.json"), modelOriginalJSON(t, input))
	a := modelOriginalRun(t, root, saveRoot, out, "A", raw, sha, nil, nil, present, palette, name, c, groups, b.Stop, b.Checkpoints)
	b2 := modelOriginalRun(t, root, saveRoot, out, "B2", raw, sha, lib, tables, present, palette, name, c, groups, b.Stop, b.Checkpoints)
	input.R7.RawA, input.R7.RawB2 = a.RawFirstR7, b2.RawFirstR7
	modelOriginalWrite(t, filepath.Join(out, "input.json"), modelOriginalJSON(t, input))
	var failures []string
	failures = append(failures, b.Failures...)
	failures = append(failures, b2.Failures...)
	if b.RawFirstR7 == 0 || a.RawFirstR7 != b.RawFirstR7 || b2.RawFirstR7 != b.RawFirstR7 || b.FirstGroupR7 != b.RawFirstR7 || b2.FirstGroupR7 != b2.RawFirstR7 {
		failures = append(failures, fmt.Sprintf("原始首R7 A/B/B2=%d/%d/%d，錄製首R7 B/B2=%d/%d，須非零且相同", a.RawFirstR7, b.RawFirstR7, b2.RawFirstR7, b.FirstGroupR7, b2.FirstGroupR7))
	}
	if !reflect.DeepEqual(b.Checkpoints, a.Checkpoints) {
		failures = append(failures, "A/B 的 Digest、MachineDigest、FlightCommit、Frame 或最後 BeforeStep/Deliveries 不同")
	}
	if !reflect.DeepEqual(b.Checkpoints, b2.Checkpoints) {
		failures = append(failures, "B/B2 的完整檢查點不同")
	}
	if !reflect.DeepEqual(b.GroupSteps, b2.GroupSteps) || !reflect.DeepEqual(b.GroupDeliveries, b2.GroupDeliveries) || !reflect.DeepEqual(b.FlightBaseValid, b2.FlightBaseValid) {
		failures = append(failures, "B/B2 的群組升格時點、送達讀值或 Flight.Base 狀態不同")
	}
	if len(b.GroupJSON) != len(b2.GroupJSON) {
		failures = append(failures, "B/B2 群組數不同")
	} else {
		for i := range b.GroupJSON {
			if !bytes.Equal(b.GroupJSON[i], b2.GroupJSON[i]) {
				failures = append(failures, fmt.Sprintf("B/B2 第 %d 群組 JSON bytes 不同", i+1))
			}
		}
	}
	modelOriginalWrite(t, filepath.Join(out, "comparison.json"), modelOriginalJSON(t, struct {
		Stop                          uint64
		Groups                        int
		Failures                      []string
		R7                            modelOriginalR7Comparison
		Raster                        bool
		RawJSONBytes, StoredJSONBytes uint64
	}{b.Stop, len(b.GroupJSON), failures, input.R7, tables != nil, b.RawJSONBytes + b2.RawJSONBytes, b.StoredJSONBytes + b2.StoredJSONBytes}))
	for _, f := range failures {
		t.Error(f)
	}
}

func modelOriginalRun(t *testing.T, root, saveRoot, out, mode string, raw []byte, sha [32]byte, lib *realspace.Library, tables *hd3d.Tables, present *modelOriginalPresent, palette bool, name string, c modelOriginalCase, want int, limit uint64, checkpoints []modelOriginalSnapshot) modelOriginalResult {
	t.Helper()
	dir := filepath.Join(out, mode)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	modelOriginalRestore(t, root, saveRoot)
	var res modelOriginalResult
	cats := flightTestCats()
	if present != nil {
		cats = present.Flight
	}
	// 只用 catalog 的原文鍵與分類，譯文不進這條驗收路徑。
	fo := NewFlightObserver(func(k Kind, s string) (string, int, bool) { key, n, ok := ResolveFlight(k, s, cats); return key, n, ok })
	var commits []json.RawMessage
	fo.OnCommit = func(fc FlightCommit) { commits = append(commits, modelOriginalJSON(t, fc)) }
	so := &modelOriginalObserver{SessionObserver: &SessionObserver{Labels: NewLabelObserver(), Flight: fo}, SampleFrom: modelOriginalSampleFrom(name, c)}
	var frameStep uint64
	so.OnFrame = func(_, _ []byte, steps uint64) { frameStep = steps }
	var dma []machine.SBDMAEvent
	ovs := map[int]*Overlay{}
	fovs := map[int]*FlightOverlay{}
	if present != nil {
		var err error
		so.Subtitles, err = NewSubtitlesText(present.Index, present.Speech, present.Fonts, 0, SubtitleDefaultT)
		if err != nil {
			t.Fatal(err)
		}
		so.OnSBDMA = func(ev machine.SBDMAEvent) { dma = append(dma, ev) }
		for _, sc := range []int{2, 3} {
			ovs[sc], err = NewOverlayText(present.Fonts, present.Labels, present.Gfx, sc)
			if err != nil {
				t.Fatal(err)
			}
			fovs[sc], err = NewFlightOverlayText(present.Fonts, present.Flight, sc)
			if err != nil {
				t.Fatal(err)
			}
		}
		so.Labels.InkCheck = true
		so.AttachOverlays(ovs[2], ovs[3])
	}
	var rec *ModelRecorder
	if lib != nil {
		rec = NewModelRecorder(lib)
		if tables != nil {
			rec = NewSceneRecorder(lib, *tables)
			res.RasterMode = true
		}
		so.Models = rec
		rec.OnGroup = func(g *ModelGroup) {
			if g.Steps < so.SampleFrom || len(res.GroupJSON) >= want {
				return
			}
			if len(res.GroupJSON) == 0 {
				res.FirstGroupR7 = g.Steps
			}
			res.GroupJSON = append(res.GroupJSON, modelOriginalJSON(t, g))
			res.GroupSteps = append(res.GroupSteps, so.LastBeforeStep)
			res.GroupDeliveries = append(res.GroupDeliveries, so.LastDeliveries)
			_, valid := fo.Base()
			res.FlightBaseValid = append(res.FlightBaseValid, valid)
		}
	}
	cfg := SessionConfig(so)
	cfg.SparseObserve = false // 第一期逐步錄製；不受其他測試的 sparseOverride 影響。
	o, err := session.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	boot, err := o.BootFromState(session.StateInput{State: raw, ExpectedStateSHA256: sha, SaveRoot: saveRoot, RebaseFrom: []string{"/scratch"}})
	if err != nil {
		t.Fatal(err)
	}
	if boot.Steps != c.start {
		t.Fatalf("%s 起點 %d，要 %d", name, boot.Steps, c.start)
	}
	current := boot.Steps
	checkpointIndex := 0
	previousGroups := 0
	snapshot := func() {
		d, err := o.Digest()
		if err != nil {
			t.Fatal(err)
		}
		md, err := o.MachineDigest()
		if err != nil {
			t.Fatal(err)
		}
		cp := modelOriginalSnapshot{Steps: current, Digest: d, MachineDigest: md, Frames: so.Frames, LastBeforeStep: so.LastBeforeStep, LastDeliveries: so.LastDeliveries, FlightCommits: append([]json.RawMessage(nil), commits...)}
		if palette {
			p := so.LastPalette
			cp.Palette = &p
			cp.PaletteFrameStep = frameStep
		}
		if present != nil {
			modelOriginalCompose(t, &cp, so, ovs, fovs, dma)
		}
		res.Checkpoints = append(res.Checkpoints, cp)
	}
	for current < limit {
		next := min(current+100000, limit)
		var scans []session.ScanEvent
		if name == "s800f2" {
			for _, ev := range []struct {
				step uint64
				up   bool
			}{{8000100000, false}, {8000300000, true}} {
				if ev.step == current {
					scans = append(scans, session.ScanEvent{Scan: 0x3c, Break: ev.up})
				} else if ev.step > current && ev.step < next {
					next = ev.step
				}
			}
		}
		if checkpoints != nil && checkpointIndex < len(checkpoints) {
			next = min(next, checkpoints[checkpointIndex].Steps)
		}
		v, err := o.View()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = o.Deliver(session.CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration, ScanEvents: scans}); err != nil {
			t.Fatal(err)
		}
		r, err := o.Advance(session.InstructionBudget(next - current))
		if err != nil {
			t.Fatalf("%s Advance %d：%v", mode, r.MachineStepsAfter, err)
		}
		if r.MachineStepsBefore != current || r.MachineStepsAfter != next {
			t.Fatalf("%s 非預期停止：%d..%d，要 %d..%d", mode, r.MachineStepsBefore, r.MachineStepsAfter, current, next)
		}
		current = next
		if checkpoints != nil {
			if checkpointIndex < len(checkpoints) && current == checkpoints[checkpointIndex].Steps {
				snapshot()
				checkpointIndex++
			}
		} else if len(res.GroupJSON) != previousGroups {
			snapshot()
			previousGroups = len(res.GroupJSON)
			if previousGroups >= want {
				break
			}
		}
	}
	res.Stop = current
	res.RawFirstR7 = so.RawFirstR7
	if len(res.Checkpoints) == 0 || res.Checkpoints[len(res.Checkpoints)-1].Steps != current {
		snapshot()
	}
	if checkpoints != nil && checkpointIndex != len(checkpoints) {
		t.Fatalf("%s 漏檢查點", mode)
	}
	if rec != nil {
		res.Counts, res.Logs = rec.Counts(), rec.Logs()
		if len(res.GroupJSON) != want {
			res.Failures = append(res.Failures, fmt.Sprintf("%s 達步數上限只收 %d/%d 個升格群組", mode, len(res.GroupJSON), want))
		}
		modelOriginalGroups(t, dir, name, c, want, &res)
	}
	modelOriginalWrite(t, filepath.Join(dir, "checkpoints.json"), modelOriginalJSON(t, res.Checkpoints))
	modelOriginalWrite(t, filepath.Join(dir, "recorder-counts.json"), modelOriginalJSON(t, res.Counts))
	modelOriginalWrite(t, filepath.Join(dir, "logs.json"), modelOriginalJSON(t, res.Logs))
	modelOriginalWrite(t, filepath.Join(dir, "r7.json"), modelOriginalJSON(t, struct {
		RawFirstR7, FirstGroupR7, LegacyFirst, SampleFrom uint64
		LegacyOffset                                      int64
	}{res.RawFirstR7, res.FirstGroupR7, c.first, so.SampleFrom, int64(res.RawFirstR7) - int64(c.first)}))
	return res
}

func modelOriginalCompose(t *testing.T, cp *modelOriginalSnapshot, so *modelOriginalObserver, ovs map[int]*Overlay, fovs map[int]*FlightOverlay, dma []machine.SBDMAEvent) {
	t.Helper()
	if so.Frames == 0 || len(so.LastIndexed) != 64000 {
		t.Fatal("正式合成沒有已完成回掃")
	}
	cp.CompositeSHA256, cp.PlainSHA256 = map[int][32]byte{}, map[int][32]byte{}
	comps := map[int][]byte{}
	outs := map[int]scaleOut{}
	for _, sc := range []int{2, 3} {
		x := composeScale(scaleIn{Overlay: ovs[sc], Flight: fovs[sc], FlightV: so.Flight.Visible(), FlightG: so.Flight.Generation(), LabelsV: so.Labels.Visible(), LabelsG: so.Labels.Generation(), Scale: sc, Frame: so.LastIndexed, Palette: so.LastPalette, HD: nil})
		comps[sc] = x.Comp
		cp.PlainSHA256[sc] = sha256.Sum256(x.Plain)
		x.Comp, x.Plain = nil, nil
		outs[sc] = x
	}
	s := so.Subtitles
	shot, regions := drawSubtitlesChecked(s, []int{2, 3}, comps)
	for _, sc := range []int{2, 3} {
		cp.CompositeSHA256[sc] = sha256.Sum256(comps[sc])
	}
	current, showing := s.current()
	cp.Subtitles = modelOriginalJSON(t, struct {
		FrameStep                           uint64
		Enabled, Blocked, Disabled, Showing bool
		Current                             *Subtitle
		History                             []*Subtitle
		Skips                               []SubSkip
		Pending, SBDMA                      []machine.SBDMAEvent
		PrevKey                             string
		PrevAcc                             uint64
		Object                              struct {
			Valid          bool
			CS, SS, DS, DI uint16
		}
		Shot    SubShot
		Regions map[int]SubtitleRegionCheck
	}{s.FrameStep(), s.Enabled(), s.Blocked(), s.disabled, showing, current, s.Subs, s.Skips, s.pending, dma, s.prevKey, s.prevAcc, struct {
		Valid          bool
		CS, SS, DS, DI uint16
	}{s.obj.valid, s.obj.cs, s.obj.ss, s.obj.ds, s.obj.di}, shot, regions})
	cp.Composition = modelOriginalJSON(t, outs)
}

func modelOriginalGroups(t *testing.T, dir, name string, c modelOriginalCase, want int, res *modelOriginalResult) {
	t.Helper()
	var objs, faces, summary, rasterRows, rasterMismatches strings.Builder
	fmt.Fprintln(&objs, "group\tkey\tsteps\tend_steps\tc1\to1\tpassed\tmodels\txyz_compared\tscreen_compared")
	fmt.Fprintln(&faces, "group\tobject\tsi\tface_vt\tal\treturned\tvertices\tstatus\treason\tfirst_mismatch")
	fmt.Fprintln(&rasterRows, "group\tkey\tcalls\tvalid\tchecked\trepaintable\treplay_ready\tpixel_mismatch\tfirst_mismatch\tmissing_writer\tcall_kinds\tfaults")
	fmt.Fprintln(&rasterMismatches, "group\tkey\tindex\tx\ty\tbuf3d\treplayed\twriter\tcall_kind")
	models := map[string]int{}
	hist := map[int]int{}
	kinds := map[realspace.ModelKind][3]uint64{} // 物件數、XYZ、螢幕比對數。
	var xyz, screens, passed, failed, unchecked, unidentified, near, clipped, clamped, xyzMismatch, screenMismatch uint64
	var palettes []struct {
		Group                                                                int
		Key, GroupSteps, PromotionFrameStep, SnapshotSteps, PaletteFrameStep uint64
		Palette                                                              [256][3]uint8
	}
	for i, b := range res.GroupJSON {
		res.RawJSONBytes += uint64(len(b))
		res.StoredJSONBytes += modelOriginalWriteGroup(t, filepath.Join(dir, fmt.Sprintf("group-%03d.json", i+1)), b, res.RasterMode)
		var g ModelGroup
		if err := json.Unmarshal(b, &g); err != nil {
			t.Fatal(err)
		}
		bad := func(s string) { res.Failures = append(res.Failures, fmt.Sprintf("%s 群組%d：%s", name, i+1, s)) }
		for _, cp := range res.Checkpoints {
			if cp.Steps >= res.GroupSteps[i] && cp.Palette != nil {
				palettes = append(palettes, struct {
					Group                                                                int
					Key, GroupSteps, PromotionFrameStep, SnapshotSteps, PaletteFrameStep uint64
					Palette                                                              [256][3]uint8
				}{i + 1, g.Key, g.Steps, res.GroupSteps[i], cp.Steps, cp.PaletteFrameStep, *cp.Palette})
				break
			}
		}
		if i == 0 && g.Steps != res.RawFirstR7 {
			bad(fmt.Sprintf("首個R7 %d，不等於獨立原始命中 %d", g.Steps, res.RawFirstR7))
		}
		if len(g.Cameras) != 1 || len(g.Objects) != c.objects {
			bad(fmt.Sprintf("C1/O1=%d/%d，要1/%d", len(g.Cameras), len(g.Objects), c.objects))
		}
		if g.ReadFailed || g.UnmatchedFrames != 0 || len(g.Buf3D) != 64000 || len(g.List) == 0 || len(g.List) > 201 {
			bad("群組讀取、配對或Buf3D/列表不足")
		}
		if res.RasterMode {
			modelOriginalCheckRaster(t, i+1, &g, bad, &rasterRows, &rasterMismatches)
		} else if g.ReplayReady {
			bad("第一期不應宣稱012重播已就緒")
		}
		if len(g.CameraReadFailed) != len(g.Cameras) {
			bad("相機讀取結果筆數不符")
		}
		for _, f := range g.CameraReadFailed {
			if f {
				bad("相機讀取失敗")
			}
		}
		var groupXYZ, groupScreen uint64
		groupPassed := 0
		groupNames := map[string]bool{}
		for oi, o := range g.Objects {
			v := o.Verdict
			hist[len(v.Candidates)]++
			xyz += v.XYZCompared
			screens += v.ScreenCompared
			groupXYZ += v.XYZCompared
			groupScreen += v.ScreenCompared
			near += v.NearNoDivide
			clipped += v.ClipFlag
			clamped += v.Clamped
			xyzMismatch += v.XYZMismatch
			screenMismatch += v.ScreenMismatch
			if v.Unidentified {
				unidentified++
			}
			switch v.Status {
			case realspace.Passed:
				passed++
				groupPassed++
			case realspace.Failed:
				failed++
			default:
				unchecked++
			}
			if v.Status != realspace.Passed {
				bad(fmt.Sprintf("物件%d %s：%s", oi, v.Status, v.Reason))
			}
			if o.CamIdx < 0 || o.CamIdx >= len(g.Cameras) {
				bad(fmt.Sprintf("物件%d缺相機綁定", oi))
			} else if o.CamObj != g.Cameras[o.CamIdx].CamObj {
				bad(fmt.Sprintf("物件%d相機物件不符", oi))
			}
			if v.Selected != nil {
				models[v.Selected.Name]++
				groupNames[v.Selected.Name] = true
				kc := kinds[v.Selected.Kind]
				kc[0]++
				kc[1] += v.XYZCompared
				kc[2] += v.ScreenCompared
				kinds[v.Selected.Kind] = kc
			}
			for _, f := range o.Faces {
				fmt.Fprintf(&faces, "%d\t%d\t%d\t%04x\t%d\t%t\t%v\t%s\t%s\t%v\n", i+1, oi, f.SI, f.FaceVT, f.AL, f.Returned, f.VIdx, v.Status, v.Reason, v.First)
			}
		}
		names := make([]string, 0, len(groupNames))
		for n := range groupNames {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Fprintf(&objs, "%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\t%d\t%d\n", i+1, g.Key, g.Steps, g.EndSteps, len(g.Cameras), len(g.Objects), groupPassed, strings.Join(names, ","), groupXYZ, groupScreen)
		if res.FlightBaseValid[i] {
			found := false
			for _, cp := range res.Checkpoints {
				for _, raw := range cp.FlightCommits {
					var fc FlightCommit
					if err := json.Unmarshal(raw, &fc); err != nil {
						t.Fatal(err)
					}
					if fc.CUSteps == g.Key {
						found = true
					}
				}
			}
			if !found {
				bad(fmt.Sprintf("Flight.Base有效時Key %d未出現在FlightCommit", g.Key))
			}
		}
	}
	if want == 30 && (xyz != c.xyz || screens != c.screens) {
		res.Failures = append(res.Failures, fmt.Sprintf("%s 30群組比對數 %d/%d，要RE028 %d/%d", name, xyz, screens, c.xyz, c.screens))
	}
	if want == 30 && name == "n2" && (kinds[realspace.POLY] != [3]uint64{90, 14730, 14730} || kinds[realspace.APPR] != [3]uint64{30, 360, 360}) {
		res.Failures = append(res.Failures, fmt.Sprintf("n2 POLY/APPR 分項不符：%v", kinds))
	}
	if xyzMismatch != 0 || screenMismatch != 0 || unidentified != 0 {
		res.Failures = append(res.Failures, fmt.Sprintf("%s XYZ/螢幕不符或不可辨識非零：%d/%d/%d", name, xyzMismatch, screenMismatch, unidentified))
	}
	if res.Counts.SignatureFailures != 0 || res.Counts.ReadFailures != 0 || res.Counts.FaceRetMismatch != 0 || res.Counts.F1RUnpaired != 0 || res.Counts.UnmatchedFrame != 0 {
		res.Failures = append(res.Failures, fmt.Sprintf("%s 錄製錯誤計數非零：%+v", name, res.Counts))
	}
	fmt.Fprintln(&summary, "metric\tvalue")
	fmt.Fprintf(&summary, "raster_mode\t%t\njson_raw_bytes\t%d\njson_stored_bytes\t%d\n", res.RasterMode, res.RawJSONBytes, res.StoredJSONBytes)
	fmt.Fprintf(&summary, "legacy_first_r7\t%d\nraw_first_r7\t%d\nrecorded_first_r7\t%d\nlegacy_first_r7_offset\t%d\n", c.first, res.RawFirstR7, res.FirstGroupR7, int64(res.FirstGroupR7)-int64(c.first))
	for _, x := range []struct {
		n string
		v uint64
	}{{"groups", uint64(len(res.GroupJSON))}, {"passed", passed}, {"failed", failed}, {"unchecked", unchecked}, {"unidentified", unidentified}, {"xyz_compared", xyz}, {"screen_compared", screens}, {"xyz_mismatch", xyzMismatch}, {"screen_mismatch", screenMismatch}, {"near_no_divide", near}, {"clip_flag", clipped}, {"clamped", clamped}, {"stop_steps", res.Stop}} {
		fmt.Fprintf(&summary, "%s\t%d\n", x.n, x.v)
	}
	for _, k := range []realspace.ModelKind{realspace.POLY, realspace.APPR} {
		kc := kinds[k]
		fmt.Fprintf(&summary, "%s_objects\t%d\n%s_xyz_compared\t%d\n%s_screen_compared\t%d\n", k, kc[0], k, kc[1], k, kc[2])
	}
	var ns []string
	for n := range models {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	for _, n := range ns {
		fmt.Fprintf(&summary, "model:%s\t%d\n", n, models[n])
	}
	var hs []int
	for h := range hist {
		hs = append(hs, h)
	}
	sort.Ints(hs)
	for _, h := range hs {
		fmt.Fprintf(&summary, "compatible_set:%d\t%d\n", h, hist[h])
	}
	modelOriginalWrite(t, filepath.Join(dir, "objs.tsv"), []byte(objs.String()))
	modelOriginalWrite(t, filepath.Join(dir, "faces.tsv"), []byte(faces.String()))
	modelOriginalWrite(t, filepath.Join(dir, "summary.tsv"), []byte(summary.String()))
	modelOriginalWrite(t, filepath.Join(dir, "failures.json"), modelOriginalJSON(t, res.Failures))
	if len(palettes) != 0 {
		modelOriginalWrite(t, filepath.Join(dir, "palette-groups.json"), modelOriginalJSON(t, palettes))
	}
	if res.RasterMode {
		modelOriginalWrite(t, filepath.Join(dir, "raster.tsv"), []byte(rasterRows.String()))
		modelOriginalWrite(t, filepath.Join(dir, "raster-mismatch.tsv"), []byte(rasterMismatches.String()))
	}
	t.Logf("%s：群組 JSON 原始 %d bytes，儲存 %d bytes，光柵化模式 %t", dir, res.RawJSONBytes, res.StoredJSONBytes, res.RasterMode)
}

func modelOriginalCheckRaster(t *testing.T, group int, g *ModelGroup, bad func(string), rows, diffs *strings.Builder) {
	t.Helper()
	if g.Raster == nil {
		bad("沒有光柵化結果")
		return
	}
	r := g.Raster
	kinds := map[hd3d.CallKind]int{}
	seq := map[uint16]hd3d.CallKind{}
	for _, c := range g.Calls {
		kinds[c.Kind]++
		seq[c.Seq] = c.Kind
	}
	if len(g.Calls) == 0 {
		bad("光柵化呼叫為空")
	}
	if !r.Valid || !r.Checked || !r.Repaintable || !g.ReplayReady {
		bad(fmt.Sprintf("光柵化狀態 Valid/Checked/Repaintable/ReplayReady=%t/%t/%t/%t", r.Valid, r.Checked, r.Repaintable, g.ReplayReady))
	}
	mismatch, first, missing := 0, -1, 0
	for i, p := range r.Pixels {
		kind, found := seq[r.Writer[i]]
		if !r.Written[i] || !found {
			missing++
		}
		if len(g.Buf3D) == hd3d.PixelCount && p != g.Buf3D[i] {
			mismatch++
			if first < 0 {
				first = i
			}
			fmt.Fprintf(diffs, "%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n", group, g.Key, i, i%320, i/320, g.Buf3D[i], p, r.Writer[i], kind)
		}
	}
	if mismatch != 0 || missing != 0 {
		bad(fmt.Sprintf("光柵化逐點比對：不符%d、第一點%d、缺寫入者%d", mismatch, first, missing))
	}
	if r.Diagnostics.PixelMismatch != mismatch {
		bad(fmt.Sprintf("光柵化不符計數%d與獨立逐點比對%d不同", r.Diagnostics.PixelMismatch, mismatch))
	}
	if len(r.Diagnostics.Faults) != 0 || len(r.Diagnostics.Uninitialized) != 0 || r.Diagnostics.OutOfBounds != 0 || r.Diagnostics.CaptureShort != 0 || r.Diagnostics.Unwritten != 0 || len(g.RasterProblems) != 0 {
		bad(fmt.Sprintf("光柵化診斷錯誤：%+v；錄製問題%v", r.Diagnostics, g.RasterProblems))
	}
	kb, err := json.Marshal(kinds)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := json.Marshal(r.Diagnostics.Faults)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(rows, "%d\t%d\t%d\t%t\t%t\t%t\t%t\t%d\t%d\t%d\t%s\t%s\n", group, g.Key, len(g.Calls), r.Valid, r.Checked, r.Repaintable, g.ReplayReady, mismatch, first, missing, kb, fb)
}
