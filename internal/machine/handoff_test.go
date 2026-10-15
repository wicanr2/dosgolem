package machine

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/cpu386"
)

// docs/spec/433-real-mode-system-instructions §8 第 5 項：交棒。

// stubPM 是 PMCore 替身：記下 Enter 收到的 Handoff 與 Step 次數。
type stubPM struct {
	entered  []Handoff
	enterErr error
	steps    int
}

func (s *stubPM) Enter(h Handoff) error {
	s.entered = append(s.entered, h)
	return s.enterErr
}

func (s *stubPM) Step() error { s.steps++; return nil }

// handoffProg：mov cr0,eax（EAX=1，PE 0→1）；之後是 nop。
func handoffProg(t *testing.T) *Machine {
	t.Helper()
	m := newSysProg(t, []byte{0x0F, 0x22, 0xC0, 0x90, 0x90})
	m.CPU.R[cpu.AX], m.CPU.Hi[cpu.AX] = 0x0001, 0
	return m
}

func TestHandoffNoPMCoreHalts(t *testing.T) {
	m := handoffProg(t)
	cs, ip := m.CPU.Seg[cpu.CS], m.CPU.IP
	steps := m.Steps
	err := m.Step()
	if !errors.Is(err, ErrPMCoreNotAvailable) {
		t.Fatalf("m.pm 為 nil：應回 ErrPMCoreNotAvailable，實際 %v", err)
	}
	if m.Steps != steps+1 {
		t.Errorf("交棒指令本身照常計步：Steps %d→%d", steps, m.Steps)
	}
	// 診斷含起點與下一道位址、CR0。
	for _, want := range []string{
		strings.ToUpper(hex4(cs) + ":" + hex4(ip)),
		strings.ToUpper(hex4(cs) + ":" + hex4(ip+3)),
		"CR0=00000011",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("診斷應含 %q：%v", want, err)
		}
	}
	if m.CPU.IP != ip+3 || m.CR0()&cr0PE == 0 {
		t.Errorf("IP 應已越過 mov cr0（+%d），CR0=%08X", m.CPU.IP-ip, m.CR0())
	}
	// 之後每次 Step 都回同一個錯誤，既有核心不再執行。
	for i := 0; i < 3; i++ {
		if err2 := m.Step(); err2 != err {
			t.Errorf("第 %d 次再 Step：應回同一個錯誤值，實際 %v", i+2, err2)
		}
	}
	if m.CPU.IP != ip+3 || m.Steps != steps+1 {
		t.Errorf("停擺後不得再執行：IP +%d、Steps %d", m.CPU.IP-ip, m.Steps)
	}
}

func hex4(v uint16) string {
	const d = "0123456789ABCDEF"
	return string([]byte{d[v>>12], d[v>>8&15], d[v>>4&15], d[v&15]})
}

// TestHandoffRestoreRejudge：Restore 到實模式快照後清除停擺，到 PE=1、VM=0 的快照時重新設起
// （m.pm 為 nil 與非 nil 各一例）；LoadState 同理。
func TestHandoffRestoreRejudge(t *testing.T) {
	for _, withPM := range []bool{false, true} {
		m := handoffProg(t)
		realSnap := m.Snapshot()
		realState := saveBytes(t, m)
		stub := &stubPM{}
		if withPM {
			m.setPMCore(stub)
		}
		err := m.Step()
		if withPM {
			mustNil(t, "Enter 成功的那一步", err)
			if !m.entered || m.mode != modePM {
				t.Fatalf("withPM：應進入 PM 分派（entered=%v mode=%d）", m.entered, m.mode)
			}
		} else if !errors.Is(err, ErrPMCoreNotAvailable) {
			t.Fatalf("m.pm 為 nil：%v", err)
		}
		pmSnap := m.Snapshot() // PE=1、VM=0
		pmState := saveBytes(t, m)

		m.Restore(realSnap)
		if m.halt != nil || m.entered || m.mode != modeNormal {
			t.Errorf("withPM=%v：Restore 到實模式快照後應清除停擺與 entered", withPM)
		}
		m.Restore(pmSnap)
		if !errors.Is(m.halt, ErrPMCoreNotAvailable) || m.entered || m.mode != modeHalt {
			t.Errorf("withPM=%v：Restore 到 PE=1、VM=0 的快照應重新設起停擺（halt=%v entered=%v）", withPM, m.halt, m.entered)
		}
		stepsBefore := stub.steps
		if err := m.Step(); !errors.Is(err, ErrPMCoreNotAvailable) {
			t.Errorf("withPM=%v：重新設起後 Step 應回 ErrPMCoreNotAvailable，實際 %v", withPM, err)
		}
		if stub.steps != stepsBefore {
			t.Errorf("withPM=%v：停擺先於分派，不得呼叫替身的 Step", withPM)
		}

		// LoadState 同樣重新判定。
		mustNil(t, "LoadState(實模式)", m.LoadState(bytes.NewReader(realState)))
		if m.halt != nil || m.entered || m.mode != modeNormal {
			t.Errorf("withPM=%v：LoadState 實模式後應清除", withPM)
		}
		mustNil(t, "LoadState(PE=1)", m.LoadState(bytes.NewReader(pmState)))
		if !errors.Is(m.halt, ErrPMCoreNotAvailable) || m.entered {
			t.Errorf("withPM=%v：LoadState 到 PE=1、VM=0 應重新設起停擺", withPM)
		}
	}
	// PE=1、VM=1（V86）的快照不設停擺。
	m := newSysProg(t, []byte{0x90})
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	mustNil(t, "SetVM", m.SetVM(true))
	snap := m.Snapshot()
	m.Restore(snap)
	if m.halt != nil {
		t.Error("PE=1、VM=1 的快照不應設停擺")
	}
}

func TestHandoffEnterWithStub(t *testing.T) {
	m := handoffProg(t)
	c := m.CPU
	// 每個暫存器高低半都可辨識。
	for i := 0; i < 8; i++ {
		c.R[i], c.Hi[i] = uint16(0x1000+i), uint16(0xA000+i)
	}
	c.R[cpu.AX], c.Hi[cpu.AX] = 0x0001, 0xA000 // mov cr0,eax：EAX 低 bit 是 PE
	c.Seg[cpu.ES], c.Seg[cpu.SS], c.Seg[cpu.DS] = 0x1111, 0x3333, 0x4444
	c.FSeg, c.GSeg = 0x5555, 0x6666
	cs := c.Seg[cpu.CS]
	// mov cr0 不碰記憶體運算元；SS:SP 隨意。
	c.SetFlags(cpu.IF | cpu.CF | cpu.ZF)
	mustNil(t, "SetEFlagsHi", m.SetEFlagsHi(eflagsHiRF))
	stub := &stubPM{}
	m.setPMCore(stub)
	steps := m.Steps
	ip := c.IP

	mustNil(t, "Enter 成功的那一步", m.Step())
	if m.Steps != steps+1 {
		t.Errorf("Enter 成功的那一步 Steps 加 1：%d→%d", steps, m.Steps)
	}
	if len(stub.entered) != 1 {
		t.Fatalf("Enter 應被呼叫一次，實際 %d", len(stub.entered))
	}
	h := stub.entered[0]
	// 暫存器含高半（cpu386 順序 EAX…EDI 與既有核心 AX…DI 相同）。
	wantR := [8]uint32{}
	for i := 0; i < 8; i++ {
		wantR[i] = uint32(0xA000+i)<<16 | uint32(0x1000+i)
	}
	wantR[cpu386.EAX] = 0xA0000001
	if h.R != wantR {
		t.Errorf("Handoff.R = %08X，應為 %08X", h.R, wantR)
	}
	if h.EIP != uint32(ip+3) {
		t.Errorf("EIP = %X，應為下一道 %X", h.EIP, ip+3)
	}
	if h.EFlags != 0x00010000|uint32(c.Flags) || h.EFlags&uint32(cpu.IF|cpu.CF|cpu.ZF) != uint32(cpu.IF|cpu.CF|cpu.ZF) {
		t.Errorf("EFlags = %08X（含 EFlagsHi 的 RF）", h.EFlags)
	}
	wantSeg := [6]uint16{}
	wantSeg[cpu386.SegCS], wantSeg[cpu386.SegDS], wantSeg[cpu386.SegES] = cs, 0x4444, 0x1111
	wantSeg[cpu386.SegFS], wantSeg[cpu386.SegGS], wantSeg[cpu386.SegSS] = 0x5555, 0x6666, 0x3333
	if h.Seg != wantSeg {
		t.Errorf("Handoff.Seg = %04X，應為 %04X", h.Seg, wantSeg)
	}
	for i, s := range wantSeg {
		attr := uint16(0x0093)
		if i == cpu386.SegCS {
			attr = 0x009B
		}
		if want := (SegCache{Base: uint32(s) << 4, Limit: 0xFFFF, Attr: attr}); h.Cache[i] != want {
			t.Errorf("Cache[%d] = %+v，應為 %+v", i, h.Cache[i], want)
		}
	}
	// 之後的 Step 呼叫替身的 Step，Steps 照舊加 1，既有核心不動。
	ip2 := c.IP
	for i := 0; i < 3; i++ {
		mustNil(t, "PM 分派", m.Step())
	}
	if stub.steps != 3 || m.Steps != steps+4 || c.IP != ip2 {
		t.Errorf("PM 分派：替身 Step %d 次、Steps +%d、IP 變化 %d", stub.steps, m.Steps-steps, c.IP-ip2)
	}
}

func TestHandoffEnterErrorHalts(t *testing.T) {
	m := handoffProg(t)
	boom := errors.New("替身 Enter 失敗")
	stub := &stubPM{enterErr: boom}
	m.setPMCore(stub)
	err := m.Step()
	if !errors.Is(err, ErrPMCoreNotAvailable) || !errors.Is(err, boom) {
		t.Fatalf("Enter 失敗：errors.Is 應同時判得原錯誤與 ErrPMCoreNotAvailable，實際 %v", err)
	}
	if err2 := m.Step(); err2 != err {
		t.Errorf("停擺後應回同一個錯誤：%v", err2)
	}
	if stub.steps != 0 || m.entered {
		t.Error("Enter 失敗不得分派")
	}
}

func TestHandoffSetCR0WithoutEnterDoesNotDispatch(t *testing.T) {
	m := newSysProg(t, []byte{0x90, 0x90})
	stub := &stubPM{}
	m.setPMCore(stub)
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	mustStep(t, m, "nop（PE=1，未經 Enter）")
	if stub.steps != 0 || len(stub.entered) != 0 || m.mode != modeNormal {
		t.Errorf("經 SetCR0 直接設 PE=1 而未 Enter：不得呼叫替身（steps=%d enter=%d）", stub.steps, len(stub.entered))
	}
}

// pfSys 包住 SysBus：SetCR 之後注入一筆分頁紀錄（模擬同一步的分頁錯誤）。
type pfSys struct{ *Machine }

func (p pfSys) SetCR(n int, v uint32) error {
	err := p.Machine.SetCR(n, v)
	p.Machine.notePageFault(0x12345678, 0, 0, true, false)
	return err
}

func TestHandoffPageFaultAndSentinel(t *testing.T) {
	for _, withPM := range []bool{false, true} {
		m := handoffProg(t)
		stub := &stubPM{}
		if withPM {
			m.setPMCore(stub)
		}
		m.CPU.Sys = pfSys{m}
		err := m.Step()
		var pfe *PageFaultError
		if !errors.As(err, &pfe) || !errors.Is(err, cpu.ErrModeSwitch) {
			t.Fatalf("withPM=%v：同一步有分頁紀錄時回 *PageFaultError（包住哨兵），實際 %v", withPM, err)
		}
		if len(stub.entered) != 0 {
			t.Errorf("withPM=%v：哨兵不交棒", withPM)
		}
		if err := m.Step(); !errors.Is(err, ErrPMCoreNotAvailable) {
			t.Errorf("withPM=%v：之後的 Step 回停擺錯誤，實際 %v", withPM, err)
		}
	}
}

func TestSegmentMappingRoundTrip(t *testing.T) {
	c := cpu.New(nil)
	c.Seg[cpu.ES], c.Seg[cpu.CS], c.Seg[cpu.SS], c.Seg[cpu.DS] = 0x1111, 0x2222, 0x3333, 0x4444
	c.FSeg, c.GSeg = 0x5555, 0x6666
	s := segsToPM(c)
	want := [6]uint16{}
	want[cpu386.SegES], want[cpu386.SegCS], want[cpu386.SegSS] = 0x1111, 0x2222, 0x3333
	want[cpu386.SegDS], want[cpu386.SegFS], want[cpu386.SegGS] = 0x4444, 0x5555, 0x6666
	if s != want {
		t.Errorf("segsToPM = %04X，應為 %04X", s, want)
	}
	d := cpu.New(nil)
	segsFromPM(s, d)
	if d.Seg != c.Seg || d.FSeg != c.FSeg || d.GSeg != c.GSeg {
		t.Errorf("segsFromPM 往返：%04X %04X %04X", d.Seg, d.FSeg, d.GSeg)
	}
	// 負對照：CS 與 SS 對調的映射會被上面的比較抓到。
	bad := s
	bad[cpu386.SegCS], bad[cpu386.SegSS] = bad[cpu386.SegSS], bad[cpu386.SegCS]
	if bad == want {
		t.Error("負對照：CS 與 SS 對調後應與期望不同")
	}
}
