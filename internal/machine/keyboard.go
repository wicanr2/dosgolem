package machine

import "github.com/wicanr2/dosgolem/internal/cpu"

// 硬體鍵盤（IRQ1 ＝ `int 09h`，資料埠 `0x60`）。
//
// ⚠ **BIOS 的 `int 16h` 不是唯一的鍵盤入口。** 遊戲常常自己裝 IRQ1 處理常式、
// 直接從埠 `0x60` 讀掃描碼，繞過整個 BIOS。這種程式在只有 `int 16h` 的
// 執行器上**不會報錯**：它照樣輪詢、照樣畫動畫、照樣等，只是永遠等不到鍵。
// 從外面看是「選單沒反應」，看起來像按鍵送錯，而不是像少了一整個裝置。
//
// 智冠《三國演義》的主選單就是這樣（`docs/spec/014`）：
//
//	in  al,0x60          ; 讀掃描碼
//	test al,0x80         ; 放開鍵不理
//	jne  結束
//	mov  es:[0004],al    ; 存到 0000:0004
//	mov  es:[0005],1     ; 「有鍵」旗標
//	in   al,0x61 / or 80h / out / and 7Fh / out   ; 鍵盤 ack
//	mov  al,0x20 / out 0x20,al                    ; EOI
//
// 它同時也呼叫 `_bios_keybrd(1)`（`int 16h AH=01`）幾十萬次，
// 所以「BIOS 有被呼叫」不足以證明鍵是從 BIOS 進去的。

// KeyEvent 是一個掃描碼事件。Break 為真表示放開。
type KeyEvent struct {
	Scan  uint8
	Break bool
}

// PushKey 把一次「按下 ＋ 放開」排進硬體佇列。
//
// 兩個都要送：只送按下的話，會**等放開**的程式永遠停在那裡，
// 而只看按下的程式不受影響——症狀因程式而異，找起來很貴。
func (m *Machine) PushKey(scan uint8) {
	m.keyQueue = append(m.keyQueue, KeyEvent{Scan: scan}, KeyEvent{Scan: scan, Break: true})
}

// KeyQueueLen 是還沒送出去的硬體鍵盤事件數。
func (m *Machine) KeyQueueLen() int { return len(m.keyQueue) }

// IRQ1Delivered 是已經送出去的 IRQ1 次數。
func (m *Machine) IRQ1Delivered() int { return m.irq1Count }

// IRQ1ToProgram 是其中有幾次跳進**程式自己的**處理常式。
//
// 兩個數字要分開看：向量還指著 stub 時送出去的中斷等於丟進垃圾桶，
// 而總次數看起來一樣漂亮。
func (m *Machine) IRQ1ToProgram() int { return m.irq1Own }

// keyTick 送 IRQ1。
//
// 節流的理由是**真鍵盤沒那麼快**：一送完就送下一個的話，程式的 ISR
// 還沒把上一個掃描碼從 `0x60` 搬走就被覆蓋，而覆蓋是安靜的。
// 這裡用指令數當時鐘，與 `tick` 同一個模型（決定性優先）。
func (m *Machine) keyTick() {
	if len(m.keyQueue) == 0 {
		return
	}
	if m.Steps < m.nextIRQ1 {
		return
	}
	if !m.CPU.Flag(cpu.IF) {
		return // 關中斷期間先擺著，不要丟掉
	}
	ev := m.keyQueue[0]
	m.keyQueue = m.keyQueue[1:]
	m.nextIRQ1 = m.Steps + m.KeyIRQEvery

	m.kbdData = ev.Scan
	if ev.Break {
		m.kbdData |= 0x80
	}
	m.irq1Count++
	if m.Read16(0x09*4+2) != StubSeg {
		m.irq1Own++
	}
	m.CPU.Interrupt(0x09)
}
