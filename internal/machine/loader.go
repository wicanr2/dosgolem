package machine

import (
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// MZ 載入器、PSP、環境區塊與 MCB 鏈（`docs/spec/003` §3）。
//
// **輸入是已經解包的映像。** `RUN.EXE` 是雙層打包
// `LZEXE93(EXEPACK(本體))`，兩層都要先解（`rich2/CLAUDE.md` §4.1）；
// 解包工具在 `rich2/tools/`，產物由呼叫端給進來。做在這裡等於每次跑都
// 重做一遍，而且會讓「載入器對不對」與「解包對不對」兩個問題混在一起。

// mzHeader 是 MZ 檔頭裡我們用得到的欄位。
type mzHeader struct {
	LastPage  uint16 // 最後一頁用了幾個 byte（0 表示整頁）
	Pages     uint16 // 512 byte 的頁數
	Relocs    uint16 // 重定位項數
	HeaderPar uint16 // 檔頭佔幾個段
	SS, SP    uint16
	IP, CS    uint16
	RelocOff  uint16
}

func parseMZ(data []byte) (*mzHeader, error) {
	if len(data) < 28 {
		return nil, fmt.Errorf("machine: 檔案只有 %d bytes，放不下 MZ 檔頭", len(data))
	}
	if data[0] != 'M' || data[1] != 'Z' {
		return nil, fmt.Errorf("machine: 不是 MZ 執行檔（開頭是 %02X %02X）", data[0], data[1])
	}
	u := func(off int) uint16 { return binary.LittleEndian.Uint16(data[off:]) }
	return &mzHeader{
		LastPage: u(2), Pages: u(4), Relocs: u(6), HeaderPar: u(8),
		SS: u(14), SP: u(16), IP: u(20), CS: u(22), RelocOff: u(24),
	}, nil
}

// LoadEXE 把一個已經解包的 MZ 映像載進機器並把 CPU 設到進入點。
func (m *Machine) LoadEXE(data []byte) error {
	h, err := parseMZ(data)
	if err != nil {
		return err
	}
	hdr := int(h.HeaderPar) * 16
	total := (int(h.Pages)-1)*512 + int(h.LastPage)
	if h.LastPage == 0 {
		total = int(h.Pages) * 512
	}
	if hdr > len(data) || total > len(data) || total <= hdr {
		return fmt.Errorf("machine: MZ 檔頭說映像是 %d..%d，但檔案只有 %d bytes",
			hdr, total, len(data))
	}
	image := append([]byte(nil), data[hdr:total]...)

	// **重定位一定要套。** 檔案裡的遠指標段值是「相對載入段」的，
	// 載入器要加上實際載入段；沒套的話第一個 far call 就飛到錯的地方。
	applied := 0
	for i := 0; i < int(h.Relocs); i++ {
		p := int(h.RelocOff) + i*4
		if p+4 > len(data) {
			return fmt.Errorf("machine: 重定位表第 %d 筆超出檔案", i)
		}
		off := binary.LittleEndian.Uint16(data[p:])
		seg := binary.LittleEndian.Uint16(data[p+2:])
		idx := int(seg)*16 + int(off)
		if idx+2 > len(image) {
			continue // 指到映像外，跳過；不是錯誤，舊 linker 會產生這種項
		}
		v := binary.LittleEndian.Uint16(image[idx:])
		binary.LittleEndian.PutUint16(image[idx:], v+LoadSeg)
		applied++
	}
	if int(h.Relocs) > 0 && applied == 0 {
		return fmt.Errorf("machine: 有 %d 筆重定位卻一筆都沒套上——映像可能被截斷",
			h.Relocs)
	}

	m.WriteBytes(LoadSeg*16, image)
	m.ImageBase, m.ImageLen = LoadSeg*16, len(image)
	// 映像之後才是可配置區。BASIC runtime 會先要一大塊當堆積，
	// 給不夠就報 Error 07（Out of memory）。
	m.FreeSeg = LoadSeg + uint16((len(image)+15)/16) + 1

	m.initPSP()
	m.initMCB()

	c := m.CPU
	c.Seg[cpu.CS] = LoadSeg + h.CS
	c.IP = h.IP
	c.Seg[cpu.SS] = LoadSeg + h.SS
	c.R[cpu.SP] = h.SP
	c.Seg[cpu.DS] = PSPSeg
	c.Seg[cpu.ES] = PSPSeg
	return nil
}

// initPSP 建一個夠用的 PSP。
//
// 「夠用」的定義是 Microsoft C runtime 啟動不炸——每一欄都有一個
// 具體的呼叫端，不是照手冊填滿。
func (m *Machine) initPSP() {
	psp := uint32(PSPSeg * 16)
	m.WriteBytes(psp, []byte{0xCD, 0x20}) // int 20h
	m.Write16(psp+2, MemTop)              // 記憶體上限

	// PSP+16h 是父行程的 PSP。
	m.Write16(psp+0x16, PSPSeg)

	// **PSP+2Ch 是環境區塊的段位址。** Microsoft C runtime 啟動時會去讀它
	// （`__setenvp`），指到 0 會讓後續的 heap 初始化判定失敗。
	// 內容是「空環境 ＋ 計數 1 ＋ 程式路徑」。
	// 版面是「環境字串（各自 ASCIZ，最後多一個 00）＋ word 計數 ＋ 程式全路徑」。
	// 這裡給空環境。
	//
	// ⚠ **程式路徑不能硬編。** 第一版寫死 `C:\RICH2\RUN.EXE`——那是
	// 某一支程式的值放在通用層，違反 `docs/spec/006` 的分層判準
	// （「換一支 binary 之後這段還成立嗎？」）。MSC 的啟動碼會讀這個路徑，
	// 拿別支程式的路徑不會報錯，只會讓 argv[0] 是錯的。
	name := m.ProgramPath
	if name == "" {
		name = `C:\PROG.EXE`
	}
	// 給一組最小但合法的環境字串，形狀與真 DOS 一致——真 DOS 底下
	// 環境幾乎不可能是空的（至少有 COMSPEC）。
	//
	// ⚠ **這不是某個問題的修法。** 曾經拿它試智冠《三國演義》的
	// `R6009 - not enough space for environment`，**症狀完全沒變**
	// （指令數只因為多走訪幾個 byte 而差 244 道）。留著純粹是因為
	// 它比空環境更接近真 DOS，不要以為它修好了什麼。
	var blk []byte
	blk = append(blk, []byte("COMSPEC=C:\\COMMAND.COM")...)
	blk = append(blk, 0x00)       // 這一條的結尾
	blk = append(blk, 0x00)       // 環境字串區的結尾（多一個 00）
	blk = append(blk, 0x01, 0x00) // 後面跟著幾個字串
	blk = append(blk, []byte(name)...)
	blk = append(blk, 0x00)
	env := uint32(EnvSeg * 16)
	m.WriteBytes(env, blk)
	m.Write16(psp+0x2C, EnvSeg)

	// PSP+32h／34h 是檔案表大小與位址。
	m.Write16(psp+0x32, 20)
	m.Write16(psp+0x34, 0x18)
	m.Write16(psp+0x36, PSPSeg)
}

// initMCB 造一條最小的合法記憶體控制區塊鏈。
//
// ⚠ **這一段的必要性沒有獨立證明。** `rich2/docs/re/005` §4 的
// `DOS memory-arena error` 看起來像 MCB 鏈驗證失敗，但 §10 找到的根因是
// `int 21h AH=4Ah` 的探測語意（`docs/spec/004` §1.2）——連續三輪調 MCB
// 佈局都無效。
//
// 這裡照樣建，理由只有一個：**它是那份實際跑通的實作留下來的**，
// 而現在還沒有「拿掉也照跑」的收據。等 MVP-B 過了再回來拿掉試一次，
// 過得了就刪掉這一段。
//
// 關鍵是鏈上要有一個「擁有者是本程式 PSP」的區塊，而且要涵蓋程式本身，
// 所以第一個 MCB 放在 `PSPSeg − 1`，緊接著就是 PSP。
func (m *Machine) initMCB() {
	const progSize = 0x2000 // 段數，足以蓋住整個程式

	first := uint32((PSPSeg - 1) * 16)
	m.Mem[first] = 'M'
	m.Write16(first+1, PSPSeg)
	m.Write16(first+3, progSize)
	m.WriteBytes(first+8, []byte("        "))

	last := uint32((PSPSeg + progSize) * 16)
	m.Mem[last] = 'Z'
	m.Write16(last+1, 0)
	m.Write16(last+3, 0x0100)
	m.WriteBytes(last+8, []byte("        "))

	// DOS 的「list of lists」：`[BX-2]` 是第一個 MCB 的段位址。
	m.Write16(LOLSeg*16+0x0E, PSPSeg-1)
}

// LoadOverlay 是 `int 21h AH=4Bh AL=03`（載入 overlay）的底層。
//
// 與 LoadEXE 的差別是**它什麼都不設**：不建 PSP、不動 CS:IP、不碰 SS:SP。
// overlay 是被 far call 進去的程式碼，載入器只負責把映像放到指定的段
// 並把重定位套上 relocFactor。
//
// 智冠《三國演義》就是這樣把 DATA0.GRP 拉進來的——那也是為什麼那幾個
// `.GRP` 開頭是 `MZ`：它們是程式模組，不是資料容器。
//
// ⚠ **relocFactor 與 loadSeg 是兩個獨立的參數。** DOS 讓呼叫端分別指定，
// 多數程式給相同的值，但不保證。拿 loadSeg 當 relocFactor 用會在兩者
// 不同的程式上安靜地載入一份指向錯地方的映像。
func (m *Machine) LoadOverlay(data []byte, loadSeg, relocFactor uint16) error {
	h, err := parseMZ(data)
	if err != nil {
		return err
	}
	hdr := int(h.HeaderPar) * 16
	total := (int(h.Pages)-1)*512 + int(h.LastPage)
	if h.LastPage == 0 {
		total = int(h.Pages) * 512
	}
	if hdr > len(data) || total > len(data) || total <= hdr {
		return fmt.Errorf("machine: overlay 檔頭說映像是 %d..%d，但檔案只有 %d bytes",
			hdr, total, len(data))
	}
	image := append([]byte(nil), data[hdr:total]...)

	applied := 0
	for i := 0; i < int(h.Relocs); i++ {
		p := int(h.RelocOff) + i*4
		if p+4 > len(data) {
			return fmt.Errorf("machine: overlay 重定位表第 %d 筆超出檔案", i)
		}
		off := binary.LittleEndian.Uint16(data[p:])
		seg := binary.LittleEndian.Uint16(data[p+2:])
		idx := int(seg)*16 + int(off)
		if idx+2 > len(image) {
			continue
		}
		v := binary.LittleEndian.Uint16(image[idx:])
		binary.LittleEndian.PutUint16(image[idx:], v+relocFactor)
		applied++
	}
	if int(h.Relocs) > 0 && applied == 0 {
		return fmt.Errorf("machine: overlay 有 %d 筆重定位卻一筆都沒套上——映像可能被截斷",
			h.Relocs)
	}

	end := int(loadSeg)*16 + len(image)
	if end > MemTop*16 {
		return fmt.Errorf("machine: overlay 載到 %04X:0 需要 %d bytes，超出傳統記憶體",
			loadSeg, len(image))
	}
	m.WriteBytes(uint32(loadSeg)*16, image)
	return nil
}
