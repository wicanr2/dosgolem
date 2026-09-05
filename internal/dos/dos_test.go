package dos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// 這一份測試釘的全是**會安靜出錯**的規則：每一條的反面都不會報錯，
// 只會讓 `RUN.EXE` 在很後面的地方走進錯的分支。測試名字就說症狀。

// newTest 造一台掛好服務層的機器。沒有載入映像，所以自己指定 freeSeg。
func newTest(t *testing.T) (*machine.Machine, *DOS) {
	t.Helper()
	m := machine.New()
	d := New(m, t.TempDir())
	d.Install()
	d.freeSeg = 0x2000
	return m, d
}

// call 設好 AX 之後發一個中斷，走的是真正的分派路徑（含向量檢查）。
func call(m *machine.Machine, d *DOS, intNo uint8, ax uint16) {
	m.CPU.R[cpu.AX] = ax
	d.handle(m.CPU, intNo)
}

// TestSetBlockProbeMustFailFirst 釘住 `AH=4Ah` 的探測語意（`docs/spec/004` §1.2）。
//
// 呼叫端故意要求 0FFFFh 段，然後 `jae 錯誤`——**成功才是錯誤路徑**。
// 一律清 CF 報成功的話第一次呼叫就掉進 `DOS memory-arena error`，
// 而症狀看起來像 MCB 佈局不對（那條路連續三輪都無效）。
func TestSetBlockProbeMustFailFirst(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.ES] = machine.PSPSeg
	m.CPU.R[cpu.BX] = 0xFFFF
	call(m, d, 0x21, 0x4A00)

	if m.CPU.Flags&cpu.CF == 0 {
		t.Fatal("要求 0FFFFh 段竟然成功——呼叫端的 jae 會跳進錯誤路徑")
	}
	avail := m.CPU.R[cpu.BX]
	if avail == 0 || avail == 0xFFFF {
		t.Fatalf("BX 要回實際可用段數，得到 %04X", avail)
	}

	// 第二次拿 DOS 回報的大小再要一次，這次一定要成功。
	m.CPU.R[cpu.BX] = avail
	call(m, d, 0x21, 0x4A00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("拿 DOS 自己回的 %04X 再要一次還失敗——呼叫端的 jb 會判定錯誤", avail)
	}
}

// TestConOutTakesCharFromDL 釘住「字元在 DL 不是 AL」。
//
// 印字元的路徑是 `mov dx,ax / mov ah,6 / int 21h`，讀 AL 收到的是功能號的
// 殘留。第一版就是這樣把 `RUN.EXE` 的錯誤訊息整個漏掉——主控台是空的，
// 看起來像「程式什麼都沒說」。
func TestConOutTakesCharFromDL(t *testing.T) {
	for _, fn := range []uint16{0x02, 0x06} {
		m, d := newTest(t)
		m.CPU.R[cpu.DX] = 'X'
		call(m, d, 0x21, fn<<8|0x99) // AL 故意放垃圾
		if got := string(d.Console); got != "X" {
			t.Errorf("AH=%02Xh 印出 %q，預期 \"X\"（字元在 DL）", fn, got)
		}
	}
}

// TestSetVectorIsHonouredAndThenNotIntercepted 釘住 `AH=25h` 的兩半。
//
// 程式用它裝自帶的 Microsoft 浮點模擬器（`INT 34h`–`3Dh`，全檔 876 個呼叫）。
// 要做對兩件事：向量**真的寫進去**，而且之後那個中斷**要放行**——
// 攔下來的話所有浮點運算落空，而 BASIC 的金錢運算全靠它。
func TestSetVectorIsHonouredAndThenNotIntercepted(t *testing.T) {
	m, d := newTest(t)
	if d.handle(m.CPU, 0x34) != true {
		t.Fatal("裝之前的 INT 34h 應該由我們接手")
	}
	m.CPU.Seg[cpu.DS] = 0x1234
	m.CPU.R[cpu.DX] = 0x5678
	call(m, d, 0x21, 0x2534)

	if off, seg := m.Read16(0x34*4), m.Read16(0x34*4+2); off != 0x5678 || seg != 0x1234 {
		t.Fatalf("向量表 34h 是 %04X:%04X，預期 1234:5678", seg, off)
	}
	if d.handle(m.CPU, 0x34) != false {
		t.Error("程式自己裝了 INT 34h，我們還攔——浮點運算會全部落空")
	}
}

// TestReadStdinNeverReturnsZero 釘住 `AH=3Fh` 讀 handle 0。
//
// 那是 BASIC 的 `INKEY$`，也是唯一的鍵盤輪詢路徑。回「讀到 0 個」等同 EOF，
// 主程式會還原中斷向量然後 exit——`RUN.EXE` 之前就死在這裡。
func TestReadStdinNeverReturnsZero(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.DS] = 0x2000
	m.CPU.R[cpu.DX] = 0
	m.CPU.R[cpu.BX] = 0
	m.CPU.R[cpu.CX] = 1
	call(m, d, 0x21, 0x3F00)
	if m.CPU.R[cpu.AX] != 1 {
		t.Fatalf("讀 handle 0 回 %d 個位元組——0 等同 EOF，程式會 exit", m.CPU.R[cpu.AX])
	}
}

// TestUnimplementedLeavesAXAlone 釘住原則 1（`docs/spec/004` §1.1）。
//
// 沒實作的功能號寫 AX=0 會把「設中斷向量」迴圈的計數清掉，`AH` 變成 0
// ＝ `AH=00h` ＝ 結束程式。症狀是程式在初始化中途安靜地消失。
func TestUnimplementedLeavesAXAlone(t *testing.T) {
	m, d := newTest(t)
	const ax = 0x5F42 // 網路重導向，本專案沒實作
	call(m, d, 0x21, ax)
	if m.CPU.R[cpu.AX] != ax {
		t.Errorf("未實作的服務把 AX 改成 %04X（原本 %04X）", m.CPU.R[cpu.AX], ax)
	}
	if d.Unimplemented[Call{Int: 0x21, AH: 0x5F, AL: 0x42}] != 1 {
		t.Error("未實作的呼叫沒有記一筆——之後就分不出「跑得動」與「跑得動但錯」")
	}
}

// TestInt10AltSelectReadsBL 釘住「子功能選擇子在 BL 不是 AL」。
//
// 查 AL 的話 `AH=12h BL=10h` 那個分支永遠不成立，BX 保持呼叫端傳進來的值，
// 於是顯示卡偵測讀到的「記憶體大小」是它自己剛寫的 10h。
func TestInt10AltSelectReadsBL(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 0x0010
	call(m, d, 0x10, 0x1200)
	if bh := uint8(m.CPU.R[cpu.BX] >> 8); bh != 0x00 {
		t.Errorf("BH ＝ %02X，預期 00（彩色模式）", bh)
	}
	if bl := uint8(m.CPU.R[cpu.BX]); bl != 0x03 {
		t.Errorf("BL ＝ %02X，預期 03（256 KB）——分支是不是查了 AL？", bl)
	}
}

// TestInt10DisplayCombinationSaysVGA 釘住 `AH=1Ah`。
//
// BASIC runtime 用它判斷能不能 `SCREEN 13`。沒實作 → BL 是垃圾 →
// 認定不是 VGA → `SCREEN 13` 回 Illegal function call，
// 而那個錯誤出現的位置離這裡很遠。
func TestInt10DisplayCombinationSaysVGA(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x10, 0x1A00)
	if al := uint8(m.CPU.R[cpu.AX]); al != 0x1A {
		t.Errorf("AL ＝ %02X，預期 1A（表示本服務有支援）", al)
	}
	if bl := uint8(m.CPU.R[cpu.BX]); bl != 0x08 {
		t.Errorf("BL ＝ %02X，預期 08（VGA 彩色）", bl)
	}
}

// TestVideoModeIsRemembered 釘住「設了 mode 13h 之後查得到」。
//
// `AH=0Fh` 一直回 3 的話，程式設完 mode 13h 再查會以為沒設成功。
func TestVideoModeIsRemembered(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x10, 0x0013)
	call(m, d, 0x10, 0x0F00)
	if mode := uint8(m.CPU.R[cpu.AX]); mode != 0x13 {
		t.Errorf("設了 13h 之後查到 %02X", mode)
	}
	if cols := uint8(m.CPU.R[cpu.AX] >> 8); cols != 40 {
		t.Errorf("mode 13h 的欄數是 %d，預期 40", cols)
	}
}

// TestMouseButtonStatsUseFunctionNumber 釘住 `AX=5`／`AX=6` 的分派。
//
// 兩支的回傳值第一個就是寫進 AX，所以**判斷分支之前要先把功能號存起來**；
// 存之前先寫 AX 的話 `AX=6` 會回按下的統計，放開的統計永遠是 0——
// 防拷畫面的滑鼠點擊因此只有一半有效。
func TestMouseButtonStatsUseFunctionNumber(t *testing.T) {
	m, d := newTest(t)
	d.Mouse.Press[0], d.Mouse.Release[0] = 3, 7

	m.CPU.R[cpu.BX] = 0 // 問左鍵
	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.BX] != 3 {
		t.Errorf("AX=5 回 BX=%d，預期 3（按下次數）", m.CPU.R[cpu.BX])
	}
	m.CPU.R[cpu.BX] = 0
	call(m, d, 0x33, 0x0006)
	if m.CPU.R[cpu.BX] != 7 {
		t.Errorf("AX=6 回 BX=%d，預期 7（放開次數）——分支是不是讀了剛寫的 AX？",
			m.CPU.R[cpu.BX])
	}
	// 讀走就歸零。
	m.CPU.R[cpu.BX] = 0
	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.BX] != 0 {
		t.Errorf("第二次讀按下統計回 %d，預期 0", m.CPU.R[cpu.BX])
	}
}

// TestMouseResetReportsInstalled 釘住 `AX=0`。
//
// 回 0 就是「沒有驅動」，遊戲從此不發 `int 33h`；防拷畫面只吃滑鼠
// （`rich2/docs/playtest/001` §3：鍵盤全都無效），所以那等於卡死。
func TestMouseResetReportsInstalled(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x33, 0x0000)
	if m.CPU.R[cpu.AX] != 0xFFFF {
		t.Errorf("AX ＝ %04X，預期 FFFF（已安裝）", m.CPU.R[cpu.AX])
	}
	if m.CPU.R[cpu.BX] != 2 {
		t.Errorf("BX ＝ %d，預期 2（兩個鍵）", m.CPU.R[cpu.BX])
	}
}

// TestMousePollIsRecorded 釘住「每次 AX=3 都要留一筆」。
//
// 「輸入沒送到」與「送到了但答錯」的畫面表現一模一樣，
// 這份紀錄是唯一分得出來的東西。
func TestMousePollIsRecorded(t *testing.T) {
	m, d := newTest(t)
	d.Mouse.X, d.Mouse.Y = 100, 50
	call(m, d, 0x33, 0x0003)
	if len(d.Mouse.Polls) != 1 {
		t.Fatalf("輪詢了 1 次卻記了 %d 筆", len(d.Mouse.Polls))
	}
	// mode 13h 的標準驅動水平回報 0–639（`rich2/docs/re/182` §3）。
	if m.CPU.R[cpu.CX] != 200 || m.CPU.R[cpu.DX] != 50 {
		t.Errorf("回報 (%d,%d)，預期 (200,50)：水平要乘 XScale",
			m.CPU.R[cpu.CX], m.CPU.R[cpu.DX])
	}
}

// TestAllocIsRealBumpAllocator 釘住 `AH=48h`。
//
// 固定回 64 KB 的話 BASIC runtime 報 Error 07（Out of memory），
// 而錯誤訊息只是一個數字。
func TestAllocIsRealBumpAllocator(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 0x100
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("第一次配置就失敗")
	}
	first := m.CPU.R[cpu.AX]

	m.CPU.R[cpu.BX] = 0x100
	call(m, d, 0x21, 0x4800)
	second := m.CPU.R[cpu.AX]
	if second < first+0x100 {
		t.Errorf("兩次配置重疊：%04X 與 %04X（各 100h 段）", first, second)
	}

	// 要不到的時候要說要不到，並在 BX 回實際可用大小。
	m.CPU.R[cpu.BX] = 0xFFFF
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Error("要 0FFFFh 段竟然成功了")
	}
	if m.CPU.R[cpu.BX] == 0xFFFF {
		t.Error("失敗時 BX 沒回實際可用大小")
	}
}

// TestCurrentDriveIsReported 釘住 `AH=19h`。
//
// 不實作的話 AL 是垃圾，遊戲把它拼進路徑就變成 `A:\…`——
// 而我們按 basename 解析，**open 還是會成功**，錯誤完全不顯現。
func TestCurrentDriveIsReported(t *testing.T) {
	m, d := newTest(t)
	d.Drive = 2
	m.CPU.R[cpu.AX] = 0x19FF
	d.handle(m.CPU, 0x21)
	if al := uint8(m.CPU.R[cpu.AX]); al != 2 {
		t.Errorf("AL ＝ %d，預期 2", al)
	}
}

// TestOpenIgnoresPathAndCase 釘住檔名解析。
//
// 遊戲會組出多磁片版殘留的 `A:\<垃圾>\DATA.PAK`（`rich2/docs/re/006` §5），
// 而玩家的目錄可能是小寫。兩邊都要接得起來。
func TestOpenIgnoresPathAndCase(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "data.pak"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.CPU.Seg[cpu.DS] = 0x3000
	m.CPU.R[cpu.DX] = 0
	m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte(`A:\RICH2\DATA.PAK`), 0))
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("開檔失敗（AX=%d）；Missing=%v", m.CPU.R[cpu.AX], d.Missing)
	}
	h := m.CPU.R[cpu.AX]

	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = h, 5, 0x40
	call(m, d, 0x21, 0x3F00)
	if m.CPU.R[cpu.AX] != 5 {
		t.Fatalf("讀到 %d 個位元組，預期 5", m.CPU.R[cpu.AX])
	}
	got := make([]byte, 5)
	for i := range got {
		got[i] = m.Read8(cpu.Addr(0x3000, 0x40) + uint32(i))
	}
	if string(got) != "hello" {
		t.Errorf("讀到 %q", got)
	}
}

// TestMissingFileIsRecorded 釘住「找不到的檔要留名字」。
//
// 只回 CF 的話，缺一個資產與「程式自己決定不載」看起來一樣。
func TestMissingFileIsRecorded(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.DS] = 0x3000
	m.CPU.R[cpu.DX] = 0
	m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte("NOPE.PAK"), 0))
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Fatal("開一個不存在的檔竟然成功")
	}
	if len(d.Missing) != 1 || d.Missing[0] != "NOPE.PAK" {
		t.Errorf("Missing ＝ %v", d.Missing)
	}
}

// TestExitStopsTheCPU 釘住 `AH=4Ch`。
func TestExitStopsTheCPU(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x21, 0x4C03)
	if !d.Exited || d.ExitCode != 3 {
		t.Errorf("Exited=%v ExitCode=%d", d.Exited, d.ExitCode)
	}
	if !m.CPU.Halted {
		t.Error("程式結束了但 CPU 還在跑")
	}
}

// TestDefaultDriveIsHardDisk 釘住「目前磁碟預設是 C:」。
//
// 回 A: 的話程式判定自己是從磁片跑，停在
// 「Please put Disk#2 in A: and put Disk#3 in B:」等按鍵。
// 出處是 rich2/tools/dosemu.py 的 `cur_drive = 2`，那支跑通過。
func TestDefaultDriveIsHardDisk(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.AX] = 0x19FF
	d.handle(m.CPU, 0x21)
	if al := uint8(m.CPU.R[cpu.AX]); al != 2 {
		t.Errorf("目前磁碟是 %d（0=A: 1=B: 2=C:）——不是 C: 會停在換磁片提示", al)
	}
}

// TestClockIsZeroForSeedParity 釘住「固定時刻是全 0」。
//
// 原版那邊的固定種子版（rich2/tools/patch_seed.py）把 TIMER 內部的
// `mov ah,2Ch / int 21h` 換成 `xor cx,cx / xor dx,dx`。我們不改 binary，
// 讓 `AH=2Ch` 直接回 CX=DX=0，兩邊的 RANDOMIZE TIMER 才拿到同一個種子。
//
// 回別的值不會報錯——只會讓防拷畫面問**不同的一題**，
// 於是逐點比對永遠不合，而畫面看起來完全正常。
func TestClockIsZeroForSeedParity(t *testing.T) {
	m, d := newTest(t)
	call(m, d, 0x21, 0x2C00)
	if m.CPU.R[cpu.CX] != 0 || m.CPU.R[cpu.DX] != 0 {
		t.Errorf("AH=2Ch 回 CX=%04X DX=%04X，預期都是 0（與固定種子版對齊）",
			m.CPU.R[cpu.CX], m.CPU.R[cpu.DX])
	}
}

// TestMouseButtonStatsAreePerButton 釘住「`AX=5` 的 `BX` 是輸入」。
//
// 《臥龍傳》的等待迴圈（IDA `0x121E9`）**先問右鍵再問左鍵**：
//
//	mov ax,5 / mov bx,1 / int 33h   ; 右鍵按下次數 → 有就 stc（取消）
//	mov ax,5 / xor bx,bx / int 33h  ; 左鍵按下次數 → 有就讀位置
//
// 不分鍵的實作會把左鍵那一次交給問右鍵的呼叫，於是**每一次左鍵點擊
// 都被讀成右鍵**。畫面上看起來像「點什麼都是取消」，
// 而所有回傳值單獨看都合法。
func TestMouseButtonStatsArePerButton(t *testing.T) {
	m, d := newTest(t)
	d.Mouse.X, d.Mouse.Y = 320, 175
	d.Mouse.PressButton(0) // 按左鍵

	m.CPU.R[cpu.BX] = 1 // 先問右鍵
	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.BX] != 0 {
		t.Fatalf("問右鍵回 %d 次按下，預期 0——左鍵的那一次被右鍵領走了",
			m.CPU.R[cpu.BX])
	}
	m.CPU.R[cpu.BX] = 0 // 再問左鍵
	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.BX] != 1 {
		t.Fatalf("問左鍵回 %d 次按下，預期 1", m.CPU.R[cpu.BX])
	}
	// 座標是**按下那一刻**的，不是現在的。
	d.Mouse.X, d.Mouse.Y = 0, 0
	d.Mouse.PressButton(0)
	d.Mouse.X, d.Mouse.Y = 600, 300
	m.CPU.R[cpu.BX] = 0
	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.CX] != 0 || m.CPU.R[cpu.DX] != 0 {
		t.Errorf("AX=5 回 (%d,%d)，預期按下那一刻的 (0,0)",
			m.CPU.R[cpu.CX], m.CPU.R[cpu.DX])
	}
}

// TestSoundTimerCallbackActuallyFires 釘住 `int 61h AH=0Ch`。
//
// ⭐ **這一支不接的話遊戲時鐘不會走，而畫面完全正常。**
// 《臥龍傳》的日期是音效驅動用 291.3 Hz 的回呼推的
// （臥龍傳專案 `docs/re/61`）；沒有回呼，日期永遠停在第一天，
// 兩層節流的等待迴圈也永遠等不到——**看起來像遊戲在等玩家操作**。
func TestSoundTimerCallbackActuallyFires(t *testing.T) {
	m, d := newTest(t)
	// 回呼本體：一個 retf。放在一個不會被別的東西用到的段。
	const cbSeg, cbOff = 0x3000, 0x0010
	m.Write8(cbSeg*16+cbOff, 0xCB)
	// 主程式：原地跳自己（EB FE），讓機器有東西可以跑。
	const mainSeg = 0x3100
	m.Write8(mainSeg*16, 0xEB)
	m.Write8(mainSeg*16+1, 0xFE)
	m.CPU.Seg[cpu.CS], m.CPU.IP = mainSeg, 0
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP] = 0x3200, 0x100

	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = cbSeg, cbOff
	call(m, d, 0x61, 0x0C00)

	before := m.PeriodicCalls()
	for i := 0; i < 3*int(m.IRQ0Every/16); i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if got := m.PeriodicCalls() - before; got < 2 {
		t.Fatalf("跑了三個回呼週期只發出 %d 次遠呼叫——AH=0Ch 有沒有接上？", got)
	}

	// AL=1 是取消。取消之後就不該再發。
	call(m, d, 0x61, 0x0C01)
	now := m.PeriodicCalls()
	for i := 0; i < 3*int(m.IRQ0Every/16); i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.PeriodicCalls() != now {
		t.Errorf("取消之後還發了 %d 次", m.PeriodicCalls()-now)
	}
}

// mouseTestRig 造一台有主迴圈與一支會弄髒暫存器的事件常式的機器。
//
// 常式故意 clobber AX/BX/CX/DX/SI/DI：**真機上這支是從驅動的 ISR 裡呼叫的，
// ISR 會把暫存器全部存起來**。我們插進去沒存的話，被打斷的那段程式會
// 莫名其妙拿到別的值，而症狀出現在很後面、完全不指向滑鼠。
func mouseTestRig(t *testing.T) (*machine.Machine, *DOS) {
	t.Helper()
	m, d := newTest(t)
	const hSeg, mainSeg = 0x3000, 0x3100
	code := []byte{
		0x90,             // nop ← 留一格給「進到常式但還沒動暫存器」的取樣
		0xB8, 0x34, 0x12, // mov ax, 1234h
		0xBB, 0x78, 0x56, // mov bx, 5678h
		0xB9, 0xCD, 0xAB, // mov cx, ABCDh
		0xBA, 0x21, 0x43, // mov dx, 4321h
		0xCB, // retf
	}
	for i, b := range code {
		m.Write8(hSeg*16+uint32(i), b)
	}
	m.Write8(mainSeg*16, 0xEB) // jmp $
	m.Write8(mainSeg*16+1, 0xFE)
	m.CPU.Seg[cpu.CS], m.CPU.IP = mainSeg, 0
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP] = 0x3200, 0x100

	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = hSeg, 0
	m.CPU.R[cpu.CX] = EventMove
	call(m, d, 0x33, 0x000C)
	return m, d
}

func stepN(t *testing.T, m *machine.Machine, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestMouseEventCallbackRestoresEverything 釘住回呼的存檔還原。
func TestMouseEventCallbackRestoresEverything(t *testing.T) {
	m, d := mouseTestRig(t)
	m.CPU.R[cpu.AX], m.CPU.R[cpu.BX] = 0x1111, 0x2222
	m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 0x3333, 0x4444
	m.CPU.R[cpu.SI], m.CPU.R[cpu.DI] = 0x5555, 0x6666
	before := m.CPU.R
	beforeCS, beforeIP, beforeSP := m.CPU.Seg[cpu.CS], m.CPU.IP, m.CPU.R[cpu.SP]

	d.MoveMouse(50, 60)
	if m.CallbackPending() != 1 {
		t.Fatalf("移動之後佇列有 %d 筆，預期 1", m.CallbackPending())
	}
	stepN(t, m, 40)

	if m.CallbacksMade() != 1 {
		t.Fatalf("回呼送出 %d 次，預期 1——`Machine.tick` 有沒有接上？",
			m.CallbacksMade())
	}
	if m.CPU.R != before {
		t.Errorf("暫存器沒還原：%v → %v", before, m.CPU.R)
	}
	if m.CPU.Seg[cpu.CS] != beforeCS || m.CPU.IP != beforeIP {
		t.Errorf("回到 %04X:%04X，預期 %04X:%04X",
			m.CPU.Seg[cpu.CS], m.CPU.IP, beforeCS, beforeIP)
	}
	if m.CPU.R[cpu.SP] != beforeSP {
		t.Errorf("堆疊指標 %04X，預期 %04X——`retf` 與哨兵有沒有對上？",
			m.CPU.R[cpu.SP], beforeSP)
	}
}

// TestMouseEventCarriesPosition 釘住傳給常式的參數。
func TestMouseEventCarriesPosition(t *testing.T) {
	m, d := mouseTestRig(t)
	d.Mouse.XScale = 1
	d.MoveMouse(123, 45)
	// 回呼開始的那一刻攔一下：跑一步就會進去。
	stepN(t, m, 1)
	if got := m.CPU.R[cpu.CX]; got != 123 {
		t.Errorf("CX＝%d，預期 123（游標 X）", got)
	}
	if got := m.CPU.R[cpu.DX]; got != 45 {
		t.Errorf("DX＝%d，預期 45（游標 Y）", got)
	}
	if got := m.CPU.R[cpu.AX]; got != EventMove {
		t.Errorf("AX＝%04X，預期 %04X（事件遮罩）", got, EventMove)
	}
}

// TestMouseEventMaskFilters 釘住「遮罩沒開的事件不發」。
func TestMouseEventMaskFilters(t *testing.T) {
	m, d := mouseTestRig(t) // 遮罩只有 EventMove
	d.PressMouse(0)
	if n := m.CallbackPending(); n != 0 {
		t.Errorf("遮罩沒開左鍵卻排了 %d 筆", n)
	}
	d.MoveMouse(10, 10)
	if n := m.CallbackPending(); n != 1 {
		t.Errorf("移動排了 %d 筆，預期 1", n)
	}
}

// TestMouseEventQueuesWhileBusy 釘住「回呼進行中的事件要排隊，不能丟」。
//
// 丟掉的話快速移動的軌跡會少幾格，而畫面看起來完全正常。
func TestMouseEventQueuesWhileBusy(t *testing.T) {
	m, d := mouseTestRig(t)
	d.MoveMouse(10, 10)
	stepN(t, m, 1) // 第一次進到常式裡
	d.MoveMouse(20, 20)
	if n := m.CallbackPending(); n != 1 {
		t.Fatalf("回呼進行中再移動，佇列有 %d 筆，預期 1（要排隊不要丟）", n)
	}
	stepN(t, m, 40)
	if m.CallbacksMade() != 2 {
		t.Errorf("總共送出 %d 次，預期 2", m.CallbacksMade())
	}
}

// TestMouseRangeClamps 釘住 `AX=7`／`AX=8` 的夾制。
//
// ⚠ **夾制是遊戲行為的一部分。**《臥龍傳》開機把範圍設成
// 0–27Fh × 0–18Fh；不夾的話「點在畫面外」這種邊界測試會得到相反的結論。
func TestMouseRangeClamps(t *testing.T) {
	m, d := newTest(t)
	d.Mouse.XScale = 1
	m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 0, 0x27F
	call(m, d, 0x33, 0x0007)
	m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 0, 0x18F
	call(m, d, 0x33, 0x0008)

	d.MoveMouse(700, 500)
	m.CPU.R[cpu.BX] = 0
	call(m, d, 0x33, 0x0003)
	if m.CPU.R[cpu.CX] != 639 || m.CPU.R[cpu.DX] != 399 {
		t.Errorf("送 (700,500) 之後回 (%d,%d)，預期 (639,399)",
			m.CPU.R[cpu.CX], m.CPU.R[cpu.DX])
	}
}
