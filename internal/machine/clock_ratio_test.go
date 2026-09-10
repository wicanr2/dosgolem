package machine

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 兩個時鐘的隱含速度（`docs/spec/191`）。
//
// 指令數時鐘與週期時鐘是兩個獨立模型：前者每道指令算一格，後者按類別
// 計費。兩者的比就是「平均每道指令幾個週期」，而**那隨程式的指令混合走**
// ——不是一個常數，所以兩個時鐘不可能對所有程式都一致。
//
// 這幾條量出那個比值的範圍，好讓「切換時鐘會差多少」有數字可談。

// mix 是一段會重複執行的指令序列，代表一種指令混合。
type mix struct {
	name string
	code []byte
}

// clockMixes 涵蓋老遊戲實際會出現的幾種極端。
var clockMixes = []mix{
	// 暫存器對暫存器：最便宜的一類，週期時鐘與指令數時鐘最接近。
	{"暫存器運算", []byte{
		0x89, 0xD8, // mov ax, bx
		0x01, 0xC8, // add ax, cx
		0x31, 0xD0, // xor ax, dx
		0xEB, 0xF8, // jmp -8
	}},
	// 讀改寫記憶體：繪圖迴圈的另一半。
	{"記憶體讀改寫", []byte{
		0x8A, 0x04, // mov al, [si]
		0x08, 0xD8, // or al, bl
		0x88, 0x04, // mov [si], al
		0xEB, 0xF8,
	}},
	// 切平面 ＋ 讀改寫：**這就是老遊戲的繪圖迴圈**（`docs/spec/004` §5），
	// out 一道要 14 個週期，而指令數時鐘把它算成一格。
	{"繪圖（out ＋ 讀改寫）", []byte{
		0xE6, 0xC4, // out 0xC4, al
		0x8A, 0x04, // mov al, [si]
		0x08, 0xD8, // or al, bl
		0x88, 0x04, // mov [si], al
		0xEB, 0xF6,
	}},
}

// TestClockRatioAcrossMixes 量各種混合的「週期／指令」。
//
// 報表是給人看的：兩個時鐘要一致，比值得剛好等於
// `DefaultCPUHz / StepsPerSecond()`；量出來高於它，就表示走週期時鐘時
// 那段程式的遊戲內時間過得比指令數時鐘慢。
func TestClockRatioAcrossMixes(t *testing.T) {
	want := DefaultCPUHz / StepsPerSecond() // 兩個時鐘一致所需的比值
	t.Logf("兩個時鐘一致需要：%.2f 週期／指令", want)
	for _, m := range clockMixes {
		got := measureCyclesPerStep(t, m.code, 60_000)
		t.Logf("%-22s %.2f 週期／指令（走週期時鐘時慢 %.2f 倍）",
			m.name, got, got/want)
		if got < 1 {
			t.Errorf("%s：量到 %.2f 週期／指令，不可能低於 1", m.name, got)
		}
	}
}

// TestDrawingLoopIsMuchSlowerOnTheCycleClock 釘住「繪圖迴圈是兩個時鐘
// 差最多的地方」——那正是週期時鐘存在的理由（`docs/spec/004` §5）。
//
// 這一條反過來壞掉（繪圖與暫存器運算的比值變得差不多）就表示 out 的
// 成本被改掉了，而症狀是動畫速度整個變樣。
func TestDrawingLoopIsMuchSlowerOnTheCycleClock(t *testing.T) {
	reg := measureCyclesPerStep(t, clockMixes[0].code, 60_000)
	draw := measureCyclesPerStep(t, clockMixes[2].code, 60_000)
	if draw <= reg*1.5 {
		t.Errorf("繪圖 %.2f 週期／指令，暫存器運算 %.2f——差距太小，"+
			"out 的成本是不是被改掉了", draw, reg)
	}
}

// measureCyclesPerStep 跑 n 道指令，回平均每道幾個週期。
func measureCyclesPerStep(t *testing.T, code []byte, n int) float64 {
	t.Helper()
	m := New()
	if err := m.LoadCOM(code); err != nil {
		t.Fatal(err)
	}
	m.IRQ0Every = 0 // 不要讓計時器中斷混進來
	m.CPU.R[cpu.SI] = 0x2000
	for i := 0; i < n; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	return float64(m.CPU.Cycles) / float64(m.Steps)
}
