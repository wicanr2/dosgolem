package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/wicanr2/dosgolem/internal/machine"
)

// docs/spec/459-mouse-irq12-v86-delivery §3.8：保護模式剖面的 IRQ12 診斷開關與報告欄位。

// mouseIRQ12DirectName 是 §3.8 的隱藏診斷旗標：保護模式剖面下讓滑鼠事件照實模式剖面直接遠呼叫（§6.4 N1 的負對照）。
// 不出現在 -h（hideFlags），開啟時報告標記「負對照，非正式結果」。
const mouseIRQ12DirectName = "diag-mouse-irq12-direct"

// mouseIRQ12DirectFlag 註冊隱藏旗標。拆成函式是為了讓測試能在獨立的 FlagSet 上驗預設值與開啟後的值。
func mouseIRQ12DirectFlag(fs *flag.FlagSet) *bool {
	return fs.Bool(mouseIRQ12DirectName, false,
		"（診斷，docs/spec/459 §3.8）保護模式剖面下滑鼠事件改回直接遠呼叫；只供負對照，結果不是正式收據")
}

// hideFlags 讓 fs.Usage 不列出 names（其餘旗標照 PrintDefaults 的格式）。旗標本身照常可解析。
func hideFlags(fs *flag.FlagSet, names ...string) {
	hidden := map[string]bool{}
	for _, n := range names {
		hidden[n] = true
	}
	fs.Usage = func() {
		w := fs.Output()
		if fs.Name() == "" {
			fmt.Fprintf(w, "Usage:\n")
		} else {
			fmt.Fprintf(w, "Usage of %s:\n", fs.Name())
		}
		tmp := flag.NewFlagSet(fs.Name(), flag.ContinueOnError)
		tmp.SetOutput(w)
		fs.VisitAll(func(f *flag.Flag) {
			if !hidden[f.Name] {
				tmp.Var(f.Value, f.Name, f.Usage)
				tmp.Lookup(f.Name).DefValue = f.DefValue
			}
		})
		tmp.PrintDefaults()
	}
}

// configureMouseIRQ12Direct 在載入程式或狀態檔之前設定 §3.8 的診斷開關。
func configureMouseIRQ12Direct(m *machine.Machine, on bool) error {
	if !on {
		return nil
	}
	if err := m.SetMouseIRQ12Direct(true); err != nil {
		return fmt.Errorf("-%s：%w", mouseIRQ12DirectName, err)
	}
	return nil
}

// reportPIC459 列出 §3.8 的報告欄位（保護模式剖面時）：IRQ12 送達、使用者回呼、丟棄、A0h／A1h 讀寫次數、
// 結束時的在服務位元、UIR 與佇列、升級血統（PM 旗標 bit 5）、§3.3a 的 EOI 計數與各 IRQ 被擋住的最長連續步數、L。
func reportPIC459(w io.Writer, m *machine.Machine) {
	st := m.PICStats()
	tag := ""
	if st.Direct {
		tag = "【負對照，非正式結果：-" + mouseIRQ12DirectName + "】"
	}
	fmt.Fprintf(w, "\nPIC 與 IRQ12（docs/spec/459）%s：IRQ12 送達 %d 次、使用者回呼 %d 次、丟棄 %d 次、被拒排入 %d 次（溢位=%t）\n",
		tag, st.IRQ12, st.CallbacksMade, st.Discards, st.Rejected, st.Overflow)
	fmt.Fprintf(w, "  埠：A0h 讀 %d／寫 %d、A1h 讀 %d／寫 %d；從片 IMR=%02Xh、主片 IMR=%02Xh、讀取選擇 %v\n",
		st.A0Reads, st.A0Writes, st.A1Reads, st.A1Writes, st.SlaveIMR, st.MasterIMR, st.ReadISR)
	fmt.Fprintf(w, "  結束時：ISR0=%t ISR1=%t ISR2=%t ISR4=%t ISR7=%t UIR=%t 佇列 %d 筆；升級血統=%t %s %s\n",
		st.ISR0, st.ISR1, st.ISR2, st.ISR4, st.ISR7, st.UIR, st.Queued, st.LegacyMouseUpgrade, st.UpgradeNote, st.UpgradeFrames)
	fmt.Fprintf(w, "  非特定 EOI：主片 清 ISR0 %d、ISR1 %d、ISR2 %d、ISR7 %d、空 %d；從片 清 ISR4 %d、空 %d；特定 EOI %d\n",
		st.EOIMaster[0], st.EOIMaster[1], st.EOIMaster[2], st.EOIMaster[3], st.EOIMaster[4], st.EOISlave[0], st.EOISlave[1], st.EOISpecific)
	fmt.Fprintf(w, "  被在服務位元擋住的最長連續步數：IRQ0 %d（%s）、IRQ1 %d（%s）、IRQ12 %d（%s）、IRQ7 %d（%s）；L=%d（%s）\n",
		st.StuckMax[0], st.StuckMaxBy[0], st.StuckMax[1], st.StuckMaxBy[1], st.StuckMax[2], st.StuckMaxBy[2],
		st.StuckMax[3], st.StuckMaxBy[3], st.L, st.LSource)
}
