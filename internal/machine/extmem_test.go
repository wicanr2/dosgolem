package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/430-physical-memory-and-xms-unification 的機器側測試（驗收 1(a)、2、3、6、7、8）。
// MemPoison 是全域變數，相關測試以 t.Cleanup 還原，不得使用 t.Parallel。

func newExt(t *testing.T, kb int) *Machine {
	t.Helper()
	m := New()
	if err := m.SetExtendedMemoryKB(kb); err != nil {
		t.Fatalf("SetExtendedMemoryKB(%d)：%v", kb, err)
	}
	return m
}

// 驗收 1(a)：預設不變。
func TestExtDefaultLeavesStubsAlone(t *testing.T) {
	for _, poison := range []uint8{0, 0xCC} {
		setPoison(t, poison)
		m := New()
		base := uint32(StubSeg) * 16
		if got := m.Mem[base+XMSTrapOff : base+XMSTrapOff+3]; !bytes.Equal(got, []byte{0xCD, 0xF5, 0xCB}) {
			t.Errorf("poison %02X：0x40C ＝ % X，應為 CD F5 CB", poison, got)
		}
		for i := uint32(0); i < 8; i++ {
			// 國別資訊表之後、上界之前的空段：New() 之後與 MemPoison 相同＝未寫入。
			if got := m.Mem[base+XMSHookEntryOff+i]; got != poison {
				t.Errorf("poison %02X：0x4B%d ＝ %02X，預設不得寫入", poison, i, got)
			}
		}
		if m.XMSEntryOff() != XMSTrapOff {
			t.Errorf("預設 XMSEntryOff ＝ %X，應為 XMSTrapOff", m.XMSEntryOff())
		}
		if m.ExtendedMemoryKB() != 0 || m.Ext != nil || !m.EMSPresent() {
			t.Error("預設應無 Ext 且 EMS 呈現")
		}
	}
}

// 驗收 2。
func TestSetExtendedMemoryKBValues(t *testing.T) {
	for _, kb := range []int{0, 4096, 64512} {
		m := New()
		if err := m.SetExtendedMemoryKB(kb); err != nil {
			t.Errorf("%d 應成功：%v", kb, err)
		}
		if m.ExtendedMemoryKB() != kb || len(m.Ext) != kb*1024 {
			t.Errorf("%d：ExtendedMemoryKB ＝ %d、len(Ext) ＝ %d", kb, m.ExtendedMemoryKB(), len(m.Ext))
		}
	}
	for _, kb := range []int{4095, 4097, 64516, 65536, -4} {
		m := New()
		if err := m.SetExtendedMemoryKB(kb); err == nil {
			t.Errorf("%d 應失敗", kb)
		}
		if m.Ext != nil {
			t.Errorf("%d 失敗後不得建立 Ext", kb)
		}
	}
}

func TestSetExtendedMemoryKBAfterLoadFails(t *testing.T) {
	for _, kb := range []int{0, 4096} {
		m := New()
		if err := m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}); err != nil {
			t.Fatal(err)
		}
		if err := m.SetExtendedMemoryKB(kb); err == nil {
			t.Errorf("載入後 SetExtendedMemoryKB(%d) 應失敗", kb)
		}
	}
	// 檢查順序 (1)：載入後即使引數不合法也是同一類錯誤，且不建立 Ext。
	m := New()
	_ = m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21})
	if err := m.SetExtendedMemoryKB(4095); err == nil || m.Ext != nil {
		t.Error("載入後呼叫必須失敗且不建立 Ext")
	}
}

func TestSetExtendedMemoryKBMemLengthChecked(t *testing.T) {
	m := New()
	m.Mem = append(m.Mem, 0)
	if err := m.SetExtendedMemoryKB(4096); err == nil {
		t.Error("len(Mem) != MemSize 應失敗")
	}
	// 順序 (2) 先於 (3)：Mem 長度不對時，不合法的 kb 也只會得到錯誤，不建立 Ext。
	if err := m.SetExtendedMemoryKB(4095); err == nil || m.Ext != nil {
		t.Error("應失敗且不建立 Ext")
	}
	// kb == 0 仍通過範圍檢查但 (2) 先擋。
	if err := m.SetExtendedMemoryKB(0); err == nil {
		t.Error("len(Mem) != MemSize 時連 0 都失敗（檢查 2 在檢查 4 之前）")
	}
}

func TestSetExtendedMemoryKBOnlyOnce(t *testing.T) {
	m := newExt(t, 4096)
	for _, kb := range []int{0, 4096, 8192} {
		if err := m.SetExtendedMemoryKB(kb); err == nil {
			t.Errorf("設成 4096 之後再呼叫 %d 應失敗", kb)
		}
	}
	if m.ExtendedMemoryKB() != 4096 {
		t.Errorf("失敗的呼叫改動了 Ext：%d KB", m.ExtendedMemoryKB())
	}
}

func TestSetExtendedMemoryZeroIsHarmless(t *testing.T) {
	m := New()
	before := append([]byte(nil), m.Mem...)
	for i := 0; i < 2; i++ {
		if err := m.SetExtendedMemoryKB(0); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(before, m.Mem) || m.Ext != nil {
		t.Error("kb 為 0 時再呼叫 0 不得有副作用")
	}
}

// kb > 0 的三件事：建立 Ext（全 0，不套 MemPoison）、複製 hma、寫入 8 bytes 入口。
func TestSetExtendedMemoryDoesThreeThings(t *testing.T) {
	setPoison(t, 0xCC)
	m := New()
	m.hma[0], m.hma[HMASize-1] = 0x11, 0x22
	if err := m.SetExtendedMemoryKB(4096); err != nil {
		t.Fatal(err)
	}
	if m.Ext[0] != 0x11 || m.Ext[HMASize-1] != 0x22 {
		t.Error("hma 內容沒有複製到 Ext[0:HMASize]")
	}
	for _, i := range []int{HMASize, HMASize + 1, 0x100000, len(m.Ext) - 1} {
		if m.Ext[i] != 0 {
			t.Errorf("Ext[%#x] ＝ %02X，Ext 不套 MemPoison", i, m.Ext[i])
		}
	}
	base := uint32(StubSeg)*16 + XMSHookEntryOff
	if got := m.Mem[base : base+8]; !bytes.Equal(got, []byte{0xEB, 0x03, 0x90, 0x90, 0x90, 0xCD, 0xF5, 0xCB}) {
		t.Errorf("入口 ＝ % X", got)
	}
	if m.XMSEntryOff() != XMSHookEntryOff {
		t.Errorf("XMSEntryOff ＝ %X", m.XMSEntryOff())
	}
	// 舊位置不動。
	b := uint32(StubSeg)*16 + XMSTrapOff
	if !bytes.Equal(m.Mem[b:b+3], []byte{0xCD, 0xF5, 0xCB}) {
		t.Error("0x40C 的 CD F5 CB 必須保留")
	}
}

// 驗收 3。
func TestReadWritePhysBoundaries(t *testing.T) {
	m := newExt(t, 8192) // 0x100000 .. 0x8FFFFF
	m.SetA20(false)      // 實體視圖不套 A20 遮罩
	type pt struct {
		a                    uint32
		inExt, unmapped, rom bool
	}
	pts := []pt{
		{0x0FFFFF, false, false, true},
		{0x100000, true, false, false},
		{0x10FFEF, true, false, false},
		{0x10FFF0, true, false, false},
		{0x8FFFFF, true, false, false},
		{0x900000, false, true, false},
	}
	for i, p := range pts {
		v := uint8(0x40 + i)
		before := m.UnmappedPhysWrites
		romBefore := m.ROMWrites
		orig := m.ReadPhys8(p.a)
		m.WritePhys8(p.a, v)
		got := m.ReadPhys8(p.a)
		switch {
		case p.unmapped:
			if got != 0xFF {
				t.Errorf("%#x 越界讀 ＝ %02X，應為 FF", p.a, got)
			}
			if m.UnmappedPhysWrites != before+1 {
				t.Errorf("%#x 越界寫計數 ＝ %d，應加一", p.a, m.UnmappedPhysWrites)
			}
		case p.rom:
			// 0xFFFFF 在 BIOS ROM 尾段：走 Write8，寫入丟棄並計 ROMWrites。
			if got != orig || m.ROMWrites != romBefore+1 {
				t.Errorf("%#x（ROM）讀 %02X 寫前 %02X、ROMWrites %d→%d", p.a, got, orig, romBefore, m.ROMWrites)
			}
			if m.UnmappedPhysWrites != before {
				t.Errorf("%#x 不應計入越界寫", p.a)
			}
		default:
			if got != v {
				t.Errorf("%#x 讀回 %02X，寫入 %02X", p.a, got, v)
			}
			if m.UnmappedPhysWrites != before {
				t.Errorf("%#x 不應計入越界寫", p.a)
			}
		}
		if p.inExt && m.Ext[p.a-0x100000] != v {
			t.Errorf("%#x 沒落在 Ext", p.a)
		}
	}
}

func TestReadWritePhysMultiByteCrossesRegions(t *testing.T) {
	m := newExt(t, 4096)
	// 0x0FFFFE、0x0FFFFF 在 ROM：寫入丟棄並計 ROMWrites；0x100000、0x100001 落在 Ext。
	romBefore := m.ROMWrites
	m.WritePhys32(0x0FFFFE, 0xDDCCBBAA)
	if m.ROMWrites != romBefore+2 || m.Ext[0] != 0xCC || m.Ext[1] != 0xDD {
		t.Errorf("跨界寫入：ROMWrites %d→%d、Ext[0:2] ＝ % X", romBefore, m.ROMWrites, m.Ext[:2])
	}
	want := uint32(m.Mem[0xFFFFE]) | uint32(m.Mem[0xFFFFF])<<8 | 0xDDCC0000
	if got := m.ReadPhys32(0x0FFFFE); got != want {
		t.Errorf("ReadPhys32 ＝ %08X，預期 %08X（ROM 兩 byte ＋ Ext 兩 byte 各自歸類）", got, want)
	}
	if got, w := m.ReadPhys16(0x0FFFFF), uint16(m.Mem[0xFFFFF])|0xCC<<8; got != w {
		t.Errorf("ReadPhys16 ＝ %04X，預期 %04X", got, w)
	}
	// 尾端：最後一個 Ext byte 與越界 byte 各自歸類。
	end := uint32(0x100000 + len(m.Ext))
	before := m.UnmappedPhysWrites
	m.WritePhys16(end-1, 0x1234)
	if m.Ext[len(m.Ext)-1] != 0x34 || m.UnmappedPhysWrites != before+1 {
		t.Error("跨越 Ext 尾端時最後一個 byte 寫入、越界 byte 丟棄並計數")
	}
	if got := m.ReadPhys16(end - 1); got != 0xFF34 {
		t.Errorf("ReadPhys16 跨尾端 ＝ %04X，應為 FF34", got)
	}
}

func TestHMAAliasesExtAndStaysIsolated(t *testing.T) {
	m := newExt(t, 8192)
	m.SetA20(true)
	// FFFF:0010 ＝ 實體 0x100000。
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0xFFFF, 0x0010
	exec(t, m, []byte{0x26, 0xC6, 0x07, 0x5A}, 1) // mov byte es:[bx],5Ah
	if got := m.ReadPhys8(0x100000); got != 0x5A {
		t.Errorf("實模式 FFFF:0010 寫入後 ReadPhys8(0x100000) ＝ %02X", got)
	}
	m.WritePhys8(0x100001, 0x6B)
	if got := m.Read8(0x100001); got != 0x6B {
		t.Errorf("WritePhys8 後 Read8 ＝ %02X", got)
	}
	if m.Peek8(0x100000) != m.Read8(0x100000) || m.Peek8(0x100001) != m.Read8(0x100001) {
		t.Error("Peek8 與 Read8 不一致")
	}
	// 寫 HMA 不改 0x110000，反之亦然。
	m.Write8(0x10FFEF, 0x77)
	if m.ReadPhys8(0x110000) != 0 {
		t.Error("寫 HMA 改到了 0x110000")
	}
	m.WritePhys8(0x110000, 0x88)
	if m.Read8(0x10FFEF) != 0x77 {
		t.Error("寫 0x110000 改到了 HMA")
	}
	// 0x10FFF0 以上實模式照舊折回低記憶體（A20 開時 hmaAddr 的上界是 MemSize+HMASize）：
	// 0x10FFF0 &^ 0x100000 ＝ 0x0FFF0。
	m.Write8(0x10FFF0, 0x99)
	if m.Mem[0xFFF0] != 0x99 {
		t.Errorf("0x10FFF0 應折回低記憶體 0xFFF0：Mem[0xFFF0] ＝ %02X", m.Mem[0xFFF0])
	}
	if m.ReadPhys8(0x10FFF0) == 0x99 {
		t.Error("折回的寫入不應落在 Ext")
	}
}

// A20 關時 HMA 不可達（行為同 kb == 0）。
func TestExtHMANeedsA20(t *testing.T) {
	m := newExt(t, 4096)
	m.WritePhys8(0x100000, 0x5A)
	m.SetA20(false)
	if m.Read8(0x100000) == 0x5A {
		t.Error("A20 關時 Read8(0x100000) 折回低記憶體，不得讀到 Ext")
	}
}

// kb == 0 時 HMA 仍用獨立陣列，實體視圖在 1 MB 之上一律不對應。
func TestPhysWithoutExt(t *testing.T) {
	m := New()
	m.SetA20(true)
	m.Write8(0x100000, 0x5A)
	if m.Read8(0x100000) != 0x5A {
		t.Error("kb == 0 的 HMA 要照舊")
	}
	if m.ReadPhys8(0x100000) != 0xFF {
		t.Error("kb == 0 時 1 MB 之上的實體讀回 FF")
	}
	m.WritePhys8(0x100000, 0x11)
	if m.UnmappedPhysWrites != 1 {
		t.Error("kb == 0 時 1 MB 之上的實體寫丟棄並計數")
	}
}

// 驗收 6：8042。
func out8042(m *Machine, cmd, data uint8) {
	m.Out8(0x64, cmd)
	m.Out8(0x60, data)
}

func TestKbd8042A20(t *testing.T) {
	m := newExt(t, 4096)
	out8042(m, 0xD1, 0xDF)
	if !m.A20Enabled() {
		t.Error("D1h+DFh 應開 A20")
	}
	out8042(m, 0xD1, 0xDD)
	if m.A20Enabled() {
		t.Error("D1h+DDh 應關 A20")
	}
}

func TestKbd8042OtherCommandCancelsPending(t *testing.T) {
	m := newExt(t, 4096)
	m.Out8(0x64, 0xD1)
	m.Out8(0x64, 0xAA) // 另一個命令，清除等待旗標
	m.Out8(0x60, 0xDF)
	if m.A20Enabled() {
		t.Error("D1h 後接另一個 64h 命令再 out 60h 不得影響 A20")
	}
	// 沒有 D1h 的 out 60h 也不影響。
	m.Out8(0x60, 0xDF)
	if m.A20Enabled() {
		t.Error("單獨的 out 60h 不得影響 A20")
	}
	// 旗標只吃一次。
	out8042(m, 0xD1, 0xDF)
	m.SetA20(false)
	m.Out8(0x60, 0xDF)
	if m.A20Enabled() {
		t.Error("旗標在第一個 out 60h 後必須清除")
	}
}

func TestKbd8042ResetBitIsOnlyLogged(t *testing.T) {
	m := newExt(t, 4096)
	m.CPU.IP = 0x1234
	steps := m.Steps
	out8042(m, 0xD1, 0xDC) // 位元 1 ＝ 0（A20 關）、位元 0 ＝ 0（要求重設）
	if m.A20Enabled() {
		t.Error("DCh 的位元 1 為 0，A20 應關")
	}
	if m.CPU.IP != 0x1234 || m.Steps != steps {
		t.Error("位元 0 為 0 只記錄，不得重設機器")
	}
	last := m.PortLog[len(m.PortLog)-1]
	if last.Port != 0x60 || last.Val != 0xDC {
		t.Errorf("PortLog 末筆 ＝ %+v，應記錄 out 60h,DCh", last)
	}
}

// kb == 0 時 8042 行為完全不變（只記錄，不開 A20）。
func TestKbd8042IgnoredWithoutExt(t *testing.T) {
	m := New()
	out8042(m, 0xD1, 0xDF)
	if m.A20Enabled() || m.kbdAwaitOutport {
		t.Error("kb == 0 時 D1h+DFh 不得開 A20")
	}
}

// kb > 0 時一般鍵盤路徑照常（埠 0x60 讀、0x61 ack、0x20 EOI 都不受 8042 邏輯影響）。
func TestKeyboardStillWorksWithExt(t *testing.T) {
	m := armKeyboard(t)
	if err := m.SetExtendedMemoryKB(4096); err != nil {
		t.Fatal(err)
	}
	if err := m.TypeScan("f"); err != nil {
		t.Fatal(err)
	}
	m.keyTick()
	if m.KeyIRQs != 1 {
		t.Fatalf("送出 %d 次鍵盤中斷，預期 1", m.KeyIRQs)
	}
	if got := m.In8(0x60); got != 0x21 {
		t.Errorf("埠 0x60 ＝ %02X，預期 21", got)
	}
	m.Out8(0x61, 0x80)
	m.Out8(0x61, 0x00)
	m.Out8(0x20, 0x20)
	if m.A20Enabled() {
		t.Error("一般鍵盤 ack 不得動到 A20")
	}
}

// 驗收 7（機器側）：SetEMSPresent 的檢查順序與撤銷。
func TestSetEMSPresentOrder(t *testing.T) {
	m := New()
	if err := m.SetEMSPresent(true); err != nil || !m.EMSPresent() {
		t.Errorf("目前 true 而 present==true：不做事（%v）", err)
	}
	if err := m.SetEMSPresent(false); err != nil || m.EMSPresent() {
		t.Errorf("目前 true 而 present==false：撤銷（%v）", err)
	}
	if err := m.SetEMSPresent(false); err != nil || m.EMSPresent() {
		t.Errorf("目前 false 而 present==false：不做事（%v）", err)
	}
	if err := m.SetEMSPresent(true); err == nil {
		t.Error("目前 false 而 present==true：應回錯誤")
	}
	if m.EMSPresent() {
		t.Error("失敗的呼叫不得復原 EMS")
	}
}

func TestSetEMSPresentAfterLoadFails(t *testing.T) {
	for _, present := range []bool{true, false} {
		m := New()
		_ = m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21})
		if err := m.SetEMSPresent(present); err == nil {
			t.Errorf("載入後 SetEMSPresent(%v) 一律回錯誤", present)
		}
	}
	// 目前 false 時載入後呼叫 false 也是錯誤（先看 programLoaded）。
	m := New()
	_ = m.SetEMSPresent(false)
	_ = m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21})
	if err := m.SetEMSPresent(false); err == nil {
		t.Error("載入後即使 false→false 也回錯誤")
	}
}

func TestSetEMSPresentFalseRevokesThreeThings(t *testing.T) {
	for _, poison := range []uint8{0, 0xCC} {
		setPoison(t, poison)
		m := New()
		base := uint32(EMSSeg) * 16
		if !bytes.Equal(m.Mem[base+0x0A:base+0x12], []byte("EMMXXXX0")) {
			t.Fatal("預設應有簽章")
		}
		if err := m.SetEMSPresent(false); err != nil {
			t.Fatal(err)
		}
		for i := uint32(0); i < 3; i++ {
			if m.Mem[base+i] != poison {
				t.Errorf("poison %02X：EMSSeg:%d ＝ %02X，應還原為 MemPoison", poison, i, m.Mem[base+i])
			}
		}
		for i := uint32(0); i < 8; i++ {
			if m.Mem[base+0x0A+i] != poison {
				t.Errorf("poison %02X：簽章第 %d byte ＝ %02X，應還原為 MemPoison", poison, i, m.Mem[base+0x0A+i])
			}
		}
		seg, off := m.Read16(0x67*4+2), m.Read16(0x67*4)
		if seg != StubSeg || off != StubOff(0x67) {
			t.Errorf("int 67h 向量 ＝ %04X:%04X，應為 %04X:%04X", seg, off, StubSeg, StubOff(0x67))
		}
		if poison == 0xCC {
			// :000A 不是 EMMXXXX0（驗收 7）
			if bytes.Equal(m.Mem[uint32(seg)*16+0x0A:uint32(seg)*16+0x12], []byte("EMMXXXX0")) {
				t.Error("int 67h 向量段的 :000A 仍是 EMMXXXX0")
			}
		}
	}
	// 預設（未呼叫）完全不變：簽章與向量仍在。
	m := New()
	if m.Read16(0x67*4+2) != EMSSeg {
		t.Error("預設 int 67h 向量應指 EMSSeg")
	}
}

// 驗收 8（機器側）：狀態往返、快照。
func saveBytes(t *testing.T, m *Machine) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := m.SaveState(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestStateRoundTripExt(t *testing.T) {
	m := newExt(t, 4096)
	_ = m.SetEMSPresent(false)
	m.SetA20(true)
	m.Write8(0x100000, 0xAB)
	m.WritePhys8(0x200000, 0xCD)
	m.WritePhys8(0x4FFFFF, 0xEF)
	m.Out8(0x64, 0xD1) // 等待輸出埠資料的旗標也要存
	data := saveBytes(t, m)
	if !bytes.Equal(data, saveBytes(t, m)) {
		t.Error("同一狀態連存兩次位元組不同")
	}

	n := newExt(t, 4096)
	_ = n.SetEMSPresent(false)
	if err := n.LoadState(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(m.Ext, n.Ext) {
		t.Error("Ext 沒有還原")
	}
	if !n.kbdAwaitOutport || n.EMSPresent() || !n.A20Enabled() {
		t.Error("8042 旗標、EMS 旗標或 A20 沒有還原")
	}
	if n.Read8(0x100000) != 0xAB {
		t.Error("HMA 內容沒有經 Ext 還原")
	}
	if !bytes.Equal(saveBytes(t, n), data) {
		t.Error("還原後再存的位元組應相同")
	}
}

func TestLoadStateExtMismatchLeavesMachineUntouched(t *testing.T) {
	src := newExt(t, 8192)
	src.WritePhys8(0x200000, 0x77)
	withExt := saveBytes(t, src)
	plain := saveBytes(t, New())
	noEMS := New()
	_ = noEMS.SetEMSPresent(false)
	absent := saveBytes(t, noEMS)

	cases := []struct {
		name string
		m    func() *Machine
		data []byte
	}{
		{"kb 不同", func() *Machine { return newExt(t, 4096) }, withExt},
		{"狀態檔有、機器漏給", func() *Machine { return New() }, withExt},
		{"機器多給", func() *Machine { return newExt(t, 4096) }, plain},
		{"EMSAbsent 不同（檔案 false、機器 true）", func() *Machine { m := New(); _ = m.SetEMSPresent(false); return m }, plain},
		{"EMSAbsent 不同（檔案 true、機器 false）", func() *Machine { return New() }, absent},
	}
	for _, tc := range cases {
		m := tc.m()
		m.Mem[0x500] = 0x5A
		before := append([]byte(nil), m.Mem...)
		extBefore := append([]byte(nil), m.Ext...)
		if err := m.LoadState(bytes.NewReader(tc.data)); err == nil {
			t.Errorf("%s：應回錯誤", tc.name)
			continue
		}
		if !bytes.Equal(before, m.Mem) || !bytes.Equal(extBefore, m.Ext) {
			t.Errorf("%s：失敗時機器必須維持原狀", tc.name)
		}
		if m.programLoaded {
			t.Errorf("%s：失敗時不得標記已載入", tc.name)
		}
	}
}

func TestSnapshotRestoreExt(t *testing.T) {
	m := newExt(t, 4096)
	_ = m.SetEMSPresent(false)
	m.WritePhys8(0x300000, 0x11)
	m.Out8(0x64, 0xD1)
	snap := m.Snapshot()
	m.WritePhys8(0x300000, 0x22)
	m.Out8(0x64, 0x00)
	m.Restore(snap)
	if m.ReadPhys8(0x300000) != 0x11 {
		t.Error("Restore 後 Ext 沒有還原")
	}
	if !m.kbdAwaitOutport {
		t.Error("Restore 後 8042 旗標沒有還原")
	}
	if m.EMSPresent() {
		t.Error("Restore 後 EMS 旗標沒有還原")
	}
	// 快照是複本：之後的寫入不改快照。
	m.WritePhys8(0x300000, 0x33)
	m.Restore(snap)
	if m.ReadPhys8(0x300000) != 0x11 {
		t.Error("快照的 Ext 必須是複本")
	}
}

func TestRestoreExtLengthMismatchPanics(t *testing.T) {
	snap := newExt(t, 4096).Snapshot()
	for _, other := range []*Machine{newExt(t, 8192), New()} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("Ext 長度不符應 panic")
				}
				msg, _ := r.(string)
				if !bytes.Contains([]byte(msg), []byte("4194304")) {
					t.Errorf("panic 訊息應寫明兩邊長度：%v", r)
				}
			}()
			other.Restore(snap)
		}()
	}
}

func TestStateDigestHashesExt(t *testing.T) {
	m := newExt(t, 4096)
	d1, err := StateDigest(bytes.NewReader(saveBytes(t, m)))
	if err != nil {
		t.Fatal(err)
	}
	m.WritePhys8(0x300000, 1)
	d2, _ := StateDigest(bytes.NewReader(saveBytes(t, m)))
	if d1 == d2 {
		t.Error("Ext 內容改變，摘要必須改變")
	}
	// kb == 0 的摘要與有無新欄位無關（錨點測試另釘住值）；這裡確認兩次相同。
	p := New()
	a, _ := StateDigest(bytes.NewReader(saveBytes(t, p)))
	b, _ := StateDigest(bytes.NewReader(saveBytes(t, p)))
	if a != b {
		t.Error("kb == 0 摘要不穩定")
	}
}
