package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 規格 445：實模式的 SB 單次 8 位元 DMA 播完要送 IRQ7。反面不會報錯，
// 只會讓等「播放中」旗標的程式永遠停住，所以判準是 ISR 有沒有真的跑。

const (
	sbTestHandlerSeg = 0x4000
	sbTestCounter    = sbTestHandlerSeg*16 + 0x20
	sbTestBuf        = 0x7B120 // 智冠《魔法門 V》進店語音用的頁 07h、位移 B120h
	sbTestLen        = 100
)

// armSB 造一台 SB 存在、IRQ7 處理常式數自己被叫幾次、CPU 在 `jmp $` 空轉的機器。
func armSB(t *testing.T) *Machine {
	t.Helper()
	m := New()
	m.SetSoundBlaster(true)
	m.Write16(0x0f*4, 0)
	m.Write16(0x0f*4+2, sbTestHandlerSeg)
	// inc byte cs:[0020] ; mov al,20h ; out 20h,al ; iret
	copy(m.Mem[sbTestHandlerSeg*16:], []byte{0x2E, 0xFE, 0x06, 0x20, 0x00, 0xB0, 0x20, 0xE6, 0x20, 0xCF})
	m.Mem[0x20000], m.Mem[0x20001] = 0xEB, 0xFE // jmp $
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x2000, 0
	m.CPU.Seg[cpu.SS] = 0x3000
	m.CPU.R[cpu.SP] = 0xFFF0
	m.CPU.SetFlags(cpu.IF)
	for i := 0; i < sbTestLen; i++ {
		m.Mem[sbTestBuf+i] = byte(i + 1)
	}
	return m
}

// startVoice 照智冠驅動 `8CC6:01D5`、`8CC6:01FD` 的順序寫埠：
// 遮罩通道 1、清正反器、模式 49h、位址、頁、計數、解除遮罩，DSP 40h BCh、14h 長度。
func startVoice(t *testing.T, m *Machine) {
	t.Helper()
	m.Out8(0x226, 1)
	m.Out8(0x226, 0)
	// 遮蔽 IRQ0：這台沒裝 DOS 層，計時器 stub 轉呼的 int 1Ch 回不來，與 IRQ7 無關。
	m.Out8(0x21, 0x01)
	for _, pv := range [][2]uint16{{0x0A, 5}, {0x0C, 0}, {0x0B, 0x49}, {0x02, 0x20}, {0x02, 0xB1},
		{0x83, 0x07}, {0x03, sbTestLen - 1}, {0x03, 0}, {0x0A, 1}} {
		m.Out8(pv[0], uint8(pv[1]))
	}
	for _, v := range []uint8{0x40, 0xBC, 0x14, sbTestLen - 1, 0} {
		m.Out8(0x22C, v)
	}
}

func runUntilIRQ7(t *testing.T, m *Machine, limit int) int {
	t.Helper()
	for i := 0; i < limit; i++ {
		if m.Mem[sbTestCounter] != 0 {
			return i
		}
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatalf("跑了 %d 道指令，IRQ7 處理常式一次都沒跑", limit)
	return 0
}

func TestRealModeSBSingleCycleRaisesIRQ7Once(t *testing.T) {
	m := armSB(t)
	startVoice(t, m)
	runUntilIRQ7(t, m, 20_000_000)
	for i := 0; i < 1_000_000; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if got := m.Mem[sbTestCounter]; got != 1 {
		t.Fatalf("單次傳輸的 IRQ7 送了 %d 次，該是 1", got)
	}
	// 驅動的 ISR（`8CC6:022B`）：清正反器、讀兩次計數，FFFFh 才算播完。
	m.Out8(0x0C, 0)
	cnt := uint16(m.In8(0x03)) | uint16(m.In8(0x03))<<8
	if cnt != 0xFFFF {
		t.Fatalf("終點後計數讀到 %04X，該是 FFFF", cnt)
	}
	addr := uint16(m.In8(0x02)) | uint16(m.In8(0x02))<<8
	if addr != 0xB120+sbTestLen {
		t.Fatalf("終點後位址 %04X，該是 %04X", addr, 0xB120+sbTestLen)
	}
	m.Out8(0x224, 0x82)
	if v := m.In8(0x225); v&1 == 0 {
		t.Fatalf("IRQ 來源暫存器 %02X 沒有 8 位元來源", v)
	}
	m.In8(0x22E)
	if v := m.In8(0x225); v&1 != 0 {
		t.Fatalf("讀 22Eh 之後 8 位元來源仍在：%02X", v)
	}
}

func TestRealModeSBTimingFollowsSampleRate(t *testing.T) {
	m := armSB(t)
	startVoice(t, m)
	steps := runUntilIRQ7(t, m, 20_000_000)
	// 時間常數 BCh：1000000/(256−188) Hz。每個取樣約 每秒指令數×68/1000000 道。
	perSample := m.StepsPerSecondNow() * 68 / 1_000_000
	want := perSample * sbTestLen
	if float64(steps) < want || float64(steps) > want+perSample+64 {
		t.Fatalf("播 %d 個取樣用了 %d 道指令，預期約 %.0f", sbTestLen, steps, want)
	}
	if len(m.SBPCM) != sbTestLen || m.SBPCM[0] != 1 || m.SBPCM[sbTestLen-1] != sbTestLen {
		t.Fatalf("傳出的取樣 %d 個，頭尾 %v", len(m.SBPCM), m.SBPCM)
	}
	if m.SBDMACompletions != 1 || m.IRQ7Deliveries != 1 || m.SBDMARejects != 0 {
		t.Fatalf("完成 %d、IRQ7 %d、拒絕 %d", m.SBDMACompletions, m.IRQ7Deliveries, m.SBDMARejects)
	}
}

func TestRealModeSBHoldsIRQ7WhileMaskedOrStub(t *testing.T) {
	m := armSB(t)
	startVoice(t, m)
	m.Out8(0x21, 0x81) // 遮蔽 IRQ7（IRQ0 照 startVoice 遮著）
	m.Write16(0x0f*4+2, StubSeg)
	for i := 0; i < 2_000_000; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.SBDMACompletions != 1 || m.Mem[sbTestCounter] != 0 {
		t.Fatalf("完成 %d，遮蔽中處理常式跑了 %d 次", m.SBDMACompletions, m.Mem[sbTestCounter])
	}
	m.Out8(0x21, 0x01)
	for i := 0; i < 1000; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.IRQ7Stalls == 0 || m.IRQ7Deliveries != 0 {
		t.Fatalf("向量指 stub：留著 %d 次、送出 %d 次（掛起=%v IF=%v 遮罩=%02X CS:IP=%04X:%04X）",
			m.IRQ7Stalls, m.IRQ7Deliveries, m.sb.irq7Pending, m.CPU.Flag(cpu.IF), m.picMask, m.CPU.Seg[cpu.CS], m.CPU.IP)
	}
	m.Write16(0x0f*4+2, sbTestHandlerSeg)
	runUntilIRQ7(t, m, 1000)
}

func TestRealModeSBResetCancelsTransfer(t *testing.T) {
	m := armSB(t)
	startVoice(t, m)
	for i := 0; i < 1000; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	m.Out8(0x226, 1)
	m.Out8(0x226, 0)
	for i := 0; i < 2_000_000; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.SBDMACompletions != 0 || m.Mem[sbTestCounter] != 0 {
		t.Fatalf("DSP 重設之後仍完成 %d 次、IRQ7 %d 次", m.SBDMACompletions, m.Mem[sbTestCounter])
	}
}

func TestRealModeSBRejectsUnprogrammedChannel(t *testing.T) {
	m := armSB(t)
	m.Out8(0x226, 1)
	m.Out8(0x226, 0)
	for _, v := range []uint8{0x40, 0xBC, 0x14, sbTestLen - 1, 0} {
		m.Out8(0x22C, v)
	}
	if m.SBDMARejects != 1 || m.sb.active {
		t.Fatalf("DMA 沒設就開始：拒絕 %d、active=%v", m.SBDMARejects, m.sb.active)
	}
}

// 播到一半存檔、讀回另一台：IRQ7 要在同一道指令送出。
func TestRealModeSBSurvivesStateFileAndSnapshot(t *testing.T) {
	ref := armSB(t)
	startVoice(t, ref)
	want := runUntilIRQ7(t, ref, 20_000_000)

	m := armSB(t)
	startVoice(t, m)
	half := want / 2
	for i := 0; i < half; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := m.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	n := New()
	n.SetSoundBlaster(true)
	if err := n.LoadState(&buf); err != nil {
		t.Fatal(err)
	}
	if got := half + runUntilIRQ7(t, n, 20_000_000); got != want {
		t.Fatalf("讀檔之後 IRQ7 在第 %d 道，連續跑是第 %d 道", got, want)
	}

	s := m.Snapshot()
	if got := half + runUntilIRQ7(t, m, 20_000_000); got != want {
		t.Fatalf("快照前接著跑 IRQ7 在第 %d 道，該是 %d", got, want)
	}
	m.Restore(s)
	if got := half + runUntilIRQ7(t, m, 20_000_000); got != want {
		t.Fatalf("還原快照之後 IRQ7 在第 %d 道，該是 %d", got, want)
	}
}
