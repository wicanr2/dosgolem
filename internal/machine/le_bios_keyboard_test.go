package machine

import (
	"encoding/binary"
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"testing"
)

func keyFixture(t *testing.T) *LEMachine {
	t.Helper()
	m := &LEMachine{Mem: make([]byte, 0x500)}
	binary.LittleEndian.PutUint16(m.Mem[0x41a:], 0x1e)
	binary.LittleEndian.PutUint16(m.Mem[0x41c:], 0x1e)
	if !InstallLEBIOSKeyboard(m) {
		t.Fatal("鍵盤安裝")
	}
	return m
}
func TestBIOSKeyboardRing(t *testing.T) {
	m := keyFixture(t)
	k := m.Keyboard
	if _, ok, e := k.ReadEnhanced(); e != nil || ok || !k.Waiting {
		t.Fatal("空佇列")
	}
	for i := 0; i < 15; i++ {
		if err := k.Enqueue(uint16(i + 1)); err != nil {
			t.Fatal(err)
		}
	}
	if k.Enqueue(99) == nil {
		t.Fatal("滿佇列")
	}
	for i := 0; i < 15; i++ {
		v, ok, e := k.ReadEnhanced()
		if e != nil || !ok || v != uint16(i+1) {
			t.Fatal("佇列順序")
		}
	}
	for _, v := range []uint16{0x50e0, 0x1c0d, 0x48f0} {
		if e := k.Enqueue(v); e != nil {
			t.Fatal(e)
		}
		w, ok, e := k.ReadEnhanced()
		want := v
		if v == 0x48f0 {
			want = 0x4800
		}
		if e != nil || !ok || w != want {
			t.Fatal("回繞／增强碼")
		}
	}
	binary.LittleEndian.PutUint16(m.Mem[0x41a:], 0x1f)
	if k.Enqueue(1) == nil {
		t.Fatal("無效指標")
	}
}
func TestWatcomKeyboardBlockingAndReturn(t *testing.T) {
	m := keyFixture(t)
	m.CPU = cpu386.New(m)
	c := m.CPU
	c.Seg[cpu386.SegSS] = 0x160
	c.SetDescriptor(0x160, cpu386.Descriptor{Limit: 0x4ff, Writable: true})
	c.EIP = 0x2000
	c.R[cpu386.ESP] = 0x20
	c.R[cpu386.EAX] = 0x12345678
	c.EFlags = cpu386.IF | cpu386.CF
	for i, v := range []uint32{0x3000, 0x16, 0x60, 0x60} {
		binary.LittleEndian.PutUint32(m.Mem[0x20+i*4:], v)
	}
	for i, v := range []uint32{0x1013, 1, 2, 3, 4, 5, 1} {
		binary.LittleEndian.PutUint32(m.Mem[0x60+i*4:], v)
	}
	s := &WatcomInt386DPMI{machine: m, entry: 0x2000}
	c.StepHook = s.Handle
	if e := c.Step(); e != nil || c.EIP != 0x2000 || c.R[cpu386.ESP] != 0x20 || c.R[cpu386.EAX] != 0x12345678 || !m.Keyboard.Waiting {
		t.Fatal("阻塞發布")
	}
	m.Keyboard.Enqueue(0x1c0d)
	if e := c.Step(); e != nil || c.EIP != 0x3000 || c.R[cpu386.ESP] != 0x24 || c.R[cpu386.EAX] != 0x1c0d || binary.LittleEndian.Uint32(m.Mem[0x64:]) != 1 || c.EFlags != cpu386.IF|cpu386.CF {
		t.Fatal("按鍵返回")
	}
}
