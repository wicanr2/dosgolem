package dos

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 這一檔是 `docs/spec/439-exec-child-environment` §3 第 2 項：`ExecEnvCopy` 開啟時，
// EXEC 替子行程建一塊新環境（父環境字串、雙 0、`01 00`、子程式正規化路徑、0）。

// 子程式（.COM 內容，檔名用 .EXE：載入器看檔頭不看副檔名，`docs/spec/009` §2.5）。
var (
	childExit4C  = []byte{0xB8, 0x00, 0x4C, 0xCD, 0x21}                        // mov ax,4C00h; int 21h
	childInt20   = []byte{0xCD, 0x20}                                          // int 20h
	childTSR31   = []byte{0xB8, 0x00, 0x31, 0xBA, 0x10, 0x00, 0xCD, 0x21}      // mov ax,3100h; mov dx,10h; int 21h
	childInt27   = []byte{0xBA, 0x00, 0x01, 0xCD, 0x27}                        // mov dx,100h; int 27h
	parentEnv    = []byte("A=1\x00B=2\x00\x00")                                // 兩條字串＋結尾
	childPath    = `C:\CHILD.EXE`                                              // d.Dir＝"" 時 canonical("CHILD.EXE")
	parentEnvAt  = uint16(0x1800)                                              // freeSeg（2000h）之下，不在 arena 內
	parentPSP2C  = uint32(machine.PSPSeg)*16 + 0x2C                            // 父行程 PSP:2Ch
	childEnvTail = append(append([]byte{0x01, 0x00}, []byte(childPath)...), 0) // word 0001h＋路徑＋0
)

// newEnvTest 是開了 ExecEnvCopy、目前目錄在根、父環境在 1800h 的測試機。
func newEnvTest(t *testing.T) (*machine.Machine, *DOS) {
	t.Helper()
	m, d := newTest(t)
	d.ExecEnvCopy = true
	d.Dir = ""
	m.WriteBytes(uint32(parentEnvAt)*16, parentEnv)
	m.Write16(parentPSP2C, parentEnvAt)
	return m, d
}

// execChildEnv 同 execChild，但參數區塊的環境段由呼叫端給。
func execChildEnv(m *machine.Machine, d *DOS, name string, envSeg uint16) {
	m.CPU.Seg[cpu.DS] = 0x3000
	m.CPU.R[cpu.DX] = 0
	m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte(name), 0))
	m.CPU.Seg[cpu.ES] = 0x3000
	m.CPU.R[cpu.BX] = 0x80
	m.Write16(cpu.Addr(0x3000, 0x80), envSeg)
	m.Write16(cpu.Addr(0x3000, 0x82), 0)
	m.Write16(cpu.Addr(0x3000, 0x84), 0)
	call(m, d, 0x21, 0x4B00)
}

// runChild 跑到行程疊彈回。
func runChild(t *testing.T, m *machine.Machine, d *DOS) {
	t.Helper()
	for i := 0; i < 200 && len(d.procStack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != 0 {
		t.Fatal("子程式沒有結束")
	}
}

// mustExec 執行並確認成功，回子 PSP 與它的環境段。
func mustExec(t *testing.T, m *machine.Machine, d *DOS, envSeg uint16) (psp, env uint16) {
	t.Helper()
	execChildEnv(m, d, "CHILD.EXE", envSeg)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("EXEC 失敗：AX=%04X Missing=%v", m.CPU.R[cpu.AX], d.Missing)
	}
	rec := d.ExecLog[len(d.ExecLog)-1]
	psp = rec.PSP
	env = m.Read16(uint32(psp)*16 + 0x2C)
	if rec.EnvSeg != env {
		t.Errorf("ExecRecord.EnvSeg＝%04X，子 PSP:2Ch＝%04X", rec.EnvSeg, env)
	}
	return psp, env
}

// checkEnv 驗環境塊內容與它的客體 MCB（擁有者、size）。
func checkEnv(t *testing.T, m *machine.Machine, env, psp uint16, body []byte, size uint16) {
	t.Helper()
	want := append(append([]byte(nil), body...), childEnvTail...)
	got := make([]byte, len(want))
	for i := range got {
		got[i] = m.Read8(uint32(env)*16 + uint32(i))
	}
	if !bytes.Equal(got, want) {
		t.Errorf("環境內容＝% X\n預期       ＝% X", got, want)
	}
	mcb := uint32(env-1) * 16
	if sig := m.Read8(mcb); sig != 'M' && sig != 'Z' {
		t.Errorf("環境 MCB 簽章＝%02X", sig)
	}
	if o := m.Read16(mcb + 1); o != psp {
		t.Errorf("環境 MCB 擁有者＝%04X，預期子 PSP %04X", o, psp)
	}
	if s := m.Read16(mcb + 3); s != size {
		t.Errorf("環境 MCB size＝%04X，預期 %04X", s, size)
	}
}

// envParags 是 §2.2 第 3 項的大小公式。
func envParags(bodyLen int) uint16 {
	keep := 2 + len(childPath) + 1
	if keep < 83 {
		keep = 83
	}
	return uint16((bodyLen + keep + 15) / 16)
}

// arenaBlock 回 MCB 段為 seg 的 arena 項目。
func arenaBlock(t *testing.T, d *DOS, seg uint16) memBlock {
	t.Helper()
	for _, b := range d.arena {
		if b.seg == seg {
			return b
		}
	}
	t.Fatalf("arena 裡沒有 MCB 段 %04X 的塊：%+v", seg, d.arena)
	return memBlock{}
}

// arenaShape 是 arena 的段、大小、自由與否（不含擁有者：自由塊的擁有者沒有意義）。
func arenaShape(d *DOS) string {
	var sb strings.Builder
	for _, b := range d.arena {
		fmt.Fprintf(&sb, "%04X:%04X:%v ", b.seg, b.size, b.free)
	}
	return sb.String()
}

// arena 為 nil：環境 MCB 在 freeSeg，子 PSP 在環境塊之後，結束時 freeSeg 退回。
func TestExecEnvInheritNilArena(t *testing.T) {
	m, d := newEnvTest(t)
	writeChild(t, d, "CHILD.EXE", childExit4C)
	free0 := d.freeSeg
	psp, env := mustExec(t, m, d, 0)
	if d.arena != nil {
		t.Fatal("arena 為 nil 的路徑不應呼叫 initArena")
	}
	n := envParags(len(parentEnv))
	if env != free0+1 {
		t.Errorf("環境段＝%04X，預期 freeSeg+1＝%04X", env, free0+1)
	}
	if want := free0 + 1 + n + 1; psp != want {
		t.Errorf("子 PSP＝%04X，預期 freeSeg+1+envParags+1＝%04X", psp, want)
	}
	if psp <= env {
		t.Errorf("子 PSP %04X 應在環境塊 %04X 之後", psp, env)
	}
	checkEnv(t, m, env, psp, parentEnv, n)
	runChild(t, m, d)
	if d.freeSeg != free0 {
		t.Errorf("結束後 freeSeg＝%04X，預期退回 %04X", d.freeSeg, free0)
	}
}

// arena 非 nil：環境塊在 arena 裡、擁有者是子 PSP，預設 first fit 下在程式塊之前。
func TestExecEnvInheritArena(t *testing.T) {
	m, d := newEnvTest(t)
	writeChild(t, d, "CHILD.EXE", childExit4C)
	d.initArena()
	largest0 := d.largestFree()
	psp, env := mustExec(t, m, d, 0)
	b := arenaBlock(t, d, env-1)
	if b.free || b.owner != psp {
		t.Errorf("環境塊 arena 項目 free=%v owner=%04X，預期已配、擁有者子 PSP %04X", b.free, b.owner, psp)
	}
	if psp <= env {
		t.Errorf("子 PSP %04X 應在環境塊 %04X 之後（first fit）", psp, env)
	}
	checkEnv(t, m, env, psp, parentEnv, b.size)
	if b.size != envParags(len(parentEnv)) {
		t.Errorf("環境塊 %04X 段，預期 %04X", b.size, envParags(len(parentEnv)))
	}
	runChild(t, m, d)
	if got := d.largestFree(); got != largest0 {
		t.Errorf("結束後 largestFree＝%04X，預期回到 %04X", got, largest0)
	}
	for _, b := range d.arena {
		if !b.free {
			t.Errorf("結束後還有已配置的塊：%+v", b)
		}
	}
}

// 參數區塊給非 0 環境段：從該段掃描與複製，不看父 PSP:2Ch。
func TestExecEnvFromParamBlock(t *testing.T) {
	m, d := newEnvTest(t)
	writeChild(t, d, "CHILD.EXE", childExit4C)
	own := []byte("X=9\x00\x00")
	m.WriteBytes(0x1900*16, own)
	psp, env := mustExec(t, m, d, 0x1900)
	checkEnv(t, m, env, psp, own, envParags(len(own)))
}

// 父 PSP:2Ch 為 0（沒有環境）：只寫一個 0 byte。
func TestExecEnvParentHasNone(t *testing.T) {
	m, d := newEnvTest(t)
	writeChild(t, d, "CHILD.EXE", childExit4C)
	m.Write16(parentPSP2C, 0)
	psp, env := mustExec(t, m, d, 0)
	checkEnv(t, m, env, psp, []byte{0}, envParags(1))
}

// 洞的情境：環境塊落在下面的小洞，程式塊落在活塊之上；兩條結束路徑都兩塊一起釋放，
// largestFree 回到 EXEC 前。
func TestExecEnvHoleReleasedOnExit(t *testing.T) {
	for _, tc := range []struct {
		name string
		code []byte
	}{{"AH=4Ch", childExit4C}, {"int 20h", childInt20}} {
		t.Run(tc.name, func(t *testing.T) {
			m, d := newEnvTest(t)
			writeChild(t, d, "CHILD.EXE", tc.code)
			n := envParags(len(parentEnv))
			// 配 A（放得下環境、放不下程式）、配 B、還 A。
			m.CPU.R[cpu.BX] = n + 2
			call(m, d, 0x21, 0x4800)
			heapA := m.CPU.R[cpu.AX]
			m.CPU.R[cpu.BX] = 0x100
			call(m, d, 0x21, 0x4800)
			heapB := m.CPU.R[cpu.AX]
			m.CPU.Seg[cpu.ES] = heapA
			call(m, d, 0x21, 0x4900)
			if m.CPU.Flags&cpu.CF != 0 {
				t.Fatal("還 A 失敗")
			}
			largest0 := d.largestFree()

			psp, env := mustExec(t, m, d, 0)
			if env != heapA {
				t.Errorf("環境段＝%04X，預期落在洞 A %04X", env, heapA)
			}
			if psp <= heapB {
				t.Errorf("子 PSP＝%04X，預期在活塊 B %04X 之上", psp, heapB)
			}
			checkEnv(t, m, env, psp, parentEnv, arenaBlock(t, d, env-1).size)
			runChild(t, m, d)
			if b := arenaBlock(t, d, heapA-1); !b.free {
				t.Errorf("結束後環境塊沒有釋放：%+v", b)
			}
			if b := arenaBlock(t, d, heapB-1); b.free {
				t.Errorf("活塊 B 被釋放了：%+v", b)
			}
			if got := d.largestFree(); got != largest0 {
				t.Errorf("結束後 largestFree＝%04X，預期回到 %04X", got, largest0)
			}
		})
	}
}

// 駐留（AH=31h、int 27h）：terminate 不呼叫 releaseOwnedBy，環境塊仍屬子 PSP。
func TestExecEnvKeptOnTSR(t *testing.T) {
	for _, tc := range []struct {
		name string
		code []byte
	}{{"AH=31h", childTSR31}, {"int 27h", childInt27}} {
		t.Run(tc.name, func(t *testing.T) {
			m, d := newEnvTest(t)
			writeChild(t, d, "CHILD.EXE", tc.code)
			d.initArena()
			psp, env := mustExec(t, m, d, 0)
			runChild(t, m, d)
			if !d.ExecLog[len(d.ExecLog)-1].TSR {
				t.Fatal("子程式沒有走駐留結束")
			}
			b := arenaBlock(t, d, env-1)
			if b.free || b.owner != psp {
				t.Errorf("駐留後環境塊 free=%v owner=%04X，預期仍屬子 PSP %04X", b.free, b.owner, psp)
			}
		})
	}
}

// 正規化後的路徑超過 127 字元：錯誤 3，什麼都沒配。
func TestExecEnvPathTooLong(t *testing.T) {
	m, d := newEnvTest(t)
	writeChild(t, d, "CHILD.EXE", childExit4C)
	d.Dir = strings.Repeat("D", 120)
	free0 := d.freeSeg
	execChildEnv(m, d, "CHILD.EXE", 0)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 3 {
		t.Fatalf("CF=%v AX=%04X，預期 CF 與錯誤 3", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
	if d.arena != nil || d.freeSeg != free0 || len(d.procStack) != 0 || len(d.ExecLog) != 0 {
		t.Errorf("失敗後狀態有變：arena=%v freeSeg=%04X 疊=%d 紀錄=%d", d.arena, d.freeSeg, len(d.procStack), len(d.ExecLog))
	}
}

// 環境掃描到上限仍沒有雙 0：錯誤 10，什麼都沒配。
func TestExecEnvScanLimit(t *testing.T) {
	m, d := newEnvTest(t)
	writeChild(t, d, "CHILD.EXE", childExit4C)
	m.WriteBytes(uint32(parentEnvAt)*16, bytes.Repeat([]byte{'A'}, execEnvScanMax+2))
	d.initArena()
	shape0 := arenaShape(d)
	execChildEnv(m, d, "CHILD.EXE", 0)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 10 {
		t.Fatalf("CF=%v AX=%04X，預期 CF 與錯誤 10", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
	if got := arenaShape(d); got != shape0 || len(d.procStack) != 0 {
		t.Errorf("失敗後 arena 有變：\n%s\n%s", shape0, got)
	}
}

// 環境塊配不到：錯誤 8（arena 與 nil 兩條路）。
func TestExecEnvNoRoomForEnv(t *testing.T) {
	t.Run("arena", func(t *testing.T) {
		m, d := newEnvTest(t)
		writeChild(t, d, "CHILD.EXE", childExit4C)
		d.initArena()
		m.CPU.R[cpu.BX] = d.largestFree() - 3 // 剩一塊 2 段的自由區
		call(m, d, 0x21, 0x4800)
		shape0 := arenaShape(d)
		execChildEnv(m, d, "CHILD.EXE", 0)
		if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 8 {
			t.Fatalf("CF=%v AX=%04X，預期錯誤 8", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
		}
		if got := arenaShape(d); got != shape0 {
			t.Errorf("失敗後 arena 有變：\n%s\n%s", shape0, got)
		}
	})
	t.Run("nil", func(t *testing.T) {
		m, d := newEnvTest(t)
		writeChild(t, d, "CHILD.EXE", childExit4C)
		d.freeSeg = machine.MemTop - envParags(len(parentEnv)) // 放不下 MCB＋環境
		free0 := d.freeSeg
		execChildEnv(m, d, "CHILD.EXE", 0)
		if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 8 {
			t.Fatalf("CF=%v AX=%04X，預期錯誤 8", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
		}
		if d.arena != nil || d.freeSeg != free0 || len(d.procStack) != 0 {
			t.Errorf("失敗後狀態有變")
		}
	})
}

// 環境塊配到了、程式塊配不到：錯誤 8，環境塊已釋放。
func TestExecEnvNoRoomForProgram(t *testing.T) {
	t.Run("arena", func(t *testing.T) {
		m, d := newEnvTest(t)
		writeChild(t, d, "CHILD.EXE", childExit4C)
		d.initArena()
		m.CPU.R[cpu.BX] = d.largestFree() - 21 // 剩一塊 20 段：環境（＋MCB）放得下，程式放不下
		call(m, d, 0x21, 0x4800)
		shape0 := arenaShape(d)
		largest0 := d.largestFree()
		execChildEnv(m, d, "CHILD.EXE", 0)
		if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 8 {
			t.Fatalf("CF=%v AX=%04X，預期錯誤 8", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
		}
		if got := arenaShape(d); got != shape0 {
			t.Errorf("環境塊沒有釋放：\n%s\n%s", shape0, got)
		}
		if got := d.largestFree(); got != largest0 || m.CPU.R[cpu.BX] != largest0 {
			t.Errorf("largestFree＝%04X BX＝%04X，預期 %04X", got, m.CPU.R[cpu.BX], largest0)
		}
	})
	t.Run("nil", func(t *testing.T) {
		m, d := newEnvTest(t)
		writeChild(t, d, "CHILD.EXE", childExit4C)
		d.freeSeg = machine.MemTop - 20
		free0 := d.freeSeg
		execChildEnv(m, d, "CHILD.EXE", 0)
		if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 8 {
			t.Fatalf("CF=%v AX=%04X，預期錯誤 8", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
		}
		if d.arena != nil || d.freeSeg != free0 || len(d.procStack) != 0 {
			t.Errorf("失敗後狀態有變")
		}
	})
}

// LoadProgramAt 失敗（空的 .COM）：環境塊與程式塊都釋放，客體記憶體不留環境內容。
func TestExecEnvLoadFailureReleasesBoth(t *testing.T) {
	m, d := newEnvTest(t)
	writeChild(t, d, "CHILD.EXE", nil)
	d.initArena()
	shape0 := arenaShape(d)
	envAt := uint32(d.arena[0].seg+1) * 16 // first fit：環境塊會落在這裡
	execChildEnv(m, d, "CHILD.EXE", 0)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 8 {
		t.Fatalf("CF=%v AX=%04X，預期錯誤 8", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
	if got := arenaShape(d); got != shape0 {
		t.Errorf("兩塊沒有都釋放：\n%s\n%s", shape0, got)
	}
	if m.Read8(envAt) == 'A' {
		t.Error("失敗時客體記憶體寫進了環境內容")
	}
}

// AL=03h（overlay）不建環境。
func TestExecEnvOverlayHasNone(t *testing.T) {
	m, d := newEnvTest(t)
	writeChild(t, d, "OVL.BIN", overlayMZ(t))
	d.initArena()
	shape0 := arenaShape(d)
	m.CPU.Seg[cpu.DS] = 0x3000
	m.CPU.R[cpu.DX] = 0
	m.WriteBytes(cpu.Addr(0x3000, 0), []byte("OVL.BIN\x00"))
	m.CPU.Seg[cpu.ES] = 0x3000
	m.CPU.R[cpu.BX] = 0x80
	m.Write16(cpu.Addr(0x3000, 0x80), 0x5000)
	m.Write16(cpu.Addr(0x3000, 0x82), 0x5000)
	call(m, d, 0x21, 0x4B03)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("overlay 失敗：AX=%04X", m.CPU.R[cpu.AX])
	}
	if got := arenaShape(d); got != shape0 {
		t.Errorf("AL=03h 動了 arena：\n%s\n%s", shape0, got)
	}
	if rec := d.ExecLog[len(d.ExecLog)-1]; rec.EnvSeg != 0 {
		t.Errorf("AL=03h 的 EnvSeg＝%04X，預期 0", rec.EnvSeg)
	}
}

// 載入器建的環境（`machine/loader.go`）並用 ExecEnvCopy 的實測行為。
//
// 預設環境 `COMSPEC=C:\COMMAND.COM 00 00 01 00 C:\PROG.EXE 00`：在 COMSPEC 之後的雙 0 停下，
// 子環境是 COMSPEC 字串、雙 0、`01 00`、子程式路徑。
//
// `-empty-env` 的環境 `00 01 00 C:\PROG.EXE 00`：偏移 0 的 word 是 0100h 不是雙 0，
// 掃描越過父程式路徑，停在它結尾的 0 與後面那個 0（本測試的機器上那裡是 0）。
// 子環境因此是 `00 01 00 C:\PROG.EXE 00 00 01 00 C:\CHILD.EXE 00`：父程式路徑被當成環境的一部分抄過去。
// 這是 §2.2 掃描規則的實測結果，不保證與真 DOS 一致（`docs/spec/439` §2.2 末段）。
func TestExecEnvWithLoaderEnv(t *testing.T) {
	for _, tc := range []struct {
		name  string
		empty bool
		body  []byte
	}{
		{"預設環境", false, []byte("COMSPEC=C:\\COMMAND.COM\x00\x00")},
		{"-empty-env", true, []byte("\x00\x01\x00C:\\PROG.EXE\x00\x00")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := machine.New()
			m.EmptyEnv = tc.empty
			if err := m.LoadCOM([]byte{0x90}); err != nil {
				t.Fatal(err)
			}
			d := New(m, t.TempDir())
			d.Install()
			d.ExecEnvCopy = true
			d.Dir = ""
			writeChild(t, d, "CHILD.EXE", childExit4C)
			if got := m.Read16(parentPSP2C); got != machine.EnvSeg {
				t.Fatalf("父 PSP:2Ch＝%04X，預期載入器的 EnvSeg %04X", got, uint16(machine.EnvSeg))
			}
			psp, env := mustExec(t, m, d, 0)
			checkEnv(t, m, env, psp, tc.body, envParags(len(tc.body)))
			runChild(t, m, d)
		})
	}
}

// 選項關閉：子 PSP:2Ch 仍是父行程的環境段（舊行為），不建新塊。
func TestExecEnvOffSharesParentEnv(t *testing.T) {
	m, d := newEnvTest(t)
	d.ExecEnvCopy = false
	writeChild(t, d, "CHILD.EXE", childExit4C)
	free0 := d.freeSeg
	execChildEnv(m, d, "CHILD.EXE", 0)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("EXEC 失敗：AX=%04X", m.CPU.R[cpu.AX])
	}
	rec := d.ExecLog[len(d.ExecLog)-1]
	if got := m.Read16(uint32(rec.PSP)*16 + 0x2C); got != parentEnvAt {
		t.Errorf("子 PSP:2Ch＝%04X，預期父環境段 %04X", got, parentEnvAt)
	}
	if rec.EnvSeg != 0 || rec.PSP != free0+1 {
		t.Errorf("EnvSeg＝%04X PSP＝%04X，預期 0 與 %04X", rec.EnvSeg, rec.PSP, free0+1)
	}
}

// 掃描上限的邊界：雙 0 在偏移 execEnvScanMax−1 還找得到，在 execEnvScanMax 就回錯誤 10
// （DOSBox-X `MakeEnv` 的 `envsize >= MAXENV-keepfree` 先於讀取判斷）。
func TestExecEnvScanLimitBoundary(t *testing.T) {
	for _, tc := range []struct {
		at   int
		fail bool
	}{{execEnvScanMax - 1, false}, {execEnvScanMax, true}} {
		m, d := newEnvTest(t)
		writeChild(t, d, "CHILD.EXE", childExit4C)
		blob := append(bytes.Repeat([]byte{'A'}, tc.at), 0, 0)
		m.WriteBytes(uint32(parentEnvAt)*16, blob)
		d.initArena()
		execChildEnv(m, d, "CHILD.EXE", 0)
		failed := m.CPU.Flags&cpu.CF != 0
		if failed != tc.fail || (failed && m.CPU.R[cpu.AX] != 10) {
			t.Errorf("雙 0 在偏移 %d：CF=%v AX=%04X，預期失敗＝%v", tc.at, failed, m.CPU.R[cpu.AX], tc.fail)
		}
		if !failed {
			env := d.ExecLog[len(d.ExecLog)-1].EnvSeg
			if got := arenaBlock(t, d, env-1).size; got != envParags(tc.at+2) {
				t.Errorf("環境塊 %04X 段，預期 %04X", got, envParags(tc.at+2))
			}
		}
	}
}
