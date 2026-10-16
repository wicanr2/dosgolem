package machine

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/443-segmented-32bit-code-and-stack §3 第 5 項：鏡像等價。
//
// 同一段指令位元組 rest，在（D=0，前綴集合 P）與（D=1，66／67 與 P 相反）兩種條件下各執行一步，暫存器、旗標、
// IP 與資料記憶體逐位元組相同。兩邊的指令起點錯開前綴長度差，讓指令結束時的 IP 相同（相對轉移的目標也因此相同）。
//
// 語料：66 的部分抄自 internal/cpu/ops386_wide_test.go 各測試的位元組序列（該檔沒有集中的語料表）；67 的部分是
// ModRM32、moffs32 與 SIB 形式；另有 D=0 無前綴對 D=1 的 66 67（相對轉移、loop、字串與一般指令）。只收本規格兩邊都
// 開放的形狀（D=0 帶 67 的隱含位址指令、D=0 帶 66 的相對轉移不收）。

type mirrorCase struct {
	p66, p67 bool   // D=0 那一邊的前綴
	rest     string // 前綴之後的位元組（十六進位，空白分隔）
}

var mirrorCorpus = []mirrorCase{
	// ---- 66（ops386_wide_test.go）----
	{true, false, "05 01 00 00 00"}, {true, false, "83 E8 01"}, {true, false, "83 C0 FF"},
	{true, false, "81 F8 FE FF FF 7F"}, {true, false, "25 FF 00 FF 00"}, {true, false, "A9 00 00 00 80"},
	{true, false, "B9 78 56 34 12"}, {true, false, "89 0E 00 02"}, {true, false, "8B 16 00 02"},
	{true, false, "C7 06 04 02 EF BE AD DE"}, {true, false, "87 0E 04 02"}, {true, false, "98"},
	{true, false, "99"}, {true, false, "93"}, {true, false, "A1 00 03"}, {true, false, "A3 00 03"},
	{true, false, "50"}, {true, false, "5A"}, {true, false, "54"}, {true, false, "68 78 56 34 12"},
	{true, false, "6A FE"}, {true, false, "60"}, {true, false, "61"}, {true, false, "9C"}, {true, false, "9D"},
	{true, false, "C1 E0 10"}, {true, false, "D1 E8"}, {true, false, "D3 E0"}, {true, false, "F7 E3"},
	{true, false, "F7 F3"}, {true, false, "F7 D8"}, {true, false, "40"}, {true, false, "48"},
	{true, false, "0F B6 C3"}, {true, false, "0F BF C3"}, {true, false, "0F A3 C8"}, {true, false, "0F BC C3"},
	{true, false, "0F AF C3"}, {true, false, "0F A4 C3 04"}, {true, false, "0F 84 10 00 00 00"},
	{true, false, "0F 85 10 00 00 00"}, {true, false, "A5"}, {true, false, "F3 A5"}, {true, false, "AB"},
	{true, false, "AD"}, {true, false, "A7"}, {true, false, "AF"}, {true, false, "F3 A7"},
	{true, false, "01 18"}, {true, false, "8D 40 10"}, {true, false, "06"}, {true, false, "1E"},
	{true, false, "FF 06 00 02"}, {true, false, "8F 06 00 02"},
	// ---- 67（ModRM32、moffs32、SIB）----
	{false, true, "8B 03"}, {true, true, "8B 03"}, {false, true, "8A 44 24 08"}, {true, true, "8B 44 24 08"},
	{false, true, "8B 1C 88"}, {false, true, "8B 5C 50 06"}, {false, true, "8B 1D 34 12 00 00"},
	{false, true, "8B 1C D5 00 01 00 00"}, {false, true, "8B 45 F0"}, {false, true, "8B 1C 68"},
	{false, true, "A1 00 03 00 00"}, {true, true, "A1 00 03 00 00"}, {false, true, "A0 10 03 00 00"},
	{false, true, "A2 10 03 00 00"}, {false, true, "A3 20 03 00 00"}, {true, true, "A3 20 03 00 00"},
	{false, true, "89 18"}, {true, true, "89 18"}, {false, true, "01 18"}, {false, true, "C6 00 55"},
	{false, true, "8D 04 88"}, {true, true, "8D 04 88"}, {false, true, "FF 00"}, {true, true, "FF 00"},
	{false, true, "F7 24 8D 00 01 00 00"}, {true, true, "0F B6 04 88"},
	// ---- 無前綴（D=1 那一邊是 66 67）----
	{false, false, "8B 07"}, {false, false, "01 C3"}, {false, false, "EB 10"}, {false, false, "EB F0"},
	{false, false, "74 10"}, {false, false, "75 10"}, {false, false, "E2 10"}, {false, false, "E1 10"},
	{false, false, "E3 10"}, {false, false, "F7 E3"}, {false, false, "A5"}, {false, false, "F3 A4"},
	{false, false, "50"}, {false, false, "58"}, {false, false, "0F 84 10 00"},
}

// mirrorMachine 造一邊：d1 決定 CS（28h 為 D=1、18h 為 D=0，基底同為 segCode），code 放在 segCode+start。
func mirrorMachine(t *testing.T, d1 bool, code []byte, start uint16) *Machine {
	t.Helper()
	var m *Machine
	if d1 {
		m = d1Prog(t, nil)
	} else {
		m = d0Prog(t, nil)
	}
	copy(m.Mem[segCode+uint32(start):], code)
	m.CPU.IP = start
	x := uint32(4242)
	for a := uint32(0x1000); a < 0x11000; a++ { // DS、ES、SS 的資料（基底 1000h）
		x = x*1103515245 + 12345
		m.Mem[a] = byte(x >> 16)
	}
	m.CPU.SetSegCache(cpu.FS, dataCache(segData, 0xFFFF, 0x93))
	m.CPU.FSeg = 0x08
	for r, v := range map[int]uint32{cpu.AX: 0x120, cpu.BX: 0x340, cpu.CX: 0x10, cpu.DX: 3,
		cpu.SP: 0x8000, cpu.BP: 0x500, cpu.SI: 0x200, cpu.DI: 0x400} {
		setReg32(m, r, v)
	}
	m.CPU.SetFlags(cpu.ZF | cpu.CF)
	return m
}

func flipPrefix(p66, p67 bool) []byte {
	var b []byte
	if p66 {
		b = append(b, 0x66)
	}
	if p67 {
		b = append(b, 0x67)
	}
	return b
}

func TestSeg443MirrorEquivalence(t *testing.T) {
	for _, c := range mirrorCorpus {
		rest, err := hex.DecodeString(strings.ReplaceAll(c.rest, " ", ""))
		if err != nil {
			t.Fatal(err)
		}
		code0 := append(flipPrefix(c.p66, c.p67), rest...)
		code1 := append(flipPrefix(!c.p66, !c.p67), rest...)
		const start0 = 0x100
		start1 := uint16(start0 + len(code0) - len(code1))
		a := mirrorMachine(t, false, code0, start0)
		b := mirrorMachine(t, true, code1, start1)
		name := "D=0 % X ↔ D=1 % X"
		errA, errB := a.Step(), b.Step()
		if errA != nil || errB != nil {
			t.Errorf(name+"：兩邊都應成功（%v；%v）", code0, code1, errA, errB)
			continue
		}
		ca, cb := a.CPU, b.CPU
		if ca.R != cb.R || ca.Hi != cb.Hi || ca.IP != cb.IP || ca.Flags != cb.Flags {
			t.Errorf(name+"：暫存器不同\n  D=0 R=%04X Hi=%04X IP=%04X F=%04X\n  D=1 R=%04X Hi=%04X IP=%04X F=%04X",
				code0, code1, ca.R, ca.Hi, ca.IP, ca.Flags, cb.R, cb.Hi, cb.IP, cb.Flags)
		}
		for _, i := range []int{cpu.ES, cpu.SS, cpu.DS} {
			if ca.Seg[i] != cb.Seg[i] || ca.SegCache(i) != cb.SegCache(i) {
				t.Errorf(name+"：段 %d 不同", code0, code1, i)
			}
		}
		if ca.FSeg != cb.FSeg || ca.GSeg != cb.GSeg || ca.SegCache(cpu.FS) != cb.SegCache(cpu.FS) {
			t.Errorf(name+"：FS／GS 不同", code0, code1)
		}
		// 資料記憶體逐位元組相同（程式所在的 segCode 一段除外）。
		for _, m := range []*Machine{a, b} {
			for i := uint32(0); i < 0x10000; i++ {
				m.Mem[segCode+i] = 0
			}
		}
		if !bytes.Equal(a.Mem, b.Mem) {
			for i := range a.Mem {
				if a.Mem[i] != b.Mem[i] {
					t.Errorf(name+"：記憶體第一個差異在 %05X：%02X ↔ %02X", code0, code1, i, a.Mem[i], b.Mem[i])
					break
				}
			}
		}
	}
}
