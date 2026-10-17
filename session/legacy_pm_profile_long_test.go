package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/448 §3.2 的觸發案例：以 SCCD.EXE 冷開機 10 億步，與 cmd/probe 的結果對拍。
//
// 長測試。只在環境變數 DOSGOLEM_SC_ROOT 指向《Strike Commander》的 SB16 設定樹時執行，
// 否則 t.Skip。每個子測試把該樹實體複製到 t.TempDir()（遊戲會寫入 Root＝Scratch，
// 子測試之間不得共用），每次約 5 到 10 分鐘，逐一以
// `go test ./session -run 'TestLegacyPMProfileTrigger/<子測試>' -timeout 30m -v` 執行。
//
// 子測試：
//   - KbdOn、KbdOff：§3.2 的基準，三項斷言都要符合期望值。
//   - Off_<欄位>：§3.2 的負對照，關掉一個新欄位，只記錄三項斷言哪些改變（t.Log），不判失敗。

const (
	legacyTriggerEnv   = "DOSGOLEM_SC_ROOT"
	legacyTriggerSteps = 1000000000
	legacySCCDSHA256   = "5dbfe18c528f51b9b1b0c989c1578cb297ba56c64038be7ca9d903dd88e93d01"
	// workplace/root/SC.CFG（SB16 設定，含 -ma -ds）的完整 SHA-256，規格 §3.2 記的是 317b0abd…34e36f。
	legacySCCFGSHA256     = "317b0abdbf538f52d95e13b7358712e6da30a1a6d6e312e78d4943254b34e36f"
	legacyTriggerIndexed  = "4d752df6353106aced440d235109abee038776ca2096953ce86c208e405e2148"
	legacyTriggerPalette  = "ea48da3ee0628b0cf7609482ed5946e7ae78939b8c45d2fb0850a8bb566a5b60"
	legacyTriggerRegsWant = "CS:IP 060F:034A AX 0000 BX 0000 CX 2042 DX FB54 DS 250B ES 250B SS:SP 250B:FAD6"
)

// legacyRegObserver 只記錄：BeforeStep 看到 Steps == legacyTriggerSteps 時記下暫存器。
type legacyRegObserver struct {
	regs string
	seen bool
}

func (r *legacyRegObserver) BeforeStep(v StepView) error {
	if v.Steps() == legacyTriggerSteps && !r.seen {
		r.seen = true
		r.regs = fmt.Sprintf("CS:IP %04X:%04X AX %04X BX %04X CX %04X DX %04X DS %04X ES %04X SS:SP %04X:%04X",
			v.CS(), v.IP(), v.AX(), v.BX(), v.CX(), v.DX(), v.DS(), v.ES(), v.SS(), v.SP())
	}
	return nil
}
func (r *legacyRegObserver) VideoWrite(machine.VideoWrite) {}
func (r *legacyRegObserver) Frame([]byte, [256][3]uint8)   {}

// legacyTriggerConfig 是 §3.2 的完整 Config（KbdIntercept 由呼叫端決定）。
func legacyTriggerConfig(obs StepObserver, kbd bool) Config {
	return Config{
		InitialScale:         host.OutputScale2,
		InitialLayout:        sessionLayout(1, host.OutputScale2, false),
		Observer:             obs,
		ProgramPath:          `C:\SCCD.EXE`,
		DOSFiles:             30,
		ExtendedMemoryKB:     8192,
		NoEMS:                true,
		ProtectedModeProfile: true,
		ExecEnvCopy:          true,
		ExecReclaim:          true,
		Mode13Phase:          true,
		SoundBlaster:         true,
		AdLib:                true,
		SBDMA8:               true,
		KbdIntercept:         kbd,
	}
}

// copyLegacyTree 把 src 實體複製到 dst（不用硬連結）。
func copyLegacyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		switch {
		case info.IsDir():
			return os.MkdirAll(out, 0o755)
		case !info.Mode().IsRegular():
			return fmt.Errorf("複製樹內有非一般檔案：%s", p)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, in); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	})
}

func fileSHA256(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type legacyTriggerResult struct {
	advanceErr error
	steps      uint64
	indexed    string
	palette    string
	regs       string
}

// runLegacyTrigger 依 §3.2 的步驟執行一次，回傳三項斷言的實測值。設定失敗才 t.Fatal。
func runLegacyTrigger(t *testing.T, cfg Config, obs *legacyRegObserver) legacyTriggerResult {
	t.Helper()
	src := os.Getenv(legacyTriggerEnv)
	root := t.TempDir()
	if err := copyLegacyTree(src, root); err != nil {
		t.Fatal(err)
	}
	if got := fileSHA256(t, filepath.Join(root, "SC.CFG")); got != legacySCCFGSHA256 {
		t.Fatalf("SC.CFG SHA-256 %s，要 %s", got, legacySCCFGSHA256)
	}
	exe, err := os.ReadFile(filepath.Join(root, "SCCD.EXE"))
	if err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	if _, err := hex.Decode(want[:], []byte(legacySCCDSHA256)); err != nil {
		t.Fatal(err)
	}
	o, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if _, err := o.BootOriginal(BootInput{EXE: exe, ExpectedEXESHA256: want, SaveRoot: root}); err != nil {
		t.Fatal(err)
	}
	var res legacyTriggerResult
	turn := func(budget InstructionBudget) (TickReceipt, error) {
		v, err := o.View()
		if err != nil {
			return TickReceipt{}, err
		}
		if _, err := o.Deliver(CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration}); err != nil {
			return TickReceipt{}, err
		}
		return o.Advance(budget)
	}
	r, err := turn(legacyTriggerSteps)
	res.steps = r.MachineStepsAfter
	if err != nil {
		res.advanceErr = err
		return res
	}
	d, err := o.Digest()
	if err != nil {
		t.Fatal(err)
	}
	res.indexed = hex.EncodeToString(d.IndexedSHA256[:])
	res.palette = hex.EncodeToString(d.PaletteSHA256[:])
	if _, err := turn(1); err != nil {
		res.advanceErr = err
	}
	res.regs = obs.regs
	return res
}

func TestLegacyPMProfileTrigger(t *testing.T) {
	if os.Getenv(legacyTriggerEnv) == "" {
		t.Skip(legacyTriggerEnv + " 未設定：§3.2 觸發案例需要原版素材的複製樹")
	}
	for _, kbd := range []struct {
		name string
		on   bool
	}{{"KbdOn", true}, {"KbdOff", false}} {
		t.Run(kbd.name, func(t *testing.T) {
			obs := &legacyRegObserver{}
			res := runLegacyTrigger(t, legacyTriggerConfig(obs, kbd.on), obs)
			t.Logf("Advance err=%v steps=%d indexed=%s palette=%s regs=%s", res.advanceErr, res.steps, res.indexed, res.palette, res.regs)
			if res.advanceErr != nil {
				t.Fatalf("Advance：%v", res.advanceErr)
			}
			if res.steps != legacyTriggerSteps {
				t.Fatalf("MachineStepsAfter %d，要 %d", res.steps, legacyTriggerSteps)
			}
			if res.indexed != legacyTriggerIndexed {
				t.Errorf("IndexedSHA256 %s，要 %s", res.indexed, legacyTriggerIndexed)
			}
			if res.palette != legacyTriggerPalette {
				t.Errorf("PaletteSHA256 %s，要 %s", res.palette, legacyTriggerPalette)
			}
			if res.regs != legacyTriggerRegsWant {
				t.Errorf("暫存器 %q，要 %q", res.regs, legacyTriggerRegsWant)
			}
		})
	}
	offs := []struct {
		name string
		off  func(*Config)
	}{
		{"ProgramPath", func(c *Config) { c.ProgramPath = "" }},
		{"DOSFiles", func(c *Config) { c.DOSFiles = 0 }},
		{"ExtendedMemoryKB", func(c *Config) { c.ExtendedMemoryKB = 0; c.ProtectedModeProfile = false }},
		{"NoEMS", func(c *Config) { c.NoEMS = false }},
		{"ProtectedModeProfile", func(c *Config) { c.ProtectedModeProfile = false }},
		{"SoundBlaster", func(c *Config) { c.SoundBlaster = false; c.SBDMA8 = false }},
		{"SBDMA8", func(c *Config) { c.SBDMA8 = false }},
		{"Mode13Phase", func(c *Config) { c.Mode13Phase = false }},
		{"ExecEnvCopy", func(c *Config) { c.ExecEnvCopy = false }},
		{"ExecReclaim", func(c *Config) { c.ExecReclaim = false }},
		{"KbdIntercept", func(c *Config) { c.KbdIntercept = false }},
	}
	for _, tc := range offs {
		t.Run("Off_"+tc.name, func(t *testing.T) {
			obs := &legacyRegObserver{}
			cfg := legacyTriggerConfig(obs, true)
			tc.off(&cfg)
			res := runLegacyTrigger(t, cfg, obs)
			changed := func(ok bool) string {
				if res.advanceErr != nil || !ok {
					return "有改變"
				}
				return "未改變"
			}
			t.Logf("Advance err=%v steps=%d indexed=%s palette=%s regs=%s", res.advanceErr, res.steps, res.indexed, res.palette, res.regs)
			t.Logf("負對照 %s：Indexed %s，Palette %s，暫存器 %s", tc.name,
				changed(res.indexed == legacyTriggerIndexed),
				changed(res.palette == legacyTriggerPalette),
				changed(res.regs == legacyTriggerRegsWant))
		})
	}
}
