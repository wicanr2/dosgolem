package dos

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"testing"
)

func TestDOSReadsTheKeyObservedInBDA(t *testing.T) {
	for _, fn := range []uint16{1, 7, 8} {
		m, d := newTest(t)
		m.PushBIOSKey(7, '6')
		if m.Read16(0x41a) == m.Read16(0x41c) {
			t.Fatal("BDA 必須可見待讀字元")
		}
		call(m, d, 0x21, fn<<8)
		if byte(m.CPU.R[cpu.AX]) != '6' || d.Blocked || d.KeysConsumed != 1 {
			t.Fatalf("AH%02X 沒有讀到 BDA 的字元", fn)
		}
		if m.Read16(0x41a) != m.Read16(0x41c) {
			t.Fatal("已取走的鍵仍留在 BDA")
		}
	}
}

func TestDOSBIOSBridgeExtendedAndStdinPriority(t *testing.T) {
	m, d := newTest(t)
	m.PushBIOSKey(0x50, 0)
	d.Stdin = []byte{'A'}
	for _, want := range []byte{'A', 0, 0x50} {
		call(m, d, 0x21, 0x0800)
		if byte(m.CPU.R[cpu.AX]) != want || d.Blocked {
			t.Fatalf("取字元要 %02X，得到 AX=%04X", want, m.CPU.R[cpu.AX])
		}
	}
	if d.KeysConsumed != 1 || len(d.Stdin) != 0 || len(d.KeyReads) != 3 {
		t.Fatal("同一硬體鍵被雙讀或擴充碼遺失")
	}
}
