package machine

import "fmt"

// DOS 系統檔案表（SFT）鏈（`docs/spec/429-dos-sft-chain`）。
//
// **預設關閉**（`dosFiles == 0`）：不寫入任何位元組。既有收據記的是當時的記憶體
// 內容，預設開啟會讓整機雜湊全變，而畫面照常、沒有測試會報錯。

const (
	// SFTSeg 是第一張 SFT（4 筆項目）。與 MCBSeg 同值但不是 MCB 鏈的一部分。
	SFTSeg = 0x0060
	// SFT2Seg 是第二張 SFT，放 BIOS ROM 區（理由見規格 §2）。
	SFT2Seg = 0xF000

	sftHeaderSize = 6
	sftEntrySize  = 59
	sftFirstCount = 4 // 第一張表的項目數
	sftNameOff    = 0x20

	sftMinFiles = 6   // 下界沿用 DOSBox-X `dos_classes.cpp:136`
	sftMaxFiles = 255 // 上界：第二張表佔 F0000–F39DE，不碰字型與開機尾段
)

// SetDOSFiles 設定 SFT 的檔案總數。0 是預設（不寫入）；其餘值 6..255。
// 程式載入後（含 LoadState）一律回錯誤，不安靜失效。
func (m *Machine) SetDOSFiles(n int) error {
	if m.programLoaded {
		return fmt.Errorf("machine: 程式已載入，不能再設定 SFT 檔案數")
	}
	if n != 0 && (n < sftMinFiles || n > sftMaxFiles) {
		return fmt.Errorf("machine: SFT 檔案數 %d 超出範圍（0 或 %d..%d）", n, sftMinFiles, sftMaxFiles)
	}
	m.dosFiles = n
	return nil
}

// DOSFiles 回 SetDOSFiles 設定的值。
func (m *Machine) DOSFiles() int { return m.dosFiles }

// initSFT 在 initMCB 之後把 SFT 鏈寫進記憶體。
//
// 一律直接寫 m.Mem：第二張表在 ROM 區，Write8 會丟棄並計 ROMWrites。
// 整個表範圍顯式寫入（含 0），MemPoison 非零時表內未命名的位元組也是 0。
func (m *Machine) initSFT() {
	if m.dosFiles == 0 {
		return
	}
	put16 := func(a uint32, v uint16) { m.Mem[a], m.Mem[a+1] = byte(v), byte(v>>8) }

	// 第一張表
	t1 := uint32(SFTSeg) * 16
	n1 := uint32(sftHeaderSize + sftFirstCount*sftEntrySize)
	clear(m.Mem[t1 : t1+n1])
	put16(t1, 0) // 下一張：SFT2Seg:0000
	put16(t1+2, SFT2Seg)
	put16(t1+4, sftFirstCount)
	for i, name := range []string{"CON ", "CON ", "CON ", "AUX "} {
		copy(m.Mem[t1+sftHeaderSize+uint32(i)*sftEntrySize+sftNameOff:], name)
	}

	// 第二張表
	t2 := uint32(SFT2Seg) * 16
	rest := m.dosFiles - sftFirstCount
	clear(m.Mem[t2 : t2+uint32(sftHeaderSize+rest*sftEntrySize)])
	put16(t2, 0xFFFF) // 鏈尾
	put16(t2+2, 0xFFFF)
	put16(t2+4, uint16(rest))

	// list of lists +14h：第一張表的 far pointer（offset 在前）
	put16(LOLSeg*16+0x14, 0)
	put16(LOLSeg*16+0x16, SFTSeg)
}
