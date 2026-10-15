package dos

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/430-physical-memory-and-xms-unification §5 第 4 項（XMS 實體化）的測試。
// 這些測試直接呼叫 d.xmsCall；第 5 項（掛鉤入口）用 Step 真的執行遠呼叫，見 xms_hook_test.go。

func newExtDOS(t *testing.T, kb int) (*machine.Machine, *DOS) {
	t.Helper()
	m := machine.New()
	if err := m.SetExtendedMemoryKB(kb); err != nil {
		t.Fatal(err)
	}
	d := New(m, t.TempDir())
	d.Install()
	d.freeSeg = 0x2000
	return m, d
}

// xmsCallRegs 呼叫 XMS 入口並回傳 AX、BX、DX。
func xmsCallRegs(m *machine.Machine, d *DOS, fn uint8, bx, dx uint16) (ax, rbx, rdx uint16) {
	m.CPU.R[cpu.AX] = uint16(fn)<<8 | 0x55
	m.CPU.R[cpu.BX] = bx
	m.CPU.R[cpu.DX] = dx
	d.xmsCall(m.CPU)
	return m.CPU.R[cpu.AX], m.CPU.R[cpu.BX], m.CPU.R[cpu.DX]
}

func mustOK(t *testing.T, what string, ax uint16, bx uint16) {
	t.Helper()
	if ax != 1 {
		t.Fatalf("%s：AX ＝ %d、BL ＝ %02X，預期成功", what, ax, uint8(bx))
	}
}

func mustFail(t *testing.T, what string, ax, bx uint16, code uint8) {
	t.Helper()
	if ax != 0 || uint8(bx) != code {
		t.Errorf("%s：AX ＝ %d、BL ＝ %02X，預期失敗 BL ＝ %02X", what, ax, uint8(bx), code)
	}
}

func xmsAlloc(t *testing.T, m *machine.Machine, d *DOS, kb uint16) uint16 {
	t.Helper()
	ax, _, dx := xmsCallRegs(m, d, 0x09, 0, kb)
	if ax != 1 {
		t.Fatalf("配置 %d KB 失敗，BL ＝ %02X", kb, uint8(m.CPU.R[cpu.BX]))
	}
	return dx
}

func xmsLock(t *testing.T, m *machine.Machine, d *DOS, h uint16) uint32 {
	t.Helper()
	ax, bx, dx := xmsCallRegs(m, d, 0x0C, 0, h)
	mustOK(t, "Lock", ax, bx)
	return uint32(dx)<<16 | uint32(bx)
}

func xmsFreeH(t *testing.T, m *machine.Machine, d *DOS, h uint16) {
	t.Helper()
	ax, bx, _ := xmsCallRegs(m, d, 0x0A, 0, h)
	mustOK(t, "Free", ax, bx)
}

// xmsQuery 回 AH=08h 的 (AX, DX, BL)。
func xmsQuery(m *machine.Machine, d *DOS) (largest, total uint16, bl uint8) {
	ax, bx, dx := xmsCallRegs(m, d, 0x08, 0, 0)
	return ax, dx, uint8(bx)
}

// xmsMoveCall 組描述子（DS:SI ＝ 3000:0000）並呼叫 AH=0Bh，回 (AX, BL)。
func xmsMoveCall(m *machine.Machine, d *DOS, n uint32, srcH uint16, srcOff uint32, dstH uint16, dstOff uint32) (uint16, uint8) {
	const seg = 0x3000
	base := cpu.Addr(seg, 0)
	m.Write16(base, uint16(n))
	m.Write16(base+2, uint16(n>>16))
	m.Write16(base+4, srcH)
	m.Write16(base+6, uint16(srcOff))
	m.Write16(base+8, uint16(srcOff>>16))
	m.Write16(base+0x0A, dstH)
	m.Write16(base+0x0C, uint16(dstOff))
	m.Write16(base+0x0E, uint16(dstOff>>16))
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = seg, 0
	m.CPU.R[cpu.BX] = 0x1234 // 成功時 BL 不被碰
	ax, bx, _ := xmsCallRegs(m, d, 0x0B, 0x1234, 0)
	return ax, uint8(bx)
}

func realPtr(seg, off uint16) uint32 { return uint32(seg)<<16 | uint32(off) }

// 驗收 1(a)：預設 int 2Fh AX=4310h 回 BX ＝ 0x40C；kb > 0 回 0x4B0。
func TestInt2F4310ReturnsCurrentEntry(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x2F, 0x4310)
	if m.CPU.Seg[cpu.ES] != machine.StubSeg || m.CPU.R[cpu.BX] != 0x40C {
		t.Errorf("預設 ES:BX ＝ %04X:%04X，預期 0080:040C", m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX])
	}
	m2, d2 := newExtDOS(t, 4096)
	call(m2, d2, 0x2F, 0x4310)
	if m2.CPU.Seg[cpu.ES] != machine.StubSeg || m2.CPU.R[cpu.BX] != 0x4B0 {
		t.Errorf("kb>0 ES:BX ＝ %04X:%04X，預期 0080:04B0", m2.CPU.Seg[cpu.ES], m2.CPU.R[cpu.BX])
	}
}

// AH=08h：總空間不含 HMA；用盡時 AX=0、BL=A0h。
func TestXMSExtQueryFree(t *testing.T) {
	m, d := newExtDOS(t, 8192)
	largest, total, bl := xmsQuery(m, d)
	if largest != 8128 || total != 8128 || bl != 0 {
		t.Errorf("空機器 AX/DX/BL ＝ %d/%d/%02X，預期 8128/8128/0（8192 KB 扣 64 KB HMA）", largest, total, bl)
	}
	h := xmsAlloc(t, m, d, 8128)
	largest, total, bl = xmsQuery(m, d)
	if largest != 0 || total != 0 || bl != 0xA0 {
		t.Errorf("用盡 AX/DX/BL ＝ %d/%d/%02X，預期 0/0/A0", largest, total, bl)
	}
	// 切成兩段：最大連續小於總量。
	xmsFreeH(t, m, d, h)
	a, b, c := xmsAlloc(t, m, d, 100), xmsAlloc(t, m, d, 200), xmsAlloc(t, m, d, 300)
	_, _ = a, c
	xmsFreeH(t, m, d, b)
	largest, total, _ = xmsQuery(m, d)
	if total != 8128-100-300 || largest != 8128-600 {
		t.Errorf("挖洞後 AX/DX ＝ %d/%d，預期 %d/%d", largest, total, 8128-600, 8128-400)
	}
}

// 配 5,000 KB 後 Lock 基底 ≥ 0x110000、≤ 0xFE0000、4 KB 對齊（SCCD 映像 0x37E 的比較）。
func TestXMSExtAllocLockAddress(t *testing.T) {
	m, d := newExtDOS(t, 8192)
	h := xmsAlloc(t, m, d, 5000)
	if m.CPU.R[cpu.AX] != 1 || uint8(m.CPU.R[cpu.BX]) != 0 {
		t.Errorf("成功時 BL ＝ %02X（DOSBox-X SET_RESULT 會把 BL 設為 0）", uint8(m.CPU.R[cpu.BX]))
	}
	addr := xmsLock(t, m, d, h)
	if addr < 0x110000 || addr > 0xFE0000 || addr%4096 != 0 {
		t.Errorf("Lock 基底 ＝ %#x", addr)
	}
	if addr != 0x110000 {
		t.Errorf("空機器的第一塊應在 0x110000，實得 %#x", addr)
	}
	// 兩區塊不重疊。
	h2 := xmsAlloc(t, m, d, 1000)
	a2 := xmsLock(t, m, d, h2)
	if !(a2 >= addr+5000*1024 || a2+1000*1024 <= addr) {
		t.Errorf("兩區塊重疊：%#x+5000K 與 %#x+1000K", addr, a2)
	}
}

func TestXMSExtWritePhysThenMoveReadsBack(t *testing.T) {
	m, d := newExtDOS(t, 8192)
	h := xmsAlloc(t, m, d, 16)
	addr := xmsLock(t, m, d, h)
	for i := uint32(0); i < 64; i++ {
		m.WritePhys8(addr+4096+i, uint8(0x80+i))
	}
	// XMS（偏移 4096）→ 傳統記憶體 3000:2000
	ax, bl := xmsMoveCall(m, d, 64, h, 4096, 0, realPtr(0x3000, 0x2000))
	if ax != 1 {
		t.Fatalf("Move 失敗 BL ＝ %02X", bl)
	}
	for i := uint32(0); i < 64; i++ {
		if got := m.Read8(0x30000 + 0x2000 + i); got != uint8(0x80+i) {
			t.Fatalf("byte %d ＝ %02X，預期 %02X", i, got, uint8(0x80+i))
		}
	}
	// 反方向：傳統記憶體 → XMS，再用 ReadPhys 驗。
	for i := uint32(0); i < 32; i++ {
		m.Write8(0x30000+0x3000+i, uint8(i^0x5A))
	}
	if ax, bl := xmsMoveCall(m, d, 32, 0, realPtr(0x3000, 0x3000), h, 100); ax != 1 {
		t.Fatalf("反向 Move 失敗 BL ＝ %02X", bl)
	}
	for i := uint32(0); i < 32; i++ {
		if got := m.ReadPhys8(addr + 100 + i); got != uint8(i^0x5A) {
			t.Fatalf("反向 byte %d ＝ %02X", i, got)
		}
	}
	// XMS → XMS
	h2 := xmsAlloc(t, m, d, 8)
	addr2 := xmsLock(t, m, d, h2)
	if ax, bl := xmsMoveCall(m, d, 32, h, 100, h2, 0); ax != 1 {
		t.Fatalf("XMS→XMS Move 失敗 BL ＝ %02X", bl)
	}
	if m.ReadPhys8(addr2+5) != uint8(5^0x5A) {
		t.Error("XMS→XMS 沒有搬到")
	}
	if len(d.XMSMoves) != 3 {
		t.Errorf("XMSMoves 記錄 %d 筆，預期 3", len(d.XMSMoves))
	}
}

// 釋放中間區塊後再配置落在洞內。
func TestXMSExtFreedHoleIsReused(t *testing.T) {
	m, d := newExtDOS(t, 8192)
	a := xmsAlloc(t, m, d, 100)
	b := xmsAlloc(t, m, d, 100)
	c := xmsAlloc(t, m, d, 100)
	aAddr, bAddr, cAddr := xmsLock(t, m, d, a), xmsLock(t, m, d, b), xmsLock(t, m, d, c)
	if !(aAddr < bAddr && bAddr < cAddr) {
		t.Fatalf("配置次序不是由低往高：%#x %#x %#x", aAddr, bAddr, cAddr)
	}
	// 解鎖才能放。
	if ax, bx, _ := xmsCallRegs(m, d, 0x0D, 0, b); ax != 1 {
		t.Fatalf("Unlock 失敗 BL ＝ %02X", uint8(bx))
	}
	xmsFreeH(t, m, d, b)
	n := xmsAlloc(t, m, d, 100)
	if got := xmsLock(t, m, d, n); got != bAddr {
		t.Errorf("再配置落在 %#x，預期洞 %#x", got, bAddr)
	}
}

func TestXMSExtOutOfSpaceAndHandles(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	// 4096 KB － 64 KB ＝ 4032 KB。
	h := xmsAlloc(t, m, d, 4032)
	ax, bx, dx := xmsCallRegs(m, d, 0x09, 0, 4)
	mustFail(t, "容量耗盡", ax, bx, 0xA0)
	if dx != 0 {
		t.Errorf("失敗時 DX（handle）＝ %d，DOSBox-X 為 0", dx)
	}
	xmsFreeH(t, m, d, h)

	// handle 用完：0 KB 區塊也占 handle（1..64）。
	for i := 1; i <= xmsMaxHandles; i++ {
		if got := xmsAlloc(t, m, d, 0); int(got) != i {
			t.Fatalf("第 %d 個 handle ＝ %d，應取最小的空 handle", i, got)
		}
	}
	ax, bx, _ = xmsCallRegs(m, d, 0x09, 0, 4)
	mustFail(t, "handle 用完", ax, bx, 0xA1)
	// 放掉 7 之後取到 7。
	xmsFreeH(t, m, d, 7)
	if got := xmsAlloc(t, m, d, 4); got != 7 {
		t.Errorf("釋放 7 後配置得 %d，預期 7", got)
	}
}

// 0 KB 區塊：不占頁，基底是 BestMatch(1)，不清零。
func TestXMSExtZeroKBBlock(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	m.WritePhys8(0x110000, 0xA5) // 舊內容，0 KB 配置不清零
	h := xmsAlloc(t, m, d, 0)
	if got := xmsLock(t, m, d, h); got != 0x110000 {
		t.Errorf("0 KB 區塊基底 ＝ %#x，預期 BestMatch(1) ＝ 0x110000", got)
	}
	if m.ReadPhys8(0x110000) != 0xA5 {
		t.Error("0 KB 配置不得清零")
	}
	// 不占頁：下一個 4 KB 區塊拿到同一頁。
	h2 := xmsAlloc(t, m, d, 4)
	if got := xmsLock(t, m, d, h2); got != 0x110000 {
		t.Errorf("0 KB 區塊不占頁，下一塊應在 0x110000，實得 %#x", got)
	}
	_, total, _ := xmsQuery(m, d)
	if total != 4096-64-4 {
		t.Errorf("總空間 ＝ %d，0 KB 區塊不減空間", total)
	}
	// 沒有空頁時基底為 0。
	m3, d3 := newExtDOS(t, 4096)
	big := xmsAlloc(t, m3, d3, 4032)
	_ = big
	z := xmsAlloc(t, m3, d3, 0)
	if got := xmsLock(t, m3, d3, z); got != 0 {
		t.Errorf("沒有空頁時 0 KB 區塊基底 ＝ %#x，預期 0", got)
	}
}

func TestXMSExtFree(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	ax, bx, _ := xmsCallRegs(m, d, 0x0A, 0, 9)
	mustFail(t, "Free 無效 handle", ax, bx, 0xA2)
	ax, bx, _ = xmsCallRegs(m, d, 0x0A, 0, 0)
	mustFail(t, "Free handle 0", ax, bx, 0xA2)
	h := xmsAlloc(t, m, d, 8)
	xmsLock(t, m, d, h)
	ax, bx, _ = xmsCallRegs(m, d, 0x0A, 0, h)
	mustFail(t, "Free 已鎖定", ax, bx, 0xAB)
	xmsCallRegs(m, d, 0x0D, 0, h)
	xmsFreeH(t, m, d, h)
	ax, bx, _ = xmsCallRegs(m, d, 0x0A, 0, h)
	mustFail(t, "Free 第二次", ax, bx, 0xA2)
}

// 0Bh 的各種錯誤：回對應碼，且鄰近區塊與 HMA 不變。
func TestXMSExtMoveErrors(t *testing.T) {
	m, d := newExtDOS(t, 8192)
	h1 := xmsAlloc(t, m, d, 5) // 2 頁，但請求 5 KB
	h2 := xmsAlloc(t, m, d, 8)
	a1, a2 := xmsLock(t, m, d, h1), xmsLock(t, m, d, h2)
	// 鄰近內容與 HMA 填 sentinel。
	m.SetA20(true)
	for i := uint32(0); i < 8192; i++ {
		m.WritePhys8(a1+i, 0xE1)
		m.WritePhys8(a2+i, 0xE2)
	}
	m.WritePhys8(0x100000, 0xE3)
	m.WritePhys8(0x10FFEF, 0xE4)
	for i := uint32(0); i < 64; i++ {
		m.Write8(0x40000+i, 0xC0) // 描述子在 3000:0000，sentinel 放別處
	}
	snap := func() []byte {
		out := append([]byte(nil), m.Ext...)
		return append(out, m.Mem[0x40000:0x40040]...)
	}
	before := snap()

	big := uint32(5 * 1024)
	cases := []struct {
		name   string
		n      uint32
		sh     uint16
		so     uint32
		dh     uint16
		do     uint32
		code   uint8
		wantOK bool
	}{
		{"長度奇數", 3, h1, 0, h2, 0, 0xA7, false},
		{"來源 handle 無效", 4, 50, 0, h2, 0, 0xA3, false},
		{"來源 handle 0 以外的未配置", 4, 63, 0, h2, 0, 0xA3, false},
		{"來源位移＝區塊大小（5 KB 不是 8 KB）", 2, h1, big, h2, 0, 0xA4, false},
		{"來源長度超過剩餘（以請求的 KB 計）", big + 2, h1, 0, h2, 0, 0xA7, false},
		{"來源長度超過剩餘（位移後）", 4, h1, big - 2, h2, 0, 0xA7, false},
		{"目的 handle 無效", 4, h1, 0, 50, 0, 0xA5, false},
		{"目的位移 ≥ 區塊大小", 2, h1, 0, h2, 8 * 1024, 0xA6, false},
		{"目的長度超過剩餘", 4, h1, 0, h2, 8*1024 - 2, 0xA7, false},
		{"來源與目的都錯：回來源的碼", 4, 50, 0, 51, 0, 0xA3, false},
		{"來源位移與目的 handle 都錯：回來源位移", 4, h1, big, 51, 0, 0xA4, false},
		{"實模式來源超過 0x10FFF0", 4, 0, realPtr(0xFFFF, 0xFFFF), h2, 0, 0xA7, false},
		{"實模式目的超過 0x10FFF0", 4, h1, 0, 0, realPtr(0xFFFF, 0x0000), 0xA7, false},
	}
	// 最後一項：FFFF:0000 ＝ 0xFFFF0，加長度 4 ＝ 0xFFFF4 ≤ 0x10FFF0，其實合法；改成明確越界。
	cases[len(cases)-1].do = realPtr(0xFFFF, 0xFFFF) // 0x10FFEF + 4 > 0x10FFF0
	for _, tc := range cases {
		ax, bl := xmsMoveCall(m, d, tc.n, tc.sh, tc.so, tc.dh, tc.do)
		if ax != 0 || bl != tc.code {
			t.Errorf("%s：AX ＝ %d、BL ＝ %02X，預期失敗 %02X", tc.name, ax, bl, tc.code)
		}
		if !bytes.Equal(before, snap()) {
			t.Errorf("%s：失敗的 Move 不得改動 Ext 或低記憶體", tc.name)
		}
	}
	// 邊界上的合法呼叫：長度剛好等於請求的 5 KB；A20 在複製前後不變。
	m.SetA20(false)
	if ax, bl := xmsMoveCall(m, d, big, h1, 0, h2, 0); ax != 1 {
		t.Errorf("剛好 5 KB 應成功，BL ＝ %02X", bl)
	}
	if m.A20Enabled() {
		t.Error("Move 結束必須還原 A20（原本關閉）")
	}
}

func TestXMSExtMoveSuccessLeavesBL(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	h := xmsAlloc(t, m, d, 4)
	m.CPU.R[cpu.BX] = 0x1234
	m.Write16(cpu.Addr(0x3000, 0), 2)
	m.Write16(cpu.Addr(0x3000, 2), 0)
	m.Write16(cpu.Addr(0x3000, 4), h)
	m.Write16(cpu.Addr(0x3000, 6), 0)
	m.Write16(cpu.Addr(0x3000, 8), 0)
	m.Write16(cpu.Addr(0x3000, 0x0A), h)
	m.Write16(cpu.Addr(0x3000, 0x0C), 2)
	m.Write16(cpu.Addr(0x3000, 0x0E), 0)
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = 0x3000, 0
	m.CPU.R[cpu.AX] = 0x0B00
	d.xmsCall(m.CPU)
	if m.CPU.R[cpu.AX] != 1 || m.CPU.R[cpu.BX] != 0x1234 {
		t.Errorf("成功時 AX/BX ＝ %d/%04X；0Bh 成功不碰 BL（DOSBox-X SET_RESULT(…, false)）", m.CPU.R[cpu.AX], m.CPU.R[cpu.BX])
	}
}

// 重疊搬移：逐 byte 由低位址往高位址複製，不做 memmove。
func TestXMSExtMoveOverlapIsByteForward(t *testing.T) {
	pattern := []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'}
	want := []byte{'a', 'b', 'a', 'b', 'a', 'b', 'a', 'b'} // 前進複製的結果；memmove 會得到 abcdef…

	t.Run("傳統記憶體", func(t *testing.T) {
		m, d := newExtDOS(t, 4096)
		m.WriteBytes(0x40000, pattern)
		if ax, bl := xmsMoveCall(m, d, 6, 0, realPtr(0x4000, 0), 0, realPtr(0x4000, 2)); ax != 1 {
			t.Fatalf("BL ＝ %02X", bl)
		}
		if got := m.Mem[0x40000:0x40008]; !bytes.Equal(got, want) {
			t.Errorf("得 %q，預期 %q", got, want)
		}
	})
	t.Run("同一個 XMS 區塊", func(t *testing.T) {
		m, d := newExtDOS(t, 4096)
		h := xmsAlloc(t, m, d, 4)
		addr := xmsLock(t, m, d, h)
		for i, b := range pattern {
			m.WritePhys8(addr+uint32(i), b)
		}
		if ax, bl := xmsMoveCall(m, d, 6, h, 0, h, 2); ax != 1 {
			t.Fatalf("BL ＝ %02X", bl)
		}
		got := make([]byte, 8)
		for i := range got {
			got[i] = m.ReadPhys8(addr + uint32(i))
		}
		if !bytes.Equal(got, want) {
			t.Errorf("得 %q，預期 %q", got, want)
		}
	})
}

// 實模式端可到達 HMA（0x100000–0x10FFEF），與 Ext 同一塊記憶體。
func TestXMSExtMoveRealModeReachesHMA(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	h := xmsAlloc(t, m, d, 4)
	addr := xmsLock(t, m, d, h)
	m.WritePhys8(addr, 0x42)
	m.WritePhys8(addr+1, 0x43)
	// FFFF:0010 ＝ 0x100000。A20 關著，複製期間視為開啟。
	if ax, bl := xmsMoveCall(m, d, 2, h, 0, 0, realPtr(0xFFFF, 0x0010)); ax != 1 {
		t.Fatalf("BL ＝ %02X", bl)
	}
	if m.ReadPhys8(0x100000) != 0x42 || m.ReadPhys8(0x100001) != 0x43 {
		t.Error("沒有搬進 HMA")
	}
	if m.A20Enabled() {
		t.Error("A20 必須還原")
	}
}

func TestXMSExtLockUnlockInfo(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	h := xmsAlloc(t, m, d, 6)
	for _, fn := range []uint8{0x0C, 0x0D, 0x0E} {
		ax, bx, _ := xmsCallRegs(m, d, fn, 0, 33)
		mustFail(t, "無效 handle", ax, bx, 0xA2)
	}
	ax, bx, _ := xmsCallRegs(m, d, 0x0D, 0, h)
	mustFail(t, "Unlock 未鎖定", ax, bx, 0xAA)
	// 鎖定計數到 255 停止。
	for i := 0; i < 300; i++ {
		xmsLock(t, m, d, h)
	}
	ax, bx, dx := xmsCallRegs(m, d, 0x0E, 0, h)
	mustOK(t, "Info", ax, bx)
	if bh := uint8(bx >> 8); bh != 255 {
		t.Errorf("BH（鎖定計數）＝ %d，預期 255", bh)
	}
	if bl := uint8(bx); bl != xmsMaxHandles-1 {
		t.Errorf("BL（空 handle 數）＝ %d，預期 %d", bl, xmsMaxHandles-1)
	}
	if dx != 6 {
		t.Errorf("DX（KB）＝ %d，預期請求的 6", dx)
	}
	for i := 0; i < 255; i++ {
		ax, bx, _ := xmsCallRegs(m, d, 0x0D, 0, h)
		mustOK(t, "Unlock", ax, bx)
	}
	ax, bx, _ = xmsCallRegs(m, d, 0x0D, 0, h)
	mustFail(t, "Unlock 第 256 次", ax, bx, 0xAA)
}

// ---- 0Fh ----

func xmsRealloc(m *machine.Machine, d *DOS, h, kb uint16) (uint16, uint8) {
	ax, bx, _ := xmsCallRegs(m, d, 0x0F, kb, h)
	return ax, uint8(bx)
}

func TestXMSExtReallocErrors(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	if ax, bl := xmsRealloc(m, d, 9, 8); ax != 0 || bl != 0xA2 {
		t.Errorf("無效 handle：%d/%02X", ax, bl)
	}
	h := xmsAlloc(t, m, d, 8)
	xmsLock(t, m, d, h)
	if ax, bl := xmsRealloc(m, d, h, 4); ax != 0 || bl != 0xAB {
		t.Errorf("已鎖定：%d/%02X", ax, bl)
	}
}

func TestXMSExtReallocSamePagesOnlyUpdatesKB(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	h := xmsAlloc(t, m, d, 5) // 2 頁
	addr := xmsLock(t, m, d, h)
	xmsCallRegs(m, d, 0x0D, 0, h)
	if ax, bl := xmsRealloc(m, d, h, 7); ax != 1 || bl != 0 { // 仍 2 頁
		t.Fatalf("%d/%02X", ax, bl)
	}
	if got := xmsLock(t, m, d, h); got != addr {
		t.Errorf("基底改變 %#x→%#x", addr, got)
	}
	_, bx, dx := xmsCallRegs(m, d, 0x0E, 0, h)
	_ = bx
	if dx != 7 {
		t.Errorf("KB ＝ %d，預期 7", dx)
	}
}

func TestXMSExtReallocGrowInPlace(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	a := xmsAlloc(t, m, d, 16)
	addr := xmsLock(t, m, d, a)
	xmsCallRegs(m, d, 0x0D, 0, a)
	m.WritePhys8(addr, 0x99)
	if ax, bl := xmsRealloc(m, d, a, 64); ax != 1 {
		t.Fatalf("BL ＝ %02X", bl)
	}
	if got := xmsLock(t, m, d, a); got != addr {
		t.Errorf("後方有空頁應原地擴大：%#x→%#x", addr, got)
	}
	_, total, _ := xmsQuery(m, d)
	if total != 4096-64-64 {
		t.Errorf("總空間 ＝ %d", total)
	}
}

func TestXMSExtReallocGrowMovesAndCopies(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	a := xmsAlloc(t, m, d, 8) // 2 頁
	b := xmsAlloc(t, m, d, 8) // 緊接其後，擋住原地擴大
	aAddr := xmsLock(t, m, d, a)
	bAddr := xmsLock(t, m, d, b)
	xmsCallRegs(m, d, 0x0D, 0, a)
	for i := uint32(0); i < 8*1024; i++ {
		m.WritePhys8(aAddr+i, uint8(i*7))
		m.WritePhys8(bAddr+i, 0xBB)
	}
	if ax, bl := xmsRealloc(m, d, a, 16); ax != 1 {
		t.Fatalf("BL ＝ %02X", bl)
	}
	newAddr := xmsLock(t, m, d, a)
	if newAddr == aAddr {
		t.Fatal("後方被擋住，應搬到別處")
	}
	if newAddr < bAddr+8*1024 {
		t.Errorf("新位置 %#x 應在 B 之後（唯一夠大的洞）", newAddr)
	}
	for i := uint32(0); i < 8*1024; i++ {
		if got := m.ReadPhys8(newAddr + i); got != uint8(i*7) {
			t.Fatalf("搬移後 byte %d ＝ %02X，內容沒有複製", i, got)
		}
		if m.ReadPhys8(bAddr+i) != 0xBB {
			t.Fatal("B 的內容被改動")
		}
	}
	// 舊頁已釋放：下一個 8 KB 區塊落在原位。
	n := xmsAlloc(t, m, d, 8)
	if got := xmsLock(t, m, d, n); got != aAddr {
		t.Errorf("舊頁應已釋放：新配置在 %#x，預期 %#x", got, aAddr)
	}
}

func TestXMSExtReallocNoRoomKeepsBlock(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	a := xmsAlloc(t, m, d, 2000)
	b := xmsAlloc(t, m, d, 2000) // 剩 32 KB
	_ = b
	addr := xmsLock(t, m, d, a)
	xmsCallRegs(m, d, 0x0D, 0, a)
	if ax, bl := xmsRealloc(m, d, a, 2100); ax != 0 || bl != 0xA0 {
		t.Errorf("沒有洞：%d/%02X，預期 A0", ax, bl)
	}
	if got := xmsLock(t, m, d, a); got != addr {
		t.Error("失敗後區塊不得移動")
	}
	_, bx, dx := xmsCallRegs(m, d, 0x0E, 0, a)
	_ = bx
	if dx != 2000 {
		t.Errorf("失敗後 KB ＝ %d，預期 2000", dx)
	}
}

func TestXMSExtReallocShrinkAndZero(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	a := xmsAlloc(t, m, d, 40)
	addr := xmsLock(t, m, d, a)
	xmsCallRegs(m, d, 0x0D, 0, a)
	if ax, _ := xmsRealloc(m, d, a, 8); ax != 1 {
		t.Fatal("縮小失敗")
	}
	_, total, _ := xmsQuery(m, d)
	if total != 4096-64-8 {
		t.Errorf("縮小後總空間 ＝ %d，預期 %d", total, 4096-64-8)
	}
	// 新大小 0：釋放全部頁，handle 保留、大小 0、基底 0（Lock 回 0）。
	if ax, _ := xmsRealloc(m, d, a, 0); ax != 1 {
		t.Fatal("縮到 0 失敗")
	}
	_, total, _ = xmsQuery(m, d)
	if total != 4096-64 {
		t.Errorf("縮到 0 後總空間 ＝ %d", total)
	}
	if got := xmsLock(t, m, d, a); got != 0 {
		t.Errorf("縮到 0 的區塊 Lock ＝ %#x，預期 0", got)
	}
	_ = addr
	_, _, dx := xmsCallRegs(m, d, 0x0E, 0, a)
	if dx != 0 {
		t.Errorf("縮到 0 的區塊 KB ＝ %d", dx)
	}
	// handle 仍有效：再擴大視為新配置。
	xmsCallRegs(m, d, 0x0D, 0, a)
	if ax, _ := xmsRealloc(m, d, a, 12); ax != 1 {
		t.Fatal("0 頁區塊擴大失敗")
	}
	if got := xmsLock(t, m, d, a); got != 0x110000 {
		t.Errorf("擴大後基底 ＝ %#x，預期 BestMatch ＝ 0x110000", got)
	}
}

// 0 KB 區塊擴大到 1 頁後不得與其他區塊共用同一頁（刻意偏離 DOSBox-X，規格 §2.2 0Fh）。
func TestXMSExtZeroKBGrowDoesNotOverlap(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	y := xmsAlloc(t, m, d, 0) // 基底記為 0x110000，但不占頁
	z := xmsAlloc(t, m, d, 4) // 拿走 0x110000
	zAddr := xmsLock(t, m, d, z)
	if zAddr != 0x110000 {
		t.Fatalf("Z 在 %#x，前提不成立", zAddr)
	}
	if ax, bl := xmsRealloc(m, d, y, 4); ax != 1 {
		t.Fatalf("BL ＝ %02X", bl)
	}
	yAddr := xmsLock(t, m, d, y)
	if yAddr == zAddr {
		t.Fatalf("0 KB 區塊擴大後與 Z 共用同一頁 %#x", yAddr)
	}
	if yAddr != 0x111000 {
		t.Errorf("Y 應落在 BestMatch(1) ＝ 0x111000，實得 %#x", yAddr)
	}
	// 沒有洞時 A0h。
	m2, d2 := newExtDOS(t, 4096)
	full := xmsAlloc(t, m2, d2, 4032)
	_ = full
	y2 := xmsAlloc(t, m2, d2, 0)
	if ax, bl := xmsRealloc(m2, d2, y2, 4); ax != 0 || bl != 0xA0 {
		t.Errorf("沒有空頁時 0 KB 擴大：%d/%02X，預期 A0", ax, bl)
	}
}

// ---- BestMatch ----

// 配 A(1) B(3) C(1) D(2) E(1) 頁；放掉 B、D 後留下 3 頁（低）與 2 頁（高）兩個洞。
func holeSetup(t *testing.T, sizesKB []uint16, freeIdx []int) (*machine.Machine, *DOS, []uint32) {
	t.Helper()
	m, d := newExtDOS(t, 4096)
	var hs []uint16
	var addrs []uint32
	for _, kb := range sizesKB {
		h := xmsAlloc(t, m, d, kb)
		hs = append(hs, h)
		addrs = append(addrs, xmsLock(t, m, d, h))
		xmsCallRegs(m, d, 0x0D, 0, h)
	}
	for _, i := range freeIdx {
		xmsFreeH(t, m, d, hs[i])
	}
	return m, d, addrs
}

func TestXMSExtBestMatchExactBeforeFirstFit(t *testing.T) {
	m, d, addrs := holeSetup(t, []uint16{4, 12, 4, 8, 4}, []int{1, 3})
	h := xmsAlloc(t, m, d, 8) // 2 頁：低位址的 3 頁洞能容納，但剛好相等的洞優先
	if got := xmsLock(t, m, d, h); got != addrs[3] {
		t.Errorf("得 %#x，預期剛好相等的洞 %#x（首次適配會給 %#x）", got, addrs[3], addrs[1])
	}
}

func TestXMSExtBestMatchSmallestFit(t *testing.T) {
	m, d, addrs := holeSetup(t, []uint16{4, 20, 4, 12, 4}, []int{1, 3}) // 5 頁洞在低、3 頁洞在高
	h := xmsAlloc(t, m, d, 8)                                           // 2 頁：兩個洞都大於需求，取最小的
	if got := xmsLock(t, m, d, h); got != addrs[3] {
		t.Errorf("得 %#x，預期最小可容納的洞 %#x（首次適配會給 %#x）", got, addrs[3], addrs[1])
	}
}

func TestXMSExtBestMatchTieTakesLowerAddress(t *testing.T) {
	m, d, addrs := holeSetup(t, []uint16{4, 12, 4, 12, 4}, []int{1, 3}) // 兩個 3 頁洞
	h := xmsAlloc(t, m, d, 8)
	if got := xmsLock(t, m, d, h); got != addrs[1] {
		t.Errorf("同大小應取位址較低者：得 %#x，預期 %#x", got, addrs[1])
	}
}

// ---- int 15h ----

func TestInt15AH88And87WithExt(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x15, 0x8800)
	if m.CPU.R[cpu.AX] != xmsTotalKB {
		t.Errorf("kb == 0 的 AH=88h ＝ %d，預期 %d（保留既有行為）", m.CPU.R[cpu.AX], xmsTotalKB)
	}

	m, d = newExtDOS(t, 8192)
	call(m, d, 0x15, 0x8800)
	if m.CPU.R[cpu.AX] != 0 || m.CPU.Flags&cpu.CF != 0 {
		t.Errorf("kb > 0 的 AH=88h：AX ＝ %d CF ＝ %v，預期 0 且無 CF", m.CPU.R[cpu.AX], m.CPU.Flags&cpu.CF != 0)
	}

	// AH=87h：GDT 在 ES:SI，來源基底（位移 0x10）與目的基底（位移 0x18）是 24 位元實體位址。
	const seg = 0x3000
	gdt := cpu.Addr(seg, 0)
	setBase := func(off uint32, base uint32) {
		m.Write16(gdt+off+2, uint16(base))
		m.Write8(gdt+off+4, uint8(base>>16))
	}
	// 傳統記憶體 → 0x200000（Ext 內）
	for i := uint32(0); i < 8; i++ {
		m.Write8(0x40000+i, uint8(0x10+i))
	}
	setBase(0x10, 0x40000)
	setBase(0x18, 0x200000)
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.SI], m.CPU.R[cpu.CX] = seg, 0, 4
	call(m, d, 0x15, 0x8700)
	if ah(m.CPU) != 0 || m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("AH=87h：AH ＝ %02X", ah(m.CPU))
	}
	for i := uint32(0); i < 8; i++ {
		if got := m.ReadPhys8(0x200000 + i); got != uint8(0x10+i) {
			t.Fatalf("實體 0x200000+%d ＝ %02X", i, got)
		}
	}
	// Ext → 傳統記憶體。
	setBase(0x10, 0x200000)
	setBase(0x18, 0x50000)
	m.CPU.R[cpu.CX] = 4
	call(m, d, 0x15, 0x8700)
	if m.Read8(0x50003) != 0x13 {
		t.Error("Ext → 傳統記憶體沒有搬到")
	}
	// 越界來源讀 FF、越界目的丟棄並計數。
	setBase(0x10, 0xA00000)
	setBase(0x18, 0x60000)
	m.CPU.R[cpu.CX] = 1
	call(m, d, 0x15, 0x8700)
	if m.Read8(0x60000) != 0xFF {
		t.Errorf("越界來源應讀到 FF：%02X", m.Read8(0x60000))
	}
	setBase(0x10, 0x60000)
	setBase(0x18, 0xA00000)
	before := m.UnmappedPhysWrites
	call(m, d, 0x15, 0x8700)
	if m.UnmappedPhysWrites != before+2 {
		t.Errorf("越界目的應丟棄並計數：%d→%d", before, m.UnmappedPhysWrites)
	}
}

// kb == 0 時 XMS 行為完全不變：不碰 xmsBlocks，奇數長度照搬，Lock 回合成位址。
func TestXMSKB0Unchanged(t *testing.T) {
	m, d := newTest(t)
	ax, _, h := xmsCallRegs(m, d, 0x09, 0, 16)
	if ax != 1 {
		t.Fatal("配置失敗")
	}
	_, bx, dx := xmsCallRegs(m, d, 0x0C, 0, h)
	if addr := uint32(dx)<<16 | uint32(bx); addr != xmsLockBase+uint32(h)<<20 {
		t.Errorf("kb == 0 的 Lock ＝ %#x，應為合成位址", addr)
	}
	if d.xmsBlocks != nil {
		t.Error("kb == 0 不得建立 xmsBlocks")
	}
	// 奇數長度照搬。
	const seg = 0x3000
	m.Write16(cpu.Addr(seg, 0), 3)
	m.Write16(cpu.Addr(seg, 2), 0)
	m.Write16(cpu.Addr(seg, 4), 0)
	m.Write16(cpu.Addr(seg, 6), 0x100)
	m.Write16(cpu.Addr(seg, 8), seg)
	m.Write16(cpu.Addr(seg, 0x0A), h)
	m.Write16(cpu.Addr(seg, 0x0C), 0)
	m.Write16(cpu.Addr(seg, 0x0E), 0)
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = seg, 0
	m.CPU.R[cpu.AX] = 0x0B00
	d.xmsCall(m.CPU)
	if m.CPU.R[cpu.AX] != 1 {
		t.Error("kb == 0 的奇數長度 Move 應照搬成功")
	}
}
