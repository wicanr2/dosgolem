package cpu

// D=1 碼段的 32 位元定址字串指令（docs/spec/443-segmented-32bit-code-and-stack §2.5）。
//
// 只在分段模式、cache[CS] 的 D=1、addr32=true 時由 execute386 呼叫（D=1 帶 67 的 16 位元定址走 stringOp／stringOp32）：
//
//   - 位址暫存器 ESI、EDI，計數 ECX，皆以 32 位元遞增或遞減（DF），不截成 16 位元；偏移以 32 位元交給
//     segRead／segWrite（w32=true）。來源段可被前綴覆寫，目的段恆為 ES（string.go 的規則）。
//   - 寬度：A4、A6、AA、AC、AE 為 byte；其餘看 operand32（dword，帶 66 為 word）。
//   - rep 先看 ECX；repe／repne 只對 cmps、scas 判 ZF，每次迭代先減 ECX 再判 ZF。
//   - 每次迭代 charge(cycString)，DOSBoxCost 時另 Cycles++（同 stringOnce）。
//   - 一步最多 repStepLimit 次迭代；達到上限仍要繼續時 IP 設回 opIP，下一步重新解碼續跑。
//   - 本道指令的 c.fault 非 nil（界限、型別違反）時，該次迭代不寫入、不減 ECX、ESI 與 EDI 不前進，迴圈結束。
//   - 記憶體存取經 strRead／strWrite：區間檢查（spanOK）通過時略過逐 byte 的 segCheck，否則照 segRead／segWrite。

// repStepLimit 是單道 rep 在一次 Step 內的迭代上限（443 §2.5，2^20）。
const repStepLimit = 1 << 20

func (c *CPU) stringOpA32(op uint8) {
	size := uint32(1)
	if op&1 == 1 {
		size = 2
		if c.operand32 {
			size = 4
		}
	}
	d := size
	if c.Flag(DF) {
		d = -size
	}
	if c.repPrefix == 0 {
		c.stringOnceA32(op, size, d)
		return
	}
	checkZF := op == 0xA6 || op == 0xA7 || op == 0xAE || op == 0xAF
	wantZF := c.repPrefix == 0xF3
	for n := 0; c.reg32(CX) != 0; n++ {
		if n == repStepLimit {
			c.IP = c.opIP // 已完成的迭代已提交；下一步重新解碼同一道指令續跑
			return
		}
		if !c.stringOnceA32(op, size, d) {
			return
		}
		c.setReg32(CX, c.reg32(CX)-1)
		if checkZF && c.Flag(ZF) != wantZF {
			return
		}
	}
}

// stringOnceA32 做一次迭代；本道指令已有或新增 fault 時回 false，且不寫入、不更新旗標、ESI 與 EDI 不前進。
func (c *CPU) stringOnceA32(op uint8, size, d uint32) bool {
	c.charge(cycString)
	if c.DOSBoxCost {
		c.Cycles++
	}
	if c.fault != nil {
		return false
	}
	src := c.dataSeg(DS) // 可被段前綴覆寫
	const dst = ES       // 不可覆寫
	n := int(size)
	esi, edi := c.reg32(SI), c.reg32(DI)
	switch op {
	case 0xA4, 0xA5: // MOVS
		v := c.strRead(src, esi, n)
		if c.fault != nil {
			return false
		}
		c.strWrite(dst, edi, n, v)
		if c.fault != nil {
			return false
		}
		c.setReg32(SI, esi+d)
		c.setReg32(DI, edi+d)
	case 0xA6, 0xA7: // CMPS：[ESI] − [EDI]
		a := c.strRead(src, esi, n)
		b := c.strRead(dst, edi, n)
		if c.fault != nil {
			return false
		}
		c.cmpWidth(a, b, n)
		c.setReg32(SI, esi+d)
		c.setReg32(DI, edi+d)
	case 0xAA, 0xAB: // STOS
		c.strWrite(dst, edi, n, c.accWidth(n))
		if c.fault != nil {
			return false
		}
		c.setReg32(DI, edi+d)
	case 0xAC, 0xAD: // LODS
		v := c.strRead(src, esi, n)
		if c.fault != nil {
			return false
		}
		switch n {
		case 1:
			c.setReg8(0, uint8(v))
		case 2:
			c.R[AX] = uint16(v)
		default:
			c.setReg32(AX, v)
		}
		c.setReg32(SI, esi+d)
	case 0xAE, 0xAF: // SCAS：AL/AX/EAX − [EDI]
		b := c.strRead(dst, edi, n)
		if c.fault != nil {
			return false
		}
		c.cmpWidth(c.accWidth(n), b, n)
		c.setReg32(DI, edi+d)
	}
	return true
}

// spanOK 是 §2.5 允許的區間檢查快速路徑：段 i 為 present 的向上展開資料段（寫入時可寫），且 off..off+n−1 不跨
// 32 位元環繞、不超過界限時回 true，此時逐 byte 的 segCheck 必定全部通過。其餘情形回 false，由 segRead／segWrite
// 逐 byte 檢查，失敗點與失敗訊息不變。
func (c *CPU) spanOK(i int, off uint32, n int, write bool) bool {
	sc := &c.cache[i]
	a := sc.Attr
	if a&(attrP|attrS|attrExec|attrEC) != attrP|attrS || (write && a&attrRW == 0) {
		return false
	}
	end := off + uint32(n-1)
	return end >= off && end <= sc.Limit
}

// strRead／strWrite：32 位元定址字串指令的記憶體存取（w32），區間檢查通過時略過逐 byte 的 segCheck。
func (c *CPU) strRead(i int, off uint32, n int) uint32 {
	if !c.spanOK(i, off, n, false) {
		return c.segRead(i, off, n, true)
	}
	a := c.cache[i].Base + off
	var v uint32
	for k := 0; k < n; k++ {
		v |= uint32(c.Bus.Read8(a+uint32(k))) << (8 * k)
	}
	return v
}

func (c *CPU) strWrite(i int, off uint32, n int, v uint32) {
	if !c.spanOK(i, off, n, true) {
		c.segWrite(i, off, n, true, v)
		return
	}
	a := c.cache[i].Base + off
	for k := 0; k < n; k++ {
		c.Bus.Write8(a+uint32(k), uint8(v>>(8*k)))
	}
}

// accWidth 回 n bytes 寬的累加器（AL、AX、EAX）。
func (c *CPU) accWidth(n int) uint32 {
	switch n {
	case 1:
		return uint32(uint8(c.R[AX]))
	case 2:
		return uint32(c.R[AX])
	}
	return c.reg32(AX)
}

// cmpWidth 以 n bytes 寬做 a − b 的旗標（cmps、scas），與 stringOnce、stringOnce32 用同一組減法。
func (c *CPU) cmpWidth(a, b uint32, n int) {
	if n == 4 {
		c.sub32(a, b, 0)
		return
	}
	c.sub(a, b, 0, 8*n)
}
