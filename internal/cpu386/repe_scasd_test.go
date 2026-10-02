package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 與 sub32 分開，按寬值差、nibble 借位與有號範圍建立六旗標 oracle。
func scasdExpectedFlags(left, right, initial uint32) uint32 {
	difference := int64(left) - int64(right)
	result := uint32(difference)
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	if difference < 0 {
		flags |= CF
	}
	if left%16 < right%16 {
		flags |= AF
	}
	signed := int64(int32(left)) - int64(int32(right))
	if signed < -2147483648 || signed > 2147483647 {
		flags |= OF
	}
	if result == 0 {
		flags |= ZF
	}
	if int32(result) < 0 {
		flags |= SF
	}
	ones := 0
	for n := int(byte(result)); n != 0; n /= 2 {
		ones += n % 2
	}
	if ones%2 == 0 {
		flags |= PF
	}
	return flags
}

func scasdFixture(code []byte) (*CPU, testBus) {
	c, mem := subByteMemoryFixture(code)
	c.Seg[SegES] = 0x188
	c.Seg[SegDS] = 0x190
	// 掃描只讀；ES 與 DS 的 base 分離，避免錯用 DS 仍通過。
	c.SetDescriptor(0x188, Descriptor{Base: 256, Limit: 255})
	c.R[EDI], c.R[ECX] = 32, 1
	return c, mem
}

func TestREPESCASDIndependentSixFlagsAndPreservation(t *testing.T) {
	values := []uint32{0, 1, 2, 15, 16, 127, 128, 255, 256, 0x7fffffff, 0x80000000, 0x80000001, 0xffff0000, 0xfffffff0, 0xffffffff, 0x55555555, 0xaaaaaaaa, 0x12345678, 0x87654321}
	arithmetic := []uint32{CF, PF, AF, ZF, SF, OF}
	for _, left := range values {
		for _, right := range values {
			for combination := 0; combination < 64; combination++ {
				for _, direction := range []uint32{0, DF} {
					initial := uint32(2|IF|0x200000) | direction
					for bit, flag := range arithmetic {
						if combination/(1<<bit)%2 != 0 {
							initial |= flag
						}
					}
					c, mem := scasdFixture([]byte{0xf3, 0xaf})
					c.R[EAX], c.EFlags = left, initial
					binary.LittleEndian.PutUint32(mem[288:292], right)
					binary.LittleEndian.PutUint32(mem[800:804], right+1)
					before := append([]byte(nil), mem...)
					want := snapshotInImmediate(c)
					want.r[ECX] = 0
					want.r[EDI] = 36
					if direction != 0 {
						want.r[EDI] = 28
					}
					want.flags = scasdExpectedFlags(left, right, initial)
					if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
						t.Fatalf("left=%X right=%X initial=%X：%v", left, right, initial, err)
					}
				}
			}
		}
	}
}

func TestREPESCASDCountTerminationDirectionAndWrap(t *testing.T) {
	for _, count := range []uint32{0, 1, 2, 17, 2048, 65536} {
		for _, direction := range []uint32{0, DF} {
			for _, initialZF := range []uint32{0, ZF} {
				for _, mismatch := range []int64{-1, 0, int64(count / 2), int64(count) - 1} {
					c, mem := inImmediateFixture([]byte{0xf3, 0xaf})
					start := uint32(0xfffffffc)
					if direction != 0 {
						start = 0
					}
					c.R[EAX], c.R[ECX], c.R[EDI] = 0xf1234567, count, start
					c.EFlags = 2 | IF | AF | CF | OF | direction | initialZF
					want := snapshotInImmediate(c)
					processed := count
					if mismatch >= 0 && mismatch < int64(count) {
						processed = uint32(mismatch) + 1
					}
					calls := uint32(0)
					c.SegmentRead8 = func(selector uint16, offset uint32) (byte, bool) {
						// 按線性元素編號建立資料，逐 byte 核對所有讀取順序。
						position, byteIndex := calls/4, calls%4
						element := start + position*4
						if direction != 0 {
							element = start - position*4
						}
						if selector != 0x188 || offset != element+byteIndex || position >= processed {
							t.Fatalf("讀取越過退出元素：selector=%X offset=%X calls=%d", selector, offset, calls)
						}
						value := uint32(0xf1234567)
						if int64(position) == mismatch {
							value = 0x81234567
						}
						calls++
						return byte(value >> (8 * byteIndex)), true
					}
					want.r[ECX] = count - processed
					want.r[EDI] = uint32((uint64(start) + uint64(processed)*4) % (1 << 32))
					if direction != 0 {
						want.r[EDI] = uint32(int64(start) - int64(processed)*4)
					}
					if processed != 0 {
						last := uint32(0xf1234567)
						if mismatch >= 0 && mismatch < int64(count) {
							last = 0x81234567
						}
						want.flags = scasdExpectedFlags(c.R[EAX], last, c.EFlags)
					}
					if err := c.Step(); err != nil || c.EIP != 2 || calls != processed*4 || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0xf3, 0xaf}) {
						t.Fatalf("count=%d mismatch=%d direction=%X ZF=%X：%v calls=%d", count, mismatch, direction, initialZF, err, calls)
					}
				}
			}
		}
	}
}

func TestREPESCASDZeroCountDoesNotReadInvalidES(t *testing.T) {
	c, mem := scasdFixture([]byte{0xf3, 0xaf})
	c.Seg[SegES], c.R[ECX], c.R[EDI] = 0x200, 0, 0xffffffff
	c.SegmentRead8 = func(uint16, uint32) (byte, bool) { t.Fatal("零計數不可讀取 ES"); return 0, false }
	before := append([]byte(nil), mem...)
	want := snapshotInImmediate(c)
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("零計數：%v", err)
	}
}

func TestREPESCASDUnalignedAndLastWholeDword(t *testing.T) {
	for _, offset := range []uint32{31, 252} {
		c, mem := scasdFixture([]byte{0xf3, 0xaf})
		c.R[EDI], c.R[EAX] = offset, 0x12345678
		c.EFlags &^= DF
		binary.LittleEndian.PutUint32(mem[256+offset:260+offset], 0x12345677)
		before := append([]byte(nil), mem...)
		want := snapshotInImmediate(c)
		want.r[ECX], want.r[EDI] = 0, offset+4
		want.flags = scasdExpectedFlags(0x12345678, 0x12345677, c.EFlags)
		if err := c.Step(); err != nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("非對齊／段末 offset=%X：%v", offset, err)
		}
	}
}

func TestREPESCASDReadFailureRestoresFlagsAndKeepsCompletedProgress(t *testing.T) {
	for completed := uint32(0); completed < 2; completed++ {
		for part := uint32(0); part < 4; part++ {
			c, mem := scasdFixture([]byte{0xf3, 0xaf})
			c.R[EAX], c.R[ECX], c.EFlags = 0xffffffff, 3, 2|IF|CF|AF|OF|SF
			for offset := 288; offset < 300; offset += 4 {
				binary.LittleEndian.PutUint32(mem[offset:offset+4], 0xffffffff)
			}
			bus := &subByteFailureBus{memory: mem, target: 288 + completed*4 + part, failRead: true}
			c.Bus = bus
			before := append([]byte(nil), mem...)
			want := snapshotInImmediate(c)
			want.r[ECX], want.r[EDI] = 3-completed, 32+4*completed
			if err := c.Step(); err == nil || c.EIP != 2 || snapshotInImmediate(c) != want || bus.writes != 0 || !bytes.Equal(mem, before) {
				t.Fatalf("第%d個元素 byte%d 拒絕：%v", completed, part, err)
			}
		}
	}
	for _, kind := range []string{"未知段", "段末截短", "bus越界"} {
		c, mem := scasdFixture([]byte{0xf3, 0xaf})
		switch kind {
		case "未知段":
			c.Seg[SegES] = 0x200
		case "段末截短":
			c.R[EDI] = 253
		case "bus越界":
			c.SetDescriptor(0x188, Descriptor{Base: 1022, Limit: 255})
		}
		before := append([]byte(nil), mem...)
		want := snapshotInImmediate(c)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("%s：%v", kind, err)
		}
	}
}

func TestREPESCASDRejectsOtherShapesBeforeData(t *testing.T) {
	codes := [][]byte{{}, {0xf3}, {0xaf}, {0xf2, 0xaf}, {0xf3, 0xf3, 0xaf}, {0xf2, 0xf3, 0xaf}, {0xf3, 0xf2, 0xaf}}
	for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf0} {
		codes = append(codes, []byte{prefix, 0xf3, 0xaf}, []byte{0xf3, prefix, 0xaf})
	}
	for _, code := range codes {
		c, mem := inImmediateFixture(code)
		c.SegmentRead8 = func(uint16, uint32) (byte, bool) { t.Fatal("拒絕形狀不可讀資料"); return 0, false }
		want := snapshotInImmediate(c)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
			t.Fatalf("拒絕形狀 %X：%v", code, err)
		}
	}
}

func TestREPESCASDKeepsExistingSCASB(t *testing.T) {
	for _, code := range [][]byte{{0xae}, {0xf3, 0xae}, {0xf2, 0xae}} {
		c, mem := scasdFixture(code)
		c.R[EAX], c.R[ECX], c.EFlags = 0x12345680, 1, 2|IF
		mem[288] = 0x7f
		before := append([]byte(nil), mem...)
		want := snapshotInImmediate(c)
		want.r[EDI] = 33
		if len(code) == 2 {
			want.r[ECX] = 0
		}
		want.flags = subByteMemoryFlags(0x80, 0x7f, c.EFlags)
		if err := c.Step(); err != nil || c.EIP != uint32(len(code)) || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("既有 SCASB %X：%v", code, err)
		}
	}
}

func TestREPESCASDOriginalScanSUBAndMOVConsumer(t *testing.T) {
	// 自製常數陣列驗規則；沒有包含原版 EXE 或完整掃描素材。
	code := []byte{0xf3, 0xaf, 0x83, 0xef, 4, 0x8b, 7}
	c, _ := inImmediateFixture(code)
	mem := make(testBus, 0x6bbd50)
	copy(mem, code)
	c.Bus = mem
	c.SetDescriptor(0x188, Descriptor{Limit: uint32(len(mem) - 1), Writable: true})
	c.R = [8]uint32{0xffffffff, 0x800, 0, 0x7cc1ef00, 0x2bdacc, 0, 0x6bbc50, 0x6bbc60}
	c.EFlags = 0x246
	for offset := 0x6bbc60; offset < 0x6bbd48; offset += 4 {
		binary.LittleEndian.PutUint32(mem[offset:offset+4], 0xffffffff)
	}
	binary.LittleEndian.PutUint32(mem[0x6bbd48:0x6bbd4c], 0xfffffbff)
	before := append([]byte(nil), mem...)
	want := snapshotInImmediate(c)
	want.r[ECX], want.r[EDI], want.flags = 0x7c5, 0x6bbd4c, 0x206
	if err := c.Step(); err != nil || c.EIP != 2 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("原始初態掃描：%v", err)
	}
	want.r[EDI] = 0x6bbd48
	if err := c.Step(); err != nil || c.EIP != 5 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("SUB 消費 EDI：%v", err)
	}
	want.r[EAX] = 0xfffffbff
	if err := c.Step(); err != nil || c.EIP != 7 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("MOV 消費目的資料：%v", err)
	}
}
