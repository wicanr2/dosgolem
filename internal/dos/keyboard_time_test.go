package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// TestKeyBufferIsTheBDARing 釘住「按鍵真的進 BDA 環形緩衝」。
//
// 為什麼不是「`int 16h` 回一個鍵」就夠：很多程式**不走 `int 16h`**，
// 直接讀 `0040:001A`／`0040:001C` 判斷有沒有按鍵（三國志 DOS 版就是）。
// 只補中斷服務的話那些程式一道都不會動，而且沒有任何錯誤訊號。
func TestKeyBufferIsTheBDARing(t *testing.T) {
	m := machine.New()
	const head, tail = 0x0040*16 + 0x1A, 0x0040*16 + 0x1C

	if h, tl := m.Read16(head), m.Read16(tail); h != tl {
		t.Fatalf("一開始應該是空的，head=%04X tail=%04X", h, tl)
	}
	if !m.PushKey(0x1C, '\n') {
		t.Fatal("送不進去")
	}
	if h, tl := m.Read16(head), m.Read16(tail); h == tl {
		t.Fatalf("送進去之後 head 不該等於 tail：%04X %04X", h, tl)
	}
	if got := m.Read16(0x0040*16 + uint32(m.Read16(head))); got != 0x1C0A {
		t.Fatalf("緩衝區內容 = %04X，要 1C0A", got)
	}
	v, ok := m.PopKey()
	if !ok || v != 0x1C0A {
		t.Fatalf("PopKey = %04X %v", v, ok)
	}
	if h, tl := m.Read16(head), m.Read16(tail); h != tl {
		t.Fatalf("取完應該回到空的：%04X %04X", h, tl)
	}
}

// TestKeyRingWrapsAndLeavesOneSlot 釘住「留一格不用」——不留的話滿與空分不出來。
func TestKeyRingWrapsAndLeavesOneSlot(t *testing.T) {
	m := machine.New()
	n := 0
	for m.PushKey(0, byte('a'+n%26)) {
		n++
		if n > 100 {
			t.Fatal("永遠塞得進去，環沒有上限")
		}
	}
	if n != 15 {
		t.Fatalf("16 筆的環要能放 15 筆，實際 %d", n)
	}
}

// TestInt16ReadsFromRing 釘住 `int 16h` 走的是同一個環，而不是另一份狀態。
func TestInt16ReadsFromRing(t *testing.T) {
	m, d := newTest(t)
	m.PushKey(0x02, '1')

	call(m, d, 0x16, 0x0100) // AH=01 查有沒有按鍵
	if m.CPU.Flag(cpu.ZF) {
		t.Fatal("有按鍵時 ZF 要是 0")
	}
	if m.CPU.R[cpu.AX] != 0x0231 {
		t.Fatalf("AH=01 要把按鍵放進 AX，得到 %04X", m.CPU.R[cpu.AX])
	}
	call(m, d, 0x16, 0x0000) // AH=00 讀走
	if m.CPU.R[cpu.AX] != 0x0231 {
		t.Fatalf("AH=00 = %04X", m.CPU.R[cpu.AX])
	}
	call(m, d, 0x16, 0x0100)
	if !m.CPU.Flag(cpu.ZF) {
		t.Fatal("讀走之後 ZF 要是 1")
	}
}

// TestInt1AReturnsBDATicks 釘住 `int 1Ah AH=00` 回的是 BDA 那一份計數。
//
// 這個功能號在 dosgolem 幾乎是白拿的（計數本來就在推進），但**沒有它，
// 靠時間換算的等待迴圈一道都轉不出去**——而且表面上完全正常。
func TestInt1AReturnsBDATicks(t *testing.T) {
	m, d := newTest(t)
	const at = 0x0040*16 + 0x6C
	m.Write16(at, 0x1234)
	m.Write16(at+2, 0x0005)
	m.Write8(0x0040*16+0x70, 1)

	call(m, d, 0x1A, 0x0000)
	if m.CPU.R[cpu.CX] != 0x0005 || m.CPU.R[cpu.DX] != 0x1234 {
		t.Fatalf("CX:DX = %04X:%04X，要 0005:1234", m.CPU.R[cpu.CX], m.CPU.R[cpu.DX])
	}
	if al(m.CPU) != 1 {
		t.Fatalf("AL（跨日旗標）= %d，要 1", al(m.CPU))
	}
	// 讀完要清：BIOS 的語意是「自從上次查詢以來」，不清的話呼叫端會一直
	// 以為剛跨過午夜。
	call(m, d, 0x1A, 0x0000)
	if al(m.CPU) != 0 {
		t.Fatalf("第二次查詢的 AL = %d，要 0", al(m.CPU))
	}
}

// TestInt21DriveAllocPointsAtRealMediaByte 釘住 `AH=1Ch` 的 `DS:BX`。
//
// 預設的「宣告成功但不動暫存器」在這一項會出事：`DS:BX` 保持呼叫端傳進來的值，
// 讀出來的媒體位元組是垃圾。
func TestInt21DriveAllocPointsAtRealMediaByte(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.BX] = 0xDEAD, 0xBEEF

	call(m, d, 0x21, 0x1C00)
	if al(m.CPU) != 8 || m.CPU.R[cpu.CX] != 512 {
		t.Fatalf("AL=%d CX=%d", al(m.CPU), m.CPU.R[cpu.CX])
	}
	addr := uint32(m.CPU.Seg[cpu.DS])*16 + uint32(m.CPU.R[cpu.BX])
	if got := m.Read8(addr); got != 0xF8 {
		t.Fatalf("DS:BX 指到 %05X，內容 %02X，要 F8", addr, got)
	}
}
