package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 拿這台機器當對照的前提是兩件事：同一份輸入永遠得到同一個結果，
// 以及快照真的是**完整**的狀態。
//
// 這兩條壞掉都不會報錯。前者會讓對拍時好時壞，看起來像被觀測的程式有問題；
// 後者會讓「從同一個狀態展開多個變體」的第二個變體開始就悄悄走偏。

// spinner 是一段會一直跑的碼，而且會動到輪詢埠、記憶體與計時器：
//
//	0100: E4 40         in  al, 40h      ; PIT，回傳值會變
//	0102: 00 06 00 02   add [0200], al
//	0106: EB F8         jmp 0100
//
// 只用 nop 迴圈測不出東西——那不會碰到任何隨時間變化的狀態。
func spinner(t *testing.T) *Machine {
	t.Helper()
	m := New()
	if err := m.LoadCOM([]byte{0xE4, 0x40, 0x00, 0x06, 0x00, 0x02, 0xEB, 0xF8}); err != nil {
		t.Fatal(err)
	}
	// 中斷要開著，否則計時器一次都不會送，這幾個測試就只驗到了記憶體。
	m.CPU.SetFlags(cpu.IF)
	return m
}

// fingerprint 是一台機器完整狀態的指紋。
func fingerprint(m *Machine) (regs [16]uint16, counters [3]uint64, mem []byte) {
	copy(regs[:8], m.CPU.R[:])
	copy(regs[8:12], m.CPU.Seg[:])
	regs[12], regs[13] = m.CPU.IP, m.CPU.Flags
	counters = [3]uint64{m.Steps, m.Ticks, m.portTicks}
	return regs, counters, m.Mem
}

func mustRun(t *testing.T, m *Machine, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := m.Step(); err != nil {
			t.Fatalf("第 %d 條：%v", i, err)
		}
	}
}

func TestSameInputGivesTheSameRun(t *testing.T) {
	a, b := spinner(t), spinner(t)
	mustRun(t, a, 200_000)
	mustRun(t, b, 200_000)

	ra, ca, ma := fingerprint(a)
	rb, cb, mb := fingerprint(b)
	if ra != rb {
		t.Errorf("暫存器不同：\n  %v\n  %v", ra, rb)
	}
	if ca != cb {
		t.Errorf("計數器不同：%v vs %v", ca, cb)
	}
	if !bytes.Equal(ma, mb) {
		t.Error("記憶體不同")
	}
	// 這段碼會一直碰輪詢埠與計時器；跑了二十萬條卻什麼都沒動的話，
	// 這個測試等於什麼都沒驗。
	if ca[1] == 0 || ca[2] == 0 {
		t.Fatalf("計時器 %d 次、輪詢埠 %d 次——這段碼沒有動到會變的狀態", ca[1], ca[2])
	}
}

func TestRestoreGivesTheSameFutureAsNotSavingAtAll(t *testing.T) {
	// 快照漏一個欄位的症狀：**還原之後那一份跑出不一樣的未來**，
	// 而且第一份（沒還原過的）完全正常。
	// 實際踩過的是漏了計時器的三個時鐘——還原之後計時器中斷從此不再送。
	m := spinner(t)
	mustRun(t, m, 50_000)
	snap := m.Snapshot()

	mustRun(t, m, 50_000)
	wantRegs, wantCounters, wantMem := fingerprint(m)
	golden := append([]byte(nil), wantMem...)

	m.Restore(snap)
	mustRun(t, m, 50_000)
	gotRegs, gotCounters, gotMem := fingerprint(m)

	if gotRegs != wantRegs {
		t.Errorf("還原之後暫存器不同：\n  想要 %v\n  得到 %v", wantRegs, gotRegs)
	}
	if gotCounters != wantCounters {
		t.Errorf("還原之後計數器不同：想要 %v，得到 %v", wantCounters, gotCounters)
	}
	if !bytes.Equal(gotMem, golden) {
		t.Error("還原之後記憶體不同")
	}
}

func TestRestoreCanBeReplayedManyTimes(t *testing.T) {
	// 「從同一個狀態展開多個變體」是這台機器的用途之一。
	// 只還原一次會過、還原三次才壞，是很典型的漏欄位症狀。
	m := spinner(t)
	mustRun(t, m, 20_000)
	snap := m.Snapshot()

	var first [3]uint64
	for i := 0; i < 4; i++ {
		m.Restore(snap)
		mustRun(t, m, 20_000)
		_, counters, _ := fingerprint(m)
		if i == 0 {
			first = counters
			continue
		}
		if counters != first {
			t.Fatalf("第 %d 次還原之後計數器是 %v，第一次是 %v", i+1, counters, first)
		}
	}
}

func TestSnapshotKeepsItsOwnCopyOfMemory(t *testing.T) {
	// 快照與機器共用同一塊記憶體的話，拍完之後繼續跑會把快照也改掉——
	// 而還原會「成功」，只是還原到一個從來不存在的狀態。
	m := spinner(t)
	mustRun(t, m, 1000)
	snap := m.Snapshot()
	before := append([]byte(nil), snap.Mem()[:0x1000]...)

	mustRun(t, m, 50_000)
	if !bytes.Equal(before, snap.Mem()[:0x1000]) {
		t.Fatal("繼續跑之後快照裡的記憶體被改掉了")
	}
}

func TestInterruptsStayDeterministic(t *testing.T) {
	// 計時器中斷是靠指令數送的，不是牆上的時鐘——所以次數必須可重現。
	// 這一條壞掉的症狀是「同一段輸入有時多跑一次中斷處理」。
	a, b := spinner(t), spinner(t)
	for _, m := range []*Machine{a, b} {
		mustRun(t, m, 500_000)
	}
	if a.Ticks != b.Ticks {
		t.Fatalf("計時器中斷次數 %d vs %d", a.Ticks, b.Ticks)
	}
	if a.Ticks < 2 {
		t.Fatalf("只送了 %d 次計時器中斷，測不出東西", a.Ticks)
	}
}
