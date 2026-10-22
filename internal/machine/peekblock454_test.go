package machine

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// docs/spec/454-session-view-block-read-and-digest §3.1 第 8 項：PeekBlock 與逐位元組 PeekOK 等價、沒有副作用；
// §3.2 第 3 項的 BenchmarkPeekBlock64000。

// wantPeekBlock 以逐位元組 PeekOK 當期望值，比對 PeekBlock 的內容與 ok。dst 先填哨兵，確認每個位元組都被寫。
func wantPeekBlock(t *testing.T, m *Machine, what string, a uint32, n int) {
	t.Helper()
	want := make([]byte, n)
	wantOK := true
	for i := range want {
		v, ok := m.PeekOK(a + uint32(i))
		want[i] = v
		if !ok {
			wantOK = false
		}
	}
	got := bytes.Repeat([]byte{0x5A}, n+1)
	ok := m.PeekBlock(a, got[:n])
	if ok != wantOK {
		t.Errorf("%s：a=%08X n=%d：ok %v，逐位元組 %v", what, a, n, ok, wantOK)
	}
	if got[n] != 0x5A {
		t.Errorf("%s：a=%08X n=%d：寫出 dst 範圍", what, a, n)
	}
	if !bytes.Equal(got[:n], want) {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s：a=%08X n=%d：第一個不同在 +%X（線性 %08X）：%02X，逐位元組 %02X", what, a, n, i, a+uint32(i), got[i], want[i])
				break
			}
		}
	}
}

// fillRand 以固定種子填 Mem、hma 與 Ext。
func fillRand454(m *Machine, seed int64) {
	r := rand.New(rand.NewSource(seed))
	r.Read(m.Mem)
	r.Read(m.hma[:])
	if m.Ext != nil {
		r.Read(m.Ext)
	}
}

// 轉換關閉：低記憶體、跨 1 MB、HMA 尾端、A20 開關、Ext 有無、A0000–BFFFF（平面模式開與關）、uint32 環繞。
func TestPeekBlock454XlatOff(t *testing.T) {
	for _, ext := range []bool{false, true} {
		for _, a20 := range []bool{false, true} {
			for _, mode := range []uint8{0x03, 0x12} {
				m := New()
				if ext {
					mustNil(t, "Ext", m.SetExtendedMemoryKB(4096))
				}
				m.SetA20(a20)
				m.SetVideoMode(mode)
				fillRand454(m, 454)
				what := fmt.Sprintf("轉換關閉 ext=%v a20=%v mode=%02X planarOn=%v", ext, a20, mode, m.planarOn)
				for _, r := range []struct {
					a uint32
					n int
				}{
					{0x1234, 5000},
					{0xFFFF0, 64},       // 跨 0xFFFFF → 0x100000
					{0x10FFE0, 32},      // 跨 HMA 尾端 0x10FFEF
					{0xFFFF0, 0x10020},  // 低記憶體尾端 + 整個 HMA + 之後
					{0x9F000, 0x22000},  // A0000–BFFFF 整段與兩側
					{0x1FFFF0, 64},      // 2 MB 邊界（A20 開時不在 HMA，遮罩環繞）
					{0xFFFFFFF0, 32},    // uint32 環繞
					{0x0, 0},            // len 0
					{0x10FFF0, 0x20000}, // HMA 之後
				} {
					wantPeekBlock(t, m, what, r.a, r.n)
				}
				if !m.PeekBlock(0x1234, nil) {
					t.Errorf("%s：len 0 應回 true", what)
				}
			}
		}
	}
}

// 轉換開、分頁關：A20 開與關、平面模式下讀 A0000 視窗、跨 1 MB、超出 Ext。
// 規格列的「Ext 不存在」在轉換開啟時做不出來：SetProtectedModeProfile 要求先有延伸記憶體（沒有時回錯誤）。
func TestPeekBlock454XlatNoPaging(t *testing.T) {
	for _, ext := range []bool{true} {
		for _, a20 := range []bool{false, true} {
			for _, mode := range []uint8{0x13, 0x12} {
				m := New()
				if ext {
					mustNil(t, "Ext", m.SetExtendedMemoryKB(4096))
				}
				mustNil(t, "profile", m.SetProtectedModeProfile(true))
				m.SetVideoMode(mode)
				mustNil(t, "PE", m.SetCR0(cr0PE))
				m.SetA20(a20)
				fillRand454(m, 4541)
				if !m.xlat || m.paging {
					t.Fatal("前提：轉換開、分頁關")
				}
				what := fmt.Sprintf("轉換開分頁關 ext=%v a20=%v mode=%02X", ext, a20, mode)
				for _, r := range []struct {
					a uint32
					n int
				}{
					{0x1234, 5000}, {0xFFFF0, 64}, {0x10FFE0, 32}, {0x9F800, 0x12000},
					{0x100000, 0x3000}, {0x1FF800, 0x1000}, {0x4FF000, 0x2000}, {0xFFFFFFF0, 32},
				} {
					wantPeekBlock(t, m, what, r.a, r.n)
				}
			}
		}
	}
}

// V86 分頁：跨兩頁且實體不連續；含不存在的頁（ok 為假、FFh）；改寫頁表項後不沖 TLB（兩者都看到舊轉換）；
// uint32 環繞；len 0。
func TestPeekBlock454V86Paging(t *testing.T) {
	// 先以亂數填記憶體，再建頁表（頁目錄 70000h，頁表往上配），最後開分頁與 VM。
	m := New()
	mustNil(t, "Ext", m.SetExtendedMemoryKB(4096))
	mustNil(t, "profile", m.SetProtectedModeProfile(true))
	m.IRQ0Every = 0
	fillRand454(m, 4542)
	m.SetA20(true)
	tb := newTables(m, 0x70000)
	tb.identity(0, 0x100000, fPWU)
	tb.enable(t)
	mustNil(t, "SetVM", m.SetVM(true))
	const L = 0x400000 // 第 2 個 4 MB：自己的頁表
	tb.mapPage(L, 0x250000)
	tb.mapPage(L+0x1000, 0x180000) // 不連續、比前一頁低
	tb.mapPage(L+0x2000, 0x0A0000) // 實體 A0000（mode 03h：Mem）
	tb.setPTE(L+0x3000, 0)         // 不存在
	tb.mapPage(L+0x4000, 0x251000)
	tb.mapPage(0xFFFFF000, 0x260000)
	what := "V86 分頁"
	wantPeekBlock(t, m, what, L+0x0F00, 0x200)   // 跨兩頁，實體不連續
	wantPeekBlock(t, m, what, L+0x0800, 0x4000)  // 含不存在的頁
	wantPeekBlock(t, m, what, L+0x3000, 0x1000)  // 整頁不存在
	wantPeekBlock(t, m, what, 0xFFFFFF80, 0x100) // uint32 環繞到線性 0
	wantPeekBlock(t, m, what, L, 0)
	if !m.PeekBlock(L+0x3000, nil) {
		t.Error("len 0 應回 true")
	}
	buf := make([]byte, 0x1000)
	if m.PeekBlock(L+0x3000, buf) || buf[0] != 0xFF || buf[0xFFF] != 0xFF {
		t.Error("不存在的頁應回 false 並填 FFh")
	}

	// TLB 有世代符合的舊條目：先以 CPU 視角讀（填 TLB），改寫 PTE 不沖 TLB，PeekBlock 與 PeekOK 都看到舊轉換。
	_ = m.Read8(L + 0x10)
	_ = m.Read8(L + 0x1010)
	tb.setPTE(L, 0x300000|fPWU)
	tb.setPTE(L+0x1000, 0x310000|fPWU)
	wantPeekBlock(t, m, what+"（TLB 未沖）", L+0x0F00, 0x200)
	old := make([]byte, 0x20)
	m.PeekBlock(L+0xFF0, old)
	if !bytes.Equal(old[:0x10], m.Ext[0x250FF0-ExtBase:0x251000-ExtBase]) || !bytes.Equal(old[0x10:], m.Ext[0x180000-ExtBase:0x180010-ExtBase]) {
		t.Error("TLB 未沖時應看到舊轉換（250000h、180000h）")
	}

	// 隨機差分：分頁下隨機起點與長度。
	r := rand.New(rand.NewSource(45421))
	for i := 0; i < 200; i++ {
		a := L - 0x3000 + uint32(r.Intn(0xA000))
		wantPeekBlock(t, m, what+"（隨機）", a, r.Intn(0x6000))
	}
}

// 轉換關閉與開啟（無分頁）的隨機差分：隨機起點（偏重邊界）與長度（至多約 70000）。
func TestPeekBlock454Random(t *testing.T) {
	r := rand.New(rand.NewSource(45422))
	starts := []uint32{0, 0x9F000, 0xA0000, 0xAFFF0, 0xFFF00, 0x100000, 0x10FF00, 0x110000, 0x1FFF00, 0x4FFF00, 0xFFFFF000}
	for k := 0; k < 16; k++ {
		ext, a20, xlat, planar := k&1 != 0, k&2 != 0, k&4 != 0, k&8 != 0
		if xlat && !ext {
			continue // 轉換開啟需要延伸記憶體
		}
		m := New()
		if ext {
			mustNil(t, "Ext", m.SetExtendedMemoryKB(4096))
		}
		if planar {
			m.SetVideoMode(0x12)
		} else {
			m.SetVideoMode(0x13)
		}
		if xlat {
			mustNil(t, "profile", m.SetProtectedModeProfile(true))
			mustNil(t, "PE", m.SetCR0(cr0PE))
		}
		m.SetA20(a20)
		fillRand454(m, int64(k))
		what := fmt.Sprintf("隨機 ext=%v a20=%v xlat=%v planar=%v", ext, a20, xlat, planar)
		for i := 0; i < 60; i++ {
			a := starts[r.Intn(len(starts))] + uint32(r.Intn(0x200)) - 0x100
			wantPeekBlock(t, m, what, a, r.Intn(70000))
		}
	}
}

// 白箱：呼叫前後 TLB、utlb、世代、頁表項 accessed／dirty、讀取監看、分頁錯誤紀錄、VGA latch 不變。
func TestPeekBlock454NoSideEffects(t *testing.T) {
	m, tb := newPagedProg(t, []byte{0x90}, fP|fW|fU)
	mustNil(t, "SetVM", m.SetVM(true))
	tb.mapFlags(0x400000, 0x200000, fPWU, fPWU) // accessed=0，且從未經 CPU 存取（不在 TLB、utlb）
	tb.mapFlags(0x401000, 0x201000, fPWU, fPWU)
	_ = m.Read8(0x500) // TLB、utlbR 有條目
	reads := 0
	m.WatchReads(0, 0xFFFFF, func(uint32, uint8) { reads++ })
	type snap454 struct {
		tlb        [][1024]tlbEntry
		utlbR      [utlbSize]utlbEntry
		utlbW      [utlbSize]utlbEntry
		gen        uint32
		pt         []byte
		ptw        uint64
		pf, pfLast PageFaultRecord
		ok1, ok2   bool
		latch      [4]uint8
	}
	take := func() snap454 {
		s := snap454{utlbR: m.utlbR, utlbW: m.utlbW, gen: m.tlbGen, ptw: m.PageTableWrites,
			pf: m.pf, pfLast: m.pfLast, ok1: m.pfOK, ok2: m.pfLastOK, latch: m.VGA.latch}
		if m.tlb != nil {
			for _, b := range m.tlb {
				if b != nil {
					s.tlb = append(s.tlb, *b)
				}
			}
		}
		s.pt = append([]byte(nil), m.Mem[0x70000:0x80000]...)
		return s
	}
	before := take()
	dst := make([]byte, 0x3000)
	m.PeekBlock(0x3FF800, dst) // 跨 4 MB 邊界進新頁表、含 accessed=0 的頁
	m.PeekBlock(0x400, dst)
	after := take()
	if !reflect.DeepEqual(before, after) || reads != 0 {
		t.Errorf("PeekBlock 改了狀態：TLB／utlb／世代／頁表／錯誤紀錄／latch 相同=%v，讀取監看 %d 次",
			reflect.DeepEqual(before, after), reads)
	}
	if tb.pte(0x400000)&fA != 0 {
		t.Error("PeekBlock 不得寫 accessed")
	}
}

// BenchmarkPeekBlock64000：三種模式，PeekBlock 與逐位元組 PeekOK（§3.2 第 3 項：V86 分頁 ≤ 100 µs、快 5 倍以上）。
func BenchmarkPeekBlock64000(b *testing.B) {
	modes := []struct {
		name string
		mk   func(b *testing.B) (*Machine, uint32)
	}{
		{"實模式", func(b *testing.B) (*Machine, uint32) {
			m := New()
			return m, 0x20000
		}},
		{"線性", func(b *testing.B) (*Machine, uint32) {
			m, _ := newPaged(b)
			if err := m.SetCR0(cr0PE); err != nil {
				b.Fatal(err)
			}
			return m, 0x20000
		}},
		{"V86分頁", func(b *testing.B) (*Machine, uint32) {
			m, _ := newPagedProg(b, []byte{0x90}, fPWU)
			if err := m.SetVM(true); err != nil {
				b.Fatal(err)
			}
			return m, 0x20000
		}},
	}
	dst := make([]byte, 64000)
	for _, md := range modes {
		b.Run(md.name+"/PeekBlock", func(b *testing.B) {
			m, a := md.mk(b)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.PeekBlock(a, dst)
			}
		})
		b.Run(md.name+"/逐位元組", func(b *testing.B) {
			m, a := md.mk(b)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := range dst {
					dst[j], _ = m.PeekOK(a + uint32(j))
				}
			}
		})
	}
}
