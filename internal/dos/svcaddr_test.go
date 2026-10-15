package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/432-linear-memory-and-paging §4、§6、§9 第 9、10 項的服務層測試。
// 這一輪（C1）沒有分頁，所以「映射狀態 1」的對照留到分頁實作。

// newPMDOSMachine 造一台 profile 開啟、PE=1、A20 開的機器（4 MB 延伸記憶體），服務層已掛好。
func newPMDOSMachine(t *testing.T) (*machine.Machine, *DOS) {
	t.Helper()
	m := machine.New()
	if err := m.SetExtendedMemoryKB(4096); err != nil {
		t.Fatal(err)
	}
	if err := m.SetProtectedModeProfile(true); err != nil {
		t.Fatal(err)
	}
	d := New(m, t.TempDir())
	d.Install()
	d.freeSeg = 0x2000
	m.SetA20(true)
	if err := m.SetCR0(1); err != nil {
		t.Fatal(err)
	}
	return m, d
}

// 轉換開啟且 A20 開時，客體的 FFFF:0010 起是線性 0x100000 起；服務層要讀到那裡，
// 不是折回 0 起。轉換關閉時維持現行的折回（行為不變）。
func TestServiceLayerSeesHMAWhenTranslating(t *testing.T) {
	m, d := newPMDOSMachine(t)
	copy(m.Ext, "HELLO\x00")
	copy(m.Mem, "LOW\x00")
	if got := d.readCString(0xFFFF, 0x10, 64); got != "HELLO" {
		t.Errorf("轉換開啟：readCString(FFFF:0010) = %q，應為 %q", got, "HELLO")
	}
	if a := m.SvcAddr(0xFFFF, 0x10); a != 0x100000 || a == cpu.Addr(0xFFFF, 0x10) {
		t.Errorf("SvcAddr(FFFF:0010) = %X", a)
	}

	// 轉換關閉（profile 開、PE=0）：現行行為。
	if err := m.SetCR0(0); err != nil {
		t.Fatal(err)
	}
	if got := d.readCString(0xFFFF, 0x10, 64); got != "LOW" {
		t.Errorf("轉換關閉：readCString(FFFF:0010) = %q，應折回低記憶體 %q", got, "LOW")
	}
	if a := m.SvcAddr(0xFFFF, 0x10); a != cpu.Addr(0xFFFF, 0x10) {
		t.Errorf("轉換關閉：SvcAddr = %X，應等於 cpu.Addr", a)
	}
}

// 寫的方向：服務層經 SvcAddr 寫回客體緩衝區（INT 21h AH=47h 取目前目錄，寫到 DS:SI）。
func TestServiceLayerWritesThroughSvcAddr(t *testing.T) {
	m, d := newPMDOSMachine(t)
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.SI] = 0xFFFF, 0x10
	m.CPU.R[cpu.DX] = 0 // 預設磁碟
	m.Ext[0] = 0x7E
	low := append([]byte(nil), m.Mem[:8]...)
	call(m, d, 0x21, 0x4700)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("AH=47h 失敗：AX=%04X", m.CPU.R[cpu.AX])
	}
	if m.Ext[0] == 0x7E {
		t.Errorf("服務層應寫到線性 0x100000（Ext[0]），Ext[0] 沒有被改")
	}
	if string(m.Mem[:8]) != string(low) {
		t.Errorf("服務層不該折回低記憶體：Mem[0:8] % X → % X", low, m.Mem[:8])
	}
}

// 診斷回溯：轉換開啟時用 Peek（不觸發讀取監看等副作用），轉換關閉時維持 Read8。
func TestDiagReadUsesPeekOnlyWhenTranslating(t *testing.T) {
	m, d := newPMDOSMachine(t)
	var reads int
	m.WatchReads(0, 0x1FFFFF, func(uint32, uint8) { reads++ })
	m.Ext[0x10] = 0x5A
	if got := d.diagRead8(0x100010); got != 0x5A {
		t.Errorf("diagRead8 = %02X", got)
	}
	if got := d.diagRead16(0x100010); got&0xFF != 0x5A {
		t.Errorf("diagRead16 = %04X", got)
	}
	if reads != 0 {
		t.Errorf("轉換開啟：診斷回溯不得有副作用（讀取監看觸發 %d 次）", reads)
	}
	if err := m.SetCR0(0); err != nil {
		t.Fatal(err)
	}
	d.diagRead8(0x1000)
	d.diagRead16(0x1000)
	if reads != 3 {
		t.Errorf("轉換關閉：維持現行的 Read8／Read16（讀取監看 1+2 次），實際 %d", reads)
	}
}

// noteKeyWord 在轉換開啟時以 SvcAddr 計算位址、以 Peek 讀取，且不觸發監看。
func TestNoteKeyWordTranslatingHasNoSideEffects(t *testing.T) {
	m, d := newPMDOSMachine(t)
	var reads int
	m.WatchReads(0, 0x1FFFFF, func(uint32, uint8) { reads++ })
	// SS:BP = FFFF:0010 → 線性 0x100000。[BP]=外層 BP、[BP+2]=返回 IP、[BP+4]=返回 CS。
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.BP] = 0xFFFF, 0x10
	put := func(off uint32, v uint16) { m.Ext[off], m.Ext[off+1] = uint8(v), uint8(v>>8) }
	put(0, 0x0020) // 外層 BP
	put(2, 0x1111)
	put(4, 0x2222)
	// 外層框在 FFFF:0020 → Ext[0x10…]
	put(0x10+2, 0x3333)
	put(0x10+4, 0x4444)
	d.noteKeyWord(m.CPU, "int16-AH00-bda", 0x1C0D)
	if len(d.KeyReads) != 1 {
		t.Fatalf("KeyReads = %d", len(d.KeyReads))
	}
	k := d.KeyReads[0]
	if k.CallerIP != 0x1111 || k.CallerCS != 0x2222 || k.Caller2IP != 0x3333 || k.Caller2CS != 0x4444 {
		t.Errorf("回溯讀到 %+v", k)
	}
	if reads != 0 {
		t.Errorf("診斷回溯觸發了 %d 次讀取監看", reads)
	}
}
