package cpu

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

// docs/spec/435-v86-hooks-and-idt-delivery §2.5 第 1–3 步（核心側）：分段模式、NT=0、CPL 0 的 66 CF 讀出 9 個 dword，
// 檢查後存進 V86Return、IP 回起點、回 ErrV86Return；只讀不寫。

const retSS = 0x20000

func newRetCPU(t *testing.T, cpl uint8, sp uint16, frame []uint32) (*CPU, *testBus) {
	t.Helper()
	c, b := newSegCPU(t, cpl)
	copy(b.mem[0x10000:], []byte{0x66, 0xCF})
	c.Seg[SS], c.R[SP] = 0x50, sp
	c.SetSegCache(SS, SegCache{Base: retSS, Limit: 0xFFFF, Attr: 0x93})
	for k, v := range frame {
		var w [4]byte
		binary.LittleEndian.PutUint32(w[:], v)
		for i := 0; i < 4; i++ {
			b.mem[retSS+uint32(uint16(sp+uint16(4*k+i)))] = w[i]
		}
	}
	return c, b
}

// 框架順序：EIP、CS、EFLAGS、ESP、SS、ES、DS、FS、GS。
var retFrame = []uint32{0x02C0, 0x0110, 0x00023202, 0x000000CC, 0x0216, 0x2000, 0x01EA, 0x0028, 0x0030}

func TestV86ReturnRead(t *testing.T) {
	for _, sp := range []uint16{0x13D8, 0xFFF0} { // 第二例的讀取跨過 SP=0 環繞
		c, b := newRetCPU(t, 0, sp, retFrame)
		before := append([]byte(nil), b.mem[retSS:retSS+0x10000]...)
		err := c.Step()
		if !errors.Is(err, ErrV86Return) {
			t.Fatalf("SP=%04X：應回 ErrV86Return，實際 %v", sp, err)
		}
		r := c.V86Return()
		want := V86Return{EIP: 0x02C0, CS: 0x0110, EFLAGS: 0x00023202, ESP: 0xCC, SS: 0x0216, ES: 0x2000, DS: 0x01EA, FS: 0x28, GS: 0x30}
		if r != want {
			t.Errorf("SP=%04X：彈出值 %+v，應為 %+v", sp, r, want)
		}
		if c.IP != 0 || c.R[SP] != sp || !c.Segmented() || string(b.mem[retSS:retSS+0x10000]) != string(before) {
			t.Errorf("SP=%04X：只讀不寫、IP 回起點（IP=%04X SP=%04X segmented=%v）", sp, c.IP, c.R[SP], c.Segmented())
		}
	}
}

func TestV86ReturnReadFailures(t *testing.T) {
	mod := func(i int, v uint32) []uint32 {
		f := append([]uint32(nil), retFrame...)
		f[i] = v
		return f
	}
	cases := []struct {
		name     string
		cpl      uint8
		frame    []uint32
		contains string
	}{
		{"彈出 VM=0", 0, mod(2, 0x00003202), "VM=0"},
		{"EIP 高半非 0", 0, mod(0, 0x000102C0), "EIP"},
		{"EFLAGS bit 18–31", 0, mod(2, 0x00063202), "bit 18"},
		{"CPL≠0", 3, retFrame, "IRET"},
	}
	for _, x := range cases {
		c, _ := newRetCPU(t, x.cpl, 0x13D8, x.frame)
		err := c.Step()
		if err == nil || errors.Is(err, ErrV86Return) || !strings.Contains(err.Error(), x.contains) ||
			!strings.Contains(err.Error(), "IRET") {
			t.Errorf("%s：應失敗（含 %q 與 IRET），實際 %v", x.name, x.contains, err)
		}
	}
	// 16 位元 CF 維持 434 的失敗即關閉。
	c, b := newRetCPU(t, 0, 0x13D8, retFrame)
	b.mem[0x10000] = 0xCF
	if err := c.Step(); err == nil || errors.Is(err, ErrV86Return) || !strings.Contains(err.Error(), "IRET") {
		t.Errorf("16 位元 CF 應失敗，實際 %v", err)
	}
	// SS 界限不足：434 §4.1 的檢查失敗。
	c, _ = newRetCPU(t, 0, 0x13D8, retFrame)
	c.SetSegCache(SS, SegCache{Base: retSS, Limit: 0x13E0, Attr: 0x93})
	if err := c.Step(); err == nil || errors.Is(err, ErrV86Return) || !strings.Contains(err.Error(), "界限") {
		t.Errorf("SS 界限不足應失敗，實際 %v", err)
	}
}
