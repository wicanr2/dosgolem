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
		if mode := al(c) & 0x7F; mode == 0x10 || mode == 0x12 {
			loadRGBrgbDAC(d.M.DAC[:])
		}

	case 0x0C: // 寫像素：AL ＝ 色號、CX ＝ X、DX ＝ Y
		// **有程式真的用 BIOS 畫點**（慢，但存在；標題畫面與工具程式尤其）。
		// 不做的話畫面整片不出現，而程式一路跑到底——看起來像它沒有畫。
		//
		// AL 的 bit7 是 XOR 模式（CGA／EGA 的定義）。
		d.putPixel(al(c), c.R[cpu.CX], c.R[cpu.DX])

	case 0x0D: // 讀像素：CX ＝ X、DX ＝ Y → AL ＝ 色號
		setAL(c, d.getPixel(c.R[cpu.CX], c.R[cpu.DX]))

	case 0x13: // 寫字串：ES:BP ＝ 字串、CX ＝ 長度、AL ＝ 模式
		// 模式 bit0 ＝ 寫完更新游標、bit1 ＝ 字串裡帶屬性位元組。
		// 我們沒有真的文字游標，只把字元收進 Console——那是「程式對我們
		// 說了什麼」的管道，漏掉的話錯誤訊息就消失了。
		step := uint16(1)
		if al(c)&0x02 != 0 {
			step = 2
		}
		for i := uint16(0); i < c.R[cpu.CX]; i++ {
			ch := d.M.Read8(cpu.Addr(c.Seg[cpu.ES], c.R[cpu.BP]+i*step))
			if ch >= 0x20 || ch == '\r' || ch == '\n' {
				d.Console = append(d.Console, ch)
			}
		}

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

	case 0x10: // 調色盤（`docs/spec/011`）
		d.PalOps = append(d.PalOps, PalOp{AL: al(c), BX: c.R[cpu.BX],
			CX: c.R[cpu.CX], DX: c.R[cpu.DX], ES: c.Seg[cpu.ES], Step: d.M.Steps})
		// ⚠ 這一支的每個 AL 語意都不一樣，而且**屬性調色盤與 DAC 是兩層
		// 不同的東西**：AL=00/01/02/07/08/09 動的是 16 色的屬性暫存器
		// （4 位元色號 → 6 位元 DAC 索引），AL=10/12/15/17 動的才是 DAC。
		// 接錯不會報錯，只會讓顏色安靜地變成別的東西。
		addr := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.DX])
		switch al(c) {
		case 0x00: // 設單一屬性暫存器：BL ＝ 索引、BH ＝ 值
			if bl(c) < 16 {
				d.M.VGA.SetPal(int(bl(c)), bh(c)&0x3F)
			}
		case 0x01: // 設 overscan：BH ＝ 值
			d.M.VGA.SetOverscan(bh(c) & 0x3F)
		case 0x02: // 設整份屬性調色盤：ES:DX → 16 個暫存器 ＋ overscan
			for i := 0; i < 16; i++ {
				d.M.VGA.SetPal(i, d.M.Read8(addr+uint32(i))&0x3F)
			}
			d.M.VGA.SetOverscan(d.M.Read8(addr+16) & 0x3F)
		case 0x07: // 讀單一屬性暫存器：BL ＝ 索引 → BH
			if bl(c) < 16 {
				setBH(c, d.M.VGA.Pal(int(bl(c))))
			}
		case 0x08: // 讀 overscan → BH
			setBH(c, d.M.VGA.Overscan())
		case 0x09: // 讀整份屬性調色盤 → ES:DX 的 16 ＋ 1 bytes
			for i := 0; i < 16; i++ {
				d.M.Write8(addr+uint32(i), d.M.VGA.Pal(i))
			}
			d.M.Write8(addr+16, d.M.VGA.Overscan())
		case 0x10: // 設**單一 DAC**：BX ＝ 索引、DH ＝ R、CH ＝ G、CL ＝ B
			i := int(c.R[cpu.BX]) & 0xFF
			d.M.DAC[i*3+0] = uint8(c.R[cpu.DX]>>8) & 0x3F
			d.M.DAC[i*3+1] = uint8(c.R[cpu.CX]>>8) & 0x3F
			d.M.DAC[i*3+2] = uint8(c.R[cpu.CX]) & 0x3F
		case 0x12: // 設一段 DAC：BX ＝ 起始、CX ＝ 個數、ES:DX → 資料
			first, n := int(c.R[cpu.BX]), int(c.R[cpu.CX])
			for i := 0; i < n*3 && first*3+i < 768; i++ {
				d.M.DAC[first*3+i] = d.M.Read8(addr+uint32(i)) & 0x3F
			}
		case 0x15: // 讀單一 DAC：BX ＝ 索引 → DH/CH/CL
			i := int(c.R[cpu.BX]) & 0xFF
			c.R[cpu.DX] = c.R[cpu.DX]&0x00FF | uint16(d.M.DAC[i*3+0])<<8
			c.R[cpu.CX] = uint16(d.M.DAC[i*3+1])<<8 | uint16(d.M.DAC[i*3+2])
		case 0x17: // 讀一段 DAC：BX ＝ 起始、CX ＝ 個數 → ES:DX
			first, n := int(c.R[cpu.BX]), int(c.R[cpu.CX])
			for i := 0; i < n*3 && first*3+i < 768; i++ {
				d.M.Write8(addr+uint32(i), d.M.DAC[first*3+i])
			}
		default:
			d.note(0x10, 0x10, al(c))
		}

	case 0x02, 0x03, 0x05, 0x06, 0x09, 0x0A:
		// 設游標／取游標／設頁／捲動／寫字元：收下就好，
	default:
		d.note(0x10, fn, al(c))
	}
}

// int33 是滑鼠驅動（`docs/spec/004` §4，出處 `rich2/docs/re/182`）。
//
// **防拷畫面只吃滑鼠**：`rich2/docs/playtest/001` §3 記著鍵盤全都無效。
// 沒有這支的話遊戲偵測不到滑鼠，整個密碼畫面就沒有任何可用輸入——
// 而且不會有錯誤訊息，看起來就只是「卡住」。
// mouseXScale 是水平虛擬座標的倍率。
//
// int 33h 的虛擬螢幕**永遠是 640 格寬**，不管實際模式幾像素寬：
// 320 寬的模式（13h）回報值是像素的兩倍（所以 X 永遠是偶數），
// 640 寬的模式（12h）一比一。寫死 2 的話，640 寬的畫面上點右半邊會
// 回報成超出畫面的座標，遊戲**判定不在任何按鈕上而安靜地什麼都不做**。
func (d *DOS) mouseXScale() uint16 {
	// 呼叫端明講的倍率優先——XScale 存在的理由就是「我知道這一支
	// 用的是別的座標系」。0 才依模式自動決定。
	if d.Mouse.XScale != 0 {
		return d.Mouse.XScale
	}
	if w := d.M.PixelWidth(); w > 0 {
		return uint16(640 / w)
	}
	return 2
}

func (d *DOS) int33(c *cpu.CPU) {
	m := &d.Mouse
	xs := d.mouseXScale()
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
		m.Handler.Set, m.Handler.Mask = false, 0

	case 0x0001, 0x0002: // 顯示／隱藏游標
		// 遊戲**從來不叫 `AX=1`**（全檔 0 個呼叫端），畫面上那隻小手是
		// 它自己畫的，所以這裡不必真的畫游標（`rich2/docs/re/182` §4.1）。

	case 0x0003: // 取位置與鍵狀態
		// **記下每次輪詢回報出去的東西。** 這是分辨「輸入沒送到」與
		// 「送到了但答錯」的唯一辦法——兩者的畫面表現一模一樣。
		m.Polls = append(m.Polls, Poll{X: m.X, Y: m.Y, Buttons: m.Buttons,
			Step: d.M.Steps})
		c.R[cpu.BX] = m.Buttons
		c.R[cpu.CX] = m.X * xs
		c.R[cpu.DX] = m.Y

	case 0x0004: // 設位置
		x := c.R[cpu.CX]
		if xs > 0 {
			x /= xs
		}
		// **程式自己設的位置也要夾**，不然遊戲把游標設到範圍外之後
		// 我們與真機就分家了。不發事件——真機的 `AX=4` 不觸發回呼。
		m.X, m.Y = m.clamp(x, c.R[cpu.DX], xs)
		m.Sets = append(m.Sets, Poll{X: m.X, Y: m.Y, Step: d.M.Steps})

	case 0x0005, 0x0006: // 按下／放開的統計
		// **BX 進來是「問哪一個鍵」**（0 左／1 右／2 中），出去才是次數。
		btn := int(c.R[cpu.BX])
		if btn > 2 {
			btn = 2
		}
		m.PressQ[btn]++
		c.R[cpu.AX] = m.Buttons
		var at [2]uint16
		if fn == 0x0005 {
			c.R[cpu.BX] = m.Press[btn]
			at = m.PressAt[btn]
			m.Press[btn] = 0
		} else {
			c.R[cpu.BX] = m.Release[btn]
			at = m.ReleaseAt[btn]
			m.Release[btn] = 0
		}
		// 回的是**那一刻**的座標，不是現在的。
		c.R[cpu.CX] = at[0] * xs
		c.R[cpu.DX] = at[1]
		if c.R[cpu.BX] != 0 {
			m.PressReads = append(m.PressReads, PressRead{Fn: fn, Button: btn,
				Count: c.R[cpu.BX], X: at[0], Y: at[1], Step: d.M.Steps,
				CS: c.Seg[cpu.CS], IP: c.IP})
		}

	case 0x0007: // 設水平範圍（虛擬座標）
		m.MinX, m.MaxX = c.R[cpu.CX], c.R[cpu.DX]
	case 0x0008: // 設垂直範圍
		m.MinY, m.MaxY = c.R[cpu.CX], c.R[cpu.DX]

	case 0x000B: // 讀相對位移（mickey）：CX ＝ 水平、DX ＝ 垂直
		// **讀過歸零**——它的定義是「自從上次呼叫以來」。不歸零的話，
		// 用相對位移轉視角的程式會一直收到同一個位移，畫面自己轉不停。
		c.R[cpu.CX] = uint16(m.MickeyX)
		c.R[cpu.DX] = uint16(m.MickeyY)
		m.MickeyX, m.MickeyY = 0, 0

	case 0x000A: // 設文字游標形狀：收下（我們不畫游標）
	case 0x0009: // 設圖形游標形狀：收下

	case 0x0010: // 條件式隱藏游標：收下

	case 0x001A: // 設靈敏度：與 AX=0Fh 同一組，收下
	case 0x001B: // 取靈敏度
		c.R[cpu.BX], c.R[cpu.CX] = 8, 8 // 預設 8 mickey/8 pixel
		c.R[cpu.DX] = 16                // 倍速門檻

	case 0x0024: // 取驅動版本／型別
		// BH:BL ＝ 版本（8.03），CH ＝ 型別（4 ＝ PS/2），CL ＝ IRQ（0 ＝ PS/2）
		c.R[cpu.BX] = 0x0803
		c.R[cpu.CX] = 0x0400

	case 0x000C: // 設事件處理常式：ES:DX ＝ 常式、CX ＝ 事件遮罩
		// **這一支要真的呼叫**（`docs/spec/009`）。《臥龍傳》靠它維持
		// 畫面上那隻手：一次四千萬道指令的跑分裡 `AX=3` 只有 5 次，
		// 而 `AX=5` 有 240 萬次——游標幾乎全靠事件常式重畫。
		m.Handler.Seg, m.Handler.Off = c.Seg[cpu.ES], c.R[cpu.DX]
		m.Handler.Mask, m.Handler.Set = c.R[cpu.CX], true

	case 0x000F: // 設 mickey/pixel 比例：收下就好

	default:
		d.note(0x33, uint8(fn>>8), uint8(fn))
	}
}

// int16 是 BIOS 鍵盤，按鍵從 `Keys` 佇列來（`docs/spec/008`）。
//
// ⚠ **走不走這條要看是哪一支程式。** 編譯後的 MS BASIC（rich2）的 `INKEY$`
// 走 `int 21h AH=3Fh` 讀 handle 0，`int 16h` 全程只被叫 20–40 次
// （`rich2/docs/re/005`「輸入路徑」）；Turbo Pascal（《Pool of Radiance》）的
// `KeyPressed`／`ReadKey` **整條都走這裡**——量到 `AH=01` 被叫 166 萬次
// （`docs/spec/008` §2）。
//
// 兩個來源：`Keys` 是掃描碼與 ASCII 都指定好的字組佇列（方向鍵這類沒有
// ASCII 的鍵只能走它），`Stdin` 是與 `int 21h` 共用的位元組佇列，
// 讓同一份可重播輸入不必知道程式採哪一種介面。**先看 Keys**。
//
// 佇列空的時候回「沒有按鍵」，**不阻塞**：這一層沒有排程器，
// 呼叫端本來就會再問一次。
func (d *DOS) int16(c *cpu.CPU) {
	switch ah(c) {
	case 0x00, 0x10: // 讀按鍵（真 BIOS 是阻塞的）
		d.KeyPolls++
		// **BDA 的環形緩衝優先。** 那是 BIOS 真正的鍵盤佇列，
		// 程式也可能繞過 int 16h 直接讀它（見 machine.PushBIOSKey）；
		// 兩邊各留一份的話，同一個鍵會被讀兩次。
		if v, ok := d.M.PopKey(); ok {
			c.R[cpu.AX] = v
			d.noteKeyWord(c, "int16-AH00-bda", v)
			d.KeysConsumed++
			return
		}
		if len(d.Keys) > 0 {
			c.R[cpu.AX] = d.Keys[0]
			d.noteKeyWord(c, "int16-AH00-queue", d.Keys[0])
			d.Keys = d.Keys[1:]
			d.KeysConsumed++
			return
		}
		if len(d.Stdin) == 0 {
			c.R[cpu.AX] = 0
			return
		}
		c.R[cpu.AX] = keyWord(d.Stdin[0])
		d.noteKeyWord(c, "int16-AH00-stdin", keyWord(d.Stdin[0]))
		d.Stdin = d.Stdin[1:]
	case 0x01, 0x11: // 查有沒有按鍵：ZF=1 表示沒有，**不消耗佇列**
		d.KeyPolls++
		if v, ok := d.M.PeekKey(); ok {
			c.SetFlags(c.Flags &^ cpu.ZF)
			c.R[cpu.AX] = v
			return
		}
		if len(d.Keys) > 0 {
			c.SetFlags(c.Flags &^ cpu.ZF)
			c.R[cpu.AX] = d.Keys[0] // 查看不取走
			return
		}
		if len(d.Stdin) == 0 {
			c.SetFlags(c.Flags | cpu.ZF)
			return
		}
		c.SetFlags(c.Flags &^ cpu.ZF)
		c.R[cpu.AX] = keyWord(d.Stdin[0]) // 查看不取走
	case 0x05: // 把一個鍵塞進緩衝區：CX ＝ 掃描碼<<8 | ASCII
		// 巨集程式與自動輸入靠它把鍵餵給別人。**滿了要回 AL=1**，
		// 回 0 的話呼叫端以為塞進去了，而那個鍵消失得無聲無息。
		if d.M.PushBIOSKey(uint8(c.R[cpu.CX]>>8), uint8(c.R[cpu.CX])) {
			setAL(c, 0)
		} else {
			setAL(c, 1)
		}

	case 0x03: // 設 typematic 速率：收下（我們沒有重複輸入的模型）

	case 0x02, 0x12: // 取旗標狀態
		setAL(c, 0)
	case 0x13: // DOS/V 的鍵盤擴充狀態：收下，回「沒有特殊狀態」
		setAL(c, 0)
	default:
		d.note(0x16, ah(c), al(c))
	}
}

// keyWord 把一個 ASCII 位元組換成 BIOS 的 AX（AH ＝ 掃描碼、AL ＝ ASCII）。
//
// **掃描碼不能省**：只填 AL 的話，凡是用 AH 判鍵的程式都會收到 0，
// 而 0 在很多程式裡是「延伸鍵」的前綴——那會被讀成方向鍵。
// 表只收得下我們真的會送的鍵；沒收錄的回掃描碼 0，並在報告裡記一筆。
func keyWord(b uint8) uint16 {
	if sc, ok := scanCode[b]; ok {
		return uint16(sc)<<8 | uint16(b)
	}
	return uint16(b)
}

// ScanCode 回傳一個 ASCII 位元組的 set-1 掃描碼。
//
// 硬體鍵盤（IRQ1 ＋ 埠 0x60）送的是掃描碼不是 ASCII，所以餵鍵的一方
// 需要這張表。查不到時回 false——**不要回 0**，0 是合法的掃描碼位置，
// 送出去會被當成一個真的鍵。
func ScanCode(b uint8) (uint8, bool) {
	sc, ok := scanCode[b]
	return sc, ok
}

// scanCode 是 IBM PC 的 set-1 掃描碼（只列我們送得出去的鍵）。
var scanCode = map[uint8]uint8{
	0x1B: 0x01, // ESC
	'1':  0x02, '2': 0x03, '3': 0x04, '4': 0x05, '5': 0x06,
	'6': 0x07, '7': 0x08, '8': 0x09, '9': 0x0A, '0': 0x0B,
	'\r': 0x1C, '\n': 0x1C, ' ': 0x39,
	'q': 0x10, 'w': 0x11, 'e': 0x12, 'r': 0x13, 't': 0x14,
	'y': 0x15, 'u': 0x16, 'i': 0x17, 'o': 0x18, 'p': 0x19,
	'a': 0x1E, 's': 0x1F, 'd': 0x20, 'f': 0x21, 'g': 0x22,
	'h': 0x23, 'j': 0x24, 'k': 0x25, 'l': 0x26,
	'z': 0x2C, 'x': 0x2D, 'c': 0x2E, 'v': 0x2F, 'b': 0x30,
	'n': 0x31, 'm': 0x32,
	'Y': 0x15, 'N': 0x31,
}

// PushKey 把一個按鍵排進佇列。
//
// **先進 BDA 的環形緩衝**：很多程式不叫 `int 16h`，直接比對 `0040:001A`
// 與 `0040:001C` 判斷有沒有鍵（三國志的 MAIN.EXE 就是），只排進自己的
// 佇列的話它們一道都不會動，而且表面上完全正常。環滿了（15 個）才退回
// 自己的佇列，`int 16h` 會在環清空之後接著讀，順序不變。
func (d *DOS) PushKey(k Key) {
	if d.M.PushBIOSKey(k.Scan, k.ASCII) {
		return
	}
	d.Keys = append(d.Keys, k.Word())
}

// KeysPending 是兩條佇列加起來還沒被讀走的鍵數。
func (d *DOS) KeysPending() int { return d.M.BIOSKeyCount() + len(d.Keys) }

// PushKeyNamed 排一個有名字的鍵（`Return`、`Space`…）。名字不認得就回 false。
func (d *DOS) PushKeyNamed(name string) bool {
	k, ok := KeyNamed(name)
	if ok {
		d.PushKey(k)
	}
	return ok
}

// PushText 把一段可列印文字逐字排進佇列。遇到表外的字元停下並回 false，
// **不要跳過**——安靜地少送一個字會讓後面整串輸入錯位。
func (d *DOS) PushText(s string) bool {
	for _, r := range s {
		k, ok := KeyForRune(r)
		if !ok {
			return false
		}
		d.PushKey(k)
	}
	return true
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

// int1A 是 BIOS 的系統時鐘服務。
//
// **不實作會卡死**：程式用 `AH=00` 讀 tick 計數當延遲的依據，
// 讀到的值一直不動就永遠等下去（`SANGOKU`／`MAIN.EXE` 開場實測，
// 沒接之前一路輪詢 13,750 次還在原地）。
//
// tick 本身已經在 BDA 的 `0040:006C`（BIOS int 08h stub 推進，見 `machine.initVectors`），
// 這裡只是把它照 BIOS 的介面交出去。
func (d *DOS) int1A(c *cpu.CPU) {
	switch ah(c) {
	case 0x00: // 取 tick 計數 → CX:DX，AL ＝ 跨日旗標
		lo := d.M.Read16(0x0040*16 + 0x6C)
		hi := d.M.Read16(0x0040*16 + 0x6E)
		c.R[cpu.CX] = hi
		c.R[cpu.DX] = lo
		setAL(c, d.M.Read8(0x0040*16+0x70))
		d.M.Write8(0x0040*16+0x70, 0) // 讀過就清，與真機同
	case 0x01: // 設 tick 計數
		d.M.Write16(0x0040*16+0x6C, c.R[cpu.DX])
		d.M.Write16(0x0040*16+0x6E, c.R[cpu.CX])
	case 0x02, 0x04: // 讀 RTC 的時、分、秒／年、月、日
		// 沒有 RTC 就照「沒有時鐘」回：進位旗標立起來。
		// 回一組編出來的時間比較危險——程式可能拿它當亂數種子。
		setCarry(c)
	default:
		d.note(0x1A, ah(c), al(c))
		clearCarry(c)
	}
}

// noteKey 記一次按鍵被取走（`int 21h` 那幾條路，只有 ASCII）。
func (d *DOS) noteKey(via string, key uint8) {
	d.KeyReads = append(d.KeyReads, KeyRead{Step: d.M.Steps, Via: via, Key: key})
}

// noteKeyWord 記一次 `int 16h` 取走按鍵，連同字組與呼叫端（`docs/spec/185`）。
//
// 為什麼要連呼叫端：`KeysConsumed` 這個計數答不出「程式吃掉了但不理它」——
// 那與「鍵根本沒送到」在報表上長得一樣。追《Pool of Radiance》的方向鍵時，
// 三個鍵回溯到同一個位址、對回一顆 overlay 的一行，才看得出是哪一支在讀。
func (d *DOS) noteKeyWord(c *cpu.CPU, via string, word uint16) {
	ss, bp := c.Seg[cpu.SS], c.R[cpu.BP]
	outer := d.M.Read16(cpu.Addr(ss, bp))
	d.KeyReads = append(d.KeyReads, KeyRead{
		Step: d.M.Steps, Via: via, Key: uint8(word), Word: word,
		CS: c.Seg[cpu.CS], IP: c.IP,
		CallerIP:  d.M.Read16(cpu.Addr(ss, bp+2)),
		CallerCS:  d.M.Read16(cpu.Addr(ss, bp+4)),
		Caller2IP: d.M.Read16(cpu.Addr(ss, outer+2)),
		Caller2CS: d.M.Read16(cpu.Addr(ss, outer+4)),
	})
}

// PressButton／ReleaseButton 記一次按下／放開。
//
// **座標要當場記下來**：`AX=5`／`AX=6` 回的是按下那一刻的位置，
// 呼叫端讀到的時候游標可能已經移開了。
func (m *Mouse) PressButton(btn int) {
	if btn < 0 || btn > 2 {
		return
	}
	m.Buttons |= 1 << btn
	m.Press[btn]++
	m.PressAt[btn] = [2]uint16{m.X, m.Y}
}

func (m *Mouse) ReleaseButton(btn int) {
	if btn < 0 || btn > 2 {
		return
	}
	m.Buttons &^= 1 << btn
	m.Release[btn]++
	m.ReleaseAt[btn] = [2]uint16{m.X, m.Y}
}

// 事件遮罩的位元（MS Mouse `AX=000Ch`）。
const (
	EventMove          = 1 << 0
	EventLeftDown      = 1 << 1
	EventLeftUp        = 1 << 2
	EventRightDown     = 1 << 3
	EventRightUp       = 1 << 4
	eventButtonDownBit = 1 // 左鍵按下的位元序號；右鍵 +2、中鍵 +4
)

// clamp 把像素座標夾進遊戲設的範圍。範圍沒設過就不夾。
//
// 範圍是**虛擬座標**，我們的 X 是像素，所以水平要先除以倍率。
// 倍率由呼叫端算好傳進來：`Mouse.XScale` 的 0 是「依視訊模式自動決定」，
// 直接拿它來除會在沒人明講倍率時整個不夾——320 寬的畫面上游標跑到
// 639，而那看起來只是「游標飄出去了」。
func (m *Mouse) clamp(x, y, xs uint16) (uint16, uint16) {
	if m.MaxX > 0 {
		lo, hi := m.MinX, m.MaxX
		if xs > 1 {
			lo, hi = lo/xs, hi/xs
		}
		if x < lo {
			x = lo
		}
		if x > hi {
			x = hi
		}
	}
	if m.MaxY > 0 {
		if y < m.MinY {
			y = m.MinY
		}
		if y > m.MaxY {
			y = m.MaxY
		}
	}
	return x, y
}

// MoveMouse 移動游標：夾進範圍，位置真的變了才發事件。
//
// ⚠ **位置沒變就不要發。** 遊戲的事件常式自己也會比一次位置，
// 但多發的那些會把「這一輪有沒有動過」的判斷弄髒。
func (d *DOS) MoveMouse(x, y int) {
	m := &d.Mouse
	nx, ny := m.clamp(uint16(x), uint16(y), d.mouseXScale())
	if nx == m.X && ny == m.Y {
		return
	}
	dx, dy := int16(nx)-int16(m.X), int16(ny)-int16(m.Y)
	m.X, m.Y = nx, ny
	// 累加給 `AX=0Bh` 用。事件回呼那一份是「這一次事件的位移」，
	// 這一份是「自從程式上次問以來的總和」——兩個問題不一樣。
	m.MickeyX += dx
	m.MickeyY += dy
	d.fireMouseEventMickeys(EventMove, dx, dy)
}

// PressMouse／ReleaseMouse 按下／放開某個鍵（0 左／1 右／2 中）。
func (d *DOS) PressMouse(btn int) {
	d.Mouse.PressButton(btn)
	d.fireMouseEvent(uint16(1) << (eventButtonDownBit + 2*btn))
}

func (d *DOS) ReleaseMouse(btn int) {
	d.Mouse.ReleaseButton(btn)
	d.fireMouseEvent(uint16(1) << (eventButtonDownBit + 1 + 2*btn))
}

// fireMouseEvent 依遮罩把事件排進機器的回呼佇列。
//
// ⚠ **是排隊不是立刻跳。** 遠呼叫只能在指令邊界插，
// 而這一支是從外面（測試腳本、oracle）呼叫的，不在指令邊界上。
func (d *DOS) fireMouseEvent(mask uint16) { d.fireMouseEventMickeys(mask, 0, 0) }

// fireMouseEventMickeys 同 fireMouseEvent，但帶這一次位移的 mickey 數。
//
// 回呼入口的暫存器契約（Ralf Brown 的 int 33h AX=000Ch；強證據，沒有拿
// 原版驅動對拍過）：AX ＝ 事件遮罩、BX ＝ 按鍵狀態、CX/DX ＝ 游標座標、
// **SI ＝ 水平 mickey、DI ＝ 垂直 mickey**。SI/DI 反過來的話，只看單一
// 軸向的程式仍然會動，游標卻沿著另一條軸漂——那看起來像靈敏度沒調好。
//
// **一個邏輯 pixel 當一個 mickey** 是決定性近似：真驅動的比例由 AX=000Fh
// 設定，而我們是絕對定位，沒有可換算的來源。事件不帶位移（按鍵、外部
// 腳本直接發）時是 0，那與真驅動「這一次沒有移動」同義。
func (d *DOS) fireMouseEventMickeys(mask uint16, dx, dy int16) {
	m := &d.Mouse
	if !m.Handler.Set || m.Handler.Mask&mask == 0 {
		return
	}
	m.Events = append(m.Events, Poll{X: m.X, Y: m.Y, Buttons: mask, Step: d.M.Steps})
	d.M.QueueCallback(machine.QueuedCall{
		Seg: m.Handler.Seg, Off: m.Handler.Off,
		AX: mask, BX: m.Buttons,
		CX: m.X * d.mouseXScale(), DX: m.Y,
		SI: uint16(dx), DI: uint16(dy),
	})
}

// putPixel／getPixel 是 `int 10h AH=0Ch`／`0Dh`。
//
// 兩種畫面版面都要認：mode 13h 是「一個位元組一個像素」的線性緩衝區，
// 平面模式（0Dh/0Eh/10h/12h）是四個位元平面。**拿錯版面的話寫進去的
// 位元組會落在別的地方**，而畫面看起來只是「多了幾個雜點」。
func (d *DOS) putPixel(color uint8, x, y uint16) {
	w, h := d.M.VideoSize()
	if w == 0 || int(x) >= w || int(y) >= h {
		return
	}
	if pw, _ := d.M.PlanarSize(); pw != 0 {
		// 平面模式：一個位元組管八個像素，位元 7 是最左邊那個。
		// 走 VGA 的寫入路徑（Map Mask、位元遮罩、latch 都照規則走），
		// 這樣程式先設過的暫存器仍然有效。
		off := uint32(int(y)*(pw/8) + int(x)/8)
		mask := uint8(0x80 >> (x % 8))
		d.M.PlanarPutPixel(off, mask, color)
		return
	}
	d.M.Write8(uint32(machine.VideoSeg)*16+uint32(int(y)*w+int(x)), color)
}

func (d *DOS) getPixel(x, y uint16) uint8 {
	w, h := d.M.VideoSize()
	if w == 0 || int(x) >= w || int(y) >= h {
		return 0
	}
	if pw, ph := d.M.PlanarSize(); pw != 0 {
		px := d.M.PlanarPixels(pw, ph)
		if i := int(y)*pw + int(x); i < len(px) {
			return px[i]
		}
		return 0
	}
	return d.M.Read8(uint32(machine.VideoSeg)*16 + uint32(int(y)*w+int(x)))
}

// loadRGBrgbDAC 把 DAC 第 0–63 格設成 BIOS 在 mode 10h／12h 載入的 64 色表
// （`docs/spec/198-mode-set-default-dac`；DOSBox-X 的 text_palette）。
//
// 色號的 bit 0／1／2 讓 B／G／R 加 2Ah，bit 3／4／5 讓 b／g／r 加 15h。
// BGI 之類的程式只設屬性調色盤（色號 → DAC 格），顏色本身靠這張表；
// DAC 全 0 的話平面裡的色號全對、畫面卻整片黑。
func loadRGBrgbDAC(dac []uint8) {
	for i := 0; i < 64; i++ {
		bit := func(n int, v uint8) uint8 {
			if i&(1<<n) != 0 {
				return v
			}
			return 0
		}
		dac[i*3+0] = bit(2, 0x2A) + bit(5, 0x15)
		dac[i*3+1] = bit(1, 0x2A) + bit(4, 0x15)
		dac[i*3+2] = bit(0, 0x2A) + bit(3, 0x15)
	}
}
