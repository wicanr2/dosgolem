package machine

import (
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// docs/spec/434-legacy-core-segmented-mode §9 第 5 項（位址）與第 6 項（段載入）。
//
// 合成環境：COM 載在 PSP 0100h（CS、DS、ES、SS 的實模式推得基底 1000h，程式在線性 1100h），
// 經 SetCR0 設 PE 再 enterSegmented 進分段模式；GDT 在線性 90000h。

const (
	segGDT    = 0x90000
	segGDTLim = 0x7F
	segData   = 0x60000 // 選擇子 08h 的資料段基底
	segCode   = 0x70000 // 選擇子 18h 的碼段基底
)

// putDesc 在 table 的 sel 處寫 8 bytes 描述元。flags 是 byte 6 的高 4 位元（G、D/B、0、AVL）。
func putDesc(m *Machine, table uint32, sel uint16, base, limit uint32, access, flags uint8) {
	a := table + uint32(sel&^7)
	d := [8]byte{byte(limit), byte(limit >> 8), byte(base), byte(base >> 8), byte(base >> 16),
		access, flags<<4 | byte(limit>>16)&0xF, byte(base >> 24)}
	copy(m.Mem[a:], d[:])
}

// segProg 載入 code、進分段模式、裝好標準 GDT：
//
//	08h 資料 可寫  基底 60000h 界限 FFFFFh
//	10h 資料 可寫  基底 1000h  界限 FFFFh（與實模式推得的 DS、SS 相同）
//	18h 碼段 可讀  基底 70000h 界限 FFFFh
//	20h 資料 唯讀  基底 60000h 界限 FFFFh
func segProg(t *testing.T, code []byte) *Machine {
	t.Helper()
	m := newSysProg(t, code)
	mustNil(t, "SetCR0", m.SetCR0(cr0PE))
	mustNil(t, "enterSegmented", m.enterSegmented())
	putDesc(m, segGDT, 0x08, segData, 0xFFFFF, 0x92, 0)
	putDesc(m, segGDT, 0x10, 0x1000, 0xFFFF, 0x92, 0)
	putDesc(m, segGDT, 0x18, segCode, 0xFFFF, 0x9A, 0)
	putDesc(m, segGDT, 0x20, segData, 0xFFFF, 0x90, 0)
	m.SetGDTR(segGDT, segGDTLim)
	return m
}

func dataCache(base, limit uint32, attr uint16) SegCache {
	return SegCache{Base: base, Limit: limit, Attr: attr}
}

func le16(m *Machine, a uint32) uint16 { return uint16(m.Mem[a]) | uint16(m.Mem[a+1])<<8 }

// ---- §9 第 5 項：位址 ----------------------------------------------------

// 仿 JEMM img 42AD–42BE：ES 以舊表載入；lgdt 換成 ES 段內的新表；經 ES 改寫新表的項 28h；
// 再以同一選擇子載入 DS，DS 取新表的新基底。證明存取以段索引取快取（ES 的快取基底），
// 不以選擇子查表（以選擇子查表會寫進舊表）。
func TestSegLGDTReplica(t *testing.T) {
	const newGDT = segData + 0x1400
	m := segProg(t, []byte{
		0xB8, 0x28, 0x00, // mov ax,28h
		0x8E, 0xC0, // mov es,ax
		0x0F, 0x01, 0x16, 0x00, 0x02, // lgdt [0200]
		0x26, 0xC7, 0x06, 0x2A, 0x14, 0x78, 0x56, // mov word es:[142A],5678h
		0x26, 0xC6, 0x06, 0x2C, 0x14, 0x07, // mov byte es:[142C],07h
		0x8E, 0xD8, // mov ds,ax
	})
	putDesc(m, segGDT, 0x28, segData, 0xFFFF, 0x92, 0)
	copy(m.Mem[newGDT:newGDT+0x80], m.Mem[segGDT:segGDT+0x80]) // 「複製到 XMS」
	// 副本的項 28h 仍指向「舊基底」50000h（JEMM 的副本項 20h 仍是 20000h）：以選擇子查新表會寫到 51400h 一帶。
	putDesc(m, newGDT, 0x28, 0x50000, 0xFFFF, 0x92, 0)
	copy(m.Mem[0x1200:], []byte{0x7F, 0x00, 0x00, 0x14, 0x06, 0x00})
	for i := 0; i < 6; i++ {
		mustStep(t, m, "42AD 仿製")
	}
	if b, l := m.GDTR(); b != newGDT || l != 0x7F {
		t.Fatalf("lgdt：GDTR=%08X/%04X", b, l)
	}
	if got := m.Mem[newGDT+0x2A : newGDT+0x2D]; got[0] != 0x78 || got[1] != 0x56 || got[2] != 0x07 {
		t.Errorf("經 ES 的寫入應落在 ES 快取基底＋偏移（新表）：% X", got)
	}
	if got := m.Mem[segGDT+0x2A : segGDT+0x2D]; got[0] != 0 || got[1] != 0 || got[2] != 0x06 {
		t.Errorf("舊表不得被寫：% X", got)
	}
	if m.Mem[0x5142A] != 0 || m.Mem[0x5142B] != 0 || m.Mem[0x5142C] != 0 {
		t.Errorf("以選擇子查新表的位置不得被寫：% X", m.Mem[0x5142A:0x5142D])
	}
	if c := m.CPU.SegCache(cpu.ES); c.Base != segData {
		t.Errorf("ES 快取不因換表而改變：%+v", c)
	}
	if c := m.CPU.SegCache(cpu.DS); m.CPU.Seg[cpu.DS] != 0x28 || c.Base != 0x075678 {
		t.Errorf("DS 應自新表載入新基底 75678h：DS=%04X %+v", m.CPU.Seg[cpu.DS], c)
	}
}

// 67 定址的有效位址 > FFFFh：分段模式可用，實模式仍記 fault（ModRM32 與 moffs32 各一例）。
func TestSeg67AddressBeyond64K(t *testing.T) {
	code := []byte{
		0x67, 0x8B, 0x03, // mov ax,[ebx]
		0x67, 0xA1, 0x46, 0x23, 0x01, 0x00, // mov ax,[00012346]
	}
	m := segProg(t, code)
	m.CPU.SetSegCache(cpu.DS, dataCache(segData, 0xFFFFF, 0x93))
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x2344, 0x0001
	copy(m.Mem[segData+0x12344:], []byte{0x34, 0x12, 0x78, 0x56})
	mustStep(t, m, "67 mov ax,[ebx]（分段）")
	if m.CPU.R[cpu.AX] != 0x1234 {
		t.Errorf("ModRM32：AX=%04X，應為 1234", m.CPU.R[cpu.AX])
	}
	mustStep(t, m, "67 mov ax,[moffs32]（分段）")
	if m.CPU.R[cpu.AX] != 0x5678 {
		t.Errorf("moffs32：AX=%04X，應為 5678", m.CPU.R[cpu.AX])
	}
	// 實模式：同樣的碼照舊失敗。
	r := newSysProg(t, code[:3])
	r.CPU.R[cpu.BX], r.CPU.Hi[cpu.BX] = 0x2344, 0x0001
	wantStepErr(t, r, "實模式 67 ModRM32 > FFFFh", "64 KB")
	r = newSysProg(t, code[3:])
	wantStepErr(t, r, "實模式 67 moffs32 > FFFFh", "64 KB")
}

// 67 定址的 dword 寫入跨過 FFFFh 以 32 位元相加；16 位元定址照舊各 byte 環繞。
func TestSeg67DwordCrossesFFFF(t *testing.T) {
	m := segProg(t, []byte{
		0x66, 0x67, 0x89, 0x03, // mov [ebx],eax
		0x66, 0x89, 0x07, // mov [bx],eax
	})
	m.CPU.SetSegCache(cpu.DS, dataCache(segData, 0xFFFFF, 0x93))
	m.CPU.R[cpu.AX], m.CPU.Hi[cpu.AX] = 0x2211, 0x4433
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0xFFFE, 0
	mustStep(t, m, "66 67 mov [ebx],eax")
	if got := m.Mem[segData+0xFFFE : segData+0x10002]; got[0] != 0x11 || got[1] != 0x22 || got[2] != 0x33 || got[3] != 0x44 {
		t.Errorf("67 定址應以 32 位元相加：% X", got)
	}
	if m.Mem[segData] != 0 {
		t.Errorf("67 定址不得環繞到偏移 0：%02X", m.Mem[segData])
	}
	m.CPU.R[cpu.AX], m.CPU.Hi[cpu.AX] = 0xBBAA, 0xDDCC
	mustStep(t, m, "66 mov [bx],eax")
	if m.Mem[segData+0xFFFE] != 0xAA || m.Mem[segData+0xFFFF] != 0xBB || m.Mem[segData] != 0xCC || m.Mem[segData+1] != 0xDD {
		t.Error("16 位元定址的各 byte 偏移各自 16 位元環繞")
	}
}

// segOne 在分段模式以指定的 DS 快取跑一道指令，回 Step 的錯誤。
func segOne(t *testing.T, ds SegCache, code []byte) (*Machine, error) {
	t.Helper()
	m := segProg(t, code)
	m.CPU.SetSegCache(cpu.DS, ds)
	return m, m.Step()
}

func moffs(op byte, off uint16) []byte { return []byte{op, byte(off), byte(off >> 8)} }

// 向上與向下展開界限邊界各一例（剛好在內、剛好超出）。
func TestSegLimits(t *testing.T) {
	up := dataCache(segData, 0x0FFF, 0x93)
	for _, c := range []struct {
		name string
		ds   SegCache
		code []byte
		ok   bool
	}{
		{"向上 byte 讀 0FFF", up, moffs(0xA0, 0x0FFF), true},
		{"向上 byte 讀 1000", up, moffs(0xA0, 0x1000), false},
		{"向上 word 讀 0FFE", up, moffs(0xA1, 0x0FFE), true},
		{"向上 word 讀 0FFF（第二個 byte 超出）", up, moffs(0xA1, 0x0FFF), false},
		{"向下 B=0 byte 讀 1000", dataCache(segData, 0x0FFF, 0x97), moffs(0xA0, 0x1000), true},
		{"向下 B=0 byte 讀 0FFF", dataCache(segData, 0x0FFF, 0x97), moffs(0xA0, 0x0FFF), false},
		{"向下 B=0 byte 讀 FFFF", dataCache(segData, 0x0FFF, 0x97), moffs(0xA0, 0xFFFF), true},
		{"向下 B=0 67 讀 10000", dataCache(segData, 0x0FFF, 0x97), []byte{0x67, 0xA0, 0, 0, 1, 0}, false},
		{"向下 B=1 67 讀 10000", dataCache(segData, 0x0FFF, 0x4097), []byte{0x67, 0xA0, 0, 0, 1, 0}, true},
	} {
		_, err := segOne(t, c.ds, c.code)
		if c.ok && err != nil {
			t.Errorf("%s：應成功，實際 %v", c.name, err)
		}
		if !c.ok && (err == nil || !strings.Contains(err.Error(), "界限")) {
			t.Errorf("%s：應失敗即關閉（界限），實際 %v", c.name, err)
		}
	}
	// 失敗的 word 寫入一個 byte 都不寫。
	m, err := segOne(t, up, moffs(0xA3, 0x0FFF))
	if err == nil || m.Mem[segData+0x0FFF] != 0 {
		t.Errorf("跨界 word 寫入應失敗且不寫任何 byte：err=%v [0FFF]=%02X", err, m.Mem[segData+0x0FFF])
	}
	// 診斷寫明段暫存器、選擇子、偏移、界限。
	if err != nil {
		for _, want := range []string{"DS=", "00001000", "00000FFF"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("診斷應含 %q：%v", want, err)
			}
		}
	}
}

// 寫入唯讀段、寫入碼段、讀不可讀碼段、經不可用快取存取失敗即關閉。
func TestSegTypeChecks(t *testing.T) {
	for _, c := range []struct {
		name string
		ds   SegCache
		code []byte
		ok   bool
		why  string
	}{
		{"唯讀資料段可讀", dataCache(segData, 0xFFFF, 0x91), moffs(0xA0, 0), true, ""},
		{"寫入唯讀資料段", dataCache(segData, 0xFFFF, 0x91), moffs(0xA2, 0), false, "唯讀"},
		{"可讀碼段可讀", dataCache(segData, 0xFFFF, 0x9B), moffs(0xA0, 0), true, ""},
		{"寫入碼段", dataCache(segData, 0xFFFF, 0x9B), moffs(0xA2, 0), false, "寫入碼段"},
		{"讀不可讀碼段", dataCache(segData, 0xFFFF, 0x99), moffs(0xA0, 0), false, "不可讀"},
		{"經不可用快取讀", SegCache{}, moffs(0xA0, 0), false, "不可用"},
		{"經不可用快取寫", SegCache{}, moffs(0xA2, 0), false, "不可用"},
	} {
		m, err := segOne(t, c.ds, c.code)
		if c.ok && err != nil {
			t.Errorf("%s：應成功，實際 %v", c.name, err)
		}
		if !c.ok && (err == nil || !strings.Contains(err.Error(), c.why)) {
			t.Errorf("%s：應失敗即關閉（%s），實際 %v", c.name, c.why, err)
		}
		if !c.ok && m.Mem[segData] != 0 {
			t.Errorf("%s：失敗的寫入不得落地", c.name)
		}
	}
	// 經載入空選擇子而不可用的 DS。
	m := segProg(t, []byte{0x31, 0xC0, 0x8E, 0xD8, 0xA0, 0x00, 0x00}) // xor ax,ax；mov ds,ax；mov al,[0]
	mustStep(t, m, "xor ax,ax")
	mustStep(t, m, "mov ds,0（允許）")
	wantStepErr(t, m, "經空選擇子的 DS 讀取", "不可用")
}

// lea 超出界限不失敗（不存取）。
func TestSegLEANoLimitCheck(t *testing.T) {
	m := segProg(t, []byte{
		0x8D, 0x87, 0x00, 0x20, // lea ax,[bx+2000h]
		0x66, 0x67, 0x8D, 0x83, 0x00, 0x00, 0x02, 0x00, // lea eax,[ebx+20000h]
	})
	m.CPU.SetSegCache(cpu.DS, dataCache(segData, 0x0FFF, 0x93))
	m.CPU.R[cpu.BX], m.CPU.Hi[cpu.BX] = 0x0010, 0
	mustStep(t, m, "lea ax（超出界限）")
	if m.CPU.R[cpu.AX] != 0x2010 {
		t.Errorf("lea ax：%04X", m.CPU.R[cpu.AX])
	}
	mustStep(t, m, "lea eax（67，超出界限）")
	if m.CPU.R[cpu.AX] != 0x0010 || m.CPU.Hi[cpu.AX] != 0x0002 {
		t.Errorf("lea eax：%04X%04X", m.CPU.Hi[cpu.AX], m.CPU.R[cpu.AX])
	}
}

// CS 基底非零時取指令正確；IP 超出 CS 界限失敗即關閉。
func TestSegFetchUsesCSBase(t *testing.T) {
	m := segProg(t, []byte{0xEA, 0x00, 0x00, 0x18, 0x00})                   // jmp 18h:0000
	copy(m.Mem[segCode:], []byte{0xB0, 0x5A, 0xEA, 0x00, 0x00, 0x30, 0x00}) // mov al,5Ah；jmp 30h:0
	putDesc(m, segGDT, 0x30, segCode+0x100, 0x0001, 0x9A, 0)
	copy(m.Mem[segCode+0x100:], []byte{0xB0, 0xA5, 0x90}) // mov al,A5h（偏移 0、1）；nop 在偏移 2（超出界限 1）
	mustStep(t, m, "jmp 18h:0")
	if m.CPU.Seg[cpu.CS] != 0x18 || m.CPU.SegCache(cpu.CS).Base != segCode {
		t.Fatalf("CS=%04X 快取 %+v", m.CPU.Seg[cpu.CS], m.CPU.SegCache(cpu.CS))
	}
	mustStep(t, m, "mov al,5Ah（自 CS 快取基底取指令）")
	if uint8(m.CPU.R[cpu.AX]) != 0x5A {
		t.Errorf("AL=%02X，應為 5A", uint8(m.CPU.R[cpu.AX]))
	}
	mustStep(t, m, "jmp 30h:0")
	mustStep(t, m, "mov al,A5h（界限內）")
	if uint8(m.CPU.R[cpu.AX]) != 0xA5 {
		t.Errorf("AL=%02X，應為 A5", uint8(m.CPU.R[cpu.AX]))
	}
	wantStepErr(t, m, "IP 超出 CS 界限", "取指令超出界限")
}

// FS／GS 前綴走各自的快取。
func TestSegFSGSPrefixes(t *testing.T) {
	m := segProg(t, []byte{
		0x64, 0xA0, 0x10, 0x00, // mov al,fs:[10]
		0x88, 0xC3, // mov bl,al
		0x65, 0xA0, 0x10, 0x00, // mov al,gs:[10]
		0xA0, 0x10, 0x00, // mov al,[10]（DS）
	})
	m.CPU.SetSegCache(cpu.FS, dataCache(segData, 0xFFFF, 0x93))
	m.CPU.SetSegCache(cpu.GS, dataCache(segCode, 0xFFFF, 0x93))
	m.CPU.SetSegCache(cpu.DS, dataCache(0x50000, 0xFFFF, 0x93))
	m.Mem[segData+0x10], m.Mem[segCode+0x10], m.Mem[0x50010] = 0x11, 0x22, 0x33
	for i := 0; i < 3; i++ {
		mustStep(t, m, "FS／GS 前綴")
	}
	if uint8(m.CPU.R[cpu.BX]) != 0x11 || uint8(m.CPU.R[cpu.AX]) != 0x22 {
		t.Errorf("fs:[10]=%02X gs:[10]=%02X，應為 11、22", uint8(m.CPU.R[cpu.BX]), uint8(m.CPU.R[cpu.AX]))
	}
	mustStep(t, m, "mov al,[10]")
	if uint8(m.CPU.R[cpu.AX]) != 0x33 {
		t.Errorf("DS:[10]=%02X，應為 33", uint8(m.CPU.R[cpu.AX]))
	}
}

// 堆疊經 SS 快取；SS 的 B=1 失敗即關閉。
func TestSegStack(t *testing.T) {
	m := segProg(t, []byte{0x50, 0x50}) // push ax；push ax
	m.CPU.SetSegCache(cpu.SS, dataCache(segData, 0xFFFF, 0x93))
	m.CPU.R[cpu.SP], m.CPU.R[cpu.AX] = 0x0100, 0xBEEF
	mustStep(t, m, "push ax")
	if le16(m, segData+0xFE) != 0xBEEF {
		t.Errorf("push 應寫 SS 快取基底＋SP：%04X", le16(m, segData+0xFE))
	}
	m.CPU.SetSegCache(cpu.SS, dataCache(segData, 0xFFFF, 0x4093))
	wantStepErr(t, m, "SS 的 B=1", "B=1")
}

// ---- §9 第 6 項：段載入 ---------------------------------------------------

// loadCase 以選擇子 40h 的描述元（access、flags）測一種段暫存器的載入。
type loadCase struct {
	name   string
	reg    int
	sel    uint16
	access uint8
	flags  uint8
	ok     bool
}

// loadCode 回載入 sel 到 reg 的程式：mov sreg,ax 或 jmp far（CS）。
func loadCode(reg int, sel uint16) []byte {
	switch reg {
	case cpu.CS:
		return []byte{0xEA, 0x00, 0x00, byte(sel), byte(sel >> 8)}
	case cpu.SS:
		return []byte{0xB8, byte(sel), byte(sel >> 8), 0x8E, 0xD0}
	case cpu.ES:
		return []byte{0xB8, byte(sel), byte(sel >> 8), 0x8E, 0xC0}
	case cpu.FS:
		return []byte{0xB8, byte(sel), byte(sel >> 8), 0x8E, 0xE0}
	case cpu.GS:
		return []byte{0xB8, byte(sel), byte(sel >> 8), 0x8E, 0xE8}
	}
	return []byte{0xB8, byte(sel), byte(sel >> 8), 0x8E, 0xD8}
}

func segValOf(c *cpu.CPU, i int) uint16 {
	switch i {
	case cpu.FS:
		return c.FSeg
	case cpu.GS:
		return c.GSeg
	}
	return c.Seg[i]
}

// runLoad 跑 loadCode；回 Step 的錯誤（CS 一步，其餘兩步）。
func runLoad(t *testing.T, m *Machine, reg int) error {
	t.Helper()
	if reg != cpu.CS {
		mustStep(t, m, "mov ax,sel")
	}
	return m.Step()
}

func TestSegLoadChecks(t *testing.T) {
	cases := []loadCase{
		// SS：RPL＝CPL＝DPL；可寫資料段；present。
		{"SS 可寫資料段", cpu.SS, 0x40, 0x92, 0, true},
		{"SS 唯讀資料段", cpu.SS, 0x40, 0x90, 0, false},
		{"SS 碼段", cpu.SS, 0x40, 0x9A, 0, false},
		{"SS RPL≠CPL", cpu.SS, 0x43, 0x92, 0, false},
		{"SS DPL≠CPL", cpu.SS, 0x40, 0xB2, 0, false},
		{"SS 不存在", cpu.SS, 0x40, 0x12, 0, false},
		{"SS 向下展開可寫（B=0）", cpu.SS, 0x40, 0x96, 0, true},
		// DS／ES／FS／GS。
		{"DS 資料段", cpu.DS, 0x40, 0x92, 0, true},
		{"DS 唯讀資料段", cpu.DS, 0x40, 0x90, 0, true},
		{"DS 非一致可讀碼段", cpu.DS, 0x40, 0x9A, 0, true},
		{"DS 不可讀碼段", cpu.DS, 0x40, 0x98, 0, false},
		{"DS 系統描述元（LDT）", cpu.DS, 0x40, 0x82, 0, false},
		{"DS RPL>DPL（資料段）", cpu.DS, 0x43, 0x92, 0, false},
		{"DS RPL>DPL（非一致可讀碼段）", cpu.DS, 0x43, 0x9A, 0, false},
		{"DS 一致可讀碼段免檢（RPL 3、DPL 0）", cpu.DS, 0x43, 0x9E, 0, true},
		{"DS RPL≤DPL（DPL 3）", cpu.DS, 0x43, 0xF2, 0, true},
		{"DS 不存在", cpu.DS, 0x40, 0x12, 0, false},
		{"ES 資料段", cpu.ES, 0x40, 0x92, 0, true},
		{"ES 不存在", cpu.ES, 0x40, 0x12, 0, false},
		{"FS 資料段", cpu.FS, 0x40, 0x92, 0, true},
		{"FS 系統描述元", cpu.FS, 0x40, 0x89, 0, false},
		{"GS 資料段", cpu.GS, 0x40, 0x92, 0, true},
		{"GS 不可讀碼段", cpu.GS, 0x40, 0x98, 0, false},
		// CS（遠跳）：非一致碼段，RPL ≤ CPL、DPL＝CPL；present；D=0。
		{"CS 非一致碼段", cpu.CS, 0x40, 0x9A, 0, true},
		{"CS 只可執行碼段", cpu.CS, 0x40, 0x98, 0, true},
		{"CS 一致碼段", cpu.CS, 0x40, 0x9E, 0, false},
		{"CS TSS", cpu.CS, 0x40, 0x89, 0, false},
		{"CS 資料段", cpu.CS, 0x40, 0x92, 0, false},
		{"CS DPL≠CPL", cpu.CS, 0x40, 0xBA, 0, false},
		{"CS RPL>CPL", cpu.CS, 0x43, 0x9A, 0, false},
		{"CS D=1", cpu.CS, 0x40, 0x9A, 0x4, false},
		{"CS 不存在", cpu.CS, 0x40, 0x1A, 0, false},
	}
	for _, c := range cases {
		m := segProg(t, loadCode(c.reg, c.sel))
		putDesc(m, segGDT, 0x40, segData, 0x0FFF, c.access, c.flags)
		beforeVal, beforeCache := segValOf(m.CPU, c.reg), m.CPU.SegCache(c.reg)
		err := runLoad(t, m, c.reg)
		if c.ok {
			if err != nil {
				t.Errorf("%s：應成功，實際 %v", c.name, err)
				continue
			}
			want := SegCache{Base: segData, Limit: 0x0FFF, Attr: uint16(c.access)}
			if c.flags&0x4 != 0 {
				want.Attr |= 0x4000
			}
			if got := m.CPU.SegCache(c.reg); got != want || segValOf(m.CPU, c.reg) != c.sel {
				t.Errorf("%s：段值 %04X 快取 %+v，應為 %04X %+v", c.name, segValOf(m.CPU, c.reg), got, c.sel, want)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), "分段模式載入") {
			t.Errorf("%s：應失敗即關閉，實際 %v", c.name, err)
		}
		if segValOf(m.CPU, c.reg) != beforeVal || m.CPU.SegCache(c.reg) != beforeCache {
			t.Errorf("%s：失敗時段值與快取不得改變", c.name)
		}
	}
}

// 描述元的 G 與 AVL：界限依 G 展開；Attr 的 bit 13 恆 0（386 保留的 bit 21 不帶進來）。
func TestSegDescriptorDecode(t *testing.T) {
	m := segProg(t, loadCode(cpu.DS, 0x40))
	putDesc(m, segGDT, 0x40, 0x12345678, 0x000AB, 0x92, 0x8|0x2|0x1) // G、bit 21、AVL
	mustNil(t, "載入", runLoad(t, m, cpu.DS))
	want := SegCache{Base: 0x12345678, Limit: 0x000ABFFF, Attr: 0x9092}
	if got := m.CPU.SegCache(cpu.DS); got != want {
		t.Errorf("快取 %+v，應為 %+v", got, want)
	}
}

// 空選擇子、GDT 與自建 LDT、表界限邊界、accessed 位元不寫回。
func TestSegLoadSelectorsAndTables(t *testing.T) {
	// 空選擇子：DS 允許（RPL 忽略），快取不可用；CS、SS 失敗。
	for _, sel := range []uint16{0x0000, 0x0003} {
		m := segProg(t, loadCode(cpu.DS, sel))
		mustNil(t, "DS 空選擇子", runLoad(t, m, cpu.DS))
		if m.CPU.Seg[cpu.DS] != sel || m.CPU.SegCache(cpu.DS) != (SegCache{}) {
			t.Errorf("DS=%04X：段值應照寫、快取不可用：%+v", sel, m.CPU.SegCache(cpu.DS))
		}
	}
	for _, reg := range []int{cpu.SS, cpu.CS} {
		m := segProg(t, loadCode(reg, 0))
		before := m.CPU.SegCache(reg)
		if err := runLoad(t, m, reg); err == nil || !strings.Contains(err.Error(), "空選擇子") {
			t.Errorf("%s 空選擇子應失敗：%v", segNameT[reg], err)
		}
		if m.CPU.SegCache(reg) != before {
			t.Errorf("%s 空選擇子失敗後快取不得改變", segNameT[reg])
		}
	}
	// 04h 不算空：TI=1 查 LDT，LDTR 不可用時失敗。
	m := segProg(t, loadCode(cpu.DS, 0x04))
	if err := runLoad(t, m, cpu.DS); err == nil || !strings.Contains(err.Error(), "LDTR") {
		t.Errorf("04h 在 LDTR 不可用時應失敗：%v", err)
	}
	// 自建 LDT（基底 91000h、界限 7）：04h 載入成功，0Ch 超出表界限。
	const ldt = 0x91000
	for _, c := range []struct {
		sel uint16
		ok  bool
	}{{0x04, true}, {0x0C, false}} {
		m := segProg(t, loadCode(cpu.DS, c.sel))
		putDesc(m, ldt, 0x00, 0x00ABC000, 0xFFFF, 0x92, 0)
		m.SetLDTR(0x48, SegCache{Base: ldt, Limit: 7, Attr: 0x82})
		err := runLoad(t, m, cpu.DS)
		switch {
		case c.ok && err != nil:
			t.Errorf("LDT 選擇子 %04X：應成功，實際 %v", c.sel, err)
		case c.ok && m.CPU.SegCache(cpu.DS).Base != 0x00ABC000:
			t.Errorf("LDT 選擇子 %04X：基底 %08X", c.sel, m.CPU.SegCache(cpu.DS).Base)
		case !c.ok && (err == nil || !strings.Contains(err.Error(), "表界限")):
			t.Errorf("LDT 選擇子 %04X：應超出表界限，實際 %v", c.sel, err)
		}
	}
	// GDT 表界限邊界：界限 2Fh 時 28h 在內、30h 超出。
	for _, c := range []struct {
		sel uint16
		ok  bool
	}{{0x28, true}, {0x30, false}} {
		m := segProg(t, loadCode(cpu.DS, c.sel))
		putDesc(m, segGDT, 0x28, segData, 0xFFFF, 0x92, 0)
		putDesc(m, segGDT, 0x30, segData, 0xFFFF, 0x92, 0)
		m.SetGDTR(segGDT, 0x2F)
		err := runLoad(t, m, cpu.DS)
		if c.ok != (err == nil) {
			t.Errorf("GDT 界限 2Fh、選擇子 %04X：ok=%v err=%v", c.sel, c.ok, err)
		}
	}
	// accessed 位元不寫回。
	m = segProg(t, loadCode(cpu.DS, 0x08))
	mustNil(t, "載入 08h", runLoad(t, m, cpu.DS))
	if m.Mem[segGDT+0x08+5] != 0x92 || m.CPU.SegCache(cpu.DS).Attr != 0x92 {
		t.Errorf("accessed 位元不得寫回：描述元 %02X 快取 %04X", m.Mem[segGDT+0x08+5], m.CPU.SegCache(cpu.DS).Attr)
	}
}

var segNameT = [6]string{"ES", "CS", "SS", "DS", "FS", "GS"}

// §5.1 表中每個位置在分段模式下都經 loadSeg：以超出 GDT 界限的選擇子 0100h 觸發，
// 經 loadSeg 就會失敗即關閉；略過 loadSeg 的話段值會被照寫。
func TestSegLoadSitesUseLoadSeg(t *testing.T) {
	const bad = 0x0100
	push := []byte{0x68, 0x00, 0x01} // push 0100h
	push32 := []byte{0x66, 0x68, 0x00, 0x01, 0x00, 0x00}
	far := func(op ...byte) []byte { return append(op, 0x00, 0x02) } // [0200] 存 0000:0100
	sites := []struct {
		name string
		reg  int
		pre  []byte // 先跑的指令（各一步）
		code []byte
	}{
		{"pop es", cpu.ES, push, []byte{0x07}},
		{"pop ss", cpu.SS, push, []byte{0x17}},
		{"pop ds", cpu.DS, push, []byte{0x1F}},
		{"mov es,ax", cpu.ES, nil, []byte{0x8E, 0xC0}},
		{"mov ss,ax", cpu.SS, nil, []byte{0x8E, 0xD0}},
		{"mov ds,ax", cpu.DS, nil, []byte{0x8E, 0xD8}},
		{"mov fs,ax", cpu.FS, nil, []byte{0x8E, 0xE0}},
		{"mov gs,ax", cpu.GS, nil, []byte{0x8E, 0xE8}},
		{"call far ptr（9A）", cpu.CS, nil, []byte{0x9A, 0x00, 0x00, 0x00, 0x01}},
		{"jmp far ptr（EA）", cpu.CS, nil, []byte{0xEA, 0x00, 0x00, 0x00, 0x01}},
		{"call far [m]（FF /3）", cpu.CS, nil, far(0xFF, 0x1E)},
		{"jmp far [m]（FF /5）", cpu.CS, nil, far(0xFF, 0x2E)},
		{"les", cpu.ES, nil, far(0xC4, 0x1E)},
		{"lds", cpu.DS, nil, far(0xC5, 0x1E)},
		{"retf imm16", cpu.CS, append(append([]byte{}, push...), 0x6A, 0x00), []byte{0xCA, 0x02, 0x00}},
		{"retf", cpu.CS, append(append([]byte{}, push...), 0x6A, 0x00), []byte{0xCB}},
		{"pop fs", cpu.FS, push, []byte{0x0F, 0xA1}},
		{"pop gs", cpu.GS, push, []byte{0x0F, 0xA9}},
		{"lss", cpu.SS, nil, far(0x0F, 0xB2, 0x1E)},
		{"lfs", cpu.FS, nil, far(0x0F, 0xB4, 0x1E)},
		{"lgs", cpu.GS, nil, far(0x0F, 0xB5, 0x1E)},
		{"66 pop es", cpu.ES, push32, []byte{0x66, 0x07}},
		{"66 pop ss", cpu.SS, push32, []byte{0x66, 0x17}},
		{"66 pop ds", cpu.DS, push32, []byte{0x66, 0x1F}},
	}
	for _, s := range sites {
		m := segProg(t, append(append([]byte{}, s.pre...), s.code...))
		m.CPU.R[cpu.AX] = bad
		copy(m.Mem[0x1200:], []byte{0x00, 0x00, 0x00, 0x01}) // DS:0200 的遠指標 0100:0000
		steps := 0
		if len(s.pre) > 0 {
			steps = 1
			if s.name == "retf imm16" || s.name == "retf" {
				steps = 2
			}
		}
		for i := 0; i < steps; i++ {
			mustStep(t, m, s.name+" 的前置")
		}
		before, beforeCache := segValOf(m.CPU, s.reg), m.CPU.SegCache(s.reg)
		err := m.Step()
		if err == nil || !strings.Contains(err.Error(), "分段模式載入") {
			t.Errorf("%s：應經 loadSeg 失敗即關閉，實際 %v", s.name, err)
		}
		if segValOf(m.CPU, s.reg) != before || m.CPU.SegCache(s.reg) != beforeCache {
			t.Errorf("%s：失敗時段值與快取不得改變（%04X→%04X）", s.name, before, segValOf(m.CPU, s.reg))
		}
	}
}

// pop cs 只在 8086（cpu 套件另測）；mov cs、iret 在分段模式失敗即關閉；遠返回的 RPL ≠ CPL 失敗。
func TestSegForbiddenSegmentWrites(t *testing.T) {
	m := segProg(t, []byte{0x8E, 0xC8}) // mov cs,ax
	m.CPU.R[cpu.AX] = 0x18
	cs := m.CPU.Seg[cpu.CS]
	wantStepErr(t, m, "mov cs", "MOV CS")
	if m.CPU.Seg[cpu.CS] != cs {
		t.Error("mov cs 失敗後 CS 不得改變")
	}
	// Machine 的 386 初始旗標是 7002h（帶 NT=1）；這兩項要測的是 NT=0 的 iret，所以先清 NT
	// （NT=1 是任務返回，docs/spec/436 §3.1）。
	m = segProg(t, []byte{0xCF}) // iret
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.NT)
	wantStepErr(t, m, "iret", "IRET")
	m = segProg(t, []byte{0x66, 0xCF}) // iretd
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.NT)
	wantStepErr(t, m, "66 iret", "IRET")
	// 遠返回彈出的 CS 的 RPL ≠ CPL：19h（RPL 1）。
	m = segProg(t, []byte{0x68, 0x19, 0x00, 0x6A, 0x00, 0xCB}) // push 19h；push 0；retf
	mustStep(t, m, "push 19h")
	mustStep(t, m, "push 0")
	wantStepErr(t, m, "retf 到 RPL 1", "RPL")
	if m.CPU.Seg[cpu.CS] != cs {
		t.Error("retf 失敗後 CS 不得改變")
	}
}

// 遠跳、遠呼叫、遠返回成功時的 CS 值與快取（§5.1 的 CS 規則；§6 的推入細節在階段 3 另驗）。
func TestSegFarTransferLoadsCS(t *testing.T) {
	m := segProg(t, []byte{0x9A, 0x00, 0x00, 0x18, 0x00}) // call 18h:0000
	copy(m.Mem[segCode:], []byte{0xCB})                   // retf
	m.CPU.SetSegCache(cpu.SS, dataCache(0x1000, 0xFFFF, 0x93))
	cs, ip := m.CPU.Seg[cpu.CS], m.CPU.IP
	mustStep(t, m, "call far 18h:0")
	if m.CPU.Seg[cpu.CS] != 0x18 || m.CPU.IP != 0 || m.CPU.SegCache(cpu.CS).Base != segCode {
		t.Fatalf("call far：CS:IP=%04X:%04X 快取 %+v", m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.SegCache(cpu.CS))
	}
	// 回到原本的 CS 值（實模式值 0100h 經 GDT 查表：GDT 項 100h 超出界限），先把它改成可用的 10h 項。
	putDesc(m, segGDT, 0x10, 0x1000, 0xFFFF, 0x9A, 0)
	sp := m.CPU.R[cpu.SP]
	m.Mem[0x1000+uint32(sp)+2], m.Mem[0x1000+uint32(sp)+3] = 0x10, 0x00
	mustStep(t, m, "retf 到 10h")
	if m.CPU.Seg[cpu.CS] != 0x10 || m.CPU.IP != ip+5 || m.CPU.SegCache(cpu.CS).Base != 0x1000 {
		t.Errorf("retf：CS:IP=%04X:%04X（原 %04X）快取 %+v", m.CPU.Seg[cpu.CS], m.CPU.IP, cs, m.CPU.SegCache(cpu.CS))
	}
}

// Interrupt、FarCall 直接呼叫時失敗即關閉（第二道防線），不動狀態；錯誤在下一次 Step 回報。
func TestSegInterruptAndFarCallFailClosed(t *testing.T) {
	for _, c := range []struct {
		name string
		call func(*cpu.CPU)
		want string
	}{
		{"Interrupt", func(c *cpu.CPU) { c.Interrupt(0x21) }, "Interrupt"},
		{"FarCall", func(c *cpu.CPU) { c.FarCall(0x1234, 0x5678) }, "FarCall"},
		{"FarCallWithReturn", func(c *cpu.CPU) { c.FarCallWithReturn(0x1234, 0x5678, 1, 2) }, "FarCall"},
	} {
		m := segProg(t, []byte{0x90})
		cs, ip, sp, fl := m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.R[cpu.SP], m.CPU.Flags
		c.call(m.CPU)
		if m.CPU.Seg[cpu.CS] != cs || m.CPU.IP != ip || m.CPU.R[cpu.SP] != sp || m.CPU.Flags != fl {
			t.Errorf("%s：分段模式下不得改變 CS:IP、SP、旗標", c.name)
		}
		wantStepErr(t, m, c.name, c.want)
	}
}
