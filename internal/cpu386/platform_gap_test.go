package cpu386

import "testing"

func TestRegisterOR32(t *testing.T) {
	for _, v := range []struct{ a, b, w, f uint32 }{
		{0x80000000, 3, 0x80000003, SF | PF}, {0, 0, 0, ZF | PF}, {1, 2, 3, PF}, {0, 1, 1, 0},
	} {
		c := New(testBus{0x09, 0xc6})
		c.R[ESI] = v.a
		c.R[EAX] = v.b
		c.EFlags = CF | OF | IF | DF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[ESI] != v.w || c.R[EAX] != v.b || c.EFlags != (v.f|IF|DF) || c.EIP != 2 {
			t.Fatalf("OR registers=%v flags=%X", c.R, c.EFlags)
		}
	}
}

func TestALImmediateLogic(t *testing.T) {
	for _, op := range []byte{0xa8, 0x24} {
		c := New(testBus{op, 1})
		c.R[EAX] = 0xabcd1202
		c.EFlags = CF | OF | IF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		want := uint32(0xabcd1202)
		if op == 0x24 {
			want = 0xabcd1200
		}
		if c.R[EAX] != want || c.EFlags != (ZF|PF|IF) {
			t.Fatal("AL mask width/flags")
		}
	}
}
func TestRegisterAddAndMemoryTest(t *testing.T) {
	c := New(testBus{3, 0xf8})
	c.R[EDI] = 0x7fffffff
	c.R[EAX] = 1
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EDI] != 0x80000000 || c.R[EAX] != 1 || c.EFlags&(OF|SF|AF|PF) != (OF|SF|AF|PF) {
		t.Fatal("ADD overflow")
	}
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0xf7, 7, 1, 0, 0, 0})
	mem[32] = 3
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	c.R[EDI] = 32
	c.EFlags = CF | OF | IF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EFlags != IF || mem[32] != 3 || c.R[EDI] != 32 {
		t.Fatal("TEST modified memory or flags")
	}
	c.EIP = 0
	c.SetDescriptor(0x160, Descriptor{Limit: 33})
	if err := c.Step(); err == nil {
		t.Fatal("TEST crossing limit accepted")
	}
}

func TestAbsoluteImmediateByteStore(t *testing.T) {
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0xc6, 5, 32, 0, 0, 0, 0xab})
	mem[33] = 0xcd
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63, Writable: true})
	c.EFlags = 0xed7
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if mem[32] != 0xab || mem[33] != 0xcd || c.EIP != 7 || c.EFlags != 0xed7 {
		t.Fatal("byte store width")
	}
	c.EIP = 0
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	mem[32] = 0
	if err := c.Step(); err == nil || mem[32] != 0 {
		t.Fatal("readonly byte store")
	}
}

func TestCMPGenericBaseDisp8(t *testing.T) {
	for _, base := range []byte{EAX, EBP, EDI} {
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0x3b, 0x58 | base, 0xfc})
		mem[64] = 2
		c := New(mem)
		c.Seg[SegDS] = 0x160
		c.Seg[SegSS] = 0x168
		seg := uint16(0x160)
		if base == EBP {
			seg = 0x168
		}
		c.SetDescriptor(seg, Descriptor{Base: 32, Limit: 95})
		c.R[base] = 36
		c.R[EBX] = 1
		regs := c.R
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R != regs || c.EFlags&(CF|SF) != (CF|SF) || mem[64] != 2 {
			t.Fatal("CMP result")
		}
	}
}

func TestRepMOVSD(t *testing.T) {
	for _, back := range []bool{false, true} {
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0xf3, 0xa5})
		copy(mem[32:], []byte{1, 2, 3, 4, 5, 6, 7, 8})
		c := New(mem)
		c.Seg[SegDS] = 0x160
		c.Seg[SegES] = 0x168
		c.SetDescriptor(0x160, Descriptor{Limit: 63})
		c.SetDescriptor(0x168, Descriptor{Base: 64, Limit: 63, Writable: true})
		c.R[ESI] = 32
		c.R[EDI] = 0
		c.R[ECX] = 2
		c.EFlags = CF | OF | IF
		if back {
			c.EFlags |= DF
			c.R[ESI] = 36
			c.R[EDI] = 4
		}
		flags := c.EFlags
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 8; i++ {
			if mem[64+i] != byte(i+1) {
				t.Fatal("MOVSD data")
			}
		}
		wantSI, wantDI := uint32(40), uint32(8)
		if back {
			wantSI = 28
			wantDI = 0xfffffffc
		}
		if c.R[ECX] != 0 || c.R[ESI] != wantSI || c.R[EDI] != wantDI || c.EFlags != flags {
			t.Fatal("MOVSD pointers/flags")
		}
	}
	c := New(testBus{0xf3, 0xa5})
	if err := c.Step(); err != nil {
		t.Fatal("zero REP accessed memory", err)
	}
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0xf3, 0xa5})
	mem[32] = 7
	c = New(mem)
	c.Seg[SegDS] = 0x160
	c.Seg[SegES] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63, Writable: true})
	c.R[ESI] = 32
	c.R[EDI] = 36
	c.R[ECX] = 2
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if mem[40] != 7 {
		t.Fatal("MOVSD used memmove instead of sequential copy")
	}
}

func TestADDImmediate32Register(t *testing.T) {
	c := New(testBus{0x81, 0xc4, 1, 0, 0, 0})
	c.R[ESP] = 0xffffffff
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ESP] != 0 || c.EFlags&(CF|ZF|AF|PF) != (CF|ZF|AF|PF) || c.EIP != 6 {
		t.Fatal("ADD32 carry")
	}
}

func TestIndirectAbsoluteCall(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0xff, 0x15, 32, 0, 0, 0})
	mem[32] = 48
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.Seg[SegSS] = 0x168
	c.SetDescriptor(0x160, Descriptor{Limit: 127})
	c.SetDescriptor(0x168, Descriptor{Base: 64, Limit: 63, Writable: true})
	c.R[ESP] = 32
	c.EFlags = 0xed7
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 48 || c.R[ESP] != 28 || mem[92] != 6 || c.EFlags != 0xed7 {
		t.Fatal("CALL return/target")
	}
	c.EIP = 0
	c.R[ESP] = 32
	c.SetDescriptor(0x168, Descriptor{Limit: 127})
	mem[92] = 0
	if err := c.Step(); err == nil || c.R[ESP] != 32 || mem[92] != 0 {
		t.Fatal("CALL readonly stack")
	}
}

func TestMOVWordBaseDisp8(t *testing.T) {
	for _, base := range []byte{EBP, EAX} {
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0x66, 0x8b, 0x58 | base, 0xfe})
		mem[64] = 0x34
		mem[65] = 0x12
		c := New(mem)
		c.Seg[SegDS] = 0x160
		c.Seg[SegSS] = 0x168
		seg := uint16(0x160)
		if base == EBP {
			seg = 0x168
		}
		c.SetDescriptor(seg, Descriptor{Base: 32, Limit: 95})
		c.R[base] = 34
		c.R[EBX] = 0xabcdffff
		c.EFlags = 0xed7
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EBX] != 0xabcd1234 || c.R[base] != 34 || c.EFlags != 0xed7 {
			t.Fatal("MOV word")
		}
		c.EIP = 0
		c.SetDescriptor(seg, Descriptor{Base: 32, Limit: 32})
		if err := c.Step(); err == nil {
			t.Fatal("word boundary")
		}
	}
}

func TestNearJGE(t *testing.T) {
	for _, flags := range []uint32{0, SF, OF, SF | OF} {
		c := New(testBus{0x0f, 0x8d, 0xfa, 0xff, 0xff, 0xff})
		c.EFlags = flags
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		want := uint32(6)
		if flags == 0 || flags == SF|OF {
			want = 0
		}
		if c.EIP != want || c.EFlags != flags {
			t.Fatal("JGE condition/rel32")
		}
	}
}

func TestLEA32Addressing(t *testing.T) {
	for _, tc := range []struct {
		code []byte
		want uint32
	}{
		{[]byte{0x8d, 0x3c, 0x0e}, 15},
		{[]byte{0x8d, 0x3c, 0x8e}, 30},
		{[]byte{0x8d, 0x3c, 0x8d, 0xfc, 0xff, 0xff, 0xff}, 16},
		{[]byte{0x8d, 0x7c, 0x24, 0xfc}, 28},
		{[]byte{0x8d, 0x3d, 0xff, 0xff, 0xff, 0xff}, 0xffffffff},
		{[]byte{0x8d, 0xbc, 0x0e, 0xf8, 0xff, 0xff, 0xff}, 7},
	} {
		c := New(testBus(tc.code))
		c.R[ESI] = 10
		c.R[ECX] = 5
		c.R[ESP] = 32
		c.EFlags = 0xed7
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EDI] != tc.want || c.EFlags != 0xed7 || c.EIP != uint32(len(tc.code)) {
			t.Fatalf("LEA %x result=%x flags=%x", tc.code, c.R[EDI], c.EFlags)
		}
	}
	c := New(testBus{0x8d, 0xff})
	c.EFlags = 0xed7
	if err := c.Step(); err == nil || c.EFlags != 0xed7 {
		t.Fatal("非法LEA不得執行CMP")
	}
}

func TestMOVSPartialFailureAndSingleIteration(t *testing.T) {
	for _, op := range []byte{0xa4, 0xa5} {
		width := uint32(1)
		if op == 0xa5 {
			width = 4
		}
		for _, rep := range []bool{false, true} {
			mem := testBus(make([]byte, 64))
			copy(mem, []byte{op})
			if rep {
				copy(mem, []byte{0xf3, op})
			}
			for i := 32; i < 40; i++ {
				mem[i] = byte(i)
			}
			c := New(mem)
			c.Seg[SegDS] = 1
			c.Seg[SegES] = 2
			c.SetDescriptor(1, Descriptor{Limit: 63})
			c.SetDescriptor(2, Descriptor{Limit: 47 + width, Writable: true})
			c.R[ESI] = 32
			c.R[EDI] = 48
			c.R[ECX] = 2
			c.EFlags = IF | CF
			err := c.Step()
			if (err != nil) != rep || c.R[ESI] != 32+width || c.R[EDI] != 48+width || c.EFlags != IF|CF {
				t.Fatalf("op=%x rep=%v err=%v regs=%v", op, rep, err, c.R)
			}
			want := uint32(2)
			if rep {
				want = 1
			}
			if c.R[ECX] != want {
				t.Fatal("錯誤次數")
			}
			for i := uint32(0); i < width; i++ {
				if mem[48+i] != mem[32+i] {
					t.Fatal("已完成搬移遺失")
				}
			}
		}
	}
}

func TestWordStoreBaseDisp8(t *testing.T) {
	for _, base := range []byte{EAX, EBP} {
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0x66, 0x89, 0x48 | base, 0xfc})
		c := New(mem)
		c.R[base] = 36
		c.R[ECX] = 0xabcd1234
		c.EFlags = 0xed7
		c.Seg[SegDS] = 1
		c.Seg[SegSS] = 2
		seg := uint16(1)
		if base == EBP {
			seg = 2
		}
		c.SetDescriptor(seg, Descriptor{Base: 32, Limit: 95, Writable: true})
		before := c.R
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if mem[64] != 0x34 || mem[65] != 0x12 || mem[66] != 0 || c.R != before || c.EFlags != 0xed7 {
			t.Fatal("word寬度／來源／flags")
		}
		c.EIP = 0
		c.SetDescriptor(seg, Descriptor{Base: 32, Limit: 32, Writable: true})
		mem[64] = 0
		if err := c.Step(); err == nil || mem[64] != 0 {
			t.Fatal("越界word部分寫入")
		}
	}
}

func TestWordImmediateStore(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x66, 0xc7, 0x45, 0xfc, 0x34, 0x12})
	c := New(mem)
	c.Seg[SegSS] = 2
	c.SetDescriptor(2, Descriptor{Base: 32, Limit: 95, Writable: true})
	c.R[EBP] = 36
	c.EFlags = 0xed7
	before := c.R
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if mem[64] != 0x34 || mem[65] != 0x12 || mem[66] != 0 || c.R != before || c.EFlags != 0xed7 || c.EIP != 6 {
		t.Fatal("C7 word寬度／flags")
	}
}

func TestADDMemoryDisp8(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x03, 0x45, 0xfc})
	mem[64] = 1
	c := New(mem)
	c.Seg[SegSS] = 2
	c.SetDescriptor(2, Descriptor{Base: 32, Limit: 95})
	c.R[EBP] = 36
	c.R[EAX] = 0x7fffffff
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 0x80000000 || c.EFlags&(OF|SF) != (OF|SF) || mem[64] != 1 {
		t.Fatal("ADD memory flags")
	}
	c.EIP = 0
	c.R[EAX] = 7
	c.SetDescriptor(2, Descriptor{Base: 32, Limit: 32})
	flags := c.EFlags
	if err := c.Step(); err == nil || c.R[EAX] != 7 || c.EFlags != flags {
		t.Fatal("ADD失敗仍發布結果")
	}
}

func TestPUSHFWordAndPopAX(t *testing.T) {
	mem := testBus(make([]byte, 96))
	copy(mem, []byte{0x66, 0x9c, 0x66, 0x58})
	c := New(mem)
	c.Seg[SegSS] = 2
	c.SetDescriptor(2, Descriptor{Base: 32, Limit: 63, Writable: true})
	c.R[ESP] = 32
	c.R[EAX] = 0xabcd0000
	c.EFlags = 0x200ed7
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ESP] != 30 || mem[62] != 0xd7 || mem[63] != 0x0e || mem[64] != 0 {
		t.Fatal("PUSHF寬度")
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ESP] != 32 || c.R[EAX] != 0xabcd0ed7 || c.EFlags != 0x200ed7 {
		t.Fatal("PUSHF/POP AX狀態")
	}
	c.EIP = 0
	c.R[ESP] = 1
	if err := c.Step(); err == nil || c.R[ESP] != 1 {
		t.Fatal("PUSHF下溢")
	}
}

func TestMOVSXWordMemory(t *testing.T) {
	for _, v := range []uint16{0, 0x7fff, 0x8000, 0xffff} {
		mem := testBus(make([]byte, 64))
		copy(mem, []byte{0x0f, 0xbf, 0x5b, 0xfc})
		mem[32] = byte(v)
		mem[33] = byte(v >> 8)
		c := New(mem)
		c.Seg[SegDS] = 1
		c.SetDescriptor(1, Descriptor{Limit: 63})
		c.R[EBX] = 36
		c.EFlags = 0xed7
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EBX] != uint32(int32(int16(v))) || c.EFlags != 0xed7 {
			t.Fatal("MOVSX符號擴展")
		}
	}
}

func TestCMPWordSignedImmediate(t *testing.T) {
	for _, tc := range []struct {
		v     uint16
		imm   byte
		flags uint32
	}{
		{0xffff, 0xff, ZF | PF}, {0x7fff, 0xff, CF | SF | OF | PF}, {0, 1, CF | SF | AF | PF},
	} {
		mem := testBus(make([]byte, 64))
		copy(mem, []byte{0x66, 0x83, 0x78, 0, tc.imm})
		mem[32] = byte(tc.v)
		mem[33] = byte(tc.v >> 8)
		c := New(mem)
		c.Seg[SegDS] = 1
		c.SetDescriptor(1, Descriptor{Limit: 63})
		c.R[EAX] = 32
		c.EFlags = IF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.EFlags != tc.flags|IF || mem[32] != byte(tc.v) || c.R[EAX] != 32 {
			t.Fatalf("CMP word flags=%x want=%x", c.EFlags, tc.flags|IF)
		}
	}
}

func TestWordStoreBaseNoDisplacement(t *testing.T) {
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0x66, 0x89, 0x03, 0xf4})
	c := New(mem)
	c.Seg[SegDS] = 1
	c.SetDescriptor(1, Descriptor{Limit: 63, Writable: true})
	c.R[EBX] = 32
	c.R[EAX] = 0xabcd1234
	c.EFlags = 0xed7
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.EIP != 3 || mem[32] != 0x34 || mem[33] != 0x12 || mem[34] != 0 || c.EFlags != 0xed7 {
		t.Fatal("無位移word寫入解碼錯誤")
	}
}

func TestWordStackRead(t *testing.T) {
	mem := testBus(make([]byte, 128))
	copy(mem, []byte{0x66, 0x8b, 0x44, 0x24, 0xfc})
	mem[64] = 0x34
	mem[65] = 0x12
	c := New(mem)
	c.Seg[SegSS] = 2
	c.SetDescriptor(2, Descriptor{Base: 32, Limit: 95})
	c.R[ESP] = 36
	c.R[EAX] = 0xabcd0000
	c.EFlags = 0xed7
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[EAX] != 0xabcd1234 || c.R[ESP] != 36 || c.EFlags != 0xed7 || c.EIP != 5 {
		t.Fatal("stack word位寬")
	}
}

func TestNearJBE(t *testing.T) {
	for _, flags := range []uint32{0, CF, ZF, CF | ZF} {
		c := New(testBus{0x0f, 0x86, 0xfc, 0xff, 0xff, 0xff})
		c.EFlags = flags | IF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		want := uint32(6)
		if flags != 0 {
			want = 2
		}
		if c.EIP != want || c.EFlags != flags|IF {
			t.Fatal("JBE條件或位移")
		}
	}
}

func TestWordReadAndMOVSXNoDisplacement(t *testing.T) {
	for _, sx := range []bool{false, true} {
		mem := testBus(make([]byte, 64))
		copy(mem, []byte{0x66, 0x8b, 0x03})
		if sx {
			copy(mem, []byte{0x0f, 0xbf, 0x04, 0x24})
		}
		mem[32] = 0
		mem[33] = 0x80
		c := New(mem)
		c.Seg[SegDS] = 1
		c.Seg[SegSS] = 1
		c.SetDescriptor(1, Descriptor{Limit: 63})
		c.R[EBX] = 32
		c.R[ESP] = 32
		c.R[EAX] = 0xabcd0000
		c.EFlags = 0xed7
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		want := uint32(0xabcd8000)
		if sx {
			want = 0xffff8000
		}
		if c.R[EAX] != want || c.EFlags != 0xed7 {
			t.Fatal("word讀取／符號擴展")
		}
	}
}

func TestIMULImmediate32(t *testing.T) {
	for _, tc := range []struct {
		src, imm, want uint32
		overflow       bool
	}{
		{8, 1748, 13984, false}, {0xfffffffe, 3, 0xfffffffa, false}, {0x40000000, 4, 0, true}, {0x80000000, 0xffffffff, 0x80000000, true},
	} {
		code := testBus{0x69, 0xc3, byte(tc.imm), byte(tc.imm >> 8), byte(tc.imm >> 16), byte(tc.imm >> 24)}
		c := New(code)
		c.R[EBX] = tc.src
		c.EFlags = IF | ZF | CF | OF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		flags := uint32(IF | ZF)
		if tc.overflow {
			flags |= CF | OF
		}
		if c.R[EAX] != tc.want || c.R[EBX] != tc.src || c.EFlags != flags {
			t.Fatal("IMUL signed32／overflow")
		}
	}
}

func TestSARRegisterCount(t *testing.T) {
	for _, tc := range []struct {
		v           uint32
		n           byte
		want, flags uint32
	}{
		{0x80000000, 31, 0xffffffff, SF | PF}, {0xffffffff, 1, 0xffffffff, SF | PF | CF}, {1, 1, 0, ZF | PF | CF}, {1, 32, 1, OF | CF},
	} {
		c := New(testBus{0xc1, 0xfa, tc.n})
		c.R[EDX] = tc.v
		c.EFlags = IF | OF | CF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EDX] != tc.want || c.EFlags != tc.flags|IF {
			t.Fatalf("SAR count=%d result=%x flags=%x", tc.n, c.R[EDX], c.EFlags)
		}
	}
}
func TestIDIVSignedAndFaults(t *testing.T) {
	for _, tc := range []struct {
		hi, lo, div, q, r uint32
		bad               bool
	}{
		{0, 1000000, 120, 8333, 40, false}, {0xffffffff, 0xfffffff9, 3, 0xfffffffe, 0xffffffff, false},
		{0, 7, 0xfffffffd, 0xfffffffe, 1, false}, {0, 1, 0, 0, 0, true}, {0, 0x80000000, 1, 0, 0, true}, {0x80000000, 0, 0xffffffff, 0, 0, true},
	} {
		c := New(testBus{0xf7, 0xfe})
		c.R[EDX] = tc.hi
		c.R[EAX] = tc.lo
		c.R[ESI] = tc.div
		c.EFlags = 0xed7
		before := c.R
		err := c.Step()
		if (err != nil) != tc.bad {
			t.Fatalf("IDIV err=%v", err)
		}
		if tc.bad && c.R != before {
			t.Fatal("IDIV失敗發布結果")
		}
		if !tc.bad && (c.R[EAX] != tc.q || c.R[EDX] != tc.r) {
			t.Fatal("IDIV商／餘數")
		}
		if c.EFlags != 0xed7 {
			t.Fatal("IDIV未定義flags策略")
		}
	}
}

func TestNOPAndORWord(t *testing.T) {
	c := New(testBus{0x90, 0x66, 0x81, 0xce, 0, 0x80})
	c.R[ESI] = 0xabcd0000
	c.EFlags = 0xed7
	before := c.R
	if err := c.Step(); err != nil || c.R != before || c.EFlags != 0xed7 || c.EIP != 1 {
		t.Fatal("NOP修改狀態")
	}
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ESI] != 0xabcd8000 || c.EFlags&(CF|OF|AF|SF|ZF|PF) != (SF|PF) {
		t.Fatal("OR word寬度／flags")
	}
}

func TestTESTWordImmediate(t *testing.T) {
	for _, v := range []uint32{0xabcd0000, 0xabcd8000, 0xabcd0001} {
		c := New(testBus{0x66, 0xf7, 0xc7, 0, 0x80})
		c.R[EDI] = v
		c.EFlags = CF | OF | IF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		flags := uint32(IF | PF | ZF)
		if v&0x8000 != 0 {
			flags = IF | PF | SF
		}
		if c.R[EDI] != v || c.EFlags != flags {
			t.Fatal("TEST word flags或暫存器")
		}
	}
}

func TestJECXZFullCounter(t *testing.T) {
	for _, v := range []uint32{0, 1, 0x10000} {
		c := New(testBus{0xe3, 0xfe})
		c.R[ECX] = v
		c.EFlags = 0xed7
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		want := uint32(2)
		if v == 0 {
			want = 0
		}
		if c.EIP != want || c.R[ECX] != v || c.EFlags != 0xed7 {
			t.Fatal("JECXZ 狀態")
		}
	}
}
func TestINBytePreservesState(t *testing.T) {
	for _, accept := range []bool{false, true} {
		c := New(testBus{0xec})
		c.R[EAX] = 0xabcdef01
		c.R[EDX] = 0x123403da
		c.EFlags = 0xed7
		c.PortIn = func(p uint16) (uint8, bool) {
			if p != 0x3da {
				t.Fatal("埠寬度")
			}
			return 9, accept
		}
		err := c.Step()
		if (err == nil) != accept {
			t.Fatalf("IN error %v", err)
		}
		want := uint32(0xabcdef01)
		if accept {
			want = 0xabcdef09
		}
		if c.R[EAX] != want || c.R[EDX] != 0x123403da || c.EFlags != 0xed7 {
			t.Fatal("IN 狀態")
		}
	}
}
func TestTESTAccumulatorDword(t *testing.T) {
	for _, v := range []uint32{0, 8} {
		c := New(testBus{0xa9, 8, 0, 0, 0})
		c.R[EAX] = v
		c.EFlags = IF | CF | OF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		want := uint32(IF)
		if v == 0 {
			want |= ZF | PF
		}
		if c.R[EAX] != v || c.EFlags != want {
			t.Fatalf("TEST flags %x", c.EFlags)
		}
	}
}

func TestLOOPFullCounter(t *testing.T) {
	for _, v := range []uint32{0, 1, 2, 0x10000} {
		c := New(testBus{0xe2, 0xfe})
		c.R[ECX] = v
		c.EFlags = 0xed7
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		want := uint32(0)
		if v == 1 {
			want = 2
		}
		if c.EIP != want || c.R[ECX] != v-1 || c.EFlags != 0xed7 {
			t.Fatal("LOOP 狀態")
		}
	}
}

func TestNearJL(t *testing.T) {
	for _, flags := range []uint32{0, SF, OF, SF | OF, ZF, SF | ZF, OF | ZF, SF | OF | ZF} {
		c := New(testBus{0x0f, 0x8c, 0xfc, 0xff, 0xff, 0xff})
		c.EFlags = flags | IF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		want := uint32(6)
		if (flags&SF != 0) != (flags&OF != 0) {
			want = 2
		}
		if c.EIP != want || c.EFlags != flags|IF {
			t.Fatal("JL 條件")
		}
	}
}

func TestDIVUnsignedAndFaults(t *testing.T) {
	for _, tc := range []struct {
		hi, lo, div, q, r uint32
		bad               bool
	}{
		{0, 1000000, 120, 8333, 40, false}, {1, 0, 2, 0x80000000, 0, false},
		{0, 0xffffffff, 1, 0xffffffff, 0, false}, {0, 7, 0, 0, 0, true}, {1, 0, 1, 0, 0, true},
	} {
		c := New(testBus{0xf7, 0xf3})
		c.R[EDX] = tc.hi
		c.R[EAX] = tc.lo
		c.R[EBX] = tc.div
		c.EFlags = 0xed7
		before := c.R
		err := c.Step()
		if (err != nil) != tc.bad {
			t.Fatalf("DIV error %v", err)
		}
		if tc.bad && c.R != before {
			t.Fatal("DIV 失敗發布暫存器")
		}
		if !tc.bad && (c.R[EAX] != tc.q || c.R[EDX] != tc.r) {
			t.Fatal("DIV 商餘數")
		}
		if c.R[EBX] != tc.div || c.EFlags != 0xed7 {
			t.Fatal("DIV 副作用")
		}
	}
}

func TestMULUnsigned(t *testing.T) {
	for _, tc := range []struct{ a, b, hi, lo uint32 }{
		{8333, 10000, 0, 83330000}, {0xffffffff, 0xffffffff, 0xfffffffe, 1}, {0, 0xffffffff, 0, 0}, {0x80000000, 2, 1, 0},
	} {
		c := New(testBus{0xf7, 0xe1})
		c.R[EAX] = tc.a
		c.R[ECX] = tc.b
		c.EFlags = IF | ZF | CF | OF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		flags := uint32(IF | ZF)
		if tc.hi != 0 {
			flags |= CF | OF
		}
		if c.R[EAX] != tc.lo || c.R[EDX] != tc.hi || c.R[ECX] != tc.b || c.EFlags != flags {
			t.Fatal("MUL 結果")
		}
	}
}

func TestORWordSignedImmediate(t *testing.T) {
	for _, tc := range []struct {
		imm         byte
		want, flags uint32
	}{
		{3, 0xabcd0003, PF}, {0x80, 0xabcdff80, SF}, {0xff, 0xabcdffff, SF | PF},
	} {
		c := New(testBus{0x66, 0x83, 0xce, tc.imm})
		c.R[ESI] = 0xabcd0000
		c.EFlags = IF | CF | OF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[ESI] != tc.want || c.EFlags != tc.flags|IF {
			t.Fatalf("OR word %x flags %x", c.R[ESI], c.EFlags)
		}
	}
}

func TestSBBRegisterCarryEdges(t *testing.T) {
	for _, tc := range []struct{ a, b, carry, want, flags uint32 }{
		{8207, 0, 0, 8207, PF}, {0, 0, 1, 0xffffffff, CF | AF | SF | PF},
		{0, 0xffffffff, 1, 0, CF | AF | ZF | PF}, {0x80000000, 0x7fffffff, 1, 0, OF | AF | ZF | PF},
		{0x7fffffff, 0xffffffff, 1, 0x7fffffff, CF | AF | PF}, {0x80000000, 1, 0, 0x7fffffff, OF | AF | PF},
	} {
		c := New(testBus{0x1b, 0xc2})
		c.R[EAX] = tc.a
		c.R[EDX] = tc.b
		c.EFlags = IF | tc.carry
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EAX] != tc.want || c.R[EDX] != tc.b || c.EFlags != IF|tc.flags {
			t.Fatalf("SBB %x - %x - %d = %x flags %x want %x", tc.a, tc.b, tc.carry, c.R[EAX], c.EFlags, tc.flags|IF)
		}
	}
}

func TestNearJA(t *testing.T) {
	for _, flags := range []uint32{0, CF, ZF, CF | ZF} {
		for _, neg := range []bool{true, false} {
			code := testBus{0x0f, 0x87, 4, 0, 0, 0}
			target := uint32(10)
			if neg {
				code = testBus{0x0f, 0x87, 0xfc, 0xff, 0xff, 0xff}
				target = 2
			}
			c := New(code)
			c.EFlags = IF | flags
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if flags != 0 {
				target = 6
			}
			if c.EIP != target || c.EFlags != IF|flags {
				t.Fatal("JA 條件與位移")
			}
		}
	}
}

func TestIndirectJumpCSAndBounds(t *testing.T) {
	for _, limit := range []uint32{63, 38} {
		mem := testBus(make([]byte, 160))
		copy(mem, []byte{0x2e, 0xff, 0x24, 0x85, 32, 0, 0, 0})
		mem[100] = 0x78
		mem[101] = 0x56
		mem[102] = 0x34
		mem[103] = 0x12
		mem[36] = 0xaa
		c := New(mem)
		c.Seg[SegCS] = 8
		c.Seg[SegDS] = 16
		c.SetDescriptor(8, Descriptor{Base: 64, Limit: limit})
		c.SetDescriptor(16, Descriptor{Limit: 159})
		c.R[EAX] = 1
		c.R[ESP] = 128
		c.EFlags = 0xed7
		before := c.R
		err := c.Step()
		if limit == 63 && (err != nil || c.EIP != 0x12345678) {
			t.Fatalf("CS jump EIP=%x err=%v", c.EIP, err)
		}
		if limit == 38 && err == nil {
			t.Fatal("跨段界限未拒絕")
		}
		if c.R != before || c.EFlags != 0xed7 {
			t.Fatal("跳躍改動暫存器或旗標")
		}
	}
	c := New(testBus{0xff, 0xe2})
	c.R[EDX] = 0x12345678
	if err := c.Step(); err != nil || c.EIP != c.R[EDX] {
		t.Fatalf("register JMP: %v", err)
	}
}

func TestIndexedByteCompare(t *testing.T) {
	for _, ss := range []bool{false, true} {
		for _, value := range []byte{0, 1, 255} {
			mem := testBus(make([]byte, 128))
			code := []byte{0x80, 0x3c, 0x19, 1}
			if ss {
				code = []byte{0x80, 0x7c, 0x24, 4, 1}
			}
			copy(mem, code)
			mem[68] = value
			c := New(mem)
			c.Seg[SegDS] = 1
			c.Seg[SegSS] = 2
			c.SetDescriptor(1, Descriptor{Base: 32, Limit: 95})
			c.SetDescriptor(2, Descriptor{Base: 64, Limit: 63})
			c.R[ECX] = 32
			c.R[EBX] = 4
			c.EFlags = IF | DF
			before := c.R
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if (c.EFlags&CF != 0) != (value < 1) || (c.EFlags&ZF != 0) != (value == 1) || c.R != before || mem[68] != value {
				t.Fatal("byte CMP狀態錯誤")
			}
			c.EIP = 0
			c.EFlags = IF | DF
			c.SetDescriptor(1, Descriptor{Limit: 0})
			c.SetDescriptor(2, Descriptor{Limit: 0})
			if err := c.Step(); err == nil || c.EFlags != IF|DF {
				t.Fatal("越界未保持旗標")
			}
		}
	}
}

func TestRegisterIMULTwoOperand(t *testing.T) {
	for _, tc := range []struct {
		a, b, w  uint32
		overflow bool
	}{
		{3, 7, 21, false}, {0xffffffff, 7, 0xfffffff9, false},
		{0x80000000, 1, 0x80000000, false}, {0x80000000, 0xffffffff, 0x80000000, true},
		{0x7fffffff, 2, 0xfffffffe, true}, {0, 0xffffffff, 0, false},
	} {
		c := New(testBus{0x0f, 0xaf, 0xd0})
		c.R[EDX] = tc.a
		c.R[EAX] = tc.b
		c.EFlags = IF | DF | ZF | CF | OF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		flags := uint32(IF | DF | ZF)
		if tc.overflow {
			flags |= CF | OF
		}
		if c.R[EDX] != tc.w || c.R[EAX] != tc.b || c.EFlags != flags {
			t.Fatalf("IMUL %+v: %v %x", tc, c.R, c.EFlags)
		}
	}
}

func TestStackDwordImmediateCompare(t *testing.T) {
	for _, value := range []uint32{0, 2048, 0x80000000, 0xffffffff} {
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0x81, 0x7c, 0x24, 0x20, 0, 8, 0, 0})
		for i := 0; i < 4; i++ {
			mem[96+i] = byte(value >> uint(i*8))
		}
		c := New(mem)
		c.Seg[SegSS] = 2
		c.SetDescriptor(2, Descriptor{Base: 64, Limit: 63})
		c.EFlags = IF | DF
		before := c.R
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if (c.EFlags&CF != 0) != (value < 2048) || (c.EFlags&ZF != 0) != (value == 2048) || c.R != before {
			t.Fatal("dword比較")
		}
		c.EIP = 0
		c.EFlags = IF | DF
		c.SetDescriptor(2, Descriptor{Base: 64, Limit: 34})
		if err := c.Step(); err == nil || c.EFlags != IF|DF {
			t.Fatal("跨界失敗")
		}
	}
}

func TestSAROneBit(t *testing.T) {
	for _, tc := range []struct{ v, w, f uint32 }{
		{0, 0, ZF | PF}, {1, 0, CF | ZF | PF}, {0xffffffff, 0xffffffff, CF | SF | PF},
		{0x80000000, 0xc0000000, SF | PF}, {6, 3, PF},
	} {
		c := New(testBus{0xd1, 0xf8})
		c.R[EAX] = tc.v
		c.EFlags = IF | DF | OF | CF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EAX] != tc.w || c.EFlags != tc.f|IF|DF {
			t.Fatalf("SAR result=%x flags=%x", c.R[EAX], c.EFlags)
		}
	}
}

func TestMemorySubtract32(t *testing.T) {
	for _, ss := range []bool{false, true} {
		for _, tc := range []struct{ a, b, w, f uint32 }{
			{0, 1, 0xffffffff, CF | SF}, {0x80000000, 1, 0x7fffffff, OF}, {3, 3, 0, ZF},
		} {
			mem := testBus(make([]byte, 128))
			code := []byte{0x2b, 0x46, 0x10}
			if ss {
				code = []byte{0x2b, 0x44, 0x24, 0x10}
			}
			copy(mem, code)
			for i := 0; i < 4; i++ {
				mem[80+i] = byte(tc.b >> uint(8*i))
			}
			c := New(mem)
			c.Seg[SegDS] = 1
			c.Seg[SegSS] = 2
			c.SetDescriptor(1, Descriptor{Base: 64, Limit: 63})
			c.SetDescriptor(2, Descriptor{Base: 64, Limit: 63})
			c.R[EAX] = tc.a
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if c.R[EAX] != tc.w || c.EFlags&(CF|SF|OF|ZF) != tc.f {
				t.Fatal("SUB結果或旗標")
			}
			c.EIP = 0
			c.R[EAX] = tc.a
			c.EFlags = IF
			c.SetDescriptor(1, Descriptor{Limit: 0})
			c.SetDescriptor(2, Descriptor{Limit: 0})
			if err := c.Step(); err == nil || c.R[EAX] != tc.a || c.EFlags != IF {
				t.Fatal("SUB越界副作用")
			}
		}
	}
}

func TestCDQSignExtension(t *testing.T) {
	for _, v := range []uint32{0, 1, 0x7fffffff, 0x80000000, 0xffffffff} {
		c := New(testBus{0x99})
		c.R[EAX] = v
		c.R[EDX] = 123
		c.EFlags = 0xed7
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		want := uint32(0)
		if v >= 0x80000000 {
			want = 0xffffffff
		}
		if c.R[EDX] != want || c.R[EAX] != v || c.EFlags != 0xed7 {
			t.Fatal("CDQ狀態")
		}
	}
}

func TestMemoryIMULDestinationAndSS(t *testing.T) {
	mem := testBus(make([]byte, 160))
	copy(mem, []byte{0x0f, 0xaf, 0x4d, 0x40})
	mem[128] = 0xfe
	mem[129] = 0xff
	mem[130] = 0xff
	mem[131] = 0xff
	c := New(mem)
	c.Seg[SegSS] = 2
	c.SetDescriptor(2, Descriptor{Base: 64, Limit: 95})
	c.R[ECX] = 7
	c.R[EAX] = 123
	c.EFlags = IF | CF | OF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ECX] != 0xfffffff2 || c.R[EAX] != 123 || c.EFlags != IF {
		t.Fatal("IMUL目的或SS")
	}
	c.EIP = 0
	c.EFlags = IF | CF
	c.R[ECX] = 7
	c.SetDescriptor(2, Descriptor{Base: 64, Limit: 65})
	if err := c.Step(); err == nil || c.R[ECX] != 7 || c.EFlags != IF|CF {
		t.Fatal("IMUL跨界副作用")
	}
}

func TestMemoryDwordTEST(t *testing.T) {
	for _, ss := range []bool{false, true} {
		for _, v := range []byte{0, 0x20, 0xff} {
			mem := testBus(make([]byte, 128))
			code := []byte{0xf7, 0x43, 0x1c, 0x20, 0, 0, 0}
			if ss {
				code = []byte{0xf7, 0x44, 0x24, 0x1c, 0x20, 0, 0, 0}
			}
			copy(mem, code)
			mem[92] = v
			c := New(mem)
			c.Seg[SegDS] = 1
			c.Seg[SegSS] = 2
			c.SetDescriptor(1, Descriptor{Base: 64, Limit: 63})
			c.SetDescriptor(2, Descriptor{Base: 64, Limit: 63})
			c.EFlags = IF | DF | CF | OF
			before := c.R
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if (c.EFlags&ZF != 0) != (v&0x20 == 0) || c.EFlags&(CF|OF) != 0 || c.R != before || mem[92] != v {
				t.Fatal("TEST改動資料或旗標錯誤")
			}
			c.EIP = 0
			c.EFlags = IF
			c.SetDescriptor(1, Descriptor{Limit: 0})
			c.SetDescriptor(2, Descriptor{Limit: 0})
			if err := c.Step(); err == nil || c.EFlags != IF {
				t.Fatal("TEST越界")
			}
		}
	}
}

func TestIndexedCallAndOldESP(t *testing.T) {
	for _, stackSource := range []bool{false, true} {
		mem := testBus(make([]byte, 160))
		code := []byte{0xff, 0x14, 0x85, 32, 0, 0, 0}
		targetPos := 36
		if stackSource {
			code = []byte{0xff, 0x14, 0x24}
			targetPos = 128
		}
		copy(mem, code)
		mem[targetPos] = 0x78
		mem[targetPos+1] = 0x56
		c := New(mem)
		c.Seg[SegDS] = 1
		c.Seg[SegSS] = 2
		c.SetDescriptor(1, Descriptor{Limit: 159})
		c.SetDescriptor(2, Descriptor{Base: 64, Limit: 95, Writable: true})
		c.R[EAX] = 1
		c.R[ESP] = 64
		c.EFlags = 0xed7
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.EIP != 0x5678 || c.R[ESP] != 60 || mem[124] != byte(len(code)) || c.EFlags != 0xed7 {
			t.Fatal("CALL順序或返回位址")
		}
		c.EIP = 0
		c.R[ESP] = 64
		c.SetDescriptor(2, Descriptor{Base: 64, Limit: 95})
		if err := c.Step(); err == nil || c.R[ESP] != 64 || c.EIP == 0x5678 {
			t.Fatal("唯讀堆疊仍CALL")
		}
	}
}

func TestXOREAXImmediate(t *testing.T) {
	for _, tc := range []struct{ v, w, f uint32 }{{0, 0x8000, PF}, {0x8000, 0, ZF | PF}, {0xffff8001, 0xffff0001, SF}} {
		c := New(testBus{0x35, 0, 0x80, 0, 0})
		c.R[EAX] = tc.v
		c.R[EDX] = 123
		c.EFlags = IF | DF | CF | OF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EAX] != tc.w || c.EFlags != tc.f|IF|DF || c.R[EDX] != 123 {
			t.Fatal("XOR結果或旗標")
		}
	}
}

func TestMemoryPushOldESP(t *testing.T) {
	mem := testBus(make([]byte, 160))
	copy(mem, []byte{0xff, 0x74, 0x24, 0x14})
	mem[116] = 0x78
	mem[117] = 0x56
	mem[112] = 0xaa
	c := New(mem)
	c.Seg[SegSS] = 2
	c.SetDescriptor(2, Descriptor{Base: 64, Limit: 95, Writable: true})
	c.R[ESP] = 32
	c.EFlags = 0xed7
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ESP] != 28 || mem[92] != 0x78 || mem[93] != 0x56 || c.EFlags != 0xed7 {
		t.Fatal("PUSH未用舊ESP")
	}
	c.EIP = 0
	c.R[ESP] = 32
	c.SetDescriptor(2, Descriptor{Base: 64, Limit: 95})
	if err := c.Step(); err == nil || c.R[ESP] != 32 {
		t.Fatal("唯讀堆疊未拒絕")
	}
}

func TestMemoryAddSubtract(t *testing.T) {
	for _, op := range []byte{0x01, 0x29} {
		for _, ss := range []bool{false, true} {
			for _, initial := range []uint32{0, 0x7fffffff, 0x80000000, 0xffffffff} {
				mem := testBus(make([]byte, 128))
				code := []byte{op, 0x43, 0x10}
				if ss {
					code = []byte{op, 0x44, 0x24, 0x10}
				}
				copy(mem, code)
				for i := 0; i < 4; i++ {
					mem[80+i] = byte(initial >> uint(i*8))
				}
				c := New(mem)
				c.Seg[SegDS] = 1
				c.Seg[SegSS] = 2
				c.SetDescriptor(1, Descriptor{Base: 64, Limit: 63, Writable: true})
				c.SetDescriptor(2, Descriptor{Base: 64, Limit: 63, Writable: true})
				c.R[EAX] = 1
				c.EFlags = IF
				if err := c.Step(); err != nil {
					t.Fatal(err)
				}
				got := uint32(mem[80]) | uint32(mem[81])<<8 | uint32(mem[82])<<16 | uint32(mem[83])<<24
				want := initial + 1
				cf := initial == 0xffffffff
				of := initial == 0x7fffffff
				if op == 0x29 {
					want = initial - 1
					cf = initial == 0
					of = initial == 0x80000000
				}
				if got != want || (c.EFlags&CF != 0) != cf || (c.EFlags&OF != 0) != of || c.R[EAX] != 1 {
					t.Fatal("ADD/SUB結果")
				}
				c.EIP = 0
				c.EFlags = IF
				c.SetDescriptor(1, Descriptor{Base: 64, Limit: 63})
				c.SetDescriptor(2, Descriptor{Base: 64, Limit: 63})
				if err := c.Step(); err == nil || c.EFlags != IF || mem[80] != byte(want) {
					t.Fatal("唯讀寫入副作用")
				}
			}
		}
	}
}

func TestF2MOVSCompatibility(t *testing.T) {
	for _, op := range []byte{0xa4, 0xa5} {
		for _, flags := range []uint32{IF, IF | ZF, IF | DF, IF | DF | ZF} {
			for _, count := range []uint32{0, 2} {
				mem := testBus(make([]byte, 128))
				copy(mem, []byte{0xf2, op})
				for i := 0; i < 16; i++ {
					mem[32+i] = byte(i + 1)
				}
				c := New(mem)
				c.Seg[SegDS] = 1
				c.Seg[SegES] = 2
				c.SetDescriptor(1, Descriptor{Limit: 127})
				c.SetDescriptor(2, Descriptor{Limit: 127, Writable: true})
				c.R[ESI] = 36
				c.R[EDI] = 80
				c.R[ECX] = count
				c.EFlags = flags
				width := uint32(1)
				if op == 0xa5 {
					width = 4
				}
				if count == 0 {
					c.R[ESI] = 999
					c.R[EDI] = 999
				}
				src, dst := c.R[ESI], c.R[EDI]
				if err := c.Step(); err != nil {
					t.Fatal(err)
				}
				delta := count * width
				if flags&DF != 0 {
					delta = 0 - delta
				}
				if c.R[ECX] != 0 || c.R[ESI] != src+delta || c.R[EDI] != dst+delta || c.EFlags != flags {
					t.Fatal("F2計數、方向或旗標")
				}
				for n := uint32(0); n < count; n++ {
					offset := n * width
					if flags&DF != 0 {
						offset = 0 - offset
					}
					for b := uint32(0); b < width; b++ {
						if mem[dst+offset+b] != mem[src+offset+b] {
							t.Fatal("F2資料")
						}
					}
				}
			}
		}
	}
}

func TestMemoryCompare32BaseAndSIB(t *testing.T) {
	for _, ss := range []bool{false, true} {
		for _, v := range []uint32{0, 1, 0x80000000} {
			mem := testBus(make([]byte, 128))
			code := []byte{0x3b, 0x03}
			if ss {
				code = []byte{0x3b, 0x04, 0x24}
			}
			copy(mem, code)
			mem[80] = 1
			c := New(mem)
			c.Seg[SegDS] = 1
			c.Seg[SegSS] = 2
			c.SetDescriptor(1, Descriptor{Base: 64, Limit: 63})
			c.SetDescriptor(2, Descriptor{Base: 64, Limit: 63})
			c.R[EBX] = 16
			c.R[ESP] = 16
			c.R[EAX] = v
			before := c.R
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if (c.EFlags&CF != 0) != (v == 0) || (c.EFlags&ZF != 0) != (v == 1) || (c.EFlags&OF != 0) != (v == 0x80000000) || c.R != before {
				t.Fatal("CMP結果")
			}
			c.EIP = 0
			c.EFlags = IF
			c.SetDescriptor(1, Descriptor{Limit: 0})
			c.SetDescriptor(2, Descriptor{Limit: 0})
			if err := c.Step(); err == nil || c.EFlags != IF {
				t.Fatal("CMP越界副作用")
			}
		}
	}
}

func TestMemoryUnsignedDivide(t *testing.T) {
	for _, divisor := range []byte{0, 3} {
		for _, high := range []uint32{0, 3} {
			mem := testBus(make([]byte, 128))
			copy(mem, []byte{0xf7, 0x75, 0x18})
			mem[88] = divisor
			c := New(mem)
			c.Seg[SegSS] = 2
			c.SetDescriptor(2, Descriptor{Base: 64, Limit: 63})
			c.R[EAX] = 8
			c.R[EDX] = high
			c.EFlags = 0xed7
			err := c.Step()
			if divisor == 0 || high >= uint32(divisor) {
				if err == nil || c.R[EAX] != 8 || c.R[EDX] != high {
					t.Fatal("DIV錯誤發布")
				}
			} else if err != nil || c.R[EAX] != 2 || c.R[EDX] != 2 {
				t.Fatalf("DIV商餘 %v", err)
			}
			if c.EFlags != 0xed7 {
				t.Fatal("未定義旗標策略改變")
			}
		}
	}
}

func TestWordANDImmediate(t *testing.T) {
	for _, tc := range []struct{ v, w, f uint32 }{{0xabcd0001, 0xabcd0000, ZF | PF}, {0xabcdffff, 0xabcdfe00, SF | PF}, {0xabcd7fff, 0xabcd7e00, PF}} {
		c := New(testBus{0x66, 0x81, 0xe7, 0, 0xfe})
		c.R[EDI] = tc.v
		c.EFlags = IF | DF | CF | OF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		if c.R[EDI] != tc.w || c.EFlags != tc.f|IF|DF {
			t.Fatal("word AND寬度或旗標")
		}
	}
}

func TestMemoryImmediateIMUL(t *testing.T) {
	for _, source := range []uint32{0xffffffff, 3, 0x80000000} {
		mem := testBus(make([]byte, 128))
		copy(mem, []byte{0x69, 0x54, 0x24, 0x10, 0xfe, 0xff, 0xff, 0xff})
		for i := 0; i < 4; i++ {
			mem[80+i] = byte(source >> uint(8*i))
		}
		c := New(mem)
		c.Seg[SegSS] = 2
		c.SetDescriptor(2, Descriptor{Base: 64, Limit: 63})
		c.R[EDX] = 123
		c.EFlags = IF | DF
		if err := c.Step(); err != nil {
			t.Fatal(err)
		}
		want := uint32(0 - source*2)
		overflow := source == 0x80000000
		if c.R[EDX] != want || (c.EFlags&(CF|OF) == (CF|OF)) != overflow || c.EFlags&(IF|DF) != (IF|DF) {
			t.Fatal("IMUL記憶體即值")
		}
		c.EIP = 0
		c.R[EDX] = 123
		c.EFlags = IF
		c.SetDescriptor(2, Descriptor{Limit: 0})
		if err := c.Step(); err == nil || c.R[EDX] != 123 || c.EFlags != IF {
			t.Fatal("IMUL越界")
		}
	}
}

func TestXOR33RegisterDestination(t *testing.T) {
	c := New(testBus{0x33, 0xca})
	c.R[ECX] = 0x80000000
	c.R[EDX] = 3
	c.EFlags = CF | OF | IF
	if err := c.Step(); err != nil {
		t.Fatal(err)
	}
	if c.R[ECX] != 0x80000003 || c.R[EDX] != 3 || c.EFlags != SF|PF|IF {
		t.Fatal("33目的或旗標")
	}
	c = New(testBus{0x33, 0xc0})
	c.R[EAX] = 123
	if err := c.Step(); err != nil || c.R[EAX] != 0 || c.EFlags != ZF|PF|2 {
		t.Fatal("XOR自身")
	}
}

func TestWordAccumulatorLoadAddRotate(t *testing.T) {
	mem := testBus(make([]byte, 80))
	copy(mem, []byte{0x66, 0xa1, 16, 0, 0, 0})
	mem[48] = 0x34
	mem[49] = 0x92
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 32, Limit: 17})
	c.R[EAX] = 0xabcdffff
	c.EFlags = IF | CF
	if err := c.Step(); err != nil || c.R[EAX] != 0xabcd9234 || c.EFlags != IF|CF || c.EIP != 6 {
		t.Fatal("word moffs來源或保存")
	}
	c.EIP = 0
	c.SetDescriptor(0x160, Descriptor{Base: 32, Limit: 16})
	if err := c.Step(); err == nil || c.R[EAX] != 0xabcd9234 {
		t.Fatal("word moffs越界")
	}
	for _, v := range []struct {
		a, b, w uint16
		flags   uint32
	}{
		{0xffff, 1, 0, CF | AF | ZF | PF}, {0x7fff, 1, 0x8000, OF | SF | AF | PF}, {0x8000, 0x8000, 0, CF | OF | ZF | PF},
	} {
		c = New(testBus{0x66, 5, byte(v.b), byte(v.b >> 8)})
		c.R[EAX] = 0xabcd0000 | uint32(v.a)
		c.EFlags = IF
		if err := c.Step(); err != nil || c.R[EAX] != 0xabcd0000|uint32(v.w) || c.EFlags != IF|v.flags {
			t.Fatalf("word ADD: %v %X", err, c.EFlags)
		}
	}
	for _, v := range []struct {
		a, w  uint16
		flags uint32
	}{{0x8000, 1, CF | OF}, {0x4000, 0x8000, OF}, {0xffff, 0xffff, CF}, {0, 0, 0}} {
		c = New(testBus{0x66, 0xd1, 0xc0})
		c.R[EAX] = 0xabcd0000 | uint32(v.a)
		c.EFlags = IF | SF | ZF | PF | AF | CF | OF
		if err := c.Step(); err != nil || c.R[EAX] != 0xabcd0000|uint32(v.w) || c.EFlags != IF|SF|ZF|PF|AF|v.flags {
			t.Fatalf("word ROL: %v %X", err, c.EFlags)
		}
	}
}

func TestMOVZXByteMemoryEA(t *testing.T) {
	for _, v := range []struct {
		code []byte
		seg  int
	}{{[]byte{0x0f, 0xb6, 5, 32, 0, 0, 0}, SegDS}, {[]byte{0x0f, 0xb6, 0x45, 0xfc}, SegSS}} {
		mem := testBus(make([]byte, 80))
		copy(mem, v.code)
		mem[48] = 0xfe
		c := New(mem)
		c.R[EAX] = 0xffffffff
		c.R[EBP] = 36
		c.Seg[v.seg] = 0x160
		c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 32})
		c.EFlags = CF | OF | IF
		if err := c.Step(); err != nil || c.R[EAX] != 254 || c.EFlags != CF|OF|IF {
			t.Fatal("MOVZX來源或旗標")
		}
		c.EIP = 0
		c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 31})
		if err := c.Step(); err == nil || c.R[EAX] != 254 {
			t.Fatal("MOVZX界限")
		}
	}
}

func TestCMPByteSource(t *testing.T) {
	c := New(testBus{0x3a, 0xe1})
	c.R[EAX] = 0x8000
	c.R[ECX] = 1
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[EAX] != 0x8000 || c.R[ECX] != 1 || c.EFlags != IF|OF|AF {
		t.Fatalf("CMP AH: %v %X", err, c.EFlags)
	}
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0x3a, 0x0a})
	mem[32] = 1
	c = New(mem)
	c.R[EDX] = 32
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	c.EFlags = IF
	if err := c.Step(); err != nil || c.EFlags != IF|CF|AF|SF|PF || mem[32] != 1 {
		t.Fatalf("CMP byte: %v %X", err, c.EFlags)
	}
	c.EIP = 0
	c.SetDescriptor(0x160, Descriptor{Limit: 31})
	flags := c.EFlags
	if err := c.Step(); err == nil || c.EFlags != flags {
		t.Fatal("CMP越界發布旗標")
	}
}

func TestNearConditionalBranchAllFlags(t *testing.T) {
	// 每一旗標組合以條件碼配對及Intel条件建立獨立真值。
	for bits := 0; bits < 32; bits++ {
		flags := uint32(IF)
		for i, f := range []uint32{OF, CF, ZF, SF, PF} {
			if bits&(1<<i) != 0 {
				flags |= f
			}
		}
		for pair := 0; pair < 8; pair++ {
			var yes bool
			switch pair {
			case 0:
				yes = bits&1 != 0
			case 1:
				yes = bits&2 != 0
			case 2:
				yes = bits&4 != 0
			case 3:
				yes = bits&6 != 0
			case 4:
				yes = bits&8 != 0
			case 5:
				yes = bits&16 != 0
			case 6:
				yes = (bits&8 != 0) != (bits&1 != 0)
			case 7:
				yes = bits&4 != 0 || (bits&8 != 0) != (bits&1 != 0)
			}
			for inv := 0; inv < 2; inv++ {
				c := New(testBus{15, byte(0x80 + pair*2 + inv), 0xfa, 0xff, 0xff, 0xff})
				c.EFlags = flags
				c.R[EAX] = 123
				want := uint32(6)
				if yes != (inv == 1) {
					want = 0
				}
				if err := c.Step(); err != nil || c.EIP != want || c.EFlags != flags || c.R[EAX] != 123 {
					t.Fatalf("Jcc pair%d inv%d bits%d: %v EIP%d", pair, inv, bits, err, c.EIP)
				}
			}
		}
	}
}

func TestC6StackMemoryWrite(t *testing.T) {
	mem := testBus(make([]byte, 80))
	copy(mem, []byte{0xc6, 0x44, 0x24, 0xfc, 0xab})
	mem[47] = 7
	mem[49] = 9
	c := New(mem)
	c.R[ESP] = 36
	c.Seg[SegSS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 63, Writable: true})
	c.EFlags = CF | OF | IF
	if err := c.Step(); err != nil || mem[48] != 0xab || mem[47] != 7 || mem[49] != 9 || c.EFlags != CF|OF|IF || c.R[ESP] != 36 {
		t.Fatal("C6 SS來源與寬度")
	}
	c.EIP = 0
	mem[48] = 0
	c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 63})
	if err := c.Step(); err == nil || mem[48] != 0 {
		t.Fatal("C6唯讀拒絕")
	}
}

func TestOUTDXByte(t *testing.T) {
	c := New(testBus{0xee})
	c.R[EDX] = 0xabcd03c8
	c.R[EAX] = 0x12345678
	c.EFlags = IF | CF
	calls := 0
	c.PortOut = func(p uint16, v byte) bool { calls++; return p == 0x3c8 && v == 0x78 }
	if err := c.Step(); err != nil || calls != 1 || c.EFlags != IF|CF || c.R[EAX] != 0x12345678 || c.R[EDX] != 0xabcd03c8 {
		t.Fatal("OUT DX截斷或保存")
	}
	c.EIP = 0
	c.PortOut = func(p uint16, v byte) bool { return false }
	if err := c.Step(); err == nil {
		t.Fatal("OUT未拒絕")
	}
}

func TestLODSWidthDirectionAndBounds(t *testing.T) {
	for _, word := range []bool{false, true} {
		for _, back := range []bool{false, true} {
			mem := testBus(make([]byte, 80))
			code := []byte{0xad}
			width := uint32(4)
			want := uint32(0x87654321)
			if word {
				code = []byte{0x66, 0xad}
				width = 2
				want = 0xabcd4321
			}
			copy(mem, code)
			copy(mem[48:], []byte{0x21, 0x43, 0x65, 0x87})
			c := New(mem)
			c.R[EAX] = 0xabcd0000
			c.R[ESI] = 32
			c.R[ECX] = 7
			c.EFlags = IF | CF
			if back {
				c.EFlags |= DF
			}
			flags := c.EFlags
			c.Seg[SegDS] = 0x160
			c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 63})
			pos := uint32(32) + width
			if back {
				pos = 32 - width
			}
			if err := c.Step(); err != nil || c.R[EAX] != want || c.R[ESI] != pos || c.R[ECX] != 7 || c.EFlags != flags {
				t.Fatal("LODS寬度方向")
			}
			c.EIP = 0
			c.R[ESI] = 32
			c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 32 + width - 2})
			if err := c.Step(); err == nil || c.R[ESI] != 32 || c.R[EAX] != want {
				t.Fatal("LODS失敗原子性")
			}
		}
	}
}

func TestByteShiftAndWordRegisterOperations(t *testing.T) {
	for _, v := range []struct {
		mod, count, input, want byte
		flags                   uint32
	}{
		{0xe4, 0, 0x81, 0x81, CF | OF}, {0xe4, 1, 0x81, 2, CF | OF}, {0xec, 1, 0x81, 0x40, CF | OF}, {0xec, 2, 0x81, 0x20, OF}, {0xe4, 8, 1, 0, CF | OF | ZF | PF}, {0xe4, 32, 0x81, 0x81, CF | OF},
	} {
		c := New(testBus{0xc0, v.mod, v.count})
		c.R[EAX] = 0xabcd0007 | uint32(v.input)<<8
		c.EFlags = IF | CF | OF
		if err := c.Step(); err != nil || c.R[EAX] != 0xabcd0007|uint32(v.want)<<8 || c.EFlags != IF|v.flags {
			t.Fatalf("byte shift %v: %v %X %X", v, err, c.R[EAX], c.EFlags)
		}
	}
	c := New(testBus{0xd0, 0xe1})
	c.R[ECX] = 0x80
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[ECX] != 0 || c.EFlags != IF|CF|OF|ZF|PF {
		t.Fatal("D0 SHL")
	}
	c = New(testBus{0x66, 0x2b, 0xd9})
	c.R[EBX] = 0xabcd8000
	c.R[ECX] = 1
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[EBX] != 0xabcd7fff || c.EFlags != IF|OF|AF|PF {
		t.Fatalf("SUB word %v %X", err, c.EFlags)
	}
	c = New(testBus{0x66, 0x0b, 0xd9})
	c.R[EBX] = 0xabcd8000
	c.R[ECX] = 1
	c.EFlags = IF | CF | OF
	if err := c.Step(); err != nil || c.R[EBX] != 0xabcd8001 || c.EFlags != IF|SF {
		t.Fatal("OR word")
	}
}

func TestWordMemoryDEC(t *testing.T) {
	for _, v := range []struct {
		a, w  uint16
		flags uint32
	}{{0x8000, 0x7fff, OF | AF | PF}, {1, 0, ZF | PF}, {0, 0xffff, SF | AF | PF}} {
		for _, carry := range []uint32{0, CF} {
			mem := testBus(make([]byte, 64))
			copy(mem, []byte{0x66, 0xff, 0x0d, 32, 0, 0, 0})
			mem[32] = byte(v.a)
			mem[33] = byte(v.a >> 8)
			c := New(mem)
			c.Seg[SegDS] = 0x160
			c.SetDescriptor(0x160, Descriptor{Limit: 63, Writable: true})
			c.EFlags = IF | carry
			if err := c.Step(); err != nil || uint16(mem[32])|uint16(mem[33])<<8 != v.w || c.EFlags != IF|carry|v.flags {
				t.Fatal("word DEC值或CF")
			}
			c.EIP = 0
			flags := c.EFlags
			c.SetDescriptor(0x160, Descriptor{Limit: 63})
			if err := c.Step(); err == nil || c.EFlags != flags || uint16(mem[32])|uint16(mem[33])<<8 != v.w {
				t.Fatal("word DEC唯讀拒絕")
			}
		}
	}
}

func TestMOVZXWordStackEA(t *testing.T) {
	mem := testBus(make([]byte, 80))
	copy(mem, []byte{15, 0xb7, 0x44, 0x24, 4})
	mem[48] = 0xfe
	mem[49] = 0xdc
	c := New(mem)
	c.R[ESP] = 28
	c.R[EAX] = 0xffffffff
	c.Seg[SegSS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 33})
	c.EFlags = IF | CF | OF
	if err := c.Step(); err != nil || c.R[EAX] != 0xdcfe || c.R[ESP] != 28 || c.EFlags != IF|CF|OF {
		t.Fatal("MOVZX word來源")
	}
	c.EIP = 0
	c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 32})
	if err := c.Step(); err == nil || c.R[EAX] != 0xdcfe {
		t.Fatal("MOVZX word跨界")
	}
}

func TestWordImmediateShifts(t *testing.T) {
	for _, v := range []struct {
		mod, count  byte
		input, want uint16
		flags       uint32
	}{
		{0xe0, 0, 0x8001, 0x8001, CF | OF}, {0xe0, 1, 0x8001, 2, CF | OF}, {0xe0, 2, 1, 4, OF}, {0xe8, 1, 0x8001, 0x4000, CF | OF | PF}, {0xf8, 1, 0x8001, 0xc000, CF | SF | PF}, {0xf8, 16, 0x8000, 0xffff, CF | OF | SF | PF}, {0xe0, 32, 1, 1, CF | OF},
	} {
		c := New(testBus{0x66, 0xc1, v.mod, v.count})
		c.R[EAX] = 0xabcd0000 | uint32(v.input)
		c.EFlags = IF | CF | OF
		if err := c.Step(); err != nil || c.R[EAX] != 0xabcd0000|uint32(v.want) || c.EFlags != IF|v.flags {
			t.Fatalf("word shift %v: %v %X %X", v, err, c.R[EAX], c.EFlags)
		}
	}
}

func TestDecoderWordArithmetic(t *testing.T) {
	c := New(testBus{0x66, 3, 0xd9})
	c.R[EBX] = 0xabcd7fff
	c.R[ECX] = 1
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[EBX] != 0xabcd8000 || c.EFlags != IF|OF|SF|AF|PF {
		t.Fatal("word ADD")
	}
	c = New(testBus{0x66, 0x3b, 0xda})
	c.R[EBX] = 0xabcd8000
	c.R[EDX] = 1
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[EBX] != 0xabcd8000 || c.EFlags != IF|OF|AF|PF {
		t.Fatal("word CMP")
	}
	c = New(testBus{0x66, 0x43})
	c.R[EBX] = 0xabcdffff
	c.EFlags = IF | CF
	if err := c.Step(); err != nil || c.R[EBX] != 0xabcd0000 || c.EFlags != IF|CF|ZF|AF|PF {
		t.Fatal("word INC")
	}
	c = New(testBus{0xd1, 0xe9})
	c.R[ECX] = 0x80000001
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[ECX] != 0x40000000 || c.EFlags != IF|CF|OF|PF {
		t.Fatal("SHR1")
	}
}
func TestRepeatedSTOSWord(t *testing.T) {
	for _, back := range []bool{false, true} {
		mem := testBus(make([]byte, 80))
		copy(mem, []byte{0xf3, 0x66, 0xab})
		c := New(mem)
		c.R[EAX] = 0xabcd1234
		c.R[ECX] = 2
		c.R[EDI] = 32
		c.EFlags = IF | CF
		if back {
			c.R[EDI] = 34
			c.EFlags |= DF
		}
		flags := c.EFlags
		c.Seg[SegES] = 0x160
		c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 63, Writable: true})
		want := uint32(36)
		if back {
			want = 30
		}
		if err := c.Step(); err != nil || c.R[EDI] != want || c.R[ECX] != 0 || c.EFlags != flags || mem[48] != 0x34 || mem[49] != 0x12 || mem[50] != 0x34 || mem[51] != 0x12 {
			t.Fatal("STOSW方向或寬度")
		}
		c.EIP = 0
		c.R[EDI] = 0xffff
		if err := c.Step(); err != nil || c.R[EDI] != 0xffff {
			t.Fatal("零count")
		}
	}
	mem := testBus(make([]byte, 80))
	copy(mem, []byte{0xf3, 0x66, 0xab})
	c := New(mem)
	c.R[EDI] = 32
	c.R[ECX] = 2
	c.R[EAX] = 0x1234
	c.Seg[SegES] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 33, Writable: true})
	if err := c.Step(); err == nil || c.R[ECX] != 1 || c.R[EDI] != 34 || mem[32] != 0x34 || mem[33] != 0x12 || mem[34] != 0 {
		t.Fatal("STOSW部分失敗")
	}
}

func TestRepeatedMOVSWord(t *testing.T) {
	for _, prefix := range []byte{0xf2, 0xf3} {
		for _, back := range []bool{false, true} {
			mem := testBus(make([]byte, 96))
			copy(mem, []byte{prefix, 0x66, 0xa5})
			copy(mem[32:], []byte{1, 2, 3, 4})
			c := New(mem)
			c.R[ESI] = 32
			c.R[EDI] = 64
			c.R[ECX] = 2
			c.EFlags = IF | CF
			if back {
				c.R[ESI] = 34
				c.R[EDI] = 66
				c.EFlags |= DF
			}
			c.Seg[SegDS] = 0x160
			c.Seg[SegES] = 0x160
			c.SetDescriptor(0x160, Descriptor{Limit: 95, Writable: true})
			flags := c.EFlags
			if err := c.Step(); err != nil || c.R[ECX] != 0 || c.EFlags != flags || string(mem[64:68]) != string([]byte{1, 2, 3, 4}) {
				t.Fatal("MOVSW方向")
			}
			c.EIP = 0
			c.R[ESI] = 999
			c.R[EDI] = 999
			if err := c.Step(); err != nil || c.R[ESI] != 999 || c.R[EDI] != 999 {
				t.Fatal("MOVSW零count")
			}
		}
	}
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0xf3, 0x66, 0xa5})
	mem[32] = 0xaa
	mem[33] = 0xbb
	c := New(mem)
	c.R[ESI] = 32
	c.R[EDI] = 34
	c.R[ECX] = 2
	c.Seg[SegDS] = 0x160
	c.Seg[SegES] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63, Writable: true})
	if err := c.Step(); err != nil || string(mem[32:38]) != string([]byte{0xaa, 0xbb, 0xaa, 0xbb, 0xaa, 0xbb}) {
		t.Fatal("MOVSW逐次重疊")
	}
	c.EIP = 0
	c.R[ESI] = 32
	c.R[EDI] = 62
	c.R[ECX] = 2
	if err := c.Step(); err == nil || c.R[ECX] != 1 || c.R[EDI] != 64 || c.R[ESI] != 34 {
		t.Fatal("MOVSW部分失敗")
	}
}

func TestINCMemoryByte(t *testing.T) {
	mem := testBus(make([]byte, 80))
	copy(mem, []byte{0xfe, 0x44, 0x24, 4})
	mem[48] = 0x7f
	c := New(mem)
	c.R[ESP] = 28
	c.Seg[SegSS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 63, Writable: true})
	c.EFlags = IF | CF
	if err := c.Step(); err != nil || mem[48] != 0x80 || c.EFlags != IF|CF|OF|SF|AF {
		t.Fatalf("INC byte %v %X", err, c.EFlags)
	}
	c.EIP = 0
	flags := c.EFlags
	c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 63})
	if err := c.Step(); err == nil || mem[48] != 0x80 || c.EFlags != flags {
		t.Fatal("INC唯讀拒絕")
	}
}

func TestADDMemorySIB(t *testing.T) {
	mem := testBus(make([]byte, 96))
	copy(mem, []byte{3, 0x54, 0x82, 6})
	mem[46] = 1
	c := New(mem)
	c.R[EAX] = 2
	c.R[EDX] = 32
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 95})
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[EDX] != 33 || c.R[EAX] != 2 || c.EFlags != IF|PF {
		t.Fatalf("ADD SIB %v %X", err, c.EFlags)
	}
	c.EIP = 0
	c.R[EDX] = 32
	flags := c.EFlags
	c.SetDescriptor(0x160, Descriptor{Limit: 48})
	if err := c.Step(); err == nil || c.R[EDX] != 32 || c.EFlags != flags {
		t.Fatal("ADD SIB跨界")
	}
}

func TestWordAddRotateAndByteXOR(t *testing.T) {
	c := New(testBus{0x66, 0x81, 0xc2, 1, 0})
	c.R[EDX] = 0xabcdffff
	c.EFlags = IF
	if err := c.Step(); err != nil || c.R[EDX] != 0xabcd0000 || c.EFlags != IF|CF|ZF|AF|PF {
		t.Fatal("81 word ADD")
	}
	for _, v := range []struct {
		n     byte
		a, w  uint16
		flags uint32
	}{{0, 0x8000, 0x8000, OF | CF}, {1, 0x8000, 1, OF | CF}, {3, 0xa001, 0x000d, OF | CF}, {15, 2, 1, OF | CF}} {
		c = New(testBus{0x66, 0xc1, 0xc2, v.n})
		c.R[EDX] = 0xabcd0000 | uint32(v.a)
		c.EFlags = IF | OF | CF | ZF
		if err := c.Step(); err != nil || c.R[EDX] != 0xabcd0000|uint32(v.w) || c.EFlags != IF|ZF|v.flags {
			t.Fatalf("ROL word %v %v %X", v, err, c.R[EDX])
		}
	}
	c = New(testBus{0x32, 0xe2})
	c.R[EAX] = 0xabcd8007
	c.R[EDX] = 0x83
	c.EFlags = IF | OF | CF
	if err := c.Step(); err != nil || c.R[EAX] != 0xabcd0307 || c.R[EDX] != 0x83 || c.EFlags != IF|PF {
		t.Fatal("XOR AH")
	}
}

func TestMULByteHighRegister(t *testing.T) {
	for _, v := range []struct {
		a, b  byte
		w     uint16
		flags uint32
	}{{0, 255, 0, 0}, {2, 3, 6, 0}, {255, 255, 65025, CF | OF}, {16, 16, 256, CF | OF}} {
		c := New(testBus{0xf6, 0xe4})
		c.R[EAX] = 0xabcd0000 | uint32(v.a) | uint32(v.b)<<8
		c.EFlags = IF | ZF | AF | CF | OF
		if err := c.Step(); err != nil || c.R[EAX] != 0xabcd0000|uint32(v.w) || c.EFlags != IF|ZF|AF|v.flags {
			t.Fatal("MUL byte")
		}
	}
}

func TestIMULShortImmediate(t *testing.T) {
	for _, v := range []struct {
		source      uint32
		imm         byte
		want, flags uint32
	}{{7, 0xfd, 0xffffffeb, 0}, {0x80000000, 0xff, 0x80000000, CF | OF}, {0xffffffff, 6, 0xfffffffa, 0}} {
		c := New(testBus{0x6b, 0xc1, v.imm})
		c.R[ECX] = v.source
		c.EFlags = IF | ZF | CF | OF
		if err := c.Step(); err != nil || c.R[EAX] != v.want || c.R[ECX] != v.source || c.EFlags != IF|ZF|v.flags {
			t.Fatal("6B有號即值")
		}
	}
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0x6b, 5, 32, 0, 0, 0, 6})
	mem[32] = 7
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	if err := c.Step(); err != nil || c.R[EAX] != 42 || c.EIP != 7 {
		t.Fatal("6B記憶體")
	}
}

func TestByteTESTSIB(t *testing.T) {
	mem := testBus(make([]byte, 80))
	copy(mem, []byte{0xf6, 0x44, 7, 6, 0x40})
	mem[46] = 0xc0
	c := New(mem)
	c.R[EAX] = 8
	c.R[EDI] = 32
	c.EFlags = IF | CF | OF
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 79})
	if err := c.Step(); err != nil || c.EFlags != IF || mem[46] != 0xc0 {
		t.Fatal("TEST SIB")
	}
	c.EIP = 0
	flags := c.EFlags
	c.SetDescriptor(0x160, Descriptor{Limit: 45})
	if err := c.Step(); err == nil || c.EFlags != flags {
		t.Fatal("TEST拒絕")
	}
}

func TestMOVZXByteToWord(t *testing.T) {
	c := New(testBus{0x66, 15, 0xb6, 0xcc})
	c.R[ECX] = 0xabcdffff
	c.R[EAX] = 0x9200
	c.EFlags = IF | CF
	if err := c.Step(); err != nil || c.R[ECX] != 0xabcd0092 || c.EFlags != IF|CF {
		t.Fatal("MOVZX word高位")
	}
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0x66, 15, 0xb6, 8})
	mem[32] = 0xff
	c = New(mem)
	c.R[EAX] = 32
	c.R[ECX] = 0xabcdffff
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	if err := c.Step(); err != nil || c.R[ECX] != 0xabcd00ff {
		t.Fatal("MOVZX word記憶體")
	}
}

func TestORByteMemory(t *testing.T) {
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0x80, 8, 0x80})
	mem[32] = 1
	mem[33] = 7
	c := New(mem)
	c.R[EAX] = 32
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63, Writable: true})
	c.EFlags = IF | CF | OF
	if err := c.Step(); err != nil || mem[32] != 0x81 || mem[33] != 7 || c.EFlags != IF|SF|PF {
		t.Fatal("OR byte")
	}
	c.EIP = 0
	mem[32] = 1
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	flags := c.EFlags
	if err := c.Step(); err == nil || mem[32] != 1 || c.EFlags != flags {
		t.Fatal("OR唯讀拒絕")
	}
}

func TestWordCompareAndXORMemory(t *testing.T) {
	c := New(testBus{0x66, 0x83, 0xff, 0xff})
	c.R[EDI] = 0xabcdffff
	c.EFlags = IF | CF | OF
	if err := c.Step(); err != nil || c.R[EDI] != 0xabcdffff || c.EFlags != IF|ZF|PF {
		t.Fatal("word CMP有號即值")
	}
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0x33, 5, 32, 0, 0, 0})
	mem[35] = 0x80
	c = New(mem)
	c.R[EAX] = 3
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	c.EFlags = IF | CF | OF
	if err := c.Step(); err != nil || c.R[EAX] != 0x80000003 || c.EFlags != IF|SF|PF {
		t.Fatal("XOR記憶體")
	}
	c.EIP = 0
	flags := c.EFlags
	c.SetDescriptor(0x160, Descriptor{Limit: 34})
	if err := c.Step(); err == nil || c.R[EAX] != 0x80000003 || c.EFlags != flags {
		t.Fatal("XOR來源拒絕")
	}
}

func TestMOVSXByteWidths(t *testing.T) {
	for _, word := range []bool{false, true} {
		code := []byte{15, 0xbe, 0xcc}
		want := uint32(0xffffff80)
		if word {
			code = append([]byte{0x66}, code...)
			want = 0xabcdff80
		}
		c := New(testBus(code))
		c.R[EAX] = 0x8000
		c.R[ECX] = 0xabcd0000
		c.EFlags = IF | CF
		if e := c.Step(); e != nil || c.R[ECX] != want || c.EFlags != IF|CF {
			t.Fatal("MOVSX byte寬度")
		}
	}
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0x66, 15, 0xbe, 5, 32, 0, 0, 0})
	mem[32] = 0xfe
	c := New(mem)
	c.R[EAX] = 0xabcd0000
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	if e := c.Step(); e != nil || c.R[EAX] != 0xabcdfffe {
		t.Fatal("MOVSX來源")
	}
}

func TestXORByteMemory(t *testing.T) {
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0x80, 0x35, 32, 0, 0, 0, 0x80})
	mem[32] = 0x80
	c := New(mem)
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63, Writable: true})
	c.EFlags = IF | CF | OF
	if e := c.Step(); e != nil || mem[32] != 0 || c.EFlags != IF|ZF|PF {
		t.Fatal("XOR byte抵消")
	}
	c.EIP = 0
	if e := c.Step(); e != nil || mem[32] != 0x80 || c.EFlags != IF|SF {
		t.Fatal("XOR byte符號")
	}
	c.EIP = 0
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	flags := c.EFlags
	if e := c.Step(); e == nil || mem[32] != 0x80 || c.EFlags != flags {
		t.Fatal("XOR byte唯讀")
	}
}

func TestByteORAndDECRegisters(t *testing.T) {
	c := New(testBus{0x0a, 0xff})
	c.R[EBX] = 0xabcd0007
	c.EFlags = IF | CF | OF
	if e := c.Step(); e != nil || c.R[EBX] != 0xabcd0007 || c.EFlags != IF|ZF|PF {
		t.Fatal("OR BH零")
	}
	c = New(testBus{0x0a, 0xe2})
	c.R[EAX] = 0xabcd8007
	c.R[EDX] = 1
	c.EFlags = IF | CF | OF
	if e := c.Step(); e != nil || c.R[EAX] != 0xabcd8107 || c.EFlags != IF|SF|PF {
		t.Fatal("OR AH")
	}
	c = New(testBus{0xfe, 0xcb})
	c.R[EBX] = 0xabcd1280
	c.EFlags = IF | CF
	if e := c.Step(); e != nil || c.R[EBX] != 0xabcd127f || c.EFlags != IF|CF|OF|AF {
		t.Fatal("DEC BL")
	}
}

func TestPUSHADPOPADStackContract(t *testing.T) {
	mem := testBus(make([]byte, 160))
	copy(mem, []byte{0x60, 0x61})
	c := New(mem)
	c.R = [8]uint32{1, 2, 3, 4, 128, 6, 7, 8}
	c.Seg[SegSS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 143, Writable: true})
	original := c.R
	c.EFlags = IF | CF | OF
	if e := c.Step(); e != nil || c.R[ESP] != 96 {
		t.Fatalf("PUSHAD %v", e)
	}
	for i, w := range []uint32{8, 7, 6, 128, 4, 3, 2, 1} {
		v, ok := c.readSegment32(0x160, 96+uint32(i)*4)
		if !ok || v != w {
			t.Fatal("堆疊順序")
		}
	}
	c.writeSegment32(0x160, 108, 0xffffffff)
	for _, r := range []int{EAX, ECX, EDX, EBX, EBP, ESI, EDI} {
		c.R[r] = 999
	}
	if e := c.Step(); e != nil || c.R != original || c.EFlags != IF|CF|OF {
		t.Fatalf("POPAD %v %v", e, c.R)
	}
	c.EIP = 0
	c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 143})
	if e := c.Step(); e == nil || c.R != original {
		t.Fatal("PUSHAD唯讀拒絕")
	}
	c.EIP = 1
	c.R[ESP] = 120
	before := c.R
	if e := c.Step(); e == nil || c.R != before {
		t.Fatal("POPAD越界拒絕")
	}
}
func TestWordSUBMemoryAndMoffsByte(t *testing.T) {
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0x66, 0x2b, 5, 32, 0, 0, 0, 0xa0, 32, 0, 0, 0})
	mem[32] = 1
	c := New(mem)
	c.R[EAX] = 0xabcd0000
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	c.EFlags = IF
	if e := c.Step(); e != nil || c.R[EAX] != 0xabcdffff || c.EFlags != IF|CF|PF|AF|SF {
		t.Fatal("word SUB memory")
	}
	flags := c.EFlags
	if e := c.Step(); e != nil || c.R[EAX] != 0xabcdff01 || c.EFlags != flags {
		t.Fatal("A0高位保存")
	}
	c.EIP = 0
	c.SetDescriptor(0x160, Descriptor{Limit: 32})
	before := c.R
	if e := c.Step(); e == nil || c.R != before || c.EFlags != flags {
		t.Fatal("SUB word跨界")
	}
}

func TestIDIVMemorySignedBoundaries(t *testing.T) {
	for _, v := range []struct {
		a      int64
		b      int32
		q, r   int32
		reject bool
	}{
		{-7, 3, -2, -1, false}, {7, -3, -2, 1, false}, {-7, -3, 2, -1, false},
		{-2147483648, 1, -2147483648, 0, false}, {2147483648, 1, 0, 0, true},
		{-1 << 63, -1, 0, 0, true}, {1, 0, 0, 0, true},
	} {
		mem := testBus(make([]byte, 96))
		copy(mem, []byte{0xf7, 0x7c, 0x24, 0x20})
		c := New(mem)
		c.R[ESP] = 16
		c.R[EAX] = uint32(v.a)
		c.R[EDX] = uint32(uint64(v.a) >> 32)
		c.Seg[SegSS] = 0x160
		c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 79, Writable: true})
		c.writeSegment32(0x160, 48, uint32(v.b))
		c.EFlags = IF | CF | OF
		before := c.R
		e := c.Step()
		if v.reject {
			if e == nil || c.R != before {
				t.Fatal("IDIV拒絕")
			}
		} else if e != nil || c.R[EAX] != uint32(v.q) || c.R[EDX] != uint32(v.r) {
			t.Fatalf("IDIV %v %v", v, e)
		}
		if c.EFlags != IF|CF|OF {
			t.Fatal("IDIV旗標保存")
		}
	}
}

func TestDECByteMemory(t *testing.T) {
	mem := testBus(make([]byte, 64))
	copy(mem, []byte{0xfe, 0x48, 1})
	mem[33] = 0x80
	c := New(mem)
	c.R[EAX] = 32
	c.Seg[SegDS] = 0x160
	c.SetDescriptor(0x160, Descriptor{Limit: 63, Writable: true})
	c.EFlags = IF | CF
	if e := c.Step(); e != nil || mem[33] != 0x7f || c.EFlags != IF|CF|OF|AF {
		t.Fatal("DEC memory")
	}
	c.EIP = 0
	c.SetDescriptor(0x160, Descriptor{Limit: 63})
	flags := c.EFlags
	if e := c.Step(); e == nil || mem[33] != 0x7f || c.EFlags != flags {
		t.Fatal("DEC唯讀")
	}
}

func TestMOVSXWordRegister(t *testing.T) {
	for _, v := range []uint32{0, 0x7fff, 0x8000, 0xffff} {
		for _, m := range []byte{0xc3, 0xdb} {
			c := New(testBus{0x0f, 0xbf, m})
			c.R[EBX] = 0xabcd0000 | v
			c.EFlags = IF | CF | OF
			if e := c.Step(); e != nil || c.R[(m>>3)&7] != uint32(int32(int16(v))) || c.EFlags != IF|CF|OF {
				t.Fatal("MOVSX word暫存器")
			}
		}
	}
}

func TestDECWordPreservesHighAndCarry(t *testing.T) {
	for _, v := range []struct{ a, w, flags uint32 }{{0, 0xffff, SF | PF | AF}, {0x8000, 0x7fff, OF | AF | PF}, {1, 0, ZF | PF}} {
		for _, carry := range []uint32{0, CF} {
			c := New(testBus{0x66, 0x4a})
			c.R[EDX] = 0xabcd0000 | v.a
			c.EFlags = IF | carry
			if e := c.Step(); e != nil || c.R[EDX] != 0xabcd0000|v.w || c.EFlags != IF|carry|v.flags {
				t.Fatal("DEC word")
			}
		}
	}
}

func TestXORWordRegister(t *testing.T) {
	c := New(testBus{0x66, 0x33, 0xc0})
	c.R[EAX] = 0xabcd8001
	c.EFlags = IF | CF | OF
	if e := c.Step(); e != nil || c.R[EAX] != 0xabcd0000 || c.EFlags != IF|ZF|PF {
		t.Fatal("word XOR零")
	}
	c = New(testBus{0x66, 0x33, 0xc3})
	c.R[EAX] = 0xabcd0001
	c.R[EBX] = 0x8000
	c.EFlags = IF | CF | OF
	if e := c.Step(); e != nil || c.R[EAX] != 0xabcd8001 || c.R[EBX] != 0x8000 || c.EFlags != IF|SF {
		t.Fatal("word XOR符號")
	}
}

func TestSUBHighByteImmediate(t *testing.T) {
	c := New(testBus{0x80, 0xec, 0xc1})
	c.R[EAX] = 0xabcdc307
	c.EFlags = IF | CF | OF
	if e := c.Step(); e != nil || c.R[EAX] != 0xabcd0207 || c.EFlags != IF {
		t.Fatal("SUB AH")
	}
	c.EIP = 0
	c.R[EAX] = 0xabcd0007
	if e := c.Step(); e != nil || c.R[EAX] != 0xabcd3f07 || c.EFlags != IF|CF|PF|AF {
		t.Fatal("SUB AH借位")
	}
}

func TestByteExchangeAndWordShiftOne(t *testing.T) {
	c := New(testBus{0x86, 0xc4})
	c.R[EAX] = 0xabcd1234
	c.EFlags = IF | OF | CF
	if e := c.Step(); e != nil || c.R[EAX] != 0xabcd3412 || c.EFlags != IF|OF|CF {
		t.Fatal("XCHG AL AH")
	}
	for _, v := range []struct {
		m    byte
		w, f uint32
	}{{0xe0, 0, CF | OF | ZF | PF}, {0xe8, 0x4000, OF | PF}, {0xf8, 0xc000, SF | PF}, {0xc0, 1, CF | OF}} {
		c = New(testBus{0x66, 0xd1, v.m})
		c.R[EAX] = 0xabcd8000
		c.EFlags = IF
		if e := c.Step(); e != nil || c.EIP != 3 || c.R[EAX] != 0xabcd0000|v.w || c.EFlags != IF|v.f {
			t.Fatalf("D1word %v %v %X", v, e, c.EFlags)
		}
	}
}

func TestAccumulatorSignExtension(t *testing.T) {
	for _, v := range []struct {
		code []byte
		a, w uint32
	}{
		{[]byte{0x98}, 0xabcd8000, 0xffff8000}, {[]byte{0x98}, 0xabcd7fff, 0x7fff},
		{[]byte{0x66, 0x98}, 0xabcd1280, 0xabcdff80}, {[]byte{0x66, 0x98}, 0xabcdff7f, 0xabcd007f},
	} {
		c := New(testBus(v.code))
		c.R[EAX] = v.a
		c.EFlags = IF | CF | OF
		if e := c.Step(); e != nil || c.R[EAX] != v.w || c.EFlags != IF|CF|OF {
			t.Fatal("98寬度")
		}
	}
}

func TestANDByteSIBUsesSSAndPreservesNeighbours(t *testing.T) {
	for _, v := range []struct {
		a, mask, w byte
		flags      uint32
	}{
		{0xf3, 0x7f, 0x73, 0}, {0x80, 0x7f, 0, ZF | PF}, {0x80, 0xff, 0x80, SF},
	} {
		mem := testBus(make([]byte, 256))
		copy(mem, []byte{0x80, 0x64, 0x24, 0x54, v.mask})
		mem[115], mem[116], mem[117], mem[228] = 0x11, v.a, 0x22, 0xaa
		c := New(mem)
		c.R[ESP] = 16
		c.Seg[SegSS] = 0x160
		c.Seg[SegDS] = 0x168
		c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 200, Writable: true})
		c.SetDescriptor(0x168, Descriptor{Base: 128, Limit: 100, Writable: true})
		c.EFlags = IF | DF | CF | OF
		regs := c.R
		if e := c.Step(); e != nil || mem[116] != v.w || c.EFlags != IF|DF|v.flags || c.R != regs || c.EIP != 5 {
			t.Fatalf("AND SIB: %v flags=%x bytes=%x", e, c.EFlags, mem[115:118])
		}
		if mem[115] != 0x11 || mem[117] != 0x22 || mem[228] != 0xaa {
			t.Fatal("AND污染相鄰byte或錯用DS")
		}
		c.EIP = 0
		c.SetDescriptor(0x160, Descriptor{Base: 16, Limit: 200})
		flags := c.EFlags
		if e := c.Step(); e == nil || mem[116] != v.w || c.EFlags != flags {
			t.Fatal("AND唯讀拒絕未保存狀態")
		}
	}
}
