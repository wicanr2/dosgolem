package machine

import (
	"bytes"
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// 取指令頁快取（fetchpage.go）的差分測試：同一個 V86 分頁程式分別在「有快取」與「拿掉 FetchPager」
// 兩台機器上逐步執行，每一步比暫存器與 IP，最後比記憶體。程式改寫自己下一圈要執行的立即值
// （自我修改碼），中途重映程式碼頁並 SetCR3、切換 A20。
func TestFetchPageDifferential(t *testing.T) {
	// 0100 B0 00        mov al,0
	// 0102 FE 06 01 01  inc byte [0101]   ; 改寫上一道的立即值
	// 0106 00 C4        add ah,al
	// 0108 EB F6        jmp 0100
	code := []byte{0xB0, 0x00, 0xFE, 0x06, 0x01, 0x01, 0x00, 0xC4, 0xEB, 0xF6}
	build := func(pager bool) (*Machine, *tables) {
		m, tb := newPagedProg(t, code, fPWU)
		if !pager {
			m.CPU.SetFetchPager(nil)
		}
		mustNil(t, "SetVM(true)", m.SetVM(true))
		return m, tb
	}
	a, ta := build(true)
	b, tb := build(false)
	codeLin := uint32(a.CPU.Seg[cpu.CS])<<4 + 0x100
	alt := uint32(0x60000) // 重映目標：同一段程式，但 mov al,7
	for _, m := range []*Machine{a, b} {
		page := codeLin &^ 0xFFF
		copy(m.Mem[alt:alt+0x1000], m.Mem[page:page+0x1000])
		m.Mem[alt+(codeLin&0xFFF)+1] = 7
	}
	for step := 0; step < 4000; step++ {
		switch step {
		case 1000:
			ta.mapPage(codeLin, alt)
			tb.mapPage(codeLin, alt)
		case 1500: // 改 PTE 之後才作廢
			mustNil(t, "SetCR3", a.SetCR3(ta.dir))
			mustNil(t, "SetCR3", b.SetCR3(tb.dir))
		case 2500:
			a.SetA20(false)
			b.SetA20(false)
		case 3000:
			a.SetA20(true)
			b.SetA20(true)
		}
		ea, eb := a.Step(), b.Step()
		if (ea == nil) != (eb == nil) {
			t.Fatalf("step %d：錯誤 %v，參考 %v", step, ea, eb)
		}
		if a.CPU.R != b.CPU.R || a.CPU.IP != b.CPU.IP || a.CPU.Seg != b.CPU.Seg || a.CPU.Flags != b.CPU.Flags {
			t.Fatalf("step %d：暫存器不同 %v %04X，參考 %v %04X", step, a.CPU.R, a.CPU.IP, b.CPU.R, b.CPU.IP)
		}
	}
	if !bytes.Equal(a.Mem, b.Mem) || !bytes.Equal(a.Ext, b.Ext) {
		t.Fatal("記憶體內容不同")
	}
	if a.CPU.R[cpu.AX]>>8 == 0 {
		t.Fatal("程式沒有累加，可能沒在跑")
	}
}
