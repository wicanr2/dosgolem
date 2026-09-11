package cpu386

import (
	"math"
	"testing"
)

// 掃過整個 FD2.EXE 的 code 段之後，它用到的 x87 只有 17 種、75 條指令。這一批是
// 其中原本沒有的：DC /0 FADD、DC /6 FDIV、DD /0 FLD m64、DD /3 FSTP m64、
// DD /7 FNSTSW、DA /1 FIMUL、DE C1 FADDP、DE C9 FMULP、D9 FA FSQRT、D9 E4 FTST。
// 一次盤完再補，比一輪撞一個省好幾輪四十分鐘的實跑。

func newFPU(t *testing.T, program []byte) (testBus, *CPU) {
	t.Helper()
	mem := testBus(make([]byte, 256))
	copy(mem, program)
	c := New(mem)
	c.Seg[SegDS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 0, Limit: 255, Writable: true})
	c.R[ESI] = 100
	c.FPUControl = 0x037f
	return mem, c
}

func putFloat64(mem testBus, at int, value float64) {
	bits := math.Float64bits(value)
	for i := 0; i < 8; i++ {
		mem[at+i] = byte(bits >> (8 * i))
	}
}

func getFloat64(mem testBus, at int) float64 {
	var bits uint64
	for i := 0; i < 8; i++ {
		bits |= uint64(mem[at+i]) << (8 * i)
	}
	return math.Float64frombits(bits)
}

func TestDCMemoryArithmetic(t *testing.T) {
	for _, tc := range []struct {
		name    string
		modrm   byte
		operand float64
		start   float64
		want    float64
	}{
		{"FADD", 0x06, 2.5, 4, 6.5},
		{"FMUL", 0x0e, 1.5, 8, 12},
		{"FDIV", 0x36, 4, 10, 2.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem, c := newFPU(t, []byte{0xdc, tc.modrm})
			putFloat64(mem, 100, tc.operand)
			c.FPUStack[0] = tc.start
			c.FPUDepth = 1
			if err := c.Step(); err != nil {
				t.Fatalf("%s：%v", tc.name, err)
			}
			if c.FPUStack[0] != tc.want {
				t.Fatalf("%v 之後是 %v，應為 %v", tc.name, c.FPUStack[0], tc.want)
			}
			if getFloat64(mem, 100) != tc.operand {
				t.Fatal("來源記憶體被改了；DC 的目的是堆疊頂")
			}
		})
	}
}

func TestFDIVByZeroSetsZeroDivideAndInfinity(t *testing.T) {
	mem, c := newFPU(t, []byte{0xdc, 0x36})
	putFloat64(mem, 100, 0)
	c.FPUStack[0] = 3
	c.FPUDepth = 1
	if err := c.Step(); err != nil {
		t.Fatalf("FDIV：%v", err)
	}
	if !math.IsInf(c.FPUStack[0], 1) || c.FPUStatus&(1<<2) == 0 {
		t.Fatalf("除以零之後 st0=%v status=%X", c.FPUStack[0], c.FPUStatus)
	}
}

func TestFLDAndFSTPRoundTripDouble(t *testing.T) {
	mem, c := newFPU(t, []byte{0xdd, 0x06}) // fld qword [esi]
	putFloat64(mem, 100, -12.25)
	if err := c.Step(); err != nil {
		t.Fatalf("FLD：%v", err)
	}
	if c.FPUDepth != 1 || c.FPUStack[0] != -12.25 {
		t.Fatalf("FLD 之後 depth=%d st0=%v", c.FPUDepth, c.FPUStack[0])
	}
	c.EIP = 0
	copy(mem, []byte{0xdd, 0x1e}) // fstp qword [esi]
	c.R[ESI] = 120
	if err := c.Step(); err != nil {
		t.Fatalf("FSTP：%v", err)
	}
	if getFloat64(mem, 120) != -12.25 {
		t.Fatalf("存出 %v，應為 -12.25", getFloat64(mem, 120))
	}
	if c.FPUDepth != 0 {
		t.Fatalf("FSTP 之後 depth=%d，應為 0", c.FPUDepth)
	}
}

func TestFNSTSWStoresStatus(t *testing.T) {
	mem, c := newFPU(t, []byte{0xdd, 0x3e}) // fnstsw word [esi]
	c.FPUStatus = 0x4123
	if err := c.Step(); err != nil {
		t.Fatalf("FNSTSW：%v", err)
	}
	if mem[100] != 0x23 || mem[101] != 0x41 {
		t.Fatalf("存出 %02X%02X，應為 4123", mem[101], mem[100])
	}
}

func TestFIMULUsesSignedInteger(t *testing.T) {
	mem, c := newFPU(t, []byte{0xda, 0x0e})            // fimul dword [esi]
	for i, b := range []byte{0xfd, 0xff, 0xff, 0xff} { // -3
		mem[100+i] = b
	}
	c.FPUStack[0] = 2.5
	c.FPUDepth = 1
	if err := c.Step(); err != nil {
		t.Fatalf("FIMUL：%v", err)
	}
	if c.FPUStack[0] != -7.5 {
		t.Fatalf("2.5 × -3 = %v，應為 -7.5（當成無號會變成天文數字）", c.FPUStack[0])
	}
}

func TestFADDPAndFMULPPopAfterCombining(t *testing.T) {
	for _, tc := range []struct {
		name  string
		modrm byte
		want  float64
	}{
		{"FADDP", 0xc1, 7},
		{"FMULP", 0xc9, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c := newFPU(t, []byte{0xde, tc.modrm})
			c.FPUStack[0] = 3 // st(0)
			c.FPUStack[1] = 4 // st(1)
			c.FPUStack[2] = 9
			c.FPUDepth = 3
			if err := c.Step(); err != nil {
				t.Fatalf("%s：%v", tc.name, err)
			}
			if c.FPUDepth != 2 {
				t.Fatalf("depth=%d，應為 2（算完要彈出）", c.FPUDepth)
			}
			if c.FPUStack[0] != tc.want {
				t.Fatalf("結果 %v，應為 %v", c.FPUStack[0], tc.want)
			}
			if c.FPUStack[1] != 9 {
				t.Fatalf("彈出後 st(1)=%v，應為 9", c.FPUStack[1])
			}
		})
	}
}

func TestFSQRT(t *testing.T) {
	_, c := newFPU(t, []byte{0xd9, 0xfa})
	c.FPUStack[0] = 16
	c.FPUDepth = 1
	if err := c.Step(); err != nil || c.FPUStack[0] != 4 {
		t.Fatalf("FSQRT：%v st0=%v", err, c.FPUStack[0])
	}
}

func TestFSQRTOfNegativeIsInvalid(t *testing.T) {
	_, c := newFPU(t, []byte{0xd9, 0xfa})
	c.FPUStack[0] = -1
	c.FPUDepth = 1
	if err := c.Step(); err != nil {
		t.Fatalf("FSQRT：%v", err)
	}
	if !math.IsNaN(c.FPUStack[0]) || c.FPUStatus&1 == 0 {
		t.Fatalf("負數開根號：st0=%v status=%X", c.FPUStack[0], c.FPUStatus)
	}
}

func TestFTSTSetsConditionCodesWithoutPopping(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value float64
		want  uint16
	}{
		{"負", -1, 0x0100},
		{"零", 0, 0x4000},
		{"正", 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c := newFPU(t, []byte{0xd9, 0xe4})
			c.FPUStack[0] = tc.value
			c.FPUDepth = 1
			c.FPUStatus = 0x4500
			if err := c.Step(); err != nil {
				t.Fatalf("FTST：%v", err)
			}
			if got := c.FPUStatus & 0x4500; got != tc.want {
				t.Fatalf("條件碼 %04X，應為 %04X", got, tc.want)
			}
			if c.FPUDepth != 1 {
				t.Fatal("FTST 不該彈出")
			}
		})
	}
}

// 反對照：堆疊空的時候每一個都要失敗即關閉，不能拿 0 當運算元。
func TestX87OpsRefuseEmptyStack(t *testing.T) {
	for _, tc := range []struct {
		name    string
		program []byte
	}{
		{"FADD", []byte{0xdc, 0x06}},
		{"FSTP", []byte{0xdd, 0x1e}},
		{"FIMUL", []byte{0xda, 0x0e}},
		{"FADDP", []byte{0xde, 0xc1}},
		{"FSQRT", []byte{0xd9, 0xfa}},
		{"FTST", []byte{0xd9, 0xe4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c := newFPU(t, tc.program)
			c.FPUDepth = 0
			if err := c.Step(); err == nil {
				t.Fatalf("%s 在空堆疊上被放行了", tc.name)
			}
		})
	}
}
