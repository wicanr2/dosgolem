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
	h, err := parseMZ(data)
	if err != nil {
		return err
	}
	p, err := m.loadEXEAt(PSPSeg, data)
	if err != nil {
		return err
	}
	m.ImageBase, m.ImageLen = LoadSeg*16, p.imageLen
	m.FreeSeg = p.EndSeg

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

// LoadCOM 載入 .COM 映像：無檔頭、無重定位，整份檔案放在 PSP+100h，
// 四個段暫存器都指向 PSP 段，IP ＝ 100h，SP 指向段頂並壓一個 0
// （RET 回 PSP:0 的 int 20h）。
//
// LoadSeg ＝ PSPSeg+10h，所以「PSP+100h」與 MZ 映像的位置是同一個位址，
// MCB 與 FreeSeg 的計算可以直接沿用。
func (m *Machine) LoadCOM(data []byte) error {
	p, err := m.loadCOMAt(PSPSeg, data)
	if err != nil {
		return err
	}
	m.ImageBase, m.ImageLen = LoadSeg*16, p.imageLen
	m.FreeSeg = p.EndSeg

	m.initMCB()

	c := m.CPU
	c.Seg[cpu.CS] = PSPSeg
	c.Seg[cpu.DS] = PSPSeg
	c.Seg[cpu.ES] = PSPSeg
	c.Seg[cpu.SS] = PSPSeg
	c.IP = 0x100
	c.R[cpu.SP] = 0xFFFE
	m.Write16(cpu.Addr(PSPSeg, 0xFFFE), 0)
	return nil
}

// loadEXEAt 是 LoadEXE 的主體，PSP 段由呼叫端給（`docs/spec/009`）。
func (m *Machine) loadEXEAt(pspSeg uint16, data []byte) (*Program, error) {
	h, err := parseMZ(data)
	if err != nil {
		return nil, err
	}
	hdr := int(h.HeaderPar) * 16
	total := (int(h.Pages)-1)*512 + int(h.LastPage)
	if h.LastPage == 0 {
		total = int(h.Pages) * 512
	}
	if hdr > len(data) || total > len(data) || total <= hdr {
		return nil, fmt.Errorf("machine: MZ 檔頭說映像是 %d..%d，但檔案只有 %d bytes",
			hdr, total, len(data))
	}
	image := append([]byte(nil), data[hdr:total]...)
	loadSeg := pspSeg + 0x10

	// **重定位一定要套。** 檔案裡的遠指標段值是「相對載入段」的，
	// 載入器要加上實際載入段；沒套的話第一個 far call 就飛到錯的地方。
	applied := 0
	for i := 0; i < int(h.Relocs); i++ {
		p := int(h.RelocOff) + i*4
		if p+4 > len(data) {
			return nil, fmt.Errorf("machine: 重定位表第 %d 筆超出檔案", i)
		}
		off := binary.LittleEndian.Uint16(data[p:])
		seg := binary.LittleEndian.Uint16(data[p+2:])
		idx := int(seg)*16 + int(off)
		if idx+2 > len(image) {
			continue // 指到映像外，跳過；不是錯誤，舊 linker 會產生這種項
		}
		v := binary.LittleEndian.Uint16(image[idx:])
		binary.LittleEndian.PutUint16(image[idx:], v+loadSeg)
		applied++
	}
	if int(h.Relocs) > 0 && applied == 0 {
		return nil, fmt.Errorf("machine: 有 %d 筆重定位卻一筆都沒套上——映像可能被截斷",
			h.Relocs)
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

// loadCOMAt 是 LoadCOM 的主體，PSP 段由呼叫端給。
func (m *Machine) loadCOMAt(pspSeg uint16, data []byte) (*Program, error) {
	if len(data) == 0 || len(data) > 0xFF00 {
		return nil, fmt.Errorf("machine: COM 映像大小 %d 不合法（1..65280）", len(data))
	}
	loadSeg := pspSeg + 0x10
	m.WriteBytes(uint32(loadSeg)*16, data)
	m.initPSPAt(pspSeg)

	return &Program{
		PSPSeg: pspSeg,
		CS:     pspSeg, IP: 0x100,
		SS: pspSeg, SP: 0xFFFE,
		EndSeg:   loadSeg + uint16((len(data)+15)/16) + 1,
		imageLen: len(data),
	}, nil
}

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
	// 內容是「空環境 ＋ 計數 1 ＋ 程式路徑」。整台機器共用一塊；
	// 子行程要不同的環境段由服務層自己改這一格。
	env := uint32(EnvSeg * 16)
	m.WriteBytes(env, append([]byte{0x00, 0x01, 0x00}, append(
		[]byte(`C:\RICH2\RUN.EXE`), 0x00)...))
	m.Write16(psp+0x2C, EnvSeg)

	// PSP+32h／34h 是檔案表大小與位址。
	m.Write16(psp+0x32, 20)
	m.Write16(psp+0x34, 0x18)
	m.Write16(psp+0x36, pspSeg)
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
//
// EXEC 載入的子行程不在這條鏈上——它們的假 MCB 由服務層在 `freeSeg`
// 那格寫（`docs/spec/009` §2.3），與 `AH=48h` 的慣例一致。
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

// WriteMCB 在 seg 那格寫一個假 MCB（擁有者 owner、大小 paras）。
//
// EXEC 與監督佇列載入程式時用（`docs/spec/008` §5／`009` §2.3）。
// 它不進 `initMCB` 建的那條鏈——**這台機器不做真的 MCB 走查**，
// 這一格存在的理由是程式可能讀自己的 PSP−1。
func (m *Machine) WriteMCB(seg, owner, paras uint16) {
	at := uint32(seg) * 16
	m.Mem[at] = 'M'
	m.Write16(at+1, owner)
	m.Write16(at+3, paras)
	m.WriteBytes(at+8, []byte("        "))
}
