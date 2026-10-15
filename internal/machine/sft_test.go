package machine

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// docs/spec/429-dos-sft-chain §5 的機器側測試（驗收 2(a)、3、4、6）。
// 走鏈（int 21h AH=52h）的測試在 internal/dos，因為 machine 不能引用 dos。
//
// MemPoison 是全域變數，相關測試以 t.Cleanup 還原，不得使用 t.Parallel。

var sftCOM = []byte{0xB4, 0x4C, 0xCD, 0x21}

// sftEXE 組出最小 MZ（檔頭 2 段、映像 16 bytes），內容同 sftCOM。
func sftEXE() []byte {
	b := make([]byte, 32+16)
	copy(b, "MZ")
	le := binary.LittleEndian
	le.PutUint16(b[2:], uint16(len(b)%512))
	le.PutUint16(b[4:], uint16((len(b)+511)/512))
	le.PutUint16(b[8:], 2)
	le.PutUint16(b[12:], 0xFFFF)
	le.PutUint16(b[16:], 0x100)
	le.PutUint16(b[24:], 0x1C)
	copy(b[32:], sftCOM)
	return b
}

type sftLoader struct {
	name string
	load func(*Machine) error
}

var sftLoaders = []sftLoader{
	{"EXE", func(m *Machine) error { return m.LoadEXE(sftEXE()) }},
	{"COM", func(m *Machine) error { return m.LoadCOM(sftCOM) }},
}

func setPoison(t *testing.T, v uint8) {
	t.Helper()
	old := MemPoison
	MemPoison = v
	t.Cleanup(func() { MemPoison = old })
}

// sftRanges 是 SFT 可能動到的三段：[起, 迄)。
var sftRanges = [][2]uint32{
	{0x0600, 0x0700},
	{0x0714, 0x0718},
	{0xF0000, 0xF4000},
}

// TestSFTOffDoesNotTouchMemory 是驗收 2(a)：未呼叫 SetDOSFiles 時，
// 三段記憶體與 New() 剛建好時逐位元組相同（MemPoison 為 0 與 0xCC 各一次）。
func TestSFTOffDoesNotTouchMemory(t *testing.T) {
	for _, poison := range []uint8{0, 0xCC} {
		for _, l := range sftLoaders {
			setPoison(t, poison)
			fresh := New()
			m := New()
			if err := l.load(m); err != nil {
				t.Fatal(err)
			}
			if m.DOSFiles() != 0 {
				t.Fatalf("DOSFiles() = %d，預期 0", m.DOSFiles())
			}
			for _, r := range sftRanges {
				if !bytes.Equal(m.Mem[r[0]:r[1]], fresh.Mem[r[0]:r[1]]) {
					t.Errorf("poison=%#x %s：%#x-%#x 與 New() 不同", poison, l.name, r[0], r[1])
				}
			}
		}
	}
}

// TestSFTDiffersOnlyInTables 是驗收 3：poison 0xCC 下，n=25 與 n=0 載入同一支程式，
// 差異位址集合恰好等於三個表範圍的聯集。
func TestSFTDiffersOnlyInTables(t *testing.T) {
	for _, l := range sftLoaders {
		setPoison(t, 0xCC)
		off, on := New(), New()
		if err := l.load(off); err != nil {
			t.Fatal(err)
		}
		if err := on.SetDOSFiles(25); err != nil {
			t.Fatal(err)
		}
		if err := l.load(on); err != nil {
			t.Fatal(err)
		}
		want := func(a int) bool {
			return (a >= 0x600 && a <= 0x6F1) ||
				(a >= 0x714 && a <= 0x717) ||
				(a >= 0xF0000 && a <= 0xF0000+6+21*59-1)
		}
		for a := range off.Mem {
			if got := off.Mem[a] != on.Mem[a]; got != want(a) {
				t.Fatalf("%s：位址 %#x 差異=%v，預期 %v（off=%#x on=%#x）",
					l.name, a, got, want(a), off.Mem[a], on.Mem[a])
			}
		}
	}
}

// TestSFTLayout 釘住表頭與項目（驗收 2(b) 之外的位元組級內容）。
func TestSFTLayout(t *testing.T) {
	setPoison(t, 0xCC)
	m := New()
	if err := m.SetDOSFiles(25); err != nil {
		t.Fatal(err)
	}
	if err := m.LoadCOM(sftCOM); err != nil {
		t.Fatal(err)
	}
	u16 := func(a uint32) uint16 { return binary.LittleEndian.Uint16(m.Mem[a:]) }
	if u16(0x714) != 0 || u16(0x716) != 0x0060 {
		t.Errorf("LOL+14h = %04X:%04X，預期 0060:0000", u16(0x716), u16(0x714))
	}
	if u16(0x600) != 0 || u16(0x602) != 0xF000 || u16(0x604) != 4 {
		t.Errorf("第一張表頭不對")
	}
	for i, name := range []string{"CON ", "CON ", "CON ", "AUX "} {
		a := 0x606 + i*59 + 0x20
		if got := string(m.Mem[a : a+4]); got != name {
			t.Errorf("第 %d 筆名稱 %q，預期 %q", i, got, name)
		}
	}
	if u16(0xF0000) != 0xFFFF || u16(0xF0002) != 0xFFFF || u16(0xF0004) != 21 {
		t.Errorf("第二張表頭不對")
	}
	if m.ROMWrites != 0 {
		t.Errorf("ROMWrites = %d，預期 0", m.ROMWrites)
	}
	// 表內未命名位元組在 poison 下必須是 0
	for i := 0x606; i < 0x6F2; i++ {
		inName := false
		for e := 0; e < 4; e++ {
			if o := 0x606 + e*59 + 0x20; i >= o && i < o+4 {
				inName = true
			}
		}
		if !inName && m.Mem[i] != 0 {
			t.Fatalf("%#x = %#x，預期 0", i, m.Mem[i])
		}
	}
	for i := 0xF0006; i < 0xF0006+21*59; i++ {
		if m.Mem[i] != 0 {
			t.Fatalf("%#x = %#x，預期 0", i, m.Mem[i])
		}
	}
}

// TestSFTBounds 是驗收 4：邊界與載入後呼叫。
func TestSFTBounds(t *testing.T) {
	for _, n := range []int{1, 5, 256, -1, 1000} {
		if err := New().SetDOSFiles(n); err == nil {
			t.Errorf("SetDOSFiles(%d) 應回錯誤", n)
		}
	}
	for _, c := range []struct{ n, rest int }{{6, 2}, {255, 251}} {
		setPoison(t, 0xCC)
		m := New()
		if err := m.SetDOSFiles(c.n); err != nil {
			t.Fatal(err)
		}
		if m.DOSFiles() != c.n {
			t.Errorf("DOSFiles() = %d，預期 %d", m.DOSFiles(), c.n)
		}
		if err := m.LoadCOM(sftCOM); err != nil {
			t.Fatal(err)
		}
		if got := binary.LittleEndian.Uint16(m.Mem[0xF0004:]); int(got) != c.rest {
			t.Errorf("n=%d：第二張表項目數 %d，預期 %d", c.n, got, c.rest)
		}
		end := 0xF0000 + 6 + c.rest*59
		if end > 0xFFA6E {
			t.Errorf("n=%d：第二張表結尾 %#x 超過字型起點", c.n, end)
		}
		for i := 0xF0006; i < end; i++ {
			if m.Mem[i] != 0 {
				t.Fatalf("n=%d：%#x = %#x，預期 0", c.n, i, m.Mem[i])
			}
		}
		// 表之後的 ROM 區維持 poison（不多寫）
		if m.Mem[end] != 0xCC {
			t.Errorf("n=%d：表後第一個 byte %#x，預期 poison 0xCC", c.n, m.Mem[end])
		}
	}
	// 載入後呼叫一律回錯誤，含 n=0
	for _, l := range sftLoaders {
		m := New()
		if err := l.load(m); err != nil {
			t.Fatal(err)
		}
		for _, n := range []int{0, 25} {
			if err := m.SetDOSFiles(n); err == nil {
				t.Errorf("%s 載入後 SetDOSFiles(%d) 應回錯誤", l.name, n)
			}
		}
	}
	// 載入失敗不算已載入
	m := New()
	if err := m.LoadEXE([]byte("MZ")); err == nil {
		t.Fatal("截斷的 EXE 應載入失敗")
	}
	if err := m.SetDOSFiles(25); err != nil {
		t.Errorf("載入失敗後 SetDOSFiles 應仍可用：%v", err)
	}
}

// TestSFTSaveLoadState 是驗收 6 的 SaveState/LoadState 部分。
func TestSFTSaveLoadState(t *testing.T) {
	setPoison(t, 0xCC)
	src := New()
	if err := src.SetDOSFiles(25); err != nil {
		t.Fatal(err)
	}
	if err := src.LoadCOM(sftCOM); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := src.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	dst := New()
	if err := dst.LoadState(&buf); err != nil {
		t.Fatal(err)
	}
	for _, r := range sftRanges {
		if !bytes.Equal(src.Mem[r[0]:r[1]], dst.Mem[r[0]:r[1]]) {
			t.Errorf("%#x-%#x LoadState 後不同", r[0], r[1])
		}
	}
	if dst.DOSFiles() != 0 {
		t.Errorf("LoadState 後 DOSFiles() = %d，預期 0", dst.DOSFiles())
	}
	if err := dst.SetDOSFiles(25); err == nil {
		t.Error("LoadState 後 SetDOSFiles 應回錯誤")
	}
}

// TestSFTSnapshotRestoreKeepsLoaded 是驗收 6 的 Snapshot/Restore 部分。
func TestSFTSnapshotRestoreKeepsLoaded(t *testing.T) {
	src := New()
	if err := src.SetDOSFiles(25); err != nil {
		t.Fatal(err)
	}
	if err := src.LoadCOM(sftCOM); err != nil {
		t.Fatal(err)
	}
	snap := src.Snapshot()
	dst := New()
	dst.Restore(snap)
	if err := dst.SetDOSFiles(25); err == nil {
		t.Error("Restore 後 SetDOSFiles 應回錯誤")
	}
	for _, r := range sftRanges {
		if !bytes.Equal(src.Mem[r[0]:r[1]], dst.Mem[r[0]:r[1]]) {
			t.Errorf("%#x-%#x Restore 後不同", r[0], r[1])
		}
	}
	// 反向：拍在載入前的快照，Restore 到已載入的機器後旗標回到未載入
	fresh := New()
	snap0 := fresh.Snapshot()
	if err := fresh.LoadCOM(sftCOM); err != nil {
		t.Fatal(err)
	}
	fresh.Restore(snap0)
	if err := fresh.SetDOSFiles(25); err != nil {
		t.Errorf("Restore 到載入前快照後 SetDOSFiles 應可用：%v", err)
	}
}
