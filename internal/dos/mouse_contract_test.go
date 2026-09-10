package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// `int 33h` 與事件回呼的獨立契約測試。
//
// 期望值照 Microsoft Mouse 的介面自己列，不引用分支的既有測試。
// 滑鼠這一層錯了通常只表現成「點了沒反應」，而那與「座標不對」、
// 「遊戲還沒準備好」在畫面上完全一樣。

// 虛擬座標永遠是 640 格寬：320 寬的模式回報值是像素的兩倍。
//
// 寫死倍率的話，640 寬的畫面上點右半邊會回報成超出畫面的座標，
// 遊戲**判定不在任何按鈕上而安靜地什麼都不做**。
func TestMouseVirtualCoordinatesFollowVideoMode(t *testing.T) {
	for _, c := range []struct {
		mode  uint8
		x     uint16
		wantX uint16
	}{
		{0x13, 100, 200}, // 320 寬 → 倍率 2
		{0x12, 100, 100}, // 640 寬 → 倍率 1
	} {
		m, d := newTest(t)
		m.SetVideoMode(c.mode)
		d.Mouse.X, d.Mouse.Y = c.x, 50
		call(m, d, 0x33, 0x0003)
		if m.CPU.R[cpu.CX] != c.wantX {
			t.Errorf("mode %02X 的 X 回報成 %d，預期 %d", c.mode, m.CPU.R[cpu.CX], c.wantX)
		}
		if m.CPU.R[cpu.DX] != 50 {
			t.Errorf("mode %02X 的 Y 回報成 %d，預期 50", c.mode, m.CPU.R[cpu.DX])
		}
	}
}

// `AX=0003h` 的 BX 是**目前按著哪幾個鍵**的位元遮罩（bit0 左、bit1 右）。
func TestMouseButtonStateIsABitmask(t *testing.T) {
	m, d := newTest(t)
	d.PressMouse(1) // 右鍵
	call(m, d, 0x33, 0x0003)
	if m.CPU.R[cpu.BX]&0x02 == 0 {
		t.Errorf("按著右鍵時 BX=%04X，bit1 該是 1", m.CPU.R[cpu.BX])
	}
	d.ReleaseMouse(1)
	call(m, d, 0x33, 0x0003)
	if m.CPU.R[cpu.BX]&0x02 != 0 {
		t.Errorf("放開之後 BX=%04X，bit1 該是 0", m.CPU.R[cpu.BX])
	}
}

// `AX=0005h`／`0006h` 的 **BX 進去是「問哪一個鍵」、出來才是次數**，
// 而且回的是**按下那一刻**的座標。
//
// 不分鍵的實作會把左鍵那一次交給問右鍵的呼叫端，於是每一次左鍵點擊都被
// 讀成右鍵——畫面上看起來像「點什麼都是取消」，而每個回傳值單獨看都合法。
func TestPressCountsArePerButtonAndRememberPosition(t *testing.T) {
	m, d := newTest(t)
	m.SetVideoMode(0x13)

	d.MoveMouse(10, 20)
	d.PressMouse(0) // 左鍵在 (10,20)
	d.MoveMouse(200, 100)

	m.CPU.R[cpu.BX] = 1 // 先問右鍵
	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.BX] != 0 {
		t.Fatalf("問右鍵回 %d 次按下——左鍵那一次被右鍵領走了", m.CPU.R[cpu.BX])
	}
	m.CPU.R[cpu.BX] = 0 // 再問左鍵
	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.BX] != 1 {
		t.Fatalf("問左鍵回 %d 次按下，預期 1", m.CPU.R[cpu.BX])
	}
	if m.CPU.R[cpu.CX] != 20 || m.CPU.R[cpu.DX] != 20 {
		t.Errorf("回的是 (%d,%d)，預期按下那一刻的 (20,20)（X 已乘倍率 2）",
			m.CPU.R[cpu.CX], m.CPU.R[cpu.DX])
	}
	// 次數讀過就歸零。
	m.CPU.R[cpu.BX] = 0
	call(m, d, 0x33, 0x0005)
	if m.CPU.R[cpu.BX] != 0 {
		t.Errorf("再問一次還回 %d，預期 0（讀過歸零）", m.CPU.R[cpu.BX])
	}
}

// 事件回呼是**排進佇列**，在下一個指令邊界才真的遠呼叫。
//
// 從外面（測試腳本、oracle）呼叫的那一刻不在指令邊界上；當場改 CS:IP
// 會把程式丟到常式裡而堆疊上沒有正確的返回位址。
func TestMouseCallbackFiresAtAnInstructionBoundary(t *testing.T) {
	m, d := newTest(t)
	const hSeg, mainSeg = 0x3000, 0x3100
	m.WriteBytes(cpu.Addr(hSeg, 0), []byte{0xCB})          // retf
	m.WriteBytes(cpu.Addr(mainSeg, 0), []byte{0xEB, 0xFE}) // jmp $
	m.CPU.Seg[cpu.CS], m.CPU.IP = mainSeg, 0
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP] = 0x3200, 0x100

	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = hSeg, 0
	m.CPU.R[cpu.CX] = EventMove
	call(m, d, 0x33, 0x000C)

	if !d.MouseEvent(EventMove) {
		t.Fatal("符合遮罩的事件沒有排進去")
	}
	if m.CallbackPending() != 1 {
		t.Fatalf("排隊中的回呼有 %d 個，預期 1", m.CallbackPending())
	}
	if m.CPU.Seg[cpu.CS] != mainSeg || m.CPU.IP != 0 {
		t.Fatal("排隊的那一刻就把 CS:IP 改掉了")
	}
	// 跑幾步讓它發出去再回來。
	for i := 0; i < 8 && m.CallbacksMade() == 0; i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.CallbacksMade() != 1 {
		t.Fatal("回呼沒有發出去")
	}
	for i := 0; i < 8 && m.CallbackActive(); i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.CallbackActive() {
		t.Fatal("回呼跑完了卻沒有收尾——哨兵中斷沒有被接到")
	}
	if m.CPU.Seg[cpu.CS] != mainSeg || m.CPU.IP != 0 {
		t.Errorf("回呼返回之後 CS:IP ＝ %04X:%04X，預期回到 %04X:0000",
			m.CPU.Seg[cpu.CS], m.CPU.IP, mainSeg)
	}
}

// 回呼**不能弄髒被打斷的那段程式的暫存器**。
//
// 真機上這一支是從驅動的 ISR 裡呼叫的，ISR 會把暫存器全部存起來。
// 我們插進去沒存的話，被打斷的程式會莫名其妙拿到別的 AX，
// 而症狀出現在很後面、完全不指向滑鼠。
func TestMouseCallbackRestoresCallerRegisters(t *testing.T) {
	m, d := newTest(t)
	const hSeg, mainSeg = 0x3000, 0x3100
	// 常式故意 clobber 四個暫存器再 retf。
	m.WriteBytes(cpu.Addr(hSeg, 0), []byte{
		0xB8, 0x34, 0x12, // mov ax,1234h
		0xBB, 0x78, 0x56, // mov bx,5678h
		0xB9, 0xCD, 0xAB, // mov cx,ABCDh
		0xBA, 0x21, 0x43, // mov dx,4321h
		0xCB,
	})
	m.WriteBytes(cpu.Addr(mainSeg, 0), []byte{0xEB, 0xFE})
	m.CPU.Seg[cpu.CS], m.CPU.IP = mainSeg, 0
	m.CPU.Seg[cpu.SS], m.CPU.R[cpu.SP] = 0x3200, 0x100
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = hSeg, 0
	m.CPU.R[cpu.CX] = EventMove
	call(m, d, 0x33, 0x000C)

	m.CPU.R[cpu.AX], m.CPU.R[cpu.BX] = 0x1111, 0x2222
	m.CPU.R[cpu.CX], m.CPU.R[cpu.DX] = 0x3333, 0x4444
	m.CPU.R[cpu.SI], m.CPU.R[cpu.DI] = 0x5555, 0x6666
	wantR, wantSeg, wantFlags := m.CPU.R, m.CPU.Seg, m.CPU.Flags

	if !d.MouseEvent(EventMove) {
		t.Fatal("事件沒有排進去")
	}
	for i := 0; i < 32 && (m.CallbacksMade() == 0 || m.CallbackActive()); i++ {
		if err := m.Step(); err != nil {
			t.Fatal(err)
		}
	}
	if m.CallbackActive() {
		t.Fatal("回呼沒有收尾")
	}
	if m.CPU.R != wantR || m.CPU.Seg != wantSeg || m.CPU.Flags != wantFlags {
		t.Errorf("回呼污染了呼叫端：R=%04X Seg=%04X Flags=%04X",
			m.CPU.R, m.CPU.Seg, m.CPU.Flags)
	}
}

// 遮罩沒開的事件不排隊。回 false **不代表出錯**——多數程式只登記自己在意的那幾種。
func TestMouseEventMaskFiltersWithoutQueueing(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = 0x3000, 0
	m.CPU.R[cpu.CX] = EventLeftDown // 只登記左鍵按下
	call(m, d, 0x33, 0x000C)

	if d.MouseEvent(EventMove) {
		t.Error("遮罩沒開的移動事件竟然排進去了")
	}
	if m.CallbackPending() != 0 {
		t.Errorf("排了 %d 個回呼", m.CallbackPending())
	}
	if !d.MouseEvent(EventLeftDown) {
		t.Error("遮罩開著的事件沒有排進去")
	}
}

// `AX=0000h`（重設）要回「已安裝」與鍵數，而且**把登記的回呼清掉**。
//
// 不清的話，重設之後的程式仍然會收到上一輪登記的常式的呼叫——
// 那個位址通常已經被別的東西蓋掉了。
func TestMouseResetReportsInstalledAndClearsHandler(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = 0x3000, 0
	m.CPU.R[cpu.CX] = 0xFFFF
	call(m, d, 0x33, 0x000C)

	call(m, d, 0x33, 0x0000)
	if m.CPU.R[cpu.AX] != 0xFFFF {
		t.Errorf("AX=%04X，預期 FFFF（已安裝）", m.CPU.R[cpu.AX])
	}
	if m.CPU.R[cpu.BX] != 2 {
		t.Errorf("BX=%d，預期 2（兩個鍵）", m.CPU.R[cpu.BX])
	}
	if d.MouseEvent(EventMove) {
		t.Error("重設之後還會呼叫舊的事件常式")
	}
}

// `AX=0014h`（交換使用者中斷向量）設新的、回舊的，一次做完。
//
// **用這一支的程式一次都不叫 `AX=000Ch`**（《Dungeon Master》DOS 版
// 4 億道指令裡 `000Ch` 0 次、`0014h` 5 次），所以只做 `000Ch` 等於完全沒做。
// 見 `docs/spec/194`。
func TestMouseSwapUserInterruptVectorsReturnsPrevious(t *testing.T) {
	m, d := newTest(t)

	// 沒登錄過時回 0：真驅動回它自己的預設常式，而那個位址在我們這裡
	// 不存在——回 0 讓「把舊值裝回去」等價於停用。
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = 0x3000, 0x0100
	m.CPU.R[cpu.CX] = 0x001F
	call(m, d, 0x33, 0x0014)
	if m.CPU.Seg[cpu.ES] != 0 || m.CPU.R[cpu.DX] != 0 || m.CPU.R[cpu.CX] != 0 {
		t.Errorf("沒登錄過時回 ES:DX=%04X:%04X CX=%04X，預期全 0",
			m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX], m.CPU.R[cpu.CX])
	}
	if !d.Mouse.Handler.Set || d.Mouse.Handler.Seg != 0x3000 ||
		d.Mouse.Handler.Off != 0x0100 || d.Mouse.Handler.Mask != 0x001F {
		t.Errorf("新的那一組沒生效：%+v", d.Mouse.Handler)
	}

	// 換第二組，回的是第一組。
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = 0x4000, 0x0200
	m.CPU.R[cpu.CX] = 0x0008
	call(m, d, 0x33, 0x0014)
	if m.CPU.Seg[cpu.ES] != 0x3000 || m.CPU.R[cpu.DX] != 0x0100 ||
		m.CPU.R[cpu.CX] != 0x001F {
		t.Errorf("回 ES:DX=%04X:%04X CX=%04X，預期 3000:0100 CX=001F",
			m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX], m.CPU.R[cpu.CX])
	}

	// 再換回第一組，回的是第二組——證明舊值是在覆寫之前取出來的，
	// 不是把呼叫端剛傳進來的那一份原樣回去。
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = 0x3000, 0x0100
	m.CPU.R[cpu.CX] = 0x001F
	call(m, d, 0x33, 0x0014)
	if m.CPU.Seg[cpu.ES] != 0x4000 || m.CPU.R[cpu.DX] != 0x0200 ||
		m.CPU.R[cpu.CX] != 0x0008 {
		t.Errorf("回 ES:DX=%04X:%04X CX=%04X，預期 4000:0200 CX=0008",
			m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX], m.CPU.R[cpu.CX])
	}
}

// `AX=0014h` 登錄的常式與 `AX=000Ch` 登錄的走同一條派送路徑。
func TestMouseSwapRegisteredHandlerReceivesEvents(t *testing.T) {
	m, d := newTest(t)
	m.CPU.Seg[cpu.ES], m.CPU.R[cpu.DX] = 0x3000, 0
	m.CPU.R[cpu.CX] = 0xFFFF
	call(m, d, 0x33, 0x0014)
	if !d.MouseEvent(EventMove) {
		t.Error("0014h 登錄的常式收不到事件")
	}

	// CX=0 是停用，與 000Ch 同。
	m.CPU.R[cpu.CX] = 0
	call(m, d, 0x33, 0x0014)
	if d.MouseEvent(EventMove) {
		t.Error("CX=0 之後還在派送事件")
	}
}
