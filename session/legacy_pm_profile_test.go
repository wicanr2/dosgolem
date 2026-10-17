package session

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/448 §3.1 的單元測試：session.Config 的 JEMM／SB16／鍵盤攔截選項。

// 448 §3.1.1 的字面錨點：改動前（HEAD 9544c12）以零值 Config 加 audioLoopCOM、
// 4 回合各 10 圈（680 步）取得的 Digest。dos.New 搬到 setter 之後不得改變它。
const (
	legacyAnchorSteps  = 4 * 10 * audioLoopSteps
	legacyAnchorMemory = "74169ded434eef091c5f6184bba71018871fe35115239cc24568052a1e72b86d"
	legacyAnchorCPU    = "65451108a83781849bfdf0a2513f96adff749da8e8ae3fa8a146c69a766a3125"
)

// newConfigOwner 以指定 Config 建 owner，換掉 closer；不載入程式。
func newConfigOwner(t *testing.T, cfg Config) *Owner {
	t.Helper()
	if cfg.InitialScale == 0 {
		cfg.InitialScale = host.OutputScale2
	}
	o, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	o.closer = &countingCloser{}
	return o
}

// startConfigOwner 是 startSyntheticOwner 接受 Config 的版本（448 §3.1.5）：
// New(cfg) 之後 LoadCOM、startLoadedMachine、設 layout。
func startConfigOwner(t *testing.T, cfg Config, code []byte) *Owner {
	t.Helper()
	o := newConfigOwner(t, cfg)
	layout := sessionLayout(1, host.OutputScale2, false)
	if err := o.mouse.ApplyLayout(layout); err != nil {
		t.Fatal(err)
	}
	o.layout, o.layoutSet = layout, true
	if err := o.machine.LoadCOM(code); err != nil {
		t.Fatal(err)
	}
	if err := o.startLoadedMachine(); err != nil {
		t.Fatal(err)
	}
	return o
}

func TestLegacyConfigZeroValueAnchor(t *testing.T) {
	o := startSyntheticOwner(t, audioLoopCOM)
	for i := 0; i < 4; i++ {
		if r, err := runTurn(t, o, 10*audioLoopSteps); err != nil || r.Steps != 10*audioLoopSteps {
			t.Fatalf("%+v %v", r, err)
		}
	}
	d, err := o.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if d.Steps != legacyAnchorSteps {
		t.Fatalf("Steps %d，要 %d", d.Steps, legacyAnchorSteps)
	}
	if got := hex.EncodeToString(d.MemorySHA256[:]); got != legacyAnchorMemory {
		t.Fatalf("Memory SHA-256 %s，要 %s", got, legacyAnchorMemory)
	}
	if got := hex.EncodeToString(d.CPUSHA256[:]); got != legacyAnchorCPU {
		t.Fatalf("CPU SHA-256 %s，要 %s", got, legacyAnchorCPU)
	}
}

func TestLegacyConfigProgramPathSetsDir(t *testing.T) {
	o := newConfigOwner(t, Config{ProgramPath: `C:\GAME\X.EXE`})
	if o.machine.ProgramPath != `C:\GAME\X.EXE` {
		t.Fatalf("ProgramPath %q", o.machine.ProgramPath)
	}
	if o.dos.Dir != "GAME" {
		t.Fatalf("d.Dir %q，要 GAME", o.dos.Dir)
	}
}

func TestLegacyConfigProgramPathEmptyKeepsDefault(t *testing.T) {
	o := newConfigOwner(t, Config{})
	// newDOS 的預設目前目錄是 RICH2（internal/dos），空 ProgramPath 不得改它。
	if o.machine.ProgramPath != "" || o.dos.Dir != "RICH2" {
		t.Fatalf("ProgramPath %q Dir %q", o.machine.ProgramPath, o.dos.Dir)
	}
}

func TestLegacyConfigDOSFiles(t *testing.T) {
	if got := newConfigOwner(t, Config{}).machine.DOSFiles(); got != 0 {
		t.Fatalf("零值 DOSFiles %d", got)
	}
	if got := newConfigOwner(t, Config{DOSFiles: 30}).machine.DOSFiles(); got != 30 {
		t.Fatalf("DOSFiles %d，要 30", got)
	}
}

func TestLegacyConfigExtendedMemoryKB(t *testing.T) {
	if got := newConfigOwner(t, Config{}).machine.ExtendedMemoryKB(); got != 0 {
		t.Fatalf("零值 ExtendedMemoryKB %d", got)
	}
	if got := newConfigOwner(t, Config{ExtendedMemoryKB: 8192}).machine.ExtendedMemoryKB(); got != 8192 {
		t.Fatalf("ExtendedMemoryKB %d，要 8192", got)
	}
}

func TestLegacyConfigNoEMS(t *testing.T) {
	if !newConfigOwner(t, Config{}).machine.EMSPresent() {
		t.Fatal("零值時 EMS 應呈現")
	}
	if newConfigOwner(t, Config{NoEMS: true}).machine.EMSPresent() {
		t.Fatal("NoEMS 時 EMS 仍呈現")
	}
}

func TestLegacyConfigProtectedModeProfile(t *testing.T) {
	if newConfigOwner(t, Config{}).machine.ProtectedModeProfile() {
		t.Fatal("零值時 profile 已開")
	}
	if !newConfigOwner(t, Config{ExtendedMemoryKB: 8192, ProtectedModeProfile: true}).machine.ProtectedModeProfile() {
		t.Fatal("ProtectedModeProfile 未開")
	}
}

func TestLegacyConfigSoundBlaster(t *testing.T) {
	if newConfigOwner(t, Config{}).machine.SoundBlaster() {
		t.Fatal("零值時 SB 已開")
	}
	m := newConfigOwner(t, Config{SoundBlaster: true}).machine
	if !m.SoundBlaster() || !m.AdLibPresent() {
		t.Fatalf("SoundBlaster %v AdLib %v，都要真", m.SoundBlaster(), m.AdLibPresent())
	}
}

func TestLegacyConfigSBDMA8(t *testing.T) {
	if newConfigOwner(t, Config{SoundBlaster: true}).machine.SBDMA8() {
		t.Fatal("未要求時 SBDMA8 已開")
	}
	if !newConfigOwner(t, Config{SoundBlaster: true, SBDMA8: true}).machine.SBDMA8() {
		t.Fatal("SBDMA8 未開")
	}
}

func TestLegacyConfigMode13Phase(t *testing.T) {
	if newConfigOwner(t, Config{}).machine.Mode13Phase {
		t.Fatal("零值時 Mode13Phase 已開")
	}
	if !newConfigOwner(t, Config{Mode13Phase: true}).machine.Mode13Phase {
		t.Fatal("Mode13Phase 未開")
	}
}

func TestLegacyConfigExecEnvCopy(t *testing.T) {
	if newConfigOwner(t, Config{}).dos.ExecEnvCopy {
		t.Fatal("零值時 ExecEnvCopy 已開")
	}
	if !newConfigOwner(t, Config{ExecEnvCopy: true}).dos.ExecEnvCopy {
		t.Fatal("ExecEnvCopy 未開")
	}
}

func TestLegacyConfigExecReclaim(t *testing.T) {
	if newConfigOwner(t, Config{}).dos.ExecReclaim {
		t.Fatal("零值時 ExecReclaim 已開")
	}
	if !newConfigOwner(t, Config{ExecReclaim: true}).dos.ExecReclaim {
		t.Fatal("ExecReclaim 未開")
	}
}

// 447 §2.2 的客體碼與向量 9 的 stub（字面值，不取自被測程式）。
var (
	legacyKbdCode  = []byte{0x50, 0xE4, 0x60, 0xB4, 0x4F, 0xF9, 0xCD, 0x15, 0x73, 0x02, 0xCD, 0xFA, 0x58, 0xCF}
	legacyKbdStub9 = []byte{0xE9, 0x91, 0x04}
	legacyRawStub9 = []byte{0xCD, 0x09, 0xCF}
)

func readSupBytes(m *machine.Machine, at uint32, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = m.ReadSup8(at + uint32(i))
	}
	return out
}

func checkKbdIntercept(t *testing.T, stage string, m *machine.Machine, on bool) {
	t.Helper()
	base := uint32(machine.StubSeg) * 16
	stub := readSupBytes(m, base+uint32(machine.StubOff(0x09)), 3)
	if on {
		want := machine.KbdInterceptStub9()
		if string(stub) != string(legacyKbdStub9) || string(stub) != string(want[:]) {
			t.Fatalf("%s：stub % X，要 % X", stage, stub, legacyKbdStub9)
		}
		if code := readSupBytes(m, base+uint32(machine.KbdInterceptOff), len(legacyKbdCode)); string(code) != string(legacyKbdCode) {
			t.Fatalf("%s：客體碼 % X，要 % X", stage, code, legacyKbdCode)
		}
	} else if string(stub) != string(legacyRawStub9) {
		t.Fatalf("%s：stub % X，要 % X", stage, stub, legacyRawStub9)
	}
	if m.KbdIntercept != on {
		t.Fatalf("%s：KbdIntercept %v，要 %v", stage, m.KbdIntercept, on)
	}
}

func TestLegacyConfigKbdIntercept(t *testing.T) {
	for _, on := range []bool{false, true} {
		o := newConfigOwner(t, Config{KbdIntercept: on})
		checkKbdIntercept(t, "New 之後", o.machine, on)
		exe := minimalBootMZ(t)
		if _, err := o.BootOriginal(BootInput{EXE: exe, ExpectedEXESHA256: sha256.Sum256(exe), SaveRoot: t.TempDir()}); err != nil {
			t.Fatal(err)
		}
		checkKbdIntercept(t, "BootOriginal 之後", o.machine, on)
	}
}

func TestLegacyConfigErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"SBDMA8 無 SoundBlaster", Config{SBDMA8: true}},
		{"DOSFiles 5", Config{DOSFiles: 5}},
		{"ProtectedModeProfile 無延伸記憶體", Config{ProtectedModeProfile: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.cfg.InitialScale = host.OutputScale2
			o, err := New(tc.cfg)
			if err == nil || o != nil {
				t.Fatalf("New = (%v, %v)，要 (nil, 錯誤)", o, err)
			}
		})
	}
}

func TestLegacyConfigSoundBlasterMutesOPL(t *testing.T) {
	o := startConfigOwner(t, Config{SoundBlaster: true}, audioLoopCOM)
	for i := 0; i < 3; i++ {
		if r, err := runTurn(t, o, audioLoopSteps); err != nil || r.Steps != audioLoopSteps {
			t.Fatalf("%+v %v", r, err)
		}
	}
	if n := len(o.machine.OPL); n != 0 {
		t.Fatalf("m.OPL %d 筆，要 0", n)
	}
}
