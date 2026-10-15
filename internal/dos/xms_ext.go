package dos

import (
	"fmt"
	"sort"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// XMS 區塊實體化（`docs/spec/430-physical-memory-and-xms-unification` §2.2）。
//
// 只在 `machine.SetExtendedMemoryKB(kb>0)` 之後生效；kb == 0 時 xms.go 的舊路徑一行都不變。
// 行為以 DOSBox-X `src/ints/xms.cpp` 與 `src/hardware/memory.cpp` 為準。
//
// 區塊住在 Machine.Ext：配置區是 HMA 之後（實體 0x110000）到 Ext 結尾，粒度 4 KB 頁。
// 頁號一律是**相對 Ext 起點**（實體 0x100000）的頁號，所以第一個可配置頁是 xmsFirstPage。

const (
	xmsPageSize  = 4096
	xmsFirstPage = (machine.HMASize + 16) / xmsPageSize // 實體 0x110000，Ext 的第 16 頁

	// 錯誤碼（DOSBox-X xms.cpp:70-86）。
	xmsErrNotImplemented = 0x80
	xmsErrOutOfSpace     = 0xA0
	xmsErrOutOfHandles   = 0xA1
	xmsErrInvalidHandle  = 0xA2
	xmsErrSrcHandle      = 0xA3
	xmsErrSrcOffset      = 0xA4
	xmsErrDstHandle      = 0xA5
	xmsErrDstOffset      = 0xA6
	xmsErrLength         = 0xA7
	xmsErrNotLocked      = 0xAA
	xmsErrLocked         = 0xAB

	// xmsRealModeLimit：handle 0 的 段:偏移 換算位址加長度不得超過它（xms.cpp:307、325）。
	xmsRealModeLimit = 0x10FFF0
)

// xmsBlock 是一塊實體化的 EMB。base 是相對 Ext 的頁號，pages 是占用頁數，
// kb 是請求的大小（Move 的界線以 kb×1024 計，不以取整後的頁數）。
//
// 占 0 頁的區塊（`09h` 配 0 KB，或 `0Fh` 縮到 0）不占用任何頁：
// `09h` 的 0 KB 區塊 base 是 BestMatch(1) 的結果（沒有空頁時為 0），縮到 0 的區塊 base 為 0。
type xmsBlock struct {
	base, pages, kb, locks int
}

func (d *DOS) xmsTotalPages() int { return len(d.M.Ext) / xmsPageSize }

func (d *DOS) xmsValid(h uint16) bool {
	_, ok := d.xmsBlocks[h]
	return h != 0 && ok
}

// xmsUsed 回每一頁有沒有被某個區塊占用（頁 0 到 xmsFirstPage-1 是 HMA，視為占用）。
func (d *DOS) xmsUsed() []bool {
	used := make([]bool, d.xmsTotalPages())
	for i := 0; i < xmsFirstPage && i < len(used); i++ {
		used[i] = true
	}
	for _, b := range d.xmsBlocks {
		for i := 0; i < b.pages; i++ {
			used[b.base+i] = true
		}
	}
	return used
}

// xmsBestMatch 照 DOSBox-X `BestMatch`（memory.cpp:1029-1064）：從低位址掃描，
// 遇到頁數剛好相等的空洞立即採用；否則取能容納的最小空洞，同大小取位址較低者。
// 回 0 表示沒有（0 一定是 HMA 頁，不會是合法結果）。
func xmsBestMatch(used []bool, size int) int {
	index, first, best, bestFirst := xmsFirstPage, 0, 0xfffffff, 0
	for index < len(used) {
		if first == 0 {
			if !used[index] {
				first = index
			}
		} else if used[index] {
			pages := index - first
			if pages == size {
				return first
			} else if pages > size && pages < best {
				best, bestFirst = pages, first
			}
			first = 0
		}
		index++
	}
	// 最後一段空洞（延伸到 Ext 結尾）
	if first != 0 && index-first >= size && index-first < best {
		return first
	}
	return bestFirst
}

// xmsFree 回空閒總頁數與最大連續空閒頁數（memory.cpp:991-1015）。
func xmsFree(used []bool) (total, largest int) {
	run := 0
	for i := xmsFirstPage; i < len(used); i++ {
		if !used[i] {
			total++
			run++
			continue
		}
		largest = max(largest, run)
		run = 0
	}
	return total, max(largest, run)
}

func xmsPagesFor(kb int) int { return (kb + 3) / 4 }

// xmsOK 是成功的回傳：AX=1。
func xmsOK(c *cpu.CPU) { c.R[cpu.AX] = 1 }

// xmsOKBL 是 DOSBox-X `SET_RESULT` 會碰 BL 的成功回傳：AX=1、BL=0。
func xmsOKBL(c *cpu.CPU) { c.R[cpu.AX] = 1; setBL(c, 0) }

// xmsExtCall 處理 kb > 0 時的 AH=08h–0Fh，回 true 表示已處理。
func (d *DOS) xmsExtCall(c *cpu.CPU) bool {
	switch ah(c) {
	case 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F:
	default:
		return false
	}
	if d.xmsBlocks == nil {
		d.xmsBlocks = map[uint16]*xmsBlock{}
	}
	switch ah(c) {
	case 0x08: // Query Free Extended Memory：AX＝最大連續 KB、DX＝總空閒 KB（HMA 不計）
		total, largest := xmsFree(d.xmsUsed())
		c.R[cpu.AX] = uint16(min(largest*4, 65535))
		c.R[cpu.DX] = uint16(min(total*4, 65535))
		setBL(c, 0)
		if total == 0 {
			setBL(c, xmsErrOutOfSpace)
		}

	case 0x09: // Allocate EMB：DX＝KB → DX＝handle
		kb := int(c.R[cpu.DX])
		var h uint16
		for i := uint16(1); i <= xmsMaxHandles; i++ {
			if _, used := d.xmsBlocks[i]; !used {
				h = i
				break
			}
		}
		if h == 0 {
			d.xmsFail(c, xmsErrOutOfHandles)
			c.R[cpu.DX] = 0
			return true
		}
		used := d.xmsUsed()
		blk := &xmsBlock{kb: kb}
		if kb != 0 {
			blk.pages = xmsPagesFor(kb)
			if blk.base = xmsBestMatch(used, blk.pages); blk.base == 0 {
				d.xmsFail(c, xmsErrOutOfSpace)
				c.R[cpu.DX] = 0
				return true
			}
		} else {
			// 0 KB：不占頁，基底是 BestMatch(1)，沒有空頁時為 0（memory.cpp:1190-1191）。不清零。
			blk.base = xmsBestMatch(used, 1)
		}
		d.xmsBlocks[h] = blk
		xmsOKBL(c)
		c.R[cpu.DX] = h

	case 0x0A: // Free EMB：DX＝handle
		switch {
		case !d.xmsValid(c.R[cpu.DX]):
			d.xmsFail(c, xmsErrInvalidHandle)
		case d.xmsBlocks[c.R[cpu.DX]].locks != 0:
			d.xmsFail(c, xmsErrLocked)
		default:
			delete(d.xmsBlocks, c.R[cpu.DX])
			xmsOKBL(c)
		}

	case 0x0B: // Move EMB：DS:SI → 描述子
		if code := d.xmsMoveExt(c); code != 0 {
			d.xmsFail(c, code)
		} else {
			xmsOK(c) // SET_RESULT(…, false)：成功時不碰 BL
		}

	case 0x0C: // Lock EMB：DX＝handle → DX:BX＝實體基底
		if !d.xmsValid(c.R[cpu.DX]) {
			d.xmsFail(c, xmsErrInvalidHandle)
			return true
		}
		b := d.xmsBlocks[c.R[cpu.DX]]
		if b.locks < 255 {
			b.locks++
		}
		var addr uint32
		if b.base != 0 {
			addr = machine.ExtBase + uint32(b.base)*xmsPageSize
		}
		c.R[cpu.DX] = uint16(addr >> 16)
		c.R[cpu.BX] = uint16(addr)
		xmsOK(c)

	case 0x0D: // Unlock EMB
		if !d.xmsValid(c.R[cpu.DX]) {
			d.xmsFail(c, xmsErrInvalidHandle)
			return true
		}
		b := d.xmsBlocks[c.R[cpu.DX]]
		if b.locks == 0 {
			d.xmsFail(c, xmsErrNotLocked)
			return true
		}
		b.locks--
		xmsOKBL(c)

	case 0x0E: // Get EMB Handle Information：BH＝鎖定計數、BL＝空 handle 數、DX＝KB
		if !d.xmsValid(c.R[cpu.DX]) {
			d.xmsFail(c, xmsErrInvalidHandle)
			return true
		}
		b := d.xmsBlocks[c.R[cpu.DX]]
		c.R[cpu.BX] = uint16(b.locks)<<8 | uint16(xmsMaxHandles-len(d.xmsBlocks))
		c.R[cpu.DX] = uint16(b.kb)
		xmsOK(c)

	case 0x0F: // Reallocate EMB：BX＝新的 KB、DX＝handle
		d.xmsRealloc(c)
	}
	return true
}

// xmsRealloc 是 AH=0Fh（規格 §2.2 表）。
func (d *DOS) xmsRealloc(c *cpu.CPU) {
	h := c.R[cpu.DX]
	if !d.xmsValid(h) {
		d.xmsFail(c, xmsErrInvalidHandle)
		return
	}
	b := d.xmsBlocks[h]
	if b.locks > 0 {
		d.xmsFail(c, xmsErrLocked)
		return
	}
	newKB := int(c.R[cpu.BX])
	np := xmsPagesFor(newKB)
	switch {
	case np == b.pages:
		// 新舊頁數相等：只更新 KB（memory.cpp:1225）。
		b.kb = newKB
	case b.pages == 0:
		// 占 0 頁的區塊擴大一律視為新配置，不論原基底值。刻意偏離 DOSBox-X：
		// 它的 0 KB 區塊持有一個未標記占用的頁（xms.cpp:244），擴大到 1 頁直接成功，
		// 會與其他區塊共用同一頁。
		nb := xmsBestMatch(d.xmsUsed(), np)
		if nb == 0 {
			d.xmsFail(c, xmsErrOutOfSpace)
			return
		}
		b.base, b.pages, b.kb = nb, np, newKB
	case np == 0:
		// 新大小 0：釋放全部頁，handle 保留、大小 0、基底 0（Lock 回 0）。
		b.base, b.pages, b.kb = 0, 0, 0
	case np < b.pages:
		b.pages, b.kb = np, newKB
	default:
		used := d.xmsUsed()
		need, free := np-b.pages, 0
		for i := b.base + b.pages; i < len(used) && !used[i]; i++ {
			free++
		}
		if free >= need {
			// 後方有足夠空頁：原地擴大。
			b.pages, b.kb = np, newKB
			break
		}
		// 另找空洞；找洞時舊頁仍算占用（memory.cpp:1263），配到後複製原內容再釋放舊頁。
		nb := xmsBestMatch(used, np)
		if nb == 0 {
			d.xmsFail(c, xmsErrOutOfSpace)
			return
		}
		n := b.pages * xmsPageSize
		for i := 0; i < n; i++ {
			d.M.Ext[nb*xmsPageSize+i] = d.M.Ext[b.base*xmsPageSize+i]
		}
		b.base, b.pages, b.kb = nb, np, newKB
	}
	xmsOKBL(c)
}

// xmsMoveExt 是 kb > 0 時的 AH=0Bh，回 0 成功，否則回錯誤碼（xms.cpp:272-353）。
//
// 檢查次序：長度奇數 A7h；先來源三項再目的三項。handle 非 0 時依序為 handle 無效、
// 位移 ≥ 區塊大小、長度超過區塊剩餘（區塊大小以請求的 KB×1024 計）；handle 為 0 時
// 位移是實模式 段:偏移，換算位址加長度超過 0x10FFF0 回 A7h。
// 通過後逐 byte 由低位址往高位址複製（重疊時照此順序，不做 memmove）；
// 實體存取不受 A20 影響，等同 DOSBox-X 複製期間 A20 視為開啟。
func (d *DOS) xmsMoveExt(c *cpu.CPU) uint8 {
	desc := d.M.SvcAddr(c.Seg[cpu.DS], c.R[cpu.SI])
	n := uint32(d.M.Read16(desc)) | uint32(d.M.Read16(desc+2))<<16
	if n&1 != 0 {
		return xmsErrLength
	}
	srcH := d.M.Read16(desc + 4)
	srcOff := uint32(d.M.Read16(desc+6)) | uint32(d.M.Read16(desc+8))<<16
	dstH := d.M.Read16(desc + 0x0A)
	dstOff := uint32(d.M.Read16(desc+0x0C)) | uint32(d.M.Read16(desc+0x0E))<<16

	resolve := func(h uint16, off uint32, badH, badOff uint8) (uint32, uint8) {
		if h == 0 {
			p := xmsLinear(off)
			if uint64(p)+uint64(n) > xmsRealModeLimit {
				return 0, xmsErrLength
			}
			return p, 0
		}
		if !d.xmsValid(h) {
			return 0, badH
		}
		b := d.xmsBlocks[h]
		size := uint32(b.kb) * 1024
		if off >= size {
			return 0, badOff
		}
		if n > size-off {
			return 0, xmsErrLength
		}
		return machine.ExtBase + uint32(b.base)*xmsPageSize + off, 0
	}
	srcPt, code := resolve(srcH, srcOff, xmsErrSrcHandle, xmsErrSrcOffset)
	if code != 0 {
		return code
	}
	dstPt, code := resolve(dstH, dstOff, xmsErrDstHandle, xmsErrDstOffset)
	if code != 0 {
		return code
	}
	bits := 0
	for i := uint32(0); i < n; i++ {
		v := d.M.ReadPhys8(srcPt + i)
		bits += popcount(v)
		d.M.WritePhys8(dstPt+i, v)
	}
	d.XMSMoves = append(d.XMSMoves, XMSMove{
		Step: d.M.Steps, Len: n, SrcH: srcH, SrcOff: srcOff, DstH: dstH, DstOff: dstOff, Bits: bits,
	})
	return 0
}

// xmsBlocksState 把區塊表轉成依 handle 排序的線上版本。
func (d *DOS) xmsBlocksState() []xmsBlockState {
	var out []xmsBlockState
	for h, b := range d.xmsBlocks {
		out = append(out, xmsBlockState{H: h, BasePage: b.base, Pages: b.pages, KB: b.kb, Locks: b.locks})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].H < out[j].H })
	return out
}

// checkXMSState 在任何寫入之前驗證狀態檔的 XMS 內容與這台機器相容。
func (d *DOS) checkXMSState(s *dosState) error {
	kb := d.M.ExtendedMemoryKB()
	if kb > 0 && (len(s.EMB) > 0 || len(s.EMBList) > 0) {
		return fmt.Errorf("dos: 狀態檔含 kb == 0 的 EMB，這台機器的延伸記憶體是 %d KB", kb)
	}
	if kb == 0 {
		if len(s.XMSBlocks) > 0 {
			return fmt.Errorf("dos: 狀態檔含實體化的 XMS 區塊，這台機器沒有延伸記憶體")
		}
		return nil
	}
	total := kb * 1024 / xmsPageSize
	owner := map[int]uint16{}
	seen := map[uint16]bool{}
	for _, b := range s.XMSBlocks {
		if b.H < 1 || b.H > xmsMaxHandles || seen[b.H] {
			return fmt.Errorf("dos: 狀態檔的 XMS handle %d 無效或重複", b.H)
		}
		seen[b.H] = true
		if b.KB < 0 || b.KB > 65535 || b.Pages != xmsPagesFor(b.KB) || b.Locks < 0 || b.Locks > 255 {
			return fmt.Errorf("dos: 狀態檔的 XMS handle %d 欄位不一致（KB %d、頁 %d、鎖 %d）", b.H, b.KB, b.Pages, b.Locks)
		}
		if b.Pages == 0 {
			if b.BasePage != 0 && (b.BasePage < xmsFirstPage || b.BasePage >= total) {
				return fmt.Errorf("dos: 狀態檔的 XMS handle %d 基底頁 %d 超出範圍", b.H, b.BasePage)
			}
			continue
		}
		if b.BasePage < xmsFirstPage || b.BasePage+b.Pages > total {
			return fmt.Errorf("dos: 狀態檔的 XMS handle %d 頁 %d+%d 超出 Ext（%d 頁）", b.H, b.BasePage, b.Pages, total)
		}
		for i := 0; i < b.Pages; i++ {
			if o, dup := owner[b.BasePage+i]; dup {
				return fmt.Errorf("dos: 狀態檔的 XMS handle %d 與 %d 的頁重疊", b.H, o)
			}
			owner[b.BasePage+i] = b.H
		}
	}
	return nil
}
