package machine

import (
	"bytes"
	"math/rand"
	"testing"
)

// 分頁微快取（paging.go 的 utlb）的差分測試：同一串隨機操作分別交給 readLin／writeLin（經微快取）
// 與參考實作（每次直接走 xlatPage，即加微快取之前的本體），每一步比回傳值與分頁錯誤紀錄，
// 最後比 Mem、Ext 與頁表寫回次數。操作穿插改 PTE（不作廢，測舊轉換語意）、SetCR3、SetA20 與
// user／監督者身分，線性頁跨過微快取大小以產生槽位碰撞。

func refReadLin(m *Machine, a uint32, user bool) uint8 {
	if m.paging {
		var ok bool
		if a, ok = m.xlatPage(a, false, user); !ok {
			return 0xFF
		}
	}
	if !m.a20 {
		a &^= 0x100000
	}
	return m.readPhysRaw(a)
}

func refWriteLin(m *Machine, a uint32, v uint8, user bool) {
	if m.paging {
		var ok bool
		if a, ok = m.xlatPage(a, true, user); !ok {
			return
		}
	}
	if !m.a20 {
		a &^= 0x100000
	}
	m.writePhysRaw(a, v)
}

func TestUTLBDifferential(t *testing.T) {
	for seed := int64(1); seed <= 8; seed++ {
		rng := rand.New(rand.NewSource(seed))
		// 線性頁：0–199（第一個頁目錄項，碰撞 utlbSize）與 1024–1099（第二個頁目錄項）。
		var pages []uint32
		for p := uint32(0); p < 200; p++ {
			pages = append(pages, p)
		}
		for p := uint32(1024); p < 1100; p++ {
			pages = append(pages, p)
		}
		randPTE := func() uint32 {
			phys := 0x100000 + uint32(rng.Intn(512))*0x1000 // Ext 的前 2 MB
			if rng.Intn(8) == 0 {
				phys = 0x80000 + uint32(rng.Intn(16))*0x1000 // 低記憶體
			}
			flags := uint32(rng.Intn(8)) // P、W、U 各自隨機
			if rng.Intn(4) != 0 {
				flags |= fP
			}
			return phys | flags
		}
		mk := func() (*Machine, *tables) {
			m, tb := newPaged(t)
			return m, tb
		}
		a, ta := mk()
		b, tb := mk()
		for _, p := range pages {
			v := randPTE()
			ta.mapFlags(p<<12, 0, fPWU, 0)
			tb.mapFlags(p<<12, 0, fPWU, 0)
			ta.setPTE(p<<12, v)
			tb.setPTE(p<<12, v)
		}
		// 低記憶體與 Ext 填同一組隨機內容。
		rng2 := rand.New(rand.NewSource(seed * 7919))
		for i := 0x80000; i < 0x90000; i++ {
			v := uint8(rng2.Intn(256))
			a.Mem[i], b.Mem[i] = v, v
		}
		for i := 0; i < 0x200000; i++ {
			v := uint8(rng2.Intn(256))
			a.Ext[i], b.Ext[i] = v, v
		}
		ta.enable(t)
		tb.enable(t)
		for op := 0; op < 200000; op++ {
			lin := pages[rng.Intn(len(pages))]<<12 | uint32(rng.Intn(0x1000))
			user := rng.Intn(2) == 0
			switch k := rng.Intn(100); {
			case k < 50:
				va, vb := a.readLin(lin, user), refReadLin(b, lin, user)
				if va != vb {
					t.Fatalf("seed %d op %d：讀 %08X user=%t 得 %02X，參考 %02X", seed, op, lin, user, va, vb)
				}
			case k < 90:
				v := uint8(rng.Intn(256))
				a.writeLin(lin, v, user)
				refWriteLin(b, lin, v, user)
			case k < 96:
				v := randPTE()
				ta.setPTE(lin, v)
				tb.setPTE(lin, v)
			case k < 98:
				if err := a.SetCR3(ta.dir); err != nil {
					t.Fatal(err)
				}
				if err := b.SetCR3(tb.dir); err != nil {
					t.Fatal(err)
				}
			default:
				on := rng.Intn(2) == 0
				a.SetA20(on)
				b.SetA20(on)
			}
			ra, oka := a.PageFault()
			rb, okb := b.PageFault()
			if oka != okb || ra != rb {
				t.Fatalf("seed %d op %d：錯誤紀錄 %v %v，參考 %v %v", seed, op, oka, ra, okb, rb)
			}
			a.clearPageFaults()
			b.clearPageFaults()
		}
		if !bytes.Equal(a.Mem, b.Mem) || !bytes.Equal(a.Ext, b.Ext) {
			t.Fatalf("seed %d：記憶體內容不同", seed)
		}
		if a.PageTableWrites != b.PageTableWrites {
			t.Fatalf("seed %d：頁表寫回 %d 次，參考 %d 次", seed, a.PageTableWrites, b.PageTableWrites)
		}
	}
}

// 改 PTE 而不作廢時保留舊轉換；SetCR3 作廢後應讀到新實體頁。
func TestUTLBInvalidatedByCR3(t *testing.T) {
	m, tb := newPaged(t)
	tb.mapPage(0x5000, 0x100000)
	tb.enable(t)
	m.Ext[0], m.Ext[0x1000] = 0x11, 0x22
	if v := m.readLin(0x5000, false); v != 0x11 {
		t.Fatalf("讀到 %02X", v)
	}
	tb.mapPage(0x5000, 0x101000)
	if v := m.readLin(0x5000, false); v != 0x11 {
		t.Fatalf("未作廢前應保留舊轉換，讀到 %02X", v)
	}
	if err := m.SetCR3(tb.dir); err != nil {
		t.Fatal(err)
	}
	if v := m.readLin(0x5000, false); v != 0x22 {
		t.Fatalf("SetCR3 之後應讀到新頁，讀到 %02X", v)
	}
}
