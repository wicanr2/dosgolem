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
	if len(d.procStack) != 1 {
		t.Fatalf("EXEC 之後行程疊深度 %d，預期 1", len(d.procStack))
	}

	// 跑子程式直到它結束、彈回父程式。
	for i := 0; i < 100 && len(d.procStack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != 0 {
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
	for i := 0; i < 100 && len(d.procStack) > 0; i++ {
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
	if len(d.procStack) != 0 {
		t.Error("失敗的 EXEC 動了行程疊")
	}
}

// TestTrampolineCarryReachesStackedFlags 釘住 fixStackedCF：
// 走 trampoline 進來的服務，CF 要寫進堆疊上的旗標框，否則 IRET 把它蓋掉。
// 症狀是 TSR 落腳後 EXEC 一律「失敗」，殼印 cannot execute 然後離開。
func TestTrampolineCarryReachesStackedFlags(t *testing.T) {
	m, d := newTest(t)
	// 假的中斷框：SS:SP → IP／CS／FLAGS（CF 立著）。
	m.CPU.Seg[cpu.SS] = 0x9000
	m.CPU.R[cpu.SP] = 0x100
	m.Write16(cpu.Addr(0x9000, 0x100), 0)
	m.Write16(cpu.Addr(0x9000, 0x102), 0)
	m.Write16(cpu.Addr(0x9000, 0x104), cpu.CF)

	// AH=19h（取目前磁碟）是清 CF 的服務。
	call(m, d, 0xF2, 0x1900)
	if w := m.Read16(cpu.Addr(0x9000, 0x104)); w&cpu.CF != 0 {
		t.Errorf("堆疊框的 FLAGS=%04X，CF 還立著——IRET 會把假失敗彈回去", w)
	}

	// 反向：服務設 CF（開不存在的檔）要進堆疊框。
	m.CPU.Seg[cpu.DS] = 0x3000
	m.CPU.R[cpu.DX] = 0
	m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte("NOPE.BIN"), 0))
	call(m, d, 0xF2, 0x3D00)
	if w := m.Read16(cpu.Addr(0x9000, 0x104)); w&cpu.CF == 0 {
		t.Error("服務設了 CF，但堆疊框沒有")
	}
}

// TestChildSetBlockMovesFreeSeg 釘住「子行程撐大自己的區塊之後，
// AH=48h 不能再把那塊記憶體配出去」。
//
// 反面沒有任何錯誤：配到的緩衝區落在子行程的堆疊上，讀個檔就把返回位址
// 換成檔案內容，retf 到一個看起來很像程式碼的地方，然後在資料裡一路
// 執行下去。（源平合戰的 OPEN.EXE 就是這樣飛掉的。）
func TestChildSetBlockMovesFreeSeg(t *testing.T) {
	m, d := newTest(t)
	// 子程式：AH=4Ah 把自己的區塊撐到 0x2000 段，然後 AH=48h 要 0x100 段，
	// 把結果留在 BX，最後結束。
	writeChild(t, d, "GROW.COM", []byte{
		0x8C, 0xCB, // mov bx,cs      （.COM 的 CS ＝ PSP）
		0x8E, 0xC3, // mov es,bx
		0xBB, 0x00, 0x20, // mov bx,2000h
		0xB4, 0x4A, 0xCD, 0x21, // mov ah,4Ah; int 21h
		0xBB, 0x00, 0x01, // mov bx,0100h
		0xB4, 0x48, 0xCD, 0x21, // mov ah,48h; int 21h
		0xA3, 0x00, 0x02, // mov [0200h],ax   （把配到的段存起來）
		0xB8, 0x00, 0x4C, 0xCD, 0x21, // mov ax,4C00h; int 21h
	})
	execChild(m, d, "GROW.COM")
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("EXEC 失敗，Missing=%v", d.Missing)
	}
	psp := d.ExecLog[len(d.ExecLog)-1].PSP
	for i := 0; i < 200 && len(d.procStack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != 0 {
		t.Fatal("子程式沒有結束")
	}

	got := m.Read16(cpu.Addr(psp, 0x200))
	if got == 0 {
		t.Fatal("子程式沒有配到記憶體")
	}
	// 子行程擁有 psp..psp+2000h；配出來的段必須在那之後。
	if got <= psp+0x2000 {
		t.Errorf("AH=48h 配到 %04X，落在子行程自己的區塊裡（%04X–%04X）——"+
			"那塊記憶體正被它的堆疊用著", got, psp, psp+0x2000)
	}
}

// buildChildMZ 造一支「印 CHILD$ 然後 exit 42」的最小 MZ。
func buildChildMZ() []byte {
	code := []byte{
		0x0E,             // push cs
		0x1F,             // pop ds（DS 進場時指 PSP，不是程式碼段）
		0xBA, 0x0E, 0x00, // mov dx, 14
		0xB4, 0x09, // mov ah, 9
		0xCD, 0x21, // int 21h
		0xB8, 0x2A, 0x4C, // mov ax, 4C2Ah
		0xCD, 0x21, // int 21h
	}
	code = append(code, []byte("CHILD$")...)
	hdr := make([]byte, 32)
	hdr[0], hdr[1] = 'M', 'Z'
	total := len(hdr) + len(code)
	hdr[2] = byte(total % 512)    // LastPage
	hdr[4] = byte(total/512 + 1)  // Pages
	hdr[8] = byte(len(hdr) / 16)  // HeaderPar
	hdr[14], hdr[15] = 0, 0       // SS ＝ 載入段
	hdr[16], hdr[17] = 0x00, 0x01 // SP ＝ 0x0100
	return append(hdr, code...)
}

// TestExecRunsChildAndReturnsToParent 釘住 EXEC 的兩半：
// 子程式**真的跑**（印出字、離開碼 42），而且**父程式拿得回控制權**
// （暫存器還原、CF 清 0、AH=4Dh 回 AX=0x002A）。
//
// 只做一半的典型症狀：子程式跑了但父程式永遠停機（看起來像 EXEC 成功），
// 或父程式回來了但 AH=4Dh 的 AH 留垃圾——GIN3.COM 拿整個 AX 比較，
// AH 不等於 0 會讓「回碼 0」被讀成非 0（spec §3）。
func TestExecRunsChildAndReturnsToParent(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "CHILD.EXE"), buildChildMZ(), 0o644); err != nil {
		t.Fatal(err)
	}

	// 父程式的執行環境：檔名字串與參數區放在 5000h 段。
	m.WriteBytes(0x50000, append([]byte("child.exe"), 0))
	m.Write16(0x50100, 0)      // env ＝ 繼承
	m.Write16(0x50102, 0x0120) // 尾巴 → 5000:0120
	m.Write16(0x50104, 0x5000)
	m.WriteBytes(0x50120, []byte{3, '9', ' ', '1', 0x0D})
	m.Write16(0x50106, 0x0140) // FCB1 → 5000:0140
	m.Write16(0x50108, 0x5000)
	m.Write16(0x5010A, 0x0150) // FCB2 → 5000:0150
	m.Write16(0x5010C, 0x5000)

	c := m.CPU
	c.Seg[cpu.DS] = 0x5000
	c.R[cpu.DX] = 0
	c.Seg[cpu.ES] = 0x5000
	c.R[cpu.BX] = 0x0100
	c.R[cpu.CX] = 0xBEEF // 父程式暫存器，要原樣回來
	call(m, d, 0x21, 0x4B00)

	if len(d.procStack) != 1 {
		t.Fatalf("EXEC 之後行程疊深度要是 1，得到 %d", len(d.procStack))
	}
	if d.curPSP == machine.PSPSeg {
		t.Fatal("curPSP 沒切到子程式")
	}
	// 子程式 PSP+80h 要有尾巴。
	if got := m.Read8(uint32(d.curPSP)*16 + 0x80); got != 3 {
		t.Errorf("子程式 PSP+80h 的尾巴長度 ＝ %d，預期 3", got)
	}

	// 跑子程式直到它 exit（控制權回父程式，不停機）。
	for i := 0; i < 100 && !d.Exited && len(d.procStack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != 0 {
		t.Fatal("子程式沒有結束")
	}
	if d.Exited {
		t.Fatal("子程式 exit 把整台機器停掉了——父程式拿不回控制權")
	}
	if got := string(d.Console); got != "CHILD" {
		t.Errorf("主控台 ＝ %q，預期 \"CHILD\"", got)
	}
	if c.Flags&cpu.CF != 0 {
		t.Error("EXEC 成功回來時 CF 要是 0")
	}
	if c.R[cpu.CX] != 0xBEEF {
		t.Errorf("父程式 CX ＝ %04X，預期 BEEF（暫存器沒還原）", c.R[cpu.CX])
	}

	call(m, d, 0x21, 0x4D00)
	if got := c.R[cpu.AX]; got != 0x002A {
		t.Errorf("AH=4Dh 回 AX ＝ %04X，預期 002A（AH 必須是 0）", got)
	}
}

// TestExecMissingFileFailsAndParentSurvives：找不到檔 → CF=1、AX=2，
// 而且 execStack 不推東西（父程式狀態原封不動）。
func TestExecMissingFileFailsAndParentSurvives(t *testing.T) {
	m, d := newTest(t)
	m.WriteBytes(0x50000, append([]byte("nope.exe"), 0))
	c := m.CPU
	c.Seg[cpu.DS] = 0x5000
	c.R[cpu.DX] = 0
	c.Seg[cpu.ES] = 0x5000
	c.R[cpu.BX] = 0x0100
	call(m, d, 0x21, 0x4B00)
	if c.Flags&cpu.CF == 0 || c.R[cpu.AX] != 2 {
		t.Errorf("找不到檔要 CF=1、AX=2，得到 CF=%v AX=%04X",
			c.Flags&cpu.CF != 0, c.R[cpu.AX])
	}
	if len(d.procStack) != 0 || d.curPSP != machine.PSPSeg {
		t.Error("失敗的 EXEC 動到了 exec 狀態")
	}
}

// TestEMMDeviceOpensAndCloses：EMMXXXX0 開得到、關得掉、讀回 EOF（spec §5）。
func TestEMMDeviceOpensAndCloses(t *testing.T) {
	m, d := newTest(t)
	m.WriteBytes(0x50000, append([]byte("EMMXXXX0"), 0))
	m.CPU.Seg[cpu.DS] = 0x5000
	m.CPU.R[cpu.DX] = 0
	call(m, d, 0x21, 0x3D00)
	c := m.CPU
	if c.Flags&cpu.CF != 0 {
		t.Fatal("開 EMMXXXX0 失敗——launcher 會判定沒有 EMS 驅動")
	}
	h := c.R[cpu.AX]
	m.CPU.R[cpu.BX] = h
	m.CPU.R[cpu.CX] = 16
	call(m, d, 0x21, 0x3F00)
	if m.CPU.R[cpu.AX] != 0 {
		t.Errorf("裝置讀取要回 EOF（0），得到 %d", m.CPU.R[cpu.AX])
	}
	m.CPU.R[cpu.BX] = h
	call(m, d, 0x21, 0x3E00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Error("關 EMMXXXX0 失敗")
	}
}

// TestChildExitReclaimsMemory：子程式結束時，它名下的記憶體要還給系統
// （`docs/spec/010` §1）。
//
// 不還的後果不會當場出現：下一支 EXEC 進來的程式只是被載得比較高，
// 一切照跑，直到它向 DOS 要一塊大記憶體要不到為止——而那時距離
// EXEC 已經兩千多萬道指令，看起來完全像另一件事。
func TestChildExitReclaimsMemory(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "CHILD.EXE"), buildChildMZ(), 0o644); err != nil {
		t.Fatal(err)
	}
	m.WriteBytes(0x50000, append([]byte("child.exe"), 0))

	before := d.freeSeg
	c := m.CPU
	c.Seg[cpu.DS] = 0x5000
	c.R[cpu.DX] = 0
	c.Seg[cpu.ES] = 0x5000
	c.R[cpu.BX] = 0x0100
	call(m, d, 0x21, 0x4B00)
	if d.freeSeg <= before {
		t.Fatalf("EXEC 之後配置游標要往上走：%04X → %04X", before, d.freeSeg)
	}
	for i := 0; i < 100 && !d.Exited && len(d.procStack) > 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.procStack) != 0 {
		t.Fatal("子程式沒有結束")
	}
	if d.freeSeg != before {
		t.Errorf("子程式結束後配置游標 ＝ %04X，預期退回 %04X", d.freeSeg, before)
	}
	// 回收之後，下一支子程式要能載回同一個位置。
	call(m, d, 0x21, 0x4B00)
	if d.curPSP != before+1 {
		t.Errorf("第二支子程式的 PSP ＝ %04X，預期 %04X（沒有真的回收）",
			d.curPSP, before+1)
	}
}

// TestFileOpsRecordSeekAndRead：檔案存取要記下**偏移**，不只記檔名。
//
// 「這個檔是整份載入還是按需取用」決定了下一步要去搜記憶體還是看存取記錄。
// 字型就是後者：整份 GRAPH.IMG 從來沒進過記憶體，遊戲每畫一個字才 seek
// 過去讀 30 bytes——只記檔名的話這件事完全看不出來。
func TestFileOpsRecordSeekAndRead(t *testing.T) {
	m, d := newTest(t)
	body := make([]byte, 256)
	for i := range body {
		body[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(d.Root, "DATA.BIN"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	m.WriteBytes(0x50000, append([]byte("data.bin"), 0))
	c := m.CPU
	c.Seg[cpu.DS], c.R[cpu.DX] = 0x5000, 0
	call(m, d, 0x21, 0x3D00)
	if c.Flags&cpu.CF != 0 {
		t.Fatalf("開檔失敗 AX=%04X", c.R[cpu.AX])
	}
	h := c.R[cpu.AX]

	c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX] = h, 0, 100 // seek 到 100
	call(m, d, 0x21, 0x4200)
	c.R[cpu.BX], c.R[cpu.CX] = h, 16
	c.Seg[cpu.DS], c.R[cpu.DX] = 0x6000, 0
	call(m, d, 0x21, 0x3F00)

	// FileOps 是完整的操作序列（開／定位／讀），不只讀寫。
	var opened, seek, read *FileOp
	for i := range d.FileOps {
		switch o := &d.FileOps[i]; o.Op {
		case "open":
			opened = o
		case "seek":
			seek = o
		case "read":
			read = o
		}
	}
	if opened == nil || opened.Fn != 0x3D || opened.Name != "data.bin" {
		t.Errorf("open 記錄 = %+v，預期 AH=3D data.bin", opened)
	}
	if seek == nil || seek.Fn != 0x42 || seek.Arg != 100 || seek.Pos != 100 {
		t.Errorf("seek 記錄 = %+v，預期 AH=42 Arg=100 Pos=100", seek)
	}
	if read == nil || read.Fn != 0x3F || read.Pos != 100 || read.Len != 16 || read.Arg != 16 {
		t.Errorf("read 記錄 = %+v，預期 AH=3F Pos=100 Len=16 Arg=16", read)
	}
	if got := m.Read8(0x60000); got != 100 {
		t.Errorf("讀進來的第一個 byte = %d，預期 100（seek 沒生效）", got)
	}
}
