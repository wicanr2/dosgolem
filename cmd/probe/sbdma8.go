package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// -sb-dma8、-sb-dma-log（docs/spec/444-legacy-sb16-dma8-irq7 §2.1、§2.8、§2.11）。

// sbDMA8Flags 註冊兩個旗標。拆成函式是為了讓測試能在獨立的 FlagSet 上驗。
func sbDMA8Flags(fs *flag.FlagSet) (*bool, *string) {
	on := fs.Bool("sb-dma8", false,
		"一般機器的 SB16 8 位元單次 DMA（通道 1）、ADPCM、靜音區塊與 IRQ7（`docs/spec/444-legacy-sb16-dma8-irq7`）。"+
			"需要同時給 -sound-blaster。預設關＝DMA 埠與 20h／A1h 維持舊行為、不送 IRQ7")
	log := fs.String("sb-dma-log", "",
		"把 -sb-dma8 的觀測紀錄（start、complete、cancel、irq7、defaulted、eoi）寫成 TSV，第一列為欄名"+
			"（`docs/spec/444-legacy-sb16-dma8-irq7` §2.8）")
	return on, log
}

// configureSBDMA8 依旗標開啟選項：-sb-dma8 沒有 -sound-blaster 時回錯誤（呼叫端 die）；-sb-dma-log 需要 -sb-dma8。
func configureSBDMA8(m *machine.Machine, soundBlaster, on bool, logPath string) error {
	if logPath != "" && !on {
		return errors.New("-sb-dma-log 需要 -sb-dma8")
	}
	if !on {
		return nil
	}
	if !soundBlaster {
		return errors.New("-sb-dma8 需要同時給 -sound-blaster（docs/spec/444 §2.1）")
	}
	if err := m.SetSBDMA8(true); err != nil {
		return fmt.Errorf("-sb-dma8：%w", err)
	}
	return nil
}

// openSBDMALog 建檔、寫欄名，並以 OnSBDMA 逐筆寫入（不受 SBDMALog 的 4096 筆上限影響）。
func openSBDMALog(m *machine.Machine, path string) (*os.File, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("-sb-dma-log：%w", err)
	}
	if err := writeSBDMAHeader(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("-sb-dma-log：%w", err)
	}
	m.OnSBDMA = func(e machine.SBDMAEvent) {
		if err := writeSBDMARow(f, e); err != nil {
			fmt.Fprintln(os.Stderr, "sb-dma-log:", err)
		}
	}
	return f, nil
}

func writeSBDMAHeader(w io.Writer) error {
	_, err := fmt.Fprintln(w, strings.Join(machine.SBDMAEventFields, "\t"))
	return err
}

func writeSBDMARow(w io.Writer, e machine.SBDMAEvent) error {
	_, err := fmt.Fprintln(w, strings.Join(e.Fields(), "\t"))
	return err
}

// reportSBDMA8 列出完成次數、IRQ7 遞送與預設處理次數、IRR7／ISR7 終值、20h／A0h 的讀取次數（§2.11）。
func reportSBDMA8(w io.Writer, m *machine.Machine) {
	st := m.SBDMA8Stats()
	fmt.Fprintf(w, "\nSB16 DMA8／IRQ7（docs/spec/444）：DMA 完成 %d 次、靜音完成 %d 次；IRQ7 遞送 %d 次、BIOS 預設處理 %d 次；"+
		"終值 IRR7=%t ISR7=%t 傳輸進行中=%t\n", st.DMA8Completions, st.SilenceCompletions, st.IRQ7Deliveries, st.IRQ7Defaulted,
		st.IRR7, st.ISR7, st.Active)
	fmt.Fprintf(w, "  埠讀取：20h ×%d、A0h ×%d；未寫過的 DMA 暫存器讀取 %d 次；觀測紀錄 %d 筆（丟棄 %d）\n",
		m.PortsIn[0x20], m.PortsIn[0xA0], m.SBDMAUnknownReads, len(m.SBDMALog), m.SBDMALogDropped)
	if err := m.SBDMA8Err(); err != nil {
		fmt.Fprintf(w, "  失敗即關閉：%v\n", err)
	}
}
