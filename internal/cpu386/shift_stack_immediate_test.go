package cpu386

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestSARStackDwordImmediateValuesFlagsAndSS(t *testing.T) {
	const controls = IF | DF
	const initialFlags = controls | AF | OF | CF | ZF
	for _, tc := range []struct {
		value       uint32
		count       byte
		want, flags uint32
	}{
		{0x12345fff, 4, 0x012345ff, controls | OF | CF | PF},
		{0xfffffff1, 4, 0xffffffff, controls | OF | SF | PF},
		{15, 4, 0, controls | OF | CF | ZF | PF},
		{0, 1, 0, controls | ZF | PF},
		{0x80000001, 1, 0xc0000000, controls | CF | SF | PF},
		{0x80000001, 31, 0xffffffff, controls | OF | SF | PF},
		{0x80000001, 0, 0x80000001, initialFlags},
		{0x80000001, 32, 0x80000001, initialFlags},
		{0x80000001, 33, 0xc0000000, controls | CF | SF | PF},
		{0xfffffff1, 255, 0xffffffff, controls | OF | CF | SF | PF},
		{0x7fffffff, 31, 0, controls | OF | CF | ZF | PF},
	} {
		for _, delta := range []int8{-12, 12} {
			mem := testBus(bytes.Repeat([]byte{0xa5}, 256))
			copy(mem, []byte{0xc1, 0x7d, byte(delta), tc.count})
			c := New(mem)
			c.R = [8]uint32{1, 2, 3, 4, 5, 32, 7, 8}
			c.Seg[SegSS], c.Seg[SegDS] = 0x28, 0x30
			c.SetDescriptor(0x28, Descriptor{Base: 128, Limit: 127, Writable: true})
			c.SetDescriptor(0x30, Descriptor{Base: 64, Limit: 127, Writable: true})
			addr := int(32 + delta)
			binary.LittleEndian.PutUint32(mem[128+addr:], tc.value)
			binary.LittleEndian.PutUint32(mem[64+addr:], 0x11223344)
			c.EFlags = initialFlags
			r, seg := c.R, c.Seg
			if err := c.Step(); err != nil || c.R != r || c.Seg != seg || c.EIP != 4 || c.EFlags != tc.flags {
				t.Fatalf("value=%X count=%d delta=%d flags=%X want=%X err=%v", tc.value, tc.count, delta, c.EFlags, tc.flags, err)
			}
			if binary.LittleEndian.Uint32(mem[128+addr:]) != tc.want || binary.LittleEndian.Uint32(mem[64+addr:]) != 0x11223344 || mem[128+addr-1] != 0xa5 || mem[128+addr+4] != 0xa5 {
				t.Fatal("未按 SS 寫入完整 dword 或損壞鄰接資料")
			}
		}
	}
}

func TestSARStackDwordImmediateOriginalSample(t *testing.T) {
	// 規格 260 的 DOSBox-X 同次輸入；只驗 CPU 切片，不冒稱正常玩家對拍。
	mem := testBus(make([]byte, 0x400000))
	copy(mem[0x334e5a:], []byte{0xc1, 0x7d, 0xf4, 4})
	binary.LittleEndian.PutUint32(mem[0x3ebb78:], 32)
	c := New(mem)
	c.R = [8]uint32{EAX: 0x20, EBX: 0x400, EDX: 0x14, ESI: 0x3e0151, EDI: 0x3a2090, EBP: 0x3ebb84, ESP: 0x3ebb3c}
	c.Seg = [6]uint16{SegCS: 0x180, SegDS: 0x188, SegES: 0x188, SegGS: 0x20, SegSS: 0x188}
	c.SetDescriptor(0x188, Descriptor{Limit: 0x3fffff, Writable: true})
	c.EIP, c.EFlags = 0x334e5a, 0x246
	r, seg := c.R, c.Seg
	if err := c.Step(); err != nil || c.R != r || c.Seg != seg || c.EIP != 0x334e5e || binary.LittleEndian.Uint32(mem[0x3ebb78:]) != 2 {
		t.Fatalf("原版 CPU 樣本的值／架構不符：%v", err)
	}
	// 原版 EFLAGS=0212h 的 AF=1；多位 SAR 的 AF／OF 未定義，不作逐位元對拍。
	if c.EFlags & ^uint32(AF|OF) != 0x212 & ^uint32(AF|OF) || c.EFlags != 0x202 {
		t.Fatalf("定義旗標或明示未定義策略不符：%X", c.EFlags)
	}
}

func TestSARStackDwordImmediateRejectsWithoutChangingState(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    []byte
		d       Descriptor
		missing bool
	}{
		{"缺描述子", []byte{0xc1, 0x7d, 0xf4, 4}, Descriptor{}, true},
		{"唯讀", []byte{0xc1, 0x7d, 0xf4, 4}, Descriptor{Base: 128, Limit: 127}, false},
		{"跨段界限", []byte{0xc1, 0x7d, 0xf4, 4}, Descriptor{Base: 128, Limit: 21, Writable: true}, false},
		{"跨 backing", []byte{0xc1, 0x7d, 0xf4, 4}, Descriptor{Base: 234, Limit: 127, Writable: true}, false},
		{"DS 前綴", []byte{0x3e, 0xc1, 0x7d, 0xf4, 4}, Descriptor{Base: 128, Limit: 127, Writable: true}, false},
		{"SS 前綴", []byte{0x36, 0xc1, 0x7d, 0xf4, 4}, Descriptor{Base: 128, Limit: 127, Writable: true}, false},
		{"REP 前綴", []byte{0xf3, 0xc1, 0x7d, 0xf4, 4}, Descriptor{Base: 128, Limit: 127, Writable: true}, false},
		{"word 前綴", []byte{0x66, 0xc1, 0x7d, 0xf4, 4}, Descriptor{Base: 128, Limit: 127, Writable: true}, false},
		{"其他基底", []byte{0xc1, 0x78, 0xf4, 4}, Descriptor{Base: 128, Limit: 127, Writable: true}, false},
		{"其他 group", []byte{0xc1, 0x65, 0xf4, 4}, Descriptor{Base: 128, Limit: 127, Writable: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem := testBus(bytes.Repeat([]byte{0xa5}, 256))
			copy(mem, tc.code)
			c := New(mem)
			c.Seg[SegSS] = 0x28
			c.R[EBP] = 32
			c.EFlags = 0x647
			if !tc.missing {
				c.SetDescriptor(0x28, tc.d)
			}
			r, seg, before := c.R, c.Seg, append([]byte(nil), mem...)
			if c.Step() == nil || c.R != r || c.Seg != seg || c.EFlags != 0x647 || !bytes.Equal(mem, before) {
				t.Fatal("錯誤指令／段範圍未保持拒絕邊界")
			}
		})
	}
	code := []byte{0xc1, 0x7d, 0xf4, 4}
	for n := 0; n < len(code); n++ {
		c := New(testBus(code[:n]))
		c.R[EBP], c.EFlags = 32, 0x647
		if c.Step() == nil || c.R[EBP] != 32 || c.EFlags != 0x647 {
			t.Fatalf("截短 %d 未拒絕", n)
		}
	}
	mem := testBus(bytes.Repeat([]byte{0xa5}, 256))
	copy(mem, code)
	c := New(sarWriteRefusalBus{mem})
	c.R[EBP] = 32
	c.EFlags = 0x647
	c.Seg[SegSS] = 0x28
	c.SetDescriptor(0x28, Descriptor{Base: 128, Limit: 127, Writable: true})
	before := append([]byte(nil), mem...)
	if c.Step() == nil || c.EFlags != 0x647 || !bytes.Equal(mem, before) {
		t.Fatal("寫入拒絕後仍改值或旗標")
	}
}

type sarWriteRefusalBus struct{ testBus }

func (sarWriteRefusalBus) Write8(uint32, uint8) error { return errors.New("受控寫入失敗") }
