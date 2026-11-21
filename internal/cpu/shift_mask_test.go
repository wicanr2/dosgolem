package cpu

import "testing"

// 移位量遮罩（`docs/spec/431-cpu-shift-count-mask`）：8086 用整個 CL，
// 80186 以上取 `CL & 31`。
//
// 用的是 `BGMSEQ.COM` 的 CPU 偵測那一段：`mov ax,2; mov cl,21h; shr ax,cl`。
// 8086 把 2 移 33 位得 0，186 以上只移 1 位得 1。
func TestShiftCountMaskByModel(t *testing.T) {
	run := func(t *testing.T, model Model, code []byte, steps int) uint16 {
		t.Helper()
		b := newTestBus()
		c := New(b)
		c.Model = model
		c.Seg[CS], c.Seg[DS], c.Seg[SS] = 0x2000, 0x2000, 0x2000
		c.IP, c.R[SP] = 0, 0x1000
		for i, v := range code {
			c.Bus.Write8(Addr(c.Seg[CS], c.IP)+uint32(i), v)
		}
		for i := 0; i < steps; i++ {
			if err := c.Step(); err != nil {
				t.Fatal(err)
			}
		}
		return c.R[AX]
	}
	detect := []byte{0xB8, 0x02, 0x00, 0xB1, 0x21, 0xD3, 0xE8} // mov ax,2 / mov cl,21h / shr ax,cl
	for _, tc := range []struct {
		name  string
		model Model
		want  uint16
	}{
		{"8086 不遮", Model8086, 0},
		{"186 遮成 5 位元", Model80186, 1},
		{"386 遮成 5 位元", Model80386, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(t, tc.model, detect, 3); got != tc.want {
				t.Errorf("AX=%04X，預期 %04X", got, tc.want)
			}
		})
	}
	t.Run("186 的 C1 imm8 也遮", func(t *testing.T) {
		code := []byte{0xB8, 0x02, 0x00, 0xC1, 0xE8, 0x21} // mov ax,2 / shr ax,21h
		if got := run(t, Model80186, code, 2); got != 1 {
			t.Errorf("AX=%04X，預期 0001", got)
		}
	})
}
