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
	// 擴充鍵區 47h..53h 的**功能名**。`Up`／`Down`／`Left`／`Right` 只涵蓋
	// 四個方向，而清單類的介面常常用 Home／End／PgUp／PgDn 上下移動——
	// 《Pool of Radiance》的種族清單就是（`Home` 上移、`End` 下移，
	// `Down` 一格都不動；見那邊的 spec 133）。
	"Home":     {0x47, 0x00},
	"End":      {0x4F, 0x00},
	"PgUp":     {0x49, 0x00},
	"PageUp":   {0x49, 0x00},
	"PgDn":     {0x51, 0x00},
	"PageDown": {0x51, 0x00},
	"Insert":   {0x52, 0x00},
	"Ins":      {0x52, 0x00},
	"Delete":   {0x53, 0x00},
	"Del":      {0x53, 0x00},
	// 同一圈鍵的**位置名**。數字鍵盤上 `7 8 9` 在上、`1 2 3` 在下，
	// 有些程式（含 Gold Box 的戰鬥移動）是照那個排法想的，不是照
	// Home／End 想的。兩套都留著——腳本要表達的是意圖。
	"KP7":   {0x47, 0x00},
	"KP8":   {0x48, 0x00},
	"KP9":   {0x49, 0x00},
	"KP4":   {0x4B, 0x00},
	"KP5":   {0x4C, 0x00},
	"KP6":   {0x4D, 0x00},
	"KP1":   {0x4F, 0x00},
	"KP2":   {0x50, 0x00},
	"KP3":   {0x51, 0x00},
	"KP0":   {0x52, 0x00},
	"KPDot": {0x53, 0x00},
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

// punctScan 是主鍵區標點的 set 1 掃描碼，值取自 DOSBox-X 的
// `KEYBOARD_AddKey1`（`src/hardware/keyboard.cpp`，scancode set 1 那一組；
// 另外三組 AddKey2／AddKey3／AddKeyPCjr 的值不同，不能混用）。
// 每一項的 ASCII 直接用字元本身。
var punctScan = map[byte]uint8{
	'-': 0x0C, '=': 0x0D,
	'[': 0x1A, ']': 0x1B,
	';': 0x27, '\'': 0x28, '`': 0x29,
	'\\': 0x2B,
	',':  0x33, '.': 0x34, '/': 0x35,
}

// shiftedPunct 是按住 Shift 之後那一層：掃描碼與未按 Shift 的同一顆鍵相同，
// 送進 BIOS 緩衝的 ASCII 不同。表以 US 版面為準。
var shiftedPunct = map[byte]byte{
	'_': '-', '+': '=',
	'{': '[', '}': ']',
	':': ';', '"': '\'', '~': '`',
	'|': '\\',
	'<': ',', '>': '.', '?': '/',
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
	if scan, ok := punctScan[byte(r)]; ok {
		return Key{scan, uint8(r)}, true
	}
	// Shift 那一層送同一顆鍵的掃描碼，ASCII 用符號本身——原版讀的是 BIOS
	// 緩衝的 ASCII，掃描碼只在它自己查鍵位時才看。
	if base, ok := shiftedPunct[byte(r)]; ok {
		if scan, ok := punctScan[base]; ok {
			return Key{scan, uint8(r)}, true
		}
	}
	if r == ' ' {
		return namedKeys["Space"], true
	}
	return Key{}, false
}
