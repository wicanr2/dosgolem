package session

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/host"
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/452 §3.1：SBDMAObserver 的單元測試。合成 .COM 依序產生九種 Kind：
//
//	第 1 段：未裝 IRQ7 處理常式（IVT[0Fh] 仍是 StubSeg）時一次 14h 傳輸 → start、complete、defaulted；讀 22Eh 清 IRQPending。
//	第 2 段：裝處理常式後一次 14h 傳輸 → start、complete、irq7、eoi。
//	第 3 段：14h 後 D0h、D4h、D0h；暫停中改寫 DMA 再送 14h（隱式取消）→ pause、resume、pause、cancel、start、complete、irq7、eoi。
//	第 4 段：沒有傳輸時送 D0h → control-idle。
//	第 5 段：靜音 80h → start（silence）、complete、irq7、eoi。
//	第 6 段：14h 後寫 226h←1（DSP 重設，未經暫停）→ start、cancel。
//
// 不開分頁，所以 Read8（線性）與 sb8SHA（實體）讀到同一批 bytes（§3.1 第 2 項）。

const (
	sbtLen    = 16     // 每次傳輸的 byte 數
	sbtPage   = 0x01   // DMA 頁
	sbtAddr   = 0x2000 // DMA 位址；實體 12000h
	sbtHOff   = 0x0400 // IRQ7 處理常式在 COM 段內的位移
	sbtFlag   = 0x0500 // 處理常式設的旗標（DS = PSP 段）
	sbtBudget = 400000 // 足以跑完整支程式（每次傳輸約 1.7 萬步）
)

type sbtAsm struct{ code []byte }

func (a *sbtAsm) b(v ...byte) { a.code = append(a.code, v...) }
func (a *sbtAsm) off() uint16 { return 0x100 + uint16(len(a.code)) }
func (a *sbtAsm) out(p uint16, v uint8) {
	if p <= 0xFF {
		a.b(0xB0, v, 0xE6, byte(p))
		return
	}
	a.b(0xBA, byte(p), byte(p>>8), 0xB0, v, 0xEE)
}
func (a *sbtAsm) dsp(vs ...uint8) {
	a.b(0xBA, 0x2C, 0x02)
	for _, v := range vs {
		a.b(0xB0, v, 0xEE)
	}
}
func (a *sbtAsm) progDMA() {
	for _, w := range [][2]uint16{{0x0A, 5}, {0x83, sbtPage}, {0x0C, 0}, {0x02, sbtAddr & 0xFF}, {0x02, sbtAddr >> 8},
		{0x03, (sbtLen - 1) & 0xFF}, {0x03, (sbtLen - 1) >> 8}, {0x0B, 0x49}, {0x0A, 1}} {
		a.out(w[0], uint8(w[1]))
	}
}

// waitFlag 等處理常式設旗標，再清掉。
func (a *sbtAsm) waitFlag() {
	top := a.off()
	a.b(0x80, 0x3E, byte(sbtFlag&0xFF), byte(sbtFlag>>8), 0x01) // cmp byte [flag],1
	a.b(0x75, byte(int8(int(top)-int(a.off())-2)))              // jne top
	a.b(0xC6, 0x06, byte(sbtFlag&0xFF), byte(sbtFlag>>8), 0x00) // mov byte [flag],0
}

func sbtProgram() []byte {
	var a sbtAsm
	a.b(0xFB) // sti
	// DSP 重設（閒置時不產生事件）、讀 22Ah、時間常數 A6h；開 IRQ7 的 PIC 遮罩。
	a.out(0x226, 1)
	a.out(0x226, 0)
	a.b(0xBA, 0x2A, 0x02, 0xEC) // mov dx,22Ah；in al,dx
	a.dsp(0x40, 0xA6)
	a.b(0xE4, 0x21, 0x24, 0x7F, 0xE6, 0x21) // in al,21h；and al,7Fh；out 21h,al
	// 第 1 段：defaulted。輪詢 08h 的 TC，之後讀 22Eh 清 IRQPending。
	a.progDMA()
	a.dsp(0x14, sbtLen-1, 0)
	top := a.off()
	a.b(0xE4, 0x08, 0xA8, 0x02)                    // in al,08h；test al,02h
	a.b(0x74, byte(int8(int(top)-int(a.off())-2))) // jz top
	for i := 0; i < 8; i++ {
		a.b(0x90) // 讓 defaulted 那一步先過
	}
	a.b(0xBA, 0x2E, 0x02, 0xEC) // mov dx,22Eh；in al,dx
	// 裝處理常式：IVT[0Fh] = CS:sbtHOff。
	a.b(0x1E, 0x31, 0xC0, 0x8E, 0xD8)                                 // push ds；xor ax,ax；mov ds,ax
	a.b(0xC7, 0x06, 0x3C, 0x00, byte(sbtHOff&0xFF), byte(sbtHOff>>8)) // mov word [3Ch],HOFF
	a.b(0x8C, 0x0E, 0x3E, 0x00)                                       // mov [3Eh],cs
	a.b(0x1F)                                                         // pop ds
	// 第 2 段：irq7、eoi。
	a.progDMA()
	a.dsp(0x14, sbtLen-1, 0)
	a.waitFlag()
	// 第 3 段：pause、resume、pause、暫停中改寫 DMA 再 14h（隱式 cancel）。
	a.progDMA()
	a.dsp(0x14, sbtLen-1, 0)
	a.dsp(0xD0)
	a.dsp(0xD4)
	a.dsp(0xD0)
	a.progDMA()
	a.dsp(0x14, sbtLen-1, 0)
	a.waitFlag()
	// 第 4 段：control-idle。
	a.dsp(0xD0)
	// 第 5 段：靜音。
	a.dsp(0x80, sbtLen-1, 0)
	a.waitFlag()
	// 第 6 段：DSP 重設造成、未經暫停的 cancel。
	a.progDMA()
	a.dsp(0x14, sbtLen-1, 0)
	a.out(0x226, 1)
	a.out(0x226, 0)
	a.b(0xEB, 0xFE) // jmp $
	code := a.code
	for uint16(len(code))+0x100 < sbtHOff {
		code = append(code, 0x90)
	}
	// 處理常式：讀 22Eh、設旗標、out 20h,20h、iret。
	code = append(code, 0x50, 0x52, 0xBA, 0x2E, 0x02, 0xEC,
		0xC6, 0x06, byte(sbtFlag&0xFF), byte(sbtFlag>>8), 0x01,
		0xB0, 0x20, 0xE6, 0x20, 0x5A, 0x58, 0xCF)
	return code
}

// startSBOwner 以 SoundBlaster、SBDMA8 建 Owner 再載入程式（SetSBDMA8 要在載入之前，448 §2.1）。
func startSBOwner(t *testing.T, sbdma8 bool, obs StepObserver) *Owner {
	t.Helper()
	layout := sessionLayout(1, host.OutputScale2, false)
	o, err := New(Config{InitialScale: layout.Scale, InitialLayout: layout, SoundBlaster: true, SBDMA8: sbdma8})
	if err != nil {
		t.Fatal(err)
	}
	o.closer = &countingCloser{}
	if err := o.machine.LoadCOM(sbtProgram()); err != nil {
		t.Fatal(err)
	}
	if err := o.startLoadedMachine(); err != nil {
		t.Fatal(err)
	}
	o.observer = obs
	return o
}

// sbRec 實作 SBDMAObserver。每筆事件記下之後第一次 BeforeStep 的 Steps()（next），
// start 事件另在那次 BeforeStep 以 Read8 讀 DMA 緩衝算 SHA-256（readSHA）。
type sbRec struct {
	recordingObserver
	events  []machine.SBDMAEvent
	next    []uint64
	readSHA []string
	open    int // 尚未記 next 的事件起點
	panicAt int // 第 n 筆事件 panic（1 起算）
	misuse  bool
	misRead uint8
	latest  *StepView // 最近一次 BeforeStep 的視圖（token 與 Owner 目前的相同，只差 viewOpen 已關）
}

func (r *sbRec) BeforeStep(v StepView) error {
	for ; r.open < len(r.events); r.open++ {
		e := r.events[r.open]
		r.next = append(r.next, v.Steps())
		sha := ""
		if e.Kind == "start" && e.Command != 0x80 {
			buf := make([]byte, e.Length)
			for i := range buf {
				buf[i] = v.Read8(uint32(e.Page)<<16 | uint32(e.Addr+uint16(i)))
			}
			s := sha256.Sum256(buf)
			sha = hex.EncodeToString(s[:])
		}
		r.readSHA = append(r.readSHA, sha)
	}
	latest := v
	r.latest = &latest
	return r.recordingObserver.BeforeStep(v)
}

func (r *sbRec) SBDMA(e machine.SBDMAEvent) {
	r.events = append(r.events, e)
	if r.misuse && r.latest != nil {
		// 讀 CS:0100（程式第一個 byte FBh）：視圖若可用就讀到非 0。
		r.misRead = r.latest.Read8(uint32(r.latest.o.machine.CPU.Seg[cpu.CS])<<4 + 0x100)
		r.misuse = false
	}
	if r.panicAt == len(r.events) {
		panic("sbdma boom")
	}
}

// sbPlain 不實作 SBDMAObserver；記下每次 BeforeStep 時 OnSBDMA 是否非 nil。
type sbPlain struct {
	recordingObserver
	o       *Owner
	hookSet bool
}

func (p *sbPlain) BeforeStep(v StepView) error {
	if p.o != nil && p.o.machine.OnSBDMA != nil {
		p.hookSet = true
	}
	return p.recordingObserver.BeforeStep(v)
}

var sbtKinds = []string{"start", "complete", "cancel", "pause", "resume", "control-idle", "eoi", "defaulted", "irq7"}

// 第 1、2、8 項：轉發完整、時序、摘要。
func TestSBDMAObserverForwardsEveryEvent(t *testing.T) {
	rec := &sbRec{}
	a := startSBOwner(t, true, rec)
	plain := &sbPlain{}
	b := startSBOwner(t, true, plain)
	ra, err := runTurn(t, a, sbtBudget)
	if err != nil || ra.Steps != sbtBudget {
		t.Fatalf("甲：%+v %v", ra, err)
	}
	rb, err := runTurn(t, b, sbtBudget)
	if err != nil || rb.Steps != sbtBudget {
		t.Fatalf("乙：%+v %v", rb, err)
	}
	if len(a.machine.SBDMALog) != 0 {
		t.Fatalf("甲的 SBDMALog 有 %d 筆，觀察器掛上時應為 0", len(a.machine.SBDMALog))
	}
	log := b.machine.SBDMALog
	if len(rec.events) != len(log) || len(log) == 0 {
		t.Fatalf("甲收到 %d 筆，乙的 SBDMALog %d 筆", len(rec.events), len(log))
	}
	kinds := map[string]int{}
	for i, e := range rec.events {
		if got, want := strings.Join(e.Fields(), "\t"), strings.Join(log[i].Fields(), "\t"); got != want {
			t.Fatalf("第 %d 筆不同：\n甲 %s\n乙 %s", i, got, want)
		}
		kinds[e.Kind]++
	}
	for _, k := range sbtKinds {
		if kinds[k] == 0 {
			t.Errorf("Kind %s 沒有出現（%v）", k, kinds)
		}
	}
	// DSP 重設造成、未經暫停的 cancel：最後一筆事件，前一筆是 start。
	last := rec.events[len(rec.events)-1]
	if last.Kind != "cancel" || rec.events[len(rec.events)-2].Kind != "start" {
		t.Errorf("最後兩筆 %s、%s，要 start、cancel（DSP 重設）", rec.events[len(rec.events)-2].Kind, last.Kind)
	}
	t.Logf("事件 %d 筆：%v", len(rec.events), kinds)
	// 第 2 項：時序與 Read8 的雜湊。
	if len(rec.next) != len(rec.events) {
		t.Fatalf("next %d 筆，事件 %d 筆", len(rec.next), len(rec.events))
	}
	starts := 0
	for i, e := range rec.events {
		want := e.Step
		switch e.Kind {
		case "complete", "irq7", "defaulted":
			want = e.Step + 1
		}
		if rec.next[i] != want {
			t.Errorf("第 %d 筆 %s（Step %d）之後的 BeforeStep Steps()=%d，要 %d", i, e.Kind, e.Step, rec.next[i], want)
		}
		if e.Kind == "start" && e.Command != 0x80 {
			starts++
			if e.SHA256 == "" || rec.readSHA[i] != e.SHA256 {
				t.Errorf("第 %d 筆 start：Read8 雜湊 %s，事件 %s", i, rec.readSHA[i], e.SHA256)
			}
		}
	}
	if starts < 4 {
		t.Errorf("14h 的 start 只有 %d 筆", starts)
	}
	// 第 8 項：同一停止步的 Digest 全同。
	da, errA := a.Digest()
	db, errB := b.Digest()
	if errA != nil || errB != nil || da != db {
		t.Fatalf("Digest 不同：%v %v", errA, errB)
	}
}

// fullRun 跑完整支程式，回事件序列。
func sbtFullRun(t *testing.T) []machine.SBDMAEvent {
	t.Helper()
	rec := &sbRec{}
	o := startSBOwner(t, true, rec)
	if _, err := runTurn(t, o, sbtBudget); err != nil {
		t.Fatal(err)
	}
	return rec.events
}

// 第 3 項：Advance 的預算恰好停在 start 那一步之後。
func TestSBDMAObserverAcrossTurns(t *testing.T) {
	evs := sbtFullRun(t)
	var n uint64
	for _, e := range evs {
		if e.Kind == "start" {
			n = e.Step
			break
		}
	}
	if n == 0 {
		t.Fatal("沒有 start")
	}
	rec := &sbRec{}
	o := startSBOwner(t, true, rec)
	r, err := runTurn(t, o, InstructionBudget(n))
	if err != nil || r.Steps != n || o.machine.Steps != n {
		t.Fatalf("%+v %v，machine %d 步，要 %d", r, err, o.machine.Steps, n)
	}
	if len(rec.events) != 1 || rec.events[0].Kind != "start" || rec.events[0].Step != n {
		t.Fatalf("本回合事件 %+v，要一筆第 %d 步的 start", rec.events, n)
	}
	if len(rec.next) != 0 {
		t.Fatalf("本回合不該有事件之後的 BeforeStep：%v", rec.next)
	}
	if _, err := runTurn(t, o, 1); err != nil {
		t.Fatal(err)
	}
	if len(rec.next) == 0 || rec.next[0] != n {
		t.Fatalf("下一回合第一次 BeforeStep Steps()=%v，要 %d", rec.next, n)
	}
	if rec.readSHA[0] != rec.events[0].SHA256 {
		t.Fatalf("跨回合 Read8 雜湊 %s，事件 %s", rec.readSHA[0], rec.events[0].SHA256)
	}
}

// 第 4 項：Advance 返回後 OnSBDMA 為 nil；之後直接 Step 的事件進 SBDMALog。
func TestSBDMAObserverRemovedAfterTurn(t *testing.T) {
	evs := sbtFullRun(t)
	n := evs[0].Step // 第一筆是 start
	rec := &sbRec{}
	o := startSBOwner(t, true, rec)
	if _, err := runTurn(t, o, InstructionBudget(n-1)); err != nil {
		t.Fatal(err)
	}
	if o.machine.OnSBDMA != nil {
		t.Fatal("Advance 返回後 OnSBDMA 仍非 nil")
	}
	before := len(o.machine.SBDMALog)
	for i := 0; i < 4; i++ {
		if err := o.machine.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(o.machine.SBDMALog) != before+1 || o.machine.SBDMALog[before].Kind != "start" || len(rec.events) != 0 {
		t.Fatalf("回合外的 start：SBDMALog %d → %d，觀察器收到 %d", before, len(o.machine.SBDMALog), len(rec.events))
	}
}

// 第 5 項：不實作時 Advance 期間 OnSBDMA 為 nil，事件進 SBDMALog。
func TestSBDMAObserverNotImplemented(t *testing.T) {
	p := &sbPlain{}
	o := startSBOwner(t, true, p)
	p.o = o
	if _, err := runTurn(t, o, sbtBudget); err != nil {
		t.Fatal(err)
	}
	if p.hookSet {
		t.Fatal("觀察器不實作 SBDMAObserver，Advance 期間 OnSBDMA 卻非 nil")
	}
	if len(o.machine.SBDMALog) == 0 {
		t.Fatal("SBDMALog 沒有增加")
	}
}

// 第 6 項：SBDMA 內 panic → 觀察器錯誤，機器停在該步驟嘗試完成之後。
func TestSBDMAObserverPanicIsObserverFault(t *testing.T) {
	evs := sbtFullRun(t)
	rec := &sbRec{panicAt: 1}
	o := startSBOwner(t, true, rec)
	r, err := runTurn(t, o, sbtBudget)
	if err == nil || !strings.Contains(err.Error(), "SBDMA") {
		t.Fatalf("err=%v，要含 SBDMA", err)
	}
	assertObserverFault(t, o, r, err, evs[0].Step)
}

// 第 7 項：SBDMA 內使用保留的 StepView 讀記憶體 → 讀到 0、觀察器錯誤。
func TestSBDMAObserverRetainedViewMisuse(t *testing.T) {
	evs := sbtFullRun(t)
	rec := &sbRec{misuse: true, misRead: 0xEE}
	o := startSBOwner(t, true, rec)
	r, err := runTurn(t, o, sbtBudget)
	if rec.misRead != 0 {
		t.Fatalf("回呼內讀保留視圖得 %02X，要 0", rec.misRead)
	}
	if err == nil || !strings.Contains(err.Error(), "觀測器在回呼外使用了 StepView") {
		t.Fatalf("err=%v", err)
	}
	assertObserverFault(t, o, r, err, evs[0].Step)
}

// 第 9 項：SBDMA8 為假時照常建立，回呼一次都不被呼叫。
func TestSBDMAObserverWithoutSBDMA8(t *testing.T) {
	rec := &sbRec{}
	o := startSBOwner(t, false, rec)
	r, err := runTurn(t, o, 20000)
	t.Logf("SBDMA8 關：%+v err=%v", r, err)
	if len(rec.events) != 0 {
		t.Fatalf("SBDMA8 關，回呼被呼叫 %d 次", len(rec.events))
	}
	if r.Steps == 0 {
		t.Fatal("沒有前進")
	}
}
