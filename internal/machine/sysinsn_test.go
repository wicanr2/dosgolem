package machine

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/433-real-mode-system-instructions §8 第 4 項（指令）與第 3 項的 IDTR 守門。
// 指令經 Machine.Step 由既有核心執行；profile 開啟、實模式（PE=0）。

// newSysProg 造一台 profile 開啟、載入 code（COM，CS:IP = psp:0100）的機器。
func newSysProg(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := newPM(t)
	mustNil(t, "LoadCOM", m.LoadCOM(code))
	m.IRQ0Every = 0
	return m
}

// dsAddr 是 DS:off 的實體位址（測試在實模式、PE=0）。
func dsAddr(m *Machine, off uint16) uint32 { return cpu.Addr(m.CPU.Seg[cpu.DS], off) }

func mustStep(t *testing.T, m *Machine, what string) {
	t.Helper()
	if err := m.Step(); err != nil {
		t.Fatalf("%s：%v", what, err)
	}
}

func wantStepErr(t *testing.T, m *Machine, what, contains string) error {
	t.Helper()
	err := m.Step()
	if err == nil {
		t.Errorf("%s：應失敗即關閉", what)
		return nil
	}
	if contains != "" && !strings.Contains(err.Error(), contains) {
		t.Errorf("%s：錯誤訊息應含 %q，實際 %v", what, contains, err)
	}
	return err
}

func TestSysSMSW(t *testing.T) {
	// smsw ax；66 smsw eax；smsw [0200]；66 smsw [0204]
	m := newSysProg(t, []byte{
		0x0F, 0x01, 0xE0,
		0x66, 0x0F, 0x01, 0xE0,
		0x0F, 0x01, 0x26, 0x00, 0x02,
		0x66, 0x0F, 0x01, 0x26, 0x04, 0x02,
	})
	m.CPU.R[cpu.AX], m.CPU.Hi[cpu.AX] = 0xFFFF, 0xABCD
	mustStep(t, m, "smsw ax")
	if m.CPU.R[cpu.AX] != 0x0010 || m.CPU.Hi[cpu.AX] != 0xABCD {
		t.Errorf("smsw ax：EAX=%04X%04X，應為 ABCD0010（讀到 0010h、保留高半）", m.CPU.Hi[cpu.AX], m.CPU.R[cpu.AX])
	}
	// 讓 CR0 高半非 0（PG、PE=0 時不分頁），驗 66 形式寫整個 CR0。
	mustNil(t, "SetCR0", m.SetCR0(cr0PG|0x8))
	mustStep(t, m, "66 smsw eax")
	if got := uint32(m.CPU.Hi[cpu.AX])<<16 | uint32(m.CPU.R[cpu.AX]); got != 0x80000018 {
		t.Errorf("66 smsw eax = %08X，應為 80000018（整個 CR0）", got)
	}
	for i := uint16(0); i < 8; i++ {
		m.Mem[dsAddr(m, 0x200+i)] = 0xEE
	}
	mustStep(t, m, "smsw [mem]")
	mustStep(t, m, "66 smsw [mem]")
	for _, c := range []struct {
		off  uint16
		want uint8
	}{{0x200, 0x18}, {0x201, 0x00}, {0x202, 0xEE}, {0x203, 0xEE}, {0x204, 0x18}, {0x205, 0x00}, {0x206, 0xEE}, {0x207, 0xEE}} {
		if got := m.Mem[dsAddr(m, c.off)]; got != c.want {
			t.Errorf("smsw 記憶體形式只寫 2 bytes：[%04X]=%02X，應為 %02X", c.off, got, c.want)
		}
	}
}

func TestSysMovCRRead(t *testing.T) {
	// mov eax,cr0；mov ebx,cr3；mov ecx,cr2；66 mov edx,cr0
	m := newSysProg(t, []byte{0x0F, 0x20, 0xC0, 0x0F, 0x20, 0xDB, 0x0F, 0x20, 0xD1, 0x66, 0x0F, 0x20, 0xC2})
	mustNil(t, "SetCR3", m.SetCR3(0x12345678))
	mustNil(t, "SetCR(2)", m.SetCR(2, 0xCAFEBABE))
	for _, s := range []string{"mov eax,cr0", "mov ebx,cr3", "mov ecx,cr2", "66 mov edx,cr0"} {
		mustStep(t, m, s)
	}
	reg := func(i int) uint32 { return uint32(m.CPU.Hi[i])<<16 | uint32(m.CPU.R[i]) }
	if reg(cpu.AX) != 0x7FFFFFF0 {
		t.Errorf("mov eax,cr0 = %08X，應為 CR0|7FFFFFF0h = 7FFFFFF0", reg(cpu.AX))
	}
	if reg(cpu.BX) != 0x12345000 {
		t.Errorf("mov ebx,cr3 = %08X，應遮低 12 位元", reg(cpu.BX))
	}
	if reg(cpu.CX) != 0xCAFEBABE {
		t.Errorf("mov ecx,cr2 = %08X", reg(cpu.CX))
	}
	if reg(cpu.DX) != 0x7FFFFFF0 {
		t.Errorf("66 mov edx,cr0 = %08X（運算元與 66 前綴無關）", reg(cpu.DX))
	}
}

func TestSysMovCRWriteAndModIgnored(t *testing.T) {
	// 0F 22 40 90：mod=01 也視為暫存器形式（mov cr0,eax），不取位移；IP 恰好前進 3，
	// 下一道是 90（nop）。
	m := newSysProg(t, []byte{0x0F, 0x22, 0x40, 0x90, 0x0F, 0x22, 0xDB, 0x0F, 0x22, 0xD1})
	m.CPU.R[cpu.AX], m.CPU.Hi[cpu.AX] = 0x0008, 0 // TS，不設 PE
	ip := m.CPU.IP
	mustStep(t, m, "0F 22 40")
	if m.CPU.IP != ip+3 {
		t.Errorf("0F 22 40 xx 之後 IP 前進 %d，應恰好 3", m.CPU.IP-ip)
	}
	if m.CR0() != 0x18 {
		t.Errorf("mov cr0,eax：CR0=%08X，應為 00000018", m.CR0())
	}
	mustStep(t, m, "nop")
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x5000, 0x0007
	mustStep(t, m, "mov cr3,ebx")
	if m.CR3() != 0x00075000 {
		t.Errorf("mov cr3,ebx：CR3=%08X", m.CR3())
	}
	m.CPU.R[cpu.CX], m.CPU.Hi[cpu.CX] = 0x1234, 0x5678
	mustStep(t, m, "mov cr2,ecx")
	if v, _ := m.CR(2); v != 0x56781234 {
		t.Errorf("mov cr2,ecx：CR2=%08X", v)
	}
}

func TestSysMovCRInvalid(t *testing.T) {
	for _, c := range []struct {
		name string
		code []byte
	}{
		{"mov cr4,eax", []byte{0x0F, 0x22, 0xE0}},
		{"mov cr1,eax", []byte{0x0F, 0x22, 0xC8}},
		{"mov eax,cr4", []byte{0x0F, 0x20, 0xE0}},
		{"mov eax,cr1", []byte{0x0F, 0x20, 0xC8}},
		{"mov eax,cr7", []byte{0x0F, 0x20, 0xF8}},
	} {
		m := newSysProg(t, c.code)
		cr0 := m.CR0()
		wantStepErr(t, m, c.name, "CR")
		if m.CR0() != cr0 {
			t.Errorf("%s：CR0 不應改變", c.name)
		}
	}
}

func TestSysLGDTSGDT(t *testing.T) {
	code := []byte{
		0x0F, 0x01, 0x16, 0x00, 0x02, // lgdt [0200]
		0x0F, 0x01, 0x06, 0x10, 0x02, // sgdt [0210]
		0x66, 0x0F, 0x01, 0x16, 0x00, 0x02, // 66 lgdt [0200]
		0x0F, 0x01, 0x06, 0x20, 0x02, // sgdt [0220]
		0x0F, 0x01, 0x1E, 0x00, 0x02, // lidt [0200]
		0x0F, 0x01, 0x0E, 0x30, 0x02, // sidt [0230]
		0x66, 0x0F, 0x01, 0x1E, 0x00, 0x02, // 66 lidt [0200]
		0x0F, 0x01, 0x0E, 0x40, 0x02, // sidt [0240]
	}
	m := newSysProg(t, code)
	src := []byte{0x2F, 0x01, 0x56, 0x34, 0x12, 0xAB} // 界限 012Fh、基底 AB123456h
	for i, b := range src {
		m.Mem[dsAddr(m, 0x200+uint16(i))] = b
	}
	read := func(off uint16) []byte {
		out := make([]byte, 6)
		for i := range out {
			out[i] = m.Mem[dsAddr(m, off+uint16(i))]
		}
		return out
	}
	mustStep(t, m, "lgdt")
	if b, l := m.GDTR(); b != 0x00123456 || l != 0x012F {
		t.Errorf("16 位元 lgdt：GDTR=%08X/%04X，應為 00123456/012F（24 位元基底）", b, l)
	}
	mustStep(t, m, "sgdt")
	if got := read(0x210); string(got) != string([]byte{0x2F, 0x01, 0x56, 0x34, 0x12, 0x00}) {
		t.Errorf("sgdt（24 位元）= % X", got)
	}
	mustStep(t, m, "66 lgdt")
	if b, _ := m.GDTR(); b != 0xAB123456 {
		t.Errorf("66 lgdt：GDTR 基底 %08X，應為 AB123456", b)
	}
	mustStep(t, m, "sgdt")
	if got := read(0x220); string(got) != string(src) {
		t.Errorf("sgdt（32 位元基底，不遮 24 位元）= % X，應為 % X", got, src)
	}
	mustStep(t, m, "lidt")
	if b, l := m.IDTR(); b != 0x00123456 || l != 0x012F {
		t.Errorf("16 位元 lidt：IDTR=%08X/%04X", b, l)
	}
	mustStep(t, m, "sidt")
	if got := read(0x230); string(got) != string([]byte{0x2F, 0x01, 0x56, 0x34, 0x12, 0x00}) {
		t.Errorf("sidt（24 位元）= % X", got)
	}
	mustStep(t, m, "66 lidt")
	mustStep(t, m, "sidt")
	if got := read(0x240); string(got) != string(src) {
		t.Errorf("sidt（32 位元）= % X，應為 % X", got, src)
	}
}

func TestSysSIDTInitial(t *testing.T) {
	// 初值：sgdt 存界限 0、基底 0；sidt 存界限 03FFh、基底 0（觸發案例 §8.9 的形狀）。
	m := newSysProg(t, []byte{0x0F, 0x01, 0x06, 0x12, 0x3E, 0x0F, 0x01, 0x0E, 0x18, 0x3E})
	for i := uint16(0); i < 12; i++ {
		m.Mem[dsAddr(m, 0x3E12+i)] = 0xEE
	}
	mustStep(t, m, "sgdt")
	mustStep(t, m, "sidt")
	want := []byte{0, 0, 0, 0, 0, 0, 0xFF, 0x03, 0, 0, 0, 0}
	for i, w := range want {
		if got := m.Mem[dsAddr(m, 0x3E12+uint16(i))]; got != w {
			t.Errorf("[%04X]=%02X，應為 %02X", 0x3E12+i, got, w)
		}
	}
}

func TestSysCLTS(t *testing.T) {
	m := newSysProg(t, []byte{0x0F, 0x06})
	mustNil(t, "SetCR0", m.SetCR0(0x0E|cr0PG)) // MP、EM、TS、PG
	mustStep(t, m, "clts")
	if m.CR0() != cr0PG|0x16 {
		t.Errorf("clts：CR0=%08X，應只清 TS（80000016）", m.CR0())
	}
}

func TestSysMovDR(t *testing.T) {
	// mov dr7,eax；mov ebx,dr7；mov dr4,ecx；mov edx,dr6；mov dr0,eax；mov esi,dr0
	m := newSysProg(t, []byte{0x0F, 0x23, 0xF8, 0x0F, 0x21, 0xFB, 0x0F, 0x23, 0xE1, 0x0F, 0x21, 0xF2,
		0x0F, 0x23, 0xC0, 0x0F, 0x21, 0xC6})
	m.CPU.R[cpu.AX], m.CPU.Hi[cpu.AX] = 0xFFFF, 0xFFFF
	m.CPU.R[cpu.CX], m.CPU.Hi[cpu.CX] = 0, 0
	for _, s := range []string{"mov dr7,eax", "mov ebx,dr7", "mov dr4,ecx", "mov edx,dr6", "mov dr0,eax", "mov esi,dr0"} {
		mustStep(t, m, s)
	}
	reg := func(i int) uint32 { return uint32(m.CPU.Hi[i])<<16 | uint32(m.CPU.R[i]) }
	if reg(cpu.BX) != 0xFFFF2FFF {
		t.Errorf("DR7 讀回 %08X，應為 FFFF2FFF", reg(cpu.BX))
	}
	if reg(cpu.DX) != 0xFFFF0FF0 {
		t.Errorf("DR4→DR6 讀回 %08X，應為 FFFF0FF0", reg(cpu.DX))
	}
	if reg(cpu.SI) != 0xFFFFFFFF {
		t.Errorf("DR0 讀回 %08X", reg(cpu.SI))
	}
}

func TestSysFailClosedForms(t *testing.T) {
	cases := []struct {
		name string
		code []byte
	}{
		{"0F 01 暫存器 /0", []byte{0x0F, 0x01, 0xC0}},
		{"0F 01 暫存器 /1", []byte{0x0F, 0x01, 0xC8}},
		{"0F 01 暫存器 /2（lgdt 暫存器運算元）", []byte{0x0F, 0x01, 0xD0}},
		{"0F 01 暫存器 /3", []byte{0x0F, 0x01, 0xD8}},
		{"0F 01 暫存器 /5", []byte{0x0F, 0x01, 0xE8}},
		{"0F 01 暫存器 /7", []byte{0x0F, 0x01, 0xF8}},
		{"0F 01 記憶體 /5", []byte{0x0F, 0x01, 0x2E, 0x00, 0x02}},
		{"0F 01 記憶體 /7（invlpg）", []byte{0x0F, 0x01, 0x3E, 0x00, 0x02}},
		{"0F 02 lar", []byte{0x0F, 0x02, 0xC0}},
		{"0F 03 lsl", []byte{0x0F, 0x03, 0xC0}},
		{"0F 24", []byte{0x0F, 0x24, 0xC0}},
		{"0F 26", []byte{0x0F, 0x26, 0xC0}},
	}
	for r := 0; r < 8; r++ {
		cases = append(cases,
			struct {
				name string
				code []byte
			}{fmt.Sprintf("0F 00 /%d 暫存器", r), []byte{0x0F, 0x00, byte(0xC0 | r<<3)}},
			struct {
				name string
				code []byte
			}{fmt.Sprintf("0F 00 /%d 記憶體", r), []byte{0x0F, 0x00, byte(0x06 | r<<3), 0x00, 0x02}})
	}
	for _, c := range cases {
		m := newSysProg(t, c.code)
		wantStepErr(t, m, c.name, "失敗即關閉")
	}
}

// 合成 PE=1、VM=1（432 式的注入）：Sys.User() 為真，特權指令記 #GP(0) 事件（435 §2.2）；非特權的照常。
func TestSysPrivilegedInUserMode(t *testing.T) {
	priv := []struct {
		name string
		code []byte
	}{
		{"lmsw ax", []byte{0x0F, 0x01, 0xF0}},
		{"lgdt", []byte{0x0F, 0x01, 0x16, 0x00, 0x02}},
		{"lidt", []byte{0x0F, 0x01, 0x1E, 0x00, 0x02}},
		{"mov eax,cr0", []byte{0x0F, 0x20, 0xC0}},
		{"mov cr0,eax", []byte{0x0F, 0x22, 0xC0}},
		{"mov eax,dr7", []byte{0x0F, 0x21, 0xF8}},
		{"mov dr7,eax", []byte{0x0F, 0x23, 0xF8}},
		{"clts", []byte{0x0F, 0x06}},
	}
	for _, c := range priv {
		m := newSysProg(t, c.code)
		mustNil(t, "SetCR0", m.SetCR0(cr0PE))
		mustNil(t, "SetVM", m.SetVM(true))
		installDeliverEnv(t, m)
		m.CPU.R[cpu.AX], m.CPU.Hi[cpu.AX] = 0x0011, 0
		cr0, dr7 := m.CR0(), m.DR(7)
		ip := m.CPU.IP
		// 435 §2.2：V86 下記 #GP(0) 事件（框架 EIP 為起點、錯誤碼 0），不執行指令本體；Machine 經 IDT 遞送到向量 13。
		err := m.Step()
		ev := m.CPU.V86Event()
		if err != nil || ev.Vector != 13 || !ev.HasError || ev.ErrorCode != 0 || ev.FrameEIP != uint32(ip) ||
			m.CPU.IP != gateOff(13) || frameAt(m, dSS0Base, dESP0, 8) != uint32(ip) || frameAt(m, dSS0Base, dESP0, 9) != 0 {
			t.Errorf("%s：應記 #GP(0) 事件並遞送：err=%v 事件 %+v IP=%04X", c.name, err, ev, m.CPU.IP)
		}
		if m.CR0() != cr0 || m.DR(7) != dr7 {
			t.Errorf("%s：失敗時不得改 CR0／DR7", c.name)
		}
		// 對照：VM=0（User 為假）時同一道照常。
		m2 := newSysProg(t, c.code)
		mustNil(t, "SetCR0", m2.SetCR0(cr0PE))
		m2.CPU.R[cpu.AX], m2.CPU.Hi[cpu.AX] = 0x0011, 0
		mustStep(t, m2, c.name+"（VM=0）")
	}
	for _, c := range []struct {
		name string
		code []byte
	}{
		{"smsw ax", []byte{0x0F, 0x01, 0xE0}},
		{"sgdt", []byte{0x0F, 0x01, 0x06, 0x00, 0x02}},
		{"sidt", []byte{0x0F, 0x01, 0x0E, 0x00, 0x02}},
	} {
		m := newSysProg(t, c.code)
		mustNil(t, "SetCR0", m.SetCR0(cr0PE))
		mustNil(t, "SetVM", m.SetVM(true))
		mustStep(t, m, c.name+"（非特權）")
	}
}

func TestSysLMSW(t *testing.T) {
	// 合成 PE=1：lmsw ax（AX=0）不能清 PE；只改低 4 位元。
	m := newSysProg(t, []byte{0x0F, 0x01, 0xF0, 0x0F, 0x01, 0xF0})
	mustNil(t, "SetCR0", m.SetCR0(cr0PE|0xE)) // 不設 PG：PE=1 時會開分頁
	m.CPU.R[cpu.AX] = 0x0000
	mustStep(t, m, "lmsw 0（PE=1）")
	if m.CR0() != 0x11 {
		t.Errorf("lmsw 0：CR0=%08X，應為 00000011（PE 不清、MP／EM／TS 清掉）", m.CR0())
	}
	m.CPU.R[cpu.AX] = 0xFFFE
	mustStep(t, m, "lmsw FFFE")
	if m.CR0() != 0x1F {
		t.Errorf("lmsw FFFE：CR0=%08X，應為 0000001F（只改低 4 位元）", m.CR0())
	}
}

func TestSysModeSwitchSentinel(t *testing.T) {
	// mov cr0,eax 使 PE 0→1：指令完成（IP 前進、CR0 更新）後回 ErrModeSwitch。
	m := newSysProg(t, []byte{0x0F, 0x22, 0xC0, 0x90})
	m.CPU.R[cpu.AX] = 0x0001
	ip := m.CPU.IP
	err := m.CPU.Step() // 直接看 CPU 的回傳；Machine.Step 收到哨兵後進分段模式（434 §3.1，handoff_test.go）
	if !errors.Is(err, cpu.ErrModeSwitch) {
		t.Fatalf("mov cr0 設 PE：應回 ErrModeSwitch，實際 %v", err)
	}
	if m.CPU.IP != ip+3 || m.CR0()&cr0PE == 0 {
		t.Errorf("哨兵之前指令要完成：IP+%d CR0=%08X", m.CPU.IP-ip, m.CR0())
	}
	// lmsw 1 同樣回哨兵。
	m = newSysProg(t, []byte{0x0F, 0x01, 0xF0})
	m.CPU.R[cpu.AX] = 0x0001
	if err := m.CPU.Step(); !errors.Is(err, cpu.ErrModeSwitch) {
		t.Errorf("lmsw 設 PE：應回 ErrModeSwitch，實際 %v", err)
	}
	// 不回哨兵：mov cr0 不改 PE、lmsw 0、只動 TS、PE 已是 1 再寫 1（經 Machine.Step：
	// 若回了哨兵，Machine 會進分段模式，這裡以 Segmented() 為假判定沒有回哨兵，434 §3.1）。
	for _, c := range []struct {
		name  string
		code  []byte
		ax    uint16
		setPE bool
	}{
		{"mov cr0 不改 PE", []byte{0x0F, 0x22, 0xC0}, 0x0010, false},
		{"lmsw 0", []byte{0x0F, 0x01, 0xF0}, 0x0000, false},
		{"只動 TS（lmsw 8）", []byte{0x0F, 0x01, 0xF0}, 0x0008, false},
		{"PE 已是 1 再 mov cr0 寫 1", []byte{0x0F, 0x22, 0xC0}, 0x0001, true},
		{"PE 已是 1 再 lmsw 1", []byte{0x0F, 0x01, 0xF0}, 0x0001, true},
	} {
		m := newSysProg(t, c.code)
		if c.setPE {
			mustNil(t, "SetCR0", m.SetCR0(cr0PE))
		}
		m.CPU.R[cpu.AX] = c.ax
		if err := m.Step(); err != nil {
			t.Errorf("%s：不應回哨兵或錯誤，實際 %v", c.name, err)
		}
		if m.CPU.Segmented() {
			t.Errorf("%s：不應回哨兵（Machine 收到哨兵才進分段模式）", c.name)
		}
	}
}

func TestSysProfileOffUnchanged(t *testing.T) {
	// Sys == nil：§4 的指令錯誤訊息與現況逐字相同。
	for _, op2 := range []byte{0x00, 0x01, 0x02, 0x03, 0x06, 0x20, 0x21, 0x22, 0x23, 0x24, 0x26} {
		m := newExt(t, 4096)
		mustNil(t, "LoadCOM", m.LoadCOM([]byte{0x0F, op2, 0xC0}))
		m.IRQ0Every = 0
		if m.CPU.Sys != nil {
			t.Fatal("profile 關閉時 Sys 應為 nil")
		}
		cs, ip := m.CPU.Seg[cpu.CS], m.CPU.IP
		err := m.Step()
		want := fmt.Sprintf("cpu: %04X:%04X op %02X：0F %02X：80386 子集沒有這一道（docs/spec/197 §2.2）", cs, ip, op2, op2)
		if err == nil || err.Error() != want {
			t.Errorf("0F %02X（Sys == nil）：\n got  %v\n want %s", op2, err, want)
		}
	}
}

// ---- IDTR 守門（§2、§8 第 3 項）------------------------------------------------

// guardProg 造一台 IDTR 基底不為 0、仍在實模式的機器。
func guardProg(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := newSysProg(t, code)
	m.SetIDTR(0x00012000, 0x07FF)
	// 442 §2.10：依向量；本檔用的 IRQ 向量是 08h、09h，基底非 0 時都成立。
	if !m.IVTMismatch(0x08) || !m.IVTMismatch(0x09) {
		t.Fatal("守門條件應成立")
	}
	return m
}

func TestIVTGuardCPU(t *testing.T) {
	cases := []struct {
		name  string
		code  []byte
		setup func(m *Machine)
	}{
		{"一般 INT 80h", []byte{0xCD, 0x80}, nil},
		{"被 IntHook 攔截的 INT 21h", []byte{0xCD, 0x21}, nil},
		{"16 位元 div 0（divideError）", []byte{0xF6, 0xF3}, func(m *Machine) { m.CPU.R[cpu.BX] = 0 }},
		{"32 位元 div 0（divideFault）", []byte{0x66, 0xF7, 0xF3}, func(m *Machine) { m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0, 0 }},
		{"BOUND 越界（Model80386）", []byte{0x62, 0x06, 0x00, 0x02}, func(m *Machine) {
			m.CPU.R[cpu.AX] = 5
			for i, b := range []byte{0, 0, 1, 0} { // 界 0..1
				m.Mem[dsAddr(m, 0x200+uint16(i))] = b
			}
		}},
	}
	for _, c := range cases {
		for _, guard := range []bool{true, false} {
			m := newSysProg(t, c.code)
			if guard {
				m.SetIDTR(0x00012000, 0x07FF)
			}
			if m.CPU.Model != cpu.Model80386 {
				t.Fatal("Machine 的 CPU 應為 Model80386")
			}
			hooked := 0
			m.CPU.IntHook = func(_ *cpu.CPU, n uint8) bool {
				if n == 0x21 {
					hooked++
					return true
				}
				return false
			}
			if c.setup != nil {
				c.setup(m)
			}
			sp, cs := m.CPU.R[cpu.SP], m.CPU.Seg[cpu.CS]
			divs := len(m.CPU.DivErrors)
			err := m.Step()
			if !guard {
				// 對照：IDTR 基底 0 時照常走 IVT（或 hook）。
				if err != nil {
					t.Errorf("%s（無守門）：%v", c.name, err)
				}
				if c.name == "被 IntHook 攔截的 INT 21h" {
					if hooked != 1 {
						t.Errorf("%s（無守門）：hook 應被呼叫一次", c.name)
					}
				} else if m.CPU.R[cpu.SP] != sp-6 {
					t.Errorf("%s（無守門）：應推 6 bytes 進堆疊，SP %04X→%04X", c.name, sp, m.CPU.R[cpu.SP])
				}
				continue
			}
			if err == nil || !strings.Contains(err.Error(), "IDTR") {
				t.Errorf("%s：應失敗即關閉並寫明 IDTR，實際 %v", c.name, err)
			}
			if hooked != 0 {
				t.Errorf("%s：守門在 IntHook 之前，hook 不得被呼叫", c.name)
			}
			if m.CPU.R[cpu.SP] != sp || m.CPU.Seg[cpu.CS] != cs {
				t.Errorf("%s：不得呼叫 Interrupt（SP %04X→%04X，CS %04X→%04X）", c.name, sp, m.CPU.R[cpu.SP], cs, m.CPU.Seg[cpu.CS])
			}
			if len(m.CPU.DivErrors) != divs {
				t.Errorf("%s：守門在函式最前面，不記 DivErrors", c.name)
			}
		}
	}
}

// irqCase 描述 Machine 側一個守門點的準備與「狀態未被消耗」的判準。
type irqCase struct {
	name     string
	setup    func(m *Machine)
	pending  func(m *Machine) bool // 守門擋下後，IRQ 仍待送
	consumed func(m *Machine) bool // 清掉 IDTR 基底後下一步已送出
}

func TestIVTGuardIRQ(t *testing.T) {
	setVec9 := func(m *Machine) {
		m.WritePhys16(0x09*4, 0x0000)
		m.WritePhys16(0x09*4+2, 0x1234) // 不指 StubSeg，keyTick 才會送
		m.KeyEvery = 1
		m.nextKey = 0
	}
	cases := []irqCase{
		{
			name:     "tick（INT 08h）",
			setup:    func(m *Machine) { m.irq0Pending = true },
			pending:  func(m *Machine) bool { return m.irq0Pending && m.Ticks == 0 },
			consumed: func(m *Machine) bool { return !m.irq0Pending && m.Ticks == 1 },
		},
		{
			name:     "keyTick（INT 09h）",
			setup:    func(m *Machine) { setVec9(m); m.QueueScan(0x1E) },
			pending:  func(m *Machine) bool { return len(m.keyQueue) == 1 && m.KeyIRQs == 0 && m.nextKey == 0 },
			consumed: func(m *Machine) bool { return len(m.keyQueue) == 0 && m.KeyIRQs == 1 },
		},
		{
			name:     "timedKeyTick（INT 09h）",
			setup:    func(m *Machine) { setVec9(m); m.ScheduleKey(0, 0x1E, false) },
			pending:  func(m *Machine) bool { return len(m.timedKeys) == 1 && m.KeyIRQs == 0 },
			consumed: func(m *Machine) bool { return len(m.timedKeys) == 0 && m.KeyIRQs == 1 },
		},
	}
	for _, c := range cases {
		m := guardProg(t, []byte{0x90, 0x90, 0x90})
		m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
		m.picMask = 0
		ticks := 0
		m.SetOnTick(func() { ticks++ })
		c.setup(m)
		cs, ip, sp := m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.R[cpu.SP]
		steps := m.Steps
		err := m.Step()
		if err == nil || !strings.Contains(err.Error(), "IDTR") {
			t.Errorf("%s：錯誤應在同一步回傳並寫明 IDTR，實際 %v", c.name, err)
		}
		if m.Steps != steps {
			t.Errorf("%s：Steps 不得增加（%d→%d）", c.name, steps, m.Steps)
		}
		if m.CPU.Seg[cpu.CS] != cs || m.CPU.IP != ip || m.CPU.R[cpu.SP] != sp {
			t.Errorf("%s：不得呼叫 Interrupt、不得執行 CPU.Step（CS:IP %04X:%04X→%04X:%04X，SP %04X→%04X）",
				c.name, cs, ip, m.CPU.Seg[cpu.CS], m.CPU.IP, sp, m.CPU.R[cpu.SP])
		}
		if !c.pending(m) || ticks != 0 {
			t.Errorf("%s：IRQ 狀態不得被消耗（onTick 呼叫 %d 次）", c.name, ticks)
		}
		if m.irqErr != nil {
			t.Errorf("%s：Step 回傳後要清掉 irqErr", c.name)
		}
		// 條件仍成立：下一步回同一類錯誤，IRQ 不會悄悄消失。
		if err := m.Step(); err == nil || !c.pending(m) {
			t.Errorf("%s：條件仍成立時下一步應再回錯誤且 IRQ 仍待送（%v）", c.name, err)
		}
		// 清掉 IDTR 基底後下一步照常送出。
		m.SetIDTR(0, 0x3FF)
		mustStep(t, m, c.name+"（清掉 IDTR 基底後）")
		if !c.consumed(m) {
			t.Errorf("%s：清掉 IDTR 基底後應送出", c.name)
		}
	}
}
