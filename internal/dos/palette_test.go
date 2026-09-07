package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// int 10h AH=10h 的測試（`docs/spec/011` §1）與向量 stub 的分派
// （§2）。這一支的每個 AL 語意都不同，而且**屬性調色盤與 DAC 是兩層
// 不同的東西**；接錯不會報錯，只會讓顏色安靜地變成別的。

// TestPaletteSetSingleDACRegister：AL=10h 設的是 **DAC**，
// 暫存器是 BX=索引、DH=R、CH=G、CL=B。
//
// 接成「設屬性暫存器」的話：DAC 完全沒動（畫面留在上一個調色盤或全黑），
// 而 AttrPal 被寫進不相干的值——兩邊都錯，卻沒有任何徵兆。
func TestPaletteSetSingleDACRegister(t *testing.T) {
	m, d := newTest(t)
	c := m.CPU
	c.R[cpu.BX] = 5
	c.R[cpu.DX] = 0x2A00 // DH ＝ 紅 42
	c.R[cpu.CX] = 0x1505 // CH ＝ 綠 21、CL ＝ 藍 5
	call(m, d, 0x10, 0x1010)
	if got := [3]uint8{m.DAC[15], m.DAC[16], m.DAC[17]}; got != [3]uint8{42, 21, 5} {
		t.Errorf("DAC[5] ＝ %v，預期 [42 21 5]", got)
	}
	if m.AttrPal[5] != 5 {
		t.Errorf("AttrPal[5] 被動到了（＝%d）——AL=10h 不該碰屬性調色盤", m.AttrPal[5])
	}
}

// TestPaletteBlockRoundTrip：AL=12h 寫一段 DAC、AL=17h 讀回來要一樣。
//
// 讀回沒實作的症狀最惡劣：程式拿「存起來的調色盤」去做淡出淡入，
// 存到的是未初始化的緩衝區，於是**淡回來的是垃圾或全黑**，
// 而寫入路徑本身完全正常，查不到是誰的責任。
func TestPaletteBlockRoundTrip(t *testing.T) {
	m, d := newTest(t)
	src := []byte{1, 2, 3, 60, 61, 62, 10, 20, 30}
	m.WriteBytes(0x30000, src)
	c := m.CPU
	c.Seg[cpu.ES], c.R[cpu.DX] = 0x3000, 0
	c.R[cpu.BX], c.R[cpu.CX] = 2, 3
	call(m, d, 0x10, 0x1012)

	for i, want := range src {
		if got := m.DAC[2*3+i]; got != want {
			t.Fatalf("DAC[%d] ＝ %d，預期 %d", 2*3+i, got, want)
		}
	}
	c.Seg[cpu.ES], c.R[cpu.DX] = 0x3000, 0x100
	c.R[cpu.BX], c.R[cpu.CX] = 2, 3
	call(m, d, 0x10, 0x1017)
	for i, want := range src {
		if got := m.Read8(0x30100 + uint32(i)); got != want {
			t.Errorf("讀回的第 %d 個 byte ＝ %d，預期 %d", i, got, want)
		}
	}
}

// TestPaletteAttributeRegistersRoundTrip：AL=02h／09h 動的是
// **16 個屬性暫存器 ＋ overscan**（17 bytes），不是 768 bytes 的 DAC。
//
// 接成「設整份 DAC」的話，那 17 bytes 會被當成顏色灌進 DAC，
// 而 17 bytes 之後的記憶體也一起被讀進去——畫面顏色會變成一串遞增值。
func TestPaletteAttributeRegistersRoundTrip(t *testing.T) {
	m, d := newTest(t)
	var src [17]byte
	for i := range src {
		src[i] = uint8(0x3F - i)
	}
	m.WriteBytes(0x30000, src[:])
	m.DAC[0] = 9 // 不該被動到
	c := m.CPU
	c.Seg[cpu.ES], c.R[cpu.DX] = 0x3000, 0
	call(m, d, 0x10, 0x1002)

	for i := 0; i < 16; i++ {
		if m.AttrPal[i] != src[i] {
			t.Fatalf("AttrPal[%d] ＝ %d，預期 %d", i, m.AttrPal[i], src[i])
		}
	}
	if m.Overscan != src[16] {
		t.Errorf("overscan ＝ %d，預期 %d", m.Overscan, src[16])
	}
	if m.DAC[0] != 9 {
		t.Error("AL=02h 動到了 DAC——它只該碰屬性調色盤")
	}

	c.Seg[cpu.ES], c.R[cpu.DX] = 0x3000, 0x100
	call(m, d, 0x10, 0x1009)
	for i := 0; i < 17; i++ {
		if got := m.Read8(0x30100 + uint32(i)); got != src[i] {
			t.Errorf("讀回的第 %d 個 byte ＝ %d，預期 %d", i, got, src[i])
		}
	}
}

// TestVectorStubRunsService：程式用「AH=35h 取向量 → 直接跳過去」呼叫
// BIOS 時，服務要照樣執行（`docs/spec/011` §2）。
//
// C 的 `int86x` 就是這樣做的：它不發 `int nn`，而是自己疊一個中斷框架再
// `iret` 跳進 handler。向量指到裸 `iret` 的話這條路**安靜地什麼都不做**
// ——暫存器原樣回去，沒有錯誤也沒有 unimplemented 記錄。
func TestVectorStubRunsService(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x12)
	c := m.CPU

	// AH=35h 取 int 10h 的向量。
	c.R[cpu.AX] = 0x3510
	d.handle(c, 0x21)
	seg, off := c.Seg[cpu.ES], c.R[cpu.BX]
	if seg != machine.StubSeg {
		t.Fatalf("int 10h 的向量段 ＝ %04X，預期 %04X", seg, machine.StubSeg)
	}
	if first := m.Read8(uint32(seg)*16 + uint32(off)); first == 0xCF {
		t.Fatal("stub 的第一個位元組是 CFh——滑鼠偵測會判定沒有驅動")
	}

	// 疊一個中斷框架，然後跳進 stub：等同 int86x 的做法。
	c.Seg[cpu.SS], c.R[cpu.SP] = 0x4000, 0x100
	push := func(v uint16) {
		c.R[cpu.SP] -= 2
		m.Write16(cpu.Addr(c.Seg[cpu.SS], c.R[cpu.SP]), v)
	}
	push(0)      // flags
	push(0x2000) // 回程 CS
	push(0x0040) // 回程 IP
	c.Seg[cpu.CS], c.IP = seg, off
	c.R[cpu.AX] = 0x0F00 // 取目前視訊模式

	for i := 0; i < 4 && c.Seg[cpu.CS] == machine.StubSeg; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if got := uint8(c.R[cpu.AX]); got != 0x12 {
		t.Errorf("AL ＝ %02X，預期 12（服務沒有跑到）", got)
	}
	if c.Seg[cpu.CS] != 0x2000 || c.IP != 0x0040 {
		t.Errorf("stub 沒有回到呼叫端：CS:IP ＝ %04X:%04X", c.Seg[cpu.CS], c.IP)
	}
}
