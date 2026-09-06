package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 一支很短的 .COM：把 1234h 寫進 DS:0200，再原地無窮迴圈。
//
//	B8 34 12        mov ax, 1234h
//	A3 00 02        mov [0200h], ax
//	EB FE           jmp $
var tinyCOM = []byte{0xB8, 0x34, 0x12, 0xA3, 0x00, 0x02, 0xEB, 0xFE}

func boot(t *testing.T) (*machine.Machine, *dos.DOS) {
	t.Helper()
	m := machine.New()
	if err := m.LoadCOM(tinyCOM); err != nil {
		t.Fatal(err)
	}
	d := dos.New(m, ".")
	d.Install()
	return m, d
}

// TestSaveLoadRoundTrip 釘住「存了再讀回來是同一台機器」。
//
// **這支測試擋的是「漏抄欄位」**：漏掉的欄位不會報錯，還原之後機器
// 看起來完全正常，只是某個時鐘或某個 plane 停在別的時間點——
// 症狀出現在很後面，而且完全不指向這裡。
func TestSaveLoadRoundTrip(t *testing.T) {
	m, d := boot(t)
	for i := 0; i < 5000 && !m.CPU.Halted; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "s.gz")
	if err := Save(path, m, d); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		t.Fatalf("存檔是空的：%v", err)
	}

	m2, d2 := boot(t)
	if err := Load(path, m2, d2); err != nil {
		t.Fatal(err)
	}
	if m2.Steps != m.Steps {
		t.Errorf("步數 %d，原本 %d", m2.Steps, m.Steps)
	}
	if m2.Ticks != m.Ticks {
		t.Errorf("Ticks %d，原本 %d", m2.Ticks, m.Ticks)
	}
	if m2.CPU.IP != m.CPU.IP || m2.CPU.Seg[cpu.CS] != m.CPU.Seg[cpu.CS] {
		t.Errorf("CS:IP %04X:%04X，原本 %04X:%04X",
			m2.CPU.Seg[cpu.CS], m2.CPU.IP, m.CPU.Seg[cpu.CS], m.CPU.IP)
	}
	for i := range m.Mem {
		if m.Mem[i] != m2.Mem[i] {
			t.Fatalf("記憶體 %05X 不同：%02X vs %02X", i, m.Mem[i], m2.Mem[i])
		}
	}

	// 再各跑一段，兩邊要走到同一個地方——**時鐘也要一起還原**，
	// 只比對當下的狀態看不出 nextIRQ0 漏抄。
	for i := 0; i < 20000; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
		if err := m2.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.Ticks != m2.Ticks {
		t.Errorf("續跑之後 Ticks %d vs %d——計時器沒還原", m.Ticks, m2.Ticks)
	}
	for i := range m.Mem {
		if m.Mem[i] != m2.Mem[i] {
			t.Fatalf("續跑之後記憶體 %05X 不同：%02X vs %02X", i, m.Mem[i], m2.Mem[i])
		}
	}
}
