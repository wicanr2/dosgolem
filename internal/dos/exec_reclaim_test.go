package dos

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 規格 441：巢狀子行程在 EXEC 時 arena 為 nil（nil 路徑落子）、執行中才建 arena，
// 非常駐結束時把 [procFrame.freeSeg, arena[0].seg) 併回 arena（選項 ExecReclaim）。
// 沒有併回時這段永遠不在 arena 內，下一支大程式的 EXEC 回錯誤 8（SCCD 的 strike.exe）。

// reclaimScratch 是測試用 EXEC 參數（檔名、參數區塊）放的段：freeSeg（2000h）之下、
// 不碰 exec_env_test 的 parentEnvAt（1800h），也不會落進子行程或 arena。
const reclaimScratch = 0x1000

// execFrom 同 execChild，參數放在 reclaimScratch。
func execFrom(m *machine.Machine, d *DOS, name string) {
	m.CPU.Seg[cpu.DS] = reclaimScratch
	m.CPU.R[cpu.DX] = 0
	m.WriteBytes(cpu.Addr(reclaimScratch, 0), append([]byte(name), 0))
	m.CPU.Seg[cpu.ES] = reclaimScratch
	m.CPU.R[cpu.BX] = 0x80
	m.Write16(cpu.Addr(reclaimScratch, 0x80), 0)
	m.Write16(cpu.Addr(reclaimScratch, 0x82), 0)
	m.Write16(cpu.Addr(reclaimScratch, 0x84), 0)
	call(m, d, 0x21, 0x4B00)
}

func mustExecFrom(t *testing.T, m *machine.Machine, d *DOS, name string) uint16 {
	t.Helper()
	execFrom(m, d, name)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("EXEC %s 失敗，AX=%04X Missing=%v", name, m.CPU.R[cpu.AX], d.Missing)
	}
	return d.ExecLog[len(d.ExecLog)-1].PSP
}

// stepToDepth 單步到行程疊深度為 n（子行程結束、回到父行程）。
func stepToDepth(t *testing.T, m *machine.Machine, d *DOS, n int) {
	t.Helper()
	for i := 0; i < 400 && len(d.procStack) != n; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != n {
		t.Fatalf("沒有走到行程疊深度 %d", n)
	}
}

func newReclaimTest(t *testing.T, on bool) (*machine.Machine, *DOS) {
	t.Helper()
	m, d := newTest(t)
	d.ExecReclaim = on
	return m, d
}

// 機器碼片段。
func movBX(v uint16) []byte { return []byte{0xBB, byte(v), byte(v >> 8)} }
func movDX(v uint16) []byte { return []byte{0xBA, byte(v), byte(v >> 8)} }
func movAX(v uint16) []byte { return []byte{0xB8, byte(v), byte(v >> 8)} }

var (
	int21h    = []byte{0xCD, 0x21}
	exit4C    = []byte{0xB8, 0x00, 0x4C, 0xCD, 0x21} // mov ax,4C00h; int 21h
	exitInt20 = []byte{0xCD, 0x20}                   // int 20h
	exitAH00  = []byte{0xB4, 0x00, 0xCD, 0x21}       // mov ah,0; int 21h
	movESAX   = []byte{0x8E, 0xC0}
	movDXES   = []byte{0x8C, 0xC2}
	movESDX   = []byte{0x8E, 0xC2}
)

func code(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

// setBlock4A：mov ah,4Ah; mov bx,want; int 21h（ES 是自己的 PSP）。
func setBlock4A(want uint16) []byte { return code([]byte{0xB4, 0x4A}, movBX(want), int21h) }

// alloc48：mov ah,48h; mov bx,want; int 21h。
func alloc48(want uint16) []byte { return code([]byte{0xB4, 0x48}, movBX(want), int21h) }

// shrinkAllocExit：AH=4Ah 縮到 want、AH=48h 拿 10h 段，再以 exit 結束。
func shrinkAllocExit(want uint16, exit []byte) []byte {
	return code(setBlock4A(want), alloc48(0x10), exit)
}

// shrinkAllocStopIP 是 shrinkAllocExit 的 AH=48h 做完、還沒結束時的 IP。
const shrinkAllocStopIP = 0x100 + 14

// checkMCBChain 從 PSPSeg−1 沿客體 MCB 鏈走到 Z：第一塊之後每塊的位置、大小、自由與否等於 arena。
func checkMCBChain(t *testing.T, m *machine.Machine, d *DOS) {
	t.Helper()
	seg := uint16(machine.PSPSeg - 1)
	at := uint32(seg) * 16
	if typ := m.Read8(at); typ != 'M' {
		t.Fatalf("PSPSeg−1 的 MCB 型別 %02X，預期 'M'", typ)
	}
	next := seg + m.Read16(at+3) + 1
	if next != d.freeSeg {
		t.Errorf("PSPSeg−1 涵蓋到 %04X，預期 freeSeg %04X", next, d.freeSeg)
	}
	for i, b := range d.arena {
		if next != b.seg {
			t.Fatalf("MCB 鏈第 %d 塊在 %04X，arena 在 %04X", i+1, next, b.seg)
		}
		at = uint32(next) * 16
		want := byte('M')
		if i == len(d.arena)-1 {
			want = 'Z'
		}
		if typ := m.Read8(at); typ != want {
			t.Errorf("MCB %04X 型別 %02X，預期 %02X", next, typ, want)
		}
		if size := m.Read16(at + 3); size != b.size {
			t.Errorf("MCB %04X 大小 %04X，arena %04X", next, size, b.size)
		}
		if free := m.Read16(at+1) == 0; free != b.free {
			t.Errorf("MCB %04X 自由＝%v，arena %v", next, free, b.free)
		}
		next = b.seg + b.size + 1
	}
	if next != uint16(machine.MemTop) {
		t.Errorf("arena 尾端 %04X，預期 MemTop", next)
	}
}

// 重現：父行程不碰記憶體，nil 路徑落子；子行程縮自己、拿一塊後結束。
// 開啟時併回、再 EXEC 一支大於舊 arena 最大塊的程式成功；關閉時回錯誤 8。
func TestExecReclaimNilPathChild(t *testing.T) {
	// big 的映像 A00h 段（imageParags＝A11h），大於關閉時剩下的最大塊 7FEh。
	big := make([]byte, 0xA000)
	copy(big, exit4C)
	run := func(t *testing.T, on bool) (*machine.Machine, *DOS, uint16) {
		m, d := newReclaimTest(t, on)
		writeChild(t, d, "CHILD.COM", shrinkAllocExit(0x7800, exit4C))
		writeChild(t, d, "BIG.COM", big)
		mustExecFrom(t, m, d, "CHILD.COM")
		f := d.procStack[0]
		if f.arenaSeg != 0 || f.freeSeg != 0x2000 {
			t.Fatalf("前提：nil 路徑，arenaSeg=%04X freeSeg=%04X", f.arenaSeg, f.freeSeg)
		}
		runChild(t, m, d)
		return m, d, f.freeSeg
	}

	t.Run("關閉", func(t *testing.T) {
		m, d, _ := run(t, false)
		old := d.largestFree()
		if need := imageParags(big); need-1 <= old {
			t.Fatalf("前提：big 需 %04X 段，舊 arena 最大塊 %04X 放得下", need-1, old)
		}
		execFrom(m, d, "BIG.COM")
		if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 8 {
			t.Errorf("關閉時第二次 EXEC：CF=%v AX=%04X，預期錯誤 8", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
		}
		if len(d.Reclaims) != 0 {
			t.Errorf("關閉時 Reclaims＝%+v，預期空", d.Reclaims)
		}
	})
	t.Run("開啟", func(t *testing.T) {
		m, d, from := run(t, true)
		if got, want := d.largestFree(), uint16(machine.MemTop)-from-1; got != want {
			t.Errorf("結束後 largestFree＝%04X，預期 %04X", got, want)
		}
		if d.freeSeg != from {
			t.Errorf("freeSeg＝%04X，預期 %04X", d.freeSeg, from)
		}
		if len(d.Reclaims) != 1 || d.Reclaims[0].Blocked || d.Reclaims[0].FreeSeg != from ||
			d.Reclaims[0].ArenaSeg != 0x2001+0x7800 {
			t.Errorf("Reclaims＝%+v", d.Reclaims)
		}
		checkMCBChain(t, m, d)
		psp := mustExecFrom(t, m, d, "BIG.COM")
		if psp != from+1 {
			t.Errorf("BIG.COM 的 PSP＝%04X，預期 %04X", psp, from+1)
		}
	})
}

// 結束路徑：AH=4Ch、int 20h、AH=00h 都要併回。
func TestExecReclaimExitPaths(t *testing.T) {
	for _, tc := range []struct {
		name string
		exit []byte
	}{
		{"AH=4Ch", exit4C},
		{"int 20h", exitInt20},
		{"AH=00h", exitAH00},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, d := newReclaimTest(t, true)
			writeChild(t, d, "CHILD.COM", shrinkAllocExit(0x7800, tc.exit))
			mustExecFrom(t, m, d, "CHILD.COM")
			runChild(t, m, d)
			if got := d.largestFree(); got != uint16(machine.MemTop)-0x2000-1 {
				t.Errorf("largestFree＝%04X", got)
			}
			if len(d.arena) != 1 || d.arena[0].seg != 0x2000 {
				t.Errorf("arena＝%+v", d.arena)
			}
			checkMCBChain(t, m, d)
		})
	}
}

// 子行程 AH=48h 拿到的塊自己 AH=49h 還掉再結束：releaseOwnedBy 回 0、不會先 sync，
// 客體 MCB 鏈只能靠併回時的 syncMCB 更新。
func TestExecReclaimSyncsMCBWithoutRelease(t *testing.T) {
	m, d := newReclaimTest(t, true)
	writeChild(t, d, "CHILD.COM", code(setBlock4A(0x7800), alloc48(0x10), movESAX,
		[]byte{0xB4, 0x49}, int21h, exit4C))
	mustExecFrom(t, m, d, "CHILD.COM")
	runChild(t, m, d)
	if len(d.Reclaims) != 1 || d.Reclaims[0].Blocked {
		t.Fatalf("Reclaims＝%+v", d.Reclaims)
	}
	if len(d.arena) != 1 || d.arena[0].seg != 0x2000 {
		t.Errorf("arena＝%+v", d.arena)
	}
	checkMCBChain(t, m, d)
}

// ExecEnvCopy：f.freeSeg 處原本是子行程的環境 MCB，併回後改寫成自由 MCB。
func TestExecReclaimRewritesEnvMCB(t *testing.T) {
	m, d := newEnvTest(t)
	d.ExecReclaim = true
	writeChild(t, d, "CHILD.COM", shrinkAllocExit(0x7000, exit4C))
	execChildEnv(m, d, "CHILD.COM", 0)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("EXEC 失敗，AX=%04X", m.CPU.R[cpu.AX])
	}
	psp := d.curPSP
	env := d.ExecLog[len(d.ExecLog)-1].EnvSeg
	at := uint32(0x2000) * 16
	if env != 0x2001 || m.Read16(at+1) != psp {
		t.Fatalf("前提：環境段 %04X、2000h 的 MCB 擁有者 %04X（子 PSP %04X）", env, m.Read16(at+1), psp)
	}
	runChild(t, m, d)
	if typ, owner, size := m.Read8(at), m.Read16(at+1), m.Read16(at+3); typ != 'Z' || owner != 0 ||
		size != uint16(machine.MemTop)-0x2000-1 {
		t.Errorf("2000h 的 MCB：型別 %02X 擁有者 %04X 大小 %04X，預期自由的 Z 塊", typ, owner, size)
	}
	checkMCBChain(t, m, d)
}

// 巢狀深度 2：R → C（nil）→ G（nil），G 建 arena 後結束，再 C 結束。
func TestExecReclaimNested(t *testing.T) {
	m, d := newReclaimTest(t, true)
	writeChild(t, d, "C.COM", exit4C)
	writeChild(t, d, "G.COM", shrinkAllocExit(0x7000, exit4C))
	mustExecFrom(t, m, d, "C.COM")
	mustExecFrom(t, m, d, "G.COM")
	fG := d.procStack[1]
	if d.procStack[0].arenaSeg != 0 || fG.arenaSeg != 0 || fG.freeSeg <= 0x2000 {
		t.Fatalf("前提：兩層都是 nil 路徑，procStack=%+v", d.procStack)
	}
	stepToDepth(t, m, d, 1)
	if got, want := d.largestFree(), uint16(machine.MemTop)-fG.freeSeg-1; got != want {
		t.Errorf("G 結束後 largestFree＝%04X，預期 %04X", got, want)
	}
	if d.freeSeg != fG.freeSeg {
		t.Errorf("G 結束後 freeSeg＝%04X，預期 %04X", d.freeSeg, fG.freeSeg)
	}
	checkMCBChain(t, m, d)
	runChild(t, m, d)
	if got := d.largestFree(); got != uint16(machine.MemTop)-0x2000-1 {
		t.Errorf("C 結束後 largestFree＝%04X", got)
	}
	if len(d.Reclaims) != 2 || d.Reclaims[0].Blocked || d.Reclaims[1].Blocked {
		t.Errorf("Reclaims＝%+v", d.Reclaims)
	}
	checkMCBChain(t, m, d)
}

// TSR 護欄：C（nil）→ G（nil）常駐。C 結束時常駐區不得併回，Reclaims 記一筆被擋下。
func TestExecReclaimTSRGuard(t *testing.T) {
	for _, tc := range []struct {
		name  string
		c, g  []byte
		arena bool // G 常駐時 arena 已建
	}{
		{
			// G 自己 AH=4Ah、AH=48h 建了 arena 再常駐。
			name: "G 建 arena 後常駐", arena: true, c: exit4C,
			g: code(setBlock4A(0x100), alloc48(0x10), movAX(0x3100), movDX(0x100), int21h),
		},
		{
			// G 在 arena nil 時常駐，C 回來後 AH=48h 才建 arena。
			name: "G 在 arena nil 時常駐", arena: false, c: allocThenExit,
			g: code(movAX(0x3100), movDX(0x20), int21h),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, d := newReclaimTest(t, true)
			writeChild(t, d, "C.COM", tc.c)
			writeChild(t, d, "G.COM", tc.g)
			mustExecFrom(t, m, d, "C.COM")
			gPSP := mustExecFrom(t, m, d, "G.COM")
			stepToDepth(t, m, d, 1)
			if (d.arena != nil) != tc.arena {
				t.Fatalf("前提：G 常駐後 arena 建立＝%v，預期 %v", d.arena != nil, tc.arena)
			}
			if d.nilTSRTop < gPSP+0x20 {
				t.Errorf("nilTSRTop＝%04X，預期至少 %04X", d.nilTSRTop, gPSP+0x20)
			}
			runChild(t, m, d)
			if len(d.Reclaims) != 1 || !d.Reclaims[0].Blocked {
				t.Fatalf("Reclaims＝%+v，預期一筆被擋下", d.Reclaims)
			}
			if d.arena[0].seg != d.Reclaims[0].ArenaSeg || d.arena[0].seg < d.nilTSRTop {
				t.Errorf("arena[0]＝%+v，常駐區（頂端 %04X）被併回", d.arena[0], d.nilTSRTop)
			}
			if d.freeSeg != 0x2000 {
				t.Errorf("freeSeg＝%04X，預期 2000", d.freeSeg)
			}
		})
	}
}

// arena[0] 是已用塊：插入的自由塊不與它合併。
// C 縮自己、拿 A 塊；G（arena 路徑）還掉 A 再拿同大小的一塊（落在 A 原位，擁有者 G）後常駐。
func TestExecReclaimKeepsUsedArenaHead(t *testing.T) {
	m, d := newReclaimTest(t, true)
	writeChild(t, d, "C.COM", code(setBlock4A(0x100), alloc48(0x10), exit4C))
	cPSP := mustExecFrom(t, m, d, "C.COM")
	stepToIP(t, m, shrinkAllocStopIP)
	base := cPSP + 0x100
	if d.arena[0].seg != base || d.arena[0].free {
		t.Fatalf("前提：A 塊在 %04X，arena=%+v", base, d.arena)
	}
	writeChild(t, d, "G.COM", code(movAX(base+1), movESAX, []byte{0xB4, 0x49}, int21h,
		alloc48(0x10), movAX(0x3100), movDX(0x10), int21h))
	gPSP := mustExecFrom(t, m, d, "G.COM")
	if d.procStack[1].arenaSeg == 0 {
		t.Fatal("前提：G 走 arena 路徑")
	}
	stepToDepth(t, m, d, 1)
	if b := d.arena[0]; b.seg != base || b.free || b.owner != gPSP {
		t.Fatalf("前提：arena[0]＝%+v，預期 G（%04X）名下的已用塊", b, gPSP)
	}
	runChild(t, m, d)
	if len(d.arena) < 2 {
		t.Fatalf("arena＝%+v", d.arena)
	}
	if b := d.arena[0]; b.seg != 0x2000 || !b.free || b.size != base-0x2000-1 {
		t.Errorf("arena[0]＝%+v，預期自由塊 {2000, %04X}", b, base-0x2000-1)
	}
	if b := d.arena[1]; b.seg != base || b.free {
		t.Errorf("arena[1]＝%+v，預期 %04X 的已用塊（不得被合併）", b, base)
	}
	checkMCBChain(t, m, d)
}

// gap == 1：子行程把自己縮到 0 段，arena 從 f.freeSeg+1 起。不插塊、freeSeg 退回、arena 不變。
func TestExecReclaimGapOne(t *testing.T) {
	m, d := newReclaimTest(t, true)
	// AH=48h 拿 20h 段：剩餘自由塊的 MCB 落在程式碼之後。
	writeChild(t, d, "CHILD.COM", code(setBlock4A(0), alloc48(0x20), exit4C))
	mustExecFrom(t, m, d, "CHILD.COM")
	runChild(t, m, d)
	want := []memBlock{{seg: 0x2001, size: uint16(machine.MemTop) - 0x2001 - 1, free: true}}
	if !reflect.DeepEqual(d.arena, want) {
		t.Errorf("arena＝%+v，預期 %+v", d.arena, want)
	}
	if d.freeSeg != 0x2000 {
		t.Errorf("freeSeg＝%04X，預期 2000", d.freeSeg)
	}
	if len(d.Reclaims) != 1 || d.Reclaims[0].Blocked {
		t.Errorf("Reclaims＝%+v", d.Reclaims)
	}
}

// 子行程建 arena 後 AH=4Ah 放大自己（吃掉 arena 前端的自由部分）再結束。
func TestExecReclaimAfterGrow(t *testing.T) {
	m, d := newReclaimTest(t, true)
	writeChild(t, d, "CHILD.COM", code(movDXES, setBlock4A(0x100), alloc48(0x10), movESAX,
		[]byte{0xB4, 0x49}, int21h, movESDX, setBlock4A(0x140), exit4C))
	psp := mustExecFrom(t, m, d, "CHILD.COM")
	// 最後一個 int 21h（AH=4Ah 放大）之後停住。
	stepToIP(t, m, 0x100+2+7+7+2+4+2+7)
	if d.arena[0].seg != psp+0x140 || !d.arena[0].free {
		t.Fatalf("前提：放大後 arena[0]＝%+v，預期 %04X 起的自由塊", d.arena[0], psp+0x140)
	}
	runChild(t, m, d)
	if len(d.arena) != 1 || d.arena[0].seg != 0x2000 || d.largestFree() != uint16(machine.MemTop)-0x2000-1 {
		t.Errorf("arena＝%+v", d.arena)
	}
	checkMCBChain(t, m, d)
}

// 讀檔：nil 路徑子行程執行中、arena 已建時存檔，新機器讀檔（440 重建後 arenaSeg=0）；
// 子行程結束後照樣併回。新 DOS 的 nilTSRTop 在 LoadState 前弄髒，讀檔後必須歸零。
func TestExecReclaimAfterLoadState(t *testing.T) {
	m, d := newReclaimTest(t, true)
	writeChild(t, d, "CHILD.COM", shrinkAllocExit(0x7800, exit4C))
	mustExecFrom(t, m, d, "CHILD.COM")
	stepToIP(t, m, shrinkAllocStopIP)
	if d.arena == nil {
		t.Fatal("前提：arena 應已建立")
	}
	var mb, db bytes.Buffer
	if err := m.SaveState(&mb); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveState(&db); err != nil {
		t.Fatal(err)
	}
	m2 := machine.New()
	if err := m2.LoadState(&mb); err != nil {
		t.Fatal(err)
	}
	d2 := New(m2, d.Root)
	d2.ExecReclaim = true
	d2.Install()
	d2.nilTSRTop = 0xFFFF
	if err := d2.LoadState(&db); err != nil {
		t.Fatal(err)
	}
	if d2.nilTSRTop != 0 {
		t.Errorf("讀檔後 nilTSRTop＝%04X，預期 0", d2.nilTSRTop)
	}
	if len(d2.procStack) != 1 || d2.procStack[0].arenaSeg != 0 {
		t.Fatalf("讀檔後 procStack＝%+v，預期一層、arenaSeg=0", d2.procStack)
	}
	runChild(t, m2, d2)
	if got := d2.largestFree(); got != uint16(machine.MemTop)-0x2000-1 {
		t.Errorf("讀檔後子行程結束，largestFree＝%04X", got)
	}
	if len(d2.Reclaims) != 1 || d2.Reclaims[0].Blocked {
		t.Errorf("Reclaims＝%+v", d2.Reclaims)
	}
	checkMCBChain(t, m2, d2)
}

// 條件不成立（子程式不碰記憶體，arena 仍 nil）：開啟與關閉的結果相同。
func TestExecReclaimNoArenaSameAsOff(t *testing.T) {
	type result struct {
		FreeSeg  uint16
		ArenaNil bool
		Reclaims int
		Mem      []byte
	}
	run := func(on bool) result {
		m, d := newReclaimTest(t, on)
		writeChild(t, d, "CHILD.COM", exitOnly)
		mustExecFrom(t, m, d, "CHILD.COM")
		runChild(t, m, d)
		return result{d.freeSeg, d.arena == nil, len(d.Reclaims),
			append([]byte(nil), m.Mem[:machine.MemTop*16]...)}
	}
	off, on := run(false), run(true)
	if !on.ArenaNil || on.Reclaims != 0 {
		t.Errorf("開啟：arena nil＝%v、Reclaims %d 筆", on.ArenaNil, on.Reclaims)
	}
	if off.FreeSeg != on.FreeSeg || off.ArenaNil != on.ArenaNil || !bytes.Equal(off.Mem, on.Mem) {
		t.Errorf("開啟與關閉不同：freeSeg %04X／%04X", on.FreeSeg, off.FreeSeg)
	}
}

// 防禦條件 f.freeSeg < arena[0].seg：正常流程到不了，以人工狀態驗。arena[0] 不在框架 freeSeg
// 之上時不記錄、不併回，arena 不動。
func TestExecReclaimRequiresArenaAboveFrame(t *testing.T) {
	for _, head := range []uint16{0x2000, 0x1F00} {
		m, d := newReclaimTest(t, true)
		writeChild(t, d, "CHILD.COM", exitOnly)
		mustExecFrom(t, m, d, "CHILD.COM")
		d.arena = []memBlock{{seg: head, size: uint16(machine.MemTop) - head - 1, free: true}}
		want := append([]memBlock(nil), d.arena...)
		runChild(t, m, d)
		if len(d.Reclaims) != 0 || !reflect.DeepEqual(d.arena, want) {
			t.Errorf("arena[0]=%04X：Reclaims＝%+v、arena＝%+v", head, d.Reclaims, d.arena)
		}
		if d.freeSeg != 0x2000 {
			t.Errorf("arena[0]=%04X：freeSeg＝%04X", head, d.freeSeg)
		}
	}
}
