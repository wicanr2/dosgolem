package dos

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// EXEC 子行程環境區塊的契約測試，規格 `docs/spec/430-exec-child-environment` §5。
//
// 期望值照真 DOS 的版面自己列：字串區、word 0001、子行程完整路徑。

// exitCOM 是 `mov ax,4C00h; int 21h`。
var exitCOM = []byte{0xB8, 0x00, 0x4C, 0xCD, 0x21}

// setParentEnv 把父行程（PSPSeg）的環境換成 blk。
func setParentEnv(m *machine.Machine, blk []byte) {
	m.WriteBytes(uint32(machine.EnvSeg)*16, blk)
	m.Write16(uint32(machine.PSPSeg)*16+0x2C, machine.EnvSeg)
}

// childEnvBytes 讀目前行程（剛 EXEC 進去的子行程）的環境區塊前 n bytes。
func childEnvBytes(m *machine.Machine, d *DOS, n int) (uint16, []byte) {
	seg := m.Read16(uint32(d.curPSP)*16 + 0x2C)
	out := make([]byte, n)
	for i := range out {
		out[i] = m.Read8(uint32(seg)*16 + uint32(i))
	}
	return seg, out
}

func runToParent(t *testing.T, m *machine.Machine, d *DOS) {
	t.Helper()
	for i := 0; i < 100 && len(d.procStack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != 0 {
		t.Fatal("子程式沒有結束")
	}
}

var parentEnv = []byte("A=1\x00BB=22\x00\x00\x01\x00C:\\PARENT.EXE\x00")

func TestExecChildGetsOwnEnvironmentWithItsPath(t *testing.T) {
	m, d := newTest(t)
	setParentEnv(m, parentEnv)
	d.Dir = "GAME" // 路徑照 AH=60h 的規則補上目前目錄
	writeChild(t, d, "KID.COM", exitCOM)
	execChild(m, d, "kid.com")

	want := []byte("A=1\x00BB=22\x00\x00\x01\x00C:\\GAME\\KID.COM\x00")
	seg, got := childEnvBytes(m, d, len(want))
	if seg == machine.EnvSeg {
		t.Fatal("子行程的 PSP+2Ch 還是父行程的環境段")
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("子行程環境 = %q\n預期          %q", got, want)
	}
}

func TestExecChildEnvironmentFromGivenSegment(t *testing.T) {
	m, d := newTest(t)
	setParentEnv(m, parentEnv)
	d.Dir = ""
	writeChild(t, d, "KID.COM", exitCOM)
	// 呼叫端自備的環境放在 4000:0000，參數區塊 +0 指過去。
	m.WriteBytes(0x40000, []byte("X=9\x00\x00"))
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x3000, 0
	m.WriteBytes(cpu.Addr(0x3000, 0), []byte("KID.COM\x00"))
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0x3000, 0x80
	m.Write16(cpu.Addr(0x3000, 0x80), 0x4000)
	m.Write16(cpu.Addr(0x3000, 0x82), 0)
	m.Write16(cpu.Addr(0x3000, 0x84), 0)
	call(m, d, 0x21, 0x4B00)

	want := []byte("X=9\x00\x00\x01\x00C:\\KID.COM\x00")
	seg, got := childEnvBytes(m, d, len(want))
	if seg == 0x4000 {
		t.Fatal("子行程直接用了呼叫端的段，沒有複製")
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("子行程環境 = %q\n預期          %q", got, want)
	}
}

func TestExecChildEnvironmentOwnedAndReclaimed(t *testing.T) {
	m, d := newTest(t)
	setParentEnv(m, parentEnv)
	d.initArena()
	writeChild(t, d, "KID.COM", exitCOM)
	execChild(m, d, "KID.COM")

	seg, _ := childEnvBytes(m, d, 0)
	child := d.curPSP
	if seg >= child {
		t.Fatalf("環境段 %04X 應在子行程 PSP %04X 之前", seg, child)
	}
	if owner := m.Read16(uint32(seg-1)*16 + 1); owner != child {
		t.Fatalf("環境區塊 MCB 擁有者 = %04X，預期子行程 %04X", owner, child)
	}
	runToParent(t, m, d)
	for _, b := range d.arena {
		if b.seg+1 == seg && !b.free {
			t.Fatal("子行程結束後環境區塊沒有回到自由")
		}
	}
}

func TestExecChildEnvironmentFromEmptyEnvShape(t *testing.T) {
	m, d := newTest(t)
	// `-empty-env` 的形狀：字串區只有一個 00，後面直接是 01 00 與路徑。
	setParentEnv(m, []byte("\x00\x01\x00C:\\PARENT.EXE\x00"))
	d.Dir = ""
	writeChild(t, d, "KID.COM", exitCOM)
	execChild(m, d, "KID.COM")

	want := []byte("\x00\x01\x00C:\\KID.COM\x00")
	_, got := childEnvBytes(m, d, len(want))
	if !bytes.Equal(got, want) {
		t.Fatalf("子行程環境 = %q\n預期          %q", got, want)
	}
}
