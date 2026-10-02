package cpu386

import (
	"bytes"
	"fmt"
	"testing"
)

type inImmediateState struct {
	r               [8]uint32
	seg             [6]uint16
	flags           uint32
	control, status uint16
	depth           uint8
	fpu             [8]float64
}

func snapshotInImmediate(c *CPU) inImmediateState {
	return inImmediateState{c.R, c.Seg, c.EFlags, c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack}
}

func inImmediateFixture(code []byte) (*CPU, testBus) {
	mem := testBus(append([]byte(nil), code...))
	c := New(mem)
	for i := range c.R {
		c.R[i] = 0x98760000 + uint32(i)*0x101
	}
	c.R[EAX], c.R[EDX] = 0xabcdee99, 0xdeadbeef
	c.Seg = [6]uint16{8, 0x188, 0x188, 0, 0x20, 0x188}
	c.EFlags = 0xed7
	c.FPUControl, c.FPUStatus, c.FPUDepth, c.FPUStack = 0x37f, 0x1234, 2, [8]float64{3, 4}
	return c, mem
}

func TestINImmediateALAllPortsAndByteValues(t *testing.T) {
	for port := 0; port < 256; port++ {
		for value := 0; value < 256; value++ {
			c, mem := inImmediateFixture([]byte{0xe4, byte(port)})
			before := snapshotInImmediate(c)
			before.r[EAX] = (before.r[EAX] & 0xffffff00) | uint32(value)
			calls := 0
			c.PortIn = func(got uint16) (uint8, bool) {
				calls++
				if got != uint16(port) {
					t.Fatalf("立即埠編號未補零：%X want %X", got, port)
				}
				return byte(value), true
			}
			if err := c.Step(); err != nil || c.EIP != 2 || calls != 1 || snapshotInImmediate(c) != before || !bytes.Equal(mem, []byte{0xe4, byte(port)}) {
				t.Fatalf("port=%X value=%X：%v", port, value, err)
			}
		}
	}
}

func TestINImmediateALFailureDoesNotCommit(t *testing.T) {
	for _, name := range []string{"缺立即值", "缺平台", "平台拒絕"} {
		t.Run(name, func(t *testing.T) {
			code := []byte{0xe4, 0x40}
			if name == "缺立即值" {
				code = code[:1]
			}
			c, mem := inImmediateFixture(code)
			before := snapshotInImmediate(c)
			calls := 0
			if name != "缺平台" {
				c.PortIn = func(port uint16) (byte, bool) {
					calls++
					if port != 0x40 {
						t.Fatal("埠編號")
					}
					return 0x99, false
				}
			}
			wantCalls := 0
			if name == "平台拒絕" {
				wantCalls = 1
			}
			if err := c.Step(); err == nil || calls != wantCalls || snapshotInImmediate(c) != before || !bytes.Equal(mem, code) {
				t.Fatalf("拒絕改寫資料：%v", err)
			}
		})
	}
}

func TestINImmediateALUnsupportedFormsDoNotCallPlatform(t *testing.T) {
	for _, prefix := range []byte{0x66, 0x67, 0x26, 0x2e, 0x36, 0x3e, 0x64, 0x65, 0xf2, 0xf3, 0xf0} {
		t.Run(fmt.Sprintf("prefix%X", prefix), func(t *testing.T) {
			c, _ := inImmediateFixture([]byte{prefix, 0xe4, 0x40})
			before := snapshotInImmediate(c)
			calls := 0
			c.PortIn = func(uint16) (byte, bool) { calls++; return 1, true }
			if err := c.Step(); err == nil || calls != 0 || snapshotInImmediate(c) != before {
				t.Fatalf("未知前綴未拒絕：%v", err)
			}
		})
	}
	for _, op := range []byte{0xe4, 0xe5, 0xed} {
		c, _ := inImmediateFixture([]byte{op, 0x40})
		if op == 0xe4 {
			c.EFlags |= 1 << 17
		}
		before := snapshotInImmediate(c)
		calls := 0
		c.PortIn = func(uint16) (byte, bool) { calls++; return 1, true }
		if err := c.Step(); err == nil || calls != 0 || snapshotInImmediate(c) != before {
			t.Fatalf("VM／未知寬度 %X 未拒絕：%v", op, err)
		}
	}
}
