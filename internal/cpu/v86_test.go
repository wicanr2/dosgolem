package cpu

import (
	"errors"
	"strings"
	"testing"
)

// docs/spec/436-tss-task-return-v86-entry §6 第 4 項：§4 表每列在 IOPL=3 與 IOPL<3 各一例；popf、iret 不改 IOPL；
// pushfd 高半為 0；I/O 點陣圖。V86 以替身的 VM()=true 表示（核心的 V86 判定只看 c.Sys.VM()）。

// v86Sys 是 VM=1 的 SysBus 替身：TR 可設，ReadSup16 讀測試匯流排。
type v86Sys struct {
	fakeSys
	bus   *ioBus
	trSel uint16
	tr    SegCache
}

func (s *v86Sys) VM() bool                    { return true }
func (s *v86Sys) User() bool                  { return true }
func (s *v86Sys) TR() (uint16, SegCache)      { return s.trSel, s.tr }
func (s *v86Sys) ReadSup16(a uint32) uint16   { return uint16(s.bus.mem[a]) | uint16(s.bus.mem[a+1])<<8 }
func (s *v86Sys) ReadSup8(a uint32) uint8     { return s.bus.mem[a] }
func (s *v86Sys) WriteSup8(a uint32, v uint8) { s.bus.mem[a] = v }

// ioBus 記下每一次埠存取。
type ioBus struct {
	*testBus
	ins, outs []uint16
}

func (b *ioBus) In8(p uint16) uint8     { b.ins = append(b.ins, p); return 0x5A }
func (b *ioBus) Out8(p uint16, v uint8) { b.outs = append(b.outs, p) }

const (
	v86TSS   = 0x80000
	v86Code  = 0x1000
	v86Stack = 0x2000
)

// newV86CPU：CS:IP=1000:0000、SS:SP=2000:0100；TR=10h 指向 80000h 的 386 TSS（界限 2068h、點陣圖位移 68h、全 0）。
func newV86CPU(t *testing.T, iopl uint16, code []byte) (*CPU, *ioBus, *v86Sys) {
	t.Helper()
	b := &ioBus{testBus: newTestBus()}
	c := New(b)
	c.Model = Model80386
	c.SetFlags(iopl<<12 | IF)
	s := &v86Sys{bus: b, trSel: 0x10, tr: SegCache{Base: v86TSS, Limit: 0x2068, Attr: 0x8B}}
	c.Sys = s
	c.Seg[CS], c.IP = v86Code, 0
	c.Seg[SS], c.R[SP] = v86Stack, 0x100
	b.mem[v86TSS+0x66] = 0x68
	copy(b.mem[v86Code*16:], code)
	return c, b, s
}

func stackAt(b *ioBus, sp uint16) uint32 { return v86Stack*16 + uint32(sp) }

func wantV86Fail(t *testing.T, c *CPU, what, contains string) {
	t.Helper()
	err := c.Step()
	if err == nil || errors.Is(err, ErrV86Event) || !strings.Contains(err.Error(), contains) {
		t.Errorf("%s：應為一般失敗（含 %q），實際 %v", what, contains, err)
	}
}

// wantV86GP0 是 435 §2.2：記 #GP(0) 事件，Step 回 ErrV86Event、IP 回本道起點（測試的程式都從 IP 0 起），
// 框架 EIP 為起點，Reason 保留 436 的診斷字串。
func wantV86GP0(t *testing.T, c *CPU, what, contains string) {
	t.Helper()
	opIP := c.IP
	err := c.Step()
	if !errors.Is(err, ErrV86Event) {
		t.Errorf("%s：應回 ErrV86Event，實際 %v", what, err)
		return
	}
	ev := c.V86Event()
	if ev.Kind != V86Exception || ev.Vector != 13 || !ev.HasError || ev.ErrorCode != 0 || ev.FrameEIP != uint32(opIP) ||
		!strings.Contains(ev.Reason, contains) {
		t.Errorf("%s：事件應為 #GP(0)、框架 EIP %04X、原因含 %q：%+v", what, opIP, contains, ev)
	}
	if c.IP != opIP {
		t.Errorf("%s：IP 應回本道起點 %04X，實際 %04X", what, opIP, c.IP)
	}
}

func TestV86CliSti(t *testing.T) {
	c, _, _ := newV86CPU(t, 3, []byte{0xFA, 0xFB}) // cli；sti
	if err := c.Step(); err != nil || c.Flag(IF) {
		t.Errorf("IOPL=3 的 cli：IF=%v err=%v", c.Flag(IF), err)
	}
	if err := c.Step(); err != nil || !c.Flag(IF) {
		t.Errorf("IOPL=3 的 sti：IF=%v err=%v", c.Flag(IF), err)
	}
	for _, op := range []byte{0xFA, 0xFB} {
		c, _, _ = newV86CPU(t, 2, []byte{op})
		fl := c.Flags
		wantV86GP0(t, c, "IOPL=2 的 cli／sti", "IOPL=2")
		if c.Flags != fl {
			t.Errorf("%02X 失敗時旗標不得改變：%04X→%04X", op, fl, c.Flags)
		}
	}
}

func TestV86PushfPopf(t *testing.T) {
	// pushf；pushfd（高半 0）。
	c, b, _ := newV86CPU(t, 3, []byte{0x9C, 0x66, 0x9C})
	for i := uint32(0); i < 0x100; i++ {
		b.mem[stackAt(b, 0)+i] = 0xFF
	}
	if err := c.Step(); err != nil || c.R[SP] != 0xFE || uint16(b.mem[stackAt(b, 0xFE)])|uint16(b.mem[stackAt(b, 0xFF)])<<8 != c.Flags {
		t.Errorf("IOPL=3 的 pushf：SP=%04X err=%v", c.R[SP], err)
	}
	if err := c.Step(); err != nil || c.R[SP] != 0xFA {
		t.Fatalf("IOPL=3 的 pushfd：SP=%04X err=%v", c.R[SP], err)
	}
	if b.mem[stackAt(b, 0xFC)] != 0 || b.mem[stackAt(b, 0xFD)] != 0 {
		t.Errorf("pushfd 的高半應為 0：% X", b.mem[stackAt(b, 0xFA):stackAt(b, 0xFE)])
	}
	for _, code := range [][]byte{{0x9C}, {0x66, 0x9C}, {0x9D}, {0x66, 0x9D}} {
		c, _, _ = newV86CPU(t, 1, code)
		wantV86GP0(t, c, "IOPL=1 的 pushf／popf", "IOPL=1")
		if c.R[SP] != 0x100 {
			t.Errorf("% X 失敗時 SP 不得改變：%04X", code, c.R[SP])
		}
	}
	// popf：IOPL 保留 3，其餘照彈出值（含 NT）。
	c, b, _ = newV86CPU(t, 3, []byte{0x9D, 0x66, 0x9D})
	b.mem[stackAt(b, 0x100)], b.mem[stackAt(b, 0x101)] = 0xD5, 0x48 // 48D5h：IOPL 0、NT、OF、SF…
	copy(b.mem[stackAt(b, 0x102):], []byte{0x01, 0x10, 0xFF, 0xFF}) // FFFF1001h：IOPL 1、CF、高半全 1
	mustCPUStep(t, c, "popf")
	if c.Flags != 0x78D7 {
		t.Errorf("popf 之後 FLAGS=%04X，應為 78D7（IOPL 保留 3）", c.Flags)
	}
	mustCPUStep(t, c, "popfd")
	if c.Flags != 0x3003 || c.R[SP] != 0x106 {
		t.Errorf("popfd 之後 FLAGS=%04X SP=%04X，應為 3003（IOPL 保留 3、高半丟棄）、106", c.Flags, c.R[SP])
	}
}

func mustCPUStep(t *testing.T, c *CPU, what string) {
	t.Helper()
	if err := c.Step(); err != nil {
		t.Fatalf("%s：%v", what, err)
	}
}

func TestV86Iret(t *testing.T) {
	c, b, _ := newV86CPU(t, 3, []byte{0xCF})
	copy(b.mem[stackAt(b, 0x100):], []byte{0x34, 0x12, 0x00, 0x30, 0xD5, 0x00}) // IP 1234、CS 3000、FLAGS 00D5（IOPL 0）
	mustCPUStep(t, c, "iret")
	if c.IP != 0x1234 || c.Seg[CS] != 0x3000 || c.R[SP] != 0x106 || c.Flags != 0x30D7 {
		t.Errorf("iret：CS:IP=%04X:%04X SP=%04X FLAGS=%04X（IOPL 應保留 3）", c.Seg[CS], c.IP, c.R[SP], c.Flags)
	}
	c, b, _ = newV86CPU(t, 3, []byte{0x66, 0xCF})
	copy(b.mem[stackAt(b, 0x100):], []byte{0x34, 0x12, 0, 0, 0x00, 0x30, 0xAA, 0xAA, 0xD5, 0x40, 0x02, 0x00}) // EFLAGS 000240D5
	mustCPUStep(t, c, "iretd")
	if c.IP != 0x1234 || c.Seg[CS] != 0x3000 || c.R[SP] != 0x10C || c.Flags != 0x70D7 {
		t.Errorf("iretd：CS:IP=%04X:%04X SP=%04X FLAGS=%04X", c.Seg[CS], c.IP, c.R[SP], c.Flags)
	}
	c, b, _ = newV86CPU(t, 3, []byte{0x66, 0xCF})
	copy(b.mem[stackAt(b, 0x100):], []byte{0x34, 0x12, 0x01, 0, 0x00, 0x30, 0, 0, 0x02, 0x30, 0, 0})
	wantV86Fail(t, c, "iretd 的 EIP 高半非 0", "EIP")
	if c.R[SP] != 0x100 || c.Seg[CS] != v86Code {
		t.Errorf("iretd 失敗時 SP、CS 不得改變：SP=%04X CS=%04X", c.R[SP], c.Seg[CS])
	}
	for _, code := range [][]byte{{0xCF}, {0x66, 0xCF}} {
		c, _, _ = newV86CPU(t, 0, code)
		wantV86GP0(t, c, "IOPL=0 的 iret", "IOPL=0")
		if c.R[SP] != 0x100 || c.Seg[CS] != v86Code {
			t.Errorf("% X 失敗時 SP、CS 不得改變", code)
		}
	}
}

func TestV86IOBitmap(t *testing.T) {
	bitmap := func(b *ioBus, port uint16) { b.mem[v86TSS+0x68+uint32(port/8)] |= 1 << (port & 7) }
	forms := []struct {
		name string
		code []byte
		port uint16
		out  bool
	}{
		{"in al,imm", []byte{0xE4, 0x61}, 0x61, false},
		{"out imm,al", []byte{0xE6, 0x61}, 0x61, true},
		{"in al,dx", []byte{0xEC}, 0x3C9, false},
		{"out dx,al", []byte{0xEE}, 0x3C9, true},
		{"insb", []byte{0x6C}, 0x3C9, false},
		{"outsb", []byte{0x6E}, 0x3C9, true},
	}
	for _, iopl := range []uint16{0, 3} {
		for _, f := range forms {
			c, b, _ := newV86CPU(t, iopl, f.code)
			c.R[DX] = 0x3C9
			mustCPUStep(t, c, f.name+"（位元為 0）")
			if len(b.ins)+len(b.outs) != 1 {
				t.Errorf("IOPL=%d %s：位元為 0 應存取一次（in %v out %v）", iopl, f.name, b.ins, b.outs)
			}
			c, b, _ = newV86CPU(t, iopl, f.code)
			c.R[DX] = 0x3C9
			bitmap(b, f.port)
			wantV86GP0(t, c, f.name+"（位元為 1）", "點陣圖")
			if len(b.ins)+len(b.outs) != 0 {
				t.Errorf("IOPL=%d %s：被擋時不得存取", iopl, f.name)
			}
		}
	}
	// word 存取：第二個埠被擋時第一個埠也不存取（out、in、outsw、insw）。
	for _, code := range [][]byte{{0xEF}, {0xED}, {0x6F}, {0x6D}, {0xE7, 0x20}, {0xE5, 0x20}} {
		c, b, _ := newV86CPU(t, 3, code)
		c.R[DX] = 0x20
		bitmap(b, 0x21)
		wantV86GP0(t, c, "word 存取第二個埠被擋", "點陣圖")
		if len(b.ins)+len(b.outs) != 0 {
			t.Errorf("% X：第一個埠也不得存取（in %v out %v）", code, b.ins, b.outs)
		}
	}
	// rep outsb：每次迭代前一次檢查完；被擋時一次都不送、CX 不變。CX=0 時不檢查。
	c, b, _ := newV86CPU(t, 3, []byte{0xF3, 0x6E})
	c.R[DX], c.R[CX] = 0x3C9, 3
	mustCPUStep(t, c, "rep outsb")
	if len(b.outs) != 3 {
		t.Errorf("rep outsb 應送 3 次：%v", b.outs)
	}
	c, b, _ = newV86CPU(t, 3, []byte{0xF3, 0x6E})
	c.R[DX], c.R[CX] = 0x3C9, 3
	bitmap(b, 0x3C9)
	wantV86GP0(t, c, "rep outsb 被擋", "點陣圖")
	if len(b.outs) != 0 || c.R[CX] != 3 {
		t.Errorf("rep outsb 被擋：outs %v CX=%d", b.outs, c.R[CX])
	}
	c, _, s := newV86CPU(t, 3, []byte{0xF3, 0x6E})
	c.R[DX], c.R[CX] = 0x3C9, 0
	s.trSel, s.tr = 0, SegCache{}
	mustCPUStep(t, c, "rep outsb、CX=0（TR 無效也不檢查）")
	// 位移超過界限、TR 無效、TR 不是 386 TSS。
	c, b, _ = newV86CPU(t, 3, []byte{0xEC})
	b.mem[v86TSS+0x66], b.mem[v86TSS+0x67] = 0x69, 0x20
	wantV86GP0(t, c, "點陣圖位移超過界限", "界限")
	c, b, s = newV86CPU(t, 3, []byte{0xEC})
	s.trSel, s.tr = 0, SegCache{}
	wantV86GP0(t, c, "TR 無效", "TR")
	c, b, s = newV86CPU(t, 3, []byte{0xEC})
	s.tr.Attr = 0x83 // 286 TSS（忙碌）
	wantV86GP0(t, c, "TR 不是 386 TSS", "386 TSS")
	if len(b.ins) != 0 {
		t.Error("TR 檢查失敗時不得存取")
	}
}

func TestV86HltAndInterrupts(t *testing.T) {
	c, _, _ := newV86CPU(t, 3, []byte{0xF4})
	wantV86GP0(t, c, "hlt", "HLT")
	if c.Halted {
		t.Error("hlt 記事件時不得設 Halted")
	}
	// 435 §4：V86 的 CD nn、CC、CE 先呼叫 IntHook；回 true 就是服務完成，回 false 才記軟體中斷事件（框架 EIP 為下一道）。
	for _, x := range []struct {
		name string
		code []byte
		kind V86EventKind
		vec  uint8
		prep func(c *CPU)
	}{
		{"int 21h", []byte{0xCD, 0x21}, V86SoftInt, 0x21, nil},
		{"int 3（CD 03）", []byte{0xCD, 0x03}, V86SoftInt, 3, nil},
		{"int3（CC）", []byte{0xCC}, V86Int3, 3, nil},
		{"into", []byte{0xCE}, V86Into, 4, func(c *CPU) { c.setFlag(OF, true) }},
	} {
		for _, iopl := range []uint16{0, 3} {
			for _, serve := range []bool{true, false} {
				c, _, _ := newV86CPU(t, iopl, x.code)
				hooked := 0
				c.IntHook = func(_ *CPU, n uint8) bool {
					hooked++
					if n != x.vec {
						t.Errorf("%s：hook 收到向量 %02X", x.name, n)
					}
					return serve
				}
				if x.prep != nil {
					x.prep(c)
				}
				err := c.Step()
				if hooked != 1 || c.R[SP] != 0x100 {
					t.Errorf("IOPL=%d %s：hook 應先被呼叫一次、不推堆疊（hook %d 次、SP=%04X）", iopl, x.name, hooked, c.R[SP])
				}
				if serve {
					if err != nil || c.IP != uint16(len(x.code)) {
						t.Errorf("IOPL=%d %s：hook 服務後應照常前進（IP=%04X err=%v）", iopl, x.name, c.IP, err)
					}
					continue
				}
				if !errors.Is(err, ErrV86Event) {
					t.Fatalf("IOPL=%d %s：hook 回 false 應回 ErrV86Event，實際 %v", iopl, x.name, err)
				}
				ev := c.V86Event()
				if ev.Kind != x.kind || ev.Vector != x.vec || ev.HasError || ev.FrameEIP != uint32(len(x.code)) || c.IP != 0 {
					t.Errorf("IOPL=%d %s：事件 %+v，IP=%04X（應為種類 %d、向量 %02X、框架 EIP 下一道、IP 回起點）",
						iopl, x.name, ev, c.IP, x.kind, x.vec)
				}
			}
		}
	}
	// 例外（#DE、#BR）：不呼叫 IntHook，框架 EIP 為起點，旗標為指令前的值（386 的 #DE 不改旗標），不記 DivErrors。
	for _, x := range []struct {
		name string
		code []byte
		vec  uint8
		prep func(c *CPU, b *ioBus)
	}{
		{"div bl（除以零）", []byte{0xF6, 0xF3}, 0, func(c *CPU, _ *ioBus) { c.R[AX] = 0x1234 }},
		{"div bx（商溢位）", []byte{0xF7, 0xF3}, 0, func(c *CPU, _ *ioBus) { c.R[DX], c.R[BX] = 5, 1 }},
		{"idiv bl（除以零）", []byte{0xF6, 0xFB}, 0, func(c *CPU, _ *ioBus) { c.R[AX] = 0x8000 }},
		{"idiv bx（除以零）", []byte{0xF7, 0xFB}, 0, func(c *CPU, _ *ioBus) { c.R[DX] = 0x7FFF }},
		{"aam 0", []byte{0xD4, 0x00}, 0, func(c *CPU, _ *ioBus) { c.R[AX] = 0x00FF }},
		{"66 div ebx（除以零）", []byte{0x66, 0xF7, 0xF3}, 0, nil},
		{"bound 越界", []byte{0x62, 0x06, 0x00, 0x02}, 5, func(c *CPU, b *ioBus) { c.Seg[DS] = 0x3000; c.R[AX] = 9 }},
	} {
		c, b, _ := newV86CPU(t, 3, x.code)
		hooked := 0
		c.IntHook = func(*CPU, uint8) bool { hooked++; return true }
		c.SetFlags(c.Flags | CF | OF | SF) // 讓 divFlags／logic 會改到的位元先有值
		if x.prep != nil {
			x.prep(c, b)
		}
		fl, regs := c.Flags, c.R
		err := c.Step()
		if !errors.Is(err, ErrV86Event) {
			t.Fatalf("%s：應回 ErrV86Event，實際 %v", x.name, err)
		}
		ev := c.V86Event()
		if ev.Kind != V86Exception || ev.Vector != x.vec || ev.HasError || ev.FrameEIP != 0 || c.IP != 0 {
			t.Errorf("%s：事件 %+v、IP=%04X（應為例外、向量 %d、框架 EIP 0）", x.name, ev, c.IP, x.vec)
		}
		if hooked != 0 || c.Flags != fl || c.R != regs || len(c.DivErrors) != 0 {
			t.Errorf("%s：hook %d 次、旗標 %04X→%04X、暫存器 %v→%v、DivErrors %d（應全不變）",
				x.name, hooked, fl, c.Flags, regs, c.R, len(c.DivErrors))
		}
	}
	// 事件取走後清掉 c.ev：下一道的一般失敗不得被誤判成事件。
	c, b, _ := newV86CPU(t, 3, []byte{0xF4})
	_ = c.Step()
	c.Seg[CS], c.IP = v86Code, 0
	copy(b.mem[v86Code*16:], []byte{0x66, 0xCF})
	copy(b.mem[stackAt(b, 0x100):], []byte{0x34, 0x12, 0x01, 0, 0x00, 0x30, 0, 0, 0x02, 0x30, 0, 0})
	wantV86Fail(t, c, "事件之後的 iretd（EIP 高半非 0）", "EIP")
	// 先記的一般失敗為準：Interrupt 的第二道防線之後，同一步的 cli（IOPL<3）不記事件。
	c, _, _ = newV86CPU(t, 0, []byte{0xFA})
	c.Interrupt(8)
	wantV86Fail(t, c, "Interrupt 之後的 cli", "Interrupt(08h)")
	// 第二道防線（435 §4）：Interrupt 在 V86 仍不動狀態，錯誤在下一次 Step 回報。
	c, _, _ = newV86CPU(t, 3, []byte{0x90})
	c.Interrupt(8)
	if c.R[SP] != 0x100 || c.Seg[CS] != v86Code || c.IP != 0 {
		t.Errorf("Interrupt 不得動狀態：SP=%04X CS:IP=%04X:%04X", c.R[SP], c.Seg[CS], c.IP)
	}
	wantV86Fail(t, c, "Interrupt 之後的 Step", "Interrupt(08h)")
	// FarCallWithReturn 在 V86 照常執行：以 V86 的 SS:SP 推入返回位址（段值×16）。
	c, b, _ = newV86CPU(t, 3, []byte{0x90})
	c.FarCallWithReturn(0x1234, 0x10, 0x5678, 0x9ABC)
	if c.R[SP] != 0xFC || c.Seg[CS] != 0x1234 || c.IP != 0x10 ||
		uint16(b.mem[stackAt(b, 0xFC)])|uint16(b.mem[stackAt(b, 0xFD)])<<8 != 0x9ABC ||
		uint16(b.mem[stackAt(b, 0xFE)])|uint16(b.mem[stackAt(b, 0xFF)])<<8 != 0x5678 {
		t.Errorf("FarCallWithReturn 應照常執行：SP=%04X CS:IP=%04X:%04X", c.R[SP], c.Seg[CS], c.IP)
	}
}
