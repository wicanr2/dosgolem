package cpu386

import "testing"

// FD2 的 0x15BAF..0x15BCE 是一整串：fild dword [esp+8] → fmul qword [0x50144]
// （double 1.5）→ call 0x377A4 → fistp dword [esp+8]。那個 call 是 Watcom 的
// 「向零截斷取整」helper：fnstcw → 把控制字的 RC 設成 11 → fldcw → frndint →
// fldcw 還原。少了其中任何一個，取整方向就會變成預設的「取最近」，結果差一格。
//
// 這裡把整串照原樣走一遍，驗的是它們湊在一起算得對，不只是各自不報錯。
func TestWatcomTruncateSequence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value int32
		want  int32
	}{
		// 8 × 1.5 = 12，剛好整數。
		{"整數結果", 8, 12},
		// 5 × 1.5 = 7.5；向零截斷是 7，取最近偶數會變成 8。
		{"截斷不進位", 5, 7},
		{"負值向零", -5, -7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(make([]byte, 256))
			// fild dword [esi] ; fmul qword [edi] ; fnstcw [ebx] ; ...（手動走）
			copy(mem, []byte{0xdb, 0x06}) // fild dword [esi]
			c := New(mem)
			c.Seg[SegDS] = 0x28
			c.SetDescriptor(0x28, Descriptor{Base: 0, Limit: 255, Writable: true})
			c.R[ESI] = 100
			c.R[EDI] = 110
			c.R[EBX] = 120
			put32(mem, 100, uint32(tc.value))
			// double 1.5 = 3FF8000000000000
			put32(mem, 110, 0x00000000)
			put32(mem, 114, 0x3ff80000)
			c.FPUControl = 0x037f
			if err := c.Step(); err != nil {
				t.Fatalf("fild：%v", err)
			}

			c.EIP = 0
			copy(mem, []byte{0xdc, 0x0f}) // fmul qword [edi]
			if err := c.Step(); err != nil {
				t.Fatalf("fmul：%v", err)
			}
			if want := float64(tc.value) * 1.5; c.FPUStack[0] != want {
				t.Fatalf("乘完是 %v，應為 %v", c.FPUStack[0], want)
			}

			c.EIP = 0
			copy(mem, []byte{0xd9, 0x3b}) // fnstcw [ebx]
			if err := c.Step(); err != nil {
				t.Fatalf("fnstcw：%v", err)
			}
			if mem[120] != 0x7f || mem[121] != 0x03 {
				t.Fatalf("fnstcw 存出 %02X%02X，應為 037F", mem[121], mem[120])
			}

			// 把 RC 設成 11（向零截斷），就是 helper 裡的 `mov byte [esp+1], 0x1f`。
			mem[121] = 0x1f
			c.EIP = 0
			copy(mem, []byte{0xd9, 0x2b}) // fldcw [ebx]
			if err := c.Step(); err != nil {
				t.Fatalf("fldcw：%v", err)
			}
			if c.FPUControl != 0x1f7f {
				t.Fatalf("控制字=%04X，應為 1F7F", c.FPUControl)
			}

			c.EIP = 0
			copy(mem, []byte{0xd9, 0xfc}) // frndint
			if err := c.Step(); err != nil {
				t.Fatalf("frndint：%v", err)
			}

			c.EIP = 0
			copy(mem, []byte{0xdb, 0x1e}) // fistp dword [esi]
			if err := c.Step(); err != nil {
				t.Fatalf("fistp：%v", err)
			}
			got := int32(read32(mem, 100))
			if got != tc.want {
				t.Fatalf("%d × 1.5 截斷後是 %d，應為 %d", tc.value, got, tc.want)
			}
			if c.FPUDepth != 0 {
				t.Fatalf("fistp 之後堆疊深度=%d，應為 0", c.FPUDepth)
			}
		})
	}
}

// 反對照：RC 沒被改成截斷時，7.5 要進位成 8（取最近偶數）。這一條證明上面那串
// 真的是靠控制字在決定方向，不是實作寫死了截斷。
func TestFRNDINTFollowsRoundingControl(t *testing.T) {
	for _, tc := range []struct {
		name    string
		control uint16
		want    float64
	}{
		{"取最近偶數", 0x037f, 8},
		{"向下", 0x077f, 7},
		{"向上", 0x0b7f, 8},
		{"向零截斷", 0x0f7f, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(make([]byte, 16))
			copy(mem, []byte{0xd9, 0xfc})
			c := New(mem)
			c.FPUStack[0] = 7.5
			c.FPUDepth = 1
			c.FPUControl = tc.control
			if err := c.Step(); err != nil {
				t.Fatalf("frndint：%v", err)
			}
			if c.FPUStack[0] != tc.want {
				t.Fatalf("控制字 %04X 之下 7.5 取整成 %v，應為 %v",
					tc.control, c.FPUStack[0], tc.want)
			}
		})
	}
}

func TestX87MemoryFormsRefuseUnknownExtensions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		bytes []byte
	}{
		{"D9 /0 FLD m32", []byte{0xd9, 0x06}},
		{"D9 /2 FST m32", []byte{0xd9, 0x16}},
		{"DB /2 FIST", []byte{0xdb, 0x16}},
		{"DC /4 FSUBR", []byte{0xdc, 0x26}},
		{"DC 暫存器形式", []byte{0xdc, 0xc1}},
		{"DD /2 FST m64", []byte{0xdd, 0x16}},
		{"DD 暫存器形式", []byte{0xdd, 0xc1}},
		{"DA /0 FIADD", []byte{0xda, 0x06}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(make([]byte, 128))
			copy(mem, tc.bytes)
			c := New(mem)
			c.Seg[SegDS] = 0x28
			c.SetDescriptor(0x28, Descriptor{Base: 64, Limit: 31, Writable: true})
			c.R[ESI] = 6
			c.FPUStack[0] = 1
			c.FPUDepth = 1
			if err := c.Step(); err == nil {
				t.Fatalf("%s 被放行了", tc.name)
			}
		})
	}
}

func put32(mem testBus, at int, value uint32) {
	mem[at] = byte(value)
	mem[at+1] = byte(value >> 8)
	mem[at+2] = byte(value >> 16)
	mem[at+3] = byte(value >> 24)
}

func read32(mem testBus, at int) uint32 {
	return uint32(mem[at]) | uint32(mem[at+1])<<8 |
		uint32(mem[at+2])<<16 | uint32(mem[at+3])<<24
}
