package machine

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"io"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/459-mouse-irq12-v86-delivery §6.2 中屬於分期 (a) 的項目：PIC 埠模型（第 8 項）、只開剖面的失敗即關閉（第 23 項）、
// 升級（第 24、25、33–35 項）、主片在服務模型（第 26–29、32、36 項）、佈局（第 30 項）、Snapshot（第 31 項的 PIC 部分）、
// 載入時的從片 IMR（第 7 項的 IMR 部分）與 PM 版本 4（第 11 項，佇列 0 筆）。

// preV4State 把（以新機器存出的）狀態檔的 stub 區改回 459 之前的樣子：35 bytes 的 BIOS 計時器樁、IVT[74h] 指回
// per-vector stub、滑鼠樁區清 0。合成 PM 版本 1–3 的狀態檔時用（真正的舊檔就是這樣）。
func preV4State(st *machineState) {
	base := uint32(StubSeg) * 16
	copy(st.Mem[base+biosTimerOff:], biosTimerCode[:])
	for a := base + biosTimerOff + uint32(len(biosTimerCode)); a < base+SysConfigOff; a++ {
		st.Mem[a] = 0
	}
	for a := base + Mouse74Off; a < base+MouseUIROff+uint32(len(mouseUIRCode)); a++ {
		st.Mem[a] = 0
	}
	binary.LittleEndian.PutUint16(st.Mem[0x74*4:], StubOff(0x74))
	binary.LittleEndian.PutUint16(st.Mem[0x74*4+2:], StubSeg)
}

// asPMVersion 把狀態檔的 PM 改成版本 ver（1–3），截成該版本的長度。
func asPMVersion(st *machineState, ver uint16) {
	size := map[uint16]int{1: pmV1Size, 2: pmV2Size, 3: pmV3Size}[ver]
	p := append([]byte(nil), st.PM[:size]...)
	binary.LittleEndian.PutUint16(p, ver)
	st.PM = p
}

// prog459 組一支 COM：main 放在 0100h，handlers[off] 放在 off（off ≥ 0100h），中間補 NOP。
func prog459(main []byte, handlers map[uint16][]byte) []byte {
	code := append([]byte(nil), main...)
	for off, h := range handlers {
		end := int(off-0x100) + len(h)
		for len(code) < end {
			code = append(code, 0x90)
		}
		copy(code[off-0x100:], h)
	}
	return code
}

// pm459 造一台保護模式剖面的實模式機器並載入 code（IRQ0Every=0、IF=1）；沒有服務層。
func pm459(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := newSysProg(t, code)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	return m
}

// pm459SB8 是 pm459 加上 444 的 -sb-dma8。
func pm459SB8(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := newPM(t)
	m.SetSoundBlaster(true)
	mustNil(t, "SetSBDMA8", m.SetSBDMA8(true))
	mustNil(t, "LoadCOM", m.LoadCOM(code))
	m.IRQ0Every = 0
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	return m
}

// hook1C 是只處理 int 1Ch（BIOS 計時器樁的轉呼；dos 的 handle 對它不做事）的最小服務層。
func hook1C(m *Machine) {
	m.CPU.IntHook = func(c *cpu.CPU, n uint8) bool { return n == 0x1C }
}

// ---- 第 7 項（IMR 部分）：載入後從片 IMR 為 EFh ---------------------------------------

func TestPIC459SlaveIMRAfterLoad(t *testing.T) {
	code := []byte{0xEB, 0xFE}
	if m := pm459(t, code); m.pic.slaveIMR != 0xEF || m.In8(0xA1) != 0xEF {
		t.Errorf("只開剖面：載入後從片 IMR=%02X，應為 EFh", m.pic.slaveIMR)
	}
	if m := pm459SB8(t, code); m.pic.slaveIMR != 0xEF || m.In8(0xA1) != 0xEF {
		t.Errorf("剖面＋-sb-dma8（剖面先設）：從片 IMR=%02X，應為 EFh", m.pic.slaveIMR)
	}
	// setter 次序反過來：先 SB，再剖面。
	m := newExt(t, 4096)
	m.SetSoundBlaster(true)
	mustNil(t, "SetSBDMA8", m.SetSBDMA8(true))
	mustNil(t, "SetProtectedModeProfile", m.SetProtectedModeProfile(true))
	mustNil(t, "LoadCOM", m.LoadCOM(code))
	if m.pic.slaveIMR != 0xEF {
		t.Errorf("-sb-dma8 先設：從片 IMR=%02X，應為 EFh", m.pic.slaveIMR)
	}
	// 剖面關閉：只開 -sb-dma8 照 444 為 FFh；兩者都關時 A1h 讀 FFh。
	if m := newSB8(t, code, nil); m.In8(0xA1) != 0xFF {
		t.Errorf("只開 -sb-dma8：A1h=%02X，應為 FFh", m.In8(0xA1))
	}
	r := New()
	mustNil(t, "LoadCOM", r.LoadCOM(code))
	if r.In8(0xA1) != 0xFF || r.In8(0x20) != 0xFF || r.In8(0xA0) != 0xFF {
		t.Errorf("實模式剖面：A1h、20h、A0h 應讀 FFh：%02X %02X %02X", r.In8(0xA1), r.In8(0x20), r.In8(0xA0))
	}
}

// ---- 第 8、26、27、36 項：埠與 EOI ------------------------------------------------------------

func TestPIC459Ports(t *testing.T) {
	m := pm459(t, []byte{0xEB, 0xFE})
	// 20h 讀 ISR（OCW3=0Bh）：bit0＝ISR0、bit1＝ISR1、bit2＝ISR2，各自與同時。
	m.Out8(0x20, 0x0B)
	for _, isr := range []uint8{1, 2, 4, 7} {
		m.pic.isr = isr
		if v := m.In8(0x20); v != isr {
			t.Errorf("ISR=%03b 時讀 20h（ISR 選擇）：%02X", isr, v)
		}
	}
	m.pic.isr = 0
	m.Out8(0x20, 0x0A)
	m.irq0Pending = true
	if v := m.In8(0x20); v != 0x01 {
		t.Errorf("20h 讀 IRR：%02X，應 01h", v)
	}
	m.irq0Pending = false
	// A0h 讀 ISR（OCW3=0Bh）：bit4＝ISR4。
	m.Out8(0xA0, 0x0B)
	m.pic.isr4 = true
	if v := m.In8(0xA0); v != 0x10 {
		t.Errorf("ISR4 為 1 時讀 A0h（ISR 選擇）：%02X", v)
	}
	// A0h 的 20h、64h 清 ISR4；60h–63h、65h–67h、00h、40h 不動作。
	for _, v := range []uint8{0x60, 0x61, 0x62, 0x63, 0x65, 0x66, 0x67, 0x00, 0x40} {
		m.Out8(0xA0, v)
		if !m.pic.isr4 {
			t.Errorf("A0h←%02Xh 不應清 ISR4", v)
		}
	}
	m.Out8(0xA0, 0x64)
	if m.pic.isr4 {
		t.Error("A0h←64h 應清 ISR4")
	}
	m.pic.isr4 = true
	m.Out8(0xA0, 0x20)
	if m.pic.isr4 || m.sb8Err != nil {
		t.Errorf("A0h←20h 應清 ISR4：%t %v", m.pic.isr4, m.sb8Err)
	}
	// A1h 讀回寫入值。
	m.Out8(0xA1, 0x5A)
	if v := m.In8(0xA1); v != 0x5A {
		t.Errorf("A1h：%02X", v)
	}
	// A0h、20h 的 poll、特殊遮罩，ICW1、旋轉、設定優先序：失敗即關閉（只開剖面也寫進 sb8Err）。
	for _, x := range []struct {
		port uint16
		v    uint8
		want string
	}{{0x20, 0x0C, "poll"}, {0x20, 0x68, "特殊遮罩"}, {0xA0, 0x0C, "poll"}, {0xA0, 0x68, "特殊遮罩"},
		{0x20, 0x11, "ICW1"}, {0xA0, 0x11, "ICW1"}, {0x20, 0xA0, "旋轉"}, {0xA0, 0xC4, "優先序"}} {
		m := pm459(t, []byte{0xEB, 0xFE})
		m.Out8(x.port, x.v)
		if m.sb8Err == nil || !strings.Contains(m.sb8Err.Error(), x.want) || !strings.Contains(m.sb8Err.Error(), "PIC") {
			t.Errorf("%02Xh←%02Xh 應失敗即關閉（%s）：%v", x.port, x.v, x.want, m.sb8Err)
		}
	}
}

// 第 26、36 項：非特定 EOI 清編號最小的在服務位元；EOI 計數。
func TestPIC459NonSpecificEOI(t *testing.T) {
	m := pm459SB8(t, []byte{0xEB, 0xFE})
	m.pic.isr = 1 | 4 // IRQ12 樁內 sti 之後插入 IRQ0
	m.Out8(0x20, 0x20)
	if m.pic.isr != 4 {
		t.Errorf("ISR0、ISR2 同時為 1：第一次 EOI 應只清 ISR0，ISR=%03b", m.pic.isr)
	}
	m.Out8(0x20, 0x20)
	if m.pic.isr != 0 {
		t.Errorf("第二次 EOI 應清 ISR2，ISR=%03b", m.pic.isr)
	}
	m.pic.isr, m.sb8.isr7 = 2, true
	m.Out8(0x20, 0x20)
	if m.pic.isr != 0 || !m.sb8.isr7 {
		t.Errorf("ISR1、ISR7 同時為 1：只清 ISR1（ISR=%03b ISR7=%t）", m.pic.isr, m.sb8.isr7)
	}
	m.Out8(0x20, 0x20)
	if m.sb8.isr7 {
		t.Error("再一次 EOI 應清 ISR7")
	}
	m.Out8(0x20, 0x20) // 全部為 0：不動作，記為空 EOI
	m.Out8(0xA0, 0x20) // 從片空 EOI
	m.pic.isr4 = true
	m.Out8(0xA0, 0x20)
	want := [5]uint64{1, 1, 1, 1, 1}
	if got := m.picd.eoiMaster; got != want {
		t.Errorf("主片 EOI 計數（ISR0、ISR1、ISR2、ISR7、空）%v，應為 %v", got, want)
	}
	if got := m.picd.eoiSlave; got != [2]uint64{1, 1} {
		t.Errorf("從片 EOI 計數（ISR4、空）%v，應為 [1 1]", got)
	}
	// 雙重 EOI：IRQ0 處理常式自己送 EOI 又鏈回 BIOS 樁時，第二次 EOI 清掉下一個在服務位元或記為空 EOI。
	m.pic.isr = 1 | 4
	m.Out8(0x20, 0x20)
	m.Out8(0x20, 0x20)
	if m.pic.isr != 0 || m.picd.eoiMaster[0] != 2 || m.picd.eoiMaster[2] != 2 {
		t.Errorf("雙重 EOI：ISR=%03b 計數 %v", m.pic.isr, m.picd.eoiMaster)
	}
}

// 第 27 項：特定 EOI。
func TestPIC459SpecificEOI(t *testing.T) {
	m := pm459SB8(t, []byte{0xEB, 0xFE})
	for _, c := range []struct {
		v    uint8
		want uint8 // 清掉之後的 isr（bit0–2）
		isr7 bool
	}{{0x60, 6, true}, {0x61, 5, true}, {0x62, 3, true}, {0x67, 7, false}, {0x63, 7, true}, {0x64, 7, true}} {
		m.pic.isr, m.sb8.isr7 = 7, true
		m.Out8(0x20, c.v)
		if m.pic.isr != c.want || m.sb8.isr7 != c.isr7 {
			t.Errorf("20h←%02Xh：ISR=%03b ISR7=%t，應為 %03b %t", c.v, m.pic.isr, m.sb8.isr7, c.want, c.isr7)
		}
	}
}

// 第 23 項：只開剖面、不開 -sb-dma8：20h 寫 ICW1 的那一步 Step 回錯誤；之後每步都回（指令照樣執行）。
func TestPIC459ICW1FailsInProfileOnly(t *testing.T) {
	m := pm459(t, []byte{0xB0, 0x11, 0xE6, 0x20, 0x90, 0x90, 0xEB, 0xFE})
	mustStep(t, m, "mov al,11h")
	wantStepErr(t, m, "out 20h,al（ICW1）", "ICW1")
	if m.CPU.IP != 0x104 {
		t.Errorf("出錯那一步的 out 已執行：IP=%04X", m.CPU.IP)
	}
	wantStepErr(t, m, "下一步", "ICW1")
}

// ---- 第 28、29 項：在服務閘控、BIOS 樁送 EOI -------------------------------------------------

const (
	h459IRQ0 = 0x200 // IRQ0 處理常式（不送 EOI）：iret
	h459IRQ1 = 0x210 // IRQ1 處理常式（不送 EOI）：in al,60h; iret
	h459EOI0 = 0x220 // IRQ0 處理常式（送 EOI）：push ax; mov al,20h; out 20h,al; pop ax; iret
	h459IRQ7 = 0x230 // IRQ7 處理常式：iret
)

func gateProg() []byte {
	return prog459([]byte{0xEB, 0xFE}, map[uint16][]byte{
		h459IRQ0: {0xCF},
		h459IRQ1: {0xE4, 0x60, 0xCF},
		h459EOI0: {0x50, 0xB0, 0x20, 0xE6, 0x20, 0x58, 0xCF},
		h459IRQ7: {0xCF},
	})
}

func atMain(m *Machine) bool { return m.CPU.Seg[cpu.CS] == PSPSeg && m.CPU.IP == 0x100 }

func TestPIC459InServiceGating(t *testing.T) {
	m := pm459SB8(t, gateProg())
	setIVT(m, 0x08, PSPSeg, h459IRQ0)
	setIVT(m, 0x09, PSPSeg, h459IRQ1)
	setIVT(m, 0x0F, PSPSeg, h459IRQ7)
	m.KeyEvery = 1
	// ISR0=1：IRQ0、IRQ1、IRQ7 都不送。
	m.pic.isr = 1
	m.irq0Pending = true
	m.QueueScan(0x1E)
	m.sb8.irr7 = true
	for i := 0; i < 5; i++ {
		mustStep(t, m, "ISR0 擋住")
	}
	if m.Ticks != 0 || m.KeyIRQs != 0 || m.sb8.irq7Deliveries != 0 || !atMain(m) {
		t.Fatalf("ISR0=1 時不應遞送：Ticks=%d KeyIRQs=%d IRQ7=%d", m.Ticks, m.KeyIRQs, m.sb8.irq7Deliveries)
	}
	// ISR1=1：IRQ0 照送，IRQ1、IRQ7 不送。
	m.pic.isr = 2
	mustStep(t, m, "ISR1：送 IRQ0")
	if m.Ticks != 1 || m.pic.isr != 3 || m.KeyIRQs != 0 {
		t.Fatalf("ISR1=1 時 IRQ0 應照送並設 ISR0：Ticks=%d ISR=%03b KeyIRQs=%d", m.Ticks, m.pic.isr, m.KeyIRQs)
	}
	mustStep(t, m, "iret") // IRQ0 處理常式不送 EOI
	for i := 0; i < 3; i++ {
		mustStep(t, m, "ISR0、ISR1 擋住")
	}
	if m.KeyIRQs != 0 || m.sb8.irq7Deliveries != 0 {
		t.Fatalf("ISR0、ISR1 為 1：IRQ1、IRQ7 不送")
	}
	// EOI 之後依優先序：先清 ISR0（IRQ1 仍被 ISR1 擋）、再清 ISR1 → IRQ1 先於 IRQ7。
	m.Out8(0x20, 0x20)
	mustStep(t, m, "清 ISR0 後")
	if m.KeyIRQs != 0 || m.sb8.irq7Deliveries != 0 {
		t.Fatal("ISR1 仍為 1：IRQ1、IRQ7 不送")
	}
	m.Out8(0x20, 0x20)
	mustStep(t, m, "清 ISR1 後：送 IRQ1")
	if m.KeyIRQs != 1 || m.pic.isr != 2 || m.sb8.irq7Deliveries != 0 {
		t.Fatalf("清 ISR1 後應送 IRQ1 並設 ISR1：KeyIRQs=%d ISR=%03b IRQ7=%d", m.KeyIRQs, m.pic.isr, m.sb8.irq7Deliveries)
	}
	mustStep(t, m, "in al,60h")
	mustStep(t, m, "iret")
	m.Out8(0x20, 0x20)
	mustStep(t, m, "清 ISR1 後：送 IRQ7")
	if m.sb8.irq7Deliveries != 1 {
		t.Fatalf("ISR 全清後應送 IRQ7")
	}
}

// 第 28 項（segWait）：分段模式下只有被在服務位元擋住的 IRQ 時，計數歸零。
func TestPIC459SegWaitIgnoresGated(t *testing.T) {
	m := segmentedMachine(t)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	m.pic.isr = 1
	m.irq0Pending = true
	for i := 0; i < 3; i++ {
		m.Steps++
		m.segWait()
	}
	if m.segWaitN != 0 {
		t.Errorf("只有被 ISR0 擋住的 IRQ0：segWaitN=%d，應 0", m.segWaitN)
	}
	m.pic.isr = 0
	m.Steps++
	m.segWait()
	if m.segWaitN != 1 {
		t.Errorf("ISR0 清掉後 IRQ0 算待處理：segWaitN=%d，應 1", m.segWaitN)
	}
	// IRQ7 與 ISR2（第 17 項的 segWait 部分）。
	s := pm459SB8(t, []byte{0xEB, 0xFE})
	s.sb8.irr7 = true
	s.pic.isr = 4
	if s.sb8IRQ7Ready() {
		t.Error("ISR2 為 1 時 IRQ7 不算待處理")
	}
}

// 第 29 項：BIOS 計時器樁送 EOI；實模式剖面的兩段客體碼與 447／舊版逐位元組相同。
func TestPIC459BIOSTimerStubEOI(t *testing.T) {
	m := pm459(t, []byte{0xEB, 0xFE})
	hook1C(m)
	base := uint32(StubSeg) * 16
	if !bytes.Equal(m.Mem[base+biosTimerOff:base+biosTimerOff+40], biosTimerCodeEOI[:]) {
		t.Fatalf("保護模式剖面的計時器樁應為 40 bytes 版本：% X", m.Mem[base+biosTimerOff:base+biosTimerOff+40])
	}
	m.irq0Pending = true
	mustStep(t, m, "IRQ0 進 BIOS 樁")
	if m.pic.isr != 1 {
		t.Fatalf("IRQ0 送達應設 ISR0：%03b", m.pic.isr)
	}
	n := len(m.PortLog)
	for i := 0; i < 40 && !atMain(m); i++ {
		mustStep(t, m, "BIOS 樁")
	}
	if !atMain(m) || m.pic.isr != 0 {
		t.Fatalf("BIOS 樁 iret 後應回程式且 ISR0 清 0：ISR=%03b CS:IP=%04X:%04X", m.pic.isr, m.CPU.Seg[cpu.CS], m.CPU.IP)
	}
	if eoi := portWrites(m.PortLog[n:], 0x20); len(eoi) != 1 || eoi[0] != 0x20 {
		t.Errorf("BIOS 樁應恰送一次 out 20h,20h：%v", eoi)
	}
	// 之後 IRQ0 照常遞送（N26：不送 EOI 的話 ISR0 卡住，IRQ0 不再遞送）。
	m.irq0Pending = true
	mustStep(t, m, "第二次 IRQ0")
	if m.Ticks != 2 {
		t.Errorf("第二次 IRQ0 應遞送：Ticks=%d", m.Ticks)
	}
	// 實模式剖面：計時器樁 35 bytes、不送 EOI；鍵盤攔截 14 bytes。
	r := New()
	mustNil(t, "LoadCOM", r.LoadCOM([]byte{0xEB, 0xFE}))
	mustNil(t, "EnableKbdIntercept", r.EnableKbdIntercept())
	if !bytes.Equal(r.Mem[base+biosTimerOff:base+biosTimerOff+40], append(append([]byte(nil), biosTimerCode[:]...), 0, 0, 0, 0, 0)) {
		t.Errorf("實模式剖面的計時器樁應為舊版 35 bytes：% X", r.Mem[base+biosTimerOff:base+biosTimerOff+40])
	}
	if !bytes.Equal(r.Mem[base+KbdInterceptOff:base+KbdInterceptOff+19], append(append([]byte(nil), kbdInterceptCode[:]...), 0, 0, 0, 0, 0)) {
		t.Errorf("實模式剖面的鍵盤攔截客體碼應為 447 的 14 bytes：% X", r.Mem[base+KbdInterceptOff:base+KbdInterceptOff+19])
	}
}

func portWrites(log []PortWrite, port uint16) []uint8 {
	var out []uint8
	for _, w := range log {
		if w.Port == port {
			out = append(out, w.Val)
		}
	}
	return out
}

// 第 29 項（IRQ1）：鍵盤攔截客體碼 CF=1 與 CF=0 兩條路都送 EOI。int 15h 由最小服務層改堆疊上的 CF。
func TestPIC459KbdInterceptEOI(t *testing.T) {
	for _, cf := range []bool{true, false} {
		m := pm459(t, []byte{0xEB, 0xFE})
		mustNil(t, "EnableKbdIntercept", m.EnableKbdIntercept())
		base := uint32(StubSeg) * 16
		if !bytes.Equal(m.Mem[base+KbdInterceptOff:base+KbdInterceptOff+19], kbdInterceptCodeEOI[:]) {
			t.Fatalf("保護模式剖面的鍵盤攔截客體碼應為 19 bytes 版本")
		}
		translates := 0
		m.CPU.IntHook = func(c *cpu.CPU, n uint8) bool {
			switch n {
			case 0xF4: // int 15h 的 trampoline：改堆疊框（SP+4）的 CF
				at := cpu.Addr(c.Seg[cpu.SS], c.R[cpu.SP]+4)
				w := m.Read16(at) &^ cpu.CF
				if cf {
					w |= cpu.CF
				}
				m.Write16(at, w)
				return true
			case 0xFA:
				translates++
				return true
			}
			return false
		}
		m.KeyEvery = 1
		m.QueueScan(0x1E)
		n := len(m.PortLog)
		mustStep(t, m, "IRQ1")
		if m.pic.isr != 2 {
			t.Fatalf("CF=%t：IRQ1 送達應設 ISR1：%03b", cf, m.pic.isr)
		}
		for i := 0; i < 40 && !atMain(m); i++ {
			mustStep(t, m, "鍵盤客體碼")
		}
		if !atMain(m) || m.pic.isr != 0 || len(portWrites(m.PortLog[n:], 0x20)) != 1 {
			t.Errorf("CF=%t：應回程式、ISR1 清 0、恰一次 out 20h：ISR=%03b out=%v", cf, m.pic.isr, portWrites(m.PortLog[n:], 0x20))
		}
		if want := map[bool]int{true: 1, false: 0}[cf]; translates != want {
			t.Errorf("CF=%t：int FAh %d 次，應 %d", cf, translates, want)
		}
	}
}

// ---- 第 30 項：佈局 -------------------------------------------------------------------------

func TestPIC459StubLayout(t *testing.T) {
	type span struct {
		name     string
		off, len uint32
	}
	spans := []span{
		{"DosTrapOff", DosTrapOff, 3}, {"VideoTrapOff", VideoTrapOff, 3}, {"ATrapOff", ATrapOff, 3},
		{"XMSTrapOff", XMSTrapOff, 3}, {"CallbackRetOff", CallbackRetOff, 2}, {"FontFullOff", FontFullOff, 3},
		{"FontHalfOff", FontHalfOff, 3}, {"biosTimerOff（保護模式剖面 40 bytes）", biosTimerOff, uint32(len(biosTimerCodeEOI))},
		{"SysConfigOff", SysConfigOff, 8}, {"InDOSOff", InDOSOff, 1}, {"DBCSTableOff", DBCSTableOff, 32},
		{"CountryOff", CountryOff, 34}, {"XMSHookEntryOff", XMSHookEntryOff, 8},
		{"KbdInterceptOff（保護模式剖面 19 bytes）", KbdInterceptOff, uint32(len(kbdInterceptCodeEOI))},
		{"Mouse74Off", Mouse74Off, uint32(len(mouse74Code))}, {"Mouse74RetOff", Mouse74RetOff, uint32(len(mouse74RetCode))},
		{"MouseUIROff", MouseUIROff, uint32(len(mouseUIRCode))},
	}
	limit := uint32(EnvMCBSeg-StubSeg) * 16
	for i, a := range spans {
		if a.off < 0x400 || a.off+a.len > limit {
			t.Errorf("%s（%#x+%d）不在 0x400–%#x", a.name, a.off, a.len, limit)
		}
		for _, b := range spans[i+1:] {
			if a.off < b.off+b.len && b.off < a.off+a.len {
				t.Errorf("%s（%#x+%d）與 %s（%#x+%d）重疊", a.name, a.off, a.len, b.name, b.off, b.len)
			}
		}
	}
	for _, c := range []struct {
		name     string
		off, end uint32
	}{{"計時器樁", biosTimerOff, 0x447}, {"鍵盤攔截", KbdInterceptOff, 0x4CA}, {"Mouse74Off", Mouse74Off, 0x4DC},
		{"Mouse74RetOff", Mouse74RetOff, 0x4E8}, {"MouseUIROff", MouseUIROff, 0x4EC}} {
		n := map[string]int{"計時器樁": len(biosTimerCodeEOI), "鍵盤攔截": len(kbdInterceptCodeEOI), "Mouse74Off": len(mouse74Code),
			"Mouse74RetOff": len(mouse74RetCode), "MouseUIROff": len(mouseUIRCode)}[c.name]
		if c.off+uint32(n)-1 != c.end {
			t.Errorf("%s 佔 %#x–%#x，規格為 %#x 止", c.name, c.off, c.off+uint32(n)-1, c.end)
		}
	}
}

// ---- 第 32 項：ISR 卡住偵測 ------------------------------------------------------------------

func TestPIC459StuckDetection(t *testing.T) {
	// IRQ0 處理常式不送 EOI：ISR0 卡住，IRQ0 再次待處理後第 L+1 步回錯誤。
	m := pm459(t, gateProg())
	setIVT(m, 0x08, PSPSeg, h459IRQ0)
	m.IRQ0Every = 1000
	m.nextIRQ0 = m.Steps + 1
	l := m.stuckLimit()
	if l != 10_000 {
		t.Fatalf("L 應為 10×IRQ0Every＝10000：%d", l)
	}
	var err error
	var blockedAt uint64
	blocked := false
	for i := 0; i < 50_000 && err == nil; i++ {
		pre := m.Steps
		err = m.Step()
		if !blocked && m.irq0Pending && m.pic.isr&1 != 0 {
			blocked, blockedAt = true, pre // 這一步開始被擋住（tick 在這一步設起待處理）
		}
	}
	if err == nil || !strings.Contains(err.Error(), "IRQ0") || !strings.Contains(err.Error(), "ISR0=1") ||
		!strings.Contains(err.Error(), "連續 10001 步") {
		t.Fatalf("IRQ0 卡住應回錯誤（ISR0=1、連續 L+1 步）：%v", err)
	}
	if got := m.Steps - blockedAt; got != l {
		t.Errorf("錯誤應在被擋住的第 L+1 步回傳（被擋住期間執行了 %d 道，應 %d）", got, l)
	}

	// IRQ1 處理常式不送 EOI：ISR1 卡住（IRQ0 由送 EOI 的處理常式照常服務）。
	m = pm459(t, gateProg())
	setIVT(m, 0x09, PSPSeg, h459IRQ1)
	setIVT(m, 0x08, PSPSeg, h459EOI0)
	m.IRQ0Every = 1000
	m.nextIRQ0 = m.Steps + 1
	m.KeyEvery = 1
	m.QueueScan(0x1E, 0x9E)
	err = nil
	for i := 0; i < 50_000 && err == nil; i++ {
		err = m.Step()
	}
	if err == nil || !strings.Contains(err.Error(), "IRQ1") || !strings.Contains(err.Error(), "ISR1=1") {
		t.Fatalf("IRQ1 卡住應回錯誤（ISR1=1）：%v", err)
	}

	// 送 EOI 的正常處理常式：100 × L 步不出錯，最長值小於 L。
	m = pm459(t, gateProg())
	setIVT(m, 0x08, PSPSeg, h459EOI0)
	m.IRQ0Every = 1000
	m.nextIRQ0 = m.Steps + 1
	for i := uint64(0); i < 100*l; i++ {
		if err := m.Step(); err != nil {
			t.Fatalf("送 EOI 的處理常式不應卡住：%v", err)
		}
	}
	if m.picd.stuckMax[stuckIRQ0] >= l || m.Ticks < 900 {
		t.Errorf("最長被擋步數 %d 應小於 L=%d；Ticks=%d", m.picd.stuckMax[stuckIRQ0], l, m.Ticks)
	}

	// IF=0 或 IMR 遮住的步數不計入。
	for _, c := range []struct {
		name string
		prep func(m *Machine)
	}{{"IF=0", func(m *Machine) { m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF) }}, {"IMR", func(m *Machine) { m.picMask = 1 }}} {
		m := pm459(t, []byte{0xEB, 0xFE})
		c.prep(m)
		m.pic.isr = 1
		m.irq0Pending = true
		for i := uint64(0); i < l+5; i++ {
			if err := m.Step(); err != nil {
				t.Fatalf("%s：不應計入卡住：%v", c.name, err)
			}
		}
		if m.picd.stuckMax[stuckIRQ0] != 0 {
			t.Errorf("%s：最長值應為 0：%d", c.name, m.picd.stuckMax[stuckIRQ0])
		}
	}
}

// 第 32 項（第 5 版，N34）：IF 在 0 與 1 之間交錯時，IF=0 的步不歸零，IF=1 的步照樣累加，總和超過 L 仍要報錯
// （若 IF=0 就歸零，處理常式開頭的 cli 會讓卡住永遠偵測不到）。獨立成一個測試，前面的 Fatalf 不會擋住它。
func TestPIC459StuckInterleavedIF(t *testing.T) {
	m := pm459(t, []byte{0xEB, 0xFE})
	m.IRQ0Every = 1000
	l := m.stuckLimit()
	if l != 10000 {
		t.Fatalf("L 應為 10000：%d", l)
	}
	m.pic.isr = 1
	m.irq0Pending = true
	var err error
	for i := uint64(0); i < 3*l && err == nil; i++ {
		if i%2 == 0 {
			m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
		} else {
			m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
		}
		err = m.Step()
	}
	if err == nil || !strings.Contains(err.Error(), "ISR0=1") {
		t.Fatalf("IF 交錯時 ISR0 卡住仍應回錯誤：%v", err)
	}
}

// ---- 第 11、24、25、33、34、35 項：PM 版本 4 與升級 --------------------------------------------

func TestPIC459PMV4RoundTrip(t *testing.T) {
	src := pm459(t, []byte{0xEB, 0xFE})
	for _, f := range []struct {
		isr  uint8
		isr4 bool
		uir  bool
	}{{0, false, false}, {4, true, false}, {4, false, true}, {7, true, true}, {3, false, false}} {
		src.pic.isr, src.pic.isr4, src.pic.uir = f.isr, f.isr4, f.uir
		src.pic.readISR = [2]bool{true, false}
		src.picMask = 0x5A
		src.pic.slaveIMR = 0xA5
		data := saveBytes(t, src)
		dst := pm459(t, []byte{0xEB, 0xFE})
		mustNil(t, "LoadState", dst.LoadState(bytes.NewReader(data)))
		if dst.pic.isr != f.isr || dst.pic.isr4 != f.isr4 || dst.pic.uir != f.uir || dst.pic.readISR != src.pic.readISR ||
			dst.picMask != 0x5A || dst.pic.slaveIMR != 0xA5 || dst.pic.upgraded {
			t.Errorf("版本 4 往返（ISR=%03b ISR4=%t UIR=%t）：讀回 %+v picMask=%02X", f.isr, f.isr4, f.uir, dst.pic, dst.picMask)
		}
		if !bytes.Equal(saveBytes(t, dst), data) {
			t.Errorf("版本 4 往返後再存應逐位元組相同")
		}
	}
	// 驗證：長度不足、保留位元、n 超限。
	base := decodeStateForTest(t, saveBytes(t, src))
	for _, c := range []struct {
		name string
		f    func(p []byte) []byte
	}{
		{"長度不足 148", func(p []byte) []byte { return p[:147] }},
		{"旗標 bit 7", func(p []byte) []byte { p[142] |= 0x80; return p }},
		{"旗標 2 bit 2", func(p []byte) []byte { p[143] |= 0x04; return p }},
		{"旗標 2 bit 7", func(p []byte) []byte { p[143] |= 0x80; return p }},
		{"n 超過 4096", func(p []byte) []byte { binary.LittleEndian.PutUint16(p[146:], 4097); return p }},
		{"n=1 而長度不足", func(p []byte) []byte { binary.LittleEndian.PutUint16(p[146:], 1); return p }},
	} {
		s := base
		s.PM = c.f(append([]byte(nil), base.PM...))
		m := pm459(t, []byte{0xEB, 0xFE})
		before := saveBytes(t, m)
		if err := m.LoadState(bytes.NewReader(encodeStateForTest(t, s))); err == nil {
			t.Errorf("%s：應回錯誤", c.name)
		}
		if !bytes.Equal(before, saveBytes(t, m)) {
			t.Errorf("%s：失敗的 LoadState 不得改變機器", c.name)
		}
	}
}

// v3State 以 src 存出版本 ver（1–3）的狀態檔（stub 區改回舊版）。-sb-dma8 開啟時一併保留 sbDMA8State。
func v3State(t *testing.T, src *Machine, ver uint16, edit func(st *machineState)) []byte {
	t.Helper()
	data := saveBytes(t, src)
	var st machineState
	var sb sbDMA8State
	dec := gobDecoder(data)
	mustNil(t, "decode", dec.Decode(&st))
	if src.sb8 != nil {
		mustNil(t, "decode sb8", dec.Decode(&sb))
	}
	preV4State(&st)
	asPMVersion(&st, ver)
	if edit != nil {
		edit(&st)
	}
	var b bytes.Buffer
	enc := gobEncoder(&b)
	mustNil(t, "encode", enc.Encode(&st))
	if src.sb8 != nil {
		mustNil(t, "encode sb8", enc.Encode(&sb))
	}
	return b.Bytes()
}

func gobDecoder(b []byte) *gob.Decoder    { return gob.NewDecoder(bytes.NewReader(b)) }
func gobEncoder(w io.Writer) *gob.Encoder { return gob.NewEncoder(w) }

func TestPIC459Upgrade(t *testing.T) {
	src := pm459(t, []byte{0xEB, 0xFE})
	src.pic.isr, src.pic.isr4, src.pic.uir = 7, true, true // 舊檔沒有這些；升級後應為 0
	src.picMask = 0x12
	for _, ver := range []uint16{1, 2, 3} {
		data := v3State(t, src, ver, nil)
		m := pm459(t, []byte{0xEB, 0xFE})
		m.QueueCallback(QueuedCall{Seg: 1})
		mustNil(t, "LoadState 升級", m.LoadState(bytes.NewReader(data)))
		base := uint32(StubSeg) * 16
		switch {
		case m.pic.isr != 0 || m.pic.isr4 || m.pic.uir || len(m.cbQueue) != 0: // 第 1 步
			t.Errorf("版本 %d：升級後 ISR、UIR 應為 0、佇列空：%+v 佇列 %d", ver, m.pic, len(m.cbQueue))
		case m.pic.slaveIMR != 0xEF: // 第 2 步（沒有 sbDMA8State：FFh 清 bit 4）
			t.Errorf("版本 %d：從片 IMR %02X，應 EFh", ver, m.pic.slaveIMR)
		case m.picMask != 0: // 第 2a 步（沒有 sbDMA8State：0）
			t.Errorf("版本 %d：主片 IMR %02X，應 0", ver, m.picMask)
		case !bytes.Equal(m.Mem[base+biosTimerOff:base+biosTimerOff+40], biosTimerCodeEOI[:]): // 第 3 步
			t.Errorf("版本 %d：計時器樁應換成 40 bytes 版本", ver)
		case !m.LegacyMouseUpgrade(): // 第 4 步
			t.Errorf("版本 %d：LegacyMouseUpgrade 應為真", ver)
		}
		// 第 24 項：存成版本 4 再載入，bit 5 仍為 1。
		again := saveBytes(t, m)
		if s := decodeStateForTest(t, again); binary.LittleEndian.Uint16(s.PM) != 4 || s.PM[142]&pmfUpgraded == 0 {
			t.Errorf("版本 %d 升級後存檔：PM 版本 %d 旗標 %02X", ver, binary.LittleEndian.Uint16(s.PM), s.PM[142])
		}
		n := pm459(t, []byte{0xEB, 0xFE})
		mustNil(t, "再載入", n.LoadState(bytes.NewReader(again)))
		if !n.LegacyMouseUpgrade() {
			t.Errorf("版本 %d：再載入後 LegacyMouseUpgrade 應仍為真", ver)
		}
	}
	// 冷開機的機器存檔 bit 5 為 0。
	if s := decodeStateForTest(t, saveBytes(t, pm459(t, []byte{0xEB, 0xFE}))); s.PM[142]&pmfUpgraded != 0 {
		t.Error("冷開機存檔的升級血統應為 0")
	}
}

// 第 25、33 項：兩處從片 IMR 與主片 IMR。
func TestPIC459UpgradeWithSB8(t *testing.T) {
	src := pm459SB8(t, []byte{0xEB, 0xFE})
	src.picMask, src.pic.slaveIMR, src.pic.readISR = 0x34, 0xFF, [2]bool{false, true}
	data := v3State(t, src, 3, nil)
	m := pm459SB8(t, []byte{0xEB, 0xFE})
	mustNil(t, "LoadState 升級", m.LoadState(bytes.NewReader(data)))
	if m.pic.slaveIMR != 0xEF || m.picMask != 0x34 || m.pic.readISR != [2]bool{false, true} {
		t.Errorf("有 sbDMA8State 的升級：從片 %02X 主片 %02X 讀取選擇 %v", m.pic.slaveIMR, m.picMask, m.pic.readISR)
	}
	again := saveBytes(t, m)
	dec := decodeAllForTest(t, again)
	if dec.pm[144] != 0xEF || dec.sb.SlaveIMR != 0xEF || dec.pm[145] != 0x34 || dec.sb.PICMask != 0x34 {
		t.Errorf("升級後存檔：PM 從片 %02X、sbDMA8State 從片 %02X、PM 主片 %02X、sbDMA8State 主片 %02X",
			dec.pm[144], dec.sb.SlaveIMR, dec.pm[145], dec.sb.PICMask)
	}
	n := pm459SB8(t, []byte{0xEB, 0xFE})
	mustNil(t, "再載入（兩處一致）", n.LoadState(bytes.NewReader(again)))

	// 兩處不一致：主片 IMR、從片 IMR、讀取選擇各一例，回錯誤且機器不變。
	for _, c := range []struct {
		name string
		f    func(pm []byte, sb *sbDMA8State)
	}{
		{"主片 IMR", func(pm []byte, sb *sbDMA8State) { pm[145] = 0x77 }},
		{"從片 IMR", func(pm []byte, sb *sbDMA8State) { sb.SlaveIMR = 0xFF }},
		{"讀取選擇", func(pm []byte, sb *sbDMA8State) { pm[142] ^= pmfReadISR0 }},
	} {
		bad := reencodeAllForTest(t, again, c.f)
		n := pm459SB8(t, []byte{0xEB, 0xFE})
		before := saveBytes(t, n)
		if err := n.LoadState(bytes.NewReader(bad)); err == nil || !strings.Contains(err.Error(), "不一致") {
			t.Errorf("%s 不一致：應回錯誤：%v", c.name, err)
		}
		if !bytes.Equal(before, saveBytes(t, n)) {
			t.Errorf("%s 不一致：機器不得改變", c.name)
		}
	}

	// 第 33 項：只開剖面時主片 IMR 進 PM（寫 21h 遮 IRQ2，存檔、載入後仍遮）。規格寫 21h=FBh（遮 IRQ2），但 FBh 的
	// bit 2 為 0（放行 IRQ2、遮其餘），遮 IRQ2 是 04h；兩個值都驗：載入後 picMask 照存檔，IRQ12 的主片條件跟著 bit 2。
	for _, v := range []uint8{0xFB, 0x04} {
		p := pm459(t, []byte{0xEB, 0xFE})
		p.Out8(0x21, v)
		q := pm459(t, []byte{0xEB, 0xFE})
		mustNil(t, "LoadState", q.LoadState(bytes.NewReader(saveBytes(t, p))))
		if q.picMask != v || q.irq12Unmasked() != (v&4 == 0) {
			t.Errorf("主片 IMR %02X 應進 PM：讀回 %02X、IRQ12 未遮=%t", v, q.picMask, q.irq12Unmasked())
		}
	}
}

// 第 34 項：升級的計時器樁。
func TestPIC459UpgradeTimerStubChecks(t *testing.T) {
	src := pm459(t, []byte{0xEB, 0xFE})
	check := func(name string, data []byte, want string) {
		t.Helper()
		m := pm459(t, []byte{0xEB, 0xFE})
		before := saveBytes(t, m)
		err := m.LoadState(bytes.NewReader(data))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s：應回錯誤（含 %q）：%v", name, want, err)
		}
		if !bytes.Equal(before, saveBytes(t, m)) {
			t.Errorf("%s：失敗的 LoadState 不得改變機器", name)
		}
	}
	check("計時器樁改一個 byte", v3State(t, src, 3, func(st *machineState) {
		st.Mem[StubSeg*16+biosTimerOff+3] ^= 0xFF
	}), "BIOS 計時器樁")
	for _, ip := range []uint16{0x43F, 0x441} {
		check("CS:IP 在舊樁返回範圍", v3State(t, src, 3, func(st *machineState) {
			st.Seg[cpu.CS], st.IP = StubSeg, ip
		}), "返回位址範圍")
	}
	// 對照：CS:IP 在 0x43D（int 1Ch 本身，換版前後位元組相同）可以升級。
	m := pm459(t, []byte{0xEB, 0xFE})
	mustNil(t, "CS:IP=043D", m.LoadState(bytes.NewReader(v3State(t, src, 3, func(st *machineState) { st.Seg[cpu.CS], st.IP = StubSeg, 0x43D }))))

	// 分段模式：ring 0 堆疊頂 iretd 框架的 V86 CS:IP 落在範圍內。
	var seg *Machine
	frame := func(at uint32, ip uint16) {
		seg.CPU.R[cpu.SP] = 0x1000
		ss := seg.CPU.SegCache(cpu.SS)
		a := ss.Base + uint32(seg.CPU.R[cpu.SP]) + at
		put32(seg, a, uint32(ip))
		put32(seg, a+4, StubSeg)
		put32(seg, a+8, 0x00023202)
	}
	seg = segmentedMachine(t)
	frame(0, 0x440)
	check("ring 0 堆疊頂 iretd 框架", v3State(t, seg, 3, nil), "iretd 框架")
	seg = segmentedMachine(t)
	frame(32, 0x442) // popad 之上
	check("ring 0 堆疊 pushad 之上的 iretd 框架", v3State(t, seg, 3, nil), "iretd 框架")
	seg = segmentedMachine(t)
	frame(0, 0x100)
	m = pm459(t, []byte{0xEB, 0xFE})
	mustNil(t, "框架在範圍外", m.LoadState(bytes.NewReader(v3State(t, seg, 3, nil))))
	if !strings.Contains(m.picd.upgradeFrames, "0080:0100") {
		t.Errorf("升級紀錄應列出找到的框架：%q", m.picd.upgradeFrames)
	}
}

// 第 35 項：升級的鍵盤客體碼。
func TestPIC459UpgradeKbdCode(t *testing.T) {
	src := pm459(t, []byte{0xEB, 0xFE})
	mustNil(t, "EnableKbdIntercept", src.EnableKbdIntercept()) // 19 bytes 版本
	v19 := v3State(t, src, 3, nil)
	v14 := v3State(t, src, 3, func(st *machineState) {
		k := StubSeg*16 + KbdInterceptOff
		copy(st.Mem[k:], kbdInterceptCode[:])
		for i := len(kbdInterceptCode); i < len(kbdInterceptCodeEOI); i++ {
			st.Mem[k+i] = 0
		}
	})
	bad := v3State(t, src, 3, func(st *machineState) { st.Mem[StubSeg*16+KbdInterceptOff+1] = 0x90 })
	zero := func(st *machineState) {
		k := StubSeg*16 + KbdInterceptOff
		for i := 0; i < len(kbdInterceptCodeEOI); i++ {
			st.Mem[k+i] = 0
		}
	}
	// 第 5 版：從未寫過（全 0、向量 9 的 stub 是原本的 CD 09 CF）可接受；全 0 而 stub 已跳過去則失敗。
	never := v3State(t, src, 3, func(st *machineState) {
		zero(st)
		copy(st.Mem[StubSeg*16+uint32(StubOff(0x09)):], []byte{0xCD, 0x09, 0xCF})
	})
	zeroJmp := v3State(t, src, 3, zero)
	zeroOther := v3State(t, src, 3, func(st *machineState) {
		zero(st)
		copy(st.Mem[StubSeg*16+uint32(StubOff(0x09)):], []byte{0x90, 0x90, 0xCF})
	})
	for _, c := range []struct {
		name string
		data []byte
		ok   bool
	}{{"19 bytes", v19, true}, {"14 bytes", v14, true}, {"其他", bad, false},
		{"從未寫過", never, true}, {"全 0 但 stub 已跳過去", zeroJmp, false}, {"全 0 但 stub 是其他內容", zeroOther, false}} {
		m := pm459(t, []byte{0xEB, 0xFE})
		mustNil(t, "EnableKbdIntercept", m.EnableKbdIntercept())
		err := m.LoadState(bytes.NewReader(c.data))
		if c.ok != (err == nil) {
			t.Errorf("KbdIntercept 為真、%s：err=%v", c.name, err)
		}
	}
	// KbdIntercept 為假時不比對。
	m := pm459(t, []byte{0xEB, 0xFE})
	mustNil(t, "KbdIntercept 為假", m.LoadState(bytes.NewReader(bad)))
}

// ---- 第 31 項：Snapshot ---------------------------------------------------------------------

func TestPIC459SnapshotRoundTrip(t *testing.T) {
	// 程式送三次主片 EOI、一次從片 EOI 後空轉。
	code := []byte{0xB0, 0x20, 0xE6, 0x20, 0xE6, 0x20, 0xE6, 0xA0, 0xE6, 0x20, 0xEB, 0xFE}
	dirty := func(m *Machine) {
		m.pic.isr, m.pic.isr4, m.pic.uir = 7, true, true
		m.picMask, m.pic.slaveIMR, m.pic.readISR = 0x40, 0x5A, [2]bool{true, true}
		m.pic.stuckCnt[stuckIRQ0], m.pic.stuckLast[stuckIRQ0] = 3, 4
		m.pic.stuckCnt[stuckIRQ12], m.pic.stuckLast[stuckIRQ12] = 5, 6
		m.pic.upgraded, m.pic.overflow = true, false
	}
	m := pm459(t, code)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF) // 不讓 IRQ 干擾
	dirty(m)
	want := m.pic
	s := m.Snapshot()
	m.pic = picState{slaveIMR: 0x01}
	m.picMask = 0x99
	m.Restore(s)
	if m.pic != want || m.picMask != 0x40 {
		t.Fatalf("Restore 後應逐項等於快照：%+v，應 %+v", m.pic, want)
	}
	run := func(m *Machine) (uint64, [5]uint64, [2]uint64) {
		d0, e0, s0 := m.Deliveries(), m.picd.eoiMaster, m.picd.eoiSlave
		for i := 0; i < 8; i++ {
			mustStep(t, m, "程式")
		}
		var e [5]uint64
		for i := range e {
			e[i] = m.picd.eoiMaster[i] - e0[i]
		}
		return m.Deliveries() - d0, e, [2]uint64{m.picd.eoiSlave[0] - s0[0], m.picd.eoiSlave[1] - s0[1]}
	}
	d1, e1, s1 := run(m)
	ref := pm459(t, code)
	ref.CPU.SetFlags(ref.CPU.Flags &^ cpu.IF)
	dirty(ref)
	d2, e2, s2 := run(ref)
	if d1 != d2 || e1 != e2 || s1 != s2 {
		t.Errorf("Restore 後與直接跑不同：送達 %d／%d、主片 EOI %v／%v、從片 %v／%v", d1, d2, e1, e2, s1, s2)
	}
}

// ---- 輔助：同時解開 machineState 與 sbDMA8State ------------------------------------------------

type allState struct {
	pm []byte
	sb sbDMA8State
}

func decodeAllForTest(t *testing.T, data []byte) allState {
	t.Helper()
	var ms machineState
	var sb sbDMA8State
	dec := gobDecoder(data)
	mustNil(t, "decode", dec.Decode(&ms))
	mustNil(t, "decode sb8", dec.Decode(&sb))
	return allState{pm: ms.PM, sb: sb}
}

func reencodeAllForTest(t *testing.T, data []byte, f func(pm []byte, sb *sbDMA8State)) []byte {
	t.Helper()
	var ms machineState
	var sb sbDMA8State
	dec := gobDecoder(data)
	mustNil(t, "decode", dec.Decode(&ms))
	mustNil(t, "decode sb8", dec.Decode(&sb))
	ms.PM = append([]byte(nil), ms.PM...)
	f(ms.PM, &sb)
	var b bytes.Buffer
	enc := gobEncoder(&b)
	mustNil(t, "encode", enc.Encode(&ms))
	mustNil(t, "encode sb8", enc.Encode(&sb))
	return b.Bytes()
}
