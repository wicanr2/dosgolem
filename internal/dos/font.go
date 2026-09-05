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
const (
	intFontFull = 0xF1
	intFontHalf = 0xF2
)

// installFont 把兩段 stub 種進 StubSeg。
func (d *DOS) installFont() {
	base := uint32(machine.StubSeg) * 16
	d.M.WriteBytes(base+fontFullOff, []byte{0xCD, intFontFull, 0xCB}) // int F1h; retf
	d.M.WriteBytes(base+fontHalfOff, []byte{0xCD, intFontHalf, 0xCB}) // int F2h; retf
}

// int15 是 BIOS 系統服務。目前只認得 DOS/V 的字型服務。
func (d *DOS) int15(c *cpu.CPU) {
	if ah(c) != 0x50 || al(c) != 0x00 {
		d.note(0x15, ah(c), al(c))
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
