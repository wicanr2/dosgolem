package dos

import (
	"encoding/binary"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/435-v86-hooks-and-idt-delivery §7 第 4、5 項：V86 往返與服務層條件。
//
// 合成環境仿 §1.1（同 internal/machine 的 v86deliver_test.go）：GDT 90000h（18h 碼段基底 70000h、50h 向下展開堆疊
// 基底 50000h、10h 386 TSS 82000h）、IDT 84000h（256 項中斷閘 DPL 0，偏移 47DEh+4n，處理常式是 66 CF）、
// SS0:ESP0=50h:1400h。V86 程式在 4000:0100，堆疊 4800:0200，IOPL=3。
//
// ring 0 處理常式的「JEMM 反射」以 Go 在兩步之間模擬（jemmReflect）：改 V86 堆疊（推 FLAGS、CS、IP）、讀 IVT[nn]
// 寫回框架、清框架的 IF、TF、丟掉錯誤碼；接著 Machine.Step 執行真正的 66 CF 回 V86。

const (
	vGDT, vIDT, vTSS = 0x90000, 0x84000, 0x82000
	vCode, vSS0      = 0x70000, 0x50000
	vGate            = 0x47DE
	vProgSeg         = 0x4000
	vStackSeg        = 0x4800
	vSP              = 0x0200
)

func vPut32(m *machine.Machine, a, v uint32) { binary.LittleEndian.PutUint32(m.Mem[a:], v) }
func vGet16(m *machine.Machine, a uint32) uint16 {
	return binary.LittleEndian.Uint16(m.Mem[a:])
}

func vDesc(m *machine.Machine, sel uint16, base, limit uint32, access uint8) {
	a := vGDT + uint32(sel&^7)
	d := [8]byte{byte(limit), byte(limit >> 8), byte(base), byte(base >> 8), byte(base >> 16), access, byte(limit>>16) & 0xF, byte(base >> 24)}
	copy(m.Mem[a:], d[:])
}

// newV86DOS：PE=1、VM=1 的機器裝好服務層與 IDT 環境，V86 程式為 code。
func newV86DOS(t *testing.T, code []byte) (*machine.Machine, *DOS) {
	t.Helper()
	m := machine.New()
	if err := m.SetExtendedMemoryKB(4096); err != nil {
		t.Fatal(err)
	}
	if err := m.SetProtectedModeProfile(true); err != nil {
		t.Fatal(err)
	}
	m.IRQ0Every = 0
	d := New(m, t.TempDir())
	d.Install()
	d.freeSeg = 0x2000
	d.Calls = map[Call]int{}

	vDesc(m, 0x18, vCode, 0xFFFF, 0x9A)
	vDesc(m, 0x50, vSS0, 0, 0x96)
	vDesc(m, 0x10, vTSS, 0x2068, 0x8B)
	m.SetGDTR(vGDT, 0xFF)
	for v := uint32(0); v < 256; v++ {
		off := vGate + 4*v
		vPut32(m, vIDT+8*v, off|0x18<<16)
		vPut32(m, vIDT+8*v+4, 0x8E00)
		m.Mem[vCode+off], m.Mem[vCode+off+1] = 0x66, 0xCF
	}
	m.SetIDTR(vIDT, 0x7FF)
	vPut32(m, vTSS+4, 0x1400)
	vPut32(m, vTSS+8, 0x50)
	vPut32(m, vTSS+0x64, 0x68<<16)
	m.SetTR(0x10, machine.SegCache{Base: vTSS, Limit: 0x2068, Attr: 0x8B})
	if err := m.SetCR0(1); err != nil {
		t.Fatal(err)
	}
	if err := m.SetVM(true); err != nil {
		t.Fatal(err)
	}
	c := m.CPU
	copy(m.Mem[vProgSeg*16+0x100:], code)
	c.Seg[cpu.CS], c.IP = vProgSeg, 0x100
	c.Seg[cpu.SS], c.R[cpu.SP] = vStackSeg, vSP
	c.Seg[cpu.DS], c.Seg[cpu.ES] = vProgSeg, vProgSeg
	c.SetFlags(0x3000 | cpu.IF)
	return m, d
}

func vStep(t *testing.T, m *machine.Machine, what string) {
	t.Helper()
	if err := m.Step(); err != nil {
		t.Fatalf("%s：%v", what, err)
	}
}

// jemmReflect 模擬 JEMM 的 4EE8–4F5C（§1.1）：剛從 V86 的 CD nn 以 #GP(nn*8+2) 進入 ring 0。
func jemmReflect(t *testing.T, m *machine.Machine) {
	t.Helper()
	c := m.CPU
	if !c.Segmented() {
		t.Fatal("jemmReflect：應在 ring 0")
	}
	f := vSS0 + uint32(c.R[cpu.SP])
	errCode := binary.LittleEndian.Uint32(m.Mem[f:])
	vec := uint8((errCode - 2) / 8)
	eip := binary.LittleEndian.Uint32(m.Mem[f+4:])
	cs := binary.LittleEndian.Uint32(m.Mem[f+8:])
	efl := binary.LittleEndian.Uint32(m.Mem[f+12:])
	esp := binary.LittleEndian.Uint32(m.Mem[f+16:])
	ss := binary.LittleEndian.Uint32(m.Mem[f+20:])
	nsp := uint16(esp) - 6
	lin := ss*16 + uint32(nsp)
	binary.LittleEndian.PutUint16(m.Mem[lin:], uint16(eip)+2)
	binary.LittleEndian.PutUint16(m.Mem[lin+2:], uint16(cs))
	binary.LittleEndian.PutUint16(m.Mem[lin+4:], uint16(efl))
	vPut32(m, f+16, esp&0xFFFF0000|uint32(nsp))
	vPut32(m, f+4, uint32(vGet16(m, uint32(vec)*4)))
	vPut32(m, f+8, uint32(vGet16(m, uint32(vec)*4+2)))
	vPut32(m, f+12, efl&^uint32(cpu.IF|cpu.TF))
	c.R[cpu.SP] += 4 // 丟掉錯誤碼
}

// roundTrip 跑「V86 CD nn → ring 0 反射 → 66 CF → StubSeg 的 stub → 服務層 → CF（iret）」。
func roundTrip(t *testing.T, m *machine.Machine, what string) {
	t.Helper()
	vStep(t, m, what+"：V86 的 CD nn 經 IDT")
	jemmReflect(t, m)
	vStep(t, m, what+"：ring 0 的 66 CF 回 V86")
	if m.CPU.Seg[cpu.CS] != machine.StubSeg || m.CPU.Segmented() || !m.VM() {
		t.Fatalf("%s：應回到 V86 的 StubSeg，實際 CS=%04X segmented=%v", what, m.CPU.Seg[cpu.CS], m.CPU.Segmented())
	}
	vStep(t, m, what+"：StubSeg 的 stub 由服務層處理")
	vStep(t, m, what+"：stub 結尾的 CF")
}

func TestV86RoundTripInt21(t *testing.T) {
	for _, x := range []struct {
		name   string
		file   string
		cfIn   bool
		cfWant bool
	}{
		{"AH=3Dh 開存在的檔清 CF", "YES.BIN", true, false},
		{"AH=3Dh 開不存在的檔設 CF", "NOPE.BIN", false, true},
	} {
		m, d := newV86DOS(t, []byte{0xCD, 0x21, 0x90})
		writeChild(t, d, "YES.BIN", []byte{1})
		c := m.CPU
		c.R[cpu.AX], c.R[cpu.DX] = 0x3D00, 0x300
		copy(m.Mem[vProgSeg*16+0x300:], append([]byte(x.file), 0))
		if x.cfIn {
			c.SetFlags(c.Flags | cpu.CF)
		}
		roundTrip(t, m, x.name)
		if c.Seg[cpu.CS] != vProgSeg || c.IP != 0x102 || c.R[cpu.SP] != vSP || c.Segmented() {
			t.Errorf("%s：應回到 CD 21 的下一道：CS:IP=%04X:%04X SP=%04X", x.name, c.Seg[cpu.CS], c.IP, c.R[cpu.SP])
		}
		if c.Flag(cpu.CF) != x.cfWant || !c.Flag(cpu.IF) {
			t.Errorf("%s：CF=%v（應 %v）、IF=%v（應還原為 1）", x.name, c.Flag(cpu.CF), x.cfWant, c.Flag(cpu.IF))
		}
		n := 0
		for k, v := range d.Calls {
			if k.Int == 0x21 {
				t.Errorf("%s：CS 不是 StubSeg 的 INT 21h 不得由服務層處理（%v）", x.name, k)
			}
			n += v
		}
		if n != 1 || d.Calls[Call{Int: 0xF2, AH: 0x3D}] != 1 {
			t.Errorf("%s：服務層應只被呼叫一次（INT F2h）：%v", x.name, d.Calls)
		}
	}
}

// per-vector stub（CD n; CF）：V86 下寫回 CF、ZF。
func TestV86RoundTripPerVectorFlags(t *testing.T) {
	// INT 16h AH=01：沒有按鍵 → ZF=1（進入時 ZF=0）。
	m, _ := newV86DOS(t, []byte{0xCD, 0x16, 0x90})
	c := m.CPU
	c.R[cpu.AX] = 0x0100
	roundTrip(t, m, "INT 16h AH=01")
	if c.IP != 0x102 || !c.Flag(cpu.ZF) {
		t.Errorf("INT 16h AH=01：IP=%04X ZF=%v（應回下一道、ZF=1）", c.IP, c.Flag(cpu.ZF))
	}
	// INT 13h AH=02（讀磁區未實作）→ CF=1（進入時 CF=0）；AH=00 → CF=0（進入時 CF=1）。
	for _, x := range []struct {
		ax      uint16
		cfIn    bool
		cfWant  bool
		comment string
	}{{0x0201, false, true, "AH=02"}, {0x0000, true, false, "AH=00"}} {
		m, _ = newV86DOS(t, []byte{0xCD, 0x13, 0x90})
		c = m.CPU
		c.R[cpu.AX] = x.ax
		if x.cfIn {
			c.SetFlags(c.Flags | cpu.CF)
		}
		roundTrip(t, m, "INT 13h "+x.comment)
		if c.IP != 0x102 || c.Flag(cpu.CF) != x.cfWant {
			t.Errorf("INT 13h %s：IP=%04X CF=%v（應 %v）", x.comment, c.IP, c.Flag(cpu.CF), x.cfWant)
		}
	}
}

// V86 下 EXEC 啟動子行程：服務返回後 CS 不在 StubSeg，子行程 SS:SP+4 的位元組不得被回寫改動。
func TestV86ExecDoesNotWriteChildStack(t *testing.T) {
	m, d := newV86DOS(t, []byte{0xCD, 0x21, 0x90})
	// 最小 MZ：檔頭 2 段、映像 20h bytes；CS:IP=0:0（mov ax,4C00h; int 21h），SS:SP=0:10h，映像 10h–1Fh 為 FFh。
	exe := make([]byte, 0x40)
	copy(exe, []byte{'M', 'Z', 0x40, 0, 1, 0, 0, 0, 2, 0, 0, 0, 0xFF, 0xFF, 0, 0, 0x10, 0, 0, 0, 0, 0, 0, 0, 0x1C, 0})
	copy(exe[0x20:], []byte{0xB8, 0x00, 0x4C, 0xCD, 0x21})
	for i := 0x30; i < 0x40; i++ {
		exe[i] = 0xFF
	}
	writeChild(t, d, "KID.EXE", exe)
	c := m.CPU
	copy(m.Mem[vProgSeg*16+0x300:], append([]byte("KID.EXE"), 0))
	pb := uint32(vProgSeg*16 + 0x380)
	for i := uint32(0); i < 14; i++ {
		m.Mem[pb+i] = 0
	}
	c.R[cpu.AX], c.R[cpu.DX], c.R[cpu.BX] = 0x4B00, 0x300, 0x380
	vStep(t, m, "CD 21 經 IDT")
	jemmReflect(t, m)
	vStep(t, m, "66 CF 回 V86")
	vStep(t, m, "StubSeg 的 CD F2：EXEC")
	if c.Seg[cpu.CS] == machine.StubSeg || len(d.procStack) != 1 {
		t.Fatalf("EXEC 應啟動子行程：CS=%04X 行程疊 %d", c.Seg[cpu.CS], len(d.procStack))
	}
	if w := vGet16(m, uint32(c.Seg[cpu.SS])*16+uint32(c.R[cpu.SP])+4); w != 0xFFFF || c.R[cpu.SP] != 0x10 {
		t.Errorf("子行程 SS:SP+4 被改動：%04X（應為 FFFF），SP=%04X", w, c.R[cpu.SP])
	}
}

// 阻塞服務（AH=01、stdin 空）以 Rewind 退回 CD 起點：不回寫；下一步重跑仍由服務層處理。
func TestV86BlockingRewindNoWriteBack(t *testing.T) {
	m, d := newV86DOS(t, []byte{0xCD, 0x21, 0x90})
	c := m.CPU
	c.R[cpu.AX] = 0x0100
	c.SetFlags(c.Flags | cpu.CF | cpu.ZF)
	vStep(t, m, "CD 21 經 IDT")
	jemmReflect(t, m)
	vStep(t, m, "66 CF 回 V86")
	c.SetFlags(c.Flags &^ (cpu.CF | cpu.ZF)) // 與堆疊上的 FLAGS 不同：回寫會看得出來
	stacked := uint32(c.Seg[cpu.SS])*16 + uint32(c.R[cpu.SP]) + 4
	before := vGet16(m, stacked)
	ip := c.IP
	for i := 1; i <= 2; i++ {
		vStep(t, m, "阻塞的 CD F2")
		if c.IP != ip || c.Seg[cpu.CS] != machine.StubSeg || vGet16(m, stacked) != before {
			t.Errorf("第 %d 次：應 Rewind 且不回寫：CS:IP=%04X:%04X 堆疊 FLAGS %04X→%04X", i, c.Seg[cpu.CS], c.IP, before, vGet16(m, stacked))
		}
		if d.Calls[Call{Int: 0xF2, AH: 0x01}] != i {
			t.Errorf("第 %d 次：重跑應仍由服務層處理：%v", i, d.Calls)
		}
	}
}

// IVT[nn] 不指 StubSeg 時，StubSeg 內（不是 StubOff(nn)）的 CD nn 經 IDT；在 StubOff(nn) 的（fromOwnStub）由服務層處理。
func TestV86StubSegHookedVector(t *testing.T) {
	const freeOff = 0xF0 * 4 // StubOff(F0h) 的位置改放 CD 1C
	for _, own := range []bool{false, true} {
		m, d := newV86DOS(t, nil)
		c := m.CPU
		binary.LittleEndian.PutUint16(m.Mem[0x1C*4:], 0x0500) // 遊戲掛了 1Ch
		binary.LittleEndian.PutUint16(m.Mem[0x1C*4+2:], vProgSeg)
		off := uint16(freeOff)
		if own {
			off = machine.StubOff(0x1C)
		} else {
			copy(m.Mem[machine.StubSeg*16+uint32(off):], []byte{0xCD, 0x1C, 0xCF})
		}
		c.Seg[cpu.CS], c.IP = machine.StubSeg, off
		vStep(t, m, "StubSeg 的 CD 1C")
		served := d.Calls[Call{Int: 0x1C}] > 0
		if own && (!served || c.Segmented() || c.IP != off+2) {
			t.Errorf("fromOwnStub：應由服務層處理：served=%v segmented=%v IP=%04X", served, c.Segmented(), c.IP)
		}
		if !own && (served || !c.Segmented()) {
			t.Errorf("IVT 被掛鉤：StubSeg 內的 CD 1C 應經 IDT：served=%v segmented=%v", served, c.Segmented())
		}
	}
}
