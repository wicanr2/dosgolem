package machine

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// docs/spec/432-linear-memory-and-paging §3.1：實體層的差分測試與頁表存取。
//
// refRead8／refWrite8／refPeek8 是 432 之前 Read8／Write8／Peek8 的原樣本體（HMA 判斷、
// &0xFFFFF 之後的 VGA 視窗、ROM 保護、讀寫監看、影片寫入紀錄、Ext）。新的轉換關閉路徑
// 改走 readPhysRaw／writePhysRaw 之後，對隨機位址與各種監看、平面模式組合，兩條路徑
// 的回傳值、記憶體、VGA 平面、各種紀錄與回呼序列都必須逐位元組相同。

func refRead8(m *Machine, a uint32) uint8 {
	if off, ok := m.hmaAddr(a); ok {
		if m.Ext != nil {
			return m.Ext[off]
		}
		return m.hma[off]
	}
	a &= 0xFFFFF
	if m.planarOn && a >= vgaLo && a < vgaHi {
		return m.VGA.Read(a - vgaLo)
	}
	if m.rWatchLo <= a && a <= m.rWatchHi {
		m.onRead(a, m.Mem[a])
	}
	return m.Mem[a]
}

func refWrite8(m *Machine, a uint32, v uint8) {
	if off, ok := m.hmaAddr(a); ok {
		if m.Ext != nil {
			m.Ext[off] = v
			return
		}
		m.hma[off] = v
		return
	}
	a &= 0xFFFFF
	if a >= ROMBase {
		m.noteROMWrite(a, v)
		return
	}
	if m.onVideoWrite != nil && a >= vgaLo && a < vgaHi {
		cs, ip := m.CPU.OpAddr()
		m.onVideoWrite(VideoWrite{Step: m.Steps, CS: cs, IP: ip, Offset: a - vgaLo, Value: v, WriteMode: m.VGA.WriteMode(),
			Ctx: m.videoWriteCtx()}) // 458 §2.4 第 8 條：與 writePhysRaw 共用同一支組裝函式
	}
	if m.planarOn && a >= vgaLo && a < vgaHi {
		stride := m.planarStride()
		if m.RowWritesFrom > 0 && m.Steps >= m.RowWritesFrom {
			if r := int(a-vgaLo) / stride; r < len(m.VideoRowWrites) {
				m.VideoRowWrites[r]++
			}
		}
		m.WriteModeUse[m.VGA.WriteMode()]++
		if m.VRAMSites != nil && (m.VRAMAt < 0 || uint32(m.VRAMAt) == a-vgaLo) {
			cs, ip := m.CPU.OpAddr()
			m.VRAMSites[uint32(cs)<<16|uint32(ip)]++
		}
		if m.VGATraceRow1 > m.VGATraceRow0 && m.Steps >= m.RowWritesFrom {
			off := int(a - vgaLo)
			r, c := off/stride, off%stride
			inCol := m.VGATraceCol1 == 0 || (c >= m.VGATraceCol0 && c < m.VGATraceCol1)
			if r >= m.VGATraceRow0 && r < m.VGATraceRow1 && inCol {
				m.VGATrace = append(m.VGATrace, m.vgaSnap(a-vgaLo, v, r))
				if len(m.VGATrace) > 40 {
					m.VGATrace = m.VGATrace[1:]
				}
			}
		}
		m.VGA.Write(a-vgaLo, v)
		return
	}
	if m.watchLo <= a && a <= m.watchHi && m.Mem[a] != v {
		m.onWrite(a, m.Mem[a], v)
	}
	if len(m.writeWatches) > 0 {
		old := m.Mem[a]
		m.Mem[a] = v
		m.noteWrite(a, old, v)
		return
	}
	m.Mem[a] = v
}

func refPeek8(m *Machine, a uint32) uint8 {
	if off, ok := m.hmaAddr(a); ok {
		if m.Ext != nil {
			return m.Ext[off]
		}
		return m.hma[off]
	}
	a &= 0xFFFFF
	if m.planarOn && a >= vgaLo && a < vgaHi {
		return 0
	}
	return m.Mem[a]
}

// 430 的 ReadPhys8／WritePhys8（a < 1 MB 時呼叫舊 Read8／Write8）。
func refReadPhys8(m *Machine, a uint32) uint8 {
	if a < ExtBase {
		return refRead8(m, a)
	}
	if off := a - ExtBase; off < uint32(len(m.Ext)) {
		return m.Ext[off]
	}
	return 0xFF
}

func refWritePhys8(m *Machine, a uint32, v uint8) {
	if a < ExtBase {
		refWrite8(m, a, v)
		return
	}
	if off := a - ExtBase; off < uint32(len(m.Ext)) {
		m.Ext[off] = v
		return
	}
	m.UnmappedPhysWrites++
}

// diffRig 是一台受測機器與它的事件紀錄。
type diffRig struct {
	m   *Machine
	log []string
}

// newDiffRig 依 cfg 建一台機器，兩台用相同的 cfg 與初始內容。
func newDiffRig(t *testing.T, seed int64, ext, a20 bool, mode uint8, watch int) *diffRig {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	r := &diffRig{m: New()}
	m := r.m
	if ext {
		if err := m.SetExtendedMemoryKB(4096); err != nil {
			t.Fatal(err)
		}
	}
	m.SetA20(a20)
	m.SetVideoMode(mode)
	for i := range m.Mem[:ROMBase] {
		m.Mem[i] = uint8(rng.Intn(256))
	}
	for i := range m.Ext {
		m.Ext[i] = uint8(rng.Intn(256))
	}
	for i := range m.hma {
		m.hma[i] = uint8(rng.Intn(256))
	}
	// 監看：隨機區間，涵蓋低記憶體、VGA 視窗、ROM。watch 的三個位元分別開關
	// 讀取監看、WatchWrites、WatchWrite；全關時走 Read8／Write8 開頭的熱路徑捷徑。
	// 區間以 0xA0000 附近為中心，讓捷徑的邊界（vgaLo）也被監看覆蓋到。
	lo := uint32(0x9C000 + rng.Intn(0x8000))
	if watch&1 != 0 {
		m.WatchReads(lo, lo+uint32(rng.Intn(0x4000)), func(a uint32, v uint8) {
			r.log = append(r.log, fmt.Sprintf("R %X %02X", a, v))
		})
	}
	lo = uint32(0x9C000 + rng.Intn(0x8000))
	if watch&2 != 0 {
		m.WatchWrites(lo, lo+uint32(rng.Intn(0x4000)), func(a uint32, o, n uint8) {
			r.log = append(r.log, fmt.Sprintf("W %X %02X>%02X", a, o, n))
		})
	}
	lo = uint32(0x9C000 + rng.Intn(0x8000))
	if watch&4 != 0 {
		m.WatchWrite(lo, lo+uint32(rng.Intn(0x4000)), func(_ *Machine, a uint32, o, n uint8) {
			r.log = append(r.log, fmt.Sprintf("WW %X %02X>%02X", a, o, n))
		})
	}
	m.ObserveVideoWrites(func(w VideoWrite) { r.log = append(r.log, fmt.Sprintf("V %+v", w)) })
	m.VRAMSites = map[uint32]uint64{}
	m.VRAMAt = -1
	m.RowWritesFrom = 1
	m.VGATraceRow0, m.VGATraceRow1 = 0, 50
	m.Steps = 5
	return r
}

// diffAddrs 產生偏重邊界與特殊區域的隨機位址。
func diffAddrs(rng *rand.Rand, n int) []uint32 {
	special := []uint32{
		0, 0xFFFFF, 0xA0000, 0xAFFFF, 0xB0000, 0x9FFFF, ROMBase, ROMBase - 1, ROMBase + 1,
		MemSize, MemSize + 1, MemSize + HMASize - 1, MemSize + HMASize, MemSize + HMASize + 1,
		0x10FFEF, 0x10FFF0, 0x1FFFFF, 0x4FFFFF, 0x500000, 0x400000 + 1, 0xFFFFFFFF, 0x80000000,
	}
	out := make([]uint32, n)
	for i := range out {
		switch rng.Intn(6) {
		case 0:
			out[i] = special[rng.Intn(len(special))]
		case 1:
			out[i] = uint32(rng.Intn(0x100000))
		case 2:
			out[i] = vgaLo + uint32(rng.Intn(0x10000))
		case 3:
			out[i] = uint32(rng.Intn(0x600000))
		case 4:
			out[i] = ROMBase - 8 + uint32(rng.Intn(16))
		default:
			out[i] = rng.Uint32()
		}
	}
	return out
}

func diffSnapshot(r *diffRig) map[string]any {
	m := r.m
	return map[string]any{
		"mem": append([]uint8(nil), m.Mem...), "ext": append([]uint8(nil), m.Ext...),
		"hma": m.hma, "planes": append([]uint8(nil), m.VGA.Raw()...), "latch": m.VGA.latch,
		"romw": m.ROMWrites, "romlog": append([]ROMWrite(nil), m.ROMWriteLog...),
		"wmu": m.WriteModeUse, "rows": m.VideoRowWrites, "sites": m.VRAMSites,
		"trace": append([]VGAWrite(nil), m.VGATrace...), "unmapped": m.UnmappedPhysWrites,
		"log": append([]string(nil), r.log...),
	}
}

// TestPhysLayerDifferentialAgainstOldBody 是 432 §9 第 1 項的隨機差分測試：
// 轉換關閉時，新路徑（Read8／Write8／ReadPhys*／WritePhys*／Peek8 → 實體層）
// 與 432 之前的本體逐位元組相同。
func TestPhysLayerDifferentialAgainstOldBody(t *testing.T) {
	for _, mode := range []uint8{0x03, 0x0D, 0x13} {
		for _, ext := range []bool{false, true} {
			for _, a20 := range []bool{false, true} {
				for watch := 0; watch < 8; watch++ {
					seed := int64(watch + 1)
					name := fmt.Sprintf("mode%02X/ext=%v/a20=%v/watch%d", mode, ext, a20, watch)
					t.Run(name, func(t *testing.T) {
						oldR := newDiffRig(t, seed, ext, a20, mode, watch)
						newR := newDiffRig(t, seed, ext, a20, mode, watch)
						rng := rand.New(rand.NewSource(seed * 7919))
						addrs := diffAddrs(rng, 20000)
						for i, a := range addrs {
							v := uint8(rng.Intn(256))
							var gotO, gotN uint8
							switch rng.Intn(6) {
							case 0:
								gotO, gotN = refRead8(oldR.m, a), newR.m.Read8(a)
							case 1:
								refWrite8(oldR.m, a, v)
								newR.m.Write8(a, v)
							case 2:
								gotO, gotN = refReadPhys8(oldR.m, a), newR.m.ReadPhys8(a)
							case 3:
								refWritePhys8(oldR.m, a, v)
								newR.m.WritePhys8(a, v)
							case 4:
								gotO, gotN = refPeek8(oldR.m, a), newR.m.Peek8(a)
							default:
								// 16 位元：逐 byte，位址各自歸類。
								wo := uint16(refRead8(oldR.m, a)) | uint16(refRead8(oldR.m, a+1))<<8
								wn := newR.m.Read16(a)
								gotO, gotN = uint8(wo), uint8(wn)
								if wo != wn {
									t.Fatalf("#%d Read16(%X)：舊 %04X 新 %04X", i, a, wo, wn)
								}
							}
							if gotO != gotN {
								t.Fatalf("#%d 位址 %X：舊 %02X 新 %02X", i, a, gotO, gotN)
							}
						}
						so, sn := diffSnapshot(oldR), diffSnapshot(newR)
						for k := range so {
							if !reflect.DeepEqual(so[k], sn[k]) {
								t.Errorf("%s 不同", k)
							}
						}
					})
				}
			}
		}
	}
}

// 熱路徑捷徑（Read8／Write8 開頭的 Mem[a] 直通）在監看開啟時必須讓位給完整本體。
func TestFastPathYieldsToWatches(t *testing.T) {
	m := New()
	var reads, writes, wwrites int
	m.Write8(0x500, 7)
	m.WatchReads(0x500, 0x500, func(uint32, uint8) { reads++ })
	if m.Read8(0x500) != 7 || reads != 1 {
		t.Errorf("讀取監看：reads=%d", reads)
	}
	m.WatchReads(0, 0, nil)
	m.Read8(0x500)
	if reads != 1 {
		t.Errorf("關閉監看後不該再通知：reads=%d", reads)
	}
	m.WatchWrites(0x600, 0x600, func(uint32, uint8, uint8) { writes++ })
	m.Write8(0x600, 1)
	m.Write8(0x600, 1) // 值沒變，不通知
	if writes != 1 {
		t.Errorf("寫入監看：writes=%d", writes)
	}
	m.WatchWrites(0, 0, nil)
	id := m.WatchWrite(0x700, 0x700, func(*Machine, uint32, uint8, uint8) { wwrites++ })
	m.Write8(0x700, 1)
	m.Write8(0x700, 1) // 值沒變也算一次寫入
	if wwrites != 2 {
		t.Errorf("WatchWrite：wwrites=%d，值沒變也要算", wwrites)
	}
	m.Unwatch(id)
	m.Write8(0x700, 2)
	if wwrites != 2 {
		t.Errorf("Unwatch 之後不該再通知：%d", wwrites)
	}
}

// 頁表專用存取：直接讀寫 Mem／Ext，不經 ROM 保護與監看；範圍外讀 FFFFFFFFh、寫入丟棄。
func TestPageTableAccessBypassesDevices(t *testing.T) {
	m := newExt(t, 4096)
	var seen int
	m.WatchWrites(0, 0xFFFFF, func(uint32, uint8, uint8) { seen++ })
	m.WatchReads(0, 0xFFFFF, func(uint32, uint8) { seen++ })

	m.pageWrite32(0x2000, 0x11223344)
	if got := m.pageRead32(0x2000); got != 0x11223344 {
		t.Errorf("pageRead32 = %08X", got)
	}
	if m.Mem[0x2000] != 0x44 || m.Mem[0x2003] != 0x11 {
		t.Error("應為小端、直接落在 Mem")
	}
	// ROM 區：頁表走訪不經 ROM 保護。
	m.pageWrite32(ROMBase+0x100, 0xAABBCCDD)
	if m.Mem[ROMBase+0x100] != 0xDD || m.ROMWrites != 0 {
		t.Errorf("頁表寫入不應經 ROM 保護：Mem=%02X ROMWrites=%d", m.Mem[ROMBase+0x100], m.ROMWrites)
	}
	// 跨 1 MB 邊界：各 byte 各自歸類。
	m.pageWrite32(MemSize-2, 0x04030201)
	if m.Mem[MemSize-2] != 1 || m.Mem[MemSize-1] != 2 || m.Ext[0] != 3 || m.Ext[1] != 4 {
		t.Error("跨 1 MB 的 32 位元存取，各 byte 應分別落在 Mem 與 Ext")
	}
	if got := m.pageRead32(MemSize - 2); got != 0x04030201 {
		t.Errorf("跨界讀回 %08X", got)
	}
	// 範圍外。
	extEnd := uint32(ExtBase + len(m.Ext))
	if got := m.pageRead32(extEnd); got != 0xFFFFFFFF {
		t.Errorf("Ext 之外讀回 %08X，應為 FFFFFFFF", got)
	}
	before := m.UnmappedPhysWrites
	m.pageWrite32(extEnd, 0)
	if m.UnmappedPhysWrites != before {
		t.Error("頁表寫入範圍外是丟棄，不計 UnmappedPhysWrites")
	}
	if m.PageTableWrites != 4 {
		t.Errorf("PageTableWrites = %d，應為 4（每次 pageWrite32 一次，含丟棄的）", m.PageTableWrites)
	}
	if seen != 0 {
		t.Errorf("頁表存取不得觸發監看（%d 次）", seen)
	}
}
