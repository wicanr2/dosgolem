package dos

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/430-physical-memory-and-xms-unification §5 第 5 項：可掛鉤入口。
// 以 Step 真正執行指令，不直接呼叫 d.handle。

const hookSeg = 0x20CB // 低位元組 CBh：偏移 +3 的「入口 + 3」會落在 RETF 上（見 chain3 案例）

// 合成程式（偏移 0 起；資料在 DS:0080 起）依 SCCD 的邏輯：
//
//	mov ax,4310h / int 2Fh              ; ES:BX ＝ XMS 入口
//	mov [80h],bx / mov [82h],es         ; 存入口遠指標
//	cmp byte es:[bx],0EBh / jne done    ; 第一個 byte 必須是 EBh
//	mov al,es:[bx+1] / xor ah,ah / add ax,bx / add ax,<chain> / mov [86h],ax / mov [88h],es
//	                                    ; 第一個 byte 是 EBh：原處理常式 ＝ 入口 + 2 + rel8（chain 預設 2）
//	mov byte es:[bx],0EAh / mov word es:[bx+1],hook / mov ax,cs / mov es:[bx+3],ax
//	                                    ; 把入口前 5 bytes 改寫成 EA（遠跳到自己的掛鉤）
//	mov ax,0 / call far [80h]           ; AH=00h Get XMS Version
//	mov [90h],ax / jmp $
//
// hook：inc word [84h]，然後 jmp far [86h]（鏈回），或 retf（不鏈回）。
type hookProg struct {
	chainAdd uint16 // 加在 入口+rel8 上的常數；2 ＝ 入口+5（SCCD 的作法）
	noChain  bool
}

func (p hookProg) bytes() (code []byte, endIP uint16) {
	hookIP := uint16(65)
	code = []byte{
		0xB8, 0x10, 0x43, // mov ax,4310h
		0xCD, 0x2F, // int 2Fh
		0x89, 0x1E, 0x80, 0x00, // mov [80h],bx
		0x8C, 0x06, 0x82, 0x00, // mov [82h],es
		0x26, 0x80, 0x3F, 0xEB, // cmp byte es:[bx],0EBh
		0x75, 0x2C, // jne done（入口第一個 byte 不是 EBh：SCCD 會把 [入口+1] 當遠指標，這裡直接放棄）
		0x26, 0x8A, 0x47, 0x01, // mov al,es:[bx+1]
		0x32, 0xE4, // xor ah,ah
		0x01, 0xD8, // add ax,bx
		0x05, byte(p.chainAdd), byte(p.chainAdd >> 8), // add ax,imm16
		0xA3, 0x86, 0x00, // mov [86h],ax
		0x8C, 0x06, 0x88, 0x00, // mov [88h],es
		0x26, 0xC6, 0x07, 0xEA, // mov byte es:[bx],0EAh
		0x26, 0xC7, 0x47, 0x01, byte(hookIP), byte(hookIP >> 8), // mov word es:[bx+1],hook
		0x8C, 0xC8, // mov ax,cs
		0x26, 0x89, 0x47, 0x03, // mov es:[bx+3],ax
		0xB8, 0x00, 0x00, // mov ax,0
		0xFF, 0x1E, 0x80, 0x00, // call far [80h]
		0xA3, 0x90, 0x00, // mov [90h],ax
		0xEB, 0xFE, // jmp $
	}
	endIP = uint16(len(code) - 2)
	if len(code) != int(hookIP) {
		panic("hook 偏移與程式長度不符")
	}
	code = append(code, 0xFF, 0x06, 0x84, 0x00) // inc word [84h]
	if p.noChain {
		code = append(code, 0xCB) // retf
	} else {
		code = append(code, 0xFF, 0x2E, 0x86, 0x00) // jmp far [86h]
	}
	return code, endIP
}

// runAt 把 code 放到 hookSeg:0 執行，直到 IP 到 endIP（jmp $）；回步數。
func runAt(t *testing.T, m *machine.Machine, code []byte, endIP uint16) int {
	t.Helper()
	m.WriteBytes(cpu.Addr(hookSeg, 0), code)
	m.CPU.Seg[cpu.CS], m.CPU.IP = hookSeg, 0
	m.CPU.Seg[cpu.DS], m.CPU.Seg[cpu.ES] = hookSeg, hookSeg
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP] = 0x3000, 0xFFF0
	m.CPU.SetFlags(m.CPU.Flags &^ cpu.IF) // 不讓計時器中斷干擾
	for i := 0; i < 500; i++ {
		if m.CPU.Seg[cpu.CS] == hookSeg && m.CPU.IP == endIP {
			return i
		}
		if err := m.Step(); err != nil {
			t.Fatalf("第 %d 步：%v", i, err)
		}
	}
	t.Fatalf("500 步內沒到 %04X:%04X（停在 %04X:%04X）", hookSeg, endIP, m.CPU.Seg[cpu.CS], m.CPU.IP)
	return -1
}

func hookResult(m *machine.Machine) (counter, ax uint16) {
	return m.Read16(cpu.Addr(hookSeg, 0x84)), m.Read16(cpu.Addr(hookSeg, 0x90))
}

func TestHookChainToPlus5ServesNormally(t *testing.T) {
	m, d := newExtDOS(t, 4096)
	code, end := hookProg{chainAdd: 2}.bytes()
	runAt(t, m, code, end)
	counter, ax := hookResult(m)
	if counter != 1 {
		t.Errorf("掛鉤跑了 %d 次，預期 1", counter)
	}
	if ax != 0x0300 {
		t.Errorf("鏈回 +5 後 AH=00h 的 AX ＝ %04X，預期 0300（XMS 3.0）", ax)
	}
	// 入口前 5 bytes 被改寫成 EA，後 3 bytes（CD F5 CB）不動。
	b := uint32(machine.StubSeg)*16 + machine.XMSHookEntryOff
	if m.Mem[b] != 0xEA || !bytes.Equal(m.Mem[b+5:b+8], []byte{0xCD, 0xF5, 0xCB}) {
		t.Errorf("入口 ＝ % X", m.Mem[b:b+8])
	}
	_ = d
}

func TestHookWithoutChainingSkipsService(t *testing.T) {
	m, _ := newExtDOS(t, 4096)
	code, end := hookProg{chainAdd: 2, noChain: true}.bytes()
	runAt(t, m, code, end)
	counter, ax := hookResult(m)
	if counter != 1 {
		t.Errorf("掛鉤跑了 %d 次", counter)
	}
	if ax != 0 {
		t.Errorf("掛鉤不鏈回時 AX ＝ %04X；服務不得執行（AX 應維持呼叫前的 0）", ax)
	}
}

// 在已改寫成 EA 的入口上鏈到 +3：+3、+4 是掛鉤的段值（低位元組 CBh ＝ RETF），服務不執行。
func TestHookChainToPlus3AfterRewriteFails(t *testing.T) {
	m, _ := newExtDOS(t, 4096)
	code, end := hookProg{chainAdd: 0}.bytes() // 入口 + 3 ＋ 0：rel8 ＝ 3
	runAt(t, m, code, end)
	counter, ax := hookResult(m)
	if counter != 1 {
		t.Errorf("掛鉤跑了 %d 次", counter)
	}
	if ax == 0x0300 {
		t.Error("鏈到 +3 不得成功：該處已是 EA 改寫的一部分")
	}
}

// 未改寫前 +2、+3、+4 都是 NOP 或 jmp 的運算元，+2 起可成功：這是 +3 負對照的正對照。
func TestHookEntryUnpatchedPlus2To4AreNops(t *testing.T) {
	for _, add := range []uint16{2, 3, 4} {
		m, _ := newExtDOS(t, 4096)
		code := []byte{
			0xB8, 0x00, 0x00, // mov ax,0
			0xFF, 0x1E, 0x80, 0x00, // call far [80h]
			0xA3, 0x90, 0x00, // mov [90h],ax
			0xEB, 0xFE,
		}
		m.Write16(cpu.Addr(hookSeg, 0x80), machine.XMSHookEntryOff+add)
		m.Write16(cpu.Addr(hookSeg, 0x82), machine.StubSeg)
		m.WriteBytes(cpu.Addr(hookSeg, 0), code)
		// 資料要在 runAt 寫入程式之後仍在：程式只佔 0..11，資料在 0x80。
		runAt(t, m, code, uint16(len(code)-2))
		if _, ax := hookResult(m); ax != 0x0300 {
			t.Errorf("未改寫的入口 +%d：AX ＝ %04X，預期 0300（+2..+4 是 NOP，滑進 CD F5 CB）", add, ax)
		}
	}
}

// 預設（kb == 0）的入口仍是舊位置：int 2Fh 回 0x40C，經它呼叫服務照常。
func TestDefaultEntryStillServesFromOldLocation(t *testing.T) {
	m, _ := newTest(t)
	code := []byte{
		0xB8, 0x10, 0x43, 0xCD, 0x2F, // mov ax,4310h / int 2Fh
		0x89, 0x1E, 0x80, 0x00, 0x8C, 0x06, 0x82, 0x00,
		0xB8, 0x00, 0x00, 0xFF, 0x1E, 0x80, 0x00, // mov ax,0 / call far [80h]
		0xA3, 0x90, 0x00, 0xEB, 0xFE,
	}
	runAt(t, m, code, uint16(len(code)-2))
	if _, ax := hookResult(m); ax != 0x0300 {
		t.Errorf("AX ＝ %04X", ax)
	}
	if got := m.Read16(cpu.Addr(hookSeg, 0x80)); got != machine.XMSTrapOff {
		t.Errorf("入口位移 ＝ %04X，預期 %04X", got, machine.XMSTrapOff)
	}
}

// 版面守門（比照 stub_layout_test.go）：0x4B0–0x4B7 不與常數區塊任何項目重疊，且小於 EnvMCBSeg 換算的 0x4F0。
func TestXMSHookEntryLayoutGuard(t *testing.T) {
	type item struct {
		name      string
		off, size uint32
	}
	items := []item{
		{"DosTrapOff", machine.DosTrapOff, 3},
		{"VideoTrapOff", machine.VideoTrapOff, 3},
		{"ATrapOff", machine.ATrapOff, 3},
		{"XMSTrapOff", machine.XMSTrapOff, 3},
		{"CallbackRetOff", machine.CallbackRetOff, 2},
		{"FontFullOff", machine.FontFullOff, 3},
		{"FontHalfOff", machine.FontHalfOff, 3},
		{"biosTimerOff（0x420，35 bytes）", 0x420, 35},
		{"SysConfigOff", machine.SysConfigOff, 8},
		{"InDOSOff", machine.InDOSOff, 1},
		{"DBCSTableOff", machine.DBCSTableOff, 32},
		{"CountryOff", machine.CountryOff, 34},
	}
	hook := item{"XMSHookEntryOff", machine.XMSHookEntryOff, 8}
	if hook.off != 0x4B0 {
		t.Fatalf("XMSHookEntryOff ＝ %#x，規格是 0x4B0", hook.off)
	}
	envLimit := uint32(machine.EnvMCBSeg-machine.StubSeg) * 16
	if envLimit != 0x4F0 {
		t.Fatalf("EnvMCBSeg 換算的上界 ＝ %#x，預期 0x4F0", envLimit)
	}
	if hook.off+hook.size > envLimit {
		t.Errorf("入口 %#x–%#x 超過上界 %#x", hook.off, hook.off+hook.size, envLimit)
	}
	for _, it := range items {
		if it.off < 0x400 {
			t.Errorf("%s 落在 per-vector stub 區", it.name)
		}
		if hook.off < it.off+it.size && it.off < hook.off+hook.size {
			t.Errorf("入口與 %s（%#x+%d）重疊", it.name, it.off, it.size)
		}
	}
}

// Install() 後入口 8 bytes 完整；相鄰的國別資訊表與系統設定表內容不變。
func TestXMSHookEntryIntactAndNeighborsUnchanged(t *testing.T) {
	m0, d0 := newTest(t)
	m1, d1 := newExtDOS(t, 4096)
	for _, p := range []struct {
		m *machine.Machine
		d *DOS
	}{{m0, d0}, {m1, d1}} {
		call(p.m, p.d, 0x15, 0xC000) // 系統設定表
		call(p.m, p.d, 0x21, 0x3800) // 國別資訊
		call(p.m, p.d, 0x21, 0x6300) // DBCS 表
		call(p.m, p.d, 0x21, 0x3400) // InDOS
	}
	base := uint32(machine.StubSeg) * 16
	hook := m1.Mem[base+machine.XMSHookEntryOff : base+machine.XMSHookEntryOff+8]
	if !bytes.Equal(hook, []byte{0xEB, 0x03, 0x90, 0x90, 0x90, 0xCD, 0xF5, 0xCB}) {
		t.Errorf("入口 ＝ % X", hook)
	}
	// 0x400–0x4AF 與 0x4B8–0x4EF 兩台完全相同。
	if !bytes.Equal(m0.Mem[base+0x400:base+0x4B0], m1.Mem[base+0x400:base+0x4B0]) {
		t.Error("0x400–0x4AF（含國別資訊表與系統設定表）在 kb > 0 時被改動")
	}
	if !bytes.Equal(m0.Mem[base+0x4B8:base+0x4F0], m1.Mem[base+0x4B8:base+0x4F0]) {
		t.Error("0x4B8–0x4EF 被改動")
	}
	// 0x000–0x3FF 的 per-vector stub 不變。
	if !bytes.Equal(m0.Mem[base:base+0x400], m1.Mem[base:base+0x400]) {
		t.Error("per-vector stub 區被改動")
	}
}

// 驗收 7（DOS 側）：SetEMSPresent(false) 之後。
func TestNoEMSDOSSide(t *testing.T) {
	old := machine.MemPoison
	machine.MemPoison = 0xCC
	t.Cleanup(func() { machine.MemPoison = old })

	m := machine.New()
	if err := m.SetEMSPresent(false); err != nil {
		t.Fatal(err)
	}
	d := New(m, t.TempDir())
	d.Install()
	d.freeSeg = 0x2000

	// int 67h 向量段的 :000A 不是 EMMXXXX0；EMSSeg 區還原成 0xCC。
	seg := m.Read16(0x67*4 + 2)
	if got := m.Mem[uint32(seg)*16+0x0A : uint32(seg)*16+0x12]; bytes.Equal(got, []byte("EMMXXXX0")) {
		t.Error("int 67h 向量段仍有 EMMXXXX0")
	}
	for i := uint32(0); i < 3; i++ {
		if m.Mem[uint32(machine.EMSSeg)*16+i] != 0xCC {
			t.Errorf("EMSSeg 區 byte %d 未還原成 0xCC", i)
		}
	}
	for i := uint32(0); i < 8; i++ {
		if m.Mem[uint32(machine.EMSSeg)*16+0x0A+i] != 0xCC {
			t.Errorf("EMSSeg 區簽章 byte %d 未還原成 0xCC", i)
		}
	}

	// int 67h AH=40h：暫存器不變，並在診斷留一筆。
	m.CPU.R[cpu.AX], m.CPU.R[cpu.BX], m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 0x4000, 0x1111, 0x2222, 0x3333
	m.CPU.R[cpu.SI], m.CPU.R[cpu.DI] = 0x4444, 0x5555
	before := m.CPU.R
	flags := m.CPU.Flags
	d.handle(m.CPU, 0x67)
	if m.CPU.R != before || m.CPU.Flags != flags {
		t.Errorf("暫存器應原樣返回：%v → %v", before, m.CPU.R)
	}
	if d.Unimplemented[Call{Int: 0x67, AH: 0x40, AL: 0}] != 1 {
		t.Errorf("Unimplemented 應記一筆 67h/40h：%v", d.Unimplemented)
	}
	found := false
	for _, det := range d.UnimplementedDetails {
		if det.Int == 0x67 && det.AH == 0x40 && det.AX == 0x4000 && det.BX == 0x1111 {
			found = true
		}
	}
	if !found {
		t.Error("noteCPU 應附暫存器的詳細紀錄")
	}
	// 經 trampoline 的 int F6h 也走同一個閘。
	m.CPU.R = before
	d.handle(m.CPU, 0xF6)
	if m.CPU.R != before {
		t.Error("int F6h 也應原樣返回")
	}

	// 開 EMMXXXX0 回找不到檔案；fileAttr 回 2。
	m.WriteBytes(cpu.Addr(0x3000, 0), []byte("EMMXXXX0\x00"))
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x3000, 0
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 2 {
		t.Errorf("open EMMXXXX0：CF ＝ %v、AX ＝ %d，預期 CF 且 AX＝2", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x3000, 0
	call(m, d, 0x21, 0x4300)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 2 {
		t.Errorf("fileAttr EMMXXXX0：CF ＝ %v、AX ＝ %d，預期 CF 且 AX＝2", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
}

// 預設（EMS 呈現）：EMMXXXX0 可開、int 67h 照常服務。
func TestDefaultEMSStillServes(t *testing.T) {
	m, d := newTest(t)
	m.WriteBytes(cpu.Addr(0x3000, 0), []byte("EMMXXXX0\x00"))
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x3000, 0
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Error("預設應可開 EMMXXXX0")
	}
	call(m, d, 0xF6, 0x4600)
	if m.CPU.R[cpu.AX] != 0x0040 {
		t.Errorf("EMS 版本 AX ＝ %04X，預期 0040", m.CPU.R[cpu.AX])
	}
}
