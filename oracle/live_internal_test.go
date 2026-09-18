package oracle

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// `docs/spec/199`：即時執行介面。合成的 `jmp $` 程式，不需要原版素材。

func liveOracle(t *testing.T) *Oracle {
	t.Helper()
	code := make([]byte, 0x400)
	copy(code, []byte{0xEB, 0xFE}) // jmp $：每道指令 1 個 DOSBox cycle
	hdr := make([]byte, 32)
	put := func(off int, v uint16) { binary.LittleEndian.PutUint16(hdr[off:], v) }
	total := len(hdr) + len(code)
	copy(hdr, "MZ")
	put(0x02, uint16(total%512))
	put(0x04, uint16((total+511)/512))
	put(0x08, 2)
	put(0x0A, 0x10)
	put(0x0C, 0xFFFF)
	put(0x10, 0x0400)
	put(0x18, 0x001C)
	path := filepath.Join(t.TempDir(), "LIVE.EXE")
	if err := os.WriteFile(path, append(hdr, code...), 0o644); err != nil {
		t.Fatal(err)
	}
	o, err := Load(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(o.Close)
	return o
}

// §4 第 1、2 項。
func TestRunCyclesAdvancesExactly(t *testing.T) {
	o := liveOracle(t)
	if err := o.RunCycles(1000); err == nil {
		t.Fatal("沒開 DOSBox cycles 卻沒有回錯誤")
	}
	o.SetDOSBoxCycles(machine.CyclesAT8)
	for _, n := range []uint64{1, 12_500, 750_000} {
		before := o.Cycles()
		if err := o.RunCycles(n); err != nil {
			t.Fatal(err)
		}
		if d := o.Cycles() - before; d != n { // jmp $ 一道 1 個 cycle，剛好停在目標
			t.Errorf("RunCycles(%d) 跑了 %d 個 cycle", n, d)
		}
	}
}

func makes(ks []machine.ScheduledKey, scan uint8) int {
	n := 0
	for _, k := range ks {
		if k.Scan == scan && !k.Break {
			n++
		}
	}
	return n
}

// §4 第 3 項：以 240 cycles 分幀按住 2 秒，按下碼數與 HoldKey 相差 ≤ 1；放開後最後一個是放開碼。
// 合成程式沒有裝 int 09h，事件會一直留在排程裡，剛好拿來數。
func TestKeyDownUpTypematicMatchesHoldKey(t *testing.T) {
	o := liveOracle(t)
	o.SetDOSBoxCycles(machine.CyclesXT)
	start := o.m.Steps
	o.KeyDown(0x39)
	o.KeyDown(0x39) // 重複按下不動
	frame := uint64(machine.CyclesXT * 1000 / 60)
	for i := 0; i < 120; i++ { // 2 秒
		if err := o.RunCycles(frame); err != nil {
			t.Fatal(err)
		}
	}
	if got := o.HeldKeys(); len(got) != 1 || got[0] != 0x39 {
		t.Fatalf("HeldKeys = %v", got)
	}
	o.KeyUp(0x39)
	o.KeyUp(0x39)
	live := o.m.ScheduledKeys()
	if last := live[len(live)-1]; last.Scan != 0x39 || !last.Break {
		t.Errorf("最後一個事件 %+v，要 39 的放開碼", last)
	}
	for _, k := range live {
		if !k.Break && k.Step > o.m.Steps {
			t.Errorf("放開後還有按下碼排在第 %d 步", k.Step)
		}
	}

	ref := machine.New()
	ref.SetDOSBoxCycles(machine.CyclesXT)
	ref.HoldKey(0x39, start, o.m.Steps-start, true)
	a, b := makes(live, 0x39), makes(ref.ScheduledKeys(), 0x39)
	if d := a - b; d < -1 || d > 1 {
		t.Errorf("即時按住 %d 個按下碼，HoldKey %d 個", a, b)
	}
	if a < 10 {
		t.Errorf("2 秒只有 %d 個按下碼，typematic 沒有作用", a)
	}
}

// §4 第 4 項。
func TestCancelTimedKeysOnlyLaterMakesOfThatKey(t *testing.T) {
	m := machine.New()
	m.ScheduleKey(10, 0x39, false)
	m.ScheduleKey(20, 0x39, false)
	m.ScheduleKey(20, 0x39, true)
	m.ScheduleKey(30, 0x48, false)
	if n := m.CancelTimedKeys(0x39, 10); n != 1 {
		t.Errorf("移除 %d 個，要 1 個", n)
	}
	ks := m.ScheduledKeys()
	if len(ks) != 3 || ks[0].Step != 10 || !ks[1].Break || ks[2].Scan != 0x48 {
		t.Errorf("剩下 %+v", ks)
	}
}

// §4 第 5 項：靜音時取樣數不漂移、全為 0，紀錄被清掉。
func TestAudioSilenceLengthAndTrim(t *testing.T) {
	o := liveOracle(t)
	o.SetDOSBoxCycles(machine.CyclesAT8)
	a := o.NewAudio(44100)
	total, samples := uint64(0), 0
	for _, n := range []uint64{12_345, 1, 750_000, 99_999, 3, 256_000, 7, 12_500, 40_000, 1_234_567} {
		if err := o.RunCycles(n); err != nil {
			t.Fatal(err)
		}
		total += n
		pcm := a.Render()
		samples += len(pcm)
		for _, v := range pcm {
			if v != 0 {
				t.Fatalf("靜音卻有取樣 %d", v)
			}
		}
	}
	want := float64(total) / 750_000 * 44100
	if d := float64(samples) - want; math.Abs(d) > 1 {
		t.Errorf("取樣 %d，要 %.1f ±1", samples, want)
	}
}

// §4 第 5 項：440 Hz 方波的過零頻率，與 Render 後清掉紀錄（反向對照：拿掉清除時長度不為 0）。
func TestAudioToneFrequency(t *testing.T) {
	o := liveOracle(t)
	o.SetDOSBoxCycles(machine.CyclesAT8)
	a := o.NewAudio(44100)
	div := uint16(math.Round(machine.PITBaseHz / 440))
	o.m.Out8(0x43, 0xB6)
	o.m.Out8(0x42, uint8(div))
	o.m.Out8(0x42, uint8(div>>8))
	o.m.Out8(0x61, 0x03)
	o.m.Steps += 750_000
	o.m.CPU.Cycles += 750_000
	pcm := a.Render()
	if len(pcm) != 44100 {
		t.Fatalf("1 秒 %d 個取樣", len(pcm))
	}
	cross := 0
	for i := 1; i < len(pcm); i++ {
		if (pcm[i-1] < 0) != (pcm[i] < 0) {
			cross++
		}
	}
	if hz := float64(cross) / 2; math.Abs(hz-440)/440 > 0.01 {
		t.Errorf("過零頻率 %.1f Hz，要 440 ±1%%", hz)
	}
	if len(o.m.PortLog) != 0 {
		t.Errorf("Render 後 PortLog 還有 %d 筆", len(o.m.PortLog))
	}
}

// §4 第 5 項：步數與 cycles 不成比例時，事件依 cycles 定位。
// 本段 750,000 cycles 裡，前 1% 的步數（7,500 步）用掉 93% 的 cycles（繪圖的字串指令），之後才開始發聲。
// 反向對照：依步數定位時，音會從第 1% 的取樣附近開始，此項失敗。
func TestAudioEventsPlacedByCycles(t *testing.T) {
	o := liveOracle(t)
	o.SetDOSBoxCycles(machine.CyclesAT8)
	a := o.NewAudio(44100)
	o.m.Steps += 7_500
	o.m.CPU.Cycles += 697_500 // 93%
	div := uint16(math.Round(machine.PITBaseHz / 440))
	o.m.Out8(0x43, 0xB6)
	o.m.Out8(0x42, uint8(div))
	o.m.Out8(0x42, uint8(div>>8))
	o.m.Out8(0x61, 0x03)
	o.m.Steps += 742_500
	o.m.CPU.Cycles += 52_500
	pcm := a.Render()
	first := -1
	for i, v := range pcm {
		if v != 0 {
			first = i
			break
		}
	}
	if want := int(0.93 * 44100); first < want-50 || first > want+50 {
		t.Errorf("第一個非零取樣在 %d，要 %d ±50（依 cycles 定位）", first, want)
	}
}

// §4 第 5 項：OPL2 Key-On 之後有聲音、紀錄被清掉。
func TestAudioOPL2KeyOn(t *testing.T) {
	o := liveOracle(t)
	o.SetDOSBoxCycles(machine.CyclesAT8)
	o.SetAdLib(true)
	a := o.NewAudio(44100)
	for _, rv := range [][2]uint8{{0x20, 0x01}, {0x23, 0x01}, {0x40, 0x10}, {0x43, 0x00}, {0x60, 0xF0}, {0x63, 0xF0},
		{0x80, 0x77}, {0x83, 0x77}, {0xA0, 0x98}, {0xB0, 0x31}} {
		o.m.Out8(0x388, rv[0])
		o.m.Out8(0x389, rv[1])
	}
	o.m.Steps += 75_000
	o.m.CPU.Cycles += 75_000
	nz := 0
	for _, v := range a.Render() {
		if v != 0 {
			nz++
		}
	}
	if nz == 0 {
		t.Error("Key-On 之後沒有聲音")
	}
	if len(o.m.OPL) != 0 {
		t.Errorf("Render 後 OPL 紀錄還有 %d 筆", len(o.m.OPL))
	}
}

// §4 第 7 項。
func TestStateFileRoundTrip(t *testing.T) {
	o := liveOracle(t)
	o.SetDOSBoxCycles(machine.CyclesAT8)
	if err := o.RunCycles(5000); err != nil {
		t.Fatal(err)
	}
	o.m.Write8(0x20000, 0x5A)
	path := filepath.Join(t.TempDir(), "x.state")
	if err := o.SaveStateFile(path); err != nil {
		t.Fatal(err)
	}
	steps, ip, cs := o.m.Steps, o.m.CPU.IP, o.m.CPU.Seg[cpu.CS]
	if err := o.RunCycles(5000); err != nil {
		t.Fatal(err)
	}
	o.m.Write8(0x20000, 0)
	if err := o.LoadStateFile(path); err != nil {
		t.Fatal(err)
	}
	if o.m.Steps != steps || o.m.CPU.IP != ip || o.m.CPU.Seg[cpu.CS] != cs || o.m.Read8(0x20000) != 0x5A {
		t.Errorf("還原後 steps %d/%d、CS:IP %04X:%04X、位元組 %02X", o.m.Steps, steps, o.m.CPU.Seg[cpu.CS], o.m.CPU.IP, o.m.Read8(0x20000))
	}
	if o.DOSBoxCycles() != machine.CyclesAT8 {
		t.Errorf("還原後速度 %d", o.DOSBoxCycles())
	}
}

// §4：時鐘倒回（載入狀態檔）時 Render 不 panic、回 nil，之後照常出聲。
func TestAudioSurvivesClockRewind(t *testing.T) {
	o := liveOracle(t)
	o.SetDOSBoxCycles(machine.CyclesAT8)
	a := o.NewAudio(44100)
	if err := o.RunCycles(750_000); err != nil {
		t.Fatal(err)
	}
	if n := len(a.Render()); n == 0 {
		t.Fatal("跑了 1 秒應該有取樣")
	}
	if err := o.RunCycles(750_000); err != nil {
		t.Fatal(err)
	}
	a.Render()          // 對時到 1.5M cycles
	o.m.CPU.Cycles /= 2 // 模擬載入狀態檔：時鐘倒回到 0.75M
	o.m.Steps /= 2
	if pcm := a.Render(); pcm != nil {
		t.Fatalf("倒回的那一段應該回 nil，拿到 %d 個取樣", len(pcm))
	}
	if err := o.RunCycles(750_000); err != nil {
		t.Fatal(err)
	}
	if n := len(a.Render()); n < 40000 || n > 48000 {
		t.Errorf("倒回之後應該照常出聲，拿到 %d 個取樣", n)
	}
}
