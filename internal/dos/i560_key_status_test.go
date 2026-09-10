package dos

import (
	"testing"

	"github.com/wicanr2/dosgolem/internal/cpu"
)

// TestI560KeyboardStatusSharesBIOSQueue 釘住 `int 21h AH=0Bh` 與 BIOS
// 鍵盤佇列看的是同一批鍵。
//
// **正反兩向都要驗**：有鍵時報有（三種來源各一），讀走之後報沒有。只驗
// 前半的話，一個永遠回 0xFF 的實作也會過。
func TestI560KeyboardStatusSharesBIOSQueue(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind int
		word uint16
	}{{"空佇列", 0, 0}, {"Stdin", 1, 0x3062}, {"BIOS-Left", 2, 0x4b00}, {"備援佇列", 3, 0x3062}} {
		t.Run(tc.name, func(t *testing.T) {
			m, d := newTest(t)
			switch tc.kind {
			case 1:
				d.Stdin = []byte{0x62}
			case 2:
				d.PushKey(Key{Scan: 0x4b})
			case 3:
				d.Keys = []uint16{tc.word}
			}
			want := uint8(0)
			if tc.kind != 0 {
				want = 0xff
			}
			// 查兩次：查詢本身不該把鍵吃掉。
			for i := 0; i < 2; i++ {
				call(m, d, 0x21, 0x0b00)
				if uint8(m.CPU.R[cpu.AX]) != want {
					t.Fatalf("第 %d 次查詢 AL=%02x 要 %02x", i+1, uint8(m.CPU.R[cpu.AX]), want)
				}
			}
			if tc.kind == 0 {
				return
			}
			call(m, d, 0x16, 0)
			if m.CPU.R[cpu.AX] != tc.word {
				t.Fatalf("讀回 %04x 要 %04x", m.CPU.R[cpu.AX], tc.word)
			}
			call(m, d, 0x21, 0x0b00)
			if uint8(m.CPU.R[cpu.AX]) != 0 {
				t.Fatal("讀走之後還報有鍵")
			}
		})
	}
}
