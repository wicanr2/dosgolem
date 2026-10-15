package cpu

import "math/bits"

// 32 位元運算與旗標（`docs/spec/197-cpu386-real-mode-subset` §4）。
//
// 旗標語意**照 Intel 手冊的定義**，並與 DOSBox-X 的 lazy flags
// （`src/cpu/flags.cpp`、`src/cpu/instructions.h`）逐項交叉核對。兩處刻意
// 不照 DOSBox-X：
//
//   - PF 只看結果的**低位元組**（Intel）。DOSBox-X 的 `PARITY32` 把四個位元組
//     的奇偶全部 XOR 起來（`flags.cpp:433-434`），32 位元結果的 PF 與真機不同。
//   - `SHR r/m32, 1` 的 OF ＝ 原運算元的最高位。DOSBox-X 寫成 `var1 > 0x80000000`
//     （`flags.cpp:407`），原運算元恰為 `0x80000000` 時少了 1。
//
// 手冊標為「未定義」的旗標（移位多位的 OF、AF；MUL／IMUL／DIV／IDIV 的
// SF／ZF／AF／PF）**一律挑確定值**：移位的 AF 清 0、OF 依 DOSBox-X 的算式；
// 乘除法不動它們。程式讀這些旗標屬於程式自己的 bug，沒有硬體語料可驗。

const sign32 = 0x80000000

// setSZP32 設 SF／ZF／PF（PF 只看低位元組）。
func (c *CPU) setSZP32f(v uint32) {
	c.setFlag(SF, v&sign32 != 0)
	c.setFlag(ZF, v == 0)
	c.setFlag(PF, parity(uint16(v)))
}

// add32 是 ADD／ADC 共用的本體。carry 帶 0 或 1。
func (c *CPU) add32(a, b, carry uint32) uint32 {
	full := uint64(a) + uint64(b) + uint64(carry)
	res := uint32(full)
	c.setFlag(CF, full>>32 != 0)
	c.setFlag(AF, (a&0xF)+(b&0xF)+carry > 0xF)
	c.setFlag(OF, (a^res)&(b^res)&sign32 != 0)
	c.setSZP32f(res)
	return res
}

// sub32 是 SUB／SBB／CMP 共用的本體。borrow 帶 0 或 1。
func (c *CPU) sub32(a, b, borrow uint32) uint32 {
	res := a - b - borrow
	c.setFlag(CF, uint64(a) < uint64(b)+uint64(borrow))
	c.setFlag(AF, (a&0xF) < (b&0xF)+borrow)
	c.setFlag(OF, (a^b)&(a^res)&sign32 != 0)
	c.setSZP32f(res)
	return res
}

// logic32 是 AND／OR／XOR／TEST：CF、OF 清 0，AF 挑 0。
func (c *CPU) logic32(res uint32) uint32 {
	c.setFlag(CF, false)
	c.setFlag(OF, false)
	c.setFlag(AF, false)
	c.setSZP32f(res)
	return res
}

func (c *CPU) inc32(v uint32) uint32 {
	cf := c.Flag(CF) // INC 不動 CF
	r := c.add32(v, 1, 0)
	c.setFlag(CF, cf)
	return r
}

func (c *CPU) dec32(v uint32) uint32 {
	cf := c.Flag(CF)
	r := c.sub32(v, 1, 0)
	c.setFlag(CF, cf)
	return r
}

func (c *CPU) neg32(v uint32) uint32 {
	res := -v
	c.setFlag(CF, v != 0)
	c.setFlag(AF, v&0xF != 0)
	c.setFlag(OF, v == sign32)
	c.setSZP32f(res)
	return res
}

// alu32 是 00–3D／81／83 的八個運算，kind 順序 ADD OR ADC SBB AND SUB XOR CMP。
// 回傳 (結果, 要不要寫回)；CMP 只設旗標。
func (c *CPU) alu32(kind int, a, b uint32) (uint32, bool) {
	carry := uint32(0)
	if c.Flag(CF) {
		carry = 1
	}
	switch kind {
	case 0:
		return c.add32(a, b, 0), true
	case 1:
		return c.logic32(a | b), true
	case 2:
		return c.add32(a, b, carry), true
	case 3:
		return c.sub32(a, b, carry), true
	case 4:
		return c.logic32(a & b), true
	case 5:
		return c.sub32(a, b, 0), true
	case 6:
		return c.logic32(a ^ b), true
	default: // CMP
		c.sub32(a, b, 0)
		return 0, false
	}
}

// shift32 是 C1／D1／D3 的八個移位與旋轉，n 已遮成 1–31。
//
// `/6` 是 `/4`（SHL）的別名（DOSBox-X `GRP2D` 同）。
// 旋轉只動 CF／OF；移位動 CF／OF／SF／ZF／PF，AF 清 0。
func (c *CPU) shift32(op int, v uint32, n uint8) uint32 {
	switch op {
	case 0: // ROL
		res := bits.RotateLeft32(v, int(n))
		cf := res&1 != 0
		c.setFlag(CF, cf)
		c.setFlag(OF, cf != (res&sign32 != 0))
		return res
	case 1: // ROR
		res := bits.RotateLeft32(v, -int(n))
		c.setFlag(CF, res&sign32 != 0)
		c.setFlag(OF, (res&sign32 != 0) != (res&(sign32>>1) != 0))
		return res
	case 2: // RCL：經過 CF 的 33 位元旋轉
		cf := uint32(0)
		if c.Flag(CF) {
			cf = 1
		}
		res := v<<n | cf<<(n-1) | v>>(33-n)
		newCF := (v>>(32-n))&1 != 0
		c.setFlag(CF, newCF)
		c.setFlag(OF, newCF != (res&sign32 != 0))
		return res
	case 3: // RCR
		cf := uint32(0)
		if c.Flag(CF) {
			cf = 1
		}
		res := v>>n | cf<<(32-n) | v<<(33-n)
		c.setFlag(CF, (v>>(n-1))&1 != 0)
		c.setFlag(OF, (res&sign32 != 0) != (res&(sign32>>1) != 0))
		return res
	case 4, 6: // SHL／SAL
		res := v << n
		c.setFlag(CF, (v>>(32-n))&1 != 0)
		c.setFlag(OF, (res^v)&sign32 != 0)
		c.setFlag(AF, false)
		c.setSZP32f(res)
		return res
	case 5: // SHR
		res := v >> n
		c.setFlag(CF, (v>>(n-1))&1 != 0)
		c.setFlag(OF, n == 1 && v&sign32 != 0)
		c.setFlag(AF, false)
		c.setSZP32f(res)
		return res
	default: // 7：SAR
		res := uint32(int32(v) >> n)
		c.setFlag(CF, (int32(v)>>(n-1))&1 != 0)
		c.setFlag(OF, false)
		c.setFlag(AF, false)
		c.setSZP32f(res)
		return res
	}
}

// doubleShift32 是 SHLD／SHRD r/m32, r32, n（n 已遮成 1–31）。
//
//	SHLD：dst 左移 n，空出來的低位從 src 的高位補。CF ＝ 最後移出的位元。
//	SHRD：dst 右移 n，空出來的高位從 src 的低位補。
//
// OF ＝ 結果與原 dst 的符號位不同（Intel 只在 n＝1 時定義；其他計數沿用同一算式，
// 與 Bochs 相同。DOSBox-X 的 `t_DSHLd` 用 `res ^ var1` 在 32 位元上是同一個結果，
// 16 位元版它拿 src 當 var1，與 Intel 不符，所以 16 位元不照它）。AF 清 0。
func (c *CPU) doubleShift32(left bool, dst, src uint32, n uint8) uint32 {
	var res uint32
	if left {
		res = dst<<n | src>>(32-n)
		c.setFlag(CF, (dst>>(32-n))&1 != 0)
	} else {
		res = dst>>n | src<<(32-n)
		c.setFlag(CF, (dst>>(n-1))&1 != 0)
	}
	c.setFlag(OF, (res^dst)&sign32 != 0)
	c.setFlag(AF, false)
	c.setSZP32f(res)
	return res
}

// doubleShift16 是 16 位元版（n 在 1–16；更大的計數 Intel 標為未定義，呼叫端拒絕）。
// 做法是把兩個運算元接成 32 位元再移：SHLD 取高 16 位、SHRD 取低 16 位。
func (c *CPU) doubleShift16(left bool, dst, src uint16, n uint8) uint16 {
	var res, cf uint32
	if left {
		cat := uint32(dst)<<16 | uint32(src)
		res = (cat << n) >> 16
		cf = (cat >> (32 - n)) & 1
	} else {
		cat := uint32(src)<<16 | uint32(dst)
		res = (cat >> n) & 0xFFFF
		cf = (cat >> (n - 1)) & 1
	}
	r := uint16(res)
	c.setFlag(CF, cf != 0)
	c.setFlag(OF, (r^dst)&0x8000 != 0)
	c.setFlag(AF, false)
	c.setSZP(r, 16)
	return r
}

// mul32 是 MUL r/m32：EDX:EAX ＝ EAX × v（無號）。CF＝OF＝高半非零。
func (c *CPU) mul32(v uint32) {
	p := uint64(c.reg32(AX)) * uint64(v)
	c.setReg32(AX, uint32(p))
	c.setReg32(DX, uint32(p>>32))
	hi := p>>32 != 0
	c.setFlag(CF, hi)
	c.setFlag(OF, hi)
}

// imul32One 是 IMUL r/m32（單運算元）：EDX:EAX ＝ EAX × v（有號）。
// CF＝OF＝完整乘積不等於低 32 位元的符號延伸。
func (c *CPU) imul32One(v uint32) {
	p := int64(int32(c.reg32(AX))) * int64(int32(v))
	c.setReg32(AX, uint32(p))
	c.setReg32(DX, uint32(uint64(p)>>32))
	over := p != int64(int32(p))
	c.setFlag(CF, over)
	c.setFlag(OF, over)
}

// imul32Two 是 IMUL r32, r/m32[, imm]（0F AF、69、6B）：回傳低 32 位元。
func (c *CPU) imul32Two(a, b uint32) uint32 {
	p := int64(int32(a)) * int64(int32(b))
	over := p != int64(int32(p))
	c.setFlag(CF, over)
	c.setFlag(OF, over)
	return uint32(p)
}

// div32 是 DIV r/m32：EDX:EAX ÷ v（無號）。回傳 false 表示 #DE（除以 0 或商溢位），
// 此時暫存器不動。旗標不動（未定義）。
func (c *CPU) div32(v uint32) bool {
	if v == 0 {
		return false
	}
	num := uint64(c.reg32(DX))<<32 | uint64(c.reg32(AX))
	q := num / uint64(v)
	if q > 0xFFFFFFFF {
		return false
	}
	c.setReg32(AX, uint32(q))
	c.setReg32(DX, uint32(num%uint64(v)))
	return true
}

// idiv32 是 IDIV r/m32（有號）。商要放得進 32 位元有號數，否則 #DE。
// DOSBox-X 同樣允許商恰為 −2³¹（`IDIVD`）。
func (c *CPU) idiv32(v uint32) bool {
	d := int64(int32(v))
	if d == 0 {
		return false
	}
	num := int64(uint64(c.reg32(DX))<<32 | uint64(c.reg32(AX)))
	q := num / d // MinInt64 / −1 在 Go 回 MinInt64（不 panic），下面的範圍檢查會擋下
	r := num % d
	if q != int64(int32(q)) {
		return false
	}
	c.setReg32(AX, uint32(q))
	c.setReg32(DX, uint32(r))
	return true
}

// divideFault 是 32 位元除法的 #DE。
//
// **386 的 #DE 是 fault**：推進堆疊的返回位址是這道指令的**起點**（含前綴），
// 不是 8086 的「下一道」。16 位元除法仍走 `divideError`（語料驗證的 8086
// 行為，`docs/spec/002` §4 第 3 點）；這裡只管新的 32 位元路徑。
func (c *CPU) divideFault() {
	if c.ivtGuard(0, "#DE（32 位元除法）") { // 433 §2
		return
	}
	if len(c.DivErrors) < 32 {
		c.DivErrors = append(c.DivErrors, DivError{CS: c.opCS, IP: c.opIP})
	}
	c.Seg[CS], c.IP = c.opCS, c.opIP
	c.Interrupt(0)
}
