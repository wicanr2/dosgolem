package cpu386

import (
	"bytes"
	"strings"
	"testing"
)

func TestDIVByteRegisterAllSourcesAndAliases(t *testing.T) {
	for src := 0; src < 8; src++ {
		for _, dividend := range []uint16{0, 1, 99, 100, 255, 256, 257, 0x1234, 0x6300, 0x639b, 0x6400, 0xff00, 0xffff} {
			for _, divisor := range []byte{0, 1, 2, 3, 99, 100, 128, 255} {
				mem := testBus{0xf6, 0xf0 | byte(src), 0xa5}
				c := New(mem)
				c.R = [8]uint32{0xa5a50000 | uint32(dividend), 0x12345678, 0x87654321, 0xabcdef01, 8, 9, 10, 11}
				c.setReg8(src, divisor)
				c.Seg = [6]uint16{1, 2, 3, 4, 5, 6}
				c.EFlags = IF | DF | 2 | CF | PF | AF | SF | ZF | OF
				beforeR, beforeSeg, beforeFlags := c.R, c.Seg, c.EFlags
				beforeMem := append([]byte(nil), mem...)
				// AL／AH 別名可能改到初始 AX，依實際架構輸入核對。
				inputAX := uint32(uint16(beforeR[EAX]))
				inputDivisor := uint32(c.reg8(src))
				err := c.Step()
				wantR := beforeR
				if inputDivisor == 0 || inputAX/inputDivisor > 255 {
					if err == nil {
						t.Fatalf("來源 %d AX=%x 除數=%x 未拒絕", src, inputAX, inputDivisor)
					}
				} else {
					q, r := uint32(byte(c.R[EAX])), uint32(byte(c.R[EAX]>>8))
					if err != nil || q*inputDivisor+r != inputAX || r >= inputDivisor {
						t.Fatalf("來源 %d AX=%x 除數=%x 結果=%x err=%v", src, inputAX, inputDivisor, c.R[EAX], err)
					}
					wantR[EAX] = beforeR[EAX]&0xffff0000 | uint32(uint16(c.R[EAX]))
				}
				if c.R != wantR || c.Seg != beforeSeg || c.EFlags != beforeFlags || c.EIP != 2 || !bytes.Equal(mem, beforeMem) {
					t.Fatal("DIV byte 改到商餘之外的資料或旗標策略")
				}
			}
		}
	}
}

func TestDIVByteRegisterBoundaryAndError(t *testing.T) {
	for _, tc := range []struct {
		ax        uint16
		div       byte
		want      uint16
		errorText string
	}{{0, 100, 0, ""}, {257, 2, 0x180, ""}, {255, 1, 255, ""}, {25599, 100, 0x63ff, ""}, {25600, 100, 0, "商溢位"}, {65535, 255, 0, "商溢位"}, {1, 0, 0, "除以零"}} {
		c := New(testBus{0xf6, 0xf3})
		c.R[EAX], c.R[EBX], c.EFlags = 0xface0000|uint32(tc.ax), uint32(tc.div), IF|2|AF|ZF
		err := c.Step()
		if tc.errorText != "" {
			if err == nil || !strings.Contains(err.Error(), tc.errorText) || c.R[EAX] != 0xface0000|uint32(tc.ax) {
				t.Fatal("除法錯誤未保持資料或原因錯誤")
			}
		} else if err != nil || c.R[EAX] != 0xface0000|uint32(tc.want) {
			t.Fatalf("AX=%d / %d 商餘=%x err=%v", tc.ax, tc.div, c.R[EAX], err)
		}
	}
}

func TestDIVByteRegisterRejectsUnknownShape(t *testing.T) {
	for _, code := range [][]byte{{0x66, 0xf6, 0xf3}, {0x26, 0xf6, 0xf3}, {0xf2, 0xf6, 0xf3}, {0xf3, 0xf6, 0xf3}, {0xf6, 0x30}, {0xf6, 0xfb}, {0xf6}} {
		mem := testBus(code)
		c := New(mem)
		c.R, c.EFlags = [8]uint32{257, 1, 2, 100, 4, 5, 6, 7}, IF|DF|2|AF|CF
		beforeR, beforeFlags := c.R, c.EFlags
		beforeMem := append([]byte(nil), mem...)
		if err := c.Step(); err == nil || c.R != beforeR || c.EFlags != beforeFlags || !bytes.Equal(mem, beforeMem) {
			t.Fatalf("未知形狀 %x 未拒絕或改到資料", code)
		}
	}
}

func TestDIVByteRegisterKeepsExistingMULAndTEST(t *testing.T) {
	mul := New(testBus{0xf6, 0xe3})
	mul.R[EAX], mul.R[EBX] = 0xabcd0011, 20
	if err := mul.Step(); err != nil || mul.R[EAX] != 0xabcd0154 || mul.EFlags&(CF|OF) != CF|OF {
		t.Fatal("既有 byte MUL 回歸")
	}
	test := New(testBus{0xf6, 0xc3, 0x80})
	test.R[EBX] = 0x12340080
	if err := test.Step(); err != nil || test.R[EBX] != 0x12340080 || test.EFlags&SF == 0 || test.EFlags&ZF != 0 {
		t.Fatal("既有 byte TEST 回歸")
	}
}

func TestDIVByteRegisterOriginalSampleAndOUT(t *testing.T) {
	// 原版同次 0180:00356CCB → 00356CCD；完整定位見規格 264。
	mem := testBus{0xf6, 0xf3, 0xee}
	c := New(mem)
	c.R[EAX], c.R[EBX], c.R[ECX], c.R[EDX] = 0, 100, 128, 0x3c9
	c.R[ESI], c.R[EDI], c.R[EBP], c.R[ESP] = 0x3d135a, 0x3a2090, 0x3ebc06, 0x3ebba0
	c.Seg[SegCS], c.Seg[SegDS], c.Seg[SegES], c.Seg[SegSS], c.Seg[SegFS], c.Seg[SegGS] = 0x180, 0x188, 0x188, 0x188, 0, 0x20
	c.EFlags = 0x46
	beforeR, beforeSeg := c.R, c.Seg
	if err := c.Step(); err != nil || c.EIP != 2 || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != 0x46 {
		t.Fatal("原版商餘、資料保持或保留旗標策略不符")
	}
	// 原版返回 06h；只比較 DIV 有定義的控制位，ZF 差異明示。
	const arithmetic = CF | PF | AF | ZF | SF | OF
	if c.EFlags&^arithmetic != 0x06&^arithmetic || c.EFlags == 0x06 {
		t.Fatal("未定義旗標差異遭隱藏或控制位不符")
	}
	calls := 0
	c.PortOut = func(port uint16, value byte) bool {
		calls++
		return port == 0x3c9 && value == 0
	}
	if err := c.Step(); err != nil || calls != 1 || c.EIP != 3 || c.R != beforeR || c.Seg != beforeSeg || c.EFlags != 0x46 {
		t.Fatal("原版第一個 OUT 消費不符")
	}
}
