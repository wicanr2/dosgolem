package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 這一份測試釘 EXEC／TSR／回傳碼（`docs/spec/008`／`009`）。
// 每一條的反面都不會報錯：殼鏈只會安靜地斷在某一跳。

// writeChild 在 Root 放一支最小的 .COM：做完自己的事之後用給定的
// 離開碼結束（或常駐）。
func writeChild(t *testing.T, d *DOS, name string, code []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(d.Root, name), code, 0o644); err != nil {
		t.Fatal(err)
	}
}

// execChild 從目前的 CPU 上下文 EXEC 一支程式。
func execChild(m *machine.Machine, d *DOS, name string) {
	m.CPU.Seg[cpu.DS] = 0x3000
	m.CPU.R[cpu.DX] = 0
	m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte(name), 0))
	// 參數區塊：環境段 0（繼承）、命令列尾 0:0（空尾）。
	m.CPU.Seg[cpu.ES] = 0x3000
	m.CPU.R[cpu.BX] = 0x80
	m.Write16(cpu.Addr(0x3000, 0x80), 0)
	m.Write16(cpu.Addr(0x3000, 0x82), 0)
	m.Write16(cpu.Addr(0x3000, 0x84), 0)
	call(m, d, 0x21, 0x4B00)
}

// TestExecRunsChildAndResumesParent 釘住 EXEC 的兩半：
// 子程式真的跑起來，而且它結束之後父程式從 int 21h 的下一道接著跑。
// 只載入不跳過去的話，殼的下一道指令（AH=4Dh）會在**父程式自己的
// 上下文裡**空轉——看起來像「EXEC 成功了」。
func TestExecRunsChildAndResumesParent(t *testing.T) {
	m, d := newTest(t)
	// 子程式：mov ax,4C2Ah; int 21h（離開碼 42）。
	writeChild(t, d, "CHILD.COM", []byte{0xB8, 0x2A, 0x4C, 0xCD, 0x21})

	// 父程式的哨兵上下文。
	m.CPU.R[cpu.SI] = 0x1111
	m.CPU.R[cpu.DI] = 0x2222
	m.CPU.R[cpu.BP] = 0x3333
	parentCS, parentIP := m.CPU.Seg[cpu.CS], m.CPU.IP

	execChild(m, d, "CHILD.COM")
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("EXEC 失敗，Missing=%v", d.Missing)
	}
	if len(d.stack) != 1 {
		t.Fatalf("EXEC 之後行程疊深度 %d，預期 1", len(d.stack))
	}

	// 跑子程式直到它結束、彈回父程式。
	for i := 0; i < 100 && len(d.stack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.stack) != 0 {
		t.Fatal("子程式沒有結束")
	}
	if m.CPU.R[cpu.SI] != 0x1111 || m.CPU.R[cpu.DI] != 0x2222 || m.CPU.R[cpu.BP] != 0x3333 {
		t.Errorf("父程式暫存器沒復原：SI=%04X DI=%04X BP=%04X",
			m.CPU.R[cpu.SI], m.CPU.R[cpu.DI], m.CPU.R[cpu.BP])
	}
	if m.CPU.Seg[cpu.CS] != parentCS || m.CPU.IP != parentIP {
		t.Errorf("父程式沒回到原處：%04X:%04X（原 %04X:%04X）",
			m.CPU.Seg[cpu.CS], m.CPU.IP, parentCS, parentIP)
	}
	if d.Exited {
		t.Error("子程式結束不該讓整台機器停下來")
	}

	// AH=4Dh 拿到離開碼。
	call(m, d, 0x21, 0x4D00)
	if ax := m.CPU.R[cpu.AX]; ax != 0x002A {
		t.Errorf("AH=4Dh 回 AX=%04X，預期 002A", ax)
	}
	// 可重複讀（不清掉）。
	call(m, d, 0x21, 0x4D00)
	if ax := m.CPU.R[cpu.AX]; ax != 0x002A {
		t.Errorf("AH=4Dh 第二次回 AX=%04X——清了會給出假的 0", ax)
	}

	// 非 TSR 子程式的記憶體要 LIFO 回收。
	if d.freeSeg != 0x2000 {
		t.Errorf("子程式結束後 freeSeg=%04X，預期回 2000", d.freeSeg)
	}
	if len(d.ExecLog) != 1 || d.ExecLog[0].Exit != 42 {
		t.Errorf("ExecLog=%+v", d.ExecLog)
	}
}

// TestTSRKeepsMemory 釘住 AH=31h：常駐區留在記憶體裡，
// 下一支程式要落在它之上，不能覆蓋。
func TestTSRKeepsMemory(t *testing.T) {
	m, d := newTest(t)
	// 子程式：mov dx,40h; mov ax,3107h; int 21h（常駐 0x40 段，碼 7）。
	// keep 要比映像本身大才有「多留」可驗；反過來（DX 小於已佔用）時
	// bump 配置器只能往前推不能往回收（`docs/spec/008` §2.1）。
	writeChild(t, d, "TSR.COM", []byte{0xBA, 0x40, 0x00, 0xB8, 0x07, 0x31, 0xCD, 0x21})

	execChild(m, d, "TSR.COM")
	childPSP := d.curPSP
	for i := 0; i < 100 && len(d.stack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if d.freeSeg != childPSP+0x40 {
		t.Errorf("TSR 之後 freeSeg=%04X，預期 %04X（PSP+40h）", d.freeSeg, childPSP+0x40)
	}
	call(m, d, 0x21, 0x4D00)
	if ax := m.CPU.R[cpu.AX]; ax != 0x0007 {
		t.Errorf("AH=4Dh 回 AX=%04X，預期 0007", ax)
	}
	if !d.ExecLog[0].TSR || d.ExecLog[0].Keep != 0x40 {
		t.Errorf("ExecLog=%+v", d.ExecLog[0])
	}
}

// TestSupervisorQueueRunsNextProgram 釘住監督佇列（`009` §4）：
// 疊底程式結束後，佇列裡的下一支要接著跑，而不是整台停掉。
// 沒有這條，「DOSJP 常駐 → 再跑 Genpei.com」這條鏈不存在。
func TestSupervisorQueueRunsNextProgram(t *testing.T) {
	m, d := newTest(t)
	writeChild(t, d, "NEXT.COM", []byte{0xB8, 0x05, 0x4C, 0xCD, 0x21})
	d.Enqueue("NEXT.COM", "")

	// 疊底程式（行程疊是空的）以碼 0 結束。
	call(m, d, 0x21, 0x4C00)
	if d.Exited {
		t.Fatal("佇列裡還有程式，不該停")
	}
	for i := 0; i < 100 && !d.Exited; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if !d.Exited || d.ExitCode != 5 {
		t.Errorf("Exited=%v ExitCode=%d——佇列推出來的程式沒跑到", d.Exited, d.ExitCode)
	}
}

// TestExecMissingFileFailsInPlace 釘住「找不到檔不動行程疊」。
// 殼連跳五支程式，其中一支缺席時必須回到殼的錯誤路徑，
// 而不是把殼自己也吃掉。
func TestExecMissingFileFailsInPlace(t *testing.T) {
	m, d := newTest(t)
	execChild(m, d, "NOPE.EXE")
	if m.CPU.Flags&cpu.CF == 0 {
		t.Fatal("EXEC 不存在的檔竟然成功")
	}
	if m.CPU.R[cpu.AX] != 2 {
		t.Errorf("AX=%d，預期 2（File not found）", m.CPU.R[cpu.AX])
	}
	if len(d.stack) != 0 {
		t.Error("失敗的 EXEC 動了行程疊")
	}
}
