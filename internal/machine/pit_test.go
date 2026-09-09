package machine

import "testing"

// TestPITDivisorScalesIRQ0 程式改 PIT 的分頻，IRQ0 的間隔要跟著按比例走。
//
// FMDRV.COM 的安裝序列就是這個形狀：`mov al,36h; out 43h`（通道 0、
// 先低後高）之後 `out 40h,0` ＋ `out 40h,10h` ＝ 分頻 4096。
func TestPITDivisorScalesIRQ0(t *testing.T) {
	m := New()
	if m.PITDiv != PITDefaultDivisor || m.IRQ0Every != DefaultIRQ0Every {
		t.Fatalf("開機是分頻 %d、間隔 %d，量到 %d／%d",
			PITDefaultDivisor, DefaultIRQ0Every, m.PITDiv, m.IRQ0Every)
	}
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 0x00)
	m.Out8(0x40, 0x10)
	if m.PITDiv != 4096 {
		t.Errorf("分頻 %d，寫進去的是 4096", m.PITDiv)
	}
	if want := uint64(DefaultIRQ0Every) * 4096 / 65536; m.IRQ0Every != want {
		t.Errorf("間隔 %d，照比例應該是 %d", m.IRQ0Every, want)
	}
	// 寫 0 ＝ 65536，要回到開機頻率
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 0x00)
	m.Out8(0x40, 0x00)
	if m.PITDiv != PITDefaultDivisor || m.IRQ0Every != DefaultIRQ0Every {
		t.Errorf("寫 0 之後是分頻 %d、間隔 %d", m.PITDiv, m.IRQ0Every)
	}
}

// TestPITAccessModes 只寫低位元組／只寫高位元組也要認。
func TestPITAccessModes(t *testing.T) {
	m := New()
	m.Out8(0x43, 0x10) // 通道 0、只寫低
	m.Out8(0x40, 0x80)
	if m.PITDiv != 0x80 {
		t.Errorf("只寫低：分頻 %d，應該是 %d", m.PITDiv, 0x80)
	}
	m.Out8(0x43, 0x20) // 通道 0、只寫高
	m.Out8(0x40, 0x40)
	if m.PITDiv != 0x4000 {
		t.Errorf("只寫高：分頻 %d，應該是 %d", m.PITDiv, 0x4000)
	}
	// 別的通道不影響 IRQ0。**通道由資料埠決定**（40h／41h／42h），
	// 不是由控制位元組決定——喇叭的分頻走 42h，這裡不該被它動到。
	before := m.IRQ0Every
	m.Out8(0x43, 0xB6) // 通道 2（喇叭）
	m.Out8(0x42, 0x00)
	m.Out8(0x42, 0x01)
	if m.IRQ0Every != before {
		t.Errorf("通道 2 的分頻不該動到 IRQ0（%d → %d）", before, m.IRQ0Every)
	}
	// 而且它也不該弄壞通道 0 的「先低後高」狀態機
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 0x00)
	m.Out8(0x40, 0x10)
	if m.PITDiv != 4096 {
		t.Errorf("通道 2 之後再設通道 0：分頻 %d，應該是 4096", m.PITDiv)
	}
}

// TestPITSurvivesSnapshot 快照與還原要把分頻帶著走。
func TestPITSurvivesSnapshot(t *testing.T) {
	m := New()
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 0x00)
	m.Out8(0x40, 0x10)
	snap := m.Snapshot()
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 0x00)
	m.Out8(0x40, 0x00) // 改回 65536
	m.Restore(snap)
	if m.PITDiv != 4096 {
		t.Errorf("還原之後分頻是 %d，快照當時是 4096", m.PITDiv)
	}
	if want := uint64(DefaultIRQ0Every) * 4096 / 65536; m.IRQ0Every != want {
		t.Errorf("還原之後間隔是 %d，應該是 %d", m.IRQ0Every, want)
	}
}
