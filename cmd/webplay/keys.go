package main

import "github.com/wicanr2/dosgolem/internal/dos"

// scanByCode 把瀏覽器的 KeyboardEvent.code 對到 IBM PC/AT set 1 掃描碼（`docs/spec/200-webplay` §5）。
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

// browserKey 把一次瀏覽器按鍵換成 BIOS 的（掃描碼, ASCII）。
// ASCII 取 key 的單一可列印字元；Enter／Esc／Backspace／Tab 給控制碼；方向鍵與功能鍵給 0。
func browserKey(code, key string) (dos.Key, bool) {
	scan, ok := scanByCode[code]
	if !ok {
		return dos.Key{}, false
	}
	var ascii uint8
	switch code {
	case "Enter", "NumpadEnter":
		ascii = 0x0D
	case "Escape":
		ascii = 0x1B
	case "Backspace":
		ascii = 0x08
	case "Tab":
		ascii = 0x09
	default:
		if len(key) == 1 && key[0] >= 0x20 && key[0] < 0x7F {
			ascii = key[0]
		}
	}
	return dos.Key{Scan: scan, ASCII: ascii}, true
}
