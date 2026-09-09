package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 鍵盤輸入的三條路（合併之後的統一契約）。
//
// 一台真機上，鍵盤只有一條路：IRQ1 把掃描碼放進埠 60h，BIOS 的 `int 09h`
// 把它翻成「掃描碼 ＋ ASCII」推進 BDA 的環形緩衝，`int 16h` 從那裡取，
// `int 21h` 的字元輸入再走 `int 16h`。dosgolem 為了可重播另外開了兩條捷徑：
//
//   - `DOS.Keys`：直接放「掃描碼 ＋ ASCII」的字組（環滿了才用到）
//   - `DOS.Stdin`：位元組佇列，`int 21h AH=3Fh`（編譯後 BASIC 的 INKEY$）
//     與 `int 16h` 共用
//
// 三條路同時存在的風險是**同一個鍵被讀兩次**或**被讀走一次之後另一條路
// 還看得到**，兩種症狀在畫面上都只是「輸入怪怪的」。這一份把順序與
// 「取走就不見」釘住。

// 直接讀 BDA 的程式（不叫 int 16h）要看得到排進去的鍵。
//
// 頭指標與尾指標不同就是「有鍵」——很多 DOS 程式只比這兩個 word。
func TestPushedKeyIsVisibleInTheBDARing(t *testing.T) {
	m, d := newTest(t)
	head := func() uint16 { return m.Read16(0x0040*16 + 0x1A) }
	tail := func() uint16 { return m.Read16(0x0040*16 + 0x1C) }

	if head() != tail() {
		t.Fatalf("一開始就有鍵？head=%04X tail=%04X", head(), tail())
	}
	d.PushKey(Key{Scan: 0x1C, ASCII: '\r'})
	if head() == tail() {
		t.Fatal("排了一個鍵，BDA 的頭尾指標卻還相等——只讀 BDA 的程式看不到它")
	}
	if got := m.Read16(0x0040*16 + uint32(head())); got != 0x1C0D {
		t.Errorf("緩衝區裡是 %04X，預期 1C0D", got)
	}

	// int 16h 取走之後，頭尾要回到相等。
	call(m, d, 0x16, 0x0000)
	if m.CPU.R[cpu.AX] != 0x1C0D {
		t.Fatalf("int 16h 讀到 %04X，預期 1C0D", m.CPU.R[cpu.AX])
	}
	if head() != tail() {
		t.Errorf("取走之後 head=%04X tail=%04X，還是「有鍵」的狀態", head(), tail())
	}
}

// 環形緩衝滿了不能安靜地丟鍵：`PushKey` 溢位到 DOS 自己的佇列，
// 而 `int 16h` 讀完環之後要接著讀它，順序不變。
//
// 丟掉的話症狀是「打一長串字，中間少了幾個」——而少掉的位置每次都一樣，
// 看起來像程式自己吃字。
func TestOverflowKeepsOrderInsteadOfDroppingKeys(t *testing.T) {
	m, d := newTest(t)
	const n = 20 // 環只放得下 15 個
	for i := 0; i < n; i++ {
		d.PushKey(Key{Scan: uint8(0x02 + i), ASCII: uint8('a' + i)})
	}
	if got := d.KeysPending(); got != n {
		t.Fatalf("排了 %d 個鍵，KeysPending 回 %d", n, got)
	}
	for i := 0; i < n; i++ {
		call(m, d, 0x16, 0x0000)
		want := uint16(0x02+i)<<8 | uint16('a'+i)
		if m.CPU.R[cpu.AX] != want {
			t.Fatalf("第 %d 個讀到 %04X，預期 %04X——順序在兩條佇列的接縫上斷了",
				i, m.CPU.R[cpu.AX], want)
		}
	}
	if got := d.KeysPending(); got != 0 {
		t.Errorf("讀完還剩 %d 個", got)
	}
}

// `AH=01h`（查詢）不能消耗佇列，`AH=00h`（讀取）才取走。
// 兩者搞混的症狀是「輪詢一次就把鍵吃掉」，程式永遠讀不到自己剛看到的鍵。
func TestPeekAndReadDifferOnConsumption(t *testing.T) {
	m, d := newTest(t)
	d.PushKey(Key{Scan: 0x39, ASCII: ' '})
	for i := 0; i < 3; i++ {
		call(m, d, 0x16, 0x0100)
		if m.CPU.Flags&cpu.ZF != 0 {
			t.Fatalf("第 %d 次查詢說沒有鍵", i)
		}
		if m.CPU.R[cpu.AX] != 0x3920 {
			t.Fatalf("第 %d 次查詢拿到 %04X，預期 3920", i, m.CPU.R[cpu.AX])
		}
	}
	call(m, d, 0x16, 0x0000)
	call(m, d, 0x16, 0x0100)
	if m.CPU.Flags&cpu.ZF == 0 {
		t.Error("取走之後查詢還說有鍵")
	}
}

// Stdin 是 `int 21h` 與 `int 16h` 共用的可重播佇列：同一個位元組
// **只能被讀走一次**，不管是哪一條路讀的。
func TestStdinIsSharedAndConsumedOnce(t *testing.T) {
	m, d := newTest(t)
	d.Stdin = []byte{'a', 'b'}

	call(m, d, 0x16, 0x0000) // BIOS 讀 'a'
	if got := uint8(m.CPU.R[cpu.AX]); got != 'a' {
		t.Fatalf("int 16h 讀到 %q，預期 a", got)
	}
	call(m, d, 0x21, 0x0100) // DOS 帶回顯讀 'b'
	if got := uint8(m.CPU.R[cpu.AX]); got != 'b' {
		t.Fatalf("int 21h AH=01h 讀到 %q，預期 b", got)
	}
	if len(d.Stdin) != 0 {
		t.Errorf("佇列還剩 %d 個位元組", len(d.Stdin))
	}
}

// 硬體路徑（IRQ1 ＋ 埠 60h）與 BIOS 路徑是分開的。
//
// **向量還指著 stub 的時候不能送**：送出去等於丟進垃圾桶（stub 只會 iret），
// 那個鍵就此消失，症狀是「第一個鍵沒反應、後面的才有」。
func TestIRQ1WaitsForTheProgramToInstallItsHandler(t *testing.T) {
	m, _ := newTest(t)
	m.QueueScan(0x1E, 0x9E)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)

	before := m.KeyQueueLen()
	m.Step()
	if m.IRQ1Delivered() != 0 {
		t.Fatal("向量還指著 stub 就送了 IRQ1——那個鍵會消失")
	}
	if m.KeyQueueLen() != before {
		t.Fatal("沒送出去卻把鍵從佇列拿走了")
	}
	if m.KeyStalls() == 0 {
		t.Error("沒送出去要記一筆 KeyStalls，否則與「遊戲不理會」分不開")
	}

	// 程式裝上自己的處理常式之後就送得出去。
	m.Write16(0x09*4, 0)
	m.Write16(0x09*4+2, 0x0900)
	m.WriteBytes(cpu.Addr(0x0900, 0), []byte{0xCF}) // iret
	m.SetNextKey(0)
	m.Step()
	if m.IRQ1Delivered() == 0 {
		t.Error("裝了處理常式還是送不出去")
	}
}

// 掃描碼一定是「按下 ＋ 放開」成對送。
//
// 只送按下的話，**等放開的程式會永遠停在那裡**，而只看按下的程式完全正常
// ——症狀因程式而異，查起來很貴。
func TestPushKeyQueuesMakeAndBreak(t *testing.T) {
	m, _ := newTest(t)
	m.PushKey(0x1E)
	codes := m.KeyCodes()
	if len(codes) != 2 || codes[0] != 0x1E || codes[1] != 0x9E {
		t.Fatalf("排出來的掃描碼是 % X，預期 1E 9E", codes)
	}
}

// int 16h 的 AX 是「AH ＝ 掃描碼、AL ＝ ASCII」。
//
// 只填 AL 的話，用 AH 判鍵的程式收到 0，而 **0 是延伸鍵的前綴**——
// 它會再讀一次來拿方向鍵的碼，然後永遠等不到第二個位元組。
func TestInt16ReturnsScanCodeInAH(t *testing.T) {
	m, d := newTest(t)
	d.Stdin = []byte{'1'}
	call(m, d, 0x16, 0x0000)
	if got := m.CPU.R[cpu.AX]; got != 0x0231 {
		t.Errorf("讀 '1' 得到 AX=%04X，預期 0231（掃描碼 02h）", got)
	}
}

// 沒有鍵的時候 `AH=01h` 要設 ZF，而且 **AX 不能留下上一次的值**當成「有鍵」。
func TestInt16ReportsEmptyWithZeroFlag(t *testing.T) {
	m, d := newTest(t)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.ZF)
	call(m, d, 0x16, 0x0100)
	if m.CPU.Flags&cpu.ZF == 0 {
		t.Fatal("佇列空卻沒設 ZF——呼叫端會拿 AX 裡的垃圾當按鍵")
	}
}

// BDA 的鍵盤緩衝區位址要與 IBM 的定義一致（`0040:001E`，16 筆 word）。
// 位址錯了程式讀到的是別的東西，而那東西通常不是 0，看起來就像「一直有鍵」。
func TestBDARingLivesWhereProgramsExpect(t *testing.T) {
	m, d := newTest(t)
	d.PushKey(Key{Scan: 0x02, ASCII: '1'})
	if got := m.Read16(0x0040*16 + 0x1E); got != 0x0231 {
		t.Errorf("0040:001E 是 %04X，預期第一個鍵 0231", got)
	}
	if got := m.Read16(0x0040*16 + 0x1C); got != 0x20 {
		t.Errorf("尾指標是 %04X，第一次寫完應該指到 0020h", got)
	}
}
