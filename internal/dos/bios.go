package dos

import (
	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// BIOS 服務：`int 10h` 視訊、`int 33h` 滑鼠、`int 16h` 鍵盤。

// int10 是 BIOS 視訊服務（`docs/spec/004` §3）。
//
// `RUN.EXE` 開場會做一整套顯示卡偵測。**回傳值不對就判定環境不支援然後
// 退出，而且退出訊息不走 `int 21h`**，所以主控台攔截收不到
// （`rich2/docs/re/005` §12）。這裡讓它相信自己在一張標準 VGA 上。
func (d *DOS) int10(c *cpu.CPU) {
	fn := ah(c)
	switch fn {
	case 0x00: // 設視訊模式
		// 記進 BDA。一直回 3 的話，程式設了 mode 13h 之後再查會以為沒設成功。
		d.M.SetVideoMode(al(c) & 0x7F)

	case 0x0E: // TTY 輸出
		if al(c) >= 0x20 {
			d.Console = append(d.Console, al(c))
		}

	case 0x0F: // 取目前視訊模式
		mode := d.M.VideoMode()
		cols := uint8(80)
		switch mode {
		case 0x00, 0x01, 0x04, 0x05, 0x0D, 0x13:
			cols = 40
		}
		setAH(c, cols)
		setAL(c, mode)
		setBH(c, 0) // 顯示頁

	case 0x11: // 取字型指標 → ES:BP
		if al(c) == 0x30 {
			c.Seg[cpu.ES] = machine.StubSeg
			c.R[cpu.BP] = 0
			c.R[cpu.CX] = 16 // 每字元掃描線數
			c.R[cpu.DX] = 24 // 螢幕列數 − 1
		}

	case 0x12: // EGA/VGA 替代選擇
		// ⚠ **子功能選擇子在 `BL`，不是 `AL`。**
		// 查 `AL` 的話那個分支永遠不成立（`docs/spec/004` §3）。
		switch bl(c) {
		case 0x10: // 取 EGA 資訊
			setBH(c, 0x00)       // 彩色模式
			setBL(c, 0x03)       // 記憶體 256 KB
			c.R[cpu.CX] = 0x0009 // 功能位元／切換設定
		case 0x20, 0x30, 0x31, 0x32, 0x33, 0x34:
			setAL(c, 0x12) // 表示有支援
		}

	case 0x1A: // 取顯示卡組合碼
		// **BASIC runtime 用這支判斷能不能 `SCREEN 13`。**
		// 沒實作 → `BL` 是垃圾 → runtime 認定不是 VGA → `SCREEN 13` 回
		// Illegal function call，而症狀完全不指向這裡。
		setAL(c, 0x1A) // 表示本服務有支援
		setBL(c, 0x08) // 使用中：VGA 彩色
		setBH(c, 0x00) // 替代：無

	case 0x1B: // 取功能／狀態資訊表 → ES:DI
		table := make([]byte, 64)
		table[0x25] = 0x08 // 顯示卡代碼：VGA 彩色
		table[0x29] = 0x03 // 記憶體大小：256 KB
		d.M.WriteBytes(cpu.Addr(c.Seg[cpu.ES], c.R[cpu.DI]), table)
		setAL(c, 0x1B) // 表示本服務有支援

	case 0x10:
		if al(c) != 0x12 {
			d.noteCPU(c, 0x10, fn, al(c))
			break
		}
		// 設一段DAC色彩暫存器：BL起始色號、CX色數、ES:DX為RGB三元組。
		// 色值是VGA原生6-bit；交給Machine的DAC狀態機統一遮罩與遞增。
		d.M.Out8(0x3C8, bl(c))
		addr := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.DX])
		for i := uint32(0); i < uint32(c.R[cpu.CX])*3; i++ {
			d.M.Out8(0x3C9, d.M.Read8(addr+i))
		}
	case 0x02, 0x03, 0x05, 0x06, 0x09, 0x0A:
		// 設游標／取游標／設頁／捲動／寫字元／調色盤：收下就好，
		// 呼叫端不看回傳值。

	default:
		d.note(0x10, fn, al(c))
	}
}

// int33 是滑鼠驅動（`docs/spec/004` §4，出處 `rich2/docs/re/182`）。
//
// **防拷畫面只吃滑鼠**：`rich2/docs/playtest/001` §3 記著鍵盤全都無效。
// 沒有這支的話遊戲偵測不到滑鼠，整個密碼畫面就沒有任何可用輸入——
// 而且不會有錯誤訊息，看起來就只是「卡住」。
func (d *DOS) int33(c *cpu.CPU) {
	m := &d.Mouse
	// **先把功能號存起來**：下面好幾個分支會覆寫 AX，之後再拿 AX 判斷
	// 就是在讀自己剛寫進去的值。（同一個形狀在 CPU 的 `PUSH SP` 上踩過。）
	fn := c.R[cpu.AX]
	if m.Calls != nil {
		m.Calls[fn]++
	}
	switch fn {
	case 0x0000: // 重設並偵測
		c.R[cpu.AX] = 0xFFFF // 已安裝
		c.R[cpu.BX] = 2      // 兩個鍵
		m.Buttons = 0

	case 0x0001, 0x0002: // 顯示／隱藏游標
		// 遊戲**從來不叫 `AX=1`**（全檔 0 個呼叫端），畫面上那隻小手是
		// 它自己畫的，所以這裡不必真的畫游標（`rich2/docs/re/182` §4.1）。

	case 0x0003: // 取位置與鍵狀態
		// **記下每次輪詢回報出去的東西。** 這是分辨「輸入沒送到」與
		// 「送到了但答錯」的唯一辦法——兩者的畫面表現一模一樣。
		m.Polls = append(m.Polls, Poll{X: m.X, Y: m.Y, Buttons: m.Buttons,
			Step: d.M.Steps})
		c.R[cpu.BX] = m.Buttons
		c.R[cpu.CX] = m.X * m.XScale
		c.R[cpu.DX] = m.Y

	case 0x0004: // 設位置
		if m.XScale > 0 {
			m.X = c.R[cpu.CX] / m.XScale
		}
		m.Y = c.R[cpu.DX]
		m.Sets = append(m.Sets, Poll{X: m.X, Y: m.Y, Step: d.M.Steps})

	case 0x0005, 0x0006: // 按下／放開的統計
		c.R[cpu.AX] = m.Buttons
		if fn == 0x0005 {
			c.R[cpu.BX] = m.Press
			m.Press = 0
		} else {
			c.R[cpu.BX] = m.Release
			m.Release = 0
		}
		c.R[cpu.CX] = m.X * m.XScale
		c.R[cpu.DX] = m.Y

	case 0x0007, 0x0008: // 設水平／垂直範圍：收下就好
	default:
		d.note(0x33, uint8(fn>>8), uint8(fn))
	}
}

// int16 是 BIOS 鍵盤。
//
// Rich2 的鍵盤輸入走 `int 21h AH=3Fh`，但其他DOS程式可能使用這條；兩者共用
// Stdin佇列，讓同一個可重播輸入來源不必知道程式採哪一種介面。
func (d *DOS) int16(c *cpu.CPU) {
	switch ah(c) {
	case 0x00, 0x10: // 讀按鍵（阻塞）
		if len(d.Stdin) == 0 {
			c.R[cpu.AX] = 0
			return
		}
		c.R[cpu.AX] = uint16(d.Stdin[0])
		d.Stdin = d.Stdin[1:]
	case 0x01, 0x11: // 查有沒有按鍵：不消耗佇列
		if len(d.Stdin) == 0 {
			c.SetFlags(c.Flags | cpu.ZF)
			return
		}
		c.R[cpu.AX] = uint16(d.Stdin[0])
		c.SetFlags(c.Flags &^ cpu.ZF)
	case 0x02, 0x12: // 取旗標狀態
		setAL(c, 0)
	default:
		d.note(0x16, ah(c), al(c))
	}
}

// int13 是 BIOS 磁碟服務。
//
// 遊戲用它做**防拷檢查**（`rich2/docs/playtest/001`：開場要輸入密碼）。
// 這裡一律回成功、狀態 0——真正的防拷邏輯在程式自己那邊。
func (d *DOS) int13(c *cpu.CPU) {
	switch ah(c) {
	case 0x00, 0x04: // 重設磁碟系統／驗證磁區
		setAH(c, 0)
		clearCarry(c)
	case 0x01: // 取上一次的狀態
		setAH(c, 0)
		clearCarry(c)
	default:
		// 讀寫磁區沒實作：**回失敗**，不要假裝成功。
		// 假裝成功的話呼叫端會拿沒填過的緩衝區當資料用。
		d.note(0x13, ah(c), al(c))
		setAH(c, 0x80) // 逾時
		setCarry(c)
	}
}
