package machine

import "fmt"

// BIOS int 09h 的預設處理改成客體碼，呼叫 `int 15h AH=4Fh` 鍵盤攔截
// （`docs/spec/447-bios-int09-int15-keyboard-intercept`）。
//
// 真機的 int 09h 讀到掃描碼之後以 `AH=4Fh`、`CF=1` 呼叫 `int 15h`，回來 `CF=0` 就丟掉這個碼。
// 有些程式不掛 int 09h，改在這個攔截點收每一個按下與放開碼（《Strike Commander》的 `STRIKE.EXE`，
// 447 §1.2）。Go 端的 `biosInt9`（202）直接翻譯放進緩衝，不經 `int 15h`，這種程式就收不到鍵，
// 而且不報錯：IRQ1 送了、緩衝也有鍵，只是遊戲自己的按鍵常式從來沒被呼叫。
//
// 這段碼與 DOSBox-X 的 `CB_IRQ1`（`src/cpu/callback.cpp` 413–475 行）同形，實模式剖面差在不送 EOI（447 §2.2）；
// 保護模式剖面有主片在服務模型，寫帶 EOI 的 19 bytes 版本（docs/spec/459 §3.3a）。

// kbdInterceptCode 是 KbdInterceptOff 起的 14 bytes（447 §2.2 的表）。
var kbdInterceptCode = [...]byte{
	0x50,       // push ax
	0xE4, 0x60, // in al,60h
	0xB4, 0x4F, // mov ah,4Fh
	0xF9,       // stc
	0xCD, 0x15, // int 15h
	0x73, 0x02, // jnc +2（跳過 int FAh）
	0xCD, 0xFA, // int FAh：internal/dos 以 AL 為掃描碼做 202 的翻譯
	0x58, // pop ax
	0xCF, // iret
}

// kbdInterceptCodeEOI 是保護模式剖面的 19 bytes 版本（docs/spec/459 §3.3a）：在 `pop ax` 之前加
// `cli; mov al,20h; out 20h,al`（DOSBox-X CB_IRQ1，callback.cpp:462-466），`jnc +2` 的目標變成這段 EOI，
// CF=0 與 CF=1 兩條路都送。佔 0x4B8–0x4CA。實模式剖面照 447 的 14 bytes，不送 EOI。
var kbdInterceptCodeEOI = [...]byte{
	0x50,       // push ax
	0xE4, 0x60, // in al,60h
	0xB4, 0x4F, // mov ah,4Fh
	0xF9,       // stc
	0xCD, 0x15, // int 15h
	0x73, 0x02, // jnc +2（跳過 int FAh，落在 EOI）
	0xCD, 0xFA, // int FAh
	0xFA,       // cli
	0xB0, 0x20, // mov al,20h
	0xE6, 0x20, // out 20h,al
	0x58, // pop ax
	0xCF, // iret
}

// kbdInterceptBytes 回這台機器要寫的鍵盤攔截客體碼：保護模式剖面為 19 bytes 版本，否則 447 的 14 bytes。
func (m *Machine) kbdInterceptBytes() []byte {
	if m.pmProfile {
		return kbdInterceptCodeEOI[:]
	}
	return kbdInterceptCode[:]
}

// kbdStub9 是向量 9 的 per-vector stub 改寫後的 3 bytes：`jmp near KbdInterceptOff`。
// 位移以下一道指令（StubOff(9)+3）起算。
func kbdStub9() [3]byte {
	rel := uint16(KbdInterceptOff) - (StubOff(0x09) + 3)
	return [3]byte{0xE9, uint8(rel), uint8(rel >> 8)}
}

// EnableKbdIntercept 寫入 447 §2.2 的客體碼（保護模式剖面為 459 §3.3a 的 19 bytes 版本）並把向量 9 的 stub 改成跳過去，逐 byte 讀回驗證；
// 全部相符才設 KbdIntercept 為真並回 nil。不符回錯誤，KbdIntercept 不改。可重複呼叫，結果相同。
//
// 寫入與讀回走 WriteSup8／ReadSup8（客體線性視角、監督者身分）。⚠ **不能用 WriteBytes**：
// 它經 Write8，轉換開啟且 cplUser（V86）時以使用者身分檢查頁面權限，線性 0 頁是監督者限定的話
// 寫入被安靜丟掉（`linear.go` 的 writeLin），而 V86 客體執行的是線性位址上的位元組。
//
// `IVT[09h]` 不改：沒掛 int 09h 的程式與掛了再 chain 回 `StubSeg:StubOff(9)` 的程式都走到這段碼。
// 被蓋掉的 `CD 09 CF` 是 per-vector stub「不被蓋」不變量的唯一例外（447 §2.2）。
func (m *Machine) EnableKbdIntercept() error {
	base := uint32(StubSeg) * 16
	stub := kbdStub9()
	type span struct {
		at uint32
		b  []byte
	}
	spans := []span{
		{base + uint32(KbdInterceptOff), m.kbdInterceptBytes()}, // 459 §3.3a：保護模式剖面寫 19 bytes 版本
		{base + uint32(StubOff(0x09)), stub[:]},
	}
	for _, s := range spans {
		for i, v := range s.b {
			m.WriteSup8(s.at+uint32(i), v)
		}
	}
	for _, s := range spans {
		for i, v := range s.b {
			if got := m.ReadSup8(s.at + uint32(i)); got != v {
				return fmt.Errorf("machine: 鍵盤攔截客體碼寫入後讀回不符：線性 %05X 是 %02X，應為 %02X"+
					"（docs/spec/447 §2.1）", s.at+uint32(i), got, v)
			}
		}
	}
	m.KbdIntercept = true
	return nil
}

// KbdInterceptStub9 回向量 9 的 stub 在選項開啟時的 3 bytes（`E9 91 04`）。
// 給前端做「狀態檔是在選項開啟時存的」的不一致偵測（447 §2.1）。
func KbdInterceptStub9() [3]byte { return kbdStub9() }

// CheckKbdInterceptAbsent 檢查向量 9 的 stub 不是 KbdInterceptStub9()（447 §2.1）。
// 讀檔之後、鍵盤攔截選項關閉時呼叫：stub 是 `E9 91 04` 表示狀態檔是在選項開啟時存的，回錯誤。
// 只讀不寫（ReadSup8）。cmd/probe 的 -kbd-int15 與 session 的 BootFromState（docs/spec/450 §2.3 第 6 步）共用。
func (m *Machine) CheckKbdInterceptAbsent() error {
	at := uint32(StubSeg)*16 + uint32(StubOff(0x09))
	got := [3]byte{m.ReadSup8(at), m.ReadSup8(at + 1), m.ReadSup8(at + 2)}
	if got == kbdStub9() {
		return fmt.Errorf("machine: 向量 9 的 stub 是 % X：狀態檔是在鍵盤攔截開啟時存的，讀檔時要同樣開啟"+
			"（docs/spec/447 §2.1、§4）", got[:])
	}
	return nil
}
