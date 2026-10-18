package main

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/dosgolem/internal/dos"
)

// scanByCode 把鍵名（ebiten 的 Key.String()，字母補成 `KeyA` 的形式，與瀏覽器 KeyboardEvent.code 相同）
// 對到 IBM PC/AT set 1 掃描碼。表與 cmd/webplay/keys.go 相同（`docs/spec/441-play-frontend` §2.4）。
var scanByCode = map[string]uint8{
	"Escape": 0x01, "Digit1": 0x02, "Digit2": 0x03, "Digit3": 0x04, "Digit4": 0x05,
	"Digit5": 0x06, "Digit6": 0x07, "Digit7": 0x08, "Digit8": 0x09, "Digit9": 0x0A,
	"Digit0": 0x0B, "Minus": 0x0C, "Equal": 0x0D, "Backspace": 0x0E, "Tab": 0x0F,
	"KeyQ": 0x10, "KeyW": 0x11, "KeyE": 0x12, "KeyR": 0x13, "KeyT": 0x14,
	"KeyY": 0x15, "KeyU": 0x16, "KeyI": 0x17, "KeyO": 0x18, "KeyP": 0x19,
	"BracketLeft": 0x1A, "BracketRight": 0x1B, "Enter": 0x1C, "NumpadEnter": 0x1C,
	"ControlLeft": 0x1D, "ControlRight": 0x1D,
	"KeyA": 0x1E, "KeyS": 0x1F, "KeyD": 0x20, "KeyF": 0x21, "KeyG": 0x22,
	"KeyH": 0x23, "KeyJ": 0x24, "KeyK": 0x25, "KeyL": 0x26, "Semicolon": 0x27,
	"Quote": 0x28, "Backquote": 0x29, "ShiftLeft": 0x2A, "Backslash": 0x2B,
	"KeyZ": 0x2C, "KeyX": 0x2D, "KeyC": 0x2E, "KeyV": 0x2F, "KeyB": 0x30,
	"KeyN": 0x31, "KeyM": 0x32, "Comma": 0x33, "Period": 0x34, "Slash": 0x35,
	"ShiftRight": 0x36, "AltLeft": 0x38, "AltRight": 0x38, "Space": 0x39,
	"F1": 0x3B, "F2": 0x3C, "F3": 0x3D, "F4": 0x3E, "F5": 0x3F,
	"F6": 0x40, "F7": 0x41, "F8": 0x42, "F9": 0x43, "F10": 0x44,
	"Home": 0x47, "ArrowUp": 0x48, "PageUp": 0x49, "ArrowLeft": 0x4B,
	"ArrowRight": 0x4D, "End": 0x4F, "ArrowDown": 0x50, "PageDown": 0x51,
	"Insert": 0x52, "Delete": 0x53,
}

// symbols 是數字列與符號鍵的 ASCII：[0] 不按 Shift、[1] 按 Shift（美式鍵盤）。
var symbols = map[string][2]uint8{
	"Digit1": {'1', '!'}, "Digit2": {'2', '@'}, "Digit3": {'3', '#'}, "Digit4": {'4', '$'},
	"Digit5": {'5', '%'}, "Digit6": {'6', '^'}, "Digit7": {'7', '&'}, "Digit8": {'8', '*'},
	"Digit9": {'9', '('}, "Digit0": {'0', ')'}, "Minus": {'-', '_'}, "Equal": {'=', '+'},
	"BracketLeft": {'[', '{'}, "BracketRight": {']', '}'}, "Semicolon": {';', ':'},
	"Quote": {'\'', '"'}, "Backquote": {'`', '~'}, "Backslash": {'\\', '|'},
	"Comma": {',', '<'}, "Period": {'.', '>'}, "Slash": {'/', '?'}, "Space": {' ', ' '},
}

// keyCode 回 ebiten 鍵的鍵名（字母補成 `KeyA`）。
func keyCode(k ebiten.Key) string {
	s := k.String()
	if len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z' {
		return "Key" + s
	}
	return s
}

// modifiers 是按鍵當下的修飾狀態。
type modifiers struct {
	shift, caps, ctrl, alt bool
}

// hostKey 把一個鍵名換成 BIOS 的（掃描碼, ASCII）；不認得的鍵回 false。
// 字母依 Shift 與 Caps Lock 分大小寫，Ctrl＋字母給 01h–1Ah，Alt 組合的 ASCII 為 0；
// Enter／Esc／Backspace／Tab 給控制碼；方向鍵與功能鍵給 0。
func hostKey(code string, mod modifiers) (dos.Key, bool) {
	scan, ok := scanByCode[code]
	if !ok {
		return dos.Key{}, false
	}
	var ascii uint8
	switch {
	case code == "Enter" || code == "NumpadEnter":
		ascii = 0x0D
	case code == "Escape":
		ascii = 0x1B
	case code == "Backspace":
		ascii = 0x08
	case code == "Tab":
		ascii = 0x09
	case mod.alt:
		ascii = 0
	case strings.HasPrefix(code, "Key") && len(code) == 4:
		c := code[3] // 'A'..'Z'
		switch {
		case mod.ctrl:
			ascii = c - 'A' + 1
		case mod.shift != mod.caps:
			ascii = c
		default:
			ascii = c + ('a' - 'A')
		}
	default:
		if s, ok := symbols[code]; ok && !mod.ctrl {
			if mod.shift {
				ascii = s[1]
			} else {
				ascii = s[0]
			}
		}
	}
	return dos.Key{Scan: scan, ASCII: ascii}, true
}
