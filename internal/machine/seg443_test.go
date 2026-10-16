package machine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/443-segmented-32bit-code-and-stack §3：分段模式的 D=1 碼段。
//
// 環境：seg437 的 GDT（28h 為 D=1 碼段、基底 segCode；30h 為基底 1000h 的 D=0 碼段；18h 為基底 segCode 的
// D=0 碼段）。d1Prog 把程式放在 segCode，直接以 GDT 的描述元設 CS=28h、IP=0。

// setSel 以 GDT 的描述元直接設段暫存器 i 的值與快取（測試注入，不經 loadSeg）。
func setSel(m *Machine, i int, sel uint16) {
	base, _ := m.GDTR()
	idx := uint32(sel &^ 7)
	d := cpu.DecodeDescriptor(m.ReadSup32(base+idx), m.ReadSup32(base+idx+4))
	switch i {
	case cpu.FS:
		m.CPU.FSeg = sel
	case cpu.GS:
		m.CPU.GSeg = sel
	default:
		m.CPU.Seg[i] = sel
	}
	m.CPU.SetSegCache(i, d)
}

// d1Prog：CS=28h（D=1，基底 segCode）、IP=0，code 放在 segCode。其餘段同 segProg（DS、ES、SS 基底 1000h）。
func d1Prog(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := seg437(t, []byte{0x90})
	copy(m.Mem[segCode:], code)
	setSel(m, cpu.CS, sel437D1)
	m.CPU.IP = 0
	if !m.CPU.Code32() {
		t.Fatal("前提：CS=28h 應為 D=1")
	}
	return m
}

// d0Prog：同 d1Prog，但 CS=18h（D=0，基底 segCode）。
func d0Prog(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := seg437(t, []byte{0x90})
	copy(m.Mem[segCode:], code)
	setSel(m, cpu.CS, 0x18)
	m.CPU.IP = 0
	if m.CPU.Code32() {
		t.Fatal("前提：CS=18h 應為 D=0")
	}
	return m
}

func setReg32(m *Machine, r int, v uint32) { m.CPU.R[r], m.CPU.Hi[r] = uint16(v), uint16(v>>16) }

// ---- §3 第 3 項：合成程式重現 JEMM CU ----------------------------------------------

const (
	cuX    = 0x110000      // JEMM 的 XMS 區塊線性位址（434 收據）
	cu98   = cuX + 0x7780  // 98h 段基底
	cuG    = 0x00004000    // 80h 段基底（非 0）
	cuB    = 0x00200000    // [G+12Ch]：四個 64 KB 緩衝區的基底
	cuF    = 0x3000        // ES:BX 的分派框架（ES=28h 平坦，BX=3000h）
	cuESP0 = 0x00010E00    // 5B5C 之前的 ESP（高半非 0）
	cuSent = 0x5A          // 目的緩衝區的哨兵 byte
	cuGDT  = segGDT        // GDT 線性位址
	cuGLim = uint16(0x11F) // 涵蓋 110h
)

// cu18 是 18h 段 5B5C–5BA7 的位元組（img.dis 第 3408–3434 行），其後在 5BA8 補 EB FE（自旋，測試用）。
var cu18 = []byte{
	0x66, 0x8B, 0xCC, // 5B5C mov ecx,esp
	0x36, 0x66, 0x8B, 0x26, 0x04, 0x00, // 5B5F mov esp,ss:[4]
	0x66, 0x51, // 5B65 push ecx
	0xB9, 0x10, 0x01, // 5B67 mov cx,110h
	0x8E, 0xD1, // 5B6A mov ss,cx
	0x06,                         // 5B6C push es
	0x53,                         // 5B6D push bx
	0x26, 0x66, 0x8B, 0x4F, 0x18, // 5B6E mov ecx,es:[bx+18h]
	0x26, 0x66, 0x8B, 0x57, 0x14, // 5B73 mov edx,es:[bx+14h]
	0xB8, 0x80, 0x00, // 5B78 mov ax,80h
	0x8E, 0xE8, // 5B7B mov gs,ax
	0xB8, 0x28, 0x00, // 5B7D mov ax,28h
	0x8E, 0xD8, // 5B80 mov ds,ax
	0x8E, 0xC0, // 5B82 mov es,ax
	0x8E, 0xE0, // 5B84 mov fs,ax
	0xFB,       // 5B86 sti
	0x6A, 0x00, // 5B87 push 0
	0x6A, 0x18, // 5B89 push 18h
	0x6A, 0x00, // 5B8B push 0
	0x68, 0x95, 0x5B, // 5B8D push 5B95h
	0xEA, 0x15, 0x00, 0x98, 0x00, // 5B90 jmp 98:15
	0xFA,             // 5B95 cli
	0x5B,             // 5B96 pop bx
	0x07,             // 5B97 pop es
	0xB9, 0x50, 0x00, // 5B98 mov cx,50h
	0x8E, 0xD1, // 5B9B mov ss,cx
	0x66, 0x59, // 5B9D pop ecx
	0x36, 0x66, 0x89, 0x26, 0x04, 0x00, // 5B9F mov ss:[4],esp
	0x66, 0x8B, 0xE1, // 5BA5 mov esp,ecx
	0xEB, 0xFE, // 5BA8 jmp $（補）
}

// cu98code 是 98h 段 0000–008F（r2.lst 第 3921–3977 行）。0086 的 jmp 8C 佔 0086–0087，跳過的空隙是 0088–008B
// （443 §1.2 寫成 0087h–008Bh），填 CC，誤入時失敗即關閉。
var cu98code = []byte{
	0x8B, 0xD1, // 0000 mov edx,ecx
	0x83, 0xE2, 0x03, // 0002 and edx,3
	0xC1, 0xE9, 0x02, // 0005 shr ecx,2
	0x74, 0x02, // 0008 je 0C
	0xF3, 0xA5, // 000A rep movsd
	0x0B, 0xD2, // 000C or edx,edx
	0x74, 0x04, // 000E je 14
	0x8B, 0xCA, // 0010 mov ecx,edx
	0xF3, 0xA4, // 0012 rep movsb
	0xCB,                               // 0014 retf
	0x65, 0x67, 0x8B, 0x36, 0x2C, 0x01, // 0015 mov esi,gs:[012C]
	0x8B, 0xFE, // 001B mov edi,esi
	0x8B, 0xDE, // 001D mov ebx,esi
	0x33, 0xC0, // 001F xor eax,eax
	0x8A, 0xC1, // 0021 mov al,cl
	0xC1, 0xE0, 0x10, // 0023 shl eax,10h
	0x03, 0xF0, // 0026 add esi,eax
	0x33, 0xC0, // 0028 xor eax,eax
	0x8A, 0xC5, // 002A mov al,ch
	0xC1, 0xE0, 0x10, // 002C shl eax,10h
	0x03, 0xF8, // 002F add edi,eax
	0x66, 0x0B, 0xD2, // 0031 or dx,dx
	0x74, 0x05, // 0034 je 3B
	0x80, 0xFD, 0x00, // 0036 cmp ch,0
	0x74, 0x09, // 0039 je 44
	0xB9, 0x80, 0x3E, 0x00, 0x00, // 003B mov ecx,3E80h
	0xF3, 0xA5, // 0040 rep movsd
	0xEB, 0x48, // 0042 jmp 8C
	0x33, 0xC0, // 0044 xor eax,eax
	0x8A, 0xC1, // 0046 mov al,cl
	0x34, 0x03, // 0048 xor al,3
	0xC1, 0xE0, 0x10, // 004A shl eax,10h
	0x03, 0xD8, // 004D add ebx,eax
	0x87, 0xDF, // 004F xchg edi,ebx
	0xBA, 0x04, 0x00, 0x00, 0x00, // 0051 mov edx,4
	0xB9, 0x00, 0xFA, 0x00, 0x00, // 0056 mov ecx,FA00h
	0x03, 0xD9, // 005B add ebx,ecx
	0xC1, 0xE9, 0x02, // 005D shr ecx,2
	0xF3, 0xA7, // 0060 repz cmpsd
	0x74, 0x28, // 0062 je 8C
	0xC1, 0xE1, 0x02, // 0064 shl ecx,2
	0x2B, 0xD9, // 0067 sub ebx,ecx
	0x8B, 0x46, 0xFC, // 0069 mov eax,[esi-4]
	0x64, 0x89, 0x43, 0xFC, // 006C mov fs:[ebx-4],eax
	0xE3, 0x1A, // 0070 jecxz 8C
	0x8B, 0x06, // 0072 mov eax,[esi]
	0x26, 0x3B, 0x07, // 0074 cmp eax,es:[edi]
	0x74, 0xE2, // 0077 je 5B
	0x03, 0xF2, // 0079 add esi,edx
	0x03, 0xFA, // 007B add edi,edx
	0x64, 0x89, 0x03, // 007D mov fs:[ebx],eax
	0x03, 0xDA, // 0080 add ebx,edx
	0x2B, 0xCA, // 0082 sub ecx,edx
	0x75, 0xEC, // 0084 jne 72
	0xEB, 0x04, // 0086 jmp 8C
	0xCC, 0xCC, 0xCC, 0xCC, // 0088–008B（空隙；r2.lst 沒有列出，填 CC）
	0x66, 0x33, 0xC0, // 008C xor ax,ax
	0xCB, // 008F retf
}

// cuCopy18 是 18h 段 5E60–5E6E（img.dis 第 3656–3661 行），其後在 5E6F 補 EB FE（測試用；原為 mov ax,1）。
var cuCopy18 = []byte{
	0xFB,       // 5E60 sti
	0x6A, 0x00, // 5E61 push 0
	0x6A, 0x18, // 5E63 push 18h
	0x6A, 0x00, // 5E65 push 0
	0x68, 0x6F, 0x5E, // 5E67 push 5E6Fh
	0xEA, 0x00, 0x00, 0x98, 0x00, // 5E6A jmp 98:0
	0xEB, 0xFE, // 5E6F jmp $（補）
}

// physWrite／physRead：線性＝實體（未分頁），1 MB 以上在 Ext。
func physWrite(m *Machine, a uint32, b []byte) {
	for i, v := range b {
		p := a + uint32(i)
		if p < MemSize {
			m.Mem[p] = v
		} else {
			m.Ext[p-ExtBase] = v
		}
	}
}

func physRead(m *Machine, a uint32, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		p := a + uint32(i)
		if p < MemSize {
			out[i] = m.Mem[p]
		} else {
			out[i] = m.Ext[p-ExtBase]
		}
	}
	return out
}

// cuMachine 依 §3 第 3 項的組態造機器：A20 開、GDT 依 §1.3；18h 段放 cu18（自 5B5C 起）與 cuCopy18（自 5E60 起），
// 98h 段放 cu98code。起點 CS=18h:5B5C、SS=50h、ESP=cuESP0、ES=DS=28h、BX=cuF。ss110 是 110h 的 byte 6 旗標（B=1 時 4）。
func cuMachine(t testing.TB, ss110 uint8) *Machine {
	t.Helper()
	m := New()
	if err := m.SetExtendedMemoryKB(4096); err != nil {
		t.Fatal(err)
	}
	if err := m.SetProtectedModeProfile(true); err != nil {
		t.Fatal(err)
	}
	if err := m.LoadCOM([]byte{0x90}); err != nil {
		t.Fatal(err)
	}
	m.IRQ0Every = 0
	m.SetA20(true)
	if err := m.SetCR0(cr0PE); err != nil {
		t.Fatal(err)
	}
	if err := m.enterSegmented(); err != nil {
		t.Fatal(err)
	}
	putDesc(m, cuGDT, 0x18, cuX, 0xFFFF, 0x9A, 0)
	putDesc(m, cuGDT, 0x98, cu98, 0xFFFFF, 0x9A, 0x4)
	putDesc(m, cuGDT, 0x110, cuX, 0, 0x96, ss110)
	putDesc(m, cuGDT, 0x50, cuX, 0, 0x96, 0)
	putDesc(m, cuGDT, 0x28, 0, 0xFFFFF, 0x92, 0x8)
	putDesc(m, cuGDT, 0x80, cuG, 0xFFFFF, 0x92, 0)
	m.SetGDTR(cuGDT, cuGLim)
	if len(cu98code) != 0x90 {
		t.Fatalf("98h 段應為 90h bytes，實際 %X", len(cu98code))
	}
	physWrite(m, cuX+0x5B5C, cu18)
	physWrite(m, cuX+0x5E60, cuCopy18)
	physWrite(m, cu98, cu98code)
	physWrite(m, cuX+4, []byte{0x00, 0x10, 0x00, 0x00})     // ss:[4] = 1000h
	physWrite(m, cuG+0x12C, []byte{0x00, 0x00, 0x20, 0x00}) // [G+12Ch] = cuB（00200000h）
	setSel(m, cpu.CS, 0x18)
	setSel(m, cpu.SS, 0x50)
	setSel(m, cpu.ES, 0x28)
	setSel(m, cpu.DS, 0x28)
	m.CPU.IP = 0x5B5C
	setReg32(m, cpu.SP, cuESP0)
	m.CPU.R[cpu.BX] = cuF
	m.CPU.SetFlags(0)
	return m
}

// cuVariant 是一個資料變體：框架內的 ECX、EDX，四個緩衝區的內容，以及期望的目的緩衝區。
type cuVariant struct {
	name     string
	ecx, edx uint32
	setup    func(buf *[4][]byte) // 改寫緩衝區（初始為各自的偽亂數，目的另填哨兵）
}

const cuLen = 0xFA00 // 320×200

// cuBuffers 回四個 64 KB 緩衝區的初始內容（偽亂數，各不相同）。
func cuBuffers() [4][]byte {
	var b [4][]byte
	x := uint32(12345)
	for k := range b {
		b[k] = make([]byte, 0x10000)
		for i := range b[k] {
			x = x*1103515245 + 12345
			b[k][i] = byte(x >> 16)
		}
	}
	return b
}

// cuExpect 是獨立的參考模型（§3 第 3 項）：整段複製時目的＝來源；差分時來源 ≠ 比較的 dword 目的＝來源，否則保持原值。
func cuExpect(v cuVariant, buf [4][]byte) []byte {
	cl, ch := int(v.ecx&3), int(v.ecx>>8&3)
	src := buf[cl]
	want := append([]byte(nil), buf[ch]...)
	if uint16(v.edx) == 0 || ch != 0 {
		copy(want[:cuLen], src[:cuLen])
		return want
	}
	cmp := buf[cl^3]
	for i := 0; i < cuLen; i += 4 {
		if !bytes.Equal(src[i:i+4], cmp[i:i+4]) {
			copy(want[i:i+4], src[i:i+4])
		}
	}
	return want
}

// runUntil 執行到 CS:IP＝(cs, ip)（指令邊界），最多 limit 步。
func runUntil(t testing.TB, m *Machine, cs, ip uint16, limit int) int {
	t.Helper()
	for n := 0; n < limit; n++ {
		if m.CPU.Seg[cpu.CS] == cs && m.CPU.IP == ip {
			return n
		}
		if err := m.Step(); err != nil {
			t.Fatalf("執行到 %04X:%04X 之前失敗（%d 步）：%v", cs, ip, n, err)
		}
	}
	t.Fatalf("%d 步內沒有到達 %04X:%04X（停在 %04X:%04X）", limit, cs, ip, m.CPU.Seg[cpu.CS], m.CPU.IP)
	return 0
}

var cuVariants = []cuVariant{
	{name: "DX=0（整段複製，CX=0200h）", ecx: 0x0200, edx: 0},
	{name: "DX≠0、CH≠0（整段複製）", ecx: 0x0301, edx: 0x00010001},
	{name: "DX≠0、CH=0（差分複製）", ecx: 0x0001, edx: 1, setup: func(b *[4][]byte) {
		copy(b[2], b[1]) // 比較緩衝區＝來源，再挑幾處不同
		for _, i := range []int{0, 4 * 3999, 4 * 7000} {
			b[2][i+1] ^= 0xFF
		}
		for i := 4 * 100; i < 4*121; i += 4 { // 連續一串
			b[2][i+3] ^= 0x80
		}
		b[2][cuLen-4] ^= 0x01 // 最後一個 dword
		for i := range b[0] {
			b[0][i] = cuSent
		}
	}},
	{name: "DX≠0、CH=0、只有最後一個 dword 不同（repz cmpsd 以 ECX=0、ZF=0 結束）", ecx: 0x0002, edx: 0xFFFF, setup: func(b *[4][]byte) {
		copy(b[1], b[2])
		b[1][cuLen-1] ^= 0x10
		for i := range b[0] {
			b[0][i] = cuSent
		}
	}},
}

// prepCU 把變體寫進機器：框架、緩衝區；回緩衝區的初始內容。
func prepCU(m *Machine, v cuVariant) [4][]byte {
	buf := cuBuffers()
	if v.setup != nil {
		v.setup(&buf)
	}
	for k := range buf {
		physWrite(m, cuB+uint32(k)<<16, buf[k])
	}
	physWrite(m, cuF+0x18, []byte{byte(v.ecx), byte(v.ecx >> 8), byte(v.ecx >> 16), byte(v.ecx >> 24)})
	physWrite(m, cuF+0x14, []byte{byte(v.edx), byte(v.edx >> 8), byte(v.edx >> 16), byte(v.edx >> 24)})
	return buf
}

func TestSeg443JemmCU(t *testing.T) {
	for _, v := range cuVariants {
		m := cuMachine(t, 0)
		buf := prepCU(m, v)
		want := cuExpect(v, buf)
		runUntil(t, m, 0x18, 0x5B87, 100)
		spBefore := m.CPU.R[cpu.SP]
		runUntil(t, m, 0x18, 0x5B90, 10)
		mustStep(t, m, v.name+"：jmp 98:15")
		if m.CPU.Seg[cpu.CS] != 0x98 || !m.CPU.Code32() || m.CPU.IP != 0x15 {
			t.Fatalf("%s：jmp 98:15 之後 CS:IP=%04X:%04X Code32=%v", v.name, m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.Code32())
		}
		mustStep(t, m, v.name+"：65 67 mov esi,gs:[12C]")
		if got := reg32(m.CPU, cpu.SI); got != cuB {
			t.Errorf("%s：98:0015 應讀 G+12Ch（%08X），ESI=%08X", v.name, uint32(cuB), got)
		}
		runUntil(t, m, 0x98, 0x8C, 2_000_000)
		hi := m.CPU.Hi[cpu.AX]
		mustStep(t, m, v.name+"：66 33 C0")
		if m.CPU.R[cpu.AX] != 0 || m.CPU.Hi[cpu.AX] != hi {
			t.Errorf("%s：xor ax,ax 之後 AX=%04X、EAX 高半 %04X（應保留 %04X）", v.name, m.CPU.R[cpu.AX], m.CPU.Hi[cpu.AX], hi)
		}
		mustStep(t, m, v.name+"：retf")
		if m.CPU.Seg[cpu.CS] != 0x18 || m.CPU.IP != 0x5B95 || m.CPU.Code32() {
			t.Errorf("%s：retf 之後 CS:IP=%04X:%04X Code32=%v，應為 0018:5B95、false", v.name, m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.Code32())
		}
		if m.CPU.R[cpu.SP] != spBefore {
			t.Errorf("%s：返回後 SP=%04X，應為四個 push 之前的 %04X", v.name, m.CPU.R[cpu.SP], spBefore)
		}
		runUntil(t, m, 0x18, 0x5B9F, 10)
		if got := reg32(m.CPU, cpu.CX); got != cuESP0 {
			t.Errorf("%s：5B9D pop ecx 之後 ECX=%08X，應為舊 ESP %08X", v.name, got, uint32(cuESP0))
		}
		ch := int(v.ecx >> 8 & 3)
		if got := physRead(m, cuB+uint32(ch)<<16, 0x10000); !bytes.Equal(got, want) {
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("%s：目的緩衝區第一個差異在 %04X：%02X，應為 %02X", v.name, i, got[i], want[i])
					break
				}
			}
		}
		for k := range buf {
			if k == ch {
				continue
			}
			if got := physRead(m, cuB+uint32(k)<<16, 0x10000); !bytes.Equal(got, buf[k]) {
				t.Errorf("%s：緩衝區 %d 不得被改寫", v.name, k)
			}
		}
	}
}

// 110h 的 B=1：5B6C push es 失敗即關閉（B=1 移出範圍的確認）。
func TestSeg443JemmCUStackB1(t *testing.T) {
	m := cuMachine(t, 0x4)
	prepCU(m, cuVariants[0])
	runUntil(t, m, 0x18, 0x5B6C, 10)
	wantStepErr(t, m, "110h B=1 的 push es", "B=1")
}

// 5E60–5E6A 與 98:0000–0014 的複製常式：ECX 為 0、1、3、4、5、10001h。
func TestSeg443JemmCopy(t *testing.T) {
	var buf [2][]byte // 來源、目的的初始內容（偽亂數，各 0x10010 bytes）
	x := uint32(777)
	for k := range buf {
		buf[k] = make([]byte, 0x10010)
		for i := range buf[k] {
			x = x*1103515245 + 12345
			buf[k][i] = byte(x >> 16)
		}
	}
	for _, n := range []uint32{0, 1, 3, 4, 5, 0x10001} {
		m := cuMachine(t, 0)
		physWrite(m, cuB, buf[0])
		physWrite(m, cuB+0x20000, buf[1])
		src, dst := uint32(cuB+1), uint32(cuB+0x20003)
		setSel(m, cpu.SS, 0x110)
		m.CPU.R[cpu.SP], m.CPU.Hi[cpu.SP] = 0x0FF0, 0
		m.CPU.IP = 0x5E60
		setReg32(m, cpu.CX, n)
		setReg32(m, cpu.SI, src)
		setReg32(m, cpu.DI, dst)
		runUntil(t, m, 0x98, 0x0000, 10)
		runUntil(t, m, 0x18, 0x5E6F, 100)
		want := append([]byte(nil), buf[1][3:3+n+8]...)
		copy(want, buf[0][1:1+n])
		if got := physRead(m, dst, int(n)+8); !bytes.Equal(got, want) {
			t.Errorf("ECX=%X：目的內容不對（含其後 8 bytes 不變）", n)
		}
		if reg32(m.CPU, cpu.SI) != src+n || reg32(m.CPU, cpu.DI) != dst+n || reg32(m.CPU, cpu.CX) != 0 {
			t.Errorf("ECX=%X：ESI=%08X EDI=%08X ECX=%08X，應為 %08X %08X 0", n, reg32(m.CPU, cpu.SI), reg32(m.CPU, cpu.DI), reg32(m.CPU, cpu.CX), src+n, dst+n)
		}
		if m.CPU.R[cpu.SP] != 0x0FF0 || m.CPU.Code32() {
			t.Errorf("ECX=%X：返回後 SP=%04X Code32=%v", n, m.CPU.R[cpu.SP], m.CPU.Code32())
		}
	}
}

// ---- §3 第 4 項：前綴與解碼 --------------------------------------------------------

func TestSeg443Prefixes(t *testing.T) {
	dsBig := func(m *Machine) { m.CPU.SetSegCache(cpu.DS, dataCache(segData, 0xFFFFF, 0x93)) }
	put := func(m *Machine, a uint32, v uint32) {
		physWrite(m, a, []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
	}

	// 無前綴：mov eax,[esi]（ModRM32、ESI 32 位元、運算元 32）。
	m := d1Prog(t, []byte{0x8B, 0x06})
	dsBig(m)
	setReg32(m, cpu.SI, 0x00010004)
	put(m, segData+0x10004, 0x12345678)
	mustStep(t, m, "8B 06")
	if reg32(m.CPU, cpu.AX) != 0x12345678 || m.CPU.IP != 2 {
		t.Errorf("D=1 無前綴 mov eax,[esi]：EAX=%08X IP=%04X", reg32(m.CPU, cpu.AX), m.CPU.IP)
	}
	// 66：mov ax,[esi]；66 66 與單一 66 相同。
	for _, code := range [][]byte{{0x66, 0x8B, 0x06}, {0x66, 0x66, 0x8B, 0x06}} {
		m = d1Prog(t, code)
		dsBig(m)
		setReg32(m, cpu.SI, 0x00010004)
		setReg32(m, cpu.AX, 0xAAAA0000)
		put(m, segData+0x10004, 0x12345678)
		mustStep(t, m, "66 8B 06")
		if reg32(m.CPU, cpu.AX) != 0xAAAA5678 || m.CPU.IP != uint16(len(code)) {
			t.Errorf("D=1 % X：EAX=%08X IP=%04X，應為 AAAA5678（16 位元運算元、32 位元定址）", code, reg32(m.CPU, cpu.AX), m.CPU.IP)
		}
	}
	// 67：mov eax,[1234h]（16 位元定址 disp16）；66 67：mov ax,[1234h]。
	for _, c := range []struct {
		code []byte
		want uint32
	}{
		{[]byte{0x67, 0x8B, 0x06, 0x34, 0x12}, 0x89ABCDEF},
		{[]byte{0x66, 0x67, 0x8B, 0x06, 0x34, 0x12}, 0xAAAACDEF},
	} {
		m = d1Prog(t, c.code)
		dsBig(m)
		setReg32(m, cpu.SI, 0x00010004)
		setReg32(m, cpu.AX, 0xAAAA0000)
		put(m, segData+0x1234, 0x89ABCDEF)
		mustStep(t, m, "67")
		if reg32(m.CPU, cpu.AX) != c.want || m.CPU.IP != uint16(len(c.code)) {
			t.Errorf("D=1 % X：EAX=%08X IP=%04X，應為 %08X", c.code, reg32(m.CPU, cpu.AX), m.CPU.IP, c.want)
		}
	}
	// 計數迴圈：67 E3 判 CX；E3 判 ECX；E2 以 ECX 遞減；66 E3 判 ECX、目標 16 位元環繞。
	for _, c := range []struct {
		name   string
		code   []byte
		ecx    uint32
		wantIP uint16
		wantCX uint32
	}{
		{"67 E3（CX=0 跳）", []byte{0x67, 0xE3, 0x02}, 0x10000, 5, 0x10000},
		{"E3（ECX=10000h 不跳）", []byte{0xE3, 0x02}, 0x10000, 2, 0x10000},
		{"E3（ECX=0 跳）", []byte{0xE3, 0x02}, 0, 4, 0},
		{"E2（ECX 遞減到 FFFFh，跳）", []byte{0xE2, 0x02}, 0x10000, 4, 0xFFFF},
		{"E2（ECX=1 遞減到 0，不跳）", []byte{0xE2, 0x02}, 1, 2, 0},
		{"67 E2（CX 遞減，高半保留）", []byte{0x67, 0xE2, 0x02}, 0x10000, 5, 0x1FFFF},
		{"67 E2（CX=1 到 0，不跳，高半保留）", []byte{0x67, 0xE2, 0x02}, 0x30001, 3, 0x30000},
		{"66 E3（ECX=10000h 不跳）", []byte{0x66, 0xE3, 0xF0}, 0x10000, 3, 0x10000},
		{"66 E3（ECX=0 跳，16 位元環繞）", []byte{0x66, 0xE3, 0xF0}, 0, 0xFFF3, 0},
	} {
		m = d1Prog(t, c.code)
		setReg32(m, cpu.CX, c.ecx)
		mustStep(t, m, c.name)
		if m.CPU.IP != c.wantIP || reg32(m.CPU, cpu.CX) != c.wantCX {
			t.Errorf("%s：IP=%04X ECX=%08X，應為 %04X %08X", c.name, m.CPU.IP, reg32(m.CPU, cpu.CX), c.wantIP, c.wantCX)
		}
	}
	// 67 F3 A5：SI／DI／CX 搬 dword（高半不動）。
	m = d1Prog(t, []byte{0x67, 0xF3, 0xA5})
	m.CPU.SetSegCache(cpu.ES, dataCache(0x50000, 0xFFFF, 0x93))
	m.CPU.R[cpu.SI], m.CPU.Hi[cpu.SI] = 0x10, 1
	m.CPU.R[cpu.DI], m.CPU.Hi[cpu.DI] = 0x20, 2
	m.CPU.R[cpu.CX], m.CPU.Hi[cpu.CX] = 2, 3
	put(m, 0x1000+0x10, 0x11223344)
	put(m, 0x1000+0x14, 0x55667788)
	mustStep(t, m, "67 F3 A5")
	if m.CPU.R[cpu.SI] != 0x18 || m.CPU.Hi[cpu.SI] != 1 || m.CPU.R[cpu.DI] != 0x28 || m.CPU.Hi[cpu.DI] != 2 ||
		m.CPU.R[cpu.CX] != 0 || m.CPU.Hi[cpu.CX] != 3 {
		t.Errorf("67 F3 A5：ESI=%08X EDI=%08X ECX=%08X", reg32(m.CPU, cpu.SI), reg32(m.CPU, cpu.DI), reg32(m.CPU, cpu.CX))
	}
	if got := physRead(m, 0x50020, 8); !bytes.Equal(got, []byte{0x44, 0x33, 0x22, 0x11, 0x88, 0x77, 0x66, 0x55}) {
		t.Errorf("67 F3 A5：ES:20 = % X", got)
	}
	// 0F 84 取 rel32。
	m = d1Prog(t, []byte{0x0F, 0x84, 0x10, 0x00, 0x00, 0x00})
	m.CPU.SetFlags(cpu.ZF)
	mustStep(t, m, "0F 84 rel32")
	if m.CPU.IP != 0x16 {
		t.Errorf("0F 84 rel32：IP=%04X，應為 0016", m.CPU.IP)
	}
	// D=0 碼段與實模式：66 74、67 A4 仍失敗即關閉。
	for _, code := range [][]byte{{0x66, 0x74, 0x00}, {0x67, 0xA4}} {
		wantStepErr(t, d0Prog(t, code), "D=0 分段模式 % X", "")
		wantStepErr(t, newSysProg(t, code), "實模式 % X", "")
	}
	// 指令中途遠跳到 D=1 不改變本道的預設：16 位元 EA 讀 off16（5 bytes），下一道才是 32 位元。
	m = d0Prog(t, []byte{0xEA, 0x00, 0x01, 0x28, 0x00})
	copy(m.Mem[segCode+0x100:], []byte{0xB8, 0x78, 0x56, 0x34, 0x12})
	mustStep(t, m, "D=0 EA 到 D=1")
	if m.CPU.Seg[cpu.CS] != 0x28 || m.CPU.IP != 0x100 || !m.CPU.Code32() {
		t.Fatalf("EA 到 D=1：CS:IP=%04X:%04X Code32=%v", m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.Code32())
	}
	mustStep(t, m, "D=1 mov eax,imm32")
	if reg32(m.CPU, cpu.AX) != 0x12345678 || m.CPU.IP != 0x105 {
		t.Errorf("下一道以 32 位元解碼：EAX=%08X IP=%04X", reg32(m.CPU, cpu.AX), m.CPU.IP)
	}
	// 反向：D=1 的 EA 讀 off32（7 bytes），下一道在 D=0 以 16 位元解碼。
	m = d1Prog(t, []byte{0xEA, 0x10, 0x00, 0x00, 0x00, 0x30, 0x00})
	copy(m.Mem[0x1000+0x10:], []byte{0xB8, 0x34, 0x12})
	mustStep(t, m, "D=1 EA 到 D=0")
	if m.CPU.Seg[cpu.CS] != 0x30 || m.CPU.IP != 0x10 || m.CPU.Code32() {
		t.Fatalf("EA 到 D=0：CS:IP=%04X:%04X Code32=%v", m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.Code32())
	}
	mustStep(t, m, "D=0 mov ax,imm16")
	if m.CPU.R[cpu.AX] != 0x1234 || m.CPU.IP != 0x13 {
		t.Errorf("下一道以 16 位元解碼：AX=%04X IP=%04X", m.CPU.R[cpu.AX], m.CPU.IP)
	}
}

// ---- §3 第 6 項：EIP 不變式 ----------------------------------------------------------

func TestSeg443EIPInvariant(t *testing.T) {
	for _, c := range []struct {
		name  string
		ip    uint16
		code  []byte
		setup func(*Machine)
		bad   uint16 // 越界目標的低 16 位元（IP 不得等於它）
	}{
		{"EB 正向越界", 0xFFF0, []byte{0xEB, 0x7F}, nil, 0x0071},
		{"EB 負向環繞", 0x0000, []byte{0xEB, 0x80}, nil, 0xFF82},
		{"74 正向越界", 0xFFF0, []byte{0x74, 0x7F}, func(m *Machine) { m.CPU.SetFlags(cpu.ZF) }, 0x0071},
		{"74 負向環繞", 0x0000, []byte{0x74, 0x80}, func(m *Machine) { m.CPU.SetFlags(cpu.ZF) }, 0xFF82},
		{"E2 正向越界", 0xFFF0, []byte{0xE2, 0x7F}, func(m *Machine) { setReg32(m, cpu.CX, 2) }, 0x0071},
		{"E2 負向環繞", 0x0000, []byte{0xE2, 0x80}, func(m *Machine) { setReg32(m, cpu.CX, 2) }, 0xFF82},
		{"67 E3 正向越界", 0xFFF0, []byte{0x67, 0xE3, 0x7F}, func(m *Machine) { setReg32(m, cpu.CX, 0x10000) }, 0x0072},
		{"67 E3 負向環繞", 0x0000, []byte{0x67, 0xE3, 0x80}, func(m *Machine) { setReg32(m, cpu.CX, 0x10000) }, 0xFF83},
		{"67 E2 正向越界", 0xFFF0, []byte{0x67, 0xE2, 0x7F}, func(m *Machine) { setReg32(m, cpu.CX, 2) }, 0x0072},
		{"67 E2 負向環繞", 0x0000, []byte{0x67, 0xE2, 0x80}, func(m *Machine) { setReg32(m, cpu.CX, 2) }, 0xFF83},
		{"0F 84 正向越界", 0xFFE0, []byte{0x0F, 0x84, 0x00, 0x01, 0x00, 0x00}, func(m *Machine) { m.CPU.SetFlags(cpu.ZF) }, 0x00E6},
		{"0F 84 負向環繞", 0x0000, []byte{0x0F, 0x84, 0x00, 0xFF, 0xFF, 0xFF}, func(m *Machine) { m.CPU.SetFlags(cpu.ZF) }, 0xFF06},
	} {
		m := d1Prog(t, nil)
		copy(m.Mem[segCode+uint32(c.ip):], c.code)
		m.CPU.IP = c.ip
		if c.setup != nil {
			c.setup(m)
		}
		err := wantStepErr(t, m, c.name, "EIP 超過 FFFFh")
		if err != nil && (m.CPU.IP == c.bad) {
			t.Errorf("%s：IP 不得被設為越界目標（%04X）", c.name, m.CPU.IP)
		}
		if e, ok := err.(*cpu.Error); ok && (e.CS != sel437D1 || e.IP != c.ip) {
			t.Errorf("%s：錯誤應回報本道起點 %04X:%04X，實際 %04X:%04X", c.name, sel437D1, c.ip, e.CS, e.IP)
		}
	}
	// D=1 在 IP=FFFFh 取指令失敗（單 byte 的 nop，以及第二個 byte 落在 FFFFh 的 jmp）。
	for _, c := range []struct {
		ip   uint16
		code []byte
	}{{0xFFFF, []byte{0x90}}, {0xFFFE, []byte{0xEB, 0x00}}} {
		m := d1Prog(t, nil)
		copy(m.Mem[segCode+uint32(c.ip):], c.code)
		m.CPU.IP = c.ip
		wantStepErr(t, m, "D=1 在 FFFFh 取指令", "EIP 超過 FFFFh")
	}
	// D=0 分段模式在 IP=FFFFh 取指令照常環繞。
	m := d0Prog(t, nil)
	m.Mem[segCode+0xFFFF] = 0x90
	m.CPU.IP = 0xFFFF
	mustStep(t, m, "D=0 在 FFFFh 的 nop")
	if m.CPU.IP != 0 {
		t.Errorf("D=0：IP=%04X，應環繞為 0", m.CPU.IP)
	}
	// 66 EB：16 位元環繞照舊。
	m = d1Prog(t, nil)
	copy(m.Mem[segCode+0xFFF0:], []byte{0x66, 0xEB, 0x7F})
	m.CPU.IP = 0xFFF0
	mustStep(t, m, "66 EB")
	if m.CPU.IP != 0x0072 {
		t.Errorf("66 EB：IP=%04X，應為 0072", m.CPU.IP)
	}
}

// ---- §3 第 7 項：運算元大小 16 的形式 ----------------------------------------------------

func TestSeg443Operand16Forms(t *testing.T) {
	// 66 E8：推 16 位元返回位址，目標 16 位元環繞。
	m := d1Prog(t, nil)
	copy(m.Mem[segCode+0xFFF0:], []byte{0x66, 0xE8, 0x20, 0x00})
	m.CPU.IP = 0xFFF0
	sp := m.CPU.R[cpu.SP]
	mustStep(t, m, "66 E8")
	if m.CPU.IP != 0x0014 || m.CPU.R[cpu.SP] != sp-2 || le16(m, 0x1000+uint32(sp-2)) != 0xFFF4 {
		t.Errorf("66 E8：IP=%04X SP=%04X [SP]=%04X", m.CPU.IP, m.CPU.R[cpu.SP], le16(m, 0x1000+uint32(m.CPU.R[cpu.SP])))
	}
	// 66 C3：彈出 16 位元 IP。
	m = d1Prog(t, []byte{0x66, 0xC3})
	m.CPU.R[cpu.SP] = 0x0200
	copy(m.Mem[0x1000+0x200:], []byte{0x34, 0x12, 0x78, 0x56})
	mustStep(t, m, "66 C3")
	if m.CPU.IP != 0x1234 || m.CPU.R[cpu.SP] != 0x202 {
		t.Errorf("66 C3：IP=%04X SP=%04X，應為 1234、0202", m.CPU.IP, m.CPU.R[cpu.SP])
	}
	// 66 8F /0 以 ESP 為基底的 ModRM32 失敗即關閉；以 EAX 為基底照常。
	m = d1Prog(t, []byte{0x66, 0x8F, 0x04, 0x24})
	wantStepErr(t, m, "66 8F [esp]", "443 §2.4")
	m = d1Prog(t, []byte{0x66, 0x8F, 0x00})
	m.CPU.R[cpu.SP] = 0x0200
	copy(m.Mem[0x1000+0x200:], []byte{0xCD, 0xAB})
	setReg32(m, cpu.AX, 0x300)
	mustStep(t, m, "66 8F [eax]")
	if le16(m, 0x1300) != 0xABCD || m.CPU.R[cpu.SP] != 0x202 {
		t.Errorf("66 8F [eax]：[300]=%04X SP=%04X", le16(m, 0x1300), m.CPU.R[cpu.SP])
	}
}

// ---- §3 第 10 項：進出 D=1 ----------------------------------------------------------------

func TestSeg443EnterLeave(t *testing.T) {
	const back = sel437Back // 基底 1000h 的 D=0 碼段
	type step struct {
		name   string
		code32 bool
		cs, ip uint16
	}
	for _, c := range []struct {
		name  string
		d1    bool   // 起點為 D=1（CS=28h、程式在 segCode）；否則 D=0（CS=30h、程式在 1000h）
		code  []byte // 起點的程式
		at    []byte // 目標處（D=1：segCode+0x40；D=0：1000h+0x40）的程式
		setup func(*Machine)
		steps []step
	}{
		{name: "EA 到 D=1", code: []byte{0xEA, 0x40, 0x00, 0x28, 0x00},
			steps: []step{{"EA", true, 0x28, 0x40}}},
		{name: "66 EA 到 D=1", code: append([]byte{0x66, 0xEA}, farPtr32(0x40, 0x28)...),
			steps: []step{{"66 EA", true, 0x28, 0x40}}},
		{name: "FF /5 m16:16 到 D=1", code: []byte{0xFF, 0x2E, 0x00, 0x02},
			setup: func(m *Machine) { copy(m.Mem[0x1200:], []byte{0x40, 0x00, 0x28, 0x00}) },
			steps: []step{{"FF /5", true, 0x28, 0x40}}},
		{name: "66 FF /5 m16:32 到 D=1", code: []byte{0x66, 0xFF, 0x2E, 0x00, 0x02},
			setup: func(m *Machine) { copy(m.Mem[0x1200:], farPtr32(0x40, 0x28)) },
			steps: []step{{"66 FF /5", true, 0x28, 0x40}}},
		{name: "66 9A 到 D=1，CB（32 位元框架）回 D=0", code: append([]byte{0x66, 0x9A}, farPtr32(0x40, 0x28)...), at: []byte{0xCB},
			steps: []step{{"66 9A", true, 0x28, 0x40}, {"D=1 CB", false, back, 0x108}}},
		{name: "9A 到 D=1，66 CB（16 位元框架）回 D=0", code: []byte{0x9A, 0x40, 0x00, 0x28, 0x00}, at: []byte{0x66, 0xCB},
			steps: []step{{"9A", true, 0x28, 0x40}, {"D=1 66 CB", false, back, 0x105}}},
		{name: "FF /3 m16:16 到 D=1，66 CB 回", code: []byte{0xFF, 0x1E, 0x00, 0x02}, at: []byte{0x66, 0xCB},
			setup: func(m *Machine) { copy(m.Mem[0x1200:], []byte{0x40, 0x00, 0x28, 0x00}) },
			steps: []step{{"FF /3", true, 0x28, 0x40}, {"66 CB", false, back, 0x104}}},
		{name: "66 FF /3 m16:32 到 D=1，CB 回", code: []byte{0x66, 0xFF, 0x1E, 0x00, 0x02}, at: []byte{0xCB},
			setup: func(m *Machine) { copy(m.Mem[0x1200:], farPtr32(0x40, 0x28)) },
			steps: []step{{"66 FF /3", true, 0x28, 0x40}, {"CB", false, back, 0x105}}},
		{name: "D=0 的 CB 返回 D=1", code: []byte{0xCB},
			setup: func(m *Machine) { m.CPU.R[cpu.SP] = 0x300; copy(m.Mem[0x1300:], []byte{0x40, 0x00, 0x28, 0x00}) },
			steps: []step{{"D=0 CB", true, 0x28, 0x40}}},
		{name: "D=1 的 EA（16:32）到 D=0", d1: true, code: append([]byte{0xEA}, farPtr32(0x40, back)...),
			steps: []step{{"D=1 EA", false, back, 0x40}}},
		{name: "D=1 的 66 EA（16:16）到 D=0", d1: true, code: []byte{0x66, 0xEA, 0x40, 0x00, byte(back), 0x00},
			steps: []step{{"D=1 66 EA", false, back, 0x40}}},
		{name: "D=1 的 9A 到 D=0，66 CB（16:32 框架）回 D=1", d1: true, code: append([]byte{0x9A}, farPtr32(0x40, back)...),
			at: []byte{0x66, 0xCB}, steps: []step{{"D=1 9A", false, back, 0x40}, {"D=0 66 CB", true, 0x28, 0x07}}},
		{name: "D=1 的 66 9A 到 D=0，CB 回 D=1", d1: true, code: []byte{0x66, 0x9A, 0x40, 0x00, byte(back), 0x00},
			at: []byte{0xCB}, steps: []step{{"D=1 66 9A", false, back, 0x40}, {"D=0 CB", true, 0x28, 0x06}}},
	} {
		var m *Machine
		if c.d1 {
			m = d1Prog(t, c.code)
			if c.at != nil {
				copy(m.Mem[0x1000+0x40:], c.at)
			}
		} else {
			m = seg437(t, c.code)
			m.CPU.Seg[cpu.CS] = back // 快取基底 1000h，與 30h 相同
			if c.at != nil {
				copy(m.Mem[segCode+0x40:], c.at)
			}
		}
		if c.setup != nil {
			c.setup(m)
		}
		sp := m.CPU.R[cpu.SP]
		for _, s := range c.steps {
			mustStep(t, m, c.name+"："+s.name)
			if m.CPU.Code32() != s.code32 || m.CPU.Seg[cpu.CS] != s.cs || m.CPU.IP != s.ip {
				t.Errorf("%s：%s 之後 CS:IP=%04X:%04X Code32=%v，應為 %04X:%04X %v", c.name, s.name,
					m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.Code32(), s.cs, s.ip, s.code32)
			}
		}
		if len(c.steps) == 2 && m.CPU.R[cpu.SP] != sp {
			t.Errorf("%s：呼叫與返回後 SP=%04X，應回到 %04X", c.name, m.CPU.R[cpu.SP], sp)
		}
	}
	// loadSeg(CS) 失敗：CS、快取與下一道的預設大小不變。
	for _, c := range []struct {
		name string
		d1   bool
		code []byte
	}{
		{"D=1 的 EA 到資料段", true, append([]byte{0xEA}, farPtr32(0, 0x08)...)},
		{"D=1 的 66 EA 到資料段", true, []byte{0x66, 0xEA, 0, 0, 0x08, 0}},
		{"D=0 的 EA 到資料段", false, []byte{0xEA, 0, 0, 0x08, 0}},
	} {
		var m *Machine
		if c.d1 {
			m = d1Prog(t, c.code)
		} else {
			m = d0Prog(t, c.code)
		}
		cs, cache := m.CPU.Seg[cpu.CS], m.CPU.SegCache(cpu.CS)
		wantStepErr(t, m, c.name, "分段模式載入")
		if m.CPU.Seg[cpu.CS] != cs || m.CPU.SegCache(cpu.CS) != cache || m.CPU.Code32() != c.d1 {
			t.Errorf("%s：失敗後 CS=%04X 快取 %+v Code32=%v 不得改變", c.name, m.CPU.Seg[cpu.CS], m.CPU.SegCache(cpu.CS), m.CPU.Code32())
		}
	}
}

// ---- §3 第 11 項：中斷 -------------------------------------------------------------------

func TestSeg443InterruptsDeferred(t *testing.T) {
	// D=1：sti；EA 到 D=0 的 cli；jmp $。先在 D=1 跑幾步（IF=1、待處理），再回 D=0 cli。
	code := []byte{0xFB, 0xEB, 0xFE}
	m := d1Prog(t, code)
	m.IRQ0Every = 5
	m.nextIRQ0 = m.Steps + 5
	m.PushKey(0x1E)
	ticks, keys := m.Ticks, m.KeyQueueLen()
	for i := 0; i < 40; i++ {
		mustStep(t, m, "D=1 sti；jmp $")
	}
	if !m.CPU.Code32() || !m.irq0Pending || m.Ticks != ticks || m.KeyQueueLen() != keys {
		t.Errorf("D=1：IRQ0 pending=%v Ticks %d→%d 鍵盤 %d→%d（應延後、不消耗）", m.irq0Pending, ticks, m.Ticks, keys, m.KeyQueueLen())
	}
	if m.segWaitN == 0 {
		t.Error("D=1、IF=1、待處理：segWait 應照常計數")
	}
	// 回到 D=0 並 cli：仍保留。
	copy(m.Mem[0x1000+0x40:], []byte{0xFA, 0xEB, 0xFE})
	copy(m.Mem[segCode+0x10:], append([]byte{0xEA}, farPtr32(0x40, sel437Back)...))
	m.CPU.IP = 0x10
	for i := 0; i < 20; i++ {
		mustStep(t, m, "回 D=0、cli")
	}
	if m.CPU.Code32() || m.CPU.Flag(cpu.IF) || !m.irq0Pending || m.Ticks != ticks || m.KeyQueueLen() != keys {
		t.Errorf("回 D=0 cli 後：Code32=%v IF=%v pending=%v Ticks %d 鍵盤 %d", m.CPU.Code32(), m.CPU.Flag(cpu.IF), m.irq0Pending, m.Ticks, m.KeyQueueLen())
	}
	// IF=1 且待處理一百萬步後失敗即關閉。
	m = d1Prog(t, code)
	m.IRQ0Every = 100
	m.nextIRQ0 = m.Steps + 100
	var err error
	done := 0
	for ; done < 1_200_000; done++ {
		if err = m.Step(); err != nil {
			break
		}
	}
	if err == nil || !strings.Contains(err.Error(), "連續") || done < segWaitLimit {
		t.Errorf("D=1、IF=1 且待處理：應在一百萬步後失敗，實際 %v（%d 步）", err, done)
	}
	// D=1 下 INT n、div 0、hlt 失敗即關閉。
	for _, c := range []struct {
		name string
		code []byte
	}{
		{"int 21h", []byte{0xCD, 0x21}},
		{"div ebx（0）", []byte{0xF7, 0xF3}},
		{"hlt", []byte{0xF4}},
	} {
		m := d1Prog(t, c.code)
		hooked := false
		m.CPU.IntHook = func(*cpu.CPU, uint8) bool { hooked = true; return true }
		setReg32(m, cpu.BX, 0)
		wantStepErr(t, m, c.name, "")
		if hooked {
			t.Errorf("%s：不得呼叫 IntHook", c.name)
		}
	}
}

// ---- §3 第 12 項：失敗即關閉 ---------------------------------------------------------------

func TestSeg443FailClosed(t *testing.T) {
	for _, c := range []struct {
		name string
		code []byte
	}{
		{"E8", []byte{0xE8, 0, 0, 0, 0}},
		{"E9", []byte{0xE9, 0, 0, 0, 0}},
		{"C2", []byte{0xC2, 0, 0}},
		{"C3", []byte{0xC3}},
		{"FF /2", []byte{0xFF, 0xD0}},
		{"FF /4", []byte{0xFF, 0xE0}},
		{"C8", []byte{0xC8, 0, 0, 0}},
		{"C9", []byte{0xC9}},
		{"E5", []byte{0xE5, 0x60}},
		{"E7", []byte{0xE7, 0x60}},
		{"ED", []byte{0xED}},
		{"EF", []byte{0xEF}},
		{"6D", []byte{0x6D}},
		{"6F", []byte{0x6F}},
		{"C4", []byte{0xC4, 0x06}},
		{"C5", []byte{0xC5, 0x06}},
		{"62", []byte{0x62, 0x06}},
		{"6C", []byte{0x6C}},
		{"6E", []byte{0x6E}},
		{"D7", []byte{0xD7}},
		{"66 CF（16 位元 iret）", []byte{0x66, 0xCF}},
	} {
		m := d1Prog(t, c.code)
		sp := m.CPU.R[cpu.SP]
		wantStepErr(t, m, "D=1 "+c.name, "")
		if m.CPU.R[cpu.SP] != sp {
			t.Errorf("D=1 %s：SP 不得改變", c.name)
		}
	}
	// IDT 閘目標為 D=1（435 不變）見 v86deliver_test.go 的「碼段 D=1」列。
	// 分段模式清 PE 時 cache[CS] 的 D=1：SetCR0 回錯誤、CR0 不改；經 mov cr0 的指令也失敗。
	m := d1Prog(t, []byte{0x0F, 0x22, 0xC0}) // mov cr0,eax
	cr0 := m.CR0()
	if err := m.SetCR0(0); err == nil || !strings.Contains(err.Error(), "443") || m.CR0() != cr0 {
		t.Errorf("SetCR0 清 PE（CS D=1）：應回錯誤且 CR0 不改，實際 %v、CR0 %08X→%08X", err, cr0, m.CR0())
	}
	setReg32(m, cpu.AX, 0)
	wantStepErr(t, m, "D=1 mov cr0 清 PE", "443")
	if m.CR0() != cr0 || !m.CPU.Segmented() || !m.CPU.Code32() {
		t.Errorf("mov cr0 失敗後 CR0=%08X segmented=%v Code32=%v", m.CR0(), m.CPU.Segmented(), m.CPU.Code32())
	}
	// 對照：CS D=0 時清 PE 照 442 進入子狀態。
	m = d0Prog(t, nil)
	mustNil(t, "SetCR0 清 PE（CS D=0）", m.SetCR0(0))
}

// ---- §3 第 13 項：狀態檔 -------------------------------------------------------------------

func TestSeg443StateRoundTrip(t *testing.T) {
	v := cuVariants[2] // 差分
	finish := func(m *Machine) []byte {
		t.Helper()
		runUntil(t, m, 0x18, 0x5B9D, 2_000_000)
		for i := 0; i < 100; i++ {
			mustStep(t, m, "5B9D 之後")
		}
		return saveBytes(t, m)
	}
	ref := cuMachine(t, 0)
	prepCU(ref, v)
	want := finish(ref)

	m := cuMachine(t, 0)
	prepCU(m, v)
	runUntil(t, m, 0x98, 0x5B, 2_000_000) // repz cmpsd 之前
	if !m.CPU.Code32() {
		t.Fatal("前提：存檔點在 D=1 碼段")
	}
	pm := m.encodePM()
	if len(pm) != 142 || pm[0] != 3 || pm[1] != 0 {
		t.Errorf("encodePM：版本 %d、%d bytes，應為版本 3、142 bytes", uint16(pm[0])|uint16(pm[1])<<8, len(pm))
	}
	data := saveBytes(t, m)
	snap := m.Snapshot()

	dst := newPM(t)
	mustNil(t, "LoadState", dst.LoadState(bytes.NewReader(data)))
	dst.IRQ0Every = 0
	if !dst.CPU.Code32() || dst.CPU.Seg[cpu.CS] != 0x98 || dst.CPU.IP != 0x5B {
		t.Fatalf("LoadState 後 CS:IP=%04X:%04X Code32=%v", dst.CPU.Seg[cpu.CS], dst.CPU.IP, dst.CPU.Code32())
	}
	if got := finish(dst); !bytes.Equal(got, want) {
		t.Error("SaveState／LoadState 往返後的最終存檔與不中斷的執行不同")
	}

	for i := 0; i < 1000; i++ { // 先把來源機器跑離存檔點，再 Restore
		mustStep(t, m, "跑離存檔點")
	}
	m.Restore(snap)
	if !m.CPU.Code32() || m.CPU.IP != 0x5B {
		t.Fatalf("Restore 後 IP=%04X Code32=%v", m.CPU.IP, m.CPU.Code32())
	}
	if got := finish(m); !bytes.Equal(got, want) {
		t.Error("Snapshot／Restore 往返後的最終存檔與不中斷的執行不同")
	}
}
