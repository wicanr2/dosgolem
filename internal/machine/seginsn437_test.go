package machine

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/437-segmented-32bit-far-transfers §3 第 2–5 項：分段模式下 66 前綴的 16:32 遠轉移。
// 環境見 segmode_test.go 的 segProg：CS、DS、SS 值 0100h（快取基底 1000h），程式在 IP=0100h，SP=FFFEh；
// GDT 另加 28h（D=1 碼段）與 30h（基底 1000h 的 D=0 碼段）。

const (
	sel437D1   = 0x28 // D=1 的 32 位元碼段
	sel437Back = 0x30 // 基底 1000h、D=0 的碼段（返回用）
)

func seg437(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := segProg(t, code)
	putDesc(m, segGDT, sel437D1, segCode, 0xFFFF, 0x9A, 0x4)
	putDesc(m, segGDT, sel437Back, 0x1000, 0xFFFF, 0x9A, 0)
	return m
}

func le32(m *Machine, a uint32) uint32 { return uint32(le16(m, a)) | uint32(le16(m, a+2))<<16 }

// farPtr32 回 16:32 遠指標的 6 bytes。
func farPtr32(off uint32, sel uint16) []byte {
	return []byte{byte(off), byte(off >> 8), byte(off >> 16), byte(off >> 24), byte(sel), byte(sel >> 8)}
}

func wantCS(t *testing.T, m *Machine, what string, cs, ip uint16, base uint32) {
	t.Helper()
	c := m.CPU
	if c.Seg[cpu.CS] != cs || c.IP != ip || c.SegCache(cpu.CS).Base != base || c.SegCache(cpu.CS).Attr&0x4000 != 0 {
		t.Errorf("%s：CS:IP=%04X:%04X 快取 %+v，應為 %04X:%04X 基底 %X", what, c.Seg[cpu.CS], c.IP, c.SegCache(cpu.CS), cs, ip, base)
	}
}

// ---- §3 第 2 項 -------------------------------------------------------------

// 66 FF /5 帶 36 段前綴：從 SS 快取讀遠指標（DS 的同一偏移放另一個指標）。仿 JEMM 5DA0。
func Test437JmpFarMemSSPrefix(t *testing.T) {
	m := seg437(t, []byte{0x66, 0x36, 0xFF, 0x2E, 0x4C, 0x3F}) // jmp m16:32 ss:[3F4C]
	m.CPU.SetSegCache(cpu.SS, dataCache(segData, 0xFFFF, 0x93))
	copy(m.Mem[segData+0x3F4C:], farPtr32(0x9E, 0x18))
	copy(m.Mem[0x1000+0x3F4C:], farPtr32(0x55, sel437Back)) // DS:[3F4C]，不得被讀到
	mustStep(t, m, "66 36 FF /5")
	wantCS(t, m, "66 36 FF /5", 0x18, 0x9E, segCode)
}

// 66 67 FF /5 [ebx+disp8]：32 位元有效位址超過 FFFFh，指標跨 1FFFFh／20000h，+4 不截斷。
func Test437JmpFarMemAddr32(t *testing.T) {
	m := seg437(t, []byte{0x66, 0x67, 0xFF, 0x6B, 0x10}) // jmp m16:32 [ebx+10h]
	m.CPU.SetSegCache(cpu.DS, dataCache(segData, 0xFFFFF, 0x93))
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0xFFEE, 0x0001 // EBX=1FFEEh → EA=1FFFEh
	copy(m.Mem[segData+0x1FFFE:], farPtr32(0x1234, 0x18))
	copy(m.Mem[segData+0x0002:], []byte{byte(sel437Back), 0}) // 截斷成 16 位元時的 +4
	mustStep(t, m, "66 67 FF /5")
	wantCS(t, m, "66 67 FF /5", 0x18, 0x1234, segCode)
}

// 66 EA ptr16:32。
func Test437JmpFarImm(t *testing.T) {
	m := seg437(t, append([]byte{0x66, 0xEA}, farPtr32(0x1234, 0x18)...))
	mustStep(t, m, "66 EA")
	wantCS(t, m, "66 EA", 0x18, 0x1234, segCode)
}

// 66 9A 與 66 FF /3：推 8 bytes，[SP]=EIP（下一道）、[SP+4]=CS（高 16 位元 0）。
func Test437CallFarPushes8(t *testing.T) {
	for _, c := range []struct {
		name string
		code []byte
		next uint16
	}{
		{"66 9A", append([]byte{0x66, 0x9A}, farPtr32(0x1234, 0x18)...), 0x108},
		{"66 FF /3", []byte{0x66, 0xFF, 0x1E, 0x00, 0x02}, 0x105},
	} {
		m := seg437(t, c.code)
		copy(m.Mem[0x1200:], farPtr32(0x1234, 0x18))
		for i := uint32(0); i < 8; i++ {
			m.Mem[0x1000+0xFFF6+i] = 0xCC
		}
		cs, sp := m.CPU.Seg[cpu.CS], m.CPU.R[cpu.SP]
		mustStep(t, m, c.name)
		wantCS(t, m, c.name, 0x18, 0x1234, segCode)
		if m.CPU.R[cpu.SP] != sp-8 {
			t.Errorf("%s：SP %04X→%04X，應減 8", c.name, sp, m.CPU.R[cpu.SP])
		}
		nsp := 0x1000 + uint32(m.CPU.R[cpu.SP])
		if got := le32(m, nsp); got != uint32(c.next) {
			t.Errorf("%s：[SP]=%08X，應為 %08X", c.name, got, c.next)
		}
		if got := le32(m, nsp+4); got != uint32(cs) {
			t.Errorf("%s：[SP+4]=%08X，應為 %08X", c.name, got, cs)
		}
	}
}

// 66 CB：SP += 8；66 CA 04 00：SP += 12；彈出的 CS 高半非 0 仍成功。
func Test437RetFar(t *testing.T) {
	for _, c := range []struct {
		name string
		code []byte
		add  uint16
	}{
		{"66 CB", []byte{0x66, 0xCB}, 8},
		{"66 CA 04 00", []byte{0x66, 0xCA, 0x04, 0x00}, 12},
	} {
		m := seg437(t, c.code)
		m.CPU.R[cpu.SP] = 0xFF00
		put32(m, 0x1000+0xFF00, 0x1234)
		put32(m, 0x1000+0xFF04, 0xDEAD0018)
		mustStep(t, m, c.name)
		wantCS(t, m, c.name, 0x18, 0x1234, segCode)
		if m.CPU.R[cpu.SP] != 0xFF00+c.add {
			t.Errorf("%s：SP=%04X，應為 %04X", c.name, m.CPU.R[cpu.SP], 0xFF00+c.add)
		}
	}
}

// ---- §3 第 4 項：混合 --------------------------------------------------------

// 仿 JEMM 5D9B–5DA0 與 MENU 常式：16 位元 push 組成返回幀，66 36 FF /5 跳到 D=0 段，
// 該段以 16 位元 retf 返回。
func Test437MixedJmp32Retf16(t *testing.T) {
	m := seg437(t, []byte{
		0x6A, sel437Back, // push 30h
		0x68, 0x0B, 0x01, // push 010Bh（下一道 jmp 之後）
		0x66, 0x36, 0xFF, 0x2E, 0x00, 0x02, // jmp m16:32 ss:[0200]
	})
	copy(m.Mem[0x1000+0x200:], farPtr32(0x9E, 0x18))
	m.Mem[segCode+0x9E] = 0xCB // retf
	sp := m.CPU.R[cpu.SP]
	mustStep(t, m, "push 30h")
	mustStep(t, m, "push 010Bh")
	mustStep(t, m, "66 36 FF /5")
	wantCS(t, m, "66 36 FF /5", 0x18, 0x9E, segCode)
	mustStep(t, m, "retf")
	wantCS(t, m, "retf", sel437Back, 0x10B, 0x1000)
	if m.CPU.R[cpu.SP] != sp {
		t.Errorf("retf 之後 SP=%04X，應回到 %04X", m.CPU.R[cpu.SP], sp)
	}
}

// ---- §3 第 3 項：失敗例 ------------------------------------------------------

type snap437 struct {
	cs, ip, sp uint16
	cache      cpu.SegCache
	stack      []byte
}

func take437(m *Machine) snap437 {
	c := m.CPU
	b := c.SegCache(cpu.SS).Base
	return snap437{cs: c.Seg[cpu.CS], ip: c.IP, sp: c.R[cpu.SP], cache: c.SegCache(cpu.CS),
		stack: bytes.Clone(m.Mem[b : b+0x10000])}
}

// wantFail437 執行一步，要求失敗、錯誤含 want、回報本道起點，且 CS、CS 快取、SP、堆疊位元組不變。
func wantFail437(t *testing.T, m *Machine, what, want string) {
	t.Helper()
	before := take437(m)
	err := wantStepErr(t, m, what, want)
	if err != nil {
		if p := fmt.Sprintf("cpu: %04X:%04X op", before.cs, before.ip); !strings.Contains(err.Error(), p) {
			t.Errorf("%s：錯誤應回報本道起點 %s，實際 %v", what, p, err)
		}
	}
	after := take437(m)
	if after.cs != before.cs || after.cache != before.cache || after.sp != before.sp {
		t.Errorf("%s：失敗時 CS、CS 快取、SP 不得改變：CS %04X→%04X 快取 %+v→%+v SP %04X→%04X",
			what, before.cs, after.cs, before.cache, after.cache, before.sp, after.sp)
	}
	if !bytes.Equal(after.stack, before.stack) {
		t.Errorf("%s：失敗時堆疊位元組不得改變", what)
	}
}

func Test437Failures(t *testing.T) {
	jmp := func(off uint32, sel uint16) []byte { return append([]byte{0x66, 0xEA}, farPtr32(off, sel)...) }
	call := func(off uint32, sel uint16) []byte { return append([]byte{0x66, 0x9A}, farPtr32(off, sel)...) }
	retFrame := func(ip, cs uint32) func(*Machine) {
		return func(m *Machine) {
			m.CPU.R[cpu.SP] = 0xFF00
			put32(m, 0x1000+0xFF00, ip)
			put32(m, 0x1000+0xFF04, cs)
		}
	}
	ssLimit := func(limit uint32, sp uint16) func(*Machine) {
		return func(m *Machine) {
			m.CPU.SetSegCache(cpu.SS, dataCache(0x1000, limit, 0x93))
			m.CPU.R[cpu.SP] = sp
		}
	}
	ssB1 := func(m *Machine) { m.CPU.SetSegCache(cpu.SS, dataCache(0x1000, 0xFFFF, 0x4093)) }
	// DS 為向下展開、界限 201h：偏移 1FEh–201h 讀不到、202h–203h 讀得到。
	ptrFault := func(m *Machine) {
		m.CPU.SetSegCache(cpu.DS, dataCache(0x1000, 0x201, 0x97))
		copy(m.Mem[0x1000+0x1FE:], farPtr32(0x1234, 0x18))
	}
	for _, c := range []struct {
		name  string
		code  []byte
		setup func(*Machine)
		want  string
	}{
		{"jmp 偏移高半非 0", jmp(0x1009E, 0x18), nil, "高 16 位元"},
		{"call 偏移高半非 0", call(0x1009E, 0x18), nil, "高 16 位元"},
		{"ret 偏移高半非 0", []byte{0x66, 0xCB}, retFrame(0x10000, 0x18), "高 16 位元"},
		{"jmp D=1 碼段", jmp(0, sel437D1), nil, "D=1"},
		{"call D=1 碼段", call(0, sel437D1), nil, "D=1"},
		{"ret D=1 碼段", []byte{0x66, 0xCB}, retFrame(0, sel437D1), "D=1"},
		{"jmp 資料段", jmp(0, 0x08), nil, "碼段"},
		{"call 資料段", call(0, 0x08), nil, "碼段"},
		{"jmp RPL＞CPL", jmp(0, 0x1B), nil, "RPL"},
		{"call RPL＞CPL", call(0, 0x1B), nil, "RPL"},
		{"ret RPL≠CPL", []byte{0x66, 0xCB}, retFrame(0, 0x1B), "RPL 3 ≠ CPL 0"},
		{"call SS B=1", call(0, 0x18), ssB1, "B=1"},
		{"ret SS B=1", []byte{0x66, 0xCB}, func(m *Machine) { retFrame(0, 0x18)(m); ssB1(m) }, "B=1"},
		{"66 FF /3 mod=3", []byte{0x66, 0xFF, 0xD8}, nil, "mod=3"},
		{"66 FF /5 mod=3", []byte{0x66, 0xFF, 0xE8}, nil, "mod=3"},
		{"call 推入超出 SS 界限（第一個 dword）", call(0, 0x18), ssLimit(0xFF, 0x104), "超出界限"},
		{"call 推入超出 SS 界限（第二個 dword，16 位元環繞）", call(0, 0x18), ssLimit(0xFF, 0x0004), "超出界限"},
		{"66 FF /5 讀指標失敗", []byte{0x66, 0xFF, 0x2E, 0xFE, 0x01}, ptrFault, "向下展開"},
		{"66 FF /3 讀指標失敗", []byte{0x66, 0xFF, 0x1E, 0xFE, 0x01}, ptrFault, "向下展開"},
	} {
		m := seg437(t, c.code)
		if c.setup != nil {
			c.setup(m)
		}
		wantFail437(t, m, c.name, c.want)
	}
}

// 範圍外的 66 形式維持失敗即關閉（437 §2）。
func Test437OutOfScopeFail(t *testing.T) {
	for _, c := range []struct {
		name string
		code []byte
	}{
		{"66 FF /2", []byte{0x66, 0xFF, 0x16, 0x00, 0x02}},
		{"66 FF /4", []byte{0x66, 0xFF, 0x26, 0x00, 0x02}},
		{"66 E8", []byte{0x66, 0xE8, 0, 0, 0, 0}},
		{"66 E9", []byte{0x66, 0xE9, 0, 0, 0, 0}},
		{"66 C2", []byte{0x66, 0xC2, 0, 0}},
		{"66 C3", []byte{0x66, 0xC3}},
	} {
		m := seg437(t, c.code)
		wantFail437(t, m, c.name, "")
	}
}

// 實模式的 66 遠轉移維持現狀：失敗即關閉，CS:IP 以外的狀態不變（437 §2）。
func Test437RealModeUnchanged(t *testing.T) {
	for _, c := range []struct {
		name, want string
		code       []byte
	}{
		{"66 EA", "80386 子集沒有這一道", append([]byte{0x66, 0xEA}, farPtr32(0, 0x18)...)},
		{"66 9A", "80386 子集沒有這一道", append([]byte{0x66, 0x9A}, farPtr32(0, 0x18)...)},
		{"66 CB", "80386 子集沒有這一道", []byte{0x66, 0xCB}},
		{"66 CA", "80386 子集沒有這一道", []byte{0x66, 0xCA, 0, 0}},
		{"66 FF /3", "只在分段模式", []byte{0x66, 0xFF, 0x1E, 0x00, 0x02}},
		{"66 FF /5", "只在分段模式", []byte{0x66, 0xFF, 0x2E, 0x00, 0x02}},
	} {
		m := newSysProg(t, c.code)
		cs, sp := m.CPU.Seg[cpu.CS], m.CPU.R[cpu.SP]
		wantStepErr(t, m, c.name, c.want)
		if m.CPU.Seg[cpu.CS] != cs || m.CPU.R[cpu.SP] != sp {
			t.Errorf("%s：實模式下 CS、SP 不得改變", c.name)
		}
	}
}
