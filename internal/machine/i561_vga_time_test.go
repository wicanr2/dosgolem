package machine

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// blankDot 是水平消隱窗（640–783）裡的一個 dot，離兩邊都夠遠：
// 一道指令走 frameDots/VGAFrameEvery ≈ 2.2 dots，反推再正推會落在
// dot..dot+2，貼著邊界挑會自己抖進抖出。
const blankDot = 650

func TestI561Mode13ReadDoesNotAdvanceTime(t *testing.T) {
	m := New()
	m.SetVideoMode(0x13)
	m.Steps = stepsAtDot(blankDot, m.VGAFrameEvery)
	for i := 0; i < 100; i++ {
		if got := m.In8(0x3da); got != 1 {
			t.Fatalf("第 %d 次讀得 %02x，要水平消隱（讀 3DA 本身不該推進時間）", i, got)
		}
	}
}

func TestI561Mode13ExecutionAdvancesTime(t *testing.T) {
	m := New()
	m.SetVideoMode(0x13)
	m.CPU.Seg[cpu.CS] = 0x100
	m.CPU.IP = 0
	steps := stepsAtDot(640, m.VGAFrameEvery)
	for i := uint32(0); i < uint32(steps)+64; i++ {
		m.Write8(0x1000+i, 0x90)
	}
	if m.In8(0x3da) != 0 {
		t.Fatal("一開始要在顯示期")
	}
	for i := uint64(0); i < steps; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.In8(0x3da) != 1 {
		t.Fatal("沒有 IN 的指令也要推進顯示相位")
	}
}

func TestI561Mode13TimingBoundaries(t *testing.T) {
	const frameEvery = DefaultVGAFrameEvery
	for _, tc := range []struct {
		line, x uint64
		want    uint8
	}{{0, 100, 0}, {0, blankDot, 1}, {0, 790, 0}, {399, 100, 0}, {400, 100, 1},
		{411, 100, 1}, {412, 100, 9}, {413, 100, 9}, {414, 100, 1}, {448, 100, 1}, {449, 100, 0}} {
		step := stepsAtDot(tc.line*800+tc.x, frameEvery)
		if got := mode13InputStatus(step, frameEvery); got != tc.want {
			t.Errorf("line %d dot %d 得 %d 要 %d", tc.line, tc.x, got, tc.want)
		}
	}
	for _, step := range []uint64{0, 80, 39000, 1 << 63, ^uint64(0)} {
		if mode13InputStatus(step, frameEvery) != mode13InputStatus(step%frameEvery, frameEvery) {
			t.Fatal("相位在大 steps 下溢位")
		}
	}
}

// TestI561Mode13FrameLengthComesFromMachine 釘住「這裡沒有第二份標定」。
//
// 反向對照：同一個 steps 在兩種幀長下**必須**給出不同的相位。Civ1 那份
// patch 寫死的 42,800（3 M 指令／秒）與 dosgolem 的 165,000 差 3.855 倍，
// 若哪天有人把比例搬回函式裡，frameEvery 就會被忽略而這條會紅。
func TestI561Mode13FrameLengthComesFromMachine(t *testing.T) {
	const civ1FrameEvery = 42_800
	steps := stepsAtDot(blankDot, DefaultVGAFrameEvery)
	if a, b := mode13InputStatus(steps, DefaultVGAFrameEvery), mode13InputStatus(steps, civ1FrameEvery); a == b {
		t.Fatalf("兩種幀長給出同一個相位 %02x——幀長沒有真的被用到", a)
	}
	if got := mode13InputStatus(steps, 0); got != 0 {
		t.Fatalf("frameEvery=0 是不產生幀，要回 0，得 %02x", got)
	}
	// 機器欄位就是那一份標定的來源：改欄位，3DA 跟著改。
	m := New()
	m.SetVideoMode(0x13)
	m.Steps = steps
	before := m.In8(0x3da)
	m.VGAFrameEvery = civ1FrameEvery
	if after := m.In8(0x3da); after == before {
		t.Fatalf("改 VGAFrameEvery 後 3DA 沒變（都是 %02x）", before)
	}
}
