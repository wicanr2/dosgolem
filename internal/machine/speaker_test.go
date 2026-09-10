package machine

import "testing"

// TestPITChannel0ChangesTheInterruptRate 釘住分頻值 → IRQ0 間隔。
//
// 少了這一層，程式把分頻值設成 100（約 11.9 kHz）之後中斷仍然按開機的
// 18.2 Hz 來——**而且不會報錯**，只是語音一秒要跑上億道指令。
//
// 基準值一律問 `PITStepsPerTick`，不要寫死：`DefaultIRQ0Every` 標定在
// 17,000 分頻上，不是開機的 65,536（`docs/spec/190`）。
func TestPITChannel0ChangesTheInterruptRate(t *testing.T) {
	m := New()
	boot := PITStepsPerTick(PITDefaultDivisor)
	if m.IRQ0Every != boot {
		t.Fatalf("開機的 IRQ0Every 是 %d，應該是 %d", m.IRQ0Every, boot)
	}
	// 控制字 0x36：通道 0、先低後高、模式 3、二進位。
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 100&0xFF)
	if m.IRQ0Every != boot {
		t.Errorf("只寫了低位就改了間隔（%d）——低後高模式要收滿兩個位元組", m.IRQ0Every)
	}
	m.Out8(0x40, 100>>8)
	want := PITStepsPerTick(100)
	if m.IRQ0Every != want {
		t.Errorf("分頻值 100 的 IRQ0Every 是 %d，應該是 %d", m.IRQ0Every, want)
	}
	if d := m.PITDivisor(); d != 100 {
		t.Errorf("分頻值讀回來是 %d", d)
	}
	if hz := m.PITHz(); hz < 11900 || hz > 12000 {
		t.Errorf("分頻值 100 換算是 %.0f Hz，應該在 11.9 kHz 附近", hz)
	}
	// 寫回 0（＝65536）要還原成 18.2 Hz。
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 0)
	m.Out8(0x40, 0)
	if m.IRQ0Every != boot {
		t.Errorf("分頻值 0 沒有還原成 %d（現在 %d）", boot, m.IRQ0Every)
	}
}

// TestPITOtherChannelsDoNotChangeIRQ0 是負對照。
//
// 通道 2 是喇叭的方波產生器，嗶聲會寫它；把它也拿來改中斷頻率的話，
// 一次嗶聲就會把整台機器的時鐘打亂。
func TestPITOtherChannelsDoNotChangeIRQ0(t *testing.T) {
	m := New()
	m.Out8(0x43, 0xB6) // bit7-6 ＝ 10 ＝ 通道 2
	m.Out8(0x42, 0x34)
	m.Out8(0x42, 0x12)
	if boot := PITStepsPerTick(PITDefaultDivisor); m.IRQ0Every != boot {
		t.Errorf("通道 2 的設定改到了 IRQ0Every（%d，開機值是 %d）", m.IRQ0Every, boot)
	}
}

// TestTinyDivisorIsClampedAndCounted 釘住下限與計數。
//
// 分頻值 1 照比例算是每 2 道指令一次中斷，處理常式自己跑不完。
// **夾住要記一次**——安靜地夾等於把「時間軸不可信」藏起來。
func TestTinyDivisorIsClampedAndCounted(t *testing.T) {
	m := New()
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 1)
	m.Out8(0x40, 0)
	if m.IRQ0Every != MinIRQ0Every {
		t.Errorf("分頻值 1 的間隔是 %d，應該夾到 %d", m.IRQ0Every, MinIRQ0Every)
	}
	if m.IRQ0Clamped != 1 {
		t.Errorf("夾了 %d 次，應該是 1", m.IRQ0Clamped)
	}
}

// TestSpeakerRecordsOnlyChanges 釘住喇叭波形的記錄方式。
func TestSpeakerRecordsOnlyChanges(t *testing.T) {
	m := New()
	m.Out8(0x61, 0x00) // 重置值，不記
	if len(m.Speaker) != 0 {
		t.Fatalf("開頭的靜音被記了 %d 筆", len(m.Speaker))
	}
	m.Out8(0x61, 0x02) // bit1 ＝ 1
	m.Out8(0x61, 0x02) // 同一個值，不記
	m.Out8(0x61, 0x00)
	m.Out8(0x61, 0x03) // bit0 也起來 ＝ 閘門打開（嗶聲那條路）
	got := m.Speaker
	if len(got) != 3 {
		t.Fatalf("記了 %d 筆，應該是 3：%+v", len(got), got)
	}
	if got[0].Level != 1 || got[0].Gate != 0 {
		t.Errorf("第一筆是 %+v", got[0])
	}
	if got[1].Level != 0 {
		t.Errorf("第二筆是 %+v", got[1])
	}
	if got[2].Level != 1 || got[2].Gate != 1 {
		t.Errorf("第三筆是 %+v（閘門要分得出來，否則嗶聲會被當成語音）", got[2])
	}
}

// TestSnapshotRestoresSpeakerAndPIT 釘住快照。
//
// 分頻值不還原的話，還原之後的中斷頻率是**上一次跑到最後**的那個值，
// 而那不會報錯——只會讓重跑的那一段動畫快慢不同。
func TestSnapshotRestoresSpeakerAndPIT(t *testing.T) {
	m := New()
	snap := m.Snapshot()
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 200)
	m.Out8(0x40, 0)
	m.Out8(0x61, 0x02)
	boot := PITStepsPerTick(PITDefaultDivisor)
	if m.IRQ0Every == boot || len(m.Speaker) == 0 {
		t.Fatal("前置動作沒生效")
	}
	m.Restore(snap)
	if m.IRQ0Every != boot {
		t.Errorf("還原之後 IRQ0Every 是 %d，開機值是 %d", m.IRQ0Every, boot)
	}
	// 「回到未設定」要問 PITProgrammed：PITDivisor 在沒被設過時回的是
	// BIOS 的預設 65,536，不是 0——拿 0 當判準會永遠不成立。
	if m.PITProgrammed() {
		t.Errorf("還原之後仍回報被程式設定過（分頻值 %d）", m.PITDivisor())
	}
	if len(m.Speaker) != 0 {
		t.Errorf("還原之後喇叭序列還有 %d 筆", len(m.Speaker))
	}
}

// TestPICMaskReadsBack 釘住埠 0x21 讀得回自己寫進去的值。
//
// 「讀 → or 1 → 寫回」是遮蔽 IRQ0 的標準寫法。讀回預設的 0xFF 之後
// 寫回去就把**全部**中斷關掉了，包括鍵盤——之後按什麼都沒反應，
// 而畫面照樣在動，看起來像遊戲卡住不像中斷被關。
func TestPICMaskReadsBack(t *testing.T) {
	m := New()
	if got := m.In8(0x21); got != 0 {
		t.Fatalf("開機的中斷遮罩讀回 %02X，應該是 00（全部放行）", got)
	}
	v := m.In8(0x21) | 0x01
	m.Out8(0x21, v)
	if got := m.In8(0x21); got != 0x01 {
		t.Errorf("遮蔽 IRQ0 之後讀回 %02X，應該是 01", got)
	}
	m.Out8(0x21, m.In8(0x21)&0xFE)
	if got := m.In8(0x21); got != 0x00 {
		t.Errorf("放行之後讀回 %02X", got)
	}
}

// TestMaskedIRQ0IsHeldNotDropped 釘住遮蔽期間中斷是掛起不是丟掉。
func TestMaskedIRQ0IsHeldNotDropped(t *testing.T) {
	m := New()
	m.Out8(0x21, 0x01) // 遮蔽 IRQ0
	m.CPU.SetFlags(m.CPU.Flags | 0x0200)
	m.Steps = m.IRQ0Every + 1
	m.tick()
	if m.Ticks != 0 {
		t.Fatalf("遮蔽期間送了 %d 次中斷", m.Ticks)
	}
	m.Out8(0x21, 0x00) // 放行
	m.tick()
	if m.Ticks != 1 {
		t.Errorf("放行之後補送了 %d 次，應該是 1——掛起的中斷不能丟掉", m.Ticks)
	}
}
