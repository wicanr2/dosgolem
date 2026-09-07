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
	if len(d.Keys) != 1 {
		t.Fatalf("查詢把鍵吃掉了，佇列剩 %d", len(d.Keys))
	}
	call16(c, d, 0x00)
	if c.R[cpu.AX] != 0x1C0D || len(d.Keys) != 0 || d.KeysConsumed != 1 {
		t.Fatalf("AX=%04X 佇列=%d 讀走=%d", c.R[cpu.AX], len(d.Keys), d.KeysConsumed)
	}
}

// 一整串文字照順序出來，小寫轉成大寫 ASCII 但掃描碼相同。
func TestPushTextKeepsOrderAndScanCodes(t *testing.T) {
	d, _ := newKeyboardDOS(t)
	if !d.PushText("Hi 42") {
		t.Fatal("這幾個字元都該在表裡")
	}
	want := []uint16{0x2348, 0x1769, 0x3920, 0x0534, 0x0332}
	if len(d.Keys) != len(want) {
		t.Fatalf("排了 %d 個鍵，要 %d", len(d.Keys), len(want))
	}
	for i, w := range want {
		if d.Keys[i] != w {
			t.Fatalf("第 %d 個鍵是 %04X，要 %04X", i, d.Keys[i], w)
		}
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
