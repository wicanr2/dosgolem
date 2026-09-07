package dos

import (
	"sort"

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

	// ⚠ **常規記憶體那一端要先算成線性位址再往前走。**
	// handle 0 的位移欄是 far 指標（高 word 段、低 word 位移），
	// 把位元組索引直接加上去的話，位移滿 65,536 就進位到**段 +1**
	// ＝ 線性只前進 16 bytes，於是超過 64 KB 的部分整段讀到別的地方。
	// 搬移照樣回報成功，前 64 KB 也照樣正確——所以症狀是
	// 「大部分的字都對，少數幾個字畫不出來」。
	srcBase, dstBase := srcOff, dstOff
	if srcH == 0 {
		srcBase = xmsLinear(srcOff)
	}
	if dstH == 0 {
		dstBase = xmsLinear(dstOff)
	}

	// bits 是搬過去的位元數。**「搬移回報成功」不等於「搬到了東西」**——
	// 來源算錯或區塊配小了都是靜靜回 0，而字圖全 0 在畫面上就是「沒這個字」。
	bits := 0
	for i := uint32(0); i < n; i++ {
		b := d.xmsRead(srcH, srcBase+i)
		bits += popcount(b)
		d.xmsWrite(dstH, dstBase+i, b)
	}
	c.R[cpu.AX] = 1
	d.XMSMoves = append(d.XMSMoves, XMSMove{
		Step: d.M.Steps, Len: n, SrcH: srcH, SrcOff: srcOff, DstH: dstH, DstOff: dstOff,
		Bits: bits,
	})
	clearCarry(c)
}

// xmsLinear 把 handle 0 的 far 指標（高 word 段、低 word 位移）算成
// 線性位址。算完才可以逐位元組往前走。
func xmsLinear(off uint32) uint32 { return (off>>16)<<4 + off&0xFFFF }

// xmsRead 從 EMB 或常規記憶體讀一個位元組。handle 0 收的是**線性位址**。
func (d *DOS) xmsRead(h uint16, off uint32) uint8 {
	if h == 0 {
		return d.M.Read8(off)
	}
	if blk, ok := d.emb[h]; ok && off < uint32(len(blk)) {
		return blk[off]
	}
	return 0
}

// xmsWrite 寫一個位元組。handle 0 收的是**線性位址**。
func (d *DOS) xmsWrite(h uint16, off uint32, v uint8) {
	if h == 0 {
		d.M.Write8(off, v)
		return
	}
	if blk, ok := d.emb[h]; ok && off < uint32(len(blk)) {
		blk[off] = v
	}
}

// EMBSizes 回報目前配出去的 EMB（handle 與位元組數），按 handle 排序。
//
// **「搬進去了」不等於「存下來了」**：`xmsWrite` 對超出區塊的位址是
// 靜靜丟掉。區塊配小了的話，超過那個界線的資料全部讀回 0，
// 而搬移本身回報成功。
func (d *DOS) EMBSizes() [][2]int {
	out := make([][2]int, 0, len(d.emb))
	for h, b := range d.emb {
		out = append(out, [2]int{int(h), len(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

func popcount(b uint8) int {
	n := 0
	for ; b != 0; b &= b - 1 {
		n++
	}
	return n
}
