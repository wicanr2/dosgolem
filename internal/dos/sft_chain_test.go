package dos

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/429-dos-sft-chain §5 的 DOS 側測試（驗收 1、5、7）。
// 走鏈要呼叫 int 21h AH=52h，所以放在這裡（machine 不能引用 dos）。
// MemPoison 是全域變數，本檔不改它；若要改須以 t.Cleanup 還原，且不得 t.Parallel。

var sftCOM = []byte{0xB4, 0x4C, 0xCD, 0x21}

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

var sftLoaders = []struct {
	name string
	load func(*machine.Machine) error
}{
	{"EXE", func(m *machine.Machine) error { return m.LoadEXE(sftEXE()) }},
	{"COM", func(m *machine.Machine) error { return m.LoadCOM(sftCOM) }},
}

// sftMachine 建好機器：SetDOSFiles(n) → 載入 → 掛 DOS。
func sftMachine(t *testing.T, n int, load func(*machine.Machine) error) (*machine.Machine, *DOS) {
	t.Helper()
	m := machine.New()
	if err := m.SetDOSFiles(n); err != nil {
		t.Fatal(err)
	}
	if err := load(m); err != nil {
		t.Fatal(err)
	}
	d := New(m, t.TempDir())
	d.Install()
	return m, d
}

const sftMaxTables = 16

// walkSFT 從 AH=52h 的 ES:BX 起走鏈，回檔案總數。
// 先斷言 [BX+4] 是 0060:0000；offset 為 FFFFh 即停；超過 16 張表回錯誤。
// 逐表核對：第一張表 4 筆、前三筆 "CON "、第四筆 "AUX "，其餘表只看項目數。
func walkSFT(m *machine.Machine, d *DOS) (int, error) {
	call(m, d, 0x21, 0x5200)
	es, bx := m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX]
	off := m.Read16(cpu.Addr(es, bx+4))
	seg := m.Read16(cpu.Addr(es, bx+6))
	if seg != machine.SFTSeg || off != 0 {
		return 0, fmt.Errorf("[BX+4] = %04X:%04X，預期 %04X:0000", seg, off, machine.SFTSeg)
	}
	total := 0
	for i := 0; ; i++ {
		if i >= sftMaxTables {
			return 0, fmt.Errorf("走了 %d 張表仍未見鏈尾 FFFFh", sftMaxTables)
		}
		next := m.Read16(cpu.Addr(seg, off))
		nextSeg := m.Read16(cpu.Addr(seg, off+2))
		count := int(m.Read16(cpu.Addr(seg, off+4)))
		if i == 0 {
			if count != 4 {
				return 0, fmt.Errorf("第一張表項目數 %d，預期 4", count)
			}
			for e, want := range []string{"CON ", "CON ", "CON ", "AUX "} {
				a := cpu.Addr(seg, off+6+uint16(e)*59+0x20)
				var got [4]byte
				for k := range got {
					got[k] = m.Read8(a + uint32(k))
				}
				if string(got[:]) != want {
					return 0, fmt.Errorf("第一張表第 %d 筆名稱 %q，預期 %q", e, got[:], want)
				}
			}
		}
		total += count
		if next == 0xFFFF {
			return total, nil
		}
		off, seg = next, nextSeg
	}
}

// TestSFTChainContract 是驗收 1：EXE 與 COM 各一，SetDOSFiles(25) 後走鏈總數為 25。
func TestSFTChainContract(t *testing.T) {
	for _, l := range sftLoaders {
		t.Run(l.name, func(t *testing.T) {
			m, d := sftMachine(t, 25, l.load)
			total, err := walkSFT(m, d)
			if err != nil {
				t.Fatal(err)
			}
			if total != 25 {
				t.Errorf("總數 %d，預期 25", total)
			}
			// 第二張表的 next 是 FFFF:FFFF，項目數 21
			if got := m.Read16(cpu.Addr(machine.SFT2Seg, 4)); got != 21 {
				t.Errorf("第二張表項目數 %d，預期 21", got)
			}
			if m.ROMWrites != 0 {
				t.Errorf("ROMWrites = %d，預期 0", m.ROMWrites)
			}
		})
	}
}

// TestSFTChainOffIsBroken 釘住預設行為：未開啟時 [BX+4] 是 0000:0000，walker 報錯。
// 這是觸發案例的症狀（走鏈從中斷向量表開始），也證明 walker 的第一道檢查會失敗。
func TestSFTChainOffIsBroken(t *testing.T) {
	m, d := sftMachine(t, 0, func(m *machine.Machine) error { return m.LoadCOM(sftCOM) })
	if _, err := walkSFT(m, d); err == nil {
		t.Fatal("預設關閉時 walker 應回錯誤")
	}
}

// TestSFTChainSizes 走過邊界值，總數與設定值相同。
func TestSFTChainSizes(t *testing.T) {
	for _, n := range []int{6, 7, 30, 255} {
		m, d := sftMachine(t, n, func(m *machine.Machine) error { return m.LoadCOM(sftCOM) })
		total, err := walkSFT(m, d)
		if err != nil || total != n {
			t.Errorf("n=%d：總數 %d，err=%v", n, total, err)
		}
	}
}

// TestSFTWalkerNegativeControls 是驗收 7：walker 本身要能失敗。
// 改寫一律直接改 m.Mem（F000 段經 Write16 會被 ROM 保護吞掉，測試會變成假紅）。
func TestSFTWalkerNegativeControls(t *testing.T) {
	t.Run("鏈尾改成0000:0000", func(t *testing.T) {
		m, d := sftMachine(t, 25, func(m *machine.Machine) error { return m.LoadCOM(sftCOM) })
		clear(m.Mem[0xF0000 : 0xF0000+4])
		_, err := walkSFT(m, d)
		if err == nil {
			t.Fatal("鏈尾不是 FFFFh，walker 應回錯誤")
		}
		t.Logf("walker 回報：%v", err)
	})
	t.Run("第一張表檔案數改成5", func(t *testing.T) {
		m, d := sftMachine(t, 25, func(m *machine.Machine) error { return m.LoadCOM(sftCOM) })
		m.Mem[0x604] = 5
		if _, err := walkSFT(m, d); err == nil {
			t.Fatal("第一張表檔案數是 5，walker 應回錯誤")
		}
	})
	t.Run("總數不等於設定值", func(t *testing.T) {
		m, d := sftMachine(t, 25, func(m *machine.Machine) error { return m.LoadCOM(sftCOM) })
		m.Mem[0xF0004] = 20
		total, err := walkSFT(m, d)
		if err != nil || total == 25 {
			t.Fatalf("改了第二張表項目數，總數應 != 25：total=%d err=%v", total, err)
		}
	})
}

// TestSFTConstantsGuard 是驗收 5：常數守門。
func TestSFTConstantsGuard(t *testing.T) {
	const fontStart, bootTail = 0xFFA6E, 0xFFFF0
	if machine.SFTSeg*16+6+4*59 > machine.LOLSeg*16 {
		t.Errorf("第一張表 %#x 超過 LOLSeg*16 = %#x", machine.SFTSeg*16+6+4*59, machine.LOLSeg*16)
	}
	end2 := machine.SFT2Seg*16 + 6 + 251*59
	if end2 >= fontStart || end2 >= bootTail {
		t.Errorf("第二張表上界 %#x 碰到字型 %#x 或開機尾段 %#x", end2, fontStart, bootTail)
	}
	lo, hi := machine.SFT2Seg*16, end2
	overlap := func(a, b int) bool { return lo < b && a < hi }
	if overlap(umbStart*16, umbEnd*16) {
		t.Error("SFT2Seg 與 UMB 範圍重疊")
	}
	if overlap(machine.EMSFrameSeg*16, machine.EMSFrameSeg*16+0x10000) {
		t.Error("SFT2Seg 與 EMS 頁框重疊")
	}
	if overlap(0xE0000, 0xF0000) {
		t.Error("SFT2Seg 與 E000 段重疊")
	}
}
