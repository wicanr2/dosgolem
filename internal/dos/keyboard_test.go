package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// newKeyboardDOS 造一個只用來問 int 16h 的服務層。
func newKeyboardDOS(t *testing.T) (*DOS, *cpu.CPU) {
	t.Helper()
	m := machine.New()
	d := New(m, t.TempDir())
	d.Install()
	return d, m.CPU
}

func call16(c *cpu.CPU, d *DOS, ah uint8) {
	c.R[cpu.AX] = uint16(ah) << 8
	c.SetFlags(0)
	d.int16(c)
}

// 佇列空的時候維持原行為：查詢設 ZF、讀取回 0。這一條是回歸保護——
// 第一個案例（rich2）不走 int 16h，改壞了它不會有任何症狀。
func TestEmptyKeyboardQueueStillReportsNothing(t *testing.T) {
	d, c := newKeyboardDOS(t)
	call16(c, d, 0x01)
	if c.Flags&cpu.ZF == 0 {
		t.Fatal("佇列空時 AH=01 應該設 ZF")
	}
	call16(c, d, 0x00)
	if c.R[cpu.AX] != 0 {
		t.Fatalf("佇列空時 AH=00 應該回 0，拿到 %04X", c.R[cpu.AX])
	}
	if d.KeysConsumed != 0 {
		t.Fatalf("沒有鍵可讀卻算了 %d 次", d.KeysConsumed)
	}
}

// AH=01 查詢**不取走**按鍵：連查兩次要拿到同一個，之後 AH=00 才取得走。
// 取走與不取走在畫面上完全一樣，只有這個測試分得開。
func TestPeekDoesNotConsumeTheKey(t *testing.T) {
	d, c := newKeyboardDOS(t)
	if !d.PushKeyNamed("Return") {
		t.Fatal("Return 應該在名稱表裡")
	}
	for round := 0; round < 2; round++ {
		call16(c, d, 0x01)
		if c.Flags&cpu.ZF != 0 {
			t.Fatalf("第 %d 次查詢誤報成沒有按鍵", round)
		}
		if c.R[cpu.AX] != 0x1C0D {
			t.Fatalf("第 %d 次查詢拿到 %04X，要 1C0D", round, c.R[cpu.AX])
		}
	}
	// 鍵可能停在 BDA 的環形緩衝或 DOS 自己的佇列裡（PushKey 先進環），
	// 所以要問兩邊加起來的數字。
	if d.KeysPending() != 1 {
		t.Fatalf("查詢把鍵吃掉了，佇列剩 %d", d.KeysPending())
	}
	call16(c, d, 0x00)
	if c.R[cpu.AX] != 0x1C0D || d.KeysPending() != 0 || d.KeysConsumed != 1 {
		t.Fatalf("AX=%04X 佇列=%d 讀走=%d", c.R[cpu.AX], d.KeysPending(), d.KeysConsumed)
	}
}

// 一整串文字照順序出來，小寫轉成大寫 ASCII 但掃描碼相同。
func TestPushTextKeepsOrderAndScanCodes(t *testing.T) {
	d, c := newKeyboardDOS(t)
	if !d.PushText("Hi 42") {
		t.Fatal("這幾個字元都該在表裡")
	}
	want := []uint16{0x2348, 0x1769, 0x3920, 0x0534, 0x0332}
	if d.KeysPending() != len(want) {
		t.Fatalf("排了 %d 個鍵，要 %d", d.KeysPending(), len(want))
	}
	// 從 `int 16h` 讀回來——排進去的順序與讀出來的順序是兩件事，
	// 而只看內部佇列的話兩條路（BDA 環、DOS 佇列）的接縫看不出來。
	for i, w := range want {
		call16(c, d, 0x00)
		if c.R[cpu.AX] != w {
			t.Fatalf("第 %d 個鍵是 %04X，要 %04X", i, c.R[cpu.AX], w)
		}
	}
	if d.KeysPending() != 0 {
		t.Fatalf("讀完還剩 %d 個", d.KeysPending())
	}
}

// 表外的字元整段拒絕。安靜地跳過一個字會讓後面整串輸入錯位，
// 而錯位的症狀出現在很後面、完全不指向這裡。
func TestPushTextRefusesUnknownRunes(t *testing.T) {
	d, _ := newKeyboardDOS(t)
	if d.PushText("OK!") {
		t.Fatal("驚嘆號不在表裡，應該回 false")
	}
}

// 方向鍵沒有 ASCII：低位元組是 0，程式靠掃描碼認。
func TestArrowKeysHaveNoASCII(t *testing.T) {
	for name, want := range map[string]uint16{
		"Up": 0x4800, "Down": 0x5000, "Left": 0x4B00, "Right": 0x4D00,
	} {
		k, ok := KeyNamed(name)
		if !ok || k.Word() != want {
			t.Fatalf("%s 是 %04X（ok=%v），要 %04X", name, k.Word(), ok, want)
		}
	}
}

// 主鍵區標點的 set 1 掃描碼，值對 DOSBox-X 的 `KEYBOARD_AddKey1`。
// Shift 那一層送同一顆鍵的掃描碼、不同的 ASCII。
func TestPunctuationScanCodes(t *testing.T) {
	for r, want := range map[rune]uint16{
		'-': 0x0C2D, '=': 0x0D3D,
		'[': 0x1A5B, ']': 0x1B5D,
		';': 0x273B, '\'': 0x2827, '`': 0x2960,
		'\\': 0x2B5C,
		',':  0x332C, '.': 0x342E, '/': 0x352F,
		// Shift 層：掃描碼與上面同一顆鍵相同，ASCII 換成符號本身。
		'_': 0x0C5F, '+': 0x0D2B,
		'{': 0x1A7B, '}': 0x1B7D,
		':': 0x273A, '"': 0x2822, '~': 0x297E,
		'|': 0x2B7C,
		'<': 0x333C, '>': 0x343E, '?': 0x353F,
	} {
		k, ok := KeyForRune(r)
		if !ok || k.Word() != want {
			t.Fatalf("%q 是 %04X（ok=%v），要 %04X", r, k.Word(), ok, want)
		}
	}
	// 反面對照：表外的字元仍要回 false，不能因為新增了標點就變成「什麼都認」。
	for _, r := range []rune{'!', '@', '#', '€', '。'} {
		if _, ok := KeyForRune(r); ok {
			t.Fatalf("%q 不在表裡，應該回 false", r)
		}
	}
}
