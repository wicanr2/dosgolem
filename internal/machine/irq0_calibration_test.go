package machine

import "testing"

// 計時器間隔的標定（`docs/spec/190`）。
//
// 指令數時鐘與 PITStepsPerTick 描述的是同一台機器，任何分頻下兩者要
// 算出同一個間隔。不一致的話症狀不會是錯誤——順序、因果、畫面內容都
// 對，只有時間軸整體縮放，而那要拿原版對拍才看得出來。

// TestIRQ0IntervalMatchesPITStepsPerTick 是規格 §6 的第 1 條。
func TestIRQ0IntervalMatchesPITStepsPerTick(t *testing.T) {
	for _, div := range []uint32{17000, 4096, 65536, 1000, 256, 1} {
		m := New()
		writeDivisor(t, m, div)
		// 分頻小到某個程度時間隔會被下限夾住（處理常式自己跑不完，
		// 機器會卡死在中斷裡），夾住這件事本身另有測試釘。
		want := PITStepsPerTick(div)
		if want < MinIRQ0Every {
			want = MinIRQ0Every
		}
		if m.IRQ0Every != want {
			t.Errorf("分頻 %d：IRQ0Every ＝ %d，預期 %d", div, m.IRQ0Every, want)
		}
	}
}

// TestIRQ0IntervalAtKnownDivisors 是規格 §6 的第 2 條：釘住三個實際
// 遇得到的分頻。
//
// 17,000 是《大富翁2》寫的，而 `DefaultIRQ0Every` 就是在那個分頻下量
// 出來的（`docs/spec/190` §2）——所以它要回自己。
func TestIRQ0IntervalAtKnownDivisors(t *testing.T) {
	for _, c := range []struct {
		name string
		div  uint32
		want uint64
	}{
		{"大富翁2（標定的那一個）", 17000, DefaultIRQ0Every},
		{"臥龍傳的 YNSOUND.COM", 4096, 39755},
		{"BIOS 預設", 65536, 636085},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := New()
			writeDivisor(t, m, c.div)
			if m.IRQ0Every != c.want {
				t.Errorf("分頻 %d：IRQ0Every ＝ %d，預期 %d", c.div, m.IRQ0Every, c.want)
			}
		})
	}
}

// TestBootIntervalIsBIOSRate：開機還沒被程式設過時走 BIOS 的 18.2 Hz。
//
// **開機值不是 DefaultIRQ0Every**：那個常數標定在 17,000 分頻上，
// 而開機的分頻是 65,536。兩者差 3.855 倍。
func TestBootIntervalIsBIOSRate(t *testing.T) {
	m := New()
	if want := PITStepsPerTick(PITDefaultDivisor); m.IRQ0Every != want {
		t.Errorf("開機 IRQ0Every ＝ %d，18.2 Hz 下應該是 %d", m.IRQ0Every, want)
	}
}

// writeDivisor 照程式實際的寫法設分頻：控制字 0x36 再送兩個位元組。
func writeDivisor(t *testing.T, m *Machine, div uint32) {
	t.Helper()
	v := div
	if v == PITDefaultDivisor {
		v = 0 // 8254：寫 0 代表 65,536
	}
	m.Out8(0x43, 0x36)
	m.Out8(0x40, uint8(v))
	m.Out8(0x40, uint8(v>>8))
}
