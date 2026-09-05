package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// `int 16h` 的按鍵佇列。**佇列空的時候要維持原本的行為**——
// rich2 的輸入走 `int 21h AH=3Fh`，這裡回錯了會讓它以為有人在打字。

func TestInt16SaysNoKeyWhenQueueIsEmpty(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x16, 0x0100) // AH=01：查有沒有按鍵
	if !m.CPU.Flag(cpu.ZF) {
		t.Error("佇列空的卻回 ZF=0（有按鍵）")
	}
	call(m, d, 0x16, 0x0000) // AH=00：讀按鍵
	if m.CPU.R[cpu.AX] != 0 {
		t.Errorf("佇列空的卻讀出 AX=%04X", m.CPU.R[cpu.AX])
	}
}

func TestInt16PeekDoesNotConsume(t *testing.T) {
	m, d := newTest(t)
	d.Keys = []Key{{Scan: 0x21, ASCII: 'f'}}
	call(m, d, 0x16, 0x0100)
	if m.CPU.Flag(cpu.ZF) {
		t.Fatal("佇列有東西卻回 ZF=1（沒有按鍵）")
	}
	if m.CPU.R[cpu.AX] != 0x2166 {
		t.Errorf("AX = %04X，該是 2166", m.CPU.R[cpu.AX])
	}
	if len(d.Keys) != 1 {
		t.Error("查詢把按鍵吃掉了")
	}
}

func TestInt16ReadPopsOneKey(t *testing.T) {
	m, d := newTest(t)
	d.TypeKeys("ab")
	call(m, d, 0x16, 0x0000)
	if m.CPU.R[cpu.AX] != 0x0061 {
		t.Errorf("第一次讀出 AX=%04X，該是 0061", m.CPU.R[cpu.AX])
	}
	call(m, d, 0x16, 0x0000)
	if m.CPU.R[cpu.AX] != 0x0062 {
		t.Errorf("第二次讀出 AX=%04X，該是 0062", m.CPU.R[cpu.AX])
	}
	if len(d.Keys) != 0 {
		t.Errorf("讀完還剩 %d 個", len(d.Keys))
	}
}

func TestInt16CountsPolls(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x16, 0x0100)
	call(m, d, 0x16, 0x0000)
	// 「程式到底有沒有在等鍵盤」看這個數字最快，所以它要準。
	if d.KeyPolls != 2 {
		t.Errorf("KeyPolls = %d，該是 2", d.KeyPolls)
	}
}
