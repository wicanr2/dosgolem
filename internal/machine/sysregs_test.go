package machine

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// docs/spec/433-real-mode-system-instructions §8 第 3 項的狀態類測試：SysRegs 的寫入規則與初值、
// PM 版本 2 的位移、§2.1 的驗證條件、Snapshot／Restore 與 SaveState／LoadState 往返、
// SetProtectedModeProfile 切換時回初值與 Sys 的裝卸。434 §2.2 起 encodePM 輸出版本 3，
// 版本 2 改以手造位元組驗讀取相容；版本 3 增加的部分在 segstate_test.go。

func TestSysRegsInitialValues(t *testing.T) {
	m := newPM(t)
	for _, c := range []struct {
		name      string
		got, want uint32
	}{
		{"CR2", must32(t, m, 2), 0},
		{"DR0", m.DR(0), 0}, {"DR1", m.DR(1), 0}, {"DR2", m.DR(2), 0}, {"DR3", m.DR(3), 0},
		{"DR6", m.DR(6), 0xFFFF1FF0},
		{"DR4（DR6 別名）", m.DR(4), 0xFFFF1FF0},
		{"DR7", m.DR(7), 0x00000400},
		{"DR5（DR7 別名）", m.DR(5), 0x00000400},
	} {
		if c.got != c.want {
			t.Errorf("%s 初值 = %08X，應為 %08X", c.name, c.got, c.want)
		}
	}
	if b, l := m.GDTR(); b != 0 || l != 0 {
		t.Errorf("GDTR 初值 = %08X/%04X，應為 0/0", b, l)
	}
	if b, l := m.IDTR(); b != 0 || l != 0x03FF {
		t.Errorf("IDTR 初值 = %08X/%04X，應為 0/03FF", b, l)
	}
	if m.sys.ldtr != (sysSeg{}) || m.sys.tr != (sysSeg{}) {
		t.Errorf("LDTR、TR 初值應全 0：%+v %+v", m.sys.ldtr, m.sys.tr)
	}
	// profile 關閉的新機器也是初值（不是零值）。
	if off := newExt(t, 4096); off.sys != sysRegsInit() {
		t.Errorf("profile 關閉的機器 SysRegs = %+v，應為初值", off.sys)
	}
}

func must32(t *testing.T, m *Machine, n int) uint32 {
	t.Helper()
	v, err := m.CR(n)
	if err != nil {
		t.Fatalf("CR(%d)：%v", n, err)
	}
	return v
}

func TestSysRegsWriteRules(t *testing.T) {
	m := newPM(t)
	// CR2 讀寫照存。
	mustNil(t, "SetCR(2)", m.SetCR(2, 0xCAFEBABE))
	if v := must32(t, m, 2); v != 0xCAFEBABE {
		t.Errorf("CR2 = %08X", v)
	}
	// CR0、CR3 經 432 的 setter。
	mustNil(t, "SetCR(3)", m.SetCR(3, 0x12345678))
	if v := must32(t, m, 3); v != 0x12345678 || m.CR3() != 0x12345678 {
		t.Errorf("CR3 = %08X（原始值，遮罩在指令處理函式做）", v)
	}
	mustNil(t, "SetCR(0)", m.SetCR(0, 0xFFFFFFFF&^cr0PE&^cr0PG))
	if v := must32(t, m, 0); v != 0x1E {
		t.Errorf("CR0 經 SetCR0 遮罩：%08X，應為 0000001E", v)
	}
	// CR1、CR4 起不存在。
	for _, n := range []int{1, 4, 5, 7} {
		if _, err := m.CR(n); err == nil {
			t.Errorf("CR(%d) 應回錯誤", n)
		}
		if err := m.SetCR(n, 0); err == nil {
			t.Errorf("SetCR(%d) 應回錯誤", n)
		}
	}
	// DR0–DR3 照存。
	for i := 0; i < 4; i++ {
		m.SetDR(i, 0x11111111*uint32(i+1))
		if v := m.DR(i); v != 0x11111111*uint32(i+1) {
			t.Errorf("DR%d = %08X", i, v)
		}
	}
	// DR6：(v|FFFF0FF0h)&FFFFEFFFh。
	for _, c := range []struct {
		n       int
		v, want uint32
	}{
		{6, 0, 0xFFFF0FF0},
		{6, 0xFFFFFFFF, 0xFFFFEFFF},
		{6, 0x0000F00F, 0xFFFFEFFF},
		{4, 0x00000001, 0xFFFF0FF1}, // DR4 是 DR6 的別名
	} {
		m.SetDR(c.n, c.v)
		if m.DR(6) != c.want || m.DR(4) != c.want {
			t.Errorf("SetDR(%d, %08X)：DR6=%08X DR4=%08X，應為 %08X", c.n, c.v, m.DR(6), m.DR(4), c.want)
		}
	}
	// DR7：(v|400h)&FFFF2FFFh（保留 bit 13 GD，清 bit 12、14、15）。
	for _, c := range []struct {
		n       int
		v, want uint32
	}{
		{7, 0, 0x00000400},
		{7, 0xFFFFFFFF, 0xFFFF2FFF},
		{7, 0x00002000, 0x00002400},
		{5, 0x00000003, 0x00000403}, // DR5 是 DR7 的別名
	} {
		m.SetDR(c.n, c.v)
		if m.DR(7) != c.want || m.DR(5) != c.want {
			t.Errorf("SetDR(%d, %08X)：DR7=%08X DR5=%08X，應為 %08X", c.n, c.v, m.DR(7), m.DR(5), c.want)
		}
	}
	// GDTR、IDTR 照存。
	m.SetGDTR(0x00123456, 0x012F)
	m.SetIDTR(0xFF000000, 0x07FF)
	if b, l := m.GDTR(); b != 0x00123456 || l != 0x012F {
		t.Errorf("GDTR = %08X/%04X", b, l)
	}
	if b, l := m.IDTR(); b != 0xFF000000 || l != 0x07FF {
		t.Errorf("IDTR = %08X/%04X", b, l)
	}
}

// fillSys 把每個 SysRegs 欄位設成可辨識的值（LDTR、TR 沒有指令寫入路徑，直接賦內部欄位）。
func fillSys(t *testing.T, m *Machine) {
	t.Helper()
	mustNil(t, "SetCR(2)", m.SetCR(2, 0xC2C2C2C2))
	m.SetDR(0, 0xD0D0D0D0)
	m.SetDR(1, 0xD1D1D1D1)
	m.SetDR(2, 0xD2D2D2D2)
	m.SetDR(3, 0xD3D3D3D3)
	m.SetDR(6, 0x0000400F)
	m.SetDR(7, 0x000023FF)
	m.SetGDTR(0x00ABCDEF, 0x012F)
	m.SetIDTR(0x00012345, 0x07FF)
	m.sys.ldtr = sysSeg{sel: 0x0028, cache: SegCache{Base: 0x11223344, Limit: 0x0000FFFF, Attr: 0x0082}}
	m.sys.tr = sysSeg{sel: 0x0030, cache: SegCache{Base: 0x55667788, Limit: 0x000FFFFF, Attr: 0xD08B}}
}

// TestPMV3LayoutPrefix：版本 3 的前 80 bytes 與版本 2 的位移相同（434 §2.2），未分段時尾端為 0。
func TestPMV3LayoutPrefix(t *testing.T) {
	m := newPM(t)
	mustNil(t, "SetCR0", m.SetCR0(0x80000000|cr0PE|0x8))
	mustNil(t, "SetCR3", m.SetCR3(0xDEADB000))
	mustNil(t, "SetPMCPL", m.SetPMCPL(2))
	mustNil(t, "SetEFlagsHi", m.SetEFlagsHi(eflagsHiRF))
	fillSys(t, m)
	b := m.encodePM()
	// 459 §5.1：版本 4（佇列空時 148 bytes），前 142 bytes 照版本 3；尾端 142–147 是 PIC 與佇列。
	if len(b) != 148 {
		t.Fatalf("PM 版本 4 共 148 bytes，實際 %d", len(b))
	}
	for i, v := range b[80:142] {
		if v != 0 {
			t.Errorf("未分段時位移 %d（segmented、保留、快取）應為 0：%02X", 80+i, v)
		}
	}
	if b[142] != 0 || b[143] != 0 || b[144] != 0xFF || b[145] != 0 || b[146] != 0 || b[147] != 0 {
		t.Errorf("程式未載入時 PM 版本 4 尾端應為 旗標 0、旗標 2 0、從片 IMR FFh、主片 IMR 0、佇列 0：% X", b[142:])
	}
	le := binary.LittleEndian
	for _, c := range []struct {
		name      string
		got, want uint64
	}{
		{"版本號（0，2）", uint64(le.Uint16(b[0:])), 4},
		{"PMProfile（2）", uint64(b[2]), 1},
		{"pmCPL（3）", uint64(b[3]), 2},
		{"CR0（4，4）", uint64(le.Uint32(b[4:])), 0x80000019},
		{"CR3（8，4）", uint64(le.Uint32(b[8:])), 0xDEADB000},
		{"EFlagsHi（12，2）", uint64(le.Uint16(b[12:])), eflagsHiRF},
		{"保留（14，2）", uint64(le.Uint16(b[14:])), 0},
		{"CR2（16，4）", uint64(le.Uint32(b[16:])), 0xC2C2C2C2},
		{"DR0（20，4）", uint64(le.Uint32(b[20:])), 0xD0D0D0D0},
		{"DR1（24，4）", uint64(le.Uint32(b[24:])), 0xD1D1D1D1},
		{"DR2（28，4）", uint64(le.Uint32(b[28:])), 0xD2D2D2D2},
		{"DR3（32，4）", uint64(le.Uint32(b[32:])), 0xD3D3D3D3},
		{"DR6（36，4）", uint64(le.Uint32(b[36:])), 0xFFFF4FFF},
		{"DR7（40，4）", uint64(le.Uint32(b[40:])), 0x000027FF},
		{"GDTR 基底（44，4）", uint64(le.Uint32(b[44:])), 0x00ABCDEF},
		{"GDTR 界限（48，2）", uint64(le.Uint16(b[48:])), 0x012F},
		{"IDTR 基底（50，4）", uint64(le.Uint32(b[50:])), 0x00012345},
		{"IDTR 界限（54，2）", uint64(le.Uint16(b[54:])), 0x07FF},
		{"LDTR 選擇子（56，2）", uint64(le.Uint16(b[56:])), 0x0028},
		{"LDTR 基底（58，4）", uint64(le.Uint32(b[58:])), 0x11223344},
		{"LDTR 界限（62，4）", uint64(le.Uint32(b[62:])), 0x0000FFFF},
		{"LDTR 屬性（66，2）", uint64(le.Uint16(b[66:])), 0x0082},
		{"TR 選擇子（68，2）", uint64(le.Uint16(b[68:])), 0x0030},
		{"TR 基底（70，4）", uint64(le.Uint32(b[70:])), 0x55667788},
		{"TR 界限（74，4）", uint64(le.Uint32(b[74:])), 0x000FFFFF},
		{"TR 屬性（78，2）", uint64(le.Uint16(b[78:])), 0xD08B},
	} {
		if c.got != c.want {
			t.Errorf("%s = %X，應為 %X", c.name, c.got, c.want)
		}
	}
	// 解碼回來逐欄相同。
	f, err := decodePM(b)
	mustNil(t, "decodePM", err)
	if f.sysRegs != m.sys || f.cr0 != m.cr0 || f.cr3 != m.cr3 || f.cpl != m.pmCPL || f.eflagsHi != m.eflagsHi {
		t.Errorf("decodePM(encodePM) 不同：%+v", f)
	}
}

// TestPMV2HandBuiltReadable：手造的版本 2（80 bytes）可讀，segmented=false、快取全 0（434 §2.2）。
func TestPMV2HandBuiltReadable(t *testing.T) {
	le := binary.LittleEndian
	b := make([]byte, 80)
	le.PutUint16(b[0:], 2)
	b[2], b[3] = 1, 2
	le.PutUint32(b[4:], 0x80000019)
	le.PutUint32(b[8:], 0xDEADB000)
	le.PutUint16(b[12:], eflagsHiRF)
	le.PutUint32(b[16:], 0xC2C2C2C2)
	for i := 0; i < 4; i++ {
		le.PutUint32(b[20+4*i:], 0xD0D0D0D0+uint32(i)*0x01010101)
	}
	le.PutUint32(b[36:], 0xFFFF4FFF)
	le.PutUint32(b[40:], 0x000027FF)
	le.PutUint32(b[44:], 0x00ABCDEF)
	le.PutUint16(b[48:], 0x012F)
	le.PutUint32(b[50:], 0x00012345)
	le.PutUint16(b[54:], 0x07FF)
	le.PutUint16(b[56:], 0x0028)
	le.PutUint32(b[58:], 0x11223344)
	le.PutUint32(b[62:], 0x0000FFFF)
	le.PutUint16(b[66:], 0x0082)
	le.PutUint16(b[68:], 0x0030)
	le.PutUint32(b[70:], 0x55667788)
	le.PutUint32(b[74:], 0x000FFFFF)
	le.PutUint16(b[78:], 0xD08B)
	f, err := decodePM(b)
	mustNil(t, "decodePM(手造版本 2)", err)
	want := sysRegs{cr2: 0xC2C2C2C2, dr: [4]uint32{0xD0D0D0D0, 0xD1D1D1D1, 0xD2D2D2D2, 0xD3D3D3D3},
		dr6: 0xFFFF4FFF, dr7: 0x000027FF, gdtrBase: 0x00ABCDEF, gdtrLimit: 0x012F,
		idtrBase: 0x00012345, idtrLimit: 0x07FF,
		ldtr: sysSeg{sel: 0x0028, cache: SegCache{Base: 0x11223344, Limit: 0x0000FFFF, Attr: 0x0082}},
		tr:   sysSeg{sel: 0x0030, cache: SegCache{Base: 0x55667788, Limit: 0x000FFFFF, Attr: 0xD08B}}}
	if f.sysRegs != want || f.cr0 != 0x80000019 || f.cr3 != 0xDEADB000 || f.cpl != 2 || f.eflagsHi != eflagsHiRF {
		t.Errorf("手造版本 2 解碼：%+v", f)
	}
	if f.segmented || f.cache != [6]SegCache{} {
		t.Errorf("版本 2 沒有分段模式狀態：segmented=%v cache=%+v", f.segmented, f.cache)
	}
	// 版本 2 長度不足。
	if _, err := decodePM(b[:79]); err == nil {
		t.Error("版本 2 只有 79 bytes：應回錯誤")
	}
}

func TestLoadStatePMV2Validation(t *testing.T) {
	src := newPM(t)
	mustNil(t, "LoadCOM", src.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	fillSys(t, src)
	good := saveBytes(t, src)
	base := decodeStateForTest(t, good)
	mut := func(f func(pm []byte)) []byte {
		s := base
		s.PM = append([]byte(nil), base.PM...)
		f(s.PM)
		return encodeStateForTest(t, s)
	}
	le := binary.LittleEndian
	set32 := func(off int, v uint32) func([]byte) { return func(p []byte) { le.PutUint32(p[off:], v) } }
	set16 := func(off int, v uint16) func([]byte) { return func(p []byte) { le.PutUint16(p[off:], v) } }
	bad := []struct {
		name string
		f    func([]byte)
	}{
		{"DR6 缺 bit 4", set32(36, 0xFFFF0FE0)},
		{"DR6 缺 bit 31", set32(36, 0x7FFF0FF0)},
		{"DR6 為 0", set32(36, 0)},
		{"DR7 的 bit 10 為 0", set32(40, 0)},
		{"DR7 的 bit 12 為 1", set32(40, 0x1400)},
		{"DR7 的 bit 14 為 1", set32(40, 0x4400)},
		{"DR7 的 bit 15 為 1", set32(40, 0x8400)},
		{"LDTR 屬性 bit 8", set16(66, 0x0100)},
		{"LDTR 屬性 bit 11", set16(66, 0x0800)},
		{"LDTR 屬性 bit 13", set16(66, 0x2000)},
		{"TR 屬性 bit 9", set16(78, 0x0200)},
		{"TR 屬性 bit 10", set16(78, 0x0400)},
		{"TR 屬性 bit 13", set16(78, 0x2000)},
	}
	for _, c := range bad {
		m := newPM(t)
		before := saveBytes(t, m)
		if err := m.LoadState(bytes.NewReader(mut(c.f))); err == nil {
			t.Errorf("%s：應回錯誤", c.name)
		}
		if !bytes.Equal(before, saveBytes(t, m)) {
			t.Errorf("%s：失敗的 LoadState 不得改變機器", c.name)
		}
	}
	// 不驗的位元：DR6 bit 12、DR7 bit 13、屬性 bit 12、14、15。
	okCases := []struct {
		name string
		f    func([]byte)
	}{
		{"DR6 bit 12 為 1（初值）", set32(36, 0xFFFF1FF0)},
		{"DR7 bit 13 為 1", set32(40, 0x2400)},
		{"LDTR 屬性 bit 12、14、15", set16(66, 0xD0FF)},
		{"TR 屬性 bit 12、14、15", set16(78, 0xD0FF)},
	}
	for _, c := range okCases {
		m := newPM(t)
		mustNil(t, c.name, m.LoadState(bytes.NewReader(mut(c.f))))
	}
}

func TestSysRegsRoundTrip(t *testing.T) {
	// SaveState／LoadState。
	src := newPM(t)
	mustNil(t, "LoadCOM", src.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	fillSys(t, src)
	data := saveBytes(t, src)
	dst := newPM(t)
	mustNil(t, "LoadState", dst.LoadState(bytes.NewReader(data)))
	if dst.sys != src.sys {
		t.Errorf("SaveState／LoadState 往返：\n got  %+v\n want %+v", dst.sys, src.sys)
	}
	if !bytes.Equal(saveBytes(t, dst), data) {
		t.Error("存檔→讀檔→存檔不一致")
	}

	// Snapshot／Restore：拍快照後改掉每個欄位，Restore 後全部回來。
	m := newPM(t)
	mustNil(t, "LoadCOM", m.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	fillSys(t, m)
	want := m.sys
	snap := m.Snapshot()
	m.sys = sysRegs{}
	m.SetDR(6, 0)
	m.Restore(snap)
	if m.sys != want {
		t.Errorf("Snapshot／Restore 往返：\n got  %+v\n want %+v", m.sys, want)
	}
	// 快照是複本：快照之後改機器不影響快照。
	m.SetGDTR(0, 0)
	m.Restore(snap)
	if b, l := m.GDTR(); b != 0x00ABCDEF || l != 0x012F {
		t.Errorf("快照被機器的修改影響：GDTR %08X/%04X", b, l)
	}
}

func TestSysRegsProfileOffUsesInitialValues(t *testing.T) {
	// profile 關閉時 LoadState、Restore 套用初值，不是零值（433 §2.1）。
	src := newExt(t, 4096)
	mustNil(t, "LoadCOM", src.LoadCOM([]byte{0xB4, 0x4C, 0xCD, 0x21}))
	data := saveBytes(t, src)
	snap := src.Snapshot()
	dst := newExt(t, 4096)
	dst.sys = sysRegs{} // 弄成零值，證明 LoadState 會改回初值
	mustNil(t, "LoadState", dst.LoadState(bytes.NewReader(data)))
	if dst.sys != sysRegsInit() {
		t.Errorf("profile 關閉的 LoadState 後 SysRegs = %+v，應為初值", dst.sys)
	}
	dst.sys = sysRegs{}
	dst.Restore(snap)
	if dst.sys != sysRegsInit() {
		t.Errorf("profile 關閉的 Restore 後 SysRegs = %+v，應為初值", dst.sys)
	}
}

func TestSetProtectedModeProfileResetsSysRegs(t *testing.T) {
	m := newExt(t, 4096)
	if m.CPU.Sys != nil {
		t.Fatal("profile 關閉時 CPU.Sys 應為 nil")
	}
	m.sys = sysRegs{cr2: 1, dr6: 2, dr7: 3, gdtrBase: 4, idtrBase: 5,
		ldtr: sysSeg{sel: 6, cache: SegCache{Base: 7, Limit: 8, Attr: 9}},
		tr:   sysSeg{sel: 10, cache: SegCache{Base: 11, Limit: 12, Attr: 13}}}
	mustNil(t, "SetProtectedModeProfile(true)", m.SetProtectedModeProfile(true))
	if m.sys != sysRegsInit() {
		t.Errorf("開啟後 SysRegs = %+v，應為初值", m.sys)
	}
	if m.CPU.Sys == nil || m.CPU.Sys != m {
		t.Error("開啟後 CPU.Sys 應裝上 Machine")
	}
	fillSys(t, m)
	// 同值呼叫回 nil，不動狀態。
	mustNil(t, "同值", m.SetProtectedModeProfile(true))
	if m.DR(0) != 0xD0D0D0D0 {
		t.Error("同值呼叫不應重設 SysRegs")
	}
	mustNil(t, "SetProtectedModeProfile(false)", m.SetProtectedModeProfile(false))
	if m.sys != sysRegsInit() {
		t.Errorf("關閉後 SysRegs = %+v，應為初值", m.sys)
	}
	if m.CPU.Sys != nil {
		t.Error("關閉後 CPU.Sys 應為 nil")
	}
	// CR2 的 setter 在 profile 關閉時回錯誤（同 432 的 SetCR0、SetCR3）。
	if err := m.SetCR(2, 1); err == nil {
		t.Error("profile 關閉時 SetCR(2) 應回錯誤")
	}
}

// 442 §2.7：IVTMismatch 依向量（4×vec＋3 > 界限也成立）。原預期不變，另補界限的案例。
func TestIVTMismatchCondition(t *testing.T) {
	off := newExt(t, 4096)
	off.SetIDTR(0x1000, 0x3FF)
	if off.IVTMismatch(0) || off.IVTMismatch(0xFF) {
		t.Error("profile 關閉：不成立")
	}
	m := newPM(t)
	if m.IVTMismatch(0) || m.IVTMismatch(0xFF) {
		t.Error("IDTR 基底 0、界限 3FFh：向量 0 與 FFh 都不成立")
	}
	m.SetIDTR(0, 0x1F)
	if m.IVTMismatch(7) {
		t.Error("基底 0、界限 1Fh：向量 7（4×7＋3＝1Fh）不成立")
	}
	if !m.IVTMismatch(8) {
		t.Error("基底 0、界限 1Fh：向量 8（4×8＋3＝23h）成立")
	}
	m.SetIDTR(0, 0x7FF)
	if m.IVTMismatch(0xFF) {
		t.Error("基底 0、界限 7FFh：向量 FFh 不成立（界限大於 3FFh 無害）")
	}
	m.SetIDTR(0x1000, 0x3FF)
	if !m.IVTMismatch(0) || !m.IVTMismatch(0xFF) {
		t.Error("profile 開、PE=0、IDTR 基底非 0：成立")
	}
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	if m.IVTMismatch(0) || m.IVTMismatch(0xFF) {
		t.Error("PE=1：不成立")
	}
}
