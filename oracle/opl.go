package oracle

import (
	"io"

	"github.com/wicanr2/dosgolem/internal/machine"
	"github.com/wicanr2/dosgolem/internal/opl2"
)

// OPLWAV 把 OPL2 暫存器寫入序列合成成 16 位元單聲道 WAV（`docs/spec/196`，近似合成）。
//
// 回傳用到但沒實作的功能各幾次（節奏模式、顫音、KSL…）：非空表示聲音與真機的差異
// 不只是音色近似，要講出來。rate 為 0 用 22,050。只取 OPL2 相容那一組（Bank 0）。
func (o *Oracle) OPLWAV(w io.Writer, rate int) (map[string]int, error) {
	if rate <= 0 {
		rate = 22050
	}
	pcm, unsupported, err := opl2.Render(OPLEvents(o.m.OPL), machine.StepsPerSecond(), rate)
	if err != nil {
		return nil, err
	}
	return unsupported, opl2.WriteWAV16(w, rate, pcm)
}

// OPLEvents 把機器記的 OPL 寫入轉成合成器的事件，只留 Bank 0。
func OPLEvents(ws []machine.OPLWrite) []opl2.Event {
	out := make([]opl2.Event, 0, len(ws))
	for _, x := range ws {
		if x.Bank == 0 {
			out = append(out, opl2.Event{Step: x.Step, Reg: x.Reg, Val: x.Val})
		}
	}
	return out
}
