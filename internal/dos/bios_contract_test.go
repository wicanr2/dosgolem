package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// BIOS 服務補洞的契約測試（`docs/knowledge-base/020-bios-services-coverage.md`）。

// `int 10h AH=0Ch`／`0Dh` 在 mode 13h（線性）與平面模式下都要對。
//
// 拿錯版面的話寫進去的位元組會落在別的地方，而畫面看起來只是
// 「多了幾個雜點」——不像「BIOS 畫點壞了」。
func TestBIOSPixelWorksInBothVideoLayouts(t *testing.T) {
	t.Run("mode13h", func(t *testing.T) {
		m, d := newTest(t)
		m.SetVideoMode(0x13)
		m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 10, 20
		call(m, d, 0x10, 0x0C2A)
		if got := m.Read8(uint32(machine.VideoSeg)*16 + 20*320 + 10); got != 0x2A {
			t.Fatalf("線性緩衝區裡是 %02X，預期 2A", got)
		}
		m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 10, 20
		call(m, d, 0x10, 0x0D00)
		if got := uint8(m.CPU.R[cpu.AX]); got != 0x2A {
			t.Errorf("讀回 %02X，預期 2A", got)
		}
	})

	t.Run("平面模式", func(t *testing.T) {
		m, d := newTest(t)
		m.SetVideoMode(0x10) // 640×350 平面
		m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 9, 2
		call(m, d, 0x10, 0x0C09) // 色號 9 ＝ 平面 0 與 3
		m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 9, 2
		call(m, d, 0x10, 0x0D00)
		if got := uint8(m.CPU.R[cpu.AX]); got != 9 {
			t.Fatalf("平面模式讀回 %d，預期 9", got)
		}
		// 隔壁那一點不能被畫到（位元遮罩要對）。
		m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 8, 2
		call(m, d, 0x10, 0x0D00)
		if got := uint8(m.CPU.R[cpu.AX]); got != 0 {
			t.Errorf("隔壁那一點是 %d，預期 0", got)
		}
	})
}

// 畫面外的座標要被擋掉，不能寫到別的記憶體去。
func TestBIOSPixelIgnoresOutOfRange(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x13)
	before := m.Read8(uint32(machine.VideoSeg)*16 + 100)
	m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 9999, 9999
	call(m, d, 0x10, 0x0C5A)
	if got := m.Read8(uint32(machine.VideoSeg)*16 + 100); got != before {
		t.Error("畫面外的座標寫進了視訊記憶體")
	}
}

// `int 10h AH=13h` 的字串要收進 Console——那是「程式對我們說了什麼」的管道。
func TestBIOSWriteStringReachesConsole(t *testing.T) {
	m, d := newTest(t)
	m.WriteBytes(cpu.Addr(0x2000, 0), []byte("HI"))
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BP] = 0x2000, 0
	m.CPU.R[cpu.CX] = 2
	call(m, d, 0x10, 0x1300)
	if got := string(d.Console); got != "HI" {
		t.Fatalf("Console 是 %q，預期 HI", got)
	}

	// 帶屬性的版本（AL bit1）：字元與屬性交錯，只收字元。
	m2, d2 := newTest(t)
	m2.WriteBytes(cpu.Addr(0x2000, 0), []byte{'A', 0x07, 'B', 0x07})
	m2.CPU.Seg[cpu.ES], m2.CPU.R[cpu.BP] = 0x2000, 0
	m2.CPU.R[cpu.CX] = 2
	call(m2, d2, 0x10, 0x1302)
	if got := string(d2.Console); got != "AB" {
		t.Errorf("帶屬性的字串收成 %q，預期 AB", got)
	}
}

// `int 16h AH=05h` 把鍵塞進緩衝區；**滿了要回 AL=1**。
//
// 回 0 的話呼叫端以為塞進去了，而那個鍵消失得無聲無息。
func TestBIOSPushKeyReportsFullBuffer(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.CX] = 0x1C0D // Enter
	call(m, d, 0x16, 0x0500)
	if got := uint8(m.CPU.R[cpu.AX]); got != 0 {
		t.Fatalf("塞第一個鍵回 AL=%d，預期 0（成功）", got)
	}
	call(m, d, 0x16, 0x0100)
	if m.CPU.Flags&cpu.ZF != 0 || m.CPU.R[cpu.AX] != 0x1C0D {
		t.Fatalf("塞進去的鍵讀不到：AX=%04X ZF=%t",
			m.CPU.R[cpu.AX], m.CPU.Flags&cpu.ZF != 0)
	}

	// 塞到滿。
	full := false
	for i := 0; i < 32; i++ {
		m.CPU.R[cpu.CX] = 0x1E41
		call(m, d, 0x16, 0x0500)
		if uint8(m.CPU.R[cpu.AX]) == 1 {
			full = true
			break
		}
	}
	if !full {
		t.Error("緩衝區塞不滿——滿了要回 AL=1")
	}
}

// `int 15h AH=88h` 要回實際的延伸記憶體大小。
//
// 回 0 會被讀成「沒有延伸記憶體」，於是程式連 XMS 都不問。
func TestInt15ExtendedMemorySizeIsNonZero(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x15, 0x8800)
	if m.CPU.R[cpu.AX] == 0 {
		t.Fatal("AH=88h 回 0 KB——程式會判定沒有延伸記憶體")
	}
	if m.CPU.Flags&cpu.CF != 0 {
		t.Error("AH=88h 設了 CF")
	}
}

// `int 15h AH=87h` 的 CX 是**要搬幾個 word**，不是 byte。
//
// 看成 byte 的話只搬一半，而後半段留著上一次的內容——
// 看起來像資料檔只壞了後面那一段。
func TestInt15BlockMoveCountsWords(t *testing.T) {
	m, d := newTest(t)
	const src, dst = 0x30000, 0x40000
	for i := uint32(0); i < 8; i++ {
		m.Write8(src+i, uint8(0xA0+i))
	}
	// GDT：位移 10h 是來源、18h 是目的，各自 +2..+4 放 24 位元基底。
	gdt := cpu.Addr(0x2000, 0)
	m.Write16(gdt+0x10+2, uint16(src&0xFFFF))
	m.Write8(gdt+0x10+4, uint8(src>>16))
	m.Write16(gdt+0x18+2, uint16(dst&0xFFFF))
	m.Write8(gdt+0x18+4, uint8(dst>>16))
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.SI] = 0x2000, 0
	m.CPU.R[cpu.CX] = 4 // 4 個 word ＝ 8 bytes

	call(m, d, 0x15, 0x8700)
	if m.CPU.Flags&cpu.CF != 0 || uint8(m.CPU.R[cpu.AX]>>8) != 0 {
		t.Fatalf("搬移失敗：AH=%02X CF=%t", uint8(m.CPU.R[cpu.AX]>>8),
			m.CPU.Flags&cpu.CF != 0)
	}
	for i := uint32(0); i < 8; i++ {
		if got := m.Read8(dst + i); got != uint8(0xA0+i) {
			t.Fatalf("第 %d 個位元組是 %02X，預期 %02X——CX 被當成 byte 數了",
				i, got, 0xA0+i)
		}
	}
}

// `int 15h AH=C0h` 要指到一段**合法而且看得懂**的系統設定表，
// 而且那一段不能壓到鄰居。
//
// ⚠ **上下都有鄰居。** 下面是中斷向量的 stub，上面是環境區塊——
// `StubSeg`（0x0080）與 `EnvSeg`（0x00D0）只差 0x50 段，也就是
// StubSeg 的位移 0x500 就是 `EnvSeg:0000`。壓到環境區塊的話，
// 程式讀自己的環境會拿到一段亂碼，而且完全不會報錯。
func TestInt15SystemConfigTableIsSafe(t *testing.T) {
	m, d := newTest(t)
	// 兩個鄰居先做記號：stub 區整段，以及環境區塊的開頭。
	stubBefore := make([]byte, 0x440)
	for i := range stubBefore {
		stubBefore[i] = m.Read8(uint32(machine.StubSeg)*16 + uint32(i))
	}
	const envMark = `COMSPEC=C:\COMMAND.COM`
	m.WriteBytes(uint32(machine.EnvSeg)*16, append([]byte(envMark), 0))

	call(m, d, 0x15, 0xC000)

	for i, want := range stubBefore {
		if got := m.Read8(uint32(machine.StubSeg)*16 + uint32(i)); got != want {
			t.Fatalf("設定表壓到 StubSeg 位移 %03X（%02X → %02X）", i, want, got)
		}
	}
	for i := range envMark {
		if got := m.Read8(uint32(machine.EnvSeg)*16 + uint32(i)); got != envMark[i] {
			t.Fatalf("設定表壓到環境區塊第 %d 個位元組（%q → %02X）", i, envMark[i], got)
		}
	}
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("AH=C0h 回 CF——程式會以為自己在 PC/XT 上")
	}
	seg, off := m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX]
	if n := m.Read16(cpu.Addr(seg, off)); n < 8 {
		t.Errorf("表長度是 %d，預期至少 8", n)
	}
	if model := m.Read8(cpu.Addr(seg, off) + 2); model != 0xFC {
		t.Errorf("機型碼是 %02X，預期 FC（AT）", model)
	}
	// 每個向量仍然指到會回來的碼。
	for v := 0; v < 256; v++ {
		if b := m.Read8(cpu.Addr(m.Read16(uint32(v)*4+2), m.Read16(uint32(v)*4))); b == 0 {
			t.Fatalf("向量 %02X 的第一個位元組被寫成 0 了", v)
		}
	}
}

// 切到 mode 10h／12h 時 DAC 0–63 載入 rgbRGB 表（`docs/spec/198-mode-set-default-dac`）。
//
// 反面的症狀：BGI 只設屬性調色盤、顏色靠 BIOS 的預設 DAC；DAC 全 0 的話，
// 平面裡的色號全對、畫面卻整片黑。
func TestModeSetLoadsDefaultDAC(t *testing.T) {
	// DOSBox-X int10_modes.cpp text_palette 的代表值（格, R, G, B）。
	want := [][4]uint8{{0x00, 0, 0, 0}, {0x06, 0x2A, 0x2A, 0}, {0x07, 0x2A, 0x2A, 0x2A},
		{0x14, 0x2A, 0x15, 0}, {0x38, 0x15, 0x15, 0x15}, {0x3F, 0x3F, 0x3F, 0x3F}}
	for _, mode := range []uint16{0x12, 0x10} {
		m, d := newTest(t)
		call(m, d, 0x10, mode)
		for _, w := range want {
			i := int(w[0])
			if got := [3]uint8{m.DAC[i*3], m.DAC[i*3+1], m.DAC[i*3+2]}; got != [3]uint8{w[1], w[2], w[3]} {
				t.Fatalf("mode %02Xh：DAC %02Xh 是 %02X，預期 %02X", mode, i, got, w[1:])
			}
		}
		for i := 0; i < 64; i++ { // 整張照公式
			b := func(bit int, v uint8) uint8 {
				if i&(1<<bit) != 0 {
					return v
				}
				return 0
			}
			exp := [3]uint8{b(2, 0x2A) + b(5, 0x15), b(1, 0x2A) + b(4, 0x15), b(0, 0x2A) + b(3, 0x15)}
			if got := [3]uint8{m.DAC[i*3], m.DAC[i*3+1], m.DAC[i*3+2]}; got != exp {
				t.Fatalf("mode %02Xh：DAC %02Xh 是 %02X，公式是 %02X", mode, i, got, exp)
			}
		}
	}
	// mode 13h 不在範圍：DAC 不被這條規則改寫。
	m, d := newTest(t)
	m.DAC[7*3] = 0x11
	call(m, d, 0x10, 0x0013)
	if m.DAC[7*3] != 0x11 {
		t.Fatalf("mode 13h 改寫了 DAC 7：%02X", m.DAC[7*3])
	}
}
