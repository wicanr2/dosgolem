package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// `int 16h` 的按鍵佇列。**佇列空的時候要維持原本的行為**——
// rich2 的輸入走 `int 21h AH=3Fh`，這裡回錯了會讓它以為有人在打字。

func TestInt16SaysNoKeyWhenQueueIsEmpty(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x16, 0x0100) // AH=01：查有沒有按鍵
	if !m.CPU.Flag(cpu.ZF) {
		t.Error("佇列空的卻回 ZF=0（有按鍵）")
	}
	call(m, d, 0x16, 0x0000) // AH=00：讀按鍵
	if m.CPU.R[cpu.AX] != 0 {
		t.Errorf("佇列空的卻讀出 AX=%04X", m.CPU.R[cpu.AX])
	}
}

func TestInt16PeekDoesNotConsume(t *testing.T) {
	m, d := newTest(t)
	d.Keys = []uint16{Key{Scan: 0x21, ASCII: 'f'}.Word()}
	call(m, d, 0x16, 0x0100)
	if m.CPU.Flag(cpu.ZF) {
		t.Fatal("佇列有東西卻回 ZF=1（沒有按鍵）")
	}
	if m.CPU.R[cpu.AX] != 0x2166 {
		t.Errorf("AX = %04X，該是 2166", m.CPU.R[cpu.AX])
	}
	if len(d.Keys) != 1 {
		t.Error("查詢把按鍵吃掉了")
	}
}

func TestInt16ReadPopsOneKey(t *testing.T) {
	m, d := newTest(t)
	// TypeKeys 查得到掃描碼就填：'a' 是 1Eh、'b' 是 30h。
	// 只填 AL 的話，用 AH 判鍵的程式會收到 0，而 0 是延伸鍵的前綴。
	d.TypeKeys("ab")
	call(m, d, 0x16, 0x0000)
	if m.CPU.R[cpu.AX] != 0x1E61 {
		t.Errorf("第一次讀出 AX=%04X，該是 1E61", m.CPU.R[cpu.AX])
	}
	call(m, d, 0x16, 0x0000)
	if m.CPU.R[cpu.AX] != 0x3062 {
		t.Errorf("第二次讀出 AX=%04X，該是 3062", m.CPU.R[cpu.AX])
	}
	if len(d.Keys) != 0 {
		t.Errorf("讀完還剩 %d 個", len(d.Keys))
	}
}

func TestInt16CountsPolls(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x16, 0x0100)
	call(m, d, 0x16, 0x0000)
	// 「程式到底有沒有在等鍵盤」看這個數字最快，所以它要準。
	if d.KeyPolls != 2 {
		t.Errorf("KeyPolls = %d，該是 2", d.KeyPolls)
	}
}

// `KeyReads` 要記得住方向鍵（`docs/spec/185`）。
//
// **只看 `Key` 這個 ASCII byte 的話，方向鍵全部長成 0**——`Up`、`Down`、
// `Left`、`Right` 分不出來，而「鍵送進去了」與「程式吃掉了但不理它」在
// 報表上也長得一樣。追 Pool 的方向鍵時就是卡在這裡。
func TestKeyReadsRecordTheWholeWordAndCaller(t *testing.T) {
	m, d := newTest(t)
	d.PushKey(namedKeys["Down"])
	d.PushKey(namedKeys["KP1"])
	before := len(d.KeyReads)
	call(m, d, 0x16, 0x0000)
	call(m, d, 0x16, 0x0000)
	reads := d.KeyReads[before:]
	if len(reads) != 2 {
		t.Fatalf("讀走兩個鍵卻記了 %d 筆", len(reads))
	}
	if reads[0].Word != 0x5000 {
		t.Errorf("第一筆 Word = %04X，該是 5000（Down）", reads[0].Word)
	}
	if reads[1].Word != 0x4F00 {
		t.Errorf("第二筆 Word = %04X，該是 4F00（數字鍵盤 1）", reads[1].Word)
	}
	for index, read := range reads {
		if read.Key != 0 {
			t.Errorf("第 %d 筆的 ASCII 是 %02X，方向鍵沒有 ASCII", index, read.Key)
		}
		if read.Via == "" {
			t.Errorf("第 %d 筆沒有記從哪一條出口走的", index)
		}
	}
}

// 數字鍵盤那一圈：四個角與正中央本來沒有名字可用，而 Pool 的選單元件
// 把主鍵盤 `1`..`9` 轉成的正是這一組掃描碼。
func TestKeypadNamesCoverTheWholeRing(t *testing.T) {
	want := map[string]uint16{
		"KP7": 0x4700, "KP8": 0x4800, "KP9": 0x4900,
		"KP4": 0x4B00, "KP5": 0x4C00, "KP6": 0x4D00,
		"KP1": 0x4F00, "KP2": 0x5000, "KP3": 0x5100,
		"KP0": 0x5200, "KPDot": 0x5300,
	}
	for name, word := range want {
		key, ok := KeyNamed(name)
		if !ok {
			t.Errorf("不認得 %s", name)
			continue
		}
		if key.Word() != word {
			t.Errorf("%s 的字組是 %04X，該是 %04X", name, key.Word(), word)
		}
	}
	// 方向鍵那四個與數字鍵盤同碼，兩組都要留著。
	for _, pair := range [][2]string{{"Up", "KP8"}, {"Down", "KP2"}, {"Left", "KP4"}, {"Right", "KP6"}} {
		a, _ := KeyNamed(pair[0])
		b, _ := KeyNamed(pair[1])
		if a.Word() != b.Word() {
			t.Errorf("%s 與 %s 的字組不同（%04X／%04X）", pair[0], pair[1], a.Word(), b.Word())
		}
	}
	if _, ok := KeyNamed("KP99"); ok {
		t.Error("不存在的鍵名回報認得")
	}
}

// 擴充鍵區的功能名。清單類的介面常常用 Home／End／PgUp／PgDn 上下移動，
// 而 `Up`／`Down` 只涵蓋四個方向——Pool 的種族清單一格都不吃 `Down`。
func TestExtendedKeyNamesCoverHomeEndAndPaging(t *testing.T) {
	want := map[string]uint16{
		"Home": 0x4700, "End": 0x4F00,
		"PgUp": 0x4900, "PageUp": 0x4900,
		"PgDn": 0x5100, "PageDown": 0x5100,
		"Insert": 0x5200, "Ins": 0x5200,
		"Delete": 0x5300, "Del": 0x5300,
	}
	for name, word := range want {
		key, ok := KeyNamed(name)
		if !ok {
			t.Errorf("不認得 %s", name)
			continue
		}
		if key.Word() != word {
			t.Errorf("%s 的字組是 %04X，該是 %04X", name, key.Word(), word)
		}
	}
	// 位置名與功能名指的是同一顆鍵。
	for _, pair := range [][2]string{{"Home", "KP7"}, {"End", "KP1"}, {"PgUp", "KP9"}, {"PgDn", "KP3"}} {
		a, _ := KeyNamed(pair[0])
		b, _ := KeyNamed(pair[1])
		if a.Word() != b.Word() {
			t.Errorf("%s 與 %s 的字組不同（%04X／%04X）", pair[0], pair[1], a.Word(), b.Word())
		}
	}
}
