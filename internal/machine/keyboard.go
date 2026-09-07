package machine

import "github.com/wicanr2/dosgolem/internal/cpu"

// 硬體鍵盤（IRQ1 ＝ `int 09h`，資料埠 `0x60`，控制埠 `0x61`）。
// 規格：`docs/spec/012`（IRQ1 與滑鼠倍率）、`docs/spec/014`（硬體鍵盤）。
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

// Code 是這個事件在埠 0x60 上的位元組（放開鍵是 bit7 立起來）。
func (e KeyEvent) Code() uint8 {
	if e.Break {
		return e.Scan | 0x80
	}
	return e.Scan
}

// PushKey 把一次「按下 ＋ 放開」排進硬體佇列。
//
// 兩個都要送：只送按下的話，會**等放開**的程式永遠停在那裡，
// 而只看按下的程式不受影響——症狀因程式而異，找起來很貴。
func (m *Machine) PushKey(scan uint8) {
	m.keyQueue = append(m.keyQueue, KeyEvent{Scan: scan}, KeyEvent{Scan: scan, Break: true})
}

// QueueKey 是 PushKey 的別名（按下 ＋ 放開）。
func (m *Machine) QueueKey(scan uint8) { m.PushKey(scan) }

// QueueScan 把**原始掃描碼**排進佇列：bit7 立著就是放開。
//
// 給「已經知道要送哪幾個碼」的呼叫端用；一般用 PushKey。
func (m *Machine) QueueScan(codes ...uint8) {
	for _, c := range codes {
		m.keyQueue = append(m.keyQueue, KeyEvent{Scan: c &^ 0x80, Break: c&0x80 != 0})
	}
}

// SetNextKey 設定第一個掃描碼要在第幾道指令送出。
func (m *Machine) SetNextKey(step uint64) { m.nextKey = step }

// KeyQueueLen 是還沒送出去的硬體鍵盤事件數。
func (m *Machine) KeyQueueLen() int { return len(m.keyQueue) }

// IRQ1Delivered 是已經送出去的 IRQ1 次數。
func (m *Machine) IRQ1Delivered() uint64 { return m.KeyIRQs }

// KeyStalls 是「佇列裡有鍵、但沒有人裝 int 09h」而沒送出去的次數。
//
// **送不出去與遊戲不理會是兩件事**，沒有這個數字就分不開：
// 兩者的畫面表現都是「按了沒反應」。這個數字大就是前者——
// 程式根本沒有自己的 IRQ1 處理常式，鍵要走 BIOS（`-keys`）進去。
func (m *Machine) KeyStalls() uint64 { return m.keyStalls }

// keyTick 送鍵盤中斷（IRQ1 ＝ `int 09h`）。
//
// 節流的理由是**真鍵盤沒那麼快**：一送完就送下一個的話，程式的 ISR
// 還沒把上一個掃描碼從 `0x60` 搬走就被覆蓋，而覆蓋是安靜的。
// 這裡用指令數當時鐘，與 `tick` 同一個模型（決定性優先）。
//
// ⚠ **向量還指著 stub 時不送，而且不從佇列拿走。** 送出去等於丟進垃圾桶
// （stub 只會記一筆未實作然後 iret），而那個鍵就此消失——症狀是
// 「第一個鍵沒反應，後面的才有」。留著等程式裝好處理常式再送。
func (m *Machine) keyTick() {
	if len(m.keyQueue) == 0 || m.KeyEvery == 0 || m.Steps < m.nextKey {
		return
	}
	// 與 IRQ0 同樣的理由：中斷關著的時候先留著，不要丟掉。
	if !m.CPU.Flag(cpu.IF) {
		return
	}
	if m.Read16(0x09*4+2) == StubSeg {
		m.keyStalls++
		return
	}
	m.nextKey = m.Steps + m.KeyEvery
	ev := m.keyQueue[0]
	m.keyQueue = m.keyQueue[1:]
	m.kbdData = ev.Code()
	m.KeyIRQs++
	m.CPU.Interrupt(0x09)
}
