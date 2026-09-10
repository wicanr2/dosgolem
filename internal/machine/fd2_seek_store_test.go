package machine

import (
	"github.com/wicanr2/dosgolem/internal/cpu386"
	"os"
	"testing"
)

func TestFD2StoresSeekResultThroughSS(t *testing.T) {
	if os.Getenv("DOSGOLEM_FD2_ROOT") == "" {
		t.Skip("DOSGOLEM_FD2_ROOT 未設定")
	}
	m, _ := fixedFD2Machine(t)
	for steps := 0; m.CPU.EIP != 0x3cc20; steps++ {
		if steps >= 25000 {
			t.Fatal("未抵達定位回傳")
		}
		if err := m.CPU.Step(); err != nil {
			t.Fatal(err)
		}
	}
	ptr := m.CPU.R[cpu386.EDI]
	regs, segs, flags := m.CPU.R, m.CPU.Seg, m.CPU.EFlags
	if uint16(regs[cpu386.EAX]) != 218 || uint16(regs[cpu386.EDX]) != 0 || flags&cpu386.CF != 0 {
		t.Fatal("MDI.INI 定位回傳不符 DOSBox")
	}
	beforeHi, err := m.Read16(ptr + 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.CPU.Step(); err != nil {
		t.Fatal(err)
	}
	low, _ := m.Read16(ptr)
	hi, _ := m.Read16(ptr + 2)
	if m.CPU.EIP != 0x3cc24 || low != 218 || hi != beforeHi || m.CPU.R != regs || m.CPU.Seg != segs || m.CPU.EFlags != flags {
		t.Fatal("AX store 不符")
	}
	if err := m.CPU.Step(); err != nil {
		t.Fatal(err)
	}
	value, err := m.Read32(ptr)
	if err != nil || m.CPU.EIP != 0x3cc29 || value != 218 || m.CPU.R != regs || m.CPU.Seg != segs || m.CPU.EFlags != flags {
		t.Fatal("DX store 不符")
	}
	t.Logf("DOSBox 對應結果：SS=%04X pointer=%08X value=%08X EIP=%08X", m.CPU.Seg[cpu386.SegSS], ptr, value, m.CPU.EIP)
}
