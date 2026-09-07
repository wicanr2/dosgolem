package machine

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 觀測工具的每一條規則，錯了都不會報錯：監看點沒觸發看起來像「那件事沒發生」，
// 中斷點停錯地方看起來像「程式走了別條路」。所以逐條釘。

// tinyProgram 把一小段機器碼載到 PSPSeg:0100h 並把 CPU 設好。
func tinyProgram(t *testing.T, code ...byte) *Machine {
	t.Helper()
	m := New()
	if err := m.LoadCOM(code); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestWatchWriteSeesWhoWroteIt(t *testing.T) {
	// mov word ptr [0200h], 1234h  然後  int 20h
	m := tinyProgram(t, 0xC7, 0x06, 0x00, 0x02, 0x34, 0x12, 0xCD, 0x20)
	target := uint32(PSPSeg)*16 + 0x200

	var hits int
	var atIP uint16
	m.WatchWrite(target, target+1, func(mm *Machine, addr uint32, old, new uint8) {
		hits++
		_, atIP = mm.Insn()
	})
	if err := m.Step(); err != nil {
		t.Fatal(err)
	}
	// 一次 16 位元存放在這台機器上是兩次位元組寫入。
	if hits != 2 {
		t.Fatalf("觸發了 %d 次，該是 2", hits)
	}
	// 觸發時 CPU.IP 已經被解碼推過去了，所以「誰寫的」要問 Insn。
	// 這一條釘的就是那個差別：問錯的話拿到的是下一條指令的位址。
	if atIP != 0x100 {
		t.Errorf("Insn 回 %04X，該是這條指令的起點 0100", atIP)
	}
	if _, ip := m.Insn(); ip != 0x100 {
		t.Errorf("指令做完之後 Insn 回 %04X，該還是 0100", ip)
	}
	if got := m.Read16(target); got != 0x1234 {
		t.Errorf("寫進去的是 %04X", got)
	}
}

func TestWatchWriteFiresEvenWhenTheValueIsTheSame(t *testing.T) {
	// 「誰在寫這個位址」與「這個值變了沒」是兩個問題。
	m := New()
	m.Write8(0x500, 7)
	hits := 0
	m.WatchWrite(0x500, 0x500, func(*Machine, uint32, uint8, uint8) { hits++ })
	m.Write8(0x500, 7)
	if hits != 1 {
		t.Fatalf("寫入相同的值觸發了 %d 次，該是 1", hits)
	}
}

func TestWatchWordOnlyFiresOnRealChange(t *testing.T) {
	// mov word ptr [0200h],1234h / mov word ptr [0200h],1234h / mov …,5678h
	m := tinyProgram(t,
		0xC7, 0x06, 0x00, 0x02, 0x34, 0x12,
		0xC7, 0x06, 0x00, 0x02, 0x34, 0x12,
		0xC7, 0x06, 0x00, 0x02, 0x78, 0x56,
		0xCD, 0x20)
	target := uint32(PSPSeg)*16 + 0x200

	var got [][2]uint16
	m.WatchWord(target, func(_ *Machine, _ uint32, old, new uint16) {
		got = append(got, [2]uint16{old, new})
	})
	for i := 0; i < 3; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	// 三條指令：變了、沒變、又變了。中間那條不該通知。
	// 而且**不能**在半個 word 寫好的當下通知——那不是任何程式看得到的狀態。
	if len(got) != 2 {
		t.Fatalf("通知了 %d 次：%v", len(got), got)
	}
	if got[0] != [2]uint16{0, 0x1234} || got[1] != [2]uint16{0x1234, 0x5678} {
		t.Errorf("通知的內容是 %v", got)
	}
}

func TestUnwatchStopsIt(t *testing.T) {
	m := New()
	hits := 0
	id := m.WatchWrite(0x500, 0x500, func(*Machine, uint32, uint8, uint8) { hits++ })
	m.Unwatch(id)
	m.Write8(0x500, 1)
	if hits != 0 {
		t.Fatalf("拿掉之後還觸發了 %d 次", hits)
	}
}

func TestRunUntilStopsBeforeTheBreakpoint(t *testing.T) {
	// nop / nop / nop / int 20h
	m := tinyProgram(t, 0x90, 0x90, 0x90, 0xCD, 0x20)
	m.BreakAt(PSPSeg, 0x102)

	why, err := m.RunUntil(nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if why != StopBreakpoint {
		t.Fatalf("停下來的原因是 %v", why)
	}
	// **還沒執行那一條**——停在後面的話就沒辦法看它執行前的狀態。
	if m.CPU.IP != 0x102 {
		t.Errorf("停在 %04X，該是 0102", m.CPU.IP)
	}

	// 再叫一次要走得動，不能當場又停在同一個點上。
	if _, err := m.RunUntil(nil, 1); err != nil {
		t.Fatal(err)
	}
	if m.CPU.IP == 0x102 {
		t.Error("再叫一次還停在原地——中斷點會把程式卡死")
	}
}

func TestRunUntilReportsWhyItStopped(t *testing.T) {
	m := tinyProgram(t, 0x90, 0x90, 0x90, 0x90, 0xCD, 0x20)

	why, err := m.RunUntil(func(mm *Machine) bool { return mm.CPU.IP == 0x103 }, 100)
	if err != nil {
		t.Fatal(err)
	}
	if why != StopPredicate || m.CPU.IP != 0x103 {
		t.Fatalf("停在 %04X，原因 %v", m.CPU.IP, why)
	}

	// 預算用完要說「預算用完」，不能假裝條件成立過。
	why, _ = m.RunUntil(func(*Machine) bool { return false }, 3)
	if why != StopBudget {
		t.Fatalf("預算用完卻回 %v", why)
	}
}

func TestProbesCostNothingWhenUnused(t *testing.T) {
	// 熱路徑只多一次長度檢查——這一條釘的是「沒設監看點時行為完全不變」。
	m := tinyProgram(t, 0x90, 0xCD, 0x20)
	before := m.CPU.Seg[cpu.CS]
	if err := m.Step(); err != nil {
		t.Fatal(err)
	}
	if m.CPU.Seg[cpu.CS] != before || m.CPU.IP != 0x101 {
		t.Errorf("沒設監看點卻走到 %04X:%04X", m.CPU.Seg[cpu.CS], m.CPU.IP)
	}
}

func TestWatchWriteHonoursTheRangeEdges(t *testing.T) {
	// 範圍差一格的症狀是「那個位址的寫入看不到」，而看不到不會報錯。
	m := New()
	var hit []uint32
	m.WatchWrite(0x500, 0x502, func(_ *Machine, a uint32, _, _ uint8) { hit = append(hit, a) })
	for _, a := range []uint32{0x4FF, 0x500, 0x501, 0x502, 0x503} {
		m.Write8(a, 1)
	}
	want := []uint32{0x500, 0x501, 0x502}
	if len(hit) != len(want) {
		t.Fatalf("觸發在 %v", hit)
	}
	for i := range want {
		if hit[i] != want[i] {
			t.Fatalf("觸發在 %v，該是 %v", hit, want)
		}
	}
}

func TestWatchWriteSeesHostWritesToo(t *testing.T) {
	// 載入器與測試自己擺的資料也走 Write8／Write16／WriteBytes。
	// 只掛 CPU 那條路的話，「這塊資料是誰放的」會少一半答案。
	m := New()
	hits := 0
	m.WatchWrite(0x600, 0x603, func(*Machine, uint32, uint8, uint8) { hits++ })
	m.Write16(0x600, 0x1234)
	m.WriteBytes(0x602, []byte{1, 2})
	if hits != 4 {
		t.Fatalf("觸發了 %d 次，該是 4", hits)
	}
}

func TestBreakpointBeatsThePredicate(t *testing.T) {
	// 兩個同時成立時要回中斷點——回條件成立的話呼叫端會以為沒撞到中斷點，
	// 然後在下一次 RunUntil 立刻撞上，位置卻對不起來。
	m := tinyProgram(t, 0x90, 0x90, 0x90, 0xCD, 0x20)
	m.BreakAt(PSPSeg, 0x101)
	why, err := m.RunUntil(func(mm *Machine) bool { return mm.CPU.IP == 0x101 }, 100)
	if err != nil {
		t.Fatal(err)
	}
	if why != StopBreakpoint {
		t.Fatalf("回的是 %v", why)
	}
}

func TestUnwatchIgnoresUnknownID(t *testing.T) {
	m := New()
	m.Unwatch(0)
	m.Unwatch(9999)
	m.ClearBreak(9999)
}

func TestWatchWordSurvivesUnwatchOfAnother(t *testing.T) {
	// 兩種監看點共用同一組 id。拿掉一個卻順手殺掉另一個，
	// 症狀是「某個監看點莫名其妙不觸發了」。
	m := New()
	wordHits, writeHits := 0, 0
	id := m.WatchWrite(0x700, 0x701, func(*Machine, uint32, uint8, uint8) { writeHits++ })
	m.WatchWord(0x700, func(*Machine, uint32, uint16, uint16) { wordHits++ })
	m.Unwatch(id)

	m.Write16(0x700, 0x1234)
	m.pollWords()
	if writeHits != 0 {
		t.Errorf("拿掉的那個還觸發了 %d 次", writeHits)
	}
	if wordHits != 1 {
		t.Errorf("另一個觸發了 %d 次，該是 1", wordHits)
	}
}
