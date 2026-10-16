package machine

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/444-legacy-sb16-dma8-irq7 §3 的 machine 層驗收（第 1、3–16 項；第 2 項的 machine 部分）。
// 程式以 bytes 現場組出，不讀原版檔。

// ---- 夾具 -------------------------------------------------------------------

// sbAsm 逐道組指令並記下每道的序號（從 1 起，等於執行該道那一步遞增後的 m.Steps）。
type sbAsm struct {
	code []byte
	n    int
}

func (a *sbAsm) op(b ...byte) int {
	a.code = append(a.code, b...)
	a.n++
	return a.n
}

// off 是下一道指令在 COM 段內的位移（COM 由 0100h 起載入）。
func (a *sbAsm) off() uint16 { return 0x100 + uint16(len(a.code)) }

const (
	sbFlagOff = 0x0300 // 處理常式寫的旗標（DS=PSP 段）
	sbRDOff   = 0x0301 // 讀 22Ah 的結果
	sbHOff    = 0x0200 // IRQ7 處理常式的位移
)

// sbDetectProg 依 §1.1 的次序組出偵測程式：重設（讀 22Ah）、40h A6h、讀改寫 A1h／21h、DMA 通道 1、14h 03h 00h、sti、輪詢旗標。
// 回程式、長度高位元組那一道的序號。
func sbDetectProg() ([]byte, int) {
	var a sbAsm
	a.op(0xBA, 0x26, 0x02) // mov dx,226h
	a.op(0xB0, 0x01)
	a.op(0xEE)
	a.op(0xB0, 0x00)
	a.op(0xEE)
	a.op(0xBA, 0x2A, 0x02) // mov dx,22Ah
	a.op(0xEC)             // in al,dx
	a.op(0xA2, byte(sbRDOff&0xFF), byte(sbRDOff>>8))
	a.op(0xBA, 0x2C, 0x02) // mov dx,22Ch
	a.op(0xB0, 0x40)
	a.op(0xEE)
	a.op(0xB0, 0xA6)
	a.op(0xEE)
	a.op(0xE4, 0xA1) // in al,A1h
	a.op(0xE6, 0xA1) // out A1h,al
	a.op(0xE4, 0x21) // in al,21h
	a.op(0x24, 0x7F) // and al,7Fh
	a.op(0xE6, 0x21) // out 21h,al
	a.op(0xB0, 0x05)
	a.op(0xE6, 0x0A) // 遮罩通道 1
	a.op(0xB0, 0x00)
	a.op(0xE6, 0x83) // 頁 0
	a.op(0xE6, 0x0C) // 清 flip-flop
	a.op(0xE6, 0x02) // 位址 0
	a.op(0xE6, 0x02)
	a.op(0xB0, 0x03)
	a.op(0xE6, 0x03) // 計數 3
	a.op(0xB0, 0x00)
	a.op(0xE6, 0x03)
	a.op(0xB0, 0x49)
	a.op(0xE6, 0x0B) // 模式 49h
	a.op(0xB0, 0x01)
	a.op(0xE6, 0x0A) // 解除遮罩
	a.op(0xB0, 0x14)
	a.op(0xEE)
	a.op(0xB0, 0x03)
	a.op(0xEE)
	a.op(0xB0, 0x00)
	s := a.op(0xEE) // 長度高位元組
	a.op(0xFB)      // sti
	a.op(0x80, 0x3E, byte(sbFlagOff&0xFF), byte(sbFlagOff>>8), 0x01)
	a.op(0x75, 0xF9) // jne 上一道
	a.op(0xEB, 0xFE) // jmp $
	code := a.code
	for uint16(len(code))+0x100 < sbHOff {
		code = append(code, 0x90)
	}
	// 處理常式：讀 22Eh、寫旗標、out 20h,20h、iret（不執行 sti，同 §1.2 的 05C1）。
	code = append(code,
		0x50, 0x52, // push ax／dx
		0xBA, 0x2E, 0x02, 0xEC, // mov dx,22Eh；in al,dx
		0xC6, 0x06, byte(sbFlagOff&0xFF), byte(sbFlagOff>>8), 0x01,
		0xB0, 0x20, 0xE6, 0x20, // out 20h,20h
		0x5A, 0x58, 0xCF)
	return code, s
}

// newSB8 造一台開了 Sound Blaster 與 444 選項的實模式機器並載入 code。
func newSB8(t *testing.T, code []byte, prep func(m *Machine)) *Machine {
	t.Helper()
	m := New()
	m.SetSoundBlaster(true)
	mustNil(t, "SetSBDMA8", m.SetSBDMA8(true))
	if prep != nil {
		prep(m)
	}
	mustNil(t, "LoadCOM", m.LoadCOM(code))
	return m
}

// setIVT 設中斷向量 vec 為 seg:off。
func setIVT(m *Machine, vec uint8, seg, off uint16) {
	m.WritePhys16(uint32(vec)*4, off)
	m.WritePhys16(uint32(vec)*4+2, seg)
}

// bigDelta 以 math/big 獨立算 ⌈Πnum / Πden⌉（§2.4）。
func bigDelta(num, den []uint64) uint64 {
	n, d := big.NewInt(1), big.NewInt(1)
	for _, v := range num {
		n.Mul(n, new(big.Int).SetUint64(v))
	}
	for _, v := range den {
		d.Mul(d, new(big.Int).SetUint64(v))
	}
	q, r := new(big.Int).QuoRem(n, d, new(big.Int))
	if r.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	return q.Uint64()
}

func runSteps(t *testing.T, m *Machine, n int, what string) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := m.Step(); err != nil {
			t.Fatalf("%s：第 %d 步：%v", what, m.Steps, err)
		}
	}
}

func logKinds(m *Machine) string {
	var k []string
	for _, e := range m.SBDMALog {
		k = append(k, e.Kind)
	}
	return strings.Join(k, ",")
}

// sbProgDMA 以宿主寫入程式通道 1（頁、位址、計數、模式 49h），並解除遮罩。
func sbProgDMA(m *Machine, page uint8, addr, count uint16) {
	for _, w := range [][2]uint16{{0x0A, 5}, {0x83, uint16(page)}, {0x0C, 0}, {0x02, addr & 0xFF}, {0x02, addr >> 8},
		{0x03, count & 0xFF}, {0x03, count >> 8}, {0x0B, 0x49}, {0x0A, 1}} {
		m.Out8(w[0], uint8(w[1]))
	}
}

// sbDSP 以宿主寫入 22Ch。
func sbDSP(m *Machine, vs ...uint8) {
	for _, v := range vs {
		m.Out8(0x22C, v)
	}
}

// sbIdle 是只有 jmp $ 的程式（IVT[0Fh] 由各測試自己設）。
var sbIdle = []byte{0xEB, 0xFE}

// ---- 第 1 項：選項關閉 ------------------------------------------------------

func TestSB8OffKeepsLegacyPorts(t *testing.T) {
	code, _ := sbDetectProg()
	m := New()
	m.SetSoundBlaster(true)
	mustNil(t, "LoadCOM", m.LoadCOM(code))
	setIVT(m, 0x0F, PSPSeg, sbHOff)
	runSteps(t, m, 10000, "選項關閉")
	if m.Mem[uint32(PSPSeg)*16+sbFlagOff] != 0 {
		t.Error("選項關閉：IRQ7 處理常式不應執行")
	}
	if m.Mem[uint32(PSPSeg)*16+sbRDOff] != 0xAA {
		t.Errorf("讀 22Ah 應得 AAh，得 %02X", m.Mem[uint32(PSPSeg)*16+sbRDOff])
	}
	for _, p := range []uint16{0x20, 0xA1, 0x08, 0x02} {
		if v := m.In8(p); v != 0xFF {
			t.Errorf("選項關閉：埠 %02Xh 應讀回 FFh，得 %02X", p, v)
		}
	}
	d := &m.soundBlaster
	if d.pending != 0x14 || !d.lengthHighNext || d.lengthLow != 3 || d.IRQPending {
		t.Errorf("選項關閉：DSP 半狀態應停在等長度高位元組（pending=14h、lengthHighNext、lengthLow=3）：%02X %t %d %t",
			d.pending, d.lengthHighNext, d.lengthLow, d.IRQPending)
	}
	n := 0
	for _, w := range m.PortLog {
		if w.Port <= 0x0F || w.Port == 0x83 {
			n++
		}
	}
	if n != 9 || m.SBDMA8() || len(m.SBDMALog) != 0 {
		t.Errorf("選項關閉：DMA 埠寫入應只進 PortLog（9 筆）：%d；SBDMA8=%t", n, m.SBDMA8())
	}
}

// ---- 第 2 項（machine 部分）：SetSBDMA8 的條件 ------------------------------

func TestSB8SetConditions(t *testing.T) {
	m := New()
	if err := m.SetSBDMA8(true); err == nil || m.SBDMA8() {
		t.Error("未開 Sound Blaster：應回錯誤且狀態不變")
	}
	m = New()
	m.SetSoundBlaster(true)
	mustNil(t, "LoadCOM", m.LoadCOM(sbIdle))
	if err := m.SetSBDMA8(true); err == nil || m.SBDMA8() {
		t.Error("programLoaded 為真：應回錯誤且狀態不變")
	}
	m = New()
	m.SetSoundBlaster(true)
	m.Mem = m.Mem[:MemSize-16]
	if err := m.SetSBDMA8(true); err == nil || m.SBDMA8() {
		t.Error("Mem 長度不符：應回錯誤且狀態不變")
	}
	m = New()
	m.SetSoundBlaster(true)
	mustNil(t, "SetSBDMA8(true)", m.SetSBDMA8(true))
	if !m.SBDMA8() || !m.SoundBlaster() {
		t.Error("開啟後 SBDMA8()、SoundBlaster() 應為真")
	}
	if err := m.SetSBDMA8(false); err == nil || !m.SBDMA8() {
		t.Error("開啟後再關閉：應回錯誤且狀態不變")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("sb8 開啟後 SetSoundBlaster(false) 應 panic")
			}
		}()
		m.SetSoundBlaster(false)
	}()
	if !m.SoundBlaster() {
		t.Error("panic 之後 Sound Blaster 應仍開啟")
	}
}

// ---- 第 3 項：實模式合成程式與時鐘變體 --------------------------------------

func TestSB8RealModeDetect(t *testing.T) {
	code, s := sbDetectProg()
	if s != 39 {
		t.Fatalf("長度高位元組那一道的序號應為 39，組出 %d", s)
	}
	for _, x := range []struct {
		name     string
		prep     func(m *Machine)
		delta    uint64 // 寫死的 Δ（指令數時鐘）
		big      uint64 // math/big 獨立驗算
		complete uint64 // 完成與遞送那一步（遞增前的 m.Steps）
	}{
		{"指令數時鐘（預設）", nil, 4170,
			bigDelta([]uint64{4, 90, 636085, 315000000}, []uint64{1000000, 264, 65536}), 39 + 4170},
		{"IRQ0Pinned（分頻 17000、IRQ0Every 100000）", func(m *Machine) {
			m.Out8(0x43, 0x36)
			m.Out8(0x40, 17000&0xFF)
			m.Out8(0x40, 17000>>8)
			m.IRQ0Pinned, m.IRQ0Every = true, 100000
		}, 2527, bigDelta([]uint64{4, 90, 100000, 315000000}, []uint64{1000000, 264, 17000}), 39 + 2527},
		{"IRQ0Every=0、ClockPercent=50", func(m *Machine) {
			mustNil(t, "SetClockPercent", m.SetClockPercent(50))
			m.IRQ0Every = 0
		}, 2085, bigDelta([]uint64{4, 90, 165000, 315000000, 50}, []uint64{1000000, 264, 17000, 100}), 39 + 2085},
	} {
		t.Run(x.name, func(t *testing.T) {
			if x.delta != x.big {
				t.Fatalf("寫死的 Δ %d 與 math/big 驗算 %d 不符", x.delta, x.big)
			}
			m := newSB8(t, code, x.prep)
			setIVT(m, 0x0F, PSPSeg, sbHOff)
			sbCheckDetect(t, m, s, func(m *Machine) uint64 { return m.Steps }, x.delta, x.complete)
		})
	}
	for _, hz := range []uint64{20_000_000, 0} {
		t.Run("週期時鐘", func(t *testing.T) {
			m := newSB8(t, code, func(m *Machine) { m.CycleClock, m.CPUHz = true, hz; m.RecalcIRQ0() })
			setIVT(m, 0x0F, PSPSeg, sbHOff)
			h := hz
			if h == 0 {
				h = DefaultCPUHz
			}
			delta := bigDelta([]uint64{4, 90, h}, []uint64{1000000})
			sbCheckDetect(t, m, s, func(m *Machine) uint64 { return m.CPU.Cycles }, delta, 0)
		})
	}
}

// sbCheckDetect 跑 sbDetectProg 到處理常式結束後，檢查 §3 第 3 項的各條。clock 回傳輸所用的時鐘；
// complete 為 0 時（週期時鐘）以「每步開始時的時鐘首次 ≥ 開始＋Δ」獨立推完成步號。
func sbCheckDetect(t *testing.T, m *Machine, s int, clock func(*Machine) uint64, delta, complete uint64) {
	t.Helper()
	var start, prevBefore uint64
	found := false
	for i := 0; i < 20000 && m.Mem[uint32(PSPSeg)*16+sbFlagOff] == 0; i++ {
		before := clock(m)
		if i > 0 && !found {
			prevBefore = before
		}
		stepsBefore := m.Steps
		if err := m.Step(); err != nil {
			t.Fatalf("第 %d 步：%v", m.Steps, err)
		}
		if m.Steps == uint64(s) {
			w := m.PortLog[len(m.PortLog)-1]
			if w.Port != 0x22C || w.Val != 0 || w.Step != uint64(s) {
				t.Fatalf("第 %d 步應寫長度高位元組：%+v", s, w)
			}
			start = w.Step
			if m.CycleClock {
				start = w.Cycles
			}
		}
		if !found && m.SBDMA8Stats().DMA8Completions == 1 {
			found = true
			if complete == 0 {
				if before < start+delta {
					t.Errorf("完成時的時鐘 %d 未達到期 %d", before, start+delta)
				}
				if prevBefore >= start+delta && prevBefore != before {
					t.Errorf("前一步開始時的時鐘 %d 已到期 %d，完成應更早", prevBefore, start+delta)
				}
				if prev := m.SBDMALog[0]; prev.Kind != "start" {
					t.Errorf("第一筆紀錄應為 start：%s", prev.Kind)
				}
				complete = stepsBefore
			}
			if stepsBefore != complete {
				t.Errorf("完成應發生在遞增前 m.Steps＝%d 的 Step 開頭，實際 %d", complete, stepsBefore)
			}
		}
	}
	if !found || m.Mem[uint32(PSPSeg)*16+sbFlagOff] != 1 {
		t.Fatalf("處理常式應執行並寫旗標：完成=%t", found)
	}
	runSteps(t, m, 3, "處理常式之後")
	st := m.SBDMA8Stats()
	if st.IRR7 || st.ISR7 || m.soundBlaster.IRQPending || st.IRQ7Deliveries != 1 {
		t.Errorf("處理常式之後 IRR7、ISR7、IRQPending 應為 0、遞送 1 次：%+v IRQPending=%t", st, m.soundBlaster.IRQPending)
	}
	if got := logKinds(m); got != "start,complete,irq7,eoi" {
		t.Errorf("觀測紀錄應依序為 start,complete,irq7,eoi：%s", got)
	}
	ev := m.SBDMALog
	if len(ev) == 4 {
		if ev[0].Due-func() uint64 {
			if m.CycleClock {
				return ev[0].Cycles
			}
			return ev[0].Step
		}() != delta {
			t.Errorf("start 的 Due 應為開始＋Δ（Δ=%d）：%+v", delta, ev[0])
		}
		if ev[1].Step != complete || ev[2].Step != complete || ev[2].CPUMode != "real" {
			t.Errorf("complete、irq7 應在第 %d 步（real）：%d %d %s", complete, ev[1].Step, ev[2].Step, ev[2].CPUMode)
		}
		sum := sha256.Sum256(m.Mem[0:4])
		want := hex.EncodeToString(sum[:])
		if ev[0].SHA256 != want || ev[1].SHA256 != want {
			t.Errorf("start、complete 的 SHA256 應等於 Mem[0:4] 的 %s：%s %s", want, ev[0].SHA256, ev[1].SHA256)
		}
		if ev[0].Format != "pcm8" || ev[0].Length != 4 || ev[0].Samples != 4 || ev[0].Phys != 0 || ev[0].DMAMode != 0x48 ||
			ev[0].Channel != 1 || ev[0].Count != 3 || ev[0].RateDen != 90 || ev[0].RateNum != 1000000 || ev[0].TimeConstant != 0xA6 {
			t.Errorf("start 的欄位不符：%+v", ev[0])
		}
		if ev[1].Transferred != 4 {
			t.Errorf("complete 的 Transferred 應為 4：%d", ev[1].Transferred)
		}
	}
	d := m.sb8.dma
	if d.Current[2] != 4 || d.Current[3] != 0xFFFF || d.Mask&2 == 0 {
		t.Errorf("通道 1 應為位址 4、計數 FFFFh、遮罩：%04X %04X mask=%X", d.Current[2], d.Current[3], d.Mask)
	}
	if lo, hi := m.In8(0x03), m.In8(0x03); lo != 0xFF || hi != 0xFF {
		t.Errorf("不先清 flip-flop 連讀兩次 03h 應得 FFh FFh：%02X %02X", lo, hi)
	}
	if v := m.In8(0x08); v != 0x02 {
		t.Errorf("讀 08h 應得 bit1：%02X", v)
	}
	if v := m.In8(0x08); v != 0 {
		t.Errorf("再讀 08h 應得 0：%02X", v)
	}
}

// ---- 第 4 項：ADPCM 與靜音 ---------------------------------------------------

func TestSB8ADPCMAndSilence(t *testing.T) {
	for _, x := range []struct {
		cmd  uint8
		mult uint64
		fmt  string
	}{{0x74, 2, "adpcm4"}, {0x75, 2, "adpcm4ref"}, {0x76, 3, "adpcm26"}, {0x77, 3, "adpcm26ref"},
		{0x16, 4, "adpcm2"}, {0x17, 4, "adpcm2ref"}, {0x80, 1, "silence"}} {
		m := newSB8(t, sbIdle, nil)
		m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
		runSteps(t, m, 5, "暖機")
		sbProgDMA(m, 1, 0x40, 9) // 頁 1：實體 10040h 起
		for i := uint32(0); i < 10; i++ {
			m.Mem[0x10040+i] = byte(0xA0 + i)
		}
		before := *m.sb8.dma
		sbDSP(m, 0x40, 0xA6, x.cmd, 9, 0) // n=10
		start := m.Steps
		delta := bigDelta([]uint64{10 * x.mult, 90, 636085, 315000000}, []uint64{1000000, 264, 65536})
		if len(m.SBDMALog) != 1 || m.SBDMALog[0].Due != start+delta || m.SBDMALog[0].Format != x.fmt ||
			m.SBDMALog[0].Samples != 10*x.mult {
			t.Fatalf("%02Xh：start 的到期應為 %d＋%d：%+v", x.cmd, start, delta, m.SBDMALog)
		}
		for m.Steps < start+delta {
			runSteps(t, m, 1, "等到期")
			if m.sb8.irr7 {
				t.Fatalf("%02Xh：第 %d 步提早完成", x.cmd, m.Steps)
			}
		}
		runSteps(t, m, 1, "完成")
		if !m.sb8.irr7 || m.sb8.active {
			t.Fatalf("%02Xh：到期後應拉 IRR7", x.cmd)
		}
		if x.cmd == 0x80 {
			e := m.SBDMALog[0]
			if *m.sb8.dma != before || e.Phys != 0 || e.Channel != 0 || e.SHA256 != "" || m.SBDMA8Stats().SilenceCompletions != 1 {
				t.Errorf("80h：不應改 DMA 暫存器，Phys、Channel 為 0：%+v", e)
			}
		} else {
			sum := sha256.Sum256(m.Mem[0x10040:0x1004A])
			e := m.SBDMALog[0]
			if m.SBDMA8Stats().DMA8Completions != 1 || m.sb8.dma.Current[2] != 0x4A || e.Phys != 0x10040 ||
				e.SHA256 != hex.EncodeToString(sum[:]) || m.SBDMALog[1].SHA256 != e.SHA256 {
				t.Errorf("%02Xh：DMA 完成計數、位址或實體位址（含頁）的 SHA256 不符：%04X %+v", x.cmd, m.sb8.dma.Current[2], e)
			}
		}
		// 傳輸中再送啟動命令失敗即關閉。
		m = newSB8(t, sbIdle, nil)
		sbProgDMA(m, 0, 0x40, 0x100)
		sbDSP(m, 0x40, 0xA6, x.cmd, 9, 0, 0x80, 1, 0)
		if m.sb8Err == nil || !strings.Contains(m.sb8Err.Error(), "傳輸進行中") {
			t.Errorf("%02Xh：傳輸中再送啟動命令應失敗即關閉：%v", x.cmd, m.sb8Err)
		}
	}
}

// ---- 第 5 項：V86 ------------------------------------------------------------

// deliverProgSB8 是 deliverProg 加上 Sound Blaster 與 444 選項（必須在 LoadCOM 之前開啟）。
func deliverProgSB8(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := newPM(t)
	m.SetSoundBlaster(true)
	mustNil(t, "SetSBDMA8", m.SetSBDMA8(true))
	mustNil(t, "LoadCOM", m.LoadCOM(code))
	m.IRQ0Every = 0
	installDeliverEnv(t, m)
	return m
}

func TestSB8V86Deliver(t *testing.T) {
	m := deliverProgSB8(t, []byte{0x90, 0x90})
	setIVT(m, 0x0F, 0x1234, 0) // 不是 StubSeg
	copy(m.Mem[segCode+uint32(gateOff(0x0F)):], []byte{0xB0, 0x20, 0xE6, 0x20, 0x66, 0xCF})
	m.picMask = 0
	m.sb8.irr7, m.soundBlaster.IRQPending = true, true
	b := snap(m)
	mustStep(t, m, "IRQ7")
	wantEntered(t, m, "IRQ7", 0x0F, false, 0, 0x100, b, gateOff(0x0F)+2, true)
	if !m.sb8.isr7 || m.sb8.irr7 || m.SBDMA8Stats().IRQ7Deliveries != 1 || logKinds(m) != "irq7" || m.SBDMALog[0].CPUMode != "v86" {
		t.Errorf("V86 遞送：ISR7=%t IRR7=%t 紀錄 %s", m.sb8.isr7, m.sb8.irr7, logKinds(m))
	}
	mustStep(t, m, "out 20h,20h")
	mustStep(t, m, "iretd")
	if m.sb8.isr7 || m.CPU.Segmented() || !m.VM() || m.CPU.Seg[cpu.CS] != b.seg[cpu.CS] || m.CPU.IP != b.ip {
		t.Errorf("EOI 後 iretd 回 V86：ISR7=%t segmented=%t CS:IP=%04X:%04X", m.sb8.isr7, m.CPU.Segmented(), m.CPU.Seg[cpu.CS], m.CPU.IP)
	}
	if logKinds(m) != "irq7,eoi" || m.SBDMALog[1].CPUMode != "seg" {
		t.Errorf("紀錄應為 irq7,eoi（eoi 在分段模式）：%s", logKinds(m))
	}
}

// ---- 第 6 項：BIOS 預設處理 ---------------------------------------------------

func TestSB8BIOSDefault(t *testing.T) {
	check := func(name string, m *Machine, cs, ip uint16) {
		t.Helper()
		st := m.SBDMA8Stats()
		if st.IRR7 || st.ISR7 || st.IRQ7Defaulted != 1 || st.IRQ7Deliveries != 0 || !strings.Contains(logKinds(m), "defaulted") ||
			strings.Contains(logKinds(m), "irq7") {
			t.Errorf("%s：應由 BIOS 預設處理消耗：%+v 紀錄 %s", name, st, logKinds(m))
		}
		if m.CPU.Seg[cpu.CS] != cs || m.CPU.IP != ip {
			t.Errorf("%s：不應遞送：CS:IP=%04X:%04X", name, m.CPU.Seg[cpu.CS], m.CPU.IP)
		}
	}
	m := newSB8(t, []byte{0x90, 0x90}, nil)
	m.sb8.irr7 = true
	mustStep(t, m, "實模式")
	check("實模式", m, PSPSeg, 0x101)

	m = deliverProgSB8(t, []byte{0x90, 0x90})
	m.picMask = 0
	m.sb8.irr7 = true
	cs := m.CPU.Seg[cpu.CS]
	mustStep(t, m, "V86")
	check("V86", m, cs, 0x101)
	if m.CPU.Segmented() {
		t.Error("V86：不應進 ring 0")
	}

	// §1.2 的 04B6 形狀：pushf; cli; 還原 IMR 為放行；IVT[0Fh] 改回 StubSeg；popf。到期落在 cli 之後。
	var a sbAsm
	a.op(0xBA, 0x2C, 0x02)
	a.op(0xB0, 0x40)
	a.op(0xEE)
	a.op(0xB0, 0xA6)
	a.op(0xEE)
	a.op(0xB0, 0x14)
	a.op(0xEE)
	a.op(0xB0, 0x03)
	a.op(0xEE)
	a.op(0xB0, 0x00)
	s := a.op(0xEE)
	a.op(0x9C) // pushf
	cli := a.op(0xFA)
	a.op(0xB9, 0x88, 0x13) // mov cx,5000
	a.op(0xE2, 0xFE)       // loop $
	a.op(0xE4, 0x21)
	a.op(0x0C, 0x80)
	a.op(0x24, 0x00)
	a.op(0xE6, 0x21)
	a.op(0x31, 0xC0)
	a.op(0x8E, 0xC0)
	a.op(0x26, 0xC7, 0x06, 0x3C, 0x00, 0x3C, 0x00)
	a.op(0x26, 0xC7, 0x06, 0x3E, 0x00, 0x80, 0x00)
	a.op(0x9D) // popf
	a.op(0x90)
	a.op(0xEB, 0xFE)
	code := a.code
	for uint16(len(code))+0x100 < sbHOff {
		code = append(code, 0x90)
	}
	code = append(code, 0xC6, 0x06, byte(sbFlagOff&0xFF), byte(sbFlagOff>>8), 0x01, 0xCF)
	m = newSB8(t, code, nil)
	setIVT(m, 0x0F, PSPSeg, sbHOff)
	sbProgDMA(m, 0, 0, 3)
	runSteps(t, m, 5100+40, "04B6")
	if uint64(cli)+5000 < uint64(s)+4170 || len(m.SBDMALog) < 2 || m.SBDMALog[1].Kind != "complete" || m.SBDMALog[1].Step < uint64(cli) {
		t.Fatalf("到期應落在 cli 之後：%+v", m.SBDMALog)
	}
	if m.Mem[uint32(PSPSeg)*16+sbFlagOff] != 0 {
		t.Error("04B6：新處理常式不應執行")
	}
	st := m.SBDMA8Stats()
	if logKinds(m) != "start,complete,defaulted" || st.IRQ7Defaulted != 1 || st.IRR7 || st.ISR7 {
		t.Errorf("04B6：結果應為 defaulted：%s %+v", logKinds(m), st)
	}
}

// ---- 第 7 項：分段模式 --------------------------------------------------------

// sbRing0 讓 V86 程式 int 21h 經 DPL 3 的閘 21h 進 ring 0（處理常式放 code0），進去後設 IF=1。
func sbRing0(t *testing.T, code0 []byte) *Machine {
	t.Helper()
	m := deliverProgSB8(t, []byte{0xCD, 0x21, 0x90, 0x90})
	m.picMask = 0
	setIVT(m, 0x0F, 0x1234, 0)
	putGate(m, 0x21, 0x18, uint32(gateOff(0x21)), 0xEE00) // DPL 3：V86 的 int 21h 直接進閘 21h（不轉 #GP）
	copy(m.Mem[segCode+uint32(gateOff(0x21)):], code0)
	mustStep(t, m, "進 ring 0")
	if !m.CPU.Segmented() {
		t.Fatal("應在分段模式")
	}
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	return m
}

func TestSB8Segmented(t *testing.T) {
	m := sbRing0(t, []byte{0x90, 0x90, 0x90, 0x90, 0x66, 0xCF})
	m.sb8.irr7 = true
	for i := 0; i < 3; i++ {
		mustStep(t, m, "ring 0")
	}
	if !m.sb8.irr7 || m.sb8.isr7 || m.SBDMA8Stats().IRQ7Deliveries != 0 || m.segWaitN != 3 {
		t.Errorf("分段模式：不遞送、不消耗、segWait 計 3：IRR7=%t ISR7=%t segWaitN=%d", m.sb8.irr7, m.sb8.isr7, m.segWaitN)
	}
	// IF=0 的一步讓「連續」計數歸零（434 §5.3）。
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	mustStep(t, m, "ring 0（IF=0）")
	if m.segWaitN != 0 || !m.sb8.irr7 {
		t.Errorf("IF=0 時 segWait 應歸零：segWaitN=%d", m.segWaitN)
	}
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	mustStep(t, m, "iretd 回 V86")
	if m.CPU.Segmented() {
		t.Fatalf("應回 V86：CS:IP=%04X:%04X（gate 21h 在 %04X）", m.CPU.Seg[cpu.CS], m.CPU.IP, gateOff(0x21))
	}
	mustStep(t, m, "回 V86 後遞送")
	if !m.sb8.isr7 || m.SBDMA8Stats().IRQ7Deliveries != 1 || m.CPU.Seg[cpu.CS] != 0x18 || m.CPU.IP != gateOff(0x0F)+1 {
		t.Errorf("回 V86 後應遞送：ISR7=%t CS:IP=%04X:%04X", m.sb8.isr7, m.CPU.Seg[cpu.CS], m.CPU.IP)
	}

	// 連續一百萬步失敗即關閉，錯誤訊息列出 IRQ7。
	m = sbRing0(t, []byte{0xEB, 0xFE})
	m.sb8.irr7 = true
	var err error
	for i := 0; i < segWaitLimit+2 && err == nil; i++ {
		err = m.Step()
	}
	if err == nil || !strings.Contains(err.Error(), "IRQ7") || !strings.Contains(err.Error(), "IRR7=true") {
		t.Errorf("分段模式下只有 IRQ7 待處理，一百萬步應失敗即關閉並列出 IRQ7：%v", err)
	}

	// 傳輸在分段模式期間到期時照樣完成。
	m = sbRing0(t, []byte{0xEB, 0xFE})
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	sbDSP(m, 0x40, 0xA6, 0x80, 3, 0)
	delta := bigDelta([]uint64{4, 90, 165000, 315000000, 100}, []uint64{1000000, 264, 17000, 100})
	runSteps(t, m, int(delta)+1, "分段模式等到期")
	if m.SBDMA8Stats().SilenceCompletions != 1 || !m.sb8.irr7 || m.SBDMALog[1].CPUMode != "seg" {
		t.Errorf("分段模式期間到期應照樣完成：%+v %s", m.SBDMA8Stats(), logKinds(m))
	}
}

// ---- 第 8 項：遮罩、ISR7、22Eh、IRQPending ------------------------------------

// sbIRETProg 是 jmp $ 加上 sbHOff 的 iret 處理常式。
func sbIRETProg(t *testing.T) *Machine {
	t.Helper()
	code := append([]byte{}, sbIdle...)
	for uint16(len(code))+0x100 < sbHOff {
		code = append(code, 0x90)
	}
	code = append(code, 0xCF)
	m := newSB8(t, code, nil)
	setIVT(m, 0x0F, PSPSeg, sbHOff)
	return m
}

func TestSB8MaskAndISR(t *testing.T) {
	m := sbIRETProg(t)
	m.picMask = 0x80
	m.sb8.irr7 = true
	runSteps(t, m, 2, "遮罩中")
	if !m.sb8.irr7 || m.sb8.irq7Deliveries != 0 {
		t.Error("21h bit7 設起：不遞送、保留 IRR7")
	}
	m.Out8(0x21, 0)
	runSteps(t, m, 1, "放行")
	if m.sb8.irr7 || !m.sb8.isr7 || m.sb8.irq7Deliveries != 1 {
		t.Error("清除遮罩後下一步應遞送")
	}

	m = sbIRETProg(t)
	m.sb8.irr7, m.sb8.isr7 = true, true
	runSteps(t, m, 2, "ISR7 中")
	if !m.sb8.irr7 || m.sb8.irq7Deliveries != 0 {
		t.Error("ISR7 為 1：不遞送")
	}
	m.Out8(0x20, 0x20)
	runSteps(t, m, 1, "EOI 後")
	if m.sb8.irr7 || m.sb8.irq7Deliveries != 1 {
		t.Error("EOI 後應遞送")
	}

	m = sbIRETProg(t)
	m.sb8.irr7 = true
	m.In8(0x22E)
	runSteps(t, m, 2, "22Eh 之後")
	if m.sb8.irr7 || m.sb8.irq7Deliveries != 0 {
		t.Error("22Eh 讀取在遞送前清 IRR7：不遞送")
	}

	// IRQPending 已為真時再次完成不重拉 IRR7。
	m = sbIRETProg(t)
	sbDSP(m, 0x40, 0xA6, 0x80, 0, 0)
	runSteps(t, m, 1100, "第一次靜音")
	if m.sb8.irq7Deliveries != 1 || !m.soundBlaster.IRQPending || !m.sb8.isr7 {
		t.Fatalf("第一次靜音應遞送一次：%+v", m.SBDMA8Stats())
	}
	m.Out8(0x20, 0x20)
	sbDSP(m, 0x80, 0, 0)
	runSteps(t, m, 1100, "第二次靜音")
	if m.sb8.silenceCompletions != 2 || m.sb8.irr7 || m.sb8.irq7Deliveries != 1 {
		t.Errorf("IRQPending 已真：第二次完成不應重拉 IRR7：%+v", m.SBDMA8Stats())
	}
}

// ---- 第 9 項：優先序與單一事件 ------------------------------------------------

func TestSB8PriorityAndSingleEvent(t *testing.T) {
	code := []byte{0xEB, 0xFE}
	for uint16(len(code))+0x100 < sbHOff {
		code = append(code, 0x90)
	}
	code = append(code, 0x90, 0xCF, 0x90, 0xCF) // sbHOff：IRQ7；sbHOff+2：IRQ0
	m := newSB8(t, code, nil)
	setIVT(m, 0x0F, PSPSeg, sbHOff)
	setIVT(m, 0x08, PSPSeg, sbHOff+2)
	m.irq0Pending = true
	m.sb8.irr7 = true
	mustStep(t, m, "IRQ0 與 IRQ7 同一步")
	if m.CPU.IP != sbHOff+3 || m.sb8.irq7Deliveries != 0 || !m.sb8.irr7 {
		t.Errorf("同一步先送 IRQ0：IP=%04X IRQ7 遞送 %d", m.CPU.IP, m.sb8.irq7Deliveries)
	}

	m = newSB8(t, code, nil)
	setIVT(m, 0x0F, PSPSeg, sbHOff)
	m.QueueCallback(QueuedCall{Seg: PSPSeg, Off: 0x100})
	m.sb8.irr7 = true
	mustStep(t, m, "回呼")
	if m.CallbacksMade() != 1 || m.sb8.irq7Deliveries != 0 || !m.sb8.irr7 {
		t.Errorf("同一步有回呼啟動：不送 IRQ7（made=%d、遞送 %d）", m.CallbacksMade(), m.sb8.irq7Deliveries)
	}

	m = newSB8(t, code, nil)
	setIVT(m, 0x0F, PSPSeg, sbHOff)
	m.SetPeriodicFarCall(PSPSeg, 0x100, 1)
	mustStep(t, m, "periodic 前")
	m.sb8.irr7 = true
	mustStep(t, m, "periodic")
	if m.PeriodicCalls() != 1 || m.sb8.irq7Deliveries != 0 || !m.sb8.irr7 {
		t.Errorf("同一步有週期遠呼叫：不送 IRQ7（calls=%d、遞送 %d）", m.PeriodicCalls(), m.sb8.irq7Deliveries)
	}
}

// ---- 第 10 項：PIC 埠 ---------------------------------------------------------

func TestSB8PICPorts(t *testing.T) {
	m := newSB8(t, sbIdle, nil)
	m.sb8.irr7 = true
	m.irq0Pending = true
	if v := m.In8(0x20); v != 0x81 {
		t.Errorf("20h 初值讀 IRR：%02X", v)
	}
	m.Out8(0x20, 0x0B)
	if v := m.In8(0x20); v != 0 {
		t.Errorf("OCW3 選 ISR 後讀 20h：%02X", v)
	}
	m.sb8.isr7 = true
	if v := m.In8(0x20); v != 0x80 {
		t.Errorf("ISR7 為 1 時讀 20h：%02X", v)
	}
	m.Out8(0x20, 0x0A)
	if v := m.In8(0x20); v != 0x81 {
		t.Errorf("OCW3 選回 IRR：%02X", v)
	}
	for _, v := range []uint8{0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x00, 0x40} {
		m.Out8(0x20, v)
		if !m.sb8.isr7 {
			t.Errorf("20h←%02Xh 不應清 ISR7", v)
		}
	}
	m.Out8(0x20, 0x67)
	if m.sb8.isr7 || m.sb8Err != nil {
		t.Errorf("67h 應清 ISR7：%t %v", m.sb8.isr7, m.sb8Err)
	}
	m.Out8(0xA1, 0x5A)
	if v := m.In8(0xA1); v != 0x5A {
		t.Errorf("A1h 應讀回寫入值：%02X", v)
	}
	if v := m.In8(0xA0); v != 0 {
		t.Errorf("A0h 讀回 0：%02X", v)
	}
	m.Out8(0xA0, 0x20)
	m.Out8(0xA0, 0x0B)
	if m.sb8Err != nil || !m.sb8.readISR[1] {
		t.Errorf("A0h 的 EOI、OCW3 應接受：%v", m.sb8Err)
	}
	for _, x := range []struct {
		port uint16
		v    uint8
		want string
	}{{0x20, 0x11, "ICW1"}, {0x20, 0xA0, "旋轉"}, {0x20, 0xC7, "優先序"}, {0x20, 0x0C, "poll"}, {0x20, 0x68, "特殊遮罩"},
		{0xA0, 0x11, "ICW1"}, {0xA0, 0x80, "旋轉"}} {
		m := newSB8(t, sbIdle, nil)
		m.Out8(x.port, x.v)
		if m.sb8Err == nil || !strings.Contains(m.sb8Err.Error(), x.want) {
			t.Errorf("%02Xh←%02Xh 應失敗即關閉（%s）：%v", x.port, x.v, x.want, m.sb8Err)
		}
	}
}

// ---- 第 11 項：DMA 與 DSP 的失敗即關閉 ----------------------------------------

// sbOutProg：0100 out dx,al；0101 jmp $。
var sbOutProg = []byte{0xEE, 0xEB, 0xFE}

// sbFailAt 讓 0100h 的 out dx,al 寫 port←v，檢查錯誤在那一步回傳、之後黏著且不執行指令、Restore 後清除。
func sbFailAt(t *testing.T, name string, m *Machine, port uint16, v uint8, contains string) {
	t.Helper()
	sn := m.Snapshot()
	m.CPU.IP, m.CPU.R[cpu.DX], m.CPU.R[cpu.AX] = 0x100, port, uint16(v)
	n := m.Steps
	err := m.Step()
	if err == nil || !strings.Contains(err.Error(), contains) {
		t.Errorf("%s：寫 %03Xh←%02Xh 那一步應回錯誤（含 %q）：%v", name, port, v, contains, err)
		return
	}
	if m.Steps != n+1 || len(m.PortLog) == 0 || m.PortLog[len(m.PortLog)-1].Port != port {
		t.Errorf("%s：發生錯誤的 out 應已執行完", name)
	}
	ip, steps := m.CPU.IP, m.Steps
	for i := 0; i < 3; i++ {
		if e := m.Step(); e != err {
			t.Errorf("%s：之後每一步應回同一錯誤：%v", name, e)
		}
	}
	if m.CPU.IP != ip || m.Steps != steps {
		t.Errorf("%s：錯誤之後不應執行指令", name)
	}
	m.Restore(sn)
	if m.SBDMA8Err() != nil {
		t.Errorf("%s：Restore 後應清除", name)
	}
	m.CPU.IP = 0x101
	mustStep(t, m, name+"：Restore 後")
}

func TestSB8FailClosed(t *testing.T) {
	ready := func(mode uint8, prep func(m *Machine)) *Machine {
		m := newSB8(t, sbOutProg, nil)
		sbProgDMA(m, 0, 0x40, 3)
		m.Out8(0x0B, mode)
		if prep != nil {
			prep(m)
		}
		sbDSP(m, 0x40, 0xA6, 0x14, 3)
		return m
	}
	sbFailAt(t, "模式 58h", ready(0x59, nil), 0x22C, 0, "模式 58h")
	sbFailAt(t, "模式 68h", ready(0x69, nil), 0x22C, 0, "模式 68h")
	sbFailAt(t, "通道遮罩中", ready(0x49, func(m *Machine) { m.Out8(0x0A, 5) }), 0x22C, 0, "被遮罩")
	sbFailAt(t, "n > 計數+1", ready(0x49, func(m *Machine) { m.Out8(0x0C, 0); m.Out8(0x02, 0); m.Out8(0x02, 0); m.Out8(0x03, 2); m.Out8(0x03, 0) }),
		0x22C, 0, "大於")
	for _, x := range []struct {
		name string
		skip uint16
	}{{"位址未寫", 0x02}, {"頁未寫", 0x83}} {
		m := newSB8(t, sbOutProg, nil)
		for _, w := range [][2]uint16{{0x83, 0}, {0x0C, 0}, {0x02, 0}, {0x02, 0}, {0x03, 3}, {0x03, 0}, {0x0B, 0x49}, {0x0A, 1}} {
			if w[0] != x.skip {
				m.Out8(w[0], uint8(w[1]))
			}
		}
		sbDSP(m, 0x40, 0xA6, 0x14, 3)
		sbFailAt(t, x.name, m, 0x22C, 0, "未寫齊")
	}
	for _, x := range []struct {
		page uint8
		addr uint16
	}{{0x0A, 0}, {0x0B, 0xFFFE}, {0x09, 0xFFFE}} {
		m := newSB8(t, sbOutProg, nil)
		sbProgDMA(m, x.page, x.addr, 3)
		sbDSP(m, 0x40, 0xA6, 0x14, 3)
		if x.page == 0x09 { // 09FFFEh 起 4 bytes 環繞在同一頁：09FFFE、09FFFF、090000、090001，不碰 A0000h
			m.CPU.IP, m.CPU.R[cpu.DX], m.CPU.R[cpu.AX] = 0x100, 0x22C, 0
			mustStep(t, m, "頁內環繞")
			continue
		}
		sbFailAt(t, "來源碰 A0000h–BFFFFh", m, 0x22C, 0, "A0000h")
	}
	inXfer := func() *Machine {
		m := newSB8(t, sbOutProg, nil)
		sbProgDMA(m, 0, 0x40, 3)
		sbDSP(m, 0x40, 0xA6, 0x14, 3, 0)
		if !m.sb8.active {
			t.Fatal("傳輸應進行中")
		}
		return m
	}
	for _, x := range []struct {
		port uint16
		v    uint8
		want string
	}{{0x0A, 0x05, "遮罩通道 1"}, {0x0F, 0x02, "遮罩通道 1"}, {0x0D, 0, "遮罩通道 1"},
		{0x02, 0, "位址"}, {0x03, 0, "計數"}, {0x83, 1, "頁暫存器"}, {0x0B, 0x49, "模式"}} {
		sbFailAt(t, "傳輸中", inXfer(), x.port, x.v, x.want)
	}
	sbFailAt(t, "08h 非 0", newSB8(t, sbOutProg, nil), 0x08, 0x04, "命令／請求")
	sbFailAt(t, "09h 非 0", newSB8(t, sbOutProg, nil), 0x09, 0x05, "命令／請求")
	m := newSB8(t, sbOutProg, nil)
	m.Out8(0x08, 0)
	m.Out8(0x09, 0)
	if m.sb8Err != nil {
		t.Errorf("08h、09h 寫 0 應接受：%v", m.sb8Err)
	}
	for _, x := range []struct {
		pre []uint8
		v   uint8
	}{{nil, 0x1C}, {[]uint8{0xC6, 0x00, 0x03}, 0x00}, {nil, 0x7D}, {nil, 0xD0}, {nil, 0xD4}, {nil, 0x7F}, {nil, 0x1F}} {
		m := newSB8(t, sbOutProg, nil)
		sbDSP(m, x.pre...)
		sbFailAt(t, "DSP", m, 0x22C, x.v, "未被接受")
	}
	// Δ 的商超過 2^63（極端 IRQ0Pinned）。
	for _, every := range []uint64{math.MaxUint64, 1 << 63} {
		m = newSB8(t, sbOutProg, nil)
		m.IRQ0Pinned, m.IRQ0Every = true, every
		m.Out8(0x43, 0x36)
		m.Out8(0x40, 1)
		m.Out8(0x40, 0)
		sbDSP(m, 0x40, 0xA6, 0x80, 3)
		sbFailAt(t, "Δ 溢位", m, 0x22C, 0, "溢位")
	}
	// 商落在 (2^63, 2^64)：週期時鐘、CPUHz 2e17、TC 00h（D=256）、16h（4 倍）、n=65536。
	m = newSB8(t, sbOutProg, func(m *Machine) { m.CycleClock, m.CPUHz = true, 200_000_000_000_000_000; m.RecalcIRQ0() })
	sbProgDMA(m, 0, 0, 0xFFFF)
	sbDSP(m, 0x40, 0x00, 0x16, 0xFF)
	if q := bigDelta([]uint64{65536 * 4, 256, 200_000_000_000_000_000}, []uint64{1000000}); q <= 1<<63 || q == 0 {
		t.Fatalf("構造的商應超過 2^63：%d", q)
	}
	sbFailAt(t, "Δ 商超過 2^63", m, 0x22C, 0xFF, "溢位")
	// 傳輸中遮罩通道 0、2、3 或改寫它們的暫存器不報錯；傳輸照常完成。
	m = inXfer()
	for _, w := range [][2]uint16{{0x0A, 4}, {0x0A, 6}, {0x0A, 7}, {0x00, 1}, {0x01, 2}, {0x04, 3}, {0x05, 4}, {0x06, 5}, {0x07, 6},
		{0x87, 1}, {0x81, 2}, {0x82, 3}, {0x0B, 0x48}, {0x0B, 0x4A}, {0x0B, 0x4B}, {0x0F, 0x0D}, {0x0E, 0}} {
		m.Out8(w[0], uint8(w[1]))
	}
	m.CPU.IP = 0x101
	runSteps(t, m, 4200, "其他通道")
	if m.sb8Err != nil || m.SBDMA8Stats().DMA8Completions != 1 {
		t.Errorf("傳輸中動其他通道不應報錯：%v %+v", m.sb8Err, m.SBDMA8Stats())
	}
}

// ---- 第 12 項：取消與進度 -----------------------------------------------------

func TestSB8CancelAndProgress(t *testing.T) {
	m := newSB8(t, sbIdle, nil)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	sbProgDMA(m, 0, 0x1000, 3)
	sbDSP(m, 0x40, 0xA6, 0x14, 3, 0)
	start := m.Steps
	runSteps(t, m, 2085, "傳輸一半")
	k := uint16((m.Steps - start) * 4 / 4170) // 2
	if k != 2 {
		t.Fatalf("進度應為 2：%d", k)
	}
	m.Out8(0x0C, 0)
	if v := m.In8(0x00); v != 0xFF || m.sb8.dma.high || m.SBDMAUnknownReads != 1 {
		t.Errorf("未寫過的 00h 應回 FFh 且不切換 flip-flop：%02X high=%t", v, m.sb8.dma.high)
	}
	lo, hi := m.In8(0x02), m.In8(0x02)
	clo, chi := m.In8(0x03), m.In8(0x03)
	if uint16(lo)|uint16(hi)<<8 != 0x1000+k || uint16(clo)|uint16(chi)<<8 != 3-k {
		t.Errorf("傳輸中讀通道 1 應得進度值：位址 %02X%02X 計數 %02X%02X", hi, lo, chi, clo)
	}
	if m.sb8.dma.Current[2] != 0x1000 || m.sb8.dma.Current[3] != 3 {
		t.Error("即時讀值不應寫回暫存器")
	}
	m.Out8(0x226, 1)
	m.Out8(0x226, 0)
	d := m.sb8.dma
	if d.Current[2] != 0x1000+k || d.Current[3] != 3-k || m.sb8.tc != 0 || m.sb8.irr7 || m.sb8.active || d.Mask&2 != 0 {
		t.Errorf("取消：暫存器依進度更新、不設 TC、不拉 IRR7：%04X %04X tc=%X", d.Current[2], d.Current[3], m.sb8.tc)
	}
	if logKinds(m) != "start,cancel" || m.SBDMALog[1].Transferred != uint32(k) {
		t.Errorf("紀錄應有 cancel（Transferred=%d）：%s %+v", k, logKinds(m), m.SBDMALog)
	}
	runSteps(t, m, 3000, "取消之後")
	if m.sb8.irr7 || m.SBDMA8Stats().DMA8Completions != 0 {
		t.Error("取消之後不應完成")
	}
}

// ---- 第 13 項：到期時刻固定 ---------------------------------------------------

func TestSB8DueFixed(t *testing.T) {
	m := newSB8(t, sbIdle, nil)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	sbProgDMA(m, 0, 0, 3)
	sbDSP(m, 0x40, 0xA6, 0x14, 3, 0)
	due := m.sb8.due
	runSteps(t, m, 10, "")
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 0)
	m.Out8(0x40, 0x10) // 分頻 4096
	if m.IRQ0Every == 636085 {
		t.Fatal("改分頻應改 IRQ0Every（未釘死）")
	}
	m.IRQ0Pinned, m.IRQ0Every = true, 1234
	m.Out8(0x43, 0x36)
	m.Out8(0x40, 0)
	m.Out8(0x40, 0x20) // 釘死之後再改分頻 8192
	if m.sb8.due != due {
		t.Fatalf("到期時刻不應重算：%d → %d", due, m.sb8.due)
	}
	for m.Steps < due {
		runSteps(t, m, 1, "")
		if m.sb8.irr7 {
			t.Fatalf("第 %d 步提早完成", m.Steps)
		}
	}
	runSteps(t, m, 1, "")
	if !m.sb8.irr7 {
		t.Error("原到期時刻應完成")
	}
}

// ---- 第 14 項：狀態檔、快照、觀測紀錄 -----------------------------------------

func sbSave(t *testing.T, m *Machine) []byte {
	t.Helper()
	var b bytes.Buffer
	mustNil(t, "SaveState", m.SaveState(&b))
	return b.Bytes()
}

func TestSB8StateRoundTrip(t *testing.T) {
	code, s := sbDetectProg()
	mk := func() *Machine {
		m := newSB8(t, code, nil)
		setIVT(m, 0x0F, PSPSeg, sbHOff)
		return m
	}
	const total = 39 + 4170 + 11 + 100
	ref := mk()
	runSteps(t, ref, total, "不中斷")
	if ref.Mem[uint32(PSPSeg)*16+sbFlagOff] != 1 || ref.sb8.isr7 {
		t.Fatal("不中斷的執行應跑過處理常式")
	}
	want := sbSave(t, ref)

	// SaveState／LoadState 在 start 與到期之間。
	m := mk()
	runSteps(t, m, s+100, "")
	saved := sbSave(t, m)
	var st machineState
	mustNil(t, "decode", gob.NewDecoder(bytes.NewReader(saved)).Decode(&st))
	if st.Version != 4 {
		t.Errorf("選項開啟應寫版本 4：%d", st.Version)
	}
	n := New()
	n.SetSoundBlaster(true)
	mustNil(t, "SetSBDMA8", n.SetSBDMA8(true))
	mustNil(t, "LoadState", n.LoadState(bytes.NewReader(saved)))
	if !n.sb8.active || n.LegacyState {
		t.Fatal("LoadState 後傳輸應進行中")
	}
	runSteps(t, n, total-(s+100), "LoadState 之後")
	if got := sbSave(t, n); !bytes.Equal(got, want) {
		t.Error("SaveState／LoadState 之後的最終狀態應與不中斷的執行相同")
	}

	// Snapshot／Restore 在 start 與到期之間（先多跑一段再倒回）。
	m = mk()
	runSteps(t, m, s+100, "")
	sn := m.Snapshot()
	runSteps(t, m, 4300, "多跑")
	if len(m.SBDMALog) == 0 {
		t.Fatal("多跑一段後應有紀錄")
	}
	m.Restore(sn)
	if len(m.SBDMALog) != 0 || m.SBDMALogDropped != 0 {
		t.Error("Restore 後 SBDMALog 應為空")
	}
	runSteps(t, m, total-(s+100), "Restore 之後")
	if got := sbSave(t, m); !bytes.Equal(got, want) {
		t.Error("Snapshot／Restore 之後的最終狀態應與不中斷的執行相同")
	}

	// 選項關閉寫版本 3。
	off := New()
	mustNil(t, "LoadCOM", off.LoadCOM(sbIdle))
	offState := sbSave(t, off)
	mustNil(t, "decode", gob.NewDecoder(bytes.NewReader(offState)).Decode(&st))
	if st.Version != 3 {
		t.Errorf("選項關閉應寫版本 3：%d", st.Version)
	}

	// 選項與版本不一致：兩個方向都拒絕，機器不變。
	on := mk()
	runSteps(t, on, 50, "")
	before := sbSave(t, on)
	if err := on.LoadState(bytes.NewReader(offState)); err == nil || !bytes.Equal(sbSave(t, on), before) {
		t.Errorf("版本 3 載入開啟選項的機器應拒絕且機器不變：%v", err)
	}
	off2 := New()
	mustNil(t, "LoadCOM", off2.LoadCOM(sbIdle))
	before = sbSave(t, off2)
	if err := off2.LoadState(bytes.NewReader(saved)); err == nil || !bytes.Equal(sbSave(t, off2), before) {
		t.Errorf("版本 4 載入關閉選項的機器應拒絕且機器不變：%v", err)
	}

	// sbDMA8State.V 不認得時拒絕。
	dec := gob.NewDecoder(bytes.NewReader(saved))
	var ms machineState
	var sb sbDMA8State
	mustNil(t, "decode", dec.Decode(&ms))
	mustNil(t, "decode sb", dec.Decode(&sb))
	sb.V = 2
	var bad bytes.Buffer
	enc := gob.NewEncoder(&bad)
	mustNil(t, "encode", enc.Encode(&ms))
	mustNil(t, "encode sb", enc.Encode(&sb))
	before = sbSave(t, on)
	if err := on.LoadState(bytes.NewReader(bad.Bytes())); err == nil || !strings.Contains(err.Error(), "版本 2") ||
		!bytes.Equal(sbSave(t, on), before) {
		t.Errorf("V 不認得應拒絕且機器不變：%v", err)
	}

	// Restore 開關不一致時 panic 且機器不變。
	before = sbSave(t, on)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("Restore 開關不一致應 panic")
			}
		}()
		on.Restore(off.Snapshot())
	}()
	if !bytes.Equal(sbSave(t, on), before) {
		t.Error("Restore panic 後機器應不變")
	}

	// StateDigest 讀版本 4 並能偵測 IRR7 的改變。
	d1, err := StateDigest(bytes.NewReader(saved))
	mustNil(t, "StateDigest", err)
	n.sb8.irr7 = !n.sb8.irr7
	a := sbSave(t, n)
	n.sb8.irr7 = !n.sb8.irr7
	b := sbSave(t, n)
	da, err := StateDigest(bytes.NewReader(a))
	mustNil(t, "StateDigest", err)
	db, err := StateDigest(bytes.NewReader(b))
	mustNil(t, "StateDigest", err)
	if da == db || d1 == ([32]byte{}) {
		t.Error("StateDigest 應偵測 IRR7 的改變")
	}
}

func TestSB8LogCap(t *testing.T) {
	m := newSB8(t, sbIdle, nil)
	for i := 0; i < 5000; i++ {
		m.sb8Log(SBDMAEvent{Kind: "eoi", Step: uint64(i)})
	}
	if len(m.SBDMALog) != 4096 || m.SBDMALogDropped != 904 || m.SBDMALog[0].Step != 904 || m.SBDMALog[4095].Step != 4999 {
		t.Errorf("紀錄超過 4096 筆時保留最新 4096 筆：len=%d dropped=%d 首筆 %d", len(m.SBDMALog), m.SBDMALogDropped, m.SBDMALog[0].Step)
	}
	var got []SBDMAEvent
	m.OnSBDMA = func(e SBDMAEvent) { got = append(got, e) }
	m.sb8Log(SBDMAEvent{Kind: "eoi", Step: 1})
	if len(got) != 1 || len(m.SBDMALog) != 4096 {
		t.Error("OnSBDMA 非 nil 時每筆呼叫（不追加，與 onOPL 同類）")
	}
	if n := len(SBDMAEventFields); n != 27 || len((SBDMAEvent{}).Fields()) != n {
		t.Errorf("欄名與欄值應同為 27 欄：%d %d", n, len((SBDMAEvent{}).Fields()))
	}
}

// ---- 第 15 項：LE 不變 --------------------------------------------------------

func TestSB8LEUnchanged(t *testing.T) {
	for _, v := range []uint8{0x74, 0x75, 0x76, 0x77, 0x16, 0x17, 0x80} {
		p := NewLEOPLPorts()
		if p.Out8(0x22c, v) || p.dsp.pending != 0 || p.dsp.lengthHighNext {
			t.Errorf("LEOPLPorts 收到 %02Xh 應與改動前一樣回 false 且不設 pending", v)
		}
	}
}

// ---- 445 §3：DSP D0h／D4h 的暫停與恢復 ---------------------------------------------
//
// docs/spec/445-legacy-sb16-dma8-pause-resume §3 第 2 項（machine 層）。步號在測試內寫死；期望的進度以 ⌊經過·n/Δ⌋
// 在測試內獨立計算，不呼叫 sb8Progress。D0h、D4h 與長度高位元組都以真正的 out 指令執行（sbOutStep），序號即該步遞增後的 m.Steps。

// sbCtlMachine 造一台 0100 out dx,al；0101 jmp $ 的機器，IVT[0Fh] 指向 sbHOff 的處理常式
// （push ax／dx、讀 22Eh、out 20h,20h、pop、iret；不改 DX、AX）。CPU 停在 0101。
func sbCtlMachine(t *testing.T, prep func(m *Machine)) *Machine {
	t.Helper()
	code := append([]byte{}, sbOutProg...)
	for uint16(len(code))+0x100 < sbHOff {
		code = append(code, 0x90)
	}
	code = append(code, 0x50, 0x52, 0xBA, 0x2E, 0x02, 0xEC, 0xB0, 0x20, 0xE6, 0x20, 0x5A, 0x58, 0xCF)
	m := newSB8(t, code, prep)
	setIVT(m, 0x0F, PSPSeg, sbHOff)
	m.CPU.IP = 0x101
	return m
}

// sbOutStep 讓 0100h 的 out dx,al 以一步寫 port←v，回這道的序號（遞增後的 m.Steps）。執行後 CPU 停在 0101（jmp $）。
func sbOutStep(t *testing.T, m *Machine, port uint16, v uint8) uint64 {
	t.Helper()
	m.CPU.IP, m.CPU.R[cpu.DX], m.CPU.R[cpu.AX] = 0x100, port, uint16(v)
	mustStep(t, m, "out")
	if w := m.PortLog[len(m.PortLog)-1]; w.Port != port || w.Val != v || w.Step != m.Steps {
		t.Fatalf("第 %d 步應寫 %03Xh←%02Xh：%+v", m.Steps, port, v, w)
	}
	return m.Steps
}

// sbRunTo 執行到 m.Steps == n。
func sbRunTo(t *testing.T, m *Machine, n uint64) {
	t.Helper()
	if m.Steps > n {
		t.Fatalf("已過第 %d 步（目前 %d）", n, m.Steps)
	}
	runSteps(t, m, int(n-m.Steps), "執行")
}

// sbStart 以宿主程式 DMA（頁 0）與前兩個 DSP 位元組，再以 out 指令寫長度高位元組。回長度高位元組那一道的序號。
func sbStart(t *testing.T, m *Machine, cmd uint8, addr, count uint16, n uint16) uint64 {
	t.Helper()
	sbProgDMA(m, 0, addr, count)
	sbDSP(m, 0x40, 0xA6, cmd, uint8(n-1))
	s := sbOutStep(t, m, 0x22C, uint8((n-1)>>8))
	if !m.sb8.active || m.sb8Err != nil {
		t.Fatalf("傳輸應開始：%v", m.sb8Err)
	}
	return s
}

// sbReadCh1 清 flip-flop 後讀通道 1 的位址與計數。
func sbReadCh1(m *Machine) (uint16, uint16) {
	m.Out8(0x0C, 0)
	a := uint16(m.In8(0x02)) | uint16(m.In8(0x02))<<8
	c := uint16(m.In8(0x03)) | uint16(m.In8(0x03))<<8
	return a, c
}

// sbK 是測試內獨立算的進度 ⌊經過·n/Δ⌋（夾在 n）。
func sbK(el, n, delta uint64) uint64 {
	if k := el * n / delta; k < n {
		return k
	}
	return n
}

// sbFind 回第 i 筆種類為 kind 的紀錄（從 0 起）。
func sbFind(t *testing.T, m *Machine, kind string, i int) SBDMAEvent {
	t.Helper()
	for _, e := range m.SBDMALog {
		if e.Kind == kind {
			if i == 0 {
				return e
			}
			i--
		}
	}
	t.Fatalf("找不到第 %d 筆 %s：%s", i, kind, logKinds(m))
	return SBDMAEvent{}
}

// sbRunUntilComplete 一步一步執行，直到 DMA8Completions 或 SilenceCompletions 增加；回完成那一步遞增前的 m.Steps。
func sbRunUntilComplete(t *testing.T, m *Machine, limit int) uint64 {
	t.Helper()
	st := m.SBDMA8Stats()
	for i := 0; i < limit; i++ {
		before := m.Steps
		mustStep(t, m, "等完成")
		if n := m.SBDMA8Stats(); n.DMA8Completions != st.DMA8Completions || n.SilenceCompletions != st.SilenceCompletions {
			return before
		}
	}
	t.Fatalf("%d 步內沒有完成", limit)
	return 0
}

// 第 2.1 項：14h 暫停與恢復。
func TestSB8PauseResume14h(t *testing.T) {
	m := sbCtlMachine(t, nil)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	for i := 0; i < 4; i++ {
		m.Mem[0x1000+i] = byte(0x31 + i)
	}
	const delta = 4170
	s := sbStart(t, m, 0x14, 0x1000, 3, 4)
	if s != 1 || m.sb8.due != 1+delta {
		t.Fatalf("開始應在序號 1、到期 4171：%d %d", s, m.sb8.due)
	}
	sbRunTo(t, m, 2600)
	p := sbOutStep(t, m, 0x22C, 0xD0)
	kp := sbK(p-s, 4, delta)
	if p != 2601 || kp != 2 {
		t.Fatalf("D0h 應在序號 2601、k(p)=2：%d %d", p, kp)
	}
	for i := 0; i < 5000; i++ {
		mustStep(t, m, "暫停中")
		if m.sb8.irr7 || !m.sb8.active || m.SBDMA8Stats().DMA8Completions != 0 {
			t.Fatalf("暫停中第 %d 步不應完成", m.Steps)
		}
		if i%997 == 0 {
			if a, c := sbReadCh1(m); a != 0x1000+uint16(kp) || c != 3-uint16(kp) {
				t.Fatalf("暫停中第 %d 步讀 02h、03h 應固定為 k(p)：%04X %04X", m.Steps, a, c)
			}
		}
	}
	r := sbOutStep(t, m, 0x22C, 0xD4)
	if r != 7602 {
		t.Fatalf("D4h 應在序號 7602：%d", r)
	}
	const due2 = 1 + delta + (7602 - 2601) // 9172
	if m.sb8.due != due2 || m.sb8.start != 1+(7602-2601) {
		t.Errorf("恢復後 start、due 應平移 r−p：%d %d", m.sb8.start, m.sb8.due)
	}
	if a, c := sbReadCh1(m); a != 0x1000+uint16(kp) || c != 3-uint16(kp) {
		t.Errorf("恢復那一刻讀值應等於 k(p)：%04X %04X", a, c)
	}
	sbRunTo(t, m, r+1000)
	km := sbK(r+1000-1-(r-p), 4, delta)
	if km != 3 {
		t.Fatalf("中途進度應為 3：%d", km)
	}
	if a, c := sbReadCh1(m); a != 0x1000+uint16(km) || c != 3-uint16(km) {
		t.Errorf("恢復後中途讀值應為 %d：%04X %04X", km, a, c)
	}
	if got := sbRunUntilComplete(t, m, 2000); got != due2 {
		t.Errorf("完成應在遞增前 m.Steps＝%d 的那一步：%d", due2, got)
	}
	runSteps(t, m, 20, "處理常式")
	if got := logKinds(m); got != "start,pause,resume,complete,irq7,eoi" {
		t.Fatalf("紀錄應依序為 start,pause,resume,complete,irq7,eoi：%s", got)
	}
	ev := m.SBDMALog
	sum := sha256.Sum256(m.Mem[0x1000:0x1004])
	want := hex.EncodeToString(sum[:])
	if ev[0].Due != 1+delta || ev[1].Due != 1+delta || ev[2].Due != due2 || ev[3].Due != due2 {
		t.Errorf("Due：start、pause 為原到期，resume、complete 為平移後：%d %d %d %d", ev[0].Due, ev[1].Due, ev[2].Due, ev[3].Due)
	}
	if ev[1].SHA256 != "" || ev[2].SHA256 != "" || ev[0].SHA256 != want || ev[3].SHA256 != want {
		t.Errorf("SHA256：pause、resume 為空，start、complete 為緩衝區雜湊：%q %q %q %q", ev[0].SHA256, ev[1].SHA256, ev[2].SHA256, ev[3].SHA256)
	}
	if ev[1].Transferred != 2 || ev[2].Transferred != 2 || ev[3].Transferred != 4 {
		t.Errorf("Transferred：pause、resume 為 k(p)=2：%d %d", ev[1].Transferred, ev[2].Transferred)
	}
	if ev[1].Step != p || ev[2].Step != r || ev[1].CS != PSPSeg || ev[1].IP != 0x100 || ev[2].CS != PSPSeg || ev[2].IP != 0x100 ||
		ev[1].Command != 0x14 || ev[1].Format != "pcm8" || ev[1].Addr != 0x1000 || ev[2].Length != 4 {
		t.Errorf("pause、resume 的 Step、CS:IP（本道指令起點）與傳輸欄位：%+v %+v", ev[1], ev[2])
	}
	if ev[3].Step != due2 || ev[4].Step != due2 {
		t.Errorf("complete、irq7 應在第 %d 步：%d %d", due2, ev[3].Step, ev[4].Step)
	}
	if st := m.SBDMA8Stats(); st.Paused || st.Pauses != 1 || st.Resumes != 1 || st.IRQ7Deliveries != 1 {
		t.Errorf("統計：%+v", st)
	}
}

// 第 2.2 項：74h ADPCM（Δ 以 2n 個取樣計，進度以 DMA byte 計）。
func TestSB8PauseResumeADPCM(t *testing.T) {
	m := sbCtlMachine(t, nil)
	const delta = 8339
	if b := bigDelta([]uint64{8, 90, 636085, 315000000}, []uint64{1000000, 264, 65536}); b != delta {
		t.Fatalf("Δ 應為 %d：%d", delta, b)
	}
	s := sbStart(t, m, 0x74, 0x2000, 3, 4)
	if m.sb8.due != s+delta {
		t.Fatalf("到期應為 %d：%d", s+delta, m.sb8.due)
	}
	sbRunTo(t, m, s+4999)
	p := sbOutStep(t, m, 0x22C, 0xD0)
	kp := sbK(p-s, 4, delta) // ⌊5000·4/8339⌋ = 2
	if kp != 2 {
		t.Fatalf("k(p) 應為 2：%d", kp)
	}
	runSteps(t, m, 9000, "暫停中")
	if a, c := sbReadCh1(m); a != 0x2002 || c != 1 || m.sb8.irr7 {
		t.Errorf("暫停中讀值應為 k(p)、不完成：%04X %04X", a, c)
	}
	r := sbOutStep(t, m, 0x22C, 0xD4)
	sbRunTo(t, m, r+2000)
	if a, _ := sbReadCh1(m); uint64(a) != 0x2000+sbK(p-s+2000, 4, delta) {
		t.Errorf("恢復後中途讀值：%04X", a)
	}
	if got := sbRunUntilComplete(t, m, 5000); got != s+delta+(r-p) {
		t.Errorf("完成應在 %d：%d", s+delta+(r-p), got)
	}
	if e := sbFind(t, m, "pause", 0); e.Format != "adpcm4" || e.Samples != 8 || e.Transferred != 2 {
		t.Errorf("pause 紀錄：%+v", e)
	}
}

// 第 2.3 項：兩次暫停，完成時刻為 due + d1 + d2。
func TestSB8PauseTwice(t *testing.T) {
	m := sbCtlMachine(t, nil)
	s := sbStart(t, m, 0x14, 0x1000, 3, 4)
	sbRunTo(t, m, s+999)
	p1 := sbOutStep(t, m, 0x22C, 0xD0)
	runSteps(t, m, 499, "暫停 1")
	r1 := sbOutStep(t, m, 0x22C, 0xD4)
	runSteps(t, m, 999, "恢復 1")
	p2 := sbOutStep(t, m, 0x22C, 0xD0)
	runSteps(t, m, 699, "暫停 2")
	r2 := sbOutStep(t, m, 0x22C, 0xD4)
	if r1-p1 != 500 || r2-p2 != 700 {
		t.Fatalf("d1、d2 應為 500、700：%d %d", r1-p1, r2-p2)
	}
	if got := sbRunUntilComplete(t, m, 6000); got != s+4170+500+700 {
		t.Errorf("完成應在 %d：%d", s+4170+1200, got)
	}
	if st := m.SBDMA8Stats(); st.Pauses != 2 || st.Resumes != 2 {
		t.Errorf("兩次暫停與恢復：%+v", st)
	}
	if e := sbFind(t, m, "resume", 1); e.Due != s+4170+1200 || sbFind(t, m, "complete", 0).Due != e.Due {
		t.Errorf("complete.Due 等於最後一個 resume.Due：%d", e.Due)
	}
}

// 第 2.4 項：第二個 D0h 隔幾步再送，pausedAt 不被覆寫。
func TestSB8PauseIdempotent(t *testing.T) {
	m := sbCtlMachine(t, nil)
	s := sbStart(t, m, 0x14, 0x1000, 3, 4)
	sbRunTo(t, m, s+999)
	p := sbOutStep(t, m, 0x22C, 0xD0)
	runSteps(t, m, 4, "")
	p2 := sbOutStep(t, m, 0x22C, 0xD0)
	if m.sb8.pausedAt != p || p2 != p+5 || m.sb8Err != nil {
		t.Fatalf("第二個 D0h 接受、不覆寫 pausedAt：%d（p=%d）%v", m.sb8.pausedAt, p, m.sb8Err)
	}
	runSteps(t, m, 294, "")
	r := sbOutStep(t, m, 0x22C, 0xD4)
	if r-p != 300 {
		t.Fatalf("r−p 應為 300：%d", r-p)
	}
	if got := sbRunUntilComplete(t, m, 6000); got != s+4170+300 {
		t.Errorf("平移以第一個 D0h 為準，完成應在 %d：%d", s+4170+300, got)
	}
	if st := m.SBDMA8Stats(); st.Pauses != 1 || st.Resumes != 1 || !strings.HasPrefix(logKinds(m), "start,pause,resume,complete") {
		t.Errorf("冪等的 D0h 不計數、不記事件：%+v %s", st, logKinds(m))
	}
}

// 第 2.5 項：未暫停的傳輸中送 D4h、完整暫停與恢復後再送 D4h：接受，不動作。
func TestSB8ResumeIdempotent(t *testing.T) {
	m := sbCtlMachine(t, nil)
	s := sbStart(t, m, 0x14, 0x1000, 3, 4)
	runSteps(t, m, 10, "")
	sbOutStep(t, m, 0x22C, 0xD4)
	if m.sb8Err != nil || m.sb8.start != s || m.sb8.due != s+4170 || m.sb8.paused {
		t.Fatalf("未暫停時 D4h 接受、不動作：%v %d %d", m.sb8Err, m.sb8.start, m.sb8.due)
	}
	p := sbOutStep(t, m, 0x22C, 0xD0)
	runSteps(t, m, 100, "")
	r := sbOutStep(t, m, 0x22C, 0xD4)
	start, due := m.sb8.start, m.sb8.due
	runSteps(t, m, 50, "")
	sbOutStep(t, m, 0x22C, 0xD4)
	if m.sb8Err != nil || m.sb8.start != start || m.sb8.due != due || due != s+4170+(r-p) {
		t.Errorf("恢復後再送 D4h 接受、start、due 不動：%v %d %d", m.sb8Err, m.sb8.start, m.sb8.due)
	}
	if st := m.SBDMA8Stats(); st.Pauses != 1 || st.Resumes != 1 || logKinds(m) != "start,pause,resume" {
		t.Errorf("冪等的 D4h 不計數、不記事件：%+v %s", st, logKinds(m))
	}
}

// 第 2.6 項：步數時鐘下 D0h 落在序號等於 due 的那一道。
func TestSB8PauseAtDue(t *testing.T) {
	m := sbCtlMachine(t, nil)
	s := sbStart(t, m, 0x14, 0x1000, 3, 4)
	due := s + 4170
	sbRunTo(t, m, due-1)
	p := sbOutStep(t, m, 0x22C, 0xD0)
	if p != due || !m.sb8.paused || m.sb8.pausedAt != due {
		t.Fatalf("D0h 應在序號 due=%d 暫停：%d", due, p)
	}
	for i := 0; i < 300; i++ {
		mustStep(t, m, "暫停中")
		if m.SBDMA8Stats().DMA8Completions != 0 {
			t.Fatalf("pausedAt==due：暫停中第 %d 步不應完成", m.Steps)
		}
	}
	if a, c := sbReadCh1(m); uint64(a) != 0x1000+sbK(due-s, 4, 4170) || c != 0xFFFF {
		t.Errorf("暫停中讀值為 n 對應的位址與計數：%04X %04X", a, c)
	}
	r := sbOutStep(t, m, 0x22C, 0xD4)
	if got := sbRunUntilComplete(t, m, 10); got != due+(r-p) || got != r {
		t.Errorf("D4h 後應在 due+d＝%d 完成：%d", due+(r-p), got)
	}
}

// 第 2.7 項：週期時鐘。
func TestSB8PauseCycleClock(t *testing.T) {
	prep := func(m *Machine) { m.CycleClock, m.CPUHz = true, 20_000_000; m.RecalcIRQ0() }
	const delta = 7200 // ⌈4·90·20e6/1e6⌉
	if b := bigDelta([]uint64{4, 90, 20_000_000}, []uint64{1000000}); b != delta {
		t.Fatalf("Δ：%d", b)
	}

	m := sbCtlMachine(t, prep)
	sbStart(t, m, 0x14, 0x1000, 3, 4)
	start := m.PortLog[len(m.PortLog)-1].Cycles
	if m.sb8.start != start || m.sb8.due != start+delta {
		t.Fatalf("週期時鐘的開始應為 out 當下的 Cycles：%d %d", m.sb8.start, start)
	}
	runSteps(t, m, 300, "")
	c0 := m.CPU.Cycles
	sbOutStep(t, m, 0x22C, 0xD0)
	pc := m.PortLog[len(m.PortLog)-1].Cycles
	outCost := pc - c0 // out dx,al 從 Step 開頭到埠處理為止扣的週期
	if m.sb8.pausedAt != pc {
		t.Fatalf("pausedAt 應為 D0h 那一道 out 扣過週期後的 Cycles：%d %d", m.sb8.pausedAt, pc)
	}
	runSteps(t, m, 2000, "暫停中")
	sbOutStep(t, m, 0x22C, 0xD4)
	rc := m.PortLog[len(m.PortLog)-1].Cycles
	due2 := start + delta + (rc - pc)
	// 寫死（獨立推算：out dx,al 扣 cycBase 2＋cycIO 14＝16，jmp $ 扣 2＋5＝7）：開始 16；D0h 16+300·7+16＝2132；
	// D4h 2132+2000·7+16＝16148；平移後到期 16+7200+(16148−2132)＝21232。
	if start != 16 || pc != 2132 || rc != 16148 || due2 != 21232 || m.sb8.due != due2 {
		t.Fatalf("週期時鐘的時刻：開始 %d、D0h %d、D4h %d、平移後到期 %d（實際 %d）", start, pc, rc, due2, m.sb8.due)
	}
	var prev uint64
	for i := 0; i < 5000; i++ {
		before := m.CPU.Cycles
		mustStep(t, m, "")
		if m.SBDMA8Stats().DMA8Completions == 1 {
			if before < due2 || prev >= due2 {
				t.Errorf("完成應在 Cycles 首次 ≥ %d 的那一步開頭：%d（前一步 %d）", due2, before, prev)
			}
			e := sbFind(t, m, "complete", 0)
			// 寫死：D4h 之後每步開頭 16148+7j，首次 ≥ 21232 為 j=727，即 21237。
			if e.Cycles != 21237 || e.Due != due2 {
				t.Errorf("完成時刻寫死：%+v", e)
			}
			break
		}
		prev = before
	}
	if m.SBDMA8Stats().DMA8Completions != 1 {
		t.Fatal("週期時鐘：恢復後應完成")
	}

	// pausedAt ≥ due：D0h 那一步開頭 Cycles < due，這道 out 扣過週期後越過 due。
	m = sbCtlMachine(t, prep)
	sbStart(t, m, 0x14, 0x1000, 3, 4)
	due := m.sb8.due
	for m.CPU.Cycles+outCost < due {
		mustStep(t, m, "接近到期")
	}
	c1 := m.CPU.Cycles
	sbOutStep(t, m, 0x22C, 0xD0)
	if c1 >= due || m.sb8.pausedAt < due || m.SBDMA8Stats().DMA8Completions != 0 {
		t.Fatalf("構造失敗：D0h 那一步開頭 %d、pausedAt %d、到期 %d", c1, m.sb8.pausedAt, due)
	}
	for i := 0; i < 200; i++ {
		mustStep(t, m, "暫停中")
		if m.SBDMA8Stats().DMA8Completions != 0 {
			t.Fatal("pausedAt ≥ due：暫停中不應完成")
		}
	}
	if a, c := sbReadCh1(m); a != 0x1004 || c != 0xFFFF {
		t.Errorf("暫停中讀值為 n 對應的位址與計數：%04X %04X", a, c)
	}
	sbOutStep(t, m, 0x22C, 0xD4)
	if m.sb8.due > m.CPU.Cycles {
		t.Errorf("恢復後 due' ≤ now：%d %d", m.sb8.due, m.CPU.Cycles)
	}
	mustStep(t, m, "下一步")
	if m.SBDMA8Stats().DMA8Completions != 1 {
		t.Error("D4h 之後下一個 Step 開頭應完成")
	}
}

// 第 2.8 項：暫停中的 IRQ7。前一個傳輸在 IMR bit7 設起時完成（IRR7 待處理），開始新傳輸並暫停；
// 清 IMR bit7 後照常遞送（暫停那一步之後第一個允許的步），或暫停中讀 22Eh 後 IRR7 清除、不遞送。
func TestSB8PauseIRQ7(t *testing.T) {
	for _, read22E := range []bool{false, true} {
		m := sbCtlMachine(t, nil)
		m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
		m.Out8(0x21, 0x80)
		s := sbStart(t, m, 0x14, 0x1000, 3, 4)
		sbRunTo(t, m, s+4200)
		if !m.sb8.irr7 || m.sb8.isr7 || m.sb8.irq7Deliveries != 0 || m.sb8.active {
			t.Fatalf("遮罩中完成：IRR7 待處理、不遞送：%+v", m.SBDMA8Stats())
		}
		sbStart(t, m, 0x14, 0x2000, 3, 4)
		runSteps(t, m, 10, "")
		p := sbOutStep(t, m, 0x22C, 0xD0)
		if !m.sb8.paused || !m.sb8.irr7 || !m.soundBlaster.IRQPending {
			t.Fatalf("暫停不動 IRR7、IRQPending：paused=%t IRR7=%t", m.sb8.paused, m.sb8.irr7)
		}
		if read22E {
			m.In8(0x22E)
		}
		m.Out8(0x21, 0)
		if read22E {
			runSteps(t, m, 20, "")
			if m.sb8.irr7 || m.sb8.irq7Deliveries != 0 || !m.sb8.paused {
				t.Errorf("暫停中讀 22Eh：IRR7 清除、不遞送：%+v", m.SBDMA8Stats())
			}
			continue
		}
		mustStep(t, m, "放行後第一步")
		if e := sbFind(t, m, "irq7", 0); m.sb8.irq7Deliveries != 1 || e.Step != p {
			t.Errorf("暫停那一步之後第一個允許的步應遞送：遞送 %d 次、步 %d（p=%d）", m.sb8.irq7Deliveries, e.Step, p)
		}
		runSteps(t, m, 20, "處理常式")
		if !m.sb8.paused || m.sb8.isr7 || m.sb8.irr7 || !strings.HasSuffix(logKinds(m), "pause,irq7,eoi") {
			t.Errorf("處理常式之後仍暫停：%+v %s", m.SBDMA8Stats(), logKinds(m))
		}
	}
}

// 第 2.9 項：暫停中照 0562 的次序重設 DMA 並送新 14h。
func TestSB8PauseReprogramAndRestart(t *testing.T) {
	m := sbCtlMachine(t, nil)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	s := sbStart(t, m, 0x14, 0x1000, 3, 4)
	sbRunTo(t, m, s+2599)
	sbOutStep(t, m, 0x22C, 0xD0) // k(p) = ⌊2600·4/4170⌋ = 2
	runSteps(t, m, 100, "暫停中")
	for _, w := range [][2]uint16{{0x0A, 0x05}, {0x83, 0}, {0x0C, 0}, {0x02, 0x00}, {0x02, 0x20}, {0x03, 7}, {0x03, 0}, {0x0B, 0x49}, {0x0A, 0x01}} {
		sbOutStep(t, m, w[0], uint8(w[1]))
	}
	if m.sb8Err != nil || !m.sb8.paused || !m.sb8.dmaRewritten {
		t.Fatalf("暫停中重設 DMA 不報錯：%v", m.sb8Err)
	}
	if a, c := sbReadCh1(m); a != 0x2000 || c != 7 {
		t.Errorf("改寫後讀 02h、03h 得驅動寫入值：%04X %04X", a, c)
	}
	sbDSP(m, 0x14, 7)
	s2 := sbOutStep(t, m, 0x22C, 0)
	if s2 != 2711 || m.sb8Err != nil {
		t.Fatalf("新 14h 應在序號 2711 開始：%d %v", s2, m.sb8Err)
	}
	c, st := sbFind(t, m, "cancel", 0), sbFind(t, m, "start", 1)
	if c.Transferred != 2 || c.Step != s2 || st.Step != s2 || c.CS != st.CS || c.IP != st.IP || c.CS != PSPSeg || c.IP != 0x100 {
		t.Errorf("隱式取消：Transferred=k(p)、與新 start 同 Step、CS、IP：%+v %+v", c, st)
	}
	if st.Addr != 0x2000 || st.Count != 7 || st.Length != 8 || m.sb8.tc != 0 || m.sb8.irr7 || m.sb8.paused || m.sb8.dmaRewritten {
		t.Errorf("新傳輸以驅動寫入值開始，舊傳輸沒有 TC、IRR7：%+v tc=%X", st, m.sb8.tc)
	}
	if got := sbRunUntilComplete(t, m, 9000); got != 2711+8339 {
		t.Errorf("新傳輸應在 %d 完成：%d", 2711+8339, got)
	}
	runSteps(t, m, 20, "處理常式")
	if m.sb8.irq7Deliveries != 1 || logKinds(m) != "start,pause,cancel,start,complete,irq7,eoi" {
		t.Errorf("只遞送一次 IRQ7：%d %s", m.sb8.irq7Deliveries, logKinds(m))
	}
}

// 第 2.10 項：部分改寫。只寫 0Ch、02h 的低位元組，高位元組等於 addr+k(p) 的高位元組。
func TestSB8PausePartialRewrite(t *testing.T) {
	m := sbCtlMachine(t, nil)
	s := sbStart(t, m, 0x14, 0x10FF, 3, 4)
	sbRunTo(t, m, s+2599)
	sbOutStep(t, m, 0x22C, 0xD0) // k(p) = 2：0x10FF+2 = 0x1101
	sbOutStep(t, m, 0x0C, 0)
	if m.sb8.dmaRewritten || m.sb8.dma.Current[2] != 0x10FF {
		t.Fatal("0Ch 不算改寫")
	}
	sbOutStep(t, m, 0x02, 0x55)
	d := m.sb8.dma
	if d.Current[2] != 0x1155 || d.Current[3] != 1 || !m.sb8.dmaRewritten {
		t.Errorf("Current[2] 的高位元組應保留前進後的 11h：%04X，Current[3]=%04X", d.Current[2], d.Current[3])
	}
}

// 第 2.11 項：暫停中不改 DMA 直接送新 14h，隱式取消寫回的進度用於 444 §2.4 第 5 項。
func TestSB8PauseRestartUsesWrittenBack(t *testing.T) {
	mk := func() *Machine {
		m := sbCtlMachine(t, nil)
		s := sbStart(t, m, 0x14, 0x1000, 3, 4)
		sbRunTo(t, m, s+2599)
		sbOutStep(t, m, 0x22C, 0xD0) // k(p) = 2
		runSteps(t, m, 50, "")
		return m
	}
	m := mk()
	sbDSP(m, 0x14, 1)
	sbOutStep(t, m, 0x22C, 0) // n = 2 ≤ (3−2)+1
	st := sbFind(t, m, "start", 1)
	if m.sb8Err != nil || st.Addr != 0x1002 || st.Count != 1 || sbFind(t, m, "cancel", 0).Transferred != 2 {
		t.Errorf("隱式取消寫回 Current[2]、Current[3]，新傳輸以寫回值開始：%v %+v", m.sb8Err, st)
	}
	m = mk()
	sbDSP(m, 0x14, 2)
	sbFailAt(t, "n=3 大於寫回的計數＋1", m, 0x22C, 0, "大於")
}

// 第 2.12 項：暫停中送 80h，舊傳輸隱式取消，靜音照 444 開始與完成。
func TestSB8PauseThenSilence(t *testing.T) {
	m := sbCtlMachine(t, nil)
	m.CPU.SetFlags(m.CPU.Flags | cpu.IF)
	s := sbStart(t, m, 0x14, 0x1000, 3, 4)
	sbRunTo(t, m, s+2599)
	sbOutStep(t, m, 0x22C, 0xD0)
	sbDSP(m, 0x80, 3)
	s2 := sbOutStep(t, m, 0x22C, 0)
	if m.sb8Err != nil || m.sb8.cmd != 0x80 || m.sb8.paused || sbFind(t, m, "cancel", 0).Transferred != 2 {
		t.Fatalf("80h 隱式取消舊傳輸：%v", m.sb8Err)
	}
	if d := m.sb8.dma; d.Current[2] != 0x1002 || d.Current[3] != 1 {
		t.Errorf("隱式取消寫回進度：%04X %04X", d.Current[2], d.Current[3])
	}
	if got := sbRunUntilComplete(t, m, 5000); got != s2+4170 {
		t.Errorf("靜音應在 %d 完成：%d", s2+4170, got)
	}
	runSteps(t, m, 20, "")
	if logKinds(m) != "start,pause,cancel,start,complete,irq7,eoi" || m.sb8.silenceCompletions != 1 {
		t.Errorf("紀錄：%s", logKinds(m))
	}
}

// 第 2.13 項：隱式取消前 IRR7、ISR7、IRQPending 各自為真，取消後三者不變。
func TestSB8ImplicitCancelKeepsIRQState(t *testing.T) {
	for i, set := range []func(m *Machine){
		func(m *Machine) { m.sb8.irr7 = true },
		func(m *Machine) { m.sb8.isr7 = true },
		func(m *Machine) { m.soundBlaster.IRQPending = true },
	} {
		m := sbCtlMachine(t, nil)
		m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
		s := sbStart(t, m, 0x14, 0x1000, 3, 4)
		sbRunTo(t, m, s+100)
		sbOutStep(t, m, 0x22C, 0xD0)
		set(m)
		before := [3]bool{m.sb8.irr7, m.sb8.isr7, m.soundBlaster.IRQPending}
		sbDSP(m, 0x14, 0)
		sbOutStep(t, m, 0x22C, 0)
		if after := [3]bool{m.sb8.irr7, m.sb8.isr7, m.soundBlaster.IRQPending}; after != before || !strings.Contains(logKinds(m), "cancel,start") {
			t.Errorf("第 %d 例：隱式取消不動 IRR7、ISR7、IRQPending：%v → %v", i, before, after)
		}
	}
}

// sbStepErr 讓 0100h 的 out dx,al 寫 port←v，回那一步的錯誤。
func sbStepErr(m *Machine, port uint16, v uint8) error {
	m.CPU.IP, m.CPU.R[cpu.DX], m.CPU.R[cpu.AX] = 0x100, port, uint16(v)
	return m.Step()
}

// 第 2.14 項：暫停中改寫 DMA 後送 D4h，失敗即關閉。
func TestSB8ResumeAfterRewriteRefused(t *testing.T) {
	m := sbCtlMachine(t, nil)
	s := sbStart(t, m, 0x14, 0x1000, 3, 4)
	sbRunTo(t, m, s+100)
	sbOutStep(t, m, 0x22C, 0xD0)
	sbOutStep(t, m, 0x0B, 0x49)
	sbOutStep(t, m, 0x22C, 0xD0) // 已暫停、DMA 已改寫：D0h 接受，不動作
	if m.sb8Err != nil || !m.sb8.dmaRewritten || m.sb8.pausedAt != s+101 {
		t.Fatalf("改寫後 D0h 接受：%v", m.sb8Err)
	}
	start, due := m.sb8.start, m.sb8.due
	err := sbStepErr(m, 0x22C, 0xD4)
	if err == nil || !strings.Contains(err.Error(), "未被接受") || !strings.Contains(err.Error(), "已改寫") || strings.Contains(err.Error(), "未實作") {
		t.Errorf("改寫後 D4h 應失敗即關閉：%v", err)
	}
	if !m.sb8.paused || m.sb8.start != start || m.sb8.due != due || m.SBDMA8Stats().Resumes != 0 {
		t.Error("拒絕時欄位不變")
	}
}

// 第 2.15 項：idle 與 80h 靜音中的 D0h、D4h 失敗即關閉。
func TestSB8ControlRefusedIdleAndSilence(t *testing.T) {
	for _, silence := range []bool{false, true} {
		for _, v := range []uint8{0xD0, 0xD4} {
			m := sbCtlMachine(t, nil)
			want := "沒有進行中的傳輸"
			if silence {
				sbDSP(m, 0x40, 0xA6, 0x80, 3)
				sbOutStep(t, m, 0x22C, 0)
				want = "靜音"
			}
			err := sbStepErr(m, 0x22C, v)
			if err == nil || !strings.Contains(err.Error(), "未被接受") || !strings.Contains(err.Error(), want) ||
				!strings.Contains(err.Error(), "docs/spec/445") || strings.Contains(err.Error(), "未實作") {
				t.Errorf("靜音=%t %02Xh：應失敗即關閉（含「未被接受」「%s」、不含「未實作」）：%v", silence, v, want, err)
			}
			if m.sb8.paused || m.SBDMA8Stats().Pauses != 0 {
				t.Errorf("靜音=%t %02Xh：不應暫停", silence, v)
			}
		}
	}
}

// 第 2.16 項：暫停中 226h 重設。
func TestSB8PauseReset(t *testing.T) {
	for _, rewrite := range []bool{false, true} {
		m := sbCtlMachine(t, nil)
		m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
		s := sbStart(t, m, 0x14, 0x1000, 3, 4)
		sbRunTo(t, m, s+2599)
		sbOutStep(t, m, 0x22C, 0xD0) // k(p) = 2
		if rewrite {
			for _, w := range [][2]uint16{{0x0C, 0}, {0x02, 0x00}, {0x02, 0x30}, {0x03, 0x10}, {0x03, 0}} {
				sbOutStep(t, m, w[0], uint8(w[1]))
			}
		}
		sbRunTo(t, m, s+6000) // 原始時鐘已過到期：以原始時鐘算的進度會是 4
		m.sb8.irr7 = true
		sbOutStep(t, m, 0x226, 1)
		sbOutStep(t, m, 0x226, 0)
		d := m.sb8.dma
		wantA, wantC := uint16(0x1002), uint16(1)
		if rewrite {
			wantA, wantC = 0x3000, 0x10
		}
		if e := sbFind(t, m, "cancel", 0); e.Transferred != 2 || d.Current[2] != wantA || d.Current[3] != wantC {
			t.Errorf("改寫=%t：cancel 的 Transferred=k(p)、暫存器 %04X %04X：%d %04X %04X", rewrite, wantA, wantC, e.Transferred, d.Current[2], d.Current[3])
		}
		if st := m.SBDMA8Stats(); st.Paused || st.Active || m.sb8.pausedAt != 0 || m.sb8.dmaRewritten || m.sb8.irr7 {
			t.Errorf("改寫=%t：重設清 paused、pausedAt、dmaRewritten、IRR7：%+v", rewrite, st)
		}
		if err := sbStepErr(m, 0x22C, 0xD4); err == nil || !strings.Contains(err.Error(), "沒有進行中的傳輸") {
			t.Errorf("改寫=%t：重設後的 D4h 依 idle 拒絕：%v", rewrite, err)
		}
	}
}

// 445 §2.5：恢復時 due + d 溢位 uint64 失敗即關閉，欄位都不改。
func TestSB8ResumeOverflow(t *testing.T) {
	m := sbCtlMachine(t, nil)
	s := sbStart(t, m, 0x14, 0x1000, 3, 4)
	sbRunTo(t, m, s+10)
	sbOutStep(t, m, 0x22C, 0xD0)
	runSteps(t, m, 20, "")
	m.sb8.due = math.MaxUint64 - 5
	before := *m.sb8
	err := sbStepErr(m, 0x22C, 0xD4)
	if err == nil || !strings.Contains(err.Error(), "溢位") {
		t.Errorf("due + d 溢位應失敗即關閉：%v", err)
	}
	if after := *m.sb8; after.start != before.start || after.due != before.due || !after.paused || after.pausedAt != before.pausedAt || after.resumes != 0 {
		t.Error("溢位時欄位都不改")
	}
}

// ---- 第 2.17 項：狀態檔 ---------------------------------------------------------

// sbPauseProg：設 TC、DMA 通道 1（位址 1000h、計數 3）、14h 長度 4、延遲 1000 步、D0h、延遲 3000 步、D4h、sti、等處理常式寫旗標。
func sbPauseProg() []byte {
	var a sbAsm
	a.op(0xBA, 0x2C, 0x02)
	for _, v := range []uint8{0x40, 0xA6} {
		a.op(0xB0, v)
		a.op(0xEE)
	}
	for _, w := range [][2]uint8{{0x0A, 5}, {0x83, 0}, {0x0C, 0}, {0x02, 0}, {0x02, 0x10}, {0x03, 3}, {0x03, 0}, {0x0B, 0x49}, {0x0A, 1}} {
		a.op(0xB0, w[1])
		a.op(0xE6, w[0])
	}
	for _, v := range []uint8{0x14, 0x03, 0x00} {
		a.op(0xB0, v)
		a.op(0xEE)
	}
	a.op(0xB9, 0xE8, 0x03) // mov cx,1000
	a.op(0xE2, 0xFE)       // loop $
	a.op(0xB0, 0xD0)
	a.op(0xEE)
	a.op(0xB9, 0xB8, 0x0B) // mov cx,3000
	a.op(0xE2, 0xFE)
	a.op(0xB0, 0xD4)
	a.op(0xEE)
	a.op(0xFB)
	a.op(0x80, 0x3E, byte(sbFlagOff&0xFF), byte(sbFlagOff>>8), 0x01)
	a.op(0x75, 0xF9)
	a.op(0xEB, 0xFE)
	code := a.code
	for uint16(len(code))+0x100 < sbHOff {
		code = append(code, 0x90)
	}
	return append(code, 0x50, 0x52, 0xBA, 0x2E, 0x02, 0xEC,
		0xC6, 0x06, byte(sbFlagOff&0xFF), byte(sbFlagOff>>8), 0x01,
		0xB0, 0x20, 0xE6, 0x20, 0x5A, 0x58, 0xCF)
}

// sbReencode 解開 SaveState 的兩個值，以 f 改 sbDMA8State 後重新編碼。
func sbReencode(t *testing.T, saved []byte, f func(st *sbDMA8State)) []byte {
	t.Helper()
	dec := gob.NewDecoder(bytes.NewReader(saved))
	var ms machineState
	var sb sbDMA8State
	mustNil(t, "decode", dec.Decode(&ms))
	mustNil(t, "decode sb", dec.Decode(&sb))
	f(&sb)
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	mustNil(t, "encode", enc.Encode(&ms))
	mustNil(t, "encode sb", enc.Encode(&sb))
	return b.Bytes()
}

func TestSB8PauseStateRoundTrip(t *testing.T) {
	for _, cyc := range []bool{false, true} {
		var prep func(m *Machine)
		if cyc {
			prep = func(m *Machine) { m.CycleClock, m.CPUHz = true, 100_000_000; m.RecalcIRQ0() }
		}
		mk := func() *Machine {
			m := newSB8(t, sbPauseProg(), prep)
			setIVT(m, 0x0F, PSPSeg, sbHOff)
			return m
		}
		fresh := func() *Machine {
			n := New()
			n.SetSoundBlaster(true)
			mustNil(t, "SetSBDMA8", n.SetSBDMA8(true))
			if prep != nil {
				prep(n)
			}
			return n
		}
		ref := mk()
		var pauseAt uint64
		for i := 0; i < 20000 && ref.Mem[uint32(PSPSeg)*16+sbFlagOff] == 0; i++ {
			mustStep(t, ref, "不中斷")
			if pauseAt == 0 && ref.sb8.paused {
				pauseAt = ref.Steps
			}
		}
		runSteps(t, ref, 100, "處理常式之後")
		total := ref.Steps
		if st := ref.SBDMA8Stats(); pauseAt == 0 || st.Pauses != 1 || st.Resumes != 1 || st.DMA8Completions != 1 || st.IRQ7Deliveries != 1 || st.ISR7 {
			t.Fatalf("時鐘=%t：不中斷的執行應暫停、恢復、完成並跑過處理常式：%+v", cyc, st)
		}
		if got := logKinds(ref); got != "start,pause,resume,complete,irq7,eoi" {
			t.Fatalf("時鐘=%t：紀錄 %s", cyc, got)
		}
		want := sbSave(t, ref)
		save := pauseAt + 1000

		m := mk()
		runSteps(t, m, int(save), "")
		saved := sbSave(t, m)
		n := fresh()
		mustNil(t, "LoadState", n.LoadState(bytes.NewReader(saved)))
		if st := n.SBDMA8Stats(); !st.Paused || n.sb8.pausedAt != m.sb8.pausedAt || n.sb8.dmaRewritten {
			t.Fatalf("時鐘=%t：讀回後仍暫停：%+v", cyc, st)
		}
		runSteps(t, n, int(total-save), "LoadState 之後")
		if !bytes.Equal(sbSave(t, n), want) {
			t.Errorf("時鐘=%t：SaveState／LoadState 之後的最終狀態應與不中斷的執行相同", cyc)
		}

		m = mk()
		runSteps(t, m, int(save), "")
		sn := m.Snapshot()
		runSteps(t, m, 6000, "多跑過恢復")
		if m.sb8.paused {
			t.Fatal("多跑一段後應已恢復")
		}
		m.Restore(sn)
		if !m.sb8.paused {
			t.Fatalf("時鐘=%t：Restore 後仍暫停", cyc)
		}
		runSteps(t, m, int(total-save), "Restore 之後")
		if !bytes.Equal(sbSave(t, m), want) {
			t.Errorf("時鐘=%t：Snapshot／Restore 之後的最終狀態應與不中斷的執行相同", cyc)
		}

		if !cyc {
			sbValidateCases(t, saved, mk)
			sb444Compat(t, saved, fresh)
		} else {
			// 週期時鐘不檢查 PausedAt ≤ Due（§2.5）。
			n := fresh()
			mustNil(t, "週期時鐘 PausedAt > Due", n.LoadState(bytes.NewReader(sbReencode(t, saved, func(st *sbDMA8State) {
				st.PausedAt = st.Due + 1
			}))))
		}
	}
}

// sbValidateCases：sb8Validate 的四個條件各以一份竄改的狀態檔驗證被拒且機器不變。
func sbValidateCases(t *testing.T, saved []byte, mk func() *Machine) {
	t.Helper()
	on := mk()
	runSteps(t, on, 50, "")
	if ok := sbReencode(t, saved, func(*sbDMA8State) {}); on.LoadState(bytes.NewReader(ok)) != nil {
		t.Fatal("未竄改的重新編碼應可讀")
	}
	on = mk()
	runSteps(t, on, 50, "")
	for _, x := range []struct {
		name string
		f    func(st *sbDMA8State)
	}{
		{"1 Paused 而 !Active", func(st *sbDMA8State) { st.Active = false }},
		{"1 Paused 而 Cmd=80h", func(st *sbDMA8State) { st.Cmd = 0x80 }},
		{"2 PausedAt < Start", func(st *sbDMA8State) { st.PausedAt = st.Start - 1 }},
		{"3 步數時鐘 PausedAt > Due", func(st *sbDMA8State) { st.PausedAt = st.Due + 1 }},
		{"4 !Paused 而 PausedAt≠0", func(st *sbDMA8State) { st.Paused = false }},
		{"4 !Paused 而 DMARewrittenWhilePaused", func(st *sbDMA8State) {
			st.Paused, st.PausedAt, st.DMARewrittenWhilePaused = false, 0, true
		}},
	} {
		bad := sbReencode(t, saved, x.f)
		before := sbSave(t, on)
		if err := on.LoadState(bytes.NewReader(bad)); err == nil || !strings.Contains(err.Error(), "445") || !bytes.Equal(sbSave(t, on), before) {
			t.Errorf("%s：應被拒且機器不變：%v", x.name, err)
		}
	}
}

// sb444Compat：不含三欄的 444 版 sbDMA8State（以同名欄位子集的型別編碼）讀成未暫停；StateDigest 偵測 Paused；
// 未暫停時 sbDMA8State 的 JSON 不含三個欄名。
func sb444Compat(t *testing.T, saved []byte, fresh func() *Machine) {
	t.Helper()
	var fields []reflect.StructField
	rt := reflect.TypeOf(sbDMA8State{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		switch f.Name {
		case "Paused", "PausedAt", "DMARewrittenWhilePaused":
			continue
		}
		fields = append(fields, reflect.StructField{Name: f.Name, Type: f.Type})
	}
	old := reflect.New(reflect.StructOf(fields))
	dec := gob.NewDecoder(bytes.NewReader(saved))
	var ms machineState
	mustNil(t, "decode", dec.Decode(&ms))
	mustNil(t, "decode 444", dec.Decode(old.Interface()))
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	mustNil(t, "encode", enc.Encode(&ms))
	mustNil(t, "encode 444", enc.Encode(old.Interface()))
	if bytes.Contains(b.Bytes(), []byte("PausedAt")) {
		t.Fatal("444 版的編碼不應含新欄位")
	}
	n := fresh()
	mustNil(t, "LoadState 444 版", n.LoadState(bytes.NewReader(b.Bytes())))
	if n.sb8.paused || n.sb8.pausedAt != 0 || n.sb8.dmaRewritten || !n.sb8.active {
		t.Errorf("444 版的 sbDMA8State 應讀成未暫停（傳輸仍進行中）")
	}

	d1, err := StateDigest(bytes.NewReader(saved))
	mustNil(t, "StateDigest", err)
	d2, err := StateDigest(bytes.NewReader(sbReencode(t, saved, func(st *sbDMA8State) { st.Paused = false })))
	mustNil(t, "StateDigest", err)
	if d1 == d2 {
		t.Error("StateDigest 應偵測 Paused 的改變")
	}
	jp, _ := json.Marshal(n.sb8Save())
	for _, name := range []string{`"Paused"`, `"PausedAt"`, `"DMARewrittenWhilePaused"`} {
		if bytes.Contains(jp, []byte(name)) {
			t.Errorf("未暫停時 sbDMA8State 的 JSON 不應含 %s", name)
		}
	}
	n.sb8.paused, n.sb8.pausedAt = true, n.sb8.start
	if jp, _ = json.Marshal(n.sb8Save()); !bytes.Contains(jp, []byte(`"Paused":true`)) {
		t.Error("暫停時 JSON 應含 Paused")
	}
}
