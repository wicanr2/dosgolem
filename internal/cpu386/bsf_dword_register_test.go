package cpu386

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func bsfDwordIndex(value uint32) (uint32, bool) {
	if value == 0 {
		return 0, false
	}
	index := uint32(0)
	for divisor := uint64(2); uint64(value)%divisor == 0; divisor *= 2 {
		index++
	}
	return index, true
}

func TestBSFDwordAllPairsBitsAliasesFlagsAndUndefinedModels(t *testing.T) {
	values := []uint32{0, 0xffffffff, 0x12345678, 0x87654321, 0x80000000, 0x400}
	for bit := uint(0); bit < 32; bit++ {
		values = append(values, uint32(1)<<bit, ^((uint32(1) << bit) - 1), uint32(1)<<bit|0xa0000000)
	}
	arithmetic := []uint32{CF, PF, AF, ZF, SF, OF}
	for destination := byte(0); destination < 8; destination++ {
		for source := byte(0); source < 8; source++ {
			code := []byte{0x0f, 0xbc, 0xc0 | destination<<3 | source}
			for _, value := range values {
				for combination := 0; combination < 64; combination++ {
					c, mem := inImmediateFixture(code)
					c.R[destination], c.R[source] = 0x76543210, value
					c.EFlags = 2 | IF | DF | 0x200000
					for bit, flag := range arithmetic {
						if combination/(1<<bit)%2 != 0 {
							c.EFlags |= flag
						}
					}
					want := snapshotInImmediate(c)
					index, nonzero := bsfDwordIndex(value)
					if nonzero {
						want.r[destination], want.flags = index, c.EFlags&^ZF
					} else {
						// 零來源目的保留是工具模型，不當作硬體定義結果。
						want.flags |= ZF
					}
					initial := c.EFlags
					if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
						t.Fatalf("目的%d／來源%d／值%X／初態%X：%v", destination, source, value, initial, err)
					}
					wantZF := uint32(ZF)
					if nonzero {
						wantZF = 0
					}
					if c.EFlags&ZF != wantZF {
						t.Fatal("只將 ZF 作定義旗標驗收")
					}
					if c.EFlags&^ZF != initial&^ZF {
						t.Fatal("其他旗標保持模型另驗")
					}
				}
			}
		}
	}
}

func TestBSFDwordAllLowWordValuesAndRegisterPairs(t *testing.T) {
	for destination := byte(0); destination < 8; destination++ {
		for source := byte(0); source < 8; source++ {
			code := []byte{0x0f, 0xbc, 0xc0 | destination<<3 | source}
			c, mem := inImmediateFixture(code)
			initial := snapshotInImmediate(c)
			for value := uint32(0); value < 65536; value++ {
				for _, flags := range []uint32{2 | IF | DF | 0x200000, 2 | IF | DF | 0x200000 | CF | PF | AF | ZF | SF | OF} {
					c.EIP, c.R, c.EFlags = 0, initial.r, flags
					c.R[destination], c.R[source] = 0xabcdef01, value
					want := snapshotInImmediate(c)
					index, nonzero := bsfDwordIndex(value)
					if nonzero {
						want.r[destination], want.flags = index, flags&^ZF
					} else {
						want.flags |= ZF
					}
					if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want || !bytes.Equal(mem, code) {
						t.Fatalf("低16位／目的%d／來源%d／值%X：%v", destination, source, value, err)
					}
				}
			}
		}
	}
}

func TestBSFDwordZeroSourceUndefinedDestinationModels(t *testing.T) {
	for _, old := range []uint32{0, 1, 31, 0x80000000, 0xffffffff, 0x12345678} {
		c, mem := inImmediateFixture([]byte{0x0f, 0xbc, 0xd0})
		c.R[EAX], c.R[EDX], c.EFlags = 0, old, 2|IF|CF|AF|OF|SF|PF
		want := snapshotInImmediate(c)
		want.flags |= ZF
		if err := c.Step(); err != nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, []byte{0x0f, 0xbc, 0xd0}) {
			t.Fatalf("零來源目的保持模型 old=%X：%v", old, err)
		}
	}
}

func TestBSFDwordRefusesTruncationsPrefixesAndAllMemoryForms(t *testing.T) {
	for destination := byte(0); destination < 8; destination++ {
		for source := byte(0); source < 8; source++ {
			code := []byte{0x0f, 0xbc, 0xc0 | destination<<3 | source}
			cases := [][]byte{code[:0], code[:1], code[:2]}
			for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
				cases = append(cases, append([]byte{prefix}, code...))
			}
			for mod := byte(0); mod < 3; mod++ {
				cases = append(cases, []byte{0x0f, 0xbc, mod<<6 | destination<<3 | source, 0, 0, 0, 0, 0})
			}
			for _, instruction := range cases {
				c, mem := inImmediateFixture(instruction)
				c.SegmentRead8 = func(uint16, uint32) (byte, bool) { t.Fatal("拒絕形狀不可讀資料"); return 0, false }
				want := snapshotInImmediate(c)
				if err := c.Step(); err == nil || snapshotInImmediate(c) != want || !bytes.Equal(mem, instruction) {
					t.Fatalf("拒絕%X：%v", instruction, err)
				}
			}
		}
	}
}

func TestBSFDwordOriginalXORShiftScanADDAndWordStoreChain(t *testing.T) {
	code := []byte{0x83, 0xf0, 0xff, 0xc1, 0xe7, 3, 0x0f, 0xbc, 0xd0, 3, 0xfa, 0x66, 0x89, 0x3d, 0xd0, 0x26, 0x27, 0}
	c, _ := inImmediateFixture(code)
	mem := make(testBus, 0x2726d4)
	copy(mem, code)
	c.Bus = mem
	c.SetDescriptor(0x188, Descriptor{Limit: uint32(len(mem) - 1), Writable: true})
	c.R = [8]uint32{0xfffffbff, 0x7c5, 0, 0x7cc1ef00, 0x2bdacc, 0, 0x6bbc50, 0xe8}
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x206
	want := snapshotInImmediate(c)
	before := append([]byte(nil), mem...)
	want.r[EAX] = 0x400
	if err := c.Step(); err != nil || c.EIP != 3 || snapshotInImmediate(c) != want {
		t.Fatalf("XOR 資料：%v", err)
	}
	want.r[EDI], want.flags = 0x740, 0x202
	if err := c.Step(); err != nil || c.EIP != 6 || snapshotInImmediate(c) != want {
		t.Fatalf("既有移位後保持來源：%v", err)
	}
	want.r[EDX] = 10
	if err := c.Step(); err != nil || c.EIP != 9 || snapshotInImmediate(c) != want {
		t.Fatalf("BSF 真實來源／目的：%v", err)
	}
	want.r[EDI] = 0x74a
	if err := c.Step(); err != nil || c.EIP != 11 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("ADD 消費 EDX：%v", err)
	}
	binary.LittleEndian.PutUint16(before[0x2726d0:0x2726d2], 0x74a)
	if err := c.Step(); err != nil || c.EIP != 18 || snapshotInImmediate(c) != want || !bytes.Equal(mem, before) {
		t.Fatalf("word MOV 存回 DI：%v", err)
	}
}
