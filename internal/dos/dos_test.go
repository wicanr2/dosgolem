package dos

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

func testMZOverlay() []byte {
	const headerSize = 32
	body := make([]byte, 32)
	body[0], body[1] = 0x90, 0xF4
	out := make([]byte, headerSize+len(body))
	out[0], out[1] = 'M', 'Z'
	binary.LittleEndian.PutUint16(out[2:], uint16(len(out)))
	binary.LittleEndian.PutUint16(out[4:], 1)
	binary.LittleEndian.PutUint16(out[6:], 1)
	binary.LittleEndian.PutUint16(out[8:], 2)
	binary.LittleEndian.PutUint16(out[24:], 0x1C)
	binary.LittleEndian.PutUint16(out[0x1C:], 0x10)
	copy(out[headerSize:], body)
	return out
}

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

func TestExecOverlayLoadsMZAndReturnsToCaller(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "intro.exe"), testMZOverlay(), 0o600); err != nil {
		t.Fatal(err)
	}
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x2000, 0x20
	m.WriteBytes(cpu.Addr(0x2000, 0x20), append([]byte(`C:\RICH2\INTRO.EXE`), 0))
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0x2100, 0x30
	m.Write16(cpu.Addr(0x2100, 0x30), 0x3000)
	m.Write16(cpu.Addr(0x2100, 0x32), 0x1234)
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x4444, 0x5555
	call(m, d, 0x21, 0x4B03)

	if m.CPU.Flags&cpu.CF != 0 || m.CPU.R[cpu.AX] != 0 {
		t.Fatalf("覆疊載入失敗：CF=%t AX=%04X", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
	if got := m.Read16(0x3000*16 + 0x10); got != 0x1234 {
		t.Fatalf("覆疊重定位結果 %04X，預期 1234", got)
	}
	if m.CPU.Seg[cpu.CS] != 0x4444 || m.CPU.IP != 0x5555 {
		t.Fatal("EXEC 覆疊載入不該替呼叫端跳入映像")
	}
}

func TestExecOverlayRejectsMissingAndUnsupportedSubfunction(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x2000, 0x20
	m.WriteBytes(cpu.Addr(0x2000, 0x20), append([]byte("MISSING.EXE"), 0))
	call(m, d, 0x21, 0x4B03)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 2 {
		t.Fatalf("缺檔應回 CF=1 AX=2，得到 CF=%t AX=%04X", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
	call(m, d, 0x21, 0x4B00)
	if m.CPU.Flags&cpu.CF == 0 || d.Unimplemented[Call{Int: 0x21, AH: 0x4B, AL: 0}] != 1 {
		t.Fatal("未支援的 EXEC 子功能沒有失敗即關閉並留下診斷")
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

func TestBIOSKeyboardSharesReplayQueue(t *testing.T) {
	m, d := newTest(t)
	d.Stdin = []byte{'4'}
	m.CPU.SetFlags(m.CPU.Flags | cpu.ZF)
	call(m, d, 0x16, 0x0100)
	if m.CPU.Flags&cpu.ZF != 0 || m.CPU.R[cpu.AX] != '4' || len(d.Stdin) != 1 {
		t.Fatalf("BIOS查鍵結果錯誤：AX=%04X ZF=%t pending=%d", m.CPU.R[cpu.AX], m.CPU.Flags&cpu.ZF != 0, len(d.Stdin))
	}
	call(m, d, 0x16, 0x0000)
	if m.CPU.R[cpu.AX] != '4' || len(d.Stdin) != 0 {
		t.Fatalf("BIOS讀鍵結果錯誤：AX=%04X pending=%d", m.CPU.R[cpu.AX], len(d.Stdin))
	}
	call(m, d, 0x16, 0x0100)
	if m.CPU.Flags&cpu.ZF == 0 {
		t.Fatal("空鍵盤佇列查詢沒有設定ZF")
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

func TestBIOSReadTimeOfDay(t *testing.T) {
	m, d := newTest(t)
	const base = uint32(0x0040 * 16)
	m.Write16(base+0x6C, 0x5678)
	m.Write16(base+0x6E, 0x1234)
	m.Write8(base+0x70, 1)

	call(m, d, 0x1A, 0x0000)
	if got := m.CPU.R[cpu.CX]; got != 0x1234 {
		t.Fatalf("CX=%04X，預期1234", got)
	}
	if got := m.CPU.R[cpu.DX]; got != 0x5678 {
		t.Fatalf("DX=%04X，預期5678", got)
	}
	if got := uint8(m.CPU.R[cpu.AX]); got != 1 {
		t.Fatalf("AL=%02X，預期rollover 01", got)
	}
	if got := m.Read8(base + 0x70); got != 0 {
		t.Fatalf("rollover讀後=%02X，預期00", got)
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

func TestInt10SetDACBlock(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = 0x2200, 0x10
	m.CPU.R[cpu.BX], m.CPU.R[cpu.CX] = 2, 2
	m.WriteBytes(cpu.Addr(0x2200, 0x10), []byte{0x3F, 0x20, 0x00, 0x01, 0x02, 0x03})
	call(m, d, 0x10, 0x1012)
	pal := m.Palette()
	if pal[2] != [3]uint8{255, 130, 0} || pal[3] != [3]uint8{4, 8, 12} {
		t.Fatalf("DAC block錯誤：色2=%v 色3=%v", pal[2], pal[3])
	}
	if d.Unimplemented[Call{Int: 0x10, AH: 0x10, AL: 0x12}] != 0 {
		t.Fatal("已實作的1012h仍被列為未實作")
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

func TestMouseCallbackUsesFarCallContract(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.CX] = 0x0007
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = 0x0900, 0x0010
	call(m, d, 0x33, 0x000C)
	// callback先把六個事件參數寫進DS:0100，再RETF。
	callback := []byte{
		0xA3, 0x00, 0x01, // mov [0100],ax
		0x89, 0x1E, 0x02, 0x01, // mov [0102],bx
		0x89, 0x0E, 0x04, 0x01, // mov [0104],cx
		0x89, 0x16, 0x06, 0x01, // mov [0106],dx
		0x89, 0x36, 0x08, 0x01, // mov [0108],si
		0x89, 0x3E, 0x0A, 0x01, // mov [010A],di
		0xCB,
	}
	m.WriteBytes(cpu.Addr(0x0900, 0x0010), callback)
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x0800, 0x1234
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP] = 0x0700, 0x0100
	m.CPU.Seg[cpu.DS] = 0x0A00
	m.CPU.R[cpu.AX], m.CPU.R[cpu.BX], m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] =
		0xA111, 0xB222, 0xC333, 0xD444
	m.CPU.R[cpu.SI], m.CPU.R[cpu.DI], m.CPU.R[cpu.BP] = 0x5111, 0xD111, 0xB111
	m.CPU.SetFlags(cpu.IF | cpu.DF | cpu.CF)
	wantR, wantSeg, wantIP, wantFlags := m.CPU.R, m.CPU.Seg, m.CPU.IP, m.CPU.Flags
	d.Mouse.X, d.Mouse.Y, d.Mouse.Buttons = 100, 50, 1

	dispatched, err := d.MouseEvent(0x0002, 3, -4)
	if err != nil {
		t.Fatal(err)
	}
	if !dispatched {
		t.Fatal("符合mask的左鍵按下沒有觸發callback")
	}
	if m.CPU.R != wantR || m.CPU.Seg != wantSeg || m.CPU.IP != wantIP || m.CPU.Flags != wantFlags {
		t.Fatalf("callback返回後污染呼叫者狀態：R=%04X Seg=%04X IP=%04X Flags=%04X",
			m.CPU.R, m.CPU.Seg, m.CPU.IP, m.CPU.Flags)
	}
	wantArgs := [6]uint16{2, 1, 200, 50, 0xFFFC, 3}
	for i, want := range wantArgs {
		if got := m.Read16(cpu.Addr(0x0A00, 0x0100+uint16(i*2))); got != want {
			t.Fatalf("callback參數%d=%04X，預期%04X", i, got, want)
		}
	}
}

func TestMouseCallbackReportsRightButtonContract(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.CX] = 0x0018
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = 0x0900, 0x0010
	call(m, d, 0x33, 0x000C)
	m.Write8(cpu.Addr(0x0900, 0x0010), 0xCB)
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x0800, 0x1234
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP] = 0x0700, 0x0100
	d.Mouse.X, d.Mouse.Y, d.Mouse.Buttons = 100, 50, 2

	dispatched, err := d.MouseEvent(0x0008, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !dispatched {
		t.Fatal("符合mask的右鍵按下沒有觸發callback")
	}
}

func TestMouseCallbackHonorsMaskAndReset(t *testing.T) {
	m, d := newTest(t)
	m.CPU.R[cpu.CX] = 0x0002
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = 0x0900, 0x0010
	call(m, d, 0x33, 0x000C)
	if dispatched, err := d.MouseEvent(0x0001, 1, 1); err != nil || dispatched {
		t.Fatal("mask只有左鍵按下卻觸發移動callback")
	}
	call(m, d, 0x33, 0x0000)
	if dispatched, err := d.MouseEvent(0x0002, 0, 0); err != nil || dispatched {
		t.Fatal("reset後仍觸發callback")
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

func TestClosedFileHandleIsReused(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "data.pak"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x3000, 0
	m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte("DATA.PAK"), 0))
	for i := 0; i < 256; i++ {
		call(m, d, 0x21, 0x3D00)
		if m.CPU.Flags&cpu.CF != 0 || m.CPU.R[cpu.AX] != 5 {
			t.Fatalf("第%d次open handle=%d CF=%t，預期重用5", i, m.CPU.R[cpu.AX], m.CPU.Flags&cpu.CF != 0)
		}
		m.CPU.R[cpu.BX] = 5
		call(m, d, 0x21, 0x3E00)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("第%d次close失敗", i)
		}
	}
}

func TestOpenFailsWhenDefaultHandleTableIsFull(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "data.pak"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x3000, 0
	m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte("DATA.PAK"), 0))
	for h := uint16(5); h < 20; h++ {
		call(m, d, 0x21, 0x3D00)
		if m.CPU.Flags&cpu.CF != 0 || m.CPU.R[cpu.AX] != h {
			t.Fatalf("handle=%d，預期%d", m.CPU.R[cpu.AX], h)
		}
	}
	call(m, d, 0x21, 0x3D00)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 4 {
		t.Fatalf("JFT滿時CF=%t AX=%d，預期CF=1 AX=4", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
}

func TestFileWritesRequireExplicitAllowlist(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		t.Run(fmt.Sprintf("allowed=%t", allowed), func(t *testing.T) {
			m, d := newTest(t)
			path := filepath.Join(d.Root, "save.dat")
			if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
				t.Fatal(err)
			}
			if allowed {
				if err := d.AllowFileWrites("SAVE.DAT"); err != nil {
					t.Fatal(err)
				}
			}
			m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x3000, 0
			m.WriteBytes(cpu.Addr(0x3000, 0), append([]byte("SAVE.DAT"), 0))
			call(m, d, 0x21, 0x3D02)
			h := m.CPU.R[cpu.AX]
			m.CPU.R[cpu.BX], m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = h, 3, 0x40
			m.WriteBytes(cpu.Addr(0x3000, 0x40), []byte("NEW"))
			call(m, d, 0x21, 0x4000)
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := "hello"
			if allowed {
				want = "NEWlo"
			}
			if string(got) != want {
				t.Fatalf("內容=%q，預期%q", got, want)
			}
		})
	}
	if _, d := newTest(t); d.AllowFileWrites("../save.dat") == nil {
		t.Fatal("路徑逃逸應失敗")
	}
	_, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "save.dat"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.AllowFileWrites("SAVE.DAT", "../bad.dat"); err == nil || len(d.writableFiles) != 0 {
		t.Fatalf("allowlist失敗必須原子回滾：err=%v files=%v", err, d.writableFiles)
	}
}

func TestDTASetGetAndFindFirstExactFile(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "eob.exe"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x2200, 0x40
	call(m, d, 0x21, 0x1A00)
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX] = 0, 0
	call(m, d, 0x21, 0x2F99)
	if m.CPU.Seg[cpu.ES] != 0x2200 || m.CPU.R[cpu.BX] != 0x40 {
		t.Fatalf("DTA=%04X:%04X", m.CPU.Seg[cpu.ES], m.CPU.R[cpu.BX])
	}
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x2300, 0x20
	m.WriteBytes(cpu.Addr(0x2300, 0x20), append([]byte(`C:\RICH2\EOB.EXE`), 0))
	m.CPU.R[cpu.CX] = 0x37
	call(m, d, 0x21, 0x4E80)
	if m.CPU.Flags&cpu.CF != 0 || m.CPU.R[cpu.AX] != 0 {
		t.Fatalf("Find First失敗：CF=%t AX=%d", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
	}
	base := cpu.Addr(0x2200, 0x40)
	if got := m.Read8(base + 0x15); got != 0x20 {
		t.Fatalf("attribute=%02X", got)
	}
	if got := uint32(m.Read16(base+0x1A)) | uint32(m.Read16(base+0x1C))<<16; got != 5 {
		t.Fatalf("size=%d", got)
	}
	name := make([]byte, 7)
	for i := range name {
		name[i] = m.Read8(base + 0x1E + uint32(i))
	}
	if string(name) != "EOB.EXE" {
		t.Fatalf("name=%q", name)
	}
}

func TestFileAttributesAndFindFirstFailClosed(t *testing.T) {
	m, d := newTest(t)
	if err := os.WriteFile(filepath.Join(d.Root, "intro.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.CPU.Seg[cpu.DS], m.CPU.R[cpu.DX] = 0x2200, 0x20
	m.WriteBytes(cpu.Addr(0x2200, 0x20), append([]byte("INTRO.EXE"), 0))
	call(m, d, 0x21, 0x4300)
	if m.CPU.Flags&cpu.CF != 0 || m.CPU.R[cpu.CX] != 0x20 {
		t.Fatalf("attributes CF=%t CX=%04X", m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.CX])
	}
	call(m, d, 0x21, 0x4301)
	if m.CPU.Flags&cpu.CF == 0 || d.Unimplemented[Call{Int: 0x21, AH: 0x43, AL: 1}] != 1 {
		t.Fatal("未支援屬性寫入沒有失敗即關閉")
	}
	m.WriteBytes(cpu.Addr(0x2200, 0x20), append([]byte("MISSING.EXE"), 0))
	dta := cpu.Addr(machine.PSPSeg, 0x80)
	m.Write8(dta, 0xA5)
	call(m, d, 0x21, 0x4E00)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 18 || m.Read8(dta) != 0xA5 {
		t.Fatal("Find First缺檔契約錯誤或污染DTA")
	}
}

// TestMissingFileIsRecorded 釘住「找不到的檔要留名字」。
//
// 只回 CF 的話，缺一個資產與「程式自己決定不載」看起來一樣。
func TestMissingFileIsRecorded(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.CS], m.CPU.IP = 0x1234, 0x5678
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.BP] = 0x2200, 0x0100
	m.Write16(cpu.Addr(0x2200, 0x0100), 0)
	m.Write16(cpu.Addr(0x2200, 0x0102), 0x2222)
	m.Write16(cpu.Addr(0x2200, 0x0104), 0x3333)
	m.Write16(cpu.Addr(0x2200, 0x0106), 0x4444)
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
	if len(d.MissingAccess) != 1 {
		t.Fatalf("MissingAccess=%v", d.MissingAccess)
	}
	a := d.MissingAccess[0]
	if a.Name != "NOPE.PAK" || a.CS != 0x1234 || a.IP != 0x5678 || a.DS != 0x3000 || a.DX != 0 || a.SS != 0x2200 || a.BP != 0x0100 {
		t.Fatalf("MissingAccess定位=%+v", a)
	}
	if f := a.Callers[0]; f.BP != 0x0100 || f.IP != 0x2222 || f.CS != 0x3333 || f.Args[0] != 0x4444 {
		t.Fatalf("MissingAccess frame=%+v", f)
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
