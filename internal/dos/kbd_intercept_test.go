package dos

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/447-bios-int09-int15-keyboard-intercept §3 第 2、3 項：BIOS int 09h 客體碼與 `int 15h AH=4Fh`。
//
// 實模式的情境：程式在 3000:0100 空轉（`jmp $`），堆疊 3800:0200，IF=1、CF=0。被中斷時 AX=12FFh：
// AL=FFh 是放開碼（翻不出鍵），客體碼漏了 `in al,60h` 的話緩衝不會有鍵，看得出來。

const (
	kProgSeg  = 0x3000
	kProgIP   = 0x0100
	kStackSeg = 0x3800
	kSP       = 0x0200
	kAX       = 0x12FF
	kHook15   = 0x0200 // int 15h 攔截常式
	kHook09   = 0x0300 // int 09h 包裝常式
	kLogIdx   = 0x03FE // 攔截常式的紀錄索引（byte 位移）
	kLog      = 0x0400 // 紀錄：每筆 AX、FLAGS
)

// newKbdTest 造一台開了鍵盤攔截、程式在空轉的實模式機器。
func newKbdTest(t *testing.T, prog []byte) (*machine.Machine, *DOS) {
	t.Helper()
	m, d := newTest(t)
	m.IRQ0Every = 0
	m.KeyEvery = 1
	if err := m.EnableKbdIntercept(); err != nil {
		t.Fatal(err)
	}
	if !m.KbdIntercept {
		t.Fatal("EnableKbdIntercept 成功後 KbdIntercept 應為 true")
	}
	if prog == nil {
		prog = []byte{0xEB, 0xFE} // jmp $
	}
	copy(m.Mem[kProgSeg*16+kProgIP:], prog)
	c := m.CPU
	c.Seg[cpu.CS], c.IP = kProgSeg, kProgIP
	c.Seg[cpu.SS], c.R[cpu.SP] = kStackSeg, kSP
	c.Seg[cpu.DS], c.Seg[cpu.ES] = kProgSeg, kProgSeg
	c.R[cpu.AX] = kAX
	c.SetFlags(cpu.IF)
	return m, d
}

// runUntil 逐步執行直到 done 成立；上限 max 步（漏改向量 9 的 stub 時 `CD 09` 會無窮遞迴）。
func runUntil(t *testing.T, m *machine.Machine, max int, what string, done func() bool) {
	t.Helper()
	for i := 0; i < max; i++ {
		if done() {
			return
		}
		if err := m.Step(); err != nil {
			t.Fatalf("%s：第 %d 步：%v", what, i, err)
		}
	}
	if !done() {
		c := m.CPU
		t.Fatalf("%s：%d 步內沒有回到程式：CS:IP=%04X:%04X SP=%04X", what, max, c.Seg[cpu.CS], c.IP, c.R[cpu.SP])
	}
}

// atProg：回到被中斷的那一道、堆疊已平衡。
func atProg(m *machine.Machine) bool {
	c := m.CPU
	return c.Seg[cpu.CS] == kProgSeg && c.IP == kProgIP && c.R[cpu.SP] == kSP
}

// farJmp 是 `jmp far seg:off`。
func farJmp(seg, off uint16) []byte {
	return []byte{0xEA, byte(off), byte(off >> 8), byte(seg), byte(seg >> 8)}
}

// chainOld15 是 `jmp far` 回舊的 int 15h 向量（StubSeg 的 F4h trampoline）。
func chainOld15() []byte { return farJmp(machine.StubSeg, machine.ATrapOff) }

// 攔截常式的結尾。
var (
	// 改堆疊框的 FLAGS：push bp; mov bp,sp; and word [bp+6],0FFFEh（清 CF）; pop bp
	clearStackedCF = []byte{0x55, 0x8B, 0xEC, 0x83, 0x66, 0x06, 0xFE, 0x5D}
	// push bp; mov bp,sp; or word [bp+6],1（設 CF）; pop bp
	setStackedCF = []byte{0x55, 0x8B, 0xEC, 0x83, 0x4E, 0x06, 0x01, 0x5D}
)

// hook15 在 kProgSeg:kHook15 裝 int 15h 攔截常式：記下入口的 AX 與 FLAGS，再接 tail。
func hook15(m *machine.Machine, tail ...[]byte) {
	code := []byte{
		0x53,                         // push bx
		0x2E, 0x8B, 0x1E, 0xFE, 0x03, // mov bx,cs:[03FE]
		0x2E, 0x89, 0x87, 0x00, 0x04, // mov cs:[bx+0400],ax
		0x9C,                         // pushf
		0x2E, 0x8F, 0x87, 0x02, 0x04, // pop word cs:[bx+0402]
		0x8D, 0x5F, 0x04, // lea bx,[bx+4]（不動旗標）
		0x2E, 0x89, 0x1E, 0xFE, 0x03, // mov cs:[03FE],bx
		0x5B, // pop bx
	}
	for _, t := range tail {
		code = append(code, t...)
	}
	copy(m.Mem[kProgSeg*16+kHook15:], code)
	m.Write16(0x15*4, kHook15)
	m.Write16(0x15*4+2, kProgSeg)
}

type hookEntry struct{ AX, Flags uint16 }

func hookLog(m *machine.Machine) []hookEntry {
	base := uint32(kProgSeg) * 16
	n := int(m.Read16(base+kLogIdx)) / 4
	out := make([]hookEntry, n)
	for i := range out {
		at := base + kLog + uint32(i)*4
		out[i] = hookEntry{AX: m.Read16(at), Flags: m.Read16(at + 2)}
	}
	return out
}

// checkPreserved：中斷前後 AX、SP、FLAGS（含 IF）與被中斷的 CS:IP 不變。
func checkPreserved(t *testing.T, m *machine.Machine, what string, flags uint16) {
	t.Helper()
	c := m.CPU
	if c.R[cpu.AX] != kAX || c.R[cpu.SP] != kSP || c.Flags != flags || c.Seg[cpu.CS] != kProgSeg || c.IP != kProgIP {
		t.Errorf("%s：AX=%04X SP=%04X FLAGS=%04X CS:IP=%04X:%04X，應為 %04X %04X %04X %04X:%04X",
			what, c.R[cpu.AX], c.R[cpu.SP], c.Flags, c.Seg[cpu.CS], c.IP, kAX, kSP, flags, kProgSeg, kProgIP)
	}
	if !c.Flag(cpu.IF) {
		t.Errorf("%s：IF 應還原為 1", what)
	}
}

// §3 第 2 項第 1 點：程式不掛 int 09h，QueueKey 與 HoldKey 各一組。
func TestKbdInterceptIRQ1WithoutInt09Hook(t *testing.T) {
	for _, tc := range []struct {
		name    string
		send    func(m *machine.Machine)
		pending func(m *machine.Machine) int
	}{
		{"QueueKey", func(m *machine.Machine) { m.QueueKey(0x1E) }, func(m *machine.Machine) int { return m.KeyQueueLen() }},
		{"HoldKey", func(m *machine.Machine) { m.HoldKey(0x1E, m.Steps+1, 40, false) },
			func(m *machine.Machine) int { return m.TimedKeysPending() }},
	} {
		m, d := newKbdTest(t, nil)
		flags := m.CPU.Flags
		tc.send(m)
		runUntil(t, m, 400, tc.name, func() bool { return tc.pending(m) == 0 && atProg(m) })
		if m.KeyIRQs != 2 || m.KeyStalls() != 0 {
			t.Errorf("%s：KeyIRQs=%d（應 2）KeyStalls=%d（應 0）", tc.name, m.KeyIRQs, m.KeyStalls())
		}
		if d.KbdIntercepts != 2 || d.KbdTranslates != 2 {
			t.Errorf("%s：KbdIntercepts=%d KbdTranslates=%d，應各 2（按下與放開都經 int 15h 與 int FAh）",
				tc.name, d.KbdIntercepts, d.KbdTranslates)
		}
		if n := m.BIOSKeyCount(); n != 1 {
			t.Fatalf("%s：緩衝有 %d 個鍵，應 1（放開碼不進緩衝）", tc.name, n)
		}
		checkPreserved(t, m, tc.name, flags)
		if got := readKey(t, d); got != 0x1E61 {
			t.Errorf("%s：int 16h 取到 %04X，應 1E61", tc.name, got)
		}
	}
}

// §3 第 2 項第 2 點：程式掛 int 15h、記下入口再 chain 回舊向量，按下與放開碼依序看到。
func TestKbdInterceptHookSeesMakeAndBreak(t *testing.T) {
	m, d := newKbdTest(t, nil)
	hook15(m, chainOld15())
	flags := m.CPU.Flags
	m.QueueKey(0x1E)
	runUntil(t, m, 400, "chain", func() bool { return m.KeyQueueLen() == 0 && atProg(m) })
	log := hookLog(m)
	if len(log) != 2 {
		t.Fatalf("攔截常式被呼叫 %d 次，應 2：%v", len(log), log)
	}
	for i, want := range []uint16{0x4F1E, 0x4F9E} {
		if log[i].AX != want || log[i].Flags&cpu.CF == 0 {
			t.Errorf("第 %d 次入口 AX=%04X CF=%v，應 AX=%04X、CF=1", i, log[i].AX, log[i].Flags&cpu.CF != 0, want)
		}
	}
	if d.KbdIntercepts != 2 {
		t.Errorf("chain 回 F4h trampoline 應由 Go 服務 2 次：%d", d.KbdIntercepts)
	}
	checkPreserved(t, m, "chain", flags)
	if m.BIOSKeyCount() != 1 || readKey(t, d) != 0x1E61 {
		t.Error("chain 回舊向量後緩衝應有 1E61")
	}
}

// §3 第 2 項第 3 點：攔截常式回 CF=0（不 chain）：鍵被丟掉。
func TestKbdInterceptHookConsumesKey(t *testing.T) {
	m, d := newKbdTest(t, nil)
	hook15(m, clearStackedCF, []byte{0xCF}) // iret
	m.QueueKey(0x1E)
	runUntil(t, m, 400, "CF=0", func() bool { return m.KeyQueueLen() == 0 && atProg(m) })
	if len(hookLog(m)) != 2 {
		t.Fatalf("攔截常式應被呼叫 2 次：%v", hookLog(m))
	}
	if m.BIOSKeyCount() != 0 || d.KbdTranslates != 0 || d.KbdIntercepts != 0 {
		t.Errorf("回 CF=0：緩衝 %d 個鍵、KbdTranslates=%d、KbdIntercepts=%d，應全為 0",
			m.BIOSKeyCount(), d.KbdTranslates, d.KbdIntercepts)
	}
}

// §3 第 2 項第 4 點：攔截常式把堆疊框的 CF 清成 0 之後 chain 回舊向量：服務設回 CF=1（fixStackedCF）。
func TestKbdInterceptChainRestoresCarry(t *testing.T) {
	m, d := newKbdTest(t, nil)
	hook15(m, clearStackedCF, chainOld15())
	m.QueueKey(0x1E)
	runUntil(t, m, 400, "清 CF 再 chain", func() bool { return m.KeyQueueLen() == 0 && atProg(m) })
	if d.KbdIntercepts != 2 || d.KbdTranslates != 2 {
		t.Errorf("KbdIntercepts=%d KbdTranslates=%d，應各 2", d.KbdIntercepts, d.KbdTranslates)
	}
	if m.BIOSKeyCount() != 1 || readKey(t, d) != 0x1E61 {
		t.Error("服務應把堆疊框的 CF 設回 1，緩衝應有 1E61")
	}
}

// §3 第 2 項第 5 點：攔截常式把 AL 改成另一個掃描碼並回 CF=1：緩衝得到新碼的翻譯。
func TestKbdInterceptHookRewritesScan(t *testing.T) {
	m, d := newKbdTest(t, nil)
	hook15(m, []byte{0xB0, 0x30}, setStackedCF, []byte{0xCF}) // mov al,30h（b）；設 CF；iret
	m.QueueScan(0x1E)                                         // 只送按下碼
	runUntil(t, m, 400, "改 AL", func() bool { return m.KeyQueueLen() == 0 && atProg(m) })
	if d.KbdTranslates != 1 || m.BIOSKeyCount() != 1 {
		t.Fatalf("KbdTranslates=%d 緩衝 %d 個鍵，應各 1", d.KbdTranslates, m.BIOSKeyCount())
	}
	if got := readKey(t, d); got != 0x3062 {
		t.Errorf("緩衝取到 %04X，應 3062（改過的 AL，不是埠 60h 的 1E）", got)
	}
}

// §3 第 2 項第 6 點：程式掛 int 09h、讀 60h 後 `pushf; call far` 舊向量：照樣經 int 15h 並放進緩衝。
func TestKbdInterceptInt09WrapperChains(t *testing.T) {
	m, d := newKbdTest(t, nil)
	base := uint32(kProgSeg) * 16
	copy(m.Mem[base+kHook09:], []byte{
		0x50,       // push ax
		0xE4, 0x60, // in al,60h
		0x2E, 0xA2, 0xF0, 0x03, // mov cs:[03F0],al
		0x58,                         // pop ax
		0x9C,                         // pushf
		0x2E, 0xFF, 0x1E, 0xF4, 0x03, // call far cs:[03F4]
		0xCF, // iret
	})
	m.Write16(base+0x3F4, machine.StubOff(0x09)) // 舊向量
	m.Write16(base+0x3F6, machine.StubSeg)
	m.Write16(0x09*4, kHook09)
	m.Write16(0x09*4+2, kProgSeg)
	flags := m.CPU.Flags
	m.QueueKey(0x1E)
	runUntil(t, m, 400, "int 09h 包裝", func() bool { return m.KeyQueueLen() == 0 && atProg(m) })
	if m.Read8(base+0x3F0) != 0x9E {
		t.Errorf("包裝常式最後讀到 %02X，應 9E", m.Read8(base+0x3F0))
	}
	if d.KbdIntercepts != 2 || d.KbdTranslates != 2 {
		t.Errorf("KbdIntercepts=%d KbdTranslates=%d，應各 2", d.KbdIntercepts, d.KbdTranslates)
	}
	checkPreserved(t, m, "int 09h 包裝", flags)
	if m.BIOSKeyCount() != 1 || readKey(t, d) != 0x1E61 {
		t.Error("chain 回舊向量後緩衝應有 1E61")
	}
}

// §3 第 2 項第 7 點：軟體 int 09h（IVT[09h] 在 stub）經客體碼與 int 15h，不由 0x09 分支直接翻譯。
func TestKbdInterceptSoftwareInt09(t *testing.T) {
	m, d := newKbdTest(t, []byte{0xCD, 0x09, 0xEB, 0xFE})
	d.Calls = map[Call]int{}
	m.SetKeyPort(0x1E)
	runUntil(t, m, 100, "軟體 int 09h", func() bool {
		c := m.CPU
		return c.Seg[cpu.CS] == kProgSeg && c.IP == kProgIP+2 && c.R[cpu.SP] == kSP
	})
	if d.KbdIntercepts != 1 || d.KbdTranslates != 1 {
		t.Errorf("KbdIntercepts=%d KbdTranslates=%d，應各 1（經 int 15h 與 int FAh）", d.KbdIntercepts, d.KbdTranslates)
	}
	for k := range d.Calls {
		if k.Int == 0x09 {
			t.Errorf("int 09h 不應由服務層處理：%v", d.Calls)
		}
	}
	if m.BIOSKeyCount() != 1 || readKey(t, d) != 0x1E61 {
		t.Error("緩衝應有 1E61")
	}
}

// §3 第 2 項第 8 點：int 15h 未掛時直接由 Go 服務；選項關閉時維持「記未實作並清 CF」。
func TestKbdInterceptInt15Direct(t *testing.T) {
	m, d := newKbdTest(t, nil)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.CF)
	call(m, d, 0x15, 0x4F1E)
	if !m.CPU.Flag(cpu.CF) || m.CPU.R[cpu.AX] != 0x4F1E || d.KbdIntercepts != 1 {
		t.Errorf("開啟：CF=%v AX=%04X KbdIntercepts=%d，應 CF=1、AX 不變、1", m.CPU.Flag(cpu.CF), m.CPU.R[cpu.AX], d.KbdIntercepts)
	}
	if d.Unimplemented[Call{Int: 0x15, AH: 0x4F, AL: 0x1E}] != 0 {
		t.Error("開啟：AH=4Fh 不應記未實作")
	}

	m, d = newTest(t)
	m.CPU.SetFlags(m.CPU.Flags | cpu.CF)
	call(m, d, 0x15, 0x4F1E)
	if m.CPU.Flag(cpu.CF) || d.KbdIntercepts != 0 || d.Unimplemented[Call{Int: 0x15, AH: 0x4F, AL: 0x1E}] != 1 {
		t.Errorf("關閉：CF=%v KbdIntercepts=%d，應維持現行（清 CF、記未實作）", m.CPU.Flag(cpu.CF), d.KbdIntercepts)
	}
}

// §3 第 2 項第 9 點：EnableKbdIntercept 兩次與一次相同；位元組照 §2.2。
func TestEnableKbdInterceptBytes(t *testing.T) {
	m, _ := newTest(t)
	base := uint32(machine.StubSeg) * 16
	pad := m.Read8(base + uint32(machine.StubOff(0x09)) + 3)
	if err := m.EnableKbdIntercept(); err != nil {
		t.Fatal(err)
	}
	once := append([]byte(nil), m.Mem...)
	if err := m.EnableKbdIntercept(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(once, m.Mem) || !m.KbdIntercept {
		t.Error("第二次呼叫後記憶體應與一次相同、欄位為 true")
	}
	at := base + uint32(machine.StubOff(0x09))
	if got := [4]byte{m.Read8(at), m.Read8(at + 1), m.Read8(at + 2), m.Read8(at + 3)}; got != [4]byte{0xE9, 0x91, 0x04, pad} {
		t.Errorf("StubOff(9) 起是 % X，應 E9 91 04 且 0x27 的填充不動（%02X）", got, pad)
	}
	if machine.KbdInterceptOff != 0x4B8 {
		t.Errorf("KbdInterceptOff＝%03X，應 4B8", machine.KbdInterceptOff)
	}
	want := []byte{0x50, 0xE4, 0x60, 0xB4, 0x4F, 0xF9, 0xCD, 0x15, 0x73, 0x02, 0xCD, intKbdTranslate, 0x58, 0xCF}
	got := m.Mem[base+machine.KbdInterceptOff : base+machine.KbdInterceptOff+uint32(len(want))]
	if !bytes.Equal(got, want) {
		t.Errorf("KbdInterceptOff 起是 % X，應 % X", got, want)
	}
	if machine.KbdInterceptStub9() != [3]byte{0xE9, 0x91, 0x04} {
		t.Errorf("KbdInterceptStub9＝% X", machine.KbdInterceptStub9())
	}
}

// §3 第 2 項第 10 點：選項關閉時維持現行：stub 是 CD 09 CF，不掛 int 09h 時 IRQ1 不送、KeyStalls 增加。
func TestKbdInterceptOffKeepsStall(t *testing.T) {
	m, d := newTest(t)
	m.IRQ0Every, m.KeyEvery = 0, 1
	at := uint32(machine.StubSeg)*16 + uint32(machine.StubOff(0x09))
	if got := [3]byte{m.Read8(at), m.Read8(at + 1), m.Read8(at + 2)}; got != [3]byte{0xCD, 0x09, 0xCF} {
		t.Fatalf("選項關閉：StubOff(9) 是 % X，應 CD 09 CF", got)
	}
	copy(m.Mem[kProgSeg*16+kProgIP:], []byte{0xEB, 0xFE})
	c := m.CPU
	c.Seg[cpu.CS], c.IP = kProgSeg, kProgIP
	c.Seg[cpu.SS], c.R[cpu.SP] = kStackSeg, kSP
	c.SetFlags(cpu.IF)
	m.QueueKey(0x1E)
	for i := 0; i < 50; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.KeyIRQs != 0 || m.KeyStalls() == 0 || m.KeyQueueLen() != 2 || m.BIOSKeyCount() != 0 || d.KbdTranslates != 0 {
		t.Errorf("選項關閉：KeyIRQs=%d KeyStalls=%d 佇列 %d 緩衝 %d，應 0、>0、2、0",
			m.KeyIRQs, m.KeyStalls(), m.KeyQueueLen(), m.BIOSKeyCount())
	}
}

// ---- §3 第 3 項：V86（沿用 v86_test.go 的合成環境） ----

// v86RunUntilProg 逐步執行；V86 的軟體中斷進 ring 0 時以 jemmReflect 反射回 V86。
func v86RunUntilProg(t *testing.T, m *machine.Machine, max int) {
	t.Helper()
	c := m.CPU
	for i := 0; i < max; i++ {
		if c.Seg[cpu.CS] == vProgSeg && c.IP == 0x100 && c.R[cpu.SP] == vSP && !c.Segmented() {
			return
		}
		vStep(t, m, "V86")
		if c.Segmented() {
			jemmReflect(t, m)
			vStep(t, m, "ring 0 的 66 CF 回 V86")
		}
	}
	t.Fatalf("%d 步內沒有回到 V86 程式：CS:IP=%04X:%04X SP=%04X", max, c.Seg[cpu.CS], c.IP, c.R[cpu.SP])
}

// §3 第 3 項第 1 點：IVT[15h] 被 V86 程式掛走時，客體碼的 int 15h 經 IDT 遞送（handle 回 false），
// chain 回 0080:0408 時 fixStackedCF 寫回 CF=1。攔截常式先把堆疊框的 CF 清成 0，否則看不出寫回。
func TestV86KbdInterceptHookedInt15(t *testing.T) {
	m, d := newV86DOS(t, []byte{0xEB, 0xFE})
	if err := m.EnableKbdIntercept(); err != nil {
		t.Fatal(err)
	}
	hook := append(append([]byte(nil), clearStackedCF...), chainOld15()...)
	copy(m.Mem[vProgSeg*16+0x200:], hook)
	binary.LittleEndian.PutUint16(m.Mem[0x15*4:], 0x200)
	binary.LittleEndian.PutUint16(m.Mem[0x15*4+2:], vProgSeg)

	// 仿 JEMM 把 IRQ1 反射到 IVT[09h]：V86 堆疊推 FLAGS、CS、IP，CS:IP 指 StubSeg:StubOff(9)，IF 清掉。
	c := m.CPU
	sp := uint16(vSP - 6)
	lin := uint32(vStackSeg)*16 + uint32(sp)
	binary.LittleEndian.PutUint16(m.Mem[lin:], 0x100)
	binary.LittleEndian.PutUint16(m.Mem[lin+2:], vProgSeg)
	binary.LittleEndian.PutUint16(m.Mem[lin+4:], c.Flags)
	flags := c.Flags
	c.R[cpu.SP] = sp
	c.Seg[cpu.CS], c.IP = machine.StubSeg, machine.StubOff(0x09)
	c.SetFlags(c.Flags &^ cpu.IF)
	c.R[cpu.AX] = kAX
	m.SetKeyPort(0x1E)

	v86RunUntilProg(t, m, 100)
	if d.Calls[Call{Int: 0x15, AH: 0x4F}] != 0 || d.Calls[Call{Int: 0xF4, AH: 0x4F}] != 1 {
		t.Errorf("int 15h 應經 IDT 交給攔截常式、再由 F4h trampoline 服務：%v", d.Calls)
	}
	if d.KbdIntercepts != 1 || d.KbdTranslates != 1 {
		t.Errorf("KbdIntercepts=%d KbdTranslates=%d，應各 1", d.KbdIntercepts, d.KbdTranslates)
	}
	if c.R[cpu.AX] != kAX || c.Flags != flags {
		t.Errorf("AX=%04X FLAGS=%04X，應 %04X %04X", c.R[cpu.AX], c.Flags, kAX, flags)
	}
	if m.BIOSKeyCount() != 1 || readKey(t, d) != 0x1E61 {
		t.Error("fixStackedCF 應寫回 CF=1，緩衝應有 1E61")
	}
}

// §3 第 3 項第 2 點：int FAh 在 cs＝StubSeg 時被 handle 攔下並翻譯；V86 程式自己的 int FAh 經 IDT。
func TestV86KbdTranslateOnlyFromStubSeg(t *testing.T) {
	m, d := newV86DOS(t, nil)
	if err := m.EnableKbdIntercept(); err != nil {
		t.Fatal(err)
	}
	c := m.CPU
	c.Seg[cpu.CS], c.IP = machine.StubSeg, machine.KbdInterceptOff+10 // 客體碼裡的 CD FA
	c.R[cpu.AX] = 0x001E
	vStep(t, m, "StubSeg 的 CD FA")
	if c.Segmented() || d.KbdTranslates != 1 || m.BIOSKeyCount() != 1 || c.IP != machine.KbdInterceptOff+12 {
		t.Errorf("StubSeg：應由服務層翻譯：segmented=%v KbdTranslates=%d 緩衝 %d IP=%04X",
			c.Segmented(), d.KbdTranslates, m.BIOSKeyCount(), c.IP)
	}

	m, d = newV86DOS(t, []byte{0xCD, 0xFA, 0x90})
	if err := m.EnableKbdIntercept(); err != nil {
		t.Fatal(err)
	}
	m.CPU.R[cpu.AX] = 0x001E
	vStep(t, m, "V86 程式的 CD FA")
	if !m.CPU.Segmented() || d.KbdTranslates != 0 || m.BIOSKeyCount() != 0 {
		t.Errorf("CS 不是 StubSeg：應經 IDT：segmented=%v KbdTranslates=%d", m.CPU.Segmented(), d.KbdTranslates)
	}
}

// §3 第 3 項第 3 點：分頁開啟、線性 0 頁為監督者限定時 EnableKbdIntercept 仍寫入成功；頁面不存在時回錯誤。
func TestEnableKbdInterceptSupervisorPage(t *testing.T) {
	const pd, pt = 0x30000, 0x31000
	setup := func(pte0 uint32) *machine.Machine {
		m, _ := newV86DOS(t, nil)
		vPut32(m, pd, pt|7)
		for i := uint32(0); i < 1024; i++ {
			vPut32(m, pt+4*i, i<<12|7) // P、R/W、U/S
		}
		vPut32(m, pt, pte0)
		if err := m.SetCR3(pd); err != nil {
			t.Fatal(err)
		}
		if err := m.SetCR0(0x80000001); err != nil { // PE、PG
			t.Fatal(err)
		}
		if !m.VM() || !m.Translating() {
			t.Fatal("應在 V86 且轉換開啟")
		}
		return m
	}
	at := uint32(machine.StubSeg)*16 + machine.KbdInterceptOff

	m := setup(0 | 3) // 線性 0 頁：P、R/W，U/S=0（監督者限定）
	before := m.ReadSup8(at)
	m.WriteBytes(at, []byte{^before}) // 對照：以使用者身分寫被安靜丟掉
	if m.ReadSup8(at) != before {
		t.Fatal("對照失敗：WriteBytes 在 V86 寫監督者頁應被丟掉，這個測試量不到要量的東西")
	}
	if err := m.EnableKbdIntercept(); err != nil || !m.KbdIntercept {
		t.Fatalf("監督者頁：EnableKbdIntercept 應成功：%v，KbdIntercept=%v", err, m.KbdIntercept)
	}
	if m.ReadSup8(at) != 0x50 || m.ReadSup8(uint32(machine.StubSeg)*16+uint32(machine.StubOff(0x09))) != 0xE9 {
		t.Error("監督者頁：讀回應為客體碼")
	}

	m = setup(0) // 線性 0 頁不存在
	if err := m.EnableKbdIntercept(); err == nil || m.KbdIntercept {
		t.Errorf("頁面不存在：應回錯誤、欄位為 false：err=%v KbdIntercept=%v", err, m.KbdIntercept)
	}
}
