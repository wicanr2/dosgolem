package machine

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// A20 閘門與 HMA 的定址（`docs/spec/189`）。
//
// 位址環繞是**匯流排**的行為：8086 只有 20 條位址線所以 FFFF:0010 折回
// 0000:0000，286 之後多的那條由 A20 gate 控制。這幾條釘住閘門兩邊的行為。

// exec 把一段機器碼放到 1000:0000 執行 n 道指令。
func exec(t *testing.T, m *Machine, code []byte, n int) {
	t.Helper()
	m.WriteBytes(cpu.Addr(0x1000, 0), code)
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x1000, 0
	for i := 0; i < n; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestA20OpenReachesHMA 是規格 §6 的第 1 條：A20 開著時 段:偏移 到得了 HMA。
//
// ⚠ **這一條沒過的時候，症狀不是錯誤，是中斷向量表被蓋掉。** 程式問到
// HMA 存在（XMS 的 AH=00h 回 DX=1）、配置它、開了 A20，然後往
// FFFF:xxxx 寫的每一個位元組都落在 0000:xxxx。
func TestA20OpenReachesHMA(t *testing.T) {
	m := New()
	m.SetA20(true)
	m.Write8(0x10, 0xBB) // 環繞後會落到的地方（int 04h 的向量）

	// mov es:[bx], 0xCC，ES:BX ＝ FFFF:0020 → 線性 0x100010
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0xFFFF, 0x0020
	exec(t, m, []byte{0x26, 0xC6, 0x07, 0xCC}, 1)

	if got := m.Read8(MemSize + 0x10); got != 0xCC {
		t.Errorf("HMA(0x%X) ＝ %02X，預期 CC——寫進去的位元組沒到 HMA", MemSize+0x10, got)
	}
	if got := m.Read8(0x10); got != 0xBB {
		t.Errorf("0x10 ＝ %02X，預期 BB——中斷向量表被蓋掉了", got)
	}
}

// TestA20OpenReadsHMA：讀的方向也要到得了。
func TestA20OpenReadsHMA(t *testing.T) {
	m := New()
	m.SetA20(true)
	m.Write8(MemSize+0x10, 0xAA)
	m.Write8(0x10, 0xBB)

	// mov al, es:[bx]，ES:BX ＝ FFFF:0020
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0xFFFF, 0x0020
	exec(t, m, []byte{0x26, 0x8A, 0x07}, 1)

	if got := uint8(m.CPU.R[cpu.AX]); got != 0xAA {
		t.Errorf("AL ＝ %02X，預期 AA（HMA 的內容）", got)
	}
}

// TestA20ClosedWrapsAtOneMegabyte 是規格 §6 的第 2 條：關著時維持 8086 的環繞。
//
// **程式靠這個差別偵測 A20**：寫 0000:0000 再讀 FFFF:0010，值一樣就是
// 沒有 A20。所以這一條與上面那一條要同時成立，少一邊偵測就永遠得到
// 同一個答案。
func TestA20ClosedWrapsAtOneMegabyte(t *testing.T) {
	m := New() // 預設關著
	m.Write8(MemSize+0x10, 0xAA)
	m.Write8(0x10, 0xBB)

	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0xFFFF, 0x0020
	exec(t, m, []byte{0x26, 0x8A, 0x07}, 1)

	if got := uint8(m.CPU.R[cpu.AX]); got != 0xBB {
		t.Errorf("A20 關著讀 FFFF:0020 ＝ %02X，預期 BB（環繞回 0x10）", got)
	}
}

// TestA20DetectionSequence 走一次程式實際用的偵測序列。
func TestA20DetectionSequence(t *testing.T) {
	for _, c := range []struct {
		name string
		a20  bool
		same bool // 兩邊讀到一樣 ＝ 偵測到「沒有 A20」
	}{
		{"關著：兩邊是同一個位元組", false, true},
		{"開著：兩邊是不同的位元組", true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := New()
			m.SetA20(c.a20)
			m.Write8(0, 0x11)            // 0000:0000
			m.Write8(MemSize+0x10, 0x22) // FFFF:0010 在 A20 開著時的落點
			lo := m.Read8(0)
			var hi uint8
			m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0xFFFF, 0x0010
			exec(t, m, []byte{0x26, 0x8A, 0x07}, 1)
			hi = uint8(m.CPU.R[cpu.AX])
			if same := lo == hi; same != c.same {
				t.Errorf("0000:0000 ＝ %02X、FFFF:0010 ＝ %02X（一樣＝%v），預期一樣＝%v",
					lo, hi, same, c.same)
			}
		})
	}
}

// TestA20OpenExecutesFromHMA 是規格 §6 第 1 條的取指令那一半。
//
// 取指令有一條直接索引 Mem[] 的快路徑，而 HMA 不在 Mem[] 裡
// （見 syncCodeFastPath 的判準表）。
func TestA20OpenExecutesFromHMA(t *testing.T) {
	m := New()
	m.SetA20(true)
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF)
	// 線性 0x100000 ＝ FFFF:0010 起放 mov al, 0x42
	m.Write8(MemSize+0, 0xB0)
	m.Write8(MemSize+1, 0x42)
	// 環繞後的落點放另一道，才分得出跑的是哪一份。
	m.Write8(0, 0xB0)
	m.Write8(1, 0x99)

	m.CPU.Seg[cpu.CS], m.CPU.IP = 0xFFFF, 0x0010
	if err := m.Step(); err != nil {
		t.Fatal(err)
	}
	if got := uint8(m.CPU.R[cpu.AX]); got != 0x42 {
		t.Errorf("AL ＝ %02X，預期 42——取指令環繞回低記憶體了", got)
	}
}
