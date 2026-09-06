package dos

// BIOS 鍵盤的按鍵字組與名稱表（`docs/spec/008` §4）。
//
// 一個「鍵」在 `int 16h` 眼裡就是一個 16 位元字組：高位元組掃描碼、
// 低位元組 ASCII。方向鍵這種沒有 ASCII 的低位元組是 0，程式靠掃描碼認。
//
// **表只放本專案量到會用的鍵。** 猜出來的掃描碼錯了會表現成「按了沒反應」，
// 與「鍵根本沒送到」在畫面上完全一樣，查起來很貴。

// Key 是一次按鍵：掃描碼與 ASCII 碼。
type Key struct {
	Scan, ASCII uint8
}

// Word 是 `int 16h AH=00h` 放進 `AX` 的字組。
func (k Key) Word() uint16 { return uint16(k.Scan)<<8 | uint16(k.ASCII) }

// namedKeys 是有名字的鍵。名字用得起來的形式（`Return`、`Space`）而不是
// 掃描碼，呼叫端才讀得懂自己在按什麼。
var namedKeys = map[string]Key{
	"Return":    {0x1C, 0x0D},
	"Enter":     {0x1C, 0x0D},
	"Space":     {0x39, 0x20},
	"Esc":       {0x01, 0x1B},
	"Escape":    {0x01, 0x1B},
	"Backspace": {0x0E, 0x08},
	"Tab":       {0x0F, 0x09},
	"Up":        {0x48, 0x00},
	"Down":      {0x50, 0x00},
	"Left":      {0x4B, 0x00},
	"Right":     {0x4D, 0x00},
}

// letterScan 是 A..Z 的 set 1 掃描碼，照鍵盤的三列排。
var letterScan = map[byte]uint8{
	'Q': 0x10, 'W': 0x11, 'E': 0x12, 'R': 0x13, 'T': 0x14,
	'Y': 0x15, 'U': 0x16, 'I': 0x17, 'O': 0x18, 'P': 0x19,
	'A': 0x1E, 'S': 0x1F, 'D': 0x20, 'F': 0x21, 'G': 0x22,
	'H': 0x23, 'J': 0x24, 'K': 0x25, 'L': 0x26,
	'Z': 0x2C, 'X': 0x2D, 'C': 0x2E, 'V': 0x2F, 'B': 0x30,
	'N': 0x31, 'M': 0x32,
}

// digitScan 是 1..9、0 的掃描碼（`1` 是 02h，`0` 是 0Bh）。
var digitScan = map[byte]uint8{
	'1': 0x02, '2': 0x03, '3': 0x04, '4': 0x05, '5': 0x06,
	'6': 0x07, '7': 0x08, '8': 0x09, '9': 0x0A, '0': 0x0B,
}

// KeyNamed 查一個有名字的鍵。查不到就回 false——**不要回一個看起來合理的
// 預設值**，那會讓「名字打錯」變成「原版行為不同」。
func KeyNamed(name string) (Key, bool) {
	k, ok := namedKeys[name]
	return k, ok
}

// KeyForRune 把一個可列印字元換成按鍵。小寫字母照大寫送（掃描碼相同，
// ASCII 用原字元）；表外的字元回 false。
func KeyForRune(r rune) (Key, bool) {
	if r >= 'a' && r <= 'z' {
		if scan, ok := letterScan[byte(r-'a'+'A')]; ok {
			return Key{scan, uint8(r)}, true
		}
		return Key{}, false
	}
	if scan, ok := letterScan[byte(r)]; ok {
		return Key{scan, uint8(r)}, true
	}
	if scan, ok := digitScan[byte(r)]; ok {
		return Key{scan, uint8(r)}, true
	}
	if r == ' ' {
		return namedKeys["Space"], true
	}
	return Key{}, false
}
