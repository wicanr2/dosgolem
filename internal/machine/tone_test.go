package machine

import "testing"

// `docs/spec/195`：PIT 通道 2 方波的事件與合成。

// playNote 以 BIOS 慣用的寫法發一個音：43h ← B6h、42h 低高、61h ← 03h。
func playNote(m *Machine, step uint64, div uint16) {
	m.Steps = step
	m.Out8(0x43, 0xB6)
	m.Out8(0x42, uint8(div))
	m.Out8(0x42, uint8(div>>8))
	m.Out8(0x61, 0x03)
}

func TestToneEventsFrequencyAndSilence(t *testing.T) {
	m := New()
	sps := uint64(StepsPerSecond())
	playNote(m, sps, 4830) // 247 Hz
	m.Steps = 2 * sps
	m.Out8(0x61, 0x00)
	ev := m.ToneEvents()
	if len(ev) != 2 {
		t.Fatalf("事件 %d 筆，要 2 筆：%v", len(ev), ev)
	}
	if ev[0].Step != sps || ev[0].Hz < 246.5 || ev[0].Hz > 247.5 {
		t.Errorf("第一筆 %+v，要第 %d 步 247 Hz", ev[0], sps)
	}
	if ev[1].Step != 2*sps || ev[1].Hz != 0 {
		t.Errorf("第二筆 %+v，要第 %d 步靜音", ev[1], 2*sps)
	}
}

// TestToneNeedsBothGateAndData：只有閘門或只有資料致能都不發聲；分頻值 0 ＝ 65536。
func TestToneNeedsBothGateAndData(t *testing.T) {
	for _, v := range []uint8{0x01, 0x02} {
		m := New()
		m.Out8(0x43, 0xB6)
		m.Out8(0x42, 0x00)
		m.Out8(0x42, 0x10)
		m.Out8(0x61, v)
		for _, e := range m.ToneEvents() {
			if e.Hz != 0 {
				t.Errorf("61h ＝ %02X 不該發聲，拿到 %+v", v, e)
			}
		}
	}
	m := New()
	playNote(m, 0, 0)
	ev := m.ToneEvents()
	if len(ev) == 0 || ev[len(ev)-1].Hz < 18.1 || ev[len(ev)-1].Hz > 18.3 {
		t.Errorf("分頻值 0 應是 65536（約 18.2 Hz），拿到 %v", ev)
	}
}

// TestTonePCMPitchLengthAndSilence：長度照時間軸換算、發聲段過零次數對應頻率、靜音段全為 80h。
func TestTonePCMPitchLengthAndSilence(t *testing.T) {
	m := New()
	sps := StepsPerSecond()
	playNote(m, 0, 2415) // 494 Hz，響 1 秒
	m.Steps = uint64(sps)
	m.Out8(0x61, 0x00) // 靜 0.5 秒
	m.Steps = uint64(1.5 * sps)
	playNote(m, uint64(1.5*sps), 4830) // 再響到 2 秒
	m.Steps = uint64(2 * sps)
	m.Out8(0x61, 0x00)
	const rate = 22050
	pcm, err := m.TonePCM(rate)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(pcm); n < 2*rate-2 || n > 2*rate+2 {
		t.Fatalf("2 秒算出 %d 個取樣，要 %d", n, 2*rate)
	}
	// 第一秒：數上升邊（20h → E0h），494 Hz ±2%
	rises := 0
	for i := 1; i < rate; i++ {
		if pcm[i-1] == 0x20 && pcm[i] == 0xE0 {
			rises++
		}
	}
	if rises < 484 || rises > 504 {
		t.Errorf("第一秒 %d 個週期，要 494 ±2%%", rises)
	}
	// 1.1–1.4 秒：全靜音
	for i := int(1.1 * rate); i < int(1.4*rate); i++ {
		if pcm[i] != 0x80 {
			t.Fatalf("靜音段第 %d 個取樣是 %02X", i, pcm[i])
		}
	}
}

func TestTonePCMErrorsWhenSilent(t *testing.T) {
	m := New()
	m.Out8(0x61, 0x02)
	if _, err := m.TonePCM(0); err == nil {
		t.Error("沒發過聲卻沒有回錯誤")
	}
}
