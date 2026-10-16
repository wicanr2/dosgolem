package cpu

import (
	"math/big"
	"math/rand"
	"strings"
	"testing"
)

// 80386 實模式子集的釘死測試（`docs/spec/197-cpu386-real-mode-subset`）。
//
// **沒有硬體語料可驗**（SingleStepTests 只有 8088／186 這一批），所以這裡的
// 預期值來自三個彼此獨立的來源：
//
//  1. 手算的向量（定址、堆疊、字串指令、跳躍）；
//  2. 與被測程式碼**不同算法**的參考實作（逐位元模擬移位、math/big 算乘除、
//     有號範圍檢查算溢位），用邊界值全交叉加隨機值驗旗標；
//  3. DOSBox-X 原始碼的對應段落（註解裡標出檔名與行號）。
//
// 能下的結論因此是「強推論」，不是「已證實」。

// shared386 回一顆共用的 CPU（已 reset）。**只給不碰記憶體的暫存器測試用**：
// 每次 `new386()` 配 1 MB 匯流排，旗標的交叉測試跑幾萬次就是幾十 GB 的配置。
var sharedCPU *CPU

func shared386() *CPU {
	if sharedCPU == nil {
		sharedCPU, _ = new386()
	}
	sharedCPU.Seg[CS], sharedCPU.Seg[DS], sharedCPU.Seg[SS] = 0x1000, 0x1000, 0x1000
	reset386(sharedCPU)
	return sharedCPU
}

// reset386 讓 c 回到乾淨的 Model80386 狀態，IP 歸 0，旗標清空。
func reset386(c *CPU) {
	c.R = [8]uint16{}
	c.Hi = [8]uint16{}
	c.FSeg, c.GSeg = 0, 0
	c.IP = 0
	c.SetFlags(0)
	c.R[SP] = 0xF000
}

func bits32(c *CPU) (cf, pf, af, zf, sf, of bool) {
	return c.Flag(CF), c.Flag(PF), c.Flag(AF), c.Flag(ZF), c.Flag(SF), c.Flag(OF)
}

type refFlags struct{ cf, pf, af, zf, sf, of bool }

func refPF(v uint32) bool {
	n := 0
	for b := uint8(v); b != 0; b &= b - 1 {
		n++
	}
	return n%2 == 0
}

func refSZP(v uint32) (zf, sf, pf bool) { return v == 0, v&0x80000000 != 0, refPF(v) }

// refAdd 用 math/big 與有號範圍檢查算 ADD／ADC，與實作的位元技巧無關。
func refAdd(a, b, carry uint32) (uint32, refFlags) {
	sum := new(big.Int).SetUint64(uint64(a))
	sum.Add(sum, new(big.Int).SetUint64(uint64(b)))
	sum.Add(sum, big.NewInt(int64(carry)))
	res := uint32(new(big.Int).And(sum, big.NewInt(0xFFFFFFFF)).Uint64())
	var f refFlags
	f.cf = sum.Cmp(big.NewInt(0xFFFFFFFF)) > 0
	s := int64(int32(a)) + int64(int32(b)) + int64(carry)
	f.of = s != int64(int32(s))
	f.af = (a&0xF)+(b&0xF)+carry > 0xF
	f.zf, f.sf, f.pf = refSZP(res)
	return res, f
}

// refSub 算 SUB／SBB／CMP。
func refSub(a, b, borrow uint32) (uint32, refFlags) {
	d := new(big.Int).SetUint64(uint64(a))
	d.Sub(d, new(big.Int).SetUint64(uint64(b)))
	d.Sub(d, big.NewInt(int64(borrow)))
	res := uint32(new(big.Int).And(d, big.NewInt(0xFFFFFFFF)).Uint64())
	var f refFlags
	f.cf = d.Sign() < 0
	s := int64(int32(a)) - int64(int32(b)) - int64(borrow)
	f.of = s != int64(int32(s))
	f.af = int(a&0xF)-int(b&0xF)-int(borrow) < 0
	f.zf, f.sf, f.pf = refSZP(res)
	return res, f
}

var edge32 = []uint32{0, 1, 2, 0xF, 0x10, 0x7F, 0x80, 0xFF, 0x100, 0x7FFF, 0x8000, 0xFFFF, 0x10000,
	0x7FFFFFFE, 0x7FFFFFFF, 0x80000000, 0x80000001, 0xFFFFFFFE, 0xFFFFFFFF}

func operands(rng *rand.Rand, n int) [][2]uint32 {
	var out [][2]uint32
	for _, a := range edge32 {
		for _, b := range edge32 {
			out = append(out, [2]uint32{a, b})
		}
	}
	for i := 0; i < n; i++ {
		out = append(out, [2]uint32{rng.Uint32(), rng.Uint32()})
	}
	return out
}

// TestALU32AgainstReference 對八個 ALU 運算的 32 位元形式逐一比對結果與六個旗標。
func TestALU32AgainstReference(t *testing.T) {
	rng := rand.New(rand.NewSource(386))
	// 66 modrm D8：reg＝EBX、r/m＝EAX，即 `op eax, ebx`。
	ops := []struct {
		name  string
		code  byte
		carry bool                                                // 要不要把進位也交叉
		ref   func(a, b uint32, cf bool) (uint32, refFlags, bool) // 回 (結果, 旗標, 是否寫回)
	}{
		{"ADD", 0x01, false, func(a, b uint32, _ bool) (uint32, refFlags, bool) { r, f := refAdd(a, b, 0); return r, f, true }},
		{"ADC", 0x11, true, func(a, b uint32, cf bool) (uint32, refFlags, bool) { r, f := refAdd(a, b, b2u(cf)); return r, f, true }},
		{"SUB", 0x29, false, func(a, b uint32, _ bool) (uint32, refFlags, bool) { r, f := refSub(a, b, 0); return r, f, true }},
		{"SBB", 0x19, true, func(a, b uint32, cf bool) (uint32, refFlags, bool) { r, f := refSub(a, b, b2u(cf)); return r, f, true }},
		{"CMP", 0x39, false, func(a, b uint32, _ bool) (uint32, refFlags, bool) { _, f := refSub(a, b, 0); return a, f, false }},
		{"AND", 0x21, false, func(a, b uint32, _ bool) (uint32, refFlags, bool) { return logicRef(a & b) }},
		{"OR", 0x09, false, func(a, b uint32, _ bool) (uint32, refFlags, bool) { return logicRef(a | b) }},
		{"XOR", 0x31, false, func(a, b uint32, _ bool) (uint32, refFlags, bool) { return logicRef(a ^ b) }},
		{"TEST", 0x85, false, func(a, b uint32, _ bool) (uint32, refFlags, bool) { _, f, _ := logicRef(a & b); return a, f, false }},
	}
	for _, op := range ops {
		for _, pair := range operands(rng, 400) {
			for _, cfIn := range []bool{false, true} {
				if cfIn && !op.carry {
					continue
				}
				c := shared386()
				c.setReg32(AX, pair[0])
				c.setReg32(BX, pair[1])
				c.setFlag(CF, cfIn)
				run386(t, c, []byte{0x66, op.code, 0xD8})
				want, wf, store := op.ref(pair[0], pair[1], cfIn)
				if got := c.reg32(AX); got != want {
					t.Fatalf("%s %08X,%08X cf=%v：EAX＝%08X，預期 %08X", op.name, pair[0], pair[1], cfIn, got, want)
				}
				cf, pf, af, zf, sf, of := bits32(c)
				// AF 在邏輯運算是我們挑的 0（Intel 未定義）；ref 也回 0。
				got := refFlags{cf, pf, af, zf, sf, of}
				if got != wf {
					t.Fatalf("%s %08X,%08X cf=%v：旗標 %+v，預期 %+v", op.name, pair[0], pair[1], cfIn, got, wf)
				}
				_ = store
			}
		}
	}
}

func b2u(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

func logicRef(r uint32) (uint32, refFlags, bool) {
	var f refFlags
	f.zf, f.sf, f.pf = refSZP(r)
	return r, f, true
}

// TestIncDecNeg32 釘住 INC／DEC 不動 CF，與 NEG 的 CF／OF／AF。
func TestIncDecNeg32(t *testing.T) {
	for _, v := range edge32 {
		for _, cfIn := range []bool{false, true} {
			c := shared386()
			c.setReg32(AX, v)
			c.setFlag(CF, cfIn)
			run386(t, c, []byte{0x66, 0x40}) // inc eax
			want, wf := refAdd(v, 1, 0)
			wf.cf = cfIn // INC 保留 CF
			if got := c.reg32(AX); got != want {
				t.Fatalf("INC %08X：%08X，預期 %08X", v, got, want)
			}
			if cf, pf, af, zf, sf, of := bits32(c); (refFlags{cf, pf, af, zf, sf, of}) != wf {
				t.Fatalf("INC %08X cf=%v：旗標不符 %+v 預期 %+v", v, cfIn, refFlags{cf, pf, af, zf, sf, of}, wf)
			}

			c = shared386()
			c.setReg32(AX, v)
			c.setFlag(CF, cfIn)
			run386(t, c, []byte{0x66, 0x48}) // dec eax
			want, wf = refSub(v, 1, 0)
			wf.cf = cfIn
			if got := c.reg32(AX); got != want {
				t.Fatalf("DEC %08X：%08X，預期 %08X", v, got, want)
			}
			if cf, pf, af, zf, sf, of := bits32(c); (refFlags{cf, pf, af, zf, sf, of}) != wf {
				t.Fatalf("DEC %08X cf=%v：旗標不符 %+v 預期 %+v", v, cfIn, refFlags{cf, pf, af, zf, sf, of}, wf)
			}

			c = shared386()
			c.setReg32(AX, v)
			run386(t, c, []byte{0x66, 0xF7, 0xD8}) // neg eax
			want, wf = refSub(0, v, 0)
			if got := c.reg32(AX); got != want {
				t.Fatalf("NEG %08X：%08X，預期 %08X", v, got, want)
			}
			if cf, pf, af, zf, sf, of := bits32(c); (refFlags{cf, pf, af, zf, sf, of}) != wf {
				t.Fatalf("NEG %08X：旗標 %+v 預期 %+v", v, refFlags{cf, pf, af, zf, sf, of}, wf)
			}
		}
	}
	// NOT 不動任何旗標。
	c, _ := new386()
	reset386(c)
	c.setReg32(AX, 0x0F0F0F0F)
	c.SetFlags(CF | ZF | OF)
	before := c.Flags
	run386(t, c, []byte{0x66, 0xF7, 0xD0})
	if c.reg32(AX) != 0xF0F0F0F0 || c.Flags != before {
		t.Errorf("NOT：EAX＝%08X flags %04X→%04X", c.reg32(AX), before, c.Flags)
	}
}

// refShift 用「一次移一位」的逐位元模擬算移位與旋轉，與實作的封閉式無關。
// OF 的值是 spec 197 §4 挑的確定值（Intel 只在計數 1 時定義）。
func refShift(op int, v uint32, n int, cfIn bool) (res uint32, f refFlags) {
	cf := cfIn
	orig := v
	for i := 0; i < n; i++ {
		switch op {
		case 0: // ROL
			top := v >> 31
			v = v<<1 | top
			cf = top != 0
		case 1: // ROR
			low := v & 1
			v = v>>1 | low<<31
			cf = low != 0
		case 2: // RCL
			top := v >> 31
			v = v<<1 | b2u(cf)
			cf = top != 0
		case 3: // RCR
			low := v & 1
			v = v>>1 | b2u(cf)<<31
			cf = low != 0
		case 4, 6: // SHL
			cf = v>>31 != 0
			v <<= 1
		case 5: // SHR
			cf = v&1 != 0
			v >>= 1
		case 7: // SAR
			cf = v&1 != 0
			v = uint32(int32(v) >> 1)
		}
	}
	f.cf = cf
	msb := v&0x80000000 != 0
	switch op {
	case 0, 2:
		f.of = cf != msb
	case 1, 3:
		f.of = msb != (v&0x40000000 != 0)
	case 4, 6:
		f.of = msb != (orig&0x80000000 != 0)
	case 5:
		f.of = n == 1 && orig&0x80000000 != 0
	}
	if op >= 4 {
		f.zf, f.sf, f.pf = refSZP(v)
	}
	return v, f
}

// TestShift32AgainstReference 對 C1／D1／D3 的八個子運算、計數 1–31 逐一比對。
func TestShift32AgainstReference(t *testing.T) {
	rng := rand.New(rand.NewSource(32))
	values := append([]uint32(nil), edge32...)
	for i := 0; i < 40; i++ {
		values = append(values, rng.Uint32())
	}
	for op := 0; op < 8; op++ {
		for _, v := range values {
			for n := 1; n <= 31; n++ {
				for _, cfIn := range []bool{false, true} {
					c := shared386()
					c.setReg32(AX, v)
					c.setFlag(CF, cfIn)
					// 保留值：旋轉不能動 SF／ZF／PF／AF。
					c.SetFlags(c.Flags | SF | PF | AF)
					before := c.Flags
					run386(t, c, []byte{0x66, 0xC1, byte(0xC0 | op<<3), byte(n)})
					want, wf := refShift(op, v, n, cfIn)
					if got := c.reg32(AX); got != want {
						t.Fatalf("op/%d %08X<<%d cf=%v：%08X，預期 %08X", op, v, n, cfIn, got, want)
					}
					if c.Flag(CF) != wf.cf || c.Flag(OF) != wf.of {
						t.Fatalf("op/%d %08X n=%d cf=%v：CF=%v OF=%v，預期 CF=%v OF=%v",
							op, v, n, cfIn, c.Flag(CF), c.Flag(OF), wf.cf, wf.of)
					}
					if op >= 4 {
						if c.Flag(ZF) != wf.zf || c.Flag(SF) != wf.sf || c.Flag(PF) != wf.pf || c.Flag(AF) {
							t.Fatalf("op/%d %08X n=%d：SZP／AF 不符", op, v, n)
						}
					} else {
						// 旋轉只動 CF／OF。
						if (c.Flags^before)&^(CF|OF) != 0 {
							t.Fatalf("op/%d %08X n=%d：旋轉動到了 CF／OF 以外的旗標 %04X→%04X", op, v, n, before, c.Flags)
						}
					}
				}
			}
		}
	}
}

// 計數遮成 5 位元：32 等於 0（運算元與旗標都不動）、33 等於 1。
// 386 行為；8086 的 CL 不遮罩（`docs/spec/002` §3.2）所以只套在 32 位元形式。
func TestShift32CountMask(t *testing.T) {
	c, _ := new386()
	reset386(c)
	c.setReg32(AX, 0x12345678)
	c.R[CX] = 32
	c.SetFlags(CF | ZF)
	before := c.Flags
	run386(t, c, []byte{0x66, 0xD3, 0xE0}) // shl eax, cl
	if c.reg32(AX) != 0x12345678 || c.Flags != before {
		t.Errorf("計數 32：EAX＝%08X flags %04X→%04X，預期完全不動", c.reg32(AX), before, c.Flags)
	}
	c.R[CX] = 33
	run386(t, c, []byte{0x66, 0xD3, 0xE0})
	if c.reg32(AX) != 0x2468ACF0 {
		t.Errorf("計數 33 ≡ 1：EAX＝%08X，預期 2468ACF0", c.reg32(AX))
	}
	// D1：計數 1。
	c.setReg32(AX, 0x80000001)
	c.R[CX] = 0
	run386(t, c, []byte{0x66, 0xD1, 0xE8}) // shr eax,1
	if c.reg32(AX) != 0x40000000 || !c.Flag(CF) || !c.Flag(OF) {
		t.Errorf("SHR 1：EAX＝%08X CF=%v OF=%v", c.reg32(AX), c.Flag(CF), c.Flag(OF))
	}
}

// TestMulDiv32 用 math/big 驗 MUL／IMUL／DIV／IDIV 的結果、CF／OF 與 #DE。
func TestMulDiv32(t *testing.T) {
	rng := rand.New(rand.NewSource(64))
	vals := append([]uint32(nil), edge32...)
	for i := 0; i < 60; i++ {
		vals = append(vals, rng.Uint32())
	}
	for _, a := range vals {
		for _, b := range vals {
			// MUL ebx
			c := shared386()
			c.setReg32(AX, a)
			c.setReg32(BX, b)
			run386(t, c, []byte{0x66, 0xF7, 0xE3})
			p := new(big.Int).Mul(new(big.Int).SetUint64(uint64(a)), new(big.Int).SetUint64(uint64(b)))
			lo := uint32(new(big.Int).And(p, big.NewInt(0xFFFFFFFF)).Uint64())
			hi := uint32(new(big.Int).Rsh(p, 32).Uint64())
			if c.reg32(AX) != lo || c.reg32(DX) != hi {
				t.Fatalf("MUL %08X*%08X：EDX:EAX＝%08X:%08X，預期 %08X:%08X", a, b, c.reg32(DX), c.reg32(AX), hi, lo)
			}
			if c.Flag(CF) != (hi != 0) || c.Flag(OF) != (hi != 0) {
				t.Fatalf("MUL %08X*%08X：CF／OF 要等於高半非零", a, b)
			}

			// IMUL ebx（單運算元）
			c = shared386()
			c.setReg32(AX, a)
			c.setReg32(BX, b)
			run386(t, c, []byte{0x66, 0xF7, 0xEB})
			sp := new(big.Int).Mul(big.NewInt(int64(int32(a))), big.NewInt(int64(int32(b))))
			slo := uint32(new(big.Int).And(sp, big.NewInt(0xFFFFFFFF)).Uint64())
			shi := uint32(new(big.Int).And(new(big.Int).Rsh(sp, 32), big.NewInt(0xFFFFFFFF)).Uint64())
			if c.reg32(AX) != slo || c.reg32(DX) != shi {
				t.Fatalf("IMUL %08X*%08X：EDX:EAX＝%08X:%08X，預期 %08X:%08X", a, b, c.reg32(DX), c.reg32(AX), shi, slo)
			}
			fits := sp.IsInt64() && sp.Int64() == int64(int32(slo))
			if c.Flag(CF) != !fits || c.Flag(OF) != !fits {
				t.Fatalf("IMUL %08X*%08X：CF／OF 應為 %v", a, b, !fits)
			}

			// 0F AF：imul eax, ebx（雙運算元）
			c = shared386()
			c.setReg32(AX, a)
			c.setReg32(BX, b)
			run386(t, c, []byte{0x66, 0x0F, 0xAF, 0xC3})
			if c.reg32(AX) != slo || c.Flag(CF) != !fits {
				t.Fatalf("IMUL2 %08X*%08X：EAX＝%08X CF=%v", a, b, c.reg32(AX), c.Flag(CF))
			}

			// DIV／IDIV：被除數用 (a:b) 當 EDX:EAX。
			num := new(big.Int).Lsh(new(big.Int).SetUint64(uint64(a)), 32)
			num.Or(num, new(big.Int).SetUint64(uint64(b)))
			for _, d := range []uint32{1, 2, 3, 0xFFFF, 0x80000000, 0xFFFFFFFF, rng.Uint32() | 1} {
				c = shared386()
				c.setReg32(DX, a)
				c.setReg32(AX, b)
				c.setReg32(CX, d)
				ok := c.div32(d)
				q, r := new(big.Int).QuoRem(num, new(big.Int).SetUint64(uint64(d)), new(big.Int))
				wantOK := q.BitLen() <= 32
				if ok != wantOK {
					t.Fatalf("DIV %08X%08X / %08X：成功＝%v，預期 %v", a, b, d, ok, wantOK)
				}
				if ok && (uint64(c.reg32(AX)) != q.Uint64() || uint64(c.reg32(DX)) != r.Uint64()) {
					t.Fatalf("DIV %08X%08X / %08X：商餘 %08X %08X，預期 %v %v", a, b, d, c.reg32(AX), c.reg32(DX), q, r)
				}
				if !ok && (c.reg32(AX) != b || c.reg32(DX) != a) {
					t.Fatalf("DIV 溢位時暫存器要不動")
				}

				c = shared386()
				c.setReg32(DX, a)
				c.setReg32(AX, b)
				snum := new(big.Int).Lsh(big.NewInt(int64(int32(a))), 32)
				snum.Add(snum, new(big.Int).SetUint64(uint64(b)))
				sd := big.NewInt(int64(int32(d)))
				ok = c.idiv32(d)
				sq, sr := new(big.Int).QuoRem(snum, sd, new(big.Int)) // Go 的 QuoRem 是截斷除法，與 IDIV 同
				wantOK = sq.IsInt64() && sq.Int64() == int64(int32(sq.Int64()))
				if ok != wantOK {
					t.Fatalf("IDIV %08X%08X / %08X：成功＝%v，預期 %v（商 %v）", a, b, d, ok, wantOK, sq)
				}
				if ok && (int64(int32(c.reg32(AX))) != sq.Int64() || int64(int32(c.reg32(DX))) != sr.Int64()) {
					t.Fatalf("IDIV %08X%08X / %08X：商餘不符", a, b, d)
				}
			}
		}
	}
}

// 除以 0 與商溢位走 INT 0，**返回位址是這道指令的起點**（386 的 #DE 是 fault，
// 與 8086 的「下一道」不同；`docs/spec/197` §4）。
func TestDivideFault32(t *testing.T) {
	c, b := new386()
	reset386(c)
	c.Seg[CS] = 0x2000
	c.Seg[SS] = 0x3000
	c.R[SP] = 0x100
	// IVT[0] ＝ 5000:0010
	b.Write8(0, 0x10)
	b.Write8(2, 0x00)
	b.Write8(3, 0x50)
	c.setReg32(AX, 1)
	c.setReg32(BX, 0)
	c.IP = 0x40
	code := []byte{0x66, 0xF7, 0xF3} // div ebx
	for i, v := range code {
		b.Write8(Addr(0x2000, 0x40)+uint32(i), v)
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.Seg[CS] != 0x5000 || c.IP != 0x10 {
		t.Fatalf("沒進 INT 0：%04X:%04X", c.Seg[CS], c.IP)
	}
	if got := busRead16(b, Linear(0x3000, 0x100-6)); got != 0x40 {
		t.Errorf("推進堆疊的返回 IP ＝ %04X，預期指令起點 0040", got)
	}
	if len(c.DivErrors) != 1 || c.DivErrors[0].IP != 0x40 {
		t.Errorf("DivErrors ＝ %+v", c.DivErrors)
	}
}

// ---- 定址：ModRM32／SIB ---------------------------------------------------

func TestModRM32Addressing(t *testing.T) {
	// 每列：位元組（0x67 前綴之後的 mov eax,[…] 的 ModRM／SIB／位移）、暫存器值、
	// 預期的 (段, 偏移)。預設段規則照 DOSBox-X `table_ea.h`：基底是 ESP／EBP 用 SS。
	type reg struct {
		r int
		v uint32
	}
	tests := []struct {
		name string
		code []byte // 8B 之後
		regs []reg
		seg  uint16 // 預期段值
		off  uint16
	}{
		{"[eax]", []byte{0x18}, []reg{{AX, 0x120}}, 0x1000, 0x120}, // mov ebx,[eax]：modrm 18＝mod0 reg3 rm0
		{"[ecx+disp8]", []byte{0x59, 0x08}, []reg{{CX, 0x200}}, 0x1000, 0x208},
		{"[ecx-disp8]", []byte{0x59, 0xF8}, []reg{{CX, 0x200}}, 0x1000, 0x1F8},
		{"[disp32]", []byte{0x1D, 0x34, 0x12, 0x00, 0x00}, nil, 0x1000, 0x1234},
		{"[esi+disp32]", []byte{0x9E, 0x00, 0x01, 0x00, 0x00}, []reg{{SI, 0x10}}, 0x1000, 0x110},
		{"[ebp+disp8] 預設 SS", []byte{0x5D, 0x04}, []reg{{BP, 0x300}}, 0x3000, 0x304},
		{"[esp] 經 SIB 預設 SS", []byte{0x1C, 0x24}, []reg{{SP, 0x400}}, 0x3000, 0x400},
		{"[esp+disp8] 預設 SS", []byte{0x5C, 0x24, 0x08}, []reg{{SP, 0x400}}, 0x3000, 0x408},
		{"[eax+ecx*4]", []byte{0x1C, 0x88}, []reg{{AX, 0x100}, {CX, 0x10}}, 0x1000, 0x140},
		{"[eax+edx*2+disp8]", []byte{0x5C, 0x50, 0x06}, []reg{{AX, 0x100}, {DX, 3}}, 0x1000, 0x10C},
		{"[edx*8] 無基底（SIB base＝5, mod＝0）", []byte{0x1C, 0xD5, 0x00, 0x01, 0x00, 0x00}, []reg{{DX, 2}}, 0x1000, 0x110},
		{"[eax+ebp*2] 預設仍是 DS（看基底不看索引）", []byte{0x1C, 0x68}, []reg{{AX, 0x100}, {BP, 0x20}}, 0x1000, 0x140},
		{"[ebp+eax*2+disp8] 基底是 EBP → SS", []byte{0x5C, 0x45, 0x00}, []reg{{BP, 0x100}, {AX, 0x20}}, 0x3000, 0x140},
		{"SIB index＝4 是沒有索引", []byte{0x1C, 0x20}, []reg{{AX, 0x150}, {SP, 0x9999}}, 0x1000, 0x150},
	}
	for _, tc := range tests {
		c, b := new386()
		reset386(c)
		c.Seg[DS], c.Seg[SS] = 0x1000, 0x3000
		c.R[SP] = 0xF000
		for _, r := range tc.regs {
			c.setReg32(r.r, r.v)
		}
		// 在預期的位址放一個記號 dword，確認讀到的是那裡。
		lin := Addr(tc.seg, tc.off)
		for i, v := range []byte{0xEF, 0xBE, 0xAD, 0xDE} {
			b.Write8(lin+uint32(i), v)
		}
		code := append([]byte{0x67, 0x66, 0x8B}, tc.code...) // mov ebx,[…]
		run386(t, c, code)
		if got := c.reg32(BX); got != 0xDEADBEEF {
			t.Errorf("%s：讀到 %08X，預期 DEADBEEF（有效位址 %04X:%04X）", tc.name, got, tc.seg, tc.off)
		}
	}
}

// 0x64／0x65 是 FS／GS 段覆寫。
func TestFSGSOverride(t *testing.T) {
	c, b := new386()
	reset386(c)
	c.FSeg, c.GSeg = 0x4000, 0x5000
	c.Seg[DS] = 0x1000
	b.Write8(Addr(0x4000, 0x10), 0x11)
	b.Write8(Addr(0x5000, 0x10), 0x22)
	b.Write8(Addr(0x1000, 0x10), 0x33)
	c.R[BX] = 0x10
	run386(t, c, []byte{0x64, 0x8A, 0x07}) // mov al, fs:[bx]
	if uint8(c.R[AX]) != 0x11 {
		t.Errorf("FS：AL＝%02X", uint8(c.R[AX]))
	}
	c.IP = 0
	run386(t, c, []byte{0x65, 0x8A, 0x07})
	if uint8(c.R[AX]) != 0x22 {
		t.Errorf("GS：AL＝%02X", uint8(c.R[AX]))
	}
	c.IP = 0
	run386(t, c, []byte{0x8A, 0x07})
	if uint8(c.R[AX]) != 0x33 {
		t.Errorf("無前綴：AL＝%02X", uint8(c.R[AX]))
	}
	// 0F A0／A1／A8／A9 與 mov r/m,sreg／mov sreg,r/m 的 /4 /5。
	c, _ = new386()
	reset386(c)
	c.FSeg = 0xABCD
	run386(t, c, []byte{0x0F, 0xA0}) // push fs
	c.FSeg = 0
	c.IP = 0
	run386(t, c, []byte{0x0F, 0xA1}) // pop fs
	if c.FSeg != 0xABCD {
		t.Errorf("push／pop fs：FS＝%04X", c.FSeg)
	}
	c.R[AX] = 0x1234
	c.IP = 0
	run386(t, c, []byte{0x8E, 0xE8}) // mov gs, ax（/5）
	if c.GSeg != 0x1234 {
		t.Errorf("mov gs,ax：GS＝%04X", c.GSeg)
	}
	c.IP = 0
	run386(t, c, []byte{0x8C, 0xE1}) // mov cx, fs（/4）
	if c.R[CX] != 0xABCD {
		t.Errorf("mov cx,fs：CX＝%04X", c.R[CX])
	}
}

// 有效位址超過實模式的 64 KB 段限，在真機是 #GP；這裡失敗即關閉（不截斷）。
// LEA 不碰記憶體，不受限。
func TestModRM32LimitAndLEA(t *testing.T) {
	c, _ := new386()
	reset386(c)
	c.setReg32(AX, 0x12345)
	err := stepWith(c, []byte{0x67, 0x8B, 0x18}) // mov bx,[eax]
	if err == nil || !strings.Contains(err.Error(), "64 KB") {
		t.Fatalf("EA＝12345 應報超出段限，得到 %v", err)
	}

	c, _ = new386()
	reset386(c)
	c.setReg32(AX, 0x12345)
	c.setReg32(CX, 0x10)
	run386(t, c, []byte{0x67, 0x66, 0x8D, 0x1C, 0x88}) // lea ebx,[eax+ecx*4]
	if c.reg32(BX) != 0x12385 {
		t.Errorf("lea ebx,[eax+ecx*4]＝%08X，預期 00012385", c.reg32(BX))
	}
	c.IP = 0
	run386(t, c, []byte{0x67, 0x8D, 0x1C, 0x88}) // lea bx,[eax+ecx*4]（16 位元目的地取低半）
	if c.R[BX] != 0x2385 {
		t.Errorf("lea bx,[…]＝%04X，預期 2385", c.R[BX])
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x8D, 0x5F, 0x10}) // lea ebx,[bx+0x10]（16 位元位址、32 位元目的地：零延伸）
	// BX 上一步＝2385，所以 [bx+0x10] ＝ 2395
	if c.reg32(BX) != 0x2395 {
		t.Errorf("lea ebx,[bx+10h]＝%08X", c.reg32(BX))
	}
}

// stepWith 在 CS:IP 寫入 code 並跑一步，回傳 Step 的錯誤。
func stepWith(c *CPU, code []byte) error {
	for i, v := range code {
		c.Bus.Write8(Addr(c.Seg[CS], c.IP)+uint32(i), v)
	}
	return c.Step()
}

// ---- 堆疊、移動、擴充、字串 ------------------------------------------------

func TestPushPop32(t *testing.T) {
	c, _ := new386()
	reset386(c)
	c.Seg[SS] = 0x3000
	c.setReg32(AX, 0xAABBCCDD)
	c.setReg32(DX, 0x11223344)
	sp := c.R[SP]
	run386(t, c, []byte{0x66, 0x50}) // push eax
	if c.R[SP] != sp-4 || busRead32(c.Bus, Linear(0x3000, c.R[SP])) != 0xAABBCCDD {
		t.Fatalf("push eax：SP＝%04X，堆疊頂＝%08X", c.R[SP], busRead32(c.Bus, Linear(0x3000, c.R[SP])))
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x5A}) // pop edx
	if c.reg32(DX) != 0xAABBCCDD || c.R[SP] != sp {
		t.Errorf("pop edx：EDX＝%08X SP＝%04X", c.reg32(DX), c.R[SP])
	}
	// push imm32 與 push imm8（符號延伸）。
	c.IP = 0
	run386(t, c, []byte{0x66, 0x68, 0x78, 0x56, 0x34, 0x12})
	if busRead32(c.Bus, Linear(0x3000, c.R[SP])) != 0x12345678 {
		t.Errorf("push imm32：%08X", busRead32(c.Bus, Linear(0x3000, c.R[SP])))
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x6A, 0xFE})
	if busRead32(c.Bus, Linear(0x3000, c.R[SP])) != 0xFFFFFFFE {
		t.Errorf("push imm8 −2：%08X", busRead32(c.Bus, Linear(0x3000, c.R[SP])))
	}
	// PUSH ESP 推減之前的值（286 以上）。
	c, _ = new386()
	reset386(c)
	c.Seg[SS] = 0x3000
	c.R[SP] = 0x1000
	c.Hi[SP] = 0xBEEF
	run386(t, c, []byte{0x66, 0x54})
	if busRead32(c.Bus, Linear(0x3000, 0x0FFC)) != 0xBEEF1000 {
		t.Errorf("push esp：推進 %08X，預期 BEEF1000", busRead32(c.Bus, Linear(0x3000, 0x0FFC)))
	}
}

func TestPushadPopad(t *testing.T) {
	c, _ := new386()
	reset386(c)
	c.Seg[SS] = 0x3000
	for i := 0; i < 8; i++ {
		c.setReg32(i, uint32(0x10000000*(i+1))+uint32(0x111*(i+1)))
	}
	c.R[SP] = 0x2000
	c.Hi[SP] = 0
	saved := c.R
	savedHi := c.Hi
	run386(t, c, []byte{0x66, 0x60}) // pushad
	if c.R[SP] != 0x2000-32 {
		t.Fatalf("pushad：SP＝%04X", c.R[SP])
	}
	// 推入順序 EAX ECX EDX EBX ESP(舊) EBP ESI EDI → 堆疊頂是 EDI。
	if got := busRead32(c.Bus, Linear(0x3000, c.R[SP])); got != c.reg32(DI) {
		t.Errorf("堆疊頂應是 EDI：%08X", got)
	}
	if got := busRead32(c.Bus, Linear(0x3000, c.R[SP]+12)); got != 0x2000 {
		t.Errorf("第五格應是舊 ESP（2000）：%08X", got)
	}
	for i := range c.R {
		c.R[i], c.Hi[i] = 0, 0
	}
	c.R[SP] = 0x2000 - 32
	c.IP = 0
	run386(t, c, []byte{0x66, 0x61}) // popad
	for i := 0; i < 8; i++ {
		if i == SP {
			continue
		}
		if c.R[i] != saved[i] || c.Hi[i] != savedHi[i] {
			t.Errorf("popad：reg%d＝%04X%04X，預期 %04X%04X", i, c.Hi[i], c.R[i], savedHi[i], saved[i])
		}
	}
	if c.R[SP] != 0x2000 {
		t.Errorf("popad 後 SP＝%04X，預期 2000（ESP 那格丟掉）", c.R[SP])
	}
}

// 偵測 386 的標準序列（Intel 的 AC 旗標測試）：EFLAGS 的 AC（bit18）在 386 上寫不進去，
// 所以結果 ＝ 0 ＝ 386。AESOP.EXE 的偵測常式 `124A:0139` 起走這一條路（探針報告 §4 停點 3）。
func TestEFLAGSACDetectionYields386(t *testing.T) {
	c, _ := new386()
	reset386(c)
	c.Seg[SS] = 0x3000
	code := []byte{
		0x66, 0x9C, // pushfd
		0x66, 0x58, // pop eax
		0x66, 0x8B, 0xC8, // mov ecx,eax
		0x66, 0x35, 0x00, 0x00, 0x04, 0x00, // xor eax,40000h
		0x66, 0x50, // push eax
		0x66, 0x9D, // popfd
		0x66, 0x9C, // pushfd
		0x66, 0x58, // pop eax
		0x66, 0x33, 0xC1, // xor eax,ecx
		0x66, 0xC1, 0xE8, 0x12, // shr eax,12h
		0x66, 0x83, 0xE0, 0x01, // and eax,1
		0x66, 0x51, // push ecx
		0x66, 0x9D, // popfd
	}
	for i, v := range code {
		c.Bus.Write8(Addr(c.Seg[CS], 0)+uint32(i), v)
	}
	for c.IP < uint16(len(code)) {
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if c.reg32(AX) != 0 {
		t.Errorf("AC 位元竟然寫得進去：EAX＝%08X（386 要得到 0）", c.reg32(AX))
	}
	if c.R[SP] != 0xF000 {
		t.Errorf("堆疊沒有平衡：SP＝%04X", c.R[SP])
	}
}

func TestPushfdPopfd(t *testing.T) {
	c, _ := new386()
	reset386(c)
	c.Seg[SS] = 0x3000
	c.SetFlags(CF | ZF | DF)
	run386(t, c, []byte{0x66, 0x9C})
	v := busRead32(c.Bus, Linear(0x3000, c.R[SP]))
	if v>>16 != 0 || uint16(v)&(CF|ZF|DF) != CF|ZF|DF || v&2 == 0 {
		t.Errorf("pushfd：%08X（高 16 位元要是 0、bit1 恆立）", v)
	}
	// popfd 的高半（含 AC、VM）丟掉，低 16 位元走 SetFlags。
	busWrite32(c.Bus, Linear(0x3000, c.R[SP]), 0xFFFC0000|uint32(SF|PF))
	c.IP = 0
	run386(t, c, []byte{0x66, 0x9D})
	if !c.Flag(SF) || !c.Flag(PF) || c.Flag(CF) {
		t.Errorf("popfd：flags＝%04X", c.Flags)
	}
}

func TestMov32Variants(t *testing.T) {
	c, b := new386()
	reset386(c)
	c.Seg[DS] = 0x1000
	// mov r32, imm32；mov r/m32, r32；mov r32, r/m32；xchg；cwde／cdq。
	run386(t, c, []byte{0x66, 0xB9, 0x78, 0x56, 0x34, 0x12}) // mov ecx,12345678h
	if c.reg32(CX) != 0x12345678 {
		t.Fatalf("mov ecx,imm32：%08X", c.reg32(CX))
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x89, 0x0E, 0x00, 0x02}) // mov [0200h],ecx
	if busRead32(c.Bus, Linear(0x1000, 0x200)) != 0x12345678 {
		t.Errorf("mov [m],ecx：%08X", busRead32(c.Bus, Linear(0x1000, 0x200)))
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x8B, 0x16, 0x00, 0x02}) // mov edx,[0200h]
	if c.reg32(DX) != 0x12345678 {
		t.Errorf("mov edx,[m]：%08X", c.reg32(DX))
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0xC7, 0x06, 0x04, 0x02, 0xEF, 0xBE, 0xAD, 0xDE}) // mov dword [0204h],imm32
	if busRead32(c.Bus, Linear(0x1000, 0x204)) != 0xDEADBEEF {
		t.Errorf("mov dword [m],imm32：%08X", busRead32(c.Bus, Linear(0x1000, 0x204)))
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x87, 0x0E, 0x04, 0x02}) // xchg [0204h],ecx
	if c.reg32(CX) != 0xDEADBEEF || busRead32(c.Bus, Linear(0x1000, 0x204)) != 0x12345678 {
		t.Errorf("xchg：ECX＝%08X [m]＝%08X", c.reg32(CX), busRead32(c.Bus, Linear(0x1000, 0x204)))
	}
	c.IP = 0
	c.setReg32(AX, 0x1234FFFE)
	run386(t, c, []byte{0x66, 0x98}) // cwde
	if c.reg32(AX) != 0xFFFFFFFE {
		t.Errorf("cwde：%08X", c.reg32(AX))
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x99}) // cdq
	if c.reg32(DX) != 0xFFFFFFFF {
		t.Errorf("cdq：EDX＝%08X", c.reg32(DX))
	}
	c.IP = 0
	c.setReg32(AX, 0x7FFFFFFF)
	run386(t, c, []byte{0x66, 0x99})
	if c.reg32(DX) != 0 {
		t.Errorf("cdq（正數）：EDX＝%08X", c.reg32(DX))
	}
	c.IP = 0
	c.setReg32(AX, 0x11111111)
	c.setReg32(BX, 0x22222222)
	run386(t, c, []byte{0x66, 0x93}) // xchg ebx,eax
	if c.reg32(AX) != 0x22222222 || c.reg32(BX) != 0x11111111 {
		t.Errorf("xchg ebx,eax")
	}
	// moffs：66 A1／A3，與 0x67 的 32 位元位移。
	c.IP = 0
	b.Write8(Addr(0x1000, 0x300), 0xAA)
	b.Write8(Addr(0x1000, 0x303), 0xBB)
	run386(t, c, []byte{0x66, 0xA1, 0x00, 0x03})
	if c.reg32(AX) != 0xBB0000AA {
		t.Errorf("mov eax,[0300h]：%08X", c.reg32(AX))
	}
	c.IP = 0
	run386(t, c, []byte{0x67, 0x66, 0xA1, 0x00, 0x03, 0x00, 0x00})
	if c.reg32(AX) != 0xBB0000AA {
		t.Errorf("mov eax,[00000300h]（67）：%08X", c.reg32(AX))
	}
	c.IP = 0
	if err := stepWith(c, []byte{0x67, 0x66, 0xA1, 0x00, 0x00, 0x01, 0x00}); err == nil {
		t.Error("moffs 位移 00010000 應該超出段限")
	}
	// 高半保留：16 位元寫入不清 Hi。
	c, _ = new386()
	reset386(c)
	c.setReg32(BX, 0xCAFE0000)
	run386(t, c, []byte{0xBB, 0x34, 0x12})
	if c.reg32(BX) != 0xCAFE1234 {
		t.Errorf("mov bx,1234h 不該清 EBX 高半：%08X", c.reg32(BX))
	}
}

func TestALUImmediate32(t *testing.T) {
	c, _ := new386()
	reset386(c)
	c.setReg32(AX, 0x7FFFFFFF)
	run386(t, c, []byte{0x66, 0x05, 0x01, 0x00, 0x00, 0x00}) // add eax,1
	if c.reg32(AX) != 0x80000000 || !c.Flag(OF) || !c.Flag(SF) || c.Flag(CF) {
		t.Errorf("add eax,1：EAX＝%08X flags %04X", c.reg32(AX), c.Flags)
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x83, 0xE8, 0x01}) // sub eax,1（83 /5 imm8）
	if c.reg32(AX) != 0x7FFFFFFF || !c.Flag(OF) {
		t.Errorf("sub eax,1：EAX＝%08X flags %04X", c.reg32(AX), c.Flags)
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x83, 0xC0, 0xFF}) // add eax,−1（符號延伸）
	if c.reg32(AX) != 0x7FFFFFFE {
		t.Errorf("add eax,imm8 −1：%08X", c.reg32(AX))
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x81, 0xF8, 0xFE, 0xFF, 0xFF, 0x7F}) // cmp eax,7FFFFFFEh
	if !c.Flag(ZF) {
		t.Errorf("cmp eax,imm32 應相等")
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x25, 0xFF, 0x00, 0xFF, 0x00}) // and eax,00FF00FFh
	if c.reg32(AX) != 0x00FF00FE {                           // 7FFFFFFE & 00FF00FF
		t.Errorf("and eax,imm32：%08X", c.reg32(AX))
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0xA9, 0x00, 0x00, 0x00, 0x80}) // test eax,80000000h
	if !c.Flag(ZF) {
		t.Errorf("test eax,imm32")
	}
}

func TestStringOps32(t *testing.T) {
	c, b := new386()
	reset386(c)
	c.Seg[DS], c.Seg[ES] = 0x1000, 0x2000
	for i := 0; i < 16; i++ {
		b.Write8(Addr(0x1000, 0x100)+uint32(i), uint8(0xA0+i))
	}
	c.R[SI], c.R[DI], c.R[CX] = 0x100, 0x200, 4
	run386(t, c, []byte{0x66, 0xF3, 0xA5}) // rep movsd
	for i := 0; i < 16; i++ {
		if got := b.Read8(Addr(0x2000, 0x200) + uint32(i)); got != uint8(0xA0+i) {
			t.Fatalf("rep movsd 第 %d 個位元組＝%02X", i, got)
		}
	}
	if c.R[SI] != 0x110 || c.R[DI] != 0x210 || c.R[CX] != 0 {
		t.Errorf("rep movsd 後 SI=%04X DI=%04X CX=%04X", c.R[SI], c.R[DI], c.R[CX])
	}
	// DF＝1：往回。
	c.IP = 0
	c.SetFlags(c.Flags | DF)
	c.R[SI], c.R[DI], c.R[CX] = 0x10C, 0x30C, 2
	run386(t, c, []byte{0x66, 0xF3, 0xA5})
	if c.R[SI] != 0x104 || c.R[DI] != 0x304 {
		t.Errorf("DF＝1 的 rep movsd：SI=%04X DI=%04X", c.R[SI], c.R[DI])
	}
	c.SetFlags(c.Flags &^ DF)
	// stosd／lodsd。
	c.IP = 0
	c.setReg32(AX, 0xCAFEBABE)
	c.R[DI], c.R[CX] = 0x400, 3
	run386(t, c, []byte{0x66, 0xF3, 0xAB})
	for i := 0; i < 3; i++ {
		if busRead32(c.Bus, Linear(0x2000, 0x400+uint16(4*i))) != 0xCAFEBABE {
			t.Errorf("stosd 第 %d 個", i)
		}
	}
	c.IP = 0
	c.setReg32(AX, 0)
	c.R[SI] = 0x400
	// lodsd 的來源是 DS:SI，我們把資料放在 ES；先搬到 DS 段。
	for i := 0; i < 4; i++ {
		b.Write8(Addr(0x1000, 0x400)+uint32(i), uint8(0x10*(i+1)))
	}
	run386(t, c, []byte{0x66, 0xAD})
	if c.reg32(AX) != 0x40302010 || c.R[SI] != 0x404 {
		t.Errorf("lodsd：EAX＝%08X SI＝%04X", c.reg32(AX), c.R[SI])
	}
	// repe cmpsd 在第一個不同的 dword 停。
	c.IP = 0
	c.Seg[ES] = 0x1000
	for i := 0; i < 12; i++ {
		b.Write8(Addr(0x1000, 0x500)+uint32(i), uint8(i))
		b.Write8(Addr(0x1000, 0x600)+uint32(i), uint8(i))
	}
	b.Write8(Addr(0x1000, 0x600+8), 0xFF) // 第三個 dword 不同
	c.R[SI], c.R[DI], c.R[CX] = 0x500, 0x600, 3
	run386(t, c, []byte{0x66, 0xF3, 0xA7})
	if c.R[CX] != 0 || c.Flag(ZF) || c.R[SI] != 0x50C {
		t.Errorf("repe cmpsd：CX=%d ZF=%v SI=%04X（預期掃完三個、最後不等）", c.R[CX], c.Flag(ZF), c.R[SI])
	}
	// repne scasd 找 EAX。
	c.IP = 0
	b.Write8(Addr(0x1000, 0x700), 0)
	for i := 0; i < 16; i++ {
		b.Write8(Addr(0x1000, 0x700)+uint32(i), 0)
	}
	b.Write8(Addr(0x1000, 0x708), 0x78)
	b.Write8(Addr(0x1000, 0x709), 0x56)
	b.Write8(Addr(0x1000, 0x70A), 0x34)
	b.Write8(Addr(0x1000, 0x70B), 0x12)
	c.setReg32(AX, 0x12345678)
	c.R[DI], c.R[CX] = 0x700, 4
	run386(t, c, []byte{0x66, 0xF2, 0xAF})
	if !c.Flag(ZF) || c.R[DI] != 0x70C || c.R[CX] != 1 {
		t.Errorf("repne scasd：ZF=%v DI=%04X CX=%d", c.Flag(ZF), c.R[DI], c.R[CX])
	}
}

// ---- 0F 雙位元組 ----------------------------------------------------------

func TestMovzxMovsx(t *testing.T) {
	c, b := new386()
	reset386(c)
	c.Seg[DS] = 0x1000
	// 偵測常式用的那一道：0F BE D0 ＝ movsx dx, al。
	c.R[AX] = 0x00FE
	c.R[DX] = 0x7777
	run386(t, c, []byte{0x0F, 0xBE, 0xD0})
	if c.R[DX] != 0xFFFE {
		t.Errorf("movsx dx,al(FE)＝%04X，預期 FFFE", c.R[DX])
	}
	c.IP = 0
	c.R[AX] = 0x007E
	run386(t, c, []byte{0x0F, 0xBE, 0xD0})
	if c.R[DX] != 0x007E {
		t.Errorf("movsx dx,al(7E)＝%04X", c.R[DX])
	}
	cases := []struct {
		name string
		code []byte
		src  uint32
		want uint32
	}{
		{"movzx eax,byte", []byte{0x66, 0x0F, 0xB6, 0x06, 0x00, 0x03}, 0xFF, 0x000000FF},
		{"movsx eax,byte", []byte{0x66, 0x0F, 0xBE, 0x06, 0x00, 0x03}, 0xFF, 0xFFFFFFFF},
		{"movzx eax,word", []byte{0x66, 0x0F, 0xB7, 0x06, 0x00, 0x03}, 0x8001, 0x00008001},
		{"movsx eax,word", []byte{0x66, 0x0F, 0xBF, 0x06, 0x00, 0x03}, 0x8001, 0xFFFF8001},
	}
	for _, tc := range cases {
		c.IP = 0
		c.setReg32(AX, 0xDEADBEEF)
		b.Write8(Addr(0x1000, 0x300), uint8(tc.src))
		b.Write8(Addr(0x1000, 0x301), uint8(tc.src>>8))
		run386(t, c, tc.code)
		if c.reg32(AX) != tc.want {
			t.Errorf("%s：%08X，預期 %08X", tc.name, c.reg32(AX), tc.want)
		}
	}
	// 16 位元目的地不動高半。
	c.IP = 0
	c.setReg32(AX, 0xDEAD0000)
	b.Write8(Addr(0x1000, 0x300), 0x81)
	run386(t, c, []byte{0x0F, 0xB6, 0x06, 0x00, 0x03}) // movzx ax,byte [300h]
	if c.reg32(AX) != 0xDEAD0081 {
		t.Errorf("movzx ax,byte 動到高半：%08X", c.reg32(AX))
	}
}

func TestJccNearAndSetcc(t *testing.T) {
	// 0F 82 CD 02 ＝ jb near +02CD（探針報告 §4 停點 4）。
	c, _ := new386()
	reset386(c)
	c.SetFlags(CF)
	run386(t, c, []byte{0x0F, 0x82, 0xCD, 0x02})
	if c.IP != 4+0x02CD {
		t.Errorf("jb near 成立：IP＝%04X，預期 %04X", c.IP, 4+0x02CD)
	}
	c.IP = 0
	c.SetFlags(0)
	run386(t, c, []byte{0x0F, 0x82, 0xCD, 0x02})
	if c.IP != 4 {
		t.Errorf("jb near 不成立：IP＝%04X", c.IP)
	}
	// 負位移。
	c.IP = 0x100
	c.SetFlags(ZF)
	run386(t, c, []byte{0x0F, 0x84, 0x00, 0xFF}) // je −0x100
	if c.IP != 0x104-0x100 {
		t.Errorf("je 負位移：IP＝%04X", c.IP)
	}
	// 66 0F 8x ＝ rel32。目標超過 64 KB 要失敗。
	c.IP = 0
	c.SetFlags(ZF)
	run386(t, c, []byte{0x66, 0x0F, 0x84, 0x10, 0x00, 0x00, 0x00})
	if c.IP != 7+0x10 {
		t.Errorf("66 je rel32：IP＝%04X", c.IP)
	}
	c.IP = 0
	if err := stepWith(c, []byte{0x66, 0x0F, 0x84, 0x00, 0x00, 0x01, 0x00}); err == nil {
		t.Error("rel32 跳到 64 KB 之外應該失敗")
	}
	// 十六個條件的 jcc／setcc 與 cond() 的對應：逐旗標組合驗。
	for cc := uint8(0); cc < 16; cc++ {
		for fl := uint16(0); fl < 64; fl++ {
			flags := uint16(0)
			if fl&1 != 0 {
				flags |= CF
			}
			if fl&2 != 0 {
				flags |= ZF
			}
			if fl&4 != 0 {
				flags |= SF
			}
			if fl&8 != 0 {
				flags |= OF
			}
			if fl&16 != 0 {
				flags |= PF
			}
			c := shared386()
			c.SetFlags(flags)
			c.Seg[DS] = 0x1000
			c.R[AX] = 0xFFFF
			run386(t, c, []byte{0x0F, 0x90 | cc, 0xC0}) // setcc al
			// 預期值由 Intel 的條件表另寫一份。
			want := refCond(cc, flags)
			if got := uint8(c.R[AX]) == 1; got != want || c.R[AX]>>8 != 0xFF {
				t.Fatalf("setcc %X flags=%04X：AL＝%02X AH=%02X，預期 %v", cc, flags, uint8(c.R[AX]), c.R[AX]>>8, want)
			}
		}
	}
}

// refCond 是 Intel 手冊的條件碼表，與 cond() 的 switch 分開寫。
func refCond(cc uint8, f uint16) bool {
	cf, zf, sf, of, pf := f&CF != 0, f&ZF != 0, f&SF != 0, f&OF != 0, f&PF != 0
	tbl := []bool{
		of, !of, cf, !cf, zf, !zf, cf || zf, !cf && !zf,
		sf, !sf, pf, !pf, sf != of, sf == of, zf || sf != of, !zf && sf == of,
	}
	return tbl[cc]
}

func TestBitOps(t *testing.T) {
	c, b := new386()
	reset386(c)
	c.Seg[DS] = 0x1000
	// 暫存器形式：bt／bts／btr／btc ax,cx（位元位移取低 4 位）。
	c.R[AX] = 0x0005
	c.R[CX] = 0x0012                       // 18 → 低 4 位 ＝ 2
	run386(t, c, []byte{0x0F, 0xA3, 0xC8}) // bt ax,cx
	if !c.Flag(CF) || c.R[AX] != 5 {
		t.Errorf("bt ax,cx：CF=%v AX=%04X", c.Flag(CF), c.R[AX])
	}
	c.IP = 0
	run386(t, c, []byte{0x0F, 0xAB, 0xC8}) // bts：位元 2 已立
	if !c.Flag(CF) || c.R[AX] != 5 {
		t.Errorf("bts 已立的位元")
	}
	c.IP = 0
	c.R[CX] = 1
	run386(t, c, []byte{0x0F, 0xAB, 0xC8}) // bts 位元 1（原為 0）
	if c.Flag(CF) || c.R[AX] != 7 {
		t.Errorf("bts 位元 1：CF=%v AX=%04X", c.Flag(CF), c.R[AX])
	}
	c.IP = 0
	run386(t, c, []byte{0x0F, 0xB3, 0xC8}) // btr 位元 1
	if !c.Flag(CF) || c.R[AX] != 5 {
		t.Errorf("btr：CF=%v AX=%04X", c.Flag(CF), c.R[AX])
	}
	c.IP = 0
	run386(t, c, []byte{0x0F, 0xBB, 0xC8}) // btc 位元 1
	if c.Flag(CF) || c.R[AX] != 7 {
		t.Errorf("btc：CF=%v AX=%04X", c.Flag(CF), c.R[AX])
	}
	// 32 位元、立即數形式：0F BA /4 ib。
	c.IP = 0
	c.setReg32(AX, 0x80000000)
	run386(t, c, []byte{0x66, 0x0F, 0xBA, 0xE0, 0x1F}) // bt eax,31
	if !c.Flag(CF) {
		t.Errorf("bt eax,31")
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x0F, 0xBA, 0xF0, 0x1F}) // btr eax,31
	if c.reg32(AX) != 0 || !c.Flag(CF) {
		t.Errorf("btr eax,31：EAX＝%08X", c.reg32(AX))
	}
	// 記憶體形式：位元位移超過運算元寬度，位址跟著調整（有號、可為負）。
	c.IP = 0
	b.Write8(Addr(0x1000, 0x300+4), 0x01) // 第二個 dword 的位元 0
	c.setReg32(CX, 32)                    // 位元 32 ＝ [0x300+4] 的位元 0
	run386(t, c, []byte{0x66, 0x0F, 0xA3, 0x0E, 0x00, 0x03})
	if !c.Flag(CF) {
		t.Errorf("bt [300h],ecx(32)：CF 應為 1")
	}
	c.IP = 0
	b.Write8(Addr(0x1000, 0x300-4), 0x80)
	c.setReg32(CX, 0xFFFFFFE0|7) // −25：位元組 −4 的位元 7
	run386(t, c, []byte{0x66, 0x0F, 0xA3, 0x0E, 0x00, 0x03})
	// −25 → dword 索引 −1（−25>>5 ＝ −1），位元 (−25)&31 ＝ 7 → [0x2FC] 的位元 7
	if !c.Flag(CF) {
		t.Errorf("bt [300h],ecx(−25)：CF 應為 1")
	}
}

func TestShldShrdBsfBsr(t *testing.T) {
	c, _ := new386()
	reset386(c)
	// SHLD eax,ebx,4：EAX 左移，低位從 EBX 高位補。
	c.setReg32(AX, 0x12345678)
	c.setReg32(BX, 0xABCDEF01)
	run386(t, c, []byte{0x66, 0x0F, 0xA4, 0xD8, 0x04})
	// CF ＝ 最後移出的位元 ＝ 0x12345678 的 bit28 ＝ 1。
	if c.reg32(AX) != 0x2345678A || !c.Flag(CF) {
		t.Errorf("shld eax,ebx,4：%08X CF=%v", c.reg32(AX), c.Flag(CF))
	}
	// SHRD eax,ebx,8。
	c.IP = 0
	c.setReg32(AX, 0x12345678)
	c.setReg32(BX, 0xABCDEF01)
	run386(t, c, []byte{0x66, 0x0F, 0xAC, 0xD8, 0x08})
	// CF ＝ 最後移出的位元 ＝ 0x12345678 的 bit7 ＝ 0。
	if c.reg32(AX) != 0x01123456 || c.Flag(CF) {
		t.Errorf("shrd eax,ebx,8：%08X CF=%v（預期 01123456、CF＝0）", c.reg32(AX), c.Flag(CF))
	}
	// SHLD／SHRD 以 CL 為計數，且計數 0 什麼都不做。
	c.IP = 0
	c.setReg32(AX, 0x1)
	c.setReg32(BX, 0)
	c.R[CX] = 32
	before := c.Flags
	run386(t, c, []byte{0x66, 0x0F, 0xA5, 0xD8})
	if c.reg32(AX) != 1 || c.Flags != before {
		t.Errorf("shld 計數 32（遮成 0）：EAX＝%08X flags %04X→%04X", c.reg32(AX), before, c.Flags)
	}
	// 16 位元 SHLD ax,bx,4。
	c.IP = 0
	c.R[AX] = 0x1234
	c.R[BX] = 0xABCD
	run386(t, c, []byte{0x0F, 0xA4, 0xD8, 0x04})
	if c.R[AX] != 0x234A {
		t.Errorf("shld ax,bx,4：%04X", c.R[AX])
	}
	// BSF／BSR。
	c.IP = 0
	c.setReg32(BX, 0x00F00000)
	c.setReg32(AX, 0xFFFFFFFF)
	run386(t, c, []byte{0x66, 0x0F, 0xBC, 0xC3}) // bsf eax,ebx
	if c.reg32(AX) != 20 || c.Flag(ZF) {
		t.Errorf("bsf：EAX＝%d ZF=%v", c.reg32(AX), c.Flag(ZF))
	}
	c.IP = 0
	run386(t, c, []byte{0x66, 0x0F, 0xBD, 0xC3}) // bsr
	if c.reg32(AX) != 23 {
		t.Errorf("bsr：EAX＝%d", c.reg32(AX))
	}
	c.IP = 0
	c.setReg32(BX, 0)
	c.setReg32(AX, 0x55)
	run386(t, c, []byte{0x66, 0x0F, 0xBC, 0xC3})
	if !c.Flag(ZF) || c.reg32(AX) != 0x55 {
		t.Errorf("bsf 0：ZF=%v EAX＝%X（目的暫存器要不動）", c.Flag(ZF), c.reg32(AX))
	}
	c.IP = 0
	c.R[BX] = 0x0100
	run386(t, c, []byte{0x0F, 0xBD, 0xC3}) // bsr ax,bx（16 位元）
	if c.R[AX] != 8 {
		t.Errorf("bsr ax,bx：%d", c.R[AX])
	}
}

func TestLSSLFSLGS(t *testing.T) {
	c, b := new386()
	reset386(c)
	c.Seg[DS] = 0x1000
	b.Write8(Addr(0x1000, 0x300), 0x34)
	b.Write8(Addr(0x1000, 0x301), 0x12)
	b.Write8(Addr(0x1000, 0x302), 0x00)
	b.Write8(Addr(0x1000, 0x303), 0x60)
	run386(t, c, []byte{0x0F, 0xB4, 0x06, 0x00, 0x03}) // lfs ax,[300h]
	if c.R[AX] != 0x1234 || c.FSeg != 0x6000 {
		t.Errorf("lfs：AX＝%04X FS＝%04X", c.R[AX], c.FSeg)
	}
	c.IP = 0
	run386(t, c, []byte{0x0F, 0xB5, 0x06, 0x00, 0x03}) // lgs
	if c.GSeg != 0x6000 {
		t.Errorf("lgs：GS＝%04X", c.GSeg)
	}
	c.IP = 0
	run386(t, c, []byte{0x0F, 0xB2, 0x06, 0x00, 0x03}) // lss
	if c.Seg[SS] != 0x6000 {
		t.Errorf("lss：SS＝%04X", c.Seg[SS])
	}
}

// ---- 失敗即關閉與機型閘門 ---------------------------------------------------

// Model80386 的 0F 不是 POP CS：沒實作的第二個位元組一律報錯。
func TestUnknown0FFailsClosed(t *testing.T) {
	for _, second := range []byte{0x00, 0x01, 0x02, 0x20, 0x22, 0x31, 0xA2, 0xB0, 0xC0, 0xC8, 0xFF} {
		c, _ := new386()
		reset386(c)
		sp := c.R[SP]
		err := stepWith(c, []byte{0x0F, second, 0xC0})
		if err == nil || !strings.Contains(err.Error(), "沒有這一道") {
			t.Errorf("0F %02X：應失敗即關閉，得到 %v", second, err)
		}
		if c.R[SP] != sp {
			t.Errorf("0F %02X 動了堆疊（被當成 POP CS？）", second)
		}
	}
}

// 帶 66 而 32 位元語意沒實作的 opcode 不能悄悄用 16 位元跑完。
func TestUnsupported66FailsClosed(t *testing.T) {
	for _, code := range [][]byte{
		{0x66, 0xE8, 0, 0, 0, 0},       // call rel32
		{0x66, 0xC3},                   // ret（彈 32 位元）
		{0x66, 0x9A, 0, 0, 0, 0, 0, 0}, // call far32
		{0x66, 0xE9, 0, 0, 0, 0},       // jmp rel32
		{0x66, 0x74, 0x02},             // je rel8（66 ＋ 短跳）
		{0x66, 0xFF, 0xD0},             // call eax
		{0x66, 0xC9},                   // leave
		{0x66, 0xCF},                   // iretd
	} {
		c, _ := new386()
		reset386(c)
		if err := stepWith(c, code); err == nil {
			t.Errorf("% X：應該失敗即關閉", code)
		}
	}
	// 不看運算元大小的 opcode 帶 66 照 16 位元路徑：mov al,imm8；位元組 add。
	c, _ := new386()
	reset386(c)
	run386(t, c, []byte{0x66, 0xB0, 0x42})
	if uint8(c.R[AX]) != 0x42 {
		t.Errorf("66 B0 42 ＝ mov al,42h：AL＝%02X", uint8(c.R[AX]))
	}
}

// 0x67 配隱含位址暫存器的指令（字串、xlat、loop）未實作，不能悄悄用 16 位元位址跑。
func TestUnsupported67FailsClosed(t *testing.T) {
	for _, code := range [][]byte{
		{0x67, 0xA4},       // movsb
		{0x67, 0xF3, 0xAA}, // rep stosb
		{0x67, 0xD7},       // xlat
		{0x67, 0xE2, 0xFE}, // loop
		{0x67, 0xE3, 0x00}, // jcxz
		{0x67, 0x66, 0xA5}, // movsd
	} {
		c, _ := new386()
		reset386(c)
		if err := stepWith(c, code); err == nil {
			t.Errorf("% X：應該失敗即關閉", code)
		}
	}
}

// 8086／80186 的語料路徑不受影響：0F ＝ POP CS，64／65／66／67 是條件跳躍別名。
func TestPrefixesAreAliasesBeforeModel386(t *testing.T) {
	for _, model := range []Model{Model8086, Model80186} {
		b := newTestBus()
		c := New(b)
		c.Model = model
		c.Seg[CS], c.Seg[SS] = 0x1000, 0x1000
		c.R[SP] = 0x100
		busWrite16(b, Linear(0x1000, 0x100), 0x2000)
		b.Write8(Addr(0x1000, 0), 0x0F)
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.Seg[CS] != 0x2000 {
			t.Errorf("model %d：0F 應該是 POP CS，CS＝%04X", model, c.Seg[CS])
		}
		if model != Model8086 {
			continue
		}
		// 8086：60–6F 是 70–7F 的別名。64＝JZ（別名 74），ZF 立起來 → 跳。
		for _, op := range []byte{0x64, 0x65, 0x67} {
			c := New(newTestBus())
			c.Seg[CS] = 0x1000
			c.SetFlags(ZF | CF | SF | PF)
			c.Bus.Write8(Addr(0x1000, 0), op)
			c.Bus.Write8(Addr(0x1000, 1), 0x05)
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			// 64＝JZ、65＝JNZ、67＝JA（CF＝1 不跳）
			wantJump := map[byte]bool{0x64: true, 0x65: false, 0x67: false}[op]
			if (c.IP == 7) != wantJump {
				t.Errorf("8086 的 %02X 應是條件跳躍別名：IP＝%d", op, c.IP)
			}
		}
	}
}

// 狀態保存的欄位：Hi 與 FSeg／GSeg 要隨 Reset 歸零（Snapshot／LoadState 的測試在 machine 套件）。
func TestResetClears386State(t *testing.T) {
	c, _ := new386()
	c.Hi = [8]uint16{1, 2, 3, 4, 5, 6, 7, 8}
	c.FSeg, c.GSeg = 9, 10
	c.Reset()
	if c.Hi != [8]uint16{} || c.FSeg != 0 || c.GSeg != 0 {
		t.Errorf("Reset 沒清 386 狀態：Hi=%v FS=%d GS=%d", c.Hi, c.FSeg, c.GSeg)
	}
}

// busRead16／busWrite16 以線性位址直接讀寫 Bus（記憶體原語改吃段索引之後，
// 測試裡以「段值:偏移」定址的檢查走這裡，434 §4.2）。
func busRead16(b Bus, a uint32) uint16 { return uint16(b.Read8(a)) | uint16(b.Read8(a+1))<<8 }

func busWrite16(b Bus, a uint32, v uint16) {
	b.Write8(a, uint8(v))
	b.Write8(a+1, uint8(v>>8))
}

func busRead32(b Bus, a uint32) uint32 {
	return uint32(busRead16(b, a)) | uint32(busRead16(b, a+2))<<16
}

func busWrite32(b Bus, a uint32, v uint32) {
	busWrite16(b, a, uint16(v))
	busWrite16(b, a+2, uint16(v>>16))
}
