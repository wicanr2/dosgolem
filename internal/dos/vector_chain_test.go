package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// TestChainingToSavedVectorDoesNotRecurse 釘住「常駐程式鏈回舊向量」不能變成無限遞迴。
//
// 標準寫法是 `AH=35h` 存下舊向量 → `AH=25h` 裝自己的 → 做完事 `jmp far` 回舊向量。
// 程式開機時存下來的那個「舊向量」就是我們種的 stub（`StubSeg:StubOff(n)`，
// 內容 `CD n / CF`）。分派時若只看「向量還指不指著 StubSeg」，這一道 `int n`
// 會被判成「程式自己裝了處理常式」而放行，於是跳回程式自己的處理常式——一圈。
//
// 這條錯了不會有錯誤訊息：機器照跑，只是每次計時器中斷多疊一層，
// SP 一路往下掉，畫面停在某一格。看起來像遊戲卡住，不像向量鏈接錯。
func TestChainingToSavedVectorDoesNotRecurse(t *testing.T) {
	m, _ := newTest(t)

	const (
		scratch = 0x2000 // 不要用 StubSeg，那一段是 stub 與 BIOS 常式的家
		saved   = 0x0400 // 存舊向量的地方
		handler = 0x0300
		code    = 0x0500
		spTop   = 0x0200
	)

	// 程式開機做的事：先存下舊向量，再把 1Ch 指到自己。
	m.Write16(cpu.Addr(scratch, saved), m.Read16(0x1C*4))
	m.Write16(cpu.Addr(scratch, saved+2), m.Read16(0x1C*4+2))
	m.Write16(0x1C*4, handler)
	m.Write16(0x1C*4+2, scratch)

	// 自己的處理常式：什麼都不做，直接 `jmp far cs:[saved]` 鏈回去。
	m.WriteBytes(cpu.Addr(scratch, handler), []byte{
		0x2E, 0xFF, 0x2E, byte(saved & 0xFF), byte(saved >> 8),
	})
	m.WriteBytes(cpu.Addr(scratch, code), []byte{0xCD, 0x1C, 0xF4}) // int 1Ch / hlt

	m.CPU.Seg[cpu.CS] = scratch
	m.CPU.IP = code
	m.CPU.Seg[cpu.SS] = scratch
	m.CPU.R[cpu.SP] = spTop

	for i := 0; i < 200 && !m.CPU.Halted; i++ {
		if err := m.Step(); err != nil {
			t.Fatalf("第 %d 道出錯：%v", i, err)
		}
	}
	if !m.CPU.Halted {
		t.Fatalf("200 道之後還沒停，SP=%04X（開始是 %04X）——鏈回舊向量遞迴了",
			m.CPU.R[cpu.SP], spTop)
	}
	if got := m.CPU.R[cpu.SP]; got != spTop {
		t.Errorf("回來之後 SP=%04X，要 %04X——中斷沒有平衡地返回", got, spTop)
	}
}

// TestProgramHandlerStillWins 釘住上面那條例外**不能**擴大成「一律接手」。
//
// 一般程式碼裡的 `int n`，向量指著程式自己時就該走程式自己的，
// 服務層不准插手。這一條掉了的症狀是常駐程式全部失效，而且不會報錯。
func TestProgramHandlerStillWins(t *testing.T) {
	m, d := newTest(t)

	const scratch = 0x2000
	m.Write16(0x21*4, 0x0100)
	m.Write16(0x21*4+2, scratch)
	m.CPU.Seg[cpu.CS] = scratch
	m.CPU.IP = 0x0500

	if d.handle(m.CPU, 0x21) {
		t.Error("向量指著程式自己，服務層卻接手了")
	}

	// 從我們那支 stub 裡發出來的同一道，才輪到服務層。stub 的位置由
	// 「上一道指令在哪」決定，所以要走真的執行路徑，不能只設 CS:IP。
	m.CPU.Seg[cpu.CS] = machine.StubSeg
	m.CPU.IP = machine.StubOff(0x21)
	m.CPU.Seg[cpu.SS] = scratch
	m.CPU.R[cpu.SP] = 0x0200
	m.CPU.R[cpu.AX] = 0x3000 // AH=30h 取 DOS 版本，服務層一定認得
	if err := m.Step(); err != nil {
		t.Fatalf("跑 stub 出錯：%v", err)
	}
	if m.CPU.R[cpu.AX]&0xFF == 0x00 {
		t.Errorf("stub 裡的 int 21h 沒被服務層接手，AX=%04X", m.CPU.R[cpu.AX])
	}
}
