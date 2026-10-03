package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 343：ZF／SF／OF八列真值表，逐欄索引，不呼叫CPU條件helper。
var setleTruth = [8]byte{0, 1, 1, 1, 1, 1, 0, 1}

func setleExpectedByte(flags uint32) byte {
	index := 0
	if flags&ZF != 0 {
		index += 1
	}
	if flags&SF != 0 {
		index += 2
	}
	if flags&OF != 0 {
		index += 4
	}
	return setleTruth[index]
}

// 以little-endian byte陣列替換，獨立核算低／高byte別名與鄰居保持。
func setleReplaceByte(r [8]uint32, dest byte, value byte) [8]uint32 {
	var raw [4]byte
	register, position := int(dest%4), int(dest/4)
	binary.LittleEndian.PutUint32(raw[:], r[register])
	raw[position] = value
	r[register] = binary.LittleEndian.Uint32(raw[:])
	return r
}

func TestSETLERegisterAllAliasesFlagsIgnoredFieldsAndBytes(t *testing.T) {
	for _, context := range []uint32{2 | IF | DF | 0x200000, 2 | 0x40000} {
		for combination := 0; combination < 64; combination++ {
			flags := context
			for i, flag := range []uint32{CF, PF, AF, ZF, SF, OF} {
				if combination/(1<<i)%2 != 0 {
					flags |= flag
				}
			}
			for ignored := byte(0); ignored < 8; ignored++ {
				for dest := byte(0); dest < 8; dest++ {
					for initial := 0; initial < 256; initial++ {
						code := []byte{0x0f, 0x9e, 0xc0 | ignored<<3 | dest}
						c, mem := inImmediateFixture(code)
						c.R = setleReplaceByte(c.R, dest, byte(initial))
						c.EFlags = flags
						want := snapshotInImmediate(c)
						want.r = setleReplaceByte(want.r, dest, setleExpectedByte(flags))
						bus := &subByteFailureBus{memory: mem, failWrite: true}
						c.Bus = bus
						if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) || bus.writes != 0 {
							t.Fatalf("flags%X ignored%d dest%d initial%X：%v", flags, ignored, dest, initial, err)
						}
					}
				}
			}
		}
	}
}

func TestSETLERegisterSignedCMPConsumer(t *testing.T) {
	values := []uint32{0, 1, 2, 0xffffffff, 0xfffffffe, 0x7fffffff, 0x7ffffffe, 0x80000000, 0x80000001, 0x55555555, 0xaaaaaaaa, 0x00010000, 0xffff0000}
	for n := uint(0); n < 32; n++ {
		values = append(values, 1<<n, ^uint32(1<<n))
	}
	for _, a := range values {
		for _, b := range values {
			for dest := byte(0); dest < 8; dest++ {
				code := []byte{0x39, 0xd8, 0x0f, 0x9e, 0xc0 | dest}
				c, mem := inImmediateFixture(code)
				c.R[EAX], c.R[EBX] = a, b
				before := append([]byte(nil), mem...)
				registers := c.R
				if err := c.Step(); err != nil || c.EIP != 2 || c.R != registers {
					t.Fatalf("CMP：%v", err)
				}
				difference := int64(int32(a)) - int64(int32(b))
				result := uint32(difference)
				wantComparison := uint32(0)
				if difference == 0 {
					wantComparison |= ZF
				}
				if result >= 1<<31 {
					wantComparison |= SF
				}
				if difference < -(1<<31) || difference > (1<<31)-1 {
					wantComparison |= OF
				}
				if c.EFlags&(ZF|SF|OF) != wantComparison {
					t.Fatalf("signed CMP %X−%X flags%X", a, b, c.EFlags)
				}
				value := byte(0)
				if int64(int32(a)) <= int64(int32(b)) {
					value = 1
				}
				want := snapshotInImmediate(c)
				want.r = setleReplaceByte(want.r, dest, value)
				if err := c.Step(); err != nil || c.EIP != 5 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
					t.Fatalf("signed LE %X<=%X dest%d：%v", a, b, dest, err)
				}
			}
		}
	}
}

func TestSETLERegisterRejectsWithoutDestinationMutation(t *testing.T) {
	for _, code := range [][]byte{{}, {0x0f}, {0x0f, 0x9e}} {
		c, mem := inImmediateFixture(code)
		want := snapshotInImmediate(c)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
			t.Fatalf("截短%X：%v", code, err)
		}
	}
	for mod := byte(0); mod < 3; mod++ {
		for ignored := byte(0); ignored < 8; ignored++ {
			for rm := byte(0); rm < 8; rm++ {
				code := []byte{0x0f, 0x9e, mod<<6 | ignored<<3 | rm, 0, 32, 0, 0, 0}
				c, mem := subByteMemoryFixture(code)
				want := snapshotInImmediate(c)
				before := append([]byte(nil), mem...)
				bus := &subByteFailureBus{memory: mem, failWrite: true}
				c.Bus = bus
				if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != 0 {
					t.Fatalf("memory%X：%v", code, err)
				}
			}
		}
	}
	for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf0, 0xf2, 0xf3} {
		for dest := byte(0); dest < 8; dest++ {
			code := []byte{prefix, 0x0f, 0x9e, 0xc0 | dest}
			c, mem := inImmediateFixture(code)
			want := snapshotInImmediate(c)
			if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
				t.Fatalf("prefix%X dest%d：%v", prefix, dest, err)
			}
		}
	}
	for opcode := byte(0x90); opcode <= 0x9f; opcode++ {
		if opcode == 0x94 || opcode == 0x95 || opcode == 0x9e {
			continue
		}
		code := []byte{0x0f, opcode, 0xc0}
		c, mem := inImmediateFixture(code)
		want := snapshotInImmediate(c)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
			t.Fatalf("既有拒絕0F%X：%v", opcode, err)
		}
	}
}

func TestSETLERegisterKeepsSETEAndSETNE(t *testing.T) {
	for _, opcode := range []byte{0x94, 0x95} {
		for _, zf := range []uint32{0, ZF} {
			for dest := byte(0); dest < 8; dest++ {
				for ignored := byte(0); ignored < 8; ignored++ {
					code := []byte{0x0f, opcode, 0xc0 | ignored<<3 | dest}
					c, mem := inImmediateFixture(code)
					c.EFlags = (c.EFlags &^ ZF) | zf
					value := byte(0)
					if (opcode == 0x94 && zf != 0) || (opcode == 0x95 && zf == 0) {
						value = 1
					}
					want := snapshotInImmediate(c)
					want.r = setleReplaceByte(want.r, dest, value)
					if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
						t.Fatalf("0F%X zf%X dest%d：%v", opcode, zf, dest, err)
					}
				}
			}
		}
	}
}

func TestSETLERegisterFollowingSSByteStoreAndFailures(t *testing.T) {
	for _, flags := range []uint32{0x206, 0x246} {
		for _, kind := range []string{"成功", "未知段", "唯讀", "越界", "bus寫失敗"} {
			code := []byte{0x0f, 0x9e, 0xc0, 0x88, 0x45, 0xfc}
			c, mem := subByteMemoryFixture(code)
			c.R[EBP] = 40
			c.EFlags = flags
			c.R[EAX] = 0x12345628
			mem[803], mem[804], mem[805], mem[806] = 0x11, 0xf7, 0x33, 0x44
			mem[292] = 0x55
			bus := &subByteFailureBus{memory: mem}
			c.Bus = bus
			if err := c.Step(); err != nil || c.EIP != 3 {
				t.Fatalf("SETLE：%v", err)
			}
			if byte(c.R[EAX]) != setleExpectedByte(flags) || c.EFlags != flags {
				t.Fatal("SETLE前置結果")
			}
			switch kind {
			case "未知段":
				c.Seg[SegSS] = 0x200
			case "唯讀":
				c.SetDescriptor(0x190, Descriptor{Base: 768, Limit: 255})
			case "越界":
				c.SetDescriptor(0x190, Descriptor{Base: 768, Limit: 35, Writable: true})
			case "bus寫失敗":
				bus.failWrite = true
			}
			want := snapshotInImmediate(c)
			before := append([]byte(nil), mem...)
			err := c.Step()
			writes := 0
			if kind == "成功" || kind == "bus寫失敗" {
				writes = 1
			}
			if kind == "成功" {
				before[804] = setleExpectedByte(flags)
				if err != nil || c.EIP != 6 {
					t.Fatalf("MOV：%v", err)
				}
			} else if err == nil {
				t.Fatalf("%s沒有拒絕", kind)
			}
			if snapshotInImmediate(c) != want || !bytes.Equal(mem, before) || bus.writes != writes {
				t.Fatalf("%s核心／鄰居／DS保持 writes%d", kind, bus.writes)
			}
		}
	}
}
