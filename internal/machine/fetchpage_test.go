package machine

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 取指令頁快取（fetchpage.go，docs/spec/455-fetch-page-cache §5）的測試。
//
// 差分測試以兩台機器對照：一台照常（syncCodeFastPath 在轉換開啟時裝 FetchPager），另一台設
// noFetchPager（取指令全走 Bus.Read8，即 455 之前的路徑），同樣的操作逐步比暫存器、錯誤與記憶體。

// setNoFetchPager 讓 m 成為對照機：之後的 syncCodeFastPath 都不裝 FetchPager。
func setNoFetchPager(m *Machine) {
	m.noFetchPager = true
	m.syncCodeFastPath()
}

// stepBoth 兩台各走一步，比錯誤字串與暫存器。
func stepBoth(t *testing.T, what string, step int, a, b *Machine) {
	t.Helper()
	ea, eb := a.Step(), b.Step()
	if fmt.Sprint(ea) != fmt.Sprint(eb) {
		t.Fatalf("%s step %d：錯誤 %v，參考 %v", what, step, ea, eb)
	}
	if a.CPU.R != b.CPU.R || a.CPU.IP != b.CPU.IP || a.CPU.Seg != b.CPU.Seg || a.CPU.Flags != b.CPU.Flags {
		t.Fatalf("%s step %d：暫存器不同 %v %04X，參考 %v %04X", what, step, a.CPU.R, a.CPU.IP, b.CPU.R, b.CPU.IP)
	}
}

// 同一個 V86 分頁程式在兩台機器上逐步執行。程式改寫自己下一圈要執行的立即值（自我修改碼），
// 中途重映程式碼頁並 SetCR3、切換 A20。
func TestFetchPageDifferential(t *testing.T) {
	// 0100 B0 00        mov al,0
	// 0102 FE 06 01 01  inc byte [0101]   ; 改寫上一道的立即值
	// 0106 00 C4        add ah,al
	// 0108 EB F6        jmp 0100
	code := []byte{0xB0, 0x00, 0xFE, 0x06, 0x01, 0x01, 0x00, 0xC4, 0xEB, 0xF6}
	build := func(pager bool) (*Machine, *tables) {
		m, tb := newPagedProg(t, code, fPWU)
		if !pager {
			setNoFetchPager(m)
		}
		mustNil(t, "SetVM(true)", m.SetVM(true))
		return m, tb
	}
	a, ta := build(true)
	b, tb := build(false)
	codeLin := uint32(a.CPU.Seg[cpu.CS])<<4 + 0x100
	alt := uint32(0x60000) // 重映目標：同一段程式，但 mov al,7
	for _, m := range []*Machine{a, b} {
		page := codeLin &^ 0xFFF
		copy(m.Mem[alt:alt+0x1000], m.Mem[page:page+0x1000])
		m.Mem[alt+(codeLin&0xFFF)+1] = 7
	}
	for step := 0; step < 4000; step++ {
		switch step {
		case 1000:
			ta.mapPage(codeLin, alt)
			tb.mapPage(codeLin, alt)
		case 1500: // 改 PTE 之後才作廢
			mustNil(t, "SetCR3", a.SetCR3(ta.dir))
			mustNil(t, "SetCR3", b.SetCR3(tb.dir))
		case 2500:
			a.SetA20(false)
			b.SetA20(false)
		case 3000:
			a.SetA20(true)
			b.SetA20(true)
		}
		stepBoth(t, "SMC", step, a, b)
	}
	if !bytes.Equal(a.Mem, b.Mem) || !bytes.Equal(a.Ext, b.Ext) {
		t.Fatal("記憶體內容不同")
	}
	if a.CPU.R[cpu.AX]>>8 == 0 {
		t.Fatal("程式沒有累加，可能沒在跑")
	}
}

// CPL 由 0 切到 3（只經 recalcPM，不作廢 TLB）：程式碼頁是監督者頁，切換之後取指令必須頁錯誤。
// 釘住 syncCodeFastPath 的清空（M2）與 FetchRead8 的 cplUser 比對（M4）。
func TestFetchPageCPLSwitch(t *testing.T) {
	loop := []byte{0x40, 0xEB, 0xFD} // inc ax; jmp -3
	build := func(pager bool) *Machine {
		m, _ := newPagedProg(t, loop, fP|fW) // 低 1 MB 監督者頁
		if !pager {
			setNoFetchPager(m)
		}
		// PE=1、VM=0、CPL 0、非分段模式（434 §2.1 的測試狀態，同 TestPagingStepWriteFaultInV86）。
		mustNil(t, "SetPMCPL(0)", m.SetPMCPL(0))
		return m
	}
	a, b := build(true), build(false)
	for step := 0; step < 200; step++ {
		if step == 100 {
			mustNil(t, "SetPMCPL(3)", a.SetPMCPL(3))
			mustNil(t, "SetPMCPL(3)", b.SetPMCPL(3))
			// 監督者身分在切換前已讀過這一頁，微快取讀取份留著 user=false 的條目。
		}
		stepBoth(t, "CPL", step, a, b)
		pa, oka := a.PageFault()
		pb, okb := b.PageFault()
		if oka != okb || pa != pb {
			t.Fatalf("step %d：錯誤紀錄 %v %v，參考 %v %v", step, oka, pa, okb, pb)
		}
	}
	if _, ok := b.LastPageFault(); !ok {
		t.Fatal("對照機在 CPL 3 應有頁錯誤，情境沒有成立")
	}
}

// 執行中裝讀取監看：之後每個指令位元組都要觸發監看。釘住 WatchReads 經 syncCodeFastPath 的清空（M2）
// 與 FetchRead8 的 onRead 檢查（M6）。
func TestFetchPageWatchReadsMidRun(t *testing.T) {
	loop := []byte{0x40, 0xEB, 0xFD}
	build := func(pager bool) *Machine {
		m, _ := newPagedProg(t, loop, fPWU)
		if !pager {
			setNoFetchPager(m)
		}
		mustNil(t, "SetVM(true)", m.SetVM(true))
		return m
	}
	a, b := build(true), build(false)
	lin := uint32(a.CPU.Seg[cpu.CS])<<4 + 0x100
	var na, nb int
	for step := 0; step < 300; step++ {
		if step == 100 {
			a.WatchReads(lin, lin+2, func(uint32, uint8) { na++ })
			b.WatchReads(lin, lin+2, func(uint32, uint8) { nb++ })
		}
		stepBoth(t, "WatchReads", step, a, b)
	}
	if nb == 0 {
		t.Fatal("對照機的監看沒有觸發，情境沒有成立")
	}
	if na != nb {
		t.Fatalf("監看觸發 %d 次，對照 %d 次", na, nb)
	}
}

// 實體頁 FFFFF000h：頁尾 1_0000_0000h 以 uint32 計算會環繞成 0。FetchRead8 不得 panic，結果同 Read8。
func TestFetchPageTopOf4GB(t *testing.T) {
	m, tb := newPaged(t)
	tb.mapPage(0x5000, 0xFFFFF000)
	tb.enable(t)
	want := m.Read8(0x5FFF)
	v, page := m.FetchRead8(0x5FFF)
	if v != want || page != nil {
		t.Fatalf("FetchRead8 = %02X、頁 %d bytes；Read8 = %02X", v, len(page), want)
	}
}

// 屬性測試：FetchRead8 交出的頁，4096 個位移都等於 Read8，且讀取沒有產生頁錯誤；有讀取監看時不交頁。
// 隨機頁表（低記憶體、A0000–BFFFF、Ext、Ext 之外、FFFFF000h，U/W/P 隨機）、隨機 A20、平面模式、
// 監督者／使用者切換（不作廢 TLB）。釘住 M4（cplUser）、M5（VGA 視窗）、M6（onRead）、M8（A20）。
func TestFetchPagePageEqualsRead8(t *testing.T) {
	rng := rand.New(rand.NewSource(455))
	physChoices := func() uint32 {
		switch rng.Intn(6) {
		case 0:
			return 0x80000 + uint32(rng.Intn(16))*0x1000
		case 1:
			return 0xA0000 + uint32(rng.Intn(32))*0x1000
		case 2:
			return 0xC0000 + uint32(rng.Intn(48))*0x1000
		case 3:
			return 0x100000 + uint32(rng.Intn(1024))*0x1000 // 含 bit 20：A20 關時會折回
		case 4:
			return 0x500000 + uint32(rng.Intn(4))*0x1000 // Ext（4 MB）之外
		default:
			return 0xFFFFF000
		}
	}
	handed := 0
	for trial := 0; trial < 400; trial++ {
		m, tb := newPaged(t)
		for i := 0x80000; i < 0xF0000; i++ {
			m.Mem[i] = uint8(rng.Intn(256))
		}
		for i := range m.Ext {
			m.Ext[i] = uint8(i*7 + trial)
		}
		var lins []uint32
		for k := 0; k < 8; k++ {
			lin := uint32(0x400000 + rng.Intn(64)*0x1000)
			flags := uint32(fP | fW)
			if rng.Intn(2) == 0 {
				flags |= fU
			}
			tb.mapFlags(lin, physChoices(), fPWU, flags)
			lins = append(lins, lin)
		}
		tb.enable(t)
		if rng.Intn(2) == 0 {
			m.planarOn = true
		}
		if rng.Intn(3) == 0 {
			m.SetA20(false)
		}
		// 先以監督者身分讀一輪（填微快取），再隨機切到使用者（只經 recalcPM）。
		for _, lin := range lins {
			m.Read8(lin)
		}
		if rng.Intn(2) == 0 {
			mustNil(t, "SetPMCPL(3)", m.SetPMCPL(3))
		}
		watch := rng.Intn(5) == 0
		if watch {
			m.WatchReads(0, 0xFFFFFFFF, func(uint32, uint8) {})
		}
		for _, lin := range lins {
			lin += uint32(rng.Intn(0x1000))
			m.clearPageFaults()
			_, page := m.FetchRead8(lin)
			if page == nil {
				continue
			}
			if watch {
				t.Fatalf("trial %d：有讀取監看仍交出頁 %08X", trial, lin)
			}
			if _, ok := m.PageFault(); ok {
				t.Fatalf("trial %d：%08X 的讀取有頁錯誤仍交出頁", trial, lin)
			}
			handed++
			base := lin &^ 0xFFF
			for off := uint32(0); off < 0x1000; off++ {
				if want := m.Read8(base | off); page[off] != want {
					t.Fatalf("trial %d：%08X 交出頁 %02X，Read8 %02X（A20 %t、planar %t、cplUser %t）",
						trial, base|off, page[off], want, m.a20, m.planarOn, m.cplUser)
				}
			}
			if _, ok := m.PageFault(); ok {
				t.Fatalf("trial %d：交出頁之後逐位移 Read8 出現頁錯誤", trial)
			}
		}
	}
	if handed == 0 {
		t.Fatal("沒有任何一次交出頁，測試沒有涵蓋到")
	}
}
