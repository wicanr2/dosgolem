package machine

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/459-mouse-irq12-v86-delivery §6.2 中屬於分期 (b) 的項目：IRQ12 樁、遞送、int FBh／FCh、佇列進 PM。
// 合成的服務層（svc459）照 internal/dos 的 handle：本道指令為 StubSeg:Mouse74TrapIP／MouseUIRTrapIP 且剖面開啟
// 才接手；登記以 svc459 的欄位模擬 int 33h 的 Handler。

type svc459 struct {
	m              *Machine
	seg, off, mask uint16
	set            bool
	uirMisses      int
	other          func(c *cpu.CPU, n uint8) bool
}

func (s *svc459) hook(c *cpu.CPU, n uint8) bool {
	cs, ip := c.Op()
	if s.m.ProtectedModeProfile() && cs == StubSeg {
		switch {
		case n == IntMouse74 && ip == Mouse74TrapIP:
			s.m.ServiceMouse74(func(q QueuedCall) (uint16, uint16, bool) {
				return s.seg, s.off, s.set && s.mask&q.AX != 0
			})
			return true
		case n == IntMouseUIR && ip == MouseUIRTrapIP:
			if !s.m.ServiceMouseUIR() {
				s.uirMisses++
			}
			return true
		}
	}
	if n == 0x1C {
		return true
	}
	if s.other != nil {
		return s.other(c, n)
	}
	return false
}

// 回呼程式（PSP:0300）：把 AX、BX、CX、DX、SI、DI 與 FLAGS 存到 PSP:0380 起，然後 retf。
// mov cs:[0380],ax / mov cs:[0382],bx / mov cs:[0384],cx / mov cs:[0386],dx / mov cs:[0388],si / mov cs:[038A],di /
// pushf / pop word cs:[038C] / inc word cs:[038E] / retf
var cb459 = []byte{
	0x2E, 0xA3, 0x80, 0x03,
	0x2E, 0x89, 0x1E, 0x82, 0x03,
	0x2E, 0x89, 0x0E, 0x84, 0x03,
	0x2E, 0x89, 0x16, 0x86, 0x03,
	0x2E, 0x89, 0x36, 0x88, 0x03,
	0x2E, 0x89, 0x3E, 0x8A, 0x03,
	0x9C,
	0x2E, 0x8F, 0x06, 0x8C, 0x03,
	0x2E, 0xFF, 0x06, 0x8E, 0x03,
	0xCB,
}

const (
	cb459Off  = 0x0300
	cb459Data = 0x0380
)

// mouse459 造一台保護模式剖面的實模式機器：程式在 PSP:0100 空轉、回呼在 PSP:0300，服務層登記回呼（遮罩 FFFFh）。
func mouse459(t *testing.T, main []byte) (*Machine, *svc459) {
	t.Helper()
	if main == nil {
		main = []byte{0xEB, 0xFE}
	}
	m := pm459(t, prog459(main, map[uint16][]byte{cb459Off: cb459, cb459Data: make([]byte, 0x10),
		h459IRQ0: {0xCF}, h459EOI0: {0x50, 0xB0, 0x20, 0xE6, 0x20, 0x58, 0xCF}, h459IRQ7: {0xCF}}))
	s := &svc459{m: m, seg: PSPSeg, off: cb459Off, mask: 0xFFFF, set: true}
	m.CPU.IntHook = s.hook
	return m, s
}

func (m *Machine) cbData(i int) uint16 { return m.Read16(uint32(PSPSeg)*16 + cb459Data + uint32(2*i)) }

func q459(ax uint16) QueuedCall {
	return QueuedCall{Seg: 0x1111, Off: 0x2222, AX: ax, BX: 0x0001, CX: 0x0140, DX: 0x0064, SI: 0xFFFE, DI: 0x0003}
}

// runToMain 跑到回到 PSP:0100 且堆疊回到 sp 為止。
func runToMain(t *testing.T, m *Machine, sp uint16, max int, what string) {
	t.Helper()
	for i := 0; i < max; i++ {
		if m.CPU.Seg[cpu.CS] == PSPSeg && m.CPU.IP == 0x100 && m.CPU.R[cpu.SP] == sp && i > 0 {
			return
		}
		mustStep(t, m, what)
	}
	t.Fatalf("%s：%d 步內沒有回到程式（CS:IP=%04X:%04X SP=%04X）", what, max, m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.R[cpu.SP])
}

// ---- 第 1、12、15 項：實模式 ----------------------------------------------------------------

func TestMouse459RealModeDelivery(t *testing.T) {
	m, _ := mouse459(t, nil)
	c := m.CPU
	// 被打斷前的暫存器（含高半）、段暫存器與 FLAGS。
	c.R = [8]uint16{0x1111, 0x2222, 0x3333, 0x4444, c.R[cpu.SP], 0x6666, 0x7777, 0x8888}
	c.Hi = [8]uint16{0xA1, 0xA2, 0xA3, 0xA4, 0, 0xA6, 0xA7, 0xA8}
	c.Seg[cpu.DS], c.Seg[cpu.ES] = 0x2000, 0x2100
	c.SetFlags(c.Flags | cpu.IF | cpu.CF | cpu.ZF)
	regs, hi, segs, flags, sp := c.R, c.Hi, c.Seg, c.Flags, c.R[cpu.SP]
	d0 := m.Deliveries()
	m.QueueCallback(q459(0x0001))
	n := len(m.PortLog)
	mustStep(t, m, "IRQ12 遞送")
	if m.Deliveries() != d0+1 || !m.pic.isr4 || m.pic.isr&4 == 0 || c.Seg[cpu.CS] != StubSeg {
		t.Fatalf("一步內應 raiseIRQ(74h)：送達 %d ISR4=%t ISR2=%t CS=%04X", m.Deliveries()-d0, m.pic.isr4, m.pic.isr&4 != 0, c.Seg[cpu.CS])
	}
	sawCallback, cbISR4, cbISR2, cbUIR, cbIF := false, false, false, false, false
	for i := 0; i < 200; i++ {
		if c.Seg[cpu.CS] == PSPSeg && c.IP == cb459Off {
			sawCallback = true
			cbISR4, cbISR2, cbUIR, cbIF = m.pic.isr4, m.pic.isr&4 != 0, m.pic.uir, c.Flag(cpu.IF)
		}
		if c.Seg[cpu.CS] == PSPSeg && c.IP == 0x100 && c.R[cpu.SP] == sp {
			break
		}
		mustStep(t, m, "IRQ12 處理")
	}
	if !sawCallback || cbISR4 || cbISR2 || !cbUIR || !cbIF {
		t.Errorf("回呼入口：到達=%t ISR4=%t ISR2=%t（應已 EOI）UIR=%t（應 1）IF=%t（應 1）", sawCallback, cbISR4, cbISR2, cbUIR, cbIF)
	}
	q := q459(0x0001)
	for i, want := range []uint16{q.AX, q.BX, q.CX, q.DX, q.SI, q.DI} {
		if got := m.cbData(i); got != want {
			t.Errorf("回呼入口暫存器 %d：%04X，應 %04X", i, got, want)
		}
	}
	if c.R != regs || c.Hi != hi || c.Seg != segs || c.Flags != flags {
		t.Errorf("回到被打斷處後暫存器應不變：R %v→%v Hi %v→%v Seg %v→%v FLAGS %04X→%04X", regs, c.R, hi, c.Hi, segs, c.Seg, flags, c.Flags)
	}
	if m.CallbacksMade() != 1 || m.Deliveries() != d0+1 || m.pic.uir || m.pic.isr4 || m.pic.isr != 0 {
		t.Errorf("回呼 1 次、送達只加 1：made=%d 送達 %d UIR=%t ISR4=%t ISR=%03b", m.CallbacksMade(), m.Deliveries()-d0, m.pic.uir, m.pic.isr4, m.pic.isr)
	}
	// 第 15 項：使用者路徑恰一次 A0h、一次 20h 的非特定 EOI，經 +7 入口（返回樁的 cli 與 EOI 不執行）。
	if a0, p20 := portWrites(m.PortLog[n:], 0xA0), portWrites(m.PortLog[n:], 0x20); len(a0) != 1 || len(p20) != 1 || a0[0] != 0x20 || p20[0] != 0x20 {
		t.Errorf("使用者路徑的 EOI：A0h %v、20h %v，應各一次 20h", a0, p20)
	}
}

// ---- 第 2 項：V86（合成 IDT 第 74h 項指向模擬 JEMM 反射的 ring 0 常式）--------------------------------

// reflect459 是 ring 0 的 16 位元常式：在 V86 堆疊推入框架的 FLAGS、CS、IP，框架的 CS:IP 改成 IVT[74h]、清 IF 與 TF，
// iretd 回 V86（JEMM 4F6E 一帶的作法的最小模型）。選擇子 60h（基底 0）讀 IVT，68h（基底 30000h）寫 V86 堆疊。
var reflect459 = []byte{
	0x55, 0x50, 0x53, 0x1E, 0x06, // push bp/ax/bx/ds/es
	0x89, 0xE5, // mov bp,sp（框架在 bp+0Ah）
	0xB8, 0x68, 0x00, 0x8E, 0xC0, // mov ax,68h; mov es,ax
	0xB8, 0x60, 0x00, 0x8E, 0xD8, // mov ax,60h; mov ds,ax
	0x8B, 0x5E, 0x16, // mov bx,[bp+16h]（V86 SP）
	0x83, 0xEB, 0x06, // sub bx,6
	0x89, 0x5E, 0x16, // mov [bp+16h],bx
	0x8B, 0x46, 0x0A, 0x26, 0x89, 0x07, // mov ax,[bp+0Ah]（IP）; mov es:[bx],ax
	0x8B, 0x46, 0x0E, 0x26, 0x89, 0x47, 0x02, // CS → es:[bx+2]
	0x8B, 0x46, 0x12, 0x26, 0x89, 0x47, 0x04, // FLAGS → es:[bx+4]
	0xA1, 0xD0, 0x01, 0x89, 0x46, 0x0A, // IVT[74h] 位移 → 框架 EIP
	0xA1, 0xD2, 0x01, 0x89, 0x46, 0x0E, // IVT[74h] 段 → 框架 CS
	0x81, 0x66, 0x12, 0xFF, 0xFC, // and word [bp+12h],0FCFFh（清 IF、TF）
	0x07, 0x1F, 0x5B, 0x58, 0x5D, // pop es/ds/bx/ax/bp
	0x66, 0xCF, // iretd
}

func TestMouse459V86Delivery(t *testing.T) {
	m := deliverProg(t, prog459([]byte{0xEB, 0xFE}, map[uint16][]byte{cb459Off: cb459, cb459Data: make([]byte, 0x10)}))
	s := &svc459{m: m, seg: PSPSeg, off: cb459Off, mask: 0xFFFF, set: true}
	m.CPU.IntHook = s.hook
	putDesc(m, segGDT, 0x60, 0, 0xFFFF, 0x92, 0)
	putDesc(m, segGDT, 0x68, dV86SS*16, 0xFFFF, 0x92, 0)
	copy(m.Mem[segCode+uint32(gateOff(0x74)):], reflect459)
	c := m.CPU
	regs, segs, sp := c.R, c.Seg, c.R[cpu.SP]
	m.QueueCallback(q459(0x0001))
	mustStep(t, m, "IRQ12 經 IDT")
	if !c.Segmented() || frameAt(m, dSS0Base, dESP0, 8) != 0x100 || frameAt(m, dSS0Base, dESP0, 7) != PSPSeg {
		t.Fatalf("應經 IDT 74h 進 ring 0、無錯誤碼、框架 EIP 為遞送當下的 IP：EIP=%X CS=%X SP=%04X",
			frameAt(m, dSS0Base, dESP0, 8), frameAt(m, dSS0Base, dESP0, 7), c.R[cpu.SP])
	}
	sawStub, sawCB := false, false
	for i := 0; i < 400; i++ {
		if !c.Segmented() && c.Seg[cpu.CS] == StubSeg && c.IP == Mouse74Off {
			sawStub = true
		}
		if !c.Segmented() && c.Seg[cpu.CS] == PSPSeg && c.IP == cb459Off {
			sawCB = m.pic.uir && !m.pic.isr4
		}
		if !c.Segmented() && c.Seg[cpu.CS] == PSPSeg && c.IP == 0x100 && c.R[cpu.SP] == sp {
			break
		}
		mustStep(t, m, "V86 IRQ12")
	}
	if !sawStub || !sawCB || m.cbData(7) != 1 || m.cbData(0) != 1 || m.pic.uir || m.pic.isr4 || m.pic.isr != 0 {
		t.Errorf("反射後在 V86 執行 Mouse74Off（%t）、回呼（%t，次數 %d AX=%04X）、收尾 UIR=%t ISR4=%t ISR=%03b",
			sawStub, sawCB, m.cbData(7), m.cbData(0), m.pic.uir, m.pic.isr4, m.pic.isr)
	}
	if c.R != regs || c.Seg != segs || !m.VM() {
		t.Errorf("返回樁的 iret 應回到模擬 JEMM 推的框架：R %v→%v Seg %v→%v VM=%t", regs, c.R, segs, c.Seg, m.VM())
	}
}

// ---- 第 3、4、7、12 項 ---------------------------------------------------------------------

func TestMouse459ThreeEventsInOrder(t *testing.T) {
	m, _ := mouse459(t, nil)
	sp := m.CPU.R[cpu.SP]
	for i := uint16(1); i <= 3; i++ {
		m.QueueCallback(q459(i))
	}
	d0 := m.Deliveries()
	var order []uint16
	count := m.Read16(uint32(PSPSeg)*16 + cb459Data + 14)
	for i := 0; i < 600 && len(order) < 3; i++ {
		// 第二筆在第一筆 UIR 結束、EOI 之前不遞送。
		busy, d := m.pic.uir || m.pic.isr4 || m.pic.isr&4 != 0, m.Deliveries()
		mustStep(t, m, "三筆事件")
		if busy && m.Deliveries() != d {
			t.Fatalf("UIR、ISR4 或 ISR2 為 1 時不應再遞送 IRQ12（第 %d 步）", m.Steps)
		}
		if n := m.Read16(uint32(PSPSeg)*16 + cb459Data + 14); n != count {
			count = n
			order = append(order, m.cbData(0))
		}
	}
	runToMain(t, m, sp, 200, "收尾")
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 || m.Deliveries() != d0+3 || m.CallbacksMade() != 3 {
		t.Errorf("三筆事件應依序各遞送一次：次序 %v 送達 %d 回呼 %d", order, m.Deliveries()-d0, m.CallbacksMade())
	}
}

func TestMouse459Masks(t *testing.T) {
	for _, c := range []struct {
		name  string
		block func(m *Machine)
		open  func(m *Machine)
	}{
		{"從片 IMR bit 4", func(m *Machine) { m.Out8(0xA1, 0xFF) }, func(m *Machine) { m.Out8(0xA1, 0xEF) }},
		{"主片 bit 2", func(m *Machine) { m.Out8(0x21, 0x04) }, func(m *Machine) { m.Out8(0x21, 0x00) }},
		{"IF=0", func(m *Machine) { m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF) }, func(m *Machine) { m.CPU.SetFlags(m.CPU.Flags | cpu.IF) }},
	} {
		m, _ := mouse459(t, nil)
		c.block(m)
		m.QueueCallback(q459(1))
		d0 := m.Deliveries()
		for i := 0; i < 5; i++ {
			mustStep(t, m, c.name)
		}
		if m.Deliveries() != d0 || len(m.cbQueue) != 1 {
			t.Errorf("%s：不應遞送、事件留在佇列（送達 %d 佇列 %d）", c.name, m.Deliveries()-d0, len(m.cbQueue))
		}
		c.open(m)
		mustStep(t, m, c.name+"放行")
		if m.Deliveries() != d0+1 {
			t.Errorf("%s：放行後下一個邊界應遞送", c.name)
		}
	}
}

func TestMouse459LoadInit(t *testing.T) {
	m, _ := mouse459(t, nil)
	base := uint32(StubSeg) * 16
	if seg, off := m.ReadSup16(0x74*4+2), m.ReadSup16(0x74*4); seg != StubSeg || off != Mouse74Off {
		t.Errorf("IVT[74h]=%04X:%04X，應 %04X:%04X", seg, off, StubSeg, Mouse74Off)
	}
	for _, s := range []struct {
		off  uint32
		want []byte
	}{{Mouse74Off, []byte{0xFB, 0x1E, 0x06, 0x66, 0x60, 0xCD, 0xFB, 0x50, 0xB0, 0x20, 0xE6, 0xA0, 0xE6, 0x20, 0x58, 0xFC, 0xCB}},
		{Mouse74RetOff, []byte{0xFA, 0xB0, 0x20, 0xE6, 0xA0, 0xE6, 0x20, 0x66, 0x61, 0x07, 0x1F, 0xCF}},
		{MouseUIROff, []byte{0xFA, 0xCD, 0xFC, 0xCB}}} {
		if got := m.Mem[base+s.off : base+s.off+uint32(len(s.want))]; !bytes.Equal(got, s.want) {
			t.Errorf("樁 %#x：% X，應 % X", s.off, got, s.want)
		}
	}
	// 實模式剖面：不寫樁、IVT[74h] 照舊。
	r := New()
	mustNil(t, "LoadCOM", r.LoadCOM([]byte{0xEB, 0xFE}))
	if r.ReadSup16(0x74*4) != StubOff(0x74) || r.Mem[base+Mouse74Off] != 0 {
		t.Errorf("實模式剖面不應寫 IRQ12 樁或改 IVT[74h]")
	}
}

// ---- 第 5 項：分段模式 -----------------------------------------------------------------------

func TestMouse459Segmented(t *testing.T) {
	m := segmentedMachine(t)
	s := &svc459{m: m, seg: PSPSeg, off: cb459Off, mask: 0xFFFF, set: true}
	m.CPU.IntHook = s.hook
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	m.QueueCallback(q459(1))
	m.pic.slaveIMR = 0xEF
	d0, ip := m.Deliveries(), m.CPU.IP
	mustStep(t, m, "分段模式")
	if m.Deliveries() != d0 || len(m.cbQueue) != 1 || m.pic.isr4 || m.CPU.IP != ip+1 {
		t.Fatalf("分段模式下延後不消耗：送達 %d 佇列 %d", m.Deliveries()-d0, len(m.cbQueue))
	}
	m.CPU.IP = ip // 停在 nop 上空轉
	var err error
	for i := 0; i < segWaitLimit+5 && err == nil; i++ {
		m.CPU.IP = ip
		err = m.Step()
	}
	if err == nil || !strings.Contains(err.Error(), "IRQ12") || !strings.Contains(err.Error(), "佇列 1 筆") {
		t.Errorf("segWait 一百萬步失敗即關閉的訊息應含 IRQ12 欄位：%v", err)
	}
}

// ---- 第 6、10（機器端）、21、22 項：登記失效、AX=0000h、改掛 IVT[74h] ----------------------------

func TestMouse459Discard(t *testing.T) {
	m, s := mouse459(t, nil)
	sp := m.CPU.R[cpu.SP]
	m.QueueCallback(q459(1))
	s.set = false // 排入後停用（AX=000Ch CX=0 的效果）
	n := len(m.PortLog)
	runToMain(t, m, sp, 200, "丟棄")
	if m.CallbacksMade() != 0 || m.picd.discards != 1 || m.pic.isr4 || m.pic.isr != 0 || m.cbData(7) != 0 {
		t.Errorf("登記失效：回呼 %d 丟棄 %d ISR4=%t ISR=%03b", m.CallbacksMade(), m.picd.discards, m.pic.isr4, m.pic.isr)
	}
	if a0, p20 := portWrites(m.PortLog[n:], 0xA0), portWrites(m.PortLog[n:], 0x20); len(a0) != 1 || len(p20) != 1 {
		t.Errorf("丟棄路徑（返回樁 +0）的 EOI：A0h %v、20h %v，應各一次", a0, p20)
	}
	// 第 22 項：遮罩不含該事件 → 丟棄；交換成新常式 → 以新常式呼叫。
	m, s = mouse459(t, nil)
	sp = m.CPU.R[cpu.SP]
	m.QueueCallback(q459(4))
	s.mask = 0x0002
	runToMain(t, m, sp, 200, "遮罩不含")
	if m.CallbacksMade() != 0 || m.picd.discards != 1 {
		t.Errorf("新遮罩不含事件應丟棄：回呼 %d 丟棄 %d", m.CallbacksMade(), m.picd.discards)
	}
	m.QueueCallback(q459(2))
	runToMain(t, m, sp, 200, "遮罩含")
	if m.CallbacksMade() != 1 || m.cbData(0) != 2 {
		t.Errorf("遞送當下的登記：回呼 %d AX=%04X", m.CallbacksMade(), m.cbData(0))
	}
	// AX=0000h：佇列清空、UIR 清 0、丟棄計數加上清掉的筆數、之後不再遞送；從片 IMR bit 4 清 0。
	m, _ = mouse459(t, nil)
	m.Out8(0xA1, 0xFF)
	m.QueueCallback(q459(1))
	m.QueueCallback(q459(1))
	m.pic.uir = true
	m.MouseIRQ12Reset()
	if len(m.cbQueue) != 0 || m.pic.uir || m.picd.discards != 2 || m.pic.slaveIMR != 0xEF {
		t.Errorf("AX=0000h：佇列 %d UIR=%t 丟棄 %d 從片 IMR %02X", len(m.cbQueue), m.pic.uir, m.picd.discards, m.pic.slaveIMR)
	}
	d0 := m.Deliveries()
	mustStep(t, m, "重設後")
	if m.Deliveries() != d0 {
		t.Error("AX=0000h 之後不應再遞送 IRQ12")
	}
}

func TestMouse459IVT74Hooked(t *testing.T) {
	// 0260：遊戲的 int 74h：鏈回 Mouse74Off（jmp far）；0270：不鏈回，只送 EOI 後 iret。
	main := []byte{0xEB, 0xFE}
	off, seg := uint16(Mouse74Off), uint16(StubSeg)
	chain := []byte{0xEA, byte(off), byte(off >> 8), byte(seg), byte(seg >> 8)}
	noChain := []byte{0x50, 0xB0, 0x20, 0xE6, 0xA0, 0xE6, 0x20, 0x58, 0xCF}
	m, _ := mouse459(t, prog459(main, map[uint16][]byte{0x220 + 0x40: chain, 0x220 + 0x50: noChain}))
	setIVT(m, 0x74, PSPSeg, 0x260)
	sp := m.CPU.R[cpu.SP]
	m.QueueCallback(q459(1))
	runToMain(t, m, sp, 200, "鏈回")
	if m.CallbacksMade() != 1 || len(m.cbQueue) != 0 {
		t.Errorf("鏈回 Mouse74Off：事件應照常消耗：回呼 %d 佇列 %d", m.CallbacksMade(), len(m.cbQueue))
	}
	setIVT(m, 0x74, PSPSeg, 0x270)
	var err error
	for i := 0; i < cbQueueMax+10 && err == nil; i++ {
		m.QueueCallback(q459(1))
		err = m.Step()
	}
	if err == nil || !strings.Contains(err.Error(), "IVT[74h]=0100:0270") {
		t.Errorf("不鏈回：佇列只增不減，排滿後溢位錯誤且含 IVT[74h]：%v", err)
	}
}

// ---- 第 9、16、17 項：tick 結構與 IRQ7 --------------------------------------------------------

func TestMouse459TickStructure(t *testing.T) {
	// 第 9 項：IRQ0 與 IRQ12 同時待處理時先送 IRQ0。
	m, _ := mouse459(t, nil)
	setIVT(m, 0x08, PSPSeg, h459EOI0)
	m.irq0Pending = true
	m.QueueCallback(q459(1))
	d0 := m.Deliveries()
	mustStep(t, m, "同一步")
	if m.Ticks != 1 || m.pic.isr4 || m.Deliveries() != d0+1 {
		t.Errorf("同一步先送 IRQ0、不判 IRQ12：Ticks=%d ISR4=%t", m.Ticks, m.pic.isr4)
	}
	// 第 16 項：主片 bit 0 遮住 IRQ0、IF=1、IRQ12 待處理：IRQ12 照送。
	m, _ = mouse459(t, nil)
	m.picMask = 0x01
	m.irq0Pending = true
	m.QueueCallback(q459(1))
	mustStep(t, m, "IRQ0 被遮")
	if !m.pic.isr4 {
		t.Error("主片 bit 0 遮住 IRQ0 時 IRQ12 應照送")
	}
	// 第 16 項：IRQ0 待處理但 ivtGuardIRQ(08h) 失敗的那一步不判 IRQ12。
	m, _ = mouse459(t, nil)
	m.irq0Pending = true
	m.QueueCallback(q459(1))
	m.SetIDTR(0x1000, 0x3FF) // 實模式 IDTR 基底不為 0：IRQ 守門失敗
	err := m.Step()
	if err == nil || m.pic.isr4 || len(m.cbQueue) != 1 {
		t.Errorf("IRQ0 守門失敗：應回錯誤、佇列與 ISR4 不變：err=%v ISR4=%t 佇列 %d", err, m.pic.isr4, len(m.cbQueue))
	}
	// 第 9 項：IRQ12 送達那一步 IRQ7 不送（-sb-dma8 開）。
	s := pm459SB8(t, prog459([]byte{0xEB, 0xFE}, map[uint16][]byte{cb459Off: cb459, h459IRQ7: {0xCF}}))
	sv := &svc459{m: s, seg: PSPSeg, off: cb459Off, mask: 0xFFFF, set: true}
	s.CPU.IntHook = sv.hook
	setIVT(s, 0x0F, PSPSeg, h459IRQ7)
	s.QueueCallback(q459(1))
	s.sb8.irr7 = true
	mustStep(t, s, "IRQ12 與 IRQ7")
	if !s.pic.isr4 || s.sb8.irq7Deliveries != 0 || !s.sb8.irr7 {
		t.Errorf("IRQ12 送達那一步 IRQ7 不送：ISR4=%t IRQ7=%d", s.pic.isr4, s.sb8.irq7Deliveries)
	}
	// 第 17 項：sti 之後、樁尾 out 20h 之前立起 IRR7，IRQ7 延到 ISR2 清除後才送。
	for s.CPU.Seg[cpu.CS] == StubSeg && s.CPU.IP < Mouse74Off+0x0D {
		if s.CPU.IP > Mouse74Off && !s.sb8.irr7 {
			s.sb8.irr7 = true
		}
		if s.sb8.irq7Deliveries != 0 {
			t.Fatalf("ISR2 為 1 時 IRQ7 不應送（IP=%04X）", s.CPU.IP)
		}
		mustStep(t, s, "樁內")
	}
	for i := 0; i < 5 && s.sb8.irq7Deliveries == 0; i++ {
		mustStep(t, s, "EOI 之後")
	}
	if s.sb8.irq7Deliveries != 1 {
		t.Errorf("ISR2 清除後 IRQ7 應遞送：%d", s.sb8.irq7Deliveries)
	}
}

// 第 16 項（V86）：IRQ0 待處理但 IDT[08h] 的閘不存在（ivtGuardIRQ(08h) 失敗）的那一步不判 IRQ12（IDT[74h] 正常）；
// 實模式的 IDTR 守門對 08h 與 74h 同時成立或同時失敗，分不出來，所以在 V86 驗。
func TestMouse459IRQ0GuardFailV86(t *testing.T) {
	m := deliverProg(t, prog459([]byte{0x90, 0x90, 0xEB, 0xFE}, map[uint16][]byte{cb459Off: cb459, cb459Data: make([]byte, 0x10)}))
	s := &svc459{m: m, seg: PSPSeg, off: cb459Off, mask: 0xFFFF, set: true}
	m.CPU.IntHook = s.hook
	putGate(m, 0x08, 0x18, uint32(gateOff(0x08)), 0x0E00) // P=0
	m.irq0Pending = true
	m.QueueCallback(q459(1))
	err := m.Step()
	if err == nil || m.pic.isr4 || len(m.cbQueue) != 1 || m.CPU.Segmented() {
		t.Errorf("IRQ0 守門失敗的那一步：應回錯誤、不判 IRQ12（err=%v ISR4=%t 佇列 %d 分段=%t）", err, m.pic.isr4, len(m.cbQueue), m.CPU.Segmented())
	}
	// 對照：IRQ0 不待處理時同一台機器照送 IRQ12。
	m.irq0Pending = false
	mustStep(t, m, "IRQ12")
	if !m.pic.isr4 || !m.CPU.Segmented() {
		t.Errorf("IRQ0 不待處理時應送 IRQ12：ISR4=%t", m.pic.isr4)
	}
}

// 第 8 項（IRQ12 部分）：A0h 讀 IRR bit 4＝IRQ12 待處理；20h 讀 IRR bit 2＝待處理、從片 bit 4 未遮、ISR4 為 0。
// 第 28 項（IRQ12 部分）：ISR0、ISR1、ISR2 任一為 1 時 IRQ12 不送；清掉後下一個邊界送。
func TestMouse459PICBitsAndGating(t *testing.T) {
	m, _ := mouse459(t, nil)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	m.Out8(0x20, 0x0A)
	m.Out8(0xA0, 0x0A)
	if m.In8(0x20)&4 != 0 || m.In8(0xA0) != 0 {
		t.Errorf("佇列空：20h IRR bit 2 與 A0h IRR 應為 0")
	}
	m.QueueCallback(q459(1))
	if m.In8(0x20)&4 == 0 || m.In8(0xA0) != 0x10 {
		t.Errorf("IRQ12 待處理：20h=%02X A0h=%02X", m.In8(0x20), m.In8(0xA0))
	}
	m.Out8(0xA1, 0xFF)
	if m.In8(0x20)&4 != 0 || m.In8(0xA0) != 0x10 {
		t.Errorf("從片 bit 4 遮住：20h IRR bit 2 應為 0、A0h IRR 照舊：20h=%02X A0h=%02X", m.In8(0x20), m.In8(0xA0))
	}
	m.Out8(0xA1, 0xEF)
	m.pic.isr4 = true
	if m.In8(0x20)&4 != 0 {
		t.Error("ISR4 為 1：20h IRR bit 2 應為 0")
	}
	m.pic.isr4 = false
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	for _, isr := range []uint8{1, 2, 4} {
		m.pic.isr = isr
		d0 := m.Deliveries()
		mustStep(t, m, "在服務擋住")
		if m.Deliveries() != d0 || m.pic.isr4 {
			t.Errorf("ISR=%03b 時 IRQ12 不應送", isr)
		}
	}
	m.pic.isr = 0
	mustStep(t, m, "清掉後")
	if !m.pic.isr4 {
		t.Error("在服務位元清掉後下一個邊界應送 IRQ12")
	}
}

// ---- 第 18、19、20 項 ------------------------------------------------------------------------

func TestMouse459TrapAddressOnly(t *testing.T) {
	// 程式自己的 CD FB（不在 Mouse74Off+5）：不被服務層接手，照 IVT[FBh] 走（這裡指向程式的 iret）。
	m, s := mouse459(t, []byte{0xCD, 0xFB, 0xEB, 0xFE})
	setIVT(m, 0xFB, PSPSeg, h459IRQ0)
	m.QueueCallback(q459(1))
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	mustStep(t, m, "程式自己的 int FBh")
	if m.CPU.IP != h459IRQ0 || len(m.cbQueue) != 1 || m.pic.uir {
		t.Errorf("程式自己的 CD FB 應照 IVT 走：IP=%04X 佇列 %d UIR=%t", m.CPU.IP, len(m.cbQueue), m.pic.uir)
	}
	_ = s
	// 剖面關閉：Mouse74Off+5 的 CD FB 也不接手。
	r := New()
	mustNil(t, "LoadCOM", r.LoadCOM([]byte{0xEB, 0xFE}))
	sv := &svc459{m: r, seg: PSPSeg, off: cb459Off, mask: 0xFFFF, set: true}
	r.CPU.IntHook = sv.hook
	copy(r.Mem[StubSeg*16+Mouse74Off:], mouse74Code[:])
	r.CPU.Seg[cpu.CS], r.CPU.IP = StubSeg, Mouse74TrapIP
	setIVT(r, 0xFB, PSPSeg, 0x100)
	before := r.CallbacksMade()
	mustStep(t, r, "剖面關閉的 CD FB")
	if r.CPU.Seg[cpu.CS] != PSPSeg || r.CallbacksMade() != before {
		t.Errorf("剖面關閉時不應接手 Mouse74Off+5 的 CD FB：CS=%04X", r.CPU.Seg[cpu.CS])
	}
}

func TestMouse459NeedsServiceLayer(t *testing.T) {
	m := pm459(t, []byte{0xEB, 0xFE})
	m.QueueCallback(q459(1))
	wantStepErr(t, m, "沒有服務層", "服務層")
	if len(m.cbQueue) != 1 || m.pic.isr4 {
		t.Error("沒有服務層：不遞送")
	}
}

func TestMouse459QueueLimit(t *testing.T) {
	m, _ := mouse459(t, nil)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	for i := 0; i < cbQueueMax; i++ {
		m.QueueCallback(q459(1))
	}
	mustStep(t, m, "4096 筆")
	m.QueueCallback(q459(1))
	if !m.pic.overflow || len(m.cbQueue) != cbQueueMax {
		t.Fatalf("第 4097 筆應設溢位旗標、不排入：%t %d", m.pic.overflow, len(m.cbQueue))
	}
	wantStepErr(t, m, "溢位", "佇列已滿")
	wantStepErr(t, m, "溢位之後每步", "佇列已滿")
	r := New()
	mustNil(t, "LoadCOM", r.LoadCOM([]byte{0xEB, 0xFE}))
	for i := 0; i < 5000; i++ {
		r.QueueCallback(q459(1))
	}
	if len(r.cbQueue) != 5000 {
		t.Errorf("剖面關閉：排入 5000 筆不出錯：%d", len(r.cbQueue))
	}
}

// ---- 第 11、24、31 項：佇列進 PM、升級、Snapshot -------------------------------------------------

func TestMouse459QueueInPM(t *testing.T) {
	for _, n := range []int{0, 1, 3} {
		src, _ := mouse459(t, nil)
		src.CPU.SetFlags(src.CPU.Flags &^ cpu.IF)
		for i := 0; i < n; i++ {
			src.QueueCallback(QueuedCall{Seg: uint16(i), Off: 2, AX: 3, BX: 4, CX: 5, DX: 6, SI: 7, DI: uint16(8 + i)})
		}
		data := saveBytes(t, src)
		if s := decodeStateForTest(t, data); len(s.PM) != 148+16*n || binary.LittleEndian.Uint16(s.PM[146:]) != uint16(n) {
			t.Errorf("佇列 %d 筆：PM 長度 %d", n, len(s.PM))
		}
		dst, _ := mouse459(t, nil)
		mustNil(t, "LoadState", dst.LoadState(bytes.NewReader(data)))
		if len(dst.cbQueue) != n {
			t.Fatalf("佇列 %d 筆讀回 %d", n, len(dst.cbQueue))
		}
		for i := range dst.cbQueue {
			if dst.cbQueue[i] != src.cbQueue[i] {
				t.Errorf("佇列第 %d 筆 %+v，應 %+v", i, dst.cbQueue[i], src.cbQueue[i])
			}
		}
	}
	// 升級：IVT[74h] 仍是 per-vector stub 時改成 Mouse74Off 並寫三段樁；已被程式改過時不動並記下。
	src, _ := mouse459(t, nil)
	for _, c := range []struct {
		name    string
		ivt     [2]uint16
		rewrite bool
	}{{"per-vector stub", [2]uint16{StubSeg, StubOff(0x74)}, true}, {"已被改過", [2]uint16{0x1234, 0x5678}, false}} {
		data := v3State(t, src, 3, func(st *machineState) {
			binary.LittleEndian.PutUint16(st.Mem[0x74*4:], c.ivt[1])
			binary.LittleEndian.PutUint16(st.Mem[0x74*4+2:], c.ivt[0])
		})
		m, _ := mouse459(t, nil)
		mustNil(t, "升級", m.LoadState(bytes.NewReader(data)))
		seg, off := m.ReadSup16(0x74*4+2), m.ReadSup16(0x74*4)
		base := uint32(StubSeg) * 16
		stubs := bytes.Equal(m.Mem[base+Mouse74Off:base+Mouse74Off+17], mouse74Code[:])
		switch {
		case c.rewrite && (seg != StubSeg || off != Mouse74Off || !stubs):
			t.Errorf("%s：IVT[74h]=%04X:%04X 樁=%t", c.name, seg, off, stubs)
		case !c.rewrite && (seg != 0x1234 || off != 0x5678 || !strings.Contains(m.picd.upgradeIV, "1234:5678") || !stubs):
			t.Errorf("%s：IVT[74h]=%04X:%04X 紀錄 %q", c.name, seg, off, m.picd.upgradeIV)
		}
	}
}

func TestMouse459SnapshotWithQueue(t *testing.T) {
	prep := func() *Machine {
		m, _ := mouse459(t, nil)
		m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
		setIVT(m, 0x08, PSPSeg, h459EOI0)
		m.IRQ0Every = 1000
		m.nextIRQ0 = m.Steps + 50
		for i := uint16(1); i <= 3; i++ {
			m.QueueCallback(q459(i))
		}
		m.pic.isr, m.pic.isr4, m.pic.uir = 7, true, true // 跑起來後由程式的 EOI 與 UIR 樁清掉
		m.Out8(0x21, 0x00)
		m.pic.readISR = [2]bool{true, true}
		m.pic.stuckCnt[stuckIRQ12], m.pic.stuckLast[stuckIRQ12] = 7, m.Steps
		return m
	}
	m := prep()
	want, wantQ := m.pic, append([]QueuedCall(nil), m.cbQueue...)
	s := m.Snapshot()
	m.pic = picState{slaveIMR: 0x33}
	m.cbQueue = nil
	m.picMask = 0xFF
	m.Restore(s)
	if m.pic != want || len(m.cbQueue) != 3 || m.cbQueue[2] != wantQ[2] || m.picMask != 0 {
		t.Fatalf("Restore 後應逐項等於快照：%+v 佇列 %d", m.pic, len(m.cbQueue))
	}
	run := func(m *Machine) (uint64, [5]uint64, [2]uint64, uint64) {
		// 清掉三個 ISR 與 UIR（程式自己送 EOI 的情形以宿主寫埠模擬），然後跑。
		m.Out8(0x20, 0x20)
		m.Out8(0x20, 0x20)
		m.Out8(0x20, 0x20)
		m.Out8(0xA0, 0x20)
		m.pic.uir = false
		d0 := m.Deliveries()
		for i := 0; i < 3000; i++ {
			mustStep(t, m, "跑")
		}
		return m.Deliveries() - d0, m.picd.eoiMaster, m.picd.eoiSlave, m.CallbacksMade()
	}
	d1, e1, s1, c1 := run(m)
	ref := prep()
	d2, e2, s2, c2 := run(ref)
	if d1 != d2 || e1 != e2 || s1 != s2 || c1 != c2 {
		t.Errorf("Restore 後與直接跑不同：送達 %d／%d EOI %v／%v 從片 %v／%v 回呼 %d／%d", d1, d2, e1, e2, s1, s2, c1, c2)
	}
}

// ---- 第 32 項（IRQ12）：樁尾 EOI 被拿掉時 ISR2 卡住 ------------------------------------------------

func TestMouse459StuckISR2(t *testing.T) {
	m, _ := mouse459(t, nil)
	base := uint32(StubSeg) * 16
	// 樁尾的 out A0h,al；out 20h,al 換成 NOP（N2 類突變，但只在這台機器的記憶體）。
	for i := uint32(10); i < 14; i++ {
		m.Mem[base+Mouse74Off+i] = 0x90
	}
	m.IRQ0Every = 1000 // L = 10000
	setIVT(m, 0x08, PSPSeg, h459EOI0)
	m.nextIRQ0 = m.Steps + 1
	m.QueueCallback(q459(1))
	m.QueueCallback(q459(2))
	var err error
	for i := 0; i < 50_000 && err == nil; i++ {
		err = m.Step()
	}
	if err == nil || !strings.Contains(err.Error(), "IRQ12") || !strings.Contains(err.Error(), "ISR2=1") {
		t.Errorf("IRQ12 在 ISR2 卡住應回錯誤：%v", err)
	}
}
