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
	// AL=00h（載入並執行）與 AL=03h（覆疊）都真的做；AL=01h「載入但不跳」
	// 沒有實作，那一支要失敗即關閉並留下診斷。
	call(m, d, 0x21, 0x4B01)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 1 ||
		d.Unimplemented[Call{Int: 0x21, AH: 0x4B, AL: 1}] != 1 {
		t.Fatal("未支援的 EXEC 子功能沒有失敗即關閉並留下診斷")
	}
	// AL=00h 走真的 spawn：檔案不在就是缺檔，不是「不支援」。
	call(m, d, 0x21, 0x4B00)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 2 {
		t.Fatalf("AL=00h 缺檔應回 CF=1 AX=2，得到 CF=%t AX=%04X",
			m.CPU.Flags&cpu.CF != 0, m.CPU.R[cpu.AX])
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
	// int 16h 的 AX 是 AH＝掃描碼、AL＝ASCII。只填 AL 的話，用 AH 判鍵的
	// 程式會收到 0，而 0 是延伸鍵的前綴——那會被讀成方向鍵。
	const want4 = 0x05<<8 | '4' // '4' 的 set-1 掃描碼是 05h
	call(m, d, 0x16, 0x0100)
	if m.CPU.Flags&cpu.ZF != 0 || m.CPU.R[cpu.AX] != want4 || len(d.Stdin) != 1 {
		t.Fatalf("BIOS查鍵結果錯誤：AX=%04X ZF=%t pending=%d", m.CPU.R[cpu.AX], m.CPU.Flags&cpu.ZF != 0, len(d.Stdin))
	}
	call(m, d, 0x16, 0x0000)
	if m.CPU.R[cpu.AX] != want4 || len(d.Stdin) != 0 {
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

// TestMouseButtonStatsPerButton：`AX=5`／`AX=6` 的**輸入 BX 是按鍵編號**，
// 左右鍵各記各的。
//
// 對兩顆鍵回同一個計數的話，左鍵的按下會被輪詢右鍵的那一次取走——遊戲把它
// 當成右鍵（多半是「取消」），畫面上什麼也不會發生，而且沒有任何錯誤徵兆。
func TestMouseButtonStatsPerButton(t *testing.T) {
	m, d := newTest(t)
	d.Mouse.Press = [3]uint16{1, 0, 0} // 只按了左鍵

	m.CPU.R[cpu.BX] = 1 // 先問右鍵
	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.BX] != 0 {
		t.Fatalf("問右鍵回 BX=%d，預期 0——左鍵的按下被右鍵取走了", m.CPU.R[cpu.BX])
	}
	m.CPU.R[cpu.BX] = 0 // 再問左鍵
	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.BX] != 1 {
		t.Errorf("問左鍵回 BX=%d，預期 1", m.CPU.R[cpu.BX])
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
	m.SetVideoMode(0x13) // 320 寬
	d.Mouse.X, d.Mouse.Y = 100, 50
	call(m, d, 0x33, 0x0003)
	if len(d.Mouse.Polls) != 1 {
		t.Fatalf("輪詢了 1 次卻記了 %d 筆", len(d.Mouse.Polls))
	}
	// mode 13h 的標準驅動水平回報 0–639（`rich2/docs/re/182` §3）。
	if m.CPU.R[cpu.CX] != 200 || m.CPU.R[cpu.DX] != 50 {
		t.Errorf("mode 13h 回報 (%d,%d)，預期 (200,50)：水平要乘 2",
			m.CPU.R[cpu.CX], m.CPU.R[cpu.DX])
	}
}

// TestMouseScaleFollowsVideoMode：虛擬座標倍率要**跟著視訊模式走**，
// 不能寫死。
//
// 驅動的虛擬座標系固定 640 寬，所以 320 寬的模式回報值是像素的兩倍、
// 640 寬的模式是一比一。寫死成 2 的話，mode 12h 的遊戲收到的點擊會落在
// 兩倍遠的地方——而畫面完全正常，症狀只是「點了沒反應」，
// 看起來像遊戲卡住而不是座標錯（`~/cht/logh3/docs/re/08`）。
func TestMouseScaleFollowsVideoMode(t *testing.T) {
	for _, c := range []struct {
		mode    uint8
		x, want uint16
	}{
		{0x13, 100, 200}, {0x0D, 100, 200},
		{0x12, 300, 300}, {0x10, 300, 300}, {0x03, 300, 300},
	} {
		m, d := newTest(t)
		m.SetVideoMode(c.mode)
		d.Mouse.X, d.Mouse.Y = c.x, 40
		// ⚠ 用 `AX=3`（取目前位置）不是 `AX=5`：後者回的是**按下那一刻**
		// 的座標（`PressAt`），沒按過就是 0。
		call(m, d, 0x33, 0x0003)
		if got := m.CPU.R[cpu.CX]; got != c.want {
			t.Errorf("模式 %02Xh：X=%d 回報 %d，預期 %d", c.mode, c.x, got, c.want)
		}
	}
}

func TestMouseCallbackUsesFarCallContract(t *testing.T) {
	m, d := newTest(t)
	// 回呼的 CX 是**虛擬座標**（虛擬螢幕永遠 640 格寬），所以倍率跟著模式走：
	// 13h 是 320 寬 → 倍率 2。不設模式的話預設當 640 寬，倍率 1。
	m.SetVideoMode(0x13)
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

	dispatched, err := d.RunMouseEvent(0x0002, 3, -4)
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
	// SI＝水平、DI＝垂直 mickey（int 33h AX=000Ch 的回呼契約）。
	wantArgs := [6]uint16{2, 1, 200, 50, 3, 0xFFFC}
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

	dispatched := d.MouseEvent(0x0008)
	var err error
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
	if d.MouseEvent(0x0001) {
		t.Fatal("mask只有左鍵按下卻觸發移動callback")
	}
	call(m, d, 0x33, 0x0000)
	if d.MouseEvent(0x0002) {
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
	// AL=01h 設屬性不做（素材唯讀），但要**記一筆再清 CF**：
	// 看得見的假，不是安靜的假。回 CF 的話，寫存檔前先清唯讀位元的程式
	// 會把它讀成磁碟錯誤然後整支放棄。
	call(m, d, 0x21, 0x4301)
	if m.CPU.Flags&cpu.CF != 0 || d.Unimplemented[Call{Int: 0x21, AH: 0x43, AL: 1}] != 1 {
		t.Fatalf("屬性寫入沒有留下診斷：CF=%t 診斷=%d",
			m.CPU.Flags&cpu.CF != 0, d.Unimplemented[Call{Int: 0x21, AH: 0x43, AL: 1}])
	}
	m.WriteBytes(cpu.Addr(0x2200, 0x20), append([]byte("MISSING.EXE"), 0))
	dta := cpu.Addr(machine.PSPSeg, 0x80)
	m.Write8(dta, 0xA5)
	call(m, d, 0x21, 0x4E00)
	if m.CPU.Flags&cpu.CF == 0 || m.CPU.R[cpu.AX] != 18 || m.Read8(dta) != 0xA5 {
		t.Fatal("Find First缺檔契約錯誤或污染DTA")
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

// TestFreeBlockReclaims 釘住 `AH=49h` 真的把記憶體收回來。
//
// 反面是「後面的程式配不到記憶體」，而症狀不在這裡：DOSJP 先要 286 KB
// 讀字型、搬進 XMS、再還回來，不收的話 OPEN.EXE 的第四次 AH=48h 失敗，
// 然後它走記憶體不足的路徑飛掉（`docs/spec/004` §1.2.2）。
func TestFreeBlockReclaims(t *testing.T) {
	m, d := newTest(t)
	alloc := func(want uint16) uint16 {
		m.CPU.R[cpu.BX] = want
		call(m, d, 0x21, 0x4800)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("配 %d 段失敗（可用 %d）", want, m.CPU.R[cpu.BX])
		}
		return m.CPU.R[cpu.AX]
	}
	free := func(seg uint16) {
		m.CPU.Seg[cpu.ES] = seg
		call(m, d, 0x21, 0x4900)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("釋放 %04X 失敗", seg)
		}
	}

	// 頂端的區塊還回來之後，水位要降回去：同樣大小的下一次配置拿到同一段。
	a := alloc(0x1000)
	free(a)
	if b := alloc(0x1000); b != a {
		t.Errorf("頂端釋放後再配拿到 %04X，要 %04X——水位沒降回去", b, a)
	}

	// 中間的洞也要能再用。
	mid := alloc(0x200)
	top := alloc(0x100)
	free(mid)
	if got := alloc(0x200); got != mid {
		t.Errorf("中間的洞沒被再利用：拿到 %04X，要 %04X", got, mid)
	}
	if top == mid {
		t.Error("頂端的區塊被蓋掉了")
	}

	// 大塊配置、釋放、再要一塊更大的——DOSJP 的形狀。
	big := alloc(0x4000)
	free(big)
	if got := alloc(0x6000); got == 0 {
		t.Error("釋放 0x4000 段之後配不到 0x6000 段")
	}
}

// TestXMSEntryIsFarCallNotInterrupt 釘住 XMS driver entry 的 trampoline
// 結尾是 RETF，不是 IRET。
//
// 呼叫端用 `call far` 只推 CS:IP；IRET 會連 FLAGS 一起彈，每呼叫一次
// 堆疊歪兩個位元組。症狀出現在很後面：某個 RETF／IRET 跳進垃圾，
// 而中間每一次 XMS 服務都「成功」。
func TestXMSEntryIsFarCallNotInterrupt(t *testing.T) {
	m := machine.New()
	at := uint32(machine.StubSeg)*16 + machine.XMSTrapOff
	got := []byte{m.Read8(at), m.Read8(at + 1), m.Read8(at + 2)}
	want := []byte{0xCD, 0xF5, 0xCB}
	if string(got) != string(want) {
		t.Errorf("XMS entry ＝ % X，要 % X（CD F5 ＋ RETF）", got, want)
	}
}

// TestMCBChainMirrorsArena 釘住「配置器的狀態要發布到客體記憶體」。
//
// DOS 程式看得到 MCB 而且會走它。鏈沒同步的話程式**不會報錯**——
// 它只是沿著一份與事實無關的地圖算出一個數字，然後拿那個數字去做決定。
// 智冠《三國演義》就是這樣：走鏈算「總共構得到多少段」，拿到 0x2000
// （寫死的舊值）而不是 0x9EFF，於是判定載不下下一個模組，印完
// 「程式載入中 請稍待」就以 255 離開。
//
// 這條測試走的是真正的 int 21h 分派路徑，並且照程式的走法讀鏈：
// 從 PSP 的 MCB 開始，`+1` 是擁有者、`+3` 是段數，下一個 MCB 在
// `seg+1+size`，簽章 `M` continue／`Z` 收尾。
func TestMCBChainMirrorsArena(t *testing.T) {
	m, d := newTest(t)

	// 配三塊，再放掉中間那塊——鏈上就會有「自己的、自由的、自己的」。
	var segs []uint16
	for _, want := range []uint16{0x0100, 0x0080, 0x0040} {
		m.CPU.R[cpu.BX] = want
		call(m, d, 0x21, 0x4800)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("配置 %04X 段失敗", want)
		}
		segs = append(segs, m.CPU.R[cpu.AX])
	}
	m.CPU.Seg[cpu.ES] = segs[1]
	call(m, d, 0x21, 0x4900)
	if m.CPU.Flags&cpu.CF != 0 {
		t.Fatal("釋放中間那塊失敗")
	}

	// 照程式的方式走鏈。
	seg := uint16(machine.PSPSeg - 1)
	var total, blocks int
	var sawFree bool
	for {
		lin := uint32(seg) * 16
		sig := m.Read8(lin)
		owner := m.Read16(lin + 1)
		size := m.Read16(lin + 3)
		if sig != 'M' && sig != 'Z' {
			t.Fatalf("第 %d 個 MCB（段 %04X）簽章是 %02X，鏈斷了——"+
				"走鏈的程式會判定記憶體壞掉", blocks, seg, sig)
		}
		if owner != 0 && owner != machine.PSPSeg {
			t.Fatalf("段 %04X 的擁有者是 %04X，不是 0 也不是 PSP", seg, owner)
		}
		if owner == 0 {
			sawFree = true
		}
		blocks++
		total += int(size) + 1
		if sig == 'Z' {
			break
		}
		seg += 1 + size
		if blocks > 64 {
			t.Fatal("鏈走不完——沒有 Z 收尾")
		}
	}
	if !sawFree {
		t.Error("鏈上一塊自由區塊都沒有，剛剛才釋放過一塊")
	}
	// 鏈要**反映實際配置**，不是只要合法就好：三塊都要在鏈上找得到，
	// 大小與擁有權都要對。只檢查「鏈走得完」的話，載入時建的那條
	// 「一塊自己的 ＋ 一塊全部自由」也照樣過關。
	for i, want := range []uint16{0x0100, 0x0080, 0x0040} {
		lin := uint32(segs[i]-1) * 16
		gotSize := m.Read16(lin + 3)
		gotOwner := m.Read16(lin + 1)
		wantOwner := uint16(machine.PSPSeg)
		if i == 1 {
			wantOwner = 0 // 中間那塊已經釋放
		}
		if gotSize != want || gotOwner != wantOwner {
			t.Errorf("第 %d 塊（MCB 段 %04X）鏈上寫的是 size=%04X owner=%04X，"+
				"配置器裡是 size=%04X owner=%04X",
				i, segs[i]-1, gotSize, gotOwner, want, wantOwner)
		}
	}
	// 鏈要涵蓋 PSP 到傳統記憶體上緣：走完的段數要對得上。
	if got, want := machine.PSPSeg-1+total, int(machine.MemTop); got != want {
		t.Errorf("鏈的終點是 %04X，應該是 %04X（差 %d 段）", got, want, want-got)
	}
}

// TestMCBChainDoesNotClobberImage 釘住「MCB 不准寫進程式映像」。
//
// 舊版把鏈尾寫死在 `PSPSeg+0x2000`。映像超過 128 KB 的程式，那八個 byte
// 就落在自己的碼或資料中間——而且**當下什麼事都不會發生**。
func TestMCBChainDoesNotClobberImage(t *testing.T) {
	m, d := newTest(t)
	const canary = 0xA5
	for i := uint32(0); i < 16; i++ {
		m.Write8((machine.PSPSeg+0x2000)*16+i, canary)
	}
	m.CPU.R[cpu.BX] = 0x0010
	call(m, d, 0x21, 0x4800)
	for i := uint32(0); i < 16; i++ {
		if got := m.Read8((machine.PSPSeg+0x2000)*16 + i); got != canary {
			t.Fatalf("段 %04X 的第 %d 個 byte 被改成 %02X——"+
				"MCB 寫進了映像中間", machine.PSPSeg+0x2000, i, got)
		}
	}
}

// TestHandleNumbersAreReused 釘住「關掉的 handle 號碼要放回去」。
//
// DOS 的 handle 是 PSP 那張 job file table 的索引，關檔就把那格標成空，
// 下一次開檔拿最小的空格。只增不重用的話，一支開開關關幾十次的程式
// 會拿到越來越大的號碼——而 MSC 的低階 I/O 拿 handle 當自己表的索引，
// 表和 JFT 一樣大。號碼一超出範圍，`fopen` 會**開成功之後立刻關掉並回
// NULL**，症狀是「開得好好的檔突然開不起來」。
func TestHandleNumbersAreReused(t *testing.T) {
	m, d := newTest(t)
	for _, n := range []string{"A.DAT", "B.DAT", "C.DAT"} {
		if err := os.WriteFile(filepath.Join(d.Root, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	openFile := func(name string) uint16 {
		t.Helper()
		d.M.WriteBytes(cpu.Addr(0x2000, 0x100), append([]byte(name), 0))
		m.CPU.Seg[cpu.DS] = 0x2000
		m.CPU.R[cpu.DX] = 0x100
		call(m, d, 0x21, 0x3D00)
		if m.CPU.Flags&cpu.CF != 0 {
			t.Fatalf("開 %s 失敗，AX=%04X", name, m.CPU.R[cpu.AX])
		}
		return m.CPU.R[cpu.AX]
	}
	closeFile := func(h uint16) {
		t.Helper()
		m.CPU.R[cpu.BX] = h
		call(m, d, 0x21, 0x3E00)
	}

	first := openFile("A.DAT")
	closeFile(first)
	if got := openFile("B.DAT"); got != first {
		t.Fatalf("關掉 handle %d 之後再開拿到 %d——號碼沒有放回去", first, got)
	}
	// 佔滿到上限，第 21 個要以「開太多檔」失敗，不是拿到越界的號碼。
	for i := uint16(0); ; i++ {
		d.M.WriteBytes(cpu.Addr(0x2000, 0x100), append([]byte("C.DAT"), 0))
		m.CPU.Seg[cpu.DS] = 0x2000
		m.CPU.R[cpu.DX] = 0x100
		call(m, d, 0x21, 0x3D00)
		if m.CPU.Flags&cpu.CF != 0 {
			if m.CPU.R[cpu.AX] != 4 {
				t.Errorf("開太多檔應該回錯誤 4，回的是 %d", m.CPU.R[cpu.AX])
			}
			break
		}
		if h := m.CPU.R[cpu.AX]; h >= d.MaxHandles {
			t.Fatalf("拿到 handle %d，上限是 %d——越界的號碼會讓 MSC 的表爆掉",
				h, d.MaxHandles)
		}
		if i > 64 {
			t.Fatal("開不完——上限沒有生效")
		}
	}
}
