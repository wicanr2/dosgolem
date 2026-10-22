package session

// 陸空戰將速度基準（perf-cpu 分支）：從狀態檔固定跑 N 步，量每秒步數，並輸出決定性錨點。
// 沒有設 DOSGOLEM_PERF_STATE 時 skip，不影響一般 go test。
//
// 環境變數：
//   DOSGOLEM_PERF_STATE   狀態檔路徑（probe -save-state）
//   DOSGOLEM_PERF_ROOT    SB16 設定樹（唯讀，複製到暫存目錄當 SaveRoot）
//   DOSGOLEM_PERF_STEPS   推進步數（預設 20000000）
//   DOSGOLEM_PERF_CHUNK   每次 Advance 的步數（0＝一次跑完）
//   DOSGOLEM_PERF_OBS     1＝裝空觀察器（走 runObserved，接近前端路徑）
//
// 錨點：Digest（記憶體、CPU 16 位元暫存器、indexed、palette、Ext）與
// machine.SaveState＋dos.SaveState 未壓縮串流的 SHA-256（含保護模式不透明欄位）。

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

type perfNullObserver struct{}

func (perfNullObserver) BeforeStep(StepView) error      { return nil }
func (perfNullObserver) VideoWrite(machine.VideoWrite)  {}
func (perfNullObserver) Frame([]uint8, [256][3]uint8) {}

// perfCPUTime 是本行程的使用者加系統 CPU 時間（getrusage）。主機負載浮動時牆鐘不穩，CPU 時間較能比較。
func perfCPUTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

func perfEnvU64(t *testing.T, k string, def uint64) uint64 {
	s := os.Getenv(k)
	if s == "" {
		return def
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		t.Fatalf("%s：%v", k, err)
	}
	return v
}

func perfCopyTree(t *testing.T, src, dst string) {
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		d := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(d, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(d)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSCPerfBench(t *testing.T) {
	statePath := os.Getenv("DOSGOLEM_PERF_STATE")
	root := os.Getenv("DOSGOLEM_PERF_ROOT")
	if statePath == "" || root == "" {
		t.Skip("DOSGOLEM_PERF_STATE／DOSGOLEM_PERF_ROOT 未設")
	}
	steps := perfEnvU64(t, "DOSGOLEM_PERF_STEPS", 20_000_000)
	chunk := perfEnvU64(t, "DOSGOLEM_PERF_CHUNK", 0)
	if chunk == 0 {
		chunk = steps
	}
	st, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	// 固定路徑：DOS 狀態記著 SaveRoot，暫存目錄名每次不同會讓 full 雜湊不決定。
	saveRoot := filepath.Join(os.TempDir(), "sc-perf-root")
	if err := os.RemoveAll(saveRoot); err != nil {
		t.Fatal(err)
	}
	perfCopyTree(t, root, saveRoot)

	const s = 2
	cfg := Config{
		InitialScale: host.OutputScale2,
		InitialLayout: host.MouseLayout{Epoch: 1, Scale: host.OutputScale2, ChromeHeight: 18 * s,
			Canvas: host.Canvas{Width: 320, Height: 200}, FrameWidth: 320 * s, FrameHeight: 18*s + 200*s},
		ProgramPath: `C:\SCCD.EXE`, DOSFiles: 30, ExtendedMemoryKB: 8192, NoEMS: true,
		ProtectedModeProfile: true, ExecEnvCopy: true, ExecReclaim: true, Mode13Phase: true,
		SoundBlaster: true, AdLib: true, SBDMA8: true, KbdIntercept: true,
	}
	if os.Getenv("DOSGOLEM_PERF_OBS") == "1" {
		cfg.Observer = perfNullObserver{}
	}
	o, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	boot, err := o.BootFromState(StateInput{State: st, ExpectedStateSHA256: sha256.Sum256(st), SaveRoot: saveRoot, RebaseFrom: []string{"/scratch"}})
	if err != nil {
		t.Fatal(err)
	}
	start := boot.Steps
	var elapsed, cpuTime time.Duration
	for done := uint64(0); done < steps; {
		n := chunk
		if steps-done < n {
			n = steps - done
		}
		v, err := o.View()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := o.Deliver(CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration}); err != nil {
			t.Fatal(err)
		}
		t0, u0 := time.Now(), perfCPUTime()
		r, err := o.Advance(InstructionBudget(n))
		elapsed += time.Since(t0)
		cpuTime += perfCPUTime() - u0
		if err != nil {
			t.Fatalf("Advance 停在第 %d 步：%v", r.MachineStepsAfter, err)
		}
		done += r.Steps
	}
	d, err := o.Digest()
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	if err := o.machine.SaveState(h); err != nil {
		t.Fatal(err)
	}
	if err := o.dos.SaveState(h); err != nil {
		t.Fatal(err)
	}
	full := hex.EncodeToString(h.Sum(nil))
	// cpu32：Digest 沒蓋的 386 與保護模式欄位（Hi、FS/GS、六個段快取、CR0/CR3/EFlagsHi/CPL、FPUCW、Cycles）。
	c := o.machine.CPU
	var b []byte
	for _, v := range c.Hi {
		b = binary.LittleEndian.AppendUint16(b, v)
	}
	b = binary.LittleEndian.AppendUint16(b, c.FSeg)
	b = binary.LittleEndian.AppendUint16(b, c.GSeg)
	b = binary.LittleEndian.AppendUint16(b, c.FPUCW)
	b = binary.LittleEndian.AppendUint64(b, c.Cycles)
	if c.Segmented() {
		b = append(b, 1)
	} else {
		b = append(b, 0)
	}
	for i := cpu.ES; i <= cpu.GS; i++ {
		sc := c.SegCache(i)
		b = binary.LittleEndian.AppendUint32(b, sc.Base)
		b = binary.LittleEndian.AppendUint32(b, sc.Limit)
		b = binary.LittleEndian.AppendUint16(b, sc.Attr)
	}
	b = binary.LittleEndian.AppendUint32(b, o.machine.CR0())
	b = binary.LittleEndian.AppendUint32(b, o.machine.CR3())
	b = binary.LittleEndian.AppendUint16(b, o.machine.EFlagsHi())
	b = append(b, o.machine.PMCPL())
	cpu32 := sha256.Sum256(b)
	sps := float64(steps) / elapsed.Seconds()
	t.Logf("PERF state=%s start=%d end=%d steps=%d chunk=%d obs=%q elapsed=%.3fs steps/s=%.0f cputime=%.3fs",
		filepath.Base(statePath), start, d.Steps, steps, chunk, os.Getenv("DOSGOLEM_PERF_OBS"), elapsed.Seconds(), sps, cpuTime.Seconds())
	t.Logf("HASH mem=%x cpu=%x cpu32=%x idx=%x pal=%x ext=%s cycles=%d full=%s",
		d.MemorySHA256, d.CPUSHA256, cpu32, d.IndexedSHA256, d.PaletteSHA256, d.ExtSHA256, o.machine.CPU.Cycles, full)
}
