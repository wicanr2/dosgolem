package dos

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 規格 440：讀檔後由客體 MCB 與行程疊重建 arena 擁有者、框架的 arenaSeg 與 queuedSeg。
// 狀態檔不存這三項，少了重建，子行程結束時記憶體收不回來，下一次 EXEC 回錯誤 8。

// 子程式：mov ah,48h; mov bx,0010h; int 21h; mov ax,4C00h; int 21h。
var allocThenExit = []byte{0xB4, 0x48, 0xBB, 0x10, 0x00, 0xCD, 0x21, 0xB8, 0x00, 0x4C, 0xCD, 0x21}

// 子程式：mov ax,4C00h; int 21h。
var exitOnly = []byte{0xB8, 0x00, 0x4C, 0xCD, 0x21}

// allocDoneIP 是 allocThenExit 第一個 int 21h 之後的 IP（COM 從 100h 起）。
const allocDoneIP = 0x107

// stepToIP 單步到 IP 等於 ip（子行程的 AH=48h 做完、還沒結束）。
func stepToIP(t *testing.T, m *machine.Machine, ip uint16) {
	t.Helper()
	for i := 0; i < 50 && m.CPU.IP != ip; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.CPU.IP != ip {
		t.Fatalf("沒有走到 IP=%04X（目前 %04X:%04X）", ip, m.CPU.Seg[cpu.CS], m.CPU.IP)
	}
}

// saveLoad 仿真實流程：m.SaveState、新機器 LoadState、新 DOS Install 後 LoadState。
// 順便驗 save → load → save 的 DOSState 位元組相同（§3 第 1 項）。
func saveLoad(t *testing.T, m *machine.Machine, d *DOS) (*machine.Machine, *DOS) {
	t.Helper()
	var mb, db bytes.Buffer
	if err := m.SaveState(&mb); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveState(&db); err != nil {
		t.Fatal(err)
	}
	saved := append([]byte(nil), db.Bytes()...)
	m2 := machine.New()
	if err := m2.LoadState(&mb); err != nil {
		t.Fatal(err)
	}
	d2 := New(m2, d.Root)
	d2.ExecEnvCopy = d.ExecEnvCopy
	d2.Install()
	if err := d2.LoadState(&db); err != nil {
		t.Fatal(err)
	}
	var again bytes.Buffer
	if err := d2.SaveState(&again); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, again.Bytes()) {
		t.Fatal("save → load → save 的 DOSState 位元組不同")
	}
	return m2, d2
}

// ownership 是往返要比對的部分：已用塊的擁有者（自由塊略過擁有者）、框架 arenaSeg、queuedSeg。
type ownership struct {
	Blocks   []memBlock
	ArenaSeg []uint16
	Queued   uint16
	ArenaNil bool
}

func snapOwnership(d *DOS) ownership {
	o := ownership{Queued: d.queuedSeg, ArenaNil: d.arena == nil}
	for _, b := range d.arena {
		if b.free {
			b.owner = 0
		}
		o.Blocks = append(o.Blocks, b)
	}
	for _, f := range d.procStack {
		o.ArenaSeg = append(o.ArenaSeg, f.arenaSeg)
	}
	return o
}

// checkChildPSPs 斷言不變式：procStack[i+1].psp（最上層為 curPSP）是第 i 個子行程的 PSP。
func checkChildPSPs(t *testing.T, d *DOS, want []uint16) {
	t.Helper()
	if len(d.procStack) != len(want) {
		t.Fatalf("行程疊深度 %d，預期 %d", len(d.procStack), len(want))
	}
	for i := range d.procStack {
		got := d.curPSP
		if i+1 < len(d.procStack) {
			got = d.procStack[i+1].psp
		}
		if got != want[i] {
			t.Errorf("第 %d 個子行程 PSP＝%04X，預期 %04X", i, got, want[i])
		}
	}
}

func blockAt(t *testing.T, d *DOS, seg uint16) memBlock {
	t.Helper()
	i := d.arenaIndex(seg)
	if i < 0 {
		t.Fatalf("arena 沒有 MCB 段 %04X", seg)
	}
	return d.arena[i]
}

func parentAlloc(t *testing.T, m *machine.Machine, d *DOS) uint16 {
	t.Helper()
	m.CPU.R[cpu.BX] = 0x100
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("父行程 AH=48h 失敗，Missing=%v", d.Missing)
	}
	return m.CPU.R[cpu.AX]
}

func mustExecChild(t *testing.T, m *machine.Machine, d *DOS, name string) uint16 {
	t.Helper()
	execChild(m, d, name)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("EXEC %s 失敗，AX=%04X Missing=%v", name, m.CPU.R[cpu.AX], d.Missing)
	}
	return d.ExecLog[len(d.ExecLog)-1].PSP
}

// execAfterChild 是 §3 第 2 項第一個案例：子行程 AH=48h 後停住，（可選）存讀，
// 讓子行程結束，再 EXEC 另一支子程式。
type execAfterResult struct {
	LargestFree uint16
	Arena       []memBlock
	PSP2        uint16
	CF          bool
	Mem         []byte
}

func runExecAfterChild(t *testing.T, doSaveLoad bool) execAfterResult {
	t.Helper()
	m, d := newTest(t)
	parentAlloc(t, m, d)
	before := d.largestFree()
	writeChild(t, d, "CHILD.COM", allocThenExit)
	writeChild(t, d, "CHILD2.COM", exitOnly)
	mustExecChild(t, m, d, "CHILD.COM")
	stepToIP(t, m, allocDoneIP)
	if doSaveLoad {
		m, d = saveLoad(t, m, d)
	}
	runChild(t, m, d)
	if got := d.largestFree(); got != before {
		t.Errorf("子行程結束後 largestFree＝%04X，啟動前 %04X（記憶體沒收回）", got, before)
	}
	execChild(m, d, "CHILD2.COM")
	r := execAfterResult{
		LargestFree: before,
		Arena:       append([]memBlock(nil), d.arena...),
		CF:          m.CPU.Flags&cpu.CF != 0,
		Mem:         append([]byte(nil), m.Mem[:machine.MemTop*16]...),
	}
	if r.CF {
		t.Fatalf("第二次 EXEC 失敗，AX=%04X Missing=%v", m.CPU.R[cpu.AX], d.Missing)
	}
	r.PSP2 = d.ExecLog[len(d.ExecLog)-1].PSP
	return r
}

func TestLoadStateChildExitThenExecSucceeds(t *testing.T) {
	ctl := runExecAfterChild(t, false)
	got := runExecAfterChild(t, true)
	if !reflect.DeepEqual(ctl.Arena, got.Arena) {
		t.Errorf("存讀後 arena 與對照組不同：\n存讀 %+v\n對照 %+v", got.Arena, ctl.Arena)
	}
	if ctl.PSP2 != got.PSP2 {
		t.Errorf("第二支子程式 PSP：存讀 %04X、對照 %04X", got.PSP2, ctl.PSP2)
	}
	if !bytes.Equal(ctl.Mem, got.Mem) {
		for i := range ctl.Mem {
			if ctl.Mem[i] != got.Mem[i] {
				t.Errorf("存讀後常規記憶體與對照組不同，第一個差異在 %05X：存讀 %02X、對照 %02X", i, got.Mem[i], ctl.Mem[i])
				break
			}
		}
	}
}

// 往返：已用塊擁有者、框架 arenaSeg、queuedSeg 存讀前後相同。
func TestLoadStateRebuildsArenaOwnership(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, m *machine.Machine, d *DOS) []uint16
		check func(t *testing.T, before ownership, d *DOS)
	}{
		{
			// 子行程還沒 AH=48h：Go 端程式塊擁有者是父 PSP，客體 MCB 是子 PSP。
			name: "arena 路徑、子行程未 sync",
			setup: func(t *testing.T, m *machine.Machine, d *DOS) []uint16 {
				heap := parentAlloc(t, m, d)
				writeChild(t, d, "CHILD.COM", allocThenExit)
				psp := mustExecChild(t, m, d, "CHILD.COM")
				b := blockAt(t, d, psp-1)
				if b.owner != machine.PSPSeg {
					t.Fatalf("前提：子程式塊 Go 端擁有者 %04X，預期父 PSP %04X", b.owner, machine.PSPSeg)
				}
				if g := m.ReadPhys16(uint32(psp-1)*16 + 1); g != psp {
					t.Fatalf("前提：子程式塊客體擁有者 %04X，預期子 PSP %04X（未 sync）", g, psp)
				}
				if blockAt(t, d, heap-1).owner != machine.PSPSeg {
					t.Fatal("前提：頂層塊擁有者不是 PSPSeg")
				}
				return []uint16{psp}
			},
			check: func(t *testing.T, before ownership, d *DOS) {
				psp := d.curPSP
				if got := blockAt(t, d, psp-1).owner; got != d.procStack[0].psp {
					t.Errorf("子程式塊擁有者＝%04X，預期父 PSP %04X", got, d.procStack[0].psp)
				}
				// 頂層程式配置的塊擁有者讀回仍為 PSPSeg。
				for _, b := range d.arena {
					if !b.free && b.seg != psp-1 && b.owner != machine.PSPSeg {
						t.Errorf("頂層塊 %04X 擁有者＝%04X，預期 PSPSeg", b.seg, b.owner)
					}
				}
			},
		},
		{
			name: "arena 路徑、子行程已 sync",
			setup: func(t *testing.T, m *machine.Machine, d *DOS) []uint16 {
				parentAlloc(t, m, d)
				writeChild(t, d, "CHILD.COM", allocThenExit)
				psp := mustExecChild(t, m, d, "CHILD.COM")
				stepToIP(t, m, allocDoneIP)
				if g := m.ReadPhys16(uint32(psp-1)*16 + 1); g != machine.PSPSeg {
					t.Fatalf("前提：sync 後客體擁有者 %04X，預期父 PSP", g)
				}
				return []uint16{psp}
			},
		},
		{
			// arena 還沒建時 EXEC（舊 freeSeg 路徑），子行程的 AH=48h 才建 arena。
			name: "nil 路徑",
			setup: func(t *testing.T, m *machine.Machine, d *DOS) []uint16 {
				writeChild(t, d, "CHILD.COM", allocThenExit)
				psp := mustExecChild(t, m, d, "CHILD.COM")
				if d.procStack[0].arenaSeg != 0 {
					t.Fatal("前提：nil 路徑的 arenaSeg 應為 0")
				}
				stepToIP(t, m, allocDoneIP)
				if d.arena == nil {
					t.Fatal("前提：子行程 AH=48h 之後 arena 應已建立")
				}
				return []uint16{psp}
			},
		},
		{
			name: "兩層巢狀子行程",
			setup: func(t *testing.T, m *machine.Machine, d *DOS) []uint16 {
				parentAlloc(t, m, d)
				writeChild(t, d, "CHILD.COM", allocThenExit)
				writeChild(t, d, "GRAND.COM", allocThenExit)
				p1 := mustExecChild(t, m, d, "CHILD.COM")
				stepToIP(t, m, allocDoneIP)
				p2 := mustExecChild(t, m, d, "GRAND.COM")
				return []uint16{p1, p2}
			},
			check: func(t *testing.T, before ownership, d *DOS) {
				if got := blockAt(t, d, d.curPSP-1).owner; got != d.procStack[1].psp {
					t.Errorf("孫行程程式塊擁有者＝%04X，預期 %04X", got, d.procStack[1].psp)
				}
			},
		},
		{
			name: "ExecEnvCopy 環境塊",
			setup: func(t *testing.T, m *machine.Machine, d *DOS) []uint16 {
				d.ExecEnvCopy = true
				parentAlloc(t, m, d)
				writeChild(t, d, "CHILD.COM", allocThenExit)
				psp := mustExecChild(t, m, d, "CHILD.COM")
				env := d.ExecLog[len(d.ExecLog)-1].EnvSeg
				if env == 0 || blockAt(t, d, env-1).owner != psp {
					t.Fatalf("前提：環境塊（%04X）擁有者應為子 PSP %04X", env, psp)
				}
				return []uint16{psp}
			},
			check: func(t *testing.T, before ownership, d *DOS) {
				env := d.M.Read16(uint32(d.curPSP)*16 + 0x2C)
				if got := blockAt(t, d, env-1).owner; got != d.curPSP {
					t.Errorf("環境塊擁有者＝%04X，預期子 PSP %04X", got, d.curPSP)
				}
			},
		},
		{
			// 疊底程式結束後由監督佇列推出 Q.COM，它的程式塊記在 queuedSeg。
			name: "佇列推出的底層行程",
			setup: func(t *testing.T, m *machine.Machine, d *DOS) []uint16 {
				parentAlloc(t, m, d)
				writeChild(t, d, "Q.COM", allocThenExit)
				d.Enqueue("Q.COM", "")
				call(m, d, 0x21, 0x4C00)
				if d.queuedSeg == 0 || len(d.Missing) != 0 {
					t.Fatalf("前提：佇列推出後 queuedSeg＝%04X，Missing=%v", d.queuedSeg, d.Missing)
				}
				// 先 sync：未 sync 時重建值為新 PSP，屬規格 §2 第 3 項的已知限制。
				stepToIP(t, m, allocDoneIP)
				return nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, d := newTest(t)
			psps := tc.setup(t, m, d)
			checkChildPSPs(t, d, psps)
			before := snapOwnership(d)
			_, d2 := saveLoad(t, m, d)
			checkChildPSPs(t, d2, psps)
			after := snapOwnership(d2)
			if !reflect.DeepEqual(before, after) {
				t.Errorf("存讀前後不同：\n前 %+v\n後 %+v", before, after)
			}
			if tc.check != nil {
				tc.check(t, before, d2)
			}
		})
	}
}
