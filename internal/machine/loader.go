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

// LoadOverlay 把 MZ 映像載到呼叫端指定的段位址，不改動行程狀態。
// DOS int 21h AX=4B03h 的重定位基準由另一個參數提供，可以與載入段不同。
func (m *Machine) LoadOverlay(data []byte, loadSeg, relocSeg uint16) error {
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
		return fmt.Errorf("machine: MZ 檔頭說覆疊映像是 %d..%d，但檔案只有 %d bytes",
			hdr, total, len(data))
	}
	image := append([]byte(nil), data[hdr:total]...)
	dst := uint32(loadSeg) * 16
	if uint64(dst)+uint64(len(image)) > uint64(len(m.Mem)) {
		return fmt.Errorf("machine: 覆疊目的區 %05X..%05X 超出記憶體",
			dst, uint64(dst)+uint64(len(image)))
	}
	for i := 0; i < int(h.Relocs); i++ {
		p := int(h.RelocOff) + i*4
		if p < 0 || p+4 > hdr || p+4 > len(data) {
			return fmt.Errorf("machine: 覆疊重定位表第 %d 筆超出 MZ header", i)
		}
		off := binary.LittleEndian.Uint16(data[p:])
		seg := binary.LittleEndian.Uint16(data[p+2:])
		idx := int(seg)*16 + int(off)
		if idx < 0 || idx+2 > len(image) {
			return fmt.Errorf("machine: 覆疊重定位第 %d 筆指到映像外 %04X:%04X", i, seg, off)
		}
		v := binary.LittleEndian.Uint16(image[idx:])
		binary.LittleEndian.PutUint16(image[idx:], v+relocSeg)
	}
	m.WriteBytes(dst, image)
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
	env := uint32(EnvSeg * 16)
	m.WriteBytes(env, append([]byte{0x00, 0x01, 0x00}, append(
		[]byte(`C:\RICH2\RUN.EXE`), 0x00)...))
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
