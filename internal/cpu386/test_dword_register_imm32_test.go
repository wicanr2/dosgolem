package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func testDwordRegisterIntersection(value, mask uint32) uint32 {
	var result uint32
	for bit := uint(0); bit < 32; bit++ {
		selected := uint32(1) << bit
		if value&selected != 0 && mask&selected != 0 {
			result |= selected
		}
	}
	return result
}

func testDwordRegisterDefinedFlags(result, initial uint32) uint32 {
	flags := initial &^ (CF | PF | ZF | SF | OF)
	if result == 0 {
		flags |= ZF
	}
	if result >= 0x80000000 {
		flags |= SF
	}
	ones := 0
	for value := int(byte(result)); value > 0; value /= 2 {
		ones += value % 2
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return flags
}

func checkTestDwordRegister(t *testing.T, reg byte, value, mask, initial uint32) {
	t.Helper()
	code := binary.LittleEndian.AppendUint32([]byte{0xf7, 0xc0 | reg}, mask)
	c, mem := inImmediateFixture(code)
	c.R[reg] = value
	c.EFlags = initial
	want := snapshotInImmediate(c)
	result := testDwordRegisterIntersection(value, mask)
	defined := testDwordRegisterDefinedFlags(result, initial)
	want.flags = defined &^ AF
	if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("reg=%d value=%X mask=%X：%v", reg, value, mask, err)
	}
	if c.EFlags&^AF != defined&^AF {
		t.Fatal("定義五旗標")
	}
	if c.EFlags&AF != 0 {
		t.Fatal("AF 清除只驗工具模型")
	}
}

func TestTESTDwordRegisterImmediateAllBitsAndLowByteMasks(t *testing.T) {
	initialFlags := []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF}
	values := []uint32{0, 1, 0xffffffff, 0x7fffffff, 0x80000000, 0x80000001, 0x00010000, 0xffff0000, 0xaaaaaaaa, 0x55555555}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, uint32(1)<<bit, ^(uint32(1) << bit))
	}
	for reg := byte(0); reg < 8; reg++ {
		for _, value := range values {
			for _, mask := range values {
				for _, initial := range initialFlags {
					checkTestDwordRegister(t, reg, value, mask, initial)
				}
			}
		}
		for a := uint32(0); a < 256; a++ {
			for b := uint32(0); b < 256; b++ {
				for _, initial := range initialFlags {
					checkTestDwordRegister(t, reg, a, b, initial)
				}
			}
		}
	}
}

func TestTESTDwordRegisterImmediateRejectsWithoutPublishingFlags(t *testing.T) {
	for reg := byte(0); reg < 8; reg++ {
		code := []byte{0xf7, 0xc0 | reg, 0, 0, 0, 0x80}
		cases := [][]byte{{0xf7, 0xc8 | reg}}
		for n := 0; n < len(code); n++ {
			cases = append(cases, append([]byte(nil), code[:n]...))
		}
		for _, prefix := range []byte{0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
			cases = append(cases, append([]byte{prefix}, code...))
		}
		for _, instruction := range cases {
			c, mem := inImmediateFixture(instruction)
			want := snapshotInImmediate(c)
			before := append([]byte(nil), mem...)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
				t.Fatalf("截短／前綴／未知 group %X：%v", instruction, err)
			}
		}
	}
}

func TestTESTDwordRegisterImmediateOriginalStateAndJNZ(t *testing.T) {
	code := []byte{0xf7, 0xc1, 0, 0, 0, 0x80, 0x75, 0x2a}
	c, mem := inImmediateFixture(code)
	c.R = [8]uint32{0x80000929, 0x400, 0x6bbc60, 0x3ff09, 0x2bda88, 0, 0x6bce48, 0x609070}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x213
	want := snapshotInImmediate(c)
	want.flags = 0x246
	if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want {
		t.Fatalf("原始 TEST：%v", err)
	}
	if err := c.Step(); err != nil || c.EIP != 8 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("原始 JNZ 不跳：%v", err)
	}
	c, mem = inImmediateFixture(code)
	c.R[ECX] = 0x80000400
	want = snapshotInImmediate(c)
	want.flags = testDwordRegisterDefinedFlags(0x80000000, c.EFlags) &^ AF
	if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want {
		t.Fatalf("非零 TEST：%v", err)
	}
	if err := c.Step(); err != nil || c.EIP != 50 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
		t.Fatalf("非零 JNZ 跳：%v", err)
	}
}

func TestTESTDwordRegisterImmediateKeepsWordMemoryAndOtherGroups(t *testing.T) {
	c, mem := inImmediateFixture([]byte{0x66, 0xf7, 0xc1, 0, 0x80})
	c.R[ECX] = 0x80000400
	want := snapshotInImmediate(c)
	want.flags = testDwordRegisterDefinedFlags(0, c.EFlags) &^ AF
	if err := c.Step(); err != nil || c.EIP != 5 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0x66, 0xf7, 0xc1, 0, 0x80}) {
		t.Fatalf("既有 word TEST：%v", err)
	}
	c, mem = subByteMemoryFixture([]byte{0xf7, 0x05, 32, 0, 0, 0, 0, 0, 0, 0x80})
	binary.LittleEndian.PutUint32(mem[288:], 0x80000400)
	before := append([]byte(nil), mem...)
	want = snapshotInImmediate(c)
	want.flags = testDwordRegisterDefinedFlags(0x80000000, c.EFlags) &^ AF
	if err := c.Step(); err != nil || c.EIP != 10 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("既有 memory TEST：%v", err)
	}
	c, mem = inImmediateFixture([]byte{0xf7, 0xd1})
	want = snapshotInImmediate(c)
	want.r[ECX] = ^want.r[ECX]
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0xf7, 0xd1}) {
		t.Fatalf("既有 NOT group：%v", err)
	}
}
