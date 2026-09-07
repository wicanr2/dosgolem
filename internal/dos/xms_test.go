package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// XMS（`docs/spec/011`）的釘死測試。

// TestXMSMoveCrosses64K 釘住常規記憶體那一端的位址算法。
//
// handle 0 的位移欄是 **far 指標**（高 word 段、低 word 位移）。
// 把位元組索引直接加在那個打包過的值上，位移滿 65,536 就進位到
// **段 +1** ＝ 線性只前進 16 bytes，於是超過 64 KB 的部分整段搬錯。
//
// 這一格錯了不會有任何症狀提示：搬移回報成功，前 64 KB 也完全正確。
// 源平合戰把 286 KB 的 `JIS.FNT` 一次搬進 XMS，結果只有前 64 KB 是對的，
// 畫面上的表現是「大部分的字都對，少數幾個字空白」——看起來像字型缺字。
func TestXMSMoveCrosses64K(t *testing.T) {
	m := machine.New()
	d := New(m, ".")

	const (
		srcSeg = 0x2000
		n      = 0x30000 // 192 KB，跨三個 64 KB 界線
	)
	srcLin := uint32(srcSeg) * 16
	for i := uint32(0); i < n; i++ {
		m.Write8(srcLin+i, uint8(i*7+i>>8))
	}

	// 配一塊 EMB。
	m.CPU.R[cpu.AX] = 0x0900
	m.CPU.R[cpu.DX] = n / 1024
	d.handle(m.CPU, 0xF5)
	if m.CPU.R[cpu.AX] != 1 {
		t.Fatal("配 EMB 失敗")
	}
	h := m.CPU.R[cpu.DX]

	// 描述子放在一段沒人用的低位記憶體。
	const descSeg, descOff = 0x1000, 0x0000
	desc := cpu.Addr(descSeg, descOff)
	m.Write16(desc, uint16(n&0xFFFF))
	m.Write16(desc+2, uint16(n>>16))
	m.Write16(desc+4, 0) // 來源 handle 0 ＝ 常規記憶體
	m.Write16(desc+6, 0x0000)
	m.Write16(desc+8, srcSeg)
	m.Write16(desc+0x0A, h)
	m.Write16(desc+0x0C, 0)
	m.Write16(desc+0x0E, 0)

	m.CPU.R[cpu.AX] = 0x0B00
	m.CPU.Seg[cpu.DS] = descSeg
	m.CPU.R[cpu.SI] = descOff
	d.handle(m.CPU, 0xF5)
	if m.CPU.R[cpu.AX] != 1 {
		t.Fatal("搬進 EMB 失敗")
	}

	// 搬回另一段常規記憶體，逐位元組比。
	const dstSeg = 0x6000
	dstLin := uint32(dstSeg) * 16
	m.Write16(desc+4, h)
	m.Write16(desc+6, 0)
	m.Write16(desc+8, 0)
	m.Write16(desc+0x0A, 0)
	m.Write16(desc+0x0C, 0x0000)
	m.Write16(desc+0x0E, dstSeg)

	m.CPU.R[cpu.AX] = 0x0B00
	m.CPU.Seg[cpu.DS] = descSeg
	m.CPU.R[cpu.SI] = descOff
	d.handle(m.CPU, 0xF5)
	if m.CPU.R[cpu.AX] != 1 {
		t.Fatal("搬回常規記憶體失敗")
	}

	for i := uint32(0); i < n; i++ {
		want := m.Read8(srcLin + i)
		if got := m.Read8(dstLin + i); got != want {
			t.Fatalf("第 %d（0x%X）個位元組 ＝ %02X，要 %02X；"+
				"跨 64 KB 之後就錯，表示位址是加在 far 指標上而不是線性位址上",
				i, i, got, want)
		}
	}
}
