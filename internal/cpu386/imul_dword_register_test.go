package cpu386

import (
	"bytes"
	"math/big"
	"testing"
)

func TestIMULDwordRegisterSourcesSignedProductsAndAliases(t *testing.T) {
	values := []int32{-2147483648, -2147483647, -65536, -32768, -2, -1, 0, 1, 2, 32767, 32768, 65535, 65536, 2147483646, 2147483647}
	min, max := big.NewInt(-2147483648), big.NewInt(2147483647)
	for reg := byte(0); reg < 8; reg++ {
		for _, a := range values {
			for _, b := range values {
				for _, flags := range []uint32{2 | IF, 2 | IF | DF | 0x200000 | CF | OF | SF | ZF | AF | PF} {
					mem := testBus{0xf7, 0xe8 | reg, 0xa5}
					c := New(mem)
					c.R = [8]uint32{0xaaaa0000, 0xbbbb0000, 0xcccc0000, 0xdddd0000, 0xeeee0000, 0xffff0000, 0x12340000, 0x56780000}
					c.R[EAX] = uint32(a)
					if reg != EAX {
						c.R[reg] = uint32(b)
					}
					c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
					c.EFlags = flags
					wantR, segments := c.R, c.Seg
					before := append([]byte(nil), mem...)
					// 以任意精度整數建立獨立乘積及溢位預期。
					product := new(big.Int).Mul(big.NewInt(int64(a)), big.NewInt(int64(int32(c.R[reg]))))
					raw := uint64(product.Int64())
					wantR[EAX], wantR[EDX] = uint32(raw), uint32(raw>>32)
					wantFlags := flags &^ (CF | OF)
					if product.Cmp(min) < 0 || product.Cmp(max) > 0 {
						wantFlags |= CF | OF
					}
					if err := c.Step(); err != nil || c.EIP != 2 || c.R != wantR || c.Seg != segments || c.EFlags != wantFlags || !bytes.Equal(mem, before) {
						t.Fatalf("來源 %d，EAX=%d b=%d：R=%X want=%X flags=%X want=%X err=%v", reg, a, b, c.R, wantR, c.EFlags, wantFlags, err)
					}
				}
			}
		}
	}
}

func TestIMULDwordRegisterRejectedFormsDoNotPublish(t *testing.T) {
	for _, code := range [][]byte{
		{0xf7}, {0xf7, 0x28}, {0xf7, 0x6d, 0}, {0xf7, 0x2d, 0, 0, 0, 0},
		{0x26, 0xf7, 0xeb}, {0x36, 0xf7, 0xeb},
		{0xf2, 0xf7, 0xeb}, {0xf3, 0xf7, 0xeb},
		{0x67, 0xf7, 0xeb}, {0xf0, 0xf7, 0xeb},
		{0xf7, 0xcb},
	} {
		mem := testBus(append([]byte(nil), code...))
		c := New(mem)
		c.R = [8]uint32{0xaaaa0001, 2, 0xbbbbffff, 5, 6, 7, 8, 9}
		c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
		c.EFlags = 2 | IF | CF | OF | AF | PF | ZF | SF
		regs, segments, flags := c.R, c.Seg, c.EFlags
		before := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || c.R != regs || c.Seg != segments || c.EFlags != flags || !bytes.Equal(mem, before) {
			t.Fatalf("拒絕不得發布結果：code=%X R=%X flags=%X err=%v", code, c.R, c.EFlags, err)
		}
	}
}

func TestIMULDwordRegisterOriginalSampleAndXOR(t *testing.T) {
	c := New(testBus{0xf7, 0xeb, 0x33, 0xdb, 0x33, 0xd2})
	c.R = [8]uint32{0xf0, 0, 0, 0x280, 0x3ebb74, 0x3ebba8, 0, 0x3a2090}
	c.Seg = [6]uint16{0x180, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0x246
	regs, segments := c.R, c.Seg
	regs[EAX] = 0x25800
	if err := c.Step(); err != nil || c.EIP != 2 || c.R != regs || c.Seg != segments || c.EFlags != 0x246 {
		t.Fatalf("原版乘積與定義旗標：R=%X flags=%X err=%v", c.R, c.EFlags, err)
	}
	regs[EBX] = 0
	if err := c.Step(); err != nil || c.EIP != 4 || c.R != regs || c.Seg != segments || c.EFlags != 0x246 {
		t.Fatalf("XOR 消費：R=%X flags=%X err=%v", c.R, c.EFlags, err)
	}
	if err := c.Step(); err != nil || c.EIP != 6 || c.R != regs || c.Seg != segments || c.EFlags != 0x246 {
		t.Fatalf("EDX 清除：R=%X flags=%X err=%v", c.R, c.EFlags, err)
	}
}
