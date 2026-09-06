package dos

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// XMS 最小集（`docs/spec/011`，READY）：DOSJP 把 286 KB 的 JIS.FNT
// 搬進 XMS，base memory 才放得下 MAIN.EXE。
//
// driver entry 是 StubSeg 裡的 trampoline（`CD F5h / CF`，
// 與 `docs/spec/004` §2.1 同型）；EMB 內容放 Go 端——延伸記憶體
// 本來就不在 1 MB 位址空間裡。

// XMSTrapOff 是 XMS driver entry 在 StubSeg 裡的位移。
const XMSTrapOff = machine.XMSTrapOff

// xms 是 `int 2Fh` 的 XMS 偵測（AH=43h）。
func (d *DOS) int2F(c *cpu.CPU) {
	if ah(c) != 0x43 {
		d.note(0x2F, ah(c), al(c))
		clearCarry(c)
		return
	}
	switch al(c) {
	case 0x00: // 存在性 → AL=80h（有 XMS）
		setAL(c, 0x80)
	case 0x10: // driver entry → ES:BX
		c.Seg[cpu.ES] = machine.StubSeg
		c.R[cpu.BX] = XMSTrapOff
	default:
		d.note(0x2F, 0x43, al(c))
	}
	clearCarry(c)
}

// xmsCall 是 XMS driver entry 的分派（經 `int F5h` trampoline 進來）。
func (d *DOS) xmsCall(c *cpu.CPU) {
	if d.emb == nil {
		d.emb = map[uint16][]byte{}
		d.nextEMB = 1
	}
	switch ah(c) {
	case 0x00: // Get XMS Version
		c.R[cpu.AX] = 0x0200
		c.R[cpu.BX] = 0
		c.R[cpu.DX] = 0 // 無 HMA
	case 0x08: // Query Free Extended Memory
		c.R[cpu.AX] = 8192
		c.R[cpu.DX] = 8192
	case 0x09: // Allocate EMB：DX ＝ KB → 回 DX ＝ handle
		kb := uint32(c.R[cpu.DX])
		if kb == 0 || kb > 8192 {
			c.R[cpu.AX] = 0
			setBL(c, 0xA0) // 沒有足夠空間
			return
		}
		h := d.nextEMB
		d.nextEMB++
		d.emb[h] = make([]byte, kb*1024)
		c.R[cpu.AX] = 1
		c.R[cpu.DX] = h
	case 0x0A: // Free EMB：DX ＝ handle
		delete(d.emb, c.R[cpu.DX])
		c.R[cpu.AX] = 1
	case 0x0B: // Move EMB：DS:SI → 描述子
		d.xmsMove(c)
	default:
		d.note(0xF5, ah(c), al(c))
		clearCarry(c)
	}
}

// xmsMove 搬移。描述子：+0 dword 長度、+4 word 來源 handle、
// +6 dword 來源位移、+0Ah word 目的 handle、+0Ch dword 目的位移。
// **handle 0 ＝ 常規記憶體**，此時位移欄是 seg:off 的 far 指標。
func (d *DOS) xmsMove(c *cpu.CPU) {
	desc := cpu.Addr(c.Seg[cpu.DS], c.R[cpu.SI])
	n := uint32(d.M.Read16(desc)) | uint32(d.M.Read16(desc+2))<<16
	srcH := d.M.Read16(desc + 4)
	srcOff := uint32(d.M.Read16(desc+6)) | uint32(d.M.Read16(desc+8))<<16
	dstH := d.M.Read16(desc + 0x0A)
	dstOff := uint32(d.M.Read16(desc+0x0C)) | uint32(d.M.Read16(desc+0x0E))<<16

	for i := uint32(0); i < n; i++ {
		b := d.xmsRead(srcH, srcOff+i)
		d.xmsWrite(dstH, dstOff+i, b)
	}
	c.R[cpu.AX] = 1
	clearCarry(c)
}

// xmsRead 從 EMB 或常規記憶體讀一個位元組。
func (d *DOS) xmsRead(h uint16, off uint32) uint8 {
	if h == 0 {
		// far 指標：低 word 是 offset、高 word 是 segment。
		return d.M.Read8((off>>16<<4) + off&0xFFFF)
	}
	if blk, ok := d.emb[h]; ok && off < uint32(len(blk)) {
		return blk[off]
	}
	return 0
}

func (d *DOS) xmsWrite(h uint16, off uint32, v uint8) {
	if h == 0 {
		d.M.Write8((off>>16<<4)+off&0xFFFF, v)
		return
	}
	if blk, ok := d.emb[h]; ok && off < uint32(len(blk)) {
		blk[off] = v
	}
}
