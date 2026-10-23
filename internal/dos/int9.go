package dos

import "github.com/wicanr2/dosgolem/internal/cpu"

// BIOS 的 int 09h 預設處理（`docs/spec/202-bios-int09-default-handler`）。
//
// 硬體鍵盤（`machine.keyTick`）把掃描碼放在埠 0x60 再呼叫 `int 09h`。程式有兩種
// 接法：
//
//   - 自己的 ISR 讀 0x60（三國志的主選單）——不需要這一支；
//   - 自己的 ISR **只是個包裝**，做完記帳就 chain 回舊向量（`AESOP.EXE`：
//     `1779:0008` 遞增再遞減一個重入計數，中間 `pushf / call far [舊向量]`）。
//     舊向量是我們的 `CD 09 / CF` stub，**真機上那裡是 BIOS 的鍵盤處理**：
//     讀 0x60、把掃描碼翻成 (掃描碼, ASCII) 放進 BDA 的環形緩衝。
//
// 少了這一支，鍵一個都到不了程式讀的那份緩衝，而且**不會報錯**：IRQ1 送了、
// 處理常式也跑了，畫面只是對鍵完全沒反應。
//
// 只做 BIOS 做的那部分：修飾鍵狀態（BDA `0040:0017`）、通碼翻譯、放進緩衝。
// 不做 Pause／PrintScreen／Ctrl-Alt-Del／Num Lock 的小鍵盤翻譯、延伸前綴 E0
// 之後的碼（`docs/spec/202` §3）。

const (
	bdaSeg      = 0x0040
	bdaKbdFlags = 0x17 // 修飾鍵狀態：bit0 右 Shift、bit1 左 Shift、bit2 Ctrl、bit3 Alt
)

// 掃描碼 → ASCII（美式鍵盤，未按 Shift）。表外的位置是 0。
var unshiftedASCII = func() (t [0x80]uint8) {
	set := func(first uint8, chars string) {
		for i := 0; i < len(chars); i++ {
			t[first+uint8(i)] = chars[i]
		}
	}
	t[0x01] = 0x1B
	set(0x02, "1234567890-=")
	t[0x0E] = 0x08
	t[0x0F] = 0x09
	set(0x10, "qwertyuiop[]")
	t[0x1C] = 0x0D
	set(0x1E, "asdfghjkl;'`")
	set(0x2B, "\\zxcvbnm,./")
	t[0x39] = ' '
	return
}()

var shiftedASCII = func() (t [0x80]uint8) {
	set := func(first uint8, chars string) {
		for i := 0; i < len(chars); i++ {
			t[first+uint8(i)] = chars[i]
		}
	}
	t[0x01] = 0x1B
	set(0x02, "!@#$%^&*()_+")
	t[0x0E] = 0x08
	t[0x0F] = 0x09
	set(0x10, "QWERTYUIOP{}")
	t[0x1C] = 0x0D
	set(0x1E, `ASDFGHJKL:"~`)
	set(0x2B, "|ZXCVBNM<>?")
	t[0x39] = ' '
	return
}()

// extendedKey 回報這個掃描碼是不是「沒有 ASCII 的功能鍵」：F1–F10、方向鍵與
// 編輯鍵。BIOS 給它們 ASCII ＝ 0、掃描碼照原樣（未按修飾鍵時）。
func extendedKey(scan uint8) bool {
	switch {
	case scan >= 0x3B && scan <= 0x44: // F1–F10
		return true
	case scan >= 0x47 && scan <= 0x53 && scan != 0x4A && scan != 0x4C && scan != 0x4E:
		return true // Home ↑ PgUp ← → End ↓ PgDn Ins Del
	}
	return false
}

// intKbdTranslate 是 BIOS int 09h 客體碼（`machine.KbdInterceptOff`，`docs/spec/447` §2.2）呼叫的中斷號：
// `int 15h AH=4Fh` 回 CF=1 之後以 AL 為掃描碼翻譯放進緩衝。真機沒有這一支。
//
// ⚠ 不能用 F2–F9：F2–F6 是服務型中斷的 trampoline，F7／F8 是字型 stub，F9 是回呼返回哨兵。
const intKbdTranslate = 0xFA

// biosInt9 處理一次鍵盤中斷：埠 0x60 上的掃描碼。
func (d *DOS) biosInt9(c *cpu.CPU) {
	d.biosInt9Code(d.M.In8(0x60))
	// 459 §3.3a：保護模式剖面的 BIOS 預設處理送 EOI（DOSBox-X CB_IRQ1 的 out 20h,20h），經埠模型、記入 PortLog。
	if d.M.ProtectedModeProfile() {
		d.M.Out8(0x20, 0x20)
	}
}

// kbdTranslate 是 `int FAh`（447 §2.3）：AL 是 `int 15h AH=4Fh` 之後的掃描碼（攔截常式可以改它），
// **不再讀埠 60h**。
func (d *DOS) kbdTranslate(c *cpu.CPU) {
	d.KbdTranslates++
	d.biosInt9Code(al(c))
}

// biosInt9Code 是 202 §2 的翻譯：修飾鍵狀態、通碼翻譯、放進緩衝。
func (d *DOS) biosInt9Code(code uint8) {
	scan, release := code&0x7F, code&0x80 != 0
	flags := d.M.Read8(bdaSeg*16 + bdaKbdFlags)

	// 修飾鍵：只更新 BDA 的狀態，不產生按鍵。
	var bit uint8
	switch scan {
	case 0x36:
		bit = 0x01 // 右 Shift
	case 0x2A:
		bit = 0x02 // 左 Shift
	case 0x1D:
		bit = 0x04 // Ctrl
	case 0x38:
		bit = 0x08 // Alt
	}
	if bit != 0 {
		if release {
			flags &^= bit
		} else {
			flags |= bit
		}
		d.M.Write8(bdaSeg*16+bdaKbdFlags, flags)
		return
	}
	if release {
		return // 放開鍵不進緩衝
	}

	shift := flags&0x03 != 0
	ctrl := flags&0x04 != 0
	var ascii uint8
	switch {
	case extendedKey(scan):
		// ASCII ＝ 0
	case scan < 0x80:
		if shift {
			ascii = shiftedASCII[scan]
		} else {
			ascii = unshiftedASCII[scan]
		}
		if ctrl && ascii >= 'a' && ascii <= 'z' {
			ascii -= 'a' - 1
		} else if ctrl && ascii >= 'A' && ascii <= 'Z' {
			ascii -= 'A' - 1
		}
	}
	if ascii == 0 && !extendedKey(scan) {
		return // 沒有對應的字元也不是功能鍵（例如 Caps Lock 本身）：BIOS 不放進緩衝
	}
	d.PushKey(Key{Scan: scan, ASCII: ascii})
}
