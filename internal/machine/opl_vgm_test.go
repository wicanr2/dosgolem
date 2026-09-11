package machine

import (
	"bytes"
	"math"
	"testing"
)

// TestStepsPerSecondNowMatchesTheConstantWhenUnpinned 沒釘死時，
// 新的換算與既有的常數要對得上，而且改分頻不影響它。
//
// 這一條在證明「這不是另立一套模型」：`IRQ0Every` 跟著分頻走的時候
// d 會約掉，乘積就是 StepsPerSecond()。對不上的話表示兩套標定打架，
// 而打架的結果是喇叭與 FM 兩條路的時間軸不一致。
//
// ⚠ **容許 0.1% 而不是逐位元相等**：`recalcIRQ0` 算 `IRQ0Every` 是整數
// 除法（分頻 4096 時 `165000×4096/65536 = 10312.5` 被截成 10312），
// 截掉的那半道指令就是這裡的差。相對誤差約 `1/IRQ0Every`，分頻愈小
// 愈明顯。**這是量化不是分歧**——真的分歧會差好幾倍，不是萬分之五。
func TestStepsPerSecondNowMatchesTheConstantWhenUnpinned(t *testing.T) {
	m := New()
	for _, div := range []struct {
		lo, hi uint8
		want   uint32
	}{
		{0x00, 0x00, 65536}, // 開機值
		{0x00, 0x10, 4096},  // FMDRV.COM
		{0x00, 0x08, 2048},  // logh3 的開頭動畫
	} {
		m.Out8(0x43, 0x36)
		m.Out8(0x40, div.lo)
		m.Out8(0x40, div.hi)
		if m.PITDiv != div.want {
			t.Fatalf("分頻 %d，寫進去的是 %d", m.PITDiv, div.want)
		}
		got := m.StepsPerSecondNow()
		if math.Abs(got-StepsPerSecond())/StepsPerSecond() > 1e-3 {
			t.Errorf("分頻 %d：StepsPerSecondNow() ＝ %.0f，StepsPerSecond() ＝ %.0f",
				div.want, got, StepsPerSecond())
		}
	}
}

// TestStepsPerSecondNowFollowsThePinnedTick 釘死間隔之後要跟著呼叫端走。
//
// ⚠ **這是整支的重點。** logh3 的對拍腳本用 `-tick 20000` 釘死間隔，
// 而遊戲把分頻設成 2048（582.6 Hz），所以機器實際上是
// `20,000 × 582.6 ≈ 11.65 M 指令／秒`，不是常數那個 3.00 M。
// 用錯的那個去換算時間，**曲子照樣播得出來、只是慢了將近四倍**，
// 而且沒有任何一個環節會報錯——所以錯誤要在這一行擋住，
// 不能指望事後聽出來。
func TestStepsPerSecondNowFollowsThePinnedTick(t *testing.T) {
	m := New()
	m.IRQ0Every, m.IRQ0Pinned = 20000, true
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 0x00)
	m.Out8(0x40, 0x08) // 分頻 2048
	if m.IRQ0Every != 20000 {
		t.Fatalf("釘死之後間隔被改成 %d", m.IRQ0Every)
	}

	want := 20000 * m.PITHz()
	got := m.StepsPerSecondNow()
	if math.Abs(got-want)/want > 1e-9 {
		t.Fatalf("StepsPerSecondNow() ＝ %.0f，應該是 IRQ0Every × PITHz ＝ %.0f", got, want)
	}
	// 與常數差多少要講得出來——差在三倍以上，不是捨入誤差。
	if r := got / StepsPerSecond(); r < 3.5 || r > 4.0 {
		t.Errorf("與 StepsPerSecond() 差 %.2f 倍，logh3 那一組應該落在 3.9 倍附近", r)
	}
}

// TestStepsPerSecondNowFallsBackWhenTimerOff 計時器關著時退回常數。
func TestStepsPerSecondNowFallsBackWhenTimerOff(t *testing.T) {
	m := New()
	m.IRQ0Every = 0
	if got := m.StepsPerSecondNow(); got != StepsPerSecond() {
		t.Errorf("計時器關著時 %.0f，應該退回 %.0f", got, StepsPerSecond())
	}
}

// TestOPLVGMUsesTheMachineTimeBase 不給時間基準時，倒出來的秒數
// 要照機器現在的設定，不是照常數。
func TestOPLVGMUsesTheMachineTimeBase(t *testing.T) {
	m := New()
	m.SetAdLib(true)
	m.IRQ0Every, m.IRQ0Pinned = 20000, true
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 0x00)
	m.Out8(0x40, 0x08) // 分頻 2048 → 約 11.65 M 指令／秒

	oplOut(m, 0x20, 0x01)
	// 正好一秒。**期望值不能用受測的那支函式算**——那樣它回什麼都自洽，
	// 負對照會綠。這裡用與它無關的 `PITHz()` 自己乘出來。
	m.Steps += uint64(20000 * m.PITHz())
	oplOut(m, 0x40, 0x10)

	var b bytes.Buffer
	st, err := m.OPLVGM(&b, 0)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(st.Seconds-1) > 0.01 {
		t.Errorf("兩筆相隔一秒，倒出來是 %.3f 秒", st.Seconds)
	}
	if st.Writes != 2 || st.Chip != "YM3812" {
		t.Errorf("Stats = %+v", st)
	}
	if string(b.Bytes()[:4]) != "Vgm " {
		t.Errorf("不是 VGM：% 02X", b.Bytes()[:4])
	}
}

// TestOPLVGMEmptyIsAnError AdLib 沒開的時候序列是空的，要講出來。
//
// 空序列與「這個程式沒有音樂」長得一模一樣。
func TestOPLVGMEmptyIsAnError(t *testing.T) {
	m := New()
	var b bytes.Buffer
	if _, err := m.OPLVGM(&b, 0); err == nil {
		t.Fatal("一筆都沒有應該回錯誤")
	}
}
