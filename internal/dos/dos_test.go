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
	d.Mouse.Press, d.Mouse.Release = 3, 7

	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.BX] != 3 {
		t.Errorf("AX=5 回 BX=%d，預期 3（按下次數）", m.CPU.R[cpu.BX])
	}
	call(m, d, 0x33, 0x0006)
	if m.CPU.R[cpu.BX] != 7 {
		t.Errorf("AX=6 回 BX=%d，預期 7（放開次數）——分支是不是讀了剛寫的 AX？",
			m.CPU.R[cpu.BX])
	}
	// 讀走就歸零。
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

// TestOpenTruncatesToEightThree 釘住 FAT 的 8.3 截斷。
//
// KOL 傳 `steedpics` 進來，真 DOS 開到的是 `STEEDPIC`。不截的話這裡回「找不到檔」，
// 而程式多半不檢查開檔結果——KOL 是一路跑進沒有映射的記憶體才停，看起來像模擬器
// 壞了，其實只是檔名沒對上。
func TestOpenTruncatesToEightThree(t *testing.T) {
	for _, c := range []struct{ onDisk, asked string }{
		{"STEEDPIC", "steedpics"},                 // 主檔名過長，沒有副檔名
		{"LONGNAME.DAT", "longnamedata.database"}, // 兩邊都過長
	} {
		t.Run(c.asked, func(t *testing.T) {
			m, d := newTest(t)
			if err := os.WriteFile(filepath.Join(d.Root, c.onDisk), []byte("hello"), 0o644); err != nil {
				t.Fatal(err)
			}
			m.CPU.Seg[cpu.DS] = 0x3000
			m.CPU.R[cpu.DX] = 0
			m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte(c.asked), 0))
			call(m, d, 0x21, 0x3D00)
			if m.CPU.Flags&cpu.CF != 0 {
				t.Fatalf("開 %q（磁碟上是 %q）失敗：AX=%d，Missing=%v",
					c.asked, c.onDisk, m.CPU.R[cpu.AX], d.Missing)
			}
		})
	}
}

// 截斷不能把「真的不存在」變成開得起來：8 個字元以內的名字原樣比對。
func TestTruncationDoesNotInventFiles(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "DATA.PAK"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.CPU.Seg[cpu.DS] = 0x3000
	m.CPU.R[cpu.DX] = 0
	m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte("DATA.PAX"), 0))
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Fatal("開 DATA.PAX 竟然開到了 DATA.PAK")
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

// TestSetBlockGrowTakesTheSpace 釘住「`AH=4Ah` 擴大成功就要吃掉後面的空間」。
//
// 只報成功而不推進 bump 指標的話，「把記憶體配光」那個慣用法
//
//	迴圈： bx=1 / ah=48h / int 21h / jc 收工     ; 配 1 段
//	       bx=0FFFFh / ah=4Ah / int 21h / jc 重試 ; 撐到最大
//	       es:[0]=cx / cx=es / jmp 迴圈           ; 串成鏈
//
// **永遠不會結束**。KOL.EXE 因此一路配到自己的映像上，那句 `mov es:[0], cx`
// 把自己的 `int 21h` 改成了 `int 19h`——症狀看起來像模擬器把記憶體寫爛了。
func TestSetBlockGrowTakesTheSpace(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 1
	call(m, d, 0x21, 0x4800)
	blk := m.CPU.R[cpu.AX]

	// 撐到最大：先要 0FFFFh 拿回實際可用，再要那個數字。
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = blk, 0xFFFF
	call(m, d, 0x21, 0x4A00)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Fatal("要 0FFFFh 段竟然成功了")
	}
	m.CPU.Seg[cpu.ES] = blk
	call(m, d, 0x21, 0x4A00) // BX 已經是回填的可用段數
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("用回填的大小再要一次還是失敗")
	}

	// 記憶體已經配光，下一次配置一定要失敗。
	m.CPU.R[cpu.BX] = 1
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Errorf("記憶體配光之後還配得到 %04X 段——迴圈不會結束", m.CPU.R[cpu.AX])
	}
}

// TestSetBlockShrinkReturnsTheSpace 釘住「縮小最後一塊要把空間還回去」。
//
// KOL.EXE 配光記憶體之後縮回四段，緊接著就要那四段；不還的話它判定記憶體
// 不夠，以回傳碼 255 離開，主控台只有一行 `run-time error R6006`。
func TestSetBlockShrinkReturnsTheSpace(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.BX] = 1
	call(m, d, 0x21, 0x4800)
	blk := m.CPU.R[cpu.AX]

	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = blk, 0xFFFF
	call(m, d, 0x21, 0x4A00)
	m.CPU.Seg[cpu.ES] = blk
	grown := m.CPU.R[cpu.BX]
	call(m, d, 0x21, 0x4A00)

	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = blk, grown-4
	call(m, d, 0x21, 0x4A00)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("縮小失敗")
	}
	m.CPU.R[cpu.BX] = 2
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Error("縮回四段之後要兩段還是失敗——縮小沒有把空間還回去")
	}
}

// TestAllocRefusesPastMemTop 釘住 `avail` 的下溢。
//
// 直接寫 `MemTop - seg` 的話，bump 指標越過 `MemTop` 之後 uint16 環繞成 0FFFFh，
// 於是配置**永遠成功**——那個配光記憶體的迴圈會一路配到 0FFFFh 段再繞回低位，
// 把整台機器的記憶體覆蓋掉。
func TestAllocRefusesPastMemTop(t *testing.T) {
	m, d := newTest(t)
	d.freeSeg = uint16(machine.MemTop) + 1
	m.CPU.R[cpu.BX] = 1
	call(m, d, 0x21, 0x4800)
	if m.CPU.Flags&cpu.CF == 0 {
		t.Errorf("指標越過 MemTop 之後還配得到 %04X 段", m.CPU.R[cpu.AX])
	}
}

// TestLoadOverlayPlacesImage 釘住 `AH=4Bh AL=03h`（載入 overlay）。
//
// KOL 的架構是 `KOL.EXE` 主程式 ＋ `TITLE.EXE`／`KOLTOWN.EXE` overlay：主程式
// 自己配好記憶體、叫 DOS 把映像搬進去、再 far call 進入點。沒實作的症狀是
// C runtime 印 `run-time error R6006 - bad format on exec` 然後以回傳碼 255
// 離開——**看起來像 EXE 檔壞了**，而不像少了一個服務。
func TestLoadOverlayPlacesImage(t *testing.T) {
	m, d := newTest(t)

	// 一支 48 byte 的 MZ：檔頭兩段，映像是 16 byte，位移 4 有一筆重定位項。
	hdr := make([]byte, 32)
	copy(hdr, "MZ")
	put16 := func(off int, v uint16) { hdr[off], hdr[off+1] = uint8(v), uint8(v>>8) }
	put16(2, 48)  // 最後一頁用了 48 byte
	put16(4, 1)   // 一頁
	put16(6, 1)   // 一筆重定位
	put16(8, 2)   // 檔頭兩段
	put16(24, 28) // 重定位表在 28
	put16(28, 4)  // 項：0000:0004
	img := append(hdr, make([]byte, 16)...)
	img[32+4], img[32+5] = 0x34, 0x12 // 待重定位的段值 0x1234

	if err := os.WriteFile(filepath.Join(d.Root, "OVL.EXE"), img, 0o644); err != nil {
		t.Fatal(err)
	}

	const seg, fixup = 0x3000, 0x2500
	pb := cpu.Addr(0x0800, 0x0010)
	m.Write16(pb, seg)
	m.Write16(pb+2, fixup)
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0x0800, 0x0010
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x0900, 0x0000
	m.WriteBytes(cpu.Addr(0x0900, 0), append([]byte("OVL.EXE"), 0))
	call(m, d, 0x21, 0x4B03)

	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatalf("載入 overlay 失敗，AX=%04X", m.CPU.R[cpu.AX])
	}
	if got := m.Read16(cpu.Addr(seg, 4)); got != 0x1234+fixup {
		t.Errorf("重定位項是 %04X，預期 %04X＝0x1234＋重定位因子 %04X",
			got, 0x1234+fixup, fixup)
	}
}

// TestHandlesAreReused 釘住「handle 號碼要重用」。
//
// 真 DOS 從 JFT 挑第一個空位，所以「開→關→開」拿到同一個小號碼。只增不減的話
// 開關幾十次之後號碼就爬到 20 以上，而遊戲常拿 handle 當自己那張表的索引。
//
// **症狀完全不像 handle 的問題。**《武士傳說》的表存著「這個 handle 上次 seek
// 到哪」，越界之後新開的檔繼承了別人的位置，於是該發的 `AH=42h` 整個沒發——
// 解壓器從檔案開頭讀到目錄本身，程式最後停在走不完的字典鏈裡，看起來像
// LZW 解壓器寫錯了。
func TestHandlesAreReused(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "A.DAT"), []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.WriteBytes(cpu.Addr(0x0900, 0), append([]byte("A.DAT"), 0))
	open := func() uint16 {
		m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x0900, 0
		call(m, d, 0x21, 0x3D00)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("開檔失敗，AX=%04X", m.CPU.R[cpu.AX])
		}
		return m.CPU.R[cpu.AX]
	}
	closeH := func(h uint16) {
		m.CPU.R[cpu.BX] = h
		call(m, d, 0x21, 0x3E00)
	}

	first := open()
	for i := 0; i < 40; i++ {
		closeH(first)
		h := open()
		if h != first {
			t.Fatalf("第 %d 次重開拿到 handle %d，第一次是 %d——號碼沒有重用", i, h, first)
		}
	}

	// 同時開著的才往上長，而且滿了要說滿了。
	var opened []uint16
	for {
		m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x0900, 0
		call(m, d, 0x21, 0x3D00)
		if m.CPU.Flags&cpu.CF != 0 {
			break
		}
		opened = append(opened, m.CPU.R[cpu.AX])
		if len(opened) > maxHandles {
			t.Fatalf("開了 %d 個檔還沒滿——上限沒生效", len(opened))
		}
	}
	if m.CPU.R[cpu.AX] != 4 {
		t.Errorf("開太多檔時 AX=%04X，預期 4（too many open files）", m.CPU.R[cpu.AX])
	}
	for _, h := range opened {
		if h >= maxHandles {
			t.Errorf("配出 handle %d，超過上限 %d", h, maxHandles)
		}
	}
}
