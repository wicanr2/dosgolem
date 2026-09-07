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

// XMS 的容量與上位記憶體的範圍。
//
// UMB 放在 `C000h`–`CFFFh`：`D000h` 是 EMS 的 page frame（`machine.EMSFrameSeg`），
// 兩者重疊的話 EMS 換頁會把 UMB 裡的東西換掉，而那看起來像「常駐程式突然壞了」。
const (
	xmsTotalKB    = 8192
	xmsMaxHandles = 64
	umbStart      = 0xC000
	umbEnd        = 0xD000
	// xmsLockBase 是 `AH=0Ch` 合成位址的基底，挑在 16 MB 之上——
	// 低於它的位址空間有真的東西（傳統記憶體、UMB、HMA），
	// 撞在一起的話診斷裡會看到兩種來源指向同一個位址。
	xmsLockBase = 0x01000000
)

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
		// AX ＝ XMS 版本（3.0）、BX ＝ 驅動內部版本、DX ＝ **HMA 存不存在**。
		// DX=0 的話程式連 `AH=01h` 都不會問，直接把資料放進傳統記憶體。
		c.R[cpu.AX] = 0x0300
		c.R[cpu.BX] = 0x0001
		c.R[cpu.DX] = 1

	case 0x01: // Request HMA：DX ＝ 要幾個 byte（TSR 用 0FFFFh 代表「整塊」）
		// 一次只有一個擁有者。已經有人拿走了要回 91h——回成功的話兩支程式
		// 會同時往同一段寫，而**它們都不會察覺**。
		if d.hmaOwned {
			d.xmsFail(c, 0x91)
			return
		}
		d.hmaOwned = true
		// **拿到 HMA 就要能定址**：A20 沒開的話 1 MB 之上環繞回 0，
		// 程式寫進去的東西會蓋掉中斷向量表。
		d.M.SetA20(true)
		c.R[cpu.AX] = 1

	case 0x02: // Release HMA
		if !d.hmaOwned {
			d.xmsFail(c, 0x93) // 沒配過
			return
		}
		d.hmaOwned = false
		c.R[cpu.AX] = 1

	case 0x03, 0x05: // Global／Local Enable A20
		d.M.SetA20(true)
		d.a20Local++
		c.R[cpu.AX] = 1

	case 0x04, 0x06: // Global／Local Disable A20
		// ⚠ **Local 是成對的**：巢狀開關要數次數，最後一次關掉才真的關。
		// 直接關的話，外層那一段程式以為 A20 還開著，接著寫進 HMA 的東西
		// 會環繞回低位記憶體——蓋掉的多半是中斷向量表。
		if d.a20Local > 0 {
			d.a20Local--
		}
		if d.a20Local == 0 && !d.hmaOwned {
			d.M.SetA20(false)
		}
		c.R[cpu.AX] = 1

	case 0x07: // Query A20
		c.R[cpu.AX] = 0
		if d.M.A20Enabled() {
			c.R[cpu.AX] = 1
		}

	case 0x08: // Query Free Extended Memory：AX ＝ 最大區塊、DX ＝ 總量（KB）
		free := xmsTotalKB - d.embUsedKB()
		c.R[cpu.AX] = free
		c.R[cpu.DX] = free
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
		if _, ok := d.emb[c.R[cpu.DX]]; !ok {
			d.xmsFail(c, 0xA2) // 無效的 handle
			return
		}
		if d.embLocks[c.R[cpu.DX]] > 0 {
			d.xmsFail(c, 0xAB) // 還鎖著
			return
		}
		delete(d.emb, c.R[cpu.DX])
		c.R[cpu.AX] = 1

	case 0x0C: // Lock EMB：DX ＝ handle → DX:BX ＝ 32 位元線性位址
		// **鎖定要回一個位址**，程式拿它做 DMA 或交給中斷處理常式。
		// EMB 的內容在 Go 這一側（延伸記憶體本來就不在 1 MB 位址空間裡），
		// 所以這裡回的是一個**合成的**位址：`xmsLockBase + handle<<20`。
		// 它不對應真的實體記憶體，但同一個 handle 每次鎖都拿到同一個值，
		// 而不同 handle 不重疊——那是呼叫端唯一會依賴的兩件事。
		// **假說待驗**：真的做 DMA 的程式會把這個位址交給硬體，
		// 而我們沒有硬體會去讀它。
		if _, ok := d.emb[c.R[cpu.DX]]; !ok {
			d.xmsFail(c, 0xA2)
			return
		}
		if d.embLocks == nil {
			d.embLocks = map[uint16]int{}
		}
		d.embLocks[c.R[cpu.DX]]++
		addr := xmsLockBase + uint32(c.R[cpu.DX])<<20
		c.R[cpu.DX] = uint16(addr >> 16)
		c.R[cpu.BX] = uint16(addr)
		c.R[cpu.AX] = 1

	case 0x0D: // Unlock EMB
		if d.embLocks[c.R[cpu.DX]] == 0 {
			d.xmsFail(c, 0xAA) // 沒鎖著
			return
		}
		d.embLocks[c.R[cpu.DX]]--
		c.R[cpu.AX] = 1

	case 0x0E: // Get EMB Handle Information：BH ＝ 鎖定次數、BL ＝ 可用 handle 數、DX ＝ KB
		blk, ok := d.emb[c.R[cpu.DX]]
		if !ok {
			d.xmsFail(c, 0xA2)
			return
		}
		free := xmsMaxHandles - len(d.emb)
		if free < 0 {
			free = 0
		}
		c.R[cpu.BX] = uint16(d.embLocks[c.R[cpu.DX]])<<8 | uint16(free)
		c.R[cpu.DX] = uint16(len(blk) / 1024)
		c.R[cpu.AX] = 1

	case 0x0F: // Reallocate EMB：BX ＝ 新的 KB、DX ＝ handle
		blk, ok := d.emb[c.R[cpu.DX]]
		if !ok {
			d.xmsFail(c, 0xA2)
			return
		}
		if d.embLocks[c.R[cpu.DX]] > 0 {
			d.xmsFail(c, 0xAB)
			return
		}
		want := int(c.R[cpu.BX]) * 1024
		// **內容要留著**：程式縮小之後仍然會讀前面那一段。
		grown := make([]byte, want)
		copy(grown, blk)
		d.emb[c.R[cpu.DX]] = grown
		c.R[cpu.AX] = 1

	case 0x10: // Request UMB：DX ＝ 要幾個段 → BX ＝ 段、DX ＝ 實際大小
		seg, size, ok := d.allocUMB(c.R[cpu.DX])
		if !ok {
			// **回實際最大的那一塊**（DX），呼叫端會照它再要一次。
			// 回 0 的話它會判定「完全沒有 UMB」然後放棄整條路徑。
			c.R[cpu.AX] = 0
			c.R[cpu.DX] = size
			setBL(c, 0xB0) // 只有比較小的 UMB
			if size == 0 {
				setBL(c, 0xB1) // 完全沒有 UMB
			}
			return
		}
		c.R[cpu.AX] = 1
		c.R[cpu.BX] = seg
		c.R[cpu.DX] = size

	case 0x11: // Release UMB：DX ＝ 段
		if !d.freeUMB(c.R[cpu.DX]) {
			d.xmsFail(c, 0xB2) // 無效的 UMB 段
			return
		}
		c.R[cpu.AX] = 1
	case 0x0B: // Move EMB：DS:SI → 描述子
		d.xmsMove(c)
	default:
		d.note(0xF5, ah(c), al(c))
		d.xmsFail(c, 0x80) // 沒有這個功能
	}
}

// xmsFail 是 XMS 的失敗慣例：**AX=0、BL ＝ 錯誤碼**（不是 CF）。
//
// 用 CF 的話呼叫端不會看——XMS 的介面從頭到尾不碰 CF，而它檢查的是 AX。
func (d *DOS) xmsFail(c *cpu.CPU, code uint8) {
	c.R[cpu.AX] = 0
	setBL(c, code)
}

// embUsedKB 是已經配出去的 EMB 總量（KB）。
func (d *DOS) embUsedKB() uint16 {
	total := 0
	for _, b := range d.emb {
		total += len(b) / 1024
	}
	return uint16(total)
}

// allocUMB 從上位記憶體切一塊（段數）。
//
// **不接進 MCB 鏈**：DOS 預設沒有把 UMB 連進鏈裡（`AH=58h` 的 UMB link 是關的），
// 走鏈的程式因此看不到它們——那與真 DOS 的預設狀態一致。接進去而不同步
// 兩邊的話，程式算出來的「可用記憶體」會包含它拿不到的段。
func (d *DOS) allocUMB(want uint16) (seg, size uint16, ok bool) {
	if d.umbFree == 0 {
		d.umbFree = umbEnd - umbStart
	}
	if want == 0 || want > d.umbFree {
		return 0, d.umbFree, false
	}
	seg = umbEnd - d.umbFree
	d.umbFree -= want
	if d.umbBlocks == nil {
		d.umbBlocks = map[uint16]uint16{}
	}
	d.umbBlocks[seg] = want
	return seg, want, true
}

// freeUMB 放掉一塊 UMB。**不回收位址**（與 DPMI 的線性配置同一個理由）：
// 放掉又配到同一段的話，「誰還留著舊指標」就查不出來了。
func (d *DOS) freeUMB(seg uint16) bool {
	if _, ok := d.umbBlocks[seg]; !ok {
		return false
	}
	delete(d.umbBlocks, seg)
	return true
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
