package oracle

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// `docs/spec/200`：在執行掛鉤裡改暫存器。合成程式，不需要原版素材。

// setRegsOracle 載入 `push cs; pop ds; mov al,41h; mov [0200h],al; jmp $`。
func setRegsOracle(t *testing.T) *Oracle {
	t.Helper()
	code := make([]byte, 0x400)
	copy(code, []byte{0x0E, 0x1F, 0xB0, 0x41, 0xA2, 0x00, 0x02, 0xEB, 0xFE})
	hdr := make([]byte, 32)
	put := func(off int, v uint16) { binary.LittleEndian.PutUint16(hdr[off:], v) }
	total := len(hdr) + len(code)
	copy(hdr, "MZ")
	put(0x02, uint16(total%512))
	put(0x04, uint16((total+511)/512))
	put(0x08, 2)
	put(0x0A, 0x10)
	put(0x0C, 0xFFFF)
	put(0x10, 0x0400)
	put(0x18, 0x001C)
	path := filepath.Join(t.TempDir(), "SETREGS.EXE")
	if err := os.WriteFile(path, append(hdr, code...), 0o644); err != nil {
		t.Fatal(err)
	}
	o, err := Load(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(o.Close)
	return o
}

// §3 第 1 項：掛鉤改 AL，緊接的那道指令寫出去的是改過的值；反向對照：不改時是 41h。
func TestSetRegsInHookAffectsNextInstruction(t *testing.T) {
	for _, change := range []bool{true, false} {
		o := setRegsOracle(t)
		cs := o.IP().Seg
		if change {
			o.OnCall(Addr{cs, 4}, func(o *Oracle) {
				o.SetRegs(CallRegs{AX: o.AX()&0xFF00 | 0x20, SetAX: true})
			})
		}
		if err := o.RunUntil(Cond{name: "跑幾步", ready: func(o *Oracle) bool { return o.Steps() >= 10 }}, Budget(100)); err != nil {
			t.Fatal(err)
		}
		want := uint8(0x41)
		if change {
			want = 0x20
		}
		if got := o.Byte(Addr{cs, 0x200}); got != want {
			t.Errorf("change=%v：[0200h] ＝ %02Xh，要 %02Xh", change, got, want)
		}
	}
}

// §3 第 2 項：只寫 Set* 為 true 的暫存器。
func TestSetRegsOnlyTouchesSelected(t *testing.T) {
	o := setRegsOracle(t)
	before := o.Regs()
	o.SetRegs(CallRegs{AX: 0x1234, BX: 0xBEEF, CX: 0x5678, SetBX: true})
	after := o.Regs()
	want := before
	want.BX = 0xBEEF
	if after != want {
		t.Errorf("SetRegs 之後 %+v，要 %+v", after, want)
	}
}
