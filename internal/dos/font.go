package dos

import (
	"os"

	"github.com/wicanr2/dosgolem/internal/cpu"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// DOS/V 字型服務（`INT 15h AH=50h`，`docs/spec/008` §3）。
//
// 呼叫端要的不是字模，是**一個可以 `call far` 的位址**。所以這裡在
// `StubSeg` 種兩段三個 byte 的 real mode stub（`int F1h/F2h` ＋ `retf`），
// 把位址交出去，真正的工作在 Go 這邊做。
//
// ⚠ **沒有這一支的話遊戲不是「不顯示文字」，是當掉。**
// `KI.EXE` 拿不到向量時把 `0000:0000` 留在原地，第一次畫字就是一次
// 遠呼叫到零位址；落點恰好是開機配置記憶體的那一段，於是它重跑一次配置、
// 失敗、報「記憶體不足」離開。**錯誤訊息指向的地方與根因差了 200 萬道指令。**

// Font 是字型服務的設定。
type Font struct {
	// Full／Half 是全形與半形字型檔名（相對於 Root）。
	//
	// **預設 `END_S13.DAT`／`END_S14.DAT`**，不是 `STR.EXE` 寫死的
	// `END_S10/S11`——後者在手上這份封裝裡是結局過場圖
	// （臥龍傳專案 `docs/re/29` §6）。做成參數是為了讓「照字面跑一次」
	// 仍然是一個可執行的實驗。
	Full, Half string

	// FullBytes／HalfBytes 是每個字的位元組數：全形 30（16×15）、半形 15（8×15）。
	// 宣告 16 列、實存 15 列，第 16 列由常式自己補 0。
	FullBytes, HalfBytes int

	// Calls 是兩支常式各被叫了幾次。**這是分辨「沒畫字」與「畫了但看不見」
	// 的第一個問題**——兩者的畫面一樣空。
	Calls [2]int

	// Missing 記下讀不到字模的次數（檔案缺、位移超過檔尾）。
	Missing int

	// cache 是字型檔的內容。原版每畫一個字就 open/lseek/read/close 一次
	// （`docs/re/29` §4.3），我們不必照抄那個成本——**畫出來的字模一樣**，
	// 差別只在 I/O 次數，而 I/O 次數不是對拍的比較對象。
	cache map[string][]byte
}

// DefaultFont 是松崗 DOS/V 版的設定。
func DefaultFont() Font {
	return Font{Full: "END_S13.DAT", Half: "END_S14.DAT", FullBytes: 30, HalfBytes: 15}
}

// 字型 stub 在 StubSeg 裡的位移。**不能與 mouseStubOff 撞**。
const (
	fontFullOff = 0x20
	fontHalfOff = 0x24
)

// 字型 stub 用的中斷號。真機沒有這兩支，所以不會與遊戲搶。
// intFontFull／intFontHalf 是字型 stub 用的中斷號。
//
// ⚠ **不能用 F2–F6**：那五個是服務型中斷的 trampoline
// （`machine.initVectors`）。撞號的話取字型會變成一次 int 21h，
// 而字模緩衝區裡留著垃圾——畫面上只是「某些字畫不出來」。
const (
	intFontFull = 0xF7
	intFontHalf = 0xF8
)

// installFont 把兩段 stub 種進 StubSeg。
func (d *DOS) installFont() {
	base := uint32(machine.StubSeg) * 16
	d.M.WriteBytes(base+fontFullOff, []byte{0xCD, intFontFull, 0xCB}) // int F7h; retf
	d.M.WriteBytes(base+fontHalfOff, []byte{0xCD, intFontHalf, 0xCB}) // int F8h; retf
}

// int15 是 AT 系統服務。目前只認得 DOS/V 的字型服務（`AX=5000h`）。
//
// 其餘一律只記一筆。它的另一個存在理由是 trampoline
// （`docs/spec/004` §2.1）：DOSJP 掛走 int 15h 之後 chain 回舊向量
// 要到得了這裡。
func (d *DOS) int15(c *cpu.CPU) {
	switch ah(c) {
	case 0x88: // 取延伸記憶體大小（KB，1 MB 之上）
		// XMS 之前的程式問這個。**回 0 會被讀成「沒有延伸記憶體」**，
		// 於是它連 XMS 都不問，直接把資料塞進傳統記憶體。
		c.R[cpu.AX] = xmsTotalKB
		clearCarry(c)
		return

	case 0x87: // 延伸記憶體區塊搬移：ES:SI ＝ GDT、CX ＝ 要搬幾個 word
		d.int15Move(c)
		return

	case 0x86: // 等待 CX:DX 微秒
		// 我們是指令數模型，沒有牆上的時鐘。**收下並回成功**：
		// 回失敗的話呼叫端會改用忙等迴圈，那更慢而且一樣不準。
		// 等待長度記進 Unimplemented 以外的地方沒有意義，所以只記一筆。
		d.note(0x15, 0x86, 0)
		clearCarry(c)
		return

	case 0xC0: // 取系統設定表 → ES:BX
		// 程式拿它判機型與有沒有第二個 PIC。**指到合法位址就好**——
		// 回 CF 的話有些程式會以為自己在 PC/XT 上而關掉 286 以上的路徑。
		d.M.WriteBytes(uint32(machine.StubSeg)*16+sysConfigOff, []byte{
			0x08, 0x00, // 表長度 8
			0xFC,       // 機型：AT
			0x01,       // 次機型
			0x00,       // BIOS 版本
			0x74,       // 功能：有第二個 PIC、有 RTC
			0x00, 0x00, // 保留
		})
		c.Seg[cpu.ES] = machine.StubSeg
		c.R[cpu.BX] = sysConfigOff
		setAH(c, 0)
		clearCarry(c)
		return
	}
	if ah(c) != 0x50 || al(c) != 0x00 {
		d.note(0x15, ah(c), al(c))
		clearCarry(c)
		return
	}
	// BH：0 ＝ 半形、1 ＝ 全形（`docs/re/29` §2）。
	c.Seg[cpu.ES] = machine.StubSeg
	if c.R[cpu.BX]>>8 != 0 {
		c.R[cpu.BX] = fontFullOff
	} else {
		c.R[cpu.BX] = fontHalfOff
	}
	setAH(c, 0)
	clearCarry(c)
}

// fontGlyph 是兩支 stub 的實作：CX ＝ 字碼，ES:SI ＝ 呼叫端的 32 byte 緩衝。
func (d *DOS) fontGlyph(c *cpu.CPU, full bool) {
	ch, cl := uint8(c.R[cpu.CX]>>8), uint8(c.R[cpu.CX])
	dst := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.SI])

	var name string
	var n, size int
	if full {
		d.Font.Calls[0]++
		name, size = d.Font.Full, d.Font.FullBytes
		n = fullIndex(ch, cl)
	} else {
		d.Font.Calls[1]++
		name, size = d.Font.Half, d.Font.HalfBytes
		n = int(cl)
	}

	buf := d.fontBytes(name, n*size, size)
	for i := 0; i < size; i++ {
		var v uint8
		if buf != nil {
			v = buf[i]
		}
		d.M.Write8(dst+uint32(i), v)
	}
	// 第 16 列固定補 0：全形一個 word、半形一個 byte。
	d.M.Write8(dst+uint32(size), 0)
	if full {
		d.M.Write8(dst+uint32(size)+1, 0)
	}
	c.R[cpu.AX] = 0
	clearCarry(c)
}

// fullIndex 把 Big5 字碼換成字模格號（`docs/spec/008` §3.2，
// 逐條對照 `STR.EXE` 常駐段 `+5D` 的 bytes）。
//
// ⚠ **越界那一支回 0x56 之後就結束，不加低位元組。**
// 照文字敘述往下讀會得到「0x56 ＋ 低位元組」——那也是一個合法格號，
// 畫出來仍然像個字，所以錯了不會有人發現。
func fullIndex(ch, cl uint8) int {
	var n int
	switch {
	case ch < 0xA4:
		n = int(ch-0xA1) * 0x9D
	case ch < 0xC9:
		n = int(ch-0xA4)*0x9D + 0x198
	case ch < 0xFA:
		n = int(ch-0xC9)*0x9D + 0x16B1
	default:
		return 0x56
	}
	if cl > 0x7E {
		cl -= 0x22
	}
	return n + int(cl) - 0x40
}

// fontBytes 從字型檔取一格。讀不到就回 nil（呼叫端填 0）。
func (d *DOS) fontBytes(name string, off, size int) []byte {
	data, ok := d.Font.cache[name]
	if !ok {
		if path := d.resolve(name); path != "" {
			data, _ = os.ReadFile(path)
		}
		if d.Font.cache == nil {
			d.Font.cache = map[string][]byte{}
		}
		d.Font.cache[name] = data
	}
	if off < 0 || off+size > len(data) {
		d.Font.Missing++
		return nil
	}
	return data[off : off+size]
}


// sysConfigOff 是 `int 15h AH=C0h` 的系統設定表在 StubSeg 裡的位移。
//
// ⚠ **要避開所有 stub**：每個向量的 stub 佔 0x000–0x3FF，特殊 stub 在
// 0x400 起（見 `machine.initVectors`）。寫進那些位址等於把中斷處理常式
// 改掉，而症狀出現在下一次那個中斷被叫的時候。
const sysConfigOff = 0x500

// int15Move 是 `AH=87h`：用 GDT 描述子搬移延伸記憶體。
//
// GDT 在 ES:SI，六個描述子各 8 bytes；第 2 個（位移 10h）是來源、
// 第 3 個（位移 18h）是目的。每個描述子：+0 界限、+2..+4 24 位元基底。
// CX 是**要搬幾個 word**，不是 byte——看成 byte 的話只搬一半，
// 而後半段留著上一次的內容，看起來像資料檔只壞了後面。
func (d *DOS) int15Move(c *cpu.CPU) {
	gdt := cpu.Addr(c.Seg[cpu.ES], c.R[cpu.SI])
	base := func(off uint32) uint32 {
		lo := uint32(d.M.Read16(gdt + off + 2))
		hi := uint32(d.M.Read8(gdt + off + 4))
		return lo | hi<<16
	}
	src, dst := base(0x10), base(0x18)
	n := uint32(c.R[cpu.CX]) * 2
	for i := uint32(0); i < n; i++ {
		d.M.Write8(dst+i, d.M.Read8(src+i))
	}
	setAH(c, 0) // 搬移成功
	clearCarry(c)
	d.XMSMoves = append(d.XMSMoves, XMSMove{
		Step: d.M.Steps, Len: n, SrcOff: src, DstOff: dst,
	})
}
