package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/wicanr2/dosgolem/internal/dos"
	"github.com/wicanr2/dosgolem/internal/machine"
)

// -kbd-int15（docs/spec/447-bios-int09-int15-keyboard-intercept §2.1、§2.6）。

// kbdInt15Flag 註冊 -kbd-int15。拆成函式是為了讓測試能在獨立的 FlagSet 上驗預設值與開啟後的值。
func kbdInt15Flag(fs *flag.FlagSet) *bool {
	return fs.Bool("kbd-int15", false,
		"BIOS int 09h 的預設處理改成客體碼，對每個掃描碼呼叫 int 15h AH=4Fh 鍵盤攔截"+
			"（`docs/spec/447-bios-int09-int15-keyboard-intercept`）。沒掛 int 09h 的程式也收得到 IRQ1。"+
			"預設關＝Go 端直接翻譯、IVT[09h] 在 stub 時 IRQ1 不送。不能與 -keys、-keys-at 並用")
}

// checkKbdInt15Keys 擋下 -kbd-int15 與 -keys、-keys-at 並用（447 §2.1）：feedKeys 同時排進 d.Stdin 與
// 硬體佇列，開啟本選項後硬體佇列的鍵也會進 BDA 緩衝，程式可能讀到兩次。
// -bios-keys、-bios-key-name(s) 直接寫 BDA、不經硬體路徑，不擋。
func checkKbdInt15Keys(on bool, keys, keysAt string) error {
	if !on {
		return nil
	}
	if keys != "" || keysAt != "" {
		return errors.New("-kbd-int15 不能與 -keys、-keys-at 並用：它們同時排進 Stdin 與硬體佇列，" +
			"開啟攔截後同一個鍵會進兩份緩衝（docs/spec/447 §2.1）")
	}
	return nil
}

// configureKbdInt15 在 -load-state 之後、送鍵排程之前呼叫（447 §2.1）。開啟時寫入客體碼；
// 關閉時檢查向量 9 的 stub：是 `E9 91 04` 表示狀態檔是在選項開啟時存的，回錯誤（呼叫端 die）。
func configureKbdInt15(m *machine.Machine, on bool) error {
	if on {
		if err := m.EnableKbdIntercept(); err != nil {
			return fmt.Errorf("-kbd-int15：%w", err)
		}
		return nil
	}
	if err := m.CheckKbdInterceptAbsent(); err != nil {
		return fmt.Errorf("%w；要同樣給 -kbd-int15", err)
	}
	return nil
}

// reportKbdInt15 印 447 §2.6 的一行，不論數字是否為 0。
func reportKbdInt15(w io.Writer, m *machine.Machine, d *dos.DOS) {
	fmt.Fprintf(w, "\n鍵盤攔截（-kbd-int15）：KeyIRQs=%d KeyStalls=%d TimedKeysPending=%d KbdIntercepts=%d KbdTranslates=%d\n",
		m.KeyIRQs, m.KeyStalls(), m.TimedKeysPending(), d.KbdIntercepts, d.KbdTranslates)
}
