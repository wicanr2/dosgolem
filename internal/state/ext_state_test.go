package state

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/430-physical-memory-and-xms-unification §5 第 8 項：延伸記憶體與 XMS 的狀態往返。
// 本檔以 `int F5h` 當作 XMS 服務呼叫（CPU 執行 INT，由 DOS 層接手；等同遠呼叫入口去掉 RETF）。

// extRig 造一台 kb > 0 的機器與 DOS，並載入最小程式。
func extRig(t *testing.T, kb int, noEMS bool) (*machine.Machine, *dos.DOS) {
	t.Helper()
	m := machine.New()
	if err := m.SetExtendedMemoryKB(kb); err != nil {
		t.Fatal(err)
	}
	if noEMS {
		if err := m.SetEMSPresent(false); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.LoadCOM(anchorCOM); err != nil {
		t.Fatal(err)
	}
	d := dos.New(m, anchorRoot)
	d.Install()
	d.Root = anchorRoot
	return m, d
}

// xmsInt 在 3000:0 放一道 `int F5h`，以給定暫存器執行一步，回 AX、BX、DX。
func xmsInt(t *testing.T, m *machine.Machine, ah uint8, bx, dx uint16) (ax, rbx, rdx uint16) {
	t.Helper()
	m.WriteBytes(cpu.Addr(0x3000, 0), []byte{0xCD, 0xF5})
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x3000, 0
	m.CPU.R[cpu.AX], m.CPU.R[cpu.BX], m.CPU.R[cpu.DX] = uint16(ah)<<8, bx, dx
	if err := m.Step(); err != nil {
		t.Fatal(err)
	}
	return m.CPU.R[cpu.AX], m.CPU.R[cpu.BX], m.CPU.R[cpu.DX]
}

// populate 配兩個區塊、鎖一個、寫內容、設 8042 旗標。
func populate(t *testing.T, m *machine.Machine) (h1, h2 uint16) {
	t.Helper()
	var ax uint16
	ax, _, h1 = xmsInt(t, m, 0x09, 0, 100)
	if ax != 1 {
		t.Fatal("配置失敗")
	}
	ax, _, h2 = xmsInt(t, m, 0x09, 0, 50)
	if ax != 1 {
		t.Fatal("配置失敗")
	}
	ax, bx, dx := xmsInt(t, m, 0x0C, 0, h2)
	if ax != 1 {
		t.Fatal("鎖定失敗")
	}
	base := uint32(dx)<<16 | uint32(bx)
	for i := uint32(0); i < 4096; i++ {
		m.WritePhys8(base+i, uint8(i*3+1))
	}
	_, bx, dx = xmsInt(t, m, 0x0C, 0, h1)
	_ = bx
	_ = dx
	xmsInt(t, m, 0x0D, 0, h1) // 解鎖：h1 鎖定計數回 0
	m.SetA20(true)
	m.WritePhys8(0x100000, 0xA5) // HMA 內容也在 Ext
	m.Out8(0x64, 0xD1)           // 8042 等待輸出埠資料旗標
	return h1, h2
}

func saveTo(t *testing.T, m *machine.Machine, d *dos.DOS) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.gz")
	if err := Save(p, m, d); err != nil {
		t.Fatal(err)
	}
	return p
}

func machineBytes(t *testing.T, m *machine.Machine) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := m.SaveState(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func dosBytes(t *testing.T, d *dos.DOS) []byte {
	t.Helper()
	d.Root = anchorRoot
	var b bytes.Buffer
	if err := d.SaveState(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestExtStateRoundTrip(t *testing.T) {
	m, d := extRig(t, 8192, true)
	h1, h2 := populate(t, m)
	path := saveTo(t, m, d)

	// 同一狀態連存兩次位元組相同（比照 TestDOSSaveStateIsByteIdentical）。
	if !bytes.Equal(machineBytes(t, m), machineBytes(t, m)) || !bytes.Equal(dosBytes(t, d), dosBytes(t, d)) {
		t.Fatal("同一狀態連存兩次位元組不同")
	}

	m2, d2 := extRig(t, 8192, true)
	if err := Load(path, m2, d2); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(m.Ext, m2.Ext) {
		t.Error("Ext 不同")
	}
	if !m2.A20Enabled() || m2.EMSPresent() {
		t.Error("A20 或 EMS 旗標不同")
	}
	// 配置表、鎖定計數與整台機器：位元組層級相同（含 8042 旗標與 EMSAbsent）。
	if !bytes.Equal(dosBytes(t, d), dosBytes(t, d2)) {
		t.Error("DOS 狀態（含 XMSBlocks）不同")
	}
	if !bytes.Equal(machineBytes(t, m), machineBytes(t, m2)) {
		t.Error("機器狀態不同")
	}
	// 8042 旗標：下一個 out 60h 應被當作輸出埠資料。
	m2.Out8(0x60, 0xDD)
	if m2.A20Enabled() {
		t.Error("8042 等待輸出埠資料旗標沒有還原（out 60h,DDh 應關 A20）")
	}
	m2.SetA20(true)
	// 行為層級：鎖定計數、大小，以及下一次配置落點都和原機器一致。
	_, bx, dx := xmsInt(t, m2, 0x0E, 0, h2)
	if bx>>8 != 1 || dx != 50 {
		t.Errorf("h2 的鎖定計數／KB ＝ %d/%d，預期 1/50", bx>>8, dx)
	}
	_, bx, _ = xmsInt(t, m2, 0x0E, 0, h1)
	if bx>>8 != 0 {
		t.Errorf("h1 鎖定計數 ＝ %d，預期 0", bx>>8)
	}
	_, bx1, dx1 := xmsInt(t, m, 0x09, 0, 30)
	_, bx2, dx2 := xmsInt(t, m2, 0x09, 0, 30)
	_, _ = bx1, bx2
	if dx1 != dx2 {
		t.Errorf("還原後下一個 handle ＝ %d，原機器 %d", dx2, dx1)
	}
	a1 := lock(t, m, dx1)
	a2 := lock(t, m2, dx2)
	if a1 != a2 {
		t.Errorf("還原後下一次配置落在 %#x，原機器 %#x", a2, a1)
	}
}

func lock(t *testing.T, m *machine.Machine, h uint16) uint32 {
	t.Helper()
	ax, bx, dx := xmsInt(t, m, 0x0C, 0, h)
	if ax != 1 {
		t.Fatal("鎖定失敗")
	}
	return uint32(dx)<<16 | uint32(bx)
}

func TestExtStateMismatchErrors(t *testing.T) {
	m8, d8 := extRig(t, 8192, false)
	populate(t, m8)
	file8192 := saveTo(t, m8, d8)

	m0 := machine.New()
	if err := m0.LoadCOM(anchorCOM); err != nil {
		t.Fatal(err)
	}
	d0 := dos.New(m0, anchorRoot)
	d0.Install()
	file0 := saveTo(t, m0, d0)

	mNo, dNo := extRig(t, 8192, true)
	fileNoEMS := saveTo(t, mNo, dNo)

	load := func(path string, kb int, noEMS bool) error {
		m := machine.New()
		if err := m.SetExtendedMemoryKB(kb); err != nil {
			t.Fatal(err)
		}
		if noEMS {
			_ = m.SetEMSPresent(false)
		}
		d := dos.New(m, anchorRoot)
		d.Install()
		return Load(path, m, d)
	}
	if err := load(file8192, 4096, false); err == nil {
		t.Error("kb 不同（檔案 8192、機器 4096）應回錯誤")
	}
	if err := load(file8192, 0, false); err == nil {
		t.Error("機器漏給 kb 應回錯誤")
	}
	if err := load(file0, 8192, false); err == nil {
		t.Error("機器多給 kb 應回錯誤")
	}
	if err := load(file8192, 8192, true); err == nil {
		t.Error("EMSAbsent 不同（檔案呈現、機器撤銷）應回錯誤")
	}
	if err := load(fileNoEMS, 8192, false); err == nil {
		t.Error("EMSAbsent 不同（檔案撤銷、機器呈現）應回錯誤")
	}
	if err := load(file8192, 8192, false); err != nil {
		t.Errorf("一致時應成功：%v", err)
	}
	if err := load(fileNoEMS, 8192, true); err != nil {
		t.Errorf("一致時應成功（no-ems）：%v", err)
	}
	if err := load(file0, 0, false); err != nil {
		t.Errorf("kb == 0 一致時應成功：%v", err)
	}
}

// DOS 側的互斥載入規則：EMB／EMBList 非空而機器 kb > 0、XMSBlocks 非空而機器 kb == 0 各回錯誤。
func TestExtDOSStateExclusiveLoad(t *testing.T) {
	// kb == 0 的 DOS 配一塊 EMB（EMBList 非空）。
	mPlain := machine.New()
	if err := mPlain.LoadCOM(anchorCOM); err != nil {
		t.Fatal(err)
	}
	dPlain := dos.New(mPlain, anchorRoot)
	dPlain.Install()
	if ax, _, _ := xmsInt(t, mPlain, 0x09, 0, 16); ax != 1 {
		t.Fatal("kb == 0 配置失敗")
	}
	plainBytes := dosBytes(t, dPlain)

	mExt, dExt := extRig(t, 8192, false)
	if err := dExt.LoadState(bytes.NewReader(plainBytes)); err == nil {
		t.Error("EMBList 非空的狀態檔載入到 kb > 0 應回錯誤")
	}

	// kb > 0 的 DOS 配一塊（XMSBlocks 非空，EMBList 不寫）。
	xmsInt(t, mExt, 0x09, 0, 16)
	extBytes := dosBytes(t, dExt)
	if err := dPlain.LoadState(bytes.NewReader(extBytes)); err == nil {
		t.Error("XMSBlocks 非空的狀態檔載入到 kb == 0 應回錯誤")
	}
	// 失敗要在任何寫入之前：dPlain 的 EMB 還在。
	if len(dPlain.EMBSizes()) != 1 {
		t.Errorf("失敗的載入改動了 EMB：%v", dPlain.EMBSizes())
	}
	// 相容的組合照常成功。
	mExt2, dExt2 := extRig(t, 8192, false)
	_ = mExt2
	if err := dExt2.LoadState(bytes.NewReader(extBytes)); err != nil {
		t.Errorf("kb > 0 載入 kb > 0 的 DOS 狀態應成功：%v", err)
	}
	// kb > 0 存檔只寫 XMSBlocks，不寫 EMBList：往返後 EMB 是空的。
	if len(dExt2.EMBSizes()) != 0 {
		t.Errorf("kb > 0 載入後 EMB 應為空：%v", dExt2.EMBSizes())
	}
}

func TestExtSnapshotRestoreInRig(t *testing.T) {
	m, _ := extRig(t, 8192, false)
	populate(t, m)
	snap := m.Snapshot()
	before := append([]byte(nil), m.Ext...)
	for i := 0; i < 64; i++ {
		m.WritePhys8(0x120000+uint32(i), 0xEE)
	}
	m.Restore(snap)
	if !bytes.Equal(before, m.Ext) {
		t.Error("Restore 後 Ext 沒有還原")
	}
}

// 預設（kb == 0）的存檔不寫入任何新欄位，且載入舊式存檔照常（錨點測試釘住位元組）。
func TestExtDefaultStateLoadsWithoutExt(t *testing.T) {
	m := machine.New()
	_ = m.LoadCOM(anchorCOM)
	d := dos.New(m, anchorRoot)
	d.Install()
	path := saveTo(t, m, d)
	m2 := machine.New()
	d2 := dos.New(m2, anchorRoot)
	d2.Install()
	if err := Load(path, m2, d2); err != nil {
		t.Fatal(err)
	}
	if m2.ExtendedMemoryKB() != 0 || !m2.EMSPresent() {
		t.Error("kb == 0 載入後不得出現 Ext 或撤銷 EMS")
	}
}
