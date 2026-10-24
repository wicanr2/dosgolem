//go:build linux

package strikecommander

// 013 §3.14：原版固定步數效能報告，沒有數值通過門檻。
// SC_MODEL_PERF_ROOT、SC_MODEL_PERF_STATES、SC_MODEL_PERF_REPORTS 與
// SC_MODEL_PERF_OUT 全部設定才執行。輸出檔必須不存在；只量 Advance。
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/apps/strikecommander/hd3d"
	"github.com/wicanr2/dosgolem/apps/strikecommander/realspace"
	"github.com/wicanr2/dosgolem/session"
)

type modelPerfSnapshot struct {
	Steps         uint64
	Digest        session.StateDigest
	MachineDigest session.FullDigest
	Frames        int
	FlightCommits []FlightCommit
}

type modelPerfRun struct {
	Case, Mode, SaveRoot, LoadBefore, LoadAfter string
	Round                                       int
	UserSeconds, SystemSeconds, WallSeconds     float64
	Groups, ReplayReadyGroups                   int
	Reads                                       map[uint16]ModelReadCount
	GroupReads                                  []map[uint16]ModelReadCount
	CheckpointSHA256                            []string
	Frames, FlightCommits                       int
	StateEqualToA                               bool
}

type modelPerfCaseReport struct {
	Case, StateSHA256, CheckpointsSHA256 string
	Start, Stop                          uint64
	Checkpoints                          []uint64
	A2MinusARangeSeconds                 [2]float64
	BMinusAMedianSeconds                 float64
	CMinusAMedianSeconds                 float64
}

func modelPerfUsage(t *testing.T) (float64, float64) {
	t.Helper()
	var u syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &u); err != nil {
		t.Fatal(err)
	}
	return float64(u.Utime.Sec) + float64(u.Utime.Usec)/1e6, float64(u.Stime.Sec) + float64(u.Stime.Usec)/1e6
}

func modelPerfLoad() string {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return err.Error()
	}
	return strings.TrimSpace(string(b))
}

func modelPerfCloneCommit(c FlightCommit) FlightCommit {
	c.Texts = append([]FlightText(nil), c.Texts...)
	for i := range c.Texts {
		x := &c.Texts[i]
		x.Xs = append([]int(nil), x.Xs...)
		x.Before = append([]byte(nil), x.Before...)
		x.After = append([]byte(nil), x.After...)
		x.Ref = append([]byte(nil), x.Ref...)
		if x.Clip != nil {
			v := *x.Clip
			x.Clip = &v
		}
	}
	return c
}

func modelPerfRunOriginal(t *testing.T, root, saveRoot, name, mode string, round int, raw []byte, lib *realspace.Library, tables hd3d.Tables, targets []uint64) (modelPerfRun, []modelPerfSnapshot) {
	t.Helper()
	modelOriginalRestore(t, root, saveRoot)
	res := modelPerfRun{Case: name, Mode: mode, Round: round, SaveRoot: saveRoot, Reads: map[uint16]ModelReadCount{}}
	c := modelOriginalCases[name]
	cats := flightTestCats()
	fo := NewFlightObserver(func(k Kind, s string) (string, int, bool) { return ResolveFlight(k, s, cats) })
	var commits []FlightCommit
	fo.OnCommit = func(fc FlightCommit) { commits = append(commits, modelPerfCloneCommit(fc)) }
	so := &SessionObserver{Labels: NewLabelObserver(), Flight: fo}
	var rec *ModelRecorder
	if mode == "B" {
		rec = NewModelRecorder(lib)
	} else if mode == "C" {
		rec = NewSceneRecorder(lib, tables)
	}
	if rec != nil {
		so.Models = rec
		rec.OnGroup = func(g *ModelGroup) {
			if g.Steps < modelOriginalSampleFrom(name, c) {
				return
			}
			res.Groups++
			if g.ReplayReady {
				res.ReplayReadyGroups++
			}
			reads := make(map[uint16]ModelReadCount, len(g.Reads))
			for ip, n := range g.Reads {
				reads[ip] = n
				x := res.Reads[ip]
				x.Calls, x.Bytes = x.Calls+n.Calls, x.Bytes+n.Bytes
				res.Reads[ip] = x
			}
			res.GroupReads = append(res.GroupReads, reads)
		}
	}
	cfg := SessionConfig(so)
	cfg.SparseObserve = false
	o, err := session.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	boot, err := o.BootFromState(session.StateInput{State: raw, ExpectedStateSHA256: sha256.Sum256(raw), SaveRoot: saveRoot, RebaseFrom: []string{"/scratch"}})
	if err != nil || boot.Steps != c.start {
		t.Fatalf("%s/%s boot=%d：%v", name, mode, boot.Steps, err)
	}
	var snapshots []modelPerfSnapshot
	current, targetIndex := boot.Steps, 0
	res.LoadBefore = modelPerfLoad()
	for current < targets[len(targets)-1] {
		next := min(current+100000, targets[targetIndex])
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
		v, err := o.View()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = o.Deliver(session.CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration, ScanEvents: scans}); err != nil {
			t.Fatal(err)
		}
		u0, s0 := modelPerfUsage(t)
		wall := time.Now()
		r, err := o.Advance(session.InstructionBudget(next - current))
		res.WallSeconds += time.Since(wall).Seconds()
		u1, s1 := modelPerfUsage(t)
		res.UserSeconds += u1 - u0
		res.SystemSeconds += s1 - s0
		if err != nil || r.MachineStepsBefore != current || r.MachineStepsAfter != next {
			t.Fatalf("%s/%s Advance %d..%d，要 %d..%d：%v", name, mode, r.MachineStepsBefore, r.MachineStepsAfter, current, next, err)
		}
		current = next
		if current == targets[targetIndex] {
			d, err := o.Digest()
			if err != nil {
				t.Fatal(err)
			}
			md, err := o.MachineDigest()
			if err != nil {
				t.Fatal(err)
			}
			snapshots = append(snapshots, modelPerfSnapshot{current, d, md, so.Frames, append([]FlightCommit(nil), commits...)})
			targetIndex++
		}
	}
	res.LoadAfter = modelPerfLoad()
	res.Frames, res.FlightCommits = so.Frames, len(commits)
	if rec != nil && (res.Groups != 20 || (mode == "C" && res.ReplayReadyGroups != 20) || (mode == "B" && res.ReplayReadyGroups != 0)) {
		t.Fatalf("%s/%s groups=%d ReplayReady=%d，要 20/%t", name, mode, res.Groups, res.ReplayReadyGroups, mode == "C")
	}
	return res, snapshots
}

func TestModelPhase1Performance(t *testing.T) {
	root, states, reports, out := os.Getenv("SC_MODEL_PERF_ROOT"), os.Getenv("SC_MODEL_PERF_STATES"), os.Getenv("SC_MODEL_PERF_REPORTS"), os.Getenv("SC_MODEL_PERF_OUT")
	if root == "" || states == "" || reports == "" || out == "" {
		t.Skip("requires original fixtures and SC_MODEL_PERF_* paths")
	}
	if runtime.GOMAXPROCS(0) != 1 {
		t.Fatal("效能量測要求 GOMAXPROCS=1")
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		t.Fatal("效能報告已存在，或輸出路徑無法檢查")
	}
	logPath := strings.TrimSuffix(out, filepath.Ext(out)) + "-samples.jsonl"
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	exe, err := os.ReadFile(filepath.Join(root, "STRIKE.EXE"))
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(exe)) != "883270cd0038fe571376202233b8abf5efb77889e27f4f0caf97dfab7e529cb5" {
		t.Fatalf("非指定原版 EXE：%v", err)
	}
	runtime.GC()
	var mem0, mem1 runtime.MemStats
	runtime.ReadMemStats(&mem0)
	u0, s0 := modelPerfUsage(t)
	loadStart := time.Now()
	lib, err := realspace.LoadLibrary(root)
	loadWall := time.Since(loadStart).Seconds()
	u1, s1 := modelPerfUsage(t)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&mem1)
	tables, err := hd3d.TablesFromEXE(exe)
	if err != nil {
		t.Fatal(err)
	}
	report := struct {
		GoVersion, TimingScope, ReadScope, InterfaceLimit, LoadBefore, CPUInfo, CPUSet string
		LibraryWallSeconds, LibraryCPUSeconds                                          float64
		LibraryHeapAllocDelta, LibrarySysDelta                                         int64
		LibraryTotalAllocDelta                                                         uint64
		Library                                                                        realspace.LibraryStats
		Cases                                                                          []modelPerfCaseReport
		Runs                                                                           []modelPerfRun
	}{GoVersion: runtime.Version(), TimingScope: "sum of Getrusage(RUSAGE_SELF) user+sys around each Advance; excludes restore/copy/boot/Deliver/checkpoint digests/JSON/output; GOMAXPROCS=1; SparseObserve=false; 100000-step chunks",
		ReadScope:      "ModelGroup.Reads: confirmed data-hit ReadBlock requests by raw IP, includes wrap splits and failed requests; excludes signature/CU/voided hits; per group and total",
		InterfaceLimit: "B=013 NewModelRecorder; C=012+013 NewSceneRecorder (includes 1x replay self-check). Public interface does not offer pure 012, so pure 012 vs 012+013 was not measured. No numeric pass threshold.",
		LoadBefore:     modelPerfLoad(), LibraryWallSeconds: loadWall, LibraryCPUSeconds: u1 + s1 - u0 - s0,
		LibraryHeapAllocDelta: int64(mem1.HeapAlloc) - int64(mem0.HeapAlloc), LibrarySysDelta: int64(mem1.Sys) - int64(mem0.Sys), LibraryTotalAllocDelta: mem1.TotalAlloc - mem0.TotalAlloc, Library: lib.Stats()}
	if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "model name") {
				report.CPUInfo = line
				break
			}
		}
	}
	if b, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "Cpus_allowed_list:") {
				report.CPUSet = line
			}
		}
	}
	for _, tc := range []struct {
		name, file, reference string
		stop                  uint64
	}{{"s760", "s760.state", "report-007", 7643000000}, {"s800f2", "s800.state", "report-009", 8009600000}} {
		raw, err := os.ReadFile(filepath.Join(states, tc.file))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != modelOriginalCases[tc.name].sha {
			t.Fatalf("%s state hash：%v", tc.name, err)
		}
		cpRaw, err := os.ReadFile(filepath.Join(reports, tc.reference, "B", "checkpoints.json"))
		if err != nil {
			t.Fatal(err)
		}
		var cp []struct{ Steps uint64 }
		if err = json.Unmarshal(cpRaw, &cp); err != nil || len(cp) < 20 || cp[19].Steps != tc.stop {
			t.Fatalf("%s 非固定的第20檢查點：%v", tc.name, err)
		}
		cr := modelPerfCaseReport{Case: tc.name, Start: modelOriginalCases[tc.name].start, Stop: tc.stop, StateSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), CheckpointsSHA256: fmt.Sprintf("%x", sha256.Sum256(cpRaw))}
		for _, x := range cp[:20] {
			if x.Steps <= cr.Start || x.Steps > cr.Stop || (x.Steps-cr.Start)%100000 != 0 || (len(cr.Checkpoints) > 0 && x.Steps <= cr.Checkpoints[len(cr.Checkpoints)-1]) {
				t.Fatal("檢查點並非原樣100000步分塊")
			}
			cr.Checkpoints = append(cr.Checkpoints, x.Steps)
		}
		saveRoot := filepath.Join(t.TempDir(), "save")
		var baseline []modelPerfSnapshot
		var aa, ba, ca []float64
		for round := 1; round <= 5; round++ {
			cpuByMode := map[string]float64{}
			for _, mode := range []string{"A", "A2", "B", "C"} {
				r, snapshots := modelPerfRunOriginal(t, root, saveRoot, tc.name, mode, round, raw, lib, tables, cr.Checkpoints)
				if baseline == nil {
					baseline = snapshots
				}
				r.StateEqualToA = reflect.DeepEqual(baseline, snapshots)
				if !r.StateEqualToA {
					t.Fatalf("%s round=%d %s：Digest/MachineDigest/Frames/FlightCommits 與首輪A不同", tc.name, round, mode)
				}
				for _, snap := range snapshots {
					b, err := json.Marshal(snap)
					if err != nil {
						t.Fatal(err)
					}
					r.CheckpointSHA256 = append(r.CheckpointSHA256, fmt.Sprintf("%x", sha256.Sum256(b)))
				}
				cpu := r.UserSeconds + r.SystemSeconds
				cpuByMode[mode] = cpu
				report.Runs = append(report.Runs, r)
				if err := json.NewEncoder(log).Encode(r); err != nil {
					t.Fatal(err)
				}
				t.Logf("case=%s round=%d mode=%s cpu=%.6fs wall=%.6fs groups=%d replay=%d checkpoints=20 equal_A=true load=%s", tc.name, round, mode, cpu, r.WallSeconds, r.Groups, r.ReplayReadyGroups, r.LoadAfter)
			}
			aa = append(aa, cpuByMode["A2"]-cpuByMode["A"])
			ba = append(ba, cpuByMode["B"]-cpuByMode["A"])
			ca = append(ca, cpuByMode["C"]-cpuByMode["A"])
		}
		sort.Float64s(aa)
		sort.Float64s(ba)
		sort.Float64s(ca)
		cr.A2MinusARangeSeconds = [2]float64{aa[0], aa[4]}
		cr.BMinusAMedianSeconds, cr.CMinusAMedianSeconds = ba[2], ca[2]
		report.Cases = append(report.Cases, cr)
		t.Logf("case=%s A2-A range=%v B-A median=%.6fs C-A median=%.6fs", tc.name, cr.A2MinusARangeSeconds, cr.BMinusAMedianSeconds, cr.CMinusAMedianSeconds)
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(out, append(b, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
