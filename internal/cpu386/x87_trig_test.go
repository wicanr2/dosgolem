package cpu386

import (
	"bytes"
	"math"
	"testing"
)

func TestX87TrigKnownAnglesAndPreservation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		op          byte
		angle, want float64
		exact       bool
	}{
		{"cos零", 0xff, 0, 1, true},
		{"cos負零", 0xff, math.Copysign(0, -1), 1, true},
		{"cos三分之一π", 0xff, math.Pi / 3, 0.5, false},
		{"cos負三分之一π", 0xff, -math.Pi / 3, 0.5, false},
		{"sin零", 0xfe, 0, 0, true},
		{"sin負零", 0xfe, math.Copysign(0, -1), math.Copysign(0, -1), true},
		{"sin二分之一π", 0xfe, math.Pi / 2, 1, false},
		{"sin負二分之一π", 0xfe, -math.Pi / 2, -1, false},
		{"sin六分之一π", 0xfe, math.Pi / 6, 0.5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mem, c := newFPU(t, []byte{0xd9, tc.op})
			c.FPUStack = [8]float64{tc.angle, 7, 9}
			c.FPUDepth = 3
			c.FPUControl = 0x127f
			c.FPUStatus = 0x4700
			c.EFlags = IF | DF | CF | 2
			before := append([]byte(nil), mem...)
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			got := c.FPUStack[0]
			if tc.exact && math.Float64bits(got) != math.Float64bits(tc.want) || !tc.exact && math.Abs(got-tc.want) > 1e-15 {
				t.Fatalf("got %.17g want %.17g", got, tc.want)
			}
			if c.EIP != 2 || c.FPUDepth != 3 || c.FPUStack[1] != 7 || c.FPUStack[2] != 9 || c.FPUControl != 0x127f || c.FPUStatus != 0x4300 || c.EFlags != IF|DF|CF|2 || !bytes.Equal(mem, before) {
				t.Fatal("三角函數改寫了其他狀態")
			}
		})
	}
}

func TestX87TrigRangeC2AndStatusConsumer(t *testing.T) {
	bound := math.Ldexp(1, 63)
	for _, op := range []byte{0xfe, 0xff} {
		for _, angle := range []float64{bound, -bound, math.Nextafter(bound, math.Inf(1)), math.Nextafter(-bound, math.Inf(-1)), math.Nextafter(bound, 0), math.Nextafter(-bound, 0)} {
			mem, c := newFPU(t, []byte{0xd9, op, 0xdf, 0xe0})
			source := append([]byte(nil), mem...)
			c.FPUStack[0] = angle
			c.FPUDepth = 1
			c.FPUStatus = 0x4500
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			outside := math.Abs(angle) >= bound
			if (c.FPUStatus&0x0400 != 0) != outside {
				t.Fatalf("op=%x angle=%g status=%x", op, angle, c.FPUStatus)
			}
			if outside && c.FPUStack[0] != angle {
				t.Fatal("超範圍仍發布數值")
			}
			if !outside && math.Abs(c.FPUStack[0]) > 1 {
				t.Fatal("合法範圍結果超過1")
			}
			if c.FPUStatus&^uint16(0x0400) != 0x4100 || c.FPUDepth != 1 {
				t.Fatal("其他狀態改變")
			}
			c.R[EAX] = 0xabcd0000
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
			if c.R[EAX] != 0xabcd0000|uint32(c.FPUStatus) || c.EIP != 4 || !bytes.Equal(mem, source) {
				t.Fatal("FNSTSW未消費C2")
			}
		}
	}
}

func TestX87TrigRejectsWithoutPublishing(t *testing.T) {
	for _, op := range []byte{0xfe, 0xff} {
		for _, tc := range []struct {
			name   string
			prefix []byte
			value  float64
			empty  bool
		}{
			{"空堆疊", nil, 0, true},
			{"正無窮", nil, math.Inf(1), false},
			{"負無窮", nil, math.Inf(-1), false},
			{"NaN", nil, math.NaN(), false},
			{"operand16", []byte{0x66}, 0, false},
			{"段覆寫", []byte{0x26}, 0, false},
			{"REP", []byte{0xf3}, 0, false},
			{"REPNE", []byte{0xf2}, 0, false},
		} {
			t.Run(tc.name+string(rune(op)), func(t *testing.T) {
				program := append(append([]byte(nil), tc.prefix...), 0xd9, op)
				mem, c := newFPU(t, program)
				c.FPUStack = [8]float64{tc.value, 7}
				c.FPUDepth = 2
				c.FPUStatus = 0x4700
				if tc.empty {
					c.FPUDepth = 0
				}
				before := *c
				source := append([]byte(nil), mem...)
				if err := c.Step(); err == nil {
					t.Fatal("未拒絕")
				}
				for i, v := range c.FPUStack {
					if math.Float64bits(v) != math.Float64bits(before.FPUStack[i]) {
						t.Fatal("發布了FPU值")
					}
				}
				if c.FPUStatus != before.FPUStatus || c.FPUDepth != before.FPUDepth || c.FPUControl != before.FPUControl || c.EFlags != before.EFlags || !bytes.Equal(mem, source) {
					t.Fatal("拒絕後狀態改變")
				}
			})
		}
	}
	for _, program := range [][]byte{{0xd9}, {0xd9, 0xfb}} {
		c := New(testBus(append([]byte(nil), program...)))
		c.FPUStack[0] = 1
		c.FPUDepth = 1
		if err := c.Step(); err == nil || c.FPUStack[0] != 1 {
			t.Fatalf("未知或截斷 %x err=%v", program, err)
		}
	}
}
