package machine

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// BIOS ROM 尾巴與唯讀語意（`docs/spec/191-bios-rom-tail`）。

// TestROMTailAtBoot：開機之後 `FFFF0`–`FFFFF` 是 PC/AT 的重置跳躍、BIOS 日期、機型位元組。
func TestROMTailAtBoot(t *testing.T) {
	m := New()
	want := []byte{0xEA, 0x5B, 0xE0, 0x00, 0xF0, '0', '1', '/', '0', '1', '/', '9', '2', 0x00, 0xFC, 0x55}
	for i, w := range want {
		if got := m.Read8(0xFFFF0 + uint32(i)); got != w {
			t.Fatalf("%05X = %02X，要 %02X", 0xFFFF0+i, got, w)
		}
	}
}

// TestROMTailThroughFFFFPointer 是規格 §1 的情境：把 `FFFF:FFFF` 當物件指標，
// `[si+2]` 落在 `FFFF1`。以前那裡是 0，《銀河英雄傳說III SP》讀到 AH=0 之後
// 把資料段寫壞（`~/cht/logh3/docs/re/334`）。
func TestROMTailThroughFFFFPointer(t *testing.T) {
	m := New()
	// mov ax,FFFF / mov es,ax / mov si,FFFF / mov ah,es:[si+2]
	exec(t, m, []byte{0xB8, 0xFF, 0xFF, 0x8E, 0xC0, 0xBE, 0xFF, 0xFF, 0x26, 0x8A, 0x64, 0x02}, 4)
	if ah := m.CPU.R[cpu.AX] >> 8; ah != 0x5B {
		t.Fatalf("AH = %02X，要 5B（FFFF1）", ah)
	}
}

// TestROMWritesIgnored：寫進 `F0000`–`FFFFF` 一律忽略，但要記下來。
func TestROMWritesIgnored(t *testing.T) {
	m := New()
	m.Write8(0xFFFF1, 0x00)
	m.WriteBytes(0xF0000, []byte{1, 2})
	m.Write16(0xFFFFE, 0x1234)
	// CPU 的寫入：mov byte es:[di],77h，ES:DI ＝ F000:FFFE
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DI] = 0xF000, 0xFFFE
	exec(t, m, []byte{0x26, 0xC6, 0x05, 0x77}, 1)

	if got := m.Read8(0xFFFF1); got != 0x5B {
		t.Fatalf("FFFF1 被改成 %02X", got)
	}
	if got := m.Read8(0xF0000); got != 0 {
		t.Fatalf("F0000 被改成 %02X", got)
	}
	if got := m.Read8(0xFFFFE); got != 0xFC {
		t.Fatalf("FFFFE 被改成 %02X", got)
	}
	if m.ROMWrites != 6 {
		t.Fatalf("ROMWrites = %d，要 6", m.ROMWrites)
	}
	last := m.ROMWriteLog[len(m.ROMWriteLog)-1]
	if last.Addr != 0xFFFFE || last.Val != 0x77 || last.CS != 0x1000 || last.IP != 0 {
		t.Fatalf("最後一筆 = %+v", last)
	}
	// 低於 ROM 的那一格照常可寫。
	m.Write8(ROMBase-1, 0xAB)
	if m.Read8(ROMBase-1) != 0xAB {
		t.Fatal("EFFFF 寫不進去")
	}
}

// TestStateKeepsA20AndHMA：狀態檔 v3 存 A20 與 HMA，讀回來定址行為一樣。
func TestStateKeepsA20AndHMA(t *testing.T) {
	m := New()
	m.SetA20(true)
	m.Write8(MemSize+0x10, 0xCC)
	var buf bytes.Buffer
	if err := m.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	n := New()
	if err := n.LoadState(&buf); err != nil {
		t.Fatal(err)
	}
	if !n.A20Enabled() || n.Read8(MemSize+0x10) != 0xCC || n.LegacyState {
		t.Fatalf("A20=%v HMA[10]=%02X legacy=%v", n.A20Enabled(), n.Read8(MemSize+0x10), n.LegacyState)
	}
	// A20 開著時取指令不能走快路徑（`syncCodeFastPath`）。
	if n.CPU.Code != nil {
		t.Fatal("A20 開著卻走快路徑")
	}
}

// TestLegacyStateRefillsROM：v2 狀態檔沒有 A20／HMA，記憶體裡的 ROM 尾巴是 0。
// 讀進來要是 A20 關、HMA 空，而 ROM 尾巴照這一版的內容。
func TestLegacyStateRefillsROM(t *testing.T) {
	m := New()
	var buf bytes.Buffer
	if err := m.SaveState(&buf); err != nil {
		t.Fatal(err)
	}
	var s machineState
	if err := gob.NewDecoder(&buf).Decode(&s); err != nil {
		t.Fatal(err)
	}
	s.Version = 2
	s.A20, s.HMA = false, nil
	for i := MemSize - 16; i < MemSize; i++ {
		s.Mem[i] = 0
	}
	buf.Reset()
	if err := gob.NewEncoder(&buf).Encode(&s); err != nil {
		t.Fatal(err)
	}

	n := New()
	n.SetA20(true)
	n.Write8(MemSize, 0x11)
	if err := n.LoadState(&buf); err != nil {
		t.Fatal(err)
	}
	if n.A20Enabled() || n.hma[0] != 0 || !n.LegacyState {
		t.Fatalf("A20=%v HMA[0]=%02X legacy=%v", n.A20Enabled(), n.hma[0], n.LegacyState)
	}
	if n.Read8(0xFFFF1) != 0x5B || n.Read8(0xFFFFE) != 0xFC {
		t.Fatal("v2 狀態檔讀進來之後 ROM 尾巴沒有補回")
	}
}

// TestSnapshotKeepsA20：記憶體內快照同樣要帶 A20 與 HMA。
func TestSnapshotKeepsA20(t *testing.T) {
	m := New()
	m.SetA20(true)
	m.Write8(MemSize+1, 0x42)
	s := m.Snapshot()
	m.SetA20(false)
	m.hma[1] = 0
	m.Restore(s)
	if !m.A20Enabled() || m.Read8(MemSize+1) != 0x42 {
		t.Fatalf("A20=%v HMA[1]=%02X", m.A20Enabled(), m.hma[1])
	}
}
