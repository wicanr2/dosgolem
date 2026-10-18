package session

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/state"
)

// docs/spec/450 §3.2：session 從狀態檔啟動。
//
// 合成程式（.COM，CS=DS=PSPSeg）依序做前半、計數 synthLoopK 次的空轉迴圈、後半，
// 每次 DOS 呼叫的 AX 與進位旗標寫進 synthSlot(i)（AX 在 +0，CF 在 +2；未寫時是 0xEE 哨兵）：
//
//	槽 0：開 X　槽 1：讀 X 2 bytes 到 0540h（前半）
//	槽 2：讀 X 2 bytes 到 0544h（後半）　槽 3：開 V　槽 4：開 W
const (
	synthLoopK = 3000
	synthSaveN = 1000 // 落在迴圈中段
	synthMoreM = 8000 // 讀檔後再跑的步數，足以離開迴圈做完後半
	synthSlots = 5
)

func synthCOM() []byte {
	img := make([]byte, 0x0560-0x100)
	put := func(off int, b ...byte) { copy(img[off-0x100:], b) }
	for i := 0x0500; i < 0x0500+4*synthSlots; i++ {
		img[i-0x100] = 0xEE
	}
	put(0x0400, 'X', 0, 'V', 0, 'W', 0)
	put(0x0540, 0xEE, 0xEE, 0xEE, 0xEE, 0xEE, 0xEE, 0xEE, 0xEE)
	var code []byte
	emit := func(b ...byte) { code = append(code, b...) }
	// record 把 AX 與 CF 寫進槽 i。mov 不改旗標，所以先存 AX 再以 adc 取 CF。
	record := func(i int) {
		a := 0x0500 + 4*i
		emit(0xA3, byte(a), byte(a>>8))       // mov [a],ax
		emit(0xB0, 0x00, 0x14, 0x00)          // mov al,0 / adc al,0
		emit(0xA2, byte(a+2), byte((a+2)>>8)) // mov [a+2],al
	}
	openName := func(name, slot int) {
		emit(0xBA, byte(name), byte(name>>8)) // mov dx,name
		emit(0xB8, 0x00, 0x3D)                // mov ax,3D00h
		emit(0xCD, 0x21)
		record(slot)
	}
	readX := func(buf, slot int) {
		emit(0x8B, 0x1E, 0x00, 0x05)        // mov bx,[0500h]
		emit(0xB9, 0x02, 0x00)              // mov cx,2
		emit(0xBA, byte(buf), byte(buf>>8)) // mov dx,buf
		emit(0xB4, 0x3F)                    // mov ah,3Fh
		emit(0xCD, 0x21)
		record(slot)
	}
	openName(0x0400, 0)
	readX(0x0540, 1)
	k := uint16(synthLoopK)
	emit(0xB9, byte(k), byte(k>>8)) // mov cx,K
	emit(0xE2, 0xFE)                // loop $
	readX(0x0544, 2)
	openName(0x0402, 3)
	openName(0x0404, 4)
	emit(0xEB, 0xFE) // jmp $
	if len(code) > 0x0300 {
		panic("合成程式太長")
	}
	put(0x0100, code...)
	return img
}

func synthAddr(off int) uint32 { return uint32(machine.PSPSeg)*16 + uint32(off) }

// synthSlot 回槽 i 的 AX 與 CF 位元組。
func synthSlot(m *machine.Machine, i int) (uint16, uint8) {
	a := synthAddr(0x0500 + 4*i)
	return uint16(m.Peek8(a)) | uint16(m.Peek8(a+1))<<8, m.Peek8(a + 2)
}

type synthSave struct {
	m     *machine.Machine
	d     *dos.DOS
	state []byte
	sum   [32]byte
}

// saveSynth 是存檔端：machine.New、pre、LoadCOM、dos.New(root)、Scratch、Install、post、
// RunUntil(nil, synthSaveN)、state.Save。
func saveSynth(t *testing.T, root, scratch string, pre func(*machine.Machine) error, post func(*machine.Machine, *dos.DOS)) synthSave {
	t.Helper()
	m := machine.New()
	if pre != nil {
		if err := pre(m); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.LoadCOM(synthCOM()); err != nil {
		t.Fatal(err)
	}
	d := dos.New(m, root)
	d.Scratch = scratch
	d.Install()
	t.Cleanup(d.Close)
	if post != nil {
		post(m, d)
	}
	if _, err := m.RunUntil(nil, synthSaveN); err != nil {
		t.Fatal(err)
	}
	if m.Steps != synthSaveN {
		t.Fatalf("存檔端步數 %d，要 %d", m.Steps, synthSaveN)
	}
	// N 在迴圈中段：前半做完、後半還沒做。
	if ax, cf := synthSlot(m, 1); cf != 0 || ax != 2 {
		t.Fatalf("存檔時前半讀 X 回 AX=%d CF=%d", ax, cf)
	}
	if ax, cf := synthSlot(m, 2); ax != 0xEEEE || cf != 0xEE {
		t.Fatalf("存檔時後半已經跑了：槽 2＝%04X/%02X", ax, cf)
	}
	p := filepath.Join(t.TempDir(), "s.state")
	if err := state.Save(p, m, d); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return synthSave{m: m, d: d, state: raw, sum: sha256.Sum256(raw)}
}

func synthTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func stateConfig() Config {
	return Config{InitialScale: host.OutputScale2, InitialLayout: sessionLayout(1, host.OutputScale2, false)}
}

func newStateOwner(t *testing.T, cfg Config) *Owner {
	t.Helper()
	o, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { o.Close() })
	return o
}

// stateTurn 照 §3.2：每次 Advance 之前 View、Deliver 空 update。
func stateTurn(t *testing.T, o *Owner, budget InstructionBudget) TickReceipt {
	t.Helper()
	v, err := o.View()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.Deliver(CapturedUpdate{StartedPanel: v.Panel, Layout: v.Layout, SourceGeneration: v.SourceGeneration}); err != nil {
		t.Fatal(err)
	}
	r, err := o.Advance(budget)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func dosStateDigest(t *testing.T, d *dos.DOS) [32]byte {
	t.Helper()
	var buf bytes.Buffer
	if err := d.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	sum, err := dos.StateDigest(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func synthFiles() map[string]string { return map[string]string{"X": "abcdefgh", "W": "w"} }

// §3.2 第 1 項：前置檢查回錯、phase 不變。
func TestBootFromStateRejectsWithoutFailing(t *testing.T) {
	root := synthTree(t, synthFiles())
	good := saveSynth(t, root, root, nil, nil)
	file := filepath.Join(root, "X")
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	var zero [32]byte
	in := func(mut func(*StateInput)) StateInput {
		s := StateInput{State: good.state, ExpectedStateSHA256: good.sum, SaveRoot: root}
		mut(&s)
		return s
	}
	cases := []struct {
		name  string
		cfg   func(*Config)
		input StateInput
	}{
		{"State 為空", nil, in(func(s *StateInput) { s.State = nil })},
		{"State 超過上限", nil, in(func(s *StateInput) { s.State = make([]byte, maxBootStateSize+1) })},
		{"期望值全零", nil, in(func(s *StateInput) { s.ExpectedStateSHA256 = zero })},
		{"雜湊不符", nil, in(func(s *StateInput) { s.ExpectedStateSHA256 = sha256.Sum256([]byte("nope")) })},
		{"ClockPercent 50", func(c *Config) { c.ClockPercent = 50 }, in(func(*StateInput) {})},
		{"ClockPercent 100", func(c *Config) { c.ClockPercent = 100 }, in(func(*StateInput) {})},
		{"SaveRoot 為空", nil, in(func(s *StateInput) { s.SaveRoot = "" })},
		{"SaveRoot 含 NUL", nil, in(func(s *StateInput) { s.SaveRoot = root + "\x00" })},
		{"SaveRoot 不存在", nil, in(func(s *StateInput) { s.SaveRoot = filepath.Join(root, "missing") })},
		{"SaveRoot 是 symlink", nil, in(func(s *StateInput) { s.SaveRoot = link })},
		{"SaveRoot 是檔案", nil, in(func(s *StateInput) { s.SaveRoot = file })},
	}
	if os.Geteuid() != 0 {
		locked := filepath.Join(t.TempDir(), "locked")
		if err := os.Mkdir(locked, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(locked, 0o755) })
		cases = append(cases, struct {
			name  string
			cfg   func(*Config)
			input StateInput
		}{"SaveRoot 不可寫", nil, in(func(s *StateInput) { s.SaveRoot = locked })})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := stateConfig()
			if tc.cfg != nil {
				tc.cfg(&cfg)
			}
			o := newStateOwner(t, cfg)
			steps, mem := o.machine.Steps, sha256.Sum256(o.machine.Mem)
			if _, err := o.BootFromState(tc.input); err == nil {
				t.Fatal("預期回錯")
			}
			if got := o.Status(); got.Phase != PhaseBooting || got.FirstFault != nil {
				t.Fatalf("前置檢查改了 phase：%+v", got)
			}
			if o.machine.Steps != steps || sha256.Sum256(o.machine.Mem) != mem {
				t.Fatalf("前置檢查改了機器：步數 %d→%d", steps, o.machine.Steps)
			}
		})
	}
	t.Run("nil owner", func(t *testing.T) {
		var o *Owner
		if _, err := o.BootFromState(StateInput{}); err == nil {
			t.Fatal("預期回錯")
		}
	})
	t.Run("非 Booting 的 owner", func(t *testing.T) {
		o := newStateOwner(t, stateConfig())
		if _, err := o.BootFromState(in(func(*StateInput) {})); err != nil {
			t.Fatal(err)
		}
		steps := o.machine.Steps
		if _, err := o.BootFromState(in(func(*StateInput) {})); err == nil {
			t.Fatal("第二次開機應回錯")
		}
		if got := o.Status(); got.Phase != PhaseRunning || got.FirstFault != nil || o.machine.Steps != steps {
			t.Fatalf("第二次開機改了 owner：%+v", got)
		}
	})
}

// truncatedGzip 把狀態檔解開、截掉後半、再壓回：gzip 合法、內容截斷。
func truncatedGzip(t *testing.T, raw []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(body[:len(body)/2]); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// §3.2 第 2 項：讀檔失敗，owner 進 PhaseFailed。
func TestBootFromStateLoadFailureFailsOwner(t *testing.T) {
	root := synthTree(t, synthFiles())
	plain := saveSynth(t, root, root, nil, nil)
	trunc := truncatedGzip(t, plain.state)
	for _, tc := range []struct {
		name  string
		state []byte
		pre   func(*machine.Machine) error
		cfg   func(*Config)
	}{
		{"內容截斷", trunc, nil, nil},
		{"延伸記憶體不同", nil, func(m *machine.Machine) error {
			return m.SetExtendedMemoryKB(4096)
		}, nil},
		{"NoEMS 不同", nil, func(m *machine.Machine) error {
			return m.SetEMSPresent(false)
		}, nil},
		{"PM profile 不同", nil, func(m *machine.Machine) error {
			if err := m.SetExtendedMemoryKB(4096); err != nil {
				return err
			}
			return m.SetProtectedModeProfile(true)
		}, func(c *Config) { c.ExtendedMemoryKB = 4096 }},
		{"SBDMA8 不同", nil, func(m *machine.Machine) error {
			m.SetSoundBlaster(true)
			return m.SetSBDMA8(true)
		}, func(c *Config) { c.SoundBlaster = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := tc.state
			if st == nil {
				st = saveSynth(t, root, root, tc.pre, nil).state
			}
			cfg := stateConfig()
			if tc.cfg != nil {
				tc.cfg(&cfg)
			}
			o := newStateOwner(t, cfg)
			if _, err := o.BootFromState(StateInput{State: st, ExpectedStateSHA256: sha256.Sum256(st), SaveRoot: root}); err == nil {
				t.Fatal("預期讀檔失敗")
			} else {
				t.Logf("讀檔錯誤：%v", err)
			}
			if got := o.Status(); got.Phase != PhaseFailed || got.FirstFault == nil {
				t.Fatalf("讀檔失敗後 %+v，要 PhaseFailed", got)
			}
			if o.dos.RebaseTo != "" || o.dos.RebaseFrom != nil {
				t.Errorf("RebaseTo=%q RebaseFrom=%q，應清空", o.dos.RebaseTo, o.dos.RebaseFrom)
			}
		})
	}
}

// firstStepObserver 記下第一次 BeforeStep 的步數。
type firstStepObserver struct {
	first uint64
	seen  bool
}

func (f *firstStepObserver) BeforeStep(v StepView) error {
	if !f.seen {
		f.first, f.seen = v.Steps(), true
	}
	return nil
}
func (f *firstStepObserver) VideoWrite(machine.VideoWrite) {}
func (f *firstStepObserver) Frame([]byte, [256][3]uint8)   {}

// §3.2 第 3 項：往返與收據。存檔端的 Root、Scratch 與讀檔端的 SaveRoot 是同一個暫存目錄。
func TestBootFromStateRoundTrip(t *testing.T) {
	root := synthTree(t, synthFiles())
	save := saveSynth(t, root, root, nil, nil)
	obs := &firstStepObserver{}
	cfg := stateConfig()
	cfg.Observer = obs
	o := newStateOwner(t, cfg)
	caller := append([]byte(nil), save.state...)
	receipt, err := o.BootFromState(StateInput{State: caller, ExpectedStateSHA256: save.sum, SaveRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	for i := range caller { // 呼叫端改自己的 slice 不影響 owner
		caller[i] = 0
	}
	var zero [32]byte
	if receipt.Phase != PhaseRunning || receipt.Steps != synthSaveN || receipt.StateSHA256 != save.sum ||
		receipt.EXESHA256 != zero || receipt.LegacyState {
		t.Fatalf("收據 %+v", receipt)
	}
	if o.dos.RebaseTo != "" || o.dos.RebaseFrom != nil {
		t.Errorf("RebaseTo=%q RebaseFrom=%q，應清空", o.dos.RebaseTo, o.dos.RebaseFrom)
	}
	if o.dos.Root != root || o.dos.Scratch != root {
		t.Errorf("Root=%q Scratch=%q，要 %q", o.dos.Root, o.dos.Scratch, root)
	}
	r := stateTurn(t, o, synthMoreM)
	if r.MachineStepsBefore != synthSaveN || r.MachineStepsAfter != synthSaveN+synthMoreM {
		t.Fatalf("回合 %+v", r)
	}
	if !obs.seen || obs.first != synthSaveN {
		t.Fatalf("觀察器第一次 BeforeStep 的 Steps=%d（seen=%t），要 %d", obs.first, obs.seen, synthSaveN)
	}
	if _, err := save.m.RunUntil(nil, synthMoreM); err != nil {
		t.Fatal(err)
	}
	got, err := o.Digest()
	if err != nil {
		t.Fatal(err)
	}
	want := digestOf(save.m, save.d)
	if got.Steps != want.Steps || got.MemorySHA256 != want.MemorySHA256 || got.CPUSHA256 != want.CPUSHA256 ||
		got.IndexedSHA256 != want.IndexedSHA256 || got.PaletteSHA256 != want.PaletteSHA256 {
		t.Fatalf("續跑後摘要不同：\nsession %+v\n存檔端 %+v", got, want)
	}
	if a, b := dosStateDigest(t, o.dos), dosStateDigest(t, save.d); a != b {
		t.Fatal("續跑後 dos 的 StateDigest 不同")
	}
	// 後半確實跑過：讀 X 成功、開 W 成功。
	if ax, cf := synthSlot(o.machine, 2); ax != 2 || cf != 0 {
		t.Fatalf("後半讀 X 回 AX=%d CF=%d", ax, cf)
	}
	if _, cf := synthSlot(o.machine, 4); cf != 0 {
		t.Fatalf("後半開 W 失敗")
	}
	// BootOriginal 的收據三個新欄位為零值。
	ob := bootTestOwner(t)
	defer ob.Close()
	exe := minimalBootMZ(t)
	br, err := ob.BootOriginal(BootInput{EXE: exe, ExpectedEXESHA256: sha256.Sum256(exe), SaveRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if br.StateSHA256 != zero || br.Steps != 0 || br.LegacyState {
		t.Fatalf("BootOriginal 收據 %+v", br)
	}
}

// §3.2 第 4 項：讀檔後 Root 是 SaveRoot，不是狀態檔的 Root。
func TestBootFromStateRootIsSaveRoot(t *testing.T) {
	a := synthTree(t, map[string]string{"X": "abcdefgh", "V": "only-in-A"})
	saveRoot := synthTree(t, map[string]string{"X": "abcdefgh", "W": "only-in-save-root"})
	save := saveSynth(t, a, "", nil, nil)
	o := newStateOwner(t, stateConfig())
	if _, err := o.BootFromState(StateInput{State: save.state, ExpectedStateSHA256: save.sum, SaveRoot: saveRoot}); err != nil {
		t.Fatal(err)
	}
	stateTurn(t, o, synthMoreM)
	if ax, cf := synthSlot(o.machine, 2); ax != 2 || cf != 0 {
		t.Fatalf("後半讀 X 回 AX=%d CF=%d", ax, cf)
	}
	if ax, cf := synthSlot(o.machine, 3); ax != 2 || cf != 1 {
		t.Errorf("開只在舊 Root 的 V：AX=%d CF=%d，要 AX=2 CF=1", ax, cf)
	}
	if ax, cf := synthSlot(o.machine, 4); cf != 0 {
		t.Errorf("開只在 SaveRoot 的 W：AX=%d CF=%d，要成功", ax, cf)
	}
}

// §3.2 第 5 項：存檔時滑鼠鍵按著。
func TestBootFromStateRejectsHeldMouseButton(t *testing.T) {
	root := synthTree(t, synthFiles())
	save := saveSynth(t, root, root, nil, func(_ *machine.Machine, d *dos.DOS) { d.Mouse.Buttons = 1 })
	o := newStateOwner(t, stateConfig())
	if _, err := o.BootFromState(StateInput{State: save.state, ExpectedStateSHA256: save.sum, SaveRoot: root}); err == nil {
		t.Fatal("預期回錯")
	}
	if got := o.Status(); got.Phase != PhaseFailed {
		t.Fatalf("phase %+v，要 PhaseFailed", got)
	}
}

// kbdBytes 讀向量 9 的 stub（3 bytes）與 KbdInterceptOff 的客體碼（14 bytes）。
func kbdBytes(m *machine.Machine) ([]byte, []byte) {
	base := uint32(machine.StubSeg) * 16
	read := func(at uint32, n int) []byte {
		out := make([]byte, n)
		for i := range out {
			out[i] = m.ReadSup8(at + uint32(i))
		}
		return out
	}
	return read(base+uint32(machine.StubOff(0x09)), 3), read(base+uint32(machine.KbdInterceptOff), 14)
}

// §3.2 第 6 項：鍵盤攔截。
func TestBootFromStateKbdIntercept(t *testing.T) {
	root := synthTree(t, synthFiles())
	ref := machine.New()
	if err := ref.EnableKbdIntercept(); err != nil {
		t.Fatal(err)
	}
	wantStub, wantCode := kbdBytes(ref)

	t.Run("開啟、讀關閉時存的狀態檔", func(t *testing.T) {
		save := saveSynth(t, root, root, nil, nil)
		if stub, _ := kbdBytes(save.m); bytes.Equal(stub, wantStub) {
			t.Fatal("前提不成立：關閉時存的狀態檔 stub 已是攔截版")
		}
		cfg := stateConfig()
		cfg.KbdIntercept = true
		o := newStateOwner(t, cfg)
		if _, err := o.BootFromState(StateInput{State: save.state, ExpectedStateSHA256: save.sum, SaveRoot: root}); err != nil {
			t.Fatal(err)
		}
		stub, code := kbdBytes(o.machine)
		if !bytes.Equal(stub, wantStub) || !bytes.Equal(code, wantCode) {
			t.Fatalf("讀檔後 stub % X、客體碼 % X，要 % X、% X", stub, code, wantStub, wantCode)
		}
		if !o.machine.KbdIntercept {
			t.Fatal("KbdIntercept 應為真")
		}
	})
	t.Run("關閉、讀開啟時存的狀態檔", func(t *testing.T) {
		save := saveSynth(t, root, root, nil, func(m *machine.Machine, _ *dos.DOS) {
			if err := m.EnableKbdIntercept(); err != nil {
				t.Fatal(err)
			}
		})
		o := newStateOwner(t, stateConfig())
		if _, err := o.BootFromState(StateInput{State: save.state, ExpectedStateSHA256: save.sum, SaveRoot: root}); err == nil {
			t.Fatal("預期回錯")
		}
		if got := o.Status(); got.Phase != PhaseFailed {
			t.Fatalf("phase %+v，要 PhaseFailed", got)
		}
	})
}
