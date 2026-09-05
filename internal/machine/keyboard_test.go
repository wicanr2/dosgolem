package machine

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 鍵盤這條路每一條規則的反面都不會報錯：中斷照送、程式照跑，
// 只是按鍵永遠不會出現。所以判準都寫成「掃描碼有沒有真的送到」。

const testHandlerSeg = 0x4000

// armKeyboard 造一台裝好 int 09h、堆疊可用、IF 開著的機器。
func armKeyboard(t *testing.T) *Machine {
	t.Helper()
	m := New()
	m.Write16(0x09*4, 0)
	m.Write16(0x09*4+2, testHandlerSeg)
	m.Mem[testHandlerSeg*16] = 0xCF // iret
	m.CPU.Seg[cpu.SS] = 0x3000
	m.CPU.R[cpu.SP] = 0xFFF0
	m.CPU.SetFlags(cpu.IF)
	return m
}

func TestTypeScanEmitsMakeThenBreak(t *testing.T) {
	m := New()
	if err := m.TypeScan("f"); err != nil {
		t.Fatal(err)
	}
	want := []uint8{0x21, 0x21 | 0x80}
	if len(m.KeyQueue) != len(want) {
		t.Fatalf("排了 %d 個掃描碼：% X", len(m.KeyQueue), m.KeyQueue)
	}
	for i, w := range want {
		if m.KeyQueue[i] != w {
			t.Errorf("第 %d 個是 %02X，該是 %02X", i, m.KeyQueue[i], w)
		}
	}
}

func TestTypeScanWrapsUppercaseInShift(t *testing.T) {
	m := New()
	if err := m.TypeScan("F"); err != nil {
		t.Fatal(err)
	}
	// 沒有 Shift 的話處理常式送出來的是小寫，而且從結果看不出差別。
	want := []uint8{0x2A, 0x21, 0x21 | 0x80, 0x2A | 0x80}
	if len(m.KeyQueue) != len(want) {
		t.Fatalf("排了 % X", m.KeyQueue)
	}
	for i, w := range want {
		if m.KeyQueue[i] != w {
			t.Fatalf("排出來的是 % X，該是 % X", m.KeyQueue, want)
		}
	}
}

func TestTypeScanRejectsUnknownRune(t *testing.T) {
	m := New()
	// 安靜跳過會讓後面整串輸入錯位，而且從結果完全看不出來。
	if err := m.TypeScan("féx"); err == nil {
		t.Fatal("預期要有錯誤，卻成功了")
	}
}

func TestKeyboardIRQNeedsAHandlerInstalled(t *testing.T) {
	m := New()
	m.CPU.SetFlags(cpu.IF)
	if err := m.TypeScan("f"); err != nil {
		t.Fatal(err)
	}
	m.keyTick()
	// 向量還指著 stub 就沒有人會讀掃描碼；這時送出去等於把它丟掉。
	if m.KeyIRQs != 0 || len(m.KeyQueue) != 2 {
		t.Fatalf("向量還指著 stub 卻送了 %d 次中斷，佇列剩 %d", m.KeyIRQs, len(m.KeyQueue))
	}
}

func TestKeyboardIRQHoldsWhileInterruptsAreOff(t *testing.T) {
	m := armKeyboard(t)
	m.CPU.SetFlags(0)
	if err := m.TypeScan("f"); err != nil {
		t.Fatal(err)
	}
	m.keyTick()
	if m.KeyIRQs != 0 {
		t.Fatal("IF 關著還是送了鍵盤中斷")
	}
}

func TestKeyboardIRQDeliversScanCodeOnPort60(t *testing.T) {
	m := armKeyboard(t)
	if err := m.TypeScan("f"); err != nil {
		t.Fatal(err)
	}
	m.keyTick()
	if m.KeyIRQs != 1 {
		t.Fatalf("送了 %d 次中斷，該是 1", m.KeyIRQs)
	}
	if got := m.In8(0x60); got != 0x21 {
		t.Errorf("埠 60h 讀出 %02X，該是 21（f 的通碼）", got)
	}
	if m.CPU.Seg[cpu.CS] != testHandlerSeg {
		t.Errorf("中斷之後 CS = %04X，沒有跳進處理常式", m.CPU.Seg[cpu.CS])
	}
}

func TestKeyboardIRQWaitsBetweenKeys(t *testing.T) {
	m := armKeyboard(t)
	m.KeyIRQEvery = 1000
	if err := m.TypeScan("fg"); err != nil {
		t.Fatal(err)
	}
	m.keyTick()
	m.keyTick() // 同一個 Steps，第二個不該出去
	if m.KeyIRQs != 1 {
		t.Fatalf("同一瞬間送了 %d 次；前一個掃描碼還沒被讀走就被蓋掉", m.KeyIRQs)
	}
	// 中斷會把 IF 清掉（8086 就是這樣），處理常式 IRET 之後才恢復。
	// 這裡直接模擬「處理常式跑完了」。
	m.CPU.SetFlags(cpu.IF)
	m.Steps += 1000
	m.keyTick()
	if m.KeyIRQs != 2 {
		t.Fatalf("隔了 KeyIRQEvery 之後只送了 %d 次", m.KeyIRQs)
	}
}
