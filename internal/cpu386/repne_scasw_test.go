package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 依寬值差、有號範圍與低nibble借位計算，與sub16公式分開。
func scaswExpectedFlags(left, right uint16, initial uint32) uint32 {
	difference := int32(left) - int32(right)
	result := uint16(difference)
	flags := initial &^ (CF | PF | AF | ZF | SF | OF)
	if difference < 0 {
		flags |= CF
	}
	if left%16 < right%16 {
		flags |= AF
	}
	signed := int32(int16(left)) - int32(int16(right))
	if signed < -32768 || signed > 32767 {
		flags |= OF
	}
	if result == 0 {
		flags |= ZF
	}
	if int16(result) < 0 {
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

func TestREPNESCASWIndependentSixFlagsAndPreservation(t *testing.T) {
	values := []uint16{0, 1, 2, 15, 16, 127, 128, 255, 256, 0x7fff, 0x8000, 0x8001, 0xff00, 0xfff0, 0xffff, 0x5555, 0xaaaa, 0x1234, 0x8765}
	arithmetic := []uint32{CF, PF, AF, ZF, SF, OF}
	for _, code := range [][]byte{{0xf2, 0x66, 0xaf}, {0x66, 0xf2, 0xaf}} {
		for _, left := range values {
			for _, right := range values {
				for combo := 0; combo < 64; combo++ {
					for _, direction := range []uint32{0, DF} {
						initial := uint32(2|IF|0x200000) | direction
						for bit, flag := range arithmetic {
							if combo/(1<<bit)%2 != 0 {
								initial |= flag
							}
						}
						c, mem := scasdFixture(code)
						c.R[EAX], c.EFlags = 0xa55a0000|uint32(left), initial
						binary.LittleEndian.PutUint16(mem[288:290], right)
						binary.LittleEndian.PutUint16(mem[800:802], right+1)
						before := append([]byte(nil), mem...)
						want := snapshotInImmediate(c)
						want.r[ECX], want.r[EDI] = 0, 34
						if direction != 0 {
							want.r[EDI] = 30
						}
						want.flags = scaswExpectedFlags(left, right, initial)
						if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
							t.Fatalf("word flags code=%X left=%X right=%X flags=%X: %v", code, left, right, initial, err)
						}
					}
				}
			}
		}
	}
}

func TestREPNESCASWCountMatchDirectionAndAddressWrap(t *testing.T) {
	for _, count := range []uint32{0, 1, 2, 9, 17, 65536} {
		for _, direction := range []uint32{0, DF} {
			for _, initialZF := range []uint32{0, ZF} {
				for _, match := range []int64{-1, 0, int64(count / 2), int64(count) - 1} {
					c, mem := inImmediateFixture([]byte{0xf2, 0x66, 0xaf})
					start := uint32(0xfffffffe)
					if direction != 0 {
						start = 0
					}
					c.R[EAX], c.R[ECX], c.R[EDI], c.EFlags = 0x98761234, count, start, 2|IF|AF|CF|OF|direction|initialZF
					want := snapshotInImmediate(c)
					processed := count
					if match >= 0 && match < int64(count) {
						processed = uint32(match) + 1
					}
					calls := uint32(0)
					c.SegmentRead16 = func(selector uint16, offset uint32) (uint16, bool) {
						element := start + calls*2
						if direction != 0 {
							element = start - calls*2
						}
						if selector != 0x188 || offset != element || calls >= processed {
							t.Fatalf("scan next: %X:%X call=%d", selector, offset, calls)
						}
						value := uint16(0x8123)
						if int64(calls) == match {
							value = 0x1234
						}
						calls++
						return value, true
					}
					want.r[ECX], want.r[EDI] = count-processed, start+processed*2
					if direction != 0 {
						want.r[EDI] = start - processed*2
					}
					if processed != 0 {
						last := uint16(0x8123)
						if match >= 0 && match < int64(count) {
							last = 0x1234
						}
						want.flags = scaswExpectedFlags(0x1234, last, c.EFlags)
					}
					if err := c.Step(); err != nil || c.EIP != 3 || calls != processed || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0xf2, 0x66, 0xaf}) {
						t.Fatalf("count=%d match=%d direction=%X ZF=%X: %v calls=%d", count, match, direction, initialZF, err, calls)
					}
				}
			}
		}
	}
}

func TestREPNESCASWZeroCountInvalidESAndUnalignedEnd(t *testing.T) {
	c, mem := scasdFixture([]byte{0xf2, 0x66, 0xaf})
	c.Seg[SegES], c.R[ECX], c.R[EDI] = 0x200, 0, 0xffffffff
	c.SegmentRead16 = func(uint16, uint32) (uint16, bool) { t.Fatal("zero count read"); return 0, false }
	before := append([]byte(nil), mem...)
	want := snapshotInImmediate(c)
	if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("zero: %v", err)
	}
	for _, offset := range []uint32{31, 254} {
		c, mem = scasdFixture([]byte{0xf2, 0x66, 0xaf})
		c.R[EDI], c.R[EAX], c.EFlags = offset, 0xfeed8000, 2|IF
		binary.LittleEndian.PutUint16(mem[256+offset:258+offset], 0x7fff)
		before = append([]byte(nil), mem...)
		want = snapshotInImmediate(c)
		want.r[ECX], want.r[EDI] = 0, offset+2
		want.flags = scaswExpectedFlags(0x8000, 0x7fff, c.EFlags)
		if err := c.Step(); err != nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("unaligned/end %X: %v", offset, err)
		}
	}
}

func TestREPNESCASWReadFailureFlagsAndProgress(t *testing.T) {
	for completed := uint32(0); completed < 2; completed++ {
		for part := uint32(0); part < 2; part++ {
			c, mem := scasdFixture([]byte{0xf2, 0x66, 0xaf})
			c.R[EAX], c.R[ECX], c.EFlags = 0x1234, 3, 2|IF|CF|AF|OF|SF
			for offset := 288; offset < 294; offset += 2 {
				binary.LittleEndian.PutUint16(mem[offset:offset+2], 0xffff)
			}
			bus := &subByteFailureBus{memory: mem, target: 288 + completed*2 + part, failRead: true}
			c.Bus = bus
			before := append([]byte(nil), mem...)
			want := snapshotInImmediate(c)
			want.r[ECX], want.r[EDI] = 3-completed, 32+2*completed
			if err := c.Step(); err == nil || c.EIP != 3 || snapshotInImmediate(c) != want || bus.writes != 0 || !bytes.Equal(mem, before) {
				t.Fatalf("failure element=%d part=%d: %v", completed, part, err)
			}
		}
	}
	for _, kind := range []string{"unknown", "end", "bus"} {
		c, mem := scasdFixture([]byte{0xf2, 0x66, 0xaf})
		switch kind {
		case "unknown":
			c.Seg[SegES] = 0x200
		case "end":
			c.R[EDI] = 255
		case "bus":
			c.SetDescriptor(0x188, Descriptor{Base: 1023, Limit: 255})
		}
		before := append([]byte(nil), mem...)
		want := snapshotInImmediate(c)
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
			t.Fatalf("%s: %v", kind, err)
		}
	}
}

func TestREPNESCASWRejectsOtherShapesBeforeData(t *testing.T) {
	codes := [][]byte{{}, {0xf2}, {0xf2, 0x66}, {0xaf}, {0x66, 0xaf}, {0xf2, 0xaf}, {0xf3, 0x66, 0xaf}, {0xf2, 0x66, 0xae}, {0xf2, 0xf3, 0x66, 0xaf}, {0xf2, 0xf2, 0x66, 0xaf}, {0xf2, 0x66, 0x66, 0xaf}}
	for _, prefix := range []byte{0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf0} {
		codes = append(codes, []byte{prefix, 0xf2, 0x66, 0xaf}, []byte{0xf2, 0x66, prefix, 0xaf})
	}
	for _, code := range codes {
		c, mem := inImmediateFixture(code)
		want := snapshotInImmediate(c)
		c.SegmentRead16 = func(uint16, uint32) (uint16, bool) { t.Fatal("rejected data read"); return 0, false }
		if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
			t.Fatalf("reject %X: %v", code, err)
		}
	}
}

func TestREPNESCASWSyntheticScanAndMOVConsumer(t *testing.T) {
	// 自製9word陣列，只模擬第6筆匹配及原consumer狀態，不含原版掃描素材。
	code := []byte{0xf2, 0x66, 0xaf, 0x8b, 0x45, 0xfc}
	c, _ := inImmediateFixture(code)
	mem := make(testBus, 1024)
	copy(mem, code)
	c.Bus = mem
	c.SetDescriptor(0x188, Descriptor{Limit: 1023, Writable: true})
	c.R = [8]uint32{2, 9, 0xd6, 128, 512, 520, 0x78, 129}
	c.EFlags = 0x246
	for i := 0; i < 9; i++ {
		value := uint16(100 + i)
		if i == 5 {
			value = 2
		}
		binary.LittleEndian.PutUint16(mem[129+i*2:131+i*2], value)
	}
	binary.LittleEndian.PutUint32(mem[516:520], 0x64)
	before := append([]byte(nil), mem...)
	want := snapshotInImmediate(c)
	want.r[ECX], want.r[EDI], want.flags = 3, 141, 0x246
	if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("scan: %v", err)
	}
	want.r[EAX] = 0x64
	if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("MOV consumer: %v", err)
	}
}
