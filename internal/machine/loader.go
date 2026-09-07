package machine

import (
	"encoding/binary"
	"fmt"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// MZ／COM 載入器、overlay、PSP、環境區塊與 MCB 鏈（`docs/spec/003` §3）。
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

// mzBounds 回映像在檔案裡的 [起, 迄)。
func mzBounds(data []byte, h *mzHeader) (int, int, error) {
	hdr := int(h.HeaderPar) * 16
	total := (int(h.Pages)-1)*512 + int(h.LastPage)
	if h.LastPage == 0 {
		total = int(h.Pages) * 512
	}
	if hdr > len(data) || total > len(data) || total <= hdr {
		return 0, 0, fmt.Errorf("machine: MZ 檔頭說映像是 %d..%d，但檔案只有 %d bytes",
			hdr, total, len(data))
	}
	return hdr, total, nil
}

// mzImage 解出 MZ 映像並把重定位加上 relocFactor。
//
// ⚠ **relocFactor 不一定等於載入段。** 一般程式兩者相同，但 overlay
// （`AH=4Bh AL=03`）讓呼叫端分開指定，拿載入段當 relocFactor 用會在
// 兩者不同的程式上安靜地載入一份指向錯地方的映像。
func mzImage(data []byte, relocFactor uint16) ([]byte, *mzHeader, error) {
	h, err := parseMZ(data)
	if err != nil {
		return nil, nil, err
	}
	hdr, total, err := mzBounds(data, h)
	if err != nil {
		return nil, nil, err
	}
	image := append([]byte(nil), data[hdr:total]...)

	// **重定位一定要套。** 檔案裡的遠指標段值是「相對載入段」的，
	// 載入器要加上實際載入段；沒套的話第一個 far call 就飛到錯的地方。
	applied := 0
	for i := 0; i < int(h.Relocs); i++ {
		p := int(h.RelocOff) + i*4
		if p+4 > len(data) {
			return nil, nil, fmt.Errorf("machine: 重定位表第 %d 筆超出檔案", i)
		}
		off := binary.LittleEndian.Uint16(data[p:])
		seg := binary.LittleEndian.Uint16(data[p+2:])
		idx := int(seg)*16 + int(off)
		if idx+2 > len(image) {
			continue // 指到映像外，跳過；不是錯誤，舊 linker 會產生這種項
		}
		v := binary.LittleEndian.Uint16(image[idx:])
		binary.LittleEndian.PutUint16(image[idx:], v+relocFactor)
		applied++
	}
	if int(h.Relocs) > 0 && applied == 0 {
		return nil, nil, fmt.Errorf("machine: 有 %d 筆重定位卻一筆都沒套上——映像可能被截斷",
			h.Relocs)
	}
	return image, h, nil
}

// MZImageParags 回 MZ 映像（不含檔頭）佔幾個段。
//
// **在載入之前就要知道要多少空間**——EXEC 的「記憶體不足」要在動任何
// 狀態之前判定，載到一半才發現不夠會留下半套 PSP。
func MZImageParags(data []byte) (int, error) {
	h, err := parseMZ(data)
	if err != nil {
		return 0, err
	}
	hdr, total, err := mzBounds(data, h)
	if err != nil {
		return 0, err
	}
	return (total - hdr + 15) / 16, nil
}

// Program 是一支載好的程式（`docs/spec/008` §2）。
//
// 載入器只決定「放哪裡、從哪裡開始跑」；要不要切 CPU 過去是呼叫端的事
// （LoadEXE／LoadCOM 會切，EXEC 載入子行程時由服務層自己切）。
type Program struct {
	PSPSeg uint16 // PSP 段；映像在 PSPSeg+10h
	// Entry 是進入點。COM 是 PSPSeg:0100h，MZ 來自檔頭。
	CS, IP, SS, SP uint16
	// EndSeg 是映像之後第一個段（不含 PSP 前的 MCB 那一格）。
	EndSeg uint16

	imageLen int
}

// LoadProgramAt 把一支程式載到指定的 PSP 段（`docs/spec/009` §2.5）。
//
// **副檔名不是判準**：檔頭是 `MZ` 走 MZ 載入器（含重定位），
// 否則當 .COM。與 `cmd/probe` 的既有分派一致。
func (m *Machine) LoadProgramAt(pspSeg uint16, data []byte) (*Program, error) {
	if len(data) >= 2 && data[0] == 'M' && data[1] == 'Z' {
		return m.loadEXEAt(pspSeg, data)
	}
	return m.loadCOMAt(pspSeg, data)
}

// LoadEXE 把一個已經解包的 MZ 映像載進機器並把 CPU 設到進入點。
func (m *Machine) LoadEXE(data []byte) error {
	p, err := m.loadEXEAt(PSPSeg, data)
	if err != nil {
		return err
	}
	m.ImageBase, m.ImageLen = LoadSeg*16, p.imageLen
	// 映像之後才是可配置區。BASIC runtime 會先要一大塊當堆積，
	// 給不夠就報 Error 07（Out of memory）。
	m.FreeSeg = p.EndSeg

	m.initMCB()
	m.setEntry(p)
	return nil
}

// LoadCOM 載入 .COM 映像。
func (m *Machine) LoadCOM(data []byte) error {
	p, err := m.loadCOMAt(PSPSeg, data)
	if err != nil {
		return err
	}
	m.ImageBase, m.ImageLen = LoadSeg*16, p.imageLen
	// **頂層 `.COM` 名義上擁有整個段。** 它的堆疊在段頂（SP=FFFEh），
	// 可配置區只算到映像結尾的話，第一次 `AH=48h` 就把程式自己的堆疊
	// 配出去——緩衝區一被寫，返回位址就變成資料，然後 `ret` 跳到一個
	// 看起來很像程式碼的地方。
	m.FreeSeg = PSPSeg + 0x1000

	m.initMCB()
	m.setEntry(p)
	// COM 的 SP 指向段頂，堆疊上壓一個 0：`ret` 回 PSP:0 的 `int 20h`。
	m.Write16(cpu.Addr(p.PSPSeg, 0xFFFE), 0)
	return nil
}

// setEntry 把 CPU 設到程式的進入點。
func (m *Machine) setEntry(p *Program) {
	c := m.CPU
	c.Seg[cpu.CS], c.IP = p.CS, p.IP
	c.Seg[cpu.SS], c.R[cpu.SP] = p.SS, p.SP
	c.Seg[cpu.DS], c.Seg[cpu.ES] = p.PSPSeg, p.PSPSeg
	// **中斷要開著。** DOS 把控制權交給程式的時候 IF 是 1；CPU reset 之後
	// SetFlags(0) 是 0，載入器不補就沒有人會補。
	//
	// 症狀不指向這裡：任何「等 BIOS 時鐘跳動」的迴圈都變成死迴圈——
	// tick() 的 IRQ0 被 `!m.CPU.Flag(cpu.IF)` 擋掉，連 0040:006C 都不會動，
	// 看起來只是程式停在兩道指令之間。《Pool of Radiance》的 START.EXE
	// 開場就是這個形狀（`MOV AL,ES:[DI]` / `CMP AL,ES:[DI]` / `JZ −5`）。
	c.SetFlags(c.Flags | cpu.IF)
}

// loadEXEAt 是 MZ 載入的主體，PSP 段由呼叫端給（`docs/spec/009`）。
func (m *Machine) loadEXEAt(pspSeg uint16, data []byte) (*Program, error) {
	loadSeg := pspSeg + 0x10
	image, h, err := mzImage(data, loadSeg)
	if err != nil {
		return nil, err
	}
	m.WriteBytes(uint32(loadSeg)*16, image)
	m.initPSPAt(pspSeg)

	return &Program{
		PSPSeg: pspSeg,
		CS:     loadSeg + h.CS, IP: h.IP,
		SS: loadSeg + h.SS, SP: h.SP,
		EndSeg:   loadSeg + uint16((len(image)+15)/16) + 1,
		imageLen: len(image),
	}, nil
}

// loadCOMAt 是 .COM 載入的主體：無檔頭、無重定位，整份檔案放在 PSP+100h，
// 四個段暫存器都指向 PSP 段，IP ＝ 100h，SP 指向段頂。
//
// LoadSeg ＝ PSPSeg+10h，所以「PSP+100h」與 MZ 映像的位置是同一個位址，
// MCB 與 FreeSeg 的計算可以直接沿用。
func (m *Machine) loadCOMAt(pspSeg uint16, data []byte) (*Program, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("machine: COM 映像是空的")
	}
	// 映像從段內 0100h 開始，堆疊從 FFFEh 往下長，所以放得下的上限是
	// 0FF00h——剛好塞滿的話堆疊第一次 push 就寫進映像尾巴。
	if len(data) > 0xFEFE {
		return nil, fmt.Errorf("machine: COM 映像 %d bytes，塞不進一個段", len(data))
	}
	loadSeg := pspSeg + 0x10
	m.WriteBytes(uint32(loadSeg)*16, data)
	m.initPSPAt(pspSeg)

	return &Program{
		PSPSeg: pspSeg,
		CS:     pspSeg, IP: 0x100,
		SS: pspSeg, SP: 0xFFFE,
		// ⚠ EndSeg 只算到映像結尾，但 `.COM` 的堆疊在**段頂**（SP=FFFEh）。
		// 中間那段因此同時是「可配置」與「子程式的堆疊」——EXEC 進來的
		// `.COM` 子程式踩得到。頂層程式那條路由 LoadCOM 補成整段
		// （見它的註解）；子程式這條沒有證據說 DOS 怎麼分，先不猜。
		EndSeg:   loadSeg + uint16((len(data)+15)/16) + 1,
		imageLen: len(data),
	}, nil
}

// LoadOverlay 是 `int 21h AH=4Bh AL=03`（載入 overlay）的底層。
//
// 與 LoadProgramAt 的差別是**它什麼都不設**：不建 PSP、不動 CS:IP、
// 不碰 SS:SP。overlay 是被 far call 進去的程式碼，載入器只負責把映像
// 放到指定的段並把重定位套上 relocFactor。
//
// 智冠《三國演義》就是這樣把 DATA0.GRP 拉進來的——那也是為什麼那幾個
// `.GRP` 開頭是 `MZ`：它們是程式模組，不是資料容器。
func (m *Machine) LoadOverlay(data []byte, loadSeg, relocFactor uint16) error {
	image, _, err := mzImage(data, relocFactor)
	if err != nil {
		return err
	}
	if end := int(loadSeg)*16 + len(image); end > MemTop*16 {
		return fmt.Errorf("machine: overlay 載到 %04X:0 需要 %d bytes，超出傳統記憶體",
			loadSeg, len(image))
	}
	m.WriteBytes(uint32(loadSeg)*16, image)
	return nil
}

// initPSP 在 PSPSeg 建 PSP。
func (m *Machine) initPSP() { m.initPSPAt(PSPSeg) }

// initPSPAt 在指定段建一個夠用的 PSP。
//
// 「夠用」的定義是 Microsoft C runtime 啟動不炸——每一欄都有一個
// 具體的呼叫端，不是照手冊填滿。
func (m *Machine) initPSPAt(pspSeg uint16) {
	psp := uint32(pspSeg) * 16
	m.WriteBytes(psp, []byte{0xCD, 0x20}) // int 20h
	m.Write16(psp+2, MemTop)              // 記憶體上限

	// PSP+16h 是父行程的 PSP。第一支程式指向自己；EXEC 的子行程由
	// 服務層改成父行程的 PSP（`docs/spec/009` §2.4）。
	m.Write16(psp+0x16, pspSeg)

	// **PSP+2Ch 是環境區塊的段位址。** Microsoft C runtime 啟動時會去讀它
	// （`__setenvp`），指到 0 會讓後續的 heap 初始化判定失敗。
	// 版面是「環境字串（各自 ASCIZ，最後多一個 00）＋ word 計數 ＋ 程式全路徑」。
	//
	// ⚠ **程式路徑不能硬編。** 早期版本寫死 `C:\RICH2\RUN.EXE`——那是
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
	blk = append(blk, []byte(`COMSPEC=C:\COMMAND.COM`)...)
	blk = append(blk, 0x00)       // 這一條的結尾
	blk = append(blk, 0x00)       // 環境字串區的結尾（多一個 00）
	blk = append(blk, 0x01, 0x00) // 後面跟著幾個字串
	blk = append(blk, []byte(name)...)
	blk = append(blk, 0x00)
	m.WriteBytes(uint32(EnvSeg)*16, blk)
	m.Write16(psp+0x2C, EnvSeg)

	// PSP+32h／34h 是檔案表大小與位址。
	m.Write16(psp+0x32, 20)
	m.Write16(psp+0x34, 0x18)
	m.Write16(psp+0x36, pspSeg)
}

// WriteMCB 在 seg 這一段寫一個記憶體控制區塊。
//
// 佈局是 DOS 的：`+0` 簽章（`M` 中間、`Z` 鏈尾）、`+1` 擁有者 PSP
// （0 ＝ 自由）、`+3` 資料段數、`+8` 八個字元的程式名。
// 區塊的資料從 `seg+1` 開始，下一個 MCB 在 `seg+1+size`。
func (m *Machine) WriteMCB(seg uint16, last bool, owner, size uint16) {
	lin := uint32(seg) * 16
	sig := byte('M')
	if last {
		sig = 'Z'
	}
	m.Mem[lin] = sig
	m.Write16(lin+1, owner)
	m.Write16(lin+3, size)
	m.WriteBytes(lin+5, []byte{0, 0, 0})
	m.WriteBytes(lin+8, []byte("        "))
}

// initMCB 造一條合法的記憶體控制區塊鏈：程式自己的區塊 ＋ 後面全部自由。
//
// ⚠ **鏈要跟配置器同步，不能只在載入時建一次。** 舊版寫死兩格
// （`PSPSeg−1` 大小 0x2000、`PSPSeg+0x2000` 一格鏈尾），之後不論配置器
// 怎麼變都不更新。會走 MCB 鏈的程式因此看到一份與事實無關的地圖——
// 而且那格寫死的鏈尾落在 0x2100，對大一點的映像來說是**寫進程式自己的
// 映像裡**。智冠《三國演義》就是走這條鏈算「總共有多少段」再拿去載入
// 下一個模組的（`docs/spec/009` 附錄五）。同步由 `dos.syncMCB` 負責。
func (m *Machine) initMCB() {
	m.WriteMCB(PSPSeg-1, false, PSPSeg, m.FreeSeg-PSPSeg)
	m.WriteMCB(m.FreeSeg, true, 0, MemTop-m.FreeSeg-1)

	// DOS 的「list of lists」：`[BX-2]` 是第一個 MCB 的段位址。
	m.Write16(LOLSeg*16+0x0E, PSPSeg-1)
}

